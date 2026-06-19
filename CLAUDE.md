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
├── lazygit/               config.yml + config.windows.yml (J/K + Windows Ctrl-G)
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
13. **Completion is fzf-tab over native compinit (Tab-driven, PowerShell-like).**
    `shells/zshrc` runs `compinit`, `zmodload`s `zsh/complist`, sets the
    `matcher-list` (case-insensitive) + `list-colors`, then — when `fzf` is on
    PATH — sets `zstyle ':completion:*' menu no` (fzf-tab draws the menu) and
    sources **fzf-tab** (`Aloxaf/fzf-tab`). Tab opens an fzf fuzzy picker over
    zsh's real context-aware completions; a single match completes directly.
    **Load order is load-bearing:** fzf-tab sources AFTER compinit + the
    completion `zstyle`s and BEFORE `zsh-autosuggestions`; then the fzf
    key-binding block (`source <(fzf --zsh)`) runs, which rebinds Tab to
    `fzf-completion`, so `zshrc` **reclaims Tab** with
    `bindkey '^I' fzf-tab-complete` AFTER that block (guarded on the widget
    existing). fzf keeps Ctrl-R / Ctrl-T / Alt-C. When fzf is absent, the block
    falls back to native `menu-select` (so `zsh_startup_test` stays green).
    `zsh-autosuggestions` (inline gray history, strategy `history completion`,
    `#908caa`) is the only other sourced zsh plugin. The repo-managed plugin
    root is fixed at `~/.local/share/dotfiles/zsh-plugins`; do not make it
    depend on `XDG_DATA_HOME` unless every producer, verifier, runtime source,
    uninstall path, and parity test changes together. Up/Down do prefix history
    search (PowerShell `HistorySearch` parity). Do NOT swap in an always-on
    as-you-type completion-list plugin — that paradigm rebinds Ctrl-R, fights
    `zsh-autosuggestions`, and is slow; fzf-tab + autosuggestions is the
    quiet-until-Tab PowerShell-PSReadLine analog and is the chosen design. Keep
    `shells/zshenv` minimal with `skip_global_compinit=1`. Guarded by
    `tests/shell/zsh_plugins_test.sh`.
14. **WSL is split-host by default.** Windows Terminal, Hack Nerd Font, and
    `win32yank` are Windows-host responsibilities. WSL installs the Linux CLI
    stack. Linux Ghostty and Linux fontconfig fonts in WSL require
    `--experimental-wsl-gui`; do not make them the default path again.
15. **Windows Terminal is installed by Windows setup.** Keep `wt` in the
    `install-deps.ps1` Scoop-first catalog (`extras/windows-terminal` ->
    `Microsoft.WindowsTerminal` -> `microsoft-windows-terminal`). If those
    MSIX-backed installs do not put `wt` on PATH, `Install-WindowsTerminal`
    falls back to the pinned portable GitHub release zip and verifies SHA-256
    before extraction. The app install and settings merge are separate code
    paths (`install-deps.ps1` vs chezmoi's `modify_` entry), and setup runs the
    merge by default. Opt out with `-SkipWindowsTerminalMerge`;
    `-MergeWindowsTerminal` is a retained no-op alias. The packaged `modify_`
    target still emits nothing on blank stdin so a bare `chezmoi apply` does not
    fabricate Store WT settings, but setup also handles portable WT after apply:
    it mirrors the packaged file when present, or seeds/merges the unpackaged
    path from `windows-terminal/settings.fragment.jsonc` when packaged settings
    are absent and portable WT is detected. The merge adds a fixed PowerShell 7
    profile (`pwsh.exe`) and promotes an empty or built-in Windows PowerShell 5.1
    `defaultProfile` to that profile; a custom user default is preserved.
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
19. **`treesitter_parsers` must exclude the 7 Neovim-bundled languages** (`c`,
    `lua`, `markdown`, `markdown_inline`, `query`, `vim`, `vimdoc`). Neovim ships
    matched built-in parser+query pairs for them — real parser `.so` files under
    the install prefix (e.g. `<prefix>/lib/nvim/parser/lua.so`) plus queries in
    `$VIMRUNTIME/queries/` — and that parser dir is on the runtimepath. Letting
    nvim-treesitter install its own (regenerated, sometimes stale-cached) parser
    overrides the built-in and breaks the bundled query (`E5113: Invalid field
    name "operator"` on lua). Excluding them stops future installs, but the
    config ALSO **purges** any nvim-treesitter-managed `parser/<bundled>.so`
    already present (a leftover from an older config, or restored from a CI
    cache, still overrides the built-in). The purge is **scoped to
    `stdpath('data')`** (nvim-treesitter installs under `…/site`; the install
    prefix does not) — an unscoped delete would wipe Neovim's OWN built-in
    parsers. `c` and `vim` are bundled but not auto-started, so the config starts
    them via `nvim_bundled_started_here = { "c", "vim" }`. See "Add a treesitter
    parser". Guarded by `treesitter_spec.lua`, `language_smoke_spec.lua`, and the
    Tier-2 `lsp_smoke.lua` runtime preflight.

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

`nvim-treesitter` tracks the upstream `main` rewrite and therefore requires
Neovim 0.12+. The legacy `require("nvim-treesitter.configs").setup` API is not
available here. Parser installation runs `tree-sitter generate` and
`tree-sitter build`, so fresh machines need the standalone `tree-sitter` CLI
plus a real C compiler before `:TSUpdate` can succeed.

1. Add the parser name to `treesitter_parsers` in
   `nvim/lua/plugins/treesitter.lua`.
2. If the parser name differs from Neovim's filetype, add an entry to
   `parser_filetype_aliases` so the `FileType` autocmd can call
   `vim.treesitter.start()` for real buffers.
3. Add it to `required` in `tests/nvim/spec/treesitter_spec.lua`.
4. If the parser has unusual generated sources, verify it through
   `:TSUpdate <parser>` on a machine with `tree-sitter` and a compiler on PATH.

> **Never install a Neovim-bundled language with nvim-treesitter.** Neovim 0.12
> ships matched built-in parser+query pairs for **c, lua, markdown,
> markdown_inline, query, vim, vimdoc** — the parsers are real `.so` files under
> the install prefix (e.g. `<prefix>/lib/nvim/parser/lua.so`, which is on the
> runtimepath; note this is NOT `$VIMRUNTIME/parser/`, which is empty), and the
> matched queries live in `$VIMRUNTIME/queries/`. nvim-treesitter `main`
> *regenerates* a parser from `grammar.js` via `tree-sitter generate` at install
> time and drops it under `stdpath('data')/site/parser/`, which (earlier on the
> runtimepath) **overrides** the built-in. When that regenerated/cached parser's
> field table drifts from Neovim's bundled query, every buffer of that filetype
> throws -- e.g. a lua parser generated before tree-sitter-lua commit `d760230`
> lacks the `operator` field that Neovim's bundled `lua/highlights.scm`
> references at line 74, so `ftplugin/lua.lua`'s auto `vim.treesitter.start()`
> raises `E5113: Invalid field name "operator"` on every lua file. This actually
> happened in e2e (commit `c3042df`, all three OSes). So `treesitter_parsers`
> **excludes** those 7, AND the config purges any nvim-treesitter-managed
> override already on disk — scoped to `stdpath('data')` so Neovim's own
> install-prefix parsers are never deleted. Neovim auto-starts treesitter for
> lua/markdown/vimdoc/query via its runtime ftplugins; `c` and `vim` are bundled
> but NOT auto-started, so the config starts them itself via
> `nvim_bundled_started_here = { "c", "vim" }` using the matched built-in parser.
> Guarded by `tests/nvim/spec/treesitter_spec.lua`, the per-language
> `language_smoke_spec.lua` (`bundled` rows assert the parser is *absent* from
> the install list), and the Tier-2 `lsp_smoke.lua` runtime override preflight.

**Recorded decision — `main` vs `master` branch of nvim-treesitter.** We track
`main` (the upstream rewrite: regenerates parsers from `grammar.js` via the
`tree-sitter` CLI at install, requires Neovim 0.12+) on purpose — it is the
canonical forward direction, and Neovim core increasingly owns the bundled
languages it ships, which is exactly the division the exclusion above enforces.
The legacy `master` branch is internally self-consistent (it ships precompiled
parsers *and* its own matched queries, so there is no core-vs-plugin query split
to drift, and it is therefore less exposed to the `operator`/E5113 class of bug)
**but it is deprecated and sunsetting**, so we do NOT use it. The robustness we
would have gotten from `master` is instead provided by invariant 19's strict
"one owner per language" rule (core owns its bundled 7, nvim-treesitter installs
the rest). Do not switch branches to dodge a parser/query mismatch — keep the
ownership boundary instead.

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
literals in status/borders), ghostty/config (`theme = Rose Pine` -- forced dark
on every platform, NOT the adaptive `dark:,light:` split, to match the dark
stack),
windows-terminal/settings.fragment.jsonc (`schemes` + `themes`),
shells/powershell_profile.ps1 (PSReadLine `-Colors` for syntax, `Selection`,
the version-gated prediction colors, and `$PSStyle.FileInfo.Directory` for `ls`
directory color).

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
make ci              # full local pre-PR gate: test + Renovate + migration
make test            # current-host fast suite
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
problem. The startup budget itself is strict, but the spec measures up to three
warm starts and accepts the fastest run; this filters scheduler/filesystem
outliers while still failing a consistently slow production init.

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
`tests/static/editorconfig_check.sh` feeds `editorconfig-checker` a pruned
per-file list instead of using the checker's recursive walker. Keep generated
plugin caches under `tests/.cache/` out of that list; Neovim tests clone real
plugin repositories there, and those vendored files are not repo formatting
surface.
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

