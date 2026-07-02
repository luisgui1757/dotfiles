#!/usr/bin/env bash
# Static guards for the Windows-only psmux overlay and its repo-owned Rose Pine
# renderer (a psmux-safe port of rose-pine/tmux -- see tmux/psmux-rose-pine.ps1).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

WIN_CONF="$REPO_ROOT/tmux/tmux.windows.conf"
HOME_WIN_CONF="$REPO_ROOT/home/dot_tmux.windows.conf"
PSMUX_CONF="$REPO_ROOT/tmux/psmux.conf"
HOME_PSMUX_CONF="$REPO_ROOT/home/dot_psmux.conf"
RENDERER="$REPO_ROOT/tmux/psmux-rose-pine.ps1"
HOME_RENDERER="$REPO_ROOT/home/dot_tmux.rose-pine.ps1"
ROSEPINE_VARIANTS=(main moon dawn)

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

# Rose Pine renderer wiring: default variant + source generated psmux configs.
require_line "^set[[:space:]]+-go[[:space:]]+@rosepine-variant[[:space:]]+'main'$" \
    'tmux.windows.conf must default the Rose Pine variant to main'
require_line '^source-file[[:space:]]+~/\.tmux\.rose-pine\.main\.conf$' \
    'tmux.windows.conf must source the generated main Rose Pine config'
require_line '^source-file[[:space:]]+~/\.tmux\.rose-pine\.moon\.conf$' \
    'tmux.windows.conf must source the generated moon Rose Pine config'
require_line '^source-file[[:space:]]+~/\.tmux\.rose-pine\.dawn\.conf$' \
    'tmux.windows.conf must source the generated dawn Rose Pine config'
require_line '^%if[[:space:]]+"#\{==:#\{@rosepine-variant\},moon\}"$' \
    'tmux.windows.conf must select the moon generated config by @rosepine-variant'
require_line '^%elif[[:space:]]+"#\{==:#\{@rosepine-variant\},dawn\}"$' \
    'tmux.windows.conf must select the dawn generated config by @rosepine-variant'
reject_line "^run[[:space:]]+'~/\.tmux\.rose-pine\.ps1'$" \
    'tmux.windows.conf must not use async run for startup Rose Pine'
reject_line 'set-hook[[:space:]]+-g[[:space:]]+client-attached.*\.tmux\.rose-pine' \
    'tmux.windows.conf must not rely on client-attached to apply startup Rose Pine'

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

# Ordering: variant must be set before the generated config selector, and
# status-position top must be reasserted after the sourced variant.
variant_line="$(grep -nE "^set -go @rosepine-variant 'main'$" "$WIN_CONF" | head -1 | cut -d: -f1)"
source_line="$(awk '/^source-file ~\/\.tmux\.rose-pine\.(main|moon|dawn)\.conf$/ { n = NR } END { print n + 0 }' "$WIN_CONF")"
top_line="$(awk '/^set -g status-position top$/ { n = NR } END { print n + 0 }' "$WIN_CONF")"
if [[ -z "$variant_line" || "$source_line" -eq 0 || "$variant_line" -ge "$source_line" ]]; then
    echo "FAIL: tmux.windows.conf must set @rosepine-variant before sourcing generated Rose Pine configs"
    exit 1
fi
if [[ "$top_line" -le "$source_line" ]]; then
    echo "FAIL: tmux.windows.conf must reassert status-position top after the generated Rose Pine source"
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
if [[ ! -f "$PSMUX_CONF" ]]; then
    echo "FAIL: tmux/psmux.conf psmux entrypoint is missing"
    exit 1
fi
if ! grep -Fx 'set -g warm off' "$PSMUX_CONF" >/dev/null; then
    echo "FAIL: tmux/psmux.conf must disable psmux warm sessions before source-file"
    exit 1
fi
if ! grep -Fx 'source-file ~/.tmux.conf' "$PSMUX_CONF" >/dev/null; then
    echo "FAIL: tmux/psmux.conf must source the shared tmux.conf entrypoint"
    exit 1
fi
if ! grep -Fx 'source-file ~/.tmux.windows.conf' "$PSMUX_CONF" >/dev/null; then
    echo "FAIL: tmux/psmux.conf must source the Windows overlay with psmux flag-free source-file syntax"
    exit 1
fi
if grep -Eq '^source-file[[:space:]]+-q[[:space:]]' "$PSMUX_CONF"; then
    echo "FAIL: tmux/psmux.conf must not rely on source-file -q; psmux v3.3.x treats it differently than tmux"
    exit 1
fi
shared_entry_line="$(grep -nFx 'source-file ~/.tmux.conf' "$PSMUX_CONF" | head -1 | cut -d: -f1)"
windows_entry_line="$(grep -nFx 'source-file ~/.tmux.windows.conf' "$PSMUX_CONF" | head -1 | cut -d: -f1)"
if [[ -z "$shared_entry_line" || -z "$windows_entry_line" || "$shared_entry_line" -ge "$windows_entry_line" ]]; then
    echo "FAIL: tmux/psmux.conf must source ~/.tmux.conf before the explicit Windows overlay"
    exit 1
fi
if [[ ! -f "$HOME_PSMUX_CONF" ]]; then
    echo "FAIL: home/dot_psmux.conf must manage ~/.psmux.conf on Windows"
    exit 1
fi
if ! cmp -s "$PSMUX_CONF" "$HOME_PSMUX_CONF"; then
    echo "FAIL: home/dot_psmux.conf must match tmux/psmux.conf"
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
for variant in "${ROSEPINE_VARIANTS[@]}"; do
    conf="$REPO_ROOT/tmux/psmux-rose-pine.$variant.conf"
    home_conf="$REPO_ROOT/home/dot_tmux.rose-pine.$variant.conf"
    if [[ ! -f "$conf" ]]; then
        echo "FAIL: tmux/psmux-rose-pine.$variant.conf generated config is missing"
        exit 1
    fi
    if [[ ! -f "$home_conf" ]]; then
        echo "FAIL: home/dot_tmux.rose-pine.$variant.conf must manage the generated config on Windows"
        exit 1
    fi
    if ! cmp -s "$conf" "$home_conf"; then
        echo "FAIL: home/dot_tmux.rose-pine.$variant.conf must match tmux/psmux-rose-pine.$variant.conf"
        exit 1
    fi
    if ! grep -Eq "^set -g status-right '.*#\\{b:pane_current_path\\} '$" "$conf"; then
        echo "FAIL: psmux Rose Pine $variant status-right must keep one trailing safety space"
        exit 1
    fi
    if grep -F '#{p2:}' "$conf" >/dev/null; then
        echo "FAIL: psmux Rose Pine $variant generated config must not emit literal #{p2:}; psmux does not expand it in status formats"
        exit 1
    fi
done

if ! grep -Fx '.psmux.conf' "$REPO_ROOT/home/.chezmoiignore" >/dev/null; then
    echo "FAIL: home/.chezmoiignore must ignore .psmux.conf off Windows"
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
for variant in "${ROSEPINE_VARIANTS[@]}"; do
    if ! grep -Fx ".tmux.rose-pine.$variant.conf" "$REPO_ROOT/home/.chezmoiignore" >/dev/null; then
        echo "FAIL: home/.chezmoiignore must ignore .tmux.rose-pine.$variant.conf off Windows"
        exit 1
    fi
done

echo "OK"
