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

grep -F "set -g @plugin 'tmux-plugins/tpm'" "$REPO_ROOT/tmux/tmux.posix.conf" >/dev/null \
    || { echo "FAIL: tmux.posix.conf must declare tmux-plugins/tpm"; exit 1; }
grep -F "set -g @plugin 'rose-pine/tmux'" "$REPO_ROOT/tmux/tmux.posix.conf" >/dev/null \
    || { echo "FAIL: tmux.posix.conf must declare rose-pine/tmux"; exit 1; }
grep -F "run-shell \"\$HOME/.local/share/dotfiles/tmux-plugins/tpm/tpm\"" "$REPO_ROOT/tmux/tmux.posix.conf" >/dev/null \
    || { echo "FAIL: tmux.posix.conf must run TPM from the repo-managed plugin root"; exit 1; }

# A failed tmux-theme plugin install is a hard provisioning failure: without it,
# the config would silently fall back to a plain tmux status bar and recreate the
# bug this test protects.
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
    [[ "$fc_rc" -ne 0 ]] \
        || { echo "FAIL: install_tmux_plugins must fail closed when the theme plugins are absent"; exit 1; }
)

echo "OK"
