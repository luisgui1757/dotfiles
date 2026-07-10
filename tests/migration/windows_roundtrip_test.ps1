$ErrorActionPreference = 'Stop'
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$script:RepoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$script:SourceDir = Join-Path $script:RepoRoot 'home'
$script:LocalAppDataSource = Join-Path $script:RepoRoot 'windows\chezmoi-localappdata'
$script:DocumentsSource = Join-Path $script:RepoRoot 'windows\chezmoi-documents'
$script:OverlayConfig = Join-Path $script:RepoRoot 'windows\chezmoi-overlay.toml'
$script:UninstallPath = Join-Path $script:RepoRoot 'uninstall.ps1'
$script:Chezmoi = $null

function Pass {
    param([Parameter(Mandatory)] [string]$Message)
    Write-Host "PASS: $Message"
}

function Assert-Condition {
    param(
        [Parameter(Mandatory)] [bool]$Condition,
        [Parameter(Mandatory)] [string]$Message
    )
    if (-not $Condition) { throw $Message }
}

function New-TestSandbox {
    $sandbox = Join-Path ([IO.Path]::GetTempPath()) ("dotfiles-chezmoi-roundtrip-{0}" -f [guid]::NewGuid())
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
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@(
            '--source', $script:SourceDir,
            '--destination', $env:USERPROFILE,
            '--no-tty', '--force'
        ) + $Arguments)
}

function Invoke-ChezmoiOverlay {
    param(
        [Parameter(Mandatory)] [string]$Source,
        [Parameter(Mandatory)] [string]$Destination,
        [Parameter(Mandatory)] [string]$StateName,
        [Parameter(Mandatory)] [string[]]$Arguments
    )
    $stateRoot = Join-Path $env:LOCALAPPDATA 'dotfiles\chezmoi-state'
    New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@(
            '--source', $Source,
            '--destination', $Destination,
            '--persistent-state', (Join-Path $stateRoot "$StateName.boltdb"),
            '--config', $script:OverlayConfig,
            '--config-format', 'toml',
            '--no-tty', '--force'
        ) + $Arguments)
}

function Assert-FileContent {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [Parameter(Mandatory)] [string]$Expected,
        [Parameter(Mandatory)] [string]$Label
    )
    Assert-Condition (Test-Path -LiteralPath $Path -PathType Leaf) "$Label missing: $Path"
    $actual = (Get-Content -Raw -LiteralPath $Path).TrimEnd("`r", "`n")
    Assert-Condition ($actual -eq $Expected) "$Label content mismatch actual=[$actual] expected=[$Expected]"
}

function Assert-CopyModeFilePresent {
    param([Parameter(Mandatory)] [string]$Path)
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    $linkType = if ($item.PSObject.Properties.Name -contains 'LinkType') { $item.LinkType } else { $null }
    Assert-Condition ($linkType -ne 'SymbolicLink') "expected copy-mode file, got symlink: $Path"
}

function Assert-NvimSymlinkPresent {
    param([Parameter(Mandatory)] [string]$Sandbox)
    $null = $Sandbox
    $nvimPath = Join-Path $env:LOCALAPPDATA 'nvim'
    $item = Get-Item -LiteralPath $nvimPath -Force -ErrorAction Stop
    Assert-Condition ($item.LinkType -eq 'SymbolicLink') 'nvim is not a directory symlink after apply'
}

