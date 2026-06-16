#!/usr/bin/env bash
# Manual WSL end-to-end check. Run from inside WSL after Windows host setup:
#   ./tests/wsl/e2e.sh
#   ./tests/wsl/e2e.sh --run-setup
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
RUN_SETUP=0

for arg in "$@"; do
    case "$arg" in
        --run-setup) RUN_SETUP=1 ;;
        -h|--help)
            sed -n '2,8p' "$0"
            exit 0
            ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done

fail() { echo "FAIL: $*" >&2; exit 1; }
note() { echo "OK: $*"; }
warn() { echo "WARN: $*" >&2; }

assert_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is not on PATH"
    note "$1 on PATH"
}

assert_link() {
    local path="$1" expected="$2" actual
    [[ -L "$path" ]] || fail "$path is not a symlink"
    actual="$(readlink "$path")"
    [[ "$actual" == "$expected" ]] || fail "$path points to $actual, expected $expected"
    note "$path -> $expected"
}

if ! grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null; then
    fail "this check must run inside WSL"
fi

cd "$REPO_ROOT"

if [[ "$RUN_SETUP" -eq 1 ]]; then
    ./setup.sh --all
fi

for dir in /usr/local/bin "$HOME/.local/bin"; do
    if [[ -d "$dir" && ":$PATH:" != *":$dir:"* ]]; then
        PATH="$dir:$PATH"
    fi
done
export PATH

for cmd in git nvim tmux zsh rg fd fzf starship lazygit; do
    assert_cmd "$cmd"
done

assert_link "$HOME/.config/nvim" "$REPO_ROOT/nvim"
assert_link "$HOME/.config/starship.toml" "$REPO_ROOT/starship/starship.toml"
assert_link "$HOME/.tmux.conf" "$REPO_ROOT/tmux/tmux.conf"
assert_link "$HOME/.zshenv" "$REPO_ROOT/shells/zshenv"
assert_link "$HOME/.zshrc" "$REPO_ROOT/shells/zshrc"
assert_link "$HOME/.config/lazygit/config.yml" "$REPO_ROOT/lazygit/config.yml"

if [[ -L "$HOME/.config/ghostty/config" ]]; then
    warn "WSL Ghostty config is linked; this should only happen after --experimental-wsl-gui"
else
    note "WSL Ghostty config is not linked by default"
fi

plugin_root="${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins"
[[ -r "$plugin_root/fzf-tab/fzf-tab.plugin.zsh" ]] || fail "fzf-tab plugin file missing"
[[ -r "$plugin_root/zsh-autosuggestions/zsh-autosuggestions.zsh" ]] || fail "zsh-autosuggestions plugin file missing"
zsh -i -c "source '$HOME/.zshrc'" >/dev/null 2>&1 || fail "zshrc failed to source interactively"
note "zsh plugins installed and zshrc sources cleanly"

tmux -L dotfiles-wsl-e2e -f "$HOME/.tmux.conf" new-session -d -s dotfiles-wsl-e2e 'sleep 5'
tmux -L dotfiles-wsl-e2e kill-session -t dotfiles-wsl-e2e
note "tmux starts with repo config"

lazygit --version >/dev/null || fail "lazygit --version failed"
nvim --version | head -n 1

if command -v powershell.exe >/dev/null 2>&1; then
    # shellcheck disable=SC2016 # PowerShell expands $env:... inside this script block.
    powershell.exe -NoProfile -ExecutionPolicy Bypass -Command '
        $ErrorActionPreference = "Stop"
        if (-not (Get-Command win32yank -ErrorAction SilentlyContinue)) { exit 10 }
        $settings = @(
            "$env:LOCALAPPDATA\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json",
            "$env:LOCALAPPDATA\Packages\Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe\LocalState\settings.json"
        ) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
        if (-not $settings) { exit 11 }
        $raw = Get-Content -Raw -LiteralPath $settings
        if ($raw -notmatch "Hack Nerd Font") { exit 12 }
    ' || fail "Windows host is missing win32yank or Windows Terminal Hack Nerd Font config"
    note "Windows host has win32yank and Windows Terminal Hack Nerd Font config"
else
    fail "powershell.exe is not reachable from WSL PATH"
fi

if command -v win32yank.exe >/dev/null 2>&1; then
    marker="dotfiles-wsl-e2e-$RANDOM"
    printf '%s' "$marker" | win32yank.exe -i --crlf
    got="$(win32yank.exe -o --lf 2>/dev/null | tr -d '\r')"
    [[ "$got" == "$marker" ]] || fail "win32yank clipboard round-trip failed"
    note "WSL clipboard round-trip works"
else
    fail "win32yank.exe is not reachable from WSL PATH"
fi

echo "OK: WSL e2e passed"
