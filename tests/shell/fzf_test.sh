#!/usr/bin/env bash
# Regression guard for fzf shipping by default and being wired into zsh.
# fzf adds interactive fuzzy search (Ctrl-R history, Ctrl-T files, Alt-C cd) —
# complementing zsh-autosuggestions, not replacing it. The runtime side (zshrc
# still starting cleanly WITHOUT fzf) is covered by zsh_startup_test.sh; this
# pins the wiring statically so a future edit can't silently drop it.
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"
fail() { echo "FAIL: $1"; exit 1; }

# Fzf selection uses its declared private command, followed by Tab reclaim.
grep -E 'source <\(.*tool.fzf/current/bin/fzf.* --zsh\)' "$ACTIVE_ZSH" >/dev/null \
    || fail "selected fzf must use its managed --zsh command"
if grep -Eq '^[[:space:]]*source <\(.* --zsh\)' "$ACTIVE_ZSH_CORE"; then
    fail "unselected fzf integration must not initialize from ambient PATH"
fi
echo "OK"
