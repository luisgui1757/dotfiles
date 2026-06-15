# PowerShell profile -- cross-version (5.1 + 7+), Rose Pine styled.
#
# Cardinal rule: this profile MUST NOT emit anything to stdout when loaded
# by a non-interactive subprocess. Git credential helpers, Conan wrappers,
# CI scripts, and VS Codes TerminalShellIntegration all spawn powershell
# and inherit $PROFILE; any prompt/UX work here either wastes time or
# leaks output that the parent tool tries to parse as command output.

# ---- Fast bail-out for non-interactive hosts ---------------------------------
# Credential helpers and pipe-driven invocations dont get a real console host.
# When in doubt, do nothing -- the prompt UX has zero value in those cases.
$interactive = $true
try {
    if (-not [Environment]::UserInteractive) {
        $interactive = $false
    }
    if ($interactive -and $Host.Name -notin @('ConsoleHost', 'Visual Studio Code Host', 'Windows PowerShell ISE Host')) {
        $interactive = $false
    }
} catch {
    $interactive = $false
}
if (-not $interactive) { return }

$ErrorActionPreference = 'Continue'

# ---- Path constants (cross-platform: pwsh runs on Windows + macOS + Linux) ---
$script:CacheDir = if ($env:LOCALAPPDATA) {
    $env:LOCALAPPDATA
} elseif ($env:XDG_CACHE_HOME) {
    $env:XDG_CACHE_HOME
} elseif ($env:HOME) {
    Join-Path $env:HOME '.cache'
} else {
    [System.IO.Path]::GetTempPath()
}
if (-not (Test-Path -LiteralPath $script:CacheDir)) {
    try { New-Item -ItemType Directory -Force -Path $script:CacheDir | Out-Null } catch { Write-Verbose $_.Exception.Message }
}
$script:StarshipInitPath = Join-Path $script:CacheDir 'starship.ps1'

# Windows PowerShell 5.1 Join-Path takes only ONE child path. PS 6+ allows
# additional child paths via -AdditionalChildPath. Use nested Join-Path so
# the same source works on both.
$script:HomeDir = if ($env:USERPROFILE) { $env:USERPROFILE } elseif ($env:HOME) { $env:HOME } else { '~' }
$script:StarshipConfigPath = if ($env:STARSHIP_CONFIG) {
    $env:STARSHIP_CONFIG
} else {
    Join-Path (Join-Path $script:HomeDir '.config') 'starship.toml'
}

# ---- Precompile Starship init (idempotent; regenerates when toml is newer) ---
function Test-StarshipInitRegenerationNeeded {
    [CmdletBinding()]
    param(
        [string]$InitPath = $script:StarshipInitPath,
        [string]$ConfigPath = $script:StarshipConfigPath
    )

    if (-not (Test-Path -LiteralPath $InitPath)) {
        return $true
    }
    if (Test-Path -LiteralPath $ConfigPath) {
        $initTime = (Get-Item -LiteralPath $InitPath).LastWriteTime
        $configTime = (Get-Item -LiteralPath $ConfigPath).LastWriteTime
        if ($configTime -gt $initTime) { return $true }
    }
    return $false
}

function Publish-StarshipInitScript {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string]$InitPath,
        [Parameter(Mandatory)] [string]$Content
    )

    $initDir = Split-Path -Parent $InitPath
    if (-not (Test-Path -LiteralPath $initDir -PathType Container)) {
        New-Item -ItemType Directory -Force -Path $initDir | Out-Null
    }

    $initLeaf = Split-Path -Leaf $InitPath
    $tempPath = Join-Path $initDir ("{0}.{1}.{2}.tmp" -f $initLeaf, $PID, [guid]::NewGuid().ToString('N'))
    try {
        [System.IO.File]::WriteAllText($tempPath, $Content, [System.Text.UTF8Encoding]::new($false))
        Move-Item -LiteralPath $tempPath -Destination $InitPath -Force -ErrorAction Stop
        return $true
    } catch {
        Write-Verbose $_.Exception.Message
        Remove-Item -LiteralPath $tempPath -Force -ErrorAction SilentlyContinue
        if (Test-Path -LiteralPath $InitPath -PathType Leaf) {
            return $false
        }
        throw
    }
}

