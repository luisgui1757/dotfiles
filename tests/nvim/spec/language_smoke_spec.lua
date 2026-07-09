-- Per-language smoke test, Tier 1 (fast; runs in `make test-nvim`).
--
-- Source of truth: tests/nvim/language_matrix.lua + tests/nvim/fixtures/.
-- For each fixture this asserts:
--   * Neovim detects the expected filetype,
--   * conform's formatters_by_ft matches the matrix,
--   * the matrix's parser is one the repo actually installs (treesitter_parsers).
--
-- The Tier-2 production smoke (needs Mason servers + runtimes) is a separate
-- script gated on DOTFILES_LSP_SMOKE and only runs in the e2e jobs; it covers
-- LSP attach plus formatter/LSP compatibility against real tools.
local repo_root = _G.TEST_REPO_ROOT
local matrix = dofile(repo_root .. "/tests/nvim/language_matrix.lua")
local fixtures = repo_root .. "/tests/nvim/fixtures/"

-- The parser list the production config installs (source of truth in
-- treesitter.lua; this is the same list treesitter_spec mirrors).
local function installed_parsers()
  local fh = assert(io.open(repo_root .. "/nvim/lua/plugins/treesitter.lua", "r"))
  local src = fh:read("*a")
  fh:close()
  local body = assert(src:match("local%s+treesitter_parsers%s*=%s*%{(.-)%}\n"), "treesitter_parsers block not found")
  local set = {}
  for p in body:gmatch('"([^"]+)"') do
    set[p] = true
  end
  return set
end

local function lsp_smoke_source()
  local fh = assert(io.open(repo_root .. "/tests/nvim/lsp_smoke.lua", "r"))
  local src = fh:read("*a")
  fh:close()
  return src
end

describe("language smoke (Tier 1)", function()
  local parsers = installed_parsers()
  local conform_fts = require("plugins.conform").opts.formatters_by_ft

  after_each(function()
    vim.cmd("silent! %bwipeout!")
  end)

  for _, row in ipairs(matrix) do
    it("detects filetype " .. row.filetype .. " for " .. row.fixture, function()
      vim.cmd.edit(vim.fn.fnameescape(fixtures .. row.fixture))
      assert.are.equal(row.filetype, vim.bo.filetype, row.fixture .. " filetype mismatch")
    end)

    it("matches conform formatters for " .. row.fixture, function()
      local got = conform_fts[row.filetype] or {}
      assert.are.same(row.formatters, got, "formatters_by_ft[" .. row.filetype .. "] drifted from the matrix")
    end)

    if row.parser then
      if row.bundled then
        it("does NOT install the Neovim-bundled " .. row.parser .. " parser for " .. row.fixture, function()
          -- Neovim bundles a matched parser+query for this language. Installing
          -- nvim-treesitter's would override the built-in and break the bundled
          -- query (e.g. lua highlights `operator:` -> E5113); Neovim's built-in
          -- handles it, so it must stay OUT of treesitter_parsers.
          assert.is_nil(parsers[row.parser], row.parser .. " is Neovim-bundled; it must NOT be in treesitter_parsers")
        end)
      else
        it("installs the " .. row.parser .. " parser for " .. row.fixture, function()
          assert.is_truthy(parsers[row.parser], row.parser .. " is in the matrix but not in treesitter_parsers")
        end)
      end
    end
  end

  it("Tier 2 proves declared parsers are actually installed before capture checks", function()
    local src = lsp_smoke_source()
    assert.is_truthy(
      src:find("cfg.norm_languages(explicit_parsers, { unsupported = true })", 1, true),
      "strict smoke must audit nvim-treesitter's normalized parser dependency list"
    )
    assert.is_truthy(
      src:find('nts.get_installed("parsers")', 1, true),
      "strict smoke must inspect nvim-treesitter's installed parser output"
    )
    assert.is_truthy(
      src:find("expected nvim-treesitter parser install output missing", 1, true),
      "strict smoke must fail causally when parser bootstrap is incomplete"
    )
    assert.is_truthy(
      src:find("expected nvim-treesitter query install output missing", 1, true),
      "strict smoke must fail causally when parser queries are incomplete"
    )
  end)

  it("Tier 2 uses one platform-aware LSP attach timeout helper", function()
    local src = lsp_smoke_source()
    assert.is_truthy(
      src:find('local lsp_attach_timeout_ms = vim.fn.has("win32") == 1 and 90000 or 45000', 1, true),
      "Windows CI needs a longer cold-start LSP attach budget than Unix"
    )
    assert.is_truthy(src:find("local function wait_for_lsp_client", 1, true), "shared LSP wait helper missing")
    assert.is_nil(src:find("vim.wait(45000", 1, true), "raw attach timeout must not be duplicated")
  end)

  it("Tier 2 starts and parses the expected parser before capture probes", function()
    local src = lsp_smoke_source()
    assert.is_truthy(
      src:find("local function wait_for_treesitter_capture", 1, true),
      "shared Tree-sitter capture wait helper missing"
    )
    assert.is_truthy(
      src:find("pcall(vim.treesitter.start, buf, parser)", 1, true),
      "Tier 2 must explicitly start the expected parser before checking captures"
    )
    assert.is_truthy(
      src:find("syntax_before_start", 1, true),
      "explicit Tree-sitter start must preserve production-restored syntax fallback"
    )
    assert.is_truthy(
      src:find("parser_obj:parse()", 1, true),
      "Tier 2 must explicitly parse the buffer before checking captures"
    )
    assert.is_truthy(
      src:find("wait_for_treesitter_capture(0, row.parser)", 1, true),
      "matrix runtime gate must use the shared parser/capture wait helper"
    )
  end)
end)
