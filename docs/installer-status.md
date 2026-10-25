# Interactive installer status

This file records implementation decisions and dated diagnostic evidence.
Final commit, check results and independent review are recorded on
[PR #87](https://github.com/luisgui1757/dotfiles/pull/87). Readiness requires
all 23 native/quality checks, four CodeQL checks and Opus 5.5 xhigh approval
for the committed candidate.

Hosted acceptance expansion (2026-10-20, **implemented and independently reviewed**): the existing core
matrix now starts with a public keyboard-driven install/Check/Update/Repair/
Remove/Reinstall journey and exact personal-file preservation. Agent acceptance
adds installed Sentinel discovery, actual helpers and skill recovery/lifecycle;
desktop acceptance adds Code edit/save/close/reopen. Committed-head execution
results are recorded on PR #87; all 23 native/quality and four CodeQL checks plus
full local make ci are required before claiming these new assertions passed. The owner declined dedicated VM CI infrastructure; actual
reboot and first-permission flows remain manual coverage, not a pending CI setup.

Windows timing correction (2026-10-20, **implemented and independently reviewed**): the first hosted
candidate's ordinary Windows race suite exhausted its unchanged 20-minute limit.
Inspection now reuses parses within one unchanged Terminal settings document;
byte/error equivalence and allocation regressions preserve the same validation,
approval and personal-data rules. Full local make ci passes. Final native timings and all exact-head gates are
recorded on PR #87 and remain required before delivery. No timeout or test
parallelism changed.

Native Zen follow-up (2026-10-19): official cask installation, old private-browser
handover, normal desktop launch, unchanged Update, missing-app Repair, removal,
reinstallation and interrupted public recovery pass. Repair preserves 225
personal entries; later browser use grows that baseline to 227, all preserved
through removal and personal-app reuse. Native acceptance confirms the signature
of the real vendor-updated browser and detects another authenticated account's
running copy. New formula and cask commands correct inherited `HOMEBREW_NO_EMOJI`
attribution while preserving exact schema-3 recovery. Full upstream-Nix retirement,
outside-consumer refusals, reboot and repeated migration pass, with 793 state and
profile entries unchanged by repeated migration. The corrected fresh fixture now
also completes unchanged v0.4.4 setup in one run, profile handover, Homebrew
conversion, replacement setup and all 31 approved runtime choices. Reboot,
fresh TTY login/tmux and repeated migration pass; all 766 checked state/profile
entries remain unchanged. Gatekeeper and SIP stay enabled after the owner's
normal known-developers permission. Earlier failed guests and evidence are retained.
Dedicated Mac acceptance is complete; final committed-head review and all 27
hosted checks remain delivery gates. Personal-Mac migration still requires its
own backup and review of the actual migration preview.

VM acceptance completed (2026-10-18): Linux and Windows passed fresh installation,
update, repair, removal, reinstallation and configured application checks. Released
v0.4.4 migration passed on Linux; Windows completed all 27 approved retirements,
while correctly protecting modified PSFzf and outside-bucket LazyGit. Final public
Checks are ready for Linux's 91 and Windows's 55 resources with unchanged state.
Both reinstalled Neovim consumers pass 257 strict checks without changing their
managed runtimes. Personal files, later shell edits and Windows Code permissions
survive the lifecycle; the original failed Windows stage and journal remain intact.
Full local `make ci` and actual Opus 5.5 xhigh review pass. Final-commit hosted check
results are recorded on PR #87. Dedicated Mac results follow; authenticated provider,
Tailscale enrollment and microphone/paste workflows remain user setup, outside
the installer acceptance scope clarified by the owner on 2026-10-19.

Mac acceptance (2026-10-19) has completed fresh lifecycles and authentic
Determinate retirement on sequential disposable Tart 2.40.1 Apple Silicon guests,
macOS 26.6.2 build 25G83. Full released migration now passes. The preserved Code-blocked checkpoint
was restored; Screen Sharing resolved Tart input failure, and the owner-approved
known-developers setting allowed Microsoft's notarized app to open normally.
The released installer then exposed historical prerequisite defects, recorded below. The public interactive bootstrap
provisioned previously absent CLT and Homebrew. The original 25-tool installation
and current 27-selection installation, unchanged Update, explicit edited-settings
Repair and read-only Check pass. Strict installed Neovim passes 257 checks with
33,232 runtime entries unchanged. Git divergence rebases and personal Git writes
leave the checkout unchanged. Ghostty fresh login and real tmux/Herdr panes work;
AeroSpace's normal Accessibility grant, visible tiling and login item are verified.
Code editing/saving and Neovim's native guest-pasteboard round trip pass. Remaining
application interaction is limited by Tart mouse/shortcut automation; provider
authentication and Tailscale enrollment are not claimed. Reboot exposed the
Homebrew ownership bug and GUI launch exposed the DMG signature bug detailed below.
Their native regressions pass and are wired into the existing Mac CI jobs.
An Opus-reproduced old interrupted-intent preview failure now stays local to
Homebrew, preserving independent choices and saved evidence. Corrected fresh
public bootstrap and actual reboot now pass, including the changed device number
and unchanged full state inventory. Full Remove
completed through public recovery after the root-started Tailscale service's
ownership caveat; Code/Sentinel/skills Reinstall is Ready. All 37 personal/failure
witnesses retain their contents and permissions, with original evidence archived.

The owner-requested current Sentinel release is pinned at
`6645688d08f1505595cae276832a5a5070b82160` (version 0.2.0). It supplies
four policy targets and an independent 12-skill selection for Codex and Claude.
There is no separate Helix repository dependency; `helix` and `setup-helix` are
current Sentinel skill names. Native installation, exact copied bytes/modes,
actual interrupted publication/resume/replan, edited-copy Repair and both hosts'
skill discovery pass. Hidden recovery copies do not duplicate discovered skills.
The installed model-discovery helpers and pinned Impeccable engine setup/context/
detect run successfully without a provider turn on this exact snapshot.
The current pin changes six Impeccable/Unslop files. All 206 installed files match
its bytes/modes; the installed engine and new browser crypto fallbacks pass.
Codex and Claude each rediscover all 12 skills exactly once at this revision,
with the personal skill retained; read-only Check preserves all 10,237 entries.
Actual Opus 5.5 xhigh found no material issues in the Sentinel implementation after
its ownership corrections. Full `make ci` passed the subsequent Mac fixes and pin.
A later Opus review found that a released old policy receipt blocked reinstalling
the new target inventory. The correction now passes failed-before/passed-after
regressions and the actual public old-removal/new-adoption/removal workflow.
All 29 earlier policy/evidence entries and 12 retained-path disclosures survive.
Renewed Opus 5.5 xhigh review has no material findings; full `make ci` passes.
Final-commit hosted results are recorded on PR #87. Full released Mac migration
now passes in the dedicated guest. Onboarding, provider authentication, Tailscale enrollment and physical
audio/shortcut workflows are user setup, not installer delivery blockers.

Shell verification correction (2026-10-18): the earlier local "system-zsh
prerequisite" failure was a test harness bug. zsh was installed through Linuxbrew,
but the PATH regression discarded its directory and later hardcoded /bin/zsh.
Every probe now retains the executable discovered on the original PATH, including
nested macOS login shells. A process-boundary regression delegates to real zsh
through a path containing spaces and requires every probe to use it. It fails
before the correction and passes afterward; no PATH assertions are relaxed.
Full-gate results, native macOS verification and fresh independent approval are
recorded against the final candidate on PR #87.

Agent test process correction (2026-10-17): the final 6b69e8b matrix passed
26 checks but failed Windows Codex startup with retained stdio and a locked
private .codex directory. Following public migration correctly refused the
unfinished fixture's setup state. A deterministic native child reproduced both
symptoms; the new private Windows job launcher passes that regression and
failure/environment/cancellation cases. It reaps exact owned members before
requiring EOF and removing private homes. Actual application and following
migration verification, the final matrix and fresh Opus approval remain required.
Opus's two low Linux findings also have failed-before/passed-after regressions:
vanished pidfd identities and re-exec fixtures surviving a killed test owner.

Linux consumer-scan correction (2026-10-17): independent Opus 5.5 xhigh review
reproduced non-main-thread exec hiding continuously open Nix descriptors, despite
all 27 hosted checks passing on 3082f6b. A subsequent consultation proved complete
procfs reads using the descriptor directory cursor and a later positive FDSize;
rewinding the same maps handle also detects a destroyed address-space view.
The Go subprocess helper now applies those proofs, retains inspection handles,
and uses pidfd readiness for whole-process exit. Zero-thread observations refuse
removal; kernel threads and Z/X with exactly one thread retain explicit proofs.
The previous live-state/PF_EXITING inference and shell scanner are removed.
Deterministic handover/maps tests, 3000-scan native stress, retained migration
regressions and disposable root tests pass locally. Final exact-head native CI
and independent approval remain required and are recorded on PR #87. The original
ARM gate's unlogged process interleaving remains unknown.

Windows snapshot correction (2026-10-17): native diagnostics reproduce successful
short process lists during ordinary exits on both Windows Server and the pinned
container. Inspection now waits for a complete list within its original five-second
cleanup budget, retaining the original lifetime baseline and exact process HANDLE
checks. Overflow, API failures, invalid data and new members still fail closed.
The earlier compiler failure's unlogged counts remain unverified; its standalone
50-observation lifecycle did not reproduce it. Native regression evidence and
final-head checks/review are recorded on PR #87 and in the review ledger.

Current owner-requested changes (2026-10-17): T3 Code uses fixed stable v0.1.0
standalone archives on all four supported targets, now that Orchestrator V2 is
stable. The earlier nightly decision below is historical. All four downloaded
archives match GitHub asset digests and upstream SHA256SUMS, with the bundled
client/native modules retained. An isolated Linux amd64 smoke serves the real
frontend, initializes the V2 database, and verifies the full package remains
unchanged. The real Linux archive controller also passes nightly-to-stable
package Update, old-generation cleanup, Check, unchanged Update and removal,
preserving a personal witness. The native application lifecycle now passes on
macOS, Windows and both Linux architectures at candidate ae96c42. Migration of
actual nightly conversation state remains unverified for these bytes.

Tool selection and removal now use category checkboxes with indented independent
children and a mixed `[-]` state. Categories come from resources.json after
platform/removal filtering and never enter saved selections or the dependency
graph. Adoption, retention and shared-removal prompts remain explicit per-item
choices. Git defaults is labeled **Git**; Repository development checks is labeled
**Lint and benchmark tools** under **Dotfiles development**, with their exact
purpose documented in README. The subsequent owner request adds rebase-on-pull,
scoped Git defaults and removal of the earlier runtimes through migration. Git's
real write-through regression and focused handover/preservation tests pass;
runtime retirement is implemented; the final native matrix is an acceptance gate.
The preceding installer format/vet/race checks passed, along with
focused catalog/controller and actual PTY regressions. The broader local make ci
retains a host system-zsh prerequisite failure; its unchanged PATH test passes
in a disposable container. Consolidated evidence, Opus 5.5/xhigh-requested review
and the existing POSIX Git write-through finding are recorded in the dated ledger.

Consolidated migration evidence (2026-10-17): a fresh immutable v0.4.4 Linux
installation completes configuration migration, eleven selected replacements,
Linuxbrew login-shell repair, and removal of Nix, chezmoi and verified old tools.
Both final checks report ready, a real new login succeeds, and personal Pi/Nix
witness files remain unchanged. A separate real Determinate 3.23.2 no-init
installation passes the checked-in native lifecycle in 14.28s, including outside
account refusal, preserved system configuration and build-account removal.
Interrupted upstream store deletion also recovers after a fresh approval;
new store members and a held native deletion lease block that recovery.

The root default-profile exemption now validates literal installer-package
manifests, including old generations, against the pinned upstream distribution
or validated vendor receipt. It does not exempt arbitrary root packages by name.
Completed Opus implementation passes informed the integration. The account reset
allowed a further Opus 5.5 xhigh consultation on the native Mac ownership result.
The separate final independent review and all 23 native/quality checks
at the current PR head are delivery gates, recorded on the PR.
Windows's full core job passes at ae96c42, including its native package and
migration fixtures. The real Linux daemon migrations also pass on both
architectures. Determinate service shutdown now has an additional passing ARM
systemd lifecycle and a fresh amd64 no-init lifecycle with a real outside process
holding the store open; it blocks deletion until that consumer closes.

The immutable v0.4.4 Mac setup now completes using contemporaneous official tap
snapshots. Its old Homebrew cannot reliably parse newer cask records on the
runner. Conversion now fingerprints the records directly and restores only the
reviewed Homebrew repository, avoiding the general installer's global update.
Full native Mac migration passes at diagnostic `08f2323` (run `39029433400`),
including replacement health, Nix volume/integration removal and AeroSpace launch.
The subsequent Determinate test identified stale scratch-profile roots below
root's private temporary directory. A read-only privileged lstat now distinguishes
verified absence from live roots and inspection failures; focused race tests pass.
The native vendor copy also preserves the installing user's uid, confirmed at
`994a6ed` / run `39030949968`. Opus recommended exact release authentication and
private root staging. The implementation pins all 72 published Apple Silicon
assets in the reviewed range, verifies copies before exec, and never executes a
writable source during preview. Generator reproduction passes in 122.466s;
focused staging, tamper, saved-command and recovery race tests pass.
The complete real Determinate Mac lifecycle passes at diagnostic `0651b65`
(run `39033805907`, 180.73s), including outside-process refusal, same-journal
recovery, private-file cleanup, personal-configuration preservation and refusal
of a new personal service file. The current private-staging lifecycle also passes
on fresh Debian amd64/no-init (20.69s). Focused migration race tests pass (8.635s).
Diagnostic runs establish these behaviors; the final-head matrix and independent
review on the PR determine delivery readiness.

The independent Opus 5.5 xhigh review of `0a766f5` requested two corrections:
unloaded systemd units could strand interrupted recovery, and read-only Git
inspection missed worktree filters and plugin checkout settings. Both have
failed-before regressions and passing focused migration race coverage. History
inspection also refuses partial clones after a real missing-tree fetch reproduction.
The native missing-socket case exposed an activation link the vendor leaves;
its removal now requires an exact snapshot recorded before approval. The real
ARM systemd lifecycle passes at `116f74c` (run `39036444754`, 28.56s), with
the missing socket, complete recovery and exact activation-link removal. The
final native matrix and independent re-review are required for the corrected head.
Plugin inspection recognizes the released tmux-resurrect checkout's empty test
submodule while preserving populated or cached/deinitialized data. Its failed-before
regression now passes with all focused migration race tests (7.990s).

The next independent Opus review of `d5f4877` accepted the Git correction but
found that systemd preserves failed state after stop, and that a partial vendor
revert can also retain the service enablement link. Failed-before regressions
reproduce both. New stop recipes recognize failed as stopped while preserving
historical inactive-only checks; socket and service links share the same exact
saved-proof requirements. Native failed-service recovery passes on disposable
ARM systemd at 11d1ead / run39038168571, including all activation-link removal
and personal-data protection. The final full gate and independent review remain
required for this correction.

Opus subsequently accepted the failed-unit and plugin fixes at 584c07e, but
found systemd can resolve the upstream-flavoured vendor service link directly
into the store. Recognition now requires the exact regular unit resolved from
the installation, with shared rules for upstream and vendor paths. Upstream
removes those activation links before their source; interrupted deletion and
vendor leftover cleanup retain exact proof. Both native vendor distributions
are now included in the existing opted-in Linux lanes. Diagnostic 7bcb389 /
run39040018722 passes both variants on real ARM systemd, including full recovery
and preserved configuration. Final-head CI and independent approval remain gates.

Opus approved 9584a39 with three nonblocking Low findings. Those are also
addressed: a shared ordering helper removes resolved activation links before
both fresh and recovered deletion, including after re-enablement and a second
interruption. Unproved old/dangling links retain the conservative refusal with
ownership-qualified instructions for unlinking only the named activation link;
non-link files retain a clear preservation explanation. Focused regressions
reproduce all three before correction. The updated candidate still requires
its own independent review and complete hosted checks. Opus subsequently
approved d27c927 and confirmed those corrections, while identifying one advice
scope issue: primary unit links could receive the activation-unlink instruction.
An exact parent-directory guard now limits that advice to the two systemd wants
directories; the three primary paths have failed-before regression coverage.

Opus approved 3beb054 with no material findings. Its native Debian checks then
exposed a test-only assumption: the no-init upstream receipt never owned a
systemd service, so a new personal service correctly reaches the generic
preservation guard rather than the saved-vendor-file guard. The native test now
requires Pending with a path-specific preservation explanation, then attempts
the approved operation and requires needs-action with the same protected path.
It checks unchanged personal file bytes after both preview and application.
After fixture cleanup, an explicit retry must complete the refused transaction.
Production behavior is unchanged; final-head checks and review still apply.

The final native run at 14ee10d exposed a separate race in the interruption
fixture's observer: it read a transient .state-* staging file as a committed
retirement journal while saveDocument renamed it. The observer now uses the
same operation-ID-plus-.json filename contract as production recovery. Partial
and complete unpublished staging documents both have failed-before coverage;
committed dispatch detection and malformed committed-journal failures remain
required. Production publication and recovery logic are unchanged.
Concurrent replacement of a committed journal also reproduces the reader's
explicit changed-inode retry result. As in the existing interruption fixtures,
the observer polls again within its original deadline only for that exact
diagnostic; malformed documents and other read errors remain fatal.
Opus approved c54a620 with a Low coverage gap for those other read errors.
A committed-name directory now explicitly verifies that a non-file journal
still fails; the existing malformed-JSON case separately covers decoding.
The 697d5e3 native matrix passed all real migration lifecycles, including both
Mac variants, but its Mac core suite exposed a Linux-specific /bin/true path in
the new observer fixture. Its harmless worker now uses the established portable
/bin/sh command pattern; fixture failures also retain the worker diagnostic.

Native closure (2026-10-13): `5657dc5`, macOS Desktop job `115406187261`,
passes the complete desktop lifecycle (289.44s), including Zen
(9.75s) with reported ARM64 architecture, an owned visible window, pre-update
Check, unchanged-pin Update without adoption, removal and personal preservation.
VS Code, Ghostty, AeroSpace, OpenWhispr and Obsidian also pass. This closes the
published-launcher startup defects; it does not prove default-browser/Dock routing
or a revision-1 to revision-2 launch-cache transition. Final-head required checks
remain the release-readiness authority.

Current correction (2026-10-13, second native diagnosis): head `794fc39` proves
Zen's published script bundle runs as X86-64 (translated) and stalls in Rosetta's
loader; the original vendor app and direct wrapper both create owned native
windows. Apple documents the script-only default. Preparation revision 2 declares
`LSArchitecturePriority = [arm64]` on the outer bundle while retaining the signed
vendor payload and profile environment. A failed-before/passing-after metadata
regression and saved revision-0/1 coverage accompany the fix. The hosted GUI test
also requires native ARM64 launch metadata; its rerun remains the acceptance gate.
Linux post-reboot desktop launch, restored tabs, page rendering and interactive
Check passed with an unchanged installer-state hash. Current required checks are
tracked on PR #87; earlier head results below are historical evidence.

Earlier correction (2026-10-13): macOS diagnostic head `4451fd1` reproduces the
wrapper's `Couldn't load XPCOM` failure while the identical signed vendor binary
renders successfully. Preparation revision 1 now resolves the executable's
physical path before exec; Mozilla's Darwin path cleanup corrupts `../` components.
Focused failed-before/passing-after proof preserves the vendor bundle, arguments,
working directory and saved revision-0 recipe. Native head `8fb7878` confirms
both wrapper and vendor headless rendering, but its LaunchServices completion
still times out. The GUI compatibility defect remains open while normal desktop
launch and live owned-window observations are compared; headless success is not
GUI acceptance.
Head `70adf06` passed 22/23 required checks and CodeQL; the path-fixture fixes are
confirmed on macOS and Windows, with macOS Desktop lifecycle the sole failed required job.

Earlier hosted continuation (2026-10-13, `281f127`): macOS Zen failed the existing desktop
launch smoke (LaunchServices reported no running application/window). The cause
is not established; the unsigned wrapper diagnostic alone is not a signing-root-
cause proof. The later diagnostic and correction above supersede that uncertainty. Windows quality
also exposed noncanonical temporary paths in three new fixture entrypoints;
those fixtures now use the existing native path resolver without weakening the
production guards. Matching final-head hosted checks remain required.

T3 continuation (2026-10-13): optional **T3 Code (V2 nightly)** uses the official
standalone archives and browser UI on all four supported targets, per owner
choice. The explicit `v0.0.46-nightly.20261013.3053` pin is necessary because
stable v0.0.45 predates V2. Codex and Claude Code stay independent; T3 owns provider
setup and orchestration. No provider authentication or background service is added.
Linux T3 install, browser/project/terminal use, Check, unchanged-pin update,
removal and reinstall passed, including byte-for-byte preservation of 23 T3/project
files. Cross-provider agent execution remains blocked by absent guest authentication;
other platforms and genuine version transitions are unverified locally.
The [current review entry](reviews/2026-10-08-interactive-installer-overhaul.md#2026-10-13--t3-code-standalone-v2-nightly) records runtime evidence and gaps.

Current desktop continuation (2026-10-13): independently selectable OpenWhispr,
Obsidian, Tailscale and Zen. macOS uses reviewed archive bundles plus the approved
Homebrew Tailscale CLI. Linux used approved private Obsidian Flatpak (replaced on
2026-10-22 by the official portable build without Chromium's sandbox; the Flatpak
only removes earlier installations), subsequently
approved official OpenWhispr amd64 portable archive, Tailscale APT and Zen archive.
OpenWhispr has no published Linux arm64 asset. Windows omissions follow the owner's
explicit channel exception. Native macOS additions and final-head hosted checks
remain unverified until the PR's matching jobs pass; prior green heads do not
certify these changes.

Linux acceptance uses one isolated Ubuntu 26.04.1 amd64 GNOME VM (4 vCPU, 8 GiB,
64 GiB sparse disk; nested KVM explicitly approved). Setup is driven by actual
keyboard input in Ptyxis, with applications launched through desktop entries.
The published base was fetched and verified as `9c3a0aaf2f3c6d7c327d8ee5f39fe0e4ffec694d`;
the tested changes are kept in one squashed commit. Earlier desktop publication
was blocked by GitHub connector HTTP 403 and unavailable HTTPS Git credentials.
The T3 continuation verified the owner's existing SSH Git authentication. Final-head
hosted checks must be read from PR #87; earlier green runs do not certify this work.

Verified desktop smoke: Obsidian vault editing/rendering and subsequent reopening of the
same personal note; Zen browsing, URL forwarding and retained profile/bookmark
across generation replacement; active Linux DisableAppUpdate policy; unchanged
read-only Check state; Tailscale interactive install, unauthenticated daemon
readiness and separate machine-wide removal. OpenWhispr installs and opens its
account-free graphical onboarding. Dictation is BLOCKED by absent VM audio input;
Wayland auto-paste's upstream ydotool/input-access setup is NOT TESTED. No permission
or kernel/AppArmor workaround was applied. The upstream portable launcher's
no-sandbox behavior on restricted-userns Linux is disclosed before approval.

Fixed smoke/review defects: Zen URL field code, installation lock/updater mutation
and changing profile identity; Flatpak native ref parsing, wrong-provider desktop
verification, original-versus-current receipt comparison, private runtime lookup,
portal renderer startup and special-character Exec escaping; Tailscale personal
conffiles wrongly triggering endless repair. Failing snapshots are retained.
The actual Opus 5.5 xhigh review and focused follow-up found the conffile/runtime
and escaping cases; native evidence confirmed or delimited each accepted finding.
macOS Zen self-updates/default-browser routing remain an unverified platform risk;
changed bundles remain protected, and no policy was injected into signed bundles.

The final portal-source local `make ci` gate passed: Go format/vet/race
(369.514s), 51 bootstrap tests (two environment skips), and available configuration
checks. Optional runtime checks for Neovim, PowerShell, lsd, Starship, tmux,
Ghostty and Renovate were skipped because their executables were unavailable in
this contributor environment; the hosted matrix remains required for those gates.
Foreign test compilation passed for darwin/arm64, Windows/amd64 and Linux/arm64.
The real Tailscale conffile lifecycle passed in 14.53s. The first native Flatpak
install/sandbox/check/update/remove/reinstall passed in 113.36s; the expanded
renderer-portal lifecycle passed in 115.52s after the graphical failure was found. These
are unchanged-version/channel-commit updates, not genuine version-upgrade proof.
Final interactive removal removed all three application launchers and healthy
owned payloads. The personal vault note, OpenWhispr witness, Zen profile bytes and
bookmark matched their pre-removal hashes/content. Explicitly retained libraries,
Flatpak runtimes and earlier modified payloads remain disclosed and protected.
Reinstallation through the same UI completed ready. Obsidian reopened the edited
vault note; Zen reused its profile and opened the supplied URL; OpenWhispr returned
to its saved onboarding. A final interactive Check passed with byte-identical
installer state. Exact-head hosted results remain separate until recorded.

Current delivery (2026-10-12): the owner authorized squashing all implementation,
smoke fixes and CLI replacement changes into one meaningful commit in PR #87.
Prior commit-specific results remain dated evidence; new-head checks still apply.

Current change (2026-10-12): the owner retired Pi entirely and selected Claude
Code/Codex CLI as independent replacements. The existing `6a899da` head passed
all 23 canonical checks and CodeQL; that evidence does not certify this new change.
New pins, agent lifecycle tests and current validation are recorded below.

**Delivery candidate; readiness follows the exact PR head.**
Updated 2026-10-12. This document records scope and dated evidence. Current
readiness comes from [PR #87](https://github.com/luisgui1757/dotfiles/pull/87):
all 23 required native/quality checks and independent review must pass before it
is ready. Dated results below certify their stated source, not a later head.

**Current correction batch:** `45b8b38` passed all 23 native/quality checks and
CodeQL. The owner-authorized required-check transition is applied and read back
in both layers. AeroSpace's actual tiling/configuration/focus smoke also passed
after owner-granted Accessibility consent. The subsequent independent Astra and
Opus 5.5 xhigh review accepted three behavior defects (Mason pinning, published
Neovim startup mutation and nested-login PATH priority), the POSIX public-bootstrap
coverage/umask gap, and original-engine recovery guidance. All five corrections
are implemented. Their first complete local `make ci` gate passed: 463 Neovim assertions,
49 bootstrap tests, Go format/vet/race and configuration/shell/policy/Renovate gates.
All six foreign build/test-compilation checks pass. Astra found no material
correction findings. Actual Opus correction and follow-up reviews found no further
confirmed production defects; their minor fixture/documentation findings are
resolved. The amended-head hosted matrix must pass before delivery readiness;
current results are recorded in PR #87. The historical failures below retain their
original results and do not describe new failures at `45b8b38`.
The first amended `a8260cd` attempt found a Windows Terminal fixture readiness
assumption and a separate runner Docker-service prerequisite failure. The fixture
now distinguishes shell readiness from a visible owned window under the unchanged
deadline; the original failures remain recorded. Full final-head native acceptance
continues to gate readiness. All three POSIX public lifecycles (macOS arm64 and
Linux amd64/arm64) completed their runtime
assertions but failed disposable read-only Go-cache teardown; the fixture cleanup
is corrected without changing production permissions. The `a8260cd` Windows public
lifecycle was cancelled partway through and is not a pass. All four `a8260cd` Neovim
lifecycles passed their 257 language checks, update/shared removal and final
personal restoration. Those dated results do not certify a later amended head.

The frozen `4380ae9` local `make ci` gate passed in 654.471 seconds with all 766
source files unchanged; its six foreign build/test-compilation checks also passed.
Windows Desktop passed its full native lifecycle after one window query, which
does not establish the original zero-handle failure's cause. A Windows Neovim UI
fixture separately required a POSIX-only bundled parser filename. Its correction
uses upstream's `parser/lua.*` lookup and passed all four real UI cases on a native
Windows diagnostic runner. Final canonical Windows quality and complete native
checks remain required for the final amendment.

The subsequent `f02de9f` full local gate passed in 643.554 seconds with unchanged
source. Native Windows quality then required test-only corrections for UTF-8 path
decoding, observed compiler context and both PowerShell native-error modes. Their
exact bytes pass all 50 Python and 53 Pester cases plus analyzer checks on Windows;
scoped evidence and failed diagnostic-job distinctions are recorded below.
Fresh vendor jobs now explicitly prepare the host's existing Docker service under
bounded readiness before the unchanged single-attempt container lifecycle. Native
Windows Server 2022 proof covers stopped-service startup and running-service reuse.
These fixture/host-preparation changes have no production installer delta. The
complete required matrix on the final published head remains the readiness gate.
All four `f02de9f` public bootstrap/migration lifecycles ultimately passed, including
native Windows PowerShell 5.1. The later `d83ab95` vendor jobs exposed a separate
CI probe defect: an arbitrary five-second `docker info` cap expired while the
service was Running. Readiness now queries the actual server OS through `docker
version`, without client-plugin discovery, under the original 60-second budget.
The earlier stopped-service diagnostic had first invoked `docker info`; its pass
did not prove a cold-client path. The corrected probe passed first-invocation
native proof on separate running/stopped Windows Server 2022 hosts in 2.451/2.962
seconds; both complete diagnostic jobs passed. Final canonical vendor acceptance
remains required.
The `d83ab95` Windows Desktop lifecycle separately failed image inspection during
cleanup after VS Code confirmed its settings. A real held-process regression
reproduced the same ERROR_GEN_FAILURE after exact process termination. Cleanup
now waits passively on that same handle after any image-query failure and accepts
only confirmed completion; it never signals an unvalidated executable. Live,
outside-root and canceled cases retain their existing protections. The corrected
native seven-case regression and complete Desktop lifecycle passed in 10.25 and
183.38 seconds. Final-head canonical acceptance remains required.

[PR #87](https://github.com/luisgui1757/dotfiles/pull/87) is the single delivery PR
and retains one implementation commit. #88 is incorporated and closed; #86 is
superseded. Published checkpoint `044f9780705faa996914ed95de2aed2126802ba9`
passed the full local gate, all six foreign all-package build/test-compilation
checks and actual Opus v7 review without material defects. Its hosted matrix
passed **19 of 23 jobs**; all three CodeQL analyses and the aggregate passed.
The four failed contexts remain recorded as failures: Ubuntu completion-directory
permissions on the hosted image, a PowerShell helper noun, the Windows fixture's
missing explicit shared-PowerShell removal choice, and Windows compiler-query
accounting lost across an unsafe API pointer boundary.

Each correction has native scoped proof. The Ubuntu helper and complete unchanged
shell suite pass on amd64 and arm64. Actual PSScriptAnalyzer, module binding and
complete shared-shell lifecycle pass on Windows. The corrected Windows API
boundary passes real kernel output/limit checks, the complete process-lifetime
suite and six compiler-reuse runs. Exact HANDLE waits, late-member refusal,
outside-process protection and original time bounds remain unchanged. Those
proofs certify their frozen sources; the dated ledger preserves diagnostic
failures and non-reproduced claims. Final readiness requires the consolidated
local gate, independent review and every canonical native/quality job plus
CodeQL on the exact amended PR head.

The next published checkpoint, `127e35d676c69a65068b55e15c0c3508c6a7f3dc`,
passes **21 of 23 native/quality jobs** and all CodeQL analyses plus the aggregate.
All eleven Linux jobs, six macOS jobs and Windows core, quality and both fresh
toolchain jobs pass. Windows Desktop fails in process cleanup; its correction
has complete separate native proof recorded below. Windows Neovim fails final
Git payload inspection before its strict language smoke. Its failure-only native
diagnostic passes the complete Windows lifecycle and all 257 language checks
with the same inspection guard. The original entry, predicate and cause remain
unproved. Delivery adds bounded failure metadata for a future refusal.
Neither failed job is relabeled by a scoped pass or a later source.

## Scope and roadmap

Supported targets are macOS arm64, native Windows amd64, and Ubuntu/Debian on
amd64/arm64. Linux native acceptance uses Ubuntu 26.04 and Debian 13. WSL uses
the ordinary Linux lifecycle. Fedora, macOS Intel and separate WSL provisioning
are outside this release. Nix and chezmoi are removed, with explicit migration
and passive historical live-link sources instead of a compatibility runtime.

| Milestone | Implementation | Acceptance contract |
|---|---|---|
| 1. CLI tools and prerequisites | DONE: all providers, independent fd/ripgrep choices, APT, Homebrew and Windows toolchains connected | Native install, runtime use, update and surgical removal, including fresh provider/toolchain jobs |
| 2. Neovim | DONE: Mason registry/tool pinning and read-only published startup review corrections; local regressions pass | Repeat full selection, real language use, update, shared removal and preservation on all four targets |
| 3. Product integrations | DONE: desktop apps/fonts, scoped settings, Sentinel and development checks | Native desktop/font and configured GUI lifecycles on every supported target |
| 4. Public cutover | DONE: POSIX public-launcher coverage, caller umask and original-engine recovery guidance; local regressions pass | Real public bootstrap/migration lifecycle on all three OSes; Windows PowerShell 5.1 already passed at the previous head |
| 5. Delivery | IN PROGRESS: correction verification in the single PR/commit; required-context transition DONE | Green final-head native checks and independent correction review; merge/release remain owner actions |

The design is settled. Additional infrastructure needs a demonstrated defect or
an actual supported-tool requirement. Focused regressions precede the integrated
gate; unchanged passing areas are not repeatedly redesigned. Local mocks and
cross-compilation do not substitute for real native lifecycle acceptance.

## Checkbox coverage

Every selection below has all provider routes connected. This records product
scope and implementation, not a claim that its final native acceptance passed.
A dash means unavailable by design.

| Checkbox | macOS arm64 | Ubuntu/Debian | Windows |
|---|---|---|---|
| Neovim | Connected | Connected | Connected |
| VS Code | Connected | Connected | Connected |
| zsh | Connected | Connected | — |
| PowerShell 7 | — | — | Connected |
| Starship | Connected | Connected | Connected |
| lsd | Connected | Connected | Connected |
| zoxide | Connected | Connected | Connected |
| fzf | Connected | Connected | Connected |
| ripgrep | Connected | Connected | Connected |
| fd | Connected | Connected | Connected |
| tmux | Connected | Connected | — |
| Ghostty | Connected | Connected | — |
| Windows Terminal | — | — | Connected |
| AeroSpace | Connected | — | — |
| Mac keyboard shortcuts | — | Connected (X11 sessions); native mapping lifecycle verified on amd64 and arm64 (`67aae3b`); navigation mirror (Option(+Shift)+digit, Ctrl+Option+H/L) verified on amd64 and arm64 (`60a3148`); reload after a checkout update pending its first hosted run | — |
| Herdr | Connected | Connected | Connected |
| Git | Connected | Connected | Connected |
| lazygit | Connected | Connected | Connected |
| GitHub CLI | Connected | Connected | Connected |
| gh-dash | Connected | Connected | Connected |
| T3 Code | Connected | Connected | Connected |
| Claude Code CLI | Connected; native lifecycle verified | Connected; native lifecycle verified | Connected; native lifecycle verified |
| Codex CLI | Connected; native lifecycle verified | Connected; native lifecycle verified | Connected; native lifecycle verified |
| GitHub Copilot CLI | Connected; native lifecycle pending first hosted run | Connected (glibc); native lifecycle pending first hosted run | Connected; native lifecycle pending first hosted run |
| Kiro CLI | Connected; native agent lifecycle verified (845859a) | Connected (glibc); native agent lifecycle verified on amd64 and arm64 (845859a) | — (not offered: Kiro ships Windows only as an MSI installer; owner decision 2026-10-22) |
| Sentinel policy | Connected | Connected | Connected |
| Lint and benchmark tools | Connected | Connected | Connected |

Neovim provisions the locked notes plugin automatically but activates it only
for an existing vault. Notes are not a separate checkbox. Compiler/SDK packages,
fonts and runtime libraries are hidden dependencies of their consumers.

## Published evidence

Checkpoint `28c92f5c8c2ec0e0f68ad9616660454e5991b0b4` passed `make -k ci`
with unchanged source hashes, all-package Windows amd64 and Linux amd64/arm64
builds and test compilation, and the 684-file EditorConfig check.
[Installer run 38082752162](https://github.com/luisgui1757/dotfiles/actions/runs/38082752162)
passed 12 of 20 jobs; [quality run 38082752292](https://github.com/luisgui1757/dotfiles/actions/runs/38082752292)
passed macOS and Ubuntu, while Windows failed. All four fresh Ubuntu/Debian APT
lanes now passed complete Neovim installation from absent prerequisites. Both
Ubuntu Neovim lifecycles, fresh Homebrew, fresh VC++ runtime, fresh Build Tools
and three POSIX core jobs passed. The remaining eight installer jobs remain
failed; scoped successes inside them do not change their parent conclusions.

Windows core passed public setup/migrate in 71.76 seconds, native Rust with the
274-character stdlib path (94.73 seconds), LaTeX (98.76 seconds), Yamllint
(88.70 seconds), and repeated Build Tools inspection (29.86 seconds). Windows
Desktop passed font lifecycle and real Terminal Settings Save/check/removal;
VS Code shortcut preparation failed before the desktop application launch.
The separate temporary diagnostic branch creates no PR and certifies no required
check. Its exact head `48fbcc8b5e258298630680f8d6e6edc279e57035`,
[run 38085071529](https://github.com/luisgui1757/dotfiles/actions/runs/38085071529),
passed the corrected real PowerShell 5.1 extraction, COM shortcut and protocol
boundaries, but still failed process completion and Git's strict runtime probe.
Those failures remain open until canonical correction and native rerun.

The actual immutable Opus v4 review executed `claude-opus-5-5` with requested
`xhigh` effort (the provider did not expose effective effort), made 176 read-only
tool calls, and left all 829 source/evidence files unchanged. Prior D1/D2/D3 were
revalidated. New F1 active-shell coverage, F2 POSIX palette and F3 passive Windows
LazyGit hash coverage are accepted; all thirteen originally omitted E1 logs have
been verified against their original handoff hashes for the next review.


Checkpoint `077683f8bc45e1feb4c65824fe734876aec839ee` passed `make -k ci`
with unchanged source hashes, plus Windows amd64 and Linux amd64/arm64 builds
and all-package test compilation. The PR merge checkout has the same tree as
that published head. [Quality run 38076760170](https://github.com/luisgui1757/dotfiles/actions/runs/38076760170)
passed on macOS, Ubuntu and Windows. [Installer run 38076760167](https://github.com/luisgui1757/dotfiles/actions/runs/38076760167)
has passed both Ubuntu core jobs, macOS core, fresh Homebrew, fresh VC++ runtime,
and both Ubuntu Neovim lifecycles. Its other native failures remain open below;
Windows core failed its aggregate archive deadline, PortableGit preparation and
public bootstrap extraction; its actual Rust/LaTeX/Yamllint and Pi theme steps
passed. Those scoped passes do not establish a passing core job. The actual Opus review resolved prior findings and
identified the Rust identity and passive-source corrections now being integrated.

Earlier checkpoint `fbca6c224d03ac57597628464ba0bf92e94f2df7` passed `make -k ci`
(including the 438-second Go race suite) with unchanged source hashes. Its Windows
root-package test compilation passed; final all-package foreign compilation is
still required. Completed native runs at that exact head are:

- [Installer run 38071197318](https://github.com/luisgui1757/dotfiles/actions/runs/38071197318):
  9 of 20 jobs passed: fresh Homebrew, fresh Ubuntu/Debian APT on both
  architectures, fresh Windows VC++ runtime, both Ubuntu core jobs and macOS core.
- [Quality run 38071197325](https://github.com/luisgui1757/dotfiles/actions/runs/38071197325):
  macOS and Ubuntu passed; Windows failed its Git Bash bootstrap fixtures.

Passing steps inside failed jobs are narrower evidence. Windows completed native
archive/filesystem checks, VC++ inspection, LaTeX with forced cp1252 and Python
replacement, and Yamllint's lifecycle. Windows Desktop completed font lifecycle.
They do not turn their parent jobs into passes. Fresh APT passes at this checkpoint
did not yet exercise a complete Neovim installation; that requirement is added
in the current correction batch.

Earlier checkpoint `e1c987d` passed the full local gate and
[all four native engine jobs](https://github.com/luisgui1757/dotfiles/actions/runs/38053630162),
including archive updates, configuration/profile recovery, Windows process-death
and settings-lock cases, Pi application use, and Homebrew outside-consumer update.
That is historical engine evidence, not final product certification.

## Historical hosted failures and their correction evidence

| Area | Observed failure | Current state |
|---|---|---|
| Windows Build Tools | Query regressions at `28c92f5` return before exact owned child handles signal | Verified private-job handles now signal before inspection returns under one existing five-second bound. Exact native diagnostic `9ff8dd0` passes all five unchanged immediate-wait/outside-survival cases (1.69 seconds), plus real late-member refusal. Final-head native acceptance remains required. Fresh real Build Tools lifecycle already passed at `28c92f5`. |
| Windows Rust | MSVC could not open a 273-character physical stdlib path; unrelated installer rebuilds also changed desired identity | At `077683f`, real rustc compile/run, offline Cargo and Clippy, check and removal pass with a 274-character stdlib path. Desired identity now follows only the launcher recipe while exact copied-byte provenance remains. Native public rebuild/check proof is required for that latest change. |
| Windows PortableGit | At `077683f`, extraction succeeds, then Git rejects `GIT_CONFIG_GLOBAL=NUL` and CMD loses its self-deleting post-install input | Use Git’s `/dev/null` spelling and execute an unchanged, exclusively staged sibling script. Nonzero exits, unfinished cleanup and runtime failures remain fatal. Focused regressions and Windows compilation pass; actual CMD and Git/Make lifecycle reruns remain required. |
| Windows public bootstrap | Earlier archive-module extraction exceeded the public fixture deadline | Public setup/migrate passes at `28c92f5` in 71.76 seconds. Corrected ordinary extraction also passes under actual Windows PowerShell 5.1 on the diagnostic runner; final-head coverage remains required. |
| Windows quality | At `28c92f5`, strict process/extraction fixtures and real clangd document symbols failed | Extraction is corrected and native-proven; exact child-handle completion is under correction. Clangd now records bounded original request/client/stderr diagnostics without relaxing its deadline; the Windows failure cause remains unproved. |
| Apple developer tools | Preservation now passes; receipt directory `private` is wrongly treated as an outside payload file | The latest real BOM also names an outside installer temporary file. Validate canonical receipt paths and record existing outside files for exact preservation before any CLT move; historical absent entries grant no ownership. Traversal, malformed paths and failed queries remain fatal. Focused red/green and private macOS metadata regressions pass; actual missing-payload provisioning remains pending. |
| Windows Terminal | Earlier actual UIA Save serialized an equivalent `closeTab` object and lost ownership | At `28c92f5`, actual Settings Save changed bytes and mtime, followed by check/removal PASS (26.66 seconds). Windows font lifecycle also passed (22.91 seconds); the separate desktop-application step failed later. |
| Linux Neovim | Earlier bytecode-name and APT residual-state failures | Both Ubuntu Neovim lifecycle jobs and all four fresh Ubuntu/Debian APT plus complete Neovim lanes pass at `28c92f5`. Final-head native acceptance remains required. |
| macOS Neovim | At `28c92f5`, the native entry diff proves 21 added node-gyp Python bytecode entries inside the Node payload | Scope `PYTHONDONTWRITEBYTECODE=1` to sync/check children. Real pinned gyp invocation fails the full Node snapshot before and preserves it after; integrity checks remain unchanged. Complete macOS Neovim lifecycle rerun is still required. |
| Windows Neovim | v3 reviewed/fresh plans differ only in compiler consumers: absent → `vctip.exe` | Compiler inspection captures consumers before its probe spawns the surviving child. Isolate that query's process tree without relaxing consumer or approval checks. Windows native regression and full Neovim rerun remain required; no automatic apply retry. |
| WezTerm desktop | Earlier native startup failures | Removed from all OSes by explicit owner scope decision. Historical failures are not fixes or passes; remaining desktop apps still need native acceptance. |
| Windows desktop shortcut | At `28c92f5`, VS Code shortcut preparation rejected the native digest | Scoped PowerShell progress handling and separate read-only query streams have actual PowerShell 5.1/COM diagnostic-run proof. Complete configured desktop lifecycle is still required. |
| Review F1 | Shortcut preparation retry can strand operations | Operation/attempt-scoped preparation and existing journal-bound source recovery integrated. Durable failure/unfinished-result, hash and resume regressions pass; real Windows COM rerun required. |
| Review F5 | A newly created higher-priority Bash profile strands the recorded target | Reuse the receipt-validated baseline path. Fresh-controller install/update/removal, forged-evidence and interrupted-operation regressions pass; personal profile bytes survive. |
| Review F2/F3/F4 | Old TPM fallback, missing Linux Mason prerequisites and misleading Windows Make privilege | Corrections integrated with focused regressions. Fresh Ubuntu/Debian jobs now exercise the complete Neovim lifecycle from absent curl/unzip. |
| Review F6/F7 | Unused zprofile copies/test and inconsistent Go timeout | Remove dead copies; retain generated-profile behavior tests. Local gate now uses the same 20-minute Go budget as native CI. |
| Review D1/D3 | Unrelated installer rebuild invalidated Windows Rust; change checklist named a missing language matrix | Separate desired launcher revision from exact operation/version executable provenance; controller red/green preserves the generation and still detects installed tampering. Native public rebuild/check proof is added. Correct the documentation path. |
| Review D2 | Six passive released sources changed before migration | Restore their exact v0.4.4 bytes; inventory hashes protect all present recorded passive targets/profiles and referenced templates. Migration recognizes the restored PowerShell profile and still preserves personal edits; active shell, tmux and default LazyGit recipes remain independent. |

Publication requires a complete local gate and all-package foreign compilation;
final acceptance requires the native runs.
The [append-only review ledger](reviews/2026-10-08-interactive-installer-overhaul.md)
records exact regressions, current proof and rejected alternatives. The
[field findings](reviews/field-findings.md) index the problems found on the owner's
real machines that hosted CI missed, with their status and what to recheck. The published
local gate above proves `077683f`; later working-tree corrections need their own gate.

## Remaining delivery gates

1. DONE: accepted implementation corrections have failed-before regressions,
   native verification and a passing integrated `make ci` gate.
2. Require all 23 canonical quality/native jobs and four CodeQL checks on the
   exact final PR head. Current results belong on PR #87; older green heads do
   not satisfy this gate. Native package/font mutations run only on disposable
   runners.
3. DONE: actual `claude-opus-5-5` with CLI `--effort xhigh` reviewed the Mac and
   Sentinel corrections. The final Sentinel retargeting review found no material
   issues; its evidence notes are closed by the exact-test red run and complete
   old-history preservation proof.
4. DONE: both required-check lists contain the reviewed 23 checks, strict mode
   and Actions app `15368`, under explicit owner authorization. Preserve security
   and review policy.
5. DONE: genuine one-run v0.4.4 Mac installation, released profile handover,
   Nix-managed Homebrew conversion, upstream-Nix retirement, reboot and repeated
   migration. Ordinary Code permission was completed; Gatekeeper and SIP remain
   enabled. Earlier failure evidence is preserved; historical source is unchanged.
6. Retain one reviewed squashed commit. Merge/release and the documented physical
   Mac permissions, authentication and hardware checks remain owner actions.

AeroSpace configuration/tiling/focus passed the local owner-consented smoke. The
hosted fixture reports launch separately when consent is unavailable; it never
bypasses TCC. Archived pins are reviewed inputs: Update reconciles the current
checkout rather than silently fetching arbitrary upstream versions.

The obsolete Linux compiler-to-Make ordering edge is removed. Neovim still
requires both tools directly; APT owns build-essential's native Make dependency.
Compiler-first and Make-first controller fixtures verify update, selective Make
release and final native removal without catalog ordering workarounds.

The missing-payload Apple job preserves historical receipts, moved tool roots and representative payload files, developer siblings and Homebrew. It proves neither a never-installed Mac nor a same-version reinstall until the actual native result establishes that version relationship.

## 2026-10-10 — WezTerm retired by owner scope decision

The active catalog has 22 capabilities after removing WezTerm on macOS, Linux
and Windows. Its four resources, four platform archive pins, configuration,
launcher hooks and dedicated smoke tests are removed. Eight exclusive Linux
libraries are pruned; shared fonts/tools/runtime libraries and APT ownership
regressions remain. VS Code now exercises the public singleton desktop lifecycle
and personal-launcher adoption/restoration. Passive released WezTerm bytes and
discovery records remain unchanged, with no removal authority inferred from them.
The earlier native launch failures remain unproved historical failures, not fixes.
Focused validation is recorded in the review ledger; the remaining desktop
native lifecycle still requires the disposable hosted matrix.

## 2026-10-10 — APT reinstall after retained conffiles

The Debian 13 arm64 fresh Neovim job exposed a provider evidence error after the
preceding native fixture removed Git. dpkg correctly treats `config-files` as
absent payload but logs its remembered old version on the next `install` action.
The recovery parser now accepts that action without requiring `<none>`; native
success, exact operation attribution and final configuration remain required.
Focused successful/interrupted recovery regressions and a real disposable
install/remove/reinstall fixture cover retained personal conffiles without purge.
The full fresh Neovim hosted lifecycle remains a separate required proof.

## 2026-10-10 — Explicit native archive sweep budget

The native archive sweep now has an explicit 20-minute aggregate timeout. Windows
job `114285248186` exhausted Go's implicit 10-minute package budget while Python
was verifying its final removal, after its install/use/update/integrity checks.
Python itself had run for 98 seconds; the preceding 13 pins had already completed.
Individual command deadlines, all lifecycle assertions and the 60-minute core-job
limit remain unchanged. The complete archive sweep still needs a passing hosted
rerun. The same failed job separately passed real Rust compile/Cargo/Clippy and
Pi terminal-theme checks; those passes do not establish a passing core job.

## 2026-10-10 — Windows bootstrap extraction correction

At `077683f`, system PowerShell 5.1 downloaded the pinned Go ZIP in 1,003 ms and
verified it in 112 ms, then hit the existing five-minute fixture deadline during
archive extraction. The bootstrap now calls the built-in .NET ZIP extractor,
removing the archive module's measured per-entry script overhead. Exact-byte
extraction and boundary regressions pass locally; this is not a Windows timing
result. Native PowerShell 5.1 extraction and the complete public migration/setup
lifecycle remain required on the original paths and deadline.


## 2026-10-10 — Windows Neovim Git long paths

At `28c92f5`, both Linux Neovim lifecycles pass. Windows reaches synchronization
but Git rejects the managed Lazy staging path as too long. PortableGit now enables
its documented long-path support in its private system configuration before the
archive fingerprint; the isolated pinned-checkout helper supplies the same setting
explicitly. Recipe revision 1 makes this an ordinary update for completed older
payloads and rejects replaying unfinished older preparation. Local real-Git
configuration, archive lifecycle and long-path Lua regressions cover the change;
actual Windows long-path Git and full Neovim lifecycle proof remain required.


## 2026-10-10 — Windows clangd request diagnosis

At `28c92f5`, the Windows quality fixture starts two distinct initialized clangd
clients with their own project roots, then a document-symbol request times out.
The same fixture passed on Windows at `56a4860`; its test, production LSP config,
minimal init and runner have no intervening changes. The failure cause remains
unknown. Failure-only diagnostics now retain the request error, client/exit state
and at most 8 KiB of those owned servers' stderr. The original deadline and
project-isolation assertions remain required. Local actual-clangd success and a
bounded process-fixture failure validate diagnostics, not Windows remediation.


## 2026-10-10 — macOS published application process inventory

At `28c92f5`, desktop job `114302869025` received a successful NSWorkspace launch
PID but found no owned process through `ps comm`. A real private non-GUI process
regression proves that published directory aliases retain their public path in
that field; it also proves that an alias inside the package can falsely include
an outside executable. macOS fixture inventory now queries the kernel executable
vnode path and compares it with the resolved private package root. The regression
verifies owned cleanup, outside-process preservation, removed-executable handling
and cancellation. Actual window and configured-settings witnesses are unchanged.
This corrects the inventory evidence; it does not establish that VS Code reached
a usable window in the failed run. The complete disposable macOS desktop lifecycle
remains required, without changing HOME, shortening paths or bypassing publication.

The `28c92f5` Windows core job passed the public setup/migrate lifecycle in
71.76 seconds, including cold download (1.352 s), verified extraction (15.858 s)
and build (16.767 s). The ordinary extraction fixture failed earlier because it
omitted mandatory archive verification and treated Utility module first-load
progress as extraction progress. The fixture now reproduces that preceding
Get-FileHash phase; its Stop guard and malformed/escaping/existing-file checks
remain. Actual Windows execution of this fixture correction is pending.

## Current shell and passive-source review corrections

Opus v4 F2 is corrected: the POSIX palette is a shared fragment emitted only for
selected lsd or zsh, before completion. Actual generated-profile Bash tests cover
lsd without zsh, the explicit override and unselected personal preservation. F3
now protects the passive Windows LazyGit link destination with its released
metadata hash and the physical `core.autocrlf=true` checkout test. Final integrated
and native acceptance remain required.

## 2026-10-10 — Linux Code native package correction

Both Linux desktop jobs at `28c92f5` reached Chromium's sandbox startup and
rejected the private archive's helper. The official Code 1.141.0 DEBs are now
routed through the existing APT lifecycle and launch `/usr/share/code/code`,
matching Ubuntu's shipped AppArmor profile. They do not install a setuid helper.
The 29 duplicated Code library edges and 26 exclusive hidden library resources
are removed; Ghostty's three shared requirements remain. Existing native Code
stays unowned, and fixed maintainer-script input preserves personal Microsoft
repository/key/answer state, including deferred pool removal.

Focused regressions pass. Actual worker-backed install/recovery, repair, update
and removal pass in disposable Ubuntu 26.04 ARM (root) and Debian 13 ARM
(nonroot/sudo), with apt-utils preconfiguration and personal repository/key/true
answer sentinels preserved. The first native removal exposed missing dpkg
Essential/Protected observation; the corrected pool retains/discloses those
packages and their dependencies. This does not establish GUI startup. Both Linux architectures still require the
public configured desktop lifecycle on the corrected source. All four fresh
Ubuntu/Debian APT-plus-Neovim jobs passed at `28c92f5`; those results do not cover
this later Code correction.

Windows Git/Make at `28c92f5` reaches the strict PortableGit runtime phase after
extraction and upstream post-install completion, then exits 255 with a .NET-style
path-length error. The exact subcommand was not logged. Fixed runtime labels and
upstream GCM stack tracing in the same disposable private version probe are now
prepared; no runtime check is removed, no payload path is shortened, and no GCM
long-path correction is claimed before the native diagnostic rerun.

At `28c92f5`, fresh Windows Build Tools install/compile/remove and ordinary native
reuse/environment both pass. The five private process-tree boundary scenarios
still fail because their exact child HANDLE is not signaled when inspection
returns. A failure-only five-second HANDLE wait now records exit state and elapsed
time without changing the immediate assertion. Final object rundown versus a
containment defect remains unresolved pending native output.

Temporary diagnostic `48fbcc8` / job `114309772969` passes all four actual
PowerShell 5.1 extraction cases, real private COM shortcut lifecycle and both
progress regressions. All five Build Tools process cases demonstrate exit code 1
before HANDLE signal, followed by signal 2.21–3.45 ms after the query returned.
The provider now captures verified private-job HANDLEs and waits within the same
five-second total bound, refusing changing membership. Native zero-time assertions
and a new late-member refusal case remain required in the next diagnostic run.
PortableGit now stops before LFS/GCM at the adjacent version/private-config
assertions; separate value/exit diagnostics are prepared. Its original .NET-style
failure is still unresolved, not disproved by this earlier failure.


## 2026-10-10 — Active generated-shell coverage (review F1)

The shell/Pester tests now compose current recipes through `configureShellProfiles`
and publish them with the actual profile driver into private test directories.
They no longer use frozen release files as evidence of current shell behavior.
Active invocation guards, real PSReadLine options/keybindings and ordering,
zsh live keymaps, completion/plugin order, locale, colors, PATH and startup are
covered. New active coverage exposed inherited PATH duplicates; the zsh recipe
again retains first-occurrence ordering while removing duplicates. Its regression
fails before the declaration and passes afterward.

The existing Linux/macOS quality jobs generate and execute interactive zsh; the
native archive zsh lifecycle now also runs on Linux. Windows selected-shell native
acceptance includes PowerShell and lsd and inspects real PSReadLine state using
the managed interpreter. Local PowerShell/zsh process proof is distinct from those
pending hosted native lifecycles. Passive release hashes remain unchanged.

### Windows diagnostic follow-up: native HANDLE proof and Git path boundaries

Exact temporary head `9ff8dd0` passes all five immediate child-HANDLE/outside-work
cases plus real late-member refusal, PowerShell 5.1 extraction, desktop query and
COM publication. Git/Make still fails: its version is correct, but `/etc/gitconfig`
misses the private file while an explicit native path reads `core.longpaths=true`.
The independent GCM trace identifies .NET suggestion-directory creation beneath
private TMP, beyond the native 248-character directory boundary. The correction
uses `cygpath` for native Git and a separate fixed GCM phase with extended TMP/TEMP
spelling for the same directory. No settings, executable bytes or physical paths
are changed. Focused checks pass; complete native Git/Make and Neovim acceptance
remain pending the corrected candidate, not inferred from the HANDLE passes.

### Windows Git diagnostic: supported long-file coverage

Temporary head `db366b6`, job `114315143713`, passes both verified Git archive
install/update preparations, their native GCM checks and actual suggestion files
in the original >248-character private temporary directories. Published Git
reads its private system `core.longpaths=true`; ordinary initialization and Unicode
add/commit pass. The job then fails on our newly added roughly 450-character
repository root, a separate unsupported upstream working-directory boundary
([PR 3877](https://github.com/git-for-windows/git/pull/3877)). This assertion is
replaced by >320-character nested directory/file paths inside the existing normal
worktree, with plain add/commit, ordinary-root clone, checkout, diff and exact-byte
verification. Product code and all original managed paths remain unchanged.
Full Git/Make, normal published GCM, LFS/SSH, preservation/removal and Windows
Neovim acceptance remain pending; the failed diagnostic is not a lifecycle pass.

### Windows Git diagnostic: SSH identity-file argument

Temporary head `4eea1ff`, job `114317661301`, passes the corrected 428-character
long-file add/commit/clone/checkout/diff/byte checks, interactive staging, LFS,
normal published GCM and SSH key generation. It then fails because the fixture
passes an unquoted path with spaces through OpenSSH's configuration-syntax `-o`
option. The fixture now uses the documented dedicated `-i` filename argument,
with the identical key path. Real offline OpenSSH inspection reproduces exit 255
before and exit 0 after; Windows compilation/static checks pass. No product code
changes. The complete Windows SSH/Make, final payload/personal-config checks and
owned removal remain pending the next native run; this failed context stays failed.

### Windows Git diagnostic: explicit OpenSSH no-config mode

Temporary head `07e5ed1`, job `114319529231`, confirms the corrected identity
argument, then fails when bundled MSYS OpenSSH tries to open `NUL` as a config
file. The fixture now uses its documented `-F none` mode. The checksum-verified
Git archive records OpenSSH 10.5p1-1, whose exact upstream source bypasses both
user and system config for this value. An offline process regression reproduces
the missing-file failure and succeeds with `none`, preserving the identity path.
No runtime code or later assertion changes. Complete native Git/Make, final
payload/personal-config integrity and owned removal remain pending.


### 2026-10-10 — Review v5 Bash grammar and Git fixture correction

Generated interactive initialization now parses in both Bash and zsh when zsh
is selected. Only the fixed zsh compdump glob condition uses delayed parsing;
Bash never executes it. A real failed-before grammar regression now also asserts
the ready marker, selected lsd alias and empty initialization stderr. The complete
active shell suite and focused race tests pass after the correction.

The initial Lua Git correction used a 257-character staging root and 262-character
`.git` path, retaining nested committed files beyond 320 characters. Later native
proof rejected that root as an earlier pre-configuration canonicalization limit;
the 2026-10-11 correction below reproduces the actual 254-character failure.
The original >320-character repository root also demanded unsupported Windows
current-directory behavior. All 12 pinned-checkout cases pass locally with Neovim 0.12.5;
Windows execution remains required. The default Windows runtime is 78 characters
shorter than the cited hosted fixture for the same generation/PID, so the review's
username-plus-two failure prediction does not follow. No layout or ownership-path
change was made. Exact-head native jobs and the integrated full gate remain pending.


### 2026-10-11 — Ordinary npm builds preserve the managed Node payload

A real offline npm/node-gyp install succeeded but added 18 bytecode files to the
immutable Node package outside Neovim sync. Each Node pin now applies the existing
exact source-replacement mechanism at vendored gyp's import boundary. This scopes
bytecode suppression to that process, leaves personal Python settings intact and
changes desired identity for an ordinary archive update. No wrapper, global
preference, new persistence shape or fingerprint exclusion was added.

The same real offline npm build now leaves the complete Node snapshot unchanged;
archive check and owned removal pass. Old-pin update and exact rendered-source
regressions pass. All four checksum-verified pinned sources pass exact preparation (99.74 seconds).
The existing hosted native consumer/lifecycle gates still provide native execution
proof; foreign source decoding does not replace them.
The Windows ZIP lacks development headers, so its focused consumer fixture runs
the actual gyp import entrypoint directly; full Windows npm integration remains
part of the required Neovim lifecycle. The independent Git early-config-read
failure remains under investigation and is not fixed by this Node correction.


## 2026-10-11 — Linux Code lifecycle review corrections

Opus v5 F2/F4/F5/F6/F7 have focused failed-before regressions. Linux Code now
uses the native package's desktop entry directly; its graph has no private
launcher or shell attachment. Darwin/Windows archive integrations remain.
Pending APT installs from before Protected observation can finish, while fresh
native removal checks still preserve protected packages. Native version changes
no longer discard Code ownership. Old versionful receipts require the exact
completed operation, and update shows current versus pinned version, explicitly
including a downgrade. Exact old pending command arguments remain unchanged.
Completed operations remove their verified large DEB input; changed contents and
unknown worker outcomes retain recovery evidence.

Both Ubuntu diagnostic jobs at `1206f557` reproduced root-owned `/usr/share`
mode 0777, which the reported runner image intentionally provisions. Production
correctly refuses that writable ancestor. The disposable desktop job now records
and establishes a physical root-owned 0755 `/usr/share` before acceptance, without
recursively changing paths or changing package modes. This fixture precondition
is not a production installation repair. Native package and GUI proof remain
separate; final rerun evidence is recorded below when available.

The narrow host-precondition rerun at `b1f4a191` (run 38090322930) passes the
original native Code lifecycle on Ubuntu amd64 (job 114325258197, 51.39 s) and
arm64 (114325258288, 61.28 s), with repository/key/true-answer sentinels preserved
through every phase. Both logs show only `/usr/share` changing from root-owned
0777 to 0755. This establishes the hosted health cause and fixture correction;
it does not cover the later review corrections/downgrade or actual GUI launch.


The corrected provider passes actual disposable Ubuntu 26 ARM/root (104.15 s)
and Debian 13 ARM/nonroot-sudo (97.57 s) lifecycle probes. Each verifies original
worker-reply recovery, repair, a real external dpkg epoch-version upgrade,
reviewed APT downgrade back to the exact pin, package-owned desktop entry
presence/removal, verified DEB-cache cleanup, and unchanged personal repository,
key and prior true debconf answer. The failed-before native downgrade trace is
retained. Both containers were removed after raw evidence capture. Focused
APT/Code/controller/planner regressions, race checks, vet and Linux/Windows
cross-compilation pass. Integrated Ubuntu amd64/arm64 GUI acceptance remains
required; container package evidence does not establish sandbox/window behavior.
Cleanup covers saved transaction inputs and resumable pre-correction operations;
it does not sweep unknown historical/orphan cache directories.


### 2026-10-11 — Native shell fixtures use native provider observations

The v5 Linux core failed two fixtures before shell execution: foreign Windows
composition could not inspect VC++ registration, and Linux zsh had no connected
APT provider. Native shell lifecycles now retain the actual OS identity and wire
the observed provider. Pre-existing healthy zsh/VC++ prerequisites remain reused,
with post-removal preservation checks; fresh-provider lanes retain owned install
coverage. Cross-OS PowerShell composition is covered by the active generated
profile Pester process tests, including real PSReadLine order/keymaps.

A new disposable Ubuntu 26.04 arm64 container passes selected Bash features
(2.68 seconds) and real interactive zsh/plugin/shared-fzf lifecycle (17.75 seconds),
including unchanged APT inventory and personal hook. The active PowerShell profile
suite passes all 24 tests; focused race, vet and Windows compilation pass. Actual
Windows pinned-module/native registration acceptance and final-head hosted gates
remain required. No product code, host-package mutation or workflow contract changes.

## 2026-10-11 — Native clipboard no longer retains removed shell profiles

The `de769314` macOS Neovim job `114321418694` completed application consumption,
update and both reviewed deselection requests, then failed exact restoration of
`state/shells/env.sh`. A ready request can legitimately retain resources: the
catalog incorrectly made reused macOS `pbcopy`/`pbpaste` depend on shell
integration, so retaining those OS commands retained the profile blocks too.

The clipboard shell dependency now applies only to the Windows private archive.
The macOS commands and both Linux APT helpers use the system PATH and remain
protected when reused, changed or explicitly kept. The actual providers,
retained-dependency protection and profile baseline/publication protocol are
unchanged. Linux still installs both X11 and Wayland helpers independently of
the current session.

Failed-before/passing-after fresh-controller tests exercise actual macOS
read-only clipboard observation and profile install, update, shared selection,
selective removal, final removal and a new read-only check. Absent and personal
baselines restore exactly, including generated environment files. Complete
Neovim graph regressions cover native-helper retention on macOS and both Linux
architectures while preserving the Windows archive's shell dependency. The
corrected complete hosted Neovim lifecycle remains required; these focused
checks do not replace it.

Validation: focused profile/Bash-login/shell/clipboard/Neovim-prerequisite tests
pass (47.696s); Go vet, Linux amd64/arm64 and Windows amd64 test compilation,
EditorConfig, JSON parsing and diff checks pass. Catalog counts remain 115
resources and 22 capabilities; no archive pin or package selection changed.

### Desktop native folder and launch-mode boundaries

At `de76931`, macOS and Windows Code launch APIs return without a verified owned
window or settings witness. A read-only Foundation probe proves that fake HOME
and XDG overrides leave the native macOS home/application-support directories
unchanged. macOS acceptance now uses those real folders, as Windows already does,
with the existing reviewed adoption and exact restoration assertions. Controlled
mode variables are actually removed from child environments: empty values are
not equivalent to absence in Electron's Windows entrypoint. Real child-process
regressions and the no-GUI Swift helper check pass; corrected configured GUI
launches remain pending disposable hosted verification. Product launchers and
window/settings/ownership checks are unchanged.


### 2026-10-11 — Consolidated v6 correction status

Exact delivery head `de769314978411d63e509d260ae1e7ce51437f58` completed
[installer run 38089015234](https://github.com/luisgui1757/dotfiles/actions/runs/38089015234)
with nine of twenty jobs passing and
[quality run 38089015235](https://github.com/luisgui1757/dotfiles/actions/runs/38089015235)
with one of three passing: ten of twenty-three overall. Missing-payload Apple
CLT and all fresh APT Neovim lanes passed. The native Windows clangd smoke also
passed; the earlier clangd timeout is not reproduced at this head. Failures are
not cleared by passing substeps or diagnostic branches.

Actual `claude-opus-5-5` v5 review completed with requested xhigh effort; effective
effort was not exposed. It made 111 read-only Read/Glob/Grep calls and all 935
source/evidence files remained unchanged. One Glob permission denial involved
the review parent, not source; no provider/guard error occurred. F1 shared Bash
grammar and F2/F4/F5/F6/F7 native Code findings have focused failed-before and
passing-after corrections. F3's long-file fixture correction needs further
native proof: the first 257-character-root diagnostic fails before config load.
The narrower actual 254-character staging/config-read failure remains open.
Do not describe the successful diagnostic step as a successful Git variant:
all three variants failed init, while the separate full Git/Make lifecycle
passed in 95.35 seconds at `810f50e` (run 38090569304).

The new natural-exit process fixture at that diagnostic head observed lifetime
Total increasing from one to two. The unchanged late-member guard correctly
refused completion; fixture startup is being investigated before changing the
containment contract. Ordinary native PowerShell module discovery and Build
Tools inspection timing still need evidence-based resolution. The same foreign
Windows shell subcase caused macOS core's archive-sweep failure; the native
provider fixture correction covers both POSIX hosts.

The Node consumer regression is now included in the existing private-language
acceptance step on all four native targets, without adding a workflow/context.
Desktop folder/environment corrections run separately at `9ffa038d` in
[run 38091803814](https://github.com/luisgui1757/dotfiles/actions/runs/38091803814).
Its GUI outcomes are pending. Windows/macOS diagnostics do not mutate the
ordinary developer Mac. The merged correction source still requires its full
local gate, actual immutable review and all twenty-three final-head checks.


### 2026-10-11 — Corrected desktop launch proof and Windows Terminal route

Temporary head `9ffa038d4f57f672a2e399fc3e2f1537f05e07b3`,
[run 38091803814](https://github.com/luisgui1757/dotfiles/actions/runs/38091803814),
passes the complete macOS desktop/font job `114329573680`. The actual published
Code singleton and group have owned native windows and consume their managed
settings; Ghostty consumes its configuration; update, check, owned removal and
exact personal-baseline restoration pass (desktop 113.67 seconds). AeroSpace's
published native application launches, with configuration/tiling still explicitly
unverified until owner-granted Accessibility consent. Log SHA256:
`413927f3b2defb8ad2040faf65e9d2ecb2b9604de867b7f9def19e5df2ffc801`.

Windows job `114329573865` passes the corrected Code singleton window/settings
witness in 8.47 seconds and native fonts. The group then fails at public preview:
`tool.windows-terminal has no connected native provider`. Its unpackaged pin
and action existed but its catalog binding did not; isolated Terminal tests
used ArchiveDriver directly and could not catch application routing. The new
all-pin/native-provider regression reproduces this exact missing route across
supported target catalogs. Adding only the existing Windows archive binding
makes it pass; no driver fallback or new provider is added. Every other declared
pin already routes correctly, including FontDriver's special publication path.
Log SHA256: `ddb4dfe768d99148754ef70f14507d167e76102723697af7ed9a13a31804d9fd`.

The group failure remains failed. Full Windows group update/check/removal and
personal restoration require the corrected native rerun. The macOS diagnostic
certifies its exact scoped source, not all twenty-three delivery checks.


### 2026-10-11 — Correct the Git regression to the actual failure boundary

Windows diagnostic head `810f50e`, job `114325970355`, passes its broader Git/Make
lifecycle but all three 257-character Git-root probes fail before config loads.
That root was not a valid supported-boundary assumption. The Lua regression now
keeps the actual production failure's 254-character staging root and 266-character
`.git/config`, plus the same nested committed files beyond 320 characters and all
publication/identity/recovery checks. This is a corrected failed-before fixture,
not a production fix or Windows pass. Its 12 cases pass locally on pinned
Neovim 0.12.5 macOS; native full-consumer 254-character probes remain pending.
Neither fixture home/generation roots nor product ownership paths are shortened.

Windows PS7 → Go → PS5.1 module inheritance: **correction implemented; native
verification pending**. The quality job's Get-FileHash failures are distinct from
the Bash-launched core job, where the same extraction/COM/progress tests passed.
Fixed script children now use their current engine's built-in modules; bootstrap
binds Management/Utility manifests explicitly. The new native regression proves
the original autoload failure and checks real query/job/worker/bootstrap hashes.
The separate Build Tools reuse 30-second timeout has no identified stage yet.


### 2026-10-11 — Windows and macOS complete desktop rerun passes

At temporary head `b86a3316df3061cbdeb228381e466e7de0352064`,
[run 38092427237](https://github.com/luisgui1757/dotfiles/actions/runs/38092427237)
passes both native jobs. Windows `114331421213` completes fonts, published Code
singleton/group and Windows Terminal launch, settings consumption, update,
read-only checks, owned removal and exact personal restoration (desktop
179.19 seconds). macOS `114331421334` repeats the full
Code/Ghostty lifecycle and AeroSpace consent-boundary launch successfully
(102.01 seconds). No assertions, deadlines or product
launchers were weakened. AeroSpace configuration/tiling remains consent-gated,
not certified by a launch.

Raw log SHA256 values: Windows `73d750e055ab291227f5b65ce3017ebd8407cdfaa99f722fa650d1c1d74cede0`;
macOS `3d6b9b06f3599961aef89154fb749295cf3b953193551fe47f062a5744f4bbcf`. This closes the scoped desktop failure and proves
the missing Terminal catalog binding correction natively. Canonical final-head
acceptance still requires the complete twenty-three-job delivery matrix.

### Windows native module correction proof

At temporary head `3cedd3ed81f1ec0cf43e84d3ad9baa76ee479902`, job114332033874
ran the actual PowerShell7 → Go → Windows PowerShell5.1 boundary. The raw
Get-FileHash command reproduced CommandNotFoundException; corrected vendor,
private compiler-job, integration, durable-worker and bootstrap hashes all passed.
Extraction4.41s (including unsafe/existing-file refusal), actual COM publication
and removal3.39s, real progress1.17s and both legacy journal refusals0.14s passed.
Exact product code hashes match the frozen module handoff. The complete job
remains failed for independent process-fixture and timing-instrumentation errors.
No Build Tools deadline conclusion or full native-matrix completion is implied.


### 2026-10-11 — Remaining native evidence and rejected fixes

The module correction has matching native red/green proof, and Windows/macOS
complete desktop lifecycles pass their scoped diagnostic reruns. Linux Code's
new native downgrade lifecycle and fonts pass on both hosted architectures;
its GUI run stalled inside launcher wait and was cancelled as a failed diagnostic.
A private process-boundary reproduction proves that a GUI descendant retaining
exec-managed writer pipes can keep Cmd.Run waiting beyond its context after
launcher exit. The fixture correction uses native file handles and bounded
private diagnostic reads; root failure, timeout, owned-window/settings and final
restoration assertions remain required.

The Windows natural-exit test's single-member premise was false: its ready job
has two live members. The corrected test will prove one held member's natural
exit and capture/await all remaining private members, preserving lifetime-count
and late-member rejection. This does not authorize a production containment
relaxation. The attempted Build Tools timing instrumentation refused an ambiguous
certificate-chain marker before launching the query; it supplied no timeout
cause. Its exact marker is being corrected and the thirty-second product budget
remains unchanged.

All eight Windows Git configuration/prefix variants failed the complete checkout
sequence. At the actual 254 root, system config allowed init but repository
discovery still failed. The full artificial data-root layout also placed final
locked plugin roots at 258/259 characters; changing staging alone cannot fix it.
The canonical known-folder layout keeps those roots at 184 characters or less.
Reject the incomplete system-config fix, metadata aliases and a speculative
cache-layout rewrite. The public Windows lifecycle will observe its actual OS
application folders, while separate tests retain long files beyond 320 characters
and explicit refusal/cleanup at the unsupported 254 root. Full native Neovim
install/use/update/shared removal/final restoration remains the closure gate.

## 2026-10-11 — Desktop launcher inherited-output wait

Temporary source `a5493783d8acfe1b6cb0c17dc7bfd19bb38061e8`, run `38092318318`,
passes the corrected real Code lifecycle (including reviewed downgrade) and
native font rendering/removal on Ubuntu amd64 and arm64. Both full desktop jobs
then remain inside `vscode-singleton` for more than nine minutes, beyond the
fixture's 90-second launch context. ARM records the installed Code process
activating accessibility services. The temporary run was explicitly cancelled
to collect that phase trace; it is not a successful GUI lifecycle.

A bounded private process reproduction proves that an exited launcher can still
leave `Cmd.Run` waiting on writer pipes held by its living child, past context
expiry. Native desktop fixtures now use exclusive private regular-file output,
retaining the same 1 MiB diagnostic read bound, exit/context/error checks,
90-second context and actual executable/window/settings assertions. Owned GUI
cleanup precedes the final log read. The regression releases its exact private
child and joins the original waiter even on the failed-before path. No package,
provider, sandbox or application behavior changes. Full corrected hosted GUI
acceptance remains pending the subsequent checkpoint.

Validation: failed-before process regression reproduces the inherited-pipe wait;
corrected focused cases pass (0.553s), including explicit nonzero exit,
cancellation and output-limit boundaries. Race tests pass (2.610s); Go vet,
Linux amd64/arm64 and Windows amd64 test compilation, EditorConfig and diff
checks pass. No everyday-workstation GUI or native package mutation was used.


## 2026-10-11 — Native Windows Neovim paths and Git's repository-root boundary

At diagnostic head `3cedd3ed81f1ec0cf43e84d3ad9baa76ee479902`, run
`38092637104`, job `114332033874`, all eight controlled Git variants fail the
complete sequence. At a 254-character stage, isolated system config permits init
but both normal and explicit `.git` repository discovery fail on the next command.
The extended prefix fails init. All four 257-character controls fail init.
The encompassing Git/Make lifecycle passes in 133.30 seconds; that is not a
Neovim or variant pass. Raw log SHA-256:
`bebe8e8b22024989136ed7d5e4ea36ebac99dd4c4381cddcff7fcca84e61859d`.

The native fixture's temporary XDG root adds 75 characters over its actual
LocalAppData default. Final locked plugin roots also reach 258/259 characters,
so a compact staging name cannot correct full Lazy and sync verification.
Use real observed Windows known folders for the public lifecycle, preserving its
private HOME, complete operation identities, configuration/profile/editor-data
baselines, notes, and every install/use/update/shared/final-removal assertion.
No Git recipe, staging scheme, metadata alias or product path changes are made.
The Lua suite separates supported-root files beyond 320 characters from the exact
254-root native refusal, checking original-target preservation and stage/lock cleanup.

The real process-boundary regression fails before forwarding the explicit native
XDG setting and passes after. Windows cross-compilation and local pinned Neovim
checks are scoped evidence; actual strict default-layout Git and the complete
Windows public Neovim lifecycle remain required on the temporary hosted branch.
Do not mark the Windows Neovim failure closed before those consumers pass.

Windows query capture correction: **native proof DONE** at 8994dd0, job 114334530446.
All five original process modes, outside-process survival, late-member refusal
and natural 2→0 capture passed. Existing Build Tools reuse/compiler environment
and repeated stable observations passed 20.37s with all five queries under 11s and
the unchanged 30-second limit. The prior isolated timeout's stage remains unproved;
no timeout change was made. Full job remains failed for an independent temporary
Git known-folder probe setup error; full native-matrix completion is not claimed.

## 2026-10-11 — Existing APT alternative consumers during desktop install

The corrected launcher at diagnostic `8a1af68` passed the complete ARM Code/font/
configured desktop lifecycle, including actual Code and Ghostty windows/settings,
update/check and removal. The amd64 Code lifecycle, fonts and Code singleton GUI
passed, then the full-group apply rejected GTK4. Failure-only observation at
`7bf54fe` proved preinstalled Chrome/Edge GTK3-or-GTK4 declarations were hidden
while GTK4 was absent and appeared after the earlier approved libadwaita install.

APT now preserves unresolved declaration names and discloses those consumers for
an absent package. Exact same-plan introduction proof and the executor's strict
consumer equality remain unchanged. Removal retains libraries needed by personal
consumers; it never acquires the browser or removes its existing GTK3 alternative.
Focused controller/parser, architecture/Provides, foreign-change rejection and
legacy pending-intent recovery tests pass. Corrected full hosted amd64 desktop
acceptance remains pending; ARM's earlier pass is not final-head certification.


## 2026-10-11 — Windows native Git/Lua proof and the PowerShell language prerequisite

The strict temporary native proof passes at `43b7e544c54af3c6d1048b73a2ada80604aeb7a1`,
run `38094448973`, job `114337344648`: complete default-layout Git sequences for
lazy.nvim (stage 179/target 168/config 180/nested 514) and the longest locked plugin
(stage 195/target 184/config 196/nested 530), exact 254/config 266 refusal, and all 13
actual pinned-Neovim checkout cases with zero failures/errors. The original
Git/Make install/update/consume/check/remove lifecycle also passes in 125.21s.
Raw log SHA-256: `f7ac5f14be70be964d6fd07c36e245cf6071cce944a93036d6d2924bc0d11f03`.
The earlier quick probe sampled native folders after its own USERPROFILE override;
that temporary diagnostic mistake is corrected by sampling before the override.
No production Git configuration, staging, metadata or path layout changed.

The independent complete Neovim job at `8994dd01688075a21d4bae471a944b608ed0a9f2`
(run 38093492507/job 114334530582) reaches initial public apply/check ready, survives
an installer rebuild, and passes both notes-vault states. Its strict language
smoke has exactly one failure: pwsh is missing. Node's payload remains healthy
and byte-identical. Raw SHA-256:
`ff7a7ef42518172286b827b90d711c00f2cde263d82fb84aa14a8d72a71e41ba`.
Update and shared/final removal were not reached, so the full lifecycle stays open.

Add the existing shared PowerShell archive to nvim.sync's Windows prerequisites;
its existing command-directory assembly then supplies pwsh to real Neovim use.
Closure/order/PATH regressions fail before and pass after, including the absence
of that dependency on both Linux targets and macOS. Correct the smoke's stale
legacy PSES lookup to the existing managed-runtime helper, matching production.
Missing runtime/bundle still fails on Windows. Focused race/vet/compile/syntax
checks are scoped evidence; the corrected complete Windows lifecycle remains
required and runs independently on the same temporary diagnostic branch.

### Native receipt for the absent-consumer correction

The temporary diagnostic head `49def22d24fa9894ef39b1fce0833efec9a231d7`
passed Ubuntu amd64 job `114337270150` in run `38094424487`: Code's real pinned
install/repair/approved epoch downgrade/remove (90.56s), native font rendering and
removal (6.90s), and the complete public configured desktop lifecycle (168.42s).
The latter includes Code singleton and full-group Code/Ghostty native windows,
actual settings witnesses, update/check, final removal and personal-file
preservation. Its unchanged assertions passed after the absent-consumer fix.
ARM passed the complete lifecycle at earlier diagnostic `8a1af68` and again at
`7bf54fe`; both architectures still require the integrated final-head checks.


## 2026-10-11 — CoreText query data/code boundary

CodeQL check 114321518235 on de769314 reported unsafe quoting at font_native.go:109.
Source and read-only native reproduction reject the reported injection: fixed
SFNT names became a complete JSON array expression, with direct process argv and
no enclosing quoted string. Keep the query static and pass the serialized names
as a separate osascript argument instead. No suppression, query policy, native
registration recipe, fingerprint or ownership behavior changes.

The new boundary regression fails before and passes after. Native CoreText output
for all 12 fixed faces is byte-identical before/after, and empty/single/multiple
quote/Unicode inputs pass without code execution. This local proof is read-only;
no font install or registration ran. Focused font tests and foreign compilation
are separate evidence; a new hosted CodeQL result remains required before the
previous failed check can be considered cleared.

## 2026-10-11 — Architecture-qualified absent APT consumers

The controller regression reproduced the remaining absent-consumer boundary on
both supported architectures: a browser's pre-existing GTK3-or-GTK4 declaration
with an explicit native qualifier disappeared before GTK4 installation, then
invalidated the approved plan when the same declaration resolved to its installed
identity. Foreign-qualified and bare alternatives already passed.

APT now uses its existing observed platform architecture when matching absent
selected packages, preserving native-qualified consumers without inventing
foreign consumers. Both architectures pass install/update/removal and later pool
collection, with personal browser/GTK3 preservation. The focused APT suite also
keeps changed/unattributed consumer rejection and legacy recovery green. No
parser, native command, persisted schema or executor guard changed. This is
controller/process-boundary fixture evidence; integrated final-head native
acceptance remains required.

## 2026-10-11 — Code same-pin updates and optional input cleanup

Opus v6 findings N1/N3 are corrected: an owned healthy Code at its recorded
reviewed pin is kept by the existing planner without downloading its DEB again.
A changed version/pin still plans update (including an approved downgrade), and
failed/missing payload still requires repair/install. A legacy receipt without
the existing Desired field performs one reviewed reconciliation, then uses the
same keep behavior. Reused personal Code remains unowned.

Native success and ownership are saved before optional input cleanup. Changed
DEBs, unexpected files, redirects or inspection failures preserve and disclose
their exact directory without blocking the completed package. The disclosure
survives restart, update and removal; unknown native outcomes still cannot claim
completion or discard their inputs. No command, ownership comparator or saved
schema changed. Controller/process-boundary red/green cases and focused APT/Code
checks prove this slice; final-head hosted native acceptance remains required.


## 2026-10-11 — Completed Windows vendor journals remain readable

Opus v6 N2 is fixed in code: adding the child module-path prelude had made exact
completed VC++/Build Tools commands differ from the current encoder, blocking
observation, check and removal. For complete journals only, validation also accepts
the exact previous encoding of the same trusted fixed script. Operation identity,
program, flags, input, environment and recorded successful completion remain
bound; pending legacy commands are refused unchanged and never dispatched.

Both real controller/process-boundary regressions fail before and pass after
through completed legacy observation/check/removal preview and a new exact owned
removal. Completed history remains byte-identical. Corrupted commands and missing
matching native evidence remain errors. This fixture proof performs no native
package mutation; final integration and hosted gates remain separate requirements.


## 2026-10-11 — Windows fixture baseline and Unicode boundary fidelity

Two v6 review risks are closed in test source. The raw upstream PowerShell hash
baseline may legitimately succeed after an upstream fix; that exact success is
now diagnostic alongside the known autoload failure. Wrong/empty output or other
failures remain errors. PowerShell 7 and every corrected native boundary assertion
remain required. The native Git refusal fixture pads and checks its 254/266
boundary in UTF-16 units, preserving its actual refusal and original/cleanup checks.

The baseline policy regression fails before and passes afterward. An actual
checksum-verified pinned Neovim probe of the fixture's source reproduces old
Unicode/astral padding at 247/259 and 252/264 units, then proves 254/266 for both
and the unchanged ASCII sample. Windows compilation and source/syntax checks are
focused evidence; this slice runs no native package mutation and does not replace
the final hosted Windows fixture execution. Product/runtime/layout are unchanged.

## 2026-10-11 — Complete Windows Neovim lifecycle proved

The corrected PowerShell prerequisite and managed-bundle smoke lookup pass the
complete public Neovim lifecycle at temporary head
`70fe3496f8e771ebcd1457a895b414eff32be0e6`, run `38094883281`,
[job 114338646414](https://github.com/luisgui1757/dotfiles/actions/runs/38094883281/job/114338646414).
The unchanged test passes in 1162.47s: initial install/check, check after an
installer-only rebuild, both notes-vault states, strict smoke with 257 checks
including actual PowerShell language-server attachment, update, shared removal
with Pi still runnable, final removal and exact personal/configuration/editor-data
preservation. The six relevant fixture/spec/PSES test/smoke source hashes match
the frozen handoffs; the nvim.sync resource projection matches the approved edge.

Raw log SHA-256:
`0dfaec306f00d2978f99fc66cae750c9f05a4682fb942986e32541809f6373f4`.
The hashed native receipt records source and job metadata alongside the earlier
43b7 Git/13-case success. This closes the Windows Neovim native-proof gap described
above; the historical 8994 missing-pwsh run remains failed. All 23 exact final-head
checks and independent review remain the delivery gate.


## 2026-10-11 — Integrated v6 review closure and final gate

Actual Opus `claude-opus-5-5` completed the immutable v6 review with xhigh
requested (effective effort unexposed), 98 read-only tool calls, no permission
or guard failures, and all 1185 source/evidence hashes unchanged. Execution
summary SHA-256: `a2cb9007dccb7a0cbc298a9ebdaa0bd7df348a83721175889ec742877ec4f33c`.
Review SHA-256: `a9dacc684decc6d238b05524e1ae8095579004f5883713915ec6b96756e80fb3`.
It confirmed all seven v5 findings corrected. N1/N2/N3 are accepted and corrected
in the focused entries above; their frozen code hashes were independently
checked against the writer before and after integration. Two Windows fixture
assumptions are corrected without changing product behavior or mandatory checks.

The first v6 integrated `make -k ci` failed only the PowerShell static scanner's
apostrophe-containing comment; installer tests passed in 501.879s and the
remaining configuration/editor/shell/bootstrap/Renovate checks passed. The
comment is now plain prose without an apostrophe, and the static gate passes.
The failed receipt remains failed; no diagnostic or checker suppression was
added. Final acceptance requires a fresh integrated full gate, all-package
foreign build/test compilation, a focused review of this last correction batch,
and all 23 native/quality checks plus CodeQL on the exact published head.


## 2026-10-11 — Final focused review and unchanged-source local gate pass

Actual Opus `claude-opus-5-5` completed the focused v7 correction review with
xhigh requested (effective effort unexposed), 56 Read/Glob/Grep calls,
no permission/API/guard failures, and all 811 immutable source/evidence
hashes unchanged. It found **no material defects** in this final batch, confirmed
N1/N2/N3 fixed, and accepted the small post-v6 changes. Execution summary SHA-256:
`51d0605e4152f70d2f81d3b22ead164b280e9a3bd0fb979028038967a29808dd`. Review SHA-256:
`59f013c0dc8434da3d4711b203adc2f121901af71ea389f16cdbfe6f513c4296`.

The historical N2 provenance gap is independently closed against actual
`de769314978411d63e509d260ae1e7ce51437f58` Git objects: every vendor script/prelude
constant remains byte-identical; only the new command encoder/module binding
changes. Build Tools process source itself is byte-identical. Constant-tail
SHA-256 values are `8184e8a8caa0e1bc25827ddf475b738994fad500e40eb9a7b85f3e8018d80351`
and `294d1297182fa141686f09cae4113c0964c1bf3cc5955eefa772d1432f278ab3`.
Future recipe edits require a completed-history audit; fixtures regenerated
from a new script cannot substitute for historical evidence.

`make -k ci` passes with all 747 source hashes unchanged: installer tests 494.376s,
terminal tests 14.000s, configuration/editor/shell/bootstrap/Renovate gates green.
All-package builds and test compilation also pass for Windows amd64 and Linux
amd64/arm64 on the same unchanged source. This final append changes only Markdown;
a focused static gate verifies the finished documentation before publication.
The earlier v6 static-comment failure remains a failure in its archived receipt.

Nonblocking review risks are recorded without speculative runtime changes. Durable
retained-input disclosures match other drivers and prevent renewed deletion
authority; changed inputs still block later repair before mutation and disclose
the retained path. Architecture:all roots are absent from this catalog; such a
future addition needs consumer-matching proof. An old policy snapshot may fail
readback if GitHub adds a returned restriction: this is prior, fail-closed behavior
and cannot justify dropping the flag. Native hosted checks and separately
authorized policy write/readback remain real acceptance boundaries. The reviewer
also confirms the Windows Neovim raw log's actual PSES attachment, 257 smoke checks
and full 1162.47s lifecycle pass. All 23 canonical native/quality jobs and CodeQL
must pass at the exact published head; earlier/scoped passes do not replace them.


### 2026-10-11 — Ubuntu completion test-host correction

Final-head quality job 114349412458 correctly failed Starship/zsh startup on
insecure hosted completion directories. Diagnostic run 38098923912 reproduced
both unchanged assertions and identified root-owned mode 0777
`/usr/share/zsh/vendor-completions` and `/usr/share/zsh`; their `/usr/share`
ancestor was also writable. Run 38099014868 proved that physical/root-owned
validation followed by nonrecursive 0755 on exactly those three directories
restores strict compaudit and both tests.

The shared disposable-host helper now covers Ubuntu quality and core interactive
zsh, and replaces Desktop's existing single-parent preparation without expanding
that recipe. Production completion and all startup/GUI assertions remain intact.
Unsafe ownership, redirected directories/ancestors and unsupported host/recipe
invocations have process/filesystem refusal coverage in a private mount namespace.
The full final-head quality/native matrix still requires a new published-head run.

The exact factored helper and refusal fixture passed on disposable Ubuntu in
run 38099271054/job 114351621069 at
`b73c38d5c35c8fd55ddc7f344e4f563e3d5d32f8`, followed by both unchanged startup
tests passing. Raw log SHA-256:
`37ccfaebfb15a19c8b05d294157c8f41ebc0ff471f62b49d3275f09ca2ce3e9c`.
No permissions or packages were changed on the developer workstation.


### 2026-10-11 — Frozen completion helper verified on Ubuntu ARM

Review v8's architecture evidence gap is closed for the actual Ubuntu 26 ARM
image `20260927.135.1`: run 38100313604/job 114354731869 at
`3322d7929c941e9dbb2192d77919b66ac01582ee` passed the exact frozen helper
and refusal fixture, strict compaudit, and unchanged full `make test-shell`
(including 25 bootstrap tests and ShellCheck). The job verified source SHA-256
values before execution. All three reviewed directories, including
`/usr/share/zsh/vendor-completions`, exist as physical root:root mode 0777
directories and become 0755 through the unchanged helper. No missing-directory
accommodation or production change is justified by these observations.

The raw passing log SHA-256 is
`fe21a763e728c6ff715d785eb55e2ed91164c6ccf5d0e902667824e7fcca9f89`.
The earlier ARM diagnostic 38100200478/114354384785 passed helper/zsh checks
but its full suite failed because the temporary shallow checkout lacked the
released `v0.1.0` tag. That original failure remains preserved; matching the
canonical quality workflow's `fetch-depth: 0` fixed only the diagnostic setup.
Final amended-head required checks remain a separate delivery requirement.


### 2026-10-11 — Windows bootstrap helper naming correction

Final-head Windows quality retry 114351013020 passed dependency installation,
installer race tests (641.935s), all 48 bootstrap/migration Python tests, all 49
Pester tests and the Lua suites. It failed on the single PSScriptAnalyzer 1.25.0
`PSUseSingularNouns` finding at the bootstrap module-binding helper. Renaming
that helper and both AST lookup references changes no binding, extraction,
cache or invocation behavior and adds no suppression.

Focused local extraction/parser and baseline-policy race checks pass (2.694s),
as does Windows amd64 test compilation. Native run 38100201494/job 114354387651 at
`6ebcdd0ceeb203f2ca633d6839a4edb003749917` also passes: actual PSScriptAnalyzer
1.25.0 reproduces the original one finding and reports zero after the rename;
all four PS5.1 extraction cases pass (2.21s), followed by the real PS7 → Go →
PS5.1 inherited-module regression (2.53s). The original quality failure remains
failed. Native raw log SHA-256:
`742b19bc24f1e6b81c569d7a625323f4f9253a8d5dcd1e55f8ab286c5443b086`.
The separate Gallery provisioning failure was not reproduced on the same image:
the unchanged install succeeded with PSGallery already registered, so no
repository/provider change was made.


### 2026-10-11 — Windows selected-shell fixture honors shared removal

Final-head core job 114349412548 failed only the selected-shell archive fixture:
it deselected its shared PowerShell without explicit removal authorization,
then incorrectly expected the retained runtime's shell integration to disappear.
The whole owned block and exact personal suffix remained; no personal edit was
lost. All independent language, Git/Make and public migration steps passed.

The fixture now first proves default retention of its owned PowerShell archive
and exact shell/personal bytes, then authorizes only `tool.powershell` removal.
Existing archive absence, exact personal/BOM restoration, other profile cleanup
and pre-existing VC++ runtime checks stay intact. Planner/runtime behavior and
POSIX flow are unchanged. Focused shared-retention/profile race tests pass
(2.226s), as does Windows test compilation. The complete native Windows shell
lifecycle passes in 52.26s at `b974f8261a32554e7684bb3253bfce1c474807a0`,
run 38100700339/job 114355857812. Both default retention and explicit removal
are observed before the unchanged final preservation assertions pass. Raw log
SHA-256: `2ccef4a8982dbd54c3d35a8e1494ff0f4cc2dd02d6166fb797d5cfc4438ef154`.
The original final-head core failure remains recorded; this scoped pass does
not replace the next full published-head matrix.


### 2026-10-11 — Opus v8 clears the disposable Ubuntu host correction

The actual `claude-opus-5-5` read-only review reports no material safety or
correctness defects and calls the seven-file runner correction the smallest
complete change. The client recorded 43 Read/Glob/Grep calls, no API, guard or
permission errors, and 769 unchanged supplied files. xhigh was requested;
effective effort is not exposed. Review SHA-256:
`3c29b887a7f70520345fb914e5765b18c0b27229dd253eecff35487965fa8add`.
The architecture assumption about the ARM vendor-completion directory is closed
by the exact frozen-source native ARM receipt above. The dedicated disposable
VM has no root/sudo privilege boundary for the noted check/chmod interval; full
prevalidation and postconditions remain. No product security bypass, additional
provider or absence accommodation is justified. The original four failed
canonical contexts on `044f978` remain failed (19/23 passed); scoped corrective
proofs do not replace the next amended-head matrix. CodeQL's three analyses and
aggregate passed on that checkpoint.


### 2026-10-11 — Windows compiler-query pointer boundary corrected

Windows Neovim job 114349412580 at `044f978` passed strict startup, update and
selective removal, then refused final removal at the compiler-query membership
guard. Diagnostic job 114354654925 reproduced a zero baseline followed by
Total45/Active0 despite completed exact HANDLE waits. The problem was a lost
query output buffer: the pinned x/sys plain-uintptr wrapper can outlive the
caller's movable Go stack address. The typed boundary now preserves each buffer
until the annotated native call, including the corresponding fixed job-limit
input. Membership, Active-zero, exact HANDLE, atomic assignment, output and time
bounds remain unchanged. Failure-only counts and bounded private-job IDs retain
useful evidence without paths, environment or a machine-wide process inventory.

At exact temporary head `97d5596ef74cfd04d4360af37295157997bd944d`, run
38100961481/job 114356621691 reproduced the old Query defect against an actual
empty Windows job: the successful call left all-ones accounting sentinels intact.
The corrected accounting/member/limit cases pass at 128 varied stack depths,
alongside every existing success/error/cancel/output-limit, outside-survival,
natural-exit and deliberate late-member refusal case. Six real compiler
reuse/environment lifecycles pass (34.93, 23.70, 23.95, 23.72, 23.65 and 23.78s).
Local Windows compilation/vet and focused provider race checks also pass.

That diagnostic job remains **failed**: its separate expected-red Set test did
not reproduce lost input and deliberately threw; the corrected test and reuse
steps both concluded success. Old PID-buffer failure was likewise not reproduced.
The Set/PID corrections follow the same documented unsafe-pointer contract, with
passing real limit readback and member-query coverage, not a claimed native red
for each call. No repeated series was used to erase this limitation. Raw log
SHA-256: `9eb5ac8fb77974830205023f4ec0756260e2599ecda46ee08fb369e06b11fe55`.
The [review ledger](reviews/2026-10-08-interactive-installer-overhaul.md#2026-10-11--accepted-windows-job-buffer-lifetime-defect)
records source and compiler evidence. The original failed full lifecycle stays
failed; all final amended-head native checks, including complete Windows Neovim
removal, remain required before delivery readiness.


### 2026-10-11 — Consolidated final correction gate and Opus review pass

The complete `make -k ci` gate passes with all 749 supplied source hashes
unchanged. Installer tests pass in 512.970s; unchanged terminal tests validly
reuse their passing Go cache. Configuration, editor, shell, bootstrap/migration,
policy and official Renovate checks also pass. All-package Windows amd64 and
Linux amd64/arm64 builds and test compilation pass against the identical source.
These local/cross-compile proofs do not substitute for native lifecycle checks.

Actual `claude-opus-5-5` final focused review finds no material defects in the
Windows API buffer boundary, bootstrap helper rename or shared-shell fixture.
It confirms minimal completeness and unchanged completion/ownership safeguards.
The client records 46 read-only calls, no API/guard/permission errors and all
796 snapshot/evidence hashes unchanged. xhigh was requested; effective effort
is not exposed. Review SHA-256:
`5ea58e925d64dbe1710f3915f9c3b8e4fc4ba7b928044e3fad6387d1a42bb17d`.
No runtime or test change follows that review.

The cold-call sentinel reproduces the original Query defect in an isolated native
invocation; warmed full-suite calls need not reproduce it. The same comment
promises fresh goroutines and varied stack depths, not fresh native procedure
lookup for every subcase. Keep this proof limit explicit alongside old Set/PID
non-reproduction. The original failing complete Windows Neovim lifecycle is the
mandatory highest-fidelity regression at the amended head. The reviewer recommends
no change to the existing one-member fixture's small accounting window. These
nonblocking observations do not justify new process machinery or weaker guards.

The four earlier canonical failures remain failed. This source is ready for the
one-commit amendment in PR #87; all 23 canonical native/quality contexts and
CodeQL must pass on that exact published head before the PR is ready. Required
context cutover remains a separate explicitly authorized owner action before
merge. No live policy, daily-Mac native packages, fonts or GUI were changed.


### 2026-10-11 — Windows desktop cleanup and fresh launch evidence

At final head `127e35d676c69a65068b55e15c0c3508c6a7f3dc`, Windows Desktop
job 114360377438 passed fonts, Terminal settings and the VS Code singleton,
then failed the second Code invocation in deferred cleanup with
`TerminateProcess: Access is denied.` Raw log SHA-256:
`19ff2311be0276b5a10586f87013d88e7e13ad6de1e446c0a76e95ef80841c41`.
Windows documents that an already terminated process produces this error;
the old fixture distinguished only os.ErrProcessDone. The fixture now validates
the acquired process HANDLE's image under its private root before signaling it,
and accepts a termination denial only if that exact object is already signaled.
Live denial remains an error. The existing cleanup deadline and outer inventory
are retained, and POSIX signal behavior is unchanged.

The two Code invocations also reused one settings witness in their shared home.
Each invocation now has a unique absent witness, preserving the preceding file.
The local filesystem regression fails before this change and passes afterward;
the native lifecycle must obtain fresh extension-host evidence on both launches.
Windows compilation/vet and the focused local race test pass. Actual Windows
kernel and full public Desktop lifecycle proof remain pending in the separate
scoped diagnostic; the original final-head job remains failed.


#### Native Desktop teardown refinement within the same deadline

Temporary head `c68557f42c1722a35bcaafae4c00c393198d1d2f`, run
38103229644/job 114363304745, passes the real old-Kill denial control and all
five guarded kernel cases, plus the fresh-witness regression. The full GUI
then publishes a fresh singleton settings witness but fails cleanup: PID4560
reports ACCESS_DENIED with an immediate WAIT_TIMEOUT258; two other captured
processes report image-query ACCESS_DENIED during the same rundown. Raw SHA-256:
`92a5d7b751d796bdeb514830e7a48f56c5445191e6dac4dde4ae49c7444a7ad9`.
This rejects the narrower assumption that completed termination must already
signal at a zero-time wait. It does not justify ignoring any access error.

Wait passively on the exact acquired object within the existing shared 15-second
cleanup deadline after termination denial. If image inspection is denied, only
wait: never signal an unvalidated executable. Success still requires
WAIT_OBJECT_0; an object that stays live retains the original access error and
unproved-completion result. Kernel coverage adds asynchronous termination and a
live handle lacking image-query rights, retaining outside survival. No new
retry, timeout or authority was added. The original failed diagnostic remains
preserved, and complete native GUI proof is still required.

Independent review also identified the POSIX adapter's os.Process ownership:
release the acquired Process after Kill, join release errors and preserve
os.ErrProcessDone normalization. This closes its Linux pidfd lifetime without
changing signal policy. Real macOS private-process/alias/outside-survival and
fresh-witness race checks pass in 2.524s; Windows source is unaffected by that
platform-specific refinement.


#### Completed native Desktop corrective proof

Exact head `c87a2b5c540a48499632dd448779a72cc7a788aa`, run 38103641444/job
114364496569, passes all seven Windows kernel cases (10.30s). The old Kill
boundary reproduces ACCESS_DENIED; asynchronous termination observes an initial
WAIT_TIMEOUT258 and then proves exact completion. Live termination-rights and
image-query-rights denial remain errors after their bounded 5-second waits,
and outside/canceled processes survive. The fresh-witness filesystem case passes.

The same job passes the entire public Desktop lifecycle in 181.57s: Code
singleton (8.65s), second Code (5.78s), Windows Terminal launch (2.78s), update,
check, explicit removal and every original personal-data/restoration assertion.
Each Code invocation logs its own freshly published extension-host settings
witness. Raw log SHA-256:
`d29e03a359748481ed3edd3c8acc439419cccf3293830c05873aa74dc9754522`.
The original `127e35d` and intermediate `c68557f` failures remain preserved.
This closes the two demonstrated Desktop fixture defects without changing
product runtime or native deadlines. Full amended-head required checks remain
a separate delivery requirement. The temporary diagnostic workflow is excluded
from the delivery patch.

### 2026-10-11 — Windows Git inspection metadata; original cause unproved

Final127 Windows Neovim failed final `tool.git` inspection before its separate
smoke, with no entry/predicate detail. The delivery change adds only bounded
relative-entry metadata to that refusal, preserving every existing identity,
mode, size, mtime, fingerprint, approval and retry rule. No later native identity
query or Git provenance helper is added to delivery.

The original complete Windows Neovim lifecycle passed1256.98s on isolated
diagnostic head182c04b: initial apply, installer-only rebuild check, all257 smoke
checks, update, shared Pi survival, final removal and exact personal
configuration/notes/editor-data restoration. No failure-only diagnostic fired;
the original127 failure remains failed and its cause remains unproved. This is
non-reproduction with unchanged guards, not a runtime fix or final-head
acceptance. Temporary diagnostic workflow/probes remain outside delivery.

### 2026-10-11 — Private local test tools restored; failed gate retained

The first integrated Desktop gate failed four public-launcher subcases and six
ordinary Go tests because the private macOS PowerShell tool's Management and
Utility module directories were empty. Errors name missing Join-Path,
Write-Progress, Test-Path, ConvertFrom-Json and module manifests. The gate's
source hashes remain unchanged; this is a failed environment gate, not product
regression proof. The cause of those missing files is unproved.

A separate task-owned persistent directory now holds PowerShell 7.6.3 verified
against the official asset digest and official checksum file, with every regular
file and mode checked against the archive. Its four unchanged launcher subcases
pass. The local Go 1.27.1 SDK is likewise verified against all 15,639 archive
files and modes. No system tool, personal profile or module setting changed.
The complete consolidated gate will run against these verified private tools;
the failed initial gate is retained rather than overwritten.

### 2026-10-11 — Consolidated Desktop/inspection gate and independent review

The complete `make -k ci` passes with the verified private tools: installer race
tests 503.702s, terminal race tests 14.010s, all 48 bootstrap tests and the
configuration/editor/shell/static/policy/official Renovate checks. All six
foreign all-package builds/test compilations pass for Windows amd64 and Linux
amd64/arm64. Every receipt binds the same 753 unchanged source hashes.

Actual `claude-opus-5-5` review, xhigh requested/effective effort unexposed,
finds no material defects after 62 read-only calls. All 803 supplied source and
evidence hashes remain unchanged, with no API, guard or permission failures.
Review SHA-256:
`47ea14ba6fcba69dca30f95eb0ee3cf900c82b963e2e10ac20bd89fa8b4e01c1`.
The Desktop changes are minimal and fail safe; inspection policy/error chains
are unchanged. The historical unexplained Git refusal does not independently
block delivery, but its exact final-head Windows Neovim check remains mandatory.

The review saw the local gate while running; completed receipts now close that
gap without changing runtime/test bytes. Root independently verifies the omitted
original failed logs and corrects one explanatory claim: ArchiveDriver converts
inspection errors to Unknown/Pending, whose text enters blocked plan identity.
Already-observed metadata introduces no read-time clock; unknown artifacts
still cannot authorize mutation. Native reduced-formatter/Windows race-kernel
and all configured Desktop/Neovim acceptance remain final-head requirements.
Only proof Markdown follows this review/gate, with focused static verification
before amending the existing single commit. Live policy, merge and release are
unchanged; every 23 canonical check plus CodeQL must pass on the published head.

### 2026-10-11 — Linux X window inspection boundary

Final `406218a` ARM Desktop job `114371815911` passed Code's native package
lifecycle, fonts and the singleton's fresh settings witness, then failed on
`xwininfo` BadDrawable/GetGeometry and BadWindow/QueryTree while an enumerated
child disappeared. AMD64 Desktop passed. The fixture treated this first failed
query as fatal after 0.47 seconds; this is not evidence of Code startup failure.

The correction discards failed tree output and permits only that exact typed
non-root disappearance, with fresh root and Window-specific absent-child proof,
to return absence to the unchanged 45-second poll. Other `xwininfo`/`xprop`
errors remain fatal. Owned PID matching additionally requires `IsViewable`.
Process-boundary regressions and a real disposable Ubuntu 26 ARM Xvfb fixture
cover disappearance, outside/unmapped windows and unavailable-display failure.
A real concurrent Xvfb tree deletion reproduced BadWindow/QueryTree; fresh native
root/GetProperty checks accepted only its now-absent child. Both architecture
full public desktop lifecycle proofs passed at diagnostic source
`5788d8b4b062fc7941541f23143a812ee3e50661`, run `38107178096`: ARM job
`114375040519` (167.76s) and AMD64 job `114375040614` (127.45s). Each
passed the real X boundary, native Code ownership/repair/reviewed downgrade/removal,
fonts, two fresh Code settings witnesses, Ghostty use, update and final personal
baseline restoration. The final delivery patch has identical fixture/workflow
bytes, with only this proof receipt and precise evidence wording updated. The
original final-head failure remains recorded; the next consolidated final head
still requires its own complete acceptance matrix.

### 2026-10-11 — Native X fixture indentation gate

The first consolidated Linux X correction gate failed EditorConfig on six
space-indented lines inside the Go test's embedded Python client. The remaining
gate work was stopped after that known failure; its receipt is failed, with
source unchanged. Valid Python tab indentation replaces only those six leading
spaces. Parsed Python AST and all Go bytes outside that literal are unchanged;
no checker suppression or semantic change is introduced. A clean combined
gate and final-head native X verification remain mandatory.

### 2026-10-11 — Consolidated Linux correction verification and review

Published `406218a` completed all canonical jobs: **22/23 passed**, only ARM
Desktop failed; CodeQL analyses and aggregate passed. All six macOS and all six
Windows native/quality jobs passed. Original failed logs remain failed evidence.
The isolated corrected full Linux GUI lifecycles pass on amd64 and arm64;
the supplementary exact tab-indented Python fixture passes on disposable ARM
Xvfb (native window boundary 0.02s; cancellation 0.00s).

The clean complete `make -k ci` gate passes: installer race suite 531.378s,
terminal race suite reused its valid Go cache, all 48 bootstrap tests, editor,
shell, static/policy checks and official Renovate validation/extraction. All six
Windows/Linux all-package build and test-compilation checks pass. All seven
receipts bind the same 756 source hashes before/after. The earlier gate with
EditorConfig failure was stopped and is retained as failed; it is not a pass.

Actual `claude-opus-5-5` focused review used 67 read-only calls and 792 immutable
supplied files, with no API/guard/permission failures; xhigh requested, effective
effort unexposed. Review SHA-256:
`fcc26c3ae879c8e00f5332de2ed9c6e17c77b17f3ca95fcdcec136e5ae6dac84`.
It found no material X inspection logic defects. Its sole material indentation
finding is fixed by the six-indent-only Python AST equivalence and exact native
proof above. No suppression or broader post-review code change is allowed.

Non-blocking limits remain explicit: there are no dedicated stats-
disappearance/fault table cases; the shared strict failure logic was reviewed
as correct. Other possible coordinate-race shapes remain conservative errors,
not accepted absence. No unproved error shape is added to acceptance. Existing
small fresh root/child checks remain; CLAUDE's description now names every
accepted query kind. The supplied native tree-capture proof covers 3/15, while
the original paired 9/14+3/15 form is process-boundary coverage. Neither proves
which original process owned that transient child. Original Git refusal cause,
recurrence recovery and AeroSpace effects without Accessibility consent remain
unproved; final-head Windows Neovim and canonical GUI gates remain mandatory.

Only proof Markdown follows these gates and review, with focused static
verification. Delivery remains one PR87 and one amended commit; all 23 canonical
jobs plus CodeQL must pass on its published head before ready status. Live
policy, merge and release remain unchanged/separate owner actions.


Windows bootstrap fixture correction: the archived `4380ae9` quality run failed
three new fixture boundaries (default text decoding, dynamic compiler-context
variables without observed assertions, and a single assumed native-error mode).
The isolated correction changes tests only: explicit UTF-8, observed compiler
context, and both native-error preference modes with exact exit-code/error checks.
Production isolation and error propagation remain unchanged. Native Windows
proof passed for the exact matching test bytes: PSScriptAnalyzer 1.25.0 in
diagnostic `1a199c5`, then all 50 Python and 53 Pester cases in `761ca1b`. The
diagnostic jobs still failed overall for separate temporary-workflow history and
Docker-observation reasons; final canonical quality remains required.


## 2026-10-11 — Desktop smoke-test corrections

The owner authorized a second commit on PR #87 for fixes found in a disposable
Ubuntu 26.04.1 amd64 GNOME/Wayland VM. The original implementation commit
`003ef4ae1af1115e0ea93a6f7691138535a979e3` remains intact. This is a high-level
interactive smoke round; full migration and destructive-case acceptance were
removed from its scope by the owner.

Direct Neovim file startup failed to attach Python's server because LSP enablement
preceded Mason's PATH setup. Opening the file after an empty startup did attach.
The LSP specification now depends on Mason. Native CI starts a fresh editor process
with a file on argv for each configured LSP, using the existing language matrix;
the older language smoke no longer loads Mason on behalf of production startup.
The new regression failed before the fix and all eight Linux servers initialized
after it. Windows PowerShell attachment remains part of the native Windows lane.

Pi's documented Moon/Dawn choices caused adoption prompts and needs-action/recovery
on a subsequent operation. Their exact-field fingerprints now share the existing
main-theme identity while rendering preserves the actual choice. Baselines and
journals retain their raw bytes; legacy exact-field baselines/restoration remain
validated. Interactive maintenance/removal regressions failed for Moon/Dawn before
the fix and pass for all three variants afterward. Unrelated themes still require
adoption and remain protected on removal.

LuaLS also wrote logs/state into its managed package, causing retired generations
to retain those files after normal use. Check remained ready; this was a state
placement/cleanup issue, not a health-check failure. Its supported log/meta options
now use Neovim's user state directory. Native CI compares the entire published
payload before/after the language applications, then requires Check to stay ready.

The Ubuntu VM installed every Linux-selectable tool through the actual terminal
menu. The smoke round exercised shell navigation/search, fuzzy history, a LazyGit
stage/commit, tmux split/tree, Herdr's configured navigator, desktop Ghostty launch,
and editing/saving in desktop VS Code. A direct Python startup in Ghostty now
supports the configured rename binding, save, clipboard and execution. Pi's Moon
choice survives the actual update UI without adoption or recovery. Authentication-
dependent services remain unverified; no host credentials were supplied.

The full local `make ci` gate passed in a fresh test XDG context inside the guest,
with Go/Ruby/clang contributor prerequisites recorded separately. Final format/vet/
focused race regressions and build/test-compilation for all four supported targets
also passed. The strict installed language matrix passed 257 checks; all eight Linux
LSPs initialized in fresh processes and the complete published runtime remained
byte-identical afterward. PowerShell's LSP remains enforced by the Windows lane.
The final published head still requires its own hosted native/quality/CodeQL checks;
previous green checks on the original commit do not certify the added fixes.


## 2026-10-12 — Replace Pi with Claude Code and Codex CLI

The owner explicitly retired Pi entirely and requested independently selectable
Claude Code and Codex CLI on the existing supported targets. The catalog, archive
pins, active and copied legacy Pi configuration, Pi theme normalization, package/
runtime/PTY tests and Sentinel Pi target are removed. Exact release inventories
and prior ledgers remain immutable historical evidence. Generic scoped-settings
publication and preservation/recovery tests remain for supported consumers.

Both CLIs reuse the private archive lifecycle with automatic Git and shell
integration, without an npm engine or Node prerequisite. The verified native
platform archives are published by the vendors' npm registry. Each compressed
package's SHA-512 SRI is verified before recording its SHA-256 pin; all four Claude
binaries also match Anthropic's signed release manifest. Codex's fixed vendor/
target prefix is stripped while keeping its complete native package, helpers and
resources. Claude's managed shell sets DISABLE_UPDATES=1 only when selected.
Personal credentials, model/permission settings and sessions stay unowned.

Sentinel honors CLAUDE_CONFIG_DIR and CODEX_HOME at the process boundary and no
longer writes Pi policy. Its former four-destination crash-test index is now
bound to the actual destination count, preserving first-publication/last-pending
proof. The legacy retention contract requires the specifically retired Pi files
to be absent and continues requiring every other released live-link source.
Those stale test contracts failed during integration and their corrected focused
checks pass; the original failures are preserved in exported VM evidence.

The baseline full gate passed before changes. Independent-choice/retirement
regressions failed on the prior catalog, then passed for all four target
definitions. Build and test compilation passed on all four supported targets.
The complete local gate passed after the behavior and test integration, including
51 bootstrap tests and Go formatting/vet/race. The final compressed-pin data and
actual CLI desktop/native runtime proof are still in progress; these statements
do not certify the previous head's CI for the changed product.

Two raw-binary native attempts exhausted the existing 15-minute request deadline
before runtime validation on this VM's slow download route. This is recorded as
provisioning evidence. The smaller, vendor-published compressed native packages
contain the same verified binaries; no timeout or ownership check was relaxed.
The CLI fixtures install choices incrementally under their existing deadlines.


### 2026-10-12 — CLI replacement verification

Pi's active implementation, configuration and test routes are retired. Claude
Code 2.1.287 and Codex CLI 0.162.1 are independently selectable on all four
supported target definitions, with only Git and shell discovery prerequisites.
Every native platform package passed its published SRI check; each Claude binary
also matched the signed vendor manifest. Both are direct native archives without
npm/Node execution.

The full local gate and all four build/test-compilation checks pass. The new Linux
native CLI lifecycle passes installation, offline consumption, Check, unchanged
Update, repair, selective/final removal, Git protection and personal data witnesses.
Actual Ubuntu 26.04.1 amd64 desktop launches reach both sign-in screens without
credentials; Check is byte-preserving and the Update preview keeps all four
resources. UI repair also passes. The first Codex archive layout discarded metadata
and was rejected by the safe extractor; its failing snapshot is retained, the
layout is corrected, and failed-before/passed-after coverage checks all four targets.
Fixture/provisioning failures are separately recorded in the review ledger.

Authenticated model use, genuine CLI version transitions, and native execution on
macOS/Windows/Linux arm64 are not certified by this Linux smoke. The changed
published head still requires its own complete hosted checks and CodeQL.


Desktop removal verification is complete: Codex still runs after removing Claude;
removing Codex then removes the shared shell integration and retains pre-existing
Git. Both agent settings/history witnesses are byte-identical to their personal
post-onboarding state. The original Bash profile is restored byte-for-byte with
its exact later personal edit appended. Node remains absent. The original failed
archive preparation is retained and explicitly disclosed for inspection.

### 2026-10-18 — Additional VM acceptance in progress

Ubuntu 26.04.1 amd64 completed all 25 choices through the public installer:
installation, read-only Check, unchanged-pin Update, missing-launcher Repair,
full removal and reinstall. Removal preserved 36 tracked personal files;
reinstall retained 28 app-data hashes, personal shell edits and Git identity.
Obsidian reopened the edited vault and Zen reopened its retained browser tab.
Configured Neovim passed 257 language checks without changing its runtime.
Bash/zsh, actual Git rebase pulls, tmux panes/clipboard, Herdr, VS Code and stable
T3 browser/terminal use were exercised. OpenWhispr opened but microphone input
and Wayland auto-paste remain unverified; the VM lacks those prerequisites.
Seven old baseline kernel packages disappeared through Ubuntu unattended-upgrade
before dotfiles removal, proved by APT history. A pre-existing Ptyxis terminal
crashed once; its cause is unverified and it is not classified as a dotfiles fix.

A separate Ubuntu VM also passed the real released-installation migration test
against immutable v0.4.4: the old setup installed Nix, Home Manager, Linuxbrew,
chezmoi and Pi before the public migration replaced and retired them. The test
passed in 563.58 seconds, including interruption/recovery, outside-account
protection and personal-data preservation. The final check was ready, `/nix`
and active chezmoi configuration were absent, and the login shell was `/bin/bash`.
The temporary test-only sudo rule was removed afterward.

Windows 11 default settings exposed a Python preparation path-limit bug. The
focused correction now passes native base-Python and private LaTeX/yamllint
lifecycles without changing the registry. Full Windows acceptance and genuine
released-installation migration are still running. Mac acceptance is deferred
until this VM round is complete; prior hosted checks do not certify this changed
candidate.

The Windows all-tools run also reproduced Mason's ensurepip path failure for
Gersemi and clang-format. Their standalone upstream binaries at the same versions
passed actual CMake/C formatting and archive install, immutable-payload Check,
unchanged Update, pointer Repair and exact removal with LongPathsEnabled=0.
The corresponding Windows-only prerequisites are now connected; the complete
configured Neovim rerun remains pending. Abandonment also exposed a Neovim-specific
recovery trap, reproduced for both first installation and update in engine tests.

The separate Windows migration VM applied the genuine v0.4.4 configuration and
preserved all four legacy PowerShell profiles through public migration. The old
dependency installer had reported LaTeX and Build Tools failures and skipped
authenticated gh-dash setup. Defender recorded behavioral process blocks during
that legacy run and again during the new VC++ runtime installation. Replacement
setup correctly failed its native postcondition; its interrupted transaction is
preserved. No protection was disabled and retirement has not run on this VM.

A fresh approved retry from the README's ordinary PowerShell workflow installed
VC++ successfully with Defender enabled and no new detection at that checkpoint.
This does not establish the cause of the earlier block. The replacement setup
continues; retirement and final preservation checks remain pending.

The fresh Windows VM separately exposed differing VC++ servicing versions after
Build Tools installation. The new resolver accepts the current version only when
the single Microsoft bundle, both MSI records and all three DLL versions agree,
and retains the older registry entry as a removal-blocking outside consumer.
The captured-case regression failed before correction and passes in native
PowerShell, including refusal controls. Both exact historical command encodings
remain readable only for completed intents. Expanded Neovim publication and
abandonment engine tests also pass natively on Windows. The full application
rerun and combined VC++/Build Tools container lifecycle remain outstanding.

The corrected fresh Windows preview then reached configuration review but
reported 13 pending resources, including dependencies blocked by three observed
directory-size discrepancies in sealed Neovim, CMake and Python archives.
Identity, mode and modification time matched. Microsoft's API contract gives
these Windows directory-size fields no meaning. The focused correction excludes
only that comparison; fingerprint shape and all defined checks stay unchanged.
The captured-value native regression reproduced the original refusal and now
passes alongside the defined-metadata controls. The complete application rerun
remains pending.


The genuine Windows migration replacement setup now reports ready. The legacy
Git file required a same-account elevated recovery of only integration.git;
its owner/group/DACL were unchanged, and remaining setup completed as the normal
user. The diagnostic now explains this exception. Native install/remove/restore
regressions reproduce the old message's missing guidance while proving that the
refusal preserves the original file and saved intent. The native install/remove/restore recovery test passes (8.19 seconds), along
with the existing restricted-token permission/lifecycle suite (6.33 seconds).
Migration retirement is still pending.

The integrated local `make ci` gate passes with the Windows directory inspection,
owner guidance and consumer-display changes included (Go race suite 438.612s;
terminal suite 23.809s). The all-20-tools Windows preview now has zero Pending
resources: existing sealed Python, CMake and Neovim packages remain Keep, the
older VC++ registration is visible as a removal blocker, and the failed Neovim
stage/journal are disclosed. The reviewed full plan is applying. This remains
pre-delivery evidence until the full VM lifecycle, independent review and exact-
head hosted checks finish.

The all-tools Windows run completed Neovim's actual plugin, parser and Mason
phases. It then reached an Administrators-owned VS Code settings witness; its
original file was preserved and one-resource elevated recovery is being tested.
Strict editor/application smoke and the full lifecycle remain pending.

The released-installation Windows retirement review exposed incorrect Add/Admin
counts and missing scrollable exact scope. It was cancelled without mutation.
The corrected interactive path shares the machine preview's scope revalidation
for initial approval and resource recovery. Focused portable regressions pass;
the actual 27-item retirement is still pending. Opus's independent batch review
found no material code defects, with wording/documentation corrections applied
and its historical Neovim-corruption observation still under review.

The retirement privilege follow-up now also discloses vendor-managed elevation
for Chocolatey and machine/unspecified-scope winget, using existing saved native
gates/arguments. Scoop remains ordinary-user. Focused provider fixtures pass;
the actual VM retirement is awaiting the final corrected preview. Damaged Neovim
history is now local to that resource instead of aborting unrelated observations;
active corrupt publication still requires evidence recovery before abandonment.

The final follow-up local gate passes (Go race 442.659s; terminal 23.794s), and
native Windows Neovim corruption, owner recovery, retirement and terminal tests
pass. Opus accepted the saved-gate privilege derivation and local corruption
boundary with no material defect. Its Low follow-ups now also cover corrupt
current-generation deselection, journal-bearing Abandon diagnostics and accurate
user-scope winget wording. Their regressions failed before and pass after the
corrections; the integrated gate and VM completion remain pending for these last
changes.

2026-10-18 VM acceptance follow-up: Windows Neovim passed 257 strict language
checks with all 31,158 managed entries unchanged. VS Code GUI edit/save/reopen,
Windows Terminal shortcut/profile/clipboard, and divergent Git pull with rebase
passed. Real shell acceptance found private Python tool directories on PATH;
command-only publication and legacy recovery regressions pass locally. Native
validation, the remaining Windows lifecycle, migration PATH sequencing and final
source Linux Update remain in progress. Mac acceptance has not started.

The Windows private Python console lifecycle passed natively (LaTeX 120.16s,
yamllint 155.77s, generated PowerShell lookup 1.34s). Actual Opus 5.5 xhigh
confirmed the separate legacy PATH sequencing defect; exact-entry approval now
preserves unrelated changes and the current registry type, with old whole-PATH
recipes unchanged. The original migration transaction was abandoned through the
public protocol, preserving its 22 receipts; no journal edits were used.

The final native PATH suite passed, including the two-resource controller
transaction and direct changed-entry refusal. Genuine Windows migration completed
all 27 approved retirements with personal witnesses/permissions preserved; its
truthful `needs-action` result still names modified PSFzf and outside-bucket
LazyGit. Fresh Windows Update hit a Defender-terminated Python process; the final
import probe refused publication. Recovery is in progress with protection intact.
Real Herdr pane acceptance also found its bare Windows shell override bypasses
upstream process-PATH resolution; a native regression and correction are pending.

Herdr's Windows correction now passes native pane execution (84.36s), following
an 88.41s failed-before reproduction. The live Windows Terminal pane also used
managed PowerShell 7.6.3 and loaded the personal profile. Migration setup Check
returned ready without changing the state hash after all approved retirements.


Final-source Linux acceptance passes: 25 selections/91 receipts, intended Python
console updates with base Python unchanged, real Bash/zsh consumers, 257 Neovim
checks with 29,485 entries unchanged, personal files/history preserved, and a
read-only ready Check. The final Windows native Python/Herdr/probe suite passes;
genuine migration is ready with its approved retirements completed. Fresh Windows
Defender recovery preserved all 2,078 failed-stage entries and completed without
new detections. The remaining Windows lifecycle and final review/CI are pending;
Mac acceptance remains deferred.

The fresh Windows final-source Update returned ready with 20 selections and 55
receipts. Its fresh PowerShell session passed 25 command/version checks; Neovim
passed 257 checks with all 31,157 managed entries and installer state unchanged.
The subsequent Update preview exposed a spurious shell repair after a personal
CRLF edit. The correction preserves the owned block's line endings and fixes
UTF-16 saved-block validation discovered by expanded coverage. Focused tests pass;
final native/recovery checks, Opus review and the remaining lifecycle are pending.

Windows full removal exposed a completed-restoration classification defect: the
personal Code settings file incorrectly retained Code and its prerequisites on
retry. Regression coverage reproduces that behavior through a real application
lock, and covers later JSON, whole-file and Sentinel lifecycle commands. The
correction also preserves completion evidence across temporary edits, missing
journals and later preservation failures; approved runtime removal remains
blocked while evidence is uncertain. Focused tests pass. The original VM
transaction was publicly archived with personal Code permissions and the failed
Neovim journal preserved. Final native verification, review and continuation are
recorded on PR #87; Mac manual acceptance remains deferred.

The corrected Windows removal completed with all 31 personal hashes, later shell
edits and Code settings permissions preserved; reinstallation is underway. Native
Windows restored-baseline regressions pass three repetitions, with the earlier
executor failing the reselection/final-Pending cases as expected. The latest Linux
Check returned ready for all 91 resources at 20:54 UTC, with the installer state
hash unchanged. Final Windows consumers and exact final-commit gates remain pending.


## 2026-10-19 macOS acceptance and Sentinel integration — in progress

Only `dotfiles-pr87-fresh` remains after the authorized removal of other local
VMs and the cached image. The baseline is macOS 26.6.2 (25G83), arm64, Tart
2.40.1, from vanilla digest
`eeec54bfe1f076e27786c5d92b89187a05b1d109b5071eb2dcdf02d596e34640`.
Public setup provisioned Apple CLT 27.0 and Homebrew 7.0.9 through ordinary guest
authentication, paused for prerequisite replanning, and installed tmux 3.8.
An unrelated Homebrew `hello` package is seeded for later retention checks.
Original Sentinel at `d9a3993` installed and removed successfully, retaining a
personal policy byte-for-byte with mode 0600. Full application, removal/reinstall,
recovery and released migration acceptance remain in progress.

The initial acceptance snapshot pinned Sentinel source to
`f90fc0d63b0af4927f07a0f230e602dbe432238c` (version 0.2.0). Policy now targets
Codex, Claude, Copilot and Kiro. An independent skills
selection copies the 12 upstream skills for Codex/Copilot and Claude, refusing
foreign ownership and redirects. The former Helix repository is not a dependency.
Actual Opus 5.5 xhigh reviewed the design and final Sentinel implementation with
no remaining material findings. Installed Codex and Claude each discover all 12
skills once; installed model-discovery and verified Impeccable engine helpers run.
Public process-kill recovery and edited-skill Repair preserve the original evidence
and personal bytes/modes. The full local gate passed for that patch; the subsequent
macOS reboot and disk-image fixes require a new full gate and final review.

The initial local full gate exposed detached Git maintenance launched by the
Neovim managed-startup fixture commit. A trace-only reproduction without Neovim
confirmed the cause. A failed-before trace assertion and passing focused suite
verify `maintenance.auto=false` on that fixture commit; production startup and
the complete runtime immutability assertion remain unchanged.

Native Ghostty validation also truncated an inherited CI log file. Its test
wrapper now captures output through a pipe, preserving diagnostics and status;
the native redirected-log regression has failed-before/passed-after proof.

Opus implementation review found two new skills issues before guest publication:
removed receipts retaining saved edits could offer adoption of a later outside
folder, and the redirect guard rejected a system-linked home. Both have native
failed-before/passed-after regressions. The guard now distinguishes ownership
from retained recovery evidence and stops before the observed home; foreign
consumer links remain blocked. Final Sentinel review and native discovery passed.

Strict installed Neovim passes 257 checks without changing 33,232 managed runtime
entries. Actual Git divergence rebases and personal Git writes leave source bytes
unchanged. Ghostty, Code editing/saving, tmux and Herdr have native-use evidence;
AeroSpace tiling/login and Tailscale's unauthenticated service survive reboot.

The reboot exposed a retained Homebrew device-number identity defect. Its fix uses
volume UUID/inode and refuses schema-1 pre-release evidence without rewriting it.
Native remount and outside-volume protection pass. A second pristine VM now
passes corrected public bootstrap, actual reboot, read-only Check and unchanged
Update with the original volume UUID/inode ownership evidence intact.

Real desktop launches also exposed synthetic AppleDouble files from DMG transport,
invalidating Zen and Obsidian vendor signatures. Their failed bundles are archived.
The corrected versioned transport, public Update, strict vendor signatures and
actual Zen/Obsidian desktop launches pass. Obsidian saves a personal note with its
0600 mode intact. T3's browser UI discovers both unauthenticated providers and
persists its chosen appearance; provider calls remain untested.
OpenWhispr's ZIP signature and first launch pass; its onboarding terms
are left to the owner by explicit request. Fresh Remove/Reinstall passes as
recorded below; released Nix migration remains open. Tart pointer and modifier-key routing remain unreliable; actual microphone,
clipboard/paste, authentication and enrollment are not claimed as passed.

Zen and Code self-updated their pinned private bundles during normal use;
removal correctly retains both as changed. Their original payload evidence is
archived. Code's supported update preference correction has focused regression
coverage, including old theme/font receipts and personal-value restoration;
native validation passes as recorded below. Zen's supported macOS policy channels either
alter sealed vendor contents or affect outside installations sharing its bundle
ID. Actual Opus 5.5 xhigh consultation is retained; a launch-only alternative is
unproven and no signature, permission or ownership bypass has been introduced.


### 2026-10-19 Mac acceptance continuation

Fresh Remove completed through public recovery after Homebrew's root-started
Tailscale service left a root-owned executable directory. The original failure
state and partial keg were archived; only the paths named by Homebrew's service
warning had ownership restored. Exact native formula removal completed without
autoremove or dependency bypass, then public Retry completed the saved removal.
All 37 personal/failure witnesses retained their bytes and permissions. Homebrew,
CLT and unrelated `hello` remain. Changed Code/Zen payloads remain protected.
Reinstall of Code, Sentinel policy and all skills completed through public setup.

The full gate rejected a proposed separate Code update-setting resource because
it competed with the theme resource for one settings file. That design was
replaced with a single combined owner and explicit recovery-only predecessor.
Old field identities remain unchanged. The real Code settings handover
(old theme/font owner to combined owner), ten-minute updater-disabled
payload check and public removal preview pass. Code has 3,809 unchanged payload
entries; read-only Check leaves all 14,267 state entries unchanged. Full `make ci`
passed again after the Sentinel retargeting correction. Exact final-head gates
and the remaining Mac acceptance are still open.

The second genuinely fresh guest exercised current interactive setup before CLT
or Homebrew existed. Public provisioning installed Apple's CLT 27.0 and Homebrew
7.0.9, then the reviewed follow-up installed tmux. An actual reboot changed the
APFS device number while preserving volume UUID and prefix inode. Check remained
Ready and all 201 state entries were unchanged across reboot and both read-only
Checks; unchanged Update also passed with identical Homebrew package inventory.
Five personal policy/shell witnesses retained their hashes and permissions.

Current Sentinel pin `6645688` is verified through actual installed Codex and
Claude discovery in that guest: 12 skills each, exactly once, plus personal
skills. All 206 files match the pinned source bytes/modes. The four policy targets,
model-discovery helpers and verified Impeccable engine pass; provider access is
explicitly unverified. Check leaves all 10,237 state entries unchanged.

The separate pristine migration guest has run the unmodified v0.4.4 installer
through upstream Nix/nix-darwin and Nix-managed Homebrew provisioning. Completion
is waiting on Microsoft's normal Code first-launch prompt, confirmed by the
guest's security log. Tart window capture remains unavailable; owner interaction
was requested. Released migration is not claimed complete; the subsequent independent
Determinate pass is recorded below. Gatekeeper remains enabled.


### 2026-10-19 Authentic Determinate removal and interruption recovery

A separate pristine guest installed the official pinned Determinate 3.23.2
binary, producing its real macOS receipt, encrypted APFS Nix Store, daemon and
32 build accounts. Preview refused a modified user-owned vendor binary, an
outside account profile and unreadable system configuration. A real process with
its working directory in the store blocked approved removal after the daemon
stopped; original receipt/binary hashes and the complete failure journal survived.

Public Retry dispatched the validated vendor worker. Killing only the controller
after durable dispatch left that worker running; it completed with exit 0.
Subsequent public recovery consumed the recorded result and reached Ready.
The store, vendor binary/receipt, daemon, 32 users, group, APFS volume and boot
references are absent. Personal user Nix configuration retains its hash and 0640
mode; preserved system configuration retains its hash and reviewed 0644 mode.
The distinct root-owned 0640 scenario was refused before mutation, not claimed
as a successful preservation test. A later personal file at the old service path
was explicitly protected through preview and attempted apply, then the public
transaction recovered after removal of that test-only file.

Actual reboot changed the boot-session UUID. The empty synthetic mountpoint is
now absent; a fresh zsh login and repeated runtime Check are Ready. All 30 state
and personal-configuration archive entries survive reboot unchanged; read-only
Check leaves all 28 installer-state entries unchanged. The 141-entry final
state/witness archive was verified against a second live export, then the guest
was shut down cleanly. Only this stopped test VM remains registered. The earlier
blocked released guest is retained as a verified full Tart archive, not another
registered VM. Full `make ci` passed again after the Mac/Sentinel corrections;
final-head hosted results are tracked on PR #87.


### 2026-10-19 Final-head CI follow-up

Candidate `2b5342c` passes 25 checks, including all six Mac jobs and four CodeQL
checks. Its Windows engine suite completes without a timeout but rejects six
new Code fixture cases that assumed POSIX 0640 on Windows. The corrected tests
require equality with the actual original native content/mode snapshot on every
platform; focused Mac reruns pass and native Windows confirmation is pending.
Production behavior, ownership checks and the 20-minute engine limit are unchanged.

Windows quality encountered Chocolatey HTTP 503/503/504 before its tests. It now
uses the installer's existing official Neovim 0.12.5 Windows ZIP and SHA-256,
verified before extraction into the disposable runner's temp directory. The
pin-consistency regression fails before and passes after this correction. The
independent native Chocolatey retirement fixture still uses Chocolatey and retains
all its install/ownership/removal assertions; it now records bounded install
output even when the command exits zero, so a missing package can be diagnosed.
Full `make ci` passes, including 604.809 seconds of installer race tests. Actual
Claude Opus 5.5 with xhigh effort reviewed the complete follow-up and found no
material findings; its evidence-wording corrections are recorded in the ledger.
Corrected native Windows and all next exact-head hosted results remain required
before reporting this follow-up complete.


### 2026-10-19 — vanilla Mac released-fixture prerequisites (in progress)

The restored checkpoint reproduced two historical installation problems: current
Homebrew API metadata uses `run`/`set_permissions` steps unknown to released Brew
6.0.1, and Apple's Python 3.9.6 cannot install pinned setuptools 83.0.0. Enabling
NO_INSTALL_FROM_API for the whole old setup also reproduced the previously
rejected Resource::Patch DSL failure; it is not the correction. All failed logs,
package receipts, the failed venv and generated `{{pkgetc}}` tree are retained.

Actual Claude Opus 5.5 with CLI xhigh recommended a new vanilla baseline with
real compatible prerequisites, rather than accepting a mixed failed installation.
The fixture now prepares fontconfig, yamllint and Python 3.12 using the current
Brew and the existing historical taps, checks the real commands, and supplies
the versioned Python's documented PATH. Historical source remains unchanged, and
its setup retains the normal API path. Clean native preparation passes; complete
released setup and migration remain under validation. The local baseline full
`make ci` passed before this correction.


### 2026-10-19 — native Zen and released migration continuation (in progress)

The unchanged released setup completed on its second public run. Real tmux 3.8,
fontconfig 2.18.2, yamllint 1.38.0, Python 3.14.6 and pylatexenc 2.11 run; Nix
2.34.0, nix-darwin and Nix-managed Homebrew 6.0.1 are authentic. The old manager
also created a literal `{{pkgetc}}` certificate link for OpenSSL 4. Preserve that
upstream compatibility defect; this is not a completely healthy fresh fixture.
The proposed fixture prepares tmux dependencies with current Brew, leaving the
released installer responsible for tmux. Fresh one-run verification is pending.

The public migration detached 18 released profiles/configuration links, retaining
originals and readable copies. Homebrew conversion is being checked against a
55,366-entry package/tap snapshot. An old non-login SSH environment correctly
blocked conversion because it resolved Brew through `/run/current-system`;
continue in a fresh login session as the migration guide requires.

macOS Zen now selects the official native Homebrew app. Focused regressions cover
reuse, approved removal, missing-app repair, normal vendor changes, unchanged
Update, protected foreign replacements and trusted recovery recipes. Actual
native cask acceptance and final Opus/gates remain pending. Earlier private
archive bytes and their protection remain unchanged.

The native Zen handover now passes with the old launcher/private copy removed,
the official app opening through macOS's ordinary first-launch prompt, and all
47 pre-existing packages preserved. Check leaves all 154 installer-state entries
unchanged. A real signature-probe failure was preserved and recovered by exact
native uninstall followed by public Abandon; personal browser data remained.
Actual Opus 5.5/xhigh implementation consultation identified environment, user-app
reuse, local inspection-failure, shell PATH and formula-pool edge cases. These
have failed-before regressions and focused corrections. Full final validation,
upstream-Nix retirement and the fresh corrected released fixture remain pending.


### 2026-10-19 — final Zen recovery advice correction

The final actual Opus 5.5/xhigh closure review found no high or medium issue and
no remaining dedicated-Mac acceptance blocker. It identified one low issue in
the suggested manual recovery command for a missing personal Zen app: plain
Homebrew uninstall can also autoremove unrelated formula dependencies. Both
the public preview and migration guide now explicitly set
`HOMEBREW_NO_AUTOREMOVE=1` for that uninstall. The controller regression fails
before and passes after, without changing ownership or executing recovery
automatically. Fresh final review and exact-head results remain recorded on PR #87.

The follow-up review also identified periodic cleanup after manual reinstall.
Exact Homebrew 7.0.9 source confirms that path. Reinstall advice now disables
both autoremove and automatic install cleanup; both recovery alternatives are
covered by the controller regression. Native before/after and the real cask
contract pass again, preserving all 13 unrelated dependency candidates.


### 2026-10-19 — native desktop CI follows the official Zen application

The first final-head Mac desktop job exposed a stale acceptance fixture: setup
used the official Zen cask, while the consumer still expected the retired
`~/Applications/Zen.app` launcher and private archive process root. The fixture
now snapshots, launches and tracks `/Applications/Zen.app`, while also retaining
the personal `~/Applications/Zen.app` baseline and preserving all native
window, architecture, lifecycle, Check and removal assertions. A focused provider
regression fails before and passes after. The affected real desktop job and all
other exact-head delivery checks are recorded on PR #87.


### 2026-10-19 — portable and synchronized acceptance fixtures

The final CI run also exposed two Windows cask-fixture assumptions and a Linux
notification race. Personal permissions now compare the actual native before
and after modes. The command-inspection refusal fixture requires a real native
filesystem error from an overlong component instead of assuming Unix ENOTDIR.
Linux process markers publish only after their file handles close, so the
"closed" notification cannot still occupy the descriptor being inspected.
Production behavior and all preservation/refusal assertions remain unchanged.
Native verification and final exact-head results are recorded on PR #87.


### 2026-10-19 — hosted Zen first-launch consent investigation

Dedicated VM installation and ordinary desktop launch after Open consent pass,
but the hosted desktop check still waits at first launch. The actual final guest
reproduces the timeout, with Gatekeeper accepting Zen 1.23.3b as Notarized
Developer ID and the system log recording an app-specific prompt awaiting a
response. Read-only failure diagnostics now capture the hosted policy, quarantine,
window inventory and, when existing trust permits, the consent-agent Accessibility
structure without requesting permissions or pressing a button. Hosted native window proof remains required;
a timeout or direct executable launch cannot count as that proof.

The read-only native run on `6c9ba78` confirms the hosted image already grants
Accessibility to the test process. Gatekeeper is enabled, native assessment
accepts notarized Zen, and the exact ordinary Open/Cancel dialog is visible to
Accessibility. The fixture now supplies only that normal Open consent after
checking its app, origin, Apple agent and process lifetime. Missing permission or
an unmatched prompt cannot produce success. Native completion, dismissal,
quarantine approval and a real window remain required. This correction's native
result and final review/gates are recorded on PR #87. The existing image grant
is a required part of this hosted test baseline, not a permission set by setup.

### 2026-10-19 — preserve prerequisite ownership in the released Mac fixture

The `35c2e4d` matrix passes 26/27 checks, including the real Mac desktop
lifecycle and both Windows gates. Mac core exposes a fixture error: explicitly
installing an already present prerequisite promotes an automatic Homebrew
dependency to requested. Migration then correctly protects shared fontconfig
and remains incomplete; the next Determinate test correctly refuses that
unfinished transaction. No product protection or completion rule changes.

Preparation now queries the native inventory and installs only absent formulae,
refusing conflicting sources or unlisted Cellar entries. Every pre-existing
package identity must remain unchanged. A real disposable-Mac regression fails
before in 9.97s with the changed ownership/receipt and passes after in 8.58s,
preserving the dependency, its consumer and their payloads while installing the
missing prerequisite. All seven personal/system witnesses and 32 outside package
lines remain exact. The full released migration also asserts shared fontconfig,
its payload and outside consumers stay unchanged, or requires removal of the
requested unshared copy on a clean baseline. All existing Ready assertions remain.
Payload snapshots precede conversion; semantic inventory is read only after
the file-record equality check, using the same converted Brew for both reads.
The old Brew cannot read newer casks reliably. Opus caught that new test-ordering
error before publication, and it is corrected without changing product behavior.
Actual Opus consultation confirms the fixture diagnosis; final correction review,
full make ci and all 27 checks on the next exact head remain required.

### 2026-10-20 — Interactive progress, navigation and confirmation

Owner feedback from a Linux run changed the interactive surface. One alternate-
screen session now spans the menus instead of flashing to the shell. Esc/q go back
to the main menu, keeping the draft tool selection, and Ctrl-C quits. Check returns
to the menu. By owner decision the confirmation is a compact summary defaulting to
Apply, with the complete plan behind `d`; type-ahead is discarded before it opens.
Approved work prints numbered steps, verified downloads, Neovim phases, command
lines and live native output streamed from the worker, while receipts and journals
keep their bounded evidence. The final report separates actionable reasons from
notes such as unexercised applications. Setup runs as the normal user and asks for
sudo only after approval. Local PTY, engine and renderer regressions are recorded
in the review ledger. Native macOS/Windows terminal proof and a real package
installation with streamed output remain hosted/manual verification items.

### 2026-10-20 — Streamlined interactive flow

Following owner review of the progress/navigation changes, the owner asked to ship
eight further improvements. Setup inspects each resolved selection once per menu
session, starting in the background when the menu opens; plans, Check, tool
status and review revisions reuse it, and a changed system brings back the review
instead of applying a stale plan. Retention, adoption and shared-removal choices
are checkboxes on the single confirmation. Foreground sudo is requested once per
apply and kept fresh until it ends. Each apply writes a private log beside the
state, failures are shown once in the report with that log, the tool list shows
status and pending changes, and the menu is Choose tools, Update tools, Check and
Exit with a likely default. Unchanged checkouts reuse their verified build without
the Go toolchain: about 0.1 s instead of 1.3-1.7 s per launch on the local Linux
workstation. Native macOS/Windows terminal and bootstrap proof remain hosted
verification items.

### 2026-10-21 — Personal-Mac migration fixes

The owner's personal-Mac `./migrate.sh` stopped after setup on upstream Sentinel
skill ownership and printed its report twice. Canonical corrections, each with a
failed-before regression (details in the review ledger):

- Sentinel policy targets only present agents (configuration root, discovered
  command or Kiro app, or Codex/Claude Code selected); saved ownership keeps its
  approved targets, notes newly present agents and refuses to recreate a removed
  agent's folder.
- The skills guard accepts upstream's empty post-removal registry and refuses
  `npx skills` lock claims on bundled names.
- Migration's new "Remove released agent policy for OpenCode and Pi" item
  discards only exact released Sentinel 0.1.2 files; setup no longer reports them.
- A stopped migrate names its phase without repeating the printed report.

Personal-Mac state after these fixes, using the uncommitted build: upstream
Sentinel's own `--remove` cleared both skill roots (unedited), the pinned
`skills@1.7.4` CLI released its 17 standalone Impeccable entries (backup in the
Trash), the unfinished setup run was abandoned with receipts kept, Sentinel
policy was deselected (originals restored), the empty `~/.kiro` created at
04:48:44 was removed, and Sentinel was adopted again for Codex, Claude Code and
Copilot only. All 12 skills installed; setup Check is ready. Migrate's
`retire.pi` and `retire.sentinel-policy` completed (OpenCode `AGENTS.md`
removed, three released Pi packages moved aside); the unrecognized
`pi-coding-agent` 1.0.0 was removed with npm, and OpenCode's own uninstaller
plus moving `~/.opencode` to the Trash removed OpenCode. The remaining chezmoi,
gh-dash, Nix and tools retirement needs the owner's interactive `./migrate.sh`
(administrator steps).

Local verification on the personal Mac with the pinned Go 1.27.1: static checks,
lint, shell, Starship, tmux, Ghostty, AeroSpace, Renovate, bootstrap (55 tests, two
environment skips) and the installer gate (gofmt, vet, race suite) pass;
`DOTFILES_TEST_SENTINEL_SOURCE=1` reproduces both Sentinel renders. `test-nvim`
fails four treesitter specs identically on the unchanged `fee9f5d`: the specs
read the real data directory, which on this Mac now holds the managed runtime.
Hosted checks on a new head have not run; nothing has been committed or pushed.

### 2026-10-21 — Recommended follow-ups

Shipped on the owner's request: isolated Neovim spec data, removal of empty
folders a publication created, retired Sentinel targets for uninstalled agents,
`Note:` disclosures separate from unexercised applications, and result displays
that omit deleted preserved copies. Details and regressions are in the review
ledger; verification follows the same full local gate before push.

Verification for the follow-ups, review fixes and personal-Mac Nix retirement
fixes: the complete local `make ci` passes on the personal Mac (all targets,
including 465/465 Neovim specs and the installer race suite) with the pinned Go
1.27.1. Bootstrap reports two environment skips. Hosted checks run on the pushed
head.

### 2026-10-22 — Hosted result for 67aae3b; Kiro CLI and Kiro skills

`67aae3b` finished 27/27 green. The two Fresh Windows vendor jobs that exited at
their first step on `91b8257` passed without change, and the two macOS jobs
GitHub cancelled for runner capacity passed when re-run (a rerun, not a new
head). `TestNativeMacKeyboardLifecycle` passed on Linux amd64 and arm64, so the
Mac keyboard row above is native-verified.

Owner request (2026-10-22): Kiro CLI is a new optional AI coding capability on
macOS and Ubuntu/Debian, pinned to Kiro's stable 2.33.1 downloads (macOS
universal DMG, Linux glibc ZIPs); every pin's SHA-256 was re-verified from the
versioned URL. Windows was first omitted by owner choice (superseded the same day;
see below). Kiro's background self-update is
turned off by owning one field, `app.disableAutoupdates`, in
`${KIRO_HOME:-~/.kiro}/settings/cli.json`. Sentinel skills gain a Kiro root
(`${KIRO_HOME:-~/.kiro}/skills`) that follows the policy's present-agent rule, and
the policy's Kiro target now honours KIRO_HOME and counts a Kiro CLI selection as
present. Regression tests cover pins and routing on all four targets, the Linux
layout, settings ownership, KIRO_HOME resolution, absent/present/uninstalled Kiro
and the single Impeccable note. The native agent CLI lifecycle now also runs Kiro
CLI on macOS and Linux; it is pending its first hosted run.

GitHub Copilot CLI (owner request, same day) is a new optional capability on all
four targets, pinned to the official github/copilot-cli v1.0.102 standalone assets
(each verified against the release's SHA256SUMS.txt; one executable per archive;
Linux builds need glibc 2.28+). Windows adds the shared PowerShell 7 prerequisite.
The selected shell sets `COPILOT_AUTO_UPDATE=false`, the only switch Copilot
honours; no Copilot settings are written. The Sentinel policy follows
COPILOT_HOME and counts a Copilot CLI selection as present, and Sentinel skills
copy to `$COPILOT_HOME/skills` only when COPILOT_HOME is set (otherwise Copilot
reads the existing `~/.agents/skills`). Unit regressions cover pins, layouts,
the shell switch and both skills cases; the native agent lifecycle runs
`copilot --version` on all four targets and is pending its first hosted run.

### 2026-10-22 — Kiro CLI on Windows and weekly agent CLI pin proposals

Owner decisions (2026-10-22): Kiro CLI is offered on Windows too, by private MSI
unpacking ("Unpack MSI privately"), and the agent CLIs stay pinned with weekly
bump pull requests.

- **Windows Kiro CLI:** the 2.33.1 per-user MSI (SHA-256 `f1c7e603…b190dedd`,
  267,489,280 bytes, signed by Amazon Web Services, Inc., OU AWS Kiro) has one
  file, `kiro-cli.exe` in `LocalAppDataFolder\Kiro-Cli`, no CustomAction rows and
  an AdminExecuteSequence of standard cost/copy actions, and its summary
  information declares no elevation (Word Count 10). The new `kiro_msi`
  preparation runs `msiexec /a` through system Windows PowerShell into a private
  stage and keeps exactly `kiro-cli.exe`; an unreviewed image is refused and
  preserved, and failures report the msiexec log tail. Scoop's Main bucket
  unpacks the same MSI the same way. `integration.kiro` now also applies on
  Windows, and managed PowerShell sets `KIRO_NO_AUTO_UPDATE=1`. Cross-platform
  regressions cover the pin validation, fake-worker unpacking, refusal and log
  reporting, and the Windows shell switch (fails without it).
  `TestNativeKiroMSIUnpacksPrivatelyWithoutElevation` (new Windows step) and the
  agent CLI lifecycle's Windows Kiro run are pending their first hosted run.
- **Windows Kiro CLI, revision 1 (later the same day):** hosted runs showed the
  agent lifecycle passing once its fixture passed KIRO_HOME (1dca9af), but as an
  ordinary restricted user msiexec ran its full 10-minute bound without creating
  its log or any file. `kiro_msi_revision: 1` drops Windows Installer: a stdlib Go
  reader copies the MSI's one embedded cabinet (`#cab1.cab`, LZX, 267,057,411
  bytes), its header must hold exactly `KiroCliExe` of `kiro_msi_file_size`
  (401,055,928), and the system `expand.exe` expands it; the result must match
  that size and start with `MZ`. The reader reproduced the cabinet of the pinned
  MSI byte for byte (SHA-256 `7a3dd53d…7f16298f`, cross-checked by an independent
  Python read). Cross-platform tests cover a synthetic compound file (exact copy,
  missing, duplicate, mini, looping, short and out-of-file chains, links), the
  cabinet header and the fake expand.exe outcomes; the hosted restricted-user run
  is pending.
