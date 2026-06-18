#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
import pathlib
import re
import subprocess
import sys

root = pathlib.Path(".")

scan_files = [
    pathlib.Path("install-deps.sh"),
    pathlib.Path("install-deps.ps1"),
    pathlib.Path("setup.sh"),
    pathlib.Path("setup.ps1"),
    pathlib.Path("scripts/validate-renovate.sh"),
    pathlib.Path(".github/workflows/test.yml"),
    pathlib.Path("tests/greenfield/windows-sandbox.wsb"),
    pathlib.Path("tests/greenfield/sandbox-bootstrap.ps1"),
]

allowlist = {
    (
        "install-deps.ps1",
        "Invoke-WebRequest -Uri 'https://get.scoop.sh' -OutFile $installer -UseBasicParsing -ErrorAction Stop",
    ): "Scoop is the Windows package-manager bootstrap trust root; consent-gated and documented.",
    (
        ".github/workflows/test.yml",
        "bash /tmp/cargo-binstall.sh",
    ): "CI verifies the pinned cargo-binstall installer SHA-256 immediately before execution.",
    (
        "tests/greenfield/windows-sandbox.wsb",
        """<Command>powershell.exe -NoExit -NoProfile -ExecutionPolicy Bypass -Command "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; $bootstrap = irm 'https://raw.githubusercontent.com/luisgui1757/dotfiles/main/tests/greenfield/sandbox-bootstrap.ps1'; &amp; ([scriptblock]::Create($bootstrap)) -Ref 'main'"</Command>""",
    ): "Disposable Windows Sandbox self-bootstrap trust root; documented for manual greenfield validation only.",
}

pattern_sources = [
    r"\bcurl\b.*\|\s*(?:/bin/)?(?:ba)?sh\b",
    r"\b(?:/bin/)?(?:ba)?sh\s+-c\s+\"\$\(curl\b",
    r"\b(?:/bin/)?(?:ba)?sh\s+/tmp/[^\"';&|<>]*install[^\"';&|<>]*\.sh\b",
    r"\[scriptblock\]::Create\s*\(",
    r"\bInvoke-RestMethod\b.*\|\s*Invoke-Expression\b",
]
patterns = [re.compile(source, re.IGNORECASE) for source in pattern_sources]


def executable_line(raw_line):
    stripped = raw_line.strip()
    if not stripped or stripped.startswith("#"):
        return False
    if stripped.startswith(("echo ", "printf ", "Write-Host ")):
        return False
    return True


def flag_line(path, line):
    if not executable_line(line):
        return False
    return any(pattern.search(line) for pattern in patterns)


failures = []
seen_allowlist = set()

install_deps_sh = pathlib.Path("install-deps.sh").read_text(encoding="utf-8")
required_install_deps_snippets = [
    'HOMEBREW_INSTALL_COMMIT="',
    'HOMEBREW_INSTALL_SHA256="',
    'url="https://raw.githubusercontent.com/Homebrew/install/${HOMEBREW_INSTALL_COMMIT}/install.sh"',
    'verify_sha256 "$script" "$HOMEBREW_INSTALL_SHA256"',
    'CHEZMOI_LINUX_X86_64_SHA256="',
    'CHEZMOI_LINUX_ARM64_SHA256="',
    'url="https://github.com/twpayne/chezmoi/releases/download/${CHEZMOI_VERSION}/${asset}"',
    'verify_sha256 "$tarball" "$expected"',
    'STARSHIP_VERSION="',
    'STARSHIP_LINUX_X86_64_SHA256="',
    'STARSHIP_LINUX_ARM64_SHA256="',
    'url="https://github.com/starship/starship/releases/download/${STARSHIP_VERSION}/${asset}"',
    'verify_sha256 "$tarball" "$expected"',
]
for snippet in required_install_deps_snippets:
    if snippet not in install_deps_sh:
        failures.append(f"install-deps.sh missing supply-chain guard snippet: {snippet}")

for banned in (
    "Homebrew/install/HEAD/install.sh",
    "sh -c \"$(curl -fsLS get.chezmoi.io)\"",
    "curl -fsSL https://starship.rs/install.sh | sh",
    "curl -fsSL $ubuntu_url | bash",
):
    if banned in install_deps_sh:
        failures.append(f"install-deps.sh contains banned mutable installer pattern: {banned}")

