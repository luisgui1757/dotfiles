# setup.ps1 -- one-shot end-to-end install for Windows.
#
# Local usage (from a checked-out copy):
#   .\setup.ps1                  interactive: dependency prompts, then config + sync
#   .\setup.ps1 -All             non-interactive: install or migrate, then reconcile everything
#   .\setup.ps1 -Update          reconcile the release, then refresh proven tools + Mason
#   .\setup.ps1 -Upgrade         alias for -Update
#   .\setup.ps1 -DryRun          preview every step
#   .\setup.ps1 -SkipDeps        already have nvim/starship; just config+sync
#   .\setup.ps1 -SkipBootstrap   back-compat alias: skip config apply
#   .\setup.ps1 -SkipConfig      already configured; just sync nvim
#   .\setup.ps1 -SkipNvim        skip nvim plugin/parser/Mason sync
#   .\setup.ps1 -SkipAgents      skip global Sentinel agent-policy install
#   .\setup.ps1 -BestEffort      continue past plugin/LSP/Mason phase failures
#   .\setup.ps1 -SkipWindowsTerminalMerge   config+sync but leave WT settings.json untouched
#   .\setup.ps1 -SkipLegacyKnownFolderMigration   retain v0.1 conventional targets during release rollback window
#   .\setup.ps1 -SkipConfigScripts   apply only chezmoi files/symlinks; release-migration boundary
#   .\setup.ps1 -MergeWindowsTerminal        (no-op alias; the WT rose-pine merge is now default-on)
#
# First run (no checkout yet):
#   git clone --branch v0.3.0 --single-branch https://github.com/luisgui1757/dotfiles.git "$env:USERPROFILE\dotfiles"
#   Set-Location "$env:USERPROFILE\dotfiles"
#   .\setup.ps1 -All
#
# Set DOTFILES_DEST to a different absolute path before cloning if you want a
# different checkout location.

[CmdletBinding()]
param(
    [switch]$All,
    [Alias('Upgrade')] [switch]$Update,
    [switch]$DryRun,
    [switch]$SkipDeps,
    [switch]$SkipBootstrap,
    [switch]$SkipConfig,
    [switch]$SkipNvim,
    [switch]$SkipAgents,
    [switch]$MergeWindowsTerminal,   # back-compat no-op: WT merge is now default-on
    [switch]$SkipWindowsTerminalMerge,
    [switch]$SkipLegacyKnownFolderMigration,
    [switch]$SkipConfigScripts,
    [switch]$BestEffort
)

$ErrorActionPreference = 'Stop'

$RepoUrl        = 'https://github.com/luisgui1757/dotfiles.git'
$ReleaseTag     = 'v0.3.0'
$SentinelRepoUrl = 'https://github.com/luisgui1757/sentinel.git'
$SentinelVersion = '0.1.2'
$SentinelRef     = 'ecafffa858666343c1639f996d177f460163e93e'
$V01Commit       = '015617362830280bf85c7142e69d0681d376d453'
$V01TagObject    = 'a3b4d6d7b6d289959cac68d76faec96219b3e310'
$script:CompletedV01Recovery = ''

