# ---- Rose Pine colors for fzf -------------------------------------------------
# A value that was set before stays after the colors, so it wins. The check
# keeps a second profile load from adding the colors again.
if (-not "$env:FZF_DEFAULT_OPTS".Contains($script:RosePineFzfColors)) {
    $env:FZF_DEFAULT_OPTS = ($script:RosePineFzfColors + ' ' + $env:FZF_DEFAULT_OPTS).TrimEnd()
}

# ---- PSFzf: fuzzy history / file / directory pickers -------------------------
# Unifies the shell fuzzy UX on fzf (the same tool the zsh side uses), wired
# only when BOTH the PSFzf module and the fzf binary are present so a machine
# without fzf keeps a working profile. Ctrl+R intentionally OVERRIDES the
# PSReadLine reverse-history search with the fzf fuzzy picker (POSIX parity --
# the zsh side binds the same chord). Ctrl+T = fuzzy file insert, Alt+C = fuzzy
# cd. The fzf selection installs fzf + PSFzf.
if ((Test-Path -LiteralPath $script:DotfilesPSFzfPath -PathType Leaf) -and (Get-Command fzf -ErrorAction SilentlyContinue)) {
    try {
        Import-Module PSReadLine -Global -ErrorAction Stop
        Import-Module $script:DotfilesPSFzfPath -Global -ErrorAction Stop
        Set-PsFzfOption -PSReadlineChordProvider 'Ctrl+t' `
            -PSReadlineChordReverseHistory 'Ctrl+r' `
            -PSReadlineChordSetLocation 'Alt+c' -ErrorAction Stop
    } catch { Write-Verbose $_.Exception.Message }
}

