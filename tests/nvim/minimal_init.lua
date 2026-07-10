-- Minimal nvim init for the plenary busted suite.
--
-- Two execution contexts:
--   1. Parent (`PlenaryBustedDirectory`) — uses this to discover & schedule.
--   2. Subprocess per spec file — re-runs this so each spec gets a clean nvim.
--
-- Goals:
--   * Make the repo's `nvim/lua/...` requireable as if it were $XDG_CONFIG_HOME/nvim.
--   * Install + register plenary so PlenaryBustedDirectory/Plenary's busted exist.
--   * Set the same options/keymaps/commands that `nvim/init.lua` sets, so
--     tests that ask "is vim.g.mapleader space?" see the production state.

local function script_path()
  -- debug.getinfo(1, "S").source is "@/abs/or/relative/path"
  local src = debug.getinfo(1, "S").source:sub(2)
  return vim.fn.resolve(vim.fn.fnamemodify(src, ":p"))
end

local this_file = script_path()
local repo_root = vim.fn.fnamemodify(this_file, ":h:h:h") -- tests/nvim/minimal_init.lua → repo root

-- Production-equivalent leader (must come before plugin specs / any keymap).
vim.g.mapleader = " "
vim.g.maplocalleader = " "

-- Add the repo's nvim/ tree (init.lua's stdpath-config equivalent).
vim.opt.runtimepath:prepend(repo_root .. "/nvim")

-- Plenary is executable test infrastructure, so it uses the same fail-closed
-- identity proof and transactional cache publication as production lazy.nvim.
local pinned_checkout = require("util.pinned_git_checkout")
local plenary_dir = repo_root .. "/tests/.cache/plenary.nvim"
local plenary_commit = pinned_checkout.locked_commit(repo_root .. "/nvim/lazy-lock.json", "plenary.nvim")
pinned_checkout.ensure({
  target = plenary_dir,
  url = "https://github.com/nvim-lua/plenary.nvim.git",
  commit = plenary_commit,
  required_file = "lua/plenary/init.lua",
})
vim.opt.runtimepath:prepend(plenary_dir)
-- Explicitly source plenary's plugin file (plain `-u` doesn't auto-source
-- plugin/*.vim when --noplugin was passed by PlenaryBustedDirectory).
pcall(vim.cmd, "runtime plugin/plenary.vim")

-- Production options + the :WNF command.
require("vim-options")

-- Make the repo root reachable from tests for read-the-source assertions.
_G.TEST_REPO_ROOT = repo_root
