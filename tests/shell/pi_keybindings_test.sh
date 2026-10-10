#!/usr/bin/env bash
# Pi owns the newline action explicitly while the terminal/multiplexer stack
# preserves modified Enter. Do not replace semantic Shift+Enter with Ghostty's
# legacy raw-LF remap: Pi already keeps Ctrl+J as the transport fallback.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
KEYBINDINGS="$REPO_ROOT/pi/keybindings.json"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

[[ -f "$KEYBINDINGS" ]] || fail "canonical Pi keybindings file is missing"

python3 - "$KEYBINDINGS" <<'PY'
import json
import pathlib
import sys

actual = json.loads(pathlib.Path(sys.argv[1]).read_text())
expected = {"tui.input.newLine": ["shift+enter", "ctrl+j"]}
if actual != expected:
    raise SystemExit(f"Pi newline keybindings must be exactly {expected!r}; got {actual!r}")
PY

python3 - "$REPO_ROOT/installer/config-targets.json" <<'PYTHON'
import json,pathlib,sys
config=json.loads(pathlib.Path(sys.argv[1]).read_text())
assert any(target.get("source") == "pi/keybindings.json" and target.get("path") == ".pi/agent/keybindings.json" for target in config["targets"])
PYTHON

if grep -F 'keybind = shift+enter=text:\n' "$REPO_ROOT/ghostty/config" >/dev/null; then
    fail "Ghostty raw-LF remap erases Shift+Enter identity; Pi must receive the semantic key or Ctrl+J fallback"
fi

grep -E '^[[:space:]]*set -s extended-keys on([[:space:]]|$)' \
    "$REPO_ROOT/tmux/tmux.conf" >/dev/null ||
    fail "POSIX tmux must forward modified Enter with extended keys"
grep -F "set -as terminal-features ',*:extkeys'" \
    "$REPO_ROOT/tmux/tmux.conf" >/dev/null ||
    fail "POSIX tmux must advertise extended-key transport for outer terminals"

echo "all Pi newline keybinding and modified-Enter transport invariants OK"
