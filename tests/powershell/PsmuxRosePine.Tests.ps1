# Regression tests for the repo-owned psmux Rose Pine renderer
# (tmux/psmux-rose-pine.ps1). The renderer reproduces the flat, foreground-only
# rose-pine/tmux `set -g` output for psmux. These tests pin that output so a
# future edit cannot silently reintroduce a powerline look, a per-redraw shell
# substitution (ConPTY-unsafe), a wrong palette, or a broken variant switch.
#
# The renderer honors a source-only seam (PSMUX_ROSEPINE_SOURCE_ONLY) so we can
# dot-source its functions without executing the psmux driver.

BeforeAll {
    $script:RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..' '..')).Path
    $env:PSMUX_ROSEPINE_SOURCE_ONLY = '1'
    . (Join-Path $script:RepoRoot 'tmux/psmux-rose-pine.ps1')
}

AfterAll {
    Remove-Item Env:\PSMUX_ROSEPINE_SOURCE_ONLY -ErrorAction SilentlyContinue
}

Describe 'psmux-rose-pine renderer' {
    $variants = @(
        @{ Variant = 'main'; Base = '#191724'; Pine = '#31748f'; Gold = '#f6c177'; Iris = '#c4a7e7' }
        @{ Variant = 'moon'; Base = '#232136'; Pine = '#3e8fb0'; Gold = '#f6c177'; Iris = '#c4a7e7' }
        @{ Variant = 'dawn'; Base = '#faf4ed'; Pine = '#286983'; Gold = '#ea9d34'; Iris = '#907aa9' }
    )

    It "renders the <Variant> palette on the flat rose-pine/tmux status bar" -ForEach $variants {
        $cmds = Get-PsmuxRosePineCommand -Variant $Variant -UserName 'tester' -ComputerName 'HOST'
        $opt = @{}
        foreach ($c in $cmds) { $opt[$c.Argv[2]] = $c.Argv[3] }

        $opt['status-style'] | Should -Be "fg=$Pine,bg=$Base"
        $leftSeparator = [char]::ConvertFromUtf32(0xEA9C)
        $rightSeparator = [char]::ConvertFromUtf32(0xEA9B)
        $windowStatusSeparator = [char]::ConvertFromUtf32(0xEB70)
        # foreground inlined (psmux ignores window-status-*-style)
        $opt['window-status-current-format'] | Should -Match ([regex]::Escape("#[fg=$Gold]"))
        $opt['window-status-format'] | Should -Match ([regex]::Escape("#[fg=$Iris]"))
        $opt['window-status-current-format'] | Should -Match ([regex]::Escape(" $leftSeparator "))
        $opt['window-status-format'] | Should -Match ([regex]::Escape(" $leftSeparator "))
        $opt['window-status-separator'] | Should -Be " $windowStatusSeparator "
        $opt['status-right'] | Should -Match ([regex]::Escape(" $rightSeparator "))
        # directory basename present, matching rose-pine/tmux @rose_pine_directory
        $opt['status-right'] | Should -Match ([regex]::Escape('#{b:pane_current_path}'))
        # one terminal-edge safety cell: the last visible glyph/text must not sit
        # in the final column on Windows Terminal / ConPTY.
        $opt['status-right'] | Should -Match ([regex]::Escape('#{b:pane_current_path} '))
        # stays pinned to the top like the shared tmux.conf
        ($cmds | Where-Object { $_.Argv[2] -eq 'status-position' }).Argv[3] | Should -Be 'top'
    }

    It "never emits a per-redraw shell substitution for <Variant>" -ForEach $variants {
        $cmds = Get-PsmuxRosePineCommand -Variant $Variant -UserName 'tester' -ComputerName 'HOST'
        $joined = ($cmds | ForEach-Object { $_.Argv -join ' ' }) -join "`n"
        $joined | Should -Not -Match '#\('
    }

    It "emits a startup-safe generated config for <Variant>" -ForEach $variants {
        $lines = Get-PsmuxRosePineConfigLine -Variant $Variant
        $joined = $lines -join "`n"

        $joined | Should -Match ([regex]::Escape('#{user}'))
        $joined | Should -Match ([regex]::Escape('#{host_short}'))
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

    It "renders a flat bar with no powerline separator glyphs for <Variant>" -ForEach $variants {
        $cmds = Get-PsmuxRosePineCommand -Variant $Variant
        $joined = ($cmds | ForEach-Object { $_.Argv -join ' ' }) -join "`n"
        foreach ($cp in @(0xE0B0, 0xE0B1, 0xE0B2, 0xE0B3, 0xE0B4, 0xE0B6, 0xE0BA, 0xE0BC)) {
            $joined.Contains([char]::ConvertFromUtf32($cp)) | Should -BeFalse
        }
    }

    It "defaults to the main palette for an unknown variant" {
        $cmds = Get-PsmuxRosePineCommand -Variant 'bogus'
        $opt = @{}
        foreach ($c in $cmds) { $opt[$c.Argv[2]] = $c.Argv[3] }
        $opt['status-style'] | Should -Be 'fg=#31748f,bg=#191724'
    }

    It "produces a distinct base color for each of main/moon/dawn" {
        $bases = @('main', 'moon', 'dawn') | ForEach-Object { (Get-PsmuxRosePinePalette -Variant $_).base }
        ($bases | Select-Object -Unique).Count | Should -Be 3
    }
}
