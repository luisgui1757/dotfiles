# Renovate Nix installer action update

PR #83 updates every `DeterminateSystems/nix-installer-action` invocation from
v22 to v23, pinned to `3138316df39ed29be04236d7ffc686fa525866aa`.
The official tag was resolved with `git ls-remote` on 2026-10-06.

The [upstream comparison](https://github.com/DeterminateSystems/nix-installer-action/compare/v22...v23)
preserves the existing inputs and adds optional installer-checksum inputs.
The repository uses the default Determinate distribution in CI. Same-repository
public-setup proofs continue to exercise the repository's separately verified
upstream Nix bootstrap; the action remains the fork-PR bootstrap fallback there.
No public installer or Nix package ownership changes.

The full local `make ci` baseline passed on `ec92306`, and `make ci` passed
again with this update. Independent review found no material issues. Local
PowerShell checks skipped because `pwsh` is unavailable; required hosted Windows
checks remain the native execution authority. Every required hosted check and
CodeQL must pass on the final PR head before merge.
