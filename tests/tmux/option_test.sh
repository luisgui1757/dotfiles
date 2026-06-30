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
# `source-file -q ~/.tmux.posix.conf`, and if the runner already has that
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

# Capture the config-load output. tmux WARNS-but-continues on an unknown option
# (the option checks below still pass), which is exactly how a tmux 3.5+-only
# option like `extended-keys-format` slipped past CI yet broke real tmux 3.4 on
# Ubuntu 24.04. Assert the load is error-free so a future version-incompatible
# option fails here instead of in a user's terminal. `|| true` keeps `set -e`
# from killing us before we can report the captured error.
load_output="$(tmux -L "$sock_name" -f "$REPO_ROOT/tmux/tmux.conf" \
    new-session -d -s "$session_name" 'sleep 30' 2>&1 || true)"
if printf '%s\n' "$load_output" | grep -qiE 'invalid option|unknown option|invalid command|unknown command'; then
    echo "FAIL: tmux.conf produced a config error on $(tmux -V): $load_output"
    exit 1
fi

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
check status-position top
# Status-style is the Rose Pine fallback only; POSIX tmux and Windows psmux load
# pinned upstream Rose Pine theme plugins when setup has installed them. The bar
# stays clock-free because Starship is the single time surface. Bar opacity is a
# Windows Terminal concern (WT `opacity` is window-wide), not a tmux color, so it
# is not asserted here -- the repo ships `opacity: 95` (transparent); set 100 for
# a solid bar.
check status-style "fg=#31748f,bg=#191724"
check status-right ""
check window-status-style "fg=#c4a7e7,bg=#191724"
check window-status-current-style "fg=#f6c177,bold"
# psmux v3.3.4 stores window-status-current-style but does NOT apply it when
# rendering window cells -- only inline `#[fg=...]` in the format survives.
# Real tmux applies either; the inline form works on both, so we pin it.
check window-status-current-format "#[fg=#f6c177,bold] #I:#W#F #[default]"

for required in \
    "set-environment -g TMUX_PLUGIN_MANAGER_PATH \"~/.local/share/dotfiles/tmux-plugins\"" \
    "set -g @plugin 'tmux-plugins/tpm'" \
    "set -g @plugin 'rose-pine/tmux'" \
    "set -g @rose_pine_variant 'main'" \
    "set -g @rose_pine_bar_bg_disable 'on'" \
    "set -g @rose_pine_date_time ''" \
    "if-shell 'test -x \"\$HOME/.local/share/dotfiles/tmux-plugins/tpm/tpm\"'"; do
    if ! grep -F "$required" "$REPO_ROOT/tmux/tmux.posix.conf" >/dev/null; then
        echo "FAIL: tmux.posix.conf missing required plugin line: $required"
        exit 1
    fi
done

for required in \
    "set -g @plugin 'psmux-plugins/ppm'" \
    "set -g @plugin 'psmux-plugins/psmux-theme-rosepine'" \
    "set -g @rosepine-variant 'main'" \
    "set -g @rosepine-show-date-time 'off'" \
    "set -g status-position top" \
    "run '~/.psmux/plugins/psmux-theme-rosepine/psmux-theme-rosepine.ps1'"; do
    if ! grep -F "$required" "$REPO_ROOT/tmux/tmux.windows.conf" >/dev/null; then
        echo "FAIL: tmux.windows.conf missing required plugin line: $required"
        exit 1
    fi
done

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

# Copy-mode `y` baseline. The session above booted with `-f tmux/tmux.conf`;
# its bottom-of-file `source-file -q ~/.tmux.posix.conf` is a no-op under the
# isolated HOME (the overlay is absent). So only the psmux-safe OSC52 baseline applies: `y`
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

# Prove the shared config's tilde overlay source lines actually load an overlay
# from HOME. This catches quoted-tilde regressions that real tmux tolerates less
# strictly than psmux, and prevents the Windows overlay from silently disappearing.
printf '%s\n' 'set -g @dotfiles-test-windows-overlay loaded' > "$HOME/.tmux.windows.conf"
tmux -L "$sock_name" source-file "$REPO_ROOT/tmux/tmux.conf"
if [[ "$(tmux -L "$sock_name" show-options -gv @dotfiles-test-windows-overlay 2>/dev/null)" != "loaded" ]]; then
    echo "FAIL: tmux.conf did not source ~/.tmux.windows.conf from HOME"
    exit 1
fi
rm -f "$HOME/.tmux.windows.conf"
echo "  tmux.conf sources ~/.tmux.windows.conf from HOME"

echo "OK"
