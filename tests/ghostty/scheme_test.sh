#!/usr/bin/env bash
# Rose Pine consistency invariant: ghostty/config must force the dark Rose Pine
# theme on every platform -- NOT the adaptive dark:/light: split, which would
# flip to the cream Rose Pine Dawn on a light desktop (e.g. fresh GNOME Ubuntu)
# and clash with the otherwise all-dark stack.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! grep -E "^theme\s*=\s*Rose Pine\s*$" "$REPO_ROOT/ghostty/config" >/dev/null; then
    echo "FAIL: ghostty/config must declare 'theme = Rose Pine' (forced dark, not a dark:/light: pair)"
    exit 1
fi
if grep -E "^theme\s*=\s*dark:" "$REPO_ROOT/ghostty/config" >/dev/null; then
    echo "FAIL: ghostty/config must NOT use the adaptive dark:/light: theme split"
    exit 1
fi
if ! grep -E '^scrollback-limit\s*=\s*1073741824\s*$' "$REPO_ROOT/ghostty/config" >/dev/null; then
    echo "FAIL: ghostty/config must retain the shared 1 GiB per-surface scrollback budget"
    exit 1
fi
echo "OK"
