# Supply-chain identities

The current installer has two explicit package boundaries: reviewed private
archives and native package managers. The catalog is
[`installer/resources.json`](../../installer/resources.json); reviewed archive
identities are [`installer/archive-pins.json`](../../installer/archive-pins.json).
Linux Code native DEB identities are
[`installer/linux-code-pins.json`](../../installer/linux-code-pins.json).
Neither a package name nor a version print proves downloaded bytes.

| Surface | Identity and enforcement |
|---|---|
| Checkout bootstrap | Pinned Go archive and SHA-256, verified before extraction; build identity includes the runtime and embedded source inputs. Personal Go settings, workspaces and external cache programs cannot alter the build. |
| Private tools and desktop apps | Exact upstream URL/version/SHA-256, bounded safe archive extraction, declared required files, private generation publication and recorded content identity. Update reconciles reviewed pins in the checkout. |
| Rust and Python tools | Rust's six upstream components are independently verified and combined without upstream install scripts. LaTeX conversion and Yamllint use private venvs with pinned offline sources/wheels; complete Python identity is part of the desired state. |
| Claude Code and Codex CLI | Independent checksum-pinned vendor native platform archives, consumed without npm or Node. Claude binary hashes match its signed manifest; Codex retains package metadata and native helpers. Reviewed checkout pins own updates; personal accounts, settings and sessions remain unowned. |
| GitHub Copilot CLI | Checksum-pinned standalone single-executable assets from the official github/copilot-cli stable release, verified against its published SHA256SUMS.txt; no npm, Node, install script, MSI, winget or Homebrew. The managed shell sets `COPILOT_AUTO_UPDATE=false` so the reviewed pin owns updates; a self-update outside that environment is detected by Check and reverted by Update/Repair. Accounts, settings and sessions remain unowned. |
| Kiro CLI | Checksum-pinned fixed-version stable downloads from `prod.download.cli.kiro.dev`: the universal macOS DMG read through the private disk-image reader (bundle unmodified, desktop app never launched) and the Linux glibc ZIP; Windows is not offered (Kiro ships it there only as an MSI installer, which setup neither runs nor unpacks); the vendor `install.sh`/`curl` installer never runs. Pins come from Kiro's release manifest and were re-verified by SHA-256. One owned settings field disables its self-update so the reviewed pin owns updates; accounts, other settings and sessions remain unowned. |
| Neovim | Locked plugin sources, reviewed parser/tool recipes and a private generated runtime. Check loads the existing runtime without triggering package repair. |
| Windows Neovim formatters | Gersemi's official PyInstaller executable and clang-format's native Windows wheel binary have exact SHA-256 pins matching upstream GitHub/PyPI digests. Both are unsigned PE binaries, not Microsoft Authenticode packages. The complete clang-format wheel retains its licenses/metadata and requires the native VC++ runtime. Gersemi bundles Python and runtime libraries; its per-invocation temporary extraction is upstream behavior. Versions match the reviewed POSIX Mason registry, while native formatting proves behavior. |
| Shell/tmux plugins | Pinned source archives, source corrections restricted to reviewed fresh payloads, and retained upstream license/provenance. No runtime remote-script pipeline. |
| Sentinel | One full commit hash in `installer/sentinel.go`. Git fetches exactly that commit. `HEAD` must equal the pin, and each file must equal the tree of the pin, before the Sentinel installer runs. The commit hash is the integrity check. See [Sentinel pin](#sentinel-pin). |
| Code on Linux | Exact official DEB version/architecture/URL/SHA-256. Existing APT worker, transaction attribution and shared pool own installation/removal. Canonical `/usr/share/code/code` matches the distro sandbox profile. A fixed filtered debconf input preserves personal Microsoft repository/key/answer state. |
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

Archive/bootstrap and Linux Code DEB pin updates remain reviewed maintenance in
this major release. Neither the former Linux Code archive pins nor their DEB
replacement is extracted by Renovate.
Update hashes together with extraction/layout and native-consumption evidence.
Renovate covers only sources in its generated extraction inventory; do not claim
that it maintains archive hashes that it does not extract.

## Sentinel pin

`installer/sentinel.go` holds one pin: the full commit hash of the
[Sentinel](https://github.com/luisgui1757/sentinel) repository. Apply runs
these Git commands in `<state>/sentinel/checkout`:

```sh
git init
git fetch --depth 1 https://github.com/luisgui1757/sentinel.git <commit>
git checkout --detach FETCH_HEAD
```

`<state>` is `${XDG_STATE_HOME:-~/.local/state}/dotfiles` on macOS and Linux,
and `%LOCALAPPDATA%\dotfiles` on Windows. Git runs without system or global
configuration, so a personal setting cannot change the fetch. Git uses the
`HTTPS_PROXY` and `https_proxy` variables.

Then setup checks that `git rev-parse HEAD` equals the pin. It compares each
file with the tree of the pin and refuses an extra file. Git index flags cannot
hide a change from this comparison. Check, update and removal run Sentinel only
from a checkout that passes it. The commit hash is the integrity check.

A failed fetch stops the apply.
The report shows the Git output and tells you to check the network or the proxy.

Next, setup runs the Sentinel installer of that checkout. It uses
`tools/install` with Bash on macOS and Linux, and `tools\install.ps1` with
Windows PowerShell on Windows. Dotfiles does not change the Sentinel files.

Renovate tracks the Sentinel `main` branch with a custom regex manager and the
`git-refs` datasource. When `main` moves, Renovate opens a pull request that
changes the pin. All checks that protect `main` must pass on that pull request.
After you merge and pull it, **Update tools** applies the new pin.

## Agent CLI pin proposals

The Claude Code, Codex, Copilot CLI and Kiro CLI archive pins are proposed weekly
by [`agent-cli-pins.yml`](../../.github/workflows/agent-cli-pins.yml), which runs
[`scripts/update-agent-cli-pins.py`](../../scripts/update-agent-cli-pins.py). It
pins only each vendor's latest stable release, after the same byte checks used for
the reviewed pins: Claude's gpg-verified release manifest (key fingerprint
`31DDDE24DDFAB679F42D7BD2BAA929FF1A7ECACE`) plus npm SHA-512 integrity, Codex npm
integrity, Copilot's release digest plus `SHA256SUMS.txt`, and Kiro's release
manifest plus its Linux ZIP layout. A prerelease or changed layout
fails that tool for manual review. The resulting pull request runs the normal
required native checks; the bot never merges.

`GITHUB_TOKEN` cannot open pull requests in this repository, and its pushes would
not start the required checks. To let the workflow open them, the owner creates a
GitHub App owned by the account, without administration permission, with
repository permissions Contents: read and write and Pull requests: read and write,
installs it on this repository only, then sets repository variable
`AGENT_PINS_APP_CLIENT_ID` (the App's client ID) and secret
`AGENT_PINS_APP_PRIVATE_KEY` (a private key it generated). Until then each run
verifies, writes a job summary and uploads the proposed patch as the
`agent-cli-pins` artifact. Other archive pins remain manually reviewed.

Release publication follows [RELEASING.md](../RELEASING.md) and
`release/manifest.json`. Future release certification uses exact-head native
installer jobs and captured evidence; historical published proofs remain
immutable. Checked-in branch-protection policy is separate from live GitHub
configuration; its cutover is documented in
[branch protection](branch-protection.md).

The [previous supply-chain guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/security/supply-chain.md)
preserves Nix/chezmoi-era identities and enforcement for historical releases.
Those installers and their fallback package-manager chains are retired.
