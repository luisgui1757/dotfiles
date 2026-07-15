#!/usr/bin/env bash
# Real owner-host lifecycle smoke for supported native Linux distributions.
#
# This intentionally mutates the current user's dotfiles and package layer. Run
# it only from the owner's terminal after reviewing and committing the checkout.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd -P)"
CACHE_ROOT="$REPO_ROOT/tests/.cache"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="$CACHE_ROOT/linux-owner-lifecycle-$TIMESTAMP.log"
STATE_DIR="$CACHE_ROOT/linux-owner-lifecycle-state-$TIMESTAMP"
SUDO_KEEPALIVE_PID=""
PACKAGE_BACKEND=""

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
        echo "PASS: Linux owner-host lifecycle completed; log retained at $LOG_FILE"
    else
        echo "FAIL: Linux owner-host lifecycle exited $rc; inspect $LOG_FILE" >&2
    fi
    exit "$rc"
}

require_owner_host() {
    [[ "$(uname -s)" == "Linux" ]] || fail "this lifecycle is Linux-only"
    case "$(uname -m)" in
        x86_64|amd64|aarch64|arm64) ;;
        *) fail "unsupported Linux architecture: $(uname -m)" ;;
    esac
    [[ "$(id -u)" -ne 0 ]] || fail "run as the target user, not root"
    [[ -t 0 && -t 1 ]] || fail "run from an interactive owner terminal so sudo can prompt safely"
    command -v sudo >/dev/null 2>&1 || fail "sudo is required for native package installation"
    [[ -x "$REPO_ROOT/setup.sh" && -x "$REPO_ROOT/uninstall.sh" ]] ||
        fail "setup/uninstall entry points are missing"
    git -C "$REPO_ROOT" diff --quiet ||
        fail "tracked worktree changes exist; commit them before the destructive lifecycle"
    git -C "$REPO_ROOT" diff --cached --quiet ||
        fail "staged changes exist; commit them before the destructive lifecycle"
}

refresh_runtime_path() {
    local path
    for path in \
        "${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles/profile/bin" \
        "$HOME/.nix-profile/bin" \
        "/etc/profiles/per-user/$(id -un)/bin" \
        /nix/var/nix/profiles/default/bin \
        "$HOME/.local/bin" \
        /usr/local/bin; do
        [[ -d "$path" ]] || continue
        case ":$PATH:" in *":$path:"*) ;; *) PATH="$path:$PATH" ;; esac
    done
    export PATH
    command -v nix >/dev/null 2>&1 || fail "Nix must be installed before the Linux lifecycle starts"
    nix --version >/dev/null
    nix store info >/dev/null
}

detect_package_backend() {
    if command -v dpkg-query >/dev/null 2>&1; then
        PACKAGE_BACKEND="dpkg"
    elif command -v rpm >/dev/null 2>&1; then
        PACKAGE_BACKEND="rpm"
    elif command -v apk >/dev/null 2>&1; then
        PACKAGE_BACKEND="apk"
    elif command -v pacman >/dev/null 2>&1; then
        PACKAGE_BACKEND="pacman"
    else
        fail "no supported Linux package inventory backend is available"
    fi
}

write_package_inventory() {
    local output="$1"
    case "$PACKAGE_BACKEND" in
        dpkg) dpkg-query -W -f='${binary:Package}\n' ;;
        rpm) rpm -qa --qf '%{NAME}.%{ARCH}\n' ;;
        apk) apk info ;;
        pacman) pacman -Qq ;;
        *) fail "unknown package inventory backend: $PACKAGE_BACKEND" ;;
    esac | LC_ALL=C sort -u > "$output"
}

assert_no_removed_packages() {
    local removed
    write_package_inventory "$STATE_DIR/packages.after"
    removed="$(comm -23 "$STATE_DIR/packages.before" "$STATE_DIR/packages.after")"
    if [[ -n "$removed" ]]; then
        printf 'FAIL: lifecycle removed pre-existing Linux packages:\n%s\n' "$removed" >&2
        return 1
    fi
    echo "PASS: all pre-existing Linux packages remain installed"
}

assert_installed_state() {
    local phase="$1"
    refresh_runtime_path
    command -v home-manager >/dev/null 2>&1 ||
        fail "$phase did not leave the Home Manager CLI installed"
    home-manager generations >/dev/null
    "$REPO_ROOT/tests/greenfield/validate.sh" --config-only
    echo "PASS: $phase left the declared Home Manager/config state active"
}

assert_uninstalled_state() {
    local output
    output="$("$REPO_ROOT/uninstall.sh" --dry-run --all 2>&1)"
    printf '%s\n' "$output"
    printf '%s\n' "$output" | grep -F 'uninstall: nothing to remove' >/dev/null ||
        fail "second uninstall was not an idempotent no-op"
    refresh_runtime_path
    command -v home-manager >/dev/null 2>&1 ||
        fail "config uninstall unexpectedly removed the Home Manager package layer"
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
    detect_package_backend

    echo "This performs the real Linux owner-host install/update/uninstall/reinstall lifecycle."
    echo "Linux may prompt once for sudo; input stays attached to this terminal."
    sudo -v

    while sudo -n -v >/dev/null 2>&1; do sleep 45; done &
    SUDO_KEEPALIVE_PID=$!
    trap cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM

    exec > >(tee "$LOG_FILE") 2>&1
    echo "Linux owner-host lifecycle: repo=$REPO_ROOT head=$(git -C "$REPO_ROOT" rev-parse HEAD)"
    echo "uninstall scope: chezmoi config + clean zsh externals; Nix/native packages stay installed"

    write_package_inventory "$STATE_DIR/packages.before"

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
    assert_no_removed_packages
}

if [[ -n "${DOTFILES_LINUX_OWNER_LIFECYCLE_SOURCE_ONLY:-}" ]]; then
    return 0
fi

main "$@"
