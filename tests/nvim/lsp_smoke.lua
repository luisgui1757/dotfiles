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
--   (0) no `parser/<bundled>.so` or managed `queries/<bundled>/` override
--       remains on the runtimepath/install output (the config purges them; a
--       leftover re-creates the E5113 mismatch),
--   (1) every installed parser is one nvim-treesitter `main` supports,
--   (2) every non-gated LSP attaches, and every gated LSP attaches ON its target
--       OS (powershell_es -> Windows); a gated server is skipped only OFF target,
--       and a MISSING runtime on the target OS is a failure, not a skip,
--   (3) formatter/LSP compatibility: realistic formatter-owned buffers are
--       formatted through conform.nvim's production route, must use the expected
--       external formatter(s), and must produce no LSP warnings/errors afterward,
--   (4) every matrix fixture opens under the production config with the expected
--       filetype, and every parser-backed row reports real Tree-sitter captures
--       so non-LSP parser/query runtime errors cannot hide,
--   (5) the auto-started bundled filetypes (lua/markdown/help/query) keep the
--       nvim-treesitter indentexpr the FileType autocmd promises.
--   (6) daily language buffers keep Vim regex syntax groups in addition to
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
  -- nvim-treesitter parser/query output for a Neovim-bundled language on load;
  -- after that, no nvim-treesitter-managed `parser/<bundled>.so` or
  -- `queries/<bundled>/` may remain. A leftover (e.g. restored from a CI cache,
  -- or installed by an older config) overrides Neovim's matched built-in
  -- parser/query pair and re-creates the E5113 mismatch this whole change exists
  -- to prevent. Scope parser .so checks to stdpath('data'): Neovim's OWN bundled
  -- parser .so files live under the install prefix and are legitimately on the
  -- runtimepath -- they must NOT trip this gate.
  local managed = vim.fs.normalize(vim.fn.stdpath("data")) .. "/"
  local bundled_parsers = { "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" }
  for _, lang in ipairs(bundled_parsers) do
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
  local config_ok, treesitter_config = pcall(require, "nvim-treesitter.config")
  local query_dir_ok, query_dir = false, nil
  if config_ok and type(treesitter_config.get_install_dir) == "function" then
    query_dir_ok, query_dir = pcall(treesitter_config.get_install_dir, "queries")
  end
  if not query_dir_ok or type(query_dir) ~= "string" or query_dir == "" then
    fail("cannot inspect nvim-treesitter query install output for bundled overrides")
  else
    local normalized_query_dir = vim.fs.normalize(query_dir) .. "/"
    if not vim.startswith(normalized_query_dir, managed) then
      fail("nvim-treesitter query install output is outside stdpath('data'): " .. query_dir)
    else
      for _, lang in ipairs(bundled_parsers) do
        local query_path = vim.fs.joinpath(query_dir, lang)
        if vim.fn.isdirectory(query_path) == 1 then
          fail("nvim-treesitter query override still present for bundled " .. lang .. ": " .. query_path)
        else
          note("no nvim-treesitter query override for bundled " .. lang)
        end
      end
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
      local explicit_parser_set = to_set(explicit_parsers)

      local expected_managed_parsers = {}
      local expected_managed_parser_set = {}
      local expected_managed_queries = {}
      local expected_managed_query_set = {}
      local parser_configs_ok, parser_configs = pcall(require, "nvim-treesitter.parsers")
      local bundled_parser_set = to_set({ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" })
      local cfg_ok, cfg = pcall(require, "nvim-treesitter.config")
      if
        not cfg_ok
        or type(cfg.norm_languages) ~= "function"
        or type(cfg.get_install_dir) ~= "function"
        or not parser_configs_ok
      then
        fail("nvim-treesitter config/parser metadata is unavailable; cannot audit managed parser/query output")
      else
        for _, parser in ipairs(cfg.norm_languages(explicit_parsers, { unsupported = true })) do
          local parser_config = parser_configs[parser]
          if not bundled_parser_set[parser] and parser_config then
            if parser_config.install_info and not expected_managed_parser_set[parser] then
              expected_managed_parser_set[parser] = true
              table.insert(expected_managed_parsers, parser)
            end
            if
              #vim.api.nvim_get_runtime_file("queries/" .. parser, false) > 0
              and not expected_managed_query_set[parser]
            then
              expected_managed_query_set[parser] = true
              table.insert(expected_managed_queries, parser)
            end
          end
        end
      end

      if type(nts.get_installed) ~= "function" then
        fail("nvim-treesitter.get_installed is unavailable; cannot prove parser install output")
      else
        local installed = to_set(nts.get_installed("parsers"))
        local missing = {}
        for _, parser in ipairs(expected_managed_parsers) do
          if not installed[parser] then
            table.insert(missing, parser)
          end
        end
        if #missing > 0 then
          fail("expected nvim-treesitter parser install output missing: " .. table.concat(missing, ", "))
        else
          note("all expected nvim-treesitter install-output parsers are present")
        end
      end

      if cfg_ok and type(cfg.get_install_dir) == "function" then
        local missing_queries = {}
        local missing_highlight_queries = {}
        local install_query_dir = cfg.get_install_dir("queries")
        for _, parser in ipairs(expected_managed_queries) do
          local parser_query_dir = vim.fs.joinpath(install_query_dir, parser)
          if vim.fn.isdirectory(parser_query_dir) ~= 1 then
            table.insert(missing_queries, parser)
          elseif
            explicit_parser_set[parser] and vim.uv.fs_stat(vim.fs.joinpath(parser_query_dir, "highlights.scm")) == nil
          then
            table.insert(missing_highlight_queries, parser)
          end
        end
        if #missing_queries > 0 then
          fail("expected nvim-treesitter query install output missing: " .. table.concat(missing_queries, ", "))
        elseif #missing_highlight_queries > 0 then
          fail("expected nvim-treesitter highlight query output missing: " .. table.concat(missing_highlight_queries, ", "))
        else
          note("all expected nvim-treesitter query install-output directories/highlights are present")
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
          if not expected_managed_parser_set[parser] then
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

  local function has_treesitter_highlight_query_capture(buf, parser, parser_obj, parsed_trees)
    if not parser or parser == "" then
      return false
    end

    local query_ok, query = pcall(vim.treesitter.query.get, parser, "highlights")
    if (not query_ok or not query) and vim.treesitter.query.get_query then
      query_ok, query = pcall(vim.treesitter.query.get_query, parser, "highlights")
    end
    if not query_ok or not query then
      return false
    end

    local trees = parsed_trees
    if not trees and parser_obj then
      local parse_ok, parsed_or_err = pcall(function()
        return parser_obj:parse()
      end)
      if parse_ok then
        trees = parsed_or_err
      end
    end
    if type(trees) ~= "table" or #trees == 0 then
      return false
    end

    local line_count = vim.api.nvim_buf_line_count(buf)
    local ok, found = pcall(function()
      for _, tree in ipairs(trees) do
        local root = tree and tree:root()
        if root then
          for _, node in query:iter_captures(root, buf, 0, line_count) do
            local start_row, start_col, end_row, end_col = node:range()
            if start_row < end_row or start_col < end_col then
              local text = table.concat(vim.api.nvim_buf_get_text(buf, start_row, start_col, end_row, end_col, {}), "\n")
              if text:match("%S") then
                return true
              end
            end
          end
        end
      end
      return false
    end)
    return ok and found == true
  end

  local function has_treesitter_capture(buf, parser, parser_obj, parsed_trees)
    -- Headless nvim does not always materialize highlighter captures until a
    -- redraw. `vim.treesitter.get_parser()` can already succeed at that point,
    -- which proves parsing but not visible highlighting. Force the same redraw
    -- boundary a real UI crosses before asking `inspect_pos()` for highlight
    -- captures. If a headless host still does not materialize `inspect_pos()`
    -- captures after parse, fall back to directly iterating the same parser's
    -- `highlights` query so the strict smoke still proves parser+query captures.
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
    return has_treesitter_highlight_query_capture(buf, parser, parser_obj, parsed_trees)
  end

  local function one_line_error(err)
    return tostring(err):match("([^\r\n]+)") or "error"
  end

  local function treesitter_capture_ready(buf, parser)
    if not parser or parser == "" then
      return false, "missing expected parser name"
    end

    local syntax_before_start = vim.bo[buf].syntax
    local start_ok, start_err = pcall(vim.treesitter.start, buf, parser)
    if syntax_before_start ~= "" and vim.bo[buf].syntax == "" then
      vim.bo[buf].syntax = syntax_before_start
    end
    if not start_ok then
      return false, "vim.treesitter.start(" .. parser .. ") failed: " .. one_line_error(start_err)
    end

    local parser_ok, parser_obj = pcall(vim.treesitter.get_parser, buf, parser)
    if not parser_ok then
      return false, "vim.treesitter.get_parser(" .. parser .. ") failed: " .. one_line_error(parser_obj)
    end

    local parsed_trees
    local parse_ok, parse_err = pcall(function()
      parsed_trees = parser_obj:parse()
    end)
    if not parse_ok then
      return false, "Tree-sitter parse(" .. parser .. ") failed: " .. one_line_error(parse_err)
    end

    return has_treesitter_capture(buf, parser, parser_obj, parsed_trees), nil
  end

  local function wait_for_treesitter_capture(buf, parser, timeout_ms)
    local last_err = nil
    local captured = vim.wait(timeout_ms or 5000, function()
      local ready, err = treesitter_capture_ready(buf, parser)
      last_err = err
      return ready
    end, 50)
    return captured, last_err
  end

  local function same_list(actual, expected)
    if #actual ~= #expected then
      return false
    end
    for i, value in ipairs(expected) do
      if actual[i] ~= value then
        return false
      end
    end
    return true
  end

  local function serious_diagnostics(buf)
    local diagnostics = {}
    for _, diagnostic in ipairs(vim.diagnostic.get(buf)) do
      if not diagnostic.severity or diagnostic.severity <= vim.diagnostic.severity.WARN then
        table.insert(diagnostics, diagnostic)
      end
    end
    return diagnostics
  end

  local function render_diagnostics(diagnostics)
    local rendered = {}
    for _, diagnostic in ipairs(diagnostics) do
      table.insert(
        rendered,
        table.concat({
          diagnostic.source or "?",
          vim.diagnostic.severity[diagnostic.severity] or tostring(diagnostic.severity),
          tostring((diagnostic.lnum or 0) + 1),
          tostring((diagnostic.col or 0) + 1),
          diagnostic.message or "",
        }, ":")
      )
    end
    return table.concat(rendered, " | ")
  end

  local function wait_for_serious_diagnostics_to_settle(buf)
    local start = vim.uv.now()
    local stable_since = start
    local last = nil
    vim.wait(10000, function()
      local rendered = render_diagnostics(serious_diagnostics(buf))
      if rendered ~= last then
        last = rendered
        stable_since = vim.uv.now()
      end
      local now = vim.uv.now()
      return now - start >= 1500 and now - stable_since >= 500
    end, 100)
    return serious_diagnostics(buf)
  end

  local lsp_attach_timeout_ms = vim.fn.has("win32") == 1 and 90000 or 45000
  local function wait_for_lsp_client(buf, name)
    return vim.wait(lsp_attach_timeout_ms, function()
      return #vim.lsp.get_clients({ bufnr = buf, name = name }) > 0
    end, 200)
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
        local attached = wait_for_lsp_client(buf, row.lsp)
        -- STRICT (see the header): a non-gated server that does not attach is a
        -- hard failure. The treesitter highlight error that aborted FileType is
        -- fixed canonically (the nvim-bundled langs are no longer installed by
        -- nvim-treesitter), so a non-attach is now a real LSP/Mason regression.
        if attached then
          note(row.fixture .. " [" .. row.lsp .. "]: attached")
        else
          fail(
            row.fixture
              .. " ["
              .. row.lsp
              .. "]: did NOT attach within "
              .. tostring(math.floor(lsp_attach_timeout_ms / 1000))
              .. "s"
          )
        end
        pcall(vim.cmd, "silent! bwipeout!")
        local stopped, lingering = stop_all_lsp_clients()
        if not stopped then
          fail(row.fixture .. ": LSP clients did not stop after attach gate: " .. lingering)
        end
      end
    end
  end

  -- (3) Formatter/LSP compatibility. Conform owns format-on-save, but the
  -- output still has to satisfy the language server's parser/schema rules. Use
  -- separate fixture copies under tests/.cache so the smoke can format real files
  -- without mutating tracked fixtures.
  local formatter_lsp_samples = {
    {
      source = "formatter_lsp/sample.lua",
      target = "sample.lua",
      ft = "lua",
      lsp = "lua_ls",
      formatters = { "stylua" },
    },
    {
      source = "formatter_lsp/sample.py",
      target = "sample.py",
      ft = "python",
      lsp = "pyright",
      formatters = { "ruff_fix", "ruff_format" },
    },
    {
      source = "formatter_lsp/sample.c",
      target = "sample.c",
      ft = "c",
      lsp = "clangd",
      formatters = { "clang_format" },
    },
    {
      source = "formatter_lsp/sample.cpp",
      target = "sample.cpp",
      ft = "cpp",
      lsp = "clangd",
      formatters = { "clang_format" },
    },
    {
      source = "formatter_lsp/sample.rs",
      target = "sample.rs",
      ft = "rust",
      lsp = "rust_analyzer",
      formatters = { "rustfmt" },
    },
    {
      source = "formatter_lsp/CMakeLists.txt",
      target = "cmake/CMakeLists.txt",
      ft = "cmake",
      lsp = "neocmake",
      formatters = { "gersemi" },
    },
    {
      source = "formatter_lsp/sample.bash",
      target = "sample.bash",
      ft = "sh",
      lsp = "bashls",
      formatters = { "shfmt" },
    },
    {
      source = "formatter_lsp/sample.zsh",
      target = "sample.zsh",
      ft = "zsh",
      lsp = "bashls",
      formatters = { "shfmt" },
    },
    {
      source = "formatter_lsp/sample.json",
      target = "sample.json",
      ft = "json",
      lsp = "jsonls",
      formatters = { "prettier" },
    },
    {
      source = "formatter_lsp/sample.jsonc",
      target = "sample.jsonc",
      ft = "jsonc",
      lsp = "jsonls",
      formatters = { "prettier" },
    },
    {
      source = "formatter_lsp/sample.yaml",
      target = "sample.yaml",
      ft = "yaml",
      lsp = "yamlls",
      formatters = { "prettier" },
    },
  }
  local compat_root = repo_root .. "/tests/.cache/lsp-smoke-formatters"
  vim.fn.delete(compat_root, "rf")
  vim.fn.mkdir(compat_root, "p")
  local conform_ok, conform = pcall(require, "conform")
  if not conform_ok then
    fail("formatter/LSP compatibility: require('conform') failed: " .. tostring(conform))
  else
    for _, sample in ipairs(formatter_lsp_samples) do
      local source_path = fixtures .. sample.source
      local target_path = compat_root .. "/" .. sample.target
      vim.fn.mkdir(vim.fn.fnamemodify(target_path, ":h"), "p")
      local read_ok, lines = pcall(vim.fn.readfile, source_path)
      if not read_ok then
        fail(sample.source .. ": could not read formatter/LSP fixture: " .. tostring(lines))
      else
        local write_ok, write_err = pcall(vim.fn.writefile, lines, target_path)
        if not write_ok then
          fail(sample.source .. ": could not write formatter/LSP temp file: " .. tostring(write_err))
        else
          local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(target_path))
          if not open_ok then
            fail(
              sample.source
                .. ": open raised in formatter/LSP gate: "
                .. (tostring(open_err):match("([^\r\n]+)") or "error")
            )
          elseif vim.bo.filetype ~= sample.ft then
            fail(
              sample.source
                .. ": expected filetype "
                .. sample.ft
                .. " in formatter/LSP gate, got "
                .. tostring(vim.bo.filetype)
            )
          else
            local buf = vim.api.nvim_get_current_buf()
            local attached = wait_for_lsp_client(buf, sample.lsp)
            if not attached then
              fail(
                sample.source
                  .. " ["
                  .. sample.lsp
                  .. "]: did NOT attach within "
                  .. tostring(math.floor(lsp_attach_timeout_ms / 1000))
                  .. "s in formatter/LSP gate"
              )
            else
              local formatters, uses_lsp = conform.list_formatters_to_run(buf)
              local formatter_names = {}
              local unavailable = {}
              for _, formatter in ipairs(formatters) do
                table.insert(formatter_names, formatter.name)
                if not formatter.available then
                  table.insert(unavailable, formatter.name)
                end
              end
              if not same_list(formatter_names, sample.formatters) then
                fail(
                  sample.source
                    .. ": expected conform formatter(s) "
                    .. table.concat(sample.formatters, ",")
                    .. ", got "
                    .. table.concat(formatter_names, ",")
                )
              elseif #unavailable > 0 then
                fail(sample.source .. ": formatter(s) unavailable: " .. table.concat(unavailable, ","))
              elseif uses_lsp then
                fail(sample.source .. ": conform would use LSP formatting despite an external formatter mapping")
              else
                local format_call_ok, format_ok, format_err = pcall(conform.format, {
                  bufnr = buf,
                  async = false,
                  lsp_format = "fallback",
                  timeout_ms = 10000,
                })
                if not format_call_ok then
                  fail(sample.source .. ": conform format raised: " .. tostring(format_ok))
                elseif format_ok == false then
                  fail(sample.source .. ": conform format failed: " .. tostring(format_err))
                else
                  -- This gate already invoked conform explicitly. Persist the
                  -- temp buffer without letting the normal BufWritePre hook run
                  -- a second formatter pass over the same sample.
                  vim.b[buf].skip_format_on_save = true
                  local save_ok, save_err = pcall(vim.cmd.write)
                  if not save_ok then
                    fail(sample.source .. ": could not write formatted temp file: " .. tostring(save_err))
                  else
                    local diagnostics = wait_for_serious_diagnostics_to_settle(buf)
                    if #diagnostics > 0 then
                      fail(
                        sample.source
                          .. ": LSP warning/error after conform formatting: "
                          .. render_diagnostics(diagnostics)
                      )
                    else
                      note(
                        sample.source
                          .. " ["
                          .. sample.lsp
                          .. "]: conform "
                          .. table.concat(sample.formatters, "+")
                          .. " output accepted by LSP"
                      )
                    end
                  end
                end
              end
            end
          end
        end
      end
      pcall(vim.cmd, "silent! bwipeout!")
      local stopped, lingering = stop_all_lsp_clients()
      if not stopped then
        fail(sample.source .. ": LSP clients did not stop after formatter/LSP gate: " .. lingering)
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

  -- (4) Matrix fixture runtime sanity. Parser support in gate 1 proves
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
      -- Force the expected parser through the same start/parse boundary a real
      -- buffer needs, then require visible highlight captures.
      local captured, capture_err = wait_for_treesitter_capture(0, row.parser)
      if captured then
        note(row.fixture .. ": opens as " .. row.filetype .. " with Tree-sitter captures")
      else
        fail(
          row.fixture
            .. ": opened as "
            .. row.filetype
            .. " but no Tree-sitter captures were reported"
            .. (capture_err and (" (" .. capture_err .. ")") or "")
        )
      end
    else
      note(row.fixture .. ": opens as " .. row.filetype)
    end
    pcall(vim.cmd, "silent! bwipeout!")
  end

  -- (5) indentexpr preservation for the auto-started bundled filetypes. Removing
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

  -- (6) Regex syntax fallback for daily editing languages. Tree-sitter main
  -- clears the buffer-local 'syntax' option when it starts; restore the built-in
  -- syntax file afterward so real buffers do not look like plain text.
  local syntax_fallback = {
    { fixture = "sample.c", ft = "c", syntax = { 0, 0 }, parser = "c" },
    { fixture = "sample.cpp", ft = "cpp", syntax = { 0, 0 }, parser = "cpp" },
    -- CMake arguments are the important syntax-only fallback case; command
    -- names still prove Tree-sitter is active.
    { fixture = "CMakeLists.txt", ft = "cmake", syntax = { 1, 8 }, parser = "cmake" },
    { fixture = "sample.py", ft = "python", syntax = { 0, 0 }, parser = "python" },
    { fixture = "sample.rs", ft = "rust", syntax = { 0, 0 }, parser = "rust" },
    { fixture = "sample.ps1", ft = "ps1", syntax = { 0, 0 }, parser = "powershell" },
    { fixture = "sample.sh", ft = "sh", syntax = { 1, 0 }, parser = "bash" },
    { fixture = "sample.yaml", ft = "yaml", syntax = { 0, 0 }, parser = "yaml" },
    { fixture = "sample.json", ft = "json", syntax = { 0, 2 }, parser = "json" },
    { fixture = "sample.jsonc", ft = "jsonc", syntax = { 1, 2 } },
    { fixture = "sample.curlrc", ft = "conf", syntax = { 0, 6 } },
    { fixture = "sample.md", ft = "markdown", syntax = { 0, 0 }, parser = "markdown" },
    { fixture = "sample.bat", ft = "dosbatch", syntax = { 1, 0 } },
  }
  for _, row in ipairs(syntax_fallback) do
    local open_ok, open_err = pcall(vim.cmd.edit, vim.fn.fnameescape(fixtures .. row.fixture))
    if not open_ok then
      fail(
        row.fixture .. ": open raised in syntax fallback gate: " .. (tostring(open_err):match("([^\r\n]+)") or "error")
      )
    else
      local buf = vim.api.nvim_get_current_buf()
      local capture_err = nil
      vim.wait(5000, function()
        pcall(vim.cmd, "redraw")
        local syntax_ready = #vim.inspect_pos(buf, row.syntax[1], row.syntax[2]).syntax > 0
        if not row.parser then
          return syntax_ready
        end
        local capture_ready
        capture_ready, capture_err = treesitter_capture_ready(buf, row.parser)
        return syntax_ready and capture_ready
      end, 50)
      local syntax_pos = vim.inspect_pos(buf, row.syntax[1], row.syntax[2])
      local capture_ready = row.parser and has_treesitter_capture(buf, row.parser)
      if vim.bo[buf].syntax ~= row.ft then
        fail(row.fixture .. ": syntax fallback not restored (got: " .. tostring(vim.bo[buf].syntax) .. ")")
      elseif #syntax_pos.syntax == 0 then
        fail(row.fixture .. ": syntax fallback restored but no syntax groups reported at probe position")
      elseif row.parser and not capture_ready then
        fail(
          row.fixture
            .. ": syntax fallback present but Tree-sitter captures missing"
            .. (capture_err and (" (" .. capture_err .. ")") or "")
        )
      else
        note(row.fixture .. ": syntax fallback" .. (row.parser and " + Tree-sitter captures active" or " active"))
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
