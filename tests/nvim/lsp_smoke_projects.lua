local M = {}

local function checked_mkdir(path)
  local result = vim.fn.mkdir(path, "p")
  if result ~= 1 and vim.fn.isdirectory(path) ~= 1 then
    error("could not create isolated LSP project directory: " .. path)
  end
end

function M.prepare(opts)
  vim.validate({
    root = { opts.root, "string" },
    fixtures = { opts.fixtures, "string" },
    fixture = { opts.fixture, "string" },
    lsp = { opts.lsp, "string" },
    index = { opts.index, "number" },
  })

  local safe_lsp = opts.lsp:gsub("[^%w_.-]", "-")
  local project_root = vim.fs.joinpath(opts.root, string.format("%03d-%s", opts.index, safe_lsp))
  checked_mkdir(project_root)
  -- Every attach probe gets its own project boundary. An empty .git directory
  -- is sufficient for Neovim's root-marker discovery and keeps language
  -- servers from scanning the repository-wide fixture tree.
  checked_mkdir(vim.fs.joinpath(project_root, ".git"))

  local source = vim.fs.joinpath(opts.fixtures, opts.fixture)
  local target = vim.fs.joinpath(project_root, vim.fn.fnamemodify(opts.fixture, ":t"))
  local read_ok, lines = pcall(vim.fn.readfile, source)
  if not read_ok then
    error("could not read LSP fixture " .. source .. ": " .. tostring(lines))
  end
  local write_ok, write_error = pcall(vim.fn.writefile, lines, target)
  if not write_ok then
    error("could not publish isolated LSP fixture " .. target .. ": " .. tostring(write_error))
  end

  return target, project_root
end

return M
