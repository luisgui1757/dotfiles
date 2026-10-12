#!/usr/bin/env bash
# Smoke-test the generated interactive zsh profile through its actual loader.
# Pass criteria: exit 0, no stderr output, completes within 5s.
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"

# Start through the generated .zshenv/.zshrc, using its private HOME/ZDOTDIR.
python3 - "$DOTFILES_SHELL_FIXTURES/zsh" <<'PYTHON'
import os
import subprocess
import sys

home = sys.argv[1]
env = {**os.environ, "HOME": home, "ZDOTDIR": home}
result = subprocess.run(["zsh", "-d", "-i", "-c", "print -r -- OK"],
                        env=env, capture_output=True, text=True, timeout=5)
if result.returncode or result.stdout.strip() != "OK" or result.stderr:
    raise SystemExit(f"Generated zsh startup failed: {result.returncode}: {result.stdout!r} {result.stderr!r}")
PYTHON
echo "OK"
