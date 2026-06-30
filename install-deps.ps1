# install-deps.ps1 -- interactively install dependencies on Windows.
#
# Prefers scoop per tool (most reliable for these CLI tools; sidesteps the flaky
# winget "No package found matching input criteria" source errors), then falls
# back to winget, then chocolatey -- if one manager fails for a tool, the next
# is tried automatically. Offers to bootstrap scoop when missing.
# Ensures the Scoop extras and nerd-fonts buckets when Scoop is available.
# Prints manual-install hints only when no manager carries the package.
#
# Usage:
#   .\install-deps.ps1            show a dependency table, then prompt once
#   .\install-deps.ps1 -All       skip prompts, install everything
#   .\install-deps.ps1 -Update    update present manager-owned catalog tools
#   .\install-deps.ps1 -DryRun    print what would be installed without acting

[CmdletBinding()]
param(
    [switch]$All,
    [switch]$Update,
    [switch]$DryRun
)

$ErrorActionPreference = 'Continue'
$HackNerdFontVersion = 'v3.4.0'
$HackNerdFontSha256 = '8ca33a60c791392d872b80d26c42f2bfa914a480f9eb2d7516d9f84373c36897'
$WindowsTerminalVersion = 'v1.24.11321.0'
$WindowsTerminalX64Sha256 = '7caef554147e5498ed1becdca73cdedb79fbc81f89032e46ae9b095c53433812'
$PsmuxPluginsCommit = '0f46ccca5a9b748fd03851db00b85fd784f42791'
$PsmuxPluginsRepo = 'https://github.com/psmux/psmux-plugins.git'
$VsBuildToolsBootstrapperUrl = 'https://aka.ms/vs/17/release/vs_BuildTools.exe'
$PylatexencBuildBackendVersion = '80.9.0'
$PylatexencBuildBackendSha256 = '062d34222ad13e0cc312a4c02d73f059e86a4acbfbdea8f8f76b28c99f306922'
$PylatexencVersion = '2.10'
$PylatexencSha256 = '3dd8fd84eb46dc30bee1e23eaab8d8fb5a7f507347b23e5f38ad9675c84f40d3'

# ---- Package-manager detection + scoop bootstrap -----------------------------
function Get-AvailablePM {
    if (Get-Command winget -ErrorAction SilentlyContinue) { return 'winget' }
    if (Get-Command choco  -ErrorAction SilentlyContinue) { return 'choco'  }
    if (Get-Command scoop  -ErrorAction SilentlyContinue) { return 'scoop'  }
    return $null
}

function Test-IsElevated {
    try {
        $id = [Security.Principal.WindowsIdentity]::GetCurrent()
        return ([Security.Principal.WindowsPrincipal]$id).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    } catch {
        return $false
    }
}

function Get-ScoopRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:SCOOP)) { return $env:SCOOP }
    $base = $env:USERPROFILE
    if ([string]::IsNullOrWhiteSpace($base)) { $base = $HOME }
    if ([string]::IsNullOrWhiteSpace($base)) { $base = [Environment]::GetFolderPath('UserProfile') }
    if ([string]::IsNullOrWhiteSpace($base)) { $base = [System.IO.Path]::GetTempPath() }
    return (Join-Path $base 'scoop')
}

function Add-ScoopToPathForCurrentProcess {
    $scoopRoot = Get-ScoopRoot
    $shimDir = Join-Path $scoopRoot 'shims'
    if ((Test-Path -LiteralPath $shimDir) -and (($env:PATH -split ';') -notcontains $shimDir)) {
        $env:PATH = "$shimDir;$env:PATH"
    }
}

function Add-ScoopBucketSafe {
    # Idempotent, non-interactive `scoop bucket add`. Returns $true if the bucket
    # is present AND populated afterward, $false otherwise. NEVER throws, so a
    # failed clone falls through to the next package manager instead of hanging
    # or aborting (matters under a Stop-strict ErrorActionPreference too -- the
    # chezmoi run-script port relies on this).
    #
    # Hardens two real, sporadic failures of `scoop bucket add` (it git-clones):
    #   1) git / Git Credential Manager prompting (or popping a browser) over a
    #      non-interactive console (psmux / SSH / setup.ps1 / chez apply) -> a
    #      credential challenge would otherwise HANG the whole run and eventually
    #      surface as "authentication failed". GIT_TERMINAL_PROMPT=0 +
    #      GCM_INTERACTIVE=0 make git/GCM FAIL FAST instead.
    #   2) ScoopInstaller/Scoop#5482 / #5814: `scoop bucket add` reports success
    #      even when the underlying clone fails, leaving an EMPTY bucket. We verify
    #      the bucket dir is non-empty and purge a half-clone so retry is clean.
    #
    # When $Url is empty, fall back to the bare `scoop bucket add <name>` form so
    # the scoop known-bucket table resolves the canonical URL (extras / nerd-fonts).
    param(
        [Parameter(Mandatory)][string]$Name,
        [string]$Url = ''
    )
    $scoopRoot = Get-ScoopRoot
    $bucketDir = Join-Path (Join-Path $scoopRoot 'buckets') $Name
    $populated = {
        (Test-Path -LiteralPath $bucketDir) -and
        (@(Get-ChildItem -LiteralPath $bucketDir -Force -ErrorAction SilentlyContinue).Count -gt 0)
    }

    if (& $populated) { return $true }   # already added + populated: skip the clone

    $oldPrompt = $env:GIT_TERMINAL_PROMPT
    $oldGcm = $env:GCM_INTERACTIVE
    $env:GIT_TERMINAL_PROMPT = '0'   # git: no terminal prompt -> fail instead of block
    $env:GCM_INTERACTIVE = '0'       # GCM: never prompt / open a browser -> fail fast
    try {
        foreach ($attempt in 1..2) {
            # 2>&1 keeps the diagnostic (NOT 2>$null) so a real failure is visible.
            if ([string]::IsNullOrEmpty($Url)) {
                scoop bucket add $Name 2>&1 | Out-Null
            } else {
                scoop bucket add $Name $Url 2>&1 | Out-Null
            }
            if (& $populated) { return $true }
            # Purge a half-cloned / empty bucket so the next attempt starts clean
            # (Scoop#5482: a registered-but-empty bucket otherwise blocks re-add).
            if (Test-Path -LiteralPath $bucketDir) {
                Remove-Item -LiteralPath $bucketDir -Recurse -Force -ErrorAction SilentlyContinue
            }
            scoop bucket rm $Name 2>&1 | Out-Null
        }
        Write-Warning ("scoop bucket add {0} did not populate a usable bucket; recover with 'scoop bucket rm {0}' then re-run" -f $Name)
        return $false
    } finally {
        $env:GIT_TERMINAL_PROMPT = $oldPrompt
        $env:GCM_INTERACTIVE = $oldGcm
    }
}

function Ensure-ScoopBuckets {
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) { return }
    if ($DryRun) { return }
    # `scoop bucket add` clones the bucket repo with git, so git MUST exist first.
    # On a truly fresh machine (Windows Sandbox, clean install) git is not present
    # yet at this point, and the extras/nerd-fonts adds fail with "Git is required
    # for buckets". Install git from the main bucket first -- main ships with scoop
    # and needs no git -- before adding the other buckets. (CI runners come with
    # git preinstalled, which hid this on every hosted job.)
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Host "  scoop: installing git first (required to clone buckets)"
        try {
            scoop install git
            Add-ScoopToPathForCurrentProcess
        } catch {
            Write-Warning ("scoop install git failed; bucket adds may fail: " + $_.Exception.Message)
        }
    }
    Add-ScoopBucketSafe -Name 'extras' | Out-Null
    Add-ScoopBucketSafe -Name 'nerd-fonts' | Out-Null
}

function Install-Scoop {
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        Ensure-ScoopBuckets
        return $true
    }
    Write-Host "Scoop is not installed. It is a userspace package manager that"
    Write-Host "carries tools missing from winget/choco (taplo, win32yank, etc.)."
    if (-not (Ask "Install Scoop via the official one-liner?")) { return $false }
    if ($DryRun) {
        if (Test-IsElevated) {
            Write-Host "  would: download get.scoop.sh, then run install.ps1 -RunAsAdmin"
        } else {
            Write-Host "  would: download get.scoop.sh, then run install.ps1"
        }
        return $false
    }
    try {
        # The official scoop bootstrap. RemoteSigned policy is needed for the
        # script; it is set for the current process only.
        Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process -Force
        $installer = Join-Path $env:TEMP "scoop-install-$([guid]::NewGuid()).ps1"
        Invoke-WebRequest -Uri 'https://get.scoop.sh' -OutFile $installer -UseBasicParsing -ErrorAction Stop
        if (Test-IsElevated) {
            # GitHub Windows runners are elevated. Scoop blocks elevated
            # bootstrap by default, so use the installer documented opt-in
            # instead of trying to de-elevate the CI process.
            & $installer -RunAsAdmin
        } else {
            & $installer
        }
        Remove-Item -LiteralPath $installer -Force -ErrorAction SilentlyContinue
        Add-ScoopToPathForCurrentProcess
        # Add the standard buckets so existing catalog entries resolve.
        Ensure-ScoopBuckets
        return [bool](Get-Command scoop -ErrorAction SilentlyContinue)
    } catch {
        if ($installer) {
            Remove-Item -LiteralPath $installer -Force -ErrorAction SilentlyContinue
        }
        Write-Warning ("Scoop install failed: " + $_.Exception.Message)
        return $false
    }
}

# Ask early -- needed by the catalog logic below.
function Ask {
    param([string]$prompt)
    if ($All -or $DryRun) { return $true }
    $resp = Read-Host "  $prompt [Y/n]"
    if ([string]::IsNullOrWhiteSpace($resp)) { return $true }
    return ($resp -match '^[Yy]')
}

function Test-InstallPromptAvailable {
    if ($All -or $DryRun) { return $false }
    if (-not [Environment]::UserInteractive) { return $false }
    try {
        if ([Console]::IsInputRedirected) { return $false }
    } catch {
        return $false
    }
    return $true
}

function Test-FileSha256 {
    param([string]$Path, [string]$Expected)
    return ((Get-FileHash $Path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $Expected.ToLowerInvariant())
}

function Normalize-PathListEntry {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return '' }
    return $Value.Trim().TrimEnd([char[]]@([char]92, [char]47))
}

function Test-PathListContains {
    param([string]$PathValue, [string]$Directory)
    if ([string]::IsNullOrEmpty($PathValue)) { return $false }
    $needle = Normalize-PathListEntry $Directory
    if (-not $needle) { return $false }
    foreach ($part in ($PathValue -split ';')) {
        if ((Normalize-PathListEntry $part).Equals($needle, [StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }
    return $false
}

function Add-DirectoryToUserPath {
    param([Parameter(Mandatory)][string]$Directory)
    $full = [IO.Path]::GetFullPath($Directory)
    if (-not (Test-Path -LiteralPath $full -PathType Container)) {
        throw "PATH directory does not exist: $full"
    }

    if (-not (Test-PathListContains -PathValue $env:PATH -Directory $full)) {
        $env:PATH = "$full;$env:PATH"
    }

    $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if (-not (Test-PathListContains -PathValue $userPath -Directory $full)) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $full } else { "$userPath;$full" }
        [Environment]::SetEnvironmentVariable('PATH', $newUserPath, 'User')
    }
}

function Get-DotfilesDataRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        return (Join-Path $env:LOCALAPPDATA 'dotfiles')
    }
    $base = if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
        $env:USERPROFILE
    } elseif (-not [string]::IsNullOrWhiteSpace($HOME)) {
        $HOME
    } else {
        [System.IO.Path]::GetTempPath()
    }
    return (Join-Path (Join-Path $base '.local') 'share\dotfiles')
}

function Get-PylatexencVenvRoot {
    return (Join-Path (Join-Path (Get-DotfilesDataRoot) 'python-tools') 'pylatexenc')
}

function Invoke-PythonCommand {
    param(
        [Parameter(Mandatory)][string]$Python,
        [Parameter(Mandatory)][string[]]$Arguments
    )
    & $Python @Arguments
}

# ---- Per-tool: package id per PM. Empty string means "not available there". --
# Keys are the command name we check via Get-Command.
$Catalog = @{
    git                  = @{ winget = 'Git.Git';                          choco = 'git';                  scoop = 'git'                  ; purpose = 'version control' }
    nvim                 = @{ winget = 'Neovim.Neovim';                    choco = 'neovim';               scoop = 'neovim'               ; purpose = 'Neovim 0.12+ editor' }
    starship             = @{ winget = 'Starship.Starship';                choco = 'starship';             scoop = 'starship'             ; purpose = 'cross-shell prompt' }
    rg                   = @{ winget = 'BurntSushi.ripgrep.MSVC';          choco = 'ripgrep';              scoop = 'ripgrep'              ; purpose = 'Telescope live_grep backend' }
    fd                   = @{ winget = 'sharkdp.fd';                       choco = 'fd';                   scoop = 'fd'                   ; purpose = 'Telescope find_files backend' }
    fzf                  = @{ winget = 'junegunn.fzf';                     choco = 'fzf';                  scoop = 'fzf'                  ; purpose = 'fuzzy finder (PSFzf history/file/dir pickers)' }
    lsd                  = @{ winget = 'lsd-rs.lsd';                       choco = 'lsd';                  scoop = 'lsd'                  ; purpose = 'modern ls replacement with colors, icons, and tree view' }
    chezmoi              = @{ winget = 'twpayne.chezmoi';                  choco = 'chezmoi';              scoop = 'chezmoi'              ; purpose = 'dotfiles config manager' }
    lazygit              = @{ winget = 'JesseDuffield.lazygit';            choco = 'lazygit';              scoop = 'lazygit'              ; purpose = 'terminal git UI' }
    wt                   = @{ winget = 'Microsoft.WindowsTerminal';        choco = 'microsoft-windows-terminal'; scoop = 'extras/windows-terminal'; purpose = 'Windows Terminal host for PowerShell and WSL' }
    make                 = @{ winget = 'GnuWin32.Make';                    choco = 'make';                 scoop = 'make'                 ; purpose = 'plugin builds (LuaSnip jsregexp)' }
    cmake                = @{ winget = 'Kitware.CMake';                    choco = 'cmake';                scoop = 'cmake'                ; purpose = 'CMake CLI required by neocmakelsp and CMake projects' }
    pwsh                 = @{ winget = 'Microsoft.PowerShell';             choco = 'powershell-core';      scoop = 'pwsh'                 ; purpose = 'modern PowerShell 7' }
    'win32yank'          = @{ winget = '';                                 choco = 'win32yank';            scoop = 'win32yank'            ; purpose = 'clipboard bridge for WSL nvim' }
    node                 = @{ winget = 'OpenJS.NodeJS.LTS';                choco = 'nodejs-lts';           scoop = 'nodejs-lts'           ; purpose = 'prettier + JS tooling' }
    'tree-sitter'        = @{                                               scoop = 'tree-sitter'           ; purpose = 'nvim-treesitter main: parser generate/build CLI' }
    python               = @{ winget = 'Python.Python.3.12';               choco = 'python';               scoop = 'python'               ; purpose = 'pyright + tooling' }
    zig                  = @{ winget = 'zig.zig';                          choco = 'zig';                  scoop = 'zig'                  ; purpose = 'C compiler for the LuaSnip jsregexp build' }
    jq                   = @{ winget = 'jqlang.jq';                        choco = 'jq';                   scoop = 'jq'                   ; purpose = 'general-purpose JSON CLI' }
    shellcheck           = @{ winget = 'koalaman.shellcheck';              choco = 'shellcheck';           scoop = 'shellcheck'           ; purpose = 'shell-script linter' }
    hyperfine            = @{ winget = 'sharkdp.hyperfine';                choco = 'hyperfine';            scoop = 'hyperfine'            ; purpose = 'starship perf benchmark' }
    taplo                = @{ winget = '';                                 choco = '';                     scoop = 'taplo'                ; purpose = 'TOML linter' }
    code                 = @{ winget = 'Microsoft.VisualStudioCode';       choco = 'vscode';               scoop = 'extras/vscode'        ; purpose = 'VS Code editor' }
}

