BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:InstallDeps = Join-Path $script:RepoRoot "install-deps.ps1"

    function winget {}
    function scoop {}
    function choco {}

    $script:ImportInstallDepsForTest = {
        param([switch]$DryRun)

        $oldSourceOnly = $env:INSTALL_DEPS_PS1_SOURCE_ONLY
        try {
            $env:INSTALL_DEPS_PS1_SOURCE_ONLY = '1'
            if ($DryRun) {
                . $script:InstallDeps -DryRun
            } else {
                . $script:InstallDeps -All
            }
        } finally {
            if ($null -eq $oldSourceOnly) {
                Remove-Item Env:INSTALL_DEPS_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
            } else {
                $env:INSTALL_DEPS_PS1_SOURCE_ONLY = $oldSourceOnly
            }
        }

        $script:InstallFailures = @()
        $script:InstallAttempts = @()
        $script:ToolInstalled = $false
        $global:LASTEXITCODE = 0
    }

    function Invoke-MockedManager {
        param([string]$Manager)

        $script:InstallAttempts += $Manager
        $exitCode = 1
        if ($script:ManagerExitCodes.ContainsKey($Manager)) {
            $exitCode = [int]$script:ManagerExitCodes[$Manager]
        }
        $global:LASTEXITCODE = $exitCode
        if ($exitCode -eq 0) {
            $script:ToolInstalled = $true
        }
    }

    function Mock-InstallOneManagers {
        param(
            [string[]]$InstalledManagers,
            [hashtable]$ExitCodes
        )

        $script:InstalledManagers = @($InstalledManagers)
        $script:ManagerExitCodes = $ExitCodes

        Mock -CommandName Read-Host -MockWith { throw "Read-Host must not run with -All" }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($script:InstalledManagers -contains $Name) {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName Test-Tool -MockWith { $script:ToolInstalled } -ParameterFilter { $name -eq 'git' }
        Mock -CommandName winget -MockWith { Invoke-MockedManager 'winget' }
        Mock -CommandName scoop -MockWith { Invoke-MockedManager 'scoop' }
        Mock -CommandName choco -MockWith { Invoke-MockedManager 'choco' }
    }
}

Describe "install-deps.ps1" {

    It "does not prompt before planning git under -DryRun" {
        $oldSourceOnly = $env:INSTALL_DEPS_PS1_SOURCE_ONLY
        try {
            $env:INSTALL_DEPS_PS1_SOURCE_ONLY = '1'
            . $script:InstallDeps -DryRun
        } finally {
            if ($null -eq $oldSourceOnly) {
                Remove-Item Env:INSTALL_DEPS_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
            } else {
                $env:INSTALL_DEPS_PS1_SOURCE_ONLY = $oldSourceOnly
            }
        }

        $Pm = 'winget'
        Mock -CommandName Read-Host -MockWith { throw "Read-Host must not run under -DryRun" }
        Mock -CommandName Test-Tool -MockWith { $false } -ParameterFilter { $name -eq 'git' }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }

        { Install-One git } | Should -Not -Throw
        Should -Invoke -CommandName Read-Host -Times 0 -Exactly
    }

    It "uses the documented elevated Scoop bootstrap path in dry run" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Get-Command -MockWith { return $null } -ParameterFilter { $Name -eq 'scoop' }
        Mock -CommandName Test-IsElevated -MockWith { return $true }

        # Capture the Write-Host (information) stream, not the boolean return value.
        $output = & { Install-Scoop } 6>&1 | Out-String

        $output | Should -Match '-RunAsAdmin'
    }

    It "uses winget when winget is the only installed manager" {
        . $script:ImportInstallDepsForTest
        $Pm = 'winget'
        Mock-InstallOneManagers -InstalledManagers @('winget') -ExitCodes @{ winget = 0 }

        Install-One git

        ($script:InstallAttempts -join ',') | Should -Be 'winget'
        Should -Invoke -CommandName winget -Times 1 -Exactly
        $script:InstallFailures.Count | Should -Be 0
    }

    It "uses scoop when scoop is the only installed manager" {
        . $script:ImportInstallDepsForTest
        $Pm = 'winget'
        Mock-InstallOneManagers -InstalledManagers @('scoop') -ExitCodes @{ scoop = 0 }

        Install-One git

        ($script:InstallAttempts -join ',') | Should -Be 'scoop'
        Should -Invoke -CommandName scoop -Times 1 -Exactly
        $script:InstallFailures.Count | Should -Be 0
    }

    It "uses choco when choco is the only installed manager" {
        . $script:ImportInstallDepsForTest
        $Pm = 'winget'
        Mock-InstallOneManagers -InstalledManagers @('choco') -ExitCodes @{ choco = 0 }

        Install-One git

        ($script:InstallAttempts -join ',') | Should -Be 'choco'
        Should -Invoke -CommandName choco -Times 1 -Exactly
        $script:InstallFailures.Count | Should -Be 0
    }

    It "tries scoop before the primary manager and then the remaining manager" {
        . $script:ImportInstallDepsForTest
        $Pm = 'winget'
        Mock-InstallOneManagers -InstalledManagers @('winget', 'scoop', 'choco') -ExitCodes @{
            scoop = 11
            winget = 12
            choco = 0
        }

        Install-One git

        ($script:InstallAttempts -join ',') | Should -Be 'scoop,winget,choco'
        Should -Invoke -CommandName scoop -Times 1 -Exactly
        Should -Invoke -CommandName winget -Times 1 -Exactly
        Should -Invoke -CommandName choco -Times 1 -Exactly
        $script:InstallFailures.Count | Should -Be 0
    }

    It "falls back to the next manager after a failed install" {
        . $script:ImportInstallDepsForTest
        $Pm = 'winget'
        Mock-InstallOneManagers -InstalledManagers @('winget', 'choco') -ExitCodes @{
            winget = 12
            choco = 0
        }

        Install-One git

        ($script:InstallAttempts -join ',') | Should -Be 'winget,choco'
        Should -Invoke -CommandName winget -Times 1 -Exactly
        Should -Invoke -CommandName choco -Times 1 -Exactly
        $script:InstallFailures.Count | Should -Be 0
    }

    It "records an install failure when every manager fails" {
        . $script:ImportInstallDepsForTest
        $Pm = 'winget'
        Mock-InstallOneManagers -InstalledManagers @('winget', 'scoop', 'choco') -ExitCodes @{
            scoop = 11
            winget = 12
            choco = 13
        }

        Install-One git

        ($script:InstallAttempts -join ',') | Should -Be 'scoop,winget,choco'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'git'
        $script:InstallFailures[0].Pm | Should -Be 'scoop/winget/choco'
        $script:InstallFailures[0].Pkg | Should -Be 'git'
        $script:InstallFailures[0].ExitCode | Should -Be 13
    }
}
