# Manual test checklist

The automated suite covers the deterministic surface. Some things only
make sense to verify by eye — keep this checklist alongside any
significant change to the relevant area.

## v0.1.0 to v0.2.0 release upgrade

Use throwaway users/VMs seeded from the exact annotated v0.1.0 release. Follow
`docs/UPGRADING.md`; never use `main` or update the old checkout in place.
For every row, the normal success path must be only `setup --all` / `setup -All`
from the exact new release checkout. Separately inject an interruption after
package activation and after config publication through the operator migrator,
run the printed rollback, then prove rerunning setup resumes or retries safely.
Also run setup update/upgrade once. Record the old/new tag objects, peeled
commits, recovery path, provider inventory, and whether any user data changed.

- [ ] **Apple Silicon owner-host:** begin with v0.1.0 Homebrew formulae/casks,
      real taps, no Nix, divergent config, and backup-name collisions. Install
      Nix through setup's checksum-verified helper; prove failed
      nix-darwin/later config paths restore old
      taps, `/etc` shell files, v0.1.0 config, and provider precedence. On
      success verify setup accepted the core, retained recovery, repointed
      config, and completed apps/TCC provisioning.
- [ ] **Native Linux x86_64:** seed a representative native-package/Linuxbrew/
      direct-artifact v0.1.0 host. Prove Home Manager activation, login PATH,
      later failure uninstall, exact old symlink restoration, retry, and
      acceptance without removing native packages.
- [ ] **Native Linux aarch64:** repeat on a real aarch64 host; configuration
      evaluation or an emulated filesystem is not runtime proof.
- [ ] **WSL2 split host:** run only setup on Windows and setup in the guest.
      Interrupt each platform transaction independently, prove its automatic
      rollback/retry, then run `tests/wsl/e2e.sh` after both setup invocations
      succeed.
- [ ] **Windows conventional known folders:** exact v0.1.0 checkout with
      divergent copy-mode files and nvim link. Apply from exact v0.2.0, fail
      after Terminal/config publication, and prove exact old config plus
      stable packaged/Preview/Canary/portable Terminal bytes return before retry. After recovery is
      captured, alter or temporarily move both retained checkouts and prove
      apply/rollback still consume only the frozen release trees.
- [ ] **Windows redirected/OneDrive/alternate drive:** repeat with Documents,
      LocalApplicationData, ApplicationData, and runtime `$PROFILE` on
      independent real paths.
      Include divergent packaged, Preview, Canary, and portable Terminal installations;
      no conventional path may be guessed or overwritten.
- [ ] **Release acceptance:** on the final annotated v0.2.0 tag, run the full
      local/hosted gates and public-secret scan, record the tag object and peeled
      commit, prove fresh and v0.1.0 machines both need only setup all, then
      confirm the release document contains no branch command or placeholder
      identity. Until every required row is accepted, keep v0.1.0 as the only
      user-facing release.

## Visual / GUI

- [ ] **Ghostty**, mac: opens with Rose Pine dark, translucent background
      reads cleanly over a coloured wallpaper, font is Hack Nerd at 13pt.
- [ ] **Ghostty**, any OS in light mode (e.g. fresh GNOME Ubuntu): STAYS Rose
      Pine dark -- it must NOT flip to the cream Rose Pine Dawn (theme is forced
      dark, not the adaptive dark:/light: split).
- [ ] **WezTerm** visual parity, any OS (macOS cask / Windows catalog / amd64
      Ubuntu .deb): opens **maximized** with Rose Pine dark, mild translucency
      reads cleanly over a coloured wallpaper (opacity 0.95; macOS blur), font is
      Hack Nerd at 13pt with ligatures, block cursor (no blink), and glyphs (e.g.
      a starship prompt) render via the Hack Nerd / Symbols Nerd fallback. On
      Windows a new window is `pwsh.exe`; on POSIX it is the login shell (zsh).
      It must NOT auto-launch a multiplexer -- you get a bare shell.
- [ ] **WezTerm** in light mode (e.g. fresh GNOME Ubuntu / macOS light): STAYS
      Rose Pine dark, same forced-dark rule as Ghostty.
