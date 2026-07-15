#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/ghostty-install-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT
export TMPDIR="$TMP_ROOT/tmp"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

assert_eq() {
    [[ "$1" == "$2" ]] || fail "$3 (got '$1', expected '$2')"
}

TEST_ID=ubuntu
TEST_VERSION_ID=24.04
TEST_UBUNTU_VERSION_ID=""
TEST_VERSION_CODENAME=noble
TEST_UBUNTU_CODENAME=noble
TEST_DEBIAN_CODENAME=""
TEST_ARCH=amd64

os_release_value() {
    case "$1" in
        ID) printf '%s\n' "$TEST_ID" ;;
        VERSION_ID) printf '%s\n' "$TEST_VERSION_ID" ;;
        UBUNTU_VERSION_ID) printf '%s\n' "$TEST_UBUNTU_VERSION_ID" ;;
        VERSION_CODENAME) printf '%s\n' "$TEST_VERSION_CODENAME" ;;
        UBUNTU_CODENAME) printf '%s\n' "$TEST_UBUNTU_CODENAME" ;;
        DEBIAN_CODENAME) printf '%s\n' "$TEST_DEBIAN_CODENAME" ;;
        *) return 1 ;;
    esac
}

dpkg() {
    [[ "${1:-}" == "--print-architecture" ]]
    printf '%s\n' "$TEST_ARCH"
}

resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_amd64_24.04.deb" "Ubuntu amd64 asset mismatch"
assert_eq "$GHOSTTY_DEB_SHA256" "478d440153ef544426418efc7d6d8901715359f452c46be29071901a94b8cd47" "Ubuntu amd64 digest mismatch"
assert_eq "$GHOSTTY_DEB_ARCH" "amd64" "Ubuntu amd64 architecture mismatch"
assert_eq "$GHOSTTY_DEB_URL" "https://github.com/mkasberg/ghostty-ubuntu/releases/download/${GHOSTTY_UBUNTU_VERSION}/${GHOSTTY_DEB_ASSET}" "Ubuntu immutable URL mismatch"

TEST_ARCH=arm64
resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_arm64_24.04.deb" "Ubuntu arm64 asset mismatch"
assert_eq "$GHOSTTY_DEB_SHA256" "91063815b6ce3d834d59714b4ad0310f744448b6716836d035b3d331d1923363" "Ubuntu arm64 digest mismatch"

TEST_VERSION_ID=25.10
TEST_VERSION_CODENAME=questing
TEST_UBUNTU_CODENAME=questing
TEST_ARCH=amd64
resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_amd64_25.10.deb" "Ubuntu 25.10 amd64 asset mismatch"
assert_eq "$GHOSTTY_DEB_SHA256" "793bde1c31163d8e1d12ea939c8b941f7908170e57bbf19b121434a0f6621c59" "Ubuntu 25.10 amd64 digest mismatch"

TEST_ARCH=arm64
resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_arm64_25.10.deb" "Ubuntu 25.10 arm64 asset mismatch"
assert_eq "$GHOSTTY_DEB_SHA256" "c6a4fd4fd786b4bdea42036650ef1724f535c4b636329f488f7ece36820d3d6b" "Ubuntu 25.10 arm64 digest mismatch"

TEST_ID=debian
TEST_VERSION_ID=13
TEST_VERSION_CODENAME=trixie
TEST_ARCH=amd64
resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_amd64_trixie.deb" "Debian trixie asset mismatch"
assert_eq "$GHOSTTY_DEB_SHA256" "9fda8e418d7a7f58149ba3ba823a255d6b80f8bb5431b3bd7e912ff597715b2e" "Debian trixie digest mismatch"

TEST_ARCH=arm64
resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_arm64_trixie.deb" "Debian trixie arm64 asset mismatch"
assert_eq "$GHOSTTY_DEB_SHA256" "73f384e62c419d7a7809d686bf579fea5e23f52742b34f70c74d6adf0e72f8ab" "Debian trixie arm64 digest mismatch"

