#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
FAIL=0
for t in "$SCRIPT_DIR"/*_test.sh; do
    [[ -f "$t" ]] || continue
    echo "--- ${t##*/} ---"
    if ! bash "$t"; then FAIL=1; fi
done
exit "$FAIL"
