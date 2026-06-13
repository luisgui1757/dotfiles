# Dotfiles

Cross-platform terminal and editor setup for macOS, Linux, WSL, and Windows.
The repo owns the daily shell/editor stack: Neovim, tmux/psmux, Starship, zsh,
PowerShell, Ghostty, lazygit, Windows Terminal theming, plugin sync, and LSP /
formatter provisioning.

The public interface is intentionally small:

```text
run setup -> install programs -> link configs -> sync Neovim plugins -> sync Mason tools
```

For a fresh machine, run `setup`. The split is deliberate:
`install-deps` installs programs and optional tooling; chezmoi owns the
dotfiles/config layer in `home/`. The full setup scripts now apply configs
through chezmoi.

Updates are deliberately two-track. The reproducible core is pinned in git:
Neovim plugins (`nvim/lazy-lock.json`), SHA-256-verified direct downloads,
and configs. Update that track with `git pull` and then re-run setup. The
drift-tolerant edge is package-manager CLI tools plus Mason LSPs; refresh only
that edge with `./setup.sh --update` or `.\setup.ps1 -Update`.

## Quick Start

No checkout is required. `setup.{sh,ps1}` clones the repo to `~/dotfiles`
or `%USERPROFILE%\dotfiles`, installs repo-managed dependencies, links every
config, then runs `:Lazy! sync` and `:MasonToolsInstallSync` before the first
interactive Neovim launch.

Git is the only hard prerequisite for remote setup because setup needs it to
clone this repo. If Git is missing, the setup scripts print the first install
command for the platform package manager.

```bash
# mac / linux / wsl
curl -fsSL https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.sh | bash -s -- --all
```

```powershell
# windows -- enable Developer Mode, then run from a normal PowerShell
# Settings -> Privacy & security -> For developers -> Developer Mode = On
iwr https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.ps1 -OutFile setup.ps1
.\setup.ps1 -All
```

For WSL, treat setup as split-host: run `.\setup.ps1 -All` on Windows so
Windows Terminal, Hack Nerd Font, lazygit, and `win32yank` are installed on the
rendering host, then run `./setup.sh --all` inside WSL for the Linux CLI/editor
stack. Windows Terminal settings handling runs by default, and setup backs up
the pre-merge packaged file first when it exists. If Scoop, winget, and choco
cannot register the MSIX app, setup falls back to a pinned
SHA-256-verified portable WT zip. Portable WT reads the unpackaged settings
path, so after a real config apply setup either mirrors the merged packaged
settings there or, when packaged WT is absent, seeds/merges that unpackaged file
from `windows-terminal/settings.fragment.jsonc`; pass
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

### Upgrading from a pre-chezmoi install

If you already ran an older (pre-chezmoi) version, just re-run setup — it now
applies the config layer through chezmoi and is backwards-safe:

```bash
git -C ~/dotfiles pull        # or %USERPROFILE%\dotfiles on Windows
./setup.sh --all              # macOS / Linux / WSL
```

```powershell
.\setup.ps1 -All              # Windows
```

Setup installs chezmoi if missing, then **backs up any pre-chezmoi config that
differs** to `<file>.bak.<timestamp>` before chezmoi writes the managed version
(a pre-existing file or symlink whose content already matches is left as-is — no
junk backup). Windows Terminal `settings.json` is backed up and merged in place;
VS Code `settings.json` is edited in place with your comments preserved. Nothing
is deleted. On Windows, if you ran an older psmux that froze on the previous
`tmux.conf`, clear orphaned processes once after upgrading:

```powershell
Remove-Item -LiteralPath "$HOME\.tmux.posix.conf" -Force -ErrorAction SilentlyContinue
taskkill /F /T /IM psmux.exe   # then reopen Windows Terminal
```

Restart Windows Terminal and VS Code afterward so the fonts/theme/launch mode
apply.

### Existing Checkout

```bash
./setup.sh                       # Y/n per dep, end-to-end
./setup.sh --all                 # non-interactive
./setup.sh --update              # update PM tools + Mason, no git/config/Lazy
./setup.sh --dry-run             # preview
./setup.sh --experimental-wsl-gui # WSL-only opt-in for Linux GUI terminal bits
./setup.sh --skip-config         # skip chezmoi config apply
make setup                       # same as ./setup.sh, via the Makefile
```

