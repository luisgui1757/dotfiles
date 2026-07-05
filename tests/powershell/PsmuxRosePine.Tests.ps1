# Regression tests for the repo-owned Rose Pine status renderer
# (tmux/psmux-rose-pine.ps1). The renderer emits an Omer/Catppuccin-shaped bar
# (rounded pills, session left, number-on-right window cells with a zoom marker,
# directory right) painted with Rose Pine, sourced verbatim on BOTH tmux and
# psmux. These tests pin that output so a future edit cannot silently reintroduce
# arrow-chevron powerline separators, a per-redraw shell substitution
# (ConPTY-unsafe), a wrong palette, duplicated Starship context, or a stale
# generated artifact.
#
# The renderer honors a source-only seam (PSMUX_ROSEPINE_SOURCE_ONLY) so we can
# dot-source its functions without executing the psmux driver.

BeforeAll {
    $script:RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..' '..')).Path
    $env:PSMUX_ROSEPINE_SOURCE_ONLY = '1'
    . (Join-Path $script:RepoRoot 'tmux/psmux-rose-pine.ps1')

    # Rounded pill caps (Catppuccin/Omer look), NOT arrow chevrons.
    $script:CapLeft = [char]::ConvertFromUtf32(0xE0B6)
    $script:CapRight = [char]::ConvertFromUtf32(0xE0B4)
    # Powerline arrow/flame separators the community powerline theme uses; the
    # rounded-pill design must never emit them.
    $script:ChevronCodepoints = @(0xE0B0, 0xE0B1, 0xE0B2, 0xE0B3, 0xE0B8, 0xE0BA, 0xE0BC, 0xE0BE)
}

AfterAll {
    Remove-Item Env:\PSMUX_ROSEPINE_SOURCE_ONLY -ErrorAction SilentlyContinue
}

