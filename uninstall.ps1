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
$script:ManagedTargetContexts = @{}
$script:Timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'

function Resolve-WindowsTargetIdentity {
    param(
        [scriptblock]$FolderResolver = {
            param([string]$Name)
            $folder = [Environment+SpecialFolder]([Enum]::Parse([Environment+SpecialFolder], $Name))
            return [Environment]::GetFolderPath($folder)
        },
        [AllowEmptyString()] [string]$RuntimeProfile = ([string]$PROFILE)
    )
    $resolved = @{}
    foreach ($name in 'UserProfile', 'LocalApplicationData', 'MyDocuments') {
        $path = [string](& $FolderResolver $name)
        $isWindowsAbsolute = $path -match '^(?:[A-Za-z]:[\\/]|\\\\)'
        if ([string]::IsNullOrWhiteSpace($path) -or (-not [IO.Path]::IsPathRooted($path) -and -not $isWindowsAbsolute) -or
            $path -match '^[A-Za-z]:[^\\/]') {
            throw "Windows $name known folder is missing or not absolute: $path"
        }
        $resolved[$name] = if ($isWindowsAbsolute -and $env:OS -ne 'Windows_NT') {
            ($path -replace '/', '\').TrimEnd('\')
        } else { [IO.Path]::GetFullPath($path).TrimEnd('\', '/') }
    }
    $runtimeIsWindowsAbsolute = $RuntimeProfile -match '^(?:[A-Za-z]:[\\/]|\\\\)'
    if ([string]::IsNullOrWhiteSpace($RuntimeProfile) -or
        (-not [IO.Path]::IsPathRooted($RuntimeProfile) -and -not $runtimeIsWindowsAbsolute) -or
        $RuntimeProfile -match '^[A-Za-z]:[^\\/]') {
        throw "PowerShell runtime profile path is missing or not absolute: $RuntimeProfile"
    }
    return [pscustomobject]@{
        UserProfile = $resolved.UserProfile
        LocalApplicationData = $resolved.LocalApplicationData
        Documents = $resolved.MyDocuments
        RuntimeProfile = if ($runtimeIsWindowsAbsolute -and $env:OS -ne 'Windows_NT') {
            $RuntimeProfile -replace '/', '\'
        } else { [IO.Path]::GetFullPath($RuntimeProfile) }
    }
}

function Get-WindowsLocalApplicationData {
    if ($script:WindowsIdentity) { return $script:WindowsIdentity.LocalApplicationData }
    if ($env:OS -eq 'Windows_NT') {
        return [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
    }
    return $env:LOCALAPPDATA
}

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
    $prefix = "$leaf.bak."
    $candidates = @(Get-ChildItem -LiteralPath $parent -Force -ErrorAction SilentlyContinue |
        Where-Object { $_.Name.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) })
    if ($candidates.Count -eq 0) { return $null }

    $parsed = @()
    foreach ($candidate in $candidates) {
        $suffix = $candidate.Name.Substring($prefix.Length)
        $match = [regex]::Match($suffix, '^(?<timestamp>[0-9]{8}-[0-9]{6})(?:\.(?<collision>[1-9][0-9]*))?$')
        $timestamp = [DateTime]::MinValue
        $collision = 0
        $validTimestamp = $match.Success -and [DateTime]::TryParseExact(
            $match.Groups['timestamp'].Value,
            'yyyyMMdd-HHmmss',
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::None,
            [ref]$timestamp
        )
        $validCollision = (-not $match.Success) -or (-not $match.Groups['collision'].Success) -or
            ([int]::TryParse($match.Groups['collision'].Value, [ref]$collision) -and $collision -gt 0)
        if (-not $match.Success -or -not $validTimestamp -or -not $validCollision) {
            throw "malformed backup candidate for ${Target}: $($candidate.FullName). Expected $leaf.bak.YYYYMMDD-HHMMSS[.positive-collision]."
        }
        $parsed += [pscustomobject]@{
            Path = $candidate.FullName
            Timestamp = $timestamp
            Collision = $collision
        }
    }
    $ordered = @($parsed | Sort-Object Timestamp, Collision -Descending)
    if ($ordered.Count -gt 1 -and
        $ordered[0].Timestamp -eq $ordered[1].Timestamp -and
        $ordered[0].Collision -eq $ordered[1].Collision) {
        throw "ambiguous backup candidates for ${Target}: $($ordered[0].Path), $($ordered[1].Path)"
    }
    return $ordered[0].Path
}

function Get-UniqueRecoveryPath {
    param([Parameter(Mandatory)] [string]$Base)
    $candidate = $Base
    $index = 0
    while (Test-TargetExists -Path $candidate) {
        $index++
        $candidate = "$Base.$index"
    }
    return $candidate
}

function Add-ParentDirs {
    param([Parameter(Mandatory)] [string]$Path)
    $roots = if ($script:WindowsIdentity) {
        @(
            $script:WindowsIdentity.UserProfile,
            $script:WindowsIdentity.LocalApplicationData,
            $script:WindowsIdentity.Documents
        )
    } else { @($env:USERPROFILE) }
    $canonicalPath = (Resolve-CanonicalPath -Path $Path).TrimEnd('\', '/')
    $owningRoot = @($roots | Where-Object {
            $root = (Resolve-CanonicalPath -Path $_).TrimEnd('\', '/')
            $canonicalPath.Equals($root, [StringComparison]::OrdinalIgnoreCase) -or
                $canonicalPath.StartsWith($root + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)
        } | Sort-Object Length -Descending | Select-Object -First 1)
    if ($owningRoot.Count -eq 0) { return }
    $owningRoot = (Resolve-CanonicalPath -Path $owningRoot[0]).TrimEnd('\', '/')
    $dir = Split-Path -Parent $Path
    while (-not [string]::IsNullOrWhiteSpace($dir)) {
        $canonical = (Resolve-CanonicalPath -Path $dir).TrimEnd('\', '/')
        if ($canonical.Equals($owningRoot, [StringComparison]::OrdinalIgnoreCase)) { break }
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

function Invoke-ChezmoiNative {
    param([Parameter(Mandatory)] [string[]]$Arguments)

    $command = $script:Chezmoi
    if ([string]::IsNullOrWhiteSpace($command)) {
        $command = (Get-Command chezmoi -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    }
    $stderrPath = [IO.Path]::GetTempFileName()
    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    $output = @()
    $rc = 1
    try {
        try {
            if ($hasNativePreference) {
                $oldNativePreference = $PSNativeCommandUseErrorActionPreference
                $PSNativeCommandUseErrorActionPreference = $false
            }
            $global:LASTEXITCODE = 0
            $output = @(& $command @Arguments 2> $stderrPath)
            $rc = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
        } finally {
            if ($hasNativePreference) {
                $PSNativeCommandUseErrorActionPreference = $oldNativePreference
            }
        }
        return [pscustomobject]@{
            ExitCode = $rc
            Output = @($output)
            Stderr = [IO.File]::ReadAllText($stderrPath)
        }
    } finally {
        Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
    }
}

function Test-ChezmoiTargetUnmodified {
    param(
        [Parameter(Mandatory)] [string]$Target,
        [string[]]$BaseArguments
    )
    # "Is this on-disk file still byte-for-byte what chezmoi would write?" answered
    # by the chezmoi verify command (exit 0 = matches managed state, nonzero =
    # drifted). This deliberately avoids hashing a `chezmoi cat > tmp` capture:
    # native-command redirection in PowerShell can re-encode / CRLF-translate on
    # Windows, which would make an unmodified copy look modified and wrongly leave
    # it behind. verify is byte-exact by the chezmoi logic itself. Expected
    # drift is a silent exit 1; stderr or any other nonzero result is an actual
    # invocation failure and must stop uninstall instead of being misclassified.
    if (-not $PSBoundParameters.ContainsKey('BaseArguments')) {
        $BaseArguments = if ($script:ManagedTargetContexts.ContainsKey($Target)) {
            @($script:ManagedTargetContexts[$Target])
        } else { @('--source', $SourceDir) }
    }
    $result = Invoke-ChezmoiNative -Arguments (@($BaseArguments) + @('--no-tty', 'verify', $Target))
    if ($result.ExitCode -eq 0) { return $true }
    if ($result.ExitCode -eq 1 -and $result.Output.Count -eq 0 -and [string]::IsNullOrWhiteSpace($result.Stderr)) {
        return $false
    }
    $detail = $result.Stderr.Trim()
    if (-not $detail) { $detail = 'no diagnostic text' }
    throw "chezmoi verify invocation failed for ${Target}: exit $($result.ExitCode): $detail"
}

function Test-WindowsTerminalSettingsPath {
    param([Parameter(Mandatory)] [string]$Path)
    return @((Get-WindowsTerminalRecoveryTargets) | Where-Object {
            (Resolve-CanonicalPath $_).Equals((Resolve-CanonicalPath $Path), [StringComparison]::OrdinalIgnoreCase)
        }).Count -gt 0
}

function Get-WindowsTerminalRecoveryTargets {
    $localAppData = Get-WindowsLocalApplicationData
    if (-not $localAppData) { return @() }
    return @(
        (Join-Path $localAppData 'Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json'),
        (Join-Path $localAppData 'Microsoft\Windows Terminal\settings.json')
    )
}

function Restore-WindowsTerminalSettingsBackups {
    if ($NoRestoreBackups) { return }
    $plans = @()
    # Validate every candidate name and backup JSON before touching either path.
    foreach ($target in @(Get-WindowsTerminalRecoveryTargets)) {
        $backup = Get-NewestBackup -Target $target
        if (-not $backup) { continue }
        $null = ([IO.File]::ReadAllText($backup) | ConvertFrom-Json -ErrorAction Stop)
        $plans += [pscustomobject]@{
            Target = $target
            Backup = $backup
            BackupHash = (Get-FileHash -LiteralPath $backup -Algorithm SHA256 -ErrorAction Stop).Hash
            CurrentHash = if (Test-Path -LiteralPath $target -PathType Leaf) {
                (Get-FileHash -LiteralPath $target -Algorithm SHA256 -ErrorAction Stop).Hash
            } else { $null }
        }
    }
    if ($plans.Count -eq 0) { return }
    if (-not (Confirm-Category -Prompt 'Restore pre-setup Windows Terminal settings from validated backups?')) {
        $script:Skipped++
        return
    }

    foreach ($plan in $plans) {
        if ($plan.CurrentHash -and $plan.CurrentHash -eq $plan.BackupHash) {
            Write-Host "  unchanged  $($plan.Target) already matches $($plan.Backup)"
            continue
        }
        $preserved = if ($plan.CurrentHash) {
            Get-UniqueRecoveryPath -Base "$($plan.Target).uninstall-current.$script:Timestamp"
        } else { $null }
        if ($DryRun) {
            if ($preserved) {
                Write-Host "  would: atomically restore $($plan.Backup) -> $($plan.Target); preserve current as $preserved"
            } else {
                Write-Host "  would: restore $($plan.Backup) -> $($plan.Target)"
            }
            continue
        }

        try {
            if ($preserved) {
                [IO.File]::Replace($plan.Backup, $plan.Target, $preserved)
            } else {
                $parent = Split-Path -Parent $plan.Target
                if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
                    New-Item -ItemType Directory -Force -Path $parent | Out-Null
                }
                [IO.File]::Move($plan.Backup, $plan.Target)
            }
            if ((Get-FileHash -LiteralPath $plan.Target -Algorithm SHA256 -ErrorAction Stop).Hash -ne $plan.BackupHash) {
                throw "restored bytes did not match the selected backup"
            }
            if ($preserved -and
                (Get-FileHash -LiteralPath $preserved -Algorithm SHA256 -ErrorAction Stop).Hash -ne $plan.CurrentHash) {
                throw "preserved current settings failed byte validation"
            }
            $script:Restored++
            Write-Host "  restored  $($plan.Target) <- $($plan.Backup)"
            if ($preserved) { Write-Host "             pre-uninstall settings preserved at $preserved" }
        } catch {
            throw "Windows Terminal restoration failed for $($plan.Target): $($_.Exception.Message). Backup=$($plan.Backup) preserved-current=$preserved"
        }
    }
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
        # Both packaged and portable recovery are handled together after the
        # ordinary chezmoi target pass so candidate validation is all-or-none.
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
    $identity = $script:WindowsIdentity
    $stateRoot = Join-Path $identity.LocalApplicationData 'dotfiles\chezmoi-state'
    $overlayConfig = Join-Path $RepoRoot 'windows\chezmoi-overlay.toml'
    $states = @(
        [pscustomobject]@{
            Label = 'profile root'
            Arguments = @('--source', $SourceDir, '--destination', $identity.UserProfile)
        },
        [pscustomobject]@{
            Label = 'LocalApplicationData'
            Arguments = @(
                '--source', (Join-Path $RepoRoot 'windows\chezmoi-localappdata'),
                '--destination', $identity.LocalApplicationData,
                '--persistent-state', (Join-Path $stateRoot 'localappdata.boltdb'),
                '--config', $overlayConfig,
                '--config-format', 'toml'
            )
        },
        [pscustomobject]@{
            Label = 'Documents profiles'
            Arguments = @(
                '--source', (Join-Path $RepoRoot 'windows\chezmoi-documents'),
                '--destination', $identity.Documents,
                '--persistent-state', (Join-Path $stateRoot 'documents.boltdb'),
                '--config', $overlayConfig,
                '--config-format', 'toml'
            )
        }
    )
    $targets = @()
    foreach ($state in $states) {
        $result = Invoke-ChezmoiNative -Arguments (@($state.Arguments) + @('managed', '--path-style', 'absolute'))
        if ($result.ExitCode -ne 0) {
            $text = $result.Stderr.Trim()
            throw "could not enumerate $($state.Label) managed targets. Re-run setup.ps1 before uninstall.`n$text"
        }
        foreach ($target in @($result.Output | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })) {
            $target = [string]$target
            $targets += $target
            $script:ManagedTargetContexts[$target] = @($state.Arguments)
        }
    }
    return @($targets | Sort-Object -Unique | Sort-Object Length -Descending)
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
    $profileRoot = if ($script:WindowsIdentity) { $script:WindowsIdentity.UserProfile } else { $env:USERPROFILE }
    $root = Join-Path $profileRoot '.local\share\dotfiles\zsh-plugins'
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

function Invoke-DotfilesUninstall {
    param([Parameter(Mandatory)] $Identity)
    $script:WindowsIdentity = $Identity
    Write-Host "uninstall.ps1: repo=$RepoRoot source=$SourceDir dry-run=$DryRun restore-backups=$(-not $NoRestoreBackups)"
    Write-Host

    $targets = @(Get-ManagedTargets)
    if ($targets.Count -gt 0 -and (Confirm-Category -Prompt 'Remove chezmoi-managed config targets?')) {
        foreach ($target in $targets) {
            Remove-ManagedTarget -Target $target
        }
    } else {
        $script:Skipped++
    }
    if (-not $DryRun) { Remove-EmptyDirs }
    Restore-WindowsTerminalSettingsBackups
    Remove-Externals

    Write-Host
    if ($script:Removed -eq 0 -and $script:Restored -eq 0 -and $script:DirsRemoved -eq 0 -and $script:ExternalsRemoved -eq 0) {
        Write-Host "uninstall.ps1: nothing to remove"
    }
    Write-Host ("summary: removed={0} restored={1} dirs_removed={2} externals_removed={3} skipped={4} warnings={5}" -f `
        $script:Removed, $script:Restored, $script:DirsRemoved, $script:ExternalsRemoved, $script:Skipped, $script:Warnings)
}

if ($env:DOTFILES_UNINSTALL_PS1_SOURCE_ONLY -eq '1') { return }

try {
    Invoke-DotfilesUninstall -Identity (Resolve-WindowsTargetIdentity)
} catch {
    Write-Host "FAIL: $($_.Exception.Message)"
    exit 1
}

# Explicit success exit: internal `chezmoi verify` probes leave $LASTEXITCODE=1
# when a target is user-modified/unmanaged, so without this the script would
# inherit that nonzero code on an otherwise-successful run.
exit 0