TEST_ID=linuxmint
TEST_VERSION_ID=22
TEST_VERSION_CODENAME=wilma
TEST_UBUNTU_CODENAME=noble
TEST_DEBIAN_CODENAME=""
TEST_ARCH=arm64
resolve_ghostty_deb_asset
assert_eq "$GHOSTTY_DEB_ASSET" "ghostty_1.3.1-0.ppa2_arm64_24.04.deb" "Linux Mint noble mapping mismatch"

TEST_ARCH=riscv64
if resolve_ghostty_deb_asset; then
    echo "FAIL: unsupported Ghostty architecture resolved an asset" >&2
    exit 1
else
    rc=$?
fi
[[ "$rc" -eq 2 ]] || fail "unsupported architecture returned $rc instead of 2"

TEST_ID=ubuntu
TEST_VERSION_ID=24.04
TEST_VERSION_CODENAME=noble
TEST_UBUNTU_CODENAME=noble
TEST_ARCH=amd64
resolve_ghostty_deb_asset

curl() {
    local out=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o)
                out="$2"
                shift 2
                ;;
            -* ) shift ;;
            *)
                printf '%s\n' "$1" > "$TMP_ROOT/url.log"
                shift
                ;;
        esac
    done
    printf '%s\n' "verified deb bytes" > "$out"
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    [[ "$2" == "$GHOSTTY_DEB_SHA256" ]]
}

dpkg-deb() {
    [[ "$1" == "--field" ]]
    case "$3" in
        Package) printf '%s\n' ghostty ;;
        Architecture) printf '%s\n' "$GHOSTTY_DEB_ARCH" ;;
        Version) printf '%s\n' "$GHOSTTY_UBUNTU_PACKAGE_VERSION" ;;
        *) return 2 ;;
    esac
}

have() {
    case "$1" in
        dpkg-deb|ghostty) return 0 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

maybe_sudo() {
    printf '%s\n' "$*" > "$TMP_ROOT/install.log"
}

dpkg-query() {
    printf '%s\n' "$GHOSTTY_UBUNTU_PACKAGE_VERSION"
}

install_verified_ghostty_deb \
    "$GHOSTTY_DEB_URL" "$GHOSTTY_DEB_ASSET" "$GHOSTTY_DEB_SHA256" "$GHOSTTY_DEB_ARCH"

grep -Fx "$GHOSTTY_DEB_URL" "$TMP_ROOT/url.log" >/dev/null || fail "download did not use the immutable URL"
grep -F "$GHOSTTY_DEB_SHA256" "$TMP_ROOT/sha.log" >/dev/null || fail "downloaded bytes were not checked against the selected digest"
grep -F "env DEBIAN_FRONTEND=noninteractive apt-get install -y" "$TMP_ROOT/install.log" >/dev/null \
    || fail "verified local package was not passed to apt through the noninteractive boundary"
if find "$TMP_ROOT/tmp" -mindepth 1 -print -quit | grep -q .; then
    echo "FAIL: successful Ghostty install leaked temporary content" >&2
    exit 1
fi

# Plain Debian does not advertise itself through the Ubuntu-oriented predicate;
# the native apt boundary must still route a reviewed trixie asset into dry-run.
TEST_ID=debian
TEST_VERSION_ID=13
TEST_VERSION_CODENAME=trixie
TEST_UBUNTU_VERSION_ID=""
TEST_UBUNTU_CODENAME=""
TEST_DEBIAN_CODENAME=""
TEST_ARCH=amd64
DRY_RUN=1
YES_ALL=1
PM=apt
is_wsl() { return 1; }
is_ubuntu() { return 1; }
native_linux_pm() { printf '%s\n' apt; }
have() {
    case "$1" in
        ghostty|snap|flatpak) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}
debian_preview="$(install_ghostty_linux)"
[[ "$debian_preview" == *"ghostty_1.3.1-0.ppa2_amd64_trixie.deb"* ]] \
    || fail "plain Debian apt host did not select the reviewed trixie asset: $debian_preview"

echo "OK"
