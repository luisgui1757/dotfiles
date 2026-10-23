#!/usr/bin/env bash
# A native validator must not erase earlier output from a redirected CI log.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if ! command -v ghostty >/dev/null 2>&1; then
    echo "skipped: Ghostty output preservation requires the native binary"
    exit 0
fi
output="$(mktemp)"
trap 'rm -f "$output"' EXIT
{
    printf 'Earlier CI evidence must survive.\n'
    bash "$SCRIPT_DIR/validate.sh"
} >"$output" 2>&1
python3 - "$output" <<'PY'
import pathlib, sys
data = pathlib.Path(sys.argv[1]).read_bytes()
if not data.startswith(b"Earlier CI evidence must survive.\n") or b"\0" in data:
    raise SystemExit("FAIL: Ghostty validation erased earlier redirected CI output")
if not data.endswith(b"OK\n"):
    raise SystemExit("FAIL: Ghostty validation did not report its result")
PY
echo "OK: native Ghostty validation preserves earlier redirected CI evidence"
