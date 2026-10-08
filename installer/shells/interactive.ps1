function Test-DotfilesInteractiveInvocation {
    [CmdletBinding()]
    param(
        [bool]$UserInteractive = [Environment]::UserInteractive,
        [string]$HostName = $Host.Name,
        [string[]]$CommandLineArgs = [Environment]::GetCommandLineArgs(),
        [bool]$InputRedirected = $(try { [Console]::IsInputRedirected } catch { $true }),
        [bool]$OutputRedirected = $(try { [Console]::IsOutputRedirected } catch { $true }),
        [bool]$ErrorRedirected = $(try { [Console]::IsErrorRedirected } catch { $true }),
        [AllowEmptyString()] [string]$CIValue = $env:CI
    )

    if (-not $UserInteractive -or $InputRedirected -or $OutputRedirected -or $ErrorRedirected) {
        return $false
    }
    if ($HostName -notin @('ConsoleHost', 'Visual Studio Code Host', 'Windows PowerShell ISE Host')) {
        return $false
    }
    if (-not [string]::IsNullOrWhiteSpace($CIValue) -and $CIValue -notmatch '^(?i:false|0|no)$') {
        return $false
    }

    $hasNoExit = $false
    $hasBatchSelector = $false
    foreach ($argument in @($CommandLineArgs)) {
        switch -Regex ([string]$argument) {
            '^(?i:-noninteractive|-noni)$' { return $false }
            '^(?i:-noexit|-noe)$' { $hasNoExit = $true; continue }
            '^(?i:-command|-c|-encodedcommand|-e|-ec|-file|-f|-commandwithargs)$' {
                $hasBatchSelector = $true
                continue
            }
        }
    }
    if ($HostName -eq 'ConsoleHost' -and $hasBatchSelector -and -not $hasNoExit) {
        return $false
    }
    return $true
}

# Tests can provide a deterministic invocation context before dot-sourcing the
# profile; real shells do not define this variable and always use runtime data.
$invocationContext = Get-Variable -Name DotfilesProfileInvocationContext -ValueOnly -ErrorAction SilentlyContinue
$interactive = if ($invocationContext -is [hashtable]) {
    Test-DotfilesInteractiveInvocation @invocationContext
} else {
    Test-DotfilesInteractiveInvocation
}
if (-not $interactive) { return }

