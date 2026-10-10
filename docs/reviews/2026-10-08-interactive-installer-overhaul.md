# Interactive installer review reconciliation

Date: 2026-10-08. Source baseline:
`ae9a6446eb0a837a144d77d2a6345db967c61e4e`.
Status: **RECONCILED DESIGN; implementation and runtime candidates remain open.**

The [canonical plan](../plans/interactive-installer-overhaul.md) incorporates
the primary repository assessment and an independent Claude CLI review. This
ledger preserves accepted findings, unresolved hypotheses and rejected claims.
It does not turn proposed behavior into a promise made by the current installer.

## Review provenance and limits

The independent review ran directly through the installed Claude CLI with
`--model claude-opus-5-5 --effort xhigh`, in read-only plan mode, safe/restricted
mode, with only Read/Glob/Grep, strict MCP configuration, no skills, and no session
persistence. The result confirmed `claude-opus-5-5`, completed successfully, and
reported no permission denials. The CLI accepted the explicit `xhigh` request;
the result did not expose a separate server-side effort value. The reviewer
performed source inspection, not tests or installer execution, and made no repo
edits. Severity below describes impact within the proposed lifecycle unless
explicitly called a current behavior defect.

References below are paths and symbols at the baseline above; line numbers are
provided for the most consequential traces. Reproduce open runtime candidates
on disposable target machines before claiming a confirmed runtime failure.

## Current implementation findings

### C1 — High: redirection implicitly selects everything

- **Proof/source:** `setup.sh:128-132` sets `ALL=1` without a TTY;
  `setup.ps1:498-506` sets `All` when input or output is redirected.
- **Cross-check:** both public entrypoints; existing CI explicitly requests All.
- **Disposition:** confirmed, high confidence. Remove this inference in the new
  and compatibility paths; require an explicit unattended request. Test stdin
  EOF, output-only redirection, logging pipelines and an empty request. This is
  an intentional change to behavior documented in `README.md:791-793`, not a
  claim that the old implementation violated its stated contract; see F6 below.

### C2 — High: Windows compiler provisioning follows All, not Neovim

- **Proof/source:** `install-deps.ps1:3522-3527`,
  `Install-VsBuildToolsWhenAll`, returns unless `IsAll`; its orchestration call
  is at line 3852. Existing Pester coverage encodes that gate.
- **Cross-check:** `setup.ps1` has a second toolchain detector and
  `Invoke-NvimSyncPhases` does not act on the DevShell helper's Boolean result.
  The DevShell invocation selects x64 target/host explicitly.
- **Disposition:** confirmed coupling, high confidence; fresh Windows failure
  remains to be reproduced for each claimed architecture. Resolve compiler/SDK
  from the Neovim resource graph and propagate unavailable-toolchain failure.
  Reuse a suitable existing full Visual Studio installation without claiming it.

### C3 — Medium: skipped work can still end with a success summary

- **Proof/source:** `setup.sh` Neovim phases skip when `nvim` is absent;
  `install-deps.*` gh-dash paths can defer work pending authentication.
- **Cross-check:** both dependency scripts and final setup status paths.
- **Disposition:** source-confirmed status gap for the proposed ready-state
  contract, high confidence. Selected capabilities need ready / needs-action /
  failed outcomes; distinguish an intentional legacy skip from a completed
  selected capability. Do not automatically initiate account authentication.

### C4 — Medium: Pi theme removal lacks a previous-value baseline

- **Proof/source:** `scripts/configure-pi-theme.mjs:117-135` selects a managed
  theme, then unsets a managed theme on removal; it does not retain the prior
  theme value.
- **Cross-check:** setup and uninstall on both platforms call this helper.
- **Disposition:** confirmed behavior, high confidence. Initial theme selection
  is currently intentional, not itself an unauthorized-write defect. The new
  restoration contract needs a scoped prior value plus compare-before-restore,
  preserving subsequent edits and all unrelated Pi settings.

### C5 — Medium: lifecycle scope exceeds today's uninstall contract

- **Proof/source:** `uninstall.sh`, `uninstall.ps1`, README removal guidance, and
  `tests/linux_owner_lifecycle.sh` deliberately retain installed packages.
- **Cross-check:** macOS lifecycle coverage also expects package retention.
- **Disposition:** confirmed design gap, not a current config-only uninstall
  bug. The overhaul must inventory package, PATH, shell, font and integration
  effects and own their safe removal. Tests must change only when that explicit
  contract changes, not to disguise broken legacy behavior.

### C6 — Medium candidate: WSL GUI context can differ during removal

- **Proof/source:** `setup.sh` accepts a per-run experimental WSL GUI override;
  `uninstall.sh` managed-target enumeration does not pass that override.
- **Cross-check:** `home/.chezmoi.toml.tmpl` defaults the value to false;
  `home/.chezmoiignore` uses it to gate GUI targets.
- **Disposition:** source-traced candidate, medium confidence; reproduce an
  override install followed by default uninstall. Persist effective selection
  context and compare the installed target set, rather than new defaults.

### C7 — Medium candidate: Rust formatter/checker prerequisites are unmapped

- **Proof/source:** `nvim/lua/plugins/conform.lua:11` references `rustfmt`;
  `nvim/lua/plugins/lsp-config.lua:109` configures clippy. No matching install
  path was found in dependency scripts, Nix package lists or Mason's tool list.
- **Cross-check:** both native installers and the shared Neovim configuration.
- **Disposition:** missing source mapping confirmed; clean-machine runtime
  failure remains unverified. Prove and provision the enabled language stack,
  or make a separately reviewed capability/config change. Do not silently call
  the currently enabled tools optional.

### C8 — Medium: validators can repair the installation they assess

- **Proof/source:** `tests/greenfield/validate.sh:335-337` and
  `tests/greenfield/validate.ps1:310-322` invoke Lazy, parser and Mason sync.
- **Cross-check:** both validators contain earlier assertions too; the claim
  that they repair before making *any* assertion is rejected as too broad.
- **Disposition:** confirmed evidence limitation, high confidence. Verify
  installed state without repair first, then test repair separately. Hosted
  preinstalled Rust/Visual Studio concealing gaps is a hypothesis, not a measured
  failure from this review.

### C9 — Medium: support claims require a more precise test matrix

- **Proof/source:** README's native manager list includes apt/dnf/pacman/zypper/
  apk, while `.github/workflows/e2e-install.yml` describes Ubuntu install
  coverage. `flake.nix` exposes aarch64-darwin and x86_64/aarch64-linux.
- **Cross-check:** container entrypoints, native lifecycle tests, Windows x64
  DevShell arguments and the Windows Herdr artifact selection.
- **Disposition:** confirmed coverage distinction, high confidence; do not
  infer universal Windows ARM support or absence of all ARM coverage. Publish
  an exact supported table and prove every claimed cell, or obtain an explicit
  scope decision. Linux containers do not prove native desktop/WSL behavior.

### C10 — Medium: Pi config currently requires Node outside Pi selection

- **Proof/source:** `setup.sh:273` requires Node for Pi config;
  `setup.sh:1911` calls the merge in the general config phase. The Windows
  equivalent is in `setup.ps1`; both uninstall paths also require Node cleanup.
- **Cross-check:** package installation, config publication and cleanup paths.
- **Disposition:** confirmed all-install coupling, high confidence; it is not
  evidence of an existing selective-install contract. Gate Pi resources by
  selection and keep the cleanup runtime until its final consumer is removed.

## Design-review dispositions

| ID / severity | Independent concern | Consolidated decision and evidence |
|---|---|---|
| D1 / Critical | A first selected Nix profile can remove old packages; provider removal semantics differ | Accept. `nix/home/common.nix` and platform modules install fixed sets today; macOS explicitly uses `cleanup = "none"`. Preserve the observed prior profile through adoption, then project selections. Reject blindly marking every legacy root selected as proof of ownership. |
| D2 / High | Machine selections must not dirty exact release checkouts | Accept. Persist outside source and bind the plan to source/manifest identity. Passing a validated external selection into existing Nix evaluation is an implementation prototype, not permission to mutate the flake or bypass migration. |
| D3 / High | Selective config can break live links and shared shell behavior | Accept the hazard. Prove target/run-script selection, shared integration context and parity. Reject a categorical ban on templates: the repo already uses templates safely; the constraint is preserved ownership and live-source semantics. |
| D4 / High | Initial feature lists omitted Herdr, Git integration, clipboard and platform helpers | Accept. Include all user-facing roots and map every current operation. Clipboard and Ghostty helpers are contextual dependencies/integrations, not necessarily top-level checkboxes. |
| D5 / High | Bare depends-on edges omit cleanup ordering, host scope and active-provider identity | Accept requirements using minimal explicit metadata. Do not build a general package solver or duplicate providers' internal library graphs. Check actual executable resolution and ABI/version constraints. |
| D6 / High | A compiled launcher changes bootstrap and release trust | INITIAL DISPOSITION SUPERSEDED BY F1 BELOW. Initially accepted the distribution work but rejected checksum circularity and the extra-asset prohibition based on the named-asset validation path. That inspection missed the published-release resume path; the rejection was too broad. Local builds remain unacceptable as a default bootstrap prerequisite. |
| D7 / Medium | TUI must yield control to sudo/UAC/chsh and survive host replacement | Accept. The adapter owns the privileged operation; the interface hands over terminal control and journals/resumes. Test actual PTY/ConPTY and OS elevation, not only fake prompts. |
| D8 / Medium | Existing installations have incomplete ownership history | Accept. Current owner receipts/probes are useful but not evidence of original creation or previous settings. Adopt only with evidence and a reviewed preview; preserve uncertain resources. |
| D9 / Medium | Some removal cannot safely undo global infrastructure or shared state | Accept explicit retention of pre-existing/external/shared resources and managers. Prove upstream Sentinel removal and scoped VS workload behavior. Reject declaring the requested overhaul complete merely by labeling unfinished adapters install-only. |
| D10 / Medium | Full native lifecycle coverage is expensive and runners are preprovisioned | Accept focused PR tests plus scheduled/full release matrices. Do not substitute hosted package presence, mocked tests or an optional future matrix for required clean-machine release evidence. |

## Hypotheses still requiring targeted proof

| ID | Candidate | Required proof / status |
|---|---|---|
| H1 | Windows `Ask` accepts EOF as yes | Exercise the helper and public entrypoint with redirected empty input; distinguish helper semantics from C1's confirmed public All inference. OPEN. |
| H2 | User PATH updates lose raw expandable registry representation | Inspect/test raw value and registry kind before/after `Add-DirectoryToUserPath`; do not claim corruption from a .NET API assumption alone. OPEN. |
| H3 | Windows ARM parser DLLs are built with an incompatible x64 ABI | Test the actual Neovim/provider/compiler architecture combination on the claimed target. Source contains x64 arguments; a failing ARM load was not observed. OPEN. |
| H4 | Hosted Rust/Visual Studio masks missing bootstrap dependencies | Record hosted baselines and compare with clean native fixtures. Plausible explanation for green tests, not independently measured. OPEN. |
| H5 | Pinned Sentinel lacks a suitable global uninstall | Inspect its pinned upstream contract and prove scoped removal. Reviewer did not inspect the upstream implementation; absence is not established. OPEN. |

## Additional consolidation constraints

- Preserve all four native Windows chezmoi destinations and the separate
  Windows Terminal transaction. Do not put Terminal targets into chezmoi.
- Nix continues to own POSIX packages only; chezmoi owns config. Keep native
  Neovim/tree-sitter and vendor GUI provider boundaries.
- Login shell, `/etc/shells`, fallback startup guards, PATH, npm prefix,
  notes-vault exports/directories and theme settings require scoped receipts.
  Do not capture secret-bearing whole files as generic ledger values.
- `pwsh.exe` is referenced by Windows WezTerm, Herdr and psmux configuration;
  Windows Terminal's fragment also launches it. Selecting those tools must
  resolve their configured shell. A merely installed external optional binary
  must not implicitly enable an unselected integration.
- Repair and unattended control remain supported through the same engine;
  removing human-facing argument memorization must not remove CI/recovery access.

## Verification record

- Baseline local `make ci`: **PASS**, macOS, on the source revision above before
  this documentation change. This is regression evidence for the existing repo.
- No proposed installer engine exists in this change. No clean-machine
  macOS/Linux/Windows lifecycle test of that future engine has run.
- Final documentation verification and independent diff review are recorded in
  the delivery PR. They must not be counted as resolution of C6/C7 or H1-H5.

Append future reproduction results and status changes here with their revision
and environment. Preserve the original candidate and rejection rationale.

## Follow-up documentation review and corrections (2026-10-08)

A second direct CLI run with the same model/effort and read-only restrictions
reviewed the six-file consolidation. It confirmed `claude-opus-5-5`, completed
without permission denials, and reported nine documentation findings. The
primary reviewer checked the consequential source traces and incorporated these
corrections; this is documentation resolution, not implementation verification.

| ID / severity | Finding and evidence | Resolution |
|---|---|---|
| F1 / High | `scripts/release.py:1469-1471` requires the already-published release to contain only `release-proof.json` before resuming closure. The earlier inspection of lines 229-235 and 1124 onward missed this path. Certification after a build also does not tell a same-tag bootstrap launcher which digest to trust. | Correct D6 and the plan: release asset policy must change. Stage 2a must choose and prove authenticated digest/source binding, missing-proof failure, tamper and offline behavior without uninstalled tools. Build-then-certify solves the certificate ordering only. |
| F2 / Medium | A per-user ledger cannot establish authority over machine-wide resources or prove the absence of manual, other-user or WSL consumers. npm packages can outlive the root that installed Node. | Add a scope/retention table, named provider checks, persistent Keep choices and per-item opt-in for proved-owned resources with unknown external use. Known consumers block removal; pre-existing resources stay outside removal authority. Add multi-user/global-npm/WSL cases. |
| F3 / Medium | Full clean-machine acceptance requires actual evidence infrastructure and ownership before implementation depends on it. | Stage 1 now assigns VM/image/WSL host, executor, cost/schedule owner and revision-bound acceptance for every supported cell, including qualified owner-run ledger evidence. Correct an overstatement in the review: native hosted Ubuntu setup exists too; the container is not the only current Linux install proof. |
| F4 / Medium | A later older-release run can re-add packages/config after first adoption; the next Nix projection could remove them if initial adoption were the only protection. | Detect external/older-release drift on every reconciliation, preserve unowned observations pending review, and test new engine -> older exact release -> new engine plus incompatible ledger schemas. |
| F5 / Low | C7 is wider than rustfmt/clippy: inventory the whole Rust toolchain and standard-library sources. `tests/nvim/language_matrix.lua:43` and `tests/nvim/lsp_smoke.lua:496` already expect rustfmt. | Expand the inventory to rustc/Cargo/rustfmt/clippy/rust-src and link official rust-analyzer source requirements. Runner masking remains H4, unmeasured; no runtime candidate is closed by this text correction. |
| F6 / Low | Redirection-implies-all is documented, so compatibility concerns include semantics, not just argument spelling. | State the intentional breaking change: noninteractive execution without an explicit request exits nonzero before mutation with an actionable hint. Include it in CI/runbook migration. |
| F7 / Low | Implementation "phase" references collide with current setup phase numbers. | Use delivery "stage" consistently for the plan. |
| F8 / Low | Combining the Go feasibility prototype and committed engine in one stage obscures the gate order. | Split stage 2a (distribution/terminal/trust feasibility) from 2b (planner/ledger/projections), with 2a required first. |
| F9 / Low | The explicit Windows PowerShell consumer list omitted Windows Terminal. | Add the fragment's `pwsh.exe` consumer, confirmed at `windows-terminal/settings.fragment.jsonc:70`. |

F1-F9 are addressed in the consolidated design. The implementation stages,
runtime candidates C6/C7 and hypotheses H1-H5 remain open. The second review
did not run tests or inspect private invocation records; model identity and
successful local gates were independently observed by the primary reviewer.

A third focused Opus CLI review confirmed the nine corrections and found no
blocking issue. It raised two low-severity wording gaps, both corrected here:
the per-user runtime policy now explicitly blocks removal when another consumer
is detected, and the roadmap has one numbered entry per delivery stage, including
the trust/evidence prerequisites and the 2a-before-2b gate. The primary reviewer
verified those exact text changes; no further runtime claim follows from them.

## Reconciliation of the recovered independent assessment (2026-10-08)

The user clarified that reconciliation means reconciling the primary proposal
with Opus's assessment. This delivery updates that canonical plan in one commit
and one PR. It does not deliver the proposed installer. The separate draft
[#87](https://github.com/luisgui1757/dotfiles/pull/87), observed at `44308da`,
contains experimental planner/state work and leaves production adapters,
selection projections, the user interface and native lifecycle proof unfinished.
The "implementation not started" statuses belonged to superseded iterations of
this PR before the reconciliation at `b3b706e`; those iterations are not retained
in the single delivered commit. Current status distinguishes this experiment
from an implemented production rollout.

The recovered assessment completed against the immutable baseline `ae9a644`:
exit 0, terminal result success, no permission denials, 96 tool calls restricted
to Read/Glob/Grep, all observed file paths inside the source snapshot. Runtime
messages identified `claude-opus-5-5`; `--effort xhigh` was requested and supported
by the installed catalog, but effective server-side effort was not exposed.
The following dispositions reconcile its additional objections with the plan.
They are design decisions and source observations, not runtime reproductions.

| ID / severity | Independent recommendation | Reconciled disposition and source |
|---|---|---|
| R1 / High | Inspect package-manager cascading removals | ACCEPT. Inspect the entire transaction, require every effect to belong to the approved removable set and enforce that boundary at execution. A preview alone does not prevent drift: APT simulations do not lock; DNF remove includes dependents and default cleanup. The plan links their official documentation and requires an external-dependent race case. |
| R2 / High | Restore the login shell before removing zsh | ACCEPT. `install-deps.sh:734-925` changes the account shell, `/etc/shells` and an owned bash block; current uninstall does not reverse the account shell. Restore only a valid recorded baseline while the current value still matches our write. Otherwise retain the active shell with needs action. This is a requirement for future package removal, not a current package-removal bug. |
| R3 / High | Add verification that cannot bootstrap Neovim | ACCEPT. `nvim/init.lua:15-25` calls the pinned-checkout repair path and Lazy setup; Lazy's documented default installs missing plugins. C8 also covers explicit validator provisioning. Add check-only proof, disable provisioning, assert artifacts were not repaired, then exercise actual runtime behavior. Preserve invariant 23's executable-cache proof. |
| R4 / High | Prove the VS toolchain with compilation/linking | ACCEPT. `setup.ps1:429-478` and `install-deps.ps1:3300-3303` detect the VC tools component; that alone does not prove usable SDK/UCRT and selected architecture. Verify a C link plus parser build/load. Retain provider/component scope; never infer removal authority from a detected VS path. |
| R5 / Medium | Treat Rust as external and gate its smoke test | REJECT AS A COMPLETION SHORTCUT. Reuse a compatible user toolchain without claiming it, but all promised bundled language behavior needs resolved requirements. C7/F5 remain open until bootstrap and runtime proof exist. Missing promised tooling is failed/needs action; changing the language scope needs an explicit reviewed configuration change. |
| R6 / Medium-high | Keep separate WSL/host ledgers and an explicit host root | ACCEPT WITH SCOPE CLARIFICATION. Add a Windows WSL-support root for host clipboard/font resources. Keep terminal applications independently selectable rather than forcing Windows Terminal for every guest. Host removal follows F2; guest discovery never mutates the host or starts distros to infer ownership. |
| R7 / Medium | Persist selection in every configuration/package owner | ACCEPT. `.chezmoi.toml.tmpl`, four Windows states and `flake.nix` must consume a validated persisted projection. Direct chezmoi/Nix operations cannot default to all after deselection. Retain D1/F4 adoption protection. macOS package changes through embedded Home Manager can require privileged nix-darwin activation. |
| R8 / Medium | Build the controller through Nix on the target | REJECT AS THE DEFAULT LAUNCH PATH. It would activate a provider before selection/Apply. Keep Go as a feasibility-gated candidate; prove compiler-free launch and an exact-source unreleased build/artifact lane. F1's authenticated digest and release-asset gates remain mandatory. |
| R9 / Medium | Simplify the journal and avoid choosing a TUI framework early | ACCEPT. Keep intent, verified outcomes and explicit retry/abandon; re-probe after interruption. Reuse local composite transactions. A numbered checkbox mode may satisfy accessible input; actual PTY/ConPTY and psmux behavior decides the framework. No general transaction system is required. |
| R10 / High removal risk | Adopt strong hashes/profiles as ownership; delete Neovim tool directories | QUALIFY. Hashes and provider identity prove current identity, not who created an artifact. Existing receipt evidence must establish ownership. Remove only proved owned contents; preserve foreign plugins/tools and Neovim state, undo/shada and notes. Whole data-root deletion is not authorized by a feature deselection. |
| R11 / Medium | Reduce costly real-install combinations | ACCEPT SCHEDULING, NOT MISSING COVERAGE. Exhaust feasible pure selections and test transitions; distribute real runs across PR/scheduled/release lanes. Every claimed singleton/shared edge and both removal orders still need native evidence before release. Graph tests cannot establish an exhaustive application dependency inventory. |
| R12 / Medium | Broaden providers, side effects and upstream removal inventory | ACCEPT. Stage 1 covers native package managers, npm, venvs, gh/VS Code extensions, PSGallery, pinned checkouts/downloads, fonts and policy blocks, not merely Nix versus native. Existing H5 remains an unverified Sentinel removal contract; no absent upstream implementation is asserted. |
| R13 / Medium | Same-tag binary hashes require careful build boundaries | QUALIFY F1. A same-tree digest is circular only if its containing bytes affect the binary's build identity. A reproducible controller-only subtree could avoid this; the plan no longer states unconditional impossibility. It still requires authenticated digest/source binding and proof of the chosen mechanism before distribution, and the source-confirmed release-asset policy change remains necessary. |

At `b3b706e`, the change touched six design/navigation documents (two new).
The baseline local `make ci` passed on `ae9a644` before these edits. The previous
PR #86 Ubuntu job failed in the unchanged concurrent zsh publisher test
([job 113159877121](https://github.com/luisgui1757/dotfiles/actions/runs/37731008840/job/113159877121));
that result is not erased by a local pass. Final local checks, exact-head hosted
results and the requested post-reconciliation Opus review belong to this PR's
delivery record. No prototype, platform lifecycle or open runtime candidate is
certified by documentation checks.

## Post-reconciliation review and verification repair (2026-10-08)

The first post-reconciliation pass inspected the document snapshot subsequently
committed as `b3b706e`. It returned successfully as `claude-opus-5-5`, with xhigh
requested and effective effort unexposed, using only Read/Glob/Grep (69 calls).
However, four reads of the supplied diff/evidence outside its allowed source
directory were denied. It checked source claims but could not verify the full
delta or gate evidence. This is an incomplete review, not final approval. The
next pass receives the complete sanitized diff and evidence inside its permitted
source snapshot, without expanding permissions or adding tools.

| Finding | Disposition |
|---|---|
| M1 / Medium: AeroSpace, ripgrep and fd absent from the initial inventory | ACCEPTED. Add an explicit macOS AeroSpace root with cask/config, retained tap and user-controlled Accessibility grant. Add independently selectable ripgrep/fd roots that also satisfy Neovim dependencies. The exhaustive inventory gate remains open. |
| L1 / Low: direct Nix commands lack pure/impure context | ACCEPTED. Name the supported impure invocation and validated target environment; pure/CI evaluation uses explicit fixture selection. Unsupported activation fails before mutation. |
| L2 / Low: stage 1 status overstates prototype progress | ACCEPTED. Record only the partial manifest, leave complete inventory/evidence assignments/reproductions open and pin prototype references to `44308da`. |
| L3 / Low: historical status locator says "above" incorrectly | ACCEPTED. Correct the locator to the earlier plan/navigation statuses; no prior finding or disposition was removed. The earlier six-document scope record describes `b3b706e`, before the CI repair below. |
| L4 / Low: stage 2a exit omits named requirements | ACCEPTED. Include PTY/ConPTY/psmux input proof and verified unreleased-branch artifacts without a target compiler in the exit check. |

Exact-head hosted verification then exposed a separate blocker in the unchanged
Starship performance test: [Ubuntu job 113233217715](https://github.com/luisgui1757/dotfiles/actions/runs/37753789853/job/113233217715)
installed Hyperfine 2.0.0 and crashed with `KeyError: 'mean'`. The earlier zsh
concurrency test passed on this head; its prior failure's cause was still unknown
at that review point (see the subsequent diagnosis below).
No assertion was disabled or waived.

The same Starship crash reproduced locally through the original performance
entrypoint using the real official Hyperfine 2.0.0 Apple Silicon artifact,
verified against its published SHA-256. A new process-boundary regression also
failed against the original parser on its schema-2 case. The repair reads legacy
`results[0].mean` or schema-2 `summary.time_wall_clock.mean` with seconds units;
unknown/malformed/nonfinite results fail closed. It preserves the existing
integer-millisecond comparison and 80ms local / 150ms CI thresholds.

After the repair, all 17 behavioral cases passed, including legacy shape,
schema 2, both budgets, CPU versus wall-clock distinction, invalid inputs and a
quoted temporary path. The real gate passed with local Hyperfine 1.20.0 (4ms)
and isolated 2.0.0 (6ms). Timing samples prove those runs only, not a universal
performance result. The repair is test infrastructure required to verify this
delivery; that eight-file iteration did not change public installer behavior.
At `b97b67f`, the delta included
the six design/navigation documents plus the performance test and its regression.
The full final gate and complete independent review remain required before a
successful handoff; exact-head hosted outcomes are recorded in the PR.


## Complete review and Ubuntu lock diagnosis (2026-10-08)

The complete eight-file review at `b97b67f` returned exit 0/success as
`claude-opus-5-5`, with 38 Read/Glob/Grep calls, no permission denials and no
out-of-scope accesses. xhigh was requested; effective effort was not exposed.
There were no Medium or higher findings. L3-prime (Low) identified ambiguous
references to superseded PR iterations. ACCEPTED: label the six-document scope
as historical at `b3b706e`, state that earlier statuses are absent from the sole
delivered commit, and call stage 0 a plan delivery. These corrections preserve
the prior assessments rather than asserting that they reviewed the later code.

The next exact-head [Ubuntu job 113239574694](https://github.com/luisgui1757/dotfiles/actions/runs/37755702070/job/113239574694)
failed the concurrent publisher test again. The original test reproduced in an
isolated Ubuntu 26.04 container with uutils `mkdir` 0.10.0. A syscall trace proved
two overlapping lock creations: one `mkdirat` returned 0 and the other returned
`EEXIST`, yet both `mkdir` processes exited 0. Both publishers therefore entered
the protected section; the second rename nested its staging checkout inside the
first published checkout and both final proofs failed. An ordinary sequential
attempt on an existing directory correctly returned 1, so a sequential probe
alone would miss this race. Git or final-proof flakiness was rejected as the
cause: the false-success lock acquisition preceded both publications.

Ubuntu also supplies `/usr/bin/gnumkdir` (GNU coreutils 9.7 in this container).
At `dc3c7b0`, the publisher used that companion when available, otherwise the
normal platform `mkdir`, only for lock acquisition (the missing-provider case
is superseded by the correction below). It retains the existing directory
lock, retry, cleanup, exact-commit proof and check-only behavior; it neither
replaces system utilities nor adds a package/runtime dependency. The repository
search found no other shell lock acquired via `mkdir`; the Pi lock is acquired
through Node's filesystem API. Ubuntu's [coreutils transition record](https://discourse.ubuntu.com/t/an-update-on-rust-coreutils/80773)
provides distribution context; the exact failure and companion availability
above were measured in the container, not inferred from that announcement.

A process-boundary regression models the false-success command and keeps a
live owner lock held until the competitor demonstrably waits. It fails the
original publisher (`competing publisher took over a live lock`) and passes the
repair, including successful publication after release. The existing concurrent
first-start test remains intact. The repaired complete publisher test passed
on macOS, 20 consecutive Ubuntu runs, and a separate non-root Ubuntu run. The
final eleven-file delta adds this production correctness repair, its regression
and the matching chezmoi publisher hash trigger to the prior eight files. The
pin-consistency gate caught the initially stale trigger; it was recomputed from
the helper bytes before repeating the full gate. This is an existing zsh provisioning fix, not an implementation of
the proposed selective installer. The final full gate, fresh complete Opus
review and exact-head hosted checks remain required; their final results belong
to the PR delivery record.


## Missing lock provider correction (2026-10-08)

The complete eleven-file review at `dc3c7b0` finished successfully as
`claude-opus-5-5`: 43 Read/Glob/Grep calls, no denials or scope violations;
xhigh requested, effective effort unexposed. It confirmed the prior corrections
and the fail-before/pass-after evidence. L1 (Low) noted that detecting no GNU
companion would still select an affected uutils `mkdir`. ACCEPTED despite its
non-blocking rating: supported-image packaging is not a sufficient runtime
contract for a user-modified PATH or minimal installation.

The helper now detects uutils when the GNU companion is absent and fails with
an explicit requirement before lock acquisition or publication. An unsupported
BSD `--version` probe is expected and leaves the normal BSD command selected.
The new minimal-PATH regression fails at `dc3c7b0` and passes the correction,
asserting the explicit error and absence of target/lock state. The existing
live-owner and concurrent-publisher tests continue to exercise successful use.
The helper's hash trigger is recomputed with the same change.

The suggested alternative of adding a noclobber PID-file claim was not adopted:
combining a new file-ownership protocol with the existing directory cleanup and
stale-lock recovery would need its own concurrency proof. Explicit rejection of
the known unsafe provider preserves the established protocol. No global utility
replacement, implicit package installation or weaker integrity check is added.
The next review receives environment metadata with the Ubuntu test logs as well
as the complete updated delta; final hosted checks are still required.


## Review of the missing-provider correction (2026-10-08)

The full eleven-file review at `fb0ddab` returned no unresolved findings at any
severity. Execution finished exit 0/success as `claude-opus-5-5`, with 43
Read/Glob/Grep calls, no permission denials and no scope violations. xhigh was
requested; effective server-side effort remained unexposed. The reviewer
confirmed L1 and L3-prime resolved, the unchanged benchmark budgets, the
publisher/hash consistency, and the complete local gate and Ubuntu evidence.
Hosted checks were still pending when that snapshot was provided; their final
outcomes and the review of this ledger addition belong to the PR record.

One separate pre-existing candidate remains UNVERIFIED: stale-lock recovery in
`scripts/ensure-pinned-zsh-plugin.sh` (baseline `ae9a644`, lines 154-158) reads a
dead PID and later renames the directory without binding that rename to the
observed owner. Two waiters could observe the same dead owner; after the first
reclaims and reacquires, the second could move the new live lock aside. The
reviewer supplied a source trace, not a runtime reproduction. The branch is
byte-identical to the baseline and is outside the default-mkdir regression
repaired here. Stage 1's existing-runtime inventory must reproduce or reject
this case before claiming stale-lock recovery is concurrency-safe. A correction
would require its own regression and lock-lifecycle proof; neither the current
first-start stress tests nor this review establishes that proof.

Two optional nits were not defects in this delta. The older lock-provider
paragraph is now explicitly historical. The GNU-package advice is appropriate
for the reproduced Ubuntu environment; a manually installed uutils on another
OS may need its normal GNU/BSD mkdir restored on PATH instead. The helper fails
safely in that unsupported command arrangement; no broader package-manager or
lock-protocol change is made here.


## Recovered implementation history from PR #87

The following entries are preserved from the experimental implementation.
Statements about separate PRs describe superseded delivery history; the active
delivery now consolidates the plan and implementation into PR #87.

## Implementation started (2026-10-08)

The owner authorized the complete implementation in one commit and a separate
PR, with Opus 5.5 xhigh CLI review. The stage-by-stage PR recommendation is
superseded by that delivery instruction; the acceptance requirements remain.
An isolated implementation branch starts from the audited main revision and
includes the consolidated design. Baseline `make ci` passed on macOS; local
PowerShell coverage was unavailable. Linux arm64 Docker is available; native
Windows/WSL and clean macOS infrastructure assignment remains pending.

H5 source inspection now confirms that the pinned Sentinel commit's
`tools/install` accepts `--global --remove` (and `--uninstall`) and preserves
surrounding user text. Its removal implementation and native PowerShell parity
still require lifecycle tests before H5's runtime acceptance is closed.

The initial catalog and pure planner are implementation work in progress, not a
released selective installer. They separate capabilities, prerequisites,
ownership and removal policy; adapters and installed-entrypoint proof remain
required before completion.

## Implementation-core review and dispositions (2026-10-08)

Independent read-only review used the local Claude CLI with
`--model claude-opus-5-5 --effort xhigh`, safe/restricted mode and only
Read/Glob/Grep tools. Runtime initialization and final model usage both named
`claude-opus-5-5`; the CLI accepted xhigh. The reviewer did not run tests.
The working tree changed during that first review, so its findings bind the
read versions, not a final immutable commit. A frozen-tree follow-up is required.

Verdict: **NOT READY**. The reviewer identified missing implementation and
clean native evidence independently of the core defects below. A draft is a
reviewable work artifact, not delivery of the overhaul.

| Finding | Disposition | Resolution / remaining boundary |
|---|---|---|
| E1: no-op removal can discard ownership | FIXED in core | Re-probe after removal and again at transaction end; require absence or the recorded baseline. Preserve the receipt/journal on failure. Regression uses a provider that returns success without deleting. |
| E2: deferred/reinstalled resources remain reused | FIXED in core | Ownership is acquired from the immediate absent observation, not receipt existence. Deferred absent resources do not acquire reused receipts. Resume/removal and externally removed/reinstalled cases assert ownership. |
| E3: maintenance can alter selections/Keep | FIXED in core | Maintenance derives roots from state, rejects differing choices and removal requests, and cannot perform cleanup. Apply inherits Keep unless explicitly cleared. |
| E4: update/repair overwrite drift or external resources | FIXED conservatively in core; adoption OPEN | Compare recorded artifact/provider identity before repair/update. Changed or unowned unhealthy resources become needs-action. This does not implement the missing adoption workflow. Operations expose ownership and privilege requirements. |
| E5: prerequisite repair invalidates downstream approval | FIXED in core | Separate own-artifact evidence from health; permit health-only changes and skip a repair made unnecessary by a prerequisite. Boundary-driver regression models dependent health. Provider/content/consumer drift still rejects execution. |
| E6: WSL/GUI boundary absent | FIXED basic policy; full provider modelling OPEN | Gate GUI roots in WSL, refuse host mutation from guest, and reject changed persisted context. Full clipboard/font provider variants and GUI-context migration remain unfinished. |
| E7: unsupported architectures accepted | FIXED prototype validation | Darwin requires arm64; Windows ARM remains rejected as unaudited. Linux accepts amd64/arm64. Public support is unchanged and the full distro/provider inventory remains open. |
| E8: retry erases failure journal | FIXED core history | Archive the previous transaction by content digest before publishing a new journal; keep pending outcomes separate from immutable approval. Recovery locations are explicitly prospective, not claimed backups. |
| E9: scoped baselines and privileged adapters absent | OPEN | No production adapters, scoped key schema or elevation UI exist. Observation hashes/recovery references do not provide these contracts. |
| E10: external vs managed consumer ambiguity | CONTRACT CLARIFIED; adapters OPEN | Consumers means external or uncertain users; graph-owned dependencies use reachability. Actual npm/provider inventories remain unimplemented. |
| E11: execution mutates approved plan | FIXED in core | Execution no longer rewrites the shared operation slice; pending outcomes live separately. |
| E12: stale apply creates lock infrastructure | ACCEPTED WITH BOUNDARY | Discovery/preview stays read-only. An explicitly requested apply acquires a mutation lock before reobservation, so a rejected stale apply can leave only an empty state directory/lock file; no ledger or provider mutation occurs. |
| E12: Windows reader blocks publication; newer-schema diagnostics | FIXED source; native test PENDING | Read through Go OpenRoot delete-sharing and test replacement with an open reader. Inspect schema before strict decoding to report a newer schema accurately. Native Windows execution remains required. |
| E13: go.mod tab convention missing | FIXED | Add the go.mod/go.work EditorConfig override. No existing checker is weakened. |
| Test/documentation drift during review | CORRECTED | Add both shared-removal orders, cross-process lock death, update/repair/Keep/WSL/drift/recovery regressions; synchronize status docs and contributor Go requirements. |
| Go maintenance/security/required checks | PARTIAL | Renovate gomod and native core-test jobs added. CodeQL Go coverage, protected required contexts and full release integration remain stage gates before production adoption. |

The graph property tests are retained because they guard deterministic
reachability and prerequisite ordering; they do not prove that the authored
manifest exhaustively captures real application dependencies. This distinction
is explicit in installer/README.md and the status inventory.

The original requested delivery remains one commit and one PR. Stage 1 access
and stage 2a feasibility are unresolved. The core is explicitly experimental;
no platform adapter should rely on it as an accepted production contract yet.

### Frozen-tree Opus follow-up and first CI results

The second read-only Opus CLI pass reviewed staged tree
`31edf37fba609c639c24960e1ef219bb7593ce3b`, subsequently committed as
`44308da9a9d62afca101a0366fb7b229ce97da75`. The caller verified the unchanged
tree before committing; Opus had only Read/Glob/Grep and could not independently
run Git. Invocation and returned runtime model were again `claude-opus-5-5`
with the CLI effort set to xhigh.

Verdicts: suitable **conditionally as an experimental draft**, no high-severity
core defect found; **NOT READY for delivery**. Prior fixes E1-E8 and E11-E13 were
confirmed at their stated boundary. E9/E10 remain open.

| Finding | Disposition | Evidence / follow-up |
|---|---|---|
| N1: pending completion loses ownership after absent/changed artifact | OPEN; unsafe promotion prevented | An absent pending outcome now stays uncertain, with a regression through later appearance and removal. Recovering ownership requires provider-bound completion proof. Merely accepting changed bytes at the same provider/path would risk adopting outside edits, so that proposed shortcut is rejected. Full pending-completion lifecycle remains unimplemented. |
| N2: Windows checkout breaks gofmt | FIXED; native rerun pending | [Core run 37736919580](https://github.com/luisgui1757/dotfiles/actions/runs/37736919580) passed macOS/Ubuntu but Windows stopped before tests. A fresh `core.autocrlf=true` checkout reproduced 296 CRLF lines in planner.go and gofmt rejection. Added scoped LF attributes, useful formatting diagnostics and a real Git checkout regression. |
| N3: explicit empty Keep disappears in JSON | FIXED | Remove omitempty; nil inherits and an explicit empty array clears. Regression exercises marshal/decode/planning for both forms. |
| N4: probes need explicit receipt context for consumer classification | OPEN | Settle the production driver interface before implementing npm or other shared-provider adapters; no production driver exists. |
| N5: explicit retry/abandon intent absent | OPEN | Current fresh approval can choose a new plan after archiving a failed one. The future recovery interface must distinguish retry from intentional abandonment. |
| Remaining durability/context/coverage gaps | OPEN | Parent-directory durability on first creation, context migrations, provider fault injection and several final-verification branches need further proof before production acceptance. |

Local full `make ci` passed on that tree, including Go race tests and official
Renovate extraction of 89 records. The staged diff passed Gitleaks. The core
test executable also passed as a non-root user in a network-disabled Ubuntu
26.04 arm64 container with read-only source. None is public-installer proof.

The existing Ubuntu shell suite separately failed its first concurrent zsh
publisher in [test run 37736919478](https://github.com/luisgui1757/dotfiles/actions/runs/37736919478).
The publisher and its test are unchanged from main. The same test passed in a
non-root Linux container and both local full gates; a same-head CI retry was
requested. The original failure is retained as an unresolved observation, not
hidden by a test change or a claim that its cause is known.

### Final core-delta review

A third read-only CLI pass again ran `claude-opus-5-5` with xhigh. It inspected
the exact staged N1/N2/N3 delta and surrounding code, found no new material
correctness issue, and confirmed each narrow correction. The reviewer retained
N1 reacquisition, N4/N5 and all missing delivery layers as open. Verdict remains
**suitable for a draft, not delivery**.

The complete local gate passed after these changes. The review also suggested
isolating inherited Git variables in the checkout fixture and allowing future
binary fixtures through text=auto; both were adopted, with an external-index
sentinel assertion. The Makefile now propagates gofmt failures and prints the
files needing formatting, matching the workflow's improved diagnostics.

The Ubuntu shell-concurrency failure repeated on CI attempt 2. A minimal native
Ubuntu 26.04 container also reproduced final-publication proof failures in both
concurrent processes, whereas Debian and a traced Ubuntu run passed. This is a
real unresolved timing-sensitive failure, not an accepted flaky-test waiver.
Diagnostic reproduction is ongoing; no assertions have been disabled.


## Single-PR reconciliation and recovery corrections (2026-10-08)

- Delivery: PR #87 incorporates the full reviewed #86 plan and its Hyperfine and
  GNU mkdir fixes, plus every staged/unstaged prototype correction. The original
  dirty checkout was preserved. #86 is superseded after publication to #87.
- The unchanged main baseline passed `make ci`. The combined core passed
  `make test-installer` (gofmt, vet, race tests) before these new corrections.
- **N4 resolved in code:** probes receive their recorded receipt and recovery
  reference; the real-file driver regression checks preview and execution.
- **N5 resolved in code:** unfinished work rejects implicit resume. Retry binds
  the original mode, selections and removal choices; abandonment preserves
  artifacts/receipts and archives the journal without provider mutations.
- **R1 confirmed and fixed in code:** removal accepted absence even when its
  receipt recorded an original user baseline. The real-file regression failed
  against the recovered prototype, returning `ready` after deleting that file.
  Immediate and final removal checks now require the exact original baseline.
- The implicit-resume regression also failed against the recovered prototype.
  Both fail-before transcripts were retained for independent review. Final
  native verification and review are still required; this entry is not approval.
- The user selected GitHub-hosted runners. Separate paid/disposable VM provision
  is no longer an execution prerequisite. A bounded real WSL2 host probe was
  added; its runtime result remains pending. Existing preloads, GUI/reboot and
  privilege limitations must be reported rather than hidden by fixture tests.

- Consolidated local gate: the first run reached the Renovate extraction check
  with new source files still untracked, so its Git-backed extraction omitted
  them. After staging the recovered files, it identified the additional native
  Windows runner dependency introduced by the WSL2 job. The reviewed inventory
  now includes that record; neither check was weakened.

- **W1 native reproduction:** `core (windows-2025)` in run `37769362559` failed
  `TestStateReplacementWhilePreviewReaderIsOpen` at `7231a4a` with Access denied.
  Go's `os.Root.Open` does share deletion, but Win32 `MoveFileEx` still cannot
  replace this open destination. The correction uses `os.Root.Rename`, whose
  pinned Go implementation requests `FILE_RENAME_POSIX_SEMANTICS`, followed by a
  flush of the published file. The regression additionally checks that the old
  reader retains complete old bytes. Native passing evidence remains pending.
- WSL2 availability is now proved by the same run: actual Ubuntu 24.04 startup,
  Microsoft-standard kernel, WSL interoperation registration and Windows mount.
  This closes the host-access question, not the future guest installer matrix.
- Go was added to existing GitHub default CodeQL languages, retaining Actions,
  Python, query suite and threat model. Configuration validation run
  `37769571342` passed; final-implementation Go analysis is still required.
- #86 was closed after its five code/test changes were verified byte-identical
  in #87 and the entire prior ledger was verified as a preserved prefix. #87 is
  the sole open PR; `7231a4a` is one commit above main.


## Focused provider-transaction consultation (2026-10-08)

A fresh read-only Claude CLI consultation completed successfully with actual
model `claude-opus-5-5`, requested effort `xhigh` (effective server-side effort
unexposed), 36 Read/Glob/Grep calls and no denials or scope violations. It read a
frozen source snapshot; this is architecture input, not final implementation
approval. The complete response is retained outside Git with the execution record.

Accepted: retain per-package decisions, add explicit provider bindings and full
provider inventories, split Nix activation into grow/shrink around consumer
operations, and require pre-built-generation evidence for recovery. Native
manager dependency ownership requires transaction-bound evidence, not inventory
differences alone. Unknown legacy/profile inputs block activation; preservation
checks must cover the entire prior inventory, including unselected members.
These constraints are incorporated into the plan's provider-transaction section.

Rejected as the primary model: one opaque profile resource with read-only package
aliases loses per-package Keep/removal decisions and dependency ordering. A
single mixed add/remove batch can conflict with consumer cleanup order. Running
one macOS rebuild per package adds repeated privileged activation and bundle
runs; two ordered activations preserve the same decisions with less repetition.
Native performance and upstream ordering remain unverified until adapter tests.

At `7231a4a`, all existing platform/e2e/Nix checks passed. The new Windows core
job alone failed on W1; the corrected code passed the full local `make ci` gate
before the next native run. No implementation completion is claimed by these
core and legacy-workflow results.

## Native terminal component and corrected core evidence (2026-10-08)

At `2009eff`, all 26 checks passed, including the corrected native Windows
open-reader regression, Go CodeQL, real WSL2 and the existing installer/Nix
workflows. Those results cover the core and existing public installer, not the
unimplemented selective lifecycle. PR #87 remains the sole open PR.

The selector now uses native keyboard input with no background stdin reader.
Real macOS PTY tests passed selection, all/none, cancellation, deadline, resize
and subsequent cooked input with exact terminal-state restoration. Windows
ConPTY and Linux PTY implementations are included for hosted verification.
`golang.org/x/term v0.46.0` supplies terminal-state handling; Go module checksums
and the Renovate inventory track it. No new runtime prerequisite is imposed on
users by this isolated component.

Rejected restoration alarm: the first macOS test compared termios immediately
after restoring ICANON and observed only PENDIN (0x20000000). Apple's termios
header identifies this as pending-input state; the kernel sets it on canonical
mode restoration. The test now reads a real subsequent line, which processes
that state, and compares all saved settings without masking fields. It also
proves a selector reader does not steal the next prompt's input. Production
restoration was unchanged. This is a fixture correction, not a skipped check.

## Completion ownership and hosted terminal results (2026-10-08)

- **R2 reproduced and corrected:** the old core returned ready and acquired
  removal ownership from artifact appearance without provider transaction proof.
  `TestArtifactAppearanceAloneCannotAcquireRemovalOwnership` failed against
  `e07f56c`; the corrected core requires the persisted operation identity to
  match provider-verified completion evidence.
- **N1 core recovery implemented:** the reboot-completion regression failed
  against that same baseline because pending mutation intent had no durable
  operation identity. Explicit retry now recovers proved creation and permits
  subsequent safe removal. Wrong operation, content, provider or identity still
  prevents takeover. Legacy journals missing mode/operation fields remain
  readable and can be explicitly abandoned without mutation or lost receipts.
  Native provider implementations and their evidence remain outstanding.
- Native macOS and Linux terminal/core checks passed at `e07f56c`. Windows
  ConPTY attached the child, but inherited the CI parent's redirected handles,
  so GetConsoleMode failed before the selector opened. The harness now follows
  Microsoft's node-pty startup contract: STARTF_USESTDHANDLES with null handles
  permits ConPTY to initialize them. This corrects process creation, not the
  selector's redirected-input rejection. Native rerun remains required.

## Flat projection boundary and native activation fixture (2026-10-08)

At `4573294`, native core and terminal tests passed on all three hosted runners,
including Windows ConPTY. Go CodeQL and real WSL2 also passed. This resolves the
ConPTY harness rerun noted above without claiming selective lifecycle delivery.

Explicit manifest bindings now feed the existing locked Nix modules. Linux
clangd and retained Darwin Homebrew infrastructure are graph prerequisites;
Nix receives flat resolved package IDs rather than a second feature graph.
Strict projection validation covers target identity, operation, unknown fields,
duplicates and platform/package mismatches. Empty/full sets and every singleton
passed evaluation across aarch64-darwin, aarch64-linux and x86_64-linux.

A Node-only aarch64-darwin generation built successfully with immutable metadata
for its projection, exact Node output and Home Manager generation. Nothing was
activated on the user's Mac. A hosted-only fixture now exercises real package
activation, grow/shrink, update, empty selection, reinstall and an unrelated
profile input. Its native result is pending. Full observed-inventory preservation,
transaction coordination and legacy adoption remain implementation obligations.

Rejected approach: using presence of an `infra.nix` dependency to choose the
provider would incorrectly assign native tools to Nix. Provider bindings are
separate data and validated against their actual infrastructure requirements.

The first local projection gate stopped at ShellCheck's unresolved literal
`source ./setup.sh` in the native fixture. The lint runner now follows that
source with `--external-sources`, as for the existing uninstall fixture; no
diagnostic is suppressed. The native fixture explicitly exits the source-only
seam's inert-path mode after the disposable-host guard so it exercises actual
macOS rc/tap migration. A local regression proves that ordinary hosts are
rejected before Nix or the setup helpers run.

Adding malformed-type cases exposed an error in the test helper: it inspected
`selection.system` before invoking the public validator for a null request.
The rejection test now forces the public validation boundary first; it still
requires every invalid request to fail, including null/non-object data. Native
activation also resolves the Nix executable before sudo so it does not depend
on the privileged PATH.

## Stale zsh publisher lock race (2026-10-08)

**Confirmed and fixed locally:** two real publishers can read the same dead
owner, after which the second stale waiter renames the first publisher's newly
acquired live directory lock. A deterministic process-boundary regression failed
against `0db8b87`: both publishers entered the protected body and one later failed
publication. The regression delegates real Git/filesystem operations and controls
only their scheduling; the fixture also strips inherited Git state.

Publication now re-execs the already-required zsh, holds its native fcntl lock on
a permanent file, and uses the in-process `zf_mv` builtin for publication. A
parent wrapper around a child writer was rejected: killing the parent would
release a process-owned lock while the child could still publish. Tests now kill
the actual publisher during a paused child fetch, publish a newer pin, then let
the old fetch finish and prove it cannot overwrite the newer checkout.

The previous uutils regression is retained with the new contract: broken mkdir
semantics and absence of GNU mkdir cannot bypass a native lock. Live-owner
exclusion, automatic release after SIGKILL, concurrent convergence, check-only
inode preservation and explicit legacy-directory refusal pass locally. Existing
stale directory locks need inspected recovery; this change never deletes or
reclaims them automatically. The helper hash trigger is updated with the code.

The [zsh locking implementation](https://github.com/zsh-users/zsh/blob/master/Src/Modules/system.c)
uses process-associated fcntl locks. This is why both lock ownership and final
rename stay in one process. Native Linux and the full gate still need to run for
this correction; the prior Nix-projection `make ci` gate passed separately.

The first hosted Nix provider run (`37778182042`, `0db8b87`) stopped before
activation on both OSes: `builtins.getFlake` requested `revCount` from the shallow
Actions checkout. The fixture now uses a URI-escaped Git file reference with
`shallow=1`; it does not fetch arbitrary history or change the tested source.
Native activation remains unproved until that corrected run passes.

The corrected publisher and shallow-source fixture passed the full local
`make ci` gate. The Nix build-only boundary also resolves from a real depth-1
Git clone whose path contains spaces with `shallow=1`; no host activation ran.
The native shallow failure above is the fail-before evidence; a local evaluation
cache did not reproduce that particular failure. Hosted activation must still
pass on the amended revision before provider lifecycle evidence is claimed.

## Complete Nix members and parity prerequisite (2026-10-08)

`d13bb73` passed the native Linux/macOS Nix lifecycle fixture in run
`37779745271`: six actual generation activations per OS, direct binary/version
consumption, absence after removal and preservation of the unrelated hello input.
This closes the shallow-checkout rerun for that provider fixture, not controller
acceptance. The native core/terminal jobs passed as well.

The Linux chezmoi parity job failed before plugin publication because it did not
install zsh. The full installer already installs the selected shell before its
plugins; macOS has the shell available. The parity job now installs the real zsh
prerequisite rather than bypassing publication or substituting a command stub.

Read-only inspection of a real existing Home Manager derivation exposed jq's
separate bin/man outputs and duplicate manual-output references. Inventory now
reads the complete ordered buildEnv groups, preserves priorities and deduplicates
only repeated output paths. Versioned Nix schema 4 and older env.__json shapes
are covered; unknown shapes and deriver/output mismatches fail closed. Generation
metadata reads `home.path.chosenOutputs` directly, avoiding a second implementation
of nixpkgs output selection. Resource executable paths use `lib.getBin` rather
than assuming the default output contains binaries.

A real oversized child-output regression caught an early bounded-writer defect:
embedding bytes.Buffer promoted ReadFrom, letting io.Copy bypass Write's limit.
The buffer is now a private field; a real producer exceeding 8 MiB is cancelled
and the test passes. Provider queries do not print derivation environments. Go
Nix command adapters are excluded from Windows builds and included in the remote
installer URL guard. Actual metadata/reader parity and multi-output consumption
are added to the next native provider run; this is not yet that passing evidence.

### Complete profile inventory compatibility (2026-10-08)

The read-only Go reader matched all five complete input groups of a real
Node+jq Home Manager generation built locally without activation. This is
build/inspection evidence, not a local install/removal run. jq's binary output
and manual output remain together with their buildEnv priority.

Source inspection found that the initial parser rejected existing Nix 2.32
installations and accepted an unknown entry version inside a version-4 envelope.
Focused regressions failed for both cases before the correction and passed after.
The parser now distinguishes the legacy absolute-path map, the 2.32 basename map
with version-3 entries, and the 2.33+ version-4 envelope and entries. Sources:
[Nix 2.32 derivation-show](https://github.com/NixOS/nix/blob/2.32.4/src/nix/derivation-show.cc),
[2.32 serialization](https://github.com/NixOS/nix/blob/2.32.4/src/libstore/derivations.cc),
and [2.33 derivation-show](https://github.com/NixOS/nix/blob/2.33.1/src/nix/derivation-show.cc).
Unknown or mismatched versions fail closed. The native reader fixture is now part
of each hosted package-plane transition, including actual jq consumption.

Go command arguments also receive an AST-based architecture check for the existing
ban on blanket Nix upgrades, lock rewrites and mutable registry bootstrap aliases.
Multiline calls and argv slices are covered; ordinary read-only and locked build
commands remain allowed. This is a literal-command guard, not an execution sandbox.

The first full inventory-stage gate rejected the Nix job's `go-version-file`
because the pinned Renovate extractor reported an unspecified Go version. The job
now follows the existing explicit `1.27.1` convention, and the reviewed extraction
inventory includes its Go runtime and setup action. No extraction rule was relaxed.

Verification: the corrected inventory stage passed the full local `make ci`
gate, including official Renovate extraction and Go race tests. Hosted reader
and corrected Linux parity results remain pending on the next revision.

### Preserve exact Nix members across selection changes (2026-10-08)

Problem: rebuilding all retained IDs from a changed lock updates packages during
an unrelated grow/shrink. Alternatives were re-resolution, synthetic derivation
wrappers, or preserving the actual buildEnv inputs. The implementation uses
complete prior output groups and priorities, applied through Home Manager's existing
`home.path` option transform. This keeps the upstream builder and avoids duplicating
its multi-output selection rules. The projection validates resource binding,
canonical store objects, binary membership, unique package claims and integer
priorities. Unbound vendor resources cannot enter the Nix preservation path.

A real local build retained all five baseline inputs of a Node+jq generation and
added only ripgrep. The independent Go reader matched its six inputs; no activation
ran on this Mac. Pure invalid-input fixtures and the repository's Nix architecture
and source-following ShellCheck checks passed. The native fixture adds a real jq
priority-17 baseline and asserts exact preservation through grow/shrink; its explicit
update intentionally selects the current locked default. Hosted proof is pending.

This projection is data, not an ownership claim. The unfinished controller must
prove generation provenance and classify every observed member before constructing
it. Unknown members may not be silently relabeled as infrastructure. Remaining
whole-inventory, adoption and grouped-journal requirements remain open.

Prior revision `6866889`: Linux native Nix inventory/lifecycle and corrected zsh
parity passed, as did native core/terminal checks on all three OSes. macOS Nix
activation was still running at this point; no pending result is counted as a pass.

The preservation stage passed the full local `make ci` gate. The preceding
`6866889` revision also finished native macOS Nix activation successfully; both
POSIX jobs passed the real inventory reader on every package transition. The new
priority-preservation activation case remains pending its hosted revision.

### Executor preservation regression (2026-10-08)

Two real-file engine regressions reproduced false readiness in the prior executor:
plugin removal replaced an unselected retained Node artifact, and plugin install
replaced the Node artifact installed and verified earlier in the same transaction.
Both returned ready because final verification inspected selected health and explicit
removals, without checking preserved identities. The source and failing output were
captured before the correction. Alternate explanation checked: the replacement was
healthy according to the same normal boundary probe, so this was identity loss,
not merely a missing-health check.

The executor now compares each kept/retained artifact with its approved observation
and each completed mutation with its recorded result. It checks unselected retained
resources as well. Mismatches fail and retain the journal and receipt. Focused tests
cover replacement and deletion with both retained and still-selected Node; the full
Go race suite passed after the fix. This closes the core's final-preservation gap
for planned resources, not the outstanding whole-provider discovery/adoption work.
The Go toolchain parity test now covers the Nix workflow as well as engine CI.

### Explicit grouped execution (2026-10-08)

The planner now collapses Nix mutations into separate grow/shrink graph nodes,
retaining individual operations and ownership receipts. This implements the
consultation's rejection of implicit batching inside one package's `Apply`.
Dependency ordering is revalidated after grouping; impossible intermediate native
dependencies fail before mutation. Consumer cleanup precedes shrink. Missing batch
adapters fail before prerequisite installation.

The executor persists all member operation identities in one state publication,
then calls the batch boundary. It validates the complete returned member set and
provider-bound evidence before recording completed ownership. Interrupted batches
retain individual intent; proved completion on retry does not repeat the grow.
State loading rejects partial or mismatched member intent without altering the
journal; legacy journals without the optional field remain readable.

Real-file tests pass for one grow/one shrink, mixed add/remove selection changes,
cleanup ordering, pre-existing member preservation, incomplete completion evidence,
interruption recovery, unavailable adapters, malformed intent and collapsed cycles.
The Go race suite passed. Native package managers remain single-resource operations.
These tests do not stand in for the still-missing production batch adapter,
whole-provider discovery, adoption or provider transaction proof.

Revision `f4b837f` passed all 26 hosted checks, including both native seven-step Nix
preservation runs with the non-default jq priority and an unrelated profile input.
The new grouped executor and collateral-change fix require their own final gate
and native core runs before being counted as verified on all operating systems.

The grouped executor and collateral-change fix passed the full local `make ci`
gate. A subsequent recovery check reproduced another gap: an explicit retry accepted
a different source revision when its mode and selections matched. The focused test
failed before the fix and the Go race suite passed after it. Retry now requires the
original non-empty source identity; explicit abandonment from a newer revision is
still allowed and preserves the old evidence. Authenticating that identity remains
the launcher/provider responsibility. The full gate is rerun after this correction.

The final batch/recovery state passed the full local `make ci` gate after the
source-binding correction. Hosted native core results remain pending for this
revision; the production adapter and complete installer remain unfinished.

### Native Windows workflow-byte correction (2026-10-08)

Revision `3edc9b1` passed native Linux/macOS core tests and WSL2 startup. Windows
reported one failing test: Go-version parity for `nix.yml`. The checkout attributes
forced LF for the engine workflow but omitted the Nix workflow, so Windows checked
out its version line with CRLF. A real local Git checkout with `core.autocrlf=true`
reproduced changed `nix.yml` bytes before the fix. The workflow attribute now covers
all workflow YAML; the regression and Go race suite pass after the correction.
The version comparison was not weakened or normalized. Corrected hosted and full
local gate results remain pending for this revision.

The workflow-LF correction passed the complete local `make ci` gate. The
corrected native Windows rerun is required before recording an all-platform pass.

### Complete discovery boundary and pre-batch drift (2026-10-08)

Revision `79db7cb` passed all four hosted workflows, including native Windows
core/terminal checks, both POSIX Nix fixtures and real WSL2. This resolves the
workflow checkout failure without weakening the version comparison.

The driver contract now discovers complete provider inventory before planning.
Known unselected members without receipts receive retained/reused decisions;
unmapped members block planning. Approval includes a provider fingerprint that
is separate from each resource's artifact identity. Full inventory is checked
again at completion. Real-file tests cover retained unrecorded packages, stale
approval from unrelated drift, unmapped members, and collateral changes outside
the selected artifact identities.

A prerequisite mutation reproduced another race in the new boundary: it changed
protected provider input after approval but before the batch, and the adapter
then recorded that change as its own target. The test failed before adding the
pre-batch full-inventory recheck and passes after. This closes that engine window;
it does not replace adapter publication locking/revalidation against native writers.

A separate regression, reconstructed against `79db7cb`, showed that unchanged
pending state on an unselected retained resource falsely failed preservation.
The correction compares degradation/new pending state instead of requiring every
retained artifact to become ready. Identity comparison remains strict. Both
failing-before outputs were captured; the full Go race suite passes afterward.
The full local gate is next; production discovery/adoption remains unfinished.

The complete-discovery boundary passed the full local `make ci` gate. It is
recorded in local consolidation revision `e4acc8c`; native execution of the new
boundary remains pending the next published revision.

### Real Nix batch adapter (2026-10-08, verification in progress)

The first real package adapter now connects engine batches to exact-revision Nix
builds and Home Manager/nix-darwin activation. It records the complete baseline
and built target, pins the target against garbage collection, and verifies the
actual generation/profile inventory before publishing per-member completion.
Projection changes preserve complete non-mutating output groups and priorities;
updates refresh only approved packages. Foreign default-profile entries retain
all their metadata. Unknown generation provenance and unproved Home Manager
inputs block activation, rather than acquiring ownership by naming convention.
Darwin metadata now includes its exact per-user wrapper profile.

Primary source inspection confirmed that Home Manager replaces its package member
through separate native removal/installation calls, and Nix locks each profile
publication rather than the whole Home Manager activation. Therefore a preflight
comparison alone is not a complete concurrency/recovery solution. Those native
publication, orphan-worker and intermediate-state cases remain explicit open work;
the adapter is not yet exposed through public setup. Upstream sources inspected:
[locked Home Manager activation](https://github.com/nix-community/home-manager/blob/63d02d1c19f1c2b47a5bc7e55c620b6af7b218a6/modules/home-environment.nix),
[Nix profile locking](https://github.com/NixOS/nix/blob/2.33.1/src/libstore/profiles.cc),
[normalized profile inventory](https://github.com/NixOS/nix/blob/2.33.1/src/nix/profile.cc).

Connecting the adapter reproduced an approval defect: with no selected package
operations, inventory hashes were omitted from the plan. The focused regression
failed before adding the optional complete-inventory identity list to the plan.
This field participates in approval even for empty/no-op selections, while old
schema-1 journals remain readable. The post-fix Go race suite passes. Native
engine lifecycle and the complete new local gate remain pending.

The Nix adapter stage passed the complete local `make ci` gate. The first gate
stopped because the new journal-upload step added a second pinned upload-artifact
occurrence; the reviewed extraction count was updated to match the new workflow
and the full rerun passed all 94 dependency records. Native adapter activation
results remain pending. The journal artifacts intentionally include only JSON,
never GC-root symlinks or their package closures.

### Native engine discovery correction (2026-10-08)

At `e346c66`, the test, engine and existing e2e workflows passed. Both native
Nix engine fixtures stopped before mutation: complete inventory included absent
lazygit, and discovery traversed its unneeded native Git prerequisite. A local
real-file regression reproduced the same error before correction. Discovery now
retains absent members' inventory observations without traversing their unused
prerequisites; selected, recorded and present resources still receive full closure.
This preserves approval coverage without introducing unrelated probes/receipts.
The Go race suite passes after the fix. Corrected full-gate and native results
remain pending. No activation success is claimed for the failed native run.

On 2026-10-09, the unused-prerequisite discovery correction passed the full
local `make ci` gate. A focused Opus 5.5/xhigh consultation is being prepared
for the still-open native publication/recovery protocol; it is not final review
or approval of this incomplete overhaul.

### Native engine evidence and second publication consultation (2026-10-09)

All 26 hosted checks passed at branch `46f774f`. Native Nix run
[37874436568](https://github.com/luisgui1757/dotfiles/actions/runs/37874436568)
passed on Linux and macOS, including the real engine's install, grow, shrink,
update, remove, reinstall and repeat steps plus the independent preservation
fixture. Actual checkout source was synthetic merge
`aded26dbf9c7815fc5d13e2fddc6ee1f00079fb1`. Both downloaded JSON artifact sets
contained generation-7 state with no unfinished transaction, a complete provider
journal and only the expected Node package selection. Bootstrap and full-product
lifecycle remain unproved.

The fresh read-only publication consultation returned successfully after 40
Read/Glob/Grep calls, no permission denials and no guard violations. The actual
response model was `claude-opus-5-5`; `xhigh` was requested but the CLI did not
expose executed effort. This was a focused architectural consultation, not final
independent review of the complete overhaul. Dispositions:

- **Accepted, correction verified in focused tests:** an incomplete target could
  return member completion before checking its original foreign/managed profile
  preservation. A new retry plan then bound the already damaged inventory. A
  real-file/process probe regression failed before correction for changed foreign
  input, managed placement, declaration and user profile. The original provider
  baseline is now checked before granting completion; the focused regression
  passes. Later foreign changes after an already completed transaction remain
  valid inputs to a fresh preview. Full gate/native correction checks are pending.
- **Accepted, open:** normalized `nix profile list --json` drops legacy
  `manifest.nix` metadata, so it cannot prove preservation of priority/active
  fields. The reader must inspect the immutable legacy manifest itself.
- **Accepted, open:** `--no-link` followed by later rooting leaves a GC window;
  use build-time output rooting and retain the baseline too.
- **Accepted direction, implementation open:** keep the established Home Manager
  and nix-darwin owners, add activation-tree lifetime exclusion, durable attempt
  outcomes and exact-target recovery. A second package plane or overriding
  upstream package activation adds ownership/migration problems. The consultant's
  history audit and Darwin process-liveness fallback are proposals, not proved
  exclusion. In particular, process-table checks must not be presented as a
  race-free substitute for a held lock. Native/older writers remain a boundary
  requiring explicit interference tests.

**Correction to the earlier publication statement:** “Nix locks each profile
publication” was too broad. Additional primary-source readback confirms modern
[MixProfile::updateProfile](https://github.com/NixOS/nix/blob/2.33.1/src/libcmd/command.cc#L260)
and [nix-env opSet](https://github.com/NixOS/nix/blob/2.33.1/src/nix/nix-env/nix-env.cc#L724)
call generation/link publication without `lockProfile`. Legacy
[createUserEnv](https://github.com/NixOS/nix/blob/2.33.1/src/nix/nix-env/user-env.cc)
does use a native lock and optimistic token. An external lock around upstream
activation therefore cannot claim universal native-writer exclusion. The pinned
[Darwin activation script](https://github.com/nix-darwin/nix-darwin/blob/d5bd9cd77aea4c0a8f49e7fd85545671a208ed15/modules/system/activation-scripts.nix)
writes `/run/current-system` after its activation steps; the pinned
[Home Manager Darwin module](https://github.com/nix-community/home-manager/blob/63d02d1c19f1c2b47a5bc7e55c620b6af7b218a6/nix-darwin/default.nix)
uses `launchctl asuser` and `sudo -u`, which must be included in native lease
coverage tests. These corrections preserve the earlier ledger history.

The accepted legacy-manifest finding has now been reproduced with real native
Nix in private temporary profiles. On local Nix 2.34.7, changing priority first
made the old pure listing reject the symlinked manifest; this is the actual
failing-before test result. A separate diagnostic normalized listing with
`--impure` demonstrated the underlying loss: priority `17` / active `false`
became `5` / `true`. The production correction does not enable impure evaluation.
It reads the resolved immutable manifest with offline evaluation and
import-from-derivation disabled, retaining nested fields through reversible
attribute encoding. Native legacy priority/active tests and the Go race suite
pass. The hosted fixture now requires legacy and supported JSON-format checks.
The raw output-group, metadata, numeric precision and unproved-member boundaries
have focused tests; complete gate and hosted reruns remain pending.

The first complete gate for this correction failed in the newly enabled native
legacy fixture: Nix explicitly rejects combining `--file` and `pure-eval=true`.
The invocation now uses restricted evaluation with only the resolved manifest in
its explicit Nix search path, an empty allowed-URI list and import-from-derivation
disabled. It overrides the inherited `NIX_PATH`; otherwise restricted evaluator
initialization can resolve unrelated configured flake inputs. A real native
fixture verifies that environment reads are empty and a file outside the allowed
manifest cannot be read even when the caller placed its directory in NIX_PATH.
The preceding “no impure evaluation” wording does not establish pure mode: file
evaluation uses this explicit restricted boundary instead.

Moving modern version-3 profiles to direct manifest reads also requires validation
previously provided by native normalization. A focused failing-before regression
accepted null active state; the reader now rejects null/non-boolean activity and
null/non-integer/out-of-range priorities, while supplying Nix's documented default
priority when absent. The complete gate must be rerun after these corrections.

The corrected retry-preservation and legacy/JSON inventory stage passed the full
local `make ci` gate on 2026-10-09, with the real private-profile fixtures enabled.
This resolves the first gate's incompatible evaluation flags. Hosted verification
of this exact correction and activation supervision/recovery remain pending.

### Build-time roots and native metadata readback (2026-10-09)

Native Nix run
[37878006577](https://github.com/luisgui1757/dotfiles/actions/runs/37878006577)
passed both POSIX jobs at `52e6830`. Both full logs were read back: seven engine
steps passed, as did actual legacy priority/active mutation, JSON versions 1–3,
unknown-field rejection and restricted-manifest evaluation. Core/terminal, WSL2,
parity and Go CodeQL passed too; setup e2e was still pending at this readback.

The consultation's collection-window finding now has a real failing-before
regression. A shell process boundary runs the actual exact-revision Nix build,
then holds its completed JSON before the Go adapter receives it. The original
`--no-link` path had no target root at that barrier. The correction supplies the
operation's `--out-link` during build, verifies it afterward and refuses to
overwrite an existing recovery root. The local native barrier passes after the
change; it checks the link and Nix's own GC-root query. No user profile was
activated. The prior generation is separately rooted before activation, with
hosted lifecycle assertions added for that baseline. Native root retention is
not activation-tree exclusion or partial-state recovery; those remain open.

A follow-up boundary regression also showed legacy inspection accepting an
inactive managed member, unlike the modern reader. It now rejects false or
invalid managed activity; the failing-before and passing-after focused outputs
are recorded. Unrelated inactive foreign packages remain preserved. The current
root/precondition correction still requires the full gate and hosted rerun.

The root/precondition correction passed the full local `make ci` gate, including
the real build barrier and private legacy-profile fixtures. All 26 hosted checks
on the preceding `52e6830` revision are now successful, including setup e2e on
Windows, macOS and Linux. Those results establish the corrected metadata stage;
the new build-time/baseline root assertions require their own hosted run.

### Activation lifetime and abandonment guard (2026-10-09)

All 26 hosted checks passed at `d74ca70`, including the new build-time and
baseline-root assertions. Detached activation supervision is the next stage;
these preceding results do not cover its new code.

A focused real-lock regression reproduced abandonment archiving a transaction
while the provider still held an independent lease. The engine now acquires an
optional provider mutation guard before all observation and journal changes,
including abandonment. That regression passes after correction.

The supervisor uses durable prepared/start/result records, a permanent inherited
lease and detached log output. Darwin performs a single elevation of an
immutable store copy of the worker. It rechecks the original inventory after
authentication. Private directory descriptors contain privileged result writes.
Schema-2 Nix journals require an exact successful attempt before package
completion; completed schema-1 journals remain readable, and incomplete legacy
activation journals require explicit recovery.

The consultation's proposed process-table fallback is rejected as proof of
quiescence: a dead supervisor can leave descendants that closed its lease.
Current-boot unknown outcomes therefore block mutation, including abandonment;
a kernel boot change allows recovery classification without granting ownership.
This is deliberately distinct from complete exact-target retry, which remains
unfinished. Native process and activation tests are in progress.

The original Linux activation path was reconstructed from `d74ca70` in a separate
temporary module and exercised with a real external script. After SIGKILL of its
controller, a second installer acquired the mutation lock while the orphaned
activation still completed a write: the regression failed as expected. This
local POSIX fixture does not activate the Mac's profiles. The new process tests
pass for controller death, hangup/interrupt/termination signals, and a killed
supervisor whose child deliberately closes the inherited lease. That last case
leaves a durable unknown outcome and blocks recovery on the current boot. The
complete native Go race suite also passed. The first focused run correctly
rejected the test framework's public leaf directory; the fixture now creates the
same private provider directory required in production. Native Nix activation
and the full repository gate remain pending for this stage.

The first full local `make ci` gate for supervision passed. Subsequent native
readback caught a Darwin launch defect before publishing this stage: resolving
`nix-env` through its final symlink changes argv[0] to `nix`, selecting the wrong
command parser. A real `--version` regression failed against that resolution.
The worker now retains the immutable provider's `bin/nix-env` alias. Its native
check is required on both hosted POSIX runners, and attempt JSON/logs are included
in the preserved CI artifacts. The gate is being rerun for the corrected state.

The corrected supervisor stage passed the full local `make ci` gate with the
real build-only and private legacy-profile fixtures enabled. This includes the
native `nix-env` entrypoint regression after correction. Hosted installed-worker
activation on Linux/macOS and an independent Opus review of this stage remain
pending; these local results do not complete partial-state recovery or the full
installer acceptance matrix.

### Supervisor native proof and Opus reconciliation (2026-10-09)

All 26 checks passed at `f227761`. Nix run
[37881638025](https://github.com/luisgui1757/dotfiles/actions/runs/37881638025)
executed the installed launcher/supervisor on Linux and macOS against synthetic
merge source `b86a127d05a0ffedca913c5d6f1630fb91e6b646`. Both downloaded artifact
sets contained six successful attempts, generation-7 state with Node selected,
created package/reused infrastructure ownership, and no unfinished transaction.
The seventh repeat step was a no-op. This closes the interim review's missing
normal production-launcher evidence, not its fault-injection gaps or the missing
public lifecycle/adapters. The separate direct projection fixture runs afterwards;
engine journals are evidence of the engine stage, not its machine's final state.

The completed read-only review observed `claude-opus-5-5`; `xhigh` was requested,
but execution effort was not exposed. It inspected the supervisor delta and
related files; the rest of the full task delta was only skimmed. A final complete
overhaul review remains required. Reconciliation, with correction gates pending:

- **F1 accepted:** XNU's `clock_set_calendar_microtime` changes `clock_boottime`
  when calendar time changes. The actual kernel UUID regression fails before
  the fix. Recovery now uses `kern.bootsessionuuid`. Sources: Apple's
  [clock implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/osfmk/kern/clock.c#L675)
  and [read-only boot-session sysctl](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_sysctl.c#L2735).
  Actual clock-step proof is restricted to disposable hosted macOS.
- **F3/F4 accepted and reproduced:** an actual child Bash could not trap signals,
  and an exec'd Go child retained the private attempt directory descriptor.
  The supervisor now handles signals without making children inherit SIG_IGN;
  descriptors are close-on-exec and only the lease is deliberately remapped.
  Tests compare inode/device identities, because `/dev/fd/N` stat semantics and
  runtime descriptor reuse make descriptor-number-only checks unreliable.
- **F6 accepted and reproduced:** a real Linux launcher delayed until after its
  proxy outcome still forked. Launch now checks the closed handoff under the
  lease before forking. **The proposed blanket child check is rejected:** the
  controller normally writes `launch.json` after fork, possibly before the
  child is scheduled; rejecting that child would break a successful handoff.
  The child already inherits exclusion before the launcher can return.
- **F7 accepted and reproduced:** preview during a held engine lock reported
  "restart" for its live handoff. Preview now takes the existing engine lock
  before the activation lease and holds both through observation and outcome
  verification. It creates no state files. Engine/worker observations reuse
  their held lease. Existing lock files must be private, singly linked regular
  files, matching the provider's ownership boundary.
- **F2/F8 accepted; correction pending:** bind the exact worker bytes/revision
  and authoritative account home, and root the worker during store import.
- **F5 partially accepted; recovery design pending:** replaced leases need an
  explicit recoverable protocol. **Success alone does not prove quiescence:**
  a child may still hold the old lease after result publication. Do not permit
  an automatic inode rebind from a successful result. A new kernel session can
  prove old descendants ended; any rebind must be explicit and recorded.

The prior full-gate invocation used `GOTOOLCHAIN=local`, the downloaded Go
1.27.1 directory on PATH, `DOTFILES_TEST_NIX_BUILD=1`, and
`DOTFILES_TEST_NIX_LEGACY_PACKAGE` pointing to the actual temporary hello output.
Its non-verbose output was insufficient for an independent reviewer to prove
those fixtures ran. The next gate will retain invocation/tree metadata and
verbose native fixture output. This is an evidence correction, not an assertion
that plain `go test` output proves fixture execution.

F2/F8 correction implemented: attempt schema 2 now carries the actual immutable
worker path and SHA-256. Both frontend and worker require the command's clean Git
build revision to match the approved source; the running worker also checks its
own executable path, bytes and in-process build record. The hosted native fixture
will exercise rejection of changed revision, digest, executable and account home
through the installed entrypoint. Pure build-record tests cover dirty, missing
and wrong-command metadata. Release artifact authentication remains separate and
unfinished; build metadata alone is not publisher authentication.

The previous account check was extracted unchanged, then an actual local-account
regression failed because an unrelated temporary home was accepted. The corrected
check resolves the account's native home. Native lookup uses Directory Services
on Darwin and getent/NSS on Linux (local passwd fallback only if getent is absent),
so compiler-free builds do not silently exclude directory-backed accounts.

The original `f227761` worker function was reconstructed in a private snapshot.
A real `nix-store --add` process barrier proved its output lacked the later root.
The corrected native barrier passes: a single `nix build --out-link` imports and
copies the exact digest-bound bytes using the locked Nixpkgs builder. No target
compilation or local profile activation occurs. An initial experiment returning
`builtins.path` directly was rejected by `nix build` because it is not a derivation;
the actual copy derivation was then built and its registered root and bytes were
read back. Do not reintroduce that unsupported raw-path expression.

Attempt schema 1 lacks worker identity and is deliberately preserved for explicit
recovery. A legacy-shape regression prevents silently accepting it as schema 2.
This internal, unreleased schema change does not migrate public installer state.
A native clock-step test restores time using monotonic elapsed time and is gated
on disposable elevated GitHub macOS. A second hosted-only fixture executes the
real `launchctl asuser`/`sudo -u` boundary before killing the supervisor. Their
results are still pending; normal lifecycle success is not fault-injection proof.

The first correction gate stopped at EditorConfig: the embedded Bash fixture in
a Go raw string used space indentation despite Go's tab convention. The fixture
indentation is corrected without changing its behavior; the full gate is rerun.
The rerun exposed the same rule in the newly added embedded Nix expression; the
checker stops on its first failing file, so the first result had not covered that
file. Both raw-string scripts now use the surrounding Go indentation convention.
The standalone repository-wide EditorConfig check must pass before another gate.

The corrected full `make ci` gate passed, followed by an explicit verbose run of
both native root barriers, legacy/JSON metadata, restricted evaluation and actual
nix-env command-mode fixtures. Invocation/environment metadata and SHA-256 maps
confirm the tracked tree was unchanged during verification (index tree
`fc982f660d9f8c0c6a582d2272684c5314fed5f4`). The first two gate failures above remain
recorded; no checker was suppressed. The only later changes before publishing
this stage are this evidence/status documentation. Hosted correction results and
independent re-review are still required.

**New accepted candidate, not yet fixed:** a native private-profile reproduction
at `f227761` moved `~/.nix-profile` between two distinct mutable profile bases
containing the same actual legacy package generation. `snapshot` returned the
same approval ID. Nix's `getDefaultProfile` returns the alias's immediate target
as its publication destination, so immutable package identity alone does not
bind the mutation namespace. The fixture touched only temporary profiles. The
fix must bind existing alias destinations/topology, distinguish legitimate initial
profile creation, and preserve old persisted shapes without inventing provenance.
This belongs to the pending exact-target recovery/adoption boundary; do not call
the new provider ready based solely on successful package-consumption tests.

### Default-profile namespace correction (2026-10-09)

The accepted alias finding now has failing-before and passing-after evidence on
real private legacy profiles. Inventory records both default aliases, including
absence and the canonical planned creation destination, and resolves redirected
parent directories while keeping the mutable final component. Distinct mutable
profile bases are no longer deduplicated merely because their immutable contents
match. Existing non-symlink, immutable-store or foreign dangling destinations
fail before activation; read-only observation creates no directories.

Postflight requires the same destination and parent. An initially absent alias
may appear only at that approved canonical destination. Completed generations
require exact recorded alias presence too, so later redirection needs adoption.
Legacy snapshots omit the new field on re-serialization, retaining their original
digest; they cannot supply namespace proof that was never recorded. Persisted
alias records reject missing/null activity, relative paths and unknown fields.
The native regression, boundary/legacy tests and complete Go race suite pass.
The full local gate, hosted lifecycle and independent review remain pending.

The hosted Darwin descriptor fixture now additionally reads the actual child
file descriptors after `launchctl asuser` and `sudo -u`, before running the
blocked descendant. It no longer manually closes fd 3 in that native case;
only the portable simulator does so. This separates evidence of the actual native
boundary from the deliberately modeled descriptor-loss regression.

Hosted supervisor-correction evidence at `490d85d`: Nix run
[37885751733](https://github.com/luisgui1757/dotfiles/actions/runs/37885751733)
passed Linux and macOS. Both ran seven real engine lifecycle steps, checked the
installed worker's exact build/digest/root and rejected changed source, digest,
executable and account home through its real entrypoint. macOS additionally passed
the actual clock-step test (`kern.boottime` changed while the session UUID stayed
fixed) and the first sudo/launchctl descendant-death fixture. The latter still
manually closed the child lease; the stronger actual-descriptor readback above is
new and remains pending.

Both uploaded journal sets were downloaded and read: 32 JSON documents, six
successful schema-2 attempts with matching request digests, one immutable worker
identity per OS, generation-7 Node selection, complete provider state and no
unfinished core transaction. The verified source was synthetic merge
`4d1baac8be8181d1c8f778f361797f9efef98ac0`. These results predate the alias fix and
do not establish that later code's hosted result. Remaining full CI contexts
and the new namespace gate are tracked separately.

The namespace correction passed the full local `make ci` gate and an explicit
verbose native run including the alias reproduction, both build-root barriers,
legacy/JSON metadata, restricted evaluation and nix-env command-mode proof. The
captured source hash maps are unchanged across the gate. All 26 hosted checks
passed at the preceding supervisor revision `490d85d`; the new namespace revision
still needs its own hosted result and independent review. The underlying native
publication rule was checked against Nix 2.33.1's
[`getDefaultProfile`](https://github.com/NixOS/nix/blob/2.33.1/src/libstore/profiles.cc#L306).

### Deep implementation review and corrections (2026-10-09)

The fresh review of `d2b91d8` completed with `claude-opus-5-5` observed, `xhigh`
requested, and executed effort unexposed. It made 118 read-only calls, returned
normally, and had no permission denials or guard violations. It did not approve
shipment. It covered the implemented production paths broadly but explicitly
left several test files and most historical reproduction logs unreviewed; the
required final full-delta review therefore remains outstanding.

| Finding | Reconciliation |
|---|---|
| N1: recovered provider journal stays incomplete | Accepted; seal verified completion under the mutation lease before the core clears recovered intent. Retry, shrink and successful abandonment need regression coverage. |
| N2: Ctrl-C during elevation leaves an unknown launcher | Accepted; protect the controller handoff with caught signals until its outcome is durable. Abrupt uncatchable death still requires conservative recovery. |
| N3: Linux health checks assume the legacy alias | Accepted and reproduced with actual executable consumption from an XDG-only profile. Discovery now supplies the alias containing the managed member. |
| N4: Darwin frontend XDG state differs from elevated activation | Accepted and reproduced against the native constructor. Darwin provider state follows the account home; the worker removes an inherited state override before elevation crosses into Home Manager. The engine ledger location remains independent. |
| N5: existing canonical destination omitted before alias creation | Accepted and reproduced using an actual private Nix profile. Discovery now inventories that destination without creating either alias. |
| N6: canonical alias creation after completion rejected | Accepted; canonical initial creation remains allowed after completion, while redirects and deletions still fail. Native completed-journal coverage is being added. |
| N7: needs-action keeps intent open | Existing documented contract, not an undocumented regression. The connected recovery UX must distinguish steady-state prerequisites from an interrupted provider operation; disposition remains open for that implementation. |
| N8: missing apply selection means empty selection | Accepted at the machine-request boundary; omitted/null selection must not authorize removal. Correction pending. |
| N9: required checks, stale docs and npm-prefix consumers | Accepted. Required-check cutover must follow invariant 27 after the final job topology is stable. npm-prefix removal must retain external global-package consumers; its adapter is not implemented. |

The three layout regressions failed before correction and pass afterward. The
only preparatory production edit for N3 added an unused, unexported discovery
field; it did not change the old health-check behavior. The private profile
fixture never activated or selected this Mac's real profiles. Full gates and
hosted verification of these new corrections are pending.

Evidence collected while the review ran closes its current-head runner gaps:
all 26 checks passed at `d2b91d8`, including native PowerShell suites, Go CodeQL,
uncached native core/terminal tests, and real Linux/macOS engine lifecycle. Both
uploaded journal sets were read back: six successful bound attempts, generation
7, complete provider intent and no core transaction. Both used synthetic merge
source `68d158c8d6065743aae0b7aac5b715c2d1a14d6b`. The stronger Darwin descriptor
readback passed without manually closing the lease. These facts postdate the
review's frozen evidence and do not constitute its approval of them.

The review's claim that a disposable Windows/WSL host and final-head CodeQL were
unavailable is corrected: approved GitHub runners supplied both. New selective
Windows/WSL lifecycle proof remains absent because those adapters are unfinished.
The separate F5 lease-rebind protocol, exact-target recovery, Homebrew projection
preservation and public distribution remain required work.

N1 now has a failing-before/passing-after real-filesystem boundary regression:
retry and abandonment previously cleared core intent while leaving the separate
provider journal incomplete; a simulated provider journal-write failure also
returned ready. The engine now calls provider finalization under its locks before
clearing intent. Nix seals only an exact successful target after the original
preservation check. The hosted fixture additionally reconstructs the lost-final-
write persisted shape from a real successful worker attempt, then retries and
adds an actual foreign profile input. That native result remains pending.

N2 now has failing-before/passing-after process-group evidence for SIGINT, SIGHUP
and SIGTERM. The handoff was first extracted without changing its signal policy;
each old controller died before recording the launcher outcome. It now catches
signals before publishing handoff intent and stops catching them only after the
launcher outcome is durable. The exec'd launcher retains normal signal behavior.
An actual password-prompt fixture remains separate from this process-boundary
proof, as does uncatchable controller death.

N8's JSON-boundary regressions fail before and pass after explicit-selection
validation. Internal Go request construction retains its existing semantics.
The first uncached Go suite exposed a Keep round-trip fixture that serialized an
implicit null selection; it now explicitly requests `selected: []` while retaining
both original nil/empty Keep assertions. This aligns the fixture with the new
wire contract rather than weakening its preservation check.

The corrected uncached Go race suite passes, including native local terminal
sessions. The expanded hosted lifecycle starts with both aliases absent but the
real seeded canonical profile intact. Linux uses XDG-only consumption; Darwin
uses a custom frontend state directory while activation remains in the native
account environment. It also verifies native creation of the other canonical
alias after completion. These are pending hosted cases, not local activation
claims. The native worker rejection fixture now separately covers a changed
Darwin state home while preserving its existing account-home rejection assertion.

The full correction gate passed with an unchanged tracked tree: `make ci`, an
explicit verbose run of the native build/private-profile and new regression
fixtures, and uncached `go test -race -count=1 ./...`. No hosted result is inferred
from those local checks.

A subsequent source pass found a descriptor boundary before the prior F4 fix:
`RunNixWorker` validates the native account before reaching `runNixWorker`, so
account-lookup subprocesses could inherit both worker descriptors. A real exec
fixture proves that even rejected preflight left the exact lease and private
directory in a subsequent child before the correction, and neither afterward.
The entrypoint now marks both close-on-exec before parsing or validation. The
existing activation-child test still proves explicit lease inheritance without
the private directory. This regression tests the entrypoint boundary; it does
not claim instrumentation inside Directory Services or NSS. The updated full
gate and hosted native worker execution remain pending.

The worker-preflight correction subsequently passed `make ci`, the explicit
verbose native build/private-profile/regression run and uncached Go race tests.
The captured tracked-file hashes remained identical throughout all three checks.
Only this evidence/status Markdown was updated afterward. Expanded hosted
lifecycle results and independent review of these corrections remain pending.

### Expanded native findings and explicit lease recovery (2026-10-09)

At `a4c7484`, Linux's XDG-only fixture failed inside the real activation:
Home Manager inspected the compiled legacy `home.profileDirectory`, did not see
the modern manifest, and chose `nix-env -i` against the actual XDG JSON profile.
The earlier N3 consumption correction alone was insufficient. The controller now
binds native `nix config show use-xdg-base-directories` and state home into its
snapshot and immutable projection. The locked Home Manager module's supported
`nix.assumeXdg` option selects the matching format check and session path without
changing native Nix settings. Existing generation/environment disagreement fails
for adoption rather than reusing session infrastructure with another meaning.
Optional environment fields preserve older journal serialization; newly present
environment records require an explicit boolean and canonical native path.

The new pure projection fixture initially assumed `nix.enable` was false, then
assumed changing state home leaves every infrastructure filename unchanged.
Inspection of the locked module disproved both assumptions: `nix.enable` defaults
true without generating config when settings are empty, and XDG always supplies
its state-directory `.keep`. The corrected assertions require unchanged Nix
enable/settings and unchanged file targets except that explicitly relocated
`.keep`. Other existing targets include the cache `.keep`, Home Manager's
environment.d session file and systemd tray target; these belong in the remaining
complete side-effect inventory. No existing test assertion was removed.

macOS passed the native lifecycle through recovered completion and later foreign
input at this head. It then failed the new alias-query fixture: `nix-env -q`
rejects the seeded JSON profile even when its alias creation succeeds. The fixture
now uses `nix profile list --json`, retaining both alias-creation and subsequent
observation assertions. The full macOS job did not pass; corrected hosted proof
is pending.

F5 now has a separate preview/apply lease-recovery API. It requires a kernel
session different from both the original attempt and the latest recovery binding,
holds the engine and replacement lease locks, and verifies an exact approval.
Absent leases are created exclusively only after approval. Original request and
outcome files stay unchanged; prior bindings are archived and their chain is
validated, bounded to 64 repairs per attempt. Restoring the original inode cannot
bypass a newer binding. This restores the guard only, not target completion.
Real-file tests cover changed requests/inodes, symlinks, occupied locks, cancelled
approval, missing leases, success-without-boot-change, failed publication and
history tampering. They model persisted prior-boot records, not an actual reboot.
The first chained-history test found reuse of a decode destination retained an
omitted previous-link field. Fresh decoding fixes that reproduced failure.

N2 additionally has a pending hosted PTY fixture at the actual sudo password
prompt: Ctrl-C, terminal closure and SIGTERM. On explicitly disposable runners,
it temporarily requires authentication for `/usr/bin/true`, validates sudoers
before use and restores the exact original file on exit. It supplies no password.
This exercises the production handoff with real sudo authentication, separately
from the existing real process-group regression. Developer machines reject it
before sudo or activation. Full gate, hosted results and independent review of
this correction stage remain pending.

The first full gate for this stage stopped in the static architecture checks.
The ownership scanner tokenized `==` as two assignment tokens, misclassifying the
new read-only `home.file` equality assertion as ownership. Its self-test reproduces
that false positive before the lexer fix and passes afterward; all existing
direct/nested/wrapped/imported ownership rejection cases remain. Comparison tokens
are now distinct from assignment. Negative Windows-host path inputs are stored
as JSON fixture data and fed through the same Nix rejection assertions, rather
than embedded as paths in executable `.nix` source. The host-path code guard is
unchanged. No checker suppression or exclusion was added.

The downloaded `a4c7484` artifacts confirm the differing native outcomes: Linux
has one failed attempt, generation zero and retained core/provider intent. macOS
has six successful bound attempts, generation seven, Node ownership, complete
provider state and no unfinished core transaction. Its final test-query failure
does not erase that evidence, but neither does the evidence make the job green.

The next gate reached shell lint and flagged the sudo fixture's privileged read
with an unprivileged output redirection (SC2024). The intended writer is indeed
the current user; the pipeline now expresses that with unprivileged `tee`.
No suppression was added. The runner's original sudoers file is still restored
using its preserved permissions and ownership.

The corrected stage passed all three local gates with unchanged tracked source:
`make ci`, explicit verbose native build/private-profile/recovery tests, and
uncached `go test -race -count=1 ./...`. Only this evidence/status Markdown was
updated afterward. The native configuration-change regression also fails against
an isolated copy of `a4c7484` and passes on the corrected tree. Updated hosted
activation, actual sudo-prompt interruption and independent review remain pending.
At `a4c7484`, 21 other hosted checks passed, four Nix producer/logical checks
failed, and the WSL guest-startup check was cancelled. GitHub returned HTTP 404
for that job's log, so no cause or successful startup is inferred; job-step
metadata records startup as unfinished. The next hosted run must prove it again.

### Native correction proof and bounded recovery history (2026-10-09)

All 26 hosted checks passed at `b6f2a74e406c45ce23428f6ca07b4b9e706b1eb6`.
The [Nix run](https://github.com/luisgui1757/dotfiles/actions/runs/37893676970)
passed both real engine lifecycles and actual sudo password-prompt cancellation
(Ctrl-C, terminal hangup and SIGTERM) on Linux and macOS. The corrected Linux
XDG projection activated successfully; the macOS canonical-alias query passed.
The [engine run](https://github.com/luisgui1757/dotfiles/actions/runs/37893676954)
also passed real WSL2 startup, superseding the preceding cancelled capability
probe. Existing Windows/macOS/Linux setup, parity and CodeQL checks passed.

Uploaded Linux/macOS journals were independently read back. Each contains six
successful schema-2 attempts whose result request digests, attempt IDs and boot
IDs match the original requests, generation 7 with Node selected, a completed
provider journal and no unfinished core transaction. Their actual build source
is synthetic merge `062c09caa15518258faaeea41ef50ae8032b326b`. These fixtures prove
the implemented package plane, not the missing public installer or adapters.

The fresh read-only Opus review observed `claude-opus-5-5`, with `xhigh` requested
and actual effort unexposed. It inspected source and evidence using 80
Read/Glob/Grep calls, then returned an account usage-limit error (reset 09:20
Europe/Berlin), not a review verdict. The harness initially classified the CLI's
`<synthetic>` error message as an unexpected model; inspecting the typed
`usage_limit_reached` event confirms that no alternate reviewer ran. This round
is incomplete and supplies no approval. A fresh full review remains required.

A subsequent boundary pass reproduced a lease-history defect: with 64 valid
persisted repair records, preview authorized a 65th and apply published it even
though the reader rejects chains longer than 64. The real-file regression fails
before the fix at the 64-record boundary and passes afterward, including allowed
publication of the 64th record and continued readability of existing history.
Preview now counts the validated chain and rejects overflow before altering the
lease or records. The focused recovery suite passes; the complete gate and
independent review of this correction are pending.

Concurrent PR #88 addresses login-shell startup and overlaps the publisher and
Hyperfine compatibility changes. It remains separate, unmodified work; it is
not another installer delivery. Reconcile any merged baseline changes before
final verification, preserving its login-shell behavior and the stronger
publication-lifetime guarantees established in this installer change.

### Profile-generation history and interference detection (2026-10-09)

The 64-record lease-history correction passed `make ci`, explicit native/private
profile regressions and an uncached `go test -race -count=1 ./...`, with unchanged
source hashes throughout that gate. Independent review remains outstanding.

The next native private-profile regression reproduced an approval gap: install a
package, capture the snapshot, change its priority with `nix-env --set-flag`,
then roll back. The active package metadata returns exactly to its baseline,
but the extra generation proves an intervening writer. Previously both approval
identities were equal. The new generation-history snapshot changes that identity;
the same regression now passes. It never activates the developer's default
profile and operates only on its private temporary profile.

Completion now audits every new generation of the watched home, system and
default profiles. It preserves baseline links, rejects gaps or inserted older
numbers, permits only the approved home/system target, and reads each new Linux
default-profile manifest to verify unchanged foreign inputs and canonical managed
placement. Darwin default-profile publication is rejected. Reobservation after
manifest reads catches changes during the audit. Final state alone does not prove
that every intermediate generation preserved the approved inputs. Unknown or
malformed generation-link layouts fail closed. Completed older journals remain
readable; unfinished records without history cannot gain completion authority.

Real-file tests cover absent namespaces without writes, malformed/indirect links,
pre-existing gaps versus new gaps, baseline deletion/replacement, namespace/role
changes, foreign intermediate activation with an exact final target, Darwin's
unchanged-default-profile rule and legacy missing evidence. A native JSON-profile
fixture passes Home Manager's remove/add sequence while preserving an unrelated
package, then detects a foreign add/remove whose final contents match again.
Its first run exposed incorrect fixture CLI syntax (`--profile` before the
subcommand and unsupported `--name`); inspecting the installed native help fixed
the invocation and retained the original preservation assertions. All focused
history tests pass. Both hosted POSIX fixtures now require these native cases;
the complete local/hosted gates and independent review are pending.

This implements the history component of the accepted recovery protocol. It does
not yet implement the common partial-state classifier, exact-target resume,
compensation or the full native interruption matrix. Direct/older Nix writers
still lack universal exclusion; the documented same-generation-number lost-update
race remains an upstream limitation rather than a claimed guarantee.

The first broader Go suite rejected the old positive completion fixture because
it supplied no generation history. The fixture now supplies actual temporary
profile links, retains all original ownership/preservation assertions, and adds a
foreign-history case through the same package-observation/recovery boundary.
Missing legacy proof has its own explicit rejection test and serialization
round-trip. Final focused history/completion tests pass, including reobservation
of a profile changed after its snapshot. No assertion was weakened to accept
missing history.

The complete local gate exposed an absolute-generation-link spelling mismatch on
macOS: `/var/...` and `/private/var/...` identify the same parent directory. A
separate directory-redirection fixture reproduced the rejection before the fix.
The reader now resolves only the link's parent, retaining the generation filename
needed for the history audit. The new fixture, the original native namespace
regression, native rollback/intermediate-profile tests and interrupted-completion
cases pass. The full gate is being repeated; the failed run remains evidence.

The native generation counter is an unsigned 32-bit value. Existing terminal
numbered history remains readable, but activation now rejects an exhausted
counter before building or publishing another generation. The boundary test
passes; Darwin default profiles are excluded because this adapter never publishes
them. This follows the pinned Nix generation-numbering contract.

The corrected history checkpoint passes the complete local `make ci` gate,
explicit verbose native/private-profile regressions and uncached
`go test -race -count=1 ./...`. Source hashes remained unchanged across all three
commands. Hosted verification and independent review remain pending. The live
plan and agent guide now consistently identify PR #87 as the consolidated
implementation; references to a separate plan delivery were stale. Historical
review entries are preserved rather than rewritten.

### 2026-10-09: completed history review and R-series corrections

All 26 hosted checks passed at `979c448`, including native Linux/macOS history
auditing. Both downloaded journal sets contain six successful schema-2 attempts
whose request digests, attempt IDs and boot IDs match; state is generation 7,
Node remains selected/owned, provider completion is sealed and core intent is
clear. The executed synthetic merge source was
`aa39ab7a5f846bc080f02dbb537135908c255c82`. WSL evidence remains startup/kernel/
Windows interoperability, not the unimplemented selective installer lifecycle.

The fresh Opus review completed after 119 Read/Glob/Grep calls. Actual model:
`claude-opus-5-5`; requested effort: `xhigh`; actual effort unexposed. Exit was
zero, with no API errors, permission denials or guard violations. It read the
full corrections and full-diff hunks, plus the previously unread terminal and
core test areas. It explicitly lists other baseline tests/build files/docs it
did not read in full. This is useful independent scrutiny, not whole-product
approval. Its hosted-evidence paragraph predates completion of the `979c448`
Nix/e2e jobs; their final successful results and downloaded journals now exist.

| Finding | Disposition and evidence |
|---|---|
| R1 Medium: cancelling then abandoning an unstarted target strands later no-op apply at finalization | Reproduced through real files, native leases, the Nix driver and engine; only the native Nix configuration process is stubbed. Abandon succeeds, then apply fails before the fix. Finalization now seals only an actually activated target; the same test passes and verifies byte-identical provider intent and cleared core transactions. The hosted real sudo prompt additionally runs cancellation, abandon and empty apply. |
| R2 Low: SIGQUIT bypasses durable handoff closure | A real foreground process-group regression failed with Go's SIGQUIT exit before the fix and passes afterward. `Notify`, not `Ignore`, now covers QUIT alongside INT/HUP/TERM. The PTY fixture sends actual Ctrl-backslash to sudo and requires subsequent abandon/apply success. |
| R3 Low: managed preflight accepts metadata native activation destroys | Reproduced against private native legacy profiles. Explicit default priority/active and custom description disappear during store-path replacement; keep preserves the old member and causes a real buildEnv collision. Modern remove/add also drops source fields. Preflight now accepts only empty legacy metadata or the exact normalized modern active/path/priority fields. Focused native tests pass without activating the developer's profiles. The old test declaring legacy priority="5" canonical was incorrect and is corrected against actual native output. |
| R4 Low: sibling profiles block history discovery | Real-file regression failed on profile-work-3-link. Native `nix-env --list-generations` confirms it ignores sibling names, nonnumeric/negative values and uint32 overflow, but counts leading plus/zero forms and suffixes after -link. The reader now ignores exactly the former and rejects counted noncanonical forms. Native enumeration parity and real-file tests pass. The old overflow-filename rejection expectation was a false positive; an existing maximum valid generation still blocks activation. |
| R5 Low: stale status text | Current status now identifies the last verified checkpoint, completed reviewer execution, passed sudo fixture and the new corrections' outstanding gates. Ambiguous reused N-series labels are removed from the status table. |

Failing-before logs cover R1/R2/R4 and R3's metadata preflight plus actual legacy
replacement. Corrected focused tests and native metadata/generation-name checks
pass. Complete local gates, expanded native hosted fixtures and review of the
corrections remain pending. The fresh Linux legacy/fresh-profile full activation
path, actual reboot, shared partial-state classifier/exact-target resume and all
previously listed public-product milestones remain required.

The R1-R5 correction now passes `make ci`, an explicit verbose native regression
run and uncached `go test -race -count=1 ./...`, with unchanged source hashes
throughout all three commands. R3's complete native test, including modern source
metadata, was also run against the original `979c448` reader in a separate
temporary module and failed on the expected preflight assertions; the current
reader passes. No developer profile was activated. Expanded hosted proof and
independent review of this correction remain pending.

### 2026-10-09: hosted parser mismatch and correction-review follow-up

The `7c7cf89` hosted run completed with 22 successful checks and four failures.
Both Nix producers failed only the new generation-enumeration assertion for
4294967296; their two logical-proof jobs correctly failed without producer
proof. The native engine lifecycle, metadata preservation and real sudo prompt
cancellation (including Ctrl-backslash followed by abandon/apply/repair/update)
passed on both OSes. These individual passes do not make the failed jobs green.
Run: `37902286808`; Linux job `113727407886`; macOS job `113727408109`.

The cause is a real upstream change, not a platform-specific test workaround.
[Upstream commit 6a762407](https://github.com/NixOS/nix/commit/6a762407c7d2d94a406ad7d1fd083deef726cf20)
changed parsing from unsigned int to the 64-bit GenerationNumber type. Native
Nix 2.34.7 on the developer host ignores 32-bit overflow; hosted Determinate
3.23.1 embeds Nix 2.35.2 and counts it. The earlier ledger statements calling
the native counter universally 32-bit and overflow universally unrelated are
superseded by this evidence. Nix 2.34.8 still uses the old parser; Nix 2.35.0,
2.35.1 and 2.35.2 use the new parser.

Histories now record their native parser width, selected from the read-only
`nix-env --version` query. Generation maps preserve all uint64 values; omitted
width retains the previous JSON's 32-bit meaning and identical serialized bytes.
A parser-width change during unfinished activation cannot authorize completion.
Both widths enforce their own limit before publication. Native enumeration
checks both sides of the 32/64-bit boundaries and logs the actual Nix version.
The local native checks pass on 2.34.7; hosted 2.35.2 proof remains pending.
The alternative of enumerating via `nix-env --list-generations` in preview is
rejected: its implementation opens a writable profile lock and therefore does
not satisfy the read-only discovery contract.

The fresh Opus correction review completed with 118 Read/Glob/Grep calls,
actual model `claude-opus-5-5`, requested effort `xhigh` (execution effort
unexposed), exit zero and no API errors, permission denials or guard violations.
It found no new medium-or-higher defects in the reviewed corrections, but did
not approve the unfinished product. It predates the hosted failure readback.
Its statement that R4 exactly matches native parsing applies only to the older
supplied Nix source/runtime; the new width regression corrects that limitation.

| Finding | Disposition and evidence |
|---|---|
| Correction L1 Low: absent/null legacy metadata passes preflight but changes during activation | ACCEPT. A manually constructed immutable legacy manifest reproduces actual null placement and native replacement with an empty object. The preflight test fails against `7c7cf89` and passes after requiring an explicit empty map. Native Nix does not normally emit the malformed input, but discovery must reject it before mutation. An initial fixture imported another store manifest and correctly failed the restricted evaluator; the corrected fixture embeds that manifest directly and reproduces the intended defect. |
| Correction L2 Low: stale status and ambiguous labels | ACCEPT. Updated roadmap, stage-3 status, installer guide and old pending statements; removed ambiguous N1/N4/N5 references from the guide. Historical ledger paragraphs are preserved with this correction appended. |
| Correction L3 Low: hosted WSL documentation conflicts with the new job | ACCEPT. Guides now identify the bounded non-required startup/interop probe, preserve the provider support limitation, and distinguish it from unproved full guest lifecycle. |
| Checked-in security command would remove live Go coverage | ACCEPT. Live readback confirms Actions/Go/Python with default queries. The production-command fixture fails against the old two-language policy. The correction preserves all three languages and requires successful Go analysis for exact live main; tests reject missing, failed, wrong-SHA and pull-request-only results. Tests mock only the GitHub CLI boundary and make no remote mutations. |

The correction review also lists coverage/evidence gaps, including full-delta
areas not reread, precise earlier command metadata, boot IDs in readback
summaries and a local positive finalization regression. The corresponding
native activated-target recovery step passed in this hosted run, but the local
positive test and complete final review still need completion. Real reboot,
fresh/legacy full Linux activation, shared partial-state recovery, remaining
providers and public distribution/UX remain unfinished. Focused corrected
native tests and the security command fixture pass; the complete revised gate,
hosted run and next independent review remain required.

The positive-finalization evidence gap is resolved by the native fixture rather
than a duplicate lower-fidelity local test. `TestNativeNixDriverLifecycle` at
`nix_driver_native_posix_test.go` recreates lost provider/core completion writes
after a real activation, retries through the engine, and requires the original
attempt to be sealed. Both `7c7cf89` jobs passed that exact step. Its logs are
available for the next review, which must distinguish the passed step from the
later independent parser assertion that failed the overall jobs.

The width/metadata/security correction passes the complete `make ci` gate,
explicit verbose native regressions and uncached `go test -race -count=1 ./...`.
Tracked source hashes stayed unchanged through all three commands. Earlier
R1-R4 failures were reconstructed from the original `979c448` production files
and `7c7cf89` tests in a separate module, with complete source hashes, argv,
Go/Nix versions and the native-package environment recorded. L1 was independently
reconstructed against complete `7c7cf89` production code with only its two new
regression functions. Both reproductions fail at the expected assertions.
Hosted verification and independent review of this new correction are pending.

## 2026-10-09 — Exact-target recovery implementation

All 26 hosted checks passed at `9a98a44`, including Nix producer jobs
113739043229 (Linux) and 113739043292 (macOS), run 37905866204. This closes
hosted verification of the parser-width/null-metadata corrections. Uploaded
journals and producer logs were downloaded; their new readback is pending.

Decision: recovery is its own approved phase, followed by a fresh preview of the
original transaction's remaining work. Reusing the entire earlier preview would
make later choices depend on half-published observations. Rebuilding a target
would change the original intent. Instead, a token binds the original journal,
current snapshot and authenticated outcome; resume keeps the same immutable
store generation and original member operation IDs/baselines.

The core now routes only a matching explicit retry to `ResumeBatch`, retains the
original transaction, and does not claim full readiness after recovering one
provider. Real-file tests cover partial publication, two attempts, original
ownership and recovery references, no rebuild and stale approval refusal. Nix's
shared classifier covers supported Linux/Darwin publication tuples, rejects
foreign inputs/metadata/generations and dangling roots, and requires a recorded
successful outcome before completion. Workers recheck the approved partial
snapshot after elevation. These focused tests pass locally; the native fault
matrix, complete local gate and independent review of this stage remain pending.
The full installer remains unfinished for the earlier documented reasons.

The `9a98a44` artifact readback passed for both OSes. Both use synthetic merge
source `78d2b5d87eb71336eaa7b0e786d543ec13e103d7`, generation 7 and six
successful schema-2 attempts. Request digests, kernel boot IDs, worker identities
and attempt IDs match; the final provider is complete and the core transaction
is absent. Boot IDs are included in the new readback artifacts.

Recovery testing also reproduced two continuation hazards: prematurely deleting
removal receipts invalidated the original shared-removal selection, and replaying
an update could repeat an already operation-proved update. Completed removals now
retain a non-owning receipt until reconciliation, while exact proved updates are
kept. Focused tests pass, including repeated resume and interrupted removal.

A new real-filesystem regression demonstrated that switching a watched profile
back to an existing foreign generation could pass the generation audit without
creating any new link. Verification now checks its currently published target
against the original/approved targets as well as auditing every new generation.
The test fails before this change and passes afterward on both policy contexts.

The hosted native lifecycle now includes external-command failure boundaries on
Linux (before/after HM declaration, after package removal, before current-home
publication), plus an actual preserved user-file conflict during elevated Darwin
activation. Each case must resume the same target and original ownership IDs,
continue the original operation and remove the installed fixture through the
real engine. This newly added native execution remains pending.

The exact-target recovery change passes the full local `make ci` gate, explicit
verbose native/private-profile regressions and uncached `go test -race -count=1
./...`; source hashes stayed unchanged across all three commands. The initial
gate found only space indentation in a raw Go test fixture; the fixture now
follows the Go editorconfig rule and the complete gate was rerun. A separate
temporary module reconstructed the earlier update replay and profile rollback
bugs and failed at both expected assertions, with command/environment/source
metadata recorded. These local results do not claim that the newly added hosted
partial-publication scenarios or the complete installer have passed.


## 2026-10-09 — exact-target review corrections and connected controller

The fresh bounded review of `5a1f5e9` completed successfully: actual response
model `claude-opus-5-5`, requested `xhigh` (effort unexposed), 80 read-only calls,
zero scope-guard/API errors and permission denials. The first invocation was
stopped when it tried to Glob the snapshot parent; the completed invocation
kept all evidence beneath one allowed source root. Neither run approved product
shipment. The reviewer had prior `9a98a44` native evidence; the later `5a1f5e9`
results described below arrived after its launch.

Accepted findings and dispositions:

- **F1 High, abandonment/ownership:** pending observations could enter `forget`,
  and unstarted journals offered resume without live core intent. Reproduced
  before correction. Unknown presence now retains ownership and prerequisites;
  this also preserves the pre-existing rule that an unchanged pending external
  resource does not make unrelated work fail. Only inventory with matching core
  intent offers recovery. An unchanged abandoned baseline is observed normally;
  a partly published target refuses abandonment before core intent is cleared.
  Receipt status alone cannot prove live intent after abandonment. Native owned
  baseline -> abandon -> same selection/repair/update/removal coverage is added.
- **F2 Medium, removal retry:** both ordinary single and grouped removals deleted
  receipts before finalization. Reproduced in both paths. All removal paths now
  keep non-owning tombstones until successful core reconciliation, including a
  second interrupted retry's `forget` path. The regression interrupts finalization
  twice, then proves successful retry without repeating removal.
- **F3 Low, old unfinished shape:** missing root metadata blocked even a proved
  unchanged baseline. Reproduced. Old snapshots may classify only as not-started
  when all old fields and the independently derivable current root agree; no
  partial-state or target-completion authority is inferred.
- **F4 Low, handoff publication:** the journal required the latest attempt pointer
  to match, which a two-write crash could prevent permanently. Reproduced with
  real request/pointer files. Outcome reads now load the journal's exact request;
  the guard pointer publishes before the journal switches attempts. An unclosed
  same-boot handoff still blocks until restart; absence is not process-exit proof.
- **F5 Low, documentation:** updated stale gate/recovery status in the status,
  plan, roadmap and contributor guide. Historical review entries remain intact.

Hosted `5a1f5e9` finished with 24 successes and two Linux Nix failures. Run
`37910072846`, Linux producer `113752839733`, failed before the intended injected
boundary because Home Manager's reconstructed PATH omitted nix-build. Its normal
lifecycle had passed. The fixture now forwards all native Nix aliases before
wrapping its three fault boundaries, passes the subtest handle to approval,
logs the actual failure, and stops instead of cascading into unfinished state.
A private native version probe checks nix-build/nix-instantiate dispatch without
activating the developer profile. Linux proof remains pending on the next head.

macOS producer `113752840071` and proof `113756314999` passed. The real user-file
conflict resumed original plan
`0aa6b623edaa1a7fe9af0578f9553316b4ce859e3a64bbc9a6b8f6ea953bdaf1` and target
`/nix/store/2bpnl8havg7r45qwrwpaxphny356q69y-darwin-system-26.11.d5bd9cd`.
Readback verified generation 9, nine schema-2 attempts (eight success, one expected
failure), matching boot `648374b5-d2d6-4299-9e50-afc4e4c909cc`, compact Go JSON
request digests, immutable worker/lease identities, provider completion and no
unfinished core transaction. This closes the review's missing macOS root/runtime
proof for that checkpoint, not its newly found code defects or reboot coverage.

The controller now connects selection, removal, update, repair, check and explicit
retry/abandon to the same engine. Capability checkboxes never copy a dependency
list. Shared removal is a separate per-item choice; unchecked resources become
Keep roots that can be changed later. A full scrollable plan precedes a separate
confirmation whose default is Back. Every terminal method restores input before
provider execution. Machine dispatch previews the original requested mode and
requires its exact plan identity to execute. Public launchers and plain accessible
input remain implementation work.

Local real-file controller tests cover both shared-consumer removal orders,
update/repair selection preservation, durable Keep/removal, read-only checks,
cancellation without state, stale approval and explicit recovery. Real macOS
PTY tests exercise the connected controller through private-file mutation and
prove that a subsequent ordinary prompt retains input and terminal state. This
is connected UX proof, not package-provider lifecycle proof. The full uncached
Go race suite passed before the final preview-guard/native-fixture additions;
complete revised gates and next-head hosted verification are pending.

The controller/recovery full local gate, explicit native regression selection
and uncached Go race suite all passed with unchanged source hashes. The separate
native fault-alias probe then exposed a test expectation that assumed upstream
`(Nix)` branding; the actual command correctly reported
`nix-build (Determinate Nix 3.21.1) 2.34.7`. The probe now checks the executable-mode
field, preserving the dispatch assertion for both supported Nix distributions.
This is a fixture correction; production command behavior did not change. Its
focused rerun and the final gate are recorded separately from the first pass.

Final correction verification passed: `make ci`, the explicit native/private-profile
and controller regression selection, and `go test -race -count=1 ./...` all exited
0 with unchanged tracked source hashes. The separate actual nix-build and
nix-instantiate version-mode probe also passed on Determinate Nix. The original
`5a1f5e9` failures were reconstructed from that exact Git revision with the new
regressions, recording commands, environment, Go version and source hashes;
all five tests failed as expected, including both single/batch removal cases.
Only these status/result Markdown updates follow the gate. Hosted verification
and independent review of the revised code are still outstanding.


## 2026-10-09 — controller review and verified hosted checkpoint

At `ad6bfe892733194176f433f92cd3a6caface1ed2`, all 26 checks passed. Core run
`37915662927` passed Linux, macOS, Windows ConPTY and WSL2 capability. Nix run
`37915662931` passed producers `113771090477` (Linux), `113771090161` (macOS),
and both independent proof jobs. Journal archives matched their GitHub digests:
Linux `ce749c124609b9dc9ca2479a054252488ff8b89c2d7980826cd09baa44eb55ad`,
macOS `4a28ccdb9f91c0e8db0312877270742489743cc84bcd13c5890371cc2a094068`.
Their executed merge source `e5a654d99661c62f67060194c4770c49b722b85e` has the
same tree as the head (`69298c431c13cbc4aa65d96c6a5f25cfd95023c2`). Readback
verified Linux generation 21/22 attempts (17 success, five expected faults),
macOS generation 9/nine attempts (eight success, one conflict), exact compact
request digests, boot/worker/lease bindings, complete provider journals and
no open core transaction. This closes the earlier fault-fixture gap.

The checkpoint review completed using actual `claude-opus-5-5`, 81 read-only
calls, no guard/API errors or permission denials. `xhigh` was requested but the
CLI does not expose execution effort. F1–F4 corrections held on source review.
This is not full-product approval; the reviewer explicitly listed unread older
files and missing production providers/distribution.

- A Medium: accepted the original-revision recovery availability requirement.
  Retry and partial-abandon refusal now name the original revision. The delivery
  layer must retain and authenticate the exact original engine/source; it is
  still implementation work, not resolved by clearer error text alone.
- B Low: accepted. Check now names affected resources and unfinished intent;
  healthy resources cannot turn an unfinished transaction into ready. Native
  output uses the scrollable reader. Focused failure/repair tests pass locally.
- C Low: accepted. Review begins with action counts and removal/privilege items;
  position is visible and confirmation repeats counts. The old rendering test
  failed before correction; corrected terminal tests pass.
- D Low: accepted. The shared-removal menu replans all candidates and offers only
  actual removals, preventing phantom Keep roots for retained-app prerequisites.
  A real-file controller regression failed before correction and now passes.
- E Low: accepted. Internal apply, like JSON apply, requires an explicit selected
  list. Existing deselection fixtures now explicitly pass an empty list; no
  assertion was removed. The missing-selection regression failed before the fix.
- F Low: accepted. Current status distinguishes historical checkpoints from
  verified recovery and controller behavior. History remains append-only here.

First-run selection now follows the accepted UX contract directly. Its old
behavior failed a controller regression; the corrected test passes. The native
fault fixture also stops its parent lifecycle after a failed resume scenario.
The complete revised gate and independent correction review remain pending.

## 2026-10-09 — User-authorized major release, first-principles reconciliation

A fresh independent `claude-opus-5-5` consultation completed with 56 Read/Glob/Grep
calls, exit 0, successful terminal result, no guard/API errors or permission
denials. Requested effort was `xhigh`; execution metadata did not expose effort.
It inspected the complete source snapshot and recommended a private rooted Nix
package set, removal of Home Manager/nix-darwin/nix-homebrew, bounded native
configuration management and explicit migration without chezmoi execution.

Accepted: eliminate shared system/default-profile activation, preserve locked
nixpkgs and native vendor channels, preserve scoped configuration recovery, and
keep migration distinct from normal lifecycle execution. Reconciled differences:
package appearance alone is not creation proof, and attributable unused native
dependencies require scoped removal rather than a blanket leftovers policy.
The old Darwin Homebrew takeover requires actual native detach proof before
migration can claim complete removal of old activation infrastructure.

Verification: the complete local baseline `make ci` passed (Go 1.27.1); native
PowerShell checks were explicitly skipped for the absent local runtime. A
private build-only macOS nixpkgs environment consumed hello and jq and exposed
jq's binary/manual output group. Nix buildEnv repeats the manual path when it is
also requested as an extra output; compare unique outputs rather than inventing
a third output. No developer profile or system activation was performed.
This is architecture consultation, not final implementation review or shipment.

### Major-release implementation checkpoint

Private Nix projection evaluation passes for all bound singletons on all three
POSIX targets. A real macOS controller fixture passed install/grow/update/shrink,
empty removal and reinstall, consuming jq/fzf from the private environment and
preserving jq binary/manual outputs. Removal releases unused private roots,
without global GC. A real process exit after publication recovered the exact
operation without rebuilding. A separate real eight-second Nix build retained
its inherited flock for all 90 observed samples after the parent closed its
handle; Linux and parent-death publication barriers still require native proof.

The configuration table now contains 34 explicit canonical targets. Tests cover
full config-resource coverage, source existence/type, independent redirected
Windows folders, no discovery writes, source overlap, WSL gating and unsafe
mapping rejection. Windows uses copied configurations in the new major to avoid
symlink-privilege prerequisites; POSIX retains live links.

Explicit adoption and single-resource recovery were added to the shared core.
Real-file tests verify baseline restoration, unavailable/unselected adoption
refusal, an explicit menu choice, exact interrupted install/update/removal,
remaining-work review and stale partial-state rejection. The full Go race suite
passed. These tests do not replace the missing production configuration writer,
legacy migration, full hosted product lifecycle or final independent review.

The native parent-death fixture now passes: the real completed build is rooted
before returning to Go; after SIGKILL of the frontend, the surviving process
holds exclusion, cannot publish `current`, and releases the guard before retry
cleans up its unused root. The full local `make ci` gate passed with all native
private-package tests enabled. A separate Nix format check identified three
format-only corrections, which were applied; projection revalidation passes.
The new native tests are wired into both existing hosted Nix jobs, but no hosted
result for this working tree is claimed. Read-only native folder discovery uses
Windows known-folder APIs and validates POSIX home/XDG paths without creation.

Configuration inspection now distinguishes installed-target fingerprints from
approved source payload fingerprints. Both enter plan approval; the executor
refuses a changed desired payload before applying a later resource. Existing
matching files or links are healthy/reusable but do not establish ownership.
Parent redirection is resolved without creating directories and participates in
the target identity. Bounded snapshots do not follow user links, and copy staging
rejects links/special files, source drift, oversized files and existing targets.

A native macOS live-link regression failed before correction: newly created
symlinks had mode 0755, while the desired link assumed 0777. Link identity now
uses its destination, independent of native creation permissions; regular file
and directory permissions remain fingerprinted and preserved. The native test
and all focused configuration snapshot/inspection tests pass afterward under
the race detector. This does not yet implement configuration mutation/recovery.

### Native configuration writer checkpoint, 2026-10-09

The bounded mapping now has a journaled provider. It stages complete payloads,
records separate original baselines and per-operation publication intent, and
moves user files on the same volume with no-replace native renames. Adjacent
backups preserve native permissions and ACLs. Empty identity-named directories
mark staging ownership atomically, avoiding a torn owner-document bootstrap.
Cleanup intent precedes recursive disposal of obsolete owned payloads; the first
user baseline is always excluded. No configuration template language was added.

Native macOS tests cover copy/link install, repeated update, removal/reinstall,
explicit adoption, original-baseline restoration, directory copying, dangling
user-link preservation and refusal to overwrite later user edits. Persisted crash
fixtures cover staging, backup, publication and recursive-cleanup boundaries.
These fixtures reconstruct actual storage positions, not killed-process proof;
Linux/Windows native execution and public integration remain required.

Failing-before tests found three additional lifecycle defects in the new writer:
missing source prevented uninstall, changed recovery data was not reported in
preview, and interrupted cleanup could strand verified publication. Corrections
separate apply-only input blocking from removal, inspect baseline integrity, and
journal discard intent before cleanup. A separate initial test caught a shadowed
workspace-inspection error after successful directory creation; that was fixed.
All focused configuration lifecycle tests pass after these corrections.

### Opus native configuration review and first corrections, 2026-10-09

The first launch was stopped by the read-only path guard after three Glob calls
outside the supplied source directory; it produced no review. The corrected
launch completed with actual `claude-opus-5-5`, 57 Read/Glob/Grep calls, no API or
guard errors, and `xhigh` requested (execution effort unexposed). One native CLI
permission denial blocked the adjacent evidence file. Opus explicitly limited
its conclusions to source inspection. Future review bundles must place evidence
inside the allowed source tree rather than relying on an adjacent allowlist.

Accepted findings:

- High: re-reading historical journals and baselines on every finalization could
  permanently block unrelated transactions. Reproduced for unpublished metadata
  scratch, an edited old backup, foreign workspace files, a retired catalog
  resource and a missing historical baseline document. Fixed by sealing finished
  journals, skipping sealed history before catalog/baseline access, and parsing
  only canonical operation filenames. Unknown files remain untouched. Removal
  cleans the consumed original backup's empty container explicitly.
- Medium: an unstarted publication could not be abandoned after a user edited
  the untouched target. Reproduced and fixed: the recorded empty phase proves no
  move started; abandonment does not require the old target bytes to remain.
- Low: a missing baseline document could be confused with no operation journal.
  The journal reader now has a distinct absence sentinel; baseline errors do not
  authorize overwriting existing operation intent.
- Open: re-adopting an edited managed configuration must preserve the original
  pre-install baseline and disclose retained edited copies. Windows Neovim's
  writable lazy-lock.json makes this a normal lifecycle case.
- Open: interrupted moves need a safe restoration/recovery path when the target
  is recreated or source/target bytes change; forward-only retry can deadlock.
- Open: completed-resource path/mapping/probe errors must become resource-local
  needs-action outcomes, preserving unrelated operations. Verify Windows junction
  behavior natively rather than inferring it from cross-compilation.
- Open: strengthen parent-handle identity and flush through pinned handles;
  review live-link source-content approval, nested destination validation and
  explicit migration of old-provider receipts.

The writer accepted removal of the redundant workspace owner-marker directory.
Durable intent already reserves an initially absent, operation-specific path;
unknown contents are preserved and no target ownership is inferred from bytes
alone. The first correction's focused native configuration/race suite passes.
The complete gate passed before these corrections; a new gate, full correction
review and actual Linux/Windows runner execution remain required.

A separate frozen-snapshot provider fixture exercised every canonical config
mapping on the Mac filesystem: Darwin 13 resources/22 targets, Linux 12/21,
WSL default 10/19, WSL GUI 12/21 and Windows copy policy 11/24. Every fixture
installed, checked, updated and removed, with spaces/Unicode and independent
folder roots. This is mapping/component proof, not native Windows/Linux or
application-consumption proof.

### Native configuration review corrections continued, 2026-10-09

The re-adoption regression failed for both initially absent targets and adopted
user files: a second adoption reset `Before` and `Recovery`. The corrected engine
keeps the original owned receipt baseline. The configuration provider retains
each explicitly replaced edited copy separately and reports its path. Those
informational references survive update, removal and reinstall without becoming
deletion authority. Actual filesystem/controller tests cover two successive
edits, original-baseline restoration and the copied Neovim lockfile case through
the interactive update/repair choices. First adoption of an unrelated existing
resource remains unavailable in those maintenance modes.

Completed configuration mapping/probe failures now return `Unknown` plus a local
explanation; the core rejects an unknown observation that claims presence or
completion. Redirected folders and oversized files preserve their contents while
independent resources install/remove. Completed copies can update from a moved
checkout; source unavailability still permits independent baseline restoration.
Live-link input approval binds source type/path instead of mutable source bytes,
so editing through a live link does not strand its interrupted publication.
These source and mapping regressions failed before correction and pass afterward.

Parent opening now compares native file identity before/after opening and rename
flushes use the same pinned handles. This is a source-level correction; an actual
concurrent directory-replacement stress test has not established race coverage.
Partial-move rollback/restoration and native Windows junction/rename behavior
remain open; these changes have not yet received a correction review.


## Whole-product challenge and correction, 2026-10-09

The user reopened all design choices. A fresh actual `claude-opus-5-5`
consultation completed 84 read-only tool calls with no execution or permission
errors. Requested effort: `xhigh`; effective effort: unexposed. The 630-file
snapshot remained unchanged. This is design advice, not full-delta code approval.

Accepted: one Go implementation; remove Nix as well as chezmoi; private verified
archives for portable tools; deliberate native providers without fallback
cascades; Linux graph for WSL2; installer-owned provisioning and read-only checks;
remove unsolicited global side effects. The archive provider avoids a separate
POSIX package plane that could not cover Windows in the first place.

Rejected: deleting all recovery evidence (idempotence alone cannot distinguish a
moved user baseline from a new file), blanket retention of removable dependencies
(conflicts with requested removal), removing the explicit machine approval
protocol (the normal UI already has no flag maze), and deleting review history
(conflicts with append-only documentation instructions). Retain the minimum
mechanisms needed to prove the product contract, not the intermediate Nix design.
See the current decision at the top of the implementation plan.

In-progress recovery corrections preserve and disclose modified files during
finalization; completed history no longer requires the current manifest or a
separate baseline document. Abandon changes in-progress receipt status to
needs-action while preserving ownership, artifacts and the archived transaction.
The existing abandon test compared every receipt field; it now explicitly proves
that status transition and still compares all provenance fields. Source-loss,
changed-recovery-file and removed-adopted-target regressions remain required.
No new complete-product lifecycle or release is claimed by these corrections.


### Archive, WSL and native directory-link checkpoint

The new archive provider verifies HTTPS/SHA-256 before bounded extraction and
owns private content-versioned payloads. Real Starship 1.26.0 execution, check,
unchanged-pin update and removal pass locally on macOS arm64. Race-tested
fixtures cover shared removal in both orders, changed-pin update, reinstall,
unsafe extraction, edited bytes and actual process death during download,
publication and removal. A failing-before repeat-update case justified treating
healthy, unchanged desired-input fingerprints as a no-op. Existing command paths
stay stable across version changes; a deleted owned directory link repairs from
the already verified payload. Version directory redirects are retained.

WSL/WSLGUI/Distro are removed from the unreleased engine's persisted Context.
WSL2 is Linux; GUI intent does not depend on DISPLAY. The host ownership scope is
rejected. Renderer fonts remain dependencies; CLI configs no longer pull fonts.
Clipboard remains a Neovim requirement but no longer blocks plugin sync.
Legacy released scripts still implement their previous policy until cutover.

Windows Neovim configuration now selects a native junction. Go's EvalSymlinks
cannot resolve Windows mount-point parents, so the native path resolver uses
GetFinalPathNameByHandleW; link snapshots read the junction itself. Tests cover
adopting/restoring a junction whose referent exceeds the snapshot byte limit,
live lockfile writes, and an independently redirected configuration folder.
Windows/Linux ARM64 test binaries cross-compiled before the subsequent stable
entrypoint changes; that is compilation evidence, not native execution proof.
The hosted matrix now includes Linux ARM64 and a real archive lifecycle step;
those new jobs have not run yet. Public bootstrap/profile integration, the full
catalog, migration and final independent review remain unfinished.


The first full gate reached Renovate validation and failed because adding the
Linux ARM64 runner changes its exact extracted Ubuntu count from one to two.
The reviewed inventory now records both real runner entries; the check itself
is unchanged. Windows native junction operations also use extended-length API
paths, with a long-path creation/resolution/removal test awaiting native CI.

The corrected full local `make ci` gate passes with `DOTFILES_TEST_ARCHIVES=1`
and `DOTFILES_TEST_NIX_PRIVATE=1`. The engine race suite and real macOS archive
lifecycle passed; local PowerShell checks are explicitly unavailable. This is
a component checkpoint for hosted verification, not completed public delivery.


### Connected command and scoped profiles, 2026-10-09

At `11857a1`, hosted macOS and both Linux architectures passed real archive
installation/execution/removal. Windows stopped at a fixture declaration error:
a Windows-only junction mapping retained an all-platform resource declaration.
The fixture now declares the complete graph Windows-only; production validation
is unchanged. Native Windows archive execution still awaits the next head.

The new executable now routes catalog resources through the native archive,
configuration and scoped profile providers. Starship's closure no longer pulls
Nix or chezmoi, and private archive bindings do not enter legacy Nix batches.
A compiled-command test passes machine preview/approval through install, update,
repair, check and removal on macOS arm64. A fresh Bash invocation proves the
private command, configuration path and initialized prompt function. A test first
ran under TERM=dumb, where Starship correctly rejected prompt initialization; the
fixture now supplies a normal terminal type and requires the actual prompt hook.
Binary execution alone was insufficient proof of a usable prompt.

Profile ownership covers only marked bytes. Tests prove outside user edits,
UTF-8/UTF-16 and original newline preservation, unchanged-update idempotence,
retention of edited managed blocks, and saved partial-stage/staged/moved/published
recovery. Matching bytes from another writer cannot prove publication while our
staged file still exists. These are real filesystem saved-shape tests; actual
profile process-death, native metadata and multi-platform coverage remain open.
No public bootstrap, complete catalog, migration or final code-review approval is
claimed. The checkpoint remains within PR87 and its single implementation commit.

A failing-before regression showed completed profile history still depended on
its current target catalog. Independent saved-journal validation now seals and
cleans historical intent without re-resolving current targets; affected-resource
observation still requires its declared mapping. The focused regression and
profile suite pass afterward. The revised full local `make ci` gate also passes,
including native archive/compiled-command tests and all Go race tests. Windows
cross-compilation passes, but neither native Windows execution nor independent
review of the new profile writer has completed at this checkpoint.

### Archive/profile checkpoint review, 2026-10-09

Actual `claude-opus-5-5` completed 111 read-only calls with no API, guard or
permission errors. Requested effort: `xhigh`; effective effort: unexposed.
All 652 snapshot and live source hashes were unchanged through completion.
The reviewer read the full delta artifacts and replacement provider/command code;
it expressly excluded obsolete Nix, terminal internals and legacy scripts. This
is checkpoint review, not final full-product approval.

Accepted findings requiring correction:

- High: the native fixture inherited the caller's ZDOTDIR and could overwrite
  their real `.zshrc`. A non-mutating target assertion failed before correction.
  Folder discovery now passes ZDOTDIR explicitly; a constructor with disposable
  folders cannot inherit it. The native fixture also binds ZDOTDIR and validates
  every profile destination before writing. Focused regression passes afterward.
- Medium: profile paths must come from recorded ownership for cleanup; session
  changes and one edited block must not strand all other owned blocks.
- Medium: interrupted profile publication needs explicit source-independent
  restoration, including a recreated target; forward-only recovery can deadlock.
- Medium: abandoned/partly cleaned archive versions can block reinstall. Stage
  by operation, publish no-replace, retire separately, preserve changed contents.
- Medium: damaged private tools need an explicit preserve-and-reinstall action.
- Medium: invalid historical archive/profile records must not poison unrelated
  transactions. Sealed history needs a minimal independent reader.
- Medium: whole zsh profile configuration conflicts with scoped integration.
  Use one scoped shell loader and private tool snippets; validate target overlap.
- Medium: Windows execution policy must be checked for actual profile consumers;
  Server runner defaults do not prove Windows client startup behavior.

Also open: PowerShell Unicode quoting and encoding, generated Invoke-Expression
versus the repository invariant, prompt initialization failure propagation,
profile metadata/readonly handling, disposal of completed full-profile journal
content, recovery-menu error isolation, termination signals, POSIX command modes,
and created-empty-directory cleanup. The claimed Windows CopyFile ACL loss needs
revalidation against the documented Windows 8+ API behavior and native assertions;
it is not accepted solely on the reviewer's statement.

Hosted evidence became available after the review began: commit `6f84209` passed
all 27 checks. Installer engine run `37957696777` passed actual compiled-command
Starship install/update/repair/check/remove on macOS, Windows and both Linux
architectures, plus WSL2 host startup. These unchanged-pin update/repair steps do
not prove changed-pin upgrade or damaged-tool repair. Windows prompt-function,
5.1/client-policy, redirected-folder and internal-publication death coverage remain
open. Future bundles need explicit command/environment and source-tree provenance;
historical filenames alone are insufficient failing-before evidence.

### Retire the unreleased Nix prototype, 2026-10-09

The writer removed the unused Go Nix adapter, privileged workers, activation
leases, batch planner/executor/schema and dedicated Nix projection/native fixtures.
These files did not exist on main and were never connected to the public command.
The six existing source/workflow files modified only for this prototype now match
main. Existing public setup and its meaningful Nix checks remain in place until
full replacement verification. This is the authorized architecture change, not
removal of a failing supported-runtime test.

Generic inventory and completed-update/removal regressions were ported from the
batch fixture to individual real-file operations. A focused regression proved
that protected inventory must be rechecked before each ordinary mutation as well
as at finalization; that boundary no longer depends on Nix batching. Removed
prototype batch fields fail strict decoding and leave the input state untouched.
Historical review evidence remains above; active contributor docs now describe
the surviving implementation instead of retired worker internals.

The retirement checkpoint passes the complete local `make ci` gate with
`DOTFILES_TEST_ARCHIVES=1` and `GOTOOLCHAIN=local`, using pinned Go 1.27.1.
The captured manifest records every source hash, command, selected environment,
starting HEAD/worktree status and zero exit status; all source hashes stayed
unchanged during execution. Native macOS Starship/command lifecycle and Go race
checks passed. Windows/Linux verification of this deletion checkpoint and the
remaining review corrections are still outstanding. The current Windows metadata
claim remains unresolved: documented security resource attributes are not proof
that all DACL/owner semantics survive; native assertions must settle that question.

### Profile restoration and process-death correction, 2026-10-09

The profile conflict regression failed at all nine install/update/remove and
staged/moved/published cuts because no restoration provider existed. Scoped
profiles now share configuration recovery's no-overwrite inverse moves. Saved
journals and physical paths drive restoration without the current source, targets
or catalog. The native router uses the recorded provider identity, then the
provider independently validates the saved journal. Recreated profiles survive
at disclosed paths, original files return where available, stale approvals mutate
nothing, and another installation succeeds after restoration.

Actual child processes now die after the first original profile has physically
moved while later targets remain unpublished, without production test hooks or
provider wrappers. Six native macOS race cases pass: install, update and removal,
each with resume or restoration. Each asserts the actual incomplete persisted cut
after death. Restoration retains a recreated profile, and another install/remove
cycle succeeds. Provider-boundary and saved-shape tests also cover interruption
after restoration but before the engine records completion.

### Historical-evidence, permissions and menu corrections, 2026-10-09

Failing-before regressions reproduced corrupt old configuration/profile JSON
blocking new installation, an obsolete archive intent blocking removal, completed
journals retaining complete personal profiles, a restoration error hiding the
recovery menu, and read-only staging failing with permission denied. Corrections
retain/disclose damaged unrelated evidence, gate completion on active receipts,
inspect only existing archive link workspaces, drop full staged profile content
from JSON, preserve other recovery choices, and restore staging permissions.
Uncommitted staging is recopied even when its bytes match: an interrupted writer
may not have restored permissions. A saved-shape regression covers that case.

The complete local `make ci` gate passed with `DOTFILES_TEST_ARCHIVES=1`,
`GOTOOLCHAIN=local` and pinned Go 1.27.1, with an unchanged source-hash manifest.
Core race/native archive and command tests passed in 48.845s; terminal tests passed
in 13.951s. The clean committed `1a4f2a6` then passed the verbose focused recovery
suite in 22.626s. Local PowerShell checks remain unavailable. Current named profile
restoration tests fail on archived `61aa469` at all nine cuts; current named config
restoration tests fail on archived `4655c01` at all twelve cuts. Only test fixtures
and pure path enumeration were supplied to those baselines, with commands, test
hashes and behavioral failure output recorded outside Git. These replace the old
filename-only failing-before evidence called out by Opus.

Retirement checkpoint `61aa469` passed all 27 hosted checks. At `1a4f2a6`,
installer-engine run `37965587528` passed on macOS and both Linux architectures,
including internal profile process death; the WSL2 host check passed. Windows
failed the new permission assertion as recorded below. The correction review,
public bootstrap, remaining tool providers and release migration remain open.

### Native Windows ACL finding confirmed, 2026-10-09

`TestProfilePublicationPreservesNativeWindowsPermissions` failed in both writable
and read-only cases on the hosted Windows runner: CopyFileW replaced the protected
explicit DACL with the parent's inherited entries. This accepts the previously
disputed review finding. Microsoft's
[CopyFileW documentation](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-copyfilew)
only promises security resource attributes, which are distinct from the ordinary
DACL. Native failure output is retained outside Git.

The correction uses GetNamedSecurityInfo/SetNamedSecurityInfo to copy the original
DACL and protection mode, copying owner/group only when different, then verifies
the descriptor before publication. It requests no new privilege and refuses
publication when preservation cannot be established. The native test covers both
protected and inherited-plus-explicit ACLs, with and without read-only mode,
through install/update/removal. SetKernelObjectSecurity was rejected because
Microsoft explicitly excludes filesystem objects from its recommended use.
Corrected native execution and independent review remain pending.

### Review-ledger preservation correction, 2026-10-09

A writer documentation script accidentally wrote another document's buffered text
to this ledger after appending. The full canonical history was restored byte for
byte from `61aa469`, followed by the current session entries above. Code was not
affected. Future append operations must use a dedicated ledger variable and must
verify that the result starts with the complete previous ledger before committing.

The Windows ACL correction passes cross-compilation and the complete local
`make ci` gate. POSIX Go tests were valid cache hits because the production change
is Windows-only. Only the ledger restoration above changed during the gate; the
manifest records that exact path and confirms unchanged implementation/test files.
Native Windows correction verification is still required.

Hosted correction run `37966621771` at `2d416c4` passed the four custom Windows
ACL cases, but ordinary profile operations failed descriptor equality after an
unnecessary SetNamedSecurityInfo call. The next correction first compares the
actual descriptors and leaves an exact copy unchanged; custom differences still
require native copying and exact readback. Empty descriptor serialization fails
closed. The native lifecycle assertion now includes default ACLs with and without
read-only mode. Native rerun and independent review are pending.

### Recovery correction review, 2026-10-09

Actual Opus 5.5 completed 77 read-only calls against `db56e4e`; requested
effort was xhigh and effective effort was unexposed. Exit status was zero, with
no API errors, permission denials, guard violations or model substitution. All
601 snapshot and live-source hashes were checked unchanged after completion.
An earlier attempt stopped when a large tool result led the client outside the
supplied snapshot; that incomplete attempt is not review approval. The fresh
review used bounded reads under the same restrictions.

The reviewer accepted the current failing-before proofs for profile/configuration
restoration, historical evidence, private profile content, recovery menus and
read-only publication. Native run `37967630180` confirms all six Windows ACL
shapes and six actual profile process-death cases: these tests have no skip
paths. All 27 hosted checks now pass on `db56e4e`. This remains an unfinished
product, not release approval.

New accepted corrections are in progress: archive finalization must isolate
unreadable unrelated state and defer locked-file cleanup (Medium); restoring a
profile must keep later edits in the active profile while reversing only its
owned block (Medium); torn staging bytes must not require text parsing; an
uninspectable recovery must remain retryable; damaged active journals need their
paths and recovery locations disclosed; recovery wording must match the action.
POSIX owner/group preservation and additional native Windows ACL shapes require
reproduction. The Windows AI-control-flag concern is a fail-closed compatibility
candidate, not proved data loss; exact native fixtures must settle it.

The previous open archive reinstall/repair, per-file profile ownership, shell
loader overlap, PowerShell policy/encoding, signals, executable-mode, directory
cleanup, bootstrap, catalog and migration work remains. Additional evidence
requested includes current-test failing-before runs for partial metadata and
actual restoration process death, explicit Windows verbose output, and focused
active-journal/overlap/provider-identity boundary tests.


### Recovery corrections and new boundary finding, 2026-10-09

- **M2 corrected locally:** profile restoration now reverses the managed block
  through the existing durable profile writer. Later personal edits stay active;
  uncertain edited blocks are preserved and disclosed. Physical inverse moves
  remain for unchanged/missing profiles. Four original-file/new-file cases failed
  before and pass after. The nine older conflict assertions were corrected to
  require active personal-byte preservation instead of whole-file displacement.
- **New core finding, fixed locally:** forward retry during an incomplete
  restoration replaced the original transaction and lost its restoration action.
  All four saved inverse-publication cuts reproduced this. Pending in-flight
  recovery without a forward-resume token now rejects retry before any write.
  The same tests pass, plus actual process death during eight-profile restoration.
- **L1/L3/L4/L5 corrected locally:** preserve torn staging without parsing it;
  return inspect errors instead of sealing incomplete restoration; disclose active
  journal/baseline/current-workspace locations; use file-neutral restoration copy
  and tell users to retry restoration after it starts. Torn UTF-8/UTF-16/NUL and
  temporarily redirected recovery fixtures reproduced the first two defects.
- **L6 confirmed and fixed:** macOS cp silently changed root ownership while
  copying `/etc/protocols`; uid/gid are now checked before publication. The first
  `/usr/bin/true` fixture was rejected because restricted file flags caused an
  unrelated copy failure. The corrected public protocol-table fixture fails before
  and passes after. Originals are never changed by this test.
- **M1 corrected locally:** unrelated corrupt archive selections, mismatched links
  and unreadable version inventories are preserved/disclosed during finalization.
  Locked obsolete cleanup is deferred. Active interrupted link publication still
  blocks abandonment. Five of six apply/abandon fixtures failed before; all six
  now pass. Interrupted cleanup/reinstall and damaged-package replacement remain
  separate open archive lifecycle findings.
- Added passing boundary cases for unsealed history with valid headers/invalid
  bodies, required just-completed journals, overlapping profile targets and saved
  provider-identity disagreement. Hosted Go race output now names each test.
- Existing `db56e4e` native Windows evidence proves all six tested ACL shapes. L2
  additional ACL-shape compatibility remains unverified; no permission comparison
  was weakened. Current-source full gate, native execution and review are pending.

- **Correction gate:** pinned Go 1.27.1, `GOTOOLCHAIN=local`,
  `DOTFILES_TEST_ARCHIVES=1`; uncached verbose `go -C installer test -race
  -count=1 -v ./...`, full `make ci`, and `GOOS=windows GOARCH=amd64
  CGO_ENABLED=0 go -C installer test -c` all passed. Every source hash remained
  unchanged during each command. Only these documentation receipts follow the gate.
- **Additional before proof:** the current named partial-metadata test against
  actual archived `61aa469` fails during removal after reusing staging with wrong
  permissions. The new actual-restoration-death test, with only the original
  `resource_resume.go` restored from `db56e4e`, fails because forward retry loses
  restoration eligibility. Neither failure is a compile error. Full source/test
  hashes, exact commands and logs are retained in the local review evidence.
- Pending: native execution of this correction and independent Opus review;
  previously declared product work and additional Windows ACL shapes remain open.


### Native Windows directory classification and required proof, 2026-10-09

Run `37974199870` at `fc1c1bc` passed macOS26, Ubuntu26.04 amd64/arm64 and WSL2
startup. Windows ran the full verbose race suite: profile restoration, all seven
actual process-death cases, all six ACL lifecycle cases and new recovery tests
passed. Two archive disclosure cases failed because Go ReadDir on a regular file
returned an error classified as not-found. Preserved bytes survived, but the
reported preserved-path list omitted them. A shared plain-directory reader now
checks the directory entry before callers classify absent state. The same
classification sites in profile/configuration history were corrected.

The multi-location check also found that enumeration cannot detect a deleted
just-completed journal. Both profile and config tests reproduce successful
finalization after the journal was moved away at the persistence boundary.
Finalization now explicitly requires completed-operation evidence before scanning
history. Unstarted in-progress operations retain their existing abandonment path.
The new source still needs full-gate and native verification. Opus has not started
reviewing this correction; the prepared snapshot is stale and will be rebuilt.

The directory/proof correction now passes the focused regression group, the full
uncached verbose Go race suite with real archive/command execution, full `make ci`
and Windows cross-compilation. Source hashes remained unchanged through each
command. Native rerun and fresh Opus review remain pending; only these Markdown
verification receipts follow the local gate.

## 2026-10-09: scoped-restoration correction review at 48e6383

Actual `claude-opus-5-5` completed 83 read-only calls, exit 0, terminal success,
with no model, API, guard or permission errors. Requested effort was `xhigh`;
effective effort was unexposed. All 607 source hashes remained unchanged through
review. The local uncached race suite, full `make ci` and Windows cross-compilation
evidence was accepted; all 27 hosted checks passed at the same head. Native run
`37975587340` includes named correction tests and real Starship archive/command
lifecycles on macOS, Windows and both Linux architectures. WSL2 remains a host
startup probe. The checkpoint was not approved.

- N1, Medium, accepted: scoped restoration can become unrecoverable after edits
  between attempts, invalid profile bytes/markers, or changing direction before
  checking inspectability. New regression cases fail at the controller boundary
  before correction; implementation and broader verification are in progress.
- N2, Medium, accepted: a recreated target in the moved-but-unpublished cut must
  be preserved separately while the original profile returns to its active path.
  The prior changed conflict expectation was incorrect for that cut. New
  install/update/removal regressions reproduce it before correction.
- N3, Low, accepted/open: deferred archive deletion and retained personal files
  need distinct accounting; partial cleanup currently prevents eventual cleanup.
- N4, Low, accepted/open: refused POSIX ownership copies accumulate; the public
  ownership test fixture may be absent in minimal images.
- N5, Low, accepted/open: restoration menu/diagnostic inconsistencies remain,
  including missing configuration-journal paths and generic retry instructions.
- Prior Windows L2 remains open: additional valid ACL shapes and ordinary-account
  permission behavior still need native evidence.

Evidence: `profile-restore-edit-round-corrected-before.log/.json` records the
current new test hash and actual behavioral failures against unchanged 48e6383
production code. The earlier `profile-restore-edit-round-before` run used a
ready-only helper for needs-action assertions; the corrected run replaces that
helper with an explicit controller preview/apply and is the authoritative proof.

Separate design recommendation: Opus recommends fresh operation-specific archive
generations over replacing pin-named directories. Reconcile this with the existing
entrypoint and journal implementation: reuse only a proved unchanged active
generation for pointer repair; preserve explicitly replaced edited generations;
track deferred deletion separately; refuse old unreleased metadata by schema.
This is a design recommendation, not implementation approval.

Static dependency audit, implementation still pending: checked upstream archive
SHA-256 values and inspected Windows PE imports and Linux ELF interpreters without
executing those newly inspected binaries. lsd, hyperfine, Neovim and Tree-sitter
import unbundled `vcruntime140.dll`; the shared runtime must be an automatic
dependency, not a checkbox. Tree-sitter Linux and hyperfine Linux arm64 request
glibc loaders, so their upstream binaries cannot be treated as Alpine artifacts.
GitHub runner preinstalls and Ubuntu success do not prove these prerequisites.

### Local correction follow-up

The new N1/N2 regressions and broader profile/restoration/recovery race tests pass
locally. The actual process-death fixture again uses an unrelated recreated file;
it records whether the first target was unpublished at death and checks the
appropriate physical-return or published-edit preservation result. This restores
the missing fixture without assuming a scheduling-dependent publication phase.
The retry guard test now compares saved state bytes as well as recovery files.

N4's destination-leftover regression failed before removal of the refused copy.
N5's menu and configuration-inspection/diagnostic regressions failed before their
corrections. The menu now retains restoration as the only mutation direction
until the core transaction is archived, including the provider-completed cut.
`Observation.Restoring` is optional; existing saved observations omit it and the
provider journal establishes its current value. Archive deferred-cleanup N3 and
Windows ACL L2 remain open. Full gate and native correction evidence are pending.

### Completed local verification for the restoration-edit correction

`restore-edits-final-tests-before.log/.json` replays all six final changed test
files against archived `48e6383` production code. Eleven named test groups reach
behavioral assertions and fail on the accepted N1/N2/N4/N5 cases; the metadata
records exact test hashes and command. This replaces intermediate fixture versions
as the correction review's matching before evidence.

`restore-edits-race.log/.json`, `restore-edits-full-ci.log/.json` and
`restore-edits-windows-compile.log/.json` all exit 0, with source hashes unchanged
through each command. The first is the uncached verbose complete Go race suite
with `DOTFILES_TEST_ARCHIVES=1`, including real archive and compiled-command
lifecycle. The second is the repository-required `make ci`; its local PowerShell
coverage remains unavailable and must execute natively on Windows. The third
is compilation only. Native correction tests and independent review are pending;
this record is not a release or hosted pass.

## 2026-10-09: restoration-edit review at c4ec1a5

Actual `claude-opus-5-5` completed 61 read-only calls, exit 0, terminal success,
with no API, model, guard or permission errors. Requested effort was `xhigh`;
effective effort was unexposed. All 609 source/snapshot hashes were unchanged
at completion. All 27 checks passed at this exact one-commit PR head. Native
engine run `37981909375` passed on macOS, Windows and Linux amd64/arm64, with
WSL2 startup still only a host probe.

The reviewer found no Medium-or-higher defect in this correction and accepted
N1, N2, N5 and the POSIX N4 fix. Four Low follow-ups are accepted: F1, failed
metadata-copy leftovers on Windows and other POSIX error paths; F2, missing
original-profile recovery diagnostics; F3, a permission-refused inverse copy
can leave restoration waiting for manual permission repair; F4, configuration
journals with valid JSON but invalid identity omit their diagnostic paths.
These are not release approval; N3, Windows ACL L2 and the product gaps remain.

Evidence follow-ups: force an actually published first profile before killing
the child; test the published returned-original case and missing inverse stage;
replay the current retry helper with only its guard reverted; provide hashes of
the archived before-production files and put current snapshot/check receipts
inside the next review's evidence directory. The reviewer saw native engine
logs, not the separate all-27-check receipt, so that broader claim remains a
writer-verified result.

Next cleanup batch also investigates configuration's `Discarding` retry flag:
source inspection shows it bypassing content checks after cleanup began. This
is a candidate data-preservation defect, not yet runtime-reproduced. The broader
Linux ELF audit confirms glibc loaders in Neovim on both architectures and
ripgrep on arm64, in addition to Tree-sitter and arm64 hyperfine. The existing
Windows installer separately provides Zig for LuaSnip and VS Build Tools for
the MSVC toolchain; the new dependency inventory must preserve that distinction.

### Local diagnostic and published-cut follow-up

F2/F4 reach failing behavioral assertions against the previous production code
and pass after the diagnostic changes. Missing originals retain/disclose the
active profile; valid JSON with invalid configuration intent names its journal
and baseline. G2/G4 now cover the published returned-original and missing-stage
cuts. The strengthened retry test fails after removing only the retry guard
(and its unused import), proving state changed despite the refused action;
the earlier compile-error attempt is retained separately, not counted as proof.

For G1, watching the second physical backup rename forces the first profile to
have published. This exposed a new gap after restoration correctly preserved a
recreated first profile: other targets still had managed blocks, so the composite
resource required explicit adoption, but ProfileDriver offered no such choice.
The new maintenance-menu regression also fails before the correction.

Decision: reuse the existing explicit replacement menu and journal action rather
than update ownership to match unapproved files or add another recovery schema.
Only an owned profile with a validated first baseline qualifies. Replacement
retains the current files under their existing operation workspaces, reports
their paths, and keeps the original baseline. Completion uses the saved `adopt`
action as preservation intent; no additional journal field is needed. The menu
test covers declining, archiving that unfinished intent, then separately approving
replacement. It also proves repeated replacement, update/removal/reinstall and
retained saved-file references. Unowned, missing, relocated, malformed and changed
baselines are refused. The actual-kill matrix now covers 16 install/update/adopt/
remove cases, including both first-target publication states; all pass locally.
F1/F3 and the prior product/cleanup/ACL gaps remain open. The complete gate,
hosted verification and independent review of this follow-up are pending.

## 2026-10-09: profile replacement checkpoint review and missing originals

Actual `claude-opus-5-5` completed 65 read-only calls, exit 0, terminal success,
with no API, guard or permission errors. Requested effort was `xhigh`; effective
effort was unexposed. All 610 source and snapshot hashes remained unchanged.
The reviewer verified that `profile-adoption-race.log/.json`,
`profile-adoption-full-ci.log/.json` and Windows compilation evidence all passed
against those hashes. It also checked the matching before assertions and the
previous c4ec1a5 all-27-check receipt. Those hosted checks do not cover this local
follow-up. The replacement design was accepted as necessary and as reusing the
existing menu, receipt baseline, operation action and preservation report.

Accepted findings: A (Medium), the physical inverse could move an unedited active
publication aside after its required original disappeared, leaving no active
file; a profile install could then incorrectly report ready. B (Low), absent
removal output and unpublished staging could hide the missing-original path.
The configuration caller of the same inverse is also affected. F2's earlier
claim was therefore too broad: its test covered edited active profiles only.
The isolated removal experiment with an original personal file passed, but did
not establish an absent-output removal. A subsequent fixture explicitly starting
from an installer-created profile reproduced that missing diagnostic. This was a
fixture limitation, not evidence for rejecting B.

`missing-original-before.log/.json` records eleven failing cases with exact
production/test hashes. Published cases assert active-file preservation before
status, so their failures prove displacement, not only weak diagnostics. Moved
profile cases also reproduce false readiness or missing paths. The shared inverse
now receives its recorded pre-operation snapshot and refuses to move the active
target if that required original has no saved copy. Profile observation marks a
missing personal profile plus missing backup unhealthy even when its managed
block fingerprint is empty. Profile and configuration recovery messages name
their recorded targets. The new regressions, returned-original edit checks and
broader configuration-restoration cases pass under the race detector.

Review evidence follow-ups: the actual-kill fixture now watches the in-flight
receipt's operation ID, excluding earlier retained adoption workspaces. The
20-case matrix includes removal after adoption; resumed adoption asserts every
saved original's bytes and disclosure, then checks those bytes and references
after subsequent removal. The expanded matrix and actual scoped-restoration
death test pass locally. Damaged-baseline checks now assert the baseline path.
G3's guard-reverted before-run remains an intermediate-tree precision limitation;
its relevant guard/helper matched, but a final-tree replay is still required.

The reviewer also confirmed `Discarding`'s unsafe retry from source: cleanup can
ignore both changed content and a previously saved `Preserved` path. Runtime
reproduction and correction remain required, alongside F1/F3, N3 and L2. The
latest missing-original correction still needs its full gate, native execution
and independent correction review. No release or completion claim is made.


## 2026-10-10 consolidated implementation, first corrections

The total-complexity consultation completed through actual `claude-opus-5-5`:
78 read-only calls, exit 0, no API/model/path/permission errors. xhigh was requested;
effective effort was unexposed. Source and PR #88 snapshot hashes stayed unchanged.
The consolidated plan records accepted decisions. Rejected shortcuts: blanket
native-dependency retention, dry-run-only removal authority, dropped Linux/GUI
coverage, v0.4.4-only migration, and directory-name-only cleanup proof.

PR #88 outcomes are being consolidated in the existing delivery worktree. Its
quiet login file and regression are included. Its publisher patch is superseded
by #87's OS lock with in-process publication and child-death tests: reverting to
a mkdir/pid lock would reintroduce the already documented orphan-writer race.
Hyperfine's unrounded boundary is retained, with #87's stronger schema validation
and process-boundary tests. A new 80.1 ms case failed before and passes after.
The quiet login test and all 19 performance cases pass locally.

Configuration `Discarding`: both saved-personal-file deletion and edits-after-cut
were reproduced against unchanged baseline code, then pass after the correction.
Cleanup now saves bounded entry hashes/modes and removes entries individually;
missing entries after its own partial deletion are valid, and new/changed entries
and existing preserved paths survive. Partial-tree and empty-workspace recovery,
redirected-directory, mode-change and malformed-list tests pass under the race
detector. The old boolean is decoded but grants no deletion authority.

Archive replacement: edited executable, missing executable and missing payload
all lacked a replacement choice before. Each now passes explicit replacement,
fresh generation, disclosure, update, removal and reinstall checks. A malformed
active operation previously returned both Unknown and Present/completed proof;
it now produces a valid resource-local unknown observation. Existing archive
lifecycle and actual process-death tests pass with operation-named schema-2
generations. Pointer-only repair still avoids a download. Archive cleanup adopts
the same entry primitive with persisted generation provenance; native locked-file
and partial-deletion evidence remains required.

Baseline: restored the exact official Go 1.27.1 darwin/arm64 archive after detecting
missing standard-library files. The uncached race suite passed. The full `make ci`
failed in the crash fixture's concurrent state observer, which received the
reader's documented changed-inode/retry result. The observer now retries only that
specific result within its existing deadline; it still requires the actual kill,
publication cut and preservation assertions. A new full gate is required.
No release approval or complete-catalog claim is made by these focused results.


### Additional consolidation verification and metadata correction

The complete uncached Go race suite passes after the cleanup/generation changes
(`consolidated-lifecycle-race.log`). The saved partial-generation tests also prove
cleanup can finish after version.json has already disappeared, while preserving
later personal files. Previously disclosed generations remain preserved even
when their bytes later match an old release again.

A direct POSIX metadata-copy regression reproduced overwriting a pre-existing
staging destination. Exclusive reservation now rejects it, with failing-before
and passing-after evidence. Failed-copy cleanup is restricted to the created
inode; Windows also removes its private read-only copy after post-copy metadata
failure. Linux uses native xattr/ACL copying and verification rather than GNU cp
flags, removing the Alpine incompatibility. New Linux ACL/xattr and restricted
Windows-token lifecycle tests compile for their native platforms; native execution
is still required. F3 remains open: a permission-refused inverse currently needs
manual permission repair, with its original recovery direction retained.

A declared non-executable POSIX archive command was accepted before and is now
rejected. The ZIP extraction fixture explicitly declares executable mode, matching
its intended successful-command contract; no checker or runtime check was waived.
A Windows locked-file test requires cleanup to stay retryable and finish after
unlocking; it is compiled but still awaits native execution.

### Consolidation local gate, 2026-10-10

`consolidated-lifecycle-metadata-race.log` and
`consolidated-lifecycle-metadata-full-ci.log` both pass with
`DOTFILES_TEST_ARCHIVES=1`, Go 1.27.1 and unchanged recorded source hashes.
Windows cross-compilation also passes. These results include the metadata-copy
corrections, archive generations and cleanup, expanded actual-kill recovery,
quiet login profile and fractional performance budgets. Native Linux/Windows
execution of the newly added fixtures and final independent review remain required.
This checkpoint is not a completed full-catalog delivery.

### Hosted consolidation result and shell composition work

At `e87bb8b`, native Linux amd64/arm64 and macOS engine lanes pass. Windows's
new restricted-token fixture cannot launch from Go's administrator-accessible
build directory (`Access is denied`); the actual ordinary-token lifecycle has
not run. The fixture now copies its executable into a disposable directory with
an explicit current-user ACL and uses that directory for its child/temp paths.
It still denies the Administrators SID and asserts the child is not elevated;
no permission assertion is waived. Native re-execution is required.

The next implementation connects fixed private shell feature files behind one
shared personal-profile owner. zsh/PowerShell core configuration no longer competes
with Starship over personal files. Optional feature selection controls the files;
merely installing fzf as another feature's prerequisite does not select its
keybindings. Quiet PATH setup precedes interactive detection and honors actual
ZDOTDIR/Documents paths. Verified fzf, fd, lsd and zoxide archive recipes are
connected; Windows lsd declares its separate Visual C++ runtime requirement.
Full native feature composition and the remaining providers are still in progress.

The fixed private-feature-file prototype was simplified further before delivery:
retaining personal edits in an unselected fragment must not keep its hook active.
One combined initialization block is now generated directly from the request's
resolved graph. A per-request adapter keeps simultaneous previews independent;
the planner, receipts and file publication/recovery remain unchanged. This removes
per-feature files/receipts and filesystem-presence selection rules. The actual
combined Starship/fzf/zoxide/lsd install/update/repair/selective/full-removal
lifecycle passes locally. The competing-owner regression fails against the
previous commit and passes against the combined implementation.

Pinned zsh plugins and PSFzf use the existing verified archive publisher, with
explicit required non-command entry files. No Git child publisher or new module
installer is needed for these payloads. Native plugin execution remains required.

The concurrent first-preview regression exposed `Catalog.Validate` rebuilding a
shared mutable lookup map. It now validates with a local set and uses linear
lookup over this small fixed catalog, removing shared cache state and the race
rather than adding locking. The failing race trace is recorded in
`combined-shell-race.log`; passing re-execution remains required. PowerShell
composition now uses the runtime path separator and asserts the selected private
executable, preventing an identically versioned host command from masking a PATH
error. Preserved PowerShell text retains its UTF-8 BOM; the removal assertion now
requires that encoding as well as the exact outside bytes.

The corrected concurrent-preview race test and real Bash/PowerShell composition
lifecycle now pass. PowerShell tests assert the private executable path and retain
BOM plus personal edits through removal. macOS zsh uses its existing operating-
system shell as a reused resource; dotfiles cannot own/remove `/bin/zsh`.
The real pinned zsh plugin archive exposed Git's global PAX comment header being
mistaken for a file. A focused regression fails before correction; the extractor
now accepts comment-only global metadata and rejects unsupported global file
attributes, following Go's documented non-persistence of global PAX state.

The source-archive regression and real macOS zsh composition now pass. The zsh
fixture uses a real PTY and rejects startup option diagnostics; a pipe-only
interactive invocation could not exercise ZLE correctly. It verifies both Tab and
Ctrl-R ownership before/after fzf deselection, actual private command paths, the
last-running personal hook, dependency retention and final private cleanup.
The current status inventory is condensed to current decisions, roadmap and
verified gaps; its prior committed text remains linked for historical evidence.

PR #88 is closed after its changes were incorporated in the pushed #87 checkpoint;
#87 is the sole open PR and retains one commit. Its description now reflects the
Nix-free design and actual incomplete state. The new batch's full local gate first
stopped on indentation inside generated shell string literals; this is corrected
without checker exclusions. The gate had not reached its Go/native tests.

### Unified shell checkpoint verification (2026-10-10)

The complete `make ci` gate passes with `DOTFILES_TEST_ARCHIVES=1`, pinned Go
1.27.1 and native macOS PowerShell 7.6.3. Windows amd64 test compilation also
passes. Before/after source hashes are identical for both checks; evidence is
`unified-shell-verified-full-ci.{log,json}` and
`unified-shell-verified-windows-compile.{log,json}` in the delivery artifact root.
This includes the race regression, real package/shell lifecycles and PTY zsh
composition. Native Windows restricted-token execution remains pending hosted
verification; cross-compilation does not resolve that evidence gap.

### Portable tool coverage and hosted fixture corrections (2026-10-10)

Connected 28 already researched, rehashed upstream artifacts for Neovim 0.12.5,
ripgrep 15.1.0, GitHub CLI 2.96.0, lazygit 0.66.0, jq 1.8.1, Tree-sitter 0.27.0
and Hyperfine 1.20.0. Each resource uses the existing archive provider and shared
shell attachment. Windows runtime dependencies stay explicit in the graph;
preinstalled runner runtimes are not missing-prerequisite proof. Linux glibc
selection still needs the native platform-discovery/provider work before cutover.
All seven actual macOS install/version/check/remove lifecycles pass in
`seven-portable-tools-native.log`; Neovim also locates its shipped runtime under
headless `--clean` execution. Same-pin update is covered here; changed-version
acceptance remains a separate requirement.

F3's minimum canonical resolution is now implemented: failed metadata staging
reports the active profile, staging path and a permissions/access repair followed
by retry of the saved operation. It does not claim completion, change ownership
or abandon recovery. `TestProfilePublicationExplainsPermissionRecovery` fails
against the prior writer with a real readable root-owned fixture, then passes
with the diagnostic. The fixture verifies unchanged original bytes, no phase
advance and no leftover staged copy. Evidence: `profile-permission-before.log`
and `profile-permission-after.log`. A first fixture setup using macOS's `/var`
alias was corrected to the resolved physical path before the valid before-run.

Hosted engine run 38024664727 (`cc470e7`) passes macOS and WSL startup. Linux
fails because the new shell test requires zsh, which the image does not include.
Windows exposes POSIX-target fixtures built from Windows drive paths; production
correctly refuses colon-bearing POSIX PATH entries. These fixtures now exercise
the native host; the hosted matrix supplies all three platforms without weakening
the path validation or lifecycle assertions. Linux explicitly installs the shell
as a test prerequisite.

The restricted-token child still fails to launch even with an explicit fixture
ACL. Re-examining the API contract identifies the missing `TOKEN_ASSIGN_PRIMARY`
handle access right: CreateRestrictedToken returns the original handle rights,
while Go's CreateProcessAsUserW needs QUERY, DUPLICATE and ASSIGN_PRIMARY. The
handle request is corrected; token restrictions remain unchanged. This native
rerun is required before calling the ordinary-token lifecycle proved. Sources:
[CreateRestrictedToken](https://learn.microsoft.com/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
and [CreateProcessAsUserW](https://learn.microsoft.com/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasuserw).

### Native platform facts (2026-10-10)

Added read-only Linux distro/family/libc/WSL discovery at the process boundary.
Package family follows ID and ordered ID_LIKE, never whichever unrelated package
manager happens to occur first on PATH. The two policy fields use their specified
identifier alphabet and quoting; display fields are not interpreted or executed.
The last duplicate identifier wins as required by os-release. Input is bounded.
Libc comes from the actual system shell ELF interpreter; a static BusyBox case
checks the native musl loader. Unknown libc stays unknown. WSL is an observed
kernel property and keeps the Linux graph. Source: the systemd-authored
[os-release specification](https://www.freedesktop.org/software/systemd/man/latest/os-release.html).

Previously audited dynamic Linux artifacts now declare `libc: glibc`; the archive
catalog excludes them for musl or missing libc observations while retaining
compatible static archives. Native package alternatives on musl still need the
provider connection; this intermediate revision is not release-ready. Tests cover
all five existing native package families, derivative ordering, quotes, duplicate
fields, malformed/injection-shaped values, unknown identities, ELF interpreter
classification, optional-field decoding and native host observation.
`platform-discovery-focused.log` passes under the race detector.

### Platform identity correction and actual version update (2026-10-10)

The first full portable/platform gate failed the existing unchanged
`TestLinuxChoicesDoNotDependOnSessionFacts`: the initial implementation incorrectly
added distro/WSL facts to persisted Context. These facts now live in a separate
`NativePlatform` observation supplied at the native process boundary. Context
remains exactly OS/architecture; constructor fixtures remain explicit. Provider
selection consumes observed libc without making session changes invalidate the
machine identity. No test was relaxed. The focused platform, policy, composition
and concurrency checks pass under `-race` (`platform-provider-facts-and-node.log`).
The failed gate is preserved as `portable-platform-full-ci.{log,json}` with
unchanged source hashes; it is not a pass.

Node 24.21.0 uses four archives downloaded from nodejs.org and checked against
its official SHASUMS256 manifest. Its native lifecycle also executes the bundled
npm CLI through the private Node binary. Node stays a shared runtime requiring
explicit removal handling; connecting an archive does not establish the absence
of external runtime consumers. The fresh fixture redirects npm configuration and
cache locations into its disposable home.

`TestNativeArchiveChangesTheInstalledVersion` installs upstream Starship 1.25.0,
executes its real version command, switches the reviewed pin to 1.26.0, updates
through the controller, executes the new version at the same public package
entrypoint, verifies old-generation cleanup, and removes the installation.
The real macOS test passes (`native-changed-version.log`); the checked-in old pins
are test-only data and the hosted native archive step will exercise the same
transition on all four supported target combinations. This replaces no existing
preservation or interrupted-update test.

### Portable/platform gate and narrow native-provider consultation (2026-10-10)

The corrected batch passes the complete `make ci` gate with real archive tests
and native PowerShell enabled, plus Windows amd64 compilation. Both source-hash
comparisons are unchanged (`portable-native-facts-full-ci.{log,json}` and
`portable-native-facts-windows-compile.{log,json}`). The full gate took seven
minutes, including the original stack's tests. Hosted rerun remains required.

Actual `claude-opus-5-5` completed a narrow read-only native-provider consultation
in 20 read calls, exit 0, with no guard/API/permission errors. xhigh was requested;
effective effort was not exposed. All 643 supplied source and worktree hashes
and Git status remain unchanged. Evidence: `native-provider-consultation-20261010`.
This is an implementation consultation, not final delivery approval.

Accepted: keep ordinary per-resource operations and one provider-level pool of
attributable incidental packages. A per-root incidental set can lose cleanup
authority when its root is removed but another root still uses a dependency.
A shared pool avoids that leak without reviving grouped activation. Exact native
removal commands must enforce the approved attributable set and dependency safety
at execution; preserve native configuration using package-manager semantics.
Normal package-manager updates are not equivalent to personal edits in an archive;
native ownership tracks package identity and operation evidence, with versions
and file-verification evidence retained separately. Native package versions still
need to be bound to the relevant approval/execution observations.

Rejected: Homebrew receipt timestamps or a before/after inventory delta do not
prove which process introduced a package. Nor is permanently leaving all
incidental dependencies uncertain a completed implementation of this requirement.
Manager-specific operation records must be verified before connecting ownership.
The suggested apk virtual-package deletion is also unaccepted pending proof that
it cannot sweep pre-existing orphan packages. A dry run alone is insufficient.

The consultant flagged several proposed CLI details as unverified, including
apt log markers, dnf5 history, zypper userdata, pacman interleaving and winget logs.
These are research leads, not runtime evidence. Released setup does not grant
blanket ownership of pre-existing packages; migration must preserve and disclose
those without inventing old receipts. Existing capability/config migration still
needs full implementation. No additional broad architecture review is planned.

### Native matrix completion and auxiliary package batch (2026-10-10)

Hosted engine run [38026422878](https://github.com/luisgui1757/dotfiles/actions/runs/38026422878)
passes at `94d17006a4879b8814ad06990c571851196efb73`: macOS, Windows 2025,
Linux amd64/arm64, and real WSL2 startup. The unchanged policy test, corrected
restricted-token launch and real Starship changed-version lifecycle now have
native proof. This resolves the earlier checkpoint failures, not the remaining
whole-product requirements.

Fourteen verified upstream artifacts connect ShellCheck, Taplo, gh-dash,
Windows PowerShell and psmux through the existing archive driver. ShellCheck,
Taplo and gh-dash Linux artifacts are statically linked on both architectures;
PE import inspection finds only Windows system imports in these primary
executables. PowerShell's complete runtime still needs native execution.
GitHub release digests corroborate all but Taplo, whose upstream metadata lacks
a digest: its hashes pin downloaded official release bytes, without claiming
an independent upstream checksum. Actual macOS install/execute/check/update/
remove tests pass for ShellCheck, Taplo and gh-dash. gh-dash extension registration
remains a separate integration; merely placing its binary on PATH does not claim
that `gh dash` registration is complete.

The native APT contract test uses uniquely named local packages, no maintainer
scripts, and an invocation-scoped fixture repository. It verifies actual
`Dir::Log::History` operation records instead of before/after attribution, then
exercises an outside consumer appearing before exact `dpkg --remove`, shared
root removal, and preservation of an unrelated auto-installed orphan. It checks
all nonfixture package identities/versions/statuses remain unchanged and cleans
up its private artifacts. This opt-in runner test is an interface experiment;
the production native package adapter remains under implementation. Native
execution is pending at this entry.

## 2026-10-10: portable runtimes, native contracts and multiplexer simplification

- Checkpoint `0c04302` passed the local full gate with real archives and Windows
  compilation. Hosted run 38027546732 passed macOS/Windows/WSL startup; both Linux
  APT contract jobs failed because dpkg refuses a needed package's removal but
  leaves its desired selection as deinstall. The installed package survived;
  weakening the expected `ii ` state would hide pending-removal damage.
- Added exact dpkg selection restoration after refused/partial removal, preserving
  the original failure and checking unchanged version/state. Focused tests pass;
  actual Linux rerun remains required. APT attribution now cross-checks history
  against actual dpkg install/configured records, since history End-Date alone
  can also occur on failure. Homebrew's unique successful-install badge is the
  next contract under native runner verification; timestamp ownership is rejected.
- Added CMake/Python/Herdr/EditorConfig/win32yank/font archive metadata. Local CMake,
  Herdr and EditorConfig lifecycles pass. Python imports initially invalidated
  ownership via pycache writes; compileall with checked hashes at all three
  optimization levels before fingerprinting fixes the actual import/venv/update/
  removal sequence. Source and bytecode modifications remain detected. Ignoring
  executable caches or globally disabling bytecode was rejected because it weakens
  integrity or changes downstream runtime behavior. Native Linux/Windows checks
  remain required; font registration and Windows C-runtime provisioning are open.
- User explicitly retired psmux: removed its installer, plugin provisioning,
  Windows config and archive binding, plus the PowerShell OnIdle workaround.
  The graph supports Herdr everywhere and tmux only on macOS/Linux/WSL. LazyGit
  now has one empty mapping using upstream defaults; native config paths remain.
  The old behavior failed the new defaults test and the changed config passes.
- Consolidated tmux into one config. Preserved clipboard, Rose Pine variants,
  session plugins and hot variant switching. Renamed the renderer and removed
  its live psmux driver; it only generates checked-in artifacts. Retired tests
  assert removed features, not failures to be hidden; generic Scoop recovery and
  edited-copy preservation remain tested using supported fixtures. macOS actual
  tmux and focused graph checks pass; 173 PowerShell tests pass with one native
  platform skip. Full gates and final independent review still required.

Earlier psmux decisions and verification entries in this ledger are historical
and superseded by this explicit product change. Preserve old release fixture
inventories for migration; never infer authority to erase personal session data.

The compiled public machine entrypoint also passes a real isolated macOS
Starship+Herdr install, update, repair, selective Starship removal, final removal,
and configuration comparison. The hosted installed-command fixture now includes
this shared lifecycle. A legacy template check correctly caught stale expected
ignore entries after removing Windows tmux; its exact expected sets now follow
the authorized platform boundary. No deployment checks were disabled.

The complete local `make ci` gate with actual archive execution and race tests,
plus Windows cross-compilation, passed with unchanged source hashes. The broader
PowerShell run passed 275 Pester tests with one platform-specific skip. Its exact
analyzer baseline correctly rejected the removal of 14 existing diagnostics.
Comparison against the previous source found no added diagnostic identities; the
baseline now removes only those retired psmux diagnostics and a nonexistent
generated-script path. Native runner validation remains pending for this batch.

## 2026-10-10: native execution lifetime and filesystem contract corrections

Checkpoint `f7237c5` passed the complete local gate and Windows compilation with
unchanged hashes, plus all 275 PowerShell tests (one platform skip). The actual
pinned LazyGit 0.66.0 TUI opened with shared defaults, showed help, accepted Escape
and quit normally. Hosted run 38030185521 exposed Linux Python terminfo members
that differ only by case. The extractor had incorrectly rejected these on all
filesystems; it now probes the new payload directory and rejects actual collisions.
Native Linux re-execution remains required.

A small native command worker retains the existing provider OS lock after its
controller is killed, waiting for the real child to finish. Command, refusal,
output overflow and actual controller-death tests pass locally. An overflow test
caught bytes.Buffer's promoted ReadFrom bypassing the writer's limit; composition
fixes this in both the worker and Python preparation capture. Worker output drains
before returning overflow failure and protocol-error cleanup drains the response
pipe before waiting. This is a command guard, not a replacement transaction or
rollback framework. Production native provider connection remains required.

The macOS native contract reached Homebrew's current trust boundary: the generated
fixture tap's dependency formulae were untrusted. The fixture now explicitly
trusts only its unique data-only tap using a disposable XDG configuration root,
following [Homebrew's documented trust command](https://docs.brew.sh/Manpage#trust-options-target-).
It does not disable trust checks or modify the runner's existing trust store.
Native rerun remains required; the same run passed all macOS archive lifecycles.

The complete local gate with real archives and race tests passed again on the
worker/filesystem corrections, as did Windows compilation, with unchanged source
hashes. An isolated reproduction using the exact earlier output-capture type
failed after copying 2 MiB without an error; the corrected type enforces 1 MiB.
The actual compiled worker also proved lock acquisition before command acceptance
and lock release after completion.

Windows runner findings were fixture errors, not grounds to weaken removal:
Herdr selects shared PowerShell, and deselection without `RemoveShared` correctly
retains that runtime and its shell attachment. The full-removal fixture now makes
that explicit choice. win32yank 0.1.1 has no version flag, confirmed in its pinned
source; its test now exercises Unicode/newline clipboard round-trip on the
explicitly opted-in disposable Windows host. Native package contracts run even
when the independent archive step fails, while the overall job remains failed.
The standard test, released-install E2E and legacy Nix workflows all passed at
`f7237c5`; those passes do not substitute for the unfinished new-installer matrix.

## 2026-10-10: durable command results and actual native log identities

The native worker now persists command intent before execution and its bounded
result before responding. Only the request digest is stored, not raw environment
or argument values. Tests using actual child processes prove result replay across
worker restarts without repeated execution, rejection of changed/unfinished
intent, structured exit-code preservation and retained completion output after
the controller is killed. All pass locally with the race detector. The response
record is evidence for adapters, not package ownership by itself.

Run 38031656355 at `60552b9` passes the Linux amd64/arm64 archive lifecycles and
macOS archives. The native APT contract then exposed a real parser defect:
history spells Architecture: all packages with the native architecture, whereas
dpkg records name:all. The captured shape fails the new regression before the
fix and passes afterward. Reconciliation requires matching install/configured
versions and refuses competing exact-architecture, foreign or unfinished records.
It returns dpkg's actual identity. Native repetition is pending.

The Homebrew fixture now crosses explicit tap trust. Its generated formulae had
all published share/data.txt, so dependencies collided during linking. Each
fixture now uses its own named directory; no overwrite/force-link flag is used.
The failure also proves a badge can be printed after a failed link. Production
attribution must combine operation output, successful exit and verified native
receipt/activation. This corrects the earlier badge-only wording; the fixture
already required successful command exit and did not accept the failed install.

## 2026-10-10: native pool removal policy and Homebrew inventory

At `256f952`, the complete local gate and Windows compilation pass with unchanged
source hashes. Hosted run 38032468218 passes both Linux architectures, including
actual APT attribution and refused-selection restoration, and macOS, including
the corrected Homebrew transaction/removal contract. Windows is still running.

The shared pool policy now covers two consumers removed at different times,
pre-existing orphans, outside consumers, manual promotion, held/kept dependencies
and cycles. It narrows attributable packages and never invokes a general
autoremove. The native manager remains the execution-time dependency guard.
Homebrew's v2 installed inventory supplies actual keg dependencies and cask
formula consumers. Missing runtime metadata uses its runtime dependency command;
source inspection rejected `--direct`, which selects declared dependencies.
Native inventory inspection and the expanded shared-pool lifecycle are under
verification before production adapter connection.

The complete `256f952` native run subsequently passed Windows as well. Local
read-only Homebrew inspection passed against 117 real formula/cask identities;
an empty attributable pool selected no package for deletion. The new formula
adapter implements per-resource lifecycle and saved-operation recovery. Its
expanded disposable-host contract installs two roots, loses a completed response,
restarts the worker, resumes, updates, repairs, and removes roots sequentially
while checking the shared dependency and unrelated orphan. Native execution is
pending. Local boundary tests reject changed executable/arguments/environment,
missing old command records, broadened removal and a completion flag without
actual command evidence. Public routing and provider-wide approval/guard wiring
remain required; this is not a production-complete provider claim.

## 2026-10-10: native terminal-interrupt lifetime

The expanded Homebrew adapter contract passes on macOS at `9e372ab` in run
38034597448, including actual install/update/repair, worker restart after a lost
completed response, and sequential removal protecting a shared dependency. Both
Linux architectures pass that run as well; Windows remains in progress.

A real POSIX group-interrupt regression fails before worker isolation: Ctrl-C
kills the worker and releases its lock while a child that ignores interrupt is
still active. Isolating the worker process group fixes the regression; the full
native-worker race suite passes locally. Windows workers and console commands
use detached console execution with inherited pipes. Its regression creates and
signals only a private fixture console, never the runner or user's console.
Windows compilation passes; actual Windows console execution and the complete
gate are pending. Foreground elevation authentication remains a controller
responsibility before any noninteractive privileged worker command.

## 2026-10-10: controller guard and scoped native approval

The complete local gate and Windows compilation pass at `4e5fefa`, with unchanged
source hashes. Run 38035396872 passes macOS and both Linux architectures. Its
Windows signal helper fails AttachConsole with ERROR_ACCESS_DENIED: the helper
is already attached. Following Microsoft's [AttachConsole contract](https://learn.microsoft.com/en-us/windows/console/attachconsole),
the short-lived helper now calls FreeConsole before attaching to the private
fixture console. This affects only the helper, never the runner's console;
actual Windows repetition is still required.

A new real-worker controller regression fails against `4e5fefa`: preview bypasses
the native lifetime lock. The controller now holds existing engine/provider
locks during preview and acquires provider exclusion before loading core state
for mutation or abandonment. A lazy worker handoff retains the engine lock.
Focused race tests pass, including no preview writes, no worker for archive-only
requests, two commands under one worker and lock release after actual completion.

Homebrew resource observations bind the owned roots/pool and installed consumers
to approval. Unrelated package versions are excluded. A mutation may advance the
approved snapshot only for command-attributed roots/kegs/removals; other owned
packages and manual/pin/source classifications remain protected. Tests cover
outside-consumer departure broadening a removal, new consumers, manual promotion,
pins, source changes, missing pool members, unapproved ownership, unrelated
updates and actual command-attributed dependency updates. Snapshots copy ledger
maps so subsequent publication cannot change the approved baseline in memory.
The real Homebrew fixture now also exercises the controller's preview, approval,
journal, lost-response recovery, update, damaged-link repair and shared removal.
Native execution and default-catalog/bootstrap routing remain open.

Open before production Homebrew routing: the current fixture uses data-only
formulae. It cannot prove ABI compatibility when upgrading shared libraries.
Homebrew [documents](https://docs.brew.sh/Versions) that disabling installed-dependent
checks can leave broken linkage. Validate the provider's process controls against
a compiled shared-library/consumer fixture and preserve native dependent repair
semantics; marker/metadata success alone is not sufficient runtime health proof.

At `034797c`, the full local gate and Windows compilation pass with unchanged
source hashes. Hosted Windows passes native controller exclusion and lazy worker
handoff, but the console fixture does not exit after the generated Ctrl-C event.
This is a distinct failure after attachment succeeds. Microsoft's
[control-handler contract](https://learn.microsoft.com/en-us/windows/console/setconsolectrlhandler)
specifies an inherited ignore-Ctrl-C attribute, independent of Go's signal
subscriptions. The private fixture now explicitly enables that attribute's
normal processing and verifies that the sender shares the controller's console
before generating the event. This changes no runner/user console state. Native
verification is still required; the two failed console fixtures are not passes.

The same `034797c` macOS job passes controller install, interrupted-response
recovery and update, then fails the deliberate damaged-link setup: `brew unlink`
accepts formula names directly, without `--formula`. The fixture now uses the
actual command contract. Repair/removal execution remains required; this setup
failure does not invalidate the preceding completed lifecycle steps.

## 2026-10-10: native dependency maintenance consultation and ABI proof

One narrow read-only consultation ran with actual `claude-opus-5-5`; xhigh was
requested, effective effort unexposed. It completed successfully with 35 read-only
tool calls, no permission denials, model substitution or tool-boundary violations.
It recommends removing the installed-dependent-check suppression, preserving
pre-existing source/pin/manual classification through native maintenance, probing
actual root commands, and surfacing bounded native failure output. It also found
that a later no-op success may conceal an earlier dependent-maintenance failure.
These remain implementation/verification work, not completed fixes.

The consultant's proposed acceptance of a broken outside consumer when merely
reported is rejected: the user's operational requirement needs working consumers,
or a failed/needs-action operation, never successful completion with known damage.
Its concern that native dependent repair may miss upgraded dependencies is an
upstream source inference, not an executed reproduction. The new disposable-host
contract compiles a versioned dylib plus selected and pre-existing consumers,
changes the library ABI, requires operation-marked proof that it actually upgraded,
and executes both consumers. Retaining a usable old keg and rebuilding for the
new library are both acceptable. Pre-existing ownership classifications must remain,
and removal must leave that consumer functional with an empty installer ledger.
This test compiles locally; native execution remains pending. Shared native
fixture setup now centralizes only its existing isolated tap, fixed archive,
explicit trust and exact unique-name cleanup, with an unrelated-package baseline.

The ABI test's first full gate correctly rejects spaces inside its Go raw
Ruby/C literals. Their semantically immaterial indentation now uses tabs, without
changing or suppressing the checker. At `40a25c7`, the hosted macOS controller
lifecycle passes completely, including damaged-link repair and shared removal.
Windows' native engine step also passes the corrected private-console Ctrl-C
fixture; its archive lifecycle step is still running.

## 2026-10-10: verified controller lifecycle and native maintenance preservation

Hosted run 38037273827 at `40a25c7` completed successfully on macOS, Windows,
Linux amd64/arm64 and WSL2 startup. The macOS controller lifecycle passes all
install/recovery/update/repair/shared-removal stages (195.93 seconds). Windows
passes `TestNativeWorkerRetainsLockAfterConsoleInterrupt`, controller exclusion,
and the compiled installed-command archive lifecycle.

Evidence scope correction: the terminal signal reproduction calls the worker
primitive directly. The public main function already subscribes to Interrupt,
so the failing-before SIGINT result does not prove that exact old public
entrypoint failed on Ctrl-C. It proves the worker cannot rely on its caller's
signal subscription. The isolation invariant and native process-group/console
regressions remain valid; no old public-entrypoint SIGINT failure is claimed.

The compiled ABI contract and shared fixture passed the full local gate and Windows
compilation with unchanged source hashes before checkpoint `1300bc8`; its hosted
ABI execution is pending. Separate process-boundary regressions reproduce successful
completion after a pre-existing package disappeared or changed source/pin/manual
classification. Schema-2 intents now persist that baseline, compare command-touched
classifications, and refuse unexplained disappearance before ledger publication.
Version changes remain permitted and never acquire pre-existing packages. A legacy
names-only intent is rejected because this prototype was not released and lacks
the preservation proof. The before suite failed all five damage cases and legacy
acceptance; focused race tests pass afterward.

A real worker-child failure also reproduced a generic `exit status 1` with its
useful diagnostic lost by the caller. Native command errors now include the last
8 KiB of captured output, stripping terminal controls while preserving the structured
exit code. The child-output regression failed before the fix; focused verification
and the complete gate follow. These fixes do not yet establish ABI maintenance or
production default-provider routing.

Native CLI recipes can now declare an absolute trusted health command. It runs
before ownership publication and during observation; failures retain their reason
in an optional HealthIssue field, allowing the planner to explain a repair without
turning damage into an unresolvable pending state. Previous observations without
that field retain their meaning. Process-boundary tests cover failure before ledger
publication and healthy completion. The compiled ABI fixture now also removes the
root binary's execute permission, verifies an unhealthy observation despite a linked
keg, and restores permission before continuing its update/removal test. Native
execution of this extension remains required.

The first native compiled-ABI run at `1300bc8` **reproduced real breakage**:
`TestNativeBrewSharedLibraryUpdatePreservesOutsideConsumer` aborts in dyld because
the outside binary references the v1 dylib through Homebrew's opt link, which now
points at v2. The earlier data-only controller lifecycle still passes. This rules
out a mere inventory or fixture-version assertion failure. The existing dependent
check suppression is removed, with an explicit empty child override so an inherited
caller flag cannot disable required repair. The same test now injects that caller
flag to exercise the override. Native rerun will establish whether Homebrew's own
repair is sufficient; this is not yet claimed fixed.

## 2026-10-10: failed dependent maintenance recovery and cross-platform contracts

The full local gate and Windows compilation pass at `1b04606` with unchanged
source hashes. Its hosted Windows run then exposes a test-boundary mismatch:
new process-boundary fixtures supply a native absolute drive path, while the
Homebrew evidence parser accepts only POSIX absolutes. The parser now accepts
canonical slash-normalized native or captured POSIX paths and still rejects any
marker outside the exact configured Cellar. A new host-path regression retains
the independent POSIX cases. No tests are skipped and Homebrew remains macOS-only
in the product. The same boundary review also rejects a malformed environment
control missing its equals sign before any state publication.

The recovery regressions reproduce two failures against an isolated `1b04606`
source snapshot: a failed upgrade later completes via a root-only no-op, and an
initial root build failure skips previously changed dependencies. Both pass after
recovery re-runs native reinstall for the root and exact prior-operation package
set. With a missing initial root, dependency maintenance precedes root installation.
Its lost-response test resumes after the completed first phase without repeating it.
Saved recovery requests reject unrelated extra packages. This uses Homebrew's native
repair behavior, not a second dependency solver, and preserves classification checks.

The compiled native ABI fixture now also intentionally fails an outside dependent's
rebuild after the selected root and library have upgraded. It independently runs the
old root-only retry command and requires the outside consumer still to fail, then
requires actual adapter recovery to leave both commands runnable. Native execution
of this extended acceptance case and final full gates are pending. The ordinary
compiled update test remains separate so a failure can be attributed precisely.

The hosted result at `1b04606` disproves sufficiency of merely re-enabling native
checks: the same compiled outside consumer still aborts in dyld (35.35 seconds),
while the data-only controller lifecycle passes (202.59 seconds). This supersedes
the uncommitted broader-reinstall recovery experiment above. Completion now batches
native linkage checks over command-attributed changes and their recorded runtime
consumers, inspects individual failures only when needed, and reinstalls only those
failing formulae. Repeating a root upgrade cannot bypass that check. One unsuccessful
repair of a consumer stops the attempt; recovery cannot append unrelated packages.

Intent schema 3 stores the native pre-state graph alongside classifications;
unreleased schemas 1/2 cannot provide it. These edges come from Homebrew, not a
second version solver. Existing ownership rules and classification checks remain.
An affected cask currently stops completion because formula linkage cannot verify
an application bundle; the product boundary must be resolved before default routing.
Linkage runs only during approved mutation and uses a child-only developer-command
flag; the native fixture compares Homebrew's developer preference before/after.
Its per-contract deadline increases from four to eight minutes because it now runs
real linkage inspections and compiled repairs; this is test budget, not an SLO
exception. The original data-only controller already took 202.59 seconds.

Final targeted-maintenance regressions fail against the isolated `1b04606` baseline
(root-only false success, missing-root recovery and both ignored/pinned consumer
cases) and pass locally with race checking. Root command health is rechecked after
dependent maintenance and before ownership publication, because a rebuild can itself
change dependencies. The final native acceptance and full gate remain required.


### Standalone dashboard and all-version formula removal (2026-10-10)

The new graph runs the checksum-pinned upstream gh-dash executable directly. The
alternative was a second authenticated extension installation and ownership path
for the same executable. Direct invocation removes that path while retaining
GitHub CLI and the existing dashboard configuration. The command becomes
`gh-dash`; existing extensions remain user-owned. Upstream supplies release
binaries and a standalone main (`gh-dash.go` at v4.26.0), and documents manual
binary updates at https://www.gh-dash.dev/getting-started/updating/.
The graph regression fails before removal of `gh.extension` and passes afterward
for all three OS targets. The actual pinned macOS binary completes installation,
version/help execution with empty tokens and isolated GH_CONFIG_DIR, update,
check and removal. This does not prove an authenticated API session or the full
Git-dependent capability.

Native engine run 38041004299 at `121783d` passes Windows, both Linux architectures
and WSL2 startup. Both compiled macOS tests now finish shared-library maintenance
and real outside-consumer execution, including recovery after failed repair, but
fail final removal because ordinary Homebrew uninstall removes only one version.
The same failure is reproduced at the native process boundary locally.

Use Homebrew's documented `uninstall --formula --force` for the exact owned,
unheld removal set. This selects every formula version; it does not disable the
native dependent check. The separate `--ignore-dependencies` flag is forbidden.
Source inspection of Homebrew `cmd/uninstall.rb` and `uninstall.rb` confirms the
check precedes deletion; the disposable native test now explicitly attempts
all-version removal of an outside consumer's dependency and must be refused.
Formula `--force` does bypass Homebrew's pin check, so immediately before every
command dispatch/recovery the adapter revalidates source, pin, manual status and
outside consumers. Its ordinary provider guard serializes this installer's
commands, not independent package-manager writers; do not describe the separate
native query and command as an atomic compare-and-swap.

The all-version regression and changed-input dispatch/recovery tests pass locally
with race checking. Saved removal arguments cannot insert a dependency bypass or
an unrelated package. Complete native rerun and full gate remain required.


### Retained native dependencies and npm graph correction (2026-10-10)

Removal correctly protected shared incidental packages but reported only members
of the removal set, concealing dependencies deliberately excluded from that set.
It also retained deletion authority after a manual promotion/source replacement,
and an older receipt continued reporting a dependency which no longer existed.
The process-boundary regression fails for all four initial cases before the fix;
a pin case additionally verifies retention of ownership until explicit unpinning.
The final tests pass with race checking, including recovery from the persisted
shape after ledger publication but before completed-intent publication. Reports
are saved before ownership is relinquished. The real Homebrew shared-consumer
contract now checks the retained dependency is disclosed, not merely present.
All-version argument parsing also excludes the fixed --force option when deriving
the operation's changed-package set. Native proof and the complete gate for this
additional correction remain required.

The accepted design already rejected global npm-prefix rewrites, but the new
Pi graph still referenced the obsolete integration. That resource is deleted;
Node and shell dependencies remain. This is a graph correction, not a claim that
Pi's private package provider is complete. Its regression fails before and passes
after on all three target graphs.


### Direct tmux integration and final removal disclosures (2026-10-10)

The new graph uses verified sensible/yank/resurrect/continuum archives with one
managed loader. TPM and Git are unnecessary for this path; the released loader
remains only until public cutover. Upstream documents direct entrypoint loading
in the [resurrect manual installation](https://github.com/tmux-plugins/tmux-resurrect#manual-installation)
and [continuum manual installation](https://github.com/tmux-plugins/tmux-continuum#manual-installation).
The native fixture supplies tmux explicitly; this is plugin/profile proof, not
production native-package provisioning.

Source inspection found continuum's disabled automatic-start branch removes a
personal macOS LaunchAgent or disables a Linux systemd user service. The new
loader activates its pinned save/restore helpers without invoking that handler.
The native fixture keeps a personal LaunchAgent and records any systemctl call at
the process boundary. No actual user services are changed by these fixtures.

The real pinned resurrect archive exposed three dangling test-only submodule
links; runtime code uses none of them. Exact exclusions retain path, duplicate,
link-target and expansion validation and must match actual archive entries. Real
execution then reproduced unquoted version-check/spinner paths, broken backup
retention, unquoted saved-session reads and apostrophe loss in pane restoration.
Corrections are bounded literal replacements against reviewed text/counts in a
fresh checksum-verified payload. A changed count/text fails before publication;
no arbitrary patch command or existing-install mutation is accepted. Optional
metadata preserves old pin serialization and participates in new recipe identity.
Boundary tests cover unsafe/required/repeated exclusions, escaping links, missing
members, symlink replacement, source mismatch, duplicate/count mismatch, non-text
inputs, output expansion and executable permissions.

Native before/after logs reproduce the failed retention and restore paths. The
corrected real tmux fixture passes manual save through the actual serialized
keybinding, automatic session and pane-content restoration, newest-five backup
retention, update/repair, and removal with personal snapshots and active sessions
preserved. Its home includes spaces, apostrophes, # and $. The first apostrophe
correction was rejected by the pane-content test and replaced with shell quoting
verified in both available Bash implementations. The actual configured automatic-save
hook also passes after update/repair. The full gate and hosted matrix remain required.

At 6635d3a, engine run 38042289005 passes all native targets, including compiled
Homebrew consumer maintenance/recovery and all-version removal. At 755fd94,
run 38043287892 passes Windows, both Linux architectures and WSL2 startup; macOS
exposes an obsolete retained-dependency disclosure after actual successful
collection. The provider already returned fresh observations, but the executor
kept its earlier intermediate report. A real-filesystem regression fails before
and passes after refreshing removed receipts from final verified observations,
covering one and two transactions and independently preserved personal files.
Unknown/changed observations cannot clear a prior report. The native assertion
requiring no stranded receipt remains unchanged; native repetition and the final
full gate are still required.

### Linux guests, Apple Silicon and native provider connection (2026-10-10)

Checkpoint `4525683` passes the unchanged-source local full gate and Windows
compilation. [Hosted run 38044716319](https://github.com/luisgui1757/dotfiles/actions/runs/38044716319)
passes macOS, Windows and Linux amd64/arm64, including the real tmux save/restore
lifecycle and the macOS final-removal disclosure regression described above.

The owner reconfirmed that WSL is Linux, not a fourth platform. The new engine's
WSL flag had no consumer and introduced an unnecessary kernel-file dependency.
The separate hosted job proved guest startup and interoperability but never ran
the installer. Both are removed. The current plan no longer promises a host-
support checkbox, persisted WSL GUI intent or separate guest lifecycle. Actual
clipboard/graphical capabilities still need consumer-level evidence; ordinary
Linux tests do not prove Windows desktop interoperability. The released split-
host scripts remain historical migration inputs until the verified cutover.

The owner explicitly excludes Intel macOS. Planning already rejected it, but
archive selection still accepted a leftover Intel Starship pin. The regression
fails before the correction. Discovery, constructors, package metadata and
planning now share target validation; macOS requires arm64 and the obsolete pin
is removed. The native CI matrix asserts its actual target. The focused race
tests pass, including existing unsupported-Windows/Linux cases.

The macOS application now discovers actual Homebrew paths at its process boundary
and connects Git/tmux to the existing formula worker. Constructors remain pure,
shell PATH integration follows selected dependencies, and existing infrastructure
is reused without acquisition/removal authority. Process-boundary tests cover
quoted paths, child-only environment controls, forbidden elevation/input,
selection-scoped profiles and resource-local discovery failure. A read-only
production controller preview against the real Homebrew inventory passes without
creating installer state or profiles. This does not prove fresh bootstrap or
full application installation; those remain required alongside the full gate.

The subsequent installed-application fixture shares the existing real executable
and machine-protocol harness with archive acceptance. It requires both disposable
GitHub execution and the native-package opt-in. Its baseline removes the earlier
tmux fixture prerequisite using ordinary dependency-protected Homebrew removal,
then verifies inventory absence. It exercises default tmux/gh-dash selections,
fresh-shell command resolution, update, actual unlinked-package repair, selective
removal and full removal, retaining personal profile bytes and pre-existing native
package classifications. Native execution is still pending; this is not bootstrap
proof and does not pretend pre-existing Git was freshly installed.

The platform-boundary full gate passed its behavior checks but rejected the stale
Renovate inventory entry for the removed standalone Windows/WSL job. The reviewed
inventory now drops that exact job dependency; the retained Windows matrix entry
is still checked. The shared installed-command archive lifecycle passes locally
after the harness extraction. Full-gate repetition remains required.

### External cask policy and native application checkpoint (2026-10-10)

Checkpoint `f55b281` passes full `make ci` and Windows compilation with unchanged
source hashes. Its hosted matrix is running. The added matrix metadata initially
changed GitHub's default job display names; an explicit job name now preserves
the established `core (runner)` contexts while still asserting native targets.

Actual `claude-opus-5-5` completed one focused cask-boundary consultation with 33
read-only calls, no denials, substitutions or boundary violations. xhigh was
requested; effective effort was unexposed. All 39 supplied source hashes remain
unchanged. This is a focused design opinion, not full-delta code approval.

The blanket post-mutation cask refusal is accepted as a defect: every resume
hits the same immutable affected-cask set, even when the application and native
dependencies are valid. A regression reproduces both ordinary completion and
retry failure before the correction. The reconciled policy checks affected
formula linkage, cask version/source preservation and the union of prior/current
declared dependencies for presence and activation. It explicitly reports external
applications as not exercised. Known native failure still blocks or repairs;
supported managed applications retain their own runtime health requirements.
General Mach-O/app inspection and speculative preflight refusal are rejected:
neither provides universal runtime proof, and neither is needed to fix the
unrecoverable state. Current API cask dependencies describe native manager policy,
not a promise to reconstruct every installed application's dynamic dependencies.

Optional intent/observation disclosure fields carry no mutation authority. They
are persisted before ownership publication, validated on read, and shown in
approval, completion and read-only checking. Existing shapes without those fields
remain readable. Focused race tests pass for successful completion, inactive-edge
recovery, identity changes, persistence, invalid disclosure inputs and UI reporting.
A new disposable native cask fixture consumes the actual compiled outside formula;
its ABI-changing update, continued execution, ownership and removal proof are
implemented but not yet run on macOS.

The hosted Linux amd64 archive step at `f55b281` fails when immediately restarting
tmux after `kill-server`, with `server exited unexpectedly`; Linux arm64 passes.
One hundred minimal no-config restarts on macOS did not reproduce it, so the cause
is not yet confirmed. The full fixture now requires observed old-process exit
before restart, preserving all restoration assertions. Three full local repetitions
pass. Native Linux confirmation is still required; this is not permission to
retry away a crash or weaken session/pane restoration checks.

### Native profile cleanup after deselection (2026-10-10)

The completed `f55b281` hosted run passes Windows and Linux arm64. macOS reaches
ready through real installed-command install, update, broken-link repair and both
removal steps, then the unchanged final assertion detects a managed shell block
left in the personal profile. This is a catalog dependency defect: retaining
pre-existing Git retains its incorrectly declared shell integration prerequisite.
The same edge exists for tmux. The actual program remains independently usable;
its selected configuration is what needs installer shell integration.

Move those two dependencies to `config.git` and `config.tmux`. A default-catalog
regression fails before and passes after the correction on every supported
OS/architecture, covering pre-existing, shared, explicitly kept and still-selected
programs. Existing retained-runtime preservation tests continue to pass. Disabling
retained-resource closure is rejected because genuine runtime consumers still
need their prerequisites. The native profile assertion stays unchanged; hosted
confirmation and the complete gate are required before calling the fix verified
end to end.

### Pi standalone delivery simplification (2026-10-10)

The pinned [upstream v1.0.4 release](https://github.com/earendil-works/pi/releases/tag/v1.0.4)
provides standalone artifacts for every supported target. All four downloads match
the published SHA-256 asset digests. The pinned build script compiles the runtime
and workers and includes runtime assets and native platform helpers; archive
inspection confirms those layouts. Reuse the existing archive provider instead
of introducing an npm package installation subsystem. The Linux binaries require
glibc; do not claim this proves musl support.

The upstream package manager still invokes npm and Git for user package features,
so retaining those prerequisites avoids silently reducing Pi functionality. Add
fd/ripgrep explicitly to avoid first-use application downloads. No global npm
prefix or user credentials are touched. The private CLI itself is removable;
personal Pi data remains outside that archive ownership.

The real macOS arm64 archive lifecycle passes, including version/help, update,
check and removal. A further offline RPC check loads a TypeScript extension from
a path containing spaces and apostrophes with an empty PATH, isolated agent data
and no inherited credentials. The registered command proves the bundled extension
loader executed. This does not prove authenticated model requests, external
package installation or the still-unimplemented scoped settings integration.
Hosted archive/extension execution on all targets remains required.

### Scoped Pi settings and completed native checkpoint (2026-10-10)

At `53b0c93`, the full local gate and Windows compilation pass with unchanged
source hashes. Hosted Windows and both Linux targets pass. macOS's unchanged
installed tmux/gh-dash application test now passes through install, update,
broken-link repair, selective/full removal and exact personal-profile restoration.
The compiled outside-consumer update, failed-maintenance recovery and native
removal tests also pass. The added cask fixture incorrectly expected an initial
warning even though the root reused an unchanged dependency. Correct that
expectation and add an unaffected-cask regression; keep the ABI-change marker,
later disclosure, application execution and ownership assertions unchanged.
The cask update path still requires a hosted rerun.

Pi's theme property now uses the existing profile publication/recovery machinery,
with an explicit JSON field scope persisted in its journal and baseline. Missing
scope metadata retains the previous shell-block interpretation. Scope participates
in target identity and cannot change during resume or restoration. The strict
JSON editor preserves unrelated byte ranges and exact large numbers, rejects
ambiguous duplicate keys and malformed input, and restores the previous owned
value on removal. Later changes to that owned value are retained as personal data.
No separate publication engine or npm installation subsystem was introduced.

Pi uses proper-lockfile's settings lock directory. The publisher saves its journal
before acquiring that directory and atomically includes an operation-bound owner
record. A killed operation can resume its own lock; another application's lock
is never removed based on age. The public lock is renamed before cleanup so a
new application lock cannot be deleted by the old operation. Finalization also
releases known locks during approved abandonment. Acquiring before recording the
journal was rejected because death before publication could otherwise strand an
application lock with no recoverable target record.

Scoped settings tests pass with the race detector: prior-value restoration,
unrelated edits and formatting, large numbers, new-empty-file removal, changed
owned values, saved-scope validation, real process death followed by resume or
restore, active application contention, and abandonment of both external and
installer-owned locks. Existing profile/recovery regressions also pass. Initial
new-test assumptions were corrected to compare the parsed theme while preserving
its original spacing, and to require the established second approval after
resource recovery; the product contract was not weakened.

The real compiled Pi application lifecycle passes locally in an isolated home.
It reuses pre-existing native Git, installs private Node/fd/ripgrep/Pi, explicitly
adopts the theme field, runs an offline TypeScript extension through Pi RPC,
updates, and removes the selection. The prior theme, later unrelated settings
edits and personal session bytes survive; the entire native package inventory
retains its versions and classifications. This fixture refuses native Git
mutations. RPC's upstream UI explicitly does not support theme switching, so this
is not claimed as interactive TUI theme-rendering proof. Hosted archive/settings
coverage, actual package features and TUI acceptance remain required.

### Pi retained-theme dependency correction (2026-10-10)

A default-catalog removal regression failed because a reused personal theme
preference retained `config.pi`, which unnecessarily retained `tool.pi` and its
private runtime dependencies. Remove the passive config-to-executable edge and
the obsolete Go theme-publisher-to-Node edge. The Pi root still selects its CLI,
and the CLI still selects Node/npm, Git, fd and ripgrep. The regression now passes
with the race detector for all four native targets: only the needed theme assets
and pre-existing Git remain. This changes real dependency edges, not the planner's
retained-consumer protection.

The native Homebrew suite measured 508 seconds before completing its new cask
update path or adding Pi's 75-second installed lifecycle. Give that suite a
20-minute Go test deadline within its existing 30-minute hosted job. Per-operation
timeouts and failure assertions remain unchanged.

### Pi interactive theme consumption (2026-10-10)

The pinned standalone Pi now runs in the existing native terminal fixture with
canonical theme/keybinding files and no credentials or external Node on PATH.
A real command verifies the initial Rose Pine theme, discovers all three variants,
switches each using Pi's interactive UI API, checks the actual foreground escape
sequence, restores the default and exits through Ctrl+D. macOS PTY execution
passes with the race detector. The archive CI step now includes subpackages so
the same check executes in Linux PTY and Windows ConPTY. Those native results
remain required. This is runtime theme consumption proof, distinct from the
compiled installer settings-restoration test and package-feature acceptance.


### Roadmap reset, scope and verified checkpoint (2026-10-10)

Actual `claude-opus-5-5` completed a read-only roadmap consultation with 74
Read/Glob/Grep calls, no permission denials or execution errors. xhigh was
requested; effective effort was not exposed. All 698 supplied source/evidence
hashes remained unchanged. This is planning consultation, not final code approval.
The returned provider-route counts describe static coverage, not proven runtime
journeys. Published `e1c987d` subsequently completed all four native jobs in
[run 38053630162](https://github.com/luisgui1757/dotfiles/actions/runs/38053630162),
including the corrected cask ABI fixture and macOS installed Pi lifecycle.

Accepted: preserve the tested engine, port package data/exact commands rather
than call prompting legacy installers, reuse Neovim's headless commands, isolate
parallel ownership, and use five finite product milestones. Replace redundant
fresh full-minus-one runs with singleton proof plus full selection and selective
removal, shared-edge orders and existing transition properties. Do not repeat
unchanged passing suites for every small checkpoint. Required integrated/final
gates and independent full-delta review remain. Superseded prebuilt-attestation
and backwards runtime round-trip proposals are marked historical in the plan.

Rejected: treating existing Git as sufficient provisioning when missing; silently
dropping a promised Rust language prerequisite; treating package-route counts as
end-to-end proof. Existing package infrastructure remains retained/disclosed by
its prior ownership policy; this does not require a new blanket deletion choice.
A ruleset proposal belongs to cutover, and live permissions must be checked before
claiming only the owner can apply it. No protection changes occurred here.

The owner explicitly selected Ubuntu/Debian only after considering Fedora too.
APT is the sole Linux native provider. Fedora, Arch, openSUSE, Alpine, Linuxbrew
and ID_LIKE derivatives are excluded from the new release. WSL keeps ordinary
Ubuntu/Debian discovery. The focused unsupported-distribution test fails before
the change for all eight inputs and passes afterward. The parser now needs only
exact ID and removes unused family routing/static BusyBox discovery. Actual ELF
libc inspection still prevents selecting incompatible archives. The old broad
family fixtures are replaced by supported-ID and explicit rejection cases because
the documented product boundary changed, not to conceal a failure.

Independent fd/ripgrep choices fix the previously failing checkbox regression;
they reuse existing private archive and shell dependency routes. Focused platform,
archive-selection, singleton-closure and checkbox tests pass with the race detector.
Singleton fixtures now name all four actual targets. The former Windows/arm64
fixture was inaccurately labelled but was not vacuous: graph availability is based
on OS, so it did still exercise Windows edges. No missing runtime proof is inferred
from that label correction.

Pi's subsequent native macOS package fixture passes real npm and Git package
installation, extension RPC loading, changed Git-commit update and removal with
private Node and local transports. Explicit update commands enable the upstream
update path; offline mode intentionally skips it. No authenticated remote service
or Git provisioning is claimed. The TUI and package cases still need native
Windows/Linux execution. Source and logs remain in the task artifacts; the hosted
archive step now includes the terminal subpackage.


### APT routing and checkout bootstrap integration (2026-10-10)

The Ubuntu/Debian provider now connects Git, zsh, tmux, Make, build-essential and
clangd to the application. Pure constructors accept process-boundary discovery;
missing privilege support is local to APT resources. Selection clones keep their
own package-retention list and share only the locked mutation approval. Make
precedes build-essential because the latter installs it transitively. Foreground
sudo authentication occurs only after approval, with package protection checked
again afterwards; workers remain noninteractive. Refused authentication records
no dispatched command. Integrated provider, catalog and routing checks pass with
the race detector, including exact removal and cumulative recorded-attempt recovery.
Native Ubuntu and Debian amd64/arm64 workflow execution is still pending.

APT ownership comes from saved operation-specific native records. dpkg source
identity means source package, not repository provenance. Unknown worker results
or truncated native logs require inspection; automatic power-loss recovery from
those states is not implemented. Existing packages, manual promotion, holds and
outside consumers remain protected. The new native fixture uses an actual local
APT repository, removes its own cached index to require provider refresh, restarts
the worker, updates 1.0 to 2.0, repairs a missing file and preserves outside users.
It was compiled, not executed, on this Mac. Debian CI uses the official multiarch
image pinned from Docker's repo-info metadata and tests direct root execution
without sudo; Ubuntu tests the normal sudo worker path.

The checkout launchers verify a private Go archive and cache their built binary
by actual Go runtime/embedded source inputs. They accept only the menu or explicit
machine protocol. Released v0.1.0/v0.4.4 config-target inventories are generated
from exact Git blobs, including actual Windows known-folder mappings. This is
migration source evidence, not adoption/removal authority, intermediate-release
coverage or a finished migration engine. Public setup remains unchanged.

Integration review found that the initial bootstrap reused an extracted compiler
after executing its version check, which did not verify its bytes. The regression
with a modified compiler failed before the correction and now passes: extract the
verified cached archive into private staging each launch, disable external Go
cache programs and implicit PGO input, then remove staging before launching the
installer. No additional persistent integrity framework is needed. Thirteen
POSIX launcher tests, five inventory tests, three PowerShell boundary tests,
ShellCheck and PSScriptAnalyzer pass. Real macOS build/preview took 6.65 seconds
and warm preview 4.25 seconds, with an empty isolated home and no surviving staging.
The official archive was preseeded and reverified; this does not claim another
network-download run or native Windows/Linux execution.


### Native application acceptance follow-up (2026-10-10)

A compiled Linux application fixture now exercises actual APT discovery, absent
tmux provisioning, fresh-shell command resolution, update, missing-binary repair,
selective removal and full restoration. It checks the actual Git/zsh baseline and
never treats PATH hiding as absence. The minimal Debian job no longer preinstalls
Git; the application must provision it there. This fixture compiles/skips locally
and still needs native execution. It is a subsequent batch, outside the frozen
APT/bootstrap checkpoint currently running its full gate.

Homebrew discovery now checks the declared bootstrap executable when the current
shell has not refreshed PATH, then queries actual prefix/Cellar. This avoids
misclassifying a completed bootstrap as absent. A real subprocess fixture passes
with empty PATH and an executable path containing spaces and quotes. It grants
no ownership and does not initialize an unknown prefix.


## 2026-10-10 — Native prerequisite and Neovim integration

- APT/bootstrap checkpoint `3842e84` passed the unchanged-source full `make ci`
  gate and Windows amd64 cross-compilation. It remains one commit in PR #87.
  The hosted run is https://github.com/luisgui1757/dotfiles/actions/runs/38058491320.
- Integrated frozen Homebrew bootstrap files after SHA256 verification. Retain
  created infrastructure and reuse healthy pre-existing managers without removal
  authority. Do not run global `brew update` as a selected-tool update: upstream
  manager migrations can mutate unrelated packages. Existing formula update
  behavior remains responsible for selected tools.
- Fresh Homebrew test uses a separate disposable runner whose original prefix is
  preserved elsewhere before the test. It proves actual prefix absence, not a
  hidden PATH entry. The production driver never moves an existing prefix. This
  does not prove a machine without Apple Command Line Tools/Xcode.
- Integrated pinned VC++ vendor runtime, read-only native registry/DLL queries,
  Microsoft signature checking and operation-bound Burn completion. Native
  Windows execution is outstanding; do not treat PowerShell parsing as proof.
- Integrated private Neovim sync and source-side runtime selection. Actual macOS
  bootstrap/check/removal passed: 33,235 entries, 1,301,951,813 regular-file bytes,
  nine manifest chunks, all locked plugins, 95 parser outputs and Mason tools.
  The fixture reused host Git/Make/compiler; it proves the recipe, not fresh
  native prerequisites or Windows/Linux. Chunk manifests are necessary for this
  observed size, not speculative scale. Read-only recheck preserved all bytes.
- Added pure application wiring and independently discovered XDG_DATA_HOME.
  Neovim's pinned stdpaths.c explicitly appends nvim-data on Windows. Constructors
  never inherit the caller PATH/XDG data root. Windows known-folder defaults stay
  independently observed, with no reconstructed user AppData directory.
- Focused integrated Go race tests passed; final full gate and actual hosted
  prerequisite acceptance are still required before this batch is published.


### Windows compiler integration and native fixture correction

- Integrated the frozen Build Tools patch with SHA256
  f93258b26a7f409ce878f910e0e1aaa898b60938b89b3e018cabf578cba147e2.
  Its dedicated instance is separate from pre-existing Visual Studio. Native
  health compiles/runs a Windows SDK C++ consumer; GNU Make is still separate.
  Parent wiring uses actual Program Files known-folder discovery, and lazily
  derives the approved SDK environment for Neovim after prerequisite installation.
- The VC native fixture now tests pinned 14.50.35719.0 to 14.51.36247.0 before
  repair/removal. Both fresh vendor fixtures fail on a pre-existing baseline.
  Separate Windows Server Core containers avoid one fixture provisioning the
  other's prerequisite. Native execution is pending; no new approval is implied.
- All four APT jobs passed on checkpoint 3842e84 (Ubuntu/Debian, amd64/arm64).
  Windows failed TestNativeControllerConnectsAPTAndKeepsSelectionStatePrivate
  because the Linux shell assembler correctly rejected Windows drive colons.
  The exact fixture and assertions moved to a Linux build-tagged test file; no
  assertion was weakened. Catalog tests remain portable. Windows must rerun.
- The first new Neovim application fixture failed because macOS temporary paths
  used the /var alias while private archive paths require their resolved parent.
  Canonicalizing the fixture root fixes that evidence setup, without loosening
  production destination validation. The application fixture follows the native
  host; a separate pure layout test covers all three data-directory conventions.
- Rust will use the same six checksum-pinned upstream components on every
  supported target. This avoids native-manager recipe divergence, Ubuntu universe
  enablement and distro compiler-version differences with one existing archive
  lifecycle. Official components exist for all four targets.


### Apple prerequisite observation and hosted fixture maintenance

- Connected read-only selected Apple compiler/Make/SDK and OS clipboard checks;
  actual local observation passed without installing packages or touching clipboard
  contents. Missing CLT provisioning is the next separate retained prerequisite.
- Dedicated hosted jobs cover an absent Homebrew prefix and independent Windows
  runtime/compiler baselines. Their additional Actions/Go/runner references and
  pinned Windows image are in the reviewed Renovate extraction inventory. The
  Windows image has an explicit regex manager because it is passed to docker run.


## 2026-10-10 — Clipboard capability correction

- Actual isolated tmux regression failed before the change: with wl-copy/xclip
  installed but no advertised display, the copy binding selected wl-copy. The
  corrected config passes six real tmux cases: headless, X11, Wayland, xsel-only,
  existing bridge and native pbcopy. No test accesses personal clipboard contents.
  Mirror updated while the old released chezmoi path still exists.
- Neovim warning regressions failed before the change and all nine now pass.
  Installed display helpers no longer imply an active transport; a false-valued
  g:clipboard requests builtin discovery rather than suppressing diagnostics.
  Removed the obsolete instruction to install software on a Windows host.
- Both APT helpers are hidden Linux prerequisites regardless of the setup session.
  The graph stays fixed for headless-to-desktop transitions; session capability
  disclosures are not persisted target identity or removal authority.
- Evidence: clipboard-before.log / clipboard-after.log and the matching
  nvim-clipboard-before.log / nvim-clipboard-after.log in the task artifact root.

## 2026-10-10 — Complete prerequisite wiring and native fixture corrections

- Integrated six-component Rust archives, isolated LaTeX conversion, complete
  PortableGit/GNU Make and retained Apple CLT provisioning into existing provider
  lifecycles. All Neovim prerequisites now have routes. Python's entire reviewed
  archive identity participates in the converter's desired identity; a stable
  command path lets its venv survive retiring the previous interpreter generation.
  Mac private Rust/LaTeX lifecycle proof passed; all foreign Rust layouts passed
  checksum/extraction checks. Native Windows/Linux execution remains separate.
- Apple CLT uses the pinned Homebrew recipe's install-on-demand mechanism and
  Apple's exact advertised package. Thirteen actual worker/script fixtures passed;
  missing-tool installation remains gated to the dedicated fresh Mac runner.
  Existing Homebrew, selected external Xcode and unknown native outcomes preserve
  their prior ownership and require explicit recovery where proof is missing.
- Native run `d1cf54b` proved fresh Homebrew. Its Windows vendor containers failed
  before execution because bare dotted PowerShell arguments became `-test`.
  The workflow now quotes those arguments, with an actual PowerShell-to-native
  subprocess regression reading the workflow invocation.
- Linux fixture failure was missing observed libc metadata. The shared-profile
  test now uses the existing native-platform discovery helper. Ubuntu's hosted
  `ubuntu-server` depends on tmux, so deleting that baseline is invalid. Fresh
  application lifecycle tests now assert initial absence in clean Ubuntu/Debian
  containers on both architectures; they never remove runner meta-packages.
- Windows runtime inspection compared textual versions, rejecting equivalent
  zero-padded revision strings; numeric normalization has an actual PowerShell
  regression. Its native disagreement diagnostic now reports both versions.
  Microsoft chain inspection also rejected a valid Authenticode signature; a
  timestamp-aware ownership check follows Microsoft's signing-time contract and
  preserves rejection of every non-expiry chain error. Hosted confirmation is
  required; local parsing and cross-compilation do not prove native trust behavior.
- Actual Renovate extraction covers all 116 current dependency records, including
  literal Ubuntu/Debian container digests and the additional native job references.
  Separate literal container jobs keep the existing extraction gate complete;
  no unresolved-expression exception was added to its validator.

### 2026-10-10 — Retained Git and selected shell configuration

The integrated prerequisite gate reproduced a Windows removal regression in
`TestRetainingNativeProgramDoesNotRetainRemovedShellConfiguration` and
`TestRetainedPiThemeDoesNotKeepRemovedExecutable`: the PortableGit mapping
incorrectly depended on shell publication. Removed that package-to-profile edge;
selected Git configuration and consumers already select the shell integration.
Retaining shared or pre-existing Git must not retain newly owned shell settings.
The existing behavioral regressions remain unchanged.

### 2026-10-10 — Scoped settings, policy and migration integration

- Removed the Notes-vault checkbox: it installed no software. Neovim retains
  NOTES_VAULT and existing-vault integration, without WSL host-path guessing or
  automatic directory creation. New regressions failed before (three failures)
  and passed after (nine tests); the optional plugin stays installed but loads
  only when the configured vault exists.
- Integrated pinned Yamllint/PyYAML/pathspec private wheels and the actual shared
  Python preparation. Agent native macOS old-to-new update, YAML validation,
  stable-Python-generation change, check and removal passed; Linux/Windows native
  execution remains pending.
- VS Code's six settings reuse ProfileDriver with JSONC-aware byte edits.
  Real filesystem lifecycle tests pass for adoption, update, exact restoration
  around personal comments/CRLF/BOM/large numbers, and comments added after setup.
  Invalid JSONC and duplicate keys fail closed. Strict Pi JSON is unchanged.
- Sentinel embeds exact MIT-licensed upstream 0.1.2 generated bytes; its source
  reproduction, four private consumer paths, UTF-16 baseline roundtrips and real
  process-death recovery passed locally. An integration patch accidentally
  included reversal of concurrent VS Code changes; it was rejected before apply
  and regenerated from the worker's frozen baseline. The corrected intended
  hunks preserve both integrations.
- Migration uses the existing publisher and operation receipts, with explicit
  whole-profile consent and separately preserved readable link contents. Main
  dispatch now selects setup or migration from fixed wrapper entrypoints. The
  integrated focused race suite passed (11.987s); full public/native journeys,
  old-runtime retirement and final gate remain outstanding.
- Desktop ZIP links reuse delayed bounded archive link publication. Verified DEB
  bytes stream through the observed dpkg-deb only as a decoder; no maintainer
  scripts run. Hostile ZIP/DEB fixtures passed; all desktop package bytes and
  macOS signatures were checked, and actual Code discovered the pinned Rose Pine
  extension through its stable directory link. GUI/font proof is separate.

The actual compiled main-command migration → setup/adoption → update → removal
journey also passed on macOS (3.178s). It retained personal instructions, restored
the prior adopted Sentinel block, and did not replay historical migration over a
new shell profile. Windows public migration still needs its own disposable account;
HOME is not a known-folder override. Native Windows isolated publisher tests remain
part of the ordinary matrix. APT library health now checks native `dpkg --verify`
output as well as exit status: dpkg can report modified/missing files with exit 0.
The behavioral fixture verifies that these cases are unhealthy.


### 2026-10-10 — One Linux desktop recipe and native acceptance expansion

The same pinned Trixie Ghostty package, with a fixed private launcher for its
bundled GTK layer-shell library, opened real Xvfb windows on Debian 13 and
Ubuntu 26.04 arm64. Both fixtures deliberately omitted the system layer-shell
package and cleared incoming LD_LIBRARY_PATH. This evidence replaces a proposed
distro-specific archive selector and conditional dependency graph. The original
ELF is unchanged; declared resources remain under usr/share, and check/removal
passed offline with unchanged payloads. WezTerm also opened real windows on both.
Code's actual Debian arm64 CLI discovered the pinned Rose Pine extension and
passed loader, version, payload and offline-removal checks; this is not GUI proof.
Native amd64/Windows desktop execution and OS registration remain required.

The catalog now declares hidden APT runtime libraries from the downloaded native
package controls. New separate Neovim jobs exercise the compiled public command,
production configuration and existing strict parser/LSP/formatter smoke on all
four native OS/architecture runners. The fixture also keeps Pi working after
Neovim removal and checks final configuration restoration and personal editor data.
It is compiled locally only; actual native acceptance remains pending.

Archive releases/checksums remain deliberately reviewed maintenance inputs. Update
reconciles the versions pinned by the current checkout. Retire Renovate extractors
for deleted monolithic sources; retain real CI/Go/module dependency extraction.
Do not claim automatic archive-pin updates or substitute arbitrary latest downloads.

### Fresh vendor acceptance corrections (2026-10-10)

Run `38062524070` passed fresh Ubuntu and Debian APT on both architectures,
Windows VC++ runtime installation/use/removal, and fresh Homebrew. Two failures
were reproduced before fixing their cause. Apple's fresh-host fixture queried
only matching package receipts, so `pkgutil` returned nonzero on a valid Xcode-only
host with no CLT receipts. Enumerate the receipt database successfully, then forget
only CLT-prefixed receipts; an actual query error remains fatal. The Bash process
regression covers empty, mixed and failed receipt queries.

Visual Studio inventory collapsed packages with different versions or branches
into one identity. Microsoft's [vswhere serializer](https://github.com/microsoft/vswhere/blob/main/src/vswhere.lib/Formatter.cpp)
exposes six identity tokens. Preserve all six in the ownership map rather than
ignoring duplicate records. The actual PowerShell projection regression failed
for parallel versions and branches before correction; exact duplicate identities
still fail. Existing saved maps are still readable and cannot acquire ownership
through the expanded identity: any difference requires inspection.

Tag certification disables setup-go cache use; branch checks retain it. Homebrew
bootstrap pin evidence no longer depends on the retired install-deps shell script.
These local corrections still require successful fresh native reruns.

The same run's Windows core failure shares the Visual Studio identity cause for
compiler reuse and Rust. LaTeX's pip source build additionally returned WinError
267 while using a build cwd nested below its 207-character versioned payload.
Python preparation now lets pip create/clean its ordinary unique native temporary
build directories, keeping its private home and venv in the managed generation.
The long-path native fixture remains unchanged. PortableGit's silent SFX exit is
still open; added failure-only fixture diagnostics identify which upstream phase
actually ran. Upstream SFX source accepts Go's whole-argument quoting, so changing
quoting without native evidence was rejected.

### Desktop publication and font activation integration (2026-10-10)

Fixed desktop recipes publish macOS `~/Applications` links, Linux XDG application
entries, and Windows shortcuts under independently observed FOLDERID_Programs.
VS Code discovers the pinned theme through its stable extension link. No app
registry/context-menu framework is introduced. Twelve Hack faces use private
archive bytes and per-user native registration, preserving foreign fonts and
registry values. Configuration journals own publication/adoption/restoration.
Only the enclosing exact in-progress font operation may restore a completed file
journal when native registration remains unfinished; ordinary completed-journal
restoration remains refused. NativeDriver routes this recovery before generic
configuration recovery and finalizes the shared store only once.

The Debian 13 arm64 private native fixture passed all twelve font SHA/name/glyph
checks, rendered U+F120 through Pango and removed the owned registration/files.
Focused configuration/font tests and Windows compilation passed. Actual macOS
and Windows font activation and published GUI launch still require hosted proof.
The old `install-deps.ps1 -All` Tree-sitter diagnostic now points to the Neovim
selection, and the main README replaces the retired flag-based runtime guide.

### Windows Terminal scoped integration (2026-10-10)

Connected the official pinned 1.25 ZIP and fixed Start Menu launcher to the same
archive/configuration lifecycle. Scoped profile publication owns fixed leaves and
identified entries, preserving nested additions, comments, personal profiles and
any existing nonempty default. Bookkeeping is bound to the original receipt;
ownership fingerprints cover only owned values. Native-equivalent serialization
is accepted; ambiguous legacy combined bindings fail before mutation and require
Terminal's own migration. Store/Preview/Canary remain independent installations.
Focused local lifecycle/recovery/manifest integration passed (20.500s); official
archive validation and foreign compilation are source/layout evidence only.
Hosted native GUI settings consumption and lifecycle remain an explicit gate.

### Native Homebrew linkage timeout correction (2026-10-10)

Hosted run 38062524070 exceeded the shared 30-second budget while inspecting a
large affected formula set. Inspection now allows five minutes for linkage only.
A related defect discarded the context cause and treated interrupted root/consumer
checks as failed health, potentially authorizing repair. Cancellation/deadline
now propagates before any reinstall. Process-boundary and controller regressions
failed before and pass after; the actual 31-second read-only probe plus focused
race suite passed (37.704s), and the final suite with root guard passed (8.402s).
Substantive completed batch failure remains strict. No installed package was
mutated on the developer Mac; the production native lifecycle rerun remains due.

### Published desktop consumer acceptance (2026-10-10)

Added a compiled-public-command lifecycle selecting WezTerm alone, then the
applicable desktop set, with actual published .desktop/.app/.lnk launch, native
PID/window observations, VS Code settings API witness, terminal config/font
consumption, update/check/removal and personal-launcher restoration. Windows
WezTerm now passes managed PowerShell as the persistent default_prog; GUI PATH
cannot accidentally find runner PowerShell, and initial plus subsequent tabs
must report the exact managed executable. Focused tests passed (1.142s) and
Windows compilation passed. Hosted execution remains required. AeroSpace launch
waiting for explicit macOS Accessibility consent is not configuration/tiling
proof; only its exact connection-not-started response with the launched process
alive is classified as that boundary. Changed-version desktop upgrades are not
proved by same-pin Update reconciliation.

### Old runtime and gate retirement (2026-10-10)

Removed the Nix/chezmoi activation layer, monolithic installers/uninstallers and
their implementation-specific tests. The coverage map in
`docs/installer-test-retirement.md` identifies retained behavior and its new
proof; passive released live-link sources and published schema-1 release proofs
remain unchanged. Current gates use the native engine, three configuration jobs
and 20 native jobs. Required-context policy is checked in as pending live apply;
no GitHub protection has been changed. The initial cutover needs authorized
maintainer action after exact-head checks pass, since the exact-main safeguard
helper cannot unblock its own pre-merge policy transition. No fake success aliases.
Future release certification records exact native job/workflow evidence as schema
2 while validating historical schema-1 proof. Archive hashes remain reviewed
maintenance rather than misleading Renovate coverage. The frozen retirement
snapshot passed make ci in 310 seconds with unchanged hashes; final candidate
manifest regression also passed. The complete integrated writer still needs its
full gate and native hosted acceptance.

The first integrated gate exposed the old Windows Terminal keymap test still
matching legacy combined action/key syntax. Its failure was reproduced directly;
the guard now parses the canonical recipe and follows modern action IDs while
retaining the original Ctrl+W, close-tab and close-pane assertions. Duplicate key
chords also fail. The production pinned-archive test separately verifies bundled
Terminal action identifiers. This changes the test's schema, not its behavior bar.

The public desktop group also selects Windows Terminal on Windows and launches
its published Start Menu shortcut. Its separate native fixture verifies exact
managed-profile consumption and scoped settings restoration. Both layers are
required: provider-only proof cannot establish complete checkbox wiring.

### Native Windows public migration acceptance (2026-10-10)

Added a hosted-only fixture using actual known folders and Windows PowerShell
5.1 setup/migrate wrappers. It accepts only empty completed prior setup state,
uses a fresh approval for compiled-main mutation, preserves and discloses both
migration originals, installs/updates Starship plus all four Sentinel policies,
and removes them while preserving later personal CurrentHost/AllHosts content.
The fixture runs last because its migration history intentionally remains in the
disposable account. Related migration/Sentinel/installed-POSIX race tests passed
(13.547s); Windows compilation and vet passed. Actual Windows bootstrap/migration
execution remains pending the integrated hosted run.

Integrated static checks also caught omitted security-policy guidance in the
compressed README/agent guide and space indentation inside embedded font C#/
PowerShell. Restored the CodeQL/private-reporting/immutable-release guidance and
normalized the embedded whitespace under the existing Go EditorConfig rule; no
checker suppression or behavior change was introduced.

The integrated shell gate found the obsolete Herdr Homebrew-range helper's
documentation contract. It had no remaining installer caller. Removed that helper
and its implementation-specific mock test, and added actual installed Herdr
configuration consumption to the native public lifecycle. The new archive provider
already binds exact version and bytes; a historical mutable-formula range no
longer describes installation. Native acceptance must still run the new consumer.


### Windows font query stream boundary (2026-10-10)

The `56a48602f1aa349654bc06e42be050c7a90ad3d4` native desktop job failed at
`TestNativeFontInstallConsumeRemove` before installation: fresh absence inspection
returned Unknown/Pending with `invalid character '#' looking for beginning of
value`. `runIntegrationQuery` sent both process streams to its JSON buffer. A
read-only encoded PowerShell probe reproduced valid stdout JSON and successful
`#< CLIXML` progress on stderr; the hosted log does not retain raw streams, so the
precise implicit progress-producing cmdlet is not independently established.

FIXED in code: font inspection returns stdout as protocol data, keeps stderr as a
separate bounded diagnostic, preserves nonzero exit identity and sanitized error
detail, and rejects combined output above 1 MiB. No prefix filtering, progress
preference override, native-error suppression or ownership change is involved.
The alternate explanation of malformed stdout remains protected by a regression
that still rejects it; valid JSON from a failed process is likewise never accepted.

The portable subprocess regression and the actual read-only PowerShell 7.6.3
encoded-command probe both failed before with the same JSON error and pass after.
The focused font suite passed (8.684s), including per-stream/combined output bounds.
The read-only PowerShell regression also selects Windows PowerShell 5.1 on Windows.
No native font mutation ran on the Mac; Windows compilation is a separate check,
and the hosted Windows install/consume/remove fixture must still be rerun.

## Hosted product acceptance diagnostics, 2026-10-10

Checkpoint `56a4860` passed the complete local gate and Windows compilation with
unchanged hashes, then exposed application-level failures in hosted run
38066616995. Fresh Homebrew, both Ubuntu/Debian architectures and fresh Windows
VC++ passed. Failures remain open for Linux selected-library transitive changes,
Windows font query protocol, optional Obsidian lock validation, Visual Studio
package multiplicity, Apple fresh-host receipts and macOS desktop launching.

The macOS desktop fixture now captures LaunchServices child stdout/stderr in its
own disposable directory and reports bounded output plus bundle signature checks
on failure. It still requires the published launcher to succeed and an owned
native window to appear; no direct-binary fallback or security-policy bypass was
added. The current -10810 error alone does not identify the cause.

The first full Opus invocation used observed `claude-opus-5-5` and requested xhigh,
but stopped after five reads when it tried to inspect outside the supplied source
root. Snapshot hashes stayed unchanged. This is not review approval; the next
brief must give all required evidence inside that root and state the boundary.

### Windows Visual Studio package multiplicity (2026-10-10)

Native Windows Neovim inspection on `56a48602` failed because the adapter rejected
a repeated package identity. The earlier six-token correction still assumed a
set. Microsoft's [GetPackages contract](https://learn.microsoft.com/en-us/dotnet/api/microsoft.visualstudio.setup.configuration.isetupinstance2.getpackages?view=visualstudiosdk-2022)
returns an array, without promising uniqueness. The [reviewed vswhere formatter](https://github.com/microsoft/vswhere/blob/44ff88ff4dd03f718493152b9d59c6436dc5d55a/src/vswhere.lib/Formatter.cpp#L280-L355)
also emits the optional extension flag. Rejecting repeated rows or discarding
them is therefore not a justified inventory policy.

The adapter now records exact seven-field tuples with occurrence counts, retains
ordinal case distinctions and canonicalizes row order at the Go boundary. Both
ownership fingerprints and the final PowerShell execution guard compare counts;
no duplicate is discarded. New operation journals use schema 2. Old schema-1
maps are refused with instructions to preserve the instance and original journal
for explicit recovery because their lost multiplicity cannot be reconstructed.

The actual production PowerShell projection failed before the correction on
repeated rows and passes afterward. Focused race tests also cover ordering,
extension/case variants, count bounds, changed-count removal refusal, worker
rechecks and old-shape preservation. Native Windows acceptance remains required;
the original log did not include the colliding vendor rows, so their specific
identity is not claimed to have been observed.


### Apple CLT receipt enumeration and fresh-host evidence (2026-10-10)

FIXED: native macOS `pkgutil --pkgs=^dotfiles[.]certainly[.]absent$` returns exit 1
with empty stdout/stderr. Production inspection and both guarded shell checks
incorrectly required that regex no-match to succeed. They now require successful
`--pkgs` enumeration and exact line membership for
`com.apple.pkg.CLTools_Executables`; unrelated and prefix/suffix lookalikes do not
block absence. Real Bash/native-worker boundaries with the correct exit behavior
and Go fresh lifecycle failed before and pass after. Existing receipt, racing
receipt, enumeration failure and selected/broken developer-tool protections remain.

The `56a48602` hosted preparation printed NSPosixErrorDomain Code 2 and “Forgot”
for CLT receipts, but the production preservation guard still observed the
Executables receipt. A separate private subprocess fixture reproduced that false
preparation success and now rejects it. Preparation re-queries receipts immediately
after forgetting, before relocating any Xcode/CLT payload, and reports the first
persistent receipt's metadata, SIP status and bounded standard receipt-path flags.
No package files or protected receipts are deleted by this correction.

Read-only local inspection found CLT receipts in
`/Library/Apple/System/Library/Receipts` carrying `restricted` flags with SIP
enabled, and none in `/var/db/receipts`. This supports protected receipt storage as
a concrete alternative to a move-order/cache explanation; the hosted log lacks
those details, so the exact hosted cause remains unverified until its diagnostics
rerun. Apple's installed `pkgutil(1)` manual explicitly says receipt locations can
change and must be queried/modified through pkgutil. Moving or deleting receipt
files, disabling SIP, treating “Forgot” as absence or ignoring an existing receipt
are rejected fixes. A genuinely absent disposable image may be required.

Focused Apple CLT tests passed (3.471s), all five preparation tests passed, and
shellcheck passed the actual fixed recipe and hosted preparation. Cross-compilation
is a separate check. No CLT, Xcode, package receipt or SIP mutation ran on the
developer Mac; fresh native installation/SDK consumption remains outstanding.

### Native Windows quality-fixture corrections (2026-10-10)

The `56a4860` Windows quality job failed three Go fixtures before the entire
package reached its default ten-minute timeout. Desktop publication correctly
encoded Windows separators, but its assertion required a POSIX suffix. The
escaping and font-prerequisite fixtures supplied `/tmp` or `/usr/bin` paths that
Windows correctly rejects as non-absolute. Fixtures now use native temporary
paths and retain their escaping, baseline restoration and prerequisite-removal
assertions. This is a fixture correction, not a widened production path policy.

POSIX Python subprocess fixtures had invoked Windows' system `bash.exe`, which
is a WSL launcher, and generated platform-default line endings/paths. They now
select Git Bash explicitly on Windows, use MSYS drive paths and UTF-8/LF shell
fixtures, and preserve Linux archive executable modes independently of Windows
chmod. They continue to execute the real checkout wrappers and bootstrap with
all checksum, cache, argument, mutation and exit-status assertions. No test is
skipped to obtain a pass. Windows' quality checkout now fetches full history,
matching the other hosts, so the migration generators can verify every released
tag instead of failing on missing Git objects.

The native log shows the full Go package progressing into the Visual Studio
inventory tests at the 600-second package deadline. The suite now has an explicit
20-minute total budget; the enclosing quality/core job budgets accommodate their
other real lifecycle steps. Individual native mutation timeouts remain bounded.
The original Windows log is the failed-before evidence. Focused host tests and
Windows compilation cannot substitute for the required native rerun.

### Neovim notes provisioning versus activation (2026-10-10)

The `56a48602` macOS and Linux arm64 native lifecycle jobs both failed strict
verification because `obsidian.nvim` was absent. Its Lazy `cond` excluded it
during private-HOME provisioning. Skipping this lock check was rejected: a real
vault would then trigger first-use installation outside the reviewed runtime.
The plugin is now always provisioned at its lock; the existing-vault guard is
inside configuration activation. Strict `sync_check.lua` remains unchanged.

A disposable private Neovim 0.12.5/locked Lazy proof reproduced the missing plugin
before the change and provisioned its exact locked commit afterward. Real
Obsidian consumer probes then passed both absent-vault/no-create and existing-vault
activation, using the locked Plenary and completion dependency. The native public
Neovim fixture now runs those same probes and preserves a personal note through
removal. Focused notes tests pass; the final four hosted full lifecycle jobs must
still pass. This proof used no personal vault or host package installation.

### Native Windows pinned-checkout fixture paths (2026-10-10)

The Windows quality job on `56a48602` reached the real checkout proof, but its
fixture compared the normalized result with the raw mixed-separator input path.
The helper intentionally normalizes its target. The assertion now normalizes the
expected path as well; production code and all identity, repair, concurrency and
failure-cleanup assertions remain unchanged. The 11 focused checkout cases pass
on private Neovim 0.12.5 on macOS. Native Windows execution is still required.


### APT selected incidental libraries and retained roots (2026-10-10)

Accepted the hosted `56a48602` Linux desktop failure: installing libxcb-image0
introduced the later selected libxcb-util1, then the executor rejected its own
proved change as drift. The same controller failure was reproduced locally with
native command/history/dpkg boundary fixtures before modifying production code.
The package relation is declared by the [Debian package metadata](https://packages.debian.org/trixie/libxcb-image0).

The executor now accepts an originally absent selected artifact only when its
provider proves introduction by an earlier completed operation in the exact active
plan. APT transfers that exact package from its owned incidental pool to a root
through a recoverable saved claim; it performs no reinstall or apt-mark operation.
The original absent receipt baseline remains intact. Updates of already automatic
roots preserve their classification through saved APT `--mark-auto` plus
`--only-upgrade`: the former protects real upgrades, while the latter prevents
APT's already-current install branch from marking the root manual. The switches
are defined by the [APT manual](https://manpages.debian.org/trixie/apt/apt-get.8.en.html),
and the no-op distinction is explicit in [APT's TryToInstall source](https://sources.debian.org/src/apt/3.3.3/apt-private/private-install.cc/#L1352).

Native reverse consumers are excluded from external-consumer disclosure only
when their original operation and current source/classification still prove
ownership. Native dependency narrowing remains authoritative. A removed root
needed by another owned native package can be released into the pool, including
an originally manual root, without inventing physical absence. The observation's
optional `removal_deferred` proof requires the same saved remove operation and an
exact retained pool disclosure. Later removal of the final consumer collects the
eligible pool and clears stale receipts. Reselecting a released root reacquires
its exact still-owned package without reinstalling, while preserving the original
absent baseline; personal source/manual changes remain unowned. Outside consumers, holds, manual
promotions, changed sources, unrelated orphans and native dependency refusals
remain protected. The previous failed-removal recovery for a newly protected
incidental dependency remains covered and passing.

Rejected a static catalog mirror of APT transitive dependencies: it duplicates
native resolution, becomes distribution-sensitive and cannot faithfully encode
native cycles as installer DAG edges. Rejected blind retry and broad drift-guard
relaxation: matching bytes or historical ownership does not bind the approved
transaction. New optional intent/observation fields preserve legacy decoding;
old saved native commands retain their exact recipe. Interrupted claims and
retained-root publication are replayed from saved evidence without native reruns.

Focused controller/provider tests cover install, update, selective/full removal,
manual-root retention, outside/source/classification consumer changes, foreign
installation during an approved install, incomplete original claims, corrupted
pool/removal proof and legacy observations. This is deterministic boundary proof;
actual Linux amd64/arm64 desktop acceptance remains a required hosted rerun.

Validation: the fresh original-snapshot controller reproduction fails with the
same hosted diagnostic; the final 80-test focused race suite passes (19.344s),
including existing engine/planner/recovery, APT/native-pool and Homebrew approval
regressions. `go vet ./...` and Linux amd64/arm64 plus Windows amd64 test-binary
compilation pass. No native package command ran on the developer workstation.

### LaTeX console UTF-8 on native Windows (2026-10-10)

The Windows core job on `56a48602` installed the real pinned converter, then
failed converting `\alpha`: upstream printed Unicode through cp1252 stdout.
Setting UTF-8 only in a test or a Neovim environment would leave ordinary managed
CLI use broken. The fixed LaTeX preparation now uses the existing pinned pip
[console-script writer](https://distlib.readthedocs.io/en/latest/tutorial.html#specifying-a-custom-script-template)
to regenerate the same native entrypoint with explicit UTF-8 standard streams,
using Python's [stream reconfiguration API](https://docs.python.org/3.13/library/io.html#io.TextIOWrapper.reconfigure).
It delegates conversion and arguments to upstream pylatexenc; no dependency,
global Python setting, custom binary launcher or general recipe framework is added.

Console revision 1 is part of the desired LaTeX pin identity. Completed old
records without the field retain their identity and can follow the ordinary
archive update path. New execution of an unfinished revision-0 preparation is
refused with instructions to preserve its operation and payload for recovery.
Unknown revisions are rejected. The archive schema and lifecycle are unchanged.

A real private pinned-Python lifecycle on macOS reproduced the identical cp1252
failure before the correction (21.28s) and passed afterward (20.43s), with
`PYTHONIOENCODING=cp1252` and `PYTHONUTF8=0`. It converted LaTeX, consumed literal
UTF-8 stdin after switching the stable Python generation, checked unchanged and
removed the package. Focused race tests cover old-shape update, invalid recipes,
launcher-preparation failure, read-only checks and all four desired target pins;
the existing yamllint preparation tests also pass. Native Windows execution of
the corrected console remains a required hosted gate, not a claimed local result.


### Windows core boundary diagnosis (2026-10-10, `56a48602`)

Retrieved the complete raw log for run `38066616995`, job `114255442445`.
VC++ registration/signature passed (4.49s); Yamllint install/update/runtime/removal
passed (131.51s). Build Tools and Rust hit the known duplicate-package identity
rejection. LaTeX reached its actual converter but Python cp1252 output could not
encode alpha; that correction is separately assigned. The native unit stage's
three path/fixture failures and aggregate 600-second budget are separately assigned.

PortableGit's baseline SFX exited 1 after creating `etc/post-install`, with no
Git Bash, post-install batch, Bash or Git executable. Its payload root was 204
characters long. This proves partial extraction, not an argument parser or
post-install child failure. Upstream's matching current release SFX parser accepts
Go's whole-argument quoting; no speculative quote rewrite is justified. The
[upstream SFX makefile](https://github.com/git-for-windows/7-Zip/blob/v26.03-VS2022-sfx/CPP/7zip/Bundles/SFXSetup/makefile)
sets `Z7_NO_LONG_PATH`, and its `-y` branch suppresses the error dialog. That is
supporting evidence for a path-limit hypothesis, not an observed native error.

Added hosted failure-only diagnostics: reverify the same archive digest, launch
it without `-y` in a fresh equal-length sibling, accept only the exact prefilled
fixture directory in its own PID's dialog, and capture its extraction error.
A second fresh equal-length sibling uses Microsoft's documented `\\?\` output
prefix and inspects required files, post-install remnants and the real runtime
probe. Both remain inside the existing disposable fixture. Neither changes the
production recipe, shortens a path, changes registry policy, touches other windows,
nor turns the required lifecycle failure into success. Actual dialog/error and
extended-path results remain unverified until the hosted rerun.

The public migration fixture failed at 300.26s in its initial `setup.ps1 check`,
after only the bootstrap download-start message. Its per-call five-minute context
expired before migration writes; download, extraction and compiler work were not
separately observable. Added bootstrap phase start/end/failure timing on stderr and
explicit fixture context-error reporting. The timeout stays unchanged. New Pester
phase-output/error tests fail against the previous script and pass after correction;
focused PortableGit tests and Windows cross-compilation pass. The embedded dialog
C# compiles locally without invoking Windows APIs. No native Windows mutation ran
on the Mac and none of these checks is a native lifecycle pass.

### macOS WezTerm launch diagnosis remains open (2026-10-10)

The `56a48602` desktop job failed opening the published WezTerm bundle with
LaunchServices `-10810`. Apple's SDK identifies this only as `kLSUnknownErr`;
it does not establish a root cause. Read-only inspection reconfirmed the pinned
ZIP checksum, real 0755 universal `Contents/MacOS/wezterm-gui`, matching
`CFBundleExecutable`, and Apple-system-only dynamic library paths. Publication
links the intact bundle, without moving its executable outside it.

Rejected causal claims: upstream's [malformed Nix bundle issue](https://github.com/wezterm/wezterm/issues/6731)
concerns executables outside their application bundle and does not match this
layout. The pinned [environment bootstrap](https://github.com/wezterm/wezterm/blob/20240203-110809-5046fc22/env-bootstrap/src/lib.rs#L219-L223)
clears inherited `SHELL` before account-shell discovery, so stale `SHELL` is not
supported as this failure's cause. The pinned [GUI startup](https://github.com/wezterm/wezterm/blob/20240203-110809-5046fc22/wezterm-gui/src/main.rs#L394-L408)
logs socket-listener creation failure and continues; a long socket path alone
therefore does not establish a fatal launch error. These exclusions are source
evidence, not a claim that native GUI acceptance passed.

The existing disposable native fixture now adds a failure-only log query for
WezTerm's failed launch or missing window. It admits only `lsd`/`runningboardd`
messages containing that published bundle path or the exact pinned application
identifier, limits history to two minutes and combined output to 64 KiB, and
uses a ten-second deadline with a two-second pipe-wait bound. Permission errors
remain visible; there is no elevation, retry or broad system-log dump. A later
unrelated test failure does not trigger this query for a successful launch.
No local GUI or native installation was performed. The next hosted failure's
child output, signature check and scoped system diagnostics must distinguish
launch denial from an early application crash before any production fix.

### Correction-batch integrated gate (2026-10-10)

The integrated gate passed the full Go race suite (432.689s) and all 35 bootstrap
tests, then reported two harness integration defects. The shell bootstrap wrapper
invoked a Python file directly, losing the repository package import root; it now
changes to the repository and invokes both suites through unittest modules. The
new PowerShell diagnostic test reparsed source text through ScriptBlock.Create,
which the remote-execution scan rejects. The helper now uses an ordinary body
param block, so its test can use the already parsed function body's GetScriptBlock
and function provider directly. No scanner exception or suppressed finding was
added. The shell wrapper passes all 18 cases even from outside the checkout;
all five Pester cases, script analysis and the unchanged remote-execution scan
pass. The next integrated gate remains required. The raw failed gate remains
preserved as before evidence.


### Remove the superseded compiler/Make ordering workaround (2026-10-10)

The earlier APT integration entry above and the original assertion in commit
`3842e840` explicitly required Make before build-essential only because the
latter installs it transitively, consistent with the
[Debian build-essential dependency metadata](https://packages.debian.org/trixie/build-essential).
That is now handled by operation-proved native
pool claims, so the Linux `tool.compiler -> tool.make` catalog edge and its
implementation-order assertion are removed. The compiler still binds to
build-essential and checks `cc`; Make still has its own binding/check. Neovim's
real `nvim.sync` requirements continue to include both tools directly.

Before removal, the focused controller fixtures demonstrate the workaround's
extra selected Make root and inability to release that root while keeping the
compiler. After removal, compiler-only setup attributes Make to its native pool;
fresh combined setup claims the incidental Make root in the same approved plan;
Make-first setup preserves its already-owned manual root. Both orders pass update,
selective Make release, compiler survival and final exact native removal. Tests
reuse the existing APT history/dpkg boundary fixture; no native manager ran locally.

Validation: the original-edge controller regressions fail before the change and
pass after it. The focused APT/native-pool/catalog/prerequisite race suite passes
(13.586s), along with `go vet ./...` and `git diff --check`. Hosted native compiler
and Neovim execution remains part of the integrated acceptance gate.


### Apple CLT payload absence contract correction (2026-10-10)

The receipt-veto premise in the earlier fresh-host diagnosis is superseded.
[Apple's installation guide](https://developer.apple.com/documentation/xcode/installing-the-command-line-tools)
defines uninstall by removing the CLT payload and explicitly allows receipts to
remain for updates; [TN2339](https://developer.apple.com/library/archive/technotes/tn2339/_index.html)
also names the directory as the uninstall boundary. The read-only Opus 5.5
consultation recommended that distinction. Surrounding source inspection confirmed
that existing prerequisite queries clear developer/SDK overrides and resolve real
executables without invoking compiler shims on an unselected host.

Absence now carries a strict canonical receipt identity and selection fingerprint.
Both must match the approved observation, saved schema-2 intent and native recipe;
query failure, malformed identity, occupied payload, foreign selection and drift
still refuse mutation. The native command rechecks after discovery and emits
installation evidence only with executable payloads and a changed install-time.
Completion additionally requires matching operation-bound receipt evidence,
selection and compiler/Make/SDK health. Finish-mode recovery rechecks that exact
installed receipt. Schema-1 intents are preserved and explicitly refused by this
recipe; no automatic journal rewrite or extra repair mode was added. Preview uses
the existing HealthIssue/reason channel to disclose historical receipt identity,
Apple's receipt update, global selection and retained infrastructure.

Historical-receipt absence and unexpected selection-query failure both fail on the
prior implementation; no-receipt installation is the passing control. Corrected
Go tests cover those boundaries, no-receipt/historical installation, changed-time
completion, approval and install/finish recovery drift, malformed/query errors,
legacy intent refusal, existing tools and unknown results. The real Bash worker
covers historical receipt success, discovery drift, unchanged receipt refusal,
query errors and no advertised package. Native mutations did not run locally.

Hosted preparation now reads receipts and validates every listed CLTools payload
member before moving tools, allowing the normal `.`, `Library` and
`Library/Developer` ancestor entries. No receipt is forgotten or deleted. A
preparation-phase native fixture proves healthy selected tools are reused without
ownership or dispatch. The install fixture verifies receipt-set preservation,
moved payload root metadata and representative payload hashes, sibling metadata,
Homebrew executable/metadata and a compiled SDK program. This is bounded
preservation evidence, not a full-machine file hash. The canonical job is renamed
`Missing Apple developer tools (macOS arm64)` consistently with required-check
metadata and safeguard scripts. Historical fresh-host evidence is not relabelled.
The next disposable native run must establish what Apple actually advertises and
installs. A same-version reinstall and a never-installed Mac remain unproved.

## 2026-10-10 — Owner-directed all-platform WezTerm retirement

Source of truth: the owner explicitly requested complete WezTerm removal from
all OSes. This retires the product requirement; it does not resolve or relabel
the failed Linux/macOS launch evidence. Removed the four active catalog resources,
four archive targets, configuration and desktop routes, active Lua configuration,
exclusive Windows default-shell argument code and dedicated smoke tests/CI step.
No active Homebrew binding or shell PATH hook existed. The public desktop fixture
retains singleton, update/check, full-group use and restoration proof through
VS Code; consumer subtests release their private witness extension after each use.

Pruned exactly eight no-longer-consumed hidden APT resources: libgcc-s1, libssl3t64,
libwayland-egl1, libx11-xcb1, libxcb-image0, libxcb-util1, libxkbcommon-x11-0 and
zlib1g. Shared fonts, PowerShell, shell integration and remaining libraries stay.
The libxcb-image0 → libxcb-util1 native dependency remains explicit test fixture
data so retirement cannot erase same-plan claims, selective/full removal, resume,
reselection or changed-outside-consumer regressions.

Preserved passive `home/dot_config/wezterm/wezterm.lua` and the exact-release
`legacy-config-targets.json`/generator byte-for-byte. They keep old live links and
historical discovery readable, and authorize no new install or deletion. Existing
released installation/migration/manual records are historical and remain intact.
No current-user package, configuration or application was changed.

Validation: the retirement selection/publication regression fails against exact
`fbca6c2` because WezTerm is still selectable, then passes with this change.
Focused Go race tests (APT, desktop, catalog/config, archive pins and policy) pass
(15.853s), as do vet, all static checks, six released-inventory tests and Linux
amd64/arm64 plus Windows amd64 compilation. Official Renovate 44.138.0 extraction
under Node 24.21.0 regenerated the same 63-record inventory byte-for-byte; archive
pins are manually reviewed and outside that inventory. The released WezTerm
source, released inventory and generator remain byte-identical. Native GUI
execution remains hosted-only and is not claimed by local checks.


### Apple preservation size and Terminal junction fixture corrections (2026-10-10)

Published `fbca6c224d03ac57597628464ba0bf92e94f2df7` exposed two acceptance
fixture defects. Apple job `114268787949` failed before moving payloads or invoking
an installer: its existing CLT `clang` exceeds the fixture's 256 MiB hash cap.
The helper now gives the existing streaming snapshot routine the fixed file's
observed size, retaining its one-entry bound and concurrent-change checks. A
private sparse-file regression fails with the original cap, then passes and
detects a byte changed beyond that boundary. No global size limit or production
absence contract changes, guessed larger cap, or native workstation mutation.

Windows Desktop job `114268788056` passed font lifecycle, then the Terminal
fixture failed with bare `The system cannot find the path specified.` after its
valid PowerShell witness JSON. The exact Go 1.27.1 source explains the error:
`os` classifies mount-point junctions as irregular without directory/symlink mode;
`filepath.walkSymlinks` returns bare `syscall.ENOTDIR` for the intermediate
`current` junction, and Windows aliases that errno to `ERROR_PATH_NOT_FOUND`.
The fixture now uses the existing handle-based `resolveConfigPath` and reports
path context on failure. The native junction regression also resolves a regular
file beneath a long junction path. The six-call search found no other managed
Windows junction descendant passed to `EvalSymlinks`: the other calls use plain
temporary roots, fresh extracted symlinks or POSIX-only paths.

The Apple private regression runs locally without modifying developer tools.
Windows cross-compilation checks the correction's build, not native success;
the hosted junction regression and configured Terminal window/exit/lifecycle
still require rerun. Apple package advertisement and installation likewise remain
unproved. These changes do not reinterpret either failed native job as a pass.


### Windows PowerShell bootstrap download progress (2026-10-10)

Core Windows job `114268788046` at `fbca6c2` locates the five-minute public
bootstrap failure inside the pinned Go download: `Bootstrap download: started`
is the last phase, followed by explicit context deadline exhaustion. Archive
verification, extraction, compiler execution and migration were not reached.
The log alone does not distinguish network delay from progress overhead.

[PowerShell upstream issue 2138](https://github.com/PowerShell/PowerShell/issues/2138)
documents the exact Windows PowerShell 5.1 mechanism, including a same-file
measurement of 6.07 seconds with progress suppressed versus 371.66 seconds with
default progress. Its later discussion distinguishes the PowerShell Core fix
from still-affected Windows PowerShell and recommends `SilentlyContinue`.
The bootstrap already uses basic parsing, so the newer web-parsing confirmation
is not a plausible explanation for this command.

The download now saves/restores `ProgressPreference` in a `try/finally` and
suppresses only its per-byte progress. Explicit stderr phase timings, errors,
TLS 1.2, pinned official URL, size/hash verification and the acceptance deadline
remain unchanged. Two process-boundary download mocks fail on the original
preference and pass after the correction, including exact error propagation and
preference restoration. PowerShell syntax/analyzer and Windows compilation pass;
local PowerShell 7 checks do not prove Windows PowerShell 5.1 transfer performance.

The hosted fixture records successful phase timings. If the same download phase
still exceeds its deadline, a failure-only paired probe gives default and quiet
progress separate one-minute transfers of the same checked-in official pin,
using private files, bounded output and exact successful-download checksums. It
cannot populate the real cache or turn the original lifecycle failure into a pass.
Native corrected download timing and migration completion remain pending rerun.


## 2026-10-10 — Git Bash bootstrap fixture corrections

Current-head Windows quality job `114268788341` passed Go race/vet, PowerShell
and Neovim checks, but eleven POSIX bootstrap assertions failed. Seven reached
the real Windows `uname` instead of the boundary double; four compared `/tmp`
with its equivalent drive-qualified directory. The [official Git wrapper
contract](https://gitforwindows.org/git-wrapper.html) confirms that
`Git/bin/bash.exe` prepends its own tool directories. This is fixture behavior,
not a reason to accept Windows in the POSIX production bootstrap.

The fixture now installs its PATH doubles inside initialized Bash. A real
subprocess wrapper that prepends a conflicting `uname` fails against the old
launch boundary and passes with the correction. Public-entrypoint fixtures
compare native resolved working-directory identity, preserving argument,
entrypoint and exit-status assertions. All 38 bootstrap tests pass locally with
PowerShell available; `gitbash-fixture-wrapper-red.log`,
`gitbash-fixture-bootstrap-green.log` and `gitbash-fixture-verification.json`
record the evidence. Actual Git Bash confirmation still requires the hosted
Windows rerun. No production launcher or supported-platform boundary changed.


### Native Neovim cache and Windows inspection corrections (2026-10-10)

Exact candidate `fbca6c224d03ac57597628464ba0bf92e94f2df7` reached a ready
Neovim installation on both Linux architectures, then normal editor startup
failed at Neovim 0.12.5 `vim/loader:135` with `ENAMETOOLONG`. The pinned Lazy
entrypoint aliases its bytecode cache to `vim.loader`; there is no independent
Lazy bytecode cache on this Neovim version. The loader URI-encodes the complete
source path into one cache filename. The published command's short `current`
link does not shorten Neovim's resolved physical runtime path. The previous
stage-only cache exception therefore did not cover ordinary use.

The optional bytecode cache is now disabled for every startup. Plugin lazy
loading remains enabled. A real pinned Neovim normal-init regression loads a
module under valid nested directories whose encoded cache filename exceeds
300 bytes, proves DAP UI remains unloaded until requested, then requires it and
proves Lazy activated it. The unchanged production configuration fails this
regression with `ENAMETOOLONG`; the correction passes along with the existing
800 ms macOS startup budget. The harness now places the real configuration at
`stdpath('config')`: Lazy resets runtimepath there, so only passing a different
`-u` file had allowed its cache to hide an empty configuration search path.
The highest complete installed-application lifecycle proof remains the hosted
rerun; this focused proof does not claim LSP/parser/Mason lifecycle completion.

The fresh Windows Build Tools job installed and verified its toolchain, then
failed decoding compiler-environment JSON at the native fixture's line 83 with
a leading `#`. Its fixture merged stdout/stderr although the production query
already separates them. Encoded PowerShell progress really emits `#< CLIXML`
on stderr: a private PowerShell reproduction makes the legacy combined capture
fail JSON decoding and the separate stdout capture pass. Raw hosted streams
were not retained, so that specific hosted output remains an inference. The
native fixture now directly reuses `queryWindowsVendor`. Process-boundary
regressions preserve successful JSON, reject malformed stdout and retain a
nonzero exit plus sanitized diagnostic; an actual encoded PowerShell progress
probe also passes. No parser strips arbitrary prefix bytes or silences errors.

The Windows Neovim job separately rejected a changed reviewed plan before any
mutation. Its log did not identify the changed field, so no provider or approval
policy is changed. Only a failing Windows native apply now logs its reviewed
plan and one fresh read-only preview through the same compiled executable and
environment. The preview clears `ExpectedPlan`, preserves the requested mode,
has a 90-second deadline and caps each logged plan at 64 KiB. A process-boundary
test proves exactly one preview reaches the child with no approval, while the
original request remains intact. The fresh observation is explicitly later than
the failed instant; it is diagnosis, never an automatic apply retry.


Validation for this bounded batch: real pinned Neovim normal-init regression
fails before and passes after; both startup cases pass. The focused Go race suite
passes (7.508s test time), including the native-query streams, package bag,
compiler-environment, Neovim sync and failure-preview boundaries. `go vet ./...`,
Windows amd64 test-binary cross-compilation, StyLua and `git diff --check` pass.
Windows native execution and the complete installed Neovim lifecycle still need
the next hosted run. WezTerm diagnostics added during this investigation were
withdrawn before handoff after the user removed WezTerm from product scope.


### F3: provision Mason's Linux download and ZIP prerequisites (2026-10-10)

The full-delta review correctly found that the Neovim graph relied on utilities
preinstalled on hosted Ubuntu images. At the locked Mason commit
`16ba83bfc8a25f52bb545134f5bee082b195c460`,
[`fetch.lua`](https://github.com/mason-org/mason.nvim/blob/16ba83bfc8a25f52bb545134f5bee082b195c460/lua/mason-core/fetch.lua)
tries curl, then wget; its platform fallback is Windows-only.
[`managers/std.lua`](https://github.com/mason-org/mason.nvim/blob/16ba83bfc8a25f52bb545134f5bee082b195c460/lua/mason-core/installer/managers/std.lua)
requires the `unzip` executable for Unix ZIP/VSIX payloads. Existing system tar
and gzip are part of the supported Debian/Ubuntu base; availability of curl or
unzip is not. The graph now declares hidden Linux-only APT `tool.curl` and
`tool.unzip` dependencies before `nvim.sync`, using the existing native ownership,
approval and removal contracts. Curl has an exact `/usr/bin/curl --version`
health probe; unzip reuses its existing `/usr/bin/unzip -v` probe. Neither adds a
checkbox or changes macOS/Windows requirements.

The focused regression fails against the original catalog for both Linux
architectures and passes after the correction. It checks hidden machine/shared
bindings, synchronization ordering and executable health, and confirms the
prerequisites do not leak into other platform selections. The existing complete
native Neovim fixture now requires curl/unzip package and executable absence on
an explicitly fresh Linux host, verifies created healthy APT receipts after
installation, and requires absence after its existing update, selective removal,
final removal and personal-data assertions. It never removes native packages to
manufacture the baseline.

The existing Fresh Debian APT and Fresh Ubuntu APT matrix jobs additionally run
that complete fixture, preserving the canonical check names. Each fixture has an
80-minute test deadline; the job budget becomes 110 minutes to also retain the
existing 20-minute native APT tests and setup. Bootstrap still installs only CA
certificates and the C compiler headers/toolchain; it does not install curl or
unzip. This replaces the rejected assumption that hosted runner availability
proves a complete fresh-host dependency graph. The focused race suite and vet
pass. Disposable local arm64 container results are recorded below when complete;
hosted amd64/arm64 certification remains required at the integrated head.

At this patch freeze, both private arm64 containers have demonstrated real curl
and unzip absence and are running the complete selected graph. The first Debian
attempt stopped at a TCP connection refusal from nodejs.org before sync; its raw
failure is retained separately. A new disposable Debian attempt and the Ubuntu
attempt remain running. No native pass is claimed by this entry. Their immutable
source snapshot contains this catalog correction plus the separately accepted
universal Lazy cache correction, needed for normal startup at the genuine long
managed runtime path.


### Failure-only macOS Node archive evidence (2026-10-10)

On exact candidate `fbca6c224d03ac57597628464ba0bf92e94f2df7`, run
`38071197318` failed the macOS Neovim initial apply after all operations.

The macOS Neovim public apply reached final verification and rejected Node/npm.
The archive provider does not run npm for that observation: it checks pin, link
and payload identity. Both Linux initial Neovim applies passed this boundary.
Pinned Mason's npm manager uses local staging for `npm init`/`npm install`; no
source evidence supports a speculative global-prefix change. A failure cleanup
now reports the Node observation, desired/saved pin digests, actual/expected
current link, and actual/saved payload snapshot. It downloads nothing unless the
payload differs. Only then does it checksum-verify the exact saved Node archive
into fresh fixture scratch, require its snapshot to equal saved publication, and
report up to 40 entry differences with clipped paths, hashes and modes, never
file contents. A mismatching reconstructed baseline refuses an authoritative
diff. Download deadline is three minutes; extraction/inspection retain existing
archive byte/entry limits. No Node/npm execution or installed-artifact mutation
is part of this diagnostic.

Focused tests cover unchanged/pointer/pin Node cases without re-download,
content/addition/removal diffs, corrupted saved baseline refusal, and unchanged installed evidence/integrity verdict afterward.
The hosted failures are the before evidence; these fixture corrections identify
the missing information rather than claim to fix the native failure.


Validation: the final Node-only focused race checks pass. The earlier focused
archive/desktop/Neovim diagnostic race suite passed (18.65s).
`go vet ./...` and Linux amd64/arm64 plus Windows amd64 cross-compilation pass.
A separate private macOS archive-only probe downloaded the exact pinned Node
24.21.0 archive, then ran npm `version --json`, `init --yes --scope=mason`, and
`install` in an empty project with a private HOME/TMP/cache/userconfig and explicit
PATH. The Node entry snapshot stayed identical after all three commands (25.78s).
No dependencies, scripts, OS packages, fonts or personal paths were installed or
changed. This excludes those npm startup steps in isolation, not package install
scripts, the full Mason closure or the hosted Node failure. Raw probe source and
output are preserved with the delivery artifacts; the temporary probe is not a
new production recipe or permanent test lane. Hosted diagnostics remain required.


### Full-delta Opus review and integrated corrections (2026-10-10)

The real read-only CLI review completed for `ae9a644..fbca6c2` with observed model
`claude-opus-5-5`, 168 file/search calls, exit 0 and no API/permission errors.
Requested xhigh effort is unexposed and remains unverified. The review's SHA-256
is `93d2d4405d51989956b9324371f93b9b121c2927c1601ee060b13f0762aa3121`;
all 3,107 snapshot files remained unchanged. It identified F1–F8 and verification
gaps; it did not approve runtime acceptance. Its pending-run observation is now
superseded by completed exact-head installer run `38071197318` (9/20 pass) and
quality run `38071197325` (2/3 pass). The local full gate passed on the published
source with identical pre/post hashes. Later corrections need new verification.

F2 is fixed: active tmux configuration no longer defines or invokes TPM, including
when its managed plugin attachment is absent. Resurrect/continuum options and the
verified direct loader remain. The real configuration regression seeds an old
executable TPM marker, fails on the original fallback, and passes after removal;
`tests/tmux/run_all.sh` passes. Passive `home/dot_tmux.conf` is restored byte-for-byte
to `ae9a644` as requested by review. The canonical config is `tmux/tmux.conf`.

F4 is fixed without platform-specific policy metadata: remove the incorrect static
machine/shared flags from Make and use its existing provider observation. The
actual default-catalog planner regression fails for privileged private Windows
installation before the fix; after it, install/removal approval is user-scoped on
Windows and machine-scoped on Linux/macOS. Native shared removal still requires
explicit approval. `make-scope-red.log` and `make-scope-green.log` record this.

F6 removes two unused new zprofile copies and the shell test that tested only the
dead copy. The actual generated-profile behavioral test remains. F7 aligns
`make ci` with the existing twenty-minute Go race budget. F8 reconciles the status,
roadmap and plan header with completed local/native/review evidence instead of
leaving stale pending-gate claims. F1 and F5 corrections are in progress separately.

The optional suggestion to delete released configuration-target inventory is not
accepted: it is intentionally offline historical/migration evidence, not runtime
removal authority. Its release-tree reproduction and passive-link coverage remain
valuable while the product explicitly migrates those releases. Full-history fetch
also supports the retained release-range checks. This does not add a runtime
consumer or duplicate ownership ledger merely to justify retaining evidence.

The Homebrew timeout concern is an intentional verified boundary. A canceled or
deadline-interrupted inspection cannot authorize repair or certify preservation;
`homebrew_timeout_test.go` covers typed errors and no reinstall. The documented
resource-local rule applies to discovery/unavailable providers, not permission to
complete a transaction against interrupted native inventory. No error is turned
into healthy/broken state to keep a preview progressing. Fresh `dpkg --verify`
concerns remain subject to the expanded real fresh-container lifecycle, rather
than weakening package-file verification speculatively.

### PortableGit long extraction and upstream post-install (2026-10-10)

Core Windows job `114268788046` reports the extractor's own `Cannot open output
file` dialog. A separately checksum-verified same-length extended output path
extracts all required files; its exit is zero but both post-install artifacts
remain and the runtime probe fails. The matched Git-for-Windows SFX source waits
for its child without propagating the child's exit status. These are diagnosis,
not successful installation evidence.

Production now supplies the extended output path, then invokes Git's documented
manual `git-bash.exe` post-install entrypoint when its regular upstream script
remains, with `--cd` selecting the ordinary payload path. It keeps exact absence
and complete runtime probes before publication. It neither implements a substitute
post-install recipe nor removes a failed script to fake completion. Existing
upstream-auto-completed extraction remains a two-command path; manual preparation
has its own attempt-bound worker phase. A process-boundary regression fails before
and passes after for completion, failed execution and unfinished script cleanup;
the focused PortableGit race suite passes. Current native Git/Make lifecycle proof
is still mandatory. The reused pure extended-path helper retains drive/UNC rules.
Sources: [upstream post-install](https://github.com/git-for-windows/build-extra/blob/main/post-install.bat),
[wrapper options](https://github.com/git-for-windows/MINGW-packages/blob/main/mingw-w64-git/git-wrapper.c).


## 2026-10-10 — F5: retain owned Bash login target from the existing baseline

Accepted Opus F5: `configureShellProfiles` selected the first existing Bash login
file every request. Creating `.bash_profile` after installation into `.profile`
changed the generated target list, invalidated the saved journal/baseline and
stranded updates/removal. The receipt already bound the correct path; no new
persisted shape is needed. Pass the engine's already-loaded state through the
selection adapter. A fixed Bash helper selects only the saved login slot from
`.bash_profile`, `.bash_login` or `.profile` under the canonical home, then invokes
the existing full baseline validation before the adapter is usable. All other
targets, Before identity/fingerprint, blocks and redirect checks remain enforced.

A crash before the first baseline write still uses the existing recovery path
only if current targets/blocks match the uncertain receipt's original observation.
An available interrupted baseline retains its saved login path. A new personal
profile is neither adopted nor changed. Bash may naturally prefer that new file;
ownership of the old block is not proof that every future Bash invocation loads it.
No general path migration, state reread, new schema or profile rewrite is added.

Validation: the fresh-controller lifecycle fails against exact `fbca6c2` at
update with “profile publication has invalid paths or size”; all three precedence
transitions pass with the correction. New Bash regressions pass under race
(7.141s), and the broader shell/profile/controller recovery suite passes under
race (70.799s), including real publication process-death tests. Vet, Linux
amd64/arm64 and Windows amd64 test compilation and `go build ./...` pass. The
redirect regression accepts the established non-mutating `retain` removal plan
with its explicit pending reason; it rejects any mutation authority and verifies
all saved/personal bytes remain unchanged. Full integration gate belongs to the
integrating parent. No native package or current-user profile was changed.


### 2026-10-10 — F1: recoverable Windows shortcut preparation

The full-delta review's F1 is accepted. The recipe-only worker key permanently
replayed failed replies and refused unfinished intent, even for a later approved
operation. The direct COM destination could also retain an unverified partial.
Real Controller retries through persisted `executeNativeCommand` records reproduce
both failures on `fbca6c2` before this correction.

Preparation now binds recipe, saved operation and a fresh attempt to an exclusive
private directory. COM writes its staging file there; bounded regular-file readback
must match its reported SHA-256 before a flushed, exclusive copy is atomically
published. Failed/unfinished attempts remain untouched and cannot affect another
attempt. The ordinary ConfigDriver journal records the exact chosen source and
continues to govern publication, resume, restoration and removal. Completed and
interrupted legacy recipe-only `.lnk` sources remain readable; no journal schema,
new receipt or assumption of deterministic COM bytes is introduced. A saved
configuration journal prevents re-preparation and requires its existing resume.

Focused controller tests cover failed and unfinished native records, successful
retry and a later install with different bytes, hash mismatch before publication,
interruption after publication, legacy input recovery and healthy checks. The
process fixture replaces only Windows COM; it uses the real native command store
and configuration filesystem lifecycle. Native COM execution remains a required
Windows rerun, not a claim from these private macOS fixtures or cross-compilation.

### 2026-10-10 — Windows Terminal removal and native serializer proof

The lower-confidence default-profile concern is accepted after tracing the actual
restoration helper and its existing test. With an original personal default, a
later choice of the newly managed profile survived removal even though its profile
entry was deleted. The prior test explicitly expected that dangling GUID. The
corrected regression fails on `fbca6c2`; removal now restores the recorded baseline
default only when the resulting collection no longer has the currently selected
managed GUID. Updating, an original retained profile, later personal fields that
retain the profile, and unrelated nonempty defaults keep their existing semantics.
No new default is invented and no unrelated startup preference is overwritten.

The native fixture now exercises the application's Settings Save through UI
Automation limited to its exact process/window. In the pinned upstream
[MainPage handler](https://github.com/microsoft/terminal/blob/v1.25.2733.0/src/cascadia/TerminalSettingsEditor/MainPage.cpp#L861),
Save calls `WriteSettingsToDisk`, whose
[serializer implementation](https://github.com/microsoft/terminal/blob/v1.25.2733.0/src/cascadia/TerminalSettingsModel/CascadiaSettingsSerialization.cpp#L1615)
calls the native atomic UTF-8 file writer even when serialized bytes are unchanged.
The fixture records hashes and precise UTC write ticks immediately around Save,
requires an observed write, closes only the Settings tab and retains the existing
configured-shell/window/clean-exit and provider check/update/remove assertions.
Pinned `NewTabButton` and `SaveButton` controls plus the English hosted Settings
menu identify the route. Missing controls/patterns or an absent write fail; no
foreground switch, global keystrokes, synthetic serializer or skip-success path
is used. UI selectors and the real write remain pending the disposable Windows
run. Local script parsing and cross-compilation do not certify that native proof.

## 2026-10-10 — Windows Rust compiler library paths

Accepted native failure: exact candidate `fbca6c224d03ac57597628464ba0bf92e94f2df7`,
Windows core job `114268788046`, reached the selected MSVC linker but failed
LNK1104 opening a 273-character `libstd` path below the owned Rust generation.
The same fixture had just verified the target `libstd` and `libcore` files.
This is distinct from the fixture-only PowerShell stdout/CLIXML issue: VC++
registration/signature, LaTeX UTF-8 conversion and Yamllint passed that run.

Pinned Rust 1.99.0 derives its default sysroot from the loaded compiler DLL and
canonicalizes it, so the managed `current` directory junction alone does not
retain a shorter path. Its Windows GCC path helper removes the extended prefix;
MSVC then receives the physical library pathname. An explicitly supplied sysroot
is retained in `Sysroot`, and the MSVC linker path builder passes that spelling.
See the pinned [filesearch source](https://github.com/rust-lang/rust/blob/1.99.0/compiler/rustc_session/src/filesearch.rs),
[session configuration](https://github.com/rust-lang/rust/blob/1.99.0/compiler/rustc_session/src/config.rs),
[filesystem helper](https://github.com/rust-lang/rust/blob/1.99.0/compiler/rustc_fs_util/src/lib.rs),
[linker implementation](https://github.com/rust-lang/rust/blob/1.99.0/compiler/rustc_codegen_ssa/src/back/linker.rs)
and [Microsoft's linker path limit](https://learn.microsoft.com/en-us/cpp/build/reference/linking?view=msvc-170).
Missing libraries or an undiscovered compiler do not fit the captured evidence.
A Windows native run must still establish the fix against the actual linker.

The fixed Windows preparation now preserves upstream compiler executables beside
their DLLs and copies the already observed installer executable as `rustc.exe`,
`rustdoc.exe` and `clippy-driver.exe`. Only those exact names enter the fixed
launcher branch, before normal installer signal handling or platform discovery.
The three launchers derive an absolute extended-length sysroot from their own
payload and launch their exact sibling original without a shell. Rustc/rustdoc
receive the default explicit root only when the caller did not supply one.
Stable UTF-8 `@argfile` inspection is bounded at 16 MiB and follows the pinned
[nonrecursive, one-line-per-argument contract](https://github.com/rust-lang/rust/blob/1.99.0/compiler/rustc_driver_impl/src/args.rs);
original arguments remain intact. There is no unstable shell-argfile parser.

Clippy has a separate existing upstream contract: Cargo passes the compiler as
its first positional argument, and Clippy removes that argument before compiling.
Prepending a root option would break wrapper detection. The fixed launcher
therefore preserves Clippy argv exactly and supplies its existing `SYSROOT`
environment input only when the caller has not supplied that variable. Upstream
Clippy already gives direct/argfile root options precedence; see the pinned
[driver](https://github.com/rust-lang/rust/blob/1.99.0/src/tools/clippy/src/driver.rs)
and [Cargo subcommand](https://github.com/rust-lang/rust/blob/1.99.0/src/tools/clippy/src/main.rs).
Stdin/stdout/stderr and exit status pass through. The forwarding parent subscribes
to console interrupts and waits for the original child, which receives the same
console event; the launcher does not kill it early through context cancellation.

The pin contains recipe revision 1 and the observed installer's SHA-256, bound at
the existing process boundary. Preparation reads and verifies those exact bytes
before downloads or payload mutation. Changing the installer changes Rust's
ordinary desired identity. The archive lifecycle still owns publication,
manifest checks, preservation and cleanup; no launcher registry or second
lifecycle was introduced. Published pins without the optional field retain their
serialized shape, remain readable for update/removal, and are not rewritten.
An unfinished old Windows Rust preparation is refused with explicit recovery
guidance instead of silently acquiring the new recipe.

Rejected alternatives: shortening the fixture, home or 256-bit generation
identity would hide the supported path boundary. A `RUSTFLAGS`-only change would
miss direct compiler invocations. Adding Python/distlib to Rust would introduce
an unrelated runtime dependency and desired-identity coupling. The existing
trusted installer binary supplies the bounded native entrypoints directly.

Focused race tests cover exact launcher bytes/original preservation, changed or
unbound source refusal before download, nested/unknown recipes, legacy shape and
ordinary updates without old-provenance mutation, direct/stable-argfile overrides,
argument boundaries, Clippy's Cargo protocol, Unicode stdin/argv, stdout/stderr,
startup failure and child exit status. `go vet ./...` and Windows amd64 test
cross-compilation pass. Native Windows console-interrupt proof is included using
the existing private-console sender, without signalling the runner console.
The existing real Rust lifecycle now retains its long physical library assertion,
checks managed/default/caller/argfile sysroots, compiles and runs directly, builds
through offline Cargo, runs Clippy, checks immutable ownership and removes.
Those Windows execution checks remain pending the disposable hosted run; local
cross-compilation is not evidence that MSVC linked successfully.

### 2026-10-10 F3 disposable Linux partial runtime evidence

Fresh arm64 Debian 13 and Ubuntu 26.04 containers proved that curl and unzip
were absent before the selected Neovim graph, then installed both with created,
healthy APT receipts through the public command. Both reached strict installed
Neovim LSP/parser/formatter smoke (257 checks), ordinary update, and the combined
Neovim/Pi selection. This is runtime proof of the missing prerequisite correction,
not a full removal-lifecycle pass. The local harness overrode root PATH without
`/usr/sbin` and `/sbin`; selective removal then failed because dpkg could not find
`ldconfig` and `start-stop-daemon`. Earlier read-only source mounts had also
prevented the smoke fixture from creating its own `tests/.cache`; moving source
into the private writable container corrected that separate harness defect.
No production assertion or owned-package guard was weakened. The existing hosted
fresh APT lanes preserve normal root PATH and remain the required full lifecycle
proof. Do not label these local runs green.

Raw logs and Docker inspection records were retained as
`mason-apt-{debian,ubuntu}-v2.{log,json}` before deleting those completed task-owned
container filesystems. Debian's final log SHA-256 is
`147c7ad3a17233efc396a42e225ea3ed8e6281f2565358c7affd3e9f0e5437e2`;
Ubuntu's is
`86e013c0d7ea79aefb8aa959cfddf983eabb151ceeee8e97d0e98be8cdb1a792`.
No further local harness run was performed after this result.

### 2026-10-10 — Private Node/Mason package probe

The private macOS arm64 probe installed all five current selected npm tools
using pinned Mason's command sequence: pyright 1.1.414,
bash-language-server 5.8.1, yaml-language-server 1.24.0,
vscode-langservers-extracted 4.10.0 and prettier 3.9.9. All 5,888 Node payload
entries matched after each of the 15 commands (content, membership, modes and
link targets); the actual test passed in 45.70 seconds. The Node archive,
HOME, npm configuration/cache and package directories were private and removed.
Raw output, temporary test source and actual dependency locks are retained in
`node-mason-packages-evidence-sha256.json` (SHA-256
`c3785ccd105a9ff9e795bc0e3ce3f152dafa96bd2169fce4c05821443e4465f7`).
This sequential package probe does not establish historical registry/indirect
resolution, full Neovim environment, concurrency or installed-command lifecycle.
The hosted Node integrity failure remains unresolved; the failure-only native
pin/link/payload diagnostic is still required. No integrity assertion changed.

PortableGit follow-up source inspection found no defect in invoking the upstream
batch through its supported git-bash wrapper. The Microsoft MSRC batch-process
analysis confirms CreateProcess can invoke a batch through CMD, resolving the
apparent conflict with generic API wording. The pinned SFX inherits its extraction
working directory and waits without checking the child exit status. Native logs
prove extended-path extraction left post-install artifacts, but do not contain
an explicit working-directory error. That causal explanation remains an inference;
only the next native preparation and real runtime checks can establish the fix.

### 2026-10-10 — Integrated correction gate and passive tmux assertion

The first integrated v3 `make -k ci` run preserved source hashes and passed the
complete Go race suites (installer 460.860 seconds, terminal 14.134 seconds),
all 39 bootstrap tests and the official 63-record Renovate extraction. It failed
one static assertion that required both active and released passive tmux files
to have the new extended-key setting. That contradicted F2's exact released-byte
preservation and was not a product failure. The initial raw gate and source hashes
are retained in `integrated-corrections-v3-initial-full-ci.{log,json}`.
The corrected assertion still requires the active tmux setting; separate inventory
tests now require exact released tmux and WezTerm hashes. The complete gate is
rerun after this test/documentation correction. No production code changed after
the passing full race suite, and no native acceptance is inferred from it.

The complete raw-log follow-up also identified an EditorConfig error in the
Rust launcher's embedded Go test program (spaces instead of tabs). The preceding
summary incorrectly described the tmux assertion as the only gate failure.
The next run passed the corrected tmux/inventory checks and all 40 bootstrap
tests, but still failed that formatting check; its output is preserved as
`integrated-corrections-v3-formatting-full-ci.{log,json}`. Fixture indentation is
corrected without changing its child-process behavior, and the whole local gate
is repeated. Both failed gate records remain failures, despite passing Go results.
The checker stops at the first offending file. A direct recheck exposed the
same fixture-only indentation in `nvim_native_diagnostic_test.go`; the just-started
full rerun was stopped before completing and is explicitly recorded as terminated.
Both embedded Go programs now use tabs. A complete pruned-file checker invocation
is required before starting the next full gate, avoiding another first-error cycle.

### 2026-10-10 — APT retained-conffile installation evidence

Accepted defect: Debian 13 arm64 job `114285247930` passed its APT lifecycles,
then fresh Neovim failed Git with `APT recovery installation did not start from
absence`. The inventory already excludes dpkg `config-files` from installed
payload, but the recovery parser incorrectly required every native `install`
record's old version to be `<none>`. [dpkg's action selection](https://github.com/guillemj/dpkg/blob/1.22.21/src/main/unpack.c#L1335)
uses `install` for both `not-installed` and `config-files`; the latter retains
its old version. Other prior states use `upgrade`. The hosted log did not retain
the offending package's dpkg record, so it does not identify whether Git itself
or an incidental dependency hit this guard.

The fix removes only that old-version restriction. Duplicate native installs,
foreign operation records, missing configuration, manual/source drift and native
failure still cannot grant completed ownership. Schema-1 saved intents need no
migration. Purging retained configuration or resetting the fixture was rejected:
ordinary package removal followed by installation is supported production use.

New regressions cover never-installed, same-version and changed-version residual
states, successful installation, interrupted configuration, saved successful
command recovery without repeat execution, exact root/pool ownership and removal.
The original parser fails the new focused tests. A disposable Debian 13 arm64
container also reproduces the exact error after proving normal removal retained
`config-files` state and personal conffile bytes (`apt-residual-debian-before.log`,
36.48 seconds). The fixture now reinstalls and removes that package without purge,
verifies operation attribution and outside-consumer protection, and compares all
unrelated native package state. This local container proof explicitly sets the
existing disposable-runner opt-in flags; it is not a GitHub-hosted result.
Focused APT tests pass after the correction (7.411 seconds), including the race
run (14.812 seconds). The corrected real provider/worker lifecycle passes on
disposable Debian 13 arm64 (44.77 seconds) and Ubuntu 26.04 arm64 (84.14 seconds),
with the full `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin` PATH.
Both logs show native `install ... 2.0 2.0` and preserve the personal conffile
through the second removal. Linux arm64 and Windows amd64 test compilation also
pass. Raw red/green output and source hashes accompany the frozen patch;
the full hosted fresh Neovim acceptance is still required.


### 2026-10-10 — Review of 077683f and the next native correction batch

Actual read-only Claude Opus 5.5 inspected the full task delta at `077683f`,
used 162 Read/Glob/Grep calls, and exited successfully without API, permission
or guard errors. All 3,315 supplied source/evidence files retain their frozen
hashes. Review SHA-256: `7be75b24257e91a61ac137c8acc6f2145871c0a6c8897219cc268951e9f6d5a4`. Requested effort was xhigh;
the CLI record does not expose the applied effort. F1–F8 and the Terminal
removal-default concern are resolved. Accepted new findings are D1 (Rust desired
identity coupled to every installer rebuild), D2 (modified passive released
link sources), and D3 (incorrect language-matrix documentation path).
The review snapshot predates completed native results and cannot approve them.
The owner explicitly superseded the Helix source-freeze instruction: immutable
review snapshots permit independent implementation to continue in parallel.
Each review still applies only to the exact supplied snapshot.

D1 now separates desired recipe identity from exact operation provenance.
The desired Rust identity includes the launcher revision, while the saved
operation/version pins retain the exact executable SHA-256 and preparation
checks those bytes before mutation. Payload fingerprints still cover every
installed launcher. Unrelated binary rebuilds no longer trigger a Rust repair
or update; deliberate recipe changes still do. Exact pin comparison remains in
version and cleanup provenance checks. This avoids a dedicated launcher build
pipeline and keeps the existing archive lifecycle. The focused controller test
fails before the correction (`v4-rust-identity-before.log`) and passes afterward:
check and update preserve the generation/provenance, and installed launcher
edits still produce needs-action. The native Windows Neovim fixture now rebuilds
only the installer build ID and requires the actual public check to stay ready;
that native proof remains pending. D3 points back to `tests/nvim/language_matrix.lua`.

The macOS Node diagnostic itself failed at archive-link validation because its
new temporary reference used a logical parent alias (`/var`), while link
resolution returned the physical parent (`/private/var`). A real private-symlink
regression reproduces the failure, then passes after resolving the existing
parent before extraction. Installed evidence and the alias remain unchanged,
and diagnostic output contains only metadata/hashes. The first correction
incorrectly tried to resolve the not-yet-created reference itself and failed
existing tests; `v4-node-alias-after.log` remains a failed result. The corrected
parent resolution passes all diagnostic cases in
`v4-node-alias-corrected-after.log`. This fixes observability, not the unresolved
native Node payload drift.

Native Code startup currently fails before any mapped window on macOS and both
Linux architectures. No runtime cause is established. The macOS fixture now
performs one public NSWorkspace launch through the same published application
URL and explicit fixture environment, preserving bounded underlying NSError
chains. It does not bypass LaunchServices. The helper compiles and rejects
missing input without launching an application locally. Linux retains bounded
gio output even on successful launcher exit; failed fixtures record the private
Code main.log and actual runtime-directory/user-namespace observations. Existing
PID, mapped-window, settings-witness and ownership assertions remain required.
No sandbox switch, security-policy change, shorter path or longer timeout was
introduced. The new diagnostics require the next disposable native run.

The focused parent suite passes (`v4-parent-focused-after.log`). The previously
cached terminal result was re-run uncached with `go test -race -count=1 ./terminal`
and passes in 14.029 seconds (`v4-terminal-uncached-race.log`). Full integration
and final all-platform native proof remain required after parallel patches land.


### 2026-10-10 — Build Tools inspection owns its compiler child processes

At `077683f8bc45e1feb4c65824fe734876aec839ee`, Windows Neovim job
`114285248218` rejected the initial apply. Its complete reviewed/fresh JSON differs
only in the plan ID and `tool.compiler` consumers: absent became
`Running vctip.exe (PID 2460)`. Job cleanup also reported that orphan. The compiler
observer collects consumers before compiling and running its SDK probe; that
probe can therefore create the consumer seen by the next observation. The raw
`v3-job-114285248218.log` and `v3-windows-neovim-plan-diff.json` preserve the actual
failed-before evidence. Fresh Build Tools job `114285247996` passed install,
environment acquisition, update and repair, then refused removal because consumers
remained. Its log does not name those consumers: do not report that separate
failure as proved to be `vctip.exe`.

The dedicated read-only Build Tools query now creates a suspended PowerShell
process with atomic assignment to an unnamed, non-inheritable Windows Job Object.
Only its three standard-stream handles are inherited. The job disallows breakaway
and uses kill-on-close; normal exit, failed exit, output overflow and cancellation
terminate and verify quiescence of its descendants before returning. Inspection
retains its 30-second bound and separate 1 MiB stdout/stderr buffers; termination
has a five-second bound and failure remains fatal. Atomic job-list assignment
avoids a supervisor-death gap before assignment. Real compiler/SDK compile-and-run
health, pre-existing consumer inventory, approval checks and the durable mutation
worker are unchanged. No machine telemetry setting or vendor payload is modified.

Microsoft documents [job-list assignment on Windows 10 / Server 2016 and later](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute),
which matches the [Go Windows baseline since 1.21](https://go.dev/doc/go1.21#windows).
The pinned [Build Tools 2022 requirements](https://learn.microsoft.com/en-us/visualstudio/releases/2022/system-requirements#microsoft-visual-studio-build-tools-2022-system-requirements)
also cover the Server Core 2022 acceptance container. The SDK's `WinBase.h`
defines job-list attribute 13 with the input flag; the existing x/sys version
does not export that constant. Job termination is asynchronous, so merely waiting
for PowerShell or closing a job handle is not treated as quiescence.

Rejected alternatives: process-name filtering would hide uncertain external
consumers; retrying the changed plan would conceal an observation side effect.
`VSCMD_SKIP_SENDTELEMETRY` gates the developer-shell telemetry path, without a
verified contract covering compiler/linker `vctip.exe` spawning. Microsoft's
[documented Build Tools telemetry opt-out](https://learn.microsoft.com/en-us/visualstudio/ide/visual-studio-experience-improvement-program#registry-settings)
uses machine registry settings, inappropriate for read-only inspection. A
PowerShell self-assignment prefix would add a handle protocol and a pre-assignment
`Add-Type` compiler gap; no generic process framework was added.

Focused existing Build Tools/connection race checks pass on macOS (5.505 seconds),
including PowerShell parsing and consumer-removal refusal regressions. macOS and
Windows-target `go vet ./...` and Windows amd64 test compilation pass. New ordinary
Windows process regressions cover surviving children with inherited output pipes,
success, exit-code/stderr failure, cancellation, both output bounds, unrelated
process preservation and Unicode/space/percent paths. Native fixtures now repeat
real observations and require no probe-created consumer during the existing full
owned lifecycle. Those Windows tests were compiled but not executed here; the
next ordinary Windows CI and native Neovim/Build Tools jobs remain required.
No local Windows green or full native lifecycle pass is claimed. Check logs,
source hashes and red-evidence hashes accompany `buildtools-query-handoff.json`.


### 2026-10-10 — Native Terminal serialization, Apple receipt files and passive source drift

At `077683f8bc45e1feb4c65824fe734876aec839ee`, native Desktop Windows job
`114285248086` proved the owned Terminal Settings UI Save wrote the real file
(hash and precise write time changed), then ownership failed. Pinned Terminal
1.25.2733.0 registers `CloseTabArgs` even for `"closeTab"` and serializes it as
`{"action":"closeTab"}`: see upstream
[argument registration](https://github.com/microsoft/terminal/blob/v1.25.2733.0/src/cascadia/TerminalSettingsModel/AllShortcutActions.h#L122),
[argument construction](https://github.com/microsoft/terminal/blob/v1.25.2733.0/src/cascadia/TerminalSettingsModel/ActionArgsMagic.h#L189)
and [serializer](https://github.com/microsoft/terminal/blob/v1.25.2733.0/src/cascadia/TerminalSettingsModel/ActionAndArgs.cpp#L287).
Only those exact argument-free forms now share the preexisting string fingerprint;
other actions and added arguments remain edits. Original raw values and full
restoration inventory are unchanged. Regression coverage exercises check, update
and removal with an exact escaped-string baseline. Actual native UIA Save remains
required; failure-only diagnostics expose at most 16 fixed owned-field changes,
with each value capped at 256 bytes and an omitted count. The historical log has
no field values, so the next run must establish whether other differences exist.

Apple job `114285248057` passed preinstalled preservation/reuse, then rejected the
directory entry `private` in `com.apple.pkg.CLTools_SDK_macOS13` before any move,
selection reset or install. Apple's `pkgutil(1)` documents `--only-files` to exclude
directory records. The fixture now uses that option, retains strict receipt root
checks and canonical CLT file boundaries, and reports all invalid files with a
20-path/256-character display cap plus an omitted count. Query failures remain
fatal. Process-boundary regressions reproduce the directory false positive,
directory-only receipts, outside files, malformed paths and bounded diagnostics.
The historical log does not show the remaining SDK files; native success remains
unproved and no `private/**` exception was added.

Review D2 is confirmed against recorded release evidence: the passive PowerShell
profile, POSIX LazyGit template, three tmux palette files and Windows LazyGit link
template had changed. They are restored directly from v0.4.4, including historical
comments and the old profile's multiplexer hook. Those are passive source bytes,
not a reintroduced installer runtime. Inventory tests bind every present recorded
passive target/profile and referenced template to its latest release hash. The
PowerShell migration regression fails before restoration, recognizes the restored
profile, and still classifies later personal edits as edited while preserving their
exact bytes. Active shell and tmux assertions now test only their active recipes;
the active LazyGit target still uses upstream defaults. README identifies the two
source roles. No everyday-machine native mutation or full-gate/native pass is
claimed by this isolated correction batch.

The same exact-byte invariant exposed a Windows checkout boundary: extensionless
passive profiles, YAML and template sources were still subject to `core.autocrlf`.
The existing physical Git checkout test now includes present inventory/profile
sources and verifies the recorded release hashes without newline normalization.
It fails before narrow LF attributes for `home/**` and `windows/chezmoi-*/**`;
`text=auto` retains binary detection. Active recipes and unrelated paths are not
part of this attribute change.

Focused validation: six selected Terminal tests pass with the race detector
(29.413 seconds), three migration tests pass with race (1.967 seconds), and the
physical checkout test passes with race (1.392 seconds). All 22 selected bootstrap/
release-evidence tests and 47 focused Pester tests pass. Windows amd64 root-package
test compilation, changed-file EditorConfig, workflow YAML lint, LazyGit active
defaults and `git diff --check` pass. Focused PowerShell analysis reports only its
existing reviewed test-fixture Write-Host warning. Failure logs retain the original
Terminal ownership failure, Apple directory rejection, six passive-source hash
mismatches, migration recognition failure and physical checkout byte drift.
Full integrated and native reruns remain the parent delivery lane's responsibility.


The integrated focused race run (`v4-integrated-focused.log`, 17.260 seconds)
passed available cases but skipped four PowerShell-backed groups because that
one command omitted the private pwsh directory from PATH. It is narrower proof,
not execution of those groups. The complete gate uses both the pinned Go and
private PowerShell directories, and must execute their actual parser/protocol
fixtures. Docker's non-root user namespace preflight returned EPERM with seccomp
mode 2 and no Ubuntu AppArmor sysctl files. That cannot faithfully distinguish
the hosted Code startup cause; no sandbox change or GUI installation followed.
The raw bounded preflight evidence is retained separately and is not a GUI pass.

### 2026-10-10 — Windows native archive aggregate deadline

Job `114285248186` on `077683f` ran the native archive command without an explicit
timeout and hit Go's 600-second package alarm. The pinned sweep had run for nine
minutes; Python, the 14th of 18 sequential pins, had run for 98 seconds. Its stack
was `GetFileAttributesEx → inspectTree → ArchiveDriver.Observe → Execute`, from
`archive_native_test.go:200`: the pre-removal observation after successful install,
version/import/venv execution, update, health check, deliberate bytecode-edit
detection and byte restoration. No Python child, native worker or download was
awaited in the panic stack. A single stack cannot exclude a slow filesystem call,
but it does not support treating this as a blocked Python installation.

The identical sweep source on `fbca6c2` passed in 356.79 seconds (Python 83.02),
and that complete installer archive package passed in 485.296 seconds. In this
run the preceding pins consumed about 442 seconds before Python began, versus
263.41 seconds previously. Per-pin current timings were not emitted before the
parent test's panic, so no individual slow predecessor is identified. The narrow
correction sets the sweep's aggregate limit to 20 minutes, preserving every test,
per-command deadline, path and native assertion. The core job took approximately
35 minutes 32 seconds including later failed steps; adding the full extra ten
minutes still fits its existing 60-minute limit. No overall job-budget change is
justified by this observation. This correction requires a fresh hosted pass.

Independent actual passes in the same raw log remain valid scoped evidence:
private Rust compiled and ran a program, formatted input, built through offline
Cargo, ran offline Clippy, checked unchanged and removed (121.59 seconds); Pi's
ConPTY theme consumer passed (13.41 seconds). The Windows Terminal GUI consumer
was skipped in this core job and has separate desktop acceptance. The failed
archive, PortableGit and public-bootstrap steps keep the whole job failed.


### 2026-10-10 — PortableGit interpreter input and Git null configuration

The complete Windows core log for job `114285248186` at `077683f` shows successful
SFX extraction followed by two separate post-install errors: Git rejects uppercase
`NUL` as a configuration path, and CMD reports its batch input missing. Both
`post-install.bat` and `etc/post-install` are absent afterward, while the runtime
executables remain. This is still a failed lifecycle, not a completed install.
The checksum-verified baseline 2.56.0.windows.1 archive has the same post-install
bytes as the current 2.56.0.windows.2 archive. Its final deletion names
`post-install.bat` literally. Inspected SFX and wrapper code both wait for their
children; an asynchronous child race is rejected as the explanation.

The fixed preparation executes a byte-identical, exclusively created sibling
through the existing upstream Git Bash wrapper, preserving `%~dp0` and the full
owned payload path. CMD retains its input after upstream deletes the original.
No upstream script is rewritten and no nonzero exit is accepted. Failed or
unfinished preparation retains its input as evidence; successful cleanup verifies
file identity and SHA-256 before removing the copy. Original post-install artifact
absence and the full Bash/Git/LFS/credential/SSH runtime probe remain mandatory.
The pinned [Git Windows path implementation](https://github.com/git-for-windows/git/blob/v2.56.0.windows.1/compat/mingw.c)
recognizes `/dev/null` explicitly, while uppercase `NUL` reaches ordinary path
handling. Preparation, native Git use and the physical-checkout fixture now use
the supported spelling. The [upstream batch script](https://github.com/git-for-windows/build-extra/blob/main/post-install.bat)
is preserved exactly in the staged copy; accepting its old failure status or
shortening the generation path would not satisfy the contract.

The process-boundary regression fails against the prior preparation and passes
with this correction. Focused race tests include failed-after-cleanup, unfinished,
changed-copy and occupied-copy refusal, plus actual local Git configuration
isolation and the physical checkout check. Windows amd64 compilation passes.
The ordinary Windows-only private CMD regression and full pinned native archive
lifecycle are prepared for the next runner; neither was executed on the Mac.
The first local test invocation used the repository root without a Go module and
is recorded as a harness invocation error; the actual red/green runs use
`installer/`. No full gate, native rerun or publication is claimed by this slice.

### 2026-10-10 — Remove script-module overhead from Go ZIP extraction

Native Windows core job `114285248186` at `077683f` records a 1,003 ms download,
112 ms archive verification, then `toolchain extraction: started` without a
completion before the public fixture's 300-second deadline. The original raw
log is `v3-job-114285248186.log`. Download and checksum are passing stages; neither
was repeated as a purported failure or replaced. The log does not identify the
runner's archive-module version or isolate its per-entry timing.

The official [Archive 1.0.1.0 source](https://www.powershellgallery.com/packages/Microsoft.PowerShell.Archive/1.0.1.0/Content/Microsoft.PowerShell.Archive.psm1)
wraps .NET extraction in a PowerShell loop with provider calls, growing-array
appends and progress for each entry. Microsoft's [Archive 2.0 explanation](https://devblogs.microsoft.com/powershell/archive-module-2-0-preview-1/)
identifies script-module overhead as a performance problem. A controlled private
comparison using that exact published module and the checksum-verified Go ZIP
took 115,149 ms through the module versus 3,136 ms through .NET, in one PowerShell
7.6.3 / .NET 10.0.9 process on macOS arm64. The module's progress preference was
confirmed `SilentlyContinue`. This demonstrates removable script overhead,
not the exact Windows 5.1 cause or its eventual runtime.

The bootstrap now uses [the built-in two-argument ZIP extraction API](https://learn.microsoft.com/en-us/dotnet/api/system.io.compression.zipfile.extracttodirectory?view=netframework-4.8.1),
also used by the [official PowerShell installer](https://github.com/PowerShell/PowerShell/blob/dc4e8fead44072a7ed7d8dd5d9961aec0c8661d1/tools/install-powershell.ps1#L116).
It adds no package or executable dependency. Exact archive size/SHA verification
still precedes extraction into the exclusive private generation, and compiler
reuse still requires extraction from those verified bytes. Malformed ZIPs,
escaping entries and existing-file collisions propagate errors. Existing final
cleanup, machine stdout, physical paths and the public fixture deadline remain
unchanged. No global AppContext switch, registry setting, cached-compiler trust,
alternative download, path shortening or PowerShell 7 requirement was introduced.

The pinned archive has 15,639 entries and 246,912,295 expanded bytes; its longest
entry is 108 characters and largest file is 28,696,064 bytes. A separate private
.NET extraction verified every entry's bytes against the ZIP, including hidden
files omitted by the comparison's default directory listing. A 2 GiB entry-size
explanation is therefore rejected. No source evidence establishes a Windows
long-path failure here; native path behavior must be observed rather than worked
around with global switches.

The actual extraction-action regression fails before the change on a valid ZIP
because the archive module invokes per-entry progress with `ProgressPreference`
set to `Stop`. After the change it passes with exact file bytes, Unicode/spaces/
apostrophes/brackets, the pinned archive's longest entry, no stdout pollution,
and preservation of existing bytes on invalid/escaping/colliding input. Windows
executes this test through discovered system PowerShell 5.1. Local results use
PowerShell 7.6.3 on macOS; native Windows execution and the full public lifecycle
are still pending. Source, benchmark, red/green and archive-evidence hashes are
recorded in `bootstrap-extraction-handoff.json`.
The focused bootstrap race checks pass in 3.166 seconds. Windows amd64 test
compilation, Windows-target `go vet ./...`, the changed-file EditorConfig check
and `git diff --check` also pass; the parent owns the integrated full gate.


### 2026-10-10 — Integrated initial batch and final Windows corrections

The initial v4 batch passed all seven gates with unchanged source hashes:
`make -k ci` (Go race 484.714 seconds; terminal 13.923 seconds; 45 bootstrap
tests) and Windows amd64/Linux amd64/arm64 all-package build/test compilation.
The original receipts/logs are retained under `integrated-corrections-v4-initial-*`.
The completed `077683f` matrix has 10 of 23 passing jobs: all three quality jobs
and seven native installer jobs. Windows core's exact raw log supplied the
archive sweep, PortableGit and extraction corrections appended above. Those
three newly integrated changes require a fresh complete local gate and native
runs; initial v4 gate evidence does not cover them. All six passive files were
restored from released evidence; the active no-psmux/default-keybinding behavior
passed the initial gate. The forthcoming source review remains an immutable
snapshot while independent implementation proceeds under the owner's override.

### 2026-10-10 — Final bootstrap comment compatibility check

The first complete gate after integrating the final three Windows corrections
failed the existing PowerShell comment invariant: two new bootstrap comments
contained apostrophes. Their wording is corrected without changing extraction
behavior or the checker. The failed gate is retained as failed evidence; its
source hashes also change because this correction was made while its remaining
checks completed. A fresh complete gate and foreign compilation must cover the
corrected source before publication. The all-file EditorConfig preflight passed
all 684 eligible files before this wording correction.


### 2026-10-10 — Windows shortcut PowerShell progress boundary

At candidate `28c92f5`, completed Windows Desktop job `114302868951` passed
native font install/render/check/removal (22.91 seconds) and the Terminal fixture's
real Settings UI Save/check/removal (26.66 seconds). The Save changed both the
settings hash and write time. Public VS Code singleton application then failed
before launch with `shortcut bytes differ from native preparation result`.
The complete failed job remains failed; the narrower passes are separate proof.

Shortcut preparation compared its staged file digest with the complete native
worker output. Its encoded PowerShell script called `Get-FileHash` without the
progress preference already used by vendor scripts. A real PowerShell regression
substitutes only COM file creation, keeps the production script's remaining work,
and runs the actual durable native worker. It records a correct SHA-256 mixed
with `#< CLIXML` progress and reproduces the same rejection. The hosted error did
not record the mismatched output, so attributing that particular instance to
CLIXML remains an inference supported by source and this process reproduction;
the alternate actual-byte-change explanation is not silently accepted.

The correction sets the script-local progress preference to `SilentlyContinue`,
consistent with [Microsoft's preference-variable contract](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_preference_variables?view=powershell-5.1#progresspreference).
It does not change the worker protocol or trim CLIXML, errors or arbitrary text.
Terminating errors, failed exits and exact file-digest verification still fail.
A future mismatch reports only bounded output length and CLIXML presence, not
personal output contents. The native Windows ordinary fixture uses real system
PowerShell 5.1/COM to create, inspect, check and remove private shortcuts; it does
not launch an app or write to the user's Start menu.

The real progress regression failed before the correction and passed afterward.
The focused shortcut race suite passed in 11.148 seconds, including failed and
unfinished attempts, exact saved-input recovery and native hash rejection.
Windows amd64 compilation includes the new actual-COM fixture. Actual Windows
execution and the full public GUI lifecycle remain pending; no native machine
mutation, publication or global PowerShell preference change was performed.


### 2026-10-10 — Windows desktop inspection progress boundary

A concrete multi-location check after the shortcut finding found the same mixed
stream assumption in the desktop fixture's two Windows read-only process/window
queries. Both now call the existing `runIntegrationQuery`, which separates success
stdout from bounded stderr diagnostics and preserves failed native exits. The
macOS/Linux queries and app launch behavior are unchanged. This is a source-found
follow-up: the hosted public desktop failure happened before app launch and did
not execute either query.

A real encoded PowerShell process regression emits first-use-style progress with
each of the PID array, single PID and visible-window token protocols. With the
former `CombinedOutput` boundary all three fail with recorded CLIXML mixed into
valid output; using the existing query helper all three pass. A valid token with
exit 23 still fails with its bounded diagnostic. The test uses Windows PowerShell
5.1 on Windows and the private PowerShell executable for the local process proof;
it does not open windows or inspect unrelated applications. Actual Windows GUI
consumption remains a required hosted rerun, not a claim from this boundary test.

Focused query/font boundary race tests passed in 7.767 seconds with the real
PowerShell probe enabled; Windows amd64 compilation and changed-file formatting
checks passed. No native Windows query execution is claimed by the local run.


### 2026-10-10 — Windows extraction fixture matches mandatory verification order

Completed Windows core `114302869285` and quality `114302869152` both reject the
ordinary extraction fixture's literal-path case: PowerShell 5.1 first-use Utility
module progress from Add-Type encounters the fixture-only Stop preference. This
is not the production extraction failure seen on the prior candidate. The same
core job passes public setup/migrate (71.76 s): cold download 1.352 s, archive
verification 0.093 s, extraction 15.858 s and source build 16.767 s; the cached
archive's second extraction also passes (15.934 s).

Production always verifies the archive with Get-FileHash before Add-Type; both
cmdlets belong to Microsoft.PowerShell.Utility. The isolated fixture now performs
that same preceding file-hash phase against a Go-computed expected digest under
ordinary progress handling, then retains Stop around the unchanged real extraction
action. This preserves detection of extraction progress machinery and exact
archive contents, including literal Unicode paths and the longest pinned entry.
Malformed ZIP, path escape and preexisting-file refusal remain required; those
negative cases must reach the extractor rather than pass on a module-load error.
The native failed logs are the red evidence. No bootstrap production algorithm,
download path, filesystem limit or PowerShell error handling was relaxed.

Focused extraction race tests pass locally (3.200 s) with private PowerShell
7.6.3: each negative case reports the actual ZipFile extraction exception and
retains the sentinel bytes. Windows amd64 compilation passes; Windows PowerShell
5.1 execution of the corrected fixture remains pending.


### 2026-10-10 — PortableGit runtime phase diagnosed separately from extraction

Windows core `114302869285` reaches `PortableGit runtime` then exits 255. The
upstream `post-install.bat` and `etc/post-install` are both absent, and the actual
ordinary CMD self-deleting-input regression passes. This is not another SFX or
batch-input failure. The unlabelled set-e probe checks Git, LFS, GCM and SSH; its
.NET-style path-length exception makes GCM the leading hypothesis, not yet a
captured command attribution. The exact pinned binary identifies GCM
`2.9.1+6760f0ef069c994aa2bb1d703fb374986ee82a3e`, targeting .NET Framework 4.7.2.

Each fixed runtime step now announces its label on stderr without changing
commands, expected results or strict failed-exit behavior. The opted-in native
fixture enables `GCM_TRACE=1` and explicitly disables secret tracing on that
same private version-check child. Upstream [Application.WriteException](https://github.com/git-ecosystem/git-credential-manager/blob/6760f0ef069c994aa2bb1d703fb374986ee82a3e/src/shared/Core/Application.cs#L148)
prints the exception stack only when tracing is enabled. There is no credential
operation, second native attempt, shorter path, ignored exit or global setting.
The private Git core.longpaths recipe remains a separate correction for Git's
own repository paths; it is not evidence that .NET path handling works. These
changes preserve the failure and supply the minimum next native evidence.


### 2026-10-10 — Build Tools query completion remains a failing native boundary

At `28c92f5`, core and quality fail all five private process-tree cases with
`WAIT_TIMEOUT` (258) on the exact held child HANDLE immediately after inspection
returns. Success bytes, explicit failure, cancellation and both output limits
reach their expected preceding assertions. Fresh vendor lifecycle (626.17 s) and
existing compiler/environment reuse (29.86 s) pass separately; neither proves
the ordinary process-tree completion contract.

The child is created directly by Go's CreateProcess path, not ShellExecute, and
the fixture holds its HANDLE before releasing the root. Broker escape and PID
recycling therefore do not explain this observation. Production currently checks
job ActiveProcesses after TerminateJobObject. [Microsoft's termination contract](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-terminatejobobject)
is per-process TerminateProcess behavior; [the process HANDLE signal](https://learn.microsoft.com/en-us/windows/win32/procthread/terminating-a-process)
is the completion event. The log does not yet establish whether job accounting
reaches zero before final object rundown or whether containment is incomplete.
A failure-only five-second wait on that same exact HANDLE now records initial
and final exit state and elapsed time. The original immediate assertion still
fails, even if the later wait succeeds; cleanup remains limited to fixture-owned
handles. No production delay, timeout increase, name filter, or fake-green
termination exception is introduced. The native diagnostic rerun is required
before claiming this boundary fixed.


### 2026-10-10 — Windows Neovim Git long-path boundary

Candidate `28c92f5` Windows Neovim job `114302869144` reaches synchronization but
Git rejects the long `lazy.nvim.stage.3740/.git` path during `git init`. This is
not the previous compiler-consumer drift: execution passed that approval boundary.
The failure-only Node diagnostic reports an intact Node payload on this target.
Both Linux Neovim jobs (`114302869031`, `114302869093`) complete successfully.

The pinned [Git for Windows long-path contract](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/Documentation/config/core.adoc#corelongpaths)
explicitly enables builtin support beyond 260 characters; it defaults off.
PortableGit's existing preparation now sets this in its own `etc/gitconfig`,
preserving other upstream settings, before publication/fingerprinting. Its new
recipe revision participates in desired identity. Omitted revision remains valid
historical provenance; complete old generations update normally without rewriting
old records, while unfinished old preparation fails with recovery guidance.
Failed setting commands cannot publish. No personal/global Git file is changed.

The pinned-checkout helper intentionally sets `GIT_CONFIG_NOSYSTEM=1` and null
system/global files, so it passes `-c core.longpaths=true` explicitly and uses
Git's `/dev/null` spelling on Windows as well. Exact locked Lazy `306a055` inherits
`uv.os_environ()` in `manage/process.lua`; Mason `16ba83b` clones through its
inheriting spawn boundary; `sync_check.lua` inherits as well. Synchronization
staging sets no system-config suppression. Caller Git overrides keep their normal
precedence; there is no process-wide environment count parser or added wrapper.

The real-Git regression failed before preparation with `core.longpaths=false`;
the correction sets it to true and preserves an unrelated existing key. Archive
checks cover old shapes/updates, rejected unknown revisions, failed preparation,
publication and exact cleanup. The Lua fixture creates and verifies a checkout
beyond 320 characters without changing the ownership path. The disposable Windows
Git lifecycle now uses its private system configuration and tests long init,
clone, checkout, diff and commit verification without extra long-path flags.
Local POSIX execution does not prove Windows path behavior; actual private Git
system-config lookup and the full Windows Neovim lifecycle still require hosted
execution. Integrity checks, fixture deadlines and ownership identities remain
unchanged. The parent owns the integrated full gate.

Focused verification: all PortableGit regression tests pass with Go race in
3.267 seconds; all 12 pinned-checkout Lua tests pass using private Neovim 0.12.5
and HOME/XDG directories. Windows amd64 test compilation, StyLua, changed-file
EditorConfig and `git diff --check` pass. No native Windows result is implied.