# Some Catalog keys (e.g. "rg") map to a different actual binary on Windows
# than on Unix. Provide a name -> binary mapping for Get-Command checks.
$BinaryName = @{
    scoop       = 'scoop'
    rg          = 'rg'
    fd          = 'fd'
    fzf         = 'fzf'
    lsd         = 'lsd'
    chezmoi     = 'chezmoi'
    lazygit     = 'lazygit'
    wt          = 'wt'
    nvim        = 'nvim'
    pwsh        = 'pwsh'
    'win32yank' = 'win32yank'
    starship    = 'starship'
    git         = 'git'
    make        = 'make'
    cmake       = 'cmake'
    node        = 'node'
    'tree-sitter' = 'tree-sitter'
    python      = 'python'
    zig         = 'zig'
    jq          = 'jq'
    shellcheck  = 'shellcheck'
    hyperfine   = 'hyperfine'
    taplo       = 'taplo'
    code        = 'code'
    psmux       = 'psmux'
}

function Test-Tool {
    param([string]$name)
    return [bool](Get-Command $BinaryName[$name] -ErrorAction SilentlyContinue)
}

function Get-RealPythonCommand {
    # The Microsoft Store ships "App execution alias" stubs for python.exe and
    # python3.exe under %LOCALAPPDATA%\Microsoft\WindowsApps that are NOT a real
    # Python -- run with no args they just open the Store. Get-Command finds them,
    # so a naive Test-Tool reports python as installed; Mason then fails its PyPI
    # tools (clang-format / ruff / gersemi) with "Unable to find python3
    # installation in PATH". Return the first python on PATH that is NOT that stub.
    foreach ($name in @('python', 'python3')) {
        foreach ($candidate in @(Get-Command $name -All -ErrorAction SilentlyContinue)) {
            $source = $candidate.Source
            if ([string]::IsNullOrWhiteSpace($source)) { continue }
            if ($source -like '*\WindowsApps\*') { continue }
            return $candidate
        }
    }
    return $null
}

function Install-Python {
    # Install a REAL python, not the Store stub (see Get-RealPythonCommand). Pass
    # the stub-rejecting check to Install-One so the stub never short-circuits the
    # install, then add the real python directory to the user PATH so the Mason
    # sync and future shells resolve it ahead of the stub.
    Install-One python -InstalledCheck { [bool](Get-RealPythonCommand) }
    $real = Get-RealPythonCommand
    if ($real -and -not [string]::IsNullOrWhiteSpace($real.Source)) {
        Add-DirectoryToUserPath -Directory (Split-Path -Parent $real.Source)
    }
}

function Test-PylatexencConverter {
    param([string]$VenvRoot = (Get-PylatexencVenvRoot))
    $scriptsDir = Join-Path $VenvRoot 'Scripts'
    $venvPython = Join-Path $scriptsDir 'python.exe'
    $converter = Join-Path $scriptsDir 'latex2text.exe'
    if (-not (Test-Path -LiteralPath $venvPython -PathType Leaf)) { return $false }
    if (-not (Test-Path -LiteralPath $converter -PathType Leaf)) { return $false }

    $probe = @'
import importlib.metadata
import sys

try:
    version = importlib.metadata.version("pylatexenc")
except importlib.metadata.PackageNotFoundError:
    raise SystemExit(1)

raise SystemExit(0 if version == sys.argv[1] else 1)
'@
    Invoke-PythonCommand -Python $venvPython -Arguments @('-c', $probe, $PylatexencVersion) | Out-Null
    return ($LASTEXITCODE -eq 0)
}

function Install-PylatexencConverter {
    $venvRoot = Get-PylatexencVenvRoot
    $scriptsDir = Join-Path $venvRoot 'Scripts'
    if (Test-PylatexencConverter -VenvRoot $venvRoot) {
        Add-DirectoryToUserPath -Directory $scriptsDir
        Write-Host ("  ok        {0,-26} pylatexenc {1}" -f "latex2text", $PylatexencVersion)
        return
    }

    if (-not (Ask "Install latex2text via a pinned pylatexenc venv (Markdown equations)?")) {
        Write-Host ("  skipped   {0,-26}" -f "latex2text")
        return
    }
    if ($DryRun) {
        Write-Host ("  would:    python -m venv {0}" -f $venvRoot)
        Write-Host ("  would:    pip install --require-hashes setuptools=={0}" -f $PylatexencBuildBackendVersion)
        Write-Host ("             sha256={0}" -f $PylatexencBuildBackendSha256)
        Write-Host ("  would:    pip install --require-hashes --no-build-isolation pylatexenc=={0}" -f $PylatexencVersion)
        Write-Host ("             sha256={0}" -f $PylatexencSha256)
        Write-Host ("  would:    add {0} to User PATH" -f $scriptsDir)
        return
    }

    $real = Get-RealPythonCommand
    if (-not $real) {
        Install-Python
        $real = Get-RealPythonCommand
    }
    if (-not $real -or [string]::IsNullOrWhiteSpace($real.Source)) {
        Write-Warning "python is required before installing latex2text"
        $script:InstallFailures += [pscustomobject]@{ Tool='latex2text'; Pm='python'; Pkg='pylatexenc'; ExitCode='python-missing' }
        return
    }

    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $venvRoot) | Out-Null
    Invoke-PythonCommand -Python $real.Source -Arguments @('-m', 'venv', $venvRoot)
    if ($LASTEXITCODE -ne 0) {
        Write-Warning ("python venv creation failed for pylatexenc (exit {0})" -f $LASTEXITCODE)
        $script:InstallFailures += [pscustomobject]@{ Tool='latex2text'; Pm='python'; Pkg='pylatexenc'; ExitCode=$LASTEXITCODE }
        return
    }

    $venvPython = Join-Path $scriptsDir 'python.exe'
    if (-not (Test-Path -LiteralPath $venvPython -PathType Leaf)) {
        Write-Warning "python venv creation did not create Scripts\python.exe for pylatexenc"
        $script:InstallFailures += [pscustomobject]@{ Tool='latex2text'; Pm='python'; Pkg='pylatexenc'; ExitCode='missing-venv-python' }
        return
    }

    $requirements = New-TemporaryFile
    try {
        Set-Content -LiteralPath $requirements -Value ("setuptools=={0} --hash=sha256:{1}" -f $PylatexencBuildBackendVersion, $PylatexencBuildBackendSha256) -Encoding ascii
        Invoke-PythonCommand -Python $venvPython -Arguments @('-m', 'pip', 'install', '--disable-pip-version-check', '--no-cache-dir', '--require-hashes', '--only-binary=:all:', '--no-deps', '-r', $requirements)
        if ($LASTEXITCODE -ne 0) {
            Write-Warning ("pinned setuptools install failed (exit {0})" -f $LASTEXITCODE)
            $script:InstallFailures += [pscustomobject]@{ Tool='latex2text'; Pm='pip'; Pkg='setuptools'; ExitCode=$LASTEXITCODE }
            return
        }

        Set-Content -LiteralPath $requirements -Value ("pylatexenc=={0} --hash=sha256:{1}" -f $PylatexencVersion, $PylatexencSha256) -Encoding ascii
        Invoke-PythonCommand -Python $venvPython -Arguments @('-m', 'pip', 'install', '--disable-pip-version-check', '--no-cache-dir', '--require-hashes', '--no-deps', '--no-build-isolation', '-r', $requirements)
        if ($LASTEXITCODE -ne 0) {
            Write-Warning ("pylatexenc install failed (exit {0})" -f $LASTEXITCODE)
            $script:InstallFailures += [pscustomobject]@{ Tool='latex2text'; Pm='pip'; Pkg='pylatexenc'; ExitCode=$LASTEXITCODE }
            return
        }
    } finally {
        Remove-Item -LiteralPath $requirements -Force -ErrorAction SilentlyContinue
    }

    if (-not (Test-Path -LiteralPath (Join-Path $scriptsDir 'latex2text.exe') -PathType Leaf)) {
        Write-Warning "pylatexenc installed without executable latex2text"
        $script:InstallFailures += [pscustomobject]@{ Tool='latex2text'; Pm='pip'; Pkg='pylatexenc'; ExitCode='missing-latex2text' }
        return
    }

    Add-DirectoryToUserPath -Directory $scriptsDir
    Write-Host ("  installed {0,-26} pylatexenc {1}" -f "latex2text", $PylatexencVersion)
}

function Get-InstallDependencySpec {
    $toolOrder = @(
        'scoop',
        'git',
        'nvim',
        'make',
        'cmake',
        'rg',
        'fd',
        'fzf',
        'lsd',
        'chezmoi',
        'lazygit',
        'starship',
        'wt',
        'psmux',
        'psmux plugins',
        'pwsh',
        'python',
        'node',
        'tree-sitter',
        'zig',
        'win32yank',
        'jq',
        'shellcheck',
        'hyperfine',
        'taplo',
        'code'
    )
    $emitted = @{}
    foreach ($tool in $toolOrder) {
        if (($tool -eq 'scoop') -or ($tool -eq 'psmux') -or ($tool -eq 'psmux plugins') -or $Catalog.ContainsKey($tool)) {
            $emitted[$tool] = $true
            [pscustomobject]@{
                Tool = $tool
                Kind = if ($tool -eq 'psmux plugins') { 'psmux-plugins' } else { 'tool' }
                Binary = $BinaryName[$tool]
                Module = ''
            }
        }
    }
    foreach ($tool in ($Catalog.Keys | Sort-Object)) {
        if (-not $emitted.ContainsKey($tool)) {
            [pscustomobject]@{
                Tool = $tool
                Kind = 'tool'
                Binary = $BinaryName[$tool]
                Module = ''
            }
        }
    }
    [pscustomobject]@{ Tool = 'PSFzf'; Kind = 'module'; Binary = ''; Module = 'PSFzf' }
    [pscustomobject]@{ Tool = 'Hack Nerd Font'; Kind = 'font'; Binary = ''; Module = '' }
}

function Get-CommandVersionString {
    param([string]$CommandName)
    if ([string]::IsNullOrWhiteSpace($CommandName)) { return '-' }
    $cmd = Get-Command -Name $CommandName -ErrorAction SilentlyContinue
    if (-not $cmd) { return '-' }

    # NEVER run `wt --version`: Windows Terminal (wt.exe) does NOT print a version
    # to stdout -- it LAUNCHES a new terminal window to show it, which pops an
    # annoying window during the dependency pre-flight table. Read the file
    # version instead (works for the portable install; Store/scoop shims fall back
    # to "installed"). Any other windowed/GUI tool belongs in this skip list.
    if ($CommandName -in @('wt')) {
        try {
            $src = if ($cmd.PSObject.Properties.Name -contains 'Source') { $cmd.Source } else { $cmd.Path }
            if ($src -and (Test-Path -LiteralPath $src -PathType Leaf)) {
                $ver = (Get-Item -LiteralPath $src).VersionInfo.ProductVersion
                if (-not [string]::IsNullOrWhiteSpace($ver)) { return ([string]$ver).Trim() }
            }
        } catch { }
        return 'installed'
    }

    try {
        $lines = @(& $CommandName --version 2>$null)
        foreach ($line in $lines) {
            $text = [string]$line
            if (-not [string]::IsNullOrWhiteSpace($text)) {
                return $text.Trim()
            }
        }
    } catch {
        return '-'
    }
    return '-'
}

function Get-ModuleVersionString {
    param([string]$Name)
    $module = Get-Module -ListAvailable -Name $Name |
        Sort-Object -Property Version -Descending |
        Select-Object -First 1
    if ($module) { return $module.Version.ToString() }
    return '-'
}

function Test-InstallDependencyPresent {
    param([Parameter(Mandatory)]$Spec)
    switch ($Spec.Kind) {
        'tool' { return (Test-Tool $Spec.Tool) }
        'module' { return [bool](Get-Module -ListAvailable -Name $Spec.Module) }
        'font' { return (Test-HackNerdFontInstalled) }
        'psmux-plugins' {
            return (
                (Test-PsmuxPluginPin -Name 'ppm' -Subdir 'ppm' -RequiredFile 'ppm.ps1') -and
                (Test-PsmuxPluginPin -Name 'psmux-theme-rosepine' -Subdir 'psmux-theme-rosepine' -RequiredFile 'psmux-theme-rosepine.ps1')
            )
        }
        default { return $false }
    }
}

function Get-InstallDependencyVersion {
    param([Parameter(Mandatory)]$Spec)
    switch ($Spec.Kind) {
        'tool' { return (Get-CommandVersionString -CommandName $Spec.Binary) }
        'module' { return (Get-ModuleVersionString -Name $Spec.Module) }
        'psmux-plugins' { return $PsmuxPluginsCommit }
        default { return '-' }
    }
}

function Get-InstallDependencyScan {
    param(
        [object[]]$SpecList,
        [scriptblock]$PresenceTester,
        [scriptblock]$VersionGetter
    )
    if ($null -eq $SpecList) {
        $SpecList = @(Get-InstallDependencySpec)
    }
    foreach ($spec in $SpecList) {
        $present = if ($PresenceTester) {
            [bool](& $PresenceTester $spec)
        } else {
            Test-InstallDependencyPresent -Spec $spec
        }
        $version = '-'
        $status = 'missing'
        $action = 'install'
        if ($present) {
            $status = 'present'
            $action = 'skip'
            $version = if ($VersionGetter) {
                [string](& $VersionGetter $spec)
            } else {
                Get-InstallDependencyVersion -Spec $spec
            }
            if ([string]::IsNullOrWhiteSpace($version)) {
                $version = '-'
            }
        }
        [pscustomobject]@{
            Tool = $spec.Tool
            Status = $status
            Version = $version
            Action = $action
        }
    }
}

function Format-InstallDependencyTable {
    param([Parameter(Mandatory)][object[]]$Rows)
    $lines = [System.Collections.Generic.List[string]]::new()
    $lines.Add('Dependency pre-flight:')
    $lines.Add(('{0,-22} {1,-8} {2,-34} {3,-7}' -f 'Tool', 'Status', 'Version', 'Action'))
    $lines.Add(('{0,-22} {1,-8} {2,-34} {3,-7}' -f '----------------------', '--------', '----------------------------------', '-------'))
    $present = 0
    $missing = 0
    foreach ($row in $Rows) {
        $lines.Add(('{0,-22} {1,-8} {2,-34} {3,-7}' -f $row.Tool, $row.Status, $row.Version, $row.Action))
        if ($row.Status -eq 'present') {
            $present++
        } else {
            $missing++
        }
    }
    $lines.Add(('{0} present, {1} missing' -f $present, $missing))
    return $lines.ToArray()
}

function Show-InstallDependencyTable {
    param([Parameter(Mandatory)][object[]]$Rows)
    foreach ($line in (Format-InstallDependencyTable -Rows $Rows)) {
        Write-Host $line
    }
}

