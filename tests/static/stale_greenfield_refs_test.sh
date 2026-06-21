#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

old_ref="chezmoi""-pilot"
hits="$(git grep -n --untracked --fixed-strings "$old_ref" -- . ':!docs/archive/**' || true)"

if [[ -n "$hits" ]]; then
    echo "FAIL: retired greenfield branch ref appears outside archived docs:"
    echo "$hits"
    exit 1
fi

echo "OK"
