#!/usr/bin/env bash
# WezTerm config Lua smoke: load wezterm/wezterm.lua with a stubbed
# require("wezterm") and assert the produced config (see wezterm_smoke.lua).
# Prefers `nvim -l` (a hard repo dependency, present locally and in CI's nvim
# job); falls back to luajit/lua; skips gracefully when no Lua interpreter
# exists, matching the repo's optional-tool convention.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
export REPO_ROOT
SMOKE="$REPO_ROOT/tests/wezterm/wezterm_smoke.lua"

if command -v nvim >/dev/null 2>&1; then
    # -u NONE keeps the run hermetic (no user init.lua/plugins).
    nvim -u NONE -l "$SMOKE"
elif command -v luajit >/dev/null 2>&1; then
    luajit "$SMOKE"
elif command -v lua >/dev/null 2>&1; then
    lua "$SMOKE"
else
    echo "SKIP: no Lua interpreter (nvim/luajit/lua) available for wezterm smoke"
    exit 0
fi
