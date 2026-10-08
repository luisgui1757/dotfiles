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
