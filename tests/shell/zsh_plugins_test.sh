#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

HOME="$TMP_HOME"
XDG_DATA_HOME="$TMP_HOME/.local/share"
YES_ALL=1
DRY_RUN=1

out="$(install_zsh_plugins)"
[[ "$out" == *"zsh-autocomplete.git"* ]]
[[ "$out" == *"$ZSH_AUTOCOMPLETE_VERSION"* ]]
[[ "$out" == *"$ZSH_AUTOCOMPLETE_COMMIT"* ]]
[[ "$out" == *"zsh-autosuggestions.git"* ]]
[[ "$out" == *"$ZSH_AUTOSUGGESTIONS_VERSION"* ]]
[[ "$out" == *"$ZSH_AUTOSUGGESTIONS_COMMIT"* ]]

zshrc="$REPO_ROOT/shells/zshrc"
# shellcheck disable=SC2016 # grep literals intentionally include shell syntax.
grep -F '_dotfiles_zsh_plugin_root="${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins"' "$zshrc" >/dev/null
# zsh-autosuggestions (inline gray history) is the one sourced zsh plugin.
grep -F 'zsh-autosuggestions/zsh-autosuggestions.zsh' "$zshrc" >/dev/null

# Completion is the NATIVE zsh menu-select system (Tab-driven, PowerShell-like),
# NOT zsh-autocomplete's always-on list. zshrc must NOT source zsh-autocomplete.
if grep -F 'zsh-autocomplete/zsh-autocomplete.plugin.zsh' "$zshrc" >/dev/null; then
    echo "FAIL: zshrc must NOT source zsh-autocomplete (completion is native menu-select)"
    exit 1
fi
grep -F 'autoload -Uz compinit' "$zshrc" >/dev/null || { echo "FAIL: zshrc must run compinit"; exit 1; }
grep -F "zstyle ':completion:*' menu select" "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must enable menu select"; exit 1; }
grep -F 'zmodload -i zsh/complist' "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must load zsh/complist for the menu-select widget"; exit 1; }

# Tab -> menu-select, and it must be RECLAIMED after fzf (fzf's `--zsh` integration
# rebinds Tab to fzf-completion). The reclaiming bindkey must come AFTER the fzf
# source, or fzf wins Tab and the native menu is unreachable.
if awk '
    /source <\(fzf --zsh\)/ { fzf = NR }
    /bindkey .\^I. menu-select/ { last_bind = NR }
    END { exit !(fzf && last_bind && last_bind > fzf) }
' "$zshrc"; then
    :
else
    echo "FAIL: zshrc must reclaim Tab with bindkey menu-select AFTER sourcing fzf"
    exit 1
fi

grep -F 'skip_global_compinit=1' "$REPO_ROOT/shells/zshenv" >/dev/null

echo "OK"
