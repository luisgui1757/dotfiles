describe("clipboard provider warning", function()
  local options = require("vim-options")
  local original_clipboard
  local original_executable
  local original_notify
  local original_environment

  before_each(function()
    original_clipboard = vim.g.clipboard
    original_executable = vim.fn.executable
    original_notify = vim.notify
    original_environment = { DISPLAY = vim.env.DISPLAY, WAYLAND_DISPLAY = vim.env.WAYLAND_DISPLAY, TMUX = vim.env.TMUX }
  end)

  after_each(function()
    vim.g.clipboard = original_clipboard
    vim.fn.executable = original_executable
    vim.notify = original_notify
    for _, key in ipairs({ "DISPLAY", "WAYLAND_DISPLAY", "TMUX" }) do
      vim.env[key] = original_environment[key]
    end
  end)

  for _, case in ipairs({
    { "warns for headless installed display helpers", {}, { "xclip", "wl-copy", "wl-paste" }, true },
    { "accepts both Wayland commands in a Wayland session", { WAYLAND_DISPLAY = "fixture" }, { "wl-copy", "wl-paste" }, false },
    { "warns for a missing Wayland paste command", { WAYLAND_DISPLAY = "fixture" }, { "wl-copy" }, true },
    { "accepts X11 helpers only with a display", { DISPLAY = ":fixture" }, { "xclip" }, false },
    { "accepts an existing clipboard bridge", {}, { "win32yank.exe" }, false },
    { "accepts an active tmux transport", { TMUX = "fixture" }, { "tmux" }, false },
    { "warns for tmux merely installed outside a session", {}, { "tmux" }, true },
  }) do
    it(case[1], function()
      vim.g.clipboard = false -- Explicit builtin discovery, not a custom provider.
      for _, key in ipairs({ "DISPLAY", "WAYLAND_DISPLAY", "TMUX" }) do
        vim.env[key] = case[2][key]
      end
      vim.fn.executable = function(name)
        return vim.tbl_contains(case[3], name) and 1 or 0
      end
      local notified = false
      vim.notify = function()
        notified = true
      end
      options._warn_if_missing_clipboard_provider()
      assert.are.equal(case[4], notified)
    end)
  end

  it("warns when no provider executable is available", function()
    local notifications = {}
    vim.g.clipboard = nil
    vim.fn.executable = function()
      return 0
    end
    vim.notify = function(message, level)
      table.insert(notifications, { message = message, level = level })
    end

    options._warn_if_missing_clipboard_provider()

    assert.are.equal(1, #notifications)
    assert.is_truthy(notifications[1].message:match("clipboard: no provider on PATH"))
    assert.are.equal(vim.log.levels.WARN, notifications[1].level)
  end)

  it("honors vim.g.clipboard as an escape hatch", function()
    local executable_calls = 0
    local notified = false
    vim.g.clipboard = { name = "custom" }
    vim.fn.executable = function()
      executable_calls = executable_calls + 1
      return 0
    end
    vim.notify = function()
      notified = true
    end

    options._warn_if_missing_clipboard_provider()

    assert.are.equal(0, executable_calls)
    assert.is_false(notified)
  end)
end)
