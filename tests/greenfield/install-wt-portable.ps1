[CmdletBinding()]
param([switch]$Launch)

$ErrorActionPreference = 'Stop'

# Windows Terminal ships as an MSIX (Store) package, and Windows Sandbox cannot
# register MSIX packages -- so scoop/winget/choco installs of `wt` all fail
# there. The PORTABLE build is a plain zip with no MSIX, so it runs in the
# Sandbox. This is greenfield TEST tooling for a disposable VM; the real
# install-deps.ps1 keeps using the package managers, which work on a real machine.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

if (Get-Command wt -ErrorAction SilentlyContinue) {
    Write-Host "windows-terminal: already on PATH"
    if ($Launch) { Start-Process wt }
    return
}

Write-Host "windows-terminal: installing the portable build (no MSIX, Sandbox-friendly)"
$headers = @{ 'User-Agent' = 'dotfiles-greenfield' }
$release = Invoke-RestMethod -Uri 'https://api.github.com/repos/microsoft/terminal/releases/latest' -Headers $headers -UseBasicParsing
$asset = $release.assets | Where-Object { $_.name -match '_x64\.zip$' } | Select-Object -First 1
if (-not $asset) {
    throw "no x64 portable zip in the latest Windows Terminal release"
}

$zip = Join-Path $env:TEMP 'windows-terminal-portable.zip'
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zip -UseBasicParsing

$dest = Join-Path $env:LOCALAPPDATA 'WindowsTerminalPortable'
if (Test-Path -LiteralPath $dest) {
    Remove-Item -LiteralPath $dest -Recurse -Force
}
Expand-Archive -LiteralPath $zip -DestinationPath $dest -Force

$wtExe = Get-ChildItem -LiteralPath $dest -Recurse -Filter 'wt.exe' | Select-Object -First 1
if (-not $wtExe) {
    throw "wt.exe not found after extracting the portable zip"
}
$wtDir = Split-Path -Parent $wtExe.FullName

# Put wt on PATH for this process and for new shells in the sandbox.
if (($env:PATH -split ';') -notcontains $wtDir) {
    $env:PATH = "$wtDir;$env:PATH"
}
$userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
if (-not $userPath) { $userPath = '' }
if (($userPath -split ';') -notcontains $wtDir) {
    [Environment]::SetEnvironmentVariable('PATH', ("$wtDir;$userPath").TrimEnd(';'), 'User')
}

Write-Host "windows-terminal: portable build installed at $wtDir"
if ($Launch) { Start-Process $wtExe.FullName }
