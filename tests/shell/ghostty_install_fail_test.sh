#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/ghostty-install-fail-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT
export TMPDIR="$TMP_ROOT/tmp"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

URL="https://github.com/mkasberg/ghostty-ubuntu/releases/download/${GHOSTTY_UBUNTU_VERSION}/ghostty-test.deb"
ASSET="ghostty-test.deb"
EXPECTED_SHA="478d440153ef544426418efc7d6d8901715359f452c46be29071901a94b8cd47"
EXPECTED_ARCH=amd64
CURL_MODE=ok
SHA_MODE=ok
PACKAGE_VALUE=ghostty
ARCH_VALUE=amd64
VERSION_VALUE="${GHOSTTY_UBUNTU_VERSION/-0-/-0~}"
INSTALL_MODE=ok
INSTALLED_VERSION="$VERSION_VALUE"
GHOSTTY_PRESENT=1

curl() {
    local out=""
    if [[ "$CURL_MODE" == "fail" ]]; then
        return 22
    fi
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o)
                out="$2"
                shift 2
                ;;
            *) shift ;;
        esac
    done
    if [[ "$CURL_MODE" == "partial" ]]; then
        printf '%s\n' partial > "$out"
    else
        printf '%s\n' complete > "$out"
    fi
}

verify_sha256() {
    [[ "$SHA_MODE" == "ok" ]]
}

dpkg-deb() {
    case "$3" in
        Package) printf '%s\n' "$PACKAGE_VALUE" ;;
        Architecture) printf '%s\n' "$ARCH_VALUE" ;;
        Version) printf '%s\n' "$VERSION_VALUE" ;;
        *) return 2 ;;
    esac
}

have() {
    case "$1" in
        dpkg-deb) return 0 ;;
        ghostty) [[ "$GHOSTTY_PRESENT" -eq 1 ]] ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

maybe_sudo() {
    printf '%s\n' "$*" >> "$TMP_ROOT/apt.log"
    [[ "$INSTALL_MODE" == "ok" ]]
}

dpkg-query() {
    printf '%s\n' "$INSTALLED_VERSION"
}

assert_clean() {
    if find "$TMP_ROOT/tmp" -mindepth 1 -print -quit | grep -q .; then
        echo "FAIL: Ghostty failure leaked temporary content" >&2
        exit 1
    fi
}

assert_no_apt() {
    [[ ! -e "$TMP_ROOT/apt.log" ]] || fail "apt ran before package proof completed"
}

assert_apt() {
    [[ -s "$TMP_ROOT/apt.log" ]] || fail "verified package never reached apt"
}

mktemp() { return 1; }
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: Ghostty staging failure returned success" >&2
    exit 1
fi
[[ "$output" == *"could not create private staging"* ]] || fail "staging failure diagnostic missing: $output"
assert_no_apt
unset -f mktemp

CURL_MODE=fail
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: Ghostty download failure returned success" >&2
    exit 1
fi
[[ "$output" == *"could not download pinned Ghostty .deb"* ]] || fail "download failure diagnostic missing: $output"
assert_no_apt
assert_clean

CURL_MODE=partial
SHA_MODE=fail
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: partial Ghostty .deb returned success" >&2
    exit 1
fi
[[ "$output" == *"checksum mismatch for $ASSET"* ]] || fail "checksum failure diagnostic missing: $output"
assert_no_apt
assert_clean

CURL_MODE=ok
SHA_MODE=ok
PACKAGE_VALUE=not-ghostty
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: wrong Ghostty package metadata returned success" >&2
    exit 1
fi
[[ "$output" == *"unexpected package metadata"* ]] || fail "package metadata diagnostic missing: $output"
assert_no_apt
assert_clean

PACKAGE_VALUE=ghostty
ARCH_VALUE=arm64
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: wrong Ghostty architecture returned success" >&2
    exit 1
fi
[[ "$output" == *"unexpected package metadata"* ]] || fail "architecture metadata diagnostic missing: $output"
assert_no_apt
assert_clean

ARCH_VALUE=amd64
VERSION_VALUE=wrong-version
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: wrong Ghostty version returned success" >&2
    exit 1
fi
[[ "$output" == *"unexpected package metadata"* ]] || fail "version metadata diagnostic missing: $output"
assert_no_apt
assert_clean

VERSION_VALUE="${GHOSTTY_UBUNTU_VERSION/-0-/-0~}"
INSTALL_MODE=fail
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: Ghostty apt failure returned success" >&2
    exit 1
fi
[[ "$output" == *"could not install verified Ghostty package"* ]] || fail "apt failure diagnostic missing: $output"
assert_apt
assert_clean

rm -f "$TMP_ROOT/apt.log"
INSTALL_MODE=ok
INSTALLED_VERSION=wrong-installed-version
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: Ghostty installed-version mismatch returned success" >&2
    exit 1
fi
[[ "$output" == *"Remove the package with 'sudo apt-get remove ghostty'"* ]] || fail "post-install recovery diagnostic missing: $output"
assert_apt
assert_clean

rm -f "$TMP_ROOT/apt.log"
INSTALLED_VERSION="$GHOSTTY_UBUNTU_PACKAGE_VERSION"
GHOSTTY_PRESENT=0
if output="$(install_verified_ghostty_deb "$URL" "$ASSET" "$EXPECTED_SHA" "$EXPECTED_ARCH" 2>&1)"; then
    echo "FAIL: missing Ghostty executable returned success" >&2
    exit 1
fi
[[ "$output" == *"publication could not be validated"* ]] || fail "missing-executable diagnostic missing: $output"
assert_apt
assert_clean

echo "OK"
