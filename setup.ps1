# setup.ps1 -- one-shot end-to-end install for Windows.
#
# Local usage (from a checked-out copy):
#   .\setup.ps1                  interactive: dependency prompts, then config + sync
#   .\setup.ps1 -All             non-interactive: install everything missing
#   .\setup.ps1 -Update          update package-manager tools + Mason only
#   .\setup.ps1 -DryRun          preview every step
#   .\setup.ps1 -SkipDeps        already have nvim/starship; just config+sync
#   .\setup.ps1 -SkipBootstrap   back-compat alias: skip config apply
#   .\setup.ps1 -SkipConfig      already configured; just sync nvim
#   .\setup.ps1 -SkipNvim        skip nvim plugin/parser/Mason sync
#   .\setup.ps1 -SkipAgents      skip global Polaris agent-policy install
#   .\setup.ps1 -BestEffort      continue past plugin/LSP/Mason phase failures
#   .\setup.ps1 -SkipWindowsTerminalMerge   config+sync but leave WT settings.json untouched
#   .\setup.ps1 -MergeWindowsTerminal        (no-op alias; the WT rose-pine merge is now default-on)
#
# First run (no checkout yet):
#   git clone https://github.com/luisgui1757/dotfiles.git "$env:USERPROFILE\dotfiles"
#   Set-Location "$env:USERPROFILE\dotfiles"
#   .\setup.ps1 -All
#
# Set DOTFILES_DEST to a different absolute path before cloning if you want a
# different checkout location.

[CmdletBinding()]
param(
    [switch]$All,
    [switch]$Update,
    [switch]$DryRun,
    [switch]$SkipDeps,
    [switch]$SkipBootstrap,
    [switch]$SkipConfig,
    [switch]$SkipNvim,
    [switch]$SkipAgents,
    [switch]$MergeWindowsTerminal,   # back-compat no-op: WT merge is now default-on
    [switch]$SkipWindowsTerminalMerge,
    [switch]$BestEffort
)

$ErrorActionPreference = 'Stop'

$RepoUrl        = 'https://github.com/luisgui1757/dotfiles.git'
$PolarisRepoUrl = 'https://github.com/luisgui1757/polaris.git'
$PolarisVersion = '0.1.1'
$PolarisRef     = '489dcc6f991ddcff63c460a433e983264dc54cf7'

function Get-DefaultProfileRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) { return $env:USERPROFILE }
    if (-not [string]::IsNullOrWhiteSpace($env:HOME)) { return $env:HOME }
    return [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
}

function Get-ScoopRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:SCOOP)) { return $env:SCOOP }
    $profileRoot = Get-DefaultProfileRoot
    if ([string]::IsNullOrWhiteSpace($profileRoot)) { return '' }
    return (Join-Path $profileRoot 'scoop')
}

$DefaultDest = Join-Path (Get-DefaultProfileRoot) 'dotfiles'

# Rebuild PATH from registry values plus Scoop shims, then de-duplicate.
# This differs from setup.sh, which evaluates brew shellenv and appends Unix bin dirs.
function Update-RuntimePath {
    $parts = @()
    $scoopRoot = Get-ScoopRoot
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

# ---- Locate / clone the repo -------------------------------------------------
# When piped from `irm | iex` there is no $PSCommandPath, so we clone and
# re-invoke from the clone.
$ScriptDir = $null
if ($PSCommandPath -and (Test-Path -LiteralPath $PSCommandPath)) {
    $ScriptDir = Split-Path -Parent $PSCommandPath
}
if (-not $ScriptDir -or -not (Test-Path (Join-Path $ScriptDir 'home'))) {
    $dest = if ($env:DOTFILES_DEST) { $env:DOTFILES_DEST } else { $DefaultDest }
    if ($Update) {
        $existingSetup = Join-Path $dest 'setup.ps1'
        if ((Test-Path -LiteralPath $existingSetup -PathType Leaf) -and (Test-Path -LiteralPath (Join-Path $dest 'home') -PathType Container)) {
            Write-Host "setup.ps1 -Update: using existing checkout at $dest without git pull."
            & $existingSetup @PSBoundParameters
            exit $LASTEXITCODE
        }
        Write-Error "setup.ps1 -Update needs an existing checkout at $dest; it does not clone or pull."
        exit 1
    }
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
        if ($LASTEXITCODE -ne 0) {
            Write-Error "setup.ps1: 'git pull --ff-only' failed in $dest; refusing to run against a stale checkout."
            exit 1
        }
    } else {
        Write-Host "Cloning $RepoUrl -> $dest"
        git clone $RepoUrl $dest
        if ($LASTEXITCODE -ne 0) {
            Write-Error "setup.ps1: 'git clone' of $RepoUrl failed; cannot continue."
            exit 1
        }
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
if ($Update) { $depsArgs['Update'] = $true }
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

function Get-PolarisCacheRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        return (Join-Path $env:LOCALAPPDATA 'dotfiles\polaris')
    }
    return (Join-Path (Get-DefaultProfileRoot) '.local\share\dotfiles\polaris')
}

