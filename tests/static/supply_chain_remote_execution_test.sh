#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
import pathlib
import re
import sys
import os

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
        ".github/workflows/test.yml",
        "bash /tmp/cargo-binstall.sh",
    ): "CI verifies the pinned cargo-binstall installer SHA-256 immediately before execution.",
}

pattern_sources = [
    r"\bcurl\b.*\|\s*(?:sudo\s+)?(?:/bin/)?(?:ba)?sh\b",
    r"\b(?:/bin/)?(?:ba)?sh\s+-c\s+\"\$\(curl\b",
    r"\b(?:/bin/)?(?:ba)?sh\s+/tmp/[^\"';&|<>]*install[^\"';&|<>]*\.sh\b",
    r"\[scriptblock\]::Create\s*\(",
    r"(^|[;&|=]\s*)Invoke-Expression\b",
    r"(^|[;&|=]\s*)iex(?:\s|\(|$)",
    r"\bInvoke-(?:RestMethod|WebRequest)\b.*\|\s*(?:Invoke-Expression|iex)\b",
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


def unverified_powershell_payload_executions(path, text):
    path = pathlib.Path(path)
    local_failures = []
    download_pattern = re.compile(
        r"Invoke-WebRequest\b[^\n]*-OutFile\s+\$([A-Za-z_][A-Za-z0-9_]*)[^\n]*",
        re.IGNORECASE,
    )
    assignment_prefix = r"(?:\$(?:script:|global:|local:)?[A-Za-z_][A-Za-z0-9_]*\s*=\s*)?"
    for match in download_pattern.finditer(text):
        variable = match.group(1)
        tail = text[match.end():]
        variable_ref = re.escape(variable)
        exec_match = re.search(
            rf"(?m)^\s*{assignment_prefix}(?:&\s+\${variable_ref}\b(?:\s|$)|Start-Process\b[^\n]*-FilePath\s+\${variable_ref}\b)",
            tail,
            re.IGNORECASE,
        )
        if not exec_match:
            continue
        intervening = tail[:exec_match.start()]
        verified = re.search(
            rf"(?:Test-FileSha256\s+\${variable_ref}\b|Test-[A-Za-z0-9]+Signature\b[^\n]*-Path\s+\${variable_ref}\b|Get-AuthenticodeSignature\b[^\n]*-FilePath\s+\${variable_ref}\b)",
            intervening,
            re.IGNORECASE,
        )
        if not verified:
            local_failures.append(
                f"{path}: downloaded PowerShell payload ${variable} is executed without an intervening hash or signature check"
            )
    return local_failures


def unverified_privileged_package_installs(path, text):
    """Require an independent digest between download and privileged install."""
    local_failures = []
    download_pattern = re.compile(
        r"(?m)^\s*(?:if\s+!\s+)?curl\b[^\n]*(?:-o|--output)\s+[\"']?([^\s\"']+)",
        re.IGNORECASE,
    )
    privileged_prefix = re.compile(
        r"\b(?:(?:sudo|maybe_sudo)\s+(?:dpkg\s+-i|rpm\s+(?:-i|--install)|apt(?:-get)?\s+install|installer\s+-pkg)|apt_get_noninteractive\s+install)\b",
        re.IGNORECASE,
    )
    digest_tool = re.compile(r"\b(?:sha256sum|shasum\s+-a\s+256|verify_sha256)\b", re.IGNORECASE)
    for download in download_pattern.finditer(text):
        artifact = download.group(1)
        tail = text[download.end():]
        install = None
        for candidate in privileged_prefix.finditer(tail):
            line_end = tail.find("\n", candidate.start())
            if line_end == -1:
                line_end = len(tail)
            if artifact in tail[candidate.start():line_end]:
                install = candidate
                break
        if install is None:
            continue
        intervening = tail[:install.start()]
        verified = any(
            artifact in line and digest_tool.search(line)
            for line in intervening.splitlines()
        )
        if not verified:
            local_failures.append(
                f"{path}: downloaded package {artifact} reaches privileged installation without an intervening SHA-256 check"
            )
    return local_failures


failures = []
seen_allowlist = set()

for probe in (
    "$r = Invoke-Expression $cmd",
    "iex($x)",
    "curl -fsSL https://example.invalid/install.sh | sudo bash",
):
    if not flag_line(pathlib.Path("probe.sh"), probe):
        failures.append(f"supply-chain scanner failed to catch probe: {probe}")

ps_payload_probes = [
    (
        "direct call",
        "Invoke-WebRequest -Uri $Url -OutFile $installer\n& $installer\n",
        True,
    ),
    (
        "assignment-prefixed direct call",
        "Invoke-WebRequest -Uri $Url -OutFile $installer\n$rc = & $installer -RunAsAdmin\n",
        True,
    ),
    (
        "start-process",
        "Invoke-WebRequest -Uri $Url -OutFile $installer\nStart-Process -FilePath $installer -Wait\n",
        True,
    ),
    (
        "assignment-prefixed start-process",
        "Invoke-WebRequest -Uri $Url -OutFile $installer\n$process = Start-Process -FilePath $installer -Wait -PassThru\n",
        True,
    ),
    (
        "verified assignment-prefixed start-process",
        "Invoke-WebRequest -Uri $Url -OutFile $installer\nTest-VsBuildToolsBootstrapperSignature -Path $installer\n$process = Start-Process -FilePath $installer -Wait -PassThru\n",
        False,
    ),
]
for label, probe, should_fail in ps_payload_probes:
    probe_failures = unverified_powershell_payload_executions(f"probe-{label}.ps1", probe)
    if should_fail and not probe_failures:
        failures.append(f"supply-chain scanner failed to catch PowerShell payload probe: {label}")
    if not should_fail and probe_failures:
        failures.append(f"supply-chain scanner rejected verified PowerShell payload probe: {label}")

privileged_package_probes = [
    (
        "unverified deb",
        "curl -fsSL https://example.invalid/pkg.deb -o /tmp/pkg.deb\nsudo dpkg -i /tmp/pkg.deb\n",
        True,
    ),
    (
        "verified deb",
        "curl -fsSL https://example.invalid/pkg.deb -o /tmp/pkg.deb\nprintf '%s  %s\\n' \"$PIN\" /tmp/pkg.deb | sha256sum -c -\nsudo dpkg -i /tmp/pkg.deb\n",
        False,
    ),
    (
        "unverified helper deb",
        "curl -fsSL https://example.invalid/pkg.deb -o \"$deb\"\nmaybe_sudo apt-get install -y \"$deb\"\n",
        True,
    ),
    (
        "verified helper deb",
        "if ! curl -fsSL https://example.invalid/pkg.deb -o \"$deb\"; then exit 1; fi\nverify_sha256 \"$deb\" \"$PIN\"\nif ! maybe_sudo apt-get install -y \"$deb\"; then exit 1; fi\n",
        False,
    ),
    (
        "unverified noninteractive helper deb",
        "curl -fsSL https://example.invalid/pkg.deb -o \"$deb\"\napt_get_noninteractive install -y \"$deb\"\n",
        True,
    ),
    (
        "verified noninteractive helper deb",
        "if ! curl -fsSL https://example.invalid/pkg.deb -o \"$deb\"; then exit 1; fi\nverify_sha256 \"$deb\" \"$PIN\"\nif ! apt_get_noninteractive install -y \"$deb\"; then exit 1; fi\n",
        False,
    ),
]
for label, probe, should_fail in privileged_package_probes:
    probe_failures = unverified_privileged_package_installs(f"probe-{label}.sh", probe)
    if should_fail and not probe_failures:
        failures.append(f"supply-chain scanner failed to catch privileged package probe: {label}")
    if not should_fail and probe_failures:
        failures.append(f"supply-chain scanner rejected verified privileged package probe: {label}")

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
    'GHOSTTY_UBUNTU_AMD64_2404_SHA256="',
    'GHOSTTY_UBUNTU_ARM64_2404_SHA256="',
    'GHOSTTY_UBUNTU_AMD64_2510_SHA256="',
    'GHOSTTY_UBUNTU_ARM64_2510_SHA256="',
    'GHOSTTY_DEBIAN_AMD64_TRIXIE_SHA256="',
    'GHOSTTY_DEBIAN_ARM64_TRIXIE_SHA256="',
    'GHOSTTY_DEB_URL="https://github.com/mkasberg/ghostty-ubuntu/releases/download/${GHOSTTY_UBUNTU_VERSION}/${GHOSTTY_DEB_ASSET}"',
    'verify_sha256 "$deb" "$expected_sha"',
    'apt_get_noninteractive() {',
    'maybe_sudo env DEBIAN_FRONTEND=noninteractive apt-get "$@"',
    'apt_get_noninteractive install -y "$deb"',
]
for snippet in required_install_deps_snippets:
    if snippet not in install_deps_sh:
        failures.append(f"install-deps.sh missing supply-chain guard snippet: {snippet}")