function Get-DefaultProfileRoot {
    if ($env:OS -eq 'Windows_NT') {
        $knownProfile = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
        if ([string]::IsNullOrWhiteSpace($knownProfile)) {
            throw 'Windows UserProfile known folder could not be resolved'
        }
        return $knownProfile
    }
    if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) { return $env:USERPROFILE }
    if (-not [string]::IsNullOrWhiteSpace($env:HOME)) { return $env:HOME }
    return [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
}

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
    foreach ($name in 'UserProfile', 'LocalApplicationData', 'ApplicationData', 'MyDocuments') {
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
        ApplicationData = $resolved.ApplicationData
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

function Get-ScoopRoot {
    param([scriptblock]$ProfileRootResolver = { Get-DefaultProfileRoot })
    if (-not [string]::IsNullOrWhiteSpace($env:SCOOP)) { return $env:SCOOP }
    $profileRoot = [string](& $ProfileRootResolver)
    if ([string]::IsNullOrWhiteSpace($profileRoot)) { return '' }
    return (Join-Path $profileRoot 'scoop')
}

$DefaultDest = Join-Path (Get-DefaultProfileRoot) 'dotfiles'

# Rebuild PATH from registry values plus Scoop shims, then de-duplicate.
# This differs from setup.sh, which evaluates brew shellenv and appends Unix bin dirs.
function Update-RuntimePath {
    param([scriptblock]$ProfileRootResolver = { Get-DefaultProfileRoot })
    $parts = @()
    $scoopRoot = Get-ScoopRoot -ProfileRootResolver $ProfileRootResolver
    if (-not [string]::IsNullOrWhiteSpace($scoopRoot)) {
        $shimDir = Join-Path $scoopRoot 'shims'
        if (Test-Path -LiteralPath $shimDir) {
            $parts += $shimDir
        }
    }
    foreach ($scope in 'Machine', 'User') {
        $p = [Environment]::GetEnvironmentVariable('PATH', $scope)
        if ($p) { $parts += ($p -split ';') }
    }
    $parts += ($env:PATH -split ';')
    $seen = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    $env:PATH = ($parts | Where-Object { $_ -and $seen.Add($_) }) -join ';'
}

function Stop-SetupWithExitCode {
    param([int]$ExitCode)
    exit $ExitCode
}

function Invoke-DependencyInstallerOrFail {
    param(
        [scriptblock]$Runner,
        [string]$Path,
        [hashtable]$Arguments,
        [string]$Label = 'install-deps.ps1'
    )

    if (-not $Runner) {
        $Runner = {
            param([string]$InstallerPath, [hashtable]$InstallerArguments)
            & $InstallerPath @InstallerArguments
        }
    }

    $global:LASTEXITCODE = 0
    & $Runner $Path $Arguments
    $exitCode = $LASTEXITCODE
    if ($null -eq $exitCode) { $exitCode = 0 }
    if ([int]$exitCode -ne 0) {
        Write-Host ("  FAIL: {0} exited {1}; setup cannot continue after dependency install failures." -f $Label, $exitCode) -ForegroundColor Red
        Stop-SetupWithExitCode ([int]$exitCode)
    }
}

function Get-ReleaseGitValue {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string[]]$Arguments,
        [scriptblock]$Runner
    )
    if (-not $Runner) {
        $Runner = {
            param([string]$Path, [string[]]$GitArguments)
            $git = (Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
            $oldNativePreference = $null
            $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
            try {
                if ($hasNativePreference) {
                    $oldNativePreference = $PSNativeCommandUseErrorActionPreference
                    $PSNativeCommandUseErrorActionPreference = $false
                }
                $global:LASTEXITCODE = 0
                $output = @(& $git -C $Path `
                        -c core.fsmonitor=false `
                        -c core.untrackedCache=false `
                        -c core.hooksPath=NUL @GitArguments 2>$null)
                if ($LASTEXITCODE -ne 0) { return '' }
                return [string]($output | Select-Object -First 1)
            } finally {
                if ($hasNativePreference) {
                    $PSNativeCommandUseErrorActionPreference = $oldNativePreference
                }
                $global:LASTEXITCODE = 0
            }
        }
    }
    return [string](& $Runner $Checkout $Arguments)
}

function Test-ExactV01Checkout {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [scriptblock]$GitRunner
    )
    if (-not (Test-Path -LiteralPath $Path -PathType Container)) { return $false }
    $item = Get-Item -LiteralPath $Path -Force
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { return $false }
    $canonical = [IO.Path]::GetFullPath($item.FullName)
    $head = Get-ReleaseGitValue -Checkout $canonical -Arguments @('rev-parse', '--verify', 'HEAD^{commit}') -Runner $GitRunner
    $tag = Get-ReleaseGitValue -Checkout $canonical -Arguments @('rev-parse', '--verify', 'refs/tags/v0.1.0') -Runner $GitRunner
    return ($head -eq $V01Commit -and $tag -eq $V01TagObject)
}

function Get-V01CheckoutFromLiveConfig {
    param(
        [Parameter(Mandatory)] [pscustomobject]$Identity,
        [scriptblock]$GitRunner
    )
    $nvimPath = Join-Path $Identity.LocalApplicationData 'nvim'
    if (-not (Test-Path -LiteralPath $nvimPath)) { return '' }
    $item = Get-Item -LiteralPath $nvimPath -Force
    if (-not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { return '' }
    $target = @($item.Target) | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace([string]$target)) { return '' }
    $targetPath = [string]$target
    if (-not [IO.Path]::IsPathFullyQualified($targetPath)) {
        $targetPath = Join-Path (Split-Path -Parent $nvimPath) $targetPath
    }
    $candidate = [IO.Path]::GetFullPath((Split-Path -Parent $targetPath))
    if (Test-ExactV01Checkout -Path $candidate -GitRunner $GitRunner) {
        return $candidate
    }
    return ''
}

function Read-SetupRecoveryScalar {
    param([Parameter(Mandatory)] [string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "recovery file is missing: $Path" }
    $item = Get-Item -LiteralPath $Path -Force
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "recovery file is unsafe: $Path" }
    $raw = [IO.File]::ReadAllText($item.FullName)
    if ($raw -notmatch '\A[^\r\n]+\r?\n\z') {
        throw "recovery scalar is malformed: $Path"
    }
    return $raw.TrimEnd("`r", "`n")
}

function Get-PendingV01Recovery {
    param(
        [Parameter(Mandatory)] [pscustomobject]$Identity,
        [string]$CurrentCheckout = $ScriptDir
    )
    $root = Join-Path (Join-Path $Identity.LocalApplicationData 'dotfiles') 'migrations'
    if (-not (Test-Path -LiteralPath $root -PathType Container)) { return $null }
    $active = @()
    $rolledBack = @()
    foreach ($directory in @(Get-ChildItem -LiteralPath $root -Force -Filter 'v0.1.0-to-v0.2.0.*' -ErrorAction SilentlyContinue)) {
        if (-not $directory.PSIsContainer -or ($directory.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "legacy migration recovery path is not a real directory: $($directory.FullName)"
        }
        try {
            $stage = Read-SetupRecoveryScalar -Path (Join-Path $directory.FullName 'stage')
            $newCheckout = Read-SetupRecoveryScalar -Path (Join-Path $directory.FullName 'new-checkout')
            $oldCheckout = Read-SetupRecoveryScalar -Path (Join-Path $directory.FullName 'old-checkout')
        } catch {
            throw "legacy migration recovery identity is incomplete or unsafe: $($directory.FullName): $($_.Exception.Message)"
        }
        if ($stage -in @('prepared', 'applying', 'applied', 'rolling-back', 'recovery-required')) {
            throw "unfinished v0.2.0 migration must be resolved before v0.3.0 setup: recovery=$($directory.FullName); new-checkout=$newCheckout; old-checkout=$oldCheckout"
        }
        if ($stage -notin @('accepted', 'rolled-back')) {
            throw "legacy migration recovery stage is invalid: $($directory.FullName) ($stage)"
        }
    }
    foreach ($directory in @(Get-ChildItem -LiteralPath $root -Force -Filter 'v0.1.0-to-v0.3.0.*' -ErrorAction SilentlyContinue)) {
        if (-not $directory.PSIsContainer -or ($directory.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "migration recovery path is not a real directory: $($directory.FullName)"
        }
        try {
            $stage = Read-SetupRecoveryScalar -Path (Join-Path $directory.FullName 'stage')
            $newCheckout = Read-SetupRecoveryScalar -Path (Join-Path $directory.FullName 'new-checkout')
            $oldCheckout = Read-SetupRecoveryScalar -Path (Join-Path $directory.FullName 'old-checkout')
        } catch {
            throw "migration recovery identity is incomplete or unsafe: $($directory.FullName): $($_.Exception.Message)"
        }
        if ([IO.Path]::GetFullPath($newCheckout) -ine [IO.Path]::GetFullPath($CurrentCheckout)) { continue }
        $record = [pscustomobject]@{
            Recovery = $directory.FullName
            Stage = $stage
            OldCheckout = $oldCheckout
        }
        if ($stage -in @('prepared', 'applying', 'applied', 'rolling-back', 'recovery-required')) {
            $active += $record
        } elseif ($stage -eq 'rolled-back') {
            $rolledBack += $record
        } elseif ($stage -ne 'accepted') {
            throw "migration recovery stage is invalid: $($directory.FullName) ($stage)"
        }
    }
    if ($active.Count -gt 1) {
        throw "multiple unfinished v0.1.0 migrations target this checkout: $($active.Recovery -join ', ')"
    }
    if ($active.Count -eq 1) { return $active[0] }
    $oldPaths = @($rolledBack.OldCheckout | Sort-Object -Unique)
    if ($oldPaths.Count -gt 1) {
        throw 'rolled-back migrations disagree about the v0.1.0 checkout'
    }
    if ($oldPaths.Count -eq 1) {
        return [pscustomobject]@{ Recovery = ''; Stage = 'rolled-back'; OldCheckout = $oldPaths[0] }
    }
    return $null
}

function Invoke-SetupV01Migration {
    param(
        [Parameter(Mandatory)] [pscustomobject]$Identity,
        [bool]$AllMode = $All,
        [bool]$IsDryRun = $DryRun,
        [scriptblock]$MigrationRunner,
        [scriptblock]$GitRunner,
        [scriptblock]$Prompt = { (Read-Host 'Migrate the detected v0.1.0 installation and continue? [Y/n]') -notmatch '^(?:n|no)$' }
    )
    if ($env:DOTFILES_RELEASE_MIGRATION_ACTIVE) { return }
    if ($SkipDeps) { return }
    $upgradeScript = Join-Path $ScriptDir 'scripts\upgrade-v0.1.0.ps1'
    if (-not $MigrationRunner) {
        $MigrationRunner = {
            param([string]$Mode, [string]$Argument)
            $oldMarker = $env:DOTFILES_RELEASE_MIGRATION_ACTIVE
            try {
                $env:DOTFILES_RELEASE_MIGRATION_ACTIVE = '1'
                if ($Mode -eq 'Apply') {
                    & $upgradeScript -Apply -OldCheckout $Argument 6>&1
                } elseif ($Mode -eq 'Accept') {
                    & $upgradeScript -Accept $Argument 6>&1
                } else {
                    throw "unsupported setup migration mode: $Mode"
                }
            } finally {
                if ($null -eq $oldMarker) {
                    Remove-Item Env:DOTFILES_RELEASE_MIGRATION_ACTIVE -ErrorAction SilentlyContinue
                } else {
                    $env:DOTFILES_RELEASE_MIGRATION_ACTIVE = $oldMarker
                }
            }
        }
    }

    $pending = Get-PendingV01Recovery -Identity $Identity
    if ($pending -and $pending.Stage -eq 'applied') {
        if ($IsDryRun) {
            Write-Host "  would: verify and accept the pending v0.1.0 migration at $($pending.Recovery)"
            return
        }
        @(& $MigrationRunner 'Accept' $pending.Recovery) | ForEach-Object { Write-Host ([string]$_) }
        $script:CompletedV01Recovery = $pending.Recovery
        return
    }
    if ($pending -and $pending.Stage -in @('prepared', 'applying', 'rolling-back', 'recovery-required')) {
        throw "unfinished v0.1.0 migration requires recovery first: pwsh -NoProfile -File `"$(Join-Path $pending.Recovery 'upgrade-v0.1.0.ps1')`" -Rollback `"$($pending.Recovery)`""
    }

    $oldCheckout = if ($env:DOTFILES_V0_1_CHECKOUT) {
        if (-not (Test-ExactV01Checkout -Path $env:DOTFILES_V0_1_CHECKOUT -GitRunner $GitRunner)) {
            throw "DOTFILES_V0_1_CHECKOUT is not the exact v0.1.0 checkout: $env:DOTFILES_V0_1_CHECKOUT"
        }
        [IO.Path]::GetFullPath($env:DOTFILES_V0_1_CHECKOUT)
    } elseif ($pending -and $pending.Stage -eq 'rolled-back') {
        $pending.OldCheckout
    } else {
        Get-V01CheckoutFromLiveConfig -Identity $Identity -GitRunner $GitRunner
    }
    if ([string]::IsNullOrWhiteSpace($oldCheckout)) { return }
    if ($IsDryRun) {
        Write-Host "  would: transactionally migrate exact v0.1.0 state from $oldCheckout"
        return
    }
    if (-not $AllMode -and -not (& $Prompt)) {
        throw "v0.1.0 was retained unchanged at $oldCheckout"
    }
    $output = @(& $MigrationRunner 'Apply' $oldCheckout)
    $output | ForEach-Object { Write-Host ([string]$_) }
    $recoveryLines = @($output | ForEach-Object { [string]$_ } | Where-Object { $_ -match '^Recovery directory: ' })
    if ($recoveryLines.Count -ne 1) { throw 'migration succeeded without one recovery identity' }
    $recovery = $recoveryLines[0].Substring('Recovery directory: '.Length)
    if (-not (Test-Path -LiteralPath $recovery -PathType Container)) {
        throw "migration recovery directory is missing: $recovery"
    }
    @(& $MigrationRunner 'Accept' $recovery) | ForEach-Object { Write-Host ([string]$_) }
    $script:CompletedV01Recovery = $recovery
    Write-Host "  accepted  verified v0.1.0 core migration; recovery retained at $recovery"
}

function Get-VsWherePath {
    $programFilesX86 = ${env:ProgramFiles(x86)}
    if ([string]::IsNullOrWhiteSpace($programFilesX86)) { return '' }
    return (Join-Path $programFilesX86 'Microsoft Visual Studio\Installer\vswhere.exe')
}

function Get-VsBuildToolsInstallationPath {
    param([string]$VsWherePath = (Get-VsWherePath))

    if ([string]::IsNullOrWhiteSpace($VsWherePath)) { return '' }
    if (-not (Test-Path -LiteralPath $VsWherePath -PathType Leaf)) { return '' }

    try {
        $result = @(& $VsWherePath `
                -products * `
                -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
                -property installationPath 2>$null)
        if ($LASTEXITCODE -ne 0) { return '' }
        foreach ($line in $result) {
            $path = ([string]$line).Trim()
            if (-not [string]::IsNullOrWhiteSpace($path)) {
                return $path
            }
        }
    } catch {
        return ''
    }
    return ''
}

function Join-VsDevShellPath {
    param([Parameter(Mandatory)] [string]$InstallationPath)

    if ($InstallationPath -match '^[A-Za-z]:[\\/]') {
        return ($InstallationPath.TrimEnd([char[]]@('\', '/')) + '\Common7\Tools\Microsoft.VisualStudio.DevShell.dll')
    }
    return (Join-Path $InstallationPath 'Common7/Tools/Microsoft.VisualStudio.DevShell.dll')
}

function Enter-VsDeveloperEnvironment {
    param(
        # NOTE: do NOT name this $IsWindows -- that is a read-only automatic
        # variable in PowerShell 7, so a param of that name fails to bind with
        # "Cannot overwrite variable IsWindows because it is read-only".
        [bool]$OnWindows = ($env:OS -eq 'Windows_NT'),
        [scriptblock]$InstallationPathResolver = { Get-VsBuildToolsInstallationPath },
        [scriptblock]$ModulePathTester = { param([string]$Path) Test-Path -LiteralPath $Path -PathType Leaf },
        [scriptblock]$ModuleImporter = { param([string]$Path) Import-Module $Path -ErrorAction Stop },
        [scriptblock]$DevShellInvoker = {
            param([string]$InstallPath)
            Enter-VsDevShell -VsInstallPath $InstallPath -SkipAutomaticLocation -DevCmdArguments '-arch=x64 -host_arch=x64 -no_logo'
        }
    )

    if (-not $OnWindows) { return $false }

    $vsPath = [string](& $InstallationPathResolver)
    if ([string]::IsNullOrWhiteSpace($vsPath)) {
        Write-Host "  info      VS Build Tools not detected; nvim parser rebuilds may need Developer PowerShell for VS"
        return $false
    }

    $devShell = Join-VsDevShellPath -InstallationPath $vsPath
    if (-not (& $ModulePathTester $devShell)) {
        Write-Host "  FAIL: VS DevShell module missing at $devShell" -ForegroundColor Red
        return $false
    }

    try {
        & $ModuleImporter $devShell
        & $DevShellInvoker $vsPath
        Write-Host ("  ok        {0,-26} imported for nvim parser builds" -f "VS DevShell")
        return $true
    } catch {
        Write-Host ("  FAIL: VS DevShell import failed: " + $_.Exception.Message) -ForegroundColor Red
        return $false
    }
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
if ($Update) {
    $All = $true
    $PSBoundParameters['All'] = $true
}

# ---- Locate the repo ---------------------------------------------------------
# Piped/remote setup is intentionally disabled. Running from stdin would execute
# mutable default-branch code before a local checkout exists, so setup fails
# closed and tells the user how to clone first.
$ScriptDir = $null
if ($PSCommandPath -and (Test-Path -LiteralPath $PSCommandPath)) {
    $ScriptDir = Split-Path -Parent $PSCommandPath
}
if (-not $ScriptDir -or -not (Test-Path (Join-Path $ScriptDir 'home'))) {
    $dest = if ($env:DOTFILES_DEST) { $env:DOTFILES_DEST } else { $DefaultDest }
    if ($DryRun) {
        Write-Host "setup.ps1: no local checkout was detected; remote/piped setup is disabled."
        Write-Host "  Clone first, then run setup locally:"
        Write-Host "    git clone --branch $ReleaseTag --single-branch $RepoUrl `"$dest`""
        Write-Host "    Set-Location `"$dest`""
        Write-Host "    .\setup.ps1 -All"
        Write-Host "(dry run -- no clone, no install, no writes performed)"
    }
    Write-Error "setup.ps1 must be run from a local clone. Remote/piped clone-and-reinvoke setup is disabled because it would execute mutable default-branch code."
    Write-Error "Clone first, then run setup locally: git clone --branch $ReleaseTag --single-branch $RepoUrl `"$dest`"; Set-Location `"$dest`"; .\setup.ps1 -All"
    exit 1
}

Set-Location $ScriptDir

$WindowsTerminalTargetsLibrary = Join-Path $ScriptDir 'scripts\windows-terminal-targets.ps1'
if (-not (Test-Path -LiteralPath $WindowsTerminalTargetsLibrary -PathType Leaf)) {
    throw "Windows Terminal target library is missing: $WindowsTerminalTargetsLibrary"
}
. $WindowsTerminalTargetsLibrary

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

function Get-SentinelCacheRoot {
    $localAppData = Get-WindowsLocalApplicationData
    if (-not [string]::IsNullOrWhiteSpace($localAppData)) {
        return (Join-Path $localAppData 'dotfiles\sentinel')
    }
    return (Join-Path (Get-DefaultProfileRoot) '.local\share\dotfiles\sentinel')
}

function Get-SentinelCheckoutPath {
    param(
        [string]$CacheRoot = (Get-SentinelCacheRoot),
        [string]$Ref = $SentinelRef
    )
    return (Join-Path $CacheRoot $Ref)
}

function Invoke-SentinelGit {
    param(
        [Parameter(Mandatory)] [string[]]$Arguments,
        [switch]$SuppressStderr
    )

    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    $savedEnv = @{}
    $gitEnvNames = @(
        'GIT_CONFIG_NOSYSTEM',
        'GIT_CONFIG_SYSTEM',
        'GIT_CONFIG_GLOBAL',
        'GIT_CONFIG_COUNT',
        'GIT_CONFIG_PARAMETERS',
        'GIT_TEMPLATE_DIR'
    )
    foreach ($name in $gitEnvNames) {
        $savedEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    }

    $emptyGitConfig = [System.IO.Path]::GetTempFileName()
    $hooksPath = if ($env:OS -eq 'Windows_NT') { 'NUL' } else { '/dev/null' }
    try {
        if ($hasNativePreference) {
            $oldNativePreference = $PSNativeCommandUseErrorActionPreference
            $PSNativeCommandUseErrorActionPreference = $false
        }

        [Environment]::SetEnvironmentVariable('GIT_CONFIG_NOSYSTEM', '1', 'Process')
        [Environment]::SetEnvironmentVariable('GIT_CONFIG_SYSTEM', $emptyGitConfig, 'Process')
        [Environment]::SetEnvironmentVariable('GIT_CONFIG_GLOBAL', $emptyGitConfig, 'Process')
        [Environment]::SetEnvironmentVariable('GIT_CONFIG_COUNT', '0', 'Process')
        [Environment]::SetEnvironmentVariable('GIT_CONFIG_PARAMETERS', $null, 'Process')
        [Environment]::SetEnvironmentVariable('GIT_TEMPLATE_DIR', $null, 'Process')

        $gitArgs = @(
            '-c', 'core.fsmonitor=false',
            '-c', 'core.untrackedCache=false',
            '-c', "core.hooksPath=$hooksPath",
            '-c', 'init.templateDir='
        ) + $Arguments

        if ($SuppressStderr) {
            $output = @(& git @gitArgs 2>$null)
        } else {
            $output = @(& git @gitArgs)
        }
        $rc = $LASTEXITCODE
    } finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
        foreach ($name in $gitEnvNames) {
            [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process')
        }
        Remove-Item -LiteralPath $emptyGitConfig -Force -ErrorAction SilentlyContinue
    }
    return @{
        ExitCode = $rc
        Output = @($output)
    }
}

function Invoke-SentinelCacheGit {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string[]]$Arguments
    )

    $gitDir = Join-Path $Checkout '.git'
    $gitArgs = @("--git-dir=$gitDir", "--work-tree=$Checkout") + $Arguments
    return Invoke-SentinelGit -Arguments $gitArgs -SuppressStderr
}

function Assert-SentinelCheckoutClean {
    param([Parameter(Mandatory)] [string]$Checkout)

    $result = Invoke-SentinelCacheGit -Checkout $Checkout -Arguments @(
        'status',
        '--porcelain=v1',
        '--untracked-files=all',
        '--ignored=matching'
    )
    $status = @($result.Output)
    if ($result.ExitCode -ne 0) {
        Write-Host "  FAIL: could not inspect Sentinel cache worktree: $Checkout" -ForegroundColor Red
        exit 1
    }

    if ($status.Count -gt 0) {
        Write-Host "  FAIL: Sentinel cache has local changes; refusing to execute it: $Checkout" -ForegroundColor Red
        foreach ($line in $status) {
            Write-Host "        $line" -ForegroundColor Yellow
        }
        Write-Host "        Remove this cache directory and rerun setup to fetch the pinned checkout again." -ForegroundColor Yellow
        exit 1
    }
}

function Assert-SentinelArtifact {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string]$Version,
        [Parameter(Mandatory)] [string]$Ref
    )

    $headResult = Invoke-SentinelCacheGit -Checkout $Checkout -Arguments @('rev-parse', '--verify', 'HEAD^{commit}')
    $head = ([string]($headResult.Output -join '')).Trim()
    if ($headResult.ExitCode -ne 0 -or $head -ne $Ref) {
        Write-Host "  FAIL: Sentinel cache is not at the pinned commit: $Checkout" -ForegroundColor Red
        Write-Host "        expected $Ref, found $head" -ForegroundColor Yellow
        exit 1
    }

    $actualVersion = Get-SentinelVersionFromCheckout -Checkout $Checkout
    if ($actualVersion -ne $Version) {
        Write-Host "  FAIL: Sentinel cache VERSION mismatch: expected $Version, found $actualVersion" -ForegroundColor Red
        exit 1
    }
}

function Test-SentinelGitBashCommand {
    param([string]$Candidate)

    if ([string]::IsNullOrWhiteSpace($Candidate)) { return $false }
    if (-not (Test-Path -LiteralPath $Candidate -PathType Leaf)) { return $false }

    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        if ($hasNativePreference) {
            $oldNativePreference = $PSNativeCommandUseErrorActionPreference
            $PSNativeCommandUseErrorActionPreference = $false
        }
        & $Candidate --noprofile --norc -c 'command -v cygpath >/dev/null 2>&1'
        return ($LASTEXITCODE -eq 0)
    } finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
    }
}

function Get-SentinelBashCommand {
    if ($env:OS -ne 'Windows_NT') {
        $bash = Get-Command bash -ErrorAction SilentlyContinue
        if ($bash) { return $bash.Source }
        return $null
    }

    $candidates = @()
    $git = Get-Command git -ErrorAction SilentlyContinue
    if ($git) {
        $gitDir = Split-Path -Parent $git.Source
        $gitRoot = Split-Path -Parent $gitDir
        $candidates += @(
            (Join-Path $gitDir 'bash.exe'),
            (Join-Path $gitRoot 'bin\bash.exe'),
            (Join-Path $gitRoot 'usr\bin\bash.exe')
        )
    }

    $scoopRoot = if ($env:SCOOP) { $env:SCOOP } else { Join-Path (Get-DefaultProfileRoot) 'scoop' }
    $candidates += @(
        (Join-Path $scoopRoot 'apps\git\current\bin\bash.exe'),
        (Join-Path $scoopRoot 'apps\git\current\usr\bin\bash.exe')
    )

    foreach ($candidate in $candidates) {
        if (Test-SentinelGitBashCommand -Candidate $candidate) { return $candidate }
    }

    $bash = Get-Command bash -ErrorAction SilentlyContinue
    if ($bash -and (Test-SentinelGitBashCommand -Candidate $bash.Source)) {
        return $bash.Source
    }

    return $null
}

function ConvertTo-SentinelBashPath {
    param(
        [Parameter(Mandatory)] [string]$Bash,
        [Parameter(Mandatory)] [string]$Path
    )

    if ($env:OS -ne 'Windows_NT') { return $Path }

    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        if ($hasNativePreference) {
            $oldNativePreference = $PSNativeCommandUseErrorActionPreference
            $PSNativeCommandUseErrorActionPreference = $false
        }
        $converted = & $Bash --noprofile --norc -c 'cygpath -u "$1"' -- $Path
        $rc = $LASTEXITCODE
    } finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
    }
    if ($rc -ne 0 -or [string]::IsNullOrWhiteSpace($converted)) {
        Write-Host "  FAIL: could not convert path for Git Bash: $Path" -ForegroundColor Red
        exit 1
    }
    return ([string]$converted).Trim()
}

