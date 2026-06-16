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
-- Gates (all fail the run under strict):
--   (0) no `parser/<bundled>.so` override remains on the runtimepath (the config
--       purges them; a leftover re-creates the E5113 mismatch),
--   (1) every installed parser is one nvim-treesitter `main` supports,
--   (2) every non-gated LSP attaches, and every gated LSP attaches ON its target
--       OS (powershell_es -> Windows); a gated server is skipped only OFF target,
--       and a MISSING runtime on the target OS is a failure, not a skip,
--   (3) the auto-started bundled filetypes (lua/markdown/help/query) keep the
--       nvim-treesitter indentexpr the FileType autocmd promises.
--
-- DOTFILES_LSP_SMOKE:
--   unset  -> no-op (an accidental run in the fast suite is harmless)
--   strict -> all four gates above fail the run
--   other  -> same behaviour (a gated row off its target OS still skips cleanly)

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

  -- (0) Bundled-parser override preflight. The production config purges any
  -- nvim-treesitter parser for a Neovim-bundled language on load; after that, no
  -- nvim-treesitter-managed `parser/<bundled>.so` may remain. A leftover (e.g.
  -- restored from a CI cache, or installed by an older config) overrides
  -- Neovim's matched built-in parser and re-creates the E5113 query/parser
  -- mismatch this whole change exists to prevent. Scope to stdpath('data'):
  -- Neovim's OWN bundled parser .so files live under the install prefix and are
  -- legitimately on the runtimepath -- they must NOT trip this gate.
  local managed = vim.fs.normalize(vim.fn.stdpath("data")) .. "/"
  for _, lang in ipairs({ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" }) do
    local overrides = {}
    for _, so in ipairs(vim.api.nvim_get_runtime_file("parser/" .. lang .. ".so", true)) do
      if vim.startswith(vim.fs.normalize(so), managed) then
        table.insert(overrides, so)
      end
    end
    if #overrides > 0 then
      fail("nvim-treesitter override still present for bundled " .. lang .. ": " .. table.concat(overrides, ", "))
    else
      note("no nvim-treesitter override for bundled " .. lang)
    end
  end

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
      local skip, gated_fail
      if row.lsp_gated then
        -- A gated server is skip-not-fail only OFF its target OS. ON the target
        -- (powershell_es -> Windows), the runtime MUST be present: setup.ps1
        -- -All installs pwsh and Mason installs the PSES bundle, so a missing
        -- one is a real setup regression, not a legitimate absence. Failing here
        -- is what makes the Windows STRICT path actually strict (otherwise it
        -- could pass for the wrong reason -- a silently-skipped target server).
        local pses = vim.fn.stdpath("data") .. "/mason/packages/powershell-editor-services"
        if vim.fn.has("win32") ~= 1 then
          skip = "ps1 LSP is a Windows target (not enforced on this OS)"
        elseif vim.fn.executable("pwsh") ~= 1 then
          gated_fail = "pwsh missing on the Windows target (setup.ps1 -All should install it)"
        elseif vim.fn.isdirectory(pses) ~= 1 then
          gated_fail = "PSES bundle missing on the Windows target (Mason should install powershell-editor-services)"
        end
      end
      if gated_fail then
        fail(row.fixture .. " [" .. row.lsp .. "]: " .. gated_fail)
      elseif skip then
        note(row.fixture .. " [" .. row.lsp .. "]: skipped (" .. skip .. ")")
      else
        -- Opening a fixture must NOT raise. The treesitter HIGHLIGHT query error
        -- that used to fire during BufReadPost (nvim 0.12 bundled lua highlights
        -- query vs nvim-treesitter main's older parser -> E5113) is fixed
        -- canonically: the nvim-bundled langs are no longer installed by
        -- nvim-treesitter, so the matched built-in query is in effect. A raise
        -- here now means that regression is back -- record it as a failure, but
        -- still pcall-isolate it so one bad fixture does not abort the probe.
        local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(fixtures .. row.fixture))
        if not open_ok then
          fail(
            row.fixture
              .. ": open raised (treesitter highlight regression?): "
              .. (tostring(open_err):match("([^\r\n]+)") or "error")
          )
        end
        local buf = vim.api.nvim_get_current_buf()
        local attached = vim.wait(45000, function()
          return #vim.lsp.get_clients({ bufnr = buf, name = row.lsp }) > 0
        end, 200)
        -- STRICT (see the header): a non-gated server that does not attach is a
        -- hard failure. The treesitter highlight error that aborted FileType is
        -- fixed canonically (the nvim-bundled langs are no longer installed by
        -- nvim-treesitter), so a non-attach is now a real LSP/Mason regression.
        if attached then
          note(row.fixture .. " [" .. row.lsp .. "]: attached")
        else
          fail(row.fixture .. " [" .. row.lsp .. "]: did NOT attach within 45s")
        end
        pcall(vim.cmd, "silent! bwipeout!")
      end
    end
  end

  -- (3) indentexpr preservation for the auto-started bundled filetypes. Removing
  -- the bundled langs from the install list must NOT drop the indentexpr the
  -- FileType autocmd promises (the pre-fix behavior). Source-shape tests only
  -- prove the table/loop exist; this proves the option is actually set on a real
  -- buffer after FileType processing. help has no fixture, so synthesize one.
  local indent_expr = "v:lua.require'nvim-treesitter'.indentexpr()"
  local bundled_indent = {
    { ft = "lua", fixture = "sample.lua" },
    { ft = "markdown", fixture = "sample.md" },
    { ft = "query", fixture = "queries/lua/highlights.scm" },
    { ft = "help", synth = true },
  }
  for _, b in ipairs(bundled_indent) do
    local opened = true
    if b.synth then
      vim.cmd("enew")
      vim.api.nvim_buf_set_lines(0, 0, -1, false, { "*synthetic.txt*  A synthetic help buffer", "", "Heading~" })
      vim.bo.filetype = b.ft -- fires FileType help -> our autocmd sets indentexpr
    else
      -- A fixture that fails to open (or that detects the wrong filetype) would
      -- silently make the indentexpr check meaningless -- treat both as failures
      -- so this gate can't pass for the wrong reason.
      local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(fixtures .. b.fixture))
      if not open_ok then
        opened = false
        fail(b.fixture .. ": open raised in indentexpr gate: " .. (tostring(open_err):match("([^\r\n]+)") or "error"))
      end
    end
    if opened and vim.bo.filetype ~= b.ft then
      fail(
        "indentexpr gate: expected filetype "
          .. b.ft
          .. " for "
          .. (b.fixture or "synthetic help")
          .. ", got "
          .. tostring(vim.bo.filetype)
      )
    elseif opened then
      if vim.bo.indentexpr == indent_expr then
        note("indentexpr preserved: " .. b.ft)
      else
        fail(
          "indentexpr NOT set for auto-started bundled filetype "
            .. b.ft
            .. " (got: "
            .. tostring(vim.bo.indentexpr)
            .. ")"
        )
      end
    end
    pcall(vim.cmd, "silent! bwipeout!")
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
