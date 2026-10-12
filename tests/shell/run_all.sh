#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$SCRIPT_DIR/active_fixture.sh"
FAIL=0
for t in "$SCRIPT_DIR"/*_test.sh "$SCRIPT_DIR"/lint.sh; do
    [[ -f "$t" ]] || continue
    echo "--- ${t##*/} ---"
    if ! bash "$t"; then FAIL=1; fi
done
exit "$FAIL"
