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

# Specs must not observe or write personal editor data: a published
# managed runtime under the real stdpath data changes plugin behavior.
$Isolated = Join-Path ([System.IO.Path]::GetTempPath()) ('dotfiles-nvim-spec-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Isolated | Out-Null
$PreviousXdg = @{}
foreach ($Name in 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME') {
    $PreviousXdg[$Name] = [Environment]::GetEnvironmentVariable($Name, 'Process')
    [Environment]::SetEnvironmentVariable($Name, (Join-Path $Isolated $Name), 'Process')
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
    foreach ($Name in $PreviousXdg.Keys) {
        [Environment]::SetEnvironmentVariable($Name, $PreviousXdg[$Name], 'Process')
    }
    Remove-Item -LiteralPath $Isolated -Recurse -Force -ErrorAction SilentlyContinue
}

if ($Failures -gt 0) {
    throw "Nvim plenary busted reported $Failures failed spec file(s)."
}

exit 0
