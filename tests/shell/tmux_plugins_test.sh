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

out="$(install_tmux_plugins)"
[[ "$out" == *"tmux-plugins/tpm.git"* ]]
[[ "$out" == *"$TPM_COMMIT"* ]]
[[ "$out" == *"rose-pine/tmux.git"* ]]
[[ "$out" == *"$ROSE_PINE_TMUX_COMMIT"* ]]

# A failing pinned-plugin install must still attempt both plugins, emit a FAIL:
# marker for setup logs, and return 0 so a non-critical theme clone hiccup does
# not abort the rest of setup.
(
    set +e
    attempt_log="$TMP_HOME/tmux_attempts.log"; : > "$attempt_log"
    tmux_plugin_ok() { return 1; }
    install_tmux_plugin_repo() { echo "$1" >> "$attempt_log"; return 1; }
    YES_ALL=1
    DRY_RUN=0
    fc_out="$(install_tmux_plugins 2>&1)"; fc_rc=$?
    { grep -qx 'tpm' "$attempt_log" && grep -qx 'rose-pine/tmux' "$attempt_log"; } \
        || { echo "FAIL: install_tmux_plugins must attempt BOTH plugins when the first fails"; exit 1; }
    [[ "$fc_out" == *"FAIL:"* ]] \
        || { echo "FAIL: install_tmux_plugins must emit a FAIL: marker on plugin failure"; exit 1; }
    [[ "$fc_rc" -eq 0 ]] \
        || { echo "FAIL: install_tmux_plugins must return 0 (continue) on a non-critical plugin failure"; exit 1; }
)

echo "OK"
