#!/usr/bin/env pwsh
# =============================================================================
# Rose Pine status renderer -- the repo-owned source of truth for the tmux AND
# psmux status bar. Emits an Omer/Catppuccin-shaped bar (rounded pills, session
# on the left, window cells with the number on the right and a zoom marker on the
# current window, directory on the right) painted with the Rose Pine palette.
# =============================================================================
#
# WHY THIS EXISTS
#   The bar is generated once and sourced verbatim on BOTH platforms so the look
#   is byte-identical everywhere:
#     * POSIX tmux  -> tmux/tmux.posix.conf source-files the generated variant.
#     * Windows psmux -> tmux/tmux.windows.conf source-files the same variant.
#   We deliberately do NOT use a tmux/psmux theme *plugin* for rendering:
#   rose-pine/tmux is a bash/TPM script (cannot run on psmux, ~30 load-time
#   shell-outs would hang ConPTY), and the community psmux-theme-rosepine renders
#   a different powerline bar (arrow chevrons + segment blocks). A single
#   repo-owned renderer is the only way to guarantee the same Omer-style bar on
#   both. TPM/PPM still install the FUNCTIONAL plugins (sensible/yank/resurrect/
#   continuum) -- they do not own the status bar.
#
# PSMUX-SAFE / TMUX-SAFE
#   Pure declarative `set -g` output. No load-time `if-shell`. No per-redraw shell
#   substitution (`#(...)`); dynamic fields use native formats (`#S`, `#I`, `#W`,
#   `#{?client_prefix,...}`, `#{?window_zoomed_flag,...}`, `#{b:pane_current_path}`).
#   Every colour is inlined with `#[fg=...,bg=...]` because psmux stores but does
#   NOT apply `window-status-*-style` (psmux v3.3.x), so relying on the style
#   options would leave window cells uncoloured. The rounded pill caps are the
#   Nerd Font half-circles U+E0B6 / U+E0B4; we intentionally avoid the powerline
#   arrow chevrons (U+E0B0/U+E0B2) that give the community powerline look.
#
# VARIANTS
#   Reads the requested variant (main|moon|dawn); defaults to main.
#
# INVARIANT: this file MUST stay pure ASCII (Windows PowerShell 5.1 parse safety);
# the Nerd Font glyphs are built from codepoints at runtime, never embedded.
# The generated *.conf artifacts DO carry the rendered glyphs.
# =============================================================================

param(
    [switch]$EmitConf,
    [string]$Variant = ''
)

$ErrorActionPreference = 'Continue'

