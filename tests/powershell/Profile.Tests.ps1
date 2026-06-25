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
        $src | Should -Match '\[38;2;246;193;119m'   # gold foreground (selected completion option is gold)
        $src | Should -Match 'Set-PSReadLineOption\s+-Colors\s+@\{\s*Selection\s*=\s*\$script:RosePineSelectionColor'
    }

    It "wires lsd functions only when lsd exists" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'Get-Command\s+lsd\s+-ErrorAction\s+SilentlyContinue'
        $src | Should -Match "foreach\s+\(\s*\`$name\s+in\s+'ls',\s+'l',\s+'la',\s+'lla',\s+'lt'\s*\)"
        $src | Should -Match 'Remove-Item\s+"Alias:\$name"\s+-ErrorAction\s+SilentlyContinue'
        $src | Should -Match 'function global:ls\s+\{\s+lsd\s+@args\s+\}'
        $src | Should -Match 'function global:l\s+\{\s+lsd\s+-l\s+@args\s+\}'
        $src | Should -Match 'function global:la\s+\{\s+lsd\s+-a\s+@args\s+\}'
        $src | Should -Match 'function global:lla\s+\{\s+lsd\s+-la\s+@args\s+\}'
        $src | Should -Match 'function global:lt\s+\{\s+lsd\s+--tree\s+@args\s+\}'
    }

    It "sets an authoritative Rose Pine LS_COLORS palette before wiring lsd" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match '\$script:RosePineLsColors\s+=\s+@\('
        $src | Should -Match "'di=38;2;246;193;119'"
        $src | Should -Match "'ex=38;2;235;111;146'"
        foreach ($class in 'no', 'fi', 'di', 'ln', 'pi', 'so', 'do', 'bd', 'cd', 'or', 'mi', 'su', 'sg', 'ca', 'tw', 'ow', 'st', 'ex') {
            $src | Should -Match "'$class="
        }
        $src | Should -Match '\[string\]::IsNullOrWhiteSpace\(\$env:DOTFILES_LS_COLORS\)'
        $src | Should -Match '\$env:LS_COLORS\s+=\s+\$script:RosePineLsColors'
        $src | Should -Match '\$env:LS_COLORS\s+=\s+\$env:DOTFILES_LS_COLORS'
        $src.IndexOf('$script:RosePineLsColors') | Should -BeLessThan $src.IndexOf('if (Get-Command lsd')
    }

    It "owns LS_COLORS by default and honors DOTFILES_LS_COLORS override" {
        $pwshExe = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
        if (-not $pwshExe) {
            $pwshExe = (Get-Command pwsh.exe -ErrorAction SilentlyContinue).Source
        }
        $pwshExe | Should -Not -BeNullOrEmpty

        $profilePath = $script:Profile.Replace('"', '`"')
        $scriptBlock = @"
`$env:LS_COLORS = 'di=01;34:ex=01;32:fi=00'
`$env:DOTFILES_LS_COLORS = `$null
. "$profilePath"
if (`$env:LS_COLORS -notmatch 'di=38;2;246;193;119') {
    Write-Error "LS_COLORS did not use the Rose Pine directory color: `$env:LS_COLORS"
    exit 1
}
if (`$env:LS_COLORS -match 'di=01;34') {
    Write-Error "LS_COLORS inherited the ambient palette: `$env:LS_COLORS"
    exit 1
}
if (`$env:LS_COLORS -notmatch 'ow=38;2;246;193;119' -or `$env:LS_COLORS -notmatch 'tw=38;2;246;193;119' -or `$env:LS_COLORS -notmatch 'st=38;2;246;193;119') {
    Write-Error "LS_COLORS did not include special directory classes: `$env:LS_COLORS"
    exit 1
}
`$env:LS_COLORS = 'di=01;34:ex=01;32:fi=00'
`$env:DOTFILES_LS_COLORS = 'di=38;2;1;2;3:ex=38;2;4;5;6'
. "$profilePath"
if (`$env:LS_COLORS -ne `$env:DOTFILES_LS_COLORS) {
    Write-Error "DOTFILES_LS_COLORS was not honored: `$env:LS_COLORS"
    exit 1
}
exit 0
"@

        & $pwshExe -NoProfile -Command $scriptBlock
        $LASTEXITCODE | Should -Be 0
    }

    It "removes pre-existing lsd shortcut aliases so functions win command resolution" {
        $pwshExe = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
        if (-not $pwshExe) {
            $pwshExe = (Get-Command pwsh.exe -ErrorAction SilentlyContinue).Source
        }
        $pwshExe | Should -Not -BeNullOrEmpty

        $profilePath = $script:Profile.Replace('"', '`"')
        $scriptBlock = @"
function global:lsd { param([Parameter(ValueFromRemainingArguments = `$true)] [object[]] `$Rest) }
foreach (`$name in 'ls', 'l', 'la', 'lla', 'lt') {
    Set-Alias -Name `$name -Value Get-Location -Scope Global -Force
}
. "$profilePath"
`$failures = @()
foreach (`$name in 'ls', 'l', 'la', 'lla', 'lt') {
    `$command = Get-Command `$name -All | Select-Object -First 1
    if (`$command.CommandType -ne 'Function') {
        `$failures += "`$name resolved to `$(`$command.CommandType)"
    }
}
if (`$failures.Count -gt 0) {
    Write-Error (`$failures -join '; ')
    exit 1
}
exit 0
"@

        & $pwshExe -NoProfile -Command $scriptBlock
        $LASTEXITCODE | Should -Be 0
    }
}
