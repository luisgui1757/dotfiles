"""Release certification boundaries; all GitHub/Git process boundaries are isolated."""
import copy
import hashlib
import importlib.util
import json
import os
import pathlib
import shutil
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("release", ROOT / "scripts/release.py")
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


def notes(tag):
    return f"""# {tag} — Release fixture

> Release candidate notes. Publish these only with the official annotated
> `{tag}` tag after the deterministic evidence gate below passes.

## Highlights

- Verify the native installer release.

## Compatibility and upgrade

Clone the reviewed tag and use the local launcher.

## Release identity

Exact annotated tag and commit are recorded after verification.

## Evidence required before publication

- Local and required hosted checks, native jobs, scan and fresh clone.
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
        self.historical = {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob("*") if p.is_file()}
        initial = release.validate_manifest(self.root)
        if initial["current"]["state"] == "candidate":
            self.tag = initial["current"]["tag"]
            self.manifest = initial
        else:
            major, minor, patch = release.semver(initial["current"]["tag"])
            self.tag = f"v{major}.{minor}.{patch + 1}"
            release.render_candidate(self.root, version=self.tag, notes_text=notes(self.tag), base_commit="1" * 40)
            self.manifest = release.validate_manifest(self.root)

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

    def test_publication_keeps_irreversible_and_exact_identity_guards(self):
        source = (ROOT / "scripts/release.py").read_text()
        for required in ('/immutable-releases', '"--log-opts=', '"workflow_dispatch"',
                         '"GIT_TERMINAL_PROMPT": "0"', '"--draft"',
                         'PUBLISH IMMUTABLE {tag} @ {expected_sha}', '"--draft=false"',
                         '"release-proof.json"', 'resume_publication_closure',
                         'live required-check policy differs', 'head_tree != merged_tree'):
            self.assertIn(required, source)
        for forbidden in ('"--clobber"', '"--yes"', 'git push --force', 'tag -d'):
            self.assertNotIn(forbidden, source)

    def test_prepare_preserves_history_and_uses_native_matrix(self):
        self.assertEqual(self.manifest["schema"], 2)
        self.assertEqual(self.manifest["workflow"], "installer-engine.yml")
        for path, data in self.historical.items():
            if path == pathlib.Path("release/manifest.json"):
                continue
            self.assertEqual((self.root / path).read_bytes(), data, str(path))
        with self.assertRaises(release.ReleaseError):
            release.render_candidate(self.root, version=self.tag, notes_text=notes(self.tag), base_commit="1" * 40)

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

    def closure_proof(self):
        return {"schema": 2, "kind": "publication-closure", "repository": self.manifest["repository"],
                 "tag": self.tag, "previous_tag": self.manifest["current"]["previous_tag"],
                 "tag_object": "4" * 40, "commit": "2" * 40, "tree": "5" * 40,
                 "preparation": {"pull_request": 1, "head": "6" * 40},
                 "workflow": {"name": "installer-engine.yml", "run_id": 123, "run_attempt": 1, "conclusion": "success"}, "job_evidence": self.evidence(),
                 "scans": {"release_range": "previous..candidate", "proof_bytes": self.evidence()["size"]},
                 "release": {"id": 1, "published_at": "2026-10-10T00:00:00Z", "immutable": True,
                             "latest": True, "draft": False, "prerelease": False}}

    def test_closure_retains_archived_evidence_and_all_historical_proofs(self):
        proof = self.closure_proof()
        release.render_closure(self.root, proof, "7" * 64)
        manifest = release.validate_manifest(self.root)
        self.assertEqual(manifest["current"]["state"], "published")
        self.assertEqual(release.validate_proof(self.root / manifest["current"]["proof"], manifest)["job_evidence"], proof["job_evidence"])
        for path, data in self.historical.items():
            if str(path).startswith("release/proofs/"):
                self.assertEqual((self.root / path).read_bytes(), data)
        next_tag = bump(self.tag)
        release.render_candidate(self.root, version=next_tag, notes_text=notes(next_tag), base_commit="8" * 40)

    def test_supersede_replaces_only_the_active_candidate(self):
        release.render_closure(self.root, self.closure_proof(), "7" * 64)
        published = release.validate_manifest(self.root)
        first, second = bump(self.tag), bump(bump(self.tag))
        release.render_candidate(self.root, version=first, notes_text=notes(first), base_commit="8" * 40)
        candidate = release.validate_manifest(self.root)
        with mock.patch.object(release, "ROOT", self.root), \
                mock.patch.object(release, "output", return_value=json.dumps(published)) as git:
            self.assertEqual(release.superseded_base(candidate, first), published)
            git.assert_called_once_with(["git", "show", "8" * 40 + ":release/manifest.json"])
            for label, tag in (("published", self.tag), ("unknown", second)):
                with self.subTest(label=label), self.assertRaises(release.ReleaseError):
                    release.superseded_base(candidate, tag)
        drifted = copy.deepcopy(published)
        drifted["current"]["previous_tag"] = "v0.0.1"
        with mock.patch.object(release, "ROOT", self.root), \
                mock.patch.object(release, "output", return_value=json.dumps(drifted)), \
                self.assertRaises(release.ReleaseError):
            release.superseded_base(candidate, first)
        before = {p: p.read_bytes() for p in self.root.rglob("*") if p.is_file()}
        for label, version, tag in (("inactive tag", second, self.tag), ("not newer", first, first)):
            with self.subTest(label=label), self.assertRaises(release.ReleaseError):
                release.render_candidate(self.root, version=version, notes_text=notes(version),
                                         base_commit="9" * 40, supersede=tag)
            self.assertEqual({p: p.read_bytes() for p in self.root.rglob("*") if p.is_file()}, before)
        rendered = release.render_candidate(self.root, version=second, notes_text=notes(second),
                                            base_commit="9" * 40, supersede=first)
        manifest = release.validate_manifest(self.root)
        self.assertEqual((manifest["current"]["tag"], manifest["current"]["previous_tag"]), (second, self.tag))
        self.assertEqual(sorted(rendered), sorted([f"docs/releases/{second}.md", "release/manifest.json",
                                                   f"docs/releases/{first}.md"]))
        title, rest = notes(first).split("\n", 1)
        self.assertEqual((self.root / f"docs/releases/{first}.md").read_text(encoding="utf-8"), title + "\n"
                         f"\n> Superseded before publication by `{second}`. The `{first}` tag exists,\n"
                         "> but no GitHub release was published for it.\n" + rest)
        with self.assertRaises(release.ReleaseError):
            release.mark_superseded_notes(self.root, first, second)
        # A second supersession follows the first candidate back to the release.
        chain = {"9" * 40: candidate, "8" * 40: published}
        def show(argv):
            return json.dumps(chain[argv[2].split(":")[0]])
        with mock.patch.object(release, "ROOT", self.root), mock.patch.object(release, "output", side_effect=show):
            self.assertEqual(release.superseded_base(manifest, second), published)
            chain["9" * 40] = copy.deepcopy(candidate)
            chain["9" * 40]["current"]["previous_tag"] = "v0.0.1"
            with self.assertRaises(release.ReleaseError):
                release.superseded_base(manifest, second)

    def test_supersede_requires_that_no_release_exists_for_the_tag(self):
        with mock.patch.object(release, "output", return_value="v0.4.4\nv1.0.0\n") as gh:
            with self.assertRaisesRegex(release.ReleaseError, "draft or published"):
                release.require_unreleased(self.manifest, "v1.0.0")
        self.assertIn("--paginate", gh.call_args.args[0])
        with mock.patch.object(release, "output", return_value="v0.4.4\nv1.0.10\n"):
            release.require_unreleased(self.manifest, "v1.0.1")


class CommitRenderedTests(unittest.TestCase):
    """The full gate runs between rendering and committing a release change."""

    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = pathlib.Path(temp.name)
        environment = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
        environment.update(
            GIT_CONFIG_GLOBAL=os.devnull,
            GIT_CONFIG_NOSYSTEM="1",
            GIT_AUTHOR_NAME="Fixture",
            GIT_AUTHOR_EMAIL="fixture@example.invalid",
            GIT_COMMITTER_NAME="Fixture",
            GIT_COMMITTER_EMAIL="fixture@example.invalid",
        )
        patcher = mock.patch.dict(os.environ, environment, clear=True)
        patcher.start()
        self.addCleanup(patcher.stop)
        release.run(["git", "init", "-q", "-b", "main"], cwd=self.root)
        (self.root / "tracked.txt").write_text("before\n")
        release.run(["git", "add", "tracked.txt"], cwd=self.root)
        release.run(["git", "commit", "-q", "-m", "initial"], cwd=self.root)
        (self.root / "release").mkdir()
        (self.root / "release/manifest.json").write_text("{}\n")

    def committed(self):
        return release.output(["git", "show", "--name-only", "--format=", "HEAD"], cwd=self.root).splitlines()

    def test_gate_leftovers_are_refused_and_only_rendered_files_are_committed(self):
        # v1.0.0's preparation committed the gate's Python bytecode.
        for leftover in ("tests/bootstrap/__pycache__/fixture.cpython-314.pyc", "tracked.txt"):
            with self.subTest(leftover=leftover):
                path = self.root / leftover
                path.parent.mkdir(parents=True, exist_ok=True)
                original = path.read_bytes() if path.exists() else None
                path.write_text("gate output\n")
                with self.assertRaisesRegex(release.ReleaseError, "unexpected change"):
                    release.commit_rendered(self.root, ["release/manifest.json"], "fixture")
                self.assertEqual(self.committed(), ["tracked.txt"])
                if original is None:
                    path.unlink()
                else:
                    path.write_bytes(original)
        release.commit_rendered(self.root, ["release/manifest.json"], "fixture")
        self.assertEqual(self.committed(), ["release/manifest.json"])


if __name__ == "__main__":
    unittest.main()
