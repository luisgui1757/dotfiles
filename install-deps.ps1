# install-deps.ps1 -- interactively install dependencies on Windows.
#
# Prefers scoop per tool (most reliable for these CLI tools; sidesteps the flaky
# winget "No package found matching input criteria" source errors), then falls
# back to winget, then chocolatey -- if one manager fails for a tool, the next
# is tried automatically. Offers to bootstrap scoop when missing.
# Prints manual-install hints only when no manager carries the package.
#
# Usage:
#   .\install-deps.ps1            prompt Y/n for each tool
#   .\install-deps.ps1 -All       skip prompts, install everything
#   .\install-deps.ps1 -DryRun    print what would be installed without acting

[CmdletBinding()]
param(
    [switch]$All,
    [switch]$DryRun
)

$ErrorActionPreference = 'Continue'
$HackNerdFontVersion = 'v3.4.0'
$HackNerdFontSha256 = '8ca33a60c791392d872b80d26c42f2bfa914a480f9eb2d7516d9f84373c36897'

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

function Add-ScoopToPathForCurrentProcess {
    $scoopRoot = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
    $shimDir = Join-Path $scoopRoot 'shims'
    if ((Test-Path -LiteralPath $shimDir) -and (($env:PATH -split ';') -notcontains $shimDir)) {
        $env:PATH = "$shimDir;$env:PATH"
    }
}

