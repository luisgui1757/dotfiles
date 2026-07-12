#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/home"

HOME="$TMP_ROOT/home"
export HOME
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"

SENTINEL_CACHE_ROOT="$TMP_ROOT/cache"
TEST_SENTINEL_VERSION="0.1.2"
SENTINEL_VERSION="$TEST_SENTINEL_VERSION"
SENTINEL_REF="489dcc6f991ddcff63c460a433e983264dc54cf7"
ALL=0
DRY_RUN=1
SKIP_AGENTS=0

output="$(run_sentinel_agent_policy 2>&1)"
[[ "$output" == *"Phase 6/6: apply global agent policy (Sentinel)"* ]] \
    || fail "dry-run did not report the Sentinel phase"
[[ "$output" == *"would: clone/fetch Sentinel $TEST_SENTINEL_VERSION (@"* ]] \
    || fail "dry-run did not preview the pinned Sentinel fetch"
[[ ! -e "$SENTINEL_CACHE_ROOT" ]] \
    || fail "dry-run created the Sentinel cache"

DRY_RUN=0
ALL=1
SKIP_AGENTS=1
output="$(run_sentinel_agent_policy 2>&1)"
[[ "$output" == *"skipped: Phase 6/6 (agent policy) via --skip-agents"* ]] \
    || fail "--skip-agents did not skip the Sentinel phase"

fresh_work="$TMP_ROOT/sentinel-fresh-work"
mkdir -p "$fresh_work/tools"
git -C "$fresh_work" init -q
git -C "$fresh_work" config user.name "Dotfiles Test"
git -C "$fresh_work" config user.email "dotfiles@example.invalid"
printf '%s\n' "$TEST_SENTINEL_VERSION" > "$fresh_work/VERSION"
cat > "$fresh_work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$SENTINEL_TEST_LOG"
EOF
chmod +x "$fresh_work/tools/install"
git -C "$fresh_work" add VERSION tools/install
git -C "$fresh_work" commit -q -m "fake sentinel fresh fetch"

fresh_sha="$(git -C "$fresh_work" rev-parse HEAD)"
fresh_global_marker="$TMP_ROOT/fresh-global-fsmonitor-ran"
fresh_env_marker="$TMP_ROOT/fresh-env-fsmonitor-ran"
fresh_template_marker="$TMP_ROOT/fresh-template-post-checkout-ran"
cat > "$TMP_ROOT/fresh-global-fsmonitor" <<EOF
#!/usr/bin/env bash
printf ran > "$fresh_global_marker"
exit 0
EOF
cat > "$TMP_ROOT/fresh-env-fsmonitor" <<EOF
#!/usr/bin/env bash
printf ran > "$fresh_env_marker"
exit 0
EOF
chmod +x "$TMP_ROOT/fresh-global-fsmonitor" "$TMP_ROOT/fresh-env-fsmonitor"
fresh_global_config="$TMP_ROOT/fresh-hostile.gitconfig"
cat > "$fresh_global_config" <<EOF
[core]
  fsmonitor = $TMP_ROOT/fresh-global-fsmonitor
EOF
fresh_template_dir="$TMP_ROOT/fresh-template"
mkdir -p "$fresh_template_dir/hooks"
cat > "$fresh_template_dir/hooks/post-checkout" <<EOF
#!/usr/bin/env bash
printf ran > "$fresh_template_marker"
exit 0
EOF
chmod +x "$fresh_template_dir/hooks/post-checkout"

SENTINEL_REPO_URL="$fresh_work"
SENTINEL_REF="$fresh_sha"
SENTINEL_CACHE_ROOT="$TMP_ROOT/fresh-cache"
SENTINEL_TEST_LOG="$TMP_ROOT/fresh-sentinel-install.log"
export SENTINEL_TEST_LOG
SKIP_AGENTS=0
export GIT_CONFIG_GLOBAL="$fresh_global_config"
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.fsmonitor
export GIT_CONFIG_VALUE_0="$TMP_ROOT/fresh-env-fsmonitor"
export GIT_TEMPLATE_DIR="$fresh_template_dir"

