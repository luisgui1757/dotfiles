[CmdletBinding()]
param(
    [string]$MappedRepo = 'C:\Users\WDAGUtilityAccount\Desktop\dotfiles-repo',
    [string]$WorkRepo = ''
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
    if ($content -match 'skipped: Phase 3-4') {
        Stop-Greenfield "setup.ps1 skipped Phase 3-4; log: $Log"
    }
    if ($content -match '(?m)^\s*FAIL:') {
        Stop-Greenfield "setup.ps1 emitted a FAIL marker; log: $Log"
    }
    if (($content -notmatch 'Phase 3/4') -or ($content -notmatch 'Phase 4/4')) {
        Stop-Greenfield "setup.ps1 did not run both nvim phases; log: $Log"
    }
}

if (-not $WorkRepo) {
    $WorkRepo = Join-Path $env:USERPROFILE 'dotfiles'
}

$logDir = Join-Path $env:USERPROFILE 'Desktop\dotfiles-greenfield-logs'
New-Item -ItemType Directory -Path $logDir -Force | Out-Null
$setupLog = Join-Path $logDir 'setup-ps1.log'

if (-not (Test-Path -LiteralPath $MappedRepo)) {
    Stop-Greenfield "mapped repo is missing: $MappedRepo"
}

Write-Host "greenfield sandbox: copying read-only repo to $WorkRepo"
Invoke-Robocopy -Source $MappedRepo -Destination $WorkRepo
Clear-ReadOnlyAttributes -Path $WorkRepo

Write-Host "greenfield sandbox: running setup.ps1 -All"
Invoke-SetupAndCheck -Repo $WorkRepo -Log $setupLog

Write-Host "greenfield sandbox: running shared validator"
& (Join-Path $WorkRepo 'tests\greenfield\validate.ps1') -Repo $WorkRepo
if ($LASTEXITCODE -ne 0) {
    Stop-Greenfield "validate.ps1 failed"
}

Write-Host ""
Write-Host "PASS: Windows Sandbox greenfield install checks passed" -ForegroundColor Green
Write-Host "Next: run the manual visual checklist in tests\greenfield\README.md"
Write-Host "Logs: $logDir"
