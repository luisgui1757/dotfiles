local repo = _G.TEST_REPO_ROOT
local checkout = require("util.pinned_git_checkout")
local mason = repo .. "/tests/.cache/mason.nvim"
checkout.ensure({
  target = mason,
  url = "https://github.com/mason-org/mason.nvim.git",
  commit = checkout.locked_commit(repo .. "/nvim/lazy-lock.json", "mason.nvim"),
  required_file = "lua/mason/init.lua",
})
vim.opt.rtp:prepend(mason)

describe("reviewed Mason registry", function()
  local root, prior_runtime, prior_config, prior_sync
  before_each(function()
    root = vim.fn.tempname()
    prior_runtime, prior_config, prior_sync =
      vim.env.DOTFILES_NVIM_RUNTIME, vim.env.DOTFILES_NVIM_CONFIG, vim.env.DOTFILES_NVIM_SYNC
    vim.env.DOTFILES_NVIM_RUNTIME = root
    vim.env.DOTFILES_NVIM_CONFIG = repo .. "/nvim"
    vim.env.DOTFILES_NVIM_SYNC = "1"
  end)
  after_each(function()
    vim.env.DOTFILES_NVIM_RUNTIME, vim.env.DOTFILES_NVIM_CONFIG, vim.env.DOTFILES_NVIM_SYNC =
      prior_runtime, prior_config, prior_sync
    vim.fn.delete(root, "rf")
  end)

  it("configures a fixed registry before loading package recipes", function()
    require("plugins.lsp-config")[1].config()
    local registries = require("mason.settings").current.registries
    assert.equals(1, #registries)
    assert.matches("^github:mason%-org/mason%-registry@%d%d%d%d%-%d%d%-%d%d%-", registries[1])
    local source = require("mason-registry").sources:get(1):get()
    assert.is_true(source:is_installed())
    local before = source:get_info()
    -- Exact locked upstream implementation: a fixed installed version must
    -- return without consulting GitHub, even with an expired refresh timestamp.
    before.download_timestamp = 0
    vim.fn.writefile({ vim.json.encode(before) }, source.root_dir .. "/info.json")
    local result
    require("mason-core.async").run(function()
      return source:install()
    end, function(ok, value)
      assert.is_true(ok, tostring(value))
      result = value
    end)
    assert.is_true(vim.wait(5000, function()
      return result ~= nil
    end))
    assert.is_true(result:is_success())
    assert.equals(0, source:get_info().download_timestamp)
  end)

  it("rejects changed registry bytes before loading them", function()
    local config = root .. "/config"
    vim.fn.mkdir(config .. "/mason-registry", "p")
    vim.fn.writefile(vim.fn.readfile(repo .. "/nvim/mason-registry/lock.json"), config .. "/mason-registry/lock.json")
    vim.fn.writefile({ "[]" }, config .. "/mason-registry/registry.json")
    vim.env.DOTFILES_NVIM_CONFIG = config
    assert.has_error(function()
      require("plugins.lsp-config")[1].config()
    end, "Mason registry differs from reviewed SHA-256")
    assert.is_nil(vim.uv.fs_stat(root .. "/mason"))
  end)

  it("verifies a published registry without replacing changed bytes", function()
    local registry = require("util.mason_registry")
    registry.prepare()
    local path = root .. "/mason/registries/github/mason-org/mason-registry/registry.json"
    vim.fn.writefile({ "[]" }, path)
    vim.env.DOTFILES_NVIM_SYNC = nil
    assert.has_error(function()
      registry.prepare()
    end, "installed Mason registry differs from reviewed SHA-256; run installer Repair")
    assert.same({ "[]" }, vim.fn.readfile(path))
  end)

  it("never creates a missing published registry during startup", function()
    vim.env.DOTFILES_NVIM_SYNC = nil
    assert.has_error(function()
      require("util.mason_registry").prepare()
    end)
    assert.is_nil(vim.uv.fs_stat(root))
  end)

  it("rejects a different tool version despite valid name and command links", function()
    local registry = require("util.mason_registry")
    registry.prepare()
    local lock = registry.snapshot()
    local package = root .. "/mason/packages/pyright"
    vim.fn.mkdir(package, "p")
    local receipt = {
      name = "pyright",
      schema_version = "2.0",
      source = { id = registry.sources({ "pyright" }).pyright, type = "registry+v1" },
      links = { bin = { pyright = "node_modules/.bin/pyright" } },
      registry = {
        proto = "github",
        namespace = "mason-org",
        name = "mason-registry",
        version = lock.version,
        checksums = { ["registry.json"] = lock.registry_sha256 },
      },
    }
    vim.fn.writefile({ vim.json.encode(receipt) }, package .. "/mason-receipt.json")
    registry.check_receipts(root .. "/mason", { "pyright" })
    receipt.source.id = "pkg:npm/pyright@1.0.0"
    vim.fn.writefile({ vim.json.encode(receipt) }, package .. "/mason-receipt.json")
    assert.has_error(function()
      registry.check_receipts(root .. "/mason", { "pyright" })
    end, "Mason installed source/version differs from lock: pyright")
  end)

  it("binds every selected tool to the snapshot's concrete source version", function()
    local registry = require("util.mason_registry")
    local names = require("util.mason_tools").ensure_installed()
    local sources = registry.sources(names)
    for _, name in ipairs(names) do
      local parsed = require("mason-core.purl").parse(sources[name]):get_or_throw()
      assert.is_truthy(parsed.version)
      assert.is_not.equals("latest", parsed.version)
    end
  end)

  for _, version in ipairs({ "latest", "nightly", "main", "1.x" }) do
    it("rejects the moving tool version " .. version .. " before provisioning", function()
      local registry = require("util.mason_registry")
      local lock = registry.snapshot()
      local data = vim.json.encode({ { name = "pyright", source = { id = "pkg:npm/pyright@" .. version } } })
      lock.registry_sha256 = vim.fn.sha256(data)
      local config = root .. "/config"
      vim.fn.mkdir(config .. "/mason-registry", "p")
      vim.fn.writefile({ vim.json.encode(lock) }, config .. "/mason-registry/lock.json")
      vim.fn.writefile({ data }, config .. "/mason-registry/registry.json", "b")
      assert.has_error(function()
        registry.sources({ "pyright" }, config)
      end, "selected Mason package lacks a concrete version: pyright")
    end)
  end
end)
