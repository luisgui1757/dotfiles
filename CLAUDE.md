# Repo guide for coding agents (and humans coming back to this cold)

This file is the on-ramp. If you're a future coding-agent session, read this
**before** touching anything. If you're me six months from now and forgot how
the install script works, read this too.

> **Single source of truth.** This file is canonical. `AGENTS.md` at the repo
> root is a thin pointer here so non-Claude agents discover it; do not copy this
> content there because two real guide files would drift. Claude Code auto-loads
> this file, and other agents reach it through `AGENTS.md`.

`README.md` is the operator guide. Keep its opening sections simple and
task-oriented: supported platforms/tools, install/update/remove commands, then
daily cheat sheets. When a user-visible key, command, config path, or platform
boundary changes, update the matching cheat sheet in the same change. Detailed
implementation rationale belongs later in the README or in this file; do not
make users read Nix, migration, or CI internals before they can operate a tool.

## Installer architecture

The user has authorized removing chezmoi outright, with an explicit migration
entrypoint instead of backward runtime compatibility. The reconciled design is
in [the installer plan](docs/plans/interactive-installer-overhaul.md#major-release-architecture-reconciled-2026-10-09).
The subsequent whole-product challenge removes Nix as well: portable tools use
private, checksum-pinned upstream archives on all three operating systems, with
one declared native provider for system/vendor packages. WSL uses the Linux graph,
without a WSL detector, host-support root or separate installer lifecycle. Probe
actual clipboard/graphical capabilities where consumed; do not infer them from
kernel branding or persist them as platform identity. Guest setup never provisions
Windows host resources. The unreleased Go Nix prototype, its workers and batch schema have been deleted.
Linux supports Ubuntu and Debian only, on amd64/arm64. The owner explicitly
removed Fedora, Arch, openSUSE, Alpine and Linuxbrew from this major release.
Legacy retirement separately observes Linuxbrew packages installed by the
released setup. That observation grants no Linuxbrew provider to normal setup;
shared manager infrastructure and outside dependencies stay protected.
Reject other distribution IDs before provider discovery; ID_LIKE does not widen
support to derivatives. APT provisions Linux system packages. The owner additionally authorized Flatpak
for Linux desktop applications on 2026-10-13; use private per-user installations,
retain native infrastructure and personal app data, and never mutate unrelated
user/system Flatpak installations.
APT discovery happens only at the process boundary. Its constructor receives
observed executable paths and privilege context. Foreground authentication is
injected only for the interactive command after approval; the saved worker always
uses `sudo -n`, or direct commands for an observed root process. Attribute package
ownership to operation-specific APT/dpkg records, never inventory differences.
The dpkg source-package identity does not prove repository provenance. When an approved APT operation introduces a later selected package incidentally,
claim it only from that exact earlier completed operation in the active plan;
keep its originally absent baseline and native automatic/manual classification.
Do not mirror APT's dependency graph in the catalog. A removed native root still
needed by another proved owned package moves to the owned pool with a truthful
presence/disclosure and exact removal completion proof. The final native consumer
removal can collect it. Outside or source/classification-changed consumers remain
protected; arbitrary old completion records cannot widen a new approval.
macOS supports Apple Silicon only. Platform discovery, archive selection and the
planner share the same target validation; reject Intel/Rosetta amd64 execution
before provider discovery. Archive metadata must not contain unsupported targets.
The hosted matrix asserts its actual OS/architecture, including darwin/arm64.
The current setup/migrate wrappers use the verified Go bootstrap. Implementation
is complete; delivery readiness requires all 23 final-head native/quality checks
and independent review. [Installer status](docs/installer-status.md) records the
acceptance contract and dated evidence. Retired implementation rules are preserved in
[the previous guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/CLAUDE.md).
Configuration outcomes and personal data remain requirements after runtime removal.

Git configuration retirement uses the existing explicit per-item removal choice
for a replaced whole-file mapping. Changed or uncertain entries require intact
baseline/journal evidence, preserve their current bytes separately, and restore
the first baseline before the scoped successor can take ownership. Missing
resource-specific profile evidence stays Pending locally. Validate historical
profile journals before using paths; historical lock-cleanup failures are
disclosures, while current/unsealed operation failures retain recovery intent.
Recovery-only mappings have no dependencies and cannot enter Keep, adoption or
new publication. Declining their optional restoration preserves the file without
adding a Keep root. Saved cancellation/restoration proof for an uncertain first
publication may release its bookkeeping, never claim or rewrite the personal file.
Git configuration discovery errors must remain routable to resource-local Pending
even when no target could be bound. Preserve the original cause and saved paths.

The new gh-dash capability runs its verified upstream binary as `gh-dash`.
It still requires GitHub CLI for authentication and configured actions, but has
no separate extension registration or authenticated install step. Preserve the
user's existing extensions; released `gh dash` migration is handled at cutover.
Claude Code and Codex are independent native CLI capabilities, replacing Pi by
owner instruction on 2026-10-12. Use their checksum-pinned standalone distributions
through the existing private archive lifecycle, with Git and shell discovery as
automatic prerequisites. No npm-prefix integration or Node dependency is needed.

T3 Code uses official standalone archives and a browser UI on all supported
targets by owner choice on 2026-10-13. On 2026-10-17 the owner moved it to the
stable channel after Orchestrator V2 reached stable v0.1.0. Use fixed stable
release pins, with all four archive checksums reviewed together. Keep the bundled client
and native node_modules with the binary. No Node provider, desktop integration,
background service, provider authentication or personal T3 state is installed.
Codex and Claude Code remain independent choices; T3 owns interoperability.

The owner requested OpenWhispr, Obsidian, Tailscale and Zen on 2026-10-13,
allowing Windows omissions when the existing channels do not support them.
The current additional application work uses private macOS ZIP/DMG bundles,
Zen's Linux XZ archive and Tailscale's official APT input/Homebrew formula.
Read disk images only through a private readonly mount and the bounded archive
reader; detach exactly that mount. Keep compressed-byte verification, tar path/
link checks and decoder failures intact. Tailscale's normal DEB removal retains
state; never purge it or enroll a machine with user credentials automatically.
The owner approved Flatpak for Linux OpenWhispr/Obsidian on 2026-10-13;
Obsidian used a private per-user Flatpak store, including isolated system/config
lookup and a required private runtime. Preserve legacy saved command recipes.
By owner decision on 2026-10-22 Linux Obsidian is the official portable tar.gz
(`tool.obsidian-portable`, amd64 and arm64, glibc) through the private archive
lifecycle, launched with `--no-sandbox %U` and declared only as required files:
no command on PATH, where a plain `obsidian` would start sandboxed. The private
Flatpak failed on a real Ubuntu 24.04 desktop twice: IT tooling had set 89
AppArmor profiles, including the stock `flatpak` one, to `complain`, so bwrap
could not map its user namespace; and Electron's zypak helper spawn goes through
the Flatpak portal, whose `flatpak run --commit` looks only in standard
installations ("commit ... not installed"). Hosted CI had neither a portal nor
Ubuntu's restriction. Chromium's own sandbox needs unprivileged user namespaces,
which Ubuntu restricts unless a profile names the binary's path (the stock
`obsidian` profile covers only `/opt/Obsidian/obsidian`). Disclose the disabled
sandbox in the preview; never change AppArmor, sysctl or install a setuid
`chrome-sandbox`. `tool.obsidian` keeps its Flatpak binding only so earlier
receipts are observed and removed (the capability no longer requires it on
Linux; keep FlatpakDriver and `tool.flatpak`, which is retained). The launcher
keeps `dotfiles-obsidian.desktop`, so its owned receipt is repaired in place, and
declares the vendor's `StartupWMClass=md.obsidian.Obsidian` (Electron uses the
app's `desktopName`). Update installs the portable build but retains the old
Flatpak; Choose tools removes it, preserving `~/.var/app` data and the private
runtime. The Linux desktop lifecycle runs under the stock
`kernel.apparmor_restrict_unprivileged_userns=1`
(`scripts/prepare-disposable-linux-host.sh userns`), and the native Flatpak
lifecycle uses a test-only capability to keep that removal path proven.
The owner subsequently approved testing OpenWhispr’s official Linux amd64 portable
archive; no Linux arm64 asset is published. Its vendor launcher’s automatic
no-sandbox behavior on restricted-userns systems must be disclosed in the preview.
Do not change kernel/AppArmor policy or fall back to the inspected DEBs:
vendor maintainer scripts delete model caches or overwrite/remove a pre-existing
AppArmor profile. Do not route those DEBs through the new APT input adapter.

Native package contract tests require explicit `DOTFILES_TEST_NATIVE_PACKAGES=1`
on a disposable runner. They intentionally exercise the real package database;
ordinary local gates must not enable them. Attribute dependencies to the actual
operation-specific manager record, never an inventory difference or timestamp.
The isolated Homebrew test configuration must copy only already trusted sources
from `brew trust --json=v1`; outside cask inspection otherwise loses the runner's
existing trust. Compare inventory stdout with stdout, keeping diagnostics separate.
Exact removal must protect consumers at execution and leave unrelated orphans.
Homebrew formula removal uses `uninstall --formula --force` on the exact owned,
unheld set: that flag removes every installed version but retains native outside
dependency checks. `--ignore-dependencies`, cask `--force`, and autoremove are
forbidden. Recheck pins before dispatch because formula `--force` itself unpins.
Manual recovery advice must also disable autoremove; reinstall advice additionally
disables automatic install cleanup, including Homebrew's periodic full cleanup.
Report surviving incidental packages after removal. Keep ownership of an
automatic dependency still needed by an outside consumer, but relinquish a
manually promoted or source-replaced pool entry. Save the disclosure before
publishing that ledger change; old receipts must not report a now-absent package.
The executor refreshes removed-resource disclosures from its final verified
observations, because a later operation may collect an earlier retained dependency.
Unknown or changed artifacts cannot erase a prior disclosure.

Homebrew discovery runs only at the macOS process boundary. Inspect the declared
bootstrap executable when PATH has not refreshed, then read the actual executable,
prefix and Cellar; never infer existing locations from architecture. Constructors
remain pure. Git and tmux use the formula provider and add its bin directory only
when selected by the graph. Existing Homebrew infrastructure is reused without
acquiring removal authority. Discovery failures are local to its resources;
independent archives remain available. Queries are bounded and cannot elevate;
mutations still require the native worker and approval guard. The pinned fresh Homebrew bootstrap is connected, retained, and followed by a new
inventory preview. Its stable bootstrap-only identity prevents tool updates from
invoking global manager migrations. Fresh-host acceptance remains separate from
existing-manager reuse and from missing-payload Apple Command Line Tools provisioning.

Apple compiler/Make use hidden retained `infra.apple-clt`. Healthy selected Xcode
or CLT stays unowned; only canonical payload absence permits Apple's advertised CLT install,
followed by selection and native compiler/SDK verification. Historical Apple BOM membership does not prove a file is present or belongs to the canonical CLT payload. Missing-payload fixtures validate receipt paths before any move and fingerprint every existing outside receipt file without moving or deleting it; invalid paths and metadata/query failures remain fatal. Never reinstall
Homebrew, upgrade all Apple software or remove developer infrastructure. A saved
successful package phase may resume selection; unknown worker outcomes require
native inspection, never a speculative repeat. Clear toolchain environment
overrides in read-only queries. Bash 3.2 recipes require explicit failure exits
where a failed conditional would not obey errexit.

Rust combines the six pinned upstream components in a private archive generation,
without rustup or executing upstream installation scripts. Windows compiler
entrypoints copy the observed installer executable, bound by its SHA-256 in the
saved operation pin, and forward to the preserved upstream siblings with an
extended-length sysroot. Desired Rust identity includes the launcher recipe
revision; retain the executable hash in the operation/version provenance and
verify the copied bytes through the payload snapshot. An unrelated installer
rebuild must not repair an intact Rust generation. Preserve direct/argfile caller
overrides and Clippy's
first-positional Cargo protocol via its existing SYSROOT input. Forward streams,
exit status and console interrupts; never shorten the ownership path or add a
Python dependency to Rust. LaTeX conversion uses
a private venv with pinned, offline hash-checked sources; its desired identity
includes the complete Python pin and fixed console recipe revision. The managed
`latex2text` console sets UTF-8 stdin/stdout/stderr before conversion, including
Windows pipes; parent code pages must not control rendered math. Use pinned pip
to generate its native console launcher, before bytecode and archive fingerprints.
Consume Python through its stable managed path.
Publish only each private environment's declared console in a separate `commands`
directory. Never put the venv's bin/Scripts directory on the user PATH: it exposes
private Python, pip and activation commands and can override the selected Python.
Copy the generated console unchanged before bytecode/fingerprinting; its embedded
absolute interpreter remains private. Saved bin/Scripts recipes keep their exact
layout during recovery. Windows command-only pins require venv revision 1.
Preparation uses Python's exact lowercase `nul` spelling to disable pip
configuration; Go's uppercase `NUL` is a device alias but fails pip's literal
`os.devnull` comparison. Neovim pip queries use the same lowercase
spelling. Verify isolation against an actual venv-local pip.ini on Windows.
Windows Python compilation uses an extended absolute target and compileall's
`-d` with the previous filename spelling. Keep checked hashes and all three
optimization levels; this changes filesystem access without changing base
bytecode identity. The base compiler retains its payload cwd, which Windows
locks against recovery renames while the child is alive. Private Python venvs
also use extended interpreter, destination and requirement paths. Escape the
drive colon in their local file URLs: pinned pip otherwise normalizes away the
device prefix. Bind that changed venv preparation into `windows_venv_revision`;
completed revision-0 provenance remains readable, but unfinished preparation
requires explicit recovery. Do not change system long-path policy or shorten
managed generations. Verify ordinary console use, immutable payloads and stable
Python replacement on native Windows with long paths disabled.
Windows Neovim uses hidden archive prerequisites for Gersemi and clang-format,
at the same versions as the reviewed Mason registry. Gersemi is the official
standalone executable; clang-format uses the native binary from its complete
Windows wheel, including license/metadata files, without installing Python
wrappers. Declare its VC++ runtime dependency. POSIX retains Mason's providers.
Do not rewrite the locked Mason checkout or change system path policy. Verify
actual formatting and the full archive lifecycle at production generation depth.
Neovim recovery applies only to an in-progress receipt. Abandon must preserve
and disclose an unpublished stage and its journal, then allow a fresh approved
operation. If the current pointer has moved from the saved previous target,
finish publication recovery before abandonment; never invent completion proof.
Native finalization visits profiles, configurations and archives before Neovim.
A refused Neovim Abandon may therefore follow sealing of completed journals or
cleanup of independently proved obsolete archives; its own publication and
recovery evidence remain intact. Do not infer whole-driver immutability from a
Neovim-only refusal test or reorder finalizers without checking their contracts.
Unreadable saved Neovim evidence is resource-local Unknown/Pending, with trusted
receipt-derived preservation paths and no completion/resume authority. Preserve
active corrupt intent and refuse Abandon when its prior publication cannot be
proved; a receipt baseline is not necessarily the pointer at operation approval.
Apply the same local Unknown/Pending boundary to the current generation's journal:
deselection must retain an unproved runtime without starting a removal transaction.
Abandon refusal must name the journal and the unproved prior publication.
Windows uses full PortableGit (Bash, SSH, LFS, credential manager and GUI payloads)
and GNU Make. Make's approval scope comes from its actual provider: the Windows
archive is private/user; APT and Apple observations remain machine-scoped. PortableGit's fixed preparation runs only inside the private payload;
its upstream SHA-256 is not a Microsoft Authenticode claim.
Enable `core.longpaths` only in that private distribution's `etc/gitconfig` before
fingerprinting. Its recipe revision changes desired identity; completed older
pins remain readable for update, while unfinished older preparation requires
explicit recovery. Lazy, Mason and sync verification inherit this system setting;
the pinned-checkout helper deliberately isolates Git configuration and therefore
passes its own `-c core.longpaths=true`. Preserve caller Git override precedence
and personal configuration. Native acceptance must exercise long physical paths
without global settings or shortened fixtures. Do not expose Git's
`usr/bin` ahead of Windows system utilities. Reuse the archive lifecycle for all
these packages, including preservation of modified generations.

The new tmux integration loads four verified plugin archives directly; TPM is
not part of the new graph. Missing managed plugin attachment must not invoke an
old TPM checkout. Keep released `home/dot_tmux.conf` bytes passive and unchanged.
Preserve session snapshots and existing servers on removal. Do not invoke continuum's automatic-start handler: even its disabled
branch can remove a personal LaunchAgent or disable a systemd user service.
Pinned source corrections are exact text/count replacements applied only to a
fresh verified payload. They never execute commands or patch an existing install.
Excluded members must be exact reviewed non-runtime files, still subject to path,
duplicate and link-boundary checks. Resurrect's source archive has three dangling
test-submodule links; its runtime does not need that submodule. Released live-link sources remain passive migration evidence; they are not
executed by the new installer.

The native config manifest is `installer/config-targets.json`; it points only to
canonical top-level sources. Resolve actual Windows known folders independently.
Never re-create `Documents` or AppData paths under a guessed home. New Windows
config application uses file copies and a native directory junction for Neovim,
while POSIX retains live links. Junction creation requires no symbolic-link
privilege. Snapshot the junction itself, never its referent; resolve redirected
parents with native handle paths because Go's EvalSymlinks does not follow
Windows mount-point junctions. This is an intentional major-release behavior change.
Use that same native resolver for managed executable descendants in acceptance
fixtures: Go reports an intermediate junction as ENOTDIR (Windows path-not-found).