function Test-ShouldApplyAgentPolicy {
    param(
        [bool]$SkipAgentsPhase = $SkipAgents,
        [bool]$AllMode = $All,
        [bool]$IsDryRun = $DryRun,
        [scriptblock]$Prompt
    )

    if ($SkipAgentsPhase) { return $false }
    if ($AllMode -or $IsDryRun) { return $true }
    if (-not [Environment]::UserInteractive) { return $true }
    if (-not $Prompt) {
        $Prompt = {
            param([string]$Message)
            Read-Host "  $Message [Y/n]"
        }
    }
    $answer = [string](& $Prompt 'Apply Sentinel global agent rules?')
    return ([string]::IsNullOrWhiteSpace($answer) -or $answer -match '^(?i:y|yes)$')
}

function Get-SentinelVersionFromCheckout {
    param([Parameter(Mandatory)] [string]$Checkout)
    $versionPath = Join-Path $Checkout 'VERSION'
    if (-not (Test-Path -LiteralPath $versionPath -PathType Leaf)) { return '' }
    return ([System.IO.File]::ReadAllText($versionPath).Trim())
}

function Invoke-SentinelGitChecked {
    param(
        [Parameter(Mandatory)] [string[]]$Arguments,
        [Parameter(Mandatory)] [string]$Label
    )

    $result = Invoke-SentinelGit -Arguments $Arguments
    if ($result.ExitCode -ne 0) {
        Write-Host ("  FAIL: {0} exited {1}" -f $Label, $result.ExitCode) -ForegroundColor Red
        exit $result.ExitCode
    }
    return @($result.Output)
}

