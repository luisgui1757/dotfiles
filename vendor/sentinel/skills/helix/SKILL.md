---
name: helix
description: Implement a scoped repository change, verify it and resolve independent review findings. Use for feature work and bug fixes that should finish reviewed.
---

# Helix

Make the smallest complete change, prove the requested behavior and resolve
independent review findings. The task controls scope and publication authority.
The host enforces permissions; these instructions do not.

## Choose the route

Use [host dispatch](references/dispatch.md) for Codex, Claude Code, GitHub Copilot
CLI or Kiro. Prefer native delegation when it can honor the requested settings;
use a supported API, SDK or CLI for another requested provider. Inside T3, use
its advertised orchestration and [T3 reference](references/t3.md).

Honor explicit writer, reviewer, model and effort choices. Keep configured
preferences for unspecified choices. Do not silently substitute a model, edit
settings or install an orchestration runtime. An unavailable requested writer
blocks substitute implementation; continue only work independent of that choice.
Check execution records for actual settings and label unexposed values unverified.

## Implement and verify

1. Read repository instructions, the task, relevant code and current changes.
   Revalidate known findings. Preserve unrelated work.
2. State the outcome, allowed changes, a short plan and observable acceptance
   criteria. Inspect or try a small in-scope experiment to resolve discoverable
   questions. Ask only about consequential unknowns that affect the next step.
3. Keep one writer. Normally implement, test and document yourself. If the user
   requests another writer, delegate through the supported route and avoid
   concurrent edits. Capture consequential decisions in existing project docs;
   routine choices do not need a decision file.
4. For a substantive bug, reproduce the failure and verify the same path after
   the fix. Add useful behavioral regression coverage. Follow repository rules.
5. Run relevant checks and the required gate. Exercise the installed entrypoint
   or user journey where practical. Preserve actual commands, results and gaps
   for review. Inspect the full delta, including new files and changes from tools.

Keep scratch and evidence inside the task's allowed write scope. Give delegates
that boundary too. With read-only authority, supply evidence inline. Do not create
review diaries unless required. Clean your scratch after preserving needed evidence.
A missing device or service blocks its dependent claim, not unrelated useful work.

## Choose one review procedure

Use the independent review and correction steps below by default. If the user
or repository explicitly selects [Sol-loop](../sol-loop/SKILL.md), use its
review, severity, agreement and round limits instead. Choosing a Sol model
alone does not select that procedure. Do not run both loops for the same work.

Carry the selected procedure, findings, review evidence and rounds used through
every caller and helper. Reuse a completed review when it still covers the task,
current behavior and base. Changed behavior or uncertain equivalence needs review
under that same procedure. An exhausted budget or unresolved dispute blocks
publication; only the user can authorize a new budget, not another helper call.

## Independent review (default)

After available verification, start one fresh reviewer without the writer's
conversation or private reasoning. Freeze the scoped files while it runs.
Apply available read-only controls and report the access actually exposed.
A role name or copied checkout does not prove isolation.

Give it the task, acceptance criteria, repository rules, fixed baseline and full
current delta, including new files. Supply readable diffs and captured check
output when it cannot run commands. Include failures and gaps, plus before/after
proof for bug fixes. Ask it to inspect the evidence rather than accept a summary.

Include this request:

> Review the change against the task and repository rules. Inspect surrounding
> code, correctness, regressions, scope, tests and docs. Do not edit or delegate.
> Give each finding's severity, location, wrong behavior, evidence and concrete
> fix. Check alternative explanations and other occurrences. Flag unnecessary
> work without removing required behavior. Separate defects from uncertainty.
> Inspect the full delta and verification output. Missing evidence the writer
> can supply remains a finding; request it while continuing useful source review.
> Keep unavailable checks separate from findings. Never waive a failure
> or treat an admission of missing proof as proof. Return findings and gaps, or
> say none were found.

Wait for an actual result. Supply missing evidence that can be captured or
reconstructed and obtain its review. Timeouts, failed launches and self-review
never satisfy independent review.

## Correct (default)

Judge findings against evidence. Fix valid defects and explain rejected claims
briefly in the conversation or PR. Rerun affected checks and required gates, then
have the reviewer examine corrections. Use a fresh T3 task each round; elsewhere
resume the reviewer when supported or send a fresh complete brief with prior
findings and the revised evidence. Keep one reviewer active at a time.

Continue until findings are resolved or a concrete blocker prevents progress.
Do not stop because another available review is needed. Earlier approval does not
cover later behavior changes. After repeated failed fixes, reconsider the cause.

## Finish

Hand back the outcome, relevant changes, verification, review and remaining limits.
Include requested and observed settings when specified, plus reviewer write access.
Use the repository's reporting format and keep the prose short.

- **Complete:** acceptance criteria met, required checks passed and independent
  review covers the final behavior with no unresolved findings.
- **Ready for independent review:** verification passed, but no separate reviewer
  returned a result. Supply its brief and requested settings.
- **Blocked:** no useful authorized work remains. Name the missing evidence,
  access or decision and the claim it prevents. Missing checks never become passes.

Commit, push, PR, merge and deployment happen only when the task authorizes them.
Helix alone does not grant publication or merge authority.
