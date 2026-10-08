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
        'setup.ps1',
        'migrate.ps1',
        'scripts/installer-bootstrap.ps1',
        'test.ps1',
        'shells/powershell_profile.ps1',
        'tmux/rose-pine.ps1',
        'tests/nvim/run.ps1',
        'tests/powershell'
    )
    # Reviewed baseline for broad analyzer coverage. Counts retain the rationale
    # per rule group; the fingerprint below additionally binds the exact stable
    # script/rule/message/extent identities, so one warning cannot silently
    # replace another while preserving a filename/rule/count total.
    $analyzerWarningFingerprint = '55d9d54d663a97d7cff2a741cfc4aa67f9973868ff075a06a34c322985853b0a'
    $analyzerWarningBaseline = @{
        'Profile.Tests.ps1, PSAvoidUsingWriteHost' = @{ Count = 1; Reason = 'test fixture command output assertion' }
        'run.ps1, PSAvoidUsingWriteHost' = @{ Count = 3; Reason = 'nvim test harness progress output' }
        'test.ps1, PSAvoidUsingWriteHost' = @{ Count = 4; Reason = 'test harness progress output' }
        'test.ps1, PSUseApprovedVerbs' = @{ Count = 1; Reason = 'established Require-OrSkip harness helper' }
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

Invoke-Step 'Installer format, vet and race tests' {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go is required for the contributor gate.' }
    Push-Location (Join-Path $RepoRoot 'installer')
    try {
        $formatting = & gofmt -l .
        if ($LASTEXITCODE -ne 0 -or $formatting) { throw "Go formatting check failed: $formatting" }
        & go vet ./...
        if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
        & go test -race -timeout 20m ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go race tests failed' }
    } finally {
        Pop-Location
    }
}

Invoke-Step 'Public launchers and migration evidence' {
    & python -m unittest discover -s tests/bootstrap -p '*_test.py'
    if ($LASTEXITCODE -ne 0) { throw 'Bootstrap regression tests failed' }
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