function Install-One {
    param([string]$tool, [switch]$SkipPrompt, [switch]$NoRecordFailure, [scriptblock]$InstalledCheck)
    # Callers can override the install-check (e.g. python, whose Store stub fools
    # Test-Tool -- see Get-RealPythonCommand). Call Test-Tool DIRECTLY in the
    # default case (not via a `& {Test-Tool}` scriptblock, which runs in a child
    # scope a Pester mock of Test-Tool cannot reach). NB: the param is
    # $InstalledCheck, not $Installed -- PowerShell variable names are
    # case-insensitive, so $Installed would alias the boolean $installed flag below.
    $alreadyInstalled = if ($InstalledCheck) { [bool](& $InstalledCheck) } else { Test-Tool $tool }
    if ($alreadyInstalled) {
        Write-Host ("  ok        {0,-26} already installed" -f $tool)
        return
    }
    $entry = $Catalog[$tool]
    if (-not $entry) {
        Write-Host ("  skipped   {0,-26} no catalog entry" -f $tool)
        return
    }

    # Ordered candidate managers, deduped: prefer SCOOP -- it carries every CLI
    # tool here in its main bucket and avoids the flaky winget "No package found
    # matching input criteria" (exit -1978335212) source errors. Then the
    # detected primary, then the rest. Only PMs that are installed AND carry a
    # package id for this tool make the list.
    $order = @('scoop', $Pm, 'winget', 'choco')
    $candidates = @()
    foreach ($p in $order) {
        if (-not $p) { continue }
        if (-not $entry.$p) { continue }
        if ($candidates.pm -contains $p) { continue }
        if (-not (Get-Command $p -ErrorAction SilentlyContinue)) { continue }
        $candidates += [pscustomobject]@{ pm = $p; pkg = $entry.$p }
    }
    if ($candidates.Count -eq 0) {
        Write-Host ("  manual    {0,-26} not in scoop/winget/choco; install separately" -f $tool)
        return
    }

    $first = $candidates[0]
    $purpose = $entry.purpose
    $promptText = if ($purpose) { "Install ${tool} via $($first.pm) (${purpose})?" } else { "Install ${tool} via $($first.pm)?" }
    if ((-not $SkipPrompt) -and (-not (Ask $promptText))) {
        Write-Host ("  skipped   {0,-26}" -f $tool)
        return
    }
    if ($DryRun) {
        $fallback = if ($candidates.Count -gt 1) {
            "   (fallback: " + (($candidates | Select-Object -Skip 1 | ForEach-Object { $_.pm }) -join ', ') + ")"
        } else { "" }
        Write-Host ("  would:    $($first.pm) install $($first.pkg)$fallback")
        return
    }

    # Try each manager in order; fall back to the next one on failure. This is
    # the key fix: a winget "no package found" no longer dead-ends the tool.
    $installed = $false
    foreach ($c in $candidates) {
        switch ($c.pm) {
            'winget' { winget install --id $c.pkg -e --accept-source-agreements --accept-package-agreements --silent }
            'choco'  { choco install $c.pkg -y }
            'scoop'  { scoop install $c.pkg }
        }
        if ($LASTEXITCODE -eq 0 -and $(if ($InstalledCheck) { [bool](& $InstalledCheck) } else { Test-Tool $tool })) {
            Write-Host ("  installed {0,-26} via {1}" -f $tool, $c.pm)
            $installed = $true
            break
        }
        Write-Warning ("  $($c.pm) install of $($c.pkg) failed (exit $LASTEXITCODE); trying next manager...")
    }
    if (-not $installed) {
        # Track failures so we can summarize at the end instead of faking success.
        $tried = ($candidates | ForEach-Object { $_.pm }) -join '/'
        if (-not $NoRecordFailure) {
            $script:InstallFailures += [pscustomobject]@{ Tool = $tool; Pm = $tried; Pkg = $first.pkg; ExitCode = $LASTEXITCODE }
        }
    }
}

# ---- Optional: keep a single catalog tool current ----------------------------
# Update mode is scoped to present catalog tools that a supported package manager
# actually owns. It never runs blanket upgrades such as `scoop update *`,
# `winget upgrade --all`, or `choco upgrade all`. A present tool outside
# Scoop/winget/Chocolatey is reported as unmanaged instead of silently skipped.
function Get-CatalogPackageId {
    param([string]$tool, [string]$Manager)
    if (-not $Catalog.ContainsKey($tool)) { return '' }
    $pkg = $Catalog[$tool].$Manager
    if ([string]::IsNullOrWhiteSpace([string]$pkg)) { return '' }
    return [string]$pkg
}

function Get-CatalogToolCommandSource {
    param([string]$tool)
    if (-not $BinaryName.ContainsKey($tool)) { return '' }
    $binary = $BinaryName[$tool]
    if ([string]::IsNullOrWhiteSpace($binary)) { return '' }
    $cmd = Get-Command $binary -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $cmd) { return '' }
    foreach ($prop in @('Source', 'Path', 'Definition')) {
        if (($cmd.PSObject.Properties.Name -contains $prop) -and
            -not [string]::IsNullOrWhiteSpace([string]$cmd.$prop)) {
            return [string]$cmd.$prop
        }
    }
    return ''
}

function ConvertTo-WindowsComparablePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
    return ([string]$Path).Trim().Trim('"').Replace('/', '\').TrimEnd('\')
}

function Get-WindowsPathDirectoryText {
    param([string]$Path)
    $normalized = ConvertTo-WindowsComparablePath -Path $Path
    if ([string]::IsNullOrWhiteSpace($normalized)) { return '' }
    $index = $normalized.LastIndexOf('\')
    if ($index -lt 0) { return '' }
    return $normalized.Substring(0, $index)
}

function Get-WindowsPathFileBaseText {
    param([string]$Path)
    $normalized = ConvertTo-WindowsComparablePath -Path $Path
    if ([string]::IsNullOrWhiteSpace($normalized)) { return '' }
    $index = $normalized.LastIndexOf('\')
    $name = if ($index -ge 0) { $normalized.Substring($index + 1) } else { $normalized }
    $dot = $name.LastIndexOf('.')
    if ($dot -gt 0) { return $name.Substring(0, $dot) }
    return $name
}

function Join-WindowsPathText {
    param([string]$Left, [string]$Right)
    $leftText = ConvertTo-WindowsComparablePath -Path $Left
    $rightText = (ConvertTo-WindowsComparablePath -Path $Right).TrimStart('\')
    if ([string]::IsNullOrWhiteSpace($leftText)) { return $rightText }
    if ([string]::IsNullOrWhiteSpace($rightText)) { return $leftText }
    return "$leftText\$rightText"
}

function Test-WindowsPathUnderDirectoryText {
    param([string]$Path, [string]$Directory)
    $pathText = ConvertTo-WindowsComparablePath -Path $Path
    $directoryText = ConvertTo-WindowsComparablePath -Path $Directory
    if ([string]::IsNullOrWhiteSpace($pathText) -or [string]::IsNullOrWhiteSpace($directoryText)) {
        return $false
    }
    return $pathText.Equals($directoryText, [StringComparison]::OrdinalIgnoreCase) -or
        $pathText.StartsWith("$directoryText\", [StringComparison]::OrdinalIgnoreCase)
}

function Get-ScoopRootCandidates {
    $roots = @()
    foreach ($candidate in @((Get-ScoopRoot), $env:SCOOP, $env:SCOOP_GLOBAL)) {
        if (-not [string]::IsNullOrWhiteSpace($candidate)) {
            $roots += (ConvertTo-WindowsComparablePath -Path $candidate)
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($env:ProgramData)) {
        $roots += (Join-WindowsPathText -Left $env:ProgramData -Right 'scoop')
    }
    return @($roots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)
}

function Get-ScoopShimMetadataPath {
    param([string]$Source)
    $directory = Get-WindowsPathDirectoryText -Path $Source
    $baseName = Get-WindowsPathFileBaseText -Path $Source
    if ([string]::IsNullOrWhiteSpace($directory) -or [string]::IsNullOrWhiteSpace($baseName)) { return '' }
    return (Join-WindowsPathText -Left $directory -Right "$baseName.shim")
}

function Test-ScoopShimSourceUnderKnownRoot {
    param([string]$Source)
    foreach ($root in (Get-ScoopRootCandidates)) {
        $shimDir = Join-WindowsPathText -Left $root -Right 'shims'
        if (Test-WindowsPathUnderDirectoryText -Path $Source -Directory $shimDir) {
            return $true
        }
    }
    return $false
}

function Get-ScoopRootFromShimSource {
    param([string]$Source)
    $directory = Get-WindowsPathDirectoryText -Path $Source
    if ([string]::IsNullOrWhiteSpace($directory)) { return '' }
    $leaf = Get-WindowsPathFileBaseText -Path $directory
    if (-not $leaf.Equals('shims', [StringComparison]::OrdinalIgnoreCase)) { return '' }
    return (Get-WindowsPathDirectoryText -Path $directory)
}

function Get-ScoopShimTargetFromContent {
    param([string[]]$Content)
    foreach ($line in @($Content)) {
        if ([string]$line -match '^\s*path\s*=\s*(.+?)\s*$') {
            $value = $Matches[1].Trim()
            if (($value.StartsWith('"') -and $value.EndsWith('"')) -or
                ($value.StartsWith("'") -and $value.EndsWith("'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            return $value
        }
    }
    return ''
}

function Get-ScoopPackageNameFromAppsPathText {
    param([string]$Path)
    $normalized = ConvertTo-WindowsComparablePath -Path $Path
    $match = [regex]::Match($normalized, '(?i)(^|\\)apps\\([^\\]+)(\\|$)')
    if (-not $match.Success) { return '' }
    return $match.Groups[2].Value
}

function Get-ScoopPackageNameFromKnownAppsPath {
    param([string]$Path, [string[]]$ExtraRoots = @())
    $roots = @()
    foreach ($candidate in @($ExtraRoots + (Get-ScoopRootCandidates))) {
        if (-not [string]::IsNullOrWhiteSpace($candidate)) {
            $roots += (ConvertTo-WindowsComparablePath -Path $candidate)
        }
    }
    foreach ($root in @($roots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)) {
        $appsDir = Join-WindowsPathText -Left $root -Right 'apps'
        if (Test-WindowsPathUnderDirectoryText -Path $Path -Directory $appsDir) {
            return (Get-ScoopPackageNameFromAppsPathText -Path $Path)
        }
    }
    return ''
}

function Get-ScoopShimPackageState {
    param([string]$tool)
    $source = Get-CatalogToolCommandSource -tool $tool
    if ([string]::IsNullOrWhiteSpace($source)) {
        return [pscustomobject]@{ Status = 'none'; Source = ''; Shim = ''; Package = ''; Reason = '' }
    }
    $normalizedSource = ConvertTo-WindowsComparablePath -Path $source
    if ($normalizedSource -notmatch '(?i)\\shims\\[^\\]+$') {
        return [pscustomobject]@{ Status = 'none'; Source = $source; Shim = ''; Package = ''; Reason = '' }
    }

    $shimPath = Get-ScoopShimMetadataPath -Source $normalizedSource
    $hasShim = -not [string]::IsNullOrWhiteSpace($shimPath) -and (Test-Path -LiteralPath $shimPath -PathType Leaf)
    if (-not $hasShim) {
        if (Test-ScoopShimSourceUnderKnownRoot -Source $normalizedSource) {
            return [pscustomobject]@{
                Status = 'error'; Source = $source; Shim = $shimPath; Package = '';
                Reason = 'Scoop shim metadata file is missing'
            }
        }
        return [pscustomobject]@{ Status = 'none'; Source = $source; Shim = $shimPath; Package = ''; Reason = '' }
    }

    try {
        $target = Get-ScoopShimTargetFromContent -Content @(Get-Content -LiteralPath $shimPath -ErrorAction Stop)
    } catch {
        return [pscustomobject]@{
            Status = 'error'; Source = $source; Shim = $shimPath; Package = '';
            Reason = 'Scoop shim metadata file could not be read'
        }
    }
    if ([string]::IsNullOrWhiteSpace($target)) {
        return [pscustomobject]@{
            Status = 'error'; Source = $source; Shim = $shimPath; Package = '';
            Reason = 'Scoop shim metadata has no path entry'
        }
    }

    $sourceRoot = Get-ScoopRootFromShimSource -Source $normalizedSource
    $package = Get-ScoopPackageNameFromKnownAppsPath -Path $target -ExtraRoots @($sourceRoot)
    if ([string]::IsNullOrWhiteSpace($package)) {
        return [pscustomobject]@{
            Status = 'error'; Source = $source; Shim = $shimPath; Package = '';
            Reason = ("Scoop shim target is outside the apps tree: {0}" -f $target)
        }
    }
    return [pscustomobject]@{ Status = 'found'; Source = $source; Shim = $shimPath; Package = $package; Reason = '' }
}

function Get-ScoopPackageListName {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return '' }
    return @($Package -split '/')[-1]
}

function Test-ScoopPackageManagedByList {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return $false }
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) { return $false }
    $listName = Get-ScoopPackageListName -Package $Package
    try {
        $output = @(scoop list $listName 2>$null)
        if ($LASTEXITCODE -ne 0) { return $false }
        foreach ($line in $output) {
            if ([string]$line -match "(^|\s)$([regex]::Escape($listName))(\s|$)") {
                return $true
            }
        }
    } catch {
        return $false
    }
    return $false
}

function Get-ScoopPackageOwnershipState {
    param([string]$tool, [string]$Package)
    $listName = Get-ScoopPackageListName -Package $Package
    if ([string]::IsNullOrWhiteSpace($listName)) {
        return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = ''; Package = ''; Expected = $listName }
    }

    $shimState = Get-ScoopShimPackageState -tool $tool
    if ($shimState.Status -eq 'found') {
        if ($shimState.Package.Equals($listName, [StringComparison]::OrdinalIgnoreCase)) {
            return [pscustomobject]@{ Status = 'managed'; Reason = ''; Source = $shimState.Source; Package = $shimState.Package; Expected = $listName }
        }
        return [pscustomobject]@{
            Status = 'error'; Source = $shimState.Source; Package = $shimState.Package; Expected = $listName;
            Reason = ("Scoop shim points to package {0}, expected {1}" -f $shimState.Package, $listName)
        }
    }
    if ($shimState.Status -eq 'error') {
        return [pscustomobject]@{
            Status = 'error'; Source = $shimState.Source; Package = $shimState.Package; Expected = $listName;
            Reason = $shimState.Reason
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($shimState.Source)) {
        $sourcePackage = Get-ScoopPackageNameFromKnownAppsPath -Path $shimState.Source
        if (-not [string]::IsNullOrWhiteSpace($sourcePackage)) {
            if ($sourcePackage.Equals($listName, [StringComparison]::OrdinalIgnoreCase)) {
                return [pscustomobject]@{ Status = 'managed'; Reason = ''; Source = $shimState.Source; Package = $sourcePackage; Expected = $listName }
            }
            return [pscustomobject]@{
                Status = 'error'; Source = $shimState.Source; Package = $sourcePackage; Expected = $listName;
                Reason = ("Scoop app source points to package {0}, expected {1}" -f $sourcePackage, $listName)
            }
        }
        return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = $shimState.Source; Package = ''; Expected = $listName }
    }

    if (Test-ScoopPackageManagedByList -Package $Package) {
        return [pscustomobject]@{ Status = 'managed'; Reason = ''; Source = ''; Package = $listName; Expected = $listName }
    }
    return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = $shimState.Source; Package = ''; Expected = $listName }
}

function Test-ScoopPackageManaged {
    param([string]$Package, [string]$Tool = '')
    if ([string]::IsNullOrWhiteSpace($Tool)) {
        return (Test-ScoopPackageManagedByList -Package $Package)
    }
    $state = Get-ScoopPackageOwnershipState -tool $Tool -Package $Package
    return ($state.Status -eq 'managed')
}

function Test-ScoopStatusOutputContainsPackage {
    param([object[]]$Output, [string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return $false }
    $listName = Get-ScoopPackageListName -Package $Package
    $escaped = [regex]::Escape($listName)
    foreach ($line in @($Output)) {
        if ([string]$line -match "(^|\s)$escaped(\s|$)") {
            return $true
        }
    }
    return $false
}

function Get-ScoopPackageUpgradeState {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) {
        return [pscustomobject]@{ Status = 'none'; ExitCode = 0 }
    }
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) {
        return [pscustomobject]@{ Status = 'error'; ExitCode = 'scoop-missing' }
    }
    try {
        $output = @(scoop status 2>$null)
        $exitCode = $LASTEXITCODE
        if ($exitCode -ne 0) {
            return [pscustomobject]@{ Status = 'error'; ExitCode = $exitCode }
        }
        if (Test-ScoopStatusOutputContainsPackage -Output $output -Package $Package) {
            return [pscustomobject]@{ Status = 'available'; ExitCode = 0 }
        }
        return [pscustomobject]@{ Status = 'none'; ExitCode = 0 }
    } catch {
        return [pscustomobject]@{ Status = 'error'; ExitCode = 'exception' }
    }
}

function Get-WingetRootCandidates {
    $roots = @()
    foreach ($base in @($env:LOCALAPPDATA)) {
        if (-not [string]::IsNullOrWhiteSpace($base)) {
            $roots += (Join-WindowsPathText -Left $base -Right 'Microsoft\WinGet\Links')
            $roots += (Join-WindowsPathText -Left $base -Right 'Microsoft\WinGet\Packages')
            $roots += (Join-WindowsPathText -Left $base -Right 'Microsoft\WindowsApps')
        }
    }
    foreach ($base in @($env:ProgramFiles, ${env:ProgramFiles(x86)})) {
        if (-not [string]::IsNullOrWhiteSpace($base)) {
            $roots += (Join-WindowsPathText -Left $base -Right 'WinGet\Links')
            $roots += (Join-WindowsPathText -Left $base -Right 'WinGet\Packages')
            $roots += (Join-WindowsPathText -Left $base -Right 'WindowsApps')
        }
    }
    $roots += @(
        'C:\Program Files\WinGet\Links',
        'C:\Program Files\WinGet\Packages',
        'C:\Program Files (x86)\WinGet\Links',
        'C:\Program Files (x86)\WinGet\Packages',
        'C:\Program Files\WindowsApps'
    )
    return @($roots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)
}

function Get-WingetProgramRootCandidates {
    $roots = @()
    foreach ($candidate in @($env:ProgramFiles, ${env:ProgramFiles(x86)}, 'C:\Program Files', 'C:\Program Files (x86)')) {
        if (-not [string]::IsNullOrWhiteSpace($candidate)) { $roots += (ConvertTo-WindowsComparablePath -Path $candidate) }
    }
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $roots += (Join-WindowsPathText -Left $env:LOCALAPPDATA -Right 'Programs')
    }
    return @($roots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)
}

function Get-WingetToolInstallSuffixes {
    param([string]$tool)
    switch ($tool) {
        'git' { return @('Git\cmd', 'Git\bin') }
        'nvim' { return @('Neovim\bin') }
        'starship' { return @('Starship\bin', 'starship\bin', 'starship') }
        'rg' { return @('ripgrep', 'ripgrep\bin') }
        'fd' { return @('fd', 'fd\bin') }
        'fzf' { return @('fzf', 'fzf\bin') }
        'lsd' { return @('lsd', 'lsd\bin') }
        'chezmoi' { return @('chezmoi', 'chezmoi\bin') }
        'lazygit' { return @('lazygit', 'lazygit\bin') }
        'wt' { return @('WindowsApps') }
        'make' { return @('GnuWin32\bin') }
        'cmake' { return @('CMake\bin') }
        'pwsh' { return @('PowerShell\7') }
        'node' { return @('nodejs') }
        'python' { return @('Python\Python312', 'Python312') }
        'zig' { return @('zig', 'Zig') }
        'jq' { return @('jq', 'jq\bin') }
        'shellcheck' { return @('ShellCheck', 'ShellCheck\bin') }
        'hyperfine' { return @('hyperfine', 'hyperfine\bin') }
        'code' { return @('Microsoft VS Code\bin') }
        default { return @() }
    }
}

function Test-WingetToolSourceMatchesPackage {
    param([string]$tool, [string]$Package, [string]$Source)
    if ([string]::IsNullOrWhiteSpace($tool) -or [string]::IsNullOrWhiteSpace($Package) -or [string]::IsNullOrWhiteSpace($Source)) {
        return $false
    }
    $expected = Get-CatalogPackageId -tool $tool -Manager 'winget'
    if ([string]::IsNullOrWhiteSpace($expected) -or -not $expected.Equals($Package, [StringComparison]::OrdinalIgnoreCase)) {
        return $false
    }

    foreach ($root in (Get-WingetRootCandidates)) {
        if (Test-WindowsPathUnderDirectoryText -Path $Source -Directory $root) {
            return $true
        }
    }

    foreach ($root in (Get-WingetProgramRootCandidates)) {
        foreach ($suffix in (Get-WingetToolInstallSuffixes -tool $tool)) {
            $candidate = Join-WindowsPathText -Left $root -Right $suffix
            if (Test-WindowsPathUnderDirectoryText -Path $Source -Directory $candidate) {
                return $true
            }
        }
    }
    return $false
}

function Get-WingetPackageOwnershipState {
    param([string]$tool, [string]$Package)
    $source = Get-CatalogToolCommandSource -tool $tool
    if ([string]::IsNullOrWhiteSpace($source)) {
        return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = ''; Package = ''; Expected = $Package }
    }
    if (-not (Test-WingetPackageManaged -Package $Package)) {
        return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = $source; Package = ''; Expected = $Package }
    }
    if (Test-WingetToolSourceMatchesPackage -tool $tool -Package $Package -Source $source) {
        return [pscustomobject]@{ Status = 'managed'; Reason = ''; Source = $source; Package = $Package; Expected = $Package }
    }
    return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = $source; Package = ''; Expected = $Package }
}

