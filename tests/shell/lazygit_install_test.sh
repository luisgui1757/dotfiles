#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
TMP_ROOT="$REPO_ROOT/tests/.cache/lazygit-install-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home" "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

have() {
    case "$1" in
        lazygit|sudo) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

YES_ALL=1
DRY_RUN=1
PM=apt

out="$(install_lazygit)"
version_no_v="${LAZYGIT_LINUX_VERSION#v}"
asset="lazygit_${version_no_v}_linux_x86_64.tar.gz"

[[ "$out" == *"github.com/jesseduffield/lazygit/releases/download/${LAZYGIT_LINUX_VERSION}/${asset}"* ]]
[[ "$out" == *"verify sha256 $LAZYGIT_LINUX_X86_64_SHA256"* ]]
[[ "$out" == *"/usr/local/bin/lazygit"* ]]
[[ "$out" == *"\$HOME/.local/bin/lazygit"* ]]

HOME="$TMP_ROOT/home"
export HOME
export DOTFILES_PROVENANCE_DIR="$TMP_ROOT/provenance"
export TMPDIR="$TMP_ROOT/tmp"
DRY_RUN=0

id() {
    if [[ "${1:-}" == "-u" ]]; then
        printf '%s\n' 1000
    else
        command id "$@"
    fi
}

curl() {
    local out_file=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o) out_file="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$out_file" ]] || fail "curl stub did not receive -o"
    printf '%s\n' "archive" > "$out_file"
}

tar() {
    local dest=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -C) dest="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$dest" ]] || fail "tar stub did not receive -C"
    printf '%s\n' "lazygit" > "$dest/lazygit"
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 0
}

install_lazygit_linux >/dev/null
[[ -x "$HOME/.local/bin/lazygit" ]] || fail "lazygit was not installed as executable"
grep -F "$LAZYGIT_LINUX_X86_64_SHA256" "$TMP_ROOT/sha.log" >/dev/null \
    || fail "lazygit install did not verify the pinned checksum"
grep -F "tool=lazygit" "$DOTFILES_PROVENANCE_DIR/lazygit.env" >/dev/null \
    || fail "lazygit provenance marker was not written"
grep -F "version=$LAZYGIT_LINUX_VERSION" "$DOTFILES_PROVENANCE_DIR/lazygit.env" >/dev/null \
    || fail "lazygit provenance marker has the wrong version"
grep -F "command_path=$HOME/.local/bin/lazygit" "$DOTFILES_PROVENANCE_DIR/lazygit.env" >/dev/null \
    || fail "lazygit provenance marker has the wrong command path"

if grep -Eq '^lazygit\|lazygit\|[^|]' "$REPO_ROOT/install-deps.sh"; then
    echo "FAIL: lazygit must not use apt/dnf/pacman package rows on native Linux"
    exit 1
fi

DRY_RUN=1
PM=brew
out="$(install_lazygit)"
[[ "$out" == *"would: brew install lazygit"* ]]
[[ "$out" != *"github.com/jesseduffield/lazygit/releases"* ]]

PM=apk
native_linux_pm() {
    printf '%s\n' "apk"
}
out="$(install_lazygit)"
[[ "$out" == *"would: apk install lazygit"* ]]
[[ "$out" != *"github.com/jesseduffield/lazygit/releases"* ]]

echo "OK"
