local M = {}

-- The installer switches only this private runtime link. Personal editor state
-- (Shada, undo, sessions and the existing data tree) keeps its normal location.
function M.root()
  local explicit = vim.env.DOTFILES_NVIM_RUNTIME
  if explicit and explicit ~= "" then
    return explicit
  end
  local current = vim.fs.joinpath(vim.fn.stdpath("data"), "dotfiles-runtime", "nvim.sync", "current")
  if vim.uv.fs_stat(current) then
    return current
  end
  return nil
end

function M.path(name)
  return vim.fs.joinpath(M.root() or vim.fn.stdpath("data"), name)
end

-- A published generation supplies the user site too. Old user-installed parsers
-- must not shadow the managed set or Neovim's bundled grammars and queries.
function M.activate_site()
  local root = M.root()
  if not root then
    return
  end
  local legacy = vim.fs.joinpath(vim.fn.stdpath("data"), "site")
  vim.opt.rtp:remove({ legacy, vim.fs.joinpath(legacy, "after") })
  vim.opt.rtp:prepend(vim.fs.joinpath(root, "site"))
end

return M
