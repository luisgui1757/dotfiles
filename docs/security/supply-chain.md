# Supply-chain identities

The current installer has two explicit package boundaries: reviewed private
archives and native package managers. The catalog is
[`installer/resources.json`](../../installer/resources.json); reviewed archive
identities are [`installer/archive-pins.json`](../../installer/archive-pins.json).
Neither a package name nor a version print proves downloaded bytes.

| Surface | Identity and enforcement |
|---|---|
| Checkout bootstrap | Pinned Go archive and SHA-256, verified before extraction; build identity includes the runtime and embedded source inputs. Personal Go settings, workspaces and external cache programs cannot alter the build. |
| Private tools and desktop apps | Exact upstream URL/version/SHA-256, bounded safe archive extraction, declared required files, private generation publication and recorded content identity. Update reconciles reviewed pins in the checkout. |
| Rust and Python tools | Rust's six upstream components are independently verified and combined without upstream install scripts. LaTeX conversion and Yamllint use private venvs with pinned offline sources/wheels; complete Python identity is part of the desired state. |
| Pi | Pinned standalone upstream archive with runtime assets and native helpers. Node/npm and Git support Pi package features without changing a global npm prefix. User-added packages follow Pi's own package workflow. |
| Neovim | Locked plugin sources, reviewed parser/tool recipes and a private generated runtime. Check loads the existing runtime without triggering package repair. |
| Shell/tmux plugins and Sentinel | Pinned source archives or exact generated policy, source corrections restricted to reviewed fresh payloads, and retained upstream license/provenance. No runtime remote-script pipeline. |
| Ubuntu/Debian native packages | Actual APT/dpkg transactions and metadata; ownership is attributed to the operation, never an inventory difference. Native repositories resolve system packages under their own trust model. |
| macOS native packages | Actual Homebrew formula transactions and native dependency/linkage checks. Existing infrastructure remains unowned; verified fresh bootstrap is retained infrastructure. Apple CLT uses Apple's advertised signed packages. |
| Windows vendor tools | Pinned bootstrapper bytes plus valid Microsoft Authenticode and native registration/operation evidence. Build Tools resolves Microsoft's signed servicing packages; its bootstrap hash is not a claim that those changing components are individually pinned. |
| Windows Terminal | Pinned official unpackaged stable ZIP; scoped settings at the observed LocalAppData path. Store/Preview/Canary are independent. |
| Ghostty on Linux | One reviewed Debian-family archive per architecture, extracted privately; native APT supplies declared system libraries. No upstream repository-add script or DEB maintainer script executes. |
| Hosted actions and containers | Full action commit SHAs and immutable container digests; native jobs verify their actual OS/architecture. |

Existing packages, foreign files and personal data never acquire deletion
authority merely by matching a package name. Removal verifies saved ownership and
current native consumers; it does not run global autoremove. Failed or unknown
native operations require reconciliation before another mutation.

Archive/bootstrap pin updates remain reviewed maintenance in this major release.
Update hashes together with extraction/layout and native-consumption evidence.
Renovate covers only sources in its generated extraction inventory; do not claim
that it maintains archive hashes that it does not extract.

Release publication follows [RELEASING.md](../RELEASING.md) and
`release/manifest.json`. Future release certification uses exact-head native
installer jobs and captured evidence; historical published proofs remain
immutable. Checked-in branch-protection policy is separate from live GitHub
configuration; its cutover is documented in
[branch protection](branch-protection.md).

The [previous supply-chain guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/security/supply-chain.md)
preserves Nix/chezmoi-era identities and enforcement for historical releases.
Those installers and their fallback package-manager chains are retired.
