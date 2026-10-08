local M = {}

-- Indentation
vim.opt.expandtab = true
vim.opt.tabstop = 2
vim.opt.shiftwidth = 2
vim.opt.softtabstop = 2

-- Line numbers
vim.opt.number = true
vim.opt.relativenumber = true

-- Modern defaults
vim.opt.termguicolors = true
vim.opt.signcolumn = "yes"
vim.opt.undofile = true
vim.opt.updatetime = 250
vim.opt.timeoutlen = 400
vim.opt.splitright = true
vim.opt.splitbelow = true
vim.opt.scrolloff = 16
vim.opt.ignorecase = true
vim.opt.smartcase = true
vim.opt.confirm = true

-- Mouse intentionally disabled. The `mouse = ""` line alone is enough on
-- a clean nvim install (default is "nvi" in 0.11 -- left-click places the
-- cursor when enabled), but the terminal/editor
-- input pipeline has multiple layers that can flip mouse handling on (a
-- Lazy-loaded plugin, the :terminal mouse pass-through, etc.). Belt-and-
-- braces: even if `mouse` ever gets flipped back to nvi, the other three
-- options keep wheel scroll inert, focus inert, and motion events off.
-- Use the documented zero-count form for mousescroll (NOT the empty
-- string -- that is not a valid value).
vim.opt.mouse = ""
vim.opt.mousescroll = "ver:0,hor:0"
vim.opt.mousefocus = false
vim.opt.mousemoveevent = false

-- Whitespace: tabs as ▸ , leading spaces as · (so indentation is visible
-- without polluting in-line spaces), trailing spaces as ·, nbsp marker.
-- `lead` (Neovim 0.10+) renders only leading whitespace, avoiding the
-- distraction the global `space = "·"` setting used to cause.
vim.opt.list = true
vim.opt.listchars = { tab = "▸ ", lead = "·", trail = "·", nbsp = "␣" }

-- Cross-platform system clipboard.
-- Providers follow the active display, terminal or an existing clipboard bridge.
vim.opt.clipboard = "unnamedplus"

-- An installed display helper is not usable without its display session. Warn once
-- on a delayed timer so the message lands AFTER the colorscheme + lualine
-- load (otherwise it gets buried in startup output).
function M._warn_if_missing_clipboard_provider()
  -- Escape hatch: a user-defined vim.g.clipboard provider overrides
  -- nvim's discovery chain. Don't warn in that case.
  if vim.g.clipboard ~= nil and vim.g.clipboard ~= false then
    return
  end
  local function available(...)
    for _, command in ipairs({ ... }) do
      if vim.fn.executable(command) ~= 1 then
        return false
      end
    end
    return true
  end
  local function session(name)
    return vim.env[name] ~= nil and vim.env[name] ~= ""
  end
  if
    available("pbcopy", "pbpaste")
    or available("win32yank.exe")
    or available("clip", "powershell")
    or (session("WAYLAND_DISPLAY") and available("wl-copy", "wl-paste"))
    or (session("DISPLAY") and (available("xclip") or available("xsel")))
    or (session("TMUX") and available("tmux"))
  then
    return
  end
  vim.notify(
    "clipboard: no provider on PATH for this session.\n"
      .. "Run dotfiles and select Neovim to install its clipboard helpers.\n"
      .. "A headless session needs a terminal clipboard transport or an existing bridge.\n"
      .. "See :help clipboard-osc52, or set vim.g.clipboard for a custom provider.",
    vim.log.levels.WARN
  )
end

vim.api.nvim_create_autocmd("VimEnter", {
  once = true,
  callback = function()
    vim.defer_fn(M._warn_if_missing_clipboard_provider, 500)
  end,
})

-- Toggle relative line numbers
vim.keymap.set("n", "<leader>lt", function()
  vim.wo.relativenumber = not vim.wo.relativenumber
end, { noremap = true, silent = true })

-- Clear search highlight
vim.keymap.set("n", "<leader>ch", ":nohlsearch<CR>", { noremap = true, silent = true })

-- Toggle quickfix window
vim.keymap.set("n", "<leader>qq", ":cclose<CR>", { noremap = true, silent = true })

-- Transparent backgrounds for rose-pine over translucent terminals (Ghostty/WT)
vim.api.nvim_create_autocmd("ColorScheme", {
  pattern = "*",
  callback = function()
    vim.cmd("highlight Normal ctermbg=NONE guibg=NONE")
    vim.cmd("highlight NormalNC ctermbg=NONE guibg=NONE")
    vim.cmd("highlight SignColumn ctermbg=NONE guibg=NONE")
    vim.cmd("highlight EndOfBuffer ctermbg=NONE guibg=NONE")
  end,
})

-- Alt-based window navigation
vim.keymap.set("n", "<A-h>", "<C-w>h", { noremap = true, silent = true })
vim.keymap.set("n", "<A-j>", "<C-w>j", { noremap = true, silent = true })
vim.keymap.set("n", "<A-k>", "<C-w>k", { noremap = true, silent = true })
vim.keymap.set("n", "<A-l>", "<C-w>l", { noremap = true, silent = true })

-- :WNF — Write Without Formatting. Sets a buffer-local flag that
-- conform.nvim's format_on_save checks; cleared after the write so
-- the *next* :w formats normally.
vim.api.nvim_create_user_command("WNF", function()
  vim.b.skip_format_on_save = true
  vim.cmd.write()
end, { desc = "Write without running formatters" })
-- Expand `wnf` to `WNF` ONLY when it is the entire command (command position),
-- not when the word 'wnf' appears as an argument (e.g. `:e wnf.txt`). A bare
-- `cnoreabbrev wnf WNF` is a global cmdline abbreviation that fires anywhere on
-- the cmdline.
vim.cmd([[cnoreabbrev <expr> wnf (getcmdtype() == ':' && getcmdline() ==# 'wnf') ? 'WNF' : 'wnf']])

-- Augroup so re-sourcing this file (tests, :luafile) clears the prior autocmd
-- instead of stacking a duplicate that runs the callback twice per write.
vim.api.nvim_create_autocmd("BufWritePost", {
  group = vim.api.nvim_create_augroup("WnfClearSkipFlag", { clear = true }),
  callback = function(args)
    vim.b[args.buf].skip_format_on_save = nil
  end,
})

vim.filetype.add({
  filename = {
    [".curlrc"] = "conf",
  },
  extension = {
    curlrc = "conf",
  },
})

-- Drop nvim's default :EditQuery user command. We don't use the
-- treesitter query editor, and keeping it around made :E<Enter>
-- resolve to :EditQuery instead of netrw's :Explore.
pcall(vim.api.nvim_del_user_command, "EditQuery")

-- :E opens netrw at the cwd; :E <path> edits that file/dir.
vim.api.nvim_create_user_command("E", function(opts)
  if opts.args == "" then
    vim.cmd("Explore")
  else
    vim.cmd("edit " .. vim.fn.fnameescape(opts.args))
  end
end, { nargs = "?", complete = "file", desc = "Open netrw, or edit the given file" })

return M
