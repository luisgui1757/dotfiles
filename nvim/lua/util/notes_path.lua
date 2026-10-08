-- Cross-platform notes/vault path resolver.
-- uname_fn and env_fn are dependency-injected so tests can stub them.
local M = {}

local function is_set(v)
  return v ~= nil and v ~= vim.NIL and v ~= ""
end

-- Resolve the notes/vault directory. Personal vault locations are NOT
-- hardcoded here (this repo is public + shared). Set NOTES_VAULT in a
-- gitignored local rc (e.g. ~/.zshrc.local) to point at your real vault;
-- everything below is a generic, user-agnostic fallback.
function M.resolve(uname_fn, env_fn)
  uname_fn = uname_fn or vim.uv.os_uname
  env_fn = env_fn or vim.fn.getenv
  local home = vim.fn.expand("~")
  local uname = uname_fn() or {}
  local sysname = uname.sysname or ""

  -- Explicit override always wins.
  local override = env_fn("NOTES_VAULT")
  if is_set(override) then
    return override
  end

  if sysname == "Darwin" then
    -- Obsidian's default iCloud container root (no vault name baked in).
    return home .. "/Library/Mobile Documents/iCloud~md~obsidian/Documents"
  end

  if sysname:match("Windows") or sysname:match("MINGW") then
    local userprofile = env_fn("USERPROFILE")
    if not is_set(userprofile) then
      userprofile = home
    end
    return userprofile .. "/Notes"
  end

  -- All Linux environments share the same local default.
  return home .. "/notes"
end

return M