- [ ] **psmux inside WezTerm** (Windows): open WezTerm, run `psmux`, confirm the
      pane renders without a config-load freeze, no runaway `psmux.exe`/`conhost`
      CPU, the generated Rose Pine bar draws, and `pwsh` prediction/MenuComplete
      still work inside the pane (same smoke as psmux inside Windows Terminal).
- [ ] **Windows Terminal**: rose-pine scheme applied; tabs use the
      configured theme; acrylic OFF on the body, ON in the tab row; a new tab
      opens `PowerShell 7` unless the user intentionally chose another default.
- [ ] **Windows Terminal four-variant preservation**: give stable packaged,
      Preview, Canary, and portable settings different custom profiles,
      schemes, actions, and defaults; run
      `setup.ps1 -SkipDeps -SkipNvim`. Confirm each retains only its own custom
      state plus the managed fragment and receives its own verified backup.
      Run `uninstall.ps1 -All`; confirm all pre-setup backups restore and each
      displaced current file remains as `settings.json.uninstall-current.*`.
- [ ] **Redirected Windows known folders**: redirect Documents,
      LocalApplicationData, and roaming ApplicationData to different real paths
      (include an alternate drive and spaces), run `setup.ps1 -All`, then open
      Neovim, lazygit, Herdr, ConsoleHost, VS Code, and ISE. Confirm each consumes
      the managed target in the actual known folder and no conventional
      `%USERPROFILE%\AppData`/`Documents` target was silently overwritten. Run
      `uninstall.ps1 -All` and verify the same source states are removed or
      restored without guessing paths.
- [ ] **Tmux status bar**: generated Rose Pine bar is at the top, includes the
      signal-bar segments (session, window list/current program, directory
      basename; no user/host/date/time duplication), segments are readable,
      empty bar space follows terminal transparency,
      prefix is `C-b`,
      `prefix r` reloads the conf and shows the "reloaded" message.
- [ ] **psmux status bar**: on a fresh `psmux` launch, with no manual command,
      the generated Rose Pine config (`~/.tmux.rose-pine.main.conf` by default)
      draws the same rounded pill bar as POSIX tmux at the top: session/window
      list on the left, directory basename on the right, foam session accent,
      gold active-window number, muted inactive-window number, icons render, no
      config warnings, empty bar space follows terminal transparency, no
      config-load freeze, no sustained CPU spike, and no clipped final cell at
      the right edge. Switch flavor:
      `psmux set -g @rosepine-variant moon; psmux source-file ~/.tmux.windows.conf`.
- [ ] **Starship prompt**: shows dir, git branch, git status icons
      (untracked/modified/staged), trailing time, and no opaque background
      blocks behind prompt text. The final Rose glyph on the right-aligned time
      must not be clipped by the terminal edge.
- [ ] **PowerShell Tab completion**: press Tab into MenuComplete; the selected
      item is light text on the Rose Pine overlay background.

## Window manager / multiplexer (macOS + Linux)

- [ ] **AeroSpace Accessibility (TCC) grant**, macOS: after
      `brew install --cask nikitabobko/tap/aerospace` and launch, macOS prompts
      for Accessibility — grant it (System Settings -> Privacy & Security ->
      Accessibility -> AeroSpace ON). Confirm AeroSpace starts at login
      (`start-at-login = true`) and tiles windows.
- [ ] **AeroSpace managed-config consumption**, macOS with the TCC grant above:
      run `aerospace config --config-path` and confirm its resolved path is the
      chezmoi-managed `~/.config/aerospace/aerospace.toml` target and its bytes
      match `aerospace/aerospace.toml`. Then run
      `aerospace reload-config --no-gui --dry-run --warnings-as-errors` and
      require exit 0 with no diagnostics. This remains manual because the app
      waits for Accessibility before parsing user config or starting its CLI
      server; a GitHub-hosted macOS runner has no user-granted TCC session.
