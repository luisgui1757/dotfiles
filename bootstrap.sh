#!/usr/bin/env bash
# bootstrap.sh — symlink dotfiles into OS-appropriate paths.
# Supported: macOS (Darwin), native Linux, WSL (Microsoft kernel under Linux).
# Idempotent: re-running produces zero diffs.
#
# Usage:
#   ./bootstrap.sh           # install / re-install symlinks
#   ./bootstrap.sh --dry-run # print planned actions, change nothing
#   ./bootstrap.sh --experimental-wsl-gui
#                            # WSL opt-in: link Linux Ghostty config

set -euo pipefail

DRY_RUN=0
EXPERIMENTAL_WSL_GUI="${DOTFILES_EXPERIMENTAL_WSL_GUI:-0}"
for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=1 ;;
        --experimental-wsl-gui)
            EXPERIMENTAL_WSL_GUI=1
            export DOTFILES_EXPERIMENTAL_WSL_GUI=1 ;;
        -h|--help)
            sed -n '2,12p' "$0"; exit 0 ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done
[[ "$EXPERIMENTAL_WSL_GUI" == "1" ]] || EXPERIMENTAL_WSL_GUI=0

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

# Pick a collision-free backup path: if .bak.<timestamp> already exists,
# append .1, .2, … until we find an unused name.
unique_backup() {
    local base="$1"
    if [[ ! -e "$base" ]]; then printf '%s' "$base"; return; fi
    local i=1
    while [[ -e "${base}.${i}" ]]; do i=$((i + 1)); done
    printf '%s' "${base}.${i}"
}

# ---- OS detection ------------------------------------------------------------
# DOTFILES_FORCE_OS is a test seam: it lets the suite exercise macos/linux/wsl
# link branches regardless of the host runner. Unset in normal runs, so real
# detection is used.
detect_os() {
    if [[ -n "${DOTFILES_FORCE_OS:-}" ]]; then echo "$DOTFILES_FORCE_OS"; return; fi
    case "$(uname -s)" in
        Darwin)
            echo macos ;;
        Linux)
            if grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null; then
                echo wsl
            else
                echo linux
            fi ;;
        *) echo unknown ;;
    esac
}

OS="$(detect_os)"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

# ---- Preflight ---------------------------------------------------------------
require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "bootstrap: required command '$1' not on PATH" >&2
        exit 1
    fi
}
require_cmd ln
require_cmd readlink

echo "bootstrap: OS=$OS repo=$REPO_ROOT dry-run=$DRY_RUN"
if [[ "$OS" == "wsl" ]]; then
    echo "bootstrap: experimental-wsl-gui=$EXPERIMENTAL_WSL_GUI"
fi
echo

# ---- Link primitives ---------------------------------------------------------
# Create a symlink at $2 → $1. If $2 already exists:
#   - if it's the correct symlink: no-op.
#   - if it's a different symlink or a regular file: back it up to
#     <target>.bak.<timestamp> and replace.
link() {
    local src="$1" dst="$2"

    # Ensure parent dir exists
    local parent
    parent="$(dirname "$dst")"
    if [[ ! -d "$parent" ]]; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  mkdir -p  $parent"
        else
            mkdir -p "$parent"
        fi
    fi

    if [[ -L "$dst" ]]; then
        local current
        current="$(readlink "$dst")"
        if [[ "$current" == "$src" ]]; then
            echo "  ok        $dst → $src"
            return 0
        fi
        # Existing symlink points elsewhere — back up the symlink itself
        # before replacing. We don't dereference (we want to preserve the
        # user's prior choice, not its target).
        local backup
        backup="$(unique_backup "${dst}.bak.${TIMESTAMP}")"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  relink    $dst (was → $current; backup → $backup)"
        else
            mv "$dst" "$backup"
            replace_symlink "$src" "$dst"
            echo "  relinked  $dst → $src  (prior symlink → $backup)"
        fi
        return 0
    fi

    if [[ -e "$dst" ]]; then
        local backup
        backup="$(unique_backup "${dst}.bak.${TIMESTAMP}")"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  backup    $dst → $backup; then symlink"
        else
            mv "$dst" "$backup"   # mv -n would still race; unique_backup is the guarantee
            replace_symlink "$src" "$dst"
            echo "  backed up $dst → $backup; linked → $src"
        fi
        return 0
    fi

    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  link      $dst → $src"
    else
        replace_symlink "$src" "$dst"
        echo "  linked    $dst → $src"
    fi
}

replace_symlink() {
    local src="$1" dst="$2"
    local tmp="${dst}.new"
    rm -f "$tmp"
    ln -s "$src" "$tmp"
    mv -f "$tmp" "$dst"
}

