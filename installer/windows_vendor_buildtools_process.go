package installer

// vswhere's documented -include packages supplies the instance's full package
// inventory. An unsupported or damaged locator fails closed; no registry guess
// can authorize changing a pre-existing Visual Studio installation.
const windowsBuildToolsPackagePrelude = `
function Get-BuildToolsPackageKey($Package) {
	ConvertTo-Json -InputObject @([string]$Package.id,[string]$Package.version,[string]$Package.chip,[string]$Package.language,[string]$Package.branch,[string]$Package.type,[bool]$Package.extension) -Compress
}
`

const windowsBuildToolsPrelude = windowsVendorTrustPrelude + windowsBuildToolsPackagePrelude + `
function Get-BuildToolsState([string]$InstallDirectory) {
	$locator = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
	$instances = @()
	if (Test-Path -LiteralPath $locator -PathType Leaf) {
		Assert-MicrosoftSignature $locator
		$json = & $locator -all -products '*' -format json -utf8 -include packages
		if ($LASTEXITCODE -ne 0) { throw 'Visual Studio inventory failed; its locator must support -include packages' }
		$registered = @((($json -join [Environment]::NewLine) | ConvertFrom-Json))
		if ($registered.Count -gt 128) { throw 'Visual Studio inventory exceeds its instance bound' }
		foreach ($item in $registered) {
			$packages = @()
			$bag = [Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
			if (@($item.packages).Count -gt 10000) { throw 'Visual Studio inventory exceeds its package bound' }
			foreach ($package in $item.packages) {
				foreach ($property in $package.PSObject.Properties) {
					if ($property.Name -cnotin @('id','version','chip','language','branch','type','extension')) { throw 'Unsupported Visual Studio package property' }
					if ($property.Name -ceq 'extension') {
						if ($property.Value -isnot [bool]) { throw 'Invalid Visual Studio package extension flag' }
					} elseif ($null -ne $property.Value -and $property.Value -isnot [string]) { throw 'Invalid Visual Studio package token' }
				}
				$key = Get-BuildToolsPackageKey $package
				if ($bag.ContainsKey($key)) { $bag[$key].count++ }
				else { $bag.Add($key,[pscustomobject][ordered]@{id=[string]$package.id;version=[string]$package.version;chip=[string]$package.chip;language=[string]$package.language;branch=[string]$package.branch;type=[string]$package.type;extension=[bool]$package.extension;count=1}) }
			}
			$packages = @($bag.Values)
			$instances += [pscustomobject]@{id=([string]$item.instanceId).ToLowerInvariant();path=[IO.Path]::GetFullPath([string]$item.installationPath).TrimEnd('\');version=[string]$item.installationVersion;product=[string]$item.productId;complete=[bool]$item.isComplete;packages=$packages;files=@{}}
		}
	} else {
		$store = Join-Path $env:ProgramData 'Microsoft\VisualStudio\Packages\_Instances'
		if ((Test-Path -LiteralPath $store) -and @(Get-ChildItem -LiteralPath $store -Force).Count) { throw 'Visual Studio registrations exist but its trusted locator is missing' }
	}
	$selected = @($instances | Where-Object { $_.path -ieq $InstallDirectory })
	if ($selected.Count -gt 1) { throw 'More than one Visual Studio instance occupies the dedicated directory' }
	if (-not $selected.Count) {
		$selected = @($instances | Where-Object { @($_.packages | Where-Object { $_.id -ceq 'Microsoft.VisualStudio.Component.VC.Tools.x86.x64' }).Count } | Sort-Object -Property @{Expression={[version]$_.version};Descending=$true},id | Select-Object -First 1)
	}
	$consumers = @()
	if ($selected.Count) {
		$instance = $selected[0]
		$versionFile = Join-Path $instance.path 'VC\Auxiliary\Build\Microsoft.VCToolsVersion.default.txt'
		if (Test-Path -LiteralPath $versionFile -PathType Leaf) {
			$toolVersion = ([IO.File]::ReadAllText($versionFile)).Trim()
			if ($toolVersion -notmatch '^[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Invalid registered C++ toolset version' }
			$files = @{'Microsoft.VCToolsVersion.default.txt'=$versionFile;'VsDevCmd.bat'=(Join-Path $instance.path 'Common7\Tools\VsDevCmd.bat');'cl.exe'=(Join-Path $instance.path "VC\Tools\MSVC\$toolVersion\bin\Hostx64\x64\cl.exe");'link.exe'=(Join-Path $instance.path "VC\Tools\MSVC\$toolVersion\bin\Hostx64\x64\link.exe")}
			foreach ($name in $files.Keys) { if (Test-Path -LiteralPath $files[$name] -PathType Leaf) { $instance.files[$name] = (Get-FileHash -LiteralPath $files[$name] -Algorithm SHA256).Hash.ToLowerInvariant() } }
		}
		$prefix = $instance.path.TrimEnd('\') + '\'
		$consumers = @(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { 'Running ' + $_.Name + ' (PID ' + $_.ProcessId + ')' } | Sort-Object -Unique)
	}
	$boot = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToUniversalTime().ToString('o')
	[pscustomobject]@{instances=$instances;selected=$(if($selected.Count){$selected[0].id}else{''});healthy=$false;occupied=(Test-Path -LiteralPath $InstallDirectory);boot=$boot;health_issue='';consumers=$consumers}
}
`

