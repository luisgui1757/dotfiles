return {
  {
    "MeanderingProgrammer/render-markdown.nvim",
    ft = { "markdown" },
    dependencies = {
      "nvim-treesitter/nvim-treesitter",
      "nvim-tree/nvim-web-devicons",
    },
    ---@module 'render-markdown'
    ---@type render.md.UserConfig
    opts = {
      file_types = { "markdown" },
      completions = { lsp = { enabled = true } },

      heading = {
        sign = true,
        position = "inline",
        width = { "full", "full", "block" },
        min_width = { 70, 60, 40 },
        left_pad = { 1, 1, 0 },
        right_pad = 1,
        border = { true, true, false },
        border_virtual = true,
        border_prefix = true,
        icons = { "󰉫 ", "󰉬 ", "󰉭 ", "󰉮 ", "󰉯 ", "󰉰 " },
        backgrounds = {
          "RenderMarkdownH1Bg",
          "RenderMarkdownH2Bg",
          "RenderMarkdownH3Bg",
          "RenderMarkdownH4Bg",
          "RenderMarkdownH5Bg",
          "RenderMarkdownH6Bg",
        },
        foregrounds = {
          "RenderMarkdownH1",
          "RenderMarkdownH2",
          "RenderMarkdownH3",
          "RenderMarkdownH4",
          "RenderMarkdownH5",
          "RenderMarkdownH6",
        },
      },

      code = {
        sign = false,
        style = "full",
        width = "block",
        min_width = 70,
        left_pad = 1,
        right_pad = 1,
        border = "thin",
        language = true,
        position = "left",
        language_icon = true,
        language_name = true,
        language_info = true,
        language_pad = 1,
        highlight = "RenderMarkdownCode",
        highlight_info = "RenderMarkdownCodeInfo",
        highlight_border = "RenderMarkdownCodeBorder",
        highlight_fallback = "RenderMarkdownCodeFallback",
        highlight_inline = "RenderMarkdownCodeInline",
        inline_pad = 1,
      },

      checkbox = {
        enabled = true,
        right_pad = 1,
        unchecked = { icon = "󰄱 ", highlight = "RenderMarkdownUnchecked" },
        checked = { icon = "󰱒 ", highlight = "RenderMarkdownChecked", scope_highlight = "@markup.strikethrough" },
        custom = {
          todo = { raw = "[-]", rendered = "󰥔 ", highlight = "RenderMarkdownTodo" },
          important = { raw = "[!]", rendered = "󰀦 ", highlight = "RenderMarkdownError" },
          question = { raw = "[?]", rendered = "󰘥 ", highlight = "RenderMarkdownHint" },
          cancelled = { raw = "[/]", rendered = "󰜺 ", highlight = "@markup.strikethrough" },
        },
      },

      bullet = {
        icons = { "●", "○", "◆", "◇" },
        right_pad = 1,
        highlight = "RenderMarkdownBullet",
      },

      dash = {
        icon = "─",
        width = "full",
        highlight = "RenderMarkdownDash",
      },

      quote = {
        icon = "▌",
        repeat_linebreak = true,
        highlight = {
          "RenderMarkdownQuote",
          "RenderMarkdownQuote",
          "RenderMarkdownQuote",
          "RenderMarkdownQuote",
          "RenderMarkdownQuote",
          "RenderMarkdownQuote",
        },
      },

      latex = {
        enabled = true,
        converter = "latex2text",
        highlight = "RenderMarkdownMath",
        position = "center",
        top_pad = 0,
        bottom_pad = 0,
      },

      -- Obsidian-style callouts (> [!NOTE], > [!WARNING], etc.) — these
      -- come built-in; we just confirm the common ones are enabled.
      callout = {
        note = { raw = "[!NOTE]", rendered = "󰋽 Note", highlight = "RenderMarkdownInfo", quote_icon = "▌" },
        tip = { raw = "[!TIP]", rendered = "󰌶 Tip", highlight = "RenderMarkdownSuccess", quote_icon = "▌" },
        important = {
          raw = "[!IMPORTANT]",
          rendered = "󰅾 Important",
          highlight = "RenderMarkdownHint",
          quote_icon = "▌",
        },
        warning = {
          raw = "[!WARNING]",
          rendered = "󰀪 Warning",
          highlight = "RenderMarkdownWarn",
          quote_icon = "▌",
        },
        caution = {
          raw = "[!CAUTION]",
          rendered = "󰳦 Caution",
          highlight = "RenderMarkdownError",
          quote_icon = "▌",
        },
        abstract = {
          raw = "[!ABSTRACT]",
          rendered = "󰨸 Abstract",
          highlight = "RenderMarkdownInfo",
          quote_icon = "▌",
        },
        todo = { raw = "[!TODO]", rendered = "󰗡 Todo", highlight = "RenderMarkdownInfo", quote_icon = "▌" },
        question = {
          raw = "[!QUESTION]",
          rendered = "󰘥 Question",
          highlight = "RenderMarkdownWarn",
          quote_icon = "▌",
        },
        quote = { raw = "[!QUOTE]", rendered = "󱆨 Quote", highlight = "RenderMarkdownQuote", quote_icon = "▌" },
      },

      -- Links: render wikilinks (Obsidian syntax) as a small icon.
      link = {
        enabled = true,
        image = "󰥶 ",
        email = "󰀓 ",
        hyperlink = "󰌹 ",
        wiki = { icon = "󱗖 ", highlight = "RenderMarkdownWikiLink" },
      },

      pipe_table = {
        preset = "round",
        cell = "padded",
        padding = 1,
        min_width = 3,
        head = "RenderMarkdownTableHead",
        row = "RenderMarkdownTableRow",
        style = "full",
      },

      -- Rose Pine palette overrides for the rendered Markdown surfaces.
      -- The plugin's defaults look fine on most colorschemes; these
      -- pin our rose-pine variants so headings have consistent depth.
      win_options = {
        conceallevel = { default = vim.o.conceallevel, rendered = 2 },
        concealcursor = { default = vim.o.concealcursor, rendered = "" },
      },
    },
    config = function(_, opts)
      require("render-markdown").setup(opts)

      -- Rose Pine heading backgrounds, deepest at H1, fading out.
      -- iris=#c4a7e7, foam=#9ccfd8, rose=#ebbcba, gold=#f6c177,
      -- pine=#31748f, love=#eb6f92
      local hi = function(name, opts2)
        vim.api.nvim_set_hl(0, name, opts2)
      end
      hi("RenderMarkdownH1", { fg = "#c4a7e7", bold = true })
      hi("RenderMarkdownH2", { fg = "#9ccfd8", bold = true })
      hi("RenderMarkdownH3", { fg = "#ebbcba", bold = true })
      hi("RenderMarkdownH4", { fg = "#f6c177", bold = true })
      hi("RenderMarkdownH5", { fg = "#31748f", bold = true })
      hi("RenderMarkdownH6", { fg = "#eb6f92", bold = true })
      hi("RenderMarkdownH1Bg", { bg = "#2a1f3d" })
      hi("RenderMarkdownH2Bg", { bg = "#1f2e36" })
      hi("RenderMarkdownH3Bg", { bg = "#33252a" })
      hi("RenderMarkdownH4Bg", { bg = "#332a1f" })
      hi("RenderMarkdownH5Bg", { bg = "#1f262a" })
      hi("RenderMarkdownH6Bg", { bg = "#33222a" })
      hi("RenderMarkdownCode", { bg = "#1f1d2e" })
      hi("RenderMarkdownCodeInfo", { fg = "#9ccfd8", bg = "#1f1d2e", italic = true })
      hi("RenderMarkdownCodeBorder", { fg = "#6e6a86", bg = "#1f1d2e" })
      hi("RenderMarkdownCodeFallback", { fg = "#c4a7e7", bg = "#1f1d2e" })
      hi("RenderMarkdownCodeInline", { bg = "#26233a", fg = "#ebbcba" })
      hi("RenderMarkdownDash", { fg = "#6e6a86" })
      hi("RenderMarkdownBullet", { fg = "#f6c177" })
      hi("RenderMarkdownChecked", { fg = "#9ccfd8" })
      hi("RenderMarkdownUnchecked", { fg = "#6e6a86" })
      hi("RenderMarkdownTodo", { fg = "#f6c177" })
      hi("RenderMarkdownInfo", { fg = "#9ccfd8" })
      hi("RenderMarkdownSuccess", { fg = "#31748f" })
      hi("RenderMarkdownHint", { fg = "#c4a7e7" })
      hi("RenderMarkdownWarn", { fg = "#f6c177" })
      hi("RenderMarkdownError", { fg = "#eb6f92" })
      hi("RenderMarkdownQuote", { fg = "#ebbcba", italic = true })
      hi("RenderMarkdownTableHead", { fg = "#191724", bg = "#c4a7e7", bold = true })
      hi("RenderMarkdownTableRow", { fg = "#e0def4", bg = "#1f1d2e" })
      hi("RenderMarkdownMath", { fg = "#f6c177", bg = "#1f1d2e" })
      hi("RenderMarkdownWikiLink", { fg = "#c4a7e7", underline = true })
    end,
    keys = {
      { "<leader>mr", "<cmd>RenderMarkdown toggle<cr>", desc = "Toggle markdown rendering", ft = "markdown" },
    },
  },
}
