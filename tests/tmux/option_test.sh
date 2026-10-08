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

# Isolate user plugins and deploy exactly the managed theme files.
isolated_home="$(mktemp -d)"
export HOME="$isolated_home"

cleanup() {
    tmux -L "$sock_name" kill-server >/dev/null 2>&1 || true
    rm -rf "$isolated_home"
}
trap cleanup EXIT

for variant in main moon dawn; do
    cp "$REPO_ROOT/tmux/rose-pine.$variant.conf" "$HOME/.tmux.rose-pine.$variant.conf"
done

# A migrated machine may retain TPM while the managed plugin attachment is
# absent. Loading the new configuration must never execute that old manager.
mkdir -p "$HOME/.local/share/dotfiles/tmux-plugins/tpm"
cat > "$HOME/.local/share/dotfiles/tmux-plugins/tpm/tpm" <<'SH'
#!/bin/sh
: > "$HOME/legacy-tpm-ran"
SH
chmod +x "$HOME/.local/share/dotfiles/tmux-plugins/tpm/tpm"

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

# Sourcing the POSIX config declares the functional plugins + session options,
# sources the generated Rose Pine bar, and re-binds `y` to the platform's native
# clipboard CLI. On macOS that is pbcopy; assert it there (Linux CI has no single
# guaranteed CLI installed).
posix_conf="$REPO_ROOT/tmux/tmux.conf"
if [[ -e "$HOME/legacy-tpm-ran" ]] || grep -Eq 'TMUX_PLUGIN_MANAGER_PATH|@plugin|tmux-plugins/tpm' "$posix_conf"; then
    echo "FAIL: current tmux configuration executes or references retired TPM"
    exit 1
fi
if ! grep -Fx 'source-file -q ~/.tmux.plugins.conf' "$posix_conf" >/dev/null; then
    echo "FAIL: tmux.conf must load its managed plugin attachment"
    exit 1
fi
# rose-pine/tmux must be fully retired: the bar is a repo-owned generated config.
if grep -F "rose-pine/tmux" "$posix_conf" >/dev/null; then
    echo "FAIL: tmux.conf must not reference rose-pine/tmux anymore"
    exit 1
fi
check @rosepine-variant main
check @continuum-restore on
check @resurrect-strategy-nvim session
check @continuum-save-interval 15
echo "  functional plugins + session save/restore configured"
if [[ "$(uname -s)" == "Darwin" ]]; then
    config_keys="$(tmux -L "$sock_name" list-keys -T copy-mode-vi)"
    if ! printf '%s\n' "$config_keys" | grep -Eq 'copy-mode-vi[[:space:]]+y[[:space:]]+send.*copy-pipe-and-cancel.*pbcopy'; then
        echo "FAIL: tmux.conf config must rebind copy-mode-vi y to pbcopy on macOS"
        exit 1
    fi
    echo "  tmux.conf config rebinds y -> pbcopy on macOS"
fi

# End-to-end: sourcing the config above pulled in the generated Rose Pine bar
# (variant main). Assert it actually applied the Omer-shaped bar (session pill
# left, zoom-aware current window, directory pill right). This is the POSIX proof
# that the generated artifact is not just byte-stable but valid tmux that applies.
check status-style "fg=#908caa,bg=default"
if ! show status-left | grep -q '#26233a'; then
    echo "FAIL: Rose Pine main status-left must render the main config pill"; exit 1
fi
if ! show status-left | grep -q '#S'; then
    echo "FAIL: Rose Pine status-left must render the session pill (#S)"; exit 1
fi
if show status-left | grep -q '#W'; then
    echo "FAIL: Rose Pine status-left must not duplicate window names (#W belongs to the window list)"; exit 1
fi
if ! show window-status-current-format | grep -q 'window_zoomed_flag'; then
    echo "FAIL: current window cell must carry the zoom marker"; exit 1
fi
if ! show window-status-current-format | grep -q '#f6c177'; then
    echo "FAIL: current window number segment must fill gold"; exit 1
fi
if ! show status-right | grep -q 'b:pane_current_path'; then
    echo "FAIL: status-right must render the directory pill (basename)"; exit 1
fi
if [[ "$(show status-right)" != *' ' ]]; then
    echo "FAIL: status-right must keep one trailing safety cell"; exit 1
fi
echo "  generated Rose Pine bar applies (Omer-shaped pill bar)"

# Live variant switch must PERSIST across repeated sourcing. The config uses
# `set -go @rosepine-variant` (only-if-unset), so a user's `tmux set -g
# @rosepine-variant moon` is NOT clobbered back to main every time the config is
# re-sourced. Distinguish variants by the generated pill config color (main
# #26233a vs moon #393552) because the status canvas uses bg=default so terminal
# transparency can show through. Re-sourcing an already-set `-go` option makes
# tmux `source-file` exit nonzero ("already set"), which is expected -- the
# point is that the value is preserved -- so tolerate the nonzero exit with
# `|| true`.
check @rosepine-variant main
check status-style "fg=#908caa,bg=default"
tmux -L "$sock_name" set -g @rosepine-variant moon
tmux -L "$sock_name" source-file "$REPO_ROOT/tmux/tmux.conf" || true
check @rosepine-variant moon
check status-style "fg=#908caa,bg=default"
if ! show status-left | grep -q '#393552'; then
    echo "FAIL: moon variant must repaint the generated pill config"; exit 1
fi
tmux -L "$sock_name" source-file "$REPO_ROOT/tmux/tmux.conf" || true
if [[ "$(show @rosepine-variant)" != "moon" ]]; then
    echo "FAIL: @rosepine-variant snapped back to main on re-source (set -go regressed to set -g)"; exit 1
fi
if [[ "$(show status-left)" != *"#393552"* ]]; then
    echo "FAIL: re-sourcing repainted the main bar; moon must persist"; exit 1
fi
# Restore the default so downstream assertions (if any) see main again.
tmux -L "$sock_name" set -g @rosepine-variant main
echo "  @rosepine-variant persists across repeated source (set -go, not clobbered)"

# Obsolete Windows configuration must never be sourced, even if still present.
printf '%s\n' 'set -g @dotfiles-test-windows-config loaded' > "$HOME/.tmux.windows.conf"
tmux -L "$sock_name" source-file "$REPO_ROOT/tmux/tmux.conf" || true
if tmux -L "$sock_name" show-options -gv @dotfiles-test-windows-config >/dev/null 2>&1; then
    echo "FAIL: obsolete Windows config was sourced"
    exit 1
fi

echo "OK"
