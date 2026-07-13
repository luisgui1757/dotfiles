return {
  "stevearc/conform.nvim",
  event = { "BufReadPre", "BufNewFile" },
  cmd = { "ConformInfo" },
  opts = {
    formatters_by_ft = {
      lua = { "stylua" },
      python = { "ruff_fix", "ruff_format" },
      cpp = { "clang_format" },
      c = { "clang_format" },
      rust = { "rustfmt" },
      cmake = { "gersemi" },
      sh = { "shfmt" },
      bash = { "shfmt" },
      zsh = { "shfmt" },
      javascript = { "prettier" },
      javascriptreact = { "prettier" },
      typescript = { "prettier" },
      typescriptreact = { "prettier" },
      html = { "prettier" },
      css = { "prettier" },
      scss = { "prettier" },
      graphql = { "prettier" },
      vue = { "prettier" },
      json = { "prettier" },
      jsonc = { "prettier" },
      json5 = { "prettier" },
      yaml = { "prettier" },
      markdown = { "prettier" },
      -- ps1 has no external formatter here; LSP fallback only formats it if
      -- powershell_es advertises formatting on that host.
    },
    format_on_save = function(bufnr)
      if vim.b[bufnr].skip_format_on_save then
        return
      end
      return { timeout_ms = 10000, lsp_format = "fallback" }
    end,
    formatters = {
      prettier = {
        append_args = function(_, ctx)
          local ft = vim.bo[ctx.buf].filetype
          if ft == "json" or ft == "jsonc" or ft == "json5" then
            return { "--trailing-comma", "none" }
          end
          return {}
        end,
      },
      shfmt = { prepend_args = { "-i", "2", "-ci" } },
    },
  },
  keys = {
    {
      "<leader>gf",
      function()
        require("conform").format({ async = true, lsp_format = "fallback" })
      end,
      desc = "Format buffer",
      mode = { "n", "v" },
    },
  },
}
