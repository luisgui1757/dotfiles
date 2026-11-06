package installer

const windowsAppPrelude = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
if ([Environment]::OSVersion.Platform -ne 'Win32NT' -or -not [Environment]::Is64BitProcess) { throw 'Native Windows amd64 is required' }
$inputData = [Console]::In.ReadToEnd() | ConvertFrom-Json
function Assert-PublisherSignature([string]$Path) {
	$signature = Get-AuthenticodeSignature -LiteralPath $Path
	if ($signature.Status -ne 'Valid' -or -not ($signature.SignerCertificate -is [Security.Cryptography.X509Certificates.X509Certificate2])) { throw ('The file lacks a valid Authenticode signature: ' + $Path) }
	$name = $signature.SignerCertificate.GetNameInfo([Security.Cryptography.X509Certificates.X509NameType]::SimpleName, $false)
	if ($name -cne [string]$inputData.Publisher) { throw ('The file is not signed by ' + $inputData.Publisher + ': ' + $Path) }
}
function Get-AppRegistration {
	$records = @()
	$suffix = [Regex]::Escape('\' + [string]$inputData.Uninstaller) + '" /(currentuser|allusers)$'
	foreach ($hive in @('CurrentUser', 'LocalMachine')) {
		$views = if ($hive -eq 'CurrentUser') { @([Microsoft.Win32.RegistryView]::Default) } else { @([Microsoft.Win32.RegistryView]::Registry64, [Microsoft.Win32.RegistryView]::Registry32) }
		foreach ($view in $views) {
			$base = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::$hive, $view)
			try {
				foreach ($name in @([string]$inputData.ProductCode, '{' + $inputData.ProductCode + '}')) {
					$key = $base.OpenSubKey('SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\' + $name)
					if (-not $key) { continue }
					try {
						$command = [string]$key.GetValue('UninstallString', '')
						$location = ''
						if ($command -match ('^"(.+)' + $suffix)) { $location = $Matches[1] }
						$records += [pscustomobject]@{ scope = $(if ($hive -eq 'CurrentUser') { 'user' } else { 'machine' }); location = $location; version = [string]$key.GetValue('DisplayVersion', '') }
					} finally { $key.Dispose() }
				}
			} finally { $base.Dispose() }
		}
	}
	@($records | Sort-Object scope, location, version -Unique)
}
function Test-AppRunning([string]$Location) {
	$prefix = $Location.TrimEnd('\') + '\'
	@(Get-CimInstance -ClassName Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) }).Count -ne 0
}
`

const windowsAppObserveScript = windowsAppPrelude + `
$default = Join-Path ([string]$inputData.Programs) ([string]$inputData.Folder)
$records = @(Get-AppRegistration)
$location = $default
$scope = ''
$version = ''
$healthy = $false
$issue = ''
if ($records.Count -eq 1) { $location = $records[0].location; $scope = $records[0].scope; $version = $records[0].version }
$executable = if ($location) { Join-Path $location ([string]$inputData.Executable) } else { '' }
$present = $records.Count -ne 0 -or (Test-Path -LiteralPath (Join-Path $default ([string]$inputData.Executable)))
try {
	if ($records.Count -gt 1) { throw 'The application is registered more than once' }
	if ($records.Count -eq 0) { throw 'The application files have no uninstall registration' }
	if (-not $location) { throw 'The uninstall registration names an unexpected uninstaller' }
	Assert-PublisherSignature $executable
	Assert-PublisherSignature (Join-Path $location ([string]$inputData.Uninstaller))
	$healthy = $true
} catch { if ($present) { $issue = $_.Exception.Message } }
$running = $false
if ($present -and $location) { $running = Test-AppRunning $location }
@{present=$present;healthy=$healthy;version=$version;location=$location;scope=$scope;running=$running;health_issue=$issue} | ConvertTo-Json -Compress
`

const windowsAppChangeScript = windowsAppPrelude + `
$location = Join-Path ([string]$inputData.Programs) ([string]$inputData.Folder)
if (Test-AppRunning $location) { throw ('Quit ' + $inputData.Folder + ' before setup changes its installation') }
$records = @(Get-AppRegistration)
$start = [Diagnostics.ProcessStartInfo]::new()
$start.UseShellExecute = $false
if ($inputData.Action -eq 'remove') {
	if ($records.Count -ne 1 -or $records[0].scope -cne 'user' -or $records[0].location -ine $location) { throw 'The registration changed before removal' }
	$uninstaller = Join-Path $location ([string]$inputData.Uninstaller)
	Assert-PublisherSignature (Join-Path $location ([string]$inputData.Executable))
	Assert-PublisherSignature $uninstaller
	$start.FileName = $uninstaller
	$start.Arguments = '/currentuser /S'
} else {
	if ($inputData.Action -eq 'install' -and ($records.Count -ne 0 -or (Test-Path -LiteralPath (Join-Path $location ([string]$inputData.Executable))))) { throw 'An installation appeared before setup; preserve it and preview again' }
	if ($inputData.Action -eq 'repair' -and ($records.Count -ne 1 -or $records[0].scope -cne 'user' -or $records[0].location -ine $location)) { throw 'The registration changed before repair' }
	$file = [string]$inputData.File
	$handle = [IO.File]::Open($file, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
	try {
		if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -cne $inputData.SHA256) { throw 'Installer SHA-256 changed before execution' }
		Assert-PublisherSignature $file
	} finally { $handle.Dispose() }
	$start.FileName = $file
	$start.Arguments = '/S'
}
$process = [Diagnostics.Process]::Start($start)
try { $process.WaitForExit(); $code = [int]$process.ExitCode }
finally { $process.Dispose() }
if ($code -eq 0 -and $inputData.Action -eq 'remove') {
	$deadline = [DateTime]::UtcNow.AddMinutes(10)
	while (@(Get-AppRegistration).Count -ne 0 -or (Test-Path -LiteralPath $uninstaller)) {
		if ([DateTime]::UtcNow -gt $deadline) { throw 'The vendor uninstaller did not finish in 10 minutes' }
		Start-Sleep -Seconds 2
	}
}
exit $code
`
