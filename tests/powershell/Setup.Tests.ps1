BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Setup = Join-Path $script:RepoRoot "setup.ps1"
    $script:ManagedPwshProfileGuid = '{8a0e8c9b-2b4c-5842-ac1b-29cd17efc89b}'
    $script:LegacyWindowsPowerShellProfileGuid = '{61c54bbd-c2c6-5271-96e7-009a87ff44bf}'

    $script:ImportSetupForTest = {
        param([hashtable]$Parameters = @{ All = $true })

        $oldSourceOnly = $env:DOTFILES_SETUP_PS1_SOURCE_ONLY
        try {
            $env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
            . $script:Setup @Parameters
        } finally {
            if ($null -eq $oldSourceOnly) {
                Remove-Item Env:DOTFILES_SETUP_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
            } else {
                $env:DOTFILES_SETUP_PS1_SOURCE_ONLY = $oldSourceOnly
            }
        }
    }
}

$script:ChezmoiCommandForContentTests = Get-Command chezmoi -ErrorAction SilentlyContinue

Describe "setup.ps1 Test-TargetContentMatchesChezmoi copy-mode" -Skip:(-not $script:ChezmoiCommandForContentTests) {
    BeforeAll {
        $script:OldUserProfileForContentTest = $env:USERPROFILE
        $script:ContentTestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("setup-content-" + [System.Guid]::NewGuid())
        $script:ContentTestSource = Join-Path $script:ContentTestRoot 'source'
        $script:ContentTestDest = Join-Path $script:ContentTestRoot 'dest'
        $script:ContentTestProfile = Join-Path $script:ContentTestRoot 'profile'
        New-Item -ItemType Directory -Force -Path $script:ContentTestSource, $script:ContentTestDest, $script:ContentTestProfile | Out-Null
        $env:USERPROFILE = $script:ContentTestProfile

        . $script:ImportSetupForTest

        $script:ContentTestConfig = Join-Path $script:ContentTestRoot 'chezmoi.toml'
        [System.IO.File]::WriteAllText($script:ContentTestConfig, '', [System.Text.UTF8Encoding]::new($false))
        $script:ContentTestState = Join-Path $script:ContentTestRoot 'chezmoi-state.boltdb'
        $script:ChezmoiBaseArgs = @(
            '--source', $script:ContentTestSource,
            '--destination', $script:ContentTestDest,
            '--persistent-state', $script:ContentTestState
        )
        $script:ChezmoiConfigArgs = @('--config', $script:ContentTestConfig, '--config-format', 'toml')
        $script:ProbeBytes = [byte[]](0x70, 0x72, 0x6f, 0x62, 0x65, 0x0d, 0x0a, 0x63, 0x6f, 0x70, 0x79, 0x0a)
        [System.IO.File]::WriteAllBytes((Join-Path $script:ContentTestSource 'dot_probe'), $script:ProbeBytes)

        $baseArgs = $script:ChezmoiBaseArgs
        $configArgs = $script:ChezmoiConfigArgs
        & chezmoi @baseArgs @configArgs --no-tty --force apply
        if ($LASTEXITCODE -ne 0) {
            throw "chezmoi apply failed for setup.ps1 content-match test"
        }
        $script:ProbeTarget = Join-Path $script:ContentTestDest '.probe'
    }

    BeforeEach {
        [System.IO.File]::WriteAllBytes($script:ProbeTarget, $script:ProbeBytes)
    }

    AfterAll {
        if ($null -eq $script:OldUserProfileForContentTest) {
            Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
        } else {
            $env:USERPROFILE = $script:OldUserProfileForContentTest
        }
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:ContentTestRoot
    }

    It "returns true when a copy-mode target matches chezmoi cat bytes" {
        Test-TargetContentMatchesChezmoi $script:ProbeTarget | Should -BeTrue
    }

    It "returns false when a copy-mode target differs from chezmoi cat bytes" {
        [System.IO.File]::WriteAllBytes($script:ProbeTarget, [byte[]](0x64, 0x69, 0x66, 0x66, 0x0a))

        Test-TargetContentMatchesChezmoi $script:ProbeTarget | Should -BeFalse
    }
}

