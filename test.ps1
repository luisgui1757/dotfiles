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

function Get-AnalyzerDiagnosticFingerprint {
    param(
        [Parameter(Mandatory)] [object[]]$Findings,
        [Parameter(Mandatory)] [string]$Root
    )

    $rootPath = [System.IO.Path]::GetFullPath($Root).TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    [string[]]$identities = @(
        foreach ($finding in $Findings) {
            $scriptPath = [System.IO.Path]::GetFullPath([string]$finding.ScriptPath)
            if ($scriptPath.StartsWith($rootPath, [System.StringComparison]::OrdinalIgnoreCase)) {
                $scriptPath = $scriptPath.Substring($rootPath.Length)
            }
            $scriptPath = $scriptPath.Replace('\', '/')
            $message = ([regex]::Replace([string]$finding.Message, '\s+', ' ')).Trim()
            $extent = ([regex]::Replace([string]$finding.Extent.Text, '\s+', ' ')).Trim()
            '{0}|{1}|{2}|{3}' -f $scriptPath, $finding.RuleName, $message, $extent
        }
    )
    [Array]::Sort($identities, [System.StringComparer]::Ordinal)
    $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes(($identities -join "`0"))
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        return ([System.BitConverter]::ToString($sha256.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha256.Dispose()
    }
}

Invoke-Step 'PSScriptAnalyzer' {
    if (-not (Require-OrSkip 'PSScriptAnalyzer' ([bool](Get-Module -ListAvailable PSScriptAnalyzer)) 'Install-Module PSScriptAnalyzer')) {
        return
    }
    Import-Module PSScriptAnalyzer -Force
    $analyzerPaths = @(
        'install-deps.ps1',
        'setup.ps1',
        'scripts/upgrade-v0.1.0.ps1',
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
    # Reviewed baseline for broad analyzer coverage. Counts retain the rationale
    # per rule group; the fingerprint below additionally binds the exact stable
    # script/rule/message/extent identities, so one warning cannot silently
    # replace another while preserving a filename/rule/count total.
    $analyzerWarningFingerprint = '5630775a75ab6bae93cacc874593f28907beea32909571812e54d6d2cb06f090'
    $analyzerWarningBaseline = @{
        'dot_tmux.rose-pine.ps1, PSAvoidUsingWriteHost'                               = @{ Count = 1; Reason = 'generated tmux theme status output' }
        'install-deps.ps1, PSAvoidUsingWriteHost'                                     = @{ Count = 165; Reason = 'interactive installer progress output' }
        'install-deps.ps1, PSUseApprovedVerbs'                                        = @{ Count = 5; Reason = 'established installer helper names' }
        'install-deps.ps1, PSUseShouldProcessForStateChangingFunctions'               = @{ Count = 6; Reason = 'installer entry points are explicitly invoked' }
        'install-deps.ps1, PSUseSingularNouns'                                        = @{ Count = 11; Reason = 'established installer helper names' }
        'install-wt-portable.ps1, PSAvoidUsingWriteHost'                              = @{ Count = 3; Reason = 'greenfield harness progress output' }
        'InstallDeps.Tests.ps1, PSAvoidAssignmentToAutomaticVariable'                 = @{ Count = 2; Reason = 'Pester mock scope fixtures' }
        'InstallDeps.Tests.ps1, PSAvoidOverwritingBuiltInCmdlets'                     = @{ Count = 1; Reason = 'Pester mock command fixture' }
        'InstallDeps.Tests.ps1, PSReviewUnusedParameter'                              = @{ Count = 17; Reason = 'Pester mock signatures mirror production calls' }
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
        'sandbox-run.ps1, PSAvoidUsingWriteHost'                                      = @{ Count = 15; Reason = 'greenfield harness progress output' }
        'sandbox-run.ps1, PSUseShouldProcessForStateChangingFunctions'                = @{ Count = 1; Reason = 'sandbox harness setup helper' }
        'sandbox-run.ps1, PSUseSingularNouns'                                         = @{ Count = 1; Reason = 'sandbox harness helper name' }
        'setup.ps1, PSAvoidUsingWriteHost'                                            = @{ Count = 101; Reason = 'interactive setup progress output' }
        'setup.ps1, PSUseApprovedVerbs'                                               = @{ Count = 1; Reason = 'established setup helper name' }
        'setup.ps1, PSUseShouldProcessForStateChangingFunctions'                      = @{ Count = 7; Reason = 'setup entry points are explicitly invoked' }
        'setup.ps1, PSUseSingularNouns'                                               = @{ Count = 9; Reason = 'established setup helper names' }
        'Setup.Tests.ps1, PSReviewUnusedParameter'                                    = @{ Count = 4; Reason = 'Pester mock signatures mirror production calls' }
        'test.ps1, PSAvoidUsingWriteHost'                                             = @{ Count = 4; Reason = 'test runner progress output' }
        'test.ps1, PSUseApprovedVerbs'                                                = @{ Count = 1; Reason = 'test runner step helper name' }
        'uninstall.ps1, PSAvoidUsingWriteHost'                                        = @{ Count = 21; Reason = 'interactive uninstall progress output' }
        'uninstall.ps1, PSReviewUnusedParameter'                                      = @{ Count = 5; Reason = 'interactive uninstall helper signatures' }
        'uninstall.ps1, PSUseShouldProcessForStateChangingFunctions'                  = @{ Count = 3; Reason = 'uninstall entry points are explicitly invoked' }
        'uninstall.ps1, PSUseSingularNouns'                                           = @{ Count = 7; Reason = 'established uninstall helper names' }
        'Uninstall.Tests.ps1, PSUseDeclaredVarsMoreThanAssignments'                   = @{ Count = 2; Reason = 'Pester assertion captures' }
        'validate.ps1, PSAvoidUsingWriteHost'                                         = @{ Count = 6; Reason = 'greenfield validator progress output' }
        'validate.ps1, PSUseShouldProcessForStateChangingFunctions'                   = @{ Count = 1; Reason = 'greenfield validator helper' }
        'validate.ps1, PSUseSingularNouns'                                            = @{ Count = 1; Reason = 'greenfield validator helper name' }
        'windows_apply_test.ps1, PSAvoidUsingWriteHost'                               = @{ Count = 3; Reason = 'migration test progress output' }
        'windows_apply_test.ps1, PSUseShouldProcessForStateChangingFunctions'         = @{ Count = 3; Reason = 'migration test helpers' }
        'windows_apply_test.ps1, PSUseSingularNouns'                                  = @{ Count = 5; Reason = 'migration test helper names' }
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
    $seenWarningGroups = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    $baselineDrift = [System.Collections.Generic.List[string]]::new()
    foreach ($group in $warningGroups) {
        [void]$seenWarningGroups.Add($group.Name)
        $baseline = $analyzerWarningBaseline[$group.Name]
        if ($null -eq $baseline) {
            foreach ($finding in $group.Group) {
                [void]$unexpectedDiag.Add($finding)
            }
            continue
        }
        $allowedCount = [int]$baseline.Count
        if ($group.Count -ne $allowedCount) {
            [void]$baselineDrift.Add("$($group.Name): expected $allowedCount, actual $($group.Count)")
            foreach ($finding in $group.Group) {
                [void]$unexpectedDiag.Add($finding)
            }
        }
    }
    foreach ($groupName in $analyzerWarningBaseline.Keys) {
        if (-not $seenWarningGroups.Contains($groupName)) {
            [void]$baselineDrift.Add("$groupName`: expected $($analyzerWarningBaseline[$groupName].Count), actual 0")
        }
    }
    $unexpected = @($unexpectedDiag.ToArray())
    if ($unexpected.Count -gt 0) {
        $unexpected | Format-Table -AutoSize
        $baselineDrift | ForEach-Object { Write-Output $_ }
        throw "PSScriptAnalyzer reported $($unexpected.Count) unexpected warning or error finding(s)."
    }
    if ($baselineDrift.Count -gt 0) {
        throw "PSScriptAnalyzer warning groups drifted: $($baselineDrift -join '; ')"
    }
    $actualWarningFingerprint = Get-AnalyzerDiagnosticFingerprint `
        -Findings @($diag | Where-Object { $_.Severity -ne 'Error' }) -Root $RepoRoot
    if ($actualWarningFingerprint -ne $analyzerWarningFingerprint) {
        throw "PSScriptAnalyzer warning identities drifted (expected $analyzerWarningFingerprint, actual $actualWarningFingerprint). Review the exact diagnostics before updating the baseline."
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
