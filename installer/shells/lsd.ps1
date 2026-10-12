# ---- Rose Pine file listing colors ------------------------------------------
# lsd reads LS_COLORS for file/directory names. Keep this before the lsd
# functions so every shortcut inherits the same palette. Dotfiles owns this
# palette by default; set DOTFILES_LS_COLORS before profile startup for an
# explicit override.
$script:RosePineLsColors = @(
    'no=38;2;224;222;244'
    'fi=38;2;224;222;244'
    'di=38;2;246;193;119'
    'ln=38;2;196;167;231'
    'pi=38;2;156;207;216'
    'so=38;2;156;207;216'
    'do=38;2;156;207;216'
    'bd=38;2;235;188;186'
    'cd=38;2;235;188;186'
    'or=38;2;235;111;146'
    'mi=38;2;235;111;146'
    'su=38;2;235;111;146'
    'sg=38;2;246;193;119'
    'ca=38;2;235;188;186'
    'tw=38;2;246;193;119'
    'ow=38;2;246;193;119'
    'st=38;2;246;193;119'
    'ex=38;2;235;111;146'
    '*.md=38;2;156;207;216'
    '*.markdown=38;2;156;207;216'
    '*.txt=38;2;224;222;244'
    '*.toml=38;2;156;207;216'
    '*.yaml=38;2;156;207;216'
    '*.yml=38;2;156;207;216'
    '*.json=38;2;156;207;216'
    '*.jsonc=38;2;156;207;216'
    '*.lua=38;2;196;167;231'
    '*.vim=38;2;196;167;231'
    '*.sh=38;2;246;193;119'
    '*.bash=38;2;246;193;119'
    '*.zsh=38;2;246;193;119'
    '*.ps1=38;2;196;167;231'
    '*.zip=38;2;235;188;186'
    '*.tar=38;2;235;188;186'
    '*.gz=38;2;235;188;186'
    '*.tgz=38;2;235;188;186'
    '*.xz=38;2;235;188;186'
    '*.7z=38;2;235;188;186'
    '*.png=38;2;235;188;186'
    '*.jpg=38;2;235;188;186'
    '*.jpeg=38;2;235;188;186'
    '*.webp=38;2;235;188;186'
    '*.svg=38;2;156;207;216'
) -join ':'
if ([string]::IsNullOrWhiteSpace($env:DOTFILES_LS_COLORS)) {
    $env:LS_COLORS = $script:RosePineLsColors
} else {
    $env:LS_COLORS = $env:DOTFILES_LS_COLORS
}

# ---- lsd directory listings --------------------------------------------------
# Keep this guarded so profile startup stays silent and clean before setup has
# provisioned lsd. The upstream shell aliases include ls/l/la/lla/lt; functions
# are required in PowerShell because useful entries carry arguments.
if (Get-Command lsd -ErrorAction SilentlyContinue) {
    foreach ($name in 'ls', 'l', 'la', 'lla', 'lt') {
        Remove-Item "Alias:$name" -ErrorAction SilentlyContinue
    }
    function global:ls { lsd @args }
    function global:l { lsd -l @args }
    function global:la { lsd -a @args }
    function global:lla { lsd -la @args }
    function global:lt { lsd --tree @args }
}

