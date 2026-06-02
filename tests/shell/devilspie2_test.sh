#!/usr/bin/env bash
# Regression guard for the GNOME/X11 Ghostty-maximize setup (devilspie2).
# The runtime behavior (Mutter ignoring `maximize = true`) can't be tested in
# CI, so this pins the pieces: the rule file exists and matches Ghostty's
# WM_CLASS + calls maximize(), and install-deps wires the opt-in setup.
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
fail() { echo "FAIL: $1"; exit 1; }

rule="$REPO_ROOT/linux/devilspie2/ghostty-maximize.lua"
[[ -f "$rule" ]] || fail "devilspie2 rule file missing: $rule"
grep -q "com.mitchellh.ghostty" "$rule" || fail "rule does not match Ghostty's WM_CLASS (com.mitchellh.ghostty)"
grep -q "maximize()" "$rule" || fail "rule does not call maximize()"

# install-deps must define AND call the opt-in setup.
grep -q '^setup_ghostty_maximize()' "$REPO_ROOT/install-deps.sh" || fail "setup_ghostty_maximize() not defined in install-deps.sh"
grep -qE '^setup_ghostty_maximize($|[[:space:]])' "$REPO_ROOT/install-deps.sh" \
    || fail "setup_ghostty_maximize is never called in install-deps.sh"

INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/devilspie2-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

have() {
    case "$1" in
        ghostty) return 0 ;;
        devilspie2) return 1 ;;
        apt-get) return 0 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

pm_install() {
    fail "setup_ghostty_maximize used PM=$PM instead of native Linux package manager"
}

native_linux_pm_install() {
    printf '%s\n' "$*" > "$TMP_ROOT/native-pm.log"
}

YES_ALL=1
DRY_RUN=0
PM=brew
XDG_CONFIG_HOME="$TMP_ROOT/config"
setup_ghostty_maximize >/dev/null
grep -Fx 'apt devilspie2' "$TMP_ROOT/native-pm.log" >/dev/null \
    || fail "devilspie2 was not installed through native apt helper"

echo "OK"
