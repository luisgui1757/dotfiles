#!/usr/bin/env bash
# Fail-closed guard for the pinned WezTerm .deb install. A checksum mismatch or
# a download failure must abort with a FAIL marker and must NEVER touch the
# package database (no `apt-get install`). Mirrors ghostty_install_fail_test.sh.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/wezterm-install-fail-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export TMPDIR="$TMP_ROOT/tmp"
export WEZTERM_FAIL_TEST_ROOT="$TMP_ROOT"

# curl stub: writes a fake .deb (ok mode) or fails (fail mode).
curl() {
    local out=""
    if [[ "${WEZTERM_CURL_MODE:-ok}" == "fail" ]]; then
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
    printf 'not-a-real-deb\n' > "$out"
}

# maybe_sudo records any apt-get invocation so we can prove the package database
# is never touched after a failed download/checksum. It must not actually run.
maybe_sudo() {
    if [[ "$*" == *"apt-get "* ]]; then
        printf '%s\n' "$*" >> "$WEZTERM_FAIL_TEST_ROOT/apt.log"
    fi
    return 0
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 1
}

# 1) checksum mismatch -> FAIL, no apt-get install
if output="$(run_wezterm_deb_install "https://example.invalid/wezterm.deb" "$WEZTERM_DEB_AMD64_SHA256" 2>&1)"; then
    echo "FAIL: run_wezterm_deb_install returned success after checksum failure" >&2
    echo "$output" >&2
    exit 1
fi
[[ "$output" == *"FAIL: checksum mismatch for WezTerm .deb"* ]]
grep -F "$WEZTERM_DEB_AMD64_SHA256" "$TMP_ROOT/sha.log" >/dev/null
if [[ -e "$WEZTERM_FAIL_TEST_ROOT/apt.log" ]]; then
    echo "FAIL: apt-get install ran after checksum failure" >&2
    exit 1
fi

# 2) download failure -> FAIL, no apt-get install
WEZTERM_CURL_MODE=fail
if output="$(run_wezterm_deb_install "https://example.invalid/wezterm.deb" "$WEZTERM_DEB_AMD64_SHA256" 2>&1)"; then
    echo "FAIL: run_wezterm_deb_install returned success after download failure" >&2
    echo "$output" >&2
    exit 1
fi
[[ "$output" == *"FAIL: could not download WezTerm .deb"* ]]
if [[ -e "$WEZTERM_FAIL_TEST_ROOT/apt.log" ]]; then
    echo "FAIL: apt-get install ran after download failure" >&2
    exit 1
fi

# 3) wrapper path -> FAIL marker and nonzero, so --all/e2e cannot fake-green.
YES_ALL=1
DRY_RUN=0
PM=apt
have() { return 1; }
is_wsl() { return 1; }
is_ubuntu() { return 0; }
require_downloader() { return 0; }
uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}
run_wezterm_deb_install() {
    return 44
}
if output="$(install_wezterm_linux 2>&1)"; then
    echo "FAIL: install_wezterm_linux returned success after .deb install failure" >&2
    echo "$output" >&2
    exit 1
fi
[[ "$output" == *"FAIL: WezTerm .deb install failed"* ]]

echo "OK"