`test.yml` remains the fast cross-platform suite and now also owns Renovate
schema validation and the `chezmoi-parity` migration gate. Warnings are treated
as failures where the tools expose them cleanly: shellcheck exits nonzero,
PSScriptAnalyzer runs at `Warning,Error`, yamllint/parser checks are part of
`make test-static`, `scripts/validate-renovate.sh` fails when `npx` is missing
under `CI=true`, and Windows CI treats missing test dependencies as fatal.
PSGallery module installs in Windows CI use bounded retries for transient
gallery lookup failures; the final miss still fails the job.

`e2e-install.yml` is the required real-install gate. The jobs cover different
install paths, not symmetric container platforms:

- `e2e containers / ubuntu-24.04` runs an `ubuntu:24.04` container on an Ubuntu
  runner with `DOTFILES_SKIP_BREW_BOOTSTRAP=1`, creates a non-root user, runs
  real `install-deps.sh --all` (native `apt`, no Linuxbrew), then applies
  configs with chezmoi and asserts tool presence, Neovim >= 0.12, lazygit, zsh
  plugin files under `~/.local/share/dotfiles/zsh-plugins`, config content
  matching the repo sources, and the Neovim
  directory resolving into repo `nvim/`.
  This is intentionally **not** a devcontainer. It stays because hosted Ubuntu
  has Linuxbrew available, so the container is the only automated proof of the
  clean-image native `apt` path: pinned Neovim tarball install, pinned lazygit
  release install, fixed-root zsh plugin install, `fd-find` -> `fd` shim, and apt
  fallbacks. Scope is intentionally **Ubuntu only** (the supported Linux/WSL2
  proxy target).
  Re-adding another distro requires both a matrix entry in `e2e-install.yml` and
  a matching root-prep branch in `tests/ci/container-e2e.sh`.
- `setup.sh / ubuntu-24.04`, `setup.sh / macos-15`, and
  `setup.ps1 / windows-2025` run the real public setup entry points, apply
  configs through chezmoi in Phase 2, and then rerun Lazy/Mason headless sync.
  They explicitly fail if setup skips Phase 3-4, emits a `FAIL:` marker, or
  Mason did not install expected tools. After the Mason sync they also run the
  **Tier 2 language smoke** (`tests/nvim/lsp_smoke.lua`, gated on
  `DOTFILES_LSP_SMOKE=strict`): against the production init it asserts every
  `treesitter_parsers` entry is one nvim-treesitter `main` supports
  (`get_available()`/`get_available(4)` — the jsonc "unsupported language"
  catcher) and that each fixture's LSP attaches. Non-gated servers are strict on
  every OS; `powershell_es` is enforced only on Windows (pwsh + the PSES bundle)
  and skips cleanly on Unix. The fast `make test-nvim` runs Tier 1 only
  (`tests/nvim/spec/language_smoke_spec.lua` + `tests/nvim/language_matrix.lua`):
  filetype + conform-formatter + parser-in-install-list per fixture.
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
  WSL after running `.\setup.ps1 -All` on Windows. Windows Terminal settings
  handling is default-on: packaged WT is merged when its settings file exists,
  and portable WT is seeded or merged at the unpackaged path when the packaged
  file is absent.

Local clean-machine harnesses live in `tests/greenfield/README.md`; keep them
manual VM/Sandbox tools and do not add them to the headless CI matrix.

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
checks unless asked. If live GitHub has duplicate rulesets with the same
protected name, the script fails closed instead of choosing one; delete the
duplicate live ruleset and re-run.

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

