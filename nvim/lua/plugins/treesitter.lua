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
  "jsonc",
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
      nvim_treesitter.install(treesitter_parsers)

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
