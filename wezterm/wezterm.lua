-- WezTerm configuration -- Rose Pine (dark), multiplexer-friendly, Hack Nerd Font.
-- Install path (chezmoi-managed, the SAME on every OS):
--   POSIX   -> ~/.config/wezterm/wezterm.lua
--   Windows -> %USERPROFILE%\.config\wezterm\wezterm.lua
--
-- Parity intent matches ghostty/config and the Windows Terminal fragment:
-- ALWAYS dark Rose Pine (never adaptive), mild translucency, Hack Nerd Font with
-- a Nerd Symbols fallback, block cursor, generous scrollback, open maximized.
-- The whole stack (nvim, the multiplexer, starship, Windows Terminal, ghostty) is
-- dark Rose Pine; a light desktop must not flip this terminal to Rose Pine Dawn.
--
-- Shell: pwsh.exe on Windows; on POSIX we launch the login shell (which
-- install-deps.sh's set_default_shell_zsh makes zsh) rather than hardcoding a
-- distro-specific /bin/zsh vs /usr/bin/zsh path.
--
-- IMPORTANT: this file must NOT auto-launch a terminal multiplexer. Starting a
-- multiplexer from a terminal config double-starts sessions and fights the
-- multiplexer's own session management (the same class of bug the ghostty config
-- avoids). The shell is launched bare; start the multiplexer by hand. Guarded by
-- tests/wezterm/no_autolaunch_test.sh.

local wezterm = require("wezterm")
local config = wezterm.config_builder and wezterm.config_builder() or {}

-- ---- Theme: Rose Pine (dark), explicit pinned palette ----------------------
-- One constant palette across the whole stack (see CLAUDE.md "Rebind a Rose Pine
-- color"). The terminal ANSI assignment is the official rose-pine mapping,
-- identical to ghostty's built-in "Rose Pine" theme. selection_bg is highlightMed
-- (#403d52), the canonical rose-pine selection color (matches ghostty).
config.colors = {
  foreground = "#e0def4", -- text
  background = "#191724", -- base
  cursor_bg = "#e0def4", -- text
  cursor_fg = "#191724", -- base
  cursor_border = "#e0def4", -- text
  selection_fg = "#e0def4", -- text
  selection_bg = "#403d52", -- highlightMed
  scrollbar_thumb = "#26233a", -- overlay
  split = "#26233a", -- overlay
  ansi = {
    "#26233a", -- 0 black   overlay
    "#eb6f92", -- 1 red     love
    "#31748f", -- 2 green   pine
    "#f6c177", -- 3 yellow  gold
    "#9ccfd8", -- 4 blue    foam
    "#c4a7e7", -- 5 magenta iris
    "#ebbcba", -- 6 cyan    rose
    "#e0def4", -- 7 white   text
  },
  brights = {
    "#6e6a86", -- 8  bright black   muted
    "#eb6f92", -- 9  bright red     love
    "#31748f", -- 10 bright green   pine
    "#f6c177", -- 11 bright yellow  gold
    "#9ccfd8", -- 12 bright blue    foam
    "#c4a7e7", -- 13 bright magenta iris
    "#ebbcba", -- 14 bright cyan    rose
    "#e0def4", -- 15 bright white   text
  },
}

-- ---- Font ------------------------------------------------------------------
config.font = wezterm.font_with_fallback({
  "Hack Nerd Font",
  "Symbols Nerd Font Mono", -- Nerd Symbols fallback for glyphs Hack lacks
})
config.font_size = 13.0
-- Match ghostty's +liga/+calt so programming ligatures render consistently.
config.harfbuzz_features = { "calt=1", "liga=1", "clig=1" }

-- ---- Window / transparency -------------------------------------------------
-- Mild translucency only, mirroring ghostty (opacity 0.95, blur 15). Heavy blur
-- compounds compositing cost under a multiplexer with frequent redraws. For full
-- opacity set window_background_opacity = 1.0.
config.window_background_opacity = 0.95
config.macos_window_background_blur = 15 -- macOS only; ignored elsewhere
config.window_padding = { left = 6, right = 6, top = 6, bottom = 6 }
config.hide_tab_bar_if_only_one_tab = true
config.use_fancy_tab_bar = false
config.adjust_window_size_when_changing_font_size = false

-- ---- Cursor ----------------------------------------------------------------
config.default_cursor_style = "SteadyBlock" -- block, no blink (matches ghostty)

-- ---- Scrollback ------------------------------------------------------------
config.scrollback_lines = 5000000

-- ---- Shell -----------------------------------------------------------------
-- pwsh.exe on Windows; POSIX uses the login shell (zsh after set_default_shell_zsh).
if wezterm.target_triple and wezterm.target_triple:find("windows") ~= nil then
  config.default_prog = { "pwsh.exe", "-NoLogo" }
end

-- ---- Startup: open maximized on every platform -----------------------------
-- WezTerm has no static "start maximized" key; maximize the mux window on
-- gui-startup. This spawns ONLY the default shell window -- never a multiplexer.
wezterm.on("gui-startup", function(cmd)
  local _, _, window = wezterm.mux.spawn_window(cmd or {})
  window:gui_window():maximize()
end)

return config
