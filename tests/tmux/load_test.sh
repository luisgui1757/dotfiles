#!/usr/bin/env bash
# Start a detached tmux session using our conf, then tear it down.
# Any syntax error in tmux.conf causes new-session to fail.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! command -v tmux >/dev/null 2>&1; then
    echo "skipped: tmux not installed"
    exit 0
fi

session_name="dotfiles-test-$$"
tmux_socket_parent="${DOTFILES_TMUX_SOCKET_DIR:-/tmp}"
tmux_socket_dir="$(mktemp -d "$tmux_socket_parent/dotfiles-tmux.XXXXXX")"
tmux_socket="$tmux_socket_dir/socket"

cleanup() {
    tmux -S "$tmux_socket" kill-server >/dev/null 2>&1 || true
    rm -rf "$tmux_socket_dir"
}
trap cleanup EXIT

tmux -S "$tmux_socket" -f "$REPO_ROOT/tmux/tmux.conf" \
    new-session -d -s "$session_name" 'sleep 30'

if ! tmux -S "$tmux_socket" has-session -t "$session_name" 2>/dev/null; then
    echo "FAIL: session did not start"
    exit 1
fi
echo "OK"
