#!/usr/bin/env bash
# Fail-closed guard for the pinned Herdr Linux binary install. A checksum
# mismatch or a download failure must abort with a FAIL marker and must NEVER
# install a binary. Mirrors wezterm/ghostty install-fail tests.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/herdr-install-fail-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT
export TMPDIR="$TMP_ROOT/tmp"
DEST="$TMP_ROOT/herdr"

curl() {
    local out=""
    if [[ "${HERDR_CURL_MODE:-ok}" == "fail" ]]; then
        return 22
    fi
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o)
                out="$2"
                shift 2
                ;;
            *)
                shift
                ;;
        esac
    done
    printf 'not-a-real-binary\n' > "$out"
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 1
}

# 1) checksum mismatch -> FAIL, no binary installed
if output="$(run_herdr_linux_binary_install "https://example.invalid/herdr" "$HERDR_LINUX_X86_64_SHA256" "$DEST" 2>&1)"; then
    echo "FAIL: run_herdr_linux_binary_install returned success after checksum failure" >&2
    echo "$output" >&2
    exit 1
fi
[[ "$output" == *"FAIL: checksum mismatch for herdr"* ]]
grep -F "$HERDR_LINUX_X86_64_SHA256" "$TMP_ROOT/sha.log" >/dev/null
if [[ -e "$DEST" ]]; then
    echo "FAIL: herdr binary installed after checksum failure" >&2
    exit 1
fi

# 2) download failure -> FAIL, no binary installed
HERDR_CURL_MODE=fail
if output="$(run_herdr_linux_binary_install "https://example.invalid/herdr" "$HERDR_LINUX_X86_64_SHA256" "$DEST" 2>&1)"; then
    echo "FAIL: run_herdr_linux_binary_install returned success after download failure" >&2
    echo "$output" >&2
    exit 1
fi
[[ "$output" == *"FAIL: could not download herdr binary"* ]]
if [[ -e "$DEST" ]]; then
    echo "FAIL: herdr binary installed after download failure" >&2
    exit 1
fi

# 3) wrapper path -> FAIL marker and nonzero, so --all/e2e cannot fake-green.
YES_ALL=1
DRY_RUN=0
PM=unknown
HOME="$TMP_ROOT/home"
have() { return 1; }
homebrew_bin() { return 1; }
native_linux_pm() { printf '%s\n' "unknown"; }
require_downloader() { return 0; }
uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}
run_herdr_linux_binary_install() {
    return 45
}
if output="$(install_herdr 2>&1)"; then
    echo "FAIL: install_herdr returned success after binary install failure" >&2
    echo "$output" >&2
    exit 1
fi
[[ "$output" == *"FAIL: herdr binary install failed"* ]]

echo "OK"
