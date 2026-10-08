# Interactive installer and removal overhaul

Status: **DESIGN RECONCILED; production rollout NOT IMPLEMENTED.**
Date: 2026-10-08. Reviewed source: `ae9a6446eb0a837a144d77d2a6345db967c61e4e`.
The current setup and uninstall commands remain authoritative until the rollout
below passes. This document is the implementation plan, not an install guide.
See the [review dispositions](../reviews/2026-10-08-interactive-installer-overhaul.md)
for evidence, corrections, and unverified candidates.

This delivery is the reconciled plan in one commit and one PR. A separate
[experimental draft](https://github.com/luisgui1757/dotfiles/pull/87), observed at
[`44308da`](https://github.com/luisgui1757/dotfiles/commit/44308da9a9d62afca101a0366fb7b229ce97da75), contains
initial planner/state work; it has no production adapters or usable installer
and does not satisfy the gates below. Its existence is not native lifecycle
evidence or acceptance of the proposed architecture.

## Outcome and decisions

Users choose the tools they want in one interactive CLI. The installer resolves
and explains prerequisites, installs the selected configuration, and later
removes only resources no remaining selection needs and that it can prove it
owns. Selecting Neovim on Windows therefore includes a suitable C/C++ toolchain
when missing; Visual Studio Build Tools is not a separate checkbox.

The implementation will use one capability manifest, a pure dependency planner,
a durable ownership ledger, and platform adapters extracted from the existing
scripts. Dependency resolution and removal policy belong in the planner; the
interactive interface and unattended interface invoke that same planner.
Do not wrap the current install-everything path with a checkbox screen.

Decisions:

- Use surgical reconciliation, not uninstall-everything followed by reinstall.
  Shared dependencies remain while reachable from any selected feature.
- Keep the human workflow entirely in menus. Keep an explicit, versioned machine
  request/result interface for CI and recovery; unattended execution never
  infers consent or selects everything because a stream is redirected.
- Preserve existing providers, pins, exact-release migration, config ownership,
  Windows known-folder handling, and POSIX live symlinks. Extract working
  mechanisms before replacing orchestration.
- Use a prebuilt Go CLI as the intended delivery vehicle. A distribution and
  terminal prototype is a mandatory feasibility gate before the engine is
  committed to it. Changing that choice requires updating this plan with the
  measured reason. Do not introduce Node solely to launch the installer or
  require an on-device Go compiler. An `npx` wrapper is unnecessary for v1.
- A binary is the launcher/engine, not a replacement for the configuration
  checkout. Bind it to an exact supported manifest and source revision.
- Keep feature selection, receipts, and recovery state outside the checkout.
  No machine state may dirty an immutable release tree.
- Keep recovery limited to operation intent, verified receipts and replanning
  from observed state. Reuse the existing local transactions for composite
  edits; do not build a general transaction framework or promise rollback of
  arbitrary package-manager operations.
- Treat package managers as retained infrastructure, clearly reported. Removing
  the final feature does not implicitly remove Homebrew, Nix, Scoop, or another
  manager and everything else it may own.

## Human workflow

The existing platform launchers acquire the verified CLI and enter its menu.
After installation the same CLI offers **Change installed tools**, **Update**,
**Check / repair**, **Remove tools**, and **Exit**. First run opens selection.
The documentation must show the actual shipped launcher command; the binary
name and release asset names are settled by the distribution prototype.

Selections describe useful tools, with platform availability and a short
description. Presets only preselect checkboxes; users can inspect and change
every root. No hidden mandatory full-stack preset. Existing installations open
an adoption review instead of pretending their selections are known.

Illustrative Windows selection and plan:

```text
Choose your tools                         Space: toggle   Enter: review
  [x] Neovim       Editor and bundled language tools
  [ ] Pi          Terminal coding agent
  [ ] WezTerm     Terminal with the dotfiles configuration

Neovim needs:
  Neovim + config, locked plugins, parsers and language tools
  Node.js                      reuse verified existing installation
  C/C++ compiler + Windows SDK  install VS Build Tools components
  Other required runtimes      expanded in the complete dependency list

Review: added / changed / removed / retained / needs your action
Apply changes   Back   Cancel
```

The real preview lists the entire resolved plan, not only these examples. Show
why each dependency is needed, its provider, whether it already exists, privilege
requirements, download/disk estimates where available, and any restart or login
step. Do not invent an exact estimate when the provider cannot supply one.
Cancellation before Apply makes no managed changes. Read-only discovery must
not bootstrap a package manager, create config parents, or authenticate users.

Removal uses the same selection and preview. If Neovim and Pi share managed
Node.js, removing Neovim retains Node.js for Pi. Removing the last consumer makes
Node.js eligible for removal only if ownership and external-use checks permit.
Pre-existing tools, user-modified settings, personal files, and retained
infrastructure appear explicitly in the result with reasons.

Authentication, OS permissions, unavailable host-side requirements, and reboots
produce **needs action** with a resumable next step. They do not count as ready.
Failed mandatory operations produce a failed result. Update is explicit and
preserves selection; repair reconciles selected resources without broad upgrades.

The terminal must support keyboard-only operation, readable non-color status,
small windows, plain logs, and screen-reader-friendly prompts. Suspend the UI
and hand the terminal to sudo or other interactive OS tools; resume afterwards.
On Windows, journal before elevation or replacing a running PowerShell host.
EOF, cancellation, and interrupted elevation are distinct from confirmation.
A numbered checkbox prompt is an acceptable accessible input mode; a particular
TUI library is not an acceptance criterion. Prove the intended input behavior
through Windows ConPTY and psmux before choosing a framework. The selector and
preview must be available before activating Nix or another package provider.

## Capability and dependency inventory

These are the initial selection boundaries. Separate IDs in a row remain
separate checkboxes; grouping here does not create an inseparable bundle.
Unavailable capabilities are explained and cannot be selected. Existing WSL
host/guest separation applies to every entry.

| User selections | Owned behavior and dependency boundaries |
|---|---|
| Neovim | Native Neovim and tree-sitter CLI, canonical config, locked Lazy plugins, parsers, Mason tools, and their runtime/build prerequisites. Keep the bundled language configuration together in v1. Linux clangd retains its Nix provider. |
| VS Code | Editor, Rose Pine extension and scoped JSONC settings. Preserve unrelated settings and restore previous touched values when safe. `jq` is not intrinsically a product dependency; the current helper has another parsing path. |
| Ghostty; WezTerm; Windows Terminal | Each terminal and its configuration are independently selectable on supported platforms. Configured fonts and shell requirements are dependencies. Ghostty's GNOME/X11 maximize helper is a platform-conditioned integration. Windows Terminal keeps its dedicated multi-target merge transaction. |
| Herdr | Native/vendor application and platform config. Windows config currently launches `pwsh.exe`; resolve that requirement. |
| tmux / psmux | Platform implementation, shared generated status bar, plugins and required shell. Native Windows psmux currently requires PowerShell 7 through its config. |
| zsh; PowerShell 7 | Shell and profile, plugin/module resources and their real prerequisites. Login-shell changes are a separately explained choice inside shell setup, not an automatic global side effect. |
| Starship; lsd; zoxide; fzf; ripgrep; fd | Individually selectable user-facing prompt/navigation/search tools. ripgrep and fd can also be automatic Neovim prerequisites without selecting their roots. Activate shared shell integrations only for selected capabilities, even when unselected binaries happen to exist on PATH. |
| AeroSpace (macOS only) | Vendor cask and canonical config; its trusted Homebrew tap remains retained infrastructure. Accessibility permission is a user-controlled OS grant: report needs action until usable and disclose retained grants during removal. Keep ownership-gated cask removal separate from nix-darwin's `cleanup = "none"`. |
| Git defaults; lazygit; GitHub CLI; gh-dash | Independently useful roots. Git defaults own only the repo's documented integration, never the whole user gitconfig. lazygit requires Git; gh-dash requires GitHub CLI and reports authentication separately. |
| Pi | npm package, theme/keybinding files and scoped theme-setting merge; Node/npm and cleanup runtime ordering are automatic. Do not touch sessions, credentials or provider preferences. |
| Sentinel agent policy | Pinned upstream installation, supported consumers and owned policy blocks. Prove scoped removal against the pinned upstream contract before shipping its lifecycle. |
| WSL host support (Windows only) | Explicitly retained host clipboard/font support for WSL guests. Terminal applications remain separate selections. Guest setup reports missing host requirements and the matching host action; it never installs across the boundary. |
| Repository development checks | An explicit contributor capability/preset for test-only tools. Do not install shellcheck, hyperfine, or other test tooling merely because a user selected an unrelated application. |

The inventory must map **every** current installer operation, config target,
chezmoi run script, external checkout and side effect to a root, prerequisite,
integration, retained infrastructure resource, or explicitly retired behavior.
Unmapped operations block completion of stage 1. In particular:

- A terminal's required shell executable is distinct from the selectable
  dotfiles shell profile and optional enhancements. Installing PowerShell to
  launch WezTerm must not implicitly enable an unselected profile bundle.
- Font resources provide the configured appearance; distinguish an exact
  configured font requirement from a program's ability to launch. Reuse a
  valid existing font without claiming ownership.
- Clipboard support is platform/context-specific: macOS, Wayland, X11, WSL
  host bridge, or an explicit working custom provider. Do not hardcode one
  provider when the config supports alternatives, or require a GUI on headless
  hosts. Check host-side requirements without writing across the WSL boundary.
- Neovim needs a complete Rust toolchain for its configured behavior: inventory
  `rustc`, Cargo, `rustfmt`, clippy and standard-library sources (`rust-src`).
  The config references `rustfmt` and clippy, and existing language smoke tests
  expect `rustfmt`; no installer route was found in the reviewed sources.
  [rust-analyzer's installation guide](https://rust-analyzer.github.io/book/installation.html)
  requires standard-library sources and describes its automatic install attempt.
  Reproduce and close the bootstrap gap before calling the bundled language
  stack complete. Do not silently label enabled language functionality optional.
  Future language checkboxes require matching configuration selection and their
  own lifecycle proof.
- Mason, npm, and plugin managers do not necessarily provision all external
  runtimes their artifacts need. Declare those edges in our manifest; leave
  package-manager-internal library dependency resolution to that manager.
- Capture PATH additions/reordering, npm prefix, login shell, `/etc/shells`,
  shell startup guards, notes-vault configuration, font registration, desktop
  autostart, VS Code settings and OS grants. Notes themselves are personal data.
  Classify irreversible or user-controlled OS grants honestly in the preview.

## Planner, state, and execution contract

### One graph, provider adapters

Each resource has a stable ID, applicable platforms/architectures, required
resource IDs, provider binding, probe, apply, verify and removal policy. Track
version/ABI constraints where they matter. Dependency edges express necessity;
ordering constraints express sequencing, including cleanup that still needs a
runtime. Conditional integrations and external-host requirements need explicit
metadata, not a generic new package solver or a second hidden graph in scripts.

Given selected roots `S` and platform context `P`, the planner computes their
transitive resource closure `R(S, P)`. Validate unknown IDs, missing providers,
cycles and incompatible requirements before mutation. Compare desired resources
with observations and receipts to build a deterministic operation plan.
Recompute reachability on every change; do not store mutable reference counts
as the source of truth.

Provider selection respects existing provenance and acceptable installed
versions. Probe the command the application will actually resolve, not merely a
package database entry. A matching pre-existing full Visual Studio toolchain
can satisfy Neovim without installing another Build Tools instance. Preserve
native Neovim/tree-sitter ABI boundaries and update ownership already enforced
by the repository. No globally selected package manager overrides those rules.
For Windows compiler verification, detecting `VC.Tools.x86.x64` is only an
initial observation: compile and link a small C fixture in the actual developer
environment, then build/load a parser. Prove SDK/UCRT and architecture usability
on a clean machine as well as reuse of an existing instance. Record added
instances/components separately; matching a path does not confer ownership.

Adapters reuse the current download checksums, staging/publish helpers, known
folders, merge transactions and provider commands. They accept explicit resource
operations, never ask feature-selection questions, and return structured
outcomes. Narrow privilege elevation to the necessary operation; the UI and
whole planner should not run as administrator by default.

### Durable ledger

Use a versioned schema in the platform's per-user application-state location,
outside source and managed config destinations. Persist:

- Selected roots and platform context, including WSL GUI intent.
- Explicitly kept dependency resources, so a user can retain a useful shared
  tool without turning every prerequisite into an install-screen checkbox.
- Exact manifest/source identity and completed transaction generation.
- Resource/provider identity, baseline observation, installed version and
  provenance, distinguishing created, reused, and explicitly adopted resources.
- Owned files/links/blocks/setting keys, prior values or protected backup
  references, expected written value/hash, and removal limitations.
- Operation journal, verification results, pending user actions and recovery
  instructions. Never record auth files, tokens, whole secret-bearing `.npmrc`
  files, or unrelated private settings; record only the touched non-secret keys.

Existing update receipts and config ownership checks are inputs, not proof that
the installer originally created a package or can restore an unknown value.
Adoption must distinguish those cases. Legacy observed resources with uncertain
history stay retained until a user makes an explicit reviewed ownership choice.

Use one mutation lock with process identity/liveness checks and atomic state
publication. A stale lock may be reclaimed only when the owner is proved dead;
an uncertain owner blocks mutation with a useful diagnostic. Persist intent
before each external mutation and completion after verification. Resume probes
operations left between those records instead of blindly repeating them.

Preflight freezes a plan bound to selected roots, manifest/source revision,
target identity and relevant observed state. Recheck those inputs before
destructive publication; changes require a fresh preview. Recovery is explicit:
use existing transactional rollback where available and provider-specific
compensation elsewhere. Do not promise a universal package-manager rollback.
An interrupted or reboot-pending operation needs provider-bound completion
evidence before ownership can be recovered. An artifact appearing later at the
same path is not sufficient. Recovery must distinguish retry from deliberate
abandonment and retain the original intent and uncertain outcome for either.

### Surgical removal

1. Recompute the desired closure of remaining roots. Never remove a resource
   still reachable from a remaining root.
2. Re-probe resources that became unreachable and classify created/adopted,
   pre-existing, modified, externally shared, infrastructure, and personal data.
3. Remove only proved, unchanged owned resources whose removal contract is safe.
   Package ownership alone does not prove the absence of consumers outside this
   installer. Apply the scope and retention rules below; never infer machine-wide
   removal authority from one user's ledger.
4. Restore touched config keys/blocks from baseline only while their current
   value still matches the installer's last write. Preserve later user changes
   and show the conflict. Never restore an entire old settings file over edits.
5. Order integration cleanup before removing runtimes it requires (for example
   Pi's Node-based settings helper), and order consumers before prerequisites.
   Derive the whole operation ordering; a single provider order is insufficient.
6. Verify remaining capabilities, absence of safely removed resources, retained
   resources and reasons. Commit the new selection only with durable per-resource
   outcomes; failures must remain recoverable and visible.

Do not invoke broad `autoremove`, Homebrew cleanup, whole Visual Studio removal,
or deletion of app data as a substitute for ownership. Cache deletion requires
proof it belongs exclusively to the removed feature. Complete app removal is
not permission to erase notes, workspaces, credentials or sessions.

Inspect the provider's entire proposed removal transaction, not only the named
package: a provider can remove reverse dependents and unused libraries as a
side effect. For example, [DNF documents cascading removal](https://dnf.readthedocs.io/en/latest/command_ref.html#remove-command).
The affected set must be contained in the approved, ownership-checked removal
set. Disable implicit broad cleanup, reject unknown or additional effects, and
revalidate provider state before execution. A simulation is not an execution
lock; [APT explicitly documents this limitation](https://manpages.debian.org/trixie/apt/apt-get.8.en.html).
The adapter must enforce the permitted transaction at execution or retain the
resource with an actionable explanation. Test an external dependent added
between preview and execution, including provider-side automatic cleanup.

Before removing a shell, verify the real account's current login shell. Restore
the recorded previous shell only if the current value is still the one we set
and the previous shell is valid. If safe restoration is impossible, retain the
active shell and report needs action. Remove only owned startup blocks and
unused owned `/etc/shells` entries. Never remove a binary still selected as an
account's login shell. Do not delete entire Neovim data roots: remove proved
owned plugin/parser/tool artifacts, preserve foreign contents, undo history,
shada, notes and other personal state.

| Resource class | Removal policy |
|---|---|
| Exclusive per-user files/artifacts | Automatically remove only with proved ownership, unchanged content and no remaining declared consumer. |
| Per-user runtimes/general CLI tools | Check provider dependents and other consumers, including global npm packages in the actual prefix before removing Node. A detected dependent or installer-external package blocks removal. Such checks cannot detect scripts or manual use: retain by default and offer per-item removal with that limitation explained. An explicit Keep choice persists as a retained root. |
| Machine-wide packages, SDKs and compiler workloads | A per-user ledger is insufficient. Require provider/component ownership evidence, checks for other registrations/consumers and a privileged per-item removal decision. Known remaining consumers block removal; unknown cross-user use is disclosed and defaults to retention. Never remove a whole pre-existing VS instance. |
| Windows-host resources usable by WSL | Only the native host may mutate them. Guest removal never removes host fonts, terminals, runtimes or packages; host removal treats guest/manual consumers as potentially unknown and applies the shared-resource opt-in policy. |
| Pre-existing resources, managers and personal data | Retain. Do not treat a removal checkbox as permission to take over resources outside the recorded ownership contract. |

Name and test the applicable checks in each provider adapter: installed reverse
dependencies (for example Homebrew's installed dependents), npm global inventory
under the resolved prefix, VS instance/component registrations, and platform
package database ownership/dependents. None proves the absence of manual users.
The preview must distinguish automatic removal, blocked removal, and an explicit
request to remove a proved-owned resource with uncertain external use. Test two
OS users, an unrelated global npm package, a user-kept CLI, a second VS consumer,
and a Windows host with WSL guests. The Neovim/Pi example remains eligible for
Node removal after the last declared consumer, but is subject to these rules.
Windows and each WSL guest keep separate ledgers. Host support is an explicit
Windows root so removing Windows Neovim cannot orphan a guest's clipboard/font
requirements. Do not start or inspect distros to infer ownership. Guest probes
may report an unavailable external requirement; host removal still follows the
shared-resource policy when guest usage cannot be established.

### Configuration and Nix migration

Chezmoi remains the sole config-layer owner. Gate targets **and run scripts**
from the selected manifest projection across POSIX and all four native-Windows
destination states. Windows Terminal and Pi remain explicit scoped transactions.
Compute old-minus-new config targets; an ignore rule alone does not remove a
previously installed file. Persist context needed by removal, including WSL GUI
selection, instead of rediscovering a different default.
Persist the validated selection projection in every applicable chezmoi state
and in the external input consumed by the locked Nix modules. Direct
`chezmoi apply` and the supported Nix rebuild commands must honor it. For Nix,
this means the documented `--impure` invocation with the validated
`DOTFILES_TARGET_USER` / `DOTFILES_TARGET_HOME` environment and explicit flake
target. Pure/CI evaluation must use an explicit fixture selection and remain
evaluable without reading user state. Unsupported activation, or missing,
invalid or incompatible live selection, fails before mutation and requires
adoption/recovery rather than a silent full-stack default. Test supported direct
entrypoints after deselection and rejected activation without that context. On macOS,
the Home Manager package layer runs inside nix-darwin, so its selection changes
can require privileged activation; include that in the preview.

Prefer whole-file ownership and selection-aware optional integrations. Preserve
live source links and parity; do not replace canonical shared shell configs with
per-machine generated copies merely to hide missing commands. Stage 2b must prove
the minimal selection context those shared configs need. A command-exists guard
alone is insufficient when the user has an unselected external tool installed.

Project selected Nix resources into the existing locked modules without
persisting local JSON in the checkout or giving Home Manager ownership of
dotfiles. The first projection must preserve observed prior managed packages
until adoption resolves their ownership; an empty or incomplete selection must
not silently drop the old full profile. Keep unrelated user profile inputs and
Homebrew entries. Retain scoped receipts and `cleanup = "none"`; do not use a
broad cleanup switch to implement removal. A migration fixture must prove this
before the first real selection-driven activation.

Preserve the existing side-by-side, exact-release migration/recovery path.
Introduce ledger schema migrations with old/missing-field tests, backups and
recovery, not an in-place rewrite of a live release checkout. Legacy command
arguments translate to explicit requests during a documented compatibility
window; migrate repo CI and runbooks before retiring any supported spelling.
Removing redirection-implies-all is an intentional breaking change to the
documented legacy behavior: in both new and compatibility paths, a noninteractive
invocation without an explicit request exits nonzero before mutation and prints
the explicit-request usage hint. Include this semantic change in CI/runbook
migration, not only flag spelling changes.

On every reconciliation, detect mutations from older setup releases or outside
tools, even after adoption has completed. Treat newly observed resources and
changed provider state as unowned until reviewed; preserve them rather than
dropping them through a later selected Nix projection. Test the sequence new
engine -> older exact-release setup/recovery -> new engine, including restored
packages/config and an older executable encountering a newer ledger schema.
An incompatible engine must refuse ledger mutation; unknown fields cannot be
silently discarded. Do not assume an old setup script understands the new state.

## Distribution and support contract

Prototype verified prebuilt binaries for macOS, Linux and native Windows using
the architectures actually supported by the audited provider inventory. Retain
WSL as a Linux guest with explicit Windows-host boundaries. Match binary,
manifest, config revision and release proof; test missing, mismatched and tampered
artifacts. Build first, then bind final artifact digests in certification. This
does not by itself establish bootstrap trust: the launcher still needs an
authenticated expected digest when it starts. A same-tree digest is circular
only if the digest-bearing content participates in the binary's build identity;
a reproducible controller-only source subtree is an alternative to evaluate,
not something this design has proved. Do not assume a later certificate is
automatically trustworthy or available to the same-tag launcher.
Current release resume code (`scripts/release.py:1469-1471`) requires the published
release's sole asset to be `release-proof.json`; supporting binaries requires
updating and testing that policy along with publication and closure.

Stage 2a must settle and demonstrate an authenticated source for the expected
binary digest before committing to distribution: define the trust identity,
binding to repository/tag/source, verification without uninstalled tooling,
offline behavior and downgrade policy. Evaluate an immutable-release digest or
verified attestation against the current trust contract; do not accept a checksum
downloaded beside an arbitrary binary as authentication. The post-publication
closure PR cannot be assumed available to bootstrap that same release. Missing,
untrusted or mismatched proof must fail before executing the binary. Record the
chosen mechanism and its tamper/offline tests in this plan at the stage 2a exit.

Extend release preparation/publication, immutable release validation, dependency
maintenance and language security scanning for the new compiled component. Test
actual downloaded launchers, proxy/offline diagnostics, spaces/non-ASCII paths,
terminal detection, OS execution policy, and elevation handoff. Preserve the
current no-remote-eval bootstrap policy. Bootstrap must work without Node, Go,
chezmoi, or a compiler already installed.
Prove the unreleased official-branch lane too: build and bind the controller to
the exact source using a pinned build environment, then exercise the verified
artifact without a compiler on the target. Requiring target-side Nix activation
just to build or display the selector would violate the pre-Apply boundary.
Unsigned Windows execution-policy restrictions need an explicit supported path
or a visible blocked result; never silently switch to an unverified launcher.

The support inventory is an explicit OS version / distro / architecture /
provider / host-context table checked into the implementation. Start from all
currently documented claims: README lists apt, dnf, pacman, zypper and apk while
the hosted Linux install path primarily exercises Ubuntu. Nix currently exposes
Apple Silicon macOS and x86_64/aarch64 Linux; Windows native and WSL have different
contracts. Audit Windows ARM support and x64 toolchain assumptions explicitly.
Do not claim that three OS job names prove all of these combinations, and do not
silently drop an advertised platform to get a green release. A narrowed support
contract requires a separately documented owner decision before implementation
can be called complete.

Stage 1 must also assign an evidence source to every support-table cell: native
VM/image source, provisioning/reset procedure, runner or owner-run machine,
WSL2 Windows host where applicable, executor, and infrastructure cost/schedule
owner. The repository owner approves that inventory before stages 2a and 4
depend on it; unresolved access is a blocker, not deferred release evidence.
Owner-run greenfield evidence may count when it follows the checked-in runbook,
records the exact source revision, baseline, selections, assertions and sanitized
logs in `tests/greenfield/LEDGER.md`, and includes independent readback/review.
Hosted native jobs and containers remain useful complementary evidence. No new
paid infrastructure is authorized by this document.

## Verification and release acceptance

Preserve current gates, then add the following tests at the relevant layer.
Tests observe behavior; mocks are restricted to external system boundaries.

| Layer | Required evidence |
|---|---|
| Pure planner | Empty/single/all roots; transitive dependencies; conditional providers; missing IDs; cycles; incompatible versions; stable plans; existing command/provider conflicts; shared dependency preservation; cleanup ordering. Property tests cover reachability/removal invariants. |
| State and ownership | Fresh and legacy state; unknown/missing fields; adoption; new engine -> older release -> new engine; pre-existing packages; modified targets; scoped key restoration; cross-user/WSL ownership and explicit Keep decisions; atomic writes; lock contention; stale/uncertain locks; identity mismatch; interrupted migration and disk-full/write-denied recovery. |
| Adapter contracts | Real fixture files and boundary fake providers; exact versions/pins; correct command resolution; no unrelated writes; provider errors and pending auth/reboot; failure before and after every external mutating operation. |
| Configuration | Every root's selected and unselected targets on each applicable OS, all Windows known folders and Terminal variants, parity/live links, run-script gating, shared shell integrations with externally installed unselected binaries, WSL GUI state through removal. |
| Native clean-machine lifecycle | Each supported root alone; each shared dependency edge with both removal orders; relevant pair combinations; all roots; full-minus-one; remove/reinstall; repeat install/repair; interrupted install/update/removal; legacy adoption. Assert selected functionality AND absence of unintended resources relative to the recorded baseline. |
| Real application proof | Locked Neovim plugin startup, parser compilation/load, formatter and LSP smoke for every bundled language; shell/profile consumption; terminal launching; Pi package/config loading; remaining consumers work after shared-resource removal. Authentication-dependent behavior has separate explicit evidence. |
| Terminal and privilege | Real PTY and Windows ConPTY keyboard flows, back/cancel/EOF, resize/plain output, redirected input/output, sudo/UAC denial and completion, reboot/resume, active PowerShell replacement, readable failures and exit status. |
| Distribution | Exact published/downloaded entrypoint from clean prerequisites, architecture/provider coverage, binary/source mismatch, corrupt artifact rejection and certified release binding; no local compiler dependency. |

First validate the output of the actual public installer **without repair**.
Current greenfield validators invoke Lazy/Tree-sitter/Mason reconciliation during
validation, so a green validator alone cannot establish installer completeness.
Split read-only verification from explicitly tested repair; retain earlier
assertions and all meaningful existing coverage.
This extends to production Neovim startup: `nvim/init.lua` calls the pinned
checkout repair helper, and [Lazy installs missing plugins by default](https://lazy.folke.io/configuration).
Provide a check-only pinned-checkout path and disable missing-plugin installs
and all parser/Mason provisioning in verification. Check locked plugin identity,
parser/query presence and tool availability before exercising runtime behavior;
assert that verification did not provision or repair the managed artifacts.
Update invariant 23 and its tests in the implementation change without weakening
normal startup's executable-cache proof. A missing promised Rust dependency is
a failed or needs-action capability, never a successful skipped language test.

Hosted runners are useful but not a substitute for minimal native machines:
preinstalled Visual Studio, Rust, Node or other tooling can conceal missing
dependencies. Record the real baseline; use disposable VMs with the relevant
prerequisites absent for from-scratch cases. Removing a command from PATH alone
does not prove absence of its SDK/runtime. Containers complement Linux package
tests but do not prove macOS, native Windows, GUI, elevation or WSL host behavior.

Run focused deterministic and affected-platform cases on PRs. Run the full
claimed-platform lifecycle matrix on release candidates, with scheduled runs
to detect drift. Every supported singleton and shared edge must have evidence;
do not reduce release coverage to pair sampling or allow missing mandatory
results to count as success. Publish the exact revision, environment baseline,
selected roots, receipts, outcomes and sanitized failure/recovery evidence.
Enumerate feasible root selections in the pure planner per supported platform,
and use property-based transition tests for state/order boundaries. Graph
consistency is not dependency completeness: native singleton and consumer
survival tests must also prove that the manifest matches real application needs.

Acceptance requires all three OS families and the declared support table to
pass; complete dependency closure; no unintended install/config side effects;
safe shared-resource removal; honest pending/failed status; legacy preservation;
real installed-entrypoint proof; and matching docs. An install-only adapter or a
skipped platform is unfinished work, not a shipped lifecycle implementation.

## Sequenced delivery

Stages are independently reviewable acceptance gates, not a requirement to open
one PR per stage. This plan ships as one commit in one PR; future implementation
packaging must follow the user's delivery scope without dropping any gate.
No stage may claim that the complete overhaul is delivered before stage 6.
The separate prototype does not complete stage 1, 2a or 2b.

| Stage | Deliverable | Exit check | Status |
|---|---|---|---|
| 0 | Consolidated design and both review dispositions | Canonical links, evidence/corrections and implementation boundary are reviewable | DONE (this plan delivery) |
| 1 | Exhaustive capabilities/resources/side-effects and exact support/evidence table; reproduce candidate gaps | Every current operation mapped; no silently lost feature/platform; upstream removal contracts checked; VM/WSL infrastructure, evidence acceptance and ownership assigned | PARTIAL manifest in draft `44308da`; complete inventory, evidence assignments and runtime reproductions remain open |
| 2a | Distribution/terminal and trust prototype | All three OS launchers, PTY/ConPTY/psmux input, authenticated digest/source binding, tamper/offline behavior, no-Node bootstrap and verified unreleased-branch artifacts without a target compiler proved before committing the engine to Go | PLANNED |
| 2b | Manifest, pure planner, versioned ledger and selection projections, after 2a passes | Planner/state properties, symlink parity and safe initial/subsequent Nix reconciliation proved | EXPERIMENTAL core in separate draft; 2a and projections unproved |
| 3 | Extract noninteractive platform adapters from existing mechanisms | Existing full-install and migration behavior still passes; resource probes and reversible ownership contracts tested | PLANNED |
| 4 | Complete Neovim + Pi vertical slice, including Windows compiler and shared Node | Singleton/shared install, remove in both orders, recovery and no-repair runtime verification on all applicable platforms | PLANNED |
| 5 | Remaining features, legacy adoption and human/machine interfaces | Full inventory implemented; auth/elevation/pending behavior and migration fixtures pass; old callers translated | PLANNED |
| 6 | Full clean-machine matrix, release integration and operator documentation | All acceptance evidence bound to release revision; publish only through canonical release workflow | PLANNED |

Documentation-only verification of this plan must not be presented as evidence
that the proposed engine or its three-OS lifecycle already works.
