#!/usr/bin/env bash
# Static guards for the Windows-only psmux overlay, its repo-owned Rose Pine
# renderer (an Omer/Catppuccin-shaped pill bar -- see tmux/psmux-rose-pine.ps1),
# and the vendored psmux-resurrect plugin it source-files (continuum is blocked
# pending real Windows psmux verification).
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
reject_line '^[[:space:]]*set[[:space:]][^#]*terminal-features' \
    'tmux.windows.conf must not set unsupported tmux terminal-features options under psmux'

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

# Native-Windows yank: at the pinned psmux/psmux-plugins commit there is no active
# top-level psmux-yank port (only a retired _trash/psmux-yank), so clip.exe is the
# deterministic Windows clipboard.
require_line '^bind[[:space:]]+-T[[:space:]]+copy-mode-vi[[:space:]]+y[[:space:]]+send[[:space:]]+-X[[:space:]]+copy-pipe-and-cancel[[:space:]]+"clip\.exe"$' \
    'tmux.windows.conf must keep the clip.exe copy-mode yank (no active psmux-yank port at the pin)'

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

# Vendored psmux-resurrect ONLY, source-filed directly from its pinned checkout
# under ~/.psmux/plugins/. We do NOT use PPM (it clones monorepo HEAD and rewrites
# managed config), and we do NOT declare @plugin lines here.
require_line '^source-file[[:space:]]+~/\.psmux/plugins/psmux-resurrect/plugin\.conf$' \
    'tmux.windows.conf must source the vendored psmux-resurrect plugin.conf'
# psmux-continuum is BLOCKED pending real Windows psmux verification: its
# plugin.conf registers load-time async run-shell hooks that were never validated
# on a Windows host. It must not be shipped in the Windows overlay (POSIX tmux
# still gets tmux-continuum, which is testable on Linux).
reject_line '^source-file[[:space:]]+~/\.psmux/plugins/psmux-continuum/' \
    'tmux.windows.conf must NOT source psmux-continuum (blocked pending Windows verification)'
reject_line '^set[[:space:]]+-g[[:space:]]+@continuum-restore' \
    'tmux.windows.conf must NOT enable continuum auto-restore (blocked pending Windows verification)'
reject_line '^set[[:space:]]+-g[[:space:]]+@continuum-save-interval' \
    'tmux.windows.conf must NOT set a continuum save interval (blocked pending Windows verification)'

# The community powerline plugin must not come back, and PPM must not be used:
# PPM clones the monorepo HEAD (unpinned) and rewrites managed config files.
reject_line "^set[[:space:]]+-g[[:space:]]+@plugin" \
    'tmux.windows.conf must not declare psmux @plugin entries (plugins are vendored + source-filed)'
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
    # status-right: directory basename + one trailing safety cell (a space before
    # the closing quote), no Starship-owned user/host/time context.
    if ! grep -Eq "^set -g status-right '.*#\\{b:pane_current_path\\}.* '\$" "$conf"; then
        echo "FAIL: psmux Rose Pine $variant status-right must render the directory basename with a trailing safety cell"
        exit 1
    fi
    if grep -Eq '#\{(user|host_short)\}' "$conf"; then
        echo "FAIL: psmux Rose Pine $variant status-right must not duplicate Starship user/host context"
        exit 1
    fi
    if grep -F '%a %d %b %H:%M' "$conf" >/dev/null; then
        echo "FAIL: psmux Rose Pine $variant status-right must not duplicate Starship time context"
        exit 1
    fi
    if grep -F '#{p2:}' "$conf" >/dev/null; then
        echo "FAIL: psmux Rose Pine $variant generated config must not emit literal #{p2:}; psmux does not expand it in status formats"
        exit 1
    fi
    if [[ "$variant" == "main" ]] && grep -F '#{?client_prefix,#eb6f92,#c4a7e7}' "$conf" >/dev/null; then
        echo "FAIL: psmux Rose Pine main session pill must use pine, not iris/Catppuccin-purple"
        exit 1
    fi
    if ! grep -Fx "set -g window-status-separator ' '" "$conf" >/dev/null; then
        echo "FAIL: psmux Rose Pine $variant window cells must be standalone pills separated by a single space"
        exit 1
    fi
    # Rounded pill caps must be present; arrow-chevron powerline separators must
    # not (that is the community powerline look we deliberately avoid).
    python3 - "$conf" "$variant" <<'PY'
import sys
data = open(sys.argv[1], encoding="utf-8").read()
variant = sys.argv[2]
cap_left, cap_right = chr(0xE0B6), chr(0xE0B4)
chevrons = [chr(cp) for cp in (0xE0B0, 0xE0B1, 0xE0B2, 0xE0B3, 0xE0B8, 0xE0BA, 0xE0BC, 0xE0BE)]
if cap_left not in data or cap_right not in data:
    sys.exit(f"FAIL: psmux Rose Pine {variant} must use rounded pill caps (U+E0B6/U+E0B4)")
for ch in chevrons:
    if ch in data:
        sys.exit(f"FAIL: psmux Rose Pine {variant} must not use arrow-chevron powerline separators")
PY
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
    echo "FAIL: home/.chezmoiignore must ignore .tmux.rose-pine.ps1 (renderer helper) off Windows"
    exit 1
fi
# The generated variant configs are now CROSS-PLATFORM (POSIX tmux sources them
# too), so they must NOT be ignored off Windows.
for variant in "${ROSEPINE_VARIANTS[@]}"; do
    if grep -Fx ".tmux.rose-pine.$variant.conf" "$REPO_ROOT/home/.chezmoiignore" >/dev/null; then
        echo "FAIL: home/.chezmoiignore must NOT ignore .tmux.rose-pine.$variant.conf (POSIX sources it too)"
        exit 1
    fi
done

echo "OK"
