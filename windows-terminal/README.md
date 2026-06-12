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
- `schemes[rose-pine]` — the color scheme definition.
- `themes[rose-pine]` — the tab/window theme.
- Top-level: `copyFormatting`, `copyOnSelect`, `initialRows`,
  `useAcrylicInTabRow`, `windowingBehavior`, `firstWindowPreference`,
  `launchMode` (`maximized` — opens maximized, not fullscreen).

What's intentionally **not** in the fragment: anything WT auto-generates
(`profiles.list[]`, `defaultProfile` GUID, the per-machine VS / Ubuntu / Azure
entries).

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
to `rose-pine` on every run. A bare `chezmoi apply` runs that packaged merge
but does not create setup's backup. If packaged WT has not launched yet and
`settings.json` is absent, the packaged `modify_` target leaves it absent
instead of fabricating one.

After a real non-dry-run setup apply, setup handles the portable unpackaged path
too. If the packaged settings file exists, setup best-effort copies the merged
MSIX file to `%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json`. If the
packaged file is absent but portable WT is detected, setup seeds a missing
unpackaged settings file from the fragment or merges the fragment into an
existing unpackaged file. That gives the portable fallback the same Rose Pine,
launch, scrollbar, font, and keybinding settings. All portable handling is
skipped with `-SkipWindowsTerminalMerge` and never fails setup.

For WSL, this is the supported terminal/font path: run
`.\setup.ps1 -All` on Windows, then `./setup.sh --all` inside WSL. The WSL setup
intentionally skips Linux Ghostty and Linux fontconfig fonts unless
`./setup.sh --experimental-wsl-gui` is used.
