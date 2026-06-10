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
    $scoopRoot = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
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
            Remove-Item -LiteralPath $installer -Force -ErrorAction SilentlyContinue
        } else {
            Invoke-Expression (Invoke-RestMethod -Uri 'https://get.scoop.sh' -ErrorAction Stop)
        }
        Add-ScoopToPathForCurrentProcess
        # Add the standard buckets so existing catalog entries resolve.
        Ensure-ScoopBuckets
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
    fzf                  = @{ winget = 'junegunn.fzf';                     choco = 'fzf';                  scoop = 'fzf'                  ; purpose = 'fuzzy finder (PSFzf history/file/dir pickers)' }
    chezmoi              = @{ winget = 'twpayne.chezmoi';                  choco = 'chezmoi';              scoop = 'chezmoi'              ; purpose = 'dotfiles config manager' }
    lazygit              = @{ winget = 'JesseDuffield.lazygit';            choco = 'lazygit';              scoop = 'lazygit'              ; purpose = 'terminal git UI' }
    wt                   = @{ winget = 'Microsoft.WindowsTerminal';        choco = 'microsoft-windows-terminal'; scoop = 'extras/windows-terminal'; purpose = 'Windows Terminal host for PowerShell and WSL' }
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
    fzf         = 'fzf'
    chezmoi     = 'chezmoi'
    lazygit     = 'lazygit'
    wt          = 'wt'
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

# ---- Optional: keep a single scoop tool current ------------------------------
# scoop pins to the installed version until `scoop update <pkg>`. This is the
# explicit, consent-gated, idempotent "keep latest" step for ONE tool. We do NOT
# run `scoop update *` -- that would upgrade every scoop tool (taplo, win32yank,
# nerd-fonts, ...) beyond what the caller asked for and break the "run twice = no-op"
# contract. Safe to call when the tool is absent (the install path owns that) and
# when the tool was installed by another manager (the scoop list guard no-ops).
function Update-ScoopTool {
    param([string]$tool)
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) { return }
    if (-not (Test-Tool $tool)) { return }   # not installed yet -> install path owns it
    $pkg = $Catalog[$tool].scoop
    if (-not $pkg) { return }
    # Only update if scoop actually manages this tool (avoids warning on a
    # winget/choco-installed pwsh that Install-One picked when scoop was absent).
    $managed = (scoop list $pkg 2>$null | Select-String -SimpleMatch $pkg)
    if (-not $managed) { return }
    if (-not (Ask "Update ${tool} to the latest scoop version?")) { return }
    if ($DryRun) {
        Write-Host ("  would:    scoop update; scoop update {0}" -f $pkg)
        return
    }
    scoop update | Out-Null               # refresh manifests only
    scoop update $pkg
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("  updated   {0,-26} via scoop" -f $tool)
    } else {
        Write-Warning ("  scoop update of {0} failed (exit {1})" -f $pkg, $LASTEXITCODE)
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
        Add-ScoopBucketSafe -Name 'nerd-fonts' | Out-Null
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
# Set Rose Pine plus Hack Nerd Font settings in VS Code user settings.
# The theme label has an accented e. Build it with [char]0xE9 at runtime so
# this file stays pure ASCII for Windows PowerShell 5.1.
function Get-VSCodeSettingsSpec {
    $theme = "Ros$([char]0xE9) Pine"
    $font = "'Hack Nerd Font', Consolas, monospace"
    return @(
        [pscustomobject]@{ Key = 'workbench.colorTheme'; Value = $theme },
        [pscustomobject]@{ Key = 'editor.fontFamily'; Value = $font },
        [pscustomobject]@{ Key = 'terminal.integrated.fontFamily'; Value = $font }
    )
}

function ConvertTo-JsonStringLiteral {
    param([Parameter(Mandatory)][string]$Value)
    return '"' + $Value.Replace('\', '\\').Replace('"', '\"') + '"'
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
                                Value = (ConvertTo-JsonStringLiteral -Value $spec.Value)
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
            $line = '  "' + $missing[$m].Key + '": ' + (ConvertTo-JsonStringLiteral -Value $missing[$m].Value)
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

function Set-VSCodeTheme {
    param([string]$SettingsPath)
    if ([string]::IsNullOrWhiteSpace($SettingsPath)) {
        $SettingsPath = Join-Path $env:APPDATA "Code\User\settings.json"
    }
    $dir = Split-Path -Parent $SettingsPath
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $utf8 = [System.Text.UTF8Encoding]::new($false)   # no BOM

    $raw = if (Test-Path -LiteralPath $SettingsPath) { Get-Content -Raw -LiteralPath $SettingsPath -ErrorAction SilentlyContinue } else { $null }
    if ([string]::IsNullOrWhiteSpace($raw)) {
        $specs = @(Get-VSCodeSettingsSpec)
        $lines = @()
        for ($i = 0; $i -lt $specs.Count; $i++) {
            $comma = if ($i -lt ($specs.Count - 1)) { ',' } else { '' }
            $lines += ('  "' + $specs[$i].Key + '": ' + (ConvertTo-JsonStringLiteral -Value $specs[$i].Value) + $comma)
        }
        $json = "{`r`n" + ($lines -join "`r`n") + "`r`n}`r`n"
        [System.IO.File]::WriteAllText($SettingsPath, $json, $utf8)
        Write-Host ("  set       {0,-26} theme and fonts (new settings.json)" -f "rose-pine (vscode)")
        return
    }
    try {
        $obj = $raw | ConvertFrom-Json -ErrorAction Stop
        if ($null -eq $obj -or $obj -is [System.Array]) {
            throw "settings.json root must be an object"
        }
        foreach ($spec in @(Get-VSCodeSettingsSpec)) {
            $obj | Add-Member -NotePropertyName $spec.Key -NotePropertyValue $spec.Value -Force
        }
        [System.IO.File]::WriteAllText($SettingsPath, ($obj | ConvertTo-Json -Depth 100), $utf8)
        Write-Host ("  set       {0,-26} theme and fonts (merged)" -f "rose-pine (vscode)")
    } catch {
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
        Write-Host "  would: Add-ScoopBucketSafe psmux; scoop install psmux  (fallback: winget / choco)"
        return
    }
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        if (Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux') {
            scoop install psmux
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
Install-One fzf
Install-One chezmoi
Install-One lazygit

Section "prompt"
Install-One starship

Section "terminal host"
Install-One wt

Section "terminal multiplexer (psmux: tmux for native Windows, optional)"
Install-Psmux

Section "modern shell (optional, you can stay on Windows PowerShell 5.1)"
Install-One pwsh
Update-ScoopTool pwsh
Install-PSFzf

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
Write-Host "            Use Windows Terminal (setup applies the rose-pine"
Write-Host "            fragment by default) or WezTerm for now."

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