Describe 'psmux-rose-pine renderer' {
    $variants = @(
        @{ Variant = 'main'; Base = '#191724'; Surface = '#1f1d2e'; Overlay = '#26233a'; Gold = '#f6c177'; Iris = '#c4a7e7'; Muted = '#6e6a86'; Rose = '#ebbcba'; Subtle = '#908caa'; Love = '#eb6f92'; Text = '#e0def4' }
        @{ Variant = 'moon'; Base = '#232136'; Surface = '#2a273f'; Overlay = '#393552'; Gold = '#f6c177'; Iris = '#c4a7e7'; Muted = '#6e6a86'; Rose = '#ea9a97'; Subtle = '#908caa'; Love = '#eb6f92'; Text = '#e0def4' }
        @{ Variant = 'dawn'; Base = '#faf4ed'; Surface = '#fffaf3'; Overlay = '#f2e9e1'; Gold = '#ea9d34'; Iris = '#907aa9'; Muted = '#9893a5'; Rose = '#d7827e'; Subtle = '#797593'; Love = '#b4637a'; Text = '#575279' }
    )

    It "paints the <Variant> palette on the flat pill bar" -ForEach $variants {
        $cmds = Get-PsmuxRosePineCommand -Variant $Variant
        $opt = @{}
        foreach ($c in $cmds) { $opt[$c.Argv[2]] = $c.Argv[3] }

        # Bar background is the variant base; empty regions use it.
        $opt['status-style'] | Should -Be "fg=$Subtle,bg=$Base"
        # Current window: gold number fill + rounded caps; inactive: muted fill.
        $opt['window-status-current-format'] | Should -Match ([regex]::Escape("bg=$Gold"))
        $opt['window-status-current-format'] | Should -Match ([regex]::Escape($script:CapLeft))
        $opt['window-status-current-format'] | Should -Match ([regex]::Escape($script:CapRight))
        $opt['window-status-format'] | Should -Match ([regex]::Escape("bg=$Muted"))
        # Zoom marker on the current window only.
        $opt['window-status-current-format'] | Should -Match ([regex]::Escape('#{?window_zoomed_flag,'))
        $opt['window-status-format'] | Should -Not -Match ([regex]::Escape('window_zoomed_flag'))
        # Standalone pills separated by a single space (connect_separator=no).
        $opt['window-status-separator'] | Should -Be ' '
        # Session pill: prefix turns the accent love, otherwise iris.
        $opt['status-left'] | Should -Match ([regex]::Escape("#{?client_prefix,$Love,$Iris}"))
        $opt['status-left'] | Should -Match '#S'
        $opt['status-left'] | Should -Not -Match '#W'
        # Directory pill basename + one terminal-edge safety cell.
        $opt['status-right'] | Should -Match ([regex]::Escape('#{b:pane_current_path}'))
        $opt['status-right'] | Should -Match ([regex]::Escape($Rose))
        $opt['status-right'] | Should -Match ' $'
        # tmux/psmux owns multiplexer context only. Starship owns username,
        # time/path/git; host stays out of the daily surface.
        $opt['status-right'] | Should -Not -Match ([regex]::Escape('#{user}'))
        $opt['status-right'] | Should -Not -Match ([regex]::Escape('#{host_short}'))
        $opt['status-right'] | Should -Not -Match '%a %d %b %H:%M'
        # stays pinned to the top like the shared tmux.conf
        ($cmds | Where-Object { $_.Argv[2] -eq 'status-position' }).Argv[3] | Should -Be 'top'
    }

    It "uses rounded pill caps, never arrow-chevron powerline separators for <Variant>" -ForEach $variants {
        $cmds = Get-PsmuxRosePineCommand -Variant $Variant
        $joined = ($cmds | ForEach-Object { $_.Argv -join ' ' }) -join "`n"
        $joined.Contains($script:CapLeft) | Should -BeTrue
        $joined.Contains($script:CapRight) | Should -BeTrue
        foreach ($cp in $script:ChevronCodepoints) {
            $joined.Contains([char]::ConvertFromUtf32($cp)) | Should -BeFalse
        }
    }

    It "never emits a per-redraw shell substitution for <Variant>" -ForEach $variants {
        $cmds = Get-PsmuxRosePineCommand -Variant $Variant
        $joined = ($cmds | ForEach-Object { $_.Argv -join ' ' }) -join "`n"
        $joined | Should -Not -Match '#\('
    }

    It "emits a startup-safe generated config for <Variant>" -ForEach $variants {
        $lines = Get-PsmuxRosePineConfigLine -Variant $Variant
        $joined = $lines -join "`n"

        $joined | Should -Not -Match ([regex]::Escape('#{user}'))
        $joined | Should -Not -Match ([regex]::Escape('#{host_short}'))
        $joined | Should -Not -Match '%a %d %b %H:%M'
        $joined | Should -Not -Match ([regex]::Escape('#{p2:}'))
        $joined | Should -Not -Match '#\('
        $joined | Should -Not -Match '(?m)^run'
        $joined | Should -Not -Match '(?m)^set-hook'
    }

    It "keeps the generated <Variant> config committed and fresh" -ForEach $variants {
        $expected = (Get-PsmuxRosePineConfigLine -Variant $Variant) -join "`n"
        $expected = "$expected`n"
        $actualPath = Join-Path $script:RepoRoot "tmux/psmux-rose-pine.$Variant.conf"
        $mirrorPath = Join-Path $script:RepoRoot "home/dot_tmux.rose-pine.$Variant.conf"

        (Test-Path -LiteralPath $actualPath) | Should -BeTrue
        (Test-Path -LiteralPath $mirrorPath) | Should -BeTrue
        (Get-Content -Raw -LiteralPath $actualPath) | Should -Be $expected
        (Get-Content -Raw -LiteralPath $mirrorPath) | Should -Be $expected
    }

    It "defaults to the main palette for an unknown variant" {
        $cmds = Get-PsmuxRosePineCommand -Variant 'bogus'
        $opt = @{}
        foreach ($c in $cmds) { $opt[$c.Argv[2]] = $c.Argv[3] }
        $opt['status-style'] | Should -Be 'fg=#908caa,bg=#191724'
    }

    It "produces a distinct base color for each of main/moon/dawn" {
        $bases = @('main', 'moon', 'dawn') | ForEach-Object { (Get-PsmuxRosePinePalette -Variant $_).base }
        ($bases | Select-Object -Unique).Count | Should -Be 3
    }

    It "produces a distinct overlay pill color for each of main/moon/dawn" {
        $overlays = @('main', 'moon', 'dawn') | ForEach-Object { (Get-PsmuxRosePinePalette -Variant $_).overlay }
        ($overlays | Select-Object -Unique).Count | Should -Be 3
    }
}
