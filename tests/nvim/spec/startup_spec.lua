describe("startup time", function()
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

    local result = vim.system(args, {
      env = env,
      text = true,
    }):wait()

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
    local result = vim.system({
      vim.v.progpath,
      "--headless",
      "--clean",
      "--cmd",
      "lua io.stdout:write(vim.fn.stdpath('data'))",
      "+qa",
    }, {
      env = env,
      text = true,
    }):wait()

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

  local function locked_plugin_names(repo_root)
    local lockfile = repo_root .. "/nvim/lazy-lock.json"
    local ok, lines = pcall(vim.fn.readfile, lockfile)
    assert.is_true(ok, "could not read " .. lockfile)

    local decoded_ok, lock = pcall(vim.json.decode, table.concat(lines, "\n"))
    assert.is_true(decoded_ok, "could not parse " .. lockfile)

    local names = {}
    for name in pairs(lock) do
      table.insert(names, name)
    end
    table.sort(names)
    return names
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
    assert.are.same({}, missing, "plugin prewarm did not install locked plugin cache(s): " .. table.concat(missing, ", "))
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
    if vim.fn.isdirectory(lazy_path) == 0 then
      run_real_init(env, run_root .. "/prewarm.log")
    end

    -- A cold Lazy cache can still be installing the plugin graph after lazy.nvim
    -- itself exists. Do that work before measuring, otherwise the startup budget
    -- test measures first-run network/build cost instead of warm real-init time.
    -- Once the locked plugin graph is cached, skip the sync so ordinary test
    -- runs do not contact remotes or measure dependency-management work.
    if #missing_plugin_caches(data_path, repo_root) > 0 then
      run_real_init(env, run_root .. "/prewarm-plugins.log", { "+Lazy! sync", "+qa" })
    end
    assert_plugin_cache(data_path, repo_root)

    local lazy_mtime = mtime_id(lazy_path)
    local skipped_second_prewarm = false
    if vim.fn.isdirectory(lazy_path) == 0 then
      run_real_init(env, run_root .. "/prewarm-second.log")
    else
      skipped_second_prewarm = true
    end
    assert.is_true(skipped_second_prewarm, "lazy prewarm skip path was not exercised")
    assert.are.equal(lazy_mtime, mtime_id(lazy_path), "lazy.nvim cache changed during prewarm skip")

    local measurements = {}
    local fastest_ms
    for attempt = 1, 3 do
      local attempt_logfile = string.format("%s/startuptime-%d.log", run_root, attempt)
      run_real_init(env, attempt_logfile)
      assert.are.equal(lazy_mtime, mtime_id(lazy_path), "startup did not reuse the prewarmed lazy.nvim cache")

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
