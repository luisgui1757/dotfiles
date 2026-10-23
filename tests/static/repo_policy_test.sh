#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

command -v ruby >/dev/null 2>&1 || {
    echo "FAIL: ruby is required for semantic .github/settings.yml policy checks" >&2
    exit 1
}
ruby tests/static/assert_no_probot_branches.rb .github/settings.yml

python3 - <<'PY'
import json
import os
import pathlib
import re
import stat
import sys

root = pathlib.Path(".")
failures = []


def fail(message):
    failures.append(message)


def load_json(path):
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def rule_types(ruleset):
    return {rule["type"] for rule in ruleset["rules"]}


def pull_request_rule(ruleset):
    for rule in ruleset["rules"]:
        if rule["type"] == "pull_request":
            return rule["parameters"]
    fail(f"{ruleset['name']} is missing a pull_request rule")
    return {}


def external_action_ref(line):
    match = re.search(r"\buses:\s*([^@\s]+)@([^\s#]+)", line)
    if not match or match.group(1).startswith("./"):
        return None
    return match.group(1), match.group(2)


for probe, should_fail in (
    ("uses: actions/checkout@v4", True),
    ("uses: owner/action@0123456789abcdef0123456789abcdef01234567 # v1", False),
    ("uses: ./local-action", False),
):
    parsed = external_action_ref(probe)
    invalid = bool(parsed and not re.fullmatch(r"[0-9a-f]{40}", parsed[1]))
    if invalid != should_fail:
        fail(f"external Actions SHA scanner self-test failed for: {probe}")


integrity = load_json(".github/rulesets/main-integrity.json")
review = load_json(".github/rulesets/main-review.json")
owner_updates = load_json(".github/rulesets/main-owner-updates.json")

for ruleset in (integrity, review, owner_updates):
    if ruleset.get("target") != "branch":
        fail(f"{ruleset['name']} must target branches")
    if ruleset.get("enforcement") != "active":
        fail(f"{ruleset['name']} must be active")
    refs = ruleset.get("conditions", {}).get("ref_name", {}).get("include", [])
    if refs != ["refs/heads/main"]:
        fail(f"{ruleset['name']} must target only refs/heads/main")

if integrity.get("bypass_actors") != []:
    fail("integrity ruleset must have no bypass actors")

review_bypass = review.get("bypass_actors", [])
expected_bypass = [
    {"actor_id": 139752288, "actor_type": "User", "bypass_mode": "pull_request"}
]
if review_bypass != expected_bypass:
    fail("review ruleset must have only the owner pull_request bypass")
if owner_updates.get("bypass_actors", []) != expected_bypass:
    fail("owner-updates ruleset must have only the owner pull_request bypass")

integrity_rules = rule_types(integrity)
for required in ("pull_request", "required_status_checks", "code_scanning", "required_linear_history", "deletion", "non_fast_forward"):
    if required not in integrity_rules:
        fail(f"integrity ruleset is missing {required}")
if "required_status_checks" in rule_types(review):
    fail("review ruleset must not contain required_status_checks")
if rule_types(owner_updates) != {"update"}:
    fail("owner-updates ruleset must contain only the update rule")

code_scanning_rules = [rule for rule in integrity["rules"] if rule["type"] == "code_scanning"]
expected_code_scanning_tools = [
    {
        "tool": "CodeQL",
        "alerts_threshold": "errors",
        "security_alerts_threshold": "high_or_higher",
    }
]
if len(code_scanning_rules) != 1:
    fail("integrity ruleset must contain exactly one code_scanning rule")
elif code_scanning_rules[0].get("parameters", {}).get("code_scanning_tools") != expected_code_scanning_tools:
    fail("integrity CodeQL rule must block errors and high-or-higher security alerts")

integrity_pr = pull_request_rule(integrity)
review_pr = pull_request_rule(review)
if integrity_pr.get("required_approving_review_count") != 0:
    fail("integrity ruleset must require PRs without review approvals")
if integrity_pr.get("allowed_merge_methods") != ["squash"]:
    fail("integrity ruleset must enforce squash-only merges")

review_expectations = {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews_on_push": True,
    "require_code_owner_review": True,
    "require_last_push_approval": True,
    "required_review_thread_resolution": True,
}
for key, expected in review_expectations.items():
    if review_pr.get(key) != expected:
        fail(f"review ruleset {key} must be {expected}")