Several pins are also **mirrored across files** (nvim version/SHA in
`install-deps.sh`, `test.yml`, and `tests/shell/install_nvim_linux{,_fail}_test.sh`;
zsh plugin tag/commit in `install-deps.sh`, `home/.chezmoiexternal.toml.tmpl`, and
the verify-pins run-script). A Renovate bump touches one surface and strands the
mirrors. `tests/static/pin_consistency_test.sh` is the canonical drift guard — it
fails CI when any mirror disagrees. When you bump a pin, update every mirror and
keep that test green.

Validate `renovate.json` locally with Renovate's own schema validator, not just
`jq`: `make validate-renovate`, or `make ci` for the full pre-PR bundle. That
target runs Renovate under Node 24 because Renovate's `engines.node` supports
the Node 24 LTS line; running the validator directly under an unsupported
odd/current host Node such as 25.x emits `EBADENGINE`. Do not silence that
warning with npm config. Switch runtimes or use the repo target, which shells
into Node 24 before running:

```bash
npx --yes --package node@24.11.0 -- bash -c \
  'npm exec --yes --package renovate@43.230.1 -- renovate-config-validator --strict renovate.json'
```

`tests/static/json_lint.sh` only checks JSON syntax. Its JSONC path strips `//`
comments with a string-aware Python pass so URL values like `https://...` and
quoted `//` text survive before `jq` parses the result. The Renovate validator
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
  alias for `--skip-config` / `-SkipConfig`. POSIX dry-run renders temporary
  chezmoi config/expected files and must clean them on failure; Homebrew
  `shellenv` output is evaled only after the `brew shellenv` command succeeds.
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
  merge but does not create setup's backup. After a real non-dry-run apply,
  setup also best-effort copies the merged MSIX settings file to the unpackaged
  portable-WT path `%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json`.
  If the MSIX settings file is absent but portable WT is detected, setup seeds
  or merges that unpackaged file directly from
  `windows-terminal/settings.fragment.jsonc`. Both portable paths are skipped
  when `-SkipWindowsTerminalMerge` is passed. Store WT ignores the unpackaged
  file. The managed WT profile is an explicit fixed-GUID `pwsh.exe` profile
  named `PowerShell 7`; do not rely on WT's dynamic PowerShell 7 profile GUID
  being present. Do not backport the theme/profile to Windows PowerShell 5.1:
  this repo installs/configures PS7 and 5.1 lacks the PSReadLine ListView and
  `$PSStyle` behavior used by the managed profile.
  The legacy `-MergeWindowsTerminal` switch remains accepted as a no-op alias.
- **lazygit config paths are OS-specific.** On macOS, lazygit v0.58 reports
  `~/Library/Application Support/lazygit` from `lazygit --print-config-dir`;
  on Linux/WSL it uses `~/.config/lazygit`; on Windows it uses
  `%LOCALAPPDATA%\lazygit`. POSIX hosts use `lazygit/config.yml`; native
  Windows renders `lazygit/config.windows.yml` so psmux users get a Ctrl-G
  return/cancel binding without changing macOS/Linux Esc behavior. Keep the
  chezmoi templates, README, and tests aligned with those real read paths.
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
  symlink or fragment-only replacement. WT opens **maximized** (`launchMode`,
  not fullscreen) with a **visible** scrollbar (`scrollbarState` in
  `profiles.defaults`) and defaults to the fixed `PowerShell 7` profile only
  when `defaultProfile` is empty or still the built-in Windows PowerShell 5.1
  default; a custom default is left alone. Adding a new unconditional top-level
  scalar fragment key requires FOUR edits in lockstep or the
  `windows_apply_test.ps1` deep-compare fails: the fragment (+ its
  `home/.chezmoitemplates` mirror),
  `home/.chezmoitemplates/windows-terminal/merge-settings.ps1`, the test mirror
  `Invoke-ExpectedWindowsTerminalMergeOnly`, and `$script:ManagedGlobals`.
  Conditional keys like `defaultProfile` need the same helper/test mirror but are
  intentionally excluded from `$script:ManagedGlobals`. `profiles.defaults` is
  replaced wholesale, so keys inside it (e.g. `scrollbarState`) need no
  merge-template change. The nvim tree is intentionally NOT
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
  and Windows copy-mode + WT merge parity. `parity_gate.sh` runs
  `chezmoi doctor` against a temporary copy of `home/`, not the Git checkout,
  so the migration proof is not coupled to local `git status` performance or
  unrelated working-tree noise.
- **`install-deps` provisions chezmoi itself.** Unix setup installs `chezmoi`
  with Homebrew when brew is the selected manager; native Linux without brew
  downloads the pinned `CHEZMOI_VERSION` GitHub release tarball, verifies the
  adjacent SHA-256 constant, and installs the binary into `~/.local/bin`.
  Windows setup installs `chezmoi` through the normal Scoop-first `$Catalog`
  fallback chain (`scoop` -> `winget` -> `choco`). This keeps `make chezmoi`
  usable after full setup before the Phase 2 chezmoi apply.
- **`install-deps` shows a true pre-bootstrap dependency table before the
  one-shot install prompt.** Package-manager detection runs first, but Scoop /
  Homebrew bootstrap runs only after the table and prompt. The table includes a
  package-manager row (`scoop` on Windows, detected POSIX manager on Unix),
  uses the same presence checks as the installer, best-effort `--version`
  probes, and still leaves all actual install/skip decisions to the existing
  per-tool functions.
- **`--update` is a scoped drift-edge refresh, not a repo update.**
  `setup.sh --update` / `setup.ps1 -Update` run only `install-deps --update`
  and `nvim --headless +MasonToolsUpdate +qa`. They skip git pull, chezmoi
  apply, Lazy sync, and Lazy update. `install-deps --update` updates only
  present catalog tools through scoped per-package manager commands
  (`brew upgrade <formula>`, native Linux package upgrade commands, or
  `scoop update <pkg>` after one manifest refresh). It must not run blanket
  upgrades such as `brew upgrade`, `apt upgrade`, or `scoop update *`, and it
  must not touch pinned direct downloads, PSFzf, `lazy-lock.json`, or configs.
  On native Linux without Linuxbrew or Alpine/apk, `nvim`, `lazygit`,
  `starship`, and `tree-sitter` are pinned direct-download binaries and stay
  out of the update path. Linuxbrew updates them through
  `brew upgrade <formula>`; Alpine updates its native `neovim`, `lazygit`,
  `starship`, and `tree-sitter` packages through apk.
