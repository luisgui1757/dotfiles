#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
# Install mode is presence-driven. A Homebrew cask receipt proves a macOS app
# is installed even when its optional CLI is not exported on PATH; setup must
# not turn that state into an implicit upgrade.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

PM=brew
DRY_RUN=0
YES_ALL=1
BREW_CALLS="$(mktemp "${TMPDIR:-/tmp}/macos-cask.XXXXXX")"
trap 'rm -f "$BREW_CALLS"' EXIT

have() { return 1; }
brew() {
    printf '%s\n' "$*" >> "$BREW_CALLS"
    case "$*" in
        "list --cask --versions ghostty") printf '%s\n' "ghostty 1.3.1" ;;
        "list --cask --versions wezterm") printf '%s\n' "wezterm 20240203-110809-5046fc22" ;;
        "list --cask --versions aerospace") printf '%s\n' "aerospace 0.20.2-Beta" ;;
        *) return 1 ;;
    esac
}

for installer in install_ghostty_macos install_wezterm_macos install_aerospace_macos; do
    out="$($installer)"
    [[ "$out" == *"already installed"* ]] || fail "$installer did not accept the cask receipt: $out"
done

install_scan_present ghostty macos-cask ghostty ||
    fail "pre-flight did not accept the Ghostty cask receipt"
[[ "$(install_scan_version ghostty macos-cask ghostty)" == "ghostty 1.3.1" ]] ||
    fail "pre-flight did not report the Ghostty cask receipt"

if grep -Eq '^install --cask' "$BREW_CALLS"; then
    fail "an already-installed macOS cask was reinstalled or upgraded"
fi

echo "OK"