workflow = pathlib.Path(".github/workflows/test.yml").read_text(encoding="utf-8")
required_workflow_snippets = [
    "STARSHIP_VERSION: v",
    "STARSHIP_LINUX_X86_64_SHA256:",
    "starship-x86_64-unknown-linux-gnu.tar.gz",
    "printf '%s  %s\\n' \"$STARSHIP_LINUX_X86_64_SHA256\" /tmp/starship.tar.gz | sha256sum -c -",
    "scripts/install-pinned-chezmoi.sh",
    "CHEZMOI_LINUX_X86_64_SHA256:",
    "CHEZMOI_DARWIN_ARM64_SHA256:",
    "CHEZMOI_WINDOWS_X86_64_SHA256:",
    "TREE_SITTER_CLI_LINUX_VERSION:",
    "TREE_SITTER_CLI_LINUX_X86_64_SHA256:",
    "tree-sitter-cli-linux-x64.zip",
    "sudo install -m 0755 /tmp/tree-sitter-cli/tree-sitter /usr/local/bin/tree-sitter",
    "Get-FileHash -Algorithm SHA256 -LiteralPath $zip",
]
for snippet in required_workflow_snippets:
    if snippet not in workflow:
        failures.append(f".github/workflows/test.yml missing supply-chain guard snippet: {snippet}")

for banned in (
    "starship.rs/install.sh",
    "snap install starship",
    "get.chezmoi.io",
    "scriptblock]::Create($installScript)",
):
    if banned in workflow:
        failures.append(f".github/workflows/test.yml contains banned mutable installer pattern: {banned}")

renovate_validator = pathlib.Path("scripts/validate-renovate.sh").read_text(encoding="utf-8")
for snippet in (
    'RENOVATE_NODE_VERSION="',
    'RENOVATE_VERSION="',
    '"node@$RENOVATE_NODE_VERSION"',
    '"renovate@$RENOVATE_VERSION"',
):
    if snippet not in renovate_validator:
        failures.append(f"scripts/validate-renovate.sh missing pinned validator snippet: {snippet}")
for banned in (
    "renovate@latest",
    "node@latest",
):
    if banned in renovate_validator:
        failures.append(f"scripts/validate-renovate.sh contains mutable validator package: {banned}")

helper = pathlib.Path("scripts/install-pinned-chezmoi.sh")
if not helper.exists():
    failures.append("scripts/install-pinned-chezmoi.sh is missing")
else:
    helper_text = helper.read_text(encoding="utf-8")
    for snippet in (
        "CHEZMOI_LINUX_X86_64_SHA256",
        "CHEZMOI_DARWIN_ARM64_SHA256",
        "sha256sum -c -",
        "shasum -a 256",
        'install -m 0755 "$source_bin" "$bin_dir/chezmoi"',
    ):
        if snippet not in helper_text:
            failures.append(f"scripts/install-pinned-chezmoi.sh missing guard snippet: {snippet}")

setup_doc_files = [
    pathlib.Path("README.md"),
    pathlib.Path("setup.sh"),
    pathlib.Path("setup.ps1"),
]
for path in setup_doc_files:
    text = path.read_text(encoding="utf-8")
    for banned in (
        "curl -fsSL https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.sh | bash",
        "iwr https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.ps1",
    ):
        if banned in text:
            failures.append(f"{path} still recommends raw remote setup execution: {banned}")

for path in scan_files:
    text = path.read_text(encoding="utf-8")
    for lineno, line in enumerate(text.splitlines(), start=1):
        normalized = line.strip()
        key = (path.as_posix(), normalized)
        if key in allowlist:
            seen_allowlist.add(key)

rg_pattern = "|".join(pattern_sources)
rg = subprocess.run(
    [
        "rg",
        "-n",
        "--hidden",
        "--no-heading",
        "-g", "*.sh",
        "-g", "*.ps1",
        "-g", "*.yml",
        "-g", "*.yaml",
        "-g", "*.wsb",
        "-g", "!docs/archive/**",
        "-g", "!tests/.cache/**",
        "-g", "!node_modules/**",
        "-g", "!.git/**",
        "-g", "!tests/static/supply_chain_remote_execution_test.sh",
        rg_pattern,
        ".",
    ],
    text=True,
    capture_output=True,
)
if rg.returncode not in (0, 1):
    failures.append(f"rg scan failed: {rg.stderr.strip()}")
else:
    for row in rg.stdout.splitlines():
        path_text, lineno_text, line = row.split(":", 2)
        path = pathlib.Path(path_text.removeprefix("./"))
        normalized = line.strip()
        key = (path.as_posix(), normalized)
        if key in allowlist:
            seen_allowlist.add(key)
        if not flag_line(path, line):
            continue
        if key not in allowlist:
            failures.append(f"{path}:{lineno_text}: unreviewed remote executable pattern: {normalized}")

for key in sorted(set(allowlist) - seen_allowlist):
    failures.append(f"{key[0]}: allowlist entry no longer matches and should be removed: {key[1]}")

if failures:
    for failure in failures:
        print(f"FAIL: {failure}")
    sys.exit(1)

print("OK: remote executable script patterns are reviewed and allowlisted")
PY
