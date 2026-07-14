# Supply-chain identities

Executable content is installed or executed only after it is bound to a
reviewed immutable identity. Package-manager catalog names are not treated as
proof for the direct-artifact paths below.

| Surface | Reviewed identity | Enforcement |
|---|---|---|
| v0.2.0 Nix prerequisite | Upstream Nix `2.34.0`; aarch64-darwin `47cb78c9fdc7b630dbbb9a89869c8e8bcd8c9eb17be036fba18585120693a4c1`; x86_64-linux `5676b0887f1274e62edd175b6611af49aa8170c69c16877aa9bc6cebceb19855`; aarch64-linux `cfddd4008b57a71464a16d5232cba79b1c76ae9dc81bbf71b4972b0118bc29c5` | One isolated official-remote advertisement selects the trust state: before v0.2.0 exists, only a clean exact current official branch head is accepted; once the unique annotated tag appears, branch authority closes and the local tag object, peeled commit, and HEAD must match it. The installer then downloads one versioned `releases.nixos.org` tarball with HTTPS/TLS constraints, compares the review-pinned SHA-256, rejects unsafe archive paths, and only then extracts and executes the fixed installer path. Other platforms are rejected before download. |
| v0.1.0 to v0.2.0 release sources | v0.1.0 tag object `a3b4d6d7b6d289959cac68d76faec96219b3e310`, peeled commit `015617362830280bf85c7142e69d0681d376d453`; exact official annotated v0.2.0 tag resolved at runtime | Preflight binds local tags to the official remote and requires clean side-by-side checkouts. Before mutation, both migrators archive the exact commits into private recovery, fingerprint the extracted trees, and bind apply/readback/rollback to those frozen sources. Retained-checkout drift cannot change a transaction write. |
| Pi CLI | `@earendil-works/pi-coding-agent@0.80.3`, SRI `sha512-TIggw9gCXpA+Ph7OjdTA7ka2NPwTVuPmy39KDSyUzaKq8VvHfMGR7vtRz4JB7Um/RMRblmzhu4p9tUCk6MTgGA==` | Both `npm pack` metadata and the downloaded tarball bytes must match; npm receives only the local verified tarball. |
| Windows Tree-sitter CLI | `v0.26.10`; x64 `e378c57f5de3e698058997489e69a027551dc05a09c6ff51e42ffab6ea5d5b6b`; arm64 `d30a6a6986a0fdbdb3a6c0f0e23dc6e6719e133f73dee7c65cf73839a458bced`; x86 `404a1cd17eacb76368db782859bc60521e66e16693572f52383690c554d2c3bd` | Architecture-specific release zip hash plus executable version validation before and after atomic publication. |
| gh-dash | tag `v4.25.1`; tag object `e6ebbd7e83e30161b9192ce3339972d2c8269e7f`; peeled commit `49f37e4832956c57bf52d4ea8b1b1e5c0f863700` | Remote tag mapping is checked, then `gh extension install --pin` receives the release tag required for a binary extension. |
| Ubuntu PowerShell repository config | Ubuntu 24.04 `packages-microsoft-prod.deb`, SHA-256 `c13f01ac7c3001b51a9281d40dde666db5e037e05512840c319832f7852bfec4` | Required CI verifies the downloaded file immediately before `sudo dpkg -i`. |
| Ghostty Debian-family packages | `mkasberg/ghostty-ubuntu` `1.3.1-0-ppa2`; Ubuntu 24.04 amd64 `478d440153ef544426418efc7d6d8901715359f452c46be29071901a94b8cd47`, arm64 `91063815b6ce3d834d59714b4ad0310f744448b6716836d035b3d331d1923363`; Ubuntu 25.10 amd64 `793bde1c31163d8e1d12ea939c8b941f7908170e57bbf19b121434a0f6621c59`, arm64 `c6a4fd4fd786b4bdea42036650ef1724f535c4b636329f488f7ece36820d3d6b`; Debian trixie amd64 `9fda8e418d7a7f58149ba3ba823a255d6b80f8bb5431b3bd7e912ff597715b2e`, arm64 `73f384e62c419d7a7809d686bf579fea5e23f52742b34f70c74d6adf0e72f8ab` | Setup maps reviewed distro/architecture pairs to one release URL, verifies SHA-256 and exact dpkg package/architecture/version metadata before privileged apt, then validates installed version plus command. The upstream script and its mutable `releases/latest` lookup are never executed. |
| Windows Terminal Sandbox helper | Production `v1.24.11321.0` x64 portable zip, SHA-256 `7caef554147e5498ed1becdca73cdedb79fbc81f89032e46ae9b095c53433812` | The helper imports the production pin, verifies it, transactionally publishes the portable tree, and delegates settings to setup. It never queries `releases/latest` or mirrors packaged settings. |
| Local Linux owner lifecycle container | `ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90`; `nixos/nix:2.34.0@sha256:b9c9611c8530fa8049a1215b20638536e1e71dcaf85212e47845112caf3adeea` | The checked Dockerfile uses digest-bound base images, copies the reviewed Nix store into the Ubuntu fixture, and runs the real lifecycle as a non-root user. No mutable image tag selects executed bytes. |
| Zsh plugins | fzf-tab `d7e0234614dbe5369fdd760907d12c0e05a4dccc`; zsh-autosuggestions `e52ee8ca55bcc56a17c828767a3f98f22a68d4eb` | One publisher quarantines unproved sourceable paths, fetches the exact commit into a sibling stage, verifies origin/HEAD/cleanliness/tracked entry file, and atomically publishes. |

`tests/static/supply_chain_remote_execution_test.sh` rejects remote-eval
patterns, unchecked downloaded PowerShell executables, and downloads that flow
to privileged package installation without an intervening SHA-256 check. Its
privileged-flow model includes the repository's `maybe_sudo` and
`verify_sha256` helpers, with positive and negative self-tests.
`tests/static/repo_policy_test.sh` requires every external GitHub Actions
`uses:` reference to be a full lowercase 40-hex commit SHA.

The checked-in safeguard script also requests repository-level Actions SHA
pinning. At the start of the 2026-07-10 closure branch the live API reported
`sha_pinning_required: false`; this branch does not mutate live settings. After
merge and exact cache-free proof, the owner must run the script's no-write
preflight and apply. It accepts only the expected legacy-to-stable transition,
requires public repository visibility, captures the old
Actions/integrity/classic state before writing, and restores all three on
failure. Restore freezes and exact-policy-validates every consumed snapshot file
against the manifest's captured Git commit—which must still be live `main`—
before publishing only those bytes; incomplete, altered, or cross-stage recovery
material cannot reach a live write.
Apply likewise derives every desired payload
from exact committed objects after the second live capture, freezes the complete
set in a private read-only directory, and never publishes mutable checkout
bytes. The apply verifies the live value becomes `true`.