run_sentinel_agent_policy >/dev/null
unset GIT_CONFIG_GLOBAL GIT_CONFIG_COUNT GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0 GIT_TEMPLATE_DIR

[[ ! -e "$fresh_global_marker" ]] \
    || fail "fresh Sentinel fetch executed global core.fsmonitor"
[[ ! -e "$fresh_env_marker" ]] \
    || fail "fresh Sentinel fetch executed env-injected core.fsmonitor"
[[ ! -e "$fresh_template_marker" ]] \
    || fail "fresh Sentinel fetch executed a template post-checkout hook"
grep -Fx -- "--global" "$SENTINEL_TEST_LOG" >/dev/null \
    || fail "fresh Sentinel global install was not invoked"
grep -Fx -- "--global --check" "$SENTINEL_TEST_LOG" >/dev/null \
    || fail "fresh Sentinel global check was not invoked"

invalid_work="$TMP_ROOT/sentinel-invalid-work"
mkdir -p "$invalid_work/tools"
git -C "$invalid_work" init -q
git -C "$invalid_work" config user.name "Dotfiles Test"
git -C "$invalid_work" config user.email "dotfiles@example.invalid"
printf '%s\n' "0.1.1" > "$invalid_work/VERSION"
cat > "$invalid_work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$SENTINEL_TEST_LOG"
EOF
chmod +x "$invalid_work/tools/install"
git -C "$invalid_work" add VERSION tools/install
git -C "$invalid_work" commit -q -m "fake sentinel with wrong version"
invalid_sha="$(git -C "$invalid_work" rev-parse HEAD)"
SENTINEL_REPO_URL="$invalid_work"
SENTINEL_REF="$invalid_sha"
SENTINEL_CACHE_ROOT="$TMP_ROOT/invalid-cache"
SENTINEL_TEST_LOG="$TMP_ROOT/invalid-sentinel-install.log"
set +e
invalid_output="$(run_sentinel_agent_policy 2>&1)"
invalid_rc=$?
set -e
[[ "$invalid_rc" -ne 0 ]] \
    || fail "wrong-version Sentinel artifact was accepted"
[[ "$invalid_output" == *"Sentinel cache VERSION mismatch"* ]] \
    || fail "wrong-version Sentinel artifact did not explain the refusal"
[[ ! -e "$SENTINEL_TEST_LOG" ]] \
    || fail "wrong-version Sentinel artifact executed the installer"
if find "$SENTINEL_CACHE_ROOT" -maxdepth 1 -name '.tmp.*' -print -quit 2>/dev/null | grep -q .; then
    fail "failed Sentinel validation left a staging directory"
fi

# The same cache root must be retryable after the external artifact identity is
# corrected; no failed stage may strand the next run.
printf '%s\n' "$TEST_SENTINEL_VERSION" > "$invalid_work/VERSION"
git -C "$invalid_work" add VERSION
git -C "$invalid_work" commit -q -m "repair fake sentinel version"
corrected_sha="$(git -C "$invalid_work" rev-parse HEAD)"
SENTINEL_REF="$corrected_sha"
run_sentinel_agent_policy >/dev/null
[[ -d "$SENTINEL_CACHE_ROOT/$corrected_sha/.git" ]] \
    || fail "Sentinel retry did not publish the corrected verified checkout"
if find "$SENTINEL_CACHE_ROOT" -maxdepth 1 -name '.tmp.*' -print -quit 2>/dev/null | grep -q .; then
    fail "successful Sentinel retry left a staging directory"
fi

