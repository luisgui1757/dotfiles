$ErrorActionPreference = 'Stop'
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$script:RepoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$script:SourceDir = Join-Path $script:RepoRoot 'home'
$script:Chezmoi = $null

$script:UserProfileGuid = '{11111111-1111-1111-1111-111111111111}'
$script:ManagedPwshProfileGuid = '{8a0e8c9b-2b4c-5842-ac1b-29cd17efc89b}'
$script:LegacyWindowsPowerShellProfileGuid = '{61c54bbd-c2c6-5271-96e7-009a87ff44bf}'
$script:UserSchemeName = 'UserSeedScheme'
$script:UserActionKeys = 'alt+f4'
$script:ManagedGlobals = @(
    'copyFormatting',
    'copyOnSelect',
    'firstWindowPreference',
    'initialRows',
    'launchMode',
    'theme',
    'useAcrylicInTabRow',
    'windowingBehavior'
)

function Pass {
    param([Parameter(Mandatory)] [string]$Message)
    Write-Host "PASS: $Message"
}

function Assert-Condition {
    param(
        [Parameter(Mandatory)] [bool]$Condition,
        [Parameter(Mandatory)] [string]$Message
    )
    if (-not $Condition) {
        throw $Message
    }
}

function New-TestSandbox {
    param([Parameter(Mandatory)] [string]$Name)
    $sandbox = Join-Path ([IO.Path]::GetTempPath()) ("dotfiles-chezmoi-{0}-{1}" -f $Name, [guid]::NewGuid())
    New-Item -ItemType Directory -Force -Path $sandbox | Out-Null
    return $sandbox
}

function Remove-TestSandbox {
    param([Parameter(Mandatory)] [string]$Sandbox)
    if (Test-Path -LiteralPath $Sandbox) {
        Remove-Item -LiteralPath $Sandbox -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Get-WtSettingsPath {
    param([Parameter(Mandatory)] [string]$Sandbox)
    return (Join-Path $Sandbox 'AppData\Local\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json')
}

function Invoke-WithSandboxEnv {
    param(
        [Parameter(Mandatory)] [string]$Sandbox,
        [Parameter(Mandatory)] [scriptblock]$Script
    )

    $localAppData = Join-Path $Sandbox 'AppData\Local'
    $appData = Join-Path $Sandbox 'AppData\Roaming'
    $tempDir = Join-Path $Sandbox 'Temp'
    $profilePath = Join-Path $Sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
    foreach ($dir in @($localAppData, $appData, $tempDir, (Split-Path -Parent $profilePath))) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }

    $envNames = @('USERPROFILE', 'HOME', 'LOCALAPPDATA', 'APPDATA', 'TEMP', 'TMP')
    $oldEnv = @{}
    foreach ($name in $envNames) {
        $oldEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    }
    $oldProfile = (Get-Variable -Name PROFILE -Scope Global -ErrorAction SilentlyContinue).Value

    try {
        $env:USERPROFILE = $Sandbox
        $env:HOME = $Sandbox
        $env:LOCALAPPDATA = $localAppData
        $env:APPDATA = $appData
        $env:TEMP = $tempDir
        $env:TMP = $tempDir
        Set-Variable -Name PROFILE -Scope Global -Value $profilePath -Force
        & $Script
    } finally {
        foreach ($name in $envNames) {
            if ($null -eq $oldEnv[$name]) {
                [Environment]::SetEnvironmentVariable($name, $null, 'Process')
            } else {
                [Environment]::SetEnvironmentVariable($name, $oldEnv[$name], 'Process')
            }
        }
        if ($null -ne $oldProfile) {
            Set-Variable -Name PROFILE -Scope Global -Value $oldProfile -Force
        }
    }
}

function Invoke-CheckedNative {
    param(
        [Parameter(Mandatory)] [string]$FilePath,
        [Parameter(Mandatory)] [string[]]$Arguments
    )
    & $FilePath @Arguments
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        throw "$FilePath $($Arguments -join ' ') exited $exitCode"
    }
}

