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

    It "adds required buckets when Scoop already exists" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()
        Mock -CommandName Get-Command -MockWith {
            return [pscustomobject]@{ Name = 'scoop'; Source = 'scoop' }
        } -ParameterFilter { $Name -eq 'scoop' }
        Mock -CommandName Test-Path -MockWith { return $false }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
        }

        Install-Scoop | Should -BeTrue

        $script:ScoopArgs | Should -Contain 'bucket add extras'
        $script:ScoopArgs | Should -Contain 'bucket add nerd-fonts'
    }

    It "skips scoop bucket add when the psmux bucket already exists" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()
        Mock -CommandName Test-Path -MockWith { return $true }
        Mock -CommandName Get-ChildItem -MockWith { return @([pscustomobject]@{ Name = 'manifest.json' }) }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
        }

        Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux' | Should -BeTrue

        ($script:ScoopArgs | Where-Object { $_ -like 'bucket add*' }).Count | Should -Be 0
    }

    It "sets GIT_TERMINAL_PROMPT and GCM_INTERACTIVE to 0 during the add and restores them after" {
        . $script:ImportInstallDepsForTest
        $originalPrompt = $env:GIT_TERMINAL_PROMPT
        $originalGcm = $env:GCM_INTERACTIVE
        $script:BucketPopulated = $false
        $script:CapturedGitEnv = @()
        Mock -CommandName Test-Path -MockWith { return $script:BucketPopulated }
        Mock -CommandName Get-ChildItem -MockWith {
            if ($script:BucketPopulated) {
                return @([pscustomobject]@{ Name = 'bucket.json' })
            }
            return @()
        }
        Mock -CommandName scoop -MockWith {
            if (($args -join ' ') -like 'bucket add*') {
                $script:CapturedGitEnv += [pscustomobject]@{
                    Prompt = $env:GIT_TERMINAL_PROMPT
                    Gcm = $env:GCM_INTERACTIVE
                }
                $script:BucketPopulated = $true
            }
        }

        try {
            $env:GIT_TERMINAL_PROMPT = 'original-prompt'
            $env:GCM_INTERACTIVE = 'original-gcm'

            Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux' | Should -BeTrue

            $script:CapturedGitEnv[-1].Prompt | Should -Be '0'
            $script:CapturedGitEnv[-1].Gcm | Should -Be '0'
            $env:GIT_TERMINAL_PROMPT | Should -Be 'original-prompt'
            $env:GCM_INTERACTIVE | Should -Be 'original-gcm'

            Remove-Item Env:GIT_TERMINAL_PROMPT -ErrorAction SilentlyContinue
            Remove-Item Env:GCM_INTERACTIVE -ErrorAction SilentlyContinue
            $script:BucketPopulated = $false
            $script:CapturedGitEnv = @()

            Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux' | Should -BeTrue

            $script:CapturedGitEnv[-1].Prompt | Should -Be '0'
            $script:CapturedGitEnv[-1].Gcm | Should -Be '0'
            ($null -eq $env:GIT_TERMINAL_PROMPT) | Should -BeTrue
            ($null -eq $env:GCM_INTERACTIVE) | Should -BeTrue
        } finally {
            if ($null -eq $originalPrompt) {
                Remove-Item Env:GIT_TERMINAL_PROMPT -ErrorAction SilentlyContinue
            } else {
                $env:GIT_TERMINAL_PROMPT = $originalPrompt
            }
            if ($null -eq $originalGcm) {
                Remove-Item Env:GCM_INTERACTIVE -ErrorAction SilentlyContinue
            } else {
                $env:GCM_INTERACTIVE = $originalGcm
            }
        }
    }

    It "returns false and falls through to winget when the bucket never populates" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()
        $script:WingetArgs = @()
        $script:PsmuxInstalled = $false
        Mock -CommandName Test-Path -MockWith { return $false }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -in @('scoop', 'winget')) {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName Test-Tool -MockWith { return $script:PsmuxInstalled } -ParameterFilter { $name -eq 'psmux' }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
            $global:LASTEXITCODE = 1
        }
        Mock -CommandName winget -MockWith {
            $script:WingetArgs += ($args -join ' ')
            $global:LASTEXITCODE = 0
            $script:PsmuxInstalled = $true
        }

        Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux' | Should -BeFalse
        $script:ScoopArgs | Should -Contain 'bucket rm psmux'

        $script:ScoopArgs = @()
        Install-Psmux

        $script:ScoopArgs | Should -Contain 'bucket rm psmux'
        $script:WingetArgs | Should -Contain 'install psmux --accept-source-agreements --accept-package-agreements --silent'
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

    It "registers lazygit in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('lazygit') | Should -BeTrue
        $BinaryName['lazygit'] | Should -Be 'lazygit'
        $Catalog['lazygit'].scoop | Should -Be 'lazygit'
        $Catalog['lazygit'].winget | Should -Be 'JesseDuffield.lazygit'
    }

    It "registers chezmoi in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('chezmoi') | Should -BeTrue
        $BinaryName['chezmoi'] | Should -Be 'chezmoi'
        $Catalog['chezmoi'].scoop | Should -Be 'chezmoi'
        $Catalog['chezmoi'].winget | Should -Be 'twpayne.chezmoi'
        $Catalog['chezmoi'].choco | Should -Be 'chezmoi'
    }

    It "registers Windows Terminal in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('wt') | Should -BeTrue
        $BinaryName['wt'] | Should -Be 'wt'
        $Catalog['wt'].scoop | Should -Be 'extras/windows-terminal'
        $Catalog['wt'].winget | Should -Be 'Microsoft.WindowsTerminal'
        $Catalog['wt'].choco | Should -Be 'microsoft-windows-terminal'
    }

    It "registers PowerShell 7 (pwsh) in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('pwsh') | Should -BeTrue
        $BinaryName['pwsh'] | Should -Be 'pwsh'
        $Catalog['pwsh'].scoop | Should -Be 'pwsh'
        $Catalog['pwsh'].winget | Should -Be 'Microsoft.PowerShell'
        $Catalog['pwsh'].choco | Should -Be 'powershell-core'
    }

    It "dry-runs a scoped PowerShell scoop update without blanket updates" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:ScoopArgs = @()

        Mock -CommandName Read-Host -MockWith { throw "Read-Host must not run under -DryRun" }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName Test-Tool -MockWith { return $true } -ParameterFilter { $name -eq 'pwsh' }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
            if (($args -join ' ') -eq 'list pwsh') {
                return 'pwsh 7.5.0'
            }
        }

        $output = & { Update-ScoopTool pwsh } 6>&1 | Out-String

        $wouldUpdatePwsh = @($output -split "`r?`n" | Where-Object {
                $_ -match '^\s*would:\s+scoop update;\s+scoop update pwsh\s*$'
            })
        $wouldUpdatePwsh.Count | Should -Be 1
        $output | Should -Not -Match 'scoop update \*'
        $script:ScoopArgs | Should -Contain 'list pwsh'
        ($script:ScoopArgs | Where-Object { $_ -like 'update*' }).Count | Should -Be 0
    }
}

