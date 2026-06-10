#!/usr/bin/env bash
# Render the remaining Windows .ps1.tmpl template for targetOS=windows, parse it
# with PowerShell, and exercise the WT modify_ filter. Closes the coverage gap
# where tests/static/ps1_parse.sh only globs *.ps1 (never .ps1.tmpl), so the WT
# template could be syntactically broken while the POSIX parity arm stays green.
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

WT_TMPL="$SRC/AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/modify_settings.json.ps1.tmpl"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# 1) Render the Windows-only WT modify_ template for targetOS=windows. Inject the
#    OS with --override-data (the documented data-override flag) against the real
#    home/ source, so the WT modify_ resolves its `template "windows-terminal/..."`
#    from home/.chezmoitemplates. (A hand-built fixture source proved
#    environment-sensitive in CI; --override-data is the deterministic mechanism.)
render_windows() {
    chezmoi --source "$SRC" execute-template --override-data '{"targetOS":"windows"}' < "$1"
}
render_windows "$WT_TMPL" > "$work/modify-settings.ps1"
[[ -s "$work/modify-settings.ps1" ]] || fail "WT modify template rendered empty for targetOS=windows"
pass "Windows WT modify_ template renders non-empty for targetOS=windows"

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
    "$work/modify-settings.ps1" \
    || fail "rendered Windows .ps1.tmpl failed PowerShell parse"
pass "rendered Windows WT modify_ template parses cleanly under PowerShell"

# 3) WT modify_ filter: a seeded settings.json gains the managed keys AND keeps
#    the user's keys.
cat > "$work/assert-merge.ps1" <<'PS'
$m = [Console]::In.ReadToEnd() | ConvertFrom-Json
if ($m.theme -ne 'rose-pine') { throw 'managed theme not applied' }
if ($m.profiles.defaults.colorScheme -ne 'rose-pine') { throw 'managed colorScheme not applied' }
if (-not ($m.schemes | Where-Object { $_.name -eq 'rose-pine' })) { throw 'rose-pine scheme missing' }
if (-not ($m.themes  | Where-Object { $_.name -eq 'rose-pine' })) { throw 'rose-pine theme missing' }
if (@($m.actions).Count -lt 15) { throw 'managed keybindings missing' }
if ($m.defaultProfile -ne '{u}') { throw 'user defaultProfile dropped' }
if (-not ($m.profiles.list | Where-Object { $_.guid -eq '{u}' })) { throw 'user profile dropped' }
if (-not ($m.schemes | Where-Object { $_.name -eq 'MyScheme' })) { throw 'user scheme dropped' }
if (-not ($m.actions | Where-Object { $_.keys -eq 'alt+f4' })) { throw 'user action dropped' }
if ($m.PSObject.Properties.Name -contains '$schema') { throw 'fragment $schema propagated' }
PS
seeded='{"defaultProfile":"{u}","theme":"legacyLight","profiles":{"defaults":{},"list":[{"guid":"{u}","name":"U"}]},"schemes":[{"name":"MyScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
printf '%s' "$seeded" | pwsh -NoLogo -NoProfile -File "$work/modify-settings.ps1" \
    | pwsh -NoLogo -NoProfile -File "$work/assert-merge.ps1" \
    || fail "WT modify_ merge did not preserve user keys + apply managed keys"
pass "WT modify_ merges managed keys and preserves user data (no \$schema propagated)"

# 4) Empty stdin (WT never launched) must emit NOTHING so chezmoi does not
#    fabricate a settings.json.
empty_out="$(printf '' | pwsh -NoLogo -NoProfile -File "$work/modify-settings.ps1")" \
    || fail "WT modify_ errored on empty stdin"
if [[ -n "${empty_out//[[:space:]]/}" ]]; then
    fail "WT modify_ must emit nothing on empty stdin (WT-never-launched parity); got: $empty_out"
fi
pass "WT modify_ emits nothing on empty stdin (no fabricated settings.json)"
