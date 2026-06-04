# Dotfiles

Cross-platform terminal and editor setup for macOS, Linux, WSL, and Windows.
The repo owns the daily shell/editor stack: Neovim, tmux/psmux, Starship, zsh,
PowerShell, Ghostty, lazygit, Windows Terminal theming, plugin sync, and LSP /
formatter provisioning.

The public interface is intentionally small:

```text
run setup -> install dependencies -> link configs -> sync Neovim plugins -> sync Mason tools
```

The lower-level scripts exist for debugging those phases. For a fresh machine,
or for a coworker trying this setup cold, run `setup`.

## Quick Start

No checkout is required. `setup.{sh,ps1}` clones the repo to `~/dotfiles`
or `%USERPROFILE%\dotfiles`, installs repo-managed dependencies, links every
config, then runs `:Lazy! sync` and `:MasonToolsInstallSync` before the first
interactive Neovim launch.

Git is the only hard prerequisite for remote bootstrap because setup needs it to
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

On Windows, prefer Developer Mode plus a normal PowerShell. If Developer Mode is
unavailable and you cannot enable it, run just `.\bootstrap.ps1` from an
elevated PowerShell, then return to a normal shell for
`.\setup.ps1 -SkipDeps -SkipBootstrap`. Do not elevate the whole `setup.ps1`
run; Scoop refuses admin installs.

## What Setup Does

`setup` is a four-phase, idempotent orchestrator:

