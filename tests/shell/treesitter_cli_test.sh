#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/home" "$TMP_ROOT/fakebin"
HOME="$TMP_ROOT/home"
export HOME
export DOTFILES_PROVENANCE_DIR="$TMP_ROOT/provenance"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

have() {
    case "$1" in
        tree-sitter|shasum) return 1 ;;
        curl|sha256sum|unzip) return 0 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

curl() {
    printf '%s\n' "$*" >> "$TMP_ROOT/curl.log"
    local out=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o) out="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$out" ]] || fail "curl stub did not receive -o"
    printf '%s\n' "archive" > "$out"
}

sha256sum() {
    if [[ "${TREE_SITTER_SHA_MODE:-good}" == "good" ]]; then
        printf '%s  %s\n' "$TREE_SITTER_CLI_LINUX_X86_64_SHA256" "$1"
    else
        printf '%064d  %s\n' 0 "$1"
    fi
}

unzip() {
    printf '%s\n' "$*" >> "$TMP_ROOT/unzip.log"
    local dest=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -d) dest="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$dest" ]] || fail "unzip stub did not receive -d"
    mkdir -p "$dest"
    printf '%s\n' "tree-sitter" > "$dest/tree-sitter"
}

YES_ALL=1
DRY_RUN=1
PM=apt

out="$(install_tree_sitter_cli_linux)"
asset="tree-sitter-cli-linux-x64.zip"
url="https://github.com/tree-sitter/tree-sitter/releases/download/${TREE_SITTER_CLI_LINUX_VERSION}/${asset}"

[[ "$out" == *"$url"* ]] || fail "dry run did not include pinned tree-sitter URL"
[[ "$out" == *"verify sha256 $TREE_SITTER_CLI_LINUX_X86_64_SHA256"* ]] || fail "dry run did not include x64 sha256"
[[ "$out" == *"\$HOME/.local/bin/tree-sitter"* ]] || fail "dry run did not target user-local tree-sitter"

DRY_RUN=0
TREE_SITTER_SHA_MODE=bad
if install_tree_sitter_cli_linux > "$TMP_ROOT/bad.out" 2>&1; then
    fail "tree-sitter install returned success after checksum mismatch"
fi
grep -F "FAIL: checksum mismatch for $asset" "$TMP_ROOT/bad.out" >/dev/null \
    || fail "checksum mismatch did not emit a FAIL marker"
[[ ! -e "$HOME/.local/bin/tree-sitter" ]] || fail "tree-sitter installed after checksum mismatch"
[[ ! -e "$TMP_ROOT/unzip.log" ]] || fail "unzip ran after checksum mismatch"

TREE_SITTER_SHA_MODE=good
install_tree_sitter_cli_linux > "$TMP_ROOT/good.out" 2>&1
[[ -x "$HOME/.local/bin/tree-sitter" ]] || fail "tree-sitter was not installed as executable"
grep -F "$url" "$TMP_ROOT/curl.log" >/dev/null \
    || fail "curl did not request the pinned tree-sitter URL"
printf '%s\n' "$PATH" | grep -F "$HOME/.local/bin" >/dev/null \
    || fail "tree-sitter install did not add user-local bin to PATH"
grep -F "tool=tree-sitter" "$DOTFILES_PROVENANCE_DIR/tree-sitter.env" >/dev/null \
    || fail "tree-sitter provenance marker was not written"
grep -F "version=$TREE_SITTER_CLI_LINUX_VERSION" "$DOTFILES_PROVENANCE_DIR/tree-sitter.env" >/dev/null \
    || fail "tree-sitter provenance marker has the wrong version"
grep -F "sha256=$TREE_SITTER_CLI_LINUX_X86_64_SHA256" "$DOTFILES_PROVENANCE_DIR/tree-sitter.env" >/dev/null \
    || fail "tree-sitter provenance marker has the wrong checksum"
grep -F "command_path=$HOME/.local/bin/tree-sitter" "$DOTFILES_PROVENANCE_DIR/tree-sitter.env" >/dev/null \
    || fail "tree-sitter provenance marker has the wrong command path"

echo "OK"
