#!/usr/bin/env pwsh
# =============================================================================
# psmux Rose Pine -- a psmux-safe port of the official rose-pine/tmux status bar.
# =============================================================================
#
# WHY THIS EXISTS
#   The POSIX side loads the upstream rose-pine/tmux plugin (a bash script run by
#   TPM). That plugin CANNOT run on native-Windows psmux: there is no TPM/bash,
#   and its ~30 load-time shell-outs would hang ConPTY. The community
#   psmux-theme-rosepine plugin renders a DIFFERENT, powerline bar (colored
#   segment blocks + arrow chevrons). To make Windows psmux look like the flat,
#   foreground-only rose-pine/tmux bar the POSIX side shows, this script
#   reproduces the rendered `set -g` output of rose-pine/tmux directly.
#
# PSMUX-SAFE
#   Pure declarative `set -g` output. No load-time `if-shell`. No per-redraw shell
#   substitution (`#(...)`); dynamic fields use native psmux formats (`#S`, `#I`,
#   `#W`, `#{?client_prefix,...}`, `#{b:pane_current_path}`, strftime) plus the
#   username/hostname baked in at load. Colors are inlined with `#[fg=...]`
#   because psmux stores but does NOT apply `window-status-*-style` (psmux v3.3.x),
#   so relying on the style options would leave window cells uncolored.
#
# VARIANTS
#   Reads `@rosepine-variant` (main|moon|dawn); defaults to main.
#
# SOURCE OF TRUTH
#   Palette + role mapping + composition mirror rose-pine/tmux (rose-pine.tmux)
#   at commit b6138c51573425ccdc33c91464597323baec3b7e, configured to match this
#   dotfiles POSIX overlay (tmux/tmux.posix.conf): session + current-program window
#   names on the left; user + short host + date/time + directory on the right;
#   with the same Nerd Font icons.
#
# INVARIANT: this file MUST stay pure ASCII (Windows PowerShell 5.1 parse safety);
# the Nerd Font glyphs are built from codepoints at runtime, never embedded.
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
                base = '#232136'; text = '#e0def4'; subtle = '#908caa'; muted = '#6e6a86'
                love = '#eb6f92'; gold = '#f6c177'; rose = '#ea9a97'; pine = '#3e8fb0'
                foam = '#9ccfd8'; iris = '#c4a7e7'; hlMed = '#44415a'; hlHigh = '#56526e'
            }
        }
        'dawn' {
            return @{
                base = '#faf4ed'; text = '#575279'; subtle = '#797593'; muted = '#9893a5'
                love = '#b4637a'; gold = '#ea9d34'; rose = '#d7827e'; pine = '#286983'
                foam = '#56949f'; iris = '#907aa9'; hlMed = '#dfdad9'; hlHigh = '#cecacd'
            }
        }
        default {
            return @{
                base = '#191724'; text = '#e0def4'; subtle = '#908caa'; muted = '#6e6a86'
                love = '#eb6f92'; gold = '#f6c177'; rose = '#ebbcba'; pine = '#31748f'
                foam = '#9ccfd8'; iris = '#c4a7e7'; hlMed = '#403d52'; hlHigh = '#524f67'
            }
        }
    }
}