```text
setup -> install-deps                 phase 1: packages and optional tools
      -> bootstrap                    phase 2: symlink or merge configs
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

Every script is safe to rerun. Pre-existing non-symlink targets are backed up to
`<target>.bak.<timestamp>` with collision-proof suffixes (`.1`, `.2`, ...).

### Existing Checkout

```bash
./setup.sh                       # Y/n per dep, end-to-end
./setup.sh --all                 # non-interactive
./setup.sh --dry-run             # preview
make setup                       # same as ./setup.sh, via the Makefile
```

```powershell
.\setup.ps1
.\setup.ps1 -All
.\setup.ps1 -DryRun
.\setup.ps1 -MergeWindowsTerminal     # also apply the WT rose-pine fragment
```

### Managed Configs

| Tool | macOS | Linux / WSL | Windows |
|---|---|---|---|
| Neovim | `~/.config/nvim` -> `nvim/` | `~/.config/nvim` -> `nvim/` | `%LOCALAPPDATA%\nvim` -> `nvim\` |
| Starship | `~/.config/starship.toml` -> `starship/starship.toml` | same | `%USERPROFILE%\.config\starship.toml` -> `starship\starship.toml` |
| zsh | `~/.zshrc` -> `shells/zshrc` | same | n/a |
| PowerShell | `$PROFILE` -> `shells/powershell_profile.ps1` when `pwsh` is installed | same | `$PROFILE` -> `shells\powershell_profile.ps1` |
| tmux / psmux | `~/.tmux.conf` -> `tmux/tmux.conf` | same | `%USERPROFILE%\.tmux.conf` -> `tmux\tmux.conf` for psmux; WSL uses the Unix path |
| Ghostty | `~/Library/Application Support/com.mitchellh.ghostty/config` -> `ghostty/config` | `~/.config/ghostty/config` -> `ghostty/config` when GUI-capable | n/a |
| lazygit | `~/Library/Application Support/lazygit/config.yml` -> `lazygit/config.yml` | `~/.config/lazygit/config.yml` -> `lazygit/config.yml` | `%LOCALAPPDATA%\lazygit\config.yml` -> `lazygit\config.yml` |
| Windows Terminal | n/a | n/a | merge `windows-terminal/settings.fragment.jsonc`; see [windows-terminal/README.md](windows-terminal/README.md) |

### Platform Notes

- Linux setup can make zsh your login shell. Installing the package alone is
  not enough: tmux and new terminals keep launching bash until `chsh` or the
  domain-account fallback is accepted. The prompt is consent-gated and auto-yes
  under `--all`.
- Linux without Homebrew gets Neovim from a pinned official GitHub release
  tarball installed into `/opt/nvim-linux-<arch>` and symlinked to
  `/usr/local/bin/nvim`. The tarball SHA-256 is verified before extraction.
- macOS installs Ghostty through `brew install --cask ghostty` when selected.
  Linux and GUI-capable WSL use Linux-specific Ghostty paths; the Homebrew
  Ghostty formula is macOS-only.
- VS Code is optional. On WSL, use Windows VS Code plus `code .` for Remote -
  WSL, or use a Linux GUI build when WSLg / X11 is available. Rosé Pine setup
  follows whatever `code` CLI is on PATH.
- Notes / Obsidian support writes `export NOTES_VAULT=...` to
  `~/.zshrc.local` (gitignored, sourced by `zshrc`). Non-interactive runs skip
  the prompt, so set `NOTES_VAULT` yourself there.
- macOS installs before this README may have an unused
  `~/.config/lazygit/config.yml` symlink. It is harmless; current bootstrap
  links the path lazygit actually reads:
  `~/Library/Application Support/lazygit/config.yml`.

## Test

Use the same top-level test command that CI uses for your OS:

```bash
# mac / linux / wsl
make help               # list targets
make test               # run everything that can run on this OS
make test-bootstrap     # bats coverage of the installer
make validate-renovate  # schema-check renovate.json under Renovate's Node 24
make lint               # shellcheck everything
```

```powershell
# windows
.\test.ps1          # PSScriptAnalyzer + Pester + Nvim plenary busted
```

Windows runs each Neovim Plenary spec file directly through `plenary.busted`;
do not switch it back to `PlenaryBustedDirectory`, whose parent harness can
false-fail after successful child specs under PowerShell native-command error
promotion.

Sub-targets skip themselves with a `skipped: <tool> not installed` message
when their dependency tool is missing on the current machine. In CI, missing
Windows test dependencies are fatal so the workflow cannot go green by silently
skipping the actual checks.

## CI Merge Gate

Pull requests are meant to be gated by two workflows:

- `.github/workflows/test.yml` runs the existing static, shell, bootstrap,
  tmux, starship, Neovim, and Windows Pester/PSScriptAnalyzer suites. Warnings
  are treated as failures where the tools expose them cleanly: shellcheck exits
  nonzero, PSScriptAnalyzer runs at `Warning,Error`, and YAML parsing/linting
  is part of `make test-static`.
- `.github/workflows/e2e-install.yml` is the real install guarantee. It proves
  the public setup paths on fresh hosted runners and keeps one clean Ubuntu
  container for the native `apt` branch.

The e2e jobs cover different install paths, not symmetric container platforms:

| Check | What it proves |
|---|---|
| `e2e containers / ubuntu-24.04` | Clean `ubuntu:24.04`, non-root user, native `apt`, no Linuxbrew (`DOTFILES_SKIP_BREW_BOOTSTRAP=1`), then `install-deps.sh --all`, `bootstrap.sh`, tool assertions, Neovim >= 0.11, and symlink assertions. |
| `setup.sh / ubuntu-24.04` | Full Unix setup on the hosted Ubuntu runner. This runner has Linuxbrew available, so it proves the Linuxbrew path that users may hit. |
| `setup.sh / macos-15` | Full macOS setup through the real macOS hosted runner and Homebrew path. Docker cannot model macOS. |
| `setup.ps1 / windows-2025` | Full Windows setup through the real Windows hosted runner, including Scoop/winget/choco behavior, PowerShell, symlinks, and Neovim sync. Windows containers do not model the desktop/user-profile setup well. |
| `setup.sh / WSL2 Ubuntu-24.04 (best-effort canary)` | Non-required WSL smoke signal. Hosted runners cannot provide reliable nested virtualization, so this is intentionally best-effort. |

The Ubuntu container is intentionally **not** a devcontainer. It stays because
the hosted Ubuntu runner can take the Linuxbrew path, while the container is the
only automated proof of the clean-image native `apt` path: the pinned Neovim
tarball install, `fd-find` -> `fd` shim, Ubuntu Ghostty installer path, and apt
fallback behavior. There is no matching macOS or Windows container to add for
symmetry. That asymmetry is accepted: hosted macOS and Windows runners are the
closest representative fixtures for those operating systems, while the required
WSL proxy is the Ubuntu container plus the `DOTFILES_FORCE_OS=wsl` bootstrap
coverage. Do not add the WSL2 canary to required checks unless the owner
explicitly accepts the flake risk.

These e2e jobs fail if setup skips Phase 3-4, emits a precise `FAIL:` marker,
installs Neovim below 0.11, Lazy/Mason headless sync exits nonzero, or expected
Mason-installed binaries are missing. They do not blanket-fail on benign
warning/deprecation text.

## Repository Safeguards

Repository safeguards are declared in `.github/settings.yml` for the Probot
Settings app and mirrored by `scripts/apply-repo-safeguards.sh` for a one-shot
`gh api` path. The intended main-branch policy is:

- only rebase-and-merge is allowed; merge commits and squash merges are disabled;
- required status checks include `ubuntu`, `macos`, `windows`, the required e2e
  container job, and the three full setup jobs;
- no PR reviews are required for the solo maintainer; the merge gate is required
  status checks plus enforced admins;
- conversation resolution and linear history are required, and force pushes /
  branch deletion are blocked;
- branches are deleted after merge; Renovate updates GitHub Actions and
  repo-pinned version/ref constants weekly.

Manual owner step: either install the Probot Settings app so `.github/settings.yml`
is continuously applied after it lands on `main`, or run:

```bash
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

