#!/usr/bin/env bash
# Run the plenary busted suite under nvim --headless.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! command -v nvim >/dev/null 2>&1; then
    echo "skipped: nvim not installed"
    exit 0
fi

cd "$REPO_ROOT"
# NOTE: do NOT pass --noplugin. Plenary's plugin/plenary.lua registers
# the :PlenaryBustedDirectory command; --noplugin would silently disable it
# and nvim would hang waiting for an unknown command.
exec nvim --headless \
    -u tests/nvim/minimal_init.lua \
    -c "PlenaryBustedDirectory tests/nvim/spec { minimal_init = 'tests/nvim/minimal_init.lua', sequential = true, timeout = 180000 }"
