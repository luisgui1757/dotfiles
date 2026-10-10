# Human menu by default; automation supplies the explicit machine request.
$ErrorActionPreference = 'Stop'
$PreviousEntrypoint = $env:DOTFILES_ENTRYPOINT
try {
    $env:DOTFILES_ENTRYPOINT = 'setup'
    & (Join-Path $PSScriptRoot 'scripts/installer-bootstrap.ps1') @args
    $Result = $LASTEXITCODE
} finally {
    $env:DOTFILES_ENTRYPOINT = $PreviousEntrypoint
}
exit $Result
