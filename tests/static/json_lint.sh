#!/usr/bin/env bash
# Validate every JSON / JSONC file in the repo.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CACHE_DIR="$REPO_ROOT/tests/.cache"
mkdir -p "$CACHE_DIR"
err=$(mktemp "$CACHE_DIR/json-lint.XXXXXX")
trap 'rm -f "$err"' EXIT

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
        # Strip // line comments before parsing.
        if ! sed -E 's|//[^"]*$||g' "$f" | jq empty 2>"$err"; then
            echo "FAIL (jsonc): $f"; cat "$err"; fail=1
        fi
    done
    [[ "$fail" -ne 0 ]] && exit 1
fi

echo "OK"
