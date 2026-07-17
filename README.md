# Dotfiles

One setup command gives you the same terminal-first working style, shortcuts,
and Rose Pine theme on Apple Silicon macOS, Linux, WSL2, and Windows.

If the repo is already installed, jump to the [cheat sheets](#cheat-sheets).
If this is a new machine, start with [install, update, and remove](#install-update-and-remove).

## What you get

| Area | Tools | What this repo does |
|---|---|---|
| Editor | Neovim, VS Code | Installs the editor stack, plugins, language servers, formatters, parsers, theme, and shortcuts. |
| Multiplexers | Herdr, tmux, psmux | Gives you tabs, panes, workspaces/sessions, restore support, and tmux-shaped keys. `psmux` is the native-Windows tmux equivalent. |
| Terminals | Ghostty, WezTerm, Windows Terminal | Applies Rose Pine, Hack Nerd Font, large scrollback, sensible clipboard behavior, and maximized startup. |
| Shell | zsh, PowerShell 7, Starship | Adds vi command-line editing, history suggestions, fuzzy completion, a shared prompt, and useful aliases. |
| Navigation | fzf, zoxide, lsd, ripgrep, fd | Adds fuzzy history/file/directory pickers, smart `cd`, readable file listings, and fast search. |
| Git | Git, lazygit, GitHub CLI, gh-dash | Installs the CLI tools and applies their shared terminal configuration. |
| Languages | Python, Node.js, Tree-sitter, Mason tools | Installs the base runtimes plus the language servers and formatters used by Neovim. |
| macOS desktop | AeroSpace | Adds keyboard-driven tiling workspaces on macOS. |
| Agent tooling | Pi CLI, Sentinel policy | Installs the supported CLI, selects the audited Rose Pine Pi theme, and applies the global agent-policy block. Local sessions, credentials, and all other Pi preferences stay local. |
| Config management | Nix, nix-darwin/Home Manager, chezmoi | Reconciles POSIX packages and applies the repo-owned config files. Windows uses native package managers plus chezmoi. You do not need to run these layers by hand. |

The main configuration is intentionally consistent: dark Rose Pine, Hack Nerd
Font, vi-style navigation, `Ctrl+B` as the multiplexer prefix, and the same
shell helpers wherever the platform supports them.

## Platform support

| Platform | Supported path | Important note |
|---|---|---|
| Apple Silicon macOS | `./setup.sh --all` | Fully supported. AeroSpace is macOS-only and needs one manual Accessibility grant. |
| Native Linux | `./setup.sh --all` | Supports Homebrew/Linuxbrew or `apt`, `dnf`, `pacman`, `zypper`, and `apk`. GUI app availability still depends on the distro and architecture. |
| WSL2 | Windows `setup.ps1` plus WSL `setup.sh` | Windows owns Windows Terminal, fonts, and host clipboard tools. WSL owns the Linux shell/editor stack. Linux GUI terminals are opt-in experiments. |
| Native Windows | `.\setup.ps1 -All` | Use PowerShell 7. Enable Developer Mode first so setup can create the required config links without running the whole install as Administrator. Herdr for Windows is a pinned preview build. |

## Install, update, and remove

Run setup from a local checkout of an exact release. Setup is safe to rerun.

```bash
# macOS, Linux, or WSL
cd ~/dotfiles
./setup.sh --all
```

Testing a clean, pushed branch from this official repository before it has a
release? Run the same setup with the explicit branch-test option:

```bash
./setup.sh --all --allow-unreleased
```

The checkout must be clean and its HEAD must exactly match a current official
branch head. Forks, local-only or stale commits, and dirty checkouts are refused.
Run setup as your normal user, not with `sudo`. See
[Testing an unreleased official branch](#testing-an-unreleased-official-branch)
for the complete clone example and scope.

```powershell
# native Windows
Set-ExecutionPolicy -Scope Process Bypass -Force
Set-Location $HOME\dotfiles
.\setup.ps1 -All
```

The process-scoped execution-policy command is required when Windows rejects
the checkout as unsigned. It affects only the current PowerShell process and
does not weaken the user or machine policy permanently. Run setup from that
same window; `setup.ps1` cannot apply this itself because PowerShell evaluates
the policy before loading the script.

Open a new terminal after the first install. The current shell started before
the new PATH, profile, and default shell existed.

To reconcile the checked-out release and update the tools that this repo can
prove it owns:

```bash
./setup.sh --update
```

```powershell
.\setup.ps1 -Update
```

`--update` does not pull Git or move you to a new release. For a release change,
clone the new exact tag beside the old checkout and follow
[docs/UPGRADING.md](docs/UPGRADING.md). Do not turn a live old checkout into a
new release with `git pull`.

To preview or remove the managed config layer:

```bash
./uninstall.sh --dry-run
./uninstall.sh --all
```

```powershell
.\uninstall.ps1 -DryRun
.\uninstall.ps1 -All
```

Uninstall removes repo-owned config. It does not blindly remove every package
that exists on the machine. Detailed install, migration, and recovery behavior
starts at [Detailed setup and migration reference](#detailed-setup-and-migration-reference).

## Cheat sheets

### How to read multiplexer shortcuts

`Ctrl+B`, then `w` means: hold `Ctrl`, press `B`, release both, then press `w`.
It is a sequence, not one four-key chord. This README calls `Ctrl+B` the
**prefix**.

### Herdr

Herdr is the agent-focused multiplexer. It groups terminal panes into tabs and
workspaces, then tracks the agents running inside them. The repo makes its
common navigation feel like tmux and uses Herdr's built-in `rose-pine` theme.
The managed binaries include Herdr's shifted indexed-key fix (`v0.7.4` on
stable platforms and the July 16 preview on Windows), so `Shift+1..9` reaches
the punctuation keycodes terminals actually send.

Start it from a normal shell with `herdr`. On Windows, new Herdr panes run
`pwsh.exe`, so they load the same PowerShell profile, history list, and
completion behavior as Windows Terminal. Recreate an old pane after a config
change; an already-running shell cannot change into PowerShell 7 retroactively.

| Keys | Result |
|---|---|
| `Ctrl+B`, then `w` | Open the full workspace/tab/pane navigator. Use Up/Down and Enter. |
| `Ctrl+B`, then `g` | Open the same full navigator. |
| `Ctrl+B`, then `p` / `n` | Move to the previous/next tab/window. |
| `Ctrl+B`, then `1` ... `9` | Switch tabs/windows. |
| `Ctrl+B`, then `,` | Rename the current tab/window. |
| `Ctrl+B`, then `$` | Rename the current workspace. |
| `Ctrl+B`, then Up/Down | Move to the previous/next workspace. |
| `Ctrl+B`, then `Shift+1` ... `Shift+9` | Jump directly to workspace 1 ... 9. |
| `Ctrl+B`, then `Shift+A` / `a` | Move to the previous/next agent. |
| `Ctrl+B`, then `Ctrl+1` ... `Ctrl+9` | Focus agent 1 ... 9 directly. |

Named Herdr sessions are separate server namespaces. A session does not appear
inside another session's navigator; attach to the other session from a normal
shell.

Config locations:

- macOS/Linux/WSL: `~/.config/herdr/config.toml`
- Windows: `%APPDATA%\herdr\config.toml`

### tmux and psmux

Use `tmux` on macOS, Linux, and WSL. Use `psmux` on native Windows. Both use
`Ctrl+B` as the prefix and count windows/panes from 1.

```bash
tmux              # start
tmux attach       # reattach
```

```powershell
psmux             # start
psmux attach      # reattach
```

| Keys | Result |
|---|---|
| `Ctrl+B`, then `c` | Create a window. |
| `Ctrl+B`, then `1` ... `9` | Switch windows. |
| `Ctrl+B`, then `n` / `p` | Next / previous window. |
| `Ctrl+B`, then `,` | Rename the current window. |
| `Ctrl+B`, then `$` | Rename the current session. |
| `Ctrl+B`, then `|` | Split left/right. |
| `Ctrl+B`, then `-` | Split top/bottom. |
| `Ctrl+B`, then `h` / `j` / `k` / `l` | Focus the pane left/down/up/right. |
| `Ctrl+B`, then `H` / `L` | Move the current window left/right. |
| `Ctrl+B`, then `w` | Open the session/window/pane tree. |
| `Ctrl+B`, then `d` | Detach and leave the session running. |
| `Ctrl+B`, then `r` | Reload the managed config. |
| `Ctrl+B`, then `Ctrl+S` | Save the current session layout. |
| `Ctrl+B`, then `Ctrl+R` | Restore the saved session layout. |

POSIX tmux auto-saves every 15 minutes and auto-restores on startup. Its first
launch may say `Tmux resurrect file not found!` until the first save exists.
Windows psmux is manual: the `run-shell` `Saved to ...` popup means the save is
done; close it with `q` or `Esc` (`Enter` does nothing). After restarting psmux,
restore with `Ctrl+B`, then `Ctrl+R`, then use `Ctrl+B`, then `w` to select the
restored session.

Copy text with vi-style copy mode:

1. Press `Ctrl+B`, then `[`.
2. Move with vi keys.
3. Press `v` to start selecting.
4. Press `y` to copy to the system clipboard.
5. Press `Ctrl+B`, then `]` to paste into the terminal.

On Windows, click-and-drag selection belongs to Windows Terminal; use
`Ctrl+Shift+C` to copy it. The normal tmux mouse features—pane focus, wheel
scroll, and border resize—stay enabled.

The Rose Pine variant is `main` by default. Switch it live with:

```bash
tmux set -g @rosepine-variant moon
tmux source-file ~/.tmux.posix.conf
```

```powershell
psmux set -g @rosepine-variant moon
psmux source-file ~/.tmux.windows.conf
```

Valid variants are `main`, `moon`, and `dawn`.

Configs: `~/.tmux.conf` plus `~/.tmux.posix.conf` on POSIX;
`~/.psmux.conf` plus `~/.tmux.windows.conf` on Windows.

### AeroSpace (macOS)

AeroSpace tiles macOS app windows. It starts at login and reloads its config
when the file changes. On first launch, grant it access in **System Settings ->
Privacy & Security -> Accessibility**. macOS does not allow setup to grant this
permission for you.

| Keys | Result |
|---|---|
| `Ctrl+Alt+h/j/k/l` | Focus the window left/down/up/right. |
| `Ctrl+Alt+Shift+h/j/k/l` | Move the focused window left/down/up/right. |
| `Ctrl+Alt+-` / `Ctrl+Alt+=` | Shrink / grow the focused window. |
| `Ctrl+Alt+/` | Use tiled layout. |
| `Ctrl+Alt+,` | Use accordion layout. |
| `Ctrl+Alt+f` | Toggle fullscreen. |
| `Alt+1` ... `Alt+9` | Switch workspace. |
| `Alt+Shift+1` ... `Alt+Shift+9` | Move the focused window to a workspace. |
| `Alt+Tab` | Jump back to the previous workspace. |
| `Ctrl+Alt+Shift+;` | Enter service mode. |

In service mode, press `Esc` to reload the config, `r` to flatten the workspace
tree, `f` to toggle floating/tiling, or Backspace to close every window except
the current one.

Config: `~/.config/aerospace/aerospace.toml`.

### Neovim

The leader key is Space. For example, `<leader>fg` means press Space, then `f`,
then `g`.

| Keys / command | Result |
|---|---|
| `<leader>?` | Show the keys available in the current buffer. Start here when you forget a shortcut. |
| `:WhichKey` | Open Folke's full keymap popup explicitly; press Esc to close it. |
| `Ctrl+P` | Find a file. |
| `<leader>fg` | Search text in the project. |
| `<leader>fb` | List open buffers. |
| `<leader>fd` | List diagnostics. |
| `Alt+h/j/k/l` | Focus the Neovim window left/down/up/right. |
| `gd` / `gr` | Go to definition / find references. |
| `K` | Show documentation for the item under the cursor. |
| `<leader>rn` | Rename a symbol. |
| `<leader>ca` | Show code actions. |
| `[d` / `]d` | Previous / next diagnostic. |
| `[h` / `]h` | Previous / next Git hunk. |
| `<leader>gp` | Preview the current Git hunk. |
| `<leader>gt` | Toggle blame for the current line. |
| `<leader>gf` | Format the current buffer or selection. |
| `:wnf` | Save once without formatting. The next normal `:w` formats again. |
| `<leader>u` | Open/close the undo tree. |
| `<leader>mr` | Toggle rendered Markdown. |
| `<leader>lt` | Toggle relative line numbers. |
| `gcc` | Comment/uncomment the current line. |
| `<leader>b` | Toggle a debugger breakpoint. |
| `F5` / `F10` / `F11` / `F12` | Continue / step over / step into / step out. |

Files format on `:w`. The timeout is 10 seconds. Use `:ConformInfo` to see which
formatter is active if a save fails or times out; use `:wnf` only when you
intentionally want one unformatted save.

Config: `~/.config/nvim` on macOS/Linux/WSL and `%LOCALAPPDATA%\nvim` on
Windows. Both point to this repo's `nvim/` directory.

### Starship

Starship is the prompt. It has no special mode and no shortcuts. It shows:

- your username and full current path;
- the Git branch and working-tree state;
- active C, Go, Node.js, Rust, Python, or Conda versions;
- the current time.

The Git symbols are deliberately compact:

| Symbol | Meaning |
|---|---|
| `✓` | clean |
| `?(n)` | untracked files |
| `!(n)` | modified files |
| `++(n)` | staged files |
| `✘(n)` | deleted files |
| `⇡(n)` / `⇣(n)` | commits ahead / behind |
| `$` | stash exists |

Edit the repo file `starship/starship.toml` to change the prompt. Start a new
shell to see startup-level changes.

### Shell, completion, and navigation

zsh on POSIX and PowerShell 7 on Windows use the same basic habits:

| Keys / command | Result |
|---|---|
| Tab | Open the completion menu. Keep typing to narrow it. |
| Up/Down | Search history using the text already typed as a prefix. |
| `Ctrl+R` | Fuzzy-search command history. |
| `Ctrl+T` | Fuzzy-pick a file and insert its path. |
| `Alt+C` | Fuzzy-pick a directory and change into it. |
| `Esc` | Enter vi normal mode on the command line. |
| `i` / `a` | Return to vi insert mode. |
| `z proj` | Jump to the best previously visited directory matching `proj`. |
| `zi` | Pick a known directory interactively. |

`ls`, `l`, `la`, `lla`, and `lt` use `lsd` for readable icons and colors.
zsh-only local changes belong in `~/.zshrc.local`; setup does not overwrite that
file.

Useful standalone tools:

- `lazygit`: terminal Git UI. In the commits panel, `J`/`K` move a commit. On
  Windows inside psmux, use `Ctrl+G` when a popup expects Esc.
- `gh dash`: dashboard for pull requests and issues. Run `gh auth login` first,
  then rerun setup if the extension was skipped while unauthenticated.
- `pi`: the pinned Pi CLI, with the repo's audited Rose Pine theme selected by
  default. Sessions, credentials, providers, and other preferences are not synced.

### Terminals and scrollback

- **tmux/psmux panes:** 50,000 history lines per pane. This is separate from the
  outer terminal's scrollback limit.
- **Ghostty:** macOS/Linux, dark Rose Pine, 1 GiB lazy scrollback budget per
  surface, copy-on-select, maximized startup. On macOS, Cmd+grave accent toggles
  the global quick terminal.
- **WezTerm:** macOS/Linux/Windows, dark Rose Pine, 5,000,000 scrollback lines
  per tab, maximized startup. It opens a normal shell; start Herdr/tmux/psmux
  yourself.
- **Windows Terminal:** Windows/WSL host, dark Rose Pine, 32,767 history lines
  per profile. That is Windows Terminal's hard maximum.

### Clipboard on Linux, WSL, and Windows

Neovim and tmux copy to the system clipboard. The helper depends on where the
shell is running:

| Environment | Clipboard path |
|---|---|
| macOS | `pbcopy` |
| Linux Wayland | `wl-copy` from `wl-clipboard` |
| Linux X11 | `xclip`, then `xsel` as fallback |
| WSL | `win32yank.exe` on the Windows host |
| Native Windows psmux | `clip.exe` |
| Remote shell / no helper | OSC52 through the terminal |

`devilspie2` is **not** a clipboard tool. On Linux/X11 it is an optional rule
that forces Ghostty to open maximized when the window manager ignores Ghostty's
maximize request. It does not work on Wayland; GNOME/Wayland needs a Shell
extension for that window behavior.

## Detailed setup and migration reference

The rest of this README explains the implementation, recovery paths, CI, and
maintenance rules. Normal daily use does not require it.

Clone the repo first, then run the local setup entry point. `setup.{sh,ps1}`
installs repo-managed dependencies, links every config, then runs
`:Lazy! restore`, a synchronous nvim-treesitter parser install, and
the repo's checked `:MasonToolsInstallSync` wrapper before the first interactive
Neovim launch. The wrapper exits nonzero on a command error or missing package;
headless Neovim cannot print an install error and still let setup report done.
Piped or stdin setup is intentionally disabled; if setup cannot prove it is
running from a local checkout, it fails closed with clone-first instructions.

Git is required to clone this repo. On Linux/WSL, a greenfield setup installs
`curl` plus CA certificates through the detected native package manager before
the Nix prerequisite helper needs them. On macOS/Linux/WSL, setup bootstraps Nix
when it is missing by calling the release-pinned prerequisite helper itself.
That helper downloads the official upstream Nix 2.34.0 release and verifies the
platform SHA-256 before extraction or execution, then runs the verified local
installer non-interactively with `nix-command` and flakes enabled. Nix's
multi-user path does not honor its own `--no-modify-profile` option, and its
Linux copy step can turn a restrictive invoking umask into root-only store
directories. Daemon bootstrap therefore verifies the exact extracted script,
locally guards its one profile-configuration call, normalizes store paths to
Nix's canonical read-only/traversable modes, and verifies the complete patched
script hash before execution. Setup then activates Nix in the current
transaction, and Home Manager publishes the future-session path used by the
managed zsh config. This avoids upstream reads or writes of system shell files
such as `/etc/bashrc`, works under restrictive corporate umasks, and repairs the
same inaccessible store modes left by an interrupted attempt. The wrapper also
passes upstream's public `--no-channel-add` option: this repository uses locked
flakes, so fetching the mutable `nixpkgs-unstable` channel is unnecessary and
would wrongly force the installer's bundled CA instead of the managed host's
system trust store. If an earlier attempt installed Nix but stopped before
enabling those features, rerunning setup repairs the user setting and continues.
The annotated v0.2.0 release is published, so normal setup accepts
only the exact clean official tag. For field testing before another release,
the explicit `--allow-unreleased` option accepts a clean checkout only when its
HEAD is a current branch head in the official repository. Local-only or stale
commits, forks, dirty checkouts, lightweight tags, and non-official origins
still fail before download. The versioned upgrade tools remain exact-tag-only;
this repo has no pipe-to-shell Nix bootstrap.

```bash
# Apple Silicon mac / linux / wsl
git clone --branch v0.2.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles
cd ~/dotfiles
./setup.sh --all
```

### Testing an unreleased official branch

Use this only for a greenfield or already-v0.2.0 test machine. Replace the
example branch value with the official branch you want to test:

```bash
TEST_BRANCH=BRANCH_NAME
git clone --branch "$TEST_BRANCH" --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles-test
cd ~/dotfiles-test
./setup.sh --all --allow-unreleased
```

The opt-in verifies one official remote-ref snapshot and proceeds only if the
clean local HEAD exactly matches a current official branch head. It does not
authorize a fork, a local commit, a stale checkout, or an in-place v0.1.0
migration. Setup links managed config to this checkout, so keep it in place
while the machine uses the tested configuration.

```powershell
# windows
# enable Developer Mode, then run from a normal PowerShell
# Settings -> Privacy & security -> For developers -> Developer Mode = On
git clone --branch v0.2.0 --single-branch `
  https://github.com/luisgui1757/dotfiles.git $HOME\dotfiles
Set-ExecutionPolicy -Scope Process Bypass -Force
Set-Location $HOME\dotfiles
.\setup.ps1 -All
```

For WSL, treat setup as split-host: run `.\setup.ps1 -All` on Windows so
Windows Terminal, Hack Nerd Font, lazygit, and `win32yank` are installed on the
rendering host, then run `./setup.sh --all` from the exact release checkout
inside the WSL home for the Linux CLI/editor stack; setup installs Nix there
when it is missing. Windows Terminal settings handling
runs by default, and setup independently stages, validates, backs up, and
atomically merges each existing stable packaged, Preview, Canary, and portable
target. If Scoop, winget, and choco
cannot register the MSIX app, setup falls back to a pinned
SHA-256-verified portable WT zip. Portable WT reads the unpackaged settings
path, so portable WT is merged from its own current settings (or seeded only
when it is detected and the file is absent); it is never overwritten with the
packaged installation's complete settings. Pass
`-SkipWindowsTerminalMerge` only when you want setup to leave WT settings
untouched. Linux Ghostty and Linux
fontconfig fonts inside WSL are intentionally outside the happy path; opt in
with `./setup.sh --experimental-wsl-gui` only when you explicitly want a WSLg /
X11 GUI-terminal experiment.

On Windows, prefer Developer Mode plus a normal PowerShell. If Developer Mode is
unavailable and you cannot enable it, run just the config phase from an
elevated PowerShell with `.\setup.ps1 -SkipDeps -SkipNvim`, then return to a
normal shell for `.\setup.ps1 -SkipDeps -SkipConfig`. Do not elevate the whole
dependency-install run; Scoop refuses admin installs.

### Upgrading from v0.1.0

`v0.1.0` is already chezmoi-based and its POSIX config is linked into the
checkout. **Do not run `git pull` in that checkout.** Changing it in place can
change live config before recovery exists.

The annotated v0.2.0 release is published. Retain the exact old checkout, clone
v0.2.0 beside it, and run only `./setup.sh --all`
or `.\setup.ps1 -All` from the new checkout. Setup detects the live exact-v0.1.0
owner, installs Nix on POSIX when needed, runs the existing digest-bound
transaction, verifies and accepts its reversible core, retains private recovery,
then completes additive provisioning and repoints config to v0.2.0. A pending
`applied` recovery resumes at acceptance; an unsafe/incomplete recovery fails
closed with its exact rollback command. If the old checkout is not discoverable
from the live config, set `DOTFILES_V0_1_CHECKOUT` to its real path for that same
setup invocation. macOS migration is available only on Apple Silicon.

The manual preflight/apply/rollback/accept commands remain available in
[docs/UPGRADING.md](docs/UPGRADING.md) for diagnosis and operator-controlled
recovery, but they are no longer the normal user path.

### Existing Checkout

```bash
./setup.sh                       # Y/n per dep, end-to-end
./setup.sh --all                 # install or migrate, then reconcile everything
./setup.sh --update              # full reconciliation + proven tool/Mason refresh
./setup.sh --upgrade             # alias for --update
./setup.sh --dry-run             # preview
./setup.sh --allow-unreleased    # test a clean current official branch head
./setup.sh --experimental-wsl-gui # WSL-only opt-in for Linux GUI terminal bits
./setup.sh --nix-darwin          # compatibility alias; macOS setup already applies nix-darwin
./setup.sh --home-manager        # compatibility alias; Linux/WSL setup already applies Home Manager
./setup.sh --skip-deps           # skip Nix + native/deferred dependency provisioning
./setup.sh --skip-native-deps    # keep Nix/config; skip native/deferred dependencies
./setup.sh --skip-config         # skip chezmoi config apply
./setup.sh --skip-config-scripts # apply config files/links; defer chezmoi run scripts
./setup.sh --skip-nvim           # skip Lazy/Tree-sitter/Mason phases
./setup.sh --skip-agents         # skip global Sentinel agent policy
./setup.sh --best-effort         # continue after nvim-phase failures; exit nonzero with summary
make setup                       # same as ./setup.sh, via the Makefile
```

**Nix layer (required POSIX packages only).** `flake.nix` + a committed
`flake.lock` provide the macOS/Linux/WSL package layer. macOS uses nix-darwin +
declarative Homebrew + Home Manager; Linux/WSL uses standalone Home Manager.
chezmoi still owns **every** dotfile; Nix owns no config. A normal `./setup.sh`
or `./setup.sh --all` applies the matching package layer before native/deferred
dependency provisioning. On macOS setup normalizes `uname -m` and runs only the
Apple Silicon `sudo -H env DOTFILES_TARGET_USER=... DOTFILES_TARGET_HOME=...
darwin-rebuild switch --flake .#dotfiles-aarch64 --impure` activation, which activates the
declarative Homebrew casks (WezTerm, AeroSpace) + Herdr brew and the nix-owned
CLI package set. Any other macOS architecture fails closed before Nix/Homebrew
activation. Before any phase, setup rejects root/ambiguous invocation and
resolves one invoking account plus its authoritative account-record home. It
requires ambient `HOME` to resolve to that same directory and passes
`DOTFILES_TARGET_USER` / `DOTFILES_TARGET_HOME` explicitly through sudo, so
Nix, Home Manager, chezmoi, and native setup cannot split across users or
fabricated homes. First-run bootstrap is also pinned: setup derives the locked
nix-darwin rev and `narHash` from `flake.lock` before running
`sudo -H env DOTFILES_TARGET_USER=... DOTFILES_TARGET_HOME=... nix run
github:nix-darwin/nix-darwin/<locked-rev>?narHash=<encoded-narHash>#darwin-rebuild -- ...`;
it never uses the mutable `nix-darwin` registry alias. On first bootstrap,
pre-existing `/etc/bashrc` and `/etc/zshrc` are moved only to nix-darwin's
documented `.before-nix-darwin` names after both backup paths pass a collision
preflight. Activation failure or interruption quarantines any generated
replacement and restores both originals; success retains the backups for
nix-darwin recovery/uninstall. A retry from the same pre-activation terminal
resolves the installed `/run/current-system` rebuild command even before that
terminal's `PATH` is refreshed; nix-darwin-managed `/etc/static` links and their
retained backups are recognized as the normal idempotent state. On Linux/WSL, setup runs
`home-manager switch --flake .#<arch>-linux --impure`; first-run bootstrap uses
the locked
`github:nix-community/home-manager/<locked-rev>?narHash=<encoded-narHash>#home-manager`
ref. WSL writes only to the Linux `~/.nix-profile`, never `/mnt/c`. Fresh
Linux/WSL zsh sessions source Home Manager's canonical `hm-session-vars.sh`
from the XDG Nix profile, `~/.nix-profile`, or the system-integrated
`/etc/profiles/per-user/<effective-user>` profile, in that order, so Nix-owned
tools resolve without caller-injected PATH state. The standalone Linux Home
Manager configuration places its evaluated profile `bin` in
`home.sessionPath`, which makes `hm-session-vars.sh` export the path itself.
`--skip-deps` is the explicit already-provisioned escape hatch and skips the
Nix package-layer application together with native/deferred dependency installs;
the compatibility aliases `--nix-darwin` and `--home-manager` do not override
that skip. `--skip-native-deps` is the narrower release-migration boundary: it
still applies Nix and config but leaves native/deferred tools untouched for
coordinated rollback. The versioned migration pairs it with
`--skip-config-scripts`, so plugin publication and every other chezmoi run
script remain outside that reversible core; normal setup does not use that
flag. Config apply creates required parent directories itself rather than
depending on native provisioning. Dry-run does not require Nix: on an unprovisioned POSIX host it
previews that the real run would fail until Nix is installed. On a Brew-less
Mac, it previews the verified Homebrew bootstrap, then continues every later
Brew-backed preview phase without claiming the bootstrap already happened.
The Nix-owned CLI set includes Node 24 so the pinned npm-backed Pi CLI can run
reproducibly on macOS/Linux/WSL while the `pi` package itself stays pinned by
npm integrity until nixpkgs catches up.
Homebrew is intentionally mixed ownership on macOS. nix-darwin installs the
repo-declared subset with `homebrew.onActivation.cleanup = "none"`; it never
rejects or removes extra formulae/casks installed by `install-deps.sh` or the
user. The same non-destructive contract applies on real Macs and hosted CI, so
there is no environment-only cleanup bypass.
nix-homebrew uses `autoMigrate = true` so Macs that already have official-script
Homebrew can be adopted by the declarative Nix layer; upstream's migration keeps
installed packages while replacing the Homebrew repository. `mutableTaps = true`
keeps every tap clone owned and updated by Homebrew as the target user; nix-homebrew
pins the Homebrew implementation but deliberately copies no tap trees during root
activation. A one-time scoped migration removes only the three recognizable
root-owned, non-Git tap snapshots produced by the earlier configuration, then
Homebrew recreates the required AeroSpace tap normally. Unrelated taps such as
Cirrus are never selected. The `nikitabobko/tap` tap is explicitly trusted
through nix-homebrew because Homebrew 5 refuses to load personal-tap casks,
including AeroSpace, without a trust entry.
Tap transaction and diagnostic snapshots always live beside
`Library/Taps`, never below it: Homebrew enumerates directories below `Taps` as
live taps. Setup also recognizes the exact in-tree recovery names emitted by
the short-lived broken migration and moves them to an external recovery root
before activation, so retry needs no manual `brew untap` or filesystem cleanup.
Setup also re-adopts the canonical daemon or user Nix profile binary directly
when Homebrew's `path_helper` refresh has removed Nix from `PATH` after the
upstream profile's already-sourced guard was set; it does not misclassify that
stale-shell state as a missing Nix installation.
**nvim and the
tree-sitter CLI stay native** (ABI-coupled to nvim-treesitter parser builds;
migrating them into a same-closure toolchain is a follow-up). Native Windows is
non-Nix. `nix flake check` runs in required CI (`.github/workflows/nix.yml`) on
Ubuntu + Apple Silicon macOS. The flake exports `dotfiles-aarch64` plus the
Apple-Silicon compatibility alias `dotfiles`; no other Darwin system or
configuration is exported. Historical platform results remain in the
append-only evidence ledger and are not a current support claim.

```powershell
.\setup.ps1
.\setup.ps1 -All
.\setup.ps1 -Update
.\setup.ps1 -Upgrade  # alias for -Update
.\setup.ps1 -DryRun
.\setup.ps1 -SkipConfig
.\setup.ps1 -SkipAgents
.\setup.ps1 -SkipWindowsTerminalMerge # leave WT settings.json untouched
.\setup.ps1 -MergeWindowsTerminal     # accepted no-op alias; WT merge is default-on
```

### Config Layer (chezmoi)

Chezmoi is the config-only path. It manages dotfiles from `home/`; it does not
install programs, fonts, VS Code, psmux, login shells, or other provisioning
steps. Run `install-deps.sh` / `install-deps.ps1` or full `setup` for those.

The remote chezmoi one-liner is a mutable remote config-apply trust boundary
because it asks chezmoi to fetch and apply the current default branch. It is not
the quick usable setup path; prefer a local checkout, and pin the checkout to a
reviewed commit/ref when you need reproducibility:

```bash
git clone https://github.com/luisgui1757/dotfiles.git ~/dotfiles
cd ~/dotfiles
git checkout <reviewed-commit-or-tag>
chezmoi --source ./home init
chezmoi --source ./home apply
```

From an existing checkout, initialize once so `home/.chezmoi.toml.tmpl` writes
the local OS data, then re-apply or preview the config layer:

```bash
chezmoi --source ./home init
make chezmoi
make chezmoi-diff
```

or:

```bash
chezmoi --source ./home apply
chezmoi --source ./home diff
```

### Uninstall

Use the uninstall scripts to remove the chezmoi-managed config layer from a
machine before greenfield testing:

```bash
./uninstall.sh --all
./uninstall.sh --dry-run
./uninstall.sh --keep-externals
./uninstall.sh --no-restore-backups
```

```powershell
.\uninstall.ps1 -All
.\uninstall.ps1 -DryRun
.\uninstall.ps1 -KeepExternals
.\uninstall.ps1 -NoRestoreBackups
```

They remove only repo-owned symlinks or byte-identical Windows copies, restore
the newest validated `<target>.bak.<timestamp>[.n]` backup by filename order,
and leave chezmoi's own
state/config alone. Dry-run mode prints the planned removals without deleting
files or pruning empty external parent directories. Windows Terminal settings
are not deleted: validated stable/Preview/Canary/portable backups restore independently, and
the displaced current file is preserved as `settings.json.uninstall-current.*`.
Pi settings are also preserved: uninstall removes `theme` only when it still
equals the repo-managed `rose-pine` value, and never changes a later user choice.

For a destructive Apple Silicon owner-host smoke, run
`./tests/macos_owner_lifecycle.sh` from a clean committed checkout. It prompts
for sudo in the terminal once, then exercises install, update, config uninstall,
reinstall, and final update; verifies the second uninstall is a no-op; and
proves pre-existing Homebrew formulae, casks, and unrelated taps were not
removed. POSIX uninstall intentionally removes the config layer, not Nix or
Homebrew packages. Its tap checks use `brew --prefix` because nix-homebrew's
managed implementation repository is intentionally not the installed-tap root.
Backup selection uses the filename timestamp/collision suffix, never mtime;
malformed candidates fail before removal/restoration.

## What Setup Does

`setup` is a six-phase, idempotent orchestrator:

```text
setup -> install-deps                 phase 1: programs and optional tools
      -> chezmoi apply                phase 2: config layer with pre-apply backups
      -> nvim "+Lazy! restore" +qa    phase 3: plugins from lazy-lock.json;
                                            parser-update build tasks must finish
      -> nvim +DOTFILES_TREESITTER_SYNC_INSTALL  phase 4: Tree-sitter parsers
      -> nvim +checked-MasonToolsInstallSync      phase 5: LSP servers and formatters
      -> Sentinel global install       phase 6: per-user agent policy
```

The nvim-treesitter Lazy build hook calls the upstream waitable update API,
serializes parser builds, and requires successful completion before Phase 3 can
return. A command-form `:TSUpdate` is asynchronous and must not be used as that
build hook: it is also a Lazy command trigger whose plugin config starts the
declared-parser install asynchronously. On a cold cache either task can leave
compilers publishing parsers while Phase 4 starts its explicit synchronous
install. Phase 4 remains the complete declared-parser bootstrap and proof
boundary. Plugin config also refuses to start its interactive asynchronous
auto-install path in an ordinary headless process; only a real UI session or
the explicit synchronous Phase 4 flag may start declared-parser installation.
That prevents Lazy restore, Mason, and smoke validators from creating separate
compiler tasks around the proof phase.

Pass `--all` / `-All` for explicit non-interactive installs (Y to every setup
prompt). Setup also owns consent after that decision: its POSIX dependency
child launches the verified Homebrew bootstrap with `NONINTERACTIVE=1`, then
clears Homebrew's inherited ask override and uses its supported
`HOMEBREW_NO_ASK=1` mode. Neither the bootstrap nor Homebrew 6+ package commands
can add a second confirmation. These settings exist only inside setup;
ordinary `brew` commands keep their normal behavior. Password authentication
and OS permission grants can still require the user because they are not
package-selection confirmations.
When setup detects redirected stdin/stdout and neither all nor dry-run was
requested, it defaults to all and prints `note: no TTY detected; running with
--all` (or `-All`). An interactive run (no all flag) can still ask the
dependency installer's **"install EVERYTHING without further prompts? [Y/n]"**
question; answer `Y` to pull the tool catalog in one go, or `n` to choose per
tool. Phase 6 then asks **"Apply Sentinel global agent rules? [Y/n]"** unless
`--all` / `-All`, `--dry-run` / `-DryRun`, or `--skip-agents` / `-SkipAgents`
already made that decision.
Add `--dry-run` / `-DryRun` to preview every step without touching disk.
Pass `--skip-agents` / `-SkipAgents` to leave global AI-agent entrypoints alone.
Pass `--update` / `-Update` from an exact release checkout to run the same
install-or-migrate and full-reconciliation path as all mode, followed by scoped
package-manager/direct-artifact updates for proven present tools and the checked
`MasonToolsUpdateSync` wrapper. `--upgrade` / `-Upgrade` is an alias. Update never runs
`git pull`, follows a moving branch, performs a blanket package-manager upgrade,
rewrites `flake.lock`, or runs `:Lazy update`; repository pins change only in a
reviewed release.

Every script is safe to rerun. Pre-existing non-symlink targets are backed up to
`<target>.bak.<timestamp>` with collision-proof suffixes (`.1`, `.2`, ...).
Before setup lets `chezmoi --force apply` replace an existing managed target, it
backs up only targets that are not already correct: exact chezmoi state and
content-equivalent targets are left alone. On Windows, setup still checks
Developer Mode/elevation before applying because the Neovim directory target
remains a symlink even though single-file configs are copies.

### Managed Configs

The table below is the config layer. Full setup and config-only applies use the
chezmoi source under `home/`, plus dedicated Windows known-folder source states.
Mechanisms differ: POSIX uses symlinks; ordinary UserProfile Windows files are
copies; redirected LocalApplicationData/ApplicationData/Documents targets are
symlink overlays; and Windows Terminal remains a merge.

| Tool | macOS | Linux / WSL | Windows |
|---|---|---|---|
| Neovim | `~/.config/nvim` -> `nvim/` | `~/.config/nvim` -> `nvim/` | `%LOCALAPPDATA%\nvim` -> `nvim\` |
| Starship | `~/.config/starship.toml` -> `starship/starship.toml` | same | `%USERPROFILE%\.config\starship.toml` -> `starship\starship.toml` |
| zsh | `~/.zshenv` -> `shells/zshenv`; `~/.zshrc` -> `shells/zshrc` | same | n/a |
| PowerShell | n/a | n/a | actual runtime `$PROFILE` plus Console/VS Code/ISE host profiles under the real Documents known folder -> `shells\powershell_profile.ps1` |
| tmux / psmux | `~/.tmux.conf` -> `tmux/tmux.conf`; `~/.tmux.posix.conf` -> `tmux/tmux.posix.conf` (POSIX clipboard + TPM functional plugins + generated Rose Pine bar); `~/.tmux.rose-pine.{main,moon,dawn}.conf` -> generated `tmux/psmux-rose-pine.{main,moon,dawn}.conf` (Omer-shaped Rose Pine bar, **shared** with Windows) | same | `%USERPROFILE%\.psmux.conf` -> `tmux\psmux.conf` (first psmux entrypoint, disables warm sessions, then flag-free source-files the Windows overlay); `%USERPROFILE%\.tmux.conf` -> `tmux\tmux.conf`; `%USERPROFILE%\.tmux.windows.conf` -> `tmux\tmux.windows.conf`; `%USERPROFILE%\.tmux.rose-pine.ps1` -> `tmux\psmux-rose-pine.ps1` (Rose Pine bar generator / manual live-switch helper); `%USERPROFILE%\.tmux.rose-pine.{main,moon,dawn}.conf` -> the same generated `tmux\psmux-rose-pine.{main,moon,dawn}.conf`; the POSIX overlay is **excluded** on Windows (its `if-shell` probes hang psmux); WSL uses the Unix path |
| Ghostty | `~/Library/Application Support/com.mitchellh.ghostty/config` -> `ghostty/config` (lazy 1 GiB per-surface scrollback byte budget) | native Linux links the same `~/.config/ghostty/config`; WSL links it only with `--experimental-wsl-gui` | n/a |
| WezTerm | `~/.config/wezterm/wezterm.lua` -> `wezterm/wezterm.lua` (5,000,000 scrollback lines per tab) | same; WSL links it only with `--experimental-wsl-gui` | `%USERPROFILE%\.config\wezterm\wezterm.lua` -> `wezterm\wezterm.lua` (copied; same 5,000,000-line budget) |
| AeroSpace | `~/.config/aerospace/aerospace.toml` -> `aerospace/aerospace.toml` (macOS tiling WM; focus/move on `ctrl-alt(-shift)` to avoid nvim `<A-h/j/k/l>` and fzf `Alt-c`) | n/a (macOS-only) | n/a (macOS-only) |
| Herdr | `~/.config/herdr/config.toml` -> `herdr/config.toml` (built-in `rose-pine`, forced dark; tmux-shaped navigator, rename, and workspace bindings) | same | actual roaming `%APPDATA%\herdr\config.toml` -> `herdr\config.windows.toml` (same theme/navigation plus `pwsh.exe` as the pane shell) |
| lazygit | `~/Library/Application Support/lazygit/config.yml` -> `lazygit/config.yml` | `~/.config/lazygit/config.yml` -> `lazygit/config.yml` | `%LOCALAPPDATA%\lazygit\config.yml` -> `lazygit\config.windows.yml` |
| lsd | `~/.config/lsd/{config.yaml,colors.yaml}` -> `lsd/{config.yaml,colors.yaml}` | same | `%USERPROFILE%\.config\lsd\{config.yaml,colors.yaml}` -> `lsd\{config.yaml,colors.yaml}` |
| gh-dash | `~/.config/gh-dash/config.yml` -> `gh-dash/config.yml` | same | `%USERPROFILE%\.config\gh-dash\config.yml` -> `gh-dash\config.yml` |
| Pi theme | `~/.pi/agent/themes/rose-pine.json` -> `pi/rose-pine.json`; setup merges `theme: rose-pine` into global settings | same | `%USERPROFILE%\.pi\agent\themes\rose-pine.json` is copied; setup performs the same one-key merge |
| Windows Terminal | n/a | n/a | app installed by `setup.ps1` through Scoop/winget/choco, with a SHA-256-verified portable zip fallback; one validated enumerator identifies stable packaged, Preview, Canary, and portable `settings.json` targets for setup, migration, recovery, and uninstall; all profiles receive WT's hard maximum of 32,767 history lines; setup stages and validates all selected targets before publication, creates separate verified backups, detects concurrent changes through atomic replacement rollback bytes, and rolls the group back on failure; opt out with `-SkipWindowsTerminalMerge`; see [windows-terminal/README.md](windows-terminal/README.md) |

Windows setup resolves UserProfile, LocalApplicationData, ApplicationData,
Documents, and the active host's `$PROFILE` independently through supported
runtime/known-folder APIs. It applies the UserProfile source plus dedicated
LocalApplicationData, ApplicationData, and Documents source states, then
verifies the paths Neovim, lazygit, Herdr, ConsoleHost, VS Code, and ISE consume.
For the PowerShell profiles, setup removes Mark-of-the-Web only after proving
each profile is the repo-owned source or an exact byte copy, so new terminals can
load normally under `RemoteSigned` without weakening the user's execution policy.
Redirected folders, alternate drives, and spaces are supported. Directory
ownership checks resolve both symbolic links and Windows junctions. Recognized
conventional-path legacy targets are backed up only after the new targets
publish; divergent legacy user data stays in place with a migration warning.
POSIX pwsh profile management remains provisioning-adjacent.

### Platform Notes

- Linux setup can make zsh your login shell. Installing the package alone is
  not enough: tmux and new terminals keep launching bash until setup adopts zsh.
  Local accounts use `chsh` plus an interactive bash guard so stale graphical
  sessions land in zsh without a full relogin; domain accounts use that guard
  when `chsh` cannot edit `/etc/passwd`. The prompt is consent-gated and
  auto-yes under `--all`.
- `install-deps` provisions chezmoi itself: Homebrew on macOS/Linuxbrew,
  a pinned SHA-256-verified GitHub release archive on native Linux without
  brew, and the Scoop-first catalog on Windows.
- Native Debian-family installs run every apt update/install/upgrade with an
  explicit noninteractive debconf frontend after the sudo boundary. Therefore
  `./setup.sh --all` does not stop for transitive package questions such as
  `tzdata` timezone selection.
- `install-deps` provisions Starship through Homebrew on macOS/Linuxbrew,
  Alpine's native package on Alpine, and a pinned SHA-256-verified Starship
  GitHub release archive on other native Linux/WSL hosts.
- **tmux/psmux Rose Pine is one repo-owned bar, uniform across platforms.** The
  status bar is an Omer/Catppuccin-shaped Rose Pine pill bar (rounded session
  pill on the left, number-on-right window cells with a zoom marker on the
  current window, directory pill on the right) generated by
  `tmux/psmux-rose-pine.ps1` into `tmux/psmux-rose-pine.{main,moon,dawn}.conf`.
  BOTH macOS/Linux tmux and native-Windows psmux source the SAME generated
  variant (`~/.tmux.rose-pine.{main,moon,dawn}.conf`), so the bar is
  byte-identical everywhere. We do NOT use a theme plugin for rendering:
  `rose-pine/tmux` is a bash/TPM script that cannot run on psmux, and the
  community `psmux-theme-rosepine` renders a different arrow-chevron powerline
  bar. The session pill uses Rose Pine `foam` normally and `love` while the
  prefix is held, keeping a cool Omer-style anchor without the iris/purple cast
  while retaining the Omer/Catppuccin shape. Default variant is `main`; switch
  to `moon`/`dawn` via `@rosepine-variant`
  (`tmux set -g @rosepine-variant moon; tmux source-file ~/.tmux.posix.conf` on
  POSIX, `psmux set -g @rosepine-variant moon; psmux source-file
  ~/.tmux.windows.conf` on Windows). The bar is a signal bar: tmux/psmux shows
  session, windows, and the current directory basename; Starship owns username,
  full path, git, language/runtime, and time; host stays off the daily surface.
- **Functional tmux/psmux plugins are pinned and vendored.** POSIX `install-deps`
  provisions TPM (`e261deb1b47614eed3400089ce7197dc68acc4eb`) plus the Omer-style
  functional set as pinned repo-managed checkouts under
  `~/.local/share/dotfiles/tmux-plugins`: `tmux-sensible`
  (`25cb91f42d020f675bb0a2ce3fbd3a5d96119efa`), `tmux-yank`
  (`acfd36e4fcba99f8310a7dfb432111c242fe7392`), `tmux-resurrect`
  (`cff343cf9e81983d3da0c8562b01616f12e8d548`), and `tmux-continuum`
  (`0698e8f4b17d6454c71bf5212895ec055c578da0`); session save/restore is on
  (`@continuum-restore on`, `@resurrect-strategy-nvim session`). Windows
  `install-deps.ps1` vendors ONLY the `psmux-resurrect` port from the
  `psmux/psmux-plugins` monorepo at pinned commit
  `0f46ccca5a9b748fd03851db00b85fd784f42791` into `~/.psmux/plugins/`, sourced
  directly by `tmux/tmux.windows.conf`. **`psmux-continuum` is intentionally NOT
  shipped on Windows** — its `plugin.conf` registers load-time async `run-shell`
  pwsh hooks that have not been verified on a real Windows psmux host, so it stays
  a blocked follow-up (POSIX tmux still gets `tmux-continuum`, which is testable
  on Linux). We do NOT use PPM (it clones the monorepo HEAD unpinned and rewrites
  managed config), and at the pinned commit there is no active top-level
  `psmux-yank` port (only a retired `_trash/psmux-yank`), so native-Windows yank
  stays the `clip.exe` copy-mode binding.
- Windows psmux uses a dedicated `~/.psmux.conf` entrypoint. It disables psmux
  warm sessions before sourcing `~/.tmux.conf`, so psmux cannot claim a stale
  warm server whose status theme loaded before chezmoi deployed the current
  generated Rose Pine configs. It then source-files `~/.tmux.windows.conf`
  explicitly with flag-free psmux syntax because psmux v3.3.x does not implement
  tmux's `source-file -q` config flag. psmux also does not implement tmux's
  `terminal-features` option, so extended-key feature flags live only in the
  POSIX overlay; psmux-parsed configs must not execute `set ... terminal-features`.
  The generated shared Rose Pine artifacts also stay inside the tmux/psmux option
  intersection: tmux-only display-pane color options are omitted because psmux
  stores unknown options but still warns on every config load.
- `install-deps` provisions `lsd` through the supported package managers
  (Homebrew, native Linux package managers where available, and the Windows
  Scoop-first catalog). Interactive shells replace `ls` with `lsd` and add the
  useful `l`, `la`, `lla`, and `lt` shortcuts only when the binary is present.
  Chezmoi deploys `~/.config/lsd/config.yaml` and `colors.yaml`; shell profiles
  install the Rose Pine `LS_COLORS` palette by default so direct `lsd -la`,
  aliases, zsh completions, and PowerShell functions share deterministic
  file/directory colors across machines. Set `DOTFILES_LS_COLORS` before shell
  startup for an explicit palette override; `NO_COLOR` remains the standard
  color opt-out.
- `install-deps` provisions the `cmake` CLI because the configured CMake LSP
  (`neocmakelsp`) shells out to it; Mason installs the language server, not the
  project toolchain it drives.
- `install-deps` provisions the `tree-sitter` CLI for `nvim-treesitter` main:
  Homebrew on macOS/Linuxbrew, a pinned SHA-256-verified GitHub release into
  `~/.local/bin` on native Linux/WSL, and the exact SHA-256-verified Windows
  release into `%LOCALAPPDATA%\dotfiles\bin`. Windows places that owned
  directory first in both the running process and User `PATH`, even when the
  directory was already present later in the list; an incompatible shadowing
  install remains installed but no longer wins command resolution.
  Windows `-All` also installs VS Build Tools so parser builds can find MSVC;
  after winget/choco failures it falls back to Microsoft's official
  `vs_BuildTools.exe` bootstrapper with the same VCTools workload, but only
  after Authenticode verifies a valid Microsoft-owned signer/chain.
- Neovim Markdown rendering is owned by `render-markdown.nvim`. Setup already
  installs the explicit Tree-sitter parser matrix, including `latex`; it also
  installs `latex2text` through a pinned, SHA-256-checked venv
  (`setuptools` 80.9.0, `pylatexenc` 2.10) so rendered Markdown equations work
  on fresh machines instead of depending on a random host Python package. On
  Linux, setup repairs the active distro Python's native venv/pip support even
  when Linuxbrew is the selected package manager; manager selection alone does
  not prove that `python3` on PATH is Homebrew-owned.
- `install-deps` prints a dependency pre-flight table before package-manager
  bootstrap and before the one-shot install prompt, showing the package manager
  itself, present/missing tools, best-effort versions, and the resulting
  skip/install action. The table is informational; the existing per-tool install
  logic still decides what actually runs.
- Accepted install failures are fatal to setup. Every recoverable POSIX install
  step runs through one accumulator boundary: an early archive, downloader,
  package-manager, plugin, Pi, or converter failure is recorded exactly once,
  independent later installs still run, and the consolidated summary exits
  nonzero. `setup.sh`, `setup.ps1`, and `setup.ps1 -Update` exit nonzero
  afterward; dry-run previews and explicit/manual skips remain non-failures.
- `setup.sh --update` and `setup.ps1 -Update` first run the same idempotent full
  release reconciliation as all mode, then enter a scoped, manager-aware refresh.
  The refresh updates only present catalog tools with proven per-tool ownership, then runs an
  exact per-package or repo-pinned artifact refresh such as
  `brew upgrade <formula>`, `apt-get install --only-upgrade <pkg>`,
  `scoop update <pkg>`, `winget upgrade --id <id> -e`, or
  `choco upgrade <pkg> -y`. They never run blanket upgrades such as
  `brew upgrade`, `apt upgrade`, `pacman -Syu`, `scoop update *`,
  `winget upgrade --all`, or `choco upgrade all`. Unix ownership is resolved
  from the executable source: Homebrew/Linuxbrew requires the PATH-visible
  command path and its resolved executable target to stay under `brew --prefix`,
  plus an installed formula and `brew list --formula <formula>` file ownership
  of the resolved executable. The catalog formula is the install default, not an
  ownership guess: an active versioned formula such as `python@3.14` is resolved
  from its Cellar target and receipt before a scoped update. Native Linux managers require file ownership proof
  (`dpkg-query -S`, `rpm -qf`, `pacman -Qo`, or `apk info --who-owns`);
  dotfiles-owned Linux artifacts require a durable provenance marker with the
  expected version, URL, SHA-256, command path, binary path, install root,
  installed-binary SHA-256, matching `--version` output, and a repo-managed
  install shape: Neovim is `/usr/local/bin/nvim` pointing into
  `/opt/nvim-linux-*`; lazygit and Starship are `/usr/local/bin/<tool>` or
  `~/.local/bin/<tool>`; tree-sitter, chezmoi, and Herdr are
  `~/.local/bin/<tool>`. Shadow command paths, Brew-prefix symlinks that escape the Brew prefix,
  unsupported artifact roots, and marker binaries outside the recorded install
  root are blocked provenance failures, not ownership. Output distinguishes
  `updated`, `current`, `system`, `unmanaged`,
  `blocked`, and `skipped`. `blocked` fails update mode;
  `unmanaged` reports the source path and exits successfully. On macOS,
  `/bin/zsh` is accepted as `system`, while normal Homebrew developer tools
  that still resolve from `/usr/bin` get an unmanaged line with a Homebrew
  migration hint. Setup also persists Homebrew shellenv and Homebrew GNU Make's
  `libexec/gnubin` path when the `make` formula is installed, so Brew-owned
  `make` does not require a manual export. Homebrew may intentionally emit no
  `shellenv` output when its bin/sbin already lead PATH; setup accepts that
  idempotent result only when the selected and resolved commands prove the same
  canonical Homebrew prefix and repository. This deliberately supports a
  nix-darwin `/run/current-system/sw/bin/brew` wrapper activating the matching
  architecture-native `/opt/homebrew` or `/usr/local` entrypoint without
  accepting a different installation. Failed evaluation or identity proof restores the current
  shell's prior PATH/Homebrew variables and fails with retry instructions. A
  required macOS bootstrap/activation failure prints the consolidated failure
  summary before any package install is attempted. On
  Windows, Scoop-owned tools are
  detected from shim metadata before package-list fallback; corrupt Scoop shims
  are `blocked`; winget and Chocolatey require both package-list ownership and a
  command source under that manager's supported install roots, so a manual
  `C:\Manual\...\pwsh.exe` is `unmanaged` even if a package row exists. Windows
  managers report `current` when their non-mutating availability probes have no
  exact available-upgrade row for the package. Scoop status rows with unhealthy
  `Info` or `Missing Dependencies` fields fail closed instead of updating.
- zsh plugins are installed by Unix setup as repo-managed pinned git checkouts:
  `fzf-tab` and `zsh-autosuggestions` live under
  `~/.local/share/dotfiles/zsh-plugins`. `zshrc` sources those copies first and
  falls back to Homebrew/system paths only when the managed copy is missing.
  Install-deps and the pin/helper-sensitive chezmoi `run_onchange` path share a
  checked publisher: it
  serializes concurrent starts, quarantines an unproved payload before any
  fetch, stages the exact commit beside the target, proves origin, HEAD,
  cleanliness, worktree identity, and the tracked plugin entry file, then
  publishes atomically. A clean old pin self-heals; dirty/wrong-origin/partial
  payloads remain quarantined for recovery and are never left sourceable.
  Completion is `fzf-tab` (an fzf-driven fuzzy Tab menu over native `compinit`)
  — it loads *after* `compinit` and *before*
  `zsh-autosuggestions`, and reclaims Tab after fzf's own key-bindings. This is
  the PowerShell-PSReadLine analog (Tab menu + inline gray history prediction);
  see CLAUDE.md invariant 13.
- Linux without Homebrew gets Neovim from a pinned official GitHub release
  tarball installed into `/opt/nvim-linux-<arch>` and symlinked to
  `/usr/local/bin/nvim`. The tarball SHA-256 is verified before extraction.
- macOS and Linuxbrew install lazygit through Homebrew (`brew install lazygit`).
  Alpine installs the native `lazygit` apk package. Other native Linux/WSL
  hosts without Homebrew get lazygit from a pinned GitHub release tarball with
  SHA-256 verification. Setup installs it to
  `/usr/local/bin/lazygit`, or falls back to `~/.local/bin/lazygit` when sudo is
  unavailable.
- macOS installs Ghostty through `brew install --cask ghostty` when selected.
  Supported Debian-family native Linux resolves the exact pinned
  `mkasberg/ghostty-ubuntu` release asset for distro and architecture, verifies
  its reviewed SHA-256 plus `Package`/`Architecture`/`Version` metadata,
  installs only that local `.deb`, and validates the installed dpkg version and
  command. It never executes the upstream installer, whose mutable
  `releases/latest` lookup does not bind the downloaded package bytes. WSL
  defaults to Windows Terminal on the Windows host; Linux Ghostty in WSL
  requires `--experimental-wsl-gui`.
- WezTerm installs from the vendor channel per OS: `brew install --cask wezterm`
  on macOS, the `wez.wezterm` Scoop/winget/choco catalog entry on Windows, and
  the official pinned, SHA-256-verified `.deb` on amd64 Ubuntu. WezTerm is a GUI
  terminal, but native Linux package installation does not require a display at
  install time. Split-host WSL skips Linux WezTerm unless
  `--experimental-wsl-gui` is set; arm64 Linux / non-Ubuntu get manual guidance.
  The Rose Pine + Hack Nerd Font + transparency config is chezmoi-owned, never a
  Nix/nixpkgs GUI package.
- AeroSpace (macOS-only i3-like tiling WM) installs from the official tap cask
  (`brew install --cask nikitabobko/tap/aerospace`), `start-at-login = true`,
  chezmoi-owned config. The Homebrew-owned tap is explicitly trusted so Homebrew
  5 will load the cask. Its keymap deliberately avoids the
  reserved chords:
  window focus/move live on `ctrl-alt(-shift)-h/j/k/l` so they never shadow
  Neovim's `<A-h/j/k/l>` window navigation, and nothing uses `Alt-c` (fzf-tab /
  PSFzf `cd`). On first launch grant it Accessibility permission (System Settings
  -> Privacy & Security -> Accessibility) — a TCC grant that cannot be scripted.
  AeroSpace reads the XDG path this repo manages; a legacy `~/.aerospace.toml`
  conflicts loudly with that model and should be removed or migrated before
  judging the managed config. Not a Nix/nixpkgs package.
- Herdr (agent multiplexer) installs on every host, but the channels differ:
  `brew install herdr` (homebrew-core) on macOS and Linuxbrew, a pinned,
  SHA-256-verified GitHub release binary on native Linux without brew, and a
  pinned, SHA-256-verified **Windows preview** `.exe` under
  `%LOCALAPPDATA%\Programs\Herdr\bin` on native Windows. The `herdr.dev`
  remote-eval installers remain banned. Windows setup refreshes a stale binary
  only when command resolution points at that exact repo-owned path; an
  unrelated installation remains untouched. Native-Linux Herdr writes the same
  provenance marker as the other dotfiles-owned direct artifacts, so
  `./setup.sh --update` can prove ownership and refresh only the repo-pinned
  version. Chezmoi also installs the same deterministic config on every host:
  Herdr's built-in `rose-pine` theme with onboarding and automatic light/dark
  switching disabled. Windows consumes its platform config from the
  independently resolved roaming `%APPDATA%` folder and explicitly launches
  `pwsh.exe`; this keeps new panes on the managed PowerShell 7 profile and the
  same PSReadLine ListView/history experience as Windows Terminal instead of
  falling back to Windows PowerShell 5.1. Existing panes retain their original
  shell and must be recreated after this setting changes. On every host,
  `Ctrl+B`, then `w` (or `g`) opens Herdr's full workspace/tab/pane navigator;
  use Up/Down and Enter to select. The tmux-shaped bindings use `Ctrl+B`, then
  `,` to rename the current tab/window and `Ctrl+B`, then `$` to rename the
  current workspace. `Ctrl+B`, then Up/Down moves between workspaces, while
  `Ctrl+B`, then Shift+1..9 jumps directly to workspace 1..9; unshifted
  `Ctrl+B`, then 1..9 remains tab/window selection. Herdr `v0.7.3` incorrectly
  rejected the punctuation keycodes produced by shifted digits; the repo pins
  stable `v0.7.4` and a post-fix Windows preview. Named Herdr sessions are separate server
  namespaces, so they are attached from the shell rather than listed inside
  another session's navigator. Windows Herdr is
  beta/ConPTY-backed, so runtime behavior remains a manual checklist item before
  treating it as a daily driver.
- WSL fonts are host-rendered in the supported path. Install and merge Windows
  Terminal from Windows (`.\setup.ps1 -All`; the merge is default-on); the WSL
  Linux fontconfig install is only for `--experimental-wsl-gui`.
- VS Code is optional. On WSL, use Windows VS Code plus `code .` for Remote -
  WSL, or use a Linux GUI build when WSLg / X11 is available. Rosé Pine setup
  follows whatever `code` CLI is on PATH.
- Sentinel agent policy is a supported setup phase, not a synced dotfile. `setup`
  pins Sentinel's renamed `0.1.2` tree at exact commit
  `ecafffa858666343c1639f996d177f460163e93e`,
  caches that checkout under `~/.local/share/dotfiles/sentinel/<commit>` on
  POSIX and `%LOCALAPPDATA%\dotfiles\sentinel\<commit>` on Windows, verifies that
  exact commit plus the checkout `VERSION`, and runs every Sentinel Git operation with
  system/global/env config, templates, hooks, and executable Git config features
  disabled, then runs Sentinel's Bash global installer and global check
  (`tools/install --global`, then `--global --check`; Windows uses a validated
  Git Bash with `cygpath`, not WSL bash or another PATH-only Bash).
  The published `v0.1.2` tag predates the repository rename and is deliberately
  not treated as the renamed tree's identity; the exact commit is the immutable
  authority until Sentinel publishes a tag containing that tree.
  The global installer writes the per-user AI entrypoints for Codex
  (`~/.codex/AGENTS.md`), Claude Code (`~/.claude/CLAUDE.md`), opencode
  (`~/.config/opencode/AGENTS.md`), and Pi CLI (`~/.pi/agent/AGENTS.md`);
  Copilot has no reliable global file path, so user-wide Copilot instructions
  remain a manual VS Code/github.com profile step. To remove the global blocks,
  run the cached Sentinel Bash installer with `--global --remove` on POSIX or
  from Git Bash on Windows. Project/team adoption is separate: run Sentinel
  repo-local install or vendoring in that project and commit those files there.
- Pi CLI is a provisioned binary with one repo-owned presentation default, not
  synced runtime state. Setup installs
  `@earendil-works/pi-coding-agent@0.80.9` by running `npm pack`, requiring the
  pack metadata and the actual tarball SHA-512 bytes to match the reviewed SRI,
  then passing that verified local tarball plus the exact `0.80.9` Pi
  `agent-core`, `ai`, and `tui` companions to `npm install`. Keeping the Pi
  monorepo packages on one release prevents compatible-looking npm ranges from
  mixing runtime APIs. Temporary pack state is removed on success, mismatch,
  failure, interruption, and retry.
  The reviewed SRI is
  `sha512-Clgx2Bg5NbMcCpGxusSDQwE+GC0g/d6sCBluE9aypPgSgtJ6n8VmZIIT6auXObMskpRgkr+XZ77wG5hf+cSDtg==`.
  POSIX public setup gets Node 24 from Nix first; Windows uses the native Node
  LTS catalog entry. On POSIX, Pi is published under `~/.local/bin`; setup and
  managed zsh keep that directory first and duplicate-free so an older global
  npm/Homebrew copy cannot shadow the verified CLI. Only that canonical path
  satisfies setup's installed-version proof. Setup warns about every other
  active `pi` path without executing or deleting it; when a sibling npm command
  proves ownership, the warning includes the exact same-user, no-`sudo`
  uninstall command. Chezmoi deploys the audited `pi/rose-pine.json` theme and
  setup atomically merges only `theme: rose-pine` into
  `~/.pi/agent/settings.json` under Pi's `settings.json.lock` convention.
  Invalid or busy settings fail without mutation; all unrelated keys remain.
  Local `.pi/` sessions, credentials, providers, and other preferences stay
  machine-local.
- Dependency setup finishes with a cross-platform duplicate-command audit for
  every managed CLI in its install inventory. The first physically distinct
  command on `PATH` is the selected runtime authority; later commands with the
  same name are reported without being executed or removed. Symlinked aliases
  of the same executable and immutable OS fallbacks (`/usr/bin`, `/bin`,
  Windows `System32`, and Windows app-execution aliases) are not treated as
  competing installations. POSIX ownership probes recognize exact Homebrew,
  npm-global, and Nix sources; Windows recognizes Scoop, winget, and Chocolatey
  sources. Exact uninstall commands are printed only for proven user-scoped
  Homebrew, npm, or Scoop packages. System/global managers are identified but
  still require a deliberate scope review before removal.
- Native Windows accepts an existing Tree-sitter CLI only when `tree-sitter
  --version` is exactly `0.26.10`. Missing, stale, partial, or incompatible
  commands are repaired from the architecture-specific `v0.26.10` GitHub
  release zip after SHA-256 verification; the executable is validated before
  and after atomic publication under the real LocalApplicationData known folder.
  Scoop/npm are not allowed to silently supply a different parser-build ABI.
- Notes / Obsidian support writes `export NOTES_VAULT=...` to
  `~/.zshrc.local` (gitignored, sourced by `zshrc`). Non-interactive runs skip
  the prompt, so set `NOTES_VAULT` yourself there.
- macOS installs before this README may have an unused
  `~/.config/lazygit/config.yml` symlink. It is harmless; current setup paths
  manage the location lazygit actually reads:
  `~/Library/Application Support/lazygit/config.yml`.

## Test

Use the same top-level test command that CI uses for your OS:

```bash
# mac / linux / wsl
make help               # list targets
make ci                 # full local pre-PR gate: test + Renovate + migration
make test               # current-host fast suite
make validate-renovate  # schema + official local extraction inventory under Node 24
make lint               # shellcheck everything
./tests/wsl/e2e.sh      # manual WSL split-host validation from inside WSL
./tests/greenfield/docker-greenfield.sh # local clean Ubuntu container e2e
./tests/greenfield/docker-linux-owner-lifecycle.sh # Linux install/update/uninstall/reinstall/update
```

`make lint` checks production shell scripts strictly. Source-only shell test
fixtures get a reviewed shellcheck false-positive exclude for dynamic `source`
paths, globals consumed by sourced installer functions, and indirectly invoked
command stubs; other shellcheck findings still fail.

```powershell
# windows
.\test.ps1          # PSScriptAnalyzer + Pester + Nvim plenary busted
```

Windows runs each Neovim Plenary spec file directly through `plenary.busted`;
do not switch it back to `PlenaryBustedDirectory`, whose parent harness can
false-fail after successful child specs under PowerShell native-command error
promotion.

Unix runs Neovim specs through `PlenaryBustedDirectory` with an explicit timeout
so the startup-budget spec reports its own assertion instead of being killed by
Plenary's default timeout. The startup-budget spec preclones the locked plugin
checkouts into isolated XDG dirs before measuring warm production init; it must
not invoke Lazy install/restore or leave nvim-treesitter parser outputs in that
cache, because parser builds are setup/bootstrap work rather than startup work.
Before timing, the prewarm also proves `lazy.nvim` through the same reviewed
origin, locked branch, commit, cleanliness, and entry-file boundary as production,
so stale upstream default-branch metadata is repaired outside the benchmark.
It emits `[startup_spec]` progress lines before plugin prewarm and each child
init so a parent timeout leaves the run root and last long operation in logs.

Sub-targets skip themselves with a `skipped: <tool> not installed` message
when their dependency tool is missing on the current machine. In CI, missing
Windows test dependencies are fatal so the workflow cannot go green by silently
skipping the actual checks.

For local clean-machine validation, see
[`tests/greenfield/README.md`](tests/greenfield/README.md). It wraps the
existing Ubuntu container e2e path, adds Windows Sandbox and throwaway WSL
launchers, documents macOS fresh-user/VM options, and includes the shared
post-install validators plus the manual desktop visual checklist.

## CI Merge Gate

Pull requests are meant to be gated by three required workflow families:

- `.github/workflows/test.yml` runs the static, shell, tmux,
  starship, Neovim, Windows Pester/PSScriptAnalyzer, Renovate schema, and
  `chezmoi-parity` suites. Warnings are treated as failures where the tools
  expose them cleanly: shellcheck exits nonzero, PSScriptAnalyzer scans the
  meaningful `.ps1` surface and binds every reviewed warning to a normalized
  script/rule/message/extent fingerprint (not filename/rule/count alone),
  Renovate validation fails if `npx` is missing under CI or its official local
  extraction differs from the checked-in dependency inventory, and YAML
  parsing/linting—including the Ruby/Psych semantic Settings policy—is part of
  `make test-static`. Windows PSGallery module
  installs retry transient lookup failures, but missing test dependencies remain
  fatal.
- `.github/workflows/e2e-install.yml` is the real install guarantee. It proves
  the public setup paths on fresh hosted runners and keeps one clean Ubuntu
  container for the native `apt` branch. Its setup caches include the
  `actions/cache` major version in their keys, so a cache-action major upgrade
  proves itself with a fresh archive instead of reusing one produced by the
  previous major.
- `.github/workflows/nix.yml` validates the enforced POSIX Nix layer with
  `nix flake check` on Ubuntu and macOS plus the checked-in Nix/setup
  integration assertions.

There is no hosted WSL workflow. [GitHub states that nested virtualization on
hosted runners is technically possible but not officially supported](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners), and the
former optional WSL2 canary twice hung until cancellation without producing
setup evidence. A Linux container with WSL-shaped environment variables would
be fake proof, so WSL remains supported through the real throwaway-distro and
split-host manual harnesses in `tests/greenfield/` and `tests/wsl/`.

The e2e jobs cover different install paths, not symmetric container platforms:

| Check | What it proves |
|---|---|
| `e2e containers / ubuntu-24.04` | Clean `ubuntu:24.04`, non-root user, native `apt`, no Linuxbrew (`DOTFILES_SKIP_BREW_BOOTSTRAP=1`), then `install-deps.sh --all`, chezmoi config apply, executable/version probes including `zoxide`, `gh`, WezTerm, Herdr, Neovim >= 0.12, lazygit, zsh plugin files, config content assertions, and nvim directory realpath assertion. This is the native installer regression fixture, not the Nix-backed public POSIX package-plane proof; it does not assert Pi CLI because Node 24 comes from the Nix package layer. |
| Local Linux owner lifecycle | Pinned Ubuntu and Nix container images, non-root user, real Home Manager plus native `apt`, then install, update, config uninstall, idempotent uninstall retry, reinstall, final update, full validation, and proof that no pre-existing native package disappeared. |
| `setup.sh / ubuntu-24.04` | Full public Unix setup on the hosted Ubuntu runner after installing Nix in CI: Home Manager first, then native/deferred installs, chezmoi, Lazy, Tree-sitter, Mason, and Sentinel. Its clean login/interactive PATH proof resolves the effective account's actual login zsh from the account database; this matters because fresh Ubuntu has no `/usr/bin/zsh` and setup selects Linuxbrew zsh. The shell must resolve `rg` from Nix with no caller PATH injection. |
| `setup.sh / macos-26` | Full public Apple Silicon setup through the hosted runner: architecture-matched nix-darwin/declarative Homebrew, native/deferred installs, real Ghostty/WezTerm config consumption, installed AeroSpace app/CLI identity agreement, chezmoi, Lazy, Tree-sitter, Mason, and Sentinel. AeroSpace waits for a user-granted Accessibility permission before parsing user config or starting the CLI server, so managed-config consumption remains explicit TCC-enabled desktop proof in `tests/MANUAL.md`; hosted CI does not pretend to prove it. Hosted and real Macs share the same mixed-ownership Homebrew contract: declared packages are applied, tap clones stay target-user-owned, and unrelated user state is preserved. |
| `setup.ps1 / windows-2025` | Full Windows setup through the real Windows hosted runner, including Scoop/winget/choco behavior, PowerShell, symlinks, Hack Nerd Font file/registry consumption, `zoxide`/`gh`/WezTerm/Herdr/Pi CLI command probes, and Neovim restore/sync phases. Windows containers do not model the desktop/user-profile setup well. |

After the Lazy restore, deterministic Tree-sitter parser install, and Mason sync, each
`setup.sh`/`setup.ps1` job also
runs the **Tier 2 language smoke** (`tests/nvim/lsp_smoke.lua`): against the
real Neovim config it asserts (0) no nvim-treesitter parser override for a
bundled language remains on the runtimepath under `stdpath('data')`, and no
managed nvim-treesitter query directory for a bundled language remains in the
query install output, (1) every declared treesitter parser is one nvim-treesitter
`main` supports, every expected parser `.so` and query directory is actually present in
nvim-treesitter's installed output, and every explicit parser row has a managed
`highlights.scm` query (including upstream paired parsers such as
PHP's `php_only` and query-only dependencies), and no unexpected
nvim-treesitter install-output parser `.so` is present under
`stdpath('data')/site/parser` beyond the explicit list plus non-bundled upstream
dependency parsers, (2) each language's LSP attaches (`powershell_es` enforced on Windows
only), (3) realistic formatter-owned buffers copied under `tests/.cache` format
through conform.nvim's production route with the expected external formatter(s)
and produce no LSP warnings/errors afterward, (4) every language-matrix fixture
opens with the expected filetype and every parser-backed row reports real
Tree-sitter highlight captures after the smoke explicitly starts and parses the
expected parser, using `inspect_pos()` first and direct highlight-query capture
iteration as the headless fallback, and (5) the auto-started bundled filetypes keep
nvim-treesitter's `indentexpr`. The fast `make test-nvim` runs Tier 1
(filetype + formatter + parser-list consistency per fixture), plus source-shape
guards for formatter policy such as JSON-family Prettier trailing commas. Adding
a language is "drop a fixture + a row in
`tests/nvim/language_matrix.lua`"; syntax-only fallbacks such as `.curlrc`
belong in the same matrix, must not pretend to have unsupported parsers, and
Tier 2 syntax probes must prove they still produce real Vim syntax groups. The
smoke matrix also encodes the
Neovim-bundled languages (`c`, `lua`, `markdown`, `query`, `vim`): those must
stay **out** of the install list — and any stale override of them is purged on
config load (parser files scoped to `stdpath('data')`; query directories scoped
to nvim-treesitter's managed `get_install_dir("queries")` output, which must
also live under `stdpath('data')`, so Neovim's own install-prefix runtime is
never touched) — so Neovim's matched built-in parser+query is used instead
of an nvim-treesitter parser that can drift from the bundled query (this caught
a real lua `E5113: Invalid field name "operator"` regression).

The Ubuntu container is intentionally **not** a devcontainer. It stays because
the hosted Ubuntu runner can take the Linuxbrew path, while the container is the
only automated proof of the clean-image native `apt` path: the pinned Neovim
tarball install, pinned lazygit release install, zsh plugin install, `fd-find`
-> `fd` shim, and apt fallback behavior. There is no matching macOS or Windows
container to add for symmetry. That asymmetry is accepted: hosted macOS and
Windows runners are the closest representative fixtures for those operating
systems, while the required WSL proxy is the Ubuntu container plus the
WSL config-template coverage. Those checks do not claim WSL runtime proof; use
the real manual throwaway-distro harness for that boundary.

These e2e jobs fail if setup skips Phase 3-5, omits Phase 6/6, leaves a
Nix-owned POSIX CLI resolving outside a Nix profile/store path, emits a precise
`FAIL:` marker, installs Neovim below 0.12, Lazy restore / Tree-sitter parser
install / Mason sync exits nonzero, or expected Mason-installed binaries are
missing. They do not blanket-fail on benign warning/deprecation text.

## Repository Safeguards

The canonical main-branch safeguards are the three checked-in repository rulesets
under `.github/rulesets/`, applied live with
`scripts/apply-repo-safeguards.sh`. `.github/settings.yml` deliberately limits
the Probot Settings app to repository-level settings; it does not contain a
`branches` section. This prevents a default-branch sync from racing the
transactional script and creating a mixed classic/ruleset cutover stage. The
script owns the classic fallback required-check transition because Probot
cannot model the key policy split: owner review bypass is allowed, CI bypass is
not, and only the owner may update `main`. A semantic YAML policy test rejects
the top-level key in block, inline, null, and alias forms; line-oriented text
matching is not accepted as proof of ownership isolation.

| Layer | Bypass actors | What it protects |
|---|---:|---|
| `Protect main: integrity` | none | Requires pull requests, strict required checks, CodeQL merge protection, current `main`, squash-only merges, linear history, no branch deletion, and no non-fast-forward updates. |
| `Protect main: review` | `luisgui1757` on pull requests only | Requires one approval, CODEOWNER review, stale-review dismissal, last-push approval, and resolved review threads without allowing CI bypass. |
| `Protect main: owner updates` | `luisgui1757` on pull requests only | Allows only the owner to update `main`; automation can open PRs but cannot merge them. |
| Classic branch protection fallback | none | Keeps required checks, enforced admins, conversation resolution, linear history, no force pushes, and no branch deletion if rulesets are not applied. |

Repository settings are squash-only: merge commits and rebase merges are
disabled, squash merges are enabled, branches are deleted after merge, and
repo-level auto-merge stays disabled. GitHub does not let pull request authors
approve their own PRs; owner-authored PRs use the owner review bypass, but only
after the non-bypass integrity layer has passed.

Automation must not run as the owner account. Use a separate GitHub App or bot
identity with branch/PR write access and no repository administration
permission; otherwise GitHub sees the action as `luisgui1757`.

GitHub-native security is also checked in rather than left as UI-only state.
`scripts/apply-github-security.sh` enables private vulnerability reporting,
immutable releases, and CodeQL default setup for GitHub Actions plus the real
Python scripts. After both analyses pass on exact live `main`, it adds the
non-bypassable CodeQL rule to `Protect main: integrity`, blocking error-level
findings and high/critical security alerts. Suspected vulnerabilities must be
reported through the private process in [SECURITY.md](SECURITY.md), not a public
issue.

GitHub Code Quality and the advanced non-provider/validity secret-scanning
options are not included in a user-owned GitHub Pro repository. They require an
organization-owned Team/Enterprise or Secret Protection plan; the repository
does not pretend those unavailable controls are active.

Runner-versioned required contexts are in their final staged migration. This PR
switches the checked-in required-check sources to six stable logical checks
(`nix flake check / {linux,macos}`, `e2e containers / linux`, `setup.sh /
{linux,macos}`, and `setup.ps1 / windows`) while workflows continue emitting
the six legacy producer names. Each logical check verifies the exact OS proof
marker's source head SHA, actually executed SHA, workflow run, logical identity,
and legacy producer; these are not no-op checks. Pull-request jobs execute
GitHub's synthetic merge result, so the artifact records that commit separately
from the PR branch head. Live GitHub remains on the legacy set so it can gate
this PR safely. Only after this PR merges and the cache-free producers plus all
six logical checks pass on that exact `main` SHA should the owner apply the
checked-in stable contexts. See
[branch-protection.md](docs/security/branch-protection.md) for the exact order.

Manual owner step:

```bash
scripts/apply-repo-safeguards.sh --preflight-only luisgui1757/dotfiles
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

with an authenticated `gh` that has repository admin permission. Do this only
after the post-merge cache-free and logical proof gate passes. Before any live
write, the apply command repeats the complete preflight: exact local/main/origin
identity, public repository visibility, clean reviewed sources, exact legacy
live posture, unique rulesets, GitHub Actions app/workflow/event/run provenance,
and skipped broad caches. It captures a private recovery snapshot and, before
restoration, freezes every consumed file and validates its exact manifest stage,
contexts, app IDs, ruleset identity, bypass/branch policy, Actions pinning, and
complete classic state, including every nullable-but-required field. Restore
derives its expected policy from the exact captured commit, not mutable current
worktree bytes, and requires that commit still be live `main`. After the second
preflight capture, apply independently freezes the committed check
metadata, integrity payload, manifest, classic payload, and Actions payload in a
private read-only transaction directory and publishes only those validated
files. Worktree changes after validation therefore cannot influence a later
write. Missing, malformed, altered, or cross-stage recovery material fails
before any write; failed captures clean every temporary directory, and a
recovery snapshot is retained only after mutation begins or a verified apply
succeeds. Apply mutates only the three cutover resources and automatically
restores the previous stage if apply or readback fails. See
[docs/security/branch-protection.md](docs/security/branch-protection.md) for the
exact validation, recovery command, and live verification sequence;
[docs/security/supply-chain.md](docs/security/supply-chain.md) records the
reviewed executable identities and scanners.

After that stable-context transaction is applied and verified, merge the
security-policy change and run:

```bash
scripts/apply-github-security.sh --preflight-only luisgui1757/dotfiles
scripts/apply-github-security.sh luisgui1757/dotfiles
```

The ordering is mandatory: the security script compares the live integrity
ruleset to the already-applied stable-context payload before it adds CodeQL
merge protection.

Renovate is the version-update bot for GitHub Actions, direct and matrix-held
runner labels, Nix flake inputs, and repo-pinned version/ref constants. The beta
Nix manager is explicitly enabled, Scoop tracks upstream `master`, and bot
branches rebase whenever behind `main`. GitHub-native Dependabot security alerts and automated
security fixes stay enabled through `.github/settings.yml`; Dependabot version
update PRs are intentionally not configured.

| Surface | Renovate policy | Reason |
|---|---|---|
| GitHub Actions | Managed, digest-pinned, labeled `github-actions` | Actions are repo-owned CI inputs with stable Renovate support. |
| GitHub runner images | Managed, labeled `github-runners`, reviewed separately | `ubuntu-*`, `macos-*`, and `windows-*` bumps can change the supported CI platform, so they should not be mixed with ordinary Action bumps. |
| Repo-pinned installer versions/refs | Managed, labeled `pinned-downloads`, never automerged | The v0.2.0 Nix prerequisite tarballs, Neovim Linux tarballs, chezmoi CI release archives, lazygit Linux tarballs, Starship Linux tarballs, Tree-sitter CLI Linux/Windows archives, WezTerm Ubuntu `.deb`, Herdr Linux binaries, Herdr Windows preview `.exe`, Hack Nerd Font, Windows Terminal portable zip, Ghostty distro/architecture `.deb` assets, Pi CLI npm package, zsh plugin refs, `setuptools`/`pylatexenc` converter pins, the Homebrew installer commit, the Scoop installer commit, the Renovate validator package/runtime, the Ubuntu Microsoft-repository `.deb`, and the CI `cargo-binstall` commit are explicit repo pins. |
| Adjacent SHA-256 / commit constants | Not managed; matched only as regex context | Renovate can bump the version/ref but cannot recompute archive/script hashes or verify tag commit IDs. CI must fail until a human recomputes and reviews them. |
| Package-manager catalogs | Not managed | Brew, apt, dnf, pacman, zypper, apk, Scoop, winget, and choco entries are package names/IDs, not repo version pins. Let the package manager resolve current versions. |
| Neovim plugin and Mason tools | Not managed | `lazy-lock.json` is refreshed with Lazy and tested as editor behavior; Mason intentionally has no machine-pinned lockfile. |

`make validate-renovate` runs Renovate's own strict schema validator and
`--platform=local --dry-run=extract`, captures its JSON stdout directly, then
compares the official extraction to
`tests/static/renovate_expected_inventory.txt`. A custom regex merely matching
some text is not ownership proof. Dashboard #7 reran against merged `main` at
2026-07-10 13:17 UTC and exposed the reviewed Nix, runner, and Scoop `master`
inventory without lookup problems; future config changes require a fresh bot
result rather than inheriting that claim.

Manual-review pin surfaces that Renovate may touch only partially:

| Surface | Status |
|---|---|
| Sentinel version/commit | Manual-reviewed mirror between `setup.sh`, `setup.ps1`, README, CLAUDE, and `tests/static/pin_consistency_test.sh`. The current renamed tree is exact-commit pinned because the published `v0.1.2` tag predates it. |
| Scoop installer | Renovate can bump `ScoopInstaller/Install` commit `b0ee913725139b816f9178163af0aecdba07a7ed`; SHA `48f6ea398b3a3fa26fae0093d37bd85b13e7eaa5d1d4a3e208408768408e35ae` is human-reviewed. |
| TPM/tmux plugin refs and psmux plugin ref | Commit pins are manual-reviewed and mirrored in docs/tests; Renovate does not recompute or prove tag commits. |
| `setuptools`/`pylatexenc` | Renovate can bump versions; adjacent hashes remain human-reviewed. Current pins: `setuptools` 80.9.0, `pylatexenc` 2.10. |
| Hack Nerd Font | Unix and Windows mirrors must stay identical; version/hash drift is caught by `pin_consistency_test.sh`. |
| Pi CLI | Unix/Windows install pins and e2e assertions mirror version `0.80.9`; the npm-pack metadata and downloaded coding-agent tarball bytes must both match the human-reviewed SRI, and all three Pi companion modules are requested at the exact same release. |
| Herdr | Native Linux pins stable `v0.7.4` with both architecture hashes; Windows pins post-fix preview `preview-2026-07-16-e907e6a36646` with its x64 hash. Homebrew platforms consume the reviewed `v0.7.4` formula. |
| Pi Rose Pine theme | The repo vendors only `rose-pine.json` from audited data-only package `pi-themes-rose-pine@0.1.0`, preserves its MIT license, and tests the exact palette plus all 51 Pi tokens. |
| gh-dash | Tag `v4.25.1`, annotated tag object `e6ebbd7e83e30161b9192ce3339972d2c8269e7f`, and peeled commit `49f37e4832956c57bf52d4ea8b1b1e5c0f863700` are mirrored; installers verify the tag mapping and pass the release tag required by `gh extension install --pin` for binary extensions. |

Direct network executables must be pinned and verified before execution, or be a
reviewed exception whose verification is proved by the static scanner. Scoop
bootstrap on Windows now downloads `ScoopInstaller/Install` from a pinned commit,
verifies the installer SHA-256, then executes the local temp file (using
`-RunAsAdmin` only for elevated CI). Homebrew bootstrap is downloaded from a
pinned installer commit and SHA-256 verified before execution. Microsoft's
moving `vs_BuildTools.exe` alias is the reviewed exception: after download,
`install-deps.ps1` requires a Valid Authenticode signature and Microsoft-owned
signer/chain before `Start-Process`. Recommended setup docs use `git clone`
plus local `setup`, not raw `curl | bash`/`iwr` execution of the current default
branch.

Direct-download SHA-256 values for the upstream Nix prerequisite, Neovim tarballs, chezmoi CI release archives,
lazygit tarballs, Starship tarballs, tree-sitter CLI archives, the WezTerm
Ubuntu `.deb`, Herdr Linux binaries, Herdr Windows preview `.exe`, Hack Nerd Font, the Windows Terminal
portable zip, the Ghostty Debian-family `.deb` assets, the Ubuntu 24.04 Microsoft
repository `.deb`, Homebrew installer script, Scoop
installer script, and the CI `cargo-binstall` installer script are
intentionally human-reviewed. zsh plugin tag commits and tmux/psmux plugin
commits are also human-reviewed because the installers verify the checked-out
commit after cloning/fetching. Do not capture direct-download
SHA constants as Renovate `currentDigest` values: that creates
noisy/unresolvable digest updates instead of a trustworthy checksum review. A
Renovate PR may bump the version/ref while leaving the adjacent SHA or commit
stale; CI then fails verification until a human reviews the adjacent constant.

## Repo layout

```
.
├── nvim/                  # Neovim — init.lua, lua/{vim-options,util,plugins}
├── starship/              # starship.toml (Rose Pine)
├── lsd/                   # config.yaml + colors.yaml (Rose Pine)
├── gh-dash/               # config.yml (gh-dash PR/issue dashboard)
├── shells/                # zshenv + zshrc + powershell_profile.ps1
├── tmux/                  # tmux.conf (Rose Pine, vi-mode, true-color)
├── ghostty/               # config (Rose Pine, Hack Nerd, Ghostty-tuned)
├── herdr/                 # POSIX/Windows configs (Rose Pine; Windows uses pwsh)
├── pi/                    # audited Pi Rose Pine theme
├── windows-terminal/      # settings.fragment.jsonc + merge README
├── home/                  # chezmoi source tree for the config layer
├── tests/                 # automated test tree
├── tests/greenfield/      # local clean-machine harnesses and validators
├── tests/wsl/             # manual WSL split-host e2e check
├── .github/workflows/     # CI matrix + chezmoi parity
├── .github/rulesets/      # checked-in GitHub ruleset payloads for main
├── docs/security/         # branch-protection runbook
├── setup.sh               # public macOS/Linux/WSL entry point
├── setup.ps1              # public Windows entry point
├── test.ps1               # Windows test entry point
├── Makefile               # Unix test/setup conveniences
├── AGENTS.md              # standard agent entry point, points to CLAUDE.md
├── CLAUDE.md              # canonical coding-agent operational guide
├── .editorconfig          # cross-IDE formatting rules
└── README.md              # human-facing install and usage guide
```

## Key design decisions (and why)

- **One source of truth through chezmoi-managed configs.** POSIX uses symlinks
  for live-edit behavior, Windows copies simple files, and nvim remains a
  directory symlink into repo `nvim/`.
- **chezmoi is the config-layer path, not the provisioning path.** Do not move
  package installs, binary/font installers, login-shell changes, VS Code,
  devilspie2, or distro-manager policy out of `install-deps`.
- **Sentinel is global agent policy, not a dotfile mirror.** Setup installs it
  from a pinned, version-checked upstream checkout and lets Sentinel own its
  managed global entrypoint blocks. This repo does not vendor Sentinel core or
  sync agent runtime state.
- **Rose Pine everywhere it can render.** Nvim, lualine, foreground-only
  Starship, tmux/psmux, `lsd`, ghostty, Herdr, Pi, Windows Terminal, PSReadLine — same
  palette across the stack. tmux and psmux share ONE repo-owned generated Rose
  Pine bar (`tmux/psmux-rose-pine.ps1` -> `psmux-rose-pine.{main,moon,dawn}.conf`,
  sourced on both), so the bar is byte-identical; TPM (POSIX) and the vendored
  psmux ports (Windows) only add the FUNCTIONAL plugins, not the theme. VS Code
  joins optionally: `install-deps` offers
  VS Code, and if `code` is detected it installs the `mvllow.rose-pine` theme,
  sets `workbench.colorTheme` (plus the `preferredDark`/`preferredLight` slots
  and `window.autoDetectColorScheme:false` so dark Rose Pine is forced
  regardless of OS scheme), and sets VS Code editor/terminal font families to
  Hack Nerd Font fallbacks. Existing JSONC settings are edited in place with
  comments preserved and a backup first.
- **conform.nvim is the only format-on-save handler.** Replacing the
  prior LSP-attach autocmd + null-ls duo eliminates a real race condition
  with different timeouts. Formatter output must still stay inside the LSP's
  parser/schema rules; Tier 2 proves this by formatting realistic samples and
  failing on post-format LSP warnings/errors. Save formatting is synchronous
  and bounded at 10 seconds, matching that strict formatter smoke window so a
  cold Windows Node/Prettier process is not killed by the old 3-second ceiling.
  `:WNF` (or `:wnf`) skips formatting for one save.
- **Mason installs LSP servers + formatters via mason-tool-installer.** No
  `mason-lspconfig` — redundant on nvim 0.11 with `vim.lsp.enable`.
- **DAP launches stay generic.** The shared browser launch defaults to
  `http://localhost:3000`; set `DAP_LAUNCH_URL` or put project-specific launch
  configs in workspace `.nvim.lua` files.
- **Starship language modules pared down.** Only `c, go, node, rust, python,
  conda` are enabled. Disabled languages don't spawn version probes on every
  prompt. Prompt segments use foreground styles only so transparent terminals do
  not show opaque character-width blocks behind rendered text. The prompt keeps
  the username visible at the start; tmux/psmux does not duplicate it in the
  status bar.
- **Zsh starship init is precompiled** (mirroring the PowerShell profile
  approach) — re-generated only when `starship.toml` is newer than the cache.
- **zsh plugin installs are repo-managed pins.** `fzf-tab` and
  `zsh-autosuggestions` are installed during Unix setup, sourced from XDG data
  before package-manager fallbacks, and verified against tag commit IDs so a
  Renovate ref bump still gets human review.
- **fzf wired into zsh by default** — `Ctrl-R` fuzzy history, `Ctrl-T` file
  picker, `Alt-C` fuzzy cd. It *complements* zsh-autosuggestions (inline
  ghosting), it doesn't replace it. Guarded by `command -v fzf` so a machine
  without it still starts cleanly; uses `fzf --zsh` with a share-dir fallback
  for older distro builds.
- **lsd wired and themed in interactive shells** — setup installs it where the
  platform package manager carries it, chezmoi deploys the custom Rose Pine
  theme, and zsh/PowerShell expose the documented `ls`, `l`, `la`, `lla`, and
  `lt` commands when `lsd` is present. The profiles own the Rose Pine
  `LS_COLORS` palette for file/directory names, including special directory
  classes such as sticky and other-writable directories; `colors.yaml` owns
  long-list metadata. Set `DOTFILES_LS_COLORS` before shell startup for an
  explicit palette override.
- **No `bindkey '\e' kill-whole-line`** — that shadowed the entire Meta
  prefix and silently broke Alt-h/j/k/l window nav in nvim.
- **tmux window swaps use uppercase Vim directions.** `prefix+H` swaps the
  current window left and `prefix+L` swaps it right. Lowercase `h/l` remain pane
  focus bindings, and arrow keys are left alone for terminal/psmux reliability.
- **tmux/psmux share one Omer-shaped Rose Pine bar.** Both runtimes source the
  SAME generated `tmux/psmux-rose-pine.{main,moon,dawn}.conf` (built by
  `tmux/psmux-rose-pine.ps1`), so the pill bar — rounded session pill left,
  number-on-right window cells with a zoom marker on the current window, and a
  directory pill right — is byte-identical. No theme plugin renders it:
  `rose-pine/tmux` cannot run on psmux and the community `psmux-theme-rosepine`
  draws a different arrow-chevron powerline bar. The repo default is `main`, with
  `moon` / `dawn` selected by `@rosepine-variant` on both platforms (live:
  `tmux set -g @rosepine-variant moon; tmux source-file ~/.tmux.posix.conf`;
  `psmux set -g @rosepine-variant moon; psmux source-file ~/.tmux.windows.conf`).
  Both bars are top-aligned and show session, window list, and directory
  basename; date/time, full path, username, git, and runtime state stay in
  Starship; host stays off the daily surface. The normal session pill accent is
  Rose Pine `foam`; holding the prefix changes it to `love`. The status canvas
  uses the terminal default background, so terminal transparency shows through
  between and around the colored pills. Windows psmux starts from
  `~/.psmux.conf`, which turns warm sessions off before sourcing `~/.tmux.conf`,
  then explicitly source-files `~/.tmux.windows.conf` without `-q` because psmux
  v3.3.x does not implement tmux's `source-file -q` config flag. psmux-parsed
  configs also avoid `set ... terminal-features`; tmux-only extended-key flags
  live in `~/.tmux.posix.conf`. The generated Rose Pine artifacts stay inside
  the shared tmux/psmux option set; tmux-only display-pane color options are
  omitted because psmux warns on unknown options. The generated
  status-right and Starship time segment keep one trailing safety space so the
  last visible glyph is not drawn into the terminal's final column.
- **Session save/restore: full on POSIX, resurrect-only on Windows.** POSIX runs
  the pinned `tmux-resurrect` + `tmux-continuum` (TPM): auto-save every 15 min,
  auto-restore on start (`@continuum-restore on`), nvim session restored
  (`@resurrect-strategy-nvim session`). Windows psmux vendors ONLY the pinned
  `psmux-resurrect` port (manual `Prefix+Ctrl-s` / `Prefix+Ctrl-r`);
  `psmux-continuum` is intentionally blocked pending real Windows psmux
  verification of its load-time async `run-shell` hooks. PPM is deliberately
  unused (it clones monorepo HEAD unpinned and rewrites managed config); the port
  is source-filed from a pinned checkout instead.
- **WSL is split-host by default.** Windows Terminal renders fonts and window UI
  on the Windows side; WSL installs the Linux CLI/editor stack. Linux Ghostty
  and Linux fontconfig fonts in WSL require `--experimental-wsl-gui`.
- **Windows Terminal is a Windows dependency, not just a config target.**
  `install-deps.ps1` installs `wt` through the normal Scoop-first catalog:
  `extras/windows-terminal` -> `Microsoft.WindowsTerminal` -> `microsoft-windows-terminal`.
  If those MSIX-backed installers do not make `wt` available, it falls back to
  the pinned portable Windows Terminal GitHub release zip, verifies SHA-256
  before extraction, and adds the portable folder to the current and User PATH.
  setup's config phase then transactionally merges the repo-owned visual/keybinding fragment
  by default; pass `-SkipWindowsTerminalMerge` to opt out.
  `-MergeWindowsTerminal` remains accepted as a no-op alias for older commands.
- **Windows Terminal settings.json is NOT symlinked** because WT auto-rewrites
  it. Only the user-owned keys live in `settings.fragment.jsonc`; setup reads
  each stable packaged, Preview, Canary, and portable target independently,
  stages and validates every
  result, makes a verified per-target backup, then atomically publishes. It
  updates keys by identity, adds a fixed `PowerShell 7` profile (`pwsh.exe`), promotes an empty or
  Windows PowerShell 5.1 `defaultProfile` to that profile, and resets a
  hand-edited `theme` back to `rose-pine` on every run unless you opt out. A
  custom `defaultProfile` is preserved. The portable target is never mirrored
  from MSIX; its own profiles/actions/schemes survive. Concurrency or any
  backup/parse/stage/publication failure is fatal and rollback-safe. A bare
  `chezmoi apply` has no WT target because it cannot satisfy that transaction.
- **Windows CI installs Scoop through a pinned, verified elevated path.** GitHub
  Windows runners are elevated, and Scoop blocks elevated bootstrap by default.
  `install-deps.ps1` downloads `ScoopInstaller/Install` at a pinned commit,
  verifies the installer SHA-256, runs it with `-RunAsAdmin` only when elevated,
  preserves the caller's process execution policy, then refreshes the
  current-process Scoop shims path so later installs can use `scoop` immediately.
  Existing Scoop installs also get the `extras` and
  `nerd-fonts` buckets normalized before catalog installs.
  Chezmoi's Windows nvim directory symlink still uses the elevated/native
  CreateSymbolicLink path. For local machines, Developer Mode plus a normal
  PowerShell remains the recommended setup path.

See `AGENTS.md` for the standard coding-agent entry point. `CLAUDE.md` remains
the canonical operational guide because Claude Code auto-loads it; `AGENTS.md`
points there so other agents reach the same instructions without duplicating
content.

## Maintainer workflows

Daily shortcuts live in the [cheat sheets](#cheat-sheets). The workflows below
are for changing or reproducing the managed configuration itself.

### Adding a plugin

Drop one file at `nvim/lua/plugins/<name>.lua` returning the lazy spec.
Lazy auto-discovers it. Default to lazy-loading (`event` / `cmd` / `keys` /
`ft`). Only `rose-pine` may set `lazy = false`.

### Adding an LSP server / formatter / treesitter parser

See `CLAUDE.md` -> "Common workflows". LSP servers and formatters update the
plugin spec, Mason `ensure_installed`, and the matching spec. Treesitter parsers
use `nvim-treesitter` main's `treesitter_parsers` list plus a filetype alias
when Neovim's filetype differs from the parser name, and every parser-backed
language gets a fixture row in `tests/nvim/language_matrix.lua`. The current
matrix covers daily application languages, shells, web formats, .NET project
files/Razor, data/config/build files, TeX/BibTeX/Typst/Mermaid, shader formats,
and common platform/scientific languages. Built-in regex syntax fallback is
automatic for any detected filetype with a Neovim `syntax/<filetype>.vim`
runtime file; do not add per-language syntax fallback allowlists or fake parser
names for formats nvim-treesitter does not support. If a syntax-only fallback is
important enough to add to the matrix, add a Tier 2 syntax probe too.

### Refreshing the plugin lockfile

```bash
nvim --headless "+Lazy! sync" +qa
git add nvim/lazy-lock.json   # tracked, not gitignored
```

Setup and validation use `Lazy! restore` instead. Run `Lazy! sync` only when
you intentionally want to refresh plugin pins and review the resulting lockfile
diff. The startup-budget spec is intentionally different: it preclones locked
plugin checkouts without running Lazy build hooks so the checked, waitable
parser compilation does not pollute the startup timing assertion.

### Updating Mason tools across machines

```bash
nvim --headless "+lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
```

Run on each machine; there's no machine-pinned lockfile for Mason itself. Linux
gets `clangd` from the architecture-independent Home Manager package layer;
Mason retains `clangd` ownership on macOS and Windows, where its registry has a
matching artifact.

### Reproducing the same release on another machine

```bash
git clone --branch v0.2.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles
cd ~/dotfiles
./setup.sh --all                 # prerequisites + packages + config + locked plugins
make test                       # verify the new state
```

Use `docs/UPGRADING.md` when the release version changes. A moving-branch pull
is not a release migration.

## License

MIT. See `LICENSE`.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Setup warns `multiple managed <tool> commands are on PATH` | two physically distinct installations publish the same managed command; changing `PATH` order could silently switch which one runs | keep the printed `selected` command, then remove each `duplicate` through its proven owner. Setup prints exact no-elevation cleanup only for a proven user-scoped Homebrew, npm, or Scoop package; review scope yourself for winget/Chocolatey/system managers, and use the original manager for `owner=unknown`. Setup never removes either copy automatically. Rerun setup until the warning disappears |
| Pi setup says `expected 0.80.9 after install, got 0.80.3`, or reports multiple managed `pi` commands | an older global npm/Homebrew `pi` duplicates the repo-owned `~/.local/bin/pi`; older checkouts also let the global command win `PATH` resolution | update this repo and rerun setup. Current setup proves only `~/.local/bin/pi`, makes it win in current and future shells, and reports every physical duplicate. For a proven npm-global copy, run the exact `cleanup (same user, no sudo)` command shown, then rerun setup to confirm one command remains. Never prepend `sudo`; unknown owners must be removed with their original package manager |
| v0.2.0 Linux Nix bootstrap ends with `cat: /etc/bashrc: Permission denied` and upstream's `Oh no` failure | the pinned upstream daemon installer prepared `/etc/bashrc` through its privileged path, then tried to read it as the invoking user; its multi-user path does not honor the public `--no-modify-profile` option | if `/etc/bashrc.backup-before-nix` exists after the failure, restore it with `sudo mv /etc/bashrc.backup-before-nix /etc/bashrc`. For immutable v0.2.0 without that backup, the bounded workaround is `sudo chmod a+r /etc/bashrc` before rerunning. On the current official test branch, pull the latest head and rerun `./setup.sh --all --allow-unreleased`; the wrapper applies an exact-hash-verified local patch that skips the daemon profile step and leaves shell activation to setup/Home Manager |
| Linux Nix bootstrap reports `getting status of '/nix/store/...-busybox...': Permission denied` while installing Nix | a restrictive invoking umask—common on managed corporate hosts—combined with Nix 2.34.0's Linux daemon copy and write-bit removal left store directories as root-only `0500`; a previous interrupted attempt can retain those modes | pull the latest current official test-branch head and rerun `./setup.sh --all --allow-unreleased` as the normal target user. Its checksum-bound daemon installer normalizes the copied and pre-existing store paths to read-only/traversable modes before Nix creates the default profile; do not run all of setup with `sudo` |
| Corporate Linux bootstrap warns that `https://channels.nixos.org/nixpkgs-unstable` failed TLS verification | upstream's legacy default-channel step uses its bundled CA file, which does not contain the corporate TLS-inspection root; the dotfiles package layer uses locked flakes and does not need this mutable channel | pull the latest current official test-branch head and rerun `./setup.sh --all --allow-unreleased`. The wrapper disables channel creation with upstream's public `--no-channel-add`; subsequent Nix activation selects the host system CA bundle, preserving corporate trust without weakening TLS verification |
| Neovim stops before loading Lazy with a lockfile/cache identity error | `lazy-lock.json` is missing, malformed, incomplete, has a non-40-hex commit or invalid branch; or the cached `lazy.nvim` checkout is dirty, at the wrong commit, from the wrong origin, missing locked default-branch metadata, non-Git, or partial | restore the tracked `nvim/lazy-lock.json` and restart Neovim. Startup repairs the cache through a verified staging checkout and never executes an unproved path. If publication fails, fix the destination permissions named in the error and retry |
| setup reports a Homebrew `shellenv` failure even though `brew` already resolves | the selected command and PATH-resolved command report different Homebrew prefixes/repositories, or `shellenv` exited nonzero; empty stdout alone is a normal Homebrew idempotence signal | compare `brew --prefix` and `brew --repository` through both entrypoints named in the error. Repair the shadowing PATH or Homebrew installation, then rerun setup; a nix-darwin wrapper and native brew path are accepted only when those identities match |
| a new zsh prints `compinit: no such file or directory: .../_brew` | Homebrew's core completion symlink survived a repository/Nix-generation migration but its target did not; `brew completions link` alone only reconciles tap completions | update this repo and rerun setup or `./setup.sh --update`; both paths reconcile tap completions, atomically repair a missing/dangling core `_brew` symlink to the active Homebrew implementation, and verify the resolved target. A conflicting non-symlink is preserved and reported instead of overwritten |
| first nix-darwin setup reports an existing `.before-nix-darwin` backup | setup found both an unmanaged `/etc/bashrc` or `/etc/zshrc` and an older backup, so choosing either would risk user/system data | compare the two files and resolve the collision deliberately, then rerun. Setup moves neither shell file until both backup destinations are clear and restores both if activation fails |
| setup says it is bootstrapping nix-darwin again immediately after a successful activation | the checkout predates the retry fix, so the still-open terminal cannot see the new system profile on `PATH` and setup mistakes the retry for first bootstrap | update the checkout and rerun setup. Current setup resolves `/run/current-system/sw/bin/darwin-rebuild` directly and accepts the managed `/etc/static` shell links without touching their recovery backups |
| `<leader>X` keymaps fire `\X` instead of `<Space>X` | mapleader set after lazy.setup somehow | restore the order in `nvim/init.lua` — leader **before** `require("lazy").setup` |
| Herdr `Ctrl+B`, then `Shift+1..9` does nothing | Herdr `v0.7.3` did not map shifted digits to the punctuation keycodes terminals report | update this repo and rerun setup/update. Native Linux pins `v0.7.4`, current Homebrew carries `v0.7.4`, and Windows uses the pinned post-fix preview |
| Formatter runs twice or shows two BufWritePre autocmds | someone added a second handler outside conform.nvim | `:lua print(#vim.api.nvim_get_autocmds({event="BufWritePre"}))` should be 1; if not, find the second autocmd and delete it |
| Lazy/Tree-sitter/Mason says `No C compiler found` | WSL/Linux has `make` but no `cc`/`gcc`/`clang`; Tree-sitter parsers and some plugin builds compile native code | re-run `./setup.sh --skip-config` to install the Linux compiler toolchain, or on Ubuntu run `sudo apt-get update && sudo apt-get install -y build-essential`, then `./setup.sh --skip-deps --skip-config` |
| Tree-sitter parser install reports temp-dir rename errors such as `ENOTEMPTY`, or a cold setup reports a parser with no captures | a previous/parallel parser build left partial grammar/query output, or an older command-form Lazy `:TSUpdate` returned before its compiler tasks finished | update this repo and rerun setup; both the Lazy update hook and explicit bootstrap now serialize and wait for their upstream tasks, incomplete managed output is purged, and Tier 2 fails causally if any declared parser or explicit highlight query is missing |
| nvim treesitter parsers fail to compile on Windows / `cl.exe` not found | `nvim-treesitter` main builds parsers with the Rust `cc` crate, which needs MSVC env vars | run `.\setup.ps1 -All` to install VS Build Tools and let setup import the VS DevShell before parser installation; for ad-hoc `:TSUpdate`, open a "Developer PowerShell for VS" or rerun setup |
| Windows setup says the verified Tree-sitter executable failed staged version validation | an older installer used a sibling stage name with characters after `.exe`, so Windows would not dispatch it as a native executable | update this repo and rerun `.\setup.ps1 -All`; the verified same-parent stage now ends in `.exe`, is version-checked before atomic publication, and cleans on retry |
| nvim syntax looks weak or files look plain text | Tree-sitter is inactive, or the hybrid built-in syntax fallback was not restored after Tree-sitter starts | update this repo, re-run setup, then check `:Inspect` on a token; parser-backed languages should show `treesitter` captures plus `syntax` groups, while `.bat` should show `syntax` groups |
| Clipboard not crossing host on WSL | `win32yank.exe` not on PATH | install win32yank via scoop on Windows side, ensure WSL PATH picks it up |
| Starship prompt missing in the PowerShell window you ran setup in (but it works in psmux / a new window) | that shell loaded `$PROFILE` **before** setup put starship on PATH; the profile skips starship when `Get-Command starship` finds nothing | open a **new** PowerShell window, or run `. $PROFILE` in the current one — newly-installed tools are not on an already-open shell's PATH |
| Starship init warns that `starship.ps1` is being used by another process | old checkout wrote the PowerShell init cache directly while several WT tabs or psmux panes started together | update this repo and reopen PowerShell; the profile now writes a temp file, moves it into place, and retries a short read lock |
| Starship prompt slow | a disabled language got re-enabled | check `starship/starship.toml` — only `c, go, nodejs, rust, python, conda` should be enabled |
| Starship prompt text has opaque blocks behind each segment | a local/custom Starship config reintroduced `bg:` styles | update this repo and re-run setup; the managed `starship.toml` is foreground-only so terminal transparency owns the background |
| Starship shows only the last few folders (or a leading `…/`) | the `[directory]` module was truncating the path | `starship/starship.toml` sets `truncation_length = 0` + `truncate_to_repo = false` for the full path; raise the length or set `truncate_to_repo = true` to shorten again |
| A folder like `Downloads`/`Music`/`Pictures` shows as a blank `~/` | its `[directory.substitutions]` glyph was stripped to a bare space | values are `icon + name` (e.g. `Downloads = "<nerd-font-glyph> Downloads"`) using a codepoint your font has; `tests/starship/directory_test.sh` fails on a whitespace-only value |
| `Alt-h/j/k/l` window nav doesn't work in terminal | something rebinds bare Esc in the shell | `bindkey | grep '^"\^\['` in zsh — should NOT show `kill-whole-line` |
| `Esc` does nothing in lazygit inside psmux | psmux v3.3.x has an upstream bare-Escape forwarding bug | use `Ctrl-G` to close lazygit help/popups inside psmux. The native-Windows lazygit config binds `universal.return` to `<c-g>`, so pressing `?` in lazygit shows the working return/cancel key |
| tmux (or any new terminal) launches **bash on Linux**, not zsh | either the login shell was never changed, or an already-running graphical session kept stale `$SHELL=/bin/bash` after `chsh` | if the login shell is still bash, re-run current `./setup.sh` and accept the zsh adoption prompt. Local accounts get `chsh` plus an interactive bash guard so new terminals/tmux land in zsh without a full graphical relogin; manual `chsh` or older setup runs that already changed `/etc/passwd` still need relogin |
| `chsh` fails with `user '<name>' does not exist in /etc/passwd` | you log in via a **domain** account (AD/LDAP/SSSD) that isn't in local `/etc/passwd`, so `chsh` can't touch it | re-run `./setup.sh` — it detects this and offers to re-exec interactive bash into zsh via `~/.bashrc` instead. The "proper" fix is admin-side: set the directory `loginShell` / SSSD `default_shell` |
| Move commits in lazygit, including inside psmux | Ctrl+J collides with Enter on the wire, and psmux v3.3.4 does not relay Windows Terminal's Win32-input-mode modifier data into panes | use uppercase `J` / `K`. `%LOCALAPPDATA%\lazygit\config.yml` binds commits-panel moveDownCommit / moveUpCommit to printable J/K, so no psmux root bind is needed. In the commits panel, use PgUp/PgDn or Ctrl-U/Ctrl-D to scroll the diff |
| Windows Terminal opens Windows PowerShell 5.1 instead of PowerShell 7 | settings predate the managed WT default-profile merge, or the merge was skipped | re-run `.\setup.ps1 -SkipDeps -SkipNvim`; it adds the fixed `PowerShell 7` profile and promotes only an unset or legacy Windows PowerShell default, preserving a custom default |
| tmux / psmux does not show the Rose Pine status bar | The generated variant config was not deployed or sourced, or it loaded in an already-running server | re-run setup / re-apply chezmoi, then restart all tmux/psmux sessions. The bar is the generated `~/.tmux.rose-pine.{main,moon,dawn}.conf`, sourced by `tmux/tmux.posix.conf` (POSIX) and `tmux/tmux.windows.conf` (Windows). Windows psmux starts from `~/.psmux.conf`, then flag-free source-files `~/.tmux.windows.conf`. Change the `@rosepine-variant` (`main` / `moon` / `dawn`) option for a different flavor |
| tmux says `Tmux resurrect file not found!` on first launch | Continuum tried to restore before the first snapshot existed | save once with `Ctrl+B`, then `Ctrl+S`, or wait 15 minutes for the first auto-save |
| psmux shows a `run-shell` `Saved to ...` popup but does not restore after restart | the popup is a completed save receipt; Windows restore is manual | press `q` or `Esc`, restart psmux, then press `Ctrl+B`, then `Ctrl+R`; use `Ctrl+B`, then `w` to select the restored session |
| psmux restore says a session `already exists, skipping` | psmux-resurrect will not overwrite a running same-named session | start a temporary session with `psmux new-session -s recovery`, restore there, then use `Ctrl+B`, then `w` to select the restored session |
| psmux warns `unknown option` while sourcing config | A psmux-parsed config still contains a tmux-only option | update this repo and re-run `.\setup.ps1 -SkipDeps -SkipNvim`, then restart psmux. The managed shared and Windows configs intentionally avoid `set ... terminal-features`, and the generated Rose Pine artifacts omit tmux-only `display-panes-*` color options; tmux extended-key flags live only in the POSIX overlay |
| tmux / psmux status bar looks fully opaque | The generated status canvas is not using the terminal default background, or an older generated artifact is still loaded | update this repo, re-run setup / chezmoi apply, then restart tmux/psmux. The managed bar sets `status-style` and pill outside caps to `bg=default`; only the pill interiors have explicit Rose Pine backgrounds |
| PowerShell Tab completion — the selected option is **gold** | PSReadLine `Selection` colors the highlighted MenuComplete option | it is a gold foreground. Note: PSReadLine uses that same `Selection` color for the completion suffix it inserts into the command line while you navigate, so that suffix also shows gold until you accept — it is one setting, not separable |
| A `wt --version` window popped up during `setup.ps1 -All` | the dependency version table ran `<tool> --version`, and `wt --version` opens a Windows Terminal window instead of printing | fixed — `Get-CommandVersionString` never runs `wt --version`; it reads the file version (or shows `installed`) |
| `setup.ps1` or a managed helper says it cannot be loaded because it is not digitally signed | the current PowerShell execution policy rejects the checkout before setup can run or remove Mark-of-the-Web from managed files | in that same PowerShell window run `Set-ExecutionPolicy -Scope Process Bypass -Force`, then run `.\setup.ps1 -All`; the bypass ends when that PowerShell process closes and does not change the persistent user or machine policy |
| Ghostty doesn't open maximized | `window-save-state = always` restored an old geometry over `maximize` (macOS only) | `ghostty/config` uses `window-save-state = default` (not `always`) with `maximize = true`; `always` lets the saved size win |
| Ghostty doesn't load the config | wrong path, or WSL default skip | the install path is `~/Library/Application Support/com.mitchellh.ghostty/config` on macOS and `~/.config/ghostty/config` on native Linux. WSL only links Linux Ghostty config after `./setup.sh --experimental-wsl-gui`; otherwise use Windows Terminal |
| Windows Terminal lost a profile after merge | WT rewrote one installation's file after setup, or an older pre-transactional setup was used | inspect that installation's independent `<settings.json>.bak.<YYYYMMDD-HHMMSS>[.n]` backups; `uninstall.ps1` validates every stable packaged, Preview, Canary, and portable candidate before mutation, restores each target independently, and preserves the displaced current file for recovery |
| `setup.ps1` errors "cannot create symbolic links" | Developer Mode off and not elevated | `setup.ps1` reports your *elevated* + *Developer Mode* state before chezmoi apply. Enable Developer Mode (Settings -> Privacy & security -> For developers, no admin, recommended) **then** `.\setup.ps1 -SkipDeps`; OR run just the config phase elevated with `.\setup.ps1 -SkipDeps -SkipNvim`, then return to a normal shell for `.\setup.ps1 -SkipDeps -SkipConfig`. Don't elevate the dependency-install run because Scoop refuses admin installs |
| Ghostty won't open maximized on Linux/GNOME | `maximize = true` is a hint the WM may ignore (GNOME Mutter often does) | on **X11**, `install-deps` offers a devilspie2 setup through the native Linux package manager, even when Linuxbrew is the main CLI manager; the rule is keyed on `com.mitchellh.ghostty`. Wayland needs a GNOME Shell extension instead |
| `install-deps.ps1`: winget `No package found matching input criteria` (exit `-1978335212`) | winget source/catalog flakiness | install-deps now **prefers scoop** and falls back across managers per tool -- accept the scoop bootstrap when offered and re-run; VS Build Tools has no Scoop package, so it falls through to choco and then Microsoft's official bootstrapper |
| `setup.sh --update` says a tool is `system` | the executable is an explicitly accepted OS-vendor provider, such as macOS `/bin/zsh` | no action needed unless you intentionally want to adopt a package-manager version and own that migration separately |
| `setup.sh --update` says a tool is `unmanaged` | the executable is present, but no supported Unix owner proves ownership of the resolved command source | use it as-is and update it outside dotfiles, or intentionally migrate it to Homebrew/Linuxbrew, the native package manager, or a dotfiles-provenanced direct artifact |
| `setup.sh --update` says a tool is `blocked` | the source strongly implies supported ownership, but the package/provenance proof is contradictory or unsafe | repair or reinstall that manager package/artifact, then rerun update mode; dotfiles fails closed rather than guessing |
| `setup.sh --update` still resolves `make` to `/usr/bin/make` after `brew install make` | Homebrew's GNU Make formula exposes `make` through `$(brew --prefix make)/libexec/gnubin` | rerun `./setup.sh --skip-config` or open a new shell after setup persists Homebrew shellenv; the managed shell block prepends the `gnubin` path when the formula is installed |
| A fresh Linux/WSL zsh cannot find Home Manager tools | Home Manager's session variables were not generated or none of its canonical profile paths is readable | re-run `./setup.sh --all`, then start a new zsh. The managed zshrc checks `${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles/profile`, `~/.nix-profile`, then `/etc/profiles/per-user/$(id -un)` for `etc/profile.d/hm-session-vars.sh`; no-Nix machines remain supported |
| A CI/credential-helper PowerShell process prints prompt output or writes profile caches | an old profile used host name or `UserInteractive` as a proxy for an interactive invocation | update and re-apply this repo. The profile now returns before cache work when argv selects batch/noninteractive execution, any console stream is redirected, CI is set, or the host is unsupported; normal ConsoleHost, VS Code, and ISE remain interactive |
| Mixed Linuxbrew and apt/dnf/pacman/zypper/apk tools update through different managers | update mode resolves ownership per executable source, not from one global active manager | this is expected: a Linuxbrew-owned `rg` can update through Brew while an apt-owned `/usr/bin/jq` updates through apt in the same run |
| `setup.ps1 -Update` says a tool is `unmanaged` | the executable is present, but its command source is outside supported manager ownership | install or migrate that tool through Scoop, winget, or Chocolatey if you want dotfiles to own future updates; otherwise update that manually-installed copy outside dotfiles |
| `setup.ps1 -Update` says a Scoop-owned tool is `blocked` | the command resolves through Scoop shims, but the shim metadata cannot prove the exact catalog package | repair or reinstall that Scoop package, then rerun update mode; dotfiles intentionally fails closed instead of updating a different manager's package |
