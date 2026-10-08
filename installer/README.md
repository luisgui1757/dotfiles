# Native installer under development

**Unfinished; not a release.** The public setup/removal commands stay in place
until cutover. The new `cmd/dotfiles` executable connects the controller, terminal,
private archives, native configuration and scoped profile blocks. Starship, fzf, fd, lsd, zoxide, Neovim, ripgrep, GitHub CLI, lazygit, jq,
Tree-sitter, Hyperfine, Node 24/npm, ShellCheck, Taplo, gh-dash, Windows PowerShell, CMake, Python, Herdr, EditorConfig Checker, win32yank
and pinned shell plugin/module archives, plus Pi's standalone executable, are connected
to the archive provider, with a combined selected shell assembly. Full tool
capabilities still need their system dependencies and configuration recipes. Full zsh composition is
connected on macOS and Linux through their native shell providers. Other selections fail
before mutation when their provider is not connected. Bootstrap, the remaining tools, migration and review
corrections are still required.

Supported targets are macOS **arm64 only**, Ubuntu/Debian amd64/arm64 and native
Windows amd64. WSL follows the same Ubuntu/Debian lifecycle. Other Linux
distributions and Linuxbrew are outside this major release; discovery rejects
them before provider selection. fd and ripgrep are independent checkbox choices. Intel macOS is rejected before provider discovery or
archive selection; the same target contract validates archive metadata and plans.
macOS Git/tmux selections now connect to an existing Homebrew installation using
its observed paths and the native worker. The installed tmux/gh-dash application
lifecycle passes on a disposable macOS runner, including update, broken-link
repair and final removal. Fresh Homebrew bootstrap remains required.
Retaining a native Git/tmux package after deselection does not retain the shell
configuration added for that selection. Genuine runtime dependencies remain
protected by the graph.
Ubuntu/Debian now use APT for Git, zsh, tmux, Make, build-essential and clangd.
The provider records repository refresh and package mutations through the native
worker, attributes ownership to operation-specific native logs and removes exact
owned packages while protecting other consumers. Interactive authentication stays
in the foreground; machine requests require existing noninteractive privilege.
Incomplete worker records or truncated native history require inspection and do
not grant ownership or automatic replay. Local provider tests pass; native Ubuntu
and Debian lifecycle runs remain pending for this integration batch.
The installed-application native test requires both `GITHUB_ACTIONS=true` and
`DOTFILES_TEST_NATIVE_PACKAGES=1`: it deliberately removes the runner's tmux test
prerequisite to establish a real from-absent baseline. Never enable this on a
personal machine. Preview-only tests have a separate read-only opt-in.
Formula maintenance preserves affected external casks and verifies their declared
native dependencies. It reports those applications as not exercised; supported
managed applications still need their own runtime checks. A missing or inactive
dependency blocks completion and can be corrected before retrying the saved
operation. Casks never enter formula linkage/repair commands.

Pi reuses the private archive lifecycle with its bundled runtime/assets. Node/npm
and Git remain prerequisites for Pi package features; fd/ripgrep are provisioned
explicitly. No global npm prefix is modified. macOS offline RPC/TypeScript loading
passes without Node on PATH. Scoped theme settings are connected: explicit
adoption preserves the prior theme, removal restores it, and unrelated settings
bytes and later edits survive. The compiled application lifecycle passes locally
while reusing pre-existing Git. A real macOS PTY also verifies initial theme,
all three variants, switching, rendered text colors and ordinary exit. Real npm
and Git package install, extension loading, changed Git-commit update and removal
also pass locally using private Node and local transport fixtures. The installed
Pi lifecycle now passes on hosted macOS, and direct archive/RPC checks pass on all
four targets at `e1c987d`. The additional TUI/package tests still need their native
Windows/Linux runs; existing Git in a fixture is not Git provisioning proof.
A pre-existing theme preference may retain its theme assets after deselection;
it does not retain the private Pi executable or the executable's dependencies.

The [plan](../docs/plans/interactive-installer-overhaul.md),
[status inventory](../docs/installer-status.md) and append-only
[review ledger](../docs/reviews/2026-10-08-interactive-installer-overhaul.md) separate
architecture decisions, implemented behavior and exact verification evidence.
The never-released Go Nix prototype and its dedicated experiments have been
removed. The released setup still has its original Nix gate until full cutover.

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

First run opens tool checkboxes. Later runs offer selection, update, repair,
read-only check and removal. Dependency and shared-removal choices come from the
graph. A scrollable preview precedes a distinct default-Back confirmation.
The terminal is restored before providers need foreground input.

`dotfiles machine` accepts one bounded strict JSON Request. Omit `expected_plan`
to preview, then send the same request with that exact plan identity to execute.
Ordinary apply requires `selected`, including `[]` for deliberate deselection.
Missing/null selection never means remove everything. Maintenance and retry retain
recorded selection. `DOTFILES_CHECKOUT` currently supplies the trusted checkout;
the public entrypoints have not yet switched to the new bootstrap. Development
launchers in `scripts/installer-bootstrap.{sh,ps1}` verify a private pinned Go
toolchain, build from the checkout, and cache the binary by source identity. A
native macOS launch is verified; Windows/Linux execution and released-state
migration remain pending.

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
This includes Pi's actual interactive theme check in a POSIX PTY or Windows ConPTY.
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
boot-only CI probe is retired. Historical
failures, fixes and independent reviews are retained in the append-only review
ledger; current outstanding work belongs in the status inventory.

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
