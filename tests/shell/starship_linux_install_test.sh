#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$REPO_ROOT/tests/.cache/starship-linux-install-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/tmp" "$TMP_ROOT/home"
trap 'rm -rf "$TMP_ROOT"' EXIT

HOME="$TMP_ROOT/home"
export HOME
export STARSHIP_TEST_ROOT="$TMP_ROOT"
export TMPDIR="$TMP_ROOT/tmp"
PATH="$TMP_ROOT/bin:/usr/bin:/bin"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

id() {
    if [[ "${1:-}" == "-u" ]]; then
        printf '%s\n' 1000
    else
        command id "$@"
    fi
}

have() {
    case "$1" in
        starship|shasum|sudo) return 1 ;;
        curl|sha256sum|tar) return 0 ;;
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
    if [[ "${STARSHIP_SHA_MODE:-good}" == "good" ]]; then
        printf '%s  %s\n' "$STARSHIP_LINUX_X86_64_SHA256" "$1"
    else
        printf '%064d  %s\n' 0 "$1"
    fi
}

tar() {
    printf '%s\n' "$*" >> "$TMP_ROOT/tar.log"
    local dest=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -C) dest="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$dest" ]] || fail "tar stub did not receive -C"
    printf '%s\n' "starship" > "$dest/starship"
}

native_linux_pm() {
    printf '%s\n' "apt"
}

YES_ALL=1
DRY_RUN=1
PM=apt

out="$(install_starship)"
url="https://github.com/starship/starship/releases/download/${STARSHIP_VERSION}/starship-x86_64-unknown-linux-gnu.tar.gz"

[[ "$out" == *"$url"* ]] || fail "dry run did not include pinned starship URL"
[[ "$out" == *"verify sha256 $STARSHIP_LINUX_X86_64_SHA256"* ]] || fail "dry run did not include x86_64 sha256"
[[ "$out" == *"\$HOME/.local/bin/starship"* ]] || fail "dry run did not mention user-local fallback"

DRY_RUN=0
STARSHIP_SHA_MODE=bad
if install_starship_linux > "$TMP_ROOT/bad.out" 2>&1; then
    fail "starship install returned success after checksum mismatch"
fi
grep -F "FAIL: checksum mismatch for starship-x86_64-unknown-linux-gnu.tar.gz" "$TMP_ROOT/bad.out" >/dev/null \
    || fail "checksum mismatch did not emit a FAIL marker"
[[ ! -e "$HOME/.local/bin/starship" ]] || fail "starship installed after checksum mismatch"
[[ ! -e "$TMP_ROOT/tar.log" ]] || fail "tar ran after checksum mismatch"

STARSHIP_SHA_MODE=good
install_starship_linux > "$TMP_ROOT/good.out" 2>&1
[[ -x "$HOME/.local/bin/starship" ]] || fail "starship was not installed as executable"
grep -F "$url" "$TMP_ROOT/curl.log" >/dev/null \
    || fail "curl did not request the pinned starship URL"
printf '%s\n' "$PATH" | grep -F "$HOME/.local/bin" >/dev/null \
    || fail "starship install did not add user-local bin to PATH"

PM=brew
DRY_RUN=1
out="$(install_starship)"
[[ "$out" == *"would: brew install starship"* ]] || fail "brew path did not use brew install"
[[ "$out" != *"github.com/starship/starship/releases"* ]] || fail "brew path used the direct GitHub release"

PM=apk
native_linux_pm() {
    printf '%s\n' "apk"
}
out="$(install_starship)"
[[ "$out" == *"would: apk install starship"* ]] || fail "apk path did not use the native package"
[[ "$out" != *"github.com/starship/starship/releases"* ]] || fail "apk path used the direct GitHub release"

echo "OK"
