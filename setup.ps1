# setup.ps1 -- one-shot end-to-end install for Windows.
#
# Local usage (from a checked-out copy):
#   .\setup.ps1                  interactive: Y/n per dep, then config + sync
#   .\setup.ps1 -All             non-interactive: install everything missing
#   .\setup.ps1 -DryRun          preview every step
#   .\setup.ps1 -SkipDeps        already have nvim/starship; just config+sync
#   .\setup.ps1 -SkipBootstrap   back-compat alias: skip config apply
#   .\setup.ps1 -SkipConfig      already configured; just sync plugins+LSP
#   .\setup.ps1 -SkipNvim        skip nvim plugin + Mason sync
#   .\setup.ps1 -SkipWindowsTerminalMerge   config+sync but leave WT settings.json untouched
#   .\setup.ps1 -MergeWindowsTerminal        (no-op alias; the WT rose-pine merge is now default-on)
#
# Remote usage (no checkout yet):
#   iwr https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.ps1 -OutFile setup.ps1
#   .\setup.ps1 -All
#
# The remote form clones the repo to $env:DOTFILES_DEST (default
# %USERPROFILE%\dotfiles) and re-invokes itself locally.

[CmdletBinding()]
param(
    [switch]$All,
    [switch]$DryRun,
    [switch]$SkipDeps,
    [switch]$SkipBootstrap,
    [switch]$SkipConfig,
    [switch]$SkipNvim,
    [switch]$MergeWindowsTerminal,   # back-compat no-op: WT merge is now default-on
    [switch]$SkipWindowsTerminalMerge,
    [switch]$BestEffort
)

$ErrorActionPreference = 'Stop'

$RepoUrl     = 'https://github.com/luisgui1757/dotfiles.git'
$DefaultDest = Join-Path $env:USERPROFILE 'dotfiles'

# Rebuild PATH from registry values plus Scoop shims, then de-duplicate.
# This differs from setup.sh, which evaluates brew shellenv and appends Unix bin dirs.
function Update-RuntimePath {
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

$inputRedirected = $false
$outputRedirected = $false
try { $inputRedirected = [Console]::IsInputRedirected } catch { $inputRedirected = $true }
try { $outputRedirected = [Console]::IsOutputRedirected } catch { $outputRedirected = $true }
if ((-not [Environment]::UserInteractive -or $inputRedirected -or $outputRedirected) -and (-not $All) -and (-not $DryRun)) {
    Write-Host "note: no TTY detected; running with -All"
    $All = $true
    $PSBoundParameters['All'] = $true
}

# ---- Locate / clone the repo -------------------------------------------------
# When piped from `irm | iex` there is no $PSCommandPath, so we clone and
# re-invoke from the clone.
$ScriptDir = $null
if ($PSCommandPath -and (Test-Path -LiteralPath $PSCommandPath)) {
    $ScriptDir = Split-Path -Parent $PSCommandPath
}
if (-not $ScriptDir -or -not (Test-Path (Join-Path $ScriptDir 'home'))) {
    $dest = if ($env:DOTFILES_DEST) { $env:DOTFILES_DEST } else { $DefaultDest }
    # DryRun honor: announce what we would clone and exit BEFORE any git op.
    if ($DryRun) {
        Write-Host "setup.ps1 (remote, dry-run): would clone $RepoUrl -> $dest"
        Write-Host "                             then re-invoke .\setup.ps1 from there."
        Write-Host "(dry run -- no clone, no install, no writes performed)"
        exit 0
    }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Error "setup.ps1: git is the only prerequisite for remote setup, and it is required to clone the repo. Install git first: winget install Git.Git"
        exit 1
    }
    if (Test-Path (Join-Path $dest '.git')) {
        Write-Host "Repo already cloned at $dest. Pulling latest."
        git -C $dest pull --ff-only
    } else {
        Write-Host "Cloning $RepoUrl -> $dest"
        git clone $RepoUrl $dest
    }
    Write-Host ""
    Write-Host "Re-invoking setup.ps1 from the clone."
    & (Join-Path $dest 'setup.ps1') @PSBoundParameters
    exit $LASTEXITCODE
}

Set-Location $ScriptDir

