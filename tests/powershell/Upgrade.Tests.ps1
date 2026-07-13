BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    . (Join-Path $script:RepoRoot 'scripts/upgrade-v0.1.0.ps1') -SourceOnly

    function Write-TerminalRecoveryFixture {
        param(
            [Parameter(Mandatory)] [string]$Recovery,
            [Parameter(Mandatory)] [string[]]$Paths,
            [Parameter(Mandatory)] [string[]]$Kinds,
            [Parameter(Mandatory)] [byte[][]]$BeforeBytes,
            [Parameter(Mandatory)] [byte[][]]$ExpectedBytes
        )
        if ($Paths.Count -ne 4 -or $Kinds.Count -ne 4 -or
            $BeforeBytes.Count -ne 2 -or $ExpectedBytes.Count -ne 4) {
            throw 'fixture requires four Terminal targets and two original payloads'
        }
        $entries = @()
        for ($index = 0; $index -lt 4; $index++) {
            $existed = $index -lt 2
            if ($existed) {
                [IO.File]::WriteAllBytes((Join-Path $Recovery "wt-$index.before"), $BeforeBytes[$index])
            }
            $entries += [pscustomobject]@{
                Kind = $Kinds[$index]
                Path = $Paths[$index]
                Existed = $existed
                BeforeSha = if ($existed) { Get-BytesSha256 -Bytes $BeforeBytes[$index] } else { '' }
                ExpectedPresent = $true
                ExpectedSha = Get-BytesSha256 -Bytes $ExpectedBytes[$index]
                Backup = "wt-$index.before"
            }
        }
        $entries | ConvertTo-Json -Depth 8 |
            Set-Content -LiteralPath (Join-Path $Recovery 'windows-terminal.json') -Encoding utf8
    }

    function Write-FrozenReleaseFixture {
        param([Parameter(Mandatory)] [string]$Recovery)
        $oldSource = Join-Path $Recovery 'old-release'
        $newSource = Join-Path $Recovery 'new-release'
        New-Item -ItemType Directory -Force -Path $oldSource, $newSource | Out-Null
        [IO.File]::WriteAllText((Join-Path $oldSource 'setup.ps1'), 'old setup')
        [IO.File]::WriteAllText((Join-Path $newSource 'setup.ps1'), 'new setup')
        [IO.File]::WriteAllText((Join-Path $Recovery 'old-release.tar'), 'old archive')
        [IO.File]::WriteAllText((Join-Path $Recovery 'new-release.tar'), 'new archive')
        Write-FrozenReleaseTreeManifest -Root $oldSource `
            -Path (Join-Path $Recovery 'old-release.tree')
        Write-FrozenReleaseTreeManifest -Root $newSource `
            -Path (Join-Path $Recovery 'new-release.tree')
        $manifest = [ordered]@{
            Version = 1
            OldCommit = $script:OldCommit
            OldTagObject = $script:OldTagObject
            NewCommit = '1111111111111111111111111111111111111111'
            NewTagObject = '2222222222222222222222222222222222222222'
            OldArchiveSha256 = Get-FileSha256OrEmpty -Path (Join-Path $Recovery 'old-release.tar')
            NewArchiveSha256 = Get-FileSha256OrEmpty -Path (Join-Path $Recovery 'new-release.tar')
            OldTreeManifestSha256 = Get-FileSha256OrEmpty -Path (Join-Path $Recovery 'old-release.tree')
            NewTreeManifestSha256 = Get-FileSha256OrEmpty -Path (Join-Path $Recovery 'new-release.tree')
        }
        [IO.File]::WriteAllText(
            (Join-Path $Recovery 'frozen-releases.json'),
            ($manifest | ConvertTo-Json -Compress)
        )
    }
}