function Ensure-SentinelCheckout {
    param(
        [string]$RepoUrl = $SentinelRepoUrl,
        [string]$Version = $SentinelVersion,
        [string]$Ref = $SentinelRef,
        [string]$CacheRoot = (Get-SentinelCacheRoot)
    )

    $checkout = Get-SentinelCheckoutPath -CacheRoot $CacheRoot -Ref $Ref
    if (Test-Path -LiteralPath (Join-Path $checkout '.git') -PathType Container) {
        Assert-SentinelArtifact -Checkout $checkout -Version $Version -Ref $Ref
        Assert-SentinelCheckoutClean -Checkout $checkout
        return $checkout
    }

    if (Test-Path -LiteralPath $checkout) {
        Write-Host "  FAIL: Sentinel cache path exists but is not a git checkout: $checkout" -ForegroundColor Red
        exit 1
    }

    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Host "  FAIL: git is required to fetch Sentinel. Re-run without -SkipDeps, or install git first." -ForegroundColor Red
        exit 1
    }

    New-Item -ItemType Directory -Force -Path $CacheRoot | Out-Null
    $tmp = Join-Path $CacheRoot ('.tmp.' + [guid]::NewGuid().ToString('N'))
    try {
        Invoke-SentinelGitChecked -Label 'git clone Sentinel' -Arguments @('clone', $RepoUrl, $tmp)
        Invoke-SentinelGitChecked -Label 'git checkout Sentinel pin' -Arguments @('-C', $tmp, 'checkout', '--detach', $Ref)

        Assert-SentinelArtifact -Checkout $tmp -Version $Version -Ref $Ref
        Assert-SentinelCheckoutClean -Checkout $tmp

        Move-Item -LiteralPath $tmp -Destination $checkout
        return $checkout
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-SentinelInstallChecked {
    param(
        [Parameter(Mandatory)] [string]$Bash,
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string[]]$Arguments,
        [Parameter(Mandatory)] [string]$Label
    )

    $bashCheckout = ConvertTo-SentinelBashPath -Bash $Bash -Path $Checkout
    $bashCommand = if ($env:OS -eq 'Windows_NT') {
        # Keep Git Bash on its POSIX userland. A Windows-native jq.exe in PATH
        # emits CRLF records, which the Sentinel Bash manifest reader treats
        # as literal path bytes and then fails to find core/*.md.
        'export PATH=/usr/bin:/bin; cd "$1"; shift; exec bash tools/install "$@"'
    } else {
        'cd "$1"; shift; exec bash tools/install "$@"'
    }

    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        if ($hasNativePreference) {
            $oldNativePreference = $PSNativeCommandUseErrorActionPreference
            $PSNativeCommandUseErrorActionPreference = $false
        }
        & $Bash --noprofile --norc -c $bashCommand -- $bashCheckout @Arguments
        if ($LASTEXITCODE -ne 0) {
            Write-Host ("  FAIL: {0} exited {1}" -f $Label, $LASTEXITCODE) -ForegroundColor Red
            exit $LASTEXITCODE
        }
    } finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
    }
}

function Invoke-SentinelAgentPolicy {
    param(
        [bool]$SkipAgentsPhase = $SkipAgents,
        [bool]$AllMode = $All,
        [bool]$IsDryRun = $DryRun,
        [string]$RepoUrl = $SentinelRepoUrl,
        [string]$Version = $SentinelVersion,
        [string]$Ref = $SentinelRef,
        [string]$CacheRoot = (Get-SentinelCacheRoot),
        [scriptblock]$Prompt
    )

    if ($SkipAgentsPhase) {
        Write-Host ""
        Write-Host "skipped: Phase 6/6 (agent policy) via -SkipAgents"
        return
    }

    if (-not (Test-ShouldApplyAgentPolicy -SkipAgentsPhase:$SkipAgentsPhase -AllMode:$AllMode -IsDryRun:$IsDryRun -Prompt $Prompt)) {
        Write-Host ""
        Write-Host "skipped: Phase 6/6 (agent policy)"
        return
    }

    Phase "Phase 6/6: apply global agent policy (Sentinel)"
    $checkout = Get-SentinelCheckoutPath -CacheRoot $CacheRoot -Ref $Ref
    if ($IsDryRun) {
        Write-Step "would    clone/fetch Sentinel $Version (@ $Ref)"
        Write-Step "         into $checkout"
        Write-Step "would    run Sentinel tools/install --global, then --global --check"
        return
    }

    $checkout = Ensure-SentinelCheckout -RepoUrl $RepoUrl -Version $Version -Ref $Ref -CacheRoot $CacheRoot
    $installer = Join-Path $checkout 'tools\install'
    if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
        Write-Host "  FAIL: Sentinel installer missing: $installer" -ForegroundColor Red
        exit 1
    }
    $bash = Get-SentinelBashCommand
    if (-not $bash) {
        Write-Host "  FAIL: bash is required to run the Sentinel global installer. Install Git for Windows first." -ForegroundColor Red
        exit 1
    }

    Invoke-SentinelInstallChecked -Bash $bash -Checkout $checkout -Arguments @('--global') -Label 'Sentinel global install'
    Invoke-SentinelInstallChecked -Bash $bash -Checkout $checkout -Arguments @('--global', '--check') -Label 'Sentinel global check'
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
    # Compute the probe paths OUTSIDE the try so the finally can always clean up.
    # Previously the Remove-Item calls lived in the try body, so a throwing
    # New-SymbolicLinkItem left the probe target file orphaned in %TEMP% on every
    # failed probe (the common no-Developer-Mode case).
    $tmp = Join-Path $env:TEMP "symlink-probe-$([guid]::NewGuid())"
    $target = Join-Path $env:TEMP "symlink-probe-target-$([guid]::NewGuid())"
    try {
        New-Item -ItemType File -Path $target -Force | Out-Null
        New-SymbolicLinkItem -Source $target -Destination $tmp
        return $true
    } catch {
        return $false
    } finally {
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
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
    $result = Invoke-ChezmoiNative -Arguments $Arguments -PassThroughOutput
    if ($result.ExitCode -ne 0) {
        $detail = $result.Stderr.TrimEnd()
        if (-not [string]::IsNullOrWhiteSpace($detail)) {
            Write-Output $detail
        }
        Write-Host ("  FAIL: $Label exited $($result.ExitCode)") -ForegroundColor Red
        exit $result.ExitCode
    }
}

function Invoke-ChezmoiNative {
    param(
        [Parameter(Mandatory)] [string[]]$Arguments,
        [switch]$PassThroughOutput,
        [AllowNull()] [string]$InputText
    )

    $command = Get-Command chezmoi -CommandType Application -ErrorAction Stop | Select-Object -First 1
    $baseArgs = @($script:ChezmoiBaseArgs)
    $configArgs = @($script:ChezmoiConfigArgs)
    $stderrPath = [IO.Path]::GetTempFileName()
    $output = @()
    $rc = 1
    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        try {
            if ($hasNativePreference) {
                $oldNativePreference = $PSNativeCommandUseErrorActionPreference
                $PSNativeCommandUseErrorActionPreference = $false
            }
            $global:LASTEXITCODE = 0
            if ($PassThroughOutput) {
                if ($PSBoundParameters.ContainsKey('InputText')) {
                    $InputText | & $command.Source @baseArgs @configArgs @Arguments 2> $stderrPath | Out-Host
                } else {
                    & $command.Source @baseArgs @configArgs @Arguments 2> $stderrPath | Out-Host
                }
            } elseif ($PSBoundParameters.ContainsKey('InputText')) {
                $output = @($InputText | & $command.Source @baseArgs @configArgs @Arguments 2> $stderrPath)
            } else {
                $output = @(& $command.Source @baseArgs @configArgs @Arguments 2> $stderrPath)
            }
            $rc = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
        } finally {
            if ($hasNativePreference) {
                $PSNativeCommandUseErrorActionPreference = $oldNativePreference
            }
        }
        $stderrText = [IO.File]::ReadAllText($stderrPath)
        return [pscustomobject]@{
            ExitCode = $rc
            Output = @($output)
            Stderr = $stderrText
        }
    } finally {
        Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
        # Callers consume ExitCode explicitly. Do not leak a handled verify
        # drift into the surrounding PowerShell/GitHub process exit contract.
        $global:LASTEXITCODE = 0
    }
}

function Invoke-ChezmoiOutput {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    $result = Invoke-ChezmoiNative -Arguments $Arguments
    if ($result.ExitCode -ne 0) {
        $detail = $result.Stderr.Trim()
        if ($detail) { $detail = ": $detail" }
        throw "chezmoi $($Arguments -join ' ') exited $($result.ExitCode)$detail"
    }
    return @($result.Output)
}

