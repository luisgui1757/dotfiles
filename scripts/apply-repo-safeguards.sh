#!/usr/bin/env bash
# Applies the reviewed stable-check cutover documented in
# docs/security/branch-protection.md. Every live read and proof gate completes
# before the first mutation. The three changed resources are snapshotted and
# automatically restored if application or readback fails.
set -euo pipefail

usage() {
    cat <<'EOF'
apply-repo-safeguards.sh [--preflight-only] [owner/repo]
apply-repo-safeguards.sh --restore <snapshot-directory> [owner/repo]

The normal path verifies the complete expected live posture, exact live main,
reviewed local sources, unique rulesets, and exact GitHub Actions run provenance
before changing only:
  - Actions full-SHA pinning
  - Protect main: integrity required checks
  - classic main required checks

Before the first mutation it saves a private recovery snapshot under this
checkout's Git metadata. Any apply/readback failure attempts automatic rollback
and prints the exact --restore command for manual recovery.

--preflight-only performs every read-only check without creating a persistent
snapshot or changing live repository state.
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
            [[ -z "$restore_dir" ]] || { echo "FAIL: --preflight-only and --restore are mutually exclusive" >&2; exit 2; }
            preflight_only=1
            shift
            ;;
        --restore)
            [[ "$preflight_only" -eq 0 && -z "$restore_dir" && "$#" -ge 2 ]] || {
                echo "FAIL: --restore requires one snapshot directory and cannot be combined with --preflight-only" >&2
                exit 2
            }
            restore_dir="$2"
            shift 2
            ;;
        --*)
            echo "FAIL: unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
        *)
            break
            ;;
    esac
done

if [[ "$#" -gt 1 ]]; then
    echo "FAIL: expected at most one owner/repo argument" >&2
    exit 2
fi

for command_name in gh git jq; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "FAIL: $command_name is required." >&2
        exit 1
    fi
done
gh auth status >/dev/null

repo="${1:-${GITHUB_REPOSITORY:-}}"
if [[ -z "$repo" ]]; then
    repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi
if [[ ! "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "FAIL: repository must be a literal owner/repo, got an invalid value." >&2
    exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
check_identities="$repo_root/.github/check-identities.json"
integrity_ruleset="$repo_root/.github/rulesets/main-integrity.json"
review_ruleset="$repo_root/.github/rulesets/main-review.json"
owner_updates_ruleset="$repo_root/.github/rulesets/main-owner-updates.json"
github_actions_app_id=15368

temporary_dirs=()
preflight_dir=""
postflight_dir=""
recovery_dir=""
transaction_dir=""
second_capture_dir=""
transaction_active=0
recovery_retained=0
exit_handler_active=0

create_private_temp_dir() {
    local result_name="$1" created temp_root
    temp_root="${TMPDIR:-/tmp}"
    created="$(mktemp -d "$temp_root/dotfiles-safeguards.XXXXXX")"
    chmod 700 "$created"
    temporary_dirs[${#temporary_dirs[@]}]="$created"
    printf -v "$result_name" '%s' "$created"
}

cleanup_temporary_dirs() {
    local directory
    if [[ "${#temporary_dirs[@]}" -eq 0 ]]; then
        return 0
    fi
    for directory in "${temporary_dirs[@]}"; do
        if [[ -n "$directory" && -d "$directory" ]]; then
            chmod -R u+w "$directory" 2>/dev/null || true
            rm -rf "$directory"
        fi
    done
}

gh_api_json_file() {
    local method="$1" path="$2" file="$3"
    echo "+ gh api -X $method $path --input $file"
    gh api -X "$method" "$path" --input "$file"
}

canonicalize_json() {
    jq -S '
      def sort_arrays:
        walk(if type == "array" then sort_by(tojson) else . end);
      sort_arrays
    ' "$1"
}

json_equal() {
    local left="$1" right="$2" left_canonical right_canonical rc
    left_canonical="$(mktemp)"
    right_canonical="$(mktemp)"
    canonicalize_json "$left" > "$left_canonical"
    canonicalize_json "$right" > "$right_canonical"
    if cmp -s "$left_canonical" "$right_canonical"; then
        rc=0
    else
        rc=1
    fi
    rm -f "$left_canonical" "$right_canonical"
    return "$rc"
}

require_json_equal() {
    local desc="$1" actual="$2" expected="$3" actual_canonical expected_canonical
    if json_equal "$actual" "$expected"; then
        return 0
    fi
    actual_canonical="$(mktemp)"
    expected_canonical="$(mktemp)"
    canonicalize_json "$actual" > "$actual_canonical"
    canonicalize_json "$expected" > "$expected_canonical"
    echo "FAIL: unexpected live $desc; refusing to mutate." >&2
    diff -u "$expected_canonical" "$actual_canonical" >&2 || true
    rm -f "$actual_canonical" "$expected_canonical"
    return 1
}

required_check_contexts() {
    cat <<'EOF'
ubuntu
macos
windows
chezmoi-parity
chezmoi-parity-macos
chezmoi-parity-windows
nix flake check / linux
nix flake check / macos
e2e containers / linux
setup.sh / linux
setup.sh / macos
setup.ps1 / windows
EOF
}

legacy_check_contexts() {
    jq -r '.legacyEmitted[]' "$check_identities"
}

test_workflow_contexts() {
    cat <<'EOF'
ubuntu
macos
windows
chezmoi-parity
chezmoi-parity-macos
chezmoi-parity-windows
EOF
}

nix_workflow_contexts() {
    cat <<'EOF'
nix flake check (ubuntu-24.04)
nix flake check (macos-26)
nix flake check / linux
nix flake check / macos
EOF
}

e2e_workflow_contexts() {
    cat <<'EOF'
e2e containers / ubuntu-24.04
setup.sh / ubuntu-24.04
setup.sh / macos-26
setup.ps1 / windows-2025
e2e containers / linux
setup.sh / linux
setup.sh / macos
setup.ps1 / windows
EOF
}

contexts_json() {
    "$1" | jq -R . | jq -s .
}

contexts_file_json() {
    jq -R . "$1" | jq -s .
}

build_classic_payload() {
    local contexts
    contexts="$(contexts_json "$1")"
    jq -n --argjson contexts "$contexts" --argjson app_id "$github_actions_app_id" '{
      strict: true,
      contexts: $contexts,
      checks: ($contexts | map({context: ., app_id: $app_id}))
    }'
}

build_classic_payload_from_file() {
    local contexts
    contexts="$(contexts_file_json "$1")"
    jq -n --argjson contexts "$contexts" --argjson app_id "$github_actions_app_id" '{
      strict: true,
      contexts: $contexts,
      checks: ($contexts | map({context: ., app_id: $app_id}))
    }'
}

build_classic_state() {
    local contexts
    contexts="$(contexts_json "$1")"
    jq -n --argjson contexts "$contexts" --argjson app_id "$github_actions_app_id" '{
      required_status_checks: {
        strict: true,
        contexts: ($contexts | sort),
        checks: ($contexts | map({context: ., app_id: $app_id}) | sort_by(.context))
      },
      enforce_admins: true,
      required_pull_request_reviews: null,
      restrictions: null,
      required_linear_history: true,
      allow_force_pushes: false,
      allow_deletions: false,
      required_conversation_resolution: true,
      required_signatures: false,
      block_creations: false,
      lock_branch: false,
      allow_fork_syncing: false
    }'
}

build_classic_state_from_file() {
    local contexts
    contexts="$(contexts_file_json "$1")"
    jq -n --argjson contexts "$contexts" --argjson app_id "$github_actions_app_id" '{
      required_status_checks: {
        strict: true,
        contexts: ($contexts | sort),
        checks: ($contexts | map({context: ., app_id: $app_id}) | sort_by(.context))
      },
      enforce_admins: true,
      required_pull_request_reviews: null,
      restrictions: null,
      required_linear_history: true,
      allow_force_pushes: false,
      allow_deletions: false,
      required_conversation_resolution: true,
      required_signatures: false,
      block_creations: false,
      lock_branch: false,
      allow_fork_syncing: false
    }'
}

