BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    . (Join-Path $script:RepoRoot 'scripts/upgrade-v0.1.0.ps1') -SourceOnly

    function Write-TerminalRecoveryFixture {
        param(
            [Parameter(Mandatory)] [string]$Recovery,
            [Parameter(Mandatory)] [string[]]$Paths,
            [Parameter(Mandatory)] [byte[]]$BeforeBytes,
            [Parameter(Mandatory)] [byte[][]]$ExpectedBytes
        )
        [IO.File]::WriteAllBytes((Join-Path $Recovery 'wt-0.before'), $BeforeBytes)
        $entries = @(
            [pscustomobject]@{
                Path = $Paths[0]
                Existed = $true
                BeforeSha = Get-BytesSha256 -Bytes $BeforeBytes
                ExpectedSha = Get-BytesSha256 -Bytes $ExpectedBytes[0]
                Backup = 'wt-0.before'
            },
            [pscustomobject]@{
                Path = $Paths[1]
                Existed = $false
                BeforeSha = ''
                ExpectedSha = Get-BytesSha256 -Bytes $ExpectedBytes[1]
                Backup = 'wt-1.before'
            }
        )
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
        $script:Target1 = Join-Path $script:Root 'portable/settings.json'
        New-Item -ItemType Directory -Force -Path $script:Recovery, (Split-Path -Parent $script:Target0) | Out-Null
        $script:Before = [Text.UTF8Encoding]::new($false).GetBytes('{"before":"packaged"}')
        $script:Expected0 = [Text.UTF8Encoding]::new($false).GetBytes('{"expected":"packaged"}')
        $script:Expected1 = [Text.UTF8Encoding]::new($false).GetBytes('{"expected":"portable"}')
        [IO.File]::WriteAllBytes($script:Target0, $script:Before)
        Write-TerminalRecoveryFixture -Recovery $script:Recovery `
            -Paths @($script:Target0, $script:Target1) -BeforeBytes $script:Before `
            -ExpectedBytes @($script:Expected0, $script:Expected1)
    }

    AfterEach {
        Remove-Item -LiteralPath $script:Root -Recurse -Force -ErrorAction SilentlyContinue
    }

    It 'restores exact retained bytes and removes only a transaction-created target' {
        $entries = @(Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedPaths @($script:Target0, $script:Target1))
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target1) | Out-Null
        [IO.File]::WriteAllBytes($script:Target1, $script:Expected1)

        Restore-WindowsTerminalState -Entries $entries

        [IO.File]::ReadAllBytes($script:Target0) | Should -Be $script:Before
        Test-Path -LiteralPath $script:Target1 | Should -BeFalse
    }

    It 'rejects a corrupted retained backup before changing either live target' {
        [IO.File]::WriteAllText((Join-Path $script:Recovery 'wt-0.before'), 'corrupt')
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target1) | Out-Null
        [IO.File]::WriteAllBytes($script:Target1, $script:Expected1)

        {
            Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedPaths @($script:Target0, $script:Target1)
        } | Should -Throw '*does not match its manifest*'
        [IO.File]::ReadAllBytes($script:Target0) | Should -Be $script:Expected0
        [IO.File]::ReadAllBytes($script:Target1) | Should -Be $script:Expected1
    }

    It 'rechecks every target for concurrent drift before restoring the first one' {
        $entries = @(Get-ValidatedWindowsTerminalRecovery -Recovery $script:Recovery `
                -ExpectedPaths @($script:Target0, $script:Target1))
        [IO.File]::WriteAllBytes($script:Target0, $script:Expected0)
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $script:Target1) | Out-Null
        [IO.File]::WriteAllText($script:Target1, '{"concurrent":true}')

        { Restore-WindowsTerminalState -Entries $entries } | Should -Throw '*changed concurrently*'
        [IO.File]::ReadAllBytes($script:Target0) | Should -Be $script:Expected0
        [IO.File]::ReadAllText($script:Target1) | Should -Be '{"concurrent":true}'
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
                -ExpectedPaths @($script:Target0, $script:Target1)
        } | Should -Throw '*unexpected schema*'
        (Get-FileSha256OrEmpty -Path $script:Target0) | Should -Be $beforeHash
        Test-Path -LiteralPath $script:Target1 | Should -BeFalse
    }

    It 'removes only the two transaction-created known-folder state files' {
        $stateRoot = Join-Path $script:Root 'known folder state'
        New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
        [IO.File]::WriteAllText((Join-Path $stateRoot 'localappdata.boltdb'), 'local')
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
