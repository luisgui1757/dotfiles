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
    $analyzerPaths = @(
        'install-deps.ps1',
        'setup.ps1',
        'uninstall.ps1',
        'test.ps1',
        'shells/powershell_profile.ps1',
        'tmux/psmux-rose-pine.ps1',
        'home/.chezmoitemplates/windows-terminal/merge-settings.ps1',
        'home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1',
        'home/dot_tmux.rose-pine.ps1',
        'tests/greenfield/install-wt-portable.ps1',
        'tests/greenfield/sandbox-bootstrap.ps1',
        'tests/greenfield/sandbox-run.ps1',
        'tests/greenfield/validate.ps1',
        'tests/greenfield/wsl-greenfield.ps1',
        'tests/migration/windows_apply_test.ps1',
        'tests/migration/windows_roundtrip_test.ps1',
        'tests/nvim/run.ps1',
        'tests/powershell'
    ) | Where-Object { Test-Path -LiteralPath $_ }
    # Reviewed baseline for broad analyzer coverage. These are existing
    # installer/test-harness style warnings; errors, new rule groups, and count
    # increases still fail the gate.
    $analyzerWarningBaseline = @{
        'dot_tmux.rose-pine.ps1, PSAvoidUsingWriteHost'                               = @{ Count = 1; Reason = 'generated tmux theme status output' }
        'install-deps.ps1, PSAvoidUsingWriteHost'                                     = @{ Count = 161; Reason = 'interactive installer progress output' }
        'install-deps.ps1, PSUseApprovedVerbs'                                        = @{ Count = 5; Reason = 'established installer helper names' }
        'install-deps.ps1, PSUseShouldProcessForStateChangingFunctions'               = @{ Count = 6; Reason = 'installer entry points are explicitly invoked' }
        'install-deps.ps1, PSUseSingularNouns'                                        = @{ Count = 11; Reason = 'established installer helper names' }
        'install-wt-portable.ps1, PSAvoidUsingWriteHost'                              = @{ Count = 5; Reason = 'greenfield harness progress output' }
        'InstallDeps.Tests.ps1, PSAvoidAssignmentToAutomaticVariable'                 = @{ Count = 2; Reason = 'Pester mock scope fixtures' }
        'InstallDeps.Tests.ps1, PSAvoidOverwritingBuiltInCmdlets'                     = @{ Count = 1; Reason = 'Pester mock command fixture' }
        'InstallDeps.Tests.ps1, PSReviewUnusedParameter'                              = @{ Count = 8; Reason = 'Pester mock signatures mirror production calls' }
        'InstallDeps.Tests.ps1, PSUseApprovedVerbs'                                   = @{ Count = 1; Reason = 'Pester helper name' }
        'InstallDeps.Tests.ps1, PSUseDeclaredVarsMoreThanAssignments'                 = @{ Count = 7; Reason = 'Pester assertion captures' }
        'InstallDeps.Tests.ps1, PSUseShouldProcessForStateChangingFunctions'          = @{ Count = 3; Reason = 'Pester helper fixtures' }
        'InstallDeps.Tests.ps1, PSUseSingularNouns'                                   = @{ Count = 4; Reason = 'Pester helper names' }
        'merge-settings.ps1, PSAvoidAssignmentToAutomaticVariable'                    = @{ Count = 2; Reason = 'Windows Terminal merge script fixture variables' }
        'merge-settings.ps1, PSUseShouldProcessForStateChangingFunctions'             = @{ Count = 2; Reason = 'local profile-generation script' }
        'merge-settings.ps1, PSUseSingularNouns'                                      = @{ Count = 2; Reason = 'established template helper names' }
        'Profile.Tests.ps1, PSAvoidUsingWriteHost'                                    = @{ Count = 1; Reason = 'test fixture command output assertion' }
        'psmux-rose-pine.ps1, PSAvoidUsingWriteHost'                                  = @{ Count = 1; Reason = 'generated psmux theme status output' }
        'run.ps1, PSAvoidUsingWriteHost'                                              = @{ Count = 3; Reason = 'nvim test harness progress output' }
        'sandbox-bootstrap.ps1, PSAvoidUsingWriteHost'                                = @{ Count = 2; Reason = 'greenfield harness progress output' }
        'sandbox-run.ps1, PSAvoidUsingWriteHost'                                      = @{ Count = 14; Reason = 'greenfield harness progress output' }
        'sandbox-run.ps1, PSUseShouldProcessForStateChangingFunctions'                = @{ Count = 1; Reason = 'sandbox harness setup helper' }
        'sandbox-run.ps1, PSUseSingularNouns'                                         = @{ Count = 1; Reason = 'sandbox harness helper name' }
        'setup.ps1, PSAvoidUsingWriteHost'                                            = @{ Count = 96; Reason = 'interactive setup progress output' }
        'setup.ps1, PSUseApprovedVerbs'                                               = @{ Count = 1; Reason = 'established setup helper name' }
        'setup.ps1, PSUseShouldProcessForStateChangingFunctions'                      = @{ Count = 6; Reason = 'setup entry points are explicitly invoked' }
        'setup.ps1, PSUseSingularNouns'                                               = @{ Count = 6; Reason = 'established setup helper names' }
        'Setup.Tests.ps1, PSReviewUnusedParameter'                                    = @{ Count = 2; Reason = 'Pester mock signatures mirror production calls' }
        'test.ps1, PSAvoidUsingWriteHost'                                             = @{ Count = 4; Reason = 'test runner progress output' }
        'test.ps1, PSUseApprovedVerbs'                                                = @{ Count = 1; Reason = 'test runner step helper name' }
        'uninstall.ps1, PSAvoidUsingWriteHost'                                        = @{ Count = 18; Reason = 'interactive uninstall progress output' }
        'uninstall.ps1, PSReviewUnusedParameter'                                      = @{ Count = 3; Reason = 'interactive uninstall helper signatures' }
        'uninstall.ps1, PSUseShouldProcessForStateChangingFunctions'                  = @{ Count = 3; Reason = 'uninstall entry points are explicitly invoked' }
        'uninstall.ps1, PSUseSingularNouns'                                           = @{ Count = 5; Reason = 'established uninstall helper names' }
        'validate.ps1, PSAvoidUsingWriteHost'                                         = @{ Count = 6; Reason = 'greenfield validator progress output' }
        'validate.ps1, PSUseShouldProcessForStateChangingFunctions'                   = @{ Count = 1; Reason = 'greenfield validator helper' }
        'validate.ps1, PSUseSingularNouns'                                            = @{ Count = 1; Reason = 'greenfield validator helper name' }
        'windows_apply_test.ps1, PSAvoidAssignmentToAutomaticVariable'                = @{ Count = 3; Reason = 'migration test fixtures' }
        'windows_apply_test.ps1, PSAvoidUsingWriteHost'                               = @{ Count = 3; Reason = 'migration test progress output' }
        'windows_apply_test.ps1, PSUseApprovedVerbs'                                  = @{ Count = 1; Reason = 'migration test helper name' }
        'windows_apply_test.ps1, PSUseShouldProcessForStateChangingFunctions'         = @{ Count = 4; Reason = 'migration test helpers' }
        'windows_apply_test.ps1, PSUseSingularNouns'                                  = @{ Count = 7; Reason = 'migration test helper names' }
        'windows_roundtrip_test.ps1, PSAvoidUsingWriteHost'                           = @{ Count = 3; Reason = 'migration test progress output' }
        'windows_roundtrip_test.ps1, PSUseShouldProcessForStateChangingFunctions'     = @{ Count = 2; Reason = 'migration test helpers' }
        'wsl-greenfield.ps1, PSAvoidUsingWriteHost'                                   = @{ Count = 7; Reason = 'WSL greenfield harness progress output' }
        'wsl-greenfield.ps1, PSReviewUnusedParameter'                                 = @{ Count = 1; Reason = 'WSL harness entrypoint compatibility' }
        'wsl-greenfield.ps1, PSUseShouldProcessForStateChangingFunctions'             = @{ Count = 2; Reason = 'WSL harness setup helpers' }
        'wsl-greenfield.ps1, PSUseSingularNouns'                                      = @{ Count = 2; Reason = 'WSL harness helper names' }
    }
    $diag = @(
        foreach ($path in $analyzerPaths) {
            Invoke-ScriptAnalyzer -Path $path -Recurse -Severity Warning,Error
        }
    )
    $unexpectedDiag = [System.Collections.Generic.List[object]]::new()
    $errorDiag = @($diag | Where-Object { $_.Severity -eq 'Error' })
    foreach ($finding in $errorDiag) {
        [void]$unexpectedDiag.Add($finding)
    }
    $warningGroups = @($diag | Where-Object { $_.Severity -ne 'Error' } | Group-Object ScriptName, RuleName)
    foreach ($group in $warningGroups) {
        $baseline = $analyzerWarningBaseline[$group.Name]
        if ($null -eq $baseline) {
            foreach ($finding in $group.Group) {
                [void]$unexpectedDiag.Add($finding)
            }
            continue
        }
        $allowedCount = [int]$baseline.Count
        if ($group.Count -gt $allowedCount) {
            foreach ($finding in ($group.Group | Select-Object -Skip $allowedCount)) {
                [void]$unexpectedDiag.Add($finding)
            }
        }
    }
    $unexpected = @($unexpectedDiag.ToArray())
    if ($unexpected.Count -gt 0) {
        $unexpected | Format-Table -AutoSize
        throw "PSScriptAnalyzer reported $($unexpected.Count) unexpected warning or error finding(s)."
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
    if ($result.TotalCount -lt 1) {
        throw "Pester discovered zero tests."
    }
    if ([string]$result.Result -ne 'Passed') {
        throw "Pester result was $($result.Result) with $($result.FailedCount) failed test(s)."
    }
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
