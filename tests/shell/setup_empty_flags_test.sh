#!/usr/bin/env bash
# Regression guard: setup.sh must not trip nounset on empty flag arrays.
# Bash 3.2 (macOS /bin/bash) errors on "${arr[@]}" when arr is empty and
# `set -u` is on. The canonical idiom is ${arr[@]+"${arr[@]}"}.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

out=$(/bin/bash "$REPO_ROOT/setup.sh" --skip-deps --skip-bootstrap --skip-nvim --skip-agents 2>&1)
rc=$?
if [[ "$rc" -ne 0 ]]; then
    echo "FAIL: setup.sh --skip-* exited $rc"; echo "$out"; exit 1
fi
if grep -qi 'unbound variable' <<<"$out"; then
    echo "FAIL: nounset error in setup.sh on empty flag array"; echo "$out"; exit 1
fi

echo "OK"
