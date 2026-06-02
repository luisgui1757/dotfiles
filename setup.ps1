# setup.ps1 -- one-shot end-to-end install for Windows.
#
# Local usage (from a checked-out copy):
#   .\setup.ps1                  interactive: Y/n per dep, then symlink + sync
#   .\setup.ps1 -All             non-interactive: install everything missing
#   .\setup.ps1 -DryRun          preview every step
#   .\setup.ps1 -SkipDeps        already have nvim/starship; just bootstrap+sync
#   .\setup.ps1 -SkipBootstrap   already symlinked; just sync plugins+LSP
#   .\setup.ps1 -SkipNvim        skip nvim plugin + Mason sync
#   .\setup.ps1 -MergeWindowsTerminal     also merge the WT rose-pine fragment
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
    [switch]$SkipNvim,
    [switch]$MergeWindowsTerminal,
    [switch]$BestEffort
)

$ErrorActionPreference = 'Stop'

$RepoUrl     = 'https://github.com/luisgui1757/dotfiles.git'
$DefaultDest = Join-Path $env:USERPROFILE 'dotfiles'

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
if (-not $ScriptDir -or -not (Test-Path (Join-Path $ScriptDir 'bootstrap.ps1'))) {
    $dest = if ($env:DOTFILES_DEST) { $env:DOTFILES_DEST } else { $DefaultDest }
    # DryRun honor: announce what we would clone and exit BEFORE any git op.
    if ($DryRun) {
        Write-Host "setup.ps1 (remote, dry-run): would clone $RepoUrl -> $dest"
        Write-Host "                             then re-invoke .\setup.ps1 from there."
        Write-Host "(dry run -- no clone, no install, no writes performed)"
        exit 0
    }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Error "setup.ps1: git is the only prerequisite for remote bootstrap, and it is required to clone the repo. Install git first: winget install Git.Git"
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

# ---- Self-link guard ---------------------------------------------------------
$nvimTarget = Join-Path $env:LOCALAPPDATA 'nvim'
if ((Resolve-Path $ScriptDir).Path -eq (Resolve-Path $nvimTarget -ErrorAction SilentlyContinue).Path) {
    Write-Error @"
setup.ps1: the repo lives at $ScriptDir, which is the same path that
bootstrap.ps1 would symlink to itself. Move the repo elsewhere first
(e.g. %USERPROFILE%\dotfiles) and re-run setup.ps1.
"@
    exit 1
}

# ---- Forward flags to sub-scripts --------------------------------------------
# Hashtable splatting (not array) so switches bind by NAME. Array splatting
# passes elements as positional args, which switch parameters cannot accept;
# that path produced a positional-parameter error the moment Phase 1
# invoked install-deps.ps1 with -All in CI.
$depsArgs = @{}
if ($All)    { $depsArgs['All']    = $true }
if ($DryRun) { $depsArgs['DryRun'] = $true }

$bootstrapArgs = @{}
if ($DryRun)               { $bootstrapArgs['DryRun']               = $true }
if ($MergeWindowsTerminal) { $bootstrapArgs['MergeWindowsTerminal'] = $true }

function Phase {
    param([string]$title)
    Write-Host ""
    Write-Host "================================================================"
    Write-Host "==  $title"
    Write-Host "================================================================"
}

# ---- Phase 1: dependencies ---------------------------------------------------
if (-not $SkipDeps) {
    Phase "Phase 1/4: install dependencies"
    & (Join-Path $ScriptDir 'install-deps.ps1') @depsArgs
} else {
    Write-Host ""
    Write-Host "skipped: Phase 1 (deps) via -SkipDeps"
}

# Phase 1 may install nvim/tools via scoop/winget/choco into locations not yet on
# this process PATH (a child installer cannot mutate our PATH, and persistent PATH
# edits only reach NEW shells). Re-derive PATH so Phase 3-4 can find nvim -- the
# Windows analog of refresh_runtime_path in setup.sh.
function Update-RuntimePath {
    $parts = @()
    $scoopRoot = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
    $parts += (Join-Path $scoopRoot 'shims')
    foreach ($scope in 'Machine', 'User') {
        $p = [Environment]::GetEnvironmentVariable('PATH', $scope)
        if ($p) { $parts += ($p -split ';') }
    }
    $parts += ($env:PATH -split ';')
    $seen = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
    $env:PATH = ($parts | Where-Object { $_ -and $seen.Add($_) }) -join ';'
}
Update-RuntimePath

# ---- Phase 2: symlink configs ------------------------------------------------
if (-not $SkipBootstrap) {
    Phase "Phase 2/4: symlink configs into place"
    $global:LASTEXITCODE = 0   # reset so a stale code from Phase 1 can't false-trip
    & (Join-Path $ScriptDir 'bootstrap.ps1') @bootstrapArgs
    if ($LASTEXITCODE -ne 0) {
        # bootstrap already printed the actionable fix (Dev Mode / elevation).
        # Stop here rather than running nvim sync against un-symlinked configs.
        Write-Host ""
        Write-Host "setup.ps1: stopping -- Phase 2 (bootstrap) failed; see the fix above." -ForegroundColor Red
        exit $LASTEXITCODE
    }
} else {
    Write-Host ""
    Write-Host "skipped: Phase 2 (bootstrap) via -SkipBootstrap"
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
        Write-Host "             .\setup.ps1 -SkipDeps -SkipBootstrap"
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