- **A C compiler is installed so LuaSnip can build `jsregexp`.** Without one,
  the nvim Lazy build prints "No C compiler found" and `jsregexp` is skipped
  (LuaSnip still works, minus JS-regex snippet transforms). POSIX installs the
  native toolchain (`build-essential` / `gcc` / `base-devel`), only if none of
  `cc`/`gcc`/`clang`/`zig`/`cl` is already present. On Windows a clean machine
  has none, so `install-deps.ps1` carries **`zig`** in `$Catalog` (LuaSnip
  detects `zig cc`); it is skipped if already installed like any other tool.
- **nvim-treesitter main needs both `tree-sitter` and MSVC on Windows.**
  The main-branch installer shells out to the standalone `tree-sitter` CLI and
  then builds parsers through the Rust `cc` crate. `install-deps.sh` provisions
  the CLI per OS: macOS/Linuxbrew use the Homebrew **`tree-sitter-cli`** formula
  (NOT `tree-sitter` — Homebrew split the formula, and `tree-sitter` now installs
  only `libtree-sitter` with no CLI binary, so a fresh machine would be left
  without the `tree-sitter` executable; `tree-sitter-cli` ships the 0.26.x
  binary). The `PKG_TABLE` brew column for `tree-sitter` is therefore
  `tree-sitter-cli`, while `binaries_for` still probes for the `tree-sitter`
  binary. Native Linux/WSL installs a pinned `tree-sitter/tree-sitter` release
  asset (v0.26.9) into `~/.local/bin` with SHA-256 verification. `install-deps.ps1` installs the CLI through the Scoop
  `tree-sitter` manifest first and falls back to `npm install -g
  tree-sitter-cli` after Node is present. Windows compiler support is separate:
  `install-deps.ps1 -All` auto-installs Visual Studio 2022 Build Tools with the
  `Microsoft.VisualStudio.Workload.VCTools` workload through winget or choco,
  then falls back to Microsoft's official `vs_BuildTools.exe` bootstrapper with
  the same workload. Scoop does not carry VS Build Tools, so this is the
  deliberate exception to the Scoop-first catalog rule. A failed winget/choco
  pass is not final; a failed package-manager-plus-bootstrapper pass records an
  `InstallFailures` entry so `-All` cannot report success without MSVC.
  `setup.ps1` imports the VS DevShell into the current process before headless
  `Lazy! sync`, so `tree-sitter build` inherits `cl.exe`, `INCLUDE`, and `LIB`.
  Do not put the DevShell import in the
  PowerShell profile. Zig stays installed for LuaSnip `jsregexp`, but do not
  wire zig into nvim-treesitter main: the old `master` branch could use it,
  while main emits MSVC-style `cc` crate flags that require MSVC. Ad-hoc
  `:TSUpdate` parser rebuilds on Windows should run from a "Developer
  PowerShell for VS" shell or after rerunning setup.
- **nvim-treesitter installer drift must not disable highlighting.** A stale
  lazy.nvim cache can keep `nvim-treesitter` on the frozen `master` API while
  this repo expects the `main` rewrite. In that state
  `require("nvim-treesitter").install` and `.indentexpr` are absent. The config
  must warn and continue registering the `FileType` autocmd that calls
  `vim.treesitter.start()`; never let parser auto-install API drift abort buffer
  highlighting. The recovery path is `:Lazy! sync` followed by `:TSUpdate`, or
  rerun `setup.ps1` on Windows so VS DevShell is imported before parser builds.
- **`uninstall.sh` / `uninstall.ps1` are greenfield teardown tools, not purge.**
  They enumerate targets with `chezmoi --source <repo>/home managed --path-style
  absolute`, remove only repo-owned symlinks or byte-identical Windows
  copy-mode files, restore newest bootstrap-style `<target>.bak.<timestamp>`
  backups by default, and leave chezmoi's own config/state alone. Windows
  Terminal `settings.json` is never deleted because the merge is idempotent but
  not invertible; use the printed backup path for manual restore if needed.
  Dry-run mode must also leave empty external parent directories in place; it
  prints `would:` lines only and does not prune `~/.local/share/dotfiles`.
- **Starship binary install paths differ by OS.** Homebrew owns
  macOS/Linuxbrew, Alpine uses the native `starship` apk package, Windows setup
  installs it through Scoop/winget/choco, and other native Linux/WSL hosts
  without brew use a pinned Starship GitHub release tarball with SHA-256
  verification.
- **lazygit binary install paths differ by OS.** Homebrew owns macOS/Linuxbrew,
  Alpine uses the native `lazygit` apk package, Windows setup installs it
  through Scoop/winget/choco, and other native Linux/WSL hosts without brew use
  a pinned GitHub release tarball with SHA-256 verification.
- **`install-deps.ps1` prefers scoop, then falls back across managers
  per tool.** `Install-One` builds an ordered candidate list (scoop → primary →
  winget → choco) of managers that are installed AND carry the package, and
  tries each until one succeeds — so a winget `No package found` (exit
  `-1978335212`) no longer dead-ends a tool. scoop carries the cataloged
  CLI/terminal tools, including Windows Terminal as `wt`
  (`extras/windows-terminal`). Windows Terminal is special-cased through
  `Install-WindowsTerminal`: it reuses the catalog manager chain first, then
  falls back to the pinned portable `microsoft/terminal` zip when MSIX-backed
  manager installs fail to register `wt`, such as in Windows Sandbox or
  MSIX-restricted hosts. The portable fallback is SHA-256 verified before
  extraction and adds `%LOCALAPPDATA%\Programs\WindowsTerminal` to both the
  current process PATH and persistent User PATH.
- **`Update-ScoopTool` is the only scoop update path.** It is intentionally
  single-package and consent-gated for the PowerShell 7 keep-latest path; never
  replace it with `scoop update *` or another blanket scoop upgrade. Failed
  manifest refreshes or package updates append to `InstallFailures`, so update
  mode exits nonzero when a scoped refresh did not actually succeed.
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
  PowerShell tests dot-source `install-deps.ps1` on macOS too, where
  `USERPROFILE` can be empty; use `Get-ScoopRoot` instead of direct
  `Join-Path $env:USERPROFILE` for Scoop paths so source-only tests stay
  portable.
  **`Ensure-ScoopBuckets` installs `git` (from the bucket-less `main` bucket)
  BEFORE adding `extras`/`nerd-fonts`.** `scoop bucket add` git-clones the bucket
  repo, so on a truly fresh machine (Windows Sandbox / clean install) the adds
  fail with "Git is required for buckets" because git is not installed yet at
  that point. CI runners ship git preinstalled, which hid this on every hosted
  job -- it only surfaced in a real greenfield Sandbox run. Guarded by the
  "installs git before adding scoop buckets when git is absent" Pester test in
  `InstallDeps.Tests.ps1`.
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
- **Apt `update` is best-effort, decoupled from `install`.** The apt arms of
  `pm_install`, `native_linux_pm_install`, and `pm_update` run `apt-get update`
  on its own line (`|| warn`), then ALWAYS run `apt-get install`. Do NOT restore
  the `apt-get update && apt-get install` coupling: a single flaky `update` (an
  unreachable third-party PPA, an expired repo key, a transient mirror outage)
  would short-circuit the `&&` and skip the install entirely, even for packages
  already in the local apt cache. Guarded by `tests/shell/apt_update_resilience_test.sh`.