function Test-ChezmoiVerify {
    param([Parameter(Mandatory)] [string]$Target)
    $result = Invoke-ChezmoiNative -Arguments @('verify', $Target)
    if ($result.ExitCode -eq 0) { return $true }
    if ($result.ExitCode -eq 1 -and $result.Output.Count -eq 0 -and [string]::IsNullOrWhiteSpace($result.Stderr)) {
        return $false
    }
    $detail = $result.Stderr.Trim()
    if (-not $detail) { $detail = 'no diagnostic text' }
    throw "chezmoi verify invocation failed for ${Target}: exit $($result.ExitCode): $detail"
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
    if ($linkType -in @('SymbolicLink', 'Junction')) {
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
    return (Get-DotfilesWindowsTerminalTargetDefinition `
            -LocalApplicationData (Get-WindowsLocalApplicationData) -Kind Packaged).Path
}

function Get-WindowsTerminalPreviewSettingsPath {
    return (Get-DotfilesWindowsTerminalTargetDefinition `
            -LocalApplicationData (Get-WindowsLocalApplicationData) -Kind Preview).Path
}

function Get-WindowsTerminalCanarySettingsPath {
    return (Get-DotfilesWindowsTerminalTargetDefinition `
            -LocalApplicationData (Get-WindowsLocalApplicationData) -Kind Canary).Path
}

function Get-WindowsTerminalUnpackagedSettingsPath {
    return (Get-DotfilesWindowsTerminalTargetDefinition `
            -LocalApplicationData (Get-WindowsLocalApplicationData) -Kind Portable).Path
}

function Get-WindowsTerminalSettingsFragmentPath {
    return (Join-Path $ScriptDir 'windows-terminal\settings.fragment.jsonc')
}

function Get-WindowsTerminalMergeHelperPath {
    return (Join-Path $ScriptDir 'home\.chezmoitemplates\windows-terminal\merge-settings.ps1')
}

function Test-WindowsTerminalUnpackagedPresent {
    param([string]$LocalApplicationData = (Get-WindowsLocalApplicationData))
    $localAppData = $LocalApplicationData
    if (-not $localAppData) {
        return $false
    }

    $unpackagedSettings = (Get-DotfilesWindowsTerminalTargetDefinition `
            -LocalApplicationData $localAppData -Kind Portable).Path
    $unpackagedDir = Split-Path -Parent $unpackagedSettings
    if (Test-Path -LiteralPath $unpackagedDir -PathType Container) {
        return $true
    }

    $portableRoot = Join-Path $localAppData 'Programs\WindowsTerminal'
    foreach ($candidate in @(
            (Join-Path $portableRoot 'wt.exe'),
            (Join-Path $portableRoot 'WindowsTerminal.exe')
        )) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return $true
        }
    }

    $wtCommand = Get-Command wt -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($wtCommand) {
        $wtPath = if ($wtCommand.PSObject.Properties.Name -contains 'Source') { $wtCommand.Source } else { $wtCommand.Path }
        if ($wtPath -and (Test-Path -LiteralPath $wtPath -PathType Leaf)) {
            $wtDir = Split-Path -Parent $wtPath
            $fullWtDir = [System.IO.Path]::GetFullPath($wtDir).TrimEnd([char[]]@([char]92, [char]47))
            $fullPortableRoot = [System.IO.Path]::GetFullPath($portableRoot).TrimEnd([char[]]@([char]92, [char]47))
            if ($fullWtDir.StartsWith($fullPortableRoot, [StringComparison]::OrdinalIgnoreCase)) {
                return $true
            }
            if (Test-Path -LiteralPath (Join-Path $wtDir 'WindowsTerminal.exe') -PathType Leaf) {
                return $true
            }
        }
    }

    return $false
}

function Write-WindowsTerminalSettingsJson {
    param(
        [Parameter(Mandatory)] [string]$SettingsPath,
        [Parameter(Mandatory)] [string]$Json
    )
    $settingsDir = Split-Path -Parent $SettingsPath
    if (-not (Test-Path -LiteralPath $settingsDir -PathType Container)) {
        New-Item -ItemType Directory -Force -Path $settingsDir | Out-Null
    }
    [System.IO.File]::WriteAllText($SettingsPath, $Json, [System.Text.UTF8Encoding]::new($false))
}

function Get-WindowsTerminalContentSha256 {
    param([Parameter(Mandatory)] [string]$Content)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes($Content)
        return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '')
    } finally {
        $sha.Dispose()
    }
}

function Get-WindowsTerminalFileSha256 {
    param([Parameter(Mandatory)] [string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256 -ErrorAction Stop).Hash
}

function Merge-WindowsTerminalFragmentFile {
    param(
        [Parameter(Mandatory)] [string]$SettingsPath,
        [Parameter(Mandatory)] [string]$FragmentPath
    )

    # Dot-source the shared merge helper INTO this function scope. Dot-sourcing it
    # inside a separate helper function would load these functions into THAT scope
    # and lose them on return, leaving the merge functions undefined here.
    $helper = Get-WindowsTerminalMergeHelperPath
    if (-not (Test-Path -LiteralPath $helper -PathType Leaf)) {
        throw "Windows Terminal merge helper is missing: $helper"
    }
    . $helper
    $fragmentJson = [System.IO.File]::ReadAllText($FragmentPath)
    $fragment = ConvertFrom-WindowsTerminalJsonc -Jsonc $fragmentJson
    if (Test-Path -LiteralPath $SettingsPath -PathType Leaf) {
        $currentJson = [System.IO.File]::ReadAllText($SettingsPath)
        $current = ConvertFrom-WindowsTerminalJsonc -Jsonc $currentJson
        $merged = Merge-WindowsTerminalSettingsObject -Current $current -Fragment $fragment
        return ($merged | ConvertTo-Json -Depth 100)
    }
    return ($fragment | ConvertTo-Json -Depth 100)
}

function Get-WindowsTerminalMergeTargets {
    param(
        [bool]$IsPortablePresent = (Test-WindowsTerminalUnpackagedPresent)
    )
    foreach ($target in @(Get-DotfilesWindowsTerminalTargets `
            -LocalApplicationData (Get-WindowsLocalApplicationData) `
            -PortablePresent $IsPortablePresent)) {
        Write-Output $target.Path
    }
}

function New-WindowsTerminalMergePlan {
    param(
        [Parameter(Mandatory)] [string]$SettingsPath,
        [Parameter(Mandatory)] [string]$FragmentPath,
        [bool]$IsDryRun = $false
    )
    $stagePath = $null
    try {
        $existed = Test-Path -LiteralPath $SettingsPath -PathType Leaf
        $sourceHash = if ($existed) { Get-WindowsTerminalFileSha256 -Path $SettingsPath } else { $null }
        $json = Merge-WindowsTerminalFragmentFile -SettingsPath $SettingsPath -FragmentPath $FragmentPath
        # Parse the exact staged representation before any backup or publication.
        $null = $json | ConvertFrom-Json -ErrorAction Stop
        $stageHash = Get-WindowsTerminalContentSha256 -Content $json
        $changed = (-not $existed) -or ($stageHash -ne $sourceHash)
        if ($changed -and -not $IsDryRun) {
            $settingsDir = Split-Path -Parent $SettingsPath
            if (-not (Test-Path -LiteralPath $settingsDir -PathType Container)) {
                New-Item -ItemType Directory -Force -Path $settingsDir -ErrorAction Stop | Out-Null
            }
            $stagePath = Join-Path $settingsDir ('.{0}.dotfiles-stage.{1}.tmp' -f (Split-Path -Leaf $SettingsPath), [Guid]::NewGuid().ToString('N'))
            Write-WindowsTerminalSettingsJson -SettingsPath $stagePath -Json $json
            $null = ([System.IO.File]::ReadAllText($stagePath) | ConvertFrom-Json -ErrorAction Stop)
            if ((Get-WindowsTerminalFileSha256 -Path $stagePath) -ne $stageHash) {
                throw "staged Windows Terminal settings did not match the validated merge output: $stagePath"
            }
        }
        return [pscustomobject]@{
            Target = $SettingsPath
            Existed = [bool]$existed
            SourceHash = $sourceHash
            StageHash = $stageHash
            StagePath = $stagePath
            Changed = [bool]$changed
            BackupPath = $null
            RollbackPath = $null
            Published = $false
            RecoveryRequired = $false
        }
    } catch {
        if ($stagePath) { Remove-Item -LiteralPath $stagePath -Force -ErrorAction SilentlyContinue }
        throw
    }
}

function Backup-WindowsTerminalMergePlan {
    param([Parameter(Mandatory)] $Plan)
    if (-not $Plan.Changed -or -not $Plan.Existed) { return }
    if (-not (Test-Path -LiteralPath $Plan.Target -PathType Leaf) -or
        (Get-WindowsTerminalFileSha256 -Path $Plan.Target) -ne $Plan.SourceHash) {
        throw "Windows Terminal settings changed while the merge was staged: $($Plan.Target)"
    }
    $backup = Get-UniqueBackupPath "$($Plan.Target).bak.$Timestamp"
    Copy-Item -LiteralPath $Plan.Target -Destination $backup -ErrorAction Stop
    if ((Get-WindowsTerminalFileSha256 -Path $backup) -ne $Plan.SourceHash) {
        Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
        throw "Windows Terminal backup verification failed: $backup"
    }
    $Plan.BackupPath = $backup
    Write-Step "backed up $($Plan.Target) -> $backup"
}

function Test-WindowsTerminalMergePlanSourceUnchanged {
    param([Parameter(Mandatory)] $Plan)
    if ($Plan.Existed) {
        return (Test-Path -LiteralPath $Plan.Target -PathType Leaf) -and
            ((Get-WindowsTerminalFileSha256 -Path $Plan.Target) -eq $Plan.SourceHash)
    }
    return (-not (Test-Path -LiteralPath $Plan.Target))
}

function Publish-WindowsTerminalSettingsStage {
    param(
        [Parameter(Mandatory)] [string]$StagePath,
        [Parameter(Mandatory)] [string]$TargetPath,
        [AllowNull()] [string]$RollbackPath,
        [Parameter(Mandatory)] [bool]$TargetExisted
    )
    if ($TargetExisted) {
        [System.IO.File]::Replace($StagePath, $TargetPath, $RollbackPath)
    } else {
        [System.IO.File]::Move($StagePath, $TargetPath)
    }
}

function Restore-WindowsTerminalMergePlan {
    param([Parameter(Mandatory)] $Plan)
    if (-not $Plan.Published) { return $true }
    try {
        if (-not (Test-Path -LiteralPath $Plan.Target -PathType Leaf) -or
            (Get-WindowsTerminalFileSha256 -Path $Plan.Target) -ne $Plan.StageHash) {
            $Plan.RecoveryRequired = $true
            return $false
        }
        if ($Plan.Existed) {
            if (-not (Test-Path -LiteralPath $Plan.RollbackPath -PathType Leaf)) {
                $Plan.RecoveryRequired = $true
                return $false
            }
            $failedOutput = Get-UniqueBackupPath "$($Plan.Target).dotfiles-failed.$Timestamp"
            [System.IO.File]::Replace($Plan.RollbackPath, $Plan.Target, $failedOutput)
            Remove-Item -LiteralPath $failedOutput -Force -ErrorAction SilentlyContinue
        } else {
            Remove-Item -LiteralPath $Plan.Target -Force -ErrorAction Stop
        }
        $Plan.Published = $false
        return $true
    } catch {
        $Plan.RecoveryRequired = $true
        return $false
    }
}

function Invoke-WindowsTerminalSettingsTransaction {
    # Parameters are explicit so Pester can drive dry-run/skip/presence without
    # relying on dot-source scope behavior. BeforePublish is a deterministic
    # concurrency injection seam; production never supplies it.
    param(
        [bool]$IsDryRun = $DryRun,
        [bool]$IsSkipMerge = $SkipWindowsTerminalMerge,
        [bool]$IsPortablePresent = (Test-WindowsTerminalUnpackagedPresent),
        [scriptblock]$BeforePublish
    )
    if ($IsSkipMerge) {
        Write-Step "skip     Windows Terminal settings merge via -SkipWindowsTerminalMerge"
        return
    }

    $fragmentPath = Get-WindowsTerminalSettingsFragmentPath
    if (-not (Test-Path -LiteralPath $fragmentPath -PathType Leaf)) {
        throw "Windows Terminal settings fragment is missing: $fragmentPath"
    }
    $targets = @(Get-WindowsTerminalMergeTargets -IsPortablePresent $IsPortablePresent)
    if ($targets.Count -eq 0) { return }

    $mutexMaterial = (($targets | ForEach-Object { [IO.Path]::GetFullPath($_).ToLowerInvariant() }) -join '|')
    $mutexName = 'Dotfiles.WindowsTerminal.Settings.' + (Get-WindowsTerminalContentSha256 -Content $mutexMaterial)
    $mutex = New-Object System.Threading.Mutex($false, $mutexName)
    $acquired = $false
    $plans = @()
    $completed = $false
    try {
        try {
            $acquired = $mutex.WaitOne([TimeSpan]::FromSeconds(30))
        } catch [System.Threading.AbandonedMutexException] {
            $acquired = $true
        }
        if (-not $acquired) {
            throw "timed out waiting for another Windows Terminal settings transaction"
        }

        foreach ($target in $targets) {
            $plans += New-WindowsTerminalMergePlan -SettingsPath $target -FragmentPath $fragmentPath -IsDryRun $IsDryRun
        }
        foreach ($plan in $plans) {
            if (-not $plan.Changed) {
                Write-Step "ok       Windows Terminal settings already merged: $($plan.Target)"
                continue
            }
            if ($IsDryRun) {
                if ($plan.Existed) { Write-Step "would    backup $($plan.Target); then merge atomically" }
                else { Write-Step "would    seed Windows Terminal settings atomically: $($plan.Target)" }
            } else {
                Backup-WindowsTerminalMergePlan -Plan $plan
            }
        }
        if ($IsDryRun) {
            $completed = $true
            return
        }
        if ($BeforePublish) { & $BeforePublish $plans }
        foreach ($plan in $plans) {
            if (-not $plan.Changed) { continue }
            if (-not (Test-WindowsTerminalMergePlanSourceUnchanged -Plan $plan)) {
                throw "Windows Terminal settings changed concurrently before publication: $($plan.Target)"
            }
            if ($plan.Existed) {
                $plan.RollbackPath = Join-Path (Split-Path -Parent $plan.Target) ('.{0}.dotfiles-rollback.{1}.tmp' -f (Split-Path -Leaf $plan.Target), [Guid]::NewGuid().ToString('N'))
            }
            Publish-WindowsTerminalSettingsStage -StagePath $plan.StagePath -TargetPath $plan.Target -RollbackPath $plan.RollbackPath -TargetExisted $plan.Existed
            $plan.Published = $true

            # File.Replace atomically captures the exact pre-publication bytes.
            # Comparing that rollback file closes the final check/replace race.
            if ($plan.Existed -and (Get-WindowsTerminalFileSha256 -Path $plan.RollbackPath) -ne $plan.SourceHash) {
                $null = Restore-WindowsTerminalMergePlan -Plan $plan
                throw "Windows Terminal settings changed concurrently during publication: $($plan.Target)"
            }
            if ((Get-WindowsTerminalFileSha256 -Path $plan.Target) -ne $plan.StageHash) {
                throw "published Windows Terminal settings failed byte validation: $($plan.Target)"
            }
            $null = ([System.IO.File]::ReadAllText($plan.Target) | ConvertFrom-Json -ErrorAction Stop)
            Write-Step "merged   Windows Terminal settings atomically: $($plan.Target)"
        }
        $completed = $true
    } catch {
        $cause = $_.Exception.Message
        $recovery = @()
        for ($i = $plans.Count - 1; $i -ge 0; $i--) {
            $plan = $plans[$i]
            if (-not (Restore-WindowsTerminalMergePlan -Plan $plan)) {
                $paths = @($plan.BackupPath, $plan.RollbackPath) | Where-Object { $_ }
                $recovery += "$($plan.Target) (recovery: $($paths -join ', '))"
            }
        }
        if ($recovery.Count -gt 0) {
            throw "Windows Terminal settings transaction failed: $cause. Automatic rollback was unsafe or failed for: $($recovery -join '; '). Preserve those files and restore deliberately before retrying."
        }
        throw "Windows Terminal settings transaction failed: $cause. Every published target was restored; fix the error and retry setup."
    } finally {
        foreach ($plan in $plans) {
            if ($plan.StagePath) { Remove-Item -LiteralPath $plan.StagePath -Force -ErrorAction SilentlyContinue }
            if ($completed -and $plan.RollbackPath) {
                Remove-Item -LiteralPath $plan.RollbackPath -Force -ErrorAction SilentlyContinue
            }
        }
        if ($acquired) { $mutex.ReleaseMutex() }
        $mutex.Dispose()
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
    $localAppData = Get-WindowsLocalApplicationData
    if (-not $localAppData) { return }

    $nvimTarget = Join-Path $localAppData 'nvim'
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
    $template = Get-Content -Raw -LiteralPath $templatePath
    $result = Invoke-ChezmoiNative -Arguments @('execute-template', '--init') -InputText $template
    if ($result.ExitCode -ne 0) {
        throw "chezmoi execute-template --init failed while preparing dry-run config: $($result.Stderr.Trim())"
    }
    $rendered = @($result.Output) -join [Environment]::NewLine
    Set-Content -LiteralPath $tmp.FullName -Value $rendered -Encoding UTF8
    return $tmp.FullName
}

function Get-WindowsKnownFolderOverlays {
    param([Parameter(Mandatory)] $Identity)
    $stateRoot = Join-Path $Identity.LocalApplicationData 'dotfiles\chezmoi-state'
    return @(
        [pscustomobject]@{
            Label = 'LocalApplicationData'
            Source = Join-Path $ScriptDir 'windows\chezmoi-localappdata'
            Destination = $Identity.LocalApplicationData
            State = Join-Path $stateRoot 'localappdata.boltdb'
        },
        [pscustomobject]@{
            Label = 'ApplicationData'
            Source = Join-Path $ScriptDir 'windows\chezmoi-appdata'
            Destination = $Identity.ApplicationData
            State = Join-Path $stateRoot 'appdata.boltdb'
        },
        [pscustomobject]@{
            Label = 'Documents profiles'
            Source = Join-Path $ScriptDir 'windows\chezmoi-documents'
            Destination = $Identity.Documents
            State = Join-Path $stateRoot 'documents.boltdb'
        }
    )
}

function Get-WindowsPowerShellProfileCandidate {
    param([Parameter(Mandatory)] $Identity)

    return @(
        (Join-Path $Identity.Documents 'PowerShell\Microsoft.PowerShell_profile.ps1'),
        (Join-Path $Identity.Documents 'PowerShell\Microsoft.VSCode_profile.ps1'),
        (Join-Path $Identity.Documents 'WindowsPowerShell\Microsoft.PowerShell_profile.ps1'),
        (Join-Path $Identity.Documents 'WindowsPowerShell\Microsoft.PowerShellISE_profile.ps1')
    )
}

function Assert-WindowsKnownFolderConsumption {
    param([Parameter(Mandatory)] $Identity)

    $nvimTarget = Join-Path $Identity.LocalApplicationData 'nvim'
    $lazygitTarget = Join-Path $Identity.LocalApplicationData 'lazygit\config.yml'
    $herdrTarget = Join-Path $Identity.ApplicationData 'herdr\config.toml'
    $expectedNvim = Join-Path $ScriptDir 'nvim'
    $expectedLazygit = Join-Path $ScriptDir 'lazygit\config.windows.yml'
    $expectedHerdr = Join-Path $ScriptDir 'herdr\config.windows.toml'
    if (-not (Test-SamePath (Get-RealExistingPath $nvimTarget) (Get-RealExistingPath $expectedNvim))) {
        throw "Neovim does not consume the repo config through actual LocalApplicationData: $nvimTarget"
    }
    if (-not (Test-SamePath (Get-RealExistingPath $lazygitTarget) (Get-RealExistingPath $expectedLazygit)) -and
        -not (Test-FileBytesEqual $lazygitTarget $expectedLazygit)) {
        throw "lazygit does not consume the repo config through actual LocalApplicationData: $lazygitTarget"
    }
    if (-not (Test-SamePath (Get-RealExistingPath $herdrTarget) (Get-RealExistingPath $expectedHerdr)) -and
        -not (Test-FileBytesEqual $herdrTarget $expectedHerdr)) {
        throw "Herdr does not consume the repo config through actual ApplicationData: $herdrTarget"
    }

    $profileCandidates = @(Get-WindowsPowerShellProfileCandidate -Identity $Identity)
    if (-not @($profileCandidates | Where-Object { Test-SamePath $_ $Identity.RuntimeProfile }).Count) {
        throw "unsupported PowerShell host profile path: $($Identity.RuntimeProfile). Expected a supported profile under actual Documents."
    }
    $expectedProfile = Join-Path $ScriptDir 'shells\powershell_profile.ps1'
    if (-not (Test-SamePath (Get-RealExistingPath $Identity.RuntimeProfile) (Get-RealExistingPath $expectedProfile)) -and
        -not (Test-FileBytesEqual $Identity.RuntimeProfile $expectedProfile)) {
        throw "the runtime PowerShell profile does not consume the repo profile: $($Identity.RuntimeProfile)"
    }
}

function Unblock-WindowsPowerShellProfile {
    param(
        [Parameter(Mandatory)] $Identity,
        [string]$ExpectedProfile = (Join-Path $ScriptDir 'shells\powershell_profile.ps1'),
        [scriptblock]$Unblocker = {
            param([string]$Path)
            Unblock-File -LiteralPath $Path -ErrorAction Stop
        }
    )

    $expectedReal = Get-RealExistingPath $ExpectedProfile
    if (-not $expectedReal -or -not (Test-Path -LiteralPath $expectedReal -PathType Leaf)) {
        throw "repo PowerShell profile source is missing: $ExpectedProfile"
    }

    $seen = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    foreach ($candidate in @(Get-WindowsPowerShellProfileCandidate -Identity $Identity)) {
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { continue }
        $candidateReal = Get-RealExistingPath $candidate
        $target = if ($candidateReal -and (Test-SamePath $candidateReal $expectedReal)) {
            $expectedReal
        } elseif (Test-FileBytesEqual $candidate $ExpectedProfile) {
            Get-FullPathSafe $candidate
        } else {
            throw "refusing to unblock a PowerShell profile that is not repo-owned: $candidate"
        }
        if (-not $seen.Add($target)) { continue }

        $wasBlocked = $false
        if ($env:OS -eq 'Windows_NT') {
            $wasBlocked = $null -ne (Get-Item -LiteralPath $target -Stream Zone.Identifier -ErrorAction SilentlyContinue)
        }
        & $Unblocker $target
        if ($wasBlocked) {
            Write-Step "unblocked managed PowerShell profile: $target"
        }
    }
}

function Move-LegacyWindowsKnownFolderTargets {
    param(
        [Parameter(Mandatory)] $Identity,
        [bool]$IsDryRun = $DryRun,
        [bool]$IsSuppressed = $SkipLegacyKnownFolderMigration
    )

    if ($IsSuppressed) {
        Write-Step 'retain    v0.1.0 conventional known-folder targets until release acceptance'
        return
    }

    $legacyLocal = Join-Path $Identity.UserProfile 'AppData\Local'
    $legacyDocuments = Join-Path $Identity.UserProfile 'Documents'
    $plans = @(
        [pscustomobject]@{
            Legacy = Join-Path $legacyLocal 'nvim'
            Actual = Join-Path $Identity.LocalApplicationData 'nvim'
            Expected = Join-Path $ScriptDir 'nvim'
            Directory = $true
        },
        [pscustomobject]@{
            Legacy = Join-Path $legacyLocal 'lazygit\config.yml'
            Actual = Join-Path $Identity.LocalApplicationData 'lazygit\config.yml'
            Expected = Join-Path $ScriptDir 'lazygit\config.windows.yml'
            Directory = $false
        },
        [pscustomobject]@{
            Legacy = Join-Path $legacyDocuments 'PowerShell\Microsoft.PowerShell_profile.ps1'
            Actual = Join-Path $Identity.Documents 'PowerShell\Microsoft.PowerShell_profile.ps1'
            Expected = Join-Path $ScriptDir 'shells\powershell_profile.ps1'
            Directory = $false
        }
    )
    foreach ($plan in $plans) {
        if (Test-SamePath $plan.Legacy $plan.Actual) { continue }
        if (-not (Get-Item -LiteralPath $plan.Legacy -Force -ErrorAction SilentlyContinue)) { continue }
        $owned = if ($plan.Directory) {
            Test-SamePath (Get-RealExistingPath $plan.Legacy) (Get-RealExistingPath $plan.Expected)
        } else {
            (Test-SamePath (Get-RealExistingPath $plan.Legacy) (Get-RealExistingPath $plan.Expected)) -or
                (Test-FileBytesEqual $plan.Legacy $plan.Expected)
        }
        if (-not $owned) {
            Write-Warning "preserving divergent legacy target for manual migration: $($plan.Legacy)"
            continue
        }
        $backup = Get-UniqueBackupPath "$($plan.Legacy).legacy.$Timestamp"
        if ($IsDryRun) {
            Write-Step "would    preserve legacy target $($plan.Legacy) -> $backup after known-folder apply"
        } else {
            Move-Item -LiteralPath $plan.Legacy -Destination $backup -ErrorAction Stop
            Write-Step "migrated legacy target $($plan.Legacy) -> $backup"
        }
    }
}

function Invoke-WindowsKnownFolderOverlays {
    param(
        [Parameter(Mandatory)] $Identity,
        [bool]$IsDryRun = $DryRun,
        [bool]$ExcludeScripts = $SkipConfigScripts
    )

    $oldBaseArgs = $script:ChezmoiBaseArgs
    $oldConfigArgs = $script:ChezmoiConfigArgs
    $overlayConfig = Join-Path $ScriptDir 'windows\chezmoi-overlay.toml'
    foreach ($overlay in @(Get-WindowsKnownFolderOverlays -Identity $Identity)) {
        if (-not $IsDryRun) {
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $overlay.State) | Out-Null
        }
        try {
            $script:ChezmoiBaseArgs = @(
                '--source', $overlay.Source,
                '--destination', $overlay.Destination,
                '--persistent-state', $overlay.State
            )
            $script:ChezmoiConfigArgs = @('--config', $overlayConfig, '--config-format', 'toml')
            Backup-PreexistingManagedTargets
            $arguments = if ($IsDryRun) {
                @('--dry-run', '--verbose', 'apply')
            } else {
                @('--no-tty', '--force', 'apply')
            }
            if ($ExcludeScripts) { $arguments += @('--include', 'files,symlinks') }
            Invoke-ChezmoiOrExit -Label "chezmoi $($overlay.Label) apply" -Arguments $arguments
        } finally {
            $script:ChezmoiBaseArgs = $oldBaseArgs
            $script:ChezmoiConfigArgs = $oldConfigArgs
        }
    }
    if (-not $IsDryRun) {
        Assert-WindowsKnownFolderConsumption -Identity $Identity
        Unblock-WindowsPowerShellProfile -Identity $Identity
    }
    Move-LegacyWindowsKnownFolderTargets -Identity $Identity -IsDryRun:$IsDryRun
}

function Invoke-RetiredPiThemeAliasRetirement {
    param([Parameter(Mandatory)] $Identity)

    $retiredThemes = [ordered]@{
        'rose-pine-fable' = @(
            '36d25cc144bc38ab849ec5f47f839c8aa8a8946416557c5e14939183fff56805',
            '45813d7827fbe091f2029f8e0bfccb0927d1923576ebfb94cebb192b5235953c'
        )
        'rose-pine-moon-fable' = @(
            '9f33de93c8749e2fc79831e07b175bda5018e08261372fb4e1b4b507408b4ad9',
            '2f18ee6657d6748d13b13287760494e05d4fefad3d10c824becafaf6210c3bf0'
        )
        'rose-pine-dawn-fable' = @(
            'f0a8f234c826b37998c3035178b47265c167aa9f1ae8f896f2bf81eeb48f256a',
            '57adc5fe3252ed4511d79c31beb9ed46cee6b4d3946fbdc47c71e4bdc094bad8'
        )
    }
    foreach ($themeName in $retiredThemes.Keys) {
        $path = Join-Path $Identity.UserProfile ".pi\agent\themes\$themeName.json"
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { continue }
        $actualHash = (Get-FileHash -LiteralPath $path -Algorithm SHA256 -ErrorAction Stop).Hash
        if ($retiredThemes[$themeName] -contains $actualHash) {
            Remove-Item -LiteralPath $path -Force -ErrorAction Stop
            Write-Step "removed  retired Pi theme alias $path"
        } else {
            Write-Step "kept     modified retired-name Pi theme file $path"
        }
    }
}

function Invoke-PiThemeSelectionMerge {
    param(
        [Parameter(Mandatory)] $Identity,
        [bool]$IsDryRun = $DryRun,
        [bool]$ExcludeScripts = $SkipConfigScripts
    )
    if ($ExcludeScripts) {
        Write-Step 'deferred  Pi theme selection (-SkipConfigScripts)'
        return
    }
    if ($IsDryRun) {
        Write-Step 'would    default Pi to rose-pine, preserve a managed variant, and retire recognized trial aliases'
        return
    }

    $managedThemes = @('rose-pine', 'rose-pine-moon', 'rose-pine-dawn')
    foreach ($themeName in $managedThemes) {
        $theme = Join-Path $Identity.UserProfile ".pi\agent\themes\$themeName.json"
        $expectedTheme = Join-Path $ScriptDir "pi\$themeName.json"
        if (-not (Test-FileBytesEqual $theme $expectedTheme)) {
            throw "deployed Pi Rose Pine theme differs from the reviewed source: $theme"
        }
    }
    Invoke-RetiredPiThemeAliasRetirement -Identity $Identity
    $node = Get-Command node -ErrorAction SilentlyContinue
    if (-not $node) {
        throw "node is required to merge Pi's global theme setting"
    }

    $settings = Join-Path $Identity.UserProfile '.pi\agent\settings.json'
    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        if ($hasNativePreference) {
            $oldNativePreference = $PSNativeCommandUseErrorActionPreference
            $PSNativeCommandUseErrorActionPreference = $false
        }
        & $node.Source (Join-Path $ScriptDir 'scripts\configure-pi-theme.mjs') set $settings @managedThemes
        $rc = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    } finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
    }
    if ($rc -ne 0) {
        throw "Pi theme settings merge exited $rc"
    }
}

function Invoke-ChezmoiApplyPhase {
    param(
        [bool]$ExcludeScripts = $SkipConfigScripts,
        [bool]$IsDryRun = $DryRun
    )
    if (-not (Get-Command chezmoi -ErrorAction SilentlyContinue)) {
        if ($IsDryRun) {
            # The dogfood dry-run runs BEFORE Phase 1 installs chezmoi; preview
            # rather than fail (a real run has chezmoi on PATH after Phase 1).
            Write-Step "would    chezmoi (installed in Phase 1) backs up divergent configs, then 'chezmoi apply'"
            return
        }
        Write-Host "  FAIL: chezmoi is not on PATH after Phase 1" -ForegroundColor Red
        Write-Host "        Re-run without -SkipDeps, or install chezmoi first." -ForegroundColor Yellow
        exit 1
    }

    if ($IsDryRun) {
        Write-Warning "DryRun: skipping symlink-privilege probe"
        $dryRunConfig = New-ChezmoiDryRunConfig
        $script:ChezmoiConfigArgs = @('--config', $dryRunConfig, '--config-format', 'toml')
        try {
            Backup-PreexistingManagedTargets
            # setup owns Windows Terminal publication, and the main chezmoi
            # source exposes no WT target. Apply the complete source directly;
            # absolute Windows target lists are not a portable chezmoi selector.
            $applyArgs = @('--dry-run', '--verbose', 'apply')
            if ($ExcludeScripts) { $applyArgs += @('--include', 'files,symlinks') }
            Invoke-ChezmoiOrExit -Label 'chezmoi dry-run apply' -Arguments $applyArgs
            Invoke-WindowsKnownFolderOverlays -Identity $script:WindowsIdentity `
                -IsDryRun $true -ExcludeScripts:$ExcludeScripts
            Invoke-WindowsTerminalSettingsTransaction -IsDryRun $true
            Invoke-PiThemeSelectionMerge -Identity $script:WindowsIdentity `
                -IsDryRun $true -ExcludeScripts:$ExcludeScripts
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

    if (-not $ExcludeScripts) {
        Invoke-ChezmoiOrExit -Label 'chezmoi init' -Arguments @('init')
    }
    Backup-PreexistingManagedTargets
    # setup owns the transactional WT write. The main source has no WT target,
    # so a full apply cannot publish it before the transaction below.
    $realApplyArgs = @('--no-tty', '--force', 'apply')
    if ($ExcludeScripts) { $realApplyArgs += @('--include', 'files,symlinks') }
    Invoke-ChezmoiOrExit -Label 'chezmoi apply' -Arguments $realApplyArgs
    Invoke-WindowsKnownFolderOverlays -Identity $script:WindowsIdentity `
        -ExcludeScripts:$ExcludeScripts
    Invoke-WindowsTerminalSettingsTransaction
    Invoke-PiThemeSelectionMerge -Identity $script:WindowsIdentity -ExcludeScripts:$ExcludeScripts
}

function Invoke-NvimCommandOrFail {
    param(
        [string]$Label,
        [scriptblock]$Block,
        [bool]$IsBestEffort = $BestEffort
    )
    # Callers and CI can explicitly enable native error promotion. Isolate that
    # preference while we inspect the nvim exit code ourselves, and always
    # restore the caller setting even when the command throws.
    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        if ($hasNativePreference) {
            $oldNativePreference = $PSNativeCommandUseErrorActionPreference
            $PSNativeCommandUseErrorActionPreference = $false
        }
        & $Block
        $rc = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    } finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
    }
    if ($rc -ne 0) {
        if ($IsBestEffort) {
            Write-Warning ("  $Label exited $rc (continuing because -BestEffort is set)")
            return
        }
        Write-Host ("  FAIL: $Label exited $rc") -ForegroundColor Red
        Write-Host  "        Re-run with -BestEffort to continue past plugin/LSP failures." -ForegroundColor Yellow
        exit $rc
    }
}

