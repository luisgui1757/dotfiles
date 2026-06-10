# Repo guide for coding agents (and humans coming back to this cold)

This file is the on-ramp. If you're a future coding-agent session, read this
**before** touching anything. If you're me six months from now and forgot how
the install script works, read this too.

> **Single source of truth.** This file is canonical. `AGENTS.md` at the repo
> root is a thin pointer here so non-Claude agents discover it; do not copy this
> content there because two real guide files would drift. Claude Code auto-loads
> this file, and other agents reach it through `AGENTS.md`.

## What this repo is

Cross-platform dotfiles: Neovim (lazy.nvim), Starship, Ghostty, Windows
Terminal, tmux, zshenv/zshrc, PowerShell profile, lazygit. Public installs go
through `setup.sh` (macOS / Linux / WSL) or `setup.ps1` (Windows), which install
dependencies, apply the chezmoi config layer, and sync Neovim plugins + Mason
tools. The repo can live anywhere — `~/dotfiles/`, `~/Documents/dotfiles/`,
etc. The remote-clone default in `setup.{sh,ps1}` is `~/dotfiles/`, but an
in-place clone elsewhere works too. Do NOT put the repo at `~/.config/nvim/` —
the installer creates that path as a symlink **pointing into** the repo, so a
repo there would self-overlap (the self-link guard refuses this).

The `home/` tree is the chezmoi source tree for the full config layer. `setup.*`
uses it in Phase 2. Top-level config files and their `home/` copies/templates
must stay byte-identical where the parity manifest says so; update both in the
same change.

Agent settings are intentionally **NOT** synced through this repo. Keep local
agent preferences in the agent's per-machine state directory; this repo does
not ship synced agent preference folders.

## Layout at a glance

```
~/dotfiles/
├── nvim/                  Neovim — init.lua, lua/{vim-options,util,plugins}
├── starship/              starship.toml (Rose Pine palette)
├── shells/                zshenv + zshrc + powershell_profile.ps1
├── tmux/                  tmux.conf (Rose Pine, vi-mode, OSC52 clipboard)
├── ghostty/               config (Rose Pine, Hack Nerd, tuned for tmux)
├── windows-terminal/      settings.fragment.jsonc + merge README
├── lazygit/               config.yml (J/K move-commit binding)
├── home/                  chezmoi source tree for the config layer
├── tests/                 automated tests, grouped by tool
├── tests/wsl/             manual WSL split-host e2e check
├── .github/workflows/     CI matrix + chezmoi parity
├── .github/rulesets/      checked-in GitHub ruleset payloads for main
├── docs/security/         branch-protection runbook
├── setup.sh               public macOS/Linux/WSL setup entry point
├── setup.ps1              public Windows setup entry point
├── test.ps1               Windows test entry point
├── Makefile               Unix `make setup`, `make test`, `make lint`
├── .editorconfig          formatting rules every editor + agent respects
├── stylua.toml            Lua formatter style (Spaces / width 2); conform reads it
├── README.md              the human-facing install matrix
├── AGENTS.md              standard agent entry point, points here
└── CLAUDE.md              canonical tracked coding-agent guide
```

Local agent state directories such as `.claude/` are **not** part of the synced
configuration. Leave them untracked; preferences live per machine.

## Non-negotiable invariants

These are guarded by `tests/static/invariants_test.sh`. If you change code
that violates one of these, fix it instead of disabling the test.

1. **Leader is set before lazy.** `vim.g.mapleader = " "` must appear in
   `nvim/init.lua` *before* the `require("lazy").setup(...)` line. Plugin specs
   that mention `<leader>` are resolved at spec-import time, so flipping the
   order silently re-binds every `<leader>X` to `\X`. Caught by
   `tests/nvim/spec/leader_spec.lua`.
2. **No `NODE_TLS_REJECT_UNAUTHORIZED`.** Anywhere in the repo. The old config
   disabled TLS verification globally for node-based plugins — that's a
   security regression, not a workaround.
3. **No `vim.loop`.** Use `vim.uv`. (Caught by invariants_test.)
4. **No `client.supports_method(...)` dot-call.** Use the colon form
   `client:supports_method(...)` — the dot form is deprecated in nvim 0.11.
5. **No `bindkey '\e' kill-whole-line` in zshrc.** That binding shadows the
   entire Meta prefix and breaks every `Alt-X` keybinding (including the
   `<A-h/j/k/l>` window-nav bindings in `vim-options.lua`).
6. **Starship `git_status` styles use `($style)` not `(style)`.** Literal
   `(style)` is a render-bug — the closing icon wouldn't get styled.
7. **`rose-pine.lua` is the only plugin with `lazy = false`**, and it must
   have `priority = 1000`. Everything else lazy-loads on event/cmd/keys/ft.
8. **`conform.nvim` is the ONLY format-on-save handler.** Don't add a
   `BufWritePre` autocmd in `lsp-config.lua` or in any `LspAttach` block. The
   old config raced two handlers with different timeouts.
9. **`vim.b.skip_format_on_save` controls `:WNF`.** If you add another
   format-on-save path, it MUST check this flag (see `nvim/lua/plugins/conform.lua`).
10. **The four deleted dead files must stay deleted:**
    `nvim/lua/plugins.lua`, `nvim/lua/plugins/ai.lua`,
    `nvim/lua/plugins/avante.lua`, `nvim/lua/plugins/none-ls.lua`.
11. **Markdown rendering lives in `nvim/lua/plugins/markdown.lua`
    (render-markdown.nvim).** Don't add `headlines.nvim` or
    `markview.nvim` alongside it — they overlap and fight for the same
    extmarks. Obsidian.nvim's own UI is disabled (`ui.enable = false`)
    in `notes.lua` so render-markdown owns rendering everywhere. obsidian.nvim
    loads on every markdown buffer — it is **no longer** gated on the vault dir
    existing (`enabled = isdirectory(...)` silently disabled it on machines
    without a vault). It `mkdir -p`s the resolved vault and honors `NOTES_VAULT`
    (see `util/notes_path.lua`); set that env var to point at your real vault.
12. **No `vim.lsp.set_log_level(...)`.** Deprecated in nvim 0.11; use the module
    form `vim.lsp.log.set_level(...)` (see `lsp-config.lua`). Guarded by
    `invariants_test.sh`.