- **Alpine installs Neovim through `apk`.** The official Neovim Linux tarball
  targets glibc systems, so Alpine uses its native `neovim` package instead;
  e2e still enforces the repo's Neovim >= 0.12 floor.
- **`install-deps.sh` prompts for the notes/Obsidian vault** and persists
  `export NOTES_VAULT=…` to `~/.zshrc.local` (sourced by `zshrc`, read by
  `util/notes_path.lua`). The prompt is tty-gated (skipped under `--all` /
  piped / `--dry-run`); the write logic is split into `persist_notes_vault` so
  `tests/shell/notes_vault_test.sh` can exercise it without a tty.
- **`install-deps` can install VS Code + the Rose Pine theme.** Both installers
  offer VS Code (macOS brew cask; Linux snap/flatpak/manual; Windows
  winget/scoop/choco). Then, **only if `code` is detected**, they install the
  `mvllow.rose-pine` extension, set `workbench.colorTheme` to "Rosé Pine", and
  set `editor.fontFamily` plus `terminal.integrated.fontFamily` to
  `'Hack Nerd Font', Consolas, monospace`.
  **Forcing dark is load-bearing.** Setting only `workbench.colorTheme` is NOT
  enough: when `window.autoDetectColorScheme` is `true` (Settings Sync, an
  imported profile, or a future VS Code default can enable it) VS Code IGNORES
  `colorTheme` and resolves the theme from
  `workbench.preferredDark/LightColorTheme` (defaulting to Dark Modern /
  "Dark 2026") — which is exactly why a fresh Windows install showed Dark despite
  the setting. So both installers ALSO set `window.autoDetectColorScheme` to a
  real JSON boolean `false` (NOT the string `"false"`, which VS Code ignores) and
  point BOTH `preferredDark`/`preferredLight` slots at the same dark "Rosé Pine"
  so no OS-scheme combination yields a light theme (same forced-dark rule as
  Ghostty; see `tests/MANUAL.md`). In the ps1, the boolean flows through a `Raw`
  spec flag + `ConvertTo-VSCodeSettingJson` so the text write-paths emit a bare
  literal; the clean-JSON merge path stores a native `[bool]`.
  The theme setter (`set_vscode_theme` in sh, tested by `vscode_theme_test.sh`;
  `Set-VSCodeTheme` in ps1, tested by `InstallDeps.Tests.ps1`) uses jq /
  `ConvertFrom-Json` for strict JSON and a comment-aware scanner for JSONC. The
  JSONC fallback edits only top-level keys, ignores comments/strings and nested
  objects, preserves the dominant line ending, and creates
  a non-colliding `settings.json.bak.<timestamp>[.n]` backup through the shared
  `unique_backup_path` helper before writing.
  **Encoding is load-bearing on Windows.** The theme value must keep its accented
  é to match the extension's label "Rosé Pine", but a literal `é` byte in
  `settings.json` is fragile under Windows PowerShell 5.1: its `Get-Content`
  default is the ANSI code page, so reading a UTF-8 `é` (`C3 A9`) back as ANSI
  yields two chars (`Ã©`) that re-encode to `C3 83 C2 A9` on the next write —
  the double-encoded "RosÃ© Pine" mojibake VS Code cannot resolve, so it silently
  falls back to the default dark theme. So the ps1:
  (1) reads with `Get-Content -Raw -Encoding utf8` (lossless read/modify/write on
  5.1 and 7 — so any pre-existing non-ASCII content round-trips intact instead of
  double-encoding); and (2) writes the *managed* theme value as a pure-ASCII
  `\u00e9` JSON escape via `ConvertTo-AsciiJson` (escapes every char > `0x7F` to
  `\uXXXX`) on EVERY write path — new-file, JSONC editor (through
  `ConvertTo-JsonStringLiteral`), and the clean-JSON merge (wrapping
  `ConvertTo-Json`, which only PS 7 leaves un-escaped). So the colorTheme /
  preferred-theme values VS Code must resolve are encoding-immune regardless of
  code page, and a rerun self-heals an already-double-encoded value. (Scope note:
  new-file and merge emit a wholly pure-ASCII file; the JSONC editor only
  ASCII-normalizes the values it inserts/replaces, so unrelated non-ASCII in a
  user's own comments/values is preserved verbatim — losslessly, thanks to the
  UTF-8 read — rather than rewritten as escapes, so we never mangle comment text.)
  Guarded by `Test-FileIsPureAscii`, the non-ASCII-comment round-trip JSONC test,
  and `ConvertTo-AsciiJson` unit tests in `InstallDeps.Tests.ps1`. The `.ps1`
  source itself still stays pure ASCII (invariant): the in-memory label is built
  with `[char]0xE9`. The sh side keeps the literal é (macOS/Linux are
  UTF-8-native, so the ANSI-read hazard does not exist there).
  **KNOWN ISSUE (open, shipped as-is 2026-06-16):** the encoding hardening above
  is correct and Codex-reviewed, but it did NOT resolve the field symptom — on at
  least one Windows machine VS Code still opens in default Dark even after a
  rerun + full restart with `workbench.colorTheme` correctly set. So encoding was
  a real latent bug but is NOT (or not the only) cause of the auto-apply failure.
  Do NOT re-investigate the encoding angle. Leading unexplored suspects, in
  rough priority: (1) Settings Sync pulling cloud settings that override the local
  file; (2) a non-default VS Code profile (settings written to the default
  profile dir while VS Code runs another); (3) the `mvllow.rose-pine` extension
  not yet activated when `colorTheme` is first read (needs an Extensions reload /
  second launch); (4) a workspace `.vscode/settings.json` override. Confirm which
  profile is active and whether Sync is on before touching code again.
