local repo_root = _G.TEST_REPO_ROOT
local fh = assert(io.open(repo_root .. "/nvim/lua/plugins/treesitter.lua", "r"))
local src = fh:read("*a")
fh:close()

local function extract_string_list(name)
  local body = assert(src:match("local%s+" .. name .. "%s*=%s*%{(.-)%}\n"), "missing " .. name)
  local values = {}
  for value in body:gmatch('"([^"]+)"') do
    table.insert(values, value)
  end
  return values
end

local function assert_same_list(actual, expected)
  assert.are.equal(#expected, #actual)
  for i, value in ipairs(expected) do
    assert.are.equal(value, actual[i])
  end
end

describe("treesitter main migration", function()
  local required = {
    "c",
    "cpp",
    "cmake",
    "lua",
    "python",
    "rust",
    "bash",
    "powershell",
    "json",
    "jsonc",
    "yaml",
    "toml",
    "markdown",
    "markdown_inline",
    "vim",
    "vimdoc",
    "query",
    "diff",
    "gitcommit",
  }

  it("pins nvim-treesitter to the main branch", function()
    assert.is_truthy(src:match('branch%s*=%s*"main"'), "nvim-treesitter must use the 0.12+ main branch")
  end)

  it("keeps the required parser list unchanged", function()
    assert_same_list(extract_string_list("treesitter_parsers"), required)
  end)

  it("installs parsers with the main API", function()
    assert.is_truthy(
      src:match('require%("nvim%-treesitter"%)') and src:match("nvim_treesitter%.install%(treesitter_parsers%)"),
      "main branch must use require('nvim-treesitter').install(...)"
    )
  end)

  it("guards parser compilation on the tree-sitter CLI being on PATH", function()
    -- main shells out to `tree-sitter` per parser; without this guard a missing
    -- CLI dumps one ENOENT error per parser instead of one actionable message.
    assert.is_truthy(
      src:match('vim%.fn%.executable%("tree%-sitter"%)'),
      "must check for the tree-sitter CLI before compiling parsers"
    )
    assert.is_truthy(
      src:match("vim%.notify"),
      "a missing tree-sitter CLI must surface one actionable warning, not N ENOENT errors"
    )
  end)

  it("does not use the legacy configs.setup API", function()
    assert.is_nil(src:match("nvim%-treesitter%.configs"), "legacy nvim-treesitter.configs module must be gone")
    assert.is_nil(src:match("ensure_installed"), "legacy ensure_installed must be gone")
    assert.is_nil(src:match("highlight%s*=%s*%{"), "legacy highlight module must be gone")
    assert.is_nil(src:match("indent%s*=%s*%{"), "legacy indent module must be gone")
  end)

  it("registers highlighting before FileType fires on real file opens", function()
    assert.is_truthy(src:match('event%s*=%s*%{ "BufReadPre", "BufNewFile" %}'), "must load before FileType")
    assert.is_truthy(src:match('nvim_create_autocmd%("FileType"'), "must create a FileType autocmd")
    assert.is_truthy(src:match("pattern%s*=%s*treesitter_filetypes"), "FileType autocmd must be scoped")
    assert.is_truthy(src:match("pcall%(vim%.treesitter%.start, args%.buf%)"), "missing parsers must not error")
  end)

  it("registers filetype aliases for parsers whose names differ from Neovim filetypes", function()
    assert.is_truthy(src:match('bash%s*=%s*%{ "sh" %}'), "sh files should use the bash parser")
    assert.is_truthy(src:match('powershell%s*=%s*%{ "ps1" %}'), "ps1 files should use the powershell parser")
    assert.is_truthy(src:match('vimdoc%s*=%s*%{ "help" %}'), "help buffers should use the vimdoc parser")
    assert.is_truthy(
      src:match("vim%.treesitter%.language%.register%(parser, filetypes%)"),
      "aliases must be registered"
    )
  end)

  it("uses the main-branch indentation replacement", function()
    assert.is_truthy(
      src:match([[vim%.bo%[args%.buf%]%.indentexpr%s*=%s*"v:lua%.require'nvim%-treesitter'%.indentexpr%(%)"]]),
      "legacy indent.enable should be replaced with nvim-treesitter main indentexpr"
    )
  end)

  it("keeps the documented main-branch TSUpdate build and command triggers", function()
    assert.is_truthy(src:match('build%s*=%s*":TSUpdate"'), "upstream main docs still recommend TSUpdate as build")
    for _, command in ipairs({ "TSInstall", "TSInstallFromGrammar", "TSUpdate", "TSUninstall", "TSLog" }) do
      assert.is_truthy(src:match('"' .. command .. '"'), command .. " command trigger missing")
    end
  end)

  it("does not force a specific compiler", function()
    assert.is_nil(src:match("install%.compilers"), "don't pin compilers; auto-detection is cross-platform")
  end)

  it("wraps nvim_del_user_command in pcall", function()
    assert.is_truthy(
      src:match("pcall%(vim%.api%.nvim_del_user_command"),
      "unprotected nvim_del_user_command throws on fresh installs"
    )
  end)
end)
