#!/usr/bin/env bash
# Static guards for the Windows-only psmux overlay and its repo-owned Rose Pine
# renderer (a psmux-safe port of rose-pine/tmux -- see tmux/psmux-rose-pine.ps1).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

WIN_CONF="$REPO_ROOT/tmux/tmux.windows.conf"
HOME_WIN_CONF="$REPO_ROOT/home/dot_tmux.windows.conf"
RENDERER="$REPO_ROOT/tmux/psmux-rose-pine.ps1"
HOME_RENDERER="$REPO_ROOT/home/dot_tmux.rose-pine.ps1"

require_line() {
    local pattern=$1 message=$2
    if ! grep -Eq "$pattern" "$WIN_CONF"; then
        echo "FAIL: $message"
        exit 1
    fi
}

reject_line() {
    local pattern=$1 message=$2
    if grep -Eq "$pattern" "$WIN_CONF"; then
        echo "FAIL: $message"
        exit 1
    fi
}

reject_line '^bind-key[[:space:]]+-n[[:space:]]+C-j[[:space:]]+send-keys[[:space:]]+F8$' \
    'tmux.windows.conf must not translate C-j for lazygit'
reject_line '^bind-key[[:space:]]+-n[[:space:]]+C-k[[:space:]]+send-keys[[:space:]]+F7$' \
    'tmux.windows.conf must not translate C-k for lazygit'
reject_line '^(bind|bind-key)[[:space:]]+H[[:space:]]+' \
    'tmux.windows.conf must not override prefix+H window-swap'
reject_line '^(bind|bind-key)[[:space:]]+L[[:space:]]+' \
    'tmux.windows.conf must not override prefix+L window-swap'
reject_line '^bind-key[[:space:]]+-T[[:space:]]+root[[:space:]]+Escape[[:space:]]+send-keys[[:space:]]+esc$' \
    'tmux.windows.conf must not carry the failed psmux Escape pass-through workaround'

require_line '^set[[:space:]]+-g[[:space:]]+default-shell[[:space:]]+pwsh$' \
    'tmux.windows.conf must set psmux default-shell to pwsh'
require_line '^set[[:space:]]+-g[[:space:]]+allow-predictions[[:space:]]+on$' \
    'tmux.windows.conf must keep psmux predictions enabled'
require_line '^set[[:space:]]+-g[[:space:]]+mouse-selection[[:space:]]+off$' \
    'tmux.windows.conf must leave psmux mouse-selection off'
require_line '^set[[:space:]]+-g[[:space:]]+pwsh-mouse-selection[[:space:]]+off$' \
    'tmux.windows.conf must leave psmux pwsh mouse-selection off'
require_line '^set[[:space:]]+-g[[:space:]]+scroll-enter-copy-mode[[:space:]]+on$' \
    'tmux.windows.conf must keep wheel-scroll copy-mode'

# Rose Pine renderer wiring: default variant + run the repo-owned psmux port.
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-variant[[:space:]]+'main'$" \
    'tmux.windows.conf must default the Rose Pine variant to main'
require_line "^run[[:space:]]+'~/\.tmux\.rose-pine\.ps1'$" \
    'tmux.windows.conf must run the repo-owned psmux Rose Pine renderer'

# The upstream powerline plugin must not come back: it renders a different bar
# (colored segment blocks) that does not match rose-pine/tmux and fights the
# renderer. Guard both the @plugin declarations and the plugin-root run lines.
reject_line "^set[[:space:]]+-g[[:space:]]+@plugin" \
    'tmux.windows.conf must not declare psmux @plugin entries (renderer is repo-owned)'
reject_line "^run[[:space:]]+'~/\.psmux/plugins/" \
    'tmux.windows.conf must not run PPM or a psmux plugin-root script'
# psmux config-load freeze boundary: no command-position if-shell in the overlay.
reject_line '^[[:space:]]*if-shell' \
    'tmux.windows.conf must not use load-time if-shell (psmux ConPTY freeze)'

# Ordering: variant must be set before the renderer reads it, and status-position
# top must be reasserted after the render.
variant_line="$(grep -nE "^set -g @rosepine-variant 'main'$" "$WIN_CONF" | head -1 | cut -d: -f1)"
run_line="$(grep -nE "^run '~/\.tmux\.rose-pine\.ps1'$" "$WIN_CONF" | head -1 | cut -d: -f1)"
top_line="$(awk '/^set -g status-position top$/ { n = NR } END { print n + 0 }' "$WIN_CONF")"
if [[ -z "$variant_line" || -z "$run_line" || "$variant_line" -ge "$run_line" ]]; then
    echo "FAIL: tmux.windows.conf must set @rosepine-variant before running the renderer"
    exit 1
fi
if [[ "$top_line" -le "$run_line" ]]; then
    echo "FAIL: tmux.windows.conf must reassert status-position top after the renderer runs"
    exit 1
fi

if [[ ! -f "$RENDERER" ]]; then
    echo "FAIL: tmux/psmux-rose-pine.ps1 renderer is missing"
    exit 1
fi
if [[ ! -f "$HOME_WIN_CONF" ]]; then
    echo "FAIL: home/dot_tmux.windows.conf must manage the psmux overlay on Windows"
    exit 1
fi
if ! cmp -s "$WIN_CONF" "$HOME_WIN_CONF"; then
    echo "FAIL: home/dot_tmux.windows.conf must match tmux/tmux.windows.conf"
    exit 1
fi
if [[ ! -f "$HOME_RENDERER" ]]; then
    echo "FAIL: home/dot_tmux.rose-pine.ps1 must manage the renderer on Windows"
    exit 1
fi
if ! cmp -s "$RENDERER" "$HOME_RENDERER"; then
    echo "FAIL: home/dot_tmux.rose-pine.ps1 must match tmux/psmux-rose-pine.ps1"
    exit 1
fi

if ! grep -Fx '.tmux.windows.conf' "$REPO_ROOT/home/.chezmoiignore" >/dev/null; then
    echo "FAIL: home/.chezmoiignore must ignore .tmux.windows.conf off Windows"
    exit 1
fi
if ! grep -Fx '.tmux.rose-pine.ps1' "$REPO_ROOT/home/.chezmoiignore" >/dev/null; then
    echo "FAIL: home/.chezmoiignore must ignore .tmux.rose-pine.ps1 off Windows"
    exit 1
fi

echo "OK"
