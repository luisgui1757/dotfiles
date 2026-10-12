"""Release certification boundaries; all GitHub/Git process boundaries are isolated."""
import copy
import hashlib
import importlib.util
import pathlib
import shutil
import tempfile
import unittest

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

    def test_closure_retains_archived_evidence_and_all_historical_proofs(self):
        proof = {"schema": 2, "kind": "publication-closure", "repository": self.manifest["repository"],
                 "tag": self.tag, "previous_tag": self.manifest["current"]["previous_tag"],
                 "tag_object": "4" * 40, "commit": "2" * 40, "tree": "5" * 40,
                 "preparation": {"pull_request": 1, "head": "6" * 40},
                 "workflow": {"name": "installer-engine.yml", "run_id": 123, "run_attempt": 1, "conclusion": "success"}, "job_evidence": self.evidence(),
                 "scans": {"release_range": "previous..candidate", "proof_bytes": self.evidence()["size"]},
                 "release": {"id": 1, "published_at": "2026-10-10T00:00:00Z", "immutable": True,
                             "latest": True, "draft": False, "prerelease": False}}
        release.render_closure(self.root, proof, "7" * 64)
        manifest = release.validate_manifest(self.root)
        self.assertEqual(manifest["current"]["state"], "published")
        self.assertEqual(release.validate_proof(self.root / manifest["current"]["proof"], manifest)["job_evidence"], proof["job_evidence"])
        for path, data in self.historical.items():
            if str(path).startswith("release/proofs/"):
                self.assertEqual((self.root / path).read_bytes(), data)
        next_tag = self.tag.rsplit(".", 1)[0] + "." + str(int(self.tag.rsplit(".", 1)[1]) + 1)
        release.render_candidate(self.root, version=next_tag, notes_text=notes(next_tag), base_commit="8" * 40)


if __name__ == "__main__":
    unittest.main()
