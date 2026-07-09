#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v editorconfig-checker >/dev/null 2>&1; then
    echo "skipped: editorconfig-checker not installed (brew install editorconfig-checker)"
    exit 0
fi
# Feed editorconfig-checker an explicit pruned file list instead of relying on
# the checker's recursive walker to apply excludes before entering generated
# plugin caches.
while IFS= read -r -d '' file; do
    editorconfig-checker "$file"
done < <(find . \
    \( -path './.git' -o -path './.claude' -o -path './.codex' -o -path './.pi' -o -path './tests/.cache' -o -path './home' \) -prune -o \
    -type f \
    ! -name '.DS_Store' \
    ! -path './nvim/lazy-lock.json' \
    -print0)
echo "OK"
