# Windows theming fixes - archived work order (pilot migration branch)

> **Temporary handoff doc.** Delete this file once the fixes below land + are
> validated. Written 2026-06-12 by Claude (Opus 4.8), root-cause confirmed by
> Codex 5.5 xhigh (read-only). This exists so the work can be picked up
> autonomously from a fresh session (Windows first, then macOS).
>
> **Implementation status, 2026-06-12:** local code now implements the portable
> WT seed/merge path, psmux RGB overlay, Hack.zip FAIL marker, font-change
> notification, tests, and docs. Keep this handoff file until the Windows
> Sandbox runtime pass validates the behavior.

## How to use this doc

You are (likely) a fresh Claude Code session on a **Windows** machine, picking
up three theming bugs found while greenfield-testing the pilot migration branch in Windows
Sandbox. The diagnosis is done and confirmed; your job is to **implement the
fixes test-first, validate in Sandbox, and keep CI green**. Then the same gets
re-validated on macOS (see [Cross-machine plan](#cross-machine-plan)).

Durable state lives in git, not in any chat - the pilot migration branch and
PR #21 are the source of truth. Read `CLAUDE.md` first (repo invariants), then
this file.

---

## The three symptoms

Observed in a **fresh Windows Sandbox** after `.\setup.ps1 -All` on the pilot migration branch:

1. **Windows Terminal is not Rose Pine** (default theme), and the **PowerShell
   7.6.2 prompt is not Rose Pine** either.
2. The **tmux/psmux status bar** (session / window / date / time) renders in a
   **"terrible green"**.
3. **Nerd Font glyphs do not render** in PowerShell 7.6.2 or in Windows Terminal
   (tofu boxes).

---

## Verdict (root cause)

| ID | Claim | Verdict | Confidence |
|----|-------|---------|-----------|
| **H1** | WT Rose Pine scheme/theme **and** `Hack Nerd Font` face never reach the **portable/unpackaged** WT used in Sandbox | **CONFIRMED** | **0.98** |
| **H2** | Hack Nerd Font may not be **visible** to WT (no `WM_FONTCHANGE` broadcast / restart) | PARTIAL | 0.70 |
| **H3** | tmux truecolor (`:RGB`) isn't applied under psmux → pine `#31748f` degrades to a garish green | PARTIAL (needs Sandbox `$TERM`) | 0.62 |
| PS profile | The PowerShell profile **is** applied and not gated; the "not Rose Pine" look is mostly downstream of H1/H2 | applied | 0.90 |

**H1 single-handedly explains symptom 1 (WT theme) and symptom 3 (tofu in both
WT and PowerShell).** Fix it first. It affects **every portable-WT user**, not
just Sandbox.

### H1 — the failure chain (this is the important one)

Windows Sandbox cannot register MSIX/Store packages, so the **packaged** Windows
Terminal (`%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json`)
is **never installed**. `install-deps.ps1` falls back to a **portable/unpackaged**
WT, which reads `%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json`.

But the repo only ever targets the **packaged** path:

1. The **only** chezmoi WT target is the packaged path:
   `home/AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/modify_settings.json.ps1.tmpl`.
2. That `modify_` script **emits nothing when stdin (the current packaged
   settings.json) is blank** — it deliberately refuses to fabricate a file
   (search for `IsNullOrWhiteSpace($currentJson)` → `return`). In Sandbox the
   packaged file never exists → **chezmoi leaves the packaged settings absent**.
3. `setup.ps1` → `Copy-WindowsTerminalSettingsForUnpackaged` (~`setup.ps1:466-490`)
   is supposed to mirror packaged → unpackaged, but it **returns early when the
   packaged file is absent** (`setup.ps1:476-477`). So the unpackaged WT gets
   **nothing**.
4. `install-deps.ps1`'s `Install-WindowsTerminal` installs the portable build
   but **does not seed its settings.json**. `tests/greenfield/install-wt-portable.ps1`
   (~`:54-68`) also only copies an **existing** packaged file.

**Net:** the only WT present in Sandbox (portable) launches with **defaults** —
no `theme: rose-pine`, no `colorScheme: rose-pine`, no `font.face: "Hack Nerd Font"`.
All three of those DO exist correctly in
`home/.chezmoitemplates/windows-terminal/settings.fragment.jsonc`
(`theme` ~`:29`, `colorScheme` + `font.face` ~`:53-54`, scheme/theme defs
~`:63-110`) — they just never get applied. **PowerShell tofu is downstream**:
PS 7.6.2 renders in whatever font WT is using, and WT is using its default
(non-Nerd) font.

### H2 — font visibility (secondary)

`install-deps.ps1` installs Hack Nerd Font (prefers `scoop install nerd-fonts/Hack-NF`,
falls back to a SHA-256-verified `Hack.zip` → `%LOCALAPPDATA%\Microsoft\Windows\Fonts`
+ HKCU font registry). Issues:

- Detection is **file-presence only**; there's **no `WM_FONTCHANGE` broadcast**,
  and the installer itself warns a terminal restart may be needed.
- A SHA mismatch stops extraction but only **warns** — it does not record a
  `FAIL:` marker.

H1 alone explains the tofu, but once H1 is fixed, confirm the font is actually
selectable by WT (an already-running or freshly-launched WT may not see a
just-registered HKCU font without a refresh).

### H3 — tmux "terrible green"

`tmux/tmux.conf`: `status-style "fg=#31748f,bg=#191724"` (pine on base) is
**correct** (~`:87-92`). Truecolor (`:RGB`) is enabled only via
`terminal-overrides` for `xterm-256color`, `*256col*`, `xterm-ghostty`,
`alacritty` (~`:4-8`). The Windows psmux overlay `tmux/tmux.windows.conf` has
**no RGB/truecolor handling**. If the outer `TERM` under WT + psmux doesn't
match a `:RGB` override, pine `#31748f` **degrades to the nearest 256-color** —
a garish green. **This needs a Sandbox `$TERM` check to confirm** (see below).

### PowerShell profile — applied, not the bug

`home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1` is a real Windows
chezmoi target (Windows mode = `file`, not blocked by `.chezmoiignore`), and it
loads Starship + applies PSReadLine Rose Pine colors and a gold `$PSStyle`
directory color **independently of WT**. So:
- WT background/tab theme = H1.
- Prompt-glyph tofu = H1/H2 (font).
- **But** PSReadLine syntax/prediction colors are NOT downstream of WT — if those
  are *also* missing in Sandbox, that's a separate profile-load issue to verify
  at runtime (check `$PROFILE` exists + sources without error).

---

## Fixes (prioritized, test-first)

### Fix 1 — H1 (do first): make the theme/font reach the portable WT

**Approach (preferred):** teach `Copy-WindowsTerminalSettingsForUnpackaged`
(`setup.ps1`) to **generate/merge the unpackaged settings from
`windows-terminal/settings.fragment.jsonc` when the packaged settings file is
absent AND a portable `wt` is present**. Keep the packaged `modify_` script's
"don't fabricate" behavior unchanged (that's correct for real Store-WT machines).

- Reuse the existing JSONC-comment-aware merge logic that the `modify_` script
  uses (don't duplicate a second JSON merger — factor it or mirror its strip +
  merge). Remember pwsh-7 `ConvertFrom-Json` tolerates `//` comments but PS 5.1
  + jq do not, so strip comments first.
- When the unpackaged settings.json already exists (portable WT launched once),
  **merge** the fragment into it; when absent, **seed** it from the fragment.
- All `.ps1`: pure ASCII, no apostrophes in comment lines, no assignment to
  read-only `$Home`, no `Invoke-Expression`, pass PSScriptAnalyzer at Warning+.

**Rejected alternative:** adding a dedicated unpackaged chezmoi target under
`home/AppData/Local/Microsoft/Windows Terminal/...`. Riskier: a bare
`chezmoi apply` would then touch portable settings even on real Store-WT
machines. Keep the seeding inside `setup.ps1` where the portable-vs-packaged
decision already lives.

**Tests:** add Pester coverage asserting "packaged absent + portable `wt`
present → the **unpackaged** settings.json ends up containing `rose-pine` and
`Hack Nerd Font`" (and the existing "packaged present → merge into packaged"
path still holds). See `tests/powershell/Setup.Tests.ps1`.

### Fix 2 — H2: guarantee Nerd Font visibility

- After font install, **verify** the HKCU registry entry + file exist, and
  **broadcast `WM_FONTCHANGE`** (or explicitly document/trigger a WT restart so
  it re-enumerates fonts).
- Make a SHA-256 mismatch on the direct download record a real **`FAIL:`**
  marker (currently it only warns), so CI/greenfield catches it.
- Add a validator that checks WT can actually **select/render** Hack Nerd Font,
  not just that the file is on disk.

### Fix 3 — H3: tmux truecolor under psmux

- Add a psmux-matching truecolor capability to **`tmux/tmux.windows.conf`**
  (the overlay), most likely `set -as terminal-features ',*:RGB'`.
- Re-test in Sandbox: `psmux show-options -g terminal-features` /
  `terminal-overrides`, plus a visual truecolor + status-bar check.
- If psmux ignores RGB entirely, fall back to mapping the status colors to ANSI
  slots and rely on the now-fixed WT Rose Pine `colorScheme`.
- Respect existing tmux invariants: uppercase `H`/`L` window swaps; explicit
  `window-status-format "#I:#W#F"`; inline `#[fg=...]` for the current window
  (psmux v3.3.4 ignores `window-status-current-style`). See `CLAUDE.md`.

### Fix 4 — docs + tests (doc-discipline)

The docs originally **claimed the unpackaged mirror always happened**, but the
pre-fix code only mirrored when the packaged file existed. The implementation
must keep these corrected:
- `README.md` (~`:51-55`, ~`:221`)
- `windows-terminal/README.md` (~`:80-83`)
- `CLAUDE.md` (~`:405-413`)

And make `tests/greenfield/validate.ps1` assert the **portable** WT settings.json
contains `rose-pine` + `Hack Nerd Font` (so this regression can't return
silently). Update `tests/static/invariants_test.sh` only if you add a new
invariant.

### Working agreement (from CLAUDE.md)

1. Baseline: `.\test.ps1` (Windows) / `make test` (Unix).
2. Make the change. 3. Update tests in the same diff. 4. Update `CLAUDE.md` /
`README.md` if invariants/install path changed. 5. Re-run tests green.
6. Commit. Fix the cause, not the test.

---

## Sandbox verification

### Manual quick checks (run inside the Sandbox)

```powershell
# Did anything reach the portable WT? (H1 predicts: nothing, pre-fix)
Test-Path "$env:LOCALAPPDATA\Microsoft\Windows Terminal\settings.json"
Get-Content "$env:LOCALAPPDATA\Microsoft\Windows Terminal\settings.json" -Raw |
  Select-String 'rose-pine|Hack Nerd'

# H3: what TERM does tmux see? (does it match a :RGB override?)
echo $env:TERM           # in WT directly
# then launch psmux and inside a pane:
echo $env:TERM
psmux show-options -g terminal-features
```

After the fix, the `Select-String` must return the `rose-pine` + `Hack Nerd Font`
lines, WT must open Rose Pine + render glyphs, and the tmux bar must be pine
(teal), not garish green.

### Booting Sandbox from the terminal — yes, with a caveat

Windows Sandbox **can** be launched from the command line:

```powershell
# either of these (the .wsb is associated with WindowsSandbox.exe)
WindowsSandbox.exe C:\path\to\windows-sandbox.wsb
start C:\path\to\tests\greenfield\windows-sandbox.wsb
```

Prereqs: the **"Windows Sandbox" optional feature** must be enabled (Win 11
Pro/Enterprise/Education) and virtualization on in BIOS.

**Caveat for AUTONOMOUS testing (a Claude session reading the result):** the
repo's shipped `tests/greenfield/windows-sandbox.wsb` is **self-contained** — it
downloads the *remote* pilot migration branch and writes logs to the **sandbox
desktop**, which is ephemeral and **not visible to the host**. That's fine for a
human watching the window, but a host-side Claude can't read PASS/FAIL.

For autonomous runs against your **local** fixes, use a `.wsb` that (a) maps the
local repo read-only, (b) maps a host **results** folder read-write, and (c) has
the `LogonCommand` run setup + validate **against the mapped local repo** and
write a result file the host can poll. Template:

```xml
<Configuration>
  <MappedFolders>
    <MappedFolder>
      <HostFolder>C:\path\to\dotfiles</HostFolder>
      <SandboxFolder>C:\dotfiles</SandboxFolder>
      <ReadOnly>true</ReadOnly>
    </MappedFolder>
    <MappedFolder>
      <HostFolder>C:\path\to\results</HostFolder>
      <SandboxFolder>C:\results</SandboxFolder>
      <ReadOnly>false</ReadOnly>
    </MappedFolder>
  </MappedFolders>
  <LogonCommand>
    <Command>powershell -ExecutionPolicy Bypass -Command "Copy-Item C:\dotfiles C:\Users\WDAGUtilityAccount\dotfiles -Recurse; Set-Location C:\Users\WDAGUtilityAccount\dotfiles; .\setup.ps1 -All *&gt; C:\results\setup.log; .\tests\greenfield\validate.ps1 *&gt; C:\results\validate.log; 'DONE' | Out-File C:\results\done.flag"</Command>
  </LogonCommand>
</Configuration>
```

Then the host polls `C:\path\to\results\done.flag`, reads `validate.log` for
`SUMMARY: N passed, 0 failed`, and copies the local repo into the sandbox HOME
first (don't run setup against the read-only mapped copy — symlinks need a
writable target). Build this as a small helper before relying on it; it is the
piece that makes Sandbox a real autonomous test loop.

---

## Cross-machine plan

1. **Windows first:** implement Fixes 1-4 on the pilot migration branch, validate in
   Sandbox (manual, then ideally the autonomous `.wsb` above), keep CI green,
   push. Delete this file when done.
2. **macOS next:** re-validate the non-Windows paths with `tart` —
   `tests/greenfield/macos-greenfield.sh --current-home` inside a
   `ghcr.io/cirruslabs/macos-tahoe-base:latest` VM, and the Linux CLI path in a
   `ghcr.io/cirruslabs/ubuntu:latest` VM. (Default creds `admin`/`admin`.) The
   WT/font fixes are Windows-only, but confirm nothing regressed cross-platform.
3. The macOS/Linux greenfield checks live in `tests/greenfield/`
   (`macos-greenfield.sh`, `validate.sh`, `docker-greenfield.sh`).

---

## File reference (pilot migration branch @ 70eb850; line numbers +/- a few)

- `home/AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/modify_settings.json.ps1.tmpl` — `IsNullOrWhiteSpace → return` (the no-fabricate gate)
- `home/.chezmoitemplates/windows-terminal/settings.fragment.jsonc` — `theme` ~29, `colorScheme` + `font.face` ~53-54, scheme/theme defs ~63-110
- `setup.ps1` — WT paths ~458-463; `Copy-WindowsTerminalSettingsForUnpackaged` ~466-490 (early return ~476-477); mirror invoked post-`chezmoi apply` ~647-656
- `install-deps.ps1` — `Install-WindowsTerminal` ~1095-1152; `Install-HackNerdFont` ~594-672 (SHA verify ~648-650, warn-only catch ~673-675)
- `tests/greenfield/install-wt-portable.ps1` — ~54-68 (copies existing packaged only)
- `tmux/tmux.conf` — truecolor overrides ~4-8; status bar ~87-98
- `tmux/tmux.windows.conf` — psmux overlay (no RGB handling today)
- Docs that originally claimed the mirror always happened: `README.md` ~51-55/221, `windows-terminal/README.md` ~80-83, `CLAUDE.md` ~405-413
