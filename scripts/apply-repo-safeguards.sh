#!/usr/bin/env bash
# Applies rebase-only merges, branch cleanup, required checks, no required
# reviews for a solo maintainer, enforced admins, linear history, conversation
# resolution, no force pushes, no branch deletions, and best-effort security extras.
set -euo pipefail

usage() {
    cat <<'EOF'
apply-repo-safeguards.sh [owner/repo]

Applies the same repository safeguards declared in .github/settings.yml:
  - rebase-only PR merges
  - delete branches on merge
  - main branch protection with required CI/e2e checks
  - no required reviews for a solo maintainer; checks plus enforce_admins gate merges
  - linear history and conversation resolution
  - no force pushes and no branch deletions
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

try_gh_api() {
    local desc="$1"
    shift
    if ! "$@"; then
        echo "note: could not apply optional safeguard: $desc" >&2
    fi
}

echo "Applying repository safeguards to $repo"

gh_api PATCH "repos/$repo" \
    -F allow_merge_commit=false \
    -F allow_squash_merge=false \
    -F allow_rebase_merge=true \
    -F delete_branch_on_merge=true

gh_api_json PUT "repos/$repo/branches/main/protection" <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "ubuntu",
      "macos",
      "windows",
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

echo "Repository safeguards applied. Re-run safely any time."
