#!/usr/bin/env bash
# Applies the GitHub-native security posture documented in
# docs/security/branch-protection.md. CodeQL must prove the exact live main SHA
# before the non-bypassable merge rule is published.
set -euo pipefail

usage() {
    cat <<'EOF'
apply-github-security.sh [--preflight-only] [owner/repo]
apply-github-security.sh --restore <snapshot-directory> [owner/repo]

Ensures:
  - private vulnerability reporting
  - immutable releases
  - CodeQL default setup for GitHub Actions and Python with default queries
  - successful Actions and Python analyses for the exact live main SHA
  - non-bypassable CodeQL merge protection in Protect main: integrity

The merge-protection mutation is snapshotted and rolled back automatically if
readback fails. If CodeQL must be enabled or reconfigured, the command stops
after requesting the initial scan; rerun it after that scan succeeds.
EOF
}

preflight_only=0
restore_dir=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --preflight-only)
            [[ -z "$restore_dir" ]] || {
                echo "FAIL: --preflight-only and --restore are mutually exclusive" >&2
                exit 2
            }
            preflight_only=1
            shift
            ;;
        --restore)
            [[ "$preflight_only" -eq 0 && -z "$restore_dir" && "$#" -ge 2 ]] || {
                echo "FAIL: --restore requires one snapshot directory" >&2
                exit 2
            }
            restore_dir="$2"
            shift 2
            ;;
        --*)
            echo "FAIL: unknown option: $1" >&2
            exit 2
            ;;
        *)
            break
            ;;
    esac
done

[[ "$#" -le 1 ]] || { echo "FAIL: expected at most one owner/repo argument" >&2; exit 2; }
for command_name in gh git jq; do
    command -v "$command_name" >/dev/null 2>&1 || {
        echo "FAIL: $command_name is required." >&2
        exit 1
    }
done
gh auth status >/dev/null

repo="${1:-${GITHUB_REPOSITORY:-}}"
if [[ -z "$repo" ]]; then
    repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi
if [[ ! "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "FAIL: repository must be a literal owner/repo" >&2
    exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
integrity_ruleset="$repo_root/.github/rulesets/main-integrity.json"
transaction_active=0
recovery_dir=""
work_dir=""

gh_api_json_file() {
    local method="$1" path="$2" file="$3"
    echo "+ gh api -X $method $path --input $file"
    gh api -X "$method" "$path" --input "$file"
}

canonicalize_json() {
    jq -S 'walk(if type == "array" then sort_by(tojson) else . end)' "$1"
}

json_equal() {
    local left="$1" right="$2" left_normalized right_normalized rc
    left_normalized="$(mktemp)"
    right_normalized="$(mktemp)"
    canonicalize_json "$left" > "$left_normalized"
    canonicalize_json "$right" > "$right_normalized"
    if cmp -s "$left_normalized" "$right_normalized"; then rc=0; else rc=1; fi
    rm -f "$left_normalized" "$right_normalized"
    return "$rc"
}

ruleset_payload() {
    jq '{name, target, enforcement, bypass_actors, conditions, rules}
        | del(.rules[].parameters.required_reviewers?)' "$1"
}

ruleset_id_by_name() {
    local name="$1" ids count
    ids="$(gh api "repos/$repo/rulesets?includes_parents=false" \
        --jq ".[] | select(.name == \"$name\") | .id")"
    count="$(awk 'NF { count++ } END { print count + 0 }' <<<"$ids")"
    if [[ "$count" -ne 1 ]]; then
        echo "FAIL: found $count rulesets named '$name'; expected exactly one" >&2
        return 1
    fi
    printf '%s\n' "$ids"
}

normalize_remote_repo() {
    local remote="$1" normalized
    case "$remote" in
        https://github.com/*) normalized="${remote#https://github.com/}" ;;
        git@github.com:*) normalized="${remote#git@github.com:}" ;;
        ssh://git@github.com/*) normalized="${remote#ssh://git@github.com/}" ;;
        *) return 1 ;;
    esac
    [[ "${normalized%.git}" == "$repo" ]]
}