normalize_classic_state() {
    jq '{
      required_status_checks: {
        strict: .required_status_checks.strict,
        contexts: (.required_status_checks.contexts | sort),
        checks: (.required_status_checks.checks | map({context, app_id}) | sort_by(.context))
      },
      enforce_admins: .enforce_admins.enabled,
      required_pull_request_reviews,
      restrictions,
      required_linear_history: .required_linear_history.enabled,
      allow_force_pushes: .allow_force_pushes.enabled,
      allow_deletions: .allow_deletions.enabled,
      required_conversation_resolution: .required_conversation_resolution.enabled,
      required_signatures: .required_signatures.enabled,
      block_creations: .block_creations.enabled,
      lock_branch: .lock_branch.enabled,
      allow_fork_syncing: .allow_fork_syncing.enabled
    }' "$1"
}

validate_classic_live_schema() {
    jq -e '
      type == "object"
      and has("required_status_checks")
      and has("enforce_admins")
      and has("required_linear_history")
      and has("allow_force_pushes")
      and has("allow_deletions")
      and has("required_conversation_resolution")
      and has("required_signatures")
      and has("block_creations")
      and has("lock_branch")
      and has("allow_fork_syncing")
      and (.required_status_checks | type == "object")
      and (.required_status_checks.strict | type == "boolean")
      and (.required_status_checks.contexts | type == "array")
      and all(.required_status_checks.contexts[]; type == "string" and length > 0)
      and (.required_status_checks.checks | type == "array")
      and all(.required_status_checks.checks[];
        type == "object"
        and (.context | type == "string" and length > 0)
        and (.app_id | type == "number"))
      and (.enforce_admins.enabled | type == "boolean")
      and .required_pull_request_reviews? == null
      and .restrictions? == null
      and (.required_linear_history.enabled | type == "boolean")
      and (.allow_force_pushes.enabled | type == "boolean")
      and (.allow_deletions.enabled | type == "boolean")
      and (.required_conversation_resolution.enabled | type == "boolean")
      and (.required_signatures.enabled | type == "boolean")
      and (.block_creations.enabled | type == "boolean")
      and (.lock_branch.enabled | type == "boolean")
      and (.allow_fork_syncing.enabled | type == "boolean")
    ' "$1" >/dev/null
}

classic_restore_payload() {
    jq '.required_status_checks | {
      strict,
      contexts,
      checks: (.checks | map({context, app_id}))
    }' "$1"
}

normalize_ruleset() {
    jq '
      {name, target, enforcement, bypass_actors, conditions, rules}
      | del(.rules[].parameters.required_reviewers?)
    ' "$1" | canonicalize_json /dev/stdin
}

ruleset_restore_payload() {
    jq '
      {name, target, enforcement, bypass_actors, conditions, rules}
      | del(.rules[].parameters.required_reviewers?)
    ' "$1"
}

ruleset_id_by_name() {
    local name="$1" ids count
    ids="$(gh api "repos/$repo/rulesets?includes_parents=false" \
        --jq ".[] | select(.name == \"$name\") | .id")"
    count="$(awk 'NF { count++ } END { print count + 0 }' <<<"$ids")"
    if [[ "$count" -ne 1 ]]; then
        echo "FAIL: found $count rulesets named '$name'; expected exactly one before applying safeguards." >&2
        return 1
    fi
    printf '%s\n' "$ids"
}

normalize_remote_repo() {
    local remote="$1" normalized
    case "$remote" in
        https://github.com/*)
            normalized="${remote#https://github.com/}"
            ;;
        git@github.com:*)
            normalized="${remote#git@github.com:}"
            ;;
        ssh://git@github.com/*)
            normalized="${remote#ssh://git@github.com/}"
            ;;
        *)
            return 1
            ;;
    esac
    normalized="${normalized%.git}"
    [[ "$normalized" == "$repo" ]]
}

verify_local_boundary() {
    local capture_dir="$1" local_branch local_head local_main live_main_sha origin_url dirty
    local_branch="$(git -C "$repo_root" symbolic-ref -q --short HEAD || true)"
    [[ "$local_branch" == "main" ]] || {
        echo "FAIL: safeguard apply requires the checked-out local branch to be main; got '${local_branch:-detached}'" >&2
        return 1
    }
    origin_url="$(git -C "$repo_root" remote get-url origin 2>/dev/null || true)"
    if ! normalize_remote_repo "$origin_url"; then
        echo "FAIL: checkout origin is not the requested github.com/$repo repository." >&2
        return 1
    fi
    local_head="$(git -C "$repo_root" rev-parse HEAD)"
    local_main="$(git -C "$repo_root" rev-parse refs/heads/main)"
    live_main_sha="$(jq -r .sha "$capture_dir/live-main.json")"
    if [[ "$local_head" != "$local_main" ]]; then
        echo "FAIL: checked-out HEAD $local_head is not the local main ref $local_main." >&2
        return 1
    fi
    if [[ "$local_head" != "$live_main_sha" ]]; then
        echo "FAIL: local main/HEAD $local_head is not exact live main $live_main_sha." >&2
        echo "      Fetch and update main, then rerun only after its proof workflows pass." >&2
        return 1
    fi

    dirty="$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all -- \
        .github/check-identities.json \
        .github/settings.yml \
        .github/rulesets \
        .github/workflows/e2e-install.yml \
        .github/workflows/nix.yml \
        .github/workflows/test.yml \
        scripts/apply-repo-safeguards.sh \
        scripts/ci-logical-proof.sh)"
    if [[ -n "$dirty" ]]; then
        echo "FAIL: reviewed safeguard/proof sources differ from exact live main." >&2
        printf '%s\n' "$dirty" >&2
        return 1
    fi
}

