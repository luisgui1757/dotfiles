#!/usr/bin/env bash
# Herdr must consume the same tmux-style full navigator and forced-dark Rose
# Pine theme on POSIX and through Windows' roaming ApplicationData folder.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CONFIG="$REPO_ROOT/herdr/config.toml"
WINDOWS_CONFIG="$REPO_ROOT/herdr/config.windows.toml"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

python3 - "$CONFIG" "$WINDOWS_CONFIG" <<'PY'
import pathlib
import sys
import tomllib

paths = [pathlib.Path(value) for value in sys.argv[1:]]
configs = []
for path in paths:
    with path.open("rb") as handle:
        config = tomllib.load(handle)
    configs.append(config)
    if config.get("onboarding") is not False:
        raise SystemExit(f"Herdr onboarding must be disabled in {path}")
    theme = config.get("theme", {})
    if theme.get("name") != "rose-pine":
        raise SystemExit(f"Herdr theme.name must be rose-pine in {path}")
    if theme.get("auto_switch") is not False:
        raise SystemExit(f"Herdr theme.auto_switch must be false in {path}")
    keys = config.get("keys", {})
    if keys.get("workspace_picker") != "":
        raise SystemExit(f"Herdr workspace-only picker must be disabled in {path}")
    if keys.get("goto") != ["prefix+w", "prefix+g"]:
        raise SystemExit(f"Herdr full navigator must own prefix+w and prefix+g in {path}")
    expected_keys = {
        "rename_tab": "prefix+comma",
        "rename_workspace": "prefix+$",
        "previous_workspace": "prefix+up",
        "next_workspace": "prefix+down",
        "switch_workspace": "prefix+ctrl+alt+1..9",
        "previous_agent": "prefix+shift+a",
        "next_agent": "prefix+a",
        "focus_agent": "prefix+ctrl+1..9",
    }
    for action, binding in expected_keys.items():
        if keys.get(action) != binding:
            raise SystemExit(f"Herdr {action} must be {binding} in {path}")
    ui = config.get("ui", {})
    if ui.get("agent_panel_sort") != "spaces":
        raise SystemExit(f"Herdr agent panel must stay grouped by spaces in {path}")
    if ui.get("show_agent_labels_on_pane_borders") is not False:
        raise SystemExit(f"Herdr pane-border agent labels must stay disabled in {path}")
    agent_sidebar = ui.get("sidebar", {}).get("agents", {})
    agent_rows = agent_sidebar.get("rows")
    expected_agent_rows = [
        ["state_icon", "workspace", "tab"],
        ["state_text", "agent"],
    ]
    if agent_rows != expected_agent_rows:
        raise SystemExit(
            f"Herdr expanded agent rows must preserve the 0.7.3 layout in {path}"
        )
    if agent_sidebar.get("row_gap") != 1:
        raise SystemExit(f"Herdr expanded agent rows must preserve the 0.7.3 spacing in {path}")

if configs[0].get("terminal", {}).get("default_shell") is not None:
    raise SystemExit("POSIX Herdr config must preserve the platform shell default")
if configs[1].get("terminal", {}).get("default_shell") != "pwsh.exe":
    raise SystemExit("Windows Herdr must launch PowerShell 7 through pwsh.exe")
PY

echo "Herdr config, navigation, theme and Windows shell invariants OK"
