[CmdletBinding()]
param(
    [string]$Repo = '',
    [string]$HomeOverride = '',
    [switch]$ConfigOnly
)

$ErrorActionPreference = 'Stop'
$script:PassCount = 0
$script:FailCount = 0
$script:SkipCount = 0

function Add-Pass {
    param([string]$Message)
    Write-Host "PASS: $Message"
    $script:PassCount++
}

function Add-Fail {
    param([string]$Message)
    Write-Host "FAIL: $Message" -ForegroundColor Red
    $script:FailCount++
}

function Add-Skip {
    param([string]$Message)
    Write-Host "SKIP: $Message"
    $script:SkipCount++
}

function Complete-Validation {
    Write-Host ("SUMMARY: {0} passed, {1} skipped, {2} failed" -f $script:PassCount, $script:SkipCount, $script:FailCount)
    if ($script:FailCount -ne 0) {
        exit 1
    }
}

function Update-ValidatorPath {
    $parts = @()
    $scoopRoot = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
    $shimDir = Join-Path $scoopRoot 'shims'
    if (Test-Path -LiteralPath $shimDir) {
        $parts += $shimDir
    }
    foreach ($scope in 'Machine', 'User') {
        $p = [Environment]::GetEnvironmentVariable('PATH', $scope)
        if ($p) { $parts += ($p -split ';') }
    }
    $parts += ($env:PATH -split ';')
    $seen = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    $env:PATH = ($parts | Where-Object { $_ -and $seen.Add($_) }) -join ';'
}

function Test-CommandPath {
    param([string]$Name)
    if (Get-Command $Name -ErrorAction SilentlyContinue) {
        Add-Pass "$Name is on PATH"
    } else {
        Add-Fail "$Name is not on PATH"
    }
}

function Assert-ContentEqual {
    param([string]$Path, [string]$Expected)
    if (-not (Test-Path -LiteralPath $Path)) {
        Add-Fail "$Path does not exist"
        return
    }
    if (-not (Test-Path -LiteralPath $Expected)) {
        Add-Fail "$Expected does not exist"
        return
    }
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
    $expectedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Expected).Hash
    if ($actualHash -eq $expectedHash) {
        Add-Pass "$Path content matches $Expected"
    } else {
        Add-Fail "$Path content differs from $Expected"
    }
}

function Assert-NvimSymlink {
    param([string]$Path, [string]$Expected)
    if (-not (Test-Path -LiteralPath $Path)) {
        Add-Fail "$Path does not exist"
        return
    }
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.LinkType -ne 'SymbolicLink') {
        Add-Fail "$Path is not a symlink"
        return
    }
    $target = @($item.Target)[0]
    $actualResolved = (Resolve-Path -LiteralPath $target -ErrorAction Stop).Path
    $expectedResolved = (Resolve-Path -LiteralPath $Expected -ErrorAction Stop).Path
    if ($actualResolved -eq $expectedResolved) {
        Add-Pass "$Path points to $expectedResolved"
    } else {
        Add-Fail "$Path points to $actualResolved, expected $expectedResolved"
    }
}

function Assert-NvimVersion {
    if ($ConfigOnly) {
        Add-Skip "nvim version skipped by -ConfigOnly"
        return
    }
    if (-not (Get-Command nvim -ErrorAction SilentlyContinue)) {
        Add-Fail "nvim version cannot be checked because nvim is not on PATH"
        return
    }
    $nvimLine = & nvim --version | Select-Object -First 1
    if ($nvimLine -match '^NVIM v(0\.1[2-9]|0\.[2-9][0-9]|[1-9]\.)') {
        Add-Pass "nvim version is supported: $nvimLine"
    } else {
        Add-Fail "nvim version is below 0.12: $nvimLine"
    }
}

function Invoke-NvimChecked {
    param([string]$Name, [string[]]$NvimArgs)
    if ($ConfigOnly) {
        Add-Skip "nvim $Name skipped by -ConfigOnly"
        return
    }
    if (-not (Get-Command nvim -ErrorAction SilentlyContinue)) {
        Add-Fail "nvim $Name cannot run because nvim is not on PATH"
        return
    }
    $log = Join-Path ([IO.Path]::GetTempPath()) "dotfiles-greenfield-nvim-$Name.log"
    $global:LASTEXITCODE = 0
    # PowerShell 7.4+ promotes a non-zero native-command exit into a terminating
    # NativeCommandError when PSNativeCommandUseErrorActionPreference is true; we read
    # LASTEXITCODE ourselves below, so disable that here (function-local).
    $PSNativeCommandUseErrorActionPreference = $false
    & nvim --headless @NvimArgs 2>&1 | Tee-Object -FilePath $log
    $rc = $LASTEXITCODE
    if ($rc -eq 0) {
        Add-Pass "nvim $Name exited 0"
    } else {
        Add-Fail "nvim $Name exited $rc; log: $log"
    }
}

