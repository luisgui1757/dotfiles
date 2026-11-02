"""Release certification boundaries; all GitHub/Git process boundaries are isolated."""
import argparse
import copy
import hashlib
import importlib.util
import json
import pathlib
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("release", ROOT / "scripts/release.py")
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


def notes(tag):
    return f"""# {tag} — Release fixture

## Highlights

- Verify the native installer release.

## Compatibility and upgrade

Clone the reviewed tag and use the local launcher.
"""


def bump(tag):
    major, minor, patch = release.semver(tag)
    return f"v{major}.{minor}.{patch + 1}"


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        for name in ("release", "docs/releases"):
            shutil.copytree(ROOT / name, self.root / name)
        (self.root / ".github").mkdir()
        shutil.copyfile(ROOT / ".github/check-identities.json", self.root / ".github/check-identities.json")
        self.tag = bump(release.validate_manifest(self.root)["tag"])
        self.set_version(self.tag, notes(self.tag))
        self.manifest = release.validate_manifest(self.root)

    def set_version(self, tag, text):
        (self.root / "release/manifest.json").write_text(json.dumps({"schema": 3, "version": tag}) + "\n", encoding="utf-8")
        (self.root / f"docs/releases/{tag}.md").write_text(text, encoding="utf-8")

    def evidence(self):
        run = {"databaseId": 123, "attempt": 1, "headSha": "2" * 40, "headBranch": self.tag,
               "event": "workflow_dispatch", "status": "completed", "conclusion": "success", "workflowName": "installer engine"}
        source = "steps:\n  - uses: actions/setup-go@" + "a" * 40 + "\n    with:\n      cache: ${{ !startsWith(github.ref, 'refs/tags/') }}\n"
        document = {"schema": 1, "run": run,
                    "jobs": [{"name": name, "run_id": 123, "run_attempt": 1, "head_sha": "2" * 40,
                              "status": "completed", "conclusion": "success"} for name in self.manifest["workflow_jobs"]],
                    "workflow_source": {"commit": "2" * 40, "path": ".github/workflows/installer-engine.yml",
                                        "text": source, "sha256": hashlib.sha256(source.encode()).hexdigest()}}
        return self.seal(document)

    @staticmethod
    def seal(document):
        raw = release.canonical_bytes(document)
        return {"document": document, "size": len(raw), "sha256": hashlib.sha256(raw).hexdigest()}

    def check_evidence(self, evidence):
        release.validate_job_evidence(evidence, self.manifest, "2" * 40, self.tag)

    def publish(self, parent, fail_at=None, previous="v1.1.0"):
        """Run the release with every gate replaced by a step that records its name."""
        commit = "2" * 40
        steps = []

        def step(name, result=None):
            def call(*_args, **_kwargs):
                steps.append(name)
                if name == fail_at:
                    raise release.ReleaseError(name)
                return result
            return call

        def output(argv, **_):
            return {("git", "rev-parse"): commit, ("git", "show"): json.dumps(parent),
                    ("gh", "repo"): self.manifest["repository"]}[tuple(argv[:2])]

        gates = {
            "previous_release": previous,
            "verify_merged_pr": {"tree": "5" * 40},
            "run": None,
            "verify_or_publish_tag": "4" * 40,
            "select_release_run": {"databaseId": 123},
            "archive_release_jobs": {},
            "verify_public_clone": None,
            "create_certification": {},
            "ensure_draft_release": ({"id": 42}, None, None),
            "publish_draft": None,
        }
        with mock.patch.object(release, "ROOT", self.root), mock.patch.object(release, "require_tool"), \
                mock.patch.object(release, "git_clean"), mock.patch.object(release, "output", side_effect=output):
            patches = [mock.patch.object(release, name, side_effect=step(name, result)) for name, result in gates.items()]
            for patch in patches:
                patch.start()
            try:
                release.publish(argparse.Namespace(commit=commit))
            finally:
                for patch in patches:
                    patch.stop()
        return steps

    def test_only_a_version_change_publishes_and_only_after_every_gate(self):
        changed = {"schema": 3, "version": "v1.1.0"}
        self.assertEqual(self.publish({"schema": 3, "version": self.tag}), [])
        self.assertEqual(self.publish(changed, previous=None), ["previous_release"])
        gates = ["previous_release", "verify_merged_pr", "run", "verify_or_publish_tag", "select_release_run",
                 "archive_release_jobs", "verify_public_clone", "create_certification", "ensure_draft_release",
                 "publish_draft"]
        # The manifest before this PR had no version field.
        self.assertEqual(self.publish({"schema": 2, "current": {"tag": "v1.1.0"}}), gates)
        for gate in gates[:-1]:
            with self.subTest(gate=gate), self.assertRaisesRegex(release.ReleaseError, gate):
                self.publish(changed, fail_at=gate)

    def test_manifest_names_one_version_with_its_reviewed_notes(self):
        self.assertEqual((self.manifest["tag"], self.manifest["title"]), (self.tag, f"{self.tag} — Release fixture"))
        self.assertEqual(self.manifest["workflow_jobs"], release.native_job_names(ROOT))
        self.assertTrue(release.release_body(notes(self.tag)).startswith("## Highlights\n"))
        manifest = self.root / "release/manifest.json"
        for label, data in (("extra field", {"schema": 3, "version": self.tag, "title": "x"}),
                            ("old schema", {"schema": 2, "version": self.tag}),
                            ("not canonical", {"schema": 3, "version": self.tag[1:]}),
                            ("no notes", {"schema": 3, "version": bump(self.tag)})):
            with self.subTest(label=label):
                manifest.write_text(json.dumps(data), encoding="utf-8")
                with self.assertRaises(release.ReleaseError):
                    release.validate_manifest(self.root)
        for label, text in (("other tag", notes(bump(self.tag))),
                            ("placeholder", notes(self.tag) + "\n<reviewed title>\n"),
                            ("no highlights", notes(self.tag).replace("## Highlights", "## Changes"))):
            with self.subTest(label=label):
                self.set_version(self.tag, text)
                with self.assertRaises(release.ReleaseError):
                    release.validate_manifest(self.root)

    def test_release_follows_the_latest_immutable_release_and_skips_a_published_version(self):
        def published(**fields):
            return subprocess.CompletedProcess([], 0, json.dumps({"tag_name": self.tag, **fields}), "")
        missing = subprocess.CompletedProcess([], 1, "", "HTTP 404")
        latest = {"tag_name": "v1.1.0", "immutable": True}
        with mock.patch.object(release, "run", return_value=published(immutable=True)), \
                mock.patch.object(release, "load_json_from_gh") as gh:
            self.assertIsNone(release.previous_release(self.manifest))
        gh.assert_not_called()
        with mock.patch.object(release, "run", return_value=missing), \
                mock.patch.object(release, "load_json_from_gh", return_value=latest):
            self.assertEqual(release.previous_release(self.manifest), "v1.1.0")
        for label, reply, newest in (("mutable published", published(immutable=False), latest),
                                     ("not newer", missing, {"tag_name": self.tag, "immutable": True}),
                                     ("mutable latest", missing, dict(latest, immutable=False))):
            with self.subTest(label=label), \
                    mock.patch.object(release, "run", return_value=reply), \
                    mock.patch.object(release, "load_json_from_gh", return_value=newest), \
                    self.assertRaises(release.ReleaseError):
                release.previous_release(self.manifest)

    def test_release_commit_must_be_the_checked_squash_merge_of_one_pull_request(self):
        commit, head = "2" * 40, "6" * 40
        required = json.loads((ROOT / ".github/check-identities.json").read_text(encoding="utf-8"))["required"]
        replies = {
            f"commits/{commit}/pulls": [{"number": 7, "merged_at": "2026-11-02T00:00:00Z", "merge_commit_sha": commit,
                                         "base": {"ref": "main"}, "head": {"sha": head}}],
            "rules/branches/main": [{"type": "pull_request"},
                                    {"type": "required_status_checks",
                                     "parameters": {"required_status_checks": [{"context": name} for name in required]}}],
            f"commits/{head}/check-runs?per_page=100": {"check_runs": [
                {"name": name, "head_sha": head, "status": "completed", "conclusion": "success", "app": {"id": 15368}}
                for name in [*required, "Analyze (python)"]]},
            f"git/commits/{head}": {"tree": {"sha": "5" * 40}},
            f"git/commits/{commit}": {"tree": {"sha": "5" * 40}},
        }

        def verify(change=None):
            answers = copy.deepcopy(replies)
            if change:
                change(answers)
            prefix = f"repos/{self.manifest['repository']}/"
            with mock.patch.object(release, "load_json_from_gh", side_effect=lambda args: answers[args[0].removeprefix(prefix)]):
                return release.verify_merged_pr(self.manifest, commit)

        self.assertEqual(verify(), {"number": 7, "head": head, "tree": "5" * 40,
                                    "checks": sorted([*required, "Analyze (python)"])})
        checks = f"commits/{head}/check-runs?per_page=100"
        for label, change in (
            ("direct push", lambda a: a.update({f"commits/{commit}/pulls": []})),
            ("open pull request", lambda a: a[f"commits/{commit}/pulls"][0].update(merged_at=None)),
            ("live policy drift", lambda a: a["rules/branches/main"][1]["parameters"]["required_status_checks"].pop()),
            ("failed check", lambda a: a[checks]["check_runs"][-1].update(conclusion="failure")),
            ("missing required check", lambda a: a[checks]["check_runs"].pop(0)),
            ("other tree", lambda a: a[f"git/commits/{commit}"].update(tree={"sha": "9" * 40})),
        ):
            with self.subTest(label=label), self.assertRaises(release.ReleaseError):
                verify(change)

    def test_rerun_keeps_a_passed_exact_tag_run_and_replaces_a_failed_one(self):
        commit = "2" * 40

        def tag_run(run_id, status="completed", conclusion="success", attempt=1):
            return {"databaseId": run_id, "attempt": attempt, "status": status, "conclusion": conclusion,
                    "headBranch": self.tag, "headSha": commit, "event": "workflow_dispatch",
                    "workflowName": "installer engine", "url": f"https://example.invalid/{run_id}"}

        def select(listings, watch_code=0):
            listings = iter(listings)
            calls = []

            def output(argv, **_):
                if argv[:3] == ["gh", "run", "list"]:
                    return json.dumps(next(listings))
                return json.dumps(tag_run(int(argv[3])))

            def run(argv, **_):
                calls.append(argv[:3])
                return subprocess.CompletedProcess(argv, watch_code if argv[:3] == ["gh", "run", "watch"] else 0, "", "")

            with mock.patch.object(release, "output", side_effect=output), \
                    mock.patch.object(release, "run", side_effect=run), mock.patch.object(release.time, "sleep"):
                return release.select_release_run(self.manifest, self.tag, commit)["databaseId"], calls

        failed = tag_run(10, conclusion="failure")
        self.assertEqual(select([[failed, tag_run(11), tag_run(12, attempt=2)]]),
                         (11, [["gh", "run", "watch"]]))
        self.assertEqual(select([[failed, tag_run(13, status="in_progress", conclusion="")]]),
                         (13, [["gh", "run", "watch"]]))
        self.assertEqual(select([[failed], [failed], [failed, tag_run(14, status="queued", conclusion="")]]),
                         (14, [["gh", "workflow", "run"], ["gh", "run", "watch"]]))
        with self.assertRaisesRegex(release.ReleaseError, "Re-run the release workflow"):
            select([[tag_run(15, status="in_progress", conclusion="")]], watch_code=1)

    def test_publication_requires_exact_immutable_latest_readback(self):
        draft = {"id": 42}
        published = {"id": 42, "tag_name": self.tag, "draft": False, "prerelease": False, "immutable": True}
        with mock.patch.object(release, "run") as gh, \
                mock.patch.object(release, "load_json_from_gh", side_effect=[published, published]):
            self.assertEqual(release.publish_draft(self.manifest, draft), published)
        gh.assert_called_once_with(["gh", "release", "edit", self.tag, "--draft=false", "--latest", "--verify-tag"],
                                   capture=False)
        mutable = dict(published, immutable=False)
        with mock.patch.object(release, "run"), \
                mock.patch.object(release, "load_json_from_gh", side_effect=[mutable, mutable]):
            with self.assertRaisesRegex(release.ReleaseError, "immutable/latest"):
                release.publish_draft(self.manifest, draft)

    def test_evidence_binds_archived_bytes_exact_head_attempt_and_every_job(self):
        original = self.evidence()
        self.check_evidence(original)
        for label, change in (
            ("wrong commit", lambda d: d["jobs"][0].update(head_sha="3" * 40)),
            ("wrong attempt", lambda d: d["run"].update(attempt=2)),
            ("wrong run", lambda d: d["jobs"][0].update(run_id=124)),
            ("failed job", lambda d: d["jobs"][0].update(conclusion="failure")),
            ("skipped job", lambda d: d["jobs"][0].update(status="queued", conclusion="skipped")),
            ("missing job", lambda d: d["jobs"].pop()),
            ("duplicate job", lambda d: d["jobs"].append(d["jobs"][0])),
            ("wrong tag", lambda d: d["run"].update(headBranch="main")),
        ):
            with self.subTest(label=label):
                document = copy.deepcopy(original["document"])
                change(document)
                with self.assertRaises(release.ReleaseError):
                    self.check_evidence(self.seal(document))
        original["document"]["jobs"][0]["conclusion"] = "failure"
        with self.assertRaisesRegex(release.ReleaseError, "digest"):
            self.check_evidence(original)

    def test_tag_caches_must_be_disabled_in_archived_source(self):
        for text in ("steps: []", "  - uses: actions/setup-go@" + "a" * 40 + "\n    with:\n      cache: true\n",
                     self.evidence()["document"]["workflow_source"]["text"] + "  - uses: actions/cache@" + "a" * 40):
            document = self.evidence()["document"]
            document["workflow_source"].update(text=text, sha256=hashlib.sha256(text.encode()).hexdigest())
            with self.assertRaisesRegex(release.ReleaseError, "cache"):
                self.check_evidence(self.seal(document))


if __name__ == "__main__":
    unittest.main()