function Test-WingetPackageManaged {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return $false }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { return $false }
    try {
        $output = @(winget list --id $Package -e --accept-source-agreements 2>$null)
        if ($LASTEXITCODE -ne 0) { return $false }
        return (Test-WingetOutputContainsPackageId -Output $output -Package $Package)
    } catch {
        return $false
    }
    return $false
}

function Test-WingetOutputContainsPackageId {
    param([object[]]$Output, [string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return $false }
    $escaped = [regex]::Escape($Package)
    foreach ($line in @($Output)) {
        if ([string]$line -match "(^|\s)$escaped(\s|$)") {
            return $true
        }
    }
    return $false
}

function Test-WingetNoApplicableUpgradeExitCode {
    param([object]$ExitCode)
    $code = [string]$ExitCode
    return ($code -in @('-1978335189', '-1978335153'))
}

function Get-WingetPackageUpgradeState {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) {
        return [pscustomobject]@{ Status = 'none'; ExitCode = 0 }
    }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        return [pscustomobject]@{ Status = 'error'; ExitCode = 'winget-missing' }
    }
    try {
        $output = @(winget list --upgrade-available --id $Package -e --accept-source-agreements 2>$null)
        $exitCode = $LASTEXITCODE
        if ($exitCode -ne 0) {
            if (Test-WingetNoApplicableUpgradeExitCode -ExitCode $exitCode) {
                return [pscustomobject]@{ Status = 'none'; ExitCode = $exitCode }
            }
            return [pscustomobject]@{ Status = 'error'; ExitCode = $exitCode }
        }
        if (Test-WingetOutputContainsPackageId -Output $output -Package $Package) {
            return [pscustomobject]@{ Status = 'available'; ExitCode = 0 }
        }
        return [pscustomobject]@{ Status = 'none'; ExitCode = 0 }
    } catch {
        return [pscustomobject]@{ Status = 'error'; ExitCode = 'exception' }
    }
}

function Test-ChocoPackageManaged {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return $false }
    if (-not (Get-Command choco -ErrorAction SilentlyContinue)) { return $false }
    try {
        $output = @(choco list $Package --local-only --exact --limit-output 2>$null)
        if ($LASTEXITCODE -ne 0) { return $false }
        $prefix = "$Package|"
        foreach ($line in $output) {
            if (([string]$line).StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
                return $true
            }
        }
    } catch {
        return $false
    }
    return $false
}

function Test-ChocoOutdatedOutputContainsPackage {
    param([object[]]$Output, [string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) { return $false }
    $prefix = "$Package|"
    foreach ($line in @($Output)) {
        if (([string]$line).StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }
    return $false
}

function Get-ChocoPackageUpgradeState {
    param([string]$Package)
    if ([string]::IsNullOrWhiteSpace($Package)) {
        return [pscustomobject]@{ Status = 'none'; ExitCode = 0 }
    }
    if (-not (Get-Command choco -ErrorAction SilentlyContinue)) {
        return [pscustomobject]@{ Status = 'error'; ExitCode = 'choco-missing' }
    }
    try {
        $output = @(choco outdated --limit-output 2>$null)
        $exitCode = $LASTEXITCODE
        if (($exitCode -ne 0) -and ($exitCode -ne 2)) {
            return [pscustomobject]@{ Status = 'error'; ExitCode = $exitCode }
        }
        if (Test-ChocoOutdatedOutputContainsPackage -Output $output -Package $Package) {
            return [pscustomobject]@{ Status = 'available'; ExitCode = $exitCode }
        }
        return [pscustomobject]@{ Status = 'none'; ExitCode = $exitCode }
    } catch {
        return [pscustomobject]@{ Status = 'error'; ExitCode = 'exception' }
    }
}

function Get-ChocolateyRootCandidates {
    $roots = @()
    if (-not [string]::IsNullOrWhiteSpace($env:ChocolateyInstall)) {
        $roots += (ConvertTo-WindowsComparablePath -Path $env:ChocolateyInstall)
    }
    if (-not [string]::IsNullOrWhiteSpace($env:ProgramData)) {
        $roots += (Join-WindowsPathText -Left $env:ProgramData -Right 'chocolatey')
    }
    $roots += 'C:\ProgramData\chocolatey'
    return @($roots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)
}

function Test-ChocolateyToolSourceUnderKnownRoot {
    param([string]$Source)
    foreach ($root in (Get-ChocolateyRootCandidates)) {
        $binDir = Join-WindowsPathText -Left $root -Right 'bin'
        if (Test-WindowsPathUnderDirectoryText -Path $Source -Directory $binDir) {
            return $true
        }
    }
    return $false
}

function Get-ChocoPackageOwnershipState {
    param([string]$tool, [string]$Package)
    $source = Get-CatalogToolCommandSource -tool $tool
    if ([string]::IsNullOrWhiteSpace($source)) {
        return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = ''; Package = ''; Expected = $Package }
    }
    $sourceUnderChoco = Test-ChocolateyToolSourceUnderKnownRoot -Source $source
    $packageInstalled = Test-ChocoPackageManaged -Package $Package
    if ($sourceUnderChoco -and $packageInstalled) {
        return [pscustomobject]@{ Status = 'managed'; Reason = ''; Source = $source; Package = $Package; Expected = $Package }
    }
    if ($sourceUnderChoco -and -not $packageInstalled) {
        return [pscustomobject]@{
            Status = 'error'; Source = $source; Package = ''; Expected = $Package;
            Reason = ("Chocolatey command source is under Chocolatey bin but package {0} is not installed" -f $Package)
        }
    }
    return [pscustomobject]@{ Status = 'not-managed'; Reason = ''; Source = $source; Package = ''; Expected = $Package }
}

function Get-ManagedCatalogToolUpdateTarget {
    param([string]$tool)
    foreach ($manager in @('scoop', 'winget', 'choco')) {
        $pkg = Get-CatalogPackageId -tool $tool -Manager $manager
        if ([string]::IsNullOrWhiteSpace($pkg)) { continue }
        if (-not (Get-Command $manager -ErrorAction SilentlyContinue)) { continue }
        $state = switch ($manager) {
            'scoop' { Get-ScoopPackageOwnershipState -tool $tool -Package $pkg }
            'winget' { Get-WingetPackageOwnershipState -tool $tool -Package $pkg }
            'choco' { Get-ChocoPackageOwnershipState -tool $tool -Package $pkg }
        }
        if ($state.Status -eq 'error') {
            return [pscustomobject]@{ Pm = $manager; Pkg = $pkg; Status = 'error'; Reason = $state.Reason; Source = $state.Source }
        }
        if ($state.Status -eq 'managed') {
            return [pscustomobject]@{ Pm = $manager; Pkg = $pkg; Status = 'managed'; Reason = ''; Source = $state.Source }
        }
    }
    return $null
}

function Add-UnmanagedDependency {
    param([string]$tool, [string]$Source)
    if ($null -eq $script:UnmanagedDependencies) {
        $script:UnmanagedDependencies = @()
    }
    foreach ($existing in @($script:UnmanagedDependencies)) {
        if ($existing.Tool -eq $tool) { return }
    }
    $script:UnmanagedDependencies += [pscustomobject]@{ Tool = $tool; Source = $Source }
}

function Report-UnmanagedCatalogTool {
    param([string]$tool, [switch]$ReportSkip)
    $source = Get-CatalogToolCommandSource -tool $tool
    if ([string]::IsNullOrWhiteSpace($source)) { $source = 'unknown source' }
    Add-UnmanagedDependency -tool $tool -Source $source
    if ($ReportSkip) {
        Write-Host ("  unmanaged {0,-26} source={1}" -f $tool, $source)
    }
}

function Report-BlockedCatalogToolUpdate {
    param([string]$tool, [object]$Target)
    $reason = if ($Target -and -not [string]::IsNullOrWhiteSpace($Target.Reason)) { $Target.Reason } else { 'manager provenance could not be verified' }
    $source = if ($Target -and -not [string]::IsNullOrWhiteSpace($Target.Source)) { $Target.Source } else { Get-CatalogToolCommandSource -tool $tool }
    Write-Warning ("  blocked   {0,-26} {1}; source={2}" -f $tool, $reason, $source)
    $exitCode = if ($Target.Pm -eq 'scoop') { 'scoop-shim-provenance' } else { 'manager-provenance' }
    $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm=$Target.Pm; Pkg=$Target.Pkg; ExitCode=$exitCode }
}

