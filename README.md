# Dotfiles

Cross-platform terminal and editor setup for macOS, Linux, WSL, and Windows.
The repo owns the daily shell/editor stack: Neovim, tmux/psmux, Starship, zsh,
PowerShell, Ghostty, lazygit, Windows Terminal theming, locked plugin restore,
and LSP / formatter provisioning. It also provisions the `tree-sitter` CLI
needed by `nvim-treesitter` main parser builds.

The public interface is intentionally small:

```text
run setup -> install programs -> link configs -> restore Neovim plugins -> sync Mason tools
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

Clone the repo first, then run the local setup entry point. `setup.{sh,ps1}`
installs repo-managed dependencies, links every config, then runs
`:Lazy! restore`, a synchronous nvim-treesitter parser install, and
`:MasonToolsInstallSync` before the first interactive Neovim launch.

Git is the only hard prerequisite for remote setup because setup needs it to
clone this repo. If Git is missing, the setup scripts print the first install
command for the platform package manager.

```bash
# mac / linux / wsl
git clone https://github.com/luisgui1757/dotfiles.git ~/dotfiles
cd ~/dotfiles
./setup.sh --all
```

```powershell
# windows -- enable Developer Mode, then run from a normal PowerShell
# Settings -> Privacy & security -> For developers -> Developer Mode = On
git clone https://github.com/luisgui1757/dotfiles.git $HOME\dotfiles
Set-Location $HOME\dotfiles
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
state/config alone. Dry-run mode prints the planned removals without deleting
files or pruning empty external parent directories. Windows Terminal
`settings.json` is never deleted: the merge is idempotent but not invertible, so
restore manually from the printed backup if you want to undo it.

## What Setup Does

`setup` is a five-phase, idempotent orchestrator:

