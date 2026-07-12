#!/usr/bin/env bash
# The mock gh command is invoked indirectly by the production script.
# shellcheck disable=SC2034,SC2317,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fixture="$WORK/repo"
mkdir -p "$fixture/scripts" "$fixture/.github/rulesets" "$fixture/.github/workflows"
cp "$REPO_ROOT/scripts/apply-repo-safeguards.sh" "$fixture/scripts/"
cp "$REPO_ROOT/scripts/ci-logical-proof.sh" "$fixture/scripts/"
cp "$REPO_ROOT/.github/check-identities.json" "$fixture/.github/"
cp "$REPO_ROOT/.github/settings.yml" "$fixture/.github/"
cp "$REPO_ROOT/.github/rulesets/"*.json "$fixture/.github/rulesets/"
cp "$REPO_ROOT/.github/workflows/e2e-install.yml" "$fixture/.github/workflows/"
cp "$REPO_ROOT/.github/workflows/nix.yml" "$fixture/.github/workflows/"
cp "$REPO_ROOT/.github/workflows/test.yml" "$fixture/.github/workflows/"
(
    cd "$fixture"
    git init -q
    git config user.name test
    git config user.email test@example.invalid
    git add .
    git commit -qm fixture
    git branch -M main
    git remote add origin https://github.com/owner/repo.git
)
fixture_head="$(git -C "$fixture" rev-parse HEAD)"

legacy_contexts_json="$(jq -c '.legacyEmitted' "$fixture/.github/check-identities.json")"
stable_contexts_json="$(jq -c '.required' "$fixture/.github/check-identities.json")"

write_classic_state() {
    local contexts="$1" output="$2"
    jq -n --argjson contexts "$contexts" '{
      required_status_checks: {
        strict: true,
        contexts: $contexts,
        checks: ($contexts | map({context: ., app_id: 15368}))
      },
      enforce_admins: {enabled: true},
      required_pull_request_reviews: null,
      restrictions: null,
      required_linear_history: {enabled: true},
      allow_force_pushes: {enabled: false},
      allow_deletions: {enabled: false},
      required_conversation_resolution: {enabled: true},
      required_signatures: {enabled: false},
      block_creations: {enabled: false},
      lock_branch: {enabled: false},
      allow_fork_syncing: {enabled: false}
    }' > "$output"
}

write_jobs() {
    local output="$1"
    shift
    printf '%s\n' "$@" | jq -Rn '
      [inputs | {name: ., status: "completed", conclusion: "success", steps: []}]
      | {jobs: .}
    ' > "$output"
}

make_state() {
    local state="$1"
    mkdir -p "$state"
    jq -n --arg sha "$fixture_head" '{sha: $sha}' > "$state/live-main.json"
    jq -n '{
      full_name: "owner/repo",
      private: false,
      visibility: "public",
      default_branch: "main",
      allow_merge_commit: false,
      allow_squash_merge: true,
      allow_rebase_merge: false,
      allow_auto_merge: false,
      delete_branch_on_merge: true,
      security_and_analysis: {
        dependabot_security_updates: {status: "enabled"},
        secret_scanning: {status: "enabled"},
        secret_scanning_push_protection: {status: "enabled"}
      }
    }' > "$state/repository.json"
    jq -n '{enabled: true, allowed_actions: "all", sha_pinning_required: false}' > "$state/actions.json"
    jq --slurpfile identities "$fixture/.github/check-identities.json" '
      (.rules[] | select(.type == "required_status_checks").parameters.required_status_checks) =
        ($identities[0].legacyEmitted | map({context: ., integration_id: 15368}))
    ' "$fixture/.github/rulesets/main-integrity.json" > "$state/integrity.json"
    cp "$fixture/.github/rulesets/main-review.json" "$state/review.json"
    cp "$fixture/.github/rulesets/main-owner-updates.json" "$state/owner.json"
    jq -n '{
      rulesets: [
        {id: 101, name: "Protect main: integrity", source: "owner/repo", source_type: "Repository", target: "branch", enforcement: "active"},
        {id: 102, name: "Protect main: review", source: "owner/repo", source_type: "Repository", target: "branch", enforcement: "active"},
        {id: 103, name: "Protect main: owner updates", source: "owner/repo", source_type: "Repository", target: "branch", enforcement: "active"}
      ]
    } | .rulesets' > "$state/rulesets.json"
    write_classic_state "$legacy_contexts_json" "$state/classic.json"

    jq -n --arg sha "$fixture_head" '{workflow_runs: [{
      id: 201, run_number: 1, event: "push", status: "completed", conclusion: "success",
      head_branch: "main", head_sha: $sha, path: ".github/workflows/test.yml",
      repository: {full_name: "owner/repo"}
    }]}' > "$state/test-runs.json"
    jq -n --arg sha "$fixture_head" '{workflow_runs: [{
      id: 202, run_number: 1, event: "push", status: "completed", conclusion: "success",
      head_branch: "main", head_sha: $sha, path: ".github/workflows/nix.yml",
      repository: {full_name: "owner/repo"}
    }]}' > "$state/nix-runs.json"
    jq -n --arg sha "$fixture_head" '{workflow_runs: [{
      id: 203, run_number: 1, event: "workflow_dispatch", status: "completed", conclusion: "success",
      head_branch: "main", head_sha: $sha, path: ".github/workflows/e2e-install.yml",
      repository: {full_name: "owner/repo"}
    }]}' > "$state/e2e-install-runs.json"

    write_jobs "$state/test-jobs.json" \
        ubuntu macos windows chezmoi-parity chezmoi-parity-macos chezmoi-parity-windows
    write_jobs "$state/nix-jobs.json" \
        "nix flake check (ubuntu-24.04)" "nix flake check (macos-26)" \
        "nix flake check / linux" "nix flake check / macos"
    write_jobs "$state/e2e-jobs.json" \
        "e2e containers / ubuntu-24.04" "setup.sh / ubuntu-24.04" \
        "setup.sh / macos-26" "setup.ps1 / windows-2025" \
        "e2e containers / linux" "setup.sh / linux" \
        "setup.sh / macos" "setup.ps1 / windows"
    jq '
      .jobs |= map(
        if (.name == "setup.sh / ubuntu-24.04" or
            .name == "setup.sh / macos-26" or
            .name == "setup.ps1 / windows-2025")
        then .steps = [{name: "PR-only cache: fixture", conclusion: "skipped"}]
        else . end)
    ' "$state/e2e-jobs.json" > "$state/e2e-jobs.tmp"
    mv "$state/e2e-jobs.tmp" "$state/e2e-jobs.json"

    {
        printf '%s\n' ubuntu macos windows chezmoi-parity chezmoi-parity-macos chezmoi-parity-windows \
            | jq -Rn '[inputs | {name: ., run_id: 201}]'
        printf '%s\n' "nix flake check (ubuntu-24.04)" "nix flake check (macos-26)" \
            "nix flake check / linux" "nix flake check / macos" \
            | jq -Rn '[inputs | {name: ., run_id: 202}]'
        printf '%s\n' "e2e containers / ubuntu-24.04" "setup.sh / ubuntu-24.04" \
            "setup.sh / macos-26" "setup.ps1 / windows-2025" \
            "e2e containers / linux" "setup.sh / linux" "setup.sh / macos" "setup.ps1 / windows" \
            | jq -Rn '[inputs | {name: ., run_id: 203}]'
    } | jq -s --arg repo owner/repo '
      add | {check_runs: map({
        name,
        status: "completed",
        conclusion: "success",
        app: {id: 15368, slug: "github-actions"},
        details_url: ("https://github.com/" + $repo + "/actions/runs/" + (.run_id | tostring) + "/job/1")
      })}
    ' > "$state/check-runs.json"
}

