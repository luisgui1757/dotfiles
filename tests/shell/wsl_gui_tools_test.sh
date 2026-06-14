#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

is_wsl() { return 0; }

YES_ALL=1
DRY_RUN=1
PM=brew
CODE_PRESENT=0
UBUNTU=1

is_ubuntu() {
    [[ "$UBUNTU" -eq 1 ]]
}

have() {
    case "$1" in
        ghostty) return 1 ;;
        code) [[ "$CODE_PRESENT" -eq 1 ]] ;;
        snap) return 0 ;;
        flatpak) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

code() {
    echo "FAIL: code should not be invoked in dry-run mode" >&2
    return 1
}

brew() {
    echo "FAIL: brew should not be invoked in dry-run mode" >&2
    return 1
}

unset DISPLAY WAYLAND_DISPLAY
ghostty_out="$(install_ghostty_linux)"
[[ "$ghostty_out" == *"skipped"* ]]
[[ "$ghostty_out" == *"WSL uses Windows Terminal by default"* ]]
[[ "$ghostty_out" == *"--experimental-wsl-gui"* ]]

export DISPLAY=:0
ghostty_out="$(install_ghostty_linux)"
[[ "$ghostty_out" == *"WSL uses Windows Terminal by default"* ]]

EXPERIMENTAL_WSL_GUI=1
ghostty_out="$(install_ghostty_linux)"
[[ "$ghostty_out" == *"Homebrew formula is macOS-only on Linux"* ]]
# Installer must be pinned (not HEAD) and SHA-256 verified before running.
[[ "$ghostty_out" == *"ghostty-ubuntu/${GHOSTTY_UBUNTU_VERSION}/install.sh"* ]]
[[ "$ghostty_out" == *"verify sha256 ${GHOSTTY_UBUNTU_INSTALL_SHA256}"* ]]
[[ "$ghostty_out" != *"/HEAD/install.sh"* ]]

UBUNTU=0
ghostty_out="$(install_ghostty_linux)"
[[ "$ghostty_out" == *"sudo snap install ghostty --classic"* ]]

vscode_out="$(install_vscode)"
[[ "$vscode_out" == *"sudo snap install code --classic"* ]]

CODE_PRESENT=1
theme_out="$(configure_vscode_rose_pine)"
[[ "$theme_out" == *"code --install-extension mvllow.rose-pine"* ]]

grep -F 'experimentalWslGui = {{ get . "experimentalWslGui" | default false }}' \
    "$REPO_ROOT/home/.chezmoi.toml.tmpl" >/dev/null
grep -F '{{- else if and .isWsl (not (get . "experimentalWslGui" | default false)) }}' \
    "$REPO_ROOT/home/.chezmoiignore" >/dev/null
grep -Fx '.config/ghostty' "$REPO_ROOT/home/.chezmoiignore" >/dev/null

echo "OK"
