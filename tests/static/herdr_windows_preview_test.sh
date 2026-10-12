#!/usr/bin/env bash
# Herdr Windows ships the reviewed official archive and adjacent ConPTY runtime.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fail=0

scan_files=()
while IFS= read -r f; do scan_files+=("$f"); done < <(
    find "$REPO_ROOT" \
        \( -path "$REPO_ROOT/.git" \
        -o -path "$REPO_ROOT/.claude" \
        -o -path "$REPO_ROOT/.codex" \
        -o -path "$REPO_ROOT/.pi" \
        -o -path "$REPO_ROOT/tests/.cache" \
        -o -path "$REPO_ROOT/docs/archive" \) -prune -o \
        -type f \( -name '*.sh' -o -name '*.ps1' -o -name '*.psm1' -o -name '*.cmd' -o -name '*.bat' \) \
        ! -name 'herdr_windows_preview_test.sh' \
        -print | sort
)
herdr_dev_hits=""
if [[ "${#scan_files[@]}" -gt 0 ]]; then
    herdr_dev_hits="$(grep -HniE 'herdr\.dev/install' "${scan_files[@]}" 2>/dev/null |
        grep -vE '^[^:]+:[0-9]+:[[:space:]]*#' || true)"
fi
if [[ -n "$herdr_dev_hits" ]]; then
    echo "FAIL: repo code references the herdr.dev remote-eval installer:"
    printf '%s\n' "$herdr_dev_hits" | sed 's/^/  /'
    fail=1
else
    echo "ok  : no herdr.dev remote-eval installer in repo code"
fi

python3 - "$REPO_ROOT/installer/archive-pins.json" <<'PY'
import json,pathlib,re,sys
pin=json.loads(pathlib.Path(sys.argv[1]).read_text())["resources"]["tool.herdr"]["windows/amd64"]
assert re.fullmatch(r"\d+\.\d+\.\d+",pin["version"])
assert re.fullmatch(r"[0-9a-f]{64}",pin["sha256"])
assert pin["url"].endswith("/herdr-windows-x86_64.zip")
assert pin["format"] == "zip"
assert {"conpty/conpty.dll", "conpty/herdr-conpty.json", "conpty/x64/OpenConsole.exe"} <= set(pin["required_files"])
PY
[[ "$fail" -eq 0 ]] || exit 1
echo "Herdr Windows uses the reviewed private archive"