function Invoke-Chezmoi {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    # --no-tty + --force: CI has no interactive tty, so any chezmoi prompt
    # ("X has changed since chezmoi last wrote it? diff/overwrite/skip/quit")
    # would block forever (observed: a 40-minute hang on the nvim dir-symlink).
    # --force makes every change without prompting; --no-tty refuses to grab a
    # TTY. verify ignores both (it makes no changes), so it stays a strict
    # oracle: if the nvim symlink ever fails to round-trip, verify still fails.
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@('--source', $script:SourceDir, '--no-tty', '--force') + $Arguments)
}

function Invoke-ChezmoiReapply {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    # Idempotency / second-apply check: --no-tty but deliberately NO --force. The
    # first apply uses --force (a pre-existing seeded target may legitimately need
    # an overwrite); the re-apply must be a clean no-op. Without --force, an
    # unexpected prompt aborts non-interactively (nonzero exit) instead of being
    # silently overwritten -> a prompt-on-reapply regression fails the test fast
    # rather than being masked. This is the strict idempotency oracle the Unix
    # parity gate already has (it captures second-apply output and fails on
    # non-empty); this keeps the Windows arm honest too.
    Invoke-CheckedNative -FilePath $script:Chezmoi -Arguments (@('--source', $script:SourceDir, '--no-tty') + $Arguments)
}

function Get-ArrayValue {
    param($Value)
    if ($null -eq $Value) {
        return @()
    }
    return @($Value)
}

function Get-SingleItemTarget {
    param([Parameter(Mandatory)] $Item)
    $target = $Item.Target
    if ($target -is [array]) {
        return $target[0]
    }
    return $target
}

