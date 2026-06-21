[CmdletBinding()]
param(
    [string]$MappedRepo = 'C:\Users\WDAGUtilityAccount\Desktop\dotfiles-repo',
    [string]$WorkRepo = '',
    # When the repo is already at $WorkRepo (e.g. sandbox-bootstrap.ps1 downloaded
    # it), skip the mapped-folder copy and use it in place.
    [switch]$SkipCopy
)

$ErrorActionPreference = 'Stop'

function Stop-Greenfield {
    param([string]$Message, [int]$Code = 1)
    Write-Host "FAIL: $Message" -ForegroundColor Red
    exit $Code
}

function Invoke-Robocopy {
    param([string]$Source, [string]$Destination)
    if (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Recurse -Force
    }
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    $global:LASTEXITCODE = 0
    robocopy $Source $Destination /MIR /XD .git /XF .git /R:2 /W:2 /NFL /NDL /NJH /NJS /NP
    $rc = $LASTEXITCODE
    if ($rc -gt 7) {
        Stop-Greenfield "robocopy exited $rc"
    }
}

function Clear-ReadOnlyAttributes {
    param([string]$Path)
    Get-ChildItem -LiteralPath $Path -Recurse -Force | ForEach-Object {
        $_.Attributes = $_.Attributes -band (-bnot [IO.FileAttributes]::ReadOnly)
    }
}

function Test-IsElevated {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Initialize-SandboxHost {
    # Two elevated tweaks in ONE UAC prompt:
    #  1. Developer Mode (AllowDevelopmentWithoutDevLicense) -- REQUIRED so chezmoi
    #     can create the Neovim directory symlink WITHOUT elevating setup.ps1
    #     (Scoop refuses to run elevated, so we cannot just elevate the whole setup).
    #  2. Turn off Defender real-time scanning + exclude the install dirs -- the
    #     single biggest install speedup, since Defender otherwise scans every file
    #     Scoop extracts. Best-effort: if Tamper Protection blocks it, installs are
    #     just slower, not broken.
    $key = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock'
    $name = 'AllowDevelopmentWithoutDevLicense'

    Write-Host "greenfield sandbox: enabling Developer Mode + reducing Defender scan (approve the one UAC prompt)..."
    if (Test-IsElevated) {
        New-Item -Path $key -Force | Out-Null
        New-ItemProperty -Path $key -Name $name -PropertyType DWord -Value 1 -Force | Out-Null
        try { Set-MpPreference -DisableRealtimeMonitoring $true -ErrorAction Stop } catch {}
        try { Add-MpPreference -ExclusionPath "$env:USERPROFILE\scoop", "$env:USERPROFILE\dotfiles", "$env:TEMP", "$env:LOCALAPPDATA" -ErrorAction Stop } catch {}
    } else {
        $elevatedCommands = @"
New-Item -Path '$key' -Force | Out-Null
New-ItemProperty -Path '$key' -Name '$name' -PropertyType DWord -Value 1 -Force | Out-Null
try { Set-MpPreference -DisableRealtimeMonitoring `$true -ErrorAction Stop } catch {}
try { Add-MpPreference -ExclusionPath "`$env:USERPROFILE\scoop","`$env:USERPROFILE\dotfiles","`$env:TEMP","`$env:LOCALAPPDATA" -ErrorAction Stop } catch {}
"@
        $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($elevatedCommands))
        Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -ArgumentList '-NoProfile', '-EncodedCommand', $encoded
    }

    $check = $null
    try { $check = (Get-ItemProperty -Path $key -Name $name -ErrorAction Stop).$name } catch { $check = $null }
    if ($check -ne 1) {
        Stop-Greenfield "could not enable Developer Mode. Enable it manually (Settings -> Privacy & security -> For developers), then re-run: .\setup.ps1 -SkipDeps"
    }
    Write-Host "greenfield sandbox: host prepared (Developer Mode on; Defender real-time scan reduced)"
}

function Invoke-SetupAndCheck {
    param([string]$Repo, [string]$Log)
    Push-Location $Repo
    try {
        $global:LASTEXITCODE = 0
        & .\setup.ps1 -All *>&1 | Tee-Object -FilePath $Log
        $rc = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    if ($rc -ne 0) {
        Stop-Greenfield "setup.ps1 exited $rc; log: $Log"
    }
    $content = Get-Content -Raw -LiteralPath $Log
    if ($content -match 'skipped: Phase 3-5') {
        Stop-Greenfield "setup.ps1 skipped Phase 3-5; log: $Log"
    }
    if ($content -match '(?m)^\s*FAIL:') {
        Stop-Greenfield "setup.ps1 emitted a FAIL marker; log: $Log"
    }
    if (($content -notmatch 'Phase 3/5') -or ($content -notmatch 'Phase 4/5') -or ($content -notmatch 'Phase 5/5')) {
        Stop-Greenfield "setup.ps1 did not run all nvim phases; log: $Log"
    }
}

if (-not $WorkRepo) {
    $WorkRepo = Join-Path $env:USERPROFILE 'dotfiles'
}

$logDir = Join-Path $env:USERPROFILE 'Desktop\dotfiles-greenfield-logs'
New-Item -ItemType Directory -Path $logDir -Force | Out-Null
$setupLog = Join-Path $logDir 'setup-ps1.log'

if ($SkipCopy) {
    if (-not (Test-Path -LiteralPath (Join-Path $WorkRepo 'setup.ps1'))) {
        Stop-Greenfield "repo not found at $WorkRepo (SkipCopy was set)"
    }
    Write-Host "greenfield sandbox: using repo already at $WorkRepo"
} else {
    if (-not (Test-Path -LiteralPath $MappedRepo)) {
        Stop-Greenfield "mapped repo is missing: $MappedRepo"
    }
    Write-Host "greenfield sandbox: copying read-only repo to $WorkRepo"
    Invoke-Robocopy -Source $MappedRepo -Destination $WorkRepo
    Clear-ReadOnlyAttributes -Path $WorkRepo
}

Write-Host "greenfield sandbox: preparing host (Developer Mode + Defender speedup)"
Initialize-SandboxHost

Write-Host "greenfield sandbox: running setup.ps1 -All"
Invoke-SetupAndCheck -Repo $WorkRepo -Log $setupLog

# Windows Terminal package-manager installs are MSIX-backed, and Sandbox cannot
# register MSIX. install-deps.ps1 now falls back to pinned portable WT; keep this
# idempotent helper as a best-effort safety net for the visual checks.
Write-Host "greenfield sandbox: ensuring Windows Terminal (portable build)"
try {
    & (Join-Path $WorkRepo 'tests\greenfield\install-wt-portable.ps1')
} catch {
    Write-Host "greenfield sandbox: portable Windows Terminal install failed (continuing): $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "greenfield sandbox: running shared validator"
& (Join-Path $WorkRepo 'tests\greenfield\validate.ps1') -Repo $WorkRepo
if ($LASTEXITCODE -ne 0) {
    Stop-Greenfield "validate.ps1 failed"
}

Write-Host ""
Write-Host "PASS: Windows Sandbox greenfield install checks passed" -ForegroundColor Green
Write-Host "Next: run the manual visual checklist in tests\greenfield\README.md"
Write-Host "Logs: $logDir"