if review_pr.get("allowed_merge_methods") != ["squash"]:
    fail("review ruleset must stay squash-only")

owner_update_rule = owner_updates["rules"][0]
if owner_update_rule != {"type": "update"}:
    fail("owner-updates ruleset must use GitHub's canonical update rule without fetch-and-merge parameters")

renovate = load_json("renovate.json")
if renovate.get("automerge") is not False:
    fail("renovate.json top-level automerge must stay false")
for idx, rule in enumerate(renovate.get("packageRules", []), start=1):
    if rule.get("automerge") is not False:
        fail(f"renovate package rule {idx} must not enable automerge")

settings = pathlib.Path(".github/settings.yml").read_text(encoding="utf-8")
for snippet in (
    "allow_merge_commit: false",
    "allow_squash_merge: true",
    "allow_rebase_merge: false",
    "delete_branch_on_merge: true",
    "Branch protection is deliberately absent",
):
    if snippet not in settings:
        fail(f".github/settings.yml missing {snippet}")
for workflow in pathlib.Path(".github/workflows").glob("*.yml"):
    text = workflow.read_text(encoding="utf-8")
    if "pull_request_target:" in text:
        fail(f"{workflow} must not use pull_request_target")
    if "permissions:\n  contents: read" not in text:
        fail(f"{workflow} must declare read-only contents permission")
    for line_number, line in enumerate(text.splitlines(), start=1):
        parsed = external_action_ref(line)
        if parsed and not re.fullmatch(r"[0-9a-f]{40}", parsed[1]):
            fail(f"{workflow}:{line_number} external action {parsed[0]} must use a full lowercase 40-hex commit SHA")

makefile = pathlib.Path("Makefile").read_text(encoding="utf-8")
for target in ("test-bootstrap", "test-installer"):
    if target not in makefile.split("ci:",1)[1].splitlines()[0]:
        fail(f"canonical ci gate is missing {target}")

script = pathlib.Path("scripts/apply-repo-safeguards.sh")
mode = os.stat(script).st_mode
if not (mode & stat.S_IXUSR):
    fail("scripts/apply-repo-safeguards.sh must be executable")
script_text = script.read_text(encoding="utf-8")
for snippet in (
    "github_actions_app_id=15368",
    "verify_local_boundary",
    "verify_snapshot_unchanged",
    "restore_snapshot()",
    '.private == false',
    '.visibility == "public"',
    "classic-live.json",
    'frozen="$(mktemp -d)"',
    "prepare_transaction_payloads",
    '"$frozen/integrity-restore.json"',
    '"$frozen/classic-restore.json"',
    '"$frozen/actions-restore.json"',
    "policy does not match manifest stage",
    "transaction_active=1",
    'select_workflow_run "$capture_dir" test.yml .github/workflows/test.yml',
    'select_workflow_run "$capture_dir" installer-engine.yml .github/workflows/installer-engine.yml',
    'gh_api_json_file PUT "repos/$repo/actions/permissions" "$transaction_dir/actions-desired.json"',
    'gh_api_json_file PUT "repos/$repo/rulesets/',
    '"$transaction_dir/integrity-desired.json"',
    'gh_api_json_file PATCH "repos/$repo/branches/main/protection/required_status_checks" "$transaction_dir/classic-desired.json"',
    "RECOVERY REQUIRED:",
):
    if snippet not in script_text:
        fail(f"apply-repo-safeguards.sh missing {snippet}")
for forbidden in (
    "upsert_ruleset",
    "try_gh_api",
    'gh_api PATCH "repos/$repo"',
    'gh_api_json_file PUT "repos/$repo/branches/main/protection"',
    'gh_api_json_file PUT "repos/$repo/rulesets/$integrity_id" "$snapshot/',
    'gh_api_json_file PATCH "repos/$repo/branches/main/protection/required_status_checks" "$snapshot/',
    'gh_api_json_file PUT "repos/$repo/actions/permissions" "$snapshot/',
    'gh_api_json_file PUT "repos/$repo/rulesets/$(jq -r .integrity_ruleset_id "$preflight_dir/manifest.json")" "$integrity_ruleset"',
):
    if forbidden in script_text:
        fail(f"apply-repo-safeguards.sh retains unsafe broad mutation path: {forbidden}")

if failures:
    for message in failures:
        print(f"FAIL: {message}")
    sys.exit(1)

print("OK")
PY
