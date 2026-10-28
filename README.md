# Dotfiles

Choose the tools you want. Setup installs their prerequisites, applies the shared
Rose Pine configuration, and records what it owns so update and removal can make
precise changes.

**v1.0.1** replaces the earlier Nix/chezmoi setup with this installer. Coming
from v0.4.x? Follow [the migration guide](docs/UPGRADING.md).

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
points back to the checkout. Run setup as your normal user, without `sudo`. It
requests native administrator access (a `sudo` password prompt on Linux/macOS)
only for the changes you approve. If a legacy Windows profile
is owned by Administrators, preserving that owner may need a one-resource
elevated recovery. Follow [the ownership recovery steps](docs/UPGRADING.md#windows-legacy-profile-ownership),
then return to your normal terminal for remaining work.

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

- **Choose tools:** tick the tools you want. Unticking removes a tool, and
  confirming an unchanged selection repairs failed checks.
- **Update tools:** apply this checkout's reviewed versions and repair failed
  checks; your selection stays the same.
- **Check installation:** inspect the installation without changing it, then return to the menu.

The cursor starts on the likely action: Update after the checkout changed or
when recorded work needs repair, otherwise Choose tools. The tool list labels
installed tools that need action, need repair or have an update available, and
summarizes the pending change (`Add 2, remove 1`). Setup inspects your system
once in the background when the menu opens and reuses that inspection until the
selection changes.

`Esc` (or `q`) goes back: from any screen to the main menu, keeping your tool
choices. `Ctrl+C` quits. After choosing, one confirmation screen shows the
counts, the removals, replacements and administrator changes that fit, and any
optional decisions as checkboxes: keeping retained resources, replacing an
existing configuration (the original is preserved) and removing unused shared
dependencies. `Space` toggles one and the counts update in place. `Enter` on
**Apply changes** applies, **Back to main menu** or `Esc` leaves without changes,
and `d` opens the complete reviewed plan. Keys typed before the confirmation
appears, or while it updates, are ignored. If your system changes after the
review, setup shows the updated plan instead of applying the old one.

When the plan needs administrator access, setup asks for your `sudo` password
once, right after the plan is rechecked, and keeps that credential fresh until
the changes finish, so a long run does not stop for a second prompt.

While changes run, setup prints each numbered step (`[3/12] Installing Neovim`),
downloads with their checksum verification, Neovim's plugin/parser/tool phases
and the live output of APT, Homebrew and other native commands. Each run also
writes that record to `logs/` beside the installer state (for example
`~/.local/state/dotfiles/logs/`); the newest 20 are kept. The final report lists
what needs your action first, then notes such as an application the installer
could not exercise, which are not failures, and the path of the full log. A
failure is shown once, in that report. Open a new terminal after changing shell
integration so it receives the new PATH.

After the first build, setup starts in well under a second while nothing under
`installer/` has changed: it reuses the verified build without unpacking the Go
toolchain. Any change there rebuilds or reverifies as before.

To move to a newer repository revision, first preserve your local changes, then
update your checkout (for a branch, `git pull --ff-only`) and open setup again.
The update menu uses that checkout's reviewed pins; it does not update Git or
fetch arbitrary latest package versions. Native providers update only the
selected, proven-owned packages, never the entire machine.

Coming from the Nix/chezmoi release? Run `./migrate.sh` or `.\migrate.ps1` after
updating the checkout. It detects the earlier installation, preserves its
configuration, opens the replacement tool menu, and checks the new setup before
previewing removal of the verified old runtimes and tools, including the Pi CLI
and the released Sentinel policy file for Pi. On macOS it first
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
`n` clears all, and `j`/`k` or arrow keys move through the list.

| Category | Tools (where supported) |
|---|---|
| Editors | Neovim, VS Code |
| Shell essentials | zsh on macOS/Linux or PowerShell 7 on Windows; zsh from bash terminals; Starship, lsd, zoxide, fzf, ripgrep, fd |
| Version control | Git, lazygit, GitHub CLI, gh-dash |
| Terminal apps | Ghostty, Windows Terminal |
| Terminal sessions | tmux, Herdr |
| AI coding | T3 Code, Claude Code CLI, Codex CLI, GitHub Copilot CLI, Kiro CLI, Sentinel rules and skills |
| Desktop apps | OpenWhispr, Obsidian, Tailscale, Zen Browser, AeroSpace, Mac keyboard shortcuts |
| Dotfiles development | Lint and benchmark tools |

Only tools supported on your platform appear. Categories select tools; their
required dependencies are still resolved automatically in the preview.

| Checkbox | macOS | Ubuntu/Debian | Windows |
|---|---|---|---|
| Neovim | ✓ | ✓ | ✓ |
| VS Code | ✓ | ✓ | ✓ |
| OpenWhispr | ✓ | amd64 | — |
| Obsidian | ✓ | ✓ | — |
| Tailscale | CLI | ✓ | — |
| Zen Browser | ✓ | ✓ | — |
| zsh | ✓ | ✓ | — |
| zsh from bash terminals | ✓ | ✓ | — |
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
| Mac keyboard shortcuts | — | X11 sessions | — |
| Herdr | ✓ | ✓ | ✓ |
| Git | ✓ | ✓ | ✓ |
| lazygit | ✓ | ✓ | ✓ |
| GitHub CLI | ✓ | ✓ | ✓ |
| gh-dash | ✓ | ✓ | ✓ |
| T3 Code (browser UI) | ✓ | ✓ | ✓ |
| Claude Code CLI | ✓ | ✓ | ✓ |
| Codex CLI | ✓ | ✓ | ✓ |
| GitHub Copilot CLI | ✓ | glibc | ✓ |
| Kiro CLI | ✓ | glibc | — |
| Sentinel rules and skills | ✓ | ✓ | ✓ |
| Lint and benchmark tools | ✓ | ✓ | ✓ |

The table defines the product scope.

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
current Windows provider. On Linux, Obsidian uses its official portable build
(both amd64 and arm64), pinned like the macOS app. Its launcher starts it without
Chromium's sandbox, because that sandbox needs unprivileged user namespaces,
which Ubuntu restricts unless an AppArmor profile names the program's path. This
is disclosed before approval; the installer does not change kernel settings or
AppArmor policies. Vaults and Obsidian's settings (`~/.config/obsidian`) stay
personal. Until 2026-10-22, Linux Obsidian was a Flatpak in a private store:
company AppArmor changes could stop its sandbox, and Electron's helper processes
ask Flatpak's session service to start them, which cannot see a private store.
Choose tools removes that earlier Flatpak and keeps its
`~/.var/app/md.obsidian.Obsidian` data; Update installs the portable build but,
as always, removes nothing. Obsidian then asks once for your vault: use **Open
folder as vault**.
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

On macOS, Zen uses [Homebrew's official `zen` cask](https://formulae.brew.sh/cask/zen)
and its normal signed application in `/Applications`. Existing authentic copies
in `/Applications` or `~/Applications` are reused. Browser updates stay enabled;
setup's unchanged Update leaves them alone. Removal requires the separate
machine-wide removal choice and rechecks the developer signature. Personal
profiles, settings and caches remain. Quit all copies of Zen before Repair or
Remove. A missing owned app can be repaired; a changed or unverified app stays
protected. The GUI installation does not claim a shared `zen` shell command.
Zen-only installation also requires Homebrew and Apple's command-line tools.

On Linux, Zen uses Mozilla's legacy profile selection to retain the same personal
profile across managed generations. Its application policy disables self-updates;
use setup's Update action. See the
[handover instructions for earlier Mac installations](docs/UPGRADING.md#zen-macos-native-application).

Tailscale uses its pinned official DEB on Linux, including the vendor's daemon
service. Run `sudo tailscale up` to sign in. On macOS the existing Homebrew formula
channel installs the CLI and daemon; start it with `sudo brew services start tailscale`,
then run `sudo tailscale up`. The macOS menu-bar app is not installed.
Homebrew warns that starting this root service changes ownership of its formula
paths. Stop the service before Remove. If native removal fails on those paths,
preserve the failure evidence, follow Homebrew's exact-path cleanup guidance,
then use the installer's Retry; do not change ownership of the whole prefix.
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

The VS Code selection manages theme, font and update preferences together.
It sets the supported `update.mode` preference to `none`;
use the installer's Update action to change the managed application version.
This application-wide user setting also affects other stable Code installations
using the same settings, and Settings Sync can propagate it to other machines.
Exclude `update.mode` from sync when those machines should update separately.
An existing personal value requires explicit adoption
and is restored on removal; unrelated settings and extension update preferences
remain personal. This prevents Code's background updater from replacing a sealed
installer payload during normal desktop use.

Neovim brings its locked plugins, language servers, formatters, parsers, Python,
Node, Rust, Make, Git and compiler prerequisites. On Windows that includes
PowerShell 7 for its language server, and Visual Studio C++ Build Tools and SDK
when an existing healthy toolchain cannot be reused. macOS installs Apple developer tools when required and absent.
Private Python preparation uses extended paths without changing Windows' system
path policy. Native working-directory and launcher limits still apply to unusually
deep account paths.
Windows may retain an older VC++ runtime registry entry alongside the current
bundle and DLLs. Setup reuses the corroborated current runtime and reports the
older entry; that entry prevents removing the shared runtime.
Mason's registry and selected tool versions come from the reviewed checkout.
Windows gets Gersemi and clang-format from pinned standalone archives at the
same versions, avoiding Mason's Python venv path limit. Their CMake and C/C++
formatting remains available through Neovim and the terminal.
Managed PowerShell puts these commands before personal PATH entries; use an
intentional override below the managed profile block to choose another version.
The optional Python helpers `git-clang-format` and `clang-format-diff` are not
provided on Windows. Gersemi unpacks a temporary runtime per invocation; forced
termination can leave its upstream `_MEI*` temporary directory behind.
Managed Neovim does not repair its installed runtime during startup. After a
checkout update changes dependency pins, complete the installer's **Update tools**
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
T3 owns provider discovery and orchestration. Use the installer's **Update tools**
and **Choose tools** for the managed package. Stop T3 before changing its installation.
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

Claude Code, Codex, GitHub Copilot and Kiro are independent CLI
selections; choose any of them. They use pinned standalone native distributions
and automatically resolve Git.
The installer does not install their desktop apps or change personal models,
permissions, credentials or sessions. Sign in from the chosen CLI after setup.
Claude Code's managed shell environment sets `DISABLE_UPDATES=1`; use the
installer's Update menu to reconcile reviewed pins for either CLI.
**Keeping the agent CLIs current:** Claude Code, Codex, Copilot CLI and Kiro CLI
stay pinned, so Update installs exactly the checkout's versions and changes
nothing until new pins arrive. A weekly workflow checks each vendor's latest
stable release, verifies its bytes and opens a "Bump agent CLI pins" pull request.
Merge it after its native checks pass, then run `git pull` and **Update tools** on
each machine. Opening that pull request needs a GitHub App; see
[agent CLI pin proposals](docs/security/supply-chain.md#agent-cli-pin-proposals).
**GitHub Copilot CLI** uses the pinned standalone build from its official GitHub
release (one native executable; no npm, Node or install script). On Windows it
also installs the shared PowerShell 7 it needs. Copilot ignores its `autoUpdate`
setting, so the managed shell environment sets `COPILOT_AUTO_UPDATE=false` while
it is selected; nothing is written to `~/.copilot`. A `copilot` started without
that environment (for example from an IDE or an unconfigured shell) can still
update itself and rewrite the managed executable; Check then reports the change
and Update or Repair restores the reviewed version. Sign in with `/login` inside
`copilot`, or set `GH_TOKEN` yourself.
**Kiro CLI** (macOS and Linux) uses Kiro's pinned stable download: the
`kiro-cli` command runs from a private copy of the macOS `Kiro CLI.app` bundle
(the desktop app is not launched) or the Linux glibc archive. Kiro CLI otherwise
replaces itself in the background, so setup owns exactly one setting,
`"app.disableAutoupdates": true` in `${KIRO_HOME:-~/.kiro}/settings/cli.json`;
your other Kiro settings stay yours and removal restores the original file. Use
Update for new Kiro releases. Sign in with `kiro-cli login`.
Kiro CLI is not offered on Windows. That is only a limitation of how Kiro ships
it: its Windows build comes solely as an MSI installer, whose normal install is
machine-wide, needs an admin prompt and leaves Kiro's own updater in charge, and
setup does not extract programs from vendor installers. If Kiro publishes a plain
Windows download (for example a ZIP) someday, it can be added like the other
platforms. Install Kiro yourself on Windows if you want it. Sentinel rules and
skills also go to it.

**Sentinel rules and skills** installs
[Sentinel](https://github.com/luisgui1757/sentinel), the shared rules and
skills for coding agents. Setup gets one pinned Sentinel commit with Git and
runs the Sentinel installer of that commit. That installer writes the rules
into a managed block in the instruction file of each agent. Text outside the
block stays. It also copies the `core` and `ui` skills and writes a
`.sentinel-owned` file into each skill folder. It does not replace a folder
that it did not write.

Sentinel goes to each agent that setup finds: Codex, Claude Code, GitHub
Copilot, Kiro and OpenCode. An agent counts when setup finds its command (or
the Kiro app on macOS) or its configuration folder. An agent also counts when
the same selection installs its CLI.

| Agent | Rules | Skills |
|---|---|---|
| Codex | `${CODEX_HOME:-~/.codex}/AGENTS.md` | `~/.agents/skills` |
| Claude Code | `${CLAUDE_CONFIG_DIR:-~/.claude}/CLAUDE.md` | `${CLAUDE_CONFIG_DIR:-~/.claude}/skills` |
| GitHub Copilot | `${COPILOT_HOME:-~/.copilot}/copilot-instructions.md` | `~/.agents/skills` |
| Kiro | `${KIRO_HOME:-~/.kiro}/steering/AGENTS.md` | `${KIRO_HOME:-~/.kiro}/skills` |
| OpenCode | `${XDG_CONFIG_HOME:-~/.config}/opencode/AGENTS.md` | `~/.agents/skills` |

The `core` skills are `babysit-pr`, `council`, `file-pr`, `full-stop`,
`leaving-pr-comments`, `modernize`, `sol-loop`, `takeover` and `untangle`. The
`ui` skills are `beast-ui`, `doc-view`, `frontend-design` and `impeccable`.
The skills are copies, so an edit to a skill never changes this checkout.

Kiro ignores a `KIRO_HOME` that is not an absolute folder outside your home
folder path. Setup does not install Sentinel with such a value. Copilot CLI
reads `~/.agents/skills` also when you set `COPILOT_HOME`.

Check runs the Sentinel check. When a file changed, Check reports it and
Repair installs Sentinel again. When you untick Sentinel, setup removes the
rules and skills from each agent that it installed them for. Then it deletes
its Sentinel checkout. Setup records `CODEX_HOME`, `CLAUDE_CONFIG_DIR`,
`COPILOT_HOME`, `XDG_CONFIG_HOME` and `KIRO_HOME` at the install. Check, update
and removal use these recorded values, and setup shows a note when they changed.

A Renovate pull request proposes a new Sentinel pin. After you merge and pull
it, **Update tools** applies it. Coming from dotfiles v1.0.0? Read
[Sentinel rules and skills](docs/UPGRADING.md#sentinel-rules-and-skills).

Authenticated agent execution remains a separate verification step. Notes remain personal: Neovim
enables the notes integration only when its vault exists; `NOTES_VAULT` can select another existing vault.

## Ownership and removal

Portable tools live in private, checksum-pinned generations. APT, Homebrew and
Microsoft installers handle the resources that need native installation.
There is no Nix, Home Manager, nix-darwin or chezmoi runtime. The installer
does not change the account login shell; launch the selected shell explicitly or
change that account preference through your operating system.

Removal follows the dependency graph. A dependency used by another selected tool
stays installed. Folders that setup created for a configuration file are removed
with it while they are still empty; anything you added keeps its folder. Pre-existing tools remain unowned. Removing an unused owned
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

On Linux, setup refreshes APT's package lists before installing a package, the
same way `sudo apt update` does. A source that fails with only a warning, such as
a third-party repository whose signing key expired, keeps its older verified lists
and setup continues. If the refresh fails with an error, fix or disable that
source (`sudo apt update` names it), then run setup and choose **Resume
interrupted operation**.

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

Start it from a normal shell with `herdr`. On Windows, Herdr resolves the managed
PowerShell 7 executable from that shell's PATH, so new panes load the same profile, history list, and
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

### Navigation (Mac and Linux)

The same physical keys move between apps, windows and workspaces on both
machines, using Mac key labels. On the Mac they are macOS and AeroSpace
shortcuts. On Linux, the optional "Mac keyboard shortcuts" remapper turns them
into the matching GNOME shortcuts. Windows is not changed.

| Keys | macOS | Linux (with Mac keyboard shortcuts) |
|---|---|---|
| `Cmd+Tab` | Switch apps (hold Cmd to keep choosing) | GNOME app switcher (hold Cmd to keep choosing) |
| ``Cmd+` `` | Next window of the app | GNOME switcher for the app's windows |
| `Cmd+Space` | Spotlight | Activities search |
| `Ctrl+Up` | Mission Control | Activities overview |
| `Ctrl+Left` / `Ctrl+Right` | Previous / next macOS Space | Previous / next workspace |
| `Cmd+H` / `Cmd+M` | Hide / minimize | Hide the window |
| `Ctrl+Cmd+Q` | Lock the screen | Lock the screen |
| `Option+1` ... `Option+9` | Go to workspace 1...9 (AeroSpace) | Go to workspace 1...9 (after the one-time setup below) |
| `Option+Shift+1` ... `Option+Shift+9` | Send the window to workspace 1...9 | Send the window to workspace 1...9 (same setup) |
| `Option+Tab` | Back to the previous workspace | `Alt+Tab`: a quick tap returns to the previous app |
| `Ctrl+Option+L` / `Ctrl+Option+H` | Focus the window on the right / left | Next / previous window (`Alt+Esc` / `Shift+Alt+Esc`) |

Known differences:
- Without the one-time setup, Linux `Option+1...9` opens Ubuntu Dock app 1...9
  and `Option+Shift+1...9` opens a new window of it: GNOME gives those keys to
  the dock until you move them.
- `Option+Tab` on Linux goes back to the previous app, not workspace; holding it
  shows GNOME's switcher.
- GNOME does not tile windows side by side automatically as AeroSpace does.
- Pinning an app to a workspace is an AeroSpace feature; on Linux, open the app
  and press `Option+Shift+<digit>`.

#### One-time GNOME setup (Linux)

GNOME starts with workspaces that appear and disappear as you use them, and it
gives `Super+1...9` to the dock. These commands give it nine fixed workspaces,
like AeroSpace, and move `Super+1...9` (your `Option+1...9`) to them. Setup never
runs them: they are your desktop settings. Paste them into a terminal as they
are; they take effect immediately:

```sh
gsettings set org.gnome.mutter dynamic-workspaces false
gsettings set org.gnome.desktop.wm.preferences num-workspaces 9
gsettings set org.gnome.shell.extensions.dash-to-dock hot-keys false
for i in {1..9}; do gsettings set org.gnome.shell.keybindings switch-to-application-$i "[]"; done
for i in {1..9}; do gsettings set org.gnome.desktop.wm.keybindings switch-to-workspace-$i "['<Super>$i']"; done
for i in {1..9}; do gsettings set org.gnome.desktop.wm.keybindings move-to-workspace-$i "['<Super><Shift>$i']"; done
```

To undo it:

```sh
gsettings reset org.gnome.mutter dynamic-workspaces
gsettings reset org.gnome.desktop.wm.preferences num-workspaces
gsettings reset org.gnome.shell.extensions.dash-to-dock hot-keys
for i in {1..9}; do gsettings reset org.gnome.shell.keybindings switch-to-application-$i; done
for i in {1..9}; do gsettings reset org.gnome.desktop.wm.keybindings switch-to-workspace-$i; done
for i in {1..9}; do gsettings reset org.gnome.desktop.wm.keybindings move-to-workspace-$i; done
```

### AeroSpace (macOS)

AeroSpace arranges your windows for you. Every window on the screen gets its own
tile, side by side, so nothing hides behind anything else. You get nine
workspaces, like nine separate screens, and switch between them with
`Option+<digit>`. It starts at login and reloads its config when the file
changes. AeroSpace calls the Option key `alt`.

**Get started:**
1. Grant access once: **System Settings -> Privacy & Security -> Accessibility**,
   turn on AeroSpace. macOS does not allow setup to do this for you.
2. Open Ghostty. You start on workspace 1, so it fills workspace 1. Open Zen:
   the two windows now share the screen side by side.
3. Press `Option+Shift+2` to send Zen to workspace 2. Now `Option+1` is your
   terminal and `Option+2` your browser, each full width. `Option+Tab` flips
   between the last two workspaces.
4. Use `Ctrl+Option+H/J/K/L` to move focus between windows on a workspace, and
   `Ctrl+Option+Shift+H/J/K/L` to move a window around.

**Want one window to fill the screen?** `Ctrl+Option+F` toggles AeroSpace's full
screen for the focused window. `Ctrl+Option+,` switches the workspace to a
stacked layout, where every window is full size on top of the others; switch
between them with `Ctrl+Option+H/L`, and press `Ctrl+Option+/` for side by side
again.

| Keys | Result |
|---|---|
| `Option+1` ... `Option+9` | Go to workspace 1...9. |
| `Option+Shift+1` ... `Option+Shift+9` | Send the focused window to workspace 1...9. |
| `Option+Tab` | Back to the previous workspace. |
| `Ctrl+Option+H/J/K/L` | Focus the window left/down/up/right. |
| `Ctrl+Option+Shift+H/J/K/L` | Move the focused window left/down/up/right. |
| `Ctrl+Option+-` / `Ctrl+Option+=` | Shrink / grow the focused window. |
| `Ctrl+Option+/` | Side by side (tiles); press again to flip the direction. |
| `Ctrl+Option+,` | Stacked, full size (accordion); press again to flip the direction. |
| `Ctrl+Option+F` | Toggle full screen for the focused window. |
| `Ctrl+Option+Shift+;` | Service mode: then `Esc` reloads the config, `r` resets the layout, `f` floats/tiles the window, Backspace closes all other windows. |

**Pin an app to a workspace.** AeroSpace can send an app's windows to a fixed
workspace every time they open. Nothing is pinned by default. To pin one:
1. Open the app, then run `aerospace list-apps` in a terminal. The middle column
   is the app's ID, for example `com.mitchellh.ghostty` for Ghostty or
   `app.zen-browser.zen` for Zen.
2. Open `aerospace/aerospace.toml` in your dotfiles checkout. It is the same file
   as `~/.config/aerospace/aerospace.toml`.
3. Add this at the end, with your app's ID and workspace number:
   ```toml
   [[on-window-detected]]
   if.app-id = 'com.mitchellh.ghostty'
   run = 'move-node-to-workspace 1'
   ```
4. Save. AeroSpace reloads by itself, and new windows of that app now open on
   that workspace. Windows that are already open stay where they are.
5. To unpin, delete those three lines and save.

The file belongs to your dotfiles repository, so commit the change to keep it on
every Mac: `git reset --hard` (used to update the checkout before the installer
lands on `main`) discards uncommitted edits.

**Gotchas:**
- Avoid the green full-screen button. It puts the window in its own macOS Space,
  which AeroSpace does not manage; use `Ctrl+Option+F` instead.
- AeroSpace hides the windows of other workspaces by parking them in a corner of
  the screen, so Mission Control shows them small in a corner.
- Dialogs and some small windows float instead of taking a tile.

Config: `~/.config/aerospace/aerospace.toml`.

### Mac keyboard shortcuts (Linux)

For a Mac-layout keyboard shared with a Mac, for example a Logitech keyboard with
Easy-Switch. Linux sees its Cmd key as Alt and its Option key as Super. The xremap
key remapper makes the same physical keys behave as they do on macOS, in every
app. Select it on Linux only; macOS needs nothing and Windows is not changed.

Setup asks for your sudo password once to install one udev rule. The rule lets
only the user logged in at the screen read keyboards and create the remapper's
virtual keyboard (logind seat access), never an SSH session. Like any key
remapper, programs running as you could then read keystrokes. The remapper starts
at your next login; log out and back in after setup. Unselecting the tool removes
the rule, the login entry and the configuration link.

Moving between apps and windows is in [Navigation](#navigation-mac-and-linux).
The other keys:

| Keys (Mac labels) | Result |
|---|---|
| `Cmd+C`/`V`/`X`/`Z`/`A`/`S`/`F`/`T`/`W`/`Q`, `Cmd+Shift+Z`/`T`, `Cmd+click` | The app's own copy, paste, undo, save, find, new/close tab, quit... |
| In a terminal: `Cmd+C`/`V`, `Cmd+T`/`N`/`W`/`Q`, `Cmd+D`, `Cmd+Shift+D`, `Cmd+K`, `Cmd+1...9` | Ghostty's copy, paste, tab/window, splits, clear, go to tab. Other `Cmd` keys do nothing, so `Ctrl+C`, `Ctrl+Z` and `Ctrl+D` stay distinct |
| `Cmd+Option+Up`/`Down` (or `Left`/`Right`), `Cmd+Shift+[`/`]` | Previous / next tab, including Zen |
| `Cmd+[` / `Cmd+]` in a browser | Back / forward (`Cmd+←/→` stays start/end of line, also in browsers) |
| `Cmd+Left`/`Right`, `Cmd+Up`/`Down` | Line start/end, document start/end (add `Shift` to select) |
| `Option+Left`/`Right`, `Option+Backspace` | Previous/next word, delete word |
| `Cmd+Backspace` | Delete to line start; in a file manager, move to trash |
| `Cmd+Shift+3` / `Cmd+Shift+4` / `Cmd+Shift+5` | Full screenshot / screenshot tool |

`Ctrl` keeps its Linux meaning: `Ctrl+C` interrupts a terminal and `Ctrl+Tab`
changes browser tabs. `Option` is `Alt` in terminals, as Ghostty's
`macos-option-as-alt` does on the Mac, so fzf's `Alt+C` and Neovim's `Alt+h/j/k/l`
stay on the same physical keys. The exceptions are the navigation keys
`Option+1...9`, `Option+Shift+1...9` and `Ctrl+Option+H/L`, which work the same
in every identified window; `Cmd+1...9` still selects a Ghostty tab. Rules apply only to windows the remapper can
identify: in GNOME Wayland sessions that needs xremap's GNOME Shell extension,
which setup does not install, so keys stay plain there. Right Alt (AltGr) is
unchanged for typing accented characters.

Config: `~/.config/xremap/config.yml`, a live link to `xremap/config.yml`. Edits
and dotfiles updates (`git pull`) reload automatically within half a second.
If you set it up before 2026-10-22, run **Update tools** and log out and back in
once to get this.

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

On macOS and Linux, selecting Neovim makes it your editor in new terminals: the
managed shell environment sets `EDITOR` and `VISUAL` to `nvim` (Git commit
messages, `crontab -e` and similar) and `SUDO_EDITOR` to the managed nvim's full
path. Edit a root-owned file such as `/etc/hosts` with `sudo -e /etc/hosts`
(Linux also has `sudoedit`; macOS does not). Your own nvim, configuration and
plugins open a copy as your user, and sudo writes it back when you quit. Avoid
`sudo nvim`: it runs Neovim as root without your configuration. sudoedit ignores
PATH, hence the full path; if that path contains a space, `SUDO_EDITOR` is left
unset and sudoedit uses its usual editor. Override any of these in
`~/.zshrc.local` (or below the managed block in `.bashrc`). An editor whose name
contains "vi" makes plain zsh start with vi key bindings; managed zsh uses vi mode
anyway, so add `bindkey -e` to `~/.zshrc.local` only if you want emacs keys.
Windows is unchanged.

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

**zsh from bash terminals** is for accounts whose login shell is bash and cannot
be changed, such as company directory (LDAP/SSO) accounts where `chsh` has no
local entry to edit. Ghostty, Herdr, tmux and Neovim's terminal start the login
shell or `$SHELL`, so they open bash. With this option, an interactive bash on a
terminal switches to zsh and sets `SHELL` to it, so those tools and SSH logins
open zsh too. Scripts, `bash -c` and an account whose shell is already zsh are
unchanged, and typing `bash` inside zsh still gives bash. If zsh cannot start,
the terminal stays in bash. Your account's real login shell stays bash: `sudo -i`,
`su -` and cron still use it. Functions and aliases that exist only in bash
startup files (for example company proxy or tool setup in `/etc/bash.bashrc`)
do not carry over; add zsh equivalents to `~/.zshrc.local`. Untick it to return
to bash.

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
- `copilot`: GitHub Copilot CLI. Start it in a project directory and type
  `/login` once; `copilot --help` lists its commands. Needs a Copilot
  subscription. See the [Copilot CLI docs](https://docs.github.com/copilot/concepts/agents/about-copilot-cli).
- `kiro-cli`: Kiro CLI. Run `kiro-cli login` once, then `kiro-cli`
  in a project directory to chat; `kiro-cli --help` lists its commands. See the
  [Kiro CLI docs](https://kiro.dev/docs/cli/).
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
| Neovim over SSH, no tool above | OSC 52 copy through the terminal. `p` pastes the last Neovim copy. Use the terminal paste key for the local clipboard. |
| Other headless Neovim | Explicit OSC 52 or custom provider (`:help clipboard-osc52`) |

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
The real C++ project-isolation test also requires `clang++`. Linux process-inspection
tests require `cc` with pthread support.

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

[CLAUDE.md](CLAUDE.md) is the coding guide. [Release maintenance](docs/RELEASING.md) defines
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
| `ghostty/`, `windows-terminal/`, `aerospace/`, `xremap/` | Desktop configuration |
| `git/`, `lazygit/config.yml`, `gh-dash/`, `starship/`, `lsd/` | Tool configuration |
| `lazygit/config.windows.yml`, `herdr/config.windows.toml` | Passive released Windows live-link sources for migration |
| `home/`, `shells/`, `windows/chezmoi-*` | Passive released sources needed during migration |
| `tests/`, `.github/workflows/` | Local and native verification |
| `docs/` | Upgrade, release and security guides, and release notes |

## License

See [LICENSE](LICENSE). Pinned tool artifacts and the Mason registry snapshot
keep their own licenses.


## Acceptance coverage

GitHub runs real native installation, migration, application and preservation
checks on the supported targets. The core jobs begin with keyboard-driven public
setup, followed through Check, Update, Repair and Remove/Reinstall. Agent checks
run the installed CLIs without model calls. Desktop checks use the VS Code editor
to edit, save, close and open a document again. Actual reboots, first-time
permission grants and visual usability remain manual checks. No dedicated CI
Mac is necessary.
