return {
  {
    "mfussenegger/nvim-dap",
    keys = {
      {
        "<F5>",
        function()
          require("dap").continue()
        end,
        desc = "DAP continue",
      },
      {
        "<F10>",
        function()
          require("dap").step_over()
        end,
        desc = "DAP step over",
      },
      {
        "<F11>",
        function()
          require("dap").step_into()
        end,
        desc = "DAP step into",
      },
      {
        "<F12>",
        function()
          require("dap").step_out()
        end,
        desc = "DAP step out",
      },
      {
        "<leader>b",
        function()
          require("dap").toggle_breakpoint()
        end,
        desc = "DAP toggle breakpoint",
      },
    },
    dependencies = {
      "rcarriga/nvim-dap-ui",
      "nvim-neotest/nvim-nio",
    },
    config = function()
      local dap = require("dap")

      dap.adapters["pwa-node"] = {
        type = "server",
        host = "localhost",
        port = "${port}",
        executable = {
          command = "node",
          args = {
            vim.fn.stdpath("data") .. "/mason/packages/js-debug-adapter/js-debug/src/dapDebugServer.js",
            "${port}",
          },
        },
      }
      dap.adapters["pwa-chrome"] = dap.adapters["pwa-node"]

      dap.configurations.typescript = {
        {
          type = "pwa-chrome",
          request = "launch",
          name = "Launch Chrome",
          url = vim.env.DAP_LAUNCH_URL or "http://localhost:3000",
          webRoot = "${workspaceFolder}",
        },
      }
      dap.configurations.javascript = dap.configurations.typescript
    end,
  },
  {
    "rcarriga/nvim-dap-ui",
    lazy = true,
    dependencies = { "mfussenegger/nvim-dap", "nvim-neotest/nvim-nio" },
    config = function()
      local dap, dapui = require("dap"), require("dapui")
      dapui.setup()

      -- Modern dap listener pattern: attach/launch lifecycle keys.
      dap.listeners.before.attach.dapui_config = function()
        dapui.open()
      end
      dap.listeners.before.launch.dapui_config = function()
        dapui.open()
      end
      dap.listeners.before.event_terminated.dapui_config = function()
        dapui.close()
      end
      dap.listeners.before.event_exited.dapui_config = function()
        dapui.close()
      end
    end,
  },
}
