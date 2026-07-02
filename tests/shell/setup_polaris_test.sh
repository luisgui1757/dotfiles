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
TEST_POLARIS_VERSION="0.1.2"
TEST_POLARIS_TAG="v0.1.2"
POLARIS_VERSION="$TEST_POLARIS_VERSION"
POLARIS_TAG="$TEST_POLARIS_TAG"
POLARIS_REF="489dcc6f991ddcff63c460a433e983264dc54cf7"
ALL=0
DRY_RUN=1
SKIP_AGENTS=0

output="$(run_polaris_agent_policy 2>&1)"
[[ "$output" == *"Phase 6/6: apply global agent policy (Polaris)"* ]] \
    || fail "dry-run did not report the Polaris phase"
[[ "$output" == *"would: clone/fetch Polaris $TEST_POLARIS_VERSION ($TEST_POLARIS_TAG @"* ]] \
    || fail "dry-run did not preview the pinned Polaris fetch"
[[ ! -e "$POLARIS_CACHE_ROOT" ]] \
    || fail "dry-run created the Polaris cache"

DRY_RUN=0
ALL=1
SKIP_AGENTS=1
output="$(run_polaris_agent_policy 2>&1)"
[[ "$output" == *"skipped: Phase 6/6 (agent policy) via --skip-agents"* ]] \
    || fail "--skip-agents did not skip the Polaris phase"

fresh_work="$TMP_ROOT/polaris-fresh-work"
mkdir -p "$fresh_work/tools"
git -C "$fresh_work" init -q
git -C "$fresh_work" config user.name "Dotfiles Test"
git -C "$fresh_work" config user.email "dotfiles@example.invalid"
printf '%s\n' "$TEST_POLARIS_VERSION" > "$fresh_work/VERSION"
cat > "$fresh_work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$POLARIS_TEST_LOG"
EOF
chmod +x "$fresh_work/tools/install"
git -C "$fresh_work" add VERSION tools/install
git -C "$fresh_work" commit -q -m "fake polaris fresh fetch"

fresh_sha="$(git -C "$fresh_work" rev-parse HEAD)"
git -C "$fresh_work" tag "$TEST_POLARIS_TAG" "$fresh_sha"
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

POLARIS_REPO_URL="$fresh_work"
POLARIS_REF="$fresh_sha"
POLARIS_CACHE_ROOT="$TMP_ROOT/fresh-cache"
POLARIS_TEST_LOG="$TMP_ROOT/fresh-polaris-install.log"
export POLARIS_TEST_LOG
SKIP_AGENTS=0
export GIT_CONFIG_GLOBAL="$fresh_global_config"
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=core.fsmonitor
export GIT_CONFIG_VALUE_0="$TMP_ROOT/fresh-env-fsmonitor"
export GIT_TEMPLATE_DIR="$fresh_template_dir"

run_polaris_agent_policy >/dev/null
unset GIT_CONFIG_GLOBAL GIT_CONFIG_COUNT GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0 GIT_TEMPLATE_DIR

[[ ! -e "$fresh_global_marker" ]] \
    || fail "fresh Polaris fetch executed global core.fsmonitor"
[[ ! -e "$fresh_env_marker" ]] \
    || fail "fresh Polaris fetch executed env-injected core.fsmonitor"
[[ ! -e "$fresh_template_marker" ]] \
    || fail "fresh Polaris fetch executed a template post-checkout hook"
grep -Fx -- "--global" "$POLARIS_TEST_LOG" >/dev/null \
    || fail "fresh Polaris global install was not invoked"
grep -Fx -- "--global --check" "$POLARIS_TEST_LOG" >/dev/null \
    || fail "fresh Polaris global check was not invoked"

untagged_work="$TMP_ROOT/polaris-untagged-work"
mkdir -p "$untagged_work/tools"
git -C "$untagged_work" init -q
git -C "$untagged_work" config user.name "Dotfiles Test"
git -C "$untagged_work" config user.email "dotfiles@example.invalid"
printf '%s\n' "$TEST_POLARIS_VERSION" > "$untagged_work/VERSION"
cat > "$untagged_work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$POLARIS_TEST_LOG"
EOF
chmod +x "$untagged_work/tools/install"
git -C "$untagged_work" add VERSION tools/install
git -C "$untagged_work" commit -q -m "fake polaris without release tag"
untagged_sha="$(git -C "$untagged_work" rev-parse HEAD)"
POLARIS_REPO_URL="$untagged_work"
POLARIS_REF="$untagged_sha"
POLARIS_CACHE_ROOT="$TMP_ROOT/untagged-cache"
POLARIS_TEST_LOG="$TMP_ROOT/untagged-polaris-install.log"
set +e
untagged_output="$(run_polaris_agent_policy 2>&1)"
untagged_rc=$?
set -e
[[ "$untagged_rc" -ne 0 ]] \
    || fail "untagged Polaris release artifact was accepted"
[[ "$untagged_output" == *"Polaris tag mismatch"* ]] \
    || fail "untagged Polaris release artifact did not explain the tag refusal"
[[ ! -e "$POLARIS_TEST_LOG" ]] \
    || fail "untagged Polaris release artifact executed the installer"

work="$TMP_ROOT/polaris-work"
mkdir -p "$work/tools"
git -C "$work" init -q
git -C "$work" config user.name "Dotfiles Test"
git -C "$work" config user.email "dotfiles@example.invalid"
printf '%s\n' "$TEST_POLARIS_VERSION" > "$work/VERSION"
cat > "$work/tools/install" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$POLARIS_TEST_LOG"
EOF
chmod +x "$work/tools/install"
git -C "$work" add VERSION tools/install
git -C "$work" commit -q -m "fake polaris"

sha="$(git -C "$work" rev-parse HEAD)"
git -C "$work" tag "$TEST_POLARIS_TAG" "$sha"
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
