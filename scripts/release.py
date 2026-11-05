#!/usr/bin/env python3
"""Fail-closed publication of the immutable dotfiles release that main names."""

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
SEMVER = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")
REPOSITORY = "luisgui1757/dotfiles"
OFFICIAL_REMOTE = "https://github.com/luisgui1757/dotfiles.git"
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


def semver(tag: str) -> tuple[int, int, int]:
    match = SEMVER.fullmatch(tag)
    if not match:
        fail(f"release version must be canonical vMAJOR.MINOR.PATCH: {tag}")
    return tuple(int(part) for part in match.groups())  # type: ignore[return-value]


def validate_notes(text: str, tag: str) -> str:
    lines = text.splitlines()
    if not lines or not lines[0].startswith(f"# {tag} — "):
        fail(f"release notes must start with '# {tag} — <reviewed title>'")
    for heading in ("## Highlights", "## Compatibility and upgrade"):
        if text.count(heading) != 1:
            fail(f"release notes must contain exactly one {heading!r} heading")
    if re.search(r"<[A-Za-z][^>]*>", text):
        fail("release notes retain an angle-bracket placeholder")
    return lines[0][2:]


def release_body(notes: str) -> str:
    """The public release body is the reviewed notes from their Highlights section on."""
    return notes[notes.index("## Highlights"):].rstrip() + "\n"


def validate_manifest(root: pathlib.Path = ROOT) -> dict[str, Any]:
    """Return the release that the manifest names: its version, notes and native job matrix."""
    data = load_json(root / MANIFEST_PATH)
    if not isinstance(data, dict) or set(data) != {"schema", "version"} or data["schema"] != 3:
        fail("release manifest must hold only schema 3 and version")
    tag = str(data["version"])
    semver(tag)
    notes = f"docs/releases/{tag}.md"
    if not (root / notes).is_file():
        fail(f"release notes are missing: {notes}")
    return {
        "repository": REPOSITORY,
        "official_remote": OFFICIAL_REMOTE,
        "workflow": "installer-engine.yml",
        "workflow_jobs": native_job_names(root),
        "tag": tag,
        "title": validate_notes((root / notes).read_text(encoding="utf-8"), tag),
        "notes": notes,
    }


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
    expected = {"headSha": commit, "attempt": 1, "status": "completed", "conclusion": "success", "workflowName": "installer engine"}
    if (not isinstance(run_record, dict) or any(run_record.get(key) != val for key, val in expected.items())
            or (run_record.get("event"), run_record.get("headBranch")) not in {("push", "main"), ("workflow_dispatch", tag)}
            or not isinstance(run_record.get("databaseId"), int) or run_record["databaseId"] < 1):
        fail("native job evidence run is not the first successful run of the release commit on main or its tag")
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
    cache_setting = "cache: ${{ github.event_name == 'pull_request' }}"
    if (not blocks or len(blocks) != text.count("uses: actions/setup-go@")
            or any(not re.search(r"(?m)^ +" + re.escape(cache_setting) + r" *$", block) for block in blocks)
            or "actions/cache@" in text):
        fail("native workflow source does not disable every Go cache outside pull requests and every broad Actions cache")


def git_clean(root: pathlib.Path) -> None:
    if output(["git", "status", "--porcelain=v1"], cwd=root):
        fail(f"worktree is not clean: {root}")