verify_local_boundary() {
    local live_main_sha="$1" branch head main_ref origin_url dirty
    branch="$(git -C "$repo_root" symbolic-ref -q --short HEAD || true)"
    [[ "$branch" == "main" ]] || {
        echo "FAIL: apply requires the checked-out local branch to be main" >&2
        return 1
    }
    origin_url="$(git -C "$repo_root" remote get-url origin 2>/dev/null || true)"
    normalize_remote_repo "$origin_url" || {
        echo "FAIL: checkout origin is not github.com/$repo" >&2
        return 1
    }
    head="$(git -C "$repo_root" rev-parse HEAD)"
    main_ref="$(git -C "$repo_root" rev-parse refs/heads/main)"
    [[ "$head" == "$main_ref" && "$head" == "$live_main_sha" ]] || {
        echo "FAIL: local main/HEAD must equal exact live main $live_main_sha" >&2
        return 1
    }
    dirty="$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all -- \
        .github/rulesets/main-integrity.json \
        scripts/apply-github-security.sh \
        SECURITY.md \
        docs/security/branch-protection.md)"
    [[ -z "$dirty" ]] || {
        echo "FAIL: reviewed security sources differ from exact live main" >&2
        printf '%s\n' "$dirty" >&2
        return 1
    }
}

codeql_config_is_desired() {
    jq -e '
      .state == "configured"
      and .query_suite == "default"
      and (.languages | sort) == ["actions", "python"]
    ' "$1" >/dev/null
}

verify_codeql_analyses() {
    local analyses="$1" live_main_sha="$2"
    jq -e --arg sha "$live_main_sha" '
      [.[] | select(
        .commit_sha == $sha
        and .ref == "refs/heads/main"
        and .error == ""
        and (.category == "/language:actions" or .category == "/language:python")
      ) | .category] | unique | sort == ["/language:actions", "/language:python"]
    ' "$analyses" >/dev/null
}

