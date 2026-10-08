#!/usr/bin/env bash
# Regression guard for the Meta-prefix bug. `bindkey '\e' kill-whole-line`
# would shadow every Alt-X binding (the terminal sends Alt-X as ESC+X).
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"

if grep -E "^[^#]*bindkey\s+'\\\e'\s+kill-whole-line" "$ACTIVE_ZSH" >/dev/null; then
    echo "FAIL: zshrc still binds bare Esc to kill-whole-line"
    echo "      That shadows the Meta prefix and breaks Alt-X keybindings."
    exit 1
fi
echo "OK"
