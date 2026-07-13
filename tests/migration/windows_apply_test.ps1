$ErrorActionPreference = 'Stop'
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$script:RepoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$script:SourceDir = Join-Path $script:RepoRoot 'home'
$script:LocalAppDataSource = Join-Path $script:RepoRoot 'windows\chezmoi-localappdata'
$script:AppDataSource = Join-Path $script:RepoRoot 'windows\chezmoi-appdata'
$script:DocumentsSource = Join-Path $script:RepoRoot 'windows\chezmoi-documents'
$script:OverlayConfig = Join-Path $script:RepoRoot 'windows\chezmoi-overlay.toml'
$script:Chezmoi = $null

$script:UserProfileGuid = '{11111111-1111-1111-1111-111111111111}'
$script:UserSchemeName = 'UserSeedScheme'
$script:UserActionKeys = 'alt+f4'

function Pass {
    param([Parameter(Mandatory)] [string]$Message)
    Write-Host "PASS: $Message"
}

function Assert-Condition {
    param(
        [Parameter(Mandatory)] [bool]$Condition,
        [Parameter(Mandatory)] [string]$Message
    )
    if (-not $Condition) {
        throw $Message
    }
}

function New-TestSandbox {
    param([Parameter(Mandatory)] [string]$Name)
    $sandbox = Join-Path ([IO.Path]::GetTempPath()) ("dotfiles-chezmoi-{0}-{1}" -f $Name, [guid]::NewGuid())
    New-Item -ItemType Directory -Force -Path $sandbox | Out-Null
    return $sandbox
}

function Remove-TestSandbox {
    param([Parameter(Mandatory)] [string]$Sandbox)
    if (Test-Path -LiteralPath $Sandbox) {
        Remove-Item -LiteralPath $Sandbox -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Get-WtSettingsPath {
    param([Parameter(Mandatory)] [string]$Sandbox)
    $null = $Sandbox
    return (Join-Path $env:LOCALAPPDATA 'Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json')
}

function Invoke-WithSandboxEnv {
    param(
        [Parameter(Mandatory)] [string]$Sandbox,
        [Parameter(Mandatory)] [scriptblock]$Script
    )

    $localAppData = Join-Path $Sandbox 'Redirected Local AppData'
    $appData = Join-Path $Sandbox 'AppData\Roaming'
    $tempDir = Join-Path $Sandbox 'Temp'
    $documents = Join-Path $Sandbox 'OneDrive - Example\Documents'
    $profilePath = Join-Path $documents 'PowerShell\Microsoft.PowerShell_profile.ps1'
    foreach ($dir in @($localAppData, $appData, $tempDir, (Split-Path -Parent $profilePath))) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }

    $envNames = @('USERPROFILE', 'HOME', 'LOCALAPPDATA', 'APPDATA', 'TEMP', 'TMP')
    $oldEnv = @{}
    foreach ($name in $envNames) {
        $oldEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    }
    $oldProfile = (Get-Variable -Name PROFILE -Scope Global -ErrorAction SilentlyContinue).Value

    try {
        $env:USERPROFILE = $Sandbox
        $env:HOME = $Sandbox
        $env:LOCALAPPDATA = $localAppData
        $env:APPDATA = $appData
        $env:TEMP = $tempDir
        $env:TMP = $tempDir
        Set-Variable -Name PROFILE -Scope Global -Value $profilePath -Force
        & $Script
    } finally {
        foreach ($name in $envNames) {
            if ($null -eq $oldEnv[$name]) {
                [Environment]::SetEnvironmentVariable($name, $null, 'Process')
            } else {
                [Environment]::SetEnvironmentVariable($name, $oldEnv[$name], 'Process')
            }
        }
        if ($null -ne $oldProfile) {
            Set-Variable -Name PROFILE -Scope Global -Value $oldProfile -Force
        }
    }
}

function Invoke-CheckedNative {
    param(
        [Parameter(Mandatory)] [string]$FilePath,
        [Parameter(Mandatory)] [string[]]$Arguments
    )
    & $FilePath @Arguments
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        throw "$FilePath $($Arguments -join ' ') exited $exitCode"
    }
}

