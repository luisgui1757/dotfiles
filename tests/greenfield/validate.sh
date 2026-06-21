#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
CONFIG_ONLY=0

usage() {
    cat <<'EOF'
validate.sh -- verify a greenfield POSIX dotfiles install.

Usage:
  tests/greenfield/validate.sh [--repo <path>] [--home <path>] [--config-only]

Options:
  --repo <path>     dotfiles repo root. Defaults to this script's repo.
  --home <path>     HOME to validate. Defaults to the current HOME.
  --config-only     only validate chezmoi-managed config paths and chezmoi verify.
                    Tool, zsh external, Lazy, and Mason checks are skipped.
EOF
}

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --repo)
            [[ "$#" -ge 2 ]] || { echo "FAIL: --repo needs a path" >&2; exit 2; }
            REPO_ROOT="$2"
            shift 2
            ;;
        --home)
            [[ "$#" -ge 2 ]] || { echo "FAIL: --home needs a path" >&2; exit 2; }
            HOME="$2"
            export HOME
            shift 2
            ;;
        --config-only)
            CONFIG_ONLY=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "FAIL: unknown arg: $1" >&2
            exit 2
            ;;
    esac
done

REPO_ROOT="$(cd "$REPO_ROOT" && pwd -P)"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

pass_check() {
    printf 'PASS: %s\n' "$*"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail_check() {
    printf 'FAIL: %s\n' "$*" >&2
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

skip_check() {
    printf 'SKIP: %s\n' "$*"
    SKIP_COUNT=$((SKIP_COUNT + 1))
}

finish() {
    printf 'SUMMARY: %s passed, %s skipped, %s failed\n' "$PASS_COUNT" "$SKIP_COUNT" "$FAIL_COUNT"
    if [[ "$FAIL_COUNT" -ne 0 ]]; then
        exit 1
    fi
}

refresh_runtime_path() {
    local brew_bin dir

    for brew_bin in "$(command -v brew 2>/dev/null || true)" \
        /opt/homebrew/bin/brew \
        /usr/local/bin/brew \
        "$HOME/.linuxbrew/bin/brew" \
        /home/linuxbrew/.linuxbrew/bin/brew; do
        if [[ -n "$brew_bin" && -x "$brew_bin" ]]; then
            eval "$("$brew_bin" shellenv)"
            break
        fi
    done

    for dir in /usr/local/bin "$HOME/.local/bin"; do
        if [[ -d "$dir" && ":$PATH:" != *":$dir:"* ]]; then
            PATH="$dir:$PATH"
        fi
    done
    export PATH
    hash -r 2>/dev/null || true
}

require_cmd() {
    local cmd="$1"
    if command -v "$cmd" >/dev/null 2>&1; then
        pass_check "$cmd is on PATH"
    else
        fail_check "$cmd is not on PATH"
    fi
}

assert_file_content() {
    local path="$1" expected="$2"
    if [[ ! -e "$path" ]]; then
        fail_check "$path does not exist"
        return
    fi
    if [[ ! -f "$path" ]]; then
        fail_check "$path does not dereference to a file"
        return
    fi
    if cmp -s "$path" "$expected"; then
        pass_check "$path content matches $expected"
    else
        fail_check "$path content differs from $expected"
    fi
}

resolve_dir() {
    ( cd "$1" && pwd -P )
}

assert_dir_resolves() {
    local path="$1" expected="$2" actual expected_real
    if [[ ! -d "$path" ]]; then
        fail_check "$path is not a directory"
        return
    fi
    if ! actual="$(resolve_dir "$path")"; then
        fail_check "$path could not be resolved"
        return
    fi
    if ! expected_real="$(resolve_dir "$expected")"; then
        fail_check "$expected could not be resolved"
        return
    fi
    if [[ "$actual" == "$expected_real" ]]; then
        pass_check "$path resolves to $expected_real"
    else
        fail_check "$path resolves to $actual, expected $expected_real"
    fi
}

run_nvim_checked() {
    local label="$1" log rc
    shift
    if [[ "$CONFIG_ONLY" -eq 1 ]]; then
        skip_check "nvim $label skipped by --config-only"
        return
    fi
    if ! command -v nvim >/dev/null 2>&1; then
        fail_check "nvim $label cannot run because nvim is not on PATH"
        return
    fi
    log="${TMPDIR:-/tmp}/dotfiles-greenfield-nvim-${label}.$$"
    set +e
    nvim --headless "$@" >"$log" 2>&1
    rc=$?
    set -e
    if [[ "$rc" -eq 0 ]]; then
        pass_check "nvim $label exited 0"
    else
        fail_check "nvim $label exited $rc; log: $log"
        sed -n '1,120p' "$log" >&2 || true
    fi
}

assert_mason_tool() {
    local tool="$1" dir name
    shift
    if [[ "$CONFIG_ONLY" -eq 1 ]]; then
        skip_check "Mason $tool skipped by --config-only"
        return
    fi
    for dir in "${XDG_DATA_HOME:-$HOME/.local/share}/nvim/mason/bin" \
        "$HOME/.local/share/nvim/mason/bin" \
        "$HOME/Library/Application Support/nvim/mason/bin"; do
        for name in "$@"; do
            if [[ -f "$dir/$name" ]]; then
                pass_check "Mason installed $tool at $dir/$name"
                return
            fi
        done
    done
    fail_check "Mason did not install $tool into an expected mason/bin directory"
}

assert_nvim_version() {
    local nvim_line
    if [[ "$CONFIG_ONLY" -eq 1 ]]; then
        skip_check "nvim version skipped by --config-only"
        return
    fi
    if ! command -v nvim >/dev/null 2>&1; then
        fail_check "nvim version cannot be checked because nvim is not on PATH"
        return
    fi
    nvim_line="$(nvim --version | head -n 1)"
    case "$nvim_line" in
        "NVIM v0.12"* | "NVIM v1."*) pass_check "nvim version is supported: $nvim_line" ;;
        *) fail_check "nvim version is below 0.12: $nvim_line" ;;
    esac
}