function Confirm-StarshipInitScript {
    [CmdletBinding()]
    param(
        [string]$InitPath = $script:StarshipInitPath,
        [string]$ConfigPath = $script:StarshipConfigPath
    )

    if (Test-StarshipInitRegenerationNeeded -InitPath $InitPath -ConfigPath $ConfigPath) {
        Write-Verbose 'Generating precompiled Starship init script...'
        $init = (& starship init powershell --print-full-init) -join [Environment]::NewLine
        $published = Publish-StarshipInitScript -InitPath $InitPath -Content $init
        if (-not $published -and -not (Test-Path -LiteralPath $InitPath -PathType Leaf)) {
            throw "Starship init cache could not be published"
        }
    }
}

function Import-StarshipInitScriptWithRetry {
    [CmdletBinding()]
    param(
        [string]$InitPath = $script:StarshipInitPath,
        [int]$Attempts = 4,
        [int]$DelayMilliseconds = 50
    )

    $lastError = $null
    for ($attempt = 1; $attempt -le $Attempts; $attempt += 1) {
        try {
            . $InitPath
            return
        } catch {
            $lastError = $_
            if ($attempt -lt $Attempts) {
                Start-Sleep -Milliseconds $DelayMilliseconds
            }
        }
    }
    if ($lastError) {
        throw $lastError
    }
}

if (Get-Command starship -ErrorAction SilentlyContinue) {
    try {
        Confirm-StarshipInitScript
        Import-StarshipInitScriptWithRetry
    } catch {
        # Cached starship init may be stale or corrupt from an interrupted
        # write. Nuke it and rebuild once; if it still fails, give up silently
        # rather than spam the prompt on every shell launch.
        Write-Warning ("Starship init failed: " + $_.Exception.Message + ". Regenerating.")
        Remove-Item -LiteralPath $script:StarshipInitPath -Force -ErrorAction SilentlyContinue
        try {
            Confirm-StarshipInitScript
            Import-StarshipInitScriptWithRetry
        } catch {
            Write-Warning ("Starship init still failing: " + $_.Exception.Message)
        }
    }
}

