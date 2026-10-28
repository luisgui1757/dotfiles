# Native installer

The public setup and migration entrypoints launch `cmd/dotfiles` through the
verified Go bootstrap. It connects categorized tool selection, native providers,
private pinned archives, configuration publication and scoped settings to the
same planner and recoverable executor. The [operator guide](../README.md)
lists the current capabilities and supported platform combinations.

Migration has two explicit machine phases: `DOTFILES_MIGRATION_PHASE=profiles`
keeps the released preparation protocol, while `runtime` uses a separate retirement
catalog, state and saved native outcomes. Interactive migration runs configuration
preparation, macOS Homebrew conversion, setup, Check, retirement and a final Check.
Runtime selection binds exact observed scope and the normal setup state. The
shared normal/native mutation locks protect the replacement check and removal
interval. Unknown native outcomes are never replayed; completed native results
are consulted before preconditions on recovery. Unchecked or unproved artifacts
prevent a complete migration result until the owner explicitly keeps them: when
every other runtime retirement finished, interactive migrate offers to keep each
remaining resource. The choice is recorded in `migration/retirement/kept.json`,
bound to the exact inspected scope; any change makes it unfinished again. Published `v0.4.4` and pre-overhaul main pin
evidence are checked against their immutable source commits.
Completed retirement records remain history rather than automatic selections:
reinstalled artifacts require another explicit choice. Linuxbrew discovery is
restricted to the retirement adapter; it never changes the normal APT provider.

Supported targets are macOS **arm64 only**, Ubuntu/Debian amd64/arm64 and native
Windows amd64. WSL follows the same Ubuntu/Debian lifecycle. Other Linux
distributions and Linuxbrew are outside this major release; discovery rejects
them before provider selection. Shell essentials groups fd, ripgrep and the other
shell tools while allowing individual choices. Intel macOS is rejected before provider discovery or
archive selection; the same target contract validates archive metadata and plans.
macOS Git/tmux selections use Homebrew through its observed paths and the native
worker. Verified bootstrap handles an absent Homebrew installation and requires
a new inventory preview before installing packages.
Retaining a native Git/tmux package after deselection does not retain the shell
configuration added for that selection. Genuine runtime dependencies remain
protected by the graph.
Ubuntu/Debian now use APT for Git, zsh, tmux, Make, build-essential and clangd.
The provider records repository refresh and package mutations through the native
worker, attributes ownership to operation-specific native logs and removes exact
owned packages while protecting other consumers. Interactive authentication stays
in the foreground; machine requests require existing noninteractive privilege.
Incomplete worker records or truncated native history require inspection and do
not grant ownership or automatic replay.
The installed-application native test requires both `GITHUB_ACTIONS=true` and
`DOTFILES_TEST_NATIVE_PACKAGES=1`: it deliberately removes the runner's tmux test
prerequisite to establish a real from-absent baseline. Never enable this on a
personal machine. Preview-only tests have a separate read-only opt-in.
Formula maintenance preserves affected external casks and verifies their declared
native dependencies. It reports those applications as not exercised; supported
managed applications still need their own runtime checks. A missing or inactive
dependency blocks completion and can be corrected before retrying the saved
operation. Casks never enter formula linkage/repair commands.

Claude Code and Codex reuse the private archive lifecycle as independent
capabilities. Git is resolved automatically; neither pulls in Node/npm, fd or a
second coding-agent checkbox. Codex's full package includes its native helpers.
Claude Code uses the signed-manifest stable native binary. Both native platform
packages are downloaded as compressed registry archives and extracted directly;
no npm installer or Node runtime is used. Package SRI and every Claude binary
hash were independently verified before recording the SHA-256 pins. The selected shell
sets `DISABLE_UPDATES=1` for Claude; the normal Update menu owns both CLI pins.
No personal authentication, model, permission or session settings are installed.
The native agent application test exercises offline CLI consumption, Codex
app-server initialization, unchanged update, repair and selective removal.

GitHub Copilot CLI uses the same lifecycle on every target with the standalone
single-executable assets of its official GitHub release (Linux requires glibc;
Windows adds the shared PowerShell 7 it needs). Copilot ignores its own
`autoUpdate` setting, so the selected shell sets `COPILOT_AUTO_UPDATE=false`;
no `~/.copilot` file is written. An invocation outside that environment can still
self-update in place; Check reports it and Update/Repair restores the pin.

