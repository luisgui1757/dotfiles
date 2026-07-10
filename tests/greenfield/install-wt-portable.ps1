[CmdletBinding()]
param([switch]$Launch)

$ErrorActionPreference = 'Stop'

# Windows Sandbox cannot register the MSIX package. Reuse the production
# version/hash constants and install the reviewed portable bytes transactionally.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$installDeps = Join-Path $repoRoot 'install-deps.ps1'
$oldSourceOnly = $env:INSTALL_DEPS_PS1_SOURCE_ONLY
$oldErrorActionPreference = $ErrorActionPreference
try {
    $env:INSTALL_DEPS_PS1_SOURCE_ONLY = '1'
    . $installDeps
} finally {
    $ErrorActionPreference = $oldErrorActionPreference
    if ($null -eq $oldSourceOnly) {
        Remove-Item Env:INSTALL_DEPS_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
    } else {
        $env:INSTALL_DEPS_PS1_SOURCE_ONLY = $oldSourceOnly
    }
}

if (Get-Command wt -ErrorAction SilentlyContinue) {
    Write-Host "windows-terminal: already on PATH"
    if ($Launch) { Start-Process wt }
    return
}

$localAppData = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
if ([string]::IsNullOrWhiteSpace($localAppData)) {
    throw 'LocalApplicationData known folder could not be resolved'
}
$assetVersion = $WindowsTerminalVersion.TrimStart('v')
$zipName = "Microsoft.WindowsTerminal_${assetVersion}_x64.zip"
$zipUrl = "https://github.com/microsoft/terminal/releases/download/$WindowsTerminalVersion/$zipName"
$destination = Join-Path $localAppData 'WindowsTerminalPortable'
$parent = Split-Path -Parent $destination
New-Item -ItemType Directory -Force -Path $parent | Out-Null

$temp = Join-Path ([IO.Path]::GetTempPath()) ("windows-terminal-greenfield-" + [guid]::NewGuid().ToString('N'))
$stage = Join-Path $parent (".WindowsTerminalPortable.stage." + [guid]::NewGuid().ToString('N'))
$rollback = Join-Path $parent (".WindowsTerminalPortable.rollback." + [guid]::NewGuid().ToString('N'))
$published = $false
$hadDestination = Test-Path -LiteralPath $destination
try {
    New-Item -ItemType Directory -Path $temp | Out-Null
    $zip = Join-Path $temp $zipName
    $extract = Join-Path $temp 'extract'
    Write-Host "windows-terminal: downloading pinned portable $WindowsTerminalVersion"
    Invoke-WebRequest -Uri $zipUrl -OutFile $zip -UseBasicParsing
    if (-not (Test-FileSha256 -Path $zip -Expected $WindowsTerminalX64Sha256)) {
        throw "checksum mismatch for $zipName"
    }
    Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force
    $wtExe = @(Get-ChildItem -LiteralPath $extract -Recurse -Filter 'wt.exe' -File | Select-Object -First 1)
    if ($wtExe.Count -ne 1) { throw 'verified portable archive did not contain wt.exe' }

    New-Item -ItemType Directory -Path $stage | Out-Null
    Copy-Item -Path (Join-Path $wtExe[0].DirectoryName '*') -Destination $stage -Recurse -Force
    $stagedWt = Join-Path $stage 'wt.exe'
    if (-not (Test-Path -LiteralPath $stagedWt -PathType Leaf)) {
        throw 'staged portable payload did not contain wt.exe at its root'
    }

    if ($hadDestination) { [IO.Directory]::Move($destination, $rollback) }
    try {
        [IO.Directory]::Move($stage, $destination)
        $published = $true
        $installedWt = Join-Path $destination 'wt.exe'
        if (-not (Test-Path -LiteralPath $installedWt -PathType Leaf)) {
            throw 'published portable payload failed wt.exe validation'
        }
        Add-DirectoryToUserPath -Directory $destination
    } catch {
        $cause = $_.Exception.Message
        if ($published -and (Test-Path -LiteralPath $destination)) {
            Remove-Item -LiteralPath $destination -Recurse -Force
        }
        if ($hadDestination -and (Test-Path -LiteralPath $rollback)) {
            [IO.Directory]::Move($rollback, $destination)
        }
        throw $cause
    }
    if (Test-Path -LiteralPath $rollback) {
        Remove-Item -LiteralPath $rollback -Recurse -Force
    }
    Write-Host "windows-terminal: portable build installed at $destination"
} finally {
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
}

# setup.ps1 owns the independent portable settings merge. This helper never
# copies packaged settings into the portable path.
if ($Launch) { Start-Process (Join-Path $destination 'wt.exe') }
