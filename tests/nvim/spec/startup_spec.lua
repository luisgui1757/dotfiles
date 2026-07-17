describe("startup time", function()
  local function diag(message)
    io.stderr:write("[startup_spec] " .. message .. "\n")
    io.stderr:flush()
  end

  local function mkdir(path)
    vim.fn.mkdir(path, "p")
  end

  local function run_real_init(env, logfile, commands)
    local repo_root = _G.TEST_REPO_ROOT
    local nvim_config = repo_root .. "/nvim"
    local args = {
      vim.v.progpath,
      "--headless",
      "--cmd",
      "set runtimepath^=" .. vim.fn.fnameescape(nvim_config),
      "-u",
      nvim_config .. "/init.lua",
      "--startuptime",
      logfile,
    }

    for _, command in ipairs(commands or { "+qa" }) do
      table.insert(args, command)
    end

    diag("running real init; startuptime log: " .. logfile)
    local result = vim
      .system(args, {
        env = env,
        text = true,
      })
      :wait()

    assert.are.equal(
      0,
      result.code,
      table.concat({
        "nvim exited non-zero",
        "stdout: " .. tostring(result.stdout),
        "stderr: " .. tostring(result.stderr),
      }, "\n")
    )
  end

  local function parse_total_ms(logfile)
    local fh = assert(io.open(logfile, "r"))
    local total_ms
    for line in fh:lines() do
      local t = line:match("^%s*([%d%.]+)%s")
      if t then
        total_ms = tonumber(t)
      end
    end
    fh:close()
    return total_ms
  end

  local function child_stdpath_data(env)
    local result = vim
      .system({
        vim.v.progpath,
        "--headless",
        "--clean",
        "--cmd",
        "lua io.stdout:write(vim.fn.stdpath('data'))",
        "+qa",
      }, {
        env = env,
        text = true,
      })
      :wait()

    assert.are.equal(
      0,
      result.code,
      table.concat({
        "nvim stdpath probe exited non-zero",
        "stdout: " .. tostring(result.stdout),
        "stderr: " .. tostring(result.stderr),
      }, "\n")
    )

    local data_path = (result.stdout or ""):match("^[^\r\n]+")
    assert.is_not_nil(data_path, "could not read stdpath('data') from child nvim")
    return data_path:gsub("\\", "/")
  end

  local function mtime_id(path)
    local stat = assert(vim.uv.fs_stat(path), "missing lazy.nvim cache at " .. path)
    local mtime = stat.mtime or {}
    return tostring(mtime.sec) .. ":" .. tostring(mtime.nsec)
  end

  local function locked_plugin_lock(repo_root)
    local lockfile = repo_root .. "/nvim/lazy-lock.json"
    local ok, lines = pcall(vim.fn.readfile, lockfile)
    assert.is_true(ok, "could not read " .. lockfile)

    local decoded_ok, lock = pcall(vim.json.decode, table.concat(lines, "\n"))
    assert.is_true(decoded_ok, "could not parse " .. lockfile)
    return lock
  end

  local function locked_plugin_names(repo_root)
    local lock = locked_plugin_lock(repo_root)
    local names = {}
    for name in pairs(lock) do
      table.insert(names, name)
    end
    table.sort(names)
    return names
  end

  local function plugin_name_for_repo(repo, spec)
    if type(spec) == "table" and type(spec.name) == "string" then
      return spec.name
    end
    return (repo:match("([^/]+)$") or repo):gsub("%.git$", "")
  end

  local function has_named_keys(spec)
    for key in pairs(spec) do
      if type(key) ~= "number" then
        return true
      end
    end
    return false
  end

  local function collect_plugin_sources(spec, sources)
    if type(spec) == "string" then
      if spec:find("/", 1, true) then
        sources[plugin_name_for_repo(spec)] = spec
      end
      return
    end

    if type(spec) ~= "table" then
      return
    end

    if type(spec[1]) == "string" and spec[1]:find("/", 1, true) and (#spec == 1 or has_named_keys(spec)) then
      sources[plugin_name_for_repo(spec[1], spec)] = spec[1]
      collect_plugin_sources(spec.dependencies, sources)
      return
    end

    for _, child in ipairs(spec) do
      collect_plugin_sources(child, sources)
    end
  end

  local function locked_plugin_sources(repo_root)
    local lock = locked_plugin_lock(repo_root)
    local sources = {
      ["lazy.nvim"] = "folke/lazy.nvim",
    }

    local plugin_files = vim.fn.glob(repo_root .. "/nvim/lua/plugins/*.lua", false, true)
    table.sort(plugin_files)
    for _, path in ipairs(plugin_files) do
      local loaded, spec_or_err = pcall(dofile, path)
      assert.is_true(loaded, "could not load plugin spec " .. path .. ": " .. tostring(spec_or_err))
      collect_plugin_sources(spec_or_err, sources)
    end

    local names = {}
    for name in pairs(lock) do
      table.insert(names, name)
    end
    table.sort(names)
    for _, name in ipairs(names) do
      assert.is_not_nil(sources[name], "could not resolve plugin source for locked plugin " .. name)
    end

    return sources, lock, names
  end

  local function system_checked(args, context)
    diag(context .. ": " .. table.concat(args, " "))
    local result = vim.system(args, { text = true }):wait()
    assert.are.equal(
      0,
      result.code,
      table.concat({
        context,
        "cmd: " .. table.concat(args, " "),
        "stdout: " .. tostring(result.stdout),
        "stderr: " .. tostring(result.stderr),
      }, "\n")
    )
    return result
  end

  local function read_first_line(path)
    local ok, lines = pcall(vim.fn.readfile, path)
    if not ok or type(lines) ~= "table" then
      return nil
    end
    return lines[1]
  end

  local function read_packed_ref(git_dir, ref)
    local ok, lines = pcall(vim.fn.readfile, git_dir .. "/packed-refs")
    if not ok or type(lines) ~= "table" then
      return nil
    end
    for _, line in ipairs(lines) do
      if line:sub(1, 1) ~= "#" and line:sub(1, 1) ~= "^" then
        local sha, name = line:match("^([0-9a-fA-F]+)%s+(.+)$")
        if name == ref then
          return sha
        end
      end
    end
    return nil
  end

  local function git_head(path)
    local git_dir = path .. "/.git"
    local head = read_first_line(git_dir .. "/HEAD")
    if not head then
      return nil
    end

    local ref = head:match("^ref:%s*(.+)$")
    if ref then
      return read_first_line(git_dir .. "/" .. ref) or read_packed_ref(git_dir, ref)
    end
    return head:match("^[0-9a-fA-F]+$")
  end

  local function checkout_plugin(plugin_root, name, repo, commit)
    local path = plugin_root .. "/" .. name
    if vim.fn.isdirectory(path) == 0 then
      mkdir(plugin_root)
      diag("cloning locked plugin " .. name .. " from " .. repo)
      system_checked(
        { "git", "clone", "--filter=blob:none", "https://github.com/" .. repo .. ".git", path },
        "could not clone " .. name
      )
    end

    assert.are.equal(1, vim.fn.isdirectory(path .. "/.git"), "plugin cache is not a git checkout: " .. path)
    diag("verifying locked plugin " .. name .. " at " .. path)
    local current_head = git_head(path)
    if current_head ~= commit then
      diag("checking out locked plugin " .. name .. " @ " .. commit)
      local checkout = vim.system({ "git", "-C", path, "checkout", "--detach", commit }, { text = true }):wait()
      if checkout.code ~= 0 then
        system_checked(
          { "git", "-C", path, "fetch", "--filter=blob:none", "origin", commit },
          "could not fetch locked commit for " .. name
        )
        system_checked(
          { "git", "-C", path, "checkout", "--detach", commit },
          "could not checkout locked commit for " .. name
        )
      end
    end
    current_head = git_head(path)
    assert.are.equal(commit, current_head, "plugin cache did not land on locked commit for " .. name)
  end

  local function prewarm_locked_plugin_checkouts(data_path, repo_root)
    local sources, lock, names = locked_plugin_sources(repo_root)
    local plugin_root = data_path .. "/lazy"
    diag("prewarming " .. tostring(#names) .. " locked plugin checkout(s) under " .. plugin_root)
    for _, name in ipairs(names) do
      checkout_plugin(plugin_root, name, sources[name], lock[name].commit)
    end

    -- The production bootstrap proves more than HEAD: it also requires the
    -- reviewed origin, locked default-branch metadata, cleanliness, and entry
    -- file. A persistent prewarm cloned while upstream advertised another
    -- default branch can therefore have the right commit but still require a
    -- transactional production repair. Complete that exact proof before the
    -- timed init so the benchmark tests warm startup rather than cache repair.
    local lazy_lock = assert(lock["lazy.nvim"], "lazy.nvim lock entry is missing")
    diag("proving the prewarmed lazy.nvim cache through the production identity boundary")
    require("util.pinned_git_checkout").ensure({
      target = plugin_root .. "/lazy.nvim",
      url = "https://github.com/" .. sources["lazy.nvim"] .. ".git",
      commit = lazy_lock.commit,
      branch = lazy_lock.branch,
      required_file = "lua/lazy/init.lua",
    })
  end

  local function clear_startup_parser_outputs(data_path)
    for _, child in ipairs({ "parser", "parser-info", "queries" }) do
      pcall(vim.fn.delete, data_path .. "/site/" .. child, "rf")
    end
  end

  local function startup_parser_outputs(data_path)
    local outputs = {}
    for _, pattern in ipairs({ "site/parser/*.so", "site/parser-info/*.revision", "site/queries/*" }) do
      for _, path in ipairs(vim.fn.glob(data_path .. "/" .. pattern, false, true)) do
        table.insert(outputs, path:gsub("^" .. vim.pesc(data_path .. "/"), ""))
      end
    end
    table.sort(outputs)
    return outputs
  end

  local function assert_no_parser_build_outputs(data_path, context)
    assert.are.same(
      {},
      startup_parser_outputs(data_path),
      context .. " must not run nvim-treesitter parser builds inside the startup benchmark"
    )
  end

  local function missing_plugin_caches(data_path, repo_root)
    local missing = {}
    for _, name in ipairs(locked_plugin_names(repo_root)) do
      if vim.fn.isdirectory(data_path .. "/lazy/" .. name) == 0 then
        table.insert(missing, name)
      end
    end
    return missing
  end

  local function assert_plugin_cache(data_path, repo_root)
    local missing = missing_plugin_caches(data_path, repo_root)
    assert.are.same(
      {},
      missing,
      "plugin prewarm did not install locked plugin cache(s): " .. table.concat(missing, ", ")
    )
  end

  it("real init.lua completes under the OS-appropriate budget", function()
    local sysname = (vim.uv.os_uname() or {}).sysname or ""
    local budget_ms = 1200
    if sysname == "Darwin" then
      budget_ms = 800
    elseif sysname:match("Windows") then
      budget_ms = 2000
    end

    local repo_root = _G.TEST_REPO_ROOT
    local cache_root = repo_root .. "/tests/.cache/startup-real"
    local shared_data = cache_root .. "/data"
    local run_root = cache_root .. "/" .. string.format("%d", vim.uv.hrtime())
    local localappdata = run_root .. "/localappdata"
    diag("startup run root: " .. run_root)

    mkdir(shared_data)
    mkdir(run_root .. "/config")
    mkdir(run_root .. "/state")
    mkdir(run_root .. "/cache")
    mkdir(run_root .. "/run")
    mkdir(localappdata)
    mkdir(run_root .. "/appdata")
    mkdir(run_root .. "/userprofile")

    local env = {
      XDG_CONFIG_HOME = run_root .. "/config",
      XDG_DATA_HOME = shared_data,
      XDG_STATE_HOME = run_root .. "/state",
      XDG_CACHE_HOME = run_root .. "/cache",
      XDG_RUNTIME_DIR = run_root .. "/run",
      LOCALAPPDATA = localappdata,
      APPDATA = run_root .. "/appdata",
      USERPROFILE = run_root .. "/userprofile",
    }

    local data_path = child_stdpath_data(env)
    local lazy_path = data_path .. "/lazy/lazy.nvim"
    clear_startup_parser_outputs(data_path)

    -- Preclone the locked plugin graph before running the real init. Calling
    -- Lazy's installer/restore path here would also run plugin build hooks; for
    -- nvim-treesitter main, :TSUpdate starts asynchronous compiler work that can
    -- outlive the prewarm child and make the startup timing measure dependency
    -- bootstrap load instead of warm real-init time.
    prewarm_locked_plugin_checkouts(data_path, repo_root)
    assert_plugin_cache(data_path, repo_root)
    assert_no_parser_build_outputs(data_path, "plugin cache prewarm")

    local lazy_mtime = mtime_id(lazy_path)
    run_real_init(env, run_root .. "/prewarm.log")
    assert.are.equal(lazy_mtime, mtime_id(lazy_path), "real-init prewarm changed the prewarmed lazy.nvim cache")
    assert_no_parser_build_outputs(data_path, "real-init prewarm")

    local measurements = {}
    local fastest_ms
    for attempt = 1, 3 do
      local attempt_logfile = string.format("%s/startuptime-%d.log", run_root, attempt)
      run_real_init(env, attempt_logfile)
      assert.are.equal(lazy_mtime, mtime_id(lazy_path), "startup did not reuse the prewarmed lazy.nvim cache")
      assert_no_parser_build_outputs(data_path, "startup measurement")

      local total_ms = parse_total_ms(attempt_logfile)
      assert.is_not_nil(total_ms, "could not parse startuptime log; logs kept at " .. run_root)
      table.insert(measurements, string.format("attempt %d: %.1fms", attempt, total_ms))
      if not fastest_ms or total_ms < fastest_ms then
        fastest_ms = total_ms
      end
      if total_ms < budget_ms then
        break
      end
    end

    assert.is_true(
      fastest_ms < budget_ms,
      string.format(
        "startup best-of-3 took %.1fms (budget on %s = %dms; %s); logs kept at %s",
        fastest_ms,
        sysname,
        budget_ms,
        table.concat(measurements, ", "),
        run_root
      )
    )
    pcall(vim.fn.delete, run_root, "rf")
  end)
end)
