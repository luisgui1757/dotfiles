---
name: full-stop
description: Take a task through implementation, verification and PR review, then leave it unmerged for the user. Only for an explicit full-stop workflow, never a request to stop working.
---

# Full stop

## Endpoint and authority

Assume the user is not watching the thread. Own the task until the PR is in a state they can merge.
Keep fixing, verifying and reviewing. **Do not merge it yourself.**

On this task's branch, you may commit, push, rebase, resolve conflicts and use `--force-with-lease`.
You may create or update the PR, mark a draft ready for review and keep its description accurate.
Preserve unrelated work and the task's scope.

## How to work

Use [Helix](../helix/SKILL.md) to implement, verify and independently review the change.
Full-stop owns publication, CI and PR feedback. Contact the user when the work needs their
review or a concrete blocker prevents further progress. An existing PR can enter here too;
reuse completed work and review that still cover the current task, behavior and base.

## Procedure

The active user and repository rules govern all permissions. A cancellation means stop; it is never
a trigger to keep delivering. Carry a **ready, unmerged** endpoint into every helper.

1. Read the task and current state. Work on a task branch; preserve unrelated edits. For inherited
   work, use [takeover](references/takeover.md), which calls [modernize](references/modernize.md). Keep
   the intended target base, including an explicitly stacked branch.
2. Run Helix's implementation, verification and selected review procedure. Honor the requested
   reviewer through [host dispatch](../helix/references/dispatch.md). Use
   [Sol-loop](../sol-loop/SKILL.md) only when the user or repository selects that procedure.
   Keep one review loop, including its used budget, through later PR fixes. Do not reset or
   replace a blocked loop to obtain approval. Missing required review blocks publication;
   prepare the PR body locally while completing other permitted work.
3. Update onto the current target branch using the repository's allowed workflow, resolve conflicts
   and reverify. A behavior change, or uncertainty about equivalence, invalidates the earlier review
   and goes back through it.
4. Use [file-pr](references/file-pr.md) to create or update the PR with accurate evidence and
   human contributor credit. Use [attachments](references/attachments.md) when screenshots or
   recordings materially demonstrate the result. Promote a draft when the work is ready for review,
   unless the user or repository requires it to stay a draft.
5. Use [babysit-pr](references/babysit-pr.md) until CI and expected bot reviews finish, real findings
   are handled, and the gate below holds. Verify each fix before pushing. The full-stop request
   authorizes routine replies on this PR as part of handling review feedback, subject to active
   user/repository limits; use [PR replies](references/pr-comments.md). If behavior
   changes, refresh the independent review too.
6. Refresh PR state at handoff. Leave it open and unmerged. Give the user the PR link, exact head,
   verification, review result, remaining judgment calls and the manual merge action. Reach them
   through the host's visible handoff/question mechanism when available.

## Ready-for-user gate

On the current PR head:

- Every behavior change has observed verification; required checks pass.
- The required independent review covers the current behavior diff.
- Expected bots have completed. Their findings have evidence-backed dispositions.
- No human request for changes remains. Repository-required approvals are satisfied for a
  merge-ready claim. Once agent-side work is done, hand off a missing required human approval as
  **ready for human review; approval pending**, instead of waiting indefinitely or claiming merge
  readiness.
- The branch is current with its target, has no conflicts, and the hosting service reports a
  mergeable state. Unknown state is pending, not ready.
- The PR body describes the delivered change and its evidence, without claiming checks that did not
  run.

If repository policy requires a merge queue, report **ready for the user to enqueue** when
pre-enqueue requirements pass. Queue-only integration checks remain unrun and must be named; do not
claim they passed.

## Preserve the no-merge endpoint

- Do not merge, enable auto-merge, enqueue for merging, or call a helper that can do those on your
  behalf. A helper must return control at readiness.
- Check auto-merge/queue state initially and before every push or readiness handoff. If either is
  active, pause publishing branch mutations and surface the conflict; local verification can
  continue. Do not race an automatic merge or change a queue you do not own. Ask for authority to
  disable it, or act on authority already given.
- Closing a PR without merging also requires separate authorization. An obsolete change is a
  decision for the user, not an excuse to close it silently.
- Ask only for an actual blocker or material ambiguity. Routine choices are yours. If required
  evidence, access, review or runtime is unavailable, return a blocked handoff with the exact
  missing action. Silence or no answer never grants permission.
- Keep the host's required progress updates.