# ---- Forward flags to sub-scripts --------------------------------------------
# Hashtable splatting (not array) so switches bind by NAME. Array splatting
# passes elements as positional args, which switch parameters cannot accept;
# that path produced a positional-parameter error the moment Phase 1
# invoked install-deps.ps1 with -All in CI.
$depsArgs = @{}
if ($All)    { $depsArgs['All']    = $true }
if ($DryRun) { $depsArgs['DryRun'] = $true }

# WT settings merge is now a DEFAULT config step (opt-out, not opt-in).
# -MergeWindowsTerminal is retained as a harmless no-op alias for back-compat.
$null = $MergeWindowsTerminal  # reference the alias so PSScriptAnalyzer doesn't flag it unused

$HomeSource = Join-Path $ScriptDir 'home'
$script:ChezmoiBaseArgs = @('--source', $HomeSource)
$script:ChezmoiConfigArgs = @()
$Timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'

function Phase {
    param([string]$title)
    Write-Host ""
    Write-Host "================================================================"
    Write-Host "==  $title"
    Write-Host "================================================================"
}

function Write-Step {
    param([string]$Message)
    Write-Host "  $Message"
}

function Get-UniqueBackupPath {
    param([Parameter(Mandatory)] [string]$Base)
    if (-not (Test-Path -LiteralPath $Base)) { return $Base }
    $i = 1
    while (Test-Path -LiteralPath "$Base.$i") { $i++ }
    return "$Base.$i"
}

function New-NativeSymLink {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string]$Source,
        [Parameter(Mandatory)] [string]$Destination
    )

    if (-not ('DotfilesSetupSymbolicLink' -as [type])) {
        Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

public static class DotfilesSetupSymbolicLink {
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern bool CreateSymbolicLink(
        string lpSymlinkFileName,
        string lpTargetFileName,
        int dwFlags
    );
}
"@
    }

    $sourceItem = Get-Item -LiteralPath $Source -Force -ErrorAction Stop
    $flags = 0x2
    if ($sourceItem.PSIsContainer) { $flags = $flags -bor 0x1 }

    $ok = [DotfilesSetupSymbolicLink]::CreateSymbolicLink($Destination, $Source, $flags)
    if (-not $ok) {
        $code = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
        throw (New-Object ComponentModel.Win32Exception($code))
    }
}

function New-SymbolicLinkItem {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string]$Source,
        [Parameter(Mandatory)] [string]$Destination
    )

    try {
        New-Item -ItemType SymbolicLink -Path $Destination -Target $Source -ErrorAction Stop | Out-Null
    } catch {
        $newItemError = $_
        if ($env:OS -ne 'Windows_NT') { throw }
        try {
            New-NativeSymLink -Source $Source -Destination $Destination
        } catch {
            throw @"
failed to create symlink:
  destination: $Destination
  source:      $Source
  New-Item:    $($newItemError.Exception.Message)
  native API:  $($_.Exception.Message)
"@
        }
    }
}

function Test-CanCreateSymlinks {
    try {
        $tmp = Join-Path $env:TEMP "symlink-probe-$([guid]::NewGuid())"
        $target = Join-Path $env:TEMP "symlink-probe-target-$([guid]::NewGuid())"
        New-Item -ItemType File -Path $target -Force | Out-Null
        New-SymbolicLinkItem -Source $target -Destination $tmp
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        return $true
    } catch {
        return $false
    }
}

function Test-IsElevated {
    try {
        $id = [Security.Principal.WindowsIdentity]::GetCurrent()
        return ([Security.Principal.WindowsPrincipal]$id).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    } catch { return $false }
}

function Test-DevModeOn {
    try {
        $k = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock'
        return (((Get-ItemProperty -Path $k -Name AllowDevelopmentWithoutDevLicense -ErrorAction Stop).AllowDevelopmentWithoutDevLicense) -eq 1)
    } catch { return $false }
}

function Invoke-ChezmoiOrExit {
    param(
        [Parameter(Mandatory)] [string]$Label,
        [Parameter(Mandatory)] [string[]]$Arguments
    )
    $global:LASTEXITCODE = 0
    $baseArgs = $script:ChezmoiBaseArgs
    $configArgs = $script:ChezmoiConfigArgs
    & chezmoi @baseArgs @configArgs @Arguments
    $rc = $LASTEXITCODE
    if ($rc -ne 0) {
        Write-Host ("  FAIL: $Label exited $rc") -ForegroundColor Red
        exit $rc
    }
}