Describe 'v0.1.0 to v0.2.0 Windows release migration recovery' {
    BeforeEach {
        $script:Root = Join-Path ([IO.Path]::GetTempPath()) ('upgrade recovery ' + [guid]::NewGuid())
        $script:Recovery = Join-Path $script:Root 'recovery'
        $script:Target0 = Join-Path $script:Root 'packaged/settings.json'
        $script:Target1 = Join-Path $script:Root 'preview/settings.json'
        $script:Target2 = Join-Path $script:Root 'canary/settings.json'
        $script:Target3 = Join-Path $script:Root 'portable/settings.json'
        $script:Kinds = @('Packaged', 'Preview', 'Canary', 'Portable')
        $script:ExpectedTargets = @(
            [pscustomobject]@{ Kind = $script:Kinds[0]; Path = $script:Target0 },
            [pscustomobject]@{ Kind = $script:Kinds[1]; Path = $script:Target1 },
            [pscustomobject]@{ Kind = $script:Kinds[2]; Path = $script:Target2 },
            [pscustomobject]@{ Kind = $script:Kinds[3]; Path = $script:Target3 }
        )
        New-Item -ItemType Directory -Force -Path $script:Recovery, `
            (Split-Path -Parent $script:Target0), (Split-Path -Parent $script:Target1) | Out-Null
        $script:Before0 = [Text.UTF8Encoding]::new($false).GetBytes('{"before":"packaged"}')
        $script:Before1 = [Text.UTF8Encoding]::new($false).GetBytes('{"before":"preview"}')
        $script:Expected0 = [Text.UTF8Encoding]::new($false).GetBytes('{"expected":"packaged"}')
        $script:Expected1 = [Text.UTF8Encoding]::new($false).GetBytes('{"expected":"preview"}')
        $script:Expected2 = [Text.UTF8Encoding]::new($false).GetBytes('{"expected":"canary"}')
        $script:Expected3 = [Text.UTF8Encoding]::new($false).GetBytes('{"expected":"portable"}')
        [IO.File]::WriteAllBytes($script:Target0, $script:Before0)
        [IO.File]::WriteAllBytes($script:Target1, $script:Before1)
        Write-TerminalRecoveryFixture -Recovery $script:Recovery `
            -Paths @($script:Target0, $script:Target1, $script:Target2, $script:Target3) -Kinds $script:Kinds `
            -BeforeBytes @($script:Before0, $script:Before1) `
            -ExpectedBytes @($script:Expected0, $script:Expected1, $script:Expected2, $script:Expected3)
    }

    AfterEach {
        Remove-Item -LiteralPath $script:Root -Recurse -Force -ErrorAction SilentlyContinue
    }

    It 'restores exact retained bytes and removes only transaction-created targets' {
        $entries = @(Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedTargets $script:ExpectedTargets)
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        [IO.File]::WriteAllBytes($script:Target1, $script:Expected1)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target2) | Out-Null
        [IO.File]::WriteAllBytes($script:Target2, $script:Expected2)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target3) | Out-Null
        [IO.File]::WriteAllBytes($script:Target3, $script:Expected3)

        Restore-WindowsTerminalState -Entries $entries

        [IO.File]::ReadAllBytes($script:Target0) | Should -Be $script:Before0
        [IO.File]::ReadAllBytes($script:Target1) | Should -Be $script:Before1
        Test-Path -LiteralPath $script:Target2 | Should -BeFalse
        Test-Path -LiteralPath $script:Target3 | Should -BeFalse
    }

    It 'captures stable, Preview, Canary, and detected portable identities independently' {
        Initialize-SetupLibrary -NewCheckout $script:RepoRoot
        $captureRecovery = Join-Path $script:Root 'capture recovery'
        New-Item -ItemType Directory -Force -Path $captureRecovery | Out-Null
        $localAppData = Join-Path $script:Root 'Local App Data'
        $script:WindowsIdentity = [pscustomobject]@{
            UserProfile = $script:Root
            LocalApplicationData = $localAppData
            ApplicationData = Join-Path $script:Root 'Roaming App Data'
            Documents = Join-Path $script:Root 'Documents'
            RuntimeProfile = Join-Path $script:Root 'Documents/PowerShell/Microsoft.PowerShell_profile.ps1'
        }
        $definitions = @(& $script:SetupLibraryModule {
                param($LocalApplicationData)
                Get-DotfilesWindowsTerminalTargets -LocalApplicationData $LocalApplicationData -IncludeAbsent
            } $localAppData)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $definitions[1].Path), `
            (Split-Path -Parent $definitions[3].Path) | Out-Null
        [IO.File]::WriteAllText($definitions[1].Path, '{"preview":"before"}')

        Save-WindowsTerminalRecovery -Recovery $captureRecovery

        $manifest = @(Get-Content -Raw -LiteralPath (Join-Path $captureRecovery 'windows-terminal.json') |
            ConvertFrom-Json)
        $manifest.Count | Should -Be 4
        @($manifest.Kind) | Should -Be @('Packaged', 'Preview', 'Canary', 'Portable')
        @($manifest.Existed) | Should -Be @($false, $true, $false, $false)
        @($manifest.ExpectedPresent) | Should -Be @($false, $true, $false, $true)
        $manifest[0].ExpectedSha | Should -Be ''
        $manifest[1].ExpectedSha | Should -Match '^[0-9a-f]{64}$'
        $manifest[2].ExpectedSha | Should -Be ''
        $manifest[3].ExpectedSha | Should -Match '^[0-9a-f]{64}$'
        Test-Path -LiteralPath (Join-Path $captureRecovery 'wt-0.before') | Should -BeFalse
        Test-Path -LiteralPath (Join-Path $captureRecovery 'wt-1.before') | Should -BeTrue
        Test-Path -LiteralPath (Join-Path $captureRecovery 'wt-2.before') | Should -BeFalse
        Test-Path -LiteralPath (Join-Path $captureRecovery 'wt-3.before') | Should -BeFalse
    }

    It 'rejects a corrupted retained backup before changing any live target' {
        [IO.File]::WriteAllText((Join-Path $script:Recovery 'wt-0.before'), 'corrupt')
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        [IO.File]::WriteAllBytes($script:Target1, $script:Expected1)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target2) | Out-Null
        [IO.File]::WriteAllBytes($script:Target2, $script:Expected2)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target3) | Out-Null
        [IO.File]::WriteAllBytes($script:Target3, $script:Expected3)

        {
            Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedTargets $script:ExpectedTargets
        } | Should -Throw '*does not match its manifest*'
        [IO.File]::ReadAllBytes($script:Target0) | Should -Be $script:Expected0
        [IO.File]::ReadAllBytes($script:Target1) | Should -Be $script:Expected1
        [IO.File]::ReadAllBytes($script:Target2) | Should -Be $script:Expected2
        [IO.File]::ReadAllBytes($script:Target3) | Should -Be $script:Expected3
    }

    It 'rechecks every target for concurrent drift before restoring the first one' {
        $entries = @(Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedTargets $script:ExpectedTargets)
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        [IO.File]::WriteAllBytes($script:Target1, $script:Expected1)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target2) | Out-Null
        [IO.File]::WriteAllBytes($script:Target2, $script:Expected2)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target3) | Out-Null
        [IO.File]::WriteAllText($script:Target3, '{"concurrent":true}')

        { Restore-WindowsTerminalState -Entries $entries } | Should -Throw '*changed concurrently*'
        [IO.File]::ReadAllBytes($script:Target0) | Should -Be $script:Expected0
        [IO.File]::ReadAllBytes($script:Target1) | Should -Be $script:Expected1
        [IO.File]::ReadAllBytes($script:Target2) | Should -Be $script:Expected2
        [IO.File]::ReadAllText($script:Target3) | Should -Be '{"concurrent":true}'
    }

    It 'rejects incomplete recovery schema before changing live targets' {
        $manifest = Get-Content -Raw -LiteralPath (Join-Path $script:Recovery 'windows-terminal.json') |
            ConvertFrom-Json
        $manifest[1].PSObject.Properties.Remove('ExpectedSha')
        $manifest | ConvertTo-Json -Depth 8 |
            Set-Content -LiteralPath (Join-Path $script:Recovery 'windows-terminal.json') -Encoding utf8
        $beforeHash = Get-FileSha256OrEmpty -Path $script:Target0

        {
            Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedTargets $script:ExpectedTargets
        } | Should -Throw '*unexpected schema*'
        (Get-FileSha256OrEmpty -Path $script:Target0) | Should -Be $beforeHash
        [IO.File]::ReadAllBytes($script:Target1) | Should -Be $script:Before1
        Test-Path -LiteralPath $script:Target2 | Should -BeFalse
        Test-Path -LiteralPath $script:Target3 | Should -BeFalse
    }

    It 'refuses acceptance while any Terminal target still has pre-migration bytes' {
        $entry = [pscustomobject]@{
            Path = $script:Target0
            ExpectedPresent = $true
            ExpectedSha = Get-BytesSha256 -Bytes $script:Expected0
        }

        { Confirm-WindowsTerminalExpectedState -Entries @($entry) } |
            Should -Throw '*expected post-migration state*'
    }

    It 'refuses acceptance when an expected Terminal target is absent' {
        $entry = [pscustomobject]@{
            Path = $script:Target3
            ExpectedPresent = $true
            ExpectedSha = Get-BytesSha256 -Bytes $script:Expected3
        }

        { Confirm-WindowsTerminalExpectedState -Entries @($entry) } |
            Should -Throw '*expected post-migration state*'
    }

    It 'refuses acceptance when one of four Terminal targets has external bytes' {
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        [IO.File]::WriteAllBytes($script:Target1, $script:Expected1)
        $entries = @(Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedTargets $script:ExpectedTargets)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target2) | Out-Null
        [IO.File]::WriteAllBytes($script:Target2, $script:Expected2)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target3) | Out-Null
        [IO.File]::WriteAllText($script:Target3, '{"external":true}')

        { Confirm-WindowsTerminalExpectedState -Entries $entries } |
            Should -Throw '*expected post-migration state*'
    }

    It 'accepts only the exact expected state, including expected absence' {
        $absentTarget = Join-Path $script:Root 'absent-preview/settings.json'
        $entries = @(
            [pscustomobject]@{
                Path = $script:Target0
                ExpectedPresent = $true
                ExpectedSha = Get-BytesSha256 -Bytes $script:Expected0
            },
            [pscustomobject]@{
                Path = $absentTarget
                ExpectedPresent = $false
                ExpectedSha = ''
            }
        )
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)

        { Confirm-WindowsTerminalExpectedState -Entries $entries } | Should -Not -Throw
    }

    It 'removes only the three transaction-created known-folder state files' {
        $stateRoot = Join-Path $script:Root 'known folder state'
        New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
        [IO.File]::WriteAllText((Join-Path $stateRoot 'localappdata.boltdb'), 'local')
        [IO.File]::WriteAllText((Join-Path $stateRoot 'appdata.boltdb'), 'roaming')
        [IO.File]::WriteAllText((Join-Path $stateRoot 'documents.boltdb'), 'documents')

        Restore-KnownFolderStateBoundary -StateRoot $stateRoot

        Test-Path -LiteralPath $stateRoot | Should -BeFalse
    }

    It 'preserves the whole state root when an unexpected file appears' {
        $stateRoot = Join-Path $script:Root 'known folder state'
        New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
        [IO.File]::WriteAllText((Join-Path $stateRoot 'localappdata.boltdb'), 'local')
        [IO.File]::WriteAllText((Join-Path $stateRoot 'user-data'), 'keep')

        { Restore-KnownFolderStateBoundary -StateRoot $stateRoot } |
            Should -Throw '*unexpected current-generation known-folder state*'
        [IO.File]::ReadAllText((Join-Path $stateRoot 'localappdata.boltdb')) | Should -Be 'local'
        [IO.File]::ReadAllText((Join-Path $stateRoot 'user-data')) | Should -Be 'keep'
    }

    It 'validates the complete known-folder state boundary before rollback mutation' {
        $stateRoot = Join-Path $script:Root 'known folder state'
        New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
        [IO.File]::WriteAllText((Join-Path $stateRoot 'documents.boltdb'), 'documents')
        [IO.File]::WriteAllText((Join-Path $stateRoot 'unexpected'), 'keep')

        { Assert-KnownFolderStateBoundary -StateRoot $stateRoot } |
            Should -Throw '*unexpected current-generation known-folder state*'
        [IO.File]::ReadAllText((Join-Path $stateRoot 'documents.boltdb')) | Should -Be 'documents'
        [IO.File]::ReadAllText((Join-Path $stateRoot 'unexpected')) | Should -Be 'keep'
    }

    It 'rejects malformed command-provider recovery before comparison' {
        $providers = @(Get-CommandProviderInventory)
        $providers[0] | Add-Member -NotePropertyName Unexpected -NotePropertyValue $true
        $path = Join-Path $script:Recovery 'providers.before.json'
        $providers | ConvertTo-Json | Set-Content -LiteralPath $path -Encoding utf8

        { Get-ValidatedProviderInventory -Path $path } |
            Should -Throw '*entry 0 is malformed*'
    }

    It 'verifies the exact pre-migration command-provider boundary' {
        $providers = @(Get-CommandProviderInventory)
        { Assert-ProviderBoundaryRestored -Expected $providers } | Should -Not -Throw
        $providers[0].Source = 'C:\unexpected\git.exe'

        { Assert-ProviderBoundaryRestored -Expected $providers } |
            Should -Throw '*command-provider boundary differs*'
    }

    It 'accepts complete digest-bound frozen release trees' {
        Write-FrozenReleaseFixture -Recovery $script:Recovery

        $frozen = Get-ValidatedFrozenReleaseState -Recovery $script:Recovery

        $frozen.OldSource | Should -Be (Join-Path $script:Recovery 'old-release')
        $frozen.NewSource | Should -Be (Join-Path $script:Recovery 'new-release')
        $frozen.NewCommit | Should -Be '1111111111111111111111111111111111111111'
    }

    It 'rejects frozen release drift before a caller can publish from it' {
        Write-FrozenReleaseFixture -Recovery $script:Recovery
        [IO.File]::WriteAllText((Join-Path $script:Recovery 'new-release/setup.ps1'), 'drifted setup')

        { Get-ValidatedFrozenReleaseState -Recovery $script:Recovery } |
            Should -Throw '*tree differs from its validated manifest*'
    }

    It 'rejects a changed frozen release archive before publication' {
        Write-FrozenReleaseFixture -Recovery $script:Recovery
        [IO.File]::AppendAllText((Join-Path $script:Recovery 'old-release.tar'), 'drift')

        { Get-ValidatedFrozenReleaseState -Recovery $script:Recovery } |
            Should -Throw '*payload differs from its manifest*'
    }
}

if ($env:OS -eq 'Windows_NT') {
    Describe 'v0.1.0 Windows migration private recovery ACL' {
        It 'removes inherited access before any recovery payload is written' {
            $parent = Join-Path ([IO.Path]::GetTempPath()) ('upgrade acl ' + [guid]::NewGuid())
            try {
                $recovery = Initialize-PrivateRecoveryDirectory -Parent $parent
                { Assert-PrivateRecoveryAcl -Path $recovery } | Should -Not -Throw
                Test-Path -LiteralPath (Join-Path $recovery 'stage') | Should -BeFalse
            } finally {
                Remove-Item -LiteralPath $parent -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }
}
