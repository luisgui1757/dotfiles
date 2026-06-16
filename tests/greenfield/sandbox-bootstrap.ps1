[CmdletBinding()]
param([string]$Ref = 'chezmoi-pilot')

# Self-contained Windows Sandbox greenfield bootstrap. windows-sandbox.wsb fetches
# and runs THIS via its LogonCommand, so the sandbox does NOT depend on a mapped
# folder. The mapped-folder approach is fragile: a relative HostFolder needs
# Windows 11 22H2+, and the repo has to be cloned and the .wsb launched from
# inside it -- when that did not hold, the sandbox opened to an empty PowerShell.
# This downloads the repo itself and hands off to sandbox-run.ps1 -SkipCopy.
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = Join-Path $env:USERPROFILE 'dotfiles'
$zip = Join-Path $env:TEMP 'dotfiles-greenfield.zip'
$extract = Join-Path $env:TEMP 'dotfiles-greenfield-extract'

Write-Host "greenfield sandbox: downloading the repo ($Ref)..."
Invoke-WebRequest -Uri "https://github.com/luisgui1757/dotfiles/archive/refs/heads/$Ref.zip" -OutFile $zip -UseBasicParsing

if (Test-Path -LiteralPath $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force
$inner = Get-ChildItem -LiteralPath $extract -Directory | Select-Object -First 1
if (-not $inner) { throw "downloaded repo archive was empty" }

if (Test-Path -LiteralPath $repo) { Remove-Item -LiteralPath $repo -Recurse -Force }
Move-Item -LiteralPath $inner.FullName -Destination $repo -Force

& (Join-Path $repo 'tests\greenfield\sandbox-run.ps1') -WorkRepo $repo -SkipCopy
