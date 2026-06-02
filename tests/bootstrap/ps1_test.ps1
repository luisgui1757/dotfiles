BeforeAll {
    $script:RepoRoot = (Resolve-Path "$PSScriptRoot/../..").Path
    $script:Bootstrap = Join-Path $script:RepoRoot "bootstrap.ps1"
}

Describe "bootstrap.ps1" {

    BeforeEach {
        $script:FakeHome = Join-Path ([System.IO.Path]::GetTempPath()) ("bs-" + [System.Guid]::NewGuid())
        # LOCALAPPDATA is where lazygit reads its config from (v0.58+), and
        # where bootstrap.ps1 symlinks lazygit/config.yml. Keep it under the
        # FakeHome so the test sandboxes the symlink.
        $script:FakeLocalAppData = Join-Path $script:FakeHome "AppData/Local"
        $script:FakeAppData = Join-Path $script:FakeHome "AppData/Roaming"
        New-Item -ItemType Directory -Force -Path $script:FakeHome | Out-Null
        New-Item -ItemType Directory -Force -Path $script:FakeLocalAppData | Out-Null
        New-Item -ItemType Directory -Force -Path $script:FakeAppData | Out-Null

        $script:OldUserProfile  = $env:USERPROFILE
        $script:OldLocalAppData = $env:LOCALAPPDATA
        $script:OldAppData      = $env:APPDATA
        $env:USERPROFILE = $script:FakeHome
        $env:LOCALAPPDATA = $script:FakeLocalAppData
        $env:APPDATA = $script:FakeAppData
    }

    AfterEach {
        $env:USERPROFILE = $script:OldUserProfile
        $env:LOCALAPPDATA = $script:OldLocalAppData
        $env:APPDATA = $script:OldAppData
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:FakeHome
    }

    It "fresh install creates the expected symlinks" {
        & $script:Bootstrap | Out-Null
        $nvim = Get-Item (Join-Path $env:LOCALAPPDATA 'nvim')
        $nvim.LinkType | Should -Be 'SymbolicLink'
        # tmux.conf -> the shared tmux config; psmux reads it on the Windows side
        $tmux = Get-Item (Join-Path $env:USERPROFILE '.tmux.conf')
        $tmux.LinkType | Should -Be 'SymbolicLink'
        $tmux.Target  | Should -Match 'tmux\\tmux\.conf$'
        # tmux.windows.conf -> psmux-only overlay (default-shell pwsh,
        # allow-predictions on, mouse-selection off). Main tmux.conf sources
        # it with `-q`, so the overlay being absent on Unix is silent.
        $tmuxWin = Get-Item (Join-Path $env:USERPROFILE '.tmux.windows.conf')
        $tmuxWin.LinkType | Should -Be 'SymbolicLink'
        $tmuxWin.Target  | Should -Match 'tmux\\tmux\.windows\.conf$'
        # lazygit config -- symlinked into %LOCALAPPDATA%\lazygit (where
        # lazygit v0.58 actually reads from, NOT %APPDATA%). Binds
        # move-commit to uppercase J/K.
        $lazy = Get-Item (Join-Path $env:LOCALAPPDATA 'lazygit/config.yml')
        $lazy.LinkType | Should -Be 'SymbolicLink'
        $lazy.Target  | Should -Match 'lazygit\\config\.yml$'
    }

    It "re-running is idempotent (no new backups)" {
        & $script:Bootstrap | Out-Null
        & $script:Bootstrap | Out-Null
        $backups = Get-ChildItem -Recurse -Force -Path $script:FakeHome -Filter "*.bak.*" -ErrorAction SilentlyContinue
        $backups.Count | Should -Be 0
    }

    It "non-symlink at target is backed up" {
        $profileDir = Split-Path $PROFILE
        New-Item -ItemType Directory -Force -Path $profileDir | Out-Null
        Set-Content -LiteralPath $PROFILE -Value "# user content" -Encoding UTF8
        & $script:Bootstrap | Out-Null
        (Get-Item $PROFILE).LinkType | Should -Be 'SymbolicLink'
        $bak = Get-ChildItem -Path $profileDir -Filter "*.bak.*" | Select-Object -First 1
        $bak | Should -Not -BeNullOrEmpty
        (Get-Content -LiteralPath $bak.FullName) | Should -Be "# user content"
    }

    It "broken symlink at target is backed up and replaced" {
        $dest = Join-Path $env:LOCALAPPDATA 'nvim'
        $missing = Join-Path $script:FakeHome 'missing-nvim-target'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dest) | Out-Null
        New-Item -ItemType SymbolicLink -Path $dest -Target $missing | Out-Null

        & $script:Bootstrap | Out-Null

        $nvim = Get-Item -LiteralPath $dest -Force
        $nvim.LinkType | Should -Be 'SymbolicLink'
        $nvim.Target | Should -Match 'nvim$'
        $bak = Get-ChildItem -Path (Split-Path -Parent $dest) -Filter "nvim.bak.*" | Select-Object -First 1
        $bak | Should -Not -BeNullOrEmpty
    }

    It "warns when pwsh is missing for the psmux overlay" {
        Mock -CommandName Get-Command -MockWith { $null } -ParameterFilter { $Name -eq 'pwsh' }

        $out = & $script:Bootstrap 6>&1

        ($out | Out-String) | Should -Match 'psmux will fail to spawn panes without pwsh'
    }

    It "-DryRun changes nothing" {
        & $script:Bootstrap -DryRun | Out-Null
        Test-Path (Join-Path $env:LOCALAPPDATA 'nvim') | Should -Be $false
    }

    It "Windows PowerShell 5.1 can link a profile under Documents" -Skip:(-not (Get-Command powershell.exe -ErrorAction SilentlyContinue)) {
        $probeScript = Join-Path ([System.IO.Path]::GetTempPath()) ("bs-ps5-profile-" + [System.Guid]::NewGuid() + ".ps1")
        $repoLiteral = $script:RepoRoot.Replace("'", "''")

        @"
`$ErrorActionPreference = 'Stop'
`$repo = '$repoLiteral'
`$fakeHome = Join-Path ([System.IO.Path]::GetTempPath()) ('bs-native-' + [System.Guid]::NewGuid())
`$fakeLocal = Join-Path `$fakeHome 'AppData\Local'
`$fakeRoaming = Join-Path `$fakeHome 'AppData\Roaming'
`$docs = [Environment]::GetFolderPath('MyDocuments')
`$profileDir = Join-Path `$docs 'WindowsPowerShell'
`$probeProfile = Join-Path `$profileDir ('codex-bootstrap-profile-probe-' + [System.Guid]::NewGuid() + '.ps1')

try {
    New-Item -ItemType Directory -Force -Path `$fakeLocal,`$fakeRoaming,`$profileDir | Out-Null
    `$env:USERPROFILE = `$fakeHome
    `$env:LOCALAPPDATA = `$fakeLocal
    `$env:APPDATA = `$fakeRoaming
    `$PROFILE = `$probeProfile

    & (Join-Path `$repo 'bootstrap.ps1') | Out-Null

    `$item = Get-Item -LiteralPath `$probeProfile -Force
    if (`$item.LinkType -ne 'SymbolicLink') {
        throw "profile probe was not a symlink: `$(`$item.LinkType)"
    }

    `$target = @(`$item.Target)[0]
    if (`$target -notmatch 'shells\\powershell_profile\.ps1$') {
        throw "profile probe pointed at unexpected target: `$target"
    }
} finally {
    Remove-Item -LiteralPath `$probeProfile -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath `$fakeHome -Recurse -Force -ErrorAction SilentlyContinue
}
"@ | Set-Content -LiteralPath $probeScript -Encoding UTF8

        try {
            & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $probeScript
            $LASTEXITCODE | Should -Be 0
        } finally {
            Remove-Item -LiteralPath $probeScript -Force -ErrorAction SilentlyContinue
        }
    }
}

