#!/usr/bin/env bash
# Guard: the WezTerm config must NOT auto-launch a terminal multiplexer.
# Launching tmux/psmux/zellij from a terminal config double-starts sessions and
# fights the multiplexer's own session management. The invariant is simply that
# wezterm.lua (and its byte-identical chezmoi mirror) contain NO multiplexer
# token at all -- so a future `default_prog = {"tmux"}` or a spawn of a
# multiplexer is caught. The header comment deliberately says "multiplexer"
# rather than a tool name so this stays a clean literal-absence check.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fail=0
for f in "$REPO_ROOT/wezterm/wezterm.lua" "$REPO_ROOT/home/dot_config/wezterm/wezterm.lua"; do
    if [[ ! -f "$f" ]]; then
        echo "FAIL: missing $f"
        fail=1
        continue
    fi
    if grep -inE 'tmux|psmux|zellij' "$f" >/dev/null 2>&1; then
        echo "FAIL: $f references a terminal multiplexer (no auto-launch allowed):"
        grep -inE 'tmux|psmux|zellij' "$f" | sed 's/^/  /'
        fail=1
    fi
done

[[ "$fail" -eq 0 ]] && echo "ok  : wezterm.lua does not auto-launch a terminal multiplexer"
exit "$fail"
