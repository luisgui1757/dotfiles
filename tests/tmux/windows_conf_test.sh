#!/usr/bin/env bash
# Static guards for the Windows-only psmux overlay.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

WIN_CONF="$REPO_ROOT/tmux/tmux.windows.conf"
HOME_WIN_CONF="$REPO_ROOT/home/dot_tmux.windows.conf"

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
require_line "^set[[:space:]]+-g[[:space:]]+@plugin[[:space:]]+'psmux-plugins/ppm'$" \
    'tmux.windows.conf must declare PPM'
require_line "^set[[:space:]]+-g[[:space:]]+@plugin[[:space:]]+'psmux-plugins/psmux-theme-rosepine'$" \
    'tmux.windows.conf must declare psmux-theme-rosepine'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-variant[[:space:]]+'main'$" \
    'tmux.windows.conf must use Rose Pine main by default'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-powerline[[:space:]]+'on'$" \
    'tmux.windows.conf must enable psmux powerline segments'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-icons[[:space:]]+'on'$" \
    'tmux.windows.conf must enable psmux Nerd Font icons'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-user[[:space:]]+'on'$" \
    'tmux.windows.conf must enable psmux user segment while debugging the full bar'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-zoom[[:space:]]+'on'$" \
    'tmux.windows.conf must enable psmux zoom indicator'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-sync[[:space:]]+'on'$" \
    'tmux.windows.conf must enable psmux sync indicator'
require_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-pane-count[[:space:]]+'on'$" \
    'tmux.windows.conf must enable psmux pane-count indicator'
require_line "^run[[:space:]]+'~/.psmux/plugins/ppm/ppm.ps1'$" \
    'tmux.windows.conf must load PPM from the repo-managed plugin root'
require_line "^run[[:space:]]+'~/.psmux/plugins/psmux-theme-rosepine/psmux-theme-rosepine.ps1'$" \
    'tmux.windows.conf must load the psmux Rose Pine theme entrypoint'
require_line '^set[[:space:]]+-ag[[:space:]]+status-right.*pane_current_path' \
    'tmux.windows.conf must keep a right-side current-directory segment after loading the psmux theme'
reject_line '^bind-key[[:space:]]+-T[[:space:]]+root[[:space:]]+Escape[[:space:]]+send-keys[[:space:]]+esc$' \
    'tmux.windows.conf must not carry the failed psmux Escape pass-through workaround'
reject_line "^set[[:space:]]+-g[[:space:]]+@rosepine-show-date-time[[:space:]]+'off'$" \
    'tmux.windows.conf must not disable the upstream date/time segment while the full psmux bar is being debugged'

theme_run_line="$(grep -n "^run '~/.psmux/plugins/psmux-theme-rosepine/psmux-theme-rosepine.ps1'$" "$WIN_CONF" | tail -1 | cut -d: -f1)"
top_line="$(awk '/^set -g status-position top$/ { n = NR } END { print n + 0 }' "$WIN_CONF")"
if [[ -z "$theme_run_line" || "$top_line" -le "$theme_run_line" ]]; then
    echo "FAIL: tmux.windows.conf must reassert status-position top after psmux-theme-rosepine loads"
    exit 1
fi
if grep -Eq '^set[[:space:]]+-g[[:space:]]+status-right[[:space:]]+""$' "$WIN_CONF"; then
    echo "FAIL: tmux.windows.conf must not blank status-right after the theme; keep the upstream bar plus directory append"
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

if ! grep -Fx '.tmux.windows.conf' "$REPO_ROOT/home/.chezmoiignore" >/dev/null; then
    echo "FAIL: home/.chezmoiignore must ignore .tmux.windows.conf off Windows"
    exit 1
fi

echo "OK"