function Update-ScoopTool {
    param(
        [string]$tool,
        [switch]$NoPrompt,
        [switch]$SkipManifestRefresh,
        [switch]$ReportSkip,
        [switch]$AssumePresent,
        [switch]$AssumeManaged,
        [bool]$IsDryRun = $DryRun
    )
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) { return }
    # -AssumePresent: the caller already determined presence via its own tester,
    # so skip the redundant Test-Tool recheck. Without it the two checks can
    # disagree in tests with mocked presence.
    if ((-not $AssumePresent) -and (-not (Test-Tool $tool))) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} not installed" -f $tool) }
        return
    }
    $pkg = Get-CatalogPackageId -tool $tool -Manager 'scoop'
    if (-not $pkg) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} no scoop package in catalog" -f $tool) }
        return
    }
    if ((-not $AssumeManaged) -and (-not (Test-ScoopPackageManaged -Package $pkg))) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} present, but Scoop does not manage {1}" -f $tool, $pkg) }
        return
    }
    if ((-not $SkipManifestRefresh) -and (-not $IsDryRun)) {
        scoop update | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Warning ("  scoop manifest refresh failed (exit {0})" -f $LASTEXITCODE)
            $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='scoop'; Pkg='manifest'; ExitCode=$LASTEXITCODE }
            return
        }
    }
    $upgradeState = Get-ScoopPackageUpgradeState -Package $pkg
    if ($upgradeState.Status -eq 'none') {
        if ($ReportSkip) { Write-Host ("  current   {0,-26} via scoop" -f $tool) }
        return
    }
    if ($upgradeState.Status -ne 'available') {
        Write-Warning ("  scoop status check of {0} failed (exit {1})" -f $pkg, $upgradeState.ExitCode)
        $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='scoop'; Pkg=$pkg; ExitCode=$upgradeState.ExitCode }
        return
    }
    if ((-not $NoPrompt) -and (-not (Ask "Update ${tool} to the latest scoop version?"))) { return }
    if ($IsDryRun) {
        if ($SkipManifestRefresh) {
            Write-Host ("  would:    scoop update {0}" -f $pkg)
        } else {
            Write-Host ("  would:    scoop update; scoop update {0}" -f $pkg)
        }
        return
    }
    scoop update $pkg
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("  updated   {0,-26} via scoop" -f $tool)
    } else {
        Write-Warning ("  scoop update of {0} failed (exit {1})" -f $pkg, $LASTEXITCODE)
        $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='scoop'; Pkg=$pkg; ExitCode=$LASTEXITCODE }
    }
}

function Update-WingetTool {
    param(
        [string]$tool,
        [switch]$NoPrompt,
        [switch]$ReportSkip,
        [switch]$AssumePresent,
        [switch]$AssumeManaged,
        [bool]$IsDryRun = $DryRun
    )
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { return }
    if ((-not $AssumePresent) -and (-not (Test-Tool $tool))) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} not installed" -f $tool) }
        return
    }
    $pkg = Get-CatalogPackageId -tool $tool -Manager 'winget'
    if (-not $pkg) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} no winget package in catalog" -f $tool) }
        return
    }
    if (-not $AssumeManaged) {
        $state = Get-WingetPackageOwnershipState -tool $tool -Package $pkg
        if ($state.Status -eq 'error') {
            Report-BlockedCatalogToolUpdate -tool $tool -Target ([pscustomobject]@{ Pm='winget'; Pkg=$pkg; Reason=$state.Reason; Source=$state.Source })
            return
        }
        if ($state.Status -ne 'managed') {
            if ($ReportSkip) { Write-Host ("  unmanaged {0,-26} source={1}" -f $tool, $(if ($state.Source) { $state.Source } else { 'unknown source' })) }
            Add-UnmanagedDependency -tool $tool -Source $state.Source
            return
        }
    }
    $upgradeState = Get-WingetPackageUpgradeState -Package $pkg
    if ($upgradeState.Status -eq 'none') {
        if ($ReportSkip) { Write-Host ("  current   {0,-26} via winget" -f $tool) }
        return
    }
    if ($upgradeState.Status -ne 'available') {
        Write-Warning ("  winget upgrade availability check of {0} failed (exit {1})" -f $pkg, $upgradeState.ExitCode)
        $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='winget'; Pkg=$pkg; ExitCode=$upgradeState.ExitCode }
        return
    }
    if ((-not $NoPrompt) -and (-not (Ask "Update ${tool} to the latest winget version?"))) { return }
    if ($IsDryRun) {
        Write-Host ("  would:    winget upgrade --id {0} -e --accept-source-agreements --accept-package-agreements --silent" -f $pkg)
        return
    }
    winget upgrade --id $pkg -e --accept-source-agreements --accept-package-agreements --silent
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("  updated   {0,-26} via winget" -f $tool)
    } elseif (Test-WingetNoApplicableUpgradeExitCode -ExitCode $LASTEXITCODE) {
        Write-Host ("  current   {0,-26} via winget" -f $tool)
    } else {
        Write-Warning ("  winget upgrade of {0} failed (exit {1})" -f $pkg, $LASTEXITCODE)
        $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='winget'; Pkg=$pkg; ExitCode=$LASTEXITCODE }
    }
}

function Update-ChocoTool {
    param(
        [string]$tool,
        [switch]$NoPrompt,
        [switch]$ReportSkip,
        [switch]$AssumePresent,
        [switch]$AssumeManaged,
        [bool]$IsDryRun = $DryRun
    )
    if (-not (Get-Command choco -ErrorAction SilentlyContinue)) { return }
    if ((-not $AssumePresent) -and (-not (Test-Tool $tool))) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} not installed" -f $tool) }
        return
    }
    $pkg = Get-CatalogPackageId -tool $tool -Manager 'choco'
    if (-not $pkg) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} no choco package in catalog" -f $tool) }
        return
    }
    if (-not $AssumeManaged) {
        $state = Get-ChocoPackageOwnershipState -tool $tool -Package $pkg
        if ($state.Status -eq 'error') {
            Report-BlockedCatalogToolUpdate -tool $tool -Target ([pscustomobject]@{ Pm='choco'; Pkg=$pkg; Reason=$state.Reason; Source=$state.Source })
            return
        }
        if ($state.Status -ne 'managed') {
            if ($ReportSkip) { Write-Host ("  unmanaged {0,-26} source={1}" -f $tool, $(if ($state.Source) { $state.Source } else { 'unknown source' })) }
            Add-UnmanagedDependency -tool $tool -Source $state.Source
            return
        }
    }
    $upgradeState = Get-ChocoPackageUpgradeState -Package $pkg
    if ($upgradeState.Status -eq 'none') {
        if ($ReportSkip) { Write-Host ("  current   {0,-26} via choco" -f $tool) }
        return
    }
    if ($upgradeState.Status -ne 'available') {
        Write-Warning ("  choco outdated check of {0} failed (exit {1})" -f $pkg, $upgradeState.ExitCode)
        $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='choco'; Pkg=$pkg; ExitCode=$upgradeState.ExitCode }
        return
    }
    if ((-not $NoPrompt) -and (-not (Ask "Update ${tool} to the latest Chocolatey version?"))) { return }
    if ($IsDryRun) {
        Write-Host ("  would:    choco upgrade {0} -y" -f $pkg)
        return
    }
    choco upgrade $pkg -y
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("  updated   {0,-26} via choco" -f $tool)
    } else {
        Write-Warning ("  choco upgrade of {0} failed (exit {1})" -f $pkg, $LASTEXITCODE)
        $script:InstallFailures += [pscustomobject]@{ Tool=$tool; Pm='choco'; Pkg=$pkg; ExitCode=$LASTEXITCODE }
    }
}

function Update-ManagedCatalogTool {
    param(
        [string]$tool,
        [switch]$NoPrompt,
        [switch]$SkipScoopManifestRefresh,
        [switch]$ReportSkip,
        [switch]$AssumePresent,
        [bool]$IsDryRun = $DryRun
    )
    if ((-not $AssumePresent) -and (-not (Test-Tool $tool))) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} not installed" -f $tool) }
        return
    }
    if (-not $Catalog.ContainsKey($tool)) {
        if ($ReportSkip) { Write-Host ("  skipped   {0,-26} no catalog entry" -f $tool) }
        return
    }

    $target = Get-ManagedCatalogToolUpdateTarget -tool $tool
    if (-not $target) {
        Report-UnmanagedCatalogTool -tool $tool -ReportSkip:$ReportSkip
        return
    }
    if ($target.Status -eq 'error') {
        Report-BlockedCatalogToolUpdate -tool $tool -Target $target
        return
    }

    switch ($target.Pm) {
        'scoop' {
            Update-ScoopTool -tool $tool -NoPrompt:$NoPrompt -SkipManifestRefresh:$SkipScoopManifestRefresh -ReportSkip:$ReportSkip -AssumePresent -AssumeManaged -IsDryRun $IsDryRun
        }
        'winget' {
            Update-WingetTool -tool $tool -NoPrompt:$NoPrompt -ReportSkip:$ReportSkip -AssumePresent -AssumeManaged -IsDryRun $IsDryRun
        }
        'choco' {
            Update-ChocoTool -tool $tool -NoPrompt:$NoPrompt -ReportSkip:$ReportSkip -AssumePresent -AssumeManaged -IsDryRun $IsDryRun
        }
    }
}

function Write-UnmanagedDependencySummary {
    if (($null -eq $script:UnmanagedDependencies) -or ($script:UnmanagedDependencies.Count -eq 0)) { return }
    Write-Host ""
    Write-Host "install-deps: $($script:UnmanagedDependencies.Count) present tool(s) were not updated because supported managers do not own their command source."
    Write-Host "Install or migrate those tools through a supported package manager for setup.ps1 -Update to own their updates."
}

# Track failures across the run so we can warn loudly at the end instead of
# pretending success.
$script:InstallFailures = @()
$script:UnmanagedDependencies = @()

# ---- Hack Nerd Font: prefer scoop bucket, fall back to direct download+register
function Get-HackNerdFontInstallScope {
    $userFonts = "$env:LOCALAPPDATA\Microsoft\Windows\Fonts"
    $sysFonts = "$env:WINDIR\Fonts"
    if ((Test-Path $userFonts -PathType Container) -and
        (Get-ChildItem -Path $userFonts -Filter "Hack*Nerd*" -ErrorAction SilentlyContinue)) {
        return 'user'
    }
    if ((Test-Path $sysFonts -PathType Container) -and
        (Get-ChildItem -Path $sysFonts -Filter "Hack*Nerd*" -ErrorAction SilentlyContinue)) {
        return 'system'
    }
    return ''
}

function Test-HackNerdFontInstalled {
    return -not [string]::IsNullOrEmpty((Get-HackNerdFontInstallScope))
}

