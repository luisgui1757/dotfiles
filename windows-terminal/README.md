# Windows Terminal — merge instructions

Windows Terminal **rewrites** its `settings.json` on every launch with discovered
profiles (PowerShell GUIDs, WSL distros, Azure Cloud Shell, Visual Studio shells,
etc.). A hard symlink either loses those entries or gets clobbered. So instead of
symlinking the file, we keep **only the user-owned keys** in
`settings.fragment.jsonc` and merge them during setup's chezmoi config phase.

## Path

```
%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json
```

(For the WT Preview the package is `Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe`.)

## What's in the fragment

- `actions` — keybindings.
- `profiles.defaults` — font (Hack Nerd Font 12), Rose Pine color scheme,
  acrylic off, padding, antialiasing.
- `schemes[rose-pine]` — the color scheme definition.
- `themes[rose-pine]` — the tab/window theme.
- Top-level: `copyFormatting`, `copyOnSelect`, `initialRows`,
  `useAcrylicInTabRow`, `windowingBehavior`, `firstWindowPreference`.

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

The setup config phase backs up an existing pre-merge `settings.json` as
`settings.json.bak.<timestamp>` before `chezmoi apply`. The chezmoi `modify_`
merge then initializes missing `profiles` containers and preserves custom
`actions`, `schemes`, and `themes`, while entries with the same key or name are
replaced by the repo fragment. A hand-edited top-level `theme` is reset to
`rose-pine` on every run. A bare `chezmoi apply` runs that merge but does not
create setup's backup. If WT has not launched yet and `settings.json` is absent,
chezmoi leaves it absent instead of fabricating one.

For WSL, this is the supported terminal/font path: run
`.\setup.ps1 -All` on Windows, then `./setup.sh --all` inside WSL. The WSL setup
intentionally skips Linux Ghostty and Linux fontconfig fonts unless
`./setup.sh --experimental-wsl-gui` is used.