function Get-PsmuxRosePineCommand {
    param(
        [string]$Variant = 'main',
        [string]$UserName = '',
        [string]$ComputerName = '',
        [string]$Separator = '  '
    )

    $p = Get-PsmuxRosePinePalette -Variant $Variant

    # Nerd Font glyphs (codepoints -> runtime string keeps this .ps1 pure ASCII).
    # Match the icons in tmux/tmux.posix.conf: session, current-window, folder, user,
    # host, clock.
    $iSession = [char]::ConvertFromUtf32(0xEB7F)
    $iWindow = [char]::ConvertFromUtf32(0xEB23)
    $iFolder = [char]::ConvertFromUtf32(0xF413)
    $iUser = [char]::ConvertFromUtf32(0xF007)
    $iHost = [char]::ConvertFromUtf32(0xF048B)
    $iClock = [char]::ConvertFromUtf32(0xF00F0)

    # rose-pine/tmux defaults every separator to two spaces (flat, no powerline).
    $sep = $Separator
    $field = "#[fg=$($p.text)]$sep"

    # Window list (rose-pine/tmux "show_current_program" mode). Foreground inlined
    # on the index too, so psmux paints it (window-status-*-style is ignored).
    $winFormat = "#[fg=$($p.iris)]#I$sep#[fg=$($p.iris)]#W"
    $winCurrentFormat = "#[fg=$($p.gold)]#I$sep#[fg=$($p.gold)]#W"

    # status-left: session (icon turns love while prefix is active) + window name.
    $showSession = "#{?client_prefix,#[fg=$($p.love)],#[fg=$($p.text)]}$iSession #[fg=$($p.text)]#S"
    $showWindow = "#[fg=$($p.subtle)]$iWindow #[fg=$($p.rose)]#W#[fg=$($p.text)]"
    $statusLeft = "$showSession$field$showWindow$field"

    # status-right: user + short host + date/time + directory basename. #(whoami)
    # and #h are replaced with load-time literals so no shell runs per redraw.
    $showUser = "#[fg=$($p.iris)]$UserName#[fg=$($p.subtle)]$sep#[fg=$($p.subtle)]$iUser"
    $showHost = "#[fg=$($p.text)]$ComputerName#[fg=$($p.subtle)]$sep#[fg=$($p.subtle)]$iHost"
    $showDate = "#[fg=$($p.foam)]%a %d %b %H:%M#[fg=$($p.subtle)]$sep#[fg=$($p.subtle)]$iClock #[fg=$($p.text)]"
    $showDir = "#[fg=$($p.subtle)]$iFolder #[fg=$($p.rose)]#{b:pane_current_path}"
    $statusRight = "$showUser$field$showHost$field$showDate$field$showDir"

    $cmds = [System.Collections.Generic.List[object]]::new()
    $add = { param([string[]]$Argv) $cmds.Add([pscustomobject]@{ Argv = $Argv }) }

    & $add @('set', '-g', 'status', 'on')
    & $add @('set', '-g', 'status-justify', 'left')
    & $add @('set', '-g', 'status-style', "fg=$($p.pine),bg=$($p.base)")
    & $add @('set', '-g', 'status-left-length', '200')
    & $add @('set', '-g', 'status-right-length', '200')
    & $add @('set', '-g', 'status-left', $statusLeft)
    & $add @('set', '-g', 'status-right', $statusRight)
    & $add @('set', '-g', 'window-status-separator', $sep)
    & $add @('set', '-g', 'window-status-format', $winFormat)
    & $add @('set', '-g', 'window-status-current-format', $winCurrentFormat)
    & $add @('set', '-g', 'window-status-activity-style', "fg=$($p.base),bg=$($p.rose)")
    & $add @('set', '-g', 'message-style', "fg=$($p.muted),bg=$($p.base)")
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
        throw "Unsupported psmux Rose Pine command shape"
    }

    $value = [string]$Command.Argv[3]
    if ($value.Contains("'")) {
        throw "Cannot emit psmux config value containing a single quote"
    }

    return "set -g $($Command.Argv[2]) '$value'"
}

function Get-PsmuxRosePineConfigLine {
    param([string]$Variant = 'main')

    $confVariant = if ($Variant) { $Variant } else { 'main' }
    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add('# Generated by tmux/psmux-rose-pine.ps1 - do not edit by hand.')
    $lines.Add("# Variant: $confVariant")
    foreach ($cmd in (Get-PsmuxRosePineCommand `
            -Variant $confVariant `
            -UserName '#{user}' `
            -ComputerName '#{host_short}' `
            -Separator '#{p2:}')) {
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

$who = $env:USERNAME
if (-not $who) { $who = [Environment]::UserName }
$comp = $env:COMPUTERNAME
if (-not $comp) { try { $comp = [Environment]::MachineName } catch { $comp = '' } }
if ($comp) { $comp = ($comp -split '\.')[0] }

foreach ($cmd in (Get-PsmuxRosePineCommand -Variant $Variant -UserName $who -ComputerName $comp)) {
    $argv = $cmd.Argv
    & $psmuxBin @argv 2>&1 | Out-Null
}

Write-Host "psmux-rose-pine: loaded ($Variant)" -ForegroundColor DarkGray
