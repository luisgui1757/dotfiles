# Dotfiles

Choose the tools you want. Setup installs their prerequisites, applies the shared
Rose Pine configuration, and records what it owns so update and removal can make
precise changes.

**Major-release candidate:** [current delivery status](docs/installer-status.md)
records implementation and verification. Delivery requires independent review
and all native/quality checks at the exact PR head.

## Supported platforms

- Apple Silicon macOS. Intel Macs and Rosetta execution are unsupported.
- Ubuntu and Debian on amd64 or arm64. Native acceptance targets Ubuntu 26.04
  and Debian 13; older distribution releases are not certified by this matrix.
- Native Windows amd64.

WSL uses the ordinary Ubuntu/Debian choices. There is no separate WSL installer,
Windows-host provisioning, or Linux distribution fallback. Choose graphical apps
only where you have a graphical session. AeroSpace needs a manual macOS
Accessibility grant on first launch.

## Install, update, and remove

Clone this repository to a location you intend to keep: managed configuration
points back to the checkout. Run setup as your normal user. It requests native
administrator access only for the changes you approve.

```bash
git clone https://github.com/luisgui1757/dotfiles.git ~/dotfiles
cd ~/dotfiles
./setup.sh
```

On Windows, use PowerShell from the checkout:

```powershell
git clone https://github.com/luisgui1757/dotfiles.git $HOME\dotfiles
Set-Location $HOME\dotfiles
.\setup.ps1
```

If Windows blocks the unsigned checkout script, use
`Set-ExecutionPolicy -Scope Process Bypass` in that window, then run it again.
The process-only setting does not change your permanent policy. Developer Mode
is unnecessary: Windows configuration uses copies and a Neovim directory junction.
Windows PowerShell 5.1 can launch setup before PowerShell 7 is installed.

No install flags are needed. The menu offers:

- **Choose tools:** Space toggles a category or an individual tool; Enter continues to the preview.
- **Remove tools:** select installed tools to remove and review shared dependencies.
- **Update selected tools:** reconcile your selection with the checkout's pins.
- **Repair selected tools:** repair verified managed resources after a health check.
- **Check installation:** inspect the installation without changing it.

Review the dependency and removal list before applying. Escape cancels. Open a
new terminal after changing shell integration so it receives the new PATH.

To move to a newer repository revision, first preserve your local changes, then
update your checkout (for a branch, `git pull --ff-only`) and open setup again.
The update menu uses that checkout's reviewed pins; it does not update Git or
fetch arbitrary latest package versions. Native providers update only the
selected, proven-owned packages, never the entire machine.

Coming from the Nix/chezmoi release? Run `./migrate.sh` or `.\migrate.ps1` after
updating the checkout. It detects the earlier installation, preserves its
configuration, opens the replacement tool menu, and checks the new setup before
previewing removal of the verified old runtimes and tools. On macOS it first
converts Nix-managed Homebrew while keeping its packages. Read
[the migration guide](docs/UPGRADING.md) for conflicts and interrupted work.

## Selectable tools

Only top-level tools appear in the installation checkboxes. Compiler/SDK tools,
fonts, runtimes, libraries, plugins and themes follow the selections that need
them; they are not independent product choices.

Tools are grouped under category checkboxes, with individual tools indented below.
Space on a category selects all its available tools; Space on a fully selected
category clears them. `[-]` means some children are selected; pressing Space
selects the rest. Children remain independently selectable. `a` selects all,
`n` clears all, and `j`/`k` or arrow keys move through the list. Remove tools uses
the same groups, containing only your recorded selections.