13. **zsh plugin order is intentional.** `zsh-autocomplete` must be sourced
    before local `compinit`; when autocomplete loads, `zshrc` must skip the
    repo's manual `compinit` block. `zsh-autosuggestions` loads after completion
    setup. Keep `shells/zshenv` minimal and keep `skip_global_compinit=1` there.
14. **WSL is split-host by default.** Windows Terminal, Hack Nerd Font, and
    `win32yank` are Windows-host responsibilities. WSL installs the Linux CLI
    stack. Linux Ghostty and Linux fontconfig fonts in WSL require
    `--experimental-wsl-gui`; do not make them the default path again.
15. **Windows Terminal is installed by Windows setup.** Keep `wt` in the
    `install-deps.ps1` Scoop-first catalog (`extras/windows-terminal` ->
    `Microsoft.WindowsTerminal` -> `microsoft-windows-terminal`). The app
    install and settings merge are separate code paths (`install-deps.ps1` vs
    chezmoi's `modify_` entry), and setup runs the merge by default. Opt out
    with `-SkipWindowsTerminalMerge`; `-MergeWindowsTerminal` is a retained
    no-op alias. If `settings.json` is absent because WT has not launched yet,
    the merge warns and skips so default-on setup does not break an unlaunched
    WT.
16. **tmux uppercase `H`/`L` are window swaps.** Lowercase `h`/`l` stay pane
    focus bindings. Do not replace them with arrow-key bindings unless the
    terminal/psmux behavior has been revalidated.
17. **DAP UI stays lazy.** `nvim-dap-ui` must keep `lazy = true`; otherwise the
    full debug UI and `nvim-nio` load during startup and blow the startup budget.
18. **Main-branch CI is non-bypassable and owner-only.** Keep required status
    checks in `.github/rulesets/main-integrity.json` with no bypass actors.
    Owner review bypass belongs only in `.github/rulesets/main-review.json`;
    owner update bypass belongs only in
    `.github/rulesets/main-owner-updates.json`. Both bypasses must use
    `bypass_mode = pull_request`. Do not collapse these rulesets into one.

## Common workflows

### Add a new plugin

Drop a single file under `nvim/lua/plugins/<name>.lua` returning the lazy
spec. Lazy auto-discovers `{ import = "plugins" }` from `init.lua`. Default
to lazy-loading (`event` / `cmd` / `keys` / `ft`). Only `rose-pine` may set
`lazy = false`.

### Add a new LSP server

1. Add the server name to `vim.lsp.config(...)` and the `vim.lsp.enable({...})`
   list in `nvim/lua/plugins/lsp-config.lua`.
2. Add the Mason package to the `ensure_installed` array in the same file
   (under the `mason-tool-installer` config block).
3. Add the server name to `tests/nvim/spec/lsp_spec.lua`'s `required_servers`
   so the static-check catches a future accidental removal.

> **Headless-install gotcha:** `mason-tool-installer` is `event = "VeryLazy"`
> (interactive auto-install via `run_on_start`) **and** registers its commands
> under `cmd = { … }`. Those `cmd` triggers are load-bearing — the setup phase
> runs `nvim --headless +MasonToolsInstallSync`, and `VeryLazy` never fires
> without a UI, so without the `cmd` trigger that command is `E492: Not an
> editor command`. Keep `MasonToolsInstallSync`/`MasonToolsUpdate` in the `cmd`
> list (guarded by `lsp_spec.lua`).

### Add a new formatter

1. Add to `formatters_by_ft` in `nvim/lua/plugins/conform.lua`.
2. Add the Mason package to `mason-tool-installer` `ensure_installed`.
3. No new autocmd — conform's `format_on_save` handles it. The buffer-local
   `vim.b.skip_format_on_save` short-circuit (i.e. `:WNF`) is already in place.

### Add a project-specific DAP launch

Keep shared DAP config generic. Set `DAP_LAUNCH_URL` for the default browser
launch URL, or put project-specific `dap.configurations` in a workspace
`.nvim.lua`; do not bake app names, ports, or routes into these dotfiles.

### Add a treesitter parser

1. Add the parser name to `ensure_installed` in
   `nvim/lua/plugins/treesitter.lua`.
2. Add it to `required` in `tests/nvim/spec/treesitter_spec.lua`.

### Rebind a Rose Pine color anywhere

The palette is **one constant set** used everywhere. Keep it consistent:

```
overlay  #26233a   love    #eb6f92   gold    #f6c177
rose     #ebbcba   pine    #31748f   foam    #9ccfd8
iris     #c4a7e7   base    #191724   surface #1f1d2e
muted    #6e6a86   subtle  #908caa   text    #e0def4
```

Surfaces that consume these: nvim (rose-pine plugin defaults), lualine
(theme="rose-pine"), starship.toml (`[palettes.rose-pine]`), tmux.conf (hex
literals in status/borders), ghostty/config (`theme = dark:Rose Pine,...`),
windows-terminal/settings.fragment.jsonc (`schemes` + `themes`),
shells/powershell_profile.ps1 (PSReadLine `-Colors`).

### Refresh the lazy lockfile after adding plugins

```bash
nvim --headless "+Lazy! sync" +qa
git add nvim/lazy-lock.json
```

`lazy-lock.json` is tracked (NOT gitignored) — that's how every machine ends
up on the same commits.

### Update Mason-installed tools across machines

```bash
nvim --headless "+MasonToolsUpdate" +qa
```

There's no machine-pinned lockfile for Mason itself — `mason-tool-installer`
ensures the named tools exist on each machine.

## Test runner

```bash
make help            # list all sub-targets
make test            # run everything that can run on this OS
make test-nvim       # plenary busted suite
make lint            # shellcheck across all .sh
./tests/wsl/e2e.sh   # manual WSL split-host validation from inside WSL
```

On Windows, use the same entry point as CI:

```powershell
.\test.ps1          # PSScriptAnalyzer + Pester + Nvim plenary busted
```

On Windows, `tests/nvim/run.ps1` intentionally executes each `*_spec.lua`
directly through `plenary.busted`. Do not use `PlenaryBustedDirectory` there:
its parent process can false-fail after all child specs passed when PowerShell
native-command error promotion is enabled.

On Unix, `tests/nvim/run.sh` uses `PlenaryBustedDirectory` with an explicit
`timeout = 180000`. Keep that value explicit: `startup_spec.lua` prewarms a real
production init under isolated XDG dirs, and Plenary's default 50s timeout can
SIGTERM the child before the startup-budget assertion reports the actual
problem.

Sub-targets **skip gracefully** when their tool isn't installed
(`yamllint`/`editorconfig-checker`/`hyperfine`/`bats`/`ghostty`). The
ubuntu/macos/windows CI matrix in `.github/workflows/test.yml` installs
everything, `chezmoi-parity`, `chezmoi-parity-macos`, and
`chezmoi-parity-windows` install pinned chezmoi for the config-layer migration
oracle (`template_test.sh`, canonical-only `parity_gate.sh`, Windows apply, and
round-trip uninstall), and `test.ps1` treats missing Windows test dependencies
as fatal under CI, so anything passing locally + CI is genuinely cross-platform.
Static repo walkers intentionally exclude `home/` managed copies; those copies
are validated by `tests/migration/parity_gate.sh` against the canonical
top-level sources instead of being re-linted as independent source.
`tests/static/toml_lint.sh` uses `taplo` when it is healthy, but if local macOS
`taplo` panics with the known system-configuration null-object crash it falls
back to Python `tomllib`; ordinary `taplo` lint errors still fail.

When adding a new spec:
- Plenary specs: drop a `*_spec.lua` under `tests/nvim/spec/`. Use plenary
  busted's `describe` / `it` / `before_each` / `after_each` — **do not** use
  `setup` / `teardown` (those globals don't exist in plenary's busted).
- Shell tests: drop a `*_test.sh` under `tests/shell/` (or other dir). It's
  picked up by the dir's `run_all.sh` automatically.

## CI / repository safeguards

`test.yml` remains the fast cross-platform suite and now also owns the
`chezmoi-parity` migration gate. Warnings are treated as failures where the
tools expose them cleanly: shellcheck exits nonzero, PSScriptAnalyzer runs at
`Warning,Error`, yamllint/parser checks are part of `make test-static`, and
Windows CI treats missing test dependencies as fatal.

`e2e-install.yml` is the required real-install gate. The jobs cover different
install paths, not symmetric container platforms:

- `e2e containers / ubuntu-24.04` runs an `ubuntu:24.04` container on an Ubuntu
  runner with `DOTFILES_SKIP_BREW_BOOTSTRAP=1`, creates a non-root user, runs
  real `install-deps.sh --all` (native `apt`, no Linuxbrew), then applies
  configs with chezmoi and asserts tool presence, Neovim >= 0.11, lazygit, zsh
  plugin files, config content matching the repo sources, and the Neovim
  directory resolving into repo `nvim/`.
  This is intentionally **not** a devcontainer. It stays because hosted Ubuntu
  has Linuxbrew available, so the container is the only automated proof of the
  clean-image native `apt` path: pinned Neovim tarball install, pinned lazygit
  release install, zsh plugin install, `fd-find` -> `fd` shim, and apt
  fallbacks. Scope is intentionally **Ubuntu only** (the supported Linux/WSL2
  proxy target).
  Re-adding another distro requires both a matrix entry in `e2e-install.yml` and
  a matching root-prep branch in `tests/ci/container-e2e.sh`.
- `setup.sh / ubuntu-24.04`, `setup.sh / macos-15`, and
  `setup.ps1 / windows-2025` run the real public setup entry points, apply
  configs through chezmoi in Phase 2, and then rerun Lazy/Mason headless sync.
  They explicitly fail if setup skips Phase 3-4, emits a `FAIL:` marker, or
  Mason did not install expected tools.
- There is no macOS/Windows container analog to add for symmetry. Docker cannot
  model macOS, and Windows containers do not model the real desktop/user-profile
  install surface: Scoop/winget/choco, Developer Mode symlink behavior, font
  registration, and terminal profiles. Hosted macOS/Windows runners are the
  accepted representative fixtures for those OSes.
- `setup.sh / WSL2 Ubuntu-24.04 (best-effort canary)` uses
  `Vampire/setup-wsl@v7.0.0`, but hosted runners cannot provide a reliably
  required nested-virtualization WSL2 gate. Keep this job non-required unless
  the owner intentionally accepts that flake risk. The required WSL proxy is the
  Linux Ubuntu container plus the existing `DOTFILES_FORCE_OS=wsl` bats coverage.
  Full WSL host/guest validation is manual: run `./tests/wsl/e2e.sh` from inside
  WSL after running `.\setup.ps1 -All` on Windows. The Windows Terminal settings
  merge is default-on when `settings.json` already exists.

Main-branch safeguards are canonical in `.github/rulesets/` and applied live by
`scripts/apply-repo-safeguards.sh`. `.github/settings.yml` is only the classic
branch-protection fallback for the Probot Settings app; it cannot model the
required split where owner bypass applies to review/update rules but not CI.

- `Protect main: integrity` has no bypass actors. It requires pull requests,
  strict required checks, current `main`, squash-only merges, linear history, no
  branch deletion, and no non-fast-forward updates.
- `Protect main: review` has the only bypass actor: `luisgui1757` with
  `bypass_mode = pull_request`. It requires one approval, CODEOWNER review,
  stale-review dismissal, last-push approval, and resolved review threads.
- `Protect main: owner updates` has the only update bypass actor:
  `luisgui1757` with `bypass_mode = pull_request`. It prevents automation from
  updating `main`, even when it can push branches and open PRs.
- Repository settings are squash-only. Merge commits and rebase merges are
  disabled, squash merges are enabled, branches are deleted after merge, and
  repo-level auto-merge stays disabled.

GitHub does not let pull request authors approve their own pull requests. Owner
authored PRs can use the owner review bypass, but they still cannot bypass the
integrity ruleset's required checks. Repository deletion is outside branch
protection for a personal repo; routine agents should use least-privilege
credentials, not owner-account, admin, or `delete_repo` capable tokens.

Run `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles` after changing the
rulesets, then verify the live posture with the commands in
`docs/security/branch-protection.md`. Do not add the WSL2 canary to required
checks unless asked.

`renovate.json` owns GitHub Actions version updates and repo-pinned version/ref
constants. Dependabot version-update PRs are intentionally disabled; GitHub
native Dependabot security alerts and automated security fixes stay enabled by
`.github/settings.yml`. Runner-image updates (`ubuntu-*`, `macos-*`,
`windows-*`) are still detected by the GitHub Actions manager, but they are
split into the `github runner images` group and labeled `github-runners` because
they change the CI platform contract and should be reviewed separately from
ordinary Action bumps.

Renovate custom managers can bump pinned version/ref constants, but they cannot
recompute SHA-256 values or verify tag commit IDs. The `github-releases`
datasource has no digest resolver for direct-download archives, and zsh plugin
tags are intentionally paired with adjacent expected commits. After a Renovate
bump, adjacent checksum/commit constants stay stale until a human
recomputes/reviews them; CI verification must fail in the meantime. Do not model
direct-download SHA-256 constants as Renovate `currentDigest` captures; in
Renovate terms those are not datasource digests. Only the cargo-binstall git
commit is captured as a digest.

Validate `renovate.json` locally with Renovate's own schema validator, not just
`jq`: `make validate-renovate`. That target runs Renovate under Node 24 because
Renovate's `engines.node` supports the Node 24 LTS line; running the validator
directly under an unsupported odd/current host Node such as 25.x emits
`EBADENGINE`. Do not silence that warning with npm config. Switch runtimes or use
the repo target, which shells into Node 24 before running:

```bash
npx --yes --package node@24.11.0 -- bash -c \
  'npm exec --yes --package renovate@latest -- renovate-config-validator --strict renovate.json'
```

`tests/static/json_lint.sh` only checks JSON syntax. The Renovate validator
checks schema, and the first live Dependency Dashboard/PR should still be used
to confirm the custom regex managers match the intended files.

## :WNF (Write Without Formatting)

The buffer-local feature you can rely on. `:WNF<CR>` (or lower-case
`:wnf<CR>`) writes the current buffer with formatters skipped for **this one
save only**. The next plain `:w` formats normally. Implemented in
`nvim/lua/vim-options.lua`; the skip flag is consumed in
`nvim/lua/plugins/conform.lua`'s `format_on_save` callback and cleared by a
`BufWritePost` autocmd.

## Config apply (chezmoi, via setup Phase 2)

- `setup.sh` and `setup.ps1` run `chezmoi init`, back up pre-existing managed
  file/symlink targets to `<target>.bak.<timestamp>` only when the target is not
  already exact chezmoi state or content-equivalent to the chezmoi target, then
  run `chezmoi --no-tty --force apply`. `--skip-bootstrap` remains a back-compat
  alias for `--skip-config` / `-SkipConfig`.
- `setup.ps1` runs a symlink-privilege pre-flight before chezmoi apply because
  the Windows Neovim target is still a directory symlink: dry-run warns and
  skips the probe; real runs print elevated/Developer Mode state plus the
  Developer Mode or elevated-config-step fix before attempting apply.
- Remote `setup.{sh,ps1}` has exactly one hard prerequisite: `git`, because the
  remote path must clone this repo before it can install everything else. The
  missing-git errors name the canonical first install command (`brew install
  git`, `apt install git`, or `winget install Git.Git`).
- The Windows installer does NOT symlink `settings.json` for Windows Terminal:
  WT rewrites that file on launch. `setup.ps1` Phase 2 copies an existing
  pre-merge file to `settings.json.bak.<timestamp>` before running chezmoi apply,
  unless `-SkipWindowsTerminalMerge` is passed. Chezmoi's `modify_` entry then
  merges the user-owned keys by default; a bare `chezmoi apply` performs the
  merge but does not create setup's backup. The legacy `-MergeWindowsTerminal`
  switch remains accepted as a no-op alias.
- **lazygit config paths are OS-specific.** On macOS, lazygit v0.58 reports
  `~/Library/Application Support/lazygit` from `lazygit --print-config-dir`;
  on Linux/WSL it uses `~/.config/lazygit`; on Windows it uses
  `%LOCALAPPDATA%\lazygit`. Keep the chezmoi templates, README, and tests
  aligned with those real read paths.
- **The chezmoi tree in `home/` owns the config layer, not provisioning.** The
  rule is `chezmoi=dotfiles, install-deps=provisioning`: do not port package
  installs, pinned binary/font installers, login-shell mutation, devilspie2, VS
  Code, psmux installation, or distro package-manager policy into chezmoi
  run-scripts. psmux stays in `install-deps.ps1` via `Install-Psmux` and the
  hardened `Add-ScoopBucketSafe` path; chezmoi only owns the psmux-readable
  config files (`.tmux.conf` and `.tmux.windows.conf`).
  `home/.chezmoi.toml.tmpl` is the mode switch: POSIX uses `mode = "symlink"`
  for live-edit behavior, Windows uses `mode = "file"` for simple single-file
  configs, but Windows `nvim` remains a directory symlink and still needs
  Developer Mode or elevation. Same-path config files use managed source copies; path-divergent
  lazygit and Ghostty configs use `.chezmoitemplates/**` plus POSIX
  `symlink_*.tmpl` wrappers and Windows rendered `.tmpl` copies where
  applicable. Windows Terminal is a `modify_` read-modify-write merge, not a
  symlink or fragment-only replacement. The nvim tree is intentionally NOT
  copied under `home/`: POSIX `home/dot_config/symlink_nvim.tmpl` and Windows
  `home/AppData/Local/symlink_nvim.tmpl` both point at
  `{{ .chezmoi.sourceDir }}/../nvim`, so managed targets resolve to the repo
  top-level `nvim/` directory. Windows nvim is therefore still a
  directory symlink and still needs Developer Mode or elevation; the
  no-Developer-Mode win applies to simple copied files. Do not use `exact_` for
  nvim; app runtime state lives outside `.config/nvim`, but user/plugin-added
  config files should not be deleted by chezmoi. `home/.chezmoiignore` must gate
  whole wrong-OS directories to avoid empty parent dirs. The Windows PowerShell
  7 profile path is managed at
  `Documents/PowerShell/Microsoft.PowerShell_profile.ps1`; the Windows
  PowerShell 5.1 profile path under `Documents/WindowsPowerShell/` is out of
  scope because this repo is pwsh-first. POSIX pwsh profile management remains
  outside the static chezmoi source tree because it depends on which host shell
  and `$PROFILE` path are available after `pwsh` is installed. WSL is gated
  through `home/.chezmoi.toml.tmpl`'s `isWsl` data value:
  `.chezmoiignore` skips Linux Ghostty on WSL unless setup passes the per-run
  `experimentalWslGui` data override for `--experimental-wsl-gui`. The migration oracle is `tests/migration/parity_gate.sh` +
  `tests/migration/oracle_test.sh` + `tests/migration/windows_apply_test.ps1`;
  it enforces canonical repo-source parity, nvim dir-symlink realpath/content parity,
  single-source byte equality, wrong-OS absence, zsh exact-pin failure behavior,
  and Windows copy-mode + WT merge parity.
- **`install-deps` provisions chezmoi itself.** Unix setup installs `chezmoi`
  with Homebrew when brew is the selected manager; native Linux without brew
  uses the pinned `CHEZMOI_VERSION` release through the same trusted
  `get.chezmoi.io` installer form as CI, into `~/.local/bin`. Windows setup
  installs `chezmoi` through the normal Scoop-first `$Catalog` fallback chain
  (`scoop` -> `winget` -> `choco`). This keeps `make chezmoi` usable after full
  setup before the Phase 2 chezmoi apply.
- **`uninstall.sh` / `uninstall.ps1` are greenfield teardown tools, not purge.**
  They enumerate targets with `chezmoi --source <repo>/home managed --path-style
  absolute`, remove only repo-owned symlinks or byte-identical Windows
  copy-mode files, restore newest bootstrap-style `<target>.bak.<timestamp>`
  backups by default, and leave chezmoi's own config/state alone. Windows
  Terminal `settings.json` is never deleted because the merge is idempotent but
  not invertible; use the printed backup path for manual restore if needed.
- **lazygit binary install paths differ by OS.** Homebrew owns macOS/Linuxbrew,
  Windows setup installs it through Scoop/winget/choco, and native Linux/WSL
  without brew uses a pinned GitHub release tarball with SHA-256 verification.
- **`install-deps.ps1` prefers scoop, then falls back across managers
  per tool.** `Install-One` builds an ordered candidate list (scoop → primary →
  winget → choco) of managers that are installed AND carry the package, and
  tries each until one succeeds — so a winget `No package found` (exit
  `-1978335212`) no longer dead-ends a tool. scoop carries the cataloged
  CLI/terminal tools, including Windows Terminal as `wt`
  (`extras/windows-terminal`).
- **`Update-ScoopTool` is the only scoop update path.** It is intentionally
  single-package and consent-gated for the PowerShell 7 keep-latest path; never
  replace it with `scoop update *` or another blanket scoop upgrade.
- **Windows CI uses Scoop's documented elevated bootstrap.** GitHub-hosted
  `windows-2025` runners are elevated, and Scoop blocks elevated install by
  default. `Install-Scoop` detects elevation and runs the official installer
  with `-RunAsAdmin`, then adds the Scoop `shims` dir to the current process
  PATH so the rest of `install-deps.ps1` can immediately use `scoop`.
  `Install-Scoop` also ensures the `extras` and `nerd-fonts` buckets whenever
  Scoop exists, including pre-existing user installs. Every bucket add routes
  through `Add-ScoopBucketSafe`: idempotent; non-interactive via
  `GIT_TERMINAL_PROMPT=0` + `GCM_INTERACTIVE=0`; and verified populated so
  Scoop#5482's registered-but-empty bucket state does not masquerade as
  success. All `scoop bucket add` calls in `install-deps.ps1` must go through
  `Add-ScoopBucketSafe` (guarded by `tests/static/repo_policy_test.sh`). Local
  setup still recommends Developer Mode plus a normal PowerShell; do not
  elevate the whole local setup unless you intentionally want the admin path.
- **`DOTFILES_SKIP_BREW_BOOTSTRAP=1` is a CI/native-PM test knob.** On Linux,
  when no `brew` is already installed, this prevents `install-deps.sh --all`
  from bootstrapping Linuxbrew and keeps the detected native package manager
  (`apt` in the current Ubuntu container gate). It is used by container e2e so
  that job actually exercises the native package-manager path. Do not set it for
  normal macOS setup; macOS still needs Homebrew.
- **Apt `fd-find` is shimmed to `fd`.** Debian/Ubuntu package the command as
  `fdfind`, but Telescope expects `fd`. After installing `fd-find`,
  `install-deps.sh` idempotently links `~/.local/bin/fd` to `fdfind` and adds
  that directory to the current PATH.
- **Alpine installs Neovim through `apk`.** The official Neovim Linux tarball
  targets glibc systems, so Alpine uses its native `neovim` package instead;
  e2e still enforces the repo's Neovim >= 0.11 floor.
- **`install-deps.sh` prompts for the notes/Obsidian vault** and persists
  `export NOTES_VAULT=…` to `~/.zshrc.local` (sourced by `zshrc`, read by
  `util/notes_path.lua`). The prompt is tty-gated (skipped under `--all` /
  piped / `--dry-run`); the write logic is split into `persist_notes_vault` so
  `tests/shell/notes_vault_test.sh` can exercise it without a tty.
- **`install-deps` can install VS Code + the Rose Pine theme.** Both installers
  offer VS Code (macOS brew cask; Linux snap/flatpak/manual; Windows
  winget/scoop/choco). Then, **only if `code` is detected**, they install the
  `mvllow.rose-pine` extension and set `workbench.colorTheme` to "Rosé Pine".
  The theme setter (`set_vscode_theme` in sh, tested by `vscode_theme_test.sh`;
  `Set-VSCodeTheme` in ps1) only merges into *clean* JSON (jq /
  `ConvertFrom-Json`) — VS Code settings are usually JSONC with comments, so it
  leaves those untouched rather than clobbering them. The theme value must keep
  its accented é to match the extension's label; the ps1 emits it as a `\u` JSON
  escape (or `[char]0xE9`) so that file stays pure ASCII (invariant), while the
  sh side uses the literal é.
- **Direct GitHub downloads are pinned and SHA-256 verified.** `install-deps.sh`
  verifies the pinned Neovim Linux tarballs, lazygit Linux tarballs, and Hack
  Nerd Font zip before extraction; `install-deps.ps1` verifies the pinned
  Hack.zip before registering fonts. The CI workflows also pin and verify their
  direct GitHub downloads.
  This extends to the **Ubuntu Ghostty installer**: `install_ghostty_linux`
  pins `mkasberg/ghostty-ubuntu`'s `install.sh` to `GHOSTTY_UBUNTU_VERSION` and
  SHA-256 verifies the script (`GHOSTTY_UBUNTU_INSTALL_SHA256`) before running
  it (`run_ghostty_ubuntu_installer`) — NOT a bare `curl … | bash` of `HEAD`.
  The pinned script still fetches the matching `.deb` from that project's GitHub
  release assets over HTTPS at run time (per-codename×arch, so the `.deb` itself
  is not individually pinned — same trust model as the Homebrew installer).
  A checksum mismatch fails closed: the installer is not run, a `FAIL:` marker
  is emitted, and setup continues for real users while CI fails on the marker.
  Update the version and checksum constants together. zsh plugin refs are also
  pinned by tag plus expected commit; update both after reviewing a Renovate tag
  bump. Guarded by `tests/shell/ghostty_install_fail_test.sh`,
  `tests/shell/wsl_gui_tools_test.sh`, `tests/shell/lazygit_install_test.sh`, and
  `tests/shell/zsh_plugins_test.sh`.
  Renovate can open version/ref bumps for these constants and for the CI
  cargo-binstall installer commit, but it cannot recompute adjacent SHA-256
  values or verify tag commit IDs; leave CI red until a human has reviewed the
  download/ref and updated the adjacent constant. In `renovate.json`,
  direct-download SHA-256 values must be matched as context only, not named
  `currentDigest`, otherwise Renovate will schedule same-version digest updates
  for checksums it cannot actually resolve.
- **Both installers open with an "install EVERYTHING?" prompt.** Interactive
  runs that didn't pass `--all`/`-All` get one upfront question; answering yes
  flips `YES_ALL`/`$All` so the rest runs with no per-item prompts. Skipped when
  `--all`/`--dry-run` was passed or there's no tty (so `curl|bash` and the CI
  `--dry-run --all` dogfood don't hang).
- **Windows symlink pre-flight reports WHY symlinks fail and how to fix it.**
  `setup.ps1` probes symlink capability before chezmoi apply. When the probe
  fails it prints your *elevated* (admin) and *Developer Mode* state, then the
  two fixes (Developer Mode, no admin, recommended; or an elevated config-only
  setup step), and `exit 1` via `Write-Host` (not `Write-Error`, so no stack
  trace). This keeps nvim sync from running against unapplied configs. Note:
  don't elevate the dependency-install run because Scoop refuses to run as
  admin.
- **Forcing Ghostty maximize on Linux is WM-side, not config.** `maximize = true`
  is only a hint the compositor may ignore — confirmed on **GNOME 46 / X11**,
  where Mutter does NOT honor it. `install-deps.sh`'s `setup_ghostty_maximize`
  is an opt-in step (Linux + Ghostty installed + non-Wayland) that installs
  devilspie2, symlinks `linux/devilspie2/ghostty-maximize.lua` (keyed on WM_CLASS
  `com.mitchellh.ghostty`) into `~/.config/devilspie2/`, and writes a
  `~/.config/autostart/devilspie2.desktop`. It lives in install-deps (installs a
  package + runs a daemon), NOT the chezmoi config layer; Wayland needs a GNOME
  Shell extension instead. Guarded by `tests/shell/devilspie2_test.sh`.
- **psmux is the Windows tmux** (`install-deps.ps1` → `Install-Psmux`). Picked
  because it **reads `~/.tmux.conf`** and speaks the tmux command language, so
  Windows reuses the *same* `tmux/tmux.conf` we maintain for Unix — one source
  of truth, Rose Pine carries over, no parallel config. Chezmoi manages
  `%USERPROFILE%\.tmux.conf` from `tmux\tmux.conf` (mirrors the Unix
  `~/.tmux.conf` target). If the Unix-shaped `if-shell` clipboard block ever
  chokes under ConPTY on a given machine, guard that block rather than fork the
  config. Install-Psmux is NOT in the `$Catalog` because scoop needs a custom
  psmux bucket URL; it passes that URL through `Add-ScoopBucketSafe`, then
  falls through to winget / choco if the bucket clone or scoop install fails.
- **psmux + PSReadLine: Windows-only overlay, two settings.** psmux's default
  shell is **cmd**, not pwsh — which is the *real* reason "history prediction"
  and `MenuComplete` looked broken inside panes: PSReadLine was never loaded.
  The fix is a Windows-only overlay `tmux/tmux.windows.conf`, managed as
  `~/.tmux.windows.conf` by chezmoi on Windows and pulled in by the main
  `tmux/tmux.conf` via `source-file -q` (silent no-op on Unix where it does not
  exist). The overlay sets:
  (1) `default-shell pwsh` — so fresh psmux panes spawn PowerShell 7, which
  loads PSReadLine + the profile.
  (2) `allow-predictions on` — psmux otherwise resets `PredictionSource` to
  `None` during pane init (psmux issue #150 — fresh panes ignore the profile's
  `HistoryAndPlugin`). With this on, the profile's ListView prediction +
  `Tab=MenuComplete` survive into psmux panes.
  `install-deps.ps1` owns installing PowerShell 7; the overlay intentionally
  assumes `pwsh` is present after setup.
  Diagnose inside a pane with `(Get-Process -Id $PID).Name` (expect `pwsh`).
  Do NOT add `set -g default-shell` to the main `tmux.conf` — `pwsh` does not
  exist on Unix; keep Windows-specific tmux settings in the overlay.
- **psmux residual race (v3.3.4): `OnIdle` workaround in the profile.** Even
  with `allow-predictions on`, fresh psmux panes were observed at
  `PredictionSource=None` / `PredictionViewStyle=InlineView` -- the documented
  psmux init resets PSReadLine **after** `$PROFILE` finishes (issue #150). The
  user-visible "calling pwsh inside psmux fixes it" trick works because the
  nested pwsh re-runs `$PROFILE` after psmux is done. Same idea, done
  automatically: `shells/powershell_profile.ps1` registers a one-shot
  `PowerShell.OnIdle` (gated on `$env:TMUX`) that re-applies
  `HistoryAndPlugin`/`ListView` + `Tab=MenuComplete` (+ `ShowToolTips` when the
  parameter exists, PSReadLine ≥ 2.3.4). Microsoft + psmux #150 both point at
  `OnIdle` as the right hook. Drop it once a psmux release fixes the residual
  race upstream (no PR yet; track issue #150 / #165).

## Login shell: zsh adoption (install-deps.sh)

Installing the zsh *package* does NOT make zsh your login shell — that takes a
`chsh`. `install-deps.sh` does it in the "terminal multiplexer + shell" section
(`set_default_shell_zsh`); without it, Linux keeps logging the account into bash
and tmux / new terminals never source the symlinked `~/.zshrc` (this is the
"tmux shows bash" symptom). The step is:

- **idempotent** — no-op when the login shell is already a *zsh* (compared by
  basename, so macOS's `/bin/zsh`, a distro `/usr/bin/zsh`, and a brew zsh all
  count as done; macOS therefore never churns);
- **consent-gated** — prompts before changing (auto-yes under `--all`);
- **dry-run-safe** — prints a `would:` line and mutates nothing under `--dry-run`
  (this is why the CI `--dry-run --all` dogfood stays green);
- it registers zsh in `/etc/shells` first (chsh refuses otherwise) and runs chsh
  as root / via sudo / via plain PAM, whichever is available. Takes effect on the
  next login.

It lives in `install-deps.sh`, NOT the chezmoi config layer, and
NOT `tmux.conf` (a tmux-only `default-command` would paper over the symptom while
bare TTYs and SSH sessions stayed bash).

**Domain / non-local accounts (AD/LDAP/SSSD):** these resolve through NSS but
are NOT in local `/etc/passwd`, so `chsh` fails (`user '<name>' does not exist in
/etc/passwd`). `set_default_shell_zsh` detects this on Linux via
`is_local_account` (an `awk` exact-match on `/etc/passwd`) and routes to
`adopt_zsh_domain` instead of `adopt_zsh_chsh`. The fallback (`ensure_bash_execs_zsh`)
appends an idempotent, **interactive-only** (`[[ $- == *i* ]]`, so scp/rsync and
scripts stay bash) marked block to `~/.bashrc` that `export SHELL`s zsh and
`exec zsh`, and makes `~/.bash_profile` source `~/.bashrc` so login shells (tmux,
ssh) hit it too. macOS is excluded from this branch (its accounts live in dscl,
not passwd files, and `chsh` works there). The textbook chsh path is unchanged
for local accounts.

**Test seam — leave it:** the line `if [[ -n "${INSTALL_DEPS_SOURCE_ONLY:-}" ]];
then return …` before the main install sections in `install-deps.sh` exists ONLY
so shell tests can `source` the installer function defs (without running any
installs) and exercise them against stubbed seams. Unset in normal runs, so it's
skipped. Keep similar explicit test seams in setup/config code only when the
host OS or shell would otherwise hide a branch from CI.

## Things that look weird but are intentional

- **Mouse split: tmux `mouse on`, psmux `mouse-selection off`, nvim mouse off.**
  Three different layers, three different responsibilities. (1) `set -g
  mouse on` in `tmux/tmux.conf` (guarded by `tests/tmux/option_test.sh`
  `check mouse on`) gives pane click-to-focus, scroll-wheel into copy-mode,
  drag-resize on pane borders, status-bar window-clicks. (2) `set -g
  mouse-selection off` + `pwsh-mouse-selection off` in
  `tmux/tmux.windows.conf` turns OFF psmux's separate client-side
  drag-selection layer (psmux issue #245 -- independent of `mouse`) so
  Windows Terminal owns click+drag and its native selection persists after
  release (Ctrl+Shift+C copies). `scroll-enter-copy-mode on` stays because
  that's a real feature, not selection-related. (3) Nvim is keyboard-only
  via `vim.opt.mouse = ""` + three defensive options: `mousescroll =
  "ver:0,hor:0"`, `mousefocus = false`, `mousemoveevent = false`. The
  defense matters because nvim 0.11's default is `mouse=nvi`; if any
  plugin or `:terminal` pass-through flips it back, the other three keep
  wheel-scroll inert, focus-follows-mouse off, and motion events
  suppressed. Use the documented zero-count form for `mousescroll` (NOT
  the empty string -- not a valid value per `:h 'mousescroll'`).
  Diagnostic-proven: under this config, clicks in an nvim pane do NOT
  reach nvim (the `<LeftMouse>` test from `:nnoremap <LeftMouse> <Cmd>echo
  'NVIM SAW IT'<CR>` never fires) -- any visual "cursor moves on click"
  perception is Windows Terminal's selection anchor inside the nvim
  display rect, not nvim's cursor.
- **Arrow keys are mapped to `<Nop>`** in `vim-options.lua`. User
  preference; hjkl-only navigation enforced.
- **`vim.opt.clipboard = "unnamedplus"`** even on macOS — works fine via
  pbcopy/pbpaste. The single-register `unnamed` value would lock WSL/Linux
  out.
- **`shells/zshrc` has shellcheck disable directives** at the top — zsh
  has glob qualifiers (`(#qN.mh+24)`) that shellcheck (a bash linter)
  cannot parse. The directives suppress the noise; the file is otherwise
  shellcheck-clean.
- **`shells/zshrc` probes installed locales before exporting one.** Prefer
  `en_US.UTF-8` when `locale -a` reports it, fall back to `C.UTF-8`, and leave
  the caller's locale untouched when neither exists.
- **`nvim/lazy-lock.json` is tracked** (NOT in `.gitignore`). This is how
  every machine ends up on the same plugin commits.
- **`ghostty/config` sets `window-save-state = default`** (NOT `always`)
  alongside `maximize = true`. It's not an oversight: on macOS
  `window-save-state = always` restores the last window geometry *after*
  `maximize` applies, overriding it — so the window wouldn't reliably open
  maximized. `default` keeps normal launches maximized while still allowing
  macOS OS-driven session restore (save-state is a no-op on Linux/GTK).
- **Ghostty clipboard read prompts, write/copy allows.** `clipboard-read = ask`
  keeps paste/read access consent-gated, while `clipboard-write = allow` and
  `copy-on-select = clipboard` preserve fast copying out of the terminal.
- **fzf in `shells/zshrc` is guarded by `command -v fzf`** and prefers
  `fzf --zsh` (fzf ≥ 0.48), falling back to share-dir key-binding files for
  older distro builds. The guard is load-bearing: `tests/shell/zsh_startup_test.sh`
  sources zshrc with no fzf installed and expects exit 0, so an unguarded
  `source <(fzf --zsh)` would break it. Installed by default by `install-deps.sh`.
- **starship `[directory]` shows the FULL path** (`truncation_length = 0`,
  `truncate_to_repo = false`) — not starship's default of 3 folders collapsed to
  the git-repo root. Intentional; raise `truncation_length` / flip
  `truncate_to_repo` to shorten. Guarded by `tests/starship/directory_test.sh`.
- **`[directory.substitutions]` are "icon + name"** (e.g. `Downloads` →
  download-glyph + `Downloads`), using Material Design Icon codepoints (U+F0xxx)
  verified present in Hack Nerd Font. Keep the folder name in the value so it
  stays readable if a glyph is missing on some machine, and **never let a value
  become a bare space** — an earlier encoding pass stripped three of these to
  U+0020, which rendered `~/Downloads`, `~/Music`, `~/Pictures` as a blank `~/`.
  The same `directory_test.sh` guards against whitespace-only values.
- **tmux colors track the canonical rose-pine/tmux theme.** Source:
  <https://github.com/rose-pine/tmux>, main variant. Role-based styles
  carry the colors. Map: status-style `fg=pine,bg=base`; window-status
  **UNSET** (inactive cells inherit from status-style);
  window-status-current `fg=gold,bold`;
  window-status-activity `fg=base,bg=rose`; pane-border `fg=hl_high #524f67`
  / pane-active-border `fg=gold`; message `fg=muted,bg=base`;
  message-command `fg=base,bg=gold`. **DO** set explicit
  `window-status-format "#I:#W#F"` and `window-status-current-format
  "#I:#W#F"` -- tmux's DEFAULT format contains a `#{?window_flags,...}`
  conditional that psmux v3.3.4 renders as a literal string in each cell.
  An earlier ship that used `setw -gu window-status-format` (unset, fall
  back to tmux default) lit that bug -- explicit `#I:#W#F` parses cleanly
  in both real tmux and psmux. Active windows use gold with bold weight because
  the foreground-only canonical theme was too subtle in dark terminals; bold is
  the smallest divergence that fixes legibility without breaking the palette.
  Inactive cells deliberately use `setw -gu window-status-style` (unset) so
  they fall through to `status-style` (pine on base) -- one source of
  truth, no duplicate fg/bg, and any future status-style tweak (e.g.
  transparency) ripples through cleanly.
  Status-left (iris-bold session + muted
  separator) and status-right (foam date + gold time) are our own
  customizations, palette-consistent. History/rejected attempts worth not
  re-attempting:
  (1) inactive `muted #6e6a86` -- 3.4:1 contrast, failed AA, illegible;
  (2) inactive `subtle #908caa` -- 5.5:1, borderline, still illegible;
  (3) inactive `text #e0def4` -- bright but flat, user wanted canonical;
  (4) iris inactive + gold-bold ON `bg=overlay #26233a` active block
  (commit 9cf13f8) -- added a bold + bg-block on top of canonical, user
  preferred plain canonical so the embellishment was reverted;
  (5) explicit inactive `fg=iris` (commit ee0d6c9) -- user wanted active
  to be the only styled cell, inactive should fall back to status-style;
  (6) relying on `window-status-current-style` alone (commits ee0d6c9 /
  d642a31) -- psmux v3.3.4 stores the option but does NOT apply it when
  rendering window cells. Only `#[fg=...]` INLINED in
  `window-status-current-format` actually paints the current window
  under psmux. Real tmux honors either; we ship the inline form so both
  render. **Do NOT** switch inactive to `dim` -- it is terminal/ConPTY-flaky
  under psmux and re-creates the legibility problem.
- **`stylua.toml` at repo root is load-bearing.** stylua reads ONLY its own
  config (`stylua.toml` / `.stylua.toml`) -- it does NOT respect
  `.editorconfig`. Its built-in defaults are `indent_type = "Tabs"` and
  `indent_width = 4`, which conflict with the rest of the repo. Without
  the repo-level `stylua.toml` declaring `Spaces` + `2`, conform.nvim's
  format-on-save would re-introduce tabs on every save of every .lua file,
  even after the `.editorconfig` flip from `indent_style = tab` to
  `space`. Both pieces are required: editorconfig controls nvim's buffer
  behavior while editing; stylua.toml controls what gets written back.
  Guarded by `invariants_test.sh` ("no tab-indented .lua").
- **lazygit move-commit uses uppercase J / K, and the config must be managed at
  `%LOCALAPPDATA%\lazygit\`.** Two separate gotchas wrapped together:
  1. **Config path:** lazygit v0.58 reads its config from
     `%LOCALAPPDATA%\lazygit\config.yml` (verified via `lazygit
     --print-config-dir`), NOT `%APPDATA%\lazygit\config.yml`. Earlier
     Windows config wiring targeted `%APPDATA%` -- the file existed but
     lazygit never loaded it, so EVERY custom binding looked dead.
     Chezmoi now targets LocalAppData. Asserted by
     `tests/migration/windows_apply_test.ps1`.
  2. **Binding:** `lazygit/config.yml` binds
     `keybinding.commits.moveDownCommit` / `moveUpCommit` to uppercase
     `J` / `K`. We intentionally do NOT use Ctrl+J / Ctrl+K: Ctrl+J is
     ASCII LF (0x0A), the same byte as Enter. Disambiguating requires
     Win32-input-mode (ConPTY DECSET 9001), modifyOtherKeys, or kitty
     keyboard protocol metadata. Windows Terminal sends it and lazygit's
     tcell/v3 can decode it, but psmux v3.3.4 does NOT relay the metadata
     to panes, so default Ctrl+J degrades to Enter inside psmux. Uppercase
     J / K are normal printable bytes and skip that entire transport
     problem.
     This is safe because lazygit v0.58.1 dispatches commits-context
     bindings before universal bindings, then falls through on
     ErrKeybindingNotHandled (`pkg/gui/keybindings.go:420-441` and
     `pkg/gui/keybindings.go:476-547`). So in the commits panel J / K fire
     moveDownCommit / moveUpCommit; elsewhere they still reach
     `universal.scrollDownMain-alt1` / `scrollUpMain-alt1`. Trade-off:
     while focused on the commits panel, Shift-J / Shift-K no longer scroll
     the main/diff window -- use PgUp / PgDn or Ctrl-U / Ctrl-D there.
     psmux no longer needs a root bind for this; `tmux/tmux.windows.conf`
     is back to its Windows shell / prediction / mouse overlay purpose.

## When you're about to make a change

1. Run the local test entry point (`make test` on macOS/Linux/WSL,
   `.\test.ps1` on Windows). Get baseline.
2. Make the change.
3. Update tests in the same diff (add a regression test for the new
   behavior; update an invariant assertion if you changed something this
   doc says is invariant).
4. Update this CLAUDE.md if you've changed any of the invariants or added a
   new common workflow.
5. Update `README.md` if you've changed the install path / install command.
6. Run the local test entry point again. Green.
7. Stage everything, commit.

If a test breaks: fix the cause, not the test. The test names are
deliberately worded as failure modes ("regression guard for …") — read them
carefully before "fixing" the test.

## Plan / history

The durable rationale belongs in this file, `README.md`, or the tests that
guard an invariant. Do not rely on private local plan files for public repo
maintenance.