Shell integration owns delimited profile blocks rather than full profile files.
Outside edits do not invalidate block ownership and must survive removal.
A block's leading newline belongs to it, so a setup-created profile opens with an
empty line; formatters drop it (shfmt on a Neovim save, 2026-10-24). A start
marker that opens the file, after any BOM, is the same block with that newline
missing: its bytes compare as written and replacement restores the line. Text
before a marker is still an edit. Marker errors name the profile file.
Choose an existing managed block's line endings from that block, including
Sentinel policy, so surrounding personal edits cannot change its desired bytes.
Saved UTF-16 blocks omit the file BOM: accept only validated marked fragments in
the journal reader; full profile files still require their BOM. Preserve the
schema-1 byte representation and all original encoding during recovery.
On Windows, match ERROR_INVALID_OWNER at the metadata-copy boundary and preserve
its error chain. Explain the exact owner/path and same-account, same-checkout,
one-resource elevated recovery for BUILTIN\Administrators, followed by ordinary-user
remaining work. Other owners require explicit ownership resolution outside setup;
do not promise that elevation can assign arbitrary SIDs. Never
automatically take ownership/elevate, or turn a healthy administrator-owned file
into Pending. Its read-only Check and unrelated selections stay available.
Native restricted-token tests must cover refusal before mutation, exact elevated
recovery with unchanged SDDL, and ordinary-user completion, removal and restoration. Keep
encoding/BOM and line endings intact, preserve native metadata when staging, and
never follow an existing profile symlink into an unapproved source. Saved profile
intent precedes publication; an identical target with the staged file still
present does not prove ownership. Native Windows public-command tests run only on
explicitly enabled disposable GitHub accounts; HOME cannot substitute for known
folder discovery. The owner chose to release the native installer as v1.0.0
(2026-10-25) after PR #87's full hosted matrix, including the public bootstrap and
migration lifecycles; publication follows docs/RELEASING.md.

The checkout bootstrap verifies the pinned Go archive before each extraction,
extracts into private staging and keeps only the compressed toolchain archive as
its trusted reusable input. A cached compiler's version output is not an integrity
check. By owner request on 2026-10-20 an unchanged checkout reuses its verified
build without extracting or running the toolchain: a fingerprint of the pin, the
launcher and every file under `installer/` (a superset of the compiler-selected
inputs) maps to the build identity. Record that mapping only when the fingerprint
is unchanged across the full verified build. Any change, link or special file
there takes the full path, and the cached binary hash is still checked before
every launch; archive, compiler-version and module checks apply whenever the
toolchain is needed. Go runs with personal settings/workspaces and external cache programs
disabled; source identity includes runtime and embedded inputs, not test files.
Released configuration-target inventory is migration evidence, never removal
authority on its own. The passive checkout-relative Windows LazyGit link target retains its recorded release hash and LF bytes even with `core.autocrlf=true`; it is independent of the active shared default-keybinding configuration.
Windows PowerShell 5.1 has known per-byte download progress overhead. Suppress
that progress only around the pinned download and restore the caller preference
on success/error; retain phase diagnostics on stderr, basic parsing, TLS and exact
size/hash verification. A fixture deadline is not permission to raise the timeout
or change download transport without evidence.
Extract the verified Go ZIP with the built-in .NET `ZipFile.ExtractToDirectory`
API. The Windows-era archive module adds per-entry script/provider/progress work
across more than 15,000 Go files, even with progress hidden. Keep fresh staging,
checksum-before-extraction, error propagation and the original physical paths;
do not install another extractor or change global .NET/registry path settings.
Ordinary Windows regression tests use system PowerShell 5.1 for this extraction
action; only the disposable public lifecycle proves the complete bootstrap.
The isolated extraction fixture must perform the preceding archive Get-FileHash
verification before setting progress to Stop. On Windows PowerShell 5.1 that
mandatory phase imports Utility with legitimate first-use progress; Stop applies
to extraction work, not to a fixture-only cold Add-Type module import.

Neovim synchronizes locked plugins, parsers and Mason tools into private runtime
generations under its independently observed XDG data directory (nvim-data on
Windows). Reuse the archive publication/cleanup primitives; preserve personal
Shada, undo, sessions and legacy data. Check uses a clean headless runtime and
never invokes Lazy/Mason installation. Keep large runtime manifests in fixed
4096-entry chunks, bound by the operation journal; missing or changed chunks
block publication/removal. Pass managed tool paths explicitly, never inherit a
caller PATH into construction.
Mason uses the byte-identical upstream compiled snapshot and release/hash lock in
`nvim/mason-registry/`, reproduced by `scripts/update-mason-registry.py`. Verify
its bytes before loading recipes; use Mason's existing fixed GitHub registry
format. Derive concrete selected tool versions from that snapshot and verify
canonical installed source IDs plus registry provenance before publication and
during Check. Do not duplicate version lists or let refresh resolve latest.
Registry/top-level version pins do not imply immutable transitive npm/PyPI bytes.
Only explicit `DOTFILES_NVIM_SYNC=1` staging may provision managed dependencies.
Published startup validates its Lazy checkout, plugin/parser presence and Mason
registry without replacing executable payloads; changed/missing dependencies
require approved Update/Repair. Preserve unmanaged Neovim's existing workflow.
Mason setup precedes LSP enablement when a file is opened on the command line.
LuaLS logs and generated metadata live under Neovim's user state directory,
outside the managed package. Native CI opens each configured LSP in a fresh
process and verifies that language use leaves the published payload unchanged.
Set `PYTHONDONTWRITEBYTECODE=1` only in sync/check child environments: npm's
vendored node-gyp imports Python inside the immutable Node archive. Suppress that
cache at its consumer boundary; never ignore bytecode in archive fingerprints or
rewrite personal Python settings. Ordinary external npm builds also import that
vendored source. Node pins apply one exact checked source replacement immediately
before gyp's import, setting Python's documented `sys.dont_write_bytecode` within
that process. This covers callers outside sync without changing personal Python
or npm settings. Replacement metadata changes desired identity and old completed
pins remain readable for normal update; do not add a wrapper or weaken payload
fingerprints. Prove all four upstream source matches and an actual offline npm
install with complete Node snapshot/check/removal. Windows ZIPs omit development
headers, so the focused native fixture directly executes the same gyp import
entrypoint there; the full Windows Neovim lifecycle remains npm integration proof.
Windows compiler environment is acquired after toolchain installation, not during
pure construction. Linux Mason download/ZIP
prerequisites are hidden APT dependencies, proved absent before fresh-container
Neovim installation. Disable the optional Lua bytecode cache on normal startup
as well as staging: its path-encoded filename can exceed filesystem limits.
Plugin lazy loading and its startup budget remain required. The locked Obsidian plugin
is always provisioned and verified, including with a private sync HOME. Only its
configuration activation depends on an existing vault; never create a vault or
leave first real use to install an unmanaged plugin.
On Windows its dependency graph includes the existing shared PowerShell 7 archive
before synchronization, so PowerShell Editor Services can start from the managed
PATH without a separate shell selection. Strict smoke uses the same managed
Mason bundle path as production and still fails when either pwsh or PSES is absent.
Native fixture known folders must be sampled before overriding USERPROFILE;
Windows expands them from that process context, even without changing LOCALAPPDATA.

Visual Studio inventory is a counted bag of id, version, chip, language, branch,
type and extension tuples. The vendor returns an array without a uniqueness
guarantee. Preserve repeated rows and ordinal distinctions; compare multiplicity
before mutation. Canonicalize order at the Go boundary. Schema-1 package-map
journals lack multiplicity and require explicit recovery without reinterpretation.

Build Tools inspection requires both a zero exit and exactly one execution witness
from the compiled program; an empty successful process result does not prove the
compiler/SDK probe ran. It runs inside a private Windows Job Object. Create the
query suspended with atomic job-list assignment, then
resume it and verify all
owned descendants have exited before returning; compiler telemetry must not turn
the next observation into a new external consumer. Keep the 120-second inspection
bound and the output bounds, with a separate five-second termination bound shared
by every captured member. The bound was 30 seconds until 2026-10-23, when a cold
first inspection (VsDevCmd plus the compile probe) on hosted Windows Server 2025
with Visual Studio exceeded it and failed the installer. Capture exact process
HANDLEs from that private job and verify membership before termination; wait for
HANDLE signals, not merely exit codes or zero ActiveProcesses. Windows can return
a short successful PID list during natural exits despite spare buffer capacity.
Re-query only that state until a complete list is available within the same
five-second reap deadline. Preserve
the original lifetime Total baseline across every query; never retry API errors,
overflow, invalid counts/PIDs or HANDLE ownership failures. Refuse a still
incomplete or newly extended membership instead of claiming completion, and
never terminate a PID outside the job. Preserve
pre-existing consumers, machine telemetry settings and vendor files. Do not apply
this cancellation policy to the durable native mutation worker. Neither filtering
`vctip.exe` by name nor retrying a changed plan is an ownership proof.
The job-list API requires Windows 10 / Server 2016 or later, matching the Go
toolchain's Windows baseline; native acceptance uses Windows Server 2025 and a
Server Core 2022 container. This baseline does not certify every selected desktop
application on every server edition.

The VC++ runtime provider verifies pinned bytes and Microsoft Authenticode before
elevated vendor execution. Registry installation evidence and both Burn package
completion records must match the operation. Preserve existing bundles; registered
consumers block removal of an owned shared runtime. A reboot exit remains a
needs-action result until registration and native DLL health can be verified.

Authenticode `Valid` is the authority for signed timestamp lifetime. The separate
Microsoft chain-ownership check may disregard only present-day expiry, only after
Windows accepted a timestamped signature; any other chain failure is fatal. See
[Microsoft's timestamp contract](https://learn.microsoft.com/en-us/windows/win32/seccrypto/time-stamping-authenticode-signatures).
Compare runtime registry versions numerically: `v14.51.36247.00`,
`14.51.36247.0` and `14.51.36247` identify the same revision. Differing registry
views may resolve to the newer version only when the sole Microsoft bundle,
both MSI provider versions in every observed view and all three System32 DLL
versions corroborate it; normal signature/load checks still determine health.
Disclose older registrations as outside consumers that block removal. Never
prefer a registry view, silently take the maximum, or edit those registrations.
Other disagreements remain failures. Completed original VC++ command encodings
are accepted only by exact digests tied to the preserved historical Git blob;
unfinished old commands remain refused without rewriting or dispatch.

Discover `ZDOTDIR` only at the process boundary and pass it in `ConfigFolders`.
The application constructor must never inherit profile destinations from its
caller's environment. Every native fixture validates all write targets against
its disposable home before creating any profile.

Adoption requires a provider capable of restoring its baseline and a separately
approved resource choice. Do not infer ownership from matching pre-existing bytes.
An interrupted composite resource uses its saved operation and an observation-bound
recovery token, then requires a fresh remaining-work preview. A retry must not
silently replace the saved payload, operation ID or original adoption baseline.

A first baseline records an absent target's missing ancestor folders (nearest
first, bounded). Profile and configuration removals that restore that absent
original remove those folders during finalization, after the adjacent workspace
is gone, only while each is still an empty real directory under unredirected
parents (rmdir semantics; links/junctions refused). A kept folder keeps its
ancestors. The cleanup is cosmetic: an unexpected failure discloses the folder
as preserved and never blocks finalization, Retry or Abandon. Older baselines have
no record and keep their folders. Publication that keeps an absent profile
absent creates no parent or workspace.

Observation.Notes carries informational guidance (rendered "Note:"), such as
Sentinel advisories and Impeccable's engine setup. UnverifiedApplications stays
for external applications that were genuinely not exercised. Result, Check and
review displays omit preserved paths that definitely no longer exist; receipts,
journals and ownership decisions keep every recorded path.

Config observation separates `Desired` (reviewed copy bytes or live-link source
type/path and destination) from
`Fingerprint` (the installed artifact). Do not mix them: a source update must not
invalidate existing target ownership, but must invalidate stale apply approval.
Resolve parent redirections without creating directories. Snapshot a user link
itself, never its referent. Native symlink creation modes vary with OS/umask;
compare link destinations rather than assuming mode 0777. Preserve/fingerprint
regular file and directory permissions.

The native configuration writer records its first baseline separately from each
operation. Adjacent staging keeps renamed files on their original volume and
preserves native ACLs. Publication uses pinned directory handles and native
no-replace rename; never replace that with a check followed by ordinary rename.
The durable operation reserves a fresh, identity-named workspace; a second owner
marker adds no authority. Persist a bounded list of file/link hashes and directory modes before cleanup.
Remove entries individually, never with recursive deletion on retry; missing entries
may be our own partial cleanup, while changed/new entries and previously disclosed
personal paths remain preserved. A legacy `discarding` boolean grants no authority. Seal finished
journals and leave their baseline checks to the affected resource's observation.
Ignore unpublished metadata scratch when enumerating intent, and preserve foreign
workspace contents. Unstarted publication can be abandoned even if the user has
since edited its untouched target. Never discard the original baseline during an
update. A changed baseline blocks mutation; missing source blocks apply through
`ApplyBlocked`, while removal can still restore the independent baseline.
An uninspectable completed target is `Unknown` with a resource-local explanation,
never proved absent; it must not prevent independent resources from progressing.
An explicitly approved replacement of an edited managed configuration retains
the first baseline and saves the edits separately. Keep and report the saved
paths through update, removal and reinstall; these references never authorize
deletion. Update/repair may offer this choice for an already managed resource,
but must not adopt unrelated pre-existing resources. Native directory renames
validate opened parent identity and flush those same pinned handles on POSIX.

The same explicit replacement choice applies to edited or partially missing
owned profile blocks. Validate their original baseline against the receipt and
current target paths before offering it. Preserve the replaced profiles under
their operation workspaces, disclose those paths, and retain the first uninstall
baseline through subsequent updates and removal. Unrelated pre-existing marked
blocks are not eligible for this owned-resource replacement. A declined resource
leaves the selection unfinished; archive that intent before changing the choice.
Actual process-death tests must cover a completed first target as well as an
unpublished first target; observing only the first backup rename proves neither.
Watch the operation ID in the active engine receipt: an earlier adoption's saved
profile must never trigger a later operation's crash fixture. Assert saved bytes
and their durable references after recovery and subsequent removal.
An inverse must never move an active file aside when its required original is
missing. An empty managed-block fingerprint alone cannot prove that a personal
profile was restored. Report the affected recorded paths when recovery remains
incomplete, including a removal whose intended output was absence.

Archive generations use the saved operation ID, not the archive checksum.
A damaged owned package requires explicit replacement approval and a fresh
verified generation; retain/disclose its old directory through later operations.
A pointer-only repair may reuse an unchanged generation. Archive schema 2 replaces
an unreleased prototype shape; reject schema 1 resource-locally, never infer its
ownership. Cleanup records retain publication provenance after partial deletion.
A transient sharing/permission failure remains retryable and cannot be relabeled
as permanently preserved personal data. Declared archive commands must actually
have POSIX execute permission; Windows retains its native executable semantics.
Metadata staging reserves each POSIX output exclusively; failed copies remove
only that exact created file. Linux copies/verifies ACLs and xattrs through native
syscalls, including removal of inherited extra attributes; no external copy
program is needed. Windows failure cleanup handles the private copy's read-only attribute.
Native tests must exercise ordinary/restricted-token permissions, not only the
administrator runner's default token.
OneDrive marks the folders it syncs ReadOnly (plus a cloud reparse point), which
includes workspaces beside profiles in a redirected Documents. Windows refuses to
delete a read-only directory and Go's os.Remove clears that attribute only for
files. Remove the installer's own emptied workspaces, recorded cleanup directories,
settings-lock stages and created folders through `removeOwned`/`removeEmptyDirectory`:
clear the attribute on a real directory, retry once and restore it if the folder
survives. Never clear it on links, occupied folders or unrelated paths.

The quiet zsh login profile from PR #88 adds an existing `~/.local/bin` once and
sources the unmanaged `.zprofile.local` hook. It must not load interactive widgets
or prompt setup into noninteractive remote-app streams. The major-release shell
owner must preserve this outcome. The OS-locked publisher in this branch already
supersedes PR #88's mkdir/noclobber fix: keep its actual concurrency and process-death
coverage until the replacement proves serialization at every publication boundary.
Hyperfine budgets compare unrounded wall-clock milliseconds from either supported
JSON shape; a fractional overrun must fail.

## Product configuration invariants

Keep these behaviors while changing the installer. The implementation sources are
the top-level tool configuration and `installer/shells/`; passive released files
under `home/` and `shells/` exist only so old live links survive migration.

- Set Neovim's Space leader before loading Lazy. Only Rose Pine is eager, with
  priority 1000; DAP UI stays lazy. Do not resurrect the deleted duplicate
  `plugins.lua`, `ai.lua`, `avante.lua`, or `none-ls.lua` plugin specifications.
- Conform is the only format-on-save handler. It synchronously completes within
  the strict language smoke's ten-second budget. `vim.b.skip_format_on_save`
  implements the explicit one-save `:WNF`/`:wnf` escape hatch.
- Use `vim.uv`, colon-form `client:supports_method(...)`, and
  `vim.lsp.log.set_level(...)`. Never disable Node TLS certificate verification.
- render-markdown owns Markdown rendering, including the LaTeX parser and pinned
  `latex2text` conversion. Obsidian's duplicate UI stays disabled. Its locked
  plugin activates only when an existing vault resolves; opening Markdown must
  not create personal directories. `NOTES_VAULT` may select an existing vault.
