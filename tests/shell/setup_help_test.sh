#!/usr/bin/env bash
# The remote setup form runs under `bash -s`, where $0 is "bash".
# Help must not try to read usage text from $0.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

out="$(bash -s -- --help < "$REPO_ROOT/setup.sh")"
case "$out" in
    *"setup.sh -- one-shot end-to-end install"* ) ;;
    *) echo "FAIL: setup.sh piped help did not print usage"; echo "$out"; exit 1 ;;
esac
if grep -qi 'No such file' <<<"$out"; then
    echo "FAIL: setup.sh piped help tried to read from bash"; echo "$out"; exit 1
fi
if ! grep -F 'interactive: dependency prompts, then config + sync' <<<"$out" >/dev/null; then
    echo "FAIL: setup.sh help has stale interactive prompt wording"; echo "$out"; exit 1
fi
if grep -F 'interactive: one prompt' <<<"$out" >/dev/null; then
    echo "FAIL: setup.sh help still claims a single interactive prompt"; echo "$out"; exit 1
fi
if ! grep -F './setup.sh --upgrade' <<<"$out" >/dev/null; then
    echo "FAIL: setup.sh help does not advertise the update alias"; echo "$out"; exit 1
fi
if ! grep -F './setup.sh --allow-unreleased' <<<"$out" >/dev/null; then
    echo "FAIL: setup.sh help does not advertise the explicit branch-head test lane"; echo "$out"; exit 1
fi

install_section="$(awk '
    /^## Install, update, and remove$/ { in_section = 1 }
    /^## Cheat sheets$/ { in_section = 0 }
    in_section { print }
' "$REPO_ROOT/README.md")"
if ! grep -F './setup.sh --all --allow-unreleased' <<<"$install_section" >/dev/null; then
    echo "FAIL: README top-level install section hides the unreleased branch test command"
    exit 1
fi

echo "OK"
