#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"
zshrc="$ACTIVE_ZSH"
# The production composer binds the selected private plugin paths.
grep -F '__DOTFILES_FZF_TAB=' "$zshrc" >/dev/null
grep -F '__DOTFILES_AUTOSUGGESTIONS=' "$zshrc" >/dev/null
grep -F '/plugin.zsh-autosuggestions/current/zsh-autosuggestions.zsh' "$zshrc" >/dev/null

# Completion is fzf-tab (fzf-driven fuzzy Tab menu) over native compinit -- NOT
# zsh-autocomplete. zshrc must source fzf-tab and must NOT source zsh-autocomplete.
grep -F 'plugin.fzf-tab/current/fzf-tab.plugin.zsh' "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must source fzf-tab"; exit 1; }
if grep -F 'zsh-autocomplete/zsh-autocomplete.plugin.zsh' "$zshrc" >/dev/null; then
    echo "FAIL: zshrc must NOT source zsh-autocomplete (completion is fzf-tab)"
    exit 1
fi
grep -F 'autoload -Uz compinit' "$zshrc" >/dev/null || { echo "FAIL: zshrc must run compinit"; exit 1; }
grep -F 'zmodload -i zsh/complist' "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must load zsh/complist (native menu-select fallback)"; exit 1; }
# fzf-tab requires zsh's own menu OFF (it draws the menu via fzf).
grep -F "zstyle ':completion:*' menu no" "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must set 'menu no' so fzf-tab owns the menu"; exit 1; }

# fzf's `--zsh` integration rebinds Tab to fzf-completion, so Tab must be
# RECLAIMED for fzf-tab AFTER the fzf source, or fzf wins Tab and fzf-tab is
# unreachable.
if awk '
    /source <\(.*fzf.* --zsh\)/ { fzf = NR }
    /bindkey .\^I. fzf-tab-complete/ { last_bind = NR }
    END { exit !(fzf && last_bind && last_bind > fzf) }
' "$zshrc"; then
    :
else
    echo "FAIL: zshrc must reclaim Tab with bindkey fzf-tab-complete AFTER sourcing fzf"
    exit 1
fi

# The selected managed executable initializes zoxide after native completion.
awk '
    /^[[:space:]]*compinit[[:space:]]/ { compinit_call = NR }
    /zoxide.* init zsh/ { zoxide_line = NR }
    END { exit !(compinit_call && zoxide_line > compinit_call) }
' "$zshrc" || { echo "FAIL: zoxide must follow compinit"; exit 1; }
grep -F 'skip_global_compinit=1' "$DOTFILES_SHELL_FIXTURES/posix/.zshenv" >/dev/null

echo "OK"
