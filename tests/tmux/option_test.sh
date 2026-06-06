#!/usr/bin/env bash
# Boot a session and check that the options we care about really apply.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! command -v tmux >/dev/null 2>&1; then
    echo "skipped: tmux not installed"
    exit 0
fi

session_name="dotfiles-opt-$$"
sock_name="dotfiles-opt-$$"

cleanup() {
    tmux -L "$sock_name" kill-server >/dev/null 2>&1 || true
}
trap cleanup EXIT

tmux -L "$sock_name" -f "$REPO_ROOT/tmux/tmux.conf" \
    new-session -d -s "$session_name" 'sleep 30'

show() { tmux -L "$sock_name" show-options -gv "$1" 2>&1; }

check() {
    local opt="$1" want="$2"
    local got
    got="$(show "$opt")"
    if [[ "$got" != "$want" ]]; then
        echo "FAIL: $opt = '$got' (want '$want')"
        exit 1
    fi
    echo "  $opt = $got"
}

check focus-events on
check mouse on
check escape-time 10
check history-limit 50000
# Inactive window-status-style is intentionally unset (`setw -gu`); tmux
# reports the unset value as "default", and inactive cells fall back to
# status-style (pine on base) at render time. The only explicit override is
# the current-window cell, which is gold-bold.
check window-status-style "default"
check window-status-current-style "fg=#f6c177,bold"
# psmux v3.3.4 stores window-status-current-style but does NOT apply it when
# rendering window cells -- only inline `#[fg=...]` in the format survives.
# Real tmux applies either; the inline form works on both, so we pin it.
check window-status-current-format "#[fg=#f6c177,bold] #I:#W#F #[default]"

# Prefix isn't shown by show-options; verify via list-keys instead.
if ! tmux -L "$sock_name" list-keys -T prefix >/dev/null 2>&1; then
    echo "FAIL: tmux list-keys failed"; exit 1
fi
prefix=$(tmux -L "$sock_name" display-message -p "#{prefix}")
if [[ "$prefix" != "C-b" ]]; then
    echo "FAIL: prefix = '$prefix' (want 'C-b')"
    exit 1
fi
echo "  prefix = $prefix"

keys="$(tmux -L "$sock_name" list-keys -T prefix)"
if ! printf '%s\n' "$keys" | grep -Eq 'bind-key.*[[:space:]]h[[:space:]]+select-pane[[:space:]]+-L'; then
    echo "FAIL: prefix+h must keep pane-focus-left"
    exit 1
fi
if ! printf '%s\n' "$keys" | grep -Eq 'bind-key.*[[:space:]]l[[:space:]]+select-pane[[:space:]]+-R'; then
    echo "FAIL: prefix+l must keep pane-focus-right"
    exit 1
fi
if ! printf '%s\n' "$keys" | grep -Eq 'bind-key.*[[:space:]]H[[:space:]].*swap-window[[:space:]]+-t[[:space:]]+-1'; then
    echo "FAIL: prefix+H must swap the current window left"
    exit 1
fi
if ! printf '%s\n' "$keys" | grep -Eq 'bind-key.*[[:space:]]L[[:space:]].*swap-window[[:space:]]+-t[[:space:]]+\+1'; then
    echo "FAIL: prefix+L must swap the current window right"
    exit 1
fi

tmux -L "$sock_name" rename-window -t "$session_name:1" one
tmux -L "$sock_name" new-window -d -t "$session_name:" -n two 'sleep 30'
tmux -L "$sock_name" select-window -t "$session_name:2"
tmux -L "$sock_name" swap-window -t -1
order="$(tmux -L "$sock_name" list-windows -F '#{window_index}:#{window_name}' | tr '\n' ' ')"
if [[ "$order" != *"1:two 2:one"* ]]; then
    echo "FAIL: tmux relative swap-window did not move the current window left: $order"
    exit 1
fi

echo "OK"
