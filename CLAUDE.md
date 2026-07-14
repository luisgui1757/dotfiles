# Repo guide for coding agents (and humans coming back to this cold)

This file is the on-ramp. If you're a future coding-agent session, read this
**before** touching anything. If you're me six months from now and forgot how
the install script works, read this too.

> **Single source of truth.** This file is canonical. `AGENTS.md` at the repo
> root is a thin pointer here so non-Claude agents discover it; do not copy this
> content there because two real guide files would drift. Claude Code auto-loads
> this file, and other agents reach it through `AGENTS.md`.

`README.md` is the operator guide. Keep its opening sections simple and
task-oriented: supported platforms/tools, install/update/remove commands, then
daily cheat sheets. When a user-visible key, command, config path, or platform
boundary changes, update the matching cheat sheet in the same change. Detailed
implementation rationale belongs later in the README or in this file; do not
make users read Nix, migration, or CI internals before they can operate a tool.

## What this repo is

Cross-platform dotfiles for Neovim, VS Code, Herdr, tmux/psmux, Starship,
Ghostty, WezTerm, Windows Terminal, AeroSpace, zsh, PowerShell 7, lazygit,
`lsd`, `zoxide`, fzf, GitHub CLI/gh-dash, Pi CLI, language tooling, and global
Sentinel agent-policy bootstrap. Public installs go through `setup.sh` (macOS /
Linux / WSL) or `setup.ps1` (Windows), which install dependencies, apply the
chezmoi config layer, restore locked Neovim plugins, sync Tree-sitter/Mason
tools, and apply global agent policy. The repo can live anywhere — `~/dotfiles/`,
`~/Documents/dotfiles/`, etc. The remote-clone default in `setup.{sh,ps1}` is
`~/dotfiles/`, but an
in-place clone elsewhere works too. Do NOT put the repo at `~/.config/nvim/` —
the installer creates that path as a symlink **pointing into** the repo, so a
repo there would self-overlap (the self-link guard refuses this).

The `home/` tree is the chezmoi source tree for the full config layer. `setup.*`
uses it in Phase 2. Top-level config files and their `home/` copies/templates
must stay byte-identical where the parity manifest says so; update both in the
same change.

Agent runtime state and preferences are intentionally **NOT** synced through
this repo. The supported agent surface is setup's global Sentinel policy phase;
local agent preference folders such as `.claude/`, `.codex/`, `.pi/`, sessions,
auth files, and package caches stay per machine.

## Layout at a glance

```
~/dotfiles/
├── nvim/                  Neovim — init.lua, lua/{vim-options,util,plugins}
├── starship/              starship.toml (Rose Pine palette)
├── lsd/                   config.yaml + colors.yaml (Rose Pine)
├── shells/                zshenv + zshrc + powershell_profile.ps1
├── tmux/                  tmux.conf (Rose Pine, vi-mode, OSC52 clipboard)
├── ghostty/               config (Rose Pine, Hack Nerd, tuned for tmux)
├── wezterm/               wezterm.lua (shared terminal config on every OS)
├── aerospace/             macOS tiling-window-manager config
├── herdr/                 config.toml (forced built-in Rose Pine theme)
├── windows-terminal/      settings.fragment.jsonc + merge README
├── lazygit/               config.yml + config.windows.yml (J/K + Windows Ctrl-G)
├── gh-dash/               pull-request/issue dashboard config
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
├── README.md              operator guide, cheat sheets, and detailed reference
├── AGENTS.md              standard agent entry point, points here
└── CLAUDE.md              canonical tracked coding-agent guide
```

