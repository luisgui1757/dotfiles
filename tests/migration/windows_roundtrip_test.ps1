$ErrorActionPreference = 'Stop'
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$script:RepoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$script:SourceDir = Join-Path $script:RepoRoot 'home'
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
    return (Join-Path $Sandbox 'AppData\Local\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json')
}

function Invoke-WithSandboxEnv {
    param(
        [Parameter(Mandatory)] [string]$Sandbox,
        [Parameter(Mandatory)] [scriptblock]$Script
    )

    $localAppData = Join-Path $Sandbox 'AppData\Local'
    $appData = Join-Path $Sandbox 'AppData\Roaming'
    $tempDir = Join-Path $Sandbox 'Temp'
    $profilePath = Join-Path $Sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
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
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@('--source', $script:SourceDir, '--no-tty', '--force') + $Arguments)
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
    $nvimPath = Join-Path $Sandbox 'AppData\Local\nvim'
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
            '{"profiles":{"defaults":{},"list":[]},"actions":[],"schemes":[],"themes":[]}' |
                Set-Content -LiteralPath $wtSettings -Encoding utf8
            Copy-Item -LiteralPath $wtSettings -Destination "$wtSettings.bak.20000101-000000" -Force
            Pass 'pre-seeded Windows copy target and WT backup'

            Invoke-Chezmoi -Arguments @('init')
            Invoke-Chezmoi -Arguments @('apply')
            Pass 'chezmoi apply completed'

            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.tmux.windows.conf')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox '.config\starship.toml')
            Assert-CopyModeFilePresent -Path (Join-Path $sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1')
            Assert-NvimSymlinkPresent -Sandbox $sandbox
            Pass 'managed Windows entries present after apply'

            # Adversarial: a USER-MODIFIED copy must be preserved. uninstall.ps1
            # removes a Windows copy only when `chezmoi verify` says it still
            # matches managed state; edit one so verify reports drift -> skip.
            $tmuxWin = Join-Path $sandbox '.tmux.windows.conf'
            Add-Content -LiteralPath $tmuxWin -Value '# user local edit'
            $tmuxWinExpected = Get-Content -Raw -LiteralPath $tmuxWin

            & $script:UninstallPath -All
            if ($LASTEXITCODE -ne 0) { throw "uninstall.ps1 exited $LASTEXITCODE" }
            Pass 'uninstall.ps1 -All completed'

            Assert-FileContent -Path $tmuxPath -Expected $preseed -Label 'restored .tmux.conf'
            Assert-Condition (Test-Path -LiteralPath $tmuxWin) 'user-modified .tmux.windows.conf was deleted (data loss)'
            Assert-Condition ((Get-Content -Raw -LiteralPath $tmuxWin) -eq $tmuxWinExpected) 'user-modified .tmux.windows.conf content changed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox '.config\starship.toml'))) 'starship copy was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'))) 'PowerShell profile copy was not removed'
            Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $sandbox 'AppData\Local\nvim'))) 'nvim symlink was not removed'
            Assert-Condition (Test-Path -LiteralPath $wtSettings) 'WT settings.json should not be deleted'
            Assert-Condition (Test-Path -LiteralPath "$wtSettings.bak.20000101-000000") 'WT settings backup should remain available'
            Pass 'managed entries removed, backup restored, WT left intact'

            # *>&1 (not 2>&1): uninstall.ps1 prints "nothing to remove" via
            # Write-Host (information stream 6); 2>&1 only merges the error stream,
            # so the no-op marker would not be captured.
            $secondOutput = & $script:UninstallPath -All *>&1 | Out-String
            if ($LASTEXITCODE -ne 0) { throw "second uninstall.ps1 exited $LASTEXITCODE" }
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