restore_ruleset() (
    local snapshot="$1" frozen live_main_sha integrity_id verify_dir="" live_id
    frozen="$(mktemp -d)"
    chmod 700 "$frozen"
    trap '
        chmod -R u+w "$frozen" 2>/dev/null || true
        rm -rf "$frozen"
        if [[ -n "$verify_dir" && -d "$verify_dir" ]]; then
            rm -rf "$verify_dir"
        fi
    ' EXIT
    for file in manifest.json integrity-restore.json; do
        [[ -f "$snapshot/$file" && ! -L "$snapshot/$file" ]] || {
            echo "FAIL: recovery snapshot has a missing or unsafe $file: $snapshot" >&2
            return 1
        }
        cp "$snapshot/$file" "$frozen/$file"
    done
    chmod 400 "$frozen"/*

    jq -e --arg repository "$repo" '
      type == "object"
      and (keys | sort) == (["integrity_ruleset_id", "live_main_sha", "repository", "schema"] | sort)
      and .schema == 1
      and .repository == $repository
      and (.live_main_sha | test("^[0-9a-f]{40}$"))
      and (.integrity_ruleset_id | type == "number" and . > 0 and floor == .)
    ' "$frozen/manifest.json" >/dev/null || {
        echo "FAIL: recovery manifest is invalid or belongs to another repository" >&2
        return 1
    }
    jq -e '
      type == "object"
      and (keys | sort) == (["bypass_actors", "conditions", "enforcement", "name", "rules", "target"] | sort)
      and .name == "Protect main: integrity"
      and .target == "branch"
      and .enforcement == "active"
      and .bypass_actors == []
      and .conditions.ref_name.include == ["refs/heads/main"]
      and (.rules | type == "array" and length > 0)
    ' "$frozen/integrity-restore.json" >/dev/null || {
        echo "FAIL: recovery integrity payload is malformed" >&2
        return 1
    }

    live_main_sha="$(jq -r .live_main_sha "$frozen/manifest.json")"
    integrity_id="$(jq -r .integrity_ruleset_id "$frozen/manifest.json")"
    [[ "$(gh api "repos/$repo/commits/main" --jq .sha)" == "$live_main_sha" ]] || {
        echo "FAIL: recovery snapshot does not match current live main" >&2
        return 1
    }
    live_id="$(ruleset_id_by_name "Protect main: integrity")"
    [[ "$live_id" == "$integrity_id" ]] || {
        echo "FAIL: recovery ruleset ID does not match the unique live integrity ruleset" >&2
        return 1
    }
    gh_api_json_file PUT "repos/$repo/rulesets/$integrity_id" \
        "$frozen/integrity-restore.json" >/dev/null
    verify_dir="$(mktemp -d)"
    gh api "repos/$repo/rulesets/$integrity_id" > "$verify_dir/live.json"
    ruleset_payload "$verify_dir/live.json" > "$verify_dir/live-payload.json"
    if ! json_equal "$verify_dir/live-payload.json" "$frozen/integrity-restore.json"; then
        rm -rf "$verify_dir"
        echo "FAIL: integrity ruleset restore readback differs from snapshot" >&2
        return 1
    fi
    rm -rf "$verify_dir"
    echo "GitHub security merge-protection ruleset restored and verified from $snapshot" >&2
)

handle_exit() {
    local rc=$? rollback_rc=0
    trap - EXIT HUP INT TERM
    if [[ "$transaction_active" -eq 1 && -n "$recovery_dir" ]]; then
        echo "Security apply failed after ruleset mutation; rolling back." >&2
        restore_ruleset "$recovery_dir" || rollback_rc=$?
        if [[ "$rollback_rc" -ne 0 ]]; then
            echo "RECOVERY REQUIRED: $0 --restore '$recovery_dir' '$repo'" >&2
        fi
    fi
    if [[ -n "$work_dir" && -d "$work_dir" ]]; then
        rm -rf "$work_dir"
    fi
    exit "$rc"
}
trap handle_exit EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ -n "$restore_dir" ]]; then
    restore_dir="$(cd "$restore_dir" 2>/dev/null && pwd -P)" || {
        echo "FAIL: recovery snapshot directory does not exist" >&2
        exit 2
    }
    restore_ruleset "$restore_dir"
    exit 0
fi

[[ -f "$integrity_ruleset" ]] || { echo "FAIL: missing $integrity_ruleset" >&2; exit 3; }
work_dir="$(mktemp -d)"
live_main_sha="$(gh api "repos/$repo/commits/main" --jq .sha)"
verify_local_boundary "$live_main_sha"

visibility="$(gh api "repos/$repo" --jq .visibility)"
[[ "$visibility" == "public" ]] || {
    echo "FAIL: this posture requires a public repository or a plan with equivalent features" >&2
    exit 3
}

integrity_id="$(ruleset_id_by_name "Protect main: integrity")"
gh api "repos/$repo/rulesets/$integrity_id" > "$work_dir/integrity-live.json"
ruleset_payload "$work_dir/integrity-live.json" > "$work_dir/integrity-live-payload.json"
ruleset_payload "$integrity_ruleset" > "$work_dir/integrity-desired.json"
jq 'del(.rules[] | select(.type == "code_scanning"))' \
    "$work_dir/integrity-desired.json" > "$work_dir/integrity-preapply.json"

if json_equal "$work_dir/integrity-live-payload.json" "$work_dir/integrity-desired.json"; then
    ruleset_stage="applied"
elif json_equal "$work_dir/integrity-live-payload.json" "$work_dir/integrity-preapply.json"; then
    ruleset_stage="preapply"
else
    echo "FAIL: live integrity ruleset is neither the reviewed pre-apply nor desired posture" >&2
    exit 3
fi

gh api "repos/$repo/private-vulnerability-reporting" > "$work_dir/private-reporting.json"
gh api "repos/$repo/immutable-releases" > "$work_dir/immutable-releases.json"
gh api "repos/$repo/code-scanning/default-setup" > "$work_dir/codeql-config.json"

if [[ "$preflight_only" -eq 1 ]]; then
    jq -e '.enabled == true' "$work_dir/private-reporting.json" >/dev/null || {
        echo "FAIL: private vulnerability reporting is not enabled" >&2
        exit 4
    }
    jq -e '.enabled == true' "$work_dir/immutable-releases.json" >/dev/null || {
        echo "FAIL: immutable releases are not enabled" >&2
        exit 4
    }
    codeql_config_is_desired "$work_dir/codeql-config.json" || {
        echo "FAIL: CodeQL default setup is not actions+python with default queries" >&2
        exit 4
    }
else
    if ! jq -e '.enabled == true' "$work_dir/private-reporting.json" >/dev/null; then
        gh api -X PUT "repos/$repo/private-vulnerability-reporting" --silent
    fi
    if ! jq -e '.enabled == true' "$work_dir/immutable-releases.json" >/dev/null; then
        gh api -X PUT "repos/$repo/immutable-releases" --silent
    fi
    if ! codeql_config_is_desired "$work_dir/codeql-config.json"; then
        gh api -X PATCH "repos/$repo/code-scanning/default-setup" --input - <<'JSON' >/dev/null
{
  "state": "configured",
  "languages": ["actions", "python"],
  "query_suite": "default"
}
JSON
        echo "CodeQL configuration requested. Wait for its Actions and Python jobs to pass, then rerun." >&2
        exit 4
    fi
fi

gh api "repos/$repo/private-vulnerability-reporting" > "$work_dir/private-reporting.json"
gh api "repos/$repo/immutable-releases" > "$work_dir/immutable-releases.json"
jq -e '.enabled == true' "$work_dir/private-reporting.json" >/dev/null || {
    echo "FAIL: private vulnerability reporting enablement did not persist" >&2
    exit 4
}
jq -e '.enabled == true' "$work_dir/immutable-releases.json" >/dev/null || {
    echo "FAIL: immutable releases enablement did not persist" >&2
    exit 4
}

gh api "repos/$repo/code-scanning/analyses?ref=refs/heads/main&tool_name=CodeQL&per_page=100" \
    > "$work_dir/codeql-analyses.json"
verify_codeql_analyses "$work_dir/codeql-analyses.json" "$live_main_sha" || {
    echo "FAIL: successful Actions and Python CodeQL analyses are missing for live main $live_main_sha" >&2
    exit 4
}

if [[ "$preflight_only" -eq 1 ]]; then
    echo "GitHub security preflight passed at live main $live_main_sha; ruleset stage=$ruleset_stage."
    exit 0
fi
if [[ "$ruleset_stage" == "applied" ]]; then
    echo "GitHub security posture already matches the checked-in policy; no mutation was needed."
    exit 0
fi

git_metadata_path="$(git -C "$repo_root" rev-parse --git-path dotfiles-github-security)"
case "$git_metadata_path" in /*) ;; *) git_metadata_path="$repo_root/$git_metadata_path" ;; esac
mkdir -p "$git_metadata_path"
chmod 700 "$git_metadata_path"
recovery_dir="$(mktemp -d "$git_metadata_path/recovery.XXXXXX")"
cp "$work_dir/integrity-live-payload.json" "$recovery_dir/integrity-restore.json"
jq -n --arg repository "$repo" --arg live_main_sha "$live_main_sha" \
    --argjson integrity_ruleset_id "$integrity_id" \
    '{schema: 1, repository: $repository, live_main_sha: $live_main_sha,
      integrity_ruleset_id: $integrity_ruleset_id}' > "$recovery_dir/manifest.json"
chmod 400 "$recovery_dir"/*
chmod 500 "$recovery_dir"

transaction_active=1
chmod 400 "$work_dir/integrity-desired.json"
gh_api_json_file PUT "repos/$repo/rulesets/$integrity_id" \
    "$work_dir/integrity-desired.json" >/dev/null
gh api "repos/$repo/rulesets/$integrity_id" > "$work_dir/integrity-readback.json"
ruleset_payload "$work_dir/integrity-readback.json" > "$work_dir/integrity-readback-payload.json"
json_equal "$work_dir/integrity-readback-payload.json" "$work_dir/integrity-desired.json" || {
    echo "FAIL: CodeQL merge-protection readback differs from checked-in policy" >&2
    exit 5
}
transaction_active=0

echo "GitHub security posture applied and verified."
echo "Pre-change ruleset snapshot retained at: $recovery_dir"
