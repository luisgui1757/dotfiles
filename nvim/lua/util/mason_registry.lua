local M = {}
local repository = "mason-org/mason-registry"
local config_root = vim.fn.fnamemodify(vim.fn.resolve(debug.getinfo(1, "S").source:sub(2)), ":p:h:h:h")

local function read(path)
  local file = assert(io.open(path, "rb"), "missing Mason registry evidence: " .. path .. "; run installer Repair")
  local data = file:read(2 * 1024 * 1024 + 1)
  assert(file:close())
  assert(data and #data <= 2 * 1024 * 1024, "Mason registry evidence exceeds size bound")
  return data
end

local function write(path, data)
  local file = assert(io.open(path, "wb"))
  local ok, err = file:write(data)
  local closed, close_err = file:close()
  assert(ok and closed, err or close_err)
end

function M.snapshot(config)
  config = config or vim.env.DOTFILES_NVIM_CONFIG or config_root
  local directory = vim.fs.joinpath(config, "mason-registry")
  local lock = vim.json.decode(read(vim.fs.joinpath(directory, "lock.json")))
  assert(
    lock.repository == repository
      and type(lock.version) == "string"
      and lock.version:match("^%d%d%d%d%-%d%d%-%d%d%-[a-z0-9%-]+$")
      and type(lock.registry_sha256) == "string"
      and lock.registry_sha256:match("^%x+$")
      and #lock.registry_sha256 == 64,
    "invalid Mason registry lock"
  )
  local data = read(vim.fs.joinpath(directory, "registry.json"))
  assert(vim.fn.sha256(data) == lock.registry_sha256, "Mason registry differs from reviewed SHA-256")
  return lock, data
end

local function registry_directory(root)
  return vim.fs.joinpath(root, "registries", "github", "mason-org", "mason-registry")
end

function M.verify(root, config)
  local lock = M.snapshot(config)
  local directory = registry_directory(root)
  assert(
    vim.fn.sha256(read(vim.fs.joinpath(directory, "registry.json"))) == lock.registry_sha256,
    "installed Mason registry differs from reviewed SHA-256; run installer Repair"
  )
  local info = vim.json.decode(read(vim.fs.joinpath(directory, "info.json")))
  assert(
    info.version == lock.version
      and type(info.checksums) == "table"
      and info.checksums["registry.json"] == lock.registry_sha256,
    "installed Mason registry metadata differs from lock; run installer Repair"
  )
  return lock
end

function M.prepare()
  local runtime = require("util.managed_runtime")
  local root = runtime.path("mason")
  local lock, data = M.snapshot()
  if runtime.read_only() then
    M.verify(root)
  else
    -- Reuse Mason's supported GitHub registry format. Its fixed-version source
    -- refresh is a no-op; installation never needs a live registry lookup.
    local directory = registry_directory(root)
    vim.fn.mkdir(directory, "p")
    write(vim.fs.joinpath(directory, "registry.json"), data)
    write(
      vim.fs.joinpath(directory, "info.json"),
      vim.json.encode({
        version = lock.version,
        checksums = { ["registry.json"] = lock.registry_sha256, ["registry.json.zip"] = lock.archive_sha256 },
        download_timestamp = os.time(),
      })
    )
  end
  return "github:" .. repository .. "@" .. lock.version
end

function M.sources(names, config)
  local _, data = M.snapshot(config)
  local packages = {}
  for _, package in ipairs(vim.json.decode(data)) do
    assert(not packages[package.name], "duplicate Mason registry package")
    packages[package.name] = package
  end
  local purl = require("mason-core.purl")
  local sources = {}
  for _, name in ipairs(names) do
    local package = assert(packages[name], "selected Mason package missing from snapshot: " .. name)
    local parsed = purl.parse(package.source.id):get_or_throw()
    -- These reviewed tools use numeric releases (optionally prefixed with v),
    -- never branches, ranges, latest, nightly or other moving aliases.
    assert(
      type(parsed.version) == "string"
        and (parsed.version:match("^v?%d+%.%d+%.%d+$") or parsed.version:match("^%d%d%d%d%-%d%d%-%d%d$")),
      "selected Mason package lacks a concrete version: " .. name
    )
    sources[name] = purl.compile(parsed)
  end
  return sources
end

function M.check_receipts(root, names, config)
  local lock = M.verify(root, config)
  for name, source in pairs(M.sources(names, config)) do
    local receipt = vim.json.decode(read(vim.fs.joinpath(root, "packages", name, "mason-receipt.json")))
    assert(receipt.name == name, "Mason receipt name differs: " .. name)
    local installed = require("mason-core.receipt").InstallReceipt.from_json(receipt)
    assert(installed:get_installed_purl() == source, "Mason installed source/version differs from lock: " .. name)
    assert(
      type(receipt.registry) == "table"
        and receipt.registry.proto == "github"
        and receipt.registry.namespace == "mason-org"
        and receipt.registry.name == "mason-registry"
        and receipt.registry.version == lock.version
        and type(receipt.registry.checksums) == "table"
        and receipt.registry.checksums["registry.json"] == lock.registry_sha256,
      "Mason receipt registry differs from lock: " .. name
    )
  end
end

return M
