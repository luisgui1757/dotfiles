#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

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
for required in ("pull_request", "required_status_checks", "required_linear_history", "deletion", "non_fast_forward"):
    if required not in integrity_rules:
        fail(f"integrity ruleset is missing {required}")
if "required_status_checks" in rule_types(review):
    fail("review ruleset must not contain required_status_checks")
if rule_types(owner_updates) != {"update"}:
    fail("owner-updates ruleset must contain only the update rule")

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
    "required_pull_request_reviews: null",
):
    if snippet not in settings:
        fail(f".github/settings.yml missing {snippet}")

for workflow in pathlib.Path(".github/workflows").glob("*.yml"):
    text = workflow.read_text(encoding="utf-8")
    if "pull_request_target:" in text:
        fail(f"{workflow} must not use pull_request_target")
    if "permissions:\n  contents: read" not in text:
        fail(f"{workflow} must declare read-only contents permission")

script = pathlib.Path("scripts/apply-repo-safeguards.sh")
mode = os.stat(script).st_mode
if not (mode & stat.S_IXUSR):
    fail("scripts/apply-repo-safeguards.sh must be executable")
script_text = script.read_text(encoding="utf-8")
for snippet in (
    "-F allow_squash_merge=true",
    "-F allow_rebase_merge=false",
    "-F allow_auto_merge=false",
    'upsert_ruleset "Protect main: integrity"',
    'upsert_ruleset "Protect main: review"',
    'upsert_ruleset "Protect main: owner updates"',
    'require_live_value "integrity bypass actor count" "$integrity_bypass_count" "0"',
    'require_live_value "review bypass actor" "$review_bypass" "User:139752288:pull_request"',
    'require_live_value "owner-updates bypass actor" "$owner_updates_bypass" "User:139752288:pull_request"',
):
    if snippet not in script_text:
        fail(f"apply-repo-safeguards.sh missing {snippet}")

install_deps = pathlib.Path("install-deps.ps1").read_text(encoding="utf-8")
if "function Add-ScoopBucketSafe" not in install_deps:
    fail("install-deps.ps1 must define Add-ScoopBucketSafe")
in_scoop_bucket_helper = False
for i, line in enumerate(install_deps.splitlines(), start=1):
    stripped = line.strip()
    if stripped == "function Add-ScoopBucketSafe {":
        in_scoop_bucket_helper = True
    elif stripped == "function Ensure-ScoopBuckets {":
        in_scoop_bucket_helper = False
    if (stripped.startswith("scoop bucket add ") or "| scoop bucket add " in stripped) and not in_scoop_bucket_helper:
        fail(f"install-deps.ps1:{i} uses a bare 'scoop bucket add'; route it through Add-ScoopBucketSafe")

chezmoi_wave_a_path = pathlib.Path("docs/archive/CHEZMOI_WAVE_A_SPEC.md")
chezmoi_wave_a = chezmoi_wave_a_path.read_text(encoding="utf-8")
if not re.search(r"psmux install was removed\s+from\s+the chezmoi scope", chezmoi_wave_a):
    fail(f"{chezmoi_wave_a_path} must document psmux as install-deps scope, not a chezmoi run-script")
removed_psmux_script = "run_once_after_10" + "-install-psmux"
if removed_psmux_script in chezmoi_wave_a:
    fail(f"{chezmoi_wave_a_path} must not reference the removed psmux chezmoi run-script")
if re.search(r"(?m)^\s*scoop bucket add psmux\b.*2>\$null", chezmoi_wave_a):
    fail(f"{chezmoi_wave_a_path} must not contain the old bare 'scoop bucket add psmux ... 2>$null'")

if failures:
    for message in failures:
        print(f"FAIL: {message}")
    sys.exit(1)

print("OK")
PY