Describe "setup.ps1 Update-RuntimePath" {
    BeforeEach {
        $script:OldPath = $env:PATH
        $script:OldScoop = $env:SCOOP
        $script:OldUserProfile = $env:USERPROFILE
        $script:OldHome = $env:HOME
        $script:FakeHome = Join-Path ([System.IO.Path]::GetTempPath()) ("setup-path-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path $script:FakeHome | Out-Null
        $env:USERPROFILE = $script:FakeHome
        . $script:ImportSetupForTest
    }

    AfterEach {
        $env:PATH = $script:OldPath
        if ($null -eq $script:OldUserProfile) {
            Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
        } else {
            $env:USERPROFILE = $script:OldUserProfile
        }
        if ($null -eq $script:OldHome) {
            Remove-Item Env:HOME -ErrorAction SilentlyContinue
        } else {
            $env:HOME = $script:OldHome
        }
        if ($null -eq $script:OldScoop) {
            Remove-Item Env:SCOOP -ErrorAction SilentlyContinue
        } else {
            $env:SCOOP = $script:OldScoop
        }
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:FakeHome
    }

    It "prepends existing Scoop shims and de-duplicates PATH" {
        $scoopRoot = Join-Path $script:FakeHome 'scoop-root'
        $shimDir = Join-Path $scoopRoot 'shims'
        New-Item -ItemType Directory -Force -Path $shimDir | Out-Null
        $env:SCOOP = $scoopRoot
        $fakeOne = Join-Path $script:FakeHome 'one'
        $fakeTwo = Join-Path $script:FakeHome 'two'
        $env:PATH = "$fakeOne;$fakeTwo;$fakeOne"

        Update-RuntimePath

        $parts = $env:PATH -split ';'
        $parts[0] | Should -Be $shimDir
        @($parts | Where-Object { $_ -eq $fakeOne }).Count | Should -Be 1
        @($parts | Where-Object { $_ -eq $fakeTwo }).Count | Should -Be 1
    }

    It "does not add a missing Scoop shims directory" {
        $scoopRoot = Join-Path $script:FakeHome 'missing-scoop'
        $missingShim = Join-Path $scoopRoot 'shims'
        $env:SCOOP = $scoopRoot
        $fakeOne = Join-Path $script:FakeHome 'one'
        $env:PATH = "$fakeOne;$fakeOne"

        Update-RuntimePath

        $parts = $env:PATH -split ';'
        @($parts | Where-Object { $_ -eq $missingShim }).Count | Should -Be 0
        @($parts | Where-Object { $_ -eq $fakeOne }).Count | Should -Be 1
    }

    It "falls back to HOME when USERPROFILE and SCOOP are absent" {
        Remove-Item Env:SCOOP -ErrorAction SilentlyContinue
        Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
        $env:HOME = $script:FakeHome
        $shimDir = Join-Path $script:FakeHome 'scoop/shims'
        New-Item -ItemType Directory -Force -Path $shimDir | Out-Null
        $fakeOne = Join-Path $script:FakeHome 'one'
        $env:PATH = "$fakeOne;$fakeOne"

        Update-RuntimePath

        $parts = $env:PATH -split ';'
        $parts[0] | Should -Be $shimDir
        @($parts | Where-Object { $_ -eq $fakeOne }).Count | Should -Be 1
    }
}

Describe "setup.ps1 source-only import" {
    It "does not require USERPROFILE on non-Windows hosts" {
        $oldUserProfile = $env:USERPROFILE
        try {
            Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
            $importError = $null
            try {
                . $script:ImportSetupForTest
            } catch {
                $importError = $_
            }
            $importError | Should -BeNullOrEmpty
            Get-DefaultProfileRoot | Should -Not -BeNullOrEmpty
        } finally {
            if ($null -eq $oldUserProfile) {
                Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
            } else {
                $env:USERPROFILE = $oldUserProfile
            }
        }
    }
}

Describe "setup.ps1 Polaris agent policy" {
    BeforeEach {
        . $script:ImportSetupForTest
        $script:PolarisTestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("setup-polaris-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path $script:PolarisTestRoot | Out-Null
    }

    AfterEach {
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:PolarisTestRoot
    }

    It "previews the pinned Polaris global install in dry-run mode" {
        $cache = Join-Path $script:PolarisTestRoot 'cache'
        $output = & {
            Invoke-PolarisAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$true `
                -Version '0.1.1' `
                -Ref '489dcc6f991ddcff63c460a433e983264dc54cf7' `
                -CacheRoot $cache
        } 6>&1 | Out-String

        $output | Should -Match 'Phase 6/6: apply global agent policy \(Polaris\)'
        $output | Should -Match 'would\s+clone/fetch Polaris 0\.1\.1'
        $output | Should -Match 'tools/install --global'
        Test-Path -LiteralPath $cache | Should -BeFalse
    }

    It "honors -SkipAgents" {
        $output = & {
            Invoke-PolarisAgentPolicy -SkipAgentsPhase:$true -AllMode:$true
        } 6>&1 | Out-String

        $output | Should -Match 'skipped: Phase 6/6 \(agent policy\) via -SkipAgents'
    }

    It "uses the interactive prompt only outside all and dry-run modes" {
        Test-ShouldApplyAgentPolicy `
            -SkipAgentsPhase:$false `
            -AllMode:$false `
            -IsDryRun:$false `
            -Prompt { return 'n' } | Should -BeFalse

        Test-ShouldApplyAgentPolicy `
            -SkipAgentsPhase:$false `
            -AllMode:$false `
            -IsDryRun:$false `
            -Prompt { return '' } | Should -BeTrue
    }

    It "prefers validated Git Bash over PATH bash on Windows" {
        $oldOs = $env:OS
        try {
            $env:OS = 'Windows_NT'
            $gitRoot = Join-Path $script:PolarisTestRoot 'Git'
            $script:ExpectedPolarisGitBash = Join-Path $gitRoot 'bin\bash.exe'
            $pathBash = Join-Path $script:PolarisTestRoot 'System32\bash.exe'

            Mock -CommandName Get-Command -MockWith {
                param([string]$Name)
                if ($Name -eq 'git') {
                    return [pscustomobject]@{ Source = (Join-Path $gitRoot 'cmd\git.exe') }
                }
                if ($Name -eq 'bash') {
                    return [pscustomobject]@{ Source = $pathBash }
                }
                return $null
            }
            Mock -CommandName Test-PolarisGitBashCommand -MockWith {
                param([string]$Candidate)
                return ($Candidate -eq $script:ExpectedPolarisGitBash)
            }

            Get-PolarisBashCommand | Should -Be $script:ExpectedPolarisGitBash
        } finally {
            Remove-Variable -Name ExpectedPolarisGitBash -Scope Script -ErrorAction SilentlyContinue
            if ($null -eq $oldOs) {
                Remove-Item Env:OS -ErrorAction SilentlyContinue
            } else {
                $env:OS = $oldOs
            }
        }
    }

    It "rejects PATH bash on Windows when it is not Git Bash" {
        $oldOs = $env:OS
        try {
            $env:OS = 'Windows_NT'
            $pathBash = Join-Path $script:PolarisTestRoot 'System32\bash.exe'

            Mock -CommandName Get-Command -MockWith {
                param([string]$Name)
                if ($Name -eq 'bash') {
                    return [pscustomobject]@{ Source = $pathBash }
                }
                return $null
            }
            Mock -CommandName Test-PolarisGitBashCommand -MockWith { return $false }

            Get-PolarisBashCommand | Should -BeNullOrEmpty
        } finally {
            if ($null -eq $oldOs) {
                Remove-Item Env:OS -ErrorAction SilentlyContinue
            } else {
                $env:OS = $oldOs
            }
        }
    }

    It "runs the pinned Polaris installer and global check from a verified checkout" {
        $work = Join-Path $script:PolarisTestRoot 'polaris-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.1`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "PATH=%s\n" "$PATH" >> "$POLARIS_TEST_LOG"
printf "%s\n" "$*" >> "$POLARIS_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()

        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'polaris-install.log'
            Invoke-PolarisAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.1' `
                -Ref $sha `
                -CacheRoot $cache

            $calls = Get-Content -LiteralPath $env:POLARIS_TEST_LOG
            $calls | Should -Contain '--global'
            $calls | Should -Contain '--global --check'
            if ($env:OS -eq 'Windows_NT') {
                $calls | Should -Contain 'PATH=/usr/bin:/bin'
            }
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:POLARIS_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:POLARIS_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects a dirty verified Polaris checkout before running the installer" {
        $work = Join-Path $script:PolarisTestRoot 'polaris-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.1`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$POLARIS_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()

        Add-Content -LiteralPath $installer -Value '# dirty cache regression'
        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'dirty-install.log'
            $setupLiteral = $script:Setup.Replace("'", "''")
            $cacheLiteral = $cache.Replace("'", "''")
            $shaLiteral = $sha.Replace("'", "''")
            $probe = @"
`$ErrorActionPreference = 'Stop'
`$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '$setupLiteral' -All
Invoke-PolarisAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.1' -Ref '$shaLiteral' -CacheRoot '$cacheLiteral'
"@
            $oldNativePreference = $null
            $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
            try {
                if ($hasNativePreference) {
                    $oldNativePreference = $PSNativeCommandUseErrorActionPreference
                    $PSNativeCommandUseErrorActionPreference = $false
                }
                $output = & pwsh -NoLogo -NoProfile -Command $probe 2>&1 | Out-String
                $rc = $LASTEXITCODE
            } finally {
                if ($hasNativePreference) {
                    $PSNativeCommandUseErrorActionPreference = $oldNativePreference
                }
            }

            $rc | Should -Not -Be 0
            $output | Should -Match 'Polaris cache has local changes'
            Test-Path -LiteralPath $env:POLARIS_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:POLARIS_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:POLARIS_TEST_LOG = $oldLog
            }
        }
    }
}

Describe "setup.ps1 update mode" {
    BeforeEach {
        . $script:ImportSetupForTest
        $script:SetupUpdateDepsPath = ''
        $script:SetupUpdateDepsArgs = @{}
        $script:SetupUpdateNvimRan = $false
        $script:SetupUpdateRuntimeRefreshed = $false
        Mock -CommandName Update-RuntimePath -MockWith { $script:SetupUpdateRuntimeRefreshed = $true }
        Mock -CommandName Invoke-ChezmoiApplyPhase -MockWith { throw "chezmoi apply must not run in update mode" }
    }

    It "runs install-deps Update and MasonToolsUpdate only" {
        $root = Join-Path ([System.IO.Path]::GetTempPath()) 'setup-update-root'
        $depsRunner = {
            param([string]$Path, [hashtable]$Arguments)
            $script:SetupUpdateDepsPath = $Path
            $script:SetupUpdateDepsArgs = $Arguments
        }
        $commandTester = {
            param([string]$Name)
            return ($Name -eq 'nvim')
        }
        $nvimRunner = {
            $script:SetupUpdateNvimRan = $true
            $global:LASTEXITCODE = 0
        }

        $output = & {
            Invoke-SetupUpdateMode `
                -Root $root `
                -DependencyArgs @{} `
                -DependencyRunner $depsRunner `
                -CommandTester $commandTester `
                -NvimRunner $nvimRunner
        } 6>&1 | Out-String

        $script:SetupUpdateDepsPath | Should -Be (Join-Path $root 'install-deps.ps1')
        $script:SetupUpdateDepsArgs['Update'] | Should -BeTrue
        $script:SetupUpdateNvimRan | Should -BeTrue
        $script:SetupUpdateRuntimeRefreshed | Should -BeTrue
        $output | Should -Match 'Update 1/2'
        $output | Should -Match 'Update 2/2'
        $output | Should -Match 'Plugins \(lazy-lock\.json\), pinned binaries, and configs update via `git pull` then re-run setup'
        $output | Should -Not -Match 'chezmoi|Lazy restore|Lazy sync|Tree-sitter|MasonToolsInstallSync'
        Should -Invoke -CommandName Invoke-ChezmoiApplyPhase -Times 0 -Exactly
    }

    It "dry-runs Mason update without invoking nvim" {
        $root = Join-Path ([System.IO.Path]::GetTempPath()) 'setup-update-root'
        $depsRunner = {
            param([string]$Path, [hashtable]$Arguments)
            $script:SetupUpdateDepsPath = $Path
            $script:SetupUpdateDepsArgs = $Arguments
        }
        $nvimRunner = {
            throw "nvim must not run during dry-run update mode"
        }
        $commandTester = {
            throw "nvim lookup must not run during dry-run update mode"
        }

        # Out-String emits CRLF on Windows; strip CR so the (?m) line-end
        # anchor ([ \t]*$) matches -- in .NET regex $ does not match before a bare \r.
        $output = (& {
            Invoke-SetupUpdateMode `
                -Root $root `
                -DependencyArgs @{} `
                -IsDryRun $true `
                -DependencyRunner $depsRunner `
                -CommandTester $commandTester `
                -NvimRunner $nvimRunner
        } 6>&1 | Out-String) -replace "`r", ''

        $script:SetupUpdateDepsArgs['Update'] | Should -BeTrue
        $script:SetupUpdateDepsArgs['DryRun'] | Should -BeTrue
        $script:SetupUpdateRuntimeRefreshed | Should -BeFalse
        $output | Should -Match '(?m)^\s*would:\s+nvim --headless \+MasonToolsUpdate \+qa[ \t]*$'
        Should -Invoke -CommandName Invoke-ChezmoiApplyPhase -Times 0 -Exactly
    }
}

Describe "setup.ps1 VS developer environment" {
    BeforeEach {
        . $script:ImportSetupForTest
    }

    It "imports the VS DevShell when a VC toolset is present" {
        $script:CheckedDevShellPath = ''
        $script:ImportedDevShellPath = ''
        $script:EnteredVsPath = ''

        $result = Enter-VsDeveloperEnvironment `
            -OnWindows:$true `
            -InstallationPathResolver { return 'C:\VS' } `
            -ModulePathTester {
                param([string]$Path)
                $script:CheckedDevShellPath = $Path
                return $true
            } `
            -ModuleImporter {
                param([string]$Path)
                $script:ImportedDevShellPath = $Path
            } `
            -DevShellInvoker {
                param([string]$InstallPath)
                $script:EnteredVsPath = $InstallPath
            }

        $result | Should -BeTrue
        $script:CheckedDevShellPath | Should -Match 'Microsoft\.VisualStudio\.DevShell\.dll$'
        $script:ImportedDevShellPath | Should -Be $script:CheckedDevShellPath
        $script:EnteredVsPath | Should -Be 'C:\VS'
    }

    It "attempts the VS environment before Lazy restore" {
        $script:NvimSyncEvents = @()
        $commandTester = {
            param([string]$Name)
            return ($Name -eq 'nvim')
        }
        $devEnvironment = {
            $script:NvimSyncEvents += 'dev-env'
            return $true
        }
        $lazyRunner = {
            $script:NvimSyncEvents += 'lazy'
            $global:LASTEXITCODE = 0
        }
        $treesitterRunner = {
            $script:NvimSyncEvents += 'treesitter'
            $global:LASTEXITCODE = 0
        }
        $masonRunner = {
            $script:NvimSyncEvents += 'mason'
            $global:LASTEXITCODE = 0
        }

        Invoke-NvimSyncPhases `
            -CommandTester $commandTester `
            -DevEnvironmentEntrypoint $devEnvironment `
            -LazyRunner $lazyRunner `
            -TreesitterRunner $treesitterRunner `
            -MasonRunner $masonRunner

        ($script:NvimSyncEvents -join ',') | Should -Be 'dev-env,lazy,treesitter,mason'
    }

    It "emits a FAIL marker when a present VC toolset cannot import DevShell" {
        $output = & {
            Enter-VsDeveloperEnvironment `
                -OnWindows:$true `
                -InstallationPathResolver { return 'C:\VS' } `
                -ModulePathTester { return $false }
        } 6>&1 | Out-String

        $output | Should -Match 'FAIL: VS DevShell module missing'
    }
}

Describe "setup.ps1 Windows Terminal backup" {
    BeforeEach {
        $script:OldLocalAppData = $env:LOCALAPPDATA
        $script:OldUserProfile = $env:USERPROFILE
        $script:FakeHome = Join-Path ([System.IO.Path]::GetTempPath()) ("setup-wt-" + [System.Guid]::NewGuid())
        $script:FakeLocalAppData = Join-Path $script:FakeHome 'AppData\Local'
        New-Item -ItemType Directory -Force -Path $script:FakeLocalAppData | Out-Null
        $env:USERPROFILE = $script:FakeHome
        $env:LOCALAPPDATA = $script:FakeLocalAppData
        . $script:ImportSetupForTest
    }

    AfterEach {
        if ($null -eq $script:OldUserProfile) {
            Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
        } else {
            $env:USERPROFILE = $script:OldUserProfile
        }
        if ($null -eq $script:OldLocalAppData) {
            Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue
        } else {
            $env:LOCALAPPDATA = $script:OldLocalAppData
        }
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:FakeHome
    }

    It "copies the pre-merge settings.json without moving it" {
        $settings = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        $preMerge = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        [System.IO.File]::WriteAllText($settings, $preMerge, [System.Text.UTF8Encoding]::new($false))

        Backup-WindowsTerminalSettings

        # The backup name uses the runtime $Timestamp; match by pattern rather
        # than a fixed value (Pester scoping makes pinning $Timestamp unreliable).
        $backups = @(Get-ChildItem -LiteralPath (Split-Path -Parent $settings) -Filter 'settings.json.bak.*' -ErrorAction SilentlyContinue)
        Test-Path -LiteralPath $settings -PathType Leaf | Should -BeTrue   # original intact (copy, not move)
        $backups.Count | Should -BeGreaterThan 0                            # a backup was created
        [System.IO.File]::ReadAllText($settings) | Should -Be $preMerge
        [System.IO.File]::ReadAllText($backups[0].FullName) | Should -Be $preMerge
    }

    It "does not create a backup when Windows Terminal merge is skipped" {
        . $script:ImportSetupForTest -Parameters @{ All = $true; SkipWindowsTerminalMerge = $true }
        $settings = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        [System.IO.File]::WriteAllText($settings, '{"profiles":{}}', [System.Text.UTF8Encoding]::new($false))

        Backup-WindowsTerminalSettings

        $backups = @(Get-ChildItem -LiteralPath (Split-Path -Parent $settings) -Filter 'settings.json.bak.*' -ErrorAction SilentlyContinue)
        $backups.Count | Should -Be 0
    }

    It "mirrors packaged Windows Terminal settings to the unpackaged path" {
        $settings = Get-WindowsTerminalSettingsPath
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        $merged = '{"theme":"rose-pine","profiles":{"defaults":{"scrollbarState":"visible"}}}'
        [System.IO.File]::WriteAllText($settings, $merged, [System.Text.UTF8Encoding]::new($false))

        Copy-WindowsTerminalSettingsForUnpackaged

        Test-Path -LiteralPath $unpackaged -PathType Leaf | Should -BeTrue
        [System.IO.File]::ReadAllText($unpackaged) | Should -Be $merged
    }

    It "seeds unpackaged Windows Terminal settings from the fragment when packaged settings are absent" {
        $settings = Get-WindowsTerminalSettingsPath
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath

        Test-Path -LiteralPath $settings -PathType Leaf | Should -BeFalse
        Copy-WindowsTerminalSettingsForUnpackaged -IsPortablePresent $true

        Test-Path -LiteralPath $unpackaged -PathType Leaf | Should -BeTrue
        $written = [System.IO.File]::ReadAllText($unpackaged) | ConvertFrom-Json
        $written.theme | Should -Be 'rose-pine'
        $written.profiles.defaults.colorScheme | Should -Be 'rose-pine'
        $written.profiles.defaults.font.face | Should -Be 'Hack Nerd Font'
        $written.defaultProfile | Should -Be $script:ManagedPwshProfileGuid
        $pwshProfile = @($written.profiles.list | Where-Object { $_.guid -eq $script:ManagedPwshProfileGuid })
        $pwshProfile.Count | Should -Be 1
        $pwshProfile[0].commandline | Should -Be 'pwsh.exe'
        @($written.schemes | Where-Object { $_.name -eq 'rose-pine' }).Count | Should -Be 1
        @($written.themes | Where-Object { $_.name -eq 'rose-pine' }).Count | Should -Be 1
    }

    It "promotes the legacy Windows PowerShell default to the managed PowerShell 7 profile" {
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $unpackaged) | Out-Null
        $minimal = '{"defaultProfile":"{61c54bbd-c2c6-5271-96e7-009a87ff44bf}","profiles":{"defaults":{},"list":[{"guid":"{61c54bbd-c2c6-5271-96e7-009a87ff44bf}","name":"Windows PowerShell","commandline":"powershell.exe"}]},"schemes":[],"actions":[]}'
        [System.IO.File]::WriteAllText($unpackaged, $minimal, [System.Text.UTF8Encoding]::new($false))

        Copy-WindowsTerminalSettingsForUnpackaged -IsPortablePresent $true

        $written = [System.IO.File]::ReadAllText($unpackaged) | ConvertFrom-Json
        $written.defaultProfile | Should -Be $script:ManagedPwshProfileGuid
        @($written.profiles.list | Where-Object { $_.guid -eq $script:LegacyWindowsPowerShellProfileGuid }).Count | Should -Be 1
        $pwshProfile = @($written.profiles.list | Where-Object { $_.guid -eq $script:ManagedPwshProfileGuid })
        $pwshProfile.Count | Should -Be 1
        $pwshProfile[0].name | Should -Be 'PowerShell 7'
        $pwshProfile[0].commandline | Should -Be 'pwsh.exe'
    }

    It "merges unpackaged Windows Terminal settings from the fragment and preserves user keys" {
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $unpackaged) | Out-Null
        $minimal = '{"defaultProfile":"{user}","profiles":{"defaults":{"font":{"face":"Consolas"}},"list":[{"guid":"{user}","name":"Keep"}]},"schemes":[{"name":"KeepScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
        [System.IO.File]::WriteAllText($unpackaged, $minimal, [System.Text.UTF8Encoding]::new($false))

        Copy-WindowsTerminalSettingsForUnpackaged -IsPortablePresent $true

        $written = [System.IO.File]::ReadAllText($unpackaged) | ConvertFrom-Json
        $written.defaultProfile | Should -Be '{user}'
        @($written.profiles.list | Where-Object { $_.guid -eq '{user}' }).Count | Should -Be 1
        @($written.profiles.list | Where-Object { $_.guid -eq $script:ManagedPwshProfileGuid }).Count | Should -Be 1
        @($written.schemes | Where-Object { $_.name -eq 'KeepScheme' }).Count | Should -Be 1
        @($written.actions | Where-Object { $_.keys -eq 'alt+f4' }).Count | Should -Be 1
        $written.theme | Should -Be 'rose-pine'
        $written.profiles.defaults.colorScheme | Should -Be 'rose-pine'
        $written.profiles.defaults.font.face | Should -Be 'Hack Nerd Font'
        @($written.schemes | Where-Object { $_.name -eq 'rose-pine' }).Count | Should -Be 1
        @($written.themes | Where-Object { $_.name -eq 'rose-pine' }).Count | Should -Be 1
    }

    It "keeps setup best effort when the unpackaged Windows Terminal mirror fails" {
        $settings = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        [System.IO.File]::WriteAllText($settings, '{"theme":"rose-pine"}', [System.Text.UTF8Encoding]::new($false))
        Mock -CommandName Copy-Item -MockWith { throw "copy failed" }

        { Copy-WindowsTerminalSettingsForUnpackaged } | Should -Not -Throw
    }

    It "does not mirror unpackaged Windows Terminal settings during dry run" {
        $settings = Get-WindowsTerminalSettingsPath
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        [System.IO.File]::WriteAllText($settings, '{"theme":"rose-pine"}', [System.Text.UTF8Encoding]::new($false))

        # Inject the switch directly -- Set-Variable -Scope Script does not reach
        # the dot-sourced function reliably.
        Copy-WindowsTerminalSettingsForUnpackaged -IsDryRun $true

        Test-Path -LiteralPath $unpackaged -PathType Leaf | Should -BeFalse
    }

    It "does not seed unpackaged Windows Terminal settings during dry run" {
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath

        Copy-WindowsTerminalSettingsForUnpackaged -IsDryRun $true -IsPortablePresent $true

        Test-Path -LiteralPath $unpackaged -PathType Leaf | Should -BeFalse
    }

    It "does not mirror unpackaged Windows Terminal settings when the merge is skipped" {
        $settings = Get-WindowsTerminalSettingsPath
        $unpackaged = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        [System.IO.File]::WriteAllText($settings, '{"theme":"rose-pine"}', [System.Text.UTF8Encoding]::new($false))

        Copy-WindowsTerminalSettingsForUnpackaged -IsSkipMerge $true

        Test-Path -LiteralPath $unpackaged -PathType Leaf | Should -BeFalse
    }
}
