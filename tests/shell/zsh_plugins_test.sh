#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
zshrc="$REPO_ROOT/shells/zshrc"
# shellcheck disable=SC2016 # grep literals intentionally include shell syntax.
grep -F '_dotfiles_zsh_plugin_root="$HOME/.local/share/dotfiles/zsh-plugins"' "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must use the fixed ~/.local/share zsh plugin root"; exit 1; }
if grep -F 'XDG_DATA_HOME' "$zshrc" | grep -F 'zsh-plugins' >/dev/null; then
    echo "FAIL: zshrc must not make the zsh plugin root depend on XDG_DATA_HOME"
    exit 1
fi
# zsh-autosuggestions (inline gray history) is sourced for prediction.
grep -F 'zsh-autosuggestions/zsh-autosuggestions.zsh' "$zshrc" >/dev/null

# Completion is fzf-tab (fzf-driven fuzzy Tab menu) over native compinit -- NOT
# zsh-autocomplete. zshrc must source fzf-tab and must NOT source zsh-autocomplete.
grep -F 'fzf-tab/fzf-tab.plugin.zsh' "$zshrc" >/dev/null \
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
    /source <\(fzf --zsh\)/ { fzf = NR }
    /bindkey .\^I. fzf-tab-complete/ { last_bind = NR }
    END { exit !(fzf && last_bind && last_bind > fzf) }
' "$zshrc"; then
    :
else
    echo "FAIL: zshrc must reclaim Tab with bindkey fzf-tab-complete AFTER sourcing fzf"
    exit 1
fi

# zoxide's `z`/`zi` init must be guarded (so a machine without zoxide still
# starts) AND must run AFTER compinit -- upstream requires post-compinit
# placement for its completions to register.
grep -F 'command -v zoxide' "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must guard zoxide init with 'command -v zoxide'"; exit 1; }
if awk '
    /^[[:space:]]*compinit[[:space:]]/ { compinit_call = NR }
    /zoxide init zsh/                  { zoxide_line = NR }
    END { exit !(compinit_call && zoxide_line && zoxide_line > compinit_call) }
' "$zshrc"; then
    :
else
    echo "FAIL: zshrc must run 'zoxide init zsh' AFTER compinit"
    exit 1
fi

grep -F 'skip_global_compinit=1' "$REPO_ROOT/shells/zshenv" >/dev/null

echo "OK"