- [ ] **AeroSpace reserved-chord safety**, macOS: with AeroSpace running, open a
      terminal + nvim. `ctrl-alt-h/j/k/l` moves WM focus between windows;
      **bare `Alt-h/j/k/l` still reaches nvim** (window nav) and is NOT captured
      by AeroSpace; `Alt-c` still triggers fzf-tab / PSFzf `cd`. `alt-1..9`
      switches workspaces; `alt-shift-1..9` moves the window; `ctrl-alt-f`
      fullscreen; `ctrl-alt-shift-;` enters service mode (esc reloads config).
- [ ] **Herdr session smoke**, all OSes: `herdr --version` prints the installed
      version; start a session (`herdr`), confirm it opens panes and its
      agent-state awareness works, confirm the UI uses dark Rose Pine, then exit
      cleanly. Confirm the managed config is `~/.config/herdr/config.toml` on
      POSIX and the real `%APPDATA%\herdr\config.toml` on Windows. On Windows,
      create a fresh pane and confirm `(Get-Process -Id $PID).Name` is `pwsh`,
      `$PROFILE` resolves below the managed PowerShell Documents path, and prior
      PowerShell 7 commands appear in the PSReadLine ListView prediction menu.
      Existing panes retain their old shell and are not valid evidence after a
      config change. On native Linux without
      brew, confirm the binary is the pinned SHA-256-verified release
      (`~/.local/bin/herdr`), not a remote-eval install. On native Windows,
      confirm `herdr.exe` resolves from `%LOCALAPPDATA%\Programs\Herdr\bin`, not
      `herdr.dev/install.ps1`; the Windows build is preview beta / ConPTY-backed,
      so verify it does not freeze in Windows Terminal, WezTerm, or psmux before
      treating it as a daily driver.

## Shell tooling

- [ ] **zoxide**: after visiting a few directories, `z <partial>` jumps to the
      best-matching one and `zi` opens the interactive picker — in BOTH a fresh
      zsh and a fresh PowerShell (incl. inside psmux). Plain `cd` is unchanged.
- [ ] **which-key**: in nvim press `<leader>` and pause past `timeoutlen`; a
      popup lists the follow-up keys. `<leader>?` shows the buffer-local keymaps.
- [ ] **gh-dash**: the config (`~/.config/gh-dash/config.yml`;
      `%USERPROFILE%\.config\gh-dash\config.yml` on Windows) is applied by setup
      regardless of auth; the extension binary installs only **after**
      `gh auth login` (setup skips it cleanly when unauthenticated — rerun
      setup/install-deps after authenticating). Then `gh dash` renders the
      dashboard (My Pull Requests / Needs My Review / My Issues) with Nerd Font
      icons, and `gh extension list` identifies commit
      `49f37e4832956c57bf52d4ea8b1b1e5c0f863700`.
- [ ] **Pi CLI**: `pi --version` prints `0.80.3` on macOS, Linux/WSL, and
      Windows. Confirm `.pi/` session/auth state stays local and is not created
      or modified by chezmoi.
- [ ] **Windows Tree-sitter CLI**: `tree-sitter --version` prints exactly
      `0.26.10`. A compatible unmanaged executable remains untouched; after a
      stale unmanaged fixture, the verified dotfiles executable wins PATH.
- [ ] **Zsh plugin pin recovery**: bare `chezmoi apply` self-heals a clean old
      pin. With a dirty/wrong fixture and network disabled, it fails with the
      fixed source path absent and prints a preserved quarantine path.

## Command-line vi mode

- [ ] **zsh vi mode**: in a fresh zsh, type a command, press `Esc` — the cursor
      turns to a block and `h`/`j`/`k`/`l`, `w`/`b`, `dd`, `cw`, `.` edit the
      line; `i`/`a` returns to insert (beam cursor). While typing (insert): `Tab`
      opens the fuzzy completion menu, `Up`/`Down` prefix-search history, and
      `Ctrl-R`/`Ctrl-T`/`Alt-C` are the fzf pickers. In normal mode the arrows
      still search history. `Esc` feels responsive but `Alt-C` and the arrow
      keys are NOT split (tune with `DOTFILES_KEYTIMEOUT` if needed).
