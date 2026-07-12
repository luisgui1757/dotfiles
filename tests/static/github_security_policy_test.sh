#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
import json
import os
import pathlib
import stat
import sys

failures = []


def fail(message):
    failures.append(message)


script_path = pathlib.Path("scripts/apply-github-security.sh")
script = script_path.read_text(encoding="utf-8")
if not os.stat(script_path).st_mode & stat.S_IXUSR:
    fail("apply-github-security.sh must be executable")

required_script_contract = (
    'repos/$repo/private-vulnerability-reporting',
    'repos/$repo/immutable-releases',
    'repos/$repo/code-scanning/default-setup',
    '"languages": ["actions", "python"]',
    '"query_suite": "default"',
    '"/language:actions"',
    '"/language:python"',
    'ruleset_stage="preapply"',
    'ruleset_stage="applied"',
    'restore_ruleset()',
    'transaction_active=1',
    'RECOVERY REQUIRED:',
)
for snippet in required_script_contract:
    if snippet not in script:
        fail(f"apply-github-security.sh missing {snippet}")

for forbidden in (
    "pull_request_target:",
    '"languages": ["c-cpp"',
    '"query_suite": "extended"',
):
    if forbidden in script:
        fail(f"apply-github-security.sh contains rejected policy: {forbidden}")

with open(".github/rulesets/main-integrity.json", encoding="utf-8") as handle:
    integrity = json.load(handle)
code_rules = [rule for rule in integrity["rules"] if rule["type"] == "code_scanning"]
expected_tools = [{
    "tool": "CodeQL",
    "alerts_threshold": "errors",
    "security_alerts_threshold": "high_or_higher",
}]
if len(code_rules) != 1:
    fail("main-integrity must have exactly one code_scanning rule")
elif code_rules[0].get("parameters", {}).get("code_scanning_tools") != expected_tools:
    fail("main-integrity CodeQL thresholds drifted")

security_policy = pathlib.Path("SECURITY.md").read_text(encoding="utf-8")
for snippet in (
    "Report a vulnerability",
    "Do not open a public issue",
    "No response or remediation timeline is guaranteed",
):
    if snippet not in security_policy:
        fail(f"SECURITY.md missing {snippet}")

for document in ("README.md", "CLAUDE.md", "docs/security/branch-protection.md"):
    text = pathlib.Path(document).read_text(encoding="utf-8")
    for snippet in ("CodeQL", "private vulnerability reporting", "immutable releases"):
        if snippet not in text:
            fail(f"{document} missing {snippet}")

if failures:
    for message in failures:
        print(f"FAIL: {message}")
    sys.exit(1)

print("OK")
PY
