# Maintainer-only generator for the tmux Rose Pine pill bar.
# Generate each variant with pwsh -File tmux/rose-pine.ps1 -Variant main|moon|dawn.
# Pure ASCII source; glyphs are built from codepoints. No live terminal changes.

param(
    [string]$Variant = 'main'
)

$ErrorActionPreference = 'Stop'

function Get-TmuxRosePinePalette {
    param([string]$Variant)

    switch ($Variant) {
        'moon' {
            return @{
                base = '#232136'; surface = '#2a273f'; overlay = '#393552'
                text = '#e0def4'; subtle = '#908caa'; muted = '#6e6a86'
                love = '#eb6f92'; gold = '#f6c177'; rose = '#ea9a97'; pine = '#3e8fb0'
                foam = '#9ccfd8'; iris = '#c4a7e7'; hlMed = '#44415a'; hlHigh = '#56526e'
            }
        }
        'dawn' {
            return @{
                base = '#faf4ed'; surface = '#fffaf3'; overlay = '#f2e9e1'
                text = '#575279'; subtle = '#797593'; muted = '#9893a5'
                love = '#b4637a'; gold = '#ea9d34'; rose = '#d7827e'; pine = '#286983'
                foam = '#56949f'; iris = '#907aa9'; hlMed = '#dfdad9'; hlHigh = '#cecacd'
            }
        }
        default {
            return @{
                base = '#191724'; surface = '#1f1d2e'; overlay = '#26233a'
                text = '#e0def4'; subtle = '#908caa'; muted = '#6e6a86'
                love = '#eb6f92'; gold = '#f6c177'; rose = '#ebbcba'; pine = '#31748f'
                foam = '#9ccfd8'; iris = '#c4a7e7'; hlMed = '#403d52'; hlHigh = '#524f67'
            }
        }
    }
}

function Get-TmuxRosePineCommand {
    param(
        [string]$Variant = 'main'
    )

    $p = Get-TmuxRosePinePalette -Variant $Variant

    # Nerd Font glyphs (codepoints -> runtime string keeps this .ps1 pure ASCII).
    #   capLeft/capRight : rounded pill caps (half circles), NOT arrow chevrons.
    #   iSession/iFolder : left session pill + right directory pill icons.
    #   iZoom            : zoom marker shown on the current window when zoomed.
    $capLeft = [char]::ConvertFromUtf32(0xE0B6)
    $capRight = [char]::ConvertFromUtf32(0xE0B4)
    $iSession = [char]::ConvertFromUtf32(0xEB7F)
    $iFolder = [char]::ConvertFromUtf32(0xF413)
    $iZoom = [char]::ConvertFromUtf32(0xF065)

    # Session accent turns love while the prefix is held; otherwise use foam
    # for the Omer-style cool accent without the iris/purple cast.
    $sessAccent = "#{?client_prefix,$($p.love),$($p.foam)}"
    $barBg = 'default'

    # status-left: rounded session pill. icon segment (accent bg) + name segment
    # (overlay bg). Trailing space separates it from the window list.
    $statusLeft = "#[fg=$sessAccent,bg=$barBg]$capLeft#[fg=$($p.base),bg=$sessAccent] $iSession #[fg=$($p.text),bg=$($p.overlay)] #S #[fg=$($p.overlay),bg=$barBg]$capRight "

    # window cells: name segment (overlay bg) + number segment on the RIGHT
    # (Catppuccin number_position=right, fill=number). Current window fills the
    # number in gold and appends a zoom marker; inactive fills it muted.
    $winFormat = "#[fg=$($p.overlay),bg=$barBg]$capLeft#[fg=$($p.subtle),bg=$($p.overlay)] #W #[fg=$($p.base),bg=$($p.muted)] #I #[fg=$($p.muted),bg=$barBg]$capRight"
    $winCurrentFormat = "#[fg=$($p.overlay),bg=$barBg]$capLeft#[fg=$($p.text),bg=$($p.overlay)] #W#{?window_zoomed_flag, $iZoom,} #[fg=$($p.base),bg=$($p.gold)] #I #[fg=$($p.gold),bg=$barBg]$capRight"

    # status-right: rounded directory pill (basename only). One terminal-edge
    # safety cell so the last visible glyph is not clipped by Windows Terminal.
    $statusRight = "#[fg=$($p.overlay),bg=$barBg]$capLeft#[fg=$($p.subtle),bg=$($p.overlay)] $iFolder #[fg=$($p.rose),bg=$($p.overlay)]#{b:pane_current_path} #[fg=$($p.overlay),bg=$barBg]$capRight "

    $cmds = [System.Collections.Generic.List[object]]::new()
    $add = { param([string[]]$Argv) $cmds.Add([pscustomobject]@{ Argv = $Argv }) }

    & $add @('set', '-g', 'status', 'on')
    & $add @('set', '-g', 'status-justify', 'left')
    & $add @('set', '-g', 'status-style', "fg=$($p.subtle),bg=$barBg")
    & $add @('set', '-g', 'status-left-length', '200')
    & $add @('set', '-g', 'status-right-length', '200')
    & $add @('set', '-g', 'status-left', $statusLeft)
    & $add @('set', '-g', 'status-right', $statusRight)
    & $add @('set', '-g', 'window-status-separator', ' ')
    & $add @('set', '-g', 'window-status-format', $winFormat)
    & $add @('set', '-g', 'window-status-current-format', $winCurrentFormat)
    & $add @('set', '-g', 'window-status-activity-style', "fg=$($p.base),bg=$($p.rose)")
    & $add @('set', '-g', 'message-style', "fg=$($p.text),bg=$($p.overlay)")
    & $add @('set', '-g', 'message-command-style', "fg=$($p.base),bg=$($p.gold)")
    & $add @('set', '-g', 'pane-border-style', "fg=$($p.hlHigh)")
    & $add @('set', '-g', 'pane-active-border-style', "fg=$($p.gold)")
    & $add @('set', '-g', 'clock-mode-colour', $p.love)
    & $add @('set', '-g', 'mode-style', "bg=$($p.hlMed)")
    & $add @('set', '-g', 'status-position', 'top')

    return $cmds
}

function ConvertTo-TmuxConfigLine {
    param([Parameter(Mandatory)]$Command)

    if ($Command.Argv.Count -ne 4 -or $Command.Argv[0] -ne 'set' -or $Command.Argv[1] -ne '-g') {
        throw "Unsupported Rose Pine command shape"
    }

    $value = [string]$Command.Argv[3]
    if ($value.Contains("'")) {
        throw "Cannot emit config value containing a single quote"
    }

    return "set -g $($Command.Argv[2]) '$value'"
}

function Get-TmuxRosePineConfigLine {
    param([string]$Variant = 'main')

    $confVariant = if ($Variant) { $Variant } else { 'main' }
    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add('# Generated by tmux/rose-pine.ps1 - do not edit by hand.')
    $lines.Add("# Variant: $confVariant")
    foreach ($cmd in (Get-TmuxRosePineCommand -Variant $confVariant)) {
        $lines.Add((ConvertTo-TmuxConfigLine -Command $cmd))
    }
    return $lines
}

if ($env:TMUX_ROSEPINE_SOURCE_ONLY) { return }

Get-TmuxRosePineConfigLine -Variant $Variant