work="$TMP_ROOT/sentinel-work"
mkdir -p "$work/tools"
git -C "$work" init -q
git -C "$work" config user.name "Dotfiles Test"
git -C "$work" config user.email "dotfiles@example.invalid"
printf '%s\n' "$TEST_SENTINEL_VERSION" > "$work/VERSION"
cat > "$work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$SENTINEL_TEST_LOG"
EOF
chmod +x "$work/tools/install"
git -C "$work" add VERSION tools/install
git -C "$work" commit -q -m "fake sentinel"

sha="$(git -C "$work" rev-parse HEAD)"
mkdir -p "$SENTINEL_CACHE_ROOT"
mv "$work" "$SENTINEL_CACHE_ROOT/$sha"

SENTINEL_REF="$sha"
SENTINEL_TEST_LOG="$TMP_ROOT/sentinel-install.log"
export SENTINEL_TEST_LOG
SKIP_AGENTS=0
ALL=1
DRY_RUN=0

run_sentinel_agent_policy >/dev/null

grep -Fx -- "--global" "$SENTINEL_TEST_LOG" >/dev/null \
    || fail "Sentinel global install was not invoked"
grep -Fx -- "--global --check" "$SENTINEL_TEST_LOG" >/dev/null \
    || fail "Sentinel global check was not invoked"

before_dirty_count="$(wc -l < "$SENTINEL_TEST_LOG" | tr -d ' ')"
printf '\n# dirty cache regression\n' >> "$SENTINEL_CACHE_ROOT/$sha/tools/install"
set +e
dirty_output="$( ( run_sentinel_agent_policy ) 2>&1 )"
dirty_rc=$?
set -e
[[ "$dirty_rc" -ne 0 ]] \
    || fail "dirty tracked Sentinel cache was accepted"
[[ "$dirty_output" == *"Sentinel cache has local changes"* ]] \
    || fail "dirty tracked Sentinel cache did not explain the refusal"
after_dirty_count="$(wc -l < "$SENTINEL_TEST_LOG" | tr -d ' ')"
[[ "$after_dirty_count" == "$before_dirty_count" ]] \
    || fail "dirty tracked Sentinel cache executed the installer"

git -C "$SENTINEL_CACHE_ROOT/$sha" checkout -- tools/install
touch "$SENTINEL_CACHE_ROOT/$sha/UNTRACKED"
set +e
untracked_output="$( ( run_sentinel_agent_policy ) 2>&1 )"
untracked_rc=$?
set -e
[[ "$untracked_rc" -ne 0 ]] \
    || fail "untracked Sentinel cache was accepted"
[[ "$untracked_output" == *"Sentinel cache has local changes"* ]] \
    || fail "untracked Sentinel cache did not explain the refusal"
after_untracked_count="$(wc -l < "$SENTINEL_TEST_LOG" | tr -d ' ')"
[[ "$after_untracked_count" == "$before_dirty_count" ]] \
    || fail "untracked Sentinel cache executed the installer"
rm "$SENTINEL_CACHE_ROOT/$sha/UNTRACKED"

printf 'IGNORED\n' >> "$SENTINEL_CACHE_ROOT/$sha/.git/info/exclude"
touch "$SENTINEL_CACHE_ROOT/$sha/IGNORED"
set +e
ignored_output="$( ( run_sentinel_agent_policy ) 2>&1 )"
ignored_rc=$?
set -e
[[ "$ignored_rc" -ne 0 ]] \
    || fail "ignored Sentinel cache file was accepted"
[[ "$ignored_output" == *"Sentinel cache has local changes"* ]] \
    || fail "ignored Sentinel cache file did not explain the refusal"
after_ignored_count="$(wc -l < "$SENTINEL_TEST_LOG" | tr -d ' ')"
[[ "$after_ignored_count" == "$before_dirty_count" ]] \
    || fail "ignored Sentinel cache file executed the installer"
rm "$SENTINEL_CACHE_ROOT/$sha/IGNORED"

