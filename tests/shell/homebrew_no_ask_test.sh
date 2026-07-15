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

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
export HOMEBREW_BOOTSTRAP_TEST_ROOT="$WORK"
curl() {
    local output=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o) output="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    [[ -n "$output" ]] || fail "Homebrew bootstrap download omitted -o"
    cat > "$output" <<'INSTALLER'
#!/usr/bin/env bash
[[ "${NONINTERACTIVE:-}" == "1" ]] || exit 91
printf '%s\n' ok > "$HOMEBREW_BOOTSTRAP_TEST_ROOT/noninteractive"
INSTALLER
}
verify_sha256() { return 0; }
require_downloader() { return 0; }
enable_homebrew_for_current_shell() { return 0; }
persist_homebrew_shellenv() { return 0; }

maybe_install_brew || fail "pinned Homebrew bootstrap did not run noninteractively"
[[ "$(< "$WORK/noninteractive")" == "ok" ]] ||
    fail "pinned Homebrew bootstrap did not receive NONINTERACTIVE=1"

echo "OK"
