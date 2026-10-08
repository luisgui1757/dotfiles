#!/usr/bin/env python3
"""Fail-closed preparation and publication of immutable dotfiles releases."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import time
from typing import Any, Iterable


ROOT = pathlib.Path(__file__).resolve().parent.parent
MANIFEST_PATH = pathlib.Path("release/manifest.json")
HEX40 = re.compile(r"[0-9a-f]{40}\Z")
SHA256 = re.compile(r"[0-9a-f]{64}\Z")
SEMVER = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")
RESIDUAL_EVIDENCE = [
    "redirected Windows",
    "divergent Windows Terminal targets",
    "physical Linux",
    "Apple Silicon owner host",
    "manual visual checks",
]


class ReleaseError(RuntimeError):
    pass


def fail(message: str) -> None:
    raise ReleaseError(message)


def run(
    argv: Iterable[str],
    *,
    cwd: pathlib.Path = ROOT,
    capture: bool = True,
    check: bool = True,
    env: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    command = list(argv)
    completed = subprocess.run(
        command,
        cwd=cwd,
        check=False,
        text=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        env=env,
    )
    if check and completed.returncode != 0:
        detail = (completed.stderr or completed.stdout or "").strip()
        fail(f"command failed ({completed.returncode}): {' '.join(command)}\n{detail}")
    return completed


def output(argv: Iterable[str], *, cwd: pathlib.Path = ROOT) -> str:
    return run(argv, cwd=cwd).stdout.strip()


def load_json(path: pathlib.Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"cannot read valid JSON from {path}: {exc}")


def write_json(path: pathlib.Path, value: Any) -> None:
    write_text(path, json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def write_text(path: pathlib.Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    existing_mode = stat.S_IMODE(path.stat().st_mode) if path.exists() else None
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8", newline="") as handle:
            handle.write(value)
            handle.flush()
            os.fsync(handle.fileno())
        if existing_mode is not None:
            os.chmod(temporary, existing_mode)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def replace_exact(text: str, old: str, new: str, *, count: int | None = None) -> str:
    observed = text.count(old)
    if count is not None and observed != count:
        fail(f"expected {count} occurrence(s) of {old!r}, found {observed}")
    if observed == 0:
        fail(f"required release surface is missing {old!r}")
    return text.replace(old, new)


def semver(tag: str) -> tuple[int, int, int]:
    match = SEMVER.fullmatch(tag)
    if not match:
        fail(f"release version must be canonical vMAJOR.MINOR.PATCH: {tag}")
    return tuple(int(part) for part in match.groups())  # type: ignore[return-value]


def validate_notes(text: str, tag: str, *, candidate: bool) -> str:
    lines = text.splitlines()
    if not lines or not lines[0].startswith(f"# {tag} — "):
        fail(f"release notes must start with '# {tag} — <reviewed title>'")
    for heading in (
        "## Highlights",
        "## Compatibility and upgrade",
        "## Release identity",
    ):
        if text.count(heading) != 1:
            fail(f"release notes must contain exactly one {heading!r} heading")
    evidence_heading = "## Evidence required before publication" if candidate else "## Publication evidence"
    if text.count(evidence_heading) != 1:
        fail(f"release notes must contain exactly one {evidence_heading!r} heading")
    if candidate and f"> Release candidate notes. Publish these only with the official annotated\n> `{tag}` tag" not in text:
        fail("candidate release notes must carry the exact non-publication warning")
    if "<" in text and re.search(r"<[A-Za-z][^>]*>", text):
        fail("release notes retain an angle-bracket placeholder")
    if tag not in text:
        fail("release notes do not name the candidate tag")
    return lines[0][2:]


def release_body(notes: str) -> str:
    start = notes.find("## Highlights")
    end = notes.find("## Evidence required before publication")
    if start < 0 or end <= start:
        fail("cannot extract the reviewed public release body")
    body = notes[start:end].rstrip() + "\n"
    if "Release candidate" in body:
        fail("public release body contains candidate-only language")
    return body


def validate_logical_proofs(value: Any) -> list[dict[str, str]]:
    if not isinstance(value, list) or len(value) != 4:
        fail("manifest must define exactly four logical proofs")
    expected_keys = {"artifact", "marker", "logical_context", "legacy_context"}
    result: list[dict[str, str]] = []
    for item in value:
        if not isinstance(item, dict) or set(item) != expected_keys:
            fail("logical proof entries must use the exact reviewed schema")
        if not all(isinstance(item[key], str) and item[key] for key in expected_keys):
            fail("logical proof identities must be nonempty strings")
        if any("/" in item[key] for key in ("artifact", "marker")):
            fail("logical proof artifact and marker names must be basenames")
        result.append(item)
    if len({item["artifact"] for item in result}) != 4:
        fail("logical proof artifact names must be unique")
    return result


def validate_proof(path: pathlib.Path, manifest: dict[str, Any]) -> dict[str, Any]:
    proof = load_json(path)
    if not isinstance(proof, dict) or proof.get("schema") != manifest["schema"]:
        fail(f"unsupported release proof schema: {path}")
    for key in ("tag", "previous_tag"):
        semver(str(proof.get(key, "")))
    for key in ("tag_object", "commit", "tree"):
        if not HEX40.fullmatch(str(proof.get(key, ""))):
            fail(f"release proof has invalid {key}: {path}")
    if proof.get("repository") != manifest["repository"]:
        fail(f"release proof repository mismatch: {path}")
    release = proof.get("release")
    if not isinstance(release, dict) or any(
        release.get(key) is not expected
        for key, expected in (("immutable", True), ("latest", True), ("draft", False), ("prerelease", False))
    ):
        fail(f"release proof does not describe an immutable latest release: {path}")
    if manifest["schema"] == 1:
        entries = proof.get("logical_proofs")
        if not isinstance(entries, list) or len(entries) != 4:
            fail(f"release proof must contain four logical proof digests: {path}")
        for entry in entries:
            if not isinstance(entry, dict) or not SHA256.fullmatch(str(entry.get("sha256", ""))):
                fail(f"release proof contains an invalid SHA-256: {path}")
            if not isinstance(entry.get("size"), int) or entry["size"] <= 0:
                fail(f"release proof contains an invalid marker size: {path}")
        expected_proofs = [
            (item["artifact"], item["marker"])
            for item in validate_logical_proofs(manifest["logical_proofs"])
        ]
        observed_proofs = [(item.get("artifact"), item.get("marker")) for item in entries]
        if observed_proofs != expected_proofs:
            fail(f"release proof artifact/marker identities drifted from the manifest: {path}")
    else:
        validate_job_evidence(proof.get("job_evidence"), manifest, proof["commit"], proof["tag"])
        record = proof["job_evidence"]["document"]["run"]
        workflow = proof.get("workflow", {})
        if (workflow.get("name") != manifest["workflow"] or workflow.get("run_id") != record["databaseId"]
                or workflow.get("run_attempt") != record["attempt"] or workflow.get("conclusion") != "success"):
            fail("release proof workflow disagrees with archived native evidence")

    certification_asset = proof.get("certification_asset")
    if certification_asset is not None and (
        not isinstance(certification_asset, dict)
        or certification_asset.get("name") != "release-proof.json"
        or not SHA256.fullmatch(str(certification_asset.get("sha256", "")))
    ):
        fail(f"release proof certification asset identity is invalid: {path}")
    return proof


def validate_manifest(root: pathlib.Path = ROOT) -> dict[str, Any]:
    path = root / MANIFEST_PATH
    manifest = load_json(path)
    if not isinstance(manifest, dict) or manifest.get("schema") not in {1, 2}:
        fail("release manifest schema must be 1 or 2")
    if set(manifest) != {
        "schema",
        "repository",
        "official_remote",
        "workflow",
        "current",
        "logical_proofs" if manifest["schema"] == 1 else "workflow_jobs",
    }:
        fail("release manifest contains missing or unknown top-level fields")
    if manifest.get("repository") != "luisgui1757/dotfiles":
        fail("release manifest repository is not the reviewed public repository")
    if manifest.get("official_remote") != "https://github.com/luisgui1757/dotfiles.git":
        fail("release manifest official remote drifted")
    expected_workflow = "e2e-install.yml" if manifest["schema"] == 1 else "installer-engine.yml"
    if manifest.get("workflow") != expected_workflow:
        fail("release manifest workflow drifted")
    if manifest["schema"] == 1:
        if manifest.get("current", {}).get("state") != "published":
            fail("schema 1 is historical published evidence only")
        validate_logical_proofs(manifest.get("logical_proofs"))
    else:
        validate_workflow_jobs(manifest.get("workflow_jobs"))
    current = manifest.get("current")
    if not isinstance(current, dict) or current.get("state") not in {"published", "candidate"}:
        fail("release manifest current state must be published or candidate")
    tag = str(current.get("tag", ""))
    previous = str(current.get("previous_tag", ""))
    if semver(tag) <= semver(previous):
        fail("release manifest current tag must be newer than previous_tag")
    notes = pathlib.PurePosixPath(str(current.get("notes", "")))
    if notes.is_absolute() or ".." in notes.parts or notes.as_posix() != f"docs/releases/{tag}.md":
        fail("release manifest notes path is not the versioned in-repo release note")
    notes_path = root / notes
    if not notes_path.is_file():
        fail(f"release notes are missing: {notes}")
    title = validate_notes(
        notes_path.read_text(encoding="utf-8"),
        tag,
        candidate=current["state"] == "candidate",
    )
    if title != current.get("title"):
        fail("release manifest title does not match the release-note H1")
    if current["state"] == "published":
        if set(current) != {"state", "tag", "previous_tag", "title", "notes", "proof"}:
            fail("published release manifest current fields drifted")
        proof_path = pathlib.PurePosixPath(str(current.get("proof", "")))
        if proof_path.as_posix() != f"release/proofs/{tag}.json":
            fail("published release manifest proof path drifted")
        proof = validate_proof(root / proof_path, manifest)
        if proof["tag"] != tag or proof["previous_tag"] != previous:
            fail("published release manifest and proof identities disagree")
    else:
        if set(current) != {
            "state",
            "tag",
            "previous_tag",
            "title",
            "notes",
            "preparation",
        }:
            fail("candidate release manifest current fields drifted")
        preparation = current.get("preparation")
        if not isinstance(preparation, dict) or set(preparation) != {"base_commit", "branch"}:
            fail("candidate preparation identity is incomplete")
        if not HEX40.fullmatch(str(preparation.get("base_commit", ""))):
            fail("candidate base commit is invalid")
        if preparation.get("branch") != f"release/{tag}":
            fail("candidate branch does not match the tag")
    validate_current_surfaces(root, manifest)
    return manifest


def validate_current_surfaces(root: pathlib.Path, manifest: dict[str, Any]) -> None:
    # Published schema-1 evidence describes the released tree, not today's runtime.
    # New candidates bind the reviewed native workflow matrix in their manifest.
    if manifest["schema"] == 2 and manifest["current"]["state"] == "candidate":
        if manifest["workflow_jobs"] != native_job_names(root):
            fail("candidate native job identities differ from canonical required checks")


def native_job_names(root: pathlib.Path) -> list[str]:
    policy = load_json(root / ".github/check-identities.json")
    return validate_workflow_jobs([name for name in policy["required"] if name not in {"ubuntu", "macos", "windows"}])


def validate_workflow_jobs(value: Any) -> list[str]:
    if (not isinstance(value, list) or not 1 <= len(value) <= 64
            or any(not isinstance(name, str) or not name or len(name) > 150 for name in value)
            or len(value) != len(set(value))):
        fail("native workflow job identities are invalid")
    return value


def canonical_bytes(value: Any) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True) + "\n").encode("utf-8")


def validate_job_evidence(value: Any, manifest: dict[str, Any], commit: str, tag: str) -> None:
    if not isinstance(value, dict) or set(value) != {"sha256", "size", "document"}:
        fail("native job evidence must contain the canonical archived document and digest")
    encoded = canonical_bytes(value["document"])
    if len(encoded) > 4 * 1024 * 1024 or value["size"] != len(encoded) or value["sha256"] != hashlib.sha256(encoded).hexdigest():
        fail("native job evidence bytes do not match their bounded digest")
    document = value["document"]
    if not isinstance(document, dict) or set(document) != {"schema", "run", "jobs", "workflow_source"} or document["schema"] != 1:
        fail("native job evidence document schema drifted")
    run_record = document["run"]
    expected = {"headSha": commit, "headBranch": tag, "attempt": 1, "event": "workflow_dispatch", "status": "completed", "conclusion": "success", "workflowName": "installer engine"}
    if not isinstance(run_record, dict) or any(run_record.get(key) != val for key, val in expected.items()) or not isinstance(run_record.get("databaseId"), int) or run_record["databaseId"] < 1:
        fail("native job evidence run is not the first successful exact-tag run")
    jobs = document["jobs"]
    if not isinstance(jobs, list) or len(jobs) != len(manifest["workflow_jobs"]) or any(not isinstance(job, dict) for job in jobs) or {job.get("name") for job in jobs} != set(manifest["workflow_jobs"]):
        fail("native job evidence job set differs from the reviewed matrix")
    if any(job.get("head_sha") != commit or job.get("run_id") != run_record["databaseId"] or job.get("run_attempt") != 1 or job.get("status") != "completed" or job.get("conclusion") != "success" for job in jobs):
        fail("not every native job completed successfully at the exact release identity")
    source = document["workflow_source"]
    if not isinstance(source, dict) or set(source) != {"commit", "path", "text", "sha256"} or source["commit"] != commit or source["path"] != ".github/workflows/installer-engine.yml":
        fail("native workflow source identity drifted")
    text = source["text"]
    if not isinstance(text, str) or hashlib.sha256(text.encode("utf-8")).hexdigest() != source["sha256"]:
        fail("native workflow source digest drifted")
    blocks = re.findall(r"(?ms)^ *- uses: actions/setup-go@[^\n]+\n(.*?)(?=^ *- (?:uses|name):|\Z)", text)
    cache_setting = "cache: ${{ !startsWith(github.ref, 'refs/tags/') }}"
    if (not blocks or len(blocks) != text.count("uses: actions/setup-go@")
            or any(not re.search(r"(?m)^ +" + re.escape(cache_setting) + r" *$", block) for block in blocks)
            or "actions/cache@" in text):
        fail("exact-tag workflow source does not disable every Go cache and broad Actions cache")


def git_clean(root: pathlib.Path) -> None:
    if output(["git", "status", "--porcelain=v1"], cwd=root):
        fail(f"worktree is not clean: {root}")


def require_exact_main(root: pathlib.Path, manifest: dict[str, Any]) -> str:
    git_clean(root)
    if output(["git", "branch", "--show-current"], cwd=root) != "main":
        fail("release command must start from the main branch")
    head = output(["git", "rev-parse", "HEAD^{commit}"], cwd=root)
    remote = output(["git", "remote", "get-url", "origin"], cwd=root)
    accepted = {manifest["official_remote"], "git@github.com:luisgui1757/dotfiles.git"}
    if remote not in accepted:
        fail(f"origin is not the reviewed official remote: {remote}")
    refs = output(["git", "ls-remote", "--heads", "origin", "refs/heads/main"], cwd=root).splitlines()
    if refs != [f"{head}\trefs/heads/main"]:
        fail("local main is not the exact current official main head")
    return head


def render_candidate(
    root: pathlib.Path,
    *,
    version: str,
    notes_text: str,
    base_commit: str,
) -> None:
    manifest = validate_manifest(root)
    current = manifest["current"]
    if current["state"] != "published":
        fail("cannot prepare a second candidate while one is already active")
    if semver(version) <= semver(current["tag"]):
        fail("candidate version must be newer than the published release")
    title = validate_notes(notes_text, version, candidate=True)
    current_tag = current["tag"]
    manifest = {
        "schema": 2,
        "repository": manifest["repository"],
        "official_remote": manifest["official_remote"],
        "workflow": "installer-engine.yml",
        "workflow_jobs": native_job_names(root),
    }
    note_path = root / f"docs/releases/{version}.md"
    if note_path.exists():
        fail(f"candidate release notes already exist: {note_path}")
    write_text(note_path, notes_text.rstrip() + "\n")
    manifest["current"] = {
        "state": "candidate",
        "tag": version,
        "previous_tag": current_tag,
        "title": title,
        "notes": f"docs/releases/{version}.md",
        "preparation": {
            "base_commit": base_commit,
            "branch": f"release/{version}",
        },
    }
    write_json(root / MANIFEST_PATH, manifest)
    validate_manifest(root)


def prepare(args: argparse.Namespace) -> None:
    manifest = validate_manifest(ROOT)
    check_live(manifest)
    base = require_exact_main(ROOT, manifest)
    notes_path = pathlib.Path(args.notes).expanduser().resolve()
    if not notes_path.is_file():
        fail(f"reviewed release notes file is missing: {notes_path}")
    notes = notes_path.read_text(encoding="utf-8")
    validate_notes(notes, args.version, candidate=True)
    branch = f"release/{args.version}"
    if run(["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"], check=False).returncode == 0:
        fail(f"local release branch already exists: {branch}")
    remote_branch = output(["git", "ls-remote", "--heads", "origin", f"refs/heads/{branch}"])
    if remote_branch:
        fail(f"official release branch already exists: {branch}")
    worktree = ROOT.parent / f"{ROOT.name}-release-{args.version[1:]}"
    if worktree.exists():
        fail(f"release worktree path already exists: {worktree}")
    run(["git", "worktree", "add", "-b", branch, str(worktree), base], capture=False)
    render_candidate(worktree, version=args.version, notes_text=notes, base_commit=base)
    run(["make", "ci"], cwd=worktree, capture=False)
    run(["git", "diff", "--check"], cwd=worktree)
    run(["git", "add", "--all"], cwd=worktree)
    run(["git", "commit", "-m", f"chore(release): prepare {args.version}"], cwd=worktree, capture=False)
    run(["git", "push", "-u", "origin", branch], cwd=worktree, capture=False)
    pr_url = output(
        [
            "gh",
            "pr",
            "create",
            "--base",
            "main",
            "--head",
            branch,
            "--title",
            f"chore(release): prepare {args.version}",
            "--body",
            (
                "## Summary\n\n"
                f"- prepare the exact `{args.version}` release identity and native certification matrix\n"
                "- add reviewed candidate notes and deterministic publication gates\n"
                "- preserve historical publication evidence unchanged\n\n"
                "## Verification\n\n- `make ci`\n"
            ),
        ],
        cwd=worktree,
    )
    print(f"prepared release worktree: {worktree}")
    print(f"pull request: {pr_url}")


def check_live(manifest: dict[str, Any]) -> None:
    current = manifest["current"]
    if current["state"] != "published":
        fail("live check applies only to a published manifest")
    proof = validate_proof(ROOT / current["proof"], manifest)
    refs = output(
        [
            "git",
            "ls-remote",
            "--tags",
            manifest["official_remote"],
            f"refs/tags/{current['tag']}",
            f"refs/tags/{current['tag']}^{{}}",
        ]
    ).splitlines()
    expected = {
        f"{proof['tag_object']}\trefs/tags/{current['tag']}",
        f"{proof['commit']}\trefs/tags/{current['tag']}^{{}}",
    }
    if set(refs) != expected or len(refs) != 2:
        fail("published manifest does not match the official annotated tag")
    release = load_json_from_gh(["repos/{repo}/releases/{release_id}".format(
        repo=manifest["repository"], release_id=proof["release"]["id"]
    )])
    if (
        release.get("id") != proof["release"]["id"]
        or release.get("tag_name") != current["tag"]
        or release.get("name") != current["title"]
        or release.get("immutable") is not True
        or release.get("draft") is not False
        or release.get("prerelease") is not False
    ):
        fail("published manifest does not match the immutable GitHub release")
    latest = load_json_from_gh([f"repos/{manifest['repository']}/releases/latest"])
    if latest.get("id") != proof["release"]["id"]:
        fail("published manifest release is no longer latest")
    workflow = json.loads(
        output(
            [
                "gh",
                "run",
                "view",
                str(proof["workflow"]["run_id"]),
                "--json",
                "attempt,conclusion,databaseId,event,headBranch,headSha,status,workflowName",
            ]
        )
    )
    workflow_expected = {
        "attempt": proof["workflow"]["run_attempt"],
        "conclusion": "success",
        "databaseId": proof["workflow"]["run_id"],
        "event": "workflow_dispatch",
        "headBranch": current["tag"],
        "headSha": proof["commit"],
        "status": "completed",
        "workflowName": "e2e-install" if manifest["schema"] == 1 else "installer engine",
    }
    if workflow != workflow_expected:
        fail("published manifest workflow identity no longer matches GitHub")
    certification_asset = proof.get("certification_asset")
    if certification_asset is not None:
        matching = [
            item for item in release.get("assets", []) if item.get("name") == certification_asset.get("name")
        ]
        if (
            len(matching) != 1
            or matching[0].get("digest") != f"sha256:{certification_asset.get('sha256')}"
        ):
            fail("published certification asset does not match the closure proof")


def require_tool(name: str) -> None:
    if shutil.which(name) is None:
        fail(f"required release tool is unavailable: {name}")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_preparation_pr(manifest: dict[str, Any], expected_sha: str) -> dict[str, Any]:
    pulls = load_json_from_gh(
        [
            f"repos/{manifest['repository']}/commits/{expected_sha}/pulls",
            "-H",
            "Accept: application/vnd.github+json",
        ]
    )
    matching = [
        item
        for item in pulls
        if item.get("state") == "closed"
        and item.get("merged_at")
        and item.get("merge_commit_sha") == expected_sha
        and item.get("base", {}).get("ref") == "main"
        and item.get("head", {}).get("ref") == manifest["current"]["preparation"]["branch"]
    ]
    if len(matching) != 1:
        fail("expected SHA is not the unique merge of the manifest-bound preparation PR")
    pull = matching[0]
    checks_raw = output(
        [
            "gh",
            "pr",
            "checks",
            str(pull["number"]),
            "--json",
            "name,bucket,state,workflow",
        ]
    )
    try:
        checks = json.loads(checks_raw)
    except json.JSONDecodeError as exc:
        fail(f"required PR checks returned invalid JSON: {exc}")
    if not checks or any(item.get("bucket") != "pass" for item in checks):
        fail("not every hosted preparation-PR check passed")
    required_raw = output(
        [
            "gh",
            "pr",
            "checks",
            str(pull["number"]),
            "--required",
            "--json",
            "name,bucket,state,workflow",
        ]
    )
    required = json.loads(required_raw)
    if not required or any(item.get("bucket") != "pass" for item in required):
        fail("not every required preparation-PR check passed")
    head_sha = pull.get("head", {}).get("sha")
    if not HEX40.fullmatch(str(head_sha or "")):
        fail("preparation PR head identity is invalid")
    canonical_required = load_json(ROOT / ".github/check-identities.json")["required"]
    if set(item["name"] for item in required) != set(canonical_required):
        fail("live required-check policy differs from the reviewed canonical check set")
    head_checks = load_json_from_gh([f"repos/{manifest['repository']}/commits/{head_sha}/check-runs?per_page=100", "--paginate"])
    for name in canonical_required:
        matches = [item for item in head_checks.get("check_runs", []) if item.get("name") == name
                   and item.get("head_sha") == head_sha and item.get("status") == "completed"
                   and item.get("conclusion") == "success" and item.get("app", {}).get("id") == 15368]
        if len(matches) != 1:
            fail(f"required check is not uniquely successful at the reviewed PR head: {name}")
    head_commit = load_json_from_gh([f"repos/{manifest['repository']}/git/commits/{head_sha}"])
    merged_commit = load_json_from_gh([f"repos/{manifest['repository']}/git/commits/{expected_sha}"])
    head_tree = str(head_commit.get("tree", {}).get("sha", ""))
    merged_tree = str(merged_commit.get("tree", {}).get("sha", ""))
    if not HEX40.fullmatch(head_tree) or not HEX40.fullmatch(merged_tree):
        fail("GitHub returned an invalid preparation or merge tree identity")
    if head_tree != merged_tree:
        fail("squash-merged preparation tree differs from the reviewed PR head tree")
    return {
        "pull_request": pull["number"],
        "head": head_sha,
        "tree": merged_tree,
        "hosted_checks": sorted(item["name"] for item in checks),
        "required_checks": sorted(item["name"] for item in required),
    }


def verify_or_publish_tag(manifest: dict[str, Any], tag: str, expected_sha: str) -> str:
    refs = output(
        [
            "git",
            "ls-remote",
            "--tags",
            "origin",
            f"refs/tags/{tag}",
            f"refs/tags/{tag}^{{}}",
        ]
    ).splitlines()
    if not refs:
        local = run(["git", "rev-parse", "--verify", f"refs/tags/{tag}"], check=False)
        if local.returncode == 0:
            fail(f"local tag already exists while the official tag is absent: {tag}")
        run(["git", "tag", "-a", tag, "-m", tag, expected_sha], capture=False)
        run(["git", "push", "origin", f"refs/tags/{tag}:refs/tags/{tag}"], capture=False)
        refs = output(
            [
                "git",
                "ls-remote",
                "--tags",
                "origin",
                f"refs/tags/{tag}",
                f"refs/tags/{tag}^{{}}",
            ]
        ).splitlines()
    if len(refs) != 2:
        fail("official release tag is missing, lightweight, duplicated, or incomplete")
    mapping = dict(line.split("\t", 1)[::-1] for line in refs)
    if mapping.get(f"refs/tags/{tag}^{{}}") != expected_sha:
        fail("official annotated tag does not peel to the expected release commit")
    tag_object = mapping.get(f"refs/tags/{tag}", "")
    if not HEX40.fullmatch(tag_object):
        fail("official release tag object identity is invalid")
    local_probe = run(["git", "rev-parse", "--verify", f"refs/tags/{tag}"], check=False)
    if local_probe.returncode != 0:
        run(
            ["git", "fetch", "--no-tags", "origin", f"refs/tags/{tag}:refs/tags/{tag}"],
            capture=False,
        )
    local_object = output(["git", "rev-parse", f"refs/tags/{tag}"])
    local_commit = output(["git", "rev-parse", f"refs/tags/{tag}^{{commit}}"])
    if local_object != tag_object or local_commit != expected_sha:
        fail("local and official annotated release identities disagree")
    return tag_object


def select_release_run(
    manifest: dict[str, Any], tag: str, expected_sha: str, requested_run_id: int | None
) -> dict[str, Any]:
    if requested_run_id is None:
        existing_raw = output(
            [
                "gh",
                "run",
                "list",
                "--workflow",
                manifest["workflow"],
                "--event",
                "workflow_dispatch",
                "--commit",
                expected_sha,
                "--limit",
                "100",
                "--json",
                "databaseId",
            ]
        )
        existing = {item["databaseId"] for item in json.loads(existing_raw)}
        run(
            ["gh", "workflow", "run", manifest["workflow"], "--ref", tag],
            capture=False,
        )
        deadline = time.monotonic() + 180
        selected: list[dict[str, Any]] = []
        while time.monotonic() < deadline:
            raw = output(
                [
                    "gh",
                    "run",
                    "list",
                    "--workflow",
                    manifest["workflow"],
                    "--event",
                    "workflow_dispatch",
                    "--commit",
                    expected_sha,
                    "--limit",
                    "100",
                    "--json",
                    "attempt,conclusion,createdAt,databaseId,event,headBranch,headSha,status,url,workflowName",
                ]
            )
            selected = [
                item
                for item in json.loads(raw)
                if item["databaseId"] not in existing
                and item.get("headBranch") == tag
                and item.get("headSha") == expected_sha
            ]
            if selected:
                break
            time.sleep(3)
        if len(selected) != 1:
            fail("could not select one new exact-tag workflow_dispatch run")
        requested_run_id = selected[0]["databaseId"]
    run(
        ["gh", "run", "watch", str(requested_run_id), "--exit-status", "--interval", "10"],
        capture=False,
    )
    raw = output(
        [
            "gh",
            "run",
            "view",
            str(requested_run_id),
            "--json",
            "attempt,conclusion,databaseId,event,headBranch,headSha,status,url,workflowName",
        ]
    )
    selected_run = json.loads(raw)
    expected = {
        "databaseId": requested_run_id,
        "event": "workflow_dispatch",
        "headBranch": tag,
        "headSha": expected_sha,
        "status": "completed",
        "conclusion": "success",
        "workflowName": "e2e-install" if manifest["schema"] == 1 else "installer engine",
    }
    for key, value in expected.items():
        if selected_run.get(key) != value:
            fail(f"release workflow {key} mismatch: expected {value!r}, got {selected_run.get(key)!r}")
    if selected_run.get("attempt") != 1:
        fail("release certification requires a first-attempt workflow run")
    return selected_run


def archive_release_jobs(
    manifest: dict[str, Any], run_record: dict[str, Any], expected_sha: str, tag: str, proof_root: pathlib.Path
) -> dict[str, Any]:
    response = load_json_from_gh([f"repos/{manifest['repository']}/actions/runs/{run_record['databaseId']}/jobs?per_page=100", "--paginate"])
    source = run(["git", "show", f"{expected_sha}:.github/workflows/installer-engine.yml"]).stdout
    document = {
        "schema": 1,
        "run": run_record,
        "jobs": response.get("jobs", []),
        "workflow_source": {"commit": expected_sha, "path": ".github/workflows/installer-engine.yml", "text": source, "sha256": hashlib.sha256(source.encode("utf-8")).hexdigest()},
    }
    encoded = canonical_bytes(document)
    evidence = {"sha256": hashlib.sha256(encoded).hexdigest(), "size": len(encoded), "document": document}
    validate_job_evidence(evidence, manifest, expected_sha, tag)
    proof_root.mkdir(mode=0o700)
    (proof_root / "native-jobs.json").write_bytes(encoded)
    run(["gitleaks", "dir", "--no-banner", "--redact", str(proof_root)], capture=False)
    return evidence


def verify_public_clone(manifest: dict[str, Any], tag: str, tag_object: str, commit: str) -> None:
    environment = os.environ.copy()
    environment.update(
        {
            "GIT_TERMINAL_PROMPT": "0",
            "GCM_INTERACTIVE": "Never",
            "GIT_ASKPASS": "/usr/bin/false",
            "SSH_ASKPASS": "/usr/bin/false",
        }
    )
    with tempfile.TemporaryDirectory(prefix="dotfiles-release-clone.") as temporary:
        clone = pathlib.Path(temporary) / "dotfiles"
        run(
            [
                "git",
                "-c",
                "credential.helper=",
                "clone",
                "--branch",
                tag,
                "--single-branch",
                manifest["official_remote"],
                str(clone),
            ],
            env=environment,
            capture=False,
        )
        if output(["git", "rev-parse", f"refs/tags/{tag}"], cwd=clone) != tag_object:
            fail("fresh public clone resolved a different tag object")
        if output(["git", "rev-parse", "HEAD^{commit}"], cwd=clone) != commit:
            fail("fresh public clone resolved a different release commit")
        if output(["git", "branch", "--show-current"], cwd=clone):
            fail("fresh exact-tag clone is not detached")
        git_clean(clone)
        run(["bash", "tests/static/release_upgrade_test.sh"], cwd=clone, capture=False)
        run(["python3", "scripts/release.py", "check"], cwd=clone, capture=False)


def create_certification(
    manifest: dict[str, Any],
    preparation: dict[str, Any],
    tag_object: str,
    commit: str,
    run_record: dict[str, Any],
    job_evidence: dict[str, Any],
    release_id: int,
) -> dict[str, Any]:
    return {
        "schema": 2,
        "kind": "pre-publication-certification",
        "repository": manifest["repository"],
        "tag": manifest["current"]["tag"],
        "previous_tag": manifest["current"]["previous_tag"],
        "tag_object": tag_object,
        "commit": commit,
        "tree": preparation["tree"],
        "preparation": preparation,
        "workflow": {
            "name": manifest["workflow"],
            "run_id": run_record["databaseId"],
            "run_attempt": run_record["attempt"],
            "url": run_record["url"],
            "conclusion": "success",
        },
        "job_evidence": job_evidence,
        "scans": {
            "gitleaks_version": output(["gitleaks", "version"]),
            "release_range": f"{manifest['current']['previous_tag']}..{manifest['current']['tag']}",
            "proof_bytes": job_evidence["size"],
        },
        "fresh_public_clone": "passed",
        "release": {
            "id": release_id,
            "draft": True,
            "immutable": False,
            "publication": "pending explicit confirmation",
        },
        "residual_evidence": RESIDUAL_EVIDENCE,
    }


def ensure_draft_release(
    manifest: dict[str, Any], body: str, certification_base: dict[str, Any], temporary: pathlib.Path
) -> tuple[dict[str, Any], pathlib.Path, str]:
    tag = manifest["current"]["tag"]
    existing = run(["gh", "release", "view", tag, "--json", "databaseId"], check=False)
    body_path = temporary / "release-body.md"
    write_text(body_path, body)
    if existing.returncode != 0:
        run(
            [
                "gh",
                "release",
                "create",
                tag,
                "--draft",
                "--verify-tag",
                "--title",
                manifest["current"]["title"],
                "--notes-file",
                str(body_path),
            ],
            capture=False,
        )
        existing = run(["gh", "release", "view", tag, "--json", "databaseId"])
    try:
        release_id = int(json.loads(existing.stdout)["databaseId"])
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
        fail(f"draft release did not expose one database identity: {exc}")
    release = load_json_from_gh([f"repos/{manifest['repository']}/releases/{release_id}"])
    if (
        release.get("tag_name") != tag
        or release.get("name") != manifest["current"]["title"]
        or release.get("draft") is not True
        or release.get("prerelease") is not False
        or release.get("immutable") is not False
        or str(release.get("body", "")).rstrip() != body.rstrip()
    ):
        fail("draft release does not exactly match the reviewed candidate")
    certification = dict(certification_base)
    certification["release"] = {
        "id": release["id"],
        "draft": True,
        "immutable": False,
        "publication": "pending explicit confirmation",
    }
    proof_path = temporary / "release-proof.json"
    write_json(proof_path, certification)
    proof_hash = sha256_file(proof_path)
    matching_assets = [item for item in release.get("assets", []) if item.get("name") == "release-proof.json"]
    if not matching_assets:
        run(["gh", "release", "upload", tag, str(proof_path)], capture=False)
        release = load_json_from_gh([f"repos/{manifest['repository']}/releases/{release_id}"])
        matching_assets = [item for item in release.get("assets", []) if item.get("name") == "release-proof.json"]
    if len(matching_assets) != 1:
        fail("draft release must contain exactly one release-proof.json asset")
    deadline = time.monotonic() + 60
    asset = matching_assets[0]
    while asset.get("digest") is None and time.monotonic() < deadline:
        time.sleep(2)
        release = load_json_from_gh([f"repos/{manifest['repository']}/releases/{release_id}"])
        matching_assets = [item for item in release.get("assets", []) if item.get("name") == "release-proof.json"]
        if len(matching_assets) != 1:
            fail("draft release proof asset disappeared during digest readback")
        asset = matching_assets[0]
    if asset.get("size") != proof_path.stat().st_size or asset.get("digest") != f"sha256:{proof_hash}":
        fail("draft release proof asset bytes differ from the local certification")
    return release, proof_path, proof_hash


def publish_draft(manifest: dict[str, Any], expected_sha: str, draft: dict[str, Any]) -> dict[str, Any]:
    tag = manifest["current"]["tag"]
    phrase = f"PUBLISH IMMUTABLE {tag} @ {expected_sha}"
    print("\nEvery deterministic publication gate passed.")
    print("GitHub release immutability makes the next boundary irreversible.")
    entered = input(f"Type exactly '{phrase}' to publish: ")
    if entered != phrase:
        fail("immutable publication confirmation did not match; the verified draft remains unpublished")
    run(["gh", "release", "edit", tag, "--draft=false", "--latest", "--verify-tag"], capture=False)
    release = load_json_from_gh([f"repos/{manifest['repository']}/releases/{draft['id']}"])
    latest = load_json_from_gh([f"repos/{manifest['repository']}/releases/latest"])
    if (
        release.get("id") != draft["id"]
        or release.get("tag_name") != tag
        or release.get("draft") is not False
        or release.get("prerelease") is not False
        or release.get("immutable") is not True
        or latest.get("id") != release.get("id")
    ):
        fail("published GitHub release did not read back immutable/latest and exact")
    return release


def closure_evidence(proof: dict[str, Any]) -> str:
    return f"""## Publication evidence

