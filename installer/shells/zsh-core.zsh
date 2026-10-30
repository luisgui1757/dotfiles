# Preserve first-occurrence PATH order while removing inherited duplicates.
typeset -U path PATH

# ---- Locale (set early so anything that reads LANG gets it) ------------------
_dotfiles_locale=""
if command -v locale >/dev/null 2>&1; then
  if locale -a 2>/dev/null | grep -qxE 'en_US\.(UTF-8|utf8)'; then
    _dotfiles_locale="en_US.UTF-8"
  elif locale -a 2>/dev/null | grep -qxE 'C\.(UTF-8|utf8)'; then
    _dotfiles_locale="C.UTF-8"
  fi
fi
if [[ -n "$_dotfiles_locale" ]]; then
  export LANG="$_dotfiles_locale"
  export LC_ALL="$_dotfiles_locale"
  export LANGUAGE="$_dotfiles_locale"
  export LC_MESSAGES="$_dotfiles_locale"
fi
unset _dotfiles_locale

# ---- History -----------------------------------------------------------------
HISTFILE="${HOME}/.zsh_history"
HISTSIZE=50000
SAVEHIST=50000
setopt SHARE_HISTORY HIST_IGNORE_DUPS HIST_IGNORE_ALL_DUPS HIST_REDUCE_BLANKS \
       HIST_IGNORE_SPACE HIST_VERIFY EXTENDED_HISTORY

# ---- Command-line vi mode ----------------------------------------------------
# vi keybindings on the command line. This MUST come before the completion and
# keybinding region below: `bindkey -v` swaps the main keymap from emacs to
# `viins`, so every later unqualified `bindkey ...` (the fzf-tab Tab reclaim, the
# Up/Down history search) lands on the active vi insert keymap instead of a now
# inactive emacs map. Bindings that must also work in normal mode are added to
# `vicmd` explicitly. fzf's own `--zsh` block already binds Ctrl-R/Ctrl-T/Alt-C
# in viins+vicmd, so those chords survive vi mode automatically.
bindkey -v

# KEYTIMEOUT (hundredths of a second) is how long ZLE waits after an ESC before
# treating it as a lone key. In vi mode ESC also leaves insert mode, so this is a
# direct trade-off: too low and any sequence that STARTS with ESC gets split --
# the Meta/Alt prefix (fzf's Alt-C is ESC c) and the arrow-key sequences
# (ESC [ A / ESC [ B) would be misread as "ESC, then a normal-mode command"; too
# high and a typed Esc, then b reads as Option+Left (ESC b, bound below), so the
# shell stays in insert mode. 5 (50 ms) is the Neovim ttimeoutlen default: the
# terminal writes each key sequence at one time, but a person types Esc, then b
# more slowly. Override by exporting DOTFILES_KEYTIMEOUT before startup, or by
# setting KEYTIMEOUT in ~/.zshrc.local (sourced last, so it always wins).
KEYTIMEOUT="${DOTFILES_KEYTIMEOUT:-5}"

# Cursor shape tracks the mode: steady block in normal (vicmd), steady beam in
# insert (viins). DECSCUSR (ESC [ N q) is honoured by Ghostty and Windows
# Terminal and passed through by tmux; terminals that don't support it ignore the
# sequence. Registered through add-zle-hook-widget so it COMPOSES with
# zsh-autosuggestions' own line-init hook instead of clobbering it. Registering
# HERE (before starship init, further down) is load-bearing: starship also hooks
# zle-keymap-select and only PRESERVES an already-registered one, so our
# dispatcher must exist first for the live insert<->normal cursor switch to fire.
_dotfiles_vi_cursor_shape() {
  case "${KEYMAP:-main}" in
    vicmd) print -n '\e[2 q' ;;   # steady block = command (normal) mode
    *)     print -n '\e[6 q' ;;   # steady beam  = insert mode
  esac
}
autoload -Uz add-zle-hook-widget
zle -N _dotfiles_vi_cursor_shape
add-zle-hook-widget keymap-select _dotfiles_vi_cursor_shape
add-zle-hook-widget line-init     _dotfiles_vi_cursor_shape

# ---- zsh plugins -------------------------------------------------------------
# The installer supplies pinned plugins under the private selected root.
_dotfiles_source_first_readable() {
  local plugin
  for plugin in "$@"; do
    if [[ -r "$plugin" ]]; then
      source "$plugin"
      return 0
    fi
  done
  return 1
}



# ---- Completion: native compinit + fzf-tab fuzzy Tab menu (PowerShell-like) --
# Tab opens an fzf-driven, fuzzy-filterable, navigable, colored picker over zsh's
# REAL context-aware completions (git subcommands, ssh hosts, kill PIDs, env
# vars -- not just files): the analog of PowerShell's MenuComplete, with fuzzy
# narrowing on top. fzf-tab WRAPS the Tab widget, so it must load AFTER compinit
# + the completion zstyles and BEFORE zsh-autosuggestions, and it must be the
# LAST thing to own Tab (the fzf key-binding block below reclaims it). When fzf
# is absent it falls back to the native `menu-select` widget so a no-fzf machine
# still gets a usable menu (and zsh_startup_test stays green).
autoload -Uz compinit
mkdir -p "${HOME}/.cache"
_zcompdump="${HOME}/.cache/zcompdump-${ZSH_VERSION}"
# The shared init is also parsed by Bash. Delay only this fixed zsh grammar;
# parameter expansion happens inside zsh, never as executable input to eval.
if eval '[[ -n "$_zcompdump"(#qN.mh+24) ]]'; then
  compinit -d "$_zcompdump"
