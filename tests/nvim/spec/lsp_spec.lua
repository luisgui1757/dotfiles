-- Read the LSP plugin spec and assert all expected servers are configured.

local repo_root = _G.TEST_REPO_ROOT
local fh = assert(io.open(repo_root .. "/nvim/lua/plugins/lsp-config.lua", "r"))
local src = fh:read("*a")
fh:close()

local mason_fh = assert(io.open(repo_root .. "/nvim/lua/util/mason_tools.lua", "r"))
local mason_src = mason_fh:read("*a")
mason_fh:close()

-- Strip Lua comments so test patterns don't false-positive on comment text
-- like "-- don't add a BufWritePre formatter here".
local function strip_comments(s)
  -- block comments first
  s = s:gsub("%-%-%[%[.-%]%]", "")
  -- line comments
  s = s:gsub("%-%-[^\n]*", "")
  return s
end
local code_only = strip_comments(src)

describe("LSP server coverage", function()
  local required_servers = {
    "clangd",
    "lua_ls",
    "pyright",
    "rust_analyzer",
    "bashls",
    "yamlls",
    "jsonls",
    "neocmake",
    "powershell_es",
  }

  for _, server in ipairs(required_servers) do
    it("configures " .. server, function()
      local pattern = 'vim%.lsp%.config%("' .. server .. '"'
      assert.is_truthy(code_only:match(pattern), 'vim.lsp.config("' .. server .. '", ...) not found in lsp-config.lua')
    end)
    it("enables " .. server, function()
      -- The enabled list is built as `local enabled_servers = { ... }` then
      -- passed to vim.lsp.enable. powershell_es is appended conditionally via
      -- table.insert (it needs pwsh + the PSES bundle), so accept either spot.
      assert.is_truthy(
        code_only:find("vim.lsp.enable(enabled_servers)", 1, true),
        "vim.lsp.enable(enabled_servers) call missing"
      )
      local enable_block = code_only:match("enabled_servers%s*=%s*{(.-)}")
      assert.is_not_nil(enable_block, "enabled_servers = { ... } block missing")
      local in_list = enable_block:find('"' .. server .. '"', 1, true)
      local inserted = code_only:find('table.insert(enabled_servers, "' .. server .. '")', 1, true)
      assert.is_truthy(in_list or inserted, server .. " not enabled (missing from enabled_servers and table.insert)")
    end)
  end

  it("uses :supports_method (colon syntax) not dot syntax", function()
    assert.is_nil(
      code_only:match("client%.supports_method"),
      "client.supports_method (dot) is deprecated; use client:supports_method (colon)"
    )
  end)

  it("does not register a BufWritePre formatter on LspAttach", function()
    assert.is_nil(code_only:match("BufWritePre"), "format-on-save belongs in conform.nvim, not here")
  end)

  -- setup.{sh,ps1} invoke these through util.mason_tools.run_checked. The
  -- upstream command still has to lazy-load mason-tool-installer, and the
  -- wrapper must turn command errors or missing postconditions into a nonzero
  -- Neovim exit instead of letting headless setup fake-green.
  -- VeryLazy never fires without a UI, so each command MUST also be a lazy `cmd`
  -- load-trigger or the headless invocation dies with "E492: Not an editor
  -- command". Regression guard for exactly that.
  for _, mcmd in ipairs({ "MasonToolsInstallSync", "MasonToolsUpdateSync" }) do
    it("registers " .. mcmd .. " as a cmd load-trigger (headless setup phase)", function()
      assert.is_truthy(
        code_only:find(mcmd, 1, true),
        mcmd .. " is not a cmd trigger in lsp-config.lua; headless `nvim +" .. mcmd .. "` would fail with E492"
      )
    end)
  end

  it("uses the checked Mason manifest and fail-closed sync wrapper", function()
    assert.is_truthy(
      code_only:find('require("util.mason_tools").ensure_installed()', 1, true),
      "mason-tool-installer must consume the shared checked manifest"
    )
    assert.is_truthy(
      mason_src:find('if vim.fn.has("linux") ~= 1 then', 1, true),
      "Linux must leave clangd to the Nix package layer"
    )
    assert.is_truthy(
      mason_src:find('table.insert(tools, 2, "clangd")', 1, true),
      "non-Linux hosts must retain Mason-owned clangd"
    )
    assert.is_truthy(
      mason_src:find("local ok, err = pcall(vim.cmd, command)", 1, true),
      "checked Mason sync must catch command failures"
    )
    assert.is_truthy(
      mason_src:find("package:is_installed()", 1, true),
      "checked Mason sync must validate every package postcondition"
    )
    assert.is_truthy(
      mason_src:find('vim.cmd("cquit 1")', 1, true),
      "checked Mason sync must produce a nonzero headless exit on failure"
    )
  end)

  it("starts neocmake from the real Mason package binary before PATH shims", function()
    assert.is_truthy(code_only:find("get_neocmake_cmd", 1, true), "neocmake command resolver missing")
    assert.is_truthy(
      code_only:find('cmd = get_neocmake_cmd()', 1, true),
      "neocmake must use the package-binary command resolver"
    )
    assert.is_truthy(
      code_only:find('/mason/packages/neocmakelsp/', 1, true),
      "neocmake resolver must prefer Mason's package directory over mason/bin shims"
    )
    assert.is_truthy(
      code_only:find("neocmakelsp.exe", 1, true),
      "neocmake resolver must handle Windows' real neocmakelsp.exe binary"
    )
  end)

  it("leaves compile-database discovery to each clangd project client", function()
    assert.is_truthy(
      src:find('local clangd_cmd = { "clangd", "--background-index", "--clang-tidy" }', 1, true),
      "clangd must use a cwd-independent command"
    )
    assert.is_nil(
      src:find("--compile-commands-dir", 1, true),
      "a session-wide compile database override freezes clangd to one project"
    )
  end)
end)
