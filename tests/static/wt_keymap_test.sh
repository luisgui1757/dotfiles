#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
python3 - "$REPO_ROOT/windows-terminal/settings.fragment.jsonc" <<'PY'
import json
from pathlib import Path
import sys

# The reviewed recipe uses full-line comments; installed JSONC is handled by
# the Go scoped-settings parser and its independent lifecycle tests.
source = Path(sys.argv[1]).read_text()
settings = json.loads("\n".join(line for line in source.splitlines() if not line.lstrip().startswith("//")))
bindings = {}
for item in settings["keybindings"]:
    key = "+".join(sorted(item["keys"].lower().split("+")))
    assert key not in bindings, f"Duplicate keybinding: {item['keys']}"
    bindings[key] = item["id"]
actions = {item["id"]: item["command"] for item in settings["actions"]}
assert "ctrl+w" not in bindings, "Ctrl+W must remain available to the shell"
assert bindings["ctrl+shift+w"] == "Dotfiles.CloseTab"
assert actions["Dotfiles.CloseTab"] == "closeTab"
assert bindings["alt+ctrl+w"] == "Terminal.ClosePane"
print("OK: tab/pane bindings preserve shell Ctrl+W through modern action IDs")
PY
