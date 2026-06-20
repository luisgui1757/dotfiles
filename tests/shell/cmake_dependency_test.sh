#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

for pm in brew apt dnf pacman zypper apk; do
    PM="$pm"
    [[ "$(pkg_for cmake)" == "cmake" ]] || fail "pkg_for cmake failed for $pm"
done

grep -q 'install cmake ' "$REPO_ROOT/install-deps.sh" \
    || fail "install-deps.sh no longer installs cmake"
grep -qF 'cmake|cmake|cmake|cmake|cmake|cmake|cmake' "$REPO_ROOT/install-deps.sh" \
    || fail "PKG_TABLE is missing the cross-platform cmake row"
grep -qF 'cmake|command|cmake' "$REPO_ROOT/install-deps.sh" \
    || fail "dependency scan is missing cmake"

echo "OK"
