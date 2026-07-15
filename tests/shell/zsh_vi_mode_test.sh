#!/usr/bin/env bash
# Regression guard for PR-2: command-line vi mode in zshrc.
#
# The static half proves the source keeps the load-bearing structure (vi mode
# enabled BEFORE the completion/keybinding region, explicit KEYTIMEOUT with a
# local override, and Tab/Up/Down/Ctrl-R bound on the right vi keymaps). The
# functional half sources the real zshrc under `zsh -i` with fzf deliberately off
# PATH, then inspects the LIVE keymaps so a future edit that silently reverts to
# emacs mode, or drops a keymap, fails here.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
zshrc="$REPO_ROOT/shells/zshrc"

fail() { echo "FAIL: $1"; exit 1; }

# ---- Static structure --------------------------------------------------------

# 1. vi mode is enabled.
grep -Eq '^[[:space:]]*bindkey -v[[:space:]]*$' "$zshrc" \
    || fail "zshrc must enable vi mode with 'bindkey -v'"

# 2. `bindkey -v` must run BEFORE the completion/keybinding region, so later
#    unqualified `bindkey ...` calls (Tab reclaim, Up/Down) land on the active vi
#    insert keymap. Prove it precedes compinit and every Tab/Up-Down binding.
awk '
    /^[[:space:]]*bindkey -v[[:space:]]*$/ { viv = NR }
    /autoload -Uz compinit/                { if (!compinit) compinit = NR }
    /bindkey.*fzf-tab-complete/            { last_tab = NR }
    /bindkey.*up-line-or-beginning-search/ { if (!up) up = NR }
    END {
        if (!viv)              { print "no bindkey -v";                exit 1 }
        if (!(viv < compinit)) { print "bindkey -v not before compinit"; exit 1 }
        if (!(viv < last_tab)) { print "bindkey -v not before Tab bind";  exit 1 }
        if (!(viv < up))       { print "bindkey -v not before Up bind";   exit 1 }
    }
' "$zshrc" \
    || fail "zshrc: 'bindkey -v' must precede compinit and the Tab/Up-Down bindings"

# 3. KEYTIMEOUT set explicitly, with a documented local override path.
grep -Eq 'KEYTIMEOUT="\$\{DOTFILES_KEYTIMEOUT:-[0-9]+\}"' "$zshrc" \
    || fail "zshrc must set KEYTIMEOUT explicitly with a DOTFILES_KEYTIMEOUT override"

# 4. Up/Down prefix history search bound in BOTH viins and vicmd.
for km in viins vicmd; do
    grep -Fq "bindkey -M $km '^[[A' up-line-or-beginning-search" "$zshrc" \
        || fail "zshrc must bind Up (history search) in $km"
    grep -Fq "bindkey -M $km '^[[B' down-line-or-beginning-search" "$zshrc" \
        || fail "zshrc must bind Down (history search) in $km"
done

# 5. Tab: vicmd reclaim is explicit; viins reclaim rides the unqualified (main)
#    binding, which the plugins test already asserts runs AFTER the fzf source.
grep -Fq "bindkey -M vicmd '^I' fzf-tab-complete" "$zshrc" \
    || fail "zshrc must bind Tab (fzf-tab-complete) in vicmd"
grep -Fq "bindkey '^I' fzf-tab-complete" "$zshrc" \
    || fail "zshrc must keep the unqualified Tab reclaim (viins via main keymap)"

# 6. Ctrl-R history fallback bound in viins+vicmd (fzf overrides when present).
for km in viins vicmd; do
    grep -Fq "bindkey -M $km '^R' history-incremental-search-backward" "$zshrc" \
        || fail "zshrc must bind a Ctrl-R history fallback in $km"
done

# 7. Cursor-shape handling registered via add-zle-hook-widget on keymap-select
#    AND line-init (composes with zsh-autosuggestions / starship instead of
#    clobbering their hooks).
grep -Eq 'add-zle-hook-widget[[:space:]]+keymap-select[[:space:]]+_dotfiles_vi_cursor_shape' "$zshrc" \
    || fail "zshrc must register the vi cursor via add-zle-hook-widget keymap-select"
grep -Eq 'add-zle-hook-widget[[:space:]]+line-init[[:space:]]+_dotfiles_vi_cursor_shape' "$zshrc" \
    || fail "zshrc must register the vi cursor via add-zle-hook-widget line-init"

awk '
    /add-zle-hook-widget[[:space:]]+keymap-select[[:space:]]+_dotfiles_vi_cursor_shape/ { hook = NR }
    /starship init zsh --print-full-init/ { starship = NR }
    END {
        if (!hook)     { print "missing cursor hook registration"; exit 1 }
        if (!starship) { print "missing starship init"; exit 1 }
        if (!(hook < starship)) {
            print "cursor hook registration must precede starship init"
            exit 1
        }
    }
' "$zshrc" \
    || fail "zshrc must register cursor hooks before starship init"

# 8. chezmoi twin parity — the fast suite must catch a twin drift on its own.
if ! diff -q "$zshrc" "$REPO_ROOT/home/dot_zshrc" >/dev/null; then
    fail "home/dot_zshrc is not byte-identical to shells/zshrc (chezmoi twin drift)"
fi

echo "static: OK"

# ---- Functional (live keymap inspection) -------------------------------------
# Requires zsh. Sources the real zshrc under `zsh -i` and reads the LIVE keymaps.
# Run two deterministic legs:
#   1. no fzf on PATH -> native menu-select fallback owns Tab.
#   2. fake fzf + repo-managed fzf-tab widget -> fzf may bind Tab first, then
#      zshrc must reclaim Tab for fzf-tab in both vi keymaps.
if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed (static assertions passed)"
    exit 0
fi