const windowsBuildToolsObserveScript = windowsBuildToolsPrelude + `
$inputData = [Console]::In.ReadToEnd() | ConvertFrom-Json
$state = Get-BuildToolsState $inputData.InstallDirectory
if ($state.selected) {
	$instance = @($state.instances | Where-Object { $_.id -eq $state.selected })[0]
	$temporary = $null
	try {
		if (-not $instance.complete -or $instance.files.Count -ne 4) { throw 'The registered C++ instance is incomplete' }
		$version = ([IO.File]::ReadAllText((Join-Path $instance.path 'VC\Auxiliary\Build\Microsoft.VCToolsVersion.default.txt'))).Trim()
		$compiler = Join-Path $instance.path "VC\Tools\MSVC\$version\bin\Hostx64\x64\cl.exe"
		$linker = Join-Path $instance.path "VC\Tools\MSVC\$version\bin\Hostx64\x64\link.exe"
		Assert-MicrosoftSignature $compiler
		Assert-MicrosoftSignature $linker
		$temporary = Join-Path ([IO.Path]::GetTempPath()) ('dotfiles-cxx-probe-' + [Guid]::NewGuid().ToString('N'))
		$null = New-Item -ItemType Directory -Path $temporary
		$source = "#include <windows.h>` + "`n" + `#include <string>` + "`n" + `int main(){std::string s(` + "`\"" + `ready` + "`\"" + `);return GetCurrentProcessId()>0 && s==` + "`\"" + `ready` + "`\"" + ` ? 0 : 1;}"
		[IO.File]::WriteAllText((Join-Path $temporary 'probe.cpp'),$source,[Text.UTF8Encoding]::new($false))
		$env:DOTFILES_VS_PROBE = $temporary
		$env:DOTFILES_VS_COMPILER = $compiler
		$env:DOTFILES_VS_DEVCMD = Join-Path $instance.path 'Common7\Tools\VsDevCmd.bat'
		$cmd = Join-Path ([Environment]::SystemDirectory) 'cmd.exe'
		# Environment substitutions are expanded once, without CALL's second
		# expansion, so spaces, Unicode and literal percent signs survive.
		$command = '""%DOTFILES_VS_DEVCMD%" -no_logo -arch=x64 -host_arch=x64 && "%DOTFILES_VS_COMPILER%" /nologo /MD /EHsc "%DOTFILES_VS_PROBE%\probe.cpp" /Fo"%DOTFILES_VS_PROBE%\probe.obj" /Fe"%DOTFILES_VS_PROBE%\probe.exe" && "%DOTFILES_VS_PROBE%\probe.exe""'
		$null = & $cmd /d /s /c $command 2>&1
		if ($LASTEXITCODE -ne 0) { throw ('The Microsoft C++/Windows SDK compile-and-run probe failed: ' + $LASTEXITCODE) }
		$state.healthy = $true
	} catch { $state.health_issue = $_.Exception.Message }
	finally { if ($temporary -and (Test-Path -LiteralPath $temporary)) { Remove-Item -LiteralPath $temporary -Recurse -Force } }
}
$state | ConvertTo-Json -Depth 6 -Compress
`