function Send-FontChangeNotification {
    try {
        $typeName = 'DotfilesFontChange.NativeMethods'
        if (-not ([System.Management.Automation.PSTypeName]$typeName).Type) {
            Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

namespace DotfilesFontChange {
    public static class NativeMethods {
        [DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
        public static extern IntPtr SendMessageTimeout(
            IntPtr hWnd,
            uint Msg,
            UIntPtr wParam,
            IntPtr lParam,
            uint fuFlags,
            uint uTimeout,
            out UIntPtr lpdwResult);
    }
}
"@
        }
        $result = [UIntPtr]::Zero
        [DotfilesFontChange.NativeMethods]::SendMessageTimeout(
            [IntPtr]0xffff,
            0x001D,
            [UIntPtr]::Zero,
            [IntPtr]::Zero,
            0x0002,
            1000,
            [ref]$result) | Out-Null
        Write-Host "             notified Windows that fonts changed"
    } catch {
        Write-Warning ("Could not broadcast WM_FONTCHANGE: " + $_.Exception.Message)
    }
}

function Install-HackNerdFont {
    # Already installed?
    $fontScope = Get-HackNerdFontInstallScope
    if ($fontScope) {
        Write-Host ("  ok        {0,-26} already installed ({1})" -f "Hack Nerd Font", $fontScope)
        return
    }
    if (-not (Ask "Install Hack Nerd Font?")) {
        Write-Host ("  skipped   {0,-26}" -f "Hack Nerd Font")
        return
    }

    # Path 1: scoop with the nerd-fonts bucket -- proper user-scope install.
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        if ($DryRun) {
            Write-Host "  would: scoop install nerd-fonts/Hack-NF"
            return
        }
        Add-ScoopBucketSafe -Name 'nerd-fonts' | Out-Null
        scoop install nerd-fonts/Hack-NF
        if ($LASTEXITCODE -eq 0) {
            Write-Host ("  installed {0,-26} via scoop" -f "Hack Nerd Font")
            Send-FontChangeNotification
            return
        }
        Write-Warning "scoop install failed; falling back to direct download."
    }

    # Path 2: download Hack.zip and register fonts user-scope. No admin needed.
    if ($DryRun) {
        Write-Host "  would: download nerd-fonts/$HackNerdFontVersion/Hack.zip, verify sha256, extract, register in HKCU\\Fonts"
        return
    }
    $tmp = $null
    try {
        $tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "hack-nf-$([guid]::NewGuid())")
        $zip = Join-Path $tmp.FullName "Hack.zip"
        Invoke-WebRequest -Uri "https://github.com/ryanoasis/nerd-fonts/releases/download/$HackNerdFontVersion/Hack.zip" -OutFile $zip -UseBasicParsing -ErrorAction Stop
        if (-not (Test-FileSha256 -Path $zip -Expected $HackNerdFontSha256)) {
            Write-Host "  FAIL: checksum mismatch for Hack.zip" -ForegroundColor Red
            $script:InstallFailures += [pscustomobject]@{ Tool = 'Hack Nerd Font'; Pm = 'direct'; Pkg = 'Hack.zip'; ExitCode = 'sha256' }
            return
        }
        Expand-Archive -Path $zip -DestinationPath $tmp.FullName -Force

        $fontDest = "$env:LOCALAPPDATA\Microsoft\Windows\Fonts"
        if (-not (Test-Path $fontDest)) {
            New-Item -ItemType Directory -Force -Path $fontDest | Out-Null
        }
        $regPath = "HKCU:\Software\Microsoft\Windows NT\CurrentVersion\Fonts"
        if (-not (Test-Path $regPath)) {
            New-Item -Path $regPath -Force | Out-Null
        }
        $installedCount = 0
        Get-ChildItem -Path $tmp.FullName -Recurse -Include *.ttf,*.otf | ForEach-Object {
            $destPath = Join-Path $fontDest $_.Name
            Copy-Item -LiteralPath $_.FullName -Destination $destPath -Force
            New-ItemProperty -Path $regPath -Name "$($_.BaseName) (TrueType)" `
                -Value $destPath -PropertyType String -Force | Out-Null
            $installedCount++
        }
        Write-Host ("  installed {0,-26} {1} font files registered in HKCU" -f "Hack Nerd Font", $installedCount)
        Send-FontChangeNotification
        Write-Host "             (you may need to restart your terminal to see them)"
    } catch {
        Write-Warning ("Hack Nerd Font install failed: " + $_.Exception.Message)
        Write-Host  "  manual    download Hack.zip from nerd-fonts releases and install via the Fonts control panel."
    } finally {
        if ($tmp) {
            Remove-Item -Recurse -Force $tmp.FullName -ErrorAction SilentlyContinue
        }
    }
}

# ---- VS Code Rose Pine theme -------------------------------------------------
# Set Rose Pine plus Hack Nerd Font settings in VS Code user settings.
# The theme label has an accented e. Build it with [char]0xE9 at runtime so
# this file stays pure ASCII for Windows PowerShell 5.1.
function Get-VSCodeSettingsSpec {
    $theme = "Ros$([char]0xE9) Pine"        # dark label "Rose Pine" (accented e)
    $font = "'Hack Nerd Font', Consolas, monospace"
    # Theme resolution precedence is the whole reason a pre-set colorTheme can
    # appear ignored: when window.autoDetectColorScheme is true (Settings Sync, an
    # imported profile, or a future default can enable it), VS Code IGNORES
    # workbench.colorTheme and resolves the active theme from
    # workbench.preferredDark/LightColorTheme -- which, unset, hold the built-in
    # defaults (Dark Modern / "Dark 2026"). This repo FORCES dark Rose Pine, not
    # the adaptive dark/light split (same rule as Ghostty -- see tests/MANUAL.md),
    # so: pin autoDetectColorScheme off (a real JSON boolean, not a string) to make
    # colorTheme authoritative, AND point BOTH preferred slots at the SAME dark
    # Rose Pine so no OS-scheme / autoDetect combination can ever yield a light
    # theme. startupEditor=none opens straight to an empty workbench (no noisy
    # Welcome tab) so the pre-set theme is the only thing on screen.
    return @(
        [pscustomobject]@{ Key = 'workbench.colorTheme'; Value = $theme },
        [pscustomobject]@{ Key = 'workbench.preferredDarkColorTheme'; Value = $theme },
        [pscustomobject]@{ Key = 'workbench.preferredLightColorTheme'; Value = $theme },
        [pscustomobject]@{ Key = 'window.autoDetectColorScheme'; Value = $false; Raw = $true },
        [pscustomobject]@{ Key = 'editor.fontFamily'; Value = $font },
        [pscustomobject]@{ Key = 'terminal.integrated.fontFamily'; Value = $font },
        [pscustomobject]@{ Key = 'workbench.startupEditor'; Value = 'none' }
    )
}

# Escape every non-ASCII char (> 0x7F) in valid JSON text to a \uXXXX escape,
# yielding PURE-ASCII JSON. This is the encoding-immunity fix for the VS Code
# theme label "Rose Pine" (the accented e is U+00E9): a pure-ASCII settings.json
# reads back byte-identical under ANY code page -- including the ANSI default
# that Windows PowerShell 5.1 Get-Content uses -- so a later re-write can never
# double-encode it into the "RosA(c) Pine" mojibake that VS Code cannot resolve (theme not
# found -> silent fall back to the default dark theme). Safe on any valid JSON
# because non-ASCII can only appear inside string tokens, where \uXXXX is the
# canonical equivalent. This realizes the documented design intent (the ps1
# emits the accented e as a \u JSON escape so the settings file stays pure ASCII).
function ConvertTo-AsciiJson {
    param([Parameter(Mandatory)][AllowEmptyString()][string]$Json)
    $sb = [System.Text.StringBuilder]::new()
    foreach ($ch in $Json.ToCharArray()) {
        $code = [int][char]$ch
        if ($code -gt 0x7F) {
            [void]$sb.Append(('\u{0:x4}' -f $code))
        } else {
            [void]$sb.Append($ch)
        }
    }
    return $sb.ToString()
}

function ConvertTo-JsonStringLiteral {
    param([Parameter(Mandatory)][string]$Value)
    $escaped = $Value.Replace('\', '\\').Replace('"', '\"')
    return '"' + (ConvertTo-AsciiJson -Json $escaped) + '"'
}

# Render a settings spec value for the TEXT write paths (new-file + JSONC editor),
# which otherwise quote every value. A spec with Raw=$true emits a bare JSON
# literal -- needed for window.autoDetectColorScheme, which must be the boolean
# false, not the string "false" (VS Code rejects the string form). String specs
# fall through to the normal quoted-literal renderer. The clean-JSON merge path
# does NOT use this -- it stores the native [bool] and lets ConvertTo-Json emit
# bare false.
function ConvertTo-VSCodeSettingJson {
    param([Parameter(Mandatory)]$Spec)
    if (($Spec.PSObject.Properties.Name -contains 'Raw') -and $Spec.Raw) {
        if ($Spec.Value -is [bool]) {
            if ($Spec.Value) { return 'true' } else { return 'false' }
        }
        return [string]$Spec.Value
    }
    return ConvertTo-JsonStringLiteral -Value ([string]$Spec.Value)
}

function Test-JsonCWhitespaceChar {
    param([Parameter(Mandatory)][char]$Char)
    return (
        $Char -eq [char]32 -or
        $Char -eq [char]9 -or
        $Char -eq [char]10 -or
        $Char -eq [char]13
    )
}

function Get-JsonCTriviaEnd {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][int]$Index
    )
    $length = $Text.Length
    while ($Index -lt $length) {
        $char = $Text[$Index]
        if (Test-JsonCWhitespaceChar -Char $char) {
            $Index++
            continue
        }
        if (($char -eq [char]47) -and (($Index + 1) -lt $length)) {
            $next = $Text[$Index + 1]
            if ($next -eq [char]47) {
                $Index += 2
                while (($Index -lt $length) -and ($Text[$Index] -ne [char]10)) {
                    $Index++
                }
                continue
            }
            if ($next -eq [char]42) {
                $Index += 2
                while (
                    (($Index + 1) -lt $length) -and
                    -not (($Text[$Index] -eq [char]42) -and ($Text[$Index + 1] -eq [char]47))
                ) {
                    $Index++
                }
                if (($Index + 1) -lt $length) {
                    $Index += 2
                }
                continue
            }
        }
        break
    }
    return $Index
}

function Find-JsonCStringEnd {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][int]$Start
    )
    $escaped = $false
    for ($i = $Start + 1; $i -lt $Text.Length; $i++) {
        $char = $Text[$i]
        if ($escaped) {
            $escaped = $false
            continue
        }
        if ($char -eq [char]92) {
            $escaped = $true
            continue
        }
        if ($char -eq [char]34) {
            return $i
        }
    }
    return -1
}

function Find-JsonCValueEnd {
    param(
        [Parameter(Mandatory)][string]$Text,
        [Parameter(Mandatory)][int]$Start
    )
    $pos = Get-JsonCTriviaEnd -Text $Text -Index $Start
    if ($pos -ge $Text.Length) {
        return -1
    }
    $char = $Text[$pos]
    if ($char -eq [char]34) {
        return Find-JsonCStringEnd -Text $Text -Start $pos
    }
    if (($char -eq [char]123) -or ($char -eq [char]91)) {
        $curlyDepth = 0
        $squareDepth = 0
        $i = $pos
        while ($i -lt $Text.Length) {
            $char = $Text[$i]
            if (($char -eq [char]47) -and (($i + 1) -lt $Text.Length)) {
                $next = $Text[$i + 1]
                if ($next -eq [char]47) {
                    $i += 2
                    while (($i -lt $Text.Length) -and ($Text[$i] -ne [char]10)) {
                        $i++
                    }
                    continue
                }
                if ($next -eq [char]42) {
                    $i += 2
                    while (
                        (($i + 1) -lt $Text.Length) -and
                        -not (($Text[$i] -eq [char]42) -and ($Text[$i + 1] -eq [char]47))
                    ) {
                        $i++
                    }
                    if (($i + 1) -lt $Text.Length) {
                        $i += 2
                    }
                    continue
                }
            }
            if ($char -eq [char]34) {
                $end = Find-JsonCStringEnd -Text $Text -Start $i
                if ($end -lt 0) {
                    return -1
                }
                $i = $end + 1
                continue
            }
            if ($char -eq [char]123) {
                $curlyDepth++
            } elseif ($char -eq [char]125) {
                $curlyDepth--
                if (($curlyDepth -eq 0) -and ($squareDepth -eq 0)) {
                    return $i
                }
            } elseif ($char -eq [char]91) {
                $squareDepth++
            } elseif ($char -eq [char]93) {
                $squareDepth--
                if (($curlyDepth -eq 0) -and ($squareDepth -eq 0)) {
                    return $i
                }
            }
            $i++
        }
        return -1
    }
    $endIndex = $pos
    for ($i = $pos; $i -lt $Text.Length; $i++) {
        $char = $Text[$i]
        if (($char -eq [char]44) -or ($char -eq [char]125)) {
            break
        }
        if (($char -eq [char]47) -and (($i + 1) -lt $Text.Length)) {
            $next = $Text[$i + 1]
            if (($next -eq [char]47) -or ($next -eq [char]42)) {
                break
            }
        }
        $endIndex = $i
    }
    while (($endIndex -ge $pos) -and (Test-JsonCWhitespaceChar -Char $Text[$endIndex])) {
        $endIndex--
    }
    return $endIndex
}

function Get-DominantLineEnding {
    param([Parameter(Mandatory)][string]$Text)
    $crlf = [regex]::Matches($Text, "`r`n").Count
    $lf = [regex]::Matches($Text, "`n").Count - $crlf
    if ($crlf -gt $lf) {
        return "`r`n"
    }
    return "`n"
}

function Update-VSCodeJsonCSettings {
    param([Parameter(Mandatory)][string]$Text)
    $specs = @(Get-VSCodeSettingsSpec)
    $seen = @{}
    $replacements = @()
    $rootIndex = -1
    $curlyDepth = 0
    $squareDepth = 0
    $i = 0
    while ($i -lt $Text.Length) {
        $char = $Text[$i]
        if (($char -eq [char]47) -and (($i + 1) -lt $Text.Length)) {
            $next = $Text[$i + 1]
            if ($next -eq [char]47) {
                $i += 2
                while (($i -lt $Text.Length) -and ($Text[$i] -ne [char]10)) {
                    $i++
                }
                continue
            }
            if ($next -eq [char]42) {
                $i += 2
                while (
                    (($i + 1) -lt $Text.Length) -and
                    -not (($Text[$i] -eq [char]42) -and ($Text[$i + 1] -eq [char]47))
                ) {
                    $i++
                }
                if (($i + 1) -lt $Text.Length) {
                    $i += 2
                }
                continue
            }
        }
        if ($char -eq [char]34) {
            $end = Find-JsonCStringEnd -Text $Text -Start $i
            if ($end -lt 0) {
                throw "Invalid JSONC string"
            }
            if (($curlyDepth -eq 1) -and ($squareDepth -eq 0)) {
                $after = Get-JsonCTriviaEnd -Text $Text -Index ($end + 1)
                if (($after -lt $Text.Length) -and ($Text[$after] -eq [char]58)) {
                    $key = $Text.Substring($i + 1, $end - $i - 1)
                    foreach ($spec in $specs) {
                        if ($key -eq $spec.Key) {
                            $valueStart = Get-JsonCTriviaEnd -Text $Text -Index ($after + 1)
                            $valueEnd = Find-JsonCValueEnd -Text $Text -Start $valueStart
                            if ($valueEnd -lt 0) {
                                throw "Invalid JSONC value"
                            }
                            $seen[$spec.Key] = $true
                            $replacements += [pscustomobject]@{
                                Start = $valueStart
                                End = $valueEnd
                                Value = (ConvertTo-VSCodeSettingJson -Spec $spec)
                            }
                        }
                    }
                }
            }
            $i = $end + 1
            continue
        }
        if ($char -eq [char]123) {
            $curlyDepth++
            if ($rootIndex -lt 0) {
                $rootIndex = $i
            }
        } elseif ($char -eq [char]125) {
            $curlyDepth--
            if ($curlyDepth -lt 0) {
                $curlyDepth = 0
            }
        } elseif ($char -eq [char]91) {
            $squareDepth++
        } elseif (($char -eq [char]93) -and ($squareDepth -gt 0)) {
            $squareDepth--
        }
        $i++
    }
    if ($rootIndex -lt 0) {
        throw "Root object not found"
    }
    foreach ($replacement in @($replacements | Sort-Object -Property Start -Descending)) {
        $prefix = if ($replacement.Start -gt 0) { $Text.Substring(0, $replacement.Start) } else { '' }
        $suffixIndex = $replacement.End + 1
        $suffix = if ($suffixIndex -lt $Text.Length) { $Text.Substring($suffixIndex) } else { '' }
        $Text = $prefix + $replacement.Value + $suffix
    }
    $missing = @()
    foreach ($spec in $specs) {
        if (-not $seen.ContainsKey($spec.Key)) {
            $missing += $spec
        }
    }
    if ($missing.Count -gt 0) {
        $lineEnding = Get-DominantLineEnding -Text $Text
        $first = Get-JsonCTriviaEnd -Text $Text -Index ($rootIndex + 1)
        $hasExisting = (($first -lt $Text.Length) -and ($Text[$first] -ne [char]125))
        $insertAt = $rootIndex + 1
        if (
            (($insertAt + 1) -lt $Text.Length) -and
            ($Text[$insertAt] -eq [char]13) -and
            ($Text[$insertAt + 1] -eq [char]10)
        ) {
            $insertAt += 2
        } elseif (($insertAt -lt $Text.Length) -and ($Text[$insertAt] -eq [char]10)) {
            $insertAt++
        }
        $lines = @()
        for ($m = 0; $m -lt $missing.Count; $m++) {
            $line = '  "' + $missing[$m].Key + '": ' + (ConvertTo-VSCodeSettingJson -Spec $missing[$m])
            if ($hasExisting -or ($m -lt ($missing.Count - 1))) {
                $line += ','
            }
            $lines += $line
        }
        $block = $lineEnding + ($lines -join $lineEnding) + $lineEnding
        $Text = $Text.Substring(0, $rootIndex + 1) + $block + $Text.Substring($insertAt)
    }
    return $Text
}

# String-aware detector for JSONC comments. PowerShell 7 ConvertFrom-Json
# TOLERATES // and /* */ comments (Windows PowerShell 5.1 and jq do not), so the
# strict-JSON fast path below would silently reformat a commented settings.json
# and DELETE the user comments. Gate the fast path on this returning false: only
# a // or /* outside a string counts (a "https://" inside a string value does
# not). Mirrors the comment handling in Update-VSCodeJsonCSettings.
function Test-JsonTextHasComment {
    param([Parameter(Mandatory)][string]$Text)
    $i = 0
    while ($i -lt $Text.Length) {
        $c = $Text[$i]
        if ($c -eq [char]34) {
            $end = Find-JsonCStringEnd -Text $Text -Start $i
            if ($end -lt 0) { return $true }
            $i = $end + 1
            continue
        }
        if (($c -eq [char]47) -and (($i + 1) -lt $Text.Length)) {
            $n = $Text[$i + 1]
            if (($n -eq [char]47) -or ($n -eq [char]42)) { return $true }
        }
        $i++
    }
    return $false
}

function Set-VSCodeTheme {
    param([string]$SettingsPath)
    if ([string]::IsNullOrWhiteSpace($SettingsPath)) {
        $SettingsPath = Join-Path $env:APPDATA "Code\User\settings.json"
    }
    $dir = Split-Path -Parent $SettingsPath
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $utf8 = [System.Text.UTF8Encoding]::new($false)   # no BOM

    # Read as explicit UTF-8 -- VS Code always writes settings.json as UTF-8, but
    # Windows PowerShell 5.1 defaults Get-Content to the ANSI code page, which
    # would decode an accented byte sequence (C3 A9) as two chars ("A(c)") and
    # double-encode every non-ASCII byte the moment we re-write. Pinning UTF-8
    # makes the read/modify/write round-trip lossless on 5.1 and 7 alike.
    $raw = if (Test-Path -LiteralPath $SettingsPath) { Get-Content -Raw -LiteralPath $SettingsPath -Encoding utf8 -ErrorAction SilentlyContinue } else { $null }
    if ([string]::IsNullOrWhiteSpace($raw)) {
        $specs = @(Get-VSCodeSettingsSpec)
        $lines = @()
        for ($i = 0; $i -lt $specs.Count; $i++) {
            $comma = if ($i -lt ($specs.Count - 1)) { ',' } else { '' }
            $lines += ('  "' + $specs[$i].Key + '": ' + (ConvertTo-VSCodeSettingJson -Spec $specs[$i]) + $comma)
        }
        $json = "{`r`n" + ($lines -join "`r`n") + "`r`n}`r`n"
        [System.IO.File]::WriteAllText($SettingsPath, $json, $utf8)
        Write-Host ("  set       {0,-26} theme and fonts (new settings.json)" -f "rose-pine (vscode)")
        return
    }
    # Strict-JSON fast path ONLY when there are no comments. With comments present
    # we must use the comment-preserving JSONC editor even on PowerShell 7 (whose
    # ConvertFrom-Json would otherwise accept and silently strip them).
    if (-not (Test-JsonTextHasComment -Text $raw)) {
        try {
            $obj = $raw | ConvertFrom-Json -ErrorAction Stop
            if ($null -eq $obj -or $obj -is [System.Array]) {
                throw "settings.json root must be an object"
            }
            foreach ($spec in @(Get-VSCodeSettingsSpec)) {
                $obj | Add-Member -NotePropertyName $spec.Key -NotePropertyValue $spec.Value -Force
            }
            $merged = ConvertTo-AsciiJson -Json ($obj | ConvertTo-Json -Depth 100)
            [System.IO.File]::WriteAllText($SettingsPath, $merged, $utf8)
            Write-Host ("  set       {0,-26} theme and fonts (merged)" -f "rose-pine (vscode)")
            return
        } catch {
            # Not strict JSON (e.g. trailing commas) -- fall through to the JSONC editor.
        }
    }
    $timestamp = Get-Date -Format 'yyyyMMddHHmmssfff'
    $backup = "$SettingsPath.bak.$timestamp"
    Copy-Item -LiteralPath $SettingsPath -Destination $backup -Force
    try {
        $updated = Update-VSCodeJsonCSettings -Text $raw
        [System.IO.File]::WriteAllText($SettingsPath, $updated, $utf8)
        Write-Host ("  set       {0,-26} theme and fonts (jsonc edit; backup: {1})" -f "rose-pine (vscode)", $backup)
    } catch {
        Write-Warning ("Could not edit VS Code settings in {0}; backup: {1}" -f $SettingsPath, $backup)
    }
}

# VS Code detected -> offer the Rose Pine theme extension + set it active.
function Install-VSCodeRosePine {
    if (-not (Get-Command code -ErrorAction SilentlyContinue)) {
        # VS Code was very likely JUST installed in this same run (Install-One
        # code, immediately above), so its `code` shim is already on the
        # machine/user PATH in the registry but NOT yet in THIS process -- so the
        # theme step would skip and VS Code would open as default Dark. Re-compose
        # PATH from the registry (machine + user, deduped) the way a fresh process
        # would, then look again. (On Linux the snap/apt `code` lands on PATH
        # immediately, which is why the theme worked there but not on Windows.)
        $deduped = (@(
            [Environment]::GetEnvironmentVariable('PATH', 'Machine'),
            [Environment]::GetEnvironmentVariable('PATH', 'User'),
            $env:PATH
        ) -join ';') -split ';' | Where-Object { $_ } | Select-Object -Unique
        $env:PATH = $deduped -join ';'
    }
    if (-not (Get-Command code -ErrorAction SilentlyContinue)) {
        Write-Host ("  skipped   {0,-26} no 'code' CLI on PATH (reopen your shell after installing VS Code)" -f "rose-pine (vscode)")
        return
    }
    if (-not (Ask "VS Code: install the Rose Pine theme and set it active?")) {
        Write-Host ("  skipped   {0,-26}" -f "rose-pine (vscode)")
        return
    }
    if ($DryRun) {
        Write-Host "  would:    code --install-extension mvllow.rose-pine; set VS Code theme and font settings"
        return
    }
    code --install-extension mvllow.rose-pine 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("  installed {0,-26} mvllow.rose-pine" -f "rose-pine (vscode)")
    } else {
        Write-Warning "  'code --install-extension mvllow.rose-pine' failed"
    }
    Set-VSCodeTheme
}

