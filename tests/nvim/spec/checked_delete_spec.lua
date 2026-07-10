local checked_delete = require("util.checked_delete")

describe("checked managed deletion", function()
  local root

  before_each(function()
    root = vim.fn.tempname()
    vim.fn.mkdir(root, "p")
  end)

  after_each(function()
    vim.fn.delete(root, "rf")
  end)

  it("removes a managed file and verifies absence", function()
    local path = vim.fs.joinpath(root, "site", "parser", "lua.so")
    vim.fn.mkdir(vim.fs.dirname(path), "p")
    vim.fn.writefile({ "parser" }, path)

    local ok, err = checked_delete.managed(path, nil, root)

    assert.is_true(ok, tostring(err))
    assert.is_nil(vim.uv.fs_lstat(path))
  end)

  it("recursively removes a managed query directory", function()
    local path = vim.fs.joinpath(root, "site", "queries", "lua")
    vim.fn.mkdir(path, "p")
    vim.fn.writefile({ "query" }, vim.fs.joinpath(path, "highlights.scm"))

    local ok, err = checked_delete.managed(path, "rf", root)

    assert.is_true(ok, tostring(err))
    assert.is_nil(vim.uv.fs_lstat(path))
  end)

  it("reports a delete return-code failure and preserves the remaining path", function()
    local path = vim.fs.joinpath(root, "site", "parser-info", "lua.revision")
    vim.fn.mkdir(vim.fs.dirname(path), "p")
    vim.fn.writefile({ "revision" }, path)
    local original_delete = vim.fn.delete
    vim.fn.delete = function(candidate)
      if candidate == path then
        return -1
      end
      return original_delete(candidate)
    end

    local ok, err = checked_delete.managed(path, nil, root)
    vim.fn.delete = original_delete

    assert.is_false(ok)
    assert.matches("delete returned %-1", err)
    assert.is_not_nil(vim.uv.fs_lstat(path))
  end)

  it("detects a false-success partial recursive delete", function()
    local path = vim.fs.joinpath(root, "site", "queries", "cpp")
    vim.fn.mkdir(path, "p")
    vim.fn.writefile({ "query" }, vim.fs.joinpath(path, "highlights.scm"))
    local original_delete = vim.fn.delete
    vim.fn.delete = function(candidate)
      if candidate == path then
        return 0
      end
      return original_delete(candidate)
    end

    local ok, err = checked_delete.managed(path, "rf", root)
    vim.fn.delete = original_delete

    assert.is_false(ok)
    assert.matches("cleanup failed", err)
    assert.is_not_nil(vim.uv.fs_lstat(path))
  end)

  it("refuses a parser path outside the managed data root", function()
    local outside = vim.fn.tempname() .. ".so"
    vim.fn.writefile({ "built-in parser" }, outside)

    local ok, err = checked_delete.managed(outside, nil, root)

    assert.is_false(ok)
    assert.matches("outside stdpath", err)
    assert.is_not_nil(vim.uv.fs_lstat(outside))
    vim.fn.delete(outside)
  end)
end)
