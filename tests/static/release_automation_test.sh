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
import stat
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

def next_patch(tag):
    major, minor, patch = module.semver(tag)
    return f"v{major}.{minor}.{patch + 1}"


def candidate_notes(tag, title="Automated release fixture"):
    return f"""# {tag} — {title}

> Release candidate notes. Publish these only with the official annotated
> `{tag}` tag after the deterministic evidence gate below passes.

## Highlights

- Exercise the release renderer without inventing a publication claim.

## Compatibility and upgrade

Clone the exact annotated `{tag}` tag beside the prior checkout.

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
    initial = module.validate_manifest(fixture)
    if initial["current"]["state"] == "published":
        previous_tag = initial["current"]["tag"]
        candidate_tag = next_patch(previous_tag)
        original_modes = {
            relative: stat.S_IMODE((fixture / relative).stat().st_mode)
            for relative in module.GLOBAL_VERSION_FILES
        }
        module.render_candidate(
            fixture,
            version=candidate_tag,
            notes_text=candidate_notes(candidate_tag),
            base_commit="1" * 40,
        )
        for relative, original_mode in original_modes.items():
            assert stat.S_IMODE((fixture / relative).stat().st_mode) == original_mode, (
                f"release renderer changed file mode for {relative}"
            )
        candidate = module.validate_manifest(fixture)
    else:
        candidate = initial
        candidate_tag = candidate["current"]["tag"]
        previous_tag = candidate["current"]["previous_tag"]

    assert candidate["current"]["state"] == "candidate"
    assert candidate["current"]["previous_tag"] == previous_tag
    assert (fixture / f"docs/releases/{previous_tag}.md").is_file()
    assert (fixture / f"release/proofs/{previous_tag}.json").is_file()
    for relative in (
        "scripts/install-nix-prerequisite.sh",
        "scripts/upgrade-v0.1.0.sh",
        "setup.sh",
        "tests/migration/v0_1_upgrade_test.sh",
        "tests/shell/nix_prerequisite_identity_test.sh",
        "tests/shell/setup_universal_entrypoint_test.sh",
        "tests/static/darwin_platform_contract_test.sh",
        "tests/static/release_upgrade_test.sh",
    ):
        assert stat.S_IMODE((fixture / relative).stat().st_mode) & stat.S_IXUSR, (
            f"release-controlled executable lost its owner execute bit: {relative}"
        )
    setup = (fixture / "setup.sh").read_text(encoding="utf-8")
    setup_ps1 = (fixture / "setup.ps1").read_text(encoding="utf-8")
    assert f'RELEASE_TAG="{candidate_tag}"' in setup
    assert f'"{previous_tag}"' in setup.split("LEGACY_RELEASE_TAGS=", 1)[1].splitlines()[0]
    assert f"$ReleaseTag     = '{candidate_tag}'" in setup_ps1
    assert f"'{previous_tag}'" in setup_ps1.split("$LegacyReleaseTags =", 1)[1].splitlines()[0]
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
        "tag": candidate_tag,
        "previous_tag": previous_tag,
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
            "release_range": f"{previous_tag}..{candidate_tag}",
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
    assert published["current"]["proof"] == f"release/proofs/{candidate_tag}.json"
    checked_proof = module.load_json(fixture / f"release/proofs/{candidate_tag}.json")
    assert checked_proof["certification_asset"]["sha256"] == "6" * 64
    final_notes = (fixture / f"docs/releases/{candidate_tag}.md").read_text(encoding="utf-8")
    assert "## Publication evidence" in final_notes
    assert "## Evidence required before publication" not in final_notes
    assert "immutable/latest GitHub release" in final_notes

    next_tag = next_patch(candidate_tag)
    module.render_candidate(
        fixture,
        version=next_tag,
        notes_text=candidate_notes(next_tag, "Second automated release fixture"),
        base_commit="7" * 40,
    )
    second_candidate = module.validate_manifest(fixture)
    assert second_candidate["current"]["state"] == "candidate"
    assert second_candidate["current"]["previous_tag"] == candidate_tag
    second_setup = (fixture / "setup.sh").read_text(encoding="utf-8")
    assert f'RELEASE_TAG="{next_tag}"' in second_setup
    assert f'"{candidate_tag}"' in second_setup.split("LEGACY_RELEASE_TAGS=", 1)[1].splitlines()[0]

print("release manifest, candidate renderer, and publication closure OK")
PY
