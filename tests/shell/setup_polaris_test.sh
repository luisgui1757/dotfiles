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

echo "OK"
