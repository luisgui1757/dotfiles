function Remove-WindowsTerminalJsoncCommentLines {
    param([AllowNull()][string]$Jsonc)
    if ($null -eq $Jsonc) {
        return ''
    }
    return (($Jsonc -split "`n" | Where-Object { $_ -notmatch "^\s*//" }) -join "`n")
}

function ConvertFrom-WindowsTerminalJsonc {
    param([AllowNull()][string]$Jsonc)
    $json = Remove-WindowsTerminalJsoncCommentLines -Jsonc $Jsonc
    if ([string]::IsNullOrWhiteSpace($json)) {
        return [pscustomobject]@{}
    }
    return ($json | ConvertFrom-Json)
}

function Set-WindowsTerminalProperty {
    param($Object, [string]$Name, $Value)
    if ($null -eq $Object.$Name) {
        $Object | Add-Member -NotePropertyName $Name -NotePropertyValue $Value -Force
    } else {
        $Object.$Name = $Value
    }
}

function Get-WindowsTerminalArrayValue {
    param($Value)
    if ($null -eq $Value) {
        return @()
    }
    return @($Value)
}

function Merge-WindowsTerminalObjectArrayByProperty {
    param($CurrentItems, $FragmentItems, [string]$PropertyName)
    $result = @()
    $fragmentByKey = @{}
    $emitted = @{}

    foreach ($item in (Get-WindowsTerminalArrayValue $FragmentItems)) {
        $key = [string]$item.$PropertyName
        if ($key) {
            $fragmentByKey[$key] = $item
        }
    }

    foreach ($item in (Get-WindowsTerminalArrayValue $CurrentItems)) {
        $key = [string]$item.$PropertyName
        if ($key -and $fragmentByKey.ContainsKey($key)) {
            $result += $fragmentByKey[$key]
            $emitted[$key] = $true
        } else {
            $result += $item
        }
    }

    foreach ($item in (Get-WindowsTerminalArrayValue $FragmentItems)) {
        $key = [string]$item.$PropertyName
        if (-not $key -or -not $emitted.ContainsKey($key)) {
            $result += $item
        }
    }

    return $result
}

function Get-WindowsTerminalActionKeySet {
    param($Item)
    $keys = @()
    if ($null -eq $Item -or $null -eq $Item.keys) {
        return @()
    }
    foreach ($key in (Get-WindowsTerminalArrayValue $Item.keys)) {
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

function Test-WindowsTerminalActionKeyOverlap {
    param($LeftKeys, $RightKeys)
    foreach ($leftKey in (Get-WindowsTerminalArrayValue $LeftKeys)) {
        foreach ($rightKey in (Get-WindowsTerminalArrayValue $RightKeys)) {
            if ($leftKey -eq $rightKey) {
                return $true
            }
        }
    }
    return $false
}

function Merge-WindowsTerminalActions {
    param($CurrentItems, $FragmentItems)
    $result = @()
    $fragmentEntries = @()
    $emitted = @{}
    $index = 0

    foreach ($item in (Get-WindowsTerminalArrayValue $FragmentItems)) {
        $fragmentEntries += [pscustomobject]@{
            Index = [string]$index
            Item = $item
            Keys = @(Get-WindowsTerminalActionKeySet $item)
        }
        $index += 1
    }

    foreach ($item in (Get-WindowsTerminalArrayValue $CurrentItems)) {
        $currentKeys = @(Get-WindowsTerminalActionKeySet $item)
        $matches = @()
        foreach ($fragmentEntry in $fragmentEntries) {
            if ($currentKeys.Count -gt 0 -and $fragmentEntry.Keys.Count -gt 0 -and (Test-WindowsTerminalActionKeyOverlap $currentKeys $fragmentEntry.Keys)) {
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

function Merge-WindowsTerminalSettingsObject {
    param($Current, $Fragment)

    if ($null -eq $Current) {
        $Current = [pscustomobject]@{}
    }

    if ($null -eq $Current.profiles) {
        $Current | Add-Member -NotePropertyName profiles -NotePropertyValue ([pscustomobject]@{}) -Force
    }

    if ($null -ne $Fragment.copyFormatting)        { Set-WindowsTerminalProperty $Current "copyFormatting"        $Fragment.copyFormatting }
    if ($null -ne $Fragment.copyOnSelect)          { Set-WindowsTerminalProperty $Current "copyOnSelect"          $Fragment.copyOnSelect }
    if ($null -ne $Fragment.firstWindowPreference) { Set-WindowsTerminalProperty $Current "firstWindowPreference" $Fragment.firstWindowPreference }
    if ($null -ne $Fragment.initialRows)           { Set-WindowsTerminalProperty $Current "initialRows"           $Fragment.initialRows }
    if ($null -ne $Fragment.launchMode)            { Set-WindowsTerminalProperty $Current "launchMode"            $Fragment.launchMode }
    if ($null -ne $Fragment.theme)                 { Set-WindowsTerminalProperty $Current "theme"                 $Fragment.theme }
    if ($null -ne $Fragment.useAcrylicInTabRow)    { Set-WindowsTerminalProperty $Current "useAcrylicInTabRow"    $Fragment.useAcrylicInTabRow }
    if ($null -ne $Fragment.windowingBehavior)     { Set-WindowsTerminalProperty $Current "windowingBehavior"     $Fragment.windowingBehavior }

    if ($null -eq $Current.profiles.defaults) {
        $Current.profiles | Add-Member -NotePropertyName defaults -NotePropertyValue $Fragment.profiles.defaults -Force
    } else {
        $Current.profiles.defaults = $Fragment.profiles.defaults
    }

    Set-WindowsTerminalProperty $Current "actions" @(Merge-WindowsTerminalActions $Current.actions $Fragment.actions)
    Set-WindowsTerminalProperty $Current "schemes" @(Merge-WindowsTerminalObjectArrayByProperty $Current.schemes $Fragment.schemes "name")
    Set-WindowsTerminalProperty $Current "themes"  @(Merge-WindowsTerminalObjectArrayByProperty $Current.themes  $Fragment.themes  "name")

    return $Current
}
