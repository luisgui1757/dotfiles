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
        $script:ContentTestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("setup content " + [System.Guid]::NewGuid())
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

    It "treats verify drift as false and restores native preference <Preference>" -TestCases @(
        @{ Preference = $true },
        @{ Preference = $false }
    ) {
        param([bool]$Preference)
        $originalPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $Preference
            Test-ChezmoiVerify $script:ProbeTarget | Should -BeTrue
            $PSNativeCommandUseErrorActionPreference | Should -Be $Preference
            $LASTEXITCODE | Should -Be 0

            [System.IO.File]::WriteAllText($script:ProbeTarget, 'divergent user bytes')
            Test-ChezmoiVerify $script:ProbeTarget | Should -BeFalse
            $PSNativeCommandUseErrorActionPreference | Should -Be $Preference
            $LASTEXITCODE | Should -Be 0
        } finally {
            $PSNativeCommandUseErrorActionPreference = $originalPreference
        }
    }

    It "keeps real verify invocation failures fatal and restores native preference" {
        $oldBaseArgs = $script:ChezmoiBaseArgs
        $originalPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $script:ChezmoiBaseArgs = @('--source', (Join-Path $script:ContentTestRoot 'missing source'))
            $PSNativeCommandUseErrorActionPreference = $true
            { Test-ChezmoiVerify $script:ProbeTarget } | Should -Throw '*verify invocation failed*missing source*'
            $PSNativeCommandUseErrorActionPreference | Should -BeTrue
            $LASTEXITCODE | Should -Be 0
        } finally {
            $script:ChezmoiBaseArgs = $oldBaseArgs
            $PSNativeCommandUseErrorActionPreference = $originalPreference
        }
    }
}

Describe "setup.ps1 main chezmoi apply boundary" {
    BeforeEach {
        . $script:ImportSetupForTest
        $script:ApplyBoundaryOldIdentity = $script:WindowsIdentity
        $script:DryRun = $false
        $script:WindowsIdentity = [pscustomobject]@{
            UserProfile = 'C:\User'
            LocalApplicationData = 'D:\Local Data'
            ApplicationData = 'D:\Roaming Data'
            Documents = 'E:\Documents'
            RuntimeProfile = 'E:\Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        }
        Mock -CommandName Get-Command -ParameterFilter { $Name -eq 'chezmoi' } -MockWith {
            [pscustomobject]@{ Source = 'C:\tools\chezmoi.exe' }
        }
        Mock -CommandName Test-CanCreateSymlinks -MockWith { $true }
        Mock -CommandName Backup-PreexistingManagedTargets
        Mock -CommandName Invoke-WindowsKnownFolderOverlays
        Mock -CommandName Invoke-WindowsTerminalSettingsTransaction
        Mock -CommandName Invoke-PiThemeSelectionMerge
        Mock -CommandName Invoke-ChezmoiOrExit
    }

    AfterEach {
        $script:WindowsIdentity = $script:ApplyBoundaryOldIdentity
        $script:DryRun = $false
    }

    It "applies the complete source without non-portable absolute target selectors" {
        Invoke-ChezmoiApplyPhase

        Should -Invoke Invoke-ChezmoiOrExit -Times 1 -ParameterFilter {
            $Label -eq 'chezmoi apply' -and
            ($Arguments -join '|') -eq '--no-tty|--force|apply'
        }
        Should -Invoke Invoke-PiThemeSelectionMerge -Times 1 -ParameterFilter { -not $ExcludeScripts }
    }

    It "limits release migration apply to files and symlinks without init" {
        Invoke-ChezmoiApplyPhase -ExcludeScripts $true

        Should -Invoke Invoke-ChezmoiOrExit -Times 0 -ParameterFilter {
            $Label -eq 'chezmoi init'
        }
        Should -Invoke Invoke-ChezmoiOrExit -Times 1 -ParameterFilter {
            $Label -eq 'chezmoi apply' -and
            ($Arguments -join '|') -eq '--no-tty|--force|apply|--include|files,symlinks'
        }
        Should -Invoke Invoke-PiThemeSelectionMerge -Times 1 -ParameterFilter { $ExcludeScripts }
    }

    It "keeps the files and symlinks boundary in release migration previews" {
        Mock -CommandName New-ChezmoiDryRunConfig -MockWith { 'C:\temporary\chezmoi.toml' }

        Invoke-ChezmoiApplyPhase -ExcludeScripts $true -IsDryRun $true

        Should -Invoke Invoke-ChezmoiOrExit -Times 1 -ParameterFilter {
            $Label -eq 'chezmoi dry-run apply' -and
            ($Arguments -join '|') -eq '--dry-run|--verbose|apply|--include|files,symlinks'
        }
        Should -Invoke Invoke-WindowsKnownFolderOverlays -Times 1 -ParameterFilter {
            $IsDryRun -and $ExcludeScripts
        }
    }
}

Describe "setup.ps1 Pi theme selection" -Skip:(-not (Get-Command node -ErrorAction SilentlyContinue)) {
    BeforeEach {
        . $script:ImportSetupForTest
        $script:PiThemeRoot = Join-Path ([IO.Path]::GetTempPath()) ("setup-pi-theme-" + [guid]::NewGuid())
        $script:PiThemeIdentity = [pscustomobject]@{ UserProfile = $script:PiThemeRoot }
        $themeRoot = Join-Path $script:PiThemeRoot '.pi\agent\themes'
        New-Item -ItemType Directory -Force -Path $themeRoot | Out-Null
        foreach ($themeName in @(
                'rose-pine', 'rose-pine-moon', 'rose-pine-dawn'
            )) {
            Copy-Item -LiteralPath (Join-Path $script:RepoRoot "pi\$themeName.json") `
                -Destination (Join-Path $themeRoot "$themeName.json")
        }
    }

    AfterEach {
        Remove-Item -LiteralPath $script:PiThemeRoot -Recurse -Force -ErrorAction SilentlyContinue
    }

    It "merges only theme into existing global Pi settings" {
        $settings = Join-Path $script:PiThemeRoot '.pi\agent\settings.json'
        [IO.File]::WriteAllText($settings, '{"theme":"dark","provider":"keep","nested":{"keep":true}}')

        Invoke-PiThemeSelectionMerge -Identity $script:PiThemeIdentity

        $result = Get-Content -Raw -LiteralPath $settings | ConvertFrom-Json
        $result.theme | Should -Be 'rose-pine'
        $result.provider | Should -Be 'keep'
        $result.nested.keep | Should -BeTrue
    }

    It "preserves every managed variant across setup reruns" {
        $settings = Join-Path $script:PiThemeRoot '.pi\agent\settings.json'
        foreach ($themeName in @(
                'rose-pine', 'rose-pine-moon', 'rose-pine-dawn'
            )) {
            [IO.File]::WriteAllText($settings, "{`"theme`":`"$themeName`",`"keep`":true}")

            Invoke-PiThemeSelectionMerge -Identity $script:PiThemeIdentity

            $result = Get-Content -Raw -LiteralPath $settings | ConvertFrom-Json
            $result.theme | Should -Be $themeName
            $result.keep | Should -BeTrue
        }
    }

    It "retires only content-identical trial aliases across checkout line endings" {
        $themeRoot = Join-Path $script:PiThemeRoot '.pi\agent\themes'
        $aliases = [ordered]@{
            'rose-pine-fable' = 'rose-pine'
            'rose-pine-moon-fable' = 'rose-pine-moon'
            'rose-pine-dawn-fable' = 'rose-pine-dawn'
        }
        foreach ($aliasName in $aliases.Keys) {
            $canonicalName = $aliases[$aliasName]
            $canonical = [IO.File]::ReadAllText((Join-Path $script:RepoRoot "pi\$canonicalName.json"))
            $retired = $canonical.Replace(
                "`"name`": `"$canonicalName`"",
                "`"name`": `"$aliasName`""
            )
            [IO.File]::WriteAllText((Join-Path $themeRoot "$aliasName.json"), $retired)
        }
        $modified = Join-Path $themeRoot 'rose-pine-fable.json'
        [IO.File]::AppendAllText($modified, "`n")

        Invoke-PiThemeSelectionMerge -Identity $script:PiThemeIdentity

        Test-Path -LiteralPath $modified | Should -BeTrue
        Test-Path -LiteralPath (Join-Path $themeRoot 'rose-pine-moon-fable.json') | Should -BeFalse
        Test-Path -LiteralPath (Join-Path $themeRoot 'rose-pine-dawn-fable.json') | Should -BeFalse
    }
}

