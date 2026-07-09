#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
import pathlib
import sys

failures = []

setup_sh = pathlib.Path("setup.sh").read_text(encoding="utf-8")
setup_ps1 = pathlib.Path("setup.ps1").read_text(encoding="utf-8")
readme = pathlib.Path("README.md").read_text(encoding="utf-8")
claude = pathlib.Path("CLAUDE.md").read_text(encoding="utf-8")

for path, text in (("setup.sh", setup_sh), ("setup.ps1", setup_ps1)):
    if "Remote/piped clone-and-reinvoke setup is disabled" not in text:
        failures.append(f"{path} must explicitly fail closed for remote/piped setup")

for banned in (
    'exec bash "$DEST/setup.sh" "$@"',
    'git clone "$REPO_URL" "$DEST"',
    "git -C \"$DEST\" pull --ff-only",
):
    if banned in setup_sh:
        failures.append(f"setup.sh still contains clone/reinvoke compatibility: {banned}")

for banned in (
    "& (Join-Path $dest 'setup.ps1') @PSBoundParameters",
    "git clone $RepoUrl $dest",
    "git -C $dest pull --ff-only",
):
    if banned in setup_ps1:
        failures.append(f"setup.ps1 still contains clone/reinvoke compatibility: {banned}")

for path, text in (("README.md", readme), ("CLAUDE.md", claude)):
    for banned in (
        "curl -fsSL https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.sh | bash",
        "iwr https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.ps1",
        "setup.sh (remote, dry-run)",
        "setup.ps1 (remote, dry-run)",
    ):
        if banned in text:
            failures.append(f"{path} still documents raw remote setup compatibility: {banned}")

if failures:
    for failure in failures:
        print(f"FAIL: {failure}")
    sys.exit(1)

print("OK: setup entrypoints are local-clone only")
PY
