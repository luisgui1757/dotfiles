[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $RepoRoot
$SpecRoot = Join-Path $RepoRoot 'tests\nvim\spec'

if (-not (Get-Command nvim -ErrorAction SilentlyContinue)) {
    Write-Host 'skipped: nvim not installed'
    exit 0
}

$Specs = @(Get-ChildItem -LiteralPath $SpecRoot -Filter '*_spec.lua' -File | Sort-Object FullName)
if ($Specs.Count -eq 0) {
    throw "No Neovim spec files found under $SpecRoot."
}

$Failures = 0
$OriginalNativePreference = $PSNativeCommandUseErrorActionPreference
try {
    # Plenary directory harness exits through :cquit after joining child nvim
    # jobs, which currently false-fails on Windows under PowerShell native
    # command error promotion. Run each spec directly and aggregate real exits.
    $PSNativeCommandUseErrorActionPreference = $false
    foreach ($Spec in $Specs) {
        $SpecPath = $Spec.FullName -replace '\\', '/'
        Write-Host "Running: $SpecPath"
        $global:LASTEXITCODE = 0
        & nvim --headless -u tests/nvim/minimal_init.lua -c "lua require('plenary.busted').run([[$SpecPath]])" 2>&1
        $ExitCode = $LASTEXITCODE
        if ($ExitCode -ne 0) {
            $Failures += 1
            Write-Host "FAIL: $SpecPath exited with code $ExitCode" -ForegroundColor Red
        }
    }
} finally {
    $PSNativeCommandUseErrorActionPreference = $OriginalNativePreference
}

if ($Failures -gt 0) {
    throw "Nvim plenary busted reported $Failures failed spec file(s)."
}

exit 0
