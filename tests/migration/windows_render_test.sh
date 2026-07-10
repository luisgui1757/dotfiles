#!/usr/bin/env bash
# Parse and exercise the shared Windows Terminal merge policy without exposing
# a non-transactional chezmoi target. setup.ps1 owns staging/backup/publication;
# this oracle proves the pure merge policy still preserves user data.
# Skips gracefully when pwsh is absent (matches the repo's tool policy); the
# chezmoi-parity CI job runs on ubuntu-latest, which ships pwsh.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SRC="$REPO_ROOT/home"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

command -v chezmoi >/dev/null 2>&1 || fail "chezmoi is required on PATH"
if ! command -v pwsh >/dev/null 2>&1; then
    echo "skipped: pwsh not installed (Windows .ps1.tmpl render/parse/merge coverage)"
    exit 0
fi

WT_HELPER="$SRC/.chezmoitemplates/windows-terminal/merge-settings.ps1"
WT_FRAGMENT="$REPO_ROOT/windows-terminal/settings.fragment.jsonc"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# 1) Build a test-only stdin harness around the production pure merge helper.
cat > "$work/merge-settings.ps1" <<'PS'
param(
    [Parameter(Mandatory)] [string]$Helper,
    [Parameter(Mandatory)] [string]$Fragment
)
$ErrorActionPreference = 'Stop'
. $Helper
$current = ConvertFrom-WindowsTerminalJsonc -Jsonc ([Console]::In.ReadToEnd())
$fragmentObject = ConvertFrom-WindowsTerminalJsonc -Jsonc ([IO.File]::ReadAllText($Fragment))
Merge-WindowsTerminalSettingsObject -Current $current -Fragment $fragmentObject | ConvertTo-Json -Depth 100
PS
run_merge() {
    pwsh -NoLogo -NoProfile -File "$work/merge-settings.ps1" -Helper "$WT_HELPER" -Fragment "$WT_FRAGMENT"
}

# 2) Parse the rendered script with PowerShell (catches .ps1.tmpl breakage).
cat > "$work/parse.ps1" <<'PS'
param([string[]]$Files)
$failed = $false
foreach ($f in $Files) {
    $errors = $null; $tokens = $null
    [System.Management.Automation.Language.Parser]::ParseFile($f, [ref]$tokens, [ref]$errors) | Out-Null
    if ($errors.Count -gt 0) {
        Write-Host "PARSE FAIL: $f"
        $errors | ForEach-Object { Write-Host $_.Message }
        $failed = $true
    }
}
if ($failed) { exit 1 }
PS
pwsh -NoLogo -NoProfile -File "$work/parse.ps1" \
    "$WT_HELPER" "$work/merge-settings.ps1" \
    || fail "Windows Terminal merge helper/harness failed PowerShell parse"
pass "Windows Terminal pure merge helper parses cleanly under PowerShell"

# 3) WT modify_ filter: a seeded settings.json gains the managed keys AND keeps
#    the user's keys.
cat > "$work/assert-merge.ps1" <<'PS'
$managedPwshProfileGuid = '{8a0e8c9b-2b4c-5842-ac1b-29cd17efc89b}'
$m = [Console]::In.ReadToEnd() | ConvertFrom-Json
if ($m.theme -ne 'rose-pine') { throw 'managed theme not applied' }
if ($m.profiles.defaults.colorScheme -ne 'rose-pine') { throw 'managed colorScheme not applied' }
if (-not ($m.schemes | Where-Object { $_.name -eq 'rose-pine' })) { throw 'rose-pine scheme missing' }
if (-not ($m.themes  | Where-Object { $_.name -eq 'rose-pine' })) { throw 'rose-pine theme missing' }
if (@($m.actions).Count -lt 15) { throw 'managed keybindings missing' }
if ($m.defaultProfile -ne '{u}') { throw 'user defaultProfile dropped' }
if (-not ($m.profiles.list | Where-Object { $_.guid -eq '{u}' })) { throw 'user profile dropped' }
if (-not ($m.profiles.list | Where-Object { $_.guid -eq $managedPwshProfileGuid -and $_.commandline -eq 'pwsh.exe' })) { throw 'managed PowerShell 7 profile missing' }
if (-not ($m.schemes | Where-Object { $_.name -eq 'MyScheme' })) { throw 'user scheme dropped' }
if (-not ($m.actions | Where-Object { $_.keys -eq 'alt+f4' })) { throw 'user action dropped' }
if ($m.PSObject.Properties.Name -contains '$schema') { throw 'fragment $schema propagated' }
PS
seeded='{"defaultProfile":"{u}","theme":"legacyLight","profiles":{"defaults":{},"list":[{"guid":"{u}","name":"U"}]},"schemes":[{"name":"MyScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
printf '%s' "$seeded" | run_merge \
    | pwsh -NoLogo -NoProfile -File "$work/assert-merge.ps1" \
    || fail "WT modify_ merge did not preserve user keys + apply managed keys"
pass "WT policy merges managed keys and preserves user data (no \$schema propagated)"

# 4) WT modify_ promotes the built-in Windows PowerShell default to the managed
#    PowerShell 7 profile, without deleting the built-in profile.
cat > "$work/assert-pwsh-default.ps1" <<'PS'
$managedPwshProfileGuid = '{8a0e8c9b-2b4c-5842-ac1b-29cd17efc89b}'
$legacyWindowsPowerShellGuid = '{61c54bbd-c2c6-5271-96e7-009a87ff44bf}'
$m = [Console]::In.ReadToEnd() | ConvertFrom-Json
if ($m.defaultProfile -ne $managedPwshProfileGuid) { throw 'managed PowerShell 7 defaultProfile not applied' }
if (-not ($m.profiles.list | Where-Object { $_.guid -eq $managedPwshProfileGuid -and $_.commandline -eq 'pwsh.exe' })) { throw 'managed PowerShell 7 profile missing' }
if (-not ($m.profiles.list | Where-Object { $_.guid -eq $legacyWindowsPowerShellGuid })) { throw 'legacy Windows PowerShell profile dropped' }
PS
legacy_seed='{"defaultProfile":"{61c54bbd-c2c6-5271-96e7-009a87ff44bf}","profiles":{"defaults":{},"list":[{"guid":"{61c54bbd-c2c6-5271-96e7-009a87ff44bf}","name":"Windows PowerShell","commandline":"powershell.exe"}]},"schemes":[],"actions":[]}'
printf '%s' "$legacy_seed" | run_merge \
    | pwsh -NoLogo -NoProfile -File "$work/assert-pwsh-default.ps1" \
    || fail "WT modify_ did not promote the legacy Windows PowerShell default"
pass "WT policy promotes the legacy default to managed PowerShell 7"

# 5) Chezmoi must not expose a Windows Terminal settings target. setup.ps1 owns
#    both packaged and portable publication transactions.
managed="$(chezmoi --source "$SRC" managed --path-style absolute --override-data '{"targetOS":"windows"}')" \
    || fail "chezmoi could not enumerate Windows targets"
if printf '%s\n' "$managed" | grep -E 'Microsoft\.WindowsTerminal_8wekyb3d8bbwe/LocalState/settings\.json$' >/dev/null; then
    fail "chezmoi still exposes a non-transactional Windows Terminal target"
fi
pass "chezmoi leaves Windows Terminal publication exclusively to setup.ps1"
