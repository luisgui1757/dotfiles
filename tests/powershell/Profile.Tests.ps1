BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Profile  = Join-Path $script:RepoRoot "shells/powershell_profile.ps1"
    $script:InteractiveProfileContext = @'
$DotfilesProfileInvocationContext = @{
    UserInteractive = $true
    HostName = 'ConsoleHost'
    CommandLineArgs = @('pwsh', '-NoExit')
    InputRedirected = $false
    OutputRedirected = $false
    ErrorRedirected = $false
    CIValue = ''
}
'@
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

    It "uses invocation, redirection, CI, and host checks before profile work" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $guardIndex = $src.IndexOf('function Test-DotfilesInteractiveInvocation')
        $cacheIndex = $src.IndexOf('$script:CacheDir')
        $guardIndex | Should -BeGreaterOrEqual 0
        $cacheIndex | Should -BeGreaterThan $guardIndex
        $src | Should -Match '\[Environment\]::GetCommandLineArgs\(\)'
        $src | Should -Match '\[Console\]::IsInputRedirected'
        $src | Should -Match '\[Console\]::IsOutputRedirected'
        $src | Should -Match '\[Console\]::IsErrorRedirected'
        $src | Should -Match '\$env:CI'
    }

    It "classifies supported interactive hosts without accepting batch invocations" {
        . $script:Profile
        foreach ($hostName in 'ConsoleHost', 'Visual Studio Code Host', 'Windows PowerShell ISE Host') {
            Test-DotfilesInteractiveInvocation -UserInteractive $true -HostName $hostName `
                -CommandLineArgs @('pwsh') -InputRedirected $false -OutputRedirected $false `
                -ErrorRedirected $false -CIValue '' | Should -BeTrue
        }
        Test-DotfilesInteractiveInvocation -UserInteractive $true -HostName 'ConsoleHost' `
            -CommandLineArgs @('pwsh', '-NoExit', '-Command', 'prompt') -InputRedirected $false `
            -OutputRedirected $false -ErrorRedirected $false -CIValue '' | Should -BeTrue
        Test-DotfilesInteractiveInvocation -UserInteractive $true -HostName 'ConsoleHost' `
            -CommandLineArgs @('pwsh', '-Command', 'git credential fill') -InputRedirected $false `
            -OutputRedirected $false -ErrorRedirected $false -CIValue '' | Should -BeFalse
    }

    It "rejects noninteractive, redirected, CI, and unsupported invocation contexts" {
        . $script:Profile
        $base = @{
            UserInteractive = $true
            HostName = 'ConsoleHost'
            CommandLineArgs = @('pwsh')
            InputRedirected = $false
            OutputRedirected = $false
            ErrorRedirected = $false
            CIValue = ''
        }
        foreach ($override in @(
                @{ CommandLineArgs = @('pwsh', '-NonInteractive') },
                @{ InputRedirected = $true },
                @{ OutputRedirected = $true },
                @{ ErrorRedirected = $true },
                @{ CIValue = 'true' },
                @{ HostName = 'ServerRemoteHost' }
            )) {
            $context = $base.Clone()
            foreach ($key in $override.Keys) { $context[$key] = $override[$key] }
            Test-DotfilesInteractiveInvocation @context | Should -BeFalse
        }
    }

    It "does no output or profile-owned cache work in real noninteractive subprocesses" -TestCases @(
        @{ Name = 'noninteractive'; Arguments = @('-NonInteractive', '-Command'); CIValue = '' },
        @{ Name = 'credential-command'; Arguments = @('-Command'); CIValue = '' },
        @{ Name = 'ci-command'; Arguments = @('-Command'); CIValue = 'true' }
    ) {
        param([string]$Name, [string[]]$Arguments, [string]$CIValue)
        $pwshExe = (Get-Command pwsh -ErrorAction Stop).Source
        $sandbox = Join-Path ([IO.Path]::GetTempPath()) ("profile guard $Name " + [Guid]::NewGuid())
        $cache = Join-Path $sandbox 'cache must stay absent'
        New-Item -ItemType Directory -Force -Path $sandbox | Out-Null
        try {
            $startInfo = [Diagnostics.ProcessStartInfo]::new()
            $startInfo.FileName = $pwshExe
            $startInfo.UseShellExecute = $false
            $startInfo.RedirectStandardInput = $true
            $startInfo.RedirectStandardOutput = $true
            $startInfo.RedirectStandardError = $true
            $startInfo.ArgumentList.Add('-NoLogo')
            $startInfo.ArgumentList.Add('-NoProfile')
            foreach ($argument in $Arguments) { $startInfo.ArgumentList.Add($argument) }
            $startInfo.ArgumentList.Add(". '$($script:Profile.Replace("'", "''"))'")
            $startInfo.Environment['LOCALAPPDATA'] = $cache
            $startInfo.Environment['XDG_CACHE_HOME'] = $cache
            $startInfo.Environment['CI'] = $CIValue
            $process = [Diagnostics.Process]::Start($startInfo)
            $process.StandardInput.Close()
            $stdout = $process.StandardOutput.ReadToEnd()
            $stderr = $process.StandardError.ReadToEnd()
            $process.WaitForExit()

            $process.ExitCode | Should -Be 0
            $stdout | Should -BeNullOrEmpty
            $stderr | Should -BeNullOrEmpty
            Test-Path -LiteralPath (Join-Path $cache 'starship.ps1') | Should -BeFalse
            Test-Path -LiteralPath (Join-Path $cache 'zoxide.ps1') | Should -BeFalse
            Test-Path -LiteralPath (Join-Path $cache 'zoxide.ps1.version') | Should -BeFalse
        } finally {
            Remove-Item -LiteralPath $sandbox -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "writes the starship init cache with UTF8 no-BOM encoding" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'function Publish-StarshipInitScript'
        $src | Should -Match '\[System\.Text\.UTF8Encoding\]::new\(\$false\)'
    }

    It "publishes the starship init cache atomically from a temp file" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match '"\{0\}\.\{1\}\.\{2\}\.tmp"\s+-f\s+\$initLeaf,\s+\$PID'
        $src | Should -Match '\[IO\.File\]::Replace\(\$tempPath,\s*\$InitPath,\s*\$rollbackPath,\s*\$true\)'
        $src | Should -Match 'Test-StarshipInitScriptValid\s+-Path\s+\$InitPath'
        $src | Should -Match 'function Import-StarshipInitScriptWithRetry'
        $src | Should -Match 'Start-Sleep\s+-Milliseconds\s+\$DelayMilliseconds'
    }

    It "repairs an invalid Starship cache and preserves old bytes on generation failure" {
        $pwshExe = (Get-Command pwsh -ErrorAction Stop).Source
        $profilePath = $script:Profile.Replace('"', '`"')
        $sandbox = Join-Path ([IO.Path]::GetTempPath()) ('starship cache ' + [Guid]::NewGuid())
        $scriptBlock = @"
$($script:InteractiveProfileContext)
`$env:PATH = ''
. "$profilePath"
`$root = '$($sandbox.Replace("'", "''"))'
`$cache = Join-Path `$root 'starship.ps1'
`$config = Join-Path `$root 'starship.toml'
New-Item -ItemType Directory -Force -Path `$root | Out-Null
[IO.File]::WriteAllText(`$cache, 'function broken {', [Text.UTF8Encoding]::new(`$false))
Confirm-StarshipInitScript -InitPath `$cache -ConfigPath `$config -Generator {
    `$global:LASTEXITCODE = 0
    'function global:prompt { return "repaired" }'
}
if (-not (Test-StarshipInitScriptValid -Path `$cache)) { exit 10 }
if ([IO.File]::ReadAllText(`$cache) -notmatch 'repaired') { exit 11 }

`$preserved = [IO.File]::ReadAllBytes(`$cache)
[IO.File]::WriteAllText(`$config, 'newer', [Text.UTF8Encoding]::new(`$false))
(Get-Item -LiteralPath `$config).LastWriteTimeUtc = [DateTime]::UtcNow.AddMinutes(1)
try {
    Confirm-StarshipInitScript -InitPath `$cache -ConfigPath `$config -Generator {
        `$global:LASTEXITCODE = 9
        'function global:prompt { return "must-not-publish" }'
    }
    exit 12
} catch {
    if (`$_.Exception.Message -notmatch 'exited 9') { exit 13 }
}
`$after = [IO.File]::ReadAllBytes(`$cache)
if (`$after.Length -ne `$preserved.Length) { exit 14 }
for (`$i = 0; `$i -lt `$after.Length; `$i++) {
    if (`$after[`$i] -ne `$preserved[`$i]) { exit 15 }
}

[IO.File]::WriteAllText(`$cache, 'function broken {', [Text.UTF8Encoding]::new(`$false))
try {
    Confirm-StarshipInitScript -InitPath `$cache -ConfigPath `$config -Generator {
        `$global:LASTEXITCODE = 0
        '   '
    }
    exit 16
} catch {
    if (`$_.Exception.Message -notmatch 'empty or invalid') { exit 17 }
}
if ([IO.File]::ReadAllText(`$cache) -ne 'function broken {') { exit 18 }
exit 0
"@
        try {
            & $pwshExe -NoLogo -NoProfile -Command $scriptBlock
            $LASTEXITCODE | Should -Be 0
        } finally {
            Remove-Item -LiteralPath $sandbox -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "caches the zoxide init without remote eval (no Invoke-Expression / iex)" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Not -Match 'Invoke-Expression'
        $src | Should -Not -Match '(^|[^A-Za-z0-9_])iex([^A-Za-z0-9_]|$)'
        $src | Should -Match 'function Confirm-ZoxideInitScript'
        $src | Should -Match 'function Import-ZoxideInitScriptWithRetry'
    }

    It "publishes the zoxide init cache atomically (UTF8 no-BOM temp + Move-Item)" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'function Publish-ZoxideInitScript'
        $src | Should -Match '"\{0\}\.\{1\}\.\{2\}\.tmp"\s+-f\s+\$initLeaf,\s+\$PID'
        $src | Should -Match '\[System\.Text\.UTF8Encoding\]::new\(\$false\)'
        $src | Should -Match 'Move-Item\s+-LiteralPath\s+\$tempPath\s+-Destination\s+\$InitPath\s+-Force'
    }

    It "regenerates the zoxide cache only when missing or the version changed" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'function Test-ZoxideInitRegenerationNeeded'
        $src | Should -Match 'zoxide --version'
        $src | Should -Match '\$script:ZoxideVersionPath'
    }

    It "wires zoxide only when the binary exists, after starship" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'Get-Command\s+zoxide\s+-ErrorAction\s+SilentlyContinue'
        $starshipIdx = $src.IndexOf('Get-Command starship')
        $zoxideIdx = $src.IndexOf('Get-Command zoxide')
        $starshipIdx | Should -BeGreaterOrEqual 0
        $zoxideIdx | Should -BeGreaterThan $starshipIdx
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
$($script:InteractiveProfileContext)
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
$($script:InteractiveProfileContext)
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
$($script:InteractiveProfileContext)
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

    It "sets EditMode Vi before any key handler (changing EditMode resets handlers)" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'Set-PSReadLineOption -EditMode Vi'
        $src | Should -Not -Match 'Set-PSReadLineOption -EditMode Windows'
        # Anchor on the real cmdlet calls, not the surrounding comment prose.
        $editModeIdx = $src.IndexOf('Set-PSReadLineOption -EditMode Vi')
        $firstHandlerIdx = $src.IndexOf('Set-PSReadLineKeyHandler -Key Tab')
        $editModeIdx | Should -BeGreaterOrEqual 0
        $firstHandlerIdx | Should -BeGreaterThan $editModeIdx
    }

    It "binds Tab to MenuComplete on the vi Insert keymap" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'Set-PSReadLineKeyHandler\s+-Key Tab\s+-Function MenuComplete\s+-ViMode Insert'
    }

    It "binds Up/Down history search in both vi Insert and Command modes" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'Set-PSReadLineKeyHandler\s+-Key UpArrow\s+-Function HistorySearchBackward\s+-ViMode Insert'
        $src | Should -Match 'Set-PSReadLineKeyHandler\s+-Key UpArrow\s+-Function HistorySearchBackward\s+-ViMode Command'
        $src | Should -Match 'Set-PSReadLineKeyHandler\s+-Key DownArrow\s+-Function HistorySearchForward\s+-ViMode Insert'
        $src | Should -Match 'Set-PSReadLineKeyHandler\s+-Key DownArrow\s+-Function HistorySearchForward\s+-ViMode Command'
    }

    It "version-gates -ViMode / -ViModeIndicator with a PS 5.1 fallback" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match "Parameters\.ContainsKey\('ViMode'\)"
        $src | Should -Match "Parameters\.ContainsKey\('ViModeIndicator'\)"
        $src | Should -Match 'Set-PSReadLineOption -ViModeIndicator Cursor'
        # The -ViMode gate keeps an unscoped Tab binding for older PSReadLine.
        $src | Should -Match 'Set-PSReadLineKeyHandler -Key Tab\s+-Function MenuComplete\s+-ErrorAction'
    }

    It "reapplies the vi-mode key handlers from the psmux OnIdle block" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $onIdleIdx = $src.IndexOf('PowerShell.OnIdle')
        $onIdleIdx | Should -BeGreaterThan 0
        $tail = $src.Substring($onIdleIdx)
        $tail | Should -Match 'Set-PSReadLineKeyHandler\s+-Key Tab\s+-Function MenuComplete\s+-ViMode Insert'
        $tail | Should -Match 'Set-PSReadLineKeyHandler\s+-Key UpArrow\s+-Function HistorySearchBackward\s+-ViMode Insert'
    }

    It "does not reset EditMode from the psmux OnIdle block" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $onIdleIdx = $src.IndexOf('PowerShell.OnIdle')
        $psfzfIdx = $src.IndexOf('Set-PsFzfOption')
        $onIdleIdx | Should -BeGreaterThan 0
        $psfzfIdx | Should -BeGreaterThan $onIdleIdx
        $onIdleBlock = $src.Substring($onIdleIdx, $psfzfIdx - $onIdleIdx)
        $onIdleBlock | Should -Not -Match 'Set-PSReadLineOption\s+-EditMode'
    }

    It "keeps the PSFzf chords and wires them after EditMode" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'PSReadlineChordProvider'
        $src | Should -Match 'PSReadlineChordReverseHistory'
        $src | Should -Match 'PSReadlineChordSetLocation'
        $editModeIdx = $src.IndexOf('-EditMode Vi')
        $psfzfIdx = $src.IndexOf('Set-PsFzfOption')
        $editModeIdx | Should -BeGreaterOrEqual 0
        $psfzfIdx | Should -BeGreaterThan $editModeIdx
    }

    It "keeps the Rose Pine prediction + colors after enabling vi mode" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'PredictionSource HistoryAndPlugin'
        $src | Should -Match 'PredictionViewStyle\s+ListView'
        $src | Should -Match 'RosePineSelectionColor'
    }

    It "applies EditMode Vi, cursor indicator, and MenuComplete Tab when dot-sourced" {
        $pwshExe = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
        if (-not $pwshExe) {
            $pwshExe = (Get-Command pwsh.exe -ErrorAction SilentlyContinue).Source
        }
        $pwshExe | Should -Not -BeNullOrEmpty

        $profilePath = $script:Profile.Replace('"', '`"')
        $scriptBlock = @"
$($script:InteractiveProfileContext)
. "$profilePath"
`$o = Get-PSReadLineOption
if (`$o.EditMode -ne 'Vi') { Write-Error "EditMode=`$(`$o.EditMode)"; exit 1 }
`$tab = Get-PSReadLineKeyHandler -Bound | Where-Object { `$_.Key -eq 'Tab' }
if (-not (`$tab | Where-Object { `$_.Function -eq 'MenuComplete' })) { Write-Error 'Tab not MenuComplete'; exit 2 }
`$up = Get-PSReadLineKeyHandler -Bound | Where-Object { `$_.Key -eq 'UpArrow' }
if (-not (`$up | Where-Object { `$_.Function -eq 'HistorySearchBackward' })) { Write-Error 'Up not HistorySearchBackward'; exit 3 }
exit 0
"@

        & $pwshExe -NoProfile -Command $scriptBlock
        $LASTEXITCODE | Should -Be 0
    }
}
