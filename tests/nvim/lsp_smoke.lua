-- Language smoke, Tier 2: parser support + Tree-sitter captures + LSP attach, against the PRODUCTION
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
--   (3) every matrix fixture opens under the production config with the expected
--       filetype, and every parser-backed row reports real Tree-sitter captures
--       so non-LSP parser/query runtime errors cannot hide,
--   (4) the auto-started bundled filetypes (lua/markdown/help/query) keep the
--       nvim-treesitter indentexpr the FileType autocmd promises.
--   (5) daily language buffers keep Vim regex syntax groups in addition to
--       Tree-sitter captures where parsers exist.
--
-- DOTFILES_LSP_SMOKE:
--   unset  -> no-op (an accidental run in the fast suite is harmless)
--   strict -> all gates above fail the run
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

local function stop_all_lsp_clients()
  local clients = vim.lsp.get_clients()
  if #clients == 0 then
    return true
  end
  for _, client in ipairs(clients) do
    pcall(function()
      client:stop(true)
    end)
  end
  local stopped = vim.wait(5000, function()
    for _, client in ipairs(clients) do
      local ok, is_stopped = pcall(function()
        return client:is_stopped()
      end)
      if not ok or not is_stopped then
        return false
      end
    end
    return true
  end, 50)
  if stopped then
    return true
  end
  local lingering = {}
  for _, client in ipairs(clients) do
    local ok, is_stopped = pcall(function()
      return client:is_stopped()
    end)
    if not ok or not is_stopped then
      table.insert(lingering, (client.name or "<unnamed>") .. "#" .. tostring(client.id))
    end
  end
  return false, table.concat(lingering, ", ")
end

