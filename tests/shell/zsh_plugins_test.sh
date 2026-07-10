#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

HOME="$TMP_HOME"
XDG_DATA_HOME="$TMP_HOME/.xdg-data"
YES_ALL=1
DRY_RUN=1

out="$(install_zsh_plugins)"
[[ "$out" == *"fzf-tab.git"* ]] \
    || { echo "FAIL: zsh plugin dry-run omitted the fzf-tab origin"; exit 1; }
[[ "$out" == *"$FZF_TAB_VERSION"* ]] \
    || { echo "FAIL: zsh plugin dry-run omitted the fzf-tab reviewed ref"; exit 1; }
[[ "$out" == *"$FZF_TAB_COMMIT"* ]] \
    || { echo "FAIL: zsh plugin dry-run omitted the fzf-tab exact commit"; exit 1; }
[[ "$out" == *"zsh-autosuggestions.git"* ]] \
    || { echo "FAIL: zsh plugin dry-run omitted the autosuggestions origin"; exit 1; }
[[ "$out" == *"$ZSH_AUTOSUGGESTIONS_VERSION"* ]] \
    || { echo "FAIL: zsh plugin dry-run omitted the autosuggestions reviewed ref"; exit 1; }
[[ "$out" == *"$ZSH_AUTOSUGGESTIONS_COMMIT"* ]] \
    || { echo "FAIL: zsh plugin dry-run omitted the autosuggestions exact commit"; exit 1; }

# A failing pinned-plugin install must (1) still attempt BOTH plugins, (2) surface
# a FAIL: marker so CI catches it (the old `|| true` swallowed it and reported
# success), yet (3) CONTINUE (return 0) -- zsh plugins are non-critical, so a clone
# hiccup must not abort the whole setup under set -e. Stubs isolated in a subshell;
# the attempt log is a FILE because the $(...) capture runs in its own subshell.
(
    set +e
    attempt_log="$TMP_HOME/zsh_attempts.log"; : > "$attempt_log"
    zsh_plugin_ok() { return 1; }                                        # force "not installed"
    install_zsh_plugin_repo() { echo "$1" >> "$attempt_log"; return 1; } # log name, simulate failure
    YES_ALL=1
    DRY_RUN=0
    fc_out="$(install_zsh_plugins 2>&1)"; fc_rc=$?
    { grep -qx 'fzf-tab' "$attempt_log" && grep -qx 'zsh-autosuggestions' "$attempt_log"; } \
        || { echo "FAIL: install_zsh_plugins must attempt BOTH plugins when the first fails"; exit 1; }
    [[ "$fc_out" == *"FAIL:"* ]] \
        || { echo "FAIL: install_zsh_plugins must emit a FAIL: marker on plugin failure"; exit 1; }
    [[ "$fc_rc" -eq 0 ]] \
        || { echo "FAIL: install_zsh_plugins must return 0 (continue) on a non-critical plugin failure"; exit 1; }
)

zshrc="$REPO_ROOT/shells/zshrc"
# shellcheck disable=SC2016 # grep literals intentionally include shell syntax.
grep -F '_dotfiles_zsh_plugin_root="$HOME/.local/share/dotfiles/zsh-plugins"' "$zshrc" >/dev/null \
    || { echo "FAIL: zshrc must use the fixed ~/.local/share zsh plugin root"; exit 1; }
if grep -F 'XDG_DATA_HOME' "$zshrc" | grep -F 'zsh-plugins' >/dev/null; then
    echo "FAIL: zshrc must not make the zsh plugin root depend on XDG_DATA_HOME"
    exit 1
fi
if grep -F 'type = "git-repo"' "$REPO_ROOT/home/.chezmoiexternal.toml.tmpl" >/dev/null; then
    echo "FAIL: generic chezmoi git-repo externals must not publish sourceable zsh payloads"
    exit 1
fi
ensure_template="$REPO_ROOT/home/.chezmoiscripts/run_onchange_after_20-ensure-zsh-plugin-pins.sh.tmpl"
# shellcheck disable=SC2016 # literal template syntax is intentional.
grep -F 'root="$HOME/.local/share/dotfiles/zsh-plugins"' "$ensure_template" >/dev/null \
    || { echo "FAIL: checked publisher must use the fixed plugin root"; exit 1; }
grep -F 'scripts/ensure-pinned-zsh-plugin.sh' "$ensure_template" >/dev/null \
    || { echo "FAIL: chezmoi must call the canonical staged publisher"; exit 1; }
grep -F "$FZF_TAB_COMMIT" "$ensure_template" >/dev/null \
    || { echo "FAIL: chezmoi fzf-tab commit pin drift"; exit 1; }
grep -F "$ZSH_AUTOSUGGESTIONS_COMMIT" "$ensure_template" >/dev/null \
    || { echo "FAIL: chezmoi autosuggestions commit pin drift"; exit 1; }
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