Describe "setup.ps1 native chezmoi failure reporting" {
    It "surfaces captured stderr and preserves the native exit code" {
        $probe = Join-Path ([System.IO.Path]::GetTempPath()) ("setup-chezmoi-failure-" + [System.Guid]::NewGuid() + ".ps1")
        $probeContent = (@'
$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '__SETUP_PATH__'
function Invoke-ChezmoiNative {
    param([string[]]$Arguments, [switch]$PassThroughOutput)
    [pscustomobject]@{
        ExitCode = 23
        Stderr = 'fatal chezmoi detail from redirected stderr'
        Output = @()
    }
}
Invoke-ChezmoiOrExit -Label 'chezmoi apply' -Arguments @('apply')
'@).Replace('__SETUP_PATH__', $script:Setup.Replace("'", "''"))
        [System.IO.File]::WriteAllText($probe, $probeContent, [System.Text.UTF8Encoding]::new($false))

        $originalPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $false
            $currentPowerShell = (Get-Process -Id $PID).Path
            $output = & $currentPowerShell -NoLogo -NoProfile -File $probe 2>&1
            $exitCode = $LASTEXITCODE
        } finally {
            $PSNativeCommandUseErrorActionPreference = $originalPreference
            Remove-Item -LiteralPath $probe -Force -ErrorAction SilentlyContinue
        }

        $exitCode | Should -Be 23
        ($output | Out-String) | Should -Match 'fatal chezmoi detail from redirected stderr'
        ($output | Out-String) | Should -Match 'FAIL: chezmoi apply exited 23'
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

    It "uses the resolved profile root when SCOOP is absent" {
        Remove-Item Env:SCOOP -ErrorAction SilentlyContinue
        Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
        $env:HOME = $script:FakeHome
        $shimDir = Join-Path $script:FakeHome 'scoop/shims'
        New-Item -ItemType Directory -Force -Path $shimDir | Out-Null
        $fakeOne = Join-Path $script:FakeHome 'one'
        $env:PATH = "$fakeOne;$fakeOne"

        Update-RuntimePath -ProfileRootResolver { $script:FakeHome }

        $parts = $env:PATH -split ';'
        $parts[0] | Should -Be $shimDir
        @($parts | Where-Object { $_ -eq $fakeOne }).Count | Should -Be 1
    }

    It "uses the platform's canonical profile source when environment variables are absent" {
        Remove-Item Env:SCOOP -ErrorAction SilentlyContinue
        Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue
        $env:HOME = $script:FakeHome

        if ($env:OS -eq 'Windows_NT') {
            Get-DefaultProfileRoot | Should -Be ([Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile))
            Get-DefaultProfileRoot | Should -Not -Be $script:FakeHome
        } else {
            Get-DefaultProfileRoot | Should -Be $script:FakeHome
        }
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

Describe "setup.ps1 Windows known-folder identity" {
    BeforeEach {
        $script:KnownFolderOldUserProfile = $env:USERPROFILE
        $script:KnownFolderOldLocalAppData = $env:LOCALAPPDATA
        . $script:ImportSetupForTest
    }

    AfterEach {
        if ($null -eq $script:KnownFolderOldUserProfile) { Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue }
        else { $env:USERPROFILE = $script:KnownFolderOldUserProfile }
        if ($null -eq $script:KnownFolderOldLocalAppData) { Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue }
        else { $env:LOCALAPPDATA = $script:KnownFolderOldLocalAppData }
    }

    It "resolves redirected folders, alternate drives, spaces, and runtime profile independently" {
        $env:USERPROFILE = 'C:\Conventional User'
        $env:LOCALAPPDATA = 'C:\Conventional User\AppData\Local'
        $resolver = {
            param([string]$Name)
            switch ($Name) {
                'UserProfile' { 'D:\Actual User With Spaces' }
                'LocalApplicationData' { 'E:\Redirected Local Data' }
                'ApplicationData' { 'G:\Redirected Roaming Data' }
                'MyDocuments' { 'F:\OneDrive - Example\Documents' }
            }
        }
        $identity = Resolve-WindowsTargetIdentity -FolderResolver $resolver `
            -RuntimeProfile 'F:\OneDrive - Example\Documents\PowerShell\Microsoft.VSCode_profile.ps1'

        $identity.UserProfile | Should -Be 'D:\Actual User With Spaces'
        $identity.LocalApplicationData | Should -Be 'E:\Redirected Local Data'
        $identity.ApplicationData | Should -Be 'G:\Redirected Roaming Data'
        $identity.Documents | Should -Be 'F:\OneDrive - Example\Documents'
        $identity.RuntimeProfile | Should -Be 'F:\OneDrive - Example\Documents\PowerShell\Microsoft.VSCode_profile.ps1'
        $identity.UserProfile | Should -Not -Be $env:USERPROFILE
        $identity.LocalApplicationData | Should -Not -Be $env:LOCALAPPDATA
    }

    It "rejects missing or relative known-folder and runtime-profile identities" {
        $goodResolver = { param([string]$Name) "C:\$Name" }
        { Resolve-WindowsTargetIdentity -FolderResolver { param($Name) if ($Name -eq 'MyDocuments') { '' } else { "C:\$Name" } } -RuntimeProfile 'C:\profile.ps1' } |
            Should -Throw '*MyDocuments known folder*'
        { Resolve-WindowsTargetIdentity -FolderResolver { param($Name) if ($Name -eq 'LocalApplicationData') { 'relative' } else { "C:\$Name" } } -RuntimeProfile 'C:\profile.ps1' } |
            Should -Throw '*LocalApplicationData known folder*'
        { Resolve-WindowsTargetIdentity -FolderResolver { param($Name) if ($Name -eq 'ApplicationData') { 'relative' } else { "C:\$Name" } } -RuntimeProfile 'C:\profile.ps1' } |
            Should -Throw '*ApplicationData known folder*'
        { Resolve-WindowsTargetIdentity -FolderResolver $goodResolver -RuntimeProfile 'relative-profile.ps1' } |
            Should -Throw '*runtime profile path*'
    }

    It "maps each overlay to the actual application destination" {
        $root = Join-Path ([IO.Path]::GetTempPath()) 'overlay mapping root'
        $identity = [pscustomobject]@{
            UserProfile = Join-Path $root 'Actual User'
            LocalApplicationData = Join-Path $root 'Redirected Local Data'
            ApplicationData = Join-Path $root 'Redirected Roaming Data'
            Documents = Join-Path $root 'OneDrive Documents'
            RuntimeProfile = Join-Path $root 'OneDrive Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        }
        $overlays = @(Get-WindowsKnownFolderOverlays -Identity $identity)

        $overlays.Count | Should -Be 3
        $overlays[0].Destination | Should -Be $identity.LocalApplicationData
        $overlays[1].Destination | Should -Be $identity.ApplicationData
        $overlays[2].Destination | Should -Be $identity.Documents
        $overlays[0].State | Should -Match 'localappdata\.boltdb$'
        $overlays[1].State | Should -Match 'appdata\.boltdb$'
        $overlays[2].State | Should -Match 'documents\.boltdb$'
    }

    It "post-checks actual Neovim, lazygit, Herdr, Console, VS Code, and ISE consumers" {
        $root = Join-Path ([IO.Path]::GetTempPath()) ('known folders ' + [Guid]::NewGuid())
        $localAppData = Join-Path $root 'Redirected Local Data'
        $appData = Join-Path $root 'Redirected Roaming Data'
        $documents = Join-Path $root 'OneDrive Documents'
        $nvimTarget = Join-Path $localAppData 'nvim'
        $lazygitTarget = Join-Path $localAppData 'lazygit\config.yml'
        $herdrTarget = Join-Path $appData 'herdr\config.toml'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $lazygitTarget), (Split-Path -Parent $herdrTarget) | Out-Null
        if ($env:OS -eq 'Windows_NT') {
            New-Item -ItemType Junction -Path $nvimTarget -Target (Join-Path $script:RepoRoot 'nvim') | Out-Null
        } else {
            New-Item -ItemType SymbolicLink -Path $nvimTarget -Target (Join-Path $script:RepoRoot 'nvim') | Out-Null
        }
        Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'lazygit\config.windows.yml') -Destination $lazygitTarget
        Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'herdr\config.windows.toml') -Destination $herdrTarget
        $profiles = @(
            (Join-Path $documents 'PowerShell\Microsoft.PowerShell_profile.ps1'),
            (Join-Path $documents 'PowerShell\Microsoft.VSCode_profile.ps1'),
            (Join-Path $documents 'WindowsPowerShell\Microsoft.PowerShell_profile.ps1'),
            (Join-Path $documents 'WindowsPowerShell\Microsoft.PowerShellISE_profile.ps1')
        )
        try {
            foreach ($profilePath in $profiles) {
                New-Item -ItemType Directory -Force -Path (Split-Path -Parent $profilePath) | Out-Null
                Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'shells\powershell_profile.ps1') -Destination $profilePath
            }
            foreach ($profilePath in $profiles) {
                $identity = [pscustomobject]@{
                    UserProfile = $root
                    LocalApplicationData = $localAppData
                    ApplicationData = $appData
                    Documents = $documents
                    RuntimeProfile = $profilePath
                }
                { Assert-WindowsKnownFolderConsumption -Identity $identity } | Should -Not -Throw
            }
        } finally {
            Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "unblocks only validated repo-owned PowerShell profile consumers" {
        $root = Join-Path $TestDrive 'profile trust publication'
        $documents = Join-Path $root 'Documents'
        $expected = Join-Path $root 'powershell_profile.ps1'
        $runtimeProfile = Join-Path $documents 'PowerShell\Microsoft.PowerShell_profile.ps1'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $runtimeProfile) | Out-Null
        [System.IO.File]::WriteAllText($expected, 'exit 0', [System.Text.UTF8Encoding]::new($false))
        New-Item -ItemType SymbolicLink -Path $runtimeProfile -Target $expected | Out-Null
        $identity = [pscustomobject]@{
            UserProfile = $root
            LocalApplicationData = Join-Path $root 'Local'
            ApplicationData = Join-Path $root 'Roaming'
            Documents = $documents
            RuntimeProfile = $runtimeProfile
        }
        $script:UnblockedProfilePaths = @()

        Unblock-WindowsPowerShellProfile -Identity $identity -ExpectedProfile $expected -Unblocker {
            param([string]$Path)
            $script:UnblockedProfilePaths += $Path
        }

        $script:UnblockedProfilePaths | Should -HaveCount 1
        $script:UnblockedProfilePaths[0] | Should -Be ([System.IO.Path]::GetFullPath($expected))

        if ($env:OS -eq 'Windows_NT') {
            Set-Content -LiteralPath $expected -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=3"
            (Get-Item -LiteralPath $expected -Stream Zone.Identifier -ErrorAction Stop) | Should -Not -BeNullOrEmpty

            Unblock-WindowsPowerShellProfile -Identity $identity -ExpectedProfile $expected

            Get-Item -LiteralPath $expected -Stream Zone.Identifier -ErrorAction SilentlyContinue |
                Should -BeNullOrEmpty
            $pwsh = (Get-Command pwsh -ErrorAction Stop).Source
            & $pwsh -NoProfile -ExecutionPolicy RemoteSigned -File $runtimeProfile
            $LASTEXITCODE | Should -Be 0
        }
    }

    It "refuses to unblock a divergent PowerShell profile" {
        $root = Join-Path $TestDrive 'divergent profile trust'
        $documents = Join-Path $root 'Documents'
        $expected = Join-Path $root 'powershell_profile.ps1'
        $runtimeProfile = Join-Path $documents 'PowerShell\Microsoft.PowerShell_profile.ps1'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $runtimeProfile) | Out-Null
        [System.IO.File]::WriteAllText($expected, 'repo profile', [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::WriteAllText($runtimeProfile, 'user profile', [System.Text.UTF8Encoding]::new($false))
        $identity = [pscustomobject]@{
            UserProfile = $root
            LocalApplicationData = Join-Path $root 'Local'
            ApplicationData = Join-Path $root 'Roaming'
            Documents = $documents
            RuntimeProfile = $runtimeProfile
        }
        $script:UnblockedProfilePaths = @()

        {
            Unblock-WindowsPowerShellProfile -Identity $identity -ExpectedProfile $expected -Unblocker {
                param([string]$Path)
                $script:UnblockedProfilePaths += $Path
            }
        } | Should -Throw '*not repo-owned*'
        $script:UnblockedProfilePaths | Should -BeNullOrEmpty
    }

    It "backs up recognized legacy-shape targets but preserves divergent legacy user data" {
        $root = Join-Path ([IO.Path]::GetTempPath()) ('legacy shape ' + [Guid]::NewGuid())
        $identity = [pscustomobject]@{
            UserProfile = Join-Path $root 'User'
            LocalApplicationData = Join-Path $root 'Redirected Local'
            ApplicationData = Join-Path $root 'Redirected Roaming'
            Documents = Join-Path $root 'Redirected Documents'
            RuntimeProfile = Join-Path $root 'Redirected Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        }
        $legacyNvim = Join-Path $identity.UserProfile 'AppData\Local\nvim'
        $legacyLazygit = Join-Path $identity.UserProfile 'AppData\Local\lazygit\config.yml'
        $legacyProfile = Join-Path $identity.UserProfile 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $legacyNvim), (Split-Path -Parent $legacyLazygit), (Split-Path -Parent $legacyProfile) | Out-Null
        if ($env:OS -eq 'Windows_NT') {
            New-Item -ItemType Junction -Path $legacyNvim -Target (Join-Path $script:RepoRoot 'nvim') | Out-Null
        } else {
            New-Item -ItemType SymbolicLink -Path $legacyNvim -Target (Join-Path $script:RepoRoot 'nvim') | Out-Null
        }
        Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'lazygit\config.windows.yml') -Destination $legacyLazygit
        [IO.File]::WriteAllText($legacyProfile, 'user-owned divergent profile')
        try {
            Move-LegacyWindowsKnownFolderTargets -Identity $identity -IsDryRun:$false

            Test-Path -LiteralPath $legacyNvim | Should -BeFalse
            Test-Path -LiteralPath $legacyLazygit | Should -BeFalse
            @(Get-ChildItem -LiteralPath (Split-Path -Parent $legacyNvim) -Filter 'nvim.legacy.*').Count | Should -Be 1
            @(Get-ChildItem -LiteralPath (Split-Path -Parent $legacyLazygit) -Filter 'config.yml.legacy.*').Count | Should -Be 1
            [IO.File]::ReadAllText($legacyProfile) | Should -Be 'user-owned divergent profile'
        } finally {
            Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "retains conventional v0.1 targets while release rollback authority is open" {
        $root = Join-Path ([IO.Path]::GetTempPath()) ('legacy retained ' + [Guid]::NewGuid())
        $identity = [pscustomobject]@{
            UserProfile = Join-Path $root 'User'
            LocalApplicationData = Join-Path $root 'Redirected Local'
            ApplicationData = Join-Path $root 'Redirected Roaming'
            Documents = Join-Path $root 'Redirected Documents'
            RuntimeProfile = Join-Path $root 'Redirected Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        }
        $legacyLazygit = Join-Path $identity.UserProfile 'AppData\Local\lazygit\config.yml'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $legacyLazygit) | Out-Null
        Copy-Item -LiteralPath (Join-Path $script:RepoRoot 'lazygit\config.windows.yml') -Destination $legacyLazygit
        try {
            Move-LegacyWindowsKnownFolderTargets -Identity $identity -IsDryRun:$false -IsSuppressed:$true

            Test-Path -LiteralPath $legacyLazygit -PathType Leaf | Should -BeTrue
            @(Get-ChildItem -LiteralPath (Split-Path -Parent $legacyLazygit) -Filter 'config.yml.legacy.*').Count | Should -Be 0
        } finally {
            Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

Describe "setup.ps1 Sentinel agent policy" {
    BeforeEach {
        . $script:ImportSetupForTest
        $script:SentinelTestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("setup-sentinel-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path $script:SentinelTestRoot | Out-Null
    }

    AfterEach {
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:SentinelTestRoot
    }

    function script:New-SetupTestSentinelRepo {
        param(
            [string]$Name = ('sentinel-work-' + [System.Guid]::NewGuid().ToString('N')),
            [string]$Version = '0.1.2',
            [string]$Installer = @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$SENTINEL_TEST_LOG"
'@
        )

        $work = Join-Path $script:SentinelTestRoot $Name
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "$Version`n", [System.Text.UTF8Encoding]::new($false))
        $installerPath = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installerPath, $Installer, [System.Text.UTF8Encoding]::new($false))
        if ($env:OS -ne 'Windows_NT') {
            & chmod +x $installerPath
        }
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake sentinel'
        $sha = (& git -C $work rev-parse HEAD).Trim()
        return [pscustomobject]@{
            Work = $work
            Installer = $installerPath
            Sha = $sha
        }
    }

    function script:Invoke-SetupTestSentinelPolicyChild {
        param(
            [Parameter(Mandatory)] [string]$Cache,
            [Parameter(Mandatory)] [string]$Ref,
            [string]$RepoUrl = ''
        )

        $setupLiteral = $script:Setup.Replace("'", "''")
        $cacheLiteral = $Cache.Replace("'", "''")
        $refLiteral = $Ref.Replace("'", "''")
        $repoArgument = if ([string]::IsNullOrWhiteSpace($RepoUrl)) {
            ''
        } else {
            " -RepoUrl '$($RepoUrl.Replace("'", "''"))'"
        }
        $probe = @"
`$ErrorActionPreference = 'Stop'
`$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '$setupLiteral' -All
Invoke-SentinelAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.2' -Ref '$refLiteral' -CacheRoot '$cacheLiteral'$repoArgument
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

    It "previews the pinned Sentinel global install in dry-run mode" {
        $cache = Join-Path $script:SentinelTestRoot 'cache'
        $output = & {
            Invoke-SentinelAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$true `
                -Version '0.1.2' `
                -Ref '489dcc6f991ddcff63c460a433e983264dc54cf7' `
                -CacheRoot $cache
        } 6>&1 | Out-String

        $output | Should -Match 'Phase 6/6: apply global agent policy \(Sentinel\)'
        $output | Should -Match 'would\s+clone/fetch Sentinel 0\.1\.2'
        $output | Should -Match '489dcc6f991ddcff63c460a433e983264dc54cf7'
        $output | Should -Match 'tools/install --global'
        Test-Path -LiteralPath $cache | Should -BeFalse
    }

    It "honors -SkipAgents" {
        $output = & {
            Invoke-SentinelAgentPolicy -SkipAgentsPhase:$true -AllMode:$true
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
            $gitRoot = Join-Path $script:SentinelTestRoot 'Git'
            $script:ExpectedSentinelGitBash = Join-Path $gitRoot 'bin\bash.exe'
            $pathBash = Join-Path $script:SentinelTestRoot 'System32\bash.exe'

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
            Mock -CommandName Test-SentinelGitBashCommand -MockWith {
                param([string]$Candidate)
                return ($Candidate -eq $script:ExpectedSentinelGitBash)
            }

            Get-SentinelBashCommand | Should -Be $script:ExpectedSentinelGitBash
        } finally {
            Remove-Variable -Name ExpectedSentinelGitBash -Scope Script -ErrorAction SilentlyContinue
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
            $pathBash = Join-Path $script:SentinelTestRoot 'System32\bash.exe'

            Mock -CommandName Get-Command -MockWith {
                param([string]$Name)
                if ($Name -eq 'bash') {
                    return [pscustomobject]@{ Source = $pathBash }
                }
                return $null
            }
            Mock -CommandName Test-SentinelGitBashCommand -MockWith { return $false }

            Get-SentinelBashCommand | Should -BeNullOrEmpty
        } finally {
            if ($null -eq $oldOs) {
                Remove-Item Env:OS -ErrorAction SilentlyContinue
            } else {
                $env:OS = $oldOs
            }
        }
    }

    It "runs the pinned Sentinel installer and global check from a verified checkout" {
        $work = Join-Path $script:SentinelTestRoot 'sentinel-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "PATH=%s\n" "$PATH" >> "$SENTINEL_TEST_LOG"
printf "%s\n" "$*" >> "$SENTINEL_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake sentinel'
        $sha = (& git -C $work rev-parse HEAD).Trim()

        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'sentinel-install.log'
            Invoke-SentinelAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.2' `
                -Ref $sha `
                -CacheRoot $cache

            $calls = Get-Content -LiteralPath $env:SENTINEL_TEST_LOG
            $calls | Should -Contain '--global'
            $calls | Should -Contain '--global --check'
            if ($env:OS -eq 'Windows_NT') {
                $calls | Should -Contain 'PATH=/usr/bin:/bin'
            }
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "fetches Sentinel without executing ambient Git config or template hooks" {
        $repo = New-SetupTestSentinelRepo -Name 'sentinel-fresh-work'
        $cache = Join-Path $script:SentinelTestRoot 'fresh-cache'
        $oldLog = $env:SENTINEL_TEST_LOG
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

        $globalMarker = Join-Path $script:SentinelTestRoot 'fresh-global-fsmonitor-ran'
        $envMarker = Join-Path $script:SentinelTestRoot 'fresh-env-fsmonitor-ran'
        $templateMarker = Join-Path $script:SentinelTestRoot 'fresh-template-post-checkout-ran'
        if ($env:OS -eq 'Windows_NT') {
            $globalFsmonitor = Join-Path $script:SentinelTestRoot 'fresh-global-fsmonitor.cmd'
            $envFsmonitor = Join-Path $script:SentinelTestRoot 'fresh-env-fsmonitor.cmd'
            [System.IO.File]::WriteAllText($globalFsmonitor, "@echo off`r`necho ran> `"$globalMarker`"`r`nexit /b 0`r`n", [System.Text.UTF8Encoding]::new($false))
            [System.IO.File]::WriteAllText($envFsmonitor, "@echo off`r`necho ran> `"$envMarker`"`r`nexit /b 0`r`n", [System.Text.UTF8Encoding]::new($false))
        } else {
            $globalFsmonitor = Join-Path $script:SentinelTestRoot 'fresh-global-fsmonitor'
            $envFsmonitor = Join-Path $script:SentinelTestRoot 'fresh-env-fsmonitor'
            [System.IO.File]::WriteAllText($globalFsmonitor, "#!/usr/bin/env bash`nprintf ran > '$globalMarker'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
            [System.IO.File]::WriteAllText($envFsmonitor, "#!/usr/bin/env bash`nprintf ran > '$envMarker'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
            & chmod +x $globalFsmonitor $envFsmonitor
        }

        $globalConfig = Join-Path $script:SentinelTestRoot 'fresh-hostile.gitconfig'
        $globalFsmonitorForGit = $globalFsmonitor -replace '\\', '/'
        [System.IO.File]::WriteAllText($globalConfig, "[core]`n`tfsmonitor = $globalFsmonitorForGit`n", [System.Text.UTF8Encoding]::new($false))

        $templateDir = Join-Path $script:SentinelTestRoot 'fresh-template'
        $templateHooks = Join-Path $templateDir 'hooks'
        New-Item -ItemType Directory -Force -Path $templateHooks | Out-Null
        $templateMarkerForGit = $templateMarker -replace '\\', '/'
        $postCheckout = Join-Path $templateHooks 'post-checkout'
        [System.IO.File]::WriteAllText($postCheckout, "#!/usr/bin/env sh`nprintf ran > '$templateMarkerForGit'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
        if ($env:OS -ne 'Windows_NT') {
            & chmod +x $postCheckout
        }

        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'fresh-sentinel-install.log'
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_GLOBAL', $globalConfig, 'Process')
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_COUNT', '1', 'Process')
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_KEY_0', 'core.fsmonitor', 'Process')
            [Environment]::SetEnvironmentVariable('GIT_CONFIG_VALUE_0', ($envFsmonitor -replace '\\', '/'), 'Process')
            [Environment]::SetEnvironmentVariable('GIT_TEMPLATE_DIR', $templateDir, 'Process')

            Invoke-SentinelAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.2' `
                -Ref $repo.Sha `
                -RepoUrl $repo.Work `
                -CacheRoot $cache

            Test-Path -LiteralPath $globalMarker | Should -BeFalse
            Test-Path -LiteralPath $envMarker | Should -BeFalse
            Test-Path -LiteralPath $templateMarker | Should -BeFalse
            $calls = Get-Content -LiteralPath $env:SENTINEL_TEST_LOG
            $calls | Should -Contain '--global'
            $calls | Should -Contain '--global --check'
        } finally {
            foreach ($name in $gitEnvNames) {
                [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process')
            }
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects a dirty verified Sentinel checkout before running the installer" {
        $work = Join-Path $script:SentinelTestRoot 'sentinel-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$SENTINEL_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake sentinel'
        $sha = (& git -C $work rev-parse HEAD).Trim()

        Add-Content -LiteralPath $installer -Value '# dirty cache regression'
        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'dirty-install.log'
            $setupLiteral = $script:Setup.Replace("'", "''")
            $cacheLiteral = $cache.Replace("'", "''")
            $shaLiteral = $sha.Replace("'", "''")
            $probe = @"
`$ErrorActionPreference = 'Stop'
`$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '$setupLiteral' -All
Invoke-SentinelAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.2' -Ref '$shaLiteral' -CacheRoot '$cacheLiteral'
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
            $output | Should -Match 'Sentinel cache has local changes'
            Test-Path -LiteralPath $env:SENTINEL_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects an untracked file in a verified Sentinel checkout before running the installer" {
        $repo = New-SetupTestSentinelRepo -Name 'sentinel-untracked-work'
        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        $checkout = Join-Path $cache $repo.Sha
        Move-Item -LiteralPath $repo.Work -Destination $checkout
        [System.IO.File]::WriteAllText((Join-Path $checkout 'UNTRACKED'), "dirty`n", [System.Text.UTF8Encoding]::new($false))

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'untracked-install.log'
            $result = Invoke-SetupTestSentinelPolicyChild -Cache $cache -Ref $repo.Sha

            $result.ExitCode | Should -Not -Be 0
            $result.Output | Should -Match 'Sentinel cache has local changes'
            Test-Path -LiteralPath $env:SENTINEL_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects a cached Sentinel checkout whose VERSION is wrong" {
        $repo = New-SetupTestSentinelRepo -Name 'sentinel-wrong-version-work' -Version '0.1.1'
        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $repo.Work -Destination (Join-Path $cache $repo.Sha)

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'wrong-version-install.log'
            $result = Invoke-SetupTestSentinelPolicyChild -Cache $cache -Ref $repo.Sha

            $result.ExitCode | Should -Not -Be 0
            $result.Output | Should -Match 'Sentinel cache VERSION mismatch'
            Test-Path -LiteralPath $env:SENTINEL_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "cleans a failed Sentinel stage and retries after artifact identity is repaired" {
        $repo = New-SetupTestSentinelRepo -Name 'sentinel-stage-retry-work' -Version '0.1.1'
        $cache = Join-Path $script:SentinelTestRoot 'stage-retry-cache'
        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'stage-retry-install.log'
            $failed = Invoke-SetupTestSentinelPolicyChild -Cache $cache -Ref $repo.Sha -RepoUrl $repo.Work
            $failed.ExitCode | Should -Not -Be 0
            $failed.Output | Should -Match 'Sentinel cache VERSION mismatch'
            @(Get-ChildItem -LiteralPath $cache -Filter '.tmp.*' -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
            Test-Path -LiteralPath (Join-Path $cache $repo.Sha) | Should -BeFalse

            [System.IO.File]::WriteAllText((Join-Path $repo.Work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
            & git -C $repo.Work add VERSION
            & git -C $repo.Work commit -q -m 'repair fake sentinel version'
            $correctedSha = (& git -C $repo.Work rev-parse HEAD).Trim()
            $retried = Invoke-SetupTestSentinelPolicyChild -Cache $cache -Ref $correctedSha -RepoUrl $repo.Work
            $retried.ExitCode | Should -Be 0
            Test-Path -LiteralPath (Join-Path (Join-Path $cache $correctedSha) '.git') | Should -BeTrue
            @(Get-ChildItem -LiteralPath $cache -Filter '.tmp.*' -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
        } finally {
            if ($null -eq $oldLog) { Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue }
            else { $env:SENTINEL_TEST_LOG = $oldLog }
        }
    }

    It "rejects an ignored file in a verified Sentinel checkout before running the installer" {
        $repo = New-SetupTestSentinelRepo -Name 'sentinel-ignored-work'
        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        $checkout = Join-Path $cache $repo.Sha
        Move-Item -LiteralPath $repo.Work -Destination $checkout
        $gitInfo = Join-Path $checkout '.git/info'
        New-Item -ItemType Directory -Force -Path $gitInfo | Out-Null
        Add-Content -LiteralPath (Join-Path $gitInfo 'exclude') -Value 'IGNORED'
        [System.IO.File]::WriteAllText((Join-Path $checkout 'IGNORED'), "dirty`n", [System.Text.UTF8Encoding]::new($false))

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'ignored-install.log'
            $result = Invoke-SetupTestSentinelPolicyChild -Cache $cache -Ref $repo.Sha

            $result.ExitCode | Should -Not -Be 0
            $result.Output | Should -Match 'Sentinel cache has local changes'
            Test-Path -LiteralPath $env:SENTINEL_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "does not execute a Sentinel cache core.fsmonitor command during validation" {
        $work = Join-Path $script:SentinelTestRoot 'sentinel-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, @'
#!/usr/bin/env bash
printf "%s\n" "$*" >> "$SENTINEL_TEST_LOG"
'@, [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake sentinel'
        $sha = (& git -C $work rev-parse HEAD).Trim()

        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)
        $checkout = Join-Path $cache $sha
        $marker = Join-Path $script:SentinelTestRoot 'fsmonitor-ran'
        if ($env:OS -eq 'Windows_NT') {
            $fsmonitor = Join-Path $script:SentinelTestRoot 'fsmonitor.cmd'
            [System.IO.File]::WriteAllText($fsmonitor, "@echo off`r`necho ran> `"$marker`"`r`nexit /b 0`r`n", [System.Text.UTF8Encoding]::new($false))
        } else {
            $fsmonitor = Join-Path $script:SentinelTestRoot 'fsmonitor'
            [System.IO.File]::WriteAllText($fsmonitor, "#!/usr/bin/env bash`nprintf ran > '$marker'`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
            & chmod +x $fsmonitor
        }
        & git -C $checkout config core.fsmonitor $fsmonitor

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'fsmonitor-install.log'
            Invoke-SentinelAgentPolicy `
                -AllMode:$true `
                -IsDryRun:$false `
                -Version '0.1.2' `
                -Ref $sha `
                -CacheRoot $cache

            Test-Path -LiteralPath $marker | Should -BeFalse
            $calls = Get-Content -LiteralPath $env:SENTINEL_TEST_LOG
            $calls | Should -Contain '--global'
            $calls | Should -Contain '--global --check'
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }

    It "rejects a cache whose core.worktree points at a clean alternate tree" {
        $work = Join-Path $script:SentinelTestRoot 'sentinel-work'
        $tools = Join-Path $work 'tools'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        & git -C $work init -q
        & git -C $work config user.name 'Dotfiles Test'
        & git -C $work config user.email 'dotfiles@example.invalid'
        [System.IO.File]::WriteAllText((Join-Path $work 'VERSION'), "0.1.2`n", [System.Text.UTF8Encoding]::new($false))
        $installer = Join-Path $tools 'install'
        [System.IO.File]::WriteAllText($installer, "#!/usr/bin/env bash`nexit 0`n", [System.Text.UTF8Encoding]::new($false))
        & git -C $work add VERSION tools/install
        & git -C $work commit -q -m 'fake sentinel'
        $sha = (& git -C $work rev-parse HEAD).Trim()

        $cleanWorktree = Join-Path $script:SentinelTestRoot 'clean-worktree'
        New-Item -ItemType Directory -Force -Path (Join-Path $cleanWorktree 'tools') | Out-Null
        Copy-Item -LiteralPath (Join-Path $work 'VERSION') -Destination (Join-Path $cleanWorktree 'VERSION')
        Copy-Item -LiteralPath $installer -Destination (Join-Path (Join-Path $cleanWorktree 'tools') 'install')

        $marker = Join-Path $script:SentinelTestRoot 'core-worktree-dirty-installer-ran'
        [System.IO.File]::WriteAllText($installer, "#!/usr/bin/env bash`nprintf ran > '$marker'`n", [System.Text.UTF8Encoding]::new($false))
        & git -C $work config core.worktree $cleanWorktree

        $cache = Join-Path $script:SentinelTestRoot 'cache'
        New-Item -ItemType Directory -Force -Path $cache | Out-Null
        Move-Item -LiteralPath $work -Destination (Join-Path $cache $sha)

        $oldLog = $env:SENTINEL_TEST_LOG
        try {
            $env:SENTINEL_TEST_LOG = Join-Path $script:SentinelTestRoot 'core-worktree-install.log'
            $setupLiteral = $script:Setup.Replace("'", "''")
            $cacheLiteral = $cache.Replace("'", "''")
            $shaLiteral = $sha.Replace("'", "''")
            $probe = @"
`$ErrorActionPreference = 'Stop'
`$env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
. '$setupLiteral' -All
Invoke-SentinelAgentPolicy -AllMode:`$true -IsDryRun:`$false -Version '0.1.2' -Ref '$shaLiteral' -CacheRoot '$cacheLiteral'
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
            $output | Should -Match 'Sentinel cache has local changes'
            Test-Path -LiteralPath $marker | Should -BeFalse
            Test-Path -LiteralPath $env:SENTINEL_TEST_LOG | Should -BeFalse
        } finally {
            if ($null -eq $oldLog) {
                Remove-Item Env:SENTINEL_TEST_LOG -ErrorAction SilentlyContinue
            } else {
                $env:SENTINEL_TEST_LOG = $oldLog
            }
        }
    }
}

Describe "setup.ps1 universal install and migration entrypoint" {
    BeforeEach {
        . $script:ImportSetupForTest
        $script:UniversalRoot = Join-Path ([IO.Path]::GetTempPath()) ('setup universal ' + [Guid]::NewGuid())
        $script:UniversalLocal = Join-Path $script:UniversalRoot 'Local Data'
        $script:UniversalOld = Join-Path $script:UniversalRoot 'dotfiles-v0.1.0'
        $script:UniversalRecovery = Join-Path $script:UniversalRoot 'recovery'
        New-Item -ItemType Directory -Force -Path $script:UniversalLocal, $script:UniversalOld, $script:UniversalRecovery | Out-Null
        $script:UniversalIdentity = [pscustomobject]@{
            UserProfile = (Join-Path $script:UniversalRoot 'User')
            LocalApplicationData = $script:UniversalLocal
            ApplicationData = (Join-Path $script:UniversalRoot 'Roaming')
            Documents = (Join-Path $script:UniversalRoot 'Documents')
            RuntimeProfile = (Join-Path $script:UniversalRoot 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1')
        }
        $script:UniversalCalls = @()
        $script:UniversalOldOverride = $env:DOTFILES_V0_1_CHECKOUT
        $env:DOTFILES_V0_1_CHECKOUT = $script:UniversalOld
        $script:CompletedV01Recovery = ''
    }

    AfterEach {
        if ($null -eq $script:UniversalOldOverride) {
            Remove-Item Env:DOTFILES_V0_1_CHECKOUT -ErrorAction SilentlyContinue
        } else {
            $env:DOTFILES_V0_1_CHECKOUT = $script:UniversalOldOverride
        }
        Remove-Item -LiteralPath $script:UniversalRoot -Recurse -Force -ErrorAction SilentlyContinue
    }

    It "accepts Upgrade as an alias for Update and makes update non-interactive" {
        (Get-Command $script:Setup).Parameters['Update'].Aliases | Should -Contain 'Upgrade'

        . $script:ImportSetupForTest -Parameters @{ Update = $true }

        $Update | Should -BeTrue
        $All | Should -BeTrue
        $depsArgs.ContainsKey('Update') | Should -BeFalse
        $depsArgs['All'] | Should -BeTrue
    }

    It "applies then accepts an exact v0.1.0 installation from setup All" {
        $gitRunner = {
            param([string]$Path, [string[]]$Arguments)
            $null = $Path
            if (($Arguments -join ' ') -match 'HEAD\^\{commit\}') { return $V01Commit }
            return $V01TagObject
        }
        $migrationRunner = {
            param([string]$Mode, [string]$Argument)
            $script:UniversalCalls += "$Mode|$Argument"
            if ($Mode -eq 'Apply') { return "Recovery directory: $($script:UniversalRecovery)" }
            return 'accepted fixture migration'
        }

        Invoke-SetupV01Migration `
            -Identity $script:UniversalIdentity `
            -AllMode $true `
            -MigrationRunner $migrationRunner `
            -GitRunner $gitRunner

        $script:UniversalCalls | Should -Be @(
            "Apply|$($script:UniversalOld)",
            "Accept|$($script:UniversalRecovery)"
        )
        $script:CompletedV01Recovery | Should -Be $script:UniversalRecovery
    }

    It "resumes an already-applied migration at acceptance" {
        Remove-Item Env:DOTFILES_V0_1_CHECKOUT -ErrorAction SilentlyContinue
        $pending = Join-Path (Join-Path (Join-Path $script:UniversalLocal 'dotfiles') 'migrations') 'v0.1.0-to-v0.2.0.pending'
        New-Item -ItemType Directory -Force -Path $pending | Out-Null
        [IO.File]::WriteAllText((Join-Path $pending 'stage'), "applied`n")
        [IO.File]::WriteAllText((Join-Path $pending 'new-checkout'), "$ScriptDir`n")
        [IO.File]::WriteAllText((Join-Path $pending 'old-checkout'), "$($script:UniversalOld)`n")
        $migrationRunner = {
            param([string]$Mode, [string]$Argument)
            $script:UniversalCalls += "$Mode|$Argument"
            return 'accepted pending migration'
        }

        Invoke-SetupV01Migration `
            -Identity $script:UniversalIdentity `
            -AllMode $true `
            -MigrationRunner $migrationRunner

        $script:UniversalCalls | Should -Be @("Accept|$pending")
        $script:CompletedV01Recovery | Should -Be $pending
    }

    It "refuses to cross an unfinished recovery-required boundary" {
        Remove-Item Env:DOTFILES_V0_1_CHECKOUT -ErrorAction SilentlyContinue
        $pending = Join-Path (Join-Path (Join-Path $script:UniversalLocal 'dotfiles') 'migrations') 'v0.1.0-to-v0.2.0.pending'
        New-Item -ItemType Directory -Force -Path $pending | Out-Null
        [IO.File]::WriteAllText((Join-Path $pending 'stage'), "recovery-required`n")
        [IO.File]::WriteAllText((Join-Path $pending 'new-checkout'), "$ScriptDir`n")
        [IO.File]::WriteAllText((Join-Path $pending 'old-checkout'), "$($script:UniversalOld)`n")

        { Invoke-SetupV01Migration -Identity $script:UniversalIdentity -AllMode $true } |
            Should -Throw '*unfinished v0.1.0 migration requires recovery first*'
    }

    It "rejects malformed pending recovery instead of starting another migration" {
        Remove-Item Env:DOTFILES_V0_1_CHECKOUT -ErrorAction SilentlyContinue
        $pending = Join-Path (Join-Path (Join-Path $script:UniversalLocal 'dotfiles') 'migrations') 'v0.1.0-to-v0.2.0.invalid'
        New-Item -ItemType Directory -Force -Path $pending | Out-Null
        [IO.File]::WriteAllText((Join-Path $pending 'stage'), "applied`n")
        [IO.File]::WriteAllText((Join-Path $pending 'new-checkout'), "$ScriptDir`n")

        { Invoke-SetupV01Migration -Identity $script:UniversalIdentity -AllMode $true } |
            Should -Throw '*migration recovery identity is incomplete or unsafe*'
    }

    It "rejects a pending recovery scalar without exact newline framing" {
        Remove-Item Env:DOTFILES_V0_1_CHECKOUT -ErrorAction SilentlyContinue
        $pending = Join-Path (Join-Path (Join-Path $script:UniversalLocal 'dotfiles') 'migrations') 'v0.1.0-to-v0.2.0.invalid'
        New-Item -ItemType Directory -Force -Path $pending | Out-Null
        [IO.File]::WriteAllText((Join-Path $pending 'stage'), 'applied')
        [IO.File]::WriteAllText((Join-Path $pending 'new-checkout'), "$ScriptDir`n")
        [IO.File]::WriteAllText((Join-Path $pending 'old-checkout'), "$($script:UniversalOld)`n")

        { Invoke-SetupV01Migration -Identity $script:UniversalIdentity -AllMode $true } |
            Should -Throw '*migration recovery identity is incomplete or unsafe*'
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
        ($script:SetupUpdateNvimArgs -join ' ') | Should -Be "--headless +lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
        $script:SetupUpdateRuntimeRefreshed | Should -BeTrue
        $output | Should -Match 'Update 1/2'
        $output | Should -Match 'Update 2/2'
        $output | Should -Match 'checked-out release, pinned plugins, configs, and missing tools were reconciled'
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
        $output | Should -Match "(?m)^\s*would:\s+nvim --headless \+lua require\('util\.mason_tools'\)\.run_checked\('MasonToolsUpdateSync'\)[ \t]*$"
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
        $script:OldWindowsIdentity = $script:WindowsIdentity
        $script:WindowsIdentity = [pscustomobject]@{
            UserProfile = $script:FakeHome
            LocalApplicationData = $script:FakeLocalAppData
            ApplicationData = Join-Path $script:FakeHome 'AppData\Roaming'
            Documents = Join-Path $script:FakeHome 'Documents'
            RuntimeProfile = Join-Path $script:FakeHome 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        }
    }

    AfterEach {
        $script:WindowsIdentity = $script:OldWindowsIdentity
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
        $written.profiles.defaults.historySize | Should -Be 32767
        $written.defaultProfile | Should -Be $script:ManagedPwshProfileGuid
        $pwshProfile = @($written.profiles.list | Where-Object { $_.guid -eq $script:ManagedPwshProfileGuid })
        $pwshProfile.Count | Should -Be 1
        $pwshProfile[0].commandline | Should -Be 'pwsh.exe'
        @($written.schemes | Where-Object { $_.name -eq 'rose-pine' }).Count | Should -Be 1
        @($written.themes | Where-Object { $_.name -eq 'rose-pine' }).Count | Should -Be 1
    }

    It "merges Preview-only settings as an independent target" {
        $packaged = Get-WindowsTerminalSettingsPath
        $preview = Get-WindowsTerminalPreviewSettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $preview) | Out-Null
        $original = '{"defaultProfile":"{preview}","profiles":{"defaults":{},"list":[{"guid":"{preview}","name":"PreviewOnly"}]},"schemes":[{"name":"PreviewScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
        [System.IO.File]::WriteAllText($preview, $original, [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false

        Test-Path -LiteralPath $packaged | Should -BeFalse
        Test-Path -LiteralPath $portable | Should -BeFalse
        $written = [System.IO.File]::ReadAllText($preview) | ConvertFrom-Json
        @($written.profiles.list | Where-Object { $_.guid -eq '{preview}' }).Count | Should -Be 1
        @($written.schemes | Where-Object { $_.name -eq 'PreviewScheme' }).Count | Should -Be 1
        @($written.actions | Where-Object { $_.keys -eq 'alt+f4' }).Count | Should -Be 1
        $backups = @(Get-ChildItem -LiteralPath (Split-Path -Parent $preview) -Filter 'settings.json.bak.*')
        $backups.Count | Should -Be 1
        [System.IO.File]::ReadAllText($backups[0].FullName) | Should -Be $original
    }

    It "merges Canary-only settings as an independent target" {
        $packaged = Get-WindowsTerminalSettingsPath
        $canary = Get-WindowsTerminalCanarySettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $canary) | Out-Null
        $original = '{"defaultProfile":"{canary}","profiles":{"defaults":{},"list":[{"guid":"{canary}","name":"CanaryOnly"}]},"schemes":[{"name":"CanaryScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
        [System.IO.File]::WriteAllText($canary, $original, [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false

        Test-Path -LiteralPath $packaged | Should -BeFalse
        Test-Path -LiteralPath $portable | Should -BeFalse
        $written = [System.IO.File]::ReadAllText($canary) | ConvertFrom-Json
        $written.profiles.defaults.historySize | Should -Be 32767
        @($written.profiles.list | Where-Object { $_.guid -eq '{canary}' }).Count | Should -Be 1
        @($written.schemes | Where-Object { $_.name -eq 'CanaryScheme' }).Count | Should -Be 1
        @($written.actions | Where-Object { $_.keys -eq 'alt+f4' }).Count | Should -Be 1
        $backups = @(Get-ChildItem -LiteralPath (Split-Path -Parent $canary) -Filter 'settings.json.bak.*')
        $backups.Count | Should -Be 1
        [System.IO.File]::ReadAllText($backups[0].FullName) | Should -Be $original
    }

    It "preserves divergent packaged, Preview, Canary, and portable state without mirroring variants" {
        $packaged = Get-WindowsTerminalSettingsPath
        $preview = Get-WindowsTerminalPreviewSettingsPath
        $canary = Get-WindowsTerminalCanarySettingsPath
        $portable = Get-WindowsTerminalUnpackagedSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged), `
            (Split-Path -Parent $preview), (Split-Path -Parent $canary), `
            (Split-Path -Parent $portable) | Out-Null
        $packagedOriginal = '{"defaultProfile":"{pkg}","profiles":{"defaults":{},"list":[{"guid":"{pkg}","name":"PackagedOnly"}]},"schemes":[{"name":"PackagedScheme"}],"actions":[{"command":"closeWindow","keys":"alt+f4"}]}'
        $previewOriginal = '{"defaultProfile":"{pre}","profiles":{"defaults":{},"list":[{"guid":"{pre}","name":"PreviewOnly"}]},"schemes":[{"name":"PreviewScheme"}],"actions":[{"command":"newTab","keys":"ctrl+shift+8"}]}'
        $canaryOriginal = '{"defaultProfile":"{can}","profiles":{"defaults":{},"list":[{"guid":"{can}","name":"CanaryOnly"}]},"schemes":[{"name":"CanaryScheme"}],"actions":[{"command":"newTab","keys":"ctrl+shift+7"}]}'
        $portableOriginal = '{"defaultProfile":"{port}","profiles":{"defaults":{},"list":[{"guid":"{port}","name":"PortableOnly"}]},"schemes":[{"name":"PortableScheme"}],"actions":[{"command":"newTab","keys":"ctrl+shift+9"}]}'
        [System.IO.File]::WriteAllText($packaged, $packagedOriginal, [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::WriteAllText($preview, $previewOriginal, [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::WriteAllText($canary, $canaryOriginal, [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::WriteAllText($portable, $portableOriginal, [System.Text.UTF8Encoding]::new($false))

        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $true

        $pkg = [System.IO.File]::ReadAllText($packaged) | ConvertFrom-Json
        $pre = [System.IO.File]::ReadAllText($preview) | ConvertFrom-Json
        $can = [System.IO.File]::ReadAllText($canary) | ConvertFrom-Json
        $port = [System.IO.File]::ReadAllText($portable) | ConvertFrom-Json
        @($pkg.profiles.list | Where-Object { $_.guid -eq '{pkg}' }).Count | Should -Be 1
        @($pkg.profiles.list | Where-Object { $_.guid -in @('{pre}', '{can}', '{port}') }).Count | Should -Be 0
        @($pre.profiles.list | Where-Object { $_.guid -eq '{pre}' }).Count | Should -Be 1
        @($pre.profiles.list | Where-Object { $_.guid -in @('{pkg}', '{can}', '{port}') }).Count | Should -Be 0
        @($can.profiles.list | Where-Object { $_.guid -eq '{can}' }).Count | Should -Be 1
        @($can.profiles.list | Where-Object { $_.guid -in @('{pkg}', '{pre}', '{port}') }).Count | Should -Be 0
        @($port.profiles.list | Where-Object { $_.guid -eq '{port}' }).Count | Should -Be 1
        @($port.profiles.list | Where-Object { $_.guid -in @('{pkg}', '{pre}', '{can}') }).Count | Should -Be 0
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.bak.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $preview) -Filter 'settings.json.bak.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $canary) -Filter 'settings.json.bak.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $portable) -Filter 'settings.json.bak.*').Count | Should -Be 1
    }

    It "does nothing when neither installation has a settings target" {
        Invoke-WindowsTerminalSettingsTransaction -IsPortablePresent $false
        Test-Path -LiteralPath (Get-WindowsTerminalSettingsPath) | Should -BeFalse
        Test-Path -LiteralPath (Get-WindowsTerminalPreviewSettingsPath) | Should -BeFalse
        Test-Path -LiteralPath (Get-WindowsTerminalCanarySettingsPath) | Should -BeFalse
        Test-Path -LiteralPath (Get-WindowsTerminalUnpackagedSettingsPath) | Should -BeFalse
    }

    It "fails invalid JSON before changing any selected target" {
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