- **Direct network executables are pinned or explicitly allowlisted.** New
  installer code must prefer release artifacts with adjacent SHA-256
  verification over fetched installer scripts. If a mutable bootstrap script is
  unavoidable, it must be a named trust root with rationale in
  `tests/static/supply_chain_remote_execution_test.sh`; the current installer
  trust root is Scoop's official installer. Homebrew bootstrap is downloaded
  from a pinned installer commit and SHA-256 verified before execution.
  Recommended setup docs use `git clone` plus local `setup`, not raw remote
  setup script execution from the current default branch.
- **Direct GitHub downloads are pinned and SHA-256 verified.** `install-deps.sh`
  verifies the pinned Homebrew installer script, Neovim Linux tarballs,
  native-Linux chezmoi tarballs, lazygit Linux tarballs, Starship Linux
  tarballs, tree-sitter CLI Linux archives, and Hack Nerd Font zip before extraction;
  CI also verifies the pinned chezmoi Linux, macOS, and Windows release
  archives used by the parity jobs;
  `install-deps.ps1` verifies the pinned Hack.zip before registering fonts and
  the pinned Windows Terminal portable zip before extracting the fallback
  install. POSIX helpers that unpack into `mktemp -d` install a cleanup trap
  immediately after creating the directory, so failure paths do not leak
  archives or partial extracts. A Hack.zip checksum mismatch records a `FAIL:`
  install marker and does not extract. A successful Windows font install broadcasts
  `WM_FONTCHANGE` best-effort so Windows Terminal can re-enumerate fonts without
  making setup depend on that notification. The CI workflows also pin and
  verify their direct GitHub downloads.
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
  `tests/shell/wsl_gui_tools_test.sh`, `tests/shell/lazygit_install_test.sh`,
  `tests/shell/starship_linux_install_test.sh`,
  `tests/shell/treesitter_cli_test.sh`, and `tests/shell/zsh_plugins_test.sh`.
  Renovate can open version/ref bumps for these constants and for the CI
  cargo-binstall installer commit and Renovate validator package/runtime pins,
  but it cannot recompute adjacent SHA-256
  values or verify tag commit IDs; leave CI red until a human has reviewed the
  download/ref and updated the adjacent constant. The
  `CHEZMOI_VERSION`, `STARSHIP_VERSION`, and
  `TREE_SITTER_CLI_LINUX_VERSION` custom managers follow the lazygit shape:
  Renovate may bump the version constants, while their adjacent SHA-256 values
  remain context only. In
  `renovate.json`, direct-download SHA-256 values must be matched as context
  only, not named `currentDigest`, otherwise Renovate will schedule same-version
  digest updates for checksums it cannot actually resolve.
- **Both installers open with an "install EVERYTHING?" prompt.** Interactive
  runs that didn't pass `--all`/`-All` get one upfront question; answering yes
  flips `YES_ALL`/`$All` so the rest runs with no per-item prompts. Skipped when
  `--all`/`--dry-run` was passed or there's no tty (so noninteractive setup and the CI
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
  `~/.tmux.conf` target). Install-Psmux is NOT in the `$Catalog` because scoop needs a custom
  psmux bucket URL; it passes that URL through `Add-ScoopBucketSafe`, installs
  the explicit `psmux/psmux` manifest to avoid ambiguous Scoop bucket matches,
  then falls through to winget / choco if the bucket clone or scoop install
  fails.