function Assert-MasonTool {
    param([string]$Name, [string[]]$FileNames)
    if ($ConfigOnly) {
        Add-Skip "Mason $Name skipped by -ConfigOnly"
        return
    }
    $masonBin = Join-Path $env:LOCALAPPDATA 'nvim-data\mason\bin'
    foreach ($fileName in $FileNames) {
        $candidate = Join-Path $masonBin $fileName
        if (Test-Path -LiteralPath $candidate) {
            Add-Pass "Mason installed $Name at $candidate"
            return
        }
    }
    Add-Fail "Mason did not install $Name into $masonBin"
}

function Assert-ChezmoiVerify {
    $chezmoi = Get-Command chezmoi -ErrorAction SilentlyContinue
    if (-not $chezmoi) {
        if ($ConfigOnly) {
            Add-Skip "chezmoi verify skipped because chezmoi is not on PATH"
        } else {
            Add-Fail "chezmoi verify cannot run because chezmoi is not on PATH"
        }
        return
    }
    $global:LASTEXITCODE = 0
    if ($ConfigOnly) {
        $output = & chezmoi --source (Join-Path $Repo 'home') verify --exclude externals,scripts 2>&1
    } else {
        $output = & chezmoi --source (Join-Path $Repo 'home') verify 2>&1
    }
    $rc = $LASTEXITCODE
    if ($rc -eq 0) {
        Add-Pass "chezmoi verify is clean"
    } else {
        Add-Fail "chezmoi verify exited $rc"
        $output | ForEach-Object { Write-Host $_ }
    }
}

function Test-WindowsTerminalPresent {
    if (Get-Command wt -ErrorAction SilentlyContinue) {
        return $true
    }
    $portableRoot = Join-Path $env:LOCALAPPDATA 'Programs\WindowsTerminal'
    foreach ($candidate in @(
            (Join-Path $portableRoot 'wt.exe'),
            (Join-Path $portableRoot 'WindowsTerminal.exe')
        )) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return $true
        }
    }
    $packagedSettings = Join-Path $env:LOCALAPPDATA 'Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json'
    $unpackagedSettings = Join-Path $env:LOCALAPPDATA 'Microsoft\Windows Terminal\settings.json'
    return (Test-Path -LiteralPath $packagedSettings -PathType Leaf) -or
        (Test-Path -LiteralPath $unpackagedSettings -PathType Leaf)
}

function Read-JsoncFile {
    param([Parameter(Mandatory)] [string]$Path)
    $raw = Get-Content -Raw -LiteralPath $Path
    $json = (($raw -split "`n" | Where-Object { $_ -notmatch "^\s*//" }) -join "`n")
    return ($json | ConvertFrom-Json)
}

function Assert-WindowsTerminalPortableSettings {
    $settingsPath = Join-Path $env:LOCALAPPDATA 'Microsoft\Windows Terminal\settings.json'
    if (-not (Test-Path -LiteralPath $settingsPath -PathType Leaf)) {
        if (Test-WindowsTerminalPresent) {
            Add-Fail "portable Windows Terminal settings.json is missing"
        } else {
            Add-Skip "portable Windows Terminal settings skipped because WT is not present"
        }
        return
    }

    try {
        $settings = Read-JsoncFile -Path $settingsPath
        $hasRosePine = ($settings.theme -eq 'rose-pine') -and
            ($settings.profiles.defaults.colorScheme -eq 'rose-pine')
        $hasHackFont = ($settings.profiles.defaults.font.face -eq 'Hack Nerd Font')
        if ($hasRosePine -and $hasHackFont) {
            Add-Pass "portable Windows Terminal settings contain rose-pine and Hack Nerd Font"
        } else {
            Add-Fail "portable Windows Terminal settings missing rose-pine or Hack Nerd Font"
        }
    } catch {
        Add-Fail ("portable Windows Terminal settings could not be parsed: " + $_.Exception.Message)
    }
}

if (-not $Repo) {
    $scriptDir = Split-Path -Parent $PSCommandPath
    $Repo = Join-Path $scriptDir '..\..'
}
$Repo = (Resolve-Path -LiteralPath $Repo -ErrorAction Stop).Path

