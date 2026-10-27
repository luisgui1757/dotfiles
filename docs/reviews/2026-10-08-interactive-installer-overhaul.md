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


### 2026-10-10 — Mac Neovim Node-gyp bytecode proof and correction

Candidate `28c92f5` macOS Neovim job `114302869072` fails final Node verification.
The corrected reference diagnostic proves exactly 21 added `__pycache__`/`.pyc`
entries beneath npm's vendored node-gyp; upstream content and the current link
match. This rejects the earlier unproved npm-prefix/global-install explanation.
The source of the same pinned Node archive shows `configure.js` spawning Python
with `gyp_main.py`, which imports the bundled `gyp/pylib` tree.

The canonical sync/check child environment now supplies Python's documented
[`PYTHONDONTWRITEBYTECODE=1`](https://docs.python.org/3.13/using/cmdline.html#envvar-PYTHONDONTWRITEBYTECODE).
Construction remains pure. All three install phases and clean verification use
that environment, without changing the caller's process or personal Python
configuration. No new wrapper, dependency, bytecode exclusion or fingerprint
relaxation is introduced. The setting controls these installer descendants;
external npm builds retain ordinary Python behavior and can still create caches
inside vendored sources. Such externally changed managed bytes remain detected
and preserved under the existing ownership policy.

An actual private, checksum-pinned Node 24.21.0 / Python 3.13.16 gyp invocation
fails before the change: its full Node snapshot starts at `e58043ce…`, the exact
saved hash in native CI, and changes after `gyp_main.py --help` imports. After the
correction, the same consumer leaves the complete snapshot unchanged in both
sync and verification environments (33.24 seconds including fresh downloads).
This proof invokes the real vendored consumer, not a mock Python module. It is
also called by the existing complete-bootstrap native fixture, reusing its
private archives. Focused command-boundary coverage checks every phase and that
the parent environment remains unchanged. Native Windows and the complete Mac
installed-application lifecycle remain separate pending proof; no wider runtime
claim follows from this bounded gyp test. Parent owns the integrated full gate.

All focused Neovim-sync race regressions pass in 3.437 seconds. Windows amd64
test compilation, changed-file EditorConfig and `git diff --check` pass. The
initial local proof setup used a logical macOS temporary alias and was rejected
by the existing archive link guard; resolving that private fixture root retained
the guard and allowed the actual failing-before/passing-after consumer proof.


### 2026-10-10 — Windows clangd quality diagnostic gap

Windows quality job `114302869152` on `28c92f5` fails the unchanged two-project
clangd fixture at `buf_request_sync`, after both root-specific clients appear.
The same test passed in 6.39 seconds in the `56a4860` Windows log. Source comparison
finds no changes in this spec, production `lsp-config.lua`, minimal init or the
PowerShell spec runner between those candidates. Both use Neovim 0.12.5.
This does not prove a flaky test or justify extending its 15-second deadline.

The [pinned Neovim implementation](https://github.com/neovim/neovim/blob/v0.12.5/runtime/lua/vim/lsp.lua)
filters uninitialized clients from `get_clients` by default. The existing wait
therefore already verifies initialization: rejecting the request as an ordinary
missing-initialization race is unsupported. `buf_request_sync` returns an error
string that the fixture previously discarded, and the
[transport](https://github.com/neovim/neovim/blob/v0.12.5/runtime/lua/vim/lsp/_transport.lua)
forwards server stderr to its logger, outside the current assertion output.

The fixture now preserves that request error and captures only this process's
own clangd stderr events while forwarding the original logger unchanged. It
retains an 8 KiB tail and reports executable/compiler paths, command, client roots,
initialization/stopped state and exits only on failure. It restores the logger
before normal cleanup and never reads a shared/personal log. Product LSP config,
project roots, request/deadline, client distinction and clean-diagnostic assertions
are unchanged; there is no retry or alternate success path.

The original real-clangd fixture passes locally with private HOME/XDG directories
and pinned Neovim 0.12.5, both before and after instrumentation. A process-boundary
LSP fixture answers initialization, emits over 10 KiB of stderr, then leaves the
document request unanswered. It still fails at the original deadline and includes
the bounded tail/client state while omitting the discarded prefix. This proves
the diagnostic path only. The Windows server cause and a native successful rerun
remain required; no product fix is claimed.


### 2026-10-10 — macOS application alias inventory correction

Hosted desktop job `114302869025` at `28c92f5` reports NSWorkspace PID 7459,
followed by no owned PID/window and no private Code main.log. A non-GUI private Go
process reproduces the inventory defect: `ps comm` reports the public directory
alias, whereas the kernel reports its physical executable. The old fixture also
incorrectly includes an outside executable reached through an alias beneath the
package root. Both assertions fail before the correction, without launching any
GUI or changing workstation registrations.

The Darwin-only fixture now enumerates the current user's processes through the
existing x/sys boundary and queries `PROC_PIDPATHINFO`, the native operation used
by Apple's [proc_pidpath implementation](https://github.com/apple-oss-distributions/xnu/blob/main/libsyscall/wrappers/libproc/libproc.c).
The fixed 4096-byte buffer follows the SDK maximum. A resolved package-root
boundary determines ownership; argv and `ps comm` confer no authority. The
[kernel implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/proc_info.c)
can report an exited process or missing executable vnode: neither permits cleanup.
Other query errors and malformed paths remain failures. The native regression
covers Unicode/spaces/long ancestry, an outside alias, an unlinked outside
executable, cancellation and actual owned cleanup while both outside children
remain alive. Existing real window/configuration witnesses and deadlines remain.
Only the Darwin branch changes in the shared native fixture; production routing
and Windows/Linux process inventories are unchanged.

Rejected explanation: a missing private main.log does not prove Code exited or
ignored HOME. The exact pinned [Code user-data selection](https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/src/vs/platform/environment/node/userDataPath.ts)
uses Node os.homedir on macOS; [mainImpl](https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/src/mainImpl.ts)
sets that userData path before ready. Node's [POSIX contract](https://nodejs.org/api/os.html#oshomedir)
honors HOME. Explicit portable/appdata/CLI overrides precede the default, but this
trace establishes no such override or delivery failure in the hosted child.
No HOME/path workaround or direct executable launch is added. Correct inventory
is not GUI acceptance; the original published application must still pass its
native window and extension-host settings witness on the disposable runner.


Focused verification passes: the native race suite and existing Swift launcher
build/rejected-input test (2.985 seconds), Go vet, Windows amd64/Linux amd64 test
compilation, changed-file EditorConfig and diff checks. The failed-before log,
passing log, primary source snapshots and handoff hashes are retained as
`desktop-mac-inventory-*` artifacts. These are ordinary private process tests,
not an actual application/font installation or a macOS GUI pass; the parent owns
the integrated gate and hosted acceptance.


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

### 2026-10-10 — Preserve outside Apple receipt files without treating BOM as ownership

At exact head `28c92f5`, missing-Apple job `114302868957` passed preinstalled reuse,
then correctly obtained `pkgutil --only-files` and rejected Apple's historical
`private/tmp/.BBE...` file before moving any payload. The premise that every BOM
file must be beneath CLT was false: membership records historical installation,
not present filesystem state or cleanup authority. The fixture now validates
relative receipt paths, records all existing outside files before any developer
payload move and verifies their bytes, link target, mode, owner, mtime and inode
after provisioning. It never moves or deletes these outside files. Historical
absent entries grant no ownership or pre-existing data to preserve.

The original guard fails the real temporary-file shape in
`v5-apple-outside-before.log`; eleven corrected Python boundary/ordering tests
pass in `v5-apple-outside-after-python-corrected.log`. The Go race regressions
(`v5-apple-outside-after-go-corrected.log`, 1.404 s) detect modified bytes,
permissions and removal using private temporary files only. Bounded path input,
failed queries, traversal and invalid receipt roots remain fatal. The first
post-fix Python invocation included the new native phase in a fake command block;
that harness failure remains recorded separately. No native package mutation or
full missing-payload acceptance ran on the developer workstation. The disposable
macOS provisioning lane must still pass.

### 2026-10-10 — Opus v4 F2/F3: selected palette and passive LazyGit bytes

Accepted F2: the active POSIX lsd integration supplied aliases but only selected
zsh set Rose Pine `LS_COLORS`. Extract the existing exact palette into one POSIX
fragment and emit it once when either lsd or zsh is selected, before zsh
completion. No graph node, new setting or duplicate palette is introduced. The
actual generated initializer fails in Bash before correction and passes for lsd
only, zsh/shared selection, explicit `DOTFILES_LS_COLORS`, and unselected
preservation (`v5-shell-palette-checkout-before.log` and corrected focused log).

Accepted F3: the passive `lazygit/config.windows.yml` destination bypassed the
retired home-template hash test. Its expected hash now comes from the recorded
release metadata, with both an inventory assertion and physical LF checkout
proof under `core.autocrlf=true`. A deliberate passive-file mutation is rejected
(`v5-lazygit-passive-mutation-red.log`); the original physical checkout also fails
before the exact LF attribute is added. Active LazyGit defaults are independent.
README now identifies this Windows file as passive migration evidence.

### 2026-10-10 — Completed v4 matrix and review reconciliation

Exact published `28c92f5` completed 14/23 canonical jobs successfully: 12/20
installer and 2/3 quality. All four fresh APT lanes prove full Neovim provisioning;
fresh owned Build Tools and the two Ubuntu Neovim jobs pass. Public Windows
setup/migrate, Rust/LaTeX/Yamllint, fonts and Terminal Save pass inside failed
parent jobs and remain scoped evidence. No failed job is relabeled successful.

The real Opus v4 review completed with model `claude-opus-5-5`, requested xhigh,
176 read-only calls and all 829 immutable input files unchanged. Effective effort
was not exposed. F1/F2/F3 are accepted; E1's thirteen original red/green logs
exist and now match the original handoff hashes in a supplemental review bundle.
They were omitted by evidence selection, not fabricated or rerun as substitutes.
Correction slices are reconciled in the sole delivery writer. Chained Windows
Markdown patches needed their exact predecessor preimages; the initial HEAD-only
merge attempts failed explicitly and preserved all code and source evidence.
Independent status rows and append-only review records were reconciled manually.

Temporary diagnostic head `48fbcc8` and run `38085071529` create no PR or
required-check substitute. Actual PowerShell 5.1 extraction, COM and stream
boundaries pass; its process and Git runtime steps remain failed. Immediate exact
child-handle diagnostics identify 2–3 ms of asynchronous rundown. Fresh native
compiler success alone does not close that process contract. Production/runtime
correction, final integrated gates, native matrices and final review remain open.

### 2026-10-10 — Linux Code sandbox boundary and canonical native package

Accepted: both Linux desktop failures at `28c92f5` identify Chromium's sandbox
startup, not IPC. The runtime directory exists. Exact pinned official amd64 and
arm64 DEBs contain root-owned mode-0755 `chrome-sandbox`; the canonical Code
executable matches Ubuntu Resolute's shipped AppArmor `vscode` profile. Use APT
for this fixed package, retain its operation-attributed pool and dependency
guards, and preserve all macOS/Windows archive routes. No sandbox flag, custom
policy, setuid user-tree helper or shortened application path is introduced.

Rejected candidate: a relative temporary debconf override and APT working
directory do not protect maintainer scripts. The real package probe changed its
repository sentinel because dpkg deliberately chdirs scripts to `/`
(`src/main/script.c`, `maintscript_pre_exec`). The accepted fixed filtered input
uses an exclusive operation-bound native `/var/tmp` directory. Real apt-utils
install/removal preserved the original true answer plus repository/key bytes.
The native worker stays unchanged. Unknown command outcomes retain saved inputs;
exact original pins survive retries; unexpected temporary contents are preserved.

The regression first failed on the old Linux archive binding. Focused Code, APT
and desktop checks pass after the route change; actual provider and final hosted
GUI acceptance remain separately required. Tests cover preexisting reuse, saved
completion and unknown outcomes, conflicting personal commands, unchanged legacy
APT command records, and Code pool collection with outside consumers preserved.

The first actual provider lifecycle passed install/recovery, repair and update,
then failed removal because newly introduced `systemd-sysv` is marked Protected.
This was an inventory omission, not a dependency-order failure. dpkg correctly
refused removal and retained its dependencies. Current inventory now reads both
[Essential and Protected](https://manpages.debian.org/trixie/dpkg-dev/deb-control.5.en.html)
and feeds the existing pool filter. Their dependency closure stays owned and
disclosed; no force option or static package graph is added. Missing legacy flags
do not authorize removal: every dispatch checks current native observation.

Actual native worker/provider install with lost-reply recovery, repair, update
and removal now pass in disposable Ubuntu 26.04 ARM as root (99.43 seconds) and
Debian 13 ARM through noninteractive sudo (93.07 seconds). Both had apt-utils
enabled; every phase preserved the personal repository/key bytes and original
true debconf answer. These runs use explicit disposable-container gates, not
forged GitHub runner identity. They prove package semantics, while hosted Linux
amd64/arm64 configured GUI startup remains pending.


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


### 2026-10-10 — Native HANDLE rundown confirmed and exact completion awaited

Temporary diagnostic head `48fbcc8b5e258298630680f8d6e6edc279e57035`, run
`38085071529`, job `114309772969` is failed evidence. Its raw log SHA-256 is
`d8aa147fa2d5a59c98408438da3966adb3209d76ab8b562865ab2b0fcc55cd7c`. All five
process-tree modes report an initial unsignaled exact child HANDLE despite exit
code 1, then a signaled HANDLE after 2.21–3.45 ms. This confirms final process
object rundown rather than an uncontained, still-executing child in these cases.
The original immediate completion assertions remain failed; no grace period
changes their outcome.

[TerminateProcess is asynchronous and requires a process HANDLE wait](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-terminateprocess).
Termination now captures the [private job's member PIDs](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_process_id_list),
opens exact synchronization handles and verifies each with [IsProcessInJob](https://learn.microsoft.com/en-us/windows/win32/api/jobapi/nf-jobapi-isprocessinjob)
before terminating the job. It awaits every captured HANDLE within the existing
single five-second deadline, then requires empty accounting and unchanged job
lifetime membership. The bounded member buffer, failed opens, changed identity
or new membership all fail the read-only inspection; none can authorize a
success from a partial snapshot. Only the private job is terminated. This avoids
unconditional delays, per-handle timeout multiplication, unrelated-process
filtering, and treating an exit code as completion. The existing five native
cases still require immediately signaled children and surviving outside work.
A new real-kernel fixture creates a late job member between capture and
termination and requires explicit refusal of that incomplete proof.

The same native diagnostic passes PowerShell 5.1 extraction (4.73 s, all four
cases), private COM shortcut install/check/remove (1.39 s), shortcut progress hash
protocol (0.75 s), and read-only desktop query progress/error handling (1.28 s).
These are actual Windows passes, not cross-compilation claims.

Git/Make remains failed. The `Git version` diagnostic label covered both version
equality and the newly added private core.longpaths query, so the log proves only
that one of this pair failed before LFS/GCM/SSH. The probe now labels them
separately and reports only version text, bool value or exact failed query exit.
The original strict comparisons, physical paths, byte pins and GCM trace remain;
there is no guessed .NET fix or weakened acceptance. The next temporary native
run must verify exact process completion and resolve the earlier Git assertion.

Focused local preflight: all PortableGit tests pass with Go race (1.986 s);
Windows amd64 test-binary compilation and Windows-target `go vet ./...` pass.
Gofmt, EditorConfig, embedded Bash syntax, diagnostic workflow YAML lint and
`git diff --check` pass. The new exact-HANDLE and late-member cases cannot run
on this Mac; their red-to-green native proof remains the next hosted run.

The private-config failure branch additionally makes a read-only comparison with
`cygpath -w /etc/gitconfig`, reports whether Bash sees `MSYS_NO_PATHCONV`, then
runs the unchanged-path GCM version command with the fixture's existing bounded,
secret-disabled trace. It always returns the original strict config failure.
[MSYS source checks variable presence, not its value](https://github.com/git-for-windows/msys2-runtime/blob/main/winsup/cygwin/msys2_path_conv.cc)
while preparation currently exports an empty value. This is a concrete competing
path-conversion explanation, not yet native proof or an accepted fallback. No
configuration/authentication is changed by these additional diagnostics.


### 2026-10-10 — F1: test current generated shell behavior

Correction to the earlier ledger statement, "Active shell and tmux assertions
now test only their active recipes": the shell portion was inaccurate. Most
PowerShell and zsh behavior checks still read frozen release sources at `28c92f5`.
The active composer currently met the reviewed keymap/guard invariants, but those
checks could not protect it. Passive source hash checks remain the evidence for
released live-link compatibility; they are not current behavior tests.

A test-only Go exporter now calls the real composer and profile driver into an
explicit fresh private directory. The existing shell/Pester assertions consume
those generated init/attachment files. External initializer/plugin fixtures stay
at the process/module boundary; PSReadLine handlers and zsh keymaps are real.
Invocation classification and actual redirected/batch/CI child processes precede
profile-owned cache work. Pester checks real vi mode, Tab/Up history and PSFzf
configuration order. Its active cache test covers parsed UTF8 publication, valid
reuse, invalid-cache preservation and failed/empty/malformed generator refusal.
The unsupported old mtime caches, cache repair/retry helpers and chezmoi parity
assertions are replaced by the current contracts, with the retirement map updated.

The generated PATH test exposed a real missed behavior: the new zsh core omitted
`typeset -U path PATH`. The old suite passed its passive source; the active test
failed with duplicate `/bin` and `/usr/bin`. Restore only that native declaration,
preserving first occurrence and personal ordering. Active startup goes through
the published .zshenv/.zshrc in private HOME/ZDOTDIR; locale, palette override,
actual lsd rendering, cursor/vi/history/Tab bindings and selected fzf ordering
remain covered. The Linux quality job already runs this shell suite; explicit
Go setup now makes generation reproducible in Linux/macOS quality jobs. The
existing real archive zsh lifecycle also gains the Linux script(1) invocation,
without a new job or relaxed assertion. Windows native selected-shell acceptance
now selects PowerShell/lsd and reads actual PSReadLine state from managed pwsh.

Rejected encoding finding: a plain-file exporter initially omitted the BOM and
triggered an analyzer Unicode warning. Production `ProfileDriver` already emits
a BOM for new Windows profiles. Using actual publication fixes the fixture;
there is no new encoding workaround or production mode. The generated script's
two `PSAvoidGlobalVars` warnings refer exactly to the process-wide ready sentinel,
which intentionally spans dot-sourced host profiles. The test binds both exact
rule/extent identities and rejects any additional diagnostic, with no blanket
rule suppression. Ambient PowerShell's own XDG cache is not profile-owned cache
work; the guard test observes the actual private shell-cache directory instead.

ShellCheck cannot parse zsh's native glob qualifier and the old passive file
suppressed that parser failure. Active zsh/generated init syntax is now checked
by zsh itself; actual Bash/POSIX scripts remain checked by ShellCheck, including
followed test helper sources. Static forbidden-pattern assertions scan active
recipes. The canonical required-check contexts and passive released files are
unchanged. Hosted Windows/Linux lifecycle proof remains pending after this
isolated correction; no everyday-host packages or profiles were installed.

Verification: the complete generated shell suite and static suite pass; Pester
passes all 24 cases without skips. Focused Go race checks pass in 1.568 seconds,
`go vet ./...` and Windows amd64/Linux amd64 test compilation pass, and all ten
passive-release inventory tests pass. The existing repository-wide analyzer
warning fingerprint is unchanged. The pinned Renovate validator/extractor passes
with 67 inventory records, accounting for the two explicit quality Go setups.

Mutation evidence distinguishes coverage from merely passing tests: changing the
active PSReadLine mode to Windows leaves the old passive Pester suite green, but
fails the new generated-profile suite (including actual PSFzf-observed vi mode).
Disabling the active interactive guard fails all three actual batch subprocess
cache assertions. Both mutations were restored before the green run. The PATH
regression failed before the approved declaration and passes afterward. These are
local process/composition proofs, not native Windows or Linux package-lifecycle
results; those existing hosted gates and the parent integrated full gate remain
required. Evidence and exact hashes accompany `shell-f1-handoff.json`.

### 2026-10-10 — Exact native process completion and Git path diagnosis

Temporary `9ff8dd0`, run `38085987819`, job `114312463601` passes all five
original immediate HANDLE-signaling/outside-survival assertions (1.69 seconds),
and the real late-member refusal (0.01 seconds). Bootstrap extraction (4.08 s),
private native COM (1.20 s), progress hash (0.63 s) and separated desktop queries
(0.85 s) pass. Git/Make still fails, so the parent job remains failed. Raw log
SHA-256 is `ffdfd483339d9137db16f13a2436cf837c81fd6a3797affd409f568ea70df14a`.

The Git version matches. The plain POSIX private-config query fails; the same
query with its native cygpath spelling returns true. Independent GCM tracing
identifies System.CommandLine FeatureRegistration creating its suggestion cache
under the same private TMP, reaching the legacy native directory-length bound.
The supported direct native GCM subprocess can receive the same physical TMP in
Windows extended-length spelling. MSYS treats TMP/TEMP specially before its
exclusion setting; claiming preservation through that uncertain round-trip was
rejected from upstream source. No native Git/Make pass or final source certification
is claimed before this correction and its full native rerun.


### 2026-10-10 — Native HANDLE green; exact Git and GCM path boundaries

Temporary head `9ff8dd0f6ec71f29912b6bd2d075e7cc786e4b9e`, run `38085987819`,
job `114312463601` is failed Git evidence and passing process-boundary evidence.
Raw log SHA-256: `ffdfd483339d9137db16f13a2436cf837c81fd6a3797affd409f568ea70df14a`.
All five original immediate HANDLE assertions and outside-process survival pass
(1.69 s), as does the new real late-member refusal (0.01 s). PowerShell 5.1
extraction (4.08 s), desktop query (0.85 s), actual COM publication (1.20 s) and
shortcut hash progress handling (0.63 s) pass. No assertion was weakened.

Git/Make remains failed (38.52 s). The expected and actual Git version match.
The original `/etc/gitconfig` query exits 1, Bash sees `MSYS_NO_PATHCONV` present,
and the same read-only query using `cygpath -w` returns true. This proves a path
representation defect, not missing private settings. The fixed probe now always
converts the path at the native Git boundary, following the [MSYS path contract](https://www.msys2.org/docs/filesystem-paths/).
The private `core.longpaths` preparation and normal published Git consumers stay
unchanged; physical ownership paths are never shortened.

The independent GCM trace reports .NET Framework 4.8.9345 and
`Directory.InternalCreateDirectory -> FeatureRegistration.EnsureRegistered ->
RegisterWithDotnetSuggest` before its version handler. The pinned [System.CommandLine
2.0.0-beta4.22272.1 source](https://github.com/dotnet/command-line-api/blob/2.0.0-beta4.22272.1/src/System.CommandLine/Invocation/FeatureRegistration.cs)
creates `system-commandline-sentinel-files` beneath `Path.GetTempPath()`. This
run's exact directory is 252 characters. The shipped GCM config already targets
.NET Framework 4.7.2 (extracted SHA-256
`a64e209b9476be9acb9e4a3b7813e0ba48321ae2da1b675cd584f5319dc0c66f`), so adding
already-default managed AppContext switches is not supported by this evidence.
The OS-level path failure instead calls for the [documented extended native
spelling](https://learn.microsoft.com/dotnet/standard/io/file-path-formats#skip-normalization)
of the same directory; [CreateDirectoryW supports that spelling](https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-createdirectoryw).

The fixed GCM `--version` check now uses the existing native session directly,
with a copied environment overriding only TMP/TEMP to `windowsExtendedPath` of
the original private directory. This is a distinct fixed phase with the existing
operation/attempt identity and bounded strict failure handling. It avoids MSYS's
special TMP/TEMP conversion, which occurs before generic environment exclusions
in [environ.cc](https://github.com/git-for-windows/msys2-runtime/blob/main/winsup/cygwin/environ.cc).
Other preparation programs retain their existing environment. GCM suggestions,
signed executable/config bytes, personal settings and default installed command
behavior are unchanged; there is no authentication, disabled registration,
shorter temp path, global policy change or accepted failure. Completed revision1
generations remain valid: only verification path spelling/transport changes, not
prepared settings or desired payload, so no recipe revision is bumped.

The process-boundary regression fails before the fix because no native GCM phase
receives its private extended temporary path. Focused tests require one fixed
version-only call, unchanged environments for other phases, drive/UNC/already-
extended spelling, and refusal when GCM fails. The hosted lifecycle additionally
requires the actual suggestion-registration file at the original >248-character
temporary directory for both installed generations. Its existing normal published
`git credential-manager --version`, offline Git/LFS/SSH/Make usage, update, check,
personal-config preservation and removal remain required. No local Mac execution
can claim that final native green; the next exact-head hosted run is decisive.

Focused local checks: the native-environment boundary regression is red before
this correction, then all PortableGit tests pass with Go race in 1.983 s. Windows
amd64 compilation, Windows-target vet, gofmt/EditorConfig, embedded Bash syntax,
temporary workflow YAML lint and diff checks pass. The temporary ordinary Windows
step also includes the fixed native-environment assertions and existing real-CMD
self-deleting-input regression before the complete Git/Make lifecycle.

Rejected review candidate: moving GCM to its direct native phase was alleged to
remove a nonempty stdout contract. Checking the original fixed Bash source shows
`git credential-manager --version >/dev/null`: its contract is successful exit,
while the separate published lifecycle checks the version. No additional result
field, guard or test was added from that mistaken premise. The signed binary,
prepared payload settings and strict exit propagation remain unchanged.

### 2026-10-10 — Reject unsupported Git working-directory assertion

Temporary exact head `db366b69e611d582827f9066f1115d3f5bf93705`, run `38086884459`,
job `114315143713` remains failed evidence. Raw log SHA-256:
`10d7cc361449ed0f560401c08f90d11f12f0c184c187804ecffc6b90273a0806`.
Both original archive install/update preparations and fixed native GCM version
checks succeed; the fixture observes actual suggestion files beneath each
original >248-character private temporary directory. Published Git consumes its
private system long-path setting and ordinary Unicode add/commit succeeds.
All five exact-HANDLE/outside-process cases, late-member refusal, PowerShell 5.1
extraction, read-only progress and actual shortcut COM regressions also pass.

The later failure is our added `git init` at an approximately 450-character
repository root, not extraction, missing system configuration or GCM. Pinned
Git's `mingw_chdir` still reaches `_wchdir` after normalizing the native path;
upstream [PR 3877](https://github.com/git-for-windows/git/pull/3877) reproduces this
separate working-directory limit. Its pinned [long-path regression](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/t/t2031-checkout-long-paths.sh)
covers long file paths within an ordinary worktree. Requiring an arbitrarily long
repository root was unsupported test overreach, explicitly rejected by the owner;
it was not the observed Neovim case (root 257 characters, `.git` 262).

Replace only that added root assertion with a >320-character directory/file path
inside the original ordinary worktree. Plain managed Git add/commit and
`clone --no-local` into another ordinary root must preserve exact binary bytes
through checkout, diff and commit verification, with no long-path command or
environment override. Binary fixture contents isolate this check from unrelated
line-ending conversion. Original payload/home/Neovim paths and all normal init,
interactive staging, GCM/LFS/SSH/Make, ownership and removal assertions remain.
No production code changes. Native completion of those later assertions remains
required; local compile/static checks cannot turn the earlier failed job green.

Focused preflight passes: Windows amd64 test compilation, Windows-target Go vet,
gofmt, EditorConfig and `git diff --check`. The native fixture is compiled, not
executed locally. Its complete hosted rerun remains required.

### 2026-10-10 — SSH fixture identity path uses its filename option

Exact diagnostic head `4eea1ff115e7048609693e09f7d5c4ad6f8968d2`, run `38087734757`,
job `114317661301` reaches SSH configuration inspection after passing both archive
preparations, original long-temp GCM registration, changed-version update,
428-character file operations, interactive staging, LFS and published GCM. Its
raw log SHA-256 is
`ea76bcf088c1b3d3e2f8c623d37257dcde99e9c13f305b44d5268e6a2c986b0a`.
All ordinary PowerShell/COM/private-process boundary tests also pass. The full
Git/Make lifecycle remains failed at SSH, before Make, final integrity/personal
configuration verification and removal; these later assertions are not waived.

The fixture supplies `-o IdentityFile=<path with spaces>` as one argv value.
OpenSSH reports `keyword identityfile extra arguments at end of line`: `-o`
applies configuration-file parsing after argv handling. The primary [ssh(1)
contract](https://man.openbsd.org/ssh.1) provides `-i identity_file` for this exact
filename boundary. Change only those two arguments, retaining the same physical
path, `-F NUL`, offline `-G` mode, hostname assertion and every later check. A
multi-location search finds no other `IdentityFile=` construction. This is a
fixture syntax defect, not a bundled SSH or managed-path failure.

A real local `/usr/bin/ssh -F none -G` process with a private spaces/Unicode path
reproduces the identical error and exit 255 using the old arguments; `-i` exits 0
and emits the exact identity path and expected hostname. No connection,
authentication, personal SSH config or native package mutation occurs. The
portable parser proof complements the native failed-before evidence; it does not
claim the pending corrected Windows lifecycle is green. Windows amd64 test
compilation, target vet, gofmt, EditorConfig and diff checks pass.

### 2026-10-10 — MSYS OpenSSH uses explicit no-config mode

Exact head `07e5ed15e79d1f4c60eb735d2e228586187209d1`, run `38088366432`, job
`114319529231`, passes ordinary boundaries and the unchanged long-file/LFS/GCM
checks, then reports `Can't open user config file NUL: No such file or directory`.
Raw supplied log SHA-256:
`eb7cd6895bd6767da63ce528daf4dd0cbf140b3f6a386c01f2744f3557122c22`.
This later parser boundary was obscured by the preceding IdentityFile syntax
error; the successful `-i` correction did not establish config-file isolation.

The exact PortableGit archive rehashes to its pin
`075e158ef8e1f0ab80b347e245405d3eca735c2dc88fd8e032e137d0ca61f61b`.
Its `etc/package-versions.txt` identifies OpenSSH 10.5p1-1. The matching
[ssh.c process_config_files](https://github.com/openssh/openssh-portable/blob/V_10_5_P1/ssh.c#L541-L560)
and [ssh(1)](https://github.com/openssh/openssh-portable/blob/V_10_5_P1/ssh.1)
explicitly support `-F none`, bypassing both personal and system config reads.
Change only `NUL` to `none`; keep the exact key path, dedicated `-i`, offline `-G`,
hostname check and all later Bash/Make/integrity/preservation/removal assertions.
No private empty-file fallback, path reduction, skipped assertion or product code
change is needed.

A real local OpenSSH query reproduces the same missing-NUL error and exit 255;
`-F none` exits 0 and reports the same identity path and expected hostname, even
with an invalid file named `none` in the private working directory. No connection
or authentication occurs. A nearby multi-location audit finds no other executable
NUL boundary; remaining occurrences are archive rejection fixtures or explanatory
comments. Later Bash assertions use actual MSYS `/etc` and `/dev` paths. GNU Make
receives a dedicated full-path SHELL value, preserved by its Windows shell lookup;
no further demonstrable path defect was found, so those assertions stay intact.
Windows compilation, gofmt/EditorConfig and diff checks pass. Complete native
Git/Make acceptance remains required; this failed context remains failed.


### 2026-10-10 — v5 F1/F3: shared Bash grammar and supported Git boundary

The review's Bash parser finding is confirmed. `shell-v6-bash-red.log` records
`bash -n` rejecting the generated zsh-selected initialization at the native glob
qualifier. The previous palette test consumed stdout without proving that the
rest of initialization ran. Keep the existing shared profile and delay only the
fixed compdump condition through a literal `eval`. No path/environment content
becomes executable command text; zsh still expands the same condition at runtime,
while Bash can parse its inactive branch. The regression requires successful
Bash syntax, complete interactive initialization, its ready marker, the selected
lsd alias and empty sourced-initialization stderr. Bash's own non-TTY startup
notice is captured separately for failure diagnostics. Active lint checks both
Bash and zsh grammar. The focused race checks pass in 1.556 seconds, and the
complete active shell suite passes, preserving the existing real zsh behavior.

The Lua long-path spec's >320-character repository root contradicted the already
accepted Git fixture contract. Pinned [mingw_chdir](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/compat/mingw.c#L1189-L1214)
normalizes the path then calls `_wchdir`; [the upstream regression](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/t/t2031-checkout-long-paths.sh)
covers long files inside a supported root. The corrected spec calculates an
exact 257-character stage including the current PID suffix, verifies its `.git`
path is 262 characters, and commits/checks out a nested file beyond 320 characters.
Only remote fixture construction explicitly enables long paths; the actual
production helper still supplies its own flag while excluding system/global
configuration. Exact file contents, checkout verification and stage cleanup
remain assertions. All 12 specs pass on pinned Neovim 0.12.5 locally. This is not
a Windows pass; its existing quality/native jobs must execute the corrected case.

Rejected default-layout headroom prediction: the recorded 257-character stage
includes a private TEMP/test-name/data-root suffix, not the default installation
layout. `DiscoverConfigFolders` uses actual LocalAppData for data unless explicitly
overridden; `nvimRuntimeDirectory` appends `nvim-data/dotfiles-runtime` independently
of StateRoot. The same observed generation and PID produce a 179-character stage
under `C:/Users/runneradmin/AppData/Local`, 78 characters shorter. Thus a username
two characters longer does not establish the claimed failure. Actual PID widths,
plugin names and custom/redirected data roots consume their actual margin against
the upstream current-directory limit. No arbitrary-root promise, layout redesign,
identity shortening or fixture-path change is justified by this evidence. Exact
component calculations are saved in `shell-v6-windows-headroom.json`.


### 2026-10-11 — v5 ordinary external npm bytecode finding confirmed

The review's Node concern is demonstrated through the real consumer. In a fresh
private HOME, pinned Node 24.21.0/npm 11.19.0 and Python 3.13 run an offline
`npm install` for a local no-op `binding.gyp` target, using the Node archive's
own headers. The npm/node-gyp configure and make phases succeed, then the entire
Node snapshot changes from `e58043ce...` to `825ea4b0...`: 18 `.pyc` files appear
under vendored gyp/packaging. The 35.16-second failed-before log is
`shell-v6-external-npm-red.log`. No personal configuration or native package
manager was used. The previous sync-only environment fix does not cover this
ordinary external invocation.

Reuse the existing exact pin `Replacements`: immediately before the unique
`import gyp` in each platform's bundled `gyp_main.py`, set
`sys.dont_write_bytecode = True`. [Python documents this process-local control](https://docs.python.org/3.13/library/sys.html#sys.dont_write_bytecode).
The SHA-verified archive must contain exactly the reviewed original occurrence;
a different or repeated source fails closed. This changes desired identity
through existing metadata, without a new recipe field, launcher, lifecycle or
fingerprint exception. Keep personal Python settings and user edits protected.
The new normal regression publishes the old shape, updates to the prepared shape,
checks the exact rendered source, verifies readiness and removes its owned data.

The corrected real offline npm install passes in 57.06 seconds with the complete
Node snapshot unchanged, successful archive check/removal and preserved personal
Python startup bytes/environment. The focused race suite passes in 3.671 seconds.
All-target archive checks decode foreign artifacts without claiming their
executable permission semantics on macOS; runtime proof remains native. Windows
Node ZIPs omit development headers, so that focused native fixture exercises the
same gyp import entrypoint directly without inherited bytecode suppression;
its required Neovim lifecycle remains the actual npm integration gate. Explicit
user modifications, including deliberately writing into the distribution, remain
subject to the unchanged ownership/fingerprint checks.

All four pinned archives pass checksum, extraction and exact single-occurrence
source preparation in 99.74 seconds (`node-v6-four-archives.log`). The POSIX
original gyp source hashes to `80eb0a19...`; Windows preserves its distinct
original bytes (`a4c2d327...`). Prepared sources hash to `bec0cbed...` and
`ddbc7ade...` respectively. Source/version/checksum pins are unchanged; only
reviewed preparation metadata changes. Vet, Windows test cross-compilation and
focused formatting checks pass. Actual Windows consumer execution and the
integrated full/native gates remain required; foreign source proof is not a
Windows runtime pass.

### 2026-10-10 — Hosted Linux Code health diagnosis

ARM desktop job `114321418527` failed the native Code ownership fixture before
GUI setup: canonical package health returned exit1 without identifying its
failed predicate. The same pinned package passed root and sudo container runs.
The fixed-path read-only probe now reports the failing symlink target or native
owner/mode, with existing output bounds; every health requirement remains intact.
Focused process-boundary tests prove failures stay failures and healthy checks
stay silent. The actual hosted metadata/cause remains pending; no permission,
sandbox or application path change is justified by the incomplete trace.


### 2026-10-11 — Opus v5 Linux Code lifecycle corrections

Accepted F2/F4/F5/F6/F7: controller regression first fails at the redundant
Linux desktop resource's missing archive, reused Code retains a needless shell
attachment, older pending APT completion rejects a newly observed Protected
flag, external Code version drift strands update/removal, and success leaves
its large DEB input. The correction removes Linux's duplicate launcher/shell
graph, keeps current native protection authoritative at removal, binds legacy
Code fingerprint compatibility to exact completed provider evidence, discloses
native current/pinned versions (including reviewed downgrade), and cleans only
verified transaction payloads at known success. Unknown worker outcomes and
modified inputs remain recoverable; legacy command arguments are never widened.

Hosted diagnostic run 38089764768 at `1206f557` failed identically on Ubuntu
amd64 (job 114323625317) and arm64 (114323625510): `/usr/share owner=0 mode=777`.
The exact reported runner-image tag `ubuntu26/20260927.149` contains
`chmod -R 777 /usr/share` in `images/ubuntu/scripts/build/configure-system.sh`.
Rejected fix: relax the production guard or change package payload permissions.
The approved fixture correction verifies and logs the disposable physical/root
parent, changes only `/usr/share` to 0755, and verifies/logs the result. Other
paths retain their strict health checks. This proves the earlier missing health
metadata; it does not itself prove GUI startup.


The first real Ubuntu 26 ARM correction probe reached the external native
upgrade and reviewed downgrade, then correctly stopped because the old APT
history parser rejected every `Downgrade:` field (120.16 seconds). The bounded
correction allows only the explicitly permitted saved Code identity and exact
Before.Version-to-pin tuple in both success and interrupted-attempt evidence;
foreign/additional downgrades, absent permission, removals and incomplete dpkg
completion still fail. The native fixture uses a repacked exact payload with
synthetic epoch version `1:1.141.0-1` solely to prove real dpkg/APT downgrade
semantics, not as an upstream release or production pin.

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


### 2026-10-11 — Correct native shell fixture provider composition

Accepted the two concrete v5 Linux core failures in job `114321418480`:
`powershell-script-composition` required a Windows VC++ resource while running a
Linux native catalog without that provider; the interactive zsh fixture omitted
observed APT even though `tool.zsh` is now APT-bound. Raw log SHA-256:
`94e0f4623367ef885ac7f751c025830ca3cccc7638610908281570e0a42726cc`.
The source of truth is the actual platform catalog and process-boundary provider
discovery, not the old archive/profile-only fixture assumptions. The same omission
also affected the actual Windows selected-shell fixture, so its native system
PowerShell discovery is connected in this correction.

Keep real native identity and prerequisites: Linux observes pre-existing healthy
APT zsh, executes the published interactive profile, then verifies reused shell
ownership and identical complete native inventory after selective/full removal.
Keep plugin composition, live fzf history versus normal history keymaps, shared
fzf retention and personal hook preservation. Windows observes healthy pre-existing
VC++ registration and verifies reused ownership/unchanged fingerprint after its
existing selected PowerShell/PSFzf lifecycle. Neither archive fixture installs a
native prerequisite; the separate fresh-provider lifecycles cover that behavior.
Do not emulate foreign package databases or remove dependency requirements.

Retire the foreign Windows native subcase in favor of the already active
`TestWriteActiveShellFixtures` / `Profile.Tests.ps1` process boundary. It composes
and publishes the actual Windows profile on each host, runs real PSReadLine and
checks invocation guards, vi-before-Fzf order and keymaps; no handler is mocked.
Pinned PSFzf/module integration, update/repair/deselection and personal-profile
preservation remain in the real Windows native case. The explicit mapping is in
`docs/installer-test-retirement.md`; this does not claim Windows runtime proof from
PowerShell on a POSIX host.

Actual verification: a new disposable Ubuntu 26.04 arm64 container (image
`sha256:f144425ff09be612d6d9ad965196e9cdc23dae1f42110a8a11a3e9a8198759f7`)
passes native selected-shell lifecycle in 2.68 seconds and interactive zsh lifecycle
in 17.75 seconds. The latter proves the unchanged outside APT inventory and reused
zsh receipt. Container setup installs fixture prerequisites before baseline and
removes the whole container afterward; the developer Mac is unchanged. Generated
PowerShell Pester passes 24/24 in 15.26 seconds. Focused Go race passes in 1.573
seconds; vet, Windows compilation and formatting pass. Actual Windows native
execution and the integrated/final-head hosted gates remain required.

### 2026-10-11 — Retained native clipboard blocked shell baseline restoration

Accepted graph defect, not a profile publication/recovery defect. Exact candidate
`de769314978411d63e509d260ae1e7ce51437f58`, macOS Neovim job `114321418694`, reached
normal editor/Pi use, update and both deselection requests before its original
profile/configuration snapshot assertion failed at `state/shells/env.sh`. The
raw job log SHA-256 is
`c4a72e73f1e47e029e46303ab96cbdbbc94af2e96ff20ca88cfd2f2f9e1bbfcc`.
A request reported ready does not mean every recorded resource was removed.

`MacOSPrerequisiteDriver` only reads the canonical OS clipboard commands and
correctly records their reuse. `tool.clipboard` nevertheless had an unconditional
`integration.shells` dependency. On deselection, the planner correctly retains
reused clipboard infrastructure, then its dependency closure retains the owned
shell blocks. A private fresh-controller lifecycle with the actual read-only
macOS clipboard provider reproduces the same snapshot failure for both absent
and personal baselines. Omitting clipboard from that lifecycle restores them;
this rules out a generic ProfileDriver removal or constructor-write failure.

Limit this dependency to Windows, whose private clipboard archive actually
requires the managed PATH. Linux APT helpers also use system paths; fixed
xclip/wl-clipboard installation and their dependency remain intact. Full-catalog
planning tests cover pre-existing, externally changed and explicitly retained
clipboard resources across macOS, Linux amd64/arm64 and Windows. Retained Windows
archives still protect their shell prerequisite. The fresh-controller regression
now restores every original shell snapshot after update, shared/selective/final
removal and read-only reconstruction/check, without deleting user bytes or
weakening retained-resource protection. No profile/state schema, ownership guard,
provider mutation or private-script cleanup workaround is added.

Focused native-macOS filesystem/controller tests pass; they only read OS command
files and write private temporary profiles. No clipboard contents, package,
font or personal installed state is changed. The full corrected hosted Neovim
lifecycle remains pending; prior consumption success does not certify its final
restoration on the new candidate.

Validation: the new regression fails against the original catalog and passes
with only the provider-specific dependency correction. The surrounding focused
profile/Bash-login/shell/clipboard/Neovim-prerequisite suite passes in 47.696s;
Go vet, all three foreign supported-target test compilations, EditorConfig,
JSON parsing and diff checks pass. No shared writer or frozen Code correction
was changed.

### 2026-10-11 — Native desktop folders and absent mode variables

Exact `de769314978411d63e509d260ae1e7ce51437f58` jobs `114321418370` (macOS) and
`114321418462` (Windows) fail the published Code singleton window assertion.
LaunchServices reports a PID, but neither case observes an owned window or a
private settings witness/main.log. These failed jobs remain failed; a successful
launch API return is not runtime readiness.

A real read-only Foundation executable, given private fake HOME/XDG directories,
still returns the real user home and native Library/Application Support. The
macOS fixture's redirected shell folders therefore cannot establish the actual
GUI settings contract. Use discovered native folders on both macOS and Windows;
the existing adopted configuration snapshots, private witness, public singleton
launch, update and surgical restoration remain required. No everyday-Mac GUI or
package mutation ran.

VS Code 1.141.0's [.npmrc](https://github.com/microsoft/vscode/blob/1.141.0/.npmrc)
binds Electron 43.7.7. Its [Windows entrypoint](https://github.com/electron/electron/blob/v43.7.7/shell/app/electron_main_win.cc)
uses getenv_s's required-size test for ELECTRON_RUN_AS_NODE; an empty value can
still denote presence. The [macOS entrypoint](https://github.com/electron/electron/blob/v43.7.7/shell/app/electron_main_mac.cc)
checks nonempty contents instead, so this is not claimed as the macOS failure
cause. The controlled fixture clears its seven existing mode/resource overrides
by removing names from child environment vectors. It does not change the caller
or product launcher behavior. A real private child process reproduces presence
with the old empty/nonempty environment and absence after cleanup, while its
private HOME remains exact. The first red-harness attempt retained now-unused
imports and failed compilation; that setup failure is retained separately from
the subsequent behavioral red.

Swift launch completion now reports bounded executable-path/termination metadata;
Windows ShellExecute reports its returned PID. Neither report grants ownership,
changes a deadline, bypasses sandboxing or substitutes for a native window and
settings witness. Private process-inventory/outside-survival and the Swift helper
compile/reject-missing-input regressions pass without opening an application.
Corrected configured GUI lifecycles remain pending hosted proof.


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


### 2026-10-11 — Reject the 257-character Git-root premise; retain the real 254-character regression

Supersedes the earlier F3 assertion that a 257-character staging root was supported
because it remained within the current-directory ceiling. Actual diagnostic head
`810f50e`, run `38090569304`, job `114325970355` disproves that premise: all three
same-path variants fail init with exit 128 before reading configuration. Normal
and fixed isolated-system config both reject the 262-character `.git`; explicit
extended git-dir rejects path canonicalization too. Raw log SHA-256:
`a501002268ace3d8afdab757faa0d2d90a43730aa1812ef7cee5ef4412d6413d`.
The encompassing Git/Make test passes in 95.35 seconds because the diagnostic records each
outcome; that step status is not a successful variant or a Neovim fix.

Exact pinned [Git init_db](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/setup.c)
canonicalizes git_dir before reading repository/config state. Its
[Windows path adapter](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/compat/mingw.c)
returns false for long-path support before the repository config hash exists and
rejects expansion before considering an existing extended prefix. Later
repository discovery also reads HEAD/objects/refs before normal config. Therefore
neither the CWD ceiling nor an isolated system setting alone establishes full
checkout support; native init through verify must succeed at the same path.

The actual de769 Neovim failure is stage 254 / .git 259 / config 266 (job `114321418451`,
SHA-256 `501b9b5c1b5b22d21e6dae5f832e4abec6f6c7da28de92d6a674b6f485e0fa0f`).
Correct only the Lua fixture's padding and matching assertions to that observed
case; retain nested files beyond 320 characters, bytes, exact locked identity, transactional
publication, lock and failure cleanup. 257 remains a diagnostic unsupported
control, not an active promised root. All 12 specs pass on real pinned Neovim
0.12.5 macOS (`git-v6-pinned-spec-macos.log`); Windows production correction and
full-consumer proof remain open. No source/runtime layout, hash, home, identity or
published physical path changes in this slice.

Multi-location confirmation: macOS v5 core job `114321418362` has the same sole
foreign-Windows `powershell-script-composition` provider failure already corrected
by the separate native-shell fixture patch. Its actual Mac shell/zsh/tmux cases
pass. Log SHA-256 `5040510d950ada75f7b42f8225403a3ff9963350a31c473ca3fe217a92355db4`.
No additional product or Mac fixture change is justified by that duplicate.

### 2026-10-11 — native PowerShell module inheritance

Quality job 114321418460 at de76931 failed Get-FileHash in extraction and both
shortcut boundaries. Its parent was PowerShell 7 invoking Go; core job
114321418397 launched Go from Bash and passed those same three boundaries.
[Microsoft's module-path contract](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_psmodulepath#starting-windows-powershell-from-powershell-7)
explicitly identifies indirect PS7 launches: the intermediary keeps PS7 module
paths, which can resolve Utility to an incompatible edition and raise
CommandNotFoundException. This supports a module boundary correction, not a
timeout, warm-up, retry, or engine fallback.

Existing encoded fixed scripts need only engine-shipped modules, so the common
argument builder sets their child process's module search to `$PSHOME/Modules`.
The public bootstrap imports its own Management/Utility manifests by exact path,
retaining the caller's PSModulePath. The extraction fixture invokes that actual
binding before its existing archive hash and strict extraction assertions.
The native regression launches PS7 → Go → PS5.1, requires the unmodified hash
command to reproduce the original error, then validates actual SHA-256 through
vendor, Job Object, integration, durable worker and bootstrap boundaries. All
files are private; no profiles, registry or global environment change. Changed
encoded commands remain subject to existing saved-command identity validation;
old in-progress commands are not silently reinterpreted. Native proof is pending.

The core Build Tools reuse deadline is separate: it occurred under Bash without
a module error. The existing 30-second query limit remains unchanged; bounded
native phase diagnostics are required before attributing or changing it.

Regression coverage explicitly preserves/refuses both legacy unprefixed VC++ and
Build Tools pending commands without journal writes or another native call;
existing completed-command replay and unfinished-intent refusal tests pass.


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

### 2026-10-11 — GUI launcher output outlives its root process

Accepted native-fixture defect. Diagnostic head `a5493783d8acfe1b6cb0c17dc7bfd19bb38061e8`
run `38092318318` passes actual Code install/recovery/repair/approved downgrade/
removal on Ubuntu arm64 (86.51s) and amd64 (91.26s), and native font lifecycle
(6.64s/7.39s). Both public desktop tests reach `vscode-singleton` and remain there
for over nine minutes despite a 90-second context. ARM records the actual
`/usr/share/code/code` process activating the accessibility bus. The scoped
run was then cancelled, not passed. Raw arm64/amd64 job log SHA-256s:
`f95e66a00b3f1d273d31bdbf8fef97321f25d79568a8aee80cffadb2db966f4d` and
`a77ebdea4a76ba3eff469f8da30e06ae4a5b9645c4c7621c8ecb9fe138922ce2`.

The fixture captures launcher output through a Go writer. A launched GUI can
inherit those pipes after its successful launcher exits, leaving `Cmd.Wait`
blocked on copy goroutines that context cancellation cannot complete. A private
root/child process reproduces root exit 0 but a 3.028s wait with a 300ms context;
regular OS file handles return in 14ms while the child remains alive. No GUI
was launched locally. The focused regression fails on the original writer-pipe
operation and passes with exclusive private file handles; it releases the exact
private child and joins the waiter on both paths. Separate cases retain nonzero
root exit, context cancellation and the 1 MiB diagnostic bound.

Change only native test launch capture. Preserve the original 90-second context,
all owned-process/window/settings witnesses and exact owned-process cleanup.
Read bounded output on launch completion and again after owned GUI cleanup;
late output/read errors still fail the subtest. Do not ignore ErrWaitDelay,
extend deadlines, treat root exit as GUI proof or use a detached Wait goroutine.
No production provider or sandbox behavior changes. Native corrected GUI
acceptance remains required; the cancelled jobs only prove their completed Code
and font steps plus the recorded stalled phase.

Validation: failed-before process regression reproduces the inherited-pipe wait;
corrected focused cases pass (0.553s), including explicit nonzero exit,
cancellation and output-limit boundaries. Race tests pass (2.610s); Go vet,
Linux amd64/arm64 and Windows amd64 test compilation, EditorConfig and diff
checks pass. No everyday-workstation GUI or native package mutation was used.


### 2026-10-11 — Reject unsupported native Git roots; test the actual Windows application path

The `3cedd3e` diagnostic (run `38092637104`, job `114332033874`) has eight failed
variants, despite its encompassing Git/Make step passing. Exact raw SHA-256:
`bebe8e8b22024989136ed7d5e4ea36ebac99dd4c4381cddcff7fcca84e61859d`.
At stage254/config266 the fixed system setting advances init only; both ordinary
and explicit `.git` paths then fail remote with “not a git repository”. At257,
all variants fail init. The extended prefix also fails canonicalization at254.
This supersedes the hypothesis that isolated system configuration alone could
correct the actual bootstrap. No failed variant is counted as successful proof.

Pinned [Git setup.c](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/setup.c)
checks HEAD/objects/refs before ordinary configuration; its
[Windows path adapter](https://github.com/git-for-windows/git/blob/v2.56.0.windows.2/compat/mingw.c)
disables expansion before the config hash exists. The actual v5 fixture puts lazy's
parent at233, lazy.nvim at243, and final longest plugins at258/259, with metadata
beyond260. A shorter temporary basename therefore cannot fix the complete product
consumer at that root. Reject metadata aliases, extra config machinery and cache
layout rewrites: none is necessary for the documented native default, whose same
full-identity paths are75 characters shorter (lazy stage179; longest final root184).
The earlier “two more username characters breaks the default” claim incorrectly
used the long temporary XDG root as the default. Neither canonical-path validation
nor core.longpaths promises unlimited native repository/CWD roots.

The corrected public Windows fixture observes real native known folders and leaves
XDG_DATA_HOME empty through preview/apply/check, failure diagnostics and normal
Neovim use. Private HOME, original profile/configuration snapshots, all existing
editor-data entries, notes, full operation IDs, exact approval and full lifecycle
assertions remain. Its new personal-data witness uses exclusive creation and
removes only unchanged fixture content. No production Git or staging code changes.
The Lua suite still checks files beyond320 characters through the real helper at a
supported root; a separate Windows case retains the exact254 failure, asserts the
explicit init error and proves unchanged original target plus stage/lock cleanup.
This is an evidence-backed correction of an unsupported-root assertion, not a
fallback, ignored failure, removed consumer check or fixture-only path alias.

The existing real-process plan-diagnostic regression now also proves the explicit
empty XDG setting reaches the child; without forwarding it the child exits11 and
no preview is recorded. Focused local results and Windows compilation are recorded
in the handoff. Strict native default-layout Git plus the entire public Windows
Neovim install/use/update/shared-removal/final-restore lifecycle remain required
before this investigation is closed. The diagnostic probe is not delivery code.

### 2026-10-11 — native private-job capture proof

Temporary 8994dd0/job 114334530446 passes all five original process-tree modes,
outside-process survival, immediate exact HANDLE completion, unsafe-program
refusal and late-member refusal. The revised natural-exit regression identifies
test.test.exe plus conhost.exe through already-proved private-job HANDLEs; Active
falls 2→0 while lifetime Total stays 2. The exact root exits 0, and every initially
held HANDLE is immediately signaled after completion. This disproves the earlier
single-member fixture premise without weakening any production capture predicate.
Only the invalid comparison across independent accounting/PID snapshots is removed.

The full existing Build Tools reuse/environment/stable-observation test passes
20.37s: five queries take 10.093,2.733,2.157,2.646,2.725s under the unchanged 30-second
limit. The earlier deadline failure's exact stage remains unknown; this success
is not a causal explanation or a reason to widen the bound. Raw log SHA256:
`aa0e4178c2b441f40a8f3abb7fa3c5bb22d56b9dc75634421d5363aa4f817e0c`.
The complete job is still failed for a separate strict Git probe that inherited
the Git fixture's temporary LOCALAPPDATA. The temporary timing helper/workflow
and Git probe do not belong in the product patch. Frozen product code is
`windows-query-natural-final-code.patch`, SHA256
`73bd712e1623172695776de67eb7bb184afff8d8afcc1f71bb4672cf2ddb7a03`.

## 2026-10-11 — APT absent alternatives concealed existing browser consumers

Accepted production observation defect, isolated from the prior GUI-pipe fixture
correction. At `8a1af683120cfb5f15287d8c13d009d09565f87b`, ARM job
`114334224074` passed the complete configured Code/Ghostty lifecycle; amd64 job
`114334224279` passed Code/font lifecycles and the real Code singleton GUI, then
failed GTK4 approval. Diagnostic `7bf54fe5c2a0c526a521f3c5b08b883d165acaba`
job `114335724004` showed the exact difference: approved GTK4 absent with no
consumers; fresh GTK4 healthy with valid same-plan libadwaita introduction proof
and outside consumers `google-chrome-stable:amd64` and
`microsoft-edge-stable:amd64`. The raw log SHA-256 is
`56e47c9656fcad7b682d9cec4a76dd2a65feb12744b69037dfa430354073a2b7`.
Their existing alternative declarations became visible because the parser
previously discarded uninstalled names and Observe inspected consumers only for
present packages. No evidence supported changing the executor approval guard.

Keep unresolved declared dependency names in the existing inventory field and
observe outside consumers for absent selected packages. Installed aliases,
architecture identities and virtual Provides keep their existing resolution.
Unresolved references never fabricate installed packages. No persisted schema,
manager command, native ownership proof, graph order or approval comparison
changes. Old pending inventory without absent references still recovers through
its saved command and operation-specific evidence. Native removal remains
conservative about all installed alternatives; unchanged personal browsers and
GTK3 remain, and GTK4 is retained while declared outside use exists.

The controller reproduction fails before the fix at the same approval guard and
passes install/update/removal afterward, preserving the outside browser and both
alternatives until the outside consumer disappears. Parser :all/multiarchitecture/
Provides and legacy interrupted-operation coverage pass, as do existing manual,
source and newly introduced consumer rejection tests. Corrected full amd64
configured desktop proof is pending the next hosted run. The temporary diagnostic
helper/workflow/note remain outside the delivery patch.


### 2026-10-11 — Native root/refusal proof closes; Windows PSES prerequisite corrected

Exact head `43b7e544c54af3c6d1048b73a2ada80604aeb7a1`, run `38094448973`,
job `114337344648` proves the default Windows layout through real private pinned
Git init/remote/fetch/checkout/ref creation, publication, identity/cleanliness/diff
and long-file bytes. lazy.nvim is stage 179/target 168/config 180 with a 514-character
file; the longest locked plugin is stage 195/target 184/config 196 with a 530-character
file. The unchanged exact 254/config 266 root explicitly refuses. All13 actual
pinned-Neovim checkout cases pass (zero failed/errors), including original-target
preservation and stage/lock cleanup. The encompassing full Git/Make lifecycle
passes 125.21s. Raw SHA-256:
`f7ac5f14be70be964d6fd07c36e245cf6071cce944a93036d6d2924bc0d11f03`.
This is actual consumer evidence, unlike earlier log-only variant experiments.

The preceding quick probe's failure at8994 was diagnostic setup: DiscoverConfigFolders
ran after the Git/Make fixture set USERPROFILE to its private Unicode home.
Windows expanded native folders from that profile; the source did not assign
LOCALAPPDATA. Capture genuine native folders before environment substitution.
Its raw SHA-256 is `aa0e4178c2b441f40a8f3abb7fa3c5bb22d56b9dc75634421d5363aa4f817e0c`.
The independent public Neovim fixture sampled correctly before such overrides.
Do not reopen fixed config, extended-prefix, metadata-alias or staging-layout
hypotheses: none was needed, and native full-sequence proof now supports refusal
of overlong custom roots while preserving ordinary long-file support.

The independent 8994 full Neovim job 114334530582 (run 38093492507) reaches ready on
initial apply/check, remains ready after an installer-only rebuild, and verifies
both absent/existing notes vaults. Strict language use reports exactly one failure:
pwsh absent. Node reference diagnostics remain healthy and byte-identical. Raw
SHA-256 `ff7a7ef42518172286b827b90d711c00f2cde263d82fb84aa14a8d72a71e41ba`.
The prior fixture-root failure is gone; full update/shared/final removal remains
unproved because language use stopped the test.

Source trace: nvim.sync omitted tool.powershell from Windows platform_requires;
configureNvimSync assembles PATH only from that dependency closure. Add that one
edge to the already declared shared archive, before synchronization. A new
cross-target closure/order/real-constructor PATH regression reproduces the omission
and passes after; its temporary root follows the existing canonical native-path
fixture convention. No provider, package pin, runtime model or user capability is
added. Multi-location inspection also finds the smoke's PSES directory still at
legacy stdpath(data)/mason, whereas production lsp-config uses managed_runtime.path.
Correct the smoke lookup to that same helper and replace retired setup.ps1 -All
wording. Runtime and bundle absence remain hard Windows failures, with no new
skip, timeout or relaxed language assertion. Focused results are in the frozen
handoff; the actual complete public Windows Neovim rerun remains the closure gate.

The matching immutable quick-proof receipt is `windows-v7-quick-proof-receipt.json`
(SHA-256 `28a20864b7af5268339c6b4af79dae1bd4e360819526f927b28c52c02063a828`);
its Markdown companion is SHA-256
`af45de75690524274a0401062dcb3128551cd222454db57adebcbaa68b6bee08`.

### Native correction receipt — existing APT alternative consumers

The exact corrected production source at temporary diagnostic
`49def22d24fa9894ef39b1fce0833efec9a231d7` passed Ubuntu amd64 run
`38094424487`, job `114337270150`. Real Code pinned install/repair, externally
upgraded version observation, reviewed epoch downgrade and removal passed in
90.56s with repository/key/debconf preservation. Native font install/render/remove
passed in 6.90s. The complete public desktop lifecycle passed in 168.42s,
including Code singleton/full-group and Ghostty windows/settings, update/check,
final removal and personal baselines. Raw job log SHA-256:
`b8fbfd76b9afcd460d5645ddb509fb7287f837ca3950ca64777d252a5c6a6b9f`.
The frozen six-file implementation patch remains unchanged; this append closes
its pending amd64 native proof. Earlier ARM passes establish its observed scope,
not final integrated-head certification. No temporary diagnostic workflow/helper
is part of the delivery change, and no everyday workstation native mutation was
performed.

## 2026-10-11 — Preserve the existing extra-approval ruleset restriction

Read-only cutover preparation found both live main integrity and review rulesets
include `require_extra_approval_for_unattributed_changes: true`, while both
checked-in payloads omitted it. Existing helper normalization strips only
`required_reviewers`; its exact comparison therefore failed the current review
ruleset and would reject integrity even after the 23-check cutover. The isolated
fake-API preflight reproduces that failure against the observed live shape.

Explicitly record true in the two policy JSONs. Preserve all counts, bypasses,
checks, strictness, CodeQL and owner restrictions; change no helper algorithm or
live setting. Focused cases reject false/missing drift in each ruleset before
writes, confirm true survives apply and its saved restore payload, and retain
historical recovery against a committed policy without the field.

GitHub's official ruleset guide documents the additional approval setting for
unattributed Copilot PRs as enabled by default and ineffective at zero approvals.
The exact REST spelling is live-GET-confirmed but absent from official OpenAPI
commit `58b1e0c00b39b9e46c24dc4a0ba6a2669c90e9c1` (source SHA-256
`da3d22e417611cca9a4131dceea89d4d61c3be021b0917f32d9e4422e4fe60f1`).
Do not claim a published REST contract or unperformed write acceptance. Preserve
true in the proposed cutover payload, and require authorized readback rather
than dropping it on an API error. The live cutover remains a separate owner
approval after all 23 exact-final-head checks and review; no live writes or merge
occurred in this correction.

Validation: the full fake-API safeguard preflight/transaction suite, repository
policy and GitHub security policy tests, ruleset identity test, ShellCheck,
EditorConfig and diff checks pass. These are isolated policy-boundary tests,
not proof of live API write acceptance.


### 2026-10-11 — Rejected CoreText injection finding; static argument boundary

CodeQL check 114321518235 at de769314 identifies font_native.go:109 as
`go/unsafe-quoting` (security severity 9.3). The reported single-quote escape is a
false positive for this source, not a vulnerability fixed by escaping. Trace:
`hackFontFaces` derives 12 names solely from fixed SFNT constants; `json.Marshal`
emits a complete array expression; Go passes the script directly to osascript,
without a shell. That array is not inside any JavaScript string literal.

The [official query documentation](https://codeql.github.com/codeql-query-help/go/go-unsafe-quoting/)
concerns data inside quoted strings. Its
[model at b5b81654](https://github.com/github/codeql/blob/b5b81654b6e16786e6c5669f696d423d8764e336/go/ql/lib/semmle/go/security/StringBreakCustomizations.qll)
treats every JSON marshaling output as a source and requires only matching quote
characters somewhere in both adjacent concatenation leaves. Here the preceding
`'CoreText'` and subsequent empty path string satisfy that heuristic, despite
not enclosing the data. The model does not establish attacker-controlled input
or parse the JavaScript expression. This is the concrete rejected-finding
rationale; do not dismiss a future genuinely quoted interpolation by analogy.

Apple's installed osascript(1) contract passes arguments after the script to its
run handler; with `-e`, it does not treat that argument as a script filename. Use
a static JXA `run(argv)` and `JSON.parse(argv[0])`, with JSON as a distinct argv
element. This removes the source/data concatenation rather than suppressing
CodeQL or introducing quote escaping. The same 12 ordered names and all native
name/path/glyph/hash checks remain. No saved operation or registration command
changes; the mutation recipe in font_driver.go is deliberately unchanged.

Multi-location inspection covered every osascript invocation: the registration
recipe also inserts a bare JSON array; desktop window inspection inserts numeric
PID JSON; the rendering fixture uses a complete JSON string expression. None
encloses serialized JSON inside another quoted string. No adjacent rewrite or
new general JXA abstraction is needed for this read-only query boundary.

Evidence: the new argument-separation regression fails on the original source.
Read-only native proof executes the original query with its 12 fixed faces and
round-trips empty/single/multiple malicious-looking quotes, backslashes, newlines,
Unicode and U+2028/U+2029 as data. The new actual query accepts the same inputs;
normal fixed-face output before/after has SHA-256
`c3ae52dc52802df483aa8d138b2079387eec60115292327c88c4dc5570afa892`.
The new tests pass with race detection. No font/package/GUI/registration mutation
ran on the workstation. The original hosted failure remains failed until the
corrected source receives a new CodeQL result; native/local tests do not claim
that hosted check passed. Frozen source, source-model hashes, logs and focused
results accompany the isolated handoff.

## 2026-10-11 — Explicit architecture qualifiers in absent APT consumers

Accepted boundary correction: an unresolved `libgtk-4-1:amd64` declaration did not
match the catalog's absent bare `libgtk-4-1`. After an earlier approved operation
introduced that package, the unchanged declaration resolved to the installed ID
and appeared as a new outside consumer. The existing controller fixture reproduces
`libgtk-4-1 desktop runtime changed after approval` for the native qualifier on
both amd64 and arm64. Bare and foreign-qualified alternatives pass before the fix,
which rules out a general failure of incidental ownership or the controller.

[Debian's binary control-file specification](https://manpages.debian.org/trixie/dpkg-dev/deb-control.5.en.html)
defines an explicit real architecture qualifier as matching exactly that
architecture. Retain the architecture already supplied to `configureLinuxAPT`
and match both bare and native-qualified unresolved declarations while absent.
Continue matching the exact installed ID while present. Foreign-qualified
alternatives remain distinct; no new dependency solver, virtual-provider rule,
parser normalization, command recipe, state schema or executor relaxation is
needed. Reject provider construction/observation without a supported architecture.
The direct native fixtures now pass their actual runtime architecture explicitly.

Expanded the existing controller lifecycle fixture to both supported architectures
and all three declaration forms. Verify reviewed apply, original absence and
stable consumers in receipts, update, removal retention only for real native
consumers, personal browser/GTK3 preservation, and final collection after the
outside consumer disappears. Constructor tests cover observed-architecture
propagation and invalid metadata. The full focused APT suite passes, retaining
unattributed/changed consumer rejection, exact architecture/Provides parser tests
and historical pending-intent recovery. No package mutation ran on the workstation;
this correction still requires the integrated final-head native acceptance.

## 2026-10-11 — Opus v6 N1/N3: Code pin identity and completion before cleanup

Revalidated both accepted findings through the real controller with native
process/download boundaries replaced by deterministic fixtures. At the same pin,
Update planned a Code mutation after the prior operation removed its 230–250 MB
DEB. Separately, an unexpected child, changed DEB, redirected payload directory,
or unexpected debconf file made an already successful package install fail its
completion. These red outputs are captured before the correction.

For N1, publish the reviewed pin digest as the existing Desired observation only
for a healthy Code version matching that pin, through exact completed-operation
and native-root ownership proof. The unchanged planner keeps an already recorded
matching pin; its download transport may be offline and no native command runs.
Changed checkout pins, older/newer installed versions, failed health and absence
continue to plan their update/downgrade/repair/install. Pre-existing Code remains
unowned. Legacy schema-1 receipts omitting Desired remain readable and perform one
explicit reconciliation before they can use this recorded-input optimization.

For N3, durable native completion follows the same successful command, exact
ownership and native health checks, then precedes optional input cleanup. Cleanup
errors retain the exact path through existing Preserved disclosures. Read-only
observation reconstructs leftover paths from a completed intent after process
death; later updates/removal retain the disclosure, and a previously preserved
path never regains cleanup authority. Unexpected data and redirect targets remain
untouched. This adds no saved fields, finalizer framework, command permissions,
planner relaxation or native ownership exception. Unknown/failed worker outcomes
remain unfinished and retain inputs; pre-dispatch tampering still blocks mutation.

The existing successful-cleanup native assertions stay intact: direct successful
package mutations with unchanged inputs still remove their DEB and debconf inputs.
Controller regressions cover same-pin offline Update, all four changed-input
cases through install/update/remove, native outcome recovery, legacy Desired
absence, changed-pin reconciliation and a completed journal interrupted before
cleanup. Focused tests and Linux cross-compilation are scoped evidence; no native
package mutation ran on the workstation and hosted final-head proof remains open.


### 2026-10-11 — N2: read completed pre-prelude vendor commands without replay

Accepted Opus v6 N2: both vendor readIntent methods compared every saved command
to the current module-prefixed encoding. This was correct for pending dispatch
but also rejected already complete VC++ and schema-2 Build Tools journals before
their successful native evidence could be inspected. Normal check/preview/removal
then failed globally through observation. Native installer receipt readability is
independent of the authorized retirement of Nix/chezmoi runtime compatibility.

Factor only the existing UTF-16/base64 encoder. A complete intent may substitute
its arguments in the expected command only when they exactly equal the encoding
of that provider's unchanged trusted script without the historical module prefix.
Then retain the full command digest comparison, including operation, executable,
flags, input and environment. Observation still requires the exact worker result
and native success, matching registered state, and VC++ package log evidence.
Complete change paths return observation before prepare/Run, so this alternative
is never dispatchable. Pending old commands remain errors and no saved bytes are
rewritten. Build Tools inventory schema-1 refusal remains unchanged.

New regressions use the existing native process/network boundary fixtures plus
the real Controller. Both providers install, receive an exact historical complete
command and matching worker evidence, then observe/check/preview removal and
remove with a new module-prefixed command. Re-entering the old complete apply is
read-only and its original journal stays byte-identical after removal. Changed
script, flags, program, input, operation or environment are rejected even with a
matching forged fixture worker hash; missing matching completion evidence also
fails. Existing pending-refusal tests still assert no replay or rewrite. An
independent encoder assertion keeps new commands exactly module-prefixed.

The controller lifecycle regression reproduces both failures before the fix;
focused race tests pass afterward. These are deterministic provider/engine proof,
not claims of native vendor installation on the developer Mac. Source/patch/log
hashes and scoped compile/parser results are recorded in the frozen handoff.
No generic compatibility registry, migration mode, native package mutation or
live policy change was added.


### 2026-10-11 — Remove two accidental assumptions from Windows fixtures

Accepted the narrow v6 fidelity risks. The inherited-module fixture required the
raw upstream PS7 → Go → PS5.1 call to keep failing, even though its product contract
is the corrected native hash output. Classify that raw baseline separately: an
exact successful hash and the known Get-FileHash CommandNotFoundException are
valid diagnostics; empty/wrong hashes, error-looking success and unrelated errors
remain failures. The required PS7 launcher and all vendor, compiler, integration,
worker and real-bootstrap corrected assertions are unchanged. A seven-case
portable policy regression fails for already-correct upstream output before the
change and passes afterward. No optional corrected assertion or fake success was
introduced; the actual native chain remains part of the Windows suite.

The exact Windows Git stage refusal fixture used UTF-8 byte length for the native
Windows path limit. The pinned Neovim 0.12.5 `vim.str_utfindex` API explicitly counts
UTF-16 units, including surrogate pairs. With the actual old fixture padding
block, an ASCII root reaches 254/266 units, a BMP Unicode sample 247/259, and an
astral sample 252/264. After using the native unit count for root padding and final
assertions, all three reach 254/266; their UTF-8 byte lengths are 254/261/256. The
probe runs the actual source block through the archive/hash-verified pinned
Neovim binary, not a reimplementation of the padding formula. Syntax and Windows
compilation supplement that proof. The ordinary long-file case, actual Git
refusal, original-file preservation and stage/lock cleanup are byte-identical
outside the changed boundary calculation. No product path, runtime or package
policy changed, and no native mutation ran on the Mac. Final Windows execution
remains distinct from this deterministic/native-Neovim arithmetic proof.

## 2026-10-11 — Windows Neovim complete native acceptance

Accepted PowerShell prerequisite correction now has complete native proof at
`70fe3496f8e771ebcd1457a895b414eff32be0e6`: run `38094883281`, job `114338646414`,
`TestNativeNeovimInstalledApplicationLifecycle` PASS in 1162.47s. Archived job
metadata reports success on windows-2025. Raw log SHA-256:
`0dfaec306f00d2978f99fc66cae750c9f05a4682fb942986e32541809f6373f4`.
The JSON receipt SHA-256 is `5741504de79dcd8c9259b220b5faaabadd06f26a16f33a12dcd4a79cddfb62ab`.

The strict smoke reports 257 checks and confirms `sample.ps1 [powershell_es]`
attached. The original later assertions execute: update ready; add Pi ready;
remove Neovim and its unneeded shared prerequisites while Pi 1.0.4 still runs;
final empty selection ready; exact restoration of every snapshotted configuration,
profile and original editor-data child; unchanged personal note/data witnesses.
Earlier checks also prove absent/existing vault activation and an intact runtime
remaining ready after an installer-only executable rebuild. No integrity check,
consumer guard, timeout, full operation identity or physical path was weakened.

Re-read the six fixture/spec/PSES test/smoke files from the exact Git object and
matched every byte hash to the frozen handoffs; separately matched nvim.sync's
resource projection, including the Windows tool.powershell dependency. Unrelated
catalog edits are not represented as whole-file identity. The receipt records
all six hashes, exact raw log/metadata/publication attestation hashes and the
separate 43b7 proof of default-layout Git and all 13 Windows checkout cases.

This is a passing complete temporary-head native lifecycle, not a reinterpretation
of 8994's missing-pwsh failure and not final integrated-head certification. No
further production Git/configuration/layout workaround is needed. All 23 final
required checks and independent review remain required for the delivery PR.


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


## 2026-10-11 — Hosted Ubuntu zsh completion precondition

Quality run 38098526788/job 114349412458 at head
`044f9780705faa996914ed95de2aed2126802ba9` failed only the two active zsh
startup smoke assertions reached in Shell smoke: compinit could not open a
terminal for its insecure-completion prompt. Native Desktop passed on both Linux
architectures at that same head (114349412476 and 114349412553).

Diagnostic run 38098923912/job 114350585028 reproduced both failures and recorded
compaudit's exact paths: `/usr/share/zsh/vendor-completions` and `/usr/share/zsh`,
physical root:root directories at mode 0777. Their `/usr/share` parent was also
0777; newly installed zsh functions were 0755. This matches the image's
[reviewed configure-system script](https://github.com/actions/runner-images/blob/ubuntu26/20260927.149/images/ubuntu/scripts/build/configure-system.sh),
which recursively changes `/usr/share` permissions during image construction.
The source SHA-256 is `604976982f8515d86df578993b6c852bc636d53555fefeaa61fa984ecc7fef11`.
A permission change only to `/usr/share`, as used by Desktop, would leave the
completion audit failure unresolved.

Run 38099014868/job 114350857283 at
`a08cb858c2d48e94f8de13581dd14b426d70f719` then proved strict empty compaudit
and both unchanged tests after validating and setting exactly those three
physical root-owned directories to 0755. The original failing raw log SHA-256
is `82a0adb888f650c018aa580795b8862813f79cd32b8e8205309564e963c6f067`;
the passing raw log SHA-256 is
`ecbb149653ff5fb1b97ce8f01b83e3b7fd6f0fed952e8fbe525fc1c914f1a5a4`.

The correction factors the existing Desktop host preparation into one small
fixed helper, gated to GitHub-hosted Linux. Desktop retains its single
`/usr/share` scope; quality/core use the three-directory shell recipe and strict
compaudit. Core includes the real archive zsh PTY consumer. All paths must pass
physical-directory/root:root checks before any chmod, with no recursive changes
or production-runtime changes. The focused refusal fixture bind-mounts private
directories over `/usr/share` inside a private root mount namespace and proves
foreign ownership, final/ancestor symlinks, unsupported hosts and unknown recipes
are rejected without changing earlier directories. A positive desktop control
proves completion directories are outside that recipe's mutation scope.

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


### 2026-10-11 — Accepted Windows bootstrap singular-noun finding

Windows quality retry 114351013020 proves one real naming defect:
PSScriptAnalyzer 1.25.0 reports `PSUseSingularNouns` for the internal plural
module-binding helper at `scripts/installer-bootstrap.ps1:13`. Microsoft's
[rule contract](https://learn.microsoft.com/en-us/powershell/utility-modules/psscriptanalyzer/rules/usesingularnouns)
requires the singular noun. Rename the helper to `Import-BootstrapModule` and
update its call and both AST lookups. This is a naming-only correction: exact
current-engine imports, inherited-module protection and all process assertions
are unchanged. A checker suppression or change to module-path behavior is
unnecessary and rejected. The raw failed log is SHA-256
`a6113b231229bfd6ee5c6aeab8188a7e795f4688772fe2535e63223ba3cf76b1`.

The earlier Gallery no-match failure is separate. Diagnostic run 38098914061
on the same image found PSGallery already registered, received HTTP 200 and
installed/imported the unchanged required module on its first attempt. That
proof rejects guessing absent registration; it does not establish the original
transient cause or turn the failed job green. Diagnostic raw log SHA-256:
`833edf7be60338989dbde95154c69af8de90625266047e0d29e246e3855e96e5`.

The corrected three source files match the exact native diagnostic head
`6ebcdd0ceeb203f2ca633d6839a4edb003749917`. Run 38100201494/job 114354387651
reproduced the original warning with the actual Windows PSScriptAnalyzer 1.25.0,
then proved zero findings after the rename. The same job passed four native
PS5.1 extraction cases and the real PS7 → Go → PS5.1 inherited-module boundary,
including the unchanged vendor, compiler, integration, worker and bootstrap
hash assertions. Local focused race tests and Windows test compilation also
pass. Temporary diagnostic workflow/note are excluded from the delivery patch;
no full final-head quality rerun is claimed by this scoped proof.


### 2026-10-11 — Accepted shared-PowerShell shell-fixture omission

At `044f9780705faa996914ed95de2aed2126802ba9`, Windows core job 114349412548
failed `TestNativeArchiveSelectedShellFeaturesLifecycle` at its final profile
assertion. The diagnostic message said outside edits were lost, but its bytes
prove the exact personal suffix survived alongside the entire managed block.
`tool.powershell` is shared and requires `integration.shells`; the planner
correctly retains both when deselection omits `RemoveShared`. The public native
application fixture already makes the corresponding explicit per-item choice.
Raw core log SHA-256:
`651219736ea6c9e3515ffd5417b9e75cd0b8c1ef13cb112db0e83e3c1a1e6e67`.

Correct only the fixture: first assert owned PowerShell and exact managed/personal
bytes survive default deselection, then explicitly remove only its owned
PowerShell runtime before the unchanged exact cleanup and pre-existing VC++
assertions. Reject removing the catalog dependency, broad shared removal,
normalizing away a leftover block or changing ownership policy. The revised
native lifecycle is the regression; focused existing planner/profile tests and
Windows test compilation also pass. No provider or workstation mutation is part
of local verification.

Native run 38100700339/job 114355857812 at
`b974f8261a32554e7684bb3253bfce1c474807a0` passes the complete updated selected-shell
lifecycle in 52.26s. It executes original selected hooks before/after removing
zoxide, then logs exact default retention and explicit owned PowerShell removal
before all original final cleanup and VC++ preservation assertions pass. A source
comparison confirms every original initial and final assertion is byte-identical;
only the Windows shared-removal sequence was added. Native raw log SHA-256:
`2ccef4a8982dbd54c3d35a8e1494ff0f4cc2dd02d6166fb797d5cfc4438ef154`.
Temporary hosted workflow/note are excluded from the delivery change. No new
POSIX package lifecycle was run on the developer workstation.


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


### 2026-10-11 — Accepted Windows job-buffer lifetime defect

The original Windows Neovim final-removal failure at `044f978` (job
114349412580; raw SHA-256
`088439cbcd7f17d341cca89738031cee5ee6de17d7095bc064b0058456b04f00`) did
not distinguish changed lifetime membership from nonzero final Active. Bounded
private-job diagnostics at `383880fb692a1367d501c48fcf291e56230ce460`
(job 114354654925) then observed baseline Total/Active/Terminated `0/0/0`,
after-capture `45/1/0`, final `45/0/0`, one captured HANDLE and no final members.
An ordinary process case independently observed baseline zero then Total3.
The raw diagnostic log SHA-256 is
`182b37903096e9b08a1fca8a09bae60f338740a9114f3507d8fa25bdc43d2f63`.
These values do not prove 45 late children or accounting lag. [Windows defines
TotalProcesses as cumulative lifetime membership](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information).
A zero baseline after the root ran demands investigation of the query itself.

The pinned [x/sys v0.48.0 generated Query/Set wrappers](https://github.com/golang/sys/blob/v0.48.0/windows/zsyscall_windows.go)
accept uintptr without uintptrescapes. Converting a movable stack pointer before
entering those Go functions violates [Go's unsafe syscall-pointer contract](https://pkg.go.dev/unsafe#Pointer).
Their procedure lookup may grow the stack after conversion. In contrast,
[LazyProc.Call and Proc.Call are annotated](https://github.com/golang/sys/blob/v0.48.0/windows/dll_windows.go).
Keep query buffers as unsafe.Pointer through the small shared query helper and
convert only in direct LazyProc.Call; use that same boundary for the fixed
KILL_ON_JOB_CLOSE input and check BOOL before its last-error result. Go 1.27.1
escape analysis of the old accounting call leaves its buffer on the stack; the
correction reports `leaking param: data` and heap placement for accounting,
member and limit buffers. The retained escape receipt SHA-256 is
`23109bb9d041a0b8778a9a7813e41f863698514b6fdd856f972fcb8ba489e6dd`.

At `97d5596ef74cfd04d4360af37295157997bd944d`, run 38100961481/job
114356621691 proves the old Query defect against a real empty Windows job:
a successful call at depth0 leaves Total/Active/Terminated at `4294967295`.
After source restoration, all accounting/member/limit cases pass across 128
varied stack depths. All existing owned-process, outside-survival, cancellation,
error/output-bound, natural-exit and deliberate late-member cases pass. The
latter still refuses baselineTotal1/finalTotal3 after the exact captured HANDLE
signals. Six unchanged real compiler reuse/environment tests also pass.
Native raw log SHA-256:
`9eb5ac8fb77974830205023f4ec0756260e2599ecda46ee08fb369e06b11fe55`.

The overall diagnostic conclusion is failure solely because a separate old-Set
expected-red probe passed and its harness threw. That non-reproduction and the
old PID-buffer passing result are verification limits; neither is reported as a
native red. Both corresponding corrected kernel readbacks pass, and the unsafe
contract applies independently of incidental stack placement. The earlier
expected-red harness also retained LASTEXITCODE=1, and a diagnostic assertion
incorrectly assumed a second Go child implied Total2; both diagnostic-only
assumptions were corrected with the original failed logs preserved.

No lifetime-count guard, Active-zero requirement, exact HANDLE authority, atomic
assignment, output limit, five-second reap budget or mutation worker changes.
No accounting poll, stability solver, deadline increase, process-name exclusion,
global telemetry setting or blind apply retry is justified. The multi-location
check found other native struct crossings (profile copy, terminal input and
IsProcessInJob) already use direct annotated calls. Failure diagnostics query
only bounded counts/IDs from the private job and retain the original refusal
if the later evidence query fails. The original full lifecycle failure is not
turned green by this scoped proof; final published-head Windows lifecycle and
quality gates remain required.


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


### 2026-10-11 — Windows Desktop fixture exit race and stale witness

Final `127e35d` job 114360377438 passed native fonts (22.29s), Terminal settings
(31.43s) and the Code singleton (9.20s); its second Code invocation failed in
`desktopStopOwned` after 4.57s with `TerminateProcess: Access is denied.` This
is deferred fixture cleanup, not a launch/window assertion. Raw log SHA-256:
`19ff2311be0276b5a10586f87013d88e7e13ad6de1e446c0a76e95ef80841c41`.
The log alone cannot identify whether the exact process had already exited.
[Microsoft documents post-exit ERROR_ACCESS_DENIED](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-terminateprocess),
while [Go 1.27.1 Kill returns the native error](https://github.com/golang/go/blob/go1.27.1/src/os/exec_windows.go)
without converting it to os.ErrProcessDone. Sequentially stopping an application's
parent and children must handle that documented boundary without ignoring a live
permission failure.

The Windows-only test adapter opens an exact HANDLE, validates its executable
against the physical private package root, and then terminates/waits within the
existing cleanup deadline. This closes the CIM PID-to-handle reuse gap: a candidate
PID cannot authorize terminating an outside executable. ACCESS_DENIED is accepted
only if a zero-time wait proves that exact held object has already terminated;
live denial and unproved completion remain fatal. The existing final empty-root
inventory and all public lifecycle assertions remain required. There is no new
process framework, retry budget, global process exclusion or product change.
POSIX cleanup keeps its existing behavior.

The same fixture also reused `Code settings witness.json` under the same home
for singleton and complete-group launches, while per-launch cleanup removed only
the extension. A real filesystem regression retains the first witness, recreates
the extension and proves the second invocation must use a distinct absent file
without changing the earlier bytes. It fails on the old same-path recipe and
passes with one t.TempDir per invocation. Both native launches now log successful
fresh settings evidence only after the extension host writes that invocation's
file and all existing theme/font assertions pass. The earlier short second
invocation is not proof that stale evidence caused the observed process race.

The real Windows kernel regression will exercise the original os.Process.Kill
post-exit denial, corrected exact-HANDLE acceptance, a live handle lacking
termination rights, cancellation and outside-image survival. Compilation/vet
and local race coverage pass; native results are pending. Initial local test
setup failed because the shared private Go source tree was incomplete; it is not
counted as the regression red. A separate toolchain was extracted from the
previously verified Go 1.27.1 archive without changing the workstation.


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

### 2026-10-11 — Completed published checkpoint 127e35d matrix

Published `127e35d676c69a65068b55e15c0c3508c6a7f3dc` completes
[installer run 38102238964](https://github.com/luisgui1757/dotfiles/actions/runs/38102238964)
and [quality run 38102238808](https://github.com/luisgui1757/dotfiles/actions/runs/38102238808)
with 21/23 successful native/quality jobs. All eleven Linux jobs, all six
Apple Silicon macOS jobs, Windows core/quality and both fresh Windows toolchain
jobs pass. CodeQL's actions, Go and Python analyses and its app57789 aggregate
pass on that exact head. Check identity is app15368 for the 23 canonical jobs;
the verifier binds each check URL to the actual workflow job. Its incomplete
result is not an accepted final-head receipt.

Windows core proves the corrected default shared-PowerShell retention and
explicit owned removal, exact personal suffix/BOM restoration and retained
preexisting VC++ runtime. It also passes full Git/Make use/removal, private
language prerequisites, native Build Tools reuse and public PowerShell 5.1
setup/migration. Windows quality passes the full ordinary race suite, 48 Python
tests, 49 Pester tests with no skips/failures and native PSScriptAnalyzer 1.25.
macOS core passes archive/interactive shell/Pi use, native Homebrew outside-
consumer update and repair/recovery, private language prerequisites and locked
Neovim bootstrap. Complete macOS Neovim, Desktop/fonts, fresh Homebrew and
missing-payload Apple tools pass in their separate native jobs.

The two failed contexts remain failed: Windows Desktop cleanup, corrected and
native-proved in the preceding entry, and Windows Neovim's final `tool.git`
inspection before language smoke. The latter is not an approval-plan mismatch
or proof of bytecode drift: the saved/actual Node archive is intact, while the
original log does not identify the Git entry or changed predicate. A separate
failure-only native diagnostic keeps the exact refusal and original lifecycle
budgets; no cache, antivirus, sharing or concurrent-mutation cause is inferred.
Complete amended-head verification remains required after consolidation. No
new PR, blind rerun, live policy write or native workstation mutation occurred.

### 2026-10-11 — Git tree-inspection refusal: bounded metadata, no inferred fix

**Original evidence:** canonical127e35d job114360377352 failed after626.52s at
initial selected Neovim apply with `tool.git: configuration changed during
inspection; review a fresh plan`. That exact error originates in final per-entry
Lstat/identity/mode/size/mtime inspection, not the planner's approved-fingerprint
comparison. The log does not identify an entry, changed metadata field or native
identity error. Node's separate failure diagnostic proved only its own payload
unchanged. A legitimate outside mutation and a failed identity query cannot be
distinguished from the original log; neither cache/antivirus nor a runtime bug is
established.

**Actual reproduction attempt:** temporary head
`182c04b58f6c90b453b9ed9273d008b5c2c4215d`, run38103528922,
job114364170320 passed the original complete
`TestNativeNeovimInstalledApplicationLifecycle` in1256.98s. Initial apply and
public check after installer-only rebuild were ready; absent/existing notes
vaults and all257 smoke checks passed; update and combined Neovim/Pi apply were
ready; selective Pi1.0.4 survival, final removal, exact profile/configuration
snapshots, personal notes and editor-data restoration passed. The preliminary
bounded metadata test and real Windows read-only identity-probe test passed. The
existing POSIX symlink test retained its explicit Windows skip; no lifecycle
assertion was weakened or skipped. No failure-only Git diagnostic fired.

**Minimal delivery:** retain only a bounded quoted root-relative entry, kind,
before/after mode/size/mtime, actual SameFile bool and the original joined Lstat
error. Compute the same original predicates once; no query, retry, changed
fingerprint, changed approval or successful fallback is introduced. Regression
coverage verifies truncation, quote/Unicode/newline safety, nil metadata, identity
result, original error cause and continued refusal of edited personal snapshots.
The subsequent Windows probe and native Git provenance logger stay solely in the
preserved diagnostic commit: another open cannot expose the original native
query error and would add unnecessary Go-internal flag maintenance to delivery.
The temporary workflow/note are also excluded. The original127 failure remains
failed with unknown cause;182 is non-reproduction, not a declared runtime fix.

Original127 raw log SHA256:
`915bbed90eab6433d3474fce4ae0958b29b22507bc85c15098d60a2bc1b3a153`.
Diagnostic182 raw log SHA256:
`2423b533c1dc9a2c401d776401b422fb714a395df56c54591a9024ac830874ee`.
Exact182 job metadata SHA256:
`94ea83c47583486af00d0d028b951783806b497e09fd267f11845c6878d60652`.
No daily-Mac native packages, fonts, registration, GUI or live policy were changed.

### 2026-10-11 — Local private tool integrity and cohesive gate retry

The first integrated Desktop gate remains failed with unchanged source hashes.
Four launcher subcases and six ordinary Go tests failed because the task's
private macOS PowerShell Management/Utility module directories were empty:
missing built-in commands/manifests are explicit in the raw failures. The cause
of the file absence is unproved. Do not attribute this to the product, change
its module-loading behavior, suppress errors or skip PowerShell tests.

Restore a separate private PowerShell 7.6.3 tool from its official pinned asset:
SHA-256 `f0263c2072fe7d0953781c60497a574bea99b37237f2554a59ce4bad07de8d36`.
GitHub asset metadata and the independently hash-verified official checksum file
agree; the checksum file is BOM-marked UTF-16 and is decoded accordingly.
All 677 regular-file bytes and modes match the archive; Management/Utility
cmdlets load, and the four unchanged public-launcher subcases pass. A separate
private persistent Go 1.27.1 SDK also matches all 15,639 archive files and modes.
Original incomplete tool directories are untouched for evidence. These are
CLI-only test tools, with no system installation or personal setting change.

The minimal Git metadata patch is then integrated, preserving the existing
Desktop changes and every historical Markdown prefix. Its two Go files match
the frozen focused-test source. Temporary Windows probes, fixture provenance
loggers and diagnostic workflows are excluded. This source change and the
restored test environment justify a new complete seven-check gate; preserve the
earlier failed receipt under its original name. Final-head native/quality checks
and CodeQL remain required after the one-commit amendment.

### 2026-10-11 — Opus v10 disposition and completed consolidated proof

All seven consolidated gate receipts pass against the same 753 unchanged source
hashes: `make -k ci` (installer race 503.702s, terminal race 14.010s, 48 bootstrap
tests and complete static/configuration/editor/shell/policy/official Renovate
checks), plus all-package build/test compilation on Windows amd64 and Linux
amd64/arm64. The failed initial private-tool gate remains failed and preserved.

Actual `claude-opus-5-5`, xhigh requested/effective effort unexposed, performs
62 Read/Glob/Grep calls with no API, permission or guard errors. All 803 supplied
hashes remain unchanged. Snapshot SHA-256:
`0e6a6489edb126f5faed0f6c7f4d1531743ab9c9938e6efd6fee85172e2b1263`;
execution-summary SHA-256:
`396b89b41b2327c4bdf53fd7b3e3148d72be6429fa6b05c5f24fc5ff5249cadc`;
review SHA-256:
`47ea14ba6fcba69dca30f95eb0ee3cf900c82b963e2e10ac20bd89fa8b4e01c1`.

**Disposition: no material defects.** Exact-HANDLE/image/completion guards,
passive rundown waits, outside survival, fresh per-launch Code evidence and
POSIX handle release are minimal and correct. The Git patch changes only
failure details, preserving every inspection predicate and original error cause.
The unexplained127 refusal alone is not a delivery blocker; final-head Windows
Neovim remains mandatory and recurrence blocks readiness. No runtime cause or
fix is inferred. The snapshot contained a partial running local gate; actual
completed seven-gate receipts now close that gap with identical shipping bytes.
Root also independently verifies both original failed logs against their saved
SHA-256 and actual failure markers; they were not included in the closed review
snapshot, so no independent reviewer claim is made for reading those originals.

**Review explanation corrected, no code defect:** the review said inspection
errors never enter plan identity. At `ArchiveDriver.Observe`, this error instead
becomes an Unknown/Pending observation with nil returned error. The planner
records that observation/reason and digests the plan. Metadata here comes from
the same observed file information, not a read-time clock; changing observed
metadata appropriately changes a blocked preview. Unknown/Pending cannot grant
mutation authority. Preserve this distinction rather than copying the incorrect
review explanation into the architecture guide.

**Optional refinements rejected:** rendering skipped SameFile as unavailable is
cosmetic because unavailable after-metadata and the joined stat cause already
explain it. Reaching image-query denial through the public adapter would require
a real process in rundown; native live-denial helper proof and its one-call
adapter source suffice. The edited-snapshot test does not simulate a mid-walk
change; adding an internal hook would add unnecessary complexity to unchanged
inspection policy. No change was recommended for these nonblocking notes.

Every canonical native/quality context and CodeQL must pass on the eventual
amended head. This includes native reduced-formatter tests, Windows quality's
race kernel cases, full Windows Neovim and all OS Desktop/font/Terminal proofs.
Successful GUI logs do not identify which teardown path executed; controlled
kernel tests prove those paths separately. Actual recovery after the original
Git failure was not exercised because it did not recur; existing transaction
and inspection recovery gates remain required. Runtime/test bytes are frozen;
only this proof Markdown follows, with a focused static gate before publication.

### 2026-10-11 — Accepted Linux X tree disappearance boundary

- Evidence: exact final `406218a` ARM Desktop job `114371815911`, raw SHA-256
  `d6aa5553182f5dadf5bb7ffe9c10331338c0b5b7c0a1f381dd8b44f12dd40e4d`,
  lines 423–439. Code singleton settings passed before the second launch's X
  tree query failed on a disappearing child. Code package ownership/preservation
  and font lifecycle had passed; full ARM GUI acceptance did not.
- Source truth: upstream [xwininfo tree traversal](https://gitlab.freedesktop.org/xorg/app/xwininfo/-/blob/master/xwininfo.c)
  queues geometry/tree requests after enumerating children;
  [its error printer](https://gitlab.freedesktop.org/xorg/app/xwininfo/-/blob/master/dsimple.c)
  emits native error/request/resource numbers. A disposable Ubuntu 26 ARM Xvfb
  reproduction captured the same BadWindow/QueryTree failure. The hosted paired
  geometry/tree form is covered by the process-boundary regression. Xlib GetProperty returns
  BadWindow for a nonexistent window; BadDrawable alone is not the fresh proof.
- Resolution: failed tree output never proves a window. Exact typed errors plus
  a fresh root and matching Window-specific absent-child query may return
  absence to the existing poll. Unrecognized/root/display/permission/context
  errors remain fatal, including the previously unchecked `xprop` failures.
  Native PID ownership and `IsViewable` are both required for positive proof.
- Verification: focused red/green process-boundary cases and actual Xvfb owned,
  outside, unmapped, destroyed and unavailable-display cases. No new runtime
  dependency, time budget, provider behavior or sandbox policy. Corrected full
  hosted amd64/arm64 desktop lifecycle proof passed at diagnostic source
  `5788d8b4b062fc7941541f23143a812ee3e50661`, run `38107178096`.
  ARM job `114375040519` completed the full configured lifecycle in 167.76s;
  AMD64 job `114375040614` completed it in 127.45s. Both also passed the real
  disappearance/display boundary, native Code install/repair/reviewed downgrade/
  removal preservation, fonts, both fresh Code settings witnesses and final
  personal baseline restoration. Raw log SHA-256s, ARM then AMD64:
  `51da7b917f3362a100c84245130a74ee9acb544c79a40822625b294cb458a010`;
  `82803ab963e82780af0ddfac2bc7b0602283e94aa772b7017ebfa429e994b24f`.
  Temporary workflow/note are excluded from delivery. This scoped corrective
  proof does not rewrite the failed `406218a` result or replace final-head gates.

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


### 2026-10-11 — Independent full review and accepted correction batch

The complete `45b8b38` candidate passed all 23 canonical native/quality checks
and CodeQL. Owner-authorized required-check changes applied to both live layers
and independently read back as 23 checks, strict mode, Actions app `15368`.
AeroSpace's actual tiling/configuration/focus smoke passed after owner-granted
Accessibility consent. These supersede the prior pending statements for those
specific gates; they do not certify subsequent source changes.

Native GPT-6 Astra and actual `claude-opus-5-5` reviewed the complete candidate.
The Opus CLI explicitly accepted `--effort xhigh` and
`CLAUDE_CODE_EFFORT_LEVEL=xhigh`; 150 Read/Glob/Grep calls completed with no
model substitution, API error, guard violation or permission denial. Effective
effort was absent from emitted metadata. The previous claim that the CLI cannot
accept an effort level was incorrect; absence of metadata is a separate fact.

Accepted corrections (implementation verification in progress):

- **R1/P2:** Mason used a live registry and bare tool names. The bundled exact
  upstream compiled registry, release and two SHA-256 pins now bind recipe/tool
  selection to the checkout. The existing Mason fixed GitHub registry format
  consumes those bytes; selected concrete versions derive from one source.
  Source/version and registry provenance are checked before publication and in
  Check. Native install/update/removal reruns remain required. The locked Mason
  compiler uses the source ID's version without an override; adding a duplicate
  explicit version list would activate unnecessary upstream version validation.
- **R2/P2:** ordinary managed Neovim startup could replace executable dependencies
  inside its published generation. Explicit sync staging retains provisioning;
  published startup verifies and directs failures to Update/Repair. Keep normal
  unmanaged behavior and executable-checkout validation. The real UI regression
  must prove changed Lazy pins/missing plugins/parsers cannot alter the payload.
- **R3/P2:** macOS path_helper moved inherited managed directories behind system
  commands. Real nested login reproduced outer managed Git versus nested
  `/usr/bin/git`. The generated profile loader now restores selected-directory
  priority idempotently. The regression fails before and passes after, including
  actual macOS system/user login profiles and POSIX shell processes.
- **R4/coverage and umask:** restore caller umask before exec while keeping private
  bootstrap preparation. Add real verified POSIX setup/migrate download/build and
  isolated configuration lifecycle to existing hosted core jobs. Fake-compiler
  boundary regressions do not replace that pending native proof.
- **R5/P3:** original-engine identity is an executable SHA-256, not a Git source
  revision or bootstrap cache filename. Keep the safe resume refusal and explain
  how to hash retained binaries and recover the matching checkout. Finish any
  interrupted operation before updating checkout sources.

Rejected/qualified findings remain explicit: Mason's normal silent startup
installation does not execute under the locked VeryLazy/VimEnter event ordering;
no fix is based on that claim. Check already detects desired/runtime differences.
Historical migration/known-issues/manual documents are marked historical and are
not rewritten as current bugs. The required-check metadata stage is a validator
format/provenance field; the live transition is complete and its consumers remain
unchanged. Registry and direct tool-version pinning does not promise byte-identical
npm/PyPI transitive resolution. No merge, release, policy weakening or extra PR.


### 2026-10-11 — Correction implementation and local acceptance

R1–R5 are implemented. The final complete `make ci` gate passes, including 463
Neovim assertions, 49 bootstrap tests, all configuration/shell/static/policy checks,
official Renovate validation and 67-record extraction, and Go format/vet/race.
The installer race suite passed in 531.238s and terminal race suite in 14.010s;
the final complete gate reused their valid Go results. All six all-package builds
and test compilations pass for Linux amd64/arm64 and Windows amd64. Cross-compiling
is not native execution. The earlier generated `__pycache__` static failure remains
failed evidence; removing those task-created files restored the unchanged checker.
An earlier successful combined log was truncated by a child process; a complete
pipe-captured rerun passed with the full output retained.

Failed-before/passed-after evidence covers immutable Mason registry configuration,
wrong installed tool versions/registry provenance, real attached-UI preservation
for changed Lazy pins and missing plugins/parsers, nested macOS login PATH,
caller umask through public cold/warm launchers, and original-engine recovery
refusal/guidance. Native public POSIX bootstrap now exercises real toolchain
download/hash/build, setup/migrate, update/check/removal and personal preservation
in the existing hosted core jobs. Its final native execution remains mandatory.

Native GPT-6 Astra found no material correction findings against the immutable
765-file export and patch SHA-256
`2dadd6bc7f55bf2c8f309ee30fef39ef61f1568663b92117fcbad72830aaabce`.
The only subsequent production-source delta changes two full-line Tree-sitter
comment paragraphs; executable Lua remains identical. Review scope and limits:
receipt fixtures do not replace real package installation; reduced real-UI fixtures
do not prove every plugin; configuration-only public-bootstrap coverage complements
existing provider/terminal acceptance. Recovery source guards remain unchanged.
The correction review does not re-audit unrelated providers or every upstream
registry recipe. Actual Opus correction review and all amended-head native checks
gate readiness; their final receipts and current status are recorded in PR #87.
No merge or release has occurred.


### 2026-10-11 — Final-head Windows fixture readiness correction

The first `a8260cd` native attempt exposed two separate failures. Fresh Build-Tools
failed before fixture compilation because the local Docker daemon pipe did not
exist; its guard correctly refused to proceed. The same unchanged workflow on the
same Windows image passed for the current VC-runtime job and the previous full
Build-Tools job. This establishes a runner prerequisite failure, not its service
root cause. Preserve the failed attempt and require actual fresh-container success.

Windows Desktop reached the configured managed PowerShell witness but its single
MainWindowHandle query returned valid zero. It failed before the existing readiness
allowance expired. The source/pins/settings were unchanged from the prior passing
head. Shell readiness does not establish visible-window readiness; Microsoft
[documents this startup distinction](https://learn.microsoft.com/en-us/dotnet/api/system.diagnostics.process.mainwindowhandle?view=net-10.0).
Independent Astra diagnosis confirmed the unsynchronized assertion. The original
log does not prove a window subsequently appeared.

The fixture now polls only valid zero for the same owned PID, under its original
30-second startup budget. Native queries inherit that deadline and retain their
15-second cap. Query/parse errors, deadline expiry and process exit remain fatal;
nonzero native window, exact settings save, shell release and clean process exit
remain required. Portable process-boundary regressions cover zero-to-window,
persistent zero, malformed output, query failure, process exit and late success.
No production installer behavior, provider, timeout budget or success assertion
was weakened. Full native proof must still pass; the query count records whether
a native run actually observed the zero-to-window transition.


### 2026-10-11 — Actual Opus correction review disposition

Actual `claude-opus-5-5` completed 112 Read/Glob/Grep calls with `--effort xhigh`
and no model substitution, API error, guard violation or permission denial.
Effective effort remains absent from provider metadata. Review SHA-256:
`2c611c0647520c92e791233fb1b53d019b83c860c0d271423a648058b5a269b6`.
It found no confirmed R1–R5 bugs and verified each accepted correction's code path.
The exported Tree-sitter comment delta was independently checked by the writer;
the reviewer appropriately did not claim to inspect unavailable final bytes.

Two design choices remain deliberate, now documented explicitly. Published startup
fails closed when checkout pins no longer approve the installed Lazy/registry;
Update/Repair must succeed before normal configured editor use resumes. Loading
against a second historical approval source would add architecture and split the
checkout's authority. Selected managed command directories take priority over
personal as well as system PATH entries, matching the released shell behavior;
users retain the documented later local hooks for intentional overrides. The
shell regression now includes a personal directory and verifies its preservation.

The real UI suite now separately exercises missing parser/highlight queries
without a bundled override, while retaining the override-preservation case. Both
assert their exact diagnostic and unchanged payload. Four UI cases pass. Real
Mason receipts and clean Check remain native lifecycle responsibilities rather
than claims made from generated receipt fixtures alone. The public-bootstrap log
now distinguishes requested umask from the separate launcher test's direct proof;
the PowerShell recovery example restores its temporary environment variables.
The legacy-journal diagnostic explicitly says its original digest is unavailable;
no digest is guessed and its safe recovery refusal is unchanged.

The review also exposed inherited GOMODCACHE escaping the launchers' declared
private compiler-cache boundary. Both compiler subprocess environments now bind
it to their private GOPATH module directory, preserving caller/application values.
This is a narrow environment correction with failed-before/passed-after launcher
boundary coverage, not a new cache mechanism or provider. Subsequent focused
Astra review found no material issues in the Windows Terminal readiness helper
and its nine boundary cases; complete native execution still gates readiness.


### 2026-10-11 — Native correction proof and disposable module-cache teardown

All four `a8260cd` Neovim lifecycles passed: macOS arm64 (899.16s), native
Windows amd64 (905.28s), Linux arm64 (435.79s) and Linux amd64 (439.05s).
Each passed all 257 strict language checks, absent/existing vault use, Update and
Check, shared Pi survival after selective removal, final empty removal and exact
personal/profile restoration. Windows also verified readiness after an installer
binary rebuild. These certify the reviewed runtime corrections at that source;
the later separate UI test adds coverage and does not change runtime behavior.

All three POSIX public setup/migrate lifecycles (macOS arm64 and Linux amd64/arm64)
completed every runtime assertion,
including cold verified bootstrap, exact executable identity, adoption, update,
check, removal and personal preservation. The jobs nevertheless failed when
`t.TempDir` teardown encountered Go's read-only module-cache directories. The
fixture now uses the executing host Go's `clean -modcache` with GOMODCACHE bound
only to that disposable private cache, before ordinary temporary-tree cleanup.
A focused regression reproduces ordinary removal refusal and verifies neighboring
personal cache preservation. Production cache permissions and module integrity
remain unchanged. The original job failures are not relabeled as passes; corrected
full jobs and all final-head checks remain required.

### 2026-10-11 — Final focused review and Windows parser fixture correction

Actual `claude-opus-5-5 --effort xhigh` completed the final bootstrap/fixture review
with 45 Read/Glob/Grep calls, no substituted model, errors or permission denials.
Effective effort is not exposed in its metadata. It found no confirmed production
defects and three low-severity diagnostic/documentation issues, now corrected:
the cache-teardown failure record includes macOS as well as both Linux targets;
Terminal deadline failures preserve both context and process errors and report
the query count; PATH override guidance qualifies the managed-zsh-only local hook.
A failed-before/passed-after process-boundary regression covers a query killed at
the readiness deadline. The original timeout and every refusal remain intact.
The cancelled `a8260cd` Windows public bootstrap job is not a passing lifecycle.

The Windows quality job exposed a separate fixture precondition: pinned Neovim
0.12.5 bundles `lib/nvim/parser/lua.dll`, while POSIX bundles `lua.so`. Upstream
loads `parser/<lang>.*`; the fixture now uses that same source lookup and copies
the unchanged binary bytes into its original managed `site/parser/lua.so` target.
Both exact diagnostics and whole-payload preservation assertions remain required.
No runtime implementation or workflow changed. Diagnostic commit
`d67f17292da84c08930f90116a13902804df8031`,
[Windows job 114411941912](https://github.com/luisgui1757/dotfiles/actions/runs/38119780237/job/114411941912),
passed all four real UI cases against checksum-verified Neovim 0.12.5. The temporary
diagnostic workflow is excluded from the delivery PR; final canonical quality
acceptance still gates readiness.

Frozen source `4380ae9` passed complete local `make ci` in 654.471 seconds with
all 766 source files unchanged, and all six foreign build/test-compilation checks.
Its Windows Desktop job passed the full Terminal/Code/font lifecycle, with the
window ready on query one. That is native acceptance, not a reproduced zero-to-
window transition or proof of the historical failure's root cause. Failed attempts
remain failed evidence; no test was bypassed, weakened or relabeled.

### 2026-10-11 — Explicit Docker prerequisite on disposable Windows vendor hosts

The `f02de9f` VC-runtime job failed before fixture compilation because the Docker
named pipe was absent, as the earlier `a8260cd` Build Tools job had. Both vendor
jobs passed at `4380ae9`, and `f02de9f` Build Tools passed. These results establish
intermittent host availability, not whether the original service was stopped,
starting or crashed. The vendor jobs use Windows Server 2022, not the Windows 2025
host used by ordinary core, desktop and editor acceptance.

The exact image's [Docker provisioning source](https://github.com/actions/runner-images/blob/win22/20261004.326/images/windows/scripts/build/Install-Docker.ps1)
uses Microsoft's registered `docker` service; its
[pinned installer](https://github.com/microsoft/Windows-Containers/blob/8ea2a92d8dfa7815abcf5cc44a756a0ac3d0f513/helpful_tools/Install-DockerCE/install-docker-ce.ps1#L650-L724)
starts that service and waits for the API. The CI preparation now establishes that
same prerequisite: record service state, start an existing stopped service once,
then require successful Windows API readiness. Individual queries have a five-
second bound within a 60-second API budget; the two-minute step also bounds service
startup. A missing/stopped service, query timeout or responding non-Windows engine
is fatal. Existing compilation, exact pinned image and vendor lifecycle guards
are unchanged and single-attempt. No service installation, policy change or repeated
restart is introduced.

Native diagnostic commit `52c687e3c8c92449cb45a6210ee3e11e6dd2c5f3`,
[Windows Server 2022 job 114416681147](https://github.com/luisgui1757/dotfiles/actions/runs/38121393361/job/114416681147),
proved the stopped-service guard failure, exact preparation recovery (552 ms),
and already-running reuse (74 ms) with the same service process. The consumed
script SHA-256 was
`f253ecdaf1798ab61174424cda4b3d9e32afef5057f9507fa204b0fae3ab21a1`.
Actual PSScriptAnalyzer reported no warning/error diagnostics; Astra found no
material implementation findings. This proves the prerequisite handling, not the
cause of either historical runner failure. Final canonical vendor jobs still gate
delivery; the temporary diagnostic workflow is excluded from the delivery patch.


### 2026-10-11 — Windows bootstrap fixture fidelity

Archived Windows quality job `114410980175` at `4380ae9` exposed three test
defects. The new Bash path marker was decoded with Python's host-default text
encoding; it now uses UTF-8 like the adjacent application-result marker.
PSScriptAnalyzer correctly reported that Toolchain/Stage/Go were only assigned
in the AST-driven fixture; the child now records its actual GOROOT, GOTMPDIR
and executable and the parent asserts each exact input. No warning is suppressed
and the analyzer baseline is unchanged.

With PSNativeCommandUseErrorActionPreference=true, PowerShell itself raises
NativeCommandExitException for child exit 17 before the wrapper's custom check.
That original exception is a valid failure, and the existing finally restores
the caller environment. Four real-child cases now cover success and exit 17
under both native-error preference values, requiring either the exact custom
message or the exact exception type and ExitCode=17. Production scripts are
unchanged. The original failed log SHA256 is
`42d986fbd9c7528167ab911cf140333c8ab2479be6b51b8047b8f6004d7c414f`.

The native proof is source-bound across two temporary diagnostics with identical
Python/Pester test bytes. [Job 114416524318 at `1a199c5`](https://github.com/luisgui1757/dotfiles/actions/runs/38121340400/job/114416524318)
passed PSScriptAnalyzer 1.25.0 with zero findings, the corrected Unicode case,
and all 53 Pester tests (32.86 seconds). Its complete Python suite failed three
release-inventory cases because the temporary workflow omitted canonical
`fetch-depth: 0`; this failed job is retained. The only follow-up changed that
temporary checkout setting. [Job 114417122665 at `761ca1b`](https://github.com/luisgui1757/dotfiles/actions/runs/38121541198/job/114417122665)
passed all 50 Python tests (42.084 seconds) and all 53 Pester tests (37.67 seconds),
with zero skips. It failed overall when an unrelated read-only Docker observation
returned exit 1 (Stopped/Auto service, service ExitCode 1066, missing daemon pipe);
its analyzer step was skipped. Analyzer proof belongs to the first job, not the
second. Neither diagnostic is relabeled as an overall pass, and its temporary
workflow is excluded from delivery. Final canonical quality remains required.

The error-mode distinction follows Microsoft's [native-command preference
contract](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_preference_variables?view=powershell-7.6#psnativecommanduseerroractionpreference).
Python's [Path.read_text encoding contract](https://docs.python.org/3.13/library/pathlib.html#pathlib.Path.read_text)
likewise requires explicitly selecting UTF-8 for these Bash-written bytes. This
is a fixture correction, not evidence of changed paths or a compiler isolation
defect. Original failed log and both diagnostic logs remain archived with SHA256
identities in the source-bound handoff receipt.


### 2026-10-11 — Cold Docker readiness probes

Both `d83ab95` fresh Windows vendor jobs failed the new readiness step before
fixture compilation: [VC runtime](https://github.com/luisgui1757/dotfiles/actions/runs/38122626662/job/114420325937)
and [Build Tools](https://github.com/luisgui1757/dotfiles/actions/runs/38122626662/job/114420325953).
Their services remained Running, with unchanged process IDs and zero service exit
codes, while the first `docker info` process exceeded its five-second cap. The
Build Tools cleanup also reported an orphaned docker-compose child. These remain
failed jobs, not vendor lifecycle failures or accepted readiness.

Docker CLI v29.7.2's [info implementation](https://github.com/docker/cli/blob/v29.7.2/cli/command/system/info.go#L78-L105)
unconditionally enumerates client plugins before its daemon query even with a
server-only format; [plugin metadata](https://github.com/docker/cli/blob/v29.7.2/cli-plugins/manager/candidate.go#L13-L19)
executes subprocesses without a context deadline. In contrast,
[version](https://github.com/docker/cli/blob/v29.7.2/cli/command/system/version.go#L198-L214)
queries the actual server Version API and propagates API errors. Readiness now
uses `version --format '{{.Server.Os}}'`, requiring exit zero and exact `windows`.
Its process wait consumes the existing remaining 60-second budget rather than an
arbitrary five-second sub-deadline; late success remains rejected and the enclosing
two-minute step still bounds launch/service/cleanup overhead. No deadline for the
vendor lifecycle, image identity or acceptance predicate changes.

The earlier `52c687e` stopped-service diagnostic ran `docker info` before the
exact preparation script. It proved service startup and reuse with a warmed
client, not cold-client readiness. Client-plugin work is a supported explanation
for the historical timeout, but those logs do not isolate its elapsed time and
we do not claim a proven historical root cause.

Corrected diagnostic `65459064058f300e5763aa4ad08640fb5515de09` ran the exact
preparation as the first Docker CLI invocation on two separate Windows Server
2022 hosts. [As-found service](https://github.com/luisgui1757/dotfiles/actions/runs/38122799096/job/114420825465)
passed in 2451 ms and [explicitly stopped service](https://github.com/luisgui1757/dotfiles/actions/runs/38122799096/job/114420825444)
passed in 2962 ms. Subsequent checks took 47/78 ms and preserved the same service
PID. The original full info guards also passed, taking 4410/4484 ms after those
server-only probes. Both complete diagnostic jobs passed. Exact script SHA256:
`a138b01829e0b197655194e2aef2c0e7caba2fb2695b923f9454f2e19cfd1110`.
PSScriptAnalyzer reported zero warning/error findings and Astra found no material
finding in the two-line correction. Temporary diagnostic files are excluded from
delivery. Final native vendor jobs on the consolidated commit remain required.


### 2026-10-11 — Exact process completion after image-query failure

The `d83ab95` [Windows Desktop job](https://github.com/luisgui1757/dotfiles/actions/runs/38122626662/job/114420326028)
launched Code and verified its fresh settings witness, then failed fixture cleanup
because QueryFullProcessImageName returned ERROR_GEN_FAILURE for PID 4696. That
log alone does not prove the PID had completed. The helper previously allowed
passive exact-handle completion only after ERROR_ACCESS_DENIED, an incomplete
error-code assumption unrelated to whether termination is actually complete.

Microsoft's [image-query contract](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-queryfullprocessimagenamew)
does not promise one post-exit error code. A [held process handle](https://learn.microsoft.com/en-us/windows/win32/procthread/process-handles-and-identifiers)
remains valid after termination, and the [process object becomes signaled](https://learn.microsoft.com/en-us/windows/win32/procthread/terminating-a-process)
when termination completes. Every failed image query now permits only passive
waiting on that same handle under the existing shared deadline. WAIT_OBJECT_0 is
independent completion proof. A live/inaccessible object preserves the original
query error plus timeout/wait failure; no failed ownership query grants signaling
authority. Outside-process checks and the final empty-root inventory are unchanged.
Other image-query consumers were inspected; this change is limited to desktop
fixture cleanup, not production inventory or native compiler ownership checks.

The existing real held-process regression now calls the full desktopStopProcess
path after confirmed termination. Against the unchanged helper,
[diagnostic `dad9fd4`](https://github.com/luisgui1757/dotfiles/actions/runs/38123079560/job/114421657240)
reproduced the same ERROR_GEN_FAILURE in the exited case; all other six cases
passed. No mock or error-code-specific test assumption was added. With the
correction, [diagnostic `b8a1bc3`](https://github.com/luisgui1757/dotfiles/actions/runs/38123228923/job/114422108404)
passed all seven native cases in 10.25 seconds, including live-denial/outside/
cancellation preservation, then the full Desktop lifecycle in 183.38 seconds.
Both Code launches supplied their own fresh settings witness; Windows Terminal,
update, check, final removal and personal restoration assertions passed.

Exact corrected test-file SHA256:
`7e6bc5d5000453302692edc272bb08ee7ba0fb80f2f9ba97a60cf247459db742`.
Red log SHA256: `2a402990950c8d4d3525c4baf5fa389c559617a4bdf6712f56f41fbe0e6ad314`.
Green log SHA256: `1057f684e5d0679b2fa3a02e3a51ce19ba4b77b32b4a2561e42037367bcd0682`.
Astra found no material correction finding and no missing relevant negative case.
This is a test-only cleanup correction with unchanged signaling authority and the
same deadline. Temporary diagnostic files are excluded from delivery; all final
canonical native checks remain required on the consolidated commit.


### 2026-10-11 — Desktop smoke-test corrections

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


### 2026-10-12 — Replace Pi with Claude Code and Codex CLI

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
binaries also match Anthropic's signed release manifest. Codex strips only the top-level package prefix, preserving metadata, its vendor/
target layout, helpers and resources. Claude's managed shell sets DISABLE_UPDATES=1 only when selected.
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


The first actual desktop Codex installation exposed an archive-layout error in
this new integration: stripping three components would discard the upstream
package.json and README.md. The safe extractor rejected it before publication
and retained recovery evidence. The failing disk snapshot was preserved; the
original installer archived the interrupted transaction through its real menu.
All four Codex pins now strip only package/, with explicit vendor/target command
and helper paths. TestCodexNativePackagePreservesUpstreamMetadata reproduces the
failure on all four old layouts and passes on their corrections. The extraction
safety rule, approval flow and journals are unchanged.


The final Linux agent fixture passes both installed applications, real Codex
app-server initialization, Check after application use, unchanged-pin Update,
repair of a deleted owned Claude entrypoint, selective removal preserving Codex,
final removal, unchanged pre-existing Git and personal history witnesses. Its
first process fixture closed stdin before asynchronous initialization could reply;
it now waits for the acknowledgement before sending initialized and closing.
Its isolated home is under the disposable account because Codex rejects helper
aliases under the OS temporary directory. The inspection controller now discovers
the real APT/Homebrew provider before verifying reused Git, matching the public
process boundary; a constructor without that provider failed the final assertion.
These are test-fixture corrections, not product defects or relaxed checks.

The actual Ubuntu 26.04.1 amd64 desktop installed Claude Code 2.1.287 alone,
then Codex 0.162.1, with Node absent. Both reached their real sign-in menus from
fresh terminals; Claude's offline MCP list and Codex's app-server handshake work.
No credentials or model calls were supplied. Concurrent contributor APT preparation
changed inventory during the Codex operation; the safety guard rejected finalization,
and the original UI resumed successfully after preparation finished. This was
VM test setup interference. The desktop Check left state and profiles byte-identical;
unchanged Update kept every selected resource, and UI Repair restored the owned
Claude pointer. The complete local gate passed with Go format/vet/race and 51
bootstrap tests. All four final target builds and test compilations pass; foreign
runtime and final published-head native/CodeQL results remain pending.


Desktop selective removal keeps Codex 0.162.1 working after Claude is removed.
The last removal preview explicitly retains reused APT Git and removes the owned
CLI and shared shell integration. The CLI onboarding choices add their own theme/
screen-reader preferences; the original harmless settings and history witnesses
remain present. Removal preservation is checked against those post-onboarding
bytes, while original profile bytes and later outside-block edits are checked
separately. The saved failed Codex preparation remains disclosed for inspection.


Final desktop preservation assertions pass: all four settings/history witnesses
match their pre-removal bytes, and the original Bash profile hash matches after
subtracting only the exact personally appended post-install witness. Both owned
CLI entrypoints are absent, selections are empty, Git remains usable and Node
is absent. The VM is shut down after evidence export and retained for inspection.


### 2026-10-12 — Owner-authorized single-commit delivery

The owner explicitly requested squashing all branch changes into one commit with
a meaningful message, superseding the earlier two-commit and no-rewrite limits.
The tested implementation, smoke corrections and native Claude Code/Codex
replacement are preserved together. Active delivery instructions now match this
single-commit policy; dated evidence retains its original source references.
Publishing uses a lease on the previously inspected branch head. Merge, release
and repository protection changes remain outside this action.


### 2026-10-13 — Additional application scope and vendor-package blockers

The owner requested independent OpenWhispr, Obsidian, Tailscale and Zen choices,
allowing Windows omissions where existing installer channels do not fit. The
new candidate reuses private archives for macOS desktop bundles, an official
Linux Zen archive, APT for Tailscale and the existing macOS Homebrew CLI channel.
Native disk-image/XZ decoding feeds the existing bounded archive reader. Saved
APT vendor inputs add an optional pin field; original Code intent and debconf
handling remain separate and old intent shapes remain readable.

The authenticated OpenWhispr 1.10.3 DEB (SHA-256
`9e2372291acf78c15284489c4cc9b73cd67c5973263a3b2c939da1fc57aa41b0`)
contains `postrm` logic recursively deleting `$REAL_HOME/.cache/openwhispr/models`.
Its `postinst` also writes a uinput rule and enables user service/group changes.
The Obsidian 1.14.4 DEB (SHA-256
`85b10dcba6edfc1c0460a6d18260cf31c30447a444bd858a6440b9c9c8806d25`)
copies over `/etc/apparmor.d/obsidian` from `postinst` and removes/unloads that
path from `postrm`; that profile already exists on the clean Ubuntu guest.
These are inspected vendor-script side effects, not observed destructive tests.
Neither DEB is enabled in the catalog. A separate Linux channel decision remains
open; do not interpret macOS availability as Linux acceptance.

Tailscale's reviewed DEB stops its daemon on normal removal but deletes
`/var/lib/tailscale` only on purge. The adapter uses the existing exact dpkg
removal path, never purge, and keeps operation-specific ownership proof.
The full unmodified baseline gate passed in the VM. Focused new tests passed,
and the real interactive menu installed Tailscale 1.104.1 with an active daemon
and unauthenticated `NeedsLogin` state. Remaining acceptance is in progress.

The new Zen Linux launcher smoke found and corrected a URL-forwarding defect in
`installer/desktop_driver.go`: the generic desktop entry lacked `%u`, so
`gio launch dotfiles-zen.desktop https://example.com/` opened Zen with no URL.
The real GNOME launcher reproduced this with an executable argument witness;
`TestZenDesktopLauncherPassesURLsWithoutConvertingToFiles` failed before the
correction and passes after it. Zen now declares browser categories/HTTP/HTTPS
MIME handlers without setting itself as the default. Ghostty retains its existing
entry semantics. The failure VM snapshot is preserved as `failure-zen-url`.
Native CI now includes Tailscale's actual daemon lifecycle and the new available
apps in the existing desktop launch/lifecycle jobs; their final-head hosted
results remain required and are not implied by local compilation.

Subsequent real Zen lifecycle acceptance is **FAIL**, not certified by the green
unit gate. On Linux, launching pinned Zen 1.23.2b adds a zero-byte `.parentlock`
(mode 0664) inside the private payload; the next Check correctly reports changed
archive bytes. Update preserves that old generation and creates another one.
Zen's default profile is keyed by the physical installation directory, so the
replacement opens first-run onboarding with a different profile. Original
bookmarks remain in the original profile. Source points: `tool.zen` pins in
`installer/archive-pins.json`, private publication in `installer/archive_driver.go`,
and desktop launch in `installer/desktop_driver.go`. Multi-location review also
identifies macOS private Zen bundles as needing native validation for the same
installation-identity assumption; no macOS reproduction is claimed.

Minimal reproduction: select Zen through setup, launch its desktop entry, finish
onboarding and bookmark a page, quit, then Check and Update through setup and
launch again. `TestNativeDesktopInstalledApplicationLifecycle` now selects Zen
and will exercise the launch/check path in hosted CI. The saved snapshot
`failure-zen-payload-and-profile` retains the failing state. Check's saved-state
SHA-256 remained unchanged. Canonical completion needs an application channel
that handles Zen's writable update state and stable profile identity; ignoring
archive changes or deleting the lock is not a fix. Mozilla documents the
underlying per-installation profile behavior and supported legacy-profile opt-out
in `toolkit/profile/nsToolkitProfileService.cpp` (`UseLegacyProfiles`); its
[update-mutex implementation history](https://bugzilla.mozilla.org/show_bug.cgi?id=1900726)
explains the installation lock. No lock suppression, permission workaround or
journal deletion was used. These findings remain open in this candidate.

Final Tailscale scoped result: real interactive install and separate machine-wide
removal PASS; the new native daemon lifecycle fixture PASS in 13.79s, including
read-only Check, unchanged-pin update, default retention, explicit removal and
pre-existing ownership/classification protection. Daemon state survives normal
removal. Personal note, profile bytes outside retained managed blocks and Zen's
original bookmark survive. Five pre-existing package versions changed during the
round; `/var/log/apt/history.log` and unattended-upgrade logs attribute all five
to Ubuntu's background security updates at 06:11 UTC, not installer operations.
No pre-existing package record disappeared. Both failed Zen snapshots are kept.
The candidate remains uncommitted, and full four-app delivery remains incomplete.


### 2026-10-13 desktop continuation and Opus 5.5 xhigh follow-up

The owner approved Flatpak for Linux desktop apps, then the official OpenWhispr
amd64 portable archive after upstream/Flathub inspection found no published Flatpak.
The upstream archive SHA-256 was verified against GitHub release metadata. Its
bundled launcher automatically adds `--no-sandbox` on restricted-userns systems;
the candidate exposes that fact in the approval preview and documentation. No
kernel/AppArmor changes, journal deletion or manual product dependency repair
were used. Architecture filtering follows the published vendor targets.

Accepted smoke findings and fixes:

- Flatpak `list --columns=ref` omits the `app/` prefix. The original provider
  misreported a completed install as absent. Parse the native output shape;
  saved native-worker evidence and a legacy-owner regression bind completion.
- Obsidian desktop publication incorrectly verified an archive provider. It now
  verifies the completed Flatpak deployment and adopts/restores a personal desktop
  file through the existing configuration publisher. The regression exercises
  actual controller publication/restoration rather than mocking its internals.
- Flatpak update/removal compared current files with receipt.Before (the permanent
  original absence), rejecting every subsequent update. Compare receipt.After;
  preserve the original uninstall baseline. Reproduced in the interactive menu,
  saved as `failure-flatpak-update-baseline`, then corrected and updated successfully.
- Zen's initial generation adds `.parentlock` and later `updates/`. Preparation
  creates the empty lock before fingerprinting and Linux revision 1 uses the
  supported DisableAppUpdate policy. Revision 0 records remain readable. No tree
  fingerprint exclusion was added. The live browser confirmed the active policy,
  kept the existing profile/bookmark across generations and passed its part of
  read-only Check while running. This is a recipe transition at an unchanged
  browser version, not proof of a real browser version upgrade.
- Opus identified Tailscale's personal `/etc/default/tailscaled` conffile as a
  false health failure and endless repair under `--force-confold`. A failed-before
  regression reproduces it; corrected health ignores dpkg c-marked conffiles and
  still rejects altered non-config files. Native acceptance adds a personal
  conffile witness through Check/update/removal.
- Opus identified Flatpak's system dependency lookup as a private-runtime gap.
  Flatpak 1.16.6 `flatpak_cli_transaction_new` calls
  `flatpak_transaction_add_default_dependency_sources`, which includes all system
  installations. New operations and launchers isolate FLATPAK_SYSTEM_DIR and
  FLATPAK_CONFIG_DIR; Check requires the private runtime. Legacy saved command
  recipes remain valid. Missing-runtime and no-shared-store regressions cover
  the fixed boundary; native lifecycle acceptance is being run separately.

The actual Opus review used model `claude-opus-5-5` and effort `xhigh`; its result
and follow-up are exported outside the checkout. It did not run tests. Its macOS
Zen self-updater concern is not native-reproduced: policy injection inside the
signed vendor bundle is not acceptable, and native LaunchServices/signature and
update behavior remain explicit proof requirements. Linux results cannot certify
those macOS paths. Historical failures above remain dated evidence.

Follow-up corrections: Opus found an over-escaped quote in the new Flatpak Exec
line. A native GIO test with quotes, backslashes, dollar/percent/backtick and spaces
failed before the fix; both desktop paths now use the established two-stage Exec
argument escaping, and the GIO test passes. The graphical rerun then found that
pointing FLATPAK_SYSTEM_DIR at an absent private placeholder breaks Zypak renderer
startup. The saved `failure-flatpak-isolated-launch` snapshot retains this state.
A minimal real `flatpak-spawn --sandbox /usr/bin/true` reproduces the repository
lookup failure; pointing both installation lookups at the same owned private store
passes without exposing normal system runtimes. New mutations still use --user.
The native Flatpak test now includes that portal subprocess path.

OpenWhispr installed through the actual interactive menu and opened its graphical
account-free onboarding. Microphone setup is BLOCKED by the VM having no audio
input device (confirmed in PipeWire); Wayland auto-paste also reports upstream
optional ydotool/ydotoold and input-device permission requirements. Neither
permission changes nor manual missing-product-dependency installation were used.
These are explicit functional gaps, not a successful dictation claim. Its
upstream Linux launcher’s no-sandbox behavior was observed as disclosed.

Final Linux desktop continuation: the final source passed `make ci` (Go race
369.514s, 51 bootstrap tests with two skips; optional unavailable runtime checks
remain explicit in the exported log). Real Flatpak portal lifecycle PASS 115.52s;
real Tailscale conffile lifecycle PASS 14.53s. All selected resources passed
interactive Check without changing state. Full UI removal deleted the three app
launchers/healthy payloads and preserved the note, OpenWhispr witness, Zen profile
bytes and bookmark. Explicit retained libraries/runtimes and earlier modified
payloads remained disclosed. Reinstall completed ready; Obsidian rendered the same
edited note. These are smoke results, not authenticated or version-transition proof.


## 2026-10-13 — T3 Code standalone V2 nightly

Owner choice: use the official standalone package and browser UI on every
supported target. Native desktop packaging was considered: the Linux DEB changes
an AppArmor profile and Windows uses a separate installer channel. The standalone
archive reuses existing ownership, checksum, launcher and preservation machinery.
It ships its own runtime; no Node/npm or new provider adapter is needed.

Release inspection: stable `v0.0.45` was published before PR #2829 merged. Pin
`v0.0.46-nightly.20261013.3053`, source
`6524a8d43795dc543f7cfb572388cdea3662ad48`, explicitly as a nightly. All four
standalone artifacts were downloaded and checked against upstream release digests.
T3 can use existing authenticated CLIs or its own supported provider setup,
including managed Codex through Connect with ChatGPT. Neither CLI is forced.

Linux baseline `make ci` passed. Focused independent-selection checks and the
native application lifecycle passed. The native fixture starts the real T3
server, fetches its frontend and JS bundle, checks database initialization, then
checks no-op update, repair and removal with personal witnesses. Existing Claude
repair coverage remains. The existing four-target native CI job includes T3;
only Linux amd64 has been executed locally. This is not macOS/Windows/arm64 proof.

Interactive desktop smoke and final verification are in progress. Authenticated
cross-provider execution is blocked by absent guest credentials; no host
credentials are copied. Genuine version transitions remain untested by this
unchanged-pin smoke. Historical green hosted runs do not certify this revision.


T3 review corrections: independent Opus 5.5 source review found missing explicit
`glibc` constraints on the Linux T3 pins. A focused regression failed for unknown
libc and musl on both architectures before the metadata fix, then passed with the
constraints. This narrow selection test is the appropriate reproduction because
the desktop guest has a known healthy glibc runtime. The native lifecycle passed
again (79.705s), now also preserving the real SQLite database/sidecars after
removal and starting T3 in the personal project. No archive download bytes changed.

The review also identified disclosure gaps: a first V2 launch copies V1 history
once, after which conversation histories diverge while other state stays shared;
and T3 provider Update/Update all can invoke CLI self-updaters outside dotfiles.
README/UPGRADING now explain a stopped-server backup or separate T3 home and using
dotfiles Update for its managed provider CLIs. ROADMAP includes this owner scope.

Interactive Linux smoke PASS: T3-only selection alongside the earlier desktop
apps; fresh-terminal `t3` opened Firefox and bootstrapped the project; appearance
settings and the embedded terminal worked. Separately selecting Codex and
restarting T3 changed its message from missing CLI to unauthenticated, confirming
discovery without claiming account access. Check was healthy with all 29 captured
control-state entries unchanged. UI Update kept the exact T3 generation while
refreshing existing APT/Flatpak selections. UI removal deleted the managed T3
entrypoint and preserved all 23 captured T3/project files, including the real
initialized database, byte-for-byte; Codex still ran afterward. A working snapshot
was retained before removal. Authentication-dependent delegation and actual
version transitions remain unverified.

Reinstall after the libc correction preserved the same 23 T3/project entries
before startup. The browser reopened the same project, appearance choice and
terminal history, with the expected unauthenticated Codex boundary. This completes
the bounded Linux interactive smoke; no real provider task or version transition
is claimed. Exported evidence remains outside the VM, and the VM is retained for
inspection with the clean and working T3 snapshots.

Final source-bound local verification passed: `make ci` (Go race 369.036s,
51 bootstrap tests with two environment skips) and the focused real native
T3/Claude/Codex lifecycle (74.693s). Command logs bind the observed guest, source
file inventory and complete patch digest; source was unchanged during each run.
The interactive update/removal preceded the libc and description corrections;
reinstall and final interactive Check used the corrected catalog. The final
native lifecycle independently rechecked update, repair and removal. This does
not claim that every earlier interactive action used the final installer build.

Fresh Opus 5.5 correction review found no remaining dotfiles defects and accepted
the libc, provider-prerequisite, V1-state and provider-updater corrections. It
received the complete final delta, captured checks and upstream source. Requested
effort was `xhigh`; returned model metadata identified Opus 5.5, while effective
effort was not independently exposed. Read-only tools were used for this source
review. Live T3 delegation, other platform execution, an old T3 installation and
genuine version transitions remain separate gaps. Hosted readiness still requires
the matching PR head's native/quality checks, rather than historical green runs.

Hosted findings on `281f127` (2026-10-13): Windows quality reported eight failures
from three new fixture entrypoints using raw `t.TempDir()` paths. Native handle
resolution can return a different physical spelling on Windows, so preservation
guards correctly rejected the fixture paths. The fixtures now follow the existing
`resolveConfigPath(t.TempDir())` pattern, and declared launcher assertions use
native path components. No test is skipped and no production guard is changed.
The original hosted failure log is retained; Windows confirmation requires the
new head's quality job.

OPEN, High: macOS Zen's published launcher did not create a native window in
`TestNativeDesktopInstalledApplicationLifecycle/zen`. LaunchServices returned
PID -1, missing executable, terminated=true; the owned process list stayed empty.
The outer wrapper is intentionally unsigned, so its failure-only signature
diagnostic is not sufficient evidence of the cause. Relevant sources are
`archive_zen.go:prepareMacZen`, `desktop_driver.go:recipe` and the native launch
fixture. Reproduce on a disposable macOS arm64 account with
`DOTFILES_NATIVE_DESKTOP=1 go test -v -count=1 -timeout 70m -run '^TestNativeDesktopInstalledApplicationLifecycle$' .`
from `installer/`. VS Code and Ghostty launched successfully in the same job,
which argues against a wholly unavailable graphical session. A supported native
launch diagnosis is needed before changing the wrapper or declaring this platform
ready. This is separate from T3's standalone server/browser integration; no
signature check or browser coverage is disabled to conceal the failure.

The fixture correction passed local `make ci` (Go race 366.105s), with source
unchanged during the captured gate. Independent Opus 5.5/xhigh source review found
no blocking defect and accepted the canonical-root correction; native Windows
confirmation remains a separate requirement. On `281f127`, the T3/Claude/Codex
native lifecycle passed in Linux amd64 (71.02s) and arm64 (44.30s) job logs; the
macOS arm64 and Windows amd64 jobs also reported that specific step successful.
Their broader engine suites failed before that step, so these observations do
not turn either whole job green. The subsequent change only touches fixtures
and this evidence record; the T3 application/runtime source and pins are unchanged.
### 2026-10-13 — Zen macOS launch investigation and current native results

Head `70adf06` completed 22 of 23 required checks successfully, plus CodeQL.
The native temporary-path fixture corrections are confirmed on Windows and macOS;
the sole remaining failed required job is macOS Desktop lifecycle (job
`115258318849`, run `38386101990`). Zen still produces LaunchServices PID -1,
no executable and no owned native window. Other exercised desktop applications
in that job start normally. Keep this failure visible.

The Homebrew `zen` cask and our pin use the identical official `1.23.2b` DMG,
SHA-256 `97548c8123d31c8123f097621b47e6b99cdeb57a8ba3c29b790dd065157f1d9e`.
The artifact does exist and supports Apple Silicon. The vendor Info.plist names
`zen`, matching the wrapper executable; a metadata-name mismatch is ruled out.
The next native run compares headless startup of the wrapper and original vendor
executable after the GUI failure, with separate disposable profiles, bounded
output/time and owned-process cleanup. These diagnostics preserve the failing
GUI assertion and do not establish the root cause or count as a product fix.

### 2026-10-13 — Resolve Zen's macOS executable path before startup

Diagnostic head `4451fd1`, macOS job `115365245769` / run `38417361053`, narrows
the failure: direct wrapper invocation exits 255 with `Couldn't load XPCOM`,
while the original signed binary renders a real headless screenshot successfully.
Both use the identical extracted vendor payload and isolated profiles. This
rules out a missing package and narrows the issue beyond unsigned-wrapper status.

[Mozilla BinaryPath::Get](https://github.com/mozilla-firefox/firefox/blob/2223a8bee2976d938fe4fc2f7b9e9a137d519521/xpcom/build/BinaryPath.h)
uses CFBundleCopyExecutableURL on Darwin and removes `./` substrings; it also
removes them from `../` components. Our wrapper exec path traversed three parents,
so the subsequent XPCOM library lookup used a corrupted path. Preparation revision
1 resolves the vendor directory physically before exec in a subshell, retaining
the caller's directory, arguments, supported profile selection and signed bytes.
Revision 0 remains byte-identical for saved recovery; current desired pin identity
changes to revision 1 for Update. The official DMG remains the same version/hash
as the [Homebrew cask](https://formulae.brew.sh/cask/zen); no new provider is needed.

The process-boundary regression failed before correction and passed afterward.
Expanded coverage exercises spaces/Unicode, a published application symlink,
relative arguments, caller working directory and unchanged vendor tree. A saved-
revision regression retains the original launcher bytes. Native GUI/lifecycle
confirmation remains required and is not inferred from Linux execution.

Native correction head `8fb7878`, job `115372652607` / run `38419316157`, confirms
the XPCOM path correction: both wrapper and vendor now render screenshots. The
GUI fixture instead reaches its LaunchServices-completion timeout. The next
diagnostic records owned windows before cleanup and compares ordinary `open`
against the forced-new-instance NSWorkspace helper; its failure remains a failure.
Opus accepted the launcher/recovery correction and identified a native-test gap:
automatic adoption during Update could hide app-mutated payloads. The fixture
now requires Check ready immediately after use and refuses adoption during any
Update. Exact revision-1 launcher bytes are covered as well as revision 0.

Native head `dd8e256`, job `115382374303`, proves both forced-instance and ordinary
LaunchServices startup retain an owned Zen process without a visible native window.
The headless wrapper/vendor comparison still passes. This rules out merely an
NSWorkspace callback mismatch. The next failure-only diagnostic compares an
untouched vendor bundle's ordinary launch and direct wrapper GUI startup, and
samples the already-proved owned stalled process. No launch assertion is waived.
The stricter local `make ci` gate passed (Go race 371.849s), with source unchanged.

### 2026-10-13 — Declare the Zen launcher's Apple Silicon architecture

Head `794fc39`, macOS job `115391521717`, identifies the second startup defect:
`sample` reports `Code Type: X86-64 (translated)` for the published shell bundle,
stalled inside Rosetta's dynamic-loader path. Ordinary launch of the untouched
vendor bundle creates a native window, as does direct execution of the wrapper
with a fresh profile. Neither the downloaded app nor the desktop session is
wholly broken. [Apple documents script-only bundles' Rosetta default and the
LSArchitecturePriority correction](https://developer.apple.com/documentation/apple-silicon/building-a-universal-macos-binary).

Preparation revision 2 adds the ARM64-only declaration to the outer Info.plist,
using XML token offsets to preserve all unrelated metadata bytes and refusing an
unexpected/conflicting upstream shape. The signed bundle remains unchanged;
revisions 0 and 1 retain their saved bytes. Linux still permits only its own
revisions 0/1. The regression fails before the change and passes afterward. Native
GUI acceptance additionally checks the launched application's architecture;
window ownership, pre-update Check, no-adoption Update and preservation assertions
remain mandatory. Full native closure is pending this revision's run.

The source-bound local gate on `794fc39` passed (Go race 371.079s). A normal Linux
desktop launch after reboot restored the prior tabs and rendered a page, followed
by interactive `./setup.sh` Check ready while Zen remained open. The installer
state hash was unchanged; screenshots/transcript are exported alongside prior
failures. This Linux result does not substitute for the macOS correction rerun.

Independent Opus 5.5/xhigh review found no blocking defect. Its accepted low-risk
gap adds an exact revision-2 metadata golden fixture, including Apple's DOCTYPE;
its wording correction records that any existing architecture declaration is
refused and the native revision-1 to revision-2 transition is unverified. The
review received only selected sample lines: the retained full sample does include
dyld `dlopen_from`, `Loader::getLoader` and `makeDiskLoader` above Rosetta frames,
so "dynamic-loader path" is supported. A causal distinction between architecture
and the other control differences still depends on the normal native rerun.
Effective review effort is not independently exposed; requested effort was xhigh,
and returned metadata identifies Opus 5.5. This source review is not a GUI pass.

Native closure: `5657dc5`, macOS job `115406187261`, passes the complete desktop
lifecycle in 289.44s. Zen passes in 9.75s with LaunchServices architecture 16777228
(ARM64), the existing owned-window assertion, Check before Update, no adoption on
unchanged Update, removal and personal-file restoration. VS Code, Ghostty,
AeroSpace, OpenWhispr and Obsidian pass as well. This closes the two published Zen
startup defects without changing the vendor artifact or disabling any assertion.
The default-browser/Dock routes and actual old-recipe transition remain unverified.


### 2026-10-17 — Stable T3 and categorized tool selection

Owner instruction: move T3 off nightly now that Orchestrator V2 is stable, and
provide category checkboxes with independently selectable indented children.
The Git/development labels and old-main installation behavior were also examined;
legacy detection and migration changes are explicitly outside this batch.

Upstream stable v0.1.0 was published 2026-10-15. All four supported standalone
archives were downloaded and verified against both release asset SHA-256 digests
and the independently downloaded SHA256SUMS file. Required command, client and
node-pty metadata paths exist in every archive. The Linux amd64 binary reports
v0.1.0; its isolated real server serves HTML/JavaScript, initializes statev2.sqlite,
and leaves every payload file byte-identical. This does not certify authenticated
agent use, other native platforms or migration of actual nightly user state.
The stable-channel regression failed against the original nightly pin and passes
with the new pins. Ordinary archived-operation recovery semantics are unchanged.

The catalog supplies presentation-only category names. The controller filters
capabilities before passing category metadata to the terminal; virtual parents
never become graph resources, request IDs or persisted choices. Empty categories
cannot appear. Mixed parents select all children, fully checked parents clear
all, and children remain independently toggleable. Existing category-free state
still opens correctly, and removal offers only recorded leaf selections. Adoption,
retention and shared-removal prompts retain their explicit per-item decisions.
Git and Lint and benchmark tools now have clearer labels and operator explanations.

Focused controller/catalog coverage passes for macOS arm64, Linux amd64/arm64 and
Windows amd64 definitions. Actual Linux PTY coverage passes parent/child selection,
mixed/saved selections, all/none, singleton categories, noncontiguous input groups,
narrow-terminal scrolling, cancellation, cooked-input restoration and a connected
controller publication only after explicit approval. Native ConPTY/macOS execution
remains a hosted check.

Baseline make ci initially stopped at missing Ruby. Private temporary Go/Ruby/Clang
tooling was prepared without installing host packages. Baseline Go race checks
passed (370.042s); the Neovim baseline passed after isolating the temporary compiler
libraries to that compiler process, avoiding interference with the existing Nix
clangd runtime. Integrated final verification and Opus review follow below.

Opus review reconciliation (2026-10-17): the initial read-only repository review
and a separate frozen-packet review both returned claude-opus-5-5 metadata;
xhigh was requested with the supported CLI flag, while effective effort is not
independently reported. The first review observed concurrent edits and is not a
frozen final-patch review. The second reviewed supplied complete source, diff and
runtime evidence, with no tools; it found no correctness defect in selection,
platform filtering, ID handling or ownership prompts. Neither independently ran
our tests.

Accepted corrections: keep category members contiguous in the README's fixed
order across platforms; validate all-or-none capability categories, reject hidden,
blank/padded/control labels and retain category-free catalog compatibility; show
an explicit selected count; assert flat ownership/action prompts in controller
journeys; target the actual singleton parent in its PTY case; pin T3 document
links to verified commit 81493efb71e8e8244f95d3e9f71d8369654ab151; add the missing
T3 row to the status matrix; explain that adopting an XDG Git config replaces its
active settings while saving the original. The stable release notes independently
confirm the documented provider floors. Pinned binary --help confirms both
--base-dir and T3CODE_HOME; the documentation's differing --home-dir spelling is
not evidence to change the verified invocation.

Rejected findings/preferences: the user explicitly requested a better group name
and offered taxonomy examples, so Shell essentials/Editors/Terminal sessions do
not require another permission round. Merging both terminal categories is optional
and would discard the useful app/session distinction requested in the examples.
The suspected stale README T3 checkbox was already “T3 Code (browser UI)”, not a
nightly label. Arbitrary category length limits or a second category registry are
unnecessary: the renderer bounds labels, and the four-platform default-catalog
regression checks exact complete category order. The footer truthfully counts
navigable rows, including parents. Standalone argument behavior and actual nightly
conversation-state compatibility remain separate verification questions.

OPEN, Medium — existing POSIX Git configuration write-through (outside this
batch's ownership-change scope). Location: installer/config-targets.json's
config.git target, config_targets.go's default POSIX link mode, and git/config.
With no ~/.gitconfig, Git chooses the XDG Git config for --global writes and
follows its live symlink into the checkout. A disposable-home reproduction used
only fixture@example.invalid and confirmed the link stayed intact, the checkout
source acquired the fixture identity and no ~/.gitconfig was created. This
revalidates Opus's initially uncertain write-through hypothesis without inspecting
or changing personal Git configuration. Both current and passive released Git
targets use this pattern; Windows uses a copy. The current user instruction asks
what the selection does, not to change its ownership semantics. README now
explains the consequence and explicit --file ~/.gitconfig alternative. A separate
fix should design scoped Git settings or an explicit copy recipe with original
baseline/recovery compatibility and native regression coverage. Confidence: high
for the reproduced POSIX behavior; no Windows write-through claim.

Additional native evidence: the actual Linux ArchiveDriver/Controller installed
and executed the previously pinned nightly, then updated to stable v0.1.0,
executed it, removed the unchanged old generation, passed read-only Check and an
unchanged Update, and removed the owned payload while preserving a personal file.
Only the HTTP transport used already verified local release bytes where available;
the real checksum, extraction, publication and ownership paths ran unchanged.
This proves a package-version transition, not migration of real T3 conversations.

The full local gate's shell PATH test requires /bin/zsh; this host only has
Homebrew zsh. Its unchanged test passed in a disposable Debian container after
installing zsh there, using the actual generated ProfileDriver fixture. No host
package was installed and no check was weakened. An initial container preparation
failed because the image defaults to a non-root user; preparation was rerun as
container root. The host make -k ci retains its failure status while continuing
independent gates; final source-bound results follow below.

Final Opus follow-up also returned only claude-opus-5-5 model metadata with
requested xhigh effort. It found no correctness/compatibility defect in the
corrected validator or selector, but identified two weak regressions and one
coverage gap. These are now corrected: category validation uses an interior
escape character, ownership journeys put categories on the actual adoption/shared
resources and assert the adoption/shared-removal/retention titles were reached,
and a default-catalog removal test proves catalog order wins over saved-ID order.
All strengthened category/controller cases pass with the race detector (1.095s).
The suggestion that the adapter lacked proof was a packet-coverage limitation:
TestNativeControllerWorkflowAppliesOnlyAfterExplicitConfirmation already waits
for the actual AI coding parent, toggles it through Workflow, and verifies only
the child ID was saved after explicit approval. No production changes followed
this last review; only the two regression-test files changed.

The earlier standalone launch uncertainty is closed: the verified Linux stable
binary also serves its frontend with no subcommand, using --no-browser and an
isolated --base-dir. Actual nightly conversation-state compatibility remains
unverified. Native macOS/Windows/Linux arm64 runtime validation remains required;
successful builds and test compilation for those targets do not replace it.

Integrated verification caught an additional fixture incompatibility in
TestGeneratedShellProfilesRestoreAfterFreshControllerSelectionChanges: it adds
two uncategorized capabilities to the now-categorized default catalog. Both the
absent-profile and personal-profile cases failed category validation. The fixture
now assigns its two synthetic capabilities to Shell fixtures, preserving every
lifecycle/restoration assertion. The focused race regression passes afterward;
other appended-capability fixtures were inspected and use independent flat
catalogs. A fresh complete installer format/vet/race gate follows this correction.

The integrated make -k ci run passed static/lint, Neovim, Starship, tmux,
AeroSpace, Renovate's 67-record extraction, bootstrap (51 tests, two native
PowerShell skips) and terminal race coverage (23.825s). PowerShell lint and
Ghostty-native validation reported missing-tool skips. Its failures were the
host shell PATH prerequisite described above and the subsequently corrected
shell fixture (full root race run 361.768s). Neither failure was suppressed.
Final macOS arm64, Windows amd64 and Linux arm64 test compilation passes again
with the strengthened regressions. Catalog comparison also confirms every
dependency, platform restriction, binding and ownership field is unchanged.

The confirmed Git write-through finding's exact current locations are
installer/config-targets.json:63, installer/config_targets.go:138 and git/config:1.
No personal Git configuration was used in the reproduction.

Final installer gate passes after the fixture correction: make test-installer
completed gofmt validation, go vet ./... and go test -race -timeout 20m ./...
(root package 361.974s; terminal uses the passing 23.825s cached result, with no
terminal source changes). All 380 installer-file hashes match the snapshot taken
before this final run. git diff --check passes. The broader make ci result remains
non-green on this host because its shell PATH test requires system-path zsh;
that unchanged test passed in the disposable container. These local results do
not certify final-head hosted checks or native macOS/Windows/Linux arm64 runtime.
Temporary downloaded toolchains, isolated T3 data/payloads, build outputs and
shell fixtures are removed after verification. No commit, push or merge was made.

The final documentation pass also completes make test-static successfully,
with the same explicit missing-PowerShell parse skip.

### 2026-10-17 — Git default ownership and requested legacy retirement

The owner's follow-up explicitly authorizes fixing the Git write-through finding,
setting rebase-on-pull, and implementing migration that detects and retires the
previous Nix/chezmoi installation. This supersedes the earlier analysis-only and
retention-only scope. Windows Terminal remains available only on Windows under
Terminal apps; a four-platform catalog regression now asserts that boundary.

Actual Opus 5.5 was invoked with `--effort xhigh` for design consultation and a
separate frozen Git packet review. Returned model metadata identifies
claude-opus-5-5; effective effort is not independently reported. Neither review
ran tests. The design review highlighted Nix-managed Homebrew, native installer
flavor, other users/roots and login shells as required retirement boundaries.
The isolated implementation proposal and final full review are separate work;
retirement is not yet certified or complete.

Git failed-before evidence: in a disposable home, the original application's
whole-file link allowed real `git config --global user.email` to modify its
checkout fixture. The old source also lacked pull.rebase. The new resource uses
a marked include at the beginning of a regular XDG file. Actual Git tests prove
personal override precedence, fetch pruning, divergent-history pull by rebase,
global identity writes without checkout changes, and personal settings surviving
Update/Remove. Existing UTF-8 BOMs are preserved; UTF-16 is refused, and new
Windows Git files do not receive the PowerShell publisher's BOM. Defaults remain
live checkout configuration: their content is not represented as a delayed
approval requirement. A missing source blocks application but allows removal.

Opus's accepted Git corrections: finalization ignores native Git's foreign regular
lock file; global include.path edits are documented as edits to the managed
section and remain retained on removal; invalid UTF-8 paths are rejected; saved
Git targets remain bound across HOME/XDG changes, with relocation requiring
explicit removal. Git's process configuration paths are independent of Windows
known folders. GIT_CONFIG_GLOBAL produces a resource-local block. A native Windows
regression compares discovery against real `git var GIT_CONFIG_GLOBAL`; native
execution remains pending, not implied by test compilation.

The new include cannot compete with an old created/uncertain config.git receipt.
Explicit old removal restores the first baseline before re-selection. A regression
exposed the old reused receipt's final preservation check still competing with
the successor's writes. The catalog now declares replaced_by for that transition:
the pure planner releases only reused bookkeeping when its successor is selected
or recorded, leaving owned/uncertain entries and all physical backups protected.
The real application lifecycle covers old adoption, blocked competing setup,
refused legacy detachment, old removal, new setup and new removal with exact
baseline restoration. Passive released Git configuration is unchanged.

Migration preparation now recognizes released configuration links and preserves
all readable bytes in regular files, including old Git identity/settings. Existing
regular files are not inferred as owned from an inventory path. Runtime uninstall
work is separate and incomplete at this checkpoint. No commit or push has been
made for this batch. Final gate and independent review evidence will be appended
when those actually finish.

The first integrated Git gate exposed two fixture assumptions: the native-package
retention test named config.git, and the all-selections owner test counted the
recovery-only historical mapping as a current owner. They now assert the scoped
integration's removal and reject any current capability closure that reaches a
replaced mapping, retaining every existing personal/native preservation assertion.
The failed run completed root race tests in 370.425s and terminal in 23.899s;
its failure is preserved as evidence, not reported green. Focused race tests pass
after correcting those expectations. A fresh complete gate follows.

The unchanged system-path shell test passes in a disposable Debian 13 container
with zsh installed there. Neovim checks pass with an independently extracted APT
Clang toolchain scoped to that compiler process; host packages were unchanged.
The whole host shell gate still correctly reports its missing /bin/zsh
prerequisite. Static checks pass. These are local results, not final-head native
macOS/Windows/Linux arm64 certification.

The corrected full installer gate passes (root race 369.452s; terminal retains
its passing cached result). Subsequent case-insensitive migration target deduplication
passes focused Linux Git/Legacy race checks (2.531s); its new Windows regression
still requires native execution. macOS arm64, Linux arm64 and Windows amd64 test
binaries compile; cross-compilation is not native execution.

A disposable Debian 13 fixture installs actual released upstream Nix 2.34.0 and
chezmoi 2.73.0 using the retained main helpers. The released chezmoi configuration
layer is applied with files/symlinks only and the same parent-directory preparation
as main. Direct chezmoi apply initially failed because those parents were absent;
that fixture preparation error was corrected without changing the source layer.
The new public machine entrypoint previews and migrates all 17 detected profiles
and configuration links. A fixture identity deliberately written through the old
Git link remains active afterward; the old source stays byte-identical, the target
is regular, and a subsequent real --global write leaves the checkout unchanged.
Normal setup then selects Git through the actual native APT/profile providers and
Check returns ready. This proves configuration preparation and the replacement
Git path only; Nix/chezmoi retirement has not run and is not claimed by this proof.


Opus 5.5 (`--effort xhigh`) completed a second frozen-packet Git review with no
tool access and no permission denials. It is a component review, not the final PR
review. Three accepted findings reproduced before correction: a redirected Git
parent could publish a self-include into the checkout; missing baseline evidence
aborted independent previews; historical locks could fail finalization after a
parent redirect. The new checks reject checkout overlap locally, retain damaged
evidence as resource-local Pending, validate every journal before cleanup, and
disclose historical cleanup failures while preserving fatal current-operation
recovery. Git paths with casing-only changes on Windows retain the saved spelling.
The same saved-baseline error propagation in shell selection is now resource-local.

A fourth accepted finding reproduced for edited whole-file Git configurations
and an uncertain receipt with intact completed publication evidence. The existing
per-item removal choice now offers explicit restoration of replaced configurations:
current edits are preserved separately and the first baseline is restored. Default
removal remains protective. Missing legacy publication evidence cannot authorize
this choice. Old reused bookkeeping can then yield to scoped defaults. Tests
exercise both ownership states through the real controller and both generations'
removal; no original baseline is overwritten. Personal non-dotfiles Git sync links
are no longer offered as released migration targets. The passive released Git
source has fetch.prune only, so it does not shadow the new pull.rebase default.

The post-review Git/Legacy race suite passes (6.083s before the final personal-link
regression). A fresh integrated gate remains required after this follow-up. The
preceding complete make-ci run passed installer race checks (363.825s), Neovim,
static, and bootstrap checks; its overall exit remained nonzero because /bin/zsh
is absent on the everyday host. The exact affected shell fixture separately
passed in a disposable Debian container. No native-host packages were installed.


The next full installer gate (369.252s root, 23.966s terminal) exposed two
integration mismatches and is recorded as failed. The shared-dependency prompt
now keeps its original wording unless a legacy-restoration option is present.
The Bash login recovery test now verifies resource-local Unknown/Pending instead
of expecting the removed global ForSelection error; it additionally calls the
publisher and proves both original and higher-priority personal profiles remain
byte-identical. This preserves the original no-retargeting invariant. A separate
filesystem-failure regression proves legacy Git removal can resume after durable
engine intent but before the provider journal is written. Static checks and the
updated Windows test cross-compilation pass; native Windows remains pending.


The failed-only diagnostic rerun of installer-engine run 38430179152 (attempt 2,
unchanged remote head dd2382b) completes successfully, including Windows core
117310460494 and Windows Neovim 117310460628. This clears the earlier Windows
Build Tools/VC++ query and SourceForge TLS failures on a fresh runner without
changing timeouts or weakening assertions. It is historical-head evidence only;
none of the current uncommitted Git/category/stable/runtime work is certified by
that rerun. The current focused Git/Legacy/Bash-login/category race checks pass
(8.096s), including the corrected integration assertions.


The disposable Linux fixture additionally applies the actual released Home
Manager package layer through main's run_home_manager_switch helper and locked
inputs. Activation succeeds; the bootstrap Nix remains 2.34.0 and the profile
contains home-manager-path plus nix-2.34.0. Normal replacement Git Check still
returns ready. Native inspection disproves the overly broad assumption that
"packages only" means no generated home links: implicit Home Manager modules
also publish .config/environment.d/10-home-manager.conf,
.config/systemd/user/tray.target, .local/state/.keep and .cache/.keep. Runtime
retirement must handle these verified links before store removal. This is
preparation evidence; runtime retirement remains unfinished at this checkpoint.


The final non-runtime Opus 5.5 review (`--effort xhigh`, tools disabled, frozen
packet) completed with no permission denials. It found two high and two medium
Git handover defects. All reproduced before correction: declining restoration
created a Keep root for the retired mapping; a failed Git target binding had no
native route; retired receipts retained Git/shell dependencies; and cancelled
first publications incorrectly required a backup already left at or restored to
the target. Corrections prevent retired publication/adoption/Keep roots, preserve
local routing and the original discovery error, remove retired dependencies, and
use saved cancellation/restoration proof to release only bookkeeping. The legacy
fixtures now explicitly model the earlier catalog without `replaced_by` metadata.
The original-baseline and edited-file preservation assertions remain intact.

Focused Git/category checks pass (3.106s). The migration constructor's dangling
parent failure also reproduced before correction. Deduplication now uses a
lexical key when binding is unavailable; invalid Git discovery is a local,
visible blocked item. Combined Git/Legacy/category race checks pass (3.533s).
Approval text discloses that restoring an older Git baseline makes settings from
the displaced copy inactive until reapplied. Saved preserved paths remain visible
in local-error and released-receipt observations. Existing historical recovery
reports are intentionally retained. A fresh component follow-up and integrated
runtime review are still required; this is not final-head readiness.

The preceding non-runtime full local `make -k ci` run completed installer race
checks (367.305s root, 23.928s terminal), static, bootstrap and editor checks. Its
whole-run exit remains nonzero for the unchanged host /bin/zsh prerequisite gap;
the exact shell fixture separately passed in a disposable Debian container.

Opus's separate runtime implementation proposal completed in an isolated tree
using actual claude-opus-5-5 with requested xhigh effort, with no permission
rejections. It explicitly reported unrun tests and remaining Windows, historical
pin, native-acceptance and recovery gaps. Consolidation does not treat the proposal
as accepted proof. The first real-store preview regression found that adding a
valid Nix store member did not change approval; binding store membership and
profile/root snapshots makes that same native before/after regression pass.
No store deletion has run at this checkpoint. Initial integration also caught
main-vs-v0.4.4 pin provenance confusion: the proposed pins match the immutable
pre-overhaul main ae9a644, not the earlier release tag. The evidence and native
fixture now explicitly identify that main commit. Runtime acceptance is unfinished.


Runtime consolidation (2026-10-17, continuing): exact machine preview paths now
match the approved scope. Replacement-state changes invalidate recovery; Linux
process inspection refuses unreadable live processes and hidden proc inventories.
macOS parsing keeps lsof warnings fatal, identifies the launchd daemon by PID and
kernel text executable, and binds APFS removal to the observed store membership.
Root preservation uses one saved native outcome per adjacent recovery path.
Focused legacy tests pass; macOS process/APFS behavior still requires native proof.

The actual pre-overhaul main single-user fixture removed chezmoi and Home Manager
links, then stopped before Nix deletion because Docker root lacked SYS_PTRACE.
A separate same-fixture process read reproduced that permission error. Recreating
the disposable checkpoint with SYS_PTRACE and reviewing a fresh remaining plan
allowed store deletion. That image's lower-layer directory rename then failed
with EXDEV during user-state preservation. The fixture copied the remaining
private legacy directories into its writable layer with exact recursive diff
checks, abandoned the old intent, and approved the remaining work. Final runtime
result is ready; Nix/chezmoi and four Home Manager links are absent. Personal Git
identity, main checkout bytes and replacement setup Check are unchanged/ready.
These environment corrections are disclosed: this is single-user deletion and
fresh-approval recovery evidence, not a clean first-pass daemon/macOS/Windows run.

The native recovery exercise exposed lost earlier backup disclosures after an
abandoned partial operation. A two-package engine regression fails before and
passes after carrying earlier Preserved paths into the new intent. Combined
legacy/installed-migration race checks pass (3.716s). A real private npm prefix
query also confirms hidden adjacent Pi recovery directories are absent from npm's
installed package inventory; no host package was installed or removed.

Published v0.4.4 pins are now separately recognized alongside exact main ae9a6446.
Both source sets retain immutable provenance tests. The native acceptance fixture
uses the published tag because the released prerequisite helper deliberately
rejects old branch heads after main advances. It uses a real Nix Bash from Home
Manager's activation script for login-shell coverage, instead of a non-shell
stand-in. Pin and focused legacy checks pass (1.940s); the new native fixture has
not run yet. Core checkout now includes the history required by provenance checks.

The second non-runtime Opus5.5 xhigh review completed (847789ms, tools disabled,
no denials). It found one high double-binding/approval issue and two medium
persisted Git recovery issues; the prior fixes did not cover execution or later
personal edits after restoration. These are not accepted as finished work.
A separate isolated Opus implementation pass is reproducing and correcting them.
Runtime review and Mac/Windows native-manager consolidation remain in progress.


Runtime review consolidation (2026-10-17): actual Opus5.5 xhigh completed its
read-only component review in 1113971ms with no permission denials. It did not
approve the implementation. High findings concern partial-deletion recovery,
macOS busy-volume ordering and insufficient Determinate uninstall scope proof.
Nix recovery correction and native-manager integration are still underway.

The repeated-retirement finding reproduced through the engine: a second released
Pi install planned Keep after the first completed removal. Fresh inspection now
offers a new approval, saves its complete plan with the operation, and keeps the
original baseline and previous recovery paths. Old intents without the optional
approval document remain validated against their original Before digest. The
engine regression passes; no existing receipt shape was discarded.

Three local recovery failures also reproduced before correction: a redirected
cache parent was refused; missing staged edit bytes deleted the system-file
fixture; an existing intact backup prevented completion. Approval now binds the
physical destination parent. Root stages and verifies the exact replacement
bytes before publication, and only an explicit absent outcome permits deletion.
An identical existing backup can complete recovery. Focused race checks pass
(1.154s), retaining the original/personal-content assertions.

An inspection failure previously hid independent runtime choices. A real invalid
plugin-parent fixture now leaves Pi selectable and reports the tool-local error.
A procfs fixture without Kthread now recognizes PF_KTHREAD from stat flags;
spaces and closing parentheses in comm are covered. The bit and stat layout
were checked against Linux's sched.h and procfs documentation. Combined focused
race checks pass (1.100s). Missing root files no longer create fictitious backup
disclosures; restoration destinations are not labeled backups (1.167s focused
race check). Recovery gate refusal no longer consumes a native command attempt
(1.031s focused race check). Combined legacy/installed-migration checks pass
(4.058s before the final attempt/disclosure integration).

Review hypotheses F6 and F7 are rejected for the actual released source: the
prerequisite helper supplies --no-channel-add and --no-modify-profile, and
nix/darwin/configuration.nix explicitly sets nix.enable=false because Determinate
owns the daemon. Removing all Determinate support would not fulfill the owner's
request for that earlier macOS setup. The unsafe path-only receipt allowlist still
needs replacement with verified vendor scope; this rejection is not an approval
of that implementation.

The published v0.4.4 full setup fixture initially stopped in the released
latex2text step because the clean Debian account lacked python3. After installing
python3/python3-venv as explicit disposable-runner prerequisites, the unchanged
published setup completed successfully. It installed Linuxbrew and manager-owned
chezmoi, not the direct marker assumed by the proposed fixture. Migration coverage
and native assertions must follow that observed provider. No host package or
service was changed. The latest full local gate passed installer race checks
(375.716s root, 23.807s terminal), static, editor, bootstrap and official Renovate
extraction (67 records); its only failure remains the host /bin/zsh prerequisite.
Final-head native checks and final independent review remain required.


Runtime consolidation follow-up (2026-10-17): the actual published v0.4.4
installation now passes released configuration migration, replacement setup,
chezmoi retirement through Linuxbrew, Pi retirement, old plugin checkout removal,
and pylatexenc environment removal. Every detected explicit Linuxbrew tool delta
then retired through Homebrew; the normal replacement Check remained ready.
These were real native operations in the disposable Debian container. Nix was
intentionally left for the remaining recovery correction; this is not a full
migration acceptance claim. Shared Homebrew infrastructure and automatic
unattributed dependencies were retained.

The actual release environment exposed setuptools 83.0.0, while pre-overhaul
main pins 84.0.0. The failed-before regression now covers both immutable release
shapes through complete filesystem retirement and rejects an additional personal
distribution. Focused manager race tests pass (1.039s). SourceDir parser cases,
macOS query-failure handling, platform boundaries and Brew consumer protections
also pass; no native macOS or Windows execution is inferred from these tests.

A second engine regression proved unchecked reinstalled Pi was selected again
because all historical receipts were appended to the new choices. Retirement
now uses only the current choices, and the planner retains retirement history
without interpreting actionable residue as disposable bookkeeping. The prior
failure and passing consolidated race run (5.438s) are retained. This also keeps
runtime history out of the earlier Homebrew preparation selection.

The isolated actual Opus Git correction pass ended at the account usage limit,
not an approval. Its final full race log passed (372.742s root; 23.861s terminal),
and its scoped changes are integrated: single selection binding, repeated
restoration consent, restoration proof after later personal writes, local
migration inspection failures, and saved Git destinations under changed discovery.
Parent consolidated focused tests pass. Final independent review remains open.


Additional parent consolidation (2026-10-17): the full pre-consolidation local
race suite passed (372.256s root; 23.961s terminal), with bootstrap and official
Renovate extraction also passing. The host gate still fails only the unavailable
/bin/zsh path; the unchanged generated-shell PATH test passes in the disposable
published-install container. Windows amd64, macOS arm64 and Linux arm64 test
compilation passes; this is not native execution evidence.

A refused user-file preservation move also recorded a nonexistent backup. The
inode-replacement regression reproduced the misleading disclosure; local moves
now record only an existing backup, as native moves already do. Focused legacy
race checks pass (2.153s). A same-inode content edit is not this bug: preserving a
path intentionally keeps its current contents intact, while identity replacement
requires another approval. The narrower first hypothesis was rejected after
checking the move's documented inode contract.

Native font reuse exposed a separate conflict. FontDriver accepts a healthy
existing family as reused, but manager retirement still offered its removal.
The failing regression now protects a selected reused native family while keeping
unselected historical receipts from blocking explicit retirement. Consolidated
focused legacy race checks pass (2.349s). A Mac native acceptance extension now
selects AeroSpace and exercises its published Applications entry after old-cask
removal; it compiles, but native execution remains pending.

The actual Linuxbrew retirement also exposed a high-severity login-shell gap:
the released installer changed the fixture's login shell to the Brew zsh path,
and removing that formula left the account pointing at a missing executable.
Normal setup Check was still ready because it does not own the login shell.
Failed-before regressions cover both installed and already-removed formula
states. The native trace and required account/consumer correction were passed to
the ongoing actual Opus manager pass. This remains open until corrected and
verified through a new native login. Earlier package checks do not close it.


A native process-consumer test found that the Linux scan missed a system
executable whose cwd and open file descriptor still referred to /nix/store.
The original scan exited zero with no consumers; the corrected kernel-reference
scan reports that live PID, alongside its parent holding the same directory.
Focused regressions also cover cwd, process root and an open descriptor separately.
Uninspectable live references fail closed; descriptors that close during the scan
are tolerated. This extends the existing no-live-consumer removal contract to
open data, not just executable/library mappings. Native before/after evidence was
produced in the disposable published-install container, not the daily host.

The native Linux acceptance now exercises the released Linuxbrew account shell
before the separate forced Nix-shell case, and starts the configured account
through su --login after each retirement. The already-migrated fixture's
before-fix login fails with exit 127 and the missing Brew zsh path. A second fresh
Debian fixture has fully installed immutable v0.4.4 and remains untouched by new
migration for the final ordinary-path proof. Login repair remains open.


The actual Opus 5.5 xhigh manager-completion v2 pass finished successfully
(159 turns, 3346.951s). It adds native-manager provenance checks, Windows Pi
shim retirement, stable-release direct pins, Linux DEB/font/rule deltas, and
Git/GitHub-CLI-independent query paths. Scoop accepts its current metadata names;
Chocolatey scripts must match retained package archives and its community source;
Windows process queries no longer discard unreadable target images.

The worker reports 15 failed-before regressions, focused race checks, all 14
released archive downloads/member hashes, and a full race pass (423.684s).
That full run preceded its final npmrc correction; the correction had focused
race coverage. The parent integrated the six scoped files while preserving the
selected native-font guard; consolidated legacy race checks pass (2.772s).
The earlier independent parent core and font checks passed (2.781s, 1.036s).

Native manager lifecycles are connected to the existing core CI jobs. The
Windows fixture uses the released-supported hyperfine rather than preinstalled
jq; the official Windows Server 2025 runner inventory lists jq. The fixture
still requires its chosen package absent before installation. Native Windows,
macOS, and Linux pinned-DEB execution remains pending. Scoop native lifecycle,
Windows direct-artifact/Pi proof, and the confirmed Linuxbrew login-shell repair
are still open; the implementation pass is not a final review approval.


The manager pass also identified a reporting gap: a deliberate shared/reused
keep with no removal steps disappeared from the result. A failed-before test
using a persisted reused native-font receipt reproduced the missing explanation.
Runtime reports now distinguish Remaining actions from Kept explanations. Kept
components do not block readiness; pending, unrecognized and interrupted
retirements retain their existing incomplete status.

The actual Opus 5.5 xhigh upstream-Nix recovery pass completed (203 turns,
3934.926s). Store deletion now retains classification evidence until deletion
finishes, records approved membership in bounded chunks, and holds a native
deletion lease independently of the worker. Unknown outcomes still require
explicit abandonment and a newly approved remaining plan. macOS boot/volume
evidence survives failed deletion; live or busy volumes cannot report ready.
Login-shell inspection errors propagate, and other accounts' Nix shells block
retirement. Its full race pass was 417.309s plus 23.926s terminal; subsequent
membership changes had focused coverage. The integrated legacy race suite
passes (4.194s), including the manager query-Git gate and strict account records.

Correction to the earlier F6 rejection: `--no-channel-add` does not prevent
the upstream daemon installer from creating root's default profile. Its
`nix-env` calls can create generations for Nix, its manual and CA certificates.
The initial name-only exemption is insufficient to distinguish a personal
package in those generations; a content-based correction is under review.
F7 remains rejected: released macOS configuration has `nix.enable = false`.

Linux account inventory previously skipped malformed records and overwrote
duplicate names, potentially hiding another Nix-shell consumer. A failed-before
regression now requires either condition to block inspection. The consolidated
legacy race suite passes. A separate recovery test initially failed because it
indexed the store gate position after the new query-Git gate was inserted; the
test now finds the gate by kind and validates its approval digest.

A real upstream 2.34.0 installation made by main's prerequisite helper was
interrupted during native store deletion. Its worker was killed after deletion
began; on this sudo build the child also terminated, so this does not prove a
surviving child. A separately held native flock on the same root demonstrably
blocked recovery. An added root member also blocked recovery and stayed intact.
After removing only that test sentinel and approving the observed remainder,
runtime migration and setup both reported ready, `/nix` was absent, and a real
account login succeeded. This was a disposable Debian container; native macOS
volume behavior and the final combined review remain pending.

### 2026-10-17 — Consolidated retirement candidate and native Linux closure

The actual Opus 5.5 xhigh Linuxbrew login follow-up hit the account quota after
implementing the correction. The parent integrated and independently verified it:
failed-before tests cover installed and already-removed formulae; both now repair
only the current local account to selected native zsh or system Bash. Other
accounts protect the old formula. The previously broken disposable account now
starts through a real `su --login`. This closes the earlier login finding; the
quota-ended implementation pass is not a completed review.

The Determinate pass also hit the quota after producing its implementation.
Integration validates typed vendor actions, native accounts/services/volumes,
and the root-owned installed uninstaller's matching version and bytes. The
preview discloses that installed-copy trust rather than claiming an independent
release digest. System Nix configuration is preserved before vendor removal;
receipt/identity evidence survives outside the store. A no-init receipt must
exclude no daemon and perform no systemctl query; the parent corrected that
native failure. The checked-in real 3.23.2 lifecycle then passed in a fresh
Debian container (14.28s), including outside-account refusal, build-account/group
removal, preserved personal system configuration and working login.

F6 is now fixed rather than rejected. The parent added bounded literal-manifest
validation for every root default-profile generation. Upstream Nix/manual/CA
store paths come from the checksum-verified 2.34.0 distributions for all three
supported POSIX targets. The vendor path uses a validated receipt and its
restricted runtime package set. Personal packages, another store output, Nix
expressions, an older personal generation and missing first-approval manifests
all failed before the correction and pass the refusal tests afterward. Durable
partial-deletion recovery requires saved content-validation proof; an older
membership record without that proof cannot authorize a missing manifest.
Actual nix-env serialization was checked in the disposable upstream fixture.
The combined Nix focused race suite passed (2.932s); all legacy focused race
checks passed (6.176s).

A fresh immutable v0.4.4 installation exposed an independent completion bug:
removing a retired native package changes the whole package inventory, even
when every selected replacement remains intact. The replacement prerequisite
fingerprint incorrectly included that inventory. A failed-before regression now
separates preservation identity from approval identity: full inventory still
changes the approved plan, while completed preservation checks bind selected
state, operations and health. The focused race test passes (1.023s). The first
native run's completed removals recovered through explicit abandonment and a
fresh preview; both final checks and a new login then passed.

A second fresh v0.4.4 Debian installation passed the ordinary complete migration
with the corrected binary: configuration handover, eleven selected replacements,
Linuxbrew zsh retirement with a working login, remaining old-runtime retirement,
and final ready results for migration and setup. Nix, chezmoi and the verified Pi
command are absent; personal Pi/Nix witnesses are unchanged. This proof used the
real released installer and native providers, not mocked installed state.

Windows native fixtures now exercise real Scoop metadata/uninstall, npm's actual
Pi shims and the released direct tree-sitter artifact. Existing native fixtures
cover Chocolatey, Winget, Homebrew and the Linux WezTerm DEB. The POSIX core and
fresh Debian jobs also run real Determinate retirement after upstream migration.
Windows and Darwin test binaries compile; those native lifecycles still require
hosted execution. Linux-only deletion fixtures require real util-linux flock and
GNU rm and therefore run only on Linux; Darwin volume fixtures remain separate.

The final independent actual Opus review is outstanding after the account-limit
reset. No implementation pass or earlier-head green check is final approval.

Further consolidation found a vendor-recovery ownership gap. After a real
Determinate uninstall, a new personal file at its former socket path was offered
for removal. The first preview was blocked only because the no-init container
lacked systemctl; installing that actual native tool exposed the faulty removal
plan. The corrected preview refuses it using original approved file fingerprints.
New approval evidence includes those snapshots; older evidence without them
remains readable but cannot remove a present file. Focused tests cover changed,
new and older-record cases. A separate failed-before regression showed that an
intact vendor unit was wrongly rejected by the upstream symlink recipe; proven
vendor remainder plans now keep their own service/account/boot recipe. Service
stop steps also recheck the approved file, and a loaded macOS service without
that file stays blocked. The focused Determinate race suite passes (1.561s).

The preceding consolidated local gate passed Go format, vet, full race
(375.166s; terminal 23.759s), 51 bootstrap tests (two environment skips), and the
available configuration/Neovim/Renovate checks. It failed only the unchanged PATH
test because the everyday host lacks system zsh; the same test passed in a
prepared disposable container. This gate precedes the last vendor recovery
correction, so the post-correction full gate and expanded native fixture are
running separately before candidate publication.

Correction to the preceding gate summary: the full log also contains three
EditorConfig failures in embedded macOS plist fixtures; the earlier summary
missed that static-check output. XML indentation is now tabs as required by the
Go-file policy, with semantic plist content unchanged. The checker is rerun
without an exclusion or suppression. The expanded fresh Determinate lifecycle
passes (21.24s), including the public refusal of a newly created service file.
The opt-in Winget lifecycle now fails when its required manager is unavailable,
rather than silently skipping native acceptance.

The post-vendor-correction full local Go race suite passes (375.069s); terminal
was cached from the unchanged preceding pass. The two other reported failures
were the now-corrected plist indentation and the unchanged host system-zsh
prerequisite. EditorConfig passes after correction, and focused Nix/Determinate
race coverage passes (3.123s). The native candidate is published as one commit
for the exact-head matrix; final Opus review remains queued for 12:00 UTC.

CodeQL on candidate `418c1c1` reported alerts #2 and #3 at the two rune conversions
in `legacyTOMLString` (`go/incorrect-integer-conversion`). Investigation rejects
the claimed high-severity vulnerability: ParseUint is bounded to 32 bits, and
all values above signed-rune range become negative and fail the existing
utf8.ValidRune check. Boundary tests through chezmoi TOML source discovery pass
before the change (1.019s), including both surrogate edges, the maximum Unicode
scalar, and 0x7fffffff/0x80000000/0xffffffff. The canonical correction makes
`code <= utf8.MaxRune` explicit before conversion, without suppressing CodeQL
or changing accepted values. The [TOML string specification](https://toml.io/en/v1.1.0#string)
requires scalar values. Other retirement ParseUint/cast sites were inspected;
these were the only such Unicode conversions. Hosted CodeQL must verify the
corrected commit before closure.

The first native candidate exposed three portability assumptions. macOS and
Windows temporary-directory aliases broke a Git lock test that had bypassed the
production physical-path resolver; that fixture now uses the same resolver.
Windows Git fixtures also lacked the actual Windows Terminal source read by
controller construction, so they now copy that source into their isolated
checkout. Neither correction relaxes a production ownership check.

Determinate receipt paths describe POSIX targets even when the receipt parser
runs on Windows. Parsing now uses POSIX absolute/clean/parent operations; live
filesystem inspection still uses native paths. The pure receipt tests remain
enabled on Windows. Only the root-file-ownership test is POSIX-only, matching the
unsupported Windows ownership implementation. A new path-boundary regression
rejects Windows drive syntax and noncanonical POSIX paths on every host.

The real Windows inventory also found that Chocolatey's official bootstrap
retains `lib/chocolatey/chocolatey.nupkg` without a loose nuspec. The former
reader blocked all retirement at that manager package. The corrected reader
accepts this exact bootstrap shape, reads bounded metadata without extracting or
executing it, and fingerprints the whole archive. Its dependencies still protect
outside consumers. Missing metadata for ordinary packages and ambiguous, invalid
or oversized bootstrap metadata remain errors. Sources: the
[runner bootstrap](https://github.com/actions/runner-images/blob/main/images/windows/scripts/build/Install-Chocolatey.ps1)
and [official Chocolatey bootstrap](https://community.chocolatey.org/install.ps1).
The inventory regression failed before correction with the native error. Focused
Git/Determinate/Chocolatey/chezmoi race tests now pass (9.462s), including a
read-only inventory proof using the actual official Chocolatey 2.7.4 archive.
Windows amd64 and Darwin arm64 test compilation passes; native reruns are still
required. Windows quality's failures match these same causes. The earlier
installer run was cancelled after diagnosis, so cancelled jobs provide no
acceptance evidence.

Candidate `4d7a64b` passes CodeQL and the complete native Go race steps on all
four targets, closing the preceding portability failures. Its corrected local
full race also passes (387.950s; terminal 23.865s); the sole local gate failure is
the unchanged system-zsh prerequisite. Hosted migration then exposed two distinct
fixture defects. On Linux amd64, the outside-account refusal succeeded, but its
preservation assertion tried the user-profile Nix executable even though the
released helper had installed daemon Nix in the system profile. The test now
binds the actual installed executable before the operation and checks that same
executable afterward, using the helper's system/user profile distinction.

On Linux arm64, the immutable release failed before configuration migration:
its optional VS Code installer chose Snap, whose stable Code package supports
amd64 only. On arm64 hosts with Snap and no Code CLI, the fixture now preinstalls
the real checksum-pinned Microsoft DEB and runs the released setup unchanged.
It also verifies this pre-existing
Code CLI and package files survive retirement. This is a released-installation
migration fixture with supported pre-existing Code, not proof that the old
release can freshly provision Code through Snap on arm64. Neither test skips
the released setup or hides its exit status. The following Determinate tests
failed because the interrupted fixtures had correctly left Nix installed; they
must rerun after successful upstream retirement. The focused Nix/Determinate
race suite passes (3.410s); both complete native reruns remain outstanding.

Both fresh Debian single-user jobs reached the same outside-account refusal,
then failed a second fixture assumption: a completed Linuxbrew zsh retirement
journal already existed. Refusal must create no new intent and leave previous
intent intact, rather than requiring an empty historical journal directory. The
test now fingerprints that directory before refusal and verifies it unchanged
afterward, including all prior entries. This preserves the no-mutation assertion
while allowing the preceding real login-shell retirement. The corrected complete
single-user fixture is also being run in a fresh disposable Debian container.

That corrected fresh run reached the final preservation assertions (369.64s):
Nix and the expected old artifacts were absent, both login-shell repairs worked,
outside-account refusal left existing journals unchanged, interrupted worker
recovery completed, personal witnesses survived, and replacement setup checked
ready. Its sole failure required a backup of `/etc/nix` even though the released
single-user helper never created that directory. The fixture now records the
actual system configuration: daemon retirement must preserve its full snapshot
at the reported backup; single-user retirement must leave it unchanged in place,
including when absent. A second fresh run verifies the corrected complete test.

All three quality jobs, native archive/agent/editor/desktop/provider steps and
CodeQL passed on `4d7a64b`. Two Windows manager fixtures then failed. The Winget
absence precheck populated the documented 20-second preview cache; after the
external fixture install, the test now obtains a fresh native inventory before
planning removal. The Scoop fixture omitted the Git executable that production
discovers in its constructor; its native driver now uses that same discovery.
Chocolatey removal, real npm Pi-shim retirement and direct tree-sitter retirement
passed on Windows. Matching native reruns remain required for the two corrections.

macOS's immutable released setup failed in Ghostty and AeroSpace cask installs,
before migration. The cause remains unverified because the fixture printed only
the final 8 KiB, after Homebrew's underlying errors. Failed fixture commands now
emit their complete already-bounded output through the existing terminal-control
filter, with CI tokens redacted before chunking. No package behavior or release
contents were changed based on that incomplete diagnostic. The next native run
must establish and resolve the cause; the following Determinate failure was the
expected refusal to install over the Nix store left by the failed setup.

The second fresh Debian run passes the complete corrected single-user native
fixture (368.39s), including outside-account refusal, unchanged prior journals,
both real login-shell transitions, interrupted worker recovery, removal of old
runtimes/artifacts, personal-data preservation and the final ready check. The
test correctly leaves absent system Nix configuration absent. The fixture and
manager-focused race suite and Windows test compilation also pass after the
latest test-driver corrections; final native reruns still govern acceptance.

The next full local race gate passes (396.066s; terminal cached), with the same
host-only system-zsh prerequisite failure elsewhere in `make ci`. The final
test-driver binding and diagnostic changes were checked afterward by the focused
race suite (3.361s) and Windows amd64, Darwin arm64 and Linux arm64 compilation.


### 2026-10-17 — Native manager and daemon follow-through

The isolated Windows manager suite passes at diagnostic head `1a5e7fe` (product
source `ae96c42`), run `39023994919`, job `117371260758`: Chocolatey, Winget,
Scoop, Pi npm shims, direct tree-sitter, native process mapping and read-only PATH
observation. The fixture now discovers Git as the production constructor does
and refreshes Winget inventory after an external installation. These are native
proofs; they do not substitute for final-head required checks.

The full released Linux daemon migration passes on both Ubuntu architectures at
`ae96c42`, including ARM's actual pre-existing Microsoft Code DEB. Fresh Debian
single-user jobs also pass. The subsequent Determinate service lifecycle exposes
a production defect: `determinate-nixd` supervises a separate `nix-daemon`, and
Sentry's crash reporter remains in that systemd service even after reparenting
to PID 1. The old MainPID-only exemption therefore blocks ordinary uninstallation.
Native failed-before proof is run `39023779796`, jobs `117370635385` and
`117370635487`; isolated run `39025021464`, job `117374208630`, confirms the actual
service tree. This is not transient installation readiness.

The correction stops only receipt-owned Nix sockets/daemon after replacement,
store, receipt, static-consumer and build-account checks. The irreversible vendor
call then requires a complete kernel consumer scan with no process exemptions.
Outside processes retain the store and recovery intent while the service remains
stopped. The vendor's v3.23.2 `ConfigureInitService::revert` and macOS
`retry_bootout` explicitly handle an already stopped service. Older saved gate
recipes remain readable. No process-name, ancestry or cgroup blanket exemption
was introduced.

Native corrected proof: diagnostic head `2480991`, run `39025317537`, Linux ARM
job `117375049792`, passes the full Determinate lifecycle. Its strengthened test
keeps a separate process's cwd in `/nix/store`, verifies that exact PID blocks the
vendor call, proves every receipt unit inactive and vendor receipt/binary
unchanged, then closes the process and resumes to complete removal. A preceding
fixture assertion expected the literal cwd path; the kernel scanner deliberately
reports `open path in /nix`. The corrected assertion binds the actual process ID
rather than weakening consumer protection. Ordinary focused retirement tests pass.

Mac diagnostic `39023915684` reproduced an immutable-release setup failure before
migration: v0.4.4's Homebrew 6.0.1 cannot evaluate current cask DSL. Historical
official taps are being verified against the unchanged release, with actual
Ghostty installed before nix-homebrew takes over. Native Mac migration and final
independent Opus review remain pending at this entry; no completion is claimed.


The strengthened vendor test also passes in a fresh Debian amd64 no-init
container (21.47s), with no USER/LOGNAME environment variables. Its first run had
completed uninstall and preservation but failed the fixture's login assertion:
`docker exec --user` does not synthesize USER. The fixture now uses the actual
account database via `user.Current`, matching production, for all five native
login/account checks. No native account behavior changed. Full local format,
vet and race checks pass (engine 381.255s, terminal 23.931s); `make -k ci` retains
only the previously documented host system-zsh prerequisite failure.

Historical Mac fixture diagnosis was refined: official July core/cask revisions
must remain paired. Combining July cask with current core creates an azure-cli
migration cycle (cask points to core, core points back to cask), proved directly
from both official `tap_migrations.json` files. Local Ruby formula evaluation also
exposes Resource::Patch DSL newer than Homebrew 6.0.1. The fixture therefore keeps
matching official July tap trees, limits NO_INSTALL_FROM_API to preparing Ghostty
under the current runner Brew, and leaves the released setup on its normal API
formula path. These are fixture inputs; the immutable release remains unchanged.
The existing native taps are retained and restored through the fixture lifecycle.
The full Mac migration remains under native validation.


### 2026-10-17 — preserve installed Homebrew records during Nix conversion

The matched historical tap fixture completes the immutable v0.4.4 setup on
macOS (run 39025865684, job 117376602169). Its subsequent old-Brew inventory
fails on a newer Firefox cask's `command_wrapper`. Homebrew 6.0.1's cask loader
also rescues unsupported definitions, so JSON inventory can silently omit a cask.
The pinned official Homebrew installer explicitly invokes `brew update --force
--quiet`, which can migrate package records and taps even when automatic update
is disabled. Reusing that installer violates conversion's preservation contract.

New conversion reads bounded installation records without evaluating old Ruby
and restores the official Homebrew 7.0.9 repository at
`cdc1ca503231b9a96da64526774915d913c3ec9f`. Every fetched source path must match
its reviewed root set; existing source is preserved beside the original, and
Git cannot overwrite ignored collisions. Cellar, Caskroom, installed links,
pins and taps are outside that source set. Only Homebrew's own Ruby bootstrap
runs; no global update or package migration runs. Before/after fingerprints bind
formula and cask records, version membership, links, pins and tracked/untracked
tap changes. Redirected taps require inspection instead of stranding Nix links.
The complete command is saved in the existing retirement intent, and an unknown
Git repository cannot be adopted as interrupted conversion work. Existing saved
homebrew-install and brew-inventory recipes remain readable.

Regression coverage exercises unsupported newer cask metadata without a working
Brew, manager-only changes, modified formula/cask records, empty new versions,
same-name untracked tap changes, redirected taps and refusal to alter an
unrecognized repository. The full released Mac migration is running on a
disposable native runner; this entry does not claim it has passed.

Homebrew repository recovery additionally binds the newly created Git directory's
native identity and exact local configuration before any retry invokes Git. A
changed local worktree, remote rewrite or credential setting cannot be trusted
through the original approval. A native Darwin process regression verifies that
changed configuration is refused before any fetch or checkout and leaves all
files intact. The stable tag is fetched explicitly and its dereferenced commit
must match the reviewed pin; no mutable tag can select different source.


### 2026-10-17 — native Mac conversion passes; final retirement diagnosis

Diagnostic run 39027060354 / job 117380036152 executes the complete immutable
v0.4.4 setup, migrates profiles, restores Homebrew's repository with unchanged
package/tap fingerprints, and installs the selected native replacements. The
subsequent full retirement does not converge within the fixture's eight rounds
(902.12s overall). The old helper discarded remaining-item diagnostics; each
check/apply round now reports its bounded status/message so the native cause can
be established. No removal guard or acceptance requirement has been relaxed.

The fixture's cleanup also incorrectly required the released AeroSpace tap to
remain after its approved retirement. It now permits that one tap to be absent
at cleanup while restoring the runner's original tap tree. The separate
conversion fingerprint still requires every tap unchanged before replacement
setup/retirement. Core and cask snapshots must remain at their exact revisions.
The refreshed diagnostic uses the current production source restoration and
recovery guards; the full final-head matrix remains a delivery requirement.

Reviewing the pinned Homebrew 7.0.9 `brew.sh` found that invoking the developer
command `brew ruby` persists `homebrew.devcmdrun=true`, changing subsequent
update behavior and invalidating the original recovery configuration hash.
Conversion now uses the public `brew setup-ruby` command, which provisions the
same verified private runtime before developer-command handling. The native
conversion fixture asserts developer mode stays disabled. Source of truth:
[the pinned dispatcher](https://github.com/Homebrew/brew/blob/cdc1ca503231b9a96da64526774915d913c3ec9f/Library/Homebrew/brew.sh)
and [runtime bootstrap](https://github.com/Homebrew/brew/blob/cdc1ca503231b9a96da64526774915d913c3ec9f/Library/Homebrew/cmd/setup-ruby.sh).

A real temporary Git repository reproduced another read-only boundary issue:
status and diff can execute a tap's clean filter even with external diff,
text conversion, hooks and fsmonitor disabled. The new tap inspection refuses
configured clean/process filters (including included local configuration) and
submodules before worktree inspection. A failed-before process regression and
a passing correction require the personal command witness to remain absent.
Normal tracked/untracked tap fingerprints remain strict.

Run 39028280324 / job 117383501463 identifies the remaining Mac blocker as
the upstream `_nixbld1` account shape. Homebrew conversion and selected setup
complete; automatic fontconfig is correctly retained without blocking readiness.
The fixture now records only the build account's relevant non-secret attributes
so they can be compared with the pinned upstream and nix-darwin sources. Account
deletion checks have not been relaxed pending that evidence.

The account refusal is reproduced through the host process boundary using
`dsAttrTypeNative:IsHidden: 1`, the native spelling also captured in
[the vendor's account diagnostic](https://github.com/DeterminateSystems/nix-installer/issues/33).
The text parser previously split at the first colon, losing IsHidden. It now
retains qualified attribute names and reads the native hidden flag explicitly,
with the older short-name representation still readable. Account UID, GID,
name, description, home, shell and hidden requirements remain unchanged.
The failed-before regression also provides a native lookalike UniqueID and
requires it not to replace the standard account ID. Focused legacy race tests
pass after the parser correction; the native rerun remains required.

The read-only Git boundary also refuses partial-clone configuration before
status/diff: a real temporary local-origin repository with a missing promised
blob reproduced a successful diff that wrote four new Git object files through
automatic lazy fetching. Inspection must not fetch or invoke a configured
remote helper. The focused regression requires an explicit refusal and unchanged
repository files; ordinary complete and shallow tap repositories remain usable.
The focused Homebrew race tests pass (1.140s).

### Native Mac closure and inaccessible automatic roots (2026-10-17)

Diagnostic `08f2323`, run `39029433400` / job `117386741746`, passes the
complete released v0.4.4 Mac migration, replacement health, Nix APFS and system
integration removal, and published AeroSpace launch. The subsequent real
Determinate 3.23.2 lifecycle fails safely during preview: lstat of its stale
`profile-1-link` root returns EACCES below root's private temporary directory.
The vendor's `src/profile/nixenv/mod.rs` creates that scratch profile using
`tempfile::tempdir()` and removes the directory when the operation returns.
Its surviving automatic GC root does not prove a live outside consumer.

The correction rechecks only permission-denied root targets using the same
installed executable's private read-only lstat entrypoint and existing nonprompting
administrator credentials. Only ENOENT returns absent; dangling links return
present, and malformed paths, ENOTDIR, symlink loops, missing credentials, other
errors and invalid helper responses remain failures. The ordinary outside-root
check still runs at both preview and the irreversible removal gate. No GC root
is deleted, ignored by name or exempted by temporary-directory prefix.

The public executable regression fails before the helper exists and passes after
the correction. The real permission-boundary fixture separately verifies stale,
live, failed-authentication and invalid-reply outcomes. Focused race tests pass
(4.243s); native Determinate macOS rerun remains required.

### Ordinary Mac vendor ownership and verified private execution (2026-10-17)

Native diagnostic `994a6ed`, run `39030949968` / job `117391107195`,
confirms /nix/nix-installer retains the installing account's uid 501 and mode
0755, while /nix is root-owned 0755 and determinate-nixd is root-owned 0555.
The installed bytes match the independent official v3.23.2 release pin. The
vendor's ordinary shell downloads as the calling user before sudo, and its
copy_self_to_nix_dir uses Rust file copy followed by chmod. On macOS that copy
preserves the source owner. Making the fixture source root-owned would hide
the real compatibility failure; the fixture remains faithful.

Actual Claude Opus 5.5 xhigh consultation completed successfully (34 turns;
model metadata claude-opus-5-5). It recommends retaining protected-root trust,
adding independent exact release authentication and staging both inputs as root.
The owner already authorized canonical migration support; covering the existing
reviewed receipt range needs no new feature approval. The generated table covers
all 72 published Apple Silicon assets from 3.0.0 through 3.23.3. Version 3.19.0
published no matching asset. Sixty hashes come from official GitHub asset metadata;
the twelve older assets lacking digest metadata are downloaded and hashed, with
exact published byte lengths checked. Generator --check reproduces the table
(122.466s). These entries prove byte authentication, not 72 native lifecycles.

A protected root-owned binary retains the existing trust route. Otherwise only
the exact version/target release size and digest authenticate the installed bytes.
The /nix parent, plain-file and size restrictions remain. A writable source is
never executed during preview, including --version: the independent version pin
is its identity. This tightens Opus's proposed version probe, which would reopen
a writable executable after hashing. Root copies both inputs into a private
0700 directory under its sticky staging parent, bounds the reads, hashes the
copies and execs that copy directly. This bypasses the vendor's additional read
of the original path and keeps the native worker attached to the actual vendor
process. Retained private temporary copies are disclosed in the preview.

The failed-before staging regression executed the original paths. Passing
regressions now prove private execution, receipt preservation even when originals
change, strict mismatch/link/non-sticky-parent refusals, no stage leftovers on
refusal, unchanged nonzero-outcome recovery, and saved literal command readability.
Optional owner/trust identity fields leave the old protected-root gate hash
unchanged; a release-authenticated source cannot inherit that old approval.
Focused race suites pass, including the real source permission boundary.

The next native diagnostic `80e0a63`, run `39032285084`, passes release identity
and a real appended-byte refusal, then safely stops at /etc/synthetic.conf: the
vendor creates a new file with root-only 0600 permissions when its mode is unset
(CreateOrInsertIntoFile::execute and CreateDeterminateNixVolume::plan). A bounded
read-only helper now supplies consistent byte/mode snapshots after permission
denial. Links, directories, oversized data, malformed replies, changed approvals
and missing credentials stay errors. Privileged before/after checks use the same
helper for partial-uninstall recovery; file permissions stay untouched. Preview
requires existing administrator credentials and reports sudo -v when unavailable.
Native macOS rerun and the final exact-head matrix remain required.

Diagnostic `105ecbe`, run `39032877740` / job `117396732823`, passes the
private-file inspection and reaches daemon shutdown. The immediate postcondition
sees launchd still loaded; a fresh command 150ms later reports service absent.
There is no query cache: each state query starts launchctl. Completion checks
now wait up to 30 seconds for observed absence, honoring cancellation and
propagating inspection errors immediately. Pre-removal approval gates do not
wait for unapproved changes. This covers the other launchd stop paths too.
Focused regressions prove delayed unregistration, deadline/failure refusal and
immediate precondition checks; all focused migration race tests pass (8.627s).

A fresh supplemental Debian container initially refused deletion because Docker's
default capability set prevents root from reading /proc/<sudo>/exe. An independent
sudo/readlink reproduction confirms permission denied on its own live sudo parent.
The container is rebuilt with SYS_PTRACE and an init reaper so the native complete
process scan has normal administrator visibility. The production guard is
unchanged; incomplete process inspection still refuses deletion.

The fresh capable Debian amd64 runner passes the complete real Determinate
3.23.2 no-init lifecycle with the new private staging command (20.69s). The
outside process blocks deletion, the same journal resumes after it closes,
personal system configuration remains, vendor files/accounts are removed,
and a later personal service file is refused. The disposable container was
removed after completion; no developer-host packages or services were changed.

Diagnostic `9f6875e`, run `39033197282` / job `117397722308`, now completes
service shutdown, outside-process refusal/recovery and the vendor uninstall.
Final generic Mac cleanup still reads /etc/fstab directly, which the vendor
leaves root-private. A focused planner regression reproduces permission denied.
The common fstab parser, synthetic.conf inspection and exact-line edit planning
now use the same bounded administrator reader; no chmod or guard exemption.
The regression preserves personal bytes and mode, including a remaining synthetic
entry after vendor removal. The native lifecycle rerun remains required.

Diagnostic `0651b65`, run `39033805907` / job `117399485817`, passes the
complete real Determinate 3.23.2 Mac lifecycle (180.73s). The unchanged native
assertions prove release-byte tamper refusal, outside-process refusal with the
store intact, same-journal recovery, vendor/account/store removal, preserved
personal system configuration, final setup/migrate readiness, and refusal of a
new personal service at an old vendor path. Focused migration race tests pass
(8.635s). This closes the diagnostic Mac lifecycle; final exact-head CI and
independent Opus approval are recorded on PR #87 before delivery.

## Independent Opus review of 0a766f5 (2026-10-17)

Actual Claude Opus 5.5 xhigh completed the independent review in 71 turns
(946.978s; model metadata claude-opus-5-5), outcome CHANGES_REQUESTED. Its
focused offline race suite passed (22.06s). It accepted the selector, scoped
Git defaults, process/ownership checks, private vendor staging and Mac fixes,
and reported these two findings:

- Medium, legacy_nix.go:1108 and legacy_nix_determinate.go:2124: unconditional
  systemctl stop fails with exit 5 on unloaded receipt units, stranding a fresh
  recovery after partial deletion and daemon-reload. Accepted.
- Low, legacy_homebrew_repository.go:165 and legacy_runtime_inventory.go:492:
  --local misses executable worktree filters, and plugin status had no guard.
  Accepted; real Git regressions reproduce all three affected cases.

The Git fix shares effective-config and submodule checks across tap and plugin
inspection, and defers plugin status until HEAD and origin match. A further
real tree-filtered clone reproduces automatic fetch during log --find-object;
history evidence now refuses that partial-clone configuration too. The rules
follow Git's documented local/worktree distinction:
https://git-scm.com/docs/git-config. No filter executes and repository snapshots
remain unchanged in the passing regressions.

Systemd's stop contract explicitly fails for nonexistent units:
https://github.com/systemd/systemd/blob/main/man/systemctl.xml. Failed-before
process-boundary tests reproduce exit 5 for all-absent and missing-socket cases.
The new saved stop recipe skips observed inactive units, stops sockets before
services, and accepts exit 5 only after a fresh inactive observation. Every unit
has an explicit completion check; inspection errors, unknown states, other
failures and still-active units refuse completion. Upstream, vendor and vendor
leftover stops share this recipe; final kernel consumer checks are unchanged.
Focused migration race tests pass (9.229s). Real systemd partial-vendor testing
passes the stop/uninstall path but exposed remaining integration steps after
the vendor ran; this follow-up and final independent re-review remain open.

The real ARM systemd diagnostic `63afc25`, run `39035969437`, passed stopping
the remaining daemon and vendor uninstall with a genuinely unloaded socket.
Diagnostic `d71a359`, run `39036103431`, identifies the remaining artifact:
/etc/systemd/system/sockets.target.wants/nix-daemon.socket. Vendor disable leaves
this link when its unit file is already gone; generic cleanup then discovers
unapproved work, correctly refusing readiness.

The receipt's socket activation links are now recorded in the observed and saved
file evidence, including absence. Only exact links to its own declared unit path
qualify. Planned follow-ups remove unchanged links and reload systemd after the
vendor; later recovery requires the same saved snapshot. Legacy evidence without
the new proof, previously absent links and changed personal links cannot authorize
removal. This intentionally changes Linux approval identity and requires a fresh
preview; it does not rewrite saved commands or widen historical evidence.
Focused migration race tests pass (7.931s), including missing-unit dispatch,
disappearance races, strict command/query failures, actual interrupted-store
recovery after reload, saved-proof boundaries and personal-link protection.
The real native missing-socket lifecycle is being rerun with these follow-ups.

Diagnostic `116f74c`, run `39036444754` / job `117407184066`, passes the
complete real ARM systemd lifecycle with the removed/reloaded socket (28.56s).
The public preview accepts the verified absent socket, another live process
blocks vendor execution, the same journal resumes, all recorded socket links
are absent, and personal configuration plus later personal-service protection
remain intact. This closes the native follow-up to the accepted Medium finding.
Both accepted findings have focused regressions and native/process-boundary proof;
independent Opus re-review and the corrected exact-head matrix remain gates.

While the corrected-head review runs, consolidation found a compatibility issue
in the shared submodule refusal: the released tmux-resurrect commit cff343cf
contains lib/tmux-test as mode160000 (GitHub tree API proof), and the immutable
install-deps.sh fetch/checkout does not initialize it. A failed-before real-Git
fixture shows the clean released shape being unnecessarily preserved. Plugin
inspection now permits an empty uninitialized worktree only when the common and
worktree Git module caches are also empty/absent. Taps remain strict. Populated
and cached/deinitialized submodule data stays protected without running child Git.
The initial regression assumed a wholly absent gitlink worktree was clean; this
is rejected by actual git status, which reports D. The corrected expectation
explicitly checks that evidence and preserves the modified checkout. No existing
check or personal-data preservation requirement is weakened.

The submodule compatibility correction passes all focused migration race tests
(7.990s), including empty, deleted, populated and cached worktree shapes, with
unchanged repository snapshots. The preceding corrected full make-ci run passed
Go format/vet and full engine/terminal races (376.743s/23.930s), bootstrap51 and
configuration checks except the documented host system-zsh prerequisite. The
full gate is repeated for this final compatibility delta; final-head CI and
independent approval remain required.


### 2026-10-17 — second Opus review: failed units and service activation links

Actual Claude Opus 5.5 xhigh reviewed immutable d5f487728a61a81dc5b4ef1593697b7b45770036
(65 turns, 935125ms) and returned CHANGES_REQUESTED. The earlier Git/filter and
missing-unit findings are resolved. Accepted Medium: systemd stop leaves an
already failed service in failed state, so the new inactive-only completion
check stranded dispatch and recovery. Accepted Low: vendor enable creates
multi-user.target.wants/nix-daemon.service as well as socket links, and partial
revert can leave it dangling. The existing socket-only inventory omitted it.

Failed-before process-boundary tests reproduce exit121 for failed-unit dispatch
and recovery, and omission of the service link with recorded, absent and
historical proof. New stop recipes skip inactive/failed units and require a
fresh stopped observation after exit0/5; non5 command errors, query errors and
unknown states remain fatal. Historical inactive-only checks and command literals
retain their semantics; a fresh preview is required to adopt the new recipe.
The full kernel consumer scan is unchanged and is still the deletion authority.

Receipt-derived socket and service links now share snapshot, accepted-target,
absence and changed-personal-link rules. Upstream remainder inspection also
recognizes the exact service wants link, closing the parallel path. The native
systemd fixture kills the real daemon, verifies failed state, removes the service
and socket unit files while preserving their actual enablement links, then
requires outside-consumer refusal, same-journal recovery and absence of every
approved link. This runs only on explicitly opted-in disposable runners.

The review also noted that the interrupted-deletion host fixture models only
already-stopped recovery; the separate process-boundary tests execute the actual
shell recipe. This is a verification limit, not evidence that native exit5 was
executed by that host double. Native missing-unit coverage is recorded above.

Focused migration race coverage passes (18.110s engine / 9.275s terminal).
Diagnostic e71073c / run39038077571 proves the native failed/not-found unit
state and passes the corrected stop, but its fixture killed only the service's
main PID. Determinate uses KillMode=process: real nix/crashpad children survived
and the unchanged kernel guard correctly blocked vendor execution. This is a
fixture fault, not authorization to exempt those consumers. The fixture now
kills the entire exact daemon cgroup before asserting failed state; its separate
outside-process refusal witness remains unchanged.

The corrected disposable ARM systemd diagnostic 11d1ead / run39038168571 /
job117412104045 passes the complete public vendor retirement, including native
failed/not-found units, retained service/socket enablement links, outside-consumer
refusal and same-journal completion. Every approved activation link is absent;
personal configuration and later personal-service protection remain intact.
Final-head checks and independent Opus approval remain the delivery gates.


### 2026-10-17 — resolved unit targets from systemd enable

Actual Opus 5.5 xhigh reviewed 584c07e829ff39ebef411f69a9c9505d5abc6f3b
(70 turns, 812585ms). Failed-unit handling and released-plugin submodule
compatibility were accepted. One Medium remains: modern systemd resolves
service enablement by /etc path into the store for the vendor's upstream Nix
distribution. The static /etc/profile target rule rejects that real layout.
The reviewer reproduced it with offline systemctl --root, production inspection
and the official upstream receipt; its earlier recommendation was incomplete.

Failed-before receipt tests reproduce refusals for resolved service/socket links.
The shared rule now also accepts the exact fully resolved regular file with the
same unit name inside /nix/store, observed through the installation's declared
unit/profile path. Unrelated store files, outside targets, directories, missing
sources and resolution errors remain refusals. Existing literal targets retain
their historical semantics. Exact snapshots and old-evidence restrictions remain.

Upstream plans retire store-resolved activation links after stopping units and
before deleting their source; primary unit links remain classification evidence
until afterwards. The later integration plan does not duplicate those steps.
A recorded interrupted deletion then converges without losing target evidence.
The first new host-fixture test incorrectly entered root-owned daemon discovery
from a non-root private directory. The corrected test enters the real daemon
planner directly, executes its pre-deletion steps and an interrupted deletion,
then uses ordinary discovery for recovery. Native CI proves root-owned discovery.
This correction is a fixture diagnosis, not a weakened ownership check.

The native fixture now separately installs/removes both Determinate and upstream
Nix through the pinned vendor binary. The upstream case asserts the actual
store-resolved service link before public preview, then requires outside-consumer
refusal, recovery, all links absent and personal-data protection. Existing full
CI lanes include both distributions under their original timeouts. Final native
results and independent re-review remain required.

Focused migration race coverage passes (17.354s engine / 9.255s terminal).
The combined native diagnostic 1bb0765 / run39039662750 passes the ordinary
vendor lifecycle (22.12s), but the second distribution correctly refuses its
nonempty /etc/nix. A follow-up 829f550 / run39039825283 identifies the surviving
entry as vendor-generated sentry-endpoint; the empty-directory assumption was
wrong. Production correctly preserves it. The fixture now moves its entire
remaining configuration intact into a unique private directory under /etc, only
after proving /etc/nix was absent before this fixture and after every production
removal/preservation assertion. It neither deletes those bytes nor relaxes the
next installation's precondition. Disposable runner teardown owns that copy.

Diagnostic 7bcb389 / run39040018722 / job117417398845 passes both actual
vendor distributions on ARM systemd, including the upstream store-resolved
service link, public preview, outside-consumer refusal, same-journal recovery
and complete link removal. Production source is unchanged from that diagnostic.
The final fixture gives each distribution a distinct personal-configuration
witness so a previous run's backup cannot satisfy the next preservation check.
The exact final CI run covers that strengthened assertion.


### 2026-10-17 — close the three Low findings after Opus approval

Actual Opus 5.5 xhigh approved immutable 9584a39c1289ad75c2c9a4697775bb83cb7e263d
(57 turns, 886973ms), resolving the earlier Medium with no material regression.
Three nonblocking Low findings are nevertheless addressed in this task:

- Recovery could accept a re-enabled resolved link, then delete its source before
  removing the link. A second interruption stranded the next recovery. The exact
  early-removal loop is now shared by fresh and recovery planners. A failed-before
  process-boundary test records a first interrupted deletion that leaves the unit
  available, re-enables its link, executes the next approved deletion until another
  lost worker reply, and verifies subsequent ordinary recovery converges.
- An upgraded profile or a store removed outside the plan leaves an unproved
  link. Conservative refusal is the chosen ownership policy; it gains actionable,
  ownership-qualified advice for removing only the exact named activation link.
  Both shapes reproduce the earlier unhelpful refusal; snapshots stay unchanged.
- Regular files/directories at wants paths regain the explicit preserve-and-inspect
  explanation, while retaining the underlying readlink error. Both shapes have
  failed-before coverage.

All three initial regressions fail for their reported behavior, then the full
focused migration race suite passes (18.215s / 9.277s). A real offline systemctl
--root probe rejects the review's suggested disable command as advice here: it
removes both the primary /etc unit link and activation link (exit0 for an upgraded
fixture, exit1 but removal for a dangling fixture). That can remove classification
evidence. The displayed instruction instead uses sudo rm -- on only the confirmed
activation link, matching the existing exact unlink operation without granting
automatic authority over an unproved target. No live host service was changed.

The final focused migration race suite passes (18.464s engine / 9.261s terminal),
including the exact-link advice correction. Full local verification, final-head
hosted checks and the last independent review are tracked on PR #87.


### 2026-10-17 — constrain manual advice to activation paths

Actual Opus 5.5 xhigh approved d27c9274fcfd7d2052238abd88253f6e9f4c4070
(45 turns, 566282ms), closing all three previous Low findings. One new Low
identified that the shared advice also reached primary service/socket/tmpfiles
links. That contradicts the activation-only instruction and could remove
classification evidence if followed. A three-case preview regression reproduces
the advice for each primary path before correction. An exact parent-directory
guard now limits unlink advice to /etc/systemd/system/sockets.target.wants and
multi-user.target.wants. Primary paths keep only preserve-and-inspect text.
No link acceptance, automatic mutation or saved operation behavior changes.

The final focused migration race suite passes (18.809s engine / 9.257s terminal),
including positive activation advice and all three primary-path refusals.


### 2026-10-17 — assert personal-service preservation across no-init routes

Actual Opus 5.5 xhigh approved 3beb0545843712f042f4365753af630125b6f836
(30 turns, 231114ms), resolving the prior advice-scope Low with no material
findings. Exact-head native CI then failed both Debian architectures at
legacy_nix_determinate_native_test.go:300 (run39042644072, jobs117425125722
and117425125826). The returned operation was Pending and explicitly preserved
the new personal /etc/systemd/system/nix-daemon.service. The test incorrectly
required the saved-vendor-file guard's exact phrase. A no-init upstream receipt
never owned that service, so the generic integration guard correctly refused it
instead. The reported removal bug is rejected as a false alarm; the assertion
was wrong, with both native traces retained as failed-before evidence.

The fixture now checks the behavioral contract for both routes: preview returns
Pending, names the exact personal path and explicitly says to preserve it;
application of that same plan returns needs-action naming the path. The personal
file's checksum must remain unchanged after each boundary. No production guard,
timeout or removal assertion changes. The complete native matrix and independent
review are required for the corrected candidate and recorded on PR #87.

The first strengthened no-init diagnostic passed the ordinary vendor's refusal
and preservation checks, then the next variant correctly refused to begin with
that test's unfinished transaction. Fixture cleanup now removes only its own
personal witness and explicitly retries the same transaction, requiring Ready
before the next distribution. This exercises recovery instead of abandoning or
discarding journal state.

The corrected fixture passes both real vendor distributions in a fresh disposable
Debian amd64 container without an init system (20.52s Determinate, 14.34s
upstream). This proves the new preservation and retry assertions through the
public command boundary. Both hosted Debian architectures and the systemd/macOS
routes remain part of the required final-head matrix.


### 2026-10-17 — observe committed journals during native interruption tests

Opus 5.5 xhigh approved 7fd4e6d with no material findings, then approved the
comment-only correction at 14ee10d0f36391c3700a27f549df7d6f22079610 without
remaining findings. That candidate's native Debian ARM job117433863493 in
run39045467927 failed at the interruption observer: readDocument tried to stat
a .state-* file that saveDocument had atomically renamed. The observer scanned
every directory entry; production recovery already filters for a valid
operation ID followed by .json. The production publication race is rejected as
a false alarm; the native fixture's inventory rule was wrong.

Two deterministic failed-before cases expose both consequences: incomplete
staging JSON raises unexpected EOF, while a complete but unpublished staging
intent with an existing worker record falsely reports committed dispatch.
The observer now uses production recovery's filename contract before reading.
Both staging shapes must remain unobserved; publishing the same intent must
make its recorded dispatch observable. A malformed committed journal must still
return an error. No read errors are suppressed and no deadlines or production
behavior change. The full focused TestLegacy race suite passes in 8.496s.
Final native checks and independent review are recorded on PR #87.

An additional real concurrent-publication probe then exposed the second
interleaving: readDocument correctly rejects a committed file replaced between
lstat and open with its explicit retry-discovery result. The native observer
now follows the existing profile-crash and Sentinel interruption-test pattern:
poll again within the original deadline for that diagnostic only. An exact
message match preserves joined close errors and every other read failure.
This changes no production reader or persistence behavior. A committed-journal
publication test reproduces the failure before correction; staging, real
dispatch and malformed-journal assertions remain required.
The complete focused TestLegacy race suite passes with both observer corrections
(8.224s). The interim c3c1b8c review was superseded before a verdict so the final
independent review covers both interleavings together.

Actual Opus 5.5 xhigh approved c54a62019afe50d9365060b851bc721902a75e7b
(49 turns, 508423ms) with no material findings. It independently reproduced
both races and passed 50 consecutive race runs. Its one Low coverage gap is
closed by a committed-name directory case: a non-file journal must fail during
read, independently of the existing malformed-JSON decode assertion. The
reviewer's mutation that swallowed every read error passed the preceding tests
but failed this case. Observer and production behavior are unchanged.
The parent independently repeated that mutation: the new case fails when read
errors are swallowed, while all four observer race tests pass at the corrected
source (1.154s). The mutation existed only in a private disposable checkout.

Opus 5.5 xhigh approved 697d5e30308d0a84ac47f9bad8400f306f2ac310 without
remaining findings. Native CI passed 26 of 27 checks, including every actual
migration lifecycle: the Mac released-setup and Determinate lifecycles passed
in job117442485680/run39048634921. Its earlier Go suite failed only the two new
staging-test cases because the fixture's /bin/true command exited 127 on macOS.
The harmless command now follows the existing /bin/sh -c 'exit 0' test pattern
on both POSIX targets. It still executes through the real worker, and a failure
now includes that worker's diagnostic. No observer or production behavior changes.
All four observer race tests pass locally with the portable command (1.159s).


### 2026-10-17 — complete Windows job snapshots during natural exits

Opus 5.5 xhigh approved 08259a6c55c720d993196d7330aaea8687bf5ba6 with no
material findings; its Mac quality check passed the portable worker correction.
Windows Build Tools job117451054539/run39051568247 then refused a successful
but incomplete private-job PID-list response after installation. This differs
from the earlier initial-query timeout. The original log had no counts, so its
precise interleaving remains unverified. A separate unchanged-guard lifecycle
at 7ce038b/run39052580626 passed 50 consecutive installed observations.

Actual Opus 5.5 xhigh consultation (27 turns, 605219ms) recommended preserving
the refusal until native evidence distinguished overflow, API failure and natural
exit. Diagnostic a01a263/run39053249948 then created 16 known private processes
and observed their ordinary exits across 12 rounds. Windows returned 558 short
successful lists on the host and 575 in the same pinned Server Core container;
counts stayed below 32 and the lifetime Total remained 32. For example, assigned
32/listed31 became a complete 2/2 snapshot without any added process. This proves
the one-shot equality guard rejects legitimate natural exits; it does not prove
the exact unlogged counts in the original compiler failure.

Snapshot collection now re-queries only a successful short list within the
original shared five-second reap deadline. The 256-member capacity, original
lifetime Total, private-job-only scope, exact HANDLE verification/waits and
30-second inspection limit remain unchanged. API errors, overflow, reversed
counts, invalid PIDs and HANDLE failures remain immediate errors. A complete
response arriving after the original deadline also fails. No Observe, mutation
or approved plan is retried. This clarifies the guide's incomplete-snapshot rule:
only a complete snapshot can authorize success, and added lifetime membership
still invalidates its completion proof.

Regression coverage exercises the native syscall during actual exit churn and
controlled API-boundary results for short-to-complete, zero/one/256 members,
overflow, invalid counts, API failures and the unchanged deadline. The actual
before/after native result, final full gate and independent review are recorded
on PR #87; earlier green candidates do not certify this change.

Diagnostic a50ef39/run39053654003 reintroduced the previous one-shot guard only
in a disposable test binary: all 12 native exit rounds failed with the original
capture refusal (1.56s). The corrected binary passes the same regression on the
host (34 incomplete snapshots recovered, 1.64s) and pinned container (42, 1.81s).
Both environments also pass every existing process-tree mode, late-member
refusal, natural-exit, ABI and new API-boundary/deadline test. The final error
text describes capture deadline expiry even when its last query was complete;
this wording does not change the proved acceptance rules.


### 2026-10-17 — distinguish process exit from an exited thread-group leader

Opus 5.5 xhigh approved 8e54f508135eefec69b1fccfd5a903175f621eb1 (39 turns,
876747ms) with no material findings. Its exact-head Windows Build Tools lifecycle
passed (job117457766236/run39053851196, 621.22s). Clarifying the preceding
Windows evidence: assigned counts were at most 32; the 34/42 counts describe
short lists observed by the test before successful complete collection, not a
measured count of retries inside the helper. The unchanged per-PID exit window
remains fail-closed and unobserved; it was not broadened without evidence.

The ARM Linux core job117457766278/run39053851196 subsequently refused the
upstream-flavoured Determinate recovery at legacy_nix_determinate_native_test.go:247.
After the intentional outside consumer was stopped and reaped, the kernel gate
returned exit85: cannot inspect open path of live process423. The original log
did not capture that process's state or reference; its exact interleaving remains
unverified. This refusal prevented vendor removal, rather than bypassing a gate.

A controlled real-process regression reads a child's executable, then terminates
it and retains its unreaped kernel directory before the remaining reads. The old
scan fails at descriptor enumeration as an ordinary user (exit84), and reproduces
the original open-path exit85 inside a disposable root Debian container. The
same root-container test passes after correction. A second native C/pthread
fixture exits its main thread while retaining a live worker. The kernel reports
State Z and Threads2; the old shortcut incorrectly returned a clean scan. It now
refuses inspection. This is why accepting State Z alone at additional boundaries
would be unsafe. The Linux man-pages for proc_pid_exe/cwd/root document that an
exited main thread can make those references unavailable while workers remain.

One shared check now handles process disappearance, explicit kernel-thread proof,
or Z/X with exactly one remaining thread after every failed procfs read. Maps,
fd-directory and cwd/root/descriptor failures use the same rule as executable
failure. Unknown thread counts and live/unreadable process state remain errors;
consumer matching, restricted-proc refusal and removal approval remain unchanged.
Failure diagnostics identify the bounded reference name. Underlying command
errors are replaced by the existing explicit refusal unless kernel state proves
the process has exited; they cannot become a clean inventory without that proof.
This changes a live read-only gate, not a persisted native command recipe.

Native tests and controlled filesystem/process-boundary cases cover each read
stage, zombie/dead/live/multithreaded/unknown states and existing Nix consumers.
The first fd-directory fixture used a dangling symlink, which ls can enumerate
successfully; it was corrected to an absent directory to represent a failed read.
That fixture-model error is not a production finding. The corrected focused
process race suite passes in 1.399s; all TestLegacy race cases pass in 8.869s.
The actual C fixture requires the existing contributor C compiler with pthreads,
now documented explicitly. Hosted vendor lifecycles, full gate and independent
review of the final commit remain delivery requirements.

Diagnostic e6efcb9/run39056128142 passes the process regressions and both actual
vendor distributions on fresh hosted Linux runners. Determinate/upstream timings
are 42.88s/21.26s on amd64 and 31.08s/17.80s on ARM. Both runs retain outside
consumer refusal, same-journal recovery, exact removal and personal-file
preservation assertions. The complete final-head matrix remains separate proof.


Actual Opus 5.5 xhigh consultation (44 turns, 1002689ms) independently reproduced
two causes of the exit85 message: a newly unreaped zombie and a live process
closing/reusing the same descriptor number. It also reproduced the exited-leader
omission and identified its mid-scan counterpart. No exact cause is attributed
to the original PID423. Its kernel-source inspection shows Threads can become
zero after release; acceptance therefore requires Z/X with an explicit zero or
one, rather than exactly one. State is printed before Threads, and a zombie
leader cannot add threads; release requires an empty group. PF_EXITING is 0x4
in the published Linux v7.0 sched.h. It only narrows the closed-descriptor
inference; it never authorizes omission of a live process.

The consolidated correction re-reads a still-present descriptor once after
readlink fails, and classifies its new target normally. If it remains unreadable,
removal still fails. A descriptor that disappeared is considered closed only
with live state and a valid stat record without PF_EXITING; otherwise the shared
exit check must prove the process finished. Failure diagnostics include the
reference and bounded State/Threads fields, without argv or environment data.
This does not retry an approved operation or scan by process name.

New real C-process tests reproduce descriptor reuse and main-thread exit during
the descriptor stage against the initial correction: reuse returns exit85;
mid-scan leader exit incorrectly returns a clean inventory. Both now pass.
Controlled query-boundary tests require a reopened /nix descriptor to remain a
consumer, exactly two readlink attempts, persistent failure with state diagnostics,
and refusal for unknown/exiting leader flags. Zero-thread released states also
have failed-before coverage at every read boundary. All focused process race
cases pass in 1.960s. The initial correction's full gate passed every section
except the known local system-zsh prerequisite (engine race 380.044s), but it is
superseded by this expanded correction and cannot certify its final source.
The complete gate, both native architectures and exact-head independent review
are required again.


### 2026-10-17 — independent review rejects de_thread handover omissions

Actual Opus 5.5 xhigh reviewed 3082f6b9cd3e4c604a1cd16320460455439629ca
(50 turns, 1219835ms) and requested changes despite the preceding process fixes
passing. Its native pthread fixture execs from a non-main thread while retaining
a Nix descriptor. Linux de_thread transfers the main PID to that worker and
releases the old leader; the PID's start time is inherited as well.

F1 (Medium, blocking): legacy_retirement_steps.go's process_has_no_userspace
accepted Z/X with Threads0 while the process directory remained. The review
observed 8 clean omissions through that predicate in 6000 native scans, and
150/150 detections with the same non-execing control. This corrects the earlier
consultation's assertion that releasing the leader requires an empty group:
fs/exec.c's de_thread is the counterexample. Linux v7.0 task_sig can report zero
after the old task loses sighand while the inherited PID remains live. Every
failed-read boundary and the two zero-thread test expectations were affected.
Confidence is high from kernel source and the review's native reproduction.

The correction requires exactly one thread for Z/X. The existing fresh directory
absence check handles ordinary reaping. Tightened process-query boundary tests
fail on the previous code at all six boundaries (12 zero-thread cases). A new
real-process fixture kills and reaps its own child during executable inspection
to retain genuine-disappearance acceptance. It does not change host processes
or native packages. This correction is distinct from F2 and cannot close it.

F2 (Medium, blocking): the same native review reproduced 113 omissions through
the missing-descriptor/later-live-leader path and 8 through a truncated descriptor
listing. Both can read the old leader before de_thread and the new leader after
handover, although the descriptor stays open continuously. A second listing and
later existence check reduced but did not eliminate misses (32/6000), so they
are not accepted as a complete fix. No residual has been approved by the owner.
The process scan and documentation claims remain under correction; final review
and delivery are blocked until this finding is resolved. Earlier CI successes
do not certify absence of this race.

The F1 correction passes the focused process race suite (1.830s), the complete
TestLegacy race suite (9.542s), formatting and diff whitespace checks. These are
partial correction results while F2 remains open, not final delivery evidence.

All 27 hosted checks subsequently passed for the reviewed 3082f6b candidate,
including both Linux migration architectures and macOS/Windows retirement.
Those checks do not reproduce or close F2, and the independent verdict remains
CHANGES REQUESTED. The uncommitted F1 correction still needs final-head gates
together with the eventual complete F2 correction.


### 2026-10-17 — prove complete Linux process reads across exec

Actual Opus 5.5 xhigh consultation on immutable 3082f6b (125 turns,
3179304ms) confirmed a complete F2 correction using read-only procfs properties.
Its exact old scanner missed 112 of 6000 worker-exec scans; its prototype missed
none of 60000, with 7% safe refusals. Removing the listing proof produced
176/30000 misses; restoring the old closed-descriptor inference produced
126/30000. Controlled same-thread-group observations and kernel sources from
5.10, 5.15, 6.1, 6.8, 6.12 and 7.0 establish the directory cursor contract.
The consultation's two Monitor calls were denied by its tool allowlist; ordinary
Bash inspection completed the measurements. Its repository clone stayed clean.

F2 implementation: the installed executable's privileged read-only Go helper
replaces the shell scanner. One open process directory anchors relative reads.
After fd enumeration reaches EOF, its cursor must equal a later positive FDSize
plus two. A silently truncated old-leader listing cannot satisfy this proof for
a continuously held descriptor. After a failed readlink, a second proved listing
must show the number absent or its current target must be read and classified.
The old live-state/PF_EXITING inference is removed. Both handover paths have
native deterministic regressions, including root's empty-table view and the
ordinary user's permission-denied view. No mutation or inventory retry is added.

Sibling finding (Medium, high confidence): a maps handle bound to an address
space destroyed by exec can return successful EOF. The consultation reproduced
three clean omissions in three sequences, including single-threaded exec;
ordinary stress alone did not reproduce it. The reader now rewinds that same
maps handle after a negative scan and requires one byte. Its pinned address
space cannot recover once destroyed. Native tests cover exec after open and
after the first chunk, while a store mapping becomes a store executable.

Parent consolidation also caught the scratch prototype closing its fd-directory
handle before classification: self-inspection could repeatedly encounter its
own closed/reused number and refuse. Production retains that handle until the
process scan ends, and a real self-scan passes. Basic pidfd_open/poll readiness
proves whole-process exit across main-thread exit and worker exec; a fresh procfs
ENOENT alone is not used as identity proof. The latter is conservative reasoning
from procfs lookup, not a claimed native false-ENOENT reproduction. F1 retains
exactly-one-thread Z/X and explicit kernel-thread proofs; zero threads refuse.

Rejected approaches: repeated scans and later State/start-time checks retain
the demonstrated handover ambiguity. pidfd_getfd adds ATTACH permissions and
its older-kernel EBADF semantics do not distinguish exit; receiving sockets can
also change their cgroup metadata. It is not used. The basic exit-only pidfd
handle requires none of those descriptor-copy operations. No ptrace, freezing,
new runtime dependency, widened timeout or host package mutation is introduced.

Verification of the integrated reader: the Linux process race suite passes
(5.656s), all TestLegacy race tests pass (11.466s), and the same process tests
pass as root in a disposable container. Root stress reports 2864 consumers and
136 safe refusals across 3000 scans/242 exec generations, with zero silent misses.
Four private source mutations independently restore the old zero-thread,
unproved-listing, missing-descriptor and silent-maps rules; their targeted
regressions all fail, including both native maps sequences. The corrected
production source passes those same cases. All earlier process regressions are
ported to the active syscall boundary; synthetic status/error fixtures retain
real pidfd liveness and procfs descriptor cursors. CI now runs the process
regressions with both ordinary and root permissions on its disposable Linux
runners. Final full-gate, published-head matrix and independent approval remain
required; the blocked 3082f6b verdict is historical, never rewritten as approval.

Scope remains a point-in-time process-reference check, not an atomic freeze.
The consultation separately notes pre-existing limits: thread-private tables
created by unshare, references deliberately moving between inspected locations,
and kernel-only consumers such as loop/registered io_uring files. These are not
claimed fixed by the F1/F2/maps correction or newly approved by the owner. The
original ARM PID423 interleaving remains unknown. Private audit logs retain the
consultation, kernel-source excerpts, native fixtures and mutation failures.

The first integrated candidate a345fb4 failed macOS and local EditorConfig:
its standalone C fixture used tabs, while .editorconfig requires four spaces
for C. The C bytes are reformatted without changing fixture behavior or the
checker. Local engine/terminal race tests passed (385.871s/23.988s), but that
full gate also failed the existing system-zsh prerequisite and is not a pass.
The in-progress immutable-head Opus review was interrupted after 12 turns;
its is_error result is not approval. The corrected candidate requires a fresh
review and matrix, with the formatting failure retained as evidence.


### 2026-10-17 — Final Linux review and Windows agent fixture ownership

Actual Opus 5.5 xhigh completed its exact-6b69e8b review (100 turns), confirming
F1/F2/maps corrections. Its independent 210,000 adversarial scans found no silent
omissions; ablation of the cursor proof reproduced omissions. Both native Linux
root CI lanes passed. The overall verdict remained CHANGES REQUESTED: Windows
core failed Codex startup with ErrWaitDelay and a sharing violation on its private
.codex directory. The following public migration correctly refused the unfinished
setup state. This is a pre-existing test supervision defect, not a Linux scanner
regression; no state guard is weakened.

A second actual Opus 5.5 xhigh consultation (64 turns) traced pinned Codex 0.162.1
plugin synchronization and Rust's Windows process inheritance. Background Git
can inherit pipe handles and hold CODEX_HOME as CWD after app-server exits.
The exact child in the original failure was not logged, so that identity remains
unverified. Forty bare native handshakes and a subsequent full lifecycle passed;
these are non-reproduction evidence, not a fix. Early diagnostic-only attempts
also exposed missing CODEX_HOME creation and a relative PowerShell cleanup path;
both fixture mistakes are preserved in the private audit logs.

The deterministic Windows regression starts a root that acknowledges the actual
protocol and a child with NUL stdio but inherited extra handles, mirroring Rust's
CreateProcess behavior. Before correction, f937bf8/run 39067548224 reproduces
ErrWaitDelay and the locked .codex directory with a still-live exact child HANDLE.
The independent outside process remains outside the tested ownership boundary.
The red fixture also exposed a cleanup ordering error (context cancellation before
outside-process cleanup); cancellation now runs after that cleanup.

The correction is test-only: create each agent in a private kill-on-close Windows
job atomically, inherit exactly its three stdio handles, preserve Unicode CWD and
case-insensitive last-value-wins environment overrides, and resume only after
creation succeeds. Root protocol/exit success is separate from owned descendant
completion. The unchanged Build Tools job helpers capture and wait exact member
HANDLEs; no PID/name-wide kill or production provider refactor is introduced.
Drain Codex output after acknowledgement and require EOF within the original
two seconds. T3 requires a live root before intentional stop and never exports
its output. Nonzero exit/stderr, protocol rejection, output overflow, cancellation,
Unicode environment/CWD and outside-process preservation have native regressions.
The native regression suite passes at b8f7257/run 39067891365; full lifecycle and
following migration outcomes are recorded separately before delivery.

Low L1: older pidfd_open implementations return EINVAL when a reaped identity is
retained only as a process-group/session ID; current kernels return ENOENT when
reused by a nonleader. With positive validated PID and flags zero, both prove the
listed group identity is gone. Boundary regressions failed for both errors before
correction and now pass while EPERM/ENOSYS/EMFILE remain fatal. Source checks of
Linux 6.8/6.12 and 7.0 support the distinction. The review's precise claim that
ENOENT began in 6.15 was not confirmed: 6.15 still has the older check. The fix does
not depend on a version threshold. Native old-kernel reproduction remains absent.

Low L2: the native exec stress fixture survived a killed Go test process. A new
regression holds its pidfd, kills the owning test binary and requires whole-process
exit; it failed before correction. Every fixture mode now checks the original
owner PID across waits and exec, including initial startup. This avoids tying a
parent-death signal to a transient Go runtime thread. The full focused Linux race
suite passes (8.300s). The stale shell-scanner comment above legacyCanonicalPath
is removed. These findings and the blocked verdict remain history; only fresh
exact-head checks and independent approval can certify the next candidate.


### 2026-10-18 — Discover zsh before isolating PATH

The owner questioned the reported missing system-zsh prerequisite. Inspection
confirmed zsh was installed and discoverable through Linuxbrew; the existing
path_dedupe_test.sh then replaced PATH with system directories before invoking
bare zsh and hardcoded /bin/zsh in later probes. The direct test exits 127 on
that installation. Earlier ledger references to a missing system-zsh prerequisite
are a diagnosis error, preserved here with this correction.

The test now resolves zsh once before restricting PATH, quotes that executable
for each probe and passes it as an argument to the nested macOS login shell.
The existing duplicate-removal, managed-priority and native path_helper assertions
are unchanged. The other shell tests were checked for the same restricted-PATH
pattern; zsh_vi_mode_test.sh already retains the discovered executable.

A new process-boundary regression places a delegating zsh wrapper in a directory
containing spaces, executes the real generated-profile test and records which
probes used the selected runtime. It requires both ordinary probes and, on macOS,
both nested login invocations. This fails on the original test and passes after
correction; ShellCheck also passes without suppressions. The local full gate,
final-head native matrix and Opus 5.5 xhigh review remain delivery gates, with
their exact candidate results recorded on PR #87.

### 2026-10-18 — Default Windows Python paths found by VM acceptance

The owner's Windows 11 VM keeps LongPathsEnabled=0. A real all-tools installation
failed at Python's checked-bytecode preparation: compileall's temporary suffix
exceeded MAX_PATH inside the unchanged private generation. The original native
Python lifecycle reproduced the failure; LaTeX and yamllint's native lifecycles
also failed in venv ensurepip under their deeper private fixture directories.
Ordinary relative/absolute path controls failed, while extended paths succeeded.
The new native regression initially had a missing parent directory in its
download fixture; after correcting that fixture, it reproduced the product error.

The correction uses Windows extended paths at both compileall call sites, with
`-d` retaining the old code-object filenames. It retains checked hashes and all
optimization levels. Private venv preparation also extends interpreter,
destination and requirement-file paths, and escapes the drive colon in local
wheel URLs so pinned pip does not discard the device prefix. Its new optional
Windows recipe revision changes desired identity without rewriting historical
receipts. Completed old generations remain available for Update; unfinished
old preparation is refused before modifying the stage. POSIX recipes are unchanged.

Native Windows proof with the registry unchanged passes bytecode flags/filenames,
runtime imports, payload immutability and the cwd lock that prevents recovery
renaming an active compiler's stage. Both private tools pass their actual native
lifecycles, including Unicode conversion or valid/invalid YAML, stable Python
replacement and exact removal; yamllint also passes version update and repair.
Native historical-pin/process regressions pass. The new base regression joins
the archive lane; the existing disposable Server Core lane additionally asserts
the default path policy rather than changing it. Its hosted result remains pending.

The real interrupted UI operation was abandoned through the original installer
after reviewing its no-resource-change plan: all 11 receipts and the failed
payload remain. Full Windows application/lifecycle acceptance, released migration,
final-head CI and final Opus review remain required. This entry records focused
proof, not completion of those gates.

Actual Opus 5.5 xhigh follow-up agreed with extended interpreter paths and the
Windows-only recipe change. The additional proof checks exact local URL escaping
(including spaces, percent signs, hashes and Unicode), pinned pip's device-path
roundtrip, extended console shebangs/interpreter prefixes, PyYAML's native module,
and complete old/new base-bytecode equality. Its ctypes BOOLEAN-return correction
is included. The top-level recipe field follows existing archive conventions;
the common validator rejects it on unrelated/POSIX pins, including nested inputs
through their own Validate calls. No additional nested-recipe exception is needed.
Opus identified Mason PyPI depth as a plausible separate failure; actual Windows
Neovim setup must establish that behavior before a Mason change is considered.

Rejected diagnosis: a managed Git executable appeared missing over Windows SSH,
but its physical payload remained intact and ran. Native mitigation inspection
found RedirectionTrustFlags=1 in the SSH process and 0 in an ordinary desktop
process. The same junction command ran Git 2.56.0.windows.2 from the desktop.
This is the SSH context's rejection of a non-admin-created junction, not an
absent/corrupt Git payload. Preserve the mitigation; desktop acceptance must run
in the desktop session. No junction, ACL or system-security change was made.

### 2026-10-18 — Windows Mason formatters and abandoned Neovim recovery

The all-tools Windows 11 run failed in the locked Mason PyPI recipes for
Gersemi 0.29.2 and clang-format 23.1.3 after all parser phases and the other
14 Mason packages succeeded. Both venv ensurepip processes used ordinary paths
at managed generation depth with LongPathsEnabled=0. Actual Opus 5.5 xhigh
confirmed the provider has no setting for extending those venv paths. An
isolated extended-cwd Python probe succeeded, but the real locked Mason probe
failed earlier: its path normalization rewrote the extended root and lock-file
creation failed. System policy changes, path aliases and vendor patches were
rejected as solutions.

The official Gersemi standalone executable and clang-format's native Windows
wheel binary passed real version and formatting probes at the same depth.
Their checksums match the upstream GitHub asset/PyPI metadata. Hidden Windows
archive prerequisites now provide the same pinned versions; the complete
clang-format wheel retains its license and metadata, and its PE imports require
the existing VC++ runtime provider. POSIX keeps Mason. The dependency regression
failed before the graph change; native standalone archive lifecycle tests then
passed in 7.12 seconds, including immutable payloads, update, repair and removal.
The first invocation used unquoted PowerShell 5 native flags and failed argument
parsing; the corrected invocation reached the real test. Existing archive pins
remain manually reviewed, as declared by Renovate; no automation inventory drift
is introduced. The fresh VC-runtime container exercises the formatter consumers
after installing their prerequisite, with the default path policy asserted.

Public Abandon of the failed synchronization exposed a separate recovery defect:
the engine archived its transaction, but Neovim Observe still returned a resume
token from the now-abandoned receipt. The planner cannot resume without that
transaction. Engine regressions reproduced the trap after both failed initial
installation and failed update; a publication-boundary regression also showed
that abandonment lacked a current-pointer guard. Neovim now limits resume to
in-progress receipts, preserves/discloses incomplete stages and journals, and
refuses abandonment while publication differs from the saved previous target.
Fresh operations use new source/intent without rewriting failed evidence. The
focused Neovim suite passes; full VM integration and final independent review
remain outstanding.

### 2026-10-18 — Corroborate mixed VC++ registry views

After Build Tools installation, the actual fresh Windows 11 VM held servicing
version 14.51.36247 in Registry64 and 14.44.35211 in Registry32. The sole Burn
bundle, both MSI dependency-provider IDs/versions and all three signed System32
DLLs agreed on 14.51.36247. The previous unconditional disagreement refusal
aborted setup preview. Which component wrote the older value is unverified;
Microsoft's registry documentation does not justify preferring one view.

Actual Opus 5.5 xhigh recommended corroborated reconciliation with conservative
removal protection. Native collection now feeds a deterministic PowerShell
resolver. A mixed-view result requires the newest servicing value to agree with
the sole Microsoft bundle, every observed MSI provider version and all three
DLL versions. Normal signature/hash/load checks remain in force. Older values
become disclosed outside consumers, preventing runtime removal through both
planner and native guards. Nothing edits or claims the older registrations.

The captured VM regression failed with the original decision rule and passes
after correction. Native controls reject missing/conflicting bundles, MSI IDs
or versions, DLLs, newer unsupported values and changed publishers. The first
fixture invocation failed because its testdata directory was absent; that
fixture error is separate from the subsequent reproduced product failure.
Completed old command encodings, with and without module binding, are matched
against exact digests tied to preserved Git blob
1cf1a58f06749d227ae95b564598e06cf69f4715. Other command fields and successful
native evidence remain mandatory; unfinished historical commands are refused
without journal writes or replay. Native compatibility tests pass. Fresh vendor
CI now installs VC++ before Build Tools, observes it after every compiler phase,
and verifies removal or conservative retention according to actual consumers.
That combined container result and full public VM rerun remain pending.

The separate released-installation migration VM's first replacement VC++ install
was blocked by Defender behavior detection. Exact reviewed SHA-256 and valid
Microsoft Authenticode were verified. One fresh public setup attempt from the
documented ordinary PowerShell workflow installed VC++ successfully with Defender
unchanged; no new detection was recorded at that checkpoint. This is not proof
that the earlier detection was false or that the launch workflow caused it.
Retirement and full migration preservation checks remain outstanding.


The VC++ consumer disclosure was present in machine observations but absent from
the interactive review. A failing-before renderer regression now verifies that
Keep and Retain show the specific outside registration blocking removal. The
full terminal suite passes after rendering that existing observation field.

### 2026-10-18 — Windows directory sizes are undefined inspection data

The real all-tools preview reported 13 pending resources, including propagation
from Neovim, CMake and Python directories whose before/after sizes differed
(0 versus 4096, 65536 and 16384). Each captured refusal had identical native file
identity, permissions and modification time. Later read-only native probes found
stable name-list hashes and consistent sizes; those later observations do not
prove the earlier cause. No evidence attributes it to Defender or another writer.

Microsoft's WIN32_FILE_ATTRIBUTE_DATA contract explicitly says both size fields
have no meaning for directories. Go 1.27.1's Windows Lstat exposes those fields
and can obtain metadata through different native APIs. Actual Opus 5.5 xhigh
endorsed excluding only Windows directory size from the during-inspection guard.
The earlier rule to preserve every predicate is clarified to mean every defined
predicate: an undefined value cannot establish a concurrent change. SameFile,
mode, time, directory membership in the fingerprint and each child's checks remain;
regular files, links/junctions and POSIX size comparisons are unchanged. Directory
size was never serialized, so existing sealed fingerprints retain their meaning.

The deterministic native regression feeds the captured filesystem size values
with real directory mode/time into the metadata predicate; it fails under the
original comparison. Native controls for file growth with restored timestamp,
permission changes and directory timestamps continue to reject changes. The first
invocation had a shell-quoting error; the corrected invocation reached the real
regression. No cold-volume cause or reproducibility is claimed. Complete public
preview, runtime/lifecycle verification and final independent review remain owed.


### 2026-10-18 — Actionable Windows legacy-owner recovery

The genuine released-installation Windows migration preserved an Administrators-
owned Git configuration. Its normal-user replacement correctly refused native
ERROR_INVALID_OWNER before publication, retaining the original bytes/SDDL and
saved intent. The generic error lacked a useful recovery path. Actual same-user
elevated Resume showed exactly one resource, integration.git, then required a
separate remaining-work review. That remainder completed as the normal user and
reported ready; the Git descriptor stayed unchanged. Retirement remains pending.

Actual Opus 5.5 xhigh initially classified the gap High and inferred that retry
would elevate all remaining work or require permanent elevation. The observed
one-resource recovery disproved those inferences. Its follow-up corrected the
finding to Medium actionability and endorsed typed native-error guidance, docs
and restricted/elevated native tests. Always-Pending/ApplyBlocked preflights were
rejected because healthy Check and unrelated work must stay available; automatic
elevation and ownership changes would widen authority unnecessarily.

The native regression reproduces ERROR_INVALID_OWNER for install, removal and
restoration while proving unchanged original bytes/SDDL, unadvanced publication
and removal of the refused staging copy. It fails before the diagnostic change.
The correction matches only that native error, retains its cause, names the owner
SID (and the local Administrators identity without a network account lookup),
and explains same-account scoped recovery. Primary-group refusal behavior remains
unverified and receives no special handling. Native passing recovery proof and
final full gates remain outstanding.

The corrected owner tests pass natively for all three operations in 8.19 seconds:
elevated recovery preserves exact SDDL and operation identity, normal-user
completion succeeds, Check leaves state bytes unchanged, and an unrelated new
selection keeps the administrator-owned profile healthy. The existing restricted-
token permission/lifecycle suite also passes in 6.33 seconds. A first green-run
invocation failed PowerShell's native argument parsing before tests; its corrected
quoted invocation produced these results.

The existing native finalization order runs profiles/configurations/archives
before Neovim. As the earlier Opus consultation noted, a refused Neovim Abandon
can therefore follow cleanup of independently proved obsolete archive generations.
The migration guide discloses that behavior; the Neovim guard itself preserves
its publication and recovery artifacts. No global finalizer reordering was added.

### 2026-10-18 — Windows retirement approval and Opus wording follow-up

The actual released-installation Windows VM reached a 27-item retirement preview
that called removals INSTALL/Add and reported Admin25, although the inspected
Scoop steps needed no administrator authority. The exact scope existed only in
clipped choice details and machine preview output. That interactive plan was
cancelled without applying retirement. This is an approval-disclosure defect.

Interactive review now reuses the machine scope description, including its
reinspection/digest and saved-resume-token checks, before either ordinary or
resource-recovery confirmation. The terminal presents the full text in its
scrollable review and retains default Back. Removal/Change labels affect display
only; engine actions and serialized receipts stay unchanged. Retirement privilege
uses actual remaining steps, preserving existing catalog and observation scope.
Ordinary machine-resource privilege policy is unchanged. Focused tests reproduced
the omissions/counts before correction and now pass for initial/recovery scope,
changed native identity refusal, long/narrow text and ordinary privilege controls.
A first changed-scope test added a personal file inside a preserved Pi package;
that is intentionally retained without changing removal authority. The corrected
fixture changes the package version, which actually changes the inspected scope.

Actual Opus 5.5 xhigh's independent 49-file review found no material batch defects
and identified five low-severity follow-ups. L1 corrects this ledger's earlier
claim that old Python preparation is refused before modifying its stage: the
archive publisher can first move the interrupted stage aside and create an empty
replacement; all old bytes remain preserved and disclosed. This follows the
existing recipe-refusal ordering and does not prove a mutation-free Resume.
L2's finalization-order disclosure is now also explicit in CLAUDE.md; there is no
claim that a refused Neovim Abandon leaves every other provider untouched.
L3 (damaged historical Neovim evidence blocking unrelated work) is under review.

L4 is corrected: ERROR_INVALID_OWNER recommends same-account elevation only for
BUILTIN\Administrators. Other owners receive explicit outside-setup ownership
resolution guidance. The native restricted-token regression copies a read-only
TrustedInstaller-owned system executable only into a private fixture, reproduces
the original misleading advice, and passes after correction with original
bytes/SDDL preserved and refused copy removed. The first diagnostic ran elevated
and reached descriptor verification instead of the target refusal; the corrected
restricted-token fixture supplies the intended authority boundary. All three
administrator-owned install/remove/restore regressions still pass (9.47 seconds).
No source system file or privilege was changed. Microsoft documents the
[owner-SID constraint](https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-setnamedsecurityinfow)
and [ownership privileges](https://devblogs.microsoft.com/oldnewthing/20050818-09/?p=34533).

L5 uses the neutral “Outside consumer” label for existing consumer observations,
including install and Pending, instead of claiming every entry blocks removal.
The renderer regression failed before the wording correction and passes after it.
Full VM retirement, application/lifecycle acceptance, final gate and final Opus
review remain pending.

The focused Opus follow-up identified an additional Medium disclosure error in
the initial privilege correction: Step.Privileged controls POSIX sudo dispatch,
whereas Chocolatey and machine/unspecified-scope winget can elevate within their
native commands. The final shared disclosure predicate includes those vendors and
historical Homebrew-install steps. It reads the existing typed package gates and
fixed argv, including the trailing winget scope, so historical intents receive
correct disclosure without adding a field, changing their digest or rewriting
approval data. Native dispatch and its privilege checks are unchanged. Initial
and resumed observations, summary counts and step markers use the same predicate.
Recovery review also shows the observed native outcome before approval.

Actual provider-planning fixtures reproduced the Chocolatey and unspecified
winget omissions, then passed for Chocolatey, user/machine/unspecified winget,
and user Scoop. The first fixture revision omitted a Go import; the subsequent
user-scope case needed a fresh native inventory because the production observer
intentionally caches a short preview burst. Both harness corrections preserve the
actual provider plans and assertion. Initial/recovery planner and full terminal
regressions pass on Windows, including ConPTY confirmation/cancel tests.

L3 is corrected at observation: unreadable saved Neovim JSON, manifest chunks or
partial-stage inspection become resource-local Unknown/Pending, with trusted
receipt-derived preservation locations and no completion/resume authority.
Engine regressions reproduce the original global failure after abandoned install
and after a missing current pointer, then verify unrelated profile publication,
selected-resource Check, unchanged evidence and no native re-execution. Check
continues to preserve recorded selections: a failed first installation followed
by Abandon leaves an empty selection, so its historical paths are disclosed while
that empty selection is ready. Selecting the damaged resource reports needs-action.

The active-corrupt-intent Abandon refusal remains a deliberate safety boundary.
Its previous publication target cannot be recovered from untrusted JSON; the
receipt baseline is not necessarily the pointer at this operation's approval.
The test proves local Resume refusal, unchanged engine state after refused
Abandon, and preserved evidence. No guessed pointer, ownership acquisition or
new recovery authority was added. This issue was found in controlled fixtures;
neither VM has shown damaged Neovim journals. Native proof and the integrated gate
for this final follow-up remain pending.

### 2026-10-18 — Final retirement/recovery Opus follow-up

Actual Claude Opus 5.5 at requested xhigh found no material defect in the scoped
final changes. It accepted deriving vendor elevation from existing typed native
gates/argv, including historical saved recipes, without a new persisted field.
It also accepted local Neovim corruption and the active Abandon refusal boundary.
The integrated gate completed successfully (Go race 442.659s; terminal 23.794s),
and native Windows corruption/owner/retirement tests and the full terminal suite
passed before the following final Low corrections.

The user-scope winget detail no longer contradicts its zero-admin summary. A
provider-planning regression reproduced the contradictory wording and passes with
scope-specific disclosure. A driver-level interrupted-retirement test executes
and records a completed administrator step, then proves that only the remaining
user step contributes to the recovery observation's privilege flag.

Corrupt current-generation Neovim evidence previously became merely unhealthy;
deselection could therefore begin a removal that the provider then refused.
It now uses the same Unknown/Pending boundary as saved historical evidence, with
paths derived from the validated owned generation. The engine regression proves
retention on deselection without an active transaction, continued independent
configuration work and unchanged damaged evidence. Active corrupt Abandon errors
now identify the journal and explain that prior publication cannot be proved.
Both failures were reproduced before correction; all focused Neovim/winget/
interrupted-observation regressions pass afterward. No fallback publication,
new removal authority or guessed ownership was introduced. Native final-source
verification, full Windows VM completion and exact-head CI remain pending.

### 2026-10-18 — Actual Windows shell exposed private Python environments

**High, fixed; native/full validation in progress.** The fresh Windows Terminal
resolved `python` to yamllint's private `Scripts/python.exe`. The private LaTeX
and yamllint pins exposed their entire venv bin/Scripts directories, also making
private pip and activation commands public on POSIX. A real generated-profile
regression failed in Bash, zsh and native Windows PowerShell before correction.
Actual Opus 5.5 xhigh confirmed that PATH reordering would leave private commands
exposed and recommended dedicated command directories.

All four targets now expose only the tool's declared console in `commands`.
Preparation copies the generated console's bytes, preserving its absolute private
interpreter. Existing bin/Scripts pins and interrupted operations retain their
original console layout; new Windows console pins require venv revision 1.
Focused tests pass for generated shell lookup, both console copies, all target
pins, legacy serialization and recovery after desired pins change. Native tests
exercise actual conversion/linting, unchanged payloads, a legacy-layout update,
repair, stable Python replacement and removal; completion evidence is pending.

### 2026-10-18 — Windows legacy PATH removals invalidated one another

**Medium, fixed; native validation in progress.** Genuine v0.4.4 Windows
retirement stopped before tree-sitter at transaction index 22 after the earlier
Herdr removal changed the User PATH. Each resource had bound the entire raw
registry value, so the first approved removal invalidated the later scope.
The native registry regression reproduced the same failure for two independently
planned removals and for an unrelated newly added entry; the account's original
raw PATH and registry type were restored after each test run.

New typed `user-path-entry` gates bind only exact matching entry strings and
count, encoded as UTF-16LE. Fixed removal commands reread the current value, retain all
other raw entries/order/empty fields and recheck that value before writing.
Changed owned entries still invalidate approval; the current supported registry
type stays intact even when an earlier approved uninstaller changes it. Historical
`user-path` gates and commands keep their original full-value semantics. Existing
interrupted operations require ordinary Abandon and a fresh approval; no saved
plan, journal or completion record is rewritten. Focused Go regressions pass;
native sequential removal, full VM recovery and independent review are pending.

Actual Claude Opus 5.5 xhigh independently confirmed the cause from the saved
transaction and approved exact-entry ownership. Its refinement avoids binding the
registry type, because approved upstream package removals may change that type;
the fresh supported type is preserved instead. Native step-level tests passed
before that refinement, including sequential removal and unrelated edit
preservation. Final native tests now additionally exercise the complete retirement
controller with both resources in one approved transaction, original-registry
restoration checks, Unicode paths, registry-type changes and historical intents.
The original three whole-PATH script hashes are pinned to their immutable Git blob.


### 2026-10-18 — Native PATH validation and completed approved Windows retirements

Actual Opus 5.5 xhigh found no material defect in the private-console/PATH batch.
Its Low follow-ups now use an ordinal comparison for the fresh complete registry
value and directly execute the fixed removal command after an owned-entry change,
proving exit 72 without writes. Before touching the registry, the native fixture
requires its original raw bytes to round-trip through the captured UTF-16 value;
unsupported representations fail before mutation. The final native PATH suite
passed in 32.79s, including both resources in one controller approval and unchanged
historical recipe semantics. The previous integrated `make ci` passed (Go race
456.645s; terminal 24.014s); these final Low edits require another integrated gate.

The genuine v0.4.4 migration completed the six remaining retirements through a
fresh public approval after Abandon; all 27 approved items now have completed
receipts and no active retirement transaction. Four personal witnesses, original
Git owner/group/permissions and four readable released profiles remain intact.
The public result truthfully remains `needs-action`: a modified PSFzf directory
and LazyGit from Scoop extras rather than the released main bucket stay protected.
Scoop, 7zip and dark infrastructure remain; active chezmoi configuration is absent
and its preserved copy is disclosed. This is completion of the approved scope,
not a claim that every observed legacy artifact was deleted.

### 2026-10-18 — Defender interrupted a private Python update

The fresh Windows public Update stopped at the final yamllint import probe with
missing `pathspec`, retaining its failed stage and transaction. The first wheel
process returned zero with no output. Defender events 1116/1117 at 16:07:47 UTC
identify that exact private-environment Python and its base interpreter as
`Behavior:Win32/SuspClickFix.G2`; later wheel phases completed. The final consumer
probe correctly prevented publication despite the misleading process result.
Identical native preparation tests and the migration VM's update had passed.
No antivirus policy or exclusion was changed. Recovery and final lifecycle proof
remain pending; this is not evidence of a dependency-pin or packaging defect.


### 2026-10-18 — Herdr could not open Windows PowerShell panes

**Medium, fixed; final publication pending.** Real Windows Terminal acceptance
reproduced Herdr 0.9.3's CreateProcessW error 2 opening a pane. The explicit
`default_shell = "pwsh.exe"` override reached portable-pty's registry-derived PATH,
which lacks the installer's deliberately process-local private command paths.
The alternative of a missing PowerShell package was rejected: the parent resolved
and ran the managed PowerShell 7.6.3 executable successfully.

Pinned upstream source `7b116c05bfda646af39d2524c54e70c751f57ee8`
(`src/pane.rs` and `vendor/portable-pty/src/cmdbuilder.rs`) proves the difference:
the default shell resolver validates and returns the absolute PowerShell path
from the process environment before constructing the PTY command. The Windows
configuration now leaves that default unset; the catalog's PowerShell prerequisite
and process-only PATH ownership remain intact. POSIX defaults are unchanged.

A normal Windows Terminal live pane and configured navigator worked with that
configuration. The new opt-in native archive fixture publishes the actual profile
and configuration, launches a foreground server in the existing owned-process job,
creates a workspace, and executes a witness inside the real pane. It failed with
the original configuration (88.41s) and passed with the correction (84.36s), proving
that the pane runs the exact private PowerShell 7.6.3 executable. Its private jobs
and temporary home are cleaned up. Static configuration checks pass. The fixture
is included by the existing native archive CI test selection; final-head hosted
execution and independent final review remain required.


Actual Opus 5.5 xhigh confirmed the Herdr root cause and the native consumer proof,
and identified that the released Windows config is still a live-link target.
The final correction therefore preserves that file's exact v0.4.4 bytes and points
Windows `config.herdr` at the existing shared `herdr/config.toml`. Static checks
resolve the active manifest source; checkout tests bind the passive file's released
SHA-256. This supersedes the intermediate direct edit described above. Opus also
confirmed the Defender interruption and accepted both earlier PATH Low fixes.
Its pip null-device and C++ execution-witness follow-ups are being reproduced and
addressed before final delivery. The integrated gate passed at the intermediate
Herdr/PATH state (Go race 458.152s); later follow-ups still require a new full gate.

### 2026-10-18 — Native pip isolation and compiler execution witnesses

**Low, fixed; final native verification pending.** Actual Opus identified that
Go's Windows `os.DevNull` is `NUL`, while pinned pip compares its configuration
override literally with Python's lowercase `nul`. A real private Python fixture
proved that `--isolated` still read its site pip.ini (failed-before, 32.17s).
Revision 2 uses lowercase `nul`; saved revision-1 operations retain their exact
uppercase environment, and both revisions remain readable. The base Python pin
is unchanged. Neovim's pip queries use the same exact spelling. Source validation,
desired identity, legacy recovery and native command environments have focused
coverage; native isolation and lifecycle verification are pending.

The Defender interruption also demonstrated that a child process can report zero
without completing its work. The C++ health probe now requires exactly one token
printed by the compiled program as well as zero exit. A real PowerShell/cmd
regression failed before for empty, wrong and duplicate output (3.17s); nonzero
exit remains a failure. This strengthens read-only observation without changing
historical vendor mutation commands. Final native verification is pending.


Actual Opus 5.5 xhigh reviewed the concrete follow-ups and found no material
Herdr fixture or compiler-probe defect. Its two Low fixture notes are addressed:
remove the unrelated VC++ prerequisite (Herdr does not depend on it), retain
bounded stderr diagnostics, and fail promptly when stderr closes before readiness.

Opus also established that revision 1 exists only in these uncommitted acceptance
builds. The final implementation therefore folds `nul` into revision 1 and removes
the invented revision-2 compatibility branch. This supersedes the intermediate
revision-2 description above. Completed revision-0 provenance remains readable;
unfinished old preparation still requires explicit recovery. Any interrupted
acceptance build must finish with its own original checkout. The prior native
batch passed yamllint install/update/repair/removal (185.70s), actual Herdr pane
execution (35.55s), pip configuration isolation (29.90s), and execution-witness
validation; final simplified-source native checks remain required.


### 2026-10-18 — Final native follow-up and Linux update acceptance

The simplified final Windows native suite passes: private-console layouts and
legacy shapes, isolated pip environment, yamllint lifecycle (170.20s), real Herdr
PowerShell pane (32.67s), pip.ini isolation (27.13s), and C++ execution-witness
validation (5.16s). The genuine Windows migration's final Update returned ready
with 19 selections and 42 receipts; all personal witnesses, Git permissions and
retained released profiles remain intact. Both Windows VMs have no machine pip
configuration. The fresh VM recovered the Defender-interrupted yamllint through
its original source. All 2,078 preserved payload entries match the failed-stage
snapshot. No new Defender detection occurred. The public recovery correctly
returned `needs-action` to review the remaining work; after complete publication,
public Abandon archived the superseded plan without changing resources. The final
source was then transferred with all 851 tracked file hashes verified and the
installer state unchanged. Remaining Windows lifecycle acceptance is pending.

The already completed Linux install/update/repair/remove/reinstall and genuine
released-installation migration now have final-source follow-up. Public Update
returned ready with 25 selections and 91 receipts. Exactly the two private Python
tool archives changed; the other 35 archive identities, including base Python,
remain unchanged. Actual Bash and zsh terminals resolve only the declared console
directories and exercise good/bad YAML plus Unicode LaTeX conversion. Neovim passed
257 strict language checks in 31.19s; all 29,485 managed entries are unchanged.
Final Check returned ready with an identical state hash before and after.

Twenty-seven personal-file hashes and both later shell edits remain intact. The
first acceptance-helper assertion used a stale pre-reopen Zen database hash; its
only mismatch was places.sqlite, last modified at 11:49 UTC, before the final update. Read-only SQLite integrity and the retained Example Domain
history pass. The corrected helper checks that database semantically and requires
its mtime to predate this update; no product test was weakened or skipped.


Actual Opus 5.5 xhigh's final working-tree review found no material defect and
confirmed the simplified revision-1 preparation, revision-0 read/update/removal
contract, Windows bytecode filename identity, unchanged historical vendor recipes,
and dependency graph. Its optional Low notes that direct provider Resume of an
unfinished revision-0 preparation can preserve another empty stage before refusing.
This preserves every artifact; the documented original-checkout recovery and
public Abandon remain available. No runtime data-loss defect was established.
The final integrated gate subsequently passed (Go race 451.178s; terminal cached;
51 bootstrap tests with two platform skips; 67 Renovate records). Targeted final
Windows coverage and final-head hosted checks remain in progress. The review's
pending Linux and Defender-recovery gaps are closed by the evidence above.

The fresh Windows final-source Update returned `ready` at 17:46 UTC with 20
selections, 55 receipts and no active transaction. A desktop-launched PowerShell
session passed all 25 command/version checks, including the base Python lookup
and the absence of private tool `Scripts` directories on PATH. Neovim passed
257 strict language checks; all 31,157 entries in its final managed runtime and
the installer state hash were unchanged. The earlier failed runtime's 31,158
entries are a separate preservation witness, still awaiting the final lifecycle
comparison. Code and T3 were closed after their GUI checks, with the edited
project file unchanged; removal's baseline records 31 personal hashes and a
later shell-profile edit.

Two acceptance-helper corrections do not change product code. The application
closer now handles a Code process exiting between enumeration and path lookup,
while still refusing an uninspectable live process. The public-command wrapper
now retains the root process handle and waits for that process's exit. Its old
`Start-Process -Wait` invocation remained waiting after the completed Update's
root had exited; a vendor telemetry process remained, but its causal role was
not proved. The ready JSON and cleared transaction were captured before the old
wrapper wait was interrupted. That original wrapper did not capture an exit
code; subsequent commands use the corrected root-exit capture. The unchanged
Update preview, shortcut repair and remaining Windows removal/reinstall checks
are still pending.

### 2026-10-18 — Personal profile line endings and saved UTF-16 blocks

The fresh Windows unchanged-Update preview returned 54 Keep and one spurious
shell Repair after the acceptance fixture appended a personal CRLF suffix to an
LF profile. Its state hash was unchanged, and no repair was applied. Rendering
chose line endings from the whole file, so outside text changed the desired
bytes of the untouched owned block. **Low, fixed; native rerun pending.** Existing
blocks now supply their own line endings. A multi-location check found the same
behavior in Sentinel rendering and applies the same ownership boundary there.

Expanded controller-level encoding coverage also exposed **Medium, fixed;
native rerun pending**: installing into UTF-16 profiles published the file but
then failed completion verification. Schema-1 journals correctly save raw block
bytes without the surrounding BOM; the generic block reader rejected those
fragments. Sentinel already had a validated marked-fragment reader. That reader
is now shared with ordinary profile blocks, with exact marker and UTF-16 unit
validation. Full consumer files still require a BOM through readProfile; no
journal schema, fingerprint or stored bytes change. Existing files and saved
recovery artifacts remain intact.

The original controller regression failed for LF/CRLF personal edits and both
UTF-16 byte orders. The corrected focused profile/Sentinel suite passes, including
mixed-line-ending Check/Update/removal and rejection of unmarked, incomplete,
ambiguous and BOM-less full-file inputs. Expanded interruption-boundary tests,
independent Opus review, the integrated gate and native reruns are in progress.

The native Windows follow-up passed mixed line endings and encoded publication
recovery (15.65s and 10.43s), including unchanged permissions/publication paths;
the integrated gate passed with installer race tests in 412.814s. Linux's final
Check returned ready for 91 resources with unchanged state. Windows's corrected
Update preview returned all 55 resources Keep, with the personal CRLF suffix
retained and identical installer state before/after. A prior console-launch
attempt failed before setup because its test wrapper inherited PowerShell 7's
module search path; launching through native powershell.exe corrected the harness.
Production PowerShell module isolation already covers this boundary.

Actual Opus 5.5 xhigh confirmed the newline fix, old/new UTF-16 recovery and
unchanged ownership semantics. Its Low validation finding is addressed: the
BOM-less fallback must span exactly the saved block, including for Sentinel;
trailing personal text cannot be accepted as a complete journal payload. The
negative regression failed before and the focused suites pass after correction.
The stored schema and bytes remain unchanged. Final native/gate confirmation for
this stricter reader is pending.

Opus also reported unchanged, nonblocking fixture limitations: polling a physical
rename and then killing a process can miss the publication window. Its repeated
old/new runs observed the same timing failures in profile and Sentinel process-
death tests. The real process-death assertions remain intact; the complete gate
above passed. No runtime recovery regression was identified. Separately, prepending
prose before a Sentinel block that originally began at byte zero changes its
owned separator; both old and new code require adoption and preserve the file.

The strict fragment correction passed native Windows encoded recovery (11.37s),
mixed-line-ending lifecycle (21.46s), full-file/fragment negative validation and
Sentinel adoption/removal (1.91s). Actual Opus 5.5 xhigh found no material defect
in the exact-span correction; this last review was static because its process
PATH did not expose Go. Its optional request for redundant positive separator
rows is already covered by controller/lifecycle cases. The final Linux Check on
the same strict candidate returned ready for 91 resources with unchanged state.
A subsequent integrated local gate lost its make supervisor before completion;
the retained log explicitly reports termination, so that invocation is not
claimed as a passing gate. A supervised rerun records the actual final status.

The orphaned installer suite subsequently finished with one real fixture failure:
`TestPublicBootstrapModuleCleanupPreservesInheritedCache` left only newly created
Go telemetry directories under its private compiler configuration during TempDir
removal. **Low, corrected:** cleanup now runs `go telemetry off` in that same
isolated environment before `go clean -modcache`. This preserves the inherited
module cache and all caller settings. The native regression queries the resulting
private mode; the old helper fails with `local`, and the correction passes 20
runs (0.187s). This changes test cleanup only. Go's documented configuration and
the executing pinned toolchain's telemetry startup path support the diagnosis:
https://go.dev/doc/telemetry. Final integration and independent review are pending.

Actual Opus 5.5 xhigh independently confirmed the private telemetry correction,
reproduced the old-helper failure 3/3 times, and passed the corrected test 20 times
(0.193s), gofmt/vet and diff checks. No material defect was found; the remaining
notes concern unchanged internal Go test-environment overrides and earlier
production compiler children, neither observed in this acceptance. The supervised
strict-profile integrated gate passed (454.761s total); the test-cleanup change
has its own final integrated run in progress.

The fresh Windows strict-candidate Repair preview completed read-only with exit
zero and identical installer state. Exactly the deliberately deleted Windows
Terminal shortcut requires installation; the other 54 resources remain Keep.
The preview took 11m58s including a fresh verified bootstrap and complete package
inspection. This is recorded as observed latency, not a hang or verification
shortcut. The approved single-shortcut repair and removal lifecycle follow.

The final integrated local gate, including the telemetry cleanup regression,
completed successfully with recorded exit 0 (470.214s total). Formatting, vet,
installer/terminal race tests, available configuration checks, 51 bootstrap cases
(two explicit platform/tool skips) and 67 Renovate extraction records passed.
Earlier failed/terminated runs remain recorded above.

### Hosted Windows fixture corrections after candidate a9fa162

The Windows quality job failed two ordinary fixtures; the Windows core job
reported the same owner fixture failure. **Low, corrected:** the new owner-recovery
fixture passed a raw 8.3 TEMP prefix directly to the profile controller, unlike
its established canonical-home helper. The real parent-identity guard correctly
refused publication. Resolve the fixture home through the existing native resolver;
keep production's guard intact. A junction-root lifecycle also reproduces the
boundary independently of volume short-name policy.

**Low, corrected:** the agent protocol fixture's atomic PID-file rename can expose
the destination before its DELETE-access handle closes. Go's ordinary read open
does not share deletion, so the polling reader must treat the exact native
sharing-violation result as pending inside its existing 15-second deadline.
Other errors still fail immediately. The real fixture now holds a DELETE-access
handle until the reader acknowledges publication, reproducing the race without
sleeps or internal mocks. It accepts a successful first read too, should future
Go sharing semantics improve. Root/child liveness, outside-process survival,
exact job cleanup and the two-second stream EOF bound remain unchanged.
Source: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew
and the executing Go toolchain's syscall.Open share flags. Other profileControllerAt
callers already use canonical fixture roots; the Build Tools PID fixture writes its
marker directly instead of renaming it, so it does not expose this DELETE-handle
sharing boundary. Its separate stat/read publication pattern is unchanged.

A real elevated Windows VM run forced TEMP/TMP through an actual 8.3 alias. The
old variants reproduced the same owner refusals and PID sharing violation; the
corrected native suite passed three repetitions, including ordinary restricted-
token completion, owner/group/DACL preservation, Unicode environment, deadlines,
intentional stop and external-process survival. Installer state remained exactly
unchanged. These are test-only corrections; the VM's production candidate and
active lifecycle source are unchanged. Focused Opus review and the integrated
gate are in progress; this candidate's failed CI runs remain recorded.

The missing-shortcut Repair completed ready with the correct target restored,
then all eight managed terminal processes closed. Default deselection previews
40 Remove and 15 Retain; explicit shared approval previews 54 Remove and only
VC++ runtime Retain, protected by an older Registry32 runtime registration.
Both previews are read-only. A harness assertion initially expected Healthy on
every removal observation; that was a false alarm for shell integration, whose
desired block reflects the now-empty selection. Its original fingerprint and
identity are unchanged. The approved full removal is now running.

Actual Opus 5.5 xhigh found no material issue in either Windows fixture fix.
It checked the pinned Go sharing flags, exact native errors, canonical-home
callers, retained assertions and native red/green evidence. It independently
corrected the Build Tools rationale to direct publication, as recorded above.
The final local integrated gate passed with exit 0 (49.291s; production Go tests
cached because these changes are Windows-only test files). Hosted confirmation
on the amended commit remains required.

Windows full removal completed its native Build Tools uninstall, then correctly
refused staging the deliberately Administrators-owned Code settings as the
ordinary user. The original file remained active. A read-only public retry
preview contained exactly integration.vscode; same-account elevated recovery
completed that one saved removal and stopped at the next resource. Original
owner/group/DACL remained unchanged. Public needs-action intentionally exits 1;
the acceptance wrapper initially treated that as failure. Reading the captured
result verified successful one-resource recovery without rerunning it, and the
harness now handles only that exact expected outcome. Ordinary-user remaining
removal and reinstall are still in progress.

### Restored personal configuration must not retain removed runtimes

The next real Windows removal preview exposed a Medium planner defect: completed
one-resource Code removal restored the personal theme but then treated that
present file as an installed integration. This retained Code, its theme/fonts
and shell prerequisites despite the reviewed deselection. The preview was not
applied. The same missing completed-removal case affected later maintenance of
restored JSON and whole-file configurations.

The narrow correction recognizes removed/reused receipts only when their saved
result and original baseline both still match the current observation. It uses
existing Forget bookkeeping before maintenance handling; it grants no file
mutation and preserves normal retained-resource dependency protection. An actual
application lock reproduces interrupted JSON removal and one-resource recovery
without mocking the provider. That regression and real whole-file/JSON lifecycle
cases fail before and pass after correction. Pure policy controls retain changed,
pending, unknown, uncertain and incomplete evidence; schema-1 receipts without
an operation ID keep their existing saved baseline contract. Native verification,
independent Opus review, the full gate and the VM continuation are pending.

The removal-bookkeeping regression reproduced the same failure in a native
Windows binary. The corrected suite passed three repetitions, covering real
application-lock interruption/recovery, shared prerequisite removal, subsequent
JSON/whole-file lifecycle commands and all refusal controls. The real VM state
hash stayed identical. Build evidence records the exact test and planner hashes
for both binaries; no production VM source was replaced during its transaction.

The full local gate for the removal-bookkeeping correction passed with exit 0
(505.663s total; installer race tests 435.053s, terminal 23.888s, 51 bootstrap
cases with two explicit skips, and 67 Renovate records). Native Windows evidence
and the existing refusal/retained-dependency tests also pass. On preceding
commit f26d290, both previously failing Windows quality/core jobs and the native
Windows Neovim lifecycle now pass; final-head confirmation remains required for
the planner correction.

Actual Opus 5.5 xhigh confirmed the restored-baseline defect also affects ordinary
later removal and identified two follow-ups. Medium: Retain and preservation
failure handling could erase the historical removed status when evidence was
temporarily changed or unavailable. Low: the initial predicate relabeled absent
tools as preserved personal artifacts. Real edit/missing-journal lifecycle
regressions and a collateral-filesystem-change regression fail before and pass
after the corrections. Completed removal history now survives current failures;
the transaction still records the failure and cannot report ready. Absent tools
retain their original explanation. Changed restored files remain protected.

The collateral-change regression also exposed a gap introduced by releasing a
present baseline: ordinary Forget did not verify it after later operations. The
existing final removal verification now rechecks these proved baselines; declared
legacy replacement mappings keep their separately verified successor handover.
Both retained and released cases fail before and pass after correction, and the
real Git handover tests pass. Sentinel restoration is included with JSON and
whole-file configuration. The earlier integrated/native passes precede these
follow-ups; their final runs and independent review are required.

The original Windows public Abandon completed ready at 20:21 UTC. No resource was
in flight; Code remains reused/removed with its original theme, personal keys and
owner/group/DACL. The original failed Neovim journal hash is unchanged. The
transaction is cleared and its history and preserved artifacts are disclosed.
No state or recovery journal was edited manually.

The complete follow-up implementation passes the native Windows suite three
times, including edit/journal restoration, collateral-change refusal with durable
history, Sentinel and legacy-receipt controls; installer state is unchanged.
The final integrated gate passed with exit 0 (505.945s; installer race 434.190s,
terminal 23.814s). Both VMs received the verified production candidate. Linux's
read-only Check is ready for 91 resources with unchanged state. The corrected
Windows preview is exactly 29 Forget, 25 Remove and one retained VC++ runtime,
with unchanged state; its approved remaining removal is running. All 27 hosted
checks passed on preceding f26d290; the corrected final commit still needs its
own exact-head matrix and final independent review.

The corrected Windows full removal completed ready at 20:36 UTC with no active
transaction and an empty selection. All 31 personal hashes match; the later
PowerShell suffix, Code personal keys and original owner/group/DACL are intact.
Seven evidence/retained receipts remain, including the protected VC++ runtime and
restored/preserved configuration history. Reinstall preview correctly requests
adoption only for the restored Neovim configuration and scoped Code settings;
Neovim synchronization waits on that explicit configuration choice.

### 2026-10-18 — Preserve completed removal across reselection and final Pending

Opus 5.5 xhigh confirmed the restored-baseline correction and found the same
history loss in pending/keep reselection. Local real JSON-profile lifecycle
regressions reproduced both paths: reselecting without adoption rewrote a proved
removed receipt and subsequently pinned the runtime. A further filesystem-boundary
regression moves the completed journal during a later publication, exercising the
final wanted-resource Pending path. Preserve removed history in all three paths;
retain blocked reasons, pending status and required-journal finalization errors.
Explicit new publication still establishes new ownership normally. No state schema
or never-adopted resource policy changes. UPGRADING now describes select/adopt/remove
for intentional personal edits. Focused tests pass three repetitions; native/full
gate and the focused independent follow-up are pending at this entry.

Native follow-up proof: Windows RED exited 1 and GREEN passed three repetitions
at 20:55 UTC, leaving the live installer state unchanged. Green binary SHA-256:
`f221a3744ddccfe557d8b8c69ac5e98ba73ad07dca6e18d2c75c7ef95a09037e`.
The public Linux Check with final source identity
`a23c3534ef90289d234ed9cf3359092288062988321ba7baae34e8cfad2815d1`
returned ready for 91 resources at 20:54 UTC; state SHA-256 remained
`72c9d2591fbf11f0d52c8b66197bb62cb942e02ee169c093d86c55488f90d784`.
The corrected Windows full removal had already completed at 20:36 UTC: 29 Forget,
25 Remove and one protected VC++ Retain, all 31 personal hashes preserved, with
Code's personal settings and original SDDL unchanged. No failed Neovim journal or
state was edited. Reinstallation and final consumers are still in progress here.

The integrated final correction passed full `make ci` in 502.980s: installer race
suite 431.129s, terminal race suite 23.878s, formatting/vet and configuration checks,
51 bootstrap tests (two explicit platform skips), and 67 Renovate tests. No checks
or assertions were weakened. Windows reinstall is now at the real Neovim sync.

Opus's follow-up accepted the Pending/Keep guards and intentional-edit guidance,
then identified a Low regression: a Retry-recovered deferred removal was only a
local value and the new early break could discard it. The real APT provider and
controller fixture reproduces this at the native-process boundary: retain the
executor's durable pre-completion checkpoint while preserving the provider's
actual completed journal/pool ledger, then add an outside consumer and Retry.
Before correction the transaction cleared with a stale created/in-progress
receipt. Save the recovered receipt before both non-mutating early exits. The
regression now proves removed/reused/deferred completion, unchanged outside
consumer, zero repeated native commands and explicit reclaim on reselection.
All nine focused tests pass three repetitions. No VM state or journal was edited;
the crash checkpoint exists only in the isolated regression fixture.

The retry-persistence correction passed native Windows RED/GREEN at 21:07 UTC:
RED exited 1 on the exact lost-receipt assertion; GREEN passed three repetitions,
with the live installer state unchanged. Green binary SHA-256:
`9b4dc875f09dbf9ef9e2c569c45c7c3505e528a87d3ae87ce2dae334c78dfca5`.
Linux Check also passed at 21:07 UTC for 91 resources, preserving state hash
`72c9d2591fbf11f0d52c8b66197bb62cb942e02ee169c093d86c55488f90d784`.
Its final public source identity is
`599fb20f5bb97e02f1756589d2831e3f33913cab7a6f5c3051cfef074433e815`.

Actual Opus 5.5 xhigh final follow-up: no material findings. It verified both
receipt assignments, pending-result preservation and unchanged authority, and ran
12 focused tests plus three race repetitions of the APT regression. Its independent
process-boundary probe fails one read-only dpkg query immediately after provider
removal, before executor finalization; that also reproduces RED and passes GREEN
without replaying worker commands. The mirror Pending assignment has no dedicated
mutation-killing test; review confirmed it preserves the same recovered value.
An unchanged fresh-observation Pending branch was noted as a possible analogous
case, without a reachable reproduction or loss of the pending transaction; it is
not a validated defect from this patch. Reviewer scratch copies were cleaned.
The parent full gate, native Windows regressions and VM acceptance remain the
sources of broader verification; the reviewer did not claim those executions.

The final two-assignment follow-up passed the complete local `make ci` gate in
511.437s (installer race suite 435.755s; terminal race suite 23.865s). Formatting,
vet, configuration checks, bootstrap tests and Renovate checks passed; the two
explicit bootstrap platform skips remain disclosed. Windows Neovim reinstall
also completed and advanced to the remaining application archives. No source
was changed underneath its active transaction.

Windows reinstall reached the same administrator-owned personal Code settings
boundary after successfully reinstalling Neovim (95 parsers and Mason tools).
The ordinary preview preserved state and contained exactly one saved adoption.
Same-account elevation recovered only that operation at 21:35 UTC, advancing the
transaction from 50 to 51 with original SDDL unchanged. The public needs-action
exit 1 correctly requires a fresh normal-permission review of remaining work;
it is not a failed resource recovery or authority to elevate the entire plan.


### 2026-10-18 — Windows full reinstallation and preserved recovery evidence

The ordinary-user remainder completed ready at 22:03 UTC: 51 Keep and four
Install operations, following the previously recorded one-resource Code recovery.
The final state has 20 selections, 55 receipts and no active transaction; SHA-256
`93ac14c13decdba050c2d8e3020f8770d94285be180e4c02e0a79c9b79f3fc91`.
The reviewed final source was then published with all 852 source files verified
and installer state unchanged. No source was replaced during the transaction.

All 30 applicable personal-file hashes survived reinstall, alongside the later
PowerShell suffix, Code personal keys and original owner/group/DACL. The removed
personal Neovim baseline is deliberately replaced again only through its approved
configuration adoption. The reopened Windows Terminal selected managed PowerShell
7.6.3 with RemoteSigned; its fresh shell passed 25 command/version checks, private
Python command isolation, personal profile markers and scoped Git defaults.
Code reopened the existing saved project with the same file hash, Rosé Pine theme
and personal font size. The task-local PS5 verifier initially read its UTF-8 JSON
using the default encoding; explicit UTF-8 verification passed without modifying
the settings or product source. The real project window was independently observed.

The original failed Neovim stage remains byte-for-byte intact across removal and
reinstallation: all 31,158 recorded entries match, including paths, modes, links
and file hashes. Its original journal still has SHA-256
`279eaa2820292d2bf93cb3b6c63e7e5f037e46031bcc164cafe32abba838fff5`.
The final snapshot completed at 22:15 UTC. Defender antivirus, real-time protection
and behavior monitoring remain enabled, with no new detections since 17:00 UTC.
The strict Neovim consumer reported 257 passing checks; its post-use fingerprint
and the public final-source Check are still running at this entry.


Final Windows verification completed at 22:19 UTC. Public Check reports ready for
all 55 resources with unchanged state SHA-256 `93ac14c13decdba050c2d8e3020f8770d94285be180e4c02e0a79c9b79f3fc91`.
Its platform executable source identity is
`5f2a190cd4eb9606b648aa99f8b82e06343aea1f95b00ddbc8aee2eeb584d4db`.
The reinstalled Neovim consumer passed all 257 strict checks, and its before/after
fingerprints match across all 31,157 entries:
`101fedf5932b38bb8e74bc8d03a38a780d81bffe088bf9ea0a5b94e240857cc4`.
The consumer run also left installer state unchanged. Post-application personal
preservation passed all 30 hashes, both personal profile markers, later suffix,
Code keys and original SDDL. The 22:19 Defender verification again found all three
protection components enabled and no new detections since 17:00 UTC.

Linux and Windows VM acceptance is complete within the recorded scope. Genuine
Windows migration completed all 27 approved retirements; modified PSFzf and
outside-bucket LazyGit remain intentionally protected by its truthful needs-action
result. Provider authentication, Tailscale enrollment and microphone/paste remain
manual boundaries. Mac manual acceptance is deferred to the next step. The final
implementation passed full local CI and actual Opus 5.5 xhigh review with no material
findings; the ensuing single-commit PR update requires its own 27 hosted checks,
whose exact commit and results are recorded on PR #87.


## 2026-10-19 — Mac bootstrap and latest Sentinel work in progress

The approved starting head remains `d9a39938d5571634427d00c4c76a580d55662ec8`.
The user requested exactly one retained VM and latest Sentinel skills following
Helix repository retirement. Other local VMs and the cached image were removed
after inventory preservation; only the fresh acceptance VM remains. Public GUI
setup, ordinary sudo authentication, Apple CLT provisioning, Homebrew bootstrap,
the mandatory replan and tmux installation passed on macOS 26.6.2/25G83 arm64.
Original-revision Sentinel install/removal preserved personal policy bytes and
0600 mode. An apparent tmux repair requirement was caused by changing the checkout
path spelling from `/users` to `/Users`; repeating from the original spelling
produced Keep, so no ownership check was weakened.

Baseline full CI found a test-fixture race: Git commit spawned detached automatic
maintenance, which could write a lock after the published-runtime snapshot. A
standalone trace reproduced it without Neovim. The regression failed before
adding commit-local `maintenance.auto=false` and all four focused managed-startup
cases passed afterward. The claim that Neovim startup caused that particular
write is rejected by the independent trace; the complete snapshot assertion is
retained. Full post-integration CI remains pending.

Actual Claude CLI 2.1.301 with model `claude-opus-5-5`, effort `xhigh`, plan
permissions and Read/Glob/Grep completed the Sentinel design consultation. Its
recommendations were to use vendored upstream copies and the existing journal,
refuse outside ownership, and retain the explicit original-revision removal
contract instead of inventing an unreleased policy migration. Current source is
Sentinel `da84979b208273fe3850b16fadae5f8bca5e3498`, archive SHA-256
`3bffa9ef0e127d5a156128c59daa6deef3ad06e7c38ff620fc2bc41c66bdc272`.
The 12 vendored skill folders include upstream licenses. Copies, current four
policy targets, and skill ownership guards are under focused verification;
installed discovery/engine execution, final Opus review, full Mac acceptance
and the updated exact-head 27 hosted checks are not yet claimed.

Focused Mac tests now pass the complete skills copy lifecycle, foreign-folder/
identical-copy/link/junction-root/registry/lock/stage refusals, independent-root
progress, old three-target policy refusal, and actual process-kill resume and
restore. The opt-in upstream archive/renderer check matches all vendored bytes.
The first integration CI run rejected the unchanged upstream Impeccable YAML
solely for missing final LF. Preserve the upstream bytes: the exact vendor path
is exempt only from that style rule. Direct probes confirm repository-owned
missing-LF YAML and malformed vendor YAML still fail. Syntax checks and whole-file
provenance remain required; a broad vendor lint exclusion was not introduced.

The macOS full gate also exposed a native Ghostty validation logging defect: its
CLI reopened inherited regular-file stdout and erased preceding CI evidence,
leaving a sparse zero-filled prefix. The standalone preface/validation/suffix
reproduction and new redirected-log regression both failed before the fix.
`tests/ghostty/validate.sh` now captures the native command through a pipe and
retains its diagnostics and exact status. The native regression passes afterward;
no configuration assertion or CLI failure is suppressed. The earlier incomplete
log is retained as evidence and a complete full gate will be captured again.

Actual Opus 5.5 xhigh implementation review identified an ownership gap in the new
skills driver: a completed removal retaining saved edits also retains Recovery,
which must not authorize adoption or Keep of a subsequent outside installation.
The real copy/edit/repair/remove regression reproduced the incorrect adoptable
preview. The correction checks created ownership or in-progress publication,
and never exposes adoption without created ownership. Different and identical
outside copies after removal now remain Pending; preserved edits survive both
reinstall cycles. A separate native regression reproduced refusal of a linked
system home. The guard now stops before the observed home, while consumer-link
refusals and canonical destination binding remain intact. Both focused cases and
the complete skills collision/recovery suite pass. The reviewer also correctly
flagged that its frozen patch preceded the YAML exception; the final review must
include that exception, the Ghostty logging correction and these ownership fixes.


### 2026-10-19 — Sentinel native discovery, recovery, repair and full gate

Actual Claude Code 2.1.301, `claude-opus-5-5`, `--effort xhigh`, plan mode and
Read/Glob/Grep reviewed the final implementation patch, SHA-256
`9630ceeb88693149cc7506ad24401422ba15e55d4f62ecdc265302694183d42f`.
It found no material issues and confirmed the ownership and linked-home fixes.
The complete local `make ci` gate passed: installer race suite 681.782s,
terminal suite cached, 51 bootstrap checks with two documented skips, 67 Renovate
records and all configuration/static gates. The captured log is intact with zero
NUL bytes, proving the Ghostty regression correction under the actual full gate.
Final-commit hosted native/quality and CodeQL checks remain required.

Trusted working-source archive SHA-256
`db97fce54533671050d8887c625108bd0dc5c2b6420bae91026ac9038ec3ec5e`
was transferred with all 962 source files verified. Guest executable identity is
`3b13df443718df795b3a8a582db27748903a04085125445200f752822ec8fe59`.
The current public install adds only Sentinel policy and the two skill roots to
the existing 25 selections. SIGKILL after the first skill publication produced an
authentic in-progress receipt; original state, journals and partial files were
archived before public Resume. Resume recovered that resource and required a fresh
approval for the remaining root. No state or journal was edited to force progress.
All 206 installed bundle files match upstream bytes and modes across both roots.

Actual installed Codex 0.162.1 `skills/list` and Claude Code 2.1.287 control
initialization discover all 12 Sentinel skills and the personal skill. The
installed model-discovery helper returns eight Codex and four Claude models,
without claiming authenticated access. Impeccable correctly refuses an absent
engine, then verifies setup of pinned 0.1.15; context, detect and cached setup pass.
Its engine cache stays personal. The upstream installer correctly refuses to
claim the dotfiles-owned folders and creates no registry. No provider turn ran.

Public Repair of both deliberately edited helix copies preserves 24 skill
folders, including exact edited bytes and modes. Both real agent hosts still
list each skill once: hidden recovery copies do not become duplicate skills.
Public Update completes with identical Homebrew package versions and all pinned
package/configuration resources kept. Read-only Check preserves 107 state/journal
files after the subsequent explicit Code-settings Repair and reports ready.
An earlier Check preserved 106 files and correctly reported the test harness's
Unicode-escaped rewrite of managed Code values. That conservative adoption
requirement is not treated as an ownership bug; the user edit was preserved via
the public approval flow. A harness assertion run before transaction finalization
was repeated only after the public result completed; no recovery state was changed.

Strict installed Neovim passes 257 checks with all 33,232 managed runtime entries
unchanged (fingerprint
`c522e9667dda79385aab5f6c1afb5beb2e60e909b92301ac1b0cb436d0a06573`).
Real divergent Git pull rebases; a personal global Git write leaves source config
SHA-256 `a6966f0d597db7cb060d385187a31d9d7c4f7ea72a89a745a105fd361ebe259c`
unchanged. Native terminal consumers and real tmux/Herdr panes pass. Ghostty's
fresh login displays the configured prompt and preserved personal markers.
Normal guest Accessibility permission enables AeroSpace's visible tiling and
login item. Tailscale starts through the documented service command and reports
NeedsLogin; stable T3 serves HTTP and initializes its personal V2 database.
Neither enrollment nor authenticated provider execution is claimed.

Tart coordinate clicks fail with `noWindowsAvailable`, including after reset and
visible-window assistance, and modifier shortcuts do not reliably reach the guest.
Ordinary Tab/Space successfully handled Ghostty's Documents prompt. VS Code
launches with Rose Pine, but delayed onboarding intercepted the attempted edit;
the witness file is still empty. Document editing is explicitly not passed.
AeroSpace shortcut routing, remaining desktop application workflows and actual
microphone/paste remain unverified. SIP and Gatekeeper stay enabled. The guest
reports an Apple virtual input/output audio device and no camera; that inventory
is not proof of a physical microphone workflow. Reboot/lifecycle and dedicated
Mac migration continue separately from these UI limitations.

### 2026-10-19 — Reboot finds retained Homebrew identity defect (open)

The real guest reboot changed the APFS device number from `16777234` to
`16777229` while `/opt/homebrew` retained inode `165831`. Its saved creation
record uses device/inode, so public Check incorrectly blocks the retained manager
and its dependants after reboot. It leaves all 107 state/journal files unchanged.
The original record, failed Check and native stat/volume probes are preserved;
no ownership record is rewritten to bypass the failure. Apple's `fgetattrlist`
volume UUID query on the held prefix descriptor succeeds and matches the Data
volume UUID. A native disposable-APFS remount regression is being added before
correcting the durable identity. Other configuration-recovery parent identities
bind a fresh approval token rather than a long-lived ownership record; they must
not be widened as an incidental change. Actual Opus 5.5 xhigh consultation is
running. The previously passed full gate/review precedes this new finding and
must be repeated after its correction.

Reboot also cleared Code's first-run screen. Actual document typing and saving
now pass: the 0600 witness contains exactly `pr87 native vscode editor witness`
plus a newline. A fresh interactive login preserves all four before/after shell
markers; a noninteractive login correctly loads only the login-profile markers.
AeroSpace and the Tailscale service restart automatically; Tailscale remains
NeedsLogin. These successes do not waive the Homebrew lifecycle failure.

### 2026-10-19 — Durable Homebrew identity and DMG signature investigation

The native APFS remount regression fails against the old Homebrew identity and
passes with volume UUID/inode, while an outside volume at the same path remains
blocked. It checks unchanged ownership evidence. Opus 5.5 xhigh consultation
confirmed that the pre-reboot schema-1 record cannot be safely rebound by any
public recovery contract. The implementation queries the documented volume-root
descriptor after matching it to the held prefix filesystem, rejects missing UUIDs,
and explicitly refuses old ownership records and intents. New interrupted and
completed records use schema 2. Generic fresh-approval identities are unchanged.
The real failed record is preserved; a fresh sequential VM must prove bootstrap
and reboot with the corrected format.

Real Zen and Obsidian desktop launches report damaged applications with Gatekeeper
enabled. Deep vendor signature verification finds 143 and 231 synthetic AppleDouble
files respectively. The DMG-to-tar bridge generated these from filesystem metadata;
the bounded reader materialized them as extra files inside signed bundles. The
original payload archive is retained on the host and guest with SHA-256
`9a42481094add56321331981bf35fee168723b4c81fc9c6433ed0d8eab9fb2ae`.
OpenWhispr's ZIP has no such files and its signature and GUI launch pass.

DMG transport revision 1 uses Apple's `--no-mac-metadata`; it does not filter real
filenames, modify vendor signatures or remove quarantine. The pin identity causes
an ordinary approved Update to replace earlier generations. Revision 0 stays
available for exact saved-operation recovery. The native fixture was corrected
after diagnostics showed `hdiutil -srcfolder` itself discards real `._` files;
writing the fixture directly inside its mounted APFS image tests both the unwanted
metadata files and preservation of a real `._` filename. Earlier harness failures
are retained and are not claimed as product regression evidence. Native red/green
and public replacement/GUI validation continue. Actual Opus 5.5 xhigh is consulting
on both fixes; the previous full gate and clean review do not cover them.

The final corrected native DMG fixture now fails on the original bridge and passes
with revision 1, including exact preservation of the real `._` file. Public Update
plan `fcadedbe289a6fd546930072170430a2a1fa0ee18c097c7a03b51b5500931c36`
successfully replaces Zen and Obsidian and republishes their desktop links. Both
vendor bundles pass deep strict codesign with zero AppleDouble files. The plan
truthfully remains needs-action for the 17 Homebrew-dependent resources. Public
Abandon archives those unstarted pending choices without undoing completed work.
The repaired Zen launcher reaches its real welcome UI with Gatekeeper still on;
command-line spctl alone reports rejection despite identifying notarized Developer
ID, so GUI behavior remains the acceptance authority. Installed Neovim's configured
clipboard round trip and independent pbpaste pass, with the original guest clipboard
restored. This does not prove host sharing or GUI paste shortcuts.

OpenWhispr's onboarding states that continuing accepts its terms. The owner chose
to leave onboarding to them; microphone and paste remain a precise manual follow-up.

Upstream published Sentinel 0.2.0 during acceptance. The refreshed immutable pin is
`f90fc0d63b0af4927f07a0f230e602dbe432238c`, archive SHA-256
`21318e35ca808da7b67c8adc07f174c93bd412c893ca23fc493c4b83799558f8`.
The two intervening commits change release metadata and upstream bot activation,
not the vendored skills. Every skill byte matches the new archive. The actual
upstream renderer regenerates policy SHA-256
`bc866201ed01c9495c5f058af540db264e1a78ce09f2418d34371990bcb4c053`;
the bundle hash is unchanged. The version-update regression now derives its old
version from the pin and asserts that its simulated update really changes it.


### 2026-10-19 — Opus consultation follow-through for the Mac fixes

Actual Claude Code 2.1.301 with `claude-opus-5-5 --effort xhigh` completed a
read-only consultation; the provider/model receipt confirms first-party Opus 5.5.
The effort is evidenced by the CLI invocation, not a separate server echo. It
confirmed the volume-root binding, schema revision and saved DMG recipe handling.
Its legacy in-progress Homebrew-intent finding reproduced: Observe returned an
error that aborted independent resource discovery. The regression fails before
and passes after returning a resource-local blocked/pending observation without
resume authority. Original saved evidence remains unchanged.

The existing Mac core/desktop jobs now explicitly run the APFS remount regression,
DMG metadata regression and pinned desktop archive lifecycle with strict vendor
codesign assertions. The remount negative control isolates the UUID by matching
the outside directory's inode in private fixture evidence. The DMG test also
retains a revision-0 control: native tar creates the synthetic metadata entry and
discards the real AppleDouble-shaped filename, while revision 1 preserves the
complete original inventory. The consultant's expectation that revision 0 keeps
that real filename was contradicted by the retained native red evidence.

Linux Flatpak also persists the shared helper's old device/inode identity. Its
non-Darwin behavior is unchanged in this Mac fix, and the helper comment now
accurately names that consumer. Whether a supported Linux reboot changes that
identity remains an open follow-up requiring native reproduction; no Linux fix
or reboot guarantee is inferred from the Mac result.


### 2026-10-19 — Real Mac desktop interaction and Zen self-update finding

Tart pointer input recovered during this session; earlier pointer failures remain
historical evidence. Obsidian's repaired signed bundle opens through the actual
guest terminal/LaunchServices path with Gatekeeper enabled. The native vault
picker opens the pre-existing PR87 vault. A GUI edit appends a saved-note witness;
the exact resulting content has SHA-256
`c98ae8179ccb16d7d6230473d42207658f0047a01b275e1f3056dc985c00e6b6`
and retains mode 0600 and both earlier personal-note lines. Zen renders the local
HTML page and completes macOS's normal default-browser prompt. LaunchServices
records Zen for HTTP, HTTPS and HTML. A bookmark attempt did not create a saved
bookmark and is not passed. Shift/punctuation routing still corrupts typed URLs;
Zen's guest-pasteboard suggestion successfully opens the exact local T3 URL.

Zen then visibly reports an update and changes its installed vendor version from
reviewed 1.23.2b to 1.23.3b. Public Check correctly protects the changed package
and desktop link rather than overwriting them. Obsidian remains Keep after GUI
use. The changed Zen payload and personal profile are being preserved before
choosing a correction; its current README already discloses self-update and
profile-routing limitations. Actual Opus 5.5 xhigh consultation is running on
the smallest supported update-policy correction that preserves vendor signatures.
No Gatekeeper, signature or fingerprint bypass is authorized or used.

The earlier full gate compiled Sentinel 0.1.2 before the working-tree pin moved
to 0.2.0, then correctly failed its on-disk provenance comparison. It is not a
passing final gate. Fresh 0.2.0 source-proof/focused tests pass; final full CI
must use a frozen tree after the remaining acceptance fixes.

### 2026-10-19 — Code self-update and latest Sentinel native proof

Code's real native editor session updated its private signed bundle from pinned
1.141.0 to 1.142.0. The removal preview correctly retains changed Code and Zen,
plus their shared shell integration; no ownership assertion was weakened.
The complete Code failure archive has SHA-256
`65e02c14b5da45f9428534d0bd46637000e97e97e2dcaffe14259692353fc0b4`.
Zen's changed payload and personal profile archive has SHA-256
`d1fb9224b74e1931c6049313330ae02ee0637da47bcde3b1f7127a47bfd3009d`.

The supported Code correction owns only `update.mode: none` through a separate
JSONC profile resource. Source of truth is Microsoft's update-mode FAQ and the
observed sealed-payload change. The first focused regression failed because no
update setting was published and a prior personal value did not require adoption.
An initial direct addition to the old field list was rejected during inspection:
profile field identity is persisted in baselines/journals. The separate resource
keeps those old identities intact. Its first focused run exposed the need to
route the new resource through the existing JSONC parser/empty-document handling;
that wiring now includes both Code resources. No assertions were removed.
The native extension-host witness now verifies the consumed update mode too.
Native revalidation and final gates remain pending.

Sentinel 0.2.0 public Update publishes the exact pinned policy once in all four
consumer targets (0600); all 206 installed skill files match vendor bytes/modes.
The skill payload is unchanged from the already exercised 0.1.2 upstream snapshot.
The original Homebrew schema-1 evidence and changed Zen remain pending; public
Abandon archives the completed partial transaction without editing journals.
T3's real web interface discovers Codex 0.162.1 and Claude 2.1.287, both correctly
unauthenticated, and preserves Dark appearance after reload. No provider calls
were made. The user explicitly leaves OpenWhispr onboarding for themselves.

Opus 5.5 xhigh's read-only Zen consultation confirms there is no documented
per-install macOS update-policy channel that leaves the signature and other Zen
installations untouched. It suggests testing Apple's argument domain only after
proving typed policy values, restarts and every launch path. That alternative is
not implemented or claimed supported. Its source inspection used an independent
host Zen 1.23.2b bundle read-only, not a hash-verified guest artifact; native proof
remains necessary. This consultation is not the final implementation review.


### 2026-10-19 — Reject competing Code owners; preserve removal evidence

The full gate caught `TestShellProfilesHaveOneOwnerAcrossAllSelections` failing
for the separate update-setting resource described above. The assertion remains
unchanged. Opus also traced a possible stale-lock deadlock between those two
owners; that scenario was not reproduced and the competing-owner design is
rejected. The current correction combines theme/font/update fields under one
successor. `integration.vscode` retains its exact historical field identity as
a recovery-only predecessor, restored through the existing approved handover.
The successor remains blocked while predecessor ownership is created/uncertain.
Changed predecessor settings require explicit restoration and preserve the old
file; new adoption is previewed only after restoration. A failed-before regression
records the unsupported handover, then exercises both unchanged and changed old
receipts, personal settings, permissions and retained edit evidence. No installer
state or journal is rewritten to upgrade it.

Fresh Remove completed with 37 personal/failure witnesses unchanged. Starting
Tailscale with Homebrew's root service command had changed the exact executable
directory ownership, as Homebrew warned. Normal stop did not revert ownership;
uninstall partially removed the receipt before failing. The original state
archive SHA-256 is `02179f231c7be0239fd85192ae753fa856ffac7849a22642c930514b739468a9`;
the partial keg is `a63c9dbad7e10080c2d5b0eb4a058c9950e436de612bd623fc168634c9f5c911`.
After preserving both, restoring ownership only on the two verified warned paths
and native exact-formula uninstall, public Retry completed. The first Retry
correctly refused changed native receipt/classification. Shared Homebrew/CLT and
unrelated hello remain. This is an operational Homebrew service caveat, not an
automatic-removal pass. Code/Sentinel/skills public Reinstall then reached Ready.


### 2026-10-19 — Follow latest Sentinel maintenance revision

Upstream advanced during Mac acceptance to
`6645688d08f1505595cae276832a5a5070b82160` (still version 0.2.0), importing
Impeccable's browser-context fallbacks/download interruption cleanup and typography
guidance, plus Unslop wording. The owner explicitly requested latest Sentinel.
The new source archive has SHA-256
`10f3d72b1d61a7c7afeb35b981db630ff2a5b007bb1b11fceb29e682810a267b`.
Six vendored files change; core policy, twelve skill names, licenses and engine
0.1.15 pin remain unchanged. Bytes are copied from that exact upstream archive,
with regenerated per-file provenance. Source proof and native installed skill
checks must be repeated; the earlier skill runtime evidence names the prior pin.


Opus 5.5 xhigh consultation of the single-owner revision (session
`5d33bec9-24fd-4c52-a3c7-777243e16aff`, CLI 2.1.301, read-only tool set) found
incorrect handover guidance for personal restored settings: Resume cannot widen
its saved adoption list. The adopted-theme and personal-update regression fails
before correction. Guidance now explains Abandon followed by Choose tools and a
fresh explicit adoption. The historical six-field identity is independently pinned;
current settings use separate literals. Restoration copy preservation applies to
unchanged files too and is now documented/tested. The legacy restoration choice
no longer names Git for a Code operation. The one-owner assertion is strengthened
to compare predecessor/successor paths and field containment. Missing old evidence
remains blocked with recovery guidance and a non-mutation regression.

Microsoft's 1.141.0 update configuration declares `update.mode` as application
scope, without a sync exclusion; its Settings Sync guide excludes machine scopes
by default. The Code selection/preview and guides now disclose propagation to
other synchronized machines and the user's ignored-setting control. No account
or sync setting was changed by acceptance. Native Code main.log directly reports
`update#disable` due to the user preference and disabled updater state. The
ad-hoc extension directory was rejected by Code's installed-extension registry;
that attempted witness is not counted as a pass. Payload persistence/removal
preview and final native acceptance remain to be completed.


### 2026-10-19 — Native preservation results and released Sentinel correction

Code's normal desktop session kept all 3,809 payload entries unchanged for 602
seconds and after normal exit. Its native updater log reports updates disabled
by the managed preference, and public Remove previews removal of the owned app.
The real old theme/font receipt then handed over to the single combined owner
through public restoration and Resume. Code's four personal settings and 0640
permissions survive. All 37 original personal/failure witnesses remain exact.
Public Check is Ready and leaves 14,267 state entries unchanged; unchanged Update
is Ready with only keep/retain/forget operations. Latest Sentinel's 206 installed
files match pinned bytes/modes; its engine setup/context/detect and browser crypto
fallbacks run successfully. Consumer rediscovery at the new pin remains open.

Full local `make ci` passes the formatted candidate, including race tests. Actual
Claude Opus 5.5 with `--effort xhigh` reviewed that candidate read-only (session
`d6f10de8-ee6e-4a70-bd30-dc91d7489a6e`, verified `claude-opus-5-5`). It found one
material defect: an old removed/reused Sentinel receipt still validated its
three-target history against the new four-target recipe, permanently blocking
the documented removal/reinstall path for adopted policies and preserved edits.
The regression creates genuine profile receipts through adoption, optional edit
Repair and removal, then retargets. Both cases fail before correction. Released
Sentinel ownership now follows the existing Git rule: inspect current targets and
require fresh approval/baseline, while retaining prior evidence disclosures.
The corrected transition and still-owned-retargeting refusal pass. Tests verify
personal contents/modes, unchanged original journals and retained edit copies.
Two stale guide statements were corrected: current Code owns seven fields, and
the earlier Sentinel pin is explicitly historical. Native public-wrapper proof,
renewed review and final-commit gates remain outstanding.

The first VM was cleanly shut down and deleted only after exporting and checking
all final state/personal entries and acceptance logs. The full state archive is
`c040be2aa0bca1cfcc072fb47116466802dc833deb1444351d32ec6ce8c4fa6c`;
14,824 entries were validated against the live guest before shutdown. The log
archive is `91d5ecc18fbb5a10b924c072b5ccf0d638968be97551ee26f002467a3fefca0d`.
Private T3 pairing output is excluded. The pristine replacement uses the original
immutable vanilla image digest; only one local test VM is retained at a time.


### 2026-10-19 — Public Sentinel retargeting and current interactive bootstrap

The pristine replacement has a new verified guest identity, the same macOS
26.6.2/25G83 arm64 image, no CLT/Homebrew/Nix and 19.0 GB free in its APFS
container. SIP remains enabled; Gatekeeper was enabled through its normal
command before further acceptance. Both Tart and its documented Screen Sharing
transport currently return `cgWindowNotFound` to the UI tool. SSH/PTY works;
desktop interaction remains an external automation limitation.

Actual `./setup.sh` from d9a3993 adopted three personal policy blocks, preserved
a controlled managed edit through Repair, and removed the old selection to
Ready. The unfixed candidate then refused explicit adoption and reported the
target-count mismatch in an ordinary preview. Both refused requests leave all
30 state/policy entries unchanged. The corrected public workflow approves fresh
adoption, reaches Ready with four targets, then removes to Ready. All 29 original
entries other than the legitimately updated state.json remain byte/mode/owner/
link-identical, including every earlier journal, baseline and edit copy. All 12
old preserved paths remain disclosed. Three personal policies retain their
original hashes and 0640 modes; the newly created Copilot/Kiro policy files are
removed and the older OpenCode policy is untouched. No journals were edited.
The pre-transition archive is
`f7ad8bafeb821192d3457486df1b58b96d82f3c872874893837b5b27fc51b1a4`;
the post-transition archive is
`cad31f5ff5799a7a47d9e8ba32a828dddc1ff5ff8a7855f343079e1fc931ad11`.

The exact final regression was also rerun in an isolated copy with only the
production condition reverted; both cases fail at the adoption assertion.
The test source matches the passing current test byte-for-byte. Actual Opus 5.5
with xhigh effort found no material issues in the correction (session
`dfb5f162-12ff-479d-bb7f-e88d6a679f1f`). Full `make ci` passes again, including
640.313 seconds of installer race tests. This does not replace final-head CI.

Current interactive `./setup.sh` shows 27 Mac choices in 35 category/child rows:
category toggles, mixed Editor selection and independent children work, both
Sentinel choices and AeroSpace appear, and Windows Terminal is absent. Reviewing
the exact tmux plan and choosing Back leaves state bytes unchanged and provisions
no prerequisites. A second identical plan was approved through the real terminal
and ordinary administrator authentication. Its public bootstrap is provisioning
CLT/Homebrew; corrected reboot and subsequent migration acceptance remain open.


### 2026-10-19 — Corrected pristine bootstrap survives actual reboot

The approved interactive public bootstrap completed with Apple CLT
27.0.0.0.1788430756 and Homebrew 7.0.9; its mandatory follow-up preview installed
tmux 3.8 and reached Ready. No compiler, Homebrew or Git had been preinstalled
for automation. The schema-2 bootstrap record identifies APFS volume
`a19fe379be6242979a393934cc88214b` and prefix inode 384173. Ordinary reboot
changed the observed device number from 16777234 to 16777230, reproducing the
original failure condition. The corrected record remained byte-identical and
Check stayed Ready. All 201 state entries match across reboot and each read-only
Check; five personal shell/policy witnesses retain their hashes and modes.
A fresh zsh login finds tmux 3.8. Public unchanged Update
`53ab91197274da3d71a758b2525343ff2b8da72aa72bbfbc72e0d3bdc3e56e4a`
also reaches Ready with the entire Homebrew version inventory unchanged.

Latest Sentinel commit `6645688d08f1505595cae276832a5a5070b82160` was rechecked
against upstream main. Public installation in the replacement guest reaches
Ready. Installed Codex 0.162.1 and Claude Code 2.1.287 each discover all 12 skills
exactly once plus the personal test skill; neither discovers hidden recovery
copies. All 206 installed skill files match the pinned source bytes and modes,
and all four policy targets contain exactly one current policy block. Both
installed model-discovery helpers run with `accessVerified: false`, and the
verified Impeccable 0.1.15 setup/context/detect helpers run. No authenticated
provider call is claimed. Public Check leaves all 10,237 state entries unchanged.

The same current selection then removes to Ready with explicit shared-resource
choices (`5f0de6fac2b3f3705f0fd247caae965bc18f2172c0be757b29434a672c3c49f7`).
Code restores personal `update.mode: manual` while keeping its original font-size
setting, original personal key and later personal key, all at 0640. Three personal
policies retain their exact original hash and 0640 modes; both personal skills
and the 0640/0600 shell files survive. All managed skill folders are removed;
the verified personal Impeccable cache remains. Homebrew inventory contains only
the deliberately unrelated `hello` 2.12.3; Homebrew and CLT remain installed.
Read-only Check stays Ready with all 186 final state entries unchanged.
All 339 final state/personal entries and 119 acceptance logs were exported and
verified against the live guest, then checked again after transfer. Their archive
hashes are `c53bbe52b1a16d03f3b9e097890d17598e4ffa192a33899db4108094d7bcd7eb`
and `1ec0af45db74f2c83a9c8b9ff81581908093fabf6f95cf7e5f93d2909c08c02c`.
The guest was shut down cleanly before its replacement migration clone.


### 2026-10-19 — Genuine released Mac fixture reaches first-launch permission

The sole replacement VM, `dotfiles-pr87-migration`, starts from the same
immutable vanilla image with no CLT, Homebrew, Nix or installer state. After
ordinary CLT/Homebrew provisioning, the checked-in Darwin fixture's exact core,
cask and AeroSpace tap snapshots were checked out and historical Ghostty installed.
A clean clone of released v0.4.4 at `05874e536372f6a73f8971c84e675e95666662d4`
ran its unmodified `./setup.sh --all --skip-nvim --skip-agents`. Its authentic
upstream Nix 2.34.0 installation created the APFS volume/build accounts, and the
locked nix-darwin activation produced Nix-managed Homebrew and the released tools.

The old installer is currently waiting at Code's extension installation. A native
process sample identifies its blocked first launch, and macOS syspolicyd records
“Prompt shown ... waiting for response” for Microsoft `com.microsoft.VSCode`,
team `UBF8T346G9`. The UI tool still returns `cgWindowNotFound` for Tart, so an
ordinary guest Open action was requested from the owner. Gatekeeper remains
enabled; no quarantine, signature or permission bypass was attempted. This is
an external interaction blocker, not a completed released installation or a
migration pass. The complete historical installer and security logs are retained.


### 2026-10-19 — Authentic Determinate retirement, killed controller and reboot

The blocked released VM was cleanly shut down and exported before deletion.
Apple Archive verified every stopped disk/config/NVRAM entry against that export;
its SHA-256 is `ad75f1277d231bc83331526b7ea64bf96cd5674467e17eb3b97b5e69fc9d5655`.
Only one test VM was registered at a time. The immutable vanilla image remains
`eeec54bfe1f076e27786c5d92b89187a05b1d109b5071eb2dcdf02d596e34640`.
The replacement Determinate guest is macOS 26.6.2/25G83 arm64, 4 CPUs/8 GiB RAM,
with 50 GB disk and sufficient free APFS space; no resizing or host share was used.

Official v3.23.2 installer SHA-256
`136f40a09ac56e3638530d57099d748c1023677911eb5afd45a4ba438986b648`
created the authentic receipt, encrypted Nix volume, native daemon and 32 build
users. The installed binary retained its normal admin UID 501 and 0755 mode.
Its modified-byte refusal, outside-account profile refusal and unreadable system
configuration refusal all pass through public migration. A live sleep process
with cwd in `/nix/store` blocked the approved vendor command after daemon stop;
the original binary/receipt hashes remained intact. The failure archive and
original journal were retained. Only that test process was then terminated.

Public Retry `40161836f3aa5c0210be49f243640a84816a487643561bd7363fede832727f0d`
started the vendor worker. After its durable command record appeared, controller
7256 was killed; the independent worker completed with exit 0. Public Retry
`830cdb5115617576cafd00568787b733297b22d049dc1e4441e0cd1ac9f549e7`
recovered the saved outcome, and the subsequent reviewed empty recovery reached
Ready. No installer state or journal was edited. Store, vendor executable/receipt,
Nixd, build accounts/group, volume UUID and boot references are absent. User Nix
configuration keeps SHA `d67bac055884461fba448aa83c60bc7e93dbeaef8dcf233b140c5798fa962baa`
at 0640; preserved system custom configuration keeps SHA
`dd68cf01b8528bfa020b6a755adc2c7404a767007c8d0ffd3dfe91b0e55136b9` at 0644.
The separate root-owned 0640 fixture was refused; changing that fixture to 0644
allowed the independent readable case and is not evidence of migrating unreadable
configuration. Root Nix files/cache and vendor-generated `/etc/nix` remain disclosed.

A newly created personal file at the former daemon plist path was protected by
preview and attempted apply, preserving its hash/mode. Removing only that test
file and approving public Retry returned Ready. Ordinary reboot changed the boot
UUID from `0F152A04-E7DF-45DF-8A82-9634203FBD31` to
`A77F8B84-1339-4691-881C-DF8E41A3BF67`; the synthetic `/nix` mountpoint disappeared,
fresh zsh login works and runtime Check stays Ready. Reboot preserves every entry
in the state/personal archive, and read-only Check preserves all 28 installer-state
entries. The final 141-entry export was compared with a second live export:
SHA `33627a9ebefcf17805dee8beb17a7ef8cc8d8a2fc20763930abd424d69e5aa51`.
The guest was shut down cleanly. Reboot normally removed the temporary privileged
vendor staging directory; original authenticated input bytes and the pre-removal
failure archive remain retained. A first export naming that expired temporary
path failed and was retained separately; the verified final export lists only
existing retained paths.

No new production fix was needed in this pass. The documentation-inclusive full
`make ci` gate passed again. Source comparison confirms all production/test bytes
are identical to the earlier Opus-reviewed source6 and native-tested transfer;
subsequent changes are documentation only. Latest upstream Sentinel remains
`6645688d08f1505595cae276832a5a5070b82160`. Full released Mac migration remains
blocked at the ordinary Code permission prompt; its incomplete installation,
Homebrew conversion and upstream-Nix retirement are not claimed as passed.


### 2026-10-19 — Final-head Windows fixture diagnosis

Candidate `2b5342c74328f8d1fdd036d625526b360cb117fb` exposes two new Code
fixture failures (six cases) on native Windows: they require literal POSIX 0640
from Go's Windows file mode. The requested chmod succeeds, but Go documents only
the writable/read-only attribute there. The fixtures now capture the original
native configuration snapshot and require exact restoration of its contents and
mode on every OS, retaining the explicit personal-text assertion. No Windows
skip or production ownership change is introduced. The original failed job log
is retained; focused Mac tests pass and corrected Windows proof is pending.

Rejected provisional diagnosis: the engine suite did not time out. Its recorded
1164.385 seconds completed within the unchanged 20-minute limit; the failures are
the mode assertions above. No timeout was raised. The previous successful Windows
suite took 795.953 seconds before the additional Mac/Sentinel test coverage.

Separately, Windows quality failed before tests on three fresh runners because
Chocolatey's Neovim metadata requests returned HTTP 503, 503 and 504. A native
Chocolatey retirement fixture also found no installed Hyperfine after the install
command returned success; its prior success output was discarded. The fixture now
uses the existing bounded/redacted command helper and records success output too,
while retaining its original install, ownership, consumer and removal assertions.
That missing-package result is not yet attributed to a production defect.


Windows quality now provisions the same reviewed Neovim 0.12.5 Windows ZIP as the
installer, using its exact SHA-256 before extraction and prepending only a private
runner-temp directory to the job PATH. This removes an unpinned intermediary from
quality setup, not from the independent Chocolatey retirement acceptance. All
Pester/Neovim assertions remain. The expanded pin-consistency regression fails
with `NVIM_WINDOWS_VERSION` missing before the workflow change and passes after
both version/digest are present. The verified 12,623,588-byte official archive has
2,235 entries, root `nvim-win64`, its real PE executable and required runtime file;
SHA `de8625ba8cf65ebf40eb80a388ba1ec8e9c15b30218821e2c639119b05920de1`.
The existing remote-execution supply-chain check passes unchanged. Actual Windows
quality execution, full gate and renewed Opus review are still pending.

On the same original candidate all six Mac checks pass, including the native
APFS remount regression (15.56s), genuine hosted v0.4.4 Mac migration (430.58s),
and authentic hosted Determinate retirement (205.74s). This does not replace the
separately blocked Tart released-installation permission workflow.


### 2026-10-19 — Windows CI consultation and final regression pairing

Actual Claude Opus 5.5 (`claude-opus-5-5`, CLI 2.1.301, `--effort xhigh`;
session `308b8f80-ab7b-407f-9e2d-0091ddf5a12c`) independently confirmed the native
mode diagnosis and unchanged timeout. It recommended reading the production
archive pin directly, preserving the real Chocolatey fixture, checking that its
package exists before retirement and quoting failed personal-data comparisons.
Those recommendations are implemented. The earlier workflow-env pin experiment
above is superseded: no additional version/digest or Renovate input remains.

The final pin regression, SHA-256
`41d534601e86484659e1d0171f099625119b4c4327be74b98932eec64071f3f2`,
fails against the committed `2b5342c` workflow with “Windows quality must use the
installer Neovim pin”. Changing only that fixture's workflow to the final version
makes the identical test pass. The unchanged remote-execution supply-chain check
also passes. The actual Windows runner still needs to prove the verified archive
launch, resolved executable, version and complete quality suite.

This follow-up changes CI, tests and documentation only. Production bytes remain
identical to the Opus-reviewed source6 tested in the Mac guests. The Chocolatey
Hyperfine failure remains unclassified until the new diagnostic captures its
provider output; the observed Neovim HTTP failures alone do not prove its cause.


### 2026-10-19 — Final Windows CI review

Actual Claude Opus 5.5 (`claude-opus-5-5`, CLI 2.1.301, `--effort xhigh`;
session `24b2a45a-f861-48ce-86c4-c88955a41946`) reviewed the complete seven-file
follow-up and found no material findings. It confirmed that the native mode
snapshots preserve the original Mac/Linux 0640 assertion and the observable
Windows mode plus exact bytes, and that the Chocolatey postcondition strengthens
rather than bypasses retirement acceptance. Windows execution remains required.

Two evidence wording corrections from that review: the existing command helper
redacts configured GitHub-token text on failure; the added success diagnostic is
bounded to the last 8 KiB, but is not redacted. This fixture sets no GitHub token.
The previous 1164.385-second Windows suite had only about 36 seconds of headroom
under its unchanged 20-minute limit. New Sentinel fixtures contribute about
89 seconds; unrelated tests also ran materially slower on that runner. This is
a recorded rerun risk, not proof of a hang or justification for skipping work.
The original missing Hyperfine package still cannot be attributed to an outage
without the discarded provider output. A later read-only metadata request
returned HTTP 200; only the new native run can prove installation and retirement.

The final follow-up passes full `make ci`, including 604.809 seconds of installer
race tests and the unchanged native-mode snapshot assertions. The focused six
Code preservation cases also pass on Mac. An earlier gate attempt found five
generated Python bytecode files left by a preceding run; only that exact recorded
set was removed, and the complete passing rerun disabled bytecode generation.
No test or lint exclusion was added. Corrected native Windows and the next exact
commit's 23 native/quality plus four CodeQL checks remain required.


### 2026-10-19 — resumed Mac acceptance and historical prerequisite failures

Owner scope clarification: application onboarding/authentication, OpenWhispr
audio/paste, physical shortcut routing and Tailscale enrollment are user setup,
not installer acceptance blockers. Preserve previous observations as evidence;
do not claim those workflows passed. Sentinel needs no upstream fix or new tag
for the completed dotfiles integration corrections. Its latest upstream HEAD was
reverified as `6645688d08f1505595cae276832a5a5070b82160`.

The archived migration VM was restored after verifying its SHA-256, with only
one registered test VM. Tart scrolling crashed in Virtualization.framework's HID
handler; both owner and automation observed the subsequent frozen input. Clean
shutdown and Screen Sharing restored control. The owner-approved ordinary
App Store & Known Developers setting allowed VS Code to launch through Finder;
spctl accepted it as Notarized Developer ID. Gatekeeper and SIP stayed enabled.

The unchanged v0.4.4 installer then failed on fontconfig/python@3.15 post-install
steps unsupported by its Brew 6.0.1 and on setuptools 83 under Apple's Python
3.9.6. A preserved rerun with NO_INSTALL_FROM_API reproduced Resource::Patch
DSL incompatibility. This corroborates, rather than reverses, the earlier
rejection of that flag for the complete released setup. The failed venv was
not edited or removed to force progress.

Actual Claude Opus 5.5 was observed in consultation session
`977fa28d-5a16-4ea0-b876-0731d8d796ce`, with CLI `--effort xhigh`. It rejected
using the mixed failed installation as the healthy legacy baseline. The original
full checkpoint remains. A further archive with SHA-256
`80eccaf1308897533f5847f11fa0ca2ae45e6cc3257c7a1a5373016f15161125` retains
32,224 verified entries, including failure logs, native receipts, failed Python
environment, personal configuration and the generated fontconfig artifact. The
failed VM was shut down cleanly and replaced with one fresh clone of the same
immutable vanilla image.

The fixture correction installs real fontconfig, yamllint and Python 3.12 using
the current Homebrew and historical taps before the original release takes over,
verifies the actual commands, and exposes Python's documented libexec PATH. It
does not change historical source or synthesize legacy state. Native preparation
passes with fontconfig 2.18.2, yamllint 1.38.0 and Python 3.12.13. Full released
setup and migration are still running at this entry. Baseline `make ci` passed
(engine race suite 767.008 seconds); correction and final review remain pending.


### 2026-10-19 continuation: released retry and native Zen implementation

- Actual Opus 5.5/xhigh consultation `dd3c9fa1-b36f-47cb-98dd-6b419f47430e`
  accepted an ordinary unchanged retry as authentic two-run installation evidence,
  requiring native health checks and a separate fresh fixture test. The retry
  exited zero. Subsequent inspection found the OpenSSL placeholder link defect;
  it remains preserved and disclosed, not a clean-fixture pass.
- The public profile migration recognized and detached 18 released artifacts.
  Its exact approved plan is `c14a1862dc657a9f64fb275772c56eba5b33a1a4fb906ff2437eff6b652569b4`.
  Personal witnesses have modes 0640 and 0600. Before-conversion Homebrew inventory
  contains 55,366 entries; comparison remains in progress.
- Actual Opus cask consultation `f6028001-11b1-4fde-925a-73c51de95a13`
  recommended the canonical cask inside the existing Brew provider. The earlier
  install-and-relinquish proposal was rejected because it discarded Remove.
  Implementation keeps approved native removal, signature identity, no browser
  downgrade through Update, no profile edits, and unchanged archive recovery.
- Official Homebrew 7.0.9 source confirms `CommandWrapper < Binary`; `--no-binaries`
  avoids claiming the shared command. Its Symlinked uninstall still unlinks foreign
  symlinks, so an existing command blocks owned-app mutation. Installed cask JSON
  and appdir are checked because uninstall loads that recipe, not the current API.
- Failed-before regression: cask badge rejected as an invalid keg summary.
  Focused passing tests now cover lifecycle, reuse, self-update identity, personal
  file preservation, foreign target protection and broadened-intent rejection.
  Native proof, independent final review and full gates are still pending.

### 2026-10-19 — native cask boundary corrections

The real disposable Mac cask fixture passed install/reinstall, operation badge
attribution, no shared command, inherited appdir isolation, actual codesign and
uninstall. It exposed codesign's literal `=` requirement prefix. The subsequent
real Zen installation exposed Homebrew 7's expanded API paths and compact cask
metadata; failed-before regressions now cover the authoritative native tab
receipt without admitting executable uninstall hooks or redirected targets.

Real Zen 1.23.3b then installed successfully but failed the extra all-strict
signature probe because the official bundle retains FinderInfo. Gatekeeper
accepted the unchanged bundle as Notarized Developer ID, and the complete team/
bundle signature requirement passed with Apple's explicit sealed-symlink check.
The failure journal and original app remain preserved. Native failed-before
regression accepts legitimate Finder metadata while separately requiring rejection
of changed Mach-O bytes, changed sealed resources and an outside symlink. The
correction uses `--strict=symlinks`; it does not remove attributes, clear quarantine
or re-sign code. Source of truth: the guest's Apple `codesign(1)` strict-option
contract and [Apple QA1940](https://developer.apple.com/library/archive/qa/qa1940/_index.html).

The full gate also caught the older desktop-choice test expecting the archive
provider on macOS. Its assertion now requires app.zen there and tool.zen on Linux.
Native fixture setup preserves the runner's already approved Homebrew trust in
its isolated configuration, and compares cask inventory stdout consistently;
deprecation diagnostics cannot masquerade as package changes. Final native
recovery, review and full gates remain pending at this entry.

### 2026-10-19 — Opus implementation consultation and real Zen recovery

Actual Claude Opus 5.5, selected with CLI `--effort xhigh`, completed implementation
consultation `7598531f-9533-4375-a1c7-412bdd268a79` and signature consultation
`432f1c5a-f032-4063-921e-fcf9cd977416`. Neither is final approval. Native comparison
accepts FinderInfo while both strict policies reject changed executable/resources,
outside symlinks, extra root files and unsigned nested code. The actual Zen bundle
rejects a different developer requirement with exit 3.

The failed Zen app archive has SHA-256
`f91bb4e274223c72a61c2dc273c0c3efd22862a75cfb04c164a7e3e9fc93e635`;
its original installer state archive is
`e957127ce6144a6a791820f314dd79f874778928830c1ca4c86c76e24997ff57`.
Exact native uninstall, with autoremove disabled, restored all 47 original package
versions and preserved all unrelated native metadata. Public Abandon archived the
failed operation. The corrected public handover then completed and Zen reached
its welcome screen after the normal macOS Open prompt. Personal browser witness
and modes remain preserved; Check left all 154 installer-state entries unchanged.

Confirmed Opus findings have failed-before regressions: inherited NO_EMOJI hides
Homebrew's operation badge; inherited cask appdir affects queries; an existing
user cask was not reused; unknown signature inspection escaped to a global error;
GUI-only selection published a shell command directory; and cask removal exempted
a promoted formula pool entry from version preservation. Schema 4 binds the new
cask output control while schema-3 command recovery remains exact. Formula intent
format is unchanged. Focused passing reruns are recorded separately from the
pending final native acceptance and required full gates.

The current-recipe removal concern is rejected by actual Homebrew 7.0.9 behavior:
after install, the native fixture adds a delete stanza for an outside witness to
the current Ruby cask. Uninstall uses the saved receipt and preserves that file.
`CaskLoader.resolve_installed_artifacts` returns nonempty recorded artifacts before
considering fallback; the validator requires those artifacts and forbids flight
hooks. The receipt leaf's symlink check plus its already-bound metadata parent
also rejects the proposed parent-redirection concern. The native no-sudo case
fails promptly at a non-writable app directory without installing there, then
normal install/reinstall/removal succeeds. Real SSH removal detects the open
console Zen instance, refuses mutation and retains its recovery journal.

### 2026-10-19 — formula attribution and personal browser Repair

The public missing-app Repair reinstalls the official Zen app and preserves all
225 personal browser entries by contents, modes, ownership and symlink target.
The witness includes an edit made after installation with mode 0600.

The inherited NO_EMOJI issue also affects formula summaries in Homebrew 7.0.9.
A real native worker test installs a unique local formula, but completion fails
with "Homebrew did not prove introduction of this root" before the correction.
New formula intents therefore use schema 4 too. Earlier formula and cask schema-3
intents retain their original environment and command hash; legacy-shape tests
must continue reading their completed operations. This supersedes the preceding
entry's statement that new formula intents remain unchanged. Native rerun and
final integrated gates are recorded separately when complete.

### 2026-10-19 — v7 Opus review and upstream-Nix retirement

Actual Opus 5.5 final code review, CLI `--effort xhigh`, session
`48e7900d-a8b2-4fa4-ac89-b39734250c3b`, found no high or medium defects and three
low findings. The command-path inspection escape and missing personal-cask
recovery message reproduce through failed-before controller tests and are fixed.
The cross-user running-app concern is under native investigation; it is not yet
closed. The no-sudo evidence caveat is resolved by Homebrew 7.0.9's explicit
`EnvConfig.no_sudo?` and `SystemCommand.sudo_prefix` checks. The native refusal
fixture now first requires working non-interactive administrator credentials.

The v7 full local gate passed (engine 635.245 seconds). Actual user-app reuse and
deselection preserved all 144 app entries and 227 personal browser entries.
Authentic upstream Nix 2.34.0 retirement passed outside-consumer and unknown-root
refusals. After durable APFS deletion dispatch, only the controller was killed;
the worker succeeded and public Resume completed all 76 steps without repeating
deletion (same native record, attempt zero). All 32 build accounts, group,
services, system links and boot references were removed. Original shell files,
personal contents/modes and an unrelated package/tap remained intact. Original
interruption archive SHA-256 is
`c778c86c5f81e68ff7c42db848f08a58bd33b9f01fc7af78625832e814cdd55a`.
The final remaining-work approval completed retirement. Reboot, repeated
migration, the fresh corrected release fixture and renewed final gates remain.

Adding the unrelated local tap exposed a native-fixture defect: Homebrew exports
its approved local formula as a canonical source path, which `brew trust
--formulae` cannot import. The failed native rerun is preserved. Fixture setup now
copies those already-approved sets into its private native trust store and checks
the result through Homebrew's own JSON export. It never alters the runner's trust
or grants broader sources. The original runner/package inventories still must
match after cleanup.

The proposed cross-user running-app blind spot is rejected by native evidence.
An ordinary separately authenticated SSH account (UID 502, audit session 100108)
launched the authentic shared Zen application with its own browser profile.
The unchanged production NSWorkspace query from admin UID 501 reported
`running`. Earlier UID-0 probes did too, but shared admin's audit session and
were insufficient by themselves. The distinct authenticated session provides
the counterexample; no speculative all-process scanner was added. This proves
cross-account detection in the tested session arrangement, not an unperformed
physical-Mac fast-user-switching GUI workflow.

The corrected native cask fixture passes in 6.49 seconds, including the working
sudo precondition, explicit no-sudo refusal and outside package/tap preservation.
The preserved real Zen bundle that updated from 1.23.2b to 1.23.3b also passes
the exact new Developer ID signature requirement. The public missing-personal-app
preview now gives the native reinstall/uninstall recovery instructions; restoring
the app leaves all 227 personal entries unchanged.

Upstream-Nix reboot verification passes: the volume/mount, accounts, service and
boot integration stay removed, fresh TTY login loads the personal override, and
a real tmux session works. Check preserves all 788 installer-state entries.
Repeated profile and runtime migration both return Ready with all 793 state and
profile entries unchanged; completed detachment does not replay. Seven original
system/personal baselines and the unrelated package/tap remain intact. Running
Check from a separate exported source folder correctly reported its different
configuration source; the installed source passes. This was a harness-path
mismatch, not a reboot or preservation defect.

Renewed actual Opus 5.5 review with CLI `--effort xhigh`, session
`99fa9d6a-7a66-4811-8268-d23b62291f6d`, found no remaining high, medium or low code
defects in v8. It accepted the L1/L3 corrections, cross-account counterproof and
isolated trust-store validation. Pending downloaded-update coverage is optional:
no unsafe code path was identified, and the actually applied vendor update is
verified. Exact-head checks and the clean one-pass release fixture remain gates.

Its optional fixture nit identified a formula baseline combining diagnostics
with stdout, unlike cleanup. Both formula and cask inventories now compare only
stdout, retaining command failures and full package equality. A native regression
adds diagnostic stderr at the process boundary while executing every actual
Homebrew query; native failed-before/passed-after results follow separately.

That regression fails against the original fixture in 3.01 seconds with
"native fixture changed unrelated installed formulae" and passes after correction
in 2.11 seconds. The real inventory is unchanged in both runs. Full v8 `make ci`
passes (747.059-second installer suite); the final fixture-inclusive gate is
running separately. The completed upstream guest was shut down and exported as
`upstream-retirement-final-checkpoint.tvm`, SHA-256
`52593d558c22e03419a9fe7d513b665dd19e730178179238b02893a0f0fa8194`,
then removed before the next clean VM was created. Its 311 diagnostic files are
also preserved in archive SHA-256
`d5669e44f19ea2b6bff4e48f97ab58a3aab992c8b4c821b60fb94a165004ea82`.

Final check-selection inspection caught the signed-app regression outside the
existing Mac job's `TestNativeBrew` filter. Its name now follows that provider
prefix; the test body and all six native assertions are unchanged. Before/after
test-list evidence verifies selection by the exact hosted command. The existing
23-job matrix is unchanged; the Mac Homebrew job now also executes this regression.

The v9 delta review identified one remaining test-only comparison in the full
transaction contract: its final `require` still combined stderr while the shared
baseline was now stdout-only. That read now uses the same stdout-only query,
with errors still fatal. The diagnostic regression now runs the complete native
transaction/removal contract, including its final assertion and cleanup, instead
of fixture construction alone. Native red/green binaries are retained for the
next Homebrew 7 test opportunity; the current first-run legacy installer is
waiting at the guest's ordinary Code permission prompt, so package mutations
remain serialized. This additional native rerun is not yet claimed as passed.

The fresh one-pass guest uses the same immutable vanilla image, macOS 26.6.2
build 25G83 arm64, 4 CPUs and 8 GiB RAM. Its pristine baseline lacked CLT,
Homebrew, Nix and installer state. Official bootstrap provisioned CLT 27.0 and
Homebrew 7.0.9; the corrected pinned prerequisite preparation completed with
exit zero. Unchanged v0.4.4 then installed authentic upstream Nix/nix-darwin,
Nix-managed Homebrew, native tools and VS Code without the earlier certificate
failure. It is still in its first invocation, waiting at Code extension setup.
Native assessment returns exit 3, `rejected`, `source=Notarized Developer ID`.
The owner has approved the normal known-developers setting, but Screen Sharing
returns `cgWindowNotFound` to the UI tool. Manual interaction is requested;
the release fixture is blocked, not passed. The guest's current `spctl(8)` manual
also confirms that direct rule-editing flags are unsupported since macOS 15.
No policy-database edit, quarantine removal, Gatekeeper bypass or historical
installer modification was used to get past this permission.

The full v9 and v10 local gates pass (645.518 and 630.775-second installer
suites). Actual Opus 5.5 with CLI xhigh reviewed the v10 delta in session
`5409e740-1806-45dd-baae-985222f32a3e` and found no material findings. A separate
byte comparison confirms all 176 production Go/JSON files are unchanged from
the fully reviewed v8 snapshot; subsequent changes are native tests and evidence.

The complete diagnostic-wrapper native transaction now has real before/after
proof on Homebrew 7.0.10: the old final comparison fails in 100.73 seconds at
"native contract changed unrelated installed formulae"; the correction passes
in 86.26 seconds. Both runs use actual native operations and cleanup. Afterwards
only the pre-existing `pr87-personal` formula and `pr87/personal` tap remain;
their bytes, commit and native trust survive. All seven personal/system baseline
witnesses preserve contents, permissions, ownership and original absence.

The UI-blocked guest was cleanly shut down, preserving its stopped disk, config
and NVRAM through an APFS clone before removing its Tart registration. Its disk
SHA-256 is `1e1f05d767f68fc73ae85cd34a3c4af772522acf196295f84d0042aba4e9d425`;
its separate 20-file diagnostic archive is
`24d5b42c04c6fbcb36dd64c9967859543ee1d432a9792f0238b1e05623208478`.
This preserves the interrupted first invocation, not a completed installation.
Only one local VM remains registered at a time.

The new clone uses the same immutable Tahoe image, macOS 26.6.2/25G83 arm64,
4 CPUs, 8 GiB RAM and a 50 GB disk. Its recorded pristine baseline lacks CLT,
Homebrew, Nix and installer state; official bootstrap provisions CLT 27.0 and
Homebrew 7.0.10. After native regressions and verified package cleanup, it is
also the fresh corrected release-fixture guest. The owner completed the normal
App Store & Known Developers setting; Screen Sharing visibly confirms it, with
Gatekeeper and SIP enabled. The previous permission blocker is cleared for this
new run. No security database, quarantine, historical source or installer state
was changed to force progress. The clean release fixture and final-head hosted
checks are still in progress.

The corrected clean v0.4.4 fixture now completes its unchanged public setup in
one invocation (the fixture's documented skip-Neovim/skip-agent flags). Authentic
upstream Nix 2.34.0, nix-darwin, Nix-managed Homebrew 6.0.1, tmux, fontconfig,
yamllint, pylatexenc and the real Code theme extension pass health checks.
The tracked historical source is unchanged; the OpenSSL certificate exists at
its canonical target and no literal `{{pkgetc}}` tree was created. Public migration
then detaches all 18 recognized profiles/configuration links with originals and
readable copies preserved, and converts Homebrew to the reviewed 7.0.9 source.

The extra full-tree comparison initially flags four tap `.git/index` files.
This is not lost package or repository content: all other 55,406 entries match
exactly in bytes, links, modes and owners. Native Git inspection of preserved
before/after index copies proves all 16,843 staged paths, blob identities, file
modes and index flags unchanged. The only debug fields that differ are cached
ctimes. The raw failed comparison and both sets of indexes remain retained;
no index, installer state or journal was edited. This wider observation spans
migration discovery and native manager diagnostics as well as conversion; it
does not attribute the cache refresh to a particular command. Package/tap
preservation passes; a speculative product change to freeze Git stat caches
is rejected. Replacement setup and final retirement continue separately.

The fresh released fixture's replacement setup passes for all ten selected
capabilities with four explicit configuration adoptions. Its public Check leaves
644 state entries unchanged. The subsequent exact preview for all 31 runtime
choices completes in one apply: all 76 Nix steps, the build accounts/group,
volume, services, boot references and approved legacy tools are retired. The
unrelated native package/tap and tracked historical source remain unchanged.

The additional pre-release witness comparison intentionally remains recorded as
failed where the historical installation itself changed the baseline. The user's
local zsh line and mode 0600 survive; its only extra bytes exactly match the
Homebrew shellenv block generated by unchanged v0.4.4 `persist_homebrew_shellenv`.
That is retained personal configuration, not a new installer write. The two
personal data files and original `/etc/zshrc` and `/etc/bashrc` match their complete
original bytes, permissions and owners. Previously absent fstab/synthetic files
retain only macOS's vifs comment header and the shared `run\tprivate/var/run`
alias; no Nix mount or boot entry remains. Whole-file absence is not the removal
contract for these shared files. Original failed comparisons remain preserved;
no witness baseline, installer state or journal was rewritten to force a pass.
Reboot and read-only repeated migration are the remaining native checks.

Dedicated Mac acceptance is complete. The fresh corrected released fixture
survives reboot with a new boot-session UUID; the Nix mount point, volume,
services, build accounts/group and active boot references remain absent.
All seven post-retirement witnesses retain their exact contents, modes and
owners across reboot. Fresh interactive zsh loads the personal override and
Starship; the configured Git include resolves `pull.rebase=true`. Real ripgrep
and fzf filtering and a tmux pane execute successfully. The transcript retains
the harness's initial TERM=dumb fallback and the subsequent normal xterm session.

Public Check and repeated profile/runtime migration are Ready. Each preserves
all 766 captured state/profile entries; repeated migration observes existing
receipts without replaying old detachment. The stopped earlier failure guest and
its originals remain preserved. The final guest's 77 diagnostic files are retained
in an independently checksum-verified archive, SHA-256
`77ecb5c1d6232bf3b0b02e898b25363e588e3ca8b73dae7292c1d895cdc626e3`.
Full state/profile snapshots, personal-data witnesses, native failed-before logs,
review receipts and source exports remain available for inspection. Authentication,
user onboarding, microphone/paste and physical shortcut routing are outside the
owner-clarified installer acceptance scope; none is reported as authenticated
or hardware-tested. The final committed-candidate review and exact-head check
results are recorded on PR #87, which remains unmerged.


Actual Opus 5.5 with CLI `--effort xhigh`, final closure session
`cdba0566-b78e-4923-b191-998177813bdf`, found no high/medium issue and no remaining
dedicated-Mac acceptance blocker. One low recovery-advice defect is confirmed:
plain `brew uninstall --cask zen` can invoke Homebrew's default autoremove. The
public pending preview and migration guide now prefix that manual command with
`HOMEBREW_NO_AUTOREMOVE=1`. A controller regression requires the protected
command and fails before (0.450 seconds), then passes after (0.350 seconds).
The guest's actual Homebrew 7.0.9 uninstall source confirms the environment guard;
its read-only autoremove preview lists 13 unrelated unused formulae. No unsafe
uninstall was executed against those resources. Ownership, automatic mutation,
application signatures and approval checks are unchanged. The guide additionally
discloses the retained released Homebrew shell block's possible PATH precedence.

The review accepted both investigated preservation differences. The original
seven-witness failure was on the tool's stderr stream, leaving the separate stdout
log empty. Its exact original timestamped tool-output record is now also exported
as `native-final-original-witness-failure-transcript.json`; it names precisely the
three already-investigated differences. No comparison was recreated or original
baseline rewritten. The full-tree claim covers bytes, links, modes and owners,
not unmeasured extended attributes or ACLs. The retained `run` synthetic alias was
introduced by the released nix-darwin setup; retirement preserves it because it
cannot prove exclusive ownership, not because it is a vanilla macOS default.
The exact final committed-tree make gate passed using valid cached v10 Go results;
the next production-string change requires its own gate and review.

The recovery-advice regression also fails before and passes after in the actual
macOS guest. The real native cask contract passes again in 6.26 seconds, including
scoped removal and complete outside-package/tap preservation; all 13 unrelated
autoremove candidates remain installed. An initial non-login SSH harness run
missed Homebrew in PATH; its failure is retained and the rerun uses the observed
Homebrew directory. A disposable Git 2.55.0 probe with changed cached ctimes and
`GIT_OPTIONAL_LOCKS=0 git diff --no-ext-diff --no-textconv HEAD` leaves raw index
bytes unchanged. The review's speculative index-writer explanation is therefore
not promoted to a confirmed defect or an attribution claim.

Actual Opus 5.5/xhigh delta review `52a2af76-4e4b-4aad-809f-e652e748b7ac`
confirmed the uninstall correction but identified the parallel reinstall path.
The exact converted Homebrew 7.0.9 source at
`cdc1ca503231b9a96da64526774915d913c3ec9f` confirms that `cmd/reinstall.rb` calls
`Install.finish_installation`, which invokes both install and periodic cleanup.
Periodic `Cleanup.clean!` can autoremove unrelated dependencies. Reinstall advice
now sets both `HOMEBREW_NO_AUTOREMOVE=1` and `HOMEBREW_NO_INSTALL_CLEANUP=1`;
uninstall retains its explicit no-autoremove setting. Documentation and the
controller contract cover both alternatives. This second regression fails before
in 0.351 seconds and passes after in 0.340 seconds. It also fails before and passes
after in the guest; the full real cask install/reinstall/remove contract passes
in 7.71 seconds. All 13 outside dependency candidates remain installed. The
manual command was not used to trigger unsafe cleanup as a demonstration.
The final candidate still needs its fresh full gate, actual Opus delta approval
and all 27 exact-head hosted checks; no earlier result is substituted for them.

The first hosted gate on `aa145847bcca5f839ed5a2b2123649cae30454f3` exposed a
test-fixture integration omission. Mac desktop job `118223160848` failed at the
Zen launcher assertion: it still expected `~/Applications/Zen.app`, although the
current selection uses the official cask at `/Applications/Zen.app`. Code and
Ghostty native consumers and the separate strict disk-image signature cases
passed. The failed job log is retained as `zen-final-desktop-macos-failure.log`.

The native desktop recipe now identifies `app.zen` and its canonical application
path for both the original/removal snapshot and LaunchServices launch. Its process
root is that exact bundle, so window detection and cleanup observe the launched
app rather than the retired private archive. No selection, native launch, Apple
Silicon architecture, window, post-use Check, Update or preservation assertion
was removed or relaxed. The production installer is unchanged. The focused
provider regression fails before in 0.307 seconds with the old private recipe
and passes after with the adjacent desktop graph checks in 0.297 seconds.
An initial unresolved temporary-parent failure is retained separately and is not
the regression proof. Portable path comparison normalizes separators only; it
still requires the exact application declared by the native provider. Actual
Mac desktop re-execution, renewed Opus review, full local gate and all 27 checks
are required on the next consolidated head and recorded on PR #87.

Actual Opus 5.5/xhigh fixture review `8fa7ef60-9c64-4322-a403-dea015600ece`
found no material defect and confirmed process cleanup remains bounded to the
current user's exact bundle executable paths, with pre-existing processes refused
before cleanup registration. Its optional preservation note is addressed too:
`~/Applications/Zen.app` retains its own before/after snapshot alongside the new
`/Applications/Zen.app` baseline. The fresh-runner fixture therefore checks both
locations after removal. A large pre-existing app still fails the bounded snapshot
before any installer mutation; separate cask/VM cases prove reuse preservation.
The focused final log is verbose and names all three passing desktop-provider
checks; the real GUI rerun and final exact-head gates remain required.

The same `aa145847` gate exposed three additional fixture failures. Ubuntu core
job `118223160698` observed a real descriptor link after the "closed" marker: the
C fixture's fopen created that marker and reused the just-closed descriptor,
while the reader advanced before fclose. Notifications now write an adjacent
pending file, close it, then rename it into view. No retry, delay, synthetic
readlink result or process-inspection policy change hides the race. The existing
regression still requires a real ENOENT gap and exactly two native reads.

Windows quality job `118223160108` exposed the cask test comparing actual Windows
modes against requested Unix 0600 and expecting a regular-file parent to produce
ENOTDIR. Permissions now compare the original native mode with the final mode,
without skipping that assertion. The refusal test uses a 256-character component
and first requires a real non-missing Lstat error. It still requires resource-local
pending/unknown status with no package command. Failed native job logs remain
`aa145-core-ubuntu-failure.log` and `aa145-windows-quality-failure.log`. Native
passed-after evidence, review and final gates remain required.

The unchanged real descriptor-reuse regression passes 50 consecutive Linux arm64
runs with the race detector (2.716 seconds) in an isolated container with copied
source and no host shares. The two corrected cask cases pass locally on macOS
in 0.530 seconds. These focused results do not substitute for the pending
native Windows rerun or the final complete hosted matrix. Actual Opus 5.5/xhigh
session `a4aa5f64-8e7a-41fb-a048-12557cfd77bb` also confirmed the additional
personal Zen path baseline, with no remaining findings.

The next exact head `523e7ee72ec289a3355e876208675d3d580994d5` passes full
local make ci (installer race 613.768 seconds), all four CodeQL checks and
Windows quality including both corrected cask cases. Its Ubuntu quality run
failed on an npm version published two minutes after the request; the failed
job and registry timestamp are retained, with a same-head rerun requested.

Hosted Mac desktop job `118243050768` now reaches the official Zen app, but
LaunchServices waits 30 seconds without completion or a window. The held ARM64
process remains at `_dyld_start` with a 96 KiB footprint. Direct/headless probes
work only as diagnostics and do not satisfy the failed desktop assertion.
The broad all-strict FinderInfo diagnostic is not the product's signature policy
and does not establish that Gatekeeper rejected the app. The original full log
is retained as `523e7ee-desktop-macos-failure.log`.

A new public Zen installation in the retained native-final guest is Ready and
reproduces the exact launch timeout. Its quarantine mark remains present;
spctl accepts the 1.23.3b bundle as Notarized Developer ID with the expected
9V5K9TP787 signer. At 19:12:23 UTC the system explicitly records the prompt for
app.zen-browser.zen and waits for a response. No consent, quarantine, policy or
permission database was rewritten. Existing migration evidence is preserved.

Actual Opus 5.5/xhigh consultations `eeb67a49-efb3-4ff4-bc6b-63449c274fe8`
and `319aacb5-263d-4f67-92cf-3d6b34696e11` distinguish the normal consent
boundary from installation failure. The second corrects the first's overly broad
assumption about pre-existing hosted Accessibility grants: using an existing
runner permission does not itself edit or bypass TCC. That permission is not yet
proved effective for this test process. First collect its trust status with prompt
requests disabled and the exact consent-agent structure on the real CI target.
New bounded read-only failure diagnostics do this before process cleanup; no UI
action, permission request or changed success assertion is introduced. Swift
compilation passes natively in the guest. Ordinary Open automation, if feasible,
still needs exact dialog evidence and genuine launch/window proof.

Actual Opus 5.5/xhigh diagnostic review
`37a458b3-bc21-40a3-9355-c022ffd4dd93` confirms read-only behavior, native
compilation, lifecycle placement and unchanged success assertions. Its requested
log filter/order correction now captures only relevant policy/consent categories
before potentially slow assessment/compilation. Replaying the retained guest log
reduces 362,270 bytes to 11,096. Both original and filtered 8 KiB tails contain the
prompt, so that sample does not reproduce the reviewer's predicted truncation;
no failed-before claim is made for this preventive diagnostic refinement.
The real failed desktop launch remains the regression to resolve. The retained
guest is cleanly stopped, with all seven personal witnesses and 32 outside package
lines unchanged. The read-only CI probe is an intermediate investigation, not
a completed delivery gate.

Actual Opus 5.5/xhigh follow-up `54b3b578-a9db-4a08-9abd-a8490314a552`
confirms the corrected diagnostic has no material issue for an unmerged native
probe. Focused provider/lifecycle/refusal checks pass in 0.571 seconds. This
intermediate push retains the final full-gate and 27-check requirements.

Native diagnostic head `6c9ba78d3119ff2a0d0b75c3ad4fd0cfc504f6c0` passes
full local make ci (installer race 617.477 seconds). Mac desktop job
`118261425211` preserves the genuine failed launch and proves existing
Accessibility trust on macOS 26.6.2/25G83. Gatekeeper is enabled; Zen 1.23.2b
is accepted as Notarized Developer ID with team 9V5K9TP787. The held PID has
no app window. The exact Apple consent-agent tree contains the Zen Internet
download question, Homebrew Cask origin, Apple's clean-malware verdict, Help,
Cancel and enabled Open (`action-button-1`). Quarantine remains 0181. This
settles the permission/shape uncertainty without changing any security state.

The fixture correction supplies ordinary consent only in the already opted-in
hosted lifecycle. Preflight verifies Gatekeeper, notarization and both signatures.
The helper refuses existing Zen processes/consent windows, requires the exact
observed prompt and revalidates the held process's executable/PID/start time and
the same window/button immediately before a single AXPress. It never requests
permissions or changes quarantine. LaunchServices must complete for that same
ARM64 process; the prompt must disappear and macOS must record user approval
while retaining quarantine provenance. The existing real-window, Check, Update,
Remove and personal-baseline assertions remain intact. Other dialog shapes wait
without action inside the original launch deadline; there is no input fallback.
The system's user-approved bit is defined by Apple's
[QuarantineSPI.h](https://github.com/apple-oss-distributions/WebKit/blob/WebKit-7618.2.12.11.6/Source/WebCore/PAL/pal/spi/mac/QuarantineSPI.h#L38-L43).

The captured normal prompt is rejected by the pre-correction helper, which has
no consent mode. The new native Swift boundary checks accept it and reject unrelated/security prompts.
That focused check does not prove a UI press works. The original actual native
failure remains the end-to-end regression; its passed-after run, actual Opus
review and final exact-head gates remain required. No product installer behavior
or user onboarding requirement changes.

Actual Opus 5.5/xhigh review `e2da99ca-0fc2-4feb-b455-6dc377b48606` confirms
the safety boundary and the absence of a false-pass path. Its transition finding
is addressed: the first observation may encounter a closing XProtect progress
window, multiple transient windows or a stale AX element. It now waits without
action, preserving the first inspection error for a timed-out launch. A press
still requires two successful matching reads and exactly one window. The final
revalidation and all post-press checks remain hard failures. Completion updates
are serialized onto the same main queue as the polling timer. This correction
does not retry an Open action or extend the launch deadline.

Actual Opus 5.5/xhigh closure review `5bea235f-7009-4e9f-91a7-abf03be38d25`
confirms that transition finding is closed, with no material issue blocking
the native run. The requested success diagnostic now separately records AXPress
returning success and the later verified native completion. Focused race tests
pass in 3.371 seconds, including all 16 classifier/host-boundary cases. The actual
GUI press and final committed-head gate results are recorded on PR #87; these
focused checks are not substituted for native window proof.

The same `6c9ba78` matrix also hit the Windows quality suite's global 20-minute
Go timeout, while a late eight-scenario Terminal test had run for 67 seconds.
No assertion failure was reported. The unchanged Windows code passed the previous
`523e7ee` suite in 906.067 seconds. The timeout log and stack are preserved;
its cause remains unconfirmed. The next head reruns the same native Windows gate
without changing its timeout or removing assertions. Opus found no concrete
connection to these Mac-only runtime test changes.

### 2026-10-19 — released fixture must preserve existing prerequisite ownership

Head `35c2e4d` passes 26 of 27 final checks, including Windows quality/core and
the real Mac desktop lifecycle. Mac core `118277785911` fails after genuine
released setup and Homebrew conversion: the fixture repeatedly selects
`retire.delta.fontconfig`, but eight outside runner packages still require it.
The installer correctly preserves the shared formula. Its unfinished operation
then blocks the following Determinate test; that is not independent removal
proof. The complete original failure log is retained.

Investigation found that prerequisite preparation unconditionally requests
already-installed formulae. Homebrew's install implementation promotes an
existing automatic dependency to `installed_on_request` even when no version is
installed. A new real native regression exercises an automatic dependency, its
outside consumer and a genuinely absent prerequisite, requiring exact existing
package metadata and payload preservation. Native failed-before/passed-after
proof and actual Opus consultation are in progress. No journal, package receipt,
retirement completion rule or dependency protection is edited to force progress.

The real Homebrew 7.0.9 regression reproduces the promotion in 9.97s: the same
shared formula changes from Automatic=true to false, and its native install
receipt changes. This is the existing unconditional command extracted into a
helper, not a fabricated package receipt. Missing-only preparation passes the
same native test; the final inventory/source/Cellar guards pass in 8.58s. The
existing dependency and requested consumer retain exact native metadata and
payload snapshots, the absent formula is genuinely installed, and the unique
fixture packages are cleaned up with all outside inventory unchanged. All seven
post-retirement personal/system witnesses remain exact; SIP and Gatekeeper stay
enabled. The native source of the promotion is
[Homebrew install/check.rb](https://github.com/Homebrew/brew/blob/cdc1ca503231b9a96da64526774915d913c3ec9f/Library/Homebrew/install/check.rb#L188-L198).
The earlier wording "no version is installed" means nothing new is installed
or upgraded; an existing version and its receipt are present.

Actual Opus 5.5/xhigh consultation `76b63d01-133e-4cb4-b5c3-45dfd5870889`
independently traces the fixture's promotion and rejects the initial proposal
to thread Keep/deselection through retirement. That alternative cannot satisfy
the documented Ready contract and would preserve manufactured ownership.
`finishRuntime`, `runtimeSelection`, production code and every Ready assertion
remain unchanged. The full Darwin fixture now independently records fontconfig
after released setup and proves either unchanged automatic shared ownership,
payload/consumers and the Kept disclosure, or absence of a requested unshared
copy. Unknown, pinned or requested/shared baselines fail explicitly. The prior
Determinate failure remains a documented cascade; no automatic abandonment is
added. Final review and hosted full migration/Determinate proof are still required.

Actual Opus 5.5/xhigh review `982a06b3-e52a-4091-90b6-fa4827406a0b`
finds one ordering issue in the new, unpublished test assertion. Reading semantic
inventory before conversion would run Brew 6.0.1 against current runner casks;
the earlier observed Firefox `command_wrapper` failure (run `39025865684`)
already proves that boundary unsafe, and old Brew can silently omit definitions.
This candidate was not pushed or claimed to pass hosted migration. The review
confirms the preparation helper and native regression have no other material
issue. Full local make ci on the intermediate `cf16343` passes (installer race
617.356s), but does not exercise this opted-in native ordering.

The correction keeps the fontconfig payload snapshot before conversion and
moves its semantic classification until after the existing exact file-record
conversion comparison. Both semantic reads now use converted Homebrew. The
earlier phrase "records fontconfig after released setup" is narrowed to this
verified post-conversion point; no pre-conversion JSON query is added. All
original and strengthened preservation, removal, Ready and recovery assertions
remain. Final review and exact-head gates are required again for this correction.


### 2026-10-20 — automate the remaining hosted acceptance journeys

The owner requested canonical CI automation of repeatable VM acceptance, then
explicitly declined dedicated Mac/VM CI infrastructure. The previous final head
`94275a826829aae37394c3e2a18a7006ce7d4998` remains the passed baseline: all 23
native/quality plus four CodeQL checks, full local make ci and actual Opus
implementation closure. Its Windows timing reliability caveat remains open.

Read-only inspection found existing real PTY/ConPTY category/mixed/child tests,
divergent Git pulls, native migrations, personal preservation and an APFS remount
regression. These are not new gaps or VM-only coverage. The additions connect the
public keyboard UI to real providers and a continuous preservation journey,
exercise actual installed skill discovery/helpers, and require Code document
edit/save/close/reopen through its extension host. Machine requests in the public
UI journey only inspect; every mutation receives keyboard approval.

Actual Claude CLI 2.1.301 was invoked with `--model claude-opus-5-5 --effort xhigh`
for design consultation `ab1085bb-af27-46fc-b56d-3b44b06a3585`. Its independent
fresh-Windows-state concern is addressed by putting the new journey before all
other installer fixtures in existing core jobs, with an absent-state assertion.
The 150-minute core budget is distinct from its unchanged 20-minute Go race
budget; previous actual core durations leave room for a separately bounded
journey. No check identities or live protection settings change. Its claim that
the UI omits the plan ID is rejected: terminal/workflow.go explicitly renders
Plan and Source, which the native journey verifies. Dedicated-runner proposals
are superseded by the owner's later scope clarification. No reboot automation or
authenticated provider proof is claimed. Final patch review and all affected
native/final-head gates remain pending.

The first local baseline attempt encountered the pre-existing unrelated Python
bytecode cache in EditorConfig. The original cache was preserved and temporarily
moved outside the checkout for the complete baseline, then restored with exact
hashes/modes. Baseline make ci passes in 117.89 seconds. This is local fixture
isolation, not a source exemption or deletion of unrelated work.

Local integration exposed two unpublished harness issues: EditorConfig correctly
rejected space indentation inside the Go-embedded Code extension (corrected to
tabs), and source inspection found the existing Windows archive/application
fixture also requires a fresh known-folder state directory. Both fresh-state
assertions remain. The new UI journey now proves its entire state directory was
absent, completes public removal and Check, refuses active/deferred ownership or
transactions, then relocates that exclusively created directory intact to a
same-volume evidence sibling. Original contents/modes are compared and journals
archived. Failed or unfinished journeys are never relocated. This is disposable
fixture isolation after completion, not journal editing or recovery bypass.

Actual Opus 5.5/xhigh review `16de2444-52a0-4e16-84e6-c6bc4eb2cde7` required
corrections to the unpublished harness. The first attempt's restricted working
directory blocked the frozen files; it was stopped and is not counted as final
approval. The resumed review read the immutable candidate from its own directory,
with no denied reads. Full local make ci on that candidate passed in 761.84s
(installer race 623.47s); opted-in hosted journeys were still unrun.

Accepted F1: public Check emits `needs-action` JSON and exits 1 after deliberate
entrypoint damage. The test discarded that result before decoding it. A real
child-process regression fails before correction. The helper now decodes stdout
and requires the exact exit/status pairing, while the journey also requires the
`tool.rg` diagnostic. Invalid JSON, transport errors, an unexpected failure exit
and false-success exits remain failures. Subtest failures now use the subtest's
own testing handle (L1).

Accepted F2: Windows archive pointers are junctions (`ModeIrregular`), which the
new snapshot/tar code rejected. A portable regression reproduces the native
filesystem metadata contract and fails before correction. Snapshots record the
native target without following it; tar represents the link with an explicit
junction-kind PAX record. A real native directory-link regression also requires
that unrelated target contents are never traversed and a subsequent journal is
present unchanged. The Windows variant creates an authentic junction; its hosted
execution remains required. Both focused failed-before cases now pass locally.

F3 was presented as a likely Code-harness failure, not an observed app defect.
The [VS Code API](https://code.visualstudio.com/api/references/vscode-api#workspace.onDidCloseTextDocument)
does not guarantee disposal when a tab closes, and its
[document reference collection](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/api/browser/mainThreadDocuments.ts)
retains `openTextDocument` references for minutes. The fixture now opens through
`vscode.open`, takes the active editor's document, then keeps the same save,
disposal, new-document identity and native-mode checks. Actual desktop execution
must prove the revised sequence; no timing waiver or cached-buffer fallback is
introduced.

L3 is addressed by parsing installed helper stdout separately from diagnostics
and excluding server requests from Codex response matching. L2's optional
workflow-condition expansion is declined: a failed first journey remains red,
its evidence is uploaded, and later fresh-state mutation fixtures stay skipped;
independent quality jobs still run. Claude control-only EOF remains an explicitly
unproved hosted behavior, not a claimed bug or silently accepted abnormal exit.
Review closure and exact-head native gates are still required.

Opus's same-session v3 correction review closes F1/F2/F3/L1/L3 and accepts the
L2 decline, while raising N1 in the newly added junction regression: the hosted
Windows temporary path can use an 8.3 alias while PowerShell stores its expanded
path. The fixture now canonicalizes both temporary directories before creating
the native link, preserving exact target comparisons. It also binds Windows
PowerShell's own Management module before New-Item, following the existing native
module-origin convention. This is a test fixture correction; no product path
contract is relaxed and no Windows runtime pass is claimed before CI.

Opus's v4 closure (same actual Claude 5.5 session, explicit CLI `--effort xhigh`)
accepts N1 with all comparisons intact and reports no open implementation
findings. It approves pushing for hosted verification, explicitly separating
that approval from unrun Windows junctions, Claude EOF, Code disposal and the
four real public keyboard journeys. The local v2 complete gate and v3 focused
corrections are passing; final full local and exact-head hosted receipts are
required and recorded on PR #87. Sentinel upstream was rechecked on 2026-10-20:
its current main remains the tested `6645688d08f1505595cae276832a5a5070b82160`.
No production installer logic changed in this CI expansion.


### 2026-10-20 — Windows settings inspection timing correction

Candidate `cfb54576c6270c19f909e561368fd147559e7a7a` passes all four new public
keyboard journeys and installed Sentinel/CLI/helper lifecycles. Code's new
edit/save/disposal/reopen assertions pass on every supported target. Linux amd64
desktop initially stopped at a pinned Code download with unexpected EOF; one
unchanged fresh-runner retry passes, with the original full log retained. Linux
ARM's separate Rust HTTP/2 peer failure remains recorded; it is not waived.

Windows core job `118447773887` instead exhausted the ordinary root package's
unchanged 1200-second race limit at 1200.203s. No assertion failed: the active
Terminal removal case had run seven seconds and was computing JSON. Its later
native steps, including Sentinel, bootstrap/migration and legacy retirement,
passed because the workflow intentionally continues independent steps. Progress
into those steps is not success evidence for the earlier suite. Same-head Windows
quality passes the root suite in 1084.544s; the recurring cumulative timing risk
cannot be called fixed by a favorable retry. The original timeout/stack is retained.

Actual Opus 5.5/xhigh consultation in session
`16de2444-52a0-4e16-84e6-c6bc4eb2cde7` identifies repeated parsing of the same
Terminal object, nested parents and collection entries during one extraction.
An isolated Mac race profile confirms that work and allocation overhead. The
139-second legacy Pi test is in a separate opted-in step, not this package budget.
Opus recommends correcting the read path rather than another Windows retry.

The correction uses one inspection-local object/array parse cache, keyed by the
exact input slice start and length without unsafe arithmetic. Shared lookup and
identity-selection logic keeps uncached writers identical; no JSONC mask, saved
projection, approval, ownership or restoration rule changes. A frozen uncached
reference requires exact output bytes and error text across the real recipe,
schema-1 projections, malformed/duplicate containers, grouped/legacy bindings,
case/GUID identity cases, comments, trailing commas and input bounds. Separate
checks cover edited input in a later inspection and different slice windows.

The allocation regression fails on the original committed source: 18,360
allocations exceed its 10,000 limit. The revised extraction uses 4,512 allocations
for the same pinned recipe, with all equivalence checks passing. This deterministic
regression avoids a machine-dependent elapsed-time assertion. Full Terminal race
coverage, full make ci, independent patch review and all 27 new-head hosted checks
remain required; native timing improvement is not yet claimed.

A private overlay experiment also passed all 18 recovery cases in 114.554s
serially versus 41.236s with four workers. Test parallelism was not retained: it
would leave the repeated production parsing in place, and the repository's
established tests run serially. Timeouts, check identities and every recovery
case/assertion remain unchanged.


Opus's correction review (same actual 5.5 session and explicit xhigh invocation)
reports no material findings. It confirms cache lifetime, exact slice-window
keys, shared unchanged validation and a faithful frozen reference. The ordinary
Terminal race group passes locally in 141.157s; its 18 recovery cases take 60.90s
without parallelism, versus the serial pre-fix experiment's 114.554s. These are
local measurements, not a final Windows timing result.

Allocation measurements vary slightly under the race detector: current runs report
4,512 and 4,460 (about 4.5k), both below the unchanged 10,000 budget; pre-fix runs
report 18,360 and 18,316 (about 18.3k). The red proof
uses the actual archived `cfb5457` source with only a standalone regression file
added; its fixture helper, test body and threshold are the same as the current
`TestWindowsTerminalExtractionAllocationBudget`. The different regression
filename in that log is intentional. All new-head native/quality/CodeQL checks and
full make ci remain required before delivery.


Full local make ci passes on the reviewed correction candidate in 637.48s
(root race package 495.535s; terminal package 24.763s). The unrelated pre-existing
Python cache is restored with its original bytes/modes. No source changed after
Opus's correction review; documentation records completion and the remaining
exact-commit native gates. The amended commit must pass those gates, including
both Windows race suites, before delivery is claimed.

### 2026-10-20 — Owner feedback: apply progress, navigation and confirmation

The owner ran setup on Linux and reported: no output during the entire Neovim
installation after the bootstrap's `all modules verified` line; a final report whose
last line, `External application not exercised: X11 clipboard round-trip …`, read
as the failure; Esc on the tool list quitting setup; the terminal flashing back to
the shell between every screen; and a review document too long to read before
every confirmation. The owner asked for live package-manager output plus numbered
installer progress, Esc for back and Ctrl+C for quit, a confirmation whose default
is Apply with the full review behind a hotkey, and Back returning to the main menu.
The owner also asked whether Linux setup runs under sudo.

Root causes, confirmed in source:

- Severity Medium (usability) — `installer/terminal/workflow.go`: every Choose,
  Review and Report opened and closed its own alternate screen
  (`\x1b[?1049h`/`\x1b[?1049l`), so the shell, including the bootstrap's
  `go mod verify` line from `scripts/installer-bootstrap.sh`, reappeared between
  screens and during all apply work.
- Severity Medium (usability) — providers capture native output into bounded
  buffers; the native worker returned it only in its final reply, and Execute
  emitted nothing before its result. A long Neovim synchronization therefore
  showed no activity at all.
- Severity Low — `characterKey` mapped Ctrl-C, Ctrl-D, Esc and q to one back key,
  which the controller treated as cancellation of the whole run.
- Severity Low — Execute joined needs-action reasons with `; ` on one line and
  appended disclosures after them; interactive main then re-printed the entire
  message as the error, so the trailing disclosure looked like the cause.

Changes:

- One alternate-screen session spans the menus; raw input exists only while a
  screen reads keys, so Ctrl-C between screens still interrupts a preview. A busy
  screen replaces a finished menu while the system is inspected.
  `Interaction.BeginApply` releases the screen before approved work and returns the
  context that carries progress; the controller, profile migration and runtime
  retirement call it before execution.
- `engine.ErrBack` wraps `ErrCancelled`. The controller loops to its main menu on
  Back, keeps the draft tool selection, and returns to the menu after Check. A
  first run opens tool selection directly and Back reaches the menu. Flows without
  a menu still treat Back as cancellation. Ctrl-C/Ctrl-D quit.
- By owner decision the confirmation is compact and defaults to Apply. It shows
  the counts, then the removals, replacements and administrator changes that fit,
  then `d` for the complete unclipped plan (including retirement exact scope).
  Pending input is discarded when the confirmation opens, so type-ahead cannot
  approve; EOF still never consents.
- Execute reports `[n/N]` begin/done/failed/needs-action steps for every counted
  mutation, plus check, verification, resume and restoration phases. Downloads
  show a rewritable percentage line and report completion only after the SHA-256
  matches. Neovim reports its three synchronization phases and verification. The
  native session shows each command line, shortened and without environment.
- The worker forwards child output as `stream` messages before its single final
  reply; the client rejects mixed messages, and saved replies containing a stream
  are invalid. Captured evidence keeps the 1 MiB bound. The worker handles SIGPIPE
  so a vanished controller cannot kill it mid-command (the existing
  controller-death regression exercises output written after that death). A
  replayed saved result says so rather than implying a new execution.
- The terminal strips whole CSI/OSC/DCS/charset sequences and controls from native
  output, honors carriage-return rewrites and CRLF, wraps endless lines at
  character boundaries, and returns its first display failure with the result
  report rather than to a provider pipe.
- Needs-action reasons are joined with newlines. The report heads with the status,
  lists actions, then puts preserved-file and unexercised-application lines under
  "Notes (informational, not failures)". Interactive setup no longer re-prints a
  report it has already shown; the exit status still fails.
- Bootstraps no longer print successful `go mod verify` output; failures still
  reach stderr and stop the bootstrap. Foreground `sudo -v` names its requester
  through `-p`.

Sudo answer: run setup as the normal user. APT, Homebrew and Apple CLT
authenticate with a foreground `sudo -v` after approval, and the durable worker
uses `sudo -n`. README now says so explicitly.

Regression proof: real PTY tests cover apply with details and live
`==> [1/1] Installing …` output, Back from the confirmation, declining with the
Back choice, Back from tools to the menu and into tools again, and quit. The type-
ahead case (`" \r\r"`) fails with discarding disabled, because the leftover Enter
applies the plan, and passes with it. Engine tests cover numbered steps and the
failure line, menu/draft navigation, Check returning to the menu, live worker
streaming before the child exits, replay disclosure, evidence without stream data,
and verified-only download completion. Renderer tests cover escape removal,
carriage returns, character-boundary wrapping, display-failure retention and
report sections.

Recorded choices for owner review: the retirement exact scope is now reached via
`d` like other plans rather than shown before the confirmation; and Ctrl-C on a
needs-action check report during migrate now ends with the cancellation error
instead of the "Migration stopped after …" message (Enter/Esc are unchanged).

### 2026-10-20 — Streamlined interactive flow (owner: "ship all canonically")

After the progress/navigation change, an assessment from source found the
remaining friction structural: up to four whole-machine inspections before a
confirmation (initial preview, adoption, shared-removal trial and final preview),
three separate decision screens, `sudo -v` before every APT dispatch (a long
Neovim sync could outlive the timestamp), no persistent log, failures split
between the report and a second `Dotfiles:` line, a 1.3-1.7 s toolchain unpack
and source listing before every menu, and five overlapping menu entries. The
owner approved all eight proposed changes, including the two needing a decision:
sudo credential refresh and a toolchain-free bootstrap path.

Findings that shaped the design, each confirmed in source:

- Previews take exclusive non-blocking `flock` locks. A background inspection
  must be owned by the controller and joined or cancelled before any other
  preview or Execute, or the second lock attempt fails immediately.
- `NativeDriver.ForSelection` binds shell assembly and the APT/Homebrew keep sets
  to the resolved selection. Observations are therefore reusable only for an
  identical closure of Selected and Keep; the cache key is that closure.
- `operationPrivileged` is true for every machine-scope resource, including
  Homebrew formulae that never use sudo. Up-front elevation instead follows the
  providers that already authenticate in the foreground.
- Every mode already repairs unhealthy owned resources, and an unchanged apply
  selection is a repair; removing the Remove/Repair menu entries loses no ability.
- The Pester harness initially defined extracted functions through
  `[scriptblock]::Create`, which the supply-chain gate correctly rejects, and its
  comment tripped the PS 5.1 apostrophe gate. `Get-SHA256` now declares its
  parameter inside its body, so tests use the existing AST-body extraction.

Changes: `session.go` (inspection cache, background inspection, tool status,
dependent review pipeline, `checkResult`), merged review with `Proposal`/`Revise`
and `ErrPlanChanged` re-review, `elevation.go` (`ElevationDriver`,
`KeepSudoAlive`), `applylog.go` (`Result.Log`), `ReportedError`, the four-entry
menu with a likely default, the terminal tool screen and scrolling confirmation,
and the POSIX/PowerShell bootstrap fast path. Docs: README, UPGRADING, CLAUDE.md,
status, roadmap and the test retirement map.

Evidence: engine tests cover single inspection per closure, background reuse and
joining before a preview, worst-first status labels, forged review choices, in-place
shared-removal revision, the stale-approval re-review (never applying the stale
plan), elevation only for foreground providers and exactly around approved work,
credential refresh until release and a reported stopped refresh, log contents,
privacy and retention, a log failure as a note, the likely menu default and a
failure reported once with its log. A real-PTY journey reviews an installed tool's
status, unticks it, toggles the shared runtime in the confirmation, watches the
counts change and applies with the log path shown. Bootstrap tests prove no
toolchain on an unchanged checkout, the full path after any file, nested file,
link or launcher change, refusal of a damaged binary, and no reuse record after a
checkout changed mid-build; two earlier tests now force the toolchain path so they
still prove archive and compiler-version checks. PowerShell 7.6.3 (official
Linux build, SHA-256 verified against the release list) ran the 13 bootstrap
Pester tests and the extraction test; PSScriptAnalyzer 1.25.0 (gallery SHA-512
verified) reports no findings in the changed scripts. Measured on the local
workstation: 8.5 s first build, 1.3-1.7 s per launch on the full path with a
cached binary, about 0.1 s on the fast path. A read-only real-binary PTY run
reviewed an actual inspection, opened details, returned to the menu and quit
without creating state. The confirmation now names needs-action items, after that
run showed only their count.

Not proven locally: native macOS/Windows terminals, Windows PowerShell 5.1 for
the fingerprint, a real sudo refresh across a long APT run, and type-ahead during
a slow revision (the revision window is too short with fixture drivers). The
local `ruby` and `clang++` prerequisites are missing, so the settings/required-
checks policy tests and one clangd spec fail locally before and after this change.

### 2026-10-20 — Integration onto the current PR head

Before delivery, a fetch showed PR #87 force-updated to `7ce3ded` while this work
was based on the earlier `d9a3993`. Nothing was pushed over it. The work was
reapplied with a three-way merge onto `7ce3ded`; the superseded `d9a3993`
base was not kept as a separate branch. Conflicts
were limited to the append-only status and ledger files, kept in full, and
`controller.go`, whose only upstream change (the legacy-restoration choice
wording) now lives in `session.go`. Upstream additions compile against the new
interaction API unchanged. `installer/README.md` and a stale test title still
described the removed prompts and were corrected.

`7ce3ded` adds the hosted, opt-in `TestNativePublicInteractiveLifecycle`, which
drove the old screens. It now navigates the four-entry menu from the actual
initial cursor, removes by unticking, ticks requested replacements on the
confirmation (waiting for each revision), verifies the exact plan identity and
operations through `d`, returns through the menu's Exit after Back and Check, and
uses Update as the repair journey. Its screen helpers are exercised locally by
the merged-review PTY test; the journey itself still requires the disposable
hosted lane, because it installs real tools.

### 2026-10-20 — Hosted public journey: two quick Escapes were both lost

The first hosted run of the ported public journey on `055819d` failed on
`core (ubuntu-26.04-arm)` in `01-back`. It sent Esc to close the plan details
and Esc to leave the confirmation in one burst; the screen stayed on "Review
every change". Cause, in `input_unix.go`: after an Escape the reader waits 75 ms
for a sequence, and a second Escape inside that window was consumed as an
unknown sequence, discarding both presses. A person tapping Esc twice would see
nothing happen. The reader now treats Escape-Escape as two Back presses, holding
the second for the next read (a discard of type-ahead clears it). The new
`double-escape` PTY case fails without the change and passes with it. The public
journey also waits for the confirmation before its second Escape, so it no
longer depends on key timing. Windows reads native key records and was unaffected.

### 2026-10-20 — ConPTY frames and Alt+Escape on Windows

On `a401bf1` the Windows quality job and the Windows public journey failed in
the new screen helpers, and the Windows `double-escape` case failed. Hosted
evidence: ConPTY never passes the console's `\x1b[H\x1b[2J` through; it repaints
every cleared frame as `\x1b[2J\x1b[H` plus the full screen (30 such frames in
the captured journey, including a complete confirmation with the Sentinel
replacement checkbox). The helpers now split frames on the clear common to
both. ConPTY also reads an Escape burst as one Alt+Escape key event, the VT
convention for Alt, which a person double-tapping Escape in Windows Terminal
produces as well; this corrects the previous entry's statement that Windows was
unaffected. Alt+Escape has no other meaning on these screens, so the Windows
reader now yields two Back presses for it, matching POSIX. The same run's
`Fresh Windows vendor (Build-Tools)` failure timed out at its fixed 30-second
read-only inspection of the runner's pre-registered Build Tools, before any
install step, in code this change does not touch and that passed on `055819d`.
The bound stays unchanged; the next run re-executes it.

### 2026-10-21 — Personal-Mac migrate stop: Sentinel targets, skills owners and legacy policy

The owner's `./migrate.sh` on the personal Mac stopped after the setup phase.
Evidence: both skill roots held upstream Sentinel's `.sentinel-skills.json`
from a direct `tools/skills.mjs` install on 2026-10-19 (seven unedited roots,
hashes matching that registry); policy had been written to `~/.kiro` although
Kiro was never installed; the released v0.4.4 setup had written the Sentinel
0.1.2 block as the whole of `~/.config/opencode/AGENTS.md` (byte-identical to
the ecafffa render); and the terminal printed the setup report twice. The owner
asked for canonical fixes ("only install Sentinel on Kiro if .kiro exists or
Kiro is installed", "fix canonically", remove OpenCode and Pi from this Mac).

Findings and corrections, each with a failed-before regression:

- Sentinel policy wrote all four consumers unconditionally. It now targets an
  agent only when its configuration root exists, its command (`codex`, `claude`,
  `copilot`, `kiro`, `kiro-cli`) or Kiro's macOS app is found at the process
  boundary, or the bound closure installs tool.codex/tool.claude. No present
  agent is a resource-local pending state. Saved ownership keeps its baseline's
  targets through `ForSelection` (the Git saved-path pattern), so update and
  removal still reach every written file. A newly present agent is a note
  (reselect to add it); a covered agent whose root vanished sets ApplyBlocked
  instead of recreating its folder. With the old behavior forced, both new
  tests fail (absent agents targeted; `~/.kiro` recreated by adoption repair).
- Upstream's complete `--remove` leaves `{"schema_version":1,"roots":[],"skills":{}}`.
  The skills guard refused any registry file, so the documented upstream removal
  still blocked setup. An empty schema-1 registry now owns nothing and stays
  untouched; any other or unreadable registry still refuses.
- The `npx skills` CLI (1.7.4 source read: `~/.agents/.skill-lock.json`, or
  `$XDG_STATE_HOME/skills/` when set) still claimed `impeccable` from an April
  standalone install. Its `remove` deletes every agent's folder of a name with
  no ownership check, so a later run could delete dotfiles-owned copies. A claim
  on a bundled name, or an unreadable lock, now blocks both roots.
- Setup disclosed the released OpenCode/Pi policy as "External application not
  exercised" and pointed to another tool. Migration retirement now owns it:
  `retire.sentinel-policy` discards a file only when its whole content hashes to
  the pinned 0.1.2 render (`testdata`; the opt-in source test reproduces it from
  the ecafffa archive, codeload SHA-256 6523513b…). Mixed or edited files are kept
  and reported; edits after preview are preserved by the existing discard check.
  Setup no longer reports these files.
- Migrate's stop message appended the full report the setup phase had already
  printed. It now names the phase and points to that report; a check report,
  shown on the alternate screen, is still repeated.

Rejected: making removal of an already-absent target skip parent creation.
The writer did recreate the parent, but the planner retains a changed resource
on deselection, so that path is unreachable; the change and its test were
reverted. Rejected: one resource per Sentinel consumer, which would need planner
presence semantics. Known limits: a covered agent removed later leaves Sentinel
blocked until the agent returns (no clean narrowing operation yet), and parent
folders created by a profile publication (for example `steering/`) stay after
removal.

Verification: failed-before/passing regressions for each correction above; full
local race suite and the remaining `make ci` targets pass on the final tree. The
opt-in source tests reproduce the 0.1.2 and 0.2.0 renders. Pre-existing and
unrelated: four `tests/nvim/spec/treesitter_spec.lua` cases fail on this Mac on
both `fee9f5d` and this tree, because the specs use the real `stdpath("data")`,
which now holds the published managed runtime
(`dotfiles-runtime/nvim.sync/current`). Hosted runners have no such runtime. The
spec runner should isolate XDG_DATA_HOME; not changed here.

### 2026-10-21 — Recommended follow-ups shipped (owner: "ship all your recommended fixes canonically")

The four known limits and advisories recorded above are now corrected, each with
a failed-before regression:

- Neovim specs read the real `stdpath("data")`. Both spec runners now use a
  private temporary XDG data/state/cache root; all 465 specs pass on the personal
  Mac (461 before, the four treesitter cases failing on `fee9f5d`), and the suite
  no longer writes shada, swap, undo or logs into personal state.
- Folders a publication created stayed after removal. Profile and configuration
  first baselines now record an absent target's missing ancestors; finalization
  of a removal that restores that absence removes them, nearest first, only while
  each is an empty real directory, after the adjacent workspace is gone. Older
  baselines keep their folders. Publication that keeps an absent profile absent
  creates nothing, which supersedes the earlier "unreachable" rejection: retired
  Sentinel targets now reach that path.
- A covered agent removed later left Sentinel blocked. Its target is now retired:
  healthy without a block, never written. The changed owned policy keeps the
  existing explicit replacement rule, after which removal restores the rest.
- Informational guidance was labelled "External application not exercised".
  Observation.Notes now carries Sentinel advisories and the Impeccable setup hint
  ("Note:"); genuine unexercised-application disclosures keep their label.
- Deleted preserved copies kept appearing in results. Displays omit only paths
  with a definite not-exist answer; receipts, journals and ownership decisions
  are unchanged.

Owner preference recorded the same day: shipped fixes are pushed to the PR as a
single squashed commit so other machines can test them.

Personal-Mac Nix retirement then refused two consumers that are the vendor's
own state. Root's default profile held `determinate-nix-3.21.1` and
`nss-cacert-3.121` while `/nix/receipt.json` says installer 3.21.2: the check
required equal versions, but the installer bundles the Nix of its `nix-src`
flake input, and v3.21.2 only bumped determinate-nixd (its flake.lock still
pins nix-src 3.21.1; all 73 other reviewed releases match). The release-pin
generator now also records each tag's bundled Nix version; the lookup keeps the
equality assumption only beyond the reviewed table. The second refusal was the
indirect root Nix registers for `~/.cache/nix/flake-registry.json`; that cache
is already moved aside as retired user state, so roots into it are no longer
outside consumers, while a project `result` root still is. Both regressions
reproduce the live refusal text before the change.

A third refusal followed: `/etc/bashrc is a link; the vendor uninstall would edit
its destination`. nix-darwin was detected only through `/run/current-system`,
which this Mac no longer had: `/run` is cleared at boot and the
`org.nixos.activate-system` daemon was not loaded, while `/etc/static`,
`/etc/bashrc`/`/etc/zshenv` links, the `*.before-nix-darwin` originals and
`/nix/var/nix/profiles/system` remained. Without detection, retirement skipped the
nix-darwin `/etc` restoration and the vendor check refused the remaining link.
`nixDarwinSystem` now falls back to the system profile only when its etc is
exactly `/etc/static`'s target; the fallback reproduces the activation-link scope
exactly, and a foreign `/etc/static` or missing profile infers nothing. The
personal-Mac preview then planned 67 verified steps (57 administrator, one
irreversible store deletion) with no blocker; only the receipt's nix-daemon runs
from /nix.

Independent review of these follow-ups (read-only subagent, no toolchain) raised
seven findings; all were confirmed against the code and corrected with
failed-before regressions where behavior changed:

1. Medium: with every saved Sentinel agent removed, the observation was absent
   but healthy, so the planner proposed an install that could not publish and
   always failed. Apply is now blocked with a deselect instruction; deselection
   forgets the absent policy without recreating folders.
2. Medium: a mixed, linked or unreadable OpenCode/Pi file was a Retained item,
   which kept runtime retirement needs-action forever. Such files are now kept
   details; a linked personal file is no longer described as "other text".
3. Low-Medium: an unexpected rmdir error during folder cleanup failed
   finalization and blocked Retry/Abandon. Cleanup is now best-effort and
   discloses the kept folder as preserved.
4. Low: an uninspectable agent root was treated as uninstalled and retired.
   Unknown presence now refuses retention with the original cause.
5. Low: a later `npx skills` claim blocked removal of dotfiles-owned skills. For
   owned copies it is now a note; new selections still refuse.
6. Low: the saved-path refusal told users to use an older revision, which cannot
   read state containing the new optional fields. The message now names the
   CODEX_HOME/CLAUDE_CONFIG_DIR remedy and the before-advancing rule.
7. Low: folder removal now re-binds each folder and stops under a redirected
   ancestor; the retired-target comment now states that removal still restores
   an adopted original (that is the baseline invariant, not a defect).

The reviewer's open question about Windows released policy bytes no longer
matters for completion: any non-matching file is kept and reported.

Hosted run on `67e7a41`: Windows core and quality failed only in the new
coverage. The native Sentinel factory lacked Windows known folders, and the
public Windows migration lifecycle indexed the first configured Sentinel target,
which presence gating correctly leaves empty on a fresh runner. That test now
takes the Codex location from the full consumer list and reads the published
targets from the saved baseline. `core (ubuntu-26.04-arm)` failed
`TestLegacyLinuxReaderWorkerExecStress` once ("continuous store descriptor
silently omitted") in procfs scanner code unchanged since `fee9f5d`, where it
passed; the next head reruns it, and a repeat is treated as a scanner race.

Personal-Mac migration completed every retirement except the old Homebrew
Ghostty, which still hosted the operator's session: chezmoi, gh-dash, 13 Homebrew
packages, Herdr, legacy tools and the Determinate Nix with nix-darwin and Home
Manager (vendor uninstall reported success; store and volume removed). The first
Nix attempt stopped at the kernel consumer scan on three old Herdr panes running
Nix-store `node`/`lazygit`; closing them and resuming completed it. The tmux
session driving two passes ended unexpectedly after retirement verification
with no crash report; each item's durable intent made the following pass resume
cleanly. Unrecognized older released caches (the pre-Sentinel agent-policy
cache and a Rose Pine tmux checkout) were moved to the Trash by hand.

Windows field report (2026-10-21, corporate laptop with OneDrive Known Folder
Move for Documents): migrate's setup apply finished every operation, then failed
finalization twice with `remove ...\Documents\PowerShell\.dotfiles-profile-<op>-1:
Access is denied.`, keeping its recovery journal. The owner's PowerShell listing
showed OneDrive had marked the operation workspaces `ReadOnly, Directory,
ReparsePoint`. Windows refuses to delete a read-only directory, and the pinned
Go 1.27.1 `os.Remove` clears that attribute only for files; `Remove-Item -Force`
(which clears it) succeeded on the same folder. Fixed: the installer's own emptied
profile/config workspaces, recorded cleanup directories, settings-lock stages and
created folders go through `removeOwned`/`removeEmptyDirectory`, which clear the
attribute on a real directory only, retry once and restore it when the folder
survives. Native Windows regressions first assert the `ERROR_ACCESS_DENIED`
refusal, then prove removal, attribute restoration for an occupied folder, and
profile/configuration finalization of a read-only workspace. Each failed attempt
cost a full re-verification of every installed tool (about 25 minutes with
endpoint scanning); no timeout or skip was added.

Operator incident in the same session: an assistant note wrongly said the
migration's `.dotfiles-config-*\previous` backups could be deleted after
migrate. They are the first baselines that removal restores and that a completed
migration verifies. After deletion migrate stopped with "configuration artifact
changed"; the owner restored the folders from the Recycle Bin and continued.
Changing that hard stop to an item-local note was considered and not made:
`TestLegacyMigrationRejectsChangedPreservationAndUnfinishedReleasedUpgrade` and the
driver deliberately fail closed on changed saved migration evidence; reversing
that needs an owner decision.

Shared Mac-layout keyboard (2026-10-21, owner request): Ghostty now binds
`super+c=copy_to_clipboard` and `super+v=paste_from_clipboard` in the shared
config. Ghostty 1.3.1 `+list-keybinds` on macOS shows the identical
`super+c=copy_to_clipboard:mixed`/`super+v=paste_from_clipboard` defaults with
and without the lines, so the Mac is unchanged; on Linux they add Super+C/V
beside the default Ctrl+Shift+C/V. `+validate-config` passes; the new
`tests/ghostty/clipboard_test.sh` assertions fail on the previous config. The
README documents the GNOME left Alt/Super swap and Super+V release the owner
applies on Ubuntu; dotfiles does not manage GNOME settings. Linux runtime use is
not yet verified on the owner's machine.

Open findings from the same field session, not yet changed:
- Linux launchers `dotfiles-zen.desktop` and `dotfiles-ghostty.desktop` lack
  `StartupWMClass` (`zen`; `com.mitchellh.ghostty` in the pinned DEB's own entry),
  so GNOME shows their windows under a generic icon and a second launcher click
  opens another Zen window. Ghostty additionally opened no window from the GNOME
  dock on a fresh Ubuntu; its cause is undiagnosed. The native desktop fixture
  launches Ghostty directly with `--gtk-single-instance=false`, so the real dock
  path is untested.
- An apply that ends with a pending adoption keeps its transaction, so the next
  menu offers only Resume/Abandon. Resume keeps the original approval and never
  offers adoption, leaving migrate stopped after setup on every rerun; the report
  does not name the Abandon, Choose tools, Replace-existing path.
- Zen on Linux cannot use Cmd+Option+Up/Down (Super+Alt) for tabs: Zen's patch
  maps them in Firefox's `ShortcutUtils`, whose `metaKeyIsCommandKey()` is
  macOS-only, and Zen's shortcut editor has no next/previous-tab action.

Withdrawn the same day: the owner clarified that the shared keyboard's Cmd key,
which Linux and Windows see as Alt, must act as Cmd in every application, as an
optional dotfiles capability. The Ghostty-only `super+c`/`super+v` bindings, their
test assertions and the README Alt/Super swap advice were removed again; the
config is back to its previous bytes. The system-wide design is pending.

Ghostty Linux crash (2026-10-21, fresh Ubuntu 24.04, X11): the dock launcher and
`ghostty` from a terminal both exited with `ghostty-bin: symbol lookup error ...
undefined symbol: gtk_css_section_get_bytes`. The pin was upstream's Debian 13
build of `mkasberg/ghostty-ubuntu@1.3.1-0-ppa2`, which imports that GTK 4.16
function; Ubuntu 24.04 ships GTK 4.14, and the released installer had selected a
per-release asset. Check reported ready because archive health verifies payload
bytes, and lazy binding lets `--version` succeed. The Ubuntu 24.04 builds have the
same layout, required files, `NEEDED` sonames, build-directory RUNPATH and
GLIBC_2.38 ceiling, without that import (objdump, amd64 and arm64), and GTK/
libadwaita stay compatible forward, so both Linux pins now use them (amd64
`478d4401...`, arm64 `91063815...`, matching upstream release digests).
`TestNativeGhosttyLoadsOnOldestSupportedUbuntu` prepares the real pin, installs
the catalog's APT libraries in a digest-pinned `ubuntu:24.04` container and runs
`ghostty --version` with `LD_BIND_NOW=1`; it also requires the Debian 13 build to
reproduce the field error, which proves the check would have caught it. It runs in
the existing Linux desktop lifecycle step list on both architectures; Renovate
tracks only that image's digest. Ubuntu 26.04 remains covered by the hosted GUI
lifecycle. The missing `StartupWMClass` finding above is still open.

Windows runtime retirement field report (2026-10-21, same corporate laptop): after
setup passed, retirement removed chezmoi, the earlier Herdr, pylatexenc, PSFzf,
psmux, tree-sitter and the released OpenCode/Pi policy, then stopped with every
Scoop and winget delta retained:
- Fixed: every Scoop package was reported "still required by Scoop apps ;" with
  no app named, because the consumer scan required a bucket and version in each
  installed app's record and the owner's `helm` was installed from a URL. Consumers
  now need only the installed manifest's `depends`; an unreadable manifest still
  keeps the package, now reported as "installed Scoop apps could not be checked".
  `TestLegacyScoopAppRequiresBucketProvenance` reproduces the field message before
  the change and covers URL-installed unrelated, consuming and unreadable apps.
- Open, by design: `winget export --source winget` exits 0x8A150012
  (`APPINSTALLER_CLI_ERROR_SOURCE_NAME_DOES_NOT_EXIST`): this managed laptop has no
  `winget` source, so no winget installation can be proved and every delta with a
  winget identifier stays retained. Pi packages under Scoop's nodejs-lts are at
  versions other than the released 1.0.4/0.82.1, and the Windows PowerShell PSFzf
  folder is not only the Gallery module; both are preserved. Retained items keep
  migrate's runtime phase unfinished with no explicit keep choice; whether to add
  one needs an owner decision.
- Open, cosmetic: retirement steps that run Windows PowerShell stream its
  first-use "Preparing modules" CLIXML progress into the live display. Changing
  the fixed script prelude changes saved command identities, so it was not
  touched here.

Launcher window classes (same day): the private Linux launchers for Ghostty and
Zen now declare `StartupWMClass=com.mitchellh.ghostty` and `StartupWMClass=zen`.
The values are the pinned Ghostty DEB's own desktop entry and the pinned Zen
1.23.2b archive's `application.ini` `RemotingName=zen`, both read from
hash-verified downloads. `TestLinuxLaunchersDeclareTheirWindowClass` fails with
the classes removed; the hosted Linux desktop lifecycle now also requires
`xwininfo -root -tree` to show a window with the launcher's class after its
`gio launch`. Correction to the earlier open finding: that lifecycle already
launches the published Ghostty launcher through `gio launch` (Ubuntu 26.04
under Xvfb); only the archive fixture launches the binary directly. The
undiagnosed Ghostty dock failure was the GTK symbol error recorded above.

Keep leftovers and finish (2026-10-21, owner decision on the Windows report
above): the open question is resolved. When a runtime retirement apply (or a run
with nothing to approve) finishes and every remaining resource is keepable, the
interactive phase offers one checklist of those resources, all checked, each with
its full scope lines. Accepted resources are saved in
`migration/retirement/kept.json` as resource ID to scope digest, after a fresh
inspection proves the scope still equals what was shown. `remaining()` then
reports them under "Kept" with the scope lines, Observe treats a kept
retained-only scope as finished instead of pending, and migrate continues to its
final check. Scoop retained text now names the app folder. Interrupted,
unreadable, uninspectable or blocked work suppresses the offer; machine requests
cannot record the choice. `TestLegacyRetirementKeepsReviewedLeftoversAndFinishes`
covers suppression, declining, keeping, the machine check, no repeated offer and a
changed leftover returning as unfinished.

Mac keyboard shortcuts on Linux (2026-10-21, owner request): the owner shares a
Mac-layout Logitech keyboard between a Mac and Ubuntu and wants the same physical
keys everywhere on Linux, optionally, leaving Windows unchanged. Design choices:
- Remapper: pinned static xremap 0.15.15 "full" ZIPs (amd64 `50814398...`, arm64
  `b1240c0e...`, matching the upstream release digests), run as the user from an
  XDG autostart entry. Its source shows unknown applications match `not:` filters
  and its automatic X11 probe uses display `:0`; the config therefore applies only
  to identified windows and the entry selects `--desktop x11`/`gnome` from
  `XDG_SESSION_TYPE`. The xremap author's own example also maps Alt_L to Ctrl_R.
- Mapping: modmap Cmd(Alt_L) to Right Ctrl and Option(Super_L) to Alt in
  identified windows; exact matches win over inexact ones, so Shift passes
  through navigation. Terminals map or swallow every Cmd combination. Cmd+Tab and
  Cmd+` release Right Ctrl, hold Alt and enter a `switching` mode that releases
  Alt when Cmd is released. Zen's Linux build cannot bind Cmd+Option+Up/Down
  itself; the remapper turns them into Ctrl+PageUp/PageDown, which Zen accepts.
- Privilege: the owner chose a logind-seat udev rule over `input` group membership
  or a manual step. The new `KeyboardAccessDriver` follows the Flatpak intent and
  worker-record pattern with sudo; unit tests use fake GNU-shaped programs for
  install, collision, preserved changes, removal and resume.
Verification at this commit: unit tests and both Linux cross-compilations pass
locally; `TestNativeMacKeyboardLifecycle` (root, Xvfb, uinput, real rule, real
xremap, autostart entry and config, terminal sweep, removal) and the gio entry
test run only in hosted Linux CI and are not yet observed. GNOME Wayland
application rules need xremap's Shell extension, which setup does not install.

First hosted run of the Mac keyboard lifecycle (`91b8257`, Desktop lifecycle on
ubuntu-26.04 and ubuntu-26.04-arm): every mapping scenario and the terminal Cmd
sweep passed against real xremap 0.15.15 under Xvfb, including Alt held across two
Tabs while Cmd was held, and the session-selected X11 backend. Deselection then
left the udev rule: the planner keeps machine-scope resources unless the owner
ticks a separate shared-removal box described as "another application may use
it". The rule serves only this capability, so `system.keyboard-access` is now user
scope with `Observation.Privileged`, which keeps the administrator marker and
removes it with the tool; `TestKeyboardAccessLeavesWithItsTool` fails with the old
scope. The same run's two Fresh Windows vendor jobs failed their first native VC++
inspection at about 30 seconds with exit status 1 and no diagnostic; they passed on
`c3f0f0e`, and this change touches no Windows vendor code. The next run decides
whether that repeats.

Hosted result for `67aae3b` (2026-10-22): 27/27 checks green. The Fresh Windows
vendor failure did not repeat; it is recorded as non-reproduction, with no cause
established. The two macOS jobs (core and Fresh Homebrew) were cancelled by
GitHub for runner capacity ("not acquired by Runner") and passed when re-run on
the same head, which created no new commit. `TestNativeMacKeyboardLifecycle`
passed on Linux amd64 and arm64, closing the first-hosted-run item above.

Known cosmetic issue, deliberately not changed (2026-10-22): a healthy Flatpak
application (Obsidian on the owner's Ubuntu) lists its live private store as
"Preserved files for inspection" after every apply. `FlatpakDriver.Observe` sets
`Preserved` to the store and `~/.var/app/<id>` whenever the store exists, and kept
receipts never refresh `After`, so a driver-only change would not clear existing
receipts. A proper fix touches shared receipt reporting; the line is harmless.

Kiro CLI and Kiro skills (2026-10-22, owner request). Decisions: offer Kiro CLI
(not the IDE, Web or Crew) on macOS and Linux; skip Windows, which only has a
machine-wide MSI with vendor self-update and an open uninstall bug
(kirodotdev/Kiro#11398); pin it and let dotfiles own its self-update switch.
Research: Kiro's install script reads `stable/latest/manifest.json` and downloads
`stable/latest/<file>`; fixed versions are served at `stable/2.33.1/<file>`, but
no versioned manifest exists (403). The 2.33.1 pins (macOS universal DMG
`f544b4f9...`, Linux glibc ZIPs `1ea73824...`/`8ce43b94...`) come from that
manifest and were re-downloaded from the versioned URLs and SHA-256 verified. The
Linux ZIP holds `kiro-cli`, `kiro-cli-chat` and `kiro-cli-term` in `bin/` plus two
legacy scripts that run `$HOME/.local/bin/kiro-cli`; the reviewed exclusions drop
only those scripts. Its `install.sh` copies the three binaries side by side into
`~/.local/bin` and may run `kiro-cli setup` (shell integrations); setup never runs
it. The macOS DMG's `Kiro CLI.app` bundles the desktop app (`kiro_cli_desktop`)
and the three CLI binaries in `Contents/MacOS`, each separately Developer ID
signed (team 94KV3E626L); the private copy is read through the DMG reader and the
desktop app is never launched. How `kiro-cli` locates `kiro-cli-chat` (own
directory or PATH) is not documented; both are satisfied because the helpers
are siblings and that directory is on PATH. Self-update: Kiro documents only
`app.disableAutoupdates` in `~/.kiro/settings/cli.json` for macOS/Linux
(`KIRO_NO_AUTO_UPDATE` is Windows-only), so `integration.kiro` owns that single
strict-JSON field through ProfileDriver. KIRO_HOME is honoured as Kiro documents
it (absolute, not a root, the home or its ancestor), for the settings file, the
skills root and the Sentinel policy target, which previously hard-coded `~/.kiro`
and did not count a Kiro CLI selection as present
(`TestSentinelPolicyFollowsKiroHomeAndTheKiroCLISelection` fails against that
code). Kiro documents KIRO_HOME for its V3 interface; a Classic-mode CLI may
still read `~/.kiro`. Skills: the new `skills.sentinel-kiro` root follows the
present-agent rule. With Kiro absent and no ownership, the selected resource
observes as satisfied (reused bookkeeping, nothing written) so machines without
Kiro never need action; without that branch the selection fails trying to
install and an uninstalled Kiro is planned for reinstall
(`TestSentinelSkillsLeaveAnAbsentKiroUntouched` and
`TestSentinelSkillsNeverRecreateAnUninstalledKiro` fail). The Impeccable engine
note is now emitted once, from the Codex/Copilot root, because every copy shares
the engine cache. Verification at this commit: gofmt, vet and the full Go suite
pass locally; the opt-in native agent CLI lifecycle now runs both Kiro commands
without credentials, the owned setting, Kiro skill copies and removal on macOS
and Linux, and has not run yet.

GitHub Copilot CLI (2026-10-22, owner scope addition in the same commit).
Decisions: pin the official github/copilot-cli stable standalone assets like Claude
Code (no npm, install script, MSI, winget or Homebrew), on darwin/arm64,
linux/amd64, linux/arm64 and windows/amd64 (the repository has no windows/arm64
target). v1.0.102's four assets match the release's SHA256SUMS.txt (re-downloaded
and hashed locally); each archive holds one executable (`copilot`, or
`copilot.exe` in the Windows zip); the Linux builds are dynamically linked glibc
ELF executables needing at most GLIBC_2.28. The upstream README lists PowerShell
6+ as a Windows prerequisite, so Windows adds the shared `tool.powershell`. The
native lifecycle explicitly removes it at the end so the disposable Windows state
stays empty for the later public migration fixture. Self-update: upstream reports
the `autoUpdate` setting is ignored (github/copilot-cli#2615, #4534), and #4534
says only `COPILOT_AUTO_UPDATE=false` stops re-execution into a cached build under
`~/.copilot/pkg`. The generated shell environment therefore sets it when
tool.copilot is selected, beside `DISABLE_UPDATES=1`
(`TestCopilotUpdatesAreDisabledOnlyForSelectedCLI` fails without it). No
`~/.copilot` file is written. A copilot launched outside that environment can
still self-update in place (#4728); Check reports the changed payload and
Update/Repair restores the pin, and the fingerprint is not weakened. Renovate:
`renovate.json` states that installer archive pins are manually reviewed and no
manager covers `installer/archive-pins.json`, so no Renovate tracking was added,
consistent with every other archive pin. Sentinel: the policy's Copilot target now
follows COPILOT_HOME and counts tool.copilot as present
(`TestSentinelPolicyFollowsCopilotHomeAndTheCopilotCLISelection` fails against the
hard-coded `~/.copilot` consumer); COPILOT_HOME must be absolute and canonical,
like CODEX_HOME. Skills: upstream 1.0.101 says "COPILOT_HOME and --config-dir stop
loading skills from ~/.agents/skills", and Copilot otherwise reads
`~/.agents/skills` (already copied). The new `skills.sentinel-copilot` resource
reuses the conditional-consumer path built for Kiro: it copies to
`$COPILOT_HOME/skills` only with COPILOT_HOME set and Copilot present, and is
otherwise satisfied without writing, so `~/.copilot/skills` never duplicates
`~/.agents`. Owned copies whose COPILOT_HOME disappears are refused for update
and removal until the original value is restored. The native agent lifecycle
runs `copilot --version` (with COPILOT_AUTO_UPDATE=false and no credentials) on
all four targets, then Check, unchanged-pin Update and removal; it has not run
yet.

### 2026-10-22 — Kiro CLI on Windows by private MSI unpacking; weekly agent CLI pin proposals

Owner decisions: offer Kiro CLI on Windows "regardless of the shitty approach",
confirmed as "Unpack MSI privately"; keep agent CLI pins with weekly bump PRs.

Findings before implementation (local, no vendor code executed): the pinned MSI's
tables were read with a stdlib compound-file reader. It installs one file,
`kiro-cli.exe` (401,055,928 bytes), into LocalAppDataFolder\Kiro-Cli; the normal
install also writes `HKCU\Software\Kiro\CLI` and prepends PATH through the
Environment table. CustomAction is empty and the AdminExecuteSequence contains only
CostInitialize, FileCost, CostFinalize, InstallValidate, InstallInitialize,
InstallAdminPackage, InstallFiles and InstallFinalize, so administrative
extraction runs no vendor code. Summary Word Count is 10 (compressed, no elevation
required). Authenticode signer: Amazon Web Services, Inc. (OU AWS Kiro, EV, DigiCert
G4 chain). Scoop's Main bucket ships this same MSI by administrative extraction
(`extract_dir` LocalApp\Kiro-Cli), which is cited precedent, not proof.

Design: `file` pin plus fixed `kiro_msi` preparation. msiexec parses its own
command line; Go argument quoting cannot produce `TARGETDIR="path with spaces"`
(the worker has no raw command line), so a fixed encoded script runs it through
system Windows PowerShell with paths as JSON environment data and built-in
modules bound. The image must be exactly the package copy plus
`LocalApp/Kiro-Cli/kiro-cli.exe`; otherwise it is refused and preserved. A
read-only copied executable becomes writable before fingerprinting. Authenticode
was not added: like the other archive pins, the reviewed SHA-256 is the byte
authority, and the bump script re-checks the MSI tables for each new release.

Weekly proposals: no earlier pin-preparation script existed, so
`scripts/update-agent-cli-pins.py` implements the recorded rules (Claude stable
channel, manifest gpg signature against fingerprint 31DDDE24…1A7ECACE from
Anthropic's documentation, npm SHA-512 SRI and binary checksums; Codex npm SRI
and layout via abbreviated packument and per-platform dist-tags; Copilot release
digest plus SHA256SUMS.txt and layout; Kiro manifest size/SHA-256, Linux ZIP layout
and MSI tables). Read-only `gh api` showed `default_workflow_permissions: read`,
`can_approve_pull_request_reviews: false`, all actions allowed with SHA pinning
required, so GITHUB_TOKEN cannot open the PR and its pushes would not trigger the
required checks; the workflow uses an optional owner GitHub App and otherwise
only verifies and uploads the patch. No repository setting was changed.

Verification: focused Go tests and 14 offline script tests pass; the Windows
shell switch test fails without the change. A real-network run of the script on
a scratch copy of the pins (local gpg) verified Claude's 2.1.308 signed manifest,
all npm SRI values and manifest binary checksums, Codex 0.163.0 layouts, the
current Copilot 1.0.102 digests/SHA256SUMS and Kiro 2.33.1 manifest values, ZIP
layouts and real MSI tables; it would bump Claude 2.1.287 to 2.1.308 and Codex
0.162.1 to 0.163.0. Those bumps are deliberately left to the workflow's reviewed
pull request; this change keeps the existing pins. Native proof (restricted-token
MSI unpacking, Windows Kiro in the agent lifecycle) and the workflow itself have
not run on GitHub yet.

### 2026-10-22 — Windows race budget exceeded by the new conditional skills tests

CI on 5dbe4cc: 25 of 27 checks passed; `windows` (Windows test suite) and
`core (windows-2025)` both hit `go test -timeout 20m` ("panic: test timed out
after 20m0s") while running ordinary fast tests. The previous green Windows run
(67aae3b) needed 1078s of the 1200s budget. Per-test timings from the verbose
core log attribute the overrun to the five new `sentinel_skills_conditional_test.go`
tests: 92s, 85s, 21s, 14s and 14s on Windows (67s on a Mac), because every
presence variant ran a full apply/check/removal copying all twelve bundles into
two or three roots. The budget is not raised. The tests now check presence and
selection variants (folder, discovered command, CLI selection, KIRO_HOME,
COPILOT_HOME) with read-only previews, run one combined KIRO_HOME/COPILOT_HOME
copy-and-remove lifecycle, and require only the roots each test exercises; the
default-home Copilot case keeps the ~/.agents copy. Local race time fell from 67s
to 13s. Mutation checks (dropping the selection, discovered-command or existing-
folder presence term) each still fail the new tests. Hosted Windows timing is
pending the next run.

### 2026-10-22 — Windows retirement: unrelated same-named processes and CLIXML display

Owner's corporate Windows run of `migrate.ps1` at 67aae3b: the Scoop fix worked
(chezmoi, win32yank, fd, fzf, gh, hyperfine, jq, lsd and make were uninstalled
through Scoop), then step 10/20 stopped: "Remove the earlier Node.js ...
windows-processes {"directories":["C:\\Users\\...\\scoop\\apps\\nodejs-lts"],
"names":["node"]} differs from the approved state: found "8148 C:\\Program
Files\\Adobe\\Adobe Creative Cloud Experience\\libs\\node.exe", approved """.
The gate matched the name `node` anywhere, so an unrelated application's
bundled node.exe blocked Scoop's Node.js; the same rule would block Scoop's
PowerShell 7 whenever any pwsh runs (including setup's own and the shell running
migrate) and Python whenever any python.exe runs. Scoop's own pre-uninstall test
is path-based. Fix: when a gate names directories (Scoop), a process with a
readable image path matches only by directory; names still match anywhere for
directory-less winget/Chocolatey gates and for processes whose image cannot be
read. Gate subjects keep their saved format, so the owner's interrupted Node.js
step resumes against the same approval. The native Windows process-gate test now
also requires a same-named program outside the gate's directory not to block;
with the previous script it is reported (names matched anywhere).

The earlier open cosmetic item (first-use "Preparing modules" CLIXML in the live
display) is resolved at the display layer: the progress view hides the
`#< CLIXML` marker and `<Objs>` documents, including documents split by its
4096-byte wrap, and keeps the plain Scoop lines ("Uninstalling 'fd' (10.4.2).").
Captured evidence, logs and failure tails are unchanged; fixed retirement
scripts were not edited because their bytes are saved command identities.
`TestProgressHidesPowerShellCLIXMLRecords` fails before the change.

### 2026-10-22 — Shared navigation keys; AeroSpace kept as configured

Owner request, refined twice before delivery: first a single-screen "every app
full screen" AeroSpace policy (accordion, no padding or gaps) with Ghostty and
Zen pinned to workspaces 1 and 2; then "don't pin apps to Aerospace ... teach me
how to set up the pinning"; finally "Keep Aerospace vanilla and teach me ... how
to pin and use it ... then mimic that setup as easily as possible on Linux",
with "vanilla" confirmed as today's config. `aerospace/aerospace.toml` and
`tests/aerospace/keymap_test.sh` are therefore byte-for-byte unchanged (the
drafted accordion/gap policy, the two `on-window-detected` homes and a test
requiring them were reverted). README gains a beginner AeroSpace guide, the
stacked accordion layout as an option, and step-by-step pinning with
`aerospace list-apps` and `on-window-detected`. The IDs it cites were checked
against what the catalog installs: the pinned `ghostty-macos-universal.zip`
1.3.1 (SHA-256 matching its pin) has `CFBundleIdentifier = com.mitchellh.ghostty`;
Zen's cask identity in `brew_application.go` is `app.zen-browser.zen`. No offline
AeroSpace validation exists: its CLI, including `reload-config --dry-run`, talks
to the running app, which waits for the Accessibility grant.

Linux: a new `exact_match` keymap entry after Desktop and before Terminals maps
`Alt_L-KEY_1..9` → `Super-KEY_1..9`, `Alt_L-Shift-KEY_1..9` →
`Super-Shift-KEY_1..9` and `Ctrl_L-Alt_L-L/H` → `Alt-Esc` / `Shift-Alt-Esc`,
mirroring AeroSpace's workspace, move-to-workspace and focus right/left keys.
GNOME gives `Super+1...9` to the dock by default, so README teaches a one-time,
reversible setup the user runs: fixed nine workspaces (`org.gnome.mutter
dynamic-workspaces false`, `num-workspaces 9`), dash-to-dock `hot-keys false`,
cleared `switch-to-application-N`, and `switch-to-workspace-N` /
`move-to-workspace-N` on `<Super>N` / `<Super><Shift>N`; setup writes none of it
and installs no extension. Defaults were read from source: gsettings-desktop-
schemas `cycle-windows` `['<Alt>Escape']`, `cycle-windows-backward`
`['<Shift><Alt>Escape']`, `switch-to-workspace-1` `['<Super>Home']` and 2...9
empty, `move-to-workspace-1` `['<Super><Shift>Home']` and 2...9 empty,
`num-workspaces` 4; gnome-shell `switch-to-application-1` `['<Super>1']`;
dash-to-dock `hot-keys` true and `app-hotkey-1` `['<Super>1']`. xremap v0.15.15
source (`event_handler.rs` `find_keymap`/`send_key_combo`, `key_combo.rs`)
confirms that exact matches are tried first in config order, `exact_match`
entries are skipped in the inexact pass, `Alt_L`/`Ctrl_L` name the left keys,
and held modifiers absent from the output are released around the emitted key.
Hence Cmd+digit keeps Alt+digit (Ghostty tabs) in terminals and Ctrl+digit in
apps. Option+Tab has no rule. `TestNativeMacKeyboardLifecycle` now requires, in
an app and a terminal window, Option+1/9 → Super+1/9, Option+Shift+1/9 →
Shift+Super+1/9, Ctrl+Option+L/H → Alt+Esc / Alt+Shift+Esc with no Alt or Ctrl
left pressed, Option+Tab → Alt+Tab, Ctrl+Option+J unchanged, Cmd+1 → Alt+1
(terminal) / Ctrl+1 (app), and plain keys in an unidentified window. With the
previous keymap the new checks see Alt+1, Alt+Shift+1 and Ctrl+Alt+L. Hosted
result at 60a3148: `TestNativeMacKeyboardLifecycle` passed on ubuntu-26.04
(105.94s) and ubuntu-26.04-arm (105.82s).

### 2026-10-22 — Mac keyboard keymap did not reload after a checkout update

Owner report on Ubuntu after `git reset --hard origin/feat/interactive-installer`
and the one-time GNOME workspace setup: workspaces existed, but Option+2 did not
switch to workspace 2 ("Somethin is clashing (perhaps Zen shortcuts??)"). The
running xremap still used the previous keymap, so Option+2 reached Zen as Alt+2,
Firefox/Zen's "select tab 2". Cause, from xremap v0.15.15
`src/platform_linux/config_watcher.rs`: `--watch=config` adds an `IN_MODIFY`
watch on the configuration path (inotify follows the live link to the checkout
file's inode) and `IN_CREATE`/`IN_MOVED_TO` on the path's parent directory,
which is `~/.config/xremap`. Git replaces the checkout file (unlink and create),
so the watched inode is deleted without `IN_MODIFY` and the creation happens in
the checkout's `xremap/` directory, which was not watched. The keymap only
changed at the next login. Fix: the autostart entry resolves the link with
`readlink -f` before exec (falling back to the link path when resolution fails),
so xremap watches the checkout file and its directory; `config_watch_debounce_ms:
500` (default 0) reloads only after the new file is written. Existing installs
get the new entry through Update and use it from the next login.
`TestMacKeyboardAutostartRunsXremapForTheSession` now requires the resolved
checkout path (it received the link path before), and `TestNativeMacKeyboardLifecycle`
replaces the checkout keymap as git does (unlink, create with the same bytes,
mode and owner) and requires a new "Config Reloaded" from the running remapper
within 15 seconds, then Option+1 → Super+1 again. Hosted proof is pending. The
owner's immediate workaround: restart the remapper (`pkill -x xremap`, then
`gio launch` the autostart entry if GNOME did not restart it) or log in again.

### 2026-10-22 — First hosted Windows run of Kiro CLI (60a3148): three failures

`core (windows-2025)` failed in the steps added for Kiro on Windows; the other 25
checks, including both Linux desktop lifecycles with the extended Mac keyboard
navigation test, passed.

1. `TestNativeAgentCLIInstalledApplicationLifecycle`: "Kiro CLI can still replace
   its pinned payload or lost a personal setting" with the fixture's
   `~/.kiro/settings/cli.json` holding only its personal key. The public machine
   fixture sets HOME, CODEX_HOME and CLAUDE_CONFIG_DIR, but on Windows the
   installer resolves home through known folders, so `integration.kiro` wrote the
   owned field to the runner's real profile. Product behaviour was correct; the
   fixture now also passes `KIRO_HOME=<fixture home>/.kiro`, which equals the
   POSIX default.
2. `TestNativeKiroMSIUnpacksPrivatelyWithoutElevation` hit the step's 15-minute
   timeout. The parent's stack shows it waiting for the restricted child process
   itself for 14 minutes. A test binary run directly has no default timeout, so the
   child never printed where it hung. Causes are not yet proven; two changes:
   the product script now waits for msiexec through System.Diagnostics.Process
   with a 10-minute bound instead of `Start-Process -Wait` (documented to wait for
   descendants too), and `runRestrictedProfileProcess` passes `-test.timeout=12m`
   and collects output through a file, so a hang prints every goroutine and a
   lingering descendant cannot block on an open pipe. Unpacking without Windows
   Installer was considered and rejected for now: the MSI's single embedded
   cabinet (`#cab1.cab`, one folder, one file) is LZX-compressed (typeCompress
   0x1203).
3. `TestNativeWindowsPublicMigrationSetupLifecycle` refused the setup state left
   by the failed agent lifecycle ("public migration requires empty completed
   setup state"), a cascade of (1).

### 2026-10-22 — Linux Obsidian: from private Flatpak to the portable build

Owner report on the Ubuntu 24.04 corporate laptop: Obsidian did not open from the
dock ("same behavior as what we had with Ghostty"). Running the launcher's command
in a terminal printed "bwrap: setting up uid map: Permission denied" and "error:
ldconfig failed, exit status 256". Diagnosis with the owner:
`kernel.apparmor_restrict_unprivileged_userns = 1` (Ubuntu's default); the stock
`/etc/apparmor.d/flatpak` and `/etc/apparmor.d/obsidian` profiles were present
but read `flags=(complain)` instead of Ubuntu's `flags=(unconfined)`; `dpkg -V
apparmor` reported both modified ("??5?????? c"); 89 profiles were in complain
mode, changed on 2026-10-15, while `/etc/apparmor.d/code` stayed `unconfined`.
`aa-exec -p flatpak -- bwrap --unshare-user ...` failed with the same uid-map
error. After the owner restored the flatpak profile to `unconfined` by hand,
both bwrap checks exited 0, but Obsidian still did not start: "error:
app/md.obsidian.Obsidian/x86_64/stable (commit ba4cac4f...) not installed" and
"[2 preload-host-spawn-strategy] Failed to let stub process know of exit: Bad
file descriptor (errno 9)". Electron's zypak helper asks the Flatpak portal to
spawn its sandboxed helpers; the portal runs `flatpak run --commit ...` against
the standard installations and cannot see the installer's private store. The
same command with Electron's `--no-sandbox` opened a window (owner confirmed).
Hosted CI never saw either failure: GitHub's image relaxes the userns
restriction, and the native Flatpak probe's `flatpak-spawn --sandbox` passed on
Xvfb with a fresh session bus, so it did not reproduce zypak's spawn on a real
desktop.

Owner decision ("let's go with #2"): Linux Obsidian uses the official portable
`obsidian-1.14.4.tar.gz` / `obsidian-1.14.4-arm64.tar.gz` (SHA-256 e70b7cd6…/
fc212804…, equal to GitHub's release digests; identical layouts; `chrome-sandbox`
not setuid). The new `tool.obsidian-portable` declares `obsidian`,
`resources/icon.png` and `resources/obsidian.asar` as required files and no
command, so nothing is added to PATH. The launcher keeps
`applications/dotfiles-obsidian.desktop`, runs `obsidian --no-sandbox %U`, and
declares `StartupWMClass=md.obsidian.Obsidian`, the value in Obsidian's own
1.14.4 `.deb` desktop entry (its `app.asar` package.json sets `desktopName`
`md.obsidian.Obsidian.desktop`). The Electron libraries match OpenWhispr's
(GTK 3, ALSA, NSS, GBM, libstdc++). The preview discloses the disabled sandbox.
Setup changes no AppArmor or sysctl and installs no setuid helper; the owner's
manual flatpak-profile change is theirs to keep or undo.

Transition: `tool.obsidian` keeps its Flatpak binding (recovery/removal only);
the capability and launcher now require the portable build on Linux. The
FlatpakDriver stays for observation and removal, and the desktop driver's
Flatpak launcher code and field are gone. `TestObsidianFlatpakReceiptsMoveToThePortableBuild`
plans, from saved receipts, an install of the portable build, a repair of the
owned launcher, `tool.flatpak` retained, and the earlier Flatpak removed by
Choose tools but retained by Update (Update never removes); it fails on the
previous catalog. `TestObsidianDesktopLauncherDisablesChromiumSandboxAndForwardsLinks`
replaces the Flatpak launcher consumer test, and
`TestFlatpakApplicationPublishesAndRestoresItsDesktopLauncher` is removed with
that launcher. Native: `TestNativeFlatpakApplicationLifecycle` recreates an
earlier Flatpak through a test-only capability, then one approved Choose tools
removes it and installs the portable build and launcher, keeping `~/.var/app`
data. The Linux desktop lifecycle now launches Obsidian on amd64 and arm64 under
the stock restriction, set by the new `prepare-disposable-linux-host.sh userns`
recipe and asserted by the test, and requires a window with the declared class
and `--no-sandbox` in the owned process arguments. Hosted results are pending.

### 2026-10-22 — Windows race budget raised to 30 minutes; Kiro MSI unpack evidence

1dca9af: 26 of 27 checks passed; `core (windows-2025)` failed. Its full race step
(`go test -race -timeout 20m ./...`) timed out in an ordinary 6-second Windows
Terminal test. That step took 962s at 67aae3b and 1057s at 60a3148; at 1dca9af
the same tests summed to 1279s instead of 1098s (16% slower across many tests,
not one regression), and the separate `windows` job passed the same commit. The
owner chose to raise the full race-suite limit to 30 minutes (Makefile, test.ps1,
installer-engine core step) and the `windows` job to 50 minutes. Assertions are
unchanged; a hung test still fails at the limit.

The same run proved the KIRO_HOME fixture fix
(`TestNativeAgentCLIInstalledApplicationLifecycle` passed, 598s) and gave the MSI
diagnostics the harness change was for: as the restricted ordinary user, msiexec
ran the full 10-minute bound without creating its verbose log or any file in the
target image ("msiexec did not finish unpacking within 10 minutes"). It blocks
before starting work. Windows Installer is therefore dropped for this step: the
next change reads the cabinet stream from the verified MSI in Go and unpacks it
with the system `expand.exe`, which needs neither the installer service nor
elevation.

### 2026-10-22 — Kiro CLI on Windows: cabinet read and expand.exe (kiro_msi revision 1)

The pinned 2.33.1 MSI (SHA-256 f1c7e603...b190dedd) was read locally, without
executing anything: a version-4 compound file (4096-byte sectors, 64 FAT sectors,
no DIFAT sectors); Media has one row, Cabinet `#cab1.cab`, an embedded stream whose
packed name units are 0x4126 0x3865 0x41be 0x4164; File has one row, `KiroCliExe`,
FileName `kiro-cli.exe`, FileSize 401055928. The cabinet is `MSCF`, cbCabinet
267057411, one folder (typeCompress 0x1203, LZX with a 2^18 window), one CFFILE
`KiroCliExe` of 401055928 bytes, flags 0.

Preparation revision 1 (`kiro_msi_revision: 1`, `kiro_msi_file_size: 401055928`)
copies that stream with a stdlib compound-file reader (header, DIFAT, FAT, regular
sector chains; refuses links, mini streams, duplicate or missing names, loops,
short chains and sectors outside the file), checks the cabinet header, and runs the
system `expand.exe -F:KiroCliExe <cab> <empty private dir>` through the native
worker. Exactly one regular `KiroCliExe` of the reviewed size starting with `MZ`
becomes `kiro-cli.exe`; anything else is refused with the stage preserved. The
reader reproduced the pinned MSI's cabinet byte for byte (SHA-256 7a3dd53d...
7f16298f, identical to an independent Python sector walk). expand.exe is discovered
from the native system directory at the process boundary; the archive driver's
PowerShell field, the msiexec script and its log reader are gone. Revision 0 pins
stay readable, and an unfinished revision-0 preparation is refused for explicit
recovery rather than re-run. `scripts/update-agent-cli-pins.py` now checks
CustomAction (empty), Media (one `#cab1.cab`) and File (one `KiroCliExe`,
kiro-cli.exe) and records the File size in the pin; the msiexec-only
AdminExecuteSequence and Directory checks were dropped. Why msiexec blocked for
the restricted token was not established; hosted proof of revision 1 as an
ordinary user is pending.

### 2026-10-22 — First hosted run of portable Obsidian (7662e51): two test faults

Both Linux desktop lifecycles and both Linux core jobs failed on the new
Obsidian tests; the product behaved. The desktop runners already reported
`kernel.apparmor_restrict_unprivileged_userns=1` before the host recipe ran, and
the published launcher started Obsidian ("Loaded main app package ...
tool.obsidian-portable/.../resources/obsidian.asar", update check "App is up to
date"). (1) `TestNativeDesktopInstalledApplicationLifecycle/obsidian`: "Obsidian
was not started through its --no-sandbox launcher". Chromium rewrites its process
title on Linux, so `/proc/<pid>/cmdline` can hold the command line as one
space-separated string; the check split only on NUL. It now also splits on
spaces. (2) `TestObsidianDesktopLauncherDisablesChromiumSandboxAndForwardsLinks`:
"gio: Unable to load application information". The test's install path contained
a literal `%`; GLib's GDesktopAppInfo checks that the Exec program exists before
it expands `%%`, so it looked for a path with `%%`. The test path drops `%`
(the Exec escaping itself is unchanged and spec-correct) and the limitation is
documented at `desktopEntry`: a program path containing `%` cannot be started
through GIO.

### 2026-10-22 — cec98fb: cabinet copy works; expand.exe needs relative names

25 of 27 checks passed, including every Linux job: portable Obsidian launched
under the stock userns restriction on amd64 and arm64, the Flatpak-to-portable
transition and the keymap reload passed. The Windows race suite passed in 1218.967s
under the new 30-minute limit (it would have failed the old 20-minute one).
`core (windows-2025)` failed in three Kiro-related steps:
1. `TestNativeKiroMSIUnpacksPrivatelyWithoutElevation` failed in 10s, not 10
   minutes: the Go reader copied the embedded cabinet exactly (267057411 bytes,
   the reviewed cbCabinet), then expand.exe printed "Can't open input file:
   ...\private kiro cli ?\...\.prepare-kiro.cab". expand.exe opens files through
   the system code page and the fixture's `ü` became `?`; a real profile such as
   C:\Users\Müller would fail the same way. Fix: `nativeCommand` gains an optional
   `Directory` (omitted when empty, so earlier records keep their encoding and
   hash; validated absolute and clean); expand.exe runs inside the stage with
   `.prepare-kiro.cab` and `.prepare-image`. `TestNativeWorkerCommandBoundary` now
   proves a non-ASCII working directory reaches the child and a relative one is
   refused.
2. `TestNativeAgentCLIInstalledApplicationLifecycle` failed in Codex skill
   discovery: "Build Tools inspection membership changed during termination;
   completion is unproved; baseline(total=3 active=2) final(total=4 active=0)".
   A Codex helper joined the private job while it was being reaped; the strict
   membership rule refused it as designed. It passed at 1dca9af; treated as an
   intermittent race, not a regression.
3. `TestNativeWindowsPublicMigrationSetupLifecycle` refused the leftover state
   of (2).
`Neovim lifecycle (macos-26)` failed once with "plugin differs from lock:
nvim-treesitter"; the same lifecycle passed on Linux and Windows in this run and
on macOS earlier, and no Neovim input changed. Recorded as non-reproduction
pending the next run.

### 2026-10-22 — Kiro CLI on Windows: Go cabinet/LZX decoder (kiro_msi revision 2)

The next hosted runs refused revision 1 both ways: as an administrator expand.exe
left `.prepare-kiro.cab` instead of `KiroCliExe` in the image ("unreviewed layout
(.prepare-kiro.cab)"), and as the restricted user it could not start inside the
stage ("fork/exec C:\Windows\system32\expand.exe: The directory name is
invalid."). Decision: no Windows unpacker at all.

- `installer/cabinet_lzx.go` decodes the cabinet with the standard library only,
  written from Microsoft's published cabinet format and the LZX description in
  [MS-PATCH], not from libmspack (LGPL). It accepts only the reviewed shape:
  `MSCF` 1.3, cbCabinet equal to the file, one folder, one file, no flags (so no
  reserved areas or spanned cabinets), CFFILE at 44, data immediately after the
  name, LZX with a 2^15 to 2^21 window, cCFData equal to the frame count, and
  every non-last CFDATA holding one 32 KiB frame. Non-zero CFDATA checksums are
  verified. The decoder covers verbatim, aligned and uncompressed blocks,
  pretree-coded delta lengths (including runs 17, 18 and 19), R0 to R2, frame
  realignment, the ring window and per-frame E8 translation. Complete Huffman
  codes are required; only the length tree may be empty. Matches cannot cross a
  block or frame or reach before the output start or beyond the window. Memory
  is the window plus one frame, and the output is streamed.
- Preparation revision 2 copies the cabinet stream, streams the decoded file into
  `.prepare-kiro.exe` while hashing it, and requires `kiro_msi_file_size` and the
  new `kiro_msi_file_sha256`. Any refusal preserves the MSI, cabinet and stage.
  Pins of revisions 0 and 1 stay readable and are refused for preparation.
  `DiscoverWindowsExpand`, `ArchiveDriver.Expand`, its platform/application/cmd
  wiring and `nativeCommand.Directory` (with its worker test) are removed. The
  KIRO_HOME fixture fix stays.
- The bump script extracts the cabinet stream and hashes `KiroCliExe` with
  libarchive's `bsdtar -xOf` after `bsdtar -tf` lists exactly that one entry; the
  workflow installs `libarchive-tools`. Using an independent decoder means the
  pin never certifies the Go decoder's own output.
- Local evidence (macOS, decoding only, no vendor code executed):
  - the real cabinet (267,057,411 bytes) decodes to 401,055,928 bytes with SHA-256
    908c4925f70660b0360f206b2a5b2ef27f7982dc1cb01a0ee737b3006f7a8728 in 1.85 s;
  - `prepareKiroMSI` on the pinned MSI takes 2.8 s and leaves only `kiro-cli.exe`;
  - libarchive 3.7.4 gives the same SHA-256;
  - a probe of that stream found 461 aligned, 789 verbatim and 2,818
    uncompressed block changes sampled at frame ends, an E8 translation size of
    12,000,000, one trailing padding word and no unread input.
- Synthetic tests use a test-only LZX encoder that shares no code with the
  decoder except the format tables and the checksum. They cover:
  - every block kind at windows 2^15 and 2^21, with CFDATA splits at word and odd
    offsets;
  - all repeated-offset forms, zero and value length runs, and the window wrap;
  - E8 edge cases, including untranslated window data and an E8 inside an operand;
  - layout, checksum, truncation and invalid-stream refusals;
  - a checksum known-answer test.

  Twelve deliberate decoder mutations were all caught. Four only after adding
  targeted cases: E8 skip, R1 swap, checksum tail and the offset bound.
- Still to prove: the hosted restricted-user `TestNativeKiroMSIUnpacksPrivatelyWithoutElevation`
  and the Windows agent lifecycle, both unchanged in intent.

## 2026-10-22 — Kiro CLI withdrawn from Windows

Owner decision: "leave it out of Windows ... remove this altogether", and record
that the gap is a limitation of how Kiro distributes the tool. Kiro publishes its
Windows CLI only as an MSI installer; the install it performs is machine-wide,
elevated and self-updating, and the owner declined extracting the program from
that installer. Removed: the Windows `tool.kiro` pin, the `kiro_msi` archive
preparation and its MSI, cabinet and LZX readers with their tests, the hosted
ordinary-user unpack step, the bump workflow's bsdtar step, the bump script's MSI
layout checks and the Windows `KIRO_NO_AUTO_UPDATE` profile line. The catalog
returns to macOS/Linux-only `kiro-cli`, `tool.kiro` and `integration.kiro`;
Sentinel policy and skills still reach a Kiro the user installs on Windows.
Retained: the KIRO_HOME fixture variable, the restricted-child timeout/file-output
harness and the 30-minute Windows race budget. Evidence for the remaining targets:
`TestNativeAgentCLIInstalledApplicationLifecycle` passed in core macOS (322 s),
Ubuntu amd64 (150 s) and arm64 (148 s) at 845859a (run 39766959358). The earlier
Windows findings stay in `docs/reviews/field-findings.md` item 10 as history. The
user's Windows migration finished on 2026-10-22 without Kiro.

## 2026-10-23 — OpenWhispr Linux launcher class; personal Mac migration complete

OpenWhispr's Linux launcher lacked `StartupWMClass`, so GNOME showed its window
under a generic icon (owner screenshot, corporate Ubuntu laptop). Evidence: the
pinned `OpenWhispr-1.10.3-linux-x64.tar.gz` (SHA-256 verified) has
`package.json` name `open-whispr` with no `productName` or `desktopName`; its
`main.js` calls `app.setDesktopName("open-whispr.desktop")` on Linux; the bundled
Electron is 41.10.7, whose `native_window_views.cc` sets both WM_CLASS fields
from `GetXdgAppId()` (`CHROME_DESKTOP` without `.desktop`), falling back to the
app name, also `open-whispr`. The vendor launcher adds `--ozone-platform=x11`
under Wayland, so GNOME sees that X11 class there too. The earlier class
regression had listed OpenWhispr as having no class instead of checking it.
Correction: `StartupWMClass=open-whispr` in the launcher recipe; the unit
regression fails on the previous recipe and passes now; the native Linux
desktop assertion now includes OpenWhispr's window class. Hosted proof runs on
the pushed head.

Personal Mac: migration completed on 2026-10-23 (old Ghostty, Herdr and Nix
retired; fontconfig kept because `cairo` and the user-requested `poppler` depend
on it; the Hack Nerd Font kept because setup reuses the existing family). The
runtime phase stopped twice before that. Once, an agent's concurrent Check held
the installer lock; once, the sudo ticket had expired before the read-only
`/etc/fstab` inspection. `sudo -v && ./migrate.sh` completed it. After the
restart `/nix` is gone (no volume, mount, `_nixbld` accounts, launchd services
or commands) and the read-only setup Check is ready. An agent shell without a
terminal cannot reuse the user's sudo ticket, so its migration Check reports the
same `/etc/fstab` inspection as needing `sudo -v`; that is expected there.

Hosted result for `366f7d9` (2026-10-23, doc and Linux launcher changes only):
Fresh Windows vendor (Build-Tools) failed `TestWindowsBuildToolsNativeOwnedLifecycle`
at its first inspection of registered Microsoft Build Tools, "Build Tools
inspection failed: context deadline exceeded" after 30.07 s (run 39901962007,
job 120245877286). `bb6a092` passed all 27 checks, and nothing since touches
Windows vendor code. Like the VC++ inspection failure at `91b8257`, it ended at
the 30-second inspection bound on a fresh container; no cause is established.
Recorded as a second occurrence; the bound is unchanged.

Hosted result for `bab9926` (2026-10-23): 26/27. Fresh Windows vendor
(Build-Tools) passed. core (macos-26) failed `TestNativeLegacyRetirementDarwinReleasedSetup`
before the release ran: "prerequisite preparation changed a pre-existing package
openssl@3", 3.5.9 unlinked became 3.5.9,3.6.3 linked (run 39903643201, job
120251125466). Cause: the runner image changed from 20261013 (OpenSSL 3.6.5) to
20261021 (OpenSSL 3.5.9, current Homebrew `openssl@3`); the fixture installs
python@3.12 from the 2026-07-26 core snapshot, whose `openssl@3` is 3.6.3, and
Homebrew upgrades an outdated dependency even with `HOMEBREW_NO_INSTALL_UPGRADE`.
Owner decision: keep the guard exact on ownership evidence (source, pin,
protection, requested/automatic, every earlier keg, runtime dependencies) and
accept only an added, linked keg, logged. `legacyPrerequisitePreserved` and
`TestLegacyPrerequisitePreservationAcceptsOnlyAnAddedKeg` (exact observed values
plus ten refused changes) fail under the previous exact comparison and pass now.

Hosted result for `e9f728b` (2026-10-23), while running: `ubuntu` failed "Install
deps" on HTTP 502 from GitHub's release download of Starship (transient; re-run by
the next push). Fresh Windows vendor (Build-Tools) failed again at its first
inspection, "context deadline exceeded" after 30.15 s (run 39910598203, job
120272447937). Runner images: both Build Tools failures (366f7d9, e9f728b) ran on
windows-2022 20261021.618 (host build 20348.5761, October update); all six
Fresh Windows vendor job passes since bb6a092 ran on 20261012.601 (build 20348.5622).
The pinned `dotnet/framework/runtime:4.8-windowsservercore-ltsc2022` container is
older than both. Leading suspect: the first `Get-CimInstance Win32_OperatingSystem`
in a cold container (the VC++ inspection makes the same call); not established.
Added a final always-run, continue-on-error step that times a separate cold
container's start, first and second WMI query and an Authenticode check, with host
and container builds, so both image versions report evidence. The 30-second bound
is unchanged pending that evidence.

Hosted result for `49076d1` (2026-10-23): 27/27 checks green. Fresh Windows vendor
(Build-Tools) passed on windows-2022 20261021 (host build 20348.5761), so the
first-inspection timeout is intermittent on that image. The diagnostic container,
started after the lifecycle, reported container build 20348.4171, first WMI query
584 ms, second 17 ms, Authenticode 280 ms and a 4.96 s PowerShell probe (20261012:
593 ms, 23 ms, 309 ms, 6.14 s): WMI does not explain a 30-second first inspection.
New, unproven hypothesis: the first container started from the freshly pulled image
is slow, and in the Build-Tools job that was the lifecycle's own container (the
VC-runtime job runs its Python check first). By owner choice the timing step now
runs right after the pull, timing the first PowerShell container, a bare container
and a second PowerShell container; the lifecycle runs after it. core (macos-26) ran
on image 20261013, which needs no `openssl@3` change, so the macOS prerequisite rule
from bab9926 is still proved only by its unit regression; native proof waits for a
run on the 20261021 macOS image.

Hosted result for `451a525` (2026-10-23): 24/27. Both Fresh Windows vendor jobs
passed; the diagnostic before the lifecycle measured the first PowerShell
container after the pull at 31.9 s (windows-2022 20261012) and 34.7 s (20261021),
a bare container at 4.3 to 6.9 s and a second PowerShell container at 8.5 to 9.8 s,
with the script itself under 7 s. Failures:

- core (windows-2025) `TestWindowsBuildToolsNativeReuseAndEnvironment` and Neovim
  lifecycle (windows-2025), whose installer stopped with "inspect C/C++ toolchain:
  ... Build Tools inspection failed: context deadline exceeded" (runs 39945040493,
  jobs 120385511014 and 120385510979). Both ran on windows-2025 20261021.618 with
  Visual Studio installed; that test took 28.7 s for three inspections at bb6a092
  (20261007) and 40.2 s at 49076d1 (20261021). Owner decision: raise the Build
  Tools inspection bound to 120 s (`buildToolsInspectionTimeout`); output limits
  and the five-second termination bound are unchanged, VC++ stays at 30 s. No
  test depended on the old deadline; hosted Windows runs are the proof.
- core (ubuntu-26.04) `TestLegacyLinuxReaderWorkerExecStress`: "continuous store
  descriptor silently omitted" (job 120385511011), the second occurrence after
  67e7a41. The ledger had recorded that a repeat means a scanner race. Reproduced
  in a disposable `ubuntu:24.04` container on Docker Desktop's arm64 Linux
  6.12.76 kernel with the pinned Go: 2 failures in 200 runs, 5 in 300. An
  instrumented scratch copy showed `refuse` omitting the process because
  `noUserspace` saw pidfd readiness, while status read moments later was State R,
  Threads 1. A standalone C probe (worker-thread exec loop, observer) measured 14
  ready polls among about 6 million freshly opened pidfds, each followed by a live
  process (State S, Threads 2) and a not-ready re-poll; a pidfd held across the
  execs never polled ready in 16 million polls, and neither pidfd_open nor
  /proc/<pid> failed there. The kernel source did not show the window directly;
  the fix does not depend on it. `exited` now requires readiness plus a fresh
  pidfd_open of the same PID failing with ESRCH, EINVAL or ENOENT; an unreaped
  zombie is proved by its existing Z/X one-thread status instead. A directory that
  cannot be opened now reports "cannot inspect the directory of live process",
  a refusal like the others. Verification in the container: all `TestLegacyLinux*`
  tests pass with -race, 400 stress runs pass (about 2.5% of scans safely refused),
  and the new `TestLegacyLinuxReaderTransientPidfdReadinessIsNotExit` fails on the
  previous code ("transient pidfd readiness hid a live store consumer") and passes
  now. `TestLegacyLinuxReaderHandleWaitsForAllThreads` now proves the killed,
  unreaped fixture through its status, since readiness alone no longer counts.
- core (macos-26) passed again on image 20261013, so the macOS prerequisite rule
  still awaits a run on the 20261021 image.

Hosted result for `33b61b7` (2026-10-23): 27/27 green. The 120-second Build Tools
bound held on windows-2025 20261021 (core reuse test 38.08 s; Neovim lifecycle
passed). The exec stress test passed on Ubuntu amd64 and arm64, safely refusing 4
to 7% of scans. core (macos-26) again ran on image 20261013, so the macOS
prerequisite rule still has only its unit proof. The owner merged PR #87 as
`a64f804` the same day.

## 2026-10-23 — zsh from bash terminals (follow-up to PR #87)

Owner report from the corporate Ubuntu laptop: Ghostty opened bash. `getent
passwd` gives `/bin/bash` and `/etc/passwd` has no line for the account, so it is
a directory account that `chsh` cannot change; IT will not change it. Herdr starts
`$SHELL` (empty `default_shell` means `$SHELL`, then `/bin/sh`, per its v0.9.3
template), as do tmux and Neovim terminals, so a `.bashrc` `exec zsh` alone would
leave Herdr panes in bash. Owner choice: an optional capability, `zsh-handoff`
("zsh from bash terminals", Shell essentials), requiring `config.zsh` and a new
graph-only `integration.zsh-handoff`. The shared init hands an interactive bash on
a terminal to the observed zsh (`/bin/zsh` on macOS, `/usr/bin/zsh` on Linux),
exporting `SHELL` and `DOTFILES_ZSH_HANDOFF=1`, with `-l` when bash was a login
shell. It does nothing for `bash -c`, non-terminal input or output, an account
whose `SHELL` already ends in `/zsh`, or a set marker. The receiving zsh sources
Ghostty's zsh integration when `GHOSTTY_RESOURCES_DIR` provides it (Ghostty 1.3.1
documents this after `exec zsh`, and the script guards double loading), then sets
the marker to 2 so nested shells skip it.

The first failure test exposed that macOS bash 3.2 exits (status 1, "cannot
execute binary file") when `exec` fails while reading startup files, despite the
manual's interactive-shell rule; that would have closed the terminal. The handoff
now enables `execfail` around the exec only and restores it, `SHELL` and the
marker; `TestZshHandoffFailureKeepsBash` failed before that change and passes now.
Tests: `TestZshHandoffFromInteractiveTerminalBash` drives a real interactive bash
on a pseudo-terminal with a recording zsh fixture (a path with a space) for the
handover, login, already-zsh, marker, `bash -c`, piped-input and unselected cases;
`TestZshHandoffLoadsGhosttyIntegrationOnce` uses real zsh. They pass with bash
3.2 on macOS and bash 5.2 in a disposable Ubuntu 24.04 container. An end-to-end
run there with the real `/usr/bin/zsh` showed `bash -i` and `bash -l -i` ending in
zsh 5.9 with `SHELL=/usr/bin/zsh`, `bash -c` staying bash, and `bash` typed inside
the zsh staying bash. The shared POSIX shell fixture now selects the handoff, so
shell lint parses it with both `bash -n` and `zsh -n`.


## 2026-10-24 — Neovim as the shell and sudoedit editor

PR #91 merged as `a6246ba` after 27/27 checks on its final head `9d1a38a`.

Owner request: editing `/etc/hosts` on the Ubuntu laptop opened the system
default editor. Setup never set an editor. Selected `config.nvim` now adds
`export EDITOR=nvim VISUAL=nvim` to the shared `env.sh` and
`export SUDO_EDITOR=<managed current/bin/nvim>`; Git sets no `core.editor`, so
commit messages follow. Probes in disposable containers with a recording editor
under the user's home:

- Ubuntu 24.04, sudo 1.9.15p5: `EDITOR=<bare name>` on the user's PATH gives
  "command not found" (sudoers `secure_path`); an absolute `SUDO_EDITOR` runs as
  the user with HOME and XDG_CONFIG_HOME intact; a path with a space fails
  unescaped and works backslash-escaped; a file in a root-owned directory under
  `/tmp` is accepted.
- Ubuntu 26.04, sudo-rs 0.2.13: absolute `SUDO_EDITOR` works the same way; a path
  with a space fails escaped or not (falls back to `/usr/bin/vi`); a file below
  any user-writable component, including `/tmp`, is refused ("cannot open a file
  in a path writable by the user").
- macOS 26 ships sudo 1.9.17p2 with `sudo -e` and no `sudoedit` command; not run
  locally (it needs the owner's password), so the hosted macOS core job proves it.

Hence: `SUDO_EDITOR` is omitted when the path contains whitespace or a backslash,
and the native fixture creates its root-owned file under `/etc`.
`TestNeovimIsTheShellEditor` sources the generated `env.sh` in bash and zsh
(selected, unselected keeps personal values, whitespace path); it failed on the
previous generator ("nano|nano|nano"). `TestNativeSudoEditorUsesGeneratedNeovim`
(opt-in `DOTFILES_TEST_SUDO_EDITOR=1`, a new step in the Linux and macOS core
jobs) runs real `sudo -n -e` through the generated environment and requires the
edit written back, root ownership kept, and the editor run with the user's uid and
HOME. It passed in both containers as an ordinary user and failed on the previous
generator. The shared POSIX shell fixture now selects `config.nvim`, so shell lint
and the zsh runners consume the new lines. Windows is unchanged.

The owner also asked whether the earlier local VM rounds are still needed.
Assessment given (no new decision recorded): hosted native jobs already cover
install, update and removal on every target, and the field findings show
real-machine problems (IT AppArmor policy, a directory account, window classes)
that a clean VM would not have shown. Reboots, first-time permission grants and
visual checks stay manual evidence from the owner's own machines.

## 2026-10-24 — APT refresh report and profile blocks after formatting

PR #92 merged as `9ec6a9b` after 27/27 checks on its final head `a9cfa40`.
`TestNativeSudoEditorUsesGeneratedNeovim` passed in core ubuntu-26.04,
ubuntu-26.04-arm and macos-26, the first run against Apple's sudo. The weekly
agent pin workflow, through the owner's GitHub App, opened PR #93, merged as
`b723e97`; Renovate PR #89 merged as `d632e80`.

Field report (Ubuntu laptop): setup's menu resumed an interrupted clangd APT
operation whose saved refresh had no recorded result. The re-dispatched
`apt-get -o APT::Update::Error-Mode=any update` exited 100: the OneDrive client's
openSUSE repository returned `EXPKEYSIG B8AC39B0876D807E`. The key served at
`download.opensuse.org/.../xUbuntu_24.04/Release.key` today is the same key with
its expiry extended to 2028-12-18, so re-importing it fixes the machine. The
report showed APT's output three times: once live (by design) and twice in the
failure report. `LinuxAPTDriver.mutationError` appended `nativeDiagnostic` to a
worker `nativeCommandError` whose message already ended with that same tail.

Probe in a disposable `ubuntu:24.04` container: with verified lists from an
earlier successful update present, a signature failure on that source made plain
`apt-get update` print warnings and exit 0, while `Error-Mode=any` exited 100. On
a first fetch with no earlier lists, both exited 100 ("is not signed"). APT keeps
using only the previously verified lists in the warning case.

Change: the worker's failure message is built by `nativeReply.failure()`, which
the APT test fixture now also uses, so tests see the production error shape.
`mutationError` skips its own tail for a `nativeCommandError` and, for a refresh,
says to fix or disable the source named in the E: lines, then resume.
`TestLinuxAPTFailedRefreshExplainsItsSourceOnceAndResumes` failed on the previous
driver (tail twice, no guidance) and passes; it also proves that resuming after
the source is fixed refreshes again and installs.

Owner decision (same day, on recommendation): the refresh drops `Error-Mode=any`
and runs plain `apt-get update`. A source that fails only with a warning (stale
but verified lists, transient network errors) no longer stops setup; persistent
errors still fail, and an unreachable package still fails at install. `validate`
compares every saved command with the current recipe, so `command` reproduces the
earlier strict arguments only when a saved refresh carries exactly them; new
refresh commands, including later ones in an old operation, are plain.
`TestLinuxAPTRefreshIsPlainAndSavedStrictRefreshStaysValid` rewrites a saved
refresh to the strict recipe with its reply lost (the laptop's first failure):
resume re-runs it unchanged, then installs; other saved arguments are refused
before any command. It failed on the previous recipe ("not plain") and, with
the compatibility branch removed, on the saved strict command ("differs from its
trusted recipe"). The opted-in native APT fixture keeps its own strict refresh as
test setup.

Second field report the same day: after the key fix, every selected tool waited
on `integration.shells: profile block markers were edited; preserve the file`,
with no file named. A read-only listing on the laptop showed `~/.zshrc` with the
start marker on line 1 and a personal `export SUDO_EDITOR=...` after the block;
`.zshenv`, `.zprofile`, `.bashrc` and `.profile` were intact. Setup creates
`.zshrc` with its block, whose leading newline makes the file start with an empty
line. Conform runs shfmt on zsh files when saving, and the managed shfmt v3.14.1
from the Mason lock drops leading blank lines, with or without
`--language-dialect zsh` (reproduced on the Mac with that exact binary). The
parser required a newline before the start marker. The owner restored the line
and setup completed.

Change: `profileBlockBounds` also accepts a start marker at the beginning of the
file after any BOM (never in a saved UTF-16 fragment) and reports the missing
newline, which `replaceProfileBlock` puts back into the returned block bytes.
Fingerprints, ownership checks and removal therefore see the block as written,
and an update or replacement writes the empty line again. Any other text before
a marker is still "edited". The profile driver's observation and apply paths
prefix block errors with the profile path. `TestProfileBlockOpeningTheFileSurvivesFormatterAndLifecycle`
(Check, Update, Remove) failed on the previous parser with needs-action and
passes; `TestProfileBlockOpeningTheFileMayLoseOnlyItsLeadingNewline` covers LF,
CRLF, no BOM, UTF-8 and both UTF-16 BOMs and the refused prefix;
`TestProfileEditedMarkersNameTheirFile` failed before the path prefix. One
accepted edge: a restoration that reinserts a saved original whose own block had
lost its newline writes the block back with it.

## 2026-10-25 — v1.0.0 release preparation

PR #94 merged as `a8190f6` (27/27 checks on its head `d9c1fa4`). The owner asked
whether to release; the recommendation was to first run Update and Check on the
MacBook Air and the Windows laptop, because neither had run this week's changes
and release tags are immutable. The owner chose to release now. The candidate
notes say so under field evidence and do not claim those runs.

`make release-check` reported the v0.4.4 manifest and proof identities OK.
`make release-prepare VERSION=v1.0.0` (notes written outside the checkout and
validated with `validate_notes`/`release_body`) created `release/v1.0.0`, ran
`make ci`, and opened PR #95. A first attempt stopped before any change because
the validation import had left `scripts/__pycache__` in the checkout. A second
commit on the branch replaces pre-release wording: the README "Major-release
candidate" banner, the CLAUDE.md sentence that the public command was not
release-ready, and a v1.0.0 entry in known issues listing the open field items.

## 2026-10-27 — v1.0.0 preparation committed gate bytecode

The owner merged PR #95 as `c9afe16` and ran `make release-publish`. The pasted
command wrapped, so `EXPECTED_SHA` never reached make and the publisher stopped
before any check or write ("EXPECTED_SHA=... is required"). The pull output
showed four `tests/bootstrap/__pycache__/*.pyc` files (Python 3.14 bytecode) in
the merge. `prepare` runs `make ci` in the worktree and then `git add --all`; the
Python test suites write bytecode and `.gitignore` did not cover it. The agent
had described the preparation commit without reading its file list. The closure
path had the same staging, so publication would have repeated it.

Correction (no tag existed): `commit_rendered` stages only the rendered paths
(`docs/releases/{version}.md` and the manifest for preparation; notes, proof and
manifest for closure) and fails on any other status entry, including untracked
files and unstaged changes; `.gitignore` covers `__pycache__/` and `*.pyc`; the
four files are deleted. `CommitRenderedTests` drives a real Git repository with
a bytecode leftover and a modified tracked file; it failed with the previous
`git add --all` staging ("ReleaseError not raised") and passes. The correction
uses the branch name `release/v1.0.0` again, because `verify_preparation_pr`
accepts only the unique merge of that branch as the release commit.
