#!/usr/bin/env bash
# Regression: ensure_python_pip_venv installs python3-venv + python3-pip on apt.
# Debian/Ubuntu ship python3 without ensurepip/venv, so Mason's PyPI tools
# (clang-format / ruff / gersemi) fail until those packages are present.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
INSTALL_LOG="$TMP_ROOT/install.log"

# python3 stub: `-c 'import ensurepip, venv'` exits per PYTHON_VENV_OK so we can
# simulate a Debian python3 with and without the venv/pip packages.
python3() {
    if [[ "${1:-}" == "-c" ]]; then
        [[ "${PYTHON_VENV_OK:-1}" -eq 1 ]] && return 0 || return 1
    fi
    return 0
}
native_linux_pm() { printf '%s\n' "${NATIVE_PM:-apt}"; }
native_linux_pm_install() { printf '%s\n' "$*" >> "$INSTALL_LOG"; return 0; }

PM=apt
YES_ALL=1
DRY_RUN=0

# Case 1: venv + pip already present -> no install attempt.
: > "$INSTALL_LOG"
PYTHON_VENV_OK=1 ensure_python_pip_venv >/dev/null
[[ -s "$INSTALL_LOG" ]] && fail "installed venv/pip even though already present"

# Case 2: venv/pip missing on apt -> installs both packages.
: > "$INSTALL_LOG"
PYTHON_VENV_OK=0 NATIVE_PM=apt ensure_python_pip_venv >/dev/null
grep -q "apt python3-venv python3-pip" "$INSTALL_LOG" \
    || fail "did not install python3-venv python3-pip on apt (log: $(cat "$INSTALL_LOG"))"

# Case 3: dry-run on apt -> previews, mutates nothing.
: > "$INSTALL_LOG"
out="$(PYTHON_VENV_OK=0 NATIVE_PM=apt DRY_RUN=1 ensure_python_pip_venv)"
[[ -s "$INSTALL_LOG" ]] && fail "dry-run still installed packages"
[[ "$out" == *"would:"* ]] || fail "dry-run did not print a would line"

echo "OK"
