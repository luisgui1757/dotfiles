#!/usr/bin/env bash
# Real owner-host lifecycle smoke for the supported Apple Silicon macOS path.
#
# This intentionally mutates the current user's dotfiles and package layer. Run
# it only from the owner's terminal after reviewing and committing the checkout.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd -P)"
CACHE_ROOT="$REPO_ROOT/tests/.cache"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="$CACHE_ROOT/macos-owner-lifecycle-$TIMESTAMP.log"
STATE_DIR="$CACHE_ROOT/macos-owner-lifecycle-state-$TIMESTAMP"
SUDO_KEEPALIVE_PID=""

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

cleanup() {
    local rc=$?
    trap - EXIT INT TERM
    if [[ -n "$SUDO_KEEPALIVE_PID" ]]; then
        kill "$SUDO_KEEPALIVE_PID" >/dev/null 2>&1 || true
        wait "$SUDO_KEEPALIVE_PID" >/dev/null 2>&1 || true
    fi
    sudo -k >/dev/null 2>&1 || true
    rm -rf "$STATE_DIR"
    if [[ "$rc" -eq 0 ]]; then
        echo "PASS: owner-host lifecycle completed; log retained at $LOG_FILE"
    else
        echo "FAIL: owner-host lifecycle exited $rc; inspect $LOG_FILE" >&2
    fi
    exit "$rc"
}

require_owner_host() {
    [[ "$(uname -s)" == "Darwin" ]] || fail "this lifecycle is macOS-only"
    [[ "$(uname -m)" == "arm64" ]] || fail "supported macOS lifecycle requires arm64"
    [[ "$(id -u)" -ne 0 ]] || fail "run as the target user, not root"
    [[ -t 0 && -t 1 ]] || fail "run from an interactive owner terminal so sudo can prompt safely"
    [[ -x "$REPO_ROOT/setup.sh" && -x "$REPO_ROOT/uninstall.sh" ]] ||
        fail "setup/uninstall entry points are missing"
    git -C "$REPO_ROOT" diff --quiet ||
        fail "tracked worktree changes exist; commit them before the destructive lifecycle"
    git -C "$REPO_ROOT" diff --cached --quiet ||
        fail "staged changes exist; commit them before the destructive lifecycle"
}

refresh_runtime_path() {
    local brew_bin
    for brew_bin in \
        "$(command -v brew 2>/dev/null || true)" \
        /opt/homebrew/bin/brew; do
        if [[ -n "$brew_bin" && -x "$brew_bin" ]]; then
            eval "$("$brew_bin" shellenv)"
            BREW="$brew_bin"
            export BREW
            break
        fi
    done
    [[ -n "${BREW:-}" ]] || fail "Homebrew is required on the owner host"

    PATH="/run/current-system/sw/bin:$HOME/.nix-profile/bin:/nix/var/nix/profiles/default/bin:$PATH"
    export PATH
}

snapshot_homebrew_state() {
    "$BREW" list --formula | LC_ALL=C sort -u > "$STATE_DIR/formulae.before"
    "$BREW" list --cask | LC_ALL=C sort -u > "$STATE_DIR/casks.before"
    "$BREW" tap | LC_ALL=C awk '
        $0 != "homebrew/core" &&
        $0 != "homebrew/cask" &&
        $0 != "nikitabobko/tap" &&
        $0 !~ /\.dotfiles-/ { print }
    ' | sort -u > "$STATE_DIR/unrelated-taps.before"
}

assert_no_removed_rows() {
    local kind="$1" before="$2" after="$3" removed
    removed="$(comm -23 "$before" "$after")"
    if [[ -n "$removed" ]]; then
        printf 'FAIL: lifecycle removed pre-existing Homebrew %s:\n%s\n' "$kind" "$removed" >&2
        return 1
    fi
    echo "PASS: all pre-existing Homebrew $kind remain installed"
}

assert_homebrew_preserved() {
    "$BREW" list --formula | LC_ALL=C sort -u > "$STATE_DIR/formulae.after"
    "$BREW" list --cask | LC_ALL=C sort -u > "$STATE_DIR/casks.after"
    "$BREW" tap | LC_ALL=C sort -u > "$STATE_DIR/taps.after"

    assert_no_removed_rows formulae "$STATE_DIR/formulae.before" "$STATE_DIR/formulae.after"
    assert_no_removed_rows casks "$STATE_DIR/casks.before" "$STATE_DIR/casks.after"
    assert_no_removed_rows unrelated-taps "$STATE_DIR/unrelated-taps.before" "$STATE_DIR/taps.after"
}