function Get-PsmuxRosePinePalette {
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

function Get-PsmuxRosePineCommand {
    param(
        [string]$Variant = 'main'
    )

    $p = Get-PsmuxRosePinePalette -Variant $Variant

    # Nerd Font glyphs (codepoints -> runtime string keeps this .ps1 pure ASCII).
    #   capLeft/capRight : rounded pill caps (half circles), NOT arrow chevrons.
    #   iSession/iFolder : left session pill + right directory pill icons.
    #   iZoom            : zoom marker shown on the current window when zoomed.
    $capLeft = [char]::ConvertFromUtf32(0xE0B6)
    $capRight = [char]::ConvertFromUtf32(0xE0B4)
    $iSession = [char]::ConvertFromUtf32(0xEB7F)
    $iFolder = [char]::ConvertFromUtf32(0xF413)
    $iZoom = [char]::ConvertFromUtf32(0xF065)

    # Session accent turns love while the prefix is held; otherwise use pine so
    # the default session pill reads Rose Pine, not Catppuccin-purple.
    $sessAccent = "#{?client_prefix,$($p.love),$($p.pine)}"

    # status-left: rounded session pill. icon segment (accent bg) + name segment
    # (overlay bg). Trailing space separates it from the window list.
    $statusLeft = "#[fg=$sessAccent,bg=$($p.base)]$capLeft#[fg=$($p.base),bg=$sessAccent] $iSession #[fg=$($p.text),bg=$($p.overlay)] #S #[fg=$($p.overlay),bg=$($p.base)]$capRight "

    # window cells: name segment (overlay bg) + number segment on the RIGHT
    # (Catppuccin number_position=right, fill=number). Current window fills the
    # number in gold and appends a zoom marker; inactive fills it muted.
    $winFormat = "#[fg=$($p.overlay),bg=$($p.base)]$capLeft#[fg=$($p.subtle),bg=$($p.overlay)] #W #[fg=$($p.base),bg=$($p.muted)] #I #[fg=$($p.muted),bg=$($p.base)]$capRight"
    $winCurrentFormat = "#[fg=$($p.overlay),bg=$($p.base)]$capLeft#[fg=$($p.text),bg=$($p.overlay)] #W#{?window_zoomed_flag, $iZoom,} #[fg=$($p.base),bg=$($p.gold)] #I #[fg=$($p.gold),bg=$($p.base)]$capRight"

    # status-right: rounded directory pill (basename only). One terminal-edge
    # safety cell so the last visible glyph is not clipped by Windows Terminal.
    $statusRight = "#[fg=$($p.overlay),bg=$($p.base)]$capLeft#[fg=$($p.subtle),bg=$($p.overlay)] $iFolder #[fg=$($p.rose),bg=$($p.overlay)]#{b:pane_current_path} #[fg=$($p.overlay),bg=$($p.base)]$capRight "

    $cmds = [System.Collections.Generic.List[object]]::new()
    $add = { param([string[]]$Argv) $cmds.Add([pscustomobject]@{ Argv = $Argv }) }

    & $add @('set', '-g', 'status', 'on')
    & $add @('set', '-g', 'status-justify', 'left')
    & $add @('set', '-g', 'status-style', "fg=$($p.subtle),bg=$($p.base)")
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
    & $add @('set', '-g', 'display-panes-colour', $p.gold)
    & $add @('set', '-g', 'display-panes-active-colour', $p.text)
    & $add @('set', '-g', 'clock-mode-colour', $p.love)
    & $add @('set', '-g', 'mode-style', "bg=$($p.hlMed)")
    & $add @('set', '-g', 'status-position', 'top')

    return $cmds
}

function ConvertTo-PsmuxConfigLine {
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

function Get-PsmuxRosePineConfigLine {
    param([string]$Variant = 'main')

    $confVariant = if ($Variant) { $Variant } else { 'main' }
    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add('# Generated by tmux/psmux-rose-pine.ps1 - do not edit by hand.')
    $lines.Add("# Variant: $confVariant")
    foreach ($cmd in (Get-PsmuxRosePineCommand -Variant $confVariant)) {
        $lines.Add((ConvertTo-PsmuxConfigLine -Command $cmd))
    }
    return $lines
}

if ($env:PSMUX_ROSEPINE_SOURCE_ONLY) { return }

if ($EmitConf) {
    $confVariant = if ($Variant) { $Variant } else { 'main' }
    Get-PsmuxRosePineConfigLine -Variant $confVariant
    return
}

function Get-PsmuxBin {
    foreach ($n in 'psmux', 'pmux', 'tmux') {
        $c = Get-Command $n -ErrorAction SilentlyContinue
        if ($c) { return $c.Source }
    }
    return 'psmux'
}

$psmuxBin = Get-PsmuxBin
if (-not $Variant) {
    $Variant = (& $psmuxBin show-options -gv '@rosepine-variant' 2>&1 | Out-String).Trim()
}
if (-not $Variant -or ($Variant -match 'unknown|error|invalid')) { $Variant = 'main' }

foreach ($cmd in (Get-PsmuxRosePineCommand -Variant $Variant)) {
    $argv = $cmd.Argv
    & $psmuxBin @argv 2>&1 | Out-Null
}

Write-Host "psmux-rose-pine: loaded ($Variant)" -ForegroundColor DarkGray
