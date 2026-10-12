#!/usr/bin/env bash
# Ask ghostty itself to validate the config when available.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! command -v ghostty >/dev/null 2>&1; then
    echo "skipped: ghostty binary not on PATH (mac local + Linux CI usually skip this)"
    exit 0
fi

# Ghostty reopens stdout on macOS, truncating an inherited regular-file log.
# A pipe isolates that upstream behavior while preserving diagnostics and status.
status=0
output="$(ghostty +validate-config "--config-file=$REPO_ROOT/ghostty/config" 2>&1)" || status=$?
[[ -z "$output" ]] || printf '%s\n' "$output"
[[ "$status" -eq 0 ]] || exit "$status"
echo "OK"