```powershell
.\setup.ps1
.\setup.ps1 -All
.\setup.ps1 -Update
.\setup.ps1 -DryRun
.\setup.ps1 -SkipConfig
.\setup.ps1 -SkipWindowsTerminalMerge # leave WT settings.json untouched
.\setup.ps1 -MergeWindowsTerminal     # accepted no-op alias; WT merge is default-on
```

### Config Layer (chezmoi)

Chezmoi is the config-only path. It manages dotfiles from `home/`; it does not
install programs, fonts, VS Code, psmux, login shells, or other provisioning
steps. Run `install-deps.sh` / `install-deps.ps1` or full `setup` for those.

The remote one-liner works because `.chezmoiroot` points chezmoi at `home/`:

```bash
chezmoi init --apply luisgui1757/dotfiles
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
the newest `<target>.bak.<timestamp>` backup by default, and leave chezmoi's own
state/config alone. Windows Terminal `settings.json` is never deleted: the merge
is idempotent but not invertible, so restore manually from the printed backup if
you want to undo it.

## What Setup Does

`setup` is a four-phase, idempotent orchestrator:

```text
setup -> install-deps                 phase 1: programs and optional tools
      -> chezmoi apply                phase 2: config layer with pre-apply backups
      -> nvim "+Lazy! sync" +qa       phase 3: plugins
      -> nvim "+MasonToolsInstallSync" +qa  phase 4: LSP servers and formatters
