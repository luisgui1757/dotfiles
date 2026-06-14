#!/usr/bin/env bash
set -euo pipefail

# Adversarial safety coverage for uninstall.sh. The greenfield round-trip proves
# the clean path; this proves uninstall preserves user data it must never touch:
# a dirty external checkout, a user-replaced managed file, and that it still
# cleans a broken repo-pointing symlink. Mirrors parity_gate.sh sandboxing.
unset XDG_DATA_HOME

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SRC="$REPO_ROOT/home"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "required command '$1' is not on PATH"; }

require_cmd chezmoi
require_cmd git

HOME="$(mktemp -d)"
export HOME
trap 'rm -rf "$HOME"' EXIT

# --refresh-externals on every apply: chezmoi caches externals (refreshPeriod=0)
# and will NOT re-create one a prior scenario removed, so later scenarios that
# need the externals back must force a re-fetch.
apply() { chezmoi --source "$SRC" --no-tty --force --refresh-externals apply >/dev/null 2>&1; }

chezmoi --source "$SRC" init
apply
pass "chezmoi apply completed"

ext="$HOME/.local/share/dotfiles/zsh-plugins/zsh-autocomplete"
[[ -d "$ext" ]] || fail "external missing after apply: $ext"

# Scenario A: a DIRTY external must be preserved by --all (no --force-externals).
printf 'my local hack\n' > "$ext/USER_UNCOMMITTED.txt" # untracked -> dirty
"$REPO_ROOT/uninstall.sh" --all >"$HOME/A.log" 2>&1 || fail "uninstall --all failed: $(cat "$HOME/A.log")"
[[ -d "$ext" && -f "$ext/USER_UNCOMMITTED.txt" ]] || fail "dirty external was deleted by --all (data loss)"
grep -q "keeping .*zsh-autocomplete" "$HOME/A.log" || fail "expected a 'keeping' warning for the dirty external"
pass "dirty external preserved by --all"

"$REPO_ROOT/uninstall.sh" --all --force-externals >/dev/null 2>&1 || fail "uninstall --force-externals failed"
[[ ! -e "$ext" ]] || fail "--force-externals did not remove the dirty external"
pass "--force-externals removes a dirty external"

# Scenario B: a user-replaced managed path (now a regular file) must NOT be deleted.
apply
rm -f "$HOME/.tmux.conf"
printf 'USER OWNED FILE\n' > "$HOME/.tmux.conf"
"$REPO_ROOT/uninstall.sh" --all --keep-externals >/dev/null 2>&1 || fail "uninstall (scenario B) failed"
[[ -f "$HOME/.tmux.conf" && ! -L "$HOME/.tmux.conf" ]] || fail "user-owned file at managed path was removed (data loss)"
[[ "$(cat "$HOME/.tmux.conf")" == "USER OWNED FILE" ]] || fail "user-owned file content changed"
pass "user-replaced managed file preserved"

# Scenario C: a BROKEN repo-pointing symlink at a managed path is still removed.
apply
ln -sfn "$REPO_ROOT/home/this-target-does-not-exist" "$HOME/.zshrc"
[[ -L "$HOME/.zshrc" && ! -e "$HOME/.zshrc" ]] || fail "failed to set up broken-symlink fixture"
"$REPO_ROOT/uninstall.sh" --all --keep-externals >/dev/null 2>&1 || fail "uninstall (scenario C) failed"
[[ ! -L "$HOME/.zshrc" ]] || fail "broken repo-pointing symlink was not removed"
pass "broken repo-pointing symlink removed"

# Scenario D: a git-IGNORED user file inside an otherwise clean external is kept.
# Plain `git status --porcelain` would not report it; the dirty check uses
# --ignored, so the external is treated as dirty and preserved under --all.
apply
ext2="$HOME/.local/share/dotfiles/zsh-plugins/zsh-autosuggestions"
[[ -d "$ext2" ]] || fail "external missing for scenario D: $ext2"
printf 'my-cache\n' > "$ext2/.git/info/exclude"
printf 'user cache data\n' > "$ext2/my-cache"
"$REPO_ROOT/uninstall.sh" --all >/dev/null 2>&1 || fail "uninstall (scenario D) failed"
[[ -f "$ext2/my-cache" ]] || fail "git-ignored user file in a clean external was deleted (data loss)"
pass "git-ignored file in external preserved"

pass "uninstall_safety_test.sh completed"