fsmonitor_marker="$TMP_ROOT/fsmonitor-ran"
cat > "$TMP_ROOT/fsmonitor" <<EOF
#!/usr/bin/env bash
printf ran > "$fsmonitor_marker"
exit 0
EOF
chmod +x "$TMP_ROOT/fsmonitor"
git -C "$SENTINEL_CACHE_ROOT/$sha" config core.fsmonitor "$TMP_ROOT/fsmonitor"
run_sentinel_agent_policy >/dev/null
[[ ! -e "$fsmonitor_marker" ]] \
    || fail "Sentinel cache validation executed core.fsmonitor"

before_worktree_count="$(wc -l < "$SENTINEL_TEST_LOG" | tr -d ' ')"
clean_worktree="$TMP_ROOT/clean-worktree"
mkdir -p "$clean_worktree/tools"
cp "$SENTINEL_CACHE_ROOT/$sha/VERSION" "$clean_worktree/VERSION"
cp "$SENTINEL_CACHE_ROOT/$sha/tools/install" "$clean_worktree/tools/install"
worktree_marker="$TMP_ROOT/core-worktree-dirty-installer-ran"
cat > "$SENTINEL_CACHE_ROOT/$sha/tools/install" <<EOF
#!/usr/bin/env bash
printf ran > "$worktree_marker"
EOF
chmod +x "$SENTINEL_CACHE_ROOT/$sha/tools/install"
git -C "$SENTINEL_CACHE_ROOT/$sha" config core.worktree "$clean_worktree"
set +e
worktree_output="$( ( run_sentinel_agent_policy ) 2>&1 )"
worktree_rc=$?
set -e
[[ "$worktree_rc" -ne 0 ]] \
    || fail "Sentinel cache with redirected core.worktree was accepted"
[[ "$worktree_output" == *"Sentinel cache has local changes"* ]] \
    || fail "redirected core.worktree cache did not explain the refusal"
[[ ! -e "$worktree_marker" ]] \
    || fail "redirected core.worktree executed the dirty installer"
after_worktree_count="$(wc -l < "$SENTINEL_TEST_LOG" | tr -d ' ')"
[[ "$after_worktree_count" == "$before_worktree_count" ]] \
    || fail "redirected core.worktree cache executed the installer"

# A signal during the fetch must run the same cleanup path as an ordinary
# failure. Override only the external git boundary and interrupt the function's
# subshell after its same-parent staging directory exists.
SENTINEL_CACHE_ROOT="$TMP_ROOT/interrupted-cache"
SENTINEL_REF="1111111111111111111111111111111111111111"
SENTINEL_REPO_URL="$TMP_ROOT/interrupted-source"
interrupt_started="$TMP_ROOT/interrupted-fetch-started"
sentinel_git() {
    : > "$interrupt_started"
    while :; do :; done
}
ensure_sentinel_checkout > "$TMP_ROOT/interrupted.out" 2> "$TMP_ROOT/interrupted.err" &
interrupt_pid=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do
    [[ -e "$interrupt_started" ]] && break
    sleep 0.1
done
[[ -e "$interrupt_started" ]] || fail "interrupted Sentinel fixture did not reach fetch"
trap_pid="$(pgrep -P "$interrupt_pid" | head -1 || true)"
[[ -n "$trap_pid" ]] || trap_pid="$interrupt_pid"
kill -TERM "$trap_pid"
set +e
wait "$interrupt_pid"
interrupt_rc=$?
set -e
[[ "$interrupt_rc" -ne 0 ]] || fail "interrupted Sentinel fetch returned success"
if find "$SENTINEL_CACHE_ROOT" -maxdepth 1 -name '.tmp.*' -print -quit 2>/dev/null | grep -q .; then
    fail "interrupted Sentinel fetch left a staging directory"
fi
[[ ! -e "$SENTINEL_CACHE_ROOT/$SENTINEL_REF" ]] \
    || fail "interrupted Sentinel fetch published an unverified checkout"

echo "OK"
