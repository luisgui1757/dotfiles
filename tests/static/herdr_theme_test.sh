#!/usr/bin/env bash
# Herdr must consume the same tmux-style full navigator and forced-dark Rose
# Pine theme on POSIX and through Windows' roaming ApplicationData folder.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CONFIG="$REPO_ROOT/herdr/config.toml"
WINDOWS_CONFIG="$REPO_ROOT/herdr/config.windows.toml"
MIRROR="$REPO_ROOT/home/.chezmoitemplates/herdr/config.toml"

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
        "switch_workspace": "prefix+shift+1..9",
        "previous_agent": "prefix+shift+a",
        "next_agent": "prefix+a",
        "focus_agent": "prefix+ctrl+1..9",
    }
    for action, binding in expected_keys.items():
        if keys.get(action) != binding:
            raise SystemExit(f"Herdr {action} must be {binding} in {path}")

if configs[0].get("terminal", {}).get("default_shell") is not None:
    raise SystemExit("POSIX Herdr config must preserve the platform shell default")
if configs[1].get("terminal", {}).get("default_shell") != "pwsh.exe":
    raise SystemExit("Windows Herdr must launch PowerShell 7 through pwsh.exe")
PY

cmp -s "$CONFIG" "$MIRROR" || fail "canonical Herdr config and chezmoi mirror differ"
grep -F '.chezmoitemplates/herdr/config.toml' \
    "$REPO_ROOT/home/dot_config/herdr/symlink_config.toml.tmpl" >/dev/null ||
    fail "POSIX Herdr target does not reference the managed mirror"
grep -F '\herdr\config.windows.toml' \
    "$REPO_ROOT/windows/chezmoi-appdata/herdr/symlink_config.toml.tmpl" >/dev/null ||
    fail "Windows ApplicationData overlay does not reference the Windows Herdr config"

for script in setup.ps1 uninstall.ps1; do
    grep -F "'ApplicationData'" "$REPO_ROOT/$script" >/dev/null ||
        fail "$script does not resolve roaming ApplicationData"
    grep -F 'windows\chezmoi-appdata' "$REPO_ROOT/$script" >/dev/null ||
        fail "$script does not own the Herdr ApplicationData overlay"
    grep -F 'appdata.boltdb' "$REPO_ROOT/$script" >/dev/null ||
        fail "$script does not use the Herdr overlay state boundary"
done

echo "all Herdr tab/workspace/agent navigation, Rose Pine theme, and Windows pwsh invariants OK"
