# Greenfield test runbook (copy-paste)

A literal step-by-step for validating a clean install in a throwaway machine.
Three environments are covered: **Windows Sandbox**, a **`tart` macOS VM**, and a
**`tart` Linux VM** (both `tart` guests run on Apple Silicon).

What is automated vs manual:

- **Scripted** (no eyeballs): spin-up + `setup` + `validate.{sh,ps1}` assert the
  install (tools on PATH, Neovim >= 0.12, managed configs match the repo, Mason,
  `chezmoi verify`). See the launchers in this directory.
- **Manual** (you must LOOK): the visual/interactive behaviour — colours, glyphs,
  predictions, "does psmux freeze", VS Code theme/font. That is the checklist in
  Part 3 below. There is no way to script "is this glyph a tofu box".

> Default target: these steps validate the checkout/ref you intentionally select.
> For PR validation, use the commit/ref notes in each environment, then record the
> result in
> [LEDGER.md](LEDGER.md).

---

## Part 1 - spin up + install

### Windows Sandbox

The `.wsb` maps a local checkout read-only, then does spin-up + install +
automated validation for you. First put the host checkout at the exact commit
you intend to test:

```powershell
git fetch origin pull/<PR>/head
git checkout --detach <full-40-character-sha>
git rev-parse HEAD
```

Then **double-click `tests\greenfield\windows-sandbox.wsb`** (or from a
PowerShell on the host: `explorer <path>\windows-sandbox.wsb`). The checked-in
relative `HostFolder` maps the repo root when launched from this file on Windows
builds that support relative Sandbox mappings. If your host does not support
that, edit `HostFolder` to the absolute path of the checkout already at the
commit/ref you intend to test.

The Sandbox boots clean, its logon command runs the checked-out local
`sandbox-run.ps1`, **enables Developer Mode + reduces Defender scanning for you**
(one UAC prompt -- click **Yes**), copies the mapped repo to
`%USERPROFILE%\dotfiles`, runs `setup.ps1 -All`, then `validate.ps1`. Watch the
auto-opened PowerShell window for `PASS:` lines and a final summary; logs are on
the sandbox desktop.

Supply-chain boundary: the `.wsb` path does not download raw `main`, does not
use `[scriptblock]::Create`, and does not execute a remote script. It executes
the local checkout you mapped intentionally.

That single UAC prompt is the ONLY thing you click. You do NOT need to open
Settings. (Background: the Neovim config is a symlink, which Windows only allows
with Developer Mode on; the script flips that one registry key elevated, then
runs setup non-elevated so Scoop still works.)

Optional self-contained path: after installing Git inside the Sandbox, run
`tests\greenfield\sandbox-bootstrap.ps1 -CommitSha <full-40-character-sha>`.
That helper fetches the exact commit, verifies `git rev-parse HEAD`, then runs
the checked-out local `sandbox-run.ps1`. For more RAM, see "Speeding up the
Sandbox" below.

Then do Part 3 inside the sandbox (it is a full Windows desktop: open Windows
Terminal, VS Code, psmux).

#### Manual alternative (no `.wsb`)

If you would rather drive it by hand -- open Windows Sandbox yourself, make Git
available first, then in an **admin** PowerShell run:

```powershell
# 1. PS 5.1 in the Sandbox defaults to old TLS; force 1.2 and allow scripts.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Set-ExecutionPolicy -Scope Process Bypass -Force

# 2. Clone/fetch the exact full commit SHA you intend to test, then verify it.
git clone https://github.com/luisgui1757/dotfiles "$env:USERPROFILE\dotfiles"
Set-Location "$env:USERPROFILE\dotfiles"
git fetch --depth 1 origin <full-40-character-sha>
git checkout --detach FETCH_HEAD
if ((git rev-parse HEAD).Trim() -ne '<full-40-character-sha>') {
    throw 'checked-out commit did not match the requested SHA'
}

# 3. Install + apply everything. Admin is fine HERE (disposable sandbox): the
#    same path the elevated windows-2025 CI runner uses -- Install-Scoop detects
#    elevation and bootstraps Scoop with -RunAsAdmin, and the elevated token
#    creates the Neovim symlink so you do NOT need Developer Mode.
.\setup.ps1 -All

# 4. Auto-check the install, then do the Part 3 visual checklist.
.\tests\greenfield\validate.ps1
```

Caveat: the admin path is fine because the sandbox is throwaway. On your REAL
machine prefer Developer Mode + a normal (non-admin) PowerShell, so Scoop is
owned by your user rather than admin.

