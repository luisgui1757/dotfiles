#!/usr/bin/env bash
# Regression: ensure_npm installs npm on apt. Debian/Ubuntu ship `nodejs`
# without npm, so Mason's npm tools (pyright / prettier / LSPs) fail until npm
# is present.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
INSTALL_LOG="$TMP_ROOT/install.log"

# node is always present (its package installed); npm presence is controlled by
# NPM_PRESENT so we can simulate a Debian nodejs without npm.
command() {
    if [[ "${1:-}" == "-v" ]]; then
        case "${2:-}" in
            node) return 0 ;;
            npm) [[ "${NPM_PRESENT:-1}" -eq 1 ]] && return 0 || return 1 ;;
            *) builtin command "$@" ;;
        esac
    fi
    builtin command "$@"
}
native_linux_pm() { printf '%s\n' "${NATIVE_PM:-apt}"; }
native_linux_pm_install() { printf '%s\n' "$*" >> "$INSTALL_LOG"; return 0; }

PM=apt
YES_ALL=1
DRY_RUN=0

# Case 1: npm already present -> no install attempt.
: > "$INSTALL_LOG"
NPM_PRESENT=1 ensure_npm >/dev/null
[[ -s "$INSTALL_LOG" ]] && fail "installed npm even though already present"

# Case 2: npm missing on apt -> installs the npm package.
: > "$INSTALL_LOG"
NPM_PRESENT=0 NATIVE_PM=apt ensure_npm >/dev/null
grep -q "apt npm" "$INSTALL_LOG" \
    || fail "did not install npm on apt (log: $(cat "$INSTALL_LOG"))"

# Case 3: dry-run on apt -> previews, mutates nothing.
: > "$INSTALL_LOG"
out="$(NPM_PRESENT=0 NATIVE_PM=apt DRY_RUN=1 ensure_npm)"
[[ -s "$INSTALL_LOG" ]] && fail "dry-run still installed npm"
[[ "$out" == *"would:"* ]] || fail "dry-run did not print a would line"

echo "OK"