function Get-CanonicalPath {
    param([Parameter(Mandatory)] [string]$Path)
    return (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
}

function Assert-FileContentMatches {
    param(
        [Parameter(Mandatory)] [string]$ActualPath,
        [Parameter(Mandatory)] [string]$ExpectedPath,
        [Parameter(Mandatory)] [string]$Label
    )

    Assert-Condition (Test-Path -LiteralPath $ActualPath -PathType Leaf) "$Label was not created"
    Assert-Condition (Test-Path -LiteralPath $ExpectedPath -PathType Leaf) "$Label expected source is missing: $ExpectedPath"
    $actualHash = (Get-FileHash -LiteralPath $ActualPath -Algorithm SHA256).Hash
    $expectedHash = (Get-FileHash -LiteralPath $ExpectedPath -Algorithm SHA256).Hash
    Assert-Condition ($actualHash -eq $expectedHash) "$Label content mismatch actual=$actualHash expected=$expectedHash"
}

function Assert-CopyModeFileMatches {
    param(
        [Parameter(Mandatory)] [string]$ActualPath,
        [Parameter(Mandatory)] [string]$ExpectedPath,
        [Parameter(Mandatory)] [string]$Label
    )

    Assert-FileContentMatches -ActualPath $ActualPath -ExpectedPath $ExpectedPath -Label $Label
    $item = Get-Item -LiteralPath $ActualPath -Force
    $linkType = if ($item.PSObject.Properties.Name -contains 'LinkType') { $item.LinkType } else { $null }
    Assert-Condition ($linkType -ne 'SymbolicLink') "$Label is a symlink; expected Windows copy mode"
}

function Assert-NvimSymlinkMatchesRepo {
    param([Parameter(Mandatory)] [string]$Sandbox)

    $nvimPath = Join-Path $Sandbox 'AppData\Local\nvim'
    Assert-Condition (Test-Path -LiteralPath $nvimPath -PathType Container) 'nvim directory was not created under AppData\Local'
    $nvimItem = Get-Item -LiteralPath $nvimPath -Force
    Assert-Condition ($nvimItem.LinkType -eq 'SymbolicLink') 'nvim is not a symlink; expected Windows dir-symlink mode'

    $target = Get-SingleItemTarget -Item $nvimItem
    Assert-Condition ([string]::IsNullOrWhiteSpace($target) -eq $false) 'nvim symlink has no target'
    $resolvedTarget = Get-CanonicalPath -Path $target
    $repoNvim = Get-CanonicalPath -Path (Join-Path $script:RepoRoot 'nvim')
    Assert-Condition ($resolvedTarget -eq $repoNvim) "nvim symlink target mismatch actual=$resolvedTarget expected=$repoNvim"

    Assert-FileContentMatches `
        -ActualPath (Join-Path $nvimPath 'init.lua') `
        -ExpectedPath (Join-Path $script:RepoRoot 'nvim\init.lua') `
        -Label 'nvim init.lua'
}

function New-BaselineWtSettings {
    return [ordered]@{
        defaultProfile = $script:UserProfileGuid
        theme = 'legacyLight'
        profiles = [ordered]@{
            defaults = [ordered]@{
                colorScheme = $script:UserSchemeName
                font = [ordered]@{
                    face = 'Consolas'
                }
            }
            list = @(
                [ordered]@{
                    guid = $script:UserProfileGuid
                    name = 'Seeded User Profile'
                    commandline = 'powershell.exe'
                    colorScheme = $script:UserSchemeName
                }
            )
        }
        schemes = @(
            [ordered]@{
                name = $script:UserSchemeName
                foreground = '#ffffff'
                background = '#000000'
            }
        )
        actions = @(
            [ordered]@{
                command = 'closeWindow'
                keys = $script:UserActionKeys
            }
        )
    }
}

function Write-BaselineWtSettings {
    param([Parameter(Mandatory)] [string]$Sandbox)
    $settingsPath = Get-WtSettingsPath -Sandbox $Sandbox
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $settingsPath) | Out-Null
    New-BaselineWtSettings |
        ConvertTo-Json -Depth 100 |
        Set-Content -LiteralPath $settingsPath -Encoding utf8
    return $settingsPath
}

function Read-JsonFile {
    param([Parameter(Mandatory)] [string]$Path)
    return (Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json)
}

function Get-NamedItem {
    param(
        $Items,
        [Parameter(Mandatory)] [string]$Name,
        [Parameter(Mandatory)] [string]$Label
    )
    $matches = @(Get-ArrayValue $Items | Where-Object { $_.name -eq $Name })
    Assert-Condition ($matches.Count -gt 0) "$Label missing: $Name"
    return $matches[0]
}

function Assert-WtUserSeedSurvived {
    param(
        [Parameter(Mandatory)] $Settings,
        [Parameter(Mandatory)] [string]$Label
    )

    Assert-Condition ($Settings.defaultProfile -eq $script:UserProfileGuid) "$Label dropped the seeded defaultProfile"
    $profiles = @(Get-ArrayValue $Settings.profiles.list | Where-Object { $_.guid -eq $script:UserProfileGuid })
    Assert-Condition ($profiles.Count -gt 0) "$Label dropped the seeded user profile"
    $schemes = @(Get-ArrayValue $Settings.schemes | Where-Object { $_.name -eq $script:UserSchemeName })
    Assert-Condition ($schemes.Count -gt 0) "$Label dropped the seeded user scheme"
    $actions = @(Get-ArrayValue $Settings.actions | Where-Object { $_.keys -eq $script:UserActionKeys })
    Assert-Condition ($actions.Count -gt 0) "$Label dropped the seeded user action"
}

function Assert-WtManagedPwshProfilePresent {
    param(
        [Parameter(Mandatory)] $Settings,
        [Parameter(Mandatory)] [string]$Label
    )

    $profiles = @(Get-ArrayValue $Settings.profiles.list | Where-Object { $_.guid -eq $script:ManagedPwshProfileGuid })
    Assert-Condition ($profiles.Count -eq 1) "$Label missing managed PowerShell 7 profile"
    Assert-Condition ($profiles[0].name -eq 'PowerShell 7') "$Label managed PowerShell 7 profile name mismatch"
    Assert-Condition ($profiles[0].commandline -eq 'pwsh.exe') "$Label managed PowerShell 7 commandline mismatch"
}

function Read-WtFragment {
    $fragmentPath = Join-Path $script:RepoRoot 'windows-terminal\settings.fragment.jsonc'
    return ((Strip-Jsonc (Get-Content -Raw -LiteralPath $fragmentPath)) | ConvertFrom-Json)
}

function Get-WTActionKeysFromItems {
    param($Items)

    $keys = @()
    foreach ($item in (Get-ArrayValue $Items)) {
        $keys += @(Get-WTActionKeySet $item)
    }
    return @($keys | Sort-Object -Unique)
}

function Assert-StringSetEqual {
    param(
        [Parameter(Mandatory)] [string[]]$Actual,
        [Parameter(Mandatory)] [string[]]$Expected,
        [Parameter(Mandatory)] [string]$Label
    )

    $actualText = (@($Actual | Sort-Object -Unique) -join ',')
    $expectedText = (@($Expected | Sort-Object -Unique) -join ',')
    Assert-Condition ($actualText -eq $expectedText) "$Label mismatch actual=[$actualText] expected=[$expectedText]"
}

function Assert-WtManagedActionKeySet {
    param(
        [Parameter(Mandatory)] $Settings,
        [Parameter(Mandatory)] [string]$Label
    )

    $fragment = Read-WtFragment
    $expectedKeys = @(Get-WTActionKeysFromItems $fragment.actions)
    $actualKeys = @(Get-WTActionKeysFromItems $Settings.actions)
    $actualManagedKeys = @($actualKeys | Where-Object { $expectedKeys -contains $_ })
    Assert-StringSetEqual -Actual $actualManagedKeys -Expected $expectedKeys -Label "$Label managed WT action key set"
}

function Assert-Part1WtMerge {
    param([Parameter(Mandatory)] [string]$SettingsPath)

    $settings = Read-JsonFile -Path $SettingsPath
    Assert-Condition ($settings.theme -eq 'rose-pine') 'WT theme was not set to rose-pine'
    Assert-Condition ($settings.profiles.defaults.colorScheme -eq 'rose-pine') 'WT profiles.defaults.colorScheme was not set to rose-pine'
    Assert-WtManagedPwshProfilePresent -Settings $settings -Label 'WT merge'
    Get-NamedItem -Items $settings.schemes -Name 'rose-pine' -Label 'WT rose-pine scheme' | Out-Null
    Get-NamedItem -Items $settings.themes -Name 'rose-pine' -Label 'WT rose-pine theme' | Out-Null
    Assert-Condition (@(Get-ArrayValue $settings.actions).Count -ge 15) 'WT managed actions count is below 15'
    Assert-WtManagedActionKeySet -Settings $settings -Label 'chezmoi WT merge'
    Assert-WtUserSeedSurvived -Settings $settings -Label 'chezmoi WT merge'
    Assert-Condition (-not ($settings.PSObject.Properties.Name -contains '$schema')) 'WT merge fabricated a top-level $schema'
}

function Assert-Part1Files {
    param([Parameter(Mandatory)] [string]$Sandbox)

    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\tmux.conf') `
        -Label '~/.tmux.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.windows.conf') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\tmux.windows.conf') `
        -Label '~/.tmux.windows.conf'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.tmux.rose-pine.ps1') `
        -ExpectedPath (Join-Path $script:RepoRoot 'tmux\psmux-rose-pine.ps1') `
        -Label '~/.tmux.rose-pine.ps1'
    # psmux freeze boundary: the POSIX-only clipboard overlay carries the
    # `if-shell` probes that hang psmux at config-load time. It MUST NOT be
    # deployed on Windows (home/.chezmoiignore ignores it). Assert its absence so
    # a regression in the ignore rule can never silently reintroduce the freeze.
    Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $Sandbox '.tmux.posix.conf'))) `
        '~/.tmux.posix.conf must NOT be deployed on Windows (psmux config-load freeze boundary)'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox 'AppData\Local\lazygit\config.yml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'lazygit\config.windows.yml') `
        -Label 'lazygit config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\starship.toml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'starship\starship.toml') `
        -Label 'starship config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\lsd\config.yaml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'lsd\config.yaml') `
        -Label 'lsd config'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox '.config\lsd\colors.yaml') `
        -ExpectedPath (Join-Path $script:RepoRoot 'lsd\colors.yaml') `
        -Label 'lsd colors'
    Assert-CopyModeFileMatches `
        -ActualPath (Join-Path $Sandbox 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1') `
        -ExpectedPath (Join-Path $script:RepoRoot 'shells\powershell_profile.ps1') `
        -Label 'PowerShell profile'
    Assert-NvimSymlinkMatchesRepo -Sandbox $Sandbox
}

function Set-OrAdd-Property {
    param($Obj, [string]$Name, $Value)
    if ($null -eq $Obj.$Name) {
        $Obj | Add-Member -NotePropertyName $Name -NotePropertyValue $Value -Force
    } else {
        $Obj.$Name = $Value
    }
}

function Merge-ObjectArrayByProperty {
    param($CurrentItems, $FragmentItems, [string]$PropertyName)
    $result = @()
    $fragmentByKey = @{}
    $emitted = @{}

    foreach ($item in (Get-ArrayValue $FragmentItems)) {
        $key = [string]$item.$PropertyName
        if ($key) {
            $fragmentByKey[$key] = $item
        }
    }

    foreach ($item in (Get-ArrayValue $CurrentItems)) {
        $key = [string]$item.$PropertyName
        if ($key -and $fragmentByKey.ContainsKey($key)) {
            $result += $fragmentByKey[$key]
            $emitted[$key] = $true
        } else {
            $result += $item
        }
    }

    foreach ($item in (Get-ArrayValue $FragmentItems)) {
        $key = [string]$item.$PropertyName
        if (-not $key -or -not $emitted.ContainsKey($key)) {
            $result += $item
        }
    }

    return $result
}

function Get-WTActionKeySet {
    param($Item)
    $keys = @()
    if ($null -eq $Item -or $null -eq $Item.keys) {
        return @()
    }
    foreach ($key in (Get-ArrayValue $Item.keys)) {
        if ($null -eq $key) {
            continue
        }
        $keyText = ([string]$key).Trim()
        if ($keyText) {
            $keys += $keyText.ToLowerInvariant()
        }
    }
    return @($keys | Sort-Object -Unique)
}

function Test-WTActionKeyOverlap {
    param($LeftKeys, $RightKeys)
    foreach ($leftKey in (Get-ArrayValue $LeftKeys)) {
        foreach ($rightKey in (Get-ArrayValue $RightKeys)) {
            if ($leftKey -eq $rightKey) {
                return $true
            }
        }
    }
    return $false
}

function Merge-WTActions {
    param($CurrentItems, $FragmentItems)
    $result = @()
    $fragmentEntries = @()
    $emitted = @{}
    $index = 0

    foreach ($item in (Get-ArrayValue $FragmentItems)) {
        $fragmentEntries += [pscustomobject]@{
            Index = [string]$index
            Item = $item
            Keys = @(Get-WTActionKeySet $item)
        }
        $index += 1
    }

    foreach ($item in (Get-ArrayValue $CurrentItems)) {
        $currentKeys = @(Get-WTActionKeySet $item)
        $matches = @()
        foreach ($fragmentEntry in $fragmentEntries) {
            if ($currentKeys.Count -gt 0 -and $fragmentEntry.Keys.Count -gt 0 -and (Test-WTActionKeyOverlap $currentKeys $fragmentEntry.Keys)) {
                $matches += $fragmentEntry
            }
        }
        if ($matches.Count -gt 0) {
            foreach ($match in $matches) {
                if (-not $emitted.ContainsKey($match.Index)) {
                    $result += $match.Item
                    $emitted[$match.Index] = $true
                }
            }
        } else {
            $result += $item
        }
    }

    foreach ($fragmentEntry in $fragmentEntries) {
        if (-not $emitted.ContainsKey($fragmentEntry.Index)) {
            $result += $fragmentEntry.Item
        }
    }

    return $result
}

function Strip-Jsonc {
    param([string]$Jsonc)
    return (($Jsonc -split "`n" | Where-Object { $_ -notmatch "^\s*//" }) -join "`n")
}