zsh_bin="$(command -v zsh)"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

run_probe() {
    local home="$1" path_value="$2"
    # shellcheck disable=SC2016 # the single-quoted body is zsh code; $ must NOT
    # expand in bash. Only "$zshrc" (double-quoted) is interpolated by bash.
    HOME="$home" HOMEBREW_PREFIX="$home/no-brew" __HM_SESS_VARS_SOURCED=1 \
        DOTFILES_TEST_PATH="$path_value" "$zsh_bin" -i -c '
        export PATH="$DOTFILES_TEST_PATH"
        export HOMEBREW_PREFIX="$HOME/no-brew"
        path=(${(s.:.)PATH})
        rehash
        source '"$zshrc"' >/dev/null 2>&1 || exit 1
        print -r -- "MAIN=$(bindkey -lL main)"
        print -r -- "KEYTIMEOUT=$KEYTIMEOUT"
        print -r -- "VIINS_UP=${${(z)$(bindkey -M viins "^[[A")}[2]}"
        print -r -- "VICMD_UP=${${(z)$(bindkey -M vicmd "^[[A")}[2]}"
        print -r -- "VIINS_TAB=${${(z)$(bindkey -M viins "^I")}[2]}"
        print -r -- "VICMD_TAB=${${(z)$(bindkey -M vicmd "^I")}[2]}"
        print -r -- "VIINS_CR=${${(z)$(bindkey -M viins "^R")}[2]}"
        print -r -- "CURSOR_WIDGET=${widgets[_dotfiles_vi_cursor_shape]:-MISSING}"
    ' 2>/dev/null
}

value_of() { # value_of <probe> <label>
    printf '%s\n' "$1" | grep -F "$2=" | head -1 | sed "s/^$2=//"
}

check_contains() { # check_contains <leg> <probe> <label> <expected-substring>
    local leg="$1" probe="$2" label="$3" expected="$4" got
    got="$(value_of "$probe" "$label")"
    [[ -n "$got" ]] || fail "functional/$leg: no '$label=' in probe output"
    [[ "$got" == *"$expected"* ]] || fail "functional/$leg: $label expected '*$expected*', got '$got'"
}

check_equals() { # check_equals <leg> <probe> <label> <expected>
    local leg="$1" probe="$2" label="$3" expected="$4" got
    got="$(value_of "$probe" "$label")"
    [[ -n "$got" ]] || fail "functional/$leg: no '$label=' in probe output"
    [[ "$got" == "$expected" ]] || fail "functional/$leg: $label expected '$expected', got '$got'"
}

common_checks() {
    local leg="$1" probe="$2"
    check_contains "$leg" "$probe" "MAIN" "viins"
    check_contains "$leg" "$probe" "KEYTIMEOUT" "25"
    check_equals "$leg" "$probe" "VIINS_UP" "up-line-or-beginning-search"
    check_equals "$leg" "$probe" "VICMD_UP" "up-line-or-beginning-search"
    check_contains "$leg" "$probe" "CURSOR_WIDGET" "_dotfiles_vi_cursor_shape"
}

NO_FZF_HOME="$TMP_ROOT/no-fzf-home"
NO_FZF_BIN="$TMP_ROOT/no-fzf-bin"
mkdir -p "$NO_FZF_HOME" "$NO_FZF_BIN"
ln -s "$(command -v mkdir)" "$NO_FZF_BIN/mkdir"
no_fzf_probe="$(run_probe "$NO_FZF_HOME" "$NO_FZF_BIN")" \
    || fail "functional/no-fzf: zshrc failed to source"
common_checks "no-fzf" "$no_fzf_probe"
check_equals "no-fzf" "$no_fzf_probe" "VIINS_TAB" "menu-select"
check_equals "no-fzf" "$no_fzf_probe" "VICMD_TAB" "menu-select"
check_equals "no-fzf" "$no_fzf_probe" "VIINS_CR" "history-incremental-search-backward"

FZF_HOME="$TMP_ROOT/with-fzf-home"
FZF_BIN="$TMP_ROOT/fake-bin"
mkdir -p "$FZF_HOME/.local/share/dotfiles/zsh-plugins/fzf-tab" "$FZF_BIN"
ln -s "$(command -v mkdir)" "$FZF_BIN/mkdir"
ln -s "$(command -v cat)" "$FZF_BIN/cat"
cat > "$FZF_HOME/.local/share/dotfiles/zsh-plugins/fzf-tab/fzf-tab.plugin.zsh" <<'ZSH'
fzf-tab-complete() { zle menu-select }
zle -N fzf-tab-complete
ZSH
cat > "$FZF_BIN/fzf" <<'SH'
#!/bin/sh
if [ "${1:-}" = "--zsh" ]; then
  cat <<'ZSH'
fzf-completion() { zle menu-select }
zle -N fzf-completion
bindkey '^I' fzf-completion
fzf-history-widget() { zle history-incremental-search-backward }
zle -N fzf-history-widget
bindkey -M viins '^R' fzf-history-widget
bindkey -M vicmd '^R' fzf-history-widget
ZSH
  exit 0
fi
exit 0
SH
chmod +x "$FZF_BIN/fzf"
with_fzf_probe="$(run_probe "$FZF_HOME" "$FZF_BIN")" \
    || fail "functional/with-fzf-tab: zshrc failed to source"
common_checks "with-fzf-tab" "$with_fzf_probe"
check_equals "with-fzf-tab" "$with_fzf_probe" "VIINS_TAB" "fzf-tab-complete"
check_equals "with-fzf-tab" "$with_fzf_probe" "VICMD_TAB" "fzf-tab-complete"
check_equals "with-fzf-tab" "$with_fzf_probe" "VIINS_CR" "fzf-history-widget"

echo "functional: OK"
echo "OK"
