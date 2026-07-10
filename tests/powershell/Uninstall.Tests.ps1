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
    }

    AfterEach {
        if ($null -eq $script:OldLocalAppData) { Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue }
        else { $env:LOCALAPPDATA = $script:OldLocalAppData }
        if ($null -eq $script:OldUserProfile) { Remove-Item Env:USERPROFILE -ErrorAction SilentlyContinue }
        else { $env:USERPROFILE = $script:OldUserProfile }
        Remove-Item -LiteralPath $script:Root -Recurse -Force -ErrorAction SilentlyContinue
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

    It 'restores packaged and portable settings independently while preserving current bytes' {
        $targets = @(Get-WindowsTerminalRecoveryTargets)
        $packaged = $targets[0]
        $portable = $targets[1]
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $packaged), (Split-Path -Parent $portable) | Out-Null
        [IO.File]::WriteAllText($packaged, '{"current":"packaged"}')
        [IO.File]::WriteAllText($portable, '{"current":"portable"}')
        $packagedBackup = "$packaged.bak.20260202-020202.1"
        $portableBackup = "$portable.bak.20260303-030303"
        [IO.File]::WriteAllText($packagedBackup, '{"before":"packaged"}')
        [IO.File]::WriteAllText($portableBackup, '{"before":"portable"}')
        (Get-Item $packagedBackup).LastWriteTimeUtc = [DateTime]::UtcNow.AddYears(-2)

        Restore-WindowsTerminalSettingsBackups

        [IO.File]::ReadAllText($packaged) | Should -Be '{"before":"packaged"}'
        [IO.File]::ReadAllText($portable) | Should -Be '{"before":"portable"}'
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.uninstall-current.*').Count | Should -Be 1
        @(Get-ChildItem -LiteralPath (Split-Path -Parent $portable) -Filter 'settings.json.uninstall-current.*').Count | Should -Be 1
        [IO.File]::ReadAllText(@(Get-ChildItem -LiteralPath (Split-Path -Parent $packaged) -Filter 'settings.json.uninstall-current.*')[0].FullName) | Should -Be '{"current":"packaged"}'
        [IO.File]::ReadAllText(@(Get-ChildItem -LiteralPath (Split-Path -Parent $portable) -Filter 'settings.json.uninstall-current.*')[0].FullName) | Should -Be '{"current":"portable"}'
    }

    It 'validates both paths before restoring either one' {
        $targets = @(Get-WindowsTerminalRecoveryTargets)
        foreach ($target in $targets) {
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null
            [IO.File]::WriteAllText($target, '{"current":true}')
            [IO.File]::WriteAllText("$target.bak.20260101-010101", '{"before":true}')
        }
        [IO.File]::WriteAllText("$($targets[1]).bak.latest", '{"bad":true}')

        { Restore-WindowsTerminalSettingsBackups } | Should -Throw '*malformed backup candidate*'
        [IO.File]::ReadAllText($targets[0]) | Should -Be '{"current":true}'
        [IO.File]::ReadAllText($targets[1]) | Should -Be '{"current":true}'
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
