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

    function script:New-SetupTestPolarisRepo {
        param(
            [string]$Name = ('polaris-work-' + [System.Guid]::NewGuid().ToString('N')),
            [string]$Installer = @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$POLARIS_TEST_LOG"
'@
        )

        $work = Join-Path $script:PolarisTestRoot $Name
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installerPath = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installerPath, $Installer, [System.Text.UTF8Encoding]::new($false))
        if ($env:OS -ne 'Windows_NT') {
            & chmod +x $installerPath
        }
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()
        & git -C $work tag 'v0.1.2' $sha
        return [pscustomobject]@{
            Work = $work
            Installer = $installerPath
            Sha = $sha
            Tag = 'v0.1.2'
        }
    }

    function script:Invoke-SetupTestPolarisPolicyChild {
        param(
            [Parameter(Mandatory)] [string]$Cache,
            [Parameter(Mandatory)] [string]$Ref
        )

        $setupLiteral = $script:Setup.Replace("'", "''")
        $cacheLiteral = $Cache.Replace("'", "''")
        $refLiteral = $Ref.Replace("'", "''")
        $probe = @"
`$ErrorActionPreference = 'Stop'
`$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '$setupLiteral' -All
Invoke-PolarisAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.2' -Ref '$refLiteral' -CacheRoot '$cacheLiteral'
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
        return [pscustomobject]@{
            ExitCode = $rc
            Output = $output
        }
    }

    It "previews the pinned Polaris global install in dry-run mode" {
        $cache = Join-Path $script:PolarisTestRoot 'cache'
        $output = & {
            Invoke-PolarisAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$true `
                -Version '0.1.2' `
                -Ref '489dcc6f991ddcff63c460a433e983264dc54cf7' `
                -CacheRoot $cache
        } 6>&1 | Out-String

        $output | Should -Match 'Phase 6/6: apply global agent policy \(Polaris\)'
        $output | Should -Match 'would\s+clone/fetch Polaris 0\.1\.2'
        $output | Should -Match 'v0\.1\.2'
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
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "PATH=%s\n" "$PATH" >> "$POLARIS_TEST_LOG"
printf "%s\n" "$*" >> "$POLARIS_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()
        & git -C $work tag 'v0.1.2' $sha

        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'polaris-install.log'
            Invoke-PolarisAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.2' `
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

    It "fetches Polaris without executing ambient Git config or template hooks" {
        $repo = New-SetupTestPolarisRepo -Name 'polaris-fresh-work'
        $cache = Join-Path $script:PolarisTestRoot 'fresh-cache'
        $oldLog = $env:POLARIS_TEST_LOG
        $gitEnvNames = @(
            'GIT_CONFIG_GLOBAL',
            'GIT_CONFIG_COUNT',
            'GIT_CONFIG_KEY_0',
            'GIT_CONFIG_VALUE_0',
            'GIT_TEMPLATE_DIR'
        )
        $savedEnv = @{}
        foreach ($name in $gitEnvNames) {
            $savedEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
        }

        $globalMarker = Join-Path $script:PolarisTestRoot 'fresh-global-fsmonitor-ran'
        $envMarker = Join-Path $script:PolarisTestRoot 'fresh-env-fsmonitor-ran'
        $templateMarker = Join-Path $script:PolarisTestRoot 'fresh-template-post-checkout-ran'
        if ($env:OS -eq 'Windows_NT') {
            $globalFsmonitor = Join-Path $script:PolarisTestRoot 'fresh-global-fsmonitor.cmd'
            $envFsmonitor = Join-Path $script:PolarisTestRoot 'fresh-env-fsmonitor.cmd'
            [System.IO.File]::WriteAllText($globalFsmonitor, "@echo off`r`necho ran> `"$globalMarker`"`r`nexit /b 0`r`n", [System.Text.UTF8Encoding]::new($false))
            [System.IO.File]::WriteAllText($envFsmonitor, "@echo off`r`necho ran> `"$envMarker`"`r`nexit /b 0`r`n", [System.Text.UTF8Encoding]::new($false))
        } else {
            $globalFsmonitor = Join-Path $script:PolarisTestRoot 'fresh-global-fsmonitor'
            $envFsmonitor = Join-Path $script:PolarisTestRoot 'fresh-env-fsmonitor'
            [System.IO.File]::WriteAllText($globalFsmonitor, "#!/usr/bin/env bash`nprintf ran > '$globalMarker'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
            [System.IO.File]::WriteAllText($envFsmonitor, "#!/usr/bin/env bash`nprintf ran > '$envMarker'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
            & chmod +x $globalFsmonitor $envFsmonitor
        }

        $globalConfig = Join-Path $script:PolarisTestRoot 'fresh-hostile.gitconfig'
        $globalFsmonitorForGit = $globalFsmonitor -replace '\\', '/'
        [System.IO.File]::WriteAllText($globalConfig, "[core]`n`tfsmonitor = $globalFsmonitorForGit`n", [System.Text.UTF8Encoding]::new($false))

        $templateDir = Join-Path $script:PolarisTestRoot 'fresh-template'
        $templateHooks = Join-Path $templateDir 'hooks'
        New-Item -ItemType Directory -Force -Path $templateHooks | Out-Null
        $templateMarkerForGit = $templateMarker -replace '\\', '/'
        $postCheckout = Join-Path $templateHooks 'post-checkout'
        [System.IO.File]::WriteAllText($postCheckout, "#!/usr/bin/env sh`nprintf ran > '$templateMarkerForGit'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
        if ($env:OS -ne 'Windows_NT') {
            & chmod +x $postCheckout
        }

        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'fresh-polaris-install.log'
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_GLOBAL', $globalConfig, 'Process')
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_COUNT', '1', 'Process')
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_KEY_0', 'core.fsmonitor', 'Process')
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_VALUE_0', ($envFsmonitor -replace '\\', '/'), 'Process')
            [Environment]::SetEnvironmentVariable('GIT_TEMPLATE_DIR', $templateDir, 'Process')

            Invoke-PolarisAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.2' `
                -Ref $repo.Sha `
                -RepoUrl $repo.Work `
                -CacheRoot $cache

            Test-Path -LiteralPath $globalMarker | Should -BeFalse
            Test-Path -LiteralPath $envMarker | Should -BeFalse
            Test-Path -LiteralPath $templateMarker | Should -BeFalse
            $calls = Get-Content -LiteralPath $env:POLARIS_TEST_LOG
            $calls | Should -Contain '--global'
            $calls | Should -Contain '--global --check'
        } finally {
            foreach ($name in $gitEnvNames) {
                [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process')
            }
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
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$POLARIS_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()
        & git -C $work tag 'v0.1.2' $sha

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
Invoke-PolarisAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.2' -Ref '$shaLiteral' -CacheRoot '$cacheLiteral'
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

    It "rejects an untracked file in a verified Polaris checkout before running the installer" {
        $repo = New-SetupTestPolarisRepo -Name 'polaris-untracked-work'
        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        $checkout = Join-Path $cache $repo.Sha
        Move-Item -LiteralPath $repo.Work -Destination $checkout
        [System.IO.File]::WriteAllText((Join-Path $checkout 'UNTRACKED'), "dirty`n", [System.Text.UTF8Encoding]::new($false))

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'untracked-install.log'
            $result = Invoke-SetupTestPolarisPolicyChild -Cache $cache -Ref $repo.Sha

            $result.ExitCode | Should -Not -Be 0
            $result.Output | Should -Match 'Polaris cache has local changes'
            Test-Path -LiteralPath $env:POLARIS_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:POLARIS_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:POLARIS_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects a cached Polaris checkout whose release tag is missing" {
        $repo = New-SetupTestPolarisRepo -Name 'polaris-untagged-work'
        & git -C $repo.Work tag -d $repo.Tag | Out-Null
        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $repo.Work -Destination (Join-Path $cache $repo.Sha)

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'untagged-install.log'
            $result = Invoke-SetupTestPolarisPolicyChild -Cache $cache -Ref $repo.Sha

            $result.ExitCode | Should -Not -Be 0
            $result.Output | Should -Match 'Polaris tag mismatch'
            Test-Path -LiteralPath $env:POLARIS_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:POLARIS_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:POLARIS_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects an ignored file in a verified Polaris checkout before running the installer" {
        $repo = New-SetupTestPolarisRepo -Name 'polaris-ignored-work'
        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        $checkout = Join-Path $cache $repo.Sha
        Move-Item -LiteralPath $repo.Work -Destination $checkout
        $gitInfo = Join-Path $checkout '.git/info'
        New-Item -ItemType Directory -Force -Path $gitInfo | Out-Null
        Add-Content -LiteralPath (Join-Path $gitInfo 'exclude') -Value 'IGNORED'
        [System.IO.File]::WriteAllText((Join-Path $checkout 'IGNORED'), "dirty`n", [System.Text.UTF8Encoding]::new($false))

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'ignored-install.log'
            $result = Invoke-SetupTestPolarisPolicyChild -Cache $cache -Ref $repo.Sha

            $result.ExitCode | Should -Not -Be 0
            $result.Output | Should -Match 'Polaris cache has local changes'
            Test-Path -LiteralPath $env:POLARIS_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:POLARIS_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:POLARIS_TEST_LOG = $oldLog
            }
        }
    }

    It "does not execute a Polaris cache core.fsmonitor command during validation" {
        $work = Join-Path $script:PolarisTestRoot 'polaris-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$POLARIS_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()
        & git -C $work tag 'v0.1.2' $sha

        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)
        $checkout = Join-Path $cache $sha
        $marker = Join-Path $script:PolarisTestRoot 'fsmonitor-ran'
        if ($env:OS -eq 'Windows_NT') {
            $fsmonitor = Join-Path $script:PolarisTestRoot 'fsmonitor.cmd'
            [System.IO.File]::WriteAllText($fsmonitor, "@echo off`r`necho ran> `"$marker`"`r`nexit /b 0`r`n", [System.Text.UTF8Encoding]::new($false))
        } else {
            $fsmonitor = Join-Path $script:PolarisTestRoot 'fsmonitor'
            [System.IO.File]::WriteAllText($fsmonitor, "#!/usr/bin/env bash`nprintf ran > '$marker'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
            & chmod +x $fsmonitor
        }
        & git -C $checkout config core.fsmonitor $fsmonitor

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'fsmonitor-install.log'
            Invoke-PolarisAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.2' `
                -Ref $sha `
                -CacheRoot $cache

            Test-Path -LiteralPath $marker | Should -BeFalse
            $calls = Get-Content -LiteralPath $env:POLARIS_TEST_LOG
            $calls | Should -Contain '--global'
            $calls | Should -Contain '--global --check'
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:POLARIS_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:POLARIS_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects a cache whose core.worktree points at a clean alternate tree" {
        $work = Join-Path $script:PolarisTestRoot 'polaris-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, "#!/usr/bin/env bash`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake polaris'
        $sha = (& git -C $work rev-parse HEAD).Trim()
        & git -C $work tag 'v0.1.2' $sha

        $cleanWorktree = Join-Path $script:PolarisTestRoot 'clean-worktree'
        New-Item -ItemType Directory -Force -Path (Join-Path $cleanWorktree 'tools') | Out-Null
        Copy-Item -LiteralPath (Join-Path $work 'VERSION') -Destination (Join-Path $cleanWorktree 'VERSION')
        Copy-Item -LiteralPath $installer -Destination (Join-Path (Join-Path $cleanWorktree 'tools') 'install')

        $marker = Join-Path $script:PolarisTestRoot 'core-worktree-dirty-installer-ran'
        [System.IO.File]::WriteAllText($installer, "#!/usr/bin/env bash`nprintf ran > '$marker'`n", [System.Text.UTF8Encoding]::new($false))
        & git -C $work config core.worktree $cleanWorktree

        $cache = Join-Path $script:PolarisTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:POLARIS_TEST_LOG
        try {
            $env:POLARIS_TEST_LOG = Join-Path $script:PolarisTestRoot 'core-worktree-install.log'
            $setupLiteral = $script:Setup.Replace("'", "''")
            $cacheLiteral = $cache.Replace("'", "''")
            $shaLiteral = $sha.Replace("'", "''")
            $probe = @"
`$ErrorActionPreference = 'Stop'
`$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '$setupLiteral' -All
Invoke-PolarisAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.2' -Ref '$shaLiteral' -CacheRoot '$cacheLiteral'
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
            Test-Path -LiteralPath $marker | Should -BeFalse
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
        $script:SetupUpdateNvimArgs = @()
        $script:SetupUpdateRuntimeRefreshed = $false
        Mock -CommandName Update-RuntimePath -MockWith { $script:SetupUpdateRuntimeRefreshed = $true }
        Mock -CommandName Invoke-ChezmoiApplyPhase -MockWith { throw "chezmoi apply must not run in update mode" }
    }

    It "runs install-deps Update and MasonToolsUpdateSync only" {
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
        function global:nvim {
            $script:SetupUpdateNvimRan = $true
            $script:SetupUpdateNvimArgs = $args
            $global:LASTEXITCODE = 0
        }

        try {
            $output = & {
                Invoke-SetupUpdateMode `
                    -Root $root `
                    -DependencyArgs @{} `
                    -DependencyRunner $depsRunner `
                    -CommandTester $commandTester
            } 6>&1 | Out-String
        } finally {
            Remove-Item Function:\global:nvim -ErrorAction SilentlyContinue
        }

        $script:SetupUpdateDepsPath | Should -Be (Join-Path $root 'install-deps.ps1')
        $script:SetupUpdateDepsArgs['Update'] | Should -BeTrue
        $script:SetupUpdateNvimRan | Should -BeTrue
        ($script:SetupUpdateNvimArgs -join ' ') | Should -Be '--headless +MasonToolsUpdateSync +qa'
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
        $output | Should -Match '(?m)^\s*would:\s+nvim --headless \+MasonToolsUpdateSync \+qa[ \t]*$'
        Should -Invoke -CommandName Invoke-ChezmoiApplyPhase -Times 0 -Exactly
    }

    It "propagates install-deps failures in normal dependency phase helper" {
        Mock -CommandName Stop-SetupWithExitCode -MockWith {
            param([int]$ExitCode)
            throw "setup-exit:$ExitCode"
        }
        $depsRunner = {
            param([string]$Path, [hashtable]$Arguments)
            $global:LASTEXITCODE = 23
        }

        { Invoke-DependencyInstallerOrFail -Runner $depsRunner -Path 'C:\dotfiles\install-deps.ps1' -Arguments @{} } |
            Should -Throw 'setup-exit:23'

        Should -Invoke -CommandName Stop-SetupWithExitCode -Times 1 -Exactly -ParameterFilter { $ExitCode -eq 23 }
    }

    It "ignores stale LASTEXITCODE before a successful normal dependency phase helper" {
        Mock -CommandName Stop-SetupWithExitCode -MockWith {
            param([int]$ExitCode)
            throw "setup-exit:$ExitCode"
        }
        $global:LASTEXITCODE = 81
        $depsRunner = {
            param([string]$Path, [hashtable]$Arguments)
            $null = $Path
            $null = $Arguments
        }

        { Invoke-DependencyInstallerOrFail -Runner $depsRunner -Path 'C:\dotfiles\install-deps.ps1' -Arguments @{} } |
            Should -Not -Throw

        $LASTEXITCODE | Should -Be 0
        Should -Invoke -CommandName Stop-SetupWithExitCode -Times 0 -Exactly
    }

    It "propagates install-deps failures in Update mode before Mason update" {
        $root = Join-Path ([System.IO.Path]::GetTempPath()) 'setup-update-root'
        Mock -CommandName Stop-SetupWithExitCode -MockWith {
            param([int]$ExitCode)
            throw "setup-exit:$ExitCode"
        }
        $depsRunner = {
            param([string]$Path, [hashtable]$Arguments)
            $script:SetupUpdateDepsPath = $Path
            $script:SetupUpdateDepsArgs = $Arguments
            $global:LASTEXITCODE = 37
        }
        $nvimRunner = {
            throw "Mason update must not run after dependency update failure"
        }

        {
            Invoke-SetupUpdateMode `
                -Root $root `
                -DependencyArgs @{} `
                -DependencyRunner $depsRunner `
                -NvimRunner $nvimRunner
        } | Should -Throw 'setup-exit:37'

        $script:SetupUpdateDepsArgs['Update'] | Should -BeTrue
        $script:SetupUpdateRuntimeRefreshed | Should -BeFalse
        Should -Invoke -CommandName Stop-SetupWithExitCode -Times 1 -Exactly -ParameterFilter { $ExitCode -eq 37 }
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

Describe "setup.ps1 transactional Windows Terminal merge" {
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

    It "merges packaged-only settings and backs up that target independently" {
        $packaged = Get-WindowsTerminalSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"defaultProfile":"{packaged}","profiles":{"defaults":{},"list":[{"guid":"{packaged}","name":"PackagedOnly"}]},"schemes":[{"name":"PackagedScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false

        Test-Path -LiteralPath $portable | Should -BeFalse
        $written = [System.IO.File]::ReadAllText($packaged) | ConvertFrom-Json
        @($written.profiles.list | Where-Object { $_.guid -eq '{packaged}' }).Count | Should -Be 1
        @($written.schemes | Where-Object { $_.name -eq 'PackagedScheme' }).Count | Should -Be 1
        @($written.actions | Where-Object { $_.keys -eq 'alt+f4' }).Count | Should -Be 1
        $backups = @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.bak.*')
        $backups.Count | Should -Be 1
        [System.IO.File]::ReadAllText($backups[0].FullName) | Should -Be $original
    }

    It "seeds portable-only settings from the fragment" {
        $packaged = Get-WindowsTerminalSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $true

        Test-Path -LiteralPath $packaged | Should -BeFalse
        Test-Path -LiteralPath $portable -PathType Leaf | Should -BeTrue
        $written = [System.IO.File]::ReadAllText($portable) | ConvertFrom-Json
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

    It "preserves divergent packaged and portable state without mirroring either" {
        $packaged = Get-WindowsTerminalSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged), (Split-Path -Parent $portable) | Out-Null
        $packagedOriginal = '{"defaultProfile":"{pkg}","profiles":{"defaults":{},"list":[{"guid":"{pkg}","name":"PackagedOnly"}]},"schemes":[{"name":"PackagedScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
        $portableOriginal = '{"defaultProfile":"{port}","profiles":{"defaults":{},"list":[{"guid":"{port}","name":"PortableOnly"}]},"schemes":[{"name":"PortableScheme"}],"actions":[{"command":"newTab","keys":"ctrl+shift+9"}]}'
        [System.IO.File]::WriteAllText($packaged, $packagedOriginal, [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::WriteAllText($portable, $portableOriginal, [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $true

        $pkg = [System.IO.File]::ReadAllText($packaged) | ConvertFrom-Json
        $port = [System.IO.File]::ReadAllText($portable) | ConvertFrom-Json
        @($pkg.profiles.list | Where-Object { $_.guid -eq '{pkg}' }).Count | Should -Be 1
        @($pkg.profiles.list | Where-Object { $_.guid -eq '{port}' }).Count | Should -Be 0
        @($port.profiles.list | Where-Object { $_.guid -eq '{port}' }).Count | Should -Be 1
        @($port.profiles.list | Where-Object { $_.guid -eq '{pkg}' }).Count | Should -Be 0
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.bak.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $portable) -Filter 'settings.json.bak.*').Count | Should -Be 1
    }

    It "does nothing when neither installation has a settings target" {
        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false
        Test-Path -LiteralPath (Get-WindowsTerminalSettingsPath) | Should -BeFalse
        Test-Path -LiteralPath (Get-WindowsTerminalUnpackagedSettingsPath) | Should -BeFalse
    }

    It "fails invalid JSON before changing either target" {
        $packaged = Get-WindowsTerminalSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged), (Split-Path -Parent $portable) | Out-Null
        [System.IO.File]::WriteAllText($packaged, '{invalid', [System.Text.UTF8Encoding]::new($false))
        $portableOriginal = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        [System.IO.File]::WriteAllText($portable, $portableOriginal, [System.Text.UTF8Encoding]::new($false))

        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $true } | Should -Throw

        [System.IO.File]::ReadAllText($packaged) | Should -Be '{invalid'
        [System.IO.File]::ReadAllText($portable) | Should -Be $portableOriginal
        @(Get-ChildItem -LiteralPath $script:FakeHome -Recurse -Force | Where-Object { $_.Name -match 'dotfiles-(stage|rollback)' }).Count | Should -Be 0
    }

    It "fails closed and cleans staging when the staged write fails" {
        $packaged = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))
        Mock -CommandName Write-WindowsTerminalSettingsJson -MockWith { throw 'write failed' }

        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false } | Should -Throw
        [System.IO.File]::ReadAllText($packaged) | Should -Be $original
        @(Get-ChildItem -LiteralPath $script:FakeHome -Recurse -Force | Where-Object { $_.Name -match 'dotfiles-(stage|rollback)' }).Count | Should -Be 0
    }

    It "fails setup when backup creation fails and keeps the original" {
        $packaged = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))
        Mock -CommandName Copy-Item -MockWith { throw 'backup failed' }

        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false } | Should -Throw
        [System.IO.File]::ReadAllText($packaged) | Should -Be $original
    }

    It "rolls back an earlier target when later publication fails" {
        $packaged = Get-WindowsTerminalSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged), (Split-Path -Parent $portable) | Out-Null
        $packagedOriginal = '{"profiles":{"defaults":{},"list":[{"guid":"{pkg}","name":"Pkg"}]},"actions":[]}'
        $portableOriginal = '{"profiles":{"defaults":{},"list":[{"guid":"{port}","name":"Port"}]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $packagedOriginal, [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::WriteAllText($portable, $portableOriginal, [System.Text.UTF8Encoding]::new($false))
        $script:WtPublishCalls = 0
        Mock -CommandName Publish-WindowsTerminalSettingsStage -MockWith {
            param($StagePath, $TargetPath, $RollbackPath, $TargetExisted)
            $script:WtPublishCalls++
            if ($script:WtPublishCalls -eq 1) {
                [System.IO.File]::Replace($StagePath, $TargetPath, $RollbackPath)
                return
            }
            throw 'publish failed'
        }

        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $true } | Should -Throw

        [System.IO.File]::ReadAllText($packaged) | Should -Be $packagedOriginal
        [System.IO.File]::ReadAllText($portable) | Should -Be $portableOriginal
        @(Get-ChildItem -LiteralPath $script:FakeHome -Recurse -Force | Where-Object { $_.Name -match 'dotfiles-(stage|rollback)' }).Count | Should -Be 0
    }

    It "detects a change before publication without overwriting it" {
        $packaged = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        $concurrent = '{"profiles":{"defaults":{},"list":[{"guid":"{new}","name":"Concurrent"}]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))
        $hook = { param($plans) [System.IO.File]::WriteAllText($plans[0].Target, $concurrent, [System.Text.UTF8Encoding]::new($false)) }

        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false -BeforePublish $hook } | Should -Throw

        [System.IO.File]::ReadAllText($packaged) | Should -Be $concurrent
        @(Get-ChildItem -LiteralPath $script:FakeHome -Recurse -Force | Where-Object { $_.Name -match 'dotfiles-(stage|rollback)' }).Count | Should -Be 0
    }

    It "uses File.Replace rollback bytes to close the final publication race" {
        $packaged = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        $concurrent = '{"profiles":{"defaults":{},"list":[{"guid":"{race}","name":"Race"}]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))
        Mock -CommandName Publish-WindowsTerminalSettingsStage -MockWith {
            param($StagePath, $TargetPath, $RollbackPath, $TargetExisted)
            [System.IO.File]::WriteAllText($TargetPath, $concurrent, [System.Text.UTF8Encoding]::new($false))
            [System.IO.File]::Replace($StagePath, $TargetPath, $RollbackPath)
        }

        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false } | Should -Throw
        [System.IO.File]::ReadAllText($packaged) | Should -Be $concurrent
    }

    It "uses a collision suffix without overwriting an earlier backup" {
        $packaged = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))
        $collision = "$packaged.bak.$Timestamp"
        [System.IO.File]::WriteAllText($collision, 'older-backup', [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false

        [System.IO.File]::ReadAllText($collision) | Should -Be 'older-backup'
        [System.IO.File]::ReadAllText("$collision.1") | Should -Be $original
    }

    It "is write-free in dry-run and skip modes" {
        $packaged = Get-WindowsTerminalSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsDryRun $true -IsPortablePresent $true
        Invoke-WindowsTerminalSettingsTransaction -IsSkipMerge $true -IsPortablePresent $true

        [System.IO.File]::ReadAllText($packaged) | Should -Be $original
        Test-Path -LiteralPath $portable | Should -BeFalse
        @(Get-ChildItem -LiteralPath $script:FakeHome -Recurse -Force | Where-Object { $_.Name -match '(settings\.json\.bak|dotfiles-(stage|rollback))' }).Count | Should -Be 0
    }

    It "retries from a concurrently updated original and then becomes idempotent" {
        $packaged = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged) | Out-Null
        $original = '{"profiles":{"defaults":{},"list":[]},"actions":[]}'
        $retrySource = '{"profiles":{"defaults":{},"list":[{"guid":"{retry}","name":"RetrySource"}]},"actions":[]}'
        [System.IO.File]::WriteAllText($packaged, $original, [System.Text.UTF8Encoding]::new($false))
        $hook = { param($plans) [System.IO.File]::WriteAllText($plans[0].Target, $retrySource, [System.Text.UTF8Encoding]::new($false)) }
        { Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false -BeforePublish $hook } | Should -Throw

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false
        $afterFirstSuccess = [System.IO.File]::ReadAllText($packaged)
        $backupCount = @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.bak.*').Count
        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false

        ([System.IO.File]::ReadAllText($packaged) | ConvertFrom-Json).profiles.list.name | Should -Contain 'RetrySource'
        [System.IO.File]::ReadAllText($packaged) | Should -Be $afterFirstSuccess
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.bak.*').Count | Should -Be $backupCount
    }
}