- **The clipboard `if-shell` probes are POSIX-ONLY (`tmux/tmux.posix.conf`) —
  they FREEZE psmux.** `if-shell 'command -v pbcopy …'` spawns a shell at
  config-**load** time. Under psmux/ConPTY on Windows that shell never returns:
  the pane initializes but never finishes rendering, the orphaned probe shells
  pin CPU, and a `conhost.exe` leaks per session (plus an "auth failed, retry
  works" stale-server artifact). So the five native-CLI probes
  (pbcopy→wl-copy→xclip→xsel→win32yank) live in a **POSIX-only overlay**
  `tmux/tmux.posix.conf`, managed as `~/.tmux.posix.conf` by chezmoi on Unix/WSL
  and **ignored on Windows** in `home/.chezmoiignore` (mirror of how
  `.tmux.windows.conf` is ignored off-Windows). The cross-platform
  `tmux/tmux.conf` keeps only a psmux-safe OSC52 baseline
  (`bind -T copy-mode-vi y send -X copy-pipe-and-cancel`, no shell) and
  `source-file -q "~/.tmux.posix.conf"` — a silent no-op on Windows where the
  file is absent (same proven mechanism as the Windows overlay source). On POSIX
  the overlay re-binds `y` to the native CLI when one exists. This is the
  canonical "guard the block, don't fork the config" fix. **Invariant:** the
  cross-platform `tmux/tmux.conf` (and its `home/dot_tmux.conf` mirror) must
  contain NO command-position `if-shell` — guarded by `invariants_test.sh`
  ("psmux freeze guard") and `tests/tmux/option_test.sh` (baseline binds with no
  shell probe; the overlay rebinds `y`→pbcopy on macOS), plus a Windows apply
  absence assertion (`tests/migration/windows_apply_test.ps1` +
  `windows_roundtrip_test.ps1`: `~/.tmux.posix.conf` must NOT exist on Windows).
  Native-Windows copy-mode `y` is rebound to built-in `clip.exe` in
  `tmux/tmux.windows.conf` (runs at copy time, not load time — psmux-safe — and
  does not depend on unverified OSC52-through-ConPTY). One-time cleanup on an
  already-broken Windows box (orphaned servers/conhost + any stale overlay from
  prior loads):
  `Remove-Item -LiteralPath "$HOME\.tmux.posix.conf" -Force -ErrorAction SilentlyContinue; taskkill /F /T /IM psmux.exe`
  then reopen the terminal.
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
- **psmux bare Escape is NOT patched in tmux config.** psmux v3.3.x has an
  upstream Esc-forwarding bug that breaks modal TUIs such as lazygit. A
  Windows-only root-table `Escape -> send-keys esc` binding was tested and did
  not reliably reach lazygit, so do not re-add it. The supported native-Windows
  lazygit workaround lives in `lazygit/config.windows.yml`: it binds
  `keybinding.universal.return` to `<c-g>`, which makes lazygit's `?`
  keybindings view show Ctrl-G as the working return/cancel key inside psmux.
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
- **Prediction colors are version-gated and isolated from the syntax `-Colors`.**
  The ListView prediction (the inline + dropdown history suggestions — our
  "fzf-like" history UI) defaulted to a near-background grey that is invisible on
  Rose Pine. The fix sets `InlinePrediction` (`#908caa`, PSReadLine ≥ 2.1.0) and
  `ListPrediction` (`#ebbcba`) / `ListPredictionSelected` (`#f6c177`) (≥ 2.2.0).
  These are applied in SEPARATE `Set-PSReadLineOption -Colors` calls, NOT folded
  into the main syntax hashtable: an unknown color key throws and would drop the
  WHOLE hashtable, so on an old PSReadLine the syntax colors must not depend on a
  prediction key existing. `Selection` is also isolated; it is a `gold #f6c177`
  FOREGROUND SGR because the owner wants the selected MenuComplete option gold.
  Known, UNAVOIDABLE caveat (do not "fix" it by removing the gold -- the owner
  explicitly wants the gold option): PSReadLine uses the ONE `Selection` color
  for BOTH the highlighted MenuComplete item AND the completion suffix it inserts
  into the command line while navigating (the ".exe" of `lazygit.exe`), so that
  suffix also shows gold until accepted. There is no separate color for the two.
  Do NOT use a *dark* `Selection` foreground (the original dark-on-dark bug). Do
  NOT set `ListPredictionTooltip`.
- **PowerShell Starship init is cached without `Invoke-Expression`, and cache
  publication is race-safe.** (Race-safe, not a single atomic syscall: on the
  Windows PowerShell 5.1 host this profile also supports, `Move-Item -Force` is
  replace-via-delete-then-rename, not the .NET atomic-replace overload. The
  no-torn-reads guarantee comes from writing to a private temp first, and a lost
  race degrades to the existing cache.) `Confirm-StarshipInitScript` still writes
  `%LOCALAPPDATA%\starship.ps1` (or the cross-platform cache dir) and dot-sources
  it because `Invoke-Expression (& starship init powershell)` is banned. When the
  cache is missing or older than `starship.toml`, `Publish-StarshipInitScript`
  writes UTF-8 no-BOM to a same-directory `starship.ps1.<pid>.<guid>.tmp`, then
  `Move-Item -Force`s it into place. If another shell wins the race and the move
  fails, the profile falls back to the existing cache instead of warning.
  `Import-StarshipInitScriptWithRetry` tolerates a short read lock while another
  tab is publishing.
- **fzf is unified across shells via PSFzf.** `install-deps.ps1` installs `fzf`
  (catalog: winget `junegunn.fzf` / choco / scoop) and `Install-PSFzf` installs
  the PSFzf module from PSGallery (NOT in `$Catalog` — it is a module, not a
  binary; bootstraps the NuGet provider first so `Install-Module` never blocks on
  a prompt in CI). The profile wires Ctrl+R (fuzzy history, intentionally
  overriding PSReadLine's reverse-search for POSIX parity with zsh), Ctrl+T
  (file) and Alt+C (cd) ONLY when both the PSFzf module and the `fzf` binary are
  present. The zsh side already used `fzf`; this brings Windows to parity.
- **`ls`/`Get-ChildItem` directories are gold via `$PSStyle.FileInfo.Directory`.**
  The default directory color (bright blue) is unreadable on Rose Pine dark.
  Guarded by `if ($PSStyle)` (absent on Windows PowerShell 5.1 and pwsh < 7.2);
  uses `$PSStyle.Foreground.FromRgb(0xf6c177)` so the source carries no raw ANSI
  escape byte (keeps the `.ps1` pure-ASCII invariant).

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
  as root / via sudo / via plain PAM, whichever is available;
- on Linux local accounts, a successful chsh also installs the same
  interactive-bash guard used for domain accounts, so new terminals and new tmux
  sessions land in zsh immediately even when the already-running graphical
  session still has stale `$SHELL=/bin/bash`. The chsh remains the source of
  truth for future logins.

It lives in `install-deps.sh`, NOT the chezmoi config layer, and
NOT `tmux.conf` or Ghostty config (a terminal-specific command would paper over
one launcher while bare TTYs, SSH sessions, and tmux stayed coupled to whatever
the account or parent shell says).

**Domain / non-local accounts (AD/LDAP/SSSD):** these resolve through NSS but
are NOT in local `/etc/passwd`, so `chsh` fails (`user '<name>' does not exist in
/etc/passwd`). `set_default_shell_zsh` detects this on Linux via
`is_local_account` (an `awk` exact-match on `/etc/passwd`) and routes to
`adopt_zsh_domain` instead of `adopt_zsh_chsh`. The fallback (`ensure_bash_execs_zsh`)
appends an idempotent, **interactive-only** (`[[ $- == *i* ]]`, so scp/rsync and
scripts stay bash) marked block to `~/.bashrc` that `export SHELL`s zsh and
`exec zsh`, and makes `~/.bash_profile` source `~/.bashrc` so login shells (tmux,
ssh) hit it too. The marker is now generic (`interactive bash fallback`), but
the helper also recognizes the legacy `domain login; chsh unavailable` marker so
old domain installs do not get a duplicate block on re-run. macOS is excluded
from the bashrc safety net (its accounts live in dscl, not passwd files, and
Terminal/iTerm use the passwd shell normally).

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
- **Arrow keys are left at their Neovim defaults** (they move the cursor). An
  earlier config mapped them to `<Nop>` to enforce hjkl-only navigation; that was
  removed at the owner's request, so do NOT re-add the arrow `<Nop>` block in
  `vim-options.lua`.
- **`vim.opt.clipboard = "unnamedplus"`** even on macOS — works fine via
  pbcopy/pbpaste. The single-register `unnamed` value would lock WSL/Linux
  out. The delayed missing-provider warning in `vim-options.lua` is routed
  through `_warn_if_missing_clipboard_provider` so `clipboard_spec.lua` can test
  both the warning branch and the `vim.g.clipboard` escape hatch.
- **`shells/zshrc` has shellcheck disable directives** at the top — zsh
  has glob qualifiers (`(#qN.mh+24)`) that shellcheck (a bash linter)
  cannot parse. The directives suppress the noise; the file is otherwise
  shellcheck-clean.
- **`shells/zshrc` probes installed locales before exporting one.** Prefer
  `en_US.UTF-8` when `locale -a` reports it, fall back to `C.UTF-8`, and leave
  the caller's locale untouched when neither exists.
- **`nvim/lazy-lock.json` is tracked** (NOT in `.gitignore`). This is how
  every machine ends up on the same plugin commits.
- **VS Build Tools for nvim-treesitter main is intentional.** It is large, but
  it is the compiler path the Rust `cc` crate expects on Windows. The existing
  zig install remains for LuaSnip `jsregexp`; it is not a substitute for
  nvim-treesitter main parser builds.
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
- **Language/branch module symbols are starship DEFAULTS, glyph-verified against
  Hack Nerd Font.** The same bare-space stripping bug had also emptied the
  `git_branch` / `c` / `golang` / `rust` / `python` symbols (and left `nodejs` on
  an MDI glyph). They are restored to starship's default dev-icons (U+E0A0 branch,
  U+E61E c, U+E627 go, U+E718 node, U+E7A8 rust, U+E606 python) — every codepoint
  CONFIRMED present in `HackNerdFont-Regular.ttf` via `fontTools` (load the cmap,
  test membership; that is the authoritative no-tofu check, NOT eyeballing). The
  ONE starship default NOT in Hack Nerd Font is `conda` `🅒` (U+1F152, an emoji),
  kept as the documented default (renders via the terminal emoji fallback). The
  `git_status` `✓` (U+2713) / `✘` (U+2718) are also outside the font but are
  common BMP symbols every terminal renders via fallback (unlike PUA nerd
  glyphs), so they stay. Re-audit after any symbol change by loading the font
  cmap with fontTools and asserting each non-ASCII codepoint is a member.
- **tmux colors track the canonical rose-pine/tmux theme.** Source:
  <https://github.com/rose-pine/tmux>, main variant. Role-based styles
  carry the colors. Map: status-style `fg=pine,bg=base`; window-status
  `fg=iris,bg=base` (inactive windows; this is the "cool" default the owner chose
  after auditioning `tmux/themes/`); window-status-current `fg=gold,bold`;
  window-status-activity `fg=base,bg=rose`; pane-border `fg=hl_high #524f67`
  / pane-active-border `fg=gold`; message `fg=muted,bg=base`;
  message-command `fg=base,bg=gold`. (Status-area bgs are `base` -- the dark
  terminal color; bar opacity comes from WT `opacity: 100`, NOT the bg color, see
  the opaque-bar note below.) **DO** set explicit
  `window-status-format "#I:#W#F"` and `window-status-current-format
  "#I:#W#F"` -- tmux's DEFAULT format contains a `#{?window_flags,...}`
  conditional that psmux v3.3.4 renders as a literal string in each cell.
  An earlier ship that used `setw -gu window-status-format` (unset, fall
  back to tmux default) lit that bug -- explicit `#I:#W#F` parses cleanly
  in both real tmux and psmux. Active windows use gold with bold weight because
  the foreground-only canonical theme was too subtle in dark terminals; bold is
  the smallest divergence that fixes legibility without breaking the palette.
  Inactive cells use iris (`setw -g window-status-style "fg=#c4a7e7,bg=#191724"`,
  8.4:1) -- the "cool" theme, which the owner chose as the default after
  auditioning the alternatives in `tmux/themes/` (warm/minimal/teal). The teal
  variant (inactive inherits pine via `setw -gu`) lives in `tmux/themes/teal.conf`
  if you want it back. Earlier churn (transparency was misdiagnosed as a color
  problem) is settled; the legibility knob was always WT opacity, not the color.
  **OPAQUE STATUS BAR -- the WT reality (owner-confirmed on a real machine).**
  Windows Terminal applies its `opacity` WINDOW-WIDE to every cell, regardless of
  the cell's background color. So a transparent WT (`opacity < 100`) has a
  transparent status bar no matter what bg the bar uses -- giving the bar an
  explicit, different-from-default color does NOT make it opaque in WT (this was
  tried with `surface #1f1d2e` at `opacity: 95` and the bar stayed see-through).
  That "explicit bg renders opaque" behavior exists in some terminals (alacritty)
  but NOT in Windows Terminal. The only way to a fully solid bar in WT is a fully
  opaque window (`opacity: 100`). **The owner chose transparency over a solid
  bar:** the fragment ships **`opacity: 95`** (see-through terminal), so the
  status bar is transparent too -- an accepted trade for the see-through look,
  themed via colors rather than opacity. Status-area bgs stay `base #191724` (the
  dark terminal color). macOS/Linux Ghostty use `background-opacity 0.95` +
  `background-blur-radius 15`, whose blur renders an opaque-LOOKING dark bar even
  while transparent (WT acrylic is the analog but is unreliable in a VM, so WT
  gets no blur). Do NOT re-add an overlay `status-style` hack (a prior wrong
  attempt assuming psmux ignores `window-status-style`, since removed) and do NOT
  switch the bar bg to surface to "fix" opacity (it does not, in WT). To flip to
  a fully-solid bar, set WT `opacity: 100` (whole window). Guarded by
  `tests/tmux/option_test.sh`.
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
  (5) inactive color (iris vs inherited-pine "teal") flip-flopped while
  transparency was misdiagnosed as a color problem; settled by the owner
  auditioning `tmux/themes/` and picking iris ("cool") as the default, with teal
  kept as `tmux/themes/teal.conf`. If inactive reads poorly it is WT transparency
  (`opacity`), NOT the color -- do not chase it by re-coloring;
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
  2. **Binding:** `lazygit/config.yml` and `lazygit/config.windows.yml` bind
     `keybinding.commits.moveDownCommit` / `moveUpCommit` to uppercase
     `J` / `K`. The Windows variant also binds `keybinding.universal.return`
     to `<c-g>` because psmux does not reliably deliver bare Esc to lazygit, and
     lazygit's `?` keybindings view should advertise the working cancel key. We
     intentionally do NOT use Ctrl+J / Ctrl+K for commit movement: Ctrl+J is
     ASCII LF (0x0A), the same byte as Enter. Disambiguating requires
     Win32-input-mode (ConPTY DECSET 9001), modifyOtherKeys, or kitty keyboard
     protocol metadata. Windows Terminal sends it and lazygit's tcell/v3 can
     decode it, but psmux v3.3.4 does NOT relay the metadata to panes, so
     default Ctrl+J degrades to Enter inside psmux. Uppercase J / K are normal
     printable bytes and skip that entire transport problem.
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

1. Run the local gate (`make ci` on macOS/Linux/WSL, `.\test.ps1` on Windows).
   Get baseline.
2. Make the change.
3. Update tests in the same diff (add a regression test for the new
   behavior; update an invariant assertion if you changed something this
   doc says is invariant).
4. Update this CLAUDE.md if you've changed any of the invariants or added a
   new common workflow.
5. Update `README.md` if you've changed the install path / install command.
6. Run the local gate again. Green.
7. Stage everything, commit.

If a test breaks: fix the cause, not the test. The test names are
deliberately worded as failure modes ("regression guard for …") — read them
carefully before "fixing" the test.

## Plan / history

The durable rationale belongs in this file, `README.md`, or the tests that
guard an invariant. Do not rely on private local plan files for public repo
maintenance.
