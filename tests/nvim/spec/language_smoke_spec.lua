-- Per-language smoke test, Tier 1 (fast; runs in `make test-nvim`).
--
-- Source of truth: tests/nvim/language_matrix.lua + tests/nvim/fixtures/.
-- For each fixture this asserts:
--   * Neovim detects the expected filetype,
--   * conform's formatters_by_ft matches the matrix,
--   * the matrix's parser is one the repo actually installs (treesitter_parsers).
--
-- The Tier-2 LSP-attach smoke (needs Mason servers + runtimes) is a separate
-- spec gated on DOTFILES_LSP_SMOKE and only runs in the e2e jobs.
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
      it("installs the " .. row.parser .. " parser for " .. row.fixture, function()
        assert.is_truthy(parsers[row.parser], row.parser .. " is in the matrix but not in treesitter_parsers")
      end)
    end
  end
end)
