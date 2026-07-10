# Manual test checklist

The automated suite covers the deterministic surface. Some things only
make sense to verify by eye — keep this checklist alongside any
significant change to the relevant area.

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
- [ ] **Windows Terminal dual-install preservation**: give packaged and portable
      settings different custom profiles, schemes, actions, and defaults; run
      `setup.ps1 -SkipDeps -SkipNvim`. Confirm each retains only its own custom
      state plus the managed fragment and receives its own verified backup.
      Run `uninstall.ps1 -All`; confirm both pre-setup backups restore and each
      displaced current file remains as `settings.json.uninstall-current.*`.
- [ ] **Redirected Windows known folders**: redirect Documents and
      LocalApplicationData to different real paths (include an alternate drive
      and spaces), run `setup.ps1 -All`, then open Neovim, lazygit, ConsoleHost,
      VS Code, and ISE. Confirm each consumes the managed target in the actual
      known folder and no conventional `%USERPROFILE%\AppData\Local`/`Documents`
      target was silently overwritten. Run `uninstall.ps1 -All` and verify the
      same source states are removed or restored without guessing paths.
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
- [ ] **AeroSpace reserved-chord safety**, macOS: with AeroSpace running, open a
      terminal + nvim. `ctrl-alt-h/j/k/l` moves WM focus between windows;
      **bare `Alt-h/j/k/l` still reaches nvim** (window nav) and is NOT captured
      by AeroSpace; `Alt-c` still triggers fzf-tab / PSFzf `cd`. `alt-1..9`
      switches workspaces; `alt-shift-1..9` moves the window; `ctrl-alt-f`
      fullscreen; `ctrl-alt-shift-;` enters service mode (esc reloads config).
- [ ] **Herdr session smoke**, all OSes: `herdr --version` prints the installed
      version; start a session (`herdr`), confirm it opens panes and its
      agent-state awareness works, then exit cleanly. On native Linux without
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
- [ ] **nix-darwin bootstrap/switch**, macOS: on Apple Silicon and Intel with Nix installed, run
      `./setup.sh --all` (or the compatibility alias `./setup.sh --nix-darwin`;
      equivalent activation:
      `sudo env DOTFILES_TARGET_USER="$USER" DOTFILES_TARGET_HOME="$HOME"
      darwin-rebuild switch --flake .#dotfiles-aarch64 --impure` or
      `.#dotfiles-x86_64` as appropriate; first-run setup
      derives the locked
      `github:nix-darwin/nix-darwin/<rev>?narHash=<encoded-narHash>#darwin-rebuild`
      ref from `flake.lock`). Confirm activation uses sudo but targets the
      setup-validated real invoking user/home via `DOTFILES_TARGET_*` (not
      `root` or a fabricated home), installs the WezTerm + AeroSpace casks and the Herdr brew via
      declarative Homebrew (no `brew update`/`upgrade`; `cleanup = check` only
      reports drift), and puts the nix-owned CLI set on PATH from
      `~/.nix-profile` / the system profile.
      If Homebrew already existed, confirm nix-homebrew auto-migrated the
      Homebrew repositories while keeping installed packages. If the old
      architecture-correct `Library/Taps` directory existed, confirm setup moved
      it to a `Taps.dotfiles-pre-nix-*` backup and nix-homebrew replaced it with
      the declarative pinned tap symlink. Inject/fix an activation failure and
      confirm the original taps return before retrying. Confirm
      `brew tap-info nikitabobko/tap`
      reports a trusted tap so Homebrew 5 can load the AeroSpace cask.
      The `DOTFILES_NIX_DARWIN_HOSTED_CI=1` cleanup override is only for
      GitHub's disposable macOS runner; do not use it for this real-host check.
- [ ] **Intel macOS runtime proof is pending**: the branch adds official
      `macos-26-intel` Nix/setup lanes, but do not mark this row complete until
      the exact PR head has a green real runner result. Cross-evaluation is not
      runtime proof. Nixpkgs 26.05 is the final Intel-darwin release and remains
      supported only through 2026-12-31; keep its warning visible and track the
      required post-26.05 package-plane migration separately from this current
      host proof.
- [ ] **Home Manager (Linux/WSL)**: with Nix installed inside the Linux/WSL
      environment, run
      `./setup.sh --all` (or the compatibility alias `./setup.sh --home-manager`;
      equivalent installed command:
      `home-manager switch --flake .#$(uname -m)-linux --impure`; first-run
      setup derives the locked
      `github:nix-community/home-manager/<rev>?narHash=<encoded-narHash>#home-manager`
      ref from `flake.lock`). Confirm the nix CLI set (ripgrep/fd/fzf/jq/lazygit/node/
      npm/starship/zoxide) lands in `~/.nix-profile/bin` with NO root, and that `nvim` +
      `tree-sitter` are still the native install-deps binaries (NOT nix) so
      parser builds keep working.
- [ ] **Fresh Home Manager zsh session**, native Linux and WSL: with no caller
      PATH injection, run `env -i HOME="$HOME" USER="$USER" PATH=/usr/bin:/bin
      TERM=xterm zsh -l -i -c 'command -v rg'`. Confirm it resolves through a
      Nix profile/store path. Repeat with a custom home containing spaces and
      with neither session-vars file present (the latter must remain harmless).
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
      `.\setup.ps1 -All`, then inside WSL install Nix and run
      `./setup.sh --all` and `./tests/wsl/e2e.sh`. Confirm the script passes:
      Windows Terminal uses Hack Nerd Font, `win32yank` is reachable, lazygit is
      installed on both sides, zsh plugins are installed in WSL, nvim starts,
      tmux starts, and clipboard from WSL -> Windows round-trips.
- [ ] **WSL experimental GUI opt-in only**: if testing Linux Ghostty under WSLg /
      X11, run `./setup.sh --experimental-wsl-gui`; otherwise confirm
      `~/.config/ghostty/config` is not linked by default in WSL.
