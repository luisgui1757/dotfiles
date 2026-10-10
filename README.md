# Dotfiles

Choose the tools you want. Setup installs their prerequisites, applies the shared
Rose Pine configuration, and records what it owns so update and removal can make
precise changes.

**Major-release candidate:** implementation and native acceptance are still in
progress. [Current delivery status](docs/installer-status.md) distinguishes
connected functionality from verified runtime behavior.

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

- **Choose tools:** Space toggles checkboxes; Enter continues to the preview.
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
updating the checkout. It previews released shell-profile detachment, preserves
the originals, and then opens the normal selection menu. Read
[the migration guide](docs/UPGRADING.md) for conflicts and interrupted work.

## Selectable tools

Only top-level tools appear in the installation checkboxes. Compiler/SDK tools,
fonts, runtimes, libraries, plugins and themes follow the selections that need
them; they are not independent product choices.

| Checkbox | macOS | Ubuntu/Debian | Windows |
|---|---|---|---|
| Neovim | ✓ | ✓ | ✓ |
| VS Code | ✓ | ✓ | ✓ |
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
| Git defaults | ✓ | ✓ | ✓ |
| lazygit | ✓ | ✓ | ✓ |
| GitHub CLI | ✓ | ✓ | ✓ |
| gh-dash | ✓ | ✓ | ✓ |
| Pi | ✓ | ✓ | ✓ |
| Sentinel policy | ✓ | ✓ | ✓ |
| Repository development checks | ✓ | ✓ | ✓ |

The table defines the product scope. Completion evidence for each integration is
tracked separately in [installer status](docs/installer-status.md).

Neovim brings its locked plugins, language servers, formatters, parsers, Python,
Node, Rust, Make, Git and compiler prerequisites. On Windows that includes the
Visual Studio C++ Build Tools and SDK when an existing healthy toolchain cannot
be reused. macOS installs Apple developer tools when required and absent.

Pi receives the three Fable-tuned Rose Pine variants and multiline-input keys.
Sentinel installs a pinned policy block for supported agent consumers. Neither
selection syncs credentials, sessions, model/provider settings or other local
agent state. Notes remain personal: Neovim enables the notes integration only
when its vault exists; `NOTES_VAULT` can select another existing vault.

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

Inside Pi, `Enter` submits and `Shift+Enter` inserts a line break; `Ctrl+J`
remains the transport-compatible fallback. Herdr preserves modified Enter, so
the repo does not install Ghostty's legacy raw-LF remap, which would erase the
difference between `Shift+Enter` and `Ctrl+J` before Pi receives the key.

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
file. The managed `~/.zprofile` also reads `~/.zprofile.local` last for login
environment settings. Keep that file silent and free of interactive prompts.
Setup adds a marked block to existing profiles. Your text outside that block stays
in place; removal restores the previous managed block. An existing conflicting
configuration is shown for explicit adoption before replacement.

Useful standalone tools:

- `lazygit`: terminal Git UI with upstream default keybindings on every OS.
  Press `?` for keys in the current context; `Esc` closes help and popups.
- `gh-dash`: dashboard for pull requests and issues. Run `gh auth login` first
  to access your GitHub work; installing the dashboard does not require login.
- `pi`: the pinned Pi CLI, with canonical Fable-tuned `rose-pine`,
  `rose-pine-moon`, and `rose-pine-dawn` themes. Main is selected on first
  setup; switch among all three through Pi's `/settings`. Later setup/update
  runs preserve any managed selection. Sessions, credentials, providers, and
  other preferences are not synced.

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
The active X11 or Wayland session selects the transport; an existing clipboard
bridge or tmux transport may also be used. Headless hosts remain supported without
provisioning a Windows host. Terminal clipboard access still depends on the actual
terminal's OSC 52 support and permissions (`:help clipboard-osc52` in Neovim).

## Development and verification

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
the pending required-check cutover are documented in
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
| `git/`, `lazygit/`, `gh-dash/`, `starship/`, `lsd/`, `pi/` | Tool configuration |
| `home/`, `shells/`, `windows/chezmoi-*` | Passive released sources needed during migration |
| `tests/`, `.github/workflows/` | Local and native verification |
| `docs/` | Operator guidance, delivery status and review evidence |

## License

See [LICENSE](LICENSE). Vendored policy and tool artifacts retain their own
licenses and provenance.