function Invoke-NvimSyncPhases {
    param(
        [bool]$SkipNvimPhase = $SkipNvim,
        [bool]$IsDryRun = $DryRun,
        [scriptblock]$CommandTester = { param([string]$Name) [bool](Get-Command $Name -ErrorAction SilentlyContinue) },
        [scriptblock]$DevEnvironmentEntrypoint = { Enter-VsDeveloperEnvironment },
        [scriptblock]$LazyRunner = { & nvim --headless "+Lazy! restore" "+qa" },
        [scriptblock]$TreesitterRunner = {
            $oldSyncInstall = $env:DOTFILES_TREESITTER_SYNC_INSTALL
            try {
                $env:DOTFILES_TREESITTER_SYNC_INSTALL = '1'
                & nvim --headless "+lua require('lazy').load({ plugins = { 'nvim-treesitter' } })" "+qa"
            } finally {
                if ($null -eq $oldSyncInstall) {
                    Remove-Item Env:DOTFILES_TREESITTER_SYNC_INSTALL -ErrorAction SilentlyContinue
                } else {
                    $env:DOTFILES_TREESITTER_SYNC_INSTALL = $oldSyncInstall
                }
            }
        },
        [scriptblock]$MasonRunner = { & nvim --headless "+lua require('util.mason_tools').run_checked('MasonToolsInstallSync')" }
    )

    if (-not $SkipNvimPhase -and -not $IsDryRun) {
        if (& $CommandTester 'nvim') {
            & $DevEnvironmentEntrypoint | Out-Null
            Phase "Phase 3/6: restore Neovim plugins (lazy.nvim)"
            Invoke-NvimCommandOrFail "Lazy restore" $LazyRunner

            Phase "Phase 4/6: install Tree-sitter parsers"
            Write-Host "  this compiles nvim-treesitter parsers and can take several minutes."
            Invoke-NvimCommandOrFail "Tree-sitter parser install" $TreesitterRunner

            Phase "Phase 5/6: install LSP servers + formatters (Mason)"
            Write-Host "  this can take 3-8 minutes on a fresh Windows machine."
            Invoke-NvimCommandOrFail "Mason install" $MasonRunner
        } else {
            Write-Host ""
            Write-Host "skipped: Phase 3-5 (nvim plugins/parsers/tools) -- nvim not on PATH yet."
            Write-Host "         Open a new shell so PATH refreshes, then run:"
            Write-Host "             .\setup.ps1 -SkipDeps -SkipConfig"
        }
    } elseif ($IsDryRun) {
        Write-Host ""
        Write-Host "skipped: Phase 3-5 (nvim plugins/parsers/tools) in -DryRun mode"
    } else {
        Write-Host ""
        Write-Host "skipped: Phase 3-5 (nvim plugins/parsers/tools) via -SkipNvim"
    }
}

