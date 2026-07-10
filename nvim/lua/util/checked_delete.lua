local M = {}

local function path_exists(path)
  return vim.uv.fs_lstat(path) ~= nil
end

function M.managed(path, mode, data_root)
  local root = vim.fs.normalize(data_root or vim.fn.stdpath("data")):gsub("/$", "")
  local normalized = vim.fs.normalize(path)
  if normalized == root or not vim.startswith(normalized, root .. "/") then
    return false, "refusing cleanup outside stdpath('data'): " .. normalized
  end
  if not path_exists(path) then
    return true
  end

  local call_ok, rc
  if mode then
    call_ok, rc = pcall(vim.fn.delete, path, mode)
  else
    call_ok, rc = pcall(vim.fn.delete, path)
  end
  if not call_ok then
    return false, "cleanup raised for " .. normalized .. ": " .. tostring(rc)
  end
  if rc ~= 0 or path_exists(path) then
    return false,
      "cleanup failed for "
        .. normalized
        .. " (delete returned "
        .. tostring(rc)
        .. "); remove it manually and rerun setup"
  end
  return true
end

return M