function Get-PolarisCheckoutPath {
    param(
        [string]$CacheRoot = (Get-PolarisCacheRoot),
        [string]$Ref = $PolarisRef
    )
    return (Join-Path $CacheRoot $Ref)
}

function Invoke-PolarisGit {
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

function Invoke-PolarisCacheGit {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string[]]$Arguments
    )

    $gitDir = Join-Path $Checkout '.git'
    $gitArgs = @("--git-dir=$gitDir", "--work-tree=$Checkout") + $Arguments
    return Invoke-PolarisGit -Arguments $gitArgs -SuppressStderr
}

function Assert-PolarisCheckoutClean {
    param([Parameter(Mandatory)] [string]$Checkout)

    $result = Invoke-PolarisCacheGit -Checkout $Checkout -Arguments @(
        'status',
        '--porcelain=v1',
        '--untracked-files=all',
        '--ignored=matching'
    )
    $status = @($result.Output)
    if ($result.ExitCode -ne 0) {
        Write-Host "  FAIL: could not inspect Polaris cache worktree: $Checkout" -ForegroundColor Red
        exit 1
    }

    if ($status.Count -gt 0) {
        Write-Host "  FAIL: Polaris cache has local changes; refusing to execute it: $Checkout" -ForegroundColor Red
        foreach ($line in $status) {
            Write-Host "        $line" -ForegroundColor Yellow
        }
        Write-Host "        Remove this cache directory and rerun setup to fetch the pinned checkout again." -ForegroundColor Yellow
        exit 1
    }
}

function Test-PolarisGitBashCommand {
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

function Get-PolarisBashCommand {
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
        if (Test-PolarisGitBashCommand -Candidate $candidate) { return $candidate }
    }

    $bash = Get-Command bash -ErrorAction SilentlyContinue
    if ($bash -and (Test-PolarisGitBashCommand -Candidate $bash.Source)) {
        return $bash.Source
    }

    return $null
}

function ConvertTo-PolarisBashPath {
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
    $answer = [string](& $Prompt 'Apply Polaris global agent rules?')
    return ([string]::IsNullOrWhiteSpace($answer) -or $answer -match '^(?i:y|yes)$')
}

function Get-PolarisVersionFromCheckout {
    param([Parameter(Mandatory)] [string]$Checkout)
    $versionPath = Join-Path $Checkout 'VERSION'
    if (-not (Test-Path -LiteralPath $versionPath -PathType Leaf)) { return '' }
    return ([System.IO.File]::ReadAllText($versionPath).Trim())
}

function Invoke-PolarisGitChecked {
    param(
        [Parameter(Mandatory)] [string[]]$Arguments,
        [Parameter(Mandatory)] [string]$Label
    )

    $result = Invoke-PolarisGit -Arguments $Arguments
    if ($result.ExitCode -ne 0) {
        Write-Host ("  FAIL: {0} exited {1}" -f $Label, $result.ExitCode) -ForegroundColor Red
        exit $result.ExitCode
    }
    return @($result.Output)
}

