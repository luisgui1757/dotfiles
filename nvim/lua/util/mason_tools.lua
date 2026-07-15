local M = {}

local function expected_tools()
  local tools = {
    -- LSP servers
    "lua-language-server",
    "pyright",
    "rust-analyzer",
    "bash-language-server",
    "yaml-language-server",
    "json-lsp",
    "neocmakelsp",
    "powershell-editor-services",
    -- Formatters
    "stylua",
    "shfmt",
    "prettier",
    "clang-format",
    "gersemi",
    "ruff",
    -- DAP adapters
    "js-debug-adapter",
  }

  -- The Mason registry publishes clangd for macOS, Windows, and Linux x64,
  -- but not Linux arm64. Linux owns clangd through Home Manager on every
  -- supported architecture instead, avoiding platform-dependent ownership.
  if vim.fn.has("linux") ~= 1 then
    table.insert(tools, 2, "clangd")
  end

  return tools
end

local function quit_failed(message)
  vim.api.nvim_err_writeln("dotfiles Mason sync failed: " .. message)
  vim.cmd("cquit 1")
end

function M.ensure_installed()
  return expected_tools()
end

function M.run_checked(command)
  local ok, err = pcall(vim.cmd, command)
  if not ok then
    quit_failed(command .. ": " .. tostring(err))
    return
  end

  local registry_ok, registry = pcall(require, "mason-registry")
  if not registry_ok then
    quit_failed("could not load mason-registry: " .. tostring(registry))
    return
  end

  local missing = {}
  for _, name in ipairs(expected_tools()) do
    local package_ok, package = pcall(registry.get_package, name)
    if not package_ok or not package:is_installed() then
      table.insert(missing, name)
    end
  end

  if #missing > 0 then
    quit_failed("missing packages after " .. command .. ": " .. table.concat(missing, ", "))
    return
  end

  vim.cmd("qa")
end

return M
