# Supply-chain identities

Executable content is installed or executed only after it is bound to a
reviewed immutable identity. Package-manager catalog names are not treated as
proof for the direct-artifact paths below.

| Surface | Reviewed identity | Enforcement |
|---|---|---|
| Pi CLI | `@earendil-works/pi-coding-agent@0.80.3`, SRI `sha512-TIggw9gCXpA+Ph7OjdTA7ka2NPwTVuPmy39KDSyUzaKq8VvHfMGR7vtRz4JB7Um/RMRblmzhu4p9tUCk6MTgGA==` | Both `npm pack` metadata and the downloaded tarball bytes must match; npm receives only the local verified tarball. |
| Windows Tree-sitter CLI | `v0.26.10`; x64 `e378c57f5de3e698058997489e69a027551dc05a09c6ff51e42ffab6ea5d5b6b`; arm64 `d30a6a6986a0fdbdb3a6c0f0e23dc6e6719e133f73dee7c65cf73839a458bced`; x86 `404a1cd17eacb76368db782859bc60521e66e16693572f52383690c554d2c3bd` | Architecture-specific release zip hash plus executable version validation before and after atomic publication. |
| gh-dash | tag `v4.25.1`; tag object `e6ebbd7e83e30161b9192ce3339972d2c8269e7f`; peeled commit `49f37e4832956c57bf52d4ea8b1b1e5c0f863700` | Remote tag mapping is checked, then `gh extension install --pin` receives the commit. |
| Ubuntu PowerShell repository config | Ubuntu 24.04 `packages-microsoft-prod.deb`, SHA-256 `c13f01ac7c3001b51a9281d40dde666db5e037e05512840c319832f7852bfec4` | Required CI verifies the downloaded file immediately before `sudo dpkg -i`. |
| Windows Terminal Sandbox helper | Production `v1.24.11321.0` x64 portable zip, SHA-256 `7caef554147e5498ed1becdca73cdedb79fbc81f89032e46ae9b095c53433812` | The helper imports the production pin, verifies it, transactionally publishes the portable tree, and delegates settings to setup. It never queries `releases/latest` or mirrors packaged settings. |
| Zsh plugins | fzf-tab `d7e0234614dbe5369fdd760907d12c0e05a4dccc`; zsh-autosuggestions `e52ee8ca55bcc56a17c828767a3f98f22a68d4eb` | One publisher quarantines unproved sourceable paths, fetches the exact commit into a sibling stage, verifies origin/HEAD/cleanliness/tracked entry file, and atomically publishes. |

`tests/static/supply_chain_remote_execution_test.sh` rejects remote-eval
patterns, unchecked downloaded PowerShell executables, and downloads that flow
to privileged package installation without an intervening SHA-256 check.
`tests/static/repo_policy_test.sh` requires every external GitHub Actions
`uses:` reference to be a full lowercase 40-hex commit SHA.

The checked-in safeguard script also requests repository-level Actions SHA
pinning. At the start of the 2026-07-10 closure branch the live API reported
`sha_pinning_required: false`; this branch does not mutate live settings. After
merge, the owner must run `scripts/apply-repo-safeguards.sh
luisgui1757/dotfiles` and verify the live value becomes `true`.
