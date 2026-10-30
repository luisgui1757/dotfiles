BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Fixture = Join-Path ([IO.Path]::GetTempPath()) ('active shell ' + [char]0xfc + ' ' + [Guid]::NewGuid())
    [void][IO.Directory]::CreateDirectory($script:Fixture)
    $previousExport = $env:DOTFILES_WRITE_SHELL_FIXTURES
    try {
        $env:DOTFILES_WRITE_SHELL_FIXTURES = $script:Fixture
        & go -C (Join-Path $script:RepoRoot 'installer') test -count=1 -run '^TestWriteActiveShellFixtures$' .
        if ($LASTEXITCODE -ne 0) { throw 'Active shell generation failed' }
    } finally {
        $env:DOTFILES_WRITE_SHELL_FIXTURES = $previousExport
    }
    $script:Profile = Join-Path $script:Fixture 'windows/state/shells/init.ps1'
    $script:CacheDirectory = Join-Path $script:Fixture 'windows/state/shell-cache'

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

AfterAll {
    if ($script:Fixture) { Remove-Item -LiteralPath $script:Fixture -Recurse -Force }
}

Describe "PowerShell profile" {

    It "matches the exact reviewed generated-profile analyzer findings" {
        $diags = Invoke-ScriptAnalyzer -Path $script:Profile -Severity Warning,Error
        if ($diags) { $diags | Format-Table | Out-String | Write-Host }
        # This process-wide sentinel prevents duplicate initialization across
        # dot-sourced host profiles. It is the only permitted warning identity.
        $diags.Count | Should -Be 2
        foreach ($finding in $diags) {
            $finding.RuleName | Should -BeExactly 'PSAvoidGlobalVars'
            $finding.Extent.Text | Should -BeExactly '$global:DotfilesShellReady'
        }
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

    It "uses an approved verb for the shared initialization cache" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Match 'function Get-DotfilesInitScript'
        $src | Should -Not -Match 'function Ensure-DotfilesInitScript'
    }

    It "uses invocation, redirection, CI, and host checks before profile work" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $guardIndex = $src.IndexOf('function Test-DotfilesInteractiveInvocation')
        $cacheIndex = $src.IndexOf('function Get-DotfilesInitScript')
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
            Test-Path -LiteralPath $script:CacheDirectory | Should -BeFalse
        } finally {
            Remove-Item -LiteralPath $sandbox -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    It "uses parsed local UTF8 caches with exclusive atomic publication" {
        $src = Get-Content -Raw -LiteralPath $script:Profile
        $src | Should -Not -Match 'Invoke-Expression|(^|[^A-Za-z0-9_])iex([^A-Za-z0-9_]|$)'
        $src | Should -Match '\[Text\.UTF8Encoding\]::new\(\$false\)'
        $src | Should -Match '\[IO\.File\]::Move\(\$temporary, \$CachePath\)'
        $src.IndexOf('tool.starship/current/bin/starship.ps1'.Replace('/', [IO.Path]::DirectorySeparatorChar)) | Should -BeLessThan $src.LastIndexOf('tool.zoxide/current/bin/zoxide.ps1'.Replace('/', [IO.Path]::DirectorySeparatorChar))
    }

    It "reuses valid caches and preserves invalid or failed-generation evidence" {
        $sandbox = Join-Path $TestDrive ('cache contract ' + [char]0xfc)
        [void][IO.Directory]::CreateDirectory($sandbox)
        $child = @"
$($script:InteractiveProfileContext)
. '$($script:Profile.Replace("'", "''"))'
`$cache = '$((Join-Path $sandbox 'init.ps1').Replace("'", "''"))'
`$generator = '$((Join-Path $sandbox 'generator.ps1').Replace("'", "''"))'
[IO.File]::WriteAllText(`$generator, '`$global:LASTEXITCODE=0; ''function global:FixtureInit { }''')
`$path = Get-DotfilesInitScript -Command `$generator -Arguments @() -CachePath `$cache
if (`$path -ne `$cache) { throw 'Wrong initialization cache' }
`$bytes = [IO.File]::ReadAllBytes(`$cache)
if (`$bytes.Length -ge 3 -and `$bytes[0] -eq 239 -and `$bytes[1] -eq 187 -and `$bytes[2] -eq 191) { throw 'Cache has a BOM' }
[IO.File]::WriteAllText(`$generator, 'throw "Must not regenerate a valid cache"')
`$null = Get-DotfilesInitScript -Command `$generator -Arguments @() -CachePath `$cache
[IO.File]::WriteAllText(`$cache, 'function broken {')
try { `$null = Get-DotfilesInitScript -Command `$generator -Arguments @() -CachePath `$cache; throw 'Accepted invalid cache' } catch {
    if (`$_.Exception.Message -notmatch 'Invalid initialization cache') { throw }
}
if ([IO.File]::ReadAllText(`$cache) -cne 'function broken {') { throw 'Changed invalid cache evidence' }
[IO.File]::Delete(`$cache)
foreach (`$body in @('`$global:LASTEXITCODE=9; ''function global:bad { }''', '`$global:LASTEXITCODE=0; ''function broken {''', '`$global:LASTEXITCODE=0; ''   ''')) {
    [IO.File]::WriteAllText(`$generator, `$body)
    try { `$null = Get-DotfilesInitScript -Command `$generator -Arguments @() -CachePath `$cache; throw 'Accepted bad generator' } catch {
        if (`$_.Exception.Message -notmatch 'Initialization generation failed') { throw }
    }
    if (Test-Path -LiteralPath `$cache) { throw 'Published failed generator' }
}
if (@(Get-ChildItem -LiteralPath (Split-Path -Parent `$cache) -Filter '*.tmp').Count) { throw 'Temporary cache leaked' }
"@
        & (Get-Command pwsh -ErrorAction Stop).Source -NoLogo -NoProfile -Command $child
        $LASTEXITCODE | Should -Be 0
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
`$global:DotfilesShellReady = `$false
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

    It "does not register a delayed multiplexer rewrite in the active profile" {
        Get-Content -Raw -LiteralPath $script:Profile | Should -Not -Match 'Register-EngineEvent|PowerShell.OnIdle'

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

    It "puts the Rose Pine fzf colors before a value the user set, one time only" {
        $scriptBlock = "`$env:FZF_DEFAULT_OPTS = '--height 50%'`n" + $script:InteractiveProfileContext + "`n. '" + $script:Profile.Replace("'", "''") + "'`n" + @'
$first = $env:FZF_DEFAULT_OPTS
if (-not $first.StartsWith('--color=fg:#908caa,') -or -not $first.EndsWith(' --height 50%')) { Write-Error "Order: $first"; exit 1 }
$global:DotfilesShellReady = $false
'@ + "`n. '" + $script:Profile.Replace("'", "''") + "'`n" + @'
if ($env:FZF_DEFAULT_OPTS -cne $first) { Write-Error "Second load changed: $env:FZF_DEFAULT_OPTS"; exit 2 }
exit 0
'@
        & (Get-Command pwsh -ErrorAction Stop).Source -NoLogo -NoProfile -Command $scriptBlock
        $LASTEXITCODE | Should -Be 0
    }

    It "starts Atuin after PSFzf without its Up Arrow or AI keys" {
        $scriptBlock = $script:InteractiveProfileContext + "`n. '" + $script:Profile.Replace("'", "''") + "'`n" + @'
$expected = 'init powershell --disable-up-arrow --disable-ai|Vi,Ctrl+t,Ctrl+r,Alt+c'
if ($global:DotfilesFixtureAtuin -cne $expected) { Write-Error "Atuin init: $global:DotfilesFixtureAtuin"; exit 1 }
exit 0
'@
        & (Get-Command pwsh -ErrorAction Stop).Source -NoLogo -NoProfile -Command $scriptBlock
        $LASTEXITCODE | Should -Be 0
    }

    It "changes to the folder superfile saved on Shift+Q and never runs the saved text" {
        $root = Join-Path ([IO.Path]::GetTempPath()) ("spf [root] it's " + [Guid]::NewGuid())
        [void][IO.Directory]::CreateDirectory((Join-Path $root "it's [a] folder"))
        [void][IO.Directory]::CreateDirectory((Join-Path $root 'start'))
        $previousRoot = $env:SPF_TEST_ROOT
        try {
            $env:SPF_TEST_ROOT = $root
            $scriptBlock = $script:InteractiveProfileContext + "`n. '" + $script:Profile.Replace("'", "''") + "'`n" + @'
$root = $env:SPF_TEST_ROOT
$target = Join-Path $root "it's [a] folder"
$start = Join-Path $root 'start'
$env:SPF_TEST_LAST = Join-Path $root 'lastdir'
# A function named spf.exe takes the place of the application.
function global:spf.exe {
    if ($args[0] -eq 'path-list') { return $env:SPF_TEST_LAST }
    if ($env:SPF_TEST_CONTENT) { [IO.File]::WriteAllText($env:SPF_TEST_LAST, $env:SPF_TEST_CONTENT) }
}
Set-Location -LiteralPath $start
$env:SPF_TEST_CONTENT = "cd '" + $target.Replace("'", "'\''") + "'"
spf
if ((Get-Location).ProviderPath -ne $target) { Write-Error "Shift+Q went to $((Get-Location).ProviderPath)"; exit 1 }
if (Test-Path -LiteralPath $env:SPF_TEST_LAST) { Write-Error 'The saved folder file stayed'; exit 2 }
Set-Location -LiteralPath $start
$env:SPF_TEST_CONTENT = "Set-Content -LiteralPath '" + (Join-Path $root 'pwned') + "' -Value x"
spf
if ((Get-Location).ProviderPath -ne $start) { Write-Error 'Changed folder for text that is not a cd line'; exit 3 }
if (Test-Path -LiteralPath (Join-Path $root 'pwned')) { Write-Error 'The saved text ran as code'; exit 4 }
if (Test-Path -LiteralPath $env:SPF_TEST_LAST) { Write-Error 'The rejected folder file stayed'; exit 5 }
exit 0
'@
            & (Get-Command pwsh -ErrorAction Stop).Source -NoLogo -NoProfile -Command $scriptBlock
            $LASTEXITCODE | Should -Be 0
        } finally {
            $env:SPF_TEST_ROOT = $previousRoot
            Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
        }
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
if ((`$global:DotfilesFixtureFzf -join '|') -cne 'Vi|Ctrl+t|Ctrl+r|Alt+c') { throw 'PSFzf did not initialize after actual PSReadLine vi mode' }
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