- [ ] **PowerShell vi mode**: in a fresh pwsh, `Get-PSReadLineOption` shows
      `EditMode = Vi`. `Esc` enters command mode (cursor becomes a block on
      Windows Terminal); `Tab` still opens MenuComplete while typing, `Up`/`Down`
      history-search, and PSFzf `Ctrl+R`/`Ctrl+T`/`Alt+C` still work.
- [ ] **PowerShell vi mode inside psmux**: open a fresh psmux pane, wait for the
      prompt to settle (the `OnIdle` re-apply runs ~300 ms in), then confirm
      `Get-PSReadLineOption` still shows `EditMode = Vi`, `Tab` = MenuComplete,
      the ListView history prediction is back, and the PSFzf `Ctrl+R` picker
      still works (the re-apply must NOT have wiped the fzf chords).
- [ ] **PowerShell invocation guard on Windows**: run the profile through
      `pwsh -NonInteractive -Command`, a credential-helper-shaped `-Command`,
      redirected stdin/stdout, and a CI subprocess. Confirm zero prompt output
      and no Starship/zoxide cache writes. Then confirm normal ConsoleHost, VS
      Code, and ISE sessions still load the prompt and bindings.

## Nix layer (enforced on macOS/Linux/WSL)

- [ ] **`nix flake check`**: on a Nix host run `nix flake check` at the repo root
      — it evaluates `darwinConfigurations.dotfiles` (build skipped) and builds
      `checks.<system>.toolchain`, exit 0.
- [ ] **nix-darwin bootstrap/switch**, Apple Silicon macOS: with Nix installed, run
      `./setup.sh --all` (or the compatibility alias `./setup.sh --nix-darwin`;
      equivalent activation:
      `sudo -H env DOTFILES_TARGET_USER="$USER" DOTFILES_TARGET_HOME="$HOME"
      darwin-rebuild switch --flake .#dotfiles-aarch64 --impure`; first-run setup
      derives the locked
      `github:nix-darwin/nix-darwin/<rev>?narHash=<encoded-narHash>#darwin-rebuild`
      ref from `flake.lock`). Confirm activation uses sudo but targets the
      setup-validated real invoking user/home via `DOTFILES_TARGET_*` (not
      `root` or a fabricated home), installs the WezTerm + AeroSpace casks and the Herdr brew via
      declarative Homebrew (no `brew update`/`upgrade`; `cleanup = none`
      preserves mixed-ownership packages), and puts the nix-owned CLI set on PATH from
      `~/.nix-profile` / the system profile.
      On a first bootstrap with existing `/etc/bashrc` or `/etc/zshrc`, confirm
      setup retains their exact bytes at `.before-nix-darwin`; an injected
      activation failure/interruption must restore both originals and preserve
      generated replacements at collision-safe `.dotfiles-failed-*` paths. A
      pre-existing `.before-nix-darwin` collision must move neither file and
      must print explicit compare/resolve/retry guidance.
      Without opening a new terminal after first activation, rerun
      `./setup.sh --all`. Confirm setup uses the installed
      `/run/current-system/sw/bin/darwin-rebuild`, does not print the bootstrap
      message, and leaves the `/etc/static/{bashrc,zshrc}` links and both
      `.before-nix-darwin` recovery files unchanged.
      If Homebrew already existed, confirm nix-homebrew auto-migrated the
      Homebrew repositories while keeping installed packages. If the old
      `Library/Taps` contains an unrelated user tap, confirm setup leaves that
      tap installed across two runs. If the retired root-owned pinned-tap shape
      exists, confirm setup migrates only those three snapshots and the resulting
      `nikitabobko/tap` checkout is owned by the target user. Confirm
      `brew tap-info nikitabobko/tap`
      reports a trusted tap so Homebrew 5 can load the AeroSpace cask.
      Run `./tests/macos_owner_lifecycle.sh` from a clean committed checkout for
      the canonical owner-host cycle: install, update, config uninstall,
      reinstall, final update, and full greenfield validation. The runner must
      report no setup-created transaction/recovery directory below
      `Library/Taps`, an idempotent second uninstall, and no removed pre-existing
      Homebrew formula, cask, or unrelated tap. The sudo prompt belongs to the
      invoking terminal; do not pipe credentials into the runner or its log.
      Tap filesystem assertions must resolve `Library/Taps` below
      `brew --prefix`; nix-homebrew's `brew --repository` points at the managed
      implementation and is not the tap-install root.
      Run once from a shell where the Nix daemon profile guard is already set
      but `/nix/var/nix/profiles/default/bin` is absent from `PATH`; setup must
      re-adopt the installed Nix binary and must not invoke the prerequisite
      installer.