assert_zsh_plugins() {
    local plugin_root
    if [[ "$CONFIG_ONLY" -eq 1 ]]; then
        skip_check "zsh external plugin check skipped by --config-only"
        return
    fi
    plugin_root="$HOME/.local/share/dotfiles/zsh-plugins"
    if [[ -r "$plugin_root/fzf-tab/fzf-tab.plugin.zsh" ]]; then
        pass_check "fzf-tab plugin file exists"
    else
        fail_check "fzf-tab plugin file missing"
    fi
    if [[ -r "$plugin_root/zsh-autosuggestions/zsh-autosuggestions.zsh" ]]; then
        pass_check "zsh-autosuggestions plugin file exists"
    else
        fail_check "zsh-autosuggestions plugin file missing"
    fi
}

assert_chezmoi_verify() {
    local output rc
    if ! command -v chezmoi >/dev/null 2>&1; then
        if [[ "$CONFIG_ONLY" -eq 1 ]]; then
            skip_check "chezmoi verify skipped because chezmoi is not on PATH"
        else
            fail_check "chezmoi verify cannot run because chezmoi is not on PATH"
        fi
        return
    fi
    set +e
    if [[ "$CONFIG_ONLY" -eq 1 ]]; then
        output="$(chezmoi --source "$REPO_ROOT/home" verify --exclude externals,scripts 2>&1)"
        rc=$?
    else
        output="$(chezmoi --source "$REPO_ROOT/home" verify 2>&1)"
        rc=$?
    fi
    set -e
    if [[ "$rc" -eq 0 ]]; then
        pass_check "chezmoi verify is clean"
    else
        fail_check "chezmoi verify exited $rc"
        printf '%s\n' "$output" >&2
    fi
}

assert_posix_managed_configs() {
    local host_os lazygit_path
    case "$(uname -s)" in
        Darwin) host_os="darwin" ;;
        Linux) host_os="linux" ;;
        *) fail_check "unsupported POSIX host OS: $(uname -s)"; return ;;
    esac

    assert_dir_resolves "$HOME/.config/nvim" "$REPO_ROOT/nvim"
    assert_file_content "$HOME/.config/nvim/init.lua" "$REPO_ROOT/nvim/init.lua"
    assert_file_content "$HOME/.config/starship.toml" "$REPO_ROOT/starship/starship.toml"
    assert_file_content "$HOME/.tmux.conf" "$REPO_ROOT/tmux/tmux.conf"
    assert_file_content "$HOME/.tmux.posix.conf" "$REPO_ROOT/tmux/tmux.posix.conf"
    assert_file_content "$HOME/.zshenv" "$REPO_ROOT/shells/zshenv"
    assert_file_content "$HOME/.zshrc" "$REPO_ROOT/shells/zshrc"

    if [[ "$host_os" == "darwin" ]]; then
        lazygit_path="$HOME/Library/Application Support/lazygit/config.yml"
    else
        lazygit_path="$HOME/.config/lazygit/config.yml"
    fi
    assert_file_content "$lazygit_path" "$REPO_ROOT/lazygit/config.yml"
}

main() {
    if [[ ! -d "$HOME" ]]; then
        fail_check "HOME does not exist: $HOME"
        finish
    fi

    refresh_runtime_path
    printf 'validate.sh: repo=%s home=%s mode=%s\n' \
        "$REPO_ROOT" "$HOME" "$([[ "$CONFIG_ONLY" -eq 1 ]] && printf config-only || printf full)"

    if [[ "$CONFIG_ONLY" -eq 1 ]]; then
        skip_check "full setup tool checks skipped by --config-only"
    else
        for cmd in git nvim rg fd fzf tmux zsh lazygit starship chezmoi tree-sitter cmake lsd; do
            require_cmd "$cmd"
        done
    fi

    assert_nvim_version
    assert_posix_managed_configs
    assert_zsh_plugins
    assert_chezmoi_verify
    run_nvim_checked lazy "+Lazy! restore" "+qa"
    DOTFILES_TREESITTER_SYNC_INSTALL=1 run_nvim_checked treesitter -u "$REPO_ROOT/nvim/init.lua" -c "lua require('lazy').load({ plugins = { 'nvim-treesitter' } })" +qa
    run_nvim_checked mason "+MasonToolsInstallSync" "+qa"
    assert_mason_tool "lua-language-server" lua-language-server lua-language-server.cmd lua-language-server.exe
    assert_mason_tool "stylua" stylua stylua.cmd stylua.exe

    finish
}

main
