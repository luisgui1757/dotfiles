#!/usr/bin/env bash
# Exercise the production security command; GitHub is the only mocked boundary.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
python3 - "$REPO_ROOT" <<'PY'
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

source = Path(sys.argv[1])
with tempfile.TemporaryDirectory(prefix="security-languages-") as temporary:
    root = Path(temporary)
    repo = root / "repo"
    for name in ("scripts/apply-github-security.sh", ".github/rulesets/main-integrity.json"):
        destination = repo / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source / name, destination)
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    def git(*arguments):
        return subprocess.check_output(["git", "-C", str(repo), *arguments], env=env, text=True).strip()
    git("init", "-q")
    git("config", "user.name", "test")
    git("config", "user.email", "test@example.invalid")
    git("add", ".")
    git("commit", "-qm", "fixture")
    git("branch", "-M", "main")
    git("remote", "add", "origin", "https://github.com/owner/repo.git")
    head = git("rev-parse", "HEAD")
    binary = root / "bin"
    binary.mkdir()
    gh = binary / "gh"
    gh.write_text('''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
args = sys.argv[1:]
if args == ["auth", "status"]:
    sys.exit(0)
if not args or args.pop(0) != "api":
    sys.exit(91)
method = "GET"
if "-X" in args:
    index = args.index("-X")
    method = args[index + 1]
    del args[index:index + 2]
endpoint = args.pop(0)
state = pathlib.Path(os.environ["SECURITY_FIXTURE"])
if method != "GET":
    if method != "PATCH" or endpoint != "repos/owner/repo/code-scanning/default-setup" or "--input" not in args:
        sys.exit(92)
    (state / "patch.json").write_text(sys.stdin.read())
    print("{}")
    sys.exit(0)
responses = json.loads((state / "responses.json").read_text())
if endpoint not in responses:
    sys.exit(93)
data = json.dumps(responses[endpoint])
if "--jq" in args:
    data = subprocess.check_output(["jq", "-r", args[args.index("--jq") + 1]], input=data, text=True)
print(data)
''')
    gh.chmod(0o700)
    env.update(PATH=str(binary) + os.pathsep + env["PATH"], SECURITY_FIXTURE=str(root))
    all_languages = ["actions", "go", "python"]
    def responses(languages, analyses):
        return {
            "repos/owner/repo/commits/main": {"sha": head},
            "repos/owner/repo": {"visibility": "public"},
            "repos/owner/repo/rulesets?includes_parents=false": [{"id": 1, "name": "Protect main: integrity"}],
            "repos/owner/repo/rulesets/1": json.loads((repo / ".github/rulesets/main-integrity.json").read_text()),
            "repos/owner/repo/private-vulnerability-reporting": {"enabled": True},
            "repos/owner/repo/immutable-releases": {"enabled": True},
            "repos/owner/repo/code-scanning/default-setup": {"state": "configured", "query_suite": "default", "languages": languages},
            "repos/owner/repo/code-scanning/analyses?ref=refs/heads/main&tool_name=CodeQL&per_page=100": analyses,
        }
    successful = [{"commit_sha": head, "ref": "refs/heads/main", "error": "", "category": "/language:" + language} for language in all_languages]
    cases = [("all languages verified", all_languages, successful, True),
             ("Go configuration absent", ["actions", "python"], successful, False),
             ("Go analysis absent", all_languages, [a for a in successful if a["category"] != "/language:go"], False)]
    for field, value in (("commit_sha", "0" * 40), ("ref", "refs/pull/1/merge"), ("error", "build failed")):
        changed = [dict(a, **{field: value}) if a["category"] == "/language:go" else a for a in successful]
        cases.append(("Go " + field + " invalid", all_languages, changed, False))
    failures = []
    for name, languages, analyses, expected in cases:
        (root / "responses.json").write_text(json.dumps(responses(languages, analyses)))
        result = subprocess.run(["bash", str(repo / "scripts/apply-github-security.sh"), "--preflight-only", "owner/repo"], env=env, capture_output=True, text=True)
        if (result.returncode == 0) != expected or (root / "patch.json").exists():
            failures.append(name + ": " + result.stdout + result.stderr)
        else:
            print("PASS:", name)
    (root / "responses.json").write_text(json.dumps(responses(["actions", "python"], successful)))
    result = subprocess.run(["bash", str(repo / "scripts/apply-github-security.sh"), "owner/repo"], env=env, capture_output=True, text=True)
    patch = root / "patch.json"
    if result.returncode != 4 or not patch.exists() or json.loads(patch.read_text()) != {"state": "configured", "languages": all_languages, "query_suite": "default"}:
        failures.append("configuration must add Go and stop for exact-main analysis: " + result.stdout + result.stderr)
    else:
        print("PASS: configuration adds Go while preserving Actions/Python and the scan gate")
    if failures:
        for failure in failures:
            print("FAIL:", failure)
        sys.exit(1)
PY
