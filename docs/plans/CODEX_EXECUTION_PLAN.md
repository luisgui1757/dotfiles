# Codex 5.5 xhigh — Master Execution Plan

_The single entry point for executing the current dotfiles work. It sequences **four** work-streams and
points at the two detailed specs. Read this first, then execute the specs it references in the order
below._

2026-06-09 current-branch status: this was the pre-implementation sequencing
plan. On `chezmoi-pilot`, W1-W4 have landed and W4 grew from a pilot into the
full config-layer migration. Treat the ordering notes below as execution
history; use `docs/MIGRATION_STATUS.md` for current owner-facing migration
status and sign-off caveats.

| Spec | Covers | Status |
|------|--------|--------|
| **this file** | objective, sequencing, dependencies, commit strategy, pre-flight | historical orchestration |
| `docs/plans/WINDOWS_FIXES_SPEC.md` | **W1** WT Rose Pine, **W2** pwsh keep-latest, **W3** psmux auth | landed; historical spec |
| `docs/plans/CHEZMOI_WAVE_A_SPEC.md` | **W4** chezmoi migration (DC-1…DC-6) | landed and expanded to full config layer |

Grounded against `luisgui1757/dotfiles` HEAD **`96d85ee`** (the same commit the chezmoi spec was grounded
against). All `file:line` anchors in the referenced specs are at that HEAD.

---

## Objective & the two streams

There are two distinct streams, and **they are not independent at the seam**:

- **Stream A — legacy-path hardening (W1–W3).** Three small, owner-found correctness fixes to the
  existing `setup.ps1` / `bootstrap.ps1` / `install-deps.ps1` install path. Each is independently
  shippable with its own tests.