- **Windows Kiro CLI, revision 2 (later the same day):** hosted runs refused
  revision 1: as an administrator expand.exe left `.prepare-kiro.cab` instead of
  `KiroCliExe` (refused as an unreviewed layout), and as the restricted user it
  could not start in the stage ("The directory name is invalid").
  `kiro_msi_revision: 2` runs no external program: `installer/cabinet_lzx.go`, a
  stdlib cabinet/LZX decoder written from Microsoft's published descriptions,
  streams `KiroCliExe` out of the copied cabinet, and the result must match
  `kiro_msi_file_size` and the new `kiro_msi_file_sha256`
  (`908c4925…7982dc1cb01a0ee737b3006f7a8728`). expand.exe, its discovery and the
  native command's working directory are removed; saved revision-0/1 pins stay
  readable and are refused for preparation. Local proof (decoding only, nothing
  executed): the real 267,057,411-byte cabinet decodes to the pinned
  401,055,928 bytes and SHA-256 in 1.85 s, and the whole preparation from the
  pinned MSI takes 2.8 s. That cabinet uses verbatim, aligned and uncompressed
  blocks and E8 translation, and libarchive's `bsdtar` produces the same
  SHA-256. Cross-platform tests encode synthetic LZX with every block kind,
  repeated offsets, length-run forms, window wrap, CFDATA splits inside words and
  E8 cases, plus layout, checksum, truncation and invalid-stream refusals; all 12
  decoder mutations tried were caught. The hosted restricted-user run is pending.
