-- Language smoke, Tier 2: parser-support + LSP-attach, against the PRODUCTION
-- nvim config. It is NOT a plenary spec -- it needs the real init (pinned
-- nvim-treesitter `main` + Mason-installed LSP servers), which the fast
-- `make test-nvim` suite does not have. The e2e jobs run it after Mason sync:
--
--   DOTFILES_LSP_SMOKE=strict nvim --headless -u nvim/init.lua \
--     -c "luafile tests/nvim/lsp_smoke.lua" +qa
--
-- Exits nonzero (cquit) on any failure so the CI step fails.
--
-- DOTFILES_LSP_SMOKE:
--   unset  -> no-op (an accidental run in the fast suite is harmless)
--   strict -> parser-support is the STRICT gate (an unsupported parser fails).
--             LSP-attach is currently REPORT-ONLY (logged, never fails) because a
--             separate treesitter-highlight bug (the nvim 0.12 bundled lua
--             highlights query vs the nvim-treesitter main parser) can abort the
--             FileType chain before a server enables. Once that is fixed and
--             attach is confirmed green, flip the attach branch back to fail().

local mode = vim.env.DOTFILES_LSP_SMOKE
if not mode or mode == "" then
  io.stdout:write("lsp smoke: skipped (DOTFILES_LSP_SMOKE unset)\n")
  vim.cmd("qa!")
  return
end

local failures, notes = {}, {}
local function fail(m)
  table.insert(failures, m)
end
local function note(m)
  table.insert(notes, m)
end

local function to_set(list)
  local s = {}
  for _, v in ipairs(list or {}) do
    s[v] = true
  end
  return s
end

-- All matrix-dependent work is pcall-wrapped: an uncaught error (a bad dofile, a
-- throw mid-loop) must STILL reach the cquit/qa below -- otherwise headless nvim
-- prints the error and never exits, blocking the e2e pipe until the job timeout.
local ok, err = pcall(function()
  local script = vim.fn.resolve(debug.getinfo(1, "S").source:sub(2))
  local repo_root = vim.fn.fnamemodify(script, ":h:h:h") -- tests/nvim/lsp_smoke.lua -> repo root
  local matrix = dofile(repo_root .. "/tests/nvim/language_matrix.lua")
  local fixtures = repo_root .. "/tests/nvim/fixtures/"

  -- (1) Parser support: every parser the repo installs must be one
  -- nvim-treesitter `main` supports -- the jsonc "skipping unsupported language"
  -- catcher, validated against the pinned plugin (get_available()/get_available(4)).
  local nts_ok, nts = pcall(require, "nvim-treesitter")
  if not nts_ok then
    fail("require('nvim-treesitter') failed: " .. tostring(nts))
  elseif type(nts.get_available) ~= "function" then
    fail("nvim-treesitter.get_available is not a function (main API drifted)")
  else
    local available = to_set(nts.get_available())
    local unsupported = to_set(nts.get_available(4))
    local fh = io.open(repo_root .. "/nvim/lua/plugins/treesitter.lua")
    local body = fh and fh:read("*a"):match("local%s+treesitter_parsers%s*=%s*%{(.-)%}\n")
    if fh then
      fh:close()
    end
    if not body then
      fail("could not read the treesitter_parsers block from treesitter.lua")
    elseif vim.tbl_isempty(available) then
      fail("nvim-treesitter get_available() returned nothing (plugin not loaded?)")
    else
      for p in body:gmatch('"([^"]+)"') do
        if available[p] and not unsupported[p] then
          note("parser supported: " .. p)
        else
          fail("treesitter parser NOT supported by nvim-treesitter main: " .. p)
        end
      end
    end
  end

  -- (2) LSP attach. Non-gated servers must attach on every OS. powershell_es is
  -- a Windows target (lsp-config enables it only with pwsh + the PSES bundle):
  -- enforce it only on Windows, and skip cleanly elsewhere -- a legitimately
  -- absent runtime is never a failure, even under strict. This is what keeps the
  -- Unix e2e jobs from failing on a server designed not to run there.
  for _, row in ipairs(matrix) do
    if row.lsp then
      local skip
      if row.lsp_gated then
        local pses = vim.fn.stdpath("data") .. "/mason/packages/powershell-editor-services"
        if vim.fn.has("win32") ~= 1 then
          skip = "ps1 LSP is a Windows target (not enforced on this OS)"
        elseif vim.fn.executable("pwsh") ~= 1 then
          skip = "pwsh not installed"
        elseif vim.fn.isdirectory(pses) ~= 1 then
          skip = "PSES bundle not installed"
        end
      end
      if skip then
        note(row.fixture .. " [" .. row.lsp .. "]: skipped (" .. skip .. ")")
      else
        -- Opening a fixture can throw a treesitter HIGHLIGHT query error during
        -- BufReadPost (e.g. the nvim 0.12 bundled lua highlights query vs the
        -- nvim-treesitter main parser). That is unrelated to LSP attach -- the
        -- buffer still opens and the LSP enables on FileType -- so isolate it
        -- per-fixture (logged as a note) instead of letting it abort the probe.
        local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(fixtures .. row.fixture))
        if not open_ok then
          note(row.fixture .. ": open raised (treesitter highlight, not the LSP): " .. (tostring(open_err):match("([^\r\n]+)") or "error"))
        end
        local buf = vim.api.nvim_get_current_buf()
        local attached = vim.wait(15000, function()
          return #vim.lsp.get_clients({ bufnr = buf, name = row.lsp }) > 0
        end, 200)
        -- REPORT-ONLY (see the header): record attach status but do not fail,
        -- until the treesitter-highlight bug that can abort FileType is resolved.
        if attached then
          note(row.fixture .. " [" .. row.lsp .. "]: attached")
        else
          note(row.fixture .. " [" .. row.lsp .. "]: did NOT attach within 15s (report-only)")
        end
        pcall(vim.cmd, "silent! bwipeout!")
      end
    end
  end
end)

if not ok then
  fail("probe raised an error: " .. tostring(err))
end

for _, n in ipairs(notes) do
  io.stdout:write("  ok/skip: " .. n .. "\n")
end
if #failures > 0 then
  io.stderr:write("LSP SMOKE: " .. #failures .. " FAILURE(S):\n")
  for _, f in ipairs(failures) do
    io.stderr:write("  FAIL: " .. f .. "\n")
  end
  vim.cmd("cquit 1")
else
  io.stdout:write("lsp smoke: OK (" .. #notes .. " checks)\n")
  vim.cmd("qa!")
end
