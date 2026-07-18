-- which-key.nvim must stay lazy-loaded so it never adds to the startup budget,
-- must NOT force eager load (invariant 7: only rose-pine may be lazy = false),
-- and must expose Folke's built-in :WhichKey command while keeping the
-- <leader>? buffer-local keymap popup. We inspect the spec by
-- dofile (same approach as debugging_spec) so the assertion holds without the
-- plugin being cloned.
describe("which-key spec", function()
  local repo_root = _G.TEST_REPO_ROOT

  local function which_key_spec()
    local specs = dofile(repo_root .. "/nvim/lua/plugins/which-key.lua")
    for _, spec in ipairs(specs) do
      if spec[1] == "folke/which-key.nvim" then
        return spec
      end
    end
    return nil
  end

  it("is present and lazy-loads on VeryLazy", function()
    local spec = which_key_spec()
    assert.is_not_nil(spec, "folke/which-key.nvim spec missing")
    assert.are.equal("VeryLazy", spec.event)
  end)

  it("does not force eager load (only rose-pine may be lazy = false)", function()
    local spec = which_key_spec()
    assert.is_not_nil(spec)
    assert.is_true(spec.lazy ~= false)
  end)

  it("lazy-loads for Folke's built-in :WhichKey command", function()
    local spec = which_key_spec()
    assert.is_not_nil(spec)
    assert.are.equal("WhichKey", spec.cmd)
  end)

  it("registers the <leader>? buffer-local keymap popup", function()
    local spec = which_key_spec()
    assert.is_not_nil(spec)
    assert.is_table(spec.keys)
    local found
    for _, k in ipairs(spec.keys) do
      if k[1] == "<leader>?" then
        found = k
        break
      end
    end
    assert.is_not_nil(found, "<leader>? keymap missing from which-key spec")
    assert.are.equal("function", type(found[2]))
    assert.is_truthy(found.desc)
  end)
end)
