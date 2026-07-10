-- Parsers nvim-treesitter installs/compiles. This list deliberately EXCLUDES
-- the languages Neovim 0.12 bundles (c, lua, markdown, markdown_inline, query,
-- vim, vimdoc). Neovim's built-in parser for each is matched to its bundled
-- highlight query; installing nvim-treesitter's (often older) parser would
-- override the built-in via site/parser while Neovim's bundled query is still
-- the one in effect, e.g. the lua query's `operator:` field -> "Invalid field
-- name operator" (E5113) on every lua buffer. Neovim auto-starts treesitter for
-- lua/markdown/vimdoc(help)/query through its runtime ftplugins; c and vim are
-- bundled too but Neovim does NOT auto-start them, so our FileType autocmd
-- starts them with the matched built-in parser (nvim_bundled_started_here).
-- jsonc is excluded for a different reason: nvim-treesitter main has no jsonc
-- grammar at all (it warns the language is unsupported), and the json grammar
-- errors on slash-slash comments, so jsonc uses Neovim's built-in syntax.
local treesitter_parsers = {
  "cpp",
  "cmake",
  "python",
  "rust",
  "bash",
  "zsh",
  "powershell",
  "javascript",
  "typescript",
  "tsx",
  "html",
  "css",
  "scss",
  "vue",
  "svelte",
  "astro",
  "graphql",
  "json",
  "json5",
  "yaml",
  "toml",
  "xml",
  "razor",
  "csv",
  "dockerfile",
  "hcl",
  "terraform",
  "make",
  "http",
  "sql",
  "proto",
  "prisma",
  "nix",
  "ini",
  "editorconfig",
  "git_config",
  "git_rebase",
  "gitignore",
  "ssh_config",
  "requirements",
  "fish",
  "nu",
  "just",
  "meson",
  "ninja",
  "nginx",
  "jq",
  "kdl",
  "desktop",
  "latex",
  "bibtex",
  "typst",
  "mermaid",
  "glsl",
  "wgsl",
  "ron",
  "go",
  "gomod",
  "gosum",
  "gowork",
  "c_sharp",
  "fsharp",
  "java",
  "kotlin",
  "scala",
  "php",
  "ruby",
  "perl",
  "swift",
  "zig",
  "asm",
  "nasm",
  "dart",
  "elixir",
  "erlang",
  "clojure",
  "haskell",
  "ocaml",
  "r",
  "solidity",
  "julia",
  "fortran",
  "pascal",
  "ada",
  "groovy",
  "matlab",
  "gleam",
  "qmljs",
  "bicep",
  "earthfile",
  "diff",
  "gitcommit",
}

-- Neovim-bundled languages Neovim does NOT auto-start (unlike lua/markdown/
-- vimdoc/query). We start these with Neovim's matched built-in parser. They must
-- NOT go in treesitter_parsers -- that would re-introduce the override/mismatch.
local nvim_bundled_started_here = { "c", "vim" }

-- Neovim-bundled languages Neovim DOES auto-start (lua/markdown via runtime
-- ftplugins; vimdoc as filetype `help`; query as filetype `query`). We do not
-- start these (Neovim already did, with the matched built-in parser), but we
-- still cover their filetypes in the FileType autocmd so they get
-- nvim-treesitter's indentexpr -- the same as every other treesitter buffer, and
-- the same as before the bundled-language exclusion. Verified safe: unlike its
-- highlights query, nvim-treesitter's indents.scm for c/lua/markdown/query
-- compiles cleanly against Neovim's built-in parser (it references no field the
-- built-in parser lacks). markdown_inline is injection-only (no standalone
-- filetype), so it is intentionally omitted.
local nvim_bundled_autostarted_filetypes = { "lua", "markdown", "help", "query" }

