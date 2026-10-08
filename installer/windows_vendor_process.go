package installer

import (
	"encoding/base64"
	"encoding/binary"
	"unicode/utf16"
)

func windowsVendorArguments(script string) []string {
	units := utf16.Encode([]rune(script))
	bytes := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(bytes[i*2:], unit)
	}
	return []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(bytes)}
}

// Windows Authenticode verifies timestamped signatures at signing time. The
// separate chain walk below checks Microsoft ownership; it must not reject an
// otherwise valid timestamped signature solely because its signer expired.
// Payload SHA-256 is checked again while a deny-write/delete handle is held.
const windowsVendorTrustPrelude = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
if ([Environment]::OSVersion.Platform -ne 'Win32NT' -or -not [Environment]::Is64BitProcess) { throw 'Native Windows amd64 is required' }
function Assert-MicrosoftSignature([string]$Path) {
	$signature = Get-AuthenticodeSignature -LiteralPath $Path
	if ($signature.Status -ne 'Valid' -or -not ($signature.SignerCertificate -is [Security.Cryptography.X509Certificates.X509Certificate2])) { throw 'The file lacks a valid Microsoft Authenticode signature' }
	$chain = [Security.Cryptography.X509Certificates.X509Chain]::new()
	try {
		$chain.ChainPolicy.RevocationMode = [Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck
		$chain.ChainPolicy.RevocationFlag = [Security.Cryptography.X509Certificates.X509RevocationFlag]::ExcludeRoot
		$chain.ChainPolicy.VerificationFlags = [Security.Cryptography.X509Certificates.X509VerificationFlags]::NoFlag
		if (-not $chain.Build($signature.SignerCertificate)) {
			$errors = @($chain.ChainStatus | Where-Object { $_.Status -ne [Security.Cryptography.X509Certificates.X509ChainStatusFlags]::NotTimeValid })
			if ($errors.Count -or -not ($signature.TimeStamperCertificate -is [Security.Cryptography.X509Certificates.X509Certificate2])) { throw ('Microsoft certificate chain validation failed: ' + ($chain.ChainStatus.Status -join ', ')) }
			# WinVerifyTrust already accepted the signed timestamp. Only the
			# independent present-day lifetime check is inapplicable here.
			$chain.ChainPolicy.VerificationFlags = [Security.Cryptography.X509Certificates.X509VerificationFlags]::IgnoreNotTimeValid
			if (-not $chain.Build($signature.SignerCertificate)) { throw 'Timestamped Microsoft certificate chain validation failed' }
		}
		$certificates = @($chain.ChainElements | ForEach-Object { $_.Certificate })
		$signerText = $signature.SignerCertificate.Subject + ' ' + $signature.SignerCertificate.Issuer
		$rootText = $certificates[-1].Subject + ' ' + $certificates[-1].Issuer
		$chainText = ($certificates | ForEach-Object { $_.Subject + ' ' + $_.Issuer }) -join ' '
		if ($signerText -notmatch '(?i)(CN|O)=Microsoft Corporation|Microsoft Corporation' -or $rootText -notmatch '(?i)Microsoft (Root|Corporation|Identity Verification)' -or $chainText -notmatch '(?i)Microsoft (Code Signing PCA|Corporation|Root|Windows|Identity Verification)') { throw 'The certificate signer/chain/root is not Microsoft-owned' }
	} finally { $chain.Dispose() }
}
`

const windowsRuntimeVersionFunction = `
function ConvertTo-RuntimeVersion([string]$Value) {
	$Value = $Value.TrimStart('v')
	if ($Value -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:\.[0-9]+)?$') { throw 'Invalid VC++ runtime version' }
	$version = [version]$Value
	$revision = [Math]::Max(0, $version.Revision)
	return '{0}.{1}.{2}.{3}' -f $version.Major, $version.Minor, $version.Build, $revision
}
`

const windowsVendorPrelude = windowsVendorTrustPrelude + windowsRuntimeVersionFunction + `
function Get-RuntimeRegistration {
	$versions = @()
	$bundles = @{}
	$packages = @{}
	$dependents = @()
	foreach ($view in @([Microsoft.Win32.RegistryView]::Registry64, [Microsoft.Win32.RegistryView]::Registry32)) {
		$base = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine, $view)
		try {
			$key = $base.OpenSubKey('SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\x64')
			if ($key) {
				try { if ($key.GetValue('Installed', 0) -eq 1) { $versions += ConvertTo-RuntimeVersion ([string]$key.GetValue('Version', '')) } }
				finally { $key.Dispose() }
			}
			$uninstall = $base.OpenSubKey('SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall')
			if ($uninstall) {
				try {
					$names = @($uninstall.GetSubKeyNames())
					if ($names.Count -gt 10000) { throw 'Windows uninstall registry exceeds its inspection bound' }
					foreach ($name in $names) {
						$key = $uninstall.OpenSubKey($name)
						try {
							if (@($key.GetValue('BundleUpgradeCode')) -contains '{C146EF48-4D31-3C3D-A2C5-1E91AF8A0A9B}') {
								if ($key.GetValue('Publisher') -ne 'Microsoft Corporation') { throw 'Runtime bundle has an unexpected publisher' }
								$bundles[$name.ToLowerInvariant()] = ConvertTo-RuntimeVersion ([string]$key.GetValue('DisplayVersion', ''))
							}
						} finally { if ($key) { $key.Dispose() } }
					}
				} finally { $uninstall.Dispose() }
			}
			foreach ($part in @('Minimum', 'Additional')) {
				$key = $base.OpenSubKey("SOFTWARE\Classes\Installer\Dependencies\Microsoft.VS.VC_Runtime${part}VSU_amd64,v14")
				if ($key) {
					try {
						$id = ([string]$key.GetValue('')).ToLowerInvariant()
						$name = $part.ToLowerInvariant()
						if ($packages.ContainsKey($name) -and $packages[$name] -ne $id) { throw 'Runtime package registration differs across registry views' }
						$packages[$name] = $id
						$dependentsKey = $key.OpenSubKey('Dependents')
						if ($dependentsKey) {
							try { $dependents += @($dependentsKey.GetSubKeyNames() | ForEach-Object { $_.ToLowerInvariant() }) }
							finally { $dependentsKey.Dispose() }
						}
					} finally { $key.Dispose() }
				}
			}
		} finally { $base.Dispose() }
	}
	$versions = @($versions | Select-Object -Unique)
	if ($versions.Count -gt 1 -or $bundles.Count -gt 1) { throw 'Ambiguous installed VC++ runtime registrations' }
	$version = if ($versions.Count) { $versions[0] } else { '' }
	$bundle = if ($bundles.Count) { @($bundles.Keys)[0] } else { '' }
	if ($bundle -and $version -and $bundles[$bundle] -ne $version) { throw ('Runtime bundle and servicing versions disagree: ' + $bundles[$bundle] + ' versus ' + $version) }
	[pscustomobject]@{ version=$version; bundle_id=$bundle; packages=$packages; consumers=@($dependents | Where-Object { $_ -ne $bundle } | Sort-Object -Unique) }
}
`

const windowsVendorObserveScript = windowsVendorPrelude + `
$registration = Get-RuntimeRegistration
$files = @{}
$healthy = $false
$issue = ''
$system = [Environment]::SystemDirectory
$present = $registration.version -ne '' -or $registration.bundle_id -ne '' -or $registration.packages.Count -ne 0
foreach ($name in @('vcruntime140.dll', 'vcruntime140_1.dll', 'msvcp140.dll')) {
	$path = Join-Path $system $name
	if (Test-Path -LiteralPath $path -PathType Leaf) {
		$present = $true
		if (-not $registration.version) { $registration.version = [Diagnostics.FileVersionInfo]::GetVersionInfo($path).FileVersion }
	}
}
try {
	Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class DotfilesRuntimeProbe { [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern IntPtr LoadLibraryEx(string name, IntPtr file, uint flags); [DllImport("kernel32.dll")] public static extern bool FreeLibrary(IntPtr module); }'
	foreach ($name in @('vcruntime140.dll', 'vcruntime140_1.dll', 'msvcp140.dll')) {
		$path = Join-Path $system $name
		if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw ('Missing runtime library: ' + $name) }
		Assert-MicrosoftSignature $path
		$files[$name] = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
		$module = [DotfilesRuntimeProbe]::LoadLibraryEx($path, [IntPtr]::Zero, 0x00000900)
		if ($module -eq [IntPtr]::Zero) { throw ('Runtime library could not load: ' + $name) }
		if (-not [DotfilesRuntimeProbe]::FreeLibrary($module)) { throw ('Runtime library could not unload: ' + $name) }
	}
	$healthy = $registration.version -ne ''
} catch { $issue = $_.Exception.Message }
$boot = (Get-CimInstance -ClassName Win32_OperatingSystem).LastBootUpTime.ToUniversalTime().ToString('o')
@{present=$present;healthy=$healthy;version=$registration.version;bundle_id=$registration.bundle_id;packages=$registration.packages;files=$files;consumers=$registration.consumers;boot=$boot;health_issue=$issue} | ConvertTo-Json -Depth 4 -Compress
`

const windowsVendorInstallScript = windowsVendorPrelude + `
$inputData = [Console]::In.ReadToEnd() | ConvertFrom-Json
$file = [string]$inputData.File
$handle = [IO.File]::Open($file, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
try {
	if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -cne $inputData.SHA256) { throw 'Installer SHA-256 changed before execution' }
	Assert-MicrosoftSignature $file
	$registration = Get-RuntimeRegistration
	if ($inputData.Action -eq 'install') {
		if ($registration.version -or $registration.bundle_id -or $registration.packages.Count -ne 0) { throw 'A pre-existing runtime appeared before installation; preserve it and preview again' }
		foreach ($name in @('vcruntime140.dll', 'vcruntime140_1.dll', 'msvcp140.dll')) {
			if (Test-Path -LiteralPath (Join-Path ([Environment]::SystemDirectory) $name)) { throw 'A pre-existing runtime library appeared before installation; preserve it and preview again' }
		}
	} else {
		if ($registration.bundle_id -cne $inputData.Before.bundle_id -or $registration.version -cne $inputData.Before.version -or $registration.packages.Count -ne @($inputData.Before.packages.PSObject.Properties).Count) { throw 'Runtime registration changed before execution' }
		foreach ($name in $registration.packages.Keys) {
			if ($registration.packages[$name] -cne $inputData.Before.packages.$name) { throw 'Runtime package changed before execution' }
		}
		foreach ($entry in $inputData.Before.files.PSObject.Properties) {
			$path = Join-Path ([Environment]::SystemDirectory) $entry.Name
			if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -cne $entry.Value) { throw 'Runtime library changed before execution' }
		}
	}
	if ($inputData.Action -eq 'remove') {
		if ($registration.bundle_id -cne $inputData.BundleID -or $registration.version -cne $inputData.Version -or $registration.packages.Count -ne 2 -or $registration.consumers.Count -ne 0) { throw 'Exact runtime removal is blocked by changed registration or consumers' }
	}
	$action = switch ($inputData.Action) { 'install' {'/install'} 'update' {'/install'} 'repair' {'/repair'} 'remove' {'/uninstall'} default {throw 'Unknown vendor action'} }
	$arguments = @($action, '/quiet', '/norestart', '/log', ('"' + [string]$inputData.Log + '"'))
	$start = [Diagnostics.ProcessStartInfo]::new()
	$start.FileName = $file
	$start.Arguments = $arguments -join ' '
	$start.UseShellExecute = $true
	$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
	try { $administrator = ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) }
	finally { $identity.Dispose() }
	if (-not $administrator) { $start.Verb = 'runas' }
	try {
		$process = [Diagnostics.Process]::Start($start)
		try { $process.WaitForExit(); $code = [int]$process.ExitCode }
		finally { $process.Dispose() }
	} catch {
		# Start-Process discards Win32Exception.NativeErrorCode. The direct API
		# preserves UAC denial without parsing localized exception messages.
		$failure = $_.Exception
		while ($failure -and -not ($failure -is [ComponentModel.Win32Exception])) { $failure = $failure.InnerException }
		if (-not $failure -or $failure.NativeErrorCode -ne 1223) { throw }
		$code = 1223
	}
} finally { $handle.Dispose() }
exit $code
`