#### Speeding up the Sandbox

Installs feel slow mostly because Windows Defender scans every file Scoop
extracts, and the sandbox boots with little RAM. The sandbox is NOT CPU-limited
(it uses all host cores), so the wins are:

1. **More RAM** (host side, before launch): edit `windows-sandbox.wsb` and
   uncomment/raise `<MemoryInMB>` to e.g. `16384` (16 GB) or `32768` (32 GB) on a
   high-RAM host. The `.wsb` ships it commented so it never breaks a low-RAM host.
2. **Turn off Defender real-time scanning** (the single biggest install speedup).
   The `.wsb` path does this for you (one UAC prompt). On the MANUAL admin path,
   run this BEFORE the install block:

   ```powershell
   Set-MpPreference -DisableRealtimeMonitoring $true -ErrorAction SilentlyContinue
   Add-MpPreference -ExclusionPath "$env:USERPROFILE\scoop","$env:USERPROFILE\dotfiles","$env:TEMP","$env:LOCALAPPDATA" -ErrorAction SilentlyContinue
   ```

Slow DOWNLOADS in the sandbox are usually the large one-time payloads (pwsh is
~111 MB, python ~29 MB) plus Defender scanning the stream -- step 2 helps those
too. The host network is shared as-is, so this is not a dotfiles issue.

### tart macOS VM

```bash
brew install cirruslabs/cli/tart
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest dotfiles-macos
tart run dotfiles-macos &        # opens a GUI window - use it for the visual checks
# default creds for the cirruslabs image are admin / admin
ssh admin@$(tart ip dotfiles-macos)
```

Inside the VM:

```bash
git clone https://github.com/luisgui1757/dotfiles ~/dotfiles
cd ~/dotfiles
git fetch --depth 1 origin <full-40-character-sha>
git checkout --detach FETCH_HEAD
test "$(git rev-parse HEAD)" = "<full-40-character-sha>"
./setup.sh --all
./tests/greenfield/validate.sh
```

For PR validation, use the PR head commit SHA. For main validation, use the
current full `origin/main` SHA. Do not validate a moving branch name.
Do the GUI/visual parts of Part 3 in the `tart run` window (VS Code, terminal
colours); the CLI parts over SSH are fine too.

When done: `tart stop dotfiles-macos && tart delete dotfiles-macos`.

### tart Linux VM

```bash
brew install cirruslabs/cli/tart
# Pull/boot an arm64 Linux guest (use a cirruslabs Linux image or build from an
# Ubuntu 24.04 arm64 ISO with `tart create --linux`), then:
tart run dotfiles-linux &
ssh <user>@$(tart ip dotfiles-linux)
```

Inside the VM:

```bash
git clone https://github.com/luisgui1757/dotfiles ~/dotfiles
cd ~/dotfiles
git fetch --depth 1 origin <full-40-character-sha>
git checkout --detach FETCH_HEAD
test "$(git rev-parse HEAD)" = "<full-40-character-sha>"
./setup.sh --all
./tests/greenfield/validate.sh
```

For PR validation, use the PR head commit SHA. For main validation, use the
current full `origin/main` SHA. Do not validate a moving branch name.
A `tart` Linux guest is a real desktop (unlike WSL), so Ghostty + fonts install
natively here and the visual checks apply. Over a headless SSH session the CLI
checks (tmux/nvim/lazygit/shell) still apply; skip the GUI rows.

When done: `tart stop dotfiles-linux && tart delete dotfiles-linux`.

---

## Part 2 - automated validation (all environments)

`validate.{sh,ps1}` already ran at the end of each install above. To re-run it
standalone:

```bash
./tests/greenfield/validate.sh             # POSIX (macOS / Linux)
```

```powershell
.\tests\greenfield\validate.ps1            # Windows
```

Expect every line `PASS:` and a final `SUMMARY: N passed, 0 failed`. Any `FAIL:`
names the exact assertion that broke.

---

## Part 3 - manual interactive checklist

Open a FRESH terminal first (so the new shell config + login shell are active).
Each row says what to run and what you should SEE. The right column is the fix it
proves.

### Shell + Starship prompt