select_workflow_run() {
    local capture_dir="$1" workflow_file="$2" workflow_path="$3" allowed_events="$4" output="$5"
    local runs_file="$capture_dir/${workflow_file%.yml}-runs.json"
    gh api "repos/$repo/actions/workflows/$workflow_file/runs?branch=main&per_page=100" > "$runs_file"
    if ! jq -e \
        --arg repo "$repo" \
        --arg sha "$(jq -r .sha "$capture_dir/live-main.json")" \
        --arg path "$workflow_path" \
        --argjson events "$allowed_events" '
          [.workflow_runs[]
            | select(.repository.full_name == $repo)
            | select(.head_branch == "main" and .head_sha == $sha)
            | select(.path == $path)
            | select(.status == "completed" and .conclusion == "success")
            | select(.event as $event | $events | index($event))]
          | sort_by(.run_number // .id)
          | reverse
          | .[0] // empty
        ' "$runs_file" > "$output"; then
        echo "FAIL: no successful $workflow_path run with allowed event provenance exists on exact live main." >&2
        return 1
    fi
}

verify_workflow_jobs() {
    local capture_dir="$1" desc="$2" run_file="$3" context_function="$4" require_cache_free="$5"
    local run_id jobs_file expected_file context count prefix
    run_id="$(jq -r .id "$run_file")"
    jobs_file="$capture_dir/${desc}-jobs.json"
    expected_file="$capture_dir/${desc}-expected-contexts.txt"
    gh api "repos/$repo/actions/runs/$run_id/jobs?per_page=100" > "$jobs_file"
    "$context_function" > "$expected_file"
    while IFS= read -r context; do
        count="$(jq --arg name "$context" '[.jobs[] | select(.name == $name and .status == "completed" and .conclusion == "success")] | length' "$jobs_file")"
        if [[ "$count" -ne 1 ]]; then
            echo "FAIL: $desc run $run_id has $count successful jobs named '$context'; expected exactly one." >&2
            return 1
        fi
        prefix="https://github.com/$repo/actions/runs/$run_id/job/"
        count="$(jq --arg name "$context" --arg prefix "$prefix" --argjson app_id "$github_actions_app_id" '
          [.check_runs[]
            | select(.name == $name)
            | select(.status == "completed" and .conclusion == "success")
            | select(.app.id == $app_id and .app.slug == "github-actions")
            | select(.details_url | startswith($prefix))]
          | length
        ' "$capture_dir/check-runs.json")"
        if [[ "$count" -ne 1 ]]; then
            echo "FAIL: '$context' is not uniquely bound to GitHub Actions app $github_actions_app_id and $desc run $run_id." >&2
            return 1
        fi
    done < "$expected_file"

    if [[ "$require_cache_free" -eq 1 ]]; then
        if ! jq -e '
          [.jobs[].steps[]? | select(.name | startswith("PR-only cache:"))] as $cache_steps
          | ($cache_steps | length) == 3
          and all($cache_steps[]; .conclusion == "skipped")
        ' "$jobs_file" >/dev/null; then
            echo "FAIL: $desc run $run_id did not skip every broad actions/cache step." >&2
            return 1
        fi
    fi
}

capture_and_validate_live_state() {
    local capture_dir="$1" policy_dir="${2:-}"
    local integrity_id review_id owner_updates_id stage
    local identities_source="$check_identities" integrity_source="$integrity_ruleset"
    local review_source="$review_ruleset" owner_source="$owner_updates_ruleset"
    local live_integrity_normalized expected_integrity_normalized expected_legacy_integrity_normalized
    local live_review_normalized expected_review_normalized
    local live_owner_normalized expected_owner_normalized
    local live_classic_state expected_legacy_classic expected_stable_classic
    if [[ -n "$policy_dir" ]]; then
        for file in check-identities.json integrity-desired.json review-expected.json owner-expected.json; do
            [[ -f "$policy_dir/$file" && ! -L "$policy_dir/$file" ]] || {
                echo "FAIL: frozen postflight policy is incomplete or unsafe: $file" >&2
                return 1
            }
        done
        identities_source="$policy_dir/check-identities.json"
        integrity_source="$policy_dir/integrity-desired.json"
        review_source="$policy_dir/review-expected.json"
        owner_source="$policy_dir/owner-expected.json"
    fi
    gh api "repos/$repo" > "$capture_dir/repository.json"
    if [[ "$(jq -r .full_name "$capture_dir/repository.json")" != "$repo" ]]; then
        echo "FAIL: GitHub resolved a different repository than $repo." >&2
        return 1
    fi
    if ! jq -e '
      .private == false
      and .visibility == "public"
      and .default_branch == "main"
      and .allow_merge_commit == false
      and .allow_squash_merge == true
      and .allow_rebase_merge == false
      and .allow_auto_merge == false
      and .delete_branch_on_merge == true
      and .security_and_analysis.dependabot_security_updates.status == "enabled"
      and .security_and_analysis.secret_scanning.status == "enabled"
      and .security_and_analysis.secret_scanning_push_protection.status == "enabled"
    ' "$capture_dir/repository.json" >/dev/null; then
        echo "FAIL: live repository/merge/security settings differ from the reviewed public-repo posture." >&2
        return 1
    fi
    jq '{
      full_name,
      private,
      visibility,
      default_branch,
      allow_merge_commit,
      allow_squash_merge,
      allow_rebase_merge,
      allow_auto_merge,
      delete_branch_on_merge,
      security_and_analysis: {
        dependabot_security_updates: .security_and_analysis.dependabot_security_updates.status,
        secret_scanning: .security_and_analysis.secret_scanning.status,
        secret_scanning_push_protection: .security_and_analysis.secret_scanning_push_protection.status
      }
    }' "$capture_dir/repository.json" > "$capture_dir/repository-state.json"
    gh api "repos/$repo/commits/main" > "$capture_dir/live-main.json"
    verify_local_boundary "$capture_dir"

    gh api "repos/$repo/rulesets?includes_parents=false" > "$capture_dir/rulesets.json"
    if ! jq -e --arg repo "$repo" '
      length == 3
      and ([.[].name] | sort) == (["Protect main: integrity", "Protect main: owner updates", "Protect main: review"] | sort)
      and all(.[]; .source == $repo and .source_type == "Repository" and .target == "branch" and .enforcement == "active")
    ' "$capture_dir/rulesets.json" >/dev/null; then
        echo "FAIL: live rulesets are not the exact three unique active repository rulesets." >&2
        return 1
    fi
    jq 'map({id, name, source, source_type, target, enforcement}) | sort_by(.name)' \
        "$capture_dir/rulesets.json" > "$capture_dir/rulesets-state.json"
    integrity_id="$(ruleset_id_by_name "Protect main: integrity")"
    review_id="$(ruleset_id_by_name "Protect main: review")"
    owner_updates_id="$(ruleset_id_by_name "Protect main: owner updates")"
    gh api "repos/$repo/rulesets/$integrity_id" > "$capture_dir/integrity-live.json"
    gh api "repos/$repo/rulesets/$review_id" > "$capture_dir/review-live.json"
    gh api "repos/$repo/rulesets/$owner_updates_id" > "$capture_dir/owner-updates-live.json"

    jq --slurpfile identities "$identities_source" '
      (.rules[] | select(.type == "required_status_checks").parameters.required_status_checks) =
        ($identities[0].legacyEmitted | map({context: ., integration_id: 15368}))
    ' "$integrity_source" > "$capture_dir/integrity-legacy-expected.json"
    normalize_ruleset "$capture_dir/integrity-live.json" > "$capture_dir/integrity-live-normalized.json"
    normalize_ruleset "$integrity_source" > "$capture_dir/integrity-stable-normalized.json"
    normalize_ruleset "$capture_dir/integrity-legacy-expected.json" > "$capture_dir/integrity-legacy-normalized.json"
    live_integrity_normalized="$capture_dir/integrity-live-normalized.json"
    expected_integrity_normalized="$capture_dir/integrity-stable-normalized.json"
    expected_legacy_integrity_normalized="$capture_dir/integrity-legacy-normalized.json"

    normalize_ruleset "$capture_dir/review-live.json" > "$capture_dir/review-live-normalized.json"
    normalize_ruleset "$review_source" > "$capture_dir/review-expected-normalized.json"
    live_review_normalized="$capture_dir/review-live-normalized.json"
    expected_review_normalized="$capture_dir/review-expected-normalized.json"
    require_json_equal "review ruleset" "$live_review_normalized" "$expected_review_normalized"

    normalize_ruleset "$capture_dir/owner-updates-live.json" > "$capture_dir/owner-live-normalized.json"
    normalize_ruleset "$owner_source" > "$capture_dir/owner-expected-normalized.json"
    live_owner_normalized="$capture_dir/owner-live-normalized.json"
    expected_owner_normalized="$capture_dir/owner-expected-normalized.json"
    require_json_equal "owner-updates ruleset" "$live_owner_normalized" "$expected_owner_normalized"

    gh api "repos/$repo/branches/main/protection" > "$capture_dir/classic-live.json"
    if ! validate_classic_live_schema "$capture_dir/classic-live.json"; then
        echo "FAIL: live classic branch protection is incomplete or malformed." >&2
        return 1
    fi
    normalize_classic_state "$capture_dir/classic-live.json" > "$capture_dir/classic-live-state.json"
    jq -r '.legacyEmitted[]' "$identities_source" > "$capture_dir/legacy-contexts.txt"
    jq -r '.required[]' "$identities_source" > "$capture_dir/stable-contexts.txt"
    build_classic_state_from_file "$capture_dir/legacy-contexts.txt" > "$capture_dir/classic-legacy-state.json"
    build_classic_state_from_file "$capture_dir/stable-contexts.txt" > "$capture_dir/classic-stable-state.json"
    live_classic_state="$capture_dir/classic-live-state.json"
    expected_legacy_classic="$capture_dir/classic-legacy-state.json"
    expected_stable_classic="$capture_dir/classic-stable-state.json"

    if json_equal "$live_integrity_normalized" "$expected_legacy_integrity_normalized" && \
        json_equal "$live_classic_state" "$expected_legacy_classic"; then
        stage="legacy"
    elif json_equal "$live_integrity_normalized" "$expected_integrity_normalized" && \
        json_equal "$live_classic_state" "$expected_stable_classic"; then
        stage="stable"
    else
        echo "FAIL: integrity and classic protection are not one exact, internally consistent legacy-or-stable stage." >&2
        return 1
    fi
    printf '%s\n' "$stage" > "$capture_dir/stage"

    gh api "repos/$repo/actions/permissions" > "$capture_dir/actions-live.json"
    if ! jq -e '.enabled == true and .allowed_actions == "all"' "$capture_dir/actions-live.json" >/dev/null; then
        echo "FAIL: live Actions enabled/allowed posture is unexpected." >&2
        return 1
    fi
    if [[ "$stage" == "legacy" ]]; then
        jq -e '.sha_pinning_required == false' "$capture_dir/actions-live.json" >/dev/null || {
            echo "FAIL: legacy required contexts must coincide with sha_pinning_required=false before this cutover." >&2
            return 1
        }
    else
        jq -e '.sha_pinning_required == true' "$capture_dir/actions-live.json" >/dev/null || {
            echo "FAIL: stable required contexts must coincide with sha_pinning_required=true." >&2
            return 1
        }
    fi

    gh api --silent "repos/$repo/vulnerability-alerts" >/dev/null || {
        echo "FAIL: vulnerability alerts are not enabled or could not be verified." >&2
        return 1
    }
    gh api --silent "repos/$repo/automated-security-fixes" >/dev/null || {
        echo "FAIL: automated security fixes are not enabled or could not be verified." >&2
        return 1
    }

    gh api "repos/$repo/commits/$(jq -r .sha "$capture_dir/live-main.json")/check-runs?per_page=100" > "$capture_dir/check-runs.json"
    select_workflow_run "$capture_dir" test.yml .github/workflows/test.yml '["push", "workflow_dispatch"]' "$capture_dir/test-run.json"
    select_workflow_run "$capture_dir" nix.yml .github/workflows/nix.yml '["push", "workflow_dispatch"]' "$capture_dir/nix-run.json"
    select_workflow_run "$capture_dir" e2e-install.yml .github/workflows/e2e-install.yml '["workflow_dispatch"]' "$capture_dir/e2e-run.json"
    verify_workflow_jobs "$capture_dir" test "$capture_dir/test-run.json" test_workflow_contexts 0
    verify_workflow_jobs "$capture_dir" nix "$capture_dir/nix-run.json" nix_workflow_contexts 0
    verify_workflow_jobs "$capture_dir" e2e "$capture_dir/e2e-run.json" e2e_workflow_contexts 1

    ruleset_restore_payload "$capture_dir/integrity-live.json" > "$capture_dir/integrity-restore.json"
    classic_restore_payload "$capture_dir/classic-live.json" > "$capture_dir/classic-restore.json"
    jq '{enabled, allowed_actions, sha_pinning_required}' "$capture_dir/actions-live.json" > "$capture_dir/actions-restore.json"
    jq -n \
        --arg repo "$repo" \
        --arg live_main_sha "$(jq -r .sha "$capture_dir/live-main.json")" \
        --arg stage "$stage" \
        --argjson integrity_ruleset_id "$integrity_id" \
        --argjson test_run_id "$(jq -r .id "$capture_dir/test-run.json")" \
        --argjson nix_run_id "$(jq -r .id "$capture_dir/nix-run.json")" \
        --argjson e2e_run_id "$(jq -r .id "$capture_dir/e2e-run.json")" \
        '{schema: 1, repository: $repo, live_main_sha: $live_main_sha, stage: $stage,
          integrity_ruleset_id: $integrity_ruleset_id,
          proof_runs: {test: $test_run_id, nix: $nix_run_id, e2e_cache_free: $e2e_run_id}}' \
        > "$capture_dir/manifest.json"
}

