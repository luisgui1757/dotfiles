#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

HAVE_COMPILER=0

have() {
    case "$1" in
        cc|gcc|clang|zig|cl) [[ "$HAVE_COMPILER" -eq 1 ]] ;;
        apt-get) return 0 ;;
        *) return 1 ;;
    esac
}

YES_ALL=1
DRY_RUN=1
PM=brew

output="$(install_c_toolchain_linux)"
[[ "$output" == *"apt install build-essential"* ]]

HAVE_COMPILER=1
output="$(install_c_toolchain_linux)"
[[ "$output" == *"C compiler"* ]]
[[ "$output" == *"already installed"* ]]

echo "OK"