| Category | Tools (where supported) |
|---|---|
| Editors | Neovim, VS Code |
| Shell essentials | zsh on macOS/Linux or PowerShell 7 on Windows; Starship, lsd, zoxide, fzf, ripgrep, fd |
| Version control | Git, lazygit, GitHub CLI, gh-dash |
| Terminal apps | Ghostty, Windows Terminal |
| Terminal sessions | tmux, Herdr |
| AI coding | T3 Code, Claude Code CLI, Codex CLI, Sentinel agent policy |
| Desktop apps | OpenWhispr, Obsidian, Tailscale, Zen Browser, AeroSpace |
| Dotfiles development | Lint and benchmark tools |

Only tools supported on your platform appear. Categories select tools; their
required dependencies are still resolved automatically in the preview.

| Checkbox | macOS | Ubuntu/Debian | Windows |
|---|---|---|---|
| Neovim | ✓ | ✓ | ✓ |
| VS Code | ✓ | ✓ | ✓ |
| OpenWhispr | ✓ | amd64 | — |
| Obsidian | ✓ | Flatpak | — |
| Tailscale | CLI | ✓ | — |
| Zen Browser | ✓ | ✓ | — |
| zsh | ✓ | ✓ | — |
| PowerShell 7 | — | — | ✓ |
| Starship | ✓ | ✓ | ✓ |
| lsd | ✓ | ✓ | ✓ |
| zoxide | ✓ | ✓ | ✓ |
| fzf | ✓ | ✓ | ✓ |
| ripgrep | ✓ | ✓ | ✓ |
| fd | ✓ | ✓ | ✓ |
| tmux | ✓ | ✓ | — |
| Ghostty | ✓ | ✓ | — |
| Windows Terminal | — | — | ✓ |
| AeroSpace | ✓ | — | — |
| Herdr | ✓ | ✓ | ✓ |
| Git | ✓ | ✓ | ✓ |
| lazygit | ✓ | ✓ | ✓ |
| GitHub CLI | ✓ | ✓ | ✓ |
| gh-dash | ✓ | ✓ | ✓ |
| T3 Code (browser UI) | ✓ | ✓ | ✓ |
| Claude Code CLI | ✓ | ✓ | ✓ |
| Codex CLI | ✓ | ✓ | ✓ |
| Sentinel policy | ✓ | ✓ | ✓ |
| Lint and benchmark tools | ✓ | ✓ | ✓ |

The table defines the product scope. Completion evidence for each integration is
tracked separately in [installer status](docs/installer-status.md).