else
  compinit -C -d "$_zcompdump"
fi
unset _zcompdump
zmodload -i zsh/complist
zstyle ':completion:*' matcher-list 'm:{a-zA-Z}={A-Za-z}'   # case-insensitive matching
zstyle ':completion:*' list-colors "${(s.:.)LS_COLORS}"     # colorize entries
zstyle ':completion:*:descriptions' format '[%d]'           # fzf-tab group headers

if command -v fzf >/dev/null 2>&1; then
  # fzf-tab renders the menu via fzf, so zsh's own menu must be OFF.
  zstyle ':completion:*' menu no
  zstyle ':fzf-tab:*' switch-group '<' '>'
  zstyle ':fzf-tab:*' fzf-min-height 15
  zstyle ':fzf-tab:*' fzf-flags --color=hl:#f6c177,hl+:#f6c177   # gold = selected (Rose Pine)
  zstyle ':fzf-tab:complete:cd:*' fzf-preview 'ls -1p -- "$realpath" 2>/dev/null'
  if ! _dotfiles_source_first_readable \
      "$__DOTFILES_FZF_TAB"; then
    # fzf present but fzf-tab not installed yet -> native menu fallback.
    zstyle ':completion:*' menu select
    bindkey '^I' menu-select                                # viins (main after `bindkey -v`)
    bindkey -M vicmd '^I' menu-select
  fi
else
  zstyle ':completion:*' menu select                        # no fzf -> native menu
  bindkey '^I' menu-select                                  # viins (main after `bindkey -v`)
  bindkey -M vicmd '^I' menu-select
fi

# Inline gray history suggestion (the PowerShell InlinePrediction analog; accept
# the whole suggestion with Right-arrow / End). Loads AFTER fzf-tab so widget
# wrapping order is correct. The one zsh plugin sourced for prediction.
ZSH_AUTOSUGGEST_STRATEGY=(history completion)               # ~ HistoryAndPlugin
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=#908caa'                # match PS InlinePrediction
ZSH_AUTOSUGGEST_MANUAL_REBIND=1                             # perf: no per-prompt rebinds
_dotfiles_source_first_readable \
  "$__DOTFILES_AUTOSUGGESTIONS"

# Up / Down = prefix history search (PowerShell HistorySearchBackward/Forward),
# in BOTH vi insert and command mode so the arrows behave the same regardless of
# mode. (vicmd's own k/j keep their default vi history-line motions.)
autoload -Uz up-line-or-beginning-search down-line-or-beginning-search
zle -N up-line-or-beginning-search
zle -N down-line-or-beginning-search
bindkey -M viins '^[[A' up-line-or-beginning-search
bindkey -M viins '^[[B' down-line-or-beginning-search
bindkey -M vicmd '^[[A' up-line-or-beginning-search
bindkey -M vicmd '^[[B' down-line-or-beginning-search

# Mac line editing, on macOS and on Linux with the Mac keyboard shortcuts.
# Ghostty sends Cmd+Left/Right/Backspace as Ctrl-A/E/U and Option+Left/Right/
# Backspace/Delete as Esc b, Esc f, Esc DEL and ESC [3;3~. The vi keymaps do not
# bind them: Ctrl-A types a literal ^A and each Esc key changes to normal mode.
# Other Linux terminals send Option+Left/Right as ESC [1;3D and ESC [1;3C.
# Delete (Fn+Backspace), Home and End get their usual meaning too. tmux sends
# Home and End as ESC [1~ and ESC [4~.
for _dotfiles_keymap in viins vicmd; do
  bindkey -M "$_dotfiles_keymap" '^A' beginning-of-line
  bindkey -M "$_dotfiles_keymap" '^E' end-of-line
  bindkey -M "$_dotfiles_keymap" '^U' backward-kill-line
  bindkey -M "$_dotfiles_keymap" '^[b' backward-word
  bindkey -M "$_dotfiles_keymap" '^[f' emacs-forward-word
  bindkey -M "$_dotfiles_keymap" '^[[1;3D' backward-word
  bindkey -M "$_dotfiles_keymap" '^[[1;3C' emacs-forward-word
  bindkey -M "$_dotfiles_keymap" '^[^?' backward-kill-word
  bindkey -M "$_dotfiles_keymap" '^[[3;3~' kill-word
  bindkey -M "$_dotfiles_keymap" '^[[3~' delete-char
  bindkey -M "$_dotfiles_keymap" '^[[H' beginning-of-line
  bindkey -M "$_dotfiles_keymap" '^[[1~' beginning-of-line
  bindkey -M "$_dotfiles_keymap" '^[[F' end-of-line
  bindkey -M "$_dotfiles_keymap" '^[[4~' end-of-line
done
unset _dotfiles_keymap

# Ctrl-R fallback for machines WITHOUT fzf: emacs mode defaulted Ctrl-R to
# incremental history search, but viins/vicmd default it to redisplay, so bind
# the search explicitly to preserve that behaviour. When fzf is present its
# `--zsh` block below rebinds Ctrl-R (viins+vicmd) to the fuzzy history picker,
# overriding this.
bindkey -M viins '^R' history-incremental-search-backward
bindkey -M vicmd '^R' history-incremental-search-backward

unset __DOTFILES_FZF_TAB __DOTFILES_AUTOSUGGESTIONS
unfunction _dotfiles_source_first_readable 2>/dev/null || true

