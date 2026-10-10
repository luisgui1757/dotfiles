-- markdown-preview.nvim removed deliberately. Its `cd app && npm install`
-- build step modifies app/yarn.lock inside the plugin clone, which then
-- permanently breaks `:Lazy update markdown-preview.nvim` ("You have local
-- changes ... please remove them to update"). render-markdown.nvim (see
-- markdown.lua) provides inline preview, and obsidian.nvim handles the
-- vault workflow -- a browser preview window isn't worth this maintenance
-- cost. If you ever need it again, see `previm/previm` (no build step) or
-- the precompiled `cd app && npx --yes yarn install --frozen-lockfile`
-- variant of the original.

return {
  {
    "epwalsh/obsidian.nvim",
    ft = { "markdown" },
    dependencies = { "nvim-lua/plenary.nvim" },
    -- Always provision the locked plugin, including in the installer's private
    -- HOME. Activate it only for an existing personal vault; setup creates paths.
    -- NOTES_VAULT selects an existing path; util/notes_path.lua has the defaults.
    config = function()
      local notes_path = require("util.notes_path").resolve()
      if vim.fn.isdirectory(notes_path) ~= 1 then
        return
      end
      require("obsidian").setup({
        workspaces = {
          { name = "notes", path = notes_path },
        },
        -- Obsidian's own UI stays disabled; render-markdown.nvim owns
        -- rendering everywhere (markdown.lua + CLAUDE.md invariant #11).
        ui = { enable = false },
      })
    end,
  },
}