- **Stream B — the chezmoi Wave A pilot (W4).** The migration pilot. Its keystone is the **DC-6 parity
  gate**, which proves the new chezmoi path reproduces the *old* path's behavior before anything is
  retired (retirement is Wave C). The pilot deliberately keeps the old scripts running beside chezmoi
  (`CHEZMOI_WAVE_A_SPEC.md` Resolved decision #2).

The seam: **W1 and W3 fix behavior that the chezmoi pilot directly ports** (the WT merge in DC-3 Step 2,
the psmux installer in DC-3 Step 1). If Stream A changes the old path but Stream B's spec is not updated
in lockstep, the parity gate either fails or — worse, for psmux, whose buggy line is *already copied into
the spec* — "passes" by matching buggy behavior on both sides, **blessing the bug**. So the streams must
be sequenced, not run blind.

---

## The keystone sequencing decision (argued)

**Decision: land W1–W3 in the legacy path FIRST, and in the *same commit* update the corresponding
`CHEZMOI_WAVE_A_SPEC.md` delta. Execute W4 (the migration) LAST, against the already-corrected spec.**

Why this order and not the reverse:

1. **The parity gate is an oracle, not a test.** DC-6 compares OLD-path output vs NEW-path output on the
   manifest-scoped migrated config set and asserts equality (`CHEZMOI_WAVE_A_SPEC.md` DC-6 Step 3). Its value is entirely
   that both sides are *correct*. Fixing only one side breaks the equality; fixing neither (status quo)
   leaves both buggy; fixing the old side while the spec still encodes the old bug means the future
   chezmoi port reproduces the bug and the gate green-lights it. The only sound state is **both sides
   carry the fix** — which is achieved by editing the legacy code and the spec delta together.

2. **Historical note:** at planning time, Stream B's `home/` source tree did
   **not exist yet** and `CHEZMOI_WAVE_A_SPEC.md` was markdown describing what
   Codex would build. On `chezmoi-pilot`, `home/` exists and the migration gate
   is checked in; future changes must update docs, source copies/templates, and
   parity manifest assertions together.

3. **Stream A is low-risk and unblocks confidence.** W1–W3 are bounded, testable on the existing CI, and
   independently valuable regardless of whether the chezmoi migration ever ships. They should not wait
   behind a multi-day migration.

4. **W2 is the soft case.** pwsh is not *ported* by the pilot (its install is Wave B, Appendix A). W2's
   only Stream-B touch is a Step-0 prerequisite note. So W2 can land any time before W4 runs on Windows;
   it does not gate the migration mechanics. (See "pwsh pull-forward" below.)

**Concretely:** each of W1, W2, W3 is a legacy PR whose single commit edits *both* the `*.ps1`/test/docs
**and** the matching lines in `CHEZMOI_WAVE_A_SPEC.md`. Then W4 executes the chezmoi spec, now carrying
all three deltas, so DC-6 is meaningful from its first green run.

---

## Ordering table

| # | Task | Depends on | Lands in | Branch / PR | CI gate | Parallel-safe? |
|---|------|------------|----------|-------------|---------|----------------|
| **W1** | WT Rose Pine → default-on merge | — | `setup.ps1`, `bootstrap.ps1`, `tests/bootstrap/ps1_test.ps1`, `CLAUDE.md` (inv 15), `README.md`, `windows-terminal/README.md`, `PLAN.md`, `tests/MANUAL.md` **+** `CHEZMOI_WAVE_A_SPEC.md` (DC-3 Step 2, DC-6 Step 4) | `fix/wt-rosepine-default` | `.\test.ps1` (windows-2025) | ✅ (touches setup/bootstrap, **not** `install-deps.ps1`) |
| **W3** | psmux auth hardening (`Add-ScoopBucketSafe`) | — | `install-deps.ps1`, `tests/static/repo_policy_test.sh`, `tests/powershell/InstallDeps.Tests.ps1`, `CLAUDE.md` **+** `CHEZMOI_WAVE_A_SPEC.md` (DC-3 Step 1) | `fix/psmux-bucket-auth` | static (`repo_policy_test.sh`, `ps1_parse.sh`) **+** `.\test.ps1` | ⚠️ shares `install-deps.ps1` with W2 |
| **W2** | pwsh keep-latest (`Update-ScoopTool`) + catalog guard | — | `install-deps.ps1`, `tests/powershell/InstallDeps.Tests.ps1`, `PLAN.md`, `CLAUDE.md` **+** `CHEZMOI_WAVE_A_SPEC.md` (Step 0) | `fix/pwsh-keep-latest` | `.\test.ps1` | ⚠️ shares `install-deps.ps1` with W3 |
| **W4** | chezmoi migration (DC-1…DC-6) | **W1 + W3 spec deltas merged**; pwsh present (W2 runtime) | `home/` tree, `tests/migration/`, `chezmoi-parity*` CI jobs (+ required-check sync files) | `chezmoi-pilot` | `chezmoi-parity`, `chezmoi-parity-macos`, `chezmoi-parity-windows` + manual psmux real apply | executed last |

**W2/W3 collision:** both edit `install-deps.ps1`. Do **W3 then W2** (W3 inserts `Add-ScoopBucketSafe`
near the top + edits `Install-Psmux`; W2 inserts `Update-ScoopTool` after `Install-One` + adds a call at
`:467` — re-resolve W2's anchors against the post-W3 buffer), **or** combine them in one PR. W1 is fully
independent and can land in parallel.

---

## Per-stream pointers (do not restate the specs)

- **W1 — WT Rose Pine.** Root cause: the merge is opt-in (`setup.ps1:26,122-124`); `-All` never themes
  WT. Fix flips it to default-on with `-SkipWindowsTerminalMerge` opt-out, keeping `-MergeWindowsTerminal`
  as a no-op alias. Five paste-ready edits, the regression test, the invariant-15 amendment, and the
  DC-3/DC-6 deltas are in `WINDOWS_FIXES_SPEC.md#w1`.
- **W2 — pwsh keep-latest.** pwsh is *already* scoop-first (`install-deps.ps1:123,467`); the gap is
  "keep latest" (no `scoop update`) + a missing catalog guard. Implement the consent-gated single-tool
  `Update-ScoopTool pwsh` (the owner asked for "keep latest" explicitly) and the catalog Pester guard.
  Full detail in `WINDOWS_FIXES_SPEC.md#w2`.
- **W3 — psmux auth.** Root cause: `scoop bucket add psmux <github>` git-clones with interactive prompts
  enabled and exit-code blind (`install-deps.ps1:377`). Fix: one `Add-ScoopBucketSafe` helper (idempotent,
  `GIT_TERMINAL_PROMPT=0`/`GCM_INTERACTIVE=0`, populated-bucket verification, never-throws) applied to all
  bare bucket-adds, plus the cross-platform `repo_policy_test.sh` guard, plus the DC-3 Step-1 port. Full
  detail in `WINDOWS_FIXES_SPEC.md#w3`.
- **W4 — chezmoi Wave A.** Execute `CHEZMOI_WAVE_A_SPEC.md` top-to-bottom from its Step 0 sandbox
  contract through the DC-6 parity gate. That spec is self-contained and already adversarially reviewed;
  this plan only adds the three deltas below and the "run it last" ordering.

---

## chezmoi-spec deltas rollup

Each Stream-A PR must apply its delta to `docs/plans/CHEZMOI_WAVE_A_SPEC.md` **in the same commit** as the
code fix.

| From | `CHEZMOI_WAVE_A_SPEC.md` section | One-line delta | Why same-commit |
|------|----------------------------------|----------------|-----------------|
| **W1** | DC-3 Step 2 preamble | Note the **legacy** `bootstrap.ps1` merge is now also default-on, so the chezmoi `modify_` default-on behavior is **parity**, not divergence. | Keeps the rationale honest; no behavior change. |
| **W1** | DC-6 Step 4 (WT parity probe) | OLD-side command becomes `bootstrap.ps1` with **no switch** (`-MergeWindowsTerminal` now a back-compat alias). | Otherwise the parity harness invokes a no-longer-needed switch / wrong default. |
| **W2** | Step 0 (Prerequisites) | Add a bullet: `pwsh` must be on PATH (legacy `install-deps.ps1:123,467`) before any `.ps1.tmpl` runs, since the default interpreter is `pwsh -NoLogo -File`. | The pilot's DC-3 `.ps1` scripts assume pwsh exists. |
| **W3** | DC-3 Step 1 (psmux run-script) | Port `Add-ScoopBucketSafe`; replace the bare `scoop bucket add psmux … 2>$null`. | The spec already copied the buggy line under `$ErrorActionPreference='Stop'`; parity would bless the bug. |

### Does pwsh-via-scoop need pulling forward into the pilot? — **No.**

The pilot's default `.ps1` interpreter is `pwsh -NoLogo -File` (Verified mechanics §9), and DC-3 runs two
`.ps1.tmpl` scripts. But:

- Those scripts are `{{- if eq .targetOS "windows" -}}`-gated and render to whitespace off-Windows (the
  empty-script idiom, §8), so **pwsh is never invoked on Linux/macOS** — the Ubuntu parity arm never
  touches it.
- On Windows, pwsh must merely be **present**, which the legacy `install-deps.ps1` already guarantees and
  Wave A keeps that installer running (Resolved decision #2).

So the only spec change is W2's **Step-0 prerequisite note**. The full `$Catalog → .chezmoidata` pwsh
install row stays **Wave B** (Appendix A). **Do not port a pwsh install run-script into Wave A.**

---

## Commit / PR strategy

- **Small commits, each leaving its tests green.** Do not ship a mixed diff. Mirror the WSL-hardening
  plan's discipline (`PLAN.md` Commit Strategy).
- **W1 = one PR** (independent files; parallel-safe).
- **W3 then W2** = two sequential PRs on `install-deps.ps1`, or one combined PR. If sequential, rebase W2
  on W3 and re-resolve W2's `:467`/`:229` anchors.
- **Each Stream-A commit edits both the code AND its `CHEZMOI_WAVE_A_SPEC.md` delta** (rollup table
  above) — never split the code fix from its spec delta.
- **W4 = the migration** (historical). It landed after W1+W3 and now includes
  the `home/` tree, migration tests, and `chezmoi-parity*` CI jobs. Required
  check names are checked into `.github/settings.yml`,
  `scripts/apply-repo-safeguards.sh`, and
  `.github/rulesets/main-integrity.json`; making them live remains the owner
  `scripts/apply-repo-safeguards.sh` action.
- **Branch protection reality (CLAUDE.md / `docs/security/branch-protection.md`):** `main` is owner-only,
  squash-only, with non-bypassable required checks. Routine agent work uses least-privilege credentials;
  do not assume admin/`delete_repo`. Each PR needs the owner's review bypass to merge.

---

## Pre-flight for Codex (do this before touching code)

1. **Branch off `main`** for each work-stream (this planning doc lives on `docs/codex-execution-plan`;
   implementation branches are separate — see the ordering table). Confirm you are at or rebased onto
   `96d85ee` (or re-ground if `main` has advanced — see rule 4).
2. **Baseline the tests** before any edit:
   - POSIX dev host / CI: `make test` (and `make test-static`, `make lint`).
   - Windows host: `.\test.ps1` (PSScriptAnalyzer + Pester + nvim plenary).
   Record the green baseline; if anything is already red, surface it before proceeding.
3. **Apply edits per spec**, re-running the relevant suite after each file. Add the regression test in
   the **same** diff as the behavior change (CLAUDE.md "When you're about to make a change").
4. **STOP-and-surface-drift rule (non-negotiable).** Every `file:line` in these specs is pinned to HEAD
   `96d85ee`, and line numbers shift as you edit. Before applying each edit, re-read the cited anchor in
   the current buffer and confirm it matches the quoted text. If it does not — or if a referenced
   function/test/section is absent — **STOP and report the drift**; do not guess a replacement location.
5. **For W4 only:** obey `CHEZMOI_WAVE_A_SPEC.md` **Step 0** — install pinned chezmoi (≥ v2.52.0) and use
   a real `HOME=$SANDBOX` sandbox (never `--destination` alone). Never fabricate a pin/SHA/commit; where a
   value is obtained at runtime, run the command the spec gives.
6. **Documentation discipline (global standing order):** every code/behavior/invariant change updates the
   relevant markdown in the same change — CLAUDE.md invariants, README, PLAN.md, `tests/MANUAL.md`, and
   the `CHEZMOI_WAVE_A_SPEC.md` delta. End each change with the one-line-per-file breakdown table.

---

## Open items & risks (carry forward, do not silently drop)

- **W1 first-launch gap (accepted for now).** The merge requires an existing `settings.json`; on a
  freshly-installed-but-never-launched WT, default-on correctly warns-and-skips (no fabrication). A future
  enhancement could seed a minimal `settings.json` so first run is themed — **out of scope here**, logged
  so it isn't mistaken for a bug.
- **W2 b1-vs-b2 owner call.** Default is **b2** (`Update-ScoopTool pwsh`), faithful to the explicit "keep
  latest" ask. If the owner prefers strict parity with every other catalog tool (install-if-missing only),
  fall back to **b1** and record the decision in `PLAN.md`. Either way, never introduce `scoop update *`.
- **Windows-only CI reach.** W1's live merge, W2's catalog guard, and W3's bucket-path Pester cases run
  **only** on the `windows-2025` lane via `.\test.ps1`. The real psmux clone/credential behavior and the
  live WT merge have **no** automated coverage on any lane — they are gated by the manual checks in each
  spec section. Do not over-claim e2e coverage.
- **W4 is the largest stream** and is gated behind W1+W3. Treat its DC-6 decision gate (N=10 green Ubuntu
  parity runs + 1 manual Windows + 1 manual macOS) as the authorization bar for *Wave C* retirement — not
  part of Wave A acceptance.