function Invoke-SetupUpdateMode {
    param(
        [string]$Root = $ScriptDir,
        [hashtable]$DependencyArgs = $depsArgs,
        [bool]$IsDryRun = $DryRun,
        [bool]$SkipDependencyPhase = $SkipDeps,
        [bool]$SkipNvimPhase = $SkipNvim,
        [bool]$IsBestEffort = $BestEffort,
        [scriptblock]$DependencyRunner,
        [scriptblock]$CommandTester,
        [scriptblock]$NvimRunner
    )

    if (-not $DependencyRunner) {
        $DependencyRunner = {
            param([string]$Path, [hashtable]$Arguments)
            & $Path @Arguments
        }
    }
    if (-not $CommandTester) {
        $CommandTester = {
            param([string]$Name)
            return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
        }
    }
    if (-not $NvimRunner) {
        $NvimRunner = {
            & nvim --headless "+lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
        }
    }

    if (-not $SkipDependencyPhase) {
        Phase "Update 1/2: update package-manager tools"
        $argsForDeps = @{}
        foreach ($key in $DependencyArgs.Keys) {
            $argsForDeps[$key] = $DependencyArgs[$key]
        }
        $argsForDeps['Update'] = $true
        if ($IsDryRun) { $argsForDeps['DryRun'] = $true }
        Invoke-DependencyInstallerOrFail `
            -Runner $DependencyRunner `
            -Path (Join-Path $Root 'install-deps.ps1') `
            -Arguments $argsForDeps `
            -Label 'install-deps.ps1 -Update'
    } else {
        Write-Host ""
        Write-Host "skipped: update dependency phase via -SkipDeps"
    }

    if (-not $IsDryRun) {
        Update-RuntimePath
    }

    if (-not $SkipNvimPhase) {
        Phase "Update 2/2: update Mason LSP servers + formatters"
        if ($IsDryRun) {
            Write-Host "  would: nvim --headless +lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
        } elseif (& $CommandTester 'nvim') {
            Invoke-NvimCommandOrFail -Label "Mason update" -IsBestEffort $IsBestEffort -Block $NvimRunner
        } else {
            Write-Host "  skipped   Mason update: nvim not on PATH"
        }
    } else {
        Write-Host ""
        Write-Host "skipped: Mason update via -SkipNvim"
    }

    Write-Host ""
    Write-Host 'The checked-out release, pinned plugins, configs, and missing tools were reconciled before this scoped refresh.'
    Write-Host ""
    Write-Host "================================================================"
    Write-Host "==  setup.ps1: update done"
    Write-Host "================================================================"
    if ($IsDryRun) { Write-Host "(dry run -- nothing was actually installed or changed)" }
}

