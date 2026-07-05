#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

HOME="$TMP_HOME"
YES_ALL=1
DRY_RUN=1

[[ "$(tmux_plugin_root)" == "$TMP_HOME/.local/share/dotfiles/tmux-plugins" ]]

# The Rose Pine status bar is a repo-owned generated config, NOT a plugin, so
# rose-pine/tmux is no longer installed. TPM installs only the FUNCTIONAL
# plugins: sensible, yank, resurrect, continuum (the Omer-style set).
out="$(install_tmux_plugins)"
[[ "$out" == *"tmux-plugins/tpm.git"* ]]
[[ "$out" == *"$TPM_COMMIT"* ]]
[[ "$out" == *"tmux-plugins/tmux-sensible.git"* ]]
[[ "$out" == *"$TMUX_SENSIBLE_COMMIT"* ]]
[[ "$out" == *"tmux-plugins/tmux-yank.git"* ]]
[[ "$out" == *"$TMUX_YANK_COMMIT"* ]]
[[ "$out" == *"tmux-plugins/tmux-resurrect.git"* ]]
[[ "$out" == *"$TMUX_RESURRECT_COMMIT"* ]]
[[ "$out" == *"tmux-plugins/tmux-continuum.git"* ]]
[[ "$out" == *"$TMUX_CONTINUUM_COMMIT"* ]]

# rose-pine/tmux must be fully retired from provisioning.
[[ "$out" != *"rose-pine/tmux"* ]] || { echo "FAIL: rose-pine/tmux must not be installed anymore"; exit 1; }

posix_conf="$REPO_ROOT/tmux/tmux.posix.conf"
for required in \
    "set -g @plugin 'tmux-plugins/tpm'" \
    "set -g @plugin 'tmux-plugins/tmux-sensible'" \
    "set -g @plugin 'tmux-plugins/tmux-yank'" \
    "set -g @plugin 'tmux-plugins/tmux-resurrect'" \
    "set -g @plugin 'tmux-plugins/tmux-continuum'" \
    "set -g @continuum-restore 'on'" \
    "set -g @resurrect-strategy-nvim 'session'" \
    "run-shell \"\$HOME/.local/share/dotfiles/tmux-plugins/tpm/tpm\""; do
    grep -F "$required" "$posix_conf" >/dev/null \
        || { echo "FAIL: tmux.posix.conf must declare: $required"; exit 1; }
done

# The bar is the repo-owned generated config, sourced (not a theme plugin).
grep -Eq '^source-file ~/\.tmux\.rose-pine\.main\.conf$' "$posix_conf" \
    || { echo "FAIL: tmux.posix.conf must source the generated Rose Pine main config"; exit 1; }
grep -F "set -g @plugin 'rose-pine/tmux'" "$posix_conf" >/dev/null \
    && { echo "FAIL: tmux.posix.conf must not declare rose-pine/tmux anymore"; exit 1; } || true

# A failed plugin install is a hard provisioning failure: without the functional
# plugins, session save/restore and sane defaults silently vanish. install must
# attempt EVERY declared plugin and fail closed.
(
    set +e
    attempt_log="$TMP_HOME/tmux_attempts.log"; : > "$attempt_log"
    tmux_plugin_ok() { return 1; }
    install_tmux_plugin_repo() { echo "$1" >> "$attempt_log"; return 1; }
    YES_ALL=1
    DRY_RUN=0
    fc_out="$(install_tmux_plugins 2>&1)"; fc_rc=$?
    for name in tpm tmux-sensible tmux-yank tmux-resurrect tmux-continuum; do
        grep -qx "$name" "$attempt_log" \
            || { echo "FAIL: install_tmux_plugins must attempt '$name' even when others fail"; exit 1; }
    done
    [[ "$fc_out" == *"FAIL:"* ]] \
        || { echo "FAIL: install_tmux_plugins must emit a FAIL: marker on plugin failure"; exit 1; }
    [[ "$fc_rc" -ne 0 ]] \
        || { echo "FAIL: install_tmux_plugins must fail closed when plugins are absent"; exit 1; }
)

echo "OK"
