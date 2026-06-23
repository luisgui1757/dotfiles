#!/usr/bin/env bash
set -euo pipefail

# Determinism: match parity_gate.sh. Chezmoi externals use a fixed
# ~/.local/share path; the fixture sets XDG_DATA_HOME to a hostile value below
# so path drift cannot pass by accident.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SRC="$REPO_ROOT/home"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

pass() {
    echo "PASS: $*"
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "required command '$1' is not on PATH"
}

deref() {
    local path="$1"
    if readlink -f "$path" >/dev/null 2>&1; then
        readlink -f "$path"
    else
        python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$path"
    fi
}

assert_symlink_into_repo() {
    local label="$1" path="$2" real
    [[ -L "$path" ]] || fail "$label is not a symlink: $path"
    real="$(deref "$path")"
    case "$real/" in
        "$REPO_ROOT"/*) pass "$label symlink resolves into repo" ;;
        *) fail "$label symlink resolves outside repo: $real" ;;
    esac
}

assert_absent_or_not_repo_symlink() {
    local label="$1" path="$2" real
    if [[ -L "$path" ]]; then
        real="$(deref "$path")"
        case "$real/" in
            "$REPO_ROOT"/*) fail "$label repo-owned symlink still exists: $path -> $real" ;;
        esac
    fi
}

assert_no_config_repo_symlinks() {
    local config_dir="$HOME/.config" link real
    [[ -d "$config_dir" ]] || { pass "\$HOME/.config absent after uninstall"; return; }
    while IFS= read -r link; do
        real="$(deref "$link")"
        case "$real/" in
            "$REPO_ROOT"/*) fail "repo-owned symlink remains under ~/.config: $link -> $real" ;;
        esac
    done < <(find "$config_dir" -type l -print)
    pass "\$HOME/.config has no repo-owned symlinks"
}

managed_rels_for_host() {
    case "$(uname -s)" in
        Darwin)
            cat <<'EOF'
.tmux.conf
.tmux.posix.conf
.config/lsd/colors.yaml
.config/lsd/config.yaml
.config/nvim
.config/starship.toml
.zshenv
.zshrc
Library/Application Support/com.mitchellh.ghostty/config
Library/Application Support/lazygit/config.yml
EOF
            ;;
        Linux)
            cat <<'EOF'
.tmux.conf
.tmux.posix.conf
.config/ghostty/config
.config/lazygit/config.yml
.config/lsd/colors.yaml
.config/lsd/config.yaml
.config/nvim
.config/starship.toml
.zshenv
.zshrc
EOF
            ;;
        *)
            fail "unsupported POSIX round-trip host OS: $(uname -s)"
            ;;
    esac
}

require_cmd chezmoi
require_cmd python3

HOME="$(mktemp -d)"
export HOME
export XDG_DATA_HOME="$HOME/xdg-data"
trap 'rm -rf "$HOME"' EXIT

preseed="user tmux config from before chezmoi"
mkdir -p "$HOME"
printf '%s\n' "$preseed" > "$HOME/.tmux.conf"
cp "$HOME/.tmux.conf" "$HOME/.tmux.conf.bak.20000101-000000"
pass "pre-seeded tmux config and bootstrap-style backup"

chezmoi --source "$SRC" init
chezmoi --source "$SRC" --no-tty --force apply
pass "chezmoi apply completed"

while IFS= read -r rel; do
    [[ -n "$rel" ]] || continue
    assert_symlink_into_repo "$rel" "$HOME/$rel"
done < <(managed_rels_for_host)

for plugin in fzf-tab zsh-autosuggestions; do
    [[ -d "$HOME/.local/share/dotfiles/zsh-plugins/$plugin" ]] || fail "missing external plugin: $plugin"
done
pass "zsh plugin externals exist after apply"

"$REPO_ROOT/uninstall.sh" --all
pass "uninstall.sh --all completed"

while IFS= read -r rel; do
    [[ -n "$rel" ]] || continue
    assert_absent_or_not_repo_symlink "$rel" "$HOME/$rel"
done < <(managed_rels_for_host)

[[ -f "$HOME/.tmux.conf" ]] || fail "pre-seeded ~/.tmux.conf was not restored"
[[ "$(cat "$HOME/.tmux.conf")" == "$preseed" ]] || fail "restored ~/.tmux.conf content mismatch"
pass "pre-seeded config restored from backup"

for plugin in fzf-tab zsh-autosuggestions; do
    [[ ! -e "$HOME/.local/share/dotfiles/zsh-plugins/$plugin" ]] || fail "external plugin still exists after uninstall: $plugin"
done
pass "zsh plugin externals removed"

assert_no_config_repo_symlinks

second_output="$("$REPO_ROOT/uninstall.sh" --all 2>&1)"
printf '%s\n' "$second_output" | grep -q 'nothing to remove' || fail "second uninstall did not report no-op"
[[ -f "$HOME/.tmux.conf" ]] || fail "second uninstall removed restored user config"
[[ "$(cat "$HOME/.tmux.conf")" == "$preseed" ]] || fail "second uninstall changed restored user config"
pass "second uninstall is idempotent"
