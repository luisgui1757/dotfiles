#!/usr/bin/env bash
# Source-policy guard. The real publisher, Git precedence, global writes,
# divergent pulls and removal are exercised by installer/git_settings_test.go.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CONFIG="$REPO_ROOT/git/config"

for key in fetch.prune pull.rebase; do
    if [[ "$(git config --file "$CONFIG" --bool --get "$key")" != "true" ]]; then
        echo "FAIL: managed Git defaults must enable $key" >&2
        exit 1
    fi
done
if git config --file "$CONFIG" --get-regexp '^(user\.|pull\.ff$)' >/dev/null; then
    echo "FAIL: Git defaults must not set personal identity or an ff-only pull policy" >&2
    exit 1
fi

# home/dot_config/git/config remains immutable released migration evidence.
# tests/bootstrap/legacy_inventory_test.py verifies its recorded release hash.
echo "Git rebase and fetch-pruning defaults OK"