| Run | Expect | Proves |
|-----|--------|--------|
| open a new terminal | Starship prompt renders, Rose Pine colours, no `[]`/tofu boxes | prompt loads |
| `cd ~/Downloads` then `cd ~` | folder shows a download glyph (icon + name), full path shown | directory substitutions |
| `cd` into any repo | git branch glyph + status segment render (no literal `` boxes) | **starship dev-icons restored + glyph-verified** |
| `ls -la` a dir that has normal, world-writable, and sticky sub-folders | directory names are **gold**, long-list metadata follows Rose Pine, not blue-on-white or green-background fallback | **`lsd` config + `$PSStyle`/LS_COLORS dir colour** |
| type `cd Doc` and pause (Windows) | a greyed **prediction** appears in rose/gold and is readable | **PSReadLine prediction colours** |
| press `Ctrl+R`, type a few chars | a fuzzy history picker opens (fzf / PSFzf) | **fzf + PSFzf unified** |

### tmux / psmux

| Run | Expect | Proves |
|-----|--------|--------|
| `tmux` (macOS/Linux) or launch **psmux** (Windows) | a pane appears **immediately**, fully rendered, Rose Pine status bar is at the top with matching rounded pill segments (session/window list/directory; no user/date-time duplication), and empty bar space follows terminal transparency; **no config warnings, no freeze, normal CPU** | **psmux warm-session guard + synchronous generated theme load** |
| `C-b %` / `C-b "` then `C-b h/j/k/l` | split and move between panes | pane bindings |
| `C-b H` / `C-b L` | current window swaps left / right | uppercase window-swap binding |
| enter copy-mode (`C-b [`), `v` to select, `y` | text copies to the system clipboard (paste elsewhere) | clipboard (pbcopy/xclip/win32yank on POSIX, `clip.exe`/OSC52 on Windows) |
| (Windows) open several psmux panes, check Task Manager | no runaway `conhost.exe` pile-up, CPU idle | freeze cascade gone |

### Neovim

| Run | Expect | Proves |
|-----|--------|--------|
| `nvim` | opens fast, Rose Pine theme, no error popups | startup + theme |
| `:Lazy` | all plugins **installed** (green), none failed; `q` to close | plugin sync |
| `:Mason` | LSP servers + formatters installed (lua-language-server, stylua, ...) | Mason sync |
| `:checkhealth` | no **critical** errors (warnings about optional tools are fine) | health |
| open a `.lua` file, edit, `:w` | LSP attaches; file is auto-formatted on save (stylua) | conform format-on-save |
| `:WNF` on a dirty buffer | writes WITHOUT formatting that one time | `:WNF` |

### lazygit

| Run | Expect | Proves |
|-----|--------|--------|
| in a git repo, `lazygit` | Rose Pine UI loads (config picked up) | lazygit config path |
| focus the **Commits** panel, press `J` / `K` | the selected commit moves **down / up** | uppercase J/K move-commit binding |

### VS Code (Windows + macOS GUI; Linux GUI if not headless)

| Run | Expect | Proves |
|-----|--------|--------|
| `code .` | **Rosé Pine** theme is active | VS Code theme set (even on pre-existing JSONC settings) |
| look at the editor text | renders in **Hack Nerd Font** (ligatures/glyphs, not a fallback) | `editor.fontFamily` |
| open the integrated terminal (`Ctrl+\``) | Starship prompt + Nerd Font glyphs render (no tofu boxes); predictions coloured | `terminal.integrated.fontFamily` |
| if you had a settings.json with comments | your comments are still there (a `settings.json.bak.*` exists) | **JSONC-safe edit** |

### Windows Terminal (Windows only)

WT first tries scoop/winget/choco. In **Windows Sandbox** those MSIX-backed
installs fail because Sandbox cannot register MSIX, so `install-deps.ps1`
falls back to the pinned portable build. The `.wsb` path also keeps the
idempotent portable helper as a safety net; on the manual path you can re-run it
yourself:

```powershell
.\tests\greenfield\install-wt-portable.ps1 -Launch
```

| Run | Expect | Proves |
|-----|--------|--------|
| launch Windows Terminal fresh | opens **maximized** (NOT fullscreen) | `launchMode: maximized` |
| look at the right edge | **scrollbar is visible** | `scrollbarState: visible` |
| general look | Rose Pine scheme, Hack Nerd Font, tab styling | WT fragment merge |

---

## Re-running clean

- Windows Sandbox: just close it - everything resets.
- `tart`: `tart delete <name>` and re-clone, or snapshot before install.
- In place (not a clean OS): `./uninstall.sh --all` / `.\uninstall.ps1 -All`
  restores the pre-install backups, then re-run setup.

After each completed run, append the environment, branch/SHA, command, result,
and any skipped manual rows to [LEDGER.md](LEDGER.md).
