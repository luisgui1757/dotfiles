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