```text
setup -> install-deps                 phase 1: programs and optional tools
      -> chezmoi apply                phase 2: config layer with pre-apply backups
      -> nvim "+Lazy! restore" +qa    phase 3: plugins from lazy-lock.json
      -> nvim +DOTFILES_TREESITTER_SYNC_INSTALL  phase 4: Tree-sitter parsers
      -> nvim "+MasonToolsInstallSync" +qa        phase 5: LSP servers and formatters
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
`chezmoi apply`, `:Lazy restore`, synchronous Tree-sitter parser bootstrap,
`:Lazy sync`, and `:Lazy update`; the last two can change `lazy-lock.json` and
are therefore repo updates, not machine refreshes.

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
| lazygit | `~/Library/Application Support/lazygit/config.yml` -> `lazygit/config.yml` | `~/.config/lazygit/config.yml` -> `lazygit/config.yml` | `%LOCALAPPDATA%\lazygit\config.yml` -> `lazygit\config.windows.yml` |
| Windows Terminal | n/a | n/a | app installed by `setup.ps1` through Scoop/winget/choco, with a SHA-256-verified portable zip fallback; setup backs up existing packaged `settings.json`, then chezmoi merges `windows-terminal/settings.fragment.jsonc` by default, including a fixed PowerShell 7 profile used when WT is unset or still defaulting to Windows PowerShell 5.1; after apply, setup mirrors packaged settings to the unpackaged WT path or seeds/merges that path from the fragment when packaged WT is absent; opt out with `-SkipWindowsTerminalMerge`; see [windows-terminal/README.md](windows-terminal/README.md) |

Chezmoi manages the Windows PowerShell 7 profile path
`Documents\PowerShell\Microsoft.PowerShell_profile.ps1`. The Windows
PowerShell 5.1 profile path (`Documents\WindowsPowerShell\...`) and POSIX pwsh
profile are host/provisioning-adjacent, because they depend on the host shell
and whether `pwsh` is installed.

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
- `install-deps` provisions `lsd` through the supported package managers
  (Homebrew, native Linux package managers where available, and the Windows
  Scoop-first catalog). Interactive shells replace `ls` with `lsd` and add the
  useful `l`, `la`, `lla`, and `lt` shortcuts only when the binary is present.
- `install-deps` provisions the `cmake` CLI because the configured CMake LSP
  (`neocmakelsp`) shells out to it; Mason installs the language server, not the
  project toolchain it drives.
- `install-deps` provisions the `tree-sitter` CLI for `nvim-treesitter` main:
  Homebrew on macOS/Linuxbrew, a pinned SHA-256-verified GitHub release into
  `~/.local/bin` on native Linux/WSL, and Scoop with npm fallback on Windows.
  Windows `-All` also installs VS Build Tools so parser builds can find MSVC;
  after winget/choco failures it falls back to Microsoft's official
  `vs_BuildTools.exe` bootstrapper with the same VCTools workload.
- `install-deps` prints a dependency pre-flight table before package-manager
  bootstrap and before the one-shot install prompt, showing the package manager
  itself, present/missing tools, best-effort versions, and the resulting
  skip/install action. The table is informational; the existing per-tool install
  logic still decides what actually runs.
- zsh plugins are installed by Unix setup as repo-managed pinned git checkouts:
  `fzf-tab` and `zsh-autosuggestions` live under
  `~/.local/share/dotfiles/zsh-plugins`. `zshrc` sources those copies first and
  falls back to Homebrew/system paths only when the managed copy is missing.
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
make ci                 # full local pre-PR gate: test + Renovate + migration
make test               # current-host fast suite
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
  starship, Neovim, Windows Pester/PSScriptAnalyzer, Renovate schema, and
  `chezmoi-parity` suites. Warnings are treated as failures where the tools
  expose them cleanly: shellcheck exits nonzero, PSScriptAnalyzer runs at
  `Warning,Error`, Renovate validation fails if `npx` is missing under CI, and
  YAML parsing/linting is part of `make test-static`. Windows PSGallery module
  installs retry transient lookup failures, but missing test dependencies remain
  fatal.
- `.github/workflows/e2e-install.yml` is the real install guarantee. It proves
  the public setup paths on fresh hosted runners and keeps one clean Ubuntu
  container for the native `apt` branch.

The e2e jobs cover different install paths, not symmetric container platforms:

| Check | What it proves |
|---|---|
| `e2e containers / ubuntu-24.04` | Clean `ubuntu:24.04`, non-root user, native `apt`, no Linuxbrew (`DOTFILES_SKIP_BREW_BOOTSTRAP=1`), then `install-deps.sh --all`, chezmoi config apply, tool assertions, Neovim >= 0.12, lazygit, zsh plugin files, config content assertions, and nvim directory realpath assertion. |
| `setup.sh / ubuntu-24.04` | Full Unix setup on the hosted Ubuntu runner. This runner has Linuxbrew available, so it proves the Linuxbrew path that users may hit. |
| `setup.sh / macos-15` | Full macOS setup through the real macOS hosted runner and Homebrew path. Docker cannot model macOS. |
| `setup.ps1 / windows-2025` | Full Windows setup through the real Windows hosted runner, including Scoop/winget/choco behavior, PowerShell, symlinks, and Neovim restore/sync phases. Windows containers do not model the desktop/user-profile setup well. |
| `setup.sh / WSL2 Ubuntu-24.04 (best-effort canary)` | Non-required WSL smoke signal. Hosted runners cannot provide reliable nested virtualization, so this is intentionally best-effort. |

After the Lazy restore, Tree-sitter parser install, and Mason sync, each
`setup.sh`/`setup.ps1` job also
runs the **Tier 2 language smoke** (`tests/nvim/lsp_smoke.lua`): against the
real Neovim config it asserts (0) no nvim-treesitter parser override for a
bundled language remains on the runtimepath under `stdpath('data')`, (1) every
installed treesitter parser is one nvim-treesitter `main` supports and no
unexpected nvim-treesitter install-output parser `.so` is present under
`stdpath('data')/site/parser` beyond the explicit list plus upstream dependency
parsers, (2) synchronous parser bootstrap
completes through nvim-treesitter's waitable install task and returns exactly
`true`, (3) every language-matrix fixture opens with the expected filetype and
every parser-backed row reports real Tree-sitter highlight captures, (4) each
language's LSP attaches (`powershell_es` enforced on Windows only), and (5) the
auto-started bundled filetypes keep nvim-treesitter's `indentexpr`. The
fast `make test-nvim` runs Tier 1 (filetype + formatter + parser-list
consistency per fixture). Adding a language is "drop a fixture + a row in
`tests/nvim/language_matrix.lua`"; syntax-only fallbacks such as `.curlrc`
belong in the same matrix, must not pretend to have unsupported parsers, and
Tier 2 syntax probes must prove they still produce real Vim syntax groups. The
smoke matrix also encodes the
Neovim-bundled languages (`c`, `lua`, `markdown`, `query`, `vim`): those must
stay **out** of the install list — and any stale override of them is purged on
config load (scoped to `stdpath('data')` so Neovim's own install-prefix parsers
are never touched) — so Neovim's matched built-in parser+query is used instead
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
WSL config-template coverage. Do not add the WSL2 canary to
required checks unless the owner explicitly accepts the flake risk.

These e2e jobs fail if setup skips Phase 3-5, emits a precise `FAIL:` marker,
installs Neovim below 0.12, Lazy restore / Tree-sitter parser install / Mason
sync exits nonzero, or expected Mason-installed binaries are missing. They do
not blanket-fail on benign warning/deprecation text.

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
| Repo-pinned installer versions/refs | Managed, labeled `pinned-downloads`, never automerged | Neovim Linux tarballs, chezmoi CI release archives, lazygit Linux tarballs, Starship Linux tarballs, tree-sitter CLI Linux archives, Hack Nerd Font, Windows Terminal portable zip, Ubuntu Ghostty, zsh plugin refs, the Homebrew installer commit, the Renovate validator package/runtime, and the CI `cargo-binstall` commit are explicit repo pins. |
| Adjacent SHA-256 / commit constants | Not managed; matched only as regex context | Renovate can bump the version/ref but cannot recompute archive/script hashes or verify tag commit IDs. CI must fail until a human recomputes and reviews them. |
| Package-manager catalogs | Not managed | Brew, apt, dnf, pacman, zypper, apk, Scoop, winget, and choco entries are package names/IDs, not repo version pins. Let the package manager resolve current versions. |
| Neovim plugin and Mason tools | Not managed | `lazy-lock.json` is refreshed with Lazy and tested as editor behavior; Mason intentionally has no machine-pinned lockfile. |

Direct network executables must either be pinned and verified before execution
or appear in the reviewed static allowlist with a rationale. The remaining
package-manager bootstrap trust root is Scoop's official installer on Windows;
it is consent-gated and guarded by
`tests/static/supply_chain_remote_execution_test.sh`. Homebrew bootstrap is
downloaded from a pinned installer commit and SHA-256 verified before
execution. Recommended setup docs use `git clone` plus local `setup`, not raw
`curl | bash`/`iwr` execution of the current default branch.

Direct-download SHA-256 values for Neovim tarballs, chezmoi CI release archives,
lazygit tarballs, Starship tarballs, tree-sitter CLI archives, Hack Nerd Font,
the Windows Terminal portable zip, the Ubuntu Ghostty installer, Homebrew
installer script, and the CI `cargo-binstall` installer script are
intentionally human-reviewed. zsh plugin
tag commits are also human-reviewed because the installer verifies the
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
  installs the `mvllow.rose-pine` theme, sets `workbench.colorTheme` (plus the
  `preferredDark`/`preferredLight` slots and `window.autoDetectColorScheme:false`
  so dark Rose Pine is forced regardless of OS scheme), and sets VS Code
  editor/terminal font families to Hack Nerd Font fallbacks. Existing JSONC
  settings are edited in place with comments preserved and a backup first.
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
- **zsh plugin installs are repo-managed pins.** `fzf-tab` and
  `zsh-autosuggestions` are installed during Unix setup, sourced from XDG data
  before package-manager fallbacks, and verified against tag commit IDs so a
  Renovate ref bump still gets human review.
- **fzf wired into zsh by default** — `Ctrl-R` fuzzy history, `Ctrl-T` file
  picker, `Alt-C` fuzzy cd. It *complements* zsh-autosuggestions (inline
  ghosting), it doesn't replace it. Guarded by `command -v fzf` so a machine
  without it still starts cleanly; uses `fzf --zsh` with a share-dir fallback
  for older distro builds.
- **lsd wired into interactive shells** — setup installs it where the platform
  package manager carries it, then zsh and PowerShell expose the documented
  `ls`, `l`, `la`, `lla`, and `lt` commands when `lsd` is present.
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
  by name, adds a fixed `PowerShell 7` profile (`pwsh.exe`), promotes an empty or
  Windows PowerShell 5.1 `defaultProfile` to that profile, and resets a
  hand-edited `theme` back to `rose-pine` on every run unless you opt out. A
  custom `defaultProfile` is preserved. After a real setup apply, the merged
  MSIX settings file is
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
diff.

### Updating Mason tools across machines

```bash
nvim --headless "+MasonToolsUpdate" +qa
```

Run on each machine; there's no machine-pinned lockfile for Mason itself.

### Switching machines mid-session

```bash
git -C ~/dotfiles pull          # sync configs
nvim --headless "+Lazy! restore" +qa  # match plugin commits
make test                       # verify the new state
```

## License

MIT. See `LICENSE`.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `<leader>X` keymaps fire `\X` instead of `<Space>X` | mapleader set after lazy.setup somehow | restore the order in `nvim/init.lua` — leader **before** `require("lazy").setup` |
| Formatter runs twice or shows two BufWritePre autocmds | someone added a second handler outside conform.nvim | `:lua print(#vim.api.nvim_get_autocmds({event="BufWritePre"}))` should be 1; if not, find the second autocmd and delete it |
| Lazy/Tree-sitter/Mason says `No C compiler found` | WSL/Linux has `make` but no `cc`/`gcc`/`clang`; Tree-sitter parsers and some plugin builds compile native code | re-run `./setup.sh --skip-config` to install the Linux compiler toolchain, or on Ubuntu run `sudo apt-get update && sudo apt-get install -y build-essential`, then `./setup.sh --skip-deps --skip-config` |
| nvim treesitter parsers fail to compile on Windows / `cl.exe` not found | `nvim-treesitter` main builds parsers with the Rust `cc` crate, which needs MSVC env vars | run `.\setup.ps1 -All` to install VS Build Tools and let setup import the VS DevShell before parser installation; for ad-hoc `:TSUpdate`, open a "Developer PowerShell for VS" or rerun setup |
| nvim syntax looks weak or files look plain text | Tree-sitter is inactive, or the hybrid built-in syntax fallback was not restored after Tree-sitter starts | update this repo, re-run setup, then check `:Inspect` on a token; parser-backed languages should show `treesitter` captures plus `syntax` groups, while `.bat` should show `syntax` groups |
| Clipboard not crossing host on WSL | `win32yank.exe` not on PATH | install win32yank via scoop on Windows side, ensure WSL PATH picks it up |
| Starship prompt missing in the PowerShell window you ran setup in (but it works in psmux / a new window) | that shell loaded `$PROFILE` **before** setup put starship on PATH; the profile skips starship when `Get-Command starship` finds nothing | open a **new** PowerShell window, or run `. $PROFILE` in the current one — newly-installed tools are not on an already-open shell's PATH |
| Starship init warns that `starship.ps1` is being used by another process | old checkout wrote the PowerShell init cache directly while several WT tabs or psmux panes started together | update this repo and reopen PowerShell; the profile now writes a temp file, moves it into place, and retries a short read lock |
| Starship prompt slow | a disabled language got re-enabled | check `starship/starship.toml` — only `c, go, nodejs, rust, python, conda` should be enabled |
| Starship shows only the last few folders (or a leading `…/`) | the `[directory]` module was truncating the path | `starship/starship.toml` sets `truncation_length = 0` + `truncate_to_repo = false` for the full path; raise the length or set `truncate_to_repo = true` to shorten again |
| A folder like `Downloads`/`Music`/`Pictures` shows as a blank `~/` | its `[directory.substitutions]` glyph was stripped to a bare space | values are `icon + name` (e.g. `Downloads = "<nerd-font-glyph> Downloads"`) using a codepoint your font has; `tests/starship/directory_test.sh` fails on a whitespace-only value |
| `Alt-h/j/k/l` window nav doesn't work in terminal | something rebinds bare Esc in the shell | `bindkey | grep '^"\^\['` in zsh — should NOT show `kill-whole-line` |
| `Esc` does nothing in lazygit inside psmux | psmux v3.3.x has an upstream bare-Escape forwarding bug | use `Ctrl-G` to close lazygit help/popups inside psmux. The native-Windows lazygit config binds `universal.return` to `<c-g>`, so pressing `?` in lazygit shows the working return/cancel key |
| tmux (or any new terminal) launches **bash on Linux**, not zsh | either the login shell was never changed, or an already-running graphical session kept stale `$SHELL=/bin/bash` after `chsh` | if the login shell is still bash, re-run current `./setup.sh` and accept the zsh adoption prompt. Local accounts get `chsh` plus an interactive bash guard so new terminals/tmux land in zsh without a full graphical relogin; manual `chsh` or older setup runs that already changed `/etc/passwd` still need relogin |
| `chsh` fails with `user '<name>' does not exist in /etc/passwd` | you log in via a **domain** account (AD/LDAP/SSSD) that isn't in local `/etc/passwd`, so `chsh` can't touch it | re-run `./setup.sh` — it detects this and offers to re-exec interactive bash into zsh via `~/.bashrc` instead. The "proper" fix is admin-side: set the directory `loginShell` / SSSD `default_shell` |
| Move commits in lazygit, including inside psmux | Ctrl+J collides with Enter on the wire, and psmux v3.3.4 does not relay Windows Terminal's Win32-input-mode modifier data into panes | use uppercase `J` / `K`. `%LOCALAPPDATA%\lazygit\config.yml` binds commits-panel moveDownCommit / moveUpCommit to printable J/K, so no psmux root bind is needed. In the commits panel, use PgUp/PgDn or Ctrl-U/Ctrl-D to scroll the diff |
| Windows Terminal opens Windows PowerShell 5.1 instead of PowerShell 7 | settings predate the managed WT default-profile merge, or the merge was skipped | re-run `.\setup.ps1 -SkipDeps -SkipNvim`; it adds the fixed `PowerShell 7` profile and promotes only an unset or legacy Windows PowerShell default, preserving a custom default |
| Want a fully solid (opaque) tmux/psmux status bar on Windows | Windows Terminal applies `opacity` window-wide to every cell, so a transparent WT (`opacity < 100`) has a transparent bar regardless of the bar's bg color — a distinct bg does NOT make it opaque in WT | the repo defaults to `opacity: 95` (see-through terminal). For a solid bar set WT `opacity: 100` in the fragment / `settings.json` (whole window opaque). macOS/Linux Ghostty get an opaque-looking bar from `background-opacity 0.95` + blur |
| PowerShell Tab completion — the selected option is **gold** | PSReadLine `Selection` colors the highlighted MenuComplete option | it is a gold foreground. Note: PSReadLine uses that same `Selection` color for the completion suffix it inserts into the command line while you navigate, so that suffix also shows gold until you accept — it is one setting, not separable |
| A `wt --version` window popped up during `setup.ps1 -All` | the dependency version table ran `<tool> --version`, and `wt --version` opens a Windows Terminal window instead of printing | fixed — `Get-CommandVersionString` never runs `wt --version`; it reads the file version (or shows `installed`) |
| Ghostty doesn't open maximized | `window-save-state = always` restored an old geometry over `maximize` (macOS only) | `ghostty/config` uses `window-save-state = default` (not `always`) with `maximize = true`; `always` lets the saved size win |
| Ghostty doesn't load the config | wrong path, or WSL default skip | the install path is `~/Library/Application Support/com.mitchellh.ghostty/config` on macOS and `~/.config/ghostty/config` on native Linux. WSL only links Linux Ghostty config after `./setup.sh --experimental-wsl-gui`; otherwise use Windows Terminal |
| Windows Terminal lost a profile after merge | WT auto-rewrites — pre-merge backup is at `<settings.json>.bak.<timestamp>` | restore the profile list from the backup |
| `setup.ps1` errors "cannot create symbolic links" | Developer Mode off and not elevated | `setup.ps1` reports your *elevated* + *Developer Mode* state before chezmoi apply. Enable Developer Mode (Settings -> Privacy & security -> For developers, no admin, recommended) **then** `.\setup.ps1 -SkipDeps`; OR run just the config phase elevated with `.\setup.ps1 -SkipDeps -SkipNvim`, then return to a normal shell for `.\setup.ps1 -SkipDeps -SkipConfig`. Don't elevate the dependency-install run because Scoop refuses admin installs |
| Ghostty won't open maximized on Linux/GNOME | `maximize = true` is a hint the WM may ignore (GNOME Mutter often does) | on **X11**, `install-deps` offers a devilspie2 setup through the native Linux package manager, even when Linuxbrew is the main CLI manager; the rule is keyed on `com.mitchellh.ghostty`. Wayland needs a GNOME Shell extension instead |
| `install-deps.ps1`: winget `No package found matching input criteria` (exit `-1978335212`) | winget source/catalog flakiness | install-deps now **prefers scoop** and falls back across managers per tool -- accept the scoop bootstrap when offered and re-run; VS Build Tools has no Scoop package, so it falls through to choco and then Microsoft's official bootstrapper |
