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
grep -F 'zsh-autocomplete/zsh-autocomplete.plugin.zsh' "$zshrc" >/dev/null
grep -F 'zsh-autosuggestions/zsh-autosuggestions.zsh' "$zshrc" >/dev/null
# shellcheck disable=SC2016 # grep literal intentionally includes shell syntax.
grep -F 'if [[ "$_dotfiles_autocomplete_loaded" -ne 1 ]]; then' "$zshrc" >/dev/null

if awk '
    /zsh-autocomplete\/zsh-autocomplete.plugin.zsh/ { autocomplete = NR }
    /autoload -Uz compinit/ { compinit = NR }
    END { exit !(autocomplete && compinit && autocomplete < compinit) }
' "$zshrc"; then
    :
else
    echo "FAIL: zsh-autocomplete must be sourced before local compinit"
    exit 1
fi

grep -F 'skip_global_compinit=1' "$REPO_ROOT/shells/zshenv" >/dev/null

echo "OK"
