local M = {}

local function trim(value)
  return (tostring(value or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function normalized_url(value)
  return trim(value):gsub("/+$", ""):gsub("%.git$", "")
end

local function git_environment()
  return vim.tbl_extend("force", vim.fn.environ(), {
    GIT_CONFIG_NOSYSTEM = "1",
    GIT_CONFIG_SYSTEM = vim.fn.has("win32") == 1 and "NUL" or "/dev/null",
    GIT_CONFIG_GLOBAL = vim.fn.has("win32") == 1 and "NUL" or "/dev/null",
    GIT_CONFIG_COUNT = "0",
    GIT_CONFIG_PARAMETERS = "",
    GIT_TEMPLATE_DIR = "",
    GIT_TERMINAL_PROMPT = "0",
  })
end

local function default_run(args)
  local result = vim
    .system(args, {
      text = true,
      env = git_environment(),
    })
    :wait()
  return {
    code = result.code,
    stdout = result.stdout or "",
    stderr = result.stderr or "",
  }
end

local function run(opts, args)
  local runner = opts.run_command or default_run
  local ok, result = pcall(runner, args, opts)
  if not ok then
    return { code = 127, stdout = "", stderr = tostring(result) }
  end
  if type(result) ~= "table" or type(result.code) ~= "number" then
    return { code = 127, stdout = "", stderr = "command runner returned an invalid result" }
  end
  result.stdout = result.stdout or ""
  result.stderr = result.stderr or ""
  return result
end

local function git_args(...)
  local args = {
    "git",
    "-c",
    "core.fsmonitor=false",
    "-c",
    "core.untrackedCache=false",
    "-c",
    "core.hooksPath=/dev/null",
    "-c",
    "init.templateDir=",
  }
  vim.list_extend(args, { ... })
  return args
end

local function command_failure(label, result)
  local detail = trim(result.stderr)
  if detail == "" then
    detail = trim(result.stdout)
  end
  if detail == "" then
    detail = "exit " .. tostring(result.code)
  end
  error(label .. " failed: " .. detail, 0)
end

local function run_checked(opts, label, args)
  local result = run(opts, args)
  if result.code ~= 0 then
    command_failure(label, result)
  end
  return trim(result.stdout)
end

local function path_exists(path)
  return vim.uv.fs_stat(path) ~= nil
end

local function canonical_path(path)
  return vim.fs.normalize(vim.uv.fs_realpath(path) or path)
end

local function checked_delete(path)
  if not path_exists(path) then
    return true
  end
  local rc = vim.fn.delete(path, "rf")
  return rc == 0 and not path_exists(path)
end

local function unique_sibling(path, suffix)
  local candidate = path .. suffix
  local index = 0
  while path_exists(candidate) do
    index = index + 1
    candidate = path .. suffix .. "." .. index
  end
  return candidate
end

local function valid_branch_name(branch)
  return type(branch) == "string"
    and branch ~= ""
    and branch:match("^[%w%._/-]+$") ~= nil
    and not branch:match("^[-/.]")
    and not branch:match("[/.]$")
    and not branch:find("..", 1, true)
    and not branch:find("//", 1, true)
    and not branch:find("@{", 1, true)
    and not branch:match("%.lock$")
end

function M.locked_identity(lockfile, name)
  local ok, lines = pcall(vim.fn.readfile, lockfile)
  if not ok or type(lines) ~= "table" or #lines == 0 then
    error("required plugin lockfile is missing or empty: " .. lockfile, 0)
  end

  local decoded_ok, lock = pcall(vim.json.decode, table.concat(lines, "\n"))
  if not decoded_ok or type(lock) ~= "table" then
    error("required plugin lockfile is malformed JSON: " .. lockfile, 0)
  end
  if type(lock[name]) ~= "table" then
    error("required plugin lock entry is missing: " .. name, 0)
  end

  local commit = lock[name].commit
  if type(commit) ~= "string" or not commit:match("^[0-9a-fA-F]+$") or #commit ~= 40 then
    error("required plugin lock entry has an invalid 40-hex commit: " .. name, 0)
  end
  local branch = lock[name].branch
  if not valid_branch_name(branch) then
    error("required plugin lock entry has an invalid branch: " .. name, 0)
  end
  return commit:lower(), branch
end

function M.locked_commit(lockfile, name)
  local commit = M.locked_identity(lockfile, name)
  return commit
end

function M.verify(opts)
  local target = vim.fs.normalize(assert(opts.target, "target is required"))
  local expected_url = assert(opts.url, "url is required")
  local expected_commit = assert(opts.commit, "commit is required"):lower()
  local required_file = assert(opts.required_file, "required_file is required")

  local stat = vim.uv.fs_stat(target)
  if not stat or stat.type ~= "directory" then
    return false, "checkout directory is absent"
  end
  if not path_exists(vim.fs.joinpath(target, ".git")) then
    return false, "checkout is not a Git repository"
  end
  if not path_exists(vim.fs.joinpath(target, required_file)) then
    return false, "required checkout file is missing: " .. required_file
  end

  local function probe(label, ...)
    local result = run(opts, git_args("-C", target, ...))
    if result.code ~= 0 then
      return nil, label .. " failed: " .. trim(result.stderr)
    end
    return trim(result.stdout)
  end

  local inside, reason = probe("worktree probe", "rev-parse", "--is-inside-work-tree")
  if inside ~= "true" then
    return false, reason or "checkout is not a usable worktree"
  end
  local top
  top, reason = probe("worktree-root probe", "rev-parse", "--show-toplevel")
  if not top or canonical_path(top) ~= canonical_path(target) then
    return false, reason or "checkout worktree root does not match its cache path"
  end
  local origin
  origin, reason = probe("origin probe", "remote", "get-url", "origin")
  if not origin or normalized_url(origin) ~= normalized_url(expected_url) then
    return false, reason or ("checkout origin mismatch: " .. tostring(origin))
  end
  local head
  head, reason = probe("HEAD probe", "rev-parse", "HEAD")
  if not head or head:lower() ~= expected_commit then
    return false, reason or ("checkout HEAD mismatch: " .. tostring(head))
  end
  if opts.branch then
    local remote_head
    remote_head, reason = probe("origin HEAD probe", "symbolic-ref", "refs/remotes/origin/HEAD")
    local expected_ref = "refs/remotes/origin/" .. opts.branch
    if not remote_head or remote_head ~= expected_ref then
      return false, reason or ("checkout origin HEAD mismatch: " .. tostring(remote_head))
    end
    local remote_commit
    remote_commit, reason = probe("origin branch probe", "rev-parse", "--verify", expected_ref .. "^{commit}")
    if not remote_commit or remote_commit:lower() ~= expected_commit then
      return false, reason or ("checkout origin branch mismatch: " .. tostring(remote_commit))
    end
  end
  local clean
  clean, reason = probe("cleanliness probe", "status", "--porcelain=v1", "--untracked-files=all")
  if clean == nil then
    return false, reason
  end
  if clean ~= "" then
    return false, "checkout is dirty"
  end
  return true
end

local function acquire_lock(opts)
  local lock = opts.target .. ".lock"
  local timeout_ms = opts.lock_timeout_ms or 30000
  local started = vim.uv.hrtime()
  while true do
    local made, err, code = vim.uv.fs_mkdir(lock, 448)
    if made then
      return lock
    end
    if code ~= "EEXIST" then
      error("could not create checkout lock " .. lock .. ": " .. tostring(err), 0)
    end

    local valid = M.verify(opts)
    if valid then
      return nil
    end
    local elapsed_ms = (vim.uv.hrtime() - started) / 1000000
    if elapsed_ms >= timeout_ms then
      error("timed out waiting for checkout lock: " .. lock, 0)
    end
    vim.wait(50)
  end
end

local function rename_or_error(source, destination, label)
  local ok, err = vim.uv.fs_rename(source, destination)
  if not ok then
    error(label .. " failed: " .. tostring(err), 0)
  end
end

function M.ensure(opts)
  assert(type(opts) == "table", "options are required")
  opts.target = vim.fs.normalize(assert(opts.target, "target is required"))
  opts.url = assert(opts.url, "url is required")
  opts.commit = assert(opts.commit, "commit is required"):lower()
  opts.required_file = assert(opts.required_file, "required_file is required")
  if not opts.commit:match("^[0-9a-f]+$") or #opts.commit ~= 40 then
    error("checkout commit must be a full 40-hex identity", 0)
  end
  if opts.branch ~= nil and not valid_branch_name(opts.branch) then
    error("checkout branch must be a valid locked branch name", 0)
  end

  local valid = M.verify(opts)
  if valid then
    return opts.target
  end

  vim.fn.mkdir(vim.fn.fnamemodify(opts.target, ":h"), "p")
  local lock = acquire_lock(opts)
  if not lock then
    return opts.target
  end

  local stage = unique_sibling(opts.target, ".stage." .. tostring(vim.fn.getpid()))
  local previous
  local cleanup_errors = {}
  local ok, result = xpcall(function()
    local became_valid = M.verify(opts)
    if became_valid then
      return opts.target
    end

    if not checked_delete(stage) then
      error("could not clean stale checkout staging directory: " .. stage, 0)
    end
    vim.fn.mkdir(stage, "p")
    run_checked(opts, "git init", git_args("-C", stage, "init", "--quiet"))
    run_checked(opts, "git remote add", git_args("-C", stage, "remote", "add", "origin", opts.url))
    run_checked(
      opts,
      "git fetch locked commit",
      git_args("-C", stage, "fetch", "--depth", "1", "--filter=blob:none", "origin", opts.commit)
    )
    run_checked(opts, "git checkout locked commit", git_args("-C", stage, "checkout", "--detach", opts.commit))
    if opts.branch then
      local remote_ref = "refs/remotes/origin/" .. opts.branch
      run_checked(opts, "git record locked origin branch", git_args("-C", stage, "update-ref", remote_ref, opts.commit))
      run_checked(
        opts,
        "git record locked origin HEAD",
        git_args("-C", stage, "symbolic-ref", "refs/remotes/origin/HEAD", remote_ref)
      )
    end

    local staged_valid, staged_reason = M.verify(vim.tbl_extend("force", opts, { target = stage }))
    if not staged_valid then
      error("staged checkout verification failed: " .. tostring(staged_reason), 0)
    end

    if path_exists(opts.target) then
      previous = unique_sibling(opts.target, ".previous")
      rename_or_error(opts.target, previous, "preserving previous checkout")
    end
    local published, publish_err = vim.uv.fs_rename(stage, opts.target)
    if not published then
      if previous and path_exists(previous) and not path_exists(opts.target) then
        vim.uv.fs_rename(previous, opts.target)
      end
      error("atomic checkout publication failed: " .. tostring(publish_err), 0)
    end

    local published_valid, published_reason = M.verify(opts)
    if not published_valid then
      local bad = unique_sibling(opts.target, ".failed-publication")
      vim.uv.fs_rename(opts.target, bad)
      if previous and path_exists(previous) then
        vim.uv.fs_rename(previous, opts.target)
      end
      error("published checkout verification failed: " .. tostring(published_reason), 0)
    end

    if previous and not checked_delete(previous) then
      error("verified replacement published, but previous checkout cleanup failed: " .. previous, 0)
    end
    previous = nil
    return opts.target
  end, debug.traceback)

  if path_exists(stage) and not checked_delete(stage) then
    table.insert(cleanup_errors, "staging cleanup failed: " .. stage)
  end
  if path_exists(lock) and not checked_delete(lock) then
    table.insert(cleanup_errors, "lock cleanup failed: " .. lock)
  end

  if not ok then
    local message = tostring(result)
    if #cleanup_errors > 0 then
      message = message .. "\n" .. table.concat(cleanup_errors, "\n")
    end
    error(message, 0)
  end
  if #cleanup_errors > 0 then
    error(table.concat(cleanup_errors, "\n"), 0)
  end
  return result
end

return M
