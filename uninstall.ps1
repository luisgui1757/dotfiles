# uninstall.ps1 -- safely remove the chezmoi-managed config layer on Windows.
#
# This deliberately avoids `chezmoi purge`: this checkout is the source tree,
# and purge-style commands can delete chezmoi state/source that the owner keeps.

[CmdletBinding()]
param(
    [switch]$DryRun,
    [switch]$All,
    [switch]$KeepExternals,
    [switch]$NoRestoreBackups,
    [switch]$ForceExternals
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceDir = Join-Path $RepoRoot 'home'
$Chezmoi = $null

$script:Removed = 0
$script:Restored = 0
$script:Skipped = 0
$script:Warnings = 0
$script:DirsRemoved = 0
$script:ExternalsRemoved = 0
$script:DirCandidates = New-Object System.Collections.Generic.List[string]

function Write-SafeWarning {
    param([Parameter(Mandatory)] [string]$Message)
    $script:Warnings++
    Write-Warning $Message
}

function Confirm-Category {
    param([Parameter(Mandatory)] [string]$Prompt)
    if ($All -or $DryRun) { return $true }
    if (-not [Environment]::UserInteractive) {
        Write-SafeWarning "no interactive console and -All was not passed; skipping $Prompt"
        return $false
    }
    $answer = Read-Host "$Prompt [y/N]"
    return ($answer -match '^[Yy]$')
}

function Get-ItemOrNull {
    param([Parameter(Mandatory)] [string]$Path)
    return (Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue)
}

function Test-TargetExists {
    param([Parameter(Mandatory)] [string]$Path)
    return ($null -ne (Get-ItemOrNull -Path $Path))
}

function Resolve-CanonicalPath {
    param([Parameter(Mandatory)] [string]$Path)
    try {
        return (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
    } catch {
        return [IO.Path]::GetFullPath($Path)
    }
}

function Test-PathUnderRepo {
    param([Parameter(Mandatory)] [string]$Path)
    $repo = (Resolve-CanonicalPath -Path $RepoRoot).TrimEnd('\', '/')
    $candidate = (Resolve-CanonicalPath -Path $Path).TrimEnd('\', '/')
    return $candidate.StartsWith($repo + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)
}

function Get-LinkTargetPath {
    param(
        [Parameter(Mandatory)] $Item,
        [Parameter(Mandatory)] [string]$LinkPath
    )
    $target = $Item.Target
    if ($target -is [array]) { $target = $target[0] }
    if ([string]::IsNullOrWhiteSpace($target)) { return $null }
    if ([IO.Path]::IsPathRooted($target)) { return $target }
    return (Join-Path (Split-Path -Parent $LinkPath) $target)
}

function Get-NewestBackup {
    param([Parameter(Mandatory)] [string]$Target)
    $parent = Split-Path -Parent $Target
    $leaf = Split-Path -Leaf $Target
    if (-not (Test-Path -LiteralPath $parent)) { return $null }
    $backups = @(Get-ChildItem -LiteralPath $parent -Force -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like "$leaf.bak.*" } |
        Sort-Object LastWriteTimeUtc -Descending)
    if ($backups.Count -eq 0) { return $null }
    return $backups[0].FullName
}

function Add-ParentDirs {
    param([Parameter(Mandatory)] [string]$Path)
    $userHome = (Resolve-CanonicalPath -Path $env:USERPROFILE).TrimEnd('\', '/')
    $dir = Split-Path -Parent $Path
    while (-not [string]::IsNullOrWhiteSpace($dir)) {
        $canonical = (Resolve-CanonicalPath -Path $dir).TrimEnd('\', '/')
        if ($canonical -eq $userHome) { break }
        $script:DirCandidates.Add($dir)
        $next = Split-Path -Parent $dir
        if ($next -eq $dir) { break }
        $dir = $next
    }
}

function Restore-BackupIfPresent {
    param([Parameter(Mandatory)] [string]$Target)
    if ($NoRestoreBackups) { return }
    $backup = Get-NewestBackup -Target $Target
    if (-not $backup) { return }
    if ($DryRun) {
        # Accurate preview: in a real run the target was just removed, so the
        # restore would proceed; show that rather than the exists-guard warning.
        Write-Host "  would: restore $backup -> $Target"
        return
    }
    if (Test-TargetExists -Path $Target) {
        Write-SafeWarning "not restoring $backup because $Target already exists"
        $script:Skipped++
        return
    }
    Move-Item -LiteralPath $backup -Destination $Target -Force
    $script:Restored++
    Write-Host "  restored  $Target <- $backup"
}

function Test-ChezmoiTargetUnmodified {
    param([Parameter(Mandatory)] [string]$Target)
    # "Is this on-disk file still byte-for-byte what chezmoi would write?" answered
    # by the chezmoi verify command (exit 0 = matches managed state, nonzero =
    # drifted). This deliberately avoids hashing a `chezmoi cat > tmp` capture:
    # native-command redirection in PowerShell can re-encode / CRLF-translate on
    # Windows, which would make an unmodified copy look modified and wrongly leave
    # it behind. verify is byte-exact by the chezmoi logic itself with zero
    # redirect surface. The try/catch keeps it safe even when
    # $PSNativeCommandUseErrorActionPreference turns a nonzero (drifted) verify
    # exit into a throw -> treat as modified and skip.
    try {
        & $Chezmoi --source $SourceDir --no-tty verify $Target *> $null
        return ($LASTEXITCODE -eq 0)
    } catch {
        return $false
    }
}

function Test-WindowsTerminalSettingsPath {
    param([Parameter(Mandatory)] [string]$Path)
    $normalized = $Path -replace '/', '\'
    return ($normalized -like '*\AppData\Local\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json')
}

function Test-ExternalPath {
    param([Parameter(Mandatory)] [string]$Path)
    $root = Join-Path $env:USERPROFILE '.local\share\dotfiles\zsh-plugins'
    $normalizedPath = (Resolve-CanonicalPath -Path $Path).TrimEnd('\', '/')
    $normalizedRoot = (Resolve-CanonicalPath -Path $root).TrimEnd('\', '/')
    return ($normalizedPath -eq $normalizedRoot -or $normalizedPath.StartsWith($normalizedRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase))
}

function Remove-ManagedTarget {
    param([Parameter(Mandatory)] [string]$Target)
    if (Test-ExternalPath -Path $Target) { return }

    if (Test-WindowsTerminalSettingsPath -Path $Target) {
        if (Test-TargetExists -Path $Target) {
            Write-SafeWarning "leaving Windows Terminal settings.json untouched: $Target"
            Write-Host "      WT merge is idempotent but not invertible; restore manually from backup if needed."
            $backup = Get-NewestBackup -Target $Target
            if ($backup) { Write-Host "      newest backup: $backup" }
        }
        return
    }

    $item = Get-ItemOrNull -Path $Target
    if (-not $item) { return }

    $linkType = if ($item.PSObject.Properties.Name -contains 'LinkType') { $item.LinkType } else { $null }
    if ($linkType -eq 'SymbolicLink' -or $linkType -eq 'Junction') {
        $targetPath = Get-LinkTargetPath -Item $item -LinkPath $Target
        if ($targetPath -and (Test-PathUnderRepo -Path $targetPath)) {
            if ($DryRun) {
                Write-Host "  would: remove symlink $Target -> $targetPath"
            } else {
                try {
                    Remove-Item -LiteralPath $Target -Force -ErrorAction Stop
                    $script:Removed++
                    Write-Host "  removed   $Target"
                } catch {
                    Write-SafeWarning "could not remove symlink $Target. Developer Mode or elevation may be required for symlink operations: $($_.Exception.Message)"
                    $script:Skipped++
                    return
                }
            }
            Add-ParentDirs -Path $Target
            Restore-BackupIfPresent -Target $Target
            return
        }
        Write-SafeWarning "skipping symlink outside repo: $Target -> $targetPath"
        $script:Skipped++
        return
    }

    if ($item.PSIsContainer) {
        Add-ParentDirs -Path $Target
        return
    }

    if (Test-ChezmoiTargetUnmodified -Target $Target) {
        if ($DryRun) {
            Write-Host "  would: remove managed copy $Target"
        } else {
            Remove-Item -LiteralPath $Target -Force
            $script:Removed++
            Write-Host "  removed   $Target"
        }
        Add-ParentDirs -Path $Target
        Restore-BackupIfPresent -Target $Target
        return
    }

    Write-SafeWarning "skipping user-modified or unverified file: $Target"
    $script:Skipped++
}

function Get-ManagedTargets {
    if (-not (Test-Path -LiteralPath $SourceDir -PathType Container)) {
        throw "missing chezmoi source dir: $SourceDir"
    }
    $script:Chezmoi = (Get-Command chezmoi -ErrorAction Stop).Source
    $output = & $Chezmoi --source $SourceDir managed --path-style absolute 2>&1
    if ($LASTEXITCODE -ne 0) {
        $text = ($output | Out-String).Trim()
        throw "could not enumerate managed targets. Run 'chezmoi --source `"$SourceDir`" init' first for this HOME.`n$text"
    }
    return @($output | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) } | ForEach-Object { [string]$_ } | Sort-Object Length -Descending)
}

function Remove-EmptyDirs {
    foreach ($dir in @($script:DirCandidates | Sort-Object -Unique | Sort-Object Length -Descending)) {
        if (-not (Test-Path -LiteralPath $dir -PathType Container)) { continue }
        if (@(Get-ChildItem -LiteralPath $dir -Force -ErrorAction SilentlyContinue).Count -gt 0) { continue }
        try {
            Remove-Item -LiteralPath $dir -Force -ErrorAction Stop
            $script:DirsRemoved++
            Write-Host "  rmdir     $dir"
        } catch {
            Write-SafeWarning "could not remove empty directory ${dir}: $($_.Exception.Message)"
        }
    }
}

function Test-ExternalDirty {
    # A pinned chezmoi external is a clean detached-HEAD clone. Treat any
    # uncommitted/staged change or untracked file as user work to preserve. If
    # git is unavailable or the path is not a git repo, cleanliness cannot be
    # verified, so err on the safe side and treat it as dirty.
    param([Parameter(Mandatory)] [string]$Dir)
    $git = Get-Command git -ErrorAction SilentlyContinue
    if (-not $git) { return $true }
    try {
        & $git.Source -C $Dir rev-parse --git-dir *> $null
        if ($LASTEXITCODE -ne 0) { return $true }
        # --ignored so a user file matching the plugin .gitignore (a cache or
        # build artifact git treats as ignored) still counts as dirty and is
        # kept; plain --porcelain omits ignored files.
        $status = (& $git.Source -C $Dir status --porcelain --ignored 2>$null | Out-String)
        # If the status query itself fails, cleanliness is unknown -> dirty.
        if ($LASTEXITCODE -ne 0) { return $true }
        return (-not [string]::IsNullOrWhiteSpace($status))
    } catch {
        return $true
    }
}

function Remove-Externals {
    if ($KeepExternals) {
        Write-Host "  kept      zsh plugin externals (-KeepExternals)"
        return
    }
    if (-not (Confirm-Category -Prompt 'Remove zsh plugin externals under ~/.local/share/dotfiles/zsh-plugins?')) {
        $script:Skipped++
        return
    }
    $root = Join-Path $env:USERPROFILE '.local\share\dotfiles\zsh-plugins'
    foreach ($name in @('fzf-tab', 'zsh-autosuggestions')) {
        $dir = Join-Path $root $name
        if (-not (Test-TargetExists -Path $dir)) { continue }
        if ((-not $ForceExternals) -and (Test-ExternalDirty -Dir $dir)) {
            Write-SafeWarning "keeping ${dir}: uncommitted or unverifiable changes (use -ForceExternals to remove)"
            $script:Skipped++
            continue
        }
        if ($DryRun) {
            Write-Host "  would: remove external $dir"
        } else {
            Remove-Item -LiteralPath $dir -Recurse -Force
            $script:ExternalsRemoved++
            Write-Host "  removed   $dir"
        }
    }
    if (-not $DryRun) {
        Remove-Item -LiteralPath $root -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Split-Path -Parent $root) -Force -ErrorAction SilentlyContinue
    }
}

Write-Host "uninstall.ps1: repo=$RepoRoot source=$SourceDir dry-run=$DryRun restore-backups=$(-not $NoRestoreBackups)"
Write-Host

try {
    $targets = @(Get-ManagedTargets)
    if ($targets.Count -gt 0 -and (Confirm-Category -Prompt 'Remove chezmoi-managed config targets?')) {
        foreach ($target in $targets) {
            Remove-ManagedTarget -Target $target
        }
    } else {
        $script:Skipped++
    }
    if (-not $DryRun) { Remove-EmptyDirs }
    Remove-Externals

    Write-Host
    if ($script:Removed -eq 0 -and $script:Restored -eq 0 -and $script:DirsRemoved -eq 0 -and $script:ExternalsRemoved -eq 0) {
        Write-Host "uninstall.ps1: nothing to remove"
    }
    Write-Host ("summary: removed={0} restored={1} dirs_removed={2} externals_removed={3} skipped={4} warnings={5}" -f `
        $script:Removed, $script:Restored, $script:DirsRemoved, $script:ExternalsRemoved, $script:Skipped, $script:Warnings)
} catch {
    Write-Host "FAIL: $($_.Exception.Message)"
    exit 1
}

# Explicit success exit: internal `chezmoi verify` probes leave $LASTEXITCODE=1
# when a target is user-modified/unmanaged, so without this the script would
# inherit that nonzero code on an otherwise-successful run.
exit 0