function Ensure-PolarisCheckout {
    param(
        [string]$RepoUrl = $PolarisRepoUrl,
        [string]$Version = $PolarisVersion,
        [string]$Ref = $PolarisRef,
        [string]$CacheRoot = (Get-PolarisCacheRoot)
    )

    $checkout = Get-PolarisCheckoutPath -CacheRoot $CacheRoot -Ref $Ref
    if (Test-Path -LiteralPath (Join-Path $checkout '.git') -PathType Container) {
        $headResult = Invoke-PolarisCacheGit -Checkout $checkout -Arguments @('rev-parse', '--verify', 'HEAD^{commit}')
        $head = ([string]($headResult.Output -join '')).Trim()
        if ($headResult.ExitCode -ne 0 -or $head -ne $Ref) {
            Write-Host "  FAIL: Polaris cache is not at the pinned commit: $checkout" -ForegroundColor Red
            Write-Host "        expected $Ref, found $head" -ForegroundColor Yellow
            exit 1
        }
        $actualVersion = Get-PolarisVersionFromCheckout -Checkout $checkout
        if ($actualVersion -ne $Version) {
            Write-Host "  FAIL: Polaris cache VERSION mismatch: expected $Version, found $actualVersion" -ForegroundColor Red
            exit 1
        }
        Assert-PolarisCheckoutClean -Checkout $checkout
        return $checkout
    }

    if (Test-Path -LiteralPath $checkout) {
        Write-Host "  FAIL: Polaris cache path exists but is not a git checkout: $checkout" -ForegroundColor Red
        exit 1
    }

    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Host "  FAIL: git is required to fetch Polaris. Re-run without -SkipDeps, or install git first." -ForegroundColor Red
        exit 1
    }

    New-Item -ItemType Directory -Force -Path $CacheRoot | Out-Null
    $tmp = Join-Path $CacheRoot ('.tmp.' + [guid]::NewGuid().ToString('N'))
    try {
        Invoke-PolarisGitChecked -Label 'git clone Polaris' -Arguments @('clone', $RepoUrl, $tmp)
        Invoke-PolarisGitChecked -Label 'git checkout Polaris pin' -Arguments @('-C', $tmp, 'checkout', '--detach', $Ref)

        $actualVersion = Get-PolarisVersionFromCheckout -Checkout $tmp
        if ($actualVersion -ne $Version) {
            Write-Host "  FAIL: fetched Polaris VERSION mismatch: expected $Version, found $actualVersion" -ForegroundColor Red
            exit 1
        }
        Assert-PolarisCheckoutClean -Checkout $tmp

        Move-Item -LiteralPath $tmp -Destination $checkout
        return $checkout
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-PolarisInstallChecked {
    param(
        [Parameter(Mandatory)] [string]$Bash,
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string[]]$Arguments,
        [Parameter(Mandatory)] [string]$Label
    )

    $bashCheckout = ConvertTo-PolarisBashPath -Bash $Bash -Path $Checkout
    $bashCommand = if ($env:OS -eq 'Windows_NT') {
        # Keep Git Bash on its POSIX userland. A Windows-native jq.exe in PATH
        # emits CRLF records, which the Polaris 0.1.1 Bash manifest reader treats
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

function Invoke-PolarisAgentPolicy {
    param(
        [bool]$SkipAgentsPhase = $SkipAgents,
        [bool]$AllMode = $All,
        [bool]$IsDryRun = $DryRun,
        [string]$RepoUrl = $PolarisRepoUrl,
        [string]$Version = $PolarisVersion,
        [string]$Ref = $PolarisRef,
        [string]$CacheRoot = (Get-PolarisCacheRoot),
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

    Phase "Phase 6/6: apply global agent policy (Polaris)"
    $checkout = Get-PolarisCheckoutPath -CacheRoot $CacheRoot -Ref $Ref
    if ($IsDryRun) {
        Write-Step "would    clone/fetch Polaris $Version ($Ref)"
        Write-Step "         into $checkout"
        Write-Step "would    run Polaris tools/install --global, then --global --check"
        return
    }

    $checkout = Ensure-PolarisCheckout -RepoUrl $RepoUrl -Version $Version -Ref $Ref -CacheRoot $CacheRoot
    $installer = Join-Path $checkout 'tools\install'
    if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
        Write-Host "  FAIL: Polaris installer missing: $installer" -ForegroundColor Red
        exit 1
    }
    $bash = Get-PolarisBashCommand
    if (-not $bash) {
        Write-Host "  FAIL: bash is required to run the Polaris 0.1.1 global installer. Install Git for Windows first." -ForegroundColor Red
        exit 1
    }

    Invoke-PolarisInstallChecked -Bash $bash -Checkout $checkout -Arguments @('--global') -Label 'Polaris global install'
    Invoke-PolarisInstallChecked -Bash $bash -Checkout $checkout -Arguments @('--global', '--check') -Label 'Polaris global check'
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

function Get-WindowsTerminalSettingsFragmentPath {
    return (Join-Path $ScriptDir 'windows-terminal\settings.fragment.jsonc')
}

function Get-WindowsTerminalMergeHelperPath {
    return (Join-Path $ScriptDir 'home\.chezmoitemplates\windows-terminal\merge-settings.ps1')
}

function Test-WindowsTerminalUnpackagedPresent {
    if (-not $env:LOCALAPPDATA) {
        return $false
    }

    $unpackagedSettings = Get-WindowsTerminalUnpackagedSettingsPath
    $unpackagedDir = Split-Path -Parent $unpackagedSettings
    if (Test-Path -LiteralPath $unpackagedDir -PathType Container) {
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

function Copy-WindowsTerminalSettingsForUnpackaged {
    # Params default to the script switches but are overridable so tests can drive
    # the dry-run / skip paths directly -- Pester `Set-Variable -Scope Script` does
    # NOT reliably override how a dot-sourced function reads a script variable.
    param(
        [bool]$IsDryRun = $DryRun,
        [bool]$IsSkipMerge = $SkipWindowsTerminalMerge,
        [bool]$IsPortablePresent = (Test-WindowsTerminalUnpackagedPresent)
    )
    if ($IsSkipMerge) { return }

    $packagedSettings = Get-WindowsTerminalSettingsPath
    $unpackagedSettings = Get-WindowsTerminalUnpackagedSettingsPath

    if (Test-Path -LiteralPath $packagedSettings -PathType Leaf) {
        if ($IsDryRun) {
            Write-Step "would    mirror Windows Terminal settings to unpackaged path"
            return
        }
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
        return
    }

    if (-not $IsPortablePresent) { return }

    try {
        $fragmentPath = Get-WindowsTerminalSettingsFragmentPath
        if (-not (Test-Path -LiteralPath $fragmentPath -PathType Leaf)) {
            throw "Windows Terminal settings fragment is missing: $fragmentPath"
        }
        $action = if (Test-Path -LiteralPath $unpackagedSettings -PathType Leaf) { 'merge' } else { 'seed' }
        if ($IsDryRun) {
            Write-Step ("would    {0} Windows Terminal unpackaged settings from fragment" -f $action)
            return
        }
        $json = Merge-WindowsTerminalFragmentFile -SettingsPath $unpackagedSettings -FragmentPath $fragmentPath
        Write-WindowsTerminalSettingsJson -SettingsPath $unpackagedSettings -Json $json
        if ($action -eq 'seed') {
            Write-Step "seeded Windows Terminal unpackaged settings from fragment"
        } else {
            Write-Step "merged Windows Terminal unpackaged settings from fragment"
        }
    } catch {
        Write-Warning ("Could not seed or merge Windows Terminal unpackaged settings: " + $_.Exception.Message)
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
            Copy-WindowsTerminalSettingsForUnpackaged -IsDryRun $true
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

function Invoke-NvimCommandOrFail {
    param(
        [string]$Label,
        [scriptblock]$Block,
        [bool]$IsBestEffort = $BestEffort
    )
    # PowerShell 7.4+ defaults PSNativeCommandUseErrorActionPreference to true, which turns
    # nvim/Mason stderr (e.g. clang-format installing) or a non-zero exit into a terminating
    # NativeCommandError before we can inspect LASTEXITCODE. We do our own exit-code check
    # below, so disable that promotion here (function-local; reverts on return).
    $PSNativeCommandUseErrorActionPreference = $false
    & $Block
    $rc = $LASTEXITCODE
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
        [scriptblock]$MasonRunner = { & nvim --headless "+MasonToolsInstallSync" "+qa" }
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
            & nvim --headless "+MasonToolsUpdate" "+qa"
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
        & $DependencyRunner (Join-Path $Root 'install-deps.ps1') $argsForDeps
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
            Write-Host "  would: nvim --headless +MasonToolsUpdate +qa"
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
    Write-Host 'Plugins (lazy-lock.json), pinned binaries, and configs update via `git pull` then re-run setup; `:Lazy update` re-pins plugins (a repo change).'
    Write-Host ""
    Write-Host "================================================================"
    Write-Host "==  setup.ps1: update done"
    Write-Host "================================================================"
    if ($IsDryRun) { Write-Host "(dry run -- nothing was actually installed or changed)" }
}

# Test seam: set DOTFILES_SETUP_PS1_SOURCE_ONLY and dot-source this file to load
# helper functions without running install, config, or Neovim sync phases.
if ($env:DOTFILES_SETUP_PS1_SOURCE_ONLY) { return }

Stop-NvimSelfLinkIfNeeded

if ($Update) {
    Invoke-SetupUpdateMode
    exit 0
}

# ---- Phase 1: dependencies ---------------------------------------------------
if (-not $SkipDeps) {
    Phase "Phase 1/6: install dependencies"
    & (Join-Path $ScriptDir 'install-deps.ps1') @depsArgs
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

Invoke-PolarisAgentPolicy

# ---- Summary -----------------------------------------------------------------
Write-Host ""
Write-Host "================================================================"
Write-Host "==  setup.ps1: done"
Write-Host "================================================================"
Write-Host ""
Write-Host "Repo:    $ScriptDir"
Write-Host "Try it:  nvim  (then <Space>fg for live grep, :wnf to save w/o format)"
Write-Host ""
Write-Host "Note:    open a NEW PowerShell window so starship + newly-installed"
Write-Host '         tools pick up PATH (or run  . $PROFILE  in this one) -- this'
Write-Host "         shell started before they were installed, so its prompt is"
Write-Host "         not themed yet."
Write-Host ""
if ($DryRun) { Write-Host "(dry run -- nothing was actually installed or changed)" }
