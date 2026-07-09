#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329,SC2317
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

(
    PM=apt
    DRY_RUN=0
    YES_ALL=1
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""

    have_any() { return 1; }
    pm_install() { return 42; }

    install rg "test failure collection"

    [[ "$INSTALL_FAILURES_COUNT" -eq 1 ]] \
        || { echo "FAIL: generic install failure must be recorded"; exit 1; }
    [[ "$INSTALL_FAILURES_DETAIL" == *"rg via apt (ripgrep) exit=42"* ]] \
        || { echo "FAIL: generic install failure detail must include tool, manager, pkg, and exit"; exit 1; }

    set +e
    out="$( ( exit_if_install_failures ) 2>&1 )"
    rc=$?
    set -e
    [[ "$rc" -eq 1 ]] \
        || { echo "FAIL: recorded install failures must make final exit nonzero"; exit 1; }
    [[ "$out" == *"accepted install path(s) failed"* ]] \
        || { echo "FAIL: final failure summary must explain the blocked install"; exit 1; }
)

(
    DRY_RUN=1
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""
    record_install_failure "rg" apt "ripgrep" 42
    [[ "$INSTALL_FAILURES_COUNT" -eq 0 ]] \
        || { echo "FAIL: dry-run must not record previewed failures"; exit 1; }
)

(
    PM=apt
    DRY_RUN=0
    YES_ALL=0
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""

    have_any() { return 1; }
    ask() { return 1; }
    pm_install() { echo "FAIL: pm_install must not run for skipped installs"; exit 1; }

    install rg "explicit skip"
    [[ "$INSTALL_FAILURES_COUNT" -eq 0 ]] \
        || { echo "FAIL: explicit skips must remain non-failures"; exit 1; }
)

(
    DRY_RUN=0
    YES_ALL=1
    HOME="$(mktemp -d)"
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""
    trap 'rm -rf "$HOME"' EXIT

    zsh_plugin_ok() { return 1; }
    install_zsh_plugin_repo() { return 1; }

    install_zsh_plugins >/dev/null 2>&1
    [[ "$INSTALL_FAILURES_COUNT" -eq 1 ]] \
        || { echo "FAIL: zsh plugin FAIL-then-continue path must record a failure"; exit 1; }
    [[ "$INSTALL_FAILURES_DETAIL" == *"zsh plugins via git"* ]] \
        || { echo "FAIL: zsh plugin failure detail must name the git path"; exit 1; }
)

echo "OK"
