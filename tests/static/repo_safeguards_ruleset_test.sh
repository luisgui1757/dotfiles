#!/usr/bin/env bash
# ShellCheck cannot see that the evaled ruleset_id_by_name calls these gh stubs.
# shellcheck disable=SC2034,SC2317,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="$REPO_ROOT/scripts/apply-repo-safeguards.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

function_text="$(awk '
    /^ruleset_id_by_name\(\) \{/ { in_fn = 1 }
    in_fn { print }
    in_fn && /^}/ { exit }
' "$SCRIPT")"

[[ -n "$function_text" ]] || fail "could not extract ruleset_id_by_name"

eval "$function_text"

repo="owner/repo"

gh() {
    [[ "${1:-}" == "api" ]] || fail "unexpected gh command: $*"
    printf '%s\n' 101 202
}

set +e
err="$(ruleset_id_by_name "Protect main: integrity" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || fail "duplicate rulesets were accepted: $err"
grep -F "FAIL: found 2 rulesets named 'Protect main: integrity'" <<<"$err" >/dev/null \
    || fail "duplicate ruleset error was unclear: $err"

gh() {
    [[ "${1:-}" == "api" ]] || fail "unexpected gh command: $*"
    printf '%s\n' 303
}

id="$(ruleset_id_by_name "Protect main: integrity")"
[[ "$id" == "303" ]] || fail "single matching ruleset id was not returned: $id"

echo "OK"