# ---- Windows Terminal: managers first, pinned portable zip fallback ----------
function Install-WindowsTerminal {
    if (Test-Tool 'wt') {
        Write-Host ("  ok        {0,-26} already installed" -f "wt")
        return
    }
    if (-not (Ask "Install wt (Windows Terminal host for PowerShell and WSL)?")) {
        Write-Host ("  skipped   {0,-26}" -f "wt")
        return
    }

    $assetVersion = $WindowsTerminalVersion -replace '^v', ''
    $zipName = "Microsoft.WindowsTerminal_${assetVersion}_x64.zip"
    $zipUrl = "https://github.com/microsoft/terminal/releases/download/$WindowsTerminalVersion/$zipName"

    if ($DryRun) {
        Write-Host "  would:    scoop install extras/windows-terminal   (fallback: winget / choco / pinned portable zip $WindowsTerminalVersion)"
        Write-Host "  would:    download $zipName, verify sha256, extract under LOCALAPPDATA\\Programs\\WindowsTerminal, add to User PATH"
        return
    }

    Install-One wt -SkipPrompt -NoRecordFailure
    if (Test-Tool 'wt') { return }

    $tmp = $null
    try {
        $tmp = New-Item -ItemType Directory -Force -Path (Join-Path ([IO.Path]::GetTempPath()) "wt-portable-$([guid]::NewGuid())")
        $zip = Join-Path $tmp.FullName $zipName
        $extractRoot = Join-Path $tmp.FullName 'extract'
        Invoke-WebRequest -Uri $zipUrl -OutFile $zip -UseBasicParsing -ErrorAction Stop
        if (-not (Test-FileSha256 -Path $zip -Expected $WindowsTerminalX64Sha256)) {
            Write-Host "  FAIL: checksum mismatch for $zipName" -ForegroundColor Red
            $script:InstallFailures += [pscustomobject]@{ Tool='wt'; Pm='portable'; Pkg=$zipName; ExitCode='sha256' }
            return
        }

        Expand-Archive -Path $zip -DestinationPath $extractRoot -Force
        $wtExe = @(Get-ChildItem -Path $extractRoot -Filter 'wt.exe' -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1)
        if ($wtExe.Count -eq 0) {
            throw "portable archive did not contain wt.exe"
        }

        $installRoot = Join-Path $env:LOCALAPPDATA 'Programs\WindowsTerminal'
        if (Test-Path -LiteralPath $installRoot) {
            Remove-Item -LiteralPath $installRoot -Recurse -Force
        }
        New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
        Copy-Item -Path (Join-Path $wtExe[0].DirectoryName '*') -Destination $installRoot -Recurse -Force
        Add-DirectoryToUserPath -Directory $installRoot

        if (Test-Tool 'wt') {
            Write-Host ("  installed {0,-26} portable {1}" -f "wt", $WindowsTerminalVersion)
            return
        }
        throw "portable wt.exe installed but wt is not on PATH"
    } catch {
        Write-Warning ("Windows Terminal portable install failed: " + $_.Exception.Message)
        $script:InstallFailures += [pscustomobject]@{ Tool='wt'; Pm='portable'; Pkg=$zipName; ExitCode=$LASTEXITCODE }
    } finally {
        if ($tmp) {
            Remove-Item -LiteralPath $tmp.FullName -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# ---- psmux: native Windows tmux (reads our existing tmux/tmux.conf) ---------
# Symmetrical with the Unix tmux story. scoop is preferred (one custom bucket,
# then a normal install); falls back to winget then choco. Not in the catalog
# because the scoop install needs a bucket-add first, which Install-One does not.
function Install-Psmux {
    if (Test-Tool 'psmux') {
        Write-Host ("  ok        {0,-26} already installed" -f "psmux")
        return
    }
    if (-not (Ask "Install psmux (native Windows tmux; reads our tmux.conf)?")) {
        Write-Host ("  skipped   {0,-26}" -f "psmux")
        return
    }
    if ($DryRun) {
        Write-Host "  would: Add-ScoopBucketSafe psmux; scoop install psmux/psmux  (fallback: winget / choco)"
        return
    }
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        if (Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux') {
            scoop install psmux/psmux
            if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) {
                Write-Host ("  installed {0,-26} via scoop" -f "psmux")
                return
            }
            Write-Warning "scoop install of psmux failed; trying winget..."
        } else {
            Write-Warning "scoop bucket add psmux failed (clone auth/network); trying winget..."
        }
    }
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        winget install psmux --accept-source-agreements --accept-package-agreements --silent
        if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) {
            Write-Host ("  installed {0,-26} via winget" -f "psmux")
            return
        }
        Write-Warning "winget install of psmux failed; trying choco..."
    }
    if (Get-Command choco -ErrorAction SilentlyContinue) {
        choco install psmux -y
        if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) {
            Write-Host ("  installed {0,-26} via choco" -f "psmux")
            return
        }
    }
    Write-Warning "psmux install failed across managers; see https://github.com/psmux/psmux"
    $script:InstallFailures += [pscustomobject]@{ Tool='psmux'; Pm='scoop/winget/choco'; Pkg='psmux'; ExitCode=$LASTEXITCODE }
}

function Get-PsmuxPluginRoot {
    $base = $env:USERPROFILE
    if ([string]::IsNullOrWhiteSpace($base)) { $base = $HOME }
    if ([string]::IsNullOrWhiteSpace($base)) { $base = [Environment]::GetFolderPath('UserProfile') }
    if ([string]::IsNullOrWhiteSpace($base)) { $base = [System.IO.Path]::GetTempPath() }
    return (Join-Path (Join-Path $base '.psmux') 'plugins')
}

function Get-PsmuxPluginTarget {
    param([Parameter(Mandatory)][string]$Name)
    return (Join-Path (Get-PsmuxPluginRoot) $Name)
}

function Test-PsmuxPluginPin {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$Subdir,
        [Parameter(Mandatory)][string]$RequiredFile
    )
    $target = Get-PsmuxPluginTarget -Name $Name
    $required = Join-Path $target $RequiredFile
    $pinPath = Join-Path $target '.dotfiles-pin.json'
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) { return $false }
    if (-not (Test-Path -LiteralPath $pinPath -PathType Leaf)) { return $false }
    try {
        $pin = Get-Content -LiteralPath $pinPath -Raw | ConvertFrom-Json
        return (
            [string]$pin.repository -eq $PsmuxPluginsRepo -and
            [string]$pin.commit -eq $PsmuxPluginsCommit -and
            [string]$pin.subdir -eq $Subdir
        )
    } catch {
        return $false
    }
}

function Write-PsmuxPluginPin {
    param(
        [Parameter(Mandatory)][string]$Target,
        [Parameter(Mandatory)][string]$Subdir
    )
    $pin = [ordered]@{
        repository = $PsmuxPluginsRepo
        commit = $PsmuxPluginsCommit
        subdir = $Subdir
        managedBy = 'dotfiles/install-deps.ps1'
    } | ConvertTo-Json
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText((Join-Path $Target '.dotfiles-pin.json'), $pin + [Environment]::NewLine, $utf8)
}

function Install-PsmuxPluginFromClone {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$Subdir,
        [Parameter(Mandatory)][string]$RequiredFile,
        [Parameter(Mandatory)][string]$CloneRoot
    )
    $source = Join-Path $CloneRoot $Subdir
    $target = Get-PsmuxPluginTarget -Name $Name
    if (-not (Test-Path -LiteralPath (Join-Path $source $RequiredFile) -PathType Leaf)) {
        throw "psmux plugin source $Subdir is missing required file $RequiredFile"
    }

    if (Test-Path -LiteralPath $target) {
        $pinPath = Join-Path $target '.dotfiles-pin.json'
        if (Test-Path -LiteralPath $pinPath -PathType Leaf) {
            Remove-Item -LiteralPath $target -Recurse -Force
        } else {
            $backup = "$target.bak.$(Get-Date -Format 'yyyyMMdd-HHmmss')"
            Move-Item -LiteralPath $target -Destination $backup -Force
            Write-Host ("  backup    {0,-26} {1}" -f $Name, $backup)
        }
    }

    New-Item -ItemType Directory -Force -Path $target | Out-Null
    Get-ChildItem -LiteralPath $source -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $target -Recurse -Force
    }
    Write-PsmuxPluginPin -Target $target -Subdir $Subdir
    if (-not (Test-PsmuxPluginPin -Name $Name -Subdir $Subdir -RequiredFile $RequiredFile)) {
        throw "psmux plugin $Name failed post-install pin verification"
    }
    Write-Host ("  installed {0,-26} {1}" -f $Name, $PsmuxPluginsCommit)
}

