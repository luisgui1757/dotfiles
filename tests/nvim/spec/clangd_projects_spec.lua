local repo_root = _G.TEST_REPO_ROOT

local function write_compile_database(directory, source, define)
  vim.fn.mkdir(directory, "p")
  local compiler = vim.fn.exepath("clang++")
  local database = {
    {
      directory = vim.fs.dirname(source),
      file = source,
      arguments = { compiler, "-std=c++17", "-D" .. define, "-c", source },
    },
  }
  vim.fn.writefile({ vim.json.encode(database) }, vim.fs.joinpath(directory, "compile_commands.json"))
end

local function wait_for_clean_diagnostics(buffer)
  local response = vim.lsp.buf_request_sync(buffer, "textDocument/documentSymbol", {
    textDocument = { uri = vim.uri_from_bufnr(buffer) },
  }, 15000)
  assert.is_not_nil(response, "clangd did not answer a document request")
  vim.wait(5000, function()
    return not vim.tbl_isempty(vim.lsp.get_clients({ bufnr = buffer }))
  end, 25)
  assert.are.same({}, vim.diagnostic.get(buffer))
end

describe("clangd project isolation", function()
  it("uses distinct root-local compile databases for two projects in one session", function()
    assert.are.equal(1, vim.fn.executable("clangd"), "runtime test requires a real clangd binary")
    assert.are.equal(1, vim.fn.executable("clang++"), "runtime test requires a real clang++ binary")

    local root = vim.fn.tempname()
    local project_one = vim.fs.joinpath(root, "Project One")
    local project_two = vim.fs.joinpath(root, "Project Two")
    local source_one = vim.fs.joinpath(project_one, "src", "one.cpp")
    local source_two = vim.fs.joinpath(project_two, "src", "two.cpp")
    vim.fn.mkdir(vim.fs.dirname(source_one), "p")
    vim.fn.mkdir(vim.fs.dirname(source_two), "p")
    vim.fn.writefile({
      "#ifndef PROJECT_ONE",
      '#error "PROJECT_ONE compile database was not used"',
      "#endif",
      "int project_one_symbol = 1;",
    }, source_one)
    vim.fn.writefile({
      "#ifndef PROJECT_TWO",
      '#error "PROJECT_TWO compile database was not used"',
      "#endif",
      "int project_two_symbol = 2;",
    }, source_two)
    write_compile_database(project_one, source_one, "PROJECT_ONE")
    write_compile_database(vim.fs.joinpath(project_two, "build"), source_two, "PROJECT_TWO")

    local old_cmp = package.loaded["cmp_nvim_lsp"]
    local old_config = vim.lsp.config
    local old_enable = vim.lsp.enable
    local captured
    package.loaded["cmp_nvim_lsp"] = { default_capabilities = function()
      return {}
    end }
    vim.lsp.config = function(name, config)
      if name == "clangd" then
        captured = vim.deepcopy(config)
      end
    end
    vim.lsp.enable = function() end
    local spec = dofile(repo_root .. "/nvim/lua/plugins/lsp-config.lua")
    spec[3].config()
    package.loaded["cmp_nvim_lsp"] = old_cmp
    vim.lsp.config = old_config
    vim.lsp.enable = old_enable

    assert.is_not_nil(captured)
    for _, arg in ipairs(captured.cmd) do
      assert.is_nil(arg:match("^%-%-compile%-commands%-dir="))
    end

    local buffers = {}
    local clients = {}
    local ok, err = pcall(function()
      for _, item in ipairs({
        { source = source_one, root = project_one },
        { source = source_two, root = project_two },
      }) do
        local buffer = vim.fn.bufadd(item.source)
        vim.fn.bufload(buffer)
        vim.bo[buffer].filetype = "cpp"
        table.insert(buffers, buffer)
        local config = vim.tbl_deep_extend("force", captured, {
          name = "dotfiles-clangd-isolation",
          root_dir = item.root,
        })
        local client_id = vim.lsp.start(config, { bufnr = buffer })
        assert.is_not_nil(client_id, "clangd client did not start for " .. item.source)
        table.insert(clients, client_id)
      end

      assert.are_not.equal(clients[1], clients[2])
      assert.is_true(vim.wait(15000, function()
        return #vim.lsp.get_clients({ bufnr = buffers[1] }) == 1
          and #vim.lsp.get_clients({ bufnr = buffers[2] }) == 1
      end, 25))
      assert.are.equal(project_one, vim.lsp.get_client_by_id(clients[1]).root_dir)
      assert.are.equal(project_two, vim.lsp.get_client_by_id(clients[2]).root_dir)
      wait_for_clean_diagnostics(buffers[1])
      wait_for_clean_diagnostics(buffers[2])
    end)

    for _, client_id in ipairs(clients) do
      local client = vim.lsp.get_client_by_id(client_id)
      if client then
        client:stop(true)
      end
    end
    for _, buffer in ipairs(buffers) do
      pcall(vim.api.nvim_buf_delete, buffer, { force = true })
    end
    vim.fn.delete(root, "rf")
    assert.is_true(ok, tostring(err))
  end)
end)