Kiro CLI (macOS/Linux) uses the same lifecycle with Kiro's fixed-version stable
downloads: the macOS commands run from a private, unmodified copy of
`Kiro CLI.app`, and Linux uses the glibc ZIP without its vendor installer or
`~/.local/bin` aliases. `integration.kiro` owns only `app.disableAutoupdates` in
`${KIRO_HOME:-~/.kiro}/settings/cli.json`, so Kiro cannot replace the pinned
payload; Update applies new reviewed pins. The native agent test runs Copilot on
all four targets and Kiro on macOS/Linux without credentials. Kiro CLI is not
offered on Windows: Kiro ships it
there only as an MSI installer, and setup neither runs that installer nor extracts
programs from it (owner decision 2026-10-22). A plain Windows archive from Kiro
would make it addable like the other targets.

Agent CLI pins (Claude Code, Codex, Copilot CLI, Kiro CLI) are proposed weekly by
`.github/workflows/agent-cli-pins.yml` through `scripts/update-agent-cli-pins.py`,
which verifies vendor signatures, integrity hashes and layouts before changing
only version, URL and SHA-256; the pull request's native checks and the owner's
merge remain the acceptance boundary.

Sentinel rules and skills (`sentinel`) needs the hidden
`integration.sentinel-installer` resource, which needs `tool.git`. That resource
fetches the commit that `sentinel.go` pins into `<state>/sentinel/checkout`.
Git runs without system or global configuration. `HEAD` must equal the pin, and
each file must equal the tree of the pin, independently of Git index flags.
Check, update and removal run Sentinel only from such a checkout. Removal
fetches the pin again when necessary.

`<state>/sentinel/record.json` saves the
intent of each operation before it changes anything, and its completion after.
An interrupted operation resumes: the Sentinel installer runs again. The record
also keeps the hosts and the folder variables of the install.

Then the resource runs the Sentinel installer of that checkout. On macOS and
Linux it runs `bash tools/install --global --hosts <hosts> --skills core,ui`.
On Windows it runs `powershell.exe -NoProfile -ExecutionPolicy Bypass -File
tools\install.ps1 -Global -Hosts <hosts> -Skills core,ui`. The hosts are the
present agents among Codex, Claude Code, Copilot, Kiro and OpenCode. Check runs
the same installer with `--check`. Exit 0 is healthy, exit 1 is drift for
Repair, and any other exit is an error.