try {
    if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
        Pass 'windows_roundtrip_test.ps1 skipped on non-Windows host'
        exit 0
    }

    $script:Chezmoi = (Get-Command chezmoi -ErrorAction Stop).Source
    $sandbox = New-TestSandbox
    try {
        Invoke-WithSandboxEnv -Sandbox $sandbox -Script {
            $preseed = 'user tmux config from before chezmoi'
            $tmuxPath = Join-Path $sandbox '.tmux.conf'
            Set-Content -LiteralPath $tmuxPath -Value $preseed -Encoding utf8
            Copy-Item -LiteralPath $tmuxPath -Destination "$tmuxPath.bak.20000101-000000" -Force

            $wtSettings = Get-WtSettingsPath -Sandbox $sandbox
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $wtSettings) | Out-Null
            $wtOriginal = '{"profiles":{"defaults":{},"list":[]},"actions":[],"schemes":[],"themes":[]}'
            $wtOriginal | Set-Content -LiteralPath $wtSettings -Encoding utf8
            Copy-Item -LiteralPath $wtSettings -Destination "$wtSettings.bak.20000101-000000" -Force
            Pass 'pre-seeded Windows copy target and WT backup'

            Invoke-Chezmoi -Arguments @('init')
            Invoke-Chezmoi -Arguments @('apply')
            Invoke-ChezmoiOverlay -Source $script:LocalAppDataSource -Destination $env:LOCALAPPDATA -StateName 'localappdata' -Arguments @('apply')
            Invoke-ChezmoiOverlay -Source $script:DocumentsSource -Destination (Split-Path -Parent (Split-Path -Parent ([string]$PROFILE))) -StateName 'documents' -Arguments @('apply')
            Pass 'chezmoi apply completed'

            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.psmux.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.windows.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.rose-pine.ps1')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.rose-pine.main.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.rose-pine.moon.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.rose-pine.dawn.conf')
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.tmux.posix.conf'))) `
                '~/.tmux.posix.conf must NOT be deployed on Windows (psmux config-load freeze boundary)'
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.config\starship.toml')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.config\lsd\config.yaml')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.config\lsd\colors.yaml')
            Assert-Condition ((Get-Item -LiteralPath ([string]$PROFILE) -Force).LinkType -eq 'SymbolicLink') 'runtime PowerShell profile is not a symlink'
            Assert-Condition ((Get-Item -LiteralPath (Join-Path $env:LOCALAPPDATA 'lazygit\config.yml') -Force).LinkType -eq 'SymbolicLink') 'lazygit actual config is not a symlink'
            Assert-NvimSymlinkPresent -Sandbox $sandbox
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'))) 'conventional Documents profile was created'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox 'AppData\Local\nvim'))) 'conventional LocalAppData nvim was created'
            Pass 'managed Windows entries present after apply'

            # setup.ps1, not bare chezmoi, owns the WT transaction. Model its
            # post-merge current state so uninstall must restore the independent
            # pre-setup backup while preserving these current bytes.
            '{"theme":"rose-pine","profiles":{"defaults":{},"list":[]},"actions":[],"schemes":[],"themes":[]}' |
                Set-Content -LiteralPath $wtSettings -Encoding utf8

            # Adversarial: a USER-MODIFIED copy must be preserved. uninstall.ps1
            # removes a Windows copy only when `chezmoi verify` says it still
            # matches managed state; edit one so verify reports drift -> skip.
            $tmuxWin = Join-Path $sandbox '.tmux.windows.conf'
            Add-Content -LiteralPath $tmuxWin -Value '# user local edit'
            $tmuxWinExpected = Get-Content -Raw -LiteralPath $tmuxWin

            $oldSourceOnly = $env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY
            try {
                $env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY = '1'
                . $script:UninstallPath -All
            } finally {
                if ($null -eq $oldSourceOnly) { Remove-Item Env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue }
                else { $env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY = $oldSourceOnly }
            }
            $identity = [pscustomobject]@{
                UserProfile = $sandbox
                LocalApplicationData = $env:LOCALAPPDATA
                Documents = Split-Path -Parent (Split-Path -Parent ([string]$PROFILE))
                RuntimeProfile = [string]$PROFILE
            }
            Invoke-DotfilesUninstall -Identity $identity
            Pass 'uninstall.ps1 -All completed'

            Assert-FileContent -Path $tmuxPath -Expected $preseed -Label 'restored .tmux.conf'
            Assert-Condition (Test-Path -LiteralPath $tmuxWin) 'user-modified .tmux.windows.conf was deleted (data loss)'
            Assert-Condition ((Get-Content -Raw -LiteralPath $tmuxWin) -eq $tmuxWinExpected) 'user-modified .tmux.windows.conf content changed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.psmux.conf'))) 'psmux entrypoint config was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.tmux.rose-pine.main.conf'))) 'psmux rose-pine main config was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.tmux.rose-pine.moon.conf'))) 'psmux rose-pine moon config was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.tmux.rose-pine.dawn.conf'))) 'psmux rose-pine dawn config was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.config\starship.toml'))) 'starship copy was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.config\lsd\config.yaml'))) 'lsd config copy was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.config\lsd\colors.yaml'))) 'lsd colors copy was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath ([string]$PROFILE))) 'PowerShell profile symlink was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $env:LOCALAPPDATA 'nvim'))) 'nvim symlink was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $env:LOCALAPPDATA 'lazygit\config.yml'))) 'lazygit symlink was not removed'
            Assert-Condition (Test-Path -LiteralPath $wtSettings) 'WT settings.json should not be deleted'
            Assert-Condition (-not (Test-Path -LiteralPath "$wtSettings.bak.20000101-000000")) 'selected WT backup should be consumed by restoration'
            Assert-Condition ((Get-Content -Raw -LiteralPath $wtSettings).TrimEnd("`r", "`n") -eq $wtOriginal) 'WT pre-setup backup was not restored'
            $preservedWt = @(Get-ChildItem -LiteralPath (Split-Path -Parent $wtSettings) -Filter 'settings.json.uninstall-current.*')
            Assert-Condition ($preservedWt.Count -eq 1) 'pre-uninstall WT settings were not preserved independently'
            Assert-Condition ((Get-Content -Raw -LiteralPath $preservedWt[0].FullName) -match 'rose-pine') 'preserved pre-uninstall WT settings do not contain the managed merge'
            Pass 'managed entries removed and WT backup restored without discarding current settings'

            # *>&1 (not 2>&1): uninstall.ps1 prints "nothing to remove" via
            # Write-Host (information stream 6); 2>&1 only merges the error stream,
            # so the no-op marker would not be captured.
            $script:Removed = 0
            $script:Restored = 0
            $script:DirsRemoved = 0
            $script:ExternalsRemoved = 0
            $secondOutput = Invoke-DotfilesUninstall -Identity $identity *>&1 | Out-String
            Assert-Condition ($secondOutput -match 'nothing to remove') 'second uninstall did not report no-op'
            Assert-FileContent -Path $tmuxPath -Expected $preseed -Label 'restored .tmux.conf after second run'
            Pass 'second uninstall is idempotent'
        }
    } finally {
        Remove-TestSandbox -Sandbox $sandbox
    }
    Pass 'windows_roundtrip_test.ps1 completed'
} catch {
    Write-Host "FAIL: $($_.Exception.Message)"
    if ($_.ScriptStackTrace) {
        Write-Host $_.ScriptStackTrace
    }
    exit 1
}
