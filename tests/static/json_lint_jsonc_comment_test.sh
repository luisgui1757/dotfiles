#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="$REPO_ROOT/tests/static/json_lint.sh"
FIXTURE="$REPO_ROOT/tests/static/fixtures/jsonc_url.jsonc"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

function_text="$(awk '
    /^strip_jsonc_comments\(\) \{/ { in_fn = 1 }
    in_fn { print }
    in_fn && /^}/ { exit }
' "$SCRIPT")"

[[ -n "$function_text" ]] || fail "could not extract strip_jsonc_comments"

eval "$function_text"

stripped="$(strip_jsonc_comments "$FIXTURE")"
grep -F '"url": "https://example.com/docs"' <<<"$stripped" >/dev/null \
    || fail "URL value was not preserved"
grep -F '"note": "keep https://example.com/path // inside this string"' <<<"$stripped" >/dev/null \
    || fail "quoted // text was not preserved"
grep -F 'URLs contain' <<<"$stripped" >/dev/null \
    && fail "whole-line JSONC comment was not stripped"
grep -F 'trailing JSONC comment' <<<"$stripped" >/dev/null \
    && fail "trailing JSONC comment was not stripped"

echo "OK"
