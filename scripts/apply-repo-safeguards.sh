#!/usr/bin/env bash
# Applies the public-repo safeguards documented in docs/security/branch-protection.md.
set -euo pipefail

usage() {
    cat <<'EOF'
apply-repo-safeguards.sh [owner/repo]

Applies the repository safeguard posture:
  - squash-only PR merges
  - delete branches on merge
  - auto-merge disabled
  - three active main-branch rulesets:
      * Protect main: integrity (no bypass; required PR, strict CI, no delete/force)
      * Protect main: review (owner-only pull-request bypass for review rules)
      * Protect main: owner updates (only owner can update main through PRs)
  - classic main branch protection fallback with required CI checks
  - best-effort GitHub security extras where the plan supports them

Requires an authenticated GitHub CLI with repository admin permission:
  gh auth login
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
fi

if ! command -v gh >/dev/null 2>&1; then
    echo "FAIL: gh is required. Install GitHub CLI and run gh auth login." >&2
    exit 1
fi

gh auth status >/dev/null

repo="${1:-${GITHUB_REPOSITORY:-}}"
if [[ -z "$repo" ]]; then
    repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi
if [[ "$repo" != */* ]]; then
    echo "FAIL: repository must be owner/repo, got: $repo" >&2
    exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
integrity_ruleset="$repo_root/.github/rulesets/main-integrity.json"
review_ruleset="$repo_root/.github/rulesets/main-review.json"
owner_updates_ruleset="$repo_root/.github/rulesets/main-owner-updates.json"

for f in "$integrity_ruleset" "$review_ruleset" "$owner_updates_ruleset"; do
    if [[ ! -f "$f" ]]; then
        echo "FAIL: missing ruleset file: $f" >&2
        exit 3
    fi
done

gh_api() {
    local method="$1" path="$2"
    shift 2
    echo "+ gh api -X $method $path $*"
    gh api -X "$method" "$path" "$@"
}

gh_api_json() {
    local method="$1" path="$2"
    echo "+ gh api -X $method $path --input -"
    gh api -X "$method" "$path" --input -
}

gh_api_json_file() {
    local method="$1" path="$2" file="$3"
    echo "+ gh api -X $method $path --input $file"
    gh api -X "$method" "$path" --input "$file"
}

try_gh_api() {
    local desc="$1"
    shift
    if ! "$@"; then
        echo "note: could not apply optional safeguard: $desc" >&2
    fi
}

ruleset_id_by_name() {
    local name="$1" ids count
    ids="$(gh api "repos/$repo/rulesets?includes_parents=false" \
        --jq ".[] | select(.name == \"$name\") | .id")"
    count="$(awk 'NF { count++ } END { print count + 0 }' <<<"$ids")"
    if [[ "$count" -gt 1 ]]; then
        echo "FAIL: found $count rulesets named '$name'; delete duplicates before applying safeguards." >&2
        return 1
    fi
    printf '%s\n' "$ids"
}

upsert_ruleset() {
    local name="$1" file="$2" id
    id="$(ruleset_id_by_name "$name")"
    if [[ -n "$id" ]]; then
        gh_api_json_file PUT "repos/$repo/rulesets/$id" "$file" >/dev/null
    else
        gh_api_json_file POST "repos/$repo/rulesets" "$file" >/dev/null
    fi
}

require_live_value() {
    local desc="$1" actual="$2" expected="$3"
    if [[ "$actual" != "$expected" ]]; then
        echo "FAIL: $desc is '$actual', expected '$expected'" >&2
        exit 4
    fi
}

required_check_contexts() {
    cat <<'EOF'
ubuntu
macos
windows
chezmoi-parity
chezmoi-parity-macos
chezmoi-parity-windows
nix flake check (ubuntu-24.04)
nix flake check (macos-26)
e2e containers / ubuntu-24.04
setup.sh / ubuntu-24.04
setup.sh / macos-15
setup.ps1 / windows-2025
EOF
}

verify_required_contexts() {
    local desc="$1" actual="$2" tmp_expected tmp_actual
    tmp_expected="$(mktemp)"
    tmp_actual="$(mktemp)"
    required_check_contexts | LC_ALL=C sort > "$tmp_expected"
    printf '%s\n' "$actual" | LC_ALL=C sort > "$tmp_actual"
    if ! diff -u "$tmp_expected" "$tmp_actual"; then
        rm -f "$tmp_expected" "$tmp_actual"
        echo "FAIL: $desc required status checks differ from the canonical list" >&2
        exit 4
    fi
    rm -f "$tmp_expected" "$tmp_actual"
}

echo "Applying repository safeguards to $repo"

gh_api PATCH "repos/$repo" \
    -F allow_merge_commit=false \
    -F allow_squash_merge=true \
    -F allow_rebase_merge=false \
    -F allow_auto_merge=false \
    -F delete_branch_on_merge=true >/dev/null

upsert_ruleset "Protect main: integrity" "$integrity_ruleset"
upsert_ruleset "Protect main: review" "$review_ruleset"
upsert_ruleset "Protect main: owner updates" "$owner_updates_ruleset"

gh_api_json PUT "repos/$repo/branches/main/protection" <<'JSON' >/dev/null
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "ubuntu",
      "macos",
      "windows",
      "chezmoi-parity",
      "chezmoi-parity-macos",
      "chezmoi-parity-windows",
      "nix flake check (ubuntu-24.04)",
      "nix flake check (macos-26)",
      "e2e containers / ubuntu-24.04",
      "setup.sh / ubuntu-24.04",
      "setup.sh / macos-15",
      "setup.ps1 / windows-2025"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON

try_gh_api "vulnerability alerts" \
    gh_api PUT "repos/$repo/vulnerability-alerts" --silent
try_gh_api "automated security fixes" \
    gh_api PUT "repos/$repo/automated-security-fixes" --silent
try_gh_api "secret scanning and push protection" \
    gh_api_json PATCH "repos/$repo" <<'JSON'
{
  "security_and_analysis": {
    "secret_scanning": {
      "status": "enabled"
    },
    "secret_scanning_push_protection": {
      "status": "enabled"
    }
  }
}
JSON

repo_settings="$(gh api "repos/$repo" \
    --jq '[.allow_merge_commit, .allow_squash_merge, .allow_rebase_merge, .allow_auto_merge, .delete_branch_on_merge] | @tsv')"
IFS=$'\t' read -r merge_allowed squash_allowed rebase_allowed auto_merge_allowed delete_branch_on_merge <<<"$repo_settings"
require_live_value "allow_merge_commit" "$merge_allowed" "false"
require_live_value "allow_squash_merge" "$squash_allowed" "true"
require_live_value "allow_rebase_merge" "$rebase_allowed" "false"
require_live_value "allow_auto_merge" "$auto_merge_allowed" "false"
require_live_value "delete_branch_on_merge" "$delete_branch_on_merge" "true"

integrity_id="$(ruleset_id_by_name "Protect main: integrity")"
review_id="$(ruleset_id_by_name "Protect main: review")"
owner_updates_id="$(ruleset_id_by_name "Protect main: owner updates")"
if [[ -z "$integrity_id" || -z "$review_id" || -z "$owner_updates_id" ]]; then
    echo "FAIL: expected rulesets were not found after apply" >&2
    exit 5
fi

integrity_bypass_count="$(gh api "repos/$repo/rulesets/$integrity_id" --jq '.bypass_actors | length')"
require_live_value "integrity bypass actor count" "$integrity_bypass_count" "0"

integrity_strict="$(gh api "repos/$repo/rulesets/$integrity_id" \
    --jq '.rules[] | select(.type == "required_status_checks") | .parameters.strict_required_status_checks_policy')"
require_live_value "integrity strict required-status policy" "$integrity_strict" "true"

integrity_required_contexts="$(gh api "repos/$repo/rulesets/$integrity_id" \
    --jq '.rules[] | select(.type == "required_status_checks") | .parameters.required_status_checks[].context')"
verify_required_contexts "integrity ruleset" "$integrity_required_contexts"

review_bypass="$(gh api "repos/$repo/rulesets/$review_id" \
    --jq '.bypass_actors[] | "\(.actor_type):\(.actor_id):\(.bypass_mode)"')"
require_live_value "review bypass actor" "$review_bypass" "User:139752288:pull_request"

owner_updates_bypass="$(gh api "repos/$repo/rulesets/$owner_updates_id" \
    --jq '.bypass_actors[] | "\(.actor_type):\(.actor_id):\(.bypass_mode)"')"
require_live_value "owner-updates bypass actor" "$owner_updates_bypass" "User:139752288:pull_request"

owner_updates_rule="$(gh api "repos/$repo/rulesets/$owner_updates_id" \
    --jq '.rules[] | select(.type == "update") | (.parameters.update_allows_fetch_and_merge // false)')"
require_live_value "owner-updates fetch-and-merge exception" "$owner_updates_rule" "false"

classic_strict="$(gh api "repos/$repo/branches/main/protection/required_status_checks" --jq '.strict')"
require_live_value "classic branch-protection strict required-status policy" "$classic_strict" "true"

classic_required_contexts="$(gh api "repos/$repo/branches/main/protection/required_status_checks" --jq '.contexts[]')"
verify_required_contexts "classic branch-protection fallback" "$classic_required_contexts"

echo "Repository safeguards applied and verified. Re-run safely any time."
