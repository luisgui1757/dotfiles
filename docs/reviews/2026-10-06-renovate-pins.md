# Renovate installer pin reconciliation

PR #81 updates native installer artifacts and the Renovate validation runtime.
On 2026-10-06 the proposed PR failed the cross-file pin guard in 11 places;
version-only bumps also retained old checksums and gh-dash tag identities.
All affected mirrors, executable identities, tests and operator docs are now
updated together. The unchanged cargo-binstall script was downloaded at the
new commit; its existing digest remains correct.

Pi 0.99.2 adds three direct monorepo dependencies plus transitive `pi-telemetry`. Both installers now request all seven
companions at the exact CLI release: `pi-agent-core`, `pi-ai`, `pi-tui`,
`pi-mcp`, `pi-codemode`, `chord`, and `pi-telemetry`. The shell argument regression failed before
the companion fix and passed afterward; the matching Windows assertion checks
all seven arguments and rejects extras. The real npm install in a temporary prefix
passed `pi --version`, `pi --help`, all three managed theme loads, and the
managed newline keybinding resolution without provider credentials or API calls.

Tree-sitter 0.27.0 still declares language ABI 15, matching Neovim's supported
ABI. Hosted clean parser/LSP/formatter installation remains a required gate.
Herdr 0.9.3 was already accepted by the bounded runtime/config check from PR #85.
The Windows Herdr preview pin is unchanged.

The official gh-dash `v4.26.0` annotated tag resolves to tag object
`17b8f7d6a21d79172f0f2309b607643a215c780f`, peeled commit
`c6dfbc17edfdbf1060fc06efe6aabb34f6a725f0`. Homebrew's installer now supports
Apple Silicon only on macOS, matching this repository's existing platform
contract. Scoop's new script is still invoked directly, so its new dot-source
guard does not suppress installation.

## Download evidence

Each artifact below was downloaded from its official versioned release and
its locally computed SHA-256 matched GitHub's release-asset digest.

| Artifact | SHA-256 |
|---|---|
| [nvim-linux-x86_64.tar.gz](https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-linux-x86_64.tar.gz) | `bce0f56eda1f1b1db6eee8f4133d7a38813ea07933837dd1777411ca384c6875` |
| [nvim-linux-arm64.tar.gz](https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-linux-arm64.tar.gz) | `1aa5ca085249580ae0f91eb14f27ec0919773ff2d99a163d03f3d6c21ac29725` |
| [chezmoi_2.73.0_linux_amd64.tar.gz](https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi_2.73.0_linux_amd64.tar.gz) | `b597729b687af4488a848240134cb633de8ca0f04e0d26d48f400ee2ac338ffa` |
| [chezmoi_2.73.0_linux_arm64.tar.gz](https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi_2.73.0_linux_arm64.tar.gz) | `abcb840401d3c1f2356e0f53f5d52aa10d10f572654d9626db9ad0ca4dc03355` |
| [chezmoi_2.73.0_darwin_arm64.tar.gz](https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi_2.73.0_darwin_arm64.tar.gz) | `246679a0b200e7e8be4a951be3b95d37c33ecb87eaab5af6f4949f7d0317bcc1` |
| [chezmoi_2.73.0_windows_amd64.zip](https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi_2.73.0_windows_amd64.zip) | `266938399108028a5b6dc7a137026e3ead747304122e5fe381d5396e496620f6` |
| [lazygit_0.66.0_linux_x86_64.tar.gz](https://github.com/jesseduffield/lazygit/releases/download/v0.66.0/lazygit_0.66.0_linux_x86_64.tar.gz) | `5b45541155d20bd32bf2cc5ab5b7e3d91c2eebf0fb1242281350edc27d59d2b7` |
| [lazygit_0.66.0_linux_arm64.tar.gz](https://github.com/jesseduffield/lazygit/releases/download/v0.66.0/lazygit_0.66.0_linux_arm64.tar.gz) | `9a4fc4656897ac9f7877b835473ce1a75620cc267f554c57fc4ff266407f3257` |
| [tree-sitter-cli-linux-x64.zip](https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-linux-x64.zip) | `e4a3826bcd0fe099ee3a5617767374939cbc23c4a35b5b53f5fc04142525a2c1` |
| [tree-sitter-cli-linux-arm64.zip](https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-linux-arm64.zip) | `6260b621bf5ab87027dfb463bf955504ef32cdcda62b81f28447753e48c83a62` |
| [tree-sitter-cli-windows-x64.zip](https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-windows-x64.zip) | `46188d31c1f3847307b03e92f3a3f60606eb04147609e26cdd50a07f7d0b35da` |
| [tree-sitter-cli-windows-arm64.zip](https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-windows-arm64.zip) | `e44462444fa7fc873b07e6d0735c6772980c7be6024184b40529a17904da58bf` |
| [tree-sitter-cli-windows-x86.zip](https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-windows-x86.zip) | `88510ef8cd1d4fdf3b97dc19298d3e31736226a9f53ab9ec3749d056ce3536c4` |
| [Hack.zip](https://github.com/ryanoasis/nerd-fonts/releases/download/v3.5.1/Hack.zip) | `fa24da7de7cefe7766614d27762570b20453c852fc1d5b657111666df9a5e449` |
| [Microsoft.WindowsTerminal_1.25.2733.0_x64.zip](https://github.com/microsoft/terminal/releases/download/v1.25.2733.0/Microsoft.WindowsTerminal_1.25.2733.0_x64.zip) | `bf3ef2012f6c44d8340a4c58125acc9498d19b580f9890dc043cdf831852e796` |
| [herdr-linux-x86_64](https://github.com/herdrdev/herdr/releases/download/v0.9.3/herdr-linux-x86_64) | `18a8dc65f1c2fa485884344356dea1cfd911c6f06cf46fa78e193f4087f4dba7` |
| [herdr-linux-aarch64](https://github.com/herdrdev/herdr/releases/download/v0.9.3/herdr-linux-aarch64) | `4de7aa3e25678812e92960de64f7c2aaa1bca1f0f80a3c5e559837e231e1f5c0` |

The Homebrew installer at `35da6871c4be7d7fdab2fd505fb7fa667926a2a5` hashes to
`5f333bbe53bc490e51e7ccb1df8779b3dd6ee73a1a7379efda216edb08ccb148`.
The Scoop installer at `1e2f334083d609986d8c8bc9e31ae8e87c39fab4` hashes to
`94f983b190438311e006b957db7c8422709e0ba62a6c2ac04e278164108f2512`.
The new cargo-binstall ref is `7bebc2e59eb8820162b7ac8f62a92bfdb2732447`;
its script remains `d3a93702160e0ec03e2a4e996855db1f01adee801fb84a43add24e0877ef8eae`.
Pi's npm registry integrity matched the independently hashed package bytes;
the SRI is recorded in the installers and supply-chain ledger.

## Verification boundary

`make ci` passed on the `ec92306` baseline. Final local verification, independent
review and all required hosted checks must pass before merging. The local
macOS gate skips PowerShell when unavailable; native Windows execution is
proved by the required hosted Windows jobs. Historical release notes and prior
review records retain their original identities.

Independent review rehashed all downloaded artifacts and found one Low issue:
the active Windows manual checklist still expected Tree-sitter 0.26.11. It now
expects 0.27.0. The full local `make ci` gate and final static suite passed;
required hosted checks remain the merge gate.