- [x] **Historical Intel macOS hosted runtime proof (platform retired)**: exact head
      `f4b63953f2f982702a685358b09e89bae2d78fdd` passed the real
      `macos-26-intel` Nix job (`29092384007` / `86360593091`) and full setup job
      (`29092384014` / `86360593153`). The x86_64 host installed upstream Nix
      2.34.8, selected only `dotfiles-x86_64`, completed nix-darwin and all six
      setup phases, and passed post-install plus the 257-check language smoke.
      This was runtime proof, not cross-evaluation. The PR lane restored caches
      and had no user-granted TCC desktop session. Intel support is now retired
      by explicit owner direction, so this append-only historical result is not
      a current support claim or an open package-plane migration.
- [x] **Cache-free exact behavior-head full setup proof**: workflow-dispatch run
      [`29096335827`](https://github.com/luisgui1757/dotfiles/actions/runs/29096335827)
      on merged-main SHA `5e3e7c6d93c400d67f6160c6f8f09be56aac10d3`
      skipped the broad install/plugin caches in every setup job. Attempt 1
      passed the Ubuntu container, public Ubuntu setup, and native Windows
      setup. Apple Silicon exposed an asynchronous Lazy Tree-sitter update
      overlapping Phase 4 (98/99 languages; Pascal had no captures), and Intel
      hit a separate transient GitHub DNS failure after restoring its original
      shell files and taps. Attempt 2 on the same unrepaired SHA passed Apple
      Silicon but failed Intel because the original CMake fixture's neocmake
      client did not attach within 45 seconds, even though the later formatter
      CMake fixture did attach. Rerun this exact cache-free workflow after the
      waitable-update repair merges. Branch-head cache-free run
      [`29100106370`](https://github.com/luisgui1757/dotfiles/actions/runs/29100106370)
      proved the first repair incomplete: Apple Silicon passed, while Ubuntu
      lost Astro captures and Intel lost GraphQL captures because ordinary
      headless plugin config still started an interactive asynchronous install.
      That path is now blocked outside a UI or the explicit synchronous phase.
      Exact behavior head `e5cf3e23299cbb42a157c307f2a7259979fcada0`
      then passed
      [`29103732329`](https://github.com/luisgui1757/dotfiles/actions/runs/29103732329)
      with caches skipped: Ubuntu container `86399025475`, public Ubuntu
      `86399025519`, Apple Silicon `86399025503`, Intel `86399025491`, native
      Windows `86399025722`, and all four setup logical proofs were green.
      This checks the exact branch behavior; it is not WSL, redirected Windows,
      divergent stable packaged/Preview/Canary/portable Terminal, or desktop/TCC
      evidence. Merged-main run
      [`29114125798`](https://github.com/luisgui1757/dotfiles/actions/runs/29114125798)
      on PR #48 merge SHA `f104bf066e4af7d4d707fe22ba36600711f1ae14`
      passed Ubuntu container, public Ubuntu, historical Intel, and Windows but
      failed Apple Silicon because the initial CMake LSP fixture shared a large
      project root and neocmakelsp timed out before attach; the later isolated
      CMake formatter fixture attached and validated gersemi in the same process.
      PR-head run
      [`29180481941`](https://github.com/luisgui1757/dotfiles/actions/runs/29180481941)
      later proved that two separate client lifecycles remained timing-dependent
      even after project isolation: the initial CMake client attached, but the
      formatter-only restart did not. Strict smoke now formats and checks the
      realistic CMake sample on the already-attached isolated client. Three
      repeated strict Apple-Silicon runs passed 257/257 checks. Exact repaired
      head `d744948cdccc51f3d79e45aa78f82c46445df0c6` then passed hosted E2E
      [`29181215803`](https://github.com/luisgui1757/dotfiles/actions/runs/29181215803),
      including all four producers and all four logical proof jobs. This PR run
      restored ordinary caches and is not the pending merged-main cache-free row.
- [ ] **Cache-free merged-main safeguard confirmation**: run
      `e2e-install.yml` again after this PR merges. Exact behavior head
      `f097995b49a2189db327903a20743e7cb69ba665` passed cache-free run
      [`29120109175`](https://github.com/luisgui1757/dotfiles/actions/runs/29120109175):
      all four current producers and all four setup logical proofs were green.
      On merged `main`, require those same producers (Ubuntu container, public
      Ubuntu, Apple Silicon, Windows), the four setup logical proofs, and the
      two Nix logical proofs to pass before applying the checked-in stable
      required contexts live. Record that merged-main run here and in
      `tests/greenfield/LEDGER.md`; a documentation-only descendant does not
      replace the behavior-head run. Then run the no-write safeguard preflight;
      it must identify that exact cache-free E2E dispatch, the Nix/test run
      provenance, GitHub Actions app `15368`, and the exact legacy live posture.
      Retain the apply command's `.git/dotfiles-safeguards/recovery.*` snapshot
      after mutation begins until the stable-context/SHA-policy readback has
      been independently reviewed. A pre-mutation failure must leave neither
      that recovery directory nor any temporary capture behind.
- [ ] **Home Manager (Linux/WSL)**: with Nix installed inside the Linux/WSL
      environment, run
      `./setup.sh --all` (or the compatibility alias `./setup.sh --home-manager`;
      equivalent installed command:
      `home-manager switch --flake .#$(uname -m)-linux --impure`; first-run
      setup derives the locked
      `github:nix-community/home-manager/<rev>?narHash=<encoded-narHash>#home-manager`
      ref from `flake.lock`). Confirm the nix CLI set (ripgrep/fd/fzf/jq/lazygit/node/
      npm/starship/zoxide/clangd) lands in `~/.nix-profile/bin` with NO root.
      `clangd` must resolve from Home Manager's `clang-tools` package on both
      Linux architectures and must not be expected from Mason, whose registry
      lacks a Linux arm64 artifact. Confirm `nvim` + `tree-sitter` are still the
      native install-deps binaries (NOT nix) so parser builds keep working.
      From a clean committed checkout, `./tests/linux_owner_lifecycle.sh` runs
      install, update, config uninstall, an idempotent uninstall retry,
      reinstall, final update, and full validation while proving no
      pre-existing native package disappeared. On macOS, the digest-pinned
      `./tests/greenfield/docker-linux-owner-lifecycle.sh` wrapper provides the
      same non-root Linux runtime surface without claiming WSL or physical-host
      release proof. That wrapper passed on 2026-07-13 at exact commit
      `51c5211b4b3dee4f0758533beac5e18345d668a1`, including 36/36 final
      validation checks and pre-existing-package preservation. This row remains
      open for a physical Linux host and WSL.
- [ ] **Fresh Home Manager zsh session**, native Linux and WSL: with no caller
      PATH injection, run `env -i HOME="$HOME" USER="$USER" PATH=/usr/bin:/bin
      TERM=xterm zsh -l -i -c 'command -v rg'`. Confirm it resolves through a
      Nix profile/store path. Repeat with a custom home containing spaces and
      with the XDG, `~/.nix-profile`, and
      `/etc/profiles/per-user/$(id -un)` session-vars locations individually.
      With none present, startup must remain harmless.
      Exact head `f4b63953f2f982702a685358b09e89bae2d78fdd` passed the
      hosted native-Linux account-record login-shell proof in run `29092384014`,
      job `86360593139`; this row stays open for WSL and the real custom-HOME
      permutations.
- [x] **Hosted WSL2 canary disposition**: scheduled run `29072773410` and
      manual rerun `29114215045` both reached WSL2 but stalled before setup
      evidence and were cancelled. GitHub does not officially support the
      required hosted nested virtualization, so the optional workflow is
      retired. This closes the unreliable pipeline, not WSL runtime proof.
- [ ] **WSL split-host under Home Manager**: on WSL, after `./setup.sh --all`,
      confirm nothing was written under `/mnt/c` — Home Manager touches only the
      Linux `~/.nix-profile`; Windows Terminal/fonts/WezTerm stay Windows-host.
- [ ] **Nix owns packages, chezmoi owns config**: after the switch, `~/.config`
      dotfiles (nvim, wezterm, aerospace, starship, zsh, tmux…) are still the
      chezmoi symlinks/copies — the Nix switch must NOT have replaced or
      duplicated any managed dotfile.
- [ ] **flake.lock is not silently mutated**: a normal `./setup.sh` /
      `./setup.sh --update` run leaves `git status` on `flake.lock` clean.

## Cross-OS clipboard round-trip

- [ ] **macOS**: yank in nvim, ⌘V into Notes — pastes.
- [ ] **WSL Ubuntu**: yank in nvim (inside tmux inside Windows Terminal),
      paste into a Windows app — pastes (requires `win32yank.exe`).
- [ ] **Linux X11**: same, with `xclip` installed.
- [ ] **Linux Wayland**: same, with `wl-clipboard`.

## LSP UX

- [ ] **C++ workspace** with `compile_commands.json`: hover, go-to-def,
      and clang-tidy diagnostics work in a real CMake project.
- [ ] **Two C++ workspaces in one Neovim session**: open projects whose compile
      databases require different macros (one at the root, one under `build/`).
      `:LspInfo` must show distinct clangd roots/clients and neither project may
      inherit the other's flags. The automated spec uses the real clangd binary;
      this row confirms the interactive workflow.
- [ ] **Rust workspace**: `cargo check` runs on save (rust-analyzer
      `checkOnSave`); inlay hints don't break visual layout.
- [ ] **Python with venv**: pyright picks up the venv interpreter
      (use `:LspInfo` to confirm).
- [ ] **CMakeLists.txt**: neocmake attaches; format-on-save runs gersemi
      without "Failed to format" toast.
- [ ] **PowerShell .ps1**: powershell_es attaches; signature help works.

## Setup on a fresh machine

- [ ] **New Mac**: `git clone`, `./setup.sh`, `exec zsh`, `nvim` opens
      and `:checkhealth` is clean except for optional system tools.
- [ ] **Fresh Windows**: clone, `.\setup.ps1`,
      new pwsh tab shows the rose-pine starship prompt; new WT tab has
      the rose-pine scheme and opens PowerShell 7 when WT already has
      `settings.json`; `nvim` works.
- [ ] **Windows treesitter compile**: after `.\setup.ps1 -All`, open nvim and
      run `:checkhealth nvim-treesitter`; it reports the tree-sitter CLI and
      compiler OK. Open a Python file and confirm `python` parser highlighting
      works.
- [ ] **Fresh WSL Ubuntu**: on Windows first run
      `.\setup.ps1 -All`, then inside WSL run
      `./setup.sh --all` and `./tests/wsl/e2e.sh`. Confirm the script passes:
      Windows Terminal uses Hack Nerd Font, `win32yank` is reachable, lazygit is
      installed on both sides, zsh plugins are installed in WSL, nvim starts,
      tmux starts, and clipboard from WSL -> Windows round-trips.
- [ ] **Ghostty exact Debian package consumption**: on a supported Ubuntu
      24.04/25.10 or Debian trixie amd64/arm64 host without Ghostty, run
      `./install-deps.sh --all`. Confirm setup prints the pinned
      `mkasberg/ghostty-ubuntu@1.3.1-0-ppa2` asset identity, `dpkg-query -W
      -f='${Version}' ghostty` returns `1.3.1-0~ppa2`, and `ghostty --version`
      succeeds. This remains manual for distro/architecture pairs not exercised
      by the hosted Ubuntu container.
- [ ] **WSL experimental GUI opt-in only**: if testing Linux Ghostty under WSLg /
      X11, run `./setup.sh --experimental-wsl-gui`; otherwise confirm
      `~/.config/ghostty/config` is not linked by default in WSL.
