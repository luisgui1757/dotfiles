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

return {
  {
    "nvim-treesitter/nvim-treesitter",
    branch = "main",
    build = ":TSUpdate",
    cmd = { "TSInstall", "TSInstallFromGrammar", "TSUpdate", "TSUninstall", "TSLog" },
    event = { "BufReadPre", "BufNewFile" },
    config = function()
      local nvim_treesitter = require("nvim-treesitter")
      local sync_install = vim.env.DOTFILES_TREESITTER_SYNC_INSTALL == "1"

      local function report_install_problem(message)
        if sync_install then
          error(message)
        end
        vim.schedule(function()
          vim.notify(message, vim.log.levels.WARN)
        end)
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
      -- `site/parser` and the lazy plugin dir both live there); the built-in
      -- prefix does not. So scope every delete to stdpath('data').
      local bundled_set = {}
      for _, lang in ipairs(nvim_bundled_parsers) do
        bundled_set[lang] = true
      end
      local managed = vim.fs.normalize(vim.fn.stdpath("data")) .. "/"
      local function purge(pattern)
        for _, path in ipairs(vim.api.nvim_get_runtime_file(pattern, true)) do
          if bundled_set[vim.fn.fnamemodify(path, ":t:r")] and vim.startswith(vim.fs.normalize(path), managed) then
            pcall(vim.fn.delete, path)
          end
        end
      end
      purge("parser/*.so")
      -- Also drop nvim-treesitter's install bookkeeping so it does not believe a
      -- bundled language is still installed.
      purge("parser-info/*.revision")

      -- nvim-treesitter `main` shells out to the `tree-sitter` CLI to compile
      -- each parser. When the CLI is not on PATH (nvim launched from a shell
      -- that never sourced brew shellenv, or before setup finished installing
      -- it), main emits a separate ENOENT error for EVERY parser. Guard on the
      -- CLI so a missing toolchain surfaces ONE actionable message instead of a
      -- wall of errors. setup installs it (macOS: brew; Linux/WSL: pinned
      -- release in install-deps.sh; Windows: install-deps.ps1 -All) and then
      -- forces DOTFILES_TREESITTER_SYNC_INSTALL=1 so parser installation blocks
      -- on install(...):wait(...). Interactive sessions keep the async path.
      if vim.fn.executable("tree-sitter") == 1 then
        if type(nvim_treesitter.install) == "function" then
          local ok, task_or_err = pcall(nvim_treesitter.install, treesitter_parsers)
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
              local install_ok = task_or_err:wait(300000)
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
      else
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
