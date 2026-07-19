#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
HOME="$(mktemp -d)"
export HOME

DOTFILES_UNINSTALL_SOURCE_ONLY=1 source "$REPO_ROOT/uninstall.sh"
trap 'rm -f "$DIR_CANDIDATES_FILE"; rm -rf "$HOME"' EXIT
DRY_RUN=0

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# An absent settings file needs no JSON mutation and therefore no Node runtime.
have() {
    fail "dependency probe ran for absent Pi settings: $1"
}
unset_pi_theme_selection || fail "absent Pi settings should be a successful no-op"

# Existing settings still fail closed when the required structured editor is
# unavailable; uninstall must never guess at a JSON rewrite.
mkdir -p "$HOME/.pi/agent"
printf '%s\n' '{"theme":"rose-pine"}' > "$HOME/.pi/agent/settings.json"
have() { return 1; }
if unset_pi_theme_selection >/dev/null 2>&1; then
    fail "existing Pi settings were accepted without Node"
fi

# Direct uninstall after the trial also clears a retired managed alias.
printf '%s\n' '{"theme":"rose-pine-moon-fable","keep":true}' > "$HOME/.pi/agent/settings.json"
have() { command -v "$1" >/dev/null 2>&1; }
unset_pi_theme_selection >/dev/null
python3 - "$HOME/.pi/agent/settings.json" <<'PY'
import json
import pathlib
import sys

assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {"keep": True}
PY

echo "OK"