function Invoke-ChezmoiOutput {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    $global:LASTEXITCODE = 0
    $baseArgs = $script:ChezmoiBaseArgs
    $configArgs = $script:ChezmoiConfigArgs
    $output = & chezmoi @baseArgs @configArgs @Arguments
    $rc = $LASTEXITCODE
    if ($rc -ne 0) {
        throw "chezmoi $($Arguments -join ' ') exited $rc"
    }
    return @($output)
}

function Test-ChezmoiVerify {
    param([Parameter(Mandatory)] [string]$Target)
    $global:LASTEXITCODE = 0
    $baseArgs = $script:ChezmoiBaseArgs
    $configArgs = $script:ChezmoiConfigArgs
    & chezmoi @baseArgs @configArgs verify $Target > $null 2> $null
    return ($LASTEXITCODE -eq 0)
}

function Get-FullPathSafe {
    param([Parameter(Mandatory)] [string]$Path)
    try {
        return [IO.Path]::GetFullPath($Path)
    } catch {
        return $Path
    }
}

function Get-RealExistingPath {
    param([Parameter(Mandatory)] [string]$Path)

    $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    if ($null -eq $item) { return $null }

    $linkType = if ($item.PSObject.Properties.Name -contains 'LinkType') { $item.LinkType } else { $null }
    if ($linkType -eq 'SymbolicLink') {
        $linkTarget = @($item.Target)[0]
        if ($linkTarget) {
            if (-not [IO.Path]::IsPathRooted($linkTarget)) {
                $linkTarget = Join-Path (Split-Path -Parent $Path) $linkTarget
            }
            $resolvedTarget = Resolve-Path -LiteralPath $linkTarget -ErrorAction SilentlyContinue
            if ($resolvedTarget) {
                return (Get-FullPathSafe -Path (@($resolvedTarget)[0].Path))
            }
        }
    }

    $resolved = Resolve-Path -LiteralPath $Path -ErrorAction SilentlyContinue
    if ($resolved) {
        return (Get-FullPathSafe -Path (@($resolved)[0].Path))
    }
    return (Get-FullPathSafe $Path)
}

function Test-FileBytesEqual {
    param(
        [Parameter(Mandatory)] [string]$Left,
        [Parameter(Mandatory)] [string]$Right
    )
    if (-not (Test-Path -LiteralPath $Left -PathType Leaf) -or -not (Test-Path -LiteralPath $Right -PathType Leaf)) {
        return $false
    }
    $leftBytes = [IO.File]::ReadAllBytes($Left)
    $rightBytes = [IO.File]::ReadAllBytes($Right)
    if ($leftBytes.Length -ne $rightBytes.Length) { return $false }
    for ($i = 0; $i -lt $leftBytes.Length; $i++) {
        if ($leftBytes[$i] -ne $rightBytes[$i]) { return $false }
    }
    return $true
}

