return {
  {
    -- Folke which-key: after `timeoutlen` (400ms) with a pending prefix, a popup
    -- lists the keys that can follow. It only *displays* existing keymaps (no new
    -- bindings), so it never fights conform/telescope/gitsigns for a chord.
    -- Kept lazy (event = "VeryLazy") so it never adds to the startup budget --
    -- only rose-pine.lua is allowed to load eagerly (invariant 7).
    "folke/which-key.nvim",
    event = "VeryLazy",
    opts = {},
    keys = {
      {
        "<leader>?",
        function()
          require("which-key").show({ global = false })
        end,
        desc = "Buffer Local Keymaps (which-key)",
      },
    },
  },
}