**Git** installs or reuses Git and supplies `pull.rebase = true` and
`fetch.prune = true`: pulls rebase local commits, and fetch removes stale
remote-tracking references. A marked include at the start of your XDG Git config
loads the checkout's defaults. The personal file stays writable, so ordinary
`git config --global` commands cannot write through into the checkout. Existing
personal settings in that file or `~/.gitconfig` take precedence. Setup does not
set your name/email or authenticate you. Put additional personal includes in
their own section outside the marked block, or in `~/.gitconfig`;
`git config --global --add include.path` can edit the managed include section
and requires review. Removal preserves edited blocks and personal settings.
As with other live configuration, editing the checkout's defaults takes effect
immediately. See [Git configuration handover](docs/UPGRADING.md#git-configuration-handover)
for a previous whole-file installation.

**Lint and benchmark tools** is optional for contributors to this dotfiles
repository. It supplies ShellCheck, jq, Hyperfine, Taplo, yamllint and
EditorConfig Checker (plus required runtimes); it does not run the checks or
install a complete development toolchain automatically.

The additional desktop apps use reviewed upstream downloads. Windows is omitted
for these four choices because their vendor installers are not supported by the
current Windows provider. Obsidian uses Flathub in a private per-user installation.
Flatpak itself is provisioned automatically when absent. Removing the app retains
its runtime infrastructure, vaults and `~/.var/app/md.obsidian.Obsidian` data;
ordinary user and system Flatpak installations are untouched. Update follows
Flathub stable, rather than a checkout-pinned Obsidian version.
OpenWhispr uses its official portable archive on Linux amd64; upstream does not
publish a Linux arm64 archive or Flatpak. The vendor launcher automatically
disables Chromium’s sandbox when unprivileged user namespaces are restricted,
including default Ubuntu installations. This behavior is disclosed before approval;
the installer does not change kernel settings or AppArmor policies. On Wayland,
OpenWhispr also reports its optional auto-paste setup (ydotool/ydotoold and input
device access). The installer does not grant access to global input devices;
that native permission setup and microphone/model selection remain user actions.
Their official DEB scripts delete model caches or replace/remove an existing
system AppArmor profile; those packages are not used.

Zen uses Mozilla’s supported legacy profile selection so replacing a managed
application generation keeps the same personal profile. On macOS it uses the
official signed release DMG, as [Homebrew's `zen` cask](https://formulae.brew.sh/cask/zen) does.
Its launcher declares Apple Silicon support and resolves the vendor bundle's
physical path before startup. macOS self-update behavior and default-browser
routing still need native validation; a self-modified bundle
is preserved and requires explicit adoption rather than being overwritten.
Dock, Spotlight or URL routing may launch the inner vendor bundle directly,
bypassing legacy profile selection; those launch paths remain unverified.
Linux disables browser
self-updates through an application policy; use the installer’s Update action.
See [current acceptance evidence and remaining platform gaps](docs/installer-status.md).

Tailscale uses its pinned official DEB on Linux, including the vendor's daemon
service. Run `sudo tailscale up` to sign in. On macOS the existing Homebrew formula
channel installs the CLI and daemon; start it with `sudo brew services start tailscale`,
then run `sudo tailscale up`. The macOS menu-bar app is not installed.
Removing Tailscale uses normal package removal, preserving its state. Existing
installations remain unowned. OpenWhispr's microphone/model setup, Obsidian vaults
and browser profiles remain personal; installation does not import accounts,
create a vault or change the default browser.

On Ubuntu/Debian, VS Code uses the pinned official Debian package through APT
and requires administrator approval when absent. A healthy existing native Code
installation is reused. The package supplies its standard system launcher; no
private launcher or shell profile is needed for Linux Code. Update reconciles an
owned installation to the reviewed checkout pin, including a disclosed downgrade.
A healthy owned installation already recorded at that pin is kept without another
DEB download. Setup preserves existing Microsoft repository settings and keys.
Changed temporary inputs are preserved and reported after successful package
completion; they do not block readiness or removal.

Neovim brings its locked plugins, language servers, formatters, parsers, Python,
Node, Rust, Make, Git and compiler prerequisites. On Windows that includes
PowerShell 7 for its language server, and Visual Studio C++ Build Tools and SDK
when an existing healthy toolchain cannot be reused. macOS installs Apple developer tools when required and absent.
Mason's registry and selected tool versions come from the reviewed checkout.
Managed Neovim does not repair its installed runtime during startup. After a
checkout update changes dependency pins, complete the installer's Update/Repair
before opening Neovim: a Lazy pin mismatch stops configured startup, and a Mason
registry mismatch disables its language-server/formatter setup until repair succeeds.

T3 Code is an independent selection using its official standalone package on
all four supported targets. Run `t3` in a project from a fresh terminal to start
the local server and open the browser UI; keep that terminal running and use
Ctrl-C to stop. No Node installation, desktop app or background service is added.
The pinned stable **v0.1.0** includes Orchestrator V2. Updates follow the stable
release pinned by the reviewed checkout.
[Upstream release](https://github.com/pingdotgg/t3code/releases/tag/v0.1.0).

T3 opens without provider credentials; agent work needs at least one configured,
authenticated provider. For existing CLI logins, install Codex or Claude Code,
authenticate it yourself and enable it in **Settings → Providers**. Both are
independent optional selections. Their current pins meet T3's recommended versions
(Codex 0.159.0+, Claude Code 2.1.280+). T3 also offers **Connect with ChatGPT**, which
can install its own managed Codex runtime; that route does not require selecting
Codex here. The installer does not configure authentication. See
[T3 provider setup](https://github.com/pingdotgg/t3code/blob/81493efb71e8e8244f95d3e9f71d8369654ab151/docs/user/install.md#providers).
T3 owns provider discovery and orchestration. Use the installer's Update/Repair
and Remove menus for the managed package. Stop T3 before changing its installation.
An unchanged pin stays unchanged; a no-op update is not a version upgrade.
Existing T3 installations and `~/.t3` (or your `T3CODE_HOME`), settings, credentials,
conversations and project files remain personal and survive removal. The managed
command takes precedence in configured shells without deleting outside copies.
Before first V2 use with an existing T3 home, stop T3 and back up `~/.t3/userdata`
(or your custom home). T3 copies V1 history once into its V2 database; later
conversations diverge while settings, attachments and workspaces remain shared.
For an isolated trial, run `t3 --base-dir /path/to/separate-t3-home` instead.
See [upstream migration behavior](https://github.com/pingdotgg/t3code/blob/81493efb71e8e8244f95d3e9f71d8369654ab151/docs/user/thread-migration.md).
Use dotfiles Update for its managed Codex/Claude packages too; T3's provider
**Update** and **Update all** can invoke those CLIs' own updaters outside this
pinned lifecycle.

Claude Code and Codex are independent CLI selections; choose either or both.
They use pinned standalone native distributions and automatically resolve Git.
The installer does not install their desktop apps or change personal models,
permissions, credentials or sessions. Sign in from the chosen CLI after setup.
Claude Code's managed shell environment sets `DISABLE_UPDATES=1`; use the
installer's Update menu to reconcile reviewed pins for either CLI.
Sentinel remains a separate policy selection. Notes remain personal: Neovim
enables the notes integration only when its vault exists; `NOTES_VAULT` can select another existing vault.

## Ownership and removal

Portable tools live in private, checksum-pinned generations. APT, Homebrew and
Microsoft installers handle the resources that need native installation.
There is no Nix, Home Manager, nix-darwin or chezmoi runtime. The installer
does not change the account login shell; launch the selected shell explicitly or
change that account preference through your operating system.

Removal follows the dependency graph. A dependency used by another selected tool
stays installed. Pre-existing tools remain unowned. Removing an unused owned
machine-wide dependency needs a separate choice; native outside consumers can
still prevent removal. There is no global autoremove or forced dependency bypass.
Manager/compiler infrastructure explicitly marked retained stays in place and
is disclosed in the preview.

Existing configuration requires explicit adoption. The installer preserves its
baseline and restores it on removal. Edits outside managed settings/blocks remain
untouched; changed owned content needs an explicit decision and is preserved
before replacement. Personal histories, sessions, project files and credentials
are not uninstall targets. Interrupted operations keep a recovery journal and
appear in the menu; do not delete state to force a retry.

Bash integration keeps the login profile chosen during setup. If you later create
a higher-priority `.bash_profile` or `.bash_login`, Bash follows its usual
precedence; the installer leaves that new personal file alone. Updates and removal
continue to manage the original scoped block.

## Cheat sheets

### How to read multiplexer shortcuts

`Ctrl+B`, then `w` means: hold `Ctrl`, press `B`, release both, then press `w`.
It is a sequence, not one four-key chord. This README calls `Ctrl+B` the
**prefix**.

### Herdr

Herdr is the agent-focused multiplexer. It groups terminal panes into tabs and
workspaces, then tracks the agents running inside them. The repo makes its
common navigation feel like tmux and uses Herdr's built-in `rose-pine` theme.
Expanded agent cards preserve the previous two-line layout: state icon plus
workspace/tab first, explicit `idle` / `working` plus agent second, with a blank
line between cards. Direct workspace jumps use a modifier chord that remains
distinct from the literal punctuation bindings terminals send.

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
| `Ctrl+B`, then `Ctrl+Alt+1` ... `Ctrl+Alt+9` | Jump directly to workspace 1 ... 9. On macOS, Alt is Option. |
| `Ctrl+B`, then `Shift+A` / `a` | Move to the previous/next agent. |
| `Ctrl+B`, then `Ctrl+1` ... `Ctrl+9` | Focus agent 1 ... 9 directly. |

Herdr preserves modified Enter for the selected agent CLI. The repo does not
install a raw-LF remap; each CLI keeps its own input bindings.

Named Herdr sessions are separate server namespaces. A session does not appear
inside another session's navigator; attach to the other session from a normal
shell.

Config locations:

- macOS/Linux: `~/.config/herdr/config.toml`
- Windows: `%APPDATA%\herdr\config.toml`

### tmux (macOS/Linux)

Use `tmux` on macOS and Linux. On native Windows, use Herdr. tmux uses
`Ctrl+B` as the prefix and counts windows/panes from 1.

```bash
tmux              # start
tmux attach       # reattach
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
Copy text with vi-style copy mode:

1. Press `Ctrl+B`, then `[`.
2. Move with vi keys.
3. Press `v` to start selecting.
4. Press `y` to copy to the system clipboard.
5. Press `Ctrl+B`, then `]` to paste into the terminal.

The Rose Pine variant is `main` by default. Switch it live with:

```bash
tmux set -g @rosepine-variant moon
tmux source-file ~/.tmux.conf
```

Valid variants are `main`, `moon`, and `dawn`.

Config: `~/.tmux.conf`, plus the generated Rose Pine variants. Windows installs no tmux files.

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

Config: `~/.config/nvim` on macOS/Linux and `%LOCALAPPDATA%\nvim` on
Windows. Both point to this repo's `nvim/` directory. Windows data defaults to
`%LOCALAPPDATA%\nvim-data`. Git limits repository roots separately from files;
an excessively long custom `XDG_DATA_HOME` can stop plugin provisioning with a
path-length error. Setup reports that failure without relocating your data.

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
file. The managed `~/.zprofile` also reads `~/.zprofile.local` last for login
environment settings. Keep that file silent and free of interactive prompts.
Selected managed command directories take priority over inherited system and
personal PATH entries, including Node/Python/Rust prerequisites selected with
Neovim. Put intentional PATH overrides below the managed block in `.zshrc` or
`.bashrc`, or in `~/.zshrc.local` when managed zsh configuration is selected.
Use `~/.zprofile.local` for login-only zsh shells.
Setup adds a marked block to existing profiles. Your text outside that block stays
in place; removal restores the previous managed block. An existing conflicting
configuration is shown for explicit adoption before replacement.

Useful standalone tools:

- `lazygit`: terminal Git UI with upstream default keybindings on every OS.
  Press `?` for keys in the current context; `Esc` closes help and popups.
- `gh-dash`: dashboard for pull requests and issues. Run `gh auth login` first
  to access your GitHub work; installing the dashboard does not require login.
- `t3`: start T3 Code in the current project and open its browser UI.
  `t3 serve` runs without opening a browser; `t3 --help` lists commands.
  Provider authentication is separate. Use dotfiles for package updates/removal.
- `claude`: Claude Code CLI. Start it in a project directory and follow its
  sign-in flow. `claude --help` lists its interactive and scripting commands.
- `codex`: Codex CLI. Start it in a project directory and choose your sign-in
  method. `codex --help` lists its interactive and scripting commands.
  [Official Codex guide](https://learn.chatgpt.com/docs/codex/cli) and
  [Claude Code guide](https://code.claude.com/docs/en/setup) describe their
  authentication and usage. Neither CLI is required by the other.

### Terminals and scrollback

- **tmux panes:** 50,000 history lines per pane. This is separate from the
  outer terminal's scrollback limit.
- **Ghostty:** macOS/Linux, dark Rose Pine, 1 GiB lazy scrollback budget per
  surface, copy-on-select, maximized startup. On macOS, Cmd+grave accent toggles
  the global quick terminal.
- **Windows Terminal:** native Windows, dark Rose Pine, 32,767 history lines
  per profile. That is Windows Terminal's hard maximum.

### Clipboard on macOS, Linux, and Windows

Neovim and tmux copy to the system clipboard. The helper depends on where the
shell is running. Linux guests use the same capability checks as other Linux
sessions; installing a display helper does not create a graphical session:

| Environment | Clipboard path |
|---|---|
| macOS | `pbcopy` |
| Linux Wayland | `wl-copy` from `wl-clipboard` |
| Linux X11 | `xclip`, then `xsel` as fallback |
| Native Windows or an available bridge | `win32yank.exe` |
| Headless tmux | OSC 52 through the terminal |
| Headless Neovim | Explicit OSC 52 or custom provider (`:help clipboard-osc52`) |

The new Neovim selection installs both Linux clipboard helpers automatically.
Retaining those system helpers or macOS’s built-in clipboard commands does not
retain shell profile changes after the last selected consumer is removed.
The active X11 or Wayland session selects the transport; an existing clipboard
bridge or tmux transport may also be used. Headless hosts remain supported without
provisioning a Windows host. Terminal clipboard access still depends on the actual
terminal's OSC 52 support and permissions (`:help clipboard-osc52` in Neovim).

## Development and verification

The contributor gate requires the Go version declared in `installer/go.mod` and
Ruby for the repository-policy checks. The **Dotfiles development → Lint and benchmark tools**
selection supplies the lint and benchmark helpers.
The real C++ project-isolation test also requires `clang++`.

Run the local gate from the checkout:

```bash
make ci
```

```powershell
.\test.ps1
```

The gate checks product configuration, formatting, catalog consistency, bootstrap
boundaries and the Go engine. Opted-in GitHub runners additionally exercise native
install/update/removal, fresh providers, configured applications and interruption
recovery. A local unit pass is not proof of a fresh machine installation.
Do not enable destructive native package fixtures on an everyday workstation.

[CLAUDE.md](CLAUDE.md) is the coding guide. The
[retirement map](docs/installer-test-retirement.md) records how old installer tests
map to the new lifecycle proofs. [Release maintenance](docs/RELEASING.md) defines
publication evidence and review gates. The machine JSON entrypoint exists for
these tests and automation; normal installation uses the menu.

## Repository security

The repository policy uses CodeQL, private vulnerability reporting and immutable releases.
Report vulnerabilities through [SECURITY.md](SECURITY.md); branch protections and
the completed, owner-authorized 23-check cutover are documented in
[branch protection](docs/security/branch-protection.md). Checked-in policy and
successful local checks do not establish current live GitHub settings.

## Repository layout

| Path | Purpose |
|---|---|
| `installer/` | Go CLI, dependency catalog, pinned artifacts, ownership and recovery |
| `setup.sh`, `setup.ps1` | Public interactive entrypoints |
| `migrate.sh`, `migrate.ps1` | Explicit migration from the previous release |
| `scripts/installer-bootstrap.*` | Verified private Go bootstrap |
| `nvim/`, `tmux/`, `herdr/` | Editor and multiplexer configuration |
| `installer/shells/` | Active shell integration recipes |
| `ghostty/`, `windows-terminal/`, `aerospace/` | Desktop configuration |
| `git/`, `lazygit/config.yml`, `gh-dash/`, `starship/`, `lsd/` | Tool configuration |
| `lazygit/config.windows.yml` | Passive released Windows live-link source for migration |
| `home/`, `shells/`, `windows/chezmoi-*` | Passive released sources needed during migration |
| `tests/`, `.github/workflows/` | Local and native verification |
| `docs/` | Operator guidance, delivery status and review evidence |

## License

See [LICENSE](LICENSE). Vendored policy and tool artifacts retain their own
licenses and provenance.