function ConvertTo-NativeArgument {
    param([string]$Value)

    if ($null -eq $Value) { $Value = '' }
    if ($Value.Length -gt 0 -and $Value -notmatch '[\s"]') { return $Value }

    $builder = New-Object System.Text.StringBuilder
    $backslashes = 0
    [void]$builder.Append('"')
    foreach ($char in $Value.ToCharArray()) {
        if ($char -eq '\') {
            $backslashes++
            continue
        }
        if ($char -eq '"') {
            if ($backslashes -gt 0) {
                [void]$builder.Append(('\' * ($backslashes * 2)))
                $backslashes = 0
            }
            [void]$builder.Append('\"')
            continue
        }
        if ($backslashes -gt 0) {
            [void]$builder.Append(('\' * $backslashes))
            $backslashes = 0
        }
        [void]$builder.Append($char)
    }
    if ($backslashes -gt 0) {
        [void]$builder.Append(('\' * ($backslashes * 2)))
    }
    [void]$builder.Append('"')
    return $builder.ToString()
}

function Test-DirectoryContentEqual {
    param(
        [Parameter(Mandatory)] [string]$Left,
        [Parameter(Mandatory)] [string]$Right
    )
    if (-not (Test-Path -LiteralPath $Left -PathType Container) -or -not (Test-Path -LiteralPath $Right -PathType Container)) {
        return $false
    }
    $leftRoot = (Resolve-Path -LiteralPath $Left).Path.TrimEnd('\', '/')
    $rightRoot = (Resolve-Path -LiteralPath $Right).Path.TrimEnd('\', '/')
    $leftFiles = @(Get-ChildItem -LiteralPath $leftRoot -File -Recurse -Force | ForEach-Object {
        $_.FullName.Substring($leftRoot.Length).TrimStart('\', '/')
    } | Sort-Object)
    $rightFiles = @(Get-ChildItem -LiteralPath $rightRoot -File -Recurse -Force | ForEach-Object {
        $_.FullName.Substring($rightRoot.Length).TrimStart('\', '/')
    } | Sort-Object)
    if ($leftFiles.Count -ne $rightFiles.Count) { return $false }
    for ($i = 0; $i -lt $leftFiles.Count; $i++) {
        if ($leftFiles[$i] -ne $rightFiles[$i]) { return $false }
        if (-not (Test-FileBytesEqual (Join-Path $leftRoot $leftFiles[$i]) (Join-Path $rightRoot $rightFiles[$i]))) {
            return $false
        }
    }
    return $true
}

function Test-TargetContentMatchesChezmoi {
    param([Parameter(Mandatory)] [string]$Target)
    $global:LASTEXITCODE = 0
    $expectedFile = [IO.Path]::GetTempFileName()
    $process = $null
    try {
        $arguments = @()
        if ($script:ChezmoiBaseArgs) { $arguments += @($script:ChezmoiBaseArgs) }
        if ($script:ChezmoiConfigArgs) { $arguments += @($script:ChezmoiConfigArgs) }
        $arguments += @('cat', $Target)

        $startInfo = New-Object System.Diagnostics.ProcessStartInfo
        $startInfo.FileName = 'chezmoi'
        $startInfo.UseShellExecute = $false
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true
        $startInfo.CreateNoWindow = $true
        $startInfo.Arguments = (@($arguments) | ForEach-Object { ConvertTo-NativeArgument $_ }) -join ' '

        $process = New-Object System.Diagnostics.Process
        $process.StartInfo = $startInfo
        if (-not $process.Start()) { return $false }

        $stderrTask = $process.StandardError.ReadToEndAsync()
        $outputStream = [IO.File]::Open($expectedFile, [IO.FileMode]::Create, [IO.FileAccess]::Write, [IO.FileShare]::Read)
        try {
            $process.StandardOutput.BaseStream.CopyTo($outputStream)
        } finally {
            $outputStream.Dispose()
        }
        $process.WaitForExit()
        $stderrTask.Wait()
        $global:LASTEXITCODE = $process.ExitCode
        if ($process.ExitCode -ne 0) { return $false }

        $expectedPath = ([IO.File]::ReadAllText($expectedFile)).Trim()
        if ($expectedPath -and (Test-Path -LiteralPath $expectedPath)) {
            if (Test-Path -LiteralPath $expectedPath -PathType Container) {
                return (Test-DirectoryContentEqual $Target $expectedPath)
            }
            if (Test-Path -LiteralPath $expectedPath -PathType Leaf) {
                return (Test-FileBytesEqual $Target $expectedPath)
            }
            return $false
        }

        if (Test-Path -LiteralPath $Target -PathType Leaf) {
            return (Test-FileBytesEqual $Target $expectedFile)
        }
        return $false
    } catch {
        return $false
    } finally {
        if ($process) { $process.Dispose() }
        Remove-Item -LiteralPath $expectedFile -Force -ErrorAction SilentlyContinue
    }
}

function Test-TargetAlreadyMatches {
    param([Parameter(Mandatory)] [string]$Target)
    return (Test-ChezmoiVerify $Target) -or
        (Test-TargetContentMatchesChezmoi $Target)
}

function Get-WindowsTerminalSettingsPath {
    return (Join-Path $env:LOCALAPPDATA 'Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json')
}

function Get-WindowsTerminalUnpackagedSettingsPath {
    return (Join-Path $env:LOCALAPPDATA 'Microsoft\Windows Terminal\settings.json')
}

function Copy-WindowsTerminalSettingsForUnpackaged {
    if ($DryRun -or $SkipWindowsTerminalMerge) { return }

    $packagedSettings = Get-WindowsTerminalSettingsPath
    if (-not (Test-Path -LiteralPath $packagedSettings -PathType Leaf)) { return }

    $unpackagedSettings = Get-WindowsTerminalUnpackagedSettingsPath
    try {
        $unpackagedDir = Split-Path -Parent $unpackagedSettings
        if (-not (Test-Path -LiteralPath $unpackagedDir -PathType Container)) {
            New-Item -ItemType Directory -Force -Path $unpackagedDir | Out-Null
        }
        Copy-Item -LiteralPath $packagedSettings -Destination $unpackagedSettings -Force -ErrorAction Stop
        Write-Step "mirrored Windows Terminal settings to unpackaged path"
    } catch {
        Write-Warning ("Could not mirror Windows Terminal settings to unpackaged path: " + $_.Exception.Message)
    }
}

function Test-SamePath {
    param(
        [Parameter(Mandatory)] [string]$Left,
        [Parameter(Mandatory)] [string]$Right
    )
    $leftFull = (Get-FullPathSafe $Left).TrimEnd('\', '/')
    $rightFull = (Get-FullPathSafe $Right).TrimEnd('\', '/')
    return $leftFull.Equals($rightFull, [StringComparison]::OrdinalIgnoreCase)
}

function Stop-NvimSelfLinkIfNeeded {
    if (-not $env:LOCALAPPDATA) { return }

    $nvimTarget = Join-Path $env:LOCALAPPDATA 'nvim'
    $targetItem = Get-Item -LiteralPath $nvimTarget -Force -ErrorAction SilentlyContinue
    if (-not $targetItem) { return }
    # An existing SYMLINK/Junction is the normal already-installed case: it points
    # into the repo, so its resolved value equals <repo>\nvim, but the target
    # LOCATION is not the repo. chezmoi safely replaces it -- do NOT refuse. Only
    # a REAL directory AT the target that resolves to the repo root or <repo>\nvim
    # is a genuine self-overlap (the repo lives there).
    $linkType = if ($targetItem.PSObject.Properties.Name -contains 'LinkType') { $targetItem.LinkType } else { $null }
    if ($linkType -eq 'SymbolicLink' -or $linkType -eq 'Junction') { return }

    $targetReal = Get-RealExistingPath $nvimTarget
    if (-not $targetReal) { return }

    $repoReal = Get-RealExistingPath $ScriptDir
    $repoNvimReal = Get-RealExistingPath (Join-Path $ScriptDir 'nvim')
    if ((-not $repoReal -or -not (Test-SamePath $targetReal $repoReal)) -and
        (-not $repoNvimReal -or -not (Test-SamePath $targetReal $repoNvimReal))) {
        return
    }

    Write-Error @"
setup.ps1: the repo lives at $ScriptDir, which overlaps the path that
setup.ps1 would configure as the Neovim target. Move the repo elsewhere first
(e.g. %USERPROFILE%\dotfiles) and re-run setup.ps1.
"@
    exit 1
}

function Get-ManagedConfigTargets {
    param([switch]$ExcludeWindowsTerminal)
    $wtSettings = Get-WindowsTerminalSettingsPath
    $targets = @(Invoke-ChezmoiOutput @('managed', '--path-style', 'absolute', '--include', 'files,symlinks'))
    if ($ExcludeWindowsTerminal) {
        $targets = @($targets | Where-Object { -not (Test-SamePath $_ $wtSettings) })
    }
    return $targets
}

function Backup-WindowsTerminalSettings {
    if ($SkipWindowsTerminalMerge) { return }

    $settings = Get-WindowsTerminalSettingsPath
    if (-not (Test-Path -LiteralPath $settings -PathType Leaf)) { return }

    $backup = Get-UniqueBackupPath "$settings.bak.$Timestamp"
    if ($DryRun) {
        Write-Step "backup   $settings -> $backup; then Windows Terminal merge"
    } else {
        Copy-Item -LiteralPath $settings -Destination $backup -Force
        Write-Step "backed up $settings -> $backup"
    }
}

function Backup-PreexistingManagedTargets {
    $targets = @(Get-ManagedConfigTargets -ExcludeWindowsTerminal)
    if ($targets.Count -eq 0) {
        Write-Step "backup   no managed file/symlink targets found"
        return
    }

    foreach ($target in $targets) {
        $item = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
        if ($null -eq $item) { continue }
        if (Test-TargetAlreadyMatches $target) {
            Write-Step "ok       $target"
            continue
        }

        $backup = Get-UniqueBackupPath "$target.bak.$Timestamp"
        if ($DryRun) {
            Write-Step "backup   $target -> $backup; then chezmoi apply"
        } else {
            Move-Item -LiteralPath $target -Destination $backup -Force
            Write-Step "backed up $target -> $backup"
        }
    }
}

function New-ChezmoiDryRunConfig {
    $tmp = New-TemporaryFile
    $templatePath = Join-Path $HomeSource '.chezmoi.toml.tmpl'
    $baseArgs = $script:ChezmoiBaseArgs
    $rendered = Get-Content -Raw -LiteralPath $templatePath | & chezmoi @baseArgs execute-template --init
    if ($LASTEXITCODE -ne 0) {
        throw "chezmoi execute-template --init failed while preparing dry-run config"
    }
    Set-Content -LiteralPath $tmp.FullName -Value $rendered -Encoding UTF8
    return $tmp.FullName
}

function Invoke-ChezmoiApplyPhase {
    if (-not (Get-Command chezmoi -ErrorAction SilentlyContinue)) {
        if ($DryRun) {
            # The dogfood dry-run runs BEFORE Phase 1 installs chezmoi; preview
            # rather than fail (a real run has chezmoi on PATH after Phase 1).
            Write-Step "would    chezmoi (installed in Phase 1) backs up divergent configs, then 'chezmoi apply'"
            return
        }
        Write-Host "  FAIL: chezmoi is not on PATH after Phase 1" -ForegroundColor Red
        Write-Host "        Re-run without -SkipDeps, or install chezmoi first." -ForegroundColor Yellow
        exit 1
    }

    if ($DryRun) {
        Write-Warning "DryRun: skipping symlink-privilege probe"
        $dryRunConfig = New-ChezmoiDryRunConfig
        $script:ChezmoiConfigArgs = @('--config', $dryRunConfig, '--config-format', 'toml')
        try {
            Backup-WindowsTerminalSettings
            Backup-PreexistingManagedTargets
            $applyArgs = @('--dry-run', '--verbose', 'apply')
            if ($SkipWindowsTerminalMerge) {
                Write-Step "skip     Windows Terminal settings merge via -SkipWindowsTerminalMerge"
                $applyArgs += @(Get-ManagedConfigTargets -ExcludeWindowsTerminal)
            }
            Invoke-ChezmoiOrExit -Label 'chezmoi dry-run apply' -Arguments $applyArgs
        } finally {
            Remove-Item -LiteralPath $dryRunConfig -Force -ErrorAction SilentlyContinue
            $script:ChezmoiConfigArgs = @()
        }
        return
    }

    if (-not (Test-CanCreateSymlinks)) {
        $elevated = if (Test-IsElevated) { 'yes' } else { 'no' }
        $devmode  = if (Test-DevModeOn)  { 'on' }  else { 'off' }
        Write-Host ""
        Write-Host "setup.ps1: cannot create symbolic links here." -ForegroundColor Red
        Write-Host "  elevated (admin): $elevated    Developer Mode: $devmode"
        Write-Host ""
        Write-Host "  Fix EITHER way (Developer Mode recommended -- no admin, and keeps" -ForegroundColor Yellow
        Write-Host "  scoop/nvim working unprivileged):" -ForegroundColor Yellow
        Write-Host "    1) Enable Developer Mode: Settings -> Privacy & security -> For"
        Write-Host "       developers -> Developer Mode = On.  Then:  .\setup.ps1 -SkipDeps"
        Write-Host "    2) OR run just this config step elevated (admin PowerShell):"
        Write-Host "       .\setup.ps1 -SkipDeps -SkipNvim"
        Write-Host "       then back in a normal shell:  .\setup.ps1 -SkipDeps -SkipConfig"
        Write-Host ""
        exit 1
    }

    Invoke-ChezmoiOrExit -Label 'chezmoi init' -Arguments @('init')
    Backup-WindowsTerminalSettings
    Backup-PreexistingManagedTargets
    $realApplyArgs = @('--no-tty', '--force', 'apply')
    if ($SkipWindowsTerminalMerge) {
        Write-Step "skip     Windows Terminal settings merge via -SkipWindowsTerminalMerge"
        $realApplyArgs += @(Get-ManagedConfigTargets -ExcludeWindowsTerminal)
    }
    Invoke-ChezmoiOrExit -Label 'chezmoi apply' -Arguments $realApplyArgs
    Copy-WindowsTerminalSettingsForUnpackaged
}

# Test seam: set DOTFILES_SETUP_PS1_SOURCE_ONLY and dot-source this file to load
# helper functions without running install, config, or Neovim sync phases.
if ($env:DOTFILES_SETUP_PS1_SOURCE_ONLY) { return }

Stop-NvimSelfLinkIfNeeded

# ---- Phase 1: dependencies ---------------------------------------------------
if (-not $SkipDeps) {
    Phase "Phase 1/4: install dependencies"
    & (Join-Path $ScriptDir 'install-deps.ps1') @depsArgs
} else {
    Write-Host ""
    Write-Host "skipped: Phase 1 (deps) via -SkipDeps"
}

# Phase 1 may install nvim and tools into locations not yet on this process PATH.
# A child installer cannot mutate our PATH, and persistent PATH edits only reach
# new shells. Re-derive PATH so Phase 3-4 can find nvim.
if (-not $DryRun) {
    Update-RuntimePath
}

# ---- Phase 2: apply configs --------------------------------------------------
if (-not ($SkipBootstrap -or $SkipConfig)) {
    Phase "Phase 2/4: apply configs with chezmoi"
    $global:LASTEXITCODE = 0   # reset so a stale code from Phase 1 cannot false-trip
    Invoke-ChezmoiApplyPhase
} else {
    Write-Host ""
    Write-Host "skipped: Phase 2 (config) via -SkipBootstrap/-SkipConfig"
}

# ---- Phases 3 + 4: nvim sync -------------------------------------------------
#
# Lazy + Mason failures are FATAL by default. Pass -BestEffort to downgrade
# them to warnings (useful for offline / proxy-restricted environments where
# you accept a partial install and will run :Lazy / :Mason interactively).
function Invoke-OrFail {
    param([string]$Label, [scriptblock]$Block)
    & $Block
    $rc = $LASTEXITCODE
    if ($rc -ne 0) {
        if ($BestEffort) {
            Write-Warning ("  $Label exited $rc (continuing because -BestEffort is set)")
            return
        }
        # NOTE: ErrorActionPreference = Stop (set at the top of this file)
        # makes Write-Error THROW before any line after it executes. Use
        # Write-Host to print the failure context, then exit with the real rc.
        Write-Host ("  FAIL: $Label exited $rc") -ForegroundColor Red
        Write-Host  "        Re-run with -BestEffort to continue past plugin/LSP failures." -ForegroundColor Yellow
        exit $rc
    }
}

if (-not $SkipNvim -and -not $DryRun) {
    if (Get-Command nvim -ErrorAction SilentlyContinue) {
        Phase "Phase 3/4: sync Neovim plugins (lazy.nvim)"
        Invoke-OrFail "Lazy sync" { & nvim --headless "+Lazy! sync" "+qa" }

        Phase "Phase 4/4: install LSP servers + formatters (Mason)"
        Write-Host "  this can take 3-8 minutes on a fresh Windows machine."
        Invoke-OrFail "Mason install" { & nvim --headless "+MasonToolsInstallSync" "+qa" }
    } else {
        Write-Host ""
        Write-Host "skipped: Phase 3-4 (nvim plugins) -- nvim not on PATH yet."
        Write-Host "         Open a new shell so PATH refreshes, then run:"
        Write-Host "             .\setup.ps1 -SkipDeps -SkipConfig"
    }
} elseif ($DryRun) {
    Write-Host ""
    Write-Host "skipped: Phase 3-4 (nvim plugins) in -DryRun mode"
} else {
    Write-Host ""
    Write-Host "skipped: Phase 3-4 (nvim plugins) via -SkipNvim"
}

# ---- Summary -----------------------------------------------------------------
Write-Host ""
Write-Host "================================================================"
Write-Host "==  setup.ps1: done"
Write-Host "================================================================"
Write-Host ""
Write-Host "Repo:    $ScriptDir"
Write-Host "Try it:  nvim  (then <Space>fg for live grep, :wnf to save w/o format)"
Write-Host ""
if ($DryRun) { Write-Host "(dry run -- nothing was actually installed or changed)" }
