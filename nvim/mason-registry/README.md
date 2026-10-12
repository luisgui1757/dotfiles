# Reviewed Mason registry

`registry.json` is the byte-for-byte compiled upstream registry from
[mason-org/mason-registry](https://github.com/mason-org/mason-registry), at the
release and SHA-256 values in `lock.json`. It is distributed under the upstream
[Apache 2.0 license](../../LICENSES/mason-registry-Apache-2.0.txt) (upstream license
Git blob `dd5b3a58aa1849f452abc9b5cd1638dc71a5e482`). No package recipes are modified.

To update, review a new release and its archive/registry hashes in `lock.json`,
then run `python3 scripts/update-mason-registry.py` from the repository root.
The script verifies both hashes before replacing the snapshot. Commit the lock
and reproduced snapshot together, run the Neovim tests, and require the native
Neovim lifecycle on all supported targets. Do not edit generated registry JSON.

Mason consumes this snapshot through its existing fixed-version GitHub registry
format. The selected tool list remains in `lua/util/mason_tools.lua`; each tool's
concrete source/version comes from this snapshot, without a second version list.
Provisioning and Check verify installed source IDs and registry provenance.
Published managed startup only verifies the registry; Update/Repair constructs a
new generation. Registry pins fix recipe and top-level tool versions; upstream
package-manager transitive resolution is not a claim of byte-identical builds.
