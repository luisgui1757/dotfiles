# ---- PSReadLine (history prediction + Rose Pine colors + menu complete) ------
if (Get-Module -ListAvailable PSReadLine) {
    Import-Module PSReadLine -ErrorAction SilentlyContinue

    # EditMode Vi enables command-line vi keybindings (Esc -> normal mode). It is
    # applied HERE, before any Set-PSReadLineKeyHandler call below, because
    # changing EditMode RESETS all key handlers to the defaults for that mode --
    # handlers must be (re)applied AFTER this, never before. EditMode works on
    # every PSReadLine version.
    try { Set-PSReadLineOption -EditMode Vi -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }

    # Cursor feedback for the current vi mode (block in Command, beam in Insert).
    # ViModeIndicator landed in PSReadLine 2.0; older PS 5.1 (PSReadLine 1.x) lacks
    # the parameter, so gate on it. Cursor mode is terminal-native (Windows
    # Terminal honours DECSCUSR) and needs no ViModeChangeHandler script.
    $splo = Get-Command Set-PSReadLineOption -ErrorAction SilentlyContinue
    if ($splo -and $splo.Parameters.ContainsKey('ViModeIndicator')) {
        try { Set-PSReadLineOption -ViModeIndicator Cursor -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    }

    # Options that work on every PSReadLine version since 2.0:
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

    # Vi-mode key handlers, applied AFTER EditMode above. Tab=MenuComplete and
    # Up/Down history search must land on the vi INSERT keymap (where you type),
    # so they are bound with -ViMode Insert; Up/Down are ALSO bound in Command
    # mode so the arrows still search history in normal mode. -ViMode is gated on
    # the parameter existing because older PSReadLine (PS 5.1) may lack it; there
    # we bind the keys unscoped so the behaviour still survives.
    $skh = Get-Command Set-PSReadLineKeyHandler -ErrorAction SilentlyContinue
    if ($skh -and $skh.Parameters.ContainsKey('ViMode')) {
        try {
            Set-PSReadLineKeyHandler -Key Tab       -Function MenuComplete          -ViMode Insert  -ErrorAction Stop
            Set-PSReadLineKeyHandler -Key UpArrow   -Function HistorySearchBackward  -ViMode Insert  -ErrorAction Stop
            Set-PSReadLineKeyHandler -Key UpArrow   -Function HistorySearchBackward  -ViMode Command -ErrorAction Stop
            Set-PSReadLineKeyHandler -Key DownArrow -Function HistorySearchForward   -ViMode Insert  -ErrorAction Stop
            Set-PSReadLineKeyHandler -Key DownArrow -Function HistorySearchForward   -ViMode Command -ErrorAction Stop
        } catch { Write-Verbose $_.Exception.Message }
    } else {
        try { Set-PSReadLineKeyHandler -Key Tab       -Function MenuComplete         -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
        try { Set-PSReadLineKeyHandler -Key UpArrow   -Function HistorySearchBackward -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
        try { Set-PSReadLineKeyHandler -Key DownArrow -Function HistorySearchForward  -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
    }


}

# ---- Directory listing color -------------------------------------------------
# PowerShell 7.2+ colorizes Get-ChildItem/ls via $PSStyle. The default directory
# color (bright blue) is unreadable on the Rose Pine dark background, so paint
# directories gold. $PSStyle is absent on Windows PowerShell 5.1 and pwsh < 7.2,
# hence the guard; FromRgb keeps the source free of raw ANSI escape bytes.
if ($PSStyle) {
    try { $PSStyle.FileInfo.Directory = $PSStyle.Foreground.FromRgb(0xf6c177) } catch { Write-Verbose $_.Exception.Message }
}
