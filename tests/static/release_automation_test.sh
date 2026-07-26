#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 scripts/release.py check >/dev/null

python3 - <<'PY'
import importlib.util
import pathlib
import shutil
import tempfile

root = pathlib.Path.cwd()
spec = importlib.util.spec_from_file_location("dotfiles_release", root / "scripts/release.py")
module = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(module)

source = (root / "scripts/release.py").read_text(encoding="utf-8")
for required in (
    '/immutable-releases',
    '"--log-opts=',
    '"workflow_dispatch"',
    '"PR-only cache:"',
    '"scripts/ci-logical-proof.sh"',
    '"GIT_TERMINAL_PROMPT": "0"',
    '"--draft"',
    'PUBLISH IMMUTABLE {tag} @ {expected_sha}',
    '"--draft=false"',
    '"release-proof.json"',
    'resume_publication_closure',
):
    assert required in source, f"release publisher lost required boundary: {required}"
for forbidden in ('"--clobber"', '"--yes"', 'git push --force', 'tag -d'):
    assert forbidden not in source, f"release publisher contains unsafe bypass: {forbidden}"

notes = """# v0.4.4 — Automated release fixture

> Release candidate notes. Publish these only with the official annotated
> `v0.4.4` tag after the deterministic evidence gate below passes.

## Highlights

- Exercise the release renderer without inventing a publication claim.

## Compatibility and upgrade

Clone the exact annotated `v0.4.4` tag beside the prior checkout.

## Release identity

The exact local and official annotated tag identities must agree. Before
publication, the explicit unreleased field-test lane remains separate.

## Evidence required before publication

- full local and hosted gates;
- exact tag, proof, scan, clone, and immutable release readback.
"""

with tempfile.TemporaryDirectory(prefix="dotfiles-release-render-test.") as temporary:
    fixture = pathlib.Path(temporary) / "repo"
    shutil.copytree(
        root,
        fixture,
        symlinks=True,
        ignore=shutil.ignore_patterns(".git", ".cache", "__pycache__", "*.pyc"),
    )
    base = "1" * 40
    module.render_candidate(fixture, version="v0.4.4", notes_text=notes, base_commit=base)
    candidate = module.validate_manifest(fixture)
    assert candidate["current"]["state"] == "candidate"
    assert candidate["current"]["previous_tag"] == "v0.4.3"
    assert (fixture / "docs/releases/v0.4.3.md").is_file()
    assert (fixture / "release/proofs/v0.4.3.json").is_file()
    setup = (fixture / "setup.sh").read_text(encoding="utf-8")
    setup_ps1 = (fixture / "setup.ps1").read_text(encoding="utf-8")
    assert 'RELEASE_TAG="v0.4.4"' in setup
    assert '"v0.4.3"' in setup.split("LEGACY_RELEASE_TAGS=", 1)[1].splitlines()[0]
    assert "$ReleaseTag     = 'v0.4.4'" in setup_ps1
    assert "'v0.4.3'" in setup_ps1.split("$LegacyReleaseTags =", 1)[1].splitlines()[0]
    assert "release-candidate sources" in (fixture / "docs/security/supply-chain.md").read_text(encoding="utf-8")

    logical = []
    for index, item in enumerate(candidate["logical_proofs"], start=1):
        logical.append(
            {
                "artifact": item["artifact"],
                "marker": item["marker"],
                "size": 200 + index,
                "sha256": str(index) * 64,
            }
        )
    proof = {
        "schema": 1,
        "kind": "publication-closure",
        "repository": candidate["repository"],
        "tag": "v0.4.4",
        "previous_tag": "v0.4.3",
        "tag_object": "2" * 40,
        "commit": "3" * 40,
        "tree": "4" * 40,
        "preparation": {
            "pull_request": 99,
            "head": "5" * 40,
            "tree": "4" * 40,
            "required_checks": ["fixture"],
        },
        "workflow": {
            "name": "e2e-install.yml",
            "run_id": 123456,
            "run_attempt": 1,
            "url": "https://github.com/luisgui1757/dotfiles/actions/runs/123456",
            "conclusion": "success",
        },
        "logical_proofs": logical,
        "scans": {
            "gitleaks_version": "8.30.1",
            "release_range": "v0.4.3..v0.4.4",
            "proof_bytes": sum(item["size"] for item in logical),
        },
        "fresh_public_clone": "passed",
        "release": {
            "id": 987654,
            "published_at": "2026-07-26T12:00:00Z",
            "immutable": True,
            "latest": True,
            "draft": False,
            "prerelease": False,
        },
        "certified_at": "2026-07-26T11:00:00Z",
        "residual_evidence": module.RESIDUAL_EVIDENCE,
    }
    module.render_closure(fixture, proof, "6" * 64)
    published = module.validate_manifest(fixture)
    assert published["current"]["state"] == "published"
    assert published["current"]["proof"] == "release/proofs/v0.4.4.json"
    checked_proof = module.load_json(fixture / "release/proofs/v0.4.4.json")
    assert checked_proof["certification_asset"]["sha256"] == "6" * 64
    final_notes = (fixture / "docs/releases/v0.4.4.md").read_text(encoding="utf-8")
    assert "## Publication evidence" in final_notes
    assert "## Evidence required before publication" not in final_notes
    assert "immutable/latest GitHub release" in final_notes

    next_notes = notes.replace("v0.4.4", "v0.4.5").replace(
        "Automated release fixture", "Second automated release fixture"
    )
    module.render_candidate(
        fixture,
        version="v0.4.5",
        notes_text=next_notes,
        base_commit="7" * 40,
    )
    second_candidate = module.validate_manifest(fixture)
    assert second_candidate["current"]["state"] == "candidate"
    assert second_candidate["current"]["previous_tag"] == "v0.4.4"
    second_setup = (fixture / "setup.sh").read_text(encoding="utf-8")
    assert 'RELEASE_TAG="v0.4.5"' in second_setup
    assert '"v0.4.4"' in second_setup.split("LEGACY_RELEASE_TAGS=", 1)[1].splitlines()[0]

print("release manifest, candidate renderer, and publication closure OK")
PY
