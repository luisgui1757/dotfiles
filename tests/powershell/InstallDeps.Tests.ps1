BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:InstallDeps = Join-Path $script:RepoRoot "install-deps.ps1"

    function winget {}
    function scoop {}
    function choco {}
    function wt {}
    function gh {}
    function node {}
    function npm {}
    function pi {}

    function Get-TestCertificate {
        param([Parameter(Mandatory)][string]$Subject)

        $rsa = [System.Security.Cryptography.RSA]::Create(2048)
        $request = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
            $Subject,
            $rsa,
            [System.Security.Cryptography.HashAlgorithmName]::SHA256,
            [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
        )
        return $request.CreateSelfSigned(
            [datetimeoffset]::UtcNow.AddDays(-1),
            [datetimeoffset]::UtcNow.AddDays(1)
        )
    }

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
        $script:UnmanagedDependencies = @()
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

    function New-ScoopStatusRow {
        param(
            [Parameter(Mandatory)][string]$Name,
            [Parameter(Mandatory)][string]$Installed,
            [string]$Latest = '',
            [string]$MissingDependencies = '',
            [string]$Info = ''
        )

        [pscustomobject]@{
            'Name' = $Name
            'Installed Version' = $Installed
            'Latest Version' = $Latest
            'Missing Dependencies' = $MissingDependencies
            'Info' = $Info
        }
    }

    # gh stub for Install-GhDashExtension tests. `auth status` returns
    # $script:GhAuthRc; `extension list` prints $script:GhListOut with exit
    # $script:GhListRc; `extension install` exits $script:GhInstallRc. remove and
    # install invocations are recorded to $script:GhCalls. Each branch sets
    # $global:LASTEXITCODE like a native command so the no-leak regression is real.
    function Set-GhDashMock {
        Mock -CommandName gh -MockWith {
            $a = @($args)
            if ($a.Count -ge 2 -and $a[0] -eq 'auth' -and $a[1] -eq 'status') {
                $global:LASTEXITCODE = $script:GhAuthRc
                return
            }
            if ($a.Count -ge 2 -and $a[0] -eq 'extension' -and $a[1] -eq 'list') {
                $global:LASTEXITCODE = $script:GhListRc
                if ($script:GhListOut.Count -gt 0) { return $script:GhListOut }
                return
            }
            if ($a.Count -ge 3 -and $a[0] -eq 'extension' -and $a[1] -eq 'remove') {
                $script:GhCalls += ('remove ' + (($a[2..($a.Count - 1)]) -join ' '))
                $global:LASTEXITCODE = $script:GhRemoveRc
                return
            }
            if ($a.Count -ge 3 -and $a[0] -eq 'extension' -and $a[1] -eq 'install') {
                $script:GhCalls += ('install ' + (($a[2..($a.Count - 1)]) -join ' '))
                $global:LASTEXITCODE = $script:GhInstallRc
                return
            }
            if ($a.Count -ge 2 -and $a[0] -eq 'api' -and $a[1] -match '/git/ref/tags/') {
                $global:LASTEXITCODE = $script:GhApiRc
                return $script:GhTagObjectResult
            }
            if ($a.Count -ge 2 -and $a[0] -eq 'api' -and $a[1] -match '/git/tags/') {
                $global:LASTEXITCODE = $script:GhApiRc
                return $script:GhPeeledCommitResult
            }
            $global:LASTEXITCODE = 0
        }
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

    It "formats present and missing dependency table rows without installing" {
        . $script:ImportInstallDepsForTest
        $defaultSpec = @(Get-InstallDependencySpec)
        $defaultSpec[0].Tool | Should -Be 'scoop'
        @($defaultSpec | Where-Object { $_.Tool -eq 'scoop' }).Count | Should -Be 1

        $specList = @(
            [pscustomobject]@{ Tool = 'scoop'; Kind = 'tool'; Binary = 'scoop'; Module = '' },
            [pscustomobject]@{ Tool = 'present-tool'; Kind = 'tool'; Binary = 'present-tool'; Module = '' },
            [pscustomobject]@{ Tool = 'missing-tool'; Kind = 'tool'; Binary = 'missing-tool'; Module = '' }
        )

        Mock -CommandName Install-Scoop -MockWith { throw "table scan must not bootstrap scoop" }
        Mock -CommandName scoop -MockWith { throw "table scan must not run scoop" }
        Mock -CommandName winget -MockWith { throw "table scan must not run winget" }
        Mock -CommandName choco -MockWith { throw "table scan must not run choco" }

        $rows = @(Get-InstallDependencyScan -SpecList $specList -PresenceTester {
                param($Spec)
                return ($Spec.Tool -eq 'present-tool')
            } -VersionGetter {
                param($Spec)
                if ($Spec.Tool -eq 'present-tool') { return 'present-tool 1.2.3' }
                return '-'
            })
        $table = (Format-InstallDependencyTable -Rows $rows) -join "`n"

        # The table pads columns, so rows end with trailing spaces -- allow them
        # (the shell test uses the same skip[[:space:]]*$ tolerance).
        $table | Should -Match '(?m)^scoop\s+missing\s+-\s+install[ \t]*$'
        $table | Should -Match '(?m)^present-tool\s+present\s+present-tool 1\.2\.3\s+skip[ \t]*$'
        $table | Should -Match '(?m)^missing-tool\s+missing\s+-\s+install[ \t]*$'
        $table | Should -Match '1 present, 2 missing'
        Should -Invoke -CommandName Install-Scoop -Times 0 -Exactly
        Should -Invoke -CommandName scoop -Times 0 -Exactly
        Should -Invoke -CommandName winget -Times 0 -Exactly
        Should -Invoke -CommandName choco -Times 0 -Exactly
    }

    It "uses the pinned verified elevated Scoop bootstrap path in dry run" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Get-Command -MockWith { return $null } -ParameterFilter { $Name -eq 'scoop' }
        Mock -CommandName Test-IsElevated -MockWith { return $true }

        # Capture the Write-Host (information) stream, not the boolean return value.
        $output = & { Install-Scoop } 6>&1 | Out-String

        $output | Should -Match 'ScoopInstaller/Install@[0-9a-f]{40}'
        $output | Should -Match 'SHA-256 verified: [0-9a-f]{64}'
        $output | Should -Not -Match 'get\.scoop\.sh'
        $output | Should -Match '-RunAsAdmin'
    }

    It "preserves the caller execution policy while bootstrapping Scoop" {
        . $script:ImportInstallDepsForTest
        $script:ScoopProbeCount = 0
        Mock -CommandName Get-Command -MockWith {
            $script:ScoopProbeCount++
            if ($script:ScoopProbeCount -gt 1) {
                return [pscustomobject]@{ Name = 'scoop'; Source = 'scoop' }
            }
            return $null
        } -ParameterFilter { $Name -eq 'scoop' }
        Mock -CommandName Test-IsElevated -MockWith { return $false }
        Mock -CommandName Invoke-WebRequest -MockWith {
            param([string]$Uri, [string]$OutFile)
            $Uri | Should -Be $ScoopInstallerUrl
            [System.IO.File]::WriteAllText($OutFile, '# verified test installer')
        }
        Mock -CommandName Test-FileSha256 -MockWith { return $true }
        Mock -CommandName Add-ScoopToPathForCurrentProcess -MockWith { }
        Mock -CommandName Ensure-ScoopBuckets -MockWith { }
        Mock -CommandName Set-ExecutionPolicy -MockWith {
            throw 'bootstrap must not mutate the caller execution policy'
        }

        Install-Scoop | Should -BeTrue

        Should -Invoke -CommandName Set-ExecutionPolicy -Times 0 -Exactly
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

    It "installs git before adding scoop buckets when git is absent" {
        # Regression: scoop bucket add clones with git. On a truly fresh machine
        # (Windows Sandbox) git is not installed yet, so the bucket adds failed
        # with "Git is required for buckets". git must be installed (from main,
        # which needs no git) BEFORE the bucket adds.
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()
        Mock -CommandName Get-Command -MockWith {
            return [pscustomobject]@{ Name = 'scoop'; Source = 'scoop' }
        } -ParameterFilter { $Name -eq 'scoop' }
        Mock -CommandName Get-Command -MockWith { return $null } -ParameterFilter { $Name -eq 'git' }
        Mock -CommandName Test-Path -MockWith { return $false }
        Mock -CommandName Add-ScoopToPathForCurrentProcess -MockWith { }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
        }

        Install-Scoop | Should -BeTrue

        $script:ScoopArgs | Should -Contain 'install git'
        $gitIndex = [array]::IndexOf($script:ScoopArgs, 'install git')
        $extrasIndex = [array]::IndexOf($script:ScoopArgs, 'bucket add extras')
        $gitIndex | Should -BeGreaterOrEqual 0
        $gitIndex | Should -BeLessThan $extrasIndex
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

    It "installs psmux from the explicit Scoop bucket manifest" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()
        $script:PsmuxInstalled = $false
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName Test-Tool -MockWith { return $script:PsmuxInstalled } -ParameterFilter { $name -eq 'psmux' }
        Mock -CommandName Add-ScoopBucketSafe -MockWith { return $true } -ParameterFilter {
            $Name -eq 'psmux' -and $Url -eq 'https://github.com/psmux/scoop-psmux'
        }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
            $global:LASTEXITCODE = 0
            if (($args -join ' ') -eq 'install psmux/psmux') {
                $script:PsmuxInstalled = $true
            }
        }

        Install-Psmux

        $script:ScoopArgs | Should -Contain 'install psmux/psmux'
        $script:InstallFailures.Count | Should -Be 0
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

    It "keeps every Windows package catalog key mapped to a command probe" {
        . $script:ImportInstallDepsForTest

        $missing = @($Catalog.Keys | Where-Object { -not $BinaryName.ContainsKey($_) } | Sort-Object)
        $missing | Should -Be @()
    }

    It "registers chezmoi in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('chezmoi') | Should -BeTrue
        $BinaryName['chezmoi'] | Should -Be 'chezmoi'
        $Catalog['chezmoi'].scoop | Should -Be 'chezmoi'
        $Catalog['chezmoi'].winget | Should -Be 'twpayne.chezmoi'
        $Catalog['chezmoi'].choco | Should -Be 'chezmoi'
    }

    It "reads the wt version from the file, never via 'wt --version' (which pops a window)" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Get-Command -MockWith {
            [pscustomobject]@{ Name = 'wt'; Source = 'C:\fake\wt.exe' }
        } -ParameterFilter { $Name -eq 'wt' }
        Mock -CommandName Test-Path -MockWith { return $true }
        Mock -CommandName Get-Item -MockWith {
            [pscustomobject]@{ VersionInfo = [pscustomobject]@{ ProductVersion = '1.24.99' } }
        }
        Mock -CommandName wt -MockWith { throw 'wt must never be executed (it launches a window)' }

        Get-CommandVersionString -CommandName 'wt' | Should -Be '1.24.99'
        Should -Invoke -CommandName wt -Times 0 -Exactly
    }

    It "keeps tree-sitter out of mutable package catalogs and pins release bytes" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('tree-sitter') | Should -BeFalse
        $BinaryName['tree-sitter'] | Should -Be 'tree-sitter'
        $TreeSitterCliVersion | Should -Be 'v0.26.10'
        $TreeSitterCliWindowsX64Sha256 | Should -Match '^[0-9a-f]{64}$'
        $TreeSitterCliWindowsArm64Sha256 | Should -Match '^[0-9a-f]{64}$'
        $TreeSitterCliWindowsX86Sha256 | Should -Match '^[0-9a-f]{64}$'
        @((Get-InstallDependencySpec) | Where-Object { $_.Tool -eq 'tree-sitter' }).Count | Should -Be 1
    }

    It "registers lsd in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('lsd') | Should -BeTrue
        $BinaryName['lsd'] | Should -Be 'lsd'
        $Catalog['lsd'].scoop | Should -Be 'lsd'
        $Catalog['lsd'].winget | Should -Be 'lsd-rs.lsd'
        $Catalog['lsd'].choco | Should -Be 'lsd'
    }

    It "keeps every Windows catalog entry mapped to a binary name" {
        . $script:ImportInstallDepsForTest

        foreach ($tool in $Catalog.Keys) {
            $BinaryName.ContainsKey($tool) | Should -BeTrue -Because "$tool must have a Get-Command binary mapping"
            [string]::IsNullOrWhiteSpace([string]$BinaryName[$tool]) | Should -BeFalse -Because "$tool binary mapping must not be blank"
        }
    }

    It "registers cmake in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('cmake') | Should -BeTrue
        $BinaryName['cmake'] | Should -Be 'cmake'
        $Catalog['cmake'].scoop | Should -Be 'cmake'
        $Catalog['cmake'].winget | Should -Be 'Kitware.CMake'
        $Catalog['cmake'].choco | Should -Be 'cmake'
        $Catalog['cmake'].purpose | Should -Match 'neocmakelsp'
    }

    It "dry-runs VS Build Tools without invoking package managers" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Get-VsBuildToolsInstallationPath -MockWith { return '' }
        Mock -CommandName winget -MockWith { throw "winget must not run under -DryRun" }
        Mock -CommandName choco -MockWith { throw "choco must not run under -DryRun" }

        $output = & { Install-VsBuildTools } 6>&1 | Out-String

        $output | Should -Match 'Microsoft\.VisualStudio\.2022\.BuildTools'
        $output | Should -Match 'Microsoft\.VisualStudio\.Workload\.VCTools'
        $output | Should -Match 'https://aka\.ms/vs/17/release/vs_BuildTools\.exe'
        $output | Should -Match 'verify Authenticode signer'
        Should -Invoke -CommandName winget -Times 0 -Exactly
        Should -Invoke -CommandName choco -Times 0 -Exactly
    }

    It "skips VS Build Tools when a VC toolset is already present" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Get-VsBuildToolsInstallationPath -MockWith { return 'C:\VS' }
        Mock -CommandName winget -MockWith { throw "winget must not run when VC tools are present" }
        Mock -CommandName choco -MockWith { throw "choco must not run when VC tools are present" }

        $output = & { Install-VsBuildTools } 6>&1 | Out-String

        $output | Should -Match 'VC toolset at C:\\VS'
        Should -Invoke -CommandName winget -Times 0 -Exactly
        Should -Invoke -CommandName choco -Times 0 -Exactly
    }

    It "runs VS Build Tools only under All gating" {
        . $script:ImportInstallDepsForTest
        $script:VsBuildToolsCalls = 0
        Mock -CommandName Install-VsBuildTools -MockWith { $script:VsBuildToolsCalls += 1 }

        Install-VsBuildToolsWhenAll -IsAll:$false
        Install-VsBuildToolsWhenAll -IsAll:$true

        $script:VsBuildToolsCalls | Should -Be 1
    }

    It "uses the VS Build Tools VCTools workload override" {
        . $script:ImportInstallDepsForTest
        $script:VsBuildToolsPath = ''
        $script:WingetArgs = @()
        Mock -CommandName Get-VsBuildToolsInstallationPath -MockWith { return $script:VsBuildToolsPath }
        Mock -CommandName winget -MockWith {
            $script:WingetArgs = @($args)
            $script:VsBuildToolsPath = 'C:\VS'
            $global:LASTEXITCODE = 0
        }
        Mock -CommandName choco -MockWith { throw "choco must not run after winget succeeds" }

        Install-VsBuildTools

        $joined = $script:WingetArgs -join ' '
        $joined | Should -Match 'install --id Microsoft\.VisualStudio\.2022\.BuildTools -e'
        $joined | Should -Match '--override'
        $joined | Should -Match 'Microsoft\.VisualStudio\.Workload\.VCTools'
        $joined | Should -Match '--includeRecommended'
        Should -Invoke -CommandName choco -Times 0 -Exactly
    }

    It "falls back to the official VS Build Tools bootstrapper after package managers fail" {
        . $script:ImportInstallDepsForTest
        $script:VsBuildToolsPath = ''
        $script:BootstrapperUri = ''
        $script:BootstrapperArgs = @()
        Mock -CommandName Get-VsBuildToolsInstallationPath -MockWith { return $script:VsBuildToolsPath }
        Mock -CommandName winget -MockWith { $global:LASTEXITCODE = -1978335212 }
        Mock -CommandName choco -MockWith { $global:LASTEXITCODE = 56 }
        Mock -CommandName Invoke-WebRequest -MockWith {
            param($Uri)
            $script:BootstrapperUri = $Uri
        }
        Mock -CommandName Test-VsBuildToolsBootstrapperSignature -MockWith { return $true }
        Mock -CommandName Start-Process -MockWith {
            param($FilePath, $ArgumentList)
            $script:BootstrapperArgs = @($ArgumentList)
            $script:VsBuildToolsPath = 'C:\VS'
            return [pscustomobject]@{ ExitCode = 0 }
        }

        $output = & { Install-VsBuildTools } 6>&1 | Out-String

        $script:BootstrapperUri | Should -Be 'https://aka.ms/vs/17/release/vs_BuildTools.exe'
        ($script:BootstrapperArgs -join ' ') | Should -Match 'Microsoft\.VisualStudio\.Workload\.VCTools'
        ($script:BootstrapperArgs -join ' ') | Should -Match '--includeRecommended'
        $output | Should -Match 'Microsoft bootstrapper'
        $script:InstallFailures.Count | Should -Be 0
        Should -Invoke -CommandName Test-VsBuildToolsBootstrapperSignature -Times 1 -Exactly
    }

    It "keeps VS Build Tools install failure best effort with a FAIL marker" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Get-VsBuildToolsInstallationPath -MockWith { return '' }
        Mock -CommandName winget -MockWith { $global:LASTEXITCODE = 55 }
        Mock -CommandName choco -MockWith { $global:LASTEXITCODE = 56 }
        Mock -CommandName Invoke-WebRequest -MockWith { }
        Mock -CommandName Test-VsBuildToolsBootstrapperSignature -MockWith { return $true }
        Mock -CommandName Start-Process -MockWith { return [pscustomobject]@{ ExitCode = 57 } }

        $output = & { Install-VsBuildTools } 6>&1 | Out-String

        $output | Should -Match 'FAIL: VS Build Tools install failed'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'VS Build Tools'
        $script:InstallFailures[0].Pm | Should -Be 'winget/choco/bootstrapper'
        $script:InstallFailures[0].Pkg | Should -Be 'Microsoft.VisualStudio.Workload.VCTools'
        $script:InstallFailures[0].ExitCode | Should -Be 57
    }

    It "does not run the VS Build Tools bootstrapper when Authenticode verification fails" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Get-VsBuildToolsInstallationPath -MockWith { return '' }
        Mock -CommandName winget -MockWith { $global:LASTEXITCODE = 55 }
        Mock -CommandName choco -MockWith { $global:LASTEXITCODE = 56 }
        Mock -CommandName Invoke-WebRequest -MockWith { }
        Mock -CommandName Test-VsBuildToolsBootstrapperSignature -MockWith { return $false }
        Mock -CommandName Start-Process -MockWith { throw "Start-Process must not run before verification" }

        $output = & { Install-VsBuildTools } 6>&1 | Out-String

        $output | Should -Match 'FAIL: VS Build Tools install failed'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'VS Build Tools'
        $script:InstallFailures[0].Pm | Should -Be 'winget/choco/bootstrapper'
        Should -Invoke -CommandName Start-Process -Times 0 -Exactly
        Should -Invoke -CommandName Test-VsBuildToolsBootstrapperSignature -Times 1 -Exactly
    }

    It "accepts only a valid Microsoft-owned VS Build Tools Authenticode signature" {
        . $script:ImportInstallDepsForTest
        $microsoftCertificate = Get-TestCertificate 'CN=Microsoft Corporation, O=Microsoft Corporation, C=US'
        $exampleCertificate = Get-TestCertificate 'CN=Example Publisher, O=Example Corp'
        $microsoftSignature = [pscustomobject]@{
            Status = 'Valid'
            SignerCertificate = $microsoftCertificate
        }
        $invalidSignature = [pscustomobject]@{
            Status = 'Valid'
            SignerCertificate = $exampleCertificate
        }
        $validMicrosoftChain = {
            param([System.Security.Cryptography.X509Certificates.X509Certificate2]$SignerCertificate)
            [pscustomobject]@{
                IsValid = $true
                Certificates = @($SignerCertificate)
                Statuses = @()
            }
        }

        Test-VsBuildToolsBootstrapperSignature -Path 'C:\tmp\vs.exe' -SignatureGetter { $microsoftSignature } -ChainBuilder $validMicrosoftChain | Should -BeTrue
        Test-VsBuildToolsBootstrapperSignature -Path 'C:\tmp\vs.exe' -SignatureGetter { $invalidSignature } -ChainBuilder $validMicrosoftChain | Should -BeFalse
    }

    It "rejects VS Build Tools when real certificate chain validation fails" {
        . $script:ImportInstallDepsForTest
        $microsoftCertificate = Get-TestCertificate 'CN=Microsoft Corporation, O=Microsoft Corporation, C=US'
        $microsoftSignature = [pscustomobject]@{
            Status = 'Valid'
            SignerCertificate = $microsoftCertificate
        }
        $failedChain = {
            param([System.Security.Cryptography.X509Certificates.X509Certificate2]$SignerCertificate)
            [pscustomobject]@{
                IsValid = $false
                Certificates = @($SignerCertificate)
                Statuses = @('UntrustedRoot')
            }
        }

        Test-VsBuildToolsBootstrapperSignature -Path 'C:\tmp\vs.exe' -SignatureGetter { $microsoftSignature } -ChainBuilder $failedChain | Should -BeFalse
    }

    It "registers the Pi CLI as a pinned npm-backed dependency" {
        . $script:ImportInstallDepsForTest

        $BinaryName['pi'] | Should -Be 'pi'
        @((Get-InstallDependencySpec) | Where-Object { $_.Tool -eq 'pi' }).Count | Should -Be 1
        $PiCliPackage | Should -Be '@earendil-works/pi-coding-agent'
        $PiCliVersion | Should -Be '0.80.9'
        $PiCliIntegrity | Should -Match '^sha512-'
    }

    It "accepts a real Node 24 native probe when LASTEXITCODE starts unset" {
        $nodeBin = Join-Path $TestDrive 'node-bin'
        New-Item -ItemType Directory -Force -Path $nodeBin | Out-Null
        [System.IO.File]::WriteAllText(
            (Join-Path $nodeBin 'node.cmd'),
            "@echo off`r`necho 24.18.0`r`n",
            [System.Text.Encoding]::ASCII
        )
        $oldPath = $env:PATH
        $runner = (Get-Process -Id $PID).Path
        $escapedInstaller = $script:InstallDeps.Replace("'", "''")
        $probe = @"
`$env:INSTALL_DEPS_PS1_SOURCE_ONLY = '1'
. '$escapedInstaller' -All
Remove-Item Env:INSTALL_DEPS_PS1_SOURCE_ONLY
Remove-Variable LASTEXITCODE -Scope Global -ErrorAction SilentlyContinue
if (Test-PiCliNodeReady) { exit 0 }
exit 97
"@
        try {
            $env:PATH = "$nodeBin;$oldPath"
            & $runner -NoProfile -Command $probe
            $exitCode = $LASTEXITCODE
        } finally {
            $env:PATH = $oldPath
        }

        $exitCode | Should -Be 0
    }

    It "dry-runs the Pi CLI as a packed, byte-verified local tarball" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Test-PiCliCurrent -MockWith { return $false }
        Mock -CommandName Test-PiCliNodeReady -MockWith { return $true }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -in @('npm', 'node')) { return [pscustomobject]@{ Name = $Name; Source = $Name } }
            return $null
        }

        $output = & { Install-PiCli } 6>&1 | Out-String

        $output | Should -Match 'npm pack --ignore-scripts --json --pack-destination <temp> @earendil-works/pi-coding-agent@0\.80\.9'
        $output | Should -Match ([regex]::Escape($PiCliIntegrity))
        $output | Should -Match 'npm install -g <verified-local-tarball> <exact same-release Pi companions>'
    }

    It "validates Pi tarball bytes independently against SHA-512 SRI" {
        . $script:ImportInstallDepsForTest
        $path = Join-Path $TestDrive 'pi.tgz'
        [System.IO.File]::WriteAllText($path, 'known package bytes')
        $sha = [System.Security.Cryptography.SHA512]::Create()
        try {
            $sri = 'sha512-' + [Convert]::ToBase64String($sha.ComputeHash([System.IO.File]::ReadAllBytes($path)))
        } finally {
            $sha.Dispose()
        }

        Test-PiCliTarballIntegrity -Path $path -ExpectedIntegrity $sri | Should -BeTrue
        [System.IO.File]::AppendAllText($path, 'tampered')
        Test-PiCliTarballIntegrity -Path $path -ExpectedIntegrity $sri | Should -BeFalse
        Test-PiCliTarballIntegrity -Path $path -ExpectedIntegrity 'sha256-not-supported' | Should -BeFalse
    }

    It "rejects Pi pack metadata disagreement and cleans temporary state" {
        . $script:ImportInstallDepsForTest
        $tempRoot = Join-Path $TestDrive 'pi metadata temp'
        $oldTempRoot = $env:DOTFILES_PI_CLI_TEMP_ROOT
        try {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $tempRoot
            Mock -CommandName Invoke-PiCliNpm -MockWith {
                param([string[]]$Arguments, [string]$StderrPath)
                if ($Arguments[0] -eq 'install') { throw 'install must not run after metadata mismatch' }
                $packDir = $Arguments[4]
                [System.IO.File]::WriteAllText((Join-Path $packDir 'pi.tgz'), 'bytes')
                $json = @([pscustomobject]@{ filename = 'pi.tgz'; integrity = 'sha512-not-the-pin' }) | ConvertTo-Json -Compress
                return [pscustomobject]@{ Output = @($json); ExitCode = 0 }
            }

            { Invoke-PiCliVerifiedTarballInstall } | Should -Throw '*metadata integrity mismatch*'
            @(Get-ChildItem -LiteralPath $tempRoot -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
        } finally {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $oldTempRoot
        }
    }

    It "rejects partial Pi tarballs before install and cleans temporary state" {
        . $script:ImportInstallDepsForTest
        $tempRoot = Join-Path $TestDrive 'pi partial temp'
        $oldTempRoot = $env:DOTFILES_PI_CLI_TEMP_ROOT
        try {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $tempRoot
            Mock -CommandName Invoke-PiCliNpm -MockWith {
                param([string[]]$Arguments, [string]$StderrPath)
                if ($Arguments[0] -eq 'install') { throw 'install must not run after byte mismatch' }
                $packDir = $Arguments[4]
                [System.IO.File]::WriteAllText((Join-Path $packDir 'pi.tgz'), 'partial')
                $json = @([pscustomobject]@{ filename = 'pi.tgz'; integrity = $PiCliIntegrity }) | ConvertTo-Json -Compress
                return [pscustomobject]@{ Output = @($json); ExitCode = 0 }
            }

            { Invoke-PiCliVerifiedTarballInstall } | Should -Throw '*tarball bytes do not match pinned SRI*'
            @(Get-ChildItem -LiteralPath $tempRoot -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
        } finally {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $oldTempRoot
        }
    }

    It "installs the verified Pi tarball with exact same-release companions and cleans on success" {
        . $script:ImportInstallDepsForTest
        $tempRoot = Join-Path $TestDrive 'pi success temp'
        $oldTempRoot = $env:DOTFILES_PI_CLI_TEMP_ROOT
        $script:InstalledPiArguments = @()
        try {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $tempRoot
            Mock -CommandName Test-PiCliTarballIntegrity -MockWith { return $true }
            Mock -CommandName Invoke-PiCliNpm -MockWith {
                param([string[]]$Arguments, [string]$StderrPath)
                if ($Arguments[0] -eq 'pack') {
                    $packDir = $Arguments[4]
                    [System.IO.File]::WriteAllText((Join-Path $packDir 'pi.tgz'), 'verified bytes')
                    $json = @([pscustomobject]@{ filename = 'pi.tgz'; integrity = $PiCliIntegrity }) | ConvertTo-Json -Compress
                    return [pscustomobject]@{ Output = @($json); ExitCode = 0 }
                }
                $script:InstalledPiArguments = @($Arguments)
                return [pscustomobject]@{ Output = @(); ExitCode = 0 }
            }

            Invoke-PiCliVerifiedTarballInstall

            $script:InstalledPiArguments[2] | Should -Match 'dotfiles-pi-[0-9a-f]+[\\/]pi\.tgz$'
            $script:InstalledPiArguments[3..5] | Should -Be @(
                '@earendil-works/pi-agent-core@0.80.9',
                '@earendil-works/pi-ai@0.80.9',
                '@earendil-works/pi-tui@0.80.9'
            )
            @(Get-ChildItem -LiteralPath $tempRoot -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
        } finally {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $oldTempRoot
        }
    }

    It "cleans Pi pack state when verified-local-tarball installation fails" {
        . $script:ImportInstallDepsForTest
        $tempRoot = Join-Path $TestDrive 'pi install failure temp'
        $oldTempRoot = $env:DOTFILES_PI_CLI_TEMP_ROOT
        try {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $tempRoot
            Mock -CommandName Test-PiCliTarballIntegrity -MockWith { return $true }
            Mock -CommandName Invoke-PiCliNpm -MockWith {
                param([string[]]$Arguments, [string]$StderrPath)
                if ($Arguments[0] -eq 'pack') {
                    $packDir = $Arguments[4]
                    [System.IO.File]::WriteAllText((Join-Path $packDir 'pi.tgz'), 'verified bytes')
                    $json = @([pscustomobject]@{ filename = 'pi.tgz'; integrity = $PiCliIntegrity }) | ConvertTo-Json -Compress
                    return [pscustomobject]@{ Output = @($json); ExitCode = 0 }
                }
                return [pscustomobject]@{ Output = @(); ExitCode = 61 }
            }

            { Invoke-PiCliVerifiedTarballInstall } | Should -Throw '*verified local tarball*pi.tgz*exit 61*'
            @(Get-ChildItem -LiteralPath $tempRoot -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
        } finally {
            $env:DOTFILES_PI_CLI_TEMP_ROOT = $oldTempRoot
        }
    }

    It "records one Pi failure when pack or local-tarball install fails" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Test-PiCliCurrent -MockWith { return $false }
        Mock -CommandName Test-PiCliNodeReady -MockWith { return $true }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -in @('npm', 'node')) { return [pscustomobject]@{ Name = $Name; Source = $Name } }
            return $null
        }
        Mock -CommandName Invoke-PiCliVerifiedTarballInstall -MockWith { throw 'npm pack network failure' }

        $output = & { Install-PiCli } 6>&1 | Out-String

        $output | Should -Match 'npm pack network failure'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'pi'
    }

    It "reuses a current Pi install without packing again" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Test-PiCliCurrent -MockWith { return $true }
        Mock -CommandName Invoke-PiCliVerifiedTarballInstall -MockWith { throw 'must not repack a current Pi install' }

        $output = & { Install-PiCli } 6>&1 | Out-String

        $output | Should -Match 'already installed \(0\.80\.9\)'
        Should -Invoke -CommandName Invoke-PiCliVerifiedTarballInstall -Times 0 -Exactly
    }

    It "maps every supported Windows tree-sitter architecture to pinned bytes" {
        . $script:ImportInstallDepsForTest

        (Get-TreeSitterWindowsArtifact -Architecture x64).Name | Should -Be 'tree-sitter-cli-windows-x64.zip'
        (Get-TreeSitterWindowsArtifact -Architecture x64).Sha256 | Should -Be $TreeSitterCliWindowsX64Sha256
        (Get-TreeSitterWindowsArtifact -Architecture arm64).Sha256 | Should -Be $TreeSitterCliWindowsArm64Sha256
        (Get-TreeSitterWindowsArtifact -Architecture x86).Sha256 | Should -Be $TreeSitterCliWindowsX86Sha256
        { Get-TreeSitterWindowsArtifact -Architecture riscv64 } | Should -Throw '*unsupported Windows architecture*'
    }

    It "falls back to native Windows processor architecture when RuntimeInformation is empty" {
        . $script:ImportInstallDepsForTest
        $oldArchitecture = $env:PROCESSOR_ARCHITECTURE
        $oldWowArchitecture = $env:PROCESSOR_ARCHITEW6432
        try {
            $env:PROCESSOR_ARCHITECTURE = 'AMD64'
            $env:PROCESSOR_ARCHITEW6432 = ''
            Get-WindowsOsArchitecture -RuntimeArchitecture '' | Should -Be 'x64'

            $env:PROCESSOR_ARCHITECTURE = 'x86'
            $env:PROCESSOR_ARCHITEW6432 = 'ARM64'
            Get-WindowsOsArchitecture -RuntimeArchitecture '' | Should -Be 'arm64'
        } finally {
            $env:PROCESSOR_ARCHITECTURE = $oldArchitecture
            $env:PROCESSOR_ARCHITEW6432 = $oldWowArchitecture
        }
    }

    It "promotes an owned Windows PATH directory once without removing other entries" {
        . $script:ImportInstallDepsForTest
        $managed = Join-Path $TestDrive 'managed bin'
        $shadow = Join-Path $TestDrive 'shadow bin'
        $other = Join-Path $TestDrive 'other bin'
        $pathValue = "$shadow;$managed\;$other;$managed"

        $parts = @((Get-PathListWithDirectoryFirst -PathValue $pathValue -Directory $managed) -split ';')

        $parts | Should -HaveCount 3
        $parts[0] | Should -Be $managed
        $parts[1] | Should -Be $shadow
        $parts[2] | Should -Be $other
    }

    It "accepts a compatible unmanaged tree-sitter without replacing it" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Get-TreeSitterCliVersion -MockWith { return '0.26.10' }
        Mock -CommandName Invoke-WebRequest -MockWith { throw 'compatible install must not download' }

        $output = & { Install-TreeSitterCli } 6>&1 | Out-String

        $output | Should -Match 'compatible 0\.26\.10'
        Should -Invoke -CommandName Invoke-WebRequest -Times 0 -Exactly
        $script:InstallFailures.Count | Should -Be 0
    }

    It "repairs tree-sitter when the existing managed PATH entry is shadowed" {
        . $script:ImportInstallDepsForTest
        $localAppData = Join-Path $TestDrive 'Redirected Local AppData'
        $installRoot = Join-Path (Join-Path $localAppData 'dotfiles') 'bin'
        $shadowRoot = Join-Path $TestDrive 'older tree-sitter bin'
        New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
        New-Item -ItemType Directory -Force -Path $shadowRoot | Out-Null
        $target = Join-Path $installRoot 'tree-sitter.exe'
        [System.IO.File]::WriteAllText($target, 'stale executable')
        $oldOverride = $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE
        $oldPath = $env:PATH
        try {
            $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE = $localAppData
            $env:PATH = "$shadowRoot;$installRoot"
            $script:TreeSitterValidatedPaths = @()
            Mock -CommandName Get-TreeSitterCliVersion -MockWith {
                param([string]$Path)
                if (-not [string]::IsNullOrWhiteSpace($Path)) {
                    $script:TreeSitterValidatedPaths += $Path
                    if ([IO.Path]::GetExtension($Path) -ne '.exe') { return '' }
                    return '0.26.10'
                }
                $firstPath = Normalize-PathListEntry (($env:PATH -split ';')[0])
                if ($firstPath.Equals((Normalize-PathListEntry $installRoot), [StringComparison]::OrdinalIgnoreCase)) {
                    return '0.26.10'
                }
                return '0.25.0'
            }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'zip')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $true }
            Mock -CommandName Expand-Archive -MockWith {
                param($LiteralPath, $DestinationPath)
                New-Item -ItemType Directory -Force -Path $DestinationPath | Out-Null
                [System.IO.File]::WriteAllText((Join-Path $DestinationPath 'tree-sitter.exe'), 'exact executable')
            }

            $output = & { Install-TreeSitterCli } 6>&1 | Out-String

            $output | Should -Match 'installed\s+tree-sitter\s+v0\.26\.10'
            [System.IO.File]::ReadAllText($target) | Should -Be 'exact executable'
            @(Get-ChildItem -LiteralPath $installRoot -Filter '.tree-sitter.exe.*' -Force).Count | Should -Be 0
            @($script:TreeSitterValidatedPaths | Where-Object { [IO.Path]::GetExtension($_) -ne '.exe' }).Count | Should -Be 0
            $script:InstallFailures.Count | Should -Be 0
            Should -Invoke -CommandName Invoke-WebRequest -Times 1 -Exactly
        } finally {
            $env:PATH = $oldPath
            $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE = $oldOverride
        }
    }

    It "preserves the old managed tree-sitter when post-publication setup fails" {
        . $script:ImportInstallDepsForTest
        $localAppData = Join-Path $TestDrive 'rollback Local AppData'
        $installRoot = Join-Path (Join-Path $localAppData 'dotfiles') 'bin'
        New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
        $target = Join-Path $installRoot 'tree-sitter.exe'
        [System.IO.File]::WriteAllText($target, 'old valid executable')
        $oldOverride = $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE
        try {
            $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE = $localAppData
            Mock -CommandName Get-TreeSitterCliVersion -MockWith {
                param([string]$Path)
                if (-not [string]::IsNullOrWhiteSpace($Path)) { return '0.26.10' }
                return '0.25.0'
            }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'zip')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $true }
            Mock -CommandName Expand-Archive -MockWith {
                param($LiteralPath, $DestinationPath)
                New-Item -ItemType Directory -Force -Path $DestinationPath | Out-Null
                [System.IO.File]::WriteAllText((Join-Path $DestinationPath 'tree-sitter.exe'), 'new executable')
            }
            Mock -CommandName Add-DirectoryToUserPath -MockWith { throw 'injected PATH publication failure' }

            Install-TreeSitterCli

            [System.IO.File]::ReadAllText($target) | Should -Be 'old valid executable'
            $script:InstallFailures.Count | Should -Be 1
            $script:InstallFailures[0].Tool | Should -Be 'tree-sitter'
        } finally {
            $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE = $oldOverride
        }
    }

    It "fails checksum validation without changing an existing tree-sitter" {
        . $script:ImportInstallDepsForTest
        $localAppData = Join-Path $TestDrive 'checksum Local AppData'
        $installRoot = Join-Path (Join-Path $localAppData 'dotfiles') 'bin'
        New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
        $target = Join-Path $installRoot 'tree-sitter.exe'
        [System.IO.File]::WriteAllText($target, 'old executable')
        $oldOverride = $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE
        try {
            $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE = $localAppData
            Mock -CommandName Get-TreeSitterCliVersion -MockWith { return '0.25.0' }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'wrong zip')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $false }
            Mock -CommandName Expand-Archive -MockWith { throw 'must not extract checksum mismatch' }

            Install-TreeSitterCli

            [System.IO.File]::ReadAllText($target) | Should -Be 'old executable'
            $script:InstallFailures.Count | Should -Be 1
            Should -Invoke -CommandName Expand-Archive -Times 0 -Exactly
        } finally {
            $env:DOTFILES_LOCAL_APP_DATA_OVERRIDE = $oldOverride
        }
    }

    It "dry-runs the exact tree-sitter artifact without invoking npm or Scoop" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Get-TreeSitterCliVersion -MockWith { return '' }
        Mock -CommandName npm -MockWith { throw 'npm fallback must not run' }
        Mock -CommandName scoop -MockWith { throw 'Scoop tree-sitter must not run' }

        $output = & { Install-TreeSitterCli } 6>&1 | Out-String

        $output | Should -Match 'tree-sitter-cli-windows-(x64|arm64|x86)\.zip'
        $output | Should -Match 'SHA-256 [0-9a-f]{64}'
        Should -Invoke -CommandName npm -Times 0 -Exactly
        Should -Invoke -CommandName scoop -Times 0 -Exactly
    }

    It "registers Windows Terminal in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('wt') | Should -BeTrue
        $BinaryName['wt'] | Should -Be 'wt'
        $Catalog['wt'].scoop | Should -Be 'extras/windows-terminal'
        $Catalog['wt'].winget | Should -Be 'Microsoft.WindowsTerminal'
        $Catalog['wt'].choco | Should -Be 'microsoft-windows-terminal'
        $WindowsTerminalVersion | Should -Be 'v1.24.11321.0'
        $WindowsTerminalX64Sha256 | Should -Be '7caef554147e5498ed1becdca73cdedb79fbc81f89032e46ae9b095c53433812'
    }

    It "dry-runs Windows Terminal managers plus the pinned portable fallback" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Read-Host -MockWith { throw "Read-Host must not run under -DryRun" }
        Mock -CommandName Test-Tool -MockWith { return $false } -ParameterFilter { $name -eq 'wt' }

        $output = & { Install-WindowsTerminal } 6>&1 | Out-String

        $output | Should -Match 'extras/windows-terminal'
        $output | Should -Match 'pinned portable zip v1\.24\.11321\.0'
        $output | Should -Match 'Microsoft\.WindowsTerminal_1\.24\.11321\.0_x64\.zip'
        Should -Invoke -CommandName Read-Host -Times 0 -Exactly
    }

    It "fails closed when the Windows Terminal portable checksum mismatches" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Test-Tool -MockWith { return $false } -ParameterFilter { $name -eq 'wt' }
        Mock -CommandName Install-One -MockWith { }
        Mock -CommandName Invoke-WebRequest -MockWith {
            param($Uri, $OutFile)
            [System.IO.File]::WriteAllText($OutFile, 'bad-zip')
        }
        Mock -CommandName Test-FileSha256 -MockWith { return $false }
        Mock -CommandName Expand-Archive -MockWith { throw "must not extract" }

        $output = & { Install-WindowsTerminal } 6>&1 | Out-String

        $output | Should -Match 'FAIL: checksum mismatch for Microsoft\.WindowsTerminal_1\.24\.11321\.0_x64\.zip'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'wt'
        $script:InstallFailures[0].Pm | Should -Be 'portable'
        $script:InstallFailures[0].ExitCode | Should -Be 'sha256'
        Should -Invoke -CommandName Expand-Archive -Times 0 -Exactly
    }

    It "fails closed when the Hack Nerd Font direct checksum mismatches" {
        . $script:ImportInstallDepsForTest
        $oldTemp = $env:TEMP
        $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hack-nf-test-" + [System.Guid]::NewGuid())
        try {
            New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
            $env:TEMP = $tempRoot
            Mock -CommandName Get-HackNerdFontInstallScope -MockWith { return '' }
            Mock -CommandName Get-Command -MockWith { return $null } -ParameterFilter { $Name -eq 'scoop' }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'bad-zip')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $false }
            Mock -CommandName Expand-Archive -MockWith { throw "must not extract" }

            $output = & { Install-HackNerdFont } *>&1 | Out-String

            $output | Should -Match 'FAIL: checksum mismatch for Hack\.zip'
            $script:InstallFailures.Count | Should -Be 1
            $script:InstallFailures[0].Tool | Should -Be 'Hack Nerd Font'
            $script:InstallFailures[0].Pm | Should -Be 'direct'
            $script:InstallFailures[0].Pkg | Should -Be 'Hack.zip'
            $script:InstallFailures[0].ExitCode | Should -Be 'sha256'
            Should -Invoke -CommandName Expand-Archive -Times 0 -Exactly
            Should -Invoke -CommandName Invoke-WebRequest -Times 1 -Exactly -ParameterFilter {
                $ErrorAction -eq 'Stop'
            }
        } finally {
            if ($null -eq $oldTemp) {
                Remove-Item Env:TEMP -ErrorAction SilentlyContinue
            } else {
                $env:TEMP = $oldTemp
            }
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "records Hack Nerd Font direct install exceptions in InstallFailures" {
        . $script:ImportInstallDepsForTest
        $oldTemp = $env:TEMP
        $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hack-nf-test-" + [System.Guid]::NewGuid())
        try {
            New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
            $env:TEMP = $tempRoot
            Mock -CommandName Get-HackNerdFontInstallScope -MockWith { return '' }
            Mock -CommandName Get-Command -MockWith { return $null } -ParameterFilter { $Name -eq 'scoop' }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'good-zip')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $true }
            Mock -CommandName Expand-Archive -MockWith { throw "extract failed" }

            $output = & { Install-HackNerdFont } *>&1 | Out-String

            $output | Should -Match 'Hack Nerd Font install failed'
            $script:InstallFailures.Count | Should -Be 1
            $script:InstallFailures[0].Tool | Should -Be 'Hack Nerd Font'
            $script:InstallFailures[0].Pm | Should -Be 'direct'
            $script:InstallFailures[0].Pkg | Should -Be 'Hack.zip'
            $script:InstallFailures[0].ExitCode | Should -Be 'exception'
        } finally {
            if ($null -eq $oldTemp) {
                Remove-Item Env:TEMP -ErrorAction SilentlyContinue
            } else {
                $env:TEMP = $oldTemp
            }
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "installs the Windows Terminal portable fallback after managers fail" {
        . $script:ImportInstallDepsForTest
        $oldLocalAppData = $env:LOCALAPPDATA
        $script:WtInstalled = $false
        $script:AddedWtPath = $null
        $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("wt-portable-test-" + [System.Guid]::NewGuid())
        try {
            New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
            $env:LOCALAPPDATA = $tempRoot
            Mock -CommandName Test-Tool -MockWith { return $script:WtInstalled } -ParameterFilter { $name -eq 'wt' }
            Mock -CommandName Install-One -MockWith { }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'good-zip')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $true }
            Mock -CommandName Expand-Archive -MockWith {
                param($Path, $DestinationPath)
                $portableDir = Join-Path $DestinationPath 'terminal-1.24.11321.0'
                New-Item -ItemType Directory -Force -Path $portableDir | Out-Null
                [System.IO.File]::WriteAllText((Join-Path $portableDir 'wt.exe'), 'exe')
                [System.IO.File]::WriteAllText((Join-Path $portableDir 'WindowsTerminal.exe'), 'exe')
            }
            Mock -CommandName Add-DirectoryToUserPath -MockWith {
                param($Directory)
                $script:AddedWtPath = $Directory
                $script:WtInstalled = $true
            }

            $output = & { Install-WindowsTerminal } 6>&1 | Out-String

            $installRoot = Join-Path $tempRoot 'Programs\WindowsTerminal'
            Test-Path -LiteralPath (Join-Path $installRoot 'wt.exe') -PathType Leaf | Should -BeTrue
            $script:AddedWtPath | Should -Be $installRoot
            $script:InstallFailures.Count | Should -Be 0
            $output | Should -Match 'installed\s+wt\s+portable v1\.24\.11321\.0'
            Should -Invoke -CommandName Install-One -Times 1 -Exactly -ParameterFilter {
                $tool -eq 'wt' -and $SkipPrompt -and $NoRecordFailure
            }
        } finally {
            if ($null -eq $oldLocalAppData) {
                Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue
            } else {
                $env:LOCALAPPDATA = $oldLocalAppData
            }
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "registers Herdr Windows preview as a pinned direct artifact, not a package catalog tool" {
        . $script:ImportInstallDepsForTest -DryRun

        $Catalog.ContainsKey('herdr') | Should -BeFalse
        $BinaryName['herdr'] | Should -Be 'herdr'
        $HerdrWindowsPreviewVersion | Should -Match '^preview-\d{4}-\d{2}-\d{2}-[0-9a-f]{12}$'
        $HerdrWindowsX64Sha256 | Should -Match '^[0-9a-f]{64}$'
        @((Get-InstallDependencySpec) | Where-Object { $_.Tool -eq 'herdr' }).Count | Should -Be 1
    }

    It "dry-runs Herdr Windows preview without package managers or remote eval" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Read-Host -MockWith { throw "Read-Host must not run under -DryRun" }
        Mock -CommandName Test-Tool -MockWith { return $false } -ParameterFilter { $name -eq 'herdr' }

        $output = & { Install-HerdrWindowsPreview } 6>&1 | Out-String

        $output | Should -Match 'herdr-windows-x86_64\.exe'
        $output | Should -Match ([regex]::Escape($HerdrWindowsPreviewVersion))
        $output | Should -Match $HerdrWindowsX64Sha256
        $output | Should -Not -Match 'herdr\.dev/install'
        Should -Invoke -CommandName Read-Host -Times 0 -Exactly
    }

    It "fails closed when the Herdr Windows preview checksum mismatches" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Test-Tool -MockWith { return $false } -ParameterFilter { $name -eq 'herdr' }
        Mock -CommandName Invoke-WebRequest -MockWith {
            param($Uri, $OutFile)
            [System.IO.File]::WriteAllText($OutFile, 'bad-exe')
        }
        Mock -CommandName Test-FileSha256 -MockWith { return $false }
        Mock -CommandName Copy-Item -MockWith { throw "must not copy" }

        $output = & { Install-HerdrWindowsPreview } 6>&1 | Out-String

        $output | Should -Match 'FAIL: checksum mismatch for herdr-windows-x86_64\.exe'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'herdr'
        $script:InstallFailures[0].Pm | Should -Be 'direct'
        $script:InstallFailures[0].Pkg | Should -Be 'herdr-windows-x86_64.exe'
        $script:InstallFailures[0].ExitCode | Should -Be 'sha256'
        Should -Invoke -CommandName Copy-Item -Times 0 -Exactly
    }

    It "installs the Herdr Windows preview after checksum verification" {
        . $script:ImportInstallDepsForTest
        $oldLocalAppData = $env:LOCALAPPDATA
        $script:HerdrInstalled = $false
        $script:AddedHerdrPath = $null
        $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("herdr-windows-test-" + [System.Guid]::NewGuid())
        try {
            New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
            $env:LOCALAPPDATA = $tempRoot
            Mock -CommandName Test-Tool -MockWith { return $script:HerdrInstalled } -ParameterFilter { $name -eq 'herdr' }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                [System.IO.File]::WriteAllText($OutFile, 'good-exe')
            }
            Mock -CommandName Test-FileSha256 -MockWith { return $true }
            Mock -CommandName Add-DirectoryToUserPath -MockWith {
                param($Directory)
                $script:AddedHerdrPath = $Directory
                $script:HerdrInstalled = Test-Path -LiteralPath (Join-Path $Directory 'herdr.exe') -PathType Leaf
            }

            $output = & { Install-HerdrWindowsPreview } 6>&1 | Out-String

            $installRoot = Join-Path $tempRoot 'Programs\Herdr\bin'
            Test-Path -LiteralPath (Join-Path $installRoot 'herdr.exe') -PathType Leaf | Should -BeTrue
            $script:AddedHerdrPath | Should -Be $installRoot
            $script:InstallFailures.Count | Should -Be 0
            $output | Should -Match 'installed\s+herdr'
            $output | Should -Match ([regex]::Escape($HerdrWindowsPreviewVersion))
        } finally {
            if ($null -eq $oldLocalAppData) {
                Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue
            } else {
                $env:LOCALAPPDATA = $oldLocalAppData
            }
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "updates only a stale repo-owned Herdr Windows preview" {
        . $script:ImportInstallDepsForTest -All
        $oldLocalAppData = $env:LOCALAPPDATA
        $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("herdr-windows-update-test-" + [System.Guid]::NewGuid())
        try {
            $env:LOCALAPPDATA = $tempRoot
            $installRoot = Join-Path $tempRoot 'Programs\Herdr\bin'
            $destination = Join-Path $installRoot 'herdr.exe'
            New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
            [System.IO.File]::WriteAllText($destination, 'old-managed-exe')

            Mock -CommandName Test-Tool -MockWith { return $true } -ParameterFilter { $name -eq 'herdr' }
            Mock -CommandName Get-CatalogToolCommandSource -MockWith { return $destination } -ParameterFilter { $tool -eq 'herdr' }
            Mock -CommandName Test-FileSha256 -MockWith {
                param($Path, $Expected)
                $Expected | Should -Be $HerdrWindowsX64Sha256
                return ([IO.Path]::GetFullPath($Path) -ne [IO.Path]::GetFullPath($destination))
            }
            Mock -CommandName Invoke-WebRequest -MockWith {
                param($Uri, $OutFile)
                $Uri | Should -Match ([regex]::Escape($HerdrWindowsPreviewVersion))
                [System.IO.File]::WriteAllText($OutFile, 'new-reviewed-exe')
            }
            Mock -CommandName Add-DirectoryToUserPath

            $output = & { Install-HerdrWindowsPreview } 6>&1 | Out-String

            [System.IO.File]::ReadAllText($destination) | Should -Be 'new-reviewed-exe'
            $output | Should -Match 'updated\s+herdr'
            Should -Invoke -CommandName Invoke-WebRequest -Times 1 -Exactly
        } finally {
            if ($null -eq $oldLocalAppData) {
                Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue
            } else {
                $env:LOCALAPPDATA = $oldLocalAppData
            }
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "leaves an unmanaged Herdr executable untouched" {
        . $script:ImportInstallDepsForTest -All
        Mock -CommandName Test-Tool -MockWith { return $true } -ParameterFilter { $name -eq 'herdr' }
        Mock -CommandName Get-CatalogToolCommandSource -MockWith { return 'C:\Tools\herdr.exe' } -ParameterFilter { $tool -eq 'herdr' }
        Mock -CommandName Invoke-WebRequest -MockWith { throw 'must not download' }

        $output = & { Install-HerdrWindowsPreview } 6>&1 | Out-String

        $output | Should -Match 'already installed \(unmanaged\)'
        Should -Invoke -CommandName Invoke-WebRequest -Times 0 -Exactly
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
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            if ($joined -eq 'list pwsh') {
                $global:LASTEXITCODE = 0
                return 'pwsh 7.5.0'
            }
            if ($joined -eq 'status') {
                $global:LASTEXITCODE = 0
                return (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Latest '7.6.2')
            }
            $global:LASTEXITCODE = 0
        }

        $output = & { Update-ScoopTool pwsh } 6>&1 | Out-String

        $output | Should -Match '(?m)^\s*would:\s+scoop update\s*$'
        $output | Should -Match '(?m)^\s*would:\s+scoop update pwsh\s*$'
        $output | Should -Not -Match 'scoop update \*'
        $script:ScoopArgs | Should -Contain 'list pwsh'
        $script:ScoopArgs | Should -Contain 'status'
        ($script:ScoopArgs | Where-Object { $_ -like 'update*' }).Count | Should -Be 0
    }

    It "uses Scoop shim metadata as ownership proof for PowerShell" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:ScoopArgs = @()

        Mock -CommandName Read-Host -MockWith { throw "Read-Host must not run under -DryRun" }
        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Users\luigu\scoop\shims\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName Test-Path -MockWith {
            param([string]$LiteralPath)
            return ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim')
        }
        Mock -CommandName Get-Content -MockWith {
            param([string]$LiteralPath)
            if ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim') {
                return 'path = "C:\Users\luigu\scoop\apps\pwsh\current\pwsh.exe"'
            }
            throw "unexpected Get-Content path: $LiteralPath"
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            $global:LASTEXITCODE = 0
            if ($joined -eq 'status') {
                return (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Latest '7.6.2')
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 6>&1 | Out-String

        $output | Should -Match 'scoop update pwsh'
        $output | Should -Not -Match 'unmanaged|winget upgrade|choco upgrade'
        ($script:ScoopArgs | Where-Object { $_ -like 'list*' }).Count | Should -Be 0
        ($script:ScoopArgs | Where-Object { $_ -like 'update*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
        $script:UnmanagedDependencies.Count | Should -Be 0
    }

    It "reports Scoop-owned PowerShell as current when Scoop status has no update" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Users\luigu\scoop\shims\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName Test-Path -MockWith {
            param([string]$LiteralPath)
            return ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim')
        }
        Mock -CommandName Get-Content -MockWith {
            param([string]$LiteralPath)
            if ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim') {
                return 'path = "C:\Users\luigu\scoop\apps\pwsh\current\pwsh.exe"'
            }
            throw "unexpected Get-Content path: $LiteralPath"
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            if ($joined -eq 'status') {
                $global:LASTEXITCODE = 0
                return ''
            }
            throw "scoop must not update a current package: $joined"
        }

        $output = & {
            Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -SkipScoopManifestRefresh -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'current\s+pwsh\s+via scoop'
        $script:ScoopArgs | Should -Contain 'status'
        ($script:ScoopArgs | Where-Object { $_ -like 'update*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "updates Scoop-owned PowerShell only when Scoop status reports an update" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Users\luigu\scoop\shims\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName Test-Path -MockWith {
            param([string]$LiteralPath)
            return ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim')
        }
        Mock -CommandName Get-Content -MockWith {
            param([string]$LiteralPath)
            if ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim') {
                return 'path = "C:\Users\luigu\scoop\apps\pwsh\current\pwsh.exe"'
            }
            throw "unexpected Get-Content path: $LiteralPath"
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            switch ($joined) {
                'status' {
                    $global:LASTEXITCODE = 0
                    return (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Latest '7.6.2')
                }
                'update pwsh' {
                    $global:LASTEXITCODE = 0
                    return
                }
                default {
                    throw "unexpected scoop command: $joined"
                }
            }
        }

        $output = & {
            Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -SkipScoopManifestRefresh -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'updated\s+pwsh\s+via scoop'
        $script:ScoopArgs | Should -Contain 'status'
        $script:ScoopArgs | Should -Contain 'update pwsh'
        $script:InstallFailures.Count | Should -Be 0
    }

    It "records failed Scoop status checks without updating the package" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Users\luigu\scoop\shims\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName Test-Path -MockWith {
            param([string]$LiteralPath)
            return ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim')
        }
        Mock -CommandName Get-Content -MockWith {
            param([string]$LiteralPath)
            if ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\pwsh.shim') {
                return 'path = "C:\Users\luigu\scoop\apps\pwsh\current\pwsh.exe"'
            }
            throw "unexpected Get-Content path: $LiteralPath"
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            if ($joined -eq 'status') {
                $global:LASTEXITCODE = 44
                return
            }
            throw "scoop must not update after a failed status check: $joined"
        }

        $output = & {
            Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -SkipScoopManifestRefresh -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'scoop status check of pwsh failed'
        ($script:ScoopArgs | Where-Object { $_ -like 'update*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'pwsh'
        $script:InstallFailures[0].Pm | Should -Be 'scoop'
        $script:InstallFailures[0].Pkg | Should -Be 'pwsh'
        $script:InstallFailures[0].ExitCode | Should -Be 44
    }

    It "fails closed on unhealthy Scoop status rows without updating the package" {
        . $script:ImportInstallDepsForTest
        $script:ScoopStatusRow = $null

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            if ($joined -eq 'status') {
                $global:LASTEXITCODE = 0
                return $script:ScoopStatusRow
            }
            throw "scoop must not update an unhealthy status row: $joined"
        }

        $cases = @(
            [pscustomobject]@{
                Label = 'install failed text'
                Row = 'pwsh 7.5.0 Install failed'
                Reason = 'Install failed'
            },
            [pscustomobject]@{
                Label = 'deprecated'
                Row = (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Info 'Deprecated')
                Reason = 'Deprecated'
            },
            [pscustomobject]@{
                Label = 'manifest removed'
                Row = (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Info 'Manifest removed')
                Reason = 'Manifest removed'
            },
            [pscustomobject]@{
                Label = 'held package'
                Row = (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Latest '7.6.2' -Info 'Held package')
                Reason = 'Held package'
            },
            [pscustomobject]@{
                Label = 'missing dependencies'
                Row = (New-ScoopStatusRow -Name 'pwsh' -Installed '7.5.0' -Latest '7.6.2' -MissingDependencies 'git')
                Reason = 'missing dependencies: git'
            }
        )

        foreach ($case in $cases) {
            $script:ScoopArgs = @()
            $script:InstallFailures = @()
            $script:ScoopStatusRow = $case.Row

            $output = & {
                Update-ScoopTool pwsh -NoPrompt -SkipManifestRefresh -AssumePresent -AssumeManaged -ReportSkip -IsDryRun $false
            } 3>&1 6>&1 | Out-String

            $output | Should -Match 'scoop status check of pwsh failed'
            $output | Should -Match ([regex]::Escape($case.Reason))
            ($script:ScoopArgs | Where-Object { $_ -eq 'update pwsh' }).Count | Should -Be 0
            $script:InstallFailures.Count | Should -Be 1
            $script:InstallFailures[0].Tool | Should -Be 'pwsh'
            $script:InstallFailures[0].Pm | Should -Be 'scoop'
            $script:InstallFailures[0].Pkg | Should -Be 'pwsh'
            $script:InstallFailures[0].ExitCode | Should -Be 'scoop-status-unhealthy'
        }
    }

    It "maps Scoop shim package names that differ from binary names" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'rg') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Users\luigu\scoop\shims\rg.exe' }
            }
            return $null
        }
        Mock -CommandName Test-Path -MockWith {
            param([string]$LiteralPath)
            return ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\rg.shim')
        }
        Mock -CommandName Get-Content -MockWith {
            param([string]$LiteralPath)
            if ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\rg.shim') {
                return 'path = "C:\Users\luigu\scoop\apps\ripgrep\current\rg.exe"'
            }
            throw "unexpected Get-Content path: $LiteralPath"
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            $global:LASTEXITCODE = 0
            if ($joined -eq 'status') {
                return (New-ScoopStatusRow -Name 'ripgrep' -Installed '15.1.0' -Latest '15.2.0')
            }
        }

        $output = & { Update-ManagedCatalogTool rg -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 6>&1 | Out-String

        $output | Should -Match 'scoop update ripgrep'
        $output | Should -Not -Match 'unmanaged|winget upgrade|choco upgrade'
        ($script:ScoopArgs | Where-Object { $_ -like 'list*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "does not let scoop list claim a command source outside Scoop" {
        . $script:ImportInstallDepsForTest -DryRun

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Manual\PowerShell\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith { throw "scoop list must not claim a manual command source" }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 6>&1 | Out-String

        $output | Should -Match 'unmanaged\s+pwsh\s+source=C:\\Manual\\PowerShell\\pwsh\.exe'
        $script:InstallFailures.Count | Should -Be 0
        $script:UnmanagedDependencies.Count | Should -Be 1
    }

    It "dry-runs a scoped PowerShell winget update when winget owns pwsh" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Program Files\PowerShell\7\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            if ($joined -eq 'list --id Microsoft.PowerShell -e --accept-source-agreements') {
                $global:LASTEXITCODE = 0
                return 'PowerShell Microsoft.PowerShell 7.5.0 winget'
            }
            if ($joined -eq 'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements') {
                $global:LASTEXITCODE = 0
                return 'PowerShell Microsoft.PowerShell 7.5.0 7.6.2 winget'
            }
            $global:LASTEXITCODE = 0
        }

        $output = & {
            Invoke-InstallDepsUpdateMode -SpecList @([pscustomobject]@{ Tool = 'pwsh'; Kind = 'tool'; Binary = 'pwsh'; Module = '' }) -PresenceTester {
                param([string]$Tool)
                return ($Tool -eq 'pwsh')
            } -IsDryRun $true
        } 6>&1 | Out-String

        $output | Should -Match 'winget upgrade --id Microsoft\.PowerShell -e --accept-source-agreements --accept-package-agreements --silent'
        $output | Should -Not -Match 'winget upgrade --all|scoop update \*|choco upgrade all'
        $script:WingetArgs | Should -Contain 'list --id Microsoft.PowerShell -e --accept-source-agreements'
        $script:WingetArgs | Should -Contain 'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements'
        ($script:WingetArgs | Where-Object { $_ -like 'upgrade*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "reports winget-owned PowerShell as current when winget has no upgrade available" {
        . $script:ImportInstallDepsForTest
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Program Files\PowerShell\7\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            switch ($joined) {
                'list --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.6.2 winget'
                }
                'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return ''
                }
                default {
                    $global:LASTEXITCODE = 0
                }
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $false } 3>&1 6>&1 | Out-String

        $output | Should -Match 'current\s+pwsh\s+via winget'
        $script:WingetArgs | Should -Contain 'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements'
        ($script:WingetArgs | Where-Object { $_ -like 'upgrade*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "does not let winget list claim a manual command source" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Manual\PowerShell\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            if ($joined -eq 'list --id Microsoft.PowerShell -e --accept-source-agreements') {
                $global:LASTEXITCODE = 0
                return 'PowerShell Microsoft.PowerShell 7.5.0 winget'
            }
            throw "winget must not check upgrades or update a manual command source: $joined"
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 6>&1 | Out-String

        $output | Should -Match 'unmanaged\s+pwsh\s+source=C:\\Manual\\PowerShell\\pwsh\.exe'
        $output | Should -Not -Match 'winget upgrade|current\s+pwsh\s+via winget'
        $script:WingetArgs | Should -Contain 'list --id Microsoft.PowerShell -e --accept-source-agreements'
        ($script:WingetArgs | Where-Object { $_ -like 'list --upgrade-available*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
        $script:UnmanagedDependencies.Count | Should -Be 1
    }

    It "records failed winget upgrade availability checks" {
        . $script:ImportInstallDepsForTest
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Program Files\PowerShell\7\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            switch ($joined) {
                'list --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.6.2 winget'
                }
                'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 44
                    return
                }
                default {
                    $global:LASTEXITCODE = 0
                }
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $false } 3>&1 6>&1 | Out-String

        $output | Should -Match 'winget upgrade availability check of Microsoft\.PowerShell failed'
        $script:WingetArgs | Should -Contain 'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements'
        ($script:WingetArgs | Where-Object { $_ -like 'upgrade*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'pwsh'
        $script:InstallFailures[0].Pm | Should -Be 'winget'
        $script:InstallFailures[0].Pkg | Should -Be 'Microsoft.PowerShell'
        $script:InstallFailures[0].ExitCode | Should -Be 44
    }

    It "dry-runs a scoped PowerShell Chocolatey update when Chocolatey owns pwsh" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\ProgramData\chocolatey\bin\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            if ($joined -eq 'list powershell-core --local-only --exact --limit-output') {
                $global:LASTEXITCODE = 0
                return 'powershell-core|7.5.0'
            }
            if ($joined -eq 'outdated --limit-output') {
                $global:LASTEXITCODE = 2
                return 'powershell-core|7.5.0|7.6.2|false'
            }
            $global:LASTEXITCODE = 0
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 6>&1 | Out-String

        $output | Should -Match 'choco upgrade powershell-core -y'
        $output | Should -Not -Match 'winget upgrade --all|scoop update \*|choco upgrade all'
        $script:ChocoArgs | Should -Contain 'list powershell-core --local-only --exact --limit-output'
        $script:ChocoArgs | Should -Contain 'outdated --limit-output'
        ($script:ChocoArgs | Where-Object { $_ -like 'upgrade*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "reports Chocolatey-owned PowerShell as current when Chocolatey has no update" {
        . $script:ImportInstallDepsForTest
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\ProgramData\chocolatey\bin\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            switch ($joined) {
                'list powershell-core --local-only --exact --limit-output' {
                    $global:LASTEXITCODE = 0
                    return 'powershell-core|7.6.2'
                }
                'outdated --limit-output' {
                    $global:LASTEXITCODE = 0
                    return ''
                }
                default {
                    throw "choco must not update a current package: $joined"
                }
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $false } 3>&1 6>&1 | Out-String

        $output | Should -Match 'current\s+pwsh\s+via choco'
        $script:ChocoArgs | Should -Contain 'outdated --limit-output'
        ($script:ChocoArgs | Where-Object { $_ -like 'upgrade*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "updates Chocolatey-owned PowerShell only when Chocolatey reports an update" {
        . $script:ImportInstallDepsForTest
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\ProgramData\chocolatey\bin\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            switch ($joined) {
                'list powershell-core --local-only --exact --limit-output' {
                    $global:LASTEXITCODE = 0
                    return 'powershell-core|7.5.0'
                }
                'outdated --limit-output' {
                    $global:LASTEXITCODE = 2
                    return 'powershell-core|7.5.0|7.6.2|false'
                }
                'upgrade powershell-core -y' {
                    $global:LASTEXITCODE = 0
                    return
                }
                default {
                    throw "unexpected choco command: $joined"
                }
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $false } 3>&1 6>&1 | Out-String

        $output | Should -Match 'updated\s+pwsh\s+via choco'
        $script:ChocoArgs | Should -Contain 'outdated --limit-output'
        $script:ChocoArgs | Should -Contain 'upgrade powershell-core -y'
        $script:InstallFailures.Count | Should -Be 0
    }

    It "records failed Chocolatey outdated checks without updating the package" {
        . $script:ImportInstallDepsForTest
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\ProgramData\chocolatey\bin\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            switch ($joined) {
                'list powershell-core --local-only --exact --limit-output' {
                    $global:LASTEXITCODE = 0
                    return 'powershell-core|7.5.0'
                }
                'outdated --limit-output' {
                    $global:LASTEXITCODE = 44
                    return
                }
                default {
                    throw "choco must not update after a failed outdated check: $joined"
                }
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $false } 3>&1 6>&1 | Out-String

        $output | Should -Match 'choco outdated check of powershell-core failed'
        ($script:ChocoArgs | Where-Object { $_ -like 'upgrade*' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'pwsh'
        $script:InstallFailures[0].Pm | Should -Be 'choco'
        $script:InstallFailures[0].Pkg | Should -Be 'powershell-core'
        $script:InstallFailures[0].ExitCode | Should -Be 44
    }

    It "does not let Chocolatey list claim a manual command source" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Manual\PowerShell\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            if ($joined -eq 'list powershell-core --local-only --exact --limit-output') {
                $global:LASTEXITCODE = 0
                return 'powershell-core|7.5.0'
            }
            throw "choco must not update a manual command source: $joined"
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 6>&1 | Out-String

        $output | Should -Match 'unmanaged\s+pwsh\s+source=C:\\Manual\\PowerShell\\pwsh\.exe'
        $output | Should -Not -Match 'choco upgrade powershell-core'
        $script:ChocoArgs | Should -Contain 'list powershell-core --local-only --exact --limit-output'
        $script:InstallFailures.Count | Should -Be 0
        $script:UnmanagedDependencies.Count | Should -Be 1
    }

    It "blocks a Chocolatey-bin command source when the expected package is missing" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\ProgramData\chocolatey\bin\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            if ($joined -eq 'list powershell-core --local-only --exact --limit-output') {
                $global:LASTEXITCODE = 0
                return ''
            }
            throw "choco must not update after a package/source contradiction: $joined"
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 3>&1 6>&1 | Out-String

        $output | Should -Match 'blocked\s+pwsh'
        $output | Should -Match 'Chocolatey command source is under Chocolatey bin but package powershell-core is not installed'
        $output | Should -Not -Match 'choco upgrade powershell-core'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Pm | Should -Be 'choco'
        $script:InstallFailures[0].ExitCode | Should -Be 'manager-provenance'
        $script:UnmanagedDependencies.Count | Should -Be 0
    }

    It "reports present unmanaged PowerShell instead of pretending update ownership" {
        . $script:ImportInstallDepsForTest -DryRun

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Manual\PowerShell\pwsh.exe' }
            }
            return $null
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -ReportSkip -IsDryRun $true } 3>&1 6>&1 | Out-String

        $output | Should -Match 'unmanaged\s+pwsh\s+source=C:\\Manual\\PowerShell\\pwsh\.exe'
        $output | Should -Not -Match 'update skipped because|do not claim it'
        $script:InstallFailures.Count | Should -Be 0
        $script:UnmanagedDependencies.Count | Should -Be 1
        $script:UnmanagedDependencies[0].Tool | Should -Be 'pwsh'
    }

    It "fails closed instead of falling through when a Scoop shim has corrupt provenance" {
        . $script:ImportInstallDepsForTest -DryRun

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'rg') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Users\luigu\scoop\shims\rg.exe' }
            }
            return $null
        }
        Mock -CommandName Test-Path -MockWith {
            param([string]$LiteralPath)
            return ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\rg.shim')
        }
        Mock -CommandName Get-Content -MockWith {
            param([string]$LiteralPath)
            if ($LiteralPath -eq 'C:\Users\luigu\scoop\shims\rg.shim') {
                return 'path = "C:\Tools\rg.exe"'
            }
            throw "unexpected Get-Content path: $LiteralPath"
        }
        Mock -CommandName winget -MockWith { throw "winget must not run after corrupt Scoop shim provenance" }

        $output = & { Update-ManagedCatalogTool rg -AssumePresent -NoPrompt -ReportSkip -IsDryRun $true } 3>&1 6>&1 | Out-String

        $output | Should -Match 'blocked\s+rg'
        $output | Should -Match 'outside the apps tree'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'rg'
        $script:InstallFailures[0].Pm | Should -Be 'scoop'
        $script:InstallFailures[0].Pkg | Should -Be 'ripgrep'
        $script:InstallFailures[0].ExitCode | Should -Be 'scoop-shim-provenance'
        $script:UnmanagedDependencies.Count | Should -Be 0
    }

    It "records failed winget package updates" {
        . $script:ImportInstallDepsForTest
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Program Files\PowerShell\7\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            switch ($joined) {
                'list --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.5.0 winget'
                }
                'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.5.0 7.6.2 winget'
                }
                'upgrade --id Microsoft.PowerShell -e --accept-source-agreements --accept-package-agreements --silent' {
                    $global:LASTEXITCODE = 55
                    return
                }
                default {
                    $global:LASTEXITCODE = 0
                }
            }
        }

        & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -IsDryRun $false } 3>&1 6>&1 | Out-Null

        $script:WingetArgs | Should -Contain 'upgrade --id Microsoft.PowerShell -e --accept-source-agreements --accept-package-agreements --silent'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'pwsh'
        $script:InstallFailures[0].Pm | Should -Be 'winget'
        $script:InstallFailures[0].Pkg | Should -Be 'Microsoft.PowerShell'
        $script:InstallFailures[0].ExitCode | Should -Be 55
    }

    It "treats winget update-not-applicable during upgrade as a clean no-op" {
        . $script:ImportInstallDepsForTest
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Program Files\PowerShell\7\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            switch ($joined) {
                'list --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.5.0 winget'
                }
                'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.5.0 7.6.2 winget'
                }
                'upgrade --id Microsoft.PowerShell -e --accept-source-agreements --accept-package-agreements --silent' {
                    $global:LASTEXITCODE = -1978335189
                    return 'No applicable update found.'
                }
                default {
                    $global:LASTEXITCODE = 0
                }
            }
        }

        $output = & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -IsDryRun $false } 3>&1 6>&1 | Out-String

        $script:WingetArgs | Should -Contain 'upgrade --id Microsoft.PowerShell -e --accept-source-agreements --accept-package-agreements --silent'
        $output | Should -Match 'current\s+pwsh\s+via winget'
        $script:InstallFailures.Count | Should -Be 0
    }

    It "records failed Chocolatey package updates" {
        . $script:ImportInstallDepsForTest
        $script:ChocoArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'choco') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\ProgramData\chocolatey\bin\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName choco -MockWith {
            $joined = $args -join ' '
            $script:ChocoArgs += $joined
            switch ($joined) {
                'list powershell-core --local-only --exact --limit-output' {
                    $global:LASTEXITCODE = 0
                    return 'powershell-core|7.5.0'
                }
                'outdated --limit-output' {
                    $global:LASTEXITCODE = 2
                    return 'powershell-core|7.5.0|7.6.2|false'
                }
                'upgrade powershell-core -y' {
                    $global:LASTEXITCODE = 66
                    return
                }
                default {
                    $global:LASTEXITCODE = 0
                }
            }
        }

        & { Update-ManagedCatalogTool pwsh -AssumePresent -NoPrompt -IsDryRun $false } 3>&1 6>&1 | Out-Null

        $script:ChocoArgs | Should -Contain 'upgrade powershell-core -y'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'pwsh'
        $script:InstallFailures[0].Pm | Should -Be 'choco'
        $script:InstallFailures[0].Pkg | Should -Be 'powershell-core'
        $script:InstallFailures[0].ExitCode | Should -Be 66
    }

    It "dry-runs catalog update for present Scoop tools and skips absent tools" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            if ($joined -eq 'list git') {
                $global:LASTEXITCODE = 0
                return 'git 2.50.0'
            }
            if ($joined -eq 'status') {
                $global:LASTEXITCODE = 0
                return (New-ScoopStatusRow -Name 'git' -Installed '2.50.0' -Latest '2.51.0')
            }
            $global:LASTEXITCODE = 0
        }
        Mock -CommandName Install-WindowsTerminal -MockWith { throw "Windows Terminal installer must not run in update mode" }
        Mock -CommandName Install-HackNerdFont -MockWith { throw "font installer must not run in update mode" }
        Mock -CommandName Install-PSFzf -MockWith { throw "PSFzf installer must not run in update mode" }

        $specList = @(
            [pscustomobject]@{ Tool = 'git'; Kind = 'tool'; Binary = 'git'; Module = '' },
            [pscustomobject]@{ Tool = 'fd'; Kind = 'tool'; Binary = 'fd'; Module = '' }
        )
        # Out-String emits CRLF on Windows; strip CR so the (?m) row-end
        # anchors ([ \t]*$) match -- in .NET regex $ does not match before a bare \r.
        $output = (& {
            Invoke-InstallDepsUpdateMode -SpecList $specList -PresenceTester {
                param([string]$Tool)
                return ($Tool -eq 'git')
            } -IsDryRun $true
        } 6>&1 | Out-String) -replace "`r", ''

        $output | Should -Match '(?m)^\s*would:\s+scoop update[ \t]*$'
        $output | Should -Match '(?m)^\s*would:\s+scoop update git[ \t]*$'
        $output | Should -Match '(?m)^\s*skipped\s+fd\s+not installed[ \t]*$'
        $output | Should -Not -Match 'scoop install|winget install|choco install|Install-Module|Hack\.zip'
        $script:ScoopArgs | Should -Contain 'list git'
        $script:ScoopArgs | Should -Contain 'status'
        ($script:ScoopArgs | Where-Object { $_ -like 'update*' }).Count | Should -Be 0
        Should -Invoke -CommandName Install-WindowsTerminal -Times 0 -Exactly
        Should -Invoke -CommandName Install-HackNerdFont -Times 0 -Exactly
        Should -Invoke -CommandName Install-PSFzf -Times 0 -Exactly
    }

    It "does not refresh Scoop when no catalog tool is present" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
            throw "scoop must not run without a present owned target"
        }

        $output = & {
            Invoke-InstallDepsUpdateMode -SpecList @(
                [pscustomobject]@{ Tool = 'git'; Kind = 'tool'; Binary = 'git'; Module = '' },
                [pscustomobject]@{ Tool = 'fd'; Kind = 'tool'; Binary = 'fd'; Module = '' }
            ) -PresenceTester {
                return $false
            } -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'skipped\s+git\s+not installed'
        $output | Should -Match 'skipped\s+fd\s+not installed'
        $output | Should -Not -Match 'scoop update|scoop manifest refresh'
        $script:ScoopArgs.Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "does not refresh Scoop when winget owns the present target" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()
        $script:WingetArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'winget') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            if ($Name -eq 'pwsh') {
                return [pscustomobject]@{ Name = $Name; Source = 'C:\Program Files\PowerShell\7\pwsh.exe' }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $script:ScoopArgs += ($args -join ' ')
            throw "scoop must not run for a winget-owned target"
        }
        Mock -CommandName winget -MockWith {
            $joined = $args -join ' '
            $script:WingetArgs += $joined
            switch ($joined) {
                'list --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return 'PowerShell Microsoft.PowerShell 7.6.2 winget'
                }
                'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements' {
                    $global:LASTEXITCODE = 0
                    return ''
                }
                default {
                    throw "unexpected winget command: $joined"
                }
            }
        }

        $output = & {
            Invoke-InstallDepsUpdateMode -SpecList @(
                [pscustomobject]@{ Tool = 'pwsh'; Kind = 'tool'; Binary = 'pwsh'; Module = '' }
            ) -PresenceTester {
                param([string]$Tool)
                return ($Tool -eq 'pwsh')
            } -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'current\s+pwsh\s+via winget'
        $script:ScoopArgs.Count | Should -Be 0
        $script:WingetArgs | Should -Contain 'list --upgrade-available --id Microsoft.PowerShell -e --accept-source-agreements'
        $script:InstallFailures.Count | Should -Be 0
    }

    It "refreshes Scoop manifests once for multiple Scoop-owned targets" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            switch ($joined) {
                'list git' {
                    $global:LASTEXITCODE = 0
                    return 'git 2.50.0'
                }
                'list fd' {
                    $global:LASTEXITCODE = 0
                    return 'fd 10.3.0'
                }
                'update' {
                    $global:LASTEXITCODE = 0
                    return
                }
                'status' {
                    $global:LASTEXITCODE = 0
                    return @(
                        (New-ScoopStatusRow -Name 'git' -Installed '2.50.0' -Latest '2.51.0'),
                        (New-ScoopStatusRow -Name 'fd' -Installed '10.3.0' -Latest '10.4.0')
                    )
                }
                'update git' {
                    $global:LASTEXITCODE = 0
                    return
                }
                'update fd' {
                    $global:LASTEXITCODE = 0
                    return
                }
                default {
                    throw "unexpected scoop command: $joined"
                }
            }
        }

        $output = & {
            Invoke-InstallDepsUpdateMode -SpecList @(
                [pscustomobject]@{ Tool = 'git'; Kind = 'tool'; Binary = 'git'; Module = '' },
                [pscustomobject]@{ Tool = 'fd'; Kind = 'tool'; Binary = 'fd'; Module = '' }
            ) -PresenceTester {
                param([string]$Tool)
                return ($Tool -in @('git', 'fd'))
            } -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'updated\s+git\s+via scoop'
        $output | Should -Match 'updated\s+fd\s+via scoop'
        ($script:ScoopArgs | Where-Object { $_ -eq 'update' }).Count | Should -Be 1
        $script:ScoopArgs | Should -Contain 'update git'
        $script:ScoopArgs | Should -Contain 'update fd'
        $script:InstallFailures.Count | Should -Be 0
    }

    It "records failed Scoop package updates" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            switch ($joined) {
                'list git' {
                    $global:LASTEXITCODE = 0
                    return 'git 2.50.0'
                }
                'status' {
                    $global:LASTEXITCODE = 0
                    return (New-ScoopStatusRow -Name 'git' -Installed '2.50.0' -Latest '2.51.0')
                }
                'update git' {
                    $global:LASTEXITCODE = 44
                    return
                }
                default {
                    $global:LASTEXITCODE = 0
                }
            }
        }

        & { Update-ScoopTool git -NoPrompt -SkipManifestRefresh -AssumePresent } 3>&1 6>&1 | Out-Null

        $script:ScoopArgs | Should -Contain 'update git'
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'git'
        $script:InstallFailures[0].Pm | Should -Be 'scoop'
        $script:InstallFailures[0].Pkg | Should -Be 'git'
        $script:InstallFailures[0].ExitCode | Should -Be 44
    }

    It "records failed Scoop manifest refresh in update mode" {
        . $script:ImportInstallDepsForTest
        $script:ScoopArgs = @()

        Mock -CommandName Get-Command -MockWith {
            param([string]$Name)
            if ($Name -eq 'scoop') {
                return [pscustomobject]@{ Name = $Name; Source = $Name }
            }
            return $null
        }
        Mock -CommandName scoop -MockWith {
            $joined = $args -join ' '
            $script:ScoopArgs += $joined
            switch ($joined) {
                'list git' {
                    $global:LASTEXITCODE = 0
                    return 'git 2.50.0'
                }
                'update' {
                    $global:LASTEXITCODE = 33
                    return
                }
                default {
                    throw "scoop must not continue after a failed manifest refresh: $joined"
                }
            }
        }

        $output = & {
            Invoke-InstallDepsUpdateMode -SpecList @(
                [pscustomobject]@{ Tool = 'git'; Kind = 'tool'; Binary = 'git'; Module = '' }
            ) -PresenceTester {
                param([string]$Tool)
                return ($Tool -eq 'git')
            } -IsDryRun $false
        } 3>&1 6>&1 | Out-String

        $output | Should -Match 'scoop manifest refresh failed'
        $script:ScoopArgs | Should -Contain 'update'
        ($script:ScoopArgs | Where-Object { $_ -like 'status*' -or $_ -eq 'update git' }).Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'scoop'
        $script:InstallFailures[0].Pm | Should -Be 'scoop'
        $script:InstallFailures[0].Pkg | Should -Be 'manifest'
        $script:InstallFailures[0].ExitCode | Should -Be 33
    }

    It "exits nonzero when update mode records failures" {
        $pwsh = (Get-Command pwsh -ErrorAction Stop).Source
        $installDepsPath = $script:InstallDeps.Replace("'", "''")
        $command = @"
`$env:INSTALL_DEPS_PS1_SOURCE_ONLY = '1'
. '$installDepsPath'
Remove-Item Env:INSTALL_DEPS_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
function scoop {
    `$joined = `$args -join ' '
    switch (`$joined) {
        'update' {
            `$global:LASTEXITCODE = 33
            return
        }
        default {
            throw "unexpected scoop command: `$joined"
        }
    }
}
Update-ScoopTool git -NoPrompt -AssumePresent -AssumeManaged -IsDryRun:`$false
Exit-InstallDepsIfFailures
exit 0
"@

        $oldNativeCommandUseErrorActionPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $false
            $output = & $pwsh -NoProfile -ExecutionPolicy Bypass -Command $command *>&1 | Out-String
        } finally {
            $PSNativeCommandUseErrorActionPreference = $oldNativeCommandUseErrorActionPreference
        }

        $LASTEXITCODE | Should -Be 1
        $output | Should -Match 'scoop manifest refresh failed'
        $output | Should -Match '(?m)^\s*FAIL: scoop\s+via scoop\s+pkg=manifest'
        $output | Should -Match 'install-deps: completed with 1 FAILED install'
    }

    It "exits zero on a clean dry-run despite stale ambient LASTEXITCODE" {
        $pwsh = (Get-Command pwsh -ErrorAction Stop).Source
        $installDepsPath = $script:InstallDeps.Replace("'", "''")
        $command = @"
function winget { `$global:LASTEXITCODE = 0 }
`$global:LASTEXITCODE = 77
& '$installDepsPath' -DryRun -All
"@

        $oldNativeCommandUseErrorActionPreference = $PSNativeCommandUseErrorActionPreference
        try {
            $PSNativeCommandUseErrorActionPreference = $false
            $output = & $pwsh -NoProfile -ExecutionPolicy Bypass -Command $command *>&1 | Out-String
        } finally {
            $PSNativeCommandUseErrorActionPreference = $oldNativeCommandUseErrorActionPreference
        }

        $LASTEXITCODE | Should -Be 0
        $output | Should -Match 'install-deps: done'
    }
}

Describe "Set-VSCodeTheme" {
    BeforeAll {
        . $script:ImportInstallDepsForTest
        $script:ExpectedTheme = "Ros$([char]0xE9) Pine"
        # On disk the accented e is written as a pure-ASCII \u00e9 JSON escape so
        # the file reads back byte-identical under any code page (no mojibake).
        # This literal IS pure ASCII (backslash u 0 0 e 9), keeping this test
        # file PS-5.1-safe like the rest of the .ps1 sources.
        $script:ExpectedThemeText = 'Ros\u00e9 Pine'
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
            # The on-disk theme value is the pure-ASCII \u00e9 escape (Ros\u00e9 Pine),
            # NOT a literal accented byte. Match that exact escaped text so this
            # guard fails if a write path ever regresses to literal \u00e9 (which
            # the Windows PowerShell 5.1 ANSI Get-Content default would later
            # double-encode into the unresolvable "RosA(c) Pine" mojibake).
            $escapedTheme = [regex]::Escape($script:ExpectedThemeText)
            $escapedFont = [regex]::Escape($script:ExpectedFont)
            $Text | Should -Match ('"workbench\.colorTheme"\s*:\s*"' + $escapedTheme + '"')
            # Forced dark: both preferred slots = the same dark theme, and
            # autoDetect is a BARE JSON boolean false (string "false" is ignored
            # by VS Code and lets it fall back to the default theme).
            $Text | Should -Match ('"workbench\.preferredDarkColorTheme"\s*:\s*"' + $escapedTheme + '"')
            $Text | Should -Match ('"workbench\.preferredLightColorTheme"\s*:\s*"' + $escapedTheme + '"')
            $Text | Should -Match '"window\.autoDetectColorScheme"\s*:\s*false'
            $Text | Should -Not -Match '"window\.autoDetectColorScheme"\s*:\s*"false"'
            $Text | Should -Match ('"editor\.fontFamily"\s*:\s*"' + $escapedFont + '"')
            $Text | Should -Match ('"terminal\.integrated\.fontFamily"\s*:\s*"' + $escapedFont + '"')
            $Text | Should -Match '"workbench\.startupEditor"\s*:\s*"none"'
        }

        function Test-FileIsPureAscii {
            param([Parameter(Mandatory)][string]$Path)
            $bytes = [System.IO.File]::ReadAllBytes($Path)
            $nonAscii = @($bytes | Where-Object { $_ -gt 0x7F })
            $nonAscii.Count | Should -Be 0
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
        $settings.'workbench.preferredDarkColorTheme' | Should -Be $script:ExpectedTheme
        $settings.'workbench.preferredLightColorTheme' | Should -Be $script:ExpectedTheme
        $settings.'window.autoDetectColorScheme' | Should -BeFalse
        $settings.'window.autoDetectColorScheme' | Should -BeOfType [bool]
        $settings.'editor.fontFamily' | Should -Be $script:ExpectedFont
        $settings.'terminal.integrated.fontFamily' | Should -Be $script:ExpectedFont
        $settings.'workbench.startupEditor' | Should -Be 'none'
        Test-FileIsPureAscii -Path $settingsPath
    }

    It "merges strict JSON while preserving existing keys" {
        $settingsPath = New-SettingsPath
        Write-TestSettings -Path $settingsPath -Text "{`n  `"editor.fontSize`": 14`n}`n"

        Set-VSCodeTheme -SettingsPath $settingsPath

        $settings = Read-StrictSettings -Path $settingsPath
        $settings.'editor.fontSize' | Should -Be 14
        $settings.'workbench.colorTheme' | Should -Be $script:ExpectedTheme
        $settings.'workbench.preferredDarkColorTheme' | Should -Be $script:ExpectedTheme
        $settings.'workbench.preferredLightColorTheme' | Should -Be $script:ExpectedTheme
        $settings.'window.autoDetectColorScheme' | Should -BeFalse
        $settings.'window.autoDetectColorScheme' | Should -BeOfType [bool]
        $settings.'editor.fontFamily' | Should -Be $script:ExpectedFont
        $settings.'terminal.integrated.fontFamily' | Should -Be $script:ExpectedFont
        $settings.'workbench.startupEditor' | Should -Be 'none'
        Test-FileIsPureAscii -Path $settingsPath
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
        Test-FileIsPureAscii -Path $settingsPath
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

    It "self-heals a previously double-encoded theme value to pure ASCII" {
        # Reproduce the field bug: an earlier run left settings.json with the
        # double-encoded "RosA(c) Pine" byte sequence (C3 83 C2 A9), which VS
        # Code cannot resolve to a theme. A rerun must overwrite it with the
        # pure-ASCII \u00e9 escape so the theme loads again.
        $settingsPath = New-SettingsPath
        $mojibake = 'Ros' + [char]0x00C3 + [char]0x00A9 + ' Pine'
        Write-TestSettings -Path $settingsPath -Text ("{`n  `"workbench.colorTheme`": `"" + $mojibake + "`"`n}`n")

        Set-VSCodeTheme -SettingsPath $settingsPath

        Test-FileIsPureAscii -Path $settingsPath
        $settings = Read-StrictSettings -Path $settingsPath
        $settings.'workbench.colorTheme' | Should -Be $script:ExpectedTheme
        $settings.'workbench.preferredDarkColorTheme' | Should -Be $script:ExpectedTheme
    }

    It "preserves a non-ASCII JSONC comment verbatim while escaping the managed value" {
        # The JSONC editor only ASCII-normalizes the values it inserts/replaces.
        # A user comment with its own accented char must survive byte-for-byte
        # (the UTF-8 read pin prevents double-encoding it), NOT get rewritten as a
        # \uXXXX escape inside the comment text.
        $settingsPath = New-SettingsPath
        $cafe = 'caf' + [char]0x00E9
        Write-TestSettings -Path $settingsPath -Text ("// " + $cafe + " note`n{`n  `"editor.fontSize`": 14`n}`n")

        Set-VSCodeTheme -SettingsPath $settingsPath

        # Comment text round-trips intact (read back as UTF-8), NOT escaped/mangled.
        $utf8Text = [System.IO.File]::ReadAllText($settingsPath, [System.Text.UTF8Encoding]::new($false))
        $utf8Text | Should -Match ([regex]::Escape('// ' + $cafe + ' note'))
        # The managed theme value is still the pure-ASCII escape.
        $utf8Text | Should -Match ('"workbench\.colorTheme"\s*:\s*"' + [regex]::Escape($script:ExpectedThemeText) + '"')
        $settings = Read-StrictSettings -Path $settingsPath
        $settings.'workbench.colorTheme' | Should -Be $script:ExpectedTheme
    }

    It "preserves non-ASCII content in OTHER keys across a rewrite (no double-encode)" {
        # A user value with its own accented char must survive a read/modify/write
        # round-trip intact -- the UTF-8 read pin is what prevents 5.1 from
        # decoding it as ANSI and double-encoding it on the way back out.
        $settingsPath = New-SettingsPath
        $cafe = 'caf' + [char]0x00E9
        Write-TestSettings -Path $settingsPath -Text ("{`n  `"some.userValue`": `"" + $cafe + "`"`n}`n")

        Set-VSCodeTheme -SettingsPath $settingsPath

        Test-FileIsPureAscii -Path $settingsPath
        $settings = Read-StrictSettings -Path $settingsPath
        $settings.'some.userValue' | Should -Be $cafe
        $settings.'workbench.colorTheme' | Should -Be $script:ExpectedTheme
    }
}

Describe "ConvertTo-AsciiJson" {
    BeforeAll {
        . $script:ImportInstallDepsForTest
    }

    It "escapes non-ASCII chars to lowercase \uXXXX and leaves ASCII untouched" {
        $input = 'Ros' + [char]0x00E9 + ' Pine'
        $out = ConvertTo-AsciiJson -Json $input
        $out | Should -Be 'Ros\u00e9 Pine'
        # Output must be pure ASCII.
        @([System.Text.Encoding]::UTF8.GetBytes($out) | Where-Object { $_ -gt 0x7F }).Count | Should -Be 0
    }

    It "is a no-op for already-ASCII JSON" {
        $input = '{"a":1,"b":"plain"}'
        (ConvertTo-AsciiJson -Json $input) | Should -Be $input
    }

    It "round-trips back to the original through ConvertFrom-Json" {
        $obj = [pscustomobject]@{ 'workbench.colorTheme' = ('Ros' + [char]0x00E9 + ' Pine') }
        $ascii = ConvertTo-AsciiJson -Json ($obj | ConvertTo-Json -Compress)
        ($ascii | ConvertFrom-Json).'workbench.colorTheme' | Should -Be ('Ros' + [char]0x00E9 + ' Pine')
    }
}

Describe "Python install rejects the Microsoft Store stub" {
    It "Get-RealPythonCommand returns null when only the WindowsApps stub is on PATH" {
        . $script:ImportInstallDepsForTest
        Mock Get-Command {
            [pscustomobject]@{ Source = "C:\Users\U\AppData\Local\Microsoft\WindowsApps\$Name.exe" }
        } -ParameterFilter { $Name -in @('python', 'python3') }
        Get-RealPythonCommand | Should -BeNullOrEmpty
    }

    It "Get-RealPythonCommand returns the real python when one exists alongside the stub" {
        . $script:ImportInstallDepsForTest
        Mock Get-Command {
            @(
                [pscustomobject]@{ Source = "C:\Users\U\AppData\Local\Microsoft\WindowsApps\python.exe" },
                [pscustomobject]@{ Source = "C:\Users\U\scoop\shims\python.exe" }
            )
        } -ParameterFilter { $Name -eq 'python' }
        (Get-RealPythonCommand).Source | Should -Be "C:\Users\U\scoop\shims\python.exe"
    }

    It "Install-One consults the custom -InstalledCheck and short-circuits when true" {
        . $script:ImportInstallDepsForTest
        $script:probeCount = 0
        Install-One 'python' -InstalledCheck { $script:probeCount++; $true } -SkipPrompt -NoRecordFailure
        $script:probeCount | Should -BeGreaterThan 0
    }
}

Describe "Markdown equation converter provisioning" {
    BeforeEach {
        $script:OldLocalAppData = $env:LOCALAPPDATA
        $env:LOCALAPPDATA = Join-Path $TestDrive 'localappdata'
        New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA | Out-Null
    }

    AfterEach {
        if ($null -eq $script:OldLocalAppData) {
            Remove-Item Env:LOCALAPPDATA -ErrorAction SilentlyContinue
        } else {
            $env:LOCALAPPDATA = $script:OldLocalAppData
        }
    }

    It "dry-run previews the pinned pylatexenc venv install" {
        . $script:ImportInstallDepsForTest -DryRun
        $output = Install-PylatexencConverter 6>&1 | Out-String

        $output | Should -Match 'python -m venv'
        $output | Should -Match 'setuptools==80\.9\.0'
        $output | Should -Match $PylatexencBuildBackendSha256
        $output | Should -Match 'pylatexenc==2\.10'
        $output | Should -Match $PylatexencSha256
    }

    It "creates the pinned venv and adds latex2text to User PATH" {
        . $script:ImportInstallDepsForTest
        $script:AddedPath = ''
        $script:SetuptoolsRequirementsText = ''
        $script:RequirementsText = ''
        $script:PythonCalls = @()

        Mock Get-RealPythonCommand {
            [pscustomobject]@{ Source = 'python' }
        }
        Mock Add-DirectoryToUserPath {
            param([string]$Directory)
            $script:AddedPath = $Directory
        }
        Mock Invoke-PythonCommand {
            param([string]$Python, [string[]]$Arguments)
            $script:PythonCalls += [pscustomobject]@{ Python = $Python; Arguments = $Arguments }
            $global:LASTEXITCODE = 0
            if ($Arguments[0] -eq '-m' -and $Arguments[1] -eq 'venv') {
                $scripts = Join-Path $Arguments[2] 'Scripts'
                New-Item -ItemType Directory -Force -Path $scripts | Out-Null
                New-Item -ItemType File -Force -Path (Join-Path $scripts 'python.exe') | Out-Null
                New-Item -ItemType File -Force -Path (Join-Path $scripts 'latex2text.exe') | Out-Null
            }
            if ($Arguments[0] -eq '-m' -and $Arguments[1] -eq 'pip') {
                if ($Arguments -contains '--only-binary=:all:') {
                    $script:SetuptoolsRequirementsText = Get-Content -LiteralPath $Arguments[-1] -Raw
                } else {
                    $script:RequirementsText = Get-Content -LiteralPath $Arguments[-1] -Raw
                }
            }
        }

        Install-PylatexencConverter

        $expectedScripts = Join-Path (Get-PylatexencVenvRoot) 'Scripts'
        $script:AddedPath | Should -Be $expectedScripts
        $script:SetuptoolsRequirementsText | Should -Match 'setuptools==80\.9\.0'
        $script:SetuptoolsRequirementsText | Should -Match $PylatexencBuildBackendSha256
        $script:RequirementsText | Should -Match 'pylatexenc==2\.10'
        $script:RequirementsText | Should -Match $PylatexencSha256
        @($script:PythonCalls | Where-Object { $_.Arguments -contains '--require-hashes' }).Count | Should -Be 2
        @($script:PythonCalls | Where-Object { $_.Arguments -contains '--no-build-isolation' }).Count | Should -Be 1
    }
}

Describe "psmux session plugin provisioning" {
    It "pins psmux/psmux-plugins to an immutable 40-char commit SHA" {
        . $script:ImportInstallDepsForTest
        $PsmuxPluginsCommit | Should -Match '^[0-9a-f]{40}$'
    }

    It "vendors into a fixed .psmux/plugins root under the user profile" {
        . $script:ImportInstallDepsForTest
        $oldUser = $env:USERPROFILE
        try {
            $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N'))
            $env:USERPROFILE = $tmp
            $root = Get-PsmuxPluginRoot
            $root | Should -BeLike "$tmp*"
            $root | Should -Match '[\\/]\.psmux[\\/]plugins$'
        } finally {
            $env:USERPROFILE = $oldUser
        }
    }

    It "treats a plugin as pinned only when the entry file AND a matching commit marker exist" {
        . $script:ImportInstallDepsForTest
        $dir = Join-Path ([System.IO.Path]::GetTempPath()) ("ppp-" + [System.Guid]::NewGuid().ToString('N'))
        try {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
            # no entry file yet
            Test-PsmuxPluginPinned -Dir $dir -RequiredFile 'plugin.conf' | Should -BeFalse
            Set-Content -LiteralPath (Join-Path $dir 'plugin.conf') -Value 'x'
            # entry file, but no marker
            Test-PsmuxPluginPinned -Dir $dir -RequiredFile 'plugin.conf' | Should -BeFalse
            Set-Content -LiteralPath (Join-Path $dir '.pinned-commit') -Value 'deadbeef' -NoNewline
            # marker mismatch
            Test-PsmuxPluginPinned -Dir $dir -RequiredFile 'plugin.conf' | Should -BeFalse
            Set-Content -LiteralPath (Join-Path $dir '.pinned-commit') -Value $PsmuxPluginsCommit -NoNewline
            # entry file + matching marker
            Test-PsmuxPluginPinned -Dir $dir -RequiredFile 'plugin.conf' | Should -BeTrue
        } finally {
            Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue
        }
    }

    It "does not clone in DryRun mode" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:GitCalled = $false
        Mock -CommandName Ask -MockWith { $true }
        Mock -CommandName git -MockWith { $script:GitCalled = $true }
        $oldUser = $env:USERPROFILE
        try {
            $env:USERPROFILE = (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N')))
            Install-PsmuxPlugins
        } finally {
            $env:USERPROFILE = $oldUser
        }
        $script:GitCalled | Should -BeFalse
        $script:InstallFailures.Count | Should -Be 0
    }

    It "fails closed (records a blocker) when the pinned commit cannot be checked out" {
        . $script:ImportInstallDepsForTest
        Mock -CommandName Ask -MockWith { $true }
        # git operations succeed, but rev-parse reports the WRONG commit.
        Mock -CommandName git -MockWith {
            if ($args -contains 'rev-parse') { return 'deadbeefdeadbeefdeadbeefdeadbeefdeadbeef' }
            $global:LASTEXITCODE = 0
        }
        $oldUser = $env:USERPROFILE
        try {
            $env:USERPROFILE = (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N')))
            New-Item -ItemType Directory -Path $env:USERPROFILE -Force | Out-Null
            Install-PsmuxPlugins
        } finally {
            if ($env:USERPROFILE -and (Test-Path $env:USERPROFILE)) { Remove-Item -Recurse -Force $env:USERPROFILE -ErrorAction SilentlyContinue }
            $env:USERPROFILE = $oldUser
        }
        ($script:InstallFailures | Where-Object { $_.Tool -eq 'psmux plugins' }).Count | Should -BeGreaterThan 0
    }

    It "is a no-op (no git) when psmux-resurrect is already pinned" {
        . $script:ImportInstallDepsForTest
        $script:GitCalled = $false
        Mock -CommandName git -MockWith { $script:GitCalled = $true }
        $oldUser = $env:USERPROFILE
        try {
            $env:USERPROFILE = (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N')))
            $root = Get-PsmuxPluginRoot
            $d = Join-Path $root 'psmux-resurrect'
            New-Item -ItemType Directory -Path $d -Force | Out-Null
            Set-Content -LiteralPath (Join-Path $d 'plugin.conf') -Value 'x'
            Set-Content -LiteralPath (Join-Path $d '.pinned-commit') -Value $PsmuxPluginsCommit -NoNewline
            Install-PsmuxPlugins
        } finally {
            if ($env:USERPROFILE -and (Test-Path $env:USERPROFILE)) { Remove-Item -Recurse -Force $env:USERPROFILE -ErrorAction SilentlyContinue }
            $env:USERPROFILE = $oldUser
        }
        $script:GitCalled | Should -BeFalse
        $script:InstallFailures.Count | Should -Be 0
    }

    It "never vendors psmux-continuum (blocked pending real Windows verification)" {
        . $script:ImportInstallDepsForTest
        # continuum is a documented blocker, so the vendor must never create a
        # psmux-continuum directory. The mocked fetch produces no real checkout,
        # so this fails closed, but the point is it never touches continuum.
        Mock -CommandName Ask -MockWith { $true }
        Mock -CommandName git -MockWith {
            if ($args -contains 'rev-parse') { return $PsmuxPluginsCommit }
            $global:LASTEXITCODE = 0
        }
        $oldUser = $env:USERPROFILE
        try {
            $env:USERPROFILE = (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N')))
            $root = Get-PsmuxPluginRoot
            Install-PsmuxPlugins
            (Test-Path -LiteralPath (Join-Path $root 'psmux-continuum')) | Should -BeFalse
        } finally {
            if ($env:USERPROFILE -and (Test-Path $env:USERPROFILE)) { Remove-Item -Recurse -Force $env:USERPROFILE -ErrorAction SilentlyContinue }
            $env:USERPROFILE = $oldUser
        }
    }
}

Describe "Install-GhDashExtension" {
    BeforeEach {
        $script:GhAuthRc = 0
        $script:GhListRc = 0
        $script:GhListOut = @()
        $script:GhInstallRc = 0
        $script:GhRemoveRc = 0
        $script:GhApiRc = 0
        $script:GhTagObjectResult = 'e6ebbd7e83e30161b9192ce3339972d2c8269e7f'
        $script:GhPeeledCommitResult = '49f37e4832956c57bf52d4ea8b1b1e5c0f863700'
        $script:GhCalls = @()
    }

    It "skips cleanly when gh is not installed (no failure recorded)" {
        . $script:ImportInstallDepsForTest -DryRun
        Mock -CommandName Get-Command -MockWith { $null } -ParameterFilter { $Name -eq 'gh' }
        Set-GhDashMock
        Install-GhDashExtension
        $script:InstallFailures.Count | Should -Be 0
        $script:GhCalls.Count | Should -Be 0
    }

    It "skips without a failure when gh is unauthenticated, pointing at gh auth login" {
        . $script:ImportInstallDepsForTest -DryRun
        $script:GhAuthRc = 1
        Set-GhDashMock
        $out = (Install-GhDashExtension 6>&1 | Out-String)
        $out | Should -Match 'gh auth login'
        $script:InstallFailures.Count | Should -Be 0
        $script:GhCalls.Count | Should -Be 0
    }

    It "prints the pinned install command in dry-run when authenticated and missing" {
        . $script:ImportInstallDepsForTest -DryRun
        Set-GhDashMock
        $out = (Install-GhDashExtension 6>&1 | Out-String)
        $out | Should -Match ([regex]::Escape("tag object $GhDashTagObject peels to $GhDashCommit"))
        $out | Should -Match ([regex]::Escape("gh extension install dlvhdr/gh-dash --pin $GhDashVersion"))
        $script:GhCalls.Count | Should -Be 0
    }

    It "is idempotent when installed at the expected pin" {
        . $script:ImportInstallDepsForTest
        $script:GhListOut = @("gh dash`tdlvhdr/gh-dash`t$GhDashVersion")
        Set-GhDashMock
        $out = (Install-GhDashExtension 6>&1 | Out-String)
        $out | Should -Match 'already installed'
        $script:GhCalls.Count | Should -Be 0
        $script:InstallFailures.Count | Should -Be 0
    }

    It "force re-pins when installed at a different pin" {
        . $script:ImportInstallDepsForTest
        $script:GhListOut = @("gh dash`tdlvhdr/gh-dash`tv4.20.0")
        Set-GhDashMock
        Install-GhDashExtension
        ($script:GhCalls -join "`n") | Should -Match 'remove dash'
        ($script:GhCalls -join "`n") | Should -Match ([regex]::Escape("install dlvhdr/gh-dash --pin $GhDashVersion"))
        $script:InstallFailures.Count | Should -Be 0
    }

    It "records a failure when the authenticated install fails" {
        . $script:ImportInstallDepsForTest
        $script:GhInstallRc = 1
        Set-GhDashMock
        Install-GhDashExtension
        $script:InstallFailures.Count | Should -Be 1
        $script:InstallFailures[0].Tool | Should -Be 'gh-dash'
    }

    It "rejects a moved gh-dash tag before extension mutation" {
        . $script:ImportInstallDepsForTest
        $script:GhTagObjectResult = 'deadbeef'
        Set-GhDashMock

        $out = (Install-GhDashExtension 6>&1 | Out-String)

        $out | Should -Match 'tag object mismatch'
        $script:InstallFailures.Count | Should -Be 1
        $script:GhCalls.Count | Should -Be 0
    }

    It "does not leak a nonzero LASTEXITCODE from native gh probes (PSNativeCommandUseErrorActionPreference)" {
        $PSNativeCommandUseErrorActionPreference = $true
        . $script:ImportInstallDepsForTest -DryRun
        $script:GhListRc = 1
        Set-GhDashMock
        $global:LASTEXITCODE = 0
        { Install-GhDashExtension } | Should -Not -Throw
        $LASTEXITCODE | Should -Be 0
    }
}

Describe "WezTerm catalog entry" {
    It "declares wezterm with winget/scoop/choco package IDs (native Windows install path)" {
        . $script:ImportInstallDepsForTest -DryRun
        $Catalog.ContainsKey('wezterm') | Should -BeTrue
        $Catalog['wezterm'].winget  | Should -Be 'wez.wezterm'
        $Catalog['wezterm'].choco   | Should -Be 'wezterm'
        $Catalog['wezterm'].scoop   | Should -Be 'extras/wezterm'
        $BinaryName['wezterm']      | Should -Be 'wezterm'
        $Catalog['wezterm'].purpose | Should -Match 'WezTerm'
    }
}