function Invoke-Chezmoi {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    # --no-tty + --force: CI has no interactive tty, so any chezmoi prompt
    # ("X has changed since chezmoi last wrote it? diff/overwrite/skip/quit")
    # would block forever (observed: a 40-minute hang on the nvim dir-symlink).
    # --force makes every change without prompting; --no-tty refuses to grab a
    # TTY. verify ignores both (it makes no changes), so it stays a strict
    # oracle: if the nvim symlink ever fails to round-trip, verify still fails.
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@(
            '--source', $script:SourceDir,
            '--destination', $env:USERPROFILE,
            '--no-tty', '--force'
        ) + $Arguments)
}

function Invoke-ChezmoiReapply {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    # Idempotency / second-apply check: --no-tty but deliberately NO --force. The
    # first apply uses --force (a pre-existing seeded target may legitimately need
    # an overwrite); the re-apply must be a clean no-op. Without --force, an
    # unexpected prompt aborts non-interactively (nonzero exit) instead of being
    # silently overwritten -> a prompt-on-reapply regression fails the test fast
    # rather than being masked. This is the strict idempotency oracle the Unix
    # parity gate already has (it captures second-apply output and fails on
    # non-empty); this keeps the Windows arm honest too.
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@(
            '--source', $script:SourceDir,
            '--destination', $env:USERPROFILE,
            '--no-tty'
        ) + $Arguments)
}

function Invoke-ChezmoiOverlay {
    param(
        [Parameter(Mandatory)] [string]$Source,
        [Parameter(Mandatory)] [string]$Destination,
        [Parameter(Mandatory)] [string]$StateName,
        [Parameter(Mandatory)] [string[]]$Arguments,
        [switch]$Reapply
    )
    $state = Join-Path $env:TEMP "$StateName.boltdb"
    $base = @(
        '--source', $Source,
        '--destination', $Destination,
        '--persistent-state', $state,
        '--config', $script:OverlayConfig,
        '--config-format', 'toml',
        '--no-tty'
    )
    if (-not $Reapply) { $base += '--force' }
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments ($base + $Arguments)
}

function Get-SingleItemTarget {
    param([Parameter(Mandatory)] $Item)
    $target = $Item.Target
    if ($target -is [array]) {
        return $target[0]
    }
    return $target
}