const windowsBuildToolsInstallScript = windowsBuildToolsPrelude + `
$inputData = [Console]::In.ReadToEnd() | ConvertFrom-Json
$file = [string]$inputData.File
$handle = [IO.File]::Open($file,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
try {
	if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -cne $inputData.SHA256) { throw 'Build Tools bootstrapper hash changed before execution' }
	Assert-MicrosoftSignature $file
	$state = Get-BuildToolsState $inputData.InstallDirectory
	if ($inputData.Action -eq 'install') {
		if ($state.selected -or $state.occupied) { throw 'Pre-existing C++ tools or an occupied directory appeared; preserve them and preview again' }
	} else {
		$instance = @($state.instances | Where-Object { $_.id -eq $state.selected })
		$before = @($inputData.Before.instances | Where-Object { $_.id -eq $inputData.Before.selected })
		if ($instance.Count -ne 1 -or $before.Count -ne 1 -or $state.selected -cne $inputData.Before.selected -or $instance[0].path -cne $inputData.InstallDirectory -or $instance[0].product -ne 'Microsoft.VisualStudio.Product.BuildTools' -or $instance[0].version -cne $before[0].version) { throw 'The dedicated owned instance changed before execution' }
		$expectedPackages = [Collections.Generic.Dictionary[string,int]]::new([StringComparer]::Ordinal)
		foreach ($package in $before[0].packages) { $expectedPackages.Add((Get-BuildToolsPackageKey $package),[int]$package.count) }
		if (@($instance[0].packages).Count -ne $expectedPackages.Count) { throw 'The dedicated instance package inventory changed' }
		foreach ($package in $instance[0].packages) {
			$key = Get-BuildToolsPackageKey $package
			if (-not $expectedPackages.ContainsKey($key) -or $expectedPackages[$key] -ne $package.count) { throw 'The dedicated instance package multiplicity or identity changed' }
		}
		foreach ($field in @('files')) {
			$expected = $before[0].$field
			$actual = $instance[0].$field
			if ($actual.Count -ne @($expected.PSObject.Properties).Count) { throw 'The dedicated instance inventory changed' }
			foreach ($key in $actual.Keys) { if ($actual[$key] -cne $expected.$key) { throw 'The dedicated instance package or file changed' } }
		}
		if ($inputData.Action -eq 'remove' -and $state.consumers.Count) { throw 'Running consumers block Build Tools removal' }
	}
	$arguments = @('--quiet','--wait','--norestart','--installPath',('"' + [string]$inputData.InstallDirectory + '"'))
	switch ($inputData.Action) {
		'install' { foreach ($component in $inputData.Components) { $arguments += @('--add',[string]$component) } }
		'update' { $arguments = @('update') + $arguments }
		'repair' { $arguments = @('repair') + $arguments }
		'remove' { $arguments = @('uninstall') + $arguments }
		default { throw 'Unknown Build Tools action' }
	}
	$start = [Diagnostics.ProcessStartInfo]::new()
	$start.FileName = $file
	$start.Arguments = $arguments -join ' '
	$start.UseShellExecute = $true
	$start.WorkingDirectory = [IO.Path]::GetDirectoryName($file)
	$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
	try { $administrator = ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) }
	finally { $identity.Dispose() }
	if (-not $administrator) { $start.Verb = 'runas' }
	try {
		$process = [Diagnostics.Process]::Start($start)
		try { $process.WaitForExit(); $code = [int]$process.ExitCode }
		finally { $process.Dispose() }
	} catch {
		$failure = $_.Exception
		while ($failure -and -not ($failure -is [ComponentModel.Win32Exception])) { $failure = $failure.InnerException }
		if (-not $failure -or $failure.NativeErrorCode -ne 1223) { throw }
		$code = 1223
	}
} finally { $handle.Dispose() }
exit $code
`

const windowsBuildToolsEnvironmentScript = windowsBuildToolsPrelude + `
$inputData = [Console]::In.ReadToEnd() | ConvertFrom-Json
$state = Get-BuildToolsState $inputData.InstallDirectory
if (-not $state.selected -or $state.selected -cne $inputData.Instance) { throw 'The compiler instance changed before environment resolution' }
$instance = @($state.instances | Where-Object { $_.id -eq $state.selected })[0]
$env:DOTFILES_VS_BASE = $instance.path
$env:DOTFILES_VS_SDK = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10'
$env:DOTFILES_VS_DEVCMD = Join-Path $instance.path 'Common7\Tools\VsDevCmd.bat'
$env:DOTFILES_VS_ENVHOST = [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
$command = '""%DOTFILES_VS_DEVCMD%" -no_logo -arch=x64 -host_arch=x64 && "%DOTFILES_VS_ENVHOST%" -NoLogo -NoProfile -NonInteractive -EncodedCommand ' + [string]$inputData.Probe + '"'
$output = & (Join-Path ([Environment]::SystemDirectory) 'cmd.exe') /d /s /c $command
if ($LASTEXITCODE -ne 0) { throw 'Visual Studio SDK environment resolution failed' }
$output -join [Environment]::NewLine
`

// Runs in the developer-command environment. It reads only this allowlist;
// personal environment variables and unrelated PATH entries are never emitted.
const windowsBuildToolsEnvironmentProbe = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
$roots=@($env:DOTFILES_VS_BASE,$env:DOTFILES_VS_SDK)
function Select-VendorPaths([string]$Value) {
	$paths=@()
	foreach($part in ($Value -split ';')) {
		if (-not $part) { continue }
		$path=[IO.Path]::GetFullPath($part.Trim('"')).TrimEnd('\')
		foreach($root in $roots) { if($path -ieq $root -or $path.StartsWith($root.TrimEnd('\')+'\',[StringComparison]::OrdinalIgnoreCase)) { $paths+=$path;break } }
	}
	($paths | Select-Object -Unique) -join ';'
}
$result=@{}
foreach($name in @('PATH','INCLUDE','LIB','LIBPATH')) {
	$value=Select-VendorPaths ([Environment]::GetEnvironmentVariable($name,'Process'))
	if(-not $value) { throw ('Missing approved compiler environment variable: '+$name) }
	$result[$name]=$value
}
foreach($name in @('VCINSTALLDIR','VCToolsInstallDir','WindowsSdkDir')) {
	$value=Select-VendorPaths ([Environment]::GetEnvironmentVariable($name,'Process'))
	if(-not $value -or $value.Contains(';')) { throw ('Missing approved compiler directory: '+$name) }
	$result[$name]=$value+'\'
}
$version=[Environment]::GetEnvironmentVariable('WindowsSDKVersion','Process')
if($version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\\?$') { throw 'Invalid Windows SDK environment version' }
$result.WindowsSDKVersion=$version
$result | ConvertTo-Json -Compress
`