Describe "bootstrap.ps1 -MergeWindowsTerminal" {

    BeforeEach {
        $script:FakeHome = Join-Path ([System.IO.Path]::GetTempPath()) ("bs-wt-" + [System.Guid]::NewGuid())
        $script:FakeLocalAppData = Join-Path $script:FakeHome "AppData/Local"
        # LOCALAPPDATA -- bootstrap.ps1 always runs the link block, which
        # symlinks lazygit\config.yml under %LOCALAPPDATA%. Confine that to FakeHome
        # so -MergeWindowsTerminal runs do not pollute the real user profile.
        $script:FakeAppData = Join-Path $script:FakeHome "AppData/Roaming"
        $script:WTPackageDir = Join-Path $script:FakeLocalAppData "Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState"
        New-Item -ItemType Directory -Force -Path $script:WTPackageDir | Out-Null
        New-Item -ItemType Directory -Force -Path $script:FakeAppData | Out-Null
        $env:LOCALAPPDATA = $script:FakeLocalAppData
        $env:USERPROFILE = $script:FakeHome
        $env:APPDATA = $script:FakeAppData
        $script:WTSettings = Join-Path $script:WTPackageDir "settings.json"
    }

    AfterEach {
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $script:FakeHome
    }

    It "preserves the user's discovered profile list" {
        # Seed a minimal but realistic settings.json -- what WT auto-generates.
        @"
{
    "profiles": {
        "defaults": { "fontSize": 10 },
        "list": [
            { "guid": "{aaa}", "name": "Existing WSL", "source": "Windows.Terminal.Wsl" }
        ]
    },
    "actions": [],
    "schemes": [],
    "themes": []
}
"@ | Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        & $script:Bootstrap -MergeWindowsTerminal | Out-Null

        $merged = Get-Content -Raw -LiteralPath $script:WTSettings | ConvertFrom-Json
        $merged.profiles.list.Count | Should -Be 1
        $merged.profiles.list[0].name | Should -Be 'Existing WSL'
    }

    It "creates a *.bak.<timestamp> backup of the pre-merge settings" {
        '{"profiles":{"defaults":{},"list":[]},"actions":[],"schemes":[],"themes":[]}' |
            Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        & $script:Bootstrap -MergeWindowsTerminal | Out-Null

        $bak = Get-ChildItem -Path $script:WTPackageDir -Filter "settings.json.bak.*" | Select-Object -First 1
        $bak | Should -Not -BeNullOrEmpty
    }

    It "applies the rose-pine theme and scheme from the fragment" {
        '{"profiles":{"defaults":{},"list":[]},"actions":[],"schemes":[],"themes":[]}' |
            Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        & $script:Bootstrap -MergeWindowsTerminal | Out-Null

        $merged = Get-Content -Raw -LiteralPath $script:WTSettings | ConvertFrom-Json
        $merged.theme | Should -Be 'rose-pine'
        ($merged.schemes | Where-Object { $_.name -eq 'rose-pine' }) | Should -Not -BeNullOrEmpty
        ($merged.themes | Where-Object { $_.name -eq 'rose-pine' }) | Should -Not -BeNullOrEmpty
    }

    It "tolerates a settings.json missing the profiles object" {
        '{"actions":[]}' | Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        { & $script:Bootstrap -MergeWindowsTerminal } | Should -Not -Throw
        $merged = Get-Content -Raw -LiteralPath $script:WTSettings | ConvertFrom-Json
        $merged.profiles.defaults | Should -Not -BeNullOrEmpty
    }

    It "preserves custom Windows Terminal actions, schemes, and themes" {
        @"
{
    "profiles": { "defaults": {}, "list": [] },
    "actions": [
        { "command": "openSettings", "keys": "ctrl+comma" },
        { "command": "oldCopy", "keys": "ctrl+c" }
    ],
    "schemes": [
        { "name": "custom-scheme", "background": "#000000" },
        { "name": "rose-pine", "background": "#ffffff" }
    ],
    "themes": [
        { "name": "custom-theme", "tab": { "background": "#000000" } },
        { "name": "rose-pine", "tab": { "background": "#ffffff" } }
    ]
}
"@ | Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        & $script:Bootstrap -MergeWindowsTerminal | Out-Null

        $merged = Get-Content -Raw -LiteralPath $script:WTSettings | ConvertFrom-Json
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+comma' }) | Should -Not -BeNullOrEmpty
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+c' }).command.action | Should -Be 'copy'
        ($merged.schemes | Where-Object { $_.name -eq 'custom-scheme' }) | Should -Not -BeNullOrEmpty
        ($merged.schemes | Where-Object { $_.name -eq 'rose-pine' }).background | Should -Be '#191724'
        ($merged.themes | Where-Object { $_.name -eq 'custom-theme' }) | Should -Not -BeNullOrEmpty
        ($merged.themes | Where-Object { $_.name -eq 'rose-pine' }).tab.background | Should -Be '#191724ff'
    }

    It "replaces action arrays when any key overlaps the fragment" {
        @"
{
    "profiles": { "defaults": {}, "list": [] },
    "actions": [
        { "command": "oldCopy", "keys": ["ctrl+c", "alt+c"] },
        { "command": "userOnly", "keys": ["ctrl+u"] }
    ],
    "schemes": [],
    "themes": []
}
"@ | Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        & $script:Bootstrap -MergeWindowsTerminal | Out-Null

        $merged = Get-Content -Raw -LiteralPath $script:WTSettings | ConvertFrom-Json
        ($merged.actions | Where-Object { $_.command -eq 'oldCopy' }) | Should -BeNullOrEmpty
        ($merged.actions | Where-Object { $_.keys -eq 'alt+c' }) | Should -BeNullOrEmpty
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+c' }).command.action | Should -Be 'copy'
        ($merged.actions | Where-Object { $_.command -eq 'userOnly' }).keys | Should -Be 'ctrl+u'
    }

    It "replaces mixed scalar and array action overlaps while preserving unrelated actions" {
        @"
{
    "profiles": { "defaults": {}, "list": [] },
    "actions": [
        { "command": "oldCopy", "keys": "ctrl+c" },
        { "command": "oldPaste", "keys": ["ctrl+v", "alt+v"] },
        { "command": "userOnly", "keys": "ctrl+u" }
    ],
    "schemes": [],
    "themes": []
}
"@ | Set-Content -LiteralPath $script:WTSettings -Encoding UTF8

        & $script:Bootstrap -MergeWindowsTerminal | Out-Null

        $merged = Get-Content -Raw -LiteralPath $script:WTSettings | ConvertFrom-Json
        ($merged.actions | Where-Object { $_.command -eq 'oldCopy' }) | Should -BeNullOrEmpty
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+c' }).command.action | Should -Be 'copy'
        ($merged.actions | Where-Object { $_.command -eq 'oldPaste' }) | Should -BeNullOrEmpty
        ($merged.actions | Where-Object { $_.keys -eq 'alt+v' }) | Should -BeNullOrEmpty
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+v' }).command | Should -Be 'paste'
        ($merged.actions | Where-Object { $_.command -eq 'userOnly' }).keys | Should -Be 'ctrl+u'
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+shift+f' }).command | Should -Be 'find'
        ($merged.actions | Where-Object { $_.keys -eq 'ctrl+t' }).command.action | Should -Be 'newTab'
    }
}