function Get-CanonicalPath {
    param([Parameter(Mandatory)] [string]$Path)
    return (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
}

function Assert-FileContentMatches {
    param(
        [Parameter(Mandatory)] [string]$ActualPath,
        [Parameter(Mandatory)] [string]$ExpectedPath,
        [Parameter(Mandatory)] [string]$Label
    )

    Assert-Condition (Test-Path -LiteralPath $ActualPath -PathType Leaf) "$Label was not created"
    Assert-Condition (Test-Path -LiteralPath $ExpectedPath -PathType Leaf) "$Label expected source is missing: $ExpectedPath"
    $actualHash = (Get-FileHash -LiteralPath $ActualPath -Algorithm SHA256).Hash
    $expectedHash = (Get-FileHash -LiteralPath $ExpectedPath -Algorithm SHA256).Hash
    Assert-Condition ($actualHash -eq $expectedHash) "$Label content mismatch actual=$actualHash expected=$expectedHash"
}

function Assert-CopyModeFileMatches {
    param(
        [Parameter(Mandatory)] [string]$ActualPath,
        [Parameter(Mandatory)] [string]$ExpectedPath,
        [Parameter(Mandatory)] [string]$Label
    )

    Assert-FileContentMatches -ActualPath $ActualPath -ExpectedPath $ExpectedPath -Label $Label
    $item = Get-Item -LiteralPath $ActualPath -Force
    $linkType = if ($item.PSObject.Properties.Name -contains 'LinkType') { $item.LinkType } else { $null }
    Assert-Condition ($linkType -ne 'SymbolicLink') "$Label is a symlink; expected Windows copy mode"
}

function Assert-NvimSymlinkMatchesRepo {
    param([Parameter(Mandatory)] [string]$Sandbox)

    $null = $Sandbox
    $nvimPath = Join-Path $env:LOCALAPPDATA 'nvim'
    Assert-Condition (Test-Path -LiteralPath $nvimPath -PathType Container) 'nvim directory was not created under actual LocalApplicationData'
    $nvimItem = Get-Item -LiteralPath $nvimPath -Force
    Assert-Condition ($nvimItem.LinkType -eq 'SymbolicLink') 'nvim is not a symlink; expected Windows dir-symlink mode'

    $target = Get-SingleItemTarget -Item $nvimItem
    Assert-Condition ([string]::IsNullOrWhiteSpace($target) -eq $false) 'nvim symlink has no target'
    $resolvedTarget = Get-CanonicalPath -Path $target
    $repoNvim = Get-CanonicalPath -Path (Join-Path $script:RepoRoot 'nvim')
    Assert-Condition ($resolvedTarget -eq $repoNvim) "nvim symlink target mismatch actual=$resolvedTarget expected=$repoNvim"

    Assert-FileContentMatches `
        -ActualPath (Join-Path $nvimPath 'init.lua') `
        -ExpectedPath (Join-Path $script:RepoRoot 'nvim\init.lua') `
        -Label 'nvim init.lua'
}

function Assert-SymlinkMatchesRepo {
    param(
        [Parameter(Mandatory)] [string]$ActualPath,
        [Parameter(Mandatory)] [string]$ExpectedPath,
        [Parameter(Mandatory)] [string]$Label
    )
    $item = Get-Item -LiteralPath $ActualPath -Force -ErrorAction Stop
    Assert-Condition ($item.LinkType -eq 'SymbolicLink') "$Label is not a symlink"
    $resolvedTarget = Get-CanonicalPath -Path (Get-SingleItemTarget -Item $item)
    $expectedTarget = Get-CanonicalPath -Path $ExpectedPath
    Assert-Condition ($resolvedTarget -eq $expectedTarget) "$Label target mismatch actual=$resolvedTarget expected=$expectedTarget"
}

function New-BaselineWtSettings {
    return [ordered]@{
        defaultProfile = $script:UserProfileGuid
        theme = 'legacyLight'
        profiles = [ordered]@{
            defaults = [ordered]@{
                colorScheme = $script:UserSchemeName
                font = [ordered]@{
                    face = 'Consolas'
                }
            }
            list = @(
                [ordered]@{
                    guid = $script:UserProfileGuid
                    name = 'Seeded User Profile'
                    commandline = 'powershell.exe'
                    colorScheme = $script:UserSchemeName
                }
            )
        }
        schemes = @(
            [ordered]@{
                name = $script:UserSchemeName
                foreground = '#ffffff'
                background = '#000000'
            }
        )
        actions = @(
            [ordered]@{
                command = 'closeWindow'
                keys = $script:UserActionKeys
            }
        )
    }
}

function Write-BaselineWtSettings {
    param([Parameter(Mandatory)] [string]$Sandbox)
    $settingsPath = Get-WtSettingsPath -Sandbox $Sandbox
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settingsPath) | Out-Null
    New-BaselineWtSettings |
        ConvertTo-Json -Depth 100 |
        Set-Content -LiteralPath $settingsPath -Encoding utf8
    return $settingsPath
}

function Assert-Part1Files {
    param([Parameter(Mandatory)] [string]$Sandbox)

    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\tmux.conf') `
        -Label '~/.tmux.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.psmux.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\psmux.conf') `
        -Label '~/.psmux.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.windows.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\tmux.windows.conf') `
        -Label '~/.tmux.windows.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.rose-pine.ps1') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\psmux-rose-pine.ps1') `
        -Label '~/.tmux.rose-pine.ps1'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.rose-pine.main.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\psmux-rose-pine.main.conf') `
        -Label '~/.tmux.rose-pine.main.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.rose-pine.moon.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\psmux-rose-pine.moon.conf') `
        -Label '~/.tmux.rose-pine.moon.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.rose-pine.dawn.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\psmux-rose-pine.dawn.conf') `
        -Label '~/.tmux.rose-pine.dawn.conf'
    # psmux freeze boundary: the POSIX-only clipboard overlay carries the
    # `if-shell` probes that hang psmux at config-load time. It MUST NOT be
    # deployed on Windows (home/.chezmoiignore ignores it). Assert its absence so
    # a regression in the ignore rule can never silently reintroduce the freeze.
    Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $Sandbox '.tmux.posix.conf'))) `
        '~/.tmux.posix.conf must NOT be deployed on Windows (psmux config-load freeze boundary)'
    Assert-SymlinkMatchesRepo `
        -ActualPath (Join-Path $env:LOCALAPPDATA 'lazygit\config.yml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'lazygit\config.windows.yml') `
        -Label 'lazygit config'
    Assert-SymlinkMatchesRepo `
        -ActualPath (Join-Path $env:APPDATA 'herdr\config.toml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'herdr\config.toml') `
        -Label 'Herdr config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\starship.toml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'starship\starship.toml') `
        -Label 'starship config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\gh-dash\config.yml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'gh-dash\config.yml') `
        -Label 'gh-dash config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\lsd\config.yaml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'lsd\config.yaml') `
        -Label 'lsd config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\lsd\colors.yaml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'lsd\colors.yaml') `
        -Label 'lsd colors'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\wezterm\wezterm.lua') `
        -ExpectedPath (Join-Path $script:RepoRoot 'wezterm\wezterm.lua') `
        -Label 'wezterm config'
    Assert-SymlinkMatchesRepo `
        -ActualPath ([string]$PROFILE) `
        -ExpectedPath (Join-Path $script:RepoRoot 'shells\powershell_profile.ps1') `
        -Label 'PowerShell profile'
    Assert-NvimSymlinkMatchesRepo -Sandbox $Sandbox
    Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $Sandbox 'AppData\Local\nvim'))) `
        'conventional LocalAppData target was created despite redirection'
    Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $Sandbox '.config\herdr\config.toml'))) `
        'POSIX Herdr config path was created on Windows'
    Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $Sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'))) `
        'conventional Documents profile was created despite redirection'
}

