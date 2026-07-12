# Transactional v0.1.0 to v0.2.0 migration for native Windows.
# Run only from a separate, exact v0.2.0 release checkout.
[CmdletBinding(DefaultParameterSetName = 'Source')]
param(
    [Parameter(ParameterSetName = 'Source', Mandatory)] [switch]$SourceOnly,
    [Parameter(ParameterSetName = 'Preflight', Mandatory)] [switch]$PreflightOnly,
    [Parameter(ParameterSetName = 'Apply', Mandatory)] [switch]$Apply,
    [Parameter(ParameterSetName = 'Preflight', Mandatory)]
    [Parameter(ParameterSetName = 'Apply', Mandatory)]
    [string]$OldCheckout,
    [Parameter(ParameterSetName = 'Rollback', Mandatory)] [string]$Rollback,
    [Parameter(ParameterSetName = 'Accept', Mandatory)] [string]$Accept
)

$ErrorActionPreference = 'Stop'
$script:OldTag = 'v0.1.0'
$script:OldTagObject = 'a3b4d6d7b6d289959cac68d76faec96219b3e310'
$script:OldCommit = '015617362830280bf85c7142e69d0681d376d453'
$script:NewTag = 'v0.2.0'
$script:OfficialRepo = 'https://github.com/luisgui1757/dotfiles.git'
$script:UpgradeScript = [IO.Path]::GetFullPath($MyInvocation.MyCommand.Path)
$script:DefaultNewCheckout = [IO.Path]::GetFullPath((Join-Path (Split-Path -Parent $script:UpgradeScript) '..'))
$script:SetupLibraryLoaded = $false
$script:SetupLibraryModule = $null
$script:SetupLibraryCheckout = ''
$script:WindowsIdentity = $null
$null = $PreflightOnly
$null = $Apply
$null = $SourceOnly

function Invoke-NativeCapture {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)] [string]$FilePath,
        [Parameter(Mandatory)] [string[]]$Arguments,
        [switch]$AllowFailure,
        [switch]$PassThrough
    )

    $stderrPath = [IO.Path]::GetTempFileName()
    $oldNativePreference = $null
    $hasNativePreference = Test-Path Variable:PSNativeCommandUseErrorActionPreference
    try {
        try {
            if ($hasNativePreference) {
                $oldNativePreference = $PSNativeCommandUseErrorActionPreference
                $PSNativeCommandUseErrorActionPreference = $false
            }
            $global:LASTEXITCODE = 0
            $stdout = @(& $FilePath @Arguments 2> $stderrPath)
            $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
        } finally {
            if ($hasNativePreference) {
                $PSNativeCommandUseErrorActionPreference = $oldNativePreference
            }
            $global:LASTEXITCODE = 0
        }
        $stderr = if (Test-Path -LiteralPath $stderrPath) {
            [IO.File]::ReadAllText($stderrPath)
        } else { '' }
        if ($PassThrough) {
            $stdout | ForEach-Object { Write-Information $_ -InformationAction Continue }
            if (-not [string]::IsNullOrWhiteSpace($stderr)) {
                Write-Information $stderr.TrimEnd() -InformationAction Continue
            }
        }
        if ($exitCode -ne 0 -and -not $AllowFailure) {
            throw "$FilePath exited $exitCode`: $stderr"
        }
        return [pscustomobject]@{
            ExitCode = $exitCode
            Stdout = @($stdout)
            Stderr = $stderr
        }
    } finally {
        Remove-Item -LiteralPath $stderrPath -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-GitCapture {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string[]]$Arguments,
        [switch]$AllowFailure
    )
    $git = (Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    $gitArgs = @(
        '-C', $Checkout,
        '-c', 'core.fsmonitor=false',
        '-c', 'core.untrackedCache=false',
        '-c', 'core.hooksPath=NUL',
        '-c', 'init.templateDir='
    ) + $Arguments
    return Invoke-NativeCapture -FilePath $git -Arguments $gitArgs -AllowFailure:$AllowFailure
}

function Get-CanonicalDirectory {
    param([Parameter(Mandatory)] [string]$Path)
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if (-not $item.PSIsContainer -or $item.LinkType) {
        throw "directory is missing or unsafe: $Path"
    }
    return $item.FullName.TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
}

function Test-OfficialRemote {
    param([Parameter(Mandatory)] [string]$Remote)
    $normalized = $Remote.Trim()
    foreach ($prefix in @(
            'https://github.com/',
            'git@github.com:',
            'ssh://git@github.com/'
        )) {
        if ($normalized.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
            $normalized = $normalized.Substring($prefix.Length)
            break
        }
    }
    $normalized = $normalized -replace '\.git$', ''
    return $normalized -ieq 'luisgui1757/dotfiles'
}

