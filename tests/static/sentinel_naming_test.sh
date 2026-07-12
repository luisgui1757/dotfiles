#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
retired_name="$(printf '\160\157\154\141\162\151\163')"
failed=0

while IFS= read -r -d '' path; do
    lower_path="$(printf '%s' "$path" | tr '[:upper:]' '[:lower:]')"
    if [[ "$lower_path" == *"$retired_name"* ]]; then
        echo "FAIL: retired agent-policy name remains in path: $path" >&2
        failed=1
    fi

    if [[ -f "$REPO_ROOT/$path" && ! -L "$REPO_ROOT/$path" ]] \
        && LC_ALL=C grep -I -i -q -- "$retired_name" "$REPO_ROOT/$path"; then
        echo "FAIL: retired agent-policy name remains in content: $path" >&2
        failed=1
    fi
done < <(git -C "$REPO_ROOT" ls-files -z)

if [[ "$failed" -ne 0 ]]; then
    exit 1
fi

echo "OK: Sentinel is the sole agent-policy product name in tracked paths and content"
