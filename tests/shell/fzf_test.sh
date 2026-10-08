#!/usr/bin/env bash
# Regression guard for fzf shipping by default and being wired into zsh.
# fzf adds interactive fuzzy search (Ctrl-R history, Ctrl-T files, Alt-C cd) —
# complementing zsh-autosuggestions, not replacing it. The runtime side (zshrc
# still starting cleanly WITHOUT fzf) is covered by zsh_startup_test.sh; this
# pins the wiring statically so a future edit can't silently drop it.
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
fail() { echo "FAIL: $1"; exit 1; }

# 2) zshrc wires fzf, guarded by 'command -v' so a missing fzf can't break
#    startup, and prefers the modern `fzf --zsh` integration.
grep -q 'command -v fzf' "$REPO_ROOT/shells/zshrc" \
    || fail "zshrc fzf integration is not guarded by 'command -v fzf'"
grep -q 'fzf --zsh' "$REPO_ROOT/shells/zshrc" \
    || fail "zshrc no longer uses the 'fzf --zsh' integration"

echo "OK"
