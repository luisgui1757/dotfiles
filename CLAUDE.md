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
Reject other distribution IDs before provider discovery; ID_LIKE does not widen
support to derivatives. APT is the only Linux native provider.
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
The current setup/migrate wrappers use the verified Go bootstrap. Delivery remains
in progress; [installer status](docs/installer-status.md) is the current evidence
ledger, not a provider-route count. Retired implementation rules are preserved in
[the previous guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/CLAUDE.md).
Configuration outcomes and personal data remain requirements after runtime removal.

The new gh-dash capability runs its verified upstream binary as `gh-dash`.
It still requires GitHub CLI for authentication and configured actions, but has
no separate extension registration or authenticated install step. Preserve the
user's existing extensions; released `gh dash` migration is handled at cutover.
The new Pi graph has no npm-prefix integration. Its package provider must use a
private install location without rewriting personal npm configuration.

Native package contract tests require explicit `DOTFILES_TEST_NATIVE_PACKAGES=1`
on a disposable runner. They intentionally exercise the real package database;
ordinary local gates must not enable them. Attribute dependencies to the actual
operation-specific manager record, never an inventory difference or timestamp.
Exact removal must protect consumers at execution and leave unrelated orphans.
Homebrew formula removal uses `uninstall --formula --force` on the exact owned,
unheld set: that flag removes every installed version but retains native outside
dependency checks. `--ignore-dependencies`, cask `--force`, and autoremove are
forbidden. Recheck pins before dispatch because formula `--force` itself unpins.
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
followed by selection and native compiler/SDK verification. Never reinstall
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
Windows uses full PortableGit (Bash, SSH, LFS, credential manager and GUI payloads)
and GNU Make. Make's approval scope comes from its actual provider: the Windows
archive is private/user; APT and Apple observations remain machine-scoped. PortableGit's fixed preparation runs only inside the private payload;
its upstream SHA-256 is not a Microsoft Authenticode claim. Do not expose Git's
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
Outside edits do not invalidate block ownership and must survive removal. Keep
encoding/BOM and line endings intact, preserve native metadata when staging, and
never follow an existing profile symlink into an unapproved source. Saved profile
intent precedes publication; an identical target with the staged file still
present does not prove ownership. Native Windows public-command tests run only on
explicitly enabled disposable GitHub accounts; HOME cannot substitute for known
folder discovery. The public command is not release-ready until all providers, old-runtime
retirement and public migration acceptance are complete.

The checkout bootstrap verifies the pinned Go archive before each extraction,
extracts into private staging and keeps only the compressed toolchain archive as
its trusted reusable input. A cached compiler's version output is not an integrity
check. Go runs with personal settings/workspaces and external cache programs
disabled; source identity includes runtime and embedded inputs, not test files.
Released configuration-target inventory is migration evidence, never removal
authority on its own.
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

Neovim synchronizes locked plugins, parsers and Mason tools into private runtime
generations under its independently observed XDG data directory (nvim-data on
Windows). Reuse the archive publication/cleanup primitives; preserve personal
Shada, undo, sessions and legacy data. Check uses a clean headless runtime and
never invokes Lazy/Mason installation. Keep large runtime manifests in fixed
4096-entry chunks, bound by the operation journal; missing or changed chunks
block publication/removal. Pass managed tool paths explicitly, never inherit a
caller PATH into construction. Windows compiler environment is acquired after
toolchain installation, not during pure construction. Linux Mason download/ZIP
prerequisites are hidden APT dependencies, proved absent before fresh-container
Neovim installation. Disable the optional Lua bytecode cache on normal startup
as well as staging: its path-encoded filename can exceed filesystem limits.
Plugin lazy loading and its startup budget remain required. The locked Obsidian plugin
is always provisioned and verified, including with a private sync HOME. Only its
configuration activation depends on an existing vault; never create a vault or
leave first real use to install an unmanaged plugin.

Visual Studio inventory is a counted bag of id, version, chip, language, branch,
type and extension tuples. The vendor returns an array without a uniqueness
guarantee. Preserve repeated rows and ordinal distinctions; compare multiplicity
before mutation. Canonicalize order at the Go boundary. Schema-1 package-map
journals lack multiplicity and require explicit recovery without reinterpretation.

Build Tools inspection runs its real compiler/SDK probe inside a private Windows
Job Object. Create the query suspended with atomic job-list assignment, then
resume it and verify all
owned descendants have exited before returning; compiler telemetry must not turn
the next observation into a new external consumer. Keep the 30-second inspection
and output bounds, with a separate five-second termination bound. Preserve
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
`14.51.36247.0` and `14.51.36247` identify the same revision. Distinct nonzero
revisions or bundle/servicing disagreements remain failures.

Discover `ZDOTDIR` only at the process boundary and pass it in `ConfigFolders`.
The application constructor must never inherit profile destinations from its
caller's environment. Every native fixture validates all write targets against
its disposable home before creating any profile.

