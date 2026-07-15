# Canonical Windows Terminal settings identities shared by setup, migration,
# recovery, and uninstall. Keep this file data-only: callers decide whether an
# absent target should be published, retained, or ignored.

function Get-DotfilesWindowsTerminalTargetDefinitions {
    param([Parameter(Mandatory)] [string]$LocalApplicationData)

    $isWindowsAbsolute = $LocalApplicationData -match '^(?:[A-Za-z]:[\\/]|\\\\)'
    if ([string]::IsNullOrWhiteSpace($LocalApplicationData) -or
        (-not [IO.Path]::IsPathRooted($LocalApplicationData) -and -not $isWindowsAbsolute) -or
        $LocalApplicationData -match '^[A-Za-z]:[^\\/]') {
        throw "Windows LocalApplicationData is missing or not absolute: $LocalApplicationData"
    }

    $definitions = @(
        [pscustomobject]@{
            Kind = 'Packaged'
            Path = Join-Path $LocalApplicationData 'Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json'
        },
        [pscustomobject]@{
            Kind = 'Preview'
            Path = Join-Path $LocalApplicationData 'Packages\Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe\LocalState\settings.json'
        },
        [pscustomobject]@{
            Kind = 'Canary'
            Path = Join-Path $LocalApplicationData 'Packages\Microsoft.WindowsTerminalCanary_8wekyb3d8bbwe\LocalState\settings.json'
        },
        [pscustomobject]@{
            Kind = 'Portable'
            Path = Join-Path $LocalApplicationData 'Microsoft\Windows Terminal\settings.json'
        }
    )

    $kinds = @($definitions.Kind | Sort-Object -Unique)
    $paths = @($definitions.Path | ForEach-Object {
            if ($isWindowsAbsolute -and $env:OS -ne 'Windows_NT') {
                $_.Replace('/', '\').TrimEnd('\')
            } else {
                [IO.Path]::GetFullPath($_).TrimEnd('\', '/')
            }
        } | Sort-Object -Unique)
    if ($definitions.Count -ne 4 -or $kinds.Count -ne 4 -or $paths.Count -ne 4) {
        throw 'Windows Terminal target definitions are incomplete or ambiguous'
    }
    return $definitions
}

function Get-DotfilesWindowsTerminalTargets {
    param(
        [Parameter(Mandatory)] [string]$LocalApplicationData,
        [switch]$IncludeAbsent,
        [bool]$PortablePresent = $false
    )

    foreach ($definition in @(Get-DotfilesWindowsTerminalTargetDefinitions -LocalApplicationData $LocalApplicationData)) {
        $existed = Test-Path -LiteralPath $definition.Path -PathType Leaf
        if ($IncludeAbsent -or $existed -or ($definition.Kind -eq 'Portable' -and $PortablePresent)) {
            [pscustomobject]@{
                Kind = $definition.Kind
                Path = $definition.Path
                Existed = [bool]$existed
            }
        }
    }
}

function Get-DotfilesWindowsTerminalTargetDefinition {
    param(
        [Parameter(Mandatory)] [string]$LocalApplicationData,
        [Parameter(Mandatory)]
        [ValidateSet('Packaged', 'Preview', 'Canary', 'Portable')]
        [string]$Kind
    )

    $matches = @(Get-DotfilesWindowsTerminalTargetDefinitions -LocalApplicationData $LocalApplicationData |
        Where-Object { $_.Kind -eq $Kind })
    if ($matches.Count -ne 1) {
        throw "Windows Terminal target identity is missing or ambiguous: $Kind"
    }
    return $matches[0]
}