-- Every language Neovim 0.12 bundles a matched parser+query for, by PARSER (.so)
-- name (vimdoc's filetype is `help`; markdown_inline has no standalone filetype).
-- None may be installed by nvim-treesitter; this list also drives the stale-
-- override purge in config() below.
local nvim_bundled_parsers = { "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" }

local parser_filetype_aliases = {
  bash = { "sh" },
  bibtex = { "bib" },
  c_sharp = { "cs" },
  git_config = { "gitconfig" },
  git_rebase = { "gitrebase" },
  ini = { "dosini" },
  javascript = { "javascriptreact" },
  latex = { "plaintex", "tex" },
  powershell = { "ps1" },
  qmljs = { "qml" },
  ssh_config = { "sshconfig" },
  tsx = { "typescriptreact" },
}

local treesitter_filetypes = {}
local treesitter_filetype_set = {}

local function add_unique(list, set, value)
  if not set[value] then
    set[value] = true
    table.insert(list, value)
  end
end

local function add_treesitter_filetype(filetype)
  add_unique(treesitter_filetypes, treesitter_filetype_set, filetype)
end

for _, parser in ipairs(treesitter_parsers) do
  add_treesitter_filetype(parser)
  for _, filetype in ipairs(parser_filetype_aliases[parser] or {}) do
    add_treesitter_filetype(filetype)
  end
end
for _, filetype in ipairs(nvim_bundled_started_here) do
  add_treesitter_filetype(filetype)
end
for _, filetype in ipairs(nvim_bundled_autostarted_filetypes) do
  add_treesitter_filetype(filetype)
end

local function runtime_syntax_for_filetype(buf, filetype)
  if filetype == "" or not filetype:match("^[%w_.-]+$") then
    return nil
  end

  local current = vim.bo[buf].syntax
  if current ~= "" then
    return current
  end

  if #vim.api.nvim_get_runtime_file("syntax/" .. filetype .. ".vim", false) > 0 then
    return filetype
  end

  return nil
end

local function update_installed_parsers_for_build()
  local nvim_treesitter = require("nvim-treesitter")
  if type(nvim_treesitter.update) ~= "function" then
    error("nvim-treesitter update API is unavailable; restore the locked plugin and retry", 0)
  end

  -- :TSUpdate is also a Lazy command trigger: loading the plugin runs config(),
  -- whose interactive path starts the full declared-parser install without
  -- waiting, and the command's own update task is asynchronous too. Lazy can
  -- therefore mark the build complete while compilers are still publishing
  -- parser files, then let a cold setup start Phase 4 against unfinished state.
  -- Use the upstream waitable API directly (without the command trigger) and
  -- serialize it so no parser build can outlive the plugin-restore boundary.
  local task = nvim_treesitter.update(nil, { max_jobs = 1, summary = true })
  if type(task) ~= "table" or type(task.wait) ~= "function" then
    error("nvim-treesitter parser update did not return a waitable task", 0)
  end
  if task:wait(900000) ~= true then
    error("nvim-treesitter parser update failed; see the parser build errors above", 0)
  end
end

return {
  {
    "nvim-treesitter/nvim-treesitter",
    branch = "main",
    build = update_installed_parsers_for_build,
    cmd = { "TSInstall", "TSInstallFromGrammar", "TSUpdate", "TSUninstall", "TSLog" },
    event = { "BufReadPre", "BufNewFile" },
    config = function()
      local nvim_treesitter = require("nvim-treesitter")
      local sync_install = vim.env.DOTFILES_TREESITTER_SYNC_INSTALL == "1"
      local headless = #vim.api.nvim_list_uis() == 0
      local install_requested = sync_install or not headless

      local function report_install_problem(message)
        if sync_install then
          error(message)
        end
        vim.schedule(function()
          vim.notify(message, vim.log.levels.WARN)
        end)
      end

      -- Every nvim-treesitter cleanup is constrained to stdpath("data"). A
      -- successful vim.fn.delete() return is not sufficient proof: verify the
      -- path is absent as well, because permission and partial directory
      -- failures can leave executable parser/query payloads behind.
      local managed_data_root = vim.fs.normalize(vim.fn.stdpath("data")):gsub("/$", "")
      local managed_data_prefix = managed_data_root .. "/"
      local function checked_delete_managed(path, mode)
        local deleted, delete_error = require("util.checked_delete").managed(path, mode, managed_data_root)
        if not deleted then
          return "Tree-sitter " .. delete_error
        end
        return nil
      end

      local function report_cleanup_failures(failures)
        if #failures > 0 then
          report_install_problem(table.concat(failures, "; "))
        end
      end

      local bundled_set = {}
      for _, lang in ipairs(nvim_bundled_parsers) do
        bundled_set[lang] = true
      end
      local highlight_query_required_set = {}
      for _, lang in ipairs(treesitter_parsers) do
        highlight_query_required_set[lang] = true
      end

      local function normalized_parser_dependencies()
        local config_ok, treesitter_config = pcall(require, "nvim-treesitter.config")
        if not config_ok or type(treesitter_config.norm_languages) ~= "function" then
          return nil, "nvim-treesitter.config.norm_languages is unavailable; cannot resolve parser dependencies"
        end

        local norm_ok, normalized_or_err =
          pcall(treesitter_config.norm_languages, treesitter_parsers, { unsupported = true })
        if not norm_ok or type(normalized_or_err) ~= "table" then
          return nil, "nvim-treesitter parser dependency resolution failed: " .. tostring(normalized_or_err)
        end

        local install_parsers = {}
        local install_parser_set = {}
        for _, parser in ipairs(normalized_or_err) do
          add_unique(install_parsers, install_parser_set, parser)
        end
        return install_parsers
      end

      local function remove_incomplete_parser_installs(parser_dependencies)
        local config_ok, treesitter_config = pcall(require, "nvim-treesitter.config")
        local parsers_ok, parser_configs = pcall(require, "nvim-treesitter.parsers")
        if not config_ok or not parsers_ok or type(treesitter_config.get_install_dir) ~= "function" then
          return
        end

        local parser_dir = treesitter_config.get_install_dir("parser")
        local parser_info_dir = treesitter_config.get_install_dir("parser-info")
        local query_dir = treesitter_config.get_install_dir("queries")
        local cleanup_failures = {}
        for _, parser in ipairs(parser_dependencies or {}) do
          local parser_config = parser_configs[parser]
          local has_runtime_queries = #vim.api.nvim_get_runtime_file("queries/" .. parser, false) > 0
          if not bundled_set[parser] and parser_config and parser_config.install_info and has_runtime_queries then
            local parser_path = vim.fs.joinpath(parser_dir, parser .. ".so")
            local parser_info_path = vim.fs.joinpath(parser_info_dir, parser .. ".revision")
            local query_path = vim.fs.joinpath(query_dir, parser)
            local has_query_dir = vim.uv.fs_stat(query_path)
            local has_required_highlight_query = not highlight_query_required_set[parser]
              or vim.uv.fs_stat(vim.fs.joinpath(query_path, "highlights.scm"))
            if vim.uv.fs_stat(parser_path) and (not has_query_dir or not has_required_highlight_query) then
              for _, cleanup in ipairs({
                { parser_path },
                { parser_info_path },
                { query_path, "rf" },
              }) do
                local cleanup_error = checked_delete_managed(cleanup[1], cleanup[2])
                if cleanup_error then
                  table.insert(cleanup_failures, cleanup_error)
                end
              end
            end
          end
        end
        report_cleanup_failures(cleanup_failures)
      end

      -- Purge any stale nvim-treesitter parser for a Neovim-bundled language.
      -- Excluding them from the install list stops FUTURE installs, but a
      -- `parser/<lang>.so` left from an older config -- or restored from a CI
      -- cache (the e2e jobs cache the lazy plugin + site dirs) -- still OVERRIDES
      -- Neovim's matched built-in parser via the runtimepath, re-creating the
      -- exact query/parser mismatch this whole change prevents (E5113). Runs
      -- before the FileType autocmd (plugin loads on BufReadPre/BufNewFile, ahead
      -- of FileType), so the first bundled buffer already sees the built-in.
      --
      -- CRITICAL: delete ONLY nvim-treesitter-managed copies, never Neovim's
      -- own. Neovim ships its bundled parsers as real `.so` files under the nvim
      -- install prefix (e.g. <prefix>/lib/nvim/parser/lua.so) and that dir is on
      -- the runtimepath -- so an unscoped delete would wipe Neovim's built-in
      -- parsers. nvim-treesitter installs only under stdpath('data') (its
      -- `site/parser`, `site/queries`, and lazy plugin dir all live there); the
      -- built-in prefix does not. So scope every delete to stdpath('data').
      local function purge_managed_bundled_files(pattern)
        local cleanup_failures = {}
        for _, path in ipairs(vim.api.nvim_get_runtime_file(pattern, true)) do
          local name = vim.fn.fnamemodify(path, ":t"):gsub("%.so$", ""):gsub("%.revision$", "")
          if bundled_set[name] and vim.startswith(vim.fs.normalize(path), managed_data_prefix) then
            local cleanup_error = checked_delete_managed(path)
            if cleanup_error then
              table.insert(cleanup_failures, cleanup_error)
            end
          end
        end
        report_cleanup_failures(cleanup_failures)
      end

      local function managed_query_install_dir()
        local config_ok, treesitter_config = pcall(require, "nvim-treesitter.config")
        if config_ok and type(treesitter_config.get_install_dir) == "function" then
          local dir_ok, dir = pcall(treesitter_config.get_install_dir, "queries")
          if dir_ok and type(dir) == "string" and dir ~= "" then
            return dir
          end
        end
        return vim.fs.joinpath(vim.fn.stdpath("data"), "site", "queries")
      end

      local function purge_managed_bundled_query_dirs()
        local query_dir = vim.fs.normalize(managed_query_install_dir())
        if not vim.startswith(query_dir .. "/", managed_data_prefix) then
          return
        end
        local cleanup_failures = {}
        for _, lang in ipairs(nvim_bundled_parsers) do
          local path = vim.fs.joinpath(query_dir, lang)
          if vim.fn.isdirectory(path) == 1 then
            local cleanup_error = checked_delete_managed(path, "rf")
            if cleanup_error then
              table.insert(cleanup_failures, cleanup_error)
            end
          end
        end
        report_cleanup_failures(cleanup_failures)
      end

      purge_managed_bundled_files("parser/*.so")
      -- Also drop nvim-treesitter's install bookkeeping so it does not believe a
      -- bundled language is still installed.
      purge_managed_bundled_files("parser-info/*.revision")
      -- Upstream dependencies can also install query directories for bundled
      -- languages (for example `cpp` requires `c`). Those query overrides are
      -- managed artifacts too; remove them with the parser files.
      purge_managed_bundled_query_dirs()

      -- nvim-treesitter `main` shells out to the `tree-sitter` CLI to compile
      -- each parser. When the CLI is not on PATH (nvim launched from a shell
      -- that never sourced brew shellenv, or before setup finished installing
      -- it), main emits a separate ENOENT error for EVERY parser. Guard on the
      -- CLI so a missing toolchain surfaces ONE actionable message instead of a
      -- wall of errors. Never auto-install from an ordinary headless process:
      -- Lazy restore, Mason, and smoke validators otherwise launch asynchronous
      -- compiler work outside Phase 4. setup forces the sync flag only for its
      -- explicit parser phase; UI sessions retain the interactive async path.
      if install_requested and vim.fn.executable("tree-sitter") == 1 then
        if type(nvim_treesitter.install) == "function" then
          local install_opts = nil
          if sync_install then
            -- Bootstrap/setup must be deterministic. nvim-treesitter's default
            -- parallelism is optimized for interactive speed, but CI restores
            -- parser caches across runs and can expose temp-dir races during a
            -- cold sync install. Keep interactive installs fast; serialize the
            -- proof path.
            install_opts = { max_jobs = 1, summary = true }
          end
          local parser_dependencies, parser_dependencies_err = normalized_parser_dependencies()
          if not parser_dependencies then
            report_install_problem(
              parser_dependencies_err .. ". Run :Lazy! restore and :TSUpdate after the plugin checkout is fixed."
            )
            parser_dependencies = treesitter_parsers
          end
          remove_incomplete_parser_installs(parser_dependencies)
          local ok, task_or_err = pcall(nvim_treesitter.install, treesitter_parsers, install_opts)
          if not ok then
            report_install_problem(
              "nvim-treesitter parser auto-install failed: "
                .. tostring(task_or_err)
                .. ". Run :Lazy! restore and :TSUpdate after the toolchain is fixed."
            )
          elseif sync_install then
            if type(task_or_err) ~= "table" or type(task_or_err.wait) ~= "function" then
              report_install_problem(
                "nvim-treesitter install did not return a waitable task; cannot prove parser bootstrap"
              )
            else
              local install_ok = task_or_err:wait(900000)
              purge_managed_bundled_files("parser/*.so")
              purge_managed_bundled_files("parser-info/*.revision")
              purge_managed_bundled_query_dirs()
              if install_ok ~= true then
                report_install_problem("nvim-treesitter parser install failed; see the parser build errors above")
              end
            end
          end
        else
          -- A stale local lazy.nvim checkout can still be on nvim-treesitter's
          -- frozen master branch even after this repo moved to the main rewrite.
          -- That branch lacks require("nvim-treesitter").install. Do not let the
          -- installer API mismatch abort the FileType autocmd below; existing
          -- parsers can still highlight buffers while :Lazy! restore repairs the
          -- plugin checkout.
          report_install_problem(
            "nvim-treesitter main API is missing; run :Lazy! restore, then :TSUpdate. "
              .. "Existing parsers will still be started when available."
          )
        end
      elseif install_requested then
        report_install_problem(
          "nvim-treesitter: 'tree-sitter' CLI not found on PATH; parsers were not compiled. "
            .. "Install it (macOS: brew install tree-sitter-cli; Linux/WSL: run the dotfiles setup; "
            .. "Windows: install-deps.ps1 -All), then run :TSUpdate."
        )
      end

      for parser, filetypes in pairs(parser_filetype_aliases) do
        vim.treesitter.language.register(parser, filetypes)
      end

      vim.api.nvim_create_autocmd("FileType", {
        group = vim.api.nvim_create_augroup("DotfilesTreesitter", { clear = true }),
        pattern = "*",
        callback = function(args)
          local filetype = vim.bo[args.buf].filetype
          local syntax = runtime_syntax_for_filetype(args.buf, filetype)
          local ok = true
          if treesitter_filetype_set[filetype] then
            ok = pcall(vim.treesitter.start, args.buf)
          end
          if syntax then
            vim.bo[args.buf].syntax = syntax
          end
          if ok and treesitter_filetype_set[filetype] then
            -- nvim-treesitter main removed the legacy indent module; use its
            -- documented indent expression instead when the main API is present.
            if type(nvim_treesitter.indentexpr) == "function" then
              vim.bo[args.buf].indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
            end
          end
        end,
      })

      -- EditQuery is auto-defined by treesitter on some versions and not
      -- others; deleting unconditionally throws on fresh installs.
      pcall(vim.api.nvim_del_user_command, "EditQuery")
    end,
  },
}
