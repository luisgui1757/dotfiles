#!/usr/bin/env bash
# Exercise actual tmux bindings with installed helpers and different sessions.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
tmux_binary="$(command -v tmux || true)"
if [[ -z "$tmux_binary" ]]; then
    echo 'skipped: tmux not installed'
    exit 0
fi
fixture="$(mktemp -d)"
sock="dotfiles-clipboard-$$"
cleanup() {
    "$tmux_binary" -L "$sock" kill-server >/dev/null 2>&1 || true
    rm -rf "$fixture"
}
trap cleanup EXIT
mkdir "$fixture/bin" "$fixture/home"
for tool in wl-copy wl-paste xclip xsel; do
    printf '#!/bin/sh\nexit 0\n' > "$fixture/bin/$tool"
    chmod +x "$fixture/bin/$tool"
done

check_session() {
    local name="$1" display="$2" wayland="$3" expected="$4"
    sock="dotfiles-clipboard-$$-$name"
    HOME="$fixture/home" PATH="$fixture/bin" DISPLAY="$display" WAYLAND_DISPLAY="$wayland" \
        "$tmux_binary" -L "$sock" -f "$REPO_ROOT/tmux/tmux.conf" \
        new-session -d -s fixture '/bin/sleep 30'
    local binding
    binding="$("$tmux_binary" -L "$sock" list-keys -T copy-mode-vi | awk '$4 == "y"')"
    "$tmux_binary" -L "$sock" kill-server
    if [[ -z "$binding" ]]; then
        echo "FAIL: $name did not load the configured copy key"
        exit 1
    fi
    if [[ -n "$expected" ]]; then
        if [[ "$binding" != *"$expected"* ]]; then
            echo "FAIL: $name selected the wrong clipboard: $binding"
            exit 1
        fi
    elif [[ "$binding" == *wl-copy* || "$binding" == *xclip* || "$binding" == *xsel* || "$binding" == *win32yank* ]]; then
        echo "FAIL: $name selected a display-dependent helper: $binding"
        exit 1
    fi
    echo "OK: $name clipboard binding"
}

check_session headless '' '' ''
check_session X11 ':fixture' '' xclip
check_session Wayland ':fixture' wayland-fixture wl-copy
rm "$fixture/bin/xclip"
check_session X11-xsel ':fixture' '' xsel
printf '#!/bin/sh\nexit 0\n' > "$fixture/bin/win32yank.exe"
chmod +x "$fixture/bin/win32yank.exe"
check_session existing-bridge '' '' win32yank.exe
printf '#!/bin/sh\nexit 0\n' > "$fixture/bin/pbcopy"
chmod +x "$fixture/bin/pbcopy"
check_session native-clipboard ':fixture' wayland-fixture pbcopy
