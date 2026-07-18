# Windows Terminal — merge instructions

Windows Terminal **rewrites** its `settings.json` on every launch with discovered
profiles (PowerShell GUIDs, WSL distros, Azure Cloud Shell, Visual Studio shells,
etc.). A hard symlink either loses those entries or gets clobbered. So instead of
symlinking the file, we keep **only the user-owned keys** in
`settings.fragment.jsonc` and let `setup.ps1` merge each installation as an
independent user-owned transaction.

## Path

Packaged Store/MSIX Windows Terminal:

```
%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json
```

Preview Store/MSIX Windows Terminal:

```
%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe\LocalState\settings.json
```

Canary App Installer Windows Terminal:

```
%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminalCanary_8wekyb3d8bbwe\LocalState\settings.json
```

Portable unpackaged Windows Terminal:

```
%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json
```

## What's in the fragment

- `actions` — keybindings.
- `profiles.defaults` — font (Hack Nerd Font 12), Rose Pine color scheme,
  acrylic off, padding, antialiasing, `scrollbarState: visible`, and WT's hard
  maximum `historySize` of 32,767 lines.
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
GitHub release zip (`v1.24.11911.0`, x64), verifies its SHA-256 before
extracting, installs it under `%LOCALAPPDATA%\Programs\WindowsTerminal`, and
adds that folder to the current process PATH plus the persistent User PATH.

The settings merge below is separate because Windows Terminal creates and
rewrites its own `settings.json`.

## Merge

```powershell
.\setup.ps1 -SkipDeps -SkipNvim
```

`setup.ps1` merges the fragment by default. Pass
`-SkipWindowsTerminalMerge` to leave `settings.json` byte-identical; the legacy
`-MergeWindowsTerminal` switch is still accepted on `setup.ps1` as a no-op
alias for older commands.

Stable packaged, Preview, Canary, and portable settings are never mirrored.
`scripts/windows-terminal-targets.ps1` is the single validated identity source
used by setup, release migration/recovery, and uninstall. For every existing
MSIX target and every existing/detected portable target, setup reads that
target's own current JSON, merges the fragment, and stages the result beside
the destination. It parses and byte-validates every staged output before
touching any target. Every divergent pre-existing target receives its own verified,
collision-safe `settings.json.bak.<timestamp>[.n]` copy.

Publication uses same-directory atomic replacement. The replacement operation
captures the exact pre-publication bytes in a transient rollback file; setup
compares that file with the staged source identity to detect even a change in
the final check/replace window. If any target fails backup, validation, or
publication, already-published targets roll back as a group. An unsafe rollback
fails setup with the exact persistent/rollback paths needed for manual recovery.
Stages and completed rollback files are cleaned on success and failure, and a
named transaction mutex serializes concurrent setup runs. A missing Store/MSIX
settings file stays absent; a missing portable file is seeded only when portable
WT is actually detected. Repeated setup is a no-op once all selected independent merges
already match.

The merge initializes missing `profiles` containers and preserves custom
profiles, actions, schemes, themes, and custom `defaultProfile`. Entries with
the same managed identity are replaced by the fragment; an empty or legacy
Windows PowerShell default is promoted to the fixed `PowerShell 7` profile.
Bare `chezmoi apply` deliberately has no Windows Terminal target because it
cannot provide setup's backup/concurrency/atomic-publication contract.

`uninstall.ps1 -All` validates backup filenames and JSON for all four canonical
paths before restoring any target. It orders by the filename timestamp plus collision suffix,
never filesystem mtime, atomically restores the selected pre-setup backup, and
preserves the displaced current settings as
`settings.json.uninstall-current.<timestamp>[.n]`. Use
`-NoRestoreBackups` to leave every path untouched.

For WSL, this is the supported terminal/font path: run
`.\setup.ps1 -All` on Windows, then `./setup.sh --all` inside WSL. The WSL setup
intentionally skips Linux Ghostty and Linux fontconfig fonts unless
`./setup.sh --experimental-wsl-gui` is used.