```

Pass `--all` / `-All` for explicit non-interactive installs (Y to every prompt).
When setup detects redirected stdin/stdout and neither all nor dry-run was
requested, it defaults to all and prints `note: no TTY detected; running with
--all` (or `-All`). An interactive run (no all flag) still opens with a single
**"install EVERYTHING without further prompts? [Y/n]"** question — answer `Y`
to pull the lot in one go, or `n` to choose per tool.
Add `--dry-run` / `-DryRun` to preview every step without touching disk.
Pass `--update` / `-Update` from an existing checkout to run only the
drift-edge refresh: scoped package-manager updates for present catalog tools,
then `nvim --headless +MasonToolsUpdate +qa`. It deliberately skips `git pull`,
`chezmoi apply`, `:Lazy sync`, and `:Lazy update`; the last one changes
`lazy-lock.json` and is therefore a repo update, not a machine refresh.

Every script is safe to rerun. Pre-existing non-symlink targets are backed up to
`<target>.bak.<timestamp>` with collision-proof suffixes (`.1`, `.2`, ...).
Before setup lets `chezmoi --force apply` replace an existing managed target, it
backs up only targets that are not already correct: exact chezmoi state and
content-equivalent targets are left alone. On Windows, setup still checks
Developer Mode/elevation before applying because the Neovim directory target
remains a symlink even though single-file configs are copies.

### Managed Configs

The table below is the config layer. Full setup and config-only applies use the
chezmoi source under `home/`. Mechanisms differ: POSIX chezmoi uses symlinks for
single files, Windows chezmoi copies single files, Neovim remains a directory
symlink, and Windows Terminal remains a merge.

| Tool | macOS | Linux / WSL | Windows |
|---|---|---|---|
| Neovim | `~/.config/nvim` -> `nvim/` | `~/.config/nvim` -> `nvim/` | `%LOCALAPPDATA%\nvim` -> `nvim\` |
| Starship | `~/.config/starship.toml` -> `starship/starship.toml` | same | `%USERPROFILE%\.config\starship.toml` -> `starship\starship.toml` |
| zsh | `~/.zshenv` -> `shells/zshenv`; `~/.zshrc` -> `shells/zshrc` | same | n/a |
| PowerShell | n/a | n/a | `Documents\PowerShell\Microsoft.PowerShell_profile.ps1` -> `shells\powershell_profile.ps1` |
| tmux / psmux | `~/.tmux.conf` -> `tmux/tmux.conf`; `~/.tmux.posix.conf` -> `tmux/tmux.posix.conf` (POSIX clipboard overlay) | same | `%USERPROFILE%\.tmux.conf` -> `tmux\tmux.conf` for psmux; the POSIX clipboard overlay is **excluded** on Windows (its `if-shell` probes hang psmux); WSL uses the Unix path |
| Ghostty | `~/Library/Application Support/com.mitchellh.ghostty/config` -> `ghostty/config` | native Linux links `~/.config/ghostty/config`; WSL links it only with `--experimental-wsl-gui` | n/a |
| lazygit | `~/Library/Application Support/lazygit/config.yml` -> `lazygit/config.yml` | `~/.config/lazygit/config.yml` -> `lazygit/config.yml` | `%LOCALAPPDATA%\lazygit\config.yml` -> `lazygit\config.yml` |
| Windows Terminal | n/a | n/a | app installed by `setup.ps1` through Scoop/winget/choco, with a SHA-256-verified portable zip fallback; setup backs up existing packaged `settings.json`, then chezmoi merges `windows-terminal/settings.fragment.jsonc` by default; after apply, setup mirrors packaged settings to the unpackaged WT path or seeds/merges that path from the fragment when packaged WT is absent; opt out with `-SkipWindowsTerminalMerge`; see [windows-terminal/README.md](windows-terminal/README.md) |

Chezmoi manages the Windows PowerShell 7 profile path
`Documents\PowerShell\Microsoft.PowerShell_profile.ps1`. The Windows
PowerShell 5.1 profile path (`Documents\WindowsPowerShell\...`) and POSIX pwsh
profile are host/provisioning-adjacent, because they depend on the host shell
and whether `pwsh` is installed.

### Platform Notes

- Linux setup can make zsh your login shell. Installing the package alone is
  not enough: tmux and new terminals keep launching bash until `chsh` or the
  domain-account fallback is accepted. The prompt is consent-gated and auto-yes
  under `--all`.
- `install-deps` provisions chezmoi itself: Homebrew on macOS/Linuxbrew,
  pinned `get.chezmoi.io` release on native Linux without brew, and the
  Scoop-first catalog on Windows.
- `install-deps` prints a dependency pre-flight table before package-manager
  bootstrap and before the one-shot install prompt, showing the package manager
  itself, present/missing tools, best-effort versions, and the resulting
  skip/install action. The table is informational; the existing per-tool install
  logic still decides what actually runs.
- zsh plugins are installed by Unix setup as repo-managed pinned git checkouts:
  `zsh-autocomplete` and `zsh-autosuggestions` live under
  `${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins`. `zshrc` sources
  those copies first and falls back to Homebrew/system paths only when the
  managed copy is missing. `zsh-autocomplete` loads before local `compinit`;
  `zsh-autosuggestions` loads after completion setup.
- Linux without Homebrew gets Neovim from a pinned official GitHub release
  tarball installed into `/opt/nvim-linux-<arch>` and symlinked to
  `/usr/local/bin/nvim`. The tarball SHA-256 is verified before extraction.
- macOS and Linuxbrew install lazygit through Homebrew (`brew install lazygit`).
  Native Linux/WSL without Homebrew gets lazygit from a pinned GitHub release
  tarball with SHA-256 verification. Setup installs it to
  `/usr/local/bin/lazygit`, or falls back to `~/.local/bin/lazygit` when sudo is
  unavailable.
- macOS installs Ghostty through `brew install --cask ghostty` when selected.
  Native Linux uses Linux-specific Ghostty paths. WSL defaults to Windows
  Terminal on the Windows host; Linux Ghostty in WSL requires
  `--experimental-wsl-gui`.
- WSL fonts are host-rendered in the supported path. Install and merge Windows
  Terminal from Windows (`.\setup.ps1 -All`; the merge is default-on); the WSL
  Linux fontconfig install is only for `--experimental-wsl-gui`.
- VS Code is optional. On WSL, use Windows VS Code plus `code .` for Remote -
  WSL, or use a Linux GUI build when WSLg / X11 is available. Rosé Pine setup
  follows whatever `code` CLI is on PATH.
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
make test               # run everything that can run on this OS
make validate-renovate  # schema-check renovate.json under Renovate's Node 24
make lint               # shellcheck everything
./tests/wsl/e2e.sh      # manual WSL split-host validation from inside WSL
./tests/greenfield/docker-greenfield.sh # local clean Ubuntu container e2e
```

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
Plenary's default timeout.

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

Pull requests are meant to be gated by two workflows:

- `.github/workflows/test.yml` runs the static, shell, tmux,
  starship, Neovim, Windows Pester/PSScriptAnalyzer, and `chezmoi-parity`
  suites. Warnings are treated as failures where the tools expose them cleanly:
  shellcheck exits nonzero, PSScriptAnalyzer runs at `Warning,Error`, and YAML
  parsing/linting is part of `make test-static`.
- `.github/workflows/e2e-install.yml` is the real install guarantee. It proves
  the public setup paths on fresh hosted runners and keeps one clean Ubuntu
  container for the native `apt` branch.

The e2e jobs cover different install paths, not symmetric container platforms:

| Check | What it proves |
|---|---|
| `e2e containers / ubuntu-24.04` | Clean `ubuntu:24.04`, non-root user, native `apt`, no Linuxbrew (`DOTFILES_SKIP_BREW_BOOTSTRAP=1`), then `install-deps.sh --all`, chezmoi config apply, tool assertions, Neovim >= 0.12, lazygit, zsh plugin files, config content assertions, and nvim directory realpath assertion. |
| `setup.sh / ubuntu-24.04` | Full Unix setup on the hosted Ubuntu runner. This runner has Linuxbrew available, so it proves the Linuxbrew path that users may hit. |
| `setup.sh / macos-15` | Full macOS setup through the real macOS hosted runner and Homebrew path. Docker cannot model macOS. |
| `setup.ps1 / windows-2025` | Full Windows setup through the real Windows hosted runner, including Scoop/winget/choco behavior, PowerShell, symlinks, and Neovim sync. Windows containers do not model the desktop/user-profile setup well. |
| `setup.sh / WSL2 Ubuntu-24.04 (best-effort canary)` | Non-required WSL smoke signal. Hosted runners cannot provide reliable nested virtualization, so this is intentionally best-effort. |

The Ubuntu container is intentionally **not** a devcontainer. It stays because
the hosted Ubuntu runner can take the Linuxbrew path, while the container is the
only automated proof of the clean-image native `apt` path: the pinned Neovim
tarball install, pinned lazygit release install, zsh plugin install, `fd-find`
-> `fd` shim, and apt fallback behavior. There is no matching macOS or Windows
container to add for symmetry. That asymmetry is accepted: hosted macOS and
Windows runners are the closest representative fixtures for those operating
systems, while the required WSL proxy is the Ubuntu container plus the
WSL config-template coverage. Do not add the WSL2 canary to
required checks unless the owner explicitly accepts the flake risk.

These e2e jobs fail if setup skips Phase 3-4, emits a precise `FAIL:` marker,
installs Neovim below 0.12, Lazy/Mason headless sync exits nonzero, or expected
Mason-installed binaries are missing. They do not blanket-fail on benign
warning/deprecation text.

## Repository Safeguards

The canonical main-branch safeguards are the three checked-in repository rulesets
under `.github/rulesets/`, applied live with
`scripts/apply-repo-safeguards.sh`. `.github/settings.yml` remains a classic
branch-protection fallback for the Probot Settings app, but it cannot model the
key policy split: owner review bypass is allowed, CI bypass is not, and only the
owner may update `main`.

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

Manual owner step:

```bash
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

with an authenticated `gh` that has repository admin permission. Do this after
the required checks have appeared at least once on GitHub, otherwise protection
may reference check names GitHub has not seen yet. See
[docs/security/branch-protection.md](docs/security/branch-protection.md) for
the live verification commands and deletion-risk note.

Renovate is the version-update bot for GitHub Actions and repo-pinned
version/ref constants. GitHub-native Dependabot security alerts and automated
security fixes stay enabled through `.github/settings.yml`; Dependabot version
update PRs are intentionally not configured.

| Surface | Renovate policy | Reason |
|---|---|---|
| GitHub Actions | Managed, digest-pinned, labeled `github-actions` | Actions are repo-owned CI inputs with stable Renovate support. |
| GitHub runner images | Managed, labeled `github-runners`, reviewed separately | `ubuntu-*`, `macos-*`, and `windows-*` bumps can change the supported CI platform, so they should not be mixed with ordinary Action bumps. |
| Repo-pinned installer versions/refs | Managed, labeled `pinned-downloads`, never automerged | Neovim Linux tarballs, lazygit Linux tarballs, Hack Nerd Font, Windows Terminal portable zip, Ubuntu Ghostty, zsh plugin refs, and the CI `cargo-binstall` commit are explicit repo pins. |
| Adjacent SHA-256 / commit constants | Not managed; matched only as regex context | Renovate can bump the version/ref but cannot recompute archive/script hashes or verify tag commit IDs. CI must fail until a human recomputes and reviews them. |
| Package-manager catalogs | Not managed | Brew, apt, dnf, pacman, zypper, apk, Scoop, winget, and choco entries are package names/IDs, not repo version pins. Let the package manager resolve current versions. |
| Neovim plugin and Mason tools | Not managed | `lazy-lock.json` is refreshed with Lazy and tested as editor behavior; Mason intentionally has no machine-pinned lockfile. |

Direct-download SHA-256 values for Neovim tarballs, lazygit tarballs, Hack Nerd
Font, the Windows Terminal portable zip, the Ubuntu Ghostty installer, and the
CI `cargo-binstall` installer script are intentionally human-reviewed. zsh
plugin tag commits are also human-reviewed because the installer verifies the
checked-out commit after cloning the bumped tag. Do not capture direct-download
SHA constants as Renovate `currentDigest` values: that creates
noisy/unresolvable digest updates instead of a trustworthy checksum review. A
Renovate PR may bump the version/ref while leaving the adjacent SHA or commit
stale; CI then fails verification until a human reviews the adjacent constant.

## Repo layout

```
.
├── nvim/                  # Neovim — init.lua, lua/{vim-options,util,plugins}
├── starship/              # starship.toml (Rose Pine)
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
- **Rose Pine everywhere it can render.** Nvim, lualine, starship, tmux,
  ghostty, Windows Terminal, PSReadLine — same palette across the stack. VS Code
  joins optionally: `install-deps` offers VS Code, and if `code` is detected it
  installs the `mvllow.rose-pine` theme, sets `workbench.colorTheme`, and sets
  VS Code editor/terminal font families to Hack Nerd Font fallbacks. Existing
  JSONC settings are edited in place with comments preserved and a backup first.
- **conform.nvim is the only format-on-save handler.** Replacing the
  prior LSP-attach autocmd + null-ls duo eliminates a real race condition
  with different timeouts. `:WNF` (or `:wnf`) skips formatting for one save.
- **Mason installs LSP servers + formatters via mason-tool-installer.** No
  `mason-lspconfig` — redundant on nvim 0.11 with `vim.lsp.enable`.
- **DAP launches stay generic.** The shared browser launch defaults to
  `http://localhost:3000`; set `DAP_LAUNCH_URL` or put project-specific launch
  configs in workspace `.nvim.lua` files.
- **Starship language modules pared down.** Only `c, go, node, rust, python,
  conda` are enabled. Disabled languages don't spawn version probes on every
  prompt.
- **Zsh starship init is precompiled** (mirroring the PowerShell profile
  approach) — re-generated only when `starship.toml` is newer than the cache.
- **zsh plugin installs are repo-managed pins.** `zsh-autocomplete` and
  `zsh-autosuggestions` are installed during Unix setup, sourced from XDG data
  before package-manager fallbacks, and verified against tag commit IDs so a
  Renovate ref bump still gets human review.
- **fzf wired into zsh by default** — `Ctrl-R` fuzzy history, `Ctrl-T` file
  picker, `Alt-C` fuzzy cd. It *complements* zsh-autosuggestions (inline
  ghosting), it doesn't replace it. Guarded by `command -v fzf` so a machine
  without it still starts cleanly; uses `fzf --zsh` with a share-dir fallback
  for older distro builds.
- **No `bindkey '\e' kill-whole-line`** — that shadowed the entire Meta
  prefix and silently broke Alt-h/j/k/l window nav in nvim.
- **tmux window swaps use uppercase Vim directions.** `prefix+H` swaps the
  current window left and `prefix+L` swaps it right. Lowercase `h/l` remain pane
  focus bindings, and arrow keys are left alone for terminal/psmux reliability.
- **WSL is split-host by default.** Windows Terminal renders fonts and window UI
  on the Windows side; WSL installs the Linux CLI/editor stack. Linux Ghostty
  and Linux fontconfig fonts in WSL require `--experimental-wsl-gui`.
- **Windows Terminal is a Windows dependency, not just a config target.**
  `install-deps.ps1` installs `wt` through the normal Scoop-first catalog:
  `extras/windows-terminal` -> `Microsoft.WindowsTerminal` -> `microsoft-windows-terminal`.
  If those MSIX-backed installers do not make `wt` available, it falls back to
  the pinned portable Windows Terminal GitHub release zip, verifies SHA-256
  before extraction, and adds the portable folder to the current and User PATH.
  setup's chezmoi phase then merges the repo-owned visual/keybinding fragment
  by default; pass `-SkipWindowsTerminalMerge` to opt out.
  `-MergeWindowsTerminal` remains accepted as a no-op alias for older commands.
- **Windows Terminal settings.json is NOT symlinked** because WT auto-rewrites
  it. Only the user-owned keys live in `settings.fragment.jsonc`; the install
  script's config phase backs up an existing pre-merge file to
  `settings.json.bak.<timestamp>`, then the chezmoi `modify_` merge updates keys
  by name and resets a hand-edited `theme` back to `rose-pine` on every run
  unless you opt out. After a real setup apply, the merged MSIX settings file is
  best-effort copied to `%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json`
  for portable WT; if the MSIX settings file is absent but portable WT is
  detected, setup seeds or merges that unpackaged file directly from the
  fragment so Rose Pine and Hack Nerd Font are present before first launch. A
  bare `chezmoi apply` runs the packaged merge but does not create setup's
  backup or the portable seed/mirror.
- **Windows CI installs Scoop through its documented elevated path.** GitHub
  Windows runners are elevated, and Scoop blocks elevated bootstrap by default.
  `install-deps.ps1` detects elevation and runs the official installer with
  `-RunAsAdmin`, then refreshes the current-process Scoop shims path so later
  installs can use `scoop` immediately. Existing Scoop installs also get the
  `extras` and `nerd-fonts` buckets normalized before catalog installs.
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

### Adding a plugin

Drop one file at `nvim/lua/plugins/<name>.lua` returning the lazy spec.
Lazy auto-discovers it. Default to lazy-loading (`event` / `cmd` / `keys` /
`ft`). Only `rose-pine` may set `lazy = false`.

### Adding an LSP server / formatter / treesitter parser

See `CLAUDE.md` -> "Common workflows". LSP servers and formatters update the
plugin spec, Mason `ensure_installed`, and the matching spec. Treesitter parsers
use `nvim-treesitter` main's `treesitter_parsers` list plus a filetype alias
when Neovim's filetype differs from the parser name.

### Refreshing the plugin lockfile

```bash
nvim --headless "+Lazy! sync" +qa
git add nvim/lazy-lock.json   # tracked, not gitignored
```

### Updating Mason tools across machines

```bash
nvim --headless "+MasonToolsUpdate" +qa
```

Run on each machine; there's no machine-pinned lockfile for Mason itself.

### Switching machines mid-session

```bash
git -C ~/dotfiles pull          # sync configs
nvim --headless "+Lazy! sync" +qa     # match plugin commits
make test                       # verify the new state
```

## License

MIT. See `LICENSE`.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `<leader>X` keymaps fire `\X` instead of `<Space>X` | mapleader set after lazy.setup somehow | restore the order in `nvim/init.lua` — leader **before** `require("lazy").setup` |
| Formatter runs twice or shows two BufWritePre autocmds | someone added a second handler outside conform.nvim | `:lua print(#vim.api.nvim_get_autocmds({event="BufWritePre"}))` should be 1; if not, find the second autocmd and delete it |
| Lazy/Mason says `No C compiler found` | WSL/Linux has `make` but no `cc`/`gcc`/`clang`; some plugin builds compile native code | re-run `./setup.sh --skip-config` to install the Linux compiler toolchain, or on Ubuntu run `sudo apt-get update && sudo apt-get install -y build-essential`, then `./setup.sh --skip-deps --skip-config` |
| Clipboard not crossing host on WSL | `win32yank.exe` not on PATH | install win32yank via scoop on Windows side, ensure WSL PATH picks it up |
| Starship prompt missing in the PowerShell window you ran setup in (but it works in psmux / a new window) | that shell loaded `$PROFILE` **before** setup put starship on PATH; the profile skips starship when `Get-Command starship` finds nothing | open a **new** PowerShell window, or run `. $PROFILE` in the current one — newly-installed tools are not on an already-open shell's PATH |
| Starship prompt slow | a disabled language got re-enabled | check `starship/starship.toml` — only `c, go, nodejs, rust, python, conda` should be enabled |
| Starship shows only the last few folders (or a leading `…/`) | the `[directory]` module was truncating the path | `starship/starship.toml` sets `truncation_length = 0` + `truncate_to_repo = false` for the full path; raise the length or set `truncate_to_repo = true` to shorten again |
| A folder like `Downloads`/`Music`/`Pictures` shows as a blank `~/` | its `[directory.substitutions]` glyph was stripped to a bare space | values are `icon + name` (e.g. `Downloads = "<nerd-font-glyph> Downloads"`) using a codepoint your font has; `tests/starship/directory_test.sh` fails on a whitespace-only value |
| `Alt-h/j/k/l` window nav doesn't work in terminal | something rebinds bare Esc in the shell | `bindkey | grep '^"\^\['` in zsh — should NOT show `kill-whole-line` |
| tmux (or any new terminal) launches **bash on Linux**, not zsh | the login shell was never changed — `~/.zshrc` is symlinked but the account still logs into bash | re-run `./setup.sh` and accept "Make zsh your default login shell?", or `chsh -s "$(command -v zsh)"` then log out/in. macOS already defaults to zsh |
| `chsh` fails with `user '<name>' does not exist in /etc/passwd` | you log in via a **domain** account (AD/LDAP/SSSD) that isn't in local `/etc/passwd`, so `chsh` can't touch it | re-run `./setup.sh` — it detects this and offers to re-exec interactive bash into zsh via `~/.bashrc` instead. The "proper" fix is admin-side: set the directory `loginShell` / SSSD `default_shell` |
| Move commits in lazygit, including inside psmux | Ctrl+J collides with Enter on the wire, and psmux v3.3.4 does not relay Windows Terminal's Win32-input-mode modifier data into panes | use uppercase `J` / `K`. `%LOCALAPPDATA%\lazygit\config.yml` binds commits-panel moveDownCommit / moveUpCommit to printable J/K, so no psmux root bind is needed. In the commits panel, use PgUp/PgDn or Ctrl-U/Ctrl-D to scroll the diff |
| Ghostty doesn't open maximized | `window-save-state = always` restored an old geometry over `maximize` (macOS only) | `ghostty/config` uses `window-save-state = default` (not `always`) with `maximize = true`; `always` lets the saved size win |
| Ghostty doesn't load the config | wrong path, or WSL default skip | the install path is `~/Library/Application Support/com.mitchellh.ghostty/config` on macOS and `~/.config/ghostty/config` on native Linux. WSL only links Linux Ghostty config after `./setup.sh --experimental-wsl-gui`; otherwise use Windows Terminal |
| Windows Terminal lost a profile after merge | WT auto-rewrites — pre-merge backup is at `<settings.json>.bak.<timestamp>` | restore the profile list from the backup |
| `setup.ps1` errors "cannot create symbolic links" | Developer Mode off and not elevated | `setup.ps1` reports your *elevated* + *Developer Mode* state before chezmoi apply. Enable Developer Mode (Settings -> Privacy & security -> For developers, no admin, recommended) **then** `.\setup.ps1 -SkipDeps`; OR run just the config phase elevated with `.\setup.ps1 -SkipDeps -SkipNvim`, then return to a normal shell for `.\setup.ps1 -SkipDeps -SkipConfig`. Don't elevate the dependency-install run because Scoop refuses admin installs |
| Ghostty won't open maximized on Linux/GNOME | `maximize = true` is a hint the WM may ignore (GNOME Mutter often does) | on **X11**, `install-deps` offers a devilspie2 setup through the native Linux package manager, even when Linuxbrew is the main CLI manager; the rule is keyed on `com.mitchellh.ghostty`. Wayland needs a GNOME Shell extension instead |
| `install-deps.ps1`: winget `No package found matching input criteria` (exit `-1978335212`) | winget source/catalog flakiness | install-deps now **prefers scoop** and falls back across managers per tool -- accept the scoop bootstrap when offered and re-run; scoop carries the cataloged CLI/terminal tools here |
