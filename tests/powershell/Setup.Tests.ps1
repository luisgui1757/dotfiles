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
        $env:USERPROFILE = $script:OldUserProfile
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
