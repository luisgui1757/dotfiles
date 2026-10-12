-- Native CI: every configured LSP gets a fresh process with a file on argv.
-- The parent runs with --clean so it cannot pre-load Mason or supply its PATH.
local ok, err = pcall(function()
  local script = vim.fn.resolve(debug.getinfo(1, "S").source:sub(2))
  local repo = vim.fn.fnamemodify(script, ":h:h:h")
  local matrix = dofile(repo .. "/tests/nvim/language_matrix.lua")
  local projects = dofile(repo .. "/tests/nvim/lsp_smoke_projects.lua")
  local root = vim.fn.tempname()
  local seen = {}
  local timeout = vim.fn.has("win32") == 1 and 90000 or 45000
  local passed, failure = pcall(function()
    for index, row in ipairs(matrix) do
      if row.lsp and not seen[row.lsp] then
        seen[row.lsp] = true
        if row.lsp_gated and vim.fn.has("win32") ~= 1 then
          io.stdout:write("direct startup: " .. row.lsp .. " skipped off Windows\n")
        else
          local file = projects.prepare({
            root = root,
            fixtures = repo .. "/tests/nvim/fixtures",
            fixture = row.fixture,
            lsp = row.lsp,
            index = index,
          })
          local probe = string.format(
            [[local attached = vim.wait(%d, function()
              for _, client in ipairs(vim.lsp.get_clients({bufnr=0, name=%q})) do
                if client.initialized then return true end
              end
              return false
            end, 100)
            if attached and vim.bo.filetype == %q then
              io.stdout:write(%q)
              vim.cmd('qa!')
            else
              io.stderr:write(%q .. vim.api.nvim_exec2('messages', {output=true}).output .. '\n')
              vim.cmd('cquit 1')
            end]],
            timeout,
            row.lsp,
            row.filetype,
            "direct startup: " .. row.lsp .. " initialized\n",
            "direct startup failed: " .. row.lsp .. "\n"
          )
          local result = vim
            .system({ vim.v.progpath, "--headless", "-i", "NONE", file, "-c", "lua " .. probe }, { text = true })
            :wait(timeout + 15000)
          io.stdout:write(result.stdout or "")
          assert(result.code == 0, (result.stderr or "") .. " (exit " .. tostring(result.code) .. ")")
        end
      end
    end
  end)
  local removed = vim.fn.delete(root, "rf")
  assert(passed, failure)
  assert(removed == 0, "could not remove the private LSP projects")
end)

if not ok then
  io.stderr:write("LSP direct-startup regression: " .. tostring(err) .. "\n")
  vim.cmd("cquit 1")
else
  vim.cmd("qa!")
end
