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
| Sentinel | The latest published and immutable Sentinel release, never older than the floor in `installer/sentinel.go`. Git fetches exactly the commit of its tag. `HEAD` must equal that commit, and each file must equal its tree, before the Sentinel installer runs. The commit hash is the integrity check. See [Sentinel release](#sentinel-release). |
| Code on Linux | Exact official DEB version/architecture/URL/SHA-256. Existing APT worker, transaction attribution and shared pool own installation/removal. Canonical `/usr/share/code/code` matches the distro sandbox profile. A fixed filtered debconf input preserves personal Microsoft repository/key/answer state. |
| Ubuntu/Debian native packages | Actual APT/dpkg transactions and metadata; ownership is attributed to the operation, never an inventory difference. Native repositories resolve system packages under their own trust model. |
| macOS native packages | Actual Homebrew formula transactions and native dependency/linkage checks. Existing infrastructure remains unowned; verified fresh bootstrap is retained infrastructure. Apple CLT uses Apple's advertised signed packages. |
| Windows vendor tools | Pinned bootstrapper bytes plus valid Microsoft Authenticode and native registration/operation evidence. Build Tools resolves Microsoft's signed servicing packages; its bootstrap hash is not a claim that those changing components are individually pinned. |
| Windows Terminal | Pinned official unpackaged stable ZIP; scoped settings at the observed LocalAppData path. Store/Preview/Canary are independent. |
| Ghostty on Linux | One reviewed Debian-family archive per architecture, extracted privately; native APT supplies declared system libraries. No upstream repository-add script or DEB maintainer script executes. |
| T3 Code app | macOS: the official Homebrew `t3-code` cask. Setup accepts only its reviewed app artifact and a macOS version requirement, and `codesign` must prove the bundle ID `com.t3tools.t3code` and the Developer ID team `ARK85ZXQ4Z`. T3 updates itself. Ubuntu/Debian: the official `.deb` of each architecture with an exact SHA-256, extracted privately. Its maintainer scripts never run, so setup installs no AppArmor profile and no system link. Setup adds a start script to the payload, and the launcher and T3's own link handler use it. The script turns off T3's updater and Chromium's sandbox. Windows is not offered, because T3 ships the app there only as an installer. |
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

## Sentinel release

Install and update ask the GitHub API for the latest release of the
[Sentinel](https://github.com/luisgui1757/sentinel) repository. GitHub must
report it as published and immutable, and its tag must be `vX.Y.Z`. A second
request gives the commit of that tag. Setup refuses a release older than the
floor in `installer/sentinel.go` (v0.3.0), an older release than the recorded
one, and a tag that names another commit than the recorded one. The approved
plan binds the commit. Apply runs these Git commands in
`<state>/sentinel/checkout`:

```sh
git init
git fetch --depth 1 https://github.com/luisgui1757/sentinel.git <commit>
git checkout --detach FETCH_HEAD
```

`<state>` is `${XDG_STATE_HOME:-~/.local/state}/dotfiles` on macOS and Linux,
and `%LOCALAPPDATA%\dotfiles` on Windows. Git runs without system or global
configuration, so a personal setting cannot change the fetch. Git uses the
`HTTPS_PROXY` and `https_proxy` variables.

Then setup checks that `git rev-parse HEAD` equals the commit. It compares each
file with the tree of the commit as raw bytes and refuses an extra file. Git index
flags, local filters and replacement objects cannot hide a change from this
comparison. Check, repair, removal and resume use the recorded commit, and run
Sentinel only from a checkout that passes it. An install older than the floor
is the exception: repair installs the latest release or the floor, and removal
uses the floor. The commit hash is the integrity
check.

A failed fetch stops the apply.
The report shows the Git output and tells you to check the network or the proxy.

Next, setup runs the Sentinel installer of that checkout. It uses
`tools/install` with Bash on macOS and Linux, and `tools\install.ps1` with
Windows PowerShell on Windows. Dotfiles does not change the Sentinel files.

Trust moves from a reviewed pin to the Sentinel release process. Immutable
releases keep a published tag on its commit, and a tag ruleset on Sentinel lets
only its owner create, move or delete a `v*` tag. A new release reaches a
machine only through **Update tools**. Check shows it as a note. When the
lookup fails, setup keeps the installed release and shows a note, and a first
install stops. The native tests build the command with the
`sentinelfixedrelease` tag, so they install the floor release.

## Archive pin proposals

The weekly [pin workflow](../../.github/workflows/agent-cli-pins.yml) opens one
grouped PR. It updates the four agent CLIs and the GitHub tools listed in
[the update policy](../../scripts/archive-pin-updates.json), including T3.
Each GitHub recipe names the repository, release tag and asset for every target.
The script verifies each download against the published SHA-256 and checks the
required archive paths before it changes a pin. A missing asset, missing hash,
checksum mismatch, download above 512 MiB or changed layout fails that tool
and the workflow.
Other verified updates can still reach the grouped PR.

The four agent CLIs keep their vendor checks: Claude's signed release manifest
and npm SHA-512, Codex npm SHA-512, Copilot's digest and `SHA256SUMS.txt`, and
Kiro's manifest and Linux ZIP layout. The Claude release key fingerprint is
`31DDDE24DDFAB679F42D7BD2BAA929FF1A7ECACE`.

The bot reports major releases and leaves their pins unchanged. It refuses
prereleases and downgrades. Current versions need no download.
The PR must pass the required native checks. The bot never merges.

The policy lists every archive resource as automatic or manual. CI rejects a
resource without an entry, a stale entry, and a GitHub recipe with missing
targets. Manual entries include nested components and wheels. The job summary
lists these exceptions with their reasons. It does not check their latest
versions. The other pin files, including Go bootstrap, Linux Code, Tailscale,
Build Tools, VC runtime, Homebrew, Mason and Neovim plugins, still need manual
review. Renovate continues to own its declared extraction inventory.

The workflow uses the existing GitHub App with Contents and Pull requests write
permissions. Its settings are `AGENT_PINS_APP_CLIENT_ID` and
`AGENT_PINS_APP_PRIVATE_KEY`. The App's pushes start the required checks.
When verified updates exist but the App cannot open their PR, the job fails.
The job uploads the proposed patch and summary even when PR creation fails.
The workflow path and automation branch stay the same to preserve the existing
schedule and open proposal.

Release publication follows [RELEASING.md](../RELEASING.md) and
`release/manifest.json`. Future release certification uses exact-head native
installer jobs and captured evidence; historical published proofs remain
immutable. Checked-in branch-protection policy is separate from live GitHub
configuration; its cutover is documented in
[branch protection](branch-protection.md).

The [previous supply-chain guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/security/supply-chain.md)
preserves Nix/chezmoi-era identities and enforcement for historical releases.
Those installers and their fallback package-manager chains are retired.