assert_no_scanned_tap_artifacts() {
    local taps_dir artifact tap_output
    taps_dir="$($BREW --repository)/Library/Taps"
    artifact="$(find "$taps_dir" -mindepth 2 -maxdepth 2 \
        \( -name '*.dotfiles-pre-user-taps-*' -o -name '*.dotfiles-failed-*' \) \
        -print -quit 2>/dev/null || true)"
    [[ -z "$artifact" ]] || fail "setup recovery artifact is still inside Homebrew's scanned Taps: $artifact"

    tap_output="$($BREW tap)"
    if printf '%s\n' "$tap_output" | grep -E '\.dotfiles-(pre-user-taps|failed)' >/dev/null; then
        fail "Homebrew still enumerates a setup recovery artifact as a live tap"
    fi
    echo "PASS: no setup transaction/recovery artifact is visible as a Homebrew tap"
}

assert_tap_ownership() {
    local taps_dir rel path owner
    taps_dir="$($BREW --repository)/Library/Taps"
    for rel in \
        homebrew/homebrew-core \
        homebrew/homebrew-cask \
        nikitabobko/homebrew-tap; do
        path="$taps_dir/$rel"
        [[ -e "$path" ]] || continue
        owner="$(stat -f '%Su' "$path")"
        [[ "$owner" == "$(id -un)" ]] || fail "Homebrew tap is not target-user owned: $path ($owner)"
    done
    [[ -d "$taps_dir/nikitabobko/homebrew-tap" ]] || fail "nikitabobko/tap checkout is missing"
    echo "PASS: installed Homebrew tap checkouts are target-user owned"
}

assert_installed_state() {
    local phase="$1"
    [[ -x /run/current-system/sw/bin/darwin-rebuild ]] ||
        fail "$phase did not leave the active nix-darwin rebuild command installed"
    [[ -L /etc/bashrc && "$(readlink /etc/bashrc)" == "/etc/static/bashrc" ]] ||
        fail "$phase did not leave /etc/bashrc managed by nix-darwin"
    [[ -L /etc/zshrc && "$(readlink /etc/zshrc)" == "/etc/static/zshrc" ]] ||
        fail "$phase did not leave /etc/zshrc managed by nix-darwin"

    "$BREW" list --cask wezterm >/dev/null
    "$BREW" list --cask aerospace >/dev/null
    "$BREW" list --formula herdr >/dev/null
    assert_no_scanned_tap_artifacts
    assert_tap_ownership
    "$REPO_ROOT/tests/greenfield/validate.sh" --config-only
    echo "PASS: $phase left the declared package/config state active"
}

assert_uninstalled_state() {
    local output
    output="$("$REPO_ROOT/uninstall.sh" --dry-run --all 2>&1)"
    printf '%s\n' "$output"
    printf '%s\n' "$output" | grep -F 'uninstall: nothing to remove' >/dev/null ||
        fail "second uninstall was not an idempotent no-op"
    [[ -x /run/current-system/sw/bin/darwin-rebuild ]] ||
        fail "config uninstall unexpectedly removed the Nix package layer"
    assert_no_scanned_tap_artifacts
    echo "PASS: uninstall removed the managed config layer and preserved packages"
}

run_phase() {
    local label="$1" started elapsed
    shift
    started="$(date +%s)"
    echo
    echo "================================================================"
    echo "==  $label"
    echo "================================================================"
    "$@"
    elapsed=$(( $(date +%s) - started ))
    echo "PASS: $label (${elapsed}s)"
}

main() {
    require_owner_host
    mkdir -p "$CACHE_ROOT" "$STATE_DIR"
    refresh_runtime_path

    echo "This performs the real owner-host install/update/uninstall/reinstall lifecycle."
    echo "macOS may prompt once for sudo; input stays attached to this terminal."
    sudo -v

    while sudo -n -v >/dev/null 2>&1; do sleep 45; done &
    SUDO_KEEPALIVE_PID=$!
    trap cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM

    exec > >(tee "$LOG_FILE") 2>&1
    echo "owner-host lifecycle: repo=$REPO_ROOT head=$(git -C "$REPO_ROOT" rev-parse HEAD)"
    echo "uninstall scope: chezmoi config + clean zsh externals; Nix/Homebrew packages stay installed"

    snapshot_homebrew_state

    run_phase "1/5 install" "$REPO_ROOT/setup.sh" --all
    assert_installed_state "first install"

    run_phase "2/5 update" "$REPO_ROOT/setup.sh" --update
    assert_installed_state "first update"

    run_phase "3/5 uninstall config layer" "$REPO_ROOT/uninstall.sh" --all
    assert_uninstalled_state

    run_phase "4/5 reinstall" "$REPO_ROOT/setup.sh" --all
    assert_installed_state "reinstall"

    run_phase "5/5 final update" "$REPO_ROOT/setup.sh" --update
    assert_installed_state "final update"

    run_phase "final full greenfield validation" "$REPO_ROOT/tests/greenfield/validate.sh"
    assert_homebrew_preserved
}

main "$@"