function Install-PsmuxPlugins {
    if (
        (Test-PsmuxPluginPin -Name 'ppm' -Subdir 'ppm' -RequiredFile 'ppm.ps1') -and
        (Test-PsmuxPluginPin -Name 'psmux-theme-rosepine' -Subdir 'psmux-theme-rosepine' -RequiredFile 'psmux-theme-rosepine.ps1')
    ) {
        Write-Host ("  ok        {0,-26} pinned refs already installed" -f "psmux plugins")
        return
    }
    if (-not (Ask "Install PPM + psmux-theme-rosepine (repo-managed pinned refs)?")) {
        Write-Host ("  skipped   {0,-26}" -f "psmux plugins")
        return
    }
    if ($DryRun) {
        Write-Host ("  would:    git init/fetch exact commit {0} from {1}" -f $PsmuxPluginsCommit, $PsmuxPluginsRepo)
        Write-Host ("  would:    copy ppm + psmux-theme-rosepine into {0}" -f (Get-PsmuxPluginRoot))
        return
    }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Host ("  manual    {0,-26} git is required for pinned plugin install" -f "psmux plugins")
        $script:InstallFailures += [pscustomobject]@{ Tool='psmux plugins'; Pm='git'; Pkg='psmux-plugins'; ExitCode='git-missing' }
        return
    }

    $tmp = $null
    try {
        $tmp = New-Item -ItemType Directory -Force -Path (Join-Path ([IO.Path]::GetTempPath()) "psmux-plugins-$([guid]::NewGuid())")
        $clone = Join-Path $tmp.FullName 'repo'
        New-Item -ItemType Directory -Force -Path $clone | Out-Null
        git -C $clone init -q 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "git init failed with exit $LASTEXITCODE" }
        git -C $clone remote add origin $PsmuxPluginsRepo 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "git remote add failed with exit $LASTEXITCODE" }
        git -C $clone fetch --depth 1 origin $PsmuxPluginsCommit 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "git fetch $PsmuxPluginsCommit failed with exit $LASTEXITCODE" }
        git -C $clone checkout --force FETCH_HEAD 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "git checkout FETCH_HEAD failed with exit $LASTEXITCODE" }
        $current = (git -C $clone rev-parse HEAD 2>$null).Trim()
        if ($current -ne $PsmuxPluginsCommit) {
            throw "psmux-plugins checkout resolved $current, expected $PsmuxPluginsCommit"
        }

        Install-PsmuxPluginFromClone -Name 'ppm' -Subdir 'ppm' -RequiredFile 'ppm.ps1' -CloneRoot $clone
        Install-PsmuxPluginFromClone -Name 'psmux-theme-rosepine' -Subdir 'psmux-theme-rosepine' -RequiredFile 'psmux-theme-rosepine.ps1' -CloneRoot $clone
    } catch {
        Write-Warning ("psmux plugin install failed: " + $_.Exception.Message)
        $script:InstallFailures += [pscustomobject]@{ Tool='psmux plugins'; Pm='git'; Pkg='psmux-plugins'; ExitCode=$LASTEXITCODE }
    } finally {
        if ($tmp) {
            Remove-Item -LiteralPath $tmp.FullName -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Install-TreeSitterCli {
    if (Test-Tool 'tree-sitter') {
        Write-Host ("  ok        {0,-26} already installed" -f "tree-sitter")
        return
    }
    if (-not (Ask "Install tree-sitter CLI (nvim-treesitter main parser builds)?")) {
        Write-Host ("  skipped   {0,-26}" -f "tree-sitter")
        return
    }
    if ($DryRun) {
        Write-Host "  would:    scoop install tree-sitter"
        Write-Host "  would:    npm install -g tree-sitter-cli   (fallback if package managers do not provide tree-sitter)"
        return
    }

    Install-One 'tree-sitter' -SkipPrompt -NoRecordFailure
    if (Test-Tool 'tree-sitter') { return }

    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        Write-Warning "npm is not on PATH; cannot use npm tree-sitter-cli fallback"
        $script:InstallFailures += [pscustomobject]@{ Tool='tree-sitter'; Pm='scoop/npm'; Pkg='tree-sitter-cli'; ExitCode='npm-missing' }
        return
    }

    npm install -g tree-sitter-cli
    if ($LASTEXITCODE -eq 0 -and (Test-Tool 'tree-sitter')) {
        Write-Host ("  installed {0,-26} via npm" -f "tree-sitter")
        return
    }

    Write-Warning "npm install of tree-sitter-cli failed or tree-sitter is still not on PATH"
    $script:InstallFailures += [pscustomobject]@{ Tool='tree-sitter'; Pm='scoop/npm'; Pkg='tree-sitter-cli'; ExitCode=$LASTEXITCODE }
}

function Get-VsWherePath {
    $programFilesX86 = ${env:ProgramFiles(x86)}
    if ([string]::IsNullOrWhiteSpace($programFilesX86)) { return '' }
    return (Join-Path $programFilesX86 'Microsoft Visual Studio\Installer\vswhere.exe')
}

function Get-VsBuildToolsInstallationPath {
    param([string]$VsWherePath = (Get-VsWherePath))

    if ([string]::IsNullOrWhiteSpace($VsWherePath)) { return '' }
    if (-not (Test-Path -LiteralPath $VsWherePath -PathType Leaf)) { return '' }

    try {
        $result = @(& $VsWherePath `
                -products * `
                -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
                -property installationPath 2>$null)
        if ($LASTEXITCODE -ne 0) { return '' }
        foreach ($line in $result) {
            $path = ([string]$line).Trim()
            if (-not [string]::IsNullOrWhiteSpace($path)) {
                return $path
            }
        }
    } catch {
        return ''
    }
    return ''
}

function Install-VsBuildTools {
    $existingPath = Get-VsBuildToolsInstallationPath
    if (-not [string]::IsNullOrWhiteSpace($existingPath)) {
        Write-Host ("  ok        {0,-26} VC toolset at {1}" -f "VS Build Tools", $existingPath)
        return
    }

    $wingetId = 'Microsoft.VisualStudio.2022.BuildTools'
    $override = '--quiet --wait --norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended'
    if ($DryRun) {
        Write-Host "  would:    winget install --id $wingetId -e --accept-package-agreements --accept-source-agreements --override `"$override`""
        Write-Host "  would:    choco install -y visualstudio2022buildtools; choco install -y visualstudio2022-workload-vctools"
        Write-Host "  would:    download $VsBuildToolsBootstrapperUrl"
        Write-Host "  would:    vs_BuildTools.exe $override"
        return
    }

    Write-Host "  note      VS Build Tools is a multi-GB install; this can take a while."
    $lastExit = $null
    try {
        if (Get-Command winget -ErrorAction SilentlyContinue) {
            winget install --id $wingetId -e --accept-package-agreements --accept-source-agreements --override $override
            $lastExit = $LASTEXITCODE
            if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace((Get-VsBuildToolsInstallationPath))) {
                Write-Host ("  installed {0,-26} via winget" -f "VS Build Tools")
                return
            }
            Write-Warning "winget VS Build Tools install did not leave a detected VC toolset; trying choco..."
        }

        if (Get-Command choco -ErrorAction SilentlyContinue) {
            choco install -y visualstudio2022buildtools
            $buildToolsExit = $LASTEXITCODE
            choco install -y visualstudio2022-workload-vctools
            $workloadExit = $LASTEXITCODE
            $lastExit = $workloadExit
            if ($buildToolsExit -eq 0 -and $workloadExit -eq 0 -and -not [string]::IsNullOrWhiteSpace((Get-VsBuildToolsInstallationPath))) {
                Write-Host ("  installed {0,-26} via choco" -f "VS Build Tools")
                return
            }
        }

        Write-Warning "Package managers did not leave a detected VC toolset; trying the official Microsoft bootstrapper..."
        $bootstrapExit = Install-VsBuildToolsFromBootstrapper
        $lastExit = $bootstrapExit
        if ($bootstrapExit -eq 0 -and -not [string]::IsNullOrWhiteSpace((Get-VsBuildToolsInstallationPath))) {
            Write-Host ("  installed {0,-26} via Microsoft bootstrapper" -f "VS Build Tools")
            return
        }
    } catch {
        Write-Warning ("VS Build Tools install raised an exception: " + $_.Exception.Message)
    }

    Write-Host "  FAIL: VS Build Tools install failed or VC toolset was not detected" -ForegroundColor Red
    $script:InstallFailures += [pscustomobject]@{ Tool='VS Build Tools'; Pm='winget/choco/bootstrapper'; Pkg='Microsoft.VisualStudio.Workload.VCTools'; ExitCode=$lastExit }
}

function Install-VsBuildToolsFromBootstrapper {
    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("dotfiles-vs-buildtools-" + [System.Guid]::NewGuid())
    $installer = Join-Path $tempDir 'vs_BuildTools.exe'
    $args = @(
        '--quiet',
        '--wait',
        '--norestart',
        '--add',
        'Microsoft.VisualStudio.Workload.VCTools',
        '--includeRecommended'
    )

    try {
        New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
        Invoke-WebRequest -Uri $VsBuildToolsBootstrapperUrl -OutFile $installer -UseBasicParsing -ErrorAction Stop

        $process = Start-Process -FilePath $installer -ArgumentList $args -Wait -PassThru
        $exitCode = [int]$process.ExitCode
        if ($exitCode -eq 740 -and -not (Test-IsElevated)) {
            Write-Host "  note      VS Build Tools requires elevation; requesting UAC for the Microsoft bootstrapper."
            $process = Start-Process -FilePath $installer -ArgumentList $args -Wait -PassThru -Verb RunAs
            $exitCode = [int]$process.ExitCode
        }
        return $exitCode
    } catch {
        Write-Warning ("VS Build Tools bootstrapper failed: " + $_.Exception.Message)
        return 1
    } finally {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Install-VsBuildToolsWhenAll {
    param([bool]$IsAll = $All)
    if ($IsAll) {
        Install-VsBuildTools
    }
}

# PSFzf is a PowerShell module (PSGallery), not a package-manager binary, so it
# installs via Install-Module rather than the $Catalog. It wires fzf into
# PSReadLine (Ctrl+R / Ctrl+T / Alt+C); the profile activates those bindings only
# when both PSFzf and the fzf binary are present. Not pinned -- matches the rest
# of the provisioning layer.
function Install-PSFzf {
    if (Get-Module -ListAvailable -Name PSFzf) {
        Write-Host ("  ok        {0,-26} already installed" -f "PSFzf")
        return
    }
    if (-not (Ask "Install PSFzf (fzf fuzzy pickers for PSReadLine)?")) {
        Write-Host ("  skipped   {0,-26}" -f "PSFzf")
        return
    }
    if ($DryRun) {
        Write-Host "  would: Install-Module PSFzf -Scope CurrentUser -Repository PSGallery -Force"
        return
    }
    try {
        # Non-interactive bootstrap: ensure the NuGet provider so Install-Module
        # never blocks on a Y/N prompt in CI. -Force suppresses the
        # untrusted-repository prompt for PSGallery.
        try { $null = Get-PackageProvider -Name NuGet -ForceBootstrap -ErrorAction Stop } catch { Write-Verbose $_.Exception.Message }
        Install-Module -Name PSFzf -Scope CurrentUser -Repository PSGallery -Force -AllowClobber -ErrorAction Stop
        if (Get-Module -ListAvailable -Name PSFzf) {
            Write-Host ("  installed {0,-26} via PSGallery" -f "PSFzf")
            return
        }
    } catch {
        Write-Warning ("PSFzf install failed: " + $_.Exception.Message)
    }
    $script:InstallFailures += [pscustomobject]@{ Tool='PSFzf'; Pm='PSGallery'; Pkg='PSFzf'; ExitCode=$LASTEXITCODE }
}

function Get-CatalogUpdateSpec {
    param([object[]]$SpecList = @(Get-InstallDependencySpec))
    $seen = @{}
    foreach ($spec in $SpecList) {
        if ($Catalog.ContainsKey($spec.Tool) -and -not $seen.ContainsKey($spec.Tool)) {
            $seen[$spec.Tool] = $true
            $spec
        }
    }
}

function Invoke-InstallDepsUpdateMode {
    param(
        [object[]]$SpecList = @(Get-InstallDependencySpec),
        [scriptblock]$PresenceTester,
        [bool]$IsDryRun = $DryRun
    )

    Write-Host ("install-deps: update mode  scoop=" + [bool](Get-Command scoop -ErrorAction SilentlyContinue) + "  winget=" + [bool](Get-Command winget -ErrorAction SilentlyContinue) + "  choco=" + [bool](Get-Command choco -ErrorAction SilentlyContinue) + "  dry-run=$IsDryRun")
    Write-Host "note: present catalog tools update only through the package manager that owns them."
    Write-Host ""

    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) {
        Write-Host "  skipped   scoop update              Scoop is not installed"
    } elseif ($IsDryRun) {
        Write-Host "  would:    scoop update"
    } else {
        scoop update | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Warning ("  scoop manifest refresh failed (exit {0})" -f $LASTEXITCODE)
            $script:InstallFailures += [pscustomobject]@{ Tool='scoop'; Pm='scoop'; Pkg='manifest'; ExitCode=$LASTEXITCODE }
        }
    }

    foreach ($spec in (Get-CatalogUpdateSpec -SpecList $SpecList)) {
        $tool = $spec.Tool
        $present = if ($PresenceTester) {
            [bool](& $PresenceTester $tool)
        } else {
            Test-Tool $tool
        }
        if (-not $present) {
            Write-Host ("  skipped   {0,-26} not installed" -f $tool)
            continue
        }
        Update-ManagedCatalogTool -tool $tool -NoPrompt -SkipScoopManifestRefresh -ReportSkip -AssumePresent -IsDryRun $IsDryRun
    }

    Write-UnmanagedDependencySummary
    Write-Host ""
    Write-Host "note: pinned binaries (Neovim/lazygit/tree-sitter Linux archives, Hack Nerd Font, Windows Terminal portable), PSFzf, plugins, and configs update via git pull and re-running setup."
}

function Exit-InstallDepsIfFailures {
    if ($script:InstallFailures.Count -eq 0) { return }

    Write-Host "install-deps: completed with $($script:InstallFailures.Count) FAILED install(s):"
    foreach ($f in $script:InstallFailures) {
        Write-Host ("  FAIL  {0,-20} via {1,-8} pkg={2}  (exit {3})" -f $f.Tool, $f.Pm, $f.Pkg, $f.ExitCode) -ForegroundColor Red
    }
    Write-Host ""
    Write-Host "Re-run install-deps.ps1 after addressing the failures, or"
    Write-Host "install the listed packages manually."
    if ($DryRun) { Write-Host "(dry run -- nothing was actually attempted)" }
    Write-Host ""
    Write-Host "Next: run .\setup.ps1, or let setup.ps1 continue if it invoked this phase."
    exit 1
}

if ($env:INSTALL_DEPS_PS1_SOURCE_ONLY) { return }

$Pm = Get-AvailablePM

if ($Update) {
    Invoke-InstallDepsUpdateMode -IsDryRun $DryRun
    Exit-InstallDepsIfFailures
    exit 0
}

Write-Host ""
Write-Host ("install-deps: primary PM=$Pm  scoop=" + [bool](Get-Command scoop -ErrorAction SilentlyContinue) + "  dry-run=$DryRun  yes-all=$All")
Write-Host ""

$dependencyScan = @(Get-InstallDependencyScan)
Show-InstallDependencyTable -Rows $dependencyScan
Write-Host ""
$missingDependencyCount = @($dependencyScan | Where-Object { $_.Status -eq 'missing' }).Count

# One-shot "install everything" vs per-item prompts. Skipped when -All / -DryRun
# was passed or the session is non-interactive. Enter / Y == everything.
if (Test-InstallPromptAvailable) {
    $resp = Read-Host "Install the $missingDependencyCount missing tools listed above without further prompts? [Y/n]  (n = choose per tool)"
    if ($resp -match '^[Nn]') {
        Write-Host "  -> per-item prompts"
    } else {
        $All = $true
        Write-Host "  -> installing everything; no further prompts"
    }
    Write-Host ""
}

# If no package manager at all, try scoop first (no admin required).
if (-not $Pm) {
    Write-Warning "No package manager detected (winget / choco / scoop)."
    if (Install-Scoop) { $Pm = Get-AvailablePM }
}

# Even when winget/choco are available, scoop unlocks extras
# (taplo, win32yank, nerd-fonts bucket). Offer it as a complement.
if ($Pm -and -not (Get-Command scoop -ErrorAction SilentlyContinue)) {
    Write-Host ""
    Write-Host "Detected $Pm. Scoop is also recommended -- it carries taplo,"
    Write-Host "win32yank, and the nerd-fonts bucket that $Pm does not have."
    Install-Scoop | Out-Null
    $Pm = Get-AvailablePM
}

if (-not $Pm) {
    Write-Warning "No supported package manager available. Install winget from the"
    Write-Warning "Microsoft Store ('App Installer'), or accept the Scoop offer above."
    exit 1
}

function Section { param([string]$title) Write-Host ""; Write-Host "== $title ==" }

# ---- Sections ----------------------------------------------------------------
Section "core editor stack"
Install-One git
Install-One nvim
Install-One make
Install-One cmake
Install-One rg
Install-One fd
Install-One fzf
Install-One lsd
Install-One chezmoi
Install-One lazygit

Section "prompt"
Install-One starship

Section "terminal host"
Install-WindowsTerminal

Section "terminal multiplexer (psmux: tmux for native Windows, optional)"
Install-Psmux
Install-PsmuxPlugins

Section "modern shell (optional, you can stay on Windows PowerShell 5.1)"
Install-One pwsh
Update-ManagedCatalogTool pwsh -ReportSkip
Install-PSFzf

Section "language tooling (for LSP / formatter back-ends)"
Install-Python
Install-PylatexencConverter
Install-One node
Install-TreeSitterCli
Install-One zig
Install-VsBuildToolsWhenAll

Section "WSL clipboard bridge (skip if you don't use WSL nvim)"
Install-One win32yank

Section "developer / test dependencies (optional)"
Install-One jq
Install-One shellcheck
Install-One hyperfine
Install-One taplo

Section "editor: VS Code (optional)"
Install-One code
Install-VSCodeRosePine

Section "fonts"
Install-HackNerdFont

Section "Ghostty terminal (manual step on Windows)"
Write-Host "  manual    Ghostty does not have a Windows build yet."
Write-Host "            Use Windows Terminal (setup applies the rose-pine"
Write-Host "            fragment by default) or WezTerm for now."

Write-Host ""
Exit-InstallDepsIfFailures
Write-Host "install-deps: done"
if ($DryRun) { Write-Host "(dry run -- nothing was installed)" }
Write-Host ""
Write-Host "Next: run .\setup.ps1, or let setup.ps1 continue if it invoked this phase."