function Install-Scoop {
    if (Get-Command scoop -ErrorAction SilentlyContinue) { return $true }
    Write-Host "Scoop is not installed. It is a userspace package manager that"
    Write-Host "carries tools missing from winget/choco (taplo, win32yank, etc.)."
    if (-not (Ask "Install Scoop via the official one-liner?")) { return $false }
    if ($DryRun) {
        if (Test-IsElevated) {
            Write-Host "  would: download get.scoop.sh, then run install.ps1 -RunAsAdmin"
        } else {
            Write-Host "  would: irm get.scoop.sh | iex"
        }
        return $false
    }
    try {
        # The official scoop bootstrap. RemoteSigned policy is needed for the
        # script; we set it for the current process only.
        Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process -Force
        if (Test-IsElevated) {
            # GitHub Windows runners are elevated. Scoop blocks elevated
            # bootstrap by default, so use the installer documented opt-in
            # instead of trying to de-elevate the CI process.
            $installer = Join-Path $env:TEMP "scoop-install-$([guid]::NewGuid()).ps1"
            Invoke-WebRequest -Uri 'https://get.scoop.sh' -OutFile $installer -UseBasicParsing -ErrorAction Stop
            & $installer -RunAsAdmin
            $rc = $LASTEXITCODE
            Remove-Item -LiteralPath $installer -Force -ErrorAction SilentlyContinue
            if ($rc -ne 0) {
                throw "Scoop installer exited $rc"
            }
        } else {
            Invoke-Expression (Invoke-RestMethod -Uri 'https://get.scoop.sh' -ErrorAction Stop)
            if ($LASTEXITCODE -ne 0) {
                throw "Scoop installer exited $LASTEXITCODE"
            }
        }
        Add-ScoopToPathForCurrentProcess
        # Add the standard extras bucket so we get things like win32yank.
        scoop bucket add extras 2>$null | Out-Null
        scoop bucket add nerd-fonts 2>$null | Out-Null
        return [bool](Get-Command scoop -ErrorAction SilentlyContinue)
    } catch {
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

function Test-FileSha256 {
    param([string]$Path, [string]$Expected)
    return ((Get-FileHash $Path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $Expected.ToLowerInvariant())
}

# ---- Per-tool: package id per PM. Empty string means "not available there". --
# Keys are the command name we check via Get-Command.
$Catalog = @{
    git                  = @{ winget = 'Git.Git';                          choco = 'git';                  scoop = 'git'                  ; purpose = 'version control' }
    nvim                 = @{ winget = 'Neovim.Neovim';                    choco = 'neovim';               scoop = 'neovim'               ; purpose = 'Neovim 0.11+ editor' }
    starship             = @{ winget = 'Starship.Starship';                choco = 'starship';             scoop = 'starship'             ; purpose = 'cross-shell prompt' }
    rg                   = @{ winget = 'BurntSushi.ripgrep.MSVC';          choco = 'ripgrep';              scoop = 'ripgrep'              ; purpose = 'Telescope live_grep backend' }
    fd                   = @{ winget = 'sharkdp.fd';                       choco = 'fd';                   scoop = 'fd'                   ; purpose = 'Telescope find_files backend' }
    make                 = @{ winget = 'GnuWin32.Make';                    choco = 'make';                 scoop = 'make'                 ; purpose = 'plugin builds (LuaSnip jsregexp)' }
    pwsh                 = @{ winget = 'Microsoft.PowerShell';             choco = 'powershell-core';      scoop = 'pwsh'                 ; purpose = 'modern PowerShell 7' }
    'win32yank'          = @{ winget = '';                                 choco = 'win32yank';            scoop = 'win32yank'            ; purpose = 'clipboard bridge for WSL nvim' }
    node                 = @{ winget = 'OpenJS.NodeJS.LTS';                choco = 'nodejs-lts';           scoop = 'nodejs-lts'           ; purpose = 'prettier + JS tooling' }
    python               = @{ winget = 'Python.Python.3.12';               choco = 'python';               scoop = 'python'               ; purpose = 'pyright + tooling' }
    jq                   = @{ winget = 'jqlang.jq';                        choco = 'jq';                   scoop = 'jq'                   ; purpose = 'general-purpose JSON CLI' }
    shellcheck           = @{ winget = 'koalaman.shellcheck';              choco = 'shellcheck';           scoop = 'shellcheck'           ; purpose = 'shell-script linter' }
    hyperfine            = @{ winget = 'sharkdp.hyperfine';                choco = 'hyperfine';            scoop = 'hyperfine'            ; purpose = 'starship perf benchmark' }
    taplo                = @{ winget = '';                                 choco = '';                     scoop = 'taplo'                ; purpose = 'TOML linter' }
    code                 = @{ winget = 'Microsoft.VisualStudioCode';       choco = 'vscode';               scoop = 'extras/vscode'        ; purpose = 'VS Code editor' }
}

# Some Catalog keys (e.g. "rg") map to a different actual binary on Windows
# than on Unix. Provide a name -> binary mapping for Get-Command checks.
$BinaryName = @{
    rg          = 'rg'
    fd          = 'fd'
    nvim        = 'nvim'
    pwsh        = 'pwsh'
    'win32yank' = 'win32yank'
    starship    = 'starship'
    git         = 'git'
    make        = 'make'
    node        = 'node'
    python      = 'python'
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

function Install-One {
    param([string]$tool)
    if (Test-Tool $tool) {
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
    if (-not (Ask $promptText)) {
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
        if ($LASTEXITCODE -eq 0 -and (Test-Tool $tool)) {
            Write-Host ("  installed {0,-26} via {1}" -f $tool, $c.pm)
            $installed = $true
            break
        }
        Write-Warning ("  $($c.pm) install of $($c.pkg) failed (exit $LASTEXITCODE); trying next manager...")
    }
    if (-not $installed) {
        # Track failures so we can summarize at the end instead of faking success.
        $tried = ($candidates | ForEach-Object { $_.pm }) -join '/'
        $script:InstallFailures += [pscustomobject]@{ Tool = $tool; Pm = $tried; Pkg = $first.pkg; ExitCode = $LASTEXITCODE }
    }
}

# Track failures across the run so we can warn loudly at the end instead of
# pretending success.
$script:InstallFailures = @()

# ---- Hack Nerd Font: prefer scoop bucket, fall back to direct download+register
function Install-HackNerdFont {
    # Already installed?
    $userFonts = "$env:LOCALAPPDATA\Microsoft\Windows\Fonts"
    $sysFonts = "$env:WINDIR\Fonts"
    if ((Test-Path $userFonts -PathType Container) -and
        (Get-ChildItem -Path $userFonts -Filter "Hack*Nerd*" -ErrorAction SilentlyContinue)) {
        Write-Host ("  ok        {0,-26} already installed (user)" -f "Hack Nerd Font")
        return
    }
    if ((Test-Path $sysFonts -PathType Container) -and
        (Get-ChildItem -Path $sysFonts -Filter "Hack*Nerd*" -ErrorAction SilentlyContinue)) {
        Write-Host ("  ok        {0,-26} already installed (system)" -f "Hack Nerd Font")
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
        scoop bucket add nerd-fonts 2>$null | Out-Null
        scoop install nerd-fonts/Hack-NF
        if ($LASTEXITCODE -eq 0) {
            Write-Host ("  installed {0,-26} via scoop" -f "Hack Nerd Font")
            return
        }
        Write-Warning "scoop install failed; falling back to direct download."
    }

    # Path 2: download Hack.zip and register fonts user-scope. No admin needed.
    if ($DryRun) {
        Write-Host "  would: download nerd-fonts/$HackNerdFontVersion/Hack.zip, verify sha256, extract, register in HKCU\\Fonts"
        return
    }
    try {
        $tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "hack-nf-$([guid]::NewGuid())")
        $zip = Join-Path $tmp.FullName "Hack.zip"
        Invoke-WebRequest -Uri "https://github.com/ryanoasis/nerd-fonts/releases/download/$HackNerdFontVersion/Hack.zip" -OutFile $zip -UseBasicParsing
        if (-not (Test-FileSha256 -Path $zip -Expected $HackNerdFontSha256)) {
            throw "Hack.zip SHA256 verification failed"
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
        Remove-Item -Recurse -Force $tmp.FullName
        Write-Host ("  installed {0,-26} {1} font files registered in HKCU" -f "Hack Nerd Font", $installedCount)
        Write-Host "             (you may need to restart your terminal to see them)"
    } catch {
        Write-Warning ("Hack Nerd Font install failed: " + $_.Exception.Message)
        Write-Host  "  manual    download Hack.zip from nerd-fonts releases and install via the Fonts control panel."
    }
}

# ---- VS Code Rose Pine theme -------------------------------------------------
# Set "workbench.colorTheme" to Rose Pine in %APPDATA%\Code\User\settings.json.
# The theme label has an accented e; we write it as the JSON escape \u00e9 (this
# file must stay pure ASCII) or build it with [char]0xE9 for the merge path.
function Set-VSCodeTheme {
    $settings = Join-Path $env:APPDATA "Code\User\settings.json"
    $dir = Split-Path -Parent $settings
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $utf8 = [System.Text.UTF8Encoding]::new($false)   # no BOM

    $raw = if (Test-Path -LiteralPath $settings) { Get-Content -Raw -LiteralPath $settings -ErrorAction SilentlyContinue } else { $null }
    if ([string]::IsNullOrWhiteSpace($raw)) {
        $json = "{`r`n  ""workbench.colorTheme"": ""Ros\u00e9 Pine""`r`n}`r`n"
        [System.IO.File]::WriteAllText($settings, $json, $utf8)
        Write-Host ("  set       {0,-26} workbench.colorTheme (new settings.json)" -f "rose-pine (vscode)")
        return
    }
    try {
        $obj = $raw | ConvertFrom-Json -ErrorAction Stop
        $obj | Add-Member -NotePropertyName 'workbench.colorTheme' -NotePropertyValue ("Ros$([char]0xE9) Pine") -Force
        [System.IO.File]::WriteAllText($settings, ($obj | ConvertTo-Json -Depth 100), $utf8)
        Write-Host ("  set       {0,-26} workbench.colorTheme (merged)" -f "rose-pine (vscode)")
    } catch {
        Write-Host ("  note      set workbench.colorTheme to ""Rose Pine"" in $settings (left untouched: comments/invalid JSON)")
    }
}

# VS Code detected -> offer the Rose Pine theme extension + set it active.
function Install-VSCodeRosePine {
    if (-not (Get-Command code -ErrorAction SilentlyContinue)) {
        Write-Host ("  skipped   {0,-26} no 'code' CLI on PATH (reopen your shell after installing VS Code)" -f "rose-pine (vscode)")
        return
    }
    if (-not (Ask "VS Code: install the Rose Pine theme and set it active?")) {
        Write-Host ("  skipped   {0,-26}" -f "rose-pine (vscode)")
        return
    }
    if ($DryRun) {
        Write-Host "  would:    code --install-extension mvllow.rose-pine; set workbench.colorTheme = Rose Pine"
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
        Write-Host "  would: scoop bucket add psmux https://github.com/psmux/scoop-psmux; scoop install psmux  (fallback: winget / choco)"
        return
    }
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        scoop bucket add psmux https://github.com/psmux/scoop-psmux 2>$null | Out-Null
        scoop install psmux
        if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) {
            Write-Host ("  installed {0,-26} via scoop" -f "psmux")
            return
        }
        Write-Warning "scoop install of psmux failed; trying winget..."
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

if ($env:INSTALL_DEPS_PS1_SOURCE_ONLY) { return }

$Pm = Get-AvailablePM

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
}

Write-Host ""
Write-Host ("install-deps: primary PM=$Pm  scoop=" + [bool](Get-Command scoop -ErrorAction SilentlyContinue) + "  dry-run=$DryRun  yes-all=$All")
Write-Host ""

if (-not $Pm) {
    Write-Warning "No supported package manager available. Install winget from the"
    Write-Warning "Microsoft Store ('App Installer'), or accept the Scoop offer above."
    exit 1
}

# One-shot "install everything" vs per-item prompts. Skipped when -All / -DryRun
# was passed or the session is non-interactive. Enter / Y == everything.
if ((-not $All) -and (-not $DryRun) -and [Environment]::UserInteractive) {
    $resp = Read-Host "Install EVERYTHING without further prompts? [Y/n]  (n = choose per tool)"
    if ($resp -match '^[Nn]') {
        Write-Host "  -> per-item prompts"
    } else {
        $All = $true
        Write-Host "  -> installing everything; no further prompts"
    }
    Write-Host ""
}

function Section { param([string]$title) Write-Host ""; Write-Host "== $title ==" }

# ---- Sections ----------------------------------------------------------------
Section "core editor stack"
Install-One git
Install-One nvim
Install-One make
Install-One rg
Install-One fd

Section "prompt"
Install-One starship

Section "terminal multiplexer (psmux: tmux for native Windows, optional)"
Install-Psmux

Section "modern shell (optional, you can stay on Windows PowerShell 5.1)"
Install-One pwsh

Section "language tooling (for LSP / formatter back-ends)"
Install-One python
Install-One node

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
Write-Host "            Use Windows Terminal (.\bootstrap.ps1 -MergeWindowsTerminal applies"
Write-Host "            the rose-pine fragment) or WezTerm for now."

Write-Host ""
if ($script:InstallFailures.Count -gt 0) {
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
Write-Host "install-deps: done"
if ($DryRun) { Write-Host "(dry run -- nothing was installed)" }
Write-Host ""
Write-Host "Next: run .\setup.ps1, or let setup.ps1 continue if it invoked this phase."
