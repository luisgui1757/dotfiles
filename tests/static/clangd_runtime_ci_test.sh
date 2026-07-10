#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORKFLOW="$REPO_ROOT/.github/workflows/test.yml"
SPEC="$REPO_ROOT/tests/nvim/spec/clangd_projects_spec.lua"

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

python3 - "$WORKFLOW" <<'PY' || fail "generic Ubuntu CI does not install the real clangd runtime dependency"
import pathlib
import re
import sys

text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
match = re.search(r"sudo apt-get install -y ([^\n]+)", text)
if not match or "clangd" not in match.group(1).split():
    raise SystemExit(1)
PY

grep -F 'vim.fn.executable("clangd")' "$SPEC" >/dev/null \
    || fail "two-project clangd spec no longer requires a real clangd binary"
grep -F 'assert.are_not.equal(clients[1], clients[2])' "$SPEC" >/dev/null \
    || fail "two-project clangd spec no longer proves distinct clients"

echo "OK: generic Ubuntu CI provisions the real two-project clangd runtime"
