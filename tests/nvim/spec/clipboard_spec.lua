describe("clipboard provider warning", function()
  local options = require("vim-options")
  local original_clipboard
  local original_executable
  local original_notify

  before_each(function()
    original_clipboard = vim.g.clipboard
    original_executable = vim.fn.executable
    original_notify = vim.notify
  end)

  after_each(function()
    vim.g.clipboard = original_clipboard
    vim.fn.executable = original_executable
    vim.notify = original_notify
  end)

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
