#!/usr/bin/env bash
set -euo pipefail

# Determinism: the legacy install_zsh_plugins slice honors ${XDG_DATA_HOME:-...},
# while the chezmoi externals install to a fixed ~/.local/share path. Unset
# XDG_DATA_HOME so BOTH sides resolve to $HOME/.local/share and plugin pin checks
# are apples-to-apples. (An XDG_DATA_HOME-aware managed root is Wave B.)
unset XDG_DATA_HOME

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

apply_clean_home "$COMMIT_HOME"
pass "commit-assert oracle fixture applied cleanly"

autocomplete_dir="$COMMIT_HOME/.local/share/dotfiles/zsh-plugins/zsh-autocomplete"
[[ -d "$autocomplete_dir/.git" ]] || \
    fail "zsh-autocomplete checkout missing at $autocomplete_dir"

pinned_head="$(git_head "$autocomplete_dir")"
autocomplete_ref="$(git -C "$autocomplete_dir" describe --exact-match --tags HEAD 2>/dev/null || true)"
[[ -n "$autocomplete_ref" ]] || \
    fail "zsh-autocomplete HEAD is not at a tag; cannot deepen a known pinned ref"
if ! git -C "$autocomplete_dir" rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    git -C "$autocomplete_dir" fetch --deepen=1 origin "$autocomplete_ref" >/dev/null 2>&1 || \
        fail "could not deepen zsh-autocomplete checkout to find a different real commit"
fi
bad_head="$(git -C "$autocomplete_dir" rev-parse --verify HEAD~1)" || \
    fail "could not resolve zsh-autocomplete HEAD~1 after deepen"
[[ "$bad_head" != "$pinned_head" ]] || \
    fail "zsh-autocomplete alternate commit unexpectedly equals pinned HEAD"
git -C "$autocomplete_dir" checkout --detach "$bad_head" >/dev/null 2>&1 || \
    fail "could not corrupt zsh-autocomplete checkout to $bad_head"
pass "commit-assert oracle corrupted zsh-autocomplete HEAD"

commit_assert="$WORK/verify-zsh-plugin-pins.sh"
commit_assert_log="$WORK/verify-zsh-plugin-pins.log"
render_commit_assert "$commit_assert"

commit_assert_rc=0
env HOME="$COMMIT_HOME" bash "$commit_assert" > "$commit_assert_log" 2>&1 || \
    commit_assert_rc=$?
if [[ "$commit_assert_rc" -eq 0 ]]; then
    print_log "$commit_assert_log"
    fail "commit-assert accepted a bad zsh-autocomplete pin"
fi
if ! grep -Fq "FAIL: zsh-autocomplete HEAD" "$commit_assert_log"; then
    print_log "$commit_assert_log"
    fail "commit-assert failed without the zsh-autocomplete FAIL line"
fi
pass "commit-assert fires on a bad zsh-autocomplete pin"

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
