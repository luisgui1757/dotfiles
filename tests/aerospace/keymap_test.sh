#!/usr/bin/env bash
# AeroSpace config guards (CLAUDE.md reserved-chord rule + start-at-login).
#   - No binding may use a bare alt-h/j/k/l chord: that would globally shadow
#     Neovim's <A-h/j/k/l> window navigation inside any terminal. And no alt-c:
#     that is fzf-tab's / PSFzf's cd chord. Focus/move live on ctrl-alt(-shift).
#   - start-at-login must be true so the WM is always running.
# Checks the canonical config AND its byte-identical chezmoi mirror.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fail=0
for f in "$REPO_ROOT/aerospace/aerospace.toml" "$REPO_ROOT/home/dot_config/aerospace/aerospace.toml"; do
    if [[ ! -f "$f" ]]; then
        echo "FAIL: missing $f"
        fail=1
        continue
    fi
    # Bare alt-h/j/k/l/c binding == a key line that starts with `alt-<h|j|k|l|c> =`
    # (ctrl-alt-*, cmd-alt-*, alt-shift-*, alt-<digit> all correctly do NOT match).
    if grep -nE "^[[:space:]]*alt-[hjklc][[:space:]]*=" "$f" >/dev/null 2>&1; then
        echo "FAIL: $f binds a reserved bare alt-h/j/k/l or alt-c chord:"
        grep -nE "^[[:space:]]*alt-[hjklc][[:space:]]*=" "$f" | sed 's/^/  /'
        fail=1
    fi
    if ! grep -qE "^[[:space:]]*start-at-login[[:space:]]*=[[:space:]]*true" "$f"; then
        echo "FAIL: $f must set start-at-login = true"
        fail=1
    fi
    if ! grep -qE "^[[:space:]]*ctrl-alt-h[[:space:]]*=[[:space:]]*'focus left'" "$f"; then
        echo "FAIL: $f must keep window focus on the ctrl-alt-h/j/k/l scheme"
        fail=1
    fi
    if ! grep -qE "^[[:space:]]*config-version[[:space:]]*=[[:space:]]*2" "$f"; then
        echo "FAIL: $f must declare config-version = 2"
        fail=1
    fi
done

[[ "$fail" -eq 0 ]] && echo "ok  : aerospace avoids reserved chords, focus on ctrl-alt-hjkl, start-at-login = true"
exit "$fail"