if ($HomeOverride) {
    $resolvedHome = (Resolve-Path -LiteralPath $HomeOverride -ErrorAction Stop).Path
    $env:USERPROFILE = $resolvedHome
    $env:HOME = $resolvedHome
    $env:LOCALAPPDATA = Join-Path $resolvedHome 'AppData\Local'
    $env:APPDATA = Join-Path $resolvedHome 'AppData\Roaming'
}

if (-not $env:LOCALAPPDATA) {
    $env:LOCALAPPDATA = Join-Path $env:USERPROFILE 'AppData\Local'
}

Update-ValidatorPath
Write-Host ("validate.ps1: repo={0} home={1} mode={2}" -f $Repo, $env:USERPROFILE, $(if ($ConfigOnly) { 'config-only' } else { 'full' }))

if ($ConfigOnly) {
    Add-Skip "full setup tool checks skipped by -ConfigOnly"
} else {
    foreach ($cmd in @('git', 'nvim', 'rg', 'fd', 'fzf', 'chezmoi', 'lazygit', 'starship', 'tree-sitter', 'cmake', 'lsd', 'psmux', 'pwsh', 'win32yank')) {
        Test-CommandPath $cmd
    }
}

Assert-NvimVersion

$nvimTarget = Join-Path $env:LOCALAPPDATA 'nvim'
Assert-NvimSymlink -Path $nvimTarget -Expected (Join-Path $Repo 'nvim')
Assert-ContentEqual -Path (Join-Path $nvimTarget 'init.lua') -Expected (Join-Path $Repo 'nvim\init.lua')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.config\starship.toml') -Expected (Join-Path $Repo 'starship\starship.toml')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.config\gh-dash\config.yml') -Expected (Join-Path $Repo 'gh-dash\config.yml')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.config\lsd\config.yaml') -Expected (Join-Path $Repo 'lsd\config.yaml')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.config\lsd\colors.yaml') -Expected (Join-Path $Repo 'lsd\colors.yaml')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.psmux.conf') -Expected (Join-Path $Repo 'tmux\psmux.conf')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.tmux.conf') -Expected (Join-Path $Repo 'tmux\tmux.conf')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.tmux.windows.conf') -Expected (Join-Path $Repo 'tmux\tmux.windows.conf')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.tmux.rose-pine.main.conf') -Expected (Join-Path $Repo 'tmux\psmux-rose-pine.main.conf')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.tmux.rose-pine.moon.conf') -Expected (Join-Path $Repo 'tmux\psmux-rose-pine.moon.conf')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE '.tmux.rose-pine.dawn.conf') -Expected (Join-Path $Repo 'tmux\psmux-rose-pine.dawn.conf')
Assert-ContentEqual -Path (Join-Path $env:USERPROFILE 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1') -Expected (Join-Path $Repo 'shells\powershell_profile.ps1')
Assert-ContentEqual -Path (Join-Path $env:LOCALAPPDATA 'lazygit\config.yml') -Expected (Join-Path $Repo 'lazygit\config.windows.yml')
Assert-ContentEqual -Path (Join-Path $env:APPDATA 'herdr\config.toml') -Expected (Join-Path $Repo 'herdr\config.windows.toml')
Assert-WindowsTerminalPortableSettings

Assert-ChezmoiVerify
Invoke-NvimChecked -Name lazy -NvimArgs @('+Lazy! restore', '+qa')
$oldTreeSitterSyncInstall = $env:DOTFILES_TREESITTER_SYNC_INSTALL
try {
    $env:DOTFILES_TREESITTER_SYNC_INSTALL = '1'
    Invoke-NvimChecked -Name treesitter -NvimArgs @('-u', ((Join-Path $Repo 'nvim\init.lua') -replace '\\', '/'), '-c', "lua require('lazy').load({ plugins = { 'nvim-treesitter' } })", '+qa')
} finally {
    if ($null -eq $oldTreeSitterSyncInstall) {
        Remove-Item Env:DOTFILES_TREESITTER_SYNC_INSTALL -ErrorAction SilentlyContinue
    } else {
        $env:DOTFILES_TREESITTER_SYNC_INSTALL = $oldTreeSitterSyncInstall
    }
}
Invoke-NvimChecked -Name mason -NvimArgs @("+lua require('util.mason_tools').run_checked('MasonToolsInstallSync')")
Assert-MasonTool -Name 'lua-language-server' -FileNames @('lua-language-server.cmd', 'lua-language-server.exe', 'lua-language-server')
Assert-MasonTool -Name 'stylua' -FileNames @('stylua.cmd', 'stylua.exe', 'stylua')

Complete-Validation