# Test seam: set DOTFILES_SETUP_PS1_SOURCE_ONLY and dot-source this file to load
# helper functions without running install, config, or Neovim sync phases.
if ($env:DOTFILES_SETUP_PS1_SOURCE_ONLY) { return }

$script:WindowsIdentity = Resolve-WindowsTargetIdentity
$script:ChezmoiBaseArgs = @('--source', $HomeSource, '--destination', $script:WindowsIdentity.UserProfile)
Stop-NvimSelfLinkIfNeeded
Invoke-SetupV01Migration -Identity $script:WindowsIdentity

# ---- Phase 1: dependencies ---------------------------------------------------
if (-not $SkipDeps) {
    Phase "Phase 1/6: install dependencies"
    Invoke-DependencyInstallerOrFail `
        -Path (Join-Path $ScriptDir 'install-deps.ps1') `
        -Arguments $depsArgs `
        -Label 'install-deps.ps1'
} else {
    Write-Host ""
    Write-Host "skipped: Phase 1 (deps) via -SkipDeps"
}

# Phase 1 may install nvim and tools into locations not yet on this process PATH.
# A child installer cannot mutate our PATH, and persistent PATH edits only reach
# new shells. Re-derive PATH so Phase 3-5 can find nvim.
if (-not $DryRun) {
    Update-RuntimePath
}

# ---- Phase 2: apply configs --------------------------------------------------
if (-not ($SkipBootstrap -or $SkipConfig)) {
    Phase "Phase 2/6: apply configs with chezmoi"
    $global:LASTEXITCODE = 0   # reset so a stale code from Phase 1 cannot false-trip
    Invoke-ChezmoiApplyPhase
} else {
    Write-Host ""
    Write-Host "skipped: Phase 2 (config) via -SkipBootstrap/-SkipConfig"
}

# ---- Phases 3-5: nvim sync ---------------------------------------------------
#
# Lazy + Mason failures are FATAL by default. Pass -BestEffort to downgrade
# them to warnings (useful for offline / proxy-restricted environments where
# you accept a partial install and will run :Lazy / :Mason interactively).
Invoke-NvimSyncPhases

Invoke-SentinelAgentPolicy

if ($Update) {
    Invoke-SetupUpdateMode
}

# ---- Summary -----------------------------------------------------------------
Write-Host ""
Write-Host "================================================================"
Write-Host "==  setup.ps1: done"
Write-Host "================================================================"
Write-Host ""
Write-Host "Repo:    $ScriptDir"
if (-not [string]::IsNullOrWhiteSpace($script:CompletedV01Recovery)) {
    Write-Host "Upgrade: v0.1.0 migrated and verified; recovery retained at"
    Write-Host "         $($script:CompletedV01Recovery)"
}
Write-Host "Try it:  nvim  (then <Space>fg for live grep, :wnf to save w/o format)"
Write-Host ""
Write-Host "Note:    open a NEW PowerShell window so starship + newly-installed"
Write-Host '         tools pick up PATH (or run  . $PROFILE  in this one) -- this'
Write-Host "         shell started before they were installed, so its prompt is"
Write-Host "         not themed yet."
Write-Host ""
if ($DryRun) { Write-Host "(dry run -- nothing was actually installed or changed)" }
