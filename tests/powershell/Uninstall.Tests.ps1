BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Uninstall = Join-Path $script:RepoRoot 'uninstall.ps1'
    $script:ImportUninstallForTest = {
        param([hashtable]$Parameters = @{ All = $true })
        $old = $env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY
        try {
            $env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY = '1'
            . $script:Uninstall @Parameters
        } finally {
            if ($null -eq $old) { Remove-Item Env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue }
            else { $env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY = $old }
        }
    }
}

Describe 'uninstall.ps1 backup ordering and Windows Terminal recovery' {
    BeforeEach {
        $script:OldLocalAppData = $env:LOCALAPPDATA
        $script:OldUserProfile = $env:USERPROFILE
        $script:Root = Join-Path ([IO.Path]::GetTempPath()) ('uninstall wt ' + [Guid]::NewGuid())
        $env:USERPROFILE = $script:Root
        $env:LOCALAPPDATA = Join-Path $script:Root 'Redirected Local AppData'
        New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA | Out-Null
        . $script:ImportUninstallForTest
        $script:OldWindowsIdentity = $script:WindowsIdentity
        $script:WindowsIdentity = [pscustomobject]@{
            UserProfile = $script:Root
            LocalApplicationData = $env:LOCALAPPDATA
            Documents = Join-Path $script:Root 'Redirected Documents'
            RuntimeProfile = Join-Path $script:Root 'Redirected Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
        }
    }

    AfterEach {
        $script:WindowsIdentity = $script:OldWindowsIdentity
        if ($null -eq $script:OldLocalAppData) { Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue }
        else { $env:LOCALAPPDATA = $script:OldLocalAppData }
        if ($null -eq $script:OldUserProfile) { Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue }
        else { $env:USERPROFILE = $script:OldUserProfile }
        Remove-Item -LiteralPath $script:Root -Recurse -Force -ErrorAction SilentlyContinue
    }

    It 'enumerates stable, Preview, Canary, and portable targets' {
        $targets = @(Get-WindowsTerminalRecoveryTargets)

        $targets.Count | Should -Be 4
        $targets[0] | Should -Match 'Microsoft\.WindowsTerminal_8wekyb3d8bbwe'
        $targets[1] | Should -Match 'Microsoft\.WindowsTerminalPreview_8wekyb3d8bbwe'
        $targets[2] | Should -Match 'Microsoft\.WindowsTerminalCanary_8wekyb3d8bbwe'
        $targets[3] | Should -Match 'Microsoft[\\/]Windows Terminal'
    }

    It 'rejects a relative LocalApplicationData boundary before enumerating targets' {
        { Get-DotfilesWindowsTerminalTargetDefinitions -LocalApplicationData 'relative\profile' } |
            Should -Throw '*missing or not absolute*'
    }

    It 'selects filename timestamp and collision suffix instead of mtime' {
        $target = Join-Path $script:Root 'config with spaces.json'
        $old = "$target.bak.20260101-010101"
        $new = "$target.bak.20260202-020202"
        $collision = "$target.bak.20260202-020202.2"
        [IO.File]::WriteAllText($old, '{}')
        [IO.File]::WriteAllText($new, '{}')
        [IO.File]::WriteAllText($collision, '{}')
        (Get-Item $old).LastWriteTimeUtc = [DateTime]::UtcNow.AddDays(2)
        (Get-Item $collision).LastWriteTimeUtc = [DateTime]::UtcNow.AddDays(-2)

        Get-NewestBackup -Target $target | Should -Be $collision
    }

    It 'supports directory backups using the same validated filename order' {
        $target = Join-Path $script:Root 'config-dir'
        $older = "$target.bak.20260101-010101"
        $newer = "$target.bak.20260101-010101.1"
        New-Item -ItemType Directory -Force -Path $older, $newer | Out-Null
        (Get-Item $older).LastWriteTimeUtc = [DateTime]::UtcNow.AddDays(1)

        Get-NewestBackup -Target $target | Should -Be $newer
    }

    It 'fails closed when any matching backup name is malformed' {
        $target = Join-Path $script:Root 'settings.json'
        [IO.File]::WriteAllText("$target.bak.20260101-010101", '{}')
        [IO.File]::WriteAllText("$target.bak.latest", '{}')

        { Get-NewestBackup -Target $target } | Should -Throw '*malformed backup candidate*'
    }

    It 'restores packaged, Preview, Canary, and portable settings independently while preserving current bytes' {
        $targets = @(Get-WindowsTerminalRecoveryTargets)
        $packaged = $targets[0]
        $preview = $targets[1]
        $canary = $targets[2]
        $portable = $targets[3]
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged), `
            (Split-Path -Parent $preview), (Split-Path -Parent $canary), `
            (Split-Path -Parent $portable) | Out-Null
        [IO.File]::WriteAllText($packaged, '{"current":"packaged"}')
        [IO.File]::WriteAllText($preview, '{"current":"preview"}')
        [IO.File]::WriteAllText($canary, '{"current":"canary"}')
        [IO.File]::WriteAllText($portable, '{"current":"portable"}')
        $packagedBackup = "$packaged.bak.20260202-020202.1"
        $previewBackup = "$preview.bak.20260203-020202"
        $canaryBackup = "$canary.bak.20260204-020202"
        $portableBackup = "$portable.bak.20260303-030303"
        [IO.File]::WriteAllText($packagedBackup, '{"before":"packaged"}')
        [IO.File]::WriteAllText($previewBackup, '{"before":"preview"}')
        [IO.File]::WriteAllText($canaryBackup, '{"before":"canary"}')
        [IO.File]::WriteAllText($portableBackup, '{"before":"portable"}')
        (Get-Item $packagedBackup).LastWriteTimeUtc = [DateTime]::UtcNow.AddYears(-2)

        Restore-WindowsTerminalSettingsBackups

        [IO.File]::ReadAllText($packaged) | Should -Be '{"before":"packaged"}'
        [IO.File]::ReadAllText($preview) | Should -Be '{"before":"preview"}'
        [IO.File]::ReadAllText($canary) | Should -Be '{"before":"canary"}'
        [IO.File]::ReadAllText($portable) | Should -Be '{"before":"portable"}'
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.uninstall-current.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $preview) -Filter 'settings.json.uninstall-current.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $canary) -Filter 'settings.json.uninstall-current.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $portable) -Filter 'settings.json.uninstall-current.*').Count | Should -Be 1
        [IO.File]::ReadAllText(@(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.uninstall-current.*')[0].FullName) | Should -Be '{"current":"packaged"}'
        [IO.File]::ReadAllText(@(Get-ChildItem -LiteralPath (Split-Path -Parent $preview) -Filter 'settings.json.uninstall-current.*')[0].FullName) | Should -Be '{"current":"preview"}'
        [IO.File]::ReadAllText(@(Get-ChildItem -LiteralPath (Split-Path -Parent $canary) -Filter 'settings.json.uninstall-current.*')[0].FullName) | Should -Be '{"current":"canary"}'
        [IO.File]::ReadAllText(@(Get-ChildItem -LiteralPath (Split-Path -Parent $portable) -Filter 'settings.json.uninstall-current.*')[0].FullName) | Should -Be '{"current":"portable"}'
    }

    It 'validates all four paths before restoring any one' {
        $targets = @(Get-WindowsTerminalRecoveryTargets)
        foreach ($target in $targets) {
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null
            [IO.File]::WriteAllText($target, '{"current":true}')
            [IO.File]::WriteAllText("$target.bak.20260101-010101", '{"before":true}')
        }
        [IO.File]::WriteAllText("$($targets[3]).bak.latest", '{"bad":true}')

        { Restore-WindowsTerminalSettingsBackups } | Should -Throw '*malformed backup candidate*'
        [IO.File]::ReadAllText($targets[0]) | Should -Be '{"current":true}'
        [IO.File]::ReadAllText($targets[1]) | Should -Be '{"current":true}'
        [IO.File]::ReadAllText($targets[2]) | Should -Be '{"current":true}'
        [IO.File]::ReadAllText($targets[3]) | Should -Be '{"current":true}'
    }

    It 'is write-free in dry-run and honors NoRestoreBackups' {
        $target = @(Get-WindowsTerminalRecoveryTargets)[0]
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null
        [IO.File]::WriteAllText($target, '{"current":true}')
        $backup = "$target.bak.20260101-010101"
        [IO.File]::WriteAllText($backup, '{"before":true}')

        $DryRun = $true
        Restore-WindowsTerminalSettingsBackups
        [IO.File]::ReadAllText($target) | Should -Be '{"current":true}'
        Test-Path -LiteralPath $backup | Should -BeTrue

        $DryRun = $false
        $NoRestoreBackups = $true
        Restore-WindowsTerminalSettingsBackups
        [IO.File]::ReadAllText($target) | Should -Be '{"current":true}'
        Test-Path -LiteralPath $backup | Should -BeTrue
    }
}

