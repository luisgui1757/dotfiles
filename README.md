# Dotfiles

Cross-platform terminal and editor setup for Apple Silicon macOS, Linux, WSL,
and Windows.
The repo owns the daily shell/editor stack: Neovim, tmux/psmux, Starship, zsh,
PowerShell, Ghostty, WezTerm, lazygit, `lsd`, `zoxide` smart-cd, the `gh` CLI with the
`gh-dash` dashboard, the `pi` CLI, Windows Terminal theming, locked plugin restore, LSP /
formatter provisioning, and global Polaris agent-policy bootstrap. It also provisions the `tree-sitter` CLI needed by
`nvim-treesitter` main parser builds, plus AeroSpace (macOS tiling WM) and Herdr
(agent multiplexer: macOS/Linux stable, Windows preview) through vendor-channel installs.

The public interface is intentionally small:

```text
run setup -> install programs -> link configs -> restore Neovim plugins -> sync Mason tools
```

For a fresh machine, run `setup`. The split is deliberate:
Nix is the enforced package layer on macOS/Linux/WSL; `install-deps` handles
native/deferred tools and Windows packages; chezmoi owns the dotfiles/config
layer in `home/`. The full setup scripts apply configs through chezmoi.

Updates are deliberately two-track. The reproducible core is pinned to an
immutable reviewed release: Neovim plugins (`nvim/lazy-lock.json`) and configs
change through the versioned side-by-side upgrade, then setup. The
drift-tolerant edge is package-manager CLI tools,
proven dotfiles-owned direct artifacts on Unix, and Mason LSPs; refresh only
that edge with `./setup.sh --update` or `.\setup.ps1 -Update`.

## Quick Start

Clone the repo first, then run the local setup entry point. `setup.{sh,ps1}`
installs repo-managed dependencies, links every config, then runs
`:Lazy! restore`, a synchronous nvim-treesitter parser install, and
`:MasonToolsInstallSync` before the first interactive Neovim launch.
Piped or stdin setup is intentionally disabled; if setup cannot prove it is
running from a local checkout, it fails closed with clone-first instructions.

Git is required to clone this repo. On macOS/Linux/WSL, Nix is also required
before `setup.sh` runs: setup applies the pinned nix-darwin/Home Manager layer
first and fails closed if `nix` is missing. Published v0.2.0 checkouts include
`scripts/install-nix-prerequisite.sh`, which downloads the official upstream
Nix 2.34.0 release and verifies the platform SHA-256 before extraction or
execution. The helper and the versioned upgrade tools refuse branches, moving
`main`, lightweight tags, dirty checkouts, and non-official release identities;
this repo has no pipe-to-shell Nix bootstrap.

```bash
# Apple Silicon mac / linux / wsl, after the annotated v0.2.0 release exists
git clone --branch v0.2.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles
cd ~/dotfiles
./scripts/install-nix-prerequisite.sh --install
./setup.sh --all
```

```powershell
# windows, after the annotated v0.2.0 release exists
# enable Developer Mode, then run from a normal PowerShell
# Settings -> Privacy & security -> For developers -> Developer Mode = On
git clone --branch v0.2.0 --single-branch `
  https://github.com/luisgui1757/dotfiles.git $HOME\dotfiles
