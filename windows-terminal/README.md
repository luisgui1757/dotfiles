# Windows Terminal — merge instructions

Windows Terminal **rewrites** its `settings.json` on every launch with discovered
profiles (PowerShell GUIDs, WSL distros, Azure Cloud Shell, Visual Studio shells,
etc.). A hard symlink either loses those entries or gets clobbered. So instead of
symlinking the file, we keep **only the user-owned keys** in
`settings.fragment.jsonc` and merge them in on each new machine.

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

## Merge

```powershell
.\bootstrap.ps1 -MergeWindowsTerminal
```

`bootstrap.ps1` backs up the pre-merge `settings.json` as
`settings.json.bak.<timestamp>`, initializes missing `profiles` containers, and
preserves custom `actions`, `schemes`, and `themes`, while entries with the same
key or name are replaced by the repo fragment.

For WSL, this is the supported terminal/font path: run
`.\setup.ps1 -All -MergeWindowsTerminal` on Windows, then `./setup.sh --all`
inside WSL. The WSL setup intentionally skips Linux Ghostty and Linux fontconfig
fonts unless `./setup.sh --experimental-wsl-gui` is used.
