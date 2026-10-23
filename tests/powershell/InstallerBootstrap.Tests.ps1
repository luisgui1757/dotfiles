BeforeAll {
    $script:Launcher = Join-Path $PSScriptRoot '../../scripts/installer-bootstrap.ps1'
}

Describe 'Trusted checkout bootstrap entrypoint' {
    It 'parses without errors under this PowerShell engine' {
        $tokens = $null
        $errors = $null
        [Management.Automation.Language.Parser]::ParseFile($script:Launcher, [ref]$tokens, [ref]$errors) | Out-Null
        @($errors).Count | Should -Be 0
    }

    It 'rejects released setup flags before cache or download work' {
        { & $script:Launcher --all } | Should -Throw '*without arguments*'
        { & $script:Launcher -All } | Should -Throw '*without arguments*'
        { & $script:Launcher machine unexpected } | Should -Throw '*without arguments*'
    }

    It 'rejects a 32-bit process before cache or download work' {
        $oldOS = $env:OS
        $oldArchitecture = $env:PROCESSOR_ARCHITECTURE
        try {
            $env:OS = 'Windows_NT'
            $env:PROCESSOR_ARCHITECTURE = 'x86'
            { & $script:Launcher } | Should -Throw '*native Windows amd64*'
        } finally {
            $env:OS = $oldOS
            $env:PROCESSOR_ARCHITECTURE = $oldArchitecture
        }
    }
}

Describe 'Bootstrap phase diagnostics' {
    BeforeAll {
        $tokens = $null
        $errors = $null
        $ast = [Management.Automation.Language.Parser]::ParseFile($script:Launcher, [ref]$tokens, [ref]$errors)
        $phase = $ast.Find({ param($node)
            $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Invoke-BootstrapPhase'
        }, $true)
        if ($null -eq $phase) { throw 'Bootstrap has no phase diagnostics' }
        Set-Item Function:\Invoke-BootstrapPhase -Value ($phase.Body.GetScriptBlock())
    }

    It 'keeps result output unchanged and sends timing only to stderr' {
        $prior = [Console]::Error
        $diagnostics = [IO.StringWriter]::new()
        try {
            [Console]::SetError($diagnostics)
            $result = @(Invoke-BootstrapPhase 'fixture' { '{"status":"ready"}' })
            $result.Count | Should -Be 1
            $result[0] | Should -Be '{"status":"ready"}'
            $diagnostics.ToString() | Should -Match 'Bootstrap fixture: started'
            $diagnostics.ToString() | Should -Match 'Bootstrap fixture: completed in [0-9]+ ms'
        } finally {
            [Console]::SetError($prior)
            $diagnostics.Dispose()
        }
    }

    It 'reports the failed phase and preserves the original terminating error' {
        $prior = [Console]::Error
        $diagnostics = [IO.StringWriter]::new()
        try {
            [Console]::SetError($diagnostics)
            { Invoke-BootstrapPhase 'fixture failure' { throw 'exact original boundary error' } } |
                Should -Throw '*exact original boundary error*'
            $diagnostics.ToString() | Should -Match 'Bootstrap fixture failure: failed after [0-9]+ ms'
            $diagnostics.ToString() | Should -Not -Match 'completed'
        } finally {
            [Console]::SetError($prior)
            $diagnostics.Dispose()
        }
    }
}
Describe 'Bootstrap download progress' {
    BeforeAll {
        $tokens = $null
        $errors = $null
        $ast = [Management.Automation.Language.Parser]::ParseFile($script:Launcher, [ref]$tokens, [ref]$errors)
        $download = $ast.Find({ param($node)
            $node -is [Management.Automation.Language.CommandAst] -and
            $node.GetCommandName() -eq 'Invoke-BootstrapPhase' -and
            $node.CommandElements[1].Value -eq 'download'
        }, $true)
        $script:DownloadAction = $download.CommandElements[2].ScriptBlock.GetScriptBlock()
    }

    It 'suppresses per-byte progress only during the actual download' {
        $ProgressPreference = 'Continue'
        $Pin = @{ filename = 'pinned.zip' }
        $expectedURI = "https://go.dev/dl/$($Pin.filename)"
        $Download = Join-Path $TestDrive 'archive.zip'
        Mock Invoke-WebRequest { $script:DownloadProgress = $ProgressPreference }
        . $script:DownloadAction
        $script:DownloadProgress | Should -Be 'SilentlyContinue'
        $ProgressPreference | Should -Be 'Continue'
        Should -Invoke Invoke-WebRequest -Times 1 -Exactly -ParameterFilter {
            $UseBasicParsing -and $Uri -eq $expectedURI -and $OutFile -eq $Download
        }
    }

    It 'restores progress preference and propagates a failed download' {
        $ProgressPreference = 'Continue'
        $Pin = @{ filename = 'pinned.zip' }
        $expectedURI = "https://go.dev/dl/$($Pin.filename)"
        $Download = Join-Path $TestDrive 'failed.zip'
        Mock Invoke-WebRequest {
            $script:DownloadProgress = $ProgressPreference
            throw 'exact network failure'
        }
        { . $script:DownloadAction } | Should -Throw '*exact network failure*'
        $script:DownloadProgress | Should -Be 'SilentlyContinue'
        $ProgressPreference | Should -Be 'Continue'
        Should -Invoke Invoke-WebRequest -Times 1 -Exactly -ParameterFilter {
            $UseBasicParsing -and $Uri -eq $expectedURI -and $OutFile -eq $Download
        }
    }
}

