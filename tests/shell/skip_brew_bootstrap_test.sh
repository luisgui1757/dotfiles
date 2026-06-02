#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $1"; exit 1; }

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

have() {
    case "$1" in
        brew) return 1 ;;
        apt-get) return 0 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

# CI runners ship Homebrew at a standard prefix, which homebrew_bin would find via
# its filesystem probe (independent of `have brew`); stub it so detect_pm exercises
# the native-PM path instead of short-circuiting to brew.
homebrew_bin() { return 1; }

ask() {
    fail "maybe_install_brew prompted despite DOTFILES_SKIP_BREW_BOOTSTRAP=1"
}

export DOTFILES_SKIP_BREW_BOOTSTRAP=1
PM="$(detect_pm)"
[[ "$PM" == "apt" ]] || fail "expected native apt before brew bootstrap, got $PM"

if [[ "$PM" != "brew" && "$(uname -s)" == "Linux" ]]; then
    if maybe_install_brew; then
        PM="$(detect_pm)"
    fi
fi

[[ "$PM" == "apt" ]] || fail "expected native apt to be kept, got $PM"

echo "OK"
