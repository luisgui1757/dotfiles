-- WezTerm config smoke test. Loads wezterm/wezterm.lua with a STUBBED
-- require("wezterm") and asserts it produces a sane, Rose Pine, no-multiplexer
-- config on both POSIX and Windows target triples. Runs under `nvim -l`,
-- `luajit`, or `lua` (see lua_smoke_test.sh). Any assert failure aborts the
-- interpreter with a nonzero exit, failing the test.

local repo_root = os.getenv("REPO_ROOT") or "."
local path = repo_root .. "/wezterm/wezterm.lua"

-- Minimal wezterm stub: only the surface wezterm.lua touches at config-load
-- time. gui-startup's callback body is registered but never invoked here, so
-- mux/gui_window only need to exist, not do anything.
local function make_stub(triple)
  return {
    target_triple = triple,
    config_builder = function()
      return {}
    end,
    font_with_fallback = function(list)
      return { fallback = list }
    end,
    on = function(_, _) end,
    mux = {
      spawn_window = function()
        return nil, nil, {
          gui_window = function()
            return { maximize = function() end }
          end,
        }
      end,
    },
  }
end

local function load_config(triple)
  package.loaded["wezterm"] = make_stub(triple)
  package.preload["wezterm"] = function()
    return package.loaded["wezterm"]
  end
  local chunk, err = loadfile(path)
  assert(chunk, "failed to load " .. path .. ": " .. tostring(err))
  local cfg = chunk()
  assert(type(cfg) == "table", "wezterm.lua must return a config table")
  return cfg
end

local function contains(t, v)
  for _, x in ipairs(t) do
    if x == v then
      return true
    end
  end
  return false
end

-- ---- POSIX (non-windows triple) --------------------------------------------
local cfg = load_config("aarch64-apple-darwin")

-- Rose Pine pinned palette
assert(cfg.colors, "config.colors missing")
assert(cfg.colors.background == "#191724", "background must be Rose Pine base #191724")
assert(cfg.colors.foreground == "#e0def4", "foreground must be Rose Pine text #e0def4")
assert(cfg.colors.ansi and #cfg.colors.ansi == 8, "expected 8 ansi colors")
assert(cfg.colors.brights and #cfg.colors.brights == 8, "expected 8 bright colors")
assert(contains(cfg.colors.ansi, "#eb6f92"), "love (#eb6f92) missing from ansi")
assert(contains(cfg.colors.ansi, "#f6c177"), "gold (#f6c177) missing from ansi")
assert(contains(cfg.colors.ansi, "#9ccfd8"), "foam (#9ccfd8) missing from ansi")
assert(contains(cfg.colors.ansi, "#31748f"), "pine (#31748f) missing from ansi")

-- font / transparency / scrollback / cursor parity with ghostty
assert(cfg.font, "font missing")
assert(cfg.window_background_opacity == 0.95, "opacity must be 0.95")
assert(cfg.scrollback_lines == 5000000, "scrollback must be 5000000 lines")
assert(cfg.default_cursor_style == "SteadyBlock", "cursor must be SteadyBlock")
assert(cfg.window_padding and cfg.window_padding.left == 6, "padding must be 6")

-- POSIX must NOT hardcode default_prog; it uses the login shell (zsh).
assert(cfg.default_prog == nil, "POSIX default_prog must be nil (login shell)")
print("wezterm smoke OK (posix -> login shell)")

-- ---- Windows (windows triple) ----------------------------------------------
local wcfg = load_config("x86_64-pc-windows-msvc")
assert(
  wcfg.default_prog and wcfg.default_prog[1] == "pwsh.exe",
  "Windows default_prog must launch pwsh.exe"
)
print("wezterm smoke OK (windows -> pwsh.exe)")

print("wezterm smoke: all assertions passed")
