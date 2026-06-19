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
  -- nvim-treesitter installs ONLY the languages Neovim does NOT bundle. The
  -- bundled set (c, lua, markdown, markdown_inline, query, vim, vimdoc) is
  -- deliberately excluded so Neovim's matched built-in parser+query is used --
  -- installing nvim-treesitter's would override the built-in and break the
  -- bundled query (e.g. lua highlights `operator:` -> "Invalid field name").
  local required = {
    "cpp",
    "cmake",
    "python",
    "rust",
    "bash",
    "powershell",
    "json",
    "yaml",
    "toml",
    "diff",
    "gitcommit",
  }

  it("pins nvim-treesitter to the main branch", function()
    assert.is_truthy(src:match('branch%s*=%s*"main"'), "nvim-treesitter must use the 0.12+ main branch")
  end)

  it("keeps the required parser list unchanged", function()
    assert_same_list(extract_string_list("treesitter_parsers"), required)
  end)

  it("excludes Neovim-bundled languages from the install list (canonical fix)", function()
    local installed = {}
    for _, p in ipairs(extract_string_list("treesitter_parsers")) do
      installed[p] = true
    end
    -- Languages Neovim 0.12 bundles a matched parser+query for. Installing
    -- nvim-treesitter's parser overrides the built-in and breaks the bundled
    -- query (e.g. lua highlights `operator:` -> E5113 "Invalid field name").
    for _, bundled in ipairs({ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" }) do
      assert.is_nil(installed[bundled], bundled .. " is Neovim-bundled; it must NOT be in treesitter_parsers")
    end
    -- c and vim are bundled but Neovim does not auto-start them, so we do.
    assert.is_truthy(
      src:match('nvim_bundled_started_here%s*=%s*%{ "c", "vim" %}'),
      "c and vim must be started via Neovim's matched built-in parser"
    )
  end)

  it("purges stale nvim-treesitter parser overrides for the bundled languages", function()
    -- Excluding the bundled langs stops future installs, but a parser/<lang>.so
    -- already on the runtimepath (older config, or a restored CI cache) still
    -- overrides the built-in and re-creates the E5113 mismatch. The config must
    -- actively delete those on load.
    assert.is_truthy(
      src:match('nvim_bundled_parsers%s*=%s*%{ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" %}'),
      "must enumerate the bundled parser (.so) set to purge"
    )
    assert.is_truthy(
      src:match('purge%("parser/%*%.so"%)'),
      "must scan the runtimepath for stale bundled parser overrides"
    )
    assert.is_truthy(src:match("vim%.fn%.delete"), "must delete stale bundled parser overrides")
    -- CRITICAL safety: deletes MUST be scoped to stdpath('data'). Neovim ships
    -- its own bundled parsers as .so files under the install prefix, on the
    -- runtimepath -- an unscoped purge would wipe Neovim's built-in parsers.
    assert.is_truthy(
      src:match("vim%.startswith") and src:match('vim%.fn%.stdpath%("data"%)'),
      "purge must be scoped to stdpath('data') so Neovim's built-in parsers are never deleted"
    )
  end)

  it("still covers the auto-started bundled filetypes so they keep indentexpr (no regression)", function()
    -- Excluding the bundled langs from the install list must NOT drop the
    -- FileType autocmd coverage that gives lua/markdown/help/query
    -- nvim-treesitter's indentexpr (the pre-fix behavior). The autocmd fires for
    -- treesitter_filetypes, which must include these via
    -- nvim_bundled_autostarted_filetypes.
    assert.is_truthy(
      src:match('nvim_bundled_autostarted_filetypes%s*=%s*%{ "lua", "markdown", "help", "query" %}'),
      "auto-started bundled filetypes must still get indentexpr"
    )
    assert.is_truthy(
      src:match("for _, filetype in ipairs%(nvim_bundled_autostarted_filetypes%)"),
      "auto-started bundled filetypes must be appended to treesitter_filetypes"
    )
  end)

  it("installs parsers with the main API", function()
    assert.is_truthy(
      src:match('require%("nvim%-treesitter"%)') and src:match('type%(nvim_treesitter%.install%)%s*==%s*"function"'),
      "main branch must use require('nvim-treesitter').install(...)"
    )
    assert.is_truthy(
      src:match("pcall%(nvim_treesitter%.install, treesitter_parsers%)"),
      "parser auto-install must not abort highlighting setup when the install API drifts"
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
    -- No vimdoc={help} alias: vimdoc is Neovim-bundled and Neovim already maps
    -- help -> vimdoc and auto-starts it, so the repo must NOT register it.
    assert.is_nil(src:match('vimdoc%s*=%s*%{ "help" %}'), "vimdoc is nvim-bundled; do not alias help to it")
    assert.is_truthy(
      src:match("vim%.treesitter%.language%.register%(parser, filetypes%)"),
      "aliases must be registered"
    )
  end)

  it("uses the main-branch indentation replacement", function()
    assert.is_truthy(
      src:match('type%(nvim_treesitter%.indentexpr%)%s*==%s*"function"'),
      "main indentexpr must be gated so stale plugin caches do not break buffer highlighting"
    )
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
