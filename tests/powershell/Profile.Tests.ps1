BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Profile  = Join-Path $script:RepoRoot "shells/powershell_profile.ps1"
}

Describe "PowerShell profile" {

    It "passes PSScriptAnalyzer with no Warning+ findings" {
        $diags = Invoke-ScriptAnalyzer -Path $script:Profile -Severity Warning,Error
        if ($diags) { $diags | Format-Table | Out-String | Write-Host }
        $diags.Count | Should -Be 0
    }

    It "dot-sources cleanly even when starship is not on PATH" {
        # Resolve the pwsh full path BEFORE modifying $env:PATH -- once the
        # sandbox replaces PATH, the short name pwsh cannot be found.
        $pwshExe = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
        if (-not $pwshExe) {
            $pwshExe = (Get-Command pwsh.exe -ErrorAction SilentlyContinue).Source
        }
        $pwshExe | Should -Not -BeNullOrEmpty

        # Use a sandbox PATH that does not contain starship.
        $sandbox = Join-Path ([System.IO.Path]::GetTempPath()) ("ps-sandbox-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path $sandbox | Out-Null
        $oldPath = $env:PATH
        try {
            $env:PATH = $sandbox
            $rc = 0
            & $pwshExe -NoProfile -Command "& { . `"$($script:Profile.Replace('"','`"'))`"; exit 0 }"
            $rc = $LASTEXITCODE
            $rc | Should -Be 0
        } finally {
            $env:PATH = $oldPath
            Remove-Item -Recurse -Force $sandbox -ErrorAction SilentlyContinue
        }
    }

    It "uses an approved verb (Confirm- not Ensure-)" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'function Confirm-StarshipInitScript'
        $src | Should -Not -Match 'function Ensure-StarshipInitScript'
    }

    It "checks UserInteractive before host-name refinement" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $userInteractiveIndex = $src.IndexOf('[Environment]::UserInteractive')
        $hostNameIndex = $src.IndexOf('$Host.Name')
        $userInteractiveIndex | Should -BeGreaterOrEqual 0
        $hostNameIndex | Should -BeGreaterThan $userInteractiveIndex
    }

    It "writes the starship init cache with UTF8 no-BOM encoding" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'function Publish-StarshipInitScript'
        $src | Should -Match '\[System\.Text\.UTF8Encoding\]::new\(\$false\)'
    }

    It "publishes the starship init cache atomically from a temp file" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match '"\{0\}\.\{1\}\.\{2\}\.tmp"\s+-f\s+\$initLeaf,\s+\$PID'
        $src | Should -Match 'Move-Item\s+-LiteralPath\s+\$tempPath\s+-Destination\s+\$InitPath\s+-Force'
        $src | Should -Match 'function Import-StarshipInitScriptWithRetry'
        $src | Should -Match 'Start-Sleep\s+-Milliseconds\s+\$DelayMilliseconds'
    }

    It "configures PSReadLine with Rose Pine colors" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'PredictionViewStyle\s+ListView'
        $src | Should -Match '#c4a7e7'   # iris
        $src | Should -Match '#f6c177'   # gold
        $src | Should -Match 'RosePineSelectionColor'
        $src | Should -Match '\[38;2;224;222;244;48;2;38;35;58m'
        $src | Should -Match 'Set-PSReadLineOption\s+-Colors\s+@\{\s*Selection\s*=\s*\$script:RosePineSelectionColor'
    }
}
