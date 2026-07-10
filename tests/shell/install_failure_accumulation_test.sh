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

# A direct-artifact failure must not trip `set -e`: later independent work runs,
# the failure is recorded exactly once, and the final summary remains nonzero.
(
    DRY_RUN=0
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""
    sentinel="$(mktemp)"
    rm -f "$sentinel"
    trap 'rm -f "$sentinel"' EXIT

    fail_early_artifact() { return 47; }
    run_later_install() { printf '%s\n' reached > "$sentinel"; }

    run_install_step wezterm direct pinned-deb fail_early_artifact
    run_install_step later test sentinel run_later_install

    [[ -f "$sentinel" ]] \
        || { echo "FAIL: early artifact failure prevented the later sentinel install"; exit 1; }
    [[ "$INSTALL_FAILURES_COUNT" -eq 1 ]] \
        || { echo "FAIL: wrapper must record the early failure exactly once"; exit 1; }
    [[ "$(printf '%s' "$INSTALL_FAILURES_DETAIL" | grep -c 'wezterm via direct')" -eq 1 ]] \
        || { echo "FAIL: wrapper duplicated or lost the early failure detail"; exit 1; }

    set +e
    summary="$( ( exit_if_install_failures ) 2>&1 )"
    summary_rc=$?
    set -e
    [[ "$summary_rc" -ne 0 && "$summary" == *"wezterm via direct"* ]] \
        || { echo "FAIL: consolidated summary did not report the injected failure"; exit 1; }
)

# Every bare main-flow path called out by UGR-004 must pass through the same
# wrapper; focused function tests cover their path-specific failure details.
for snippet in \
    'run_install_step nvim direct' \
    'run_install_step chezmoi direct' \
    'run_install_step lazygit direct' \
    'run_install_step starship direct' \
    'run_install_step "tmux plugins" git' \
    'run_install_step ghostty direct' \
    'run_install_step wezterm direct' \
    'run_install_step herdr direct' \
    'run_install_step latex2text direct' \
    'run_install_step pi npm' \
    'run_install_step tree-sitter direct'; do
    grep -F "$snippet" "$REPO_ROOT/install-deps.sh" >/dev/null \
        || { echo "FAIL: main flow bypasses run_install_step for: $snippet"; exit 1; }
done

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

(
    PM=apt
    DRY_RUN=0
    YES_ALL=1
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""

    have() { return 1; }
    is_wsl() { return 1; }
    is_ubuntu() { return 0; }
    require_downloader() { return 0; }
    resolve_ghostty_deb_asset() {
        GHOSTTY_DEB_ASSET="ghostty-test.deb"
        GHOSTTY_DEB_SHA256="test-sha"
        GHOSTTY_DEB_ARCH=amd64
        GHOSTTY_DEB_URL="https://example.invalid/ghostty-test.deb"
    }
    install_verified_ghostty_deb() { return 43; }

    out_file="$(mktemp)"
    install_ghostty_linux >"$out_file" 2>&1
    output="$(cat "$out_file")"

    [[ "$INSTALL_FAILURES_COUNT" -eq 1 ]] \
        || { echo "FAIL: Ubuntu ghostty package failure must be recorded"; exit 1; }
    [[ "$INSTALL_FAILURES_DETAIL" == *"ghostty via apt (mkasberg/ghostty-ubuntu@$GHOSTTY_UBUNTU_VERSION) exit=43"* ]] \
        || { echo "FAIL: Ubuntu ghostty failure detail was not precise: $INSTALL_FAILURES_DETAIL"; exit 1; }
    [[ "$output" == *"verified Debian-family Ghostty package install failed"* ]] \
        || { echo "FAIL: Ubuntu ghostty failure output missing: $output"; exit 1; }
)

echo "OK"
