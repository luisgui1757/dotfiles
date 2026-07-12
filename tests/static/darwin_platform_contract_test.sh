#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
from pathlib import Path

active = {
    "flake.nix": Path("flake.nix").read_text(encoding="utf-8"),
    "setup.sh": Path("setup.sh").read_text(encoding="utf-8"),
    "setup.ps1": Path("setup.ps1").read_text(encoding="utf-8"),
    "POSIX dependencies": Path("install-deps.sh").read_text(encoding="utf-8"),
    "Windows dependencies": Path("install-deps.ps1").read_text(encoding="utf-8"),
    "Nix prerequisite installer": Path("scripts/install-nix-prerequisite.sh").read_text(encoding="utf-8"),
    "release migrator": Path("scripts/upgrade-v0.1.0.sh").read_text(encoding="utf-8"),
    "Windows release migrator": Path("scripts/upgrade-v0.1.0.ps1").read_text(encoding="utf-8"),
    "pinned chezmoi installer": Path("scripts/install-pinned-chezmoi.sh").read_text(encoding="utf-8"),
    "test workflow": Path(".github/workflows/test.yml").read_text(encoding="utf-8"),
    "nix workflow": Path(".github/workflows/nix.yml").read_text(encoding="utf-8"),
    "e2e workflow": Path(".github/workflows/e2e-install.yml").read_text(encoding="utf-8"),
    "README": Path("README.md").read_text(encoding="utf-8"),
    "upgrade guide": Path("docs/UPGRADING.md").read_text(encoding="utf-8"),
    "release notes": Path("docs/releases/v0.2.0.md").read_text(encoding="utf-8"),
    "supply-chain guide": Path("docs/security/supply-chain.md").read_text(encoding="utf-8"),
    "safeguard guide": Path("docs/security/branch-protection.md").read_text(encoding="utf-8"),
}
for path in sorted(Path("nix").rglob("*.nix")):
    active[f"Nix module {path}"] = path.read_text(encoding="utf-8")

for name, text in active.items():
    for retired in (
        "dotfiles-x86_64",
        "x86_64-darwin",
        "macos-26-intel",
        "macos-intel",
        "Darwin:x86_64",
        "CHEZMOI_DARWIN_X86_64_SHA256",
        "Intel macOS",
    ):
        if retired in text:
            raise SystemExit(f"FAIL: {name} still contains removed macOS architecture contract {retired}")

flake = active["flake.nix"]
for supported in ("aarch64-darwin", "aarch64-linux", "x86_64-linux"):
    if supported not in flake:
        raise SystemExit(f"FAIL: flake.nix lost supported system {supported}")

setup = active["setup.sh"]
if "macOS setup requires Apple Silicon (arm64)" not in setup:
    raise SystemExit("FAIL: setup.sh does not enforce the Apple-Silicon-only contract")

if "Darwin:arm64|Darwin:aarch64" not in active["pinned chezmoi installer"]:
    raise SystemExit("FAIL: pinned chezmoi installer lost Apple Silicon support")

for name in ("nix workflow", "e2e workflow"):
    text = active[name]
    if "cachix/install-nix-action" in text:
        raise SystemExit(f"FAIL: {name} retains the removed alternate-architecture Nix bootstrap action")
    if text.count("DeterminateSystems/nix-installer-action@") != 1:
        raise SystemExit(f"FAIL: {name} must use exactly one pinned Determinate Nix action")

if Path(".github/workflows/wsl2-canary.yml").exists():
    raise SystemExit("FAIL: unsupported optional WSL2 hosted canary was not retired")

print("OK: macOS is Apple-Silicon-only with no active x86_64 product path; the unsupported WSL hosted canary is retired")
PY

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cat > "$work/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -s) echo Darwin ;;
    -m) echo x86_64 ;;
    *) echo Darwin ;;
esac
EOF
chmod +x "$work/uname"

if output="$(PATH="$work:/usr/bin:/bin" CHEZMOI_VERSION=v2.71.0 \
    scripts/install-pinned-chezmoi.sh "$work/bin" 2>&1)"; then
    echo "FAIL: pinned chezmoi installer accepted a removed macOS architecture" >&2
    exit 1
fi
[[ "$output" == *"FAIL: unsupported chezmoi release platform: Darwin/x86_64"* ]] || {
    echo "FAIL: pinned chezmoi installer did not fail at the platform boundary" >&2
    printf '%s\n' "$output" >&2
    exit 1
}
[[ ! -e "$work/bin/chezmoi" ]] || {
    echo "FAIL: pinned chezmoi installer published bytes for a removed macOS architecture" >&2
    exit 1
}
echo "OK: pinned chezmoi installer rejects removed macOS architectures before download"
