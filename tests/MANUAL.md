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
- [ ] **Herdr session smoke**, macOS/Linux: `herdr --version` prints the pinned
      version; start a session (`herdr`), confirm it opens panes and its
      agent-state awareness works, then exit cleanly. On native Linux without
      brew, confirm the binary is the pinned SHA-256-verified release
      (`~/.local/bin/herdr`), not a remote-eval install. Herdr must NOT be
      present on native Windows.

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
      icons.

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

## Nix layer (macOS opt-in)

- [ ] **`nix flake check`**: on a Nix host run `nix flake check` at the repo root
      — it evaluates `darwinConfigurations.dotfiles` (build skipped) and builds
      `checks.<system>.toolchain`, exit 0.
- [ ] **nix-darwin bootstrap/switch**, macOS: with Nix installed, run
      `./setup.sh --nix-darwin` (or `nix run nix-darwin -- switch --flake .#dotfiles --impure`
      the first time). Confirm it prompts for sudo only at activation, sets
      `system.primaryUser` to the real `$USER` (not root), installs the WezTerm +
      AeroSpace casks and the Herdr brew via declarative Homebrew (no `brew
      update`/`upgrade`; `cleanup = check` only reports drift), and puts the
      nix-owned CLI set on PATH from `~/.nix-profile` / the system profile.
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
- [ ] **WSL experimental GUI opt-in only**: if testing Linux Ghostty under WSLg /
      X11, run `./setup.sh --experimental-wsl-gui`; otherwise confirm
      `~/.config/ghostty/config` is not linked by default in WSL.
