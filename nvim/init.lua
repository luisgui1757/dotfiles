-- Leader must be set BEFORE lazy loads any plugin spec, otherwise
-- every plugin keymap that uses <leader> resolves to '\'.
vim.g.mapleader = " "
vim.g.maplocalleader = " "

local function config_dir()
  local src = debug.getinfo(1, "S").source:sub(2)
  return vim.fn.fnamemodify(vim.fn.resolve(src), ":p:h")
end

local pinned_checkout = require("util.pinned_git_checkout")
local lazypath = require("util.managed_runtime").path("lazy/lazy.nvim")
local lockfile = vim.env.DOTFILES_NVIM_LOCKFILE or (config_dir() .. "/lazy-lock.json")
local lazy_commit, lazy_branch = pinned_checkout.locked_identity(lockfile, "lazy.nvim")
pinned_checkout.ensure({
  target = lazypath,
  url = "https://github.com/folke/lazy.nvim.git",
  commit = lazy_commit,
  branch = lazy_branch,
  required_file = "lua/lazy/init.lua",
})
vim.opt.rtp:prepend(lazypath)

require("vim-options")
require("lazy").setup({
  lockfile = lockfile,
  root = require("util.managed_runtime").path("lazy"),
  spec = { { import = "plugins" } },
  change_detection = { notify = false },
  -- Retain final command output/errors without streaming every clone percentage
  -- through the installer's bounded native-worker reply.
  headless = vim.env.DOTFILES_NVIM_RUNTIME and { process = false, colors = false } or nil,
  performance = {
    -- vim.loader encodes an entire source path in one filename (NAME_MAX).
    -- Managed editor runtimes resolve to long version paths even on normal
    -- startup. Disable optional bytecode caching; plugin lazy loading remains.
    cache = { enabled = false },
    rtp = {
      -- Netrw stays enabled — it provides :Explore, gx URL-opening,
      -- and is what `:E` resolves to.
      disabled_plugins = {
        "gzip",
        "tarPlugin",
        "tohtml",
        "tutor",
        "zipPlugin",
      },
    },
  },
})
require("util.managed_runtime").activate_site()