def require_tool(name: str) -> None:
    if shutil.which(name) is None:
        fail(f"required release tool is unavailable: {name}")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_merged_pr(manifest: dict[str, Any], commit: str) -> dict[str, Any]:
    """Prove that the release commit is the squash merge of one PR whose head passed every check.

    The workflow token can read each of these REST endpoints.
    """
    repository = manifest["repository"]
    pulls = load_json_from_gh([f"repos/{repository}/commits/{commit}/pulls"])
    matching = [
        item
        for item in pulls
        if item.get("merged_at")
        and item.get("merge_commit_sha") == commit
        and item.get("base", {}).get("ref") == "main"
    ]
    if len(matching) != 1:
        fail("the release commit is not the merge of one pull request into main")
    pull = matching[0]
    head_sha = pull.get("head", {}).get("sha")
    if not HEX40.fullmatch(str(head_sha or "")):
        fail("pull request head identity is invalid")
    canonical_required = load_json(ROOT / ".github/check-identities.json")["required"]
    rules = load_json_from_gh([f"repos/{repository}/rules/branches/main"])
    live_required = [
        check.get("context")
        for rule in rules
        if rule.get("type") == "required_status_checks"
        for check in rule.get("parameters", {}).get("required_status_checks", [])
    ]
    if sorted(live_required) != sorted(canonical_required):
        fail("live required-check policy differs from the reviewed canonical check set")
    checks = load_json_from_gh([f"repos/{repository}/commits/{head_sha}/check-runs?per_page=100"]).get("check_runs", [])
    if not checks or any(
        item.get("status") != "completed" or item.get("conclusion") not in {"success", "neutral", "skipped"}
        for item in checks
    ):
        fail("not every check passed at the pull request head")
    for name in canonical_required:
        matches = [item for item in checks if item.get("name") == name
                   and item.get("head_sha") == head_sha and item.get("conclusion") == "success"
                   and item.get("app", {}).get("id") == 15368]
        if len(matches) != 1:
            fail(f"required check is not uniquely successful at the pull request head: {name}")
    head_commit = load_json_from_gh([f"repos/{repository}/git/commits/{head_sha}"])
    merged_commit = load_json_from_gh([f"repos/{repository}/git/commits/{commit}"])
    head_tree = str(head_commit.get("tree", {}).get("sha", ""))
    merged_tree = str(merged_commit.get("tree", {}).get("sha", ""))
    if not HEX40.fullmatch(head_tree) or not HEX40.fullmatch(merged_tree):
        fail("GitHub returned an invalid pull request or merge tree identity")
    if head_tree != merged_tree:
        fail("squash-merged tree differs from the checked pull request head tree")
    return {
        "number": pull["number"],
        "head": head_sha,
        "tree": merged_tree,
        "checks": sorted(item["name"] for item in checks),
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


def select_release_run(manifest: dict[str, Any], tag: str, expected_sha: str) -> dict[str, Any]:
    """Wait for a first-attempt native run of the release commit and return it after it passes.

    The push run on main comes first. When it does not exist or does not pass,
    an exact-tag run replaces it. A re-run of the release keeps an exact-tag
    run that passed or still runs, and starts a new one only after a failed one.
    """
    def runs(event: str, branch: str) -> list[dict[str, Any]]:
        raw = output(
            [
                "gh",
                "run",
                "list",
                "--workflow",
                manifest["workflow"],
                "--event",
                event,
                "--branch",
                branch,
                "--commit",
                expected_sha,
                "--limit",
                "100",
                "--json",
                "attempt,conclusion,createdAt,databaseId,event,headBranch,headSha,status,url,workflowName",
            ]
        )
        return [item for item in json.loads(raw) if item.get("headBranch") == branch and item.get("headSha") == expected_sha]

    def usable(items: list[dict[str, Any]]) -> list[dict[str, Any]]:
        return [
            item for item in items
            if item.get("attempt") == 1 and (item.get("status") != "completed" or item.get("conclusion") == "success")
        ]

    def passes(run_id: int) -> bool:
        return run(["gh", "run", "watch", str(run_id), "--exit-status", "--interval", "60"], capture=False, check=False).returncode == 0

    pushed = usable(runs("push", "main"))
    if pushed and passes(pushed[0]["databaseId"]):
        return verified_release_run(pushed[0]["databaseId"], "push", "main", expected_sha)
    existing = runs("workflow_dispatch", tag)
    passed_first = sorted(usable(existing), key=lambda item: (item.get("status") != "completed", -item["databaseId"]))
    if passed_first:
        run_id = passed_first[0]["databaseId"]
    else:
        known = {item["databaseId"] for item in existing}
        run(["gh", "workflow", "run", manifest["workflow"], "--ref", tag], capture=False)
        deadline = time.monotonic() + 180
        selected: list[dict[str, Any]] = []
        while time.monotonic() < deadline:
            selected = [item for item in runs("workflow_dispatch", tag) if item["databaseId"] not in known]
            if selected:
                break
            time.sleep(3)
        if len(selected) != 1:
            fail("could not select one new exact-tag workflow_dispatch run")
        run_id = selected[0]["databaseId"]
    if not passes(run_id):
        fail(f"exact-tag run {run_id} did not pass. Re-run the release workflow to start a new exact-tag run.")
    return verified_release_run(run_id, "workflow_dispatch", tag, expected_sha)


def verified_release_run(run_id: int, event: str, branch: str, expected_sha: str) -> dict[str, Any]:
    raw = output(
        [
            "gh",
            "run",
            "view",
            str(run_id),
            "--json",
            "attempt,conclusion,databaseId,event,headBranch,headSha,status,url,workflowName",
        ]
    )
    selected_run = json.loads(raw)
    expected = {
        "databaseId": run_id,
        "event": event,
        "headBranch": branch,
        "headSha": expected_sha,
        "status": "completed",
        "conclusion": "success",
        "workflowName": "installer engine",
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
    pull_request: dict[str, Any],
    tag_object: str,
    commit: str,
    run_record: dict[str, Any],
    job_evidence: dict[str, Any],
) -> dict[str, Any]:
    return {
        "schema": 3,
        "kind": "pre-publication-certification",
        "repository": manifest["repository"],
        "tag": manifest["tag"],
        "previous_tag": manifest["previous_tag"],
        "tag_object": tag_object,
        "commit": commit,
        "tree": pull_request["tree"],
        "pull_request": pull_request,
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
            "release_range": f"{manifest['previous_tag']}..{manifest['tag']}",
            "proof_bytes": job_evidence["size"],
        },
        "fresh_public_clone": "passed",
        "residual_evidence": RESIDUAL_EVIDENCE,
    }