Describe 'uninstall.ps1 chezmoi native verify semantics' {
    BeforeEach {
        . $script:ImportUninstallForTest
        $script:NativeRoot = Join-Path ([IO.Path]::GetTempPath()) ('uninstall native ' + [Guid]::NewGuid())
        $script:NativeSource = Join-Path $script:NativeRoot 'source with spaces'
        $script:NativeHome = Join-Path $script:NativeRoot 'home with spaces'
        $script:NativeState = Join-Path $script:NativeRoot 'state.db'
        $script:NativeConfig = Join-Path $script:NativeRoot 'empty config.toml'
        New-Item -ItemType Directory -Force -Path $script:NativeSource, $script:NativeHome | Out-Null
        [IO.File]::WriteAllText((Join-Path $script:NativeSource 'dot_probe'), 'managed bytes')
        [IO.File]::WriteAllText($script:NativeConfig, '')
        $script:Chezmoi = (Get-Command chezmoi -ErrorAction Stop).Source
        $script:NativeBaseArgs = @(
            '--source', $script:NativeSource,
            '--destination', $script:NativeHome,
            '--persistent-state', $script:NativeState,
            '--config', $script:NativeConfig,
            '--config-format', 'toml'
        )
        $oldPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $false
            & $script:Chezmoi @script:NativeBaseArgs --no-tty --force apply
            if ($LASTEXITCODE -ne 0) { throw 'could not create chezmoi verify fixture' }
        } finally {
            $PSNativeCommandUseErrorActionPreference = $oldPreference
        }
        $script:NativeTarget = Join-Path $script:NativeHome '.probe'
    }

    AfterEach {
        Remove-Item -LiteralPath $script:NativeRoot -Recurse -Force -ErrorAction SilentlyContinue
    }

    It 'returns false for expected drift and restores native preference <Preference>' -TestCases @(
        @{ Preference = $true },
        @{ Preference = $false }
    ) {
        param([bool]$Preference)
        $originalPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $Preference
            Test-ChezmoiTargetUnmodified -Target $script:NativeTarget -BaseArguments $script:NativeBaseArgs | Should -BeTrue
            $PSNativeCommandUseErrorActionPreference | Should -Be $Preference
            $LASTEXITCODE | Should -Be 0

            [IO.File]::WriteAllText($script:NativeTarget, 'user drift')
            Test-ChezmoiTargetUnmodified -Target $script:NativeTarget -BaseArguments $script:NativeBaseArgs | Should -BeFalse
            $PSNativeCommandUseErrorActionPreference | Should -Be $Preference
            $LASTEXITCODE | Should -Be 0
        } finally {
            $PSNativeCommandUseErrorActionPreference = $originalPreference
        }
    }

    It 'keeps stderr-backed invocation failures fatal and restores preference' {
        $originalPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $true
            $badArgs = @('--source', (Join-Path $script:NativeRoot 'missing source'))
            {
                Test-ChezmoiTargetUnmodified -Target $script:NativeTarget -BaseArguments $badArgs
            } | Should -Throw '*verify invocation failed*missing source*'
            $PSNativeCommandUseErrorActionPreference | Should -BeTrue
            $LASTEXITCODE | Should -Be 0
        } finally {
            $PSNativeCommandUseErrorActionPreference = $originalPreference
        }
    }
}