Set-Location $HOME\dotfiles
.\setup.ps1 -All
```

For WSL, treat setup as split-host: run `.\setup.ps1 -All` on Windows so
Windows Terminal, Hack Nerd Font, lazygit, and `win32yank` are installed on the
rendering host, install Nix inside the WSL distro, then run `./setup.sh --all`
inside WSL for the Linux CLI/editor stack. Windows Terminal settings handling
runs by default, and setup independently stages, validates, backs up, and
atomically merges each existing packaged/portable target. If Scoop, winget, and choco
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

Remain on v0.1.0 until the annotated v0.2.0 release is published. Then follow
the versioned, side-by-side procedure in [docs/UPGRADING.md](docs/UPGRADING.md):
retain the exact v0.1.0 checkout, clone v0.2.0 separately, run the read-only
preflight, apply the transactional migration, verify, and explicitly accept or
rollback from the printed private recovery directory. Apple Silicon macOS and
Linux/WSL include Nix-generation rollback; Windows independently recovers
known-folder config and exact packaged/portable Terminal bytes. macOS migration
is available only on Apple Silicon.
Every platform publishes and rolls back only from digest-bound release trees in
that private directory; full post-acceptance setup later repoints config to the
retained v0.2.0 checkout.

`./setup.sh --update` and `.\setup.ps1 -Update` are package/Mason refreshes,
not release migrations: they do not fetch Git, activate the versioned config,
or perform the Nix ownership transition.

### Existing Checkout

```bash
./setup.sh                       # Y/n per dep, end-to-end
./setup.sh --all                 # non-interactive
./setup.sh --update              # update proven tools/artifacts + Mason, no git/config/Lazy
./setup.sh --dry-run             # preview
./setup.sh --experimental-wsl-gui # WSL-only opt-in for Linux GUI terminal bits
./setup.sh --nix-darwin          # compatibility alias; macOS setup already applies nix-darwin
./setup.sh --home-manager        # compatibility alias; Linux/WSL setup already applies Home Manager
./setup.sh --skip-deps           # skip Nix + native/deferred dependency provisioning
./setup.sh --skip-native-deps    # keep Nix/config; skip native/deferred dependencies
./setup.sh --skip-config         # skip chezmoi config apply
./setup.sh --skip-config-scripts # apply config files/links; defer chezmoi run scripts
./setup.sh --skip-nvim           # skip Lazy/Tree-sitter/Mason phases
./setup.sh --skip-agents         # skip global Polaris agent policy
./setup.sh --best-effort         # continue after nvim-phase failures; exit nonzero with summary
make setup                       # same as ./setup.sh, via the Makefile
```

**Nix layer (required POSIX packages only).** `flake.nix` + a committed
`flake.lock` provide the macOS/Linux/WSL package layer. macOS uses nix-darwin +
declarative Homebrew + Home Manager; Linux/WSL uses standalone Home Manager.
chezmoi still owns **every** dotfile; Nix owns no config. A normal `./setup.sh`
or `./setup.sh --all` applies the matching package layer before native/deferred
dependency provisioning. On macOS setup normalizes `uname -m` and runs only the
Apple Silicon `sudo env DOTFILES_TARGET_USER=... DOTFILES_TARGET_HOME=...
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
`sudo env DOTFILES_TARGET_USER=... DOTFILES_TARGET_HOME=... nix run
github:nix-darwin/nix-darwin/<locked-rev>?narHash=<encoded-narHash>#darwin-rebuild -- ...`;
it never uses the mutable `nix-darwin` registry alias. On first bootstrap,
pre-existing `/etc/bashrc` and `/etc/zshrc` are moved only to nix-darwin's
documented `.before-nix-darwin` names after both backup paths pass a collision
preflight. Activation failure or interruption quarantines any generated
replacement and restores both originals; success retains the backups for
nix-darwin recovery/uninstall. On Linux/WSL, setup runs
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
On real Macs, nix-darwin keeps `homebrew.onActivation.cleanup = "check"` so
undeclared Brew drift aborts activation without uninstalling anything. The
GitHub-hosted macOS setup job is the only exception: setup passes a
repo-scoped `DOTFILES_NIX_DARWIN_HOSTED_CI=1` marker because runner images ship
a large preinstalled Brew surface outside this dotfiles Brewfile. The marker is
only automatic when Actions reports `RUNNER_ENVIRONMENT=github-hosted` and
`RUNNER_OS=macOS`; self-hosted Macs keep the strict drift check.
nix-homebrew uses `autoMigrate = true` so Macs that already have official-script
Homebrew can be adopted by the declarative Nix layer; upstream's migration keeps
installed packages while replacing the Homebrew repositories. Because this repo
keeps `mutableTaps = false`, setup moves an existing architecture-correct tap
directory (`/opt/homebrew/Library/Taps`) to a unique timestamped backup.
Activation, bootstrap, or interruption failure quarantines any replacement and
restores the original taps; rollback failure prints exact manual recovery. On
success the backup remains available. The `nikitabobko/tap` tap is also
explicitly trusted through nix-homebrew because Homebrew 5 refuses to load
personal-tap casks, including AeroSpace, without a trust entry.
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
are not deleted: validated packaged/portable backups restore independently, and
the displaced current file is preserved as `settings.json.uninstall-current.*`.
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
      -> nvim "+MasonToolsInstallSync" +qa        phase 5: LSP servers and formatters
      -> Polaris global install       phase 6: per-user agent policy
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
prompt).
When setup detects redirected stdin/stdout and neither all nor dry-run was
requested, it defaults to all and prints `note: no TTY detected; running with
--all` (or `-All`). An interactive run (no all flag) can still ask the
dependency installer's **"install EVERYTHING without further prompts? [Y/n]"**
question; answer `Y` to pull the tool catalog in one go, or `n` to choose per
tool. Phase 6 then asks **"Apply Polaris global agent rules? [Y/n]"** unless
`--all` / `-All`, `--dry-run` / `-DryRun`, or `--skip-agents` / `-SkipAgents`
already made that decision.
Add `--dry-run` / `-DryRun` to preview every step without touching disk.
Pass `--skip-agents` / `-SkipAgents` to leave global AI-agent entrypoints alone.
Pass `--update` / `-Update` from an existing checkout to run only the
drift-edge refresh: scoped package-manager updates for present catalog tools,
Unix direct-artifact refreshes only when dotfiles provenance proves ownership,
then `nvim --headless +MasonToolsUpdateSync +qa`. The synchronous Mason command
keeps headless Neovim alive until package installs finish. It deliberately skips
`git pull`, `chezmoi apply`, `:Lazy restore`, synchronous Tree-sitter parser
bootstrap, `:Lazy sync`, and `:Lazy update`; the last two can change
`lazy-lock.json` and are therefore repo updates, not machine refreshes.

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
copies; redirected LocalApplicationData/Documents targets are symlink overlays;
and Windows Terminal remains a merge.

