#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $1"
    exit 1
}

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
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
case "$(command uname -m)" in
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

PM=brew
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