- **Weekly pin proposals:** `.github/workflows/agent-cli-pins.yml` runs
  `scripts/update-agent-cli-pins.py` (16 offline unit tests). It changes only
  version, URL and SHA-256 after vendor signature, integrity and layout checks,
  plus Windows Kiro's file size and SHA-256 from libarchive's `bsdtar`.
  Opening pull requests needs the owner's GitHub App variable/secret
  (`AGENT_PINS_APP_CLIENT_ID`, `AGENT_PINS_APP_PRIVATE_KEY`) because the
  repository's `can_approve_pull_request_reviews` is false; until then runs only
  verify and upload the patch. The workflow has not run on GitHub yet.

### 2026-10-22 — Linux Obsidian portable build

- **Owner decision:** Linux Obsidian moves from the private Flatpak to the official
  portable `tar.gz` 1.14.4 (`tool.obsidian-portable`, amd64 and arm64; SHA-256
  matching GitHub's release digests), launched with `--no-sandbox %U` and the
  vendor's `StartupWMClass=md.obsidian.Obsidian`. macOS keeps its DMG pin.
- **Why:** on the owner's Ubuntu 24.04 laptop the Flatpak failed twice — AppArmor
  `complain` mode on the stock `flatpak` profile blocked bwrap's user namespace,
  and Electron's helper spawn through the Flatpak portal could not see the
  private store ("commit ... not installed").
- **Transition:** `tool.obsidian` keeps its Flatpak binding only to observe and
  remove earlier receipts. Choose tools removes it (approved, keeping
  `~/.var/app` data and the private runtime); Update installs the portable build
  and repairs the launcher but retains the Flatpak.
  `TestObsidianFlatpakReceiptsMoveToThePortableBuild` fails on the previous
  catalog (Linux closure still selected `tool.flatpak`/`tool.obsidian`).
- **Pending hosted proof:** the Linux desktop lifecycle now launches Obsidian on
  amd64 and arm64 under the stock `kernel.apparmor_restrict_unprivileged_userns=1`
  (window with the declared class, `--no-sandbox` in the process arguments,
  update and removal), and the native Flatpak step removes a real earlier
  Flatpak while installing the portable build.

### 2026-10-22 — Kiro CLI withdrawn from Windows

Owner decision, later the same day: Kiro CLI is not offered on Windows. Kiro
ships its Windows CLI only as an MSI installer. Its normal install is machine-wide,
needs elevation and leaves Kiro's updater in charge; extracting the program from
the MSI worked technically (the pure-Go decoder reproduced the reviewed bytes) but
the owner preferred not to rely on unpacking a vendor installer ("If not ... just
leave it out of Windows"). The Windows pin, the `kiro_msi` preparation, the MSI and
cabinet/LZX readers, their tests, the restricted-user MSI step, the Windows
`KIRO_NO_AUTO_UPDATE` shell line and the bump script's MSI checks are removed; the
catalog restores macOS/Linux-only `kiro-cli`, `tool.kiro` and `integration.kiro`.
The entries above stay as history. Kept: the KIRO_HOME fixture variable, the
restricted-child timeout/file-output harness and the Windows race budget. This is a
limitation of Kiro's distribution, not of the platform: a plain Windows archive
from Kiro could be pinned like the macOS/Linux builds.

### 2026-10-23 — OpenWhispr Linux launcher window class

Field report from the corporate Ubuntu laptop: OpenWhispr's window showed a
generic gear icon in the GNOME dock. The pinned 1.10.3 Linux archive's main
process calls `app.setDesktopName("open-whispr.desktop")`, and Electron 41.10.7
derives WM_CLASS from that name (`open-whispr`). The private launcher
`dotfiles-openwhispr.desktop` declared no `StartupWMClass`, so GNOME could not
match the window. The launcher now declares `StartupWMClass=open-whispr`.
`TestLinuxLaunchersDeclareTheirWindowClass` fails on the previous recipe and
passes now. The hosted Linux desktop lifecycle (amd64; no arm64 asset exists)
requires an OpenWhispr window carrying that class. Existing installs show the
launcher as an update; Update rewrites the owned launcher in place.

Agent pin proposals: the repository has the owner's `AGENT_PINS_APP_CLIENT_ID`
variable and `AGENT_PINS_APP_PRIVATE_KEY` secret (both set 2026-10-22). The
workflow exists only on this branch, and GitHub runs scheduled and dispatched
workflows from `main`, so its first run, and the first proof that the App's
installation can open a pull request, follow the merge.

Hosted macOS fixture after the 2026-10-21 runner image: the released-setup
prerequisite guard accepts only a newer, linked Homebrew keg added beside a
pre-existing one, with ownership evidence unchanged (owner decision 2026-10-23;
see the review ledger). Native proof is the next hosted core (macos-26) run.

`49076d1` passed all 27 checks. Its macOS core job ran on the older 20261013 image,
so the new prerequisite rule still awaits native proof on the 20261021 image. The
Windows Build-Tools first-inspection timeout is intermittent; its diagnostic step now
runs before the lifecycle (see the review ledger).

`451a525` (24/27): fixed after owner decisions on 2026-10-23. Windows Build Tools
inspection now has a 120-second bound, because cold first inspections with Visual
Studio exceeded 30 s on hosted windows-2025 20261021 and stopped the installer. The
Linux Nix process scanner no longer treats pidfd readiness alone as exit: a race
during a worker thread's exec let it omit a live store consumer, reproduced and
fixed with a regression test (details in the review ledger).

### 2026-10-23 — zsh from bash terminals

PR #87 merged as `a64f804` after 27/27 checks on `33b61b7`. Follow-up capability
`zsh-handoff` lets an account whose bash login shell cannot be changed (company
directory accounts) use zsh in Ghostty, Herdr, tmux and SSH: interactive terminal
bash hands over to zsh with `SHELL` exported. Verified on macOS (bash 3.2) and in a
disposable Ubuntu 24.04 container (bash 5.2, real zsh); hosted proof runs on the
pull request head. Details in the review ledger.


### 2026-10-24 — Neovim as the shell and sudoedit editor

PR #91 merged as `a6246ba` after 27/27 checks on `9d1a38a`. Owner request:
selected Neovim exports `EDITOR`/`VISUAL=nvim` and `SUDO_EDITOR` as the managed
nvim's full path on macOS and Linux, so `sudo -e /etc/hosts` opens the configured
Neovim. Verified in disposable Ubuntu 24.04 (sudo 1.9.15) and 26.04 (sudo-rs
0.2.13) containers with real sudo; hosted Linux and macOS sudo run on the pull
request head. Details in the review ledger.

### 2026-10-24 — APT refresh report and profile blocks after formatting

PR #92 merged as `9ec6a9b` after 27/27 checks on `a9cfa40`; its real sudoedit test
passed on macOS 26 (Apple sudo) and Ubuntu 26.04 amd64/arm64 (sudo-rs). On the
laptop, an unrelated APT source with an expired key stopped the package-list
refresh, and the report repeated APT's output. The report now shows it once and
says to fix or disable the failing source, then resume. By owner decision the
refresh is now plain `apt-get update`, so a source that only warns no longer stops
setup; saved strict refresh commands stay valid.
Saving the setup-created `~/.zshrc` in Neovim let shfmt drop the empty line
above the opening block, and setup then refused every tool. A block whose start
marker opens the file is now recognized, and the marker error names the file.
Details in the review ledger and field findings (Ubuntu 16 and 17).
