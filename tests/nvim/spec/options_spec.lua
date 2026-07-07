describe("vim-options", function()
  it("sets cross-platform clipboard to unnamedplus", function()
    assert.is_true(vim.tbl_contains(vim.opt.clipboard:get(), "unnamedplus"))
  end)

  it("enables termguicolors", function()
    assert.is_true(vim.opt.termguicolors:get())
  end)

  it("forces signcolumn = yes", function()
    assert.are.equal("yes", vim.opt.signcolumn:get())
  end)

  it("enables undofile", function()
    assert.is_true(vim.opt.undofile:get())
  end)

  it("has snappy updatetime + timeoutlen", function()
    assert.is_true(vim.opt.updatetime:get() <= 250)
    assert.is_true(vim.opt.timeoutlen:get() <= 400)
  end)

  it("listchars use tab/trail/nbsp (NOT space=·)", function()
    local lc = vim.opt.listchars:get()
    assert.is_not_nil(lc.tab)
    assert.is_not_nil(lc.trail)
    assert.is_not_nil(lc.nbsp)
    assert.is_nil(lc.space) -- regression guard
  end)

  it("smartcase + ignorecase both enabled", function()
    assert.is_true(vim.opt.ignorecase:get())
    assert.is_true(vim.opt.smartcase:get())
  end)

  it("keeps scrolloff at 16 (generous vertical context)", function()
    assert.are.equal(16, vim.opt.scrolloff:get())
  end)

  it("mapleader is space", function()
    assert.are.equal(" ", vim.g.mapleader)
  end)

  it(":EditQuery is removed (it clashed with :E -> :Explore)", function()
    local cmds = vim.api.nvim_get_commands({})
    assert.is_nil(cmds.EditQuery, ":EditQuery user command should be deleted so :E<Enter> reaches netrw")
  end)

  it(":E user command exists", function()
    local cmds = vim.api.nvim_get_commands({})
    assert.is_not_nil(cmds.E, ":E user command is missing")
  end)
end)
