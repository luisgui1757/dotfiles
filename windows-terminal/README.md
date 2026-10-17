# Windows Terminal

Select **Windows Terminal** in `setup.ps1`. The installer installs the pinned
upstream ZIP, managed PowerShell, Hack Nerd Font and a Start Menu shortcut. Use
the same menu to update, check or remove the selection.

The managed unpackaged stable instance uses Windows' observed LocalAppData folder:

```text
%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json
```

Store, Preview and Canary installations keep their independent settings. No
MSIX registration, global PATH edit or portable-mode marker is needed.

## Managed settings

`settings.fragment.jsonc` is the reviewed recipe. The installer merges its fixed
fields into the real JSONC document rather than replacing the whole file:

- Rosé Pine colors and window/tab theme, Hack Nerd Font, padding and scrollback.
- A fixed PowerShell profile pointing to the managed executable.
- Modern keybindings using bundled Terminal actions and one fixed close-tab action.
- Startup/window behavior, copying preferences and initial dimensions.

An existing nonempty `defaultProfile` is preserved, including Windows PowerShell.
An absent or empty default starts the managed PowerShell profile. Later personal
default choices are preserved. Unrelated settings, nested properties, profiles,
comments and line endings survive publication; Terminal may itself reformat its
settings when it opens them.

## Existing settings and removal

Overlapping personal settings require explicit adoption in the preview. The
installer saves the first baseline before publication and restores those owned
values on removal, retaining unrelated later edits. Changes to owned settings
require another explicit choice before update or removal; the original baseline
is retained. Interrupted publication uses the normal recovery menu.

Overlapping legacy combined `actions`/`keys` entries are ambiguous. Preserve the
file, launch the supported unpackaged stable Terminal once so it performs its own
settings migration, then retry. The installer does not reproduce Terminal's
serializer or generated action identifiers.

The upstream release requires Windows 10 build 19041 or newer. The supported
native acceptance runner is Windows Server 2025 x64; actual GUI launch, managed
profile consumption, check/update/removal and personal-setting preservation are
covered by the hosted desktop lifecycle fixture. Current verification status is
tracked in [installer status](../docs/installer-status.md).
