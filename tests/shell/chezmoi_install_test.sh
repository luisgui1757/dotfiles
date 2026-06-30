#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
TMP_ROOT="$REPO_ROOT/tests/.cache/chezmoi-install-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home" "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

fail() {
    echo "FAIL: $1"
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
        chezmoi) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

have_any() {
    local b
    for b in "$@"; do
        if have "$b"; then return 0; fi
    done
    return 1
}

YES_ALL=1
DRY_RUN=1
PM=apt
case "$(uname -m)" in
    x86_64|amd64)
        expected_chezmoi_sha="$CHEZMOI_LINUX_X86_64_SHA256"
        ;;
    aarch64|arm64)
        expected_chezmoi_sha="$CHEZMOI_LINUX_ARM64_SHA256"
        ;;
    *)
        expected_chezmoi_sha=""
        ;;
esac

out="$(install_chezmoi)"
[[ "$out" == *"github.com/twpayne/chezmoi/releases/download/$CHEZMOI_VERSION/"* ]] || fail "native Linux path did not use the pinned GitHub release"
[[ -n "$expected_chezmoi_sha" ]] || fail "test host arch is unsupported by install_chezmoi"
[[ "$out" == *"verify sha256 $expected_chezmoi_sha"* ]] || fail "native Linux path did not verify the expected checksum"
[[ "$out" == *"extract chezmoi -> \$HOME/.local/bin/chezmoi"* ]] || fail "native Linux path did not install to ~/.local/bin"
[[ "$out" != *"get.chezmoi.io"* ]] || fail "native Linux path still used get.chezmoi.io"
[[ "$out" != *"brew install chezmoi"* ]] || fail "native Linux path used brew"

HOME="$TMP_ROOT/home"
export HOME
export DOTFILES_PROVENANCE_DIR="$TMP_ROOT/provenance"
export TMPDIR="$TMP_ROOT/tmp"
DRY_RUN=0

curl() {
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

tar() {
    local dest=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -C) dest="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$dest" ]] || fail "tar stub did not receive -C"
    printf '%s\n' "chezmoi" > "$dest/chezmoi"
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 0
}

install_chezmoi >/dev/null
[[ -x "$HOME/.local/bin/chezmoi" ]] || fail "chezmoi was not installed as executable"
grep -F "$expected_chezmoi_sha" "$TMP_ROOT/sha.log" >/dev/null \
    || fail "chezmoi install did not verify the pinned checksum"
grep -F "tool=chezmoi" "$DOTFILES_PROVENANCE_DIR/chezmoi.env" >/dev/null \
    || fail "chezmoi provenance marker was not written"
grep -F "version=$CHEZMOI_VERSION" "$DOTFILES_PROVENANCE_DIR/chezmoi.env" >/dev/null \
    || fail "chezmoi provenance marker has the wrong version"
grep -F "command_path=$HOME/.local/bin/chezmoi" "$DOTFILES_PROVENANCE_DIR/chezmoi.env" >/dev/null \
    || fail "chezmoi provenance marker has the wrong command path"

PM=brew
DRY_RUN=1
out="$(install_chezmoi)"
[[ "$out" == *"would: brew install chezmoi"* ]] || fail "brew path did not use brew install"
[[ "$out" != *"get.chezmoi.io"* ]] || fail "brew path used get.chezmoi.io"

workflow_version="$(awk '/^[[:space:]]*CHEZMOI_VERSION:/{print $2; exit}' "$REPO_ROOT/.github/workflows/test.yml")"
[[ "$workflow_version" == "$CHEZMOI_VERSION" ]] || fail "CHEZMOI_VERSION mismatch: install-deps.sh=$CHEZMOI_VERSION test.yml=$workflow_version"

grep -qF 'chezmoi|chezmoi|||||' "$REPO_ROOT/install-deps.sh" \
    || fail "PKG_TABLE is missing the brew chezmoi row"
grep -qE '^install_chezmoi$' "$REPO_ROOT/install-deps.sh" \
    || fail "install-deps.sh no longer calls install_chezmoi"

echo "OK"
