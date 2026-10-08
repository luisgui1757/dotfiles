describe("managed Neovim runtime paths", function()
  local runtime = require("util.managed_runtime")
  local original_stdpath, original_root, original_rtp, temporary

  before_each(function()
    original_stdpath = vim.fn.stdpath
    original_rtp = vim.opt.rtp:get()
    original_root = vim.env.DOTFILES_NVIM_RUNTIME
    temporary = vim.fn.tempname()
    vim.fn.mkdir(temporary, "p")
    vim.fn.stdpath = function(name)
      assert.equals("data", name)
      return temporary
    end
    vim.env.DOTFILES_NVIM_RUNTIME = nil
  end)

  after_each(function()
    vim.fn.stdpath = original_stdpath
    vim.opt.rtp = original_rtp
    vim.env.DOTFILES_NVIM_RUNTIME = original_root
    vim.fn.delete(temporary, "rf")
  end)

  it("keeps the released runtime until the private generation is published", function()
    assert.is_nil(runtime.root())
    assert.equals(vim.fs.joinpath(temporary, "mason"), runtime.path("mason"))
    assert.same({}, vim.fn.readdir(temporary))
  end)

  it("finds the same published generation without a shell environment", function()
    local current = vim.fs.joinpath(temporary, "dotfiles-runtime", "nvim.sync", "current")
    vim.fn.mkdir(current, "p")
    assert.equals(current, runtime.root())
    assert.equals(vim.fs.joinpath(current, "lazy"), runtime.path("lazy"))
    assert.equals(vim.fs.joinpath(current, "mason/packages/neocmakelsp"), runtime.path("mason/packages/neocmakelsp"))
  end)

  it("uses the explicit unpublished generation only for installer commands", function()
    vim.env.DOTFILES_NVIM_RUNTIME = vim.fs.joinpath(temporary, "stage")
    assert.equals(vim.fs.joinpath(temporary, "stage", "site"), runtime.path("site"))
    assert.same({}, vim.fn.readdir(temporary))
  end)
  it("isolates managed parsers without deleting the old site", function()
    local legacy = vim.fs.joinpath(temporary, "site")
    vim.fn.mkdir(legacy, "p")
    vim.fn.writefile({ "personal parser" }, vim.fs.joinpath(legacy, "personal"))
    local system = vim.fs.joinpath(temporary, "system-site")
    vim.opt.rtp = { legacy, vim.fs.joinpath(legacy, "after"), system }
    vim.env.DOTFILES_NVIM_RUNTIME = vim.fs.joinpath(temporary, "stage")
    runtime.activate_site()
    assert.same({ runtime.path("site"), system }, vim.opt.rtp:get())
    assert.same({ "personal parser" }, vim.fn.readfile(vim.fs.joinpath(legacy, "personal")))
  end)
end)