base_state="$WORK/base-state"
make_state "$base_state"

mkdir -p "$WORK/bin"
cat > "$WORK/bin/gh" <<'MOCK_GH'
#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "auth" && "${2:-}" == "status" ]]; then
    exit 0
fi
if [[ "${1:-}" == "repo" && "${2:-}" == "view" ]]; then
    printf '%s\n' owner/repo
    exit 0
fi
[[ "${1:-}" == "api" ]] || { echo "unexpected gh command: $*" >&2; exit 91; }
shift

method=GET
path=""
input=""
jq_filter=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -X)
            method="$2"
            shift 2
            ;;
        --input)
            input="$2"
            shift 2
            ;;
        --jq)
            jq_filter="$2"
            shift 2
            ;;
        --silent)
            shift
            ;;
        -*)
            shift
            ;;
        *)
            if [[ -z "$path" ]]; then path="$1"; fi
            shift
            ;;
    esac
done

state="${TEST_STATE_DIR:?}"
mutation_log="${TEST_MUTATION_LOG:?}"

if [[ "$method" == "GET" && -n "${TEST_FAIL_READ_AFTER_MUTATION_PATH:-}" &&
      "$path" == "$TEST_FAIL_READ_AFTER_MUTATION_PATH" && -s "$mutation_log" ]]; then
    exit 95
fi

if [[ "$method" == "GET" && "$path" == "repos/owner/repo" &&
      ( -n "${TEST_MUTATE_BEFORE_SECOND_CAPTURE:-}" ||
        -n "${TEST_PRIVATE_BEFORE_SECOND_CAPTURE:-}" ) ]]; then
    read_count=0
    if [[ -f "$state/repository-read-count" ]]; then
        read_count="$(cat "$state/repository-read-count")"
    fi
    read_count=$((read_count + 1))
    printf '%s\n' "$read_count" > "$state/repository-read-count"
    if [[ "$read_count" -eq 2 ]]; then
        if [[ -n "${TEST_MUTATE_BEFORE_SECOND_CAPTURE:-}" ]]; then
            jq '.delete_branch_on_merge = false' "$state/repository.json" > "$state/repository.tmp"
        else
            jq '.private = true | .visibility = "private"' "$state/repository.json" > "$state/repository.tmp"
        fi
        mv "$state/repository.tmp" "$state/repository.json"
    fi
fi

if [[ "$method" == "GET" && "$path" == "repos/owner/repo/rulesets?includes_parents=false" &&
      -n "${TEST_MUTATE_APPLY_SOURCE_AFTER_POSTFLIGHT_BOUNDARY:-}" &&
      -s "$mutation_log" && ! -e "$state/postflight-source-mutated" ]]; then
    : > "$state/postflight-source-mutated"
    jq '
      (.rules[] | select(.type == "required_status_checks") |
        .parameters.required_status_checks) =
      [{context: "postflight-worktree-only-context", integration_id: 15368}]
    ' \
        "$TEST_MUTATE_APPLY_SOURCE_AFTER_POSTFLIGHT_BOUNDARY" \
        > "$TEST_MUTATE_APPLY_SOURCE_AFTER_POSTFLIGHT_BOUNDARY.tmp"
    mv "$TEST_MUTATE_APPLY_SOURCE_AFTER_POSTFLIGHT_BOUNDARY.tmp" \
        "$TEST_MUTATE_APPLY_SOURCE_AFTER_POSTFLIGHT_BOUNDARY"
fi

