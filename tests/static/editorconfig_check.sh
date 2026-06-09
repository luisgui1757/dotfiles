#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v editorconfig-checker >/dev/null 2>&1; then
    echo "skipped: editorconfig-checker not installed (brew install editorconfig-checker)"
    exit 0
fi
# Exclude generated/vendored content, local agent state, and managed chezmoi
# copies that are byte-checked against their canonical sources by the migration
# parity gate.
editorconfig-checker --exclude '\\.git/|\\.claude/|home/|nvim/lazy-lock\\.json'
echo "OK"