- Reviewed PR #{proof['preparation']['pull_request']} head `{proof['preparation']['head']}`
  and merged commit `{proof['commit']}` have exact tree `{proof['tree']}`.
- Full local and required hosted checks passed; redacted scans covered
  `{proof['scans']['release_range']}` and the archived native-job evidence.
- First-attempt exact-tag run `{proof['workflow']['run_id']}` passed every reviewed
  native job at the release commit. The archived workflow source disables tag caches.
- Canonical job/source evidence is embedded in the proof: {proof['scans']['proof_bytes']}
  bytes, SHA-256 `{proof['job_evidence']['sha256']}`.
- A fresh credential-free detached public clone reproduced the annotated tag,
  peeled commit, and release manifest checks.
- GitHub release `{proof['release']['id']}` read back immutable/latest, non-draft,
  non-prerelease, with exact reviewed body and certification asset.

Unexecuted owner-host and manual visual checks remain residual evidence; publication
never marks them complete. See the proof's `residual_evidence` list.
"""


def render_closure(root: pathlib.Path, proof: dict[str, Any], asset_sha256: str) -> None:
    manifest = validate_manifest(root)
    current = manifest["current"]
    if current["state"] != "candidate" or current["tag"] != proof["tag"]:
        fail("closure worktree is not the matching release candidate")
    tag = current["tag"]
    previous = current["previous_tag"]
    notes_path = root / current["notes"]
    notes = notes_path.read_text(encoding="utf-8")
    first_heading = notes.splitlines()[0]
    notes = re.sub(
        r"\A# .*?\n\n> .*?(?=\n\n## Highlights)",
        (
            f"{first_heading}\n\n> Published on {proof['release']['published_at'][:10]} as immutable/latest GitHub release\n"
            f"> [`{proof['release']['id']}`](https://github.com/{proof['repository']}/releases/tag/{tag}).\n"
            f"> Annotated tag object `{proof['tag_object']}` peels to commit `{proof['commit']}`."
        ),
        notes,
        count=1,
        flags=re.DOTALL,
    )
    evidence_start = notes.find("## Evidence required before publication")
    if evidence_start < 0:
        fail("candidate release notes lost the publication evidence gate")
    notes = notes[:evidence_start] + closure_evidence(proof)
    write_text(notes_path, notes.rstrip() + "\n")

    proof["certification_asset"] = {
        "name": "release-proof.json",
        "sha256": asset_sha256,
    }
    proof_path = root / f"release/proofs/{tag}.json"
    if proof_path.exists():
        fail(f"release closure proof already exists: {proof_path}")
    write_json(proof_path, proof)
    manifest["current"] = {
        "state": "published",
        "tag": tag,
        "previous_tag": previous,
        "title": current["title"],
        "notes": current["notes"],
        "proof": f"release/proofs/{tag}.json",
    }
    write_json(root / MANIFEST_PATH, manifest)
    validate_manifest(root)


def open_closure_pr(manifest: dict[str, Any], proof: dict[str, Any], asset_sha256: str) -> str:
    tag = manifest["current"]["tag"]
    branch = f"release/{tag}-closure"
    if run(["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"], check=False).returncode == 0:
        fail(f"local closure branch already exists: {branch}")
    if output(["git", "ls-remote", "--heads", "origin", f"refs/heads/{branch}"]):
        fail(f"official closure branch already exists: {branch}")
    worktree = ROOT.parent / f"{ROOT.name}-release-{tag[1:]}-closure"
    if worktree.exists():
        fail(f"release closure worktree path already exists: {worktree}")
    run(["git", "worktree", "add", "-b", branch, str(worktree), proof["commit"]], capture=False)
    render_closure(worktree, proof, asset_sha256)
    run(["make", "ci"], cwd=worktree, capture=False)
    run(["git", "diff", "--check"], cwd=worktree)
    run(["git", "add", "--all"], cwd=worktree)
    run(["git", "commit", "-m", f"docs(release): close {tag} publication"], cwd=worktree, capture=False)
    run(["git", "push", "-u", "origin", branch], cwd=worktree, capture=False)
    return output(
        [
            "gh",
            "pr",
            "create",
            "--base",
            "main",
            "--head",
            branch,
            "--title",
            f"docs(release): close {tag} publication",
            "--body",
            (
                "## Summary\n\n"
                "- record observed annotated-tag, exact-run, proof, scan, clone, and immutable-release identities\n"
                "- bind the uploaded pre-publication certification asset to the final closure proof\n"
                "- preserve every unexecuted owner-host/manual row as an explicit residual gap\n\n"
                "## Verification\n\n- `make ci`\n"
            ),
        ],
        cwd=worktree,
    )


def resume_publication_closure(
    manifest: dict[str, Any], release: dict[str, Any], expected_sha: str, tag_object: str
) -> str:
    current = manifest["current"]
    latest = load_json_from_gh([f"repos/{manifest['repository']}/releases/latest"])
    if (
        release.get("tag_name") != current["tag"]
        or release.get("draft") is not False
        or release.get("prerelease") is not False
        or release.get("immutable") is not True
        or latest.get("id") != release.get("id")
    ):
        fail("existing published release is not immutable/latest and exact")
    with tempfile.TemporaryDirectory(prefix="dotfiles-release-resume.") as temporary_name:
        temporary = pathlib.Path(temporary_name)
        run(
            [
                "gh",
                "release",
                "download",
                current["tag"],
                "--pattern",
                "release-proof.json",
                "--dir",
                str(temporary),
            ],
            capture=False,
        )
        asset_path = temporary / "release-proof.json"
        if not asset_path.is_file() or asset_path.is_symlink():
            fail("published release certification asset is missing or unsafe")
        certification = load_json(asset_path)
        if (
            not isinstance(certification, dict)
            or certification.get("schema") != manifest["schema"]
            or certification.get("kind") != "pre-publication-certification"
            or certification.get("repository") != manifest["repository"]
            or certification.get("tag") != current["tag"]
            or certification.get("previous_tag") != current["previous_tag"]
            or certification.get("commit") != expected_sha
            or certification.get("tag_object") != tag_object
            or certification.get("release", {}).get("id") != release["id"]
        ):
            fail("published release certification asset does not bind this candidate")
        asset_sha256 = sha256_file(asset_path)
        assets = [item for item in release.get("assets", []) if item.get("name") == "release-proof.json"]
        if len(assets) != 1 or assets[0].get("digest") != f"sha256:{asset_sha256}":
            fail("published release certification asset digest does not match its API identity")
        final_proof = certification
        final_proof["kind"] = "publication-closure"
        final_proof["release"] = {
            "id": release["id"],
            "published_at": release["published_at"],
            "immutable": True,
            "latest": True,
            "draft": False,
            "prerelease": False,
        }
        return open_closure_pr(manifest, final_proof, asset_sha256)


def publish(args: argparse.Namespace) -> None:
    for tool in ("gh", "git", "gitleaks", "make", "python3"):
        require_tool(tool)
    manifest = validate_manifest(ROOT)
    current = manifest["current"]
    if current["state"] != "candidate" or current["tag"] != args.version:
        fail("manifest does not describe the requested release candidate")
    expected_sha = args.expected_sha
    if not HEX40.fullmatch(expected_sha):
        fail("EXPECTED_SHA must be a full lowercase commit identity")
    head = require_exact_main(ROOT, manifest)
    if head != expected_sha:
        fail(f"exact main SHA mismatch: expected {expected_sha}, found {head}")
    if output(["gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"]) != manifest["repository"]:
        fail("GitHub CLI is not bound to the reviewed public repository")
    immutable = load_json_from_gh([f"repos/{manifest['repository']}/immutable-releases"])
    if immutable.get("enabled") is not True:
        fail("GitHub immutable releases are not enabled")
    existing_release = run(
        ["gh", "api", f"repos/{manifest['repository']}/releases/tags/{current['tag']}"],
        check=False,
    )
    if existing_release.returncode == 0:
        release = json.loads(existing_release.stdout)
        if release.get("draft") is False:
            tag_object = verify_or_publish_tag(manifest, current["tag"], expected_sha)
            assets = release.get("assets", [])
            if len(assets) != 1 or assets[0].get("name") != "release-proof.json":
                fail("published release does not contain the sole reviewed certification asset")
            closure_url = resume_publication_closure(manifest, release, expected_sha, tag_object)
            print(f"immutable release: https://github.com/{manifest['repository']}/releases/tag/{current['tag']}")
            print(f"publication closure pull request: {closure_url}")
            return
    preparation = verify_preparation_pr(manifest, expected_sha)
    run(["make", "ci"], capture=False)
    run(
        [
            "gitleaks",
            "git",
            "--no-banner",
            "--redact",
            f"--log-opts={current['previous_tag']}..{expected_sha}",
            ".",
        ],
        capture=False,
    )
    tag_object = verify_or_publish_tag(manifest, current["tag"], expected_sha)
    run_record = select_release_run(manifest, current["tag"], expected_sha, args.run_id)
    with tempfile.TemporaryDirectory(prefix="dotfiles-release-publication.") as temporary_name:
        temporary = pathlib.Path(temporary_name)
        evidence = archive_release_jobs(manifest, run_record, expected_sha, current["tag"], temporary / "proofs")
        verify_public_clone(manifest, current["tag"], tag_object, expected_sha)
        notes = (ROOT / current["notes"]).read_text(encoding="utf-8")
        body = release_body(notes)
        certification_base = create_certification(
            manifest,
            preparation,
            tag_object,
            expected_sha,
            run_record,
            evidence,
            0,
        )
        draft, _proof_path, asset_sha256 = ensure_draft_release(
            manifest, body, certification_base, temporary
        )
        published = publish_draft(manifest, expected_sha, draft)
        final_proof = certification_base
        final_proof["kind"] = "publication-closure"
        final_proof["release"] = {
            "id": published["id"],
            "published_at": published["published_at"],
            "immutable": True,
            "latest": True,
            "draft": False,
            "prerelease": False,
        }
        closure_url = open_closure_pr(manifest, final_proof, asset_sha256)
    print(f"immutable release: https://github.com/{manifest['repository']}/releases/tag/{current['tag']}")
    print(f"publication closure pull request: {closure_url}")


def load_json_from_gh(arguments: list[str]) -> Any:
    raw = output(["gh", "api", *arguments])
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        fail(f"GitHub returned invalid JSON: {exc}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    check_parser = subparsers.add_parser("check", help="validate the release manifest")
    check_parser.add_argument("--live", action="store_true", help="also compare the published identities with GitHub")
    prepare_parser = subparsers.add_parser("prepare", help="create, verify, push, and open a release-preparation PR")
    prepare_parser.add_argument("--version", required=True)
    prepare_parser.add_argument("--notes", required=True, help="reviewed candidate release-note Markdown")
    publish_parser = subparsers.add_parser("publish", help="certify and publish an exact merged release candidate")
    publish_parser.add_argument("--version", required=True)
    publish_parser.add_argument("--expected-sha", required=True)
    publish_parser.add_argument(
        "--run-id",
        type=int,
        help="resume with one already-observed exact-tag workflow_dispatch run instead of dispatching another",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "check":
            manifest = validate_manifest(ROOT)
            if args.live:
                check_live(manifest)
            print("release manifest and proof identities OK")
        elif args.command == "prepare":
            semver(args.version)
            prepare(args)
        elif args.command == "publish":
            semver(args.version)
            publish(args)
        else:
            fail(f"unsupported command: {args.command}")
    except ReleaseError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
