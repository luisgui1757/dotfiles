#!/usr/bin/env bash
# Verify the zshrc precompile cache mechanism:
#   1) created on first run,
#   2) NOT regenerated on second run when the toml has not been touched,
#   3) regenerated when the toml is newer.
#
# Uses content-sentinel checks instead of mtime comparison so the test is
# robust against filesystem timestamp precision quirks (Linux nanosecond vs
# macOS second; CI cp + immediate-create races; etc.).
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi
if ! command -v starship >/dev/null 2>&1; then
    echo "skipped: starship not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_HOME="$(mktemp -d)"
TMP_CONFIG_DIR="$(mktemp -d)"
TMP_CONFIG="$TMP_CONFIG_DIR/starship.toml"
trap 'rm -rf "$TMP_HOME" "$TMP_CONFIG_DIR"' EXIT

# Copy the toml into a scratch dir so we don't dirty the repo.
cp "$REPO_ROOT/starship/starship.toml" "$TMP_CONFIG"
# Force the toml's mtime well into the past so the cache (mtime = now)
# is reliably newer.
touch -t 202001010000 "$TMP_CONFIG"

run_shell() {
    HOME="$TMP_HOME" STARSHIP_CONFIG="$TMP_CONFIG" \
        zsh -i -c "source $REPO_ROOT/shells/zshrc; print -r -- ready" >/dev/null 2>&1
}

cache="$TMP_HOME/.cache/starship-init.zsh"
SENTINEL="# precompile-test-sentinel-$$"

# 1) First run creates the cache.
[[ -f "$cache" ]] && rm -f "$cache"
run_shell
[[ -s "$cache" ]] || { echo "FAIL: cache not created on first run"; exit 1; }

# 2) Insert a sentinel line into the cache. Second run must NOT regenerate
#    (otherwise the sentinel disappears). This bypasses every fs-mtime
#    precision quirk that an mtime comparison would have.
printf '\n%s\n' "$SENTINEL" >> "$cache"
run_shell
if ! grep -Fq "$SENTINEL" "$cache"; then
    echo "FAIL: cache regenerated on second run (config was NOT newer)"
    exit 1
fi

# 3) Touching the toml makes the next run regenerate the cache (sentinel must
#    be gone). Sleep before touching so the config mtime lands AFTER the cache
#    mtime even on filesystems with coarse-grained timestamp resolution.
sleep 1
touch "$TMP_CONFIG"
run_shell
if grep -Fq "$SENTINEL" "$cache"; then
    echo "FAIL: touching the toml did not trigger regeneration"
    exit 1
fi

echo "OK"
