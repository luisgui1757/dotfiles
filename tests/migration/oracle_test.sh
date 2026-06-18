#!/usr/bin/env bash
set -euo pipefail

# Determinism: zsh plugin externals use a fixed ~/.local/share path. The test
# sets XDG_DATA_HOME to a hostile value below so a future XDG-aware drift cannot
# still pass by accident.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SRC="$REPO_ROOT/home"
VERIFY_ZSH_PINS_TMPL="$SRC/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl"

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

apply_clean_home() {
    local home="$1"
    env HOME="$home" chezmoi --source "$SRC" init
    env HOME="$home" chezmoi --source "$SRC" apply
}

render_commit_assert() {
    local dest="$1"
    chezmoi --source "$SRC" execute-template --override-data '{"targetOS":"linux"}' \
        < "$VERIFY_ZSH_PINS_TMPL" > "$dest"
    [[ -s "$dest" ]] || fail "commit-assert template rendered empty"
}

print_log() {
    local path="$1"
    sed 's/^/  /' "$path" >&2
}

require_cmd bash
require_cmd chezmoi
require_cmd git
require_cmd python3

case "$(uname -s)" in
    Darwin)
        target_os="darwin"
        ;;
    Linux)
        target_os="linux"
        ;;
    *)
        fail "unsupported oracle host OS: $(uname -s); expected Linux or Darwin"
        ;;
esac
pass "detected oracle host OS: $target_os"

COMMIT_HOME="$(mktemp -d)"
VERIFY_HOME="$(mktemp -d)"
WORK="$(mktemp -d)"
trap 'rm -rf "$COMMIT_HOME" "$VERIFY_HOME" "$WORK"' EXIT
export XDG_DATA_HOME="$WORK/xdg-data"

apply_clean_home "$COMMIT_HOME"
pass "commit-assert oracle fixture applied cleanly"

fzf_tab_dir="$COMMIT_HOME/.local/share/dotfiles/zsh-plugins/fzf-tab"
[[ -d "$fzf_tab_dir/.git" ]] || \
    fail "fzf-tab checkout missing at $fzf_tab_dir"

pinned_head="$(git_head "$fzf_tab_dir")"
fzf_tab_ref="$(git -C "$fzf_tab_dir" describe --exact-match --tags HEAD 2>/dev/null || true)"
[[ -n "$fzf_tab_ref" ]] || \
    fail "fzf-tab HEAD is not at a tag; cannot deepen a known pinned ref"
if ! git -C "$fzf_tab_dir" rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    git -C "$fzf_tab_dir" fetch --deepen=1 origin "$fzf_tab_ref" >/dev/null 2>&1 || \
        fail "could not deepen fzf-tab checkout to find a different real commit"
fi
bad_head="$(git -C "$fzf_tab_dir" rev-parse --verify HEAD~1)" || \
    fail "could not resolve fzf-tab HEAD~1 after deepen"
[[ "$bad_head" != "$pinned_head" ]] || \
    fail "fzf-tab alternate commit unexpectedly equals pinned HEAD"
git -C "$fzf_tab_dir" checkout --detach "$bad_head" >/dev/null 2>&1 || \
    fail "could not corrupt fzf-tab checkout to $bad_head"
pass "commit-assert oracle corrupted fzf-tab HEAD"

commit_assert="$WORK/verify-zsh-plugin-pins.sh"
commit_assert_log="$WORK/verify-zsh-plugin-pins.log"
render_commit_assert "$commit_assert"

commit_assert_rc=0
env HOME="$COMMIT_HOME" bash "$commit_assert" > "$commit_assert_log" 2>&1 || \
    commit_assert_rc=$?
if [[ "$commit_assert_rc" -eq 0 ]]; then
    print_log "$commit_assert_log"
    fail "commit-assert accepted a bad fzf-tab pin"
fi
if ! grep -Fq "FAIL: fzf-tab HEAD" "$commit_assert_log"; then
    print_log "$commit_assert_log"
    fail "commit-assert failed without the fzf-tab FAIL line"
fi
pass "commit-assert fires on a bad fzf-tab pin"

apply_clean_home "$VERIFY_HOME"
pass "verify oracle fixture applied cleanly"

tmux_target="$VERIFY_HOME/.tmux.conf"
[[ -e "$tmux_target" ]] || fail "managed tmux target missing at $tmux_target"
rm -f "$tmux_target"
printf '%s\n' "oracle drift: this regular file must not match chezmoi source" > "$tmux_target"

verify_log="$WORK/verify-drift.log"
verify_rc=0
env HOME="$VERIFY_HOME" chezmoi --source "$SRC" verify > "$verify_log" 2>&1 || verify_rc=$?
if [[ "$verify_rc" -eq 0 ]]; then
    print_log "$verify_log"
    fail "chezmoi verify accepted drifted ~/.tmux.conf"
fi
pass "chezmoi verify catches drift in ~/.tmux.conf"
