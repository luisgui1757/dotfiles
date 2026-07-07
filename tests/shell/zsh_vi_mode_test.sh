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

# 8. chezmoi twin parity — the fast suite must catch a twin drift on its own.
if ! diff -q "$zshrc" "$REPO_ROOT/home/dot_zshrc" >/dev/null; then
    fail "home/dot_zshrc is not byte-identical to shells/zshrc (chezmoi twin drift)"
fi

echo "static: OK"

# ---- Functional (live keymap inspection) -------------------------------------
# Requires zsh. Sources the real zshrc under `zsh -i` and reads the LIVE keymaps.
# Assertions are written to be robust to whether fzf / fzf-tab happen to be
# installed on the host: fzf's integration rebinds viins Tab and Ctrl-R, but it
# never touches the arrow keys or the vicmd Tab, so those stay deterministic.
if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed (static assertions passed)"
    exit 0
fi

zsh_bin="$(command -v zsh)"
TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

# shellcheck disable=SC2016 # the single-quoted body is zsh code; $ must NOT
# expand in bash. Only "$zshrc" (double-quoted) is interpolated by bash.
probe="$(
    HOME="$TMP_HOME" HOMEBREW_PREFIX='' "$zsh_bin" -i -c '
        source '"$zshrc"' >/dev/null 2>&1
        print -r -- "MAIN=$(bindkey -lL main)"
        print -r -- "KEYTIMEOUT=$KEYTIMEOUT"
        print -r -- "VIINS_UP=${${(z)$(bindkey -M viins "^[[A")}[2]}"
        print -r -- "VICMD_UP=${${(z)$(bindkey -M vicmd "^[[A")}[2]}"
        print -r -- "VIINS_TAB=${${(z)$(bindkey -M viins "^I")}[2]}"
        print -r -- "VICMD_TAB=${${(z)$(bindkey -M vicmd "^I")}[2]}"
        print -r -- "VIINS_CR=${${(z)$(bindkey -M viins "^R")}[2]}"
        print -r -- "CURSOR_WIDGET=${widgets[_dotfiles_vi_cursor_shape]:-MISSING}"
    ' 2>/dev/null
)"

value_of() { # value_of <label>
    printf '%s\n' "$probe" | grep -F "$1=" | head -1 | sed "s/^$1=//"
}

check_contains() { # check_contains <label> <expected-substring>
    local got; got="$(value_of "$1")"
    [[ -n "$got" ]] || fail "functional: no '$1=' in probe output"
    [[ "$got" == *"$2"* ]] || fail "functional: $1 expected '*$2*', got '$got'"
}

check_in_set() { # check_in_set <label> <widget> [widget...]
    local label="$1" got; shift
    got="$(value_of "$label")"
    [[ -n "$got" ]] || fail "functional: no '$label=' in probe output"
    local w
    for w in "$@"; do [[ "$got" == "$w" ]] && return 0; done
    fail "functional: $label='$got' not in {$*}"
}

# `bindkey -v` actually took: the main keymap is the vi insert map.
check_contains "MAIN" "viins"
check_contains "KEYTIMEOUT" "25"
# Arrows are fzf-independent: prefix history search in both vi keymaps.
check_contains "VIINS_UP" "up-line-or-beginning-search"
check_contains "VICMD_UP" "up-line-or-beginning-search"
# Tab still completes under vi mode. vicmd Tab is ours alone (fzf never rebinds
# it); viins Tab is ours unless fzf/fzf-tab rebinds it to their completion widget.
check_in_set "VICMD_TAB" menu-select fzf-tab-complete
check_in_set "VIINS_TAB" menu-select fzf-tab-complete fzf-completion
# Ctrl-R searches history: our fallback, or fzf's picker when fzf is present.
check_in_set "VIINS_CR" history-incremental-search-backward fzf-history-widget
# Cursor-shape widget is registered.
check_contains "CURSOR_WIDGET" "_dotfiles_vi_cursor_shape"

echo "functional: OK"
echo "OK"
