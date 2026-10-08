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

# ---- Rose Pine file listing colors ------------------------------------------
# lsd reads LS_COLORS for file/directory names; zsh completion reuses the same
# palette through the list-colors zstyle below. Dotfiles owns this palette by
# default; set DOTFILES_LS_COLORS before shell startup for an explicit override.
_dotfiles_ls_colors=(
  'no=38;2;224;222;244'
  'fi=38;2;224;222;244'
  'di=38;2;246;193;119'
  'ln=38;2;196;167;231'
  'pi=38;2;156;207;216'
  'so=38;2;156;207;216'
  'do=38;2;156;207;216'
  'bd=38;2;235;188;186'
  'cd=38;2;235;188;186'
  'or=38;2;235;111;146'
  'mi=38;2;235;111;146'
  'su=38;2;235;111;146'
  'sg=38;2;246;193;119'
  'ca=38;2;235;188;186'
  'tw=38;2;246;193;119'
  'ow=38;2;246;193;119'
  'st=38;2;246;193;119'
  'ex=38;2;235;111;146'
  '*.md=38;2;156;207;216'
  '*.markdown=38;2;156;207;216'
  '*.txt=38;2;224;222;244'
  '*.toml=38;2;156;207;216'
  '*.yaml=38;2;156;207;216'
  '*.yml=38;2;156;207;216'
  '*.json=38;2;156;207;216'
  '*.jsonc=38;2;156;207;216'
  '*.lua=38;2;196;167;231'
  '*.vim=38;2;196;167;231'
  '*.sh=38;2;246;193;119'
  '*.bash=38;2;246;193;119'
  '*.zsh=38;2;246;193;119'
  '*.ps1=38;2;196;167;231'
  '*.zip=38;2;235;188;186'
  '*.tar=38;2;235;188;186'
  '*.gz=38;2;235;188;186'
  '*.tgz=38;2;235;188;186'
  '*.xz=38;2;235;188;186'
  '*.7z=38;2;235;188;186'
  '*.png=38;2;235;188;186'
  '*.jpg=38;2;235;188;186'
  '*.jpeg=38;2;235;188;186'
  '*.webp=38;2;235;188;186'
  '*.svg=38;2;156;207;216'
)
export LS_COLORS="${DOTFILES_LS_COLORS:-${(j.:.)_dotfiles_ls_colors}}"
unset _dotfiles_ls_colors

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
# high and leaving insert mode feels laggy. 25 (250 ms) keeps Alt/Meta chords and
# arrow keys reliable while staying responsive. Override by exporting
# DOTFILES_KEYTIMEOUT before startup, or by setting KEYTIMEOUT in ~/.zshrc.local
# (sourced last, so it always wins).
KEYTIMEOUT="${DOTFILES_KEYTIMEOUT:-25}"

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
if [[ -n "$_zcompdump"(#qN.mh+24) ]]; then
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

# Ctrl-R fallback for machines WITHOUT fzf: emacs mode defaulted Ctrl-R to
# incremental history search, but viins/vicmd default it to redisplay, so bind
# the search explicitly to preserve that behaviour. When fzf is present its
# `--zsh` block below rebinds Ctrl-R (viins+vicmd) to the fuzzy history picker,
# overriding this.
bindkey -M viins '^R' history-incremental-search-backward
bindkey -M vicmd '^R' history-incremental-search-backward

unset __DOTFILES_FZF_TAB __DOTFILES_AUTOSUGGESTIONS
unfunction _dotfiles_source_first_readable 2>/dev/null || true

