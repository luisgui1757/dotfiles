#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
if ! command -v jq >/dev/null 2>&1; then
    echo "skipped json path test: jq not installed"
    exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/tests/.cache" "$tmp/path with spaces"
printf '%s\n' '{"valid": true}' > "$tmp/path with spaces/valid file.json"
printf '%s\n' '{"url": "https://example.invalid/a//b"} // comment' > "$tmp/path with spaces/valid file.jsonc"

DOTFILES_JSON_LINT_ROOT="$tmp" bash "$REPO_ROOT/tests/static/json_lint.sh" >/dev/null

printf '%s\n' '{invalid' > "$tmp/path with spaces/invalid file.json"
set +e
output="$(DOTFILES_JSON_LINT_ROOT="$tmp" bash "$REPO_ROOT/tests/static/json_lint.sh" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || { echo "FAIL: invalid JSON path with spaces passed"; exit 1; }
[[ "$output" == *"path with spaces/invalid file.json"* ]] || {
    echo "FAIL: JSON lint did not preserve the failing path with spaces"
    exit 1
}

echo "OK: JSON and JSONC iteration preserves paths with spaces"
