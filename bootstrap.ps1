# bootstrap.ps1 -- symlink dotfiles into Windows-appropriate paths.
# Requires either an elevated PowerShell session OR Developer Mode enabled
# (Settings -> Privacy & security -> For developers -> Developer Mode = On).
#
# Usage:
#   .\bootstrap.ps1
#   .\bootstrap.ps1 -DryRun
#   .\bootstrap.ps1 -SkipWindowsTerminalMerge # leave WT settings.json untouched
#   .\bootstrap.ps1 -MergeWindowsTerminal     # no-op alias; WT merge is default-on

[CmdletBinding()]
param(
    [switch]$DryRun,
    [switch]$MergeWindowsTerminal,        # back-compat no-op alias; merge is default-on
    [switch]$SkipWindowsTerminalMerge
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$Timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'

function Get-UniqueBackupPath {
    param([Parameter(Mandatory)] [string]$Base)
    if (-not (Test-Path -LiteralPath $Base)) { return $Base }
    $i = 1
    while (Test-Path -LiteralPath "$Base.$i") { $i++ }
    return "$Base.$i"
}

function Write-Step { param([string]$msg) Write-Host "  $msg" }

function New-NativeSymLink {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string]$Source,
        [Parameter(Mandatory)] [string]$Destination
    )

    if (-not ('DotfilesSymbolicLink' -as [type])) {
        Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

public static class DotfilesSymbolicLink {
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
    # 0x2 permits unprivileged symlink creation when Developer Mode is enabled.
    $flags = 0x2
    if ($sourceItem.PSIsContainer) { $flags = $flags -bor 0x1 }

    $ok = [DotfilesSymbolicLink]::CreateSymbolicLink($Destination, $Source, $flags)
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

        # Windows PowerShell 5.1 / cmd mklink can throw a misleading "file not
        # found" for profile links under Documents even when both paths exist.
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

function New-SymLink {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string]$Source,
        [Parameter(Mandatory)] [string]$Destination
    )

    $parent = Split-Path -Parent $Destination
    if (-not (Test-Path -LiteralPath $parent)) {
        if ($DryRun) {
            Write-Step "mkdir   $parent"
        } else {
            New-Item -ItemType Directory -Force -Path $parent | Out-Null
        }
    }

    $item = Get-Item -LiteralPath $Destination -Force -ErrorAction SilentlyContinue
    if ($null -ne $item) {
        if ($item.LinkType -eq 'SymbolicLink') {
            $existing = $item.Target
            if ($existing -is [array]) { $existing = $existing[0] }
            $targetMissing = $false
            if ($existing) {
                $targetMissing = -not (Test-Path -LiteralPath $existing)
            }
            if (($existing -eq $Source) -and (-not $targetMissing)) {
                Write-Step "ok       $Destination -> $Source"
                return
            }
            # Existing symlink points elsewhere or to a missing path. Back up
            # the symlink itself before replacing so prior user choice is not
            # silently lost.
            $backup = Get-UniqueBackupPath "$Destination.bak.$Timestamp"
            $reason = if ($targetMissing) { "broken -> $existing" } else { "was -> $existing" }
            if ($DryRun) {
                Write-Step "relink   $Destination ($reason; backup -> $backup)"
                return
            }
            Move-Item -LiteralPath $Destination -Destination $backup -Force
            New-SymbolicLinkItem -Source $Source -Destination $Destination
            Write-Step "relinked $Destination -> $Source  (prior symlink -> $backup)"
            return
        }

        # Real file/dir at the destination -- back it up.
        $backup = Get-UniqueBackupPath "$Destination.bak.$Timestamp"
        if ($DryRun) {
            Write-Step "backup   $Destination -> $backup; then symlink"
            return
        }
        try {
            Move-Item -LiteralPath $Destination -Destination $backup -Force -ErrorAction Stop
        } catch {
            Write-Host ""
            Write-Host "  FAIL     could not back up: $Destination" -ForegroundColor Red
            Write-Host "           reason: $($_.Exception.Message)" -ForegroundColor Red
            if ($_.Exception.Message -match 'being used by another process') {
                # Use single-quoted strings -- no interpolation needed, and
                # this dodges the backtick-escapes-the-closing-quote bug that
                # made an earlier "...loaded the old `$PROFILE`" string
                # silently swallow the rest of the file at parse time.
                Write-Host '           This usually means a running program has files open inside' -ForegroundColor Yellow
                Write-Host '           the target. Most common culprits, in order:' -ForegroundColor Yellow
                Write-Host '             1. A running Neovim / Neovide / nvim-qt:' -ForegroundColor Yellow
                Write-Host '                Get-Process -Name nvim*, neovide -EA SilentlyContinue | Stop-Process -Force' -ForegroundColor Yellow
                Write-Host '             2. Another PowerShell window that loaded the old $PROFILE' -ForegroundColor Yellow
                Write-Host '             3. Windows Defender / antivirus scanning the directory' -ForegroundColor Yellow
                Write-Host '             4. A Mason-installed LSP server still attached' -ForegroundColor Yellow
                Write-Host '           Close the holder, then re-run .\bootstrap.ps1 (idempotent).' -ForegroundColor Yellow
            }
            throw
        }
        New-SymbolicLinkItem -Source $Source -Destination $Destination
        Write-Step "backed up $Destination -> $backup; linked -> $Source"
        return
    }

    if ($DryRun) {
        Write-Step "link     $Destination -> $Source"
        return
    }
    New-SymbolicLinkItem -Source $Source -Destination $Destination
    Write-Step "linked   $Destination -> $Source"
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
    # Developer Mode sets AllowDevelopmentWithoutDevLicense=1 under AppModelUnlock.
    try {
        $k = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock'
        return (((Get-ItemProperty -Path $k -Name AllowDevelopmentWithoutDevLicense -ErrorAction Stop).AllowDevelopmentWithoutDevLicense) -eq 1)
    } catch { return $false }
}

