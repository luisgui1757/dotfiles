local repo_root = vim.fn.getcwd()
local projects = dofile(repo_root .. "/tests/nvim/lsp_smoke_projects.lua")

describe("strict LSP smoke project isolation", function()
  local root

  before_each(function()
    root = vim.fn.tempname() .. " lsp projects"
    assert.are.equal(1, vim.fn.mkdir(root .. "/fixtures", "p"))
    assert.are.equal(0, vim.fn.writefile({ "project(sample)" }, root .. "/fixtures/CMakeLists.txt"))
    assert.are.equal(0, vim.fn.writefile({ "unrelated" }, root .. "/fixtures/sample.py"))
  end)

  after_each(function()
    assert.are.equal(0, vim.fn.delete(root, "rf"))
    assert.are.equal(0, vim.fn.isdirectory(root))
  end)

  it("copies only the selected fixture into a dedicated project root", function()
    local target, project_root = projects.prepare({
      root = root .. "/output",
      fixtures = root .. "/fixtures",
      fixture = "CMakeLists.txt",
      lsp = "neocmake",
      index = 6,
    })

    assert.are.equal(project_root .. "/CMakeLists.txt", target)
    assert.are.same({ "project(sample)" }, vim.fn.readfile(target))
    assert.are.equal(1, vim.fn.isdirectory(project_root .. "/.git"))
    assert.are.equal(0, vim.fn.filereadable(project_root .. "/sample.py"))
  end)

  it("uses distinct roots for consecutive probes of the same server", function()
    local _, first = projects.prepare({
      root = root .. "/output",
      fixtures = root .. "/fixtures",
      fixture = "CMakeLists.txt",
      lsp = "neocmake",
      index = 1,
    })
    local _, second = projects.prepare({
      root = root .. "/output",
      fixtures = root .. "/fixtures",
      fixture = "CMakeLists.txt",
      lsp = "neocmake",
      index = 2,
    })

    assert.are_not.equal(first, second)
  end)
end)
