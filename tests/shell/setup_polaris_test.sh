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

POLARIS_CACHE_ROOT="$TMP_ROOT/cache"
POLARIS_VERSION="0.1.1"
POLARIS_REF="489dcc6f991ddcff63c460a433e983264dc54cf7"
ALL=0
DRY_RUN=1
SKIP_AGENTS=0

output="$(run_polaris_agent_policy 2>&1)"
[[ "$output" == *"Phase 6/6: apply global agent policy (Polaris)"* ]] \
    || fail "dry-run did not report the Polaris phase"
[[ "$output" == *"would: clone/fetch Polaris 0.1.1"* ]] \
    || fail "dry-run did not preview the pinned Polaris fetch"
[[ ! -e "$POLARIS_CACHE_ROOT" ]] \
    || fail "dry-run created the Polaris cache"

DRY_RUN=0
ALL=1
SKIP_AGENTS=1
output="$(run_polaris_agent_policy 2>&1)"
[[ "$output" == *"skipped: Phase 6/6 (agent policy) via --skip-agents"* ]] \
    || fail "--skip-agents did not skip the Polaris phase"

work="$TMP_ROOT/polaris-work"
mkdir -p "$work/tools"
git -C "$work" init -q
git -C "$work" config user.name "Dotfiles Test"
git -C "$work" config user.email "dotfiles@example.invalid"
printf '0.1.1\n' > "$work/VERSION"
cat > "$work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$POLARIS_TEST_LOG"
EOF
chmod +x "$work/tools/install"
git -C "$work" add VERSION tools/install
git -C "$work" commit -q -m "fake polaris"

sha="$(git -C "$work" rev-parse HEAD)"
mkdir -p "$POLARIS_CACHE_ROOT"
mv "$work" "$POLARIS_CACHE_ROOT/$sha"

POLARIS_REF="$sha"
POLARIS_TEST_LOG="$TMP_ROOT/polaris-install.log"
export POLARIS_TEST_LOG
SKIP_AGENTS=0
ALL=1
DRY_RUN=0

run_polaris_agent_policy >/dev/null

grep -Fx -- "--global" "$POLARIS_TEST_LOG" >/dev/null \
    || fail "Polaris global install was not invoked"
grep -Fx -- "--global --check" "$POLARIS_TEST_LOG" >/dev/null \
    || fail "Polaris global check was not invoked"

before_dirty_count="$(wc -l < "$POLARIS_TEST_LOG" | tr -d ' ')"
printf '\n# dirty cache regression\n' >> "$POLARIS_CACHE_ROOT/$sha/tools/install"
set +e
dirty_output="$( ( run_polaris_agent_policy ) 2>&1 )"
dirty_rc=$?
set -e
[[ "$dirty_rc" -ne 0 ]] \
    || fail "dirty tracked Polaris cache was accepted"
[[ "$dirty_output" == *"Polaris cache has local changes"* ]] \
    || fail "dirty tracked Polaris cache did not explain the refusal"
after_dirty_count="$(wc -l < "$POLARIS_TEST_LOG" | tr -d ' ')"
[[ "$after_dirty_count" == "$before_dirty_count" ]] \
    || fail "dirty tracked Polaris cache executed the installer"

git -C "$POLARIS_CACHE_ROOT/$sha" checkout -- tools/install
touch "$POLARIS_CACHE_ROOT/$sha/UNTRACKED"
set +e
untracked_output="$( ( run_polaris_agent_policy ) 2>&1 )"
untracked_rc=$?
set -e
[[ "$untracked_rc" -ne 0 ]] \
    || fail "untracked Polaris cache was accepted"
[[ "$untracked_output" == *"Polaris cache has local changes"* ]] \
    || fail "untracked Polaris cache did not explain the refusal"
after_untracked_count="$(wc -l < "$POLARIS_TEST_LOG" | tr -d ' ')"
[[ "$after_untracked_count" == "$before_dirty_count" ]] \
    || fail "untracked Polaris cache executed the installer"
rm "$POLARIS_CACHE_ROOT/$sha/UNTRACKED"

printf 'IGNORED\n' >> "$POLARIS_CACHE_ROOT/$sha/.git/info/exclude"
touch "$POLARIS_CACHE_ROOT/$sha/IGNORED"
set +e
ignored_output="$( ( run_polaris_agent_policy ) 2>&1 )"
ignored_rc=$?
set -e
[[ "$ignored_rc" -ne 0 ]] \
    || fail "ignored Polaris cache file was accepted"
[[ "$ignored_output" == *"Polaris cache has local changes"* ]] \
    || fail "ignored Polaris cache file did not explain the refusal"
after_ignored_count="$(wc -l < "$POLARIS_TEST_LOG" | tr -d ' ')"
[[ "$after_ignored_count" == "$before_dirty_count" ]] \
    || fail "ignored Polaris cache file executed the installer"
rm "$POLARIS_CACHE_ROOT/$sha/IGNORED"

fsmonitor_marker="$TMP_ROOT/fsmonitor-ran"
cat > "$TMP_ROOT/fsmonitor" <<EOF
#!/usr/bin/env bash
printf ran > "$fsmonitor_marker"
exit 0
EOF
chmod +x "$TMP_ROOT/fsmonitor"
git -C "$POLARIS_CACHE_ROOT/$sha" config core.fsmonitor "$TMP_ROOT/fsmonitor"
run_polaris_agent_policy >/dev/null
[[ ! -e "$fsmonitor_marker" ]] \
    || fail "Polaris cache validation executed core.fsmonitor"

before_worktree_count="$(wc -l < "$POLARIS_TEST_LOG" | tr -d ' ')"
clean_worktree="$TMP_ROOT/clean-worktree"
mkdir -p "$clean_worktree/tools"
cp "$POLARIS_CACHE_ROOT/$sha/VERSION" "$clean_worktree/VERSION"
cp "$POLARIS_CACHE_ROOT/$sha/tools/install" "$clean_worktree/tools/install"
worktree_marker="$TMP_ROOT/core-worktree-dirty-installer-ran"
cat > "$POLARIS_CACHE_ROOT/$sha/tools/install" <<EOF
#!/usr/bin/env bash
printf ran > "$worktree_marker"
EOF
chmod +x "$POLARIS_CACHE_ROOT/$sha/tools/install"
git -C "$POLARIS_CACHE_ROOT/$sha" config core.worktree "$clean_worktree"
set +e
worktree_output="$( ( run_polaris_agent_policy ) 2>&1 )"
worktree_rc=$?
set -e
[[ "$worktree_rc" -ne 0 ]] \
    || fail "Polaris cache with redirected core.worktree was accepted"
[[ "$worktree_output" == *"Polaris cache has local changes"* ]] \
    || fail "redirected core.worktree cache did not explain the refusal"
[[ ! -e "$worktree_marker" ]] \
    || fail "redirected core.worktree executed the dirty installer"
after_worktree_count="$(wc -l < "$POLARIS_TEST_LOG" | tr -d ' ')"
[[ "$after_worktree_count" == "$before_worktree_count" ]] \
    || fail "redirected core.worktree cache executed the installer"

echo "OK"
