local checkout = require("util.pinned_git_checkout")

describe("pinned Git checkout", function()
  local root
  local remote
  local expected_commit
  local opts

  local function run(args)
    local result = vim.system(args, { text = true }):wait()
    return {
      code = result.code,
      stdout = result.stdout or "",
      stderr = result.stderr or "",
    }
  end

  local function git(...)
    local args = { "git" }
    vim.list_extend(args, { ... })
    local result = run(args)
    assert.are.equal(0, result.code, result.stderr)
    return vim.trim(result.stdout)
  end

  local function write(path, lines)
    vim.fn.mkdir(vim.fn.fnamemodify(path, ":h"), "p")
    vim.fn.writefile(lines, path)
  end

  local function stages_for(target)
    return vim.fn.glob(target .. ".stage.*", false, true)
  end

  before_each(function()
    root = vim.fn.tempname()
    remote = root .. "/remote"
    vim.fn.mkdir(remote, "p")
    git("-C", remote, "init", "--quiet")
    git("-C", remote, "config", "user.name", "Dotfiles Test")
    git("-C", remote, "config", "user.email", "dotfiles@example.invalid")
    write(remote .. "/lua/example/init.lua", { "return true" })
    git("-C", remote, "add", "lua/example/init.lua")
    git("-C", remote, "commit", "--quiet", "-m", "fixture")
    expected_commit = git("-C", remote, "rev-parse", "HEAD")
    local expected_branch = git("-C", remote, "symbolic-ref", "--short", "HEAD")
    opts = {
      target = root .. "/cache/example.nvim",
      url = "file://" .. remote,
      commit = expected_commit,
      branch = expected_branch,
      required_file = "lua/example/init.lua",
      lock_timeout_ms = 2000,
    }
  end)

  after_each(function()
    vim.fn.delete(root, "rf")
  end)

  it("rejects missing, empty, malformed, incomplete, and non-40-hex lock data", function()
    local lockfile = root .. "/lazy-lock.json"
    assert.has_error(function()
      checkout.locked_commit(lockfile, "example.nvim")
    end, "required plugin lockfile is missing or empty: " .. lockfile)

    write(lockfile, {})
    assert.has_error(function()
      checkout.locked_commit(lockfile, "example.nvim")
    end, "required plugin lockfile is missing or empty: " .. lockfile)

    write(lockfile, { "{" })
    assert.has_error(function()
      checkout.locked_commit(lockfile, "example.nvim")
    end, "required plugin lockfile is malformed JSON: " .. lockfile)

    write(lockfile, { '{"other":{"commit":"' .. expected_commit .. '"}}' })
    assert.has_error(function()
      checkout.locked_commit(lockfile, "example.nvim")
    end, "required plugin lock entry is missing: example.nvim")

    write(lockfile, { '{"example.nvim":{"commit":"abc"}}' })
    assert.has_error(function()
      checkout.locked_commit(lockfile, "example.nvim")
    end, "required plugin lock entry has an invalid 40-hex commit: example.nvim")

    write(lockfile, { '{"example.nvim":{"branch":"main","commit":"' .. expected_commit .. '"}}' })
    local commit, branch = checkout.locked_identity(lockfile, "example.nvim")
    assert.are.equal(expected_commit, commit)
    assert.are.equal("main", branch)

    write(lockfile, { '{"example.nvim":{"commit":"' .. expected_commit .. '"}}' })
    assert.has_error(function()
      checkout.locked_identity(lockfile, "example.nvim")
    end, "required plugin lock entry has an invalid branch: example.nvim")

    write(lockfile, { '{"example.nvim":{"branch":"../unsafe","commit":"' .. expected_commit .. '"}}' })
    assert.has_error(function()
      checkout.locked_identity(lockfile, "example.nvim")
    end, "required plugin lock entry has an invalid branch: example.nvim")
  end)

  it("creates and proves an absent cache before returning it", function()
    assert.are.equal(vim.fs.normalize(opts.target), checkout.ensure(opts))
    local valid, reason = checkout.verify(opts)
    assert.is_true(valid, reason)
    assert.are.equal(
      "refs/remotes/origin/" .. opts.branch,
      git("-C", opts.target, "symbolic-ref", "refs/remotes/origin/HEAD")
    )
    assert.are.equal(expected_commit, git("-C", opts.target, "rev-parse", "refs/remotes/origin/" .. opts.branch))
  end)

  it("checks out files beyond MAX_PATH inside a supported repository root", function()
    -- core.longpaths extends files, not repository discovery or the Windows CWD.
    local nested = string.rep("nested-managed-files/", 16) .. "proof.txt"
    write(remote .. "/" .. nested, { "locked long-file proof" })
    git("-c", "core.longpaths=true", "-C", remote, "add", nested)
    git("-c", "core.longpaths=true", "-C", remote, "commit", "--quiet", "-m", "long file fixture")
    opts.commit = git("-C", remote, "rev-parse", "HEAD")
    assert.is_true(#(opts.target .. "/" .. nested) > 320)

    -- The production helper bypasses all system/global config. Its explicit
    -- command setting must support these files without a fixture override.
    assert.are.equal(vim.fs.normalize(opts.target), checkout.ensure(opts))
    local valid, reason = checkout.verify(opts)
    assert.is_true(valid, reason)
    assert.are.equal("return true", vim.fn.readfile(opts.target .. "/lua/example/init.lua")[1])
    assert.are.equal("locked long-file proof", vim.fn.readfile(opts.target .. "/" .. nested)[1])
    assert.are.same({}, stages_for(opts.target))
  end)

  if vim.fn.has("win32") == 1 then
    it("refuses an unsupported native Git root without replacing the original target", function()
      -- Exact native failure: 254-char stage, 266-char .git/config. Discovery
      -- precedes config, so even a system longpaths setting cannot fix it.
      -- https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/compat/mingw.c
      local stage_suffix = ".stage." .. tostring(vim.fn.getpid())
      local target_suffix = "/cache/example.nvim"
      local padding = 254 - #stage_suffix - #root - 1 - #target_suffix
      assert.is_true(padding > 0 and padding < 255)
      opts.target = root .. "/" .. string.rep("g", padding) .. target_suffix
      assert.are.equal(254, #(opts.target .. stage_suffix))
      assert.are.equal(266, #(opts.target .. stage_suffix .. "/.git/config"))
      write(opts.target .. "/personal.txt", { "preserve original checkout data" })

      local ok, err = pcall(checkout.ensure, opts)

      assert.is_false(ok)
      assert.matches("git init failed", tostring(err), 1, true)
      assert.matches("Filename too long", tostring(err), 1, true)
      assert.are.same({ "personal.txt" }, vim.fn.readdir(opts.target))
      assert.are.equal("preserve original checkout data", vim.fn.readfile(opts.target .. "/personal.txt")[1])
      assert.are.same({}, stages_for(opts.target))
      assert.is_nil(vim.uv.fs_stat(opts.target .. ".lock"))
    end)
  end

  it("reuses a verified cache without init, fetch, or checkout", function()
    checkout.ensure(opts)
    local mutating = {}
    local observed = vim.tbl_extend("force", opts, {
      run_command = function(args)
        local command = table.concat(args, " ")
        if
          command:match(" init ")
          or command:match(" fetch ")
          or command:match(" checkout ")
          or command:match(" update%-ref ")
          or command:match(" symbolic%-ref refs/remotes/origin/HEAD refs/remotes/origin/")
        then
          table.insert(mutating, command)
        end
        return run(args)
      end,
    })
    checkout.ensure(observed)
    assert.are.same({}, mutating)
  end)

  it("repairs a clean cache at the wrong HEAD transactionally", function()
    checkout.ensure(opts)
    git("-C", opts.target, "config", "user.name", "Dotfiles Test")
    git("-C", opts.target, "config", "user.email", "dotfiles@example.invalid")
    git("-C", opts.target, "commit", "--quiet", "--allow-empty", "-m", "wrong head")
    assert.are_not.equal(expected_commit, git("-C", opts.target, "rev-parse", "HEAD"))
    checkout.ensure(opts)
    assert.are.equal(expected_commit, git("-C", opts.target, "rev-parse", "HEAD"))
  end)

  it("repairs dirty, non-Git, wrong-origin, and partial caches without executing them", function()
    local cases = {
      function()
        checkout.ensure(opts)
        write(opts.target .. "/lua/example/init.lua", { "return 'dirty'" })
      end,
      function()
        vim.fn.mkdir(opts.target, "p")
        write(opts.target .. "/lua/example/init.lua", { "return 'not git'" })
      end,
      function()
        checkout.ensure(opts)
        git("-C", opts.target, "remote", "set-url", "origin", "https://example.invalid/wrong.git")
      end,
      function()
        checkout.ensure(opts)
        vim.fn.delete(opts.target .. "/lua/example/init.lua")
      end,
    }

    for index, arrange in ipairs(cases) do
      vim.fn.delete(opts.target, "rf")
      arrange()
      local valid = checkout.verify(opts)
      assert.is_false(valid, "fixture " .. index .. " should be rejected before repair")
      checkout.ensure(opts)
      local repaired, reason = checkout.verify(opts)
      assert.is_true(repaired, reason)
    end
  end)

  it("cleans staging and lock state after a fetch failure", function()
    local failed = vim.tbl_extend("force", opts, {
      run_command = function(args)
        if table.concat(args, " "):match(" fetch ") then
          return { code = 42, stdout = "", stderr = "injected fetch failure" }
        end
        return run(args)
      end,
    })
    assert.has_error(function()
      checkout.ensure(failed)
    end)
    assert.is_false(vim.uv.fs_stat(opts.target .. ".lock") ~= nil)
    assert.are.same({}, stages_for(opts.target))
    assert.is_false(vim.uv.fs_stat(opts.target) ~= nil)
  end)

  it("cleans staging and lock state after a checkout failure", function()
    local failed = vim.tbl_extend("force", opts, {
      run_command = function(args)
        if table.concat(args, " "):match(" checkout ") then
          return { code = 43, stdout = "", stderr = "injected checkout failure" }
        end
        return run(args)
      end,
    })
    assert.has_error(function()
      checkout.ensure(failed)
    end)
    assert.is_false(vim.uv.fs_stat(opts.target .. ".lock") ~= nil)
    assert.are.same({}, stages_for(opts.target))
    assert.is_false(vim.uv.fs_stat(opts.target) ~= nil)
  end)

  it("waits for a concurrent first start and then reuses its verified publication", function()
    local ready = vim.tbl_extend("force", opts, { target = root .. "/ready/example.nvim" })
    checkout.ensure(ready)
    vim.fn.mkdir(opts.target .. ".lock", "p")
    vim.defer_fn(function()
      vim.uv.fs_rename(ready.target, opts.target)
      vim.fn.delete(opts.target .. ".lock", "rf")
    end, 50)

    checkout.ensure(opts)
    local valid, reason = checkout.verify(opts)
    assert.is_true(valid, reason)
  end)

  it("reclaims a lock whose recorded owner is gone and publishes the checkout", function()
    local lock = opts.target .. ".lock"
    vim.fn.mkdir(lock, "p")
    write(lock .. "/owner.pid", { "2147483647" })

    checkout.ensure(opts)

    local valid, reason = checkout.verify(opts)
    assert.is_true(valid, reason)
    assert.is_false(vim.uv.fs_stat(lock) ~= nil)
  end)

  it("does not reclaim a lock whose recorded owner is alive", function()
    local lock = opts.target .. ".lock"
    vim.fn.mkdir(lock, "p")
    write(lock .. "/owner.pid", { tostring(vim.fn.getpid()) })
    local waiting = vim.tbl_extend("force", opts, { lock_timeout_ms = 80 })

    local ok, err = pcall(checkout.ensure, waiting)

    assert.is_false(ok)
    assert.matches("timed out waiting for checkout lock", tostring(err), 1, true)
    assert.matches("Remove it only after confirming no Neovim process owns it", tostring(err), 1, true)
    assert.is_true(vim.uv.fs_stat(lock) ~= nil)
  end)

  it("keeps runtimepath and require after the production identity proof", function()
    local source = table.concat(vim.fn.readfile(_G.TEST_REPO_ROOT .. "/nvim/init.lua"), "\n")
    local lock_index = assert(source:find("pinned_checkout.locked_identity", 1, true))
    local ensure_index = assert(source:find("pinned_checkout.ensure", 1, true))
    local runtime_index = assert(source:find("vim.opt.rtp:prepend", 1, true))
    local require_index = assert(source:find('require("lazy").setup', 1, true))
    assert.is_true(lock_index < ensure_index)
    assert.is_true(ensure_index < runtime_index)
    assert.is_true(runtime_index < require_index)
  end)
end)
