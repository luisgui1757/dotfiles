-- Single source of truth for the per-language smoke test
-- (tests/nvim/spec/language_smoke_spec.lua). Each row pins, for ONE fixture:
--   fixture     path under tests/nvim/fixtures/
--   filetype    the filetype Neovim must detect when the fixture opens
--   parser      the treesitter parser for that filetype (false = none)
--   bundled     true when Neovim bundles a matched parser+query for it (c, lua,
--               markdown, query, vim) -- those must NOT be in treesitter_parsers
--               (installing nvim-treesitter's would override the built-in and
--               break the bundled query), so the spec asserts they are absent
--   formatters  conform formatters_by_ft for that filetype ({} = none)
--   lsp         the LSP server expected for that filetype (false = none)
--   lsp_gated   true when the server only enables if its runtime is present
--               (e.g. powershell_es needs pwsh), so Tier 2 skips it otherwise
--
-- Keep this in sync with nvim/lua/plugins/{treesitter,conform,lsp-config}.lua.
-- The spec asserts they agree, so drift fails CI -- this is the single place
-- that would have caught the jsonc "skipping unsupported language" bug.
return {
  {
    fixture = "sample.lua",
    filetype = "lua",
    parser = "lua",
    bundled = true,
    formatters = { "stylua" },
    lsp = "lua_ls",
  },
  {
    fixture = "sample.py",
    filetype = "python",
    parser = "python",
    formatters = { "ruff_fix", "ruff_format" },
    lsp = "pyright",
  },
  {
    fixture = "sample.c",
    filetype = "c",
    parser = "c",
    bundled = true,
    formatters = { "clang_format" },
    lsp = "clangd",
  },
  { fixture = "sample.cpp", filetype = "cpp", parser = "cpp", formatters = { "clang_format" }, lsp = "clangd" },
  { fixture = "sample.rs", filetype = "rust", parser = "rust", formatters = { "rustfmt" }, lsp = "rust_analyzer" },
  { fixture = "CMakeLists.txt", filetype = "cmake", parser = "cmake", formatters = { "gersemi" }, lsp = "neocmake" },
  { fixture = "sample.sh", filetype = "sh", parser = "bash", formatters = { "shfmt" }, lsp = "bashls" },
  { fixture = "sample.zsh", filetype = "zsh", parser = false, formatters = { "shfmt" }, lsp = "bashls" },
  {
    fixture = "sample.ps1",
    filetype = "ps1",
    parser = "powershell",
    formatters = {},
    lsp = "powershell_es",
    lsp_gated = true,
  },
  { fixture = "sample.json", filetype = "json", parser = "json", formatters = { "prettier" }, lsp = "jsonls" },
  { fixture = "sample.jsonc", filetype = "jsonc", parser = false, formatters = { "prettier" }, lsp = "jsonls" },
  { fixture = "sample.yaml", filetype = "yaml", parser = "yaml", formatters = { "prettier" }, lsp = "yamlls" },
  { fixture = "sample.toml", filetype = "toml", parser = "toml", formatters = {}, lsp = false },
  {
    fixture = "sample.md",
    filetype = "markdown",
    parser = "markdown",
    bundled = true,
    formatters = { "prettier" },
    lsp = false,
  },
  { fixture = "sample.vim", filetype = "vim", parser = "vim", bundled = true, formatters = {}, lsp = false },
  {
    fixture = "queries/lua/highlights.scm",
    filetype = "query",
    parser = "query",
    bundled = true,
    formatters = {},
    lsp = false,
  },
  { fixture = "sample.diff", filetype = "diff", parser = "diff", formatters = {}, lsp = false },
  { fixture = "COMMIT_EDITMSG", filetype = "gitcommit", parser = "gitcommit", formatters = {}, lsp = false },
}