- Do not install alternate copies of Neovim's bundled `c`, `lua`, `markdown`,
  `markdown_inline`, `query`, `vim`, and `vimdoc` parsers or queries. Purge only
  managed overrides below the private data/runtime tree through checked deletion;
  never delete the editor's upstream runtime. The strict language preflight must
  exercise parsers/queries as well as real LSP attachment and formatting.
- Executable plugin caches need a locked full commit, expected origin, usable
  worktree, clean tracked/untracked state and entrypoint before runtime loading.
  Stage repairs beside the target and publish under the existing ownership lock.
  An unprovable lock is not stale merely because time passed. Lazy additionally
  needs locked branch/origin metadata for lockfile serialization, while remaining
  detached at the approved commit.
- zsh completion remains Tab-driven fzf-tab over native compinit, followed by
  autosuggestions. Reclaim Tab after fzf's own bindings; retain Ctrl-R/Ctrl-T/Alt-C
  and prefix Up/Down history. Enable vi mode before installing keybindings.
  Compose cursor hooks with Starship and autosuggestions. Never bind bare Escape
  to kill-whole-line or set an unusably short KEYTIMEOUT. Keep inherited PATH
  deduplication with first-occurrence ordering via native zsh path semantics.
  The shared profile loader moves selected managed directories ahead of system
  and personal entries even when inherited PATH already contains them. Deliberate
  personal overrides run afterward via the documented local profile hooks. Prove nested macOS
  login behavior with the actual system profile/path_helper and generated user
  profiles; sourcing an isolated environment fragment alone misses that boundary.
- The optional `zsh-handoff` capability (owner request 2026-10-23, for directory
  accounts whose bash login shell `chsh` cannot change) makes the shared init hand
  an interactive terminal bash to the zsh setup observes (`/bin/zsh` on macOS,
  APT `/usr/bin/zsh` on Linux) before any other initialization, exporting
  `SHELL` so Herdr, tmux and Neovim terminals, which start `$SHELL`, follow. It
  never acts for `bash -c`, a non-terminal stdin/stdout, an account whose
  `SHELL` is already zsh, or a set `DOTFILES_ZSH_HANDOFF` (so `bash` typed in zsh
  stays bash). macOS bash 3.2 exits on a failed exec while reading startup
  files: keep `execfail` set around the exec only, then restore it, `SHELL` and
  the marker so a broken zsh leaves a working bash. The receiving zsh sources
  Ghostty's documented zsh integration once (marker 1 becomes 2), as Ghostty does
  for a zsh it starts. The real login shell stays bash; never call chsh or edit
  passwd/directory data. `TestZshHandoff*` drive a real interactive bash on a
  pseudo-terminal with a recording zsh fixture.
- Selected `config.nvim` makes Neovim the POSIX editor (owner request 2026-10-24):
  the shared environment exports `EDITOR=nvim VISUAL=nvim` and `SUDO_EDITOR` as
  the managed command's stable `current` path. sudoedit ignores PATH (Ubuntu's
  `secure_path`, sudo-rs), so a bare name fails with "command not found". sudo
  splits the value on whitespace (a backslash escape works) and sudo-rs fails
  either way, so a path containing whitespace or a backslash omits `SUDO_EDITOR`.
  Both run the editor as the user with HOME and XDG variables intact. sudo-rs
  refuses a file below any user-writable path component (sudo checks only the
  parent), so the native fixture uses a root-owned directory under `/etc`, not
  `/tmp`. macOS has `sudo -e` but no `sudoedit` command. zsh picks the vi keymap
  when the editor's name contains "vi"; managed zsh binds vi mode explicitly.
  Local hooks run later and may override. Windows is unchanged.
  `TestNeovimIsTheShellEditor` and the opted-in
  `TestNativeSudoEditorUsesGeneratedNeovim` (hosted sudo-rs and macOS sudo) cover it.
- PowerShell enables PSReadLine vi mode before binding keys. Its PATH setup works
  for noninteractive invocations, but interactive setup must reject batch,
  encoded/stdin, redirected, CI and unsupported-host invocation before creating
  interactive caches. Host name/UserInteractive alone is insufficient.
- tmux uses Ctrl-B, one-based windows/panes, lowercase pane navigation and
  uppercase H/L window swaps. Use the four pinned plugins directly; never invoke
  continuum's auto-start handler that can remove personal startup services.
- The shared POSIX Rose Pine `LS_COLORS` recipe loads when lsd or zsh is selected, before zsh completion. Bash users receive the same palette; an unselected integration leaves their palette alone, and `DOTFILES_LS_COLORS` remains the explicit override.
- LazyGit keeps upstream keybindings on every OS. Windows uses Herdr; POSIX offers
  Herdr and tmux. Do not restore psmux-specific key remaps or WSL host detection.
- Leave Herdr's Windows default_shell unset. Its built-in resolver selects an
  absolute PowerShell 7 path from the launching shell; an explicit bare pwsh.exe
  is searched through portable-pty's registry-derived PATH, losing private tools.
  The active Windows target uses herdr/config.toml; retain the released
  herdr/config.windows.toml bytes for unmigrated live links. Keep the PowerShell
  prerequisite and prove actual pane execution natively.
- Starship git-status styles interpolate `($style)`. Claude Code and Codex keep
  personal credentials, sessions, models and permissions. Sentinel is the
  separately selected agent-policy product; its consumers are Codex, Claude,
  Copilot and Kiro, written only for agents that are present (owner, 2026-10-21).
  Sentinel skills are an independent selection.

## Before changing this repository

Read this guide, the current status/review ledger and
[the field findings](docs/reviews/field-findings.md): real-machine problems that
hosted CI missed, with what to recheck. Add new ones there. Preserve unrelated dirty
worktrees. Keep policy in the pure graph and IO in concrete providers; add an
abstraction only for present duplication or a demonstrated product requirement.
Update the relevant Markdown with every behavior change and append findings to
`docs/reviews/2026-10-08-interactive-installer-overhaul.md` without erasing history.

Run `make ci` (or `./test.ps1` on Windows) for a baseline and after integration.
Neovim spec runners isolate XDG data/state/cache in a private temporary root:
a managed runtime or personal shada/undo must never affect or receive test state.
Focused tests are appropriate within a cohesive batch. A bug fix needs a real
failed-before regression and a passing correction. Portable permission-preservation
tests compare the original native configuration snapshot with the restored one.
Go exposes only the writable/read-only attribute through Windows file modes;
requesting POSIX 0640 does not make 0640 an observable Windows result. Keep the
full content/mode equality assertion on every platform, rather than skipping it.
Filesystem-error fixtures must verify their native error before testing policy:
a regular-file parent reports ENOTDIR on Unix but missing on Windows. Use an
overlong path component for the portable cask inspection-error case.
Hosted Mac consent diagnostics are read-only and run only after a failed native
launch, before process cleanup. Check existing Accessibility trust with prompting
disabled; never grant permissions, modify quarantine/policy, or treat a held app
as a successful desktop launch. Preserve the actual normal-consent evidence.
The opted-in GitHub-hosted Zen fixture may supply ordinary first-open consent
through the image's existing Accessibility grant. Verify enabled Gatekeeper,
native notarization, Zen's declared signer and Apple's exact consent agent first.
Require a clean prelaunch baseline and the observed normal Zen/Homebrew dialog;
press only its enabled Open button once. Bind the held process by executable,
PID and start time, then require matching native LaunchServices completion,
dialog dismissal, macOS's user-approved quarantine flag and the existing window
assertion. Never press Open Anyway, grant a permission, synthesize input, or
fall back to a direct executable launch. This is test automation, not product
configuration; the installed app keeps its ordinary first-open prompt.

Windows quality reads the installer's official Neovim archive pin directly from
`installer/archive-pins.json`, verifies its checksum before extraction into the
disposable runner temp directory, and places that executable first on the job's
PATH. Keep this direct pin consumption in the shared pin-consistency check;
do not duplicate version/digest inputs outside Renovate's reviewed inventory.
The native Chocolatey retirement fixture must still exercise Chocolatey itself
and retain its bounded install output, including exit-zero attempts that leave
no package. Require the fixture package to exist before testing retirement.

Keep the highest practical proof: process-boundary tests, then actual configured application/native lifecycle
on explicitly opted-in disposable runners. Cross-compilation is not native proof.
Do not run native package/font mutation on the everyday developer workstation.

