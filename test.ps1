[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$RepoRoot = $PSScriptRoot
Set-Location $RepoRoot
$script:Failures = 0
$script:IsCI = ($env:CI -eq 'true')

function Invoke-Step {
    param(
        [Parameter(Mandatory)] [string]$Name,
        [Parameter(Mandatory)] [scriptblock]$Block
    )

    Write-Host "--- $Name ---"
    $global:LASTEXITCODE = 0
    try {
        & $Block
        if ($LASTEXITCODE -ne 0) {
            throw "$Name exited with code $LASTEXITCODE"
        }
    } catch {
        $script:Failures += 1
        Write-Host "FAIL: $Name" -ForegroundColor Red
        Write-Host $_.Exception.Message -ForegroundColor Red
    }
}

function Require-OrSkip {
    param(
        [Parameter(Mandatory)] [string]$Name,
        [Parameter(Mandatory)] [bool]$Present,
        [Parameter(Mandatory)] [string]$InstallHint
    )

    if ($Present) {
        return $true
    }
    if ($script:IsCI) {
        throw "$Name missing in CI. Install step failed or PATH did not refresh."
    }
    Write-Host "skipped: $Name not installed ($InstallHint)"
    return $false
}

Invoke-Step 'PSScriptAnalyzer' {
    if (-not (Require-OrSkip 'PSScriptAnalyzer' ([bool](Get-Module -ListAvailable PSScriptAnalyzer)) 'Install-Module PSScriptAnalyzer')) {
        return
    }
    Import-Module PSScriptAnalyzer -Force
    $diag = @(Invoke-ScriptAnalyzer -Path shells/powershell_profile.ps1 -Severity Warning,Error)
    $diag | Format-Table -AutoSize
    if ($diag.Count -gt 0) {
        throw "PSScriptAnalyzer reported $($diag.Count) warning or error finding(s)."
    }
}

Invoke-Step 'Pester' {
    $pester = Get-Module -ListAvailable Pester |
        Where-Object { $_.Version -ge [version]'5.0.0' } |
        Sort-Object Version -Descending |
        Select-Object -First 1
    if (-not (Require-OrSkip 'Pester >= 5' ([bool]$pester) 'Install-Module Pester -MinimumVersion 5.0.0')) {
        return
    }
    Import-Module Pester -MinimumVersion 5.0.0 -Force
    $result = Invoke-Pester -Path tests/powershell -Output Detailed -PassThru
    if ($result.FailedCount -gt 0) {
        throw "Pester reported $($result.FailedCount) failed test(s)."
    }
}

Invoke-Step 'Nvim plenary busted' {
    if (-not (Require-OrSkip 'nvim' ([bool](Get-Command nvim -ErrorAction SilentlyContinue)) 'install Neovim')) {
        return
    }
    & (Join-Path $RepoRoot 'tests\nvim\run.ps1')
}

exit $script:Failures
