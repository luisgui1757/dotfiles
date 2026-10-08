-- Run after the complete native Neovim install, with a disposable NOTES_VAULT.
-- Both modes use the actual locked plugin; no user vault or stub is involved.
local ok, err = pcall(function()
  local mode = vim.env.DOTFILES_NOTES_SMOKE
  assert(mode == "absent" or mode == "existing", "notes smoke mode is required")
  local vault = require("util.notes_path").resolve()
  assert((vim.fn.isdirectory(vault) == 1) == (mode == "existing"), "unexpected notes fixture state")
  local plugin = require("lazy.core.config").plugins["obsidian.nvim"]
  assert(plugin and vim.uv.fs_stat(plugin.dir), "locked notes plugin was not provisioned")
  require("lazy").load({ plugins = { "obsidian.nvim" } })
  if mode == "absent" then
    assert(vim.fn.isdirectory(vault) == 0, "notes activation created an absent vault")
    assert(package.loaded["obsidian"] == nil, "absent vault activated Obsidian")
    assert(vim.fn.exists(":ObsidianNew") == 0, "absent vault registered Obsidian commands")
  else
    local client = require("obsidian").get_client()
    assert(vim.fs.normalize(tostring(client.dir)) == vim.fs.normalize(vault), "Obsidian selected a different vault")
    assert(vim.fn.exists(":ObsidianNew") == 2, "existing vault did not activate Obsidian commands")
  end
  print("notes smoke: " .. mode .. " vault verified")
end)
if not ok then
  vim.api.nvim_err_writeln("notes smoke failed: " .. tostring(err))
  vim.cmd("cquit 1")
end
