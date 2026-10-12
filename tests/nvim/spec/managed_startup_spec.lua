local repo = _G.TEST_REPO_ROOT
local checkout = require("util.pinned_git_checkout")

describe("published managed runtime startup", function()
  local root, env, runtime, config
  local lock = vim.json.decode(table.concat(vim.fn.readfile(repo .. "/nvim/lazy-lock.json"), "\n"))

  local function run(args)
    local result = vim.system(args, { text = true, env = env }):wait(60000)
    assert.equals(0, result.code, result.stderr)
    return vim.trim(result.stdout)
  end

  local function copy_tree(source, destination)
    assert(vim.fn.mkdir(destination, "p") >= 0)
    for name, kind in vim.fs.dir(source) do
      local from, to = vim.fs.joinpath(source, name), vim.fs.joinpath(destination, name)
      if kind == "directory" then
        copy_tree(from, to)
      else
        assert.equals("file", kind, "unexpected fixture source: " .. from)
        assert(vim.uv.fs_copyfile(from, to))
      end
    end
  end

  local function snapshot(path)
    local entries = {}
    local function walk(directory, prefix)
      for name, kind in vim.fs.dir(directory) do
        local relative = prefix .. name
        local full = vim.fs.joinpath(directory, name)
        if kind == "directory" then
          entries[relative] = "directory"
          walk(full, relative .. "/")
        else
          assert.equals("file", kind)
          local file = assert(io.open(full, "rb"))
          entries[relative] = vim.fn.sha256(file:read("*a"))
          file:close()
        end
      end
    end
    walk(path, "")
    return entries
  end

  local function assert_unchanged(before, message)
    local after, changed = snapshot(runtime), {}
    for path, value in pairs(before) do
      if after[path] ~= value then
        table.insert(changed, path)
      end
    end
    for path in pairs(after) do
      if before[path] == nil then
        table.insert(changed, path)
      end
    end
    table.sort(changed)
    assert.same({}, changed, message)
  end

  before_each(function()
    root = vim.fn.tempname()
    config = root .. "/config/nvim"
    env = {
      HOME = root,
      USERPROFILE = root,
      LOCALAPPDATA = root .. "/local",
      APPDATA = root .. "/roaming",
      XDG_CONFIG_HOME = root .. "/config",
      XDG_DATA_HOME = root .. "/data",
      XDG_STATE_HOME = root .. "/state",
      XDG_CACHE_HOME = root .. "/cache",
      DOTFILES_NVIM_RUNTIME = "",
      DOTFILES_NVIM_LOCKFILE = "",
      DOTFILES_NVIM_SYNC = "",
      DOTFILES_TREESITTER_SYNC_INSTALL = "",
      NVIM_APPNAME = "nvim",
    }
    local data = run({ vim.v.progpath, "--headless", "--clean", "+lua io.stdout:write(vim.fn.stdpath('data'))", "+qa" })
    runtime = vim.fs.joinpath(data, "dotfiles-runtime/nvim.sync/current")
    copy_tree(repo .. "/nvim", config)
    assert.equals(0, vim.fn.delete(config .. "/lua/plugins", "rf"))
    vim.fn.mkdir(config .. "/lua/plugins", "p")
    assert(vim.uv.fs_copyfile(repo .. "/nvim/lua/plugins/treesitter.lua", config .. "/lua/plugins/treesitter.lua"))
    local fixture_lock = {}
    for name, url in pairs({
      ["lazy.nvim"] = "https://github.com/folke/lazy.nvim.git",
      ["nvim-treesitter"] = "https://github.com/nvim-treesitter/nvim-treesitter.git",
    }) do
      local cached = repo .. "/tests/.cache/managed-startup/" .. name
      checkout.ensure({
        target = cached,
        url = url,
        commit = lock[name].commit,
        branch = lock[name].branch,
        required_file = name == "lazy.nvim" and "lua/lazy/init.lua" or "lua/nvim-treesitter/init.lua",
      })
      copy_tree(cached, vim.fs.joinpath(runtime, "lazy", name))
      fixture_lock[name] = lock[name]
    end
    vim.fn.writefile({ vim.json.encode(fixture_lock) }, config .. "/lazy-lock.json")
    vim.fn.mkdir(runtime .. "/site/parser", "p")
  end)

  after_each(function()
    vim.fn.delete(root, "rf")
  end)

  local function startup(file)
    local result_path = root .. "/startup.json"
    local finish = string.format(
      "vim.defer_fn(function() vim.fn.writefile({vim.json.encode({uis=#vim.api.nvim_list_uis(), messages=vim.api.nvim_exec2('messages',{output=true}).output})}, %q); vim.cmd('qa!') end, 200)",
      result_path
    )
    local args = { vim.v.progpath, "--embed", "-i", "NONE", "-u", config .. "/init.lua" }
    if file then
      vim.list_extend(args, { "-c", "edit " .. vim.fn.fnameescape(file) })
    end
    vim.list_extend(args, { "-c", "lua " .. finish })
    -- An actual external UI resumes ordinary startup. Drain its binary redraw
    -- notifications instead of dispatching them as RPC calls to this test's
    -- Neovim server. External messages retain diagnostics without a terminal's
    -- hit-enter pagination, exactly as documented by :help ui-messages.
    local read_error
    local child = vim.system(args, {
      env = env,
      stdin = true,
      stdout = function(err)
        read_error = read_error or err
      end,
    })
    child:write(vim.mpack.encode({ 0, 1, "nvim_ui_attach", { 80, 24, { rgb = true, ext_messages = true } } }))
    child:write(vim.mpack.encode({ 2, "nvim_input", { "\r" } }))
    local result = child:wait(15000)
    assert.is_nil(read_error)
    assert.equals(0, result.code, result.stderr)
    return vim.json.decode(table.concat(vim.fn.readfile(result_path), "\n"))
  end

  it("preserves the published Lazy checkout when the current configuration requests another pin", function()
    local lazy = runtime .. "/lazy/lazy.nvim"
    vim.fn.writefile({ "previous approved generation" }, lazy .. "/previous-generation.txt")
    run({ "git", "-C", lazy, "add", "previous-generation.txt" })
    run({
      "git",
      "-C",
      lazy,
      "-c",
      "user.name=Fixture",
      "-c",
      "user.email=fixture@example.invalid",
      "commit",
      "-qm",
      "previous generation",
    })
    local before = snapshot(runtime)
    local result = startup()
    assert.equals(1, result.uis, "must exercise ordinary attached-UI startup")
    assert_unchanged(before, "startup replaced the published executable generation")
    assert.matches("Update/Repair", result.messages, nil, true)
    assert.matches("HEAD mismatch", result.messages, nil, true)
  end)

  it("preserves missing plugins instead of installing into the published generation", function()
    assert.equals(0, vim.fn.delete(runtime .. "/lazy/nvim-treesitter", "rf"))
    local before = snapshot(runtime)
    local result = startup()
    assert_unchanged(before, "startup installed a missing managed plugin")
    assert.matches("Update/Repair", result.messages, nil, true)
    assert.matches("nvim-treesitter", result.messages, nil, true)
  end)

  local function parser_probe()
    if vim.fn.has("win32") == 1 then
      env.PATH = vim.fn.fnamemodify(vim.fn.exepath("git"), ":h") .. ";" .. vim.env.SystemRoot .. "\\System32"
    else
      local bin = root .. "/bin"
      vim.fn.mkdir(bin, "p")
      vim.fn.writefile({ "#!/bin/sh", "exec " .. vim.fn.shellescape(vim.fn.exepath("git")) .. ' "$@"' }, bin .. "/git")
      assert(vim.uv.fs_chmod(bin .. "/git", 493))
      env.PATH = bin
    end
    assert.equals(
      "0",
      run({ vim.v.progpath, "--headless", "--clean", "+lua io.stdout:write(vim.fn.executable('tree-sitter'))", "+qa" })
    )
    local probe = root .. "/probe.py"
    vim.fn.writefile({ "value = 1" }, probe)
    return probe
  end

  it("reports bundled overrides without deleting them or creating directories", function()
    -- No compiler is needed to expose the old mutation: its cleanup deleted
    -- this file and get_install_dir created missing bookkeeping/query dirs.
    -- Match Neovim's own loader: Windows bundles lua.dll, POSIX bundles lua.so.
    local bundled_lua = assert(vim.api.nvim_get_runtime_file("parser/lua.*", false)[1])
    assert(vim.uv.fs_copyfile(bundled_lua, runtime .. "/site/parser/lua.so"))
    local probe = parser_probe()
    local before = snapshot(runtime)
    local result = startup(probe)
    assert.equals(1, result.uis)
    assert_unchanged(before, "ordinary UI startup mutated parser payloads")
    assert.matches("Update/Repair", result.messages, nil, true)
    assert.matches("unexpected bundled parser/query override: lua", result.messages, nil, true)
  end)

  it("reports missing parsers without installing into an otherwise clean published runtime", function()
    local probe = parser_probe()
    assert.same({}, vim.fn.readdir(runtime .. "/site/parser"), "this case must not hit the bundled-override guard")
    local before = snapshot(runtime)
    local result = startup(probe)
    assert.equals(1, result.uis)
    assert_unchanged(before, "ordinary UI startup provisioned a missing parser")
    assert.matches("Update/Repair", result.messages, nil, true)
    assert.matches("missing parser or highlight queries:", result.messages, nil, true)
  end)
end)
