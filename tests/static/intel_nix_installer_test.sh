#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
from pathlib import Path

determinate = "DeterminateSystems/nix-installer-action@ef8a148080ab6020fd15196c2084a2eea5ff2d25"
intel = "cachix/install-nix-action@a49548c11d9846ad46ecc0115273879b045f001c"

for name in (".github/workflows/e2e-install.yml", ".github/workflows/nix.yml"):
    text = Path(name).read_text(encoding="utf-8")
    determinate_step = (
        "if: matrix.logical != 'macos-intel'\n"
        f"        uses: {determinate}"
    )
    intel_step = (
        "if: matrix.logical == 'macos-intel'\n"
        f"        uses: {intel}"
    )
    if text.count(determinate_step) != 1:
        raise SystemExit(f"FAIL: {name} must condition Determinate away from the Intel lane")
    if text.count(intel_step) != 1:
        raise SystemExit(f"FAIL: {name} must select the pinned upstream-Nix action only for Intel")

print("OK: Intel lanes use an explicit full-SHA upstream Nix installer")
PY

if rg -n 'allowDeprecatedx86_64Darwin' flake.nix nix >/dev/null 2>&1; then
    echo "FAIL: do not suppress Nixpkgs' time-bounded Intel support warning" >&2
    exit 1
fi
echo "OK: Intel support sunset remains visible rather than suppressed"