function Test-WtDefaultProfileShouldChange {
    param($CurrentValue, [string]$ManagedValue)
    if ([string]::IsNullOrWhiteSpace([string]$ManagedValue)) {
        return $false
    }
    if ([string]::IsNullOrWhiteSpace([string]$CurrentValue)) {
        return $true
    }

    $currentText = ([string]$CurrentValue).Trim()
    if ($currentText.Equals($ManagedValue, [StringComparison]::OrdinalIgnoreCase)) {
        return $false
    }

    foreach ($legacyDefault in @($script:LegacyWindowsPowerShellProfileGuid, 'Windows PowerShell')) {
        if ($currentText.Equals($legacyDefault, [StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }

    return $false
}

function Invoke-ExpectedWindowsTerminalMergeOnly {
    param([Parameter(Mandatory)] [string]$SettingsPath)

    $fragmentPath = Join-Path $script:RepoRoot 'windows-terminal\settings.fragment.jsonc'
    $fragment = (Strip-Jsonc (Get-Content -Raw -LiteralPath $fragmentPath)) | ConvertFrom-Json
    $current = Read-JsonFile -Path $SettingsPath

    if ($null -eq $current.profiles) {
        $current | Add-Member -NotePropertyName profiles -NotePropertyValue ([pscustomobject]@{}) -Force
    }

    if ($null -ne $fragment.copyFormatting)        { Set-OrAdd-Property $current 'copyFormatting'        $fragment.copyFormatting }
    if ($null -ne $fragment.copyOnSelect)          { Set-OrAdd-Property $current 'copyOnSelect'          $fragment.copyOnSelect }
    if ($null -ne $fragment.firstWindowPreference) { Set-OrAdd-Property $current 'firstWindowPreference' $fragment.firstWindowPreference }
    if ($null -ne $fragment.initialRows)           { Set-OrAdd-Property $current 'initialRows'           $fragment.initialRows }
    if ($null -ne $fragment.launchMode)            { Set-OrAdd-Property $current 'launchMode'            $fragment.launchMode }
    if ($null -ne $fragment.theme)                 { Set-OrAdd-Property $current 'theme'                 $fragment.theme }
    if ($null -ne $fragment.useAcrylicInTabRow)    { Set-OrAdd-Property $current 'useAcrylicInTabRow'    $fragment.useAcrylicInTabRow }
    if ($null -ne $fragment.windowingBehavior)     { Set-OrAdd-Property $current 'windowingBehavior'     $fragment.windowingBehavior }
    if ($null -ne $fragment.defaultProfile -and
        (Test-WtDefaultProfileShouldChange -CurrentValue $current.defaultProfile -ManagedValue $fragment.defaultProfile)) {
        Set-OrAdd-Property $current 'defaultProfile' $fragment.defaultProfile
    }

    if ($null -eq $current.profiles.defaults) {
        $current.profiles | Add-Member -NotePropertyName defaults -NotePropertyValue $fragment.profiles.defaults -Force
    } else {
        $current.profiles.defaults = $fragment.profiles.defaults
    }
    if ($null -ne $fragment.profiles.list) {
        Set-OrAdd-Property $current.profiles 'list' @(Merge-ObjectArrayByProperty $current.profiles.list $fragment.profiles.list 'guid')
    }
    Set-OrAdd-Property $current 'actions' @(Merge-WTActions $current.actions $fragment.actions)
    Set-OrAdd-Property $current 'schemes' @(Merge-ObjectArrayByProperty $current.schemes $fragment.schemes 'name')
    Set-OrAdd-Property $current 'themes'  @(Merge-ObjectArrayByProperty $current.themes  $fragment.themes  'name')
    $current | ConvertTo-Json -Depth 100 | Set-Content -LiteralPath $SettingsPath -Encoding utf8
}

function ConvertTo-NormalizedValue {
    param($Value)
    if ($null -eq $Value) {
        return $null
    }
    if ($Value -is [System.Management.Automation.PSCustomObject]) {
        $out = [ordered]@{}
        foreach ($prop in ($Value.PSObject.Properties | Sort-Object Name)) {
            $out[$prop.Name] = ConvertTo-NormalizedValue $prop.Value
        }
        return $out
    }
    if ($Value -is [System.Collections.IDictionary]) {
        $out = [ordered]@{}
        foreach ($key in ($Value.Keys | Sort-Object)) {
            $out[[string]$key] = ConvertTo-NormalizedValue $Value[$key]
        }
        return $out
    }
    if (($Value -is [System.Collections.IEnumerable]) -and -not ($Value -is [string])) {
        $items = @(foreach ($item in $Value) { ConvertTo-NormalizedValue $item })
        return @($items | Sort-Object { ConvertTo-CanonicalJson $_ })
    }
    return $Value
}

function ConvertTo-CanonicalJson {
    param($Value)
    return (ConvertTo-NormalizedValue $Value | ConvertTo-Json -Depth 100 -Compress)
}

function Select-WtManagedSubset {
    param([Parameter(Mandatory)] $Settings)

    $globals = [ordered]@{}
    foreach ($name in $script:ManagedGlobals) {
        Assert-Condition ($Settings.PSObject.Properties.Name -contains $name) "managed WT global missing: $name"
        $globals[$name] = $Settings.$name
    }

    return [ordered]@{
        globals = $globals
        profilesDefaults = $Settings.profiles.defaults
        managedPwshProfile = Get-NamedItem -Items $Settings.profiles.list -Name 'PowerShell 7' -Label 'managed WT PowerShell 7 profile'
        actions = @(Get-ArrayValue $Settings.actions)
        rosePineScheme = Get-NamedItem -Items $Settings.schemes -Name 'rose-pine' -Label 'managed WT scheme'
        rosePineTheme = Get-NamedItem -Items $Settings.themes -Name 'rose-pine' -Label 'managed WT theme'
    }
}

function Assert-WtManagedSubsetDeepEqual {
    param(
        [Parameter(Mandatory)] $Expected,
        [Parameter(Mandatory)] $Chezmoi
    )

    $expectedSubset = Select-WtManagedSubset -Settings $Expected
    $chezmoiSubset = Select-WtManagedSubset -Settings $Chezmoi
    $expectedJson = ConvertTo-CanonicalJson $expectedSubset
    $chezmoiJson = ConvertTo-CanonicalJson $chezmoiSubset
    Assert-Condition ($expectedJson -eq $chezmoiJson) "WT managed subset mismatch.`nexpected: $expectedJson`nchezmoi:  $chezmoiJson"
}

function Invoke-Part1 {
    $sandbox = New-TestSandbox -Name 'part1'
    try {
        Invoke-WithSandboxEnv -Sandbox $sandbox -Script {
            $settingsPath = Write-BaselineWtSettings -Sandbox $sandbox
            Invoke-Chezmoi -Arguments @('init')
            Invoke-Chezmoi -Arguments @('apply')
            Assert-Part1Files -Sandbox $sandbox
            Assert-Part1WtMerge -SettingsPath $settingsPath
            # Second apply must be a prompt-free no-op (NO --force; see wrapper).
            Invoke-ChezmoiReapply -Arguments @('apply')
            Invoke-Chezmoi -Arguments @('verify')
        }
        Pass 'part 1 real apply smoke passed'
    } finally {
        Remove-TestSandbox -Sandbox $sandbox
    }
}

function Invoke-Part2 {
    $expectedSandbox = New-TestSandbox -Name 'expected'
    $chezmoiSandbox = New-TestSandbox -Name 'chezmoi'
    try {
        $expectedSettings = Write-BaselineWtSettings -Sandbox $expectedSandbox
        $chezmoiSettings = Write-BaselineWtSettings -Sandbox $chezmoiSandbox

        Invoke-WithSandboxEnv -Sandbox $expectedSandbox -Script {
            Invoke-ExpectedWindowsTerminalMergeOnly -SettingsPath $expectedSettings
            Pass 'expected WT merge fixture completed'
        }

        Invoke-WithSandboxEnv -Sandbox $chezmoiSandbox -Script {
            Invoke-Chezmoi -Arguments @('init')
            Invoke-Chezmoi -Arguments @('apply')
        }

        $expected = Read-JsonFile -Path $expectedSettings
        $chezmoi = Read-JsonFile -Path $chezmoiSettings
        Assert-WtManagedSubsetDeepEqual -Expected $expected -Chezmoi $chezmoi
        Assert-WtUserSeedSurvived -Settings $expected -Label 'expected WT merge'
        Assert-WtUserSeedSurvived -Settings $chezmoi -Label 'chezmoi WT merge'
        Assert-WtManagedPwshProfilePresent -Settings $expected -Label 'expected WT merge'
        Assert-WtManagedPwshProfilePresent -Settings $chezmoi -Label 'chezmoi WT merge'
        Assert-WtManagedActionKeySet -Settings $expected -Label 'expected WT merge'
        Assert-WtManagedActionKeySet -Settings $chezmoi -Label 'chezmoi WT merge'
        Pass 'part 2 WT managed subset deep-compare passed'
    } finally {
        Remove-TestSandbox -Sandbox $expectedSandbox
        Remove-TestSandbox -Sandbox $chezmoiSandbox
    }
}

try {
    $script:Chezmoi = (Get-Command chezmoi -ErrorAction Stop).Source
    Assert-Condition ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) 'windows_apply_test.ps1 must run on Windows'
    Invoke-Part1
    Invoke-Part2
    Pass 'windows_apply_test.ps1 completed'
} catch {
    Write-Host "FAIL: $($_.Exception.Message)"
    if ($_.ScriptStackTrace) {
        Write-Host $_.ScriptStackTrace
    }
    exit 1
}
