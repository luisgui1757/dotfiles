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
