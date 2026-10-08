# Interactive installer and removal overhaul

Status: **DESIGN RECONCILED; implementation integrated; native acceptance IN PROGRESS.**
Updated: 2026-10-10. Original reviewed source: `ae9a6446eb0a837a144d77d2a6345db967c61e4e`.
The current candidate uses the new setup/migrate entrypoints. It is not ready to
ship until its final gates pass. This document retains design decisions and
superseded stages; use README for operation and the status inventory for proof.
See the [review dispositions](../reviews/2026-10-08-interactive-installer-overhaul.md)
for evidence, corrections, and unverified candidates.

All implementation work and the reconciled plan are consolidated in
[PR #87](https://github.com/luisgui1757/dotfiles/pull/87), as one commit above main.
PR #86 was closed after its contents were preserved there. There is no separate
plan delivery. The implementation remains unfinished until every acceptance gate
below passes; the current component tests do not establish product completion.

## Roadmap reset and support decision, 2026-10-10

The latest owner decision supports **Ubuntu/Debian only** on Linux amd64/arm64,
macOS arm64, and Windows amd64. WSL follows the supported Linux distribution.
Fedora, Arch, openSUSE, Alpine and Linuxbrew are outside this release. This
supersedes broader Linux inventories below and needs no additional provider work.
The owner also removed WezTerm on every OS. Its active selection, pins, config
and tests are retired; passive released sources and migration records remain.

Keep the implemented engine; complete concrete providers and integrations before
adding infrastructure. Reuse the released package knowledge and Neovim headless
commands. Existing Git can be reused, but missing Git still requires provisioning.
A declared bundled language remains complete; an absent Rust prerequisite cannot
become a successful skipped language test. Keep pre-existing package infrastructure
and personal data with explicit disclosures, as already required by ownership.

The current five milestones and per-capability status are maintained in
[the status inventory](../installer-status.md). APT, Neovim and bootstrap/migration
have isolated implementation owners; shared contracts, catalog, CI and docs have
one integrating writer. Update documentation with each integrated change; retain
the append-only ledger. Do not multiply checkpoint detail across every guide.

Verification is bounded by behavior: every checkbox on each applicable target
installs alone, is actually used and removes safely; each provider demonstrates
changed-version update, repair, interruption and outside-consumer protection;
shared dependencies survive both removal orders. One full install then selective
and complete removal covers combined operation, alongside deterministic graph
transition tests. This supersedes the requirement for a fresh full-minus-one
installation for every root and a new cross-user matrix. Existing meaningful
regressions remain. Failures justify focused reproduction and affected reruns;
a new shared-contract defect justifies broader regression work. Required final
gates and the full independent review remain mandatory.

The old new-engine → old-release → new-engine round trip is not a supported
workflow; explicit migration and interrupted migration are. The prebuilt binary,
attestation and official-branch distribution proposals below are superseded by
the pinned verified Go/source bootstrap. Do not implement both. The old broader
acceptance passages are historical where they conflict with this section.

## Major-release architecture, reconciled 2026-10-09

### Whole-product challenge: current decision

The user's subsequent authorization reopened **all** design choices. The
whole-product Opus consultation supersedes the private-Nix decision below; that
earlier rationale is retained as review history, not an implementation target.

- Keep one Go implementation and the pure dependency graph. Port the existing
  platform knowledge; do not preserve two shell/PowerShell implementations behind
  adapters. Human installation, update, checking, repair, migration and removal
  stay in menus. CI uses the same controller through its explicit machine protocol.
- Remove **Nix and chezmoi** from the new runtime. Use checksum-pinned upstream
  archives in private per-user directories for portable tools on all three OSes.
  Native package managers own system/vendor packages, with one declared provider
  per resource/platform and no fallback cascade. The extra digest maintenance is
  worthwhile because an archive provider is necessary on Windows regardless.
- Build the command from the trusted checkout using a pinned, verified Go
  toolchain, cached by source content. No new Node prerequisite, prebuilt-binary
  trust scheme, or official-branch-only execution gate. Download verification,
  dependency locks and the protected release workflow remain mandatory.
- WSL2 uses the Linux graph. Session facts (interop, graphical session, clipboard)
  are observations, not a fourth platform or persisted selection identity. Fonts
  belong to renderers. Clipboard availability cannot block plugin provisioning.
  No WSL detector, Windows-host support checkbox or separate guest lifecycle is
  needed. Probe capabilities where consumed, without provisioning across the
  host/guest boundary. The boot-only hosted WSL job is retired because it tested
  the runner's virtualization rather than this product.
- macOS supports Apple Silicon (arm64) only. Intel macOS is deliberately outside
  the product contract; do not add packages, build outputs or CI lanes for it.
  Reject an amd64 macOS process before provider discovery, including execution
  through Rosetta. Linux amd64/arm64 and native Windows amd64 remain separate
  supported targets. Native jobs assert their actual OS and architecture.
- A checkbox promises a working tool, including its necessary integration.
  Application startup consumes provisioned dependencies; checking must not repair
  the installation. Pin the Mason registry as well as plugins and direct tools.
  A standalone Rust choice can expose the toolchain, but bundled editor language
  features must still receive their real prerequisites automatically. Do not
  relabel missing bundled functionality as an advisory to avoid provisioning.
- Preserve durable ownership and original configuration baselines. Convergence
  still requires recorded intent before file publication and independent recovery
  evidence. Eliminate Nix-specific batching, leases and boot identity with Nix.
  Do not replace verified no-replace publication with destructive overwrites, or
  infer ownership from package presence. Simplify recovery UX without discarding
  the evidence needed after interruption or source movement.
- Removal deletes unreachable private dependencies and only attributable, unused
  native dependencies. It retains pre-existing/shared resources and personal data
  with explicit reasons. Blanket retention of all system dependencies is not the
  requested surgical uninstall, and package-manager autoremove is not a substitute.
- Native formula operations verify affected formula linkage and preserve external
  cask identity and declared dependency availability. They disclose that external
  applications were not exercised. Supported managed applications require their
  own health checks. Do not replace an unknown external runtime with a permanent
  post-mutation refusal, or claim package metadata proves application execution.
- Remove unsolicited global side effects: npm user-prefix rewrites, bash-to-zsh
  exec hooks, forced GNOME maximizing and automatic notes-vault creation. Keep
  explicit login-shell consent and documented application permissions.
  Notes is a Neovim workflow rather than an installable package: remove the
  proposed notes checkbox and its unimplemented installer resource. Keep the
  locked plugin available for existing vaults and the `NOTES_VAULT` preference,
  without installer-owned personal folders or a new preference-persistence API.
- Pi uses its upstream standalone release through the existing verified archive
  lifecycle, avoiding a separate npm package-installation engine. Include runtime
  assets and native helpers. Retain Node/npm and Git for Pi's package features,
  and provision fd/ripgrep explicitly. Scoped theme settings reuse the existing
  file publisher with explicit JSON field ownership and application locking;
  prior values and later unrelated edits survive removal. Real extension loading,
  package features and interactive theme rendering remain acceptance requirements.
  Passive theme assets do not depend on the executable; a retained preference
  protects those assets without retaining Pi or its runtime prerequisites.
  This supersedes the original
  npm delivery choice in the historical inventory below.
- Keep review history append-only. Replace obsolete runtime tests only alongside
  the corresponding replacement behavior and native proof. Keep required check
  names meaningful; never emit an empty compatibility success.

Opus executed as `claude-opus-5-5` with 84 read-only calls; `xhigh` was requested
and the effective effort was not exposed. All 630 supplied source hashes were
unchanged at completion. This is architecture consultation, not code approval.
Its suggestions to delete all recovery records, retain system dependencies by
default, replace the machine protocol with human flags, and delete review history
are not accepted. Those choices conflict with data safety, the requested product
behavior, or the repository's explicit documentation rules.

**Prototype retirement completed:** the never-released Go Nix adapters, workers,
batching and their dedicated projection/activation tests are deleted. They were
not connected to the public command. Generic lifecycle/preservation tests remain,
and the currently shipped setup/Nix gate stays intact until full cutover. This
reduces the implementation under review without removing a shipped behavior.

**Finite delivery sequence:** (1) working installed-command lifecycle using the
archive provider and native configuration, with real binaries; (2) full tool
catalog and integrations, including Neovim's Windows compiler chain; (3) explicit
legacy migration and removal; (4) delete obsolete implementations and reconcile
docs/CI; (5) exact-head hosted lifecycle matrix and fresh full-delta Opus review.
The first checkpoint is a vertical slice, not another isolated backend. Every
checkpoint is internal to the same PR and final single commit. No incomplete
checkpoint is a release. Support claims follow native evidence; cross-compilation
and mocked package managers do not prove installation.

### Superseded private-Nix decision

The user explicitly authorized a breaking release without chezmoi and without
backward-compatible execution paths. This decision supersedes the older
chezmoi/Home Manager/nix-darwin implementation directions below; those sections
remain historical until the replacement and acceptance fixtures are complete.

1. **Bounded native configuration ownership.** The Go installer maps canonical
   repository files to explicit targets, with POSIX live links, Windows known
   folders, scoped settings merges and verified recovery bytes. It is not a
   template engine. Normal operations never execute chezmoi. A separate migration
   entrypoint reads old installations as data and obtains explicit adoption;
   removal restores the approved baseline and preserves subsequent user changes.
2. **Private Nix package environment.** Keep the reviewed nixpkgs lock for POSIX
   CLI versions and Linux ARM clangd. Retire Home Manager, nix-darwin and
   nix-homebrew from the new runtime. Publish only a private, rooted immutable
   package environment; preserve unchanged members exactly and retain man pages
   and completions. No default-profile/system activation, global garbage
   collection or Homebrew cleanup. Native Homebrew continues vendor applications.
   This removes shared-profile supervision rather than weakening its recovery
   checks. Existing foreign profiles are never adopted implicitly. Migration must
   account for nix-homebrew's store-backed runtime before detaching old Darwin
   roots. Merely leaving a broken or permanently frozen runtime is not completion.
3. **One graph, native dependency semantics.** The graph owns repository-level
   requirements; managers resolve their internal closures. Isolated package
   closures leave the active environment when their last consumer is removed.
   For native managers, remove only attributable installed dependencies that the
   manager proves unused, with a reviewed exact removal set and post-verification.
   Never run global autoremove. Before/after presence or a helper exit code alone
   does not grant ownership. Pre-existing, shared and modified resources remain
   with explicit reasons. Reboots, host actions and unavailable prerequisites are
   resumable needs-action outcomes, never successful completion.

Alternatives rejected: retaining system activation preserves complexity whose
purpose is sharing global namespaces, while replacing locked POSIX packages with
five distro providers loses consistent versions without removing native-provider
work. The strongest cost of the choice is keeping Nix alongside Homebrew on Mac;
version parity and an isolated removal boundary justify it for this release.

Opus 5.5 completed the first-principles consultation (56 read-only calls, actual
`claude-opus-5-5` observed; `xhigh` requested, runtime effort unexposed). The writer
accepted its private-package recommendation but strengthened native ownership and
unused-dependency removal instead of treating inventory deltas as ownership or
leaving all incidental dependencies indefinitely. The consultation is architecture
input, not a code-review approval. Full native migration, rooted crash recovery,
package and config lifecycles, compiler/SDK provisioning and final full-delta
review remain release gates. One PR and one final implementation commit remain
required.

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
  [ ] Windows Terminal  Terminal with the dotfiles configuration

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
through Windows ConPTY and Herdr before choosing a framework. The selector and
preview must be available before activating Nix or another package provider.

## Capability and dependency inventory

These are the initial selection boundaries. Separate IDs in a row remain
separate checkboxes; grouping here does not create an inseparable bundle.
Unavailable capabilities are explained and cannot be selected. Linux guests use
the Linux choices; installation never crosses into a different host OS.

| User selections | Owned behavior and dependency boundaries |
|---|---|
| Neovim | Native Neovim and tree-sitter CLI, canonical config, locked Lazy plugins, parsers, Mason tools, and their runtime/build prerequisites. Keep the bundled language configuration together in v1. Linux clangd retains its Nix provider. |
| VS Code | Editor, Rose Pine extension and scoped JSONC settings. Preserve unrelated settings and restore previous touched values when safe. `jq` is not intrinsically a product dependency; the current helper has another parsing path. |
| Ghostty; Windows Terminal | Each terminal and its configuration are independently selectable on supported platforms. Configured fonts and shell requirements are dependencies. Ghostty's GNOME/X11 maximize helper is a platform-conditioned integration. Windows Terminal keeps its dedicated multi-target merge transaction. |
| Herdr | Native/vendor application and platform config. Windows config currently launches `pwsh.exe`; resolve that requirement. |
| tmux | macOS/Linux only, including WSL: generated status bar and pinned functional plugins. Windows uses Herdr; no psmux compatibility layer. |
| zsh; PowerShell 7 | Shell and profile, plugin/module resources and their real prerequisites. Login-shell changes are a separately explained choice inside shell setup, not an automatic global side effect. |
| Starship; lsd; zoxide; fzf; ripgrep; fd | Individually selectable user-facing prompt/navigation/search tools. ripgrep and fd can also be automatic Neovim prerequisites without selecting their roots. Activate shared shell integrations only for selected capabilities, even when unselected binaries happen to exist on PATH. |
| AeroSpace (macOS only) | Vendor cask and canonical config; its trusted Homebrew tap remains retained infrastructure. Accessibility permission is a user-controlled OS grant: report needs action until usable and disclose retained grants during removal. Keep ownership-gated cask removal separate from nix-darwin's `cleanup = "none"`. |
| Git defaults; lazygit; GitHub CLI; gh-dash | Independently useful roots. Git defaults own only the repo's documented integration, never the whole user gitconfig. lazygit requires Git; gh-dash runs the pinned upstream executable directly, requires GitHub CLI and reports authentication separately. No extension registration is needed. |
| Pi | npm package, theme/keybinding files and scoped theme-setting merge; Node/npm and cleanup runtime ordering are automatic. Do not touch sessions, credentials or provider preferences. |
| Sentinel agent policy | Pinned upstream installation, supported consumers and owned policy blocks. Prove scoped removal against the pinned upstream contract before shipping its lifecycle. |
| Repository development checks | An explicit contributor capability/preset for test-only tools. Do not install shellcheck, hyperfine, or other test tooling merely because a user selected an unrelated application. |

The inventory must map **every** current installer operation, config target,
chezmoi run script, external checkout and side effect to a root, prerequisite,
integration, retained infrastructure resource, or explicitly retired behavior.
Unmapped operations block completion of stage 1. In particular:

- A terminal's required shell executable is distinct from the selectable
  dotfiles shell profile and optional enhancements. Installing PowerShell to
  launch Windows Terminal must not implicitly enable an unselected profile bundle.
- Font resources provide the configured appearance; distinguish an exact
  configured font requirement from a program's ability to launch. Reuse a
  valid existing font without claiming ownership.
- Clipboard support follows actual capability: macOS, Wayland, X11, an available
  Windows bridge, or an explicit working custom provider. Do not infer availability
  from kernel branding, hardcode one provider or require a GUI on headless hosts.
  An external host requirement is diagnosed without provisioning another OS.
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

- Selected roots and OS/architecture identity. Graphical applications are ordinary
  selections; session observations never become persistent platform identity.
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

Use a permanent native mutation lock and atomic state publication. Providers
whose workers outlive the controller additionally hold a permanent inherited
activation lease. Never reclaim either by PID or delete its inode. A durable
started attempt without a result remains uncertain on the same kernel boot,
including when an elevated descendant closed its inherited descriptors. A
different kernel boot permits recovery classification, never automatic completion.
An uncertain owner blocks mutation with a useful diagnostic. Persist intent
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
5. Order integration cleanup before removing any runtime it actually requires,
   and order consumers before prerequisites. Pi's Go settings publisher has no
   Node cleanup dependency.
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
| Pre-existing resources, managers and personal data | Retain. Do not treat a removal checkbox as permission to take over resources outside the recorded ownership contract. |

Name and test the applicable checks in each provider adapter: installed reverse
dependencies (for example Homebrew's installed dependents), npm global inventory
under the resolved prefix, VS instance/component registrations, and platform
package database ownership/dependents. None proves the absence of manual users.
The preview must distinguish automatic removal, blocked removal, and an explicit
request to remove a proved-owned resource with uncertain external use. Test two
OS users, an unrelated global npm package, a user-kept CLI, a second VS consumer,
and an external consumer invisible to the installer's graph. The Neovim/Pi example
remains eligible for Node removal after the last declared consumer, but is subject
to these rules. Each installation owns only its native environment. Do not start
or inspect other operating systems to infer ownership; apply the same shared-
resource policy when external usage cannot be established.

### Provider transactions and package ownership

The 2026-10-08 focused Opus consultation resolves the Nix transaction boundary:
keep per-package graph resources and receipts, and give them explicit provider
bindings. A dependency on Nix infrastructure is not a provider declaration.
The planner projects a flat package list from wanted resources plus observed
members that have not passed every removal rule. Nix modules consume that list;
they must not derive a second dependency graph from feature names.

Use a grow activation before consumers and a shrink activation after consumer
cleanup. Build each target generation with its operation-bound GC root before switching,
retain the baseline generation, then journal the old
and target generation, full member/store-path maps, intended changes and the
projection digest before activation. A matching generation and member map are
completion evidence after interruption. Any other result requires reconciliation
and preserves uncertain ownership. macOS additionally verifies the Homebrew
inventory because a switched system generation does not prove its bundle phase
succeeded. Keep `cleanup = "none"` and perform separately approved cask/formula
removal only after dropping its declaration.

Provider discovery covers the entire active package set, including unselected
legacy and external members. Unknown generations or unmapped members block
activation rather than being silently dropped. Final verification covers every
preserved member as well as selected and explicitly removed resources. Updating
a locked Nix generation can refresh unowned members; disclose that impact in the
preview and preserve their ownership class.

Native managers retain resource-sized operations, with provider-bound operation
records for incidental dependencies. A before/after inventory difference alone
cannot distinguish another process's installation from ours. Require the
provider's own transaction evidence as well as the approved intent and observed
change. Keep remaining installer-created incidental dependencies in provider
state so either consumer-removal order can eventually collect an unused member.
Never use broad autoremove, generation expiry or garbage collection.

This is an accepted implementation constraint, not completed adapter code.
The consultation's upstream activation ordering, transaction attribution and
legacy-generation assumptions still require native proof.

The 2026-10-09 publication consultation keeps those owners and recommends an
activation-tree lease, durable attempt result and classification of the exact
journaled target before retry. This is a direction awaiting native interruption
proof, not a claim that upstream activation is transactional. Modern Nix profile
publication and `nix-env --set` do not universally take native locks; a separate
flock around activation cannot exclude every direct/older writer. Preserve
uncertain intent when interference cannot be proved absent. History scans or
process-table checks alone do not establish race-free exclusion.

Generation-history capture and completion auditing passed local and hosted
Linux/macOS verification at `979c448`. They bind inactive generation links and reject unexplained new
generations, gaps, baseline deletion/replacement and intermediate foreign-package
changes. The private native-profile regression reproduces approval reuse after
publication/rollback and passes with history binding. The shared partial-state
classifier and exact-target retry are implemented. macOS real conflict recovery
passed at `5a1f5e9`; Linux fault coverage and both journal readbacks passed at `ad6bfe8`. The full native
interruption matrix and adoption remain unfinished; this checkpoint does not claim universal native-writer exclusion.
The completed Opus review found a cancel/abandon/no-op finalization regression
and three smaller signal/metadata/history defects. Their corrections pass the
complete local gate and native private-profile tests; expanded hosted sudo-prompt
scenarios passed at `9a98a44`. Subsequent recovery review found abandonment,
removal-retry and publication defects; corrected regressions pass locally, with
all 26 hosted checks passing at `ad6bfe8`; later controller review corrections
are being verified. This is not final product approval.

Normal installed-supervisor lifecycle passed on hosted Linux/macOS at `f227761`.
The subsequent Opus review requires kernel-session boot identity, exact worker
bytes/clean-revision binding, native account-home validation, build-time worker
roots and an ordered proxy handoff. Their regressions are being verified; native
clock-step and sudo/launchctl descendant tests must pass on disposable macOS.
This narrows the supervisor uncertainty; exact-target recovery and external
publication/adoption are still implementation requirements.

The complete inventory must bind mutable profile publication namespaces as well
as immutable package contents. A native reproduction showed equal package bytes
at different default-profile destinations otherwise produce identical approvals.
Record alias presence, destination and resolved parent before activation; permit
only the explicitly planned canonical initial creation. Retain older snapshots
without synthesizing this missing proof.

An interrupted target must pass its original complete preservation check before
any per-package completion proof grants ownership. Recomputing the retry plan
from damaged current state cannot replace that check. Legacy `manifest.nix`
inventory must retain its actual metadata; normalized `nix profile list` discards
priority and active state. The reader now has a real temporary-profile regression
and the hosted fixture requires it on both POSIX OSes.

### Configuration and Nix migration

Chezmoi remains the sole config-layer owner. Gate targets **and run scripts**
from the selected manifest projection across POSIX and all four native-Windows
destination states. Windows Terminal and Pi remain explicit scoped transactions.
Compute old-minus-new config targets; an ignore rule alone does not remove a
previously installed file. Persist actual owned targets instead of rediscovering
destinations from a different session. The Nix/chezmoi instructions in this
historical subsection are superseded by the major-release decision above.
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
recovery, not an in-place rewrite of a live release checkout. Recovery retains the exact authenticated engine and source revision that began
an operation. After source changes, partial publication can resume only through
that retained revision; abandonment is limited to unchanged or completed provider
publication. Missing cached bytes require authenticated retrieval, never a silent
new-engine replay.

Legacy command
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
WSL as ordinary Linux, without a separate distribution artifact. Match binary,
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
Apple Silicon macOS and x86_64/aarch64 Linux; WSL uses the corresponding Linux
contract. Audit Windows ARM support and x64 toolchain assumptions explicitly.
Do not claim that three OS job names prove all of these combinations, and do not
silently drop an advertised platform to get a green release. A narrowed support
contract requires a separately documented owner decision before implementation
can be called complete.

Stage 1 must also assign an evidence source to every support-table cell: native
VM/image source, provisioning/reset procedure, runner or owner-run machine,
executor, and infrastructure cost/schedule owner. The repository owner approves
that inventory before stages 2a and 4
depend on it; unresolved access is a blocker, not deferred release evidence.
Owner-run greenfield evidence may count when it follows the checked-in runbook,
records the exact source revision, baseline, selections, assertions and sanitized
logs in `tests/greenfield/LEDGER.md`, and includes independent readback/review.
The 2026-10-08 owner decision below assigns GitHub-hosted native runners and
disposable Linux containers as the execution surface. No paid infrastructure
is authorized by this document.

## Verification and release acceptance

Preserve current gates, then add the following tests at the relevant layer.
Tests observe behavior; mocks are restricted to external system boundaries.

| Layer | Required evidence |
|---|---|
| Pure planner | Empty/single/all roots; transitive dependencies; conditional providers; missing IDs; cycles; incompatible versions; stable plans; existing command/provider conflicts; shared dependency preservation; cleanup ordering. Property tests cover reachability/removal invariants. |
| State and ownership | Fresh and legacy state; unknown/missing fields; adoption; new engine -> older release -> new engine; pre-existing packages; modified targets; scoped key restoration; cross-user ownership and explicit Keep decisions; atomic writes; lock contention; stale/uncertain locks; identity mismatch; interrupted migration and disk-full/write-denied recovery. |
| Adapter contracts | Real fixture files and boundary fake providers; exact versions/pins; correct command resolution; no unrelated writes; provider errors and pending auth/reboot; failure before and after every external mutating operation. |
| Configuration | Every root's selected and unselected targets on each applicable OS, all Windows known folders and Terminal variants, parity/live links, run-script gating, shared shell integrations with externally installed unselected binaries, graphical selection and headless-session behavior through removal. |
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

The user approved GitHub-hosted runners as the execution surface on 2026-10-08.
Use fresh native Windows, macOS and Linux jobs, record their real baseline, and
establish prerequisite absence for from-scratch cases. Preinstalled Visual
Studio, Rust and Node can conceal missing dependency edges; removing a command
from PATH alone does not prove absence of its SDK/runtime. WSL uses Linux's
lifecycle; test optional clipboard and graphical integrations against actual
capabilities, not a separate boot-only job. Record desktop, reboot or
privilege cases the host cannot exercise as gaps, never simulated successes.
Containers complement native distro package tests. This execution decision
supersedes the earlier requirement to provision separate disposable VM hosts;
it does not waive the lifecycle assertions or authorize paid infrastructure.

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

## Consolidated simplification (2026-10-10)

The final Opus 5.5 consultation inspected 611 source files and PR #88's 19-file
delta, making 78 read-only calls. The writer and reviewer agree to minimize total
complexity: preserve the tested planner/controller/receipts and publication
primitives; eliminate overlapping shell owners and duplicate runtime mechanisms.
This supersedes the old Nix prototype sequencing.

- One generated shell assembly and one attachment per physical profile. Tool
  selection controls hooks; executable presence alone does not select a feature.
  Keep quiet login PATH behavior, local hooks, profile encodings and Windows ACLs.
- Checksum-pinned private archives plus one declared native provider per supported
  resource/platform. Observe distribution/libc and session capabilities before
  selection. WSL uses Linux. Do not silently drop an existing platform or tool.
- Share file publication and bounded cleanup; keep small format-specific shell,
  JSONC and registry transformations. Do not build a universal rollback framework.
- Remove owned unused dependencies; protect pre-existing, edited and externally
  consumed resources. A package-manager dry run alone does not enforce a removal
  boundary. Do not retain every incidental native dependency forever.
- Explicit migration covers released installations without starting an old Nix
  runtime. Retire obsolete setup only after replacement outcome tests pass.
- Use one final complete Opus review after implementation and native verification;
  consult earlier only for a material new architectural blocker.

## Sequenced delivery

One implementation commit and one PR (#87) contain the complete overhaul and the
outcomes from #88. #86 is already superseded. A checkpoint is never a release.

The five active milestones are CLI providers, Neovim, remaining integrations,
public bootstrap/migration, and final acceptance/delivery. See the single current
[roadmap and coverage table](../installer-status.md#scope-and-remaining-milestones)
for deliverables, exit checks and status.

No stage can claim completion of the overhaul before all exit checks pass.

### Multiplexer simplification, accepted 2026-10-10

Herdr is supported on macOS, Linux and Windows. tmux is an additional macOS/Linux
choice. Remove psmux entirely from provisioning, archive pins, dependency graph,
profile workarounds and configuration. WSL remains Linux. LazyGit uses the same
upstream default keybindings on all three OSes, while preserving native config
locations (LocalAppData on Windows). A single tmux config replaces the platform
split; keep existing POSIX clipboard, generated Rose Pine and session outcomes.

The installer already owns plugin versions and updates. Therefore load sensible,
yank, resurrect and continuum directly from private pinned archives; TPM adds no
necessary runtime responsibility. Preserve session data and active servers on
removal. Activate continuum's save/restore helpers without its unrelated login
service-management hook. Runtime proof must include automatic restoration and
saved pane contents with spaces/apostrophes in the home path, retention of the
newest backups and preservation of personal startup settings. Correct known
upstream quoting defects through bounded, exact reviewed source replacements;
changed source text or match counts fail before publication.

Legacy migration must distinguish old managed configuration from personal files;
retiring psmux support does not authorize deleting unproved package installations
or user session data. Existing release fixtures remain historical input, not a
runtime compatibility layer.


### Integrated remaining-product slice — 2026-10-10

Sentinel and development tools now reuse the existing profile/archive lifecycles.
VS Code uses six scoped JSONC fields and a pinned offline-discoverable theme.
Explicit migration detaches old monolithic profiles through ordinary publication
journals before normal selection/adoption. No second migration engine was added.
Windows Terminal will support the managed stable unpackaged instance only; Store,
Preview and Canary remain personal installations. Preserve any existing nonempty
default-profile preference; choose managed PowerShell 7 only when unset. This
removes edition adapters and implicit preference changes while preserving theme,
font and shortcut outcomes. Desktop/font publication and native full-product
acceptance remain in progress; see installer-status for the current evidence.