Local agent state directories such as `.claude/`, `.codex/`, and `.pi/` are
**not** part of the synced configuration. Leave them untracked; preferences,
sessions, auth files, and caches live per machine.

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
   Normal save formatting stays synchronous and uses the same bounded 10-second
   timeout as the strict formatter/LSP smoke. The former 3-second ceiling
   repeatedly killed cold Windows Node/Prettier processes. Do not replace this
   with asynchronous after-save formatting: `:w` must publish the formatted
   bytes, while `:WNF` remains the explicit one-save escape hatch.
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
    Equation rendering uses render-markdown's LaTeX path with the `latex`
    Tree-sitter parser plus the `latex2text` converter. Setup installs
    `latex2text` through a pinned, SHA-256-checked `pylatexenc` venv; its
    `setuptools` build backend is pinned too. Do not replace this with an
    unpinned host pip install or a second Markdown renderer.
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
    search. Plugin publication is owned by
    `scripts/ensure-pinned-zsh-plugin.sh`, called by install-deps and the
    pin/helper-sensitive chezmoi `run_onchange` script. It neutralizes any unproved sourceable target,
    stages the exact commit in the same parent, proves expected origin, HEAD,
    clean/usable worktree, and tracked regular entry file, then atomically
    publishes under a serialized lock. Never restore generic chezmoi
    `git-repo` externals for executable zsh payloads.
    PowerShell uses `HistorySearch` parity. Do NOT swap in an always-on
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
    paths (`install-deps.ps1` vs setup's transaction), and setup runs the
    merge by default. Opt out with `-SkipWindowsTerminalMerge`;
    `-MergeWindowsTerminal` is a retained no-op alias. Chezmoi exposes no WT
    target: it cannot provide the required backup/concurrency/atomicity contract.
    One shared validated enumerator defines stable packaged, Preview, Canary,
    and portable settings identities for setup, release migration, recovery,
    and uninstall. Setup treats all selected files independently, never mirrors
    one over another, and seeds the portable path only when portable WT is
    detected. The merge adds a fixed PowerShell 7
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
    already present and any managed `queries/<bundled>/` install-output
    directory already present (leftovers from an older config, or restored from a
    CI cache, still override the matched built-in parser/query pair). Parser-file
    deletes are **scoped to `stdpath('data')`** (nvim-treesitter installs under
    `…/site`; the install prefix does not), and query-directory deletes are
    **scoped to nvim-treesitter's `get_install_dir("queries")` output**, which
    must also live under `stdpath('data')`. An unscoped delete would wipe
    Neovim's OWN built-in runtime. Every managed removal goes through
    `nvim/lua/util/checked_delete.lua`, checks `vim.fn.delete()`'s result, and
    verifies absence; synchronous setup fails if a parser, parser-info, or query
    path remains. `c` and `vim` are bundled but not
    auto-started, so the config starts them via
    `nvim_bundled_started_here = { "c", "vim" }`. See "Add a treesitter parser".
    Guarded by `treesitter_spec.lua`, `language_smoke_spec.lua`, and the Tier-2
    `lsp_smoke.lua` runtime preflight, which synchronously bootstraps parsers and
    opens every language-matrix fixture under the production config so missing
    parser builds or parser/query runtime failures cannot hide behind non-LSP
    rows. The preflight also rejects unexpected nvim-treesitter-managed
    install-output parser `.so` files under `stdpath('data')/site/parser`,
    rejects managed bundled query install-output directories, requires parser and
    query install output for the non-bundled upstream dependency set, requires
    managed `highlights.scm` for explicit parser rows (for example PHP's paired
    `php_only` parser can remain query-only), and purges bundled parser/query overrides
    after setup installs complete.
20. **Repo text stays LF-only.** `.gitattributes` force-normalizes text files to
    LF so Windows checkouts do not CRLF-corrupt shell/WSL entry points, and
    `.editorconfig` must not reintroduce `end_of_line = crlf` exceptions (even
    for `*.bat` / `*.cmd`). Batch and cmd files have explicit
    `text eol=lf` attributes, and fixtures/scripts are validated as LF text like
    the rest of the repo. Guarded by `editorconfig_check.sh` and
    `invariants_test.sh`, including `git check-attr` probes for `.bat` and
    `.cmd`.
21. **Command-line vi mode is ENABLE-BEFORE-BIND on both shells.** Both shells
    ship vi keybindings, and in both the mode switch RESETS the active bindings,
    so it must run *before* every keymap/key-handler line — never after.
    - **zsh (`shells/zshrc`):** `bindkey -v` runs in its own section *before* the
      completion/keybinding region (before `compinit`, the fzf-tab Tab reclaim,
      and the Up/Down bindings). It swaps the main keymap from emacs to `viins`,
      so later unqualified `bindkey ...` calls land on the active vi insert
      keymap; bindings that must also work in normal mode are added to `vicmd`
      explicitly (Up/Down history search, the Tab reclaim, the Ctrl-R fallback).
      `KEYTIMEOUT` is set explicitly (`${DOTFILES_KEYTIMEOUT:-25}`) — NOT left at
      the default and NOT set to 1: a too-low value splits ESC-prefixed sequences
      (the Meta/Alt prefix such as fzf's Alt-C, and the arrow-key `ESC [ A`
      sequences) into "ESC then a normal-mode command"; override via
      `DOTFILES_KEYTIMEOUT` or `~/.zshrc.local`. Cursor shape is registered
      through `add-zle-hook-widget keymap-select`/`line-init` (NOT a raw
      `zle -N zle-keymap-select`) so it COMPOSES with zsh-autosuggestions and
      starship; registering it before starship init is load-bearing because
      starship only *preserves* an already-registered `zle-keymap-select`. fzf's
      own `--zsh` block already binds Ctrl-R/Ctrl-T/Alt-C in viins+vicmd, so those
      chords survive vi mode without extra work. Guarded by
      `tests/shell/zsh_vi_mode_test.sh` (static ordering + live-keymap probe).
    - **PowerShell (`shells/powershell_profile.ps1`):** `Set-PSReadLineOption
      -EditMode Vi` runs *before* any `Set-PSReadLineKeyHandler` (changing
      EditMode wipes handlers). Tab=MenuComplete and Up/Down history search are
      re-applied after, with `-ViMode Insert` (Up/Down also `-ViMode Command`);
      `-ViMode` and `-ViModeIndicator` are gated on the parameter existing so
      PS 5.1 / old PSReadLine falls back to unscoped bindings. The psmux
      `PowerShell.OnIdle` re-apply also re-asserts these handlers but does NOT
      re-set EditMode (that would wipe PSFzf's Ctrl+R/T + Alt+C chords). Guarded
      by the vi-mode `It` blocks in `tests/powershell/Profile.Tests.ps1`.
22. **Nix owns POSIX packages only; chezmoi owns every dotfile target; native
    Windows stays non-Nix.** The Nix layer (nix-darwin + declarative Homebrew on
    macOS, Home Manager standalone on Linux/WSL userland) is a *package* provider.
    It never owns a config file. These five sub-rules are the load-bearing
    boundaries, guarded statically by `tests/static/nix_architecture_test.sh` (and,
    for the update-mode rule, `tests/shell/install_deps_update_test.sh`):
    - **(a) chezmoi-only dotfile ownership.** Exactly one owner per path. Every
      dotfile target on every OS is owned by chezmoi (the `home/` source tree).
      No Nix file may write a dotfile that chezmoi manages — not zsh, nvim, tmux,
      starship, wezterm, aerospace, lazygit, lsd, gh-dash, ghostty, powershell, or
      any other config surface. If Nix and chezmoi ever co-own a path, that is the
      bug.
    - **(b) Home Manager / nix-darwin is packages-only.** No `home.file`, no
      `xdg.configFile`, no `xdg.dataFile`, no `home.activation` at all, and no
      `programs.*` module except `programs.home-manager` (the standalone HM CLI).
      Home Manager
      declares `home.packages` (plus the minimal `home.username` /
      `home.homeDirectory` / `home.stateVersion`) and nothing that renders a
      shell/editor/terminal config file. GUI / TCC-sensitive apps (WezTerm,
      AeroSpace, Herdr) come from vendor channels (Homebrew casks / pinned
      artifacts, including the Windows Herdr preview `.exe`), never nixpkgs.
      Node 24 is in this package-only set because the pinned Pi CLI npm package
      needs a modern Node runtime; the Pi package itself remains npm-pinned until
      nixpkgs catches up.
    - **(c) native Windows is non-Nix.** Windows-host files are `setup.ps1` +
      native package managers (Scoop/winget/choco) + chezmoi. Nix has no supported
      native-Windows story; it applies to WSL2 *userland* only and must never
      touch Windows-host paths under `/mnt/c`. No `.nix` file references a Windows
      path (`/mnt/c`, `C:\`, `%USERPROFILE%`, `AppData`), and `setup.ps1` /
      `install-deps.ps1` never invoke `nix`, `darwin-rebuild`, or `home-manager`.
    - **(d) no remote-eval installer.** The repo never adds a `curl | sh`,
      `irm | iex`, `Invoke-Expression`, or `nix-installer`/`install.determinate.systems`
      pipe-to-shell path for Nix (or anything else). Public setup invokes only
      `scripts/install-nix-prerequisite.sh`. Before v0.2.0 publication it
      accepts only a clean exact current official branch head; one isolated
      remote-ref snapshot binds that prerelease decision and the absent release
      tag. Once the unique annotated tag appears, the branch path closes and
      only the matching local tag object, peeled commit, and HEAD are accepted.
      The helper then downloads the pinned upstream Nix archive, verifies its
      platform SHA-256 and archive paths, and executes only those verified local
      bytes with the upstream `--yes` non-interactive flag and the selected
      daemon mode. Its reviewed extra config enables `nix-command flakes` in
      daemon installs; single-user Linux merges those additive features into
      the user's Nix config, and a retry self-heals the same disabled-feature
      state after an otherwise-complete install. Before invoking that helper,
      greenfield Linux/WSL setup must reuse `install-deps.sh`'s source-only
      `require_downloader` path to install `curl` plus CA certificates through
      the detected package manager; this bootstrap precedes Nix because the full
      native dependency phase intentionally follows Nix activation. Guarded
      behaviorally by `tests/shell/setup_nix_downloader_test.sh`,
      `tests/shell/nix_prerequisite_identity_test.sh` and statically by
      `tests/static/supply_chain_remote_execution_test.sh`.
    - **(e) Nix owner reporting in update mode.** When `install-deps.sh --update`
      resolves a tool whose command source (or its real path) lives under a Nix
      profile/store path (`/nix/store`, `*/.nix-profile/*`,
      `*/.local/state/nix/profile/*`, `/run/current-system/sw`,
      `/etc/profiles/per-user`, `/nix/var/nix/profiles/*`), `nix_owns_tool_source`
      reports it truthfully as `skipped … owner=nix reason=managed by the Nix
      layer …` — reusing the documented `skipped` vocabulary (like the pacman
      "explicit system upgrade" case), not a new status word. It fires only when
      PATH actually resolves the tool from Nix, so every other tool keeps its
      existing per-manager ownership. Update mode must NOT run a blanket
      `nix profile upgrade`, `nix-env -u`, or `nix flake update`, and must NOT
      silently rewrite `flake.lock`; the pinned lock is bumped only by an
      explicit, reviewed PR (Renovate `nix` manager), and the Nix layer is
      refreshed by the enforced POSIX `setup.sh` switch (or the compatibility
      `--nix-darwin` / `--home-manager` aliases).
      Guarded by the `nix-owned tool reports owner=nix` case in
      `tests/shell/install_deps_update_test.sh` (which also proves update mode
      never shells out to `nix` for an owned tool) and the "no blanket nix
      upgrade / silent flake.lock rewrite" check in `nix_architecture_test.sh`.
23. **Executable plugin caches are proved before runtime use.** Production
    `lazy.nvim` and the Plenary test bootstrap both use
    `nvim/lua/util/pinned_git_checkout.lua`. A full 40-hex lock entry is a hard
    precondition before network access. An existing cache is accepted only
    after proving a usable Git worktree, the expected origin, exact locked HEAD,
    a clean tracked/untracked state, and the required Lua entrypoint. Repairs
    fetch the locked commit into a same-parent staging checkout under an atomic
    lock, verify it, preserve the previous checkout until publication succeeds,
    and clean lock/staging state on failure. Runtimepath mutation and
    `require("lazy")` occur only after that proof. Guarded behaviorally by
    `tests/nvim/spec/pinned_git_checkout_spec.lua`; do not replace this with
    grep-only evidence or a mutable `git clone` directly into the live cache.
    For lazy.nvim, validate the locked branch name and prove `origin/HEAD` plus
    that remote branch at the same locked commit while keeping HEAD detached.
    Lazy needs that metadata to serialize its lock; the branch tip never becomes
    executable authority.
24. **POSIX setup has one validated target identity and supported architecture.** Public
    `setup.sh` runs as the target non-root account. At the boundary it reads the
    real account record (`dscl` on macOS, `getent`/`/etc/passwd` on Linux),
    requires canonicalized `HOME` to identify that same existing directory, and
    exports `DOTFILES_TARGET_USER` + `DOTFILES_TARGET_HOME`. Nix, Home Manager,
    chezmoi, Sentinel, and native setup consume those values; never fabricate a
    home from a username or fall back to root. Darwin setup accepts only Apple
    Silicon, selects `dotfiles-aarch64`, and rejects every other architecture
    before Nix/Homebrew activation. The compatibility `dotfiles` alias is also
    Apple Silicon; no other Darwin system/configuration is exported.
    Homebrew is mixed ownership: nix-darwin applies the declared subset with
    `cleanup = "none"`, while `mutableTaps = true` leaves every tap clone owned
    by target-user Homebrew. nix-homebrew pins Homebrew itself but copies no tap
    contents during root activation. setup never moves or replaces the whole
    `Library/Taps` directory; it has one scoped migration for the three exact
    root-owned, non-Git snapshots produced by the retired configuration and
    never selects unrelated user taps. Transaction, failure, and recovery
    snapshots must be siblings of `Library/Taps`, never descendants: Homebrew
    enumerates descendant directories as taps. Setup relocates the exact
    descendant recovery names produced by the short-lived broken migration
    before activation, without selecting unrelated names. First
    nix-darwin bootstrap also preflights and preserves existing `/etc/bashrc`
    and `/etc/zshrc` at their documented `.before-nix-darwin` paths; collisions
    fail before either move, and activation failure/interruption quarantines
    generated replacements before restoring both originals. Repeated setup from
    a pre-activation shell resolves the installed `/run/current-system` rebuild
    command outside stale `PATH`; exact `/etc/static/{bashrc,zshrc}` links are
    already-managed state, so retained backups are not collisions. If
    Homebrew's `path_helper` removes Nix after the upstream profile's sourced
    guard was set, setup directly re-adopts the canonical daemon/user profile
    binary instead of trying to reinstall Nix. Guarded by
    `setup_target_identity_test.sh`,
    `setup_nix_darwin_test.sh`, `darwin_config_test.sh`, and
    `darwin_platform_contract_test.sh`.
25. **Windows setup uses application-consumed known folders, not fabricated
    children of UserProfile.** Resolve UserProfile, LocalApplicationData,
    ApplicationData, Documents, and runtime `$PROFILE` independently. The main
    chezmoi source is UserProfile-only; `windows/chezmoi-localappdata`,
    `windows/chezmoi-appdata`, and `windows/chezmoi-documents` are explicit
    destination overlays with separate persistent state. Post-apply checks
    cover nvim, lazygit, Herdr, ConsoleHost, VS Code, and ISE. Path proof
    resolves both directory symlinks and Windows
    junctions before comparing ownership. Recognized conventional legacy
    targets migrate only after successful publication; divergent legacy user
    data is preserved. The main source exposes no Windows Terminal target, so
    setup applies it without absolute target selectors and then runs the
    dedicated WT transaction. Native stderr is retained on apply failure.
    Uninstall enumerates the same four source states.
    Guarded by Setup/Uninstall Pester and the Windows apply/round-trip migration
    suites.
26. **PowerShell profiles run only for an actual interactive invocation.** The
    guard executes before cache path construction and rejects
    `-NonInteractive`, batch `-Command`/`-File`/encoded/stdin modes without
    `-NoExit`, redirected stdin/stdout/stderr, CI, and unsupported hosts.
    `[Environment]::UserInteractive` and host name are context, not a sufficient
    invocation predicate. Normal ConsoleHost, VS Code, and ISE stay supported.
27. **Required check identities migrate without renaming deadlock.**
    `.github/check-identities.json` records the stable target and still-emitted
    legacy producers. The checked-in required-check sources require stable
    logical jobs that verify exact per-OS proof artifacts bound to the same run,
    PR source head, and actually executed SHA. On `pull_request`, the executed
    SHA is GitHub's synthetic merge commit; it must never be mislabeled as the
    source head. Workflows retain legacy producer names until live cutover.
    Never make a no-op check to manufacture green status, and never switch live
    contexts from the cutover PR. Follow `docs/security/branch-protection.md`:
    merge while live legacy checks still gate, pass cache-free plus all logical
    checks on the exact merged `main` SHA, then have the owner run
    `--preflight-only`, apply, and verify the checked-in safeguards. Before its
    first mutation the apply command twice verifies exact branch/repository/main
    identity, clean sources, unique and exact legacy live policy, GitHub Actions
    app/workflow/event/run provenance, and cache-free E2E evidence. It snapshots
    the three changed resources, rolls all three back on apply/readback failure,
    and retains an exact `--restore` path when recovery needs owner action.
28. **Handled native PowerShell status never escapes its adapter.** Setup and
    uninstall chezmoi helpers temporarily disable native error promotion,
    capture stdout/stderr and the exact exit code, restore the caller preference,
    and reset global `LASTEXITCODE` after producing the explicit result object.
    Expected verify drift is returned as false; actual invocation failures still
    throw. This prevents an otherwise-successful script or GitHub `pwsh` step
    from inheriting a handled exit 1. Guarded under both preference states by
    Setup/Uninstall Pester and the Windows round-trip entry point.
29. **Published release upgrades are exact-tag, side-by-side transactions.**
    v0.1.0 is already chezmoi-based; its POSIX targets are live checkout
    symlinks, so `git pull` or switching that checkout before setup crosses the
    backup boundary. Never publish an in-place v0.1.0 migration. The v0.2.0
    tools require separate clean official annotated-tag checkouts, exact
    historical config, authoritative target identity, and private recovery.
    POSIX recovery archives both exact commits, validates digest-bound read-only
    trees, and uses only those frozen sources for Nix/config publication and
    rollback; post-validation checkout changes cannot affect a write.
    Windows recovery likewise archives both exact commits beneath its protected
    ACL, records all four canonical Terminal identities and their expected
    presence/hash state, and binds setup, acceptance, uninstall, and rollback to
    those validated trees rather than either retained checkout.
    The reversible core runs only Nix activation plus config files/links on POSIX
    (`--skip-native-deps --skip-config-scripts --skip-nvim --skip-agents`) and config/known-folder/
    Terminal publication on Windows
    (`-SkipDeps -SkipNvim -SkipAgents -SkipConfigScripts`). Both migrators mark
    their frozen nested setup so it cannot recursively
    rediscover the still-live v0.1.0 installation. Conventional
    v0.1 known-folder targets stay in place until acceptance. Chezmoi run scripts and
    additive native provisioning happen only after acceptance. Failure or interruption removes the first
    nix-darwin/Home Manager activation and restores v0.1.0, or restores exact
    Windows Terminal bytes and old Windows config. Keep both checkouts through
    verified acceptance. Public setup is the normal orchestrator: `--all` /
    `-All` detects exact live v0.1.0, invokes and validates this transaction,
    resumes an already-applied recovery, accepts the verified core under the
    explicit non-interactive contract, retains its recovery evidence, and
    completes ordinary setup. `--update` / `-Update` performs that same
    reconciliation before its scoped refresh; upgrade is an alias. The
    standalone migrators remain operator recovery interfaces. The source of
    truth is `docs/UPGRADING.md`.
30. **Sentinel is the sole agent-policy product name in the repository.** The
    pre-rename name must never return in a tracked repository path or file,
    including tests, docs, cache constants, and historical prose.
    `tests/static/sentinel_naming_test.sh` reconstructs the retired token at
    runtime so the guard itself does not violate the zero-residue contract.
31. **GitHub-native security is checked in and applied in migration order.**
    `scripts/apply-github-security.sh` owns private vulnerability reporting,
    immutable releases, CodeQL default setup for exactly Actions + Python with
    default queries, and the non-bypassable CodeQL rule in
    `Protect main: integrity`. The rule blocks ordinary errors and
    high-or-higher security alerts, and publication requires successful Actions
    and Python analyses on exact live `main`. Apply the stable required-context
    cutover first; only then merge/apply the security policy. The scripts fail
    closed across a mixed stage. Shell, PowerShell, Lua, and Nix remain outside
    CodeQL coverage and keep their focused CI authority. Guarded by
    `tests/static/github_security_policy_test.sh` and `repo_policy_test.sh`.

## Common workflows

### Add a new plugin

Drop a single file under `nvim/lua/plugins/<name>.lua` returning the lazy
spec. Lazy auto-discovers `{ import = "plugins" }` from `init.lua`. Default
to lazy-loading (`event` / `cmd` / `keys` / `ft`). Only `rose-pine` may set
`lazy = false`.

### Add a new LSP server

1. Add the server name to `vim.lsp.config(...)` and the `vim.lsp.enable({...})`
   list in `nvim/lua/plugins/lsp-config.lua`.
2. Add the Mason package to `expected_tools()` in
   `nvim/lua/util/mason_tools.lua`; the plugin config and fail-closed headless
   postcondition share that one manifest.
3. Add the server name to `tests/nvim/spec/lsp_spec.lua`'s `required_servers`
   so the static-check catches a future accidental removal.

For clangd, do not calculate a process-wide `--compile-commands-dir` from
Neovim's startup cwd. Let each client discover `compile_commands.json` from its
actual file/project ancestors (including `build/`).
`clangd_projects_spec.lua` starts two real clients with different databases in
one Neovim process and requires isolated roots, flags, and diagnostics. The
generic Ubuntu test lane installs the distro `clangd` package explicitly; the
spec fails when the real binary is absent rather than substituting static proof.
Production Linux/WSL owns `clangd` through Home Manager's `clang-tools` package
on both supported architectures. Mason's registry currently has Linux x64 but
no Linux arm64 clangd artifact, so Linux is deliberately excluded from the
Mason clangd entry; macOS and Windows retain Mason ownership.

> **Headless-install gotcha:** `mason-tool-installer` is `event = "VeryLazy"`
> (interactive auto-install via `run_on_start`) **and** registers its commands
> under `cmd = { … }`. Those `cmd` triggers are load-bearing — the setup phase
> invokes `MasonToolsInstallSync` through `util.mason_tools.run_checked`, and
> `VeryLazy` never fires
> without a UI, so without the `cmd` trigger that command is `E492: Not an
> editor command`. Keep `MasonToolsInstallSync`/`MasonToolsUpdateSync` in the
> `cmd` list (guarded by `lsp_spec.lua`).

### Add a new formatter

1. Add to `formatters_by_ft` in `nvim/lua/plugins/conform.lua`.
2. Add the Mason package to `expected_tools()` in
   `nvim/lua/util/mason_tools.lua`.
3. If an LSP also attaches for that filetype, add or update a realistic fixture
   under `tests/nvim/fixtures/formatter_lsp/` and the Tier-2 formatter/LSP
   compatibility table in `tests/nvim/lsp_smoke.lua`. The invariant is:
   conform's selected external formatter(s) must produce no LSP warnings/errors.
4. No new autocmd — conform's `format_on_save` handles it. The buffer-local
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
   `nvim/lua/plugins/treesitter.lua`. The list intentionally covers a broad
   everyday language surface for highlighting (web formats, Go, SQL, .NET/Razor,
   PHP/Perl/Ruby/JVM languages, Zig, assembly, infra/data formats, Git/SSH/editor
   config, build-tool files, TeX/BibTeX/Typst/Mermaid, shaders, QML, Bicep, and
   common platform/scientific languages), but it is still explicit because
   parser installation is toolchain-backed and must stay reproducible.
2. If the parser name differs from Neovim's filetype, add an entry to
   `parser_filetype_aliases` so the `FileType` autocmd can call
   `vim.treesitter.start()` for real buffers.
3. Add it to `required` in `tests/nvim/spec/treesitter_spec.lua`, and add a
   fixture row in `tests/nvim/language_matrix.lua` so filetype detection,
   formatter mapping, and parser ownership are tested together.
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
stack), herdr/config.toml + herdr/config.windows.toml (built-in `rose-pine`,
forced with `auto_switch = false`; both map `prefix+w`/`prefix+g` to the full
navigator, `prefix+comma`/`prefix+$` to tab/workspace rename,
`prefix+up`/`prefix+down` to sequential workspace navigation, and
`prefix+shift+1..9` to indexed workspace selection; agent navigation uses
`prefix+shift+a`/`prefix+a` for previous/next and `prefix+ctrl+1..9` for indexed
focus; Windows alone selects `pwsh.exe`),
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
up on the same commits. Setup, e2e assertions, and greenfield validators must
use `Lazy! restore`, not `Lazy! sync`; `sync` is only for intentional dependency
maintenance that reviews and commits a lockfile diff. The startup-budget spec is
the exception: it preclones the locked plugin checkouts into its isolated cache
instead of invoking Lazy install/restore, because restore runs plugin build hooks
and the checked, waitable nvim-treesitter update can legitimately spend minutes
compiling parsers, which is outside a startup measurement.

A missing, empty, malformed, incomplete, or non-40-hex lock entry is fatal
before any plugin fetch or execution. If a cache is dirty, at the wrong commit,
from the wrong origin, not a Git repository, or missing its required entrypoint,
startup repairs it transactionally from the locked identity. Do not hand-edit a
live cache to recover; fix/restore `nvim/lazy-lock.json` and restart Neovim.

### Update Mason-installed tools across machines

```bash
nvim --headless "+lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
```

There's no machine-pinned lockfile for Mason itself — `mason-tool-installer`
ensures the named tools exist on each machine, and the repo wrapper verifies
every expected package before allowing a zero exit.

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
`timeout = 180000`. Keep that value explicit: `startup_spec.lua` preclones the
locked plugin checkouts under isolated XDG dirs, prewarms a real production init,
and Plenary's default 50s timeout can SIGTERM the child before the startup-budget
assertion reports the actual problem. The startup spec must not invoke Lazy's
install/restore path or leave nvim-treesitter parser outputs in its cache; those
are dependency-bootstrap costs, not warm startup costs. The startup budget itself
is strict, but the spec measures up to three warm starts and accepts the fastest
run; this filters scheduler/filesystem outliers while still failing a
consistently slow production init. `startup_spec.lua` must keep its
`[startup_spec]` stderr progress lines before plugin prewarm and each child init
so a parent timeout leaves the run root and last long operation in logs. Cached
plugin HEAD verification reads `.git/HEAD`/packed refs directly; do not replace
that warm-cache path with one `git rev-parse` subprocess per plugin.

Sub-targets **skip gracefully** when their tool isn't installed
(`yamllint`/`editorconfig-checker`/`hyperfine`/`ghostty`). The
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
`tests/static/toml_lint.sh` and `tests/starship/toml_lint.sh` use `taplo` when
it is healthy, but if local macOS `taplo` panics with the known
system-configuration null-object crash they fall back to Python `tomllib`;
ordinary `taplo` lint errors still fail.
`tests/static/supply_chain_remote_execution_test.sh` must stay pure Python for
the repository-wide scan; fast CI runs static tests before optional developer
tools like ripgrep are installed.
Repo-wide static scanners must explicitly prune generated/runtime trees such as
`.git`, `.claude`, `.codex`, `.pi`, `tests/.cache`, and archived docs before
walking files; Neovim tests clone real plugin repositories into `tests/.cache`.
`tests/tmux/load_test.sh` must keep its own `tmux -S` socket path so tmux socket
creation is hermetic and does not depend on host `/tmp/tmux-$UID` permissions.

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
PSScriptAnalyzer scans the meaningful `.ps1` surface and fails on errors, exact
group-count drift, or any change to the normalized
script/rule/message/extent fingerprint in `test.ps1`; yamllint plus the
Ruby/Psych semantic Settings policy are part of `make test-static`,
`scripts/validate-renovate.sh` fails when `npx` is missing
under `CI=true`, and Windows CI treats missing test dependencies as fatal.
PSGallery module installs in Windows CI use bounded retries for transient
gallery lookup failures; the final miss still fails the job.
Shell tests use explicit `if`/`then` control flow for negative assertions; do not
write `grep ... && fail || true` guards because Ubuntu ShellCheck flags `SC2015`
and the compact form has ambiguous failure semantics.
`tests/shell/lint.sh` runs ShellCheck per file. Production scripts stay strict;
source-only test fixtures get a reviewed false-positive exclude only for
dynamic `source` paths (`SC1091`), globals consumed by sourced installer/setup
functions (`SC2034`), and indirectly invoked command stubs / code paths
(`SC2317`, `SC2329`). Do not move those excludes to production scripts.
Fixtures that source the real shell configuration must also suppress host Home
Manager session state (`__HM_SESS_VARS_SOURCED=1`) when testing a synthetic
`PATH` and reset manager variables inside the interactive command; `/etc/zshrc`
runs before `zsh -i -c`, so a provisioned `/etc/profiles/per-user/<user>` or
Homebrew shellenv can silently reintroduce host tools and invalidate
absence-path assertions.
Installer fixtures that exercise a missing or direct-artifact path must likewise
establish their synthetic `PATH` before calling the install function; a tool
installed on the developer host must not turn a positive fixture into a skip.

`e2e-install.yml` is the required real-install gate. The jobs cover different
install paths, not symmetric container platforms:

The setup caches in this workflow include the pinned `actions/cache` major
version in their keys. When Renovate bumps `actions/cache` across a major,
update the cache-key major segment with it so the new action proves the install
path from a fresh archive instead of inheriting state produced by the previous
major; `tests/static/repo_policy_test.sh` enforces this.

- `e2e containers / ubuntu-24.04` runs an `ubuntu:24.04` container on an Ubuntu
  runner with `DOTFILES_SKIP_BREW_BOOTSTRAP=1`, creates a non-root user, runs
  real `install-deps.sh --all` (native `apt`, no Linuxbrew), then applies
  configs with chezmoi and asserts tool presence, including `zoxide`, `gh`,
  WezTerm, and Herdr, Neovim >= 0.12, lazygit, zsh
  plugin files under `~/.local/share/dotfiles/zsh-plugins`, config content
  matching the repo sources, and the Neovim
  directory resolving into repo `nvim/`.
  This is intentionally **not** a devcontainer. It stays because hosted Ubuntu
  has Linuxbrew available, so the container is the only automated proof of the
  clean-image native `apt` path: pinned Neovim tarball install, pinned lazygit
  release install, fixed-root zsh plugin install, `fd-find` -> `fd` shim, and apt
  fallbacks. It does not assert the Pi CLI because this path intentionally omits
  the Nix package layer that provides Node 24; Pi is asserted in the Nix-backed
  public setup jobs. Scope is intentionally **Ubuntu only** (the supported
  Linux/WSL2 proxy target).
  Re-adding another distro requires both a matrix entry in `e2e-install.yml` and
  a matching root-prep branch in `tests/ci/container-e2e.sh`.
- `setup.sh / ubuntu-24.04`, `setup.sh / macos-26`, and
  `setup.ps1 / windows-2025` run the real public setup entry points, apply
  configs through chezmoi in Phase 2, then rerun Lazy restore, Tree-sitter
  parser install, Mason headless sync, and the Sentinel Phase 6/6 agent-policy
  install. The required POSIX jobs begin with no `/nix`, check out the exact PR
  source head separately from GitHub's synthetic merge, and run the real
  prerequisite helper before setup. Before v0.2.0 publication that source must
  be a current official branch head; after publication the smoke switches to
  the fetched exact tag. Fork PRs cannot satisfy the official-head identity, so
  only they retain the pinned Determinate action as a pre-seeded test path.
  Both jobs then apply the enforced nix-darwin/Home Manager layer before
  native/deferred installs and assert the nix-owned CLI set resolves from a Nix
  profile/store path. The Linux job first
  proves a clean login/interactive zsh resolves Nix-owned `rg` through Home
  Manager session state with no CI PATH injection. That proof resolves the
  effective account's login shell from the account record and executes that
  exact zsh; never hardcode `/usr/bin/zsh`, because the supported fresh-Ubuntu
  path installs and selects Linuxbrew zsh. Login-shell stderr is retained on
  failure so a missing shell cannot masquerade as broken session state. They explicitly fail
  if setup skips Phase 3-5, omits Phase 6/6, emits a `FAIL:` marker, or Mason
  did not install expected tools. Windows e2e
  also asserts the new Windows tools that must leave PATH commands behind
  (`zoxide`, `gh`, `wezterm`, `herdr`, `pi`), so an installer that exits 0 but fails
  its command probe cannot fake-green. Windows also requires Hack Nerd Font
  files plus registry registration. The macOS jobs run Ghostty's real
  `+validate-config` and WezTerm's real config-loading `show-keys` path. They
  also invoke the installed AeroSpace app and CLI binaries and require their
  version/hash identities to agree. AeroSpace itself waits for a user-granted
  Accessibility permission before it parses user config or starts the CLI
  server, so hosted CI reports config-consumption proof unavailable and leaves
  that exact TCC-enabled desktop check in `tests/MANUAL.md`; it must not claim
  runtime config proof from a headless launch. Scheduled/manual
  clean-install runs skip broad install/plugin caches; PR runs retain
  architecture-keyed caches.
  must assert `%LOCALAPPDATA%\lazygit\config.yml`
  against `lazygit/config.windows.yml`, not the POSIX/default
  `lazygit/config.yml`. After the full restore/sync they also run the
  **Tier 2 language smoke** (`tests/nvim/lsp_smoke.lua`, gated on
  `DOTFILES_LSP_SMOKE=strict`): against the production init it asserts every
  `treesitter_parsers` entry is one nvim-treesitter `main` supports
  (`get_available()`/`get_available(4)` — the jsonc "unsupported language"
  catcher), synchronously bootstraps parser installs with the upstream waitable
  install task and requires it to return exactly `true`, requires every declared
  parser `.so` output and query install-output directories to be present, with
  managed `highlights.scm` for explicit parser rows,
  rejects unexpected
  install-output parser `.so` files under `stdpath('data')/site/parser`, asserts
  each fixture's LSP attaches, formats realistic LSP-backed samples copied into
  that same isolated project and client lifecycle under `tests/.cache` through
  conform.nvim's production route, requires the expected
  external formatter(s), fails on post-format LSP warnings/errors, then opens
  every language-matrix fixture, requires real Tree-sitter captures for
  parser-backed rows after explicitly starting and parsing the expected parser
  (`inspect_pos()` first, direct highlight-query capture iteration as the
  headless fallback), and proves syntax-only fallback rows have real Vim syntax
  groups. Keep the LSP
  combined attach/formatter gate before the broad fixture-open gate; opening
  every fixture under the production config can start LSPs as collateral. After
  the explicit LSP/formatter gate, the smoke disables the tested LSP configs
  before opening the broad parser/syntax matrix so later non-LSP gates do not
  leave unrelated language servers alive. Each LSP attach probe is
  copied into its own minimal project root under `tests/.cache`; never open the
  shared fixture directory as an LSP project. The shared directory contains
  more than one hundred unrelated language fixtures and made neocmakelsp's
  cold-start attach timing depend on repository-wide scanning. Never add a
  second formatter-only client lifecycle after an attachment proof: hosted
  macOS exposed timing-dependent neocmakelsp restart behavior even though the
  first isolated client attached. Format and diagnose the realistic sample on
  that already-attached isolated client instead.
  Non-gated servers are strict on every OS; `powershell_es` is
  enforced only on Windows (pwsh + the PSES bundle) and skips cleanly on Unix.
  The fast `make test-nvim` runs Tier 1 only
  (`tests/nvim/spec/language_smoke_spec.lua` + `tests/nvim/language_matrix.lua`):
  filetype + conform-formatter + parser-in-install-list per fixture.
- There is no macOS/Windows container analog to add for symmetry. Docker cannot
  model macOS, and Windows containers do not model the real desktop/user-profile
  install surface: Scoop/winget/choco, Developer Mode symlink behavior, font
  registration, and terminal profiles. Hosted macOS/Windows runners are the
  accepted representative fixtures for those OSes.
- There is no hosted WSL workflow. [GitHub documents nested virtualization on
  hosted runners as technically possible but not officially supported](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners). The
  retired optional WSL2 canary's only scheduled run and a manual rerun both
  reached WSL2 but hung before setup evidence because its action-created user
  had no noninteractive sudo path. Do not replace it with Linux plus fabricated
  WSL environment variables or make an unsupported nested-virtualization job a
  required context. Linux CI and WSL config/static tests are proxies, not WSL
  runtime proof. Full WSL host/guest validation is
  manual: run `./setup.sh --all` in WSL (which installs verified Nix when
  missing), then run `./tests/wsl/e2e.sh` after running `.\setup.ps1 -All` on
  Windows. Windows Terminal settings
  handling is default-on: packaged WT is merged when its settings file exists,
  and portable WT is seeded or merged at the unpackaged path when the packaged
  file is absent.

Local clean-machine harnesses live in `tests/greenfield/README.md`; keep them
manual VM/Sandbox tools and do not add them to the headless CI matrix.

Main-branch safeguards are canonical in `.github/rulesets/` and applied live by
`scripts/apply-repo-safeguards.sh`. `.github/settings.yml` deliberately contains
only repository-level settings: the Probot Settings app must not race the
transactional script by applying classic branch-protection changes when a
cutover commit reaches the default branch. The script owns both the integrity
ruleset and classic fallback required-check transition because Probot cannot
model the required split where owner bypass applies to review/update rules but
not CI. Enforce that absence by parsing YAML and rejecting the top-level
`branches` key in every semantic form; a line regex is not a policy boundary.

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
- GitHub security settings are applied separately by
  `scripts/apply-github-security.sh`: private vulnerability reporting,
  immutable releases, and CodeQL default setup for Actions + Python. After
  exact-main analyses pass, the integrity ruleset requires CodeQL errors and
  high-or-higher security alerts to be resolved before merge.

GitHub does not let pull request authors approve their own pull requests. Owner
authored PRs can use the owner review bypass, but they still cannot bypass the
integrity ruleset's required checks. Repository deletion is outside branch
protection for a personal repo; routine agents should use least-privilege
credentials, not owner-account, admin, or `delete_repo` capable tokens.

The current stable-context cutover is applied only through
`scripts/apply-repo-safeguards.sh` after its merged-main proof and no-write
preflight. The script refuses unexpected live drift rather than acting as an
unreviewed general reconciler, mutates only Actions SHA pinning plus integrity
and classic required checks, and keeps a verified recovery snapshot. Both
preflight captures require public repository visibility. Recovery must freeze
all consumed snapshot files before validation, bind their exact legacy/stable
contexts, app IDs, ruleset identity, bypass/branch policy, Actions pinning, and
complete classic state—including disabled review/restriction sections that
GitHub may omit or return as `null`—to the manifest, while rejecting any
non-null policy, then write and verify only those frozen bytes. Expected
recovery policy comes from
the manifest's exact captured Git commit, never the current worktree, and that
commit must still be live `main`. After the second live capture, apply
must also rebuild every desired write and its metadata from exact committed
objects in one private read-only transaction directory; no later write may read
the mutable checkout. Missing, altered, or cross-stage recovery material must
fail before mutation. Every temporary capture is caller-owned and cleaned on
all exits; a persistent recovery snapshot is pruned on pre-mutation failure and
retained once mutation begins or apply succeeds. Follow the commands and
`--restore` recovery path in
`docs/security/branch-protection.md`.
Do not recreate a hosted WSL2 canary or claim Linux proxy coverage as WSL
runtime proof.

The GitHub-security apply must follow, never precede, the stable-context
cutover. It accepts only exact clean `main`, verifies successful Actions and
Python CodeQL analyses for that SHA, snapshots the prior integrity ruleset, and
publishes only the checked-in CodeQL rule. If CodeQL is not yet configured, it
enables the exact default setup and stops before ruleset mutation so the initial
scan cannot be bypassed. GitHub Code Quality and advanced generic/validity
secret scanning are unavailable to this user-owned Pro repository and must not
be documented as active.

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
Renovate terms those are not datasource digests. Installer-script commit pins
for Homebrew, Scoop, and CI cargo-binstall are the reviewed digest/currentDigest
captures; their adjacent SHA-256 constants remain manual verification context.

Several pins are also **mirrored across files** (nvim version/SHA in
`install-deps.sh`, `test.yml`, and `tests/shell/install_nvim_linux{,_fail}_test.sh`;
zsh plugin tag/commit in `install-deps.sh`, `home/.chezmoiexternal.toml.tmpl`, and
the verify-pins run-script). A Renovate bump touches one surface and strands the
mirrors. `tests/static/pin_consistency_test.sh` is the canonical drift guard — it
fails CI when any mirror disagrees. When you bump a pin, update every mirror and
keep that test green.

Validate `renovate.json` locally with Renovate's own schema validator and local
extract dry run, not just `jq`: run `scripts/validate-renovate.sh`, `make validate-renovate`, or `make ci`
for the full pre-PR bundle. Do not hardcode transient Node/Renovate package
versions in docs or prompts; the script owns the pinned runtime/package pair and
keeps Renovate's engine requirements out of ad-hoc command examples. The
official extraction must equal `tests/static/renovate_expected_inventory.txt`;
regex matchability alone is not evidence that Renovate owns the dependency.
The beta Nix manager stays explicitly enabled, matrix runner labels use the
`github-runners` datasource, Scoop follows upstream `master`, and
`rebaseWhen = behind-base-branch` matches the strict-behind-main policy.
Capture the local extract's JSON stdout directly; `LOG_FILE` is optional logger
transport and did not materialize on a hosted Ubuntu run even though Renovate
exited 0. An empty successful extraction must fail before inventory comparison.

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
  Empty output is Homebrew's valid idempotent result when its bin/sbin already
  lead PATH, so setup accepts it only after the selected and active commands
  prove one canonical prefix and repository. Executable pathname equality is
  insufficient because nix-darwin's `/run/current-system/sw` wrapper correctly
  activates the matching architecture-native Homebrew entrypoint. Empty output
  cannot bootstrap a missing PATH.
  Failed evaluation or executable proof restores the prior PATH and Homebrew
  environment variables before printing recovery instructions. Because macOS
  has no alternate package manager, a required bootstrap/activation failure is
  summarized and aborts before any package install is attempted.
- `setup.ps1` runs a symlink-privilege pre-flight before chezmoi apply because
  the Windows Neovim target is still a directory symlink: dry-run warns and
  skips the probe; real runs print elevated/Developer Mode state plus the
  Developer Mode or elevated-config-step fix before attempting apply.
- Remote `setup.{sh,ps1}` has exactly one hard prerequisite: `git`, because the
  remote path must clone this repo before it can install everything else. The
  missing-git errors name the canonical first install command (`brew install
  git`, `apt install git`, or `winget install Git.Git`).
- The Windows installer does NOT symlink `settings.json` for Windows Terminal:
  WT rewrites that file on launch. `setup.ps1` Phase 2 excludes WT from chezmoi
  publication, then builds one independent plan for each existing stable
  packaged, Preview, and Canary target plus each existing/detected portable
  target. It stages in the destination directory,
  parses and byte-validates all results, makes a verified collision-safe backup
  for each divergent existing target, detects source changes both before and
  inside atomic `File.Replace`, and rolls already-published targets back as a
  group on failure. Missing Store settings stay absent; missing portable
  settings are seeded only when portable WT is detected. No target is ever a
  full-file mirror of the other. Backup/parse/stage/publication/unsafe-rollback
  failure is fatal with recovery paths. `-SkipWindowsTerminalMerge` is fully
  write-free. The managed WT profile is an explicit fixed-GUID `pwsh.exe` profile
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
  hardened `Add-ScoopBucketSafe` path, and the pinned psmux session plugins are
  vendored by `Install-PsmuxPlugins` into `~/.psmux/plugins/` (also provisioning,
  not chezmoi); chezmoi only owns the readable config files (`.tmux.conf`,
  `.tmux.windows.conf`, and the generated `.tmux.rose-pine.{main,moon,dawn}.conf`
  files, which are now deployed on BOTH POSIX and Windows).
  `home/.chezmoi.toml.tmpl` is the mode switch: POSIX uses `mode = "symlink"`
  for live-edit behavior, Windows uses `mode = "file"` for simple single-file
  configs, but Windows `nvim` remains a directory symlink and still needs
  Developer Mode or elevation. Same-path config files use managed source copies; path-divergent
  lazygit and Ghostty configs use `.chezmoitemplates/**` plus POSIX
  `symlink_*.tmpl` wrappers and Windows rendered `.tmpl` copies where
  applicable. Windows Terminal is a setup-owned transactional merge, not a
  chezmoi target, symlink, or fragment-only replacement. WT opens **maximized** (`launchMode`,
  not fullscreen) with a **visible** scrollbar (`scrollbarState` in
  `profiles.defaults`) and defaults to the fixed `PowerShell 7` profile only
  when `defaultProfile` is empty or still the built-in Windows PowerShell 5.1
  default; a custom default is left alone. Adding a new unconditional top-level
  scalar fragment key requires the canonical fragment plus chezmoi template
  mirror and `home/.chezmoitemplates/windows-terminal/merge-settings.ps1` to
  change together. Pester's transaction cases and `windows_render_test.sh`
  exercise the production helper directly; do not reintroduce a parallel test
  implementation. `profiles.defaults` is
  replaced wholesale, so keys inside it (e.g. `scrollbarState`) need no
  merge-template change. The nvim tree is intentionally NOT
  copied under `home/`: POSIX `home/dot_config/symlink_nvim.tmpl` and the
  redirected-known-folder source `windows/chezmoi-localappdata/symlink_nvim.tmpl`
  both point at the repo top-level `nvim/` directory. Windows nvim is therefore still a
  directory symlink and still needs Developer Mode or elevation; the
  no-Developer-Mode win applies to simple copied files. Do not use `exact_` for
  nvim; app runtime state lives outside `.config/nvim`, but user/plugin-added
  config files should not be deleted by chezmoi. `home/.chezmoiignore` must gate
  whole wrong-OS directories to avoid empty parent dirs. Windows setup resolves
  the actual LocalApplicationData, ApplicationData, and Documents known folders
  plus runtime `$PROFILE`, then applies dedicated source states for
  nvim/lazygit, Herdr, and the Console/VS Code/ISE PowerShell profiles. After
  proving each deployed profile resolves to `shells/powershell_profile.ps1` or
  is an exact byte copy, setup removes Mark-of-the-Web from only those validated
  profile files; failure is fatal so a successful install cannot leave every new
  terminal broken under `RemoteSigned`. Never restore hardcoded `home/AppData`
  or `home/Documents` targets. POSIX pwsh
  profile management remains
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
  per-tool functions. Identity-sensitive entries must pass the complete
  predicate shape: zsh plugins include target, expected origin, pinned commit,
  and required entry file even during the read-only table scan.
- **`--update` is full reconciliation followed by a scoped drift-edge refresh,
  not a repo fetch.** `setup.sh --update` / `setup.ps1 -Update` first run the
  same install-or-migrate path as all mode: verified Nix bootstrap when needed,
  the pinned package layer, missing native/deferred dependencies, config apply,
  Lazy restore, synchronous parser installation, Mason install, and Sentinel.
  They then run `install-deps --update` and
  `util.mason_tools.run_checked('MasonToolsUpdateSync')`. The wrapper runs the
  synchronous command, validates every platform-expected Mason package, and
  exits via `:cquit` on any command/postcondition failure. This closes
  Neovim's normal headless behavior of printing a command error while exiting
  zero. Update mode still never runs git pull, Lazy sync, Lazy update,
  blanket package-manager upgrades, or lockfile rewrites. `--upgrade` /
  `-Upgrade` is an alias. `install-deps --update` updates only present catalog
  tools after proving ownership from the executable source, then runs a scoped
  per-package manager or repo-pinned artifact command (`brew upgrade <formula>`,
  native Linux package-specific upgrade commands, Windows `scoop update <pkg>`
  after one manifest refresh, Windows `winget upgrade --id <id> -e`, or Windows
  `choco upgrade <pkg> -y`). It must not run blanket upgrades such as
  `brew upgrade`, `apt upgrade`, `pacman -Syu`, `scoop update *`,
  `winget upgrade --all`, or `choco upgrade all`, and it must not touch PSFzf,
  `lazy-lock.json`, or configs. Unix update mode is per-tool, not one global
  active-manager pass: Homebrew/Linuxbrew requires the PATH-visible command path
  and its resolved executable target to stay under `brew --prefix`, the
  installed formula, and `brew list --formula <formula>` file ownership of the
  resolved executable. The catalog formula is only the install default: when a
  different installed versioned formula owns the active Cellar target (for
  example `python@3.14` instead of `python@3.12`), update mode verifies that
  formula's receipt and updates the actual owner. apt/dnf/zypper/pacman/apk require
  the manager's file-ownership proof for the resolved real path; repo-pinned
  direct Linux artifacts (`nvim`, `lazygit`, `starship`, `tree-sitter`,
  `chezmoi`, and `herdr`) are owned only when their durable marker matches the repo-pinned
  version, URL, SHA-256, command path, binary path, install root, installed
  binary SHA-256, executable `--version` output, and supported install shape:
  Neovim is `/usr/local/bin/nvim` pointing into `/opt/nvim-linux-*`; lazygit and
  Starship are `/usr/local/bin/<tool>` or `~/.local/bin/<tool>`; tree-sitter,
  chezmoi, and Herdr are `~/.local/bin/<tool>`. A Brew-prefix command symlink that resolves
  outside the Brew prefix is a blocked ownership contradiction, a shadow command
  path that resolves to the same binary is not ownership, an unsupported
  direct-artifact root is not ownership, and a marker binary outside the
  recorded install root is corrupt provenance. Legacy unmarked direct binaries
  remain `unmanaged`. Homebrew current packages must
  print `current` without running `brew upgrade`; Homebrew outdated detection
  must treat an exact `brew outdated --formula --quiet <pkg>` stdout row as
  outdated even when Brew returns nonzero, because Homebrew uses that exit state
  for named outdated formulae. Apt current packages must not run
  `apt-get install --only-upgrade` after a successful metadata refresh proves
  installed == candidate. Pacman-owned tools are `skipped` because package-level
  updates would violate Arch's explicit full-system-upgrade model. `/bin/zsh` on
  macOS is `system`. Normal macOS developer tools that still resolve from
  `/usr/bin` are `unmanaged` with a Homebrew migration hint. Setup owns the
  Homebrew shellenv block, Homebrew GNU Make `libexec/gnubin` PATH adoption when
  the `make` formula is installed, and completion reconciliation after Homebrew
  activation in both install and update mode. `brew completions link` owns tap
  completions but Homebrew 6 does not repair its own `_brew`; setup must also
  locate the active core completion across official Homebrew, nix-homebrew, and
  Linuxbrew layouts, atomically publish only a missing/symlink destination, and
  verify resolved source identity. A non-symlink conflict fails closed; do not
  document a hidden manual repair step instead. Scoop ownership must use shim metadata as
  the first proof layer:
  a command source under `...\scoop\shims` must parse the sibling `.shim` target
  and match `...\scoop\apps\<catalog-package>\...` before any package-list
  fallback. A resolved command source outside Scoop must not be claimed by
  `scoop list`. A corrupt or mismatched Scoop shim is a blocked update failure,
  not an unmanaged tool and not a reason to fall through to winget/Chocolatey.
  Winget and Chocolatey ownership require both an exact package-list row and an
  active command source under that manager's supported install roots; a manual
  `C:\Manual\...\pwsh.exe` is `unmanaged` even if winget/Chocolatey lists the
  package. Windows updates require a manager-specific non-mutating availability
  proof before mutation: Scoop uses the structured `scoop status` row for the
  exact `Name` only when `Latest Version` is non-empty and both `Info` and
  `Missing Dependencies` are empty, winget uses
  `winget list --upgrade-available --id <id> -e --accept-source-agreements`, and
  Chocolatey uses `choco outdated --limit-output`. No matching available update
  is reported as `current`, while a failed availability query appends to
  `InstallFailures`. Status vocabulary is stable across OSes: `updated`,
  `current`, `system`, `unmanaged`, `blocked`, and `skipped`; `blocked` exits
  nonzero, while `unmanaged` exits successfully with the resolved source path.
  Do not use vague "present, but <manager> does not
  manage" wording.
- **Accepted dependency install failures are fatal.** `install-deps.sh` records
  package-manager/direct-install failures and exits nonzero after the run
  summary; every recoverable main-flow install goes through `run_install_step`,
  which prevents `set -e` from bypassing later independent work and records a
  nonzero callee exactly once when the callee did not already record a more
  precise failure. Dry-run previews and explicit/manual skips remain
  non-failures. Immediate exit is reserved for documented unsafe preconditions
  such as an unsupported package-manager boundary.
  Accepted optional GUI/package paths still follow that contract: Ubuntu/snap
  Ghostty, VS Code cask/snap/flatpak, devilspie2, and Alpine native package arms
  record failures once the user accepted the install or `--all` selected it.
  `setup.ps1` must propagate `install-deps.ps1` nonzero exits in both normal
  Phase 1 and `-Update`; it must never finish with `exit 0` after a blocked
  dependency update. Preserve the Windows `$InstallFailures` summary contract
  rather than weakening it into warning-only output.
- **Sentinel is setup Phase 6/6 and is opt-out, not experimental.** Full setup
  (`--all` / `-All`) applies Sentinel's renamed `0.1.2` tree at exact commit
  `ecafffa858666343c1639f996d177f460163e93e` unless
  `--skip-agents` / `-SkipAgents` is passed. Interactive setup asks
  `Apply Sentinel global agent rules? [Y/n]`. The setup phase clones Sentinel into
  a dotfiles-owned cache (`~/.local/share/dotfiles/sentinel/<commit>` on POSIX,
  `%LOCALAPPDATA%\dotfiles\sentinel\<commit>` on Windows), verifies the checkout
  commit and `VERSION`, and performs all Sentinel Git operations with
  system, global, environment-injected config, templates, hooks, and executable
  Git config features disabled. It then runs Sentinel's Bash global installer
  (`tools/install --global`), then runs its global check. Windows setup invokes
  the same Bash installer through a validated Git Bash (`cygpath` must be
  present) with Git Bash's POSIX-only PATH for the `0.1.2` pin; do not use
  Sentinel `tools/install.ps1` for global installs unless a newer Sentinel pin
  proves the PowerShell path in CI. Do not inline or reimplement Sentinel
  rendering here. Project/team Sentinel adoption is a separate repo-local install
  or vendoring decision. The published `v0.1.2` tag predates the repository
  rename, so it must not be asserted as the renamed tree's identity.
- **A C compiler is installed so LuaSnip can build `jsregexp`.** Without one,
  the nvim Lazy build prints "No C compiler found" and `jsregexp` is skipped
  (LuaSnip still works, minus JS-regex snippet transforms). POSIX installs the
  native toolchain (`build-essential` / `gcc` / `base-devel`), only if none of
  `cc`/`gcc`/`clang`/`zig`/`cl` is already present. On Windows a clean machine
  has none, so `install-deps.ps1` carries **`zig`** in `$Catalog` (LuaSnip
  detects `zig cc`); it is skipped if already installed like any other tool.
- **The CMake LSP needs the real `cmake` CLI.** Mason installs `neocmakelsp`,
  but that server shells out to `cmake`; setup therefore installs `cmake` through
  every supported OS package manager. Do not remove the CLI dependency while
  keeping `neocmake` enabled, or strict Tier 2 smoke can hang on a crashing
  local language server. The `neocmake` config must also prefer Mason's real
  package executable (`mason/packages/neocmakelsp/neocmakelsp[.exe]`) over the
  `mason/bin` PATH shim. On Windows that shim is a `.cmd` wrapper; stopping the
  wrapper can leave `neocmakelsp.exe` alive after headless nvim prints a passing
  smoke result, holding CI open until the workflow timeout.
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
  asset (v0.26.10) into `~/.local/bin` with SHA-256 verification. `install-deps.ps1` installs the CLI through the Scoop
  `tree-sitter` manifest first and falls back to `npm install -g
  tree-sitter-cli` after Node is present. Windows compiler support is separate:
  `install-deps.ps1 -All` auto-installs Visual Studio 2022 Build Tools with the
  `Microsoft.VisualStudio.Workload.VCTools` workload through winget or choco,
  then falls back to Microsoft's official `vs_BuildTools.exe` bootstrapper with
  the same workload. Scoop does not carry VS Build Tools, so this is the
  deliberate exception to the Scoop-first catalog rule. The bootstrapper is
  downloaded first, then must pass Authenticode verification with `Status=Valid`,
  a real `X509Chain` built from `SignerCertificate`, and Microsoft-owned signer
  plus root/chain identity checks before `Start-Process` may run. A failed
  winget/choco pass is not final; a failed package-manager-plus-bootstrapper
  pass records an `InstallFailures` entry so `-All` cannot report success
  without MSVC.
  `setup.ps1` imports the VS DevShell into the current process before headless
  Tree-sitter parser installation, so `tree-sitter build` inherits `cl.exe`,
  `INCLUDE`, and `LIB`. Do not put the DevShell import in the PowerShell
  profile. Zig stays installed for LuaSnip `jsregexp`, but do not
  wire zig into nvim-treesitter main: the old `master` branch could use it,
  while main emits MSVC-style `cc` crate flags that require MSVC. Ad-hoc
  `:TSUpdate` parser rebuilds on Windows should run from a "Developer
  PowerShell for VS" shell or after rerunning setup. Windows release-asset
  selection prefers `RuntimeInformation.OSArchitecture` and falls back to the
  native `PROCESSOR_ARCHITEW6432` / `PROCESSOR_ARCHITECTURE` identity when the
  runtime reports an empty value during greenfield bootstrap.
- **Markdown equations use a pinned `pylatexenc` converter.** render-markdown's
  LaTeX support needs the non-bundled `latex` parser (already in
  `treesitter_parsers`) and a converter executable. `install-deps.sh` creates
  `~/.local/share/dotfiles/python-tools/pylatexenc`, installs pinned
  `setuptools==80.9.0` first, installs `pylatexenc==2.10` with pip `--require-hashes`
  and `--no-build-isolation`, and writes `~/.local/bin/latex2text`;
  interactive zsh prepends `~/.local/bin`. `install-deps.ps1` creates the same
  venv under
  `%LOCALAPPDATA%\dotfiles\python-tools\pylatexenc` and adds its `Scripts`
  directory to User PATH. Keep `nvim/lua/plugins/markdown.lua`'s converter set
  to `latex2text` so machines use the setup-owned converter rather than
  opportunistically preferring unrelated host tools.
- **Synchronous nvim-treesitter bootstrap is serialized by design.**
  nvim-treesitter `main` defaults to high parallelism for interactive installs.
  That is not the setup/CI contract: hosted runners restore parser caches and
  can expose temp-dir races such as `ENOTEMPTY` while compiling many grammars.
  Lazy's nvim-treesitter build callback must call the upstream waitable update
  API with `max_jobs = 1`, wait up to 15 minutes, and require exactly `true`
  before Lazy restore returns. Never replace it with command-form `:TSUpdate`:
  the command is also a Lazy load trigger whose config starts the declared
  parser install asynchronously, and its update task is asynchronous too.
  Either can return while compilers are still publishing parser/query output,
  racing the next setup phase.
  In addition, ordinary headless config loads must not start the interactive
  async auto-install path. Only a real UI session or
  `DOTFILES_TREESITTER_SYNC_INSTALL=1` may call the declared-parser installer;
  this keeps Lazy restore, Mason, and smoke validators from creating work
  outside Phase 4.
  When `DOTFILES_TREESITTER_SYNC_INSTALL=1`, pass `max_jobs = 1` and wait up to
  15 minutes; interactive installs keep upstream's faster default. Tier 2 then
  checks `get_installed("parsers")` plus managed query output, including
  `highlights.scm` for explicit parser rows, so a partial bootstrap fails at the
  parser/query gate before later capture checks turn it into a vague highlighting failure.
- **nvim-treesitter installer drift must not disable highlighting.** A stale
  lazy.nvim cache can keep `nvim-treesitter` on the frozen `master` API while
  this repo expects the `main` rewrite. In that state
  `require("nvim-treesitter").install` and `.indentexpr` are absent. The config
  must warn and continue registering the `FileType` autocmd that calls
  `vim.treesitter.start()`; never let parser auto-install API drift abort buffer
  highlighting. The recovery path is `:Lazy! restore` followed by `:TSUpdate`, or
  rerun `setup.ps1` on Windows so VS DevShell is imported before parser builds.
- **Regex syntax fallback is capability-driven, not a language allowlist.**
  `vim.treesitter.start()` clears the buffer-local `syntax` option. That can
  make buffers look materially worse even when Tree-sitter is active, because
  the built-in syntax files add useful secondary groups. The FileType autocmd in
  `nvim/lua/plugins/treesitter.lua` therefore runs for every detected filetype,
  starts Tree-sitter only for parser-backed filetypes, and restores `syntax`
  only when Neovim has a matching `syntax/<filetype>.vim` runtime file (or the
  runtime had already set a syntax name before Tree-sitter cleared it). Do NOT
  reintroduce a hand-curated syntax fallback table; add Tree-sitter parsers
  explicitly when richer parser-backed support is desired, and let runtime
  syntax detection cover the long tail. `:Inspect` should show both
  `treesitter` captures and `syntax` groups for parser-backed fallback
  languages; syntax-only filetypes should show syntax groups without
  Tree-sitter captures. Test probes that call `vim.treesitter.start()` a second
  time must preserve any non-empty buffer-local `syntax` value already restored
  by production config; the second start clears it again.
  Filetypes with no supported nvim-treesitter parser, such as `.curlrc`, must be
  tested as syntax-only rows instead of inventing parser names, and Tier 2 must
  probe at least one meaningful syntax position for important syntax-only rows.
  `.curlrc` maps
  to Neovim's generic `conf` filetype because upstream Neovim has no dedicated
  curl config syntax.
- **`uninstall.sh` / `uninstall.ps1` are greenfield teardown tools, not purge.**
  They enumerate targets with `chezmoi --source <repo>/home managed --path-style
  absolute`, remove only repo-owned symlinks or byte-identical Windows
  copy-mode files, restore bootstrap-style `<target>.bak.<timestamp>[.n]`
  backups by validated filename timestamp/collision order (never mtime), and
  leave chezmoi's own config/state alone. Malformed or ambiguous candidates fail
  before target removal. Windows restores stable packaged, Preview packaged,
  Canary, and portable WT backups only after validating all four canonical paths,
  atomically preserves the displaced current file
  as `settings.json.uninstall-current.*`, and honors `-NoRestoreBackups`.
  Dry-run mode must also leave empty external parent directories in place; it
  prints `would:` lines only and does not prune `~/.local/share/dotfiles`.
- **Starship binary install paths differ by OS.** Homebrew owns
  macOS/Linuxbrew, Alpine uses the native `starship` apk package, Windows setup
  installs it through Scoop/winget/choco, and other native Linux/WSL hosts
  without brew use a pinned Starship GitHub release tarball with SHA-256
  verification.
- **Terminal scrollback units are not interchangeable.** WezTerm retains
  5,000,000 lines per tab. Ghostty retains a lazily allocated 1 GiB byte budget
  per terminal surface because `scrollback-limit` is bytes, not lines. Windows
  Terminal stable/Preview/Canary/portable receive `historySize = 32767`, the
  upstream hard maximum; do not claim that WT can retain millions of lines.
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
- **`Update-ManagedCatalogTool` is the Windows update dispatcher.** It detects
  exact ownership through Scoop, winget, then Chocolatey, and dispatches only
  one scoped package update. Scoop ownership is proven from shim metadata before
  any package-list fallback: the sibling `.shim` file is authoritative for
  mapping command binaries such as `rg.exe` back to package names such as
  `ripgrep`. If the resolved command source is outside Scoop, `scoop list` must
  not claim it. If a command resolves through Scoop shims but the metadata is
  missing, unreadable, outside the apps tree, or mapped to a different package,
  update mode records a blocked Scoop provenance failure and must not fall
  through to another manager. Winget/Chocolatey package-list ownership is not
  enough to run an update: the active command source must also live under the
  supported install roots for that manager/package. A manual shadow command is
  `unmanaged`, not a manager-owned update target. A manager-specific
  availability probe must then prove the package is actually outdated before any
  mutating package command runs: Scoop filters structured `scoop status` rows by
  exact `Name` plus non-empty `Latest Version` and fails closed on non-empty
  `Info` or `Missing Dependencies`, winget filters
  `winget list --upgrade-available --id <id> -e`, and Chocolatey filters
  `choco outdated --limit-output`. No matching update is reported as `current`,
  and a failed availability query is a real update failure. `Update-ScoopTool`
  remains the only Scoop-specific update path and is intentionally
  single-package; never replace it with `scoop update *`, `winget upgrade --all`,
  `choco upgrade all`, or another blanket upgrade. Failed manifest refreshes,
  availability checks, or package updates append to `InstallFailures`, so update
  mode exits nonzero when a scoped refresh did not actually succeed. Present
  tools outside Scoop/winget/Chocolatey are reported as unmanaged and do not
  count as successful dotfiles-owned updates.
- **Windows CI uses a pinned, verified elevated Scoop bootstrap.** GitHub-hosted
  `windows-2025` runners are elevated, and Scoop blocks elevated install by
  default. `Install-Scoop` downloads `ScoopInstaller/Install` at the pinned
  `$ScoopInstallerCommit` (`b0ee913725139b816f9178163af0aecdba07a7ed`),
  verifies `$ScoopInstallerSha256`
  (`48f6ea398b3a3fa26fae0093d37bd85b13e7eaa5d1d4a3e208408768408e35ae`), runs the local temp
  installer with `-RunAsAdmin` only when elevated, then adds the Scoop `shims`
  dir to the current process PATH so the rest of `install-deps.ps1` can
  immediately use `scoop`. The bootstrap must preserve the caller's process
  execution policy. In particular, changing a greenfield setup launched with
  `Bypass` to `RemoteSigned` strands Mark-of-the-Web repository helpers such as
  the Windows Terminal merger later in the same process.
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
  portable. `setup.ps1` has the same source-only portability requirement for
  tests; its default checkout path must derive from `USERPROFILE`, then `HOME`,
  then .NET's user-profile folder instead of assuming `USERPROFILE` exists.
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
- **Every production apt mutation is explicitly noninteractive.** Route apt
  update/install/upgrade calls through `apt_get_noninteractive`, which invokes
  `sudo env DEBIAN_FRONTEND=noninteractive apt-get ...` so the setting survives
  sudo's environment filtering. This prevents transitive debconf packages such
  as `tzdata` from blocking `setup.sh --all`; keep the verified Ghostty and
  WezTerm `.deb` paths on the same boundary. Guarded by the apt resilience and
  pinned `.deb` install tests.
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
- **Direct network executables are pinned and verified.** New installer code must
  prefer release artifacts with adjacent SHA-256 verification over fetched
  installer scripts. Bootstrap scripts are acceptable only when pinned to an
  immutable commit and hash-verified before execution; current examples are the
  Homebrew installer and Windows Scoop bootstrap. `install-deps.ps1` downloads
  `ScoopInstaller/Install` at `$ScoopInstallerCommit`, verifies
  `$ScoopInstallerSha256`, then executes the checked temp file. Microsoft's
  moving `vs_BuildTools.exe` alias is the reviewed exception: it must pass
  Authenticode `Status=Valid` plus Microsoft-owned signer/chain checks before
  execution. The static scanner keeps only genuinely pinned+verified exceptions,
  including CI cargo-binstall where the SHA-256 check immediately precedes
  execution. Recommended setup docs use `git clone` plus local `setup`, not raw
  remote setup script execution from the current default branch.
- **Direct GitHub downloads are pinned and SHA-256 verified.** `install-deps.sh`
  verifies the pinned Homebrew installer script, Neovim Linux tarballs,
  native-Linux chezmoi tarballs, lazygit Linux tarballs, Starship Linux
  tarballs, tree-sitter CLI Linux archives, the WezTerm Ubuntu `.deb`, Herdr
  Linux binaries, the Herdr Windows preview `.exe`, and Hack Nerd Font zip before extraction;
  CI also verifies the pinned chezmoi Linux, macOS, and Windows release
  archives used by the parity jobs;
  `install-deps.ps1` verifies the pinned Scoop installer before execution, the
  pinned Hack.zip before registering fonts, the pinned Windows Terminal
  portable zip before extracting the fallback install, the exact compatible
  Tree-sitter CLI release before transactional publication (including a
  same-parent stage whose filename still ends in `.exe` so Windows can execute
  the staged proof), and the pinned Herdr
  Windows preview `.exe` before copying it into `%LOCALAPPDATA%\Programs\Herdr\bin`.
  Windows direct-artifact publication must put the owned bin directory first
  in both process and User `PATH`, de-duplicating an existing later entry. This
  preserves an incompatible preinstalled tool while ensuring the verified
  artifact actually wins command resolution; mere PATH membership is not a
  publication proof.
  On macOS, Hack Nerd Font presence accepts either fontconfig discovery or the
  exact `font-hack-nerd-font` Homebrew cask receipt. The receipt is authoritative
  before `fc-list` has indexed Apple's font directories and prevents a repeated
  setup from needlessly reinstalling the already-present cask.
  The same install-mode rule applies to Ghostty, WezTerm, and AeroSpace: either
  their command or exact Homebrew cask receipt proves presence. `--all` installs
  missing apps and never turns an existing receipt into an implicit upgrade.
  POSIX helpers that unpack
  into `mktemp -d` install a cleanup trap
  immediately after creating the directory, so failure paths do not leak
  archives or partial extracts. A Hack.zip checksum mismatch records a `FAIL:`
  install marker and does not extract. A successful Windows font install broadcasts
  `WM_FONTCHANGE` best-effort so Windows Terminal can re-enumerate fonts without
  making setup depend on that notification. The CI workflows also pin and
  verify their direct GitHub downloads.
  This extends to **Ghostty Debian packages**: never execute
  `mkasberg/ghostty-ubuntu`'s `install.sh`. Even at a pinned script tag it queries
  mutable `releases/latest` and downloads a `.deb` whose bytes are not bound to
  the reviewed script. `resolve_ghostty_deb_asset` instead maps the reviewed
  Debian-family distro versions and `amd64`/`arm64` architecture to one exact
  release asset and adjacent SHA-256. `install_verified_ghostty_deb` verifies
  nonempty bytes, `Package=ghostty`, exact architecture, and exact dpkg version
  before passing only that local file to privileged apt; it then proves the
  installed dpkg version and executable. Download, digest, metadata, apt, and
  post-install failures clean staging, enter the one install-failure summary,
  and print recovery instructions where apt may have changed state. The static
  privileged-package scanner understands `maybe_sudo` plus the shared
  `verify_sha256` helper and self-tests both unverified and verified flows.
  Update the release version and all applicable distro/architecture checksum
  constants together. zsh plugin refs are also
  pinned by tag plus expected commit; update both after reviewing a Renovate tag
  bump. Guarded by `tests/shell/ghostty_install_test.sh`,
  `tests/shell/ghostty_install_fail_test.sh`,
  `tests/shell/wezterm_install_test.sh`, `tests/shell/wezterm_install_fail_test.sh`,
  `tests/shell/herdr_install_test.sh`, `tests/shell/herdr_install_fail_test.sh`,
  `tests/shell/homebrew_completions_test.sh`,
  `tests/shell/wsl_gui_tools_test.sh`, `tests/shell/lazygit_install_test.sh`,
  `tests/shell/starship_linux_install_test.sh`,
  `tests/shell/treesitter_cli_test.sh`, and `tests/shell/zsh_plugins_test.sh`.
  Renovate can open version/ref bumps for these constants and for the CI
  cargo-binstall installer commit and Renovate validator package/runtime pins,
  but it cannot recompute adjacent SHA-256
  values or verify tag commit IDs; leave CI red until a human has reviewed the
  download/ref and updated the adjacent constant. The
  `CHEZMOI_VERSION`, `STARSHIP_VERSION`, `TREE_SITTER_CLI_LINUX_VERSION`,
  `WEZTERM_VERSION`, `HERDR_VERSION`, and `PI_CLI_VERSION` custom managers follow the lazygit shape:
  Renovate may bump the version constants, while their adjacent SHA-256 values
  or npm integrity values remain context only. In
  `renovate.json`, direct-download SHA-256 values must be matched as context
  only, not named `currentDigest`, otherwise Renovate will schedule same-version
  digest updates for checksums it cannot actually resolve.
- **Sentinel is pinned by immutable Git commit + `VERSION`, never a moving
  branch or a mismatched tag.** Setup may clone from GitHub, but it must checkout
  the exact `SENTINEL_REF`, assert `SENTINEL_VERSION`, reject dirty cached
  worktrees, and run only the installer
  from that verified checkout. Clone, checkout, and cache validation must all
  use the Sentinel Git wrapper: do not trust mutable system,
  global, environment-injected, template, hook, or `.git/config` state. Cache
  validation must force the intended cache path with `--git-dir`/`--work-tree`
  semantics and disable executable Git config features such as `core.fsmonitor`,
  so `core.worktree` redirection or fsmonitor hooks cannot run or hide modified
  files before the installer executes. The current renamed tree has no matching
  release tag, so the exact commit is the immutable authority; never claim a tag
  until it resolves to the selected commit. Updating Sentinel means changing the
  commit and version constants together, updating README/CLAUDE references, and
  keeping shell/Pester/static pin tests green.
- **Dependency installers own the "install EVERYTHING?" prompt; Sentinel owns a
  separate global-policy prompt.** Interactive runs that didn't pass
  `--all`/`-All` can get the dependency prompt; answering yes flips
  `YES_ALL`/`$All` for dependency prompts. Phase 6 asks
  `Apply Sentinel global agent rules? [Y/n]` unless `--all`/`-All`,
  `--dry-run`/`-DryRun`, no tty/user interaction, or `--skip-agents` /
  `-SkipAgents` already made that decision. Once the dependency installer has
  accepted a mutation, it also owns downstream package-manager consent.
  The verified Homebrew bootstrap runs with its supported `NONINTERACTIVE=1`
  environment after setup consent, so it must not pause at its separate
  "Press RETURN/ENTER" directory-creation prompt.
  Homebrew 6 enables ask mode by default for `brew install` and `brew upgrade`,
  so `install-deps.sh` clears inherited `HOMEBREW_ASK` and exports the official
  `HOMEBREW_NO_ASK=1` setting inside its own process. Do not replace this with
  piped `yes`, do not export it into the user's shell, and do not let one
  accepted package produce a second Homebrew confirmation. Password and OS
  permission boundaries remain interactive. Guarded by
  `tests/shell/homebrew_no_ask_test.sh`, which exercises both the bootstrap
  child environment and a real `brew install` call shape.
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
  `source-file -q ~/.tmux.posix.conf` — a silent no-op for real tmux when the
  file is absent. Keep the tilde path unquoted because psmux is stricter than
  real tmux about quoted path expansion. Do not rely on `source-file -q` for
  psmux-only startup includes; psmux v3.3.x does not implement that config flag,
  so `tmux/psmux.conf` source-files the Windows overlay explicitly without
  flags. On POSIX the overlay re-binds `y` to the native CLI when one exists. This is the
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
- **Herdr + PSReadLine: Windows must select PowerShell 7 explicitly.** Herdr's
  unset Windows pane shell falls back to Windows PowerShell, which has a
  different profile/history store and cannot provide the configured PSReadLine
  ListView experience. `herdr/config.windows.toml` must keep
  `terminal.default_shell = "pwsh.exe"`; do not add that setting to the shared
  POSIX config because it would replace the user's normal Unix shell. Herdr
  keeps the shell of existing panes, so recreate panes (or stop/restart the
  session) after changing this startup setting.
- **Herdr `prefix+w` is the full navigator on every host.** Upstream's default
  `prefix+w` opens only workspace navigate mode, which appears inert with one
  workspace and does not match tmux's session/window/pane tree. The managed
  configs disable `workspace_picker` and bind both `prefix+w` and the upstream
  `prefix+g` alias to `goto`, Herdr's searchable workspace/tab/pane navigator;
  Up/Down selects and Enter focuses. Named Herdr sessions remain separate server
  namespaces and cannot be listed inside another session's navigator.
- **Herdr preserves the common tmux rename and workspace-selection muscle
  memory.** Herdr calls tmux windows `tabs`, so `rename_tab = "prefix+comma"`
  implements `prefix+,`; `rename_workspace = "prefix+$"` implements
  `prefix+$`. Ordered workspace movement is `prefix+Up` / `prefix+Down`, and
  direct selection is `prefix+Shift+1..9`, leaving unshifted `prefix+1..9`
  exclusively for tabs/windows. Use the literal `$` binding: Herdr accepts
  single-character punctuation, while `prefix+shift+4` does not match the
  legacy terminal `$` event on every input path.
- **Herdr agent navigation is a third, non-destructive modifier layer.** Use
  `prefix+a` / `prefix+Shift+A` for next/previous agent: `a` is mnemonic and
  unbound by both stock tmux and Herdr. Direct agent focus uses
  `prefix+Ctrl+1..9`, matching Herdr's digit-only indexed-binding shape and the
  existing number ladder: bare number selects a tab, Shift+number selects a
  workspace, and Ctrl+number selects an agent. Plain Ctrl+number is unbound by
  stock tmux, AeroSpace, and the managed Ghostty/WezTerm keymaps. Do not use
  Alt+number here: AeroSpace owns it globally for macOS workspaces. Also do not
  consume tmux's daily-use copy/paste, pane, window, session, create, detach,
  rename, navigator, or bare-number bindings for agent actions.
- **psmux + PSReadLine: Windows-only overlay, two settings.** psmux's default
  shell is **cmd**, not pwsh — which is the *real* reason "history prediction"
  and `MenuComplete` looked broken inside panes: PSReadLine was never loaded.
  The fix is a Windows-only overlay `tmux/tmux.windows.conf`, managed as
  `~/.tmux.windows.conf` by chezmoi on Windows and pulled in by
  `tmux/psmux.conf` via unquoted, flag-free
  `source-file ~/.tmux.windows.conf`. The shared `tmux.conf` also carries
  `source-file -q ~/.tmux.windows.conf` for real tmux compatibility, but that is
  not a valid psmux startup include path. The overlay sets:
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
- **zoxide is the cross-shell smart-cd (`z` / `zi`), wired without remote eval.**
  `install-deps.sh` carries `zoxide` in `PKG_TABLE` (same package name on every
  supported PM: brew/apt/dnf/pacman/zypper/apk) and `install-deps.ps1` carries it
  in `$Catalog` (winget `ajeetdsouza.zoxide` / choco / scoop). `shells/zshrc`
  runs a `command -v zoxide`-guarded `eval "$(zoxide init zsh)"` **after**
  compinit — upstream requires post-compinit placement for its completions
  (guarded by `zsh_plugins_test.sh`). The PowerShell profile does NOT use
  zoxide's documented `Invoke-Expression` one-liner (banned here); instead it
  caches `zoxide init powershell` to `zoxide.ps1` and dot-sources it, using the
  SAME atomic temp-file + `Move-Item -Force` publish and retry-import helpers as
  Starship (`Confirm-/Publish-/Import-ZoxideInitScript`). There is no config file
  to stat, so the cache is regenerated only when missing or when `zoxide
  --version` changed (a sidecar `zoxide.ps1.version` stamp). zoxide inits AFTER
  Starship so its prompt hook wraps Starship's prompt. No `--cmd cd` override:
  plain `cd` is untouched. `invariants_test.sh` bans `Invoke-Expression`/`iex` in
  the PowerShell profiles; do not reintroduce the one-liner.
- **gh-dash is a pinned gh CLI extension, not a package.** `gh` is in both
  catalogs (`PKG_TABLE`: `gh` on brew/apt/dnf/zypper, `github-cli` on
  pacman/apk; `$Catalog`: winget `GitHub.cli` / choco `gh` / scoop `gh`).
  gh-dash itself has no brew/apt/scoop package. Tag `v4.25.1` is paired with
  annotated tag object `e6ebbd7e83e30161b9192ce3339972d2c8269e7f` and peeled
  commit `49f37e4832956c57bf52d4ea8b1b1e5c0f863700`; both installers verify that
  remote mapping before mutation and run
  `gh extension install dlvhdr/gh-dash --pin v4.25.1`. gh requires a release
  tag for binary extensions; commit refs are accepted only for script extensions.
  (`GH_DASH_VERSION` in `install-deps.sh`, mirrored as `$GhDashVersion` in
  `install-deps.ps1`; a Renovate `github-releases` manager can bump the tag and
  `pin_consistency_test.sh` fails on sh/ps1/CLAUDE drift). The installers
  (`install_gh_dash_extension` / `Install-GhDashExtension`) are gated on `gh`
  being present **and authenticated** (`gh auth status`): an unauthenticated
  `gh extension install` hits GitHub's anonymous API rate limit and fails, so
  when unauthenticated they skip cleanly (NOT a FAIL) and tell the user to run
  `gh auth login` and rerun. They verify the *installed* pin (not mere presence)
  and re-pin (`gh extension remove dash` + commit install) on mismatch; they are
  consent-gated, dry-run-safe, and emit-FAIL-and-continue only on an
  *authenticated* install failure (non-critical, like the zsh plugins). In
  PowerShell the read-only `gh` probes go through `Invoke-GhProbe`, which resets
  `$global:LASTEXITCODE` so a probe's nonzero exit never leaks into the script's
  exit code under `$PSNativeCommandUseErrorActionPreference` (that leak made
  Windows dry-run exit 1 after "install-deps: done"). Its config is a single
  same-path file on every OS: `~/.config/gh-dash/config.yml` on POSIX and
  `%USERPROFILE%\.config\gh-dash\config.yml` on Windows (XDG-style, NOT
  `%LOCALAPPDATA%`), chezmoi-managed from canonical `gh-dash/config.yml` (parity
  row in `parity_gate.sh`); the config is applied regardless of auth — only the
  extension binary is auth-gated. Running the dashboard needs `gh auth login` —
  a manual, secret-bearing step this repo never automates or stores.
- **Pi CLI is a pinned npm package, not synced `.pi/` state.** `install-deps.sh`
  and `install-deps.ps1` run `npm pack --ignore-scripts --json` for
  `@earendil-works/pi-coding-agent@0.80.3`, require both reported metadata and
  independently hashed tarball bytes to match
  `sha512-TIggw9gCXpA+Ph7OjdTA7ka2NPwTVuPmy39KDSyUzaKq8VvHfMGR7vtRz4JB7Um/RMRblmzhu4p9tUCk6MTgGA==`
  and install only the verified local tarball. Pack state is scoped to a unique
  temp directory and cleaned through return/signal/finally paths. POSIX public
  setup gets Node 24 from the enforced Nix package layer; Windows
  gets Node through the native catalog. The CLI binary is provisioned on every
  OS, but `.pi/` sessions, auth, and preferences remain machine-local. Renovate
  may bump `PI_CLI_VERSION`, but the integrity constant is context-only and must
  be recomputed/reviewed by a human. PowerShell probes Node and npm by capturing
  native output before selecting its first line; piping the native process into
  `Select-Object` loses `LASTEXITCODE` in a fresh shell and falsely rejects a
  compatible Node or skips npm global-prefix PATH publication.
- **which-key.nvim is the only keymap-hint plugin.** `nvim/lua/plugins/which-key.lua`
  loads it on `event = "VeryLazy"` (never eager — only `rose-pine.lua` may load
  eagerly, invariant 7) with `opts = {}` and a `<leader>?` popup of buffer-local
  keymaps. It only *displays* existing keymaps, so it never contends with
  conform/telescope/gitsigns for a chord; `:checkhealth which-key` must stay free
  of overlap/duplicate errors. Refresh `nvim/lazy-lock.json` only via the
  documented Lazy path (`Lazy! restore` then `Lazy! install`, or `Lazy! sync`);
  guarded by `tests/nvim/spec/which_key_spec.lua` and the startup budget.
- **lsd owns interactive `ls` ergonomics and its own Rose Pine theme.**
  `install-deps.sh` installs `lsd` through each supported OS package manager;
  `install-deps.ps1` carries it in the Scoop-first catalog (`lsd` -> winget
  `lsd-rs.lsd` -> choco `lsd`). Chezmoi deploys
  `~/.config/lsd/config.yaml` and `colors.yaml` from `lsd/`; the parity
  manifest must keep both files byte-identical between `lsd/` and
  `home/dot_config/lsd/`. zsh defines the upstream documented aliases (`ls`,
  `l`, `la`, `lla`, `lt`) only when `lsd` is on PATH. PowerShell removes the
  built-in `ls` alias only after `lsd` is present, then defines functions for
  the same names because PowerShell aliases cannot carry arguments like `-l` or
  `--tree`. Keep both profiles guarded so partially provisioned shells stay
  silent and usable. `lsd` uses `LS_COLORS` for file/directory names and
  `colors.yaml` for long-list metadata; shell profiles intentionally replace
  ambient/system `LS_COLORS` with the repo's Rose Pine palette so `ls`, `la`,
  `lla`, and direct `lsd -la` render consistently across machines. Set
  `DOTFILES_LS_COLORS` before shell/profile startup for an explicit palette
  override. The repo palette must cover normal file types and special
  directory classes such as `ow`, `tw`, and `st` so world-writable and sticky
  directories do not fall back to upstream/default backgrounds. `NO_COLOR`
  remains an explicit opt-out and must not be unset by the profiles.
- **`ls`/`Get-ChildItem` directories are gold via LS_COLORS and `$PSStyle`.**
  The default directory color (bright blue) is unreadable on Rose Pine dark.
  `LS_COLORS` paints `lsd` directories gold on every shell. `$PSStyle.FileInfo`
  still covers native PowerShell `Get-ChildItem` when users bypass the `lsd`
  functions. Guard `$PSStyle` with `if ($PSStyle)` (absent on Windows PowerShell
  5.1 and pwsh < 7.2); use `$PSStyle.Foreground.FromRgb(0xf6c177)` so the
  source carries no raw ANSI escape byte (keeps the `.ps1` pure-ASCII
  invariant).

## Nix layer (enforced POSIX packages; chezmoi still owns every dotfile)

The `flake.nix` + committed `flake.lock` are the POSIX **package** layer. They own
NO dotfiles — chezmoi does, on every OS (invariant 22). Native Windows is
non-Nix. On macOS/Linux/WSL, public `setup.sh` ensures the checksum-verified Nix
prerequisite through the official-head-before-release / exact-tag-after-release
identity transition and applies this layer before native or deferred dependency
provisioning. The repo never installs Nix through a pipe-to-shell bootstrap.

- **`flake.nix` structure.** `nixpkgs` (nixos-unstable, pinned by `flake.lock`),
  plus `nix-darwin`, `home-manager`, and `nix-homebrew`. Tap repositories are
  deliberately not flake inputs: Homebrew owns mutable tap clones as the target
  user, avoiding root-owned copies that ordinary `brew update` cannot maintain.
  `systems` covers Apple Silicon Darwin plus both
  Linux architectures — there
  is deliberately **no windows system**. Outputs: a packages-only `devShells`,
  a hermetic `checks.<system>.toolchain` (proves nixpkgs resolves the CLI
  toolchain), `formatter = nixpkgs-fmt`, and explicit
  `darwinConfigurations."dotfiles-aarch64"`. The compatibility `"dotfiles"`
  alias is also Apple Silicon; no other Darwin configuration is exported.
- **`nix flake check` in CI does NOT build the darwin toplevel.** It *evaluates*
  `darwinConfigurations.dotfiles` (catching config errors — it caught a
  `nixpkgs.hostPlatform` recursion and a null `home.homeDirectory` during
  development) but reports "(build skipped)", so CI never fetches the multi-GB
  Homebrew taps. `.github/workflows/nix.yml` runs `nix flake check`, the
  `nix fmt --check`, and `tests/nix/run_all.sh` on Ubuntu + macOS. Those matrix
  contexts are checked into the branch-protection sources because the POSIX
  package layer is now enforced by public setup.
- **nix-darwin (`nix/darwin/configuration.nix`).** `nix.enable = false` — the
  **Determinate** daemon owns Nix, so nix-darwin must not fight it. Declarative
  Homebrew: `onActivation` `autoUpdate = false`, `upgrade = false`,
  `cleanup = "none"` (preserve mixed-ownership packages); casks = WezTerm + AeroSpace
  (GUI/vendor apps, never nixpkgs); brews = Herdr. nix-homebrew runs
  `autoMigrate = true` (adopt an existing official-script Homebrew install while
  keeping installed packages), `mutableTaps = true` with an empty
  `nix-homebrew.taps` set so Homebrew owns every tap clone as the target user,
  `trust.taps = [ "nikitabobko/tap" ]` so Homebrew 5 can load the AeroSpace
  personal-tap cask, and `homebrew.taps = [ "nikitabobko/tap" ]`.
  `system.primaryUser` + `users.users.<user>.home` come from setup's validated
  `DOTFILES_TARGET_USER` / `DOTFILES_TARGET_HOME`; pure evaluation alone uses an
  inert `runner` placeholder. On hosts that ran the retired pinned-tap shape,
  setup transactionally moves only the three exact root-owned, non-Git snapshots
  to a transaction root beside `Library/Taps`; activation recreates the required
  third-party tap as the target user, failure restores the snapshots while
  retaining failed output outside Homebrew's scan tree, and success removes the
  transaction root. Exact in-tree recovery artifacts emitted by the broken
  predecessor are automatically relocated to an external recovery root before
  activation. Every unrelated tap/package remains untouched. First bootstrap moves
  existing `/etc/bashrc` and `/etc/zshrc` only to collision-free
  `.before-nix-darwin` backups; failed/interrupted activation restores the old
  files and preserves generated replacements for diagnosis. A retry in the
  terminal that launched first activation resolves the current-system
  `darwin-rebuild` by its installed absolute path even though that shell's
  `PATH` is stale. Nix prerequisite discovery also recovers
  `/nix/var/nix/profiles/default/bin/nix` or `~/.nix-profile/bin/nix` directly
  when a Homebrew `path_helper` refresh removed Nix after the upstream profile
  guard was set. If bootstrap fallback is still required, exact links to
  `/etc/static/bashrc` and `/etc/static/zshrc` are treated as nix-darwin-managed
  and their retained recovery backups remain untouched; an unmanaged source
  plus an existing backup continues to fail closed.
  `tests/macos_owner_lifecycle.sh` is the real Apple Silicon lifecycle smoke:
  from a clean committed checkout it performs install, update, config uninstall,
  reinstall, final update, and full greenfield validation under one
  terminal-owned sudo credential. It requires an idempotent uninstall retry and
  proves no pre-existing Homebrew formula, cask, or unrelated tap disappeared.
  Tap assertions derive `Library/Taps` from `brew --prefix`, never
  `brew --repository`: nix-homebrew deliberately points the repository at
  `.homebrew-is-managed-by-nix` while installed tap clones remain under the
  prefix.
  Uninstall deliberately exercises the documented config teardown; the
  Nix/Homebrew package layer remains installed.
  The mixed-ownership cleanup/tap contract is identical on real Macs and hosted
  CI; no environment marker weakens or changes it.
- **The Docker Linux owner lifecycle retains proof outside the container.**
  `tests/greenfield/docker-linux-owner-lifecycle.sh` exports exact committed
  `HEAD` as a Git bundle, runs the real lifecycle in the digest-pinned image,
  and tees all output to a timestamped host file under `tests/.cache/`. It must
  preserve the Docker process status through that tee; the ephemeral
  container's internal log is not durable evidence.
- **User resolution.** setup.sh resolves one actual non-root account and account
  home before any install phase, requires `HOME` to identify the same directory,
  and passes both variables through `sudo -H env`. The flake ignores ambient
  `SUDO_USER`/`USER`; `"runner"` exists only so pure `nix flake check`
  evaluates. The sudo boundary uses `sudo -H` so Nix sees root's home while the
  target identity continues to flow only through the validated variables.
  First-run bootstrap derives the locked rev and `narHash` from
  `flake.lock` with Nix's JSON parser, URL-encodes the hash for the flake ref,
  and runs
  `sudo -H env DOTFILES_TARGET_USER=... DOTFILES_TARGET_HOME=... nix run github:nix-darwin/nix-darwin/<locked-rev>?narHash=<encoded-narHash>#darwin-rebuild -- ...`;
  do not use the mutable `nix-darwin` registry alias or omit the locked
  `narHash`.
- **Home Manager is packages-only** (`nix/home/darwin.nix`): `home.packages` +
  the minimal `home.username`/`home.stateVersion`. `home.homeDirectory` is left
  to the nix-darwin HM integration (derived from `users.users.<user>.home`) to
  avoid a conflicting definition. No `home.file`, no `xdg.configFile`, no
  `programs.<tool>` other than `programs.home-manager`. Guarded at the SOURCE
  level by `tests/static/nix_architecture_test.sh`, which allowlists only
  `programs.home-manager` and bans `home.activation` outright. The shared
  package list includes `nodejs_24` specifically so npm-backed Pi CLI
  provisioning does not depend on a stale distro Node.
- **setup.sh integration is enforced + consent-gated.** On macOS, `setup.sh`
  applies nix-darwin by default before Phase 1 dependency provisioning.
  `--nix-darwin` remains a compatibility alias, not the switch that makes Nix
  active. It is dry-run-safe (previews verified prerequisite installation when
  Nix is absent, then the real sudo command and locked bootstrap ref), prompts
  unless `--all`, skips cleanly off-macOS, and fails closed if prerequisite
  installation, verification, or activation fails.
  `--skip-deps` is the explicit already-provisioned escape and skips the Nix
  package layer together with native/deferred dependency installs; compatibility
  aliases do not override it. `--skip-native-deps` is narrower: Nix/config
  remain active while native/deferred Phase 1 is skipped, solely so the
  versioned release transaction has a complete rollback boundary. That
  transaction also passes `--skip-config-scripts`, which limits chezmoi to
  files/symlinks; normal setup still runs reviewed run scripts. Config apply
  creates managed target parents without relying on Phase 1. Public setup selects the Apple Silicon Darwin
  configuration; every other architecture fails before activation. Guarded by
  `tests/nix/setup_nix_darwin_test.sh`.
- **Linux/WSL Home Manager (standalone, packages-only).**
  `homeConfigurations."<arch>-linux"` (`nix/home/linux.nix` + the shared
  `nix/home/common.nix`) is the nix-owned CLI set for Linux/WSL, activated by
  `setup.sh` (Linux-only, same enforced/consent/dry-run contract as
  nix-darwin). `--home-manager` remains a compatibility alias.
  `programs.home-manager.enable` is the ONE allowed `programs.*`
  (it installs the standalone HM CLI). On WSL it writes ONLY to `~/.nix-profile`,
  never `/mnt/c` — split-host preserved. Managed zsh sources Home Manager's
  canonical session-vars file once from the XDG profile, `~/.nix-profile`, or
  `/etc/profiles/per-user/<effective-user>` in that order, so standalone and
  system-integrated profiles work while no-Nix hosts remain harmless. The
  standalone Linux configuration sets `home.sessionPath` to its evaluated
  `home.profileDirectory/bin`, so that canonical file—not caller PATH
  injection—exports the Nix-owned CLI path.
  Linux also owns `clangd` through `pkgs.clang-tools` on both x86_64 and arm64;
  Mason is not a Linux clangd owner because its registry has no arm64 artifact.
  First-run bootstrap likewise uses
  `github:nix-community/home-manager/<locked-rev>?narHash=<encoded-narHash>#home-manager`
  from `flake.lock`, not a mutable registry alias. **Native install-deps arms are
  RETAINED as deferred/artifact provisioning and regression fixtures**; the Nix
  package layer is the public POSIX package plane. Guarded by
  `tests/nix/linux_home_test.sh` + `tests/nix/setup_home_manager_test.sh`.
- **nvim + the tree-sitter CLI are DELIBERATELY NOT in the Nix package set
  (deferred, with proof).** nvim-treesitter `main` compiles parsers whose ABI
  must match nvim's built-in libtree-sitter, and the repo pins the tree-sitter
  CLI to `v0.26.10` precisely to keep that build reproducible (invariant 19). A
  nix neovim / tree-sitter shadowing the pinned native binaries would risk the
  `E5113` parser/ABI-mismatch class of bug. So `nix/home/common.nix` omits both;
  they stay on the native install-deps path. Moving nvim into the SAME Nix
  closure as its parser toolchain (nix nvim + nix tree-sitter CLI + nix compiler,
  ABI-matched) is the follow-up — do not add `neovim`/`tree-sitter` to the HM
  package set until that closure is proven.
- **Renovate** owns `flake.lock` bumps via the `nix` manager (reviewed PRs). Do
  NOT rewrite `flake.lock` silently in setup/update; a bump is always a PR.
- **`--update` owner=nix** is Phase 7 (`tests/shell/install_deps_update_test.sh`).

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
  every machine ends up on the same plugin commits. Setup and validation must
  restore from it; only intentional plugin maintenance may sync/update and
  change it.
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
- **Starship prompt segments are foreground-only.** `starship/starship.toml`
  must not contain `bg:` styles. Terminal transparency belongs to Ghostty /
  Windows Terminal; Starship-owned background blocks render as opaque character
  cells and make the prompt look patched onto the transparent surface. Guarded
  by `tests/starship/render_test.sh`. The right-aligned time module deliberately
  keeps one trailing safety space so its final Nerd Font glyph is not drawn into
  the terminal's last column. The username module is enabled and always shown at
  the prompt start; tmux/psmux must not duplicate username in the status bar.
- **tmux/psmux Rose Pine is ONE repo-owned generated bar, sourced on both
  platforms.** This REPLACES the retired "POSIX loads the upstream `rose-pine/tmux`
  plugin; Windows renders a port" policy. `tmux/psmux-rose-pine.ps1` renders an
  Omer/Catppuccin-shaped Rose Pine pill bar (rounded session pill left,
  number-on-right window cells with a zoom marker on the current window, directory
  pill right) into `tmux/psmux-rose-pine.{main,moon,dawn}.conf`. BOTH POSIX tmux
  (`tmux/tmux.posix.conf`) and native-Windows psmux (`tmux/tmux.windows.conf`)
  `source-file` the SAME deployed variant (`~/.tmux.rose-pine.{main,moon,dawn}.conf`,
  chezmoi-managed on both — no longer Windows-only), so the bar is byte-identical.
  Shared `tmux/tmux.conf` stays psmux-safe and owns only cross-platform placement
  (`status-position top`); it must remain free of load-time `if-shell`,
  psmux-specific commands, and quoted overlay source paths. The bar is a signal
  bar: session, window list, directory basename only. Starship owns username,
  time, full path, git, and language/runtime context; host stays off the daily
  surface. We do NOT use a theme plugin to render it: `rose-pine/tmux` is a
  bash/TPM script that shells out ~30x at load and would hang psmux/ConPTY, and
  the community `psmux-theme-rosepine` renders a different arrow-chevron powerline
  bar. The renderer is pure declarative `set -g` (no load-time `if-shell`, no
  per-redraw `#(...)`; dynamic fields use native formats `#S`/`#I`/`#W`/
  `#{?client_prefix,...}`/`#{?window_zoomed_flag,...}`/`#{b:pane_current_path}`),
  inlines every `#[fg=...,bg=...]` because psmux stores-but-ignores
  `window-status-*-style`, uses rounded pill caps (U+E0B6/U+E0B4) and NOT
  arrow-chevron powerline separators (U+E0B0/U+E0B2), and keeps one trailing
  safety space after the status-right directory so the final cell is not clipped
  by Windows Terminal/ConPTY. The normal session pill accent is Rose Pine `foam`
  (not `iris`, which reads too close to Catppuccin purple in this layout);
  holding the prefix changes it to `love`. The empty status canvas and pill
  outside-cap backgrounds MUST use `bg=default` so terminal transparency shows
  through; only pill interiors should carry explicit Rose Pine backgrounds.
  Default variant `main`, plus `moon`/`dawn`, selected
  by `@rosepine-variant` on BOTH platforms. `tmux/psmux-rose-pine.ps1` MUST stay
  pure ASCII (PS 5.1 parse safety, guarded by `invariants_test.sh`); the Nerd Font
  glyphs are built from codepoints at runtime and only the generated `.conf`
  artifacts carry rendered glyphs. Keep local `tmux/themes/*.conf` snippets
  deleted. The generated `.conf` files are committed artifacts; regenerate with
  the renderer's `-EmitConf -Variant <name>` mode once per variant and re-mirror
  into `home/`. Emitted options must stay inside the tmux/psmux option
  intersection verified against psmux v3.3.6. Do not emit tmux-only
  `display-panes-colour` / `display-panes-active-colour` in the shared artifacts:
  psmux stores unknown options but still warns on every config load. If POSIX
  pane-number colors become important later, apply them from the POSIX overlay
  with a tmux-only `set -gF` path rather than polluting the shared artifact.
  Live switch:
  `tmux set -g @rosepine-variant moon; tmux source-file ~/.tmux.posix.conf`
  (POSIX) / `psmux set -g @rosepine-variant moon; psmux source-file
  ~/.tmux.windows.conf` (Windows).
- **Functional tmux/psmux plugins (session save/restore), pinned + vendored.**
  POSIX `tmux/tmux.posix.conf` declares TPM + the Omer functional set
  (`tmux-sensible`, `tmux-yank`, `tmux-resurrect`, `tmux-continuum`) from the
  repo-managed plugin root `~/.local/share/dotfiles/tmux-plugins`, sources the
  generated bar BEFORE running TPM (so `tmux-continuum` prepends its invisible
  save trigger to the themed `status-right`, not the default one), and sets
  `@continuum-restore on` + `@resurrect-strategy-nvim session`. Windows starts
  psmux through `tmux/psmux.conf` (`~/.psmux.conf`), which disables warm sessions
  before sourcing `~/.tmux.conf` (psmux's pre-server warm check shallow-scans only
  the first config file and could otherwise claim a stale warm server), then
  flag-free `source-file`s `~/.tmux.windows.conf` (psmux v3.3.x does not implement
  tmux's `source-file -q`). `tmux/tmux.windows.conf` source-files ONLY the vendored
  `~/.psmux/plugins/psmux-resurrect/plugin.conf` (its plugin.conf adds two
  keybinds — `Prefix+Ctrl-s`/`Prefix+Ctrl-r` — at load, no auto hooks).
  `install-deps.ps1`'s `Install-PsmuxPlugins` vendors ONLY that port from the
  `psmux/psmux-plugins` monorepo at pinned commit
  `0f46ccca5a9b748fd03851db00b85fd784f42791` into `~/.psmux/plugins/` (the
  plugin.conf hardcodes that path). We deliberately do NOT use PPM: it clones the
  monorepo HEAD (unpinned) and its `Persist-PluginActivation` rewrites
  `~/.psmux.conf`/`~/.tmux.conf`, which would corrupt the chezmoi byte-parity
  model. At that pin there is no active top-level `psmux-yank` port (only a
  retired `_trash/psmux-yank`), so native-Windows yank stays the `clip.exe`
  copy-mode binding in `tmux/tmux.windows.conf`. `psmux-sensible` is intentionally
  NOT sourced: its `plugin.conf` is unconditional `set -g` (unlike the conditional
  `tmux-sensible`) and would clobber our tuned shared `tmux.conf` (e.g.
  `escape-time`).
  The Windows overlay must not reintroduce `@plugin`/PPM or a plugin-root `run`
  (source-file of a pinned plugin.conf is the allowed mechanism); keep
  `tmux/psmux.conf` Windows-only and mirrored to `home/dot_psmux.conf`.
  **`psmux-continuum` is BLOCKED on Windows, not shipped.** Its `plugin.conf`
  registers load-time `set-hook -g session-created`/`client-attached` that
  `run-shell` pwsh (auto-save loop + auto-restore). Although `run-shell` is async
  (not the synchronous `if-shell` freeze class), it was NEVER validated on a real
  Windows psmux host (authored on macOS), so it is deliberately not vendored, not
  source-filed, and `@continuum-restore`/`@continuum-save-interval` are absent from
  the Windows overlay (guarded by `windows_conf_test.sh` reject rules). Do NOT
  ship it until someone proves on real Windows psmux that its hooks do not freeze
  ConPTY, do not spawn runaway pwsh loops, and survive restart/reattach. Follow-up
  smoke-test to unblock: open psmux, confirm panes render without freezing,
  `psmux show-hooks -g`, and Prefix+Ctrl-s/Prefix+Ctrl-r for manual resurrect
  save/restore first. POSIX tmux keeps `tmux-continuum` (testable on Linux).
  psmux v3.3.x does not support tmux's `terminal-features` option. Keep
  `set ... terminal-features` out of `tmux/tmux.conf`, `home/dot_tmux.conf`,
  `tmux/tmux.windows.conf`, and `home/dot_tmux.windows.conf`; tmux extended-key
  feature flags belong only in the POSIX overlay.
  POSIX `@rosepine-variant` uses `set -go` (only-if-unset) so a live
  `tmux set -g @rosepine-variant moon` survives repeated re-sourcing of the
  overlay instead of snapping back to main (matches Windows; guarded by
  `option_test.sh`).
  Current pins: TPM `e261deb1b47614eed3400089ce7197dc68acc4eb`, `tmux-sensible`
  `25cb91f42d020f675bb0a2ce3fbd3a5d96119efa`, `tmux-yank`
  `acfd36e4fcba99f8310a7dfb432111c242fe7392`, `tmux-resurrect`
  `cff343cf9e81983d3da0c8562b01616f12e8d548`, `tmux-continuum`
  `0698e8f4b17d6454c71bf5212895ec055c578da0`, and `psmux/psmux-plugins`
  `0f46ccca5a9b748fd03851db00b85fd784f42791`. Guarded by
  `tests/tmux/option_test.sh`, `tests/tmux/windows_conf_test.sh`,
  `tests/shell/tmux_plugins_test.sh`, `tests/powershell/PsmuxRosePine.Tests.ps1`,
  `tests/powershell/InstallDeps.Tests.ps1`, `tests/migration/parity_gate.sh`, and
  `tests/static/pin_consistency_test.sh`.
- **tmux/psmux status transparency uses default backgrounds.** The generated bar
  must not paint the whole status canvas with Rose Pine base. Use `bg=default`
  for `status-style` and the outside rounded-cap backgrounds, and reserve
  explicit Rose Pine backgrounds for the pill interiors. This keeps the status
  bar visually attached to Ghostty / Windows Terminal transparency instead of
  rendering as a solid strip.
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
