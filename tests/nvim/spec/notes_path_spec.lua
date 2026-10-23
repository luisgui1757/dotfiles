package.loaded["util.notes_path"] = nil
local resolve = require("util.notes_path").resolve

local function stub_env(map)
  return function(name)
    local v = map[name]
    if v == nil then
      return vim.NIL
    end
    return v
  end
end

describe("notes_path.resolve", function()
  it("NOTES_VAULT override wins on any OS", function()
    local p = resolve(function()
      return { sysname = "Darwin", release = "23.0.0" }
    end, stub_env({ NOTES_VAULT = "/some/custom/vault" }))
    assert.are.equal("/some/custom/vault", p)
  end)

  it("Darwin → iCloud Obsidian root (no user/vault name baked in)", function()
    local p = resolve(function()
      return { sysname = "Darwin", release = "23.0.0" }
    end, stub_env({}))
    assert.is_truthy(p:match("Library/Mobile Documents/iCloud~md~obsidian/Documents$"))
  end)

  it("Linux uses the same local default under a WSL kernel", function()
    local p = resolve(function()
      return { sysname = "Linux", release = "5.15.90.1-microsoft-standard-WSL2" }
    end, stub_env({ WINUSER = "alice" }))
    assert.are.equal(vim.fn.expand("~") .. "/notes", p)
  end)

  it("native Linux → ~/notes", function()
    local p = resolve(function()
      return { sysname = "Linux", release = "6.5.0-generic" }
    end, stub_env({}))
    assert.is_truthy(p:match("/notes$"))
    assert.is_nil(p:match("ZF"))
  end)

  it("Windows → generic USERPROFILE/Notes (no employer vault)", function()
    local p = resolve(function()
      return { sysname = "Windows_NT", release = "10.0" }
    end, stub_env({ USERPROFILE = "C:/Users/bob" }))
    assert.are.equal("C:/Users/bob/Notes", p)
    assert.is_nil(p:match("ZF"))
  end)

  it("never throws on missing env vars", function()
    assert.has_no.errors(function()
      -- Use a non-WSL Linux marker so we don't shell out to cmd.exe in CI.
      resolve(function()
        return { sysname = "Linux", release = "6.5.0" }
      end, stub_env({}))
    end)
    assert.has_no.errors(function()
      resolve(function()
        return { sysname = "Windows_NT", release = "10.0" }
      end, stub_env({}))
    end)
  end)
end)

describe("notes plugin spec", function()
  local original_system
  local original_getenv
  local original_uname
  local original_vault
  local temporary_vault

  before_each(function()
    package.loaded["plugins.notes"] = nil
    package.loaded["util.notes_path"] = nil
    original_system = vim.fn.system
    original_getenv = vim.fn.getenv
    original_uname = vim.uv.os_uname
    original_vault = vim.env.NOTES_VAULT
    temporary_vault = vim.fn.tempname()
  end)

  after_each(function()
    vim.fn.system = original_system
    vim.fn.getenv = original_getenv
    vim.uv.os_uname = original_uname
    vim.env.NOTES_VAULT = original_vault
    vim.fn.delete(temporary_vault, "rf")
    package.loaded["plugins.notes"] = nil
    package.loaded["util.notes_path"] = nil
  end)

  it("does not shell out during lazy spec import", function()
    local system_calls = 0
    vim.fn.system = function()
      system_calls = system_calls + 1
      return "alice\n"
    end
    vim.fn.getenv = function()
      return vim.NIL
    end
    vim.uv.os_uname = function()
      return { sysname = "Linux", release = "microsoft-standard-WSL2" }
    end

    assert.has_no.errors(function()
      require("plugins.notes")
    end)
    assert.are.equal(0, system_calls)
  end)

  it("provisions the locked plugin without activating or creating an absent vault", function()
    vim.env.NOTES_VAULT = temporary_vault
    local spec = require("plugins.notes")[1]
    assert.is_nil(spec.cond)
    assert.has_no.errors(spec.config)
    assert.are.equal(0, vim.fn.isdirectory(temporary_vault))
  end)

  it("keeps notes available for an existing explicitly selected vault", function()
    vim.fn.mkdir(temporary_vault, "p")
    vim.env.NOTES_VAULT = temporary_vault
    assert.is_nil(require("plugins.notes")[1].cond)
    assert.are.equal(temporary_vault, require("util.notes_path").resolve())
  end)
end)