if [[ "$method" != "GET" ]]; then
    printf '%s %s\n' "$method" "$path" >> "$mutation_log"
    if [[ -n "${TEST_FAIL_ONCE_PATH:-}" && "$path" == "$TEST_FAIL_ONCE_PATH" && ! -e "$state/failure-used" ]]; then
        : > "$state/failure-used"
        exit 90
    fi
    if [[ -n "${TEST_FAIL_AFTER_PATH:-}" && -e "$state/failure-used" && "$path" == "$TEST_FAIL_AFTER_PATH" ]]; then
        exit 92
    fi
    if [[ -n "${TEST_MUTATE_SNAPSHOT_AFTER_FIRST_WRITE:-}" && ! -e "$state/snapshot-mutated" ]]; then
        : > "$state/snapshot-mutated"
        jq '
          (.rules[] | select(.type == "required_status_checks") |
            .parameters.required_status_checks) =
          [{context: "unreviewed-after-validation", integration_id: 15368}]
        ' "$TEST_MUTATE_SNAPSHOT_AFTER_FIRST_WRITE/integrity-restore.json" \
            > "$TEST_MUTATE_SNAPSHOT_AFTER_FIRST_WRITE/integrity-restore.tmp"
        mv "$TEST_MUTATE_SNAPSHOT_AFTER_FIRST_WRITE/integrity-restore.tmp" \
            "$TEST_MUTATE_SNAPSHOT_AFTER_FIRST_WRITE/integrity-restore.json"
    fi
    if [[ -n "${TEST_MUTATE_APPLY_SOURCE_AFTER_FIRST_WRITE:-}" && ! -e "$state/apply-source-mutated" ]]; then
        : > "$state/apply-source-mutated"
        jq '
          (.rules[] | select(.type == "required_status_checks") |
            .parameters.required_status_checks) =
          [{context: "unreviewed-after-validation", integration_id: 15368}]
        ' "$TEST_MUTATE_APPLY_SOURCE_AFTER_FIRST_WRITE" \
            > "$TEST_MUTATE_APPLY_SOURCE_AFTER_FIRST_WRITE.tmp"
        mv "$TEST_MUTATE_APPLY_SOURCE_AFTER_FIRST_WRITE.tmp" \
            "$TEST_MUTATE_APPLY_SOURCE_AFTER_FIRST_WRITE"
    fi
    case "$path" in
        repos/owner/repo/actions/permissions)
            cp "$input" "$state/actions.json"
            ;;
        repos/owner/repo/rulesets/101)
            if [[ -n "${TEST_CAPTURE_INTEGRITY_INPUT:-}" && ! -e "$TEST_CAPTURE_INTEGRITY_INPUT" ]]; then
                cp "$input" "$TEST_CAPTURE_INTEGRITY_INPUT"
            fi
            cp "$input" "$state/integrity.json"
            ;;
        repos/owner/repo/branches/main/protection/required_status_checks)
            jq --slurpfile required "$input" \
                '.required_status_checks = $required[0]' \
                "$state/classic.json" > "$state/classic.tmp"
            mv "$state/classic.tmp" "$state/classic.json"
            ;;
        *)
            echo "unexpected mutation: $method $path" >&2
            exit 93
            ;;
    esac
    printf '{}\n'
    exit 0
fi