Removal runs `--remove` for each host that dotfiles installed for, then deletes
`<state>/sentinel`. The resources `integration.sentinel`,
`skills.sentinel-agents`, `skills.sentinel-claude`, `skills.sentinel-kiro` and
`skills.sentinel-copilot` stay in the catalog only for the v1.0.0 handoff. The
first apply deletes the skill folders that their receipts list, when a hash
check against the saved journal proves the copied bytes. Then it releases those
receipts. The [migration guide](../docs/UPGRADING.md#sentinel-rules-and-skills)
describes the handoff.

The never-released Go Nix prototype and its dedicated experiments have been
removed. Previous released installers and their pinned source evidence remain
available for recovery of their own unfinished transactions.

## Structure

- `resources.json` defines user-facing tools and hidden dependencies. The pure
  planner computes reachability and reverse removal order; UI code has no second
  dependency list. WSL2 uses Linux's graph. Session capabilities are observations.
- `Controller` connects interactive choices and the explicit machine protocol to
  the same planner/executor. A preview is read-only. Approval binds source,
  catalog, target, selection, state generation and observed artifacts.
- `ArchiveDriver` installs checksum-pinned HTTPS archives in private per-user
  generation directories named by the durable operation ID. Extraction bounds bytes/entries and rejects unsafe paths
  and links. Stable per-tool entrypoints use POSIX links or Windows junctions.
- `ConfigDriver` maps canonical top-level sources from `config-targets.json`.
  POSIX uses live links; Windows copies files and uses a Neovim directory junction.
  Actual Windows known folders are queried independently, never guessed from HOME.
- `ProfileDriver` owns marked shell bytes or explicitly named JSON properties.
  Shell profiles preserve outside edits, BOMs and line endings; strict JSON
  settings preserve unrelated bytes and prior property values. It stages with native metadata and publishes with no-replace moves.
  It refuses profile symlinks instead of modifying their referents. One combined shell resource owns initialization and actual profile attachments.
  Permission-refused staging identifies the source and destination, preserves
  the active original, and explains how to retry the saved operation.
- Git uses a prepended marked include in its regular XDG global configuration,
  preserving personal precedence and ordinary `git config --global` writes.
  For old whole-file receipts, the existing `remove_shared` per-item machine
  choice also authorizes legacy settings restoration: preserve current edits
  separately, restore the first baseline, then select the scoped replacement.
  Declining this choice never adds the retired mapping to Keep. Retired mappings
  cannot be kept, adopted or republished and have no runtime dependencies. An
  inherited Keep entry is omitted from the new plan without touching the file.
  A cancelled first publication can release uncertain bookkeeping only when its
  saved journal proves no publication started or verifies completed restoration.
  Intact recovery evidence is required; this grants no package removal authority.
  The old whole-file mapping remains for receipt recovery. Its `replaced_by`
  metadata releases only unowned historical bookkeeping after explicit removal;
  it never transfers deletion authority or discards an original backup.
- `NativeDriver` routes each resource to one provider. There is no fallback
  cascade and no Nix or chezmoi resource in the new graph.

Native platform and folder discovery happen at the process boundary. Linux
records its distribution family and actual system-shell libc before selecting
archives; glibc artifacts cannot be selected for musl or unknown libc. WSL keeps
the Linux graph. These facts stay in `NativePlatform`, outside persisted OS/architecture identity.
Musl distribution alternatives are outside the supported scope. Constructors with supplied
folder fixtures must not inherit caller profile paths, including ZDOTDIR. Native
tests validate every profile write target against their disposable home.

## Lifecycle and preservation

First run opens category checkboxes with indented tool choices. Category metadata
comes from the catalog and is applied only after platform/removal filtering. The
terminal returns individual tool IDs; categories never enter requests, receipts
or dependency resolution. The catalog keeps category members contiguous in a
fixed order; categorized catalogs require plain labels on every capability and
none on hidden resources. Older category-free catalogs remain valid. Space toggles
the full group, with `[-]` and a selected count indicating a partial selection;
individual children remain selectable. Later runs offer Choose tools (unticking
removes, an unchanged selection repairs), Update tools, a read-only Check and
Exit. Dependency and shared-removal choices come from the graph. One compact
confirmation, whose initial choice is Apply, shows the counts and highlights,
then the flat retention, adoption and shared-removal decisions as checkboxes;
toggling one revises the plan in place, and `d` opens the complete scrollable
plan. The terminal is restored before providers need foreground input, and the
apply's steps and native output stream to the screen and a per-run log.

Public entrypoints reuse a verified build without the toolchain while every file
under `installer/`, the launcher and the pin are unchanged; see CLAUDE.md.

`dotfiles machine` accepts one bounded strict JSON Request. Omit `expected_plan`
to preview, then send the same request with that exact plan identity to execute.
Ordinary apply requires `selected`, including `[]` for deliberate deselection.
Missing/null selection never means remove everything. Maintenance and retry retain
recorded selection. `DOTFILES_CHECKOUT` supplies the trusted checkout. Public
entrypoints call `scripts/installer-bootstrap.{sh,ps1}` to verify a private pinned
Go toolchain, build from the checkout and cache the binary by source identity.

Resources become owned only after a provider proves completion of the operation
recorded before mutation. Existing files, matching bytes or an inventory change
alone do not establish ownership. Removal deletes only owned, unchanged, unused
resources. Retained consumers retain dependencies. External/uncertain consumers
and general shared tools require explicit handling; personal data is preserved.

A configuration adoption is explicit and must be reversible. Updates retain the
first uninstall baseline. Explicitly replacing an edited owned configuration
saves the edits separately and reports those paths; they do not become deletion
authority. Configuration removal restores the independent baseline even if the
source is unavailable or an adopted overlay was deleted.

Provider journals precede publication. Interrupted work preserves original
operation identities and recovery data. Resume completes saved intent, then asks
for a fresh remaining-work preview. Configuration and profile restoration use saved
physical targets without the current source/catalog. Abandonment archives intent
without discarding receipts. Damaged owned archives offer an explicit replacement
choice; a fresh verified generation preserves and reports the old files. Pointer
repair reuses only an unchanged generation. Cleanup journals record individual
entries, keep user edits and resume after partial deletion or temporary locks.
Archive schema 2 supersedes an unreleased prototype, which is rejected locally
without changing its files.
Profile restoration reverses owned blocks while keeping later personal edits in
the active profile. Its inverse uses the same durable writer and can itself resume
after interruption. Uninspectable recovery stays retryable; attempting forward
resume cannot replace that restoration transaction. Corrupt active profile intent
discloses journal and recovery locations and refuses mutation.

Whole-provider inventory, where needed, binds protected inputs even for an empty
plan. The executor checks it before every mutation and at final verification.
Native providers also need their own serialization and publication checks.
Completed removal tombstones survive finalization failures, so retry retains
removal authority without claiming ownership of the restored baseline.

State is bounded strict schema-1 JSON. Unknown fields/schemas fail without changing
the input. Old unreleased Nix batch fields are deliberately unsupported. Missing
optional ownership proof remains uncertain. State publication is atomic and
flushes data; Windows uses `os.Root.Rename` so open preview readers retain their
complete old snapshot while a new one is published.

## Verification

Use Go from `go.mod`, then `make test-installer`. The full `make ci` gate includes
formatting, vet and race tests. Git fixture subprocesses strip inherited `GIT_*`
variables to protect the caller's index. Installer files and workflow YAML retain
LF on Windows checkouts.

The hosted engine matrix runs macOS arm64, Windows amd64, Linux amd64 and Linux
arm64, and checks the actual runner target. Its
separate archive step downloads and executes pinned commands, including Neovim
runtime discovery and the compiled command's shared Starship/Herdr machine lifecycle, including selective removal. Linux
runners explicitly provision zsh for the shell tests; that is a test prerequisite,
not proof that the new installer provisions zsh. Run that group locally with
`DOTFILES_TEST_ARCHIVES=1 go -C installer test -v -count=1 -run '^TestNativeArchive' ./...`.
The separate agent application step exercises both selected CLIs without credentials.
Public Windows command tests additionally require explicitly opted-in disposable
GitHub accounts because known folders cannot be redirected through HOME.

Core tests cover graph combinations, stale approvals, shared removal, real-file
ownership, recovery and preservation. Archive tests cover extraction, changed-pin
updates, entrypoint repair and process death at selected boundaries. Profile tests
cover encoding, outside edits and saved publication shapes. Config tests cover
native no-replace publication, adoption/restoration and Windows junctions.
These are distinct from a complete installed-product matrix.

Terminal tests use real PTY/ConPTY sessions, then verify ordinary input still works.
They exercise selection, preview, confirmation, cancellation, resize and terminal
restoration. They do not prove package installation or privilege handling.

The published checkpoint `e1c987d` passes the native matrix on all four
OS/architecture targets, including actual changed-version update and restricted-
token Windows permission tests. WSL uses the Linux lifecycle; its former separate
boot-only CI probe is retired.

`DOTFILES_TEST_NATIVE_PACKAGES=1` additionally opts a disposable Linux host into
an APT contract test. It builds uniquely named local fixture packages without
maintainer scripts, verifies operation-local installation history, exercises a
shared dependency with an outside consumer, and confirms exact removal leaves an
unrelated pre-existing orphan untouched. The test checks the remaining native
package database is unchanged and removes its fixtures. Never enable this suite
on an everyday workstation. It proves the manager interface, not the complete
production provider.

Shell integration has one shared owner (`integration.shells`) for personal
profiles. Selected fragments are assembled in a fixed order into one private initialization
file under the installer state folder. Selection changes replace its managed block
while preserving outside edits; retained files never act as selection flags. PATH setup also works in quiet
noninteractive login shells. PowerShell initialization is generated locally from
the pinned executable, parsed, cached per archive recipe and dot-sourced. Runtime
cache files remain application data; removal does not claim ownership of edits
inside them. Full tool/provider coverage remains in progress.

Multiplexers are Herdr on all three OSes and tmux on macOS/Linux (including WSL).
Windows tmux and its plugin/configuration paths are absent from the graph. LazyGit
uses one defaults-only configuration at each OS's native location. The tmux Rose
Pine renderer is a maintainer-only generator; deployed configs never need PowerShell.
The new tmux integration owns the four checksum-pinned functional plugins directly
and loads them through a managed `~/.tmux.plugins.conf`; it needs no TPM or Git
checkout. Its loader activates session saving/restoration without managing login
services. Removal preserves snapshots and existing sessions. Native tmux package
provisioning remains separate work. Exact reviewed source corrections repair
upstream path quoting, and exact non-runtime exclusions omit dangling test-only
submodule links; both are part of the approved archive recipe identity.

The new dashboard command is `gh-dash`, using its pinned upstream executable.
GitHub CLI remains a dependency for `gh auth login` and dashboard actions;
installation itself needs no login or extension registration. Existing GitHub
extensions are outside the new installer's ownership. The released setup's
`gh dash` entrypoint remains documented until the migration/cutover is complete.

Python archives precompile standard-library bytecode at optimization levels 0/1/2
with checked hashes before recording ownership. Ordinary imports and venv creation
must leave the installed fingerprint unchanged; changed source or bytecode still
requires explicit replacement approval. Compiler processes use relative stdlib paths
under their working directory so abandoned workers cannot target a reused stage.

Native contract verification now includes Homebrew on disposable macOS runners.
APT attribution checks operation-specific history against actual dpkg install and
configuration records. A refused dpkg removal can leave an installed package marked
for removal; restore only that package's original selection after verifying its
version/state, and still report the refusal. Homebrew formula attribution uses the
operation-specific install badge emitted by its successful installer, followed by
native receipt checks; inventory differences alone do not establish ownership.
Neither contract is yet a connected production system-package driver.

Native package commands use a private mode of the same executable. Its worker
holds the provider lock until the actual command exits, including after controller
death. It accepts no work before its locked handshake. Output is bounded while
excess bytes are drained, preserving command lifetime; excess output still fails.
This guard is under native verification before production provider connection.
Mutating commands persist their intent and bounded result, with a structured
exit code, before returning. A restarted controller can recover completed output
without executing twice; an unfinished command record requires native evidence.
Archive extraction observes the destination filesystem's case sensitivity; Linux
Python terminfo names differing by case must remain distinct where supported.

Native dependency cleanup narrows one shared, attributable pool against current
installed consumers. It protects selected roots, manual promotions, held packages
and external consumers, propagating protection through dependency chains. Actual
manager removal must still reject newly appearing consumers. Homebrew inventory
includes installed cask consumers and actual keg runtime dependencies; older
receipts without that field use Homebrew's runtime query, without `--direct`
(which would switch to declared formula dependencies). Production connection is
still in progress. Read-only host inspection can be exercised separately with
`DOTFILES_TEST_NATIVE_INVENTORY=1 go test -run '^TestNativeBrewInventoryReadOnly$'`.

The Homebrew formula adapter implements per-resource install/update/repair and
exact removal, with the shared pool and durable command evidence. It validates
saved commands against the trusted recipe, retains pre-existing packages, and
seals completion only after native activation and ledger publication. Its real
disposable-host lifecycle includes a lost response followed by worker restart.
This adapter is under verification and is not yet routed from the public CLI;
provider-wide approval/guard wiring remains required before that connection.

### Current native prerequisite integration

The engine now connects fresh retained Homebrew bootstrap (with a new inventory
preview), Linux APT, the Windows Microsoft VC++ runtime, and Neovim private runtime
synchronization. Neovim uses its observed XDG data root independently of config,
with the Windows `nvim-data` suffix. Its clean headless check cannot install tools.
Actual macOS plugin/parser/Mason install/check/remove passed with existing native
compiler prerequisites; full supported-platform prerequisite acceptance is still
required. Homebrew updates remain selected-formula operations, not global manager
migration. The dedicated fresh-prefix runner and Windows vendor fixtures provide
separate proof from ordinary unit tests and existing-manager reuse.


### Public keyboard acceptance

The first core step on each disposable GitHub runner executes
`DOTFILES_TEST_PUBLIC_UI=1 go test -v -count=1 -timeout 25m -run '^TestNativePublicInteractiveLifecycle$' ./terminal`,
with `DOTFILES_TEST_UI_EVIDENCE` set to a fresh absolute evidence directory. It
refuses existing installer state. Real public wrappers, catalog, keyboard approval
and ripgrep consumption are followed through the whole maintenance lifecycle.
The retained tar excludes the compiler bootstrap cache, and includes original
installer journals/payloads even on failure. Never run this on a personal host.

The Code desktop witness uses editor APIs, saves actual bytes, waits
for document disposal and reopens from disk; it also checks native file modes.
These hosted checks do not certify actual OS reboot or first-time permission
grants. The owner declined dedicated VM CI on 2026-10-20.
