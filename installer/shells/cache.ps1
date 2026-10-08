# Generate local initialization from the selected, pinned executable. Keep one
# cache per recipe; publish complete files so concurrent shells cannot read half.
function Get-DotfilesInitScript {
    param([string]$Command, [string[]]$Arguments, [string]$CachePath)
    function Test-InitContent([string]$Content) {
        if ([string]::IsNullOrWhiteSpace($Content)) { return $false }
        $tokens = $null
        $parseErrors = $null
        [void][Management.Automation.Language.Parser]::ParseInput($Content, [ref]$tokens, [ref]$parseErrors)
        return @($parseErrors).Count -eq 0
    }
    if (Test-Path -LiteralPath $CachePath -PathType Leaf) {
        if (Test-InitContent ([IO.File]::ReadAllText($CachePath))) { return $CachePath }
        throw "Invalid initialization cache at $CachePath; preserve or remove it before retrying."
    }
    $content = @(& $Command @Arguments) -join [Environment]::NewLine
    if ($LASTEXITCODE -ne 0 -or -not (Test-InitContent $content)) {
        throw "Initialization generation failed for $Command; nothing was published."
    }
    $directory = Split-Path -Parent $CachePath
    [void][IO.Directory]::CreateDirectory($directory)
    $temporary = Join-Path $directory ([guid]::NewGuid().ToString('N') + '.tmp')
    try {
        [IO.File]::WriteAllText($temporary, $content, [Text.UTF8Encoding]::new($false))
        try { [IO.File]::Move($temporary, $CachePath) } catch {
            if (-not (Test-Path -LiteralPath $CachePath -PathType Leaf) -or
                -not (Test-InitContent ([IO.File]::ReadAllText($CachePath)))) { throw }
        }
    } finally {
        if (Test-Path -LiteralPath $temporary) { [IO.File]::Delete($temporary) }
    }
    return $CachePath
}