# ---- PSReadLine (history prediction + Rose Pine colors + menu complete) ------
if (Get-Module -ListAvailable PSReadLine) {
    Import-Module PSReadLine -ErrorAction SilentlyContinue

    # Options that work on every PSReadLine version since 2.0:
    try { Set-PSReadLineOption -EditMode Windows -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    try { Set-PSReadLineOption -BellStyle None -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    try { Set-PSReadLineOption -HistoryNoDuplicates -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    try { Set-PSReadLineOption -HistorySearchCursorMovesToEnd -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }

    # PredictionSource + PredictionViewStyle landed in PSReadLine 2.1 / 2.2.
    # Older PS 5.1 installs may ship PSReadLine 2.0 which rejects these args.
    $psrl = Get-Module PSReadLine
    if ($psrl -and $psrl.Version -ge [Version]'2.2.0') {
        try {
            Set-PSReadLineOption -PredictionSource HistoryAndPlugin -ErrorAction Stop
            Set-PSReadLineOption -PredictionViewStyle ListView -ErrorAction Stop
        } catch { Write-Verbose $_.Exception.Message }
    } elseif ($psrl -and $psrl.Version -ge [Version]'2.1.0') {
        try { Set-PSReadLineOption -PredictionSource History -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    }

    $script:RosePineSelectionColor = "$([char]0x1b)[38;2;246;193;119m"

    try {
        Set-PSReadLineOption -Colors @{
            Command            = '#c4a7e7'
            Parameter          = '#9ccfd8'
            String             = '#f6c177'
            Operator           = '#ebbcba'
            Variable           = '#e0def4'
            Number             = '#eb6f92'
            Type               = '#9ccfd8'
            Comment            = '#6e6a86'
            Keyword            = '#c4a7e7'
            Error              = '#eb6f92'
            ContinuationPrompt = '#6e6a86'
            Default            = '#e0def4'
        }
    } catch { Write-Verbose $_.Exception.Message }

    # MenuComplete highlights the selected completion option with Selection -- the
    # owner wants that option GOLD, so this is a gold FOREGROUND SGR. Caveat that
    # cannot be separated: PSReadLine uses this SAME Selection color for the
    # completion suffix it inserts into the command line while you navigate the
    # menu (the ".exe" of lazygit.exe), so that suffix also shows gold until you
    # accept. It is one knob. Kept separate so invalid SGR support cannot drop the
    # syntax color table above.
    try { Set-PSReadLineOption -Colors @{ Selection = $script:RosePineSelectionColor } -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }

    # Prediction colors. The ListView prediction (the inline + dropdown
    # suggestions, our "fzf-like" history UI) defaults to a near-background grey
    # that is invisible on Rose Pine, so paint it explicitly. These keys landed
    # in separate PSReadLine versions -- InlinePrediction in 2.1.0, ListPrediction
    # + ListPredictionSelected in 2.2.0 -- and an unknown color key throws and
    # would drop the WHOLE -Colors hashtable. So they are applied here, version-
    # gated and isolated from the syntax colors above. ListPredictionTooltip is
    # left at its default.
    if ($psrl -and $psrl.Version -ge [Version]'2.1.0') {
        try { Set-PSReadLineOption -Colors @{ InlinePrediction = '#908caa' } -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    }
    if ($psrl -and $psrl.Version -ge [Version]'2.2.0') {
        try {
            Set-PSReadLineOption -Colors @{
                ListPrediction         = '#ebbcba'
                ListPredictionSelected = '#f6c177'
            } -ErrorAction Stop
        } catch { Write-Verbose $_.Exception.Message }
    }

    try { Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    try { Set-PSReadLineKeyHandler -Key UpArrow -Function HistorySearchBackward -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    try { Set-PSReadLineKeyHandler -Key DownArrow -Function HistorySearchForward -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }

    # psmux residual race-fix (issue #150 / v3.3.4): psmux resets
    # PredictionSource to None during pane init, AFTER our profile sets
    # HistoryAndPlugin. Even with allow-predictions on in tmux.windows.conf the
    # race can still bite -- confirmed via Get-PSReadLineOption showing
    # None/InlineView in a fresh pane. Re-apply our settings on the first idle
    # (PowerShell.OnIdle fires ~300 ms after the prompt is ready, by which time
    # psmux has finished its pane init). One-shot, gated on TMUX so it only
    # fires inside the multiplexer; outside, the up-front settings already win.
    if ($env:TMUX) {
        $null = Register-EngineEvent -SourceIdentifier PowerShell.OnIdle -MaxTriggerCount 1 -Action {
            $psrl = Get-Module PSReadLine
            if ($psrl -and $psrl.Version -ge [Version]'2.2.0') {
                Set-PSReadLineOption -PredictionSource HistoryAndPlugin -ErrorAction SilentlyContinue
                Set-PSReadLineOption -PredictionViewStyle ListView      -ErrorAction SilentlyContinue
            } elseif ($psrl -and $psrl.Version -ge [Version]'2.1.0') {
                Set-PSReadLineOption -PredictionSource History          -ErrorAction SilentlyContinue
            }
            # ShowToolTips landed in PSReadLine 2.3.4; only set it when present.
            $sop = Get-Command Set-PSReadLineOption -ErrorAction SilentlyContinue
            if ($sop -and $sop.Parameters.ContainsKey('ShowToolTips')) {
                Set-PSReadLineOption -ShowToolTips -ErrorAction SilentlyContinue
            }
            Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete -ErrorAction SilentlyContinue
        }
    }
}

# ---- PSFzf: fuzzy history / file / directory pickers -------------------------
# Unifies the shell fuzzy UX on fzf (the same tool the zsh side uses), wired
# only when BOTH the PSFzf module and the fzf binary are present so a machine
# without fzf keeps a working profile. Ctrl+R intentionally OVERRIDES the
# PSReadLine reverse-history search with the fzf fuzzy picker (POSIX parity --
# the zsh side binds the same chord). Ctrl+T = fuzzy file insert, Alt+C = fuzzy
# cd. install-deps installs fzf + PSFzf.
if ((Get-Module -ListAvailable PSFzf) -and (Get-Command fzf -ErrorAction SilentlyContinue)) {
    try {
        Import-Module PSFzf -ErrorAction Stop
        Set-PsFzfOption -PSReadlineChordProvider 'Ctrl+t' `
            -PSReadlineChordReverseHistory 'Ctrl+r' `
            -PSReadlineChordSetLocation 'Alt+c' -ErrorAction Stop
    } catch { Write-Verbose $_.Exception.Message }
}

# ---- Directory listing color -------------------------------------------------
# PowerShell 7.2+ colorizes Get-ChildItem/ls via $PSStyle. The default directory
# color (bright blue) is unreadable on the Rose Pine dark background, so paint
# directories gold. $PSStyle is absent on Windows PowerShell 5.1 and pwsh < 7.2,
# hence the guard; FromRgb keeps the source free of raw ANSI escape bytes.
if ($PSStyle) {
    try { $PSStyle.FileInfo.Directory = $PSStyle.Foreground.FromRgb(0xf6c177) } catch { Write-Verbose $_.Exception.Message }
}