function Get-RemoteTagIdentity {
    param([Parameter(Mandatory)] [string]$Tag)
    $git = (Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    $result = Invoke-NativeCapture -FilePath $git -Arguments @(
        'ls-remote', '--tags', $script:OfficialRepo,
        "refs/tags/$Tag", "refs/tags/$Tag^{}"
    )
    $tagObject = ''
    $commit = ''
    foreach ($line in $result.Stdout) {
        $fields = [string]$line -split "`t"
        if ($fields.Count -ne 2) { continue }
        if ($fields[1] -eq "refs/tags/$Tag") { $tagObject = $fields[0] }
        if ($fields[1] -eq "refs/tags/$Tag^{}") { $commit = $fields[0] }
    }
    if ($tagObject -notmatch '^[0-9a-f]{40}$' -or $commit -notmatch '^[0-9a-f]{40}$') {
        throw "$Tag must be an annotated official release tag"
    }
    return [pscustomobject]@{ TagObject = $tagObject; Commit = $commit }
}

function Assert-ReleaseCheckout {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string]$Tag,
        [string]$ExpectedCommit = '',
        [string]$ExpectedTagObject = '',
        [Parameter(Mandatory)] [string]$Label
    )
    $head = [string](Invoke-GitCapture -Checkout $Checkout -Arguments @('rev-parse', 'HEAD^{commit}')).Stdout[0]
    $tagCommit = [string](Invoke-GitCapture -Checkout $Checkout -Arguments @('rev-parse', "refs/tags/$Tag^{commit}")).Stdout[0]
    $tagType = [string](Invoke-GitCapture -Checkout $Checkout -Arguments @('cat-file', '-t', "refs/tags/$Tag")).Stdout[0]
    $tagObject = [string](Invoke-GitCapture -Checkout $Checkout -Arguments @('rev-parse', "refs/tags/$Tag")).Stdout[0]
    if ($head -ne $tagCommit -or $tagType -ne 'tag') {
        throw "$Label checkout is not the exact annotated $Tag release: $Checkout"
    }
    if ($ExpectedCommit -and $head -ne $ExpectedCommit) {
        throw "$Label checkout commit is $head; expected $ExpectedCommit"
    }
    if ($ExpectedTagObject -and $tagObject -ne $ExpectedTagObject) {
        throw "$Label tag object is $tagObject; expected $ExpectedTagObject"
    }
    $origin = [string](Invoke-GitCapture -Checkout $Checkout -Arguments @('remote', 'get-url', 'origin')).Stdout[0]
    if (-not (Test-OfficialRemote -Remote $origin)) {
        throw "$Label checkout origin is not the official repository: $origin"
    }
    $remote = Get-RemoteTagIdentity -Tag $Tag
    if ($tagObject -ne $remote.TagObject -or $head -ne $remote.Commit) {
        throw "$Label checkout does not match the official immutable $Tag identity"
    }
    $status = Invoke-GitCapture -Checkout $Checkout -Arguments @('status', '--porcelain=v1', '--untracked-files=all')
    if ($status.Stdout.Count -gt 0) {
        throw "$Label checkout has tracked or untracked changes: $Checkout`n$($status.Stdout -join "`n")"
    }
}

function Initialize-SetupLibrary {
    param([Parameter(Mandatory)] [string]$NewCheckout)
    if ($script:SetupLibraryLoaded) {
        if ($script:SetupLibraryCheckout -ieq $NewCheckout) { return }
        $script:SetupLibraryLoaded = $false
        $script:SetupLibraryModule = $null
        $script:SetupLibraryCheckout = ''
        $script:WindowsIdentity = $null
    }
    $oldSourceOnly = $env:DOTFILES_SETUP_PS1_SOURCE_ONLY
    try {
        $env:DOTFILES_SETUP_PS1_SOURCE_ONLY = '1'
        $setupPath = Join-Path $NewCheckout 'setup.ps1'
        $script:SetupLibraryModule = New-Module -ArgumentList $setupPath -ScriptBlock {
            param([string]$LibraryPath)
            . $LibraryPath
        }
    } finally {
        if ($null -eq $oldSourceOnly) {
            Remove-Item Env:DOTFILES_SETUP_PS1_SOURCE_ONLY -ErrorAction SilentlyContinue
        } else {
            $env:DOTFILES_SETUP_PS1_SOURCE_ONLY = $oldSourceOnly
        }
    }
    $script:SetupLibraryLoaded = $true
    $script:SetupLibraryCheckout = $NewCheckout
    $script:WindowsIdentity = & $script:SetupLibraryModule { Resolve-WindowsTargetIdentity }
}

function Invoke-ChezmoiChecked {
    param([Parameter(Mandatory)] [string[]]$Arguments)
    $chezmoi = (Get-Command chezmoi -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    return Invoke-NativeCapture -FilePath $chezmoi -Arguments $Arguments
}

function Test-OldConfig {
    param(
        [Parameter(Mandatory)] [string]$OldCheckout,
        [Parameter(Mandatory)] [string]$UserProfile
    )
    $chezmoi = (Get-Command chezmoi -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    $result = Invoke-NativeCapture -FilePath $chezmoi -Arguments @(
        '--source', (Join-Path $OldCheckout 'home'),
        '--destination', $UserProfile,
        'verify', '--include', 'files,symlinks'
    ) -AllowFailure
    return $result.ExitCode -eq 0
}

function Invoke-UpgradePreflight {
    param(
        [Parameter(Mandatory)] [string]$OldPath,
        [Parameter(Mandatory)] [string]$NewPath
    )
    if ($env:OS -ne 'Windows_NT') {
        throw 'native Windows is required for the PowerShell release migration'
    }
    $null = Get-Command git -CommandType Application -ErrorAction Stop
    $null = Get-Command chezmoi -CommandType Application -ErrorAction Stop
    $old = Get-CanonicalDirectory -Path $OldPath
    $new = Get-CanonicalDirectory -Path $NewPath
    if ($old -ieq $new) {
        throw 'in-place migration is forbidden; retain v0.1.0 and use a separate v0.2.0 checkout'
    }
    Assert-ReleaseCheckout -Checkout $old -Tag $script:OldTag `
        -ExpectedCommit $script:OldCommit -ExpectedTagObject $script:OldTagObject -Label old
    Assert-ReleaseCheckout -Checkout $new -Tag $script:NewTag -Label new
    Initialize-SetupLibrary -NewCheckout $new
    $canCreateSymlinks = & $script:SetupLibraryModule { Test-CanCreateSymlinks }
    if (-not $canCreateSymlinks) {
        throw 'Developer Mode or elevated config authority is required before migration'
    }
    if (-not (Test-OldConfig -OldCheckout $old -UserProfile $script:WindowsIdentity.UserProfile)) {
        throw 'live config does not exactly match the retained v0.1.0 checkout; no mutation was attempted'
    }
    $knownFolderStateRoot = Join-Path $script:WindowsIdentity.LocalApplicationData 'dotfiles\chezmoi-state'
    if (Test-Path -LiteralPath $knownFolderStateRoot) {
        throw "current-generation known-folder state already exists outside the v0.1.0 migration contract: $knownFolderStateRoot"
    }
    $newCommit = [string](Invoke-GitCapture -Checkout $new -Arguments @('rev-parse', 'HEAD^{commit}')).Stdout[0]
    $newTagObject = [string](Invoke-GitCapture -Checkout $new -Arguments @('rev-parse', "refs/tags/$($script:NewTag)")).Stdout[0]
    return [pscustomobject]@{
        OldCheckout = $old
        NewCheckout = $new
        OldCommit = $script:OldCommit
        OldTagObject = $script:OldTagObject
        NewCommit = $newCommit
        NewTagObject = $newTagObject
        Identity = $script:WindowsIdentity
        KnownFolderStateRoot = $knownFolderStateRoot
    }
}

function Write-PrivateText {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [Parameter(Mandatory)] [string]$Value
    )
    if ($Value.Contains("`n") -or $Value.Contains("`r")) {
        throw "recovery value contains a newline: $Path"
    }
    [IO.File]::WriteAllText($Path, "$Value`n", [Text.UTF8Encoding]::new($false))
}