Adoption requires a provider capable of restoring its baseline and a separately
approved resource choice. Do not infer ownership from matching pre-existing bytes.
An interrupted composite resource uses its saved operation and an observation-bound
recovery token, then requires a fresh remaining-work preview. A retry must not
silently replace the saved payload, operation ID or original adoption baseline.

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
  to kill-whole-line or set an unusably short KEYTIMEOUT.
- PowerShell enables PSReadLine vi mode before binding keys. Its PATH setup works
  for noninteractive invocations, but interactive setup must reject batch,
  encoded/stdin, redirected, CI and unsupported-host invocation before creating
  interactive caches. Host name/UserInteractive alone is insufficient.
- tmux uses Ctrl-B, one-based windows/panes, lowercase pane navigation and
  uppercase H/L window swaps. Use the four pinned plugins directly; never invoke
  continuum's auto-start handler that can remove personal startup services.
- LazyGit keeps upstream keybindings on every OS. Windows uses Herdr; POSIX offers
  Herdr and tmux. Do not restore psmux-specific key remaps or WSL host detection.
- Starship git-status styles interpolate `($style)`. Pi gets only its audited
  themes/keybindings and scoped theme choice; credentials, sessions and providers
  stay local. Sentinel is the agent-policy product name.

## Before changing this repository

Read this guide and the current status/review ledger. Preserve unrelated dirty
worktrees. Keep policy in the pure graph and IO in concrete providers; add an
abstraction only for present duplication or a demonstrated product requirement.
Update the relevant Markdown with every behavior change and append findings to
`docs/reviews/2026-10-08-interactive-installer-overhaul.md` without erasing history.

Run `make ci` (or `./test.ps1` on Windows) for a baseline and after integration.
Focused tests are appropriate within a cohesive batch. A bug fix needs a real
failed-before regression and a passing correction. Keep the highest practical
proof: process-boundary tests, then actual configured application/native lifecycle
on explicitly opted-in disposable runners. Cross-compilation is not native proof.
Do not run native package/font mutation on the everyday developer workstation.

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
proof is immutable. This overhaul is one reviewed commit and one PR. Merge and
release publication remain owner actions unless explicitly delegated.

## Engine and delivery invariants

The installer implementation under `installer/` is in progress. Go source uses
`gofmt` (tabs); its `.editorconfig` override must agree with the formatter.
Raw Go string contents also pass EditorConfig: indent embedded Ruby/C/JSON with
tabs where indentation is semantically immaterial; gofmt does not fix literals.
`installer/**` and its workflow require LF in `.gitattributes`: a Windows
`core.autocrlf=true` checkout otherwise fails gofmt before tests execute. The
Git checkout regression guards Go source, module files and embedded JSON bytes.
The capability manifest is `installer/resources.json`; keep dependency/removal
policy in the pure planner and platform IO in adapters. Do not add a second
dependency list to the interactive frontend. Until the public entrypoints and
required verification are complete, the new package is not the supported setup
workflow. The requested implementation delivery is one commit and one PR.

Native Git/tmux packages do not depend on installer-owned shell configuration;
their selected configuration resources do. Retaining a pre-existing or shared
native executable after deselection must not retain the newly added profile
block. Retained applications still protect genuine runtime prerequisites; do not
solve configuration leakage by disabling the planner's retained-resource closure.

Pi's CLI uses the upstream checksum-pinned standalone archive, including its
runtime assets and native helpers. Do not introduce a second npm installation
engine or mutate the global npm prefix. Keep Node/npm and Git for Pi's package
features, and provision fd/ripgrep rather than relying on application-triggered
downloads. Offline native acceptance loads a TypeScript extension through the
real RPC protocol with an empty PATH and no credentials; version output alone
does not prove the bundled runtime works.
Pi's passive config files do not require its executable, and the Go theme-field
publisher does not require Node. A retained personal theme selection protects
its theme assets without retaining the removed private CLI or its runtime tools.

Pi's strict UTF-8 JSON theme field shares the profile publisher and recovery
journal. Its optional `fields` metadata is part of target identity; absent fields
retain the existing shell-block interpretation. Preserve unrelated settings bytes
and exact large numbers. Explicit adoption saves the original field value; removal
restores it while retaining later unrelated edits. Duplicate keys, malformed JSON,
redirected paths and changed ownership scopes fail without destructive repair.

Save the publication journal before acquiring Pi's `settings.json.lock` directory.
Publish an operation-bound ownership marker atomically with that directory;
resume may reacquire only the same saved lock. Never reap another application's
lock based on age. The nonempty marker prevents stale-lock removal from stealing
an interrupted installer operation. Release the public lock by renaming it before
cleanup, so cleanup cannot delete a new application lock. Finalization and approved
abandonment release recorded installer locks, including before first publication.
Unknown lock contents remain preserved. Actual Pi TUI theme rendering is separate
from offline RPC extension proof; RPC does not support theme switching. The native
terminal fixture loads all three canonical variants in PTY/ConPTY, switches each
through Pi's real UI API, checks rendered color escapes, and exits through Ctrl+D.