with an authenticated `gh` that has repository admin permission. Do this only
after the required checks have appeared at least once on GitHub, otherwise branch
protection may reference check names GitHub has not seen yet.

Renovate is the version-update bot for GitHub Actions and repo-pinned
version/ref constants. GitHub-native Dependabot security alerts and automated
security fixes stay enabled through `.github/settings.yml`; Dependabot version
update PRs are intentionally not configured. Renovate cannot recompute the
SHA-256 values for Neovim tarballs, Hack Nerd Font, the Ubuntu Ghostty installer,
or the CI `cargo-binstall` installer script. The `github-releases` datasource
has no digest resolver for those archives, and the `git-refs` datasource only
bumps the cargo-binstall commit. Direct-download SHA-256 constants are therefore
matched only as context in `renovate.json`, not captured as Renovate
`currentDigest` values. A Renovate PR may bump the version/ref while leaving the
adjacent SHA stale; CI then fails checksum verification until a human recomputes
and reviews the SHA constants.

## Repo layout

```
.
├── nvim/                  # Neovim — init.lua, lua/{vim-options,util,plugins}
├── starship/              # starship.toml (Rose Pine)
├── shells/                # zshrc + powershell_profile.ps1
├── tmux/                  # tmux.conf (Rose Pine, vi-mode, true-color)
├── ghostty/               # config (Rose Pine, Hack Nerd, Ghostty-tuned)
├── windows-terminal/      # settings.fragment.jsonc + merge README
├── tests/                 # automated test tree
├── .github/workflows/     # CI matrix (ubuntu, macos, windows)
├── setup.sh               # public macOS/Linux/WSL entry point
├── setup.ps1              # public Windows entry point
├── bootstrap.sh           # setup phase: Unix symlinks
├── bootstrap.ps1          # setup phase: Windows symlinks
├── test.ps1               # Windows test entry point
├── Makefile               # Unix test/setup conveniences
├── AGENTS.md              # standard agent entry point, points to CLAUDE.md
├── CLAUDE.md              # canonical coding-agent operational guide
├── .editorconfig          # cross-IDE formatting rules
└── README.md              # human-facing install and usage guide
```

## Key design decisions (and why)

- **One source of truth via symlinks.** Edits in the repo are live everywhere
  without manual copy-paste; `bootstrap.{sh,ps1}` are idempotent.
- **Rose Pine everywhere it can render.** Nvim, lualine, starship, tmux,
  ghostty, Windows Terminal, PSReadLine — same palette across the stack. VS Code
  joins optionally: `install-deps` offers VS Code, and if `code` is detected it
  installs the `mvllow.rose-pine` theme and sets `workbench.colorTheme` (existing
  JSONC settings are left untouched).
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
- **fzf wired into zsh by default** — `Ctrl-R` fuzzy history, `Ctrl-T` file
  picker, `Alt-C` fuzzy cd. It *complements* zsh-autosuggestions (inline
  ghosting), it doesn't replace it. Guarded by `command -v fzf` so a machine
  without it still starts cleanly; uses `fzf --zsh` with a share-dir fallback
  for older distro builds.
- **No `bindkey '\e' kill-whole-line`** — that shadowed the entire Meta
  prefix and silently broke Alt-h/j/k/l window nav in nvim.
- **Windows Terminal settings.json is NOT symlinked** because WT auto-rewrites
  it. Only the user-owned keys live in `settings.fragment.jsonc`; the install
  script merges them in.
- **Windows CI installs Scoop through its documented elevated path.** GitHub
  Windows runners are elevated, and Scoop blocks elevated bootstrap by default.
  `install-deps.ps1` detects elevation and runs the official installer with
  `-RunAsAdmin`, then refreshes the current-process Scoop shims path so later
  installs can use `scoop` immediately. Bootstrap symlink creation still works
  through the elevated/native CreateSymbolicLink path. For local machines,
  Developer Mode plus a normal PowerShell remains the recommended setup path.

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