# ---- Self-link guard ---------------------------------------------------------
# Refuse to run if the repo lives at the same path bootstrap would symlink to
# itself. Without this, we'd back up the repo dir and replace it with a broken
# symlink, corrupting the worktree.
NVIM_DEST="${HOME}/.config/nvim"
realpath_or_self() {
    if command -v realpath >/dev/null 2>&1; then
        realpath "$1" 2>/dev/null || echo "$1"
    elif command -v readlink >/dev/null 2>&1 && readlink -f / >/dev/null 2>&1; then
        # GNU readlink -f works on Linux but not on stock macOS (until 14+).
        readlink -f "$1" 2>/dev/null || echo "$1"
    else
        # macOS fallback: physical pwd in the directory.
        ( cd "$1" 2>/dev/null && pwd -P ) || echo "$1"
    fi
}
REPO_NVIM="$(realpath_or_self "${REPO_ROOT}/nvim")"
REPO_R="$(realpath_or_self "$REPO_ROOT")"
# If the destination is already a symlink, bootstrap.sh's link() handles the
# relink / no-op cases correctly; the self-link risk only exists when the
# destination is a real directory or file. Compare physical paths only in
# that case.
SELF_LINK=0
if [[ ! -L "$NVIM_DEST" ]] && [[ -e "$NVIM_DEST" ]]; then
    DEST_R="$(realpath_or_self "$NVIM_DEST")"
    # Two self-link scenarios:
    #   A) Repo's nvim/ subdir IS the destination directory
    #   B) The repo root IS the destination directory (clone -> ~/.config/nvim)
    if [[ "$REPO_NVIM" == "$DEST_R" ]] || [[ "$REPO_R" == "$DEST_R" ]]; then
        SELF_LINK=1
    fi
fi
if [[ "$SELF_LINK" -eq 1 ]]; then
    cat >&2 <<EOF
bootstrap.sh: REFUSING to run.

  Repo root:             $REPO_R
  Repo nvim/ dir:        $REPO_NVIM
  Symlink destination:   $NVIM_DEST

  The symlink we would create would overlap your repo. Running
  bootstrap would back up the repo and replace it with a symlink
  to nothing.

  Move the repo to a different location first, e.g.:
      mv "$REPO_R" "$HOME/dotfiles"
      cd "$HOME/dotfiles"
      ./bootstrap.sh
EOF
    exit 1
fi

# ---- Universal links (mac + linux + wsl) -------------------------------------
link "${REPO_ROOT}/nvim"                "${NVIM_DEST}"
link "${REPO_ROOT}/starship/starship.toml" "${HOME}/.config/starship.toml"
link "${REPO_ROOT}/tmux/tmux.conf"      "${HOME}/.tmux.conf"
link "${REPO_ROOT}/shells/zshenv"       "${HOME}/.zshenv"
link "${REPO_ROOT}/shells/zshrc"        "${HOME}/.zshrc"

# ---- OS-specific links -------------------------------------------------------
case "$OS" in
    macos)
        link "${REPO_ROOT}/ghostty/config" \
             "${HOME}/Library/Application Support/com.mitchellh.ghostty/config"
        # lazygit on macOS reads from Application Support by default
        # (`lazygit --print-config-dir`), not ~/.config/lazygit.
        link "${REPO_ROOT}/lazygit/config.yml" \
             "${HOME}/Library/Application Support/lazygit/config.yml"
        ;;
    linux)
        link "${REPO_ROOT}/ghostty/config" "${HOME}/.config/ghostty/config"
        link "${REPO_ROOT}/lazygit/config.yml" "${HOME}/.config/lazygit/config.yml"
        ;;
    wsl)
        if [[ "$EXPERIMENTAL_WSL_GUI" -eq 1 ]]; then
            link "${REPO_ROOT}/ghostty/config" "${HOME}/.config/ghostty/config"
        else
            echo "  skipped   ${HOME}/.config/ghostty/config (WSL uses Windows Terminal by default)"
            echo "            pass --experimental-wsl-gui to link Linux Ghostty config"
        fi
        link "${REPO_ROOT}/lazygit/config.yml" "${HOME}/.config/lazygit/config.yml"
        echo "  note      WSL detected; merge Windows Terminal on the Windows host"
        echo "            run: .\\setup.ps1 -All -MergeWindowsTerminal"
        ;;
    *)
        link "${REPO_ROOT}/lazygit/config.yml" "${HOME}/.config/lazygit/config.yml"
        echo "  warn      unknown OS; skipping terminal-specific links" >&2
        ;;
esac

# ---- Optional: install pwsh profile if pwsh is on PATH -----------------------
if command -v pwsh >/dev/null 2>&1; then
    # Resolve $PROFILE for pwsh (works on mac/linux too).
    # shellcheck disable=SC2016  # single quotes intentional — $PROFILE is a PowerShell variable
    if PROFILE_PATH=$(pwsh -NoProfile -Command 'Write-Output $PROFILE' 2>/dev/null); then
        PROFILE_PATH="$(echo "$PROFILE_PATH" | tr -d '\r')"
        if [[ -n "$PROFILE_PATH" ]]; then
            link "${REPO_ROOT}/shells/powershell_profile.ps1" "$PROFILE_PATH"
        fi
    fi
fi

echo
echo "bootstrap: done"
if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "(dry run — no changes were made)"
fi