function Invoke-Part1 {
    $sandbox = New-TestSandbox -Name 'part1'
    try {
        Invoke-WithSandboxEnv -Sandbox $sandbox -Script {
            $settingsPath = Write-BaselineWtSettings -Sandbox $sandbox
            $settingsBefore = [IO.File]::ReadAllText($settingsPath)
            Invoke-Chezmoi -Arguments @('init')
            Invoke-Chezmoi -Arguments @('apply')
            Invoke-ChezmoiOverlay -Source $script:LocalAppDataSource -Destination $env:LOCALAPPDATA -StateName 'localappdata' -Arguments @('apply')
            Invoke-ChezmoiOverlay -Source $script:AppDataSource -Destination $env:APPDATA -StateName 'appdata' -Arguments @('apply')
            Invoke-ChezmoiOverlay -Source $script:DocumentsSource -Destination (Split-Path -Parent (Split-Path -Parent ([string]$PROFILE))) -StateName 'documents' -Arguments @('apply')
            Assert-Part1Files -Sandbox $sandbox
            Assert-Condition ([IO.File]::ReadAllText($settingsPath) -eq $settingsBefore) `
                'bare chezmoi apply changed Windows Terminal settings; setup.ps1 must own the transaction'
            # Second apply must be a prompt-free no-op (NO --force; see wrapper).
            Invoke-ChezmoiReapply -Arguments @('apply')
            Invoke-ChezmoiOverlay -Source $script:LocalAppDataSource -Destination $env:LOCALAPPDATA -StateName 'localappdata' -Arguments @('apply') -Reapply
            Invoke-ChezmoiOverlay -Source $script:AppDataSource -Destination $env:APPDATA -StateName 'appdata' -Arguments @('apply') -Reapply
            Invoke-ChezmoiOverlay -Source $script:DocumentsSource -Destination (Split-Path -Parent (Split-Path -Parent ([string]$PROFILE))) -StateName 'documents' -Arguments @('apply') -Reapply
            Invoke-Chezmoi -Arguments @('verify')
            Invoke-ChezmoiOverlay -Source $script:LocalAppDataSource -Destination $env:LOCALAPPDATA -StateName 'localappdata' -Arguments @('verify') -Reapply
            Invoke-ChezmoiOverlay -Source $script:AppDataSource -Destination $env:APPDATA -StateName 'appdata' -Arguments @('verify') -Reapply
            Invoke-ChezmoiOverlay -Source $script:DocumentsSource -Destination (Split-Path -Parent (Split-Path -Parent ([string]$PROFILE))) -StateName 'documents' -Arguments @('verify') -Reapply
        }
        Pass 'part 1 real apply smoke passed'
    } finally {
        Remove-TestSandbox -Sandbox $sandbox
    }
}

try {
    $script:Chezmoi = (Get-Command chezmoi -ErrorAction Stop).Source
    Assert-Condition ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) 'windows_apply_test.ps1 must run on Windows'
    Invoke-Part1
    Pass 'windows_apply_test.ps1 completed'
} catch {
    Write-Host "FAIL: $($_.Exception.Message)"
    if ($_.ScriptStackTrace) {
        Write-Host $_.ScriptStackTrace
    }
    exit 1
}
