local treesitter_parsers = {
  "c",
  "cpp",
  "cmake",
  "lua",
  "python",
  "rust",
  "bash",
  "powershell",
  "json",
  -- No jsonc here: nvim-treesitter main has no jsonc grammar (it warns that the
  -- language is unsupported). jsonc files use Neovim built-in syntax and still
  -- get prettier (conform) + the json LSP. Do NOT alias the json parser to
  -- jsonc -- the json grammar errors on slash-slash comments. (Keep this comment
  -- quote-free: treesitter_spec extracts quoted strings from this block.)
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

local parser_only = {
  markdown_inline = true,
}

local parser_filetype_aliases = {
  bash = { "sh" },
  powershell = { "ps1" },
  vimdoc = { "help" },
}

local treesitter_filetypes = {}
for _, parser in ipairs(treesitter_parsers) do
  if not parser_only[parser] then
    table.insert(treesitter_filetypes, parser)
  end
  for _, filetype in ipairs(parser_filetype_aliases[parser] or {}) do
    table.insert(treesitter_filetypes, filetype)
  end
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

      -- nvim-treesitter `main` shells out to the `tree-sitter` CLI to compile
      -- each parser. When the CLI is not on PATH (nvim launched from a shell
      -- that never sourced brew shellenv, or before setup finished installing
      -- it), main emits a separate ENOENT error for EVERY parser. Guard on the
      -- CLI so a missing toolchain surfaces ONE actionable message instead of a
      -- wall of errors. setup installs it (macOS: brew; Linux/WSL: pinned
      -- release in install-deps.sh; Windows: install-deps.ps1 -All) and re-runs
      -- the sync with it on PATH; recompile in-session after fixing PATH with
      -- :TSUpdate.
      if vim.fn.executable("tree-sitter") == 1 then
        nvim_treesitter.install(treesitter_parsers)
      else
        vim.schedule(function()
          vim.notify(
            "nvim-treesitter: 'tree-sitter' CLI not found on PATH; parsers were not compiled. "
              .. "Install it (macOS: brew install tree-sitter; Linux/WSL: run the dotfiles setup; "
              .. "Windows: install-deps.ps1 -All), then run :TSUpdate.",
            vim.log.levels.WARN
          )
        end)
      end

      for parser, filetypes in pairs(parser_filetype_aliases) do
        vim.treesitter.language.register(parser, filetypes)
      end

      vim.api.nvim_create_autocmd("FileType", {
        group = vim.api.nvim_create_augroup("DotfilesTreesitter", { clear = true }),
        pattern = treesitter_filetypes,
        callback = function(args)
          local ok = pcall(vim.treesitter.start, args.buf)
          if ok then
            -- nvim-treesitter main removed the legacy indent module; use its
            -- documented indent expression instead.
            vim.bo[args.buf].indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
          end
        end,
      })

      -- EditQuery is auto-defined by treesitter on some versions and not
      -- others; deleting unconditionally throws on fresh installs.
      pcall(vim.api.nvim_del_user_command, "EditQuery")
    end,
  },
}
