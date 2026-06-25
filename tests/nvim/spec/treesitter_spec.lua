local repo_root = _G.TEST_REPO_ROOT
local fh = assert(io.open(repo_root .. "/nvim/lua/plugins/treesitter.lua", "r"))
local src = fh:read("*a")
fh:close()

local function extract_string_list(name)
  local body = assert(src:match("local%s+" .. name .. "%s*=%s*%{(.-)%}\n"), "missing " .. name)
  local values = {}
  for value in body:gmatch('"([^"]+)"') do
    table.insert(values, value)
  end
  return values
end

local function assert_same_list(actual, expected)
  assert.are.equal(#expected, #actual)
  for i, value in ipairs(expected) do
    assert.are.equal(value, actual[i])
  end
end

describe("treesitter main migration", function()
  -- nvim-treesitter installs ONLY the languages Neovim does NOT bundle. The
  -- bundled set (c, lua, markdown, markdown_inline, query, vim, vimdoc) is
  -- deliberately excluded so Neovim's matched built-in parser+query is used --
  -- installing nvim-treesitter's would override the built-in and break the
  -- bundled query (e.g. lua highlights `operator:` -> "Invalid field name").
  local required = {
    "cpp",
    "cmake",
    "python",
    "rust",
    "bash",
    "zsh",
    "powershell",
    "javascript",
    "typescript",
    "tsx",
    "html",
    "css",
    "scss",
    "vue",
    "svelte",
    "astro",
    "graphql",
    "json",
    "json5",
    "yaml",
    "toml",
    "xml",
    "razor",
    "csv",
    "dockerfile",
    "hcl",
    "terraform",
    "make",
    "http",
    "sql",
    "proto",
    "prisma",
    "nix",
    "ini",
    "editorconfig",
    "git_config",
    "git_rebase",
    "gitignore",
    "ssh_config",
    "requirements",
    "fish",
    "nu",
    "just",
    "meson",
    "ninja",
    "nginx",
    "jq",
    "kdl",
    "desktop",
    "latex",
    "bibtex",
    "typst",
    "mermaid",
    "glsl",
    "wgsl",
    "ron",
    "go",
    "gomod",
    "gosum",
    "gowork",
    "c_sharp",
    "fsharp",
    "java",
    "kotlin",
    "scala",
    "php",
    "ruby",
    "perl",
    "swift",
    "zig",
    "asm",
    "nasm",
    "dart",
    "elixir",
    "erlang",
    "clojure",
    "haskell",
    "ocaml",
    "r",
    "solidity",
    "julia",
    "fortran",
    "pascal",
    "ada",
    "groovy",
    "matlab",
    "gleam",
    "qmljs",
    "bicep",
    "earthfile",
    "diff",
    "gitcommit",
  }

  it("pins nvim-treesitter to the main branch", function()
    assert.is_truthy(src:match('branch%s*=%s*"main"'), "nvim-treesitter must use the 0.12+ main branch")
  end)

  it("keeps the required parser list unchanged", function()
    assert_same_list(extract_string_list("treesitter_parsers"), required)
  end)

  it("excludes Neovim-bundled languages from the install list (canonical fix)", function()
    local installed = {}
    for _, p in ipairs(extract_string_list("treesitter_parsers")) do
      installed[p] = true
    end
    -- Languages Neovim 0.12 bundles a matched parser+query for. Installing
    -- nvim-treesitter's parser overrides the built-in and breaks the bundled
    -- query (e.g. lua highlights `operator:` -> E5113 "Invalid field name").
    for _, bundled in ipairs({ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" }) do
      assert.is_nil(installed[bundled], bundled .. " is Neovim-bundled; it must NOT be in treesitter_parsers")
    end
    -- c and vim are bundled but Neovim does not auto-start them, so we do.
    assert.is_truthy(
      src:match('nvim_bundled_started_here%s*=%s*%{ "c", "vim" %}'),
      "c and vim must be started via Neovim's matched built-in parser"
    )
  end)

  it("purges stale nvim-treesitter parser overrides for the bundled languages", function()
    -- Excluding the bundled langs stops future installs, but a parser/<lang>.so
    -- already on the runtimepath (older config, or a restored CI cache) still
    -- overrides the built-in and re-creates the E5113 mismatch. The config must
    -- actively delete those on load.
    assert.is_truthy(
      src:match('nvim_bundled_parsers%s*=%s*%{ "c", "lua", "markdown", "markdown_inline", "query", "vim", "vimdoc" %}'),
      "must enumerate the bundled parser (.so) set to purge"
    )
    assert.is_truthy(
      src:match('purge%("parser/%*%.so"%)'),
      "must scan the runtimepath for stale bundled parser overrides"
    )
    assert.is_truthy(src:match("vim%.fn%.delete"), "must delete stale bundled parser overrides")
    assert.is_truthy(
      src:match('purge%("queries/%*"%)'),
      "must delete stale bundled query overrides as well as parser files"
    )
    -- CRITICAL safety: deletes MUST be scoped to stdpath('data'). Neovim ships
    -- its own bundled parsers as .so files under the install prefix, on the
    -- runtimepath -- an unscoped purge would wipe Neovim's built-in parsers.
    assert.is_truthy(
      src:match("vim%.startswith") and src:match('vim%.fn%.stdpath%("data"%)'),
      "purge must be scoped to stdpath('data') so Neovim's built-in parsers are never deleted"
    )
  end)

  it("still covers the auto-started bundled filetypes so they keep indentexpr (no regression)", function()
    -- Excluding the bundled langs from the install list must NOT drop the
    -- FileType autocmd coverage that gives lua/markdown/help/query
    -- nvim-treesitter's indentexpr (the pre-fix behavior). The autocmd fires for
    -- treesitter_filetypes, which must include these via
    -- nvim_bundled_autostarted_filetypes.
    assert.is_truthy(
      src:match('nvim_bundled_autostarted_filetypes%s*=%s*%{ "lua", "markdown", "help", "query" %}'),
      "auto-started bundled filetypes must still get indentexpr"
    )
    assert.is_truthy(
      src:match("for _, filetype in ipairs%(nvim_bundled_autostarted_filetypes%)"),
      "auto-started bundled filetypes must be appended to treesitter_filetypes"
    )
  end)

  it("installs parsers with the main API", function()
    assert.is_truthy(
      src:match('require%("nvim%-treesitter"%)') and src:match('type%(nvim_treesitter%.install%)%s*==%s*"function"'),
      "main branch must use require('nvim-treesitter').install(...)"
    )
    assert.is_truthy(
      src:match("local function normalized_parser_dependencies%(%)") and src:match("treesitter_config%.norm_languages"),
      "parser auto-install must inspect upstream normalized parser dependencies before installing"
    )
    assert.is_truthy(
      src:match("local function remove_incomplete_parser_installs%(parser_dependencies%)")
        and src:match('treesitter_config%.get_install_dir%("queries"%)'),
      "parser auto-install must repair parser files that were installed without their query directories"
    )
    assert.is_truthy(
      src:match("pcall%(nvim_treesitter%.install, treesitter_parsers, install_opts%)"),
      "parser auto-install must let upstream install the declared parser list"
    )
    assert.is_truthy(
      src:match("DOTFILES_TREESITTER_SYNC_INSTALL"),
      "setup/CI must be able to force synchronous parser bootstrap"
    )
    assert.is_truthy(
      src:match("install_opts%s*=%s*%{ max_jobs%s*=%s*1, summary%s*=%s*true %}"),
      "synchronous parser bootstrap must serialize installs to avoid cache/temp-dir races"
    )
    assert.is_truthy(
      src:match('type%(task_or_err%.wait%)%s*~=%s*"function"')
        and src:match("local install_ok = task_or_err:wait%(900000%)"),
      "synchronous parser bootstrap must require and wait on nvim-treesitter's install task"
    )
    assert.is_truthy(
      src:match("if install_ok ~= true then%s*report_install_problem"),
      "synchronous parser bootstrap must fail when the waitable install task reports false"
    )
    assert.is_truthy(
      src:match("report_install_problem") and src:match("if sync_install then%s*error"),
      "synchronous parser bootstrap must fail nvim instead of warning when install cannot be proven"
    )
  end)

  it("errors in sync mode when the waitable parser install task reports false", function()
    local old_tree_sitter = package.loaded["nvim-treesitter"]
    local old_tree_sitter_config = package.loaded["nvim-treesitter.config"]
    local old_sync = vim.env.DOTFILES_TREESITTER_SYNC_INSTALL
    local old_notify = vim.notify
    local old_executable = vim.fn.executable
    local received_parsers
    local received_options
    package.loaded["nvim-treesitter"] = {
      install = function(parsers, options)
        received_parsers = parsers
        received_options = options
        return {
          wait = function()
            return false
          end,
        }
      end,
      indentexpr = function()
        return 0
      end,
    }
    package.loaded["nvim-treesitter.config"] = {
      get_install_dir = function(name)
        return vim.fs.joinpath(vim.fn.tempname(), name)
      end,
      norm_languages = function(parsers, options)
        assert_same_list(parsers, required)
        assert.are.same({ unsupported = true }, options)
        return parsers
      end,
    }
    vim.env.DOTFILES_TREESITTER_SYNC_INSTALL = "1"
    vim.notify = function() end
    vim.fn.executable = function(name)
      if name == "tree-sitter" then
        return 1
      end
      return old_executable(name)
    end

    local ok, spec = pcall(dofile, repo_root .. "/nvim/lua/plugins/treesitter.lua")
    assert.is_true(ok)
    local config_ok, config_err = pcall(spec[1].config)

    package.loaded["nvim-treesitter"] = old_tree_sitter
    package.loaded["nvim-treesitter.config"] = old_tree_sitter_config
    if old_sync == nil then
      vim.env.DOTFILES_TREESITTER_SYNC_INSTALL = nil
    else
      vim.env.DOTFILES_TREESITTER_SYNC_INSTALL = old_sync
    end
    vim.notify = old_notify
    vim.fn.executable = old_executable

    assert.is_false(config_ok)
    assert.matches("parser install failed", tostring(config_err), nil, true)
    assert_same_list(received_parsers, required)
    assert.are.same({ max_jobs = 1, summary = true }, received_options)
  end)

  it("guards parser compilation on the tree-sitter CLI being on PATH", function()
    -- main shells out to `tree-sitter` per parser; without this guard a missing
    -- CLI dumps one ENOENT error per parser instead of one actionable message.
    assert.is_truthy(
      src:match('vim%.fn%.executable%("tree%-sitter"%)'),
      "must check for the tree-sitter CLI before compiling parsers"
    )
    assert.is_truthy(
      src:match("vim%.notify"),
      "a missing tree-sitter CLI must surface one actionable warning, not N ENOENT errors"
    )
  end)

  it("does not use the legacy configs.setup API", function()
    assert.is_nil(src:match("nvim%-treesitter%.configs"), "legacy nvim-treesitter.configs module must be gone")
    assert.is_nil(src:match("ensure_installed"), "legacy ensure_installed must be gone")
    assert.is_nil(src:match("highlight%s*=%s*%{"), "legacy highlight module must be gone")
    assert.is_nil(src:match("indent%s*=%s*%{"), "legacy indent module must be gone")
  end)

  it("registers highlighting before FileType fires on real file opens", function()
    assert.is_truthy(src:match('event%s*=%s*%{ "BufReadPre", "BufNewFile" %}'), "must load before FileType")
    assert.is_truthy(src:match('nvim_create_autocmd%("FileType"'), "must create a FileType autocmd")
    assert.is_truthy(src:match('pattern%s*=%s*"%*"'), "FileType autocmd must inspect every detected filetype")
    assert.is_truthy(
      src:match("if treesitter_filetype_set%[filetype%] then"),
      "only parser-backed filetypes should start Tree-sitter"
    )
    assert.is_truthy(src:match("pcall%(vim%.treesitter%.start, args%.buf%)"), "missing parsers must not error")
  end)

  it("keeps regex syntax fallback by runtime capability, not a language allowlist", function()
    assert.is_nil(
      src:match("regex_syntax_fallback_filetypes"),
      "do not reintroduce a per-language syntax fallback table"
    )
    assert.is_truthy(src:match("runtime_syntax_for_filetype"), "must derive fallback syntax from runtime capability")
    assert.is_truthy(
      src:match('nvim_get_runtime_file%("syntax/" %.%. filetype %.%. "%.vim", false%)'),
      "fallback must check whether Neovim actually has syntax/<filetype>.vim"
    )
    assert.is_truthy(
      src:match("local syntax = runtime_syntax_for_filetype%(args%.buf, filetype%)"),
      "FileType callback must capture syntax before vim.treesitter.start() can clear it"
    )
    assert.is_truthy(
      src:match("vim%.bo%[args%.buf%]%.syntax%s*=%s*syntax"),
      "FileType callback must restore syntax fallback after vim.treesitter.start() clears it"
    )
    local installed = {}
    for _, parser in ipairs(extract_string_list("treesitter_parsers")) do
      installed[parser] = true
    end
    assert.is_nil(installed.dosbatch, "dosbatch has no pinned nvim-treesitter parser; keep it syntax-only")
  end)

  it("registers filetype aliases for parsers whose names differ from Neovim filetypes", function()
    assert.is_truthy(src:match('bash%s*=%s*%{ "sh" %}'), "sh files should use the bash parser")
    assert.is_truthy(src:match('bibtex%s*=%s*%{ "bib" %}'), "bib files should use the bibtex parser")
    assert.is_truthy(src:match('c_sharp%s*=%s*%{ "cs" %}'), "cs files should use the c_sharp parser")
    assert.is_truthy(
      src:match('git_config%s*=%s*%{ "gitconfig" %}'),
      "gitconfig files should use the git_config parser"
    )
    assert.is_truthy(
      src:match('git_rebase%s*=%s*%{ "gitrebase" %}'),
      "gitrebase files should use the git_rebase parser"
    )
    assert.is_truthy(src:match('ini%s*=%s*%{ "dosini" %}'), "dosini files should use the ini parser")
    assert.is_truthy(
      src:match('javascript%s*=%s*%{ "javascriptreact" %}'),
      "jsx files should use the javascript parser"
    )
    assert.is_truthy(src:match('latex%s*=%s*%{ "plaintex", "tex" %}'), "tex/plaintex files should use the latex parser")
    assert.is_truthy(src:match('powershell%s*=%s*%{ "ps1" %}'), "ps1 files should use the powershell parser")
    assert.is_truthy(src:match('qmljs%s*=%s*%{ "qml" %}'), "qml files should use the qmljs parser")
    assert.is_truthy(
      src:match('ssh_config%s*=%s*%{ "sshconfig" %}'),
      "sshconfig files should use the ssh_config parser"
    )
    assert.is_truthy(src:match('tsx%s*=%s*%{ "typescriptreact" %}'), "tsx files should use the tsx parser")
    -- No vimdoc={help} alias: vimdoc is Neovim-bundled and Neovim already maps
    -- help -> vimdoc and auto-starts it, so the repo must NOT register it.
    assert.is_nil(src:match('vimdoc%s*=%s*%{ "help" %}'), "vimdoc is nvim-bundled; do not alias help to it")
    assert.is_truthy(
      src:match("vim%.treesitter%.language%.register%(parser, filetypes%)"),
      "aliases must be registered"
    )
  end)

  it("uses the main-branch indentation replacement", function()
    assert.is_truthy(
      src:match('type%(nvim_treesitter%.indentexpr%)%s*==%s*"function"'),
      "main indentexpr must be gated so stale plugin caches do not break buffer highlighting"
    )
    assert.is_truthy(
      src:match([[vim%.bo%[args%.buf%]%.indentexpr%s*=%s*"v:lua%.require'nvim%-treesitter'%.indentexpr%(%)"]]),
      "legacy indent.enable should be replaced with nvim-treesitter main indentexpr"
    )
  end)

  it("keeps the documented main-branch TSUpdate build and command triggers", function()
    assert.is_truthy(src:match('build%s*=%s*":TSUpdate"'), "upstream main docs still recommend TSUpdate as build")
    for _, command in ipairs({ "TSInstall", "TSInstallFromGrammar", "TSUpdate", "TSUninstall", "TSLog" }) do
      assert.is_truthy(src:match('"' .. command .. '"'), command .. " command trigger missing")
    end
  end)

  it("does not force a specific compiler", function()
    assert.is_nil(src:match("install%.compilers"), "don't pin compilers; auto-detection is cross-platform")
  end)

  it("wraps nvim_del_user_command in pcall", function()
    assert.is_truthy(
      src:match("pcall%(vim%.api%.nvim_del_user_command"),
      "unprotected nvim_del_user_command throws on fresh installs"
    )
  end)
end)
