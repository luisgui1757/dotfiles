# Build and launch the installer from this trusted checkout. No system installs.
$ErrorActionPreference = 'Stop'
$ForwardedArguments = @($args)
if ($args.Count -ne 0 -and ($args.Count -ne 1 -or $args[0] -cne 'machine')) {
    throw "Run without arguments for the menu; automation uses 'machine' with JSON on stdin"
}
if ($env:OS -ne 'Windows_NT' -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64' -or
    ($env:PROCESSOR_ARCHITEW6432 -and $env:PROCESSOR_ARCHITEW6432 -ne 'AMD64')) {
    throw 'This launcher requires native Windows amd64; macOS/Linux use installer-bootstrap.sh'
}
$Checkout = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).ProviderPath
$Pins = @(Get-Content -LiteralPath (Join-Path $Checkout 'installer/bootstrap-toolchain.tsv') |
    Where-Object { -not $_.StartsWith('#') } | ConvertFrom-Csv -Delimiter "`t")
$Seen = @{}
foreach ($Row in $Pins) {
    if ($Row.version -cnotmatch '^go[0-9]+\.[0-9]+\.[0-9]+$' -or
        $Row.platform -cnotmatch '^(darwin-arm64|linux-amd64|linux-arm64|windows-amd64)$' -or
        $Row.sha256 -cnotmatch '^[a-f0-9]{64}$' -or $Row.size -cnotmatch '^[1-9][0-9]*$' -or
        @($Row.PSObject.Properties).Count -ne 5) { throw 'Invalid toolchain pin metadata' }
    $Suffix = if ($Row.platform -eq 'windows-amd64') { 'zip' } else { 'tar.gz' }
    if ($Row.filename -cne "$($Row.version).$($Row.platform).$Suffix") {
        throw 'Toolchain filename does not match its version and platform'
    }
    if ($Seen.ContainsKey($Row.platform)) { throw 'Duplicate toolchain target' }
    $Seen[$Row.platform] = $true
}
$Pin = @($Pins | Where-Object platform -CEQ 'windows-amd64')
if ($Pin.Count -ne 1) { throw 'Toolchain target is missing' }
$Pin = $Pin[0]
$ModuleVersion = @(Get-Content -LiteralPath (Join-Path $Checkout 'installer/go.mod') |
    Where-Object { $_ -match '^go ' })
