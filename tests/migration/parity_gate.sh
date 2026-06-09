#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SRC="$REPO_ROOT/home"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

pass() {
    echo "PASS: $*"
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        fail "required command '$1' is not on PATH"
    fi
}

sha() {
    local path="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$path" | cut -d' ' -f1
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$path" | cut -d' ' -f1
    else
        fail "sha256sum or shasum is required"
    fi
}

mode() {
    local path="$1"
    if stat -c '%a' "$path" >/dev/null 2>&1; then
        stat -c '%a' "$path"
    else
        stat -f '%Lp' "$path"
    fi
}

deref() {
    local path="$1"
    if readlink -f "$path" >/dev/null 2>&1; then
        readlink -f "$path"
    else
        python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$path"
    fi
}

git_head() {
    local dir="$1"
    git -C "$dir" rev-parse HEAD
}

assert_file_pair() {
    local label="$1" rel_path="$2" old_path new_path old_real new_real old_sha new_sha old_mode new_mode

    old_path="$HOME_OLD/$rel_path"
    new_path="$HOME_NEW/$rel_path"

    [[ -e "$old_path" ]] || fail "$label: missing old path $old_path"
    [[ -e "$new_path" ]] || fail "$label: missing new path $new_path"
    pass "$label: exists on both sides"

    [[ -L "$old_path" ]] || fail "$label: old path is not a symlink: $old_path"
    [[ -L "$new_path" ]] || fail "$label: new path is not a symlink: $new_path"
    pass "$label: symlink type on both sides"

    old_real="$(deref "$old_path")"
    new_real="$(deref "$new_path")"
    old_sha="$(sha "$old_real")"
    new_sha="$(sha "$new_real")"
    [[ "$old_sha" == "$new_sha" ]] || fail "$label: dereferenced SHA mismatch old=$old_sha new=$new_sha"
    pass "$label: dereferenced content SHA matches"

    old_mode="$(mode "$old_real")"
    new_mode="$(mode "$new_real")"
    [[ "$old_mode" == "$new_mode" ]] || fail "$label: dereferenced mode mismatch old=$old_mode new=$new_mode"
    pass "$label: dereferenced mode matches ($old_mode)"
}

assert_plugin_pair() {
    local label="$1" rel_dir="$2" old_dir new_dir old_head new_head

    old_dir="$HOME_OLD/$rel_dir"
    new_dir="$HOME_NEW/$rel_dir"

    [[ -d "$old_dir/.git" ]] || fail "$label: old git checkout missing at $old_dir"
    [[ -d "$new_dir/.git" ]] || fail "$label: new git checkout missing at $new_dir"
    pass "$label: git checkouts exist on both sides"

    old_head="$(git_head "$old_dir")"
    new_head="$(git_head "$new_dir")"
    [[ "$old_head" == "$new_head" ]] || fail "$label: HEAD mismatch old=$old_head new=$new_head"
    pass "$label: HEAD matches ($old_head)"
}

require_cmd bash
require_cmd chezmoi
require_cmd git
require_cmd python3

case "$(uname -s)" in
    Darwin)
        target_os="darwin"
        lazygit_rel="Library/Application Support/lazygit/config.yml"
        ;;
    Linux)
        target_os="linux"
        lazygit_rel=".config/lazygit/config.yml"
        ;;
    *)
        fail "unsupported parity host OS: $(uname -s); expected Linux or Darwin"
        ;;
esac
pass "detected parity host OS: $target_os"

HOME_OLD="$(mktemp -d)"
HOME_NEW="$(mktemp -d)"
trap 'rm -rf "$HOME_OLD" "$HOME_NEW"' EXIT

env HOME="$HOME_OLD" "$REPO_ROOT/bootstrap.sh"
pass "old path bootstrap slice applied"

# Interpolate REPO_ROOT into the command string so install-deps.sh is sourced
# with EMPTY positional args. Its top-level arg parser (install-deps.sh:38-47)
# runs on `$@` BEFORE the INSTALL_DEPS_SOURCE_ONLY seam, so a stray "$REPO_ROOT"
# passed as $1 would be rejected as "Unknown arg" and exit 2.
# Set YES_ALL=1 AFTER sourcing: install-deps.sh:17 initializes YES_ALL=0 at
# source time, which clobbers an env-passed YES_ALL=1. Without the auto-accept,
# install_zsh_plugins' ask() gate (install-deps.sh:199-207) skips the install on
# a non-interactive stdin and P3/P4 false-FAIL.
env HOME="$HOME_OLD" INSTALL_DEPS_SOURCE_ONLY=1 bash -c \
    'source "'"$REPO_ROOT"'/install-deps.sh"; YES_ALL=1; install_zsh_plugins'
pass "old path zsh plugin slice applied"

env HOME="$HOME_NEW" chezmoi --source "$SRC" init
env HOME="$HOME_NEW" chezmoi --source "$SRC" apply
pass "new path chezmoi apply completed"

assert_file_pair "P1 ~/.tmux.conf" ".tmux.conf"
assert_file_pair "P2 lazygit config" "$lazygit_rel"

assert_plugin_pair \
    "P3 zsh-autocomplete" \
    ".local/share/dotfiles/zsh-plugins/zsh-autocomplete"
assert_plugin_pair \
    "P4 zsh-autosuggestions" \
    ".local/share/dotfiles/zsh-plugins/zsh-autosuggestions"

repo_lazygit_sha="$(sha "$REPO_ROOT/lazygit/config.yml")"
template_lazygit_sha="$(sha "$SRC/.chezmoitemplates/lazygit/config.yml")"
[[ "$repo_lazygit_sha" == "$template_lazygit_sha" ]] || \
    fail "single-source lazygit SHA mismatch repo=$repo_lazygit_sha template=$template_lazygit_sha"
pass "single-source lazygit SHA matches"
