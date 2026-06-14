# Windows Terminal — merge instructions

Windows Terminal **rewrites** its `settings.json` on every launch with discovered
profiles (PowerShell GUIDs, WSL distros, Azure Cloud Shell, Visual Studio shells,
etc.). A hard symlink either loses those entries or gets clobbered. So instead of
symlinking the file, we keep **only the user-owned keys** in
`settings.fragment.jsonc` and merge them during setup's chezmoi config phase.

## Path

Packaged Store/MSIX Windows Terminal:

```
%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json
```

(For the WT Preview the package is `Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe`.)

Portable unpackaged Windows Terminal:

```
%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json
```

## What's in the fragment

- `actions` — keybindings.
- `profiles.defaults` — font (Hack Nerd Font 12), Rose Pine color scheme,
  acrylic off, padding, antialiasing, `scrollbarState: visible`.
- `profiles.list[PowerShell 7]` — a repo-owned fixed-GUID profile that runs
  `pwsh.exe`.
- `defaultProfile` — the desired PowerShell 7 profile GUID, consumed by the
  repo merge helper.
- `schemes[rose-pine]` — the color scheme definition.
- `themes[rose-pine]` — the tab/window theme.
- Top-level: `copyFormatting`, `copyOnSelect`, `initialRows`,
  `useAcrylicInTabRow`, `windowingBehavior`, `firstWindowPreference`,
  `launchMode` (`maximized` — opens maximized, not fullscreen).

What's intentionally **not** in the fragment: anything WT auto-generates per
machine (WSL distros, VS / Ubuntu / Azure entries, and the dynamic PowerShell 7
profile). A real WT JSON fragment extension can add profiles and schemes, but
`defaultProfile` is a root startup setting in `settings.json`; this repo sets it
in the merge layer instead of relying on fragment-extension behavior.

## Install

`setup.ps1` installs Windows Terminal as the `wt` dependency through the same
Scoop-first Windows catalog as the rest of the toolchain:

```powershell
scoop install extras/windows-terminal
# fallback: winget install --id Microsoft.WindowsTerminal -e
# fallback: choco install microsoft-windows-terminal
```

Those package-manager installs are MSIX-backed. If they fail to register WT or
do not put `wt` on PATH, `install-deps.ps1` falls back to the pinned portable
GitHub release zip (`v1.24.11321.0`, x64), verifies its SHA-256 before
extracting, installs it under `%LOCALAPPDATA%\Programs\WindowsTerminal`, and
adds that folder to the current process PATH plus the persistent User PATH.

The settings merge below is separate because Windows Terminal creates and
rewrites its own `settings.json`.

## Merge

```powershell
.\setup.ps1 -SkipDeps -SkipNvim
```

`setup.ps1` merges the fragment by default via chezmoi. Pass
`-SkipWindowsTerminalMerge` to leave `settings.json` byte-identical; the legacy
`-MergeWindowsTerminal` switch is still accepted on `setup.ps1` as a no-op
alias for older commands.

The setup config phase backs up an existing packaged pre-merge `settings.json`
as `settings.json.bak.<timestamp>` before `chezmoi apply`. The chezmoi
`modify_` merge then initializes missing `profiles` containers and preserves
custom `actions`, `schemes`, and `themes`, while entries with the same key or
name are replaced by the repo fragment. A hand-edited top-level `theme` is reset
to `rose-pine` on every run. The merge adds the fixed `PowerShell 7` profile and
sets `defaultProfile` to it only when the current value is empty or still the
built-in Windows PowerShell 5.1 default; if you chose another default profile,
that choice is preserved. A bare `chezmoi apply` runs that packaged merge but
does not create setup's backup. If packaged WT has not launched yet and
`settings.json` is absent, the packaged `modify_` target leaves it absent
instead of fabricating one.

After a real non-dry-run setup apply, setup handles the portable unpackaged path
too. If the packaged settings file exists, setup best-effort copies the merged
MSIX file to `%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json`. If the
packaged file is absent but portable WT is detected, setup seeds a missing
unpackaged settings file from the fragment or merges the fragment into an
existing unpackaged file. That gives the portable fallback the same Rose Pine,
launch, scrollbar, font, keybinding, and PowerShell 7 default-profile settings.
All portable handling is skipped with `-SkipWindowsTerminalMerge` and never
fails setup.

For WSL, this is the supported terminal/font path: run
`.\setup.ps1 -All` on Windows, then `./setup.sh --all` inside WSL. The WSL setup
intentionally skips Linux Ghostty and Linux fontconfig fonts unless
`./setup.sh --experimental-wsl-gui` is used.