See `CLAUDE.md` -> "Common workflows". Three small edits each: the plugin
spec, the Mason ensure_installed list, and the corresponding test under
`tests/nvim/spec/`.

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
| Lazy/Mason says `No C compiler found` | WSL/Linux has `make` but no `cc`/`gcc`/`clang`; some plugin builds compile native code | re-run `./setup.sh --skip-bootstrap` to install the Linux compiler toolchain, or on Ubuntu run `sudo apt-get update && sudo apt-get install -y build-essential`, then `./setup.sh --skip-deps --skip-bootstrap` |
| Clipboard not crossing host on WSL | `win32yank.exe` not on PATH | install win32yank via scoop on Windows side, ensure WSL PATH picks it up |
| Starship prompt slow | a disabled language got re-enabled | check `starship/starship.toml` — only `c, go, nodejs, rust, python, conda` should be enabled |
| Starship shows only the last few folders (or a leading `…/`) | the `[directory]` module was truncating the path | `starship/starship.toml` sets `truncation_length = 0` + `truncate_to_repo = false` for the full path; raise the length or set `truncate_to_repo = true` to shorten again |
| A folder like `Downloads`/`Music`/`Pictures` shows as a blank `~/` | its `[directory.substitutions]` glyph was stripped to a bare space | values are `icon + name` (e.g. `Downloads = "<nerd-font-glyph> Downloads"`) using a codepoint your font has; `tests/starship/directory_test.sh` fails on a whitespace-only value |
| `Alt-h/j/k/l` window nav doesn't work in terminal | something rebinds bare Esc in the shell | `bindkey | grep '^"\^\['` in zsh — should NOT show `kill-whole-line` |
| tmux (or any new terminal) launches **bash on Linux**, not zsh | the login shell was never changed — `~/.zshrc` is symlinked but the account still logs into bash | re-run `./setup.sh` and accept "Make zsh your default login shell?", or `chsh -s "$(command -v zsh)"` then log out/in. macOS already defaults to zsh |
| `chsh` fails with `user '<name>' does not exist in /etc/passwd` | you log in via a **domain** account (AD/LDAP/SSSD) that isn't in local `/etc/passwd`, so `chsh` can't touch it | re-run `./setup.sh` — it detects this and offers to re-exec interactive bash into zsh via `~/.bashrc` instead. The "proper" fix is admin-side: set the directory `loginShell` / SSSD `default_shell` |
| Move commits in lazygit, including inside psmux | Ctrl+J collides with Enter on the wire, and psmux v3.3.4 does not relay Windows Terminal's Win32-input-mode modifier data into panes | use uppercase `J` / `K`. `%LOCALAPPDATA%\lazygit\config.yml` binds commits-panel moveDownCommit / moveUpCommit to printable J/K, so no psmux root bind is needed. In the commits panel, use PgUp/PgDn or Ctrl-U/Ctrl-D to scroll the diff |
| Ghostty doesn't open maximized | `window-save-state = always` restored an old geometry over `maximize` (macOS only) | `ghostty/config` uses `window-save-state = default` (not `always`) with `maximize = true`; `always` lets the saved size win |
| Ghostty doesn't load the config | wrong path | the install path is `~/Library/Application Support/com.mitchellh.ghostty/config` on macOS, `~/.config/ghostty/config` on Linux. `bootstrap.sh` handles this |
| Windows Terminal lost a profile after merge | WT auto-rewrites — pre-merge backup is at `<settings.json>.bak.<timestamp>` | restore the profile list from the backup |
| `bootstrap.ps1` errors "cannot create symbolic links" | Developer Mode off and not elevated | `bootstrap.ps1` now reports your *elevated* + *Developer Mode* state and the fix: enable Developer Mode (Settings → Privacy & security → For developers — no admin, recommended) **then** `.\setup.ps1 -SkipDeps`; OR run just `.\bootstrap.ps1` from an elevated PowerShell. Don't elevate the whole `setup.ps1` — scoop refuses to run as admin |
| Ghostty won't open maximized on Linux/GNOME | `maximize = true` is a hint the WM may ignore (GNOME Mutter often does) | on **X11**, `install-deps` offers a devilspie2 setup through the native Linux package manager, even when Linuxbrew is the main CLI manager; the rule is keyed on `com.mitchellh.ghostty`. Wayland needs a GNOME Shell extension instead |
| `install-deps.ps1`: winget `No package found matching input criteria` (exit `-1978335212`) | winget source/catalog flakiness | install-deps now **prefers scoop** and falls back across managers per tool — accept the scoop bootstrap when offered (`irm get.scoop.sh \| iex`) and re-run; scoop carries every CLI tool here |