# The native CreateSymbolicLink fallback only exists on Windows; on non-Windows
# pwsh the P/Invoke would throw at call time, so skip the whole block there.
Describe "bootstrap.ps1 native symlink fallback" -Skip:($env:OS -ne 'Windows_NT') {

    BeforeAll {
        # Load the functions without running the symlink phase (test seam).
        $env:DOTFILES_BOOTSTRAP_SOURCE_ONLY = '1'
        try { . $script:Bootstrap } finally {
            Remove-Item Env:DOTFILES_BOOTSTRAP_SOURCE_ONLY -ErrorAction SilentlyContinue
        }
    }

    It "New-NativeSymLink links a file via the kernel32 API" {
        $dir = Join-Path ([System.IO.Path]::GetTempPath()) ("bs-native-file-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        try {
            $target = Join-Path $dir 'target.txt'
            Set-Content -LiteralPath $target -Value 'hi' -Encoding UTF8
            $link = Join-Path $dir 'link.txt'

            New-NativeSymLink -Source $target -Destination $link

            (Get-Item -LiteralPath $link -Force).LinkType | Should -Be 'SymbolicLink'
            Get-Content -LiteralPath $link | Should -Be 'hi'
        } finally {
            Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $dir
        }
    }

    It "New-NativeSymLink links a directory (sets the directory flag)" {
        $dir = Join-Path ([System.IO.Path]::GetTempPath()) ("bs-native-dir-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path (Join-Path $dir 'src') | Out-Null
        try {
            $target = Join-Path $dir 'src'
            $link = Join-Path $dir 'link'

            New-NativeSymLink -Source $target -Destination $link

            $item = Get-Item -LiteralPath $link -Force
            $item.LinkType | Should -Be 'SymbolicLink'
            $item.PSIsContainer | Should -BeTrue
        } finally {
            Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $dir
        }
    }

    It "New-SymbolicLinkItem falls back to the native API when New-Item fails" {
        $dir = Join-Path ([System.IO.Path]::GetTempPath()) ("bs-fallback-" + [System.Guid]::NewGuid())
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        try {
            $target = Join-Path $dir 'target.txt'
            Set-Content -LiteralPath $target -Value 'fallback' -Encoding UTF8
            $link = Join-Path $dir 'link.txt'

            # Shadow New-Item so the primary path throws like Windows PowerShell
            # 5.1 does for profile links under Documents -- the link can then
            # only appear via the native CreateSymbolicLink branch.
            $script:PrimaryAttempted = $false
            function New-Item {
                $script:PrimaryAttempted = $true
                throw 'simulated New-Item SymbolicLink failure'
            }

            New-SymbolicLinkItem -Source $target -Destination $link

            $script:PrimaryAttempted | Should -BeTrue
            (Get-Item -LiteralPath $link -Force).LinkType | Should -Be 'SymbolicLink'
            Get-Content -LiteralPath $link | Should -Be 'fallback'
        } finally {
            Remove-Item function:New-Item -ErrorAction SilentlyContinue
            Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $dir
        }
    }
}