def ensure_draft_release(
    manifest: dict[str, Any], body: str, certification_base: dict[str, Any], temporary: pathlib.Path
) -> tuple[dict[str, Any], pathlib.Path, str]:
    tag = manifest["tag"]
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
                manifest["title"],
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
        or release.get("name") != manifest["title"]
        or release.get("draft") is not True
        or release.get("prerelease") is not False
        or release.get("immutable") is not False
        or str(release.get("body", "")).rstrip() != body.rstrip()
    ):
        fail("draft release does not exactly match the reviewed notes")
    certification = dict(certification_base)
    certification["release"] = {
        "id": release["id"],
        "draft": True,
        "immutable": False,
        "publication": "pending",
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


def publish_draft(manifest: dict[str, Any], draft: dict[str, Any]) -> dict[str, Any]:
    # The merge that set the version is the approval. The release workflow
    # publishes only after every gate before this call passed.
    tag = manifest["tag"]
    print("\nEvery deterministic publication gate passed. Publishing the immutable release.")
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


def previous_release(manifest: dict[str, Any]) -> str | None:
    """Return the latest published tag that the new release follows, or None when the version is out."""
    tag = manifest["tag"]
    # This endpoint returns only published releases, never a draft.
    existing = run(["gh", "api", f"repos/{manifest['repository']}/releases/tags/{tag}"], check=False)
    if existing.returncode == 0:
        if json.loads(existing.stdout).get("immutable") is not True:
            fail(f"{tag} is published but not immutable")
        return None
    latest = load_json_from_gh([f"repos/{manifest['repository']}/releases/latest"])
    previous = str(latest.get("tag_name", ""))
    if latest.get("immutable") is not True:
        fail(f"the latest release {previous} is not immutable")
    if semver(tag) <= semver(previous):
        fail(f"{tag} is not newer than the latest release {previous}")
    return previous


def publish(args: argparse.Namespace) -> None:
    for tool in ("gh", "git", "gitleaks", "python3"):
        require_tool(tool)
    commit = args.commit
    if not HEX40.fullmatch(commit):
        fail("--commit must be a full lowercase commit identity")
    git_clean(ROOT)
    if output(["git", "rev-parse", "HEAD^{commit}"]) != commit:
        fail("the checkout is not the release commit")
    manifest = validate_manifest(ROOT)
    tag = manifest["tag"]
    parent = json.loads(output(["git", "show", f"{commit}^:{MANIFEST_PATH.as_posix()}"]))
    if parent.get("version") == tag:
        print(f"This merge did not change the version {tag}. There is nothing to release.")
        return
    if output(["gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"]) != manifest["repository"]:
        fail("GitHub CLI is not bound to the reviewed public repository")
    previous = previous_release(manifest)
    if previous is None:
        print(f"{tag} is already published. There is nothing to release.")
        return
    manifest["previous_tag"] = previous
    pull_request = verify_merged_pr(manifest, commit)
    run(["gitleaks", "git", "--no-banner", "--redact", f"--log-opts={previous}..{commit}", "."], capture=False)
    tag_object = verify_or_publish_tag(manifest, tag, commit)
    run_record = select_release_run(manifest, tag, commit)
    with tempfile.TemporaryDirectory(prefix="dotfiles-release-publication.") as temporary_name:
        temporary = pathlib.Path(temporary_name)
        evidence = archive_release_jobs(manifest, run_record, commit, tag, temporary / "proofs")
        verify_public_clone(manifest, tag, tag_object, commit)
        body = release_body((ROOT / manifest["notes"]).read_text(encoding="utf-8"))
        certification = create_certification(manifest, pull_request, tag_object, commit, run_record, evidence)
        draft, _proof_path, _asset_sha256 = ensure_draft_release(manifest, body, certification, temporary)
        publish_draft(manifest, draft)
    print(f"immutable release: https://github.com/{manifest['repository']}/releases/tag/{tag}")


def load_json_from_gh(arguments: list[str]) -> Any:
    raw = output(["gh", "api", *arguments])
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        fail(f"GitHub returned invalid JSON: {exc}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("check", help="validate the release manifest and its notes")
    publish_parser = subparsers.add_parser(
        "publish", help="certify and publish the manifest version at its merge commit (the release workflow runs this)"
    )
    publish_parser.add_argument("--commit", required=True, help="the merge commit that set the version")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "check":
            validate_manifest(ROOT)
            print("release manifest and notes OK")
        elif args.command == "publish":
            publish(args)
        else:
            fail(f"unsupported command: {args.command}")
    except ReleaseError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