| Tool | macOS | Linux / WSL | Windows |
|---|---|---|---|
| Neovim | `~/.config/nvim` -> `nvim/` | `~/.config/nvim` -> `nvim/` | `%LOCALAPPDATA%\nvim` -> `nvim\` |
| Starship | `~/.config/starship.toml` -> `starship/starship.toml` | same | `%USERPROFILE%\.config\starship.toml` -> `starship\starship.toml` |
| zsh | `~/.zshenv` -> `shells/zshenv`; `~/.zshrc` -> `shells/zshrc` | same | n/a |
| PowerShell | n/a | n/a | actual runtime `$PROFILE` plus Console/VS Code/ISE host profiles under the real Documents known folder -> `shells\powershell_profile.ps1` |
| tmux / psmux | `~/.tmux.conf` -> `tmux/tmux.conf`; `~/.tmux.posix.conf` -> `tmux/tmux.posix.conf` (POSIX clipboard + TPM functional plugins + generated Rose Pine bar); `~/.tmux.rose-pine.{main,moon,dawn}.conf` -> generated `tmux/psmux-rose-pine.{main,moon,dawn}.conf` (Omer-shaped Rose Pine bar, **shared** with Windows) | same | `%USERPROFILE%\.psmux.conf` -> `tmux\psmux.conf` (first psmux entrypoint, disables warm sessions, then flag-free source-files the Windows overlay); `%USERPROFILE%\.tmux.conf` -> `tmux\tmux.conf`; `%USERPROFILE%\.tmux.windows.conf` -> `tmux\tmux.windows.conf`; `%USERPROFILE%\.tmux.rose-pine.ps1` -> `tmux\psmux-rose-pine.ps1` (Rose Pine bar generator / manual live-switch helper); `%USERPROFILE%\.tmux.rose-pine.{main,moon,dawn}.conf` -> the same generated `tmux\psmux-rose-pine.{main,moon,dawn}.conf`; the POSIX overlay is **excluded** on Windows (its `if-shell` probes hang psmux); WSL uses the Unix path |
| Ghostty | `~/Library/Application Support/com.mitchellh.ghostty/config` -> `ghostty/config` | native Linux links `~/.config/ghostty/config`; WSL links it only with `--experimental-wsl-gui` | n/a |
| WezTerm | `~/.config/wezterm/wezterm.lua` -> `wezterm/wezterm.lua` | same; WSL links it only with `--experimental-wsl-gui` | `%USERPROFILE%\.config\wezterm\wezterm.lua` -> `wezterm\wezterm.lua` (copied) |
| AeroSpace | `~/.config/aerospace/aerospace.toml` -> `aerospace/aerospace.toml` (macOS tiling WM; focus/move on `ctrl-alt(-shift)` to avoid nvim `<A-h/j/k/l>` and fzf `Alt-c`) | n/a (macOS-only) | n/a (macOS-only) |
| lazygit | `~/Library/Application Support/lazygit/config.yml` -> `lazygit/config.yml` | `~/.config/lazygit/config.yml` -> `lazygit/config.yml` | `%LOCALAPPDATA%\lazygit\config.yml` -> `lazygit\config.windows.yml` |
| lsd | `~/.config/lsd/{config.yaml,colors.yaml}` -> `lsd/{config.yaml,colors.yaml}` | same | `%USERPROFILE%\.config\lsd\{config.yaml,colors.yaml}` -> `lsd\{config.yaml,colors.yaml}` |
| gh-dash | `~/.config/gh-dash/config.yml` -> `gh-dash/config.yml` | same | `%USERPROFILE%\.config\gh-dash\config.yml` -> `gh-dash\config.yml` |
| Windows Terminal | n/a | n/a | app installed by `setup.ps1` through Scoop/winget/choco, with a SHA-256-verified portable zip fallback; setup treats packaged and portable `settings.json` as independent targets, stages and validates both before publication, creates separate verified backups, detects concurrent changes through atomic replacement rollback bytes, and rolls the transaction back on failure; opt out with `-SkipWindowsTerminalMerge`; see [windows-terminal/README.md](windows-terminal/README.md) |

Windows setup resolves UserProfile, LocalApplicationData, Documents, and the
active host's `$PROFILE` independently through supported runtime/known-folder
APIs. It applies the UserProfile source plus dedicated LocalApplicationData and
Documents source states, then verifies the paths Neovim, lazygit, ConsoleHost,
VS Code, and ISE consume. Redirected folders, alternate drives, and spaces are
supported. Directory ownership checks resolve both symbolic links and Windows
junctions. Recognized conventional-path legacy targets are backed up only after
the new targets publish; divergent legacy user data stays in place with a
migration warning. POSIX pwsh profile management remains provisioning-adjacent.

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
  `~/.local/bin` on native Linux/WSL, and Scoop with npm fallback on Windows.
  Windows `-All` also installs VS Build Tools so parser builds can find MSVC;
  after winget/choco failures it falls back to Microsoft's official
  `vs_BuildTools.exe` bootstrapper with the same VCTools workload, but only
  after Authenticode verifies a valid Microsoft-owned signer/chain.
- Neovim Markdown rendering is owned by `render-markdown.nvim`. Setup already
  installs the explicit Tree-sitter parser matrix, including `latex`; it also
  installs `latex2text` through a pinned, SHA-256-checked venv
  (`setuptools` 80.9.0, `pylatexenc` 2.10) so rendered Markdown equations work
  on fresh machines instead of depending on a random host Python package.
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
- `setup.sh --update` and `setup.ps1 -Update` are scoped and manager-aware. They
  update only present catalog tools with proven per-tool ownership, then run an
  exact per-package or repo-pinned artifact refresh such as
  `brew upgrade <formula>`, `apt-get install --only-upgrade <pkg>`,
  `scoop update <pkg>`, `winget upgrade --id <id> -e`, or
  `choco upgrade <pkg> -y`. They never run blanket upgrades such as
  `brew upgrade`, `apt upgrade`, `pacman -Syu`, `scoop update *`,
  `winget upgrade --all`, or `choco upgrade all`. Unix ownership is resolved
  from the executable source: Homebrew/Linuxbrew requires the PATH-visible
  command path and its resolved executable target to stay under `brew --prefix`,
  plus an installed formula and `brew list --formula <formula>` file ownership
  of the resolved executable; native Linux managers require file ownership proof
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
  chezmoi-owned config. The tap is a pinned nix-homebrew input and is explicitly
  trusted so Homebrew 5 will load the cask. Its keymap deliberately avoids the
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
  remote-eval installers remain banned. Native-Linux Herdr writes the same
  provenance marker as the other dotfiles-owned direct artifacts, so
  `./setup.sh --update` can prove ownership and refresh only the repo-pinned
  version. Windows Herdr is beta/ConPTY-backed, so runtime behavior remains a
  manual checklist item before treating it as a daily driver.
- WSL fonts are host-rendered in the supported path. Install and merge Windows
  Terminal from Windows (`.\setup.ps1 -All`; the merge is default-on); the WSL
  Linux fontconfig install is only for `--experimental-wsl-gui`.
- VS Code is optional. On WSL, use Windows VS Code plus `code .` for Remote -
  WSL, or use a Linux GUI build when WSLg / X11 is available. Rosé Pine setup
  follows whatever `code` CLI is on PATH.
- Polaris agent policy is a supported setup phase, not a synced dotfile. `setup`
  pins Polaris `0.1.2` (`v0.1.2`) at commit
  `ecca742fa9ed1243a73981955850c1a8ef3e3b04`,
  caches that checkout under `~/.local/share/dotfiles/polaris/<commit>` on
  POSIX and `%LOCALAPPDATA%\dotfiles\polaris\<commit>` on Windows, verifies that
  `v0.1.2` peels to the pinned commit plus the checkout `VERSION`, and runs
  every Polaris Git operation with
  system/global/env config, templates, hooks, and executable Git config features
  disabled, then runs Polaris' Bash global installer and global check
  (`tools/install --global`, then `--global --check`; Windows uses a validated
  Git Bash with `cygpath`, not WSL bash or another PATH-only Bash).
  The global installer writes the per-user AI entrypoints for Codex
  (`~/.codex/AGENTS.md`), Claude Code (`~/.claude/CLAUDE.md`), opencode
  (`~/.config/opencode/AGENTS.md`), and Pi CLI (`~/.pi/agent/AGENTS.md`);
  Copilot has no reliable global file path, so user-wide Copilot instructions
  remain a manual VS Code/github.com profile step. To remove the global blocks,
  run the cached Polaris Bash installer with `--global --remove` on POSIX or
  from Git Bash on Windows. Project/team adoption is separate: run Polaris
  repo-local install or vendoring in that project and commit those files there.
- Pi CLI is a provisioned binary, not synced runtime state. Setup installs
  `@earendil-works/pi-coding-agent@0.80.3` by running `npm pack`, requiring the
  pack metadata and the actual tarball SHA-512 bytes to match the reviewed SRI,
  then passing only that verified local tarball to `npm install`. Temporary
  pack state is removed on success, mismatch, failure, interruption, and retry.
  The reviewed SRI is
  `sha512-TIggw9gCXpA+Ph7OjdTA7ka2NPwTVuPmy39KDSyUzaKq8VvHfMGR7vtRz4JB7Um/RMRblmzhu4p9tUCk6MTgGA==`.
  POSIX public setup gets Node 24 from Nix first; Windows uses the native Node
  LTS catalog entry. Local `.pi/` sessions and preferences stay machine-local.
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
| `setup.sh / ubuntu-24.04` | Full public Unix setup on the hosted Ubuntu runner after installing Nix in CI: Home Manager first, then native/deferred installs, chezmoi, Lazy, Tree-sitter, Mason, and Polaris. Its clean login/interactive PATH proof resolves the effective account's actual login zsh from the account database; this matters because fresh Ubuntu has no `/usr/bin/zsh` and setup selects Linuxbrew zsh. The shell must resolve `rg` from Nix with no caller PATH injection. |
| `setup.sh / macos-26` | Full public Apple Silicon setup through the hosted runner: architecture-matched nix-darwin/declarative Homebrew, native/deferred installs, real Ghostty/WezTerm config consumption, installed AeroSpace app/CLI identity agreement, chezmoi, Lazy, Tree-sitter, Mason, and Polaris. AeroSpace waits for a user-granted Accessibility permission before parsing user config or starting its CLI server, so managed-config consumption remains explicit TCC-enabled desktop proof in `tests/MANUAL.md`; hosted CI does not pretend to prove it. The hosted runner alone gets the cleanup override; real hosts keep `cleanup = "check"`. |
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
| `Protect main: integrity` | none | Requires pull requests, strict required checks, current `main`, squash-only merges, linear history, no branch deletion, and no non-fast-forward updates. |
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
| Polaris version/tag/commit | Manual-reviewed mirror between `setup.sh`, `setup.ps1`, README, CLAUDE, and `tests/static/pin_consistency_test.sh`. |
| Scoop installer | Renovate can bump `ScoopInstaller/Install` commit `b0ee913725139b816f9178163af0aecdba07a7ed`; SHA `48f6ea398b3a3fa26fae0093d37bd85b13e7eaa5d1d4a3e208408768408e35ae` is human-reviewed. |
| TPM/tmux plugin refs and psmux plugin ref | Commit pins are manual-reviewed and mirrored in docs/tests; Renovate does not recompute or prove tag commits. |
| `setuptools`/`pylatexenc` | Renovate can bump versions; adjacent hashes remain human-reviewed. Current pins: `setuptools` 80.9.0, `pylatexenc` 2.10. |
| Hack Nerd Font | Unix and Windows mirrors must stay identical; version/hash drift is caught by `pin_consistency_test.sh`. |
| Pi CLI | Unix/Windows install pins and e2e assertions mirror version `0.80.3`; the npm-pack metadata and downloaded tarball bytes must both match the human-reviewed SRI. |
| gh-dash | Tag `v4.25.1`, annotated tag object `e6ebbd7e83e30161b9192ce3339972d2c8269e7f`, and peeled commit `49f37e4832956c57bf52d4ea8b1b1e5c0f863700` are mirrored; installers verify the tag mapping and pin the extension to the commit. |

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
- **Polaris is global agent policy, not a dotfile mirror.** Setup installs it
  from a pinned, version-checked upstream checkout and lets Polaris own its
  managed global entrypoint blocks. This repo does not vendor Polaris core or
  sync agent runtime state.
- **Rose Pine everywhere it can render.** Nvim, lualine, foreground-only
  Starship, tmux/psmux, `lsd`, ghostty, Windows Terminal, PSReadLine — same
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
  failing on post-format LSP warnings/errors. `:WNF` (or `:wnf`) skips
  formatting for one save.
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
  each packaged/portable target independently, stages and validates every
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
  then refreshes the current-process Scoop shims path so later installs can use
  `scoop` immediately. Existing Scoop installs also get the `extras` and
  `nerd-fonts` buckets normalized before catalog installs.
  Chezmoi's Windows nvim directory symlink still uses the elevated/native
  CreateSymbolicLink path. For local machines, Developer Mode plus a normal
  PowerShell remains the recommended setup path.

See `AGENTS.md` for the standard coding-agent entry point. `CLAUDE.md` remains
the canonical operational guide because Claude Code auto-loads it; `AGENTS.md`
points there so other agents reach the same instructions without duplicating
content.

## Daily workflows

### `:wnf` — write without formatting

In any buffer: `:wnf<CR>` (lowercase) saves the file with formatters skipped
**for this one save**. The next plain `:w` formats normally. Useful in legacy
codebases where the formatter would create a noisy diff. Implemented in
`nvim/lua/vim-options.lua`.

### Neovim keymap discovery & scroll context

which-key pops up a menu of the keys that can follow a prefix once you pause
past `timeoutlen` (e.g. after `<leader>`); `<leader>?` lists the buffer-local
keymaps on demand. The cursor also keeps **16 lines** of context above and below
it (`scrolloff = 16`), so you never edit against the top or bottom edge.

### zoxide — smarter `cd`

`z <partial>` jumps to the best-matching directory you have visited (by
frecency); `zi` opens an interactive picker. Works in zsh and PowerShell; plain
`cd` is unchanged. Nothing to configure — `zoxide init` is wired into both shell
profiles (PowerShell uses a cached init, no `Invoke-Expression`).

### gh-dash — pull-request / issue dashboard

Run `gh dash` for a terminal dashboard of your PRs, review requests, and issues.
Its config (`~/.config/gh-dash/config.yml`, same relative path on Windows) is
applied by setup **always**. The dashboard binary is a pinned `gh` CLI
extension, and it only works once you have authenticated — so setup installs the
extension only **after** `gh auth login`. If you have not authenticated yet,
setup skips the extension cleanly (no error, because an unauthenticated install
hits GitHub's anonymous rate limit); run `gh auth login`, then rerun `setup` /
`install-deps` to pick it up. Before mutation, setup verifies that tag `v4.25.1`
still has annotated tag object `e6ebbd7e83e30161b9192ce3339972d2c8269e7f`
and peels to commit `49f37e4832956c57bf52d4ea8b1b1e5c0f863700`, then installs by that immutable
commit rather than the tag. The token is machine-local and never stored in this
repo.

### Pi CLI

Run `pi --version` to confirm setup installed the pinned Pi CLI. The installer
packs the exact version, verifies the tarball bytes against the pinned SRI, and
installs only that local tarball. Setup installs the CLI only; `.pi/` runtime
state remains local to each machine.

### Command-line vi mode

Both shells edit the command line with **vi keybindings**. Press `Esc` to leave
insert mode and use `h`/`j`/`k`/`l`, `w`/`b`, `dd`, `cw`, `.` etc. on the current
line; press `i`/`a`/`o` to type again. The cursor changes shape with the mode
(beam while typing, block in normal mode). Everything you already use still
works in insert mode: **Tab** opens the fuzzy completion menu, **Up/Down** do
prefix history search, and **Ctrl-R / Ctrl-T / Alt-C** are the fzf history / file
/ cd pickers. The arrows also search history from normal mode.

- **zsh:** enabled with `bindkey -v`. `Esc` responsiveness vs. Alt/Meta chords is
  governed by `KEYTIMEOUT` (250 ms by default). To retune it, export
  `DOTFILES_KEYTIMEOUT` (hundredths of a second) or set `KEYTIMEOUT` in
  `~/.zshrc.local`.
- **PowerShell:** enabled with `Set-PSReadLineOption -EditMode Vi`; the mode
  indicator changes the cursor shape on supported terminals (Windows Terminal).

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
nvim --headless "+MasonToolsUpdateSync" +qa
```