The Darwin released-installation fixture supplies real Ghostty, fontconfig,
yamllint and Python 3.12 through the current Homebrew and the paired historical
tap snapshots before v0.4.4 takes over. Install only absent formulae: an explicit
`brew install` can promote an existing automatic dependency to requested even
when it installs or upgrades nothing. Preserve every pre-existing package's
ownership evidence: source, pin, protection, requested/automatic classification,
every installed keg and runtime dependency; refuse a different source or an
unlisted Cellar entry. Homebrew still upgrades an outdated dependency of a formula
it installs (`HOMEBREW_NO_INSTALL_UPGRADE` covers only the named formulae), so the
only accepted change is an added, linked keg beside the original, logged (owner
decision 2026-10-23, after the 20261021 macOS 26 image moved `openssl@3` from
3.6.5 to 3.5.9 while the July snapshot's python@3.12 brings 3.6.3).
The real native regression must preserve the dependency and outside consumer's
metadata/payload while installing an absent prerequisite. Released migration
separately proves automatic shared fontconfig and its consumers unchanged, or
removal of a requested unshared copy. Neither branch relaxes Ready or recovery.
Read its semantic inventory only after the file-record-verified conversion:
Brew 6.0.1 cannot load newer cask definitions and can omit them from JSON.
Take payload snapshots before conversion; compare semantic inventory using the
same converted Brew on both sides. Apple's Python 3.9 cannot install the
release's pinned setuptools 83, and today's API post-install steps exceed its
Homebrew 6.0.1. Verify the prepared commands, expose Python's documented libexec
PATH, and prepare tmux's dependencies with current Brew's documented
`install --formula --only-dependencies tmux` before the old runtime takes over.
Record personal seeds before the release and complete file baselines after it:
v0.4.4 itself appends its Homebrew block to `.zshrc.local`. Retirement preserves
that block and unrelated fstab/synthetic entries. A full tap-tree audit retains
raw Git indexes; classify a stat-cache refresh only after proving staged paths,
blob identities, modes and flags unchanged, with every other entry exact.
The released installer must still install tmux itself. Current OpenSSL metadata
otherwise creates literal `{{pkgetc}}` certificate links under Brew 6.0.1.
Run the historical installer unchanged on its normal API path. Do not
enable NO_INSTALL_FROM_API for the released setup: the historical Ruby formula
DSL also exceeds that old Brew. Preserve failed attempts; do not promote a
partly failed mixed installation into the healthy migration baseline.

Shell behavior tests must consume `configureShellProfiles` output published by
`ProfileDriver`, never the frozen released `shells/` sources. The test-only
`shell_fixture_test.go` exporter writes to an explicitly supplied fresh temporary
directory; there is no production export mode. It uses process/module boundary
fixtures while PSReadLine and zsh keymap operations execute normally. New Windows
profiles receive the real publisher's BOM. The two generated
`PSAvoidGlobalVars` diagnostics on `$global:DotfilesShellReady` are reviewed
process-idempotence warnings: assert their exact count/rule/extent, rather than
excluding the rule. Other diagnostics fail. Use zsh's syntax checker for its
native glob qualifiers and Bash's syntax checker for every shared `init.sh`:
an inactive zsh branch must still parse in Bash. The fixed zsh compdump glob
condition is delayed through a literal `eval`, with no command text interpolated
from paths or environment. Assert completed interactive Bash initialization,
ready marker, selected aliases and empty initialization stderr; palette output
alone can precede a later parse failure. ShellCheck covers actual Bash/POSIX
scripts. Linux and macOS quality jobs generate and execute active interactive zsh profiles; the
existing opt-in archive lifecycle also exercises pinned zsh plugins on both.
Native shell fixtures keep their actual OS identity: connect discovered APT for
Linux zsh and system PowerShell for the Windows VC++ provider. Archive-only
fixtures require those native prerequisites healthy before applying and prove
reused ownership/unchanged outside state afterward; fresh native-provider lanes
own installation proof. Cross-OS PowerShell behavior uses generated-profile Pester
processes, never a foreign native catalog with host archive pins.

Go uses gofmt tabs. Lua uses StyLua with two spaces; other formatting follows
`.editorconfig`. Keep all text LF through `.gitattributes`, including Windows
scripts and embedded inputs. Never hand-edit generated files: change their source
and regenerate the released inventory, Sentinel policy or dependency inventory.

For a new editor plugin, update its specification and reviewed Lazy lock together.
For an LSP/formatter/parser, update `tests/nvim/language_matrix.lua`, its plugin
configuration and the corresponding strict language smoke fixture. Keep the
built-in parser exclusion above. New hidden native dependencies belong in the
catalog, with platform bindings and native behavior proof, never another UI list.
Archive/bootstrap version and hash changes are reviewed together with native
runtime proof. Renovate coverage is exactly its checked-in extraction inventory.

Preserve CodeQL, private vulnerability reporting and immutable releases in the
reviewed repository policy; distinguish that policy from verified live settings.
Required-check identity and ruleset sources live under `.github/`; use
`docs/security/branch-protection.md` for their staged cutover. No synthetic success
check may replace a removed lifecycle. Preserve the main-integrity no-bypass rule,
owner-only PR review/update rules, native security gates and exact run/head
provenance. A checked-in policy is not proof that GitHub has applied it.
Release publication follows `docs/RELEASING.md` and the manifest; historical tag
proof is immutable. This overhaul uses one PR. On 2026-10-12 the owner authorized
squashing the implementation, smoke fixes and CLI replacement into one commit.
Merge and release publication remain owner actions unless explicitly delegated.

## Engine and delivery invariants

The installer implementation lives under `installer/`. Go source uses
`gofmt` (tabs); its `.editorconfig` override must agree with the formatter.
Raw Go string contents also pass EditorConfig: indent embedded Ruby/C/JSON with
tabs where indentation is semantically immaterial; gofmt does not fix literals.
`installer/**` and its workflow require LF in `.gitattributes`: a Windows
`core.autocrlf=true` checkout otherwise fails gofmt before tests execute. The
Git checkout regression guards Go source, module files and embedded JSON bytes.
The capability manifest is `installer/resources.json`; keep dependency/removal
policy in the pure planner and platform IO in adapters. Do not add a second
dependency list to the interactive frontend. A pin and an `archive` action alone do not connect
a resource: its platform binding must select the archive provider. The ordinary
all-pin routing regression checks every supported target, including Windows
Terminal's unpackaged stable distribution. The verified public entrypoints use
this package; all required final-head native checks and independent review must
pass before delivery is ready. Delivery is one PR containing one meaningful
squashed commit, as authorized by the owner on 2026-10-12.

Native Git/tmux packages do not depend on installer-owned shell configuration;
their selected configuration resources do. Retaining a pre-existing or shared
native executable after deselection must not retain the newly added profile
block. Retained applications still protect genuine runtime prerequisites; do not
solve configuration leakage by disabling the planner's retained-resource closure.

Claude Code and Codex use independent private archive generations. Claude Code
uses the upstream stable native binary from its compressed platform package;
Codex uses the full upstream native platform package,
including bundled native helpers, rather than the desktop application. Keep all
four supported target pins and verify their actual layouts/runtime in native CI.
The vendor npm registry supplies these native archives, consumed directly by Go
without npm/Node or a JavaScript launcher. Claude binary bytes must match the
signed vendor manifest when pins are prepared; verify each package's published
SHA-512 integrity before recording its SHA-256 pin. Codex strips only the top-level
package prefix, preserving package metadata and the complete vendor/target native
helper/resource layout.
The generated shell environment sets `DISABLE_UPDATES=1` only when Claude Code
is selected, before interactive guards, so updates stay with the reviewed checkout.
Do not write personal agent settings, authentication or model/provider preferences.
The native application fixture exercises both CLIs without credentials, the Codex
app-server initialization protocol, unchanged-pin update, pointer repair, selective
removal, shared Git protection and personal-data preservation. API/model calls
remain a separate authenticated verification boundary.

Kiro CLI (owner request 2026-10-22) is an optional capability on macOS and Linux,
pinned like Claude Code/Codex: Kiro's fixed-version stable downloads
(`prod.download.cli.kiro.dev/stable/<version>/`), recorded from its release
manifest and re-verified by SHA-256 before review (no versioned manifest exists).
macOS reads the universal `Kiro CLI.dmg` through the private DMG reader and runs
`kiro-cli`, `kiro-cli-chat` and `kiro-cli-term` from the unmodified bundle's
`Contents/MacOS` (each is separately signed); never launch the desktop app, copy
it to /Applications, re-sign or clear quarantine. Linux uses the glibc ZIP (arm64
needs glibc 2.39), never the vendor `install.sh`; the reviewed exclusions drop only
`bin/q` and `bin/qchat`, legacy aliases hard-wired to `~/.local/bin/kiro-cli`.
Windows is not offered, by owner decision (2026-10-22), as a limitation of how Kiro
distributes it: its only Windows build is an MSI installer. Its normal install is
machine-wide, needs elevation and leaves Kiro's updater in charge (uninstall bug
kirodotdev/Kiro#11398); the owner declined that, and also declined extracting the
program from the MSI (license caution, after three working designs: msiexec
administrative installation, expand.exe and a pure-Go cabinet/LZX decoder; see the
review log and field findings). Revisit only if Kiro publishes a plain Windows
archive such as a ZIP; then pin it like the macOS/Linux builds. Sentinel policy and
skills still reach a Kiro the user installed on Windows. Windows public-command
fixtures pass KIRO_HOME (like CODEX_HOME): the installer resolves home through known
folders, not the fixture's HOME. Restricted-token Windows test children run with an
explicit `-test.timeout` and file output: a test binary run directly has no default
timeout, and a lingering descendant would hold an output pipe open. Kiro CLI
replaces itself in the background; its documented switch on macOS and Linux is
`app.disableAutoupdates` in `${KIRO_HOME:-~/.kiro}/settings/cli.json`, so
`integration.kiro` owns exactly that strict-JSON field through ProfileDriver, as
VS Code owns `update.mode`; every other Kiro setting is personal. A saved settings
target under a different KIRO_HOME is refused, never retargeted.
Resolve KIRO_HOME like Kiro: only an absolute path other than a filesystem root,
the home directory or an ancestor of it; otherwise `~/.kiro`. Kiro documents it for
its V3 interface, so a Classic-mode CLI may still read `~/.kiro`.

GitHub Copilot CLI (owner request 2026-10-22) is an optional capability on all four
targets, pinned to the standalone single-executable assets of the official
github/copilot-cli stable release (verified against its SHA256SUMS.txt): tar.gz
for macOS/Linux (Linux needs observed glibc), zip for Windows. Never use npm, the
install script, the MSI, winget or Homebrew. Windows adds the shared PowerShell 7
prerequisite (Copilot requires PowerShell 6+). Copilot ignores its `autoUpdate`
setting (github/copilot-cli#2615, #4534); only `COPILOT_AUTO_UPDATE=false` stops
it replacing or re-executing over the pinned binary, so the generated shell
environment sets it when tool.copilot is selected, beside Claude's
`DISABLE_UPDATES=1`. Never write `~/.copilot` settings. A copilot started without
that environment can still self-update in place (#4728): Check then reports the
changed payload and Update/Repair restores the pin; never weaken the payload
fingerprint to accept it. Discover COPILOT_HOME at the process boundary; like
CODEX_HOME it must be an absolute canonical path.

Agent CLI pins stay pinned with weekly bump pull requests (owner decision
2026-10-22). `.github/workflows/agent-cli-pins.yml` runs
`scripts/update-agent-cli-pins.py` on a schedule and on dispatch; it changes only
version, URL and SHA-256 of tool.claude, tool.codex, tool.copilot and tool.kiro,
all targets of a tool together, after the documented byte checks: Claude's stable
channel manifest verified with gpg against fingerprint
31DDDE24DDFAB679F42D7BD2BAA929FF1A7ECACE plus npm SHA-512 integrity and binary
checksums; Codex npm integrity; Copilot release digest plus SHA256SUMS.txt; Kiro's
release manifest size/SHA-256 and Linux ZIP layout. Prereleases and layout changes fail that tool for manual review while the
others proceed. GITHUB_TOKEN cannot open pull requests here
(`can_approve_pull_request_reviews: false`) and its pushes would not start the
required checks, so proposals use the owner-configured GitHub App variable
`AGENT_PINS_APP_CLIENT_ID` and secret `AGENT_PINS_APP_PRIVATE_KEY`; without them a
run only verifies, reports and uploads the patch. The bot never merges: the PR's
ordinary required native checks are the acceptance boundary and merge stays an
owner action. Every other archive pin remains manually reviewed (Renovate does
not track `installer/archive-pins.json`). Update applies the checkout's pins, so
it changes nothing until such a PR is merged and pulled.

Mac keyboard shortcuts (owner request 2026-10-21) are Linux-only and optional:
the shared Mac-layout keyboard's Cmd key reaches Linux as Alt and Option as Super.
Pinned static xremap (`tool.xremap`, the upstream "full" variant) runs from an XDG
autostart entry that passes `--desktop` from `XDG_SESSION_TYPE`: xremap's automatic
choice probes X display `:0`, which is not every session's display. The
`xremap/config.yml` live link is product policy: Cmd becomes Right Ctrl in every
identified window (native Ctrl shortcuts and Cmd+click), Option becomes Alt, and
physical Ctrl is unchanged. Rules apply only to windows xremap identifies (`/./`),
because an unidentified application matches every `not:` filter; GNOME Wayland
without xremap's Shell extension therefore stays plain. In terminals every Cmd
combination is mapped to the terminal's shortcut or swallowed: Cmd must never send
a control character. In browsers Cmd+Left/Right stays line start/end and back/
forward is Cmd+[ / Cmd+] (owner decision 2026-10-22): macOS gives Cmd+Left both
meanings by focus (page vs text field), which the remapper cannot see, and back/
forward would also fire while typing in the address bar or a form. Cmd+Tab holds
Alt through a `switching` mode that ends on
Cmd release. The autostart entry resolves the live link (`readlink -f`) before
exec: xremap's `--watch=config` follows edits of the file and replacements in the
file's own directory, and git updates replace the checkout file (unlink, create)
while the link's directory never changes. `config_watch_debounce_ms: 500` keeps
a reload from reading a half-written file. Owner-reported 2026-10-22: after a
pull, Option+2 still reached Zen as Alt+2 (its tab key) until re-login.
`TestNativeMacKeyboardLifecycle` drives a uinput keyboard as root
under Xvfb against the real binary, autostart entry and config, including a sweep
of every terminal Cmd combination; keep it when changing the keymap.
By owner choice the only privilege is `system.keyboard-access`: one udev rule,
`/etc/udev/rules.d/70-dotfiles-mac-keyboard.rules`, tagging uinput and keyboards
`uaccess` (before 73-seat-late) so logind grants the active seat user, never an
SSH session or a permanent `input` group. It is the only root-owned file setup
writes outside a package: the fixed recipe installs a temporary copy, hard-links it
into place (never replacing an existing file), names its operation in the header,
reloads udev and re-triggers misc/uinput and input. Removal requires the exact
installed fingerprint before moving the rule aside and before deleting it. Any
other file there is reported and never adopted, repaired or removed. The resource
has user scope with a privileged observation: catalog `machine` scope means a
possibly shared resource kept on deselection, while this grant serves only the
capability and must go with it (the hosted lifecycle first exposed the default).

Navigation keys shared by Mac and Linux (owner request 2026-10-22). AeroSpace
stays as configured ("vanilla" by owner choice): tiles with gaps, no app pinned
to a workspace and no layout policy; README teaches beginner use, the stacked
accordion layout and how to pin an app with `on-window-detected` (Ghostty
`com.mitchellh.ghostty`, from the pinned macOS archive's Info.plist; Zen
`app.zen-browser.zen`, its signing identity). Earlier drafts that changed the
layout or pinned Ghostty/Zen were rejected; do not reintroduce them. On Linux the
xremap "Navigation" entry mirrors AeroSpace's keys in identified windows only:
Option+1...9 → Super+1...9 and Option+Shift+1...9 → Super+Shift+1...9 (GNOME
switch-to-workspace-N / move-to-workspace-N once the user runs README's one-time
gsettings commands; Ubuntu Dock app hotkeys otherwise), Ctrl+Option+L/H →
Alt+Esc / Shift+Alt+Esc (GNOME cycle-windows defaults, like AeroSpace focus
right/left), Option+Tab unmapped (Alt+Tab, a quick tap returns to the previous
app). Setup never writes those gsettings, `favorite-apps` or extensions: they are
personal GNOME state, and README gives set and reset commands. After the modmap
physical Option is `Alt_L`, Cmd `Ctrl_R` and Ctrl `Ctrl_L`; the entry uses
`exact_match` and precedes the terminal rules. xremap tries exact matches first
in config order and skips `exact_match` entries in its inexact pass, so
Cmd+digit keeps its meaning (Alt+digit for Ghostty tabs in terminals,
Ctrl+digit in apps) and Cmd+Option+digit is not taken over. Its key sender
releases held modifiers the output does not name, so no Alt or Ctrl reaches
GNOME. `TestNativeMacKeyboardLifecycle` covers these keys.

Pi is retired from the active catalog, pins, configuration, theme publisher,
Sentinel destinations and native/terminal tests. Its copied theme/keybinding
sources and their now-unused license are removed. Immutable release inventories
and dated ledgers remain historical evidence, never a Pi installation/removal route.
Before advancing an unreleased native-installer checkout with Pi or the former
four-target or d9a3993 three-target Sentinel resource selected, remove those selections with the original
revision and finish any recovery there. Do not delete journals or invent an
ownership migration for the unreleased catalog.

The Nix prototype and old monolithic installation/runtime paths are removed.
The [test retirement map](docs/installer-test-retirement.md) identifies replacement
proof and deliberately removed behavior. Prototype batch JSON was never released;
reject and preserve it instead of inventing a runtime migration for it.

The [interactive installer plan](docs/plans/interactive-installer-overhaul.md),
[status inventory](docs/installer-status.md) and append-only
[review ledger](docs/reviews/2026-10-08-interactive-installer-overhaul.md) record
current decisions, exact evidence and unresolved findings. PR #87 delivers the
implementation, smoke fixes and CLI replacement in one squashed commit; PR #86
is superseded. No component checkpoint is
a completed release. Keep these records and ROADMAP.md synchronized with changes.

The contributor gate requires the Go version in installer/go.mod and native race
tests. Linux process-inspection regression tests also require `cc` with pthreads
to prove that an exited main thread does not hide a live worker thread. Core fixtures do not prove package installation. Hosted native lifecycle,
Go CodeQL and the trusted-checkout bootstrap remain separate verification claims.

First run opens categorized capability checkboxes. Categories are display metadata
in resources.json, never graph resources or persisted selection IDs. Filter tools
by platform and operation before grouping; mixed parents select all remaining
children, full parents clear all, and leaves stay independent. Keep adoption,
retention and shared-removal decisions flat and explicit. Shared-removal choices come from the graph;
unchecked shared resources become Keep roots. Every mutation requires a complete
preview and an explicit confirmation. By owner decision on 2026-10-20 the
confirmation is one compact screen whose initial choice is Apply: counts, the
removals, replacements and administrator changes that fit, then the retention,
adoption and shared-removal decisions as checkboxes. `d` opens the complete
reviewed plan, which must stay reachable and unclipped. Toggling a decision
recomputes the proposal in the original dependent order (retained resources,
then adoption from that plan, then shared removal from the adopted plan), with
cooked input so Ctrl-C interrupts. Discard keys typed before the confirmation
appears and during each revision, so type-ahead cannot approve; EOF never
consents. Esc/q goes back to the main menu, keeping the draft tool selection,
and Ctrl-C/Ctrl-D quit. The interactive menu is Choose tools, Update tools, Check
and Exit (owner request 2026-10-20): removal is unticking, and an unchanged
selection or Update repairs. Machine request modes are unchanged. The cursor
starts on Update when the installer source changed since the last apply or a
receipt needs action. A completed Check returns to the menu. One alternate-screen
session spans the menus; raw input exists only while a screen reads keys.
Release the screen through Interaction.BeginApply before provider prompts or
elevation. Preview holds existing engine/provider guards without creating
state. Machine apply requires a non-nil selected array; [] means deliberate
removal. Maintenance and retry preserve recorded choices and mode.

Persist each Receipt.OperationID before mutation. Mere presence, matching bytes
or before/after inventory differences do not grant ownership. Driver completion
must bind the saved operation to the exact artifact. Retry may recover that proof;
abandonment preserves uncertain receipts. Keep the first baseline through updates
and restore it on removal; absence is success only for an absent original.
Completed removal tombstones survive repeated finalization failures. Seal provider
completion before clearing core intent, even when retry has only keep/forget work.
A pending observation never proves absence.

Provider processes that can outlive the controller require a real mutation guard;
PID disappearance is not quiescence. Acquire it after the engine lock and before
observation/journal changes. Whole-provider inventory, when required, covers
unselected members and protected inputs. Bind inventory even for an empty plan,
recheck it before each mutation and at finalization, and preserve unknown members.
Native adapters must still serialize and validate their own publication boundary.
Absent unselected inventory members do not pull their unused prerequisites into
observation. Retained consumers keep their dependencies.

Final verification covers retained/unselected identities and completed operation
results, not only selected health. A healthy-but-different artifact fails
preservation. Keep the original transaction and explain the affected resource.
Read-only check reports unfinished intent even if every selected tool is healthy.

Explicit resume uses the original authenticated source and saved provider payload;
it must not rebuild old intent using current pins. The launcher must retain that
engine for recovery. Source-independent restoration of saved physical artifacts
is a separate explicit action. Configuration restoration uses physical no-overwrite
inverse moves. Profile restoration reverses only owned blocks when personal bytes
changed after publication; the inverse uses the same durable profile writer. An
interrupted inverse must finish restoration, and a forward retry must preserve
its original transaction. Check recovery-path inspectability before choosing that
direction. Rebase an unmoved inverse against fresh personal edits; retain a file
recreated after the move separately before publishing the saved inverse. A staged
file still waiting to publish is not evidence of an edited published profile:
return the original and disclose the recreated file. Unparseable personal bytes
are preserved with a needs-action result instead of stranding the transaction.
Recovery menus offer the committed direction until restoration is archived,
including after provider completion. The optional observation `Restoring` flag
is absent/false in older state and is rediscovered from the provider journal.
Uninspectable recovery files keep restoration retryable. Route recovery by the receipt's provider identity, not
the current catalog. Historical corruption is preserved and disclosed without blocking unrelated work;
current operation receipts distinguish evidence required for finalization. Drop full profile content from the
journal once staging is durable; that file is then the recovery payload. An
uncommitted staging file needs a fresh metadata-preserving copy even when its bytes
match: a killed writer may not have restored its permissions. Preserve torn staging
without parsing its encoding. POSIX copies must verify uid/gid before publication;
a successful cp exit does not prove ownership preservation. A refused copy is
removed before returning so retries do not accumulate personal-profile copies.
Windows CopyFileW
does not preserve a file DACL: copy its owner/group/DACL explicitly with native
security APIs and verify the descriptor before publication. If the copied descriptor
already matches exactly, leave it unchanged rather than rewriting inherited ACL
control flags. Preserve inherited
versus protected ACL semantics; do not change the user profile to make staging
writable. Only a private staged
copy may become temporarily writable, and its original mode must return before
publication. A restoration-preview error must not hide the other recovery choices. Damaged
active profile evidence must disclose the journal, baseline and trustworthy current
workspace locations, without following paths decoded from damaged JSON. Unrelated
archive records and deferred cleanup failures must be preserved and disclosed
without blocking independent installation or abandonment. Check a directory entry
before treating a ReadDir error as absence: Windows can report not-found for a
regular file. Finalization must also require just-completed journals that are
missing from enumeration; missing proof cannot seal a completed transaction.

Windows state replacement uses Go os.Root.Rename with delete-sharing readers.
The native regression verifies both complete old and new snapshots; MoveFileEx
is not an equivalent substitute. Git checkout fixtures strip inherited GIT_*
variables so hooks cannot redirect writes into a caller index. All workflow YAML
and installer source use LF even under core.autocrlf=true.

Terminal tests use actual PTY/ConPTY sessions and exercise ordinary input after
closing the selector. On Darwin compare restored state after the next read clears
transient PENDIN. Never leave a background stdin reader across elevation prompts.
The pinned x/term dependency and its Renovate inventory belong to the gate.

One interactive session inspects each resolved selection once. The inspection
key is the closure of the effective Selected and Keep roots, because it binds
generated configuration and native keep sets; reuse is bounded to five minutes.
Inspection of the recorded selection starts in the background when the menu
opens. Previews use exclusive non-blocking locks, so the session joins or
cancels that inspection before any other preview and before the apply. A reused
inspection never authorizes change: Execute re-observes, and ErrPlanChanged
(returned before any journal write) brings back the review with a notice. Never
apply the stale plan or accept the fresh one automatically. Tool status labels
come from the apply and update plans of the unchanged recorded selection.

Approved work that needs foreground sudo authenticates once through
ElevationDriver after Execute reproduces the plan and before any mutation. Only
providers that already authenticate in the foreground qualify (non-root APT, the
Homebrew bootstrap, Apple CLT and privileged retirement steps); machine scope
alone, such as a Homebrew formula, never prompts. KeepSudoAlive then refreshes
with `sudo -n -v` each minute until release; refresh never prompts, a stopped
refresh is reported, and per-command authentication remains the fallback.
Interactive POSIX entrypoints configure it; machine requests and Windows do not.

Each Execute writes `logs/apply-<UTC>-<n>.log` beside the state with its steps and
native output (private, newest 20 kept, only matching names removed individually).
A log is a record, never evidence; its failures become "Apply log ..." notes
rather than failing the apply, and `Result.Log` is an optional field. An apply
failure is reported once, in the result report, and returned as ReportedError so
entrypoints exit unsuccessfully without repeating it.

Approved work displays its progress on the ordinary screen (owner request,
2026-10-20): numbered executor steps, download/verification lines, provider
phases, the native command line and live native output. Progress travels in the
context and is display only: it never fails, never enters receipts or journals,
and previews/machine requests carry none. The worker forwards child output as
`stream` messages before the single final reply; saved replies with a stream are
invalid. Captured evidence keeps its 1 MiB bound. The worker handles SIGPIPE so a
vanished controller cannot kill it mid-command; a failed stream write only stops
streaming, and the final reply reports the loss after the child exits. Handled
signals reset in executed children; never use signal.Ignore, which children
inherit. The terminal strips whole escape sequences and controls from native
output, honors carriage-return rewrites and returns display failures with the
result report, never to a provider pipe. Windows keeps virtual-terminal output
enabled only while that display is open; the plain-text report and Workflow.Close
end it even when a failed phase is never reported. The final report separates
actionable reasons from disclosures such as unexercised applications.
The live display hides Windows PowerShell CLIXML stream markup (the `#< CLIXML`
marker and its `<Objs>` document, even when the 4096-byte wrap splits it); the
plain lines around it still show. This is display only: captured evidence, the
apply log and failure tails keep every byte, and protocol parsers still refuse
CLIXML. Fixed retirement scripts are not changed to silence it, because their
bytes are saved command identities.

## Native shell and package boundaries

`integration.shells` is the only native owner of personal shell attachments.
The combined private initialization and embedded shell sources in `installer/shells/`
replace whole-file config.zsh/config.powershell targets. The request adapter derives
its assembly from the resolved graph without mutating shared provider state. Tool prerequisites do
not select optional shell hooks. Keep core, Starship, fzf, lsd, zoxide and local
customizations in their tested order; PowerShell PATH setup precedes its strict
interactive guard. Actual known folders and ZDOTDIR remain explicit inputs.
Bash login ownership follows the existing schema-1 profile baseline after initial
selection. A new higher-priority personal profile cannot relocate the block or
strand removal. Bind only the three canonical home basenames, then validate the
complete baseline target list, Before identity/fingerprint and blocks against the
receipt. Pass already-loaded state through the selection adapter; do not reread
state or infer ownership from a matching path. Preserve new personal profiles and
keep redirect/malformed-baseline rejection. Bash may shadow the old file under
normal precedence; this is not permission to adopt the new one.
A restricted Windows fixture must grant its user access to the disposable test
executable and temporary directory before removing administrative access; Go's
administrator-owned build directory is not an ordinary-user test host.

Native filesystem composition tests use the host OS and real native paths; a
Windows drive path cannot model a POSIX PATH entry. The hosted OS matrix supplies
cross-platform coverage. Linux shell tests require zsh explicitly provisioned in
the runner. PATH-isolation probes resolve zsh before restricting PATH and retain
that executable for every child, including nested login shells; a system-directory
installation is not required. Provisioned test prerequisites are not
installer-bootstrap evidence.
Restricted-token child creation requires `TOKEN_ASSIGN_PRIMARY` on the original
handle as well as QUERY and DUPLICATE: CreateRestrictedToken retains handle rights,
and CreateProcessAsUserW checks them separately from the child's filesystem ACLs.

Linux target discovery reads bounded `/etc/os-release` data (falling back only
when absent to `/usr/lib/os-release`), accepts exactly ID=ubuntu or ID=debian,
and inspects `/bin/sh`'s ELF interpreter for libc. It never executes
os-release. WSL remains OS=linux; no kernel-brand detection is needed. Archive
pins declaring a libc require an exact observed match; absent observations cannot
select glibc binaries. Constructors retain explicit target fixtures; native entry
points and archive execution tests use `DiscoverPlatform`. Distro/libc are
provider observations, never persisted target identity; Context remains OS/arch.

A worker `nativeCommandError` message already ends with its output tail
(`nativeReply.failure`); wrappers must not append `nativeDiagnostic` again. The
APT refresh is plain `apt-get update` (owner decision 2026-10-24): a source that
fails only with a warning, such as an expired third-party key with earlier verified
lists, keeps those lists and no longer blocks every install; errors still fail and
the report names the fix. Saved refresh commands with the earlier
`-o APT::Update::Error-Mode=any` recipe stay valid and re-run unchanged; no other
saved refresh arguments are accepted.
Native command output bounds must use composition, not an embedded bytes.Buffer:
its promoted ReadFrom lets io.Copy bypass a Write limiter. Drain excessive native
package output before reporting failure so truncation cannot terminate a mutating
child. Keep the worker's OS lock until the command actually exits after controller
death; a PID check or killing only its supervisor does not prove quiescence.
Archive case-collision checks follow the actual destination filesystem, never an
OS guess. Python's Linux terminfo legitimately includes case-distinct filenames.
Persist native command intent before execution and its result before responding;
an unfinished record cannot authorize a second execution. Preserve structured
vendor exit codes. A Homebrew install badge alone does not prove activation:
failed linking also prints it, so verify command success and native activation.
APT may name Architecture: all packages with the host architecture in history;
reconcile only against matching dpkg install/configured evidence for name:all.
An operation-local dpkg `install` action proves absent payload even when its old
version is not `<none>`: normal removal retains conffiles and the previous
version. dpkg emits `install` for both `not-installed` and `config-files`, and
`upgrade` for other prior states ([dpkg source](https://github.com/guillemj/dpkg/blob/1.22.21/src/main/unpack.c#L1335)).
Do not purge retained configuration or require an empty old version to establish
ownership. Still require matching APT planning, unique install evidence, successful
configuration and current identity; failed attempts authorize only saved recovery.

Native incidental packages belong to one provider pool, not the resource that
first introduced them. Narrow removal against current installed consumers,
selected roots, manual promotions and pins; use the native manager's exact
dependency-checked removal as the final guard. Include Homebrew cask consumers.
APT inventory includes dpkg `Essential` and `Protected`: keep those packages
and their dependencies in the disclosed owned pool, even if this operation
introduced them. Never force their removal. Legacy saved inventories omit the
optional protection flag; fresh native observation must still govern dispatch.
`brew deps --direct` changes to declared dependencies: omit it when recovering
actual runtime metadata. A native package's version can change through ordinary
system updates without changing ownership; source/pin changes must be preserved.
Homebrew recovery commands are restricted to the saved recipe and exact removal
set, expected executable and reviewed process controls. Never persist inherited
environment/credentials or execute arbitrary commands decoded from saved state.
Keep native workers outside the controller's terminal process group (POSIX) or
console (Windows), including native console children. A Ctrl-C event must not
release the provider lock while a surviving child still mutates packages.
Authenticate elevation in the foreground controller before starting noninteractive
privileged work; background workers must never read terminal passwords.
Native controller preview holds both existing locks, without creating state.
Mutation acquires the provider guard before loading or changing core intent.
Hand the provider lock to a worker lazily, while retaining the engine lock;
archive-only requests need no worker. Homebrew approval covers owned roots,
the incidental pool and their consumers, not unrelated native package versions.
Refresh approval only after command-attributed changes preserve other owned
packages and their source/pin/manual classification. Selection clones share the
mutation approval session but derive independent Keep sets; previews never
write that session. Native controller fixtures must provide their StatePath.
Native update proof must execute compiled shared-library consumers: data-only
formulae and linked-keg metadata do not establish ABI health. Never count a warning
about a broken retained consumer as successful completion. A required repair that
failed in an earlier native attempt cannot be cleared by a later root-only no-op.
Homebrew intent schemas 3 and 4 save the pre-existing native inventory, including
source/pin/manual classification and runtime consumer edges. Unreleased schemas
1 and 2 cannot prove that complete baseline and are refused,
never guessed or silently upgraded. Successful maintenance may change versions,
but cannot lose pre-existing packages outside exact removal or change maintained
packages' classifications. Validate this before publishing ownership. Native
errors retain structured exit codes and a bounded, terminal-control-free output tail.
Homebrew CLI catalog entries supply an absolute native health command. Linked-keg
metadata is necessary but insufficient. A failed command produces an unhealthy
observation with its diagnostic and cannot publish completed ownership. The optional
HealthIssue observation field defaults empty for previous saved observations.
Homebrew dependent maintenance must remain enabled. Explicitly clear inherited
HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK in the child; merely omitting it inherits
an unsafe caller preference. Keep automatic cleanup/autoremove disabled separately.
Homebrew's automatic dependent repair can miss consumers of an upgraded dependency.
After a successful root command, check linkage for the command-attributed packages
and their recorded native consumers. Rebuild only failing formulae and recheck the
whole affected set. A successful root-only retry does not skip this verification.
Save each maintenance command before execution; replay a completed result without
repeating mutation, and require explicit recovery to retry a failed command. Native
runtime edges bound recovery scope; never invent version constraints or acquire
pre-existing packages. One failed repair of a consumer stops that attempt instead
of looping. An affected external cask must retain its installed version/source;
its previous and current declared formula dependencies must remain present and
active. Affected formulae still require linkage checks and bounded repair. Report
external applications as not exercised, without making that disclosure removal
authority or marking a verified selected root unhealthy. Managed applications
require their own runtime health checks; formula linkage cannot supply them.
Never pass casks or an empty formula set to `brew linkage`. Save the disclosure
before completing the intent and show it in approval, results and later checks.
Linkage commands set a child-only
HOMEBREW_DEV_CMD_RUN override to avoid persisting a developer preference. The native
fixture verifies that preference remains unchanged. Its eight-minute per-contract
limit includes real linker inspection and source rebuilds; it is not a product SLA.
Homebrew evidence compares normalized slash paths against its exact configured
Cellar; absolute-path validation accepts native drive paths in process-boundary
tests as well as captured POSIX paths. Homebrew remains macOS-only in the product.


Windows Build Tools uses a checksum-pinned official bootstrapper with verified
Microsoft Authenticode. The vendor resolves its signed servicing packages under
the released vendor-installer exception; do not infer immutable component bytes
from the bootstrapper hash. Reuse existing registered instances unowned. Only the
exact created dedicated instance, matching recorded package inventory and tool
hashes, can be repaired/updated/removed. Global installer and vendor-retained
shared SDK/runtime components are disclosed. Native health compiles and executes
a C++ program using Windows.h and std::string. Resolve only the approved SDK
variables after installation; preserve the already selected managed command PATH.

Native Linux application-construction fixtures belong to Linux build-tagged test
files. Their POSIX shell PATH cannot be represented by a Windows drive path. Keep
catalog graph tests cross-platform and run actual constructor/lifecycle tests on
the native target; moving a fixture must retain its assertions in that target suite.


macOS clipboard commands and Linux APT clipboard helpers use the system PATH;
only the private Windows clipboard archive requires `integration.shells`. Reusing
or retaining native clipboard infrastructure must not retain removed personal
shell attachments or generated scripts. Keep the planner’s retained dependency
closure and ProfileDriver’s exact original-baseline restoration unchanged.

Linux clipboard dependencies are fixed: xclip plus wl-clipboard. Never change the
installation graph merely because setup ran over SSH or before a desktop session.
Observe session capabilities only for honest runtime-verification disclosures.
Neovim warnings require both Wayland commands and an active Wayland display;
X11 helpers require DISPLAY, and tmux transport requires a running session.
tmux copy bindings test session availability in increasing preference order so
an installed Wayland helper cannot capture headless/X11 copies. Preserve existing
bridges without a WSL detector or Windows-host installer.


## Scoped settings and migration

Git defaults use `integration.git`, a marked include prepended to the regular
XDG Git file. Preserve personal overrides, UTF-8 BOMs and line endings; new Git
files never receive the PowerShell BOM. Git's own `.lock` file blocks publication,
and finalization must never remove a foreign lock. Defaults include pull.rebase
and fetch.prune; no identity/authentication values. The include is live like other
checkout links, so source edits apply immediately; its desired identity binds the
include path, not an approval of already-active source bytes. Verify source health
but allow removal with a missing source. Git process folder discovery is independent
of Windows known folders; GIT_CONFIG_GLOBAL blocks only this integration.

Keep the old `config.git` resource for restoration of prior receipts. A new include
must not compete with owned whole-file configuration. Explicit old removal restores
its first baseline; the catalog's `replaced_by` releases only reused, unowned
bookkeeping once the scoped successor is selected/recorded. It cannot release
created or uncertain ownership, delete backups, or authorize file mutation.
Preserve and bind the new include's saved target across HOME/XDG changes so removal
remains possible. Migration never detaches a Git link still owned by setup.

VS Code's `integration.vscode-managed` owns six top-level theme/font fields plus
`update.mode` in UTF-8 JSONC. The recovery-only `integration.vscode` keeps its
historical six fields unchanged. Mask comments
and trailing commas only for parsing; retain original offsets and unrelated bytes.
Malformed/duplicate properties fail before mutation. A comment added to a newly
created settings document prevents deleting that document on removal. Reuse the
existing ProfileDriver receipts and publication for scoped settings.

Sentinel policy is generated from pinned upstream source into
`installer/sentinel/policy.md`; never hand-edit it. `provenance.json` records the
source and renderer inputs; `TestSentinelPinnedUpstreamRendererMatchesBundledPolicy`
is its opt-in reproduction check. There is no runtime Git/Bash installer.
Discover CODEX_HOME, XDG_CONFIG_HOME, CLAUDE_CONFIG_DIR, XDG_STATE_HOME, KIRO_HOME,
COPILOT_HOME and the agents' commands only at the process boundary. A fresh
selection targets only present agents: the configuration root exists, `codex`/
`claude`/`copilot`/`kiro`/`kiro-cli` (or a Kiro macOS app, the IDE or
`Kiro CLI.app`) was found, or the bound closure installs tool.codex/tool.claude/
tool.copilot/tool.kiro. Copilot's root is `${COPILOT_HOME:-~/.copilot}`. Never create an absent agent's root (owner, 2026-10-21).
No present agent is a resource-local pending state. Saved ownership keeps its
baseline's targets through ForSelection, like Git's saved path, so update and
removal still reach every written file. A later agent is a note; reselection adds
it. A covered agent whose root vanished becomes a retired target (empty Script):
healthy without a block, never written, its folder never recreated. The changed
owned policy then needs the existing explicit replacement (Adopt) to record the
narrowing; until then deselection retains it, as for any changed resource. When
every saved agent is gone, apply is blocked and deselection forgets the absent
policy. An agent whose root cannot be inspected is never retired. A saved path
outside today's consumer locations stays a refusal. XDG_CONFIG_HOME now only
locates the released OpenCode policy for migration retirement. Preserve surrounding
prose and prior adopted blocks. Sentinel journal fragments can lack a UTF-16 BOM;
only recognized marker-prefixed fragments get that bounded decoding exception.
After completed removal releases Sentinel ownership, a new consumer inventory
must acquire a fresh baseline and explicit adoption for existing policy blocks.
Preserved paths remain disclosed; the earlier journals and copies remain intact.
Created or uncertain old ownership still blocks retargeting.

Sentinel's 12 skills are vendored byte-for-byte under `vendor/sentinel` with
licenses and provenance, exempt from Git text normalization. The exact upstream
`skills/impeccable/agents/openai.yaml` lacks final LF: YAML lint exempts only that
file from only the EOF-newline style rule, while retaining syntax and every other
rule. Whole-file provenance checks forbid silently reformatting it. Regenerate them
from the exact provenance archive, never edit upstream files. Copy each complete
bundle through ConfigDriver, one resource per consumer root; do not invoke the
upstream installer or write its ownership registry. Kiro's root
(`${KIRO_HOME:-~/.kiro}/skills`, read by the Kiro IDE and CLI) follows the policy's
present-agent rule, with Kiro CLI in the bound closure counting as present. Selected
without ownership while Kiro is absent, `skills.sentinel-kiro` observes as satisfied
(reused bookkeeping, nothing written) so machines without Kiro never need action;
deselection observes the real folders and forgets it. Owned Kiro copies keep their
real targets; if Kiro's root vanishes, apply is blocked with an explanation, the
root is never recreated, and deselection forgets the absent copies. Copilot reads
`~/.agents/skills` unless COPILOT_HOME is set, when it reads only
`$COPILOT_HOME/skills` (upstream changelog 1.0.101); `skills.sentinel-copilot`
is therefore needed only with COPILOT_HOME and a present Copilot, under the same
rules, and never duplicates `~/.agents` into `~/.copilot/skills`. Owned copies
without their COPILOT_HOME are refused, never retargeted. The one Impeccable
engine note comes from the Codex/Copilot root: every copy shares the engine cache. Refuse existing unowned
folders (even identical copies), consumer symlinks/junctions and upstream
registry/lock/stage evidence. A schema-1 registry with empty roots and skills,
which upstream's complete --remove leaves behind, owns nothing: allow it and
never edit it. A `npx skills` lock entry for a bundled name, in
`~/.agents/.skill-lock.json` or `$XDG_STATE_HOME/skills/.skill-lock.json`, is
another owner of both roots: its removal command deletes every agent's folder
of that name without an ownership check. Refuse it, and an unreadable lock, for a
new selection; for copies dotfiles already owns it is a note, so dotfiles can
still update or remove them. Never edit the lock. The ordinary configuration journal governs owned
copy edits, recovery and removal. Preserved recovery metadata after removal does
not grant ownership of a later outside folder; only created ownership or an
in-progress publication may bypass the initial collision guard. Never offer
adoption without created ownership. Stop redirect checks before the observed
home itself, leaving system home resolution to the existing destination binding.
Node is a private dependency; Impeccable engine
setup is explicit and its shared cache is never owned. Any unreleased bundle
folder-count change requires removal with the original revision first.

Managed-runtime Neovim tests disable automatic Git maintenance only on fixture
commits: detached maintenance can outlive commit and mutate the snapshot without
Neovim running. Assert the fixture spawns no maintenance; never exclude those
files from the runtime immutability comparison.
Ghostty's native config-validation CLI can reopen and truncate an inherited
regular-file stdout on macOS. Keep its test wrapper's pipe capture and original
exit status; the native redirected-log regression must preserve earlier output.

Migration uses the existing controller and ConfigDriver file-copy publication.
A read-only preview never creates dynamic source payloads. Whole-profile detachment
requires per-item adoption, preserves the original artifact and readable bytes,
and never copies unknown personal shell commands into fresh profiles. Completed
migration is historical evidence, not permanent ownership; it cannot replay over
new setup blocks. Preserve passive released link targets until users migrate.

Runtime retirement identifies the earlier provider and approves exact remaining
steps. The scrollable final review includes the revalidated full scope for both
new operations and resource recovery; clipped selection details are insufficient.
Present retirement as removal (Homebrew conversion as change) while preserving
the engine's completion-record actions. Privilege comes from verified remaining
steps, not a catalog's possible machine scope. Include vendor-managed UAC from
existing Chocolatey/winget package gates and fixed argv, plus historical Homebrew
installer authentication; Step.Privileged alone means POSIX sudo dispatch. Keep
receipt/catalog scope and saved intent digests intact.
`retire.sentinel-policy` discards a released OpenCode/Pi policy file only when
its whole content hashes to the pinned Sentinel 0.1.2 render (bytes in testdata;
the opt-in `DOTFILES_TEST_SENTINEL_SOURCE=1` test reproduces them from ecafffa).
Any other content keeps the whole file and is reported as a kept detail, never
as unfinished retirement work; never edit it in place.
Retained artifacts never gain removal authority. By owner decision (2026-10-21)
interactive migrate offers, only when every other runtime retirement is finished
or keepable, to keep the remaining resources. The record in
`migration/retirement/kept.json` binds each resource to the digest of the scope
shown; a changed scope is unfinished again, and kept resources stay selectable
for later retirement. Interrupted, unreadable, uninspectable and blocked work is
never keepable. Machine requests cannot record this choice.
Windows retirement process gates follow Scoop's own pre-uninstall test: with the
installation's directory known (Scoop), only a process running from it blocks.
Process names stand in for a location only when none is known (winget and
Chocolatey gates name no directory) or when a process image cannot be read. A
same-named program elsewhere is not that installation: Adobe's bundled node.exe
blocked Scoop Node.js retirement on 2026-10-22, and setup's own pwsh would block
Scoop's PowerShell 7. Saved gate subjects keep their format; only evaluation of
directory-bound gates changed, so an interrupted step resumes without a new plan.
Interactive migrate prints each phase report once: its stop message names the
phase and points to an ordinary-screen report, repeating only a check report.
It never runs the old installer or package-manager autoremove. Hold normal
setup's lock and bind its saved state throughout retirement. Replacement approval
includes the complete native inventory; final artifact preservation excludes that
global inventory, which approved old-package removal legitimately changes. Keep
all replacement identities and health checks strict.
Windows User PATH retirement binds exact matching entry strings/count, allowing separate approved resources to retire their own entries
in sequence. Preserve all unrelated raw entries, empty fields, order and type.
Preserve the current supported registry type, including changes by another
approved uninstaller. Recheck the full freshly read value immediately before
writing, using ordinal string equality. Historical
`user-path` gates and saved commands retain whole-value approval semantics;
new `user-path-entry` gates never reinterpret an old approval.


Upstream Linux Nix deletion preserves classification evidence until the store is
gone, records bounded membership chunks, and inherits an independent flock in rm.
Unknown native outcomes are never replayed. Fresh recovery requires the same root
and only approved remaining members, with no active deletion or live consumer.
Root's default profile is exempt only after reading its literal package manifests;
never evaluate Nix expressions for ownership. A Determinate installer's default
profile holds the Nix from its release's nix-src input, which can lag the
installer version (v3.21.2 bundled 3.21.1): match `legacyDeterminateBundledNix`,
generated with the release pins from each tag's flake.lock. An indirect root
into the user's own Nix cache (flake registry) is retired user state, not an
outside consumer; every other root target stays a consumer. nix-darwin's
activation link /run/current-system is cleared at boot when its daemon does not
run; the system profile then identifies the active generation only when its etc
is exactly /etc/static's target. Without that agreement nothing is inferred. Its semantic proof is included in
membership evidence before deletion. Old evidence without that proof cannot
authorize partially deleted root profiles. Account inventory errors or duplicate
names fail closed, and executable/library/cwd/root/open-file consumers all count.
Linux inspection runs the installed executable's read-only procfs helper under
existing nonprompting privilege. Hold one process-directory descriptor and a
pidfd. pidfd readiness alone does not prove disappearance: a fresh pidfd can poll
ready for a moment while a worker thread's exec takes over the leader PID (Linux
6.12, measured 2026-10-23; State R right afterwards), which let the exec stress
test omit a live store consumer. Readiness counts only when a fresh pidfd_open of
the same PID then finds no thread-group leader (ESRCH/EINVAL/ENOENT); an unreaped
zombie keeps its leader. After a failed read, explicit kernel-thread
status/PF_KTHREAD or Z/X with exactly one thread also proves no user address space. Zero threads can describe the old leader
released during a worker's exec while the same PID remains live. Live/unknown
processes still refuse removal, with bounded reference and State/Threads fields.

A complete fd listing must reach EOF and its final directory position must equal
a later positive FDSize plus two. This kernel cursor proof detects early success
when de_thread drops the old leader's files. Keep the directory handle open
through classification, including when inspecting the helper itself. A failed fd
read requires a second independently proved listing: absence proves closure;
presence requires one successful re-read and normal Nix classification. Later
State/PF_EXITING samples and repeated unproved listings are insufficient.
An open maps file pins one address space. After a scan finds no Nix path, rewind
that same handle and require one byte; a destroyed view can otherwise return
silent EOF. These checks do not freeze processes or retry native mutations.
Deterministic native pthread/exec regressions run as ordinary users and as root
on disposable Linux CI; the root view can be empty where user reads are denied. With
positive validated PIDs and zero pidfd flags, ESRCH/EINVAL/ENOENT also mean the
listed process identity no longer has a thread-group leader; other syscall
failures refuse inspection. Native re-exec fixtures retain their original owner
PID across exec and exit if that owner disappears, including abrupt test timeout.
Their C notification files are closed before atomic publication. Publishing a
"closed" marker while its stream remains open can itself reuse the descriptor
being tested; waiting for pathname existence alone would then race the fixture.
An inaccessible automatic root is never assumed stale. Recheck only its lstat
through the installed executable's read-only helper with existing `sudo -n`
credentials; only ENOENT proves absence, and repeat at the removal gate. Live
links, missing credentials and all other inspection errors still block removal.
Vendor system text can be root-private (including fstab and synthetic.conf).
On permission denial, read at most maxProfileBytes through the same executable
with existing nonprompting credentials; require a plain file and matching byte/
mode snapshot. Generic Mac cleanup after vendor removal and privileged step
pre/postconditions use the same bounded fallback.
Do not chmod system files to inspect them, follow links, print their bytes, or
reinterpret missing credentials as absence. Report the sudo -v prerequisite
when a preview cannot read administrator-only evidence.

Determinate receipts are typed vendor actions, not an allowlist of path strings.
Bind installed uninstaller bytes, receipt, accounts, services and APFS identity;
preserve system configuration before invoking the vendor. `uninstall --explain`
is not a read-only preview. A no-init receipt excludes no daemon process and must
not require systemd. Linuxbrew zsh retirement repairs only this account's exact
released login path, verifies its native replacement and protects other accounts.
The uninstaller's parent remains root-owned and protected. Its executable needs
either protected root ownership or the exact reviewed release size/hash for its
receipt version and Apple Silicon target. The latter route never executes the
writable source during preview, including for --version. Generate historical
release pins with `scripts/generate-determinate-release-pins.py`; `--check`
reproduces official asset metadata and downloads older assets lacking digests.
The 72 published Mac assets authenticate bytes, not 72 native lifecycle tests.
Both routes copy the binary and receipt into a new private administrator-owned
directory, bound both reads, hash the copies and exec the staged binary directly.
Keep the worker attached to the vendor process. Disclose retained staged copies.
Old literal command recipes remain readable; the optional owner/trust fields
alone preserve the old protected-root gate hash while distinguishing release
authentication. Linux socket and service activation links are additional approval evidence:
record their exact snapshots (including absence) and only accept links to the
receipt's own unit path, including systemd's fully resolved regular unit file
inside /nix/store. Resolve that exact source; a store-path pattern is not proof.
Both fresh upstream retirement and interrupted recovery remove store-resolved
activation links before deleting their source, while primary unit links retain
classification evidence until afterwards. If an upgrade changed the source or
the store was removed outside this plan, refuse unproved links and name only
the exact activation link for ownership-confirmed manual removal. Offer this
advice only below the exact systemd sockets.target.wants or multi-user.target.wants
directory; primary unit/tmpfiles links require manual inspection. Do not suggest
systemctl disable: it also removes linked primary unit files.
A partial vendor revert can leave these links when its
unit is already absent. Plan exact follow-up removal and daemon-reload before
reporting completion; old evidence cannot acquire this new removal authority.
Adding these links changes the Linux approval identity and requires a fresh
preview instead of widening an earlier approval.

Systemd stop steps query ActiveState at dispatch, skip inactive/failed/unloaded
units, and stop sockets before services. Failed is already stopped in systemd;
stop does not clear that state. Only exit 5 followed by a fresh inactive or failed
observation is an accepted race; all other failures and unknown states remain
errors. New completion checks use stopped; historical inactive checks retain
their strict meaning and require a fresh preview for a failed unit. Dedicated completion checks can prove an already-stopped
operation, including after a reboot or partial vendor revert. The final full
kernel consumer scan is never bypassed. Old saved command literals are not
rewritten; abandon a failed old stop recipe and approve a fresh preview.
For init-managed vendor installs, stop only the receipt's Nix daemon and sockets
after checking static consumers and build accounts, then require a kernel scan
with no process exemptions before uninstall. Remaining consumers retain the
store and recovery journal while the daemon stays stopped. Vendor descendants
are not ownership proof; the running service includes a separate Nix daemon and
crash reporter. Preserve older saved gate recipes for interrupted operations.
Successful launchctl bootout can precede service unregistration. Only completion
checks wait for observed absence, bounded by 30 seconds and cancellation;
inspection errors and the immediate pre-removal gates remain strict.
Preserve dscl attribute namespaces: IsHidden is normally emitted as
`dsAttrTypeNative:IsHidden`. Recognizing that spelling must not weaken build
account identity or let native lookalike attributes replace standard IDs.
Native migration fixtures run only on opted-in disposable hosts; Linux flock/GNU
rm recovery tests run on Linux, and macOS has its own native volume lifecycle.

Nix-managed Homebrew conversion restores only the reviewed, fixed repository
source, preserving installed package records, versions, links, pins and taps.
Read those records without evaluating old Ruby: it can fail on or silently omit
newer cask definitions. Do not use the general installer for conversion; its
explicit global update can migrate unrelated packages and taps. Preserve source
roots before checkout and validate every fetched path against the reviewed set.
Reject executable Git filters, partial clones and submodules before read-only
tap or released-plugin status/diff. Released plugin clones may contain empty,
never-initialized test submodules (notably tmux-resurrect); permit those only when
both the submodule worktree and Git module caches are absent or empty. Populated,
initialized or deinitialized cached data stays protected. A missing tracked
submodule still fails the subsequent ordinary clean-status requirement.
Read effective local and worktree settings,
including includes; --local alone misses config.worktree. Verify plugin HEAD and
origin before status. History-based blob evidence also refuses partial clones:
even log --find-object can fetch promised trees. Bootstrap Ruby with setup-ruby
so developer mode stays disabled.
Keep the full command recipe in saved intent; unrecognized repositories cannot
be adopted as interrupted conversion work. Historical homebrew-install recipes
remain readable for existing recovery journals. Native v0.4.4 fixtures pair
contemporaneous core/cask migration maps without editing the immutable release.

Yamllint and LaTeX conversion share only the real private-venv preparation steps;
fixed package recipes and complete Python-pin identities remain separate. Native
package install/remove is forbidden on the developer workstation; explicitly
opted-in disposable runners are the acceptance boundary.

Hidden Linux desktop libraries use APT package-file verification, not invented
executable probes. `dpkg --verify` stdout is a failure even with exit status zero;
real native GUI tests separately establish ABI and resource compatibility.

Linux Code uses its exact checksum-pinned official DEB through the existing APT
provider, matching [the vendor installation path](https://code.visualstudio.com/docs/setup/linux). `/usr/share/code/code` matches the distro's shipped AppArmor profile;
private archive paths do not. The official helper is root-owned mode 0755, not a
setuid helper. Do not disable the sandbox or install a custom AppArmor policy.
APT owns Code's dependency closure. Reuse healthy preexisting Code without
ownership, reject foreign publication collisions, and protect a changed
postinst-created `/usr/bin/code` before removal. The Code-only library catalog
mirror is removed; shared Ghostty requirements remain. Linux Code uses the
package-owned `/usr/share/applications/com.microsoft.VSCode.desktop`; do not add
an archive-health-dependent desktop resource or private shell dependency.
Native package versions are approval/display metadata, not ownership identity.
Recognize the initial versionful Code receipt only through its exact completed
operation and unchanged source/manual/hold identity; normalize after the next
successful operation. Reviewed Code updates may downgrade to the fixed checkout
pin, using Code-only saved `--allow-downgrades`. Old pending command arguments
remain frozen. APT success/recovery history accepts only the exact saved Code
old-version/new-pin downgrade tuple; other downgrades and removals remain errors. `Observation.ApplyDetail` is bounded, optional display metadata,
not authority; use native dpkg ordering to disclose current/pinned versions.
Only a healthy, operation-proved owned Code at the reviewed version receives the
pin digest as `Desired`. Existing planner equality then keeps it without another
DEB download. Drift, changed pins and failed health still require their reviewed
update/repair; unowned Code gains no authority. Older receipts without `Desired`
perform one explicit reconciliation before subsequent same-pin updates are kept.
The new Protected flag must not invalidate older pending install baselines that
omit it. Fresh native protection still forbids removal of protected packages.

Code's maintainer scripts can rewrite/remove Microsoft repository files and keys.
Every Code install/update/removal, including final collection from the owned
pool, uses a fixed debconf override accepting only `code/add-microsoft-repo`.
Save its exclusive caller-owned 0700 `/var/tmp` directory and native identity
with the operation. The nonsecret answer is 0644 inside that private directory
so a root rewrite remains inspectable. Verify its owner, bytes and location
before dispatch; retain inputs when the worker outcome is unknown. Cleanup
accepts only the exact database and known File-driver rewrite/backup files;
unexpected contents remain preserved. Persist verified native completion before
optional cleanup of the exact hash/size-verified DEB and empty hash directory.
Changed, redirected or uninspectable input paths are disclosed through existing
`Preserved` metadata, including after interrupted cleanup and later removal;
previously disclosed paths never regain deletion authority. Cleanup cannot strand
a completed package, but unfinished worker outcomes still retain all inputs and
cannot claim completion. Relative overrides are invalid: dpkg
unconditionally changes maintainer scripts' working directory to `/`, even
when APT's `DPkg::Run-Directory` is set. Never copy the personal debconf database
or substitute shorter package/profile/state/executable paths.

GitHub's Ubuntu image deliberately makes `/usr/share` mode 0777 in its
[image provisioner](https://github.com/actions/runner-images/blob/ubuntu26/20260927.149/images/ubuntu/scripts/build/configure-system.sh).
The opted-in disposable desktop job verifies a physical root-owned `/usr/share`,
logs its mode, and sets only that directory to 0755 before package acceptance.
This is an explicit trusted test-host precondition: no recursive chmod, payload
mode changes, production chmod, or relaxation of writable-parent health checks.

Ghostty uses one reviewed Linux DEB per architecture on Ubuntu/Debian. Its fixed
archive preparation supplies the bundled layer-shell library to both CLI and
GUI through the declared command wrapper; retain usr/share resource layout.
GTK and libadwaita are compatible only forward, so the pin is upstream's build
for the oldest supported release, Ubuntu 24.04 (the t64 library names exclude
22.04 and Debian 12). The Debian 13 build imports GTK 4.16 symbols and crashed on
24.04 at desktop launch (2026-10-21); archive health checks payload bytes, not
symbol binding. `TestNativeGhosttyLoadsOnOldestSupportedUbuntu` loads the pin in
a digest-pinned 24.04 image with `LD_BIND_NOW=1` and requires the old build to
reproduce that failure; the hosted desktop lifecycle covers Ubuntu 26.04. Keep
both when changing the pin. Do not introduce distro-specific recipes without
evidence that the common one fails. Private Linux launchers whose file name
differs from the window class declare `StartupWMClass` (Ghostty's GTK ID
`com.mitchellh.ghostty`, Zen's RemotingName `zen`, OpenWhispr's Electron desktop
name `open-whispr`, set by its `app.setDesktopName`); without it GNOME shows a
generic icon and a launcher click starts another window. The hosted Linux
lifecycle requires a mapped window carrying the declared WM_CLASS class. Check
every new private launcher against its application's actual window class.

Archive and bootstrap checksums are reviewed pins, updated together with real
runtime verification. The Update menu reconciles the current checkout's versions;
it does not bypass pins or fetch arbitrary upstream latest releases. Renovate
continues to cover the dependency sources declared in its extraction inventory.

Windows shortcut preparation uses an operation-and-attempt identity, exclusive
private staging and verified atomic input publication. Failed/unfinished native
records and partial inputs remain evidence; a later attempt uses new paths.
Once ConfigDriver records the source, recovery must reuse those exact bytes,
including legacy recipe-only inputs. COM shortcut bytes are not assumed stable
across runs. Never regenerate an input belonging to a saved configuration journal.
Shortcut preparation suppresses PowerShell progress only in its child script:
first-use module loading can otherwise add CLIXML to the native worker's combined
hash result. Keep terminating errors, failed exits and exact digest verification;
never strip unexpected output to manufacture a hash. The ordinary Windows COM
fixture uses system PowerShell 5.1 and private files, while the public desktop
fixture still proves actual launch/configuration/update/removal.

Windows Terminal owns fixed JSONC leaves and identified profiles/schemes/themes/
keybindings in the pinned unpackaged stable instance. Preserve every existing
nonempty defaultProfile during install/update. On removal, restore the saved
default when the current default is the managed GUID and its profile is removed;
preserve unrelated defaults and choices pointing to a retained profile. Bind raw
baseline projections to receipt inventory, but
fingerprint only owned values so personal nested additions remain removable.
Terminal 1.25 serializes the fixed `closeTab` command as `{"action":"closeTab"}`.
Treat only those exact argument-free forms as equal in the owned fingerprint;
preserve the original string fingerprint and raw baseline/restoration values.
Additional arguments or other actions remain changed. Failure-only native
diagnostics report bounded fixed owned-field differences, never entire settings.
Modern bundled action IDs replace legacy combined action/key syntax; ambiguous
legacy overlaps require the native application to migrate them before adoption.
Do not emulate its serializer or add Store/Preview/Canary target adapters.
Native acceptance must invoke the owned window's Settings Save and observe its
real file write before checking ownership. A successful serializer may write
identical bytes; record exact write timestamps and hashes rather than requiring
an arbitrary value change. Missing controls, failed Save or no write are failures.

Homebrew linkage inspection has a five-minute query budget; ordinary queries
retain 30 seconds. Preserve typed deadline/cancellation causes and propagate them
from root health and dependent maintenance. Interrupted inspection cannot prove
broken linkage or authorize reinstall. A completed substantive batch failure
remains a failure even if individual probes pass.

WezTerm is retired from every active platform by explicit owner decision. Keep
its passive `home/dot_config/wezterm/wezterm.lua` and exact released inventory
unchanged so existing live links and migration evidence remain readable. They
are not an active configuration route or permission to remove an old installation.
Native desktop acceptance launches published OS integrations;
a PID/version alone is not configured behavior. On macOS, inventory uses the
kernel executable-vnode path, not `ps comm` or launch arguments: a published
application symlink retains its alias in `ps`, and an alias beneath the package
root can point to an outside executable. Resolve the package root and require a
physical descendant before fixture cleanup. Missing processes/executable paths
never grant signal authority; other inspection errors fail the fixture. Keep
real window/configuration witnesses and the original Unicode/long paths.
AeroSpace waits for Accessibility
consent before config/server startup. Without consent, report launch separately
from unverified configuration/tiling; never bypass TCC to make a test green.

Run the opted-in Windows public migration fixture last in the core job. It may
reuse only absent or empty-ready setup state; it refuses active ownership or prior
migration evidence. Released CurrentHost Microsoft.PowerShell_profile.ps1 and new
AllHosts profile.ps1 are separate paths. Historical detachment cannot authorize
replacing either later personal content or active scoped setup blocks.


Read-only font inspection and Windows desktop process/window fixture queries parse
stdout only through `runIntegrationQuery`. Encoded PowerShell commands can emit
CLIXML module-loading progress on stderr even when they succeed. Keep diagnostic
stderr separate, preserve failed exit codes with bounded sanitized diagnostics,
and enforce the combined 1 MiB inspection limit. Never repair malformed protocol
output by trimming a CLIXML prefix or treating a failed native query as absence.
Darwin's read-only CoreText query keeps JXA source static and passes the exact
ordered PostScript names as a separate JSON argument to `run(argv)`. Parse that
argument as data; do not interpolate it into source or add shell quoting. The
prior bare JSON-array expression was safe, but matched CodeQL's quote heuristic.


Apple defines CLT uninstall by removal of `/Library/Developer/CommandLineTools`;
historical receipts may remain for Software Update. Receipt presence alone does
not veto a missing-payload install. Enumerate `pkgutil --pkgs`, strictly validate
an existing executable receipt's package ID, version, root volume/location and
install-time, and bind that identity plus selection to approval and schema-2
intent. Schema-1 operations remain readable but cannot resume with the new recipe.
Require exact receipt/selection stability before dispatch and after discovery;
require a changed install-time, operation-bound native evidence and compiler/SDK
health before completion. Resume may finish selection only against the saved
installed receipt. Present/broken payloads, foreign selections, query failures and
unknown native outcomes remain protected. Only the exact xcode-select exit/status
message denotes no selection. Never forget/delete receipts or manufacture a
receipt-free runner. Hosted acceptance proves missing-payload bootstrap with
historical receipts, preserving moved tools, developer siblings and Homebrew; it
does not prove a Mac that never had CLT or an unobserved same-version reinstall.
No advertised compatible package is an explicit failure, never a skip.
The hosted receipt boundary inspects `pkgutil --only-files --files`, because the
ordinary listing includes directory records such as `private`. Outside or
noncanonical files and query failures still stop preparation before payload moves;
diagnostics report at most 20 paths of 256 characters plus an omitted count.
Representative preservation hashes stream only the fixed files, bounded by each
file's observed size; native compiler binaries can exceed a configuration-size
cap. Keep the existing one-entry and concurrent-change checks.

Passive released profiles, target source files and referenced templates retain
their latest recorded release bytes, including obsolete comments or generators.
Release inventory hashes protect those files; active shell recipes live under
`installer/shells/`, and active tmux generation must not rewrite `home/` mirrors.
Retained `home/` and `windows/chezmoi-*` text sources force LF checkout so Windows
`core.autocrlf` cannot invalidate the recorded physical-byte hashes.
Keep default LazyGit bindings in the active `lazygit/config.yml`; migration/adoption
replaces released live-link configuration only after preserving its original.

Cross-platform boundary fixtures use native absolute paths even when exercising
another platform's recipe. Windows POSIX subprocess fixtures use Git Bash
explicitly, LF scripts and MSYS drive paths; system `bash.exe` may launch WSL.
Migration inventory gates require complete release history on every runner.
The full Go race suite has a bounded 30-minute budget (owner decision
2026-10-22): Windows' real subprocess fixtures exceeded Go's default ten-minute
total while continuing to make progress, and the 20-minute limit then left no
headroom. core (windows-2025) took 962s at 67aae3b and 1057s at 60a3148, then
timed out at 1dca9af while the same tests ran 16% slower; the separate windows
job passed that commit. A hung test still fails at the limit; never raise it to
hide one. Hosted Windows runs file-heavy tests about 3.4x slower than a Mac. Each full copy of the twelve Sentinel skill bundles is costly
there: prove presence/selection variants with read-only previews, run one
lifecycle per distinct root, and require only the roots a test exercises. The
first Kiro/Copilot skills tests added ~230s and timed out both Windows suites.


PowerShell bootstrap phase diagnostics belong on stderr; stdout in machine mode
remains the command's JSON result. Record phase start/completion/failure and elapsed
time without suppressing errors or widening a fixture timeout. A bootstrap timeout
before the first setup response does not prove a migration-engine failure.
PortableGit SFX extraction failures require native evidence: its `-y` switch hides
error dialogs, and the matched upstream SFX build disables long-path support.
The hosted failure-only dialog/extended-path experiment is diagnostic evidence,
not a production fallback or lifecycle pass. It uses checksum-reverified bytes,
fixture-owned sibling directories and only the launched extractor's window PID.

Git Bash boundary doubles must be prepended after its `bin/bash.exe` wrapper
initializes PATH; the wrapper puts Git tools first. Compare working-directory
identity through native paths on Windows because `/tmp` is a valid MSYS alias
for the same directory. Keep production bootstrap platform checks unchanged.

PortableGit extraction uses an extended Windows output path because its pinned
SFX is not long-path aware. A successful SFX exit does not prove its post-install
child succeeded. If the upstream script remains, run that exact manual entrypoint
with the normal payload working directory. Stage unchanged script bytes in an
exclusively created sibling: upstream deletes the literal `post-install.bat`, so
CMD must finish reading a different input filename. Keep the copy on failure;
remove only the same unchanged file after the command and runtime checks succeed.
Both post-install artifacts must disappear, and Bash/Git/LFS/credential/SSH must
work before publication. Never accept a nonzero exit or shorten ownership paths.
Git configuration isolation uses `/dev/null`, which Git for Windows recognizes;
Go's uppercase `os.DevNull` spelling `NUL` is not a valid Git config-file boundary.
SFX and wrapper child waits rule out the proposed post-install child race.

PortableGit runtime failures must identify the failing fixed probe step. The
disposable Windows fixture enables GCM tracing only for the existing private
`--version` runtime probe, with secret tracing disabled; preserve the original
exit and journal. Git core.longpaths does not prove .NET GCM path support.

The Build Tools process-tree fixture holds an exact child HANDLE before releasing
its parent. A failed immediate wait remains a failure even if later diagnostic
waiting observes termination; do not replace that assertion with a grace period,
PID/name filtering or an assumed empty-job synchronization guarantee.
The private Git runtime probe converts `/etc/gitconfig` explicitly with `cygpath`
before passing it to native Git; an empty `MSYS_NO_PATHCONV` still disables
implicit conversion. GCM's fixed version check runs directly through the native
session with only its copied TMP/TEMP using the extended Windows spelling of the
same private directory. MSYS specially converts TMP/TEMP before generic conversion
exclusions, so it must not mediate this native path boundary. Keep suggestions,
executable/config bytes, physical paths and strict failures unchanged. Verify the
actual suggestion-registration file in that original private directory and normal
published `git credential-manager --version`; no authentication is required.
Windows Git's `core.longpaths` supports long file paths inside an ordinary
worktree; it does not remove the native working-directory limit. Keep native
coverage beyond 320 characters for nested directories/files, plain add/commit,
clone, checkout, diff and exact bytes, using the private system setting without
an override. Do not require an arbitrarily long repository root: upstream
[PR 3877](https://github.com/git-for-windows/git/pull/3877) documents the separate
`_wchdir`/current-directory limitation. Preserve the original managed payload,
home and Neovim paths when correcting a fixture's unsupported assertion.
The Lua checkout regression proves long files beyond 320 characters inside a
supported repository root, and separately proves native Windows refusal at the
observed 254-character stage / 266-character `.git/config` boundary. The refusal
must preserve the original target and remove only its own stage/lock. The 257-root
control fails earlier still. Neither command/system config nor an extended Git-dir
prefix solves pre-config repository discovery; do not add configuration plumbing,
metadata aliases or compact staging as a purported complete remedy.

Public Windows Neovim acceptance uses observed native known folders and the
LocalAppData data default, without an artificial XDG data override. Preview,
apply, read-only diagnostics and the normal editor consumer must agree on that
root. Preserve private fixture HOME, original profile/config/editor-data snapshots,
notes, full operation identities and all install/use/update/shared-removal checks.
Create personal-data witnesses exclusively and delete only unchanged fixture bytes.
The v5 temporary XDG root adds 75 characters to the actual default: lazy.nvim's
stage is 254 instead of 179, while final plugin roots reach 259 instead of 184.
StateRoot is separate from Neovim's data root. Custom/redirected data roots remain
subject to upstream repository-discovery and working-directory limits. An explicit
native error is a refusal, never permission to silently relocate user data or
shorten ownership identities. Do not infer a native pass from a diagnostic step
that merely logged failed variants.

OpenSSH `-o` parses configuration-file syntax even when its value is one argv
entry. Use the dedicated `-i` filename argument for fixture identity paths with
spaces; preserving argv boundaries alone does not quote an `IdentityFile=` value.
Keep `-G` inspection offline and retain the same physical key path.
Use OpenSSH's documented `-F none` to disable both personal and system config
reads. The bundled MSYS OpenSSH does not accept Windows `NUL` as that file path;
its explicit no-config option avoids null-device path translation entirely.

Native GUI acceptance uses observed macOS and Windows user folders, preserving
existing configuration through the ordinary adoption/removal journal. A fake HOME
does not redirect macOS Foundation's home or application-support directories.
Clear controlled Electron/Node/resource mode variables by removing them from the
fixture child environment, not assigning empty strings: Electron's Windows
entrypoint tests getenv_s's required size. The launch API's returned PID/path is
bounded diagnostic evidence only; executable-vnode ownership and native window
checks remain the cleanup and acceptance authority. Never launch native GUI or
mutate system packages on a contributor's daily Mac to validate these fixtures.

Fixed encoded PowerShell commands must bind module discovery to their own
`$PSHOME/Modules`. PS7 adjusts module paths only for directly launched PS5.1; a
Go intermediary otherwise inherits incompatible PS7 Utility modules and loses
Get-FileHash. The bootstrap imports its two built-in Management/Utility manifests
by exact current-engine paths without changing the caller's PSModulePath. Keep
this child-scoped; no machine/user environment edits or engine fallback.

Pending unprefixed VC++ and Build Tools commands remain refused without journal
writes or another native call. Completed journals may validate the exact old
encoding of the same fixed script without the module prelude; every other command
field and successful native completion proof must still match. This is read-only
receipt compatibility, never authority to dispatch an old recipe. New commands
always bind built-in modules. Keep completed observation/check/removal and pending
refusal regressions together; retiring chezmoi does not retire native receipts.
Changes to the fixed vendor script constants require an explicit completed-receipt
compatibility audit: generating historical fixtures from the new script alone
cannot prove that actual saved commands remain readable. Keep historical script
provenance tied to Git objects and never replay a previous pending recipe.


## Native desktop fixture launcher output

A successful GUI launcher may exit while its application retains inherited
stdout/stderr. Native desktop acceptance must pass private regular OS file
handles to that launcher, then read at most 1 MiB for diagnostics. Do not attach
Go writer pipes and wait for GUI descendants to close them, ignore ErrWaitDelay,
leave a detached Wait goroutine, or extend the existing launch deadline.
Nonzero launcher exit, cancellation and file errors remain fatal. Success still
requires the exact owned executable, a native window and actual settings witness;
cleanup stops only owned application processes before the final log check.

Every VS Code launch needs its own initially absent extension-host settings
witness; singleton and complete-group launches share a home, so an earlier file
cannot certify the next invocation. Windows fixture cleanup holds the candidate
process HANDLE and validates its physical executable beneath the private root
before signaling. Termination ACCESS_DENIED can occur during rundown: wait on
that exact object within the existing shared deadline and require WAIT_OBJECT_0.
Any image-query failure permits only passive waiting, never a signal: Windows
does not guarantee one error code after process rundown. Exact WAIT_OBJECT_0
independently proves completion; a live or unproved object retains the original
inspection error. The final empty-root inventory remains required.
Release POSIX os.Process handles after signaling and retain release errors.

A private Windows job can include conhost.exe as well as the exact fixture root.
Do not assume one member or an empty post-exit list. The natural-exit regression
holds all ready private HANDLEs, proves the root's zero exit and decreasing Active
with stable lifetime Total, then captures/terminates remaining members and requires
all held HANDLEs already signaled. Image names are diagnostics, never authority.

APT consumer observation retains declared but absent dependency alternatives.
A pre-existing browser's GTK3-or-GTK4 declaration must appear in GTK4 preview
before an approved install makes GTK4 present. Resolve installed names, exact
architectures and Provides as before; unresolved names are declarations, not
installed packages or removal authority. Keep the executor's consumer equality
and native inventory approval checks strict. Conservative outside-consumer
protection may retain either installed alternative; do not delete a personal
consumer or invent an alternative-version solver to collect it.

For an absent APT selection, disclose both bare dependency declarations and
those explicitly qualified with the observed native architecture. Debian's
explicit architecture qualifier matches only that architecture; do not count a
foreign-qualified alternative as a native consumer. Pass the existing platform
architecture into the provider rather than guessing from installed packages or
executing discovery in its constructor. Present-package matching remains against
the exact installed identity; this observation rule does not resolve versions,
change persisted ownership, or relax approval equality.


The Windows inherited-module regression records the unmodified PS7 → Go → PS5.1
baseline as diagnostic evidence: accept either the exact expected hash or the
known Get-FileHash autoload error. Do not require an upstream bug to persist;
unexpected baseline results still fail, and all corrected process/bootstrap
assertions remain mandatory with PowerShell 7 required. The Windows Git refusal
fixture measures the 254-stage/config 266 boundary in UTF-16 units using pinned
Neovim's `vim.str_utfindex(..., "utf-16")`; UTF-8 byte length mismeasures Unicode
and astral temporary roots. This changes test fidelity, not supported path layout.


Hosted Ubuntu completion tests require trusted native completion directories. The
runner image recursively makes `/usr/share` writable; installing zsh does not
replace the pre-existing `/usr/share/zsh` and `vendor-completions` directories.
Use the fixed GitHub-hosted Linux preparation helper: the desktop recipe secures
only `/usr/share`, while the shell recipe also secures those two completion
directories and requires strict empty `compaudit` output. Verify the entire set
is physical and root:root before any nonrecursive mode change. Never turn this
fixture precondition into production chmod, a completion security bypass, or
ignored startup stderr. Core's archive zsh PTY is an interactive consumer too.


Bootstrap PowerShell helpers follow the same singular-noun rule as cmdlets.
Keep `Import-BootstrapModule` and the two AST-based extraction tests in sync;
its body still imports both built-in modules by exact current-engine path.
Native Windows PSScriptAnalyzer verification is required for naming findings;
portable parsing and successful module-binding tests do not replace that check.
Do not suppress `PSUseSingularNouns` for a plural internal helper.


A native Windows shell fixture that installs shared `tool.powershell` must
separately prove default retention and explicitly request its per-item removal
before asserting all shell integration is gone. Retained PowerShell requires
`integration.shells`; removing that edge or normalizing leftover block bytes
would weaken the contract. Prove the owned archive and exact personal suffix
survive default deselection, then remove only that owned runtime and retain the
pre-existing VC++ runtime. POSIX fixtures need no Windows removal authorization.


Windows job-information buffers stay typed through the `unsafe.Pointer` query
helper until the annotated `LazyProc.Call` boundary. The pinned x/sys generated
Query/Set wrappers accept bare uintptr without `uintptrescapes`; Go may move a
caller stack after that conversion. Use the same direct annotated boundary for
the fixed kill-on-close limit input. Judge the Windows BOOL return before its
always-present last-error value; keep System32 resolution, fixed buffer bounds,
exact member HANDLE waits, late-member refusal and the existing time budgets.
Failure-only accounting diagnostics cannot authorize success. Native kernel
sentinel output and limit readback, original-wrapper red proof where reproduced,
and actual compiler reuse complement compilation/escape-analysis evidence. Do
not claim an old Set runtime failure when its diagnostic did not reproduce one.

A tree-inspection refusal is distinct from a changed approved fingerprint. Its
failure message reports only a bounded, quoted root-relative entry, kind,
before/after mode, size and modification time, actual SameFile result, and the
original Lstat error. Do not perform another native identity query to explain an
earlier result: a later open cannot identify the original error. Preserve every
defined inspection predicate and refusal. Windows' WIN32_FILE_ATTRIBUTE_DATA
explicitly gives directory size no meaning: exclude only Windows directories
from size comparison, retaining identity/mode/time and all child inspections.
Regular files, links/junctions and POSIX size checks stay unchanged. This changes
no serialized fingerprint. A later complete lifecycle without recurrence
is non-reproduction, not a fix or proof of the original cause; retain the failed
context and never infer cache/antivirus mutation without evidence.

Archive inspection failures become Unknown/Pending observations, rather than
direct driver errors. Their diagnostic text is therefore bound into a blocked
preview's plan ID. Report already-observed file metadata only; do not add read-
time clocks or later probes. Unknown/Pending still cannot authorize mutation.

Linux desktop acceptance treats X tree enumeration as a changing observation,
not a snapshot. Discard failed enumeration stdout. Only exact typed
tree/property/stats disappearance errors, a fresh live-root check and Window-
specific BadWindow/GetProperty on that same non-root ID allow the existing poll
to observe absence. Unknown errors, display/root faults and cancellation stay fatal, including
`xprop` failures. Require the window's `_NET_WM_PID` to match an owned executable
process and its native map state to be `IsViewable`. The original 45-second poll
and fresh application-settings witness remain unchanged.


Native Windows Terminal acceptance treats its configured shell witness and visible
owned window as separate readiness conditions. A valid zero MainWindowHandle means
pending within the existing 30-second startup deadline; query/parse errors, owned
process exit and timeout remain fatal. Each process query inherits the remaining
startup deadline as well as its 15-second cap. Only a nonzero window belonging to
the exact launched PID can proceed to native settings serialization and clean exit.
If the deadline kills an active query, preserve both the context and process errors
and report the number of queries on failure as well as success.


Both bootstrap compiler environments explicitly set GOMODCACHE under the private
bootstrap GOPATH; inherited user module-cache settings must not redirect compiler
writes. POSIX uses the existing compiler subshell and PowerShell restores saved
environment values in finally, so the launched installer retains caller settings.


Public POSIX native-bootstrap fixtures own a real Go module cache with read-only
directories. Before t.TempDir teardown, use the executing host Go's clean -modcache
with an explicit GOMODCACHE limited to that fixture's private directory. Never
weaken production cache permissions, ignore cleanup errors or clean a caller's
personal cache. Keep a neighboring-cache preservation regression.

Real Neovim UI fixtures locate bundled parser inputs through upstream's
`parser/<lang>.*` rule: Windows ships `.dll`, POSIX `.so`. Keep the managed target
name and payload-preservation assertions unchanged; fixture source discovery must
not assume the host archive uses the managed parser naming convention.

Fresh Windows vendor CI uses Windows Server 2022 containers. A diagnostic step that
cannot fail the job runs right after the image pull and times the first PowerShell
container, a bare container and a second PowerShell container (first WMI query,
Authenticode, host and container builds). The lifecycle therefore never starts the
first container after the pull, matching the VC-runtime job's earlier Python check.
This is a hosted-runner precondition and evidence for the intermittent Build Tools
first-inspection timeout (2026-10-23), not a lifecycle proof. The inspection bound
itself rose from 30 to 120 seconds the same day, after hosted Visual Studio cold
inspections exceeded 30 seconds. Prepare its existing
`docker` service explicitly: start only a stopped service once, verify the actual
Windows API under bounded readiness/probes, and preserve an already-running
process. Missing/stopped-after-start services and non-Windows engines fail. Never
install or re-register Docker, change service policy, repeatedly restart it, or
retry the pinned image pull/vendor lifecycle to hide failure. API readiness has
a 60-second budget; the enclosing two-minute step also bounds service startup.
Use `docker version --format '{{.Server.Os}}'` with exit zero and exact `windows`
to prove the server API. `docker info` enumerates client plugins before its server
query even with a format, so it is not a minimal readiness probe. A query consumes
the remaining overall deadline; do not invent a shorter cold-client sub-deadline.
Native readiness proof must make this the first Docker CLI invocation on its host;
a preliminary `docker info` call warms plugin discovery and changes the fixture.


Bootstrap process fixtures read Bash-written path text as explicit UTF-8 on every
host. PowerShell's native-command error preference legitimately selects either
the wrapper's explicit compiler-exit error or the engine's original
NativeCommandExitException; test both preference values, exact exit 17, and
caller-environment restoration. Observe the child compiler's GOROOT, GOTMPDIR
and executable alongside its module-cache write, so dynamic AST-bound fixture
inputs have real behavioral assertions and require no analyzer suppression.

The owner expanded product scope on 2026-10-12 to replace Pi entirely with
Claude Code and Codex CLI. This capability replacement has its own current
verification. The owner subsequently authorized squashing all branch changes into
one commit with a meaningful message, superseding the earlier two-commit and
no-rewrite instructions.

Desktop smoke corrections (2026-10-13): Flatpak list ref output omits `app/`;
updates/removals compare the last verified receipt.After, preserving receipt.Before
as the original uninstall baseline. Desktop publication verified the actual Flatpak
provider until Linux Obsidian moved to its portable build on 2026-10-22; the
Flatpak launcher is no longer published. Keep native command outcomes and directory
ownership through recovery.
Tailscale conffiles are personal: dpkg `c` lines must not trigger endless repair,
while every non-conffile change remains unhealthy. Zen Linux preparation revision 1
adds DisableAppUpdate before sealing the payload; revision 0 receipts remain readable.
Check must stay read-only, including while graphical applications are running.

VS Code's application-wide `update.mode: none` shares the single JSONC owner
`integration.vscode-managed` with theme/fonts. Keep the old `integration.vscode`
field list unchanged for recovery: widening it invalidates saved baseline/journal
identities. The old resource is recovery-only and has no dependencies. Its owned
or uncertain receipt blocks successor publication until an approved restoration
releases the first baseline. Edited old fields require the existing optional
restoration choice. Every restoration preserves the replaced file separately.
Resume keeps its original approval list: when restored personal settings need
new adoption, guide the user through Abandon, then Choose tools. Missing evidence
blocks handover and must be preserved/restored, never rewritten. One settings file must never have concurrent owners. The native
extension-host witness checks the consumed update setting and theme/fonts.
Never weaken payload fingerprints to accept a vendor self-update.

The former Zen macOS archive pins and preparation revisions are recovery-only.
Keep their bytes frozen; the current selection uses `app.zen` below.

Zen macOS preparation revision 1 resolves the vendor executable's directory with
`pwd -P` before exec. Mozilla's Darwin BinaryPath sanitization removes `./`
substrings even inside `../`, breaking XPCOM lookup when a wrapper traverses to
the sibling vendor bundle. Resolve in a subshell to preserve the caller's working
directory; do not modify or re-sign vendor bytes. Revision 0 must retain its exact
saved preparation for recovery; the current pin's revision drives approved Update.

Zen macOS preparation revision 2 adds LSArchitecturePriority = [arm64] to the
outer launcher only. Apple defaults script-only apps to Rosetta; native evidence
showed a translated process stalled in the loader while direct ARM startup and
the untouched vendor bundle produced windows. Parse XML to locate the root dict,
retain other metadata bytes and refuse any existing upstream architecture declaration.
Keep saved revision-0/1 metadata and scripts unchanged. Do not patch or re-sign
vendor bundles, install Rosetta, or override the test host's architecture to pass.


Native agent CLI acceptance owns each Windows launch in a private kill-on-close
job, assigned atomically at creation with an exact stdio handle allowlist. Verify
Codex's initialization and zero root exit before reaping remaining owned helpers;
then require stream EOF. T3's deliberate stop requires a still-running root.
Retain context deadlines, bounded diagnostics and the two-second EOF bound;
never discard ErrWaitDelay or capture T3 pairing secrets. Reuse the exact job
member HANDLE completion proof from Build Tools queries without changing that
production adapter. The test launcher preserves Unicode CWD and Go's Windows
environment override semantics. POSIX keeps normal command cancellation.

The POSIX public-bootstrap test cleanup disables Go telemetry only in its private
compiler home/config before `go clean -modcache`. A default local telemetry child
can otherwise recreate directories during TempDir removal. Use the documented
`go telemetry off` command; GOTELEMETRY is a non-settable observation. Keep the
inherited-cache preservation assertion and verify the private mode through Go.

Windows owner-recovery fixtures resolve their home with the native path resolver,
just like profileController: hosted TEMP may contain an 8.3 alias. Exercise an
actual directory alias rather than relaxing publication's physical-parent check.
The agent process fixture can observe a published readiness name while the
renamer still holds DELETE access. Retry only not-found and native sharing
violations within the existing deadline; other errors remain fatal. Its real
held-handle handshake preserves root/child liveness, outside-process survival,
exact job completion and stream EOF assertions.

A completed removal can restore a pre-existing personal configuration that still
observes as present. Keep its removed/reused receipt released only when both the
recorded result and original baseline match the current artifact. Such bookkeeping
uses Forget before maintenance handling, so it cannot retain runtime prerequisites
or turn back into an installed integration. Pending, unknown or changed evidence
keeps the existing retention rules. Preserve schema-1 receipts and personal files.

Retention, reuse, pending work or a later preservation failure must not overwrite
a previously proved removed receipt. Keep its baseline/completion history and
report the current failure or pending requirement through the transaction. A new
approved adoption/publication establishes ownership normally. Recheck released
present baselines after later operations; reuse the final removal verification.
Save any removal completion recovered by Retry before preserving that receipt;
otherwise an old in-progress record can survive a successful transaction.
Recovery-only mappings retain their existing declared-successor handover contract.
Already-absent artifacts
keep that distinct explanation; native deferred-removal pools keep their proof.

Retained macOS Homebrew ownership must survive a reboot: bind the held prefix
directory to its volume UUID and inode, not the boot-dependent device number.
Query the documented volume-root descriptor after matching its mounted filesystem
to the prefix. Schema-1 pre-release ownership/intents cannot prove this identity;
preserve them and refuse affected operations without rewriting their evidence.
Both completed records and unfinished intents must produce a resource-local
refusal, so independent archives remain inspectable and selectable.
Other configuration recovery tokens still bind a fresh approval's native identity.

DMG transport revision 1 disables Apple's synthetic AppleDouble tar entries.
The bounded extractor must preserve actual vendor files, including real `._`
names, without materializing extended metadata as additional signed resources.
Do not filter names, alter signatures or bypass Gatekeeper. Current DMG pins bind
this transport revision; saved revision-0 operations retain their original recipe.


### Native Zen application (2026-10-19 owner clarification)

The owner asked for ordinary application installation with user-controlled browser
setup. macOS Zen is an explicit exception to archive version pinning: use the
canonical Homebrew cask and normal vendor updates. Keep the old private archive
and desktop resources available only for receipt recovery/removal; Linux stays
unchanged. Never relax archive fingerprints to bless an updated private bundle.
Native desktop acceptance must launch and track `/Applications/Zen.app`, including
its removal baseline; the retired private launcher is not the current consumer.
Keep the personal `~/Applications/Zen.app` baseline alongside that native path.

Use the shared Brew journal, worker, approval inventory and command-attribution
proof. Casks never enter the formula dependency pool. The reviewed recipe permits
only Zen's app, quit stanza and known optional command wrapper; no executable
hooks or package dependencies. Install with explicit appdir, required checksum,
no binaries and no sudo, clearing inherited cask options. Because Homebrew's
uninstall can unlink a foreign command even when binaries were disabled, any
existing shared command blocks an owned-app mutation. Validate the installed
uninstall JSON and appdir as well as current metadata. Preserve schema-3 formula
recovery without changing its exact recorded environment.
New formula and cask intents use schema 4 to clear `HOMEBREW_NO_EMOJI`: that
inherited flag suppresses both operation badges in real Homebrew 7. Earlier
formula and cask schema-3 commands retain their exact saved environment during
inspection/recovery. Never reinterpret an old command as the new recipe. Queries
clear cask options too, so approval and execution resolve the same app directory.
Casks publish no shell command directory and never exempt formula pool entries
from preservation checks. Unknown app inspection stays local to that resource;
cancellation still stops the whole operation. Existing authentic bundles under
the user's Applications directory remain reusable even with a native cask receipt.
Command-path inspection errors stay resource-local too. A missing unowned cask
bundle must name native reinstall/uninstall recovery, never suggest adoption.
Native no-sudo acceptance requires usable administrator credentials beforehand,
so missing credentials cannot explain the refusal being tested.

App identity is the Developer ID signature (Zen team `9V5K9TP787`, bundle
`app.zen-browser.zen`) at the approved real directory, never its version or bytes.
Pass codesign requirements with the literal `=` prefix. Use `--deep` and
`--strict=symlinks`: Apple's codesign manual separates signature/resource checks
from optional sideband rejection. The official notarized DMG retains FinderInfo;
that metadata must neither reject the app nor be stripped. Native regressions
must still reject modified executable/resources, outside symlinks and wrong
developer signatures. Never clear quarantine or re-sign an installed app.
Homebrew 7's compact `Casks/zen.json` may be empty. In that format, validate the
native `INSTALL_RECEIPT.json` source, uninstall artifacts and explicit absence of
flight hooks. Expanded native paths and public API placeholders are accepted
only when they name the exact reviewed application and optional command.
Keep native Update stable; only an absent owned app is repairable. Recheck identity,
path and all running copies immediately before mutation. Unowned apps are reused
and cannot be adopted. Remove stays per-item approved, does not zap personal data,
and discloses that a same-signer manual reinstall cannot be distinguished from a
vendor update. Normal user permissions and Gatekeeper remain in force.


### Hosted acceptance journeys (2026-10-20)

The public keyboard lifecycle runs first in each existing core job, before any
other fixture can create Windows known-folder installer state. It requires an
absent state file, real PTY/ConPTY input, the public wrappers and exact approval
previews; machine requests are read-only observations. Preserve original failure
journals and terminal evidence as artifacts. Read-only ownership comparisons
exclude only the public compiler's `bootstrap` cache, which shares the Windows
state parent; all installer journals and managed payloads remain compared.

Agent acceptance discovers installed Sentinel skills through the actual Codex
and Claude local protocols, sends no model turn and uses isolated authentication
locations. Personal skills, later edits, native modes, approved repair evidence
and the explicit Impeccable cache must survive Remove/Reinstall. Code's native
extension-host witness must prove real edit/save/close/reopen as well as settings;
an old witness or an in-memory buffer lookup is insufficient.

The owner declined dedicated VM CI infrastructure on 2026-10-20. Do not add
self-hosted/reboot jobs or a default-disabled success gate. Actual guest reboots,
first-time permission grants and human visual usability remain separately dated
manual evidence. Hosted additions use the existing 23 native/quality contexts
and four CodeQL checks; do not weaken their assertions or Windows time budget.

The Windows public keyboard fixture starts with an absent whole state directory.
Only after public Remove/Check completes, with no selection, kept resources,
transaction, active receipt or deferred removal, it relocates its exclusively
created directory intact to a same-volume evidence sibling. Compare original
contents/modes and archive the retained journals; do not edit or delete them.
This lets the next existing public fixture keep its fresh-known-folder assertion.
Failed or unfinished journeys stay at their original path and fail the job.

Public machine inspection must decode stdout before interpreting its process
exit: `needs-action` and `failed` require exit 1; healthy responses require exit
0. Do not discard the deliberate damaged-pointer Check result. Native evidence
snapshots and tar archives must record Windows `ModeIrregular` junction targets
without following them; retain their kind in PAX metadata and continue through
the later journal files. Code acceptance opens its first document through the
editor command, avoiding `openTextDocument`'s retained reference before disposal.
Keep all saved-byte, new-document-identity and native-mode assertions.


Windows Terminal inspection may reuse parsed object/array offsets only within a
single extraction of unchanged input. Keys bind the exact slice start and length;
no unsafe arithmetic or global/driver cache is needed. Mutable writers retain the
uncached path and the same validation, JSONC bytes, projections and ownership
rules. Keep the frozen pre-optimization byte/error reference and allocation-budget
regression; do not extend a reader across an edit or subsequent inspection.

Native jobs intentionally continue some independent steps after earlier failure.
Reaching a later step proves neither an earlier step nor the whole job passed;
read each conclusion and require all final-head check conclusions to be success.
