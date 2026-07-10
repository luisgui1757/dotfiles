#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
from pathlib import Path

active = {
    "flake.nix": Path("flake.nix").read_text(encoding="utf-8"),
    "setup.sh": Path("setup.sh").read_text(encoding="utf-8"),
    "nix workflow": Path(".github/workflows/nix.yml").read_text(encoding="utf-8"),
    "e2e workflow": Path(".github/workflows/e2e-install.yml").read_text(encoding="utf-8"),
}

for name, text in active.items():
    for retired in ("dotfiles-x86_64", "x86_64-darwin", "macos-26-intel", "macos-intel"):
        if retired in text:
            raise SystemExit(f"FAIL: {name} still contains retired Intel contract {retired}")

flake = active["flake.nix"]
for supported in ("aarch64-darwin", "aarch64-linux", "x86_64-linux"):
    if supported not in flake:
        raise SystemExit(f"FAIL: flake.nix lost supported system {supported}")

setup = active["setup.sh"]
if "Intel macOS support is retired; this repo supports Apple Silicon only." not in setup:
    raise SystemExit("FAIL: setup.sh does not fail closed with the retired Intel contract")

for name in ("nix workflow", "e2e workflow"):
    text = active[name]
    if "cachix/install-nix-action" in text:
        raise SystemExit(f"FAIL: {name} retains the Intel-only Nix bootstrap action")
    if text.count("DeterminateSystems/nix-installer-action@") != 1:
        raise SystemExit(f"FAIL: {name} must use exactly one pinned Determinate Nix action")

if Path(".github/workflows/wsl2-canary.yml").exists():
    raise SystemExit("FAIL: unsupported optional WSL2 hosted canary was not retired")

print("OK: macOS is Apple-Silicon-only and the unsupported WSL hosted canary is retired")
PY