Run on each machine; there's no machine-pinned lockfile for Mason itself.

### Reproducing the same release on another machine

```bash
git clone --branch v0.2.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles
nvim --headless "+Lazy! restore" +qa  # match plugin commits
make test                       # verify the new state
```

Use `docs/UPGRADING.md` when the release version changes. A moving-branch pull
is not a release migration.

## License

MIT. See `LICENSE`.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Neovim stops before loading Lazy with a lockfile/cache identity error | `lazy-lock.json` is missing, malformed, incomplete, has a non-40-hex commit or invalid branch; or the cached `lazy.nvim` checkout is dirty, at the wrong commit, from the wrong origin, missing locked default-branch metadata, non-Git, or partial | restore the tracked `nvim/lazy-lock.json` and restart Neovim. Startup repairs the cache through a verified staging checkout and never executes an unproved path. If publication fails, fix the destination permissions named in the error and retry |
| setup reports a Homebrew `shellenv` failure even though `brew` already resolves | the selected command and PATH-resolved command report different Homebrew prefixes/repositories, or `shellenv` exited nonzero; empty stdout alone is a normal Homebrew idempotence signal | compare `brew --prefix` and `brew --repository` through both entrypoints named in the error. Repair the shadowing PATH or Homebrew installation, then rerun setup; a nix-darwin wrapper and native brew path are accepted only when those identities match |
| first nix-darwin setup reports an existing `.before-nix-darwin` backup | setup found both an unmanaged `/etc/bashrc` or `/etc/zshrc` and an older backup, so choosing either would risk user/system data | compare the two files and resolve the collision deliberately, then rerun. Setup moves neither shell file until both backup destinations are clear and restores both if activation fails |
| `<leader>X` keymaps fire `\X` instead of `<Space>X` | mapleader set after lazy.setup somehow | restore the order in `nvim/init.lua` — leader **before** `require("lazy").setup` |
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
| psmux warns `unknown option` while sourcing config | A psmux-parsed config still contains a tmux-only option | update this repo and re-run `.\setup.ps1 -SkipDeps -SkipNvim`, then restart psmux. The managed shared and Windows configs intentionally avoid `set ... terminal-features`, and the generated Rose Pine artifacts omit tmux-only `display-panes-*` color options; tmux extended-key flags live only in the POSIX overlay |
| tmux / psmux status bar looks fully opaque | The generated status canvas is not using the terminal default background, or an older generated artifact is still loaded | update this repo, re-run setup / chezmoi apply, then restart tmux/psmux. The managed bar sets `status-style` and pill outside caps to `bg=default`; only the pill interiors have explicit Rose Pine backgrounds |
| PowerShell Tab completion — the selected option is **gold** | PSReadLine `Selection` colors the highlighted MenuComplete option | it is a gold foreground. Note: PSReadLine uses that same `Selection` color for the completion suffix it inserts into the command line while you navigate, so that suffix also shows gold until you accept — it is one setting, not separable |
| A `wt --version` window popped up during `setup.ps1 -All` | the dependency version table ran `<tool> --version`, and `wt --version` opens a Windows Terminal window instead of printing | fixed — `Get-CommandVersionString` never runs `wt --version`; it reads the file version (or shows `installed`) |
| Ghostty doesn't open maximized | `window-save-state = always` restored an old geometry over `maximize` (macOS only) | `ghostty/config` uses `window-save-state = default` (not `always`) with `maximize = true`; `always` lets the saved size win |
| Ghostty doesn't load the config | wrong path, or WSL default skip | the install path is `~/Library/Application Support/com.mitchellh.ghostty/config` on macOS and `~/.config/ghostty/config` on native Linux. WSL only links Linux Ghostty config after `./setup.sh --experimental-wsl-gui`; otherwise use Windows Terminal |
| Windows Terminal lost a profile after merge | WT rewrote one installation's file after setup, or an older pre-transactional setup was used | inspect that installation's independent `<settings.json>.bak.<YYYYMMDD-HHMMSS>[.n]` backups; `uninstall.ps1` validates filename order and JSON, restores packaged and portable targets independently, and preserves the displaced current file for recovery |
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