# Test seam: setting the env var DOTFILES_BOOTSTRAP_SOURCE_ONLY and dot-sourcing
# this file loads the functions above WITHOUT running the symlink phase, so tests can
# exercise New-SymbolicLinkItem / New-NativeSymLink in isolation. Unset in
# normal runs, so this is skipped.
if ($env:DOTFILES_BOOTSTRAP_SOURCE_ONLY) { return }

Write-Host "bootstrap.ps1: repo=$RepoRoot dry-run=$DryRun"
Write-Host

# DryRun must not touch the filesystem. The symlink-creation probe writes
# temp files, so skip it entirely in DryRun mode.
if ($DryRun) {
    Write-Host "  (DryRun: skipping symlink-privilege probe)"
} elseif (-not (Test-CanCreateSymlinks)) {
    # Detect WHY and tell the user exactly what to do, instead of a raw error.
    $elevated = if (Test-IsElevated) { 'yes' } else { 'no' }
    $devmode  = if (Test-DevModeOn)  { 'on' }  else { 'off' }
    Write-Host ""
    Write-Host "bootstrap.ps1: cannot create symbolic links here." -ForegroundColor Red
    Write-Host "  elevated (admin): $elevated    Developer Mode: $devmode"
    Write-Host ""
    Write-Host "  Fix EITHER way (Developer Mode recommended -- no admin, and keeps" -ForegroundColor Yellow
    Write-Host "  scoop/nvim working unprivileged):" -ForegroundColor Yellow
    Write-Host "    1) Enable Developer Mode: Settings -> Privacy & security -> For"
    Write-Host "       developers -> Developer Mode = On.  Then:  .\setup.ps1 -SkipDeps"
    Write-Host "    2) OR run just this step elevated (admin PowerShell):  .\bootstrap.ps1"
    Write-Host "       then back in a normal shell:  .\setup.ps1 -SkipDeps -SkipBootstrap"
    Write-Host ""
    exit 1
}