-- All matrix-dependent work is pcall-wrapped: an uncaught error (a bad dofile, a
-- throw mid-loop) must STILL reach the cquit/qa below -- otherwise headless nvim
-- prints the error and never exits, blocking the e2e pipe until the job timeout.
local ok, err = pcall(function()
  local script = vim.fn.resolve(debug.getinfo(1, "S").source:sub(2))
  local repo_root = vim.fn.fnamemodify(script, ":h:h:h") -- tests/nvim/lsp_smoke.lua -> repo root
  local matrix = dofile(repo_root .. "/tests/nvim/language_matrix.lua")
  local fixtures = repo_root .. "/tests/nvim/fixtures/"

  -- The bundled-parser purge runs in nvim-treesitter's `config` (on plugin load,
  -- event = BufReadPre/BufNewFile). This headless probe opens no file before
  -- gate 0, so force-load the plugin now so its purge actually runs first --
  -- otherwise a cache-restored stale `parser/<bundled>.so` (the Windows e2e
  -- caches nvim-data/site) is still present when gate 0 checks, and the gate
  -- fails for a state real sessions never see (a real session opens a file,
  -- which loads the plugin and purges before any treesitter use).
  local old_sync_install = vim.env.DOTFILES_TREESITTER_SYNC_INSTALL
  vim.env.DOTFILES_TREESITTER_SYNC_INSTALL = "1"
  local treesitter_load_ok, treesitter_load_err = pcall(function()
    require("lazy").load({ plugins = { "nvim-treesitter" } })
  end)
  if old_sync_install == nil then
    vim.env.DOTFILES_TREESITTER_SYNC_INSTALL = nil
  else
    vim.env.DOTFILES_TREESITTER_SYNC_INSTALL = old_sync_install
  end
  if not treesitter_load_ok then
    fail(
      "nvim-treesitter synchronous parser bootstrap failed: "
        .. (tostring(treesitter_load_err):match("([^\r\n]+)") or "error")
    )
  end

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
    local explicit_parsers = {}
    if fh then
      fh:close()
    end
    if not body then
      fail("could not read the treesitter_parsers block from treesitter.lua")
    elseif vim.tbl_isempty(available) then
      fail("nvim-treesitter get_available() returned nothing (plugin not loaded?)")
    else
      for p in body:gmatch('"([^"]+)"') do
        table.insert(explicit_parsers, p)
        if available[p] and not unsupported[p] then
          note("parser supported: " .. p)
        else
          fail("treesitter parser NOT supported by nvim-treesitter main: " .. p)
        end
      end

      local expected_managed = {}
      local cfg_ok, cfg = pcall(require, "nvim-treesitter.config")
      if not cfg_ok or type(cfg.norm_languages) ~= "function" then
        fail("nvim-treesitter.config.norm_languages is unavailable; cannot audit managed parser files")
      else
        for _, parser in ipairs(cfg.norm_languages(explicit_parsers, { unsupported = true })) do
          expected_managed[parser] = true
        end
      end

      -- nvim-treesitter's install task writes compiled parsers to
      -- stdpath('data')/site/parser. Lazy's plugin checkout also lives under
      -- stdpath('data') and may legitimately ship runtime parser files of its
      -- own; those are plugin assets, not install-output drift. Audit only the
      -- managed install output here.
      local managed_parser_dir = vim.fs.normalize(vim.fn.stdpath("data") .. "/site/parser") .. "/"
      local unexpected = {}
      for _, so in ipairs(vim.api.nvim_get_runtime_file("parser/*.so", true)) do
        local normalized = vim.fs.normalize(so)
        if vim.startswith(normalized, managed_parser_dir) then
          local parser = vim.fn.fnamemodify(normalized, ":t:r")
          if not expected_managed[parser] then
            table.insert(unexpected, normalized)
          end
        end
      end
      if #unexpected > 0 then
        fail("unexpected nvim-treesitter-managed parser files: " .. table.concat(unexpected, ", "))
      else
        note("no unexpected nvim-treesitter install-output parser files")
      end
    end
  end

  -- mason.nvim prepends its bin dir to PATH inside its config, which for a real
  -- session runs on VeryLazy. Headless nvim never fires VeryLazy, so force-load
  -- it before opening matrix fixtures or testing LSP attachment; otherwise LSP
  -- rows can produce spawn noise for the wrong reason.
  pcall(function()
    require("lazy").load({ plugins = { "mason.nvim" } })
  end)

  local function has_treesitter_capture(buf)
    -- Headless nvim does not always materialize highlighter captures until a
    -- redraw. `vim.treesitter.get_parser()` can already succeed at that point,
    -- which proves parsing but not visible highlighting. Force the same redraw
    -- boundary a real UI crosses before asking `inspect_pos()` for highlight
    -- captures.
    pcall(vim.cmd, "redraw")

    local line_count = math.min(vim.api.nvim_buf_line_count(buf), 120)
    for line = 0, line_count - 1 do
      local text = vim.api.nvim_buf_get_lines(buf, line, line + 1, false)[1] or ""
      for col = 0, math.min(#text, 240) do
        local char = text:sub(col + 1, col + 1)
        if char ~= "" and char:match("%S") then
          local inspect_ok, pos = pcall(vim.inspect_pos, buf, line, col)
          if inspect_ok and pos.treesitter and #pos.treesitter > 0 then
            return true
          end
        end
      end
    end
    return false
  end

  -- (2) LSP attach. Non-gated servers must attach on every OS. powershell_es is
  -- a Windows target (lsp-config enables it only with pwsh + the PSES bundle):
  -- enforce it only on Windows, and skip cleanly elsewhere -- a legitimately
  -- absent runtime is never a failure, even under strict. This is what keeps the
  -- Unix e2e jobs from failing on a server designed not to run there.
  --
  -- Sanity-check that mason's bin reached PATH so a broken load surfaces as one
  -- clear failure instead of N opaque "did not attach"s.
  if vim.fn.executable("lua-language-server") ~= 1 then
    fail(
      "Mason bin not on PATH after loading mason; LSP servers are unreachable (expected "
        .. vim.fn.stdpath("data")
        .. "/mason/bin on PATH)"
    )
  end
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
        local stopped, lingering = stop_all_lsp_clients()
        if not stopped then
          fail(row.fixture .. ": LSP clients did not stop after attach gate: " .. lingering)
        end
      end
    end
  end

  local lsp_names = {}
  local lsp_seen = {}
  for _, row in ipairs(matrix) do
    if row.lsp and not lsp_seen[row.lsp] then
      lsp_seen[row.lsp] = true
      table.insert(lsp_names, row.lsp)
    end
  end

  -- (3) Matrix fixture runtime sanity. Parser support in gate 1 proves
  -- nvim-treesitter advertises the parser; synchronous bootstrap above proves
  -- setup can build it; opening every fixture under the real production init and
  -- checking captures proves the config can actually highlight that filetype.
  -- This covers parser-backed rows with no LSP, which the LSP attach gate above
  -- intentionally skips. Keep this AFTER the explicit LSP attach gate: opening
  -- every fixture under the production config can start LSPs as collateral.
  -- Disable the tested LSP configs after their explicit gate so these later
  -- parser/syntax checks do not create unrelated server processes.
  pcall(vim.lsp.enable, lsp_names, false)
  local disabled_stopped, disabled_lingering = stop_all_lsp_clients()
  if not disabled_stopped then
    fail("LSP clients did not stop after disabling auto-start: " .. disabled_lingering)
  end

  for _, row in ipairs(matrix) do
    local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(fixtures .. row.fixture))
    if not open_ok then
      fail(
        row.fixture .. ": open raised in matrix runtime gate: " .. (tostring(open_err):match("([^\r\n]+)") or "error")
      )
    elseif vim.bo.filetype ~= row.filetype then
      fail(
        row.fixture
          .. ": expected filetype "
          .. row.filetype
          .. " in matrix runtime gate, got "
          .. tostring(vim.bo.filetype)
      )
    elseif row.parser then
      -- macOS setup e2e can materialize some post-install parser captures
      -- noticeably later than the local unit gate after the full setup pass.
      vim.wait(5000, function()
        return has_treesitter_capture(0)
      end, 50)
      if has_treesitter_capture(0) then
        note(row.fixture .. ": opens as " .. row.filetype .. " with Tree-sitter captures")
      else
        fail(row.fixture .. ": opened as " .. row.filetype .. " but no Tree-sitter captures were reported")
      end
    else
      note(row.fixture .. ": opens as " .. row.filetype)
    end
    pcall(vim.cmd, "silent! bwipeout!")
  end

  -- (4) indentexpr preservation for the auto-started bundled filetypes. Removing
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

  -- (5) Regex syntax fallback for daily editing languages. Tree-sitter main
  -- clears the buffer-local 'syntax' option when it starts; restore the built-in
  -- syntax file afterward so real buffers do not look like plain text.
  local syntax_fallback = {
    { fixture = "sample.c", ft = "c", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    { fixture = "sample.cpp", ft = "cpp", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    -- CMake arguments are the important syntax-only fallback case; command
    -- names still prove Tree-sitter is active.
    { fixture = "CMakeLists.txt", ft = "cmake", syntax = { 1, 8 }, treesitter = { 1, 0 } },
    { fixture = "sample.py", ft = "python", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    { fixture = "sample.rs", ft = "rust", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    { fixture = "sample.ps1", ft = "ps1", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    { fixture = "sample.sh", ft = "sh", syntax = { 1, 0 }, treesitter = { 1, 0 } },
    { fixture = "sample.yaml", ft = "yaml", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    { fixture = "sample.json", ft = "json", syntax = { 0, 2 }, treesitter = { 0, 2 } },
    { fixture = "sample.jsonc", ft = "jsonc", syntax = { 1, 2 }, treesitter = false },
    { fixture = "sample.curlrc", ft = "conf", syntax = { 0, 6 }, treesitter = false },
    { fixture = "sample.md", ft = "markdown", syntax = { 0, 0 }, treesitter = { 0, 0 } },
    { fixture = "sample.bat", ft = "dosbatch", syntax = { 1, 0 }, treesitter = false },
  }
  for _, row in ipairs(syntax_fallback) do
    local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(fixtures .. row.fixture))
    if not open_ok then
      fail(
        row.fixture .. ": open raised in syntax fallback gate: " .. (tostring(open_err):match("([^\r\n]+)") or "error")
      )
    else
      local buf = vim.api.nvim_get_current_buf()
      vim.wait(5000, function()
        pcall(vim.cmd, "redraw")
        local syntax_ready = #vim.inspect_pos(buf, row.syntax[1], row.syntax[2]).syntax > 0
        if not row.treesitter then
          return syntax_ready
        end
        return syntax_ready and #vim.inspect_pos(buf, row.treesitter[1], row.treesitter[2]).treesitter > 0
      end, 50)
      local syntax_pos = vim.inspect_pos(buf, row.syntax[1], row.syntax[2])
      local treesitter_pos = row.treesitter and vim.inspect_pos(buf, row.treesitter[1], row.treesitter[2])
      if vim.bo[buf].syntax ~= row.ft then
        fail(row.fixture .. ": syntax fallback not restored (got: " .. tostring(vim.bo[buf].syntax) .. ")")
      elseif #syntax_pos.syntax == 0 then
        fail(row.fixture .. ": syntax fallback restored but no syntax groups reported at probe position")
      elseif row.treesitter and #treesitter_pos.treesitter == 0 then
        fail(row.fixture .. ": syntax fallback present but Tree-sitter captures missing at probe position")
      else
        note(row.fixture .. ": syntax fallback" .. (row.treesitter and " + Tree-sitter captures active" or " active"))
      end
    end
    pcall(vim.cmd, "silent! bwipeout!")
  end

  stop_all_lsp_clients()
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