Describe 'Bootstrap compiler module-cache isolation' {
    BeforeAll {
        $tokens = $null
        $errors = $null
        $ast = [Management.Automation.Language.Parser]::ParseFile($script:Launcher, [ref]$tokens, [ref]$errors)
        $compiler = $ast.Find({ param($node)
            $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Invoke-Compiler'
        }, $true)
        if ($null -eq $compiler) { throw 'Bootstrap has no compiler boundary' }
        Set-Item Function:\Invoke-Compiler -Value ($compiler.Body.GetScriptBlock())
    }

    It 'confines module writes and restores the inherited cache after child exit <ExitCode>, native errors <NativePreference>' -TestCases @(
        @{ ExitCode = 0; NativePreference = $false }
        @{ ExitCode = 17; NativePreference = $false }
        @{ ExitCode = 0; NativePreference = $true }
        @{ ExitCode = 17; NativePreference = $true }
    ) {
        param($ExitCode, $NativePreference)
        $ErrorActionPreference = 'Stop'
        $PSNativeCommandUseErrorActionPreference = $NativePreference
        $prior = [Environment]::GetEnvironmentVariable('GOMODCACHE')
        $Cache = Join-Path $TestDrive 'bootstrap cache'
        $Toolchain = Join-Path $TestDrive 'toolchain'
        $Stage = Join-Path $TestDrive 'stage'
        $Go = (Get-Process -Id $PID).Path
        $personal = Join-Path $TestDrive 'personal module cache'
        try {
            $env:GOMODCACHE = $personal
            $probe = @'
$expected = [IO.Path]::Combine($env:GOPATH, 'pkg', 'mod')
if ($env:GOMODCACHE -cne $expected) { [Console]::Error.WriteLine('compiler inherited personal GOMODCACHE'); exit 44 }
[IO.Directory]::CreateDirectory($env:GOMODCACHE) > $null
[IO.File]::WriteAllLines([IO.Path]::Combine($env:GOMODCACHE, 'compiler-probe'), [string[]]@(
    'private module bytes', $env:GOROOT, $env:GOTMPDIR, [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
))
exit EXIT_CODE
'@.Replace('EXIT_CODE', [string]$ExitCode)
            if ($ExitCode -eq 0) {
                Invoke-Compiler -NoLogo -NoProfile -NonInteractive -Command $probe
            } else {
                $failure = { Invoke-Compiler -NoLogo -NoProfile -NonInteractive -Command $probe } |
                    Should -Throw -PassThru
                if ($NativePreference) {
                    $failure.Exception.GetType().FullName | Should -BeExactly 'System.Management.Automation.NativeCommandExitException'
                    $failure.Exception.ExitCode | Should -Be 17
                } else {
                    $failure.Exception.Message | Should -BeExactly 'Go compiler failed (exit 17)'
                }
            }
            $env:GOMODCACHE | Should -BeExactly $personal
            (Test-Path -LiteralPath $personal) | Should -BeFalse
            $observed = [IO.File]::ReadAllLines((Join-Path $Cache 'gopath/pkg/mod/compiler-probe'))
            $observed.Count | Should -Be 4
            $observed[0] | Should -BeExactly 'private module bytes'
            $observed[1] | Should -BeExactly (Join-Path $Toolchain 'go')
            $observed[2] | Should -BeExactly $Stage
            $observed[3] | Should -BeExactly $Go
        } finally {
            [Environment]::SetEnvironmentVariable('GOMODCACHE', $prior)
        }
    }
}

Describe 'Bootstrap verified build reuse' {
    BeforeAll {
        $tokens = $null
        $errors = $null
        $ast = [Management.Automation.Language.Parser]::ParseFile($script:Launcher, [ref]$tokens, [ref]$errors)
        foreach ($name in @('Get-SHA256', 'Get-CheckoutFingerprint')) {
            $function = $ast.Find({ param($node)
                $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
            }, $true)
            if ($null -eq $function) { throw "Bootstrap has no $name" }
            Set-Item "Function:\$name" -Value ($function.Body.GetScriptBlock())
        }
    }

    BeforeEach {
        $script:Checkout = Join-Path ([IO.Path]::GetTempPath()) ("bootstrap fingerprint " + [char]0x00FC + " " + [Guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path (Join-Path $script:Checkout 'installer/cmd') -Force | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $script:Checkout 'scripts') -Force | Out-Null
        Set-Content -LiteralPath (Join-Path $script:Checkout 'installer/cmd/main.go') -Value 'package main' -NoNewline
        Set-Content -LiteralPath (Join-Path $script:Checkout 'scripts/installer-bootstrap.ps1') -Value '# launcher' -NoNewline
        $script:Pin = [pscustomobject]@{ version = 'go1.27.1'; platform = 'windows-amd64'; sha256 = 'a' * 64 }
        $Checkout = $script:Checkout
        $Pin = $script:Pin
    }

    AfterEach {
        Remove-Item -LiteralPath $script:Checkout -Recurse -Force
    }

    It 'is stable for an unchanged checkout and changes with any installer file' {
        $Checkout = $script:Checkout
        $Pin = $script:Pin
        $first = Get-CheckoutFingerprint
        $first | Should -Match '^[a-f0-9]{64}$'
        Get-CheckoutFingerprint | Should -BeExactly $first
        New-Item -ItemType Directory -Path (Join-Path $Checkout 'installer/testdata') | Out-Null
        Set-Content -LiteralPath (Join-Path $Checkout 'installer/testdata/notes.md') -Value 'not a build input'
        $added = Get-CheckoutFingerprint
        $added | Should -Not -Be $first
        Set-Content -LiteralPath (Join-Path $Checkout 'scripts/installer-bootstrap.ps1') -Value '# changed launcher'
        Get-CheckoutFingerprint | Should -Not -Be $added
    }

    It 'disables reuse when the installer tree contains a reparse point' {
        $Checkout = $script:Checkout
        $Pin = $script:Pin
        $link = Join-Path $Checkout 'installer/redirected'
        $target = Join-Path $Checkout 'scripts'
        if ($IsWindows -or $PSVersionTable.PSEdition -eq 'Desktop') {
            New-Item -ItemType Junction -Path $link -Target $target | Out-Null
        } else {
            New-Item -ItemType SymbolicLink -Path $link -Target $target | Out-Null
        }
        Get-CheckoutFingerprint | Should -BeNullOrEmpty
    }
}
