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

# Hermetic HOME so the baseline check below is real: tmux.conf does
# `source-file -q "~/.tmux.posix.conf"`, and if the runner already has that
# overlay deployed in its real HOME, the POSIX probes would rebind copy-mode `y`
# and mask the OSC52 baseline we are asserting. An empty temp HOME guarantees the
# overlay is absent; we source it explicitly later for the pbcopy assertion.
isolated_home="$(mktemp -d)"
export HOME="$isolated_home"

cleanup() {
    tmux -L "$sock_name" kill-server >/dev/null 2>&1 || true
    rm -rf "$isolated_home"
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
# Status-style is pine on base (the teal bar). Inactive windows are intentionally
# UNSET (`setw -gu window-status-style`) so they inherit status-style (teal); the
# current window is the gold-bold standout. Bar opacity is a Windows Terminal
# concern (WT `opacity` is window-wide), not a tmux color, so it is not asserted
# here -- the repo ships `opacity: 95` (transparent), set 100 for a solid bar.
check status-style "fg=#31748f,bg=#191724"
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

# Copy-mode `y` baseline. The session above booted with `-f tmux/tmux.conf`,
# whose `source-file -q "~/.tmux.posix.conf"` is a no-op under the isolated HOME
# (the overlay is absent). So only the psmux-safe OSC52 baseline applies: `y`
# must be bound to a BARE `copy-pipe-and-cancel` with NO pipe command after it.
# The `$` anchor is load-bearing -- a probe rebind appends a pipe argument
# (e.g. `... copy-pipe-and-cancel pbcopy`), and without the anchor this assertion
# would pass even when the overlay had leaked in and masked the baseline.
copy_keys="$(tmux -L "$sock_name" list-keys -T copy-mode-vi)"
if ! printf '%s\n' "$copy_keys" | grep -Eq 'copy-mode-vi[[:space:]]+y[[:space:]]+send(-keys)?[[:space:]]+-X[[:space:]]+copy-pipe-and-cancel[[:space:]]*$'; then
    echo "FAIL: copy-mode-vi y must be a BARE OSC52 copy-pipe-and-cancel (no pipe command)"
    printf '%s\n' "$copy_keys" | grep -E 'copy-mode-vi[[:space:]]+y[[:space:]]' || true
    exit 1
fi
echo "  copy-mode-vi y baseline bound (bare OSC52, no shell probe)"

# Sourcing the POSIX overlay re-binds `y` to the platform's native clipboard CLI.
# On macOS that is pbcopy; assert it there (Linux CI has no single guaranteed
# CLI installed). This proves the extracted if-shell probes still work on POSIX.
tmux -L "$sock_name" source-file "$REPO_ROOT/tmux/tmux.posix.conf"
if [[ "$(uname -s)" == "Darwin" ]]; then
    overlay_keys="$(tmux -L "$sock_name" list-keys -T copy-mode-vi)"
    if ! printf '%s\n' "$overlay_keys" | grep -Eq 'copy-mode-vi[[:space:]]+y[[:space:]]+send.*copy-pipe-and-cancel.*pbcopy'; then
        echo "FAIL: tmux.posix.conf overlay must rebind copy-mode-vi y to pbcopy on macOS"
        exit 1
    fi
    echo "  tmux.posix.conf overlay rebinds y -> pbcopy on macOS"
fi

echo "OK"
