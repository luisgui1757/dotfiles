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
[[ "$out" == *"fzf-tab.git"* ]]
[[ "$out" == *"$FZF_TAB_VERSION"* ]]
[[ "$out" == *"$FZF_TAB_COMMIT"* ]]
[[ "$out" == *"zsh-autosuggestions.git"* ]]
[[ "$out" == *"$ZSH_AUTOSUGGESTIONS_VERSION"* ]]
[[ "$out" == *"$ZSH_AUTOSUGGESTIONS_COMMIT"* ]]

# Fail-closed: a failing pinned-plugin install must surface a FAIL: marker (so CI
# catches it) and return non-zero -- the old `|| true` swallowed it and reported
# success with the plugin absent. Stubs are isolated in a subshell.
(
    set +e  # we EXPECT a non-zero return; capture it instead of aborting
    zsh_plugin_ok() { return 1; }            # force "not installed" so it proceeds
    install_zsh_plugin_repo() { return 1; }  # simulate a clone/verify failure
    YES_ALL=1
    DRY_RUN=0
    fc_out="$(install_zsh_plugins 2>&1)"; fc_rc=$?
    [[ "$fc_rc" -ne 0 ]] \
        || { echo "FAIL: install_zsh_plugins must return non-zero when a plugin fails"; exit 1; }
    [[ "$fc_out" == *"FAIL:"* ]] \
        || { echo "FAIL: install_zsh_plugins must emit a FAIL: marker on plugin failure"; exit 1; }
)

zshrc="$REPO_ROOT/shells/zshrc"
# shellcheck disable=SC2016 # grep literals intentionally include shell syntax.
grep -F '_dotfiles_zsh_plugin_root="${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins"' "$zshrc" >/dev/null
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

grep -F 'skip_global_compinit=1' "$REPO_ROOT/shells/zshenv" >/dev/null

# The chezmoi source twin must stay byte-identical to the canonical file, else
# `chezmoi apply` ships a different (e.g. still-zsh-autocomplete) zshrc than the
# one these assertions just validated. The full parity_gate also enforces this;
# assert it here too so the fast suite catches a twin drift on its own.
if ! diff -q "$zshrc" "$REPO_ROOT/home/dot_zshrc" >/dev/null; then
    echo "FAIL: home/dot_zshrc is not byte-identical to shells/zshrc (chezmoi twin drift)"
    exit 1
fi
if ! diff -q "$REPO_ROOT/shells/zshenv" "$REPO_ROOT/home/dot_zshenv" >/dev/null; then
    echo "FAIL: home/dot_zshenv is not byte-identical to shells/zshenv (chezmoi twin drift)"
    exit 1
fi

echo "OK"