for banned in (
    "Homebrew/install/HEAD/install.sh",
    "sh -c \"$(curl -fsLS get.chezmoi.io)\"",
    "curl -fsSL https://starship.rs/install.sh | sh",
    "curl -fsSL $ubuntu_url | bash",
    "/releases/latest",
    "ghostty-ubuntu-install.sh",
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
    "POWERSHELL_REPO_DEB_SHA256:",
    'printf \'%s  %s\\n\' "$POWERSHELL_REPO_DEB_SHA256" /tmp/packages-microsoft-prod.deb | sha256sum -c -',
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

cargo_verify = 'printf \'%s  %s\\n\' "$CARGO_BINSTALL_INSTALLER_SHA256" /tmp/cargo-binstall.sh | sha256sum -c -'
cargo_exec = "bash /tmp/cargo-binstall.sh"
cargo_verify_idx = workflow.find(cargo_verify)
cargo_exec_idx = workflow.find(cargo_exec)
if cargo_verify_idx == -1 or cargo_exec_idx == -1 or cargo_verify_idx > cargo_exec_idx:
    failures.append(".github/workflows/test.yml must verify cargo-binstall SHA-256 immediately before executing it")
else:
    between = workflow[cargo_verify_idx + len(cargo_verify):cargo_exec_idx].strip()
    if between:
        failures.append(".github/workflows/test.yml has commands between cargo-binstall SHA-256 verification and execution")

install_deps_ps1 = pathlib.Path("install-deps.ps1").read_text(encoding="utf-8")
required_install_deps_ps1_snippets = [
    "$ScoopInstallerCommit = '",
    "$ScoopInstallerSha256 = '",
    '$ScoopInstallerUrl = "https://raw.githubusercontent.com/ScoopInstaller/Install/$ScoopInstallerCommit/install.ps1"',
    'Invoke-WebRequest -Uri $ScoopInstallerUrl -OutFile $installer -UseBasicParsing -ErrorAction Stop',
    'Test-FileSha256 $installer $ScoopInstallerSha256',
    'verified Scoop installer ScoopInstaller/Install@',
    '& $installer -RunAsAdmin',
    '& $installer',
]
for snippet in required_install_deps_ps1_snippets:
    if snippet not in install_deps_ps1:
        failures.append(f"install-deps.ps1 missing pinned Scoop bootstrap guard snippet: {snippet}")
for banned in (
    "https://get.scoop.sh",
    "Install Scoop via the official one-liner",
):
    if banned in install_deps_ps1:
        failures.append(f"install-deps.ps1 contains banned mutable Scoop bootstrap pattern: {banned}")

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

wt_greenfield = pathlib.Path("tests/greenfield/install-wt-portable.ps1").read_text(encoding="utf-8")
for snippet in (
    ". $installDeps",
    "$WindowsTerminalVersion",
    "$WindowsTerminalX64Sha256",
    "Test-FileSha256 -Path $zip -Expected $WindowsTerminalX64Sha256",
    "[IO.Directory]::Move($stage, $destination)",
):
    if snippet not in wt_greenfield:
        failures.append(f"Windows Sandbox Terminal helper missing production-pin/transaction guard: {snippet}")
for banned in (
    "/releases/latest",
    "Copy-Item -LiteralPath $managed",
):
    if banned in wt_greenfield:
        failures.append(f"Windows Sandbox Terminal helper contains mutable/destructive pattern: {banned}")

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

scan_suffixes = {".bat", ".cmd", ".ps1", ".psm1", ".sh", ".wsb", ".yaml", ".yml"}
excluded_roots = {
    pathlib.Path(".git"),
    pathlib.Path(".claude"),
    pathlib.Path(".codex"),
    pathlib.Path(".pi"),
    pathlib.Path("docs/archive"),
    pathlib.Path("docs/reviews"),
    pathlib.Path("node_modules"),
    pathlib.Path("tests/.cache"),
}
excluded_files = {
    pathlib.Path("tests/static/supply_chain_remote_execution_test.sh"),
    pathlib.Path("tests/static/setup_local_only_test.sh"),
}


def is_excluded(path):
    return path in excluded_files or any(
        path == excluded_root or excluded_root in path.parents
        for excluded_root in excluded_roots
    )


scan_paths = []
excluded_root_names = {path.as_posix() for path in excluded_roots}
for dirpath, dirnames, filenames in os.walk(root):
    current = pathlib.Path(dirpath)
    dirnames[:] = [
        dirname for dirname in dirnames
        if (current / dirname).as_posix() not in excluded_root_names
    ]
    for filename in filenames:
        path = current / filename
        if path.suffix in scan_suffixes:
            scan_paths.append(path)
scan_paths = sorted(scan_paths, key=lambda path: path.as_posix())
for path in scan_paths:
    if is_excluded(path):
        continue
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except UnicodeDecodeError as exc:
        failures.append(f"{path}: could not decode as UTF-8 during remote executable scan: {exc}")
        continue

    for lineno, line in enumerate(lines, start=1):
        normalized = line.strip()
        key = (path.as_posix(), normalized)
        if key in allowlist:
            seen_allowlist.add(key)
        if not flag_line(path, line):
            continue
        if key not in allowlist:
            failures.append(f"{path}:{lineno}: unreviewed remote executable pattern: {normalized}")

    if path.suffix.lower() == ".ps1":
        failures.extend(unverified_powershell_payload_executions(path, "\n".join(lines)))
    failures.extend(unverified_privileged_package_installs(path, "\n".join(lines)))

for key in sorted(set(allowlist) - seen_allowlist):
    failures.append(f"{key[0]}: allowlist entry no longer matches and should be removed: {key[1]}")

if failures:
    for failure in failures:
        print(f"FAIL: {failure}")
    sys.exit(1)

print("OK: remote executable script patterns are reviewed and allowlisted")
PY