# ---- Self-link guard ---------------------------------------------------------
# Refuse to run if the symlink we would create would overlap the repo. If the
# destination is already a symlink, New-SymLink handles relink / no-op cases
# correctly, so the self-link risk only exists when the destination is a real
# directory or file.
$nvimDest = Join-Path $env:LOCALAPPDATA 'nvim'
$destItem = Get-Item -LiteralPath $nvimDest -Force -ErrorAction SilentlyContinue
$destIsRealDir = $destItem -and ($destItem.PSIsContainer) -and ($destItem.LinkType -ne 'SymbolicLink')
if ($destIsRealDir) {
    $repoResolved     = (Resolve-Path -LiteralPath $RepoRoot -ErrorAction SilentlyContinue).Path
    $repoNvimResolved = (Resolve-Path -LiteralPath (Join-Path $RepoRoot 'nvim') -ErrorAction SilentlyContinue).Path
    $destResolved     = (Resolve-Path -LiteralPath $nvimDest -ErrorAction SilentlyContinue).Path
    $selfLink = $false
    # Scenario A: the repo nvim subdir IS the destination dir
    if ($repoNvimResolved -and $destResolved -and ($repoNvimResolved -eq $destResolved)) { $selfLink = $true }
    # Scenario B: the repo root IS the destination dir (clone -> %LOCALAPPDATA%\nvim)
    if ($repoResolved -and $destResolved -and ($repoResolved -eq $destResolved))       { $selfLink = $true }
    if ($selfLink) {
        Write-Error @"
bootstrap.ps1: REFUSING to run.

  Repo root:           $repoResolved
  Repo nvim dir:       $repoNvimResolved
  Symlink destination: $nvimDest

  The symlink we would create would overlap your repo. Running
  bootstrap would back up the repo and replace it with a symlink
  to nothing.

  Move the repo elsewhere first (e.g. `$env:USERPROFILE\dotfiles`) and re-run.
"@
        exit 1
    }
}

# ---- Links -------------------------------------------------------------------
New-SymLink -Source (Join-Path $RepoRoot 'nvim')                       -Destination $nvimDest
New-SymLink -Source (Join-Path $RepoRoot 'starship\starship.toml')     -Destination (Join-Path $env:USERPROFILE '.config\starship.toml')
New-SymLink -Source (Join-Path $RepoRoot 'tmux\tmux.conf')             -Destination (Join-Path $env:USERPROFILE '.tmux.conf')
if (-not (Get-Command pwsh -ErrorAction SilentlyContinue)) {
    Write-Host "  WARN     psmux will fail to spawn panes without pwsh; install via install-deps.ps1 first" -ForegroundColor Yellow
}
New-SymLink -Source (Join-Path $RepoRoot 'tmux\tmux.windows.conf')     -Destination (Join-Path $env:USERPROFILE '.tmux.windows.conf')
New-SymLink -Source (Join-Path $RepoRoot 'shells\powershell_profile.ps1') -Destination $PROFILE

# lazygit config -- %LOCALAPPDATA%\lazygit\config.yml on Windows. lazygit
# v0.58 reads its config from %LOCALAPPDATA%\lazygit, NOT %APPDATA%\lazygit
# (verified via `lazygit --print-config-dir`). The config binds move-commit
# to uppercase J/K so the chord survives psmux and bare terminals.
$lazygitDir = Join-Path $env:LOCALAPPDATA 'lazygit'
New-SymLink -Source (Join-Path $RepoRoot 'lazygit\config.yml') -Destination (Join-Path $lazygitDir 'config.yml')

# Optional: WSL Ubuntu access -- symlinks inside WSL handled by bootstrap.sh.

