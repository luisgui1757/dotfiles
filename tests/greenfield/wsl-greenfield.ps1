[CmdletBinding()]
param(
    [string]$DistroName = '',
    [string]$RootfsTar = '',
    [string]$InstallRoot = '',
    [switch]$Keep
)

$ErrorActionPreference = 'Stop'

if (-not $DistroName) {
    $DistroName = "dotfiles-greenfield-{0}" -f (Get-Date -Format 'yyyyMMdd-HHmmss')
}
if (-not $InstallRoot) {
    $InstallRoot = Join-Path $env:TEMP $DistroName
}

function Invoke-WslChecked {
    param([string[]]$Arguments)
    $global:LASTEXITCODE = 0
    & wsl.exe @Arguments
    $rc = $LASTEXITCODE
    if ($rc -ne 0) {
        throw "wsl.exe $($Arguments -join ' ') exited $rc"
    }
}

function Get-WslDistroNames {
    $raw = & wsl.exe --list --quiet 2>$null
    if ($LASTEXITCODE -ne 0) {
        return @()
    }
    return @($raw | ForEach-Object { ($_ -replace "`0", '').Trim() } | Where-Object { $_ })
}

function Test-WslDistroExists {
    param([string]$Name)
    return [bool]((Get-WslDistroNames) | Where-Object { $_ -eq $Name })
}

function Remove-WslDistroSafe {
    param([string]$Name)
    if (-not (Test-WslDistroExists -Name $Name)) {
        return
    }
    if ($Name -notlike 'dotfiles-greenfield-*') {
        throw "refusing to unregister non-greenfield distro name: $Name"
    }
    Write-Host "wsl-greenfield: unregistering existing $Name"
    & wsl.exe --unregister $Name | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "wsl --unregister $Name failed"
    }
}

function New-GreenfieldDistro {
    if ($RootfsTar) {
        $rootfs = (Resolve-Path -LiteralPath $RootfsTar -ErrorAction Stop).Path
        New-Item -ItemType Directory -Path $InstallRoot -Force | Out-Null
        Invoke-WslChecked -Arguments @('--import', $DistroName, $InstallRoot, $rootfs, '--version', '2')
        return
    }

    New-Item -ItemType Directory -Path $InstallRoot -Force | Out-Null
    $global:LASTEXITCODE = 0
    & wsl.exe --install -d Ubuntu-24.04 --name $DistroName --no-launch
    $rc = $LASTEXITCODE
    if ($rc -ne 0) {
        throw "wsl --install -d Ubuntu-24.04 --name failed. Pass -RootfsTar with an Ubuntu 24.04 rootfs tarball and rerun."
    }
}

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    throw "wsl.exe is not on PATH"
}

$scriptDir = Split-Path -Parent $PSCommandPath
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $scriptDir '..\..') -ErrorAction Stop).Path

try {
    Remove-WslDistroSafe -Name $DistroName
    if (Test-Path -LiteralPath $InstallRoot) {
        Remove-Item -LiteralPath $InstallRoot -Recurse -Force
    }

    Write-Host "wsl-greenfield: creating $DistroName"
    New-GreenfieldDistro

    $rootPrep = @'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends bash sudo ca-certificates git rsync findutils coreutils procps passwd curl tar gzip unzip xz-utils
if ! id -u dotfiles >/dev/null 2>&1; then
    useradd -m -s /bin/bash dotfiles
fi
printf '%s\n' 'dotfiles ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dotfiles
chmod 0440 /etc/sudoers.d/dotfiles
'@
    Invoke-WslChecked -Arguments @('-d', $DistroName, '-u', 'root', '--', 'bash', '-lc', $rootPrep)

    $global:LASTEXITCODE = 0
    $wslRepo = & wsl.exe -d $DistroName -u root -- wslpath -a $repoRoot
    if ($LASTEXITCODE -ne 0 -or -not $wslRepo) {
        throw "wslpath could not convert repo path: $repoRoot"
    }
    $wslRepo = @($wslRepo)[0]

    $copyRepo = @'
set -euo pipefail
src="$1"
dest=/home/dotfiles/dotfiles
rm -rf "$dest"
mkdir -p "$dest"
rsync -a --delete --exclude .git "$src"/ "$dest"/
chown -R dotfiles:dotfiles "$dest"
'@
    Invoke-WslChecked -Arguments @('-d', $DistroName, '-u', 'root', '--', 'bash', '-lc', $copyRepo, 'bash', $wslRepo)

    $runSetup = @'
set -euo pipefail
cd "$HOME/dotfiles"
for d in /usr/local/bin "$HOME/.local/bin"; do
    case ":$PATH:" in
        *":$d:"*) ;;
        *) [ -d "$d" ] && PATH="$d:$PATH" ;;
    esac
done
export PATH
setup_log="$HOME/setup-sh.log"
set +e
./setup.sh --all 2>&1 | tee "$setup_log"
rc=${PIPESTATUS[0]}
set -e
if [ "$rc" -ne 0 ]; then
    echo "FAIL: setup.sh exited $rc" >&2
    exit "$rc"
fi
if grep -Fq "skipped: Phase 3-5" "$setup_log"; then
    echo "FAIL: setup.sh skipped Phase 3-5" >&2
    exit 1
fi
if grep -Eq "^[[:space:]]*FAIL:" "$setup_log"; then
    echo "FAIL: setup.sh emitted a FAIL marker" >&2
    exit 1
fi
grep -F "Phase 3/6" "$setup_log" >/dev/null
grep -F "Phase 4/6" "$setup_log" >/dev/null
grep -F "Phase 5/6" "$setup_log" >/dev/null
grep -F "Phase 6/6" "$setup_log" >/dev/null
tests/greenfield/validate.sh
'@
    Invoke-WslChecked -Arguments @('-d', $DistroName, '-u', 'dotfiles', '--', 'bash', '-lc', $runSetup)

    Write-Host "PASS: WSL greenfield setup and validation completed for $DistroName"
} finally {
    if ($Keep) {
        Write-Host "wsl-greenfield: keeping $DistroName for debugging"
        Write-Host "wsl-greenfield: unregister later with: wsl --unregister $DistroName"
    } else {
        # Cleanup MUST keep the same `dotfiles-greenfield-*` guard as
        # Remove-WslDistroSafe. Without it, an abort earlier in the run (e.g. the
        # user passed -DistroName pointing at a REAL distro) would reach this
        # finally and unregister that real distro -- irreversible data loss.
        if (Test-WslDistroExists -Name $DistroName) {
            if ($DistroName -like 'dotfiles-greenfield-*') {
                Write-Host "wsl-greenfield: unregistering $DistroName"
                & wsl.exe --unregister $DistroName | Out-Host
            } else {
                Write-Host "wsl-greenfield: NOT unregistering non-greenfield distro $DistroName (safety guard)"
            }
        }
        if (Test-Path -LiteralPath $InstallRoot) {
            Remove-Item -LiteralPath $InstallRoot -Recurse -Force
        }
    }
}