if ($ModuleVersion.Count -ne 1 -or $ModuleVersion[0].Trim() -cne "go $($Pin.version.Substring(2))") {
    throw 'Toolchain pin must match installer/go.mod'
}
$CacheBase = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
if (-not $CacheBase) { throw 'LocalApplicationData known folder is unavailable' }
$Cache = Join-Path $CacheBase 'dotfiles/bootstrap'
New-Item -ItemType Directory -Force -Path $Cache | Out-Null
if ((Get-Item -LiteralPath $Cache).Attributes -band [IO.FileAttributes]::ReparsePoint) {
    throw 'Bootstrap cache must not be redirected'
}
$Stage = Join-Path $Cache ('.prepare.' + [Guid]::NewGuid().ToString('N'))
$LockPath = Join-Path $Cache 'lock'
$Lock = $null
$PreviousLocation = Get-Location
# Keep phase diagnostics off stdout: machine mode returns only its JSON result.
function Invoke-BootstrapPhase {
    param([string]$Name, [scriptblock]$Action)
    $Timer = [Diagnostics.Stopwatch]::StartNew()
    [Console]::Error.WriteLine("Bootstrap ${Name}: started")
    try {
        & $Action
        [Console]::Error.WriteLine("Bootstrap ${Name}: completed in $($Timer.ElapsedMilliseconds) ms")
    } catch {
        [Console]::Error.WriteLine("Bootstrap ${Name}: failed after $($Timer.ElapsedMilliseconds) ms")
        throw
    }
}
function Get-SHA256([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}
function Assert-Archive {
    $Item = Get-Item -LiteralPath $Archive
    if (($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or
        $Item.Length -ne [long]$Pin.size -or (Get-SHA256 $Archive) -cne $Pin.sha256) {
        throw "Go archive checksum or size mismatch; remove $Archive and retry"
    }
}
function Invoke-Compiler {
    $Settings = @{
        GOENV = 'off'; GOWORK = 'off'; GOTOOLCHAIN = 'local'; CGO_ENABLED = '0'; GOFLAGS = ''
        GOEXPERIMENT = ''; GO111MODULE = 'on'; GOFIPS140 = 'off'
        GOCACHEPROG = ''
        GOROOT = (Join-Path $Toolchain 'go'); GOPATH = (Join-Path $Cache 'gopath')
        GOCACHE = (Join-Path $Cache 'go-build'); GOOS = 'windows'; GOARCH = 'amd64'; GOAMD64 = 'v1'
        USERPROFILE = (Join-Path $Cache 'compiler-home'); APPDATA = (Join-Path $Cache 'compiler-config')
        GOTMPDIR = $Stage
        GOPROXY = 'https://proxy.golang.org'; GOSUMDB = 'sum.golang.org'
        GOPRIVATE = ''; GONOPROXY = ''; GONOSUMDB = ''
    }
    $Saved = @{}
    try {
        foreach ($Name in $Settings.Keys) {
            $Saved[$Name] = [Environment]::GetEnvironmentVariable($Name)
            [Environment]::SetEnvironmentVariable($Name, $Settings[$Name])
        }
        & $Go @args
        if ($LASTEXITCODE -ne 0) { throw "Go compiler failed (exit $LASTEXITCODE)" }
    } finally {
        foreach ($Name in $Saved.Keys) { [Environment]::SetEnvironmentVariable($Name, $Saved[$Name]) }
    }
}
function Get-SourceIdentity {
    $Template = '{{if .Module}}{{if .Module.Main}}{{range .GoFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .SFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .HFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .SysoFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .EmbedFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{end}}{{end}}'
    $Inputs = @(Invoke-Compiler list -mod=readonly -deps -f $Template ./cmd/dotfiles)
    $Inputs += (Join-Path $Checkout 'installer/go.mod'), (Join-Path $Checkout 'installer/go.sum')
    $Prefix = [IO.Path]::GetFullPath((Join-Path $Checkout 'installer')) + [IO.Path]::DirectorySeparatorChar
    $Lines = @($Pin.version, $Pin.platform, $Pin.sha256, (Get-SHA256 (Join-Path $Checkout 'scripts/installer-bootstrap.ps1')))
    foreach ($Path in @($Inputs | Where-Object { $_ } | Sort-Object -Unique)) {
        $FullPath = [IO.Path]::GetFullPath($Path)
        $Item = Get-Item -LiteralPath $FullPath
        if (-not $FullPath.StartsWith($Prefix, [StringComparison]::OrdinalIgnoreCase) -or
            $Item.PSIsContainer -or ($Item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw 'Unexpected or redirected build input'
        }
        $Lines += $FullPath.Substring($Prefix.Length).Replace('\', '/') + "`t" + (Get-SHA256 $FullPath)
    }
    $Bytes = [Text.Encoding]::UTF8.GetBytes(($Lines -join "`n") + "`n")
    $Hasher = [Security.Cryptography.SHA256]::Create()
    try { ([BitConverter]::ToString($Hasher.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $Hasher.Dispose() }
}
try {
    try { $Lock = [IO.File]::Open($LockPath, 'OpenOrCreate', 'ReadWrite', 'None') }
    catch [IO.IOException] { throw 'Another bootstrap is running; retry after it finishes' }
    New-Item -ItemType Directory -Path $Stage | Out-Null
    $Archive = Join-Path $Cache $Pin.filename
    if (-not (Test-Path -LiteralPath $Archive)) {
        [Console]::Error.WriteLine("Downloading verified $($Pin.version) toolchain...")
        $Download = Join-Path $Stage 'archive.zip'
        $PreviousTLS = [Net.ServicePointManager]::SecurityProtocol
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-BootstrapPhase 'download' {
                # Windows PowerShell 5.1 progress can dominate transfer time.
                # Keep the explicit phase diagnostics and restore caller policy.
                $PreviousProgress = $ProgressPreference
                try {
                    $ProgressPreference = 'SilentlyContinue'
                    Invoke-WebRequest -UseBasicParsing -Uri "https://go.dev/dl/$($Pin.filename)" -OutFile $Download
                } finally { $ProgressPreference = $PreviousProgress }
            }
        } finally { [Net.ServicePointManager]::SecurityProtocol = $PreviousTLS }
        if ((Get-Item -LiteralPath $Download).Length -ne [long]$Pin.size -or
            (Get-SHA256 $Download) -cne $Pin.sha256) {
            throw 'Go archive checksum or size mismatch; nothing was extracted or executed'
        }
        Move-Item -LiteralPath $Download -Destination $Archive
    }
    Invoke-BootstrapPhase 'archive verification' { Assert-Archive }
    # Execute only bytes freshly extracted from the reverified cached archive.
    # The reported version of an extracted compiler is not an integrity check.
    $Toolchain = Join-Path $Stage 'toolchain'
    Invoke-BootstrapPhase 'toolchain extraction' {
        # Avoid per-entry PowerShell/progress overhead in the archive module.
        # This built-in API is available in the Windows PowerShell 5.1 runtime.
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [IO.Compression.ZipFile]::ExtractToDirectory($Archive, $Toolchain)
    }
    $Go = Join-Path $Toolchain 'go/bin/go.exe'
    if (-not (Test-Path -LiteralPath $Go)) { throw 'Archive has no Go executable' }
    $CompilerVersion = Invoke-BootstrapPhase 'compiler version' { Invoke-Compiler version }
    if ($CompilerVersion -cne "go version $($Pin.version) windows/amd64") { throw 'Cached compiler version mismatch' }
    Set-Location -LiteralPath (Join-Path $Checkout 'installer')
    Invoke-BootstrapPhase 'module verification' { [Console]::Error.WriteLine((Invoke-Compiler mod verify)) }
    $Identity = Invoke-BootstrapPhase 'source identity' { Get-SourceIdentity }
    $Binary = Join-Path $Cache "dotfiles-$Identity.exe"
    if (-not (Test-Path -LiteralPath $Binary)) {
        [Console]::Error.WriteLine('Building Dotfiles from the trusted checkout...')
        $Built = Join-Path $Stage 'dotfiles.exe'
        Invoke-BootstrapPhase 'build' { Invoke-Compiler build -mod=readonly -trimpath -buildvcs=false -pgo=off -o $Built ./cmd/dotfiles }
        $BuiltIdentity = Invoke-BootstrapPhase 'source revalidation' { Get-SourceIdentity }
        if ($BuiltIdentity -cne $Identity) { throw 'Checkout changed while building; retry' }
        [IO.File]::WriteAllText("$Binary.sha256", (Get-SHA256 $Built), [Text.Encoding]::ASCII)
        Move-Item -LiteralPath $Built -Destination $Binary
    }
    if (((Get-Item -LiteralPath $Binary).Attributes -band [IO.FileAttributes]::ReparsePoint) -or
        (Get-SHA256 $Binary) -cne [IO.File]::ReadAllText("$Binary.sha256").Trim()) {
        throw "Cached installer is damaged; remove $Binary and retry"
    }
} finally {
    Set-Location -LiteralPath $PreviousLocation.Path
    if (Test-Path -LiteralPath $Stage) { Remove-Item -LiteralPath $Stage -Recurse -Force }
    if ($null -ne $Lock) { $Lock.Dispose() }
}
$PreviousCheckout = $env:DOTFILES_CHECKOUT
try {
    $env:DOTFILES_CHECKOUT = $Checkout
    & $Binary @ForwardedArguments
    $Result = $LASTEXITCODE
} finally { $env:DOTFILES_CHECKOUT = $PreviousCheckout }
exit $Result