Describe "Set-VSCodeTheme" {
    BeforeAll {
        . $script:ImportInstallDepsForTest
        $script:ExpectedTheme = "Ros$([char]0xE9) Pine"
        $script:ExpectedFont = "'Hack Nerd Font', Consolas, monospace"
        $script:Utf8NoBom = [System.Text.UTF8Encoding]::new($false)
        $script:VSCodeThemeTempDirs = @()

        function New-SettingsPath {
            $dir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N'))
            New-Item -ItemType Directory -Force -Path $dir | Out-Null
            $script:VSCodeThemeTempDirs += $dir
            return (Join-Path $dir 'settings.json')
        }

        function Write-TestSettings {
            param(
                [Parameter(Mandatory)][string]$Path,
                [Parameter(Mandatory)][string]$Text
            )
            $dir = Split-Path -Parent $Path
            if (-not (Test-Path -LiteralPath $dir)) {
                New-Item -ItemType Directory -Force -Path $dir | Out-Null
            }
            [System.IO.File]::WriteAllText($Path, $Text, $script:Utf8NoBom)
        }

        function Read-TestSettings {
            param([Parameter(Mandatory)][string]$Path)
            return [System.IO.File]::ReadAllText($Path)
        }

        function Read-StrictSettings {
            param([Parameter(Mandatory)][string]$Path)
            return (Read-TestSettings -Path $Path | ConvertFrom-Json)
        }

        function Get-LiveThemeKeyCount {
            param([Parameter(Mandatory)][string]$Text)
            return [regex]::Matches($Text, '(?m)^  "workbench\.colorTheme"\s*:').Count
        }

        function Test-AllVSCodeSettingsText {
            param([Parameter(Mandatory)][string]$Text)
            $escapedTheme = [regex]::Escape($script:ExpectedTheme)
            $escapedFont = [regex]::Escape($script:ExpectedFont)
            $Text | Should -Match ('"workbench\.colorTheme"\s*:\s*"' + $escapedTheme + '"')
            $Text | Should -Match ('"editor\.fontFamily"\s*:\s*"' + $escapedFont + '"')
            $Text | Should -Match ('"terminal\.integrated\.fontFamily"\s*:\s*"' + $escapedFont + '"')
        }
    }

    AfterAll {
        foreach ($dir in $script:VSCodeThemeTempDirs) {
            Remove-Item -LiteralPath $dir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "creates a fresh settings file with theme and font keys" {
        $settingsPath = New-SettingsPath

        Set-VSCodeTheme -SettingsPath $settingsPath

        $settings = Read-StrictSettings -Path $settingsPath
        $settings.'workbench.colorTheme' | Should -Be $script:ExpectedTheme
        $settings.'editor.fontFamily' | Should -Be $script:ExpectedFont
        $settings.'terminal.integrated.fontFamily' | Should -Be $script:ExpectedFont
    }

    It "merges strict JSON while preserving existing keys" {
        $settingsPath = New-SettingsPath
        Write-TestSettings -Path $settingsPath -Text "{`n  `"editor.fontSize`": 14`n}`n"

        Set-VSCodeTheme -SettingsPath $settingsPath

        $settings = Read-StrictSettings -Path $settingsPath
        $settings.'editor.fontSize' | Should -Be 14
        $settings.'workbench.colorTheme' | Should -Be $script:ExpectedTheme
        $settings.'editor.fontFamily' | Should -Be $script:ExpectedFont
        $settings.'terminal.integrated.fontFamily' | Should -Be $script:ExpectedFont
    }

    It "edits JSONC comments in place, preserves CRLF, and stays idempotent" {
        $settingsPath = New-SettingsPath
        Write-TestSettings -Path $settingsPath -Text "// header comment`r`n{`r`n  `"editor.fontSize`": 14, // inline`r`n}`r`n"

        Set-VSCodeTheme -SettingsPath $settingsPath
        $updated = Read-TestSettings -Path $settingsPath

        $updated | Should -Match '// header comment'
        $updated | Should -Match '// inline'
        $updated | Should -Match '"editor\.fontSize"\s*:\s*14'
        $updated.Contains("`r`n") | Should -BeTrue
        Test-AllVSCodeSettingsText -Text $updated
        (Get-LiveThemeKeyCount -Text $updated) | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $settingsPath) -Filter 'settings.json.bak.*').Count | Should -Be 1

        Set-VSCodeTheme -SettingsPath $settingsPath
        $updatedAgain = Read-TestSettings -Path $settingsPath
        (Get-LiveThemeKeyCount -Text $updatedAgain) | Should -Be 1
    }

    It "replaces an existing JSONC top-level theme without duplicating it" {
        $settingsPath = New-SettingsPath
        Write-TestSettings -Path $settingsPath -Text "{`n  // keep this comment`n  `"workbench.colorTheme`": `"Old`",`n  `"editor.fontSize`": 14`n}`n"

        Set-VSCodeTheme -SettingsPath $settingsPath
        $updated = Read-TestSettings -Path $settingsPath

        (Get-LiveThemeKeyCount -Text $updated) | Should -Be 1
        $updated | Should -Not -Match '"workbench\.colorTheme"\s*:\s*"Old"'
        $updated | Should -Match '// keep this comment'
        Test-AllVSCodeSettingsText -Text $updated
    }

    It "ignores commented-out keys and string value mentions" {
        $settingsPath = New-SettingsPath
        Write-TestSettings -Path $settingsPath -Text "{`n  // `"workbench.colorTheme`": `"x`"`n  `"notes`": `"workbench.colorTheme inside a string value`",`n  `"editor.fontSize`": 14`n}`n"

        Set-VSCodeTheme -SettingsPath $settingsPath
        $updated = Read-TestSettings -Path $settingsPath

        $updated | Should -Match '// "workbench\.colorTheme": "x"'
        $updated | Should -Match '"notes"\s*:\s*"workbench\.colorTheme inside a string value"'
        (Get-LiveThemeKeyCount -Text $updated) | Should -Be 1
        Test-AllVSCodeSettingsText -Text $updated
    }

    It "leaves nested matching keys unchanged" {
        $settingsPath = New-SettingsPath
        Write-TestSettings -Path $settingsPath -Text "{`n  // force JSONC fallback`n  `"nested`": {`n    `"workbench.colorTheme`": `"Nested Old`"`n  }`n}`n"

        Set-VSCodeTheme -SettingsPath $settingsPath
        $updated = Read-TestSettings -Path $settingsPath

        $updated | Should -Match '"workbench\.colorTheme"\s*:\s*"Nested Old"'
        (Get-LiveThemeKeyCount -Text $updated) | Should -Be 1
        Test-AllVSCodeSettingsText -Text $updated
    }
}