verify_snapshot_unchanged() {
    local verify_dir file
    create_private_temp_dir verify_dir
    capture_and_validate_live_state "$verify_dir"
    for file in \
        manifest.json \
        repository-state.json \
        rulesets-state.json \
        integrity-live-normalized.json \
        review-live-normalized.json \
        owner-live-normalized.json \
        classic-live-state.json \
        actions-restore.json; do
        if ! json_equal "$verify_dir/$file" "$recovery_dir/$file"; then
            echo "FAIL: live safeguard/proof state changed after preflight and snapshot; no mutation was attempted." >&2
            return 1
        fi
    done
    second_capture_dir="$verify_dir"
}

prepare_transaction_payloads() {
    local file
    create_private_temp_dir transaction_dir

    for file in \
        manifest.json \
        actions-restore.json \
        integrity-restore.json \
        classic-restore.json \
        classic-live.json; do
        [[ -f "$recovery_dir/$file" && ! -L "$recovery_dir/$file" ]] || {
            echo "FAIL: generated recovery snapshot is incomplete or unsafe: $file" >&2
            return 1
        }
    done

    git -C "$repo_root" show HEAD:.github/check-identities.json \
        > "$transaction_dir/check-identities.json"
    git -C "$repo_root" show HEAD:.github/rulesets/main-integrity.json \
        > "$transaction_dir/integrity-desired.json"
    git -C "$repo_root" show HEAD:.github/rulesets/main-review.json \
        > "$transaction_dir/review-expected.json"
    git -C "$repo_root" show HEAD:.github/rulesets/main-owner-updates.json \
        > "$transaction_dir/owner-expected.json"
    cp "$recovery_dir/manifest.json" "$transaction_dir/manifest.json"

    if ! cmp -s "$check_identities" "$transaction_dir/check-identities.json" || \
        ! cmp -s "$integrity_ruleset" "$transaction_dir/integrity-desired.json" || \
        ! cmp -s "$review_ruleset" "$transaction_dir/review-expected.json" || \
        ! cmp -s "$owner_updates_ruleset" "$transaction_dir/owner-expected.json"; then
        echo "FAIL: reviewed safeguard sources changed while transaction inputs were frozen." >&2
        return 1
    fi
    if ! jq -e '
      type == "object"
      and (keys | sort) == (["legacyEmitted", "replacements", "required", "schema", "stage"] | sort)
      and .schema == 2
      and .stage == "stable-required-live-applied"
      and (.legacyEmitted | type == "array" and length > 0 and length == (unique | length))
      and all(.legacyEmitted[]; type == "string" and length > 0)
      and (.required | type == "array" and length > 0 and length == (unique | length))
      and all(.required[]; type == "string" and length > 0)
      and (.replacements | type == "array" and length > 0 and length == (unique | length))
      and all(.replacements[];
        type == "object"
        and (keys | sort) == (["legacy", "logical"] | sort)
        and (.legacy | type == "string" and length > 0)
        and (.logical | type == "string" and length > 0))
    ' "$transaction_dir/check-identities.json" >/dev/null; then
        echo "FAIL: frozen required-check metadata is malformed." >&2
        return 1
    fi

    jq -r '.required[]' "$transaction_dir/check-identities.json" \
        > "$transaction_dir/required-contexts.txt"
    required_check_contexts > "$transaction_dir/required-contexts-expected.txt"
    if ! cmp -s "$transaction_dir/required-contexts.txt" \
        "$transaction_dir/required-contexts-expected.txt"; then
        echo "FAIL: frozen required-check metadata differs from the reviewed transaction contract." >&2
        return 1
    fi
    if ! jq -e --slurpfile identities "$transaction_dir/check-identities.json" \
        --argjson app_id "$github_actions_app_id" '
      type == "object"
      and .name == "Protect main: integrity"
      and .target == "branch"
      and .enforcement == "active"
      and .bypass_actors == []
      and .conditions.ref_name.include == ["refs/heads/main"]
      and ([.rules[] | select(.type == "required_status_checks")] | length) == 1
      and ([.rules[] | select(.type == "required_status_checks")][0].parameters |
        .strict_required_status_checks_policy == true
        and .do_not_enforce_on_create == false
        and .required_status_checks ==
          ($identities[0].required | map({context: ., integration_id: $app_id})))
    ' "$transaction_dir/integrity-desired.json" >/dev/null; then
        echo "FAIL: frozen integrity payload does not match the reviewed stable contexts and app identity." >&2
        return 1
    fi
    normalize_ruleset "$transaction_dir/integrity-desired.json" \
        > "$transaction_dir/integrity-desired-normalized.json"
    if ! json_equal "$transaction_dir/integrity-desired-normalized.json" \
        "$preflight_dir/integrity-stable-normalized.json"; then
        echo "FAIL: frozen integrity payload differs from the preflighted stable posture." >&2
        return 1
    fi
    normalize_ruleset "$transaction_dir/review-expected.json" \
        > "$transaction_dir/review-expected-normalized.json"
    normalize_ruleset "$transaction_dir/owner-expected.json" \
        > "$transaction_dir/owner-expected-normalized.json"
    if [[ -z "$second_capture_dir" ]] || \
        ! json_equal "$transaction_dir/review-expected-normalized.json" \
            "$second_capture_dir/review-live-normalized.json" || \
        ! json_equal "$transaction_dir/owner-expected-normalized.json" \
            "$second_capture_dir/owner-live-normalized.json"; then
        echo "FAIL: frozen unchanged ruleset expectations differ from the second live capture." >&2
        return 1
    fi
    cp "$second_capture_dir/repository-state.json" \
        "$transaction_dir/repository-state-expected.json"
    cp "$second_capture_dir/rulesets-state.json" \
        "$transaction_dir/rulesets-state-expected.json"

    build_classic_payload_from_file "$transaction_dir/required-contexts.txt" \
        > "$transaction_dir/classic-desired.json"
    jq -n '{enabled: true, allowed_actions: "all", sha_pinning_required: true}' \
        > "$transaction_dir/actions-desired.json"
    if ! jq -e --arg repo "$repo" \
        --arg sha "$(git -C "$repo_root" rev-parse HEAD)" '
      type == "object"
      and .schema == 1
      and .repository == $repo
      and .live_main_sha == $sha
      and .stage == "legacy"
      and (.integrity_ruleset_id | type == "number" and . > 0 and floor == .)
    ' "$transaction_dir/manifest.json" >/dev/null; then
        echo "FAIL: frozen transaction manifest is not the exact legacy-to-stable target." >&2
        return 1
    fi
    if ! jq -e --argjson app_id "$github_actions_app_id" '
      type == "object"
      and (keys | sort) == (["checks", "contexts", "strict"] | sort)
      and .strict == true
      and (.contexts | type == "array" and length > 0 and length == (unique | length))
      and (.checks | type == "array")
      and ((.checks | length) == (.contexts | length))
      and ((.contexts | sort) == (.checks | map(.context) | sort))
      and all(.checks[]; .app_id == $app_id)
    ' "$transaction_dir/classic-desired.json" >/dev/null; then
        echo "FAIL: frozen classic payload is malformed." >&2
        return 1
    fi

    chmod 400 "$transaction_dir"/*
    chmod 500 "$transaction_dir"
}

restore_snapshot() (
    local snapshot="$1" frozen file manifest_repo policy_sha stage integrity_id
    local expected_pin failures=0 readback_error=0 readback_mismatch=0 verify_dir
    for file in \
        manifest.json \
        actions-restore.json \
        integrity-restore.json \
        classic-restore.json \
        classic-live.json; do
        [[ -f "$snapshot/$file" && ! -L "$snapshot/$file" ]] || {
            echo "FAIL: recovery snapshot has a missing or unsafe $file: $snapshot" >&2
            return 1
        }
    done

    frozen="$(mktemp -d)"
    chmod 700 "$frozen"
    trap 'chmod -R u+w "$frozen" 2>/dev/null || true; rm -rf "$frozen"; if [[ -n "$verify_dir" ]]; then rm -rf "$verify_dir"; fi' EXIT
    for file in \
        manifest.json \
        actions-restore.json \
        integrity-restore.json \
        classic-restore.json \
        classic-live.json; do
        if ! cp "$snapshot/$file" "$frozen/$file"; then
            echo "FAIL: could not freeze recovery snapshot file before validation: $snapshot/$file" >&2
            return 1
        fi
    done
    chmod -R go-rwx "$frozen"

    if ! jq -e '
      type == "object"
      and (keys | sort) == ([
        "integrity_ruleset_id", "live_main_sha", "proof_runs",
        "repository", "schema", "stage"
      ] | sort)
      and .schema == 1
      and (.repository | type == "string")
      and (.live_main_sha | test("^[0-9a-f]{40}$"))
      and (.stage == "legacy" or .stage == "stable")
      and ((.integrity_ruleset_id | type) == "number")
      and (.integrity_ruleset_id > 0)
      and ((.integrity_ruleset_id | floor) == .integrity_ruleset_id)
      and (.proof_runs | type == "object")
      and (.proof_runs | keys | sort) == (["e2e_cache_free", "nix", "test"] | sort)
      and all(.proof_runs[]; type == "number" and . > 0 and floor == .)
    ' "$frozen/manifest.json" >/dev/null || \
        ! jq -e '
          type == "object"
          and (keys | sort) == (["allowed_actions", "enabled", "sha_pinning_required"] | sort)
          and .enabled == true
          and .allowed_actions == "all"
          and (.sha_pinning_required | type == "boolean")
        ' "$frozen/actions-restore.json" >/dev/null || \
        ! jq -e '
          type == "object"
          and .name == "Protect main: integrity"
          and .target == "branch"
          and .enforcement == "active"
          and (.bypass_actors | type == "array")
          and (.conditions | type == "object")
          and (.rules | type == "array")
        ' "$frozen/integrity-restore.json" >/dev/null || \
        ! jq -e --argjson app_id "$github_actions_app_id" '
          type == "object"
          and (keys | sort) == (["checks", "contexts", "strict"] | sort)
          and (.strict | type == "boolean")
          and (.contexts | type == "array" and length > 0 and length == (unique | length))
          and (.checks | type == "array")
          and ((.contexts | sort) == (.checks | map(.context) | sort))
          and all(.checks[]; .app_id == $app_id)
        ' "$frozen/classic-restore.json" >/dev/null || \
        ! validate_classic_live_schema "$frozen/classic-live.json"; then
        echo "FAIL: recovery snapshot schema or payload validation failed: $snapshot" >&2
        return 1
    fi
    manifest_repo="$(jq -r .repository "$frozen/manifest.json")"
    [[ "$manifest_repo" == "$repo" ]] || {
        echo "FAIL: recovery snapshot belongs to $manifest_repo, not $repo" >&2
        return 1
    }
    policy_sha="$(jq -r .live_main_sha "$frozen/manifest.json")"
    if ! gh api "repos/$repo/commits/main" > "$frozen/live-main-current.json" || \
        [[ "$(jq -r .sha "$frozen/live-main-current.json")" != "$policy_sha" ]]; then
        echo "FAIL: recovery snapshot policy commit does not match current live main: $policy_sha" >&2
        return 1
    fi
    if ! git -C "$repo_root" cat-file -e "$policy_sha^{commit}" 2>/dev/null || \
        ! git -C "$repo_root" show "$policy_sha:.github/check-identities.json" \
            > "$frozen/check-identities-reviewed.json" || \
        ! git -C "$repo_root" show "$policy_sha:.github/rulesets/main-integrity.json" \
            > "$frozen/integrity-reviewed.json"; then
        echo "FAIL: recovery snapshot's committed policy source is unavailable: $policy_sha" >&2
        return 1
    fi
    if ! jq -e '
      . as $root
      | type == "object"
      and (keys | sort) == ([
        "legacyEmitted", "replacements", "required", "schema", "stage"
      ] | sort)
      and .schema == 2
      and .stage == "stable-required-live-applied"
      and (.legacyEmitted | type == "array" and length > 0 and length == (unique | length))
      and all(.legacyEmitted[]; type == "string" and length > 0)
      and (.required | type == "array" and length > 0 and length == (unique | length))
      and all(.required[]; type == "string" and length > 0)
      and (.replacements | type == "array" and length > 0 and length == (unique | length))
      and all(.replacements[];
        type == "object"
        and (keys | sort) == (["legacy", "logical"] | sort)
        and (.legacy | type == "string" and length > 0)
        and (.logical | type == "string" and length > 0))
      and ((.replacements | map(.legacy)) | length == (unique | length))
      and ((.replacements | map(.logical)) | length == (unique | length))
      and (.replacements | all(
        (.legacy as $legacy | $legacy | IN($root.legacyEmitted[]))
        and (.logical as $logical | $logical | IN($root.required[]))))
      and ((.legacyEmitted - (.replacements | map(.legacy)) | sort) ==
        (.required - (.replacements | map(.logical)) | sort))
    ' "$frozen/check-identities-reviewed.json" >/dev/null; then
        echo "FAIL: recovery snapshot's committed check identity source is malformed: $policy_sha" >&2
        return 1
    fi
    if ! jq -e --slurpfile identities "$frozen/check-identities-reviewed.json" \
        --argjson app_id "$github_actions_app_id" '
      type == "object"
      and .name == "Protect main: integrity"
      and .target == "branch"
      and .enforcement == "active"
      and .bypass_actors == []
      and .conditions.ref_name.include == ["refs/heads/main"]
      and ([.rules[] | select(.type == "required_status_checks")] | length) == 1
      and ([.rules[] | select(.type == "required_status_checks")][0].parameters |
        .strict_required_status_checks_policy == true
        and .do_not_enforce_on_create == false
        and .required_status_checks ==
          ($identities[0].required | map({context: ., integration_id: $app_id})))
    ' "$frozen/integrity-reviewed.json" >/dev/null; then
        echo "FAIL: recovery snapshot's committed integrity and check identity sources disagree: $policy_sha" >&2
        return 1
    fi
    stage="$(jq -r .stage "$frozen/manifest.json")"
    integrity_id="$(jq -r .integrity_ruleset_id "$frozen/manifest.json")"
    if ! gh api "repos/$repo/rulesets?includes_parents=false" \
        > "$frozen/live-rulesets.json"; then
        echo "FAIL: could not verify the live integrity ruleset identity before recovery." >&2
        return 1
    fi
    if ! jq -e \
        --arg repo "$repo" \
        --argjson integrity_id "$integrity_id" '
          [.[] | select(.name == "Protect main: integrity")] as $matches
          | ($matches | length) == 1
          and $matches[0].id == $integrity_id
          and $matches[0].source == $repo
          and $matches[0].source_type == "Repository"
          and $matches[0].target == "branch"
          and $matches[0].enforcement == "active"
        ' "$frozen/live-rulesets.json" >/dev/null; then
        echo "FAIL: recovery snapshot integrity ruleset ID does not match the unique live integrity ruleset." >&2
        return 1
    fi

    if [[ "$stage" == "legacy" ]]; then
        expected_pin=false
        jq -r '.legacyEmitted[]' "$frozen/check-identities-reviewed.json" \
            > "$frozen/stage-contexts.txt"
        jq --slurpfile identities "$frozen/check-identities-reviewed.json" \
            --argjson app_id "$github_actions_app_id" '
          (.rules[] | select(.type == "required_status_checks").parameters.required_status_checks) =
            ($identities[0].legacyEmitted | map({context: ., integration_id: $app_id}))
        ' "$frozen/integrity-reviewed.json" > "$frozen/integrity-stage-source.json"
        ruleset_restore_payload "$frozen/integrity-stage-source.json" \
            > "$frozen/integrity-expected.json"
    else
        expected_pin=true
        jq -r '.required[]' "$frozen/check-identities-reviewed.json" \
            > "$frozen/stage-contexts.txt"
        ruleset_restore_payload "$frozen/integrity-reviewed.json" \
            > "$frozen/integrity-expected.json"
    fi
    build_classic_payload_from_file "$frozen/stage-contexts.txt" \
        > "$frozen/classic-expected.json"
    build_classic_state_from_file "$frozen/stage-contexts.txt" \
        > "$frozen/classic-state-expected.json"
    jq -n --argjson pin "$expected_pin" \
        '{enabled: true, allowed_actions: "all", sha_pinning_required: $pin}' \
        > "$frozen/actions-expected.json"
    if ! normalize_classic_state "$frozen/classic-live.json" \
        > "$frozen/classic-live-state.json" || \
        ! classic_restore_payload "$frozen/classic-live.json" \
        > "$frozen/classic-from-live.json" || \
        ! json_equal "$frozen/actions-restore.json" "$frozen/actions-expected.json" || \
        ! json_equal "$frozen/integrity-restore.json" "$frozen/integrity-expected.json" || \
        ! json_equal "$frozen/classic-restore.json" "$frozen/classic-expected.json" || \
        ! json_equal "$frozen/classic-live-state.json" "$frozen/classic-state-expected.json" || \
        ! json_equal "$frozen/classic-from-live.json" "$frozen/classic-restore.json"; then
        echo "FAIL: recovery snapshot policy does not match manifest stage '$stage': $snapshot" >&2
        return 1
    fi

    chmod 400 "$frozen"/*
    chmod 500 "$frozen"

    echo "Restoring repository safeguards from $snapshot" >&2
    set +e
    gh_api_json_file PATCH "repos/$repo/branches/main/protection/required_status_checks" "$frozen/classic-restore.json" >/dev/null || failures=1
    gh_api_json_file PUT "repos/$repo/rulesets/$integrity_id" "$frozen/integrity-restore.json" >/dev/null || failures=1
    gh_api_json_file PUT "repos/$repo/actions/permissions" "$frozen/actions-restore.json" >/dev/null || failures=1
    set -e
    if [[ "$failures" -ne 0 ]]; then
        echo "FAIL: safeguard rollback was incomplete." >&2
        echo "      Retry: $0 --restore '$snapshot' '$repo'" >&2
        return 1
    fi

    verify_dir="$(mktemp -d)"
    set +e
    gh api "repos/$repo/actions/permissions" > "$verify_dir/actions.json" || { failures=1; readback_error=1; }
    gh api "repos/$repo/rulesets/$integrity_id" > "$verify_dir/integrity.json" || { failures=1; readback_error=1; }
    gh api "repos/$repo/branches/main/protection" > "$verify_dir/classic.json" || { failures=1; readback_error=1; }
    if [[ "$failures" -eq 0 ]]; then
        jq '{enabled, allowed_actions, sha_pinning_required}' "$verify_dir/actions.json" \
            > "$verify_dir/actions-state.json" || { failures=1; readback_error=1; }
        normalize_ruleset "$verify_dir/integrity.json" \
            > "$verify_dir/integrity-state.json" || { failures=1; readback_error=1; }
        normalize_ruleset "$frozen/integrity-restore.json" \
            > "$verify_dir/integrity-snapshot.json" || { failures=1; readback_error=1; }
        normalize_classic_state "$verify_dir/classic.json" \
            > "$verify_dir/classic-state.json" || { failures=1; readback_error=1; }
        normalize_classic_state "$frozen/classic-live.json" \
            > "$verify_dir/classic-snapshot.json" || { failures=1; readback_error=1; }
    fi
    if [[ "$failures" -eq 0 ]] && { \
        ! json_equal "$verify_dir/actions-state.json" "$frozen/actions-restore.json" || \
        ! json_equal "$verify_dir/integrity-state.json" "$verify_dir/integrity-snapshot.json" || \
        ! json_equal "$verify_dir/classic-state.json" "$verify_dir/classic-snapshot.json"; \
    }; then
        failures=1
        readback_mismatch=1
    fi
    set -e
    rm -rf "$verify_dir"
    verify_dir=""
    if [[ "$failures" -ne 0 ]]; then
        if [[ "$readback_error" -ne 0 ]]; then
            echo "FAIL: rollback writes completed but verification readback failed." >&2
        elif [[ "$readback_mismatch" -ne 0 ]]; then
            echo "FAIL: rollback API calls returned success but readback differs from the snapshot." >&2
        else
            echo "FAIL: rollback verification failed for an unknown reason." >&2
        fi
        echo "      Retry: $0 --restore '$snapshot' '$repo'" >&2
        return 1
    fi
    echo "Repository safeguard cutover resources restored and verified from $snapshot" >&2
)

handle_exit() {
    local rc=$? rollback_rc=0
    if [[ "$exit_handler_active" -eq 1 ]]; then
        exit "$rc"
    fi
    exit_handler_active=1
    trap - EXIT HUP INT TERM
    if [[ "$transaction_active" -eq 1 && -n "$recovery_dir" ]]; then
        echo "Safeguard apply failed after live mutation; attempting automatic rollback." >&2
        restore_snapshot "$recovery_dir" || rollback_rc=$?
        if [[ "$rollback_rc" -ne 0 ]]; then
            echo "RECOVERY REQUIRED: $0 --restore '$recovery_dir' '$repo'" >&2
        else
            echo "The previous three-resource cutover state was restored. Snapshot retained: $recovery_dir" >&2
        fi
    fi
    if [[ "$recovery_retained" -eq 0 && -n "$recovery_dir" && -d "$recovery_dir" ]]; then
        rm -rf "$recovery_dir"
    fi
    cleanup_temporary_dirs
    exit "$rc"
}

trap handle_exit EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ -n "$restore_dir" ]]; then
    restore_dir="$(cd "$restore_dir" 2>/dev/null && pwd -P)" || {
        echo "FAIL: recovery snapshot directory does not exist." >&2
        exit 2
    }
    restore_snapshot "$restore_dir"
    transaction_active=0
    echo "Repository safeguard cutover resources restored and verified."
    exit 0
fi

for file in "$check_identities" "$integrity_ruleset" "$review_ruleset" "$owner_updates_ruleset"; do
    [[ -f "$file" ]] || { echo "FAIL: missing safeguard source: $file" >&2; exit 3; }
done

create_private_temp_dir preflight_dir
capture_and_validate_live_state "$preflight_dir"
stage="$(cat "$preflight_dir/stage")"
if [[ "$preflight_only" -eq 1 ]]; then
    echo "Safeguard preflight passed for exact live main in '$stage' stage; no local snapshot or live state changed."
    exit 0
fi

if [[ "$stage" == "stable" ]]; then
    echo "Repository safeguards already match the stable checked-in posture; no mutation was needed."
    exit 0
fi

git_metadata_path="$(git -C "$repo_root" rev-parse --git-path dotfiles-safeguards)"
case "$git_metadata_path" in
    /*) ;;
    *) git_metadata_path="$repo_root/$git_metadata_path" ;;
esac
mkdir -p "$git_metadata_path"
chmod 700 "$git_metadata_path"
recovery_dir="$(mktemp -d "$git_metadata_path/recovery.XXXXXX")"
cp -R "$preflight_dir/." "$recovery_dir/"
chmod -R go-rwx "$recovery_dir"
printf '%s\n' \
    "This directory is the pre-mutation recovery snapshot for $repo." \
    "Restore with:" \
    "$0 --restore '$recovery_dir' '$repo'" \
    > "$recovery_dir/RECOVERY.txt"
verify_snapshot_unchanged
prepare_transaction_payloads

echo "Applying stable required-check and SHA-pinning safeguards to $repo"
echo "Recovery snapshot: $recovery_dir"
transaction_active=1
recovery_retained=1
gh_api_json_file PUT "repos/$repo/actions/permissions" "$transaction_dir/actions-desired.json" >/dev/null
gh_api_json_file PUT "repos/$repo/rulesets/$(jq -r .integrity_ruleset_id "$transaction_dir/manifest.json")" "$transaction_dir/integrity-desired.json" >/dev/null
gh_api_json_file PATCH "repos/$repo/branches/main/protection/required_status_checks" "$transaction_dir/classic-desired.json" >/dev/null

create_private_temp_dir postflight_dir
capture_and_validate_live_state "$postflight_dir" "$transaction_dir"
if [[ "$(cat "$postflight_dir/stage")" != "stable" ]]; then
    rm -rf "$postflight_dir"
    echo "FAIL: post-apply readback did not reach the stable stage." >&2
    exit 5
fi
for state_file in repository-state rulesets-state review-live-normalized owner-live-normalized; do
    expected_file="$transaction_dir/${state_file}-expected.json"
    if [[ "$state_file" == "review-live-normalized" ]]; then
        expected_file="$transaction_dir/review-expected-normalized.json"
    elif [[ "$state_file" == "owner-live-normalized" ]]; then
        expected_file="$transaction_dir/owner-expected-normalized.json"
    fi
    if ! json_equal "$postflight_dir/$state_file.json" "$expected_file"; then
        echo "FAIL: post-apply unchanged $state_file state differs from the frozen second capture." >&2
        exit 5
    fi
done
verify_local_boundary "$postflight_dir"
rm -rf "$postflight_dir"
transaction_active=0

echo "Repository safeguards applied and verified."
echo "Pre-change recovery snapshot retained at: $recovery_dir"
