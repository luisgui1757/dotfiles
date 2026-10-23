---
name: sol-loop
description: Run a bounded independent review and correction loop with a Sol model. Use when the user or calling workflow requests sol-loop; hold publication until agreement.
---

# Sol loop

## Purpose and budget

Have Sol review the change, usually before a PR and sometimes afterwards. Make the smallest changes
that address valid findings and satisfy the intended behavior. Avoid scope growth and push back on
findings you believe are wrong.

- Three rounds by default. First round: the whole change. Later rounds: new code and unresolved
  findings.
- Extend to five only when the user asks, or an evidence-backed blocker/should remains in security,
  permissions, auth, data loss, or state/concurrency.
- Stop early when only nits remain. Declined nits or comments/docs/formatting edits do not buy
  another round.
- One reviewer. The reviewer may not delegate.
- Keep stable finding IDs, severity, disposition and evidence. Record usage only when reported.

When called by another workflow, this is its review loop, not an extra review pass.
Carry the used rounds and findings back to the caller. Re-entry does not reset the
budget or clear a dispute; an exhausted loop remains blocked until the user intervenes.
If earlier rounds or open findings cannot be established from the conversation or PR,
report that gap. Do not assume approval or a fresh budget.

## Establish the review

1. Inspect `git status --short`. Preserve unrelated work. Isolate the task's paths, including
   untracked files; ask if the scope cannot be separated.
2. Fetch the remote. For a PR, inspect its base and head; switch branches only with a clean
   worktree. HEAD must equal the PR head or be ahead only by this task's changes. Use the merge-base
   with the PR's target branch. For local or stacked work, use the intended target, defaulting to
   the remote default only when none is specified. Without a remote, use a user-specified base; HEAD is suitable only for wholly
   uncommitted work.
3. Freeze that base and one diff definition: base to working tree, restricted to the task's paths,
   including new files. State intent and exclusions briefly.
4. Run the relevant checks first. Fix failures caused by this change; disclose pre-existing failures
   and checks you cannot run.
5. Discover the available Sol models; use the latest unless the user specifies one. Record actual
   provider/model/effort when the tools expose them. Do not substitute a different model.

Use [host dispatch](../helix/references/dispatch.md) to reach the requested Sol model.
Inside T3, use its live capabilities and a new review task each round. A child thread ID is not
a continuation destination. Outside T3, use native delegation or a supported Codex CLI/API route.
A role named `sol` is suitable only if execution actually targets the requested model.

While review runs, freeze the scoped files. Wait for completion; do not poll for progress or start a
second reviewer. Use the host's documented wait and cancellation routes. Silence, an error and a timeout are not approval. On failure, cancel, confirm it
stopped, then restart with the findings.

## Reviewer brief

Send a self-contained brief with repository, fixed base, scoped paths, intent, exclusions, check
results and the exact diff inspection command. Tell the reviewer:

- Review only. No edits, commits, pushes, PR comments or subagents.
- Read surrounding code and callers. Find bugs and failures against the intended behavior. Flag
  unnecessary code, tests and guards. Skip style unless it hides a bug.
- Blocker: the intended scenario is wrong. Should: a real defect that callers encounter within
  scope. Nit: everything else. Out-of-scope paths, minor performance costs and extra compatibility
  are nits. Apply the supported-behavior qualification below to uncommon cases.
- Return APPROVE or CHANGES, followed by stable ID, severity, file:line, problem and proposed fix
  for each finding.

## Respond and repeat

Judge each claim and severity yourself. Another path, a rare case or a small cost is a nit
regardless of the reviewer's label, subject to the supported-behavior qualification below. Make the
smallest sufficient fix. Prefer narrowing or removing code to adding machinery. Add one focused
regression test that fails without the fix where warranted. Reject wrong findings with evidence;
decline out-of-scope work and nits with a short reason and record them as follow-up.

Update the findings in the conversation or PR, rerun checks and inspect diff size. If review inflated the change, look for a
smaller solution before continuing. Do not concede a wrong finding to get approval.

Another round requires budget remaining and an open blocker/should, changed behavior, or an approval
that no longer covers the behavior diff. Send the original brief, the findings with evidence,
the same-base full diff and the delta since the prior round. Ask the reviewer to resolve or retain
each prior ID with a reason. Use a fresh T3 review task, or continue the saved native reviewer.

Agreement means approval of the current behavior diff, no open blocker/should, and no unresolved
concern of your own. After a rebase, compare the diff again; rerun review if behavior changed or
equivalence is uncertain. Keep the base SHA and diff identity with the review evidence. A dispute that survives two exchanges, or exhausted rounds, goes to the user
with both positions. Neither is approval.

Hold pushes until agreement. Push afterwards only if the task authorizes it. Report the result
briefly to the caller. Include review evidence in a PR only when requested or required; never add
a model signature or automatic receipt. Effort appears as verified only when execution exposes it.

Report rounds and actual model, what the review changed, diff size before/after, fixed and declined
findings with reasons, reported usage or "not exposed," and whether anything was pushed.

## Host behavior and review evidence

- The active user and repository rules control authority. Reviewing a PR does not itself authorize
  edits, pushes or comments. In review-only mode, return findings and suggested changes without
  applying them.
- Use current tool schemas. On T3, make one capability probe through the host's supported
  direct/discovery route, even if delegation tools are not yet listed. If the host exposes no
  permitted route to that probe, or it finds no suitable Sol route, report the blocker. Tool names
  in this document do not authorize inventing a callable tool or bypassing host restrictions.
- A supported input or documented invariant is within scope even when uncommon. Never downgrade an
  auth, data-loss or other real intended-behavior failure just because it is rare. Include this
  qualification in the reviewer brief. Repository severity and test requirements take precedence
  over the simplified labels and single-test preference above.
- Reduce unnecessary diff growth, not a necessary correction. Keep fixes that the task or repository
  requires.
- When review evidence is requested, include the actual rounds, model, base and verdict in the
  verification section. For an unfinished loop, report the actual outcome; never imply approval.
- In review-only mode, inspect an existing suitable checkout or the PR's committed diff without
  changing the user's checkout. Do not switch branches or run mutating checks when the user forbids
  file changes. One round normally suffices; use another only to resolve a specific open finding
  within the budget.
- A failed launch is not a completed review round, but repeated identical execution failure is a
  blocker. Retry once after confirmed cancellation and a concrete recovery step; do not restart
  indefinitely. A user cancellation stops the loop; cancel an active reviewer through the supported
  route and report any unconfirmed termination.

This procedure requires Git, the task's checks and a working Sol delegation route. The shared
host reference describes supported routes; it does not run the Helix delivery workflow.