The Nix prototype and old monolithic installation/runtime paths are removed.
The [test retirement map](docs/installer-test-retirement.md) identifies replacement
proof and deliberately removed behavior. Prototype batch JSON was never released;
reject and preserve it instead of inventing a runtime migration for it.

The [interactive installer plan](docs/plans/interactive-installer-overhaul.md),
[status inventory](docs/installer-status.md) and append-only
[review ledger](docs/reviews/2026-10-08-interactive-installer-overhaul.md) record
current decisions, exact evidence and unresolved findings. PR #87 contains the
single implementation commit; PR #86 is superseded. No component checkpoint is
a completed release. Keep these records and ROADMAP.md synchronized with changes.

The contributor gate requires the Go version in installer/go.mod and native race
tests. Core fixtures do not prove package installation. Hosted native lifecycle,
Go CodeQL and the trusted-checkout bootstrap remain separate verification claims.

First run opens capability checkboxes. Shared-removal choices come from the graph;
unchecked shared resources become Keep roots. Every mutation requires a complete
preview and a distinct default-Back confirmation. Release the terminal before
provider prompts. Preview holds existing engine/provider guards without creating
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
the runner; provisioned test prerequisites are not installer-bootstrap evidence.
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
Homebrew intent schema 3 saves the pre-existing native inventory, including
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


Linux clipboard dependencies are fixed: xclip plus wl-clipboard. Never change the
installation graph merely because setup ran over SSH or before a desktop session.
Observe session capabilities only for honest runtime-verification disclosures.
Neovim warnings require both Wayland commands and an active Wayland display;
X11 helpers require DISPLAY, and tmux transport requires a running session.
tmux copy bindings test session availability in increasing preference order so
an installed Wayland helper cannot capture headless/X11 copies. Preserve existing
bridges without a WSL detector or Windows-host installer.


## Scoped settings and migration

VS Code owns exactly six top-level theme/font fields in UTF-8 JSONC. Mask comments
and trailing commas only for parsing; retain original offsets and unrelated bytes.
Malformed/duplicate properties fail before mutation. A comment added to a newly
created settings document prevents deleting that document on removal. Pi JSON
remains strict. Reuse ProfileDriver receipts and publication for both.

Sentinel policy is generated from pinned upstream source into
`installer/sentinel/policy.md`; never hand-edit it. `provenance.json` records the
source and renderer inputs; `TestSentinelPinnedUpstreamRendererMatchesBundledPolicy`
is its opt-in reproduction check. There is no runtime Git/Bash installer.
Discover CODEX_HOME, XDG_CONFIG_HOME and PI_CODING_AGENT_DIR only at the process
boundary. OpenCode defaults to ~/.config even on Windows. Preserve surrounding
prose and prior adopted blocks. Sentinel journal fragments can lack a UTF-16 BOM;
only recognized marker-prefixed fragments get that bounded decoding exception.

Migration uses the existing controller and ConfigDriver file-copy publication.
A read-only preview never creates dynamic source payloads. Whole-profile detachment
requires per-item adoption, preserves the original artifact and readable bytes,
and never copies unknown personal shell commands into fresh profiles. Completed
migration is historical evidence, not permanent ownership; it cannot replay over
new setup blocks. Preserve passive released link targets until users migrate.

Yamllint and LaTeX conversion share only the real private-venv preparation steps;
fixed package recipes and complete Python-pin identities remain separate. Native
package install/remove is forbidden on the developer workstation; explicitly
opted-in disposable runners are the acceptance boundary.


Hidden Linux desktop libraries use APT package-file verification, not invented
executable probes. `dpkg --verify` stdout is a failure even with exit status zero;
real native GUI tests separately establish ABI and resource compatibility.


Ghostty uses one reviewed Linux DEB per architecture on Ubuntu/Debian. Its fixed
archive preparation supplies the bundled layer-shell library to both CLI and
GUI through the declared command wrapper; retain usr/share resource layout.
Do not introduce distro-specific recipes without evidence that the common one
fails. Desktop APT library names target Debian 13 / Ubuntu 26.04 native acceptance;
older releases are not verified by these jobs.

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
a PID/version alone is not configured behavior. AeroSpace waits for Accessibility
consent before config/server startup. Without consent, report launch separately
from unverified configuration/tiling; never bypass TCC to make a test green.

Run the opted-in Windows public migration fixture last in the core job. It may
reuse only absent or empty-ready setup state; it refuses active ownership or prior
migration evidence. Released CurrentHost Microsoft.PowerShell_profile.ps1 and new
AllHosts profile.ps1 are separate paths. Historical detachment cannot authorize
replacing either later personal content or active scoped setup blocks.


Read-only font inspection parses stdout only. Encoded PowerShell commands can emit
CLIXML module-loading progress on stderr even when they succeed. Keep diagnostic
stderr separate, preserve failed exit codes with bounded sanitized diagnostics,
and enforce the combined 1 MiB inspection limit. Never repair malformed protocol
output by trimming a CLIXML prefix or treating a failed native query as absence.


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
The full Go race suite has a bounded 20-minute budget: Windows' real subprocess
fixtures exceeded Go's default ten-minute total while continuing to make progress.


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
