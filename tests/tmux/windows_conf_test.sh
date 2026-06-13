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
require_line '^setw[[:space:]]+-g[[:space:]]+window-status-format[[:space:]]+"#\[fg=#c4a7e7\]' \
    'tmux.windows.conf must inline iris for inactive windows (psmux does not render window-status-style)'

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
