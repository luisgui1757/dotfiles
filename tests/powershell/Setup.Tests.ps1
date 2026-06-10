BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Setup = Join-Path $script:RepoRoot "setup.ps1"

    $script:ImportSetupForTest = {
        $oldSourceOnly = $env:DOTFILES_SETUP_PS1_SOURCE_ONLY
        try {
            $env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
            . $script:Setup -All
        } finally {
            if ($null -eq $oldSourceOnly) {
                Remove-Item Env:DOTFILES_SETUP_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
            } else {
                $env:DOTFILES_SETUP_PS1_SOURCE_ONLY = $oldSourceOnly
            }
        }
    }
}

Describe "setup.ps1 Update-RuntimePath" {
    BeforeEach {
        $script:OldPath = $env:PATH
        $script:OldScoop = $env:SCOOP
        $script:OldUserProfile = $env:USERPROFILE
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
        Set-Variable -Name Timestamp -Scope Script -Value '20000101-000000'
        Set-Variable -Name DryRun -Scope Script -Value $false
        Set-Variable -Name SkipWindowsTerminalMerge -Scope Script -Value $false
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
        Set-Variable -Name SkipWindowsTerminalMerge -Scope Script -Value $true
        $settings = Get-WindowsTerminalSettingsPath
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settings) | Out-Null
        [System.IO.File]::WriteAllText($settings, '{"profiles":{}}', [System.Text.UTF8Encoding]::new($false))

        Backup-WindowsTerminalSettings

        Test-Path -LiteralPath "$settings.bak.20000101-000000" | Should -BeFalse
    }
}