# ---- WT fragment merge --------------------------------------------------------
# WT settings merge: DEFAULT-ON. Skips itself safely when settings.json does not
# exist yet (WT never launched) -- see the not-found warn below. Opt out with
# -SkipWindowsTerminalMerge. -MergeWindowsTerminal is a retained no-op alias.
$null = $MergeWindowsTerminal   # referenced so the back-compat switch is not flagged unused
if (-not $SkipWindowsTerminalMerge) {
    $wtSettingsCandidates = @(
        "$env:LOCALAPPDATA\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json",
        "$env:LOCALAPPDATA\Packages\Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe\LocalState\settings.json"
    )
    $wtSettings = $wtSettingsCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if (-not $wtSettings) {
        Write-Warning "Windows Terminal settings.json not found; skipping merge."
    } else {
        $fragmentPath = Join-Path $RepoRoot 'windows-terminal\settings.fragment.jsonc'
        # Strip start-of-line // comments WITHOUT a regex literal that contains
        # both // and $ -- PS 5.1 has been observed mis-tokenizing that pattern
        # and reporting "missing terminator" far downstream from the real issue.
        $fragmentLines = Get-Content -LiteralPath $fragmentPath | Where-Object {
            $_ -notmatch "^\s*//"
        }
        $fragment = ($fragmentLines -join "`n") | ConvertFrom-Json

        $backup = Get-UniqueBackupPath "$wtSettings.bak.$Timestamp"
        if ($DryRun) {
            Write-Step "merge    $wtSettings (backup -> $backup)"
        } else {
            Copy-Item -LiteralPath $wtSettings -Destination $backup -Force
            $current = Get-Content -Raw -LiteralPath $wtSettings | ConvertFrom-Json

            # Defensive: initialize containers that a damaged or minimal
            # settings.json might be missing.
            if ($null -eq $current.profiles) {
                $current | Add-Member -NotePropertyName profiles -NotePropertyValue ([pscustomobject]@{}) -Force
            }

            # Top-level scalar merge. Written as straight-line if-statements
            # rather than a foreach over an array literal because some PS 5.1
            # parser quirks have made array literals unreliable in this exact
            # block on user machines.
            function Set-OrAdd-Property {
                param($obj, [string]$name, $value)
                if ($null -eq $obj.$name) {
                    $obj | Add-Member -NotePropertyName $name -NotePropertyValue $value -Force
                } else {
                    $obj.$name = $value
                }
            }
            function Get-ArrayValue {
                param($value)
                if ($null -eq $value) {
                    return @()
                }
                return @($value)
            }
            function Merge-ObjectArrayByProperty {
                param($currentItems, $fragmentItems, [string]$propertyName)
                $result = @()
                $fragmentByKey = @{}
                $emitted = @{}

                foreach ($item in (Get-ArrayValue $fragmentItems)) {
                    $key = [string]$item.$propertyName
                    if ($key) {
                        $fragmentByKey[$key] = $item
                    }
                }

                foreach ($item in (Get-ArrayValue $currentItems)) {
                    $key = [string]$item.$propertyName
                    if ($key -and $fragmentByKey.ContainsKey($key)) {
                        $result += $fragmentByKey[$key]
                        $emitted[$key] = $true
                    } else {
                        $result += $item
                    }
                }

                foreach ($item in (Get-ArrayValue $fragmentItems)) {
                    $key = [string]$item.$propertyName
                    if (-not $key -or -not $emitted.ContainsKey($key)) {
                        $result += $item
                    }
                }

                return $result
            }
            function Get-WTActionKeySet {
                param($item)
                $keys = @()
                if ($null -eq $item -or $null -eq $item.keys) {
                    return @()
                }
                foreach ($key in (Get-ArrayValue $item.keys)) {
                    if ($null -eq $key) {
                        continue
                    }
                    $keyText = ([string]$key).Trim()
                    if ($keyText) {
                        $keys += $keyText.ToLowerInvariant()
                    }
                }
                return @($keys | Sort-Object -Unique)
            }
            function Test-WTActionKeyOverlap {
                param($leftKeys, $rightKeys)
                foreach ($leftKey in (Get-ArrayValue $leftKeys)) {
                    foreach ($rightKey in (Get-ArrayValue $rightKeys)) {
                        if ($leftKey -eq $rightKey) {
                            return $true
                        }
                    }
                }
                return $false
            }
            function Merge-WTActions {
                param($currentItems, $fragmentItems)
                $result = @()
                $fragmentEntries = @()
                $emitted = @{}
                $index = 0

                foreach ($item in (Get-ArrayValue $fragmentItems)) {
                    $fragmentEntries += [pscustomobject]@{
                        Index = [string]$index
                        Item = $item
                        Keys = @(Get-WTActionKeySet $item)
                    }
                    $index += 1
                }

                foreach ($item in (Get-ArrayValue $currentItems)) {
                    $currentKeys = @(Get-WTActionKeySet $item)
                    $matches = @()
                    foreach ($fragmentEntry in $fragmentEntries) {
                        if ($currentKeys.Count -gt 0 -and $fragmentEntry.Keys.Count -gt 0 -and (Test-WTActionKeyOverlap $currentKeys $fragmentEntry.Keys)) {
                            $matches += $fragmentEntry
                        }
                    }
                    if ($matches.Count -gt 0) {
                        foreach ($match in $matches) {
                            if (-not $emitted.ContainsKey($match.Index)) {
                                $result += $match.Item
                                $emitted[$match.Index] = $true
                            }
                        }
                    } else {
                        $result += $item
                    }
                }

                foreach ($fragmentEntry in $fragmentEntries) {
                    if (-not $emitted.ContainsKey($fragmentEntry.Index)) {
                        $result += $fragmentEntry.Item
                    }
                }

                return $result
            }
            if ($null -ne $fragment.copyFormatting)        { Set-OrAdd-Property $current "copyFormatting"        $fragment.copyFormatting }
            if ($null -ne $fragment.copyOnSelect)          { Set-OrAdd-Property $current "copyOnSelect"          $fragment.copyOnSelect }
            if ($null -ne $fragment.firstWindowPreference) { Set-OrAdd-Property $current "firstWindowPreference" $fragment.firstWindowPreference }
            if ($null -ne $fragment.initialRows)           { Set-OrAdd-Property $current "initialRows"           $fragment.initialRows }
            if ($null -ne $fragment.theme)                 { Set-OrAdd-Property $current "theme"                 $fragment.theme }
            if ($null -ne $fragment.useAcrylicInTabRow)    { Set-OrAdd-Property $current "useAcrylicInTabRow"    $fragment.useAcrylicInTabRow }
            if ($null -ne $fragment.windowingBehavior)     { Set-OrAdd-Property $current "windowingBehavior"     $fragment.windowingBehavior }

            if ($null -eq $current.profiles.defaults) {
                $current.profiles | Add-Member -NotePropertyName defaults -NotePropertyValue $fragment.profiles.defaults -Force
            } else {
                $current.profiles.defaults = $fragment.profiles.defaults
            }
            Set-OrAdd-Property $current "actions" @(Merge-WTActions $current.actions $fragment.actions)
            Set-OrAdd-Property $current "schemes" @(Merge-ObjectArrayByProperty $current.schemes $fragment.schemes "name")
            Set-OrAdd-Property $current "themes"  @(Merge-ObjectArrayByProperty $current.themes  $fragment.themes  "name")
            $tmp = "$wtSettings.tmp"
            $current | ConvertTo-Json -Depth 100 | Set-Content -LiteralPath $tmp -Encoding UTF8
            Move-Item -Force -LiteralPath $tmp -Destination $wtSettings
            Write-Step "merged   $wtSettings (backup -> $backup)"
        }
    }
}

Write-Host
Write-Host "bootstrap.ps1: done"
if ($DryRun) { Write-Host "(dry run -- no changes were made)" }
