#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# Model a caller that explicitly enabled Homebrew's ask mode. setup owns the
# accepted mutation, so its child installer must clear that override and select
# Homebrew's supported noninteractive mode before any brew command can run.
export HOMEBREW_ASK=1
unset HOMEBREW_NO_ASK
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh" --all

[[ -z "${HOMEBREW_ASK+x}" ]] || fail "install-deps preserved inherited Homebrew ask mode"
[[ "${HOMEBREW_NO_ASK:-}" == "1" ]] || fail "install-deps did not enable Homebrew no-ask mode"

BREW_CALLS=""
BREW_ENV_VIOLATIONS=""
brew() {
    if [[ -n "${HOMEBREW_ASK+x}" || "${HOMEBREW_NO_ASK:-}" != "1" ]]; then
        BREW_ENV_VIOLATIONS="${BREW_ENV_VIOLATIONS}${*}"$'\n'
    fi
    case "${1:-}" in
        list) return 1 ;;
        install)
            BREW_CALLS="${BREW_CALLS}${*}"$'\n'
            return 0
            ;;
        *) return 0 ;;
    esac
}
have() { return 1; }

YES_ALL=1
DRY_RUN=0
install_ghostty_macos

[[ -z "$BREW_ENV_VIOLATIONS" ]] || fail "a Homebrew child command escaped the no-ask boundary"
[[ "$BREW_CALLS" == *"install --cask ghostty"* ]] || fail "fixture did not exercise a real Homebrew install path"

echo "OK"
