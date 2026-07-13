#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
# A Homebrew cask receipt is authoritative on macOS even when fontconfig has
# not indexed Apple's font directories yet. Repeated setup must not reinstall
# the cask or trigger an unnecessary Homebrew update.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

PM=brew
DRY_RUN=0
YES_ALL=1
BREW_CALLS="$(mktemp "${TMPDIR:-/tmp}/font-cask.XXXXXX")"
trap 'rm -f "$BREW_CALLS"' EXIT

uname() { printf '%s\n' Darwin; }
fc-list() { return 0; }
brew() {
    printf '%s\n' "$*" >> "$BREW_CALLS"
    [[ "$*" == "list --cask --versions font-hack-nerd-font" ]]
}

hack_nerd_font_installed || fail "Homebrew cask receipt was not accepted"
install_scan_present "Hack Nerd Font" font || fail "pre-flight still reports the font missing"
out="$(install_nerd_font)"
[[ "$out" == *"already installed"* ]] || fail "install path was not idempotent: $out"
if grep -q '^install --cask' "$BREW_CALLS"; then
    fail "installer attempted to reinstall the present font cask"
fi

echo "OK"