case "$path" in
    repos/owner/repo) file="$state/repository.json" ;;
    repos/owner/repo/commits/main) file="$state/live-main.json" ;;
    'repos/owner/repo/rulesets?includes_parents=false') file="$state/rulesets.json" ;;
    repos/owner/repo/rulesets/101) file="$state/integrity.json" ;;
    repos/owner/repo/rulesets/102) file="$state/review.json" ;;
    repos/owner/repo/rulesets/103) file="$state/owner.json" ;;
    repos/owner/repo/branches/main/protection) file="$state/classic.json" ;;
    repos/owner/repo/actions/permissions) file="$state/actions.json" ;;
    repos/owner/repo/vulnerability-alerts|repos/owner/repo/automated-security-fixes) exit 0 ;;
    repos/owner/repo/commits/*/check-runs*) file="$state/check-runs.json" ;;
    repos/owner/repo/actions/workflows/test.yml/runs*) file="$state/test-runs.json" ;;
    repos/owner/repo/actions/workflows/nix.yml/runs*) file="$state/nix-runs.json" ;;
    repos/owner/repo/actions/workflows/e2e-install.yml/runs*) file="$state/e2e-install-runs.json" ;;
    repos/owner/repo/actions/runs/201/jobs*) file="$state/test-jobs.json" ;;
    repos/owner/repo/actions/runs/202/jobs*) file="$state/nix-jobs.json" ;;
    repos/owner/repo/actions/runs/203/jobs*) file="$state/e2e-jobs.json" ;;
    *) echo "unexpected read: $path" >&2; exit 94 ;;
esac

if [[ -n "$jq_filter" ]]; then
    jq -r "$jq_filter" "$file"
else
    cat "$file"
fi
MOCK_GH
chmod +x "$WORK/bin/gh"

new_case_state() {
    local name="$1" state
    state="$WORK/state-$name"
    cp -R "$base_state" "$state"
    printf '%s\n' "$state"
}

run_safeguards() {
    local state="$1" mutation_log="$2"
    shift 2
    TEST_STATE_DIR="$state" \
    TEST_MUTATION_LOG="$mutation_log" \
    PATH="$WORK/bin:$PATH" \
        "$fixture/scripts/apply-repo-safeguards.sh" "$@" owner/repo
}

expect_failure() {
    local expected="$1"
    shift
    local output rc
    set +e
    output="$("$@" 2>&1)"
    rc=$?
    set -e
    [[ "$rc" -ne 0 ]] || {
        echo "FAIL: command unexpectedly succeeded: $*" >&2
        exit 1
    }
    grep -F "$expected" <<<"$output" >/dev/null || {
        echo "FAIL: expected failure text not found: $expected" >&2
        printf '%s\n' "$output" >&2
        exit 1
    }
}

expect_restore_rejected_without_mutation() {
    local expected="$1" state="$2" snapshot="$3"
    : > "$mutation_log"
    expect_failure "$expected" \
        run_safeguards "$state" "$mutation_log" --restore "$snapshot"
    [[ ! -s "$mutation_log" ]] || {
        echo "FAIL: invalid recovery material mutated live state: $snapshot" >&2
        cat "$mutation_log" >&2
        exit 1
    }
}

mutation_log="$WORK/mutations.log"
: > "$mutation_log"
state="$(new_case_state preflight)"
output="$(run_safeguards "$state" "$mutation_log" --preflight-only)"
grep -F "no local snapshot or live state changed" <<<"$output" >/dev/null
[[ ! -s "$mutation_log" ]] || { echo "FAIL: preflight-only mutated live state" >&2; exit 1; }
[[ ! -e "$fixture/.git/dotfiles-safeguards" ]] || { echo "FAIL: preflight-only created a recovery snapshot" >&2; exit 1; }
echo "ok  : exact legacy live posture and provenance pass without mutation"

state="$(new_case_state omitted-disabled-classic-sections)"
jq 'del(.required_pull_request_reviews, .restrictions)' \
    "$state/classic.json" > "$state/classic.tmp"
mv "$state/classic.tmp" "$state/classic.json"
output="$(run_safeguards "$state" "$mutation_log" --preflight-only)"
grep -F "no local snapshot or live state changed" <<<"$output" >/dev/null
[[ ! -s "$mutation_log" ]] || {
    echo "FAIL: omitted disabled classic sections caused preflight mutation" >&2
    exit 1
}
echo "ok  : GitHub-omitted disabled review and restriction sections normalize to null"

git -C "$fixture" switch -qc topic
state="$(new_case_state wrong-branch)"
expect_failure "requires the checked-out local branch to be main" \
    run_safeguards "$state" "$mutation_log" --preflight-only
git -C "$fixture" switch -q main
git -C "$fixture" remote set-url origin https://github.com/other/repo.git
state="$(new_case_state wrong-remote)"
expect_failure "checkout origin is not the requested" \
    run_safeguards "$state" "$mutation_log" --preflight-only
git -C "$fixture" remote set-url origin https://github.com/owner/repo.git
[[ ! -s "$mutation_log" ]] || { echo "FAIL: identity failure mutated live state" >&2; exit 1; }
echo "ok  : wrong branch and remote identity fail before mutation"

state="$(new_case_state duplicate-ruleset)"
jq '. + [.[0]]' "$state/rulesets.json" > "$state/rulesets.tmp"
mv "$state/rulesets.tmp" "$state/rulesets.json"
expect_failure "exact three unique active repository rulesets" \
    run_safeguards "$state" "$mutation_log" --preflight-only
echo "ok  : duplicate rulesets fail before mutation"

state="$(new_case_state wrong-app)"
jq '(.check_runs[] | select(.name == "setup.sh / linux").app.id) = 999' \
    "$state/check-runs.json" > "$state/check-runs.tmp"
mv "$state/check-runs.tmp" "$state/check-runs.json"
expect_failure "not uniquely bound to GitHub Actions app 15368" \
    run_safeguards "$state" "$mutation_log" --preflight-only

state="$(new_case_state wrong-event)"
jq '.workflow_runs[0].event = "pull_request"' "$state/e2e-install-runs.json" > "$state/e2e.tmp"
mv "$state/e2e.tmp" "$state/e2e-install-runs.json"
expect_failure "no successful .github/workflows/e2e-install.yml run with allowed event provenance" \
    run_safeguards "$state" "$mutation_log" --preflight-only

state="$(new_case_state cached-e2e)"
jq '(.jobs[] | select(.name == "setup.sh / linux") | .steps) = [{name: "PR-only cache: fixture", conclusion: "success"}]' \
    "$state/e2e-jobs.json" > "$state/e2e.tmp"
mv "$state/e2e.tmp" "$state/e2e-jobs.json"
expect_failure "did not skip every broad actions/cache step" \
    run_safeguards "$state" "$mutation_log" --preflight-only
[[ ! -s "$mutation_log" ]] || { echo "FAIL: provenance failure mutated live state" >&2; exit 1; }
echo "ok  : wrong app, event, and cache provenance fail before mutation"

state="$(new_case_state unexpected-contexts)"
jq 'del(.required_status_checks.contexts[-1], .required_status_checks.checks[-1])' \
    "$state/classic.json" > "$state/classic.tmp"
mv "$state/classic.tmp" "$state/classic.json"
expect_failure "not one exact, internally consistent legacy-or-stable stage" \
    run_safeguards "$state" "$mutation_log" --preflight-only

cp "$fixture/.github/settings.yml" "$WORK/settings.backup"
printf '\n# unreviewed\n' >> "$fixture/.github/settings.yml"
state="$(new_case_state dirty-source)"
expect_failure "reviewed safeguard/proof sources differ from exact live main" \
    run_safeguards "$state" "$mutation_log" --preflight-only
cp "$WORK/settings.backup" "$fixture/.github/settings.yml"
[[ ! -s "$mutation_log" ]] || { echo "FAIL: live/source drift mutated live state" >&2; exit 1; }
echo "ok  : unexpected live contexts and dirty reviewed sources fail before mutation"

state="$(new_case_state concurrent-change)"
: > "$mutation_log"
TEST_MUTATE_BEFORE_SECOND_CAPTURE=1 \
    expect_failure "live repository/merge/security settings differ" \
    run_safeguards "$state" "$mutation_log"
[[ ! -s "$mutation_log" ]] || { echo "FAIL: concurrent preflight change was detected after mutation" >&2; exit 1; }
echo "ok  : second full read detects concurrent live change before mutation"

state="$(new_case_state failed-second-capture-cleanup)"
: > "$mutation_log"
capture_tmp="$WORK/capture-tmp"
mkdir -p "$capture_tmp"
saved_tmpdir="${TMPDIR-}"
export TMPDIR="$capture_tmp"
TEST_PRIVATE_BEFORE_SECOND_CAPTURE=1 \
    expect_failure "public-repo posture" \
    run_safeguards "$state" "$mutation_log"
if [[ -n "$saved_tmpdir" ]]; then
    export TMPDIR="$saved_tmpdir"
else
    unset TMPDIR
fi
[[ ! -s "$mutation_log" ]] || {
    echo "FAIL: failed second capture mutated live state" >&2
    exit 1
}
[[ -z "$(find "$capture_tmp" -mindepth 1 -maxdepth 1 -print -quit)" ]] || {
    echo "FAIL: failed second capture leaked a temporary directory" >&2
    find "$capture_tmp" -mindepth 1 -maxdepth 1 -print >&2
    exit 1
}
[[ -z "$(find "$fixture/.git/dotfiles-safeguards" -mindepth 1 -maxdepth 1 -type d -print -quit 2>/dev/null)" ]] || {
    echo "FAIL: pre-mutation failure retained an orphan recovery snapshot" >&2
    exit 1
}
echo "ok  : failed second capture cleans temporary and pre-mutation recovery state"

state="$(new_case_state frozen-apply-source)"
: > "$mutation_log"
cp "$fixture/.github/rulesets/main-integrity.json" "$WORK/integrity-source.backup"
TEST_MUTATE_APPLY_SOURCE_AFTER_FIRST_WRITE="$fixture/.github/rulesets/main-integrity.json" \
TEST_CAPTURE_INTEGRITY_INPUT="$WORK/published-integrity.json" \
    expect_failure "reviewed safeguard/proof sources differ from exact live main" \
    run_safeguards "$state" "$mutation_log"
cp "$WORK/integrity-source.backup" "$fixture/.github/rulesets/main-integrity.json"
jq -e '
  [.rules[] | select(.type == "required_status_checks") |
    .parameters.required_status_checks[].context] == $stable
' --argjson stable "$stable_contexts_json" "$WORK/published-integrity.json" >/dev/null
jq -e '.sha_pinning_required == false' "$state/actions.json" >/dev/null
jq -e '
  [.rules[] | select(.type == "required_status_checks") |
    .parameters.required_status_checks[].context] == $legacy
' --argjson legacy "$legacy_contexts_json" "$state/integrity.json" >/dev/null
rm -rf "$fixture/.git/dotfiles-safeguards"
echo "ok  : apply publishes only frozen committed integrity bytes after validation"

state="$(new_case_state frozen-postflight-expectations)"
: > "$mutation_log"
cp "$fixture/.github/rulesets/main-integrity.json" "$WORK/postflight-integrity.backup"
TEST_MUTATE_APPLY_SOURCE_AFTER_POSTFLIGHT_BOUNDARY="$fixture/.github/rulesets/main-integrity.json" \
    expect_failure "reviewed safeguard/proof sources differ from exact live main" \
    run_safeguards "$state" "$mutation_log"
cp "$WORK/postflight-integrity.backup" "$fixture/.github/rulesets/main-integrity.json"
jq -e '.sha_pinning_required == false' "$state/actions.json" >/dev/null
jq -e '
  [.rules[] | select(.type == "required_status_checks") |
    .parameters.required_status_checks[].context] == $legacy
' --argjson legacy "$legacy_contexts_json" "$state/integrity.json" >/dev/null
[[ "$(wc -l < "$mutation_log" | tr -d ' ')" == "6" ]] || {
    echo "FAIL: postflight source drift did not trigger one apply plus one rollback" >&2
    cat "$mutation_log" >&2
    exit 1
}
rm -rf "$fixture/.git/dotfiles-safeguards"
echo "ok  : postflight expectations stay frozen after the local-boundary check"

state="$(new_case_state apply)"
: > "$mutation_log"
output="$(run_safeguards "$state" "$mutation_log")"
grep -F "Repository safeguards applied and verified." <<<"$output" >/dev/null
[[ "$(wc -l < "$mutation_log" | tr -d ' ')" == "3" ]] || {
    echo "FAIL: apply did not limit mutation to exactly three resources" >&2
    cat "$mutation_log" >&2
    exit 1
}
grep -Fx "PATCH repos/owner/repo/branches/main/protection/required_status_checks" "$mutation_log" >/dev/null || {
    echo "FAIL: apply did not use the narrow classic status-check endpoint" >&2
    exit 1
}
jq -e '.sha_pinning_required == true' "$state/actions.json" >/dev/null
jq -e '[.rules[] | select(.type == "required_status_checks") | .parameters.required_status_checks[].context] | index("setup.sh / linux") != null' \
    "$state/integrity.json" >/dev/null
jq -e '.required_status_checks.contexts | index("setup.sh / linux") != null' "$state/classic.json" >/dev/null
jq -e '
  .required_signatures.enabled == false
  and .block_creations.enabled == false
  and .lock_branch.enabled == false
  and .allow_fork_syncing.enabled == false
' "$state/classic.json" >/dev/null
snapshot="$(awk -F': ' '/^Recovery snapshot:/ { print $2 }' <<<"$output")"
[[ -f "$snapshot/manifest.json" && -f "$snapshot/RECOVERY.txt" ]] || {
    echo "FAIL: apply did not retain a complete recovery snapshot" >&2
    exit 1
}
echo "ok  : apply snapshots then mutates only the three reviewed cutover resources"

legacy_worktree_drift_snapshot="$WORK/legacy-worktree-drift-snapshot"
cp -R "$snapshot" "$legacy_worktree_drift_snapshot"
cp "$fixture/.github/check-identities.json" "$WORK/check-identities.backup"
jq '.legacyEmitted = ["worktree-only-legacy-context"]' \
    "$fixture/.github/check-identities.json" \
    > "$fixture/.github/check-identities.tmp"
mv "$fixture/.github/check-identities.tmp" \
    "$fixture/.github/check-identities.json"
write_classic_state '["worktree-only-legacy-context"]' \
    "$legacy_worktree_drift_snapshot/classic-live.json"
jq '.required_status_checks' \
    "$legacy_worktree_drift_snapshot/classic-live.json" \
    > "$legacy_worktree_drift_snapshot/classic-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" \
    "$legacy_worktree_drift_snapshot"
cp "$WORK/check-identities.backup" "$fixture/.github/check-identities.json"

stable_worktree_drift_snapshot="$WORK/stable-worktree-drift-snapshot"
cp -R "$snapshot" "$stable_worktree_drift_snapshot"
jq '.stage = "stable"' "$stable_worktree_drift_snapshot/manifest.json" \
    > "$stable_worktree_drift_snapshot/manifest.tmp"
mv "$stable_worktree_drift_snapshot/manifest.tmp" \
    "$stable_worktree_drift_snapshot/manifest.json"
jq '.sha_pinning_required = true' \
    "$stable_worktree_drift_snapshot/actions-restore.json" \
    > "$stable_worktree_drift_snapshot/actions-restore.tmp"
mv "$stable_worktree_drift_snapshot/actions-restore.tmp" \
    "$stable_worktree_drift_snapshot/actions-restore.json"
cp "$fixture/.github/rulesets/main-integrity.json" \
    "$stable_worktree_drift_snapshot/integrity-restore.json"
write_classic_state '["worktree-only-stable-context"]' \
    "$stable_worktree_drift_snapshot/classic-live.json"
jq '.required_status_checks' \
    "$stable_worktree_drift_snapshot/classic-live.json" \
    > "$stable_worktree_drift_snapshot/classic-restore.json"
cp "$fixture/scripts/apply-repo-safeguards.sh" "$WORK/safeguards-script.backup"
python3 - "$fixture/scripts/apply-repo-safeguards.sh" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()
start = text.index("required_check_contexts() {")
end = text.index("\n}\n", start) + len("\n}\n")
replacement = """required_check_contexts() {
    printf '%s\\n' worktree-only-stable-context
}
"""
path.write_text(text[:start] + replacement + text[end:])
PY
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" \
    "$stable_worktree_drift_snapshot"
cp "$WORK/safeguards-script.backup" \
    "$fixture/scripts/apply-repo-safeguards.sh"
echo "ok  : restore derives legacy and stable policy only from the manifest commit"

for missing_classic_key in \
    required_status_checks \
    enforce_admins \
    required_linear_history \
    allow_force_pushes \
    allow_deletions \
    required_conversation_resolution \
    required_signatures \
    block_creations \
    lock_branch \
    allow_fork_syncing; do
    incomplete_classic_snapshot="$WORK/incomplete-classic-${missing_classic_key}-snapshot"
    cp -R "$snapshot" "$incomplete_classic_snapshot"
    jq --arg key "$missing_classic_key" 'del(.[$key])' \
        "$incomplete_classic_snapshot/classic-live.json" \
        > "$incomplete_classic_snapshot/classic-live.tmp"
    mv "$incomplete_classic_snapshot/classic-live.tmp" \
        "$incomplete_classic_snapshot/classic-live.json"
    expect_restore_rejected_without_mutation \
        "schema or payload validation failed" "$state" "$incomplete_classic_snapshot"
done

for enabled_classic_key in required_pull_request_reviews restrictions; do
    enabled_classic_snapshot="$WORK/enabled-classic-${enabled_classic_key}-snapshot"
    cp -R "$snapshot" "$enabled_classic_snapshot"
    jq --arg key "$enabled_classic_key" '.[$key] = {enabled: true}' \
        "$enabled_classic_snapshot/classic-live.json" \
        > "$enabled_classic_snapshot/classic-live.tmp"
    mv "$enabled_classic_snapshot/classic-live.tmp" \
        "$enabled_classic_snapshot/classic-live.json"
    expect_restore_rejected_without_mutation \
        "schema or payload validation failed" "$state" "$enabled_classic_snapshot"
done

symlink_snapshot="$WORK/symlink-snapshot"
cp -R "$snapshot" "$symlink_snapshot"
rm "$symlink_snapshot/actions-restore.json"
ln -s "$snapshot/actions-restore.json" "$symlink_snapshot/actions-restore.json"
expect_restore_rejected_without_mutation \
    "missing or unsafe actions-restore.json" "$state" "$symlink_snapshot"

altered_integration_snapshot="$WORK/altered-integration-snapshot"
cp -R "$snapshot" "$altered_integration_snapshot"
jq '
  (.rules[] | select(.type == "required_status_checks") |
    .parameters.required_status_checks[0].integration_id) = 999
' "$altered_integration_snapshot/integrity-restore.json" \
    > "$altered_integration_snapshot/integrity-restore.tmp"
mv "$altered_integration_snapshot/integrity-restore.tmp" \
    "$altered_integration_snapshot/integrity-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$altered_integration_snapshot"

narrow_mismatch_snapshot="$WORK/narrow-mismatch-snapshot"
cp -R "$snapshot" "$narrow_mismatch_snapshot"
jq 'del(.contexts[-1], .checks[-1])' \
    "$narrow_mismatch_snapshot/classic-restore.json" \
    > "$narrow_mismatch_snapshot/classic-restore.tmp"
mv "$narrow_mismatch_snapshot/classic-restore.tmp" \
    "$narrow_mismatch_snapshot/classic-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$narrow_mismatch_snapshot"

malformed_snapshot="$WORK/malformed-snapshot"
cp -R "$snapshot" "$malformed_snapshot"
printf '{malformed\n' > "$malformed_snapshot/actions-restore.json"
expect_restore_rejected_without_mutation \
    "schema or payload validation failed" "$state" "$malformed_snapshot"
echo "ok  : required classic keys, enabled optional policy, and adversarial recovery shapes fail before mutation"

altered_context_snapshot="$WORK/altered-context-snapshot"
cp -R "$snapshot" "$altered_context_snapshot"
jq '
  (.rules[] | select(.type == "required_status_checks") |
    .parameters.required_status_checks) =
  [{context: "unreviewed-context", integration_id: 15368}]
' "$altered_context_snapshot/integrity-restore.json" \
    > "$altered_context_snapshot/integrity-restore.tmp"
mv "$altered_context_snapshot/integrity-restore.tmp" \
    "$altered_context_snapshot/integrity-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$altered_context_snapshot"

for missing_file in \
    manifest.json \
    actions-restore.json \
    integrity-restore.json \
    classic-restore.json \
    classic-live.json; do
    incomplete_snapshot="$WORK/incomplete-${missing_file%.json}-snapshot"
    cp -R "$snapshot" "$incomplete_snapshot"
    rm "$incomplete_snapshot/$missing_file"
    expect_restore_rejected_without_mutation \
        "missing or unsafe $missing_file" "$state" "$incomplete_snapshot"
done

altered_bypass_snapshot="$WORK/altered-bypass-snapshot"
cp -R "$snapshot" "$altered_bypass_snapshot"
jq '.bypass_actors = [{actor_id: 1, actor_type: "RepositoryRole", bypass_mode: "always"}]' \
    "$altered_bypass_snapshot/integrity-restore.json" \
    > "$altered_bypass_snapshot/integrity-restore.tmp"
mv "$altered_bypass_snapshot/integrity-restore.tmp" \
    "$altered_bypass_snapshot/integrity-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$altered_bypass_snapshot"

altered_condition_snapshot="$WORK/altered-condition-snapshot"
cp -R "$snapshot" "$altered_condition_snapshot"
jq '.conditions.ref_name.include = ["refs/heads/other"]' \
    "$altered_condition_snapshot/integrity-restore.json" \
    > "$altered_condition_snapshot/integrity-restore.tmp"
mv "$altered_condition_snapshot/integrity-restore.tmp" \
    "$altered_condition_snapshot/integrity-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$altered_condition_snapshot"

cross_stage_snapshot="$WORK/cross-stage-snapshot"
cp -R "$snapshot" "$cross_stage_snapshot"
jq '.stage = "stable"' "$cross_stage_snapshot/manifest.json" \
    > "$cross_stage_snapshot/manifest.tmp"
mv "$cross_stage_snapshot/manifest.tmp" "$cross_stage_snapshot/manifest.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$cross_stage_snapshot"

altered_actions_snapshot="$WORK/altered-actions-snapshot"
cp -R "$snapshot" "$altered_actions_snapshot"
jq '.sha_pinning_required = true' "$altered_actions_snapshot/actions-restore.json" \
    > "$altered_actions_snapshot/actions-restore.tmp"
mv "$altered_actions_snapshot/actions-restore.tmp" \
    "$altered_actions_snapshot/actions-restore.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$altered_actions_snapshot"

altered_classic_snapshot="$WORK/altered-classic-snapshot"
cp -R "$snapshot" "$altered_classic_snapshot"
jq '.required_linear_history.enabled = false' \
    "$altered_classic_snapshot/classic-live.json" \
    > "$altered_classic_snapshot/classic-live.tmp"
mv "$altered_classic_snapshot/classic-live.tmp" \
    "$altered_classic_snapshot/classic-live.json"
expect_restore_rejected_without_mutation \
    "policy does not match manifest stage" "$state" "$altered_classic_snapshot"

altered_id_snapshot="$WORK/altered-id-snapshot"
cp -R "$snapshot" "$altered_id_snapshot"
jq '.integrity_ruleset_id = 102' "$altered_id_snapshot/manifest.json" \
    > "$altered_id_snapshot/manifest.tmp"
mv "$altered_id_snapshot/manifest.tmp" "$altered_id_snapshot/manifest.json"
expect_restore_rejected_without_mutation \
    "integrity ruleset ID does not match" "$state" "$altered_id_snapshot"

unavailable_policy_snapshot="$WORK/unavailable-policy-snapshot"
cp -R "$snapshot" "$unavailable_policy_snapshot"
jq '.live_main_sha = "ffffffffffffffffffffffffffffffffffffffff"' \
    "$unavailable_policy_snapshot/manifest.json" \
    > "$unavailable_policy_snapshot/manifest.tmp"
mv "$unavailable_policy_snapshot/manifest.tmp" \
    "$unavailable_policy_snapshot/manifest.json"
expect_restore_rejected_without_mutation \
    "policy commit does not match current live main" "$state" "$unavailable_policy_snapshot"
unavailable_policy_state="$(new_case_state unavailable-policy-commit)"
jq '.sha = "ffffffffffffffffffffffffffffffffffffffff"' \
    "$unavailable_policy_state/live-main.json" \
    > "$unavailable_policy_state/live-main.tmp"
mv "$unavailable_policy_state/live-main.tmp" \
    "$unavailable_policy_state/live-main.json"
expect_restore_rejected_without_mutation \
    "committed policy source is unavailable" \
    "$unavailable_policy_state" "$unavailable_policy_snapshot"
echo "ok  : incomplete, altered, and cross-stage recovery snapshots fail before mutation"

private_state="$(new_case_state private-repository)"
jq '.private = true | .visibility = "private"' \
    "$private_state/repository.json" > "$private_state/repository.tmp"
mv "$private_state/repository.tmp" "$private_state/repository.json"
expect_failure "public-repo posture" \
    run_safeguards "$private_state" "$mutation_log" --preflight-only

private_state="$(new_case_state concurrent-private-repository)"
TEST_PRIVATE_BEFORE_SECOND_CAPTURE=1 \
    expect_failure "public-repo posture" \
    run_safeguards "$private_state" "$mutation_log"
[[ ! -s "$mutation_log" ]] || {
    echo "FAIL: private repository posture was detected after mutation" >&2
    exit 1
}
echo "ok  : private visibility and public-to-private concurrent drift fail before mutation"

: > "$mutation_log"
output="$(run_safeguards "$state" "$mutation_log")"
grep -F "already match the stable checked-in posture" <<<"$output" >/dev/null
[[ ! -s "$mutation_log" ]] || { echo "FAIL: repeated stable apply mutated live state" >&2; exit 1; }
echo "ok  : repeated stable apply is verified and write-free"

corrupt_snapshot="$WORK/corrupt-snapshot"
cp -R "$snapshot" "$corrupt_snapshot"
jq '.repository = "other/repo"' "$corrupt_snapshot/manifest.json" > "$corrupt_snapshot/manifest.tmp"
mv "$corrupt_snapshot/manifest.tmp" "$corrupt_snapshot/manifest.json"
: > "$mutation_log"
expect_failure "recovery snapshot belongs to other/repo" \
    run_safeguards "$state" "$mutation_log" --restore "$corrupt_snapshot"
[[ ! -s "$mutation_log" ]] || { echo "FAIL: mismatched recovery snapshot mutated live state" >&2; exit 1; }
echo "ok  : mismatched recovery snapshot fails validation before mutation"

frozen_snapshot="$WORK/frozen-snapshot"
cp -R "$snapshot" "$frozen_snapshot"
: > "$mutation_log"
TEST_MUTATE_SNAPSHOT_AFTER_FIRST_WRITE="$frozen_snapshot" \
    run_safeguards "$state" "$mutation_log" --restore "$frozen_snapshot" >/dev/null
jq -e '
  [.rules[] | select(.type == "required_status_checks") |
    .parameters.required_status_checks[].context] == $legacy
' --argjson legacy "$legacy_contexts_json" "$state/integrity.json" >/dev/null
echo "ok  : restore writes only private frozen bytes after validation"

: > "$mutation_log"
run_safeguards "$state" "$mutation_log" --restore "$snapshot" >/dev/null
jq -e '.sha_pinning_required == false' "$state/actions.json" >/dev/null
jq -e '.required_status_checks.contexts == $legacy' --argjson legacy "$legacy_contexts_json" "$state/classic.json" >/dev/null
echo "ok  : explicit recovery restores and verifies the complete prior cutover state"

readback_state="$(new_case_state restore-readback-failure)"
: > "$mutation_log"
output="$(run_safeguards "$readback_state" "$mutation_log")"
readback_snapshot="$(awk -F': ' '/^Recovery snapshot:/ { print $2 }' <<<"$output")"
[[ -n "$readback_snapshot" && -d "$readback_snapshot" ]] || {
    echo "FAIL: could not identify recovery snapshot for readback failure test" >&2
    exit 1
}
: > "$mutation_log"
set +e
output="$(TEST_FAIL_READ_AFTER_MUTATION_PATH='repos/owner/repo/actions/permissions' \
    run_safeguards "$readback_state" "$mutation_log" --restore "$readback_snapshot" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || { echo "FAIL: injected restore readback failure succeeded" >&2; exit 1; }
grep -F "verification readback failed" <<<"$output" >/dev/null
grep -F "Retry:" <<<"$output" >/dev/null
[[ "$(wc -l < "$mutation_log" | tr -d ' ')" == "3" ]] || {
    echo "FAIL: restore readback failure did not occur after exactly three writes" >&2
    exit 1
}
echo "ok  : restore readback failure is explicit and retains the retry path"

state="$(new_case_state automatic-rollback)"
: > "$mutation_log"
set +e
output="$(TEST_FAIL_ONCE_PATH='repos/owner/repo/branches/main/protection/required_status_checks' \
    run_safeguards "$state" "$mutation_log" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || { echo "FAIL: injected apply failure succeeded" >&2; exit 1; }
grep -F "previous three-resource cutover state was restored" <<<"$output" >/dev/null
jq -e '.sha_pinning_required == false' "$state/actions.json" >/dev/null
jq -e '.required_status_checks.contexts == $legacy' --argjson legacy "$legacy_contexts_json" "$state/classic.json" >/dev/null
echo "ok  : partial apply failure automatically restores the prior valid state"

state="$(new_case_state rollback-failure)"
: > "$mutation_log"
set +e
output="$(TEST_FAIL_ONCE_PATH='repos/owner/repo/branches/main/protection/required_status_checks' \
    TEST_FAIL_AFTER_PATH='repos/owner/repo/actions/permissions' \
    run_safeguards "$state" "$mutation_log" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || { echo "FAIL: injected rollback failure succeeded" >&2; exit 1; }
grep -F "RECOVERY REQUIRED:" <<<"$output" >/dev/null
recovery_snapshot="$(find "$fixture/.git/dotfiles-safeguards" -mindepth 1 -maxdepth 1 -type d -newer "$snapshot" | tail -n 1)"
[[ -n "$recovery_snapshot" && -f "$recovery_snapshot/RECOVERY.txt" ]] || {
    echo "FAIL: rollback failure did not retain exact recovery material" >&2
    exit 1
}
: > "$mutation_log"
run_safeguards "$state" "$mutation_log" --restore "$recovery_snapshot" >/dev/null
jq -e '.sha_pinning_required == false' "$state/actions.json" >/dev/null
echo "ok  : rollback failure is explicit and the retained snapshot recovers on retry"

echo "all repository safeguard preflight and transaction behaviors OK"
