#!/usr/bin/env bash
# Validate every JSON / JSONC file in the repo.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CACHE_DIR="$REPO_ROOT/tests/.cache"
mkdir -p "$CACHE_DIR"
err=$(mktemp "$CACHE_DIR/json-lint.XXXXXX")
trap 'rm -f "$err"' EXIT

strip_jsonc_comments() {
    # Prefer a string-aware Python pass that only strips a // when it is OUTSIDE
    # a quoted string (so URL values like "https://..." and quoted // text
    # survive). Gate on python3 and fall back to the simple sed strip when it is
    # absent, matching the repo's skip-gracefully convention (yaml_lint /
    # toml_lint also gate on python3).
    if ! command -v python3 >/dev/null 2>&1; then
        sed -E 's|//[^"]*$||g' "$1"
        return
    fi
    python3 - "$1" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")

for line in text.splitlines(keepends=True):
    if line.endswith("\r\n"):
        body = line[:-2]
        newline = "\r\n"
    elif line.endswith("\n"):
        body = line[:-1]
        newline = "\n"
    else:
        body = line
        newline = ""

    in_string = False
    escaped = False
    end = len(body)
    for i, ch in enumerate(body):
        if escaped:
            escaped = False
            continue
        if in_string and ch == "\\":
            escaped = True
            continue
        if ch == '"':
            in_string = not in_string
            continue
        if not in_string and ch == "/" and i + 1 < len(body) and body[i + 1] == "/":
            end = i
            break
    sys.stdout.write(body[:end] + newline)
PY
}

# Standard JSON files: lazy-lock.json, .editorconfig-checker.json, etc.
json_files=$(find "$REPO_ROOT" -type f -name "*.json" -not -path "*/.git/*" -not -path "*/tests/.cache/*" -not -path "$REPO_ROOT/home/*" -not -name "*.tmp")
if command -v jq >/dev/null 2>&1; then
    fail=0
    for f in $json_files; do
        if ! jq empty "$f" 2>"$err"; then
            echo "FAIL: $f"; cat "$err"; fail=1
        fi
    done
    [[ "$fail" -ne 0 ]] && exit 1
else
    echo "skipped json: jq not installed"
fi

# JSONC: strip // line comments and run through jq.
jsonc_files=$(find "$REPO_ROOT" -type f -name "*.jsonc" -not -path "*/.git/*" -not -path "*/tests/.cache/*" -not -path "$REPO_ROOT/home/*")
if command -v jq >/dev/null 2>&1; then
    fail=0
    for f in $jsonc_files; do
        if ! strip_jsonc_comments "$f" | jq empty 2>"$err"; then
            echo "FAIL (jsonc): $f"; cat "$err"; fail=1
        fi
    done
    [[ "$fail" -ne 0 ]] && exit 1
fi

echo "OK"