function Save-RecoveryStage {
    param(
        [Parameter(Mandatory)] [string]$Recovery,
        [Parameter(Mandatory)] [string]$Stage
    )
    $temporary = Join-Path $Recovery 'stage.tmp'
    Write-PrivateText -Path $temporary -Value $Stage
    Move-Item -LiteralPath $temporary -Destination (Join-Path $Recovery 'stage') -Force
}

function Get-FileSha256OrEmpty {
    param([Parameter(Mandatory)] [string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return '' }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Test-ReparsePoint {
    param([Parameter(Mandatory)] [string]$Path)
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    return [bool]($item.Attributes -band [IO.FileAttributes]::ReparsePoint)
}

function Assert-RegularFile {
    param([Parameter(Mandatory)] [string]$Path)
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "recovery input is not a regular file: $Path"
    }
}

function Read-RecoveryScalar {
    param([Parameter(Mandatory)] [string]$Path)
    Assert-RegularFile -Path $Path
    $raw = [IO.File]::ReadAllText($Path)
    if ($raw -notmatch '\A[^\r\n]+\r?\n\z') {
        throw "recovery scalar is malformed: $Path"
    }
    return $raw.TrimEnd("`r", "`n")
}

function Get-BytesSha256 {
    param([Parameter(Mandatory)] [byte[]]$Bytes)
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function Assert-PrivateRecoveryAcl {
    param([Parameter(Mandatory)] [string]$Path)
    $currentSid = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $allowedSids = @(
        $currentSid.Value,
        'S-1-5-18',
        'S-1-5-32-544'
    )
    $acl = Get-Acl -LiteralPath $Path
    $ownerSid = $acl.GetOwner([Security.Principal.SecurityIdentifier]).Value
    $rules = @($acl.Access)
    if (-not $acl.AreAccessRulesProtected -or $ownerSid -ne $currentSid.Value -or
        $rules.Count -ne $allowedSids.Count) {
        throw 'recovery directory permissions are not private'
    }
    foreach ($rule in $rules) {
        $sid = $rule.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value
        if ($sid -notin $allowedSids -or $rule.IsInherited -or
            $rule.AccessControlType -ne [Security.AccessControl.AccessControlType]::Allow -or
            ($rule.FileSystemRights -band [Security.AccessControl.FileSystemRights]::FullControl) -ne
                [Security.AccessControl.FileSystemRights]::FullControl) {
            throw 'recovery directory permissions are not private'
        }
    }
    foreach ($sid in $allowedSids) {
        if (@($rules | Where-Object {
                    $_.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value -eq $sid
                }).Count -ne 1) {
            throw 'recovery directory permissions are not private'
        }
    }
}

function Initialize-PrivateRecoveryDirectory {
    param([Parameter(Mandatory)] [string]$Parent)
    New-Item -ItemType Directory -Force -Path $Parent | Out-Null
    if (Test-ReparsePoint -Path $Parent) {
        throw "recovery parent is a reparse point: $Parent"
    }
    do {
        $recovery = Join-Path $Parent ("v0.1.0-to-v0.2.0." + [guid]::NewGuid().ToString('N'))
    } while (Test-Path -LiteralPath $recovery)
    New-Item -ItemType Directory -Path $recovery | Out-Null

    $currentSid = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $systemSid = [Security.Principal.SecurityIdentifier]::new('S-1-5-18')
    $administratorsSid = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
    $inheritance = [Security.AccessControl.InheritanceFlags]::ContainerInherit -bor
        [Security.AccessControl.InheritanceFlags]::ObjectInherit
    $acl = [Security.AccessControl.DirectorySecurity]::new()
    $acl.SetOwner($currentSid)
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($sid in $currentSid, $systemSid, $administratorsSid) {
        $rule = [Security.AccessControl.FileSystemAccessRule]::new(
            $sid,
            [Security.AccessControl.FileSystemRights]::FullControl,
            $inheritance,
            [Security.AccessControl.PropagationFlags]::None,
            [Security.AccessControl.AccessControlType]::Allow
        )
        $null = $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $recovery -AclObject $acl
    try {
        Assert-PrivateRecoveryAcl -Path $recovery
    } catch {
        Remove-Item -LiteralPath $recovery -Recurse -Force -ErrorAction SilentlyContinue
        throw "could not establish a private recovery-directory ACL: $($_.Exception.Message)"
    }
    return $recovery
}

function Get-FrozenReleaseTreeFingerprint {
    param([Parameter(Mandatory)] [string]$Root)
    $directory = Get-CanonicalDirectory -Path $Root
    $lines = [Collections.Generic.List[string]]::new()
    foreach ($item in Get-ChildItem -LiteralPath $directory -Force -Recurse) {
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
            throw "frozen release contains a reparse point: $($item.FullName)"
        }
        if ($item.PSIsContainer) { continue }
        $relative = [IO.Path]::GetRelativePath($directory, $item.FullName).Replace('\', '/')
        if ([string]::IsNullOrWhiteSpace($relative) -or $relative.StartsWith('../') -or
            $relative.Contains("`t") -or $relative.Contains("`r") -or $relative.Contains("`n")) {
            throw "frozen release contains an unsafe path: $($item.FullName)"
        }
        $hash = (Get-FileHash -LiteralPath $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        $lines.Add("$hash`t$($item.Length)`t$relative")
    }
    $result = $lines.ToArray()
    [Array]::Sort($result, [StringComparer]::Ordinal)
    return $result
}

function Write-FrozenReleaseTreeManifest {
    param(
        [Parameter(Mandatory)] [string]$Root,
        [Parameter(Mandatory)] [string]$Path
    )
    $lines = @(Get-FrozenReleaseTreeFingerprint -Root $Root)
    if ($lines.Count -eq 0) { throw "frozen release tree is empty: $Root" }
    [IO.File]::WriteAllText($Path, (($lines -join "`n") + "`n"), [Text.UTF8Encoding]::new($false))
}

function Assert-FrozenReleaseTree {
    param(
        [Parameter(Mandatory)] [string]$Root,
        [Parameter(Mandatory)] [string]$ManifestPath
    )
    Assert-RegularFile -Path $ManifestPath
    $expected = [IO.File]::ReadAllText($ManifestPath)
    $lines = @(Get-FrozenReleaseTreeFingerprint -Root $Root)
    $actual = if ($lines.Count -eq 0) { '' } else { ($lines -join "`n") + "`n" }
    if ($actual -cne $expected) {
        throw "frozen release tree differs from its validated manifest: $Root"
    }
}

function Export-FrozenReleaseTree {
    param(
        [Parameter(Mandatory)] [string]$Checkout,
        [Parameter(Mandatory)] [string]$Commit,
        [Parameter(Mandatory)] [string]$ArchivePath,
        [Parameter(Mandatory)] [string]$Destination
    )
    $null = Invoke-GitCapture -Checkout $Checkout -Arguments @(
        'archive', '--format=tar', "--output=$ArchivePath", $Commit
    )
    Assert-RegularFile -Path $ArchivePath
    New-Item -ItemType Directory -Path $Destination | Out-Null
    $tar = (Get-Command tar -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    $null = Invoke-NativeCapture -FilePath $tar -Arguments @(
        '-xf', $ArchivePath, '-C', $Destination
    )
}

function Save-FrozenReleaseState {
    param(
        [Parameter(Mandatory)] [string]$Recovery,
        [Parameter(Mandatory)] $Preflight
    )
    $oldArchive = Join-Path $Recovery 'old-release.tar'
    $newArchive = Join-Path $Recovery 'new-release.tar'
    $oldSource = Join-Path $Recovery 'old-release'
    $newSource = Join-Path $Recovery 'new-release'
    $oldTreeManifest = Join-Path $Recovery 'old-release.tree'
    $newTreeManifest = Join-Path $Recovery 'new-release.tree'
    Export-FrozenReleaseTree -Checkout $Preflight.OldCheckout -Commit $Preflight.OldCommit `
        -ArchivePath $oldArchive -Destination $oldSource
    Export-FrozenReleaseTree -Checkout $Preflight.NewCheckout -Commit $Preflight.NewCommit `
        -ArchivePath $newArchive -Destination $newSource
    Write-FrozenReleaseTreeManifest -Root $oldSource -Path $oldTreeManifest
    Write-FrozenReleaseTreeManifest -Root $newSource -Path $newTreeManifest
    $manifest = [ordered]@{
        Version = 1
        OldCommit = $Preflight.OldCommit
        OldTagObject = $Preflight.OldTagObject
        NewCommit = $Preflight.NewCommit
        NewTagObject = $Preflight.NewTagObject
        OldArchiveSha256 = Get-FileSha256OrEmpty -Path $oldArchive
        NewArchiveSha256 = Get-FileSha256OrEmpty -Path $newArchive
        OldTreeManifestSha256 = Get-FileSha256OrEmpty -Path $oldTreeManifest
        NewTreeManifestSha256 = Get-FileSha256OrEmpty -Path $newTreeManifest
    }
    [IO.File]::WriteAllText(
        (Join-Path $Recovery 'frozen-releases.json'),
        ($manifest | ConvertTo-Json -Compress),
        [Text.UTF8Encoding]::new($false)
    )
    return [pscustomobject]@{ OldSource = $oldSource; NewSource = $newSource }
}

function Get-ValidatedFrozenReleaseState {
    param([Parameter(Mandatory)] [string]$Recovery)
    $manifestPath = Join-Path $Recovery 'frozen-releases.json'
    Assert-RegularFile -Path $manifestPath
    $manifest = [IO.File]::ReadAllText($manifestPath) | ConvertFrom-Json -ErrorAction Stop
    $properties = @($manifest.PSObject.Properties.Name | Sort-Object)
    $expectedProperties = @(
        'NewArchiveSha256', 'NewCommit', 'NewTagObject', 'NewTreeManifestSha256',
        'OldArchiveSha256', 'OldCommit', 'OldTagObject', 'OldTreeManifestSha256', 'Version'
    )
    if (($properties -join ',') -ne ($expectedProperties -join ',') -or
        ($manifest.Version -isnot [int] -and $manifest.Version -isnot [long]) -or
        [long]$manifest.Version -ne 1 -or
        [string]$manifest.OldCommit -ne $script:OldCommit -or
        [string]$manifest.OldTagObject -ne $script:OldTagObject -or
        [string]$manifest.NewCommit -notmatch '^[0-9a-f]{40}$' -or
        [string]$manifest.NewTagObject -notmatch '^[0-9a-f]{40}$') {
        throw 'frozen release manifest is malformed'
    }
    $files = [ordered]@{
        'old-release.tar' = [string]$manifest.OldArchiveSha256
        'new-release.tar' = [string]$manifest.NewArchiveSha256
        'old-release.tree' = [string]$manifest.OldTreeManifestSha256
        'new-release.tree' = [string]$manifest.NewTreeManifestSha256
    }
    foreach ($entry in $files.GetEnumerator()) {
        $path = Join-Path $Recovery $entry.Key
        Assert-RegularFile -Path $path
        if ($entry.Value -notmatch '^[0-9a-f]{64}$' -or
            (Get-FileSha256OrEmpty -Path $path) -cne $entry.Value) {
            throw "frozen release payload differs from its manifest: $($entry.Key)"
        }
    }
    $oldSource = Join-Path $Recovery 'old-release'
    $newSource = Join-Path $Recovery 'new-release'
    Assert-FrozenReleaseTree -Root $oldSource -ManifestPath (Join-Path $Recovery 'old-release.tree')
    Assert-FrozenReleaseTree -Root $newSource -ManifestPath (Join-Path $Recovery 'new-release.tree')
    return [pscustomobject]@{
        OldSource = $oldSource
        NewSource = $newSource
        OldCommit = [string]$manifest.OldCommit
        NewCommit = [string]$manifest.NewCommit
        NewTagObject = [string]$manifest.NewTagObject
    }
}

function Save-WindowsTerminalRecovery {
    param([Parameter(Mandatory)] [string]$Recovery)
    $fragment = & $script:SetupLibraryModule { Get-WindowsTerminalSettingsFragmentPath }
    $targets = @(& $script:SetupLibraryModule {
            Get-WindowsTerminalSettingsPath
            Get-WindowsTerminalUnpackagedSettingsPath
        })
    $entries = @()
    $index = 0
    foreach ($target in $targets) {
        $existed = Test-Path -LiteralPath $target -PathType Leaf
        $expectedJson = & $script:SetupLibraryModule {
            param($SettingsPath, $FragmentPath)
            Merge-WindowsTerminalFragmentFile -SettingsPath $SettingsPath -FragmentPath $FragmentPath
        } $target $fragment
        $expectedSha = (& $script:SetupLibraryModule {
                param($Content)
                Get-WindowsTerminalContentSha256 -Content $Content
            } $expectedJson).ToLowerInvariant()
        $backupName = "wt-$index.before"
        if ($existed) {
            Copy-Item -LiteralPath $target -Destination (Join-Path $Recovery $backupName)
        }
        $entries += [pscustomobject]@{
            Path = $target
            Existed = $existed
            BeforeSha = Get-FileSha256OrEmpty -Path $target
            ExpectedSha = $expectedSha
            Backup = $backupName
        }
        $index++
    }
    $entries | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $Recovery 'windows-terminal.json') -Encoding utf8
}

function Get-CommandProviderInventory {
    $providers = foreach ($name in 'git', 'chezmoi', 'nvim', 'pwsh', 'wt', 'lazygit') {
        $command = Get-Command $name -ErrorAction SilentlyContinue | Select-Object -First 1
        [pscustomobject]@{
            Name = $name
            Source = if ($command) { [string]$command.Source } else { 'absent' }
        }
    }
    return @($providers)
}

function Get-ValidatedProviderInventory {
    param([Parameter(Mandatory)] [string]$Path)
    Assert-RegularFile -Path $Path
    $providers = @([IO.File]::ReadAllText($Path) | ConvertFrom-Json -ErrorAction Stop)
    $expectedNames = @('git', 'chezmoi', 'nvim', 'pwsh', 'wt', 'lazygit')
    if ($providers.Count -ne $expectedNames.Count) {
        throw 'command-provider recovery inventory is incomplete'
    }
    for ($index = 0; $index -lt $expectedNames.Count; $index++) {
        $provider = $providers[$index]
        $properties = @($provider.PSObject.Properties.Name | Sort-Object)
        if (($properties -join ',') -ne 'Name,Source' -or
            [string]$provider.Name -ne $expectedNames[$index] -or
            [string]::IsNullOrWhiteSpace([string]$provider.Source)) {
            throw "command-provider recovery inventory entry $index is malformed"
        }
    }
    return $providers
}

function Assert-ProviderBoundaryRestored {
    param([Parameter(Mandatory)] [object[]]$Expected)
    $current = @(Get-CommandProviderInventory)
    for ($index = 0; $index -lt $Expected.Count; $index++) {
        if ([string]$current[$index].Name -ne [string]$Expected[$index].Name -or
            [string]$current[$index].Source -ine [string]$Expected[$index].Source) {
            throw "command-provider boundary differs after rollback: $($Expected[$index].Name)"
        }
    }
}

function Initialize-UpgradeRecovery {
    param([Parameter(Mandatory)] $Preflight)
    $stateRoot = Join-Path $Preflight.Identity.LocalApplicationData 'dotfiles\migrations'
    $recovery = Initialize-PrivateRecoveryDirectory -Parent $stateRoot
    try {
        Write-PrivateText -Path (Join-Path $recovery 'old-checkout') -Value $Preflight.OldCheckout
        Write-PrivateText -Path (Join-Path $recovery 'new-checkout') -Value $Preflight.NewCheckout
        Write-PrivateText -Path (Join-Path $recovery 'target-profile') -Value $Preflight.Identity.UserProfile
        Write-PrivateText -Path (Join-Path $recovery 'known-folder-state-root') -Value $Preflight.KnownFolderStateRoot
        Write-PrivateText -Path (Join-Path $recovery 'new-commit') -Value $Preflight.NewCommit
        Write-PrivateText -Path (Join-Path $recovery 'new-tag-object') -Value $Preflight.NewTagObject
        $frozen = Save-FrozenReleaseState -Recovery $recovery -Preflight $Preflight
        Initialize-SetupLibrary -NewCheckout $frozen.NewSource
        if ([IO.Path]::GetFullPath($script:WindowsIdentity.UserProfile) -ine
            [IO.Path]::GetFullPath($Preflight.Identity.UserProfile)) {
            throw 'frozen v0.2.0 release resolves a different Windows target identity'
        }
        Copy-Item -LiteralPath (Join-Path $frozen.NewSource 'scripts/upgrade-v0.1.0.ps1') `
            -Destination (Join-Path $recovery 'upgrade-v0.1.0.ps1')
        Save-WindowsTerminalRecovery -Recovery $recovery
        $providers = @(Get-CommandProviderInventory)
        $providers | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $recovery 'providers.before.json') -Encoding utf8
        Save-RecoveryStage -Recovery $recovery -Stage prepared
        @(
            'Private recovery material for v0.1.0 to v0.2.0.',
            "Old checkout: $($Preflight.OldCheckout)",
            "New checkout: $($Preflight.NewCheckout)",
            "Frozen rollback source: $($frozen.OldSource)",
            "Frozen apply source: $($frozen.NewSource)",
            "Rollback: pwsh -NoProfile -File `"$(Join-Path $recovery 'upgrade-v0.1.0.ps1')`" -Rollback `"$recovery`"",
            "Accept: pwsh -NoProfile -File `"$(Join-Path $recovery 'upgrade-v0.1.0.ps1')`" -Accept `"$recovery`""
        ) | Set-Content -LiteralPath (Join-Path $recovery 'RECOVERY.txt') -Encoding utf8
        return $recovery
    } catch {
        Remove-Item -LiteralPath $recovery -Recurse -Force -ErrorAction SilentlyContinue
        throw
    }
}

function Invoke-PowerShellScript {
    param(
        [Parameter(Mandatory)] [string]$Path,
        [Parameter(Mandatory)] [string[]]$Arguments
    )
    $pwsh = (Get-Command pwsh -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    $processArguments = @(
        '-NoLogo', '-NoProfile', '-File', $Path
    ) + $Arguments
    return Invoke-NativeCapture -FilePath $pwsh -Arguments $processArguments -AllowFailure -PassThrough
}

function Publish-ExactContent {
    param(
        [Parameter(Mandatory)] [byte[]]$Bytes,
        [Parameter(Mandatory)] [string]$Target
    )
    $parent = Split-Path -Parent $Target
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    $temporary = Join-Path $parent ('.dotfiles-upgrade-restore.' + [guid]::NewGuid().ToString('N'))
    [IO.File]::WriteAllBytes($temporary, $Bytes)
    try {
        if (Test-Path -LiteralPath $Target -PathType Leaf) {
            [IO.File]::Move($temporary, $Target, $true)
        } else {
            [IO.File]::Move($temporary, $Target)
        }
    } finally {
        Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
    }
}

function Get-ValidatedWindowsTerminalRecovery {
    param(
        [Parameter(Mandatory)] [string]$Recovery,
        [Parameter(Mandatory)] [string[]]$ExpectedPaths
    )
    $manifestPath = Join-Path $Recovery 'windows-terminal.json'
    Assert-RegularFile -Path $manifestPath
    $entries = @([IO.File]::ReadAllText($manifestPath) | ConvertFrom-Json -ErrorAction Stop)
    if ($entries.Count -ne 2 -or $ExpectedPaths.Count -ne 2) {
        throw 'Windows Terminal recovery must contain exactly two targets'
    }
    $validated = @()
    for ($index = 0; $index -lt 2; $index++) {
        $entry = $entries[$index]
        $propertyNames = @($entry.PSObject.Properties.Name | Sort-Object)
        if (($propertyNames -join ',') -ne 'Backup,BeforeSha,Existed,ExpectedSha,Path') {
            throw "Windows Terminal recovery entry $index has an unexpected schema"
        }
        if ($entry.Existed -isnot [bool] -or [string]$entry.Path -ine $ExpectedPaths[$index]) {
            throw "Windows Terminal recovery entry $index has an invalid identity"
        }
        $beforeSha = [string]$entry.BeforeSha
        $expectedSha = [string]$entry.ExpectedSha
        $backupName = [string]$entry.Backup
        if ($expectedSha -notmatch '^[0-9a-f]{64}$' -or $backupName -ne "wt-$index.before") {
            throw "Windows Terminal recovery entry $index has invalid hashes or backup identity"
        }
        $backupBytes = [byte[]]@()
        if ($entry.Existed) {
            if ($beforeSha -notmatch '^[0-9a-f]{64}$') {
                throw "Windows Terminal recovery entry $index is missing the original hash"
            }
            $backupPath = Join-Path $Recovery $backupName
            Assert-RegularFile -Path $backupPath
            $backupBytes = [IO.File]::ReadAllBytes($backupPath)
            if ((Get-BytesSha256 -Bytes $backupBytes) -ne $beforeSha) {
                throw "Windows Terminal recovery backup $index does not match its manifest"
            }
        } elseif ($beforeSha -ne '') {
            throw "Windows Terminal recovery entry $index records bytes for an absent target"
        }
        $currentSha = Get-FileSha256OrEmpty -Path ([string]$entry.Path)
        $allowedCurrent = if ($entry.Existed) {
            $currentSha -eq $beforeSha -or $currentSha -eq $expectedSha
        } else {
            $currentSha -eq '' -or $currentSha -eq $expectedSha
        }
        if (-not $allowedCurrent) {
            throw "Windows Terminal settings changed outside this migration: $($entry.Path)"
        }
        $validated += [pscustomobject]@{
            Path = [string]$entry.Path
            Existed = [bool]$entry.Existed
            BeforeSha = $beforeSha
            ExpectedSha = $expectedSha
            BackupBytes = $backupBytes
        }
    }
    return $validated
}

function Restore-WindowsTerminalState {
    param([Parameter(Mandatory)] [object[]]$Entries)
    foreach ($entry in $entries) {
        $currentSha = Get-FileSha256OrEmpty -Path $entry.Path
        if ($entry.Existed) {
            if (-not $currentSha -or ($currentSha -ne $entry.BeforeSha -and $currentSha -ne $entry.ExpectedSha)) {
                throw "Windows Terminal settings changed concurrently; refusing recovery: $($entry.Path)"
            }
        } elseif ($currentSha) {
            if ($currentSha -ne $entry.ExpectedSha) {
                throw "new Windows Terminal settings changed concurrently; refusing removal: $($entry.Path)"
            }
        }
    }
    foreach ($entry in $entries) {
        $currentSha = Get-FileSha256OrEmpty -Path $entry.Path
        if ($entry.Existed) {
            Publish-ExactContent -Bytes $entry.BackupBytes -Target $entry.Path
        } elseif ($currentSha) {
            Remove-Item -LiteralPath $entry.Path -Force
        }
    }
}

function Assert-KnownFolderStateBoundary {
    param([Parameter(Mandatory)] [string]$StateRoot)
    if (-not (Test-Path -LiteralPath $StateRoot)) { return }
    $root = Get-Item -LiteralPath $StateRoot -Force -ErrorAction Stop
    if (-not $root.PSIsContainer -or ($root.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "known-folder state root is unsafe: $StateRoot"
    }
    $children = @(Get-ChildItem -LiteralPath $StateRoot -Force -Recurse)
    foreach ($child in $children) {
        $relative = [IO.Path]::GetRelativePath($StateRoot, $child.FullName).Replace('\', '/')
        if ($child.PSIsContainer -or ($child.Attributes -band [IO.FileAttributes]::ReparsePoint) -or
            $relative -notin @('localappdata.boltdb', 'documents.boltdb')) {
            throw "unexpected current-generation known-folder state blocks recovery: $($child.FullName)"
        }
    }
}

function Restore-KnownFolderStateBoundary {
    param([Parameter(Mandatory)] [string]$StateRoot)
    Assert-KnownFolderStateBoundary -StateRoot $StateRoot
    if (-not (Test-Path -LiteralPath $StateRoot)) { return }
    Remove-Item -LiteralPath $StateRoot -Recurse -Force
}

function Get-RecoveryState {
    param(
        [Parameter(Mandatory)] [string]$RecoveryPath,
        [switch]$RequireCheckouts
    )
    if ($env:OS -ne 'Windows_NT') {
        throw 'native Windows is required for recovery'
    }
    $recovery = Get-CanonicalDirectory -Path $RecoveryPath
    Assert-PrivateRecoveryAcl -Path $recovery
    foreach ($file in 'old-checkout', 'new-checkout', 'target-profile', 'known-folder-state-root', 'new-commit', 'new-tag-object', 'stage', 'windows-terminal.json', 'providers.before.json', 'upgrade-v0.1.0.ps1', 'RECOVERY.txt', 'frozen-releases.json', 'old-release.tar', 'new-release.tar', 'old-release.tree', 'new-release.tree') {
        $path = Join-Path $recovery $file
        Assert-RegularFile -Path $path
    }
    $targetProfile = Read-RecoveryScalar -Path (Join-Path $recovery 'target-profile')
    $currentProfile = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
    if ([IO.Path]::GetFullPath($targetProfile) -ine [IO.Path]::GetFullPath($currentProfile)) {
        throw "recovery belongs to $targetProfile, not $currentProfile"
    }
    $oldCheckout = Read-RecoveryScalar -Path (Join-Path $recovery 'old-checkout')
    $newCheckout = Read-RecoveryScalar -Path (Join-Path $recovery 'new-checkout')
    if (-not [IO.Path]::IsPathFullyQualified($oldCheckout) -or
        -not [IO.Path]::IsPathFullyQualified($newCheckout) -or $oldCheckout -ieq $newCheckout) {
        throw 'recovery checkout identities are malformed'
    }
    $newCommit = Read-RecoveryScalar -Path (Join-Path $recovery 'new-commit')
    $newTagObject = Read-RecoveryScalar -Path (Join-Path $recovery 'new-tag-object')
    $stage = Read-RecoveryScalar -Path (Join-Path $recovery 'stage')
    if ($newCommit -notmatch '^[0-9a-f]{40}$' -or $newTagObject -notmatch '^[0-9a-f]{40}$') {
        throw 'recovery release identity is malformed'
    }
    if ($stage -notin @('prepared', 'applying', 'applied', 'rolling-back', 'rolled-back', 'recovery-required', 'accepted')) {
        throw "recovery stage is invalid: $stage"
    }
    $frozen = Get-ValidatedFrozenReleaseState -Recovery $recovery
    if ($frozen.NewCommit -ne $newCommit -or $frozen.NewTagObject -ne $newTagObject) {
        throw 'frozen release identity differs from the recovery manifest'
    }
    if ($RequireCheckouts) {
        $oldCheckout = Get-CanonicalDirectory -Path $oldCheckout
        $newCheckout = Get-CanonicalDirectory -Path $newCheckout
        $oldHeadResult = Invoke-GitCapture -Checkout $oldCheckout -Arguments @('rev-parse', 'HEAD^{commit}')
        $oldTagResult = Invoke-GitCapture -Checkout $oldCheckout -Arguments @('rev-parse', "refs/tags/$($script:OldTag)")
        $newHeadResult = Invoke-GitCapture -Checkout $newCheckout -Arguments @('rev-parse', 'HEAD^{commit}')
        $newTagResult = Invoke-GitCapture -Checkout $newCheckout -Arguments @('rev-parse', "refs/tags/$($script:NewTag)")
        if ($oldHeadResult.Stdout.Count -ne 1 -or $oldHeadResult.Stdout[0] -ne $script:OldCommit -or
            $oldTagResult.Stdout.Count -ne 1 -or $oldTagResult.Stdout[0] -ne $script:OldTagObject) {
            throw 'retained v0.1.0 checkout or tag moved; refusing acceptance'
        }
        if ($newHeadResult.Stdout.Count -ne 1 -or $newHeadResult.Stdout[0] -ne $newCommit -or
            $newTagResult.Stdout.Count -ne 1 -or $newTagResult.Stdout[0] -ne $newTagObject) {
            throw 'retained v0.2.0 checkout or tag moved; refusing acceptance'
        }
        foreach ($checkout in $oldCheckout, $newCheckout) {
            $status = Invoke-GitCapture -Checkout $checkout -Arguments @('status', '--porcelain=v1', '--untracked-files=all')
            if ($status.Stdout.Count -gt 0) {
                throw "retained release checkout is dirty; refusing acceptance: $checkout"
            }
        }
    }
    if ((Get-FileSha256OrEmpty -Path (Join-Path $recovery 'upgrade-v0.1.0.ps1')) -ne
        (Get-FileSha256OrEmpty -Path (Join-Path $frozen.NewSource 'scripts/upgrade-v0.1.0.ps1'))) {
        throw 'recovery script differs from the exact v0.2.0 release'
    }
    $providersBefore = @(Get-ValidatedProviderInventory -Path (Join-Path $recovery 'providers.before.json'))
    Initialize-SetupLibrary -NewCheckout $frozen.NewSource
    if ([IO.Path]::GetFullPath($script:WindowsIdentity.UserProfile) -ine [IO.Path]::GetFullPath($targetProfile)) {
        throw 'current Windows known-folder identity does not match the recovery manifest'
    }
    $knownFolderStateRoot = Read-RecoveryScalar -Path (Join-Path $recovery 'known-folder-state-root')
    $expectedStateRoot = Join-Path $script:WindowsIdentity.LocalApplicationData 'dotfiles\chezmoi-state'
    if ([IO.Path]::GetFullPath($knownFolderStateRoot) -ine [IO.Path]::GetFullPath($expectedStateRoot)) {
        throw 'known-folder state root does not match the recovery identity'
    }
    Assert-KnownFolderStateBoundary -StateRoot $knownFolderStateRoot
    $expectedTerminalPaths = @(& $script:SetupLibraryModule {
            Get-WindowsTerminalSettingsPath
            Get-WindowsTerminalUnpackagedSettingsPath
        })
    $terminalRecovery = @(Get-ValidatedWindowsTerminalRecovery -Recovery $recovery -ExpectedPaths $expectedTerminalPaths)
    return [pscustomobject]@{
        Recovery = $recovery
        OldCheckout = $oldCheckout
        NewCheckout = $newCheckout
        OldSource = $frozen.OldSource
        NewSource = $frozen.NewSource
        NewCommit = $newCommit
        Stage = $stage
        TerminalRecovery = $terminalRecovery
        KnownFolderStateRoot = $knownFolderStateRoot
        ProvidersBefore = $providersBefore
    }
}

function Invoke-UpgradeRollback {
    param([Parameter(Mandatory)] [string]$RecoveryPath)
    $state = Get-RecoveryState -RecoveryPath $RecoveryPath
    if ($state.Stage -eq 'accepted') {
        throw 'this migration was explicitly accepted; automatic rollback authority has ended'
    }
    $old = $state.OldSource
    $new = $state.NewSource
    Save-RecoveryStage -Recovery $state.Recovery -Stage rolling-back
    $uninstall = Invoke-PowerShellScript -Path (Join-Path $new 'uninstall.ps1') -Arguments @(
        '-All', '-NoRestoreBackups', '-KeepExternals'
    )
    if ($uninstall.ExitCode -ne 0) {
        Save-RecoveryStage -Recovery $state.Recovery -Stage recovery-required
        throw "current config removal failed; retry from $($state.Recovery)"
    }
    Restore-KnownFolderStateBoundary -StateRoot $state.KnownFolderStateRoot
    $oldSetup = Invoke-PowerShellScript -Path (Join-Path $old 'setup.ps1') -Arguments @(
        '-All', '-SkipDeps', '-SkipNvim', '-SkipAgents', '-SkipWindowsTerminalMerge'
    )
    if ($oldSetup.ExitCode -ne 0) {
        Save-RecoveryStage -Recovery $state.Recovery -Stage recovery-required
        throw "v0.1.0 config restoration failed; retry from $($state.Recovery)"
    }
    Restore-WindowsTerminalState -Entries $state.TerminalRecovery
    if (-not (Test-OldConfig -OldCheckout $old -UserProfile $script:WindowsIdentity.UserProfile)) {
        Save-RecoveryStage -Recovery $state.Recovery -Stage recovery-required
        throw 'v0.1.0 config verification failed after rollback'
    }
    try {
        Assert-ProviderBoundaryRestored -Expected $state.ProvidersBefore
    } catch {
        Save-RecoveryStage -Recovery $state.Recovery -Stage recovery-required
        throw
    }
    Save-RecoveryStage -Recovery $state.Recovery -Stage rolled-back
    Write-Information 'v0.1.0 config and exact Windows Terminal bytes were restored.' -InformationAction Continue
    Write-Information "Recovery evidence retained at: $($state.Recovery)" -InformationAction Continue
}

function Test-NewConfig {
    param([Parameter(Mandatory)] [string]$NewCheckout)
    Initialize-SetupLibrary -NewCheckout $NewCheckout
    $main = Invoke-ChezmoiChecked -Arguments @(
        '--source', (Join-Path $NewCheckout 'home'),
        '--destination', $script:WindowsIdentity.UserProfile,
        'verify', '--include', 'files,symlinks'
    )
    if ($main.ExitCode -ne 0) { return $false }
    & $script:SetupLibraryModule {
        param($Identity)
        Assert-WindowsKnownFolderConsumption -Identity $Identity
    } $script:WindowsIdentity
    return $true
}

function Invoke-UpgradeAccept {
    param([Parameter(Mandatory)] [string]$RecoveryPath)
    $state = Get-RecoveryState -RecoveryPath $RecoveryPath -RequireCheckouts
    if ($state.Stage -ne 'applied') {
        throw "only an applied migration can be accepted; current stage is $($state.Stage)"
    }
    $new = Get-CanonicalDirectory -Path $state.NewCheckout
    $head = [string](Invoke-GitCapture -Checkout $new -Arguments @('rev-parse', 'HEAD^{commit}')).Stdout[0]
    if ($head -ne $state.NewCommit) { throw 'v0.2.0 checkout moved after migration' }
    if (-not (Test-NewConfig -NewCheckout $state.NewSource)) {
        throw 'v0.2.0 config verification failed; retain both checkouts and recovery material'
    }
    Save-RecoveryStage -Recovery $state.Recovery -Stage accepted
    Write-Information 'Migration core accepted. Keep v0.1.0 until full v0.2.0 setup finalizes retained conventional targets.' -InformationAction Continue
    Write-Information 'Run full v0.2.0 setup now; archive v0.1.0 only after it succeeds.' -InformationAction Continue
}

function Invoke-UpgradeApply {
    param([Parameter(Mandatory)] [string]$OldPath)
    $preflight = Invoke-UpgradePreflight -OldPath $OldPath -NewPath $script:DefaultNewCheckout
    $recovery = Initialize-UpgradeRecovery -Preflight $preflight
    $frozen = Get-ValidatedFrozenReleaseState -Recovery $recovery
    Write-Information "Recovery directory: $recovery" -InformationAction Continue
    $transactionActive = $false
    $mutationBegan = $false
    $setupExitCode = 1
    $applyFailure = $null
    try {
        Save-RecoveryStage -Recovery $recovery -Stage applying
        $mutationBegan = $true
        $transactionActive = $true
        $setup = Invoke-PowerShellScript -Path (Join-Path $frozen.NewSource 'setup.ps1') -Arguments @(
            '-All', '-SkipDeps', '-SkipNvim', '-SkipAgents', '-SkipConfigScripts',
            '-SkipLegacyKnownFolderMigration'
        )
        $setupExitCode = $setup.ExitCode
        if ($setupExitCode -ne 0) {
            throw "v0.2.0 setup exited $setupExitCode"
        }
        if (-not (Test-NewConfig -NewCheckout $frozen.NewSource)) {
            throw 'v0.2.0 config verification failed'
        }
        Save-RecoveryStage -Recovery $recovery -Stage applied
        $transactionActive = $false
    } catch {
        $applyFailure = $_
    } finally {
        if ($transactionActive) {
            Write-Warning 'Upgrade failed after mutation began; restoring v0.1.0.'
            try {
                Invoke-UpgradeRollback -RecoveryPath $recovery
            } catch {
                Save-RecoveryStage -Recovery $recovery -Stage recovery-required
                throw "RECOVERY REQUIRED: pwsh -NoProfile -File `"$(Join-Path $recovery 'upgrade-v0.1.0.ps1')`" -Rollback `"$recovery`"`n$($_.Exception.Message)"
            }
        }
    }
    if ($applyFailure) {
        if (-not $mutationBegan) {
            Remove-Item -LiteralPath $recovery -Recurse -Force -ErrorAction SilentlyContinue
        }
        if ($setupExitCode -ne 0) {
            throw "v0.2.0 setup exited $setupExitCode; v0.1.0 was restored"
        }
        throw "$($applyFailure.Exception.Message); v0.1.0 was restored"
    }
    Write-Information 'v0.2.0 config migration applied and verified without package mutation.' -InformationAction Continue
    Write-Information 'Retain both checkouts, then accept with:' -InformationAction Continue
    Write-Information "  pwsh -NoProfile -File `"$(Join-Path $recovery 'upgrade-v0.1.0.ps1')`" -Accept `"$recovery`"" -InformationAction Continue
}

if ($SourceOnly) { return }

switch ($PSCmdlet.ParameterSetName) {
    'Preflight' {
        $result = Invoke-UpgradePreflight -OldPath $OldCheckout -NewPath $script:DefaultNewCheckout
        Write-Information 'v0.1.0 to v0.2.0 preflight passed; no live state changed.' -InformationAction Continue
        Write-Information "old=$($result.OldCheckout)" -InformationAction Continue
        Write-Information "new=$($result.NewCheckout)" -InformationAction Continue
    }
    'Apply' {
        Invoke-UpgradeApply -OldPath $OldCheckout
    }
    'Rollback' {
        Invoke-UpgradeRollback -RecoveryPath $Rollback
    }
    'Accept' {
        Invoke-UpgradeAccept -RecoveryPath $Accept
    }
}
