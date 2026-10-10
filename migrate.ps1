# Preserve and detach released shell profiles before ordinary setup adoption.
$ErrorActionPreference = 'Stop'
$PreviousEntrypoint = $env:DOTFILES_ENTRYPOINT
try {
    $env:DOTFILES_ENTRYPOINT = 'migrate'
    & (Join-Path $PSScriptRoot 'scripts/installer-bootstrap.ps1') @args
    $Result = $LASTEXITCODE
} finally {
    $env:DOTFILES_ENTRYPOINT = $PreviousEntrypoint
}
exit $Result
