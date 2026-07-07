[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidatePattern('^[0-9a-fA-F]{40}$')]
    [string]$CommitSha,
    [string]$Remote = 'https://github.com/luisgui1757/dotfiles.git'
)

# Optional self-contained Windows Sandbox greenfield bootstrap. The default
# windows-sandbox.wsb uses a mapped local checkout so it never remote-evals a
# mutable branch. If a mapped folder is impractical, run this helper only after
# intentionally selecting a full commit SHA: it fetches that exact object, checks
# out FETCH_HEAD, verifies git rev-parse HEAD equals the requested SHA, then
# executes the checked-out local sandbox-run.ps1.
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = Join-Path $env:USERPROFILE 'dotfiles'

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    throw 'git is required for commit-pinned sandbox bootstrap; use the mapped .wsb path or install Git first'
}

if (Test-Path -LiteralPath $repo) { Remove-Item -LiteralPath $repo -Recurse -Force }
New-Item -ItemType Directory -Path $repo -Force | Out-Null

Write-Host "greenfield sandbox: fetching dotfiles commit $CommitSha..."
git -C $repo init
git -C $repo remote add origin $Remote
git -C $repo fetch --depth 1 origin $CommitSha
git -C $repo checkout --detach FETCH_HEAD

$actual = (git -C $repo rev-parse HEAD).Trim()
if ($actual -ne $CommitSha.ToLowerInvariant()) {
    throw "fetched commit mismatch: expected $CommitSha, got $actual"
}
Write-Host "greenfield sandbox: verified git rev-parse HEAD = $actual"

& (Join-Path $repo 'tests\greenfield\sandbox-run.ps1') -WorkRepo $repo -SkipCopy
