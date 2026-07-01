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
- [ ] **Tmux status bar**: Rose Pine plugin bar is at the top, includes the
      official rich segments (date/time, user/host, directory/current program),
      segments are readable, prefix is `C-b`,
      `prefix r` reloads the conf and shows the "reloaded" message.
- [ ] **psmux status bar**: psmux-theme-rosepine bar is at the top after
      reopening psmux; powerline/icons/date/time render, the appended directory
      segment appears on the right, no config-load freeze, and no sustained CPU
      spike.
- [ ] **Starship prompt**: shows username, dir, git branch, git status icons
      (untracked/modified/staged), trailing time, and no opaque background
      blocks behind prompt text.
- [ ] **PowerShell Tab completion**: press Tab into MenuComplete; the selected
      item is light text on the Rose Pine overlay background.

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
