local M = {}

-- Invoked only under --clean: no init, Lazy setup, Mason refresh, parser
-- installation or cleanup is allowed in an installer observation.
function M.check()
  local runtime = assert(vim.env.DOTFILES_NVIM_RUNTIME, "missing managed runtime")
  local config = assert(vim.env.DOTFILES_NVIM_CONFIG, "missing configuration root")
  local function read_json(path)
    return vim.json.decode(table.concat(vim.fn.readfile(path), "\n"))
  end
  local lock = read_json(vim.fs.joinpath(config, "lazy-lock.json"))
  for name, pin in pairs(lock) do
    assert(name:match("^[%w_.-]+$") and pin.commit:match("^%x+$") and #pin.commit == 40, "invalid plugin lock")
    local path = vim.fs.joinpath(runtime, "lazy", name)
    local head = vim.system({ "git", "--no-optional-locks", "-C", path, "rev-parse", "HEAD" }, { text = true }):wait()
    assert(head.code == 0 and vim.trim(head.stdout) == pin.commit, "plugin differs from lock: " .. name)
    local files = vim.system({ "git", "--no-optional-locks", "-C", path, "diff", "--quiet", "HEAD", "--" }):wait()
    assert(files.code == 0, "plugin tracked files changed or are missing: " .. name)
  end

  vim.opt.rtp:prepend(vim.fs.joinpath(runtime, "site"))
  vim.opt.rtp:append(vim.fs.joinpath(runtime, "lazy", "nvim-treesitter"))
  local parsers = require("plugins.treesitter")[1].opts.dotfiles_parsers
  for _, language in ipairs(parsers) do
    local path = vim.fs.joinpath(runtime, "site", "parser", language .. ".so")
    assert(vim.uv.fs_stat(path), "missing parser: " .. language)
    assert(vim.treesitter.language.add(language, { path = path }), "cannot load parser: " .. language)
    assert(vim.treesitter.query.get(language, "highlights"), "missing highlights: " .. language)
  end
  for _, language in ipairs({ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" }) do
    assert(not vim.uv.fs_stat(vim.fs.joinpath(runtime, "site", "parser", language .. ".so")), "bundled parser override")
    assert(not vim.uv.fs_stat(vim.fs.joinpath(runtime, "site", "queries", language)), "bundled query override")
  end
  for _, name in ipairs(require("util.mason_tools").ensure_installed()) do
    local receipt = read_json(vim.fs.joinpath(runtime, "mason", "packages", name, "mason-receipt.json"))
    assert(receipt.name == name, "Mason receipt name differs: " .. name)
    assert(
      type(receipt.links) == "table" and type(receipt.links.bin) == "table",
      "missing Mason command links: " .. name
    )
    for command in pairs(receipt.links.bin) do
      local path = vim.fs.joinpath(runtime, "mason", "bin", command)
      local exists = vim.uv.fs_stat(path) or vim.uv.fs_stat(path .. ".cmd") or vim.uv.fs_stat(path .. ".exe")
      assert(exists, "missing Mason executable: " .. command)
    end
  end
end

function M.run()
  local ok, err = pcall(M.check)
  if not ok then
    vim.api.nvim_err_writeln("dotfiles Neovim check failed: " .. tostring(err))
    vim.cmd("cquit 1")
    return
  end
  print("dotfiles Neovim runtime verified")
  vim.cmd("qa")
end

return M
