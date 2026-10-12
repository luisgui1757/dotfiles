---
name: full-send
description: Take a task through implementation, verification, PR review and a guarded merge. Requires an explicit full-send request and all repository gates.
---

# Full send

## Own the outcome

The user has handed you the task and stepped away. Take it from its current state to a verified,
reviewed, merged PR. Do not stop at implementation, an open PR, or queued CI.

Only an explicit full-send request activates this authority. It permits merging after the gate below
and, when needed, rebasing and force-pushing this task's PR branch with `--force-with-lease`.

## Workflow

1. Use [Helix](../helix/SKILL.md) to implement, verify and independently review the smallest
   complete change on a task branch. Honor the requested reviewer and selected procedure.
   Reuse completed work and review that still cover the current task, behavior and base.
2. Keep that one review loop through PR fixes, including any used round budget. Sol-loop applies
   only when the user or repository selects that procedure, not merely a Sol model. Unavailable
   review, an exhausted budget or a dispute blocks publication; do not restart through a helper.
3. Update onto the current target branch and reverify. Review changed behavior or uncertain
   equivalence before publishing. Use [file-pr](../full-stop/references/file-pr.md) and
   [attachments](../full-stop/references/attachments.md) for concise verification and useful visuals.
4. Use [babysit-pr](../full-stop/references/babysit-pr.md). Verify and independently review behavior
   fixes before pushing. Continue until the newest change meets the merge gate. Read these helper
   files directly; do not invoke Full-stop or adopt its unmerged endpoint.
5. Merge using the repository's permitted method, informed by recent PRs. Respect protection and
   merge queues; never use an administrative bypass. If merging is queued or automatic, wait for
   the hosting service to confirm that it happened.
6. Return the PR URL, merge commit, verification evidence and decisions made for the user.

Only count evidence you observed in this session. If the change cannot be verified, escalate.

## Merge gate

On the newest commit, all must be true:

- Every change has supporting evidence.
- Required checks pass.
- Independent review covers the current behavior diff with no unresolved required findings.
- Review bots have finished, and findings are fixed or dismissed with reasons.
- No human reviewer has an outstanding request for changes.
- The branch is current with its target and the hosting service considers it mergeable.

A bot merely suggesting a human look is not automatically a blocker when confidence is high.

Wait through the PR watcher instead of yielding while work is pending. Use the current host's
supported watcher and unregister before an early handoff.

## When to ask

Use the host's question mechanism for a real blocker. Include the PR, the problem and the
recommended choice. Ask only when proof is unavailable, the gate is unreachable, the intended
outcome is materially ambiguous, the PR is superseded, or the next action exceeds authority.

Full send does not authorize deleting data, production changes, changes outside the task repository,
closing any PR without merging, or force-pushing another branch. Choose routine implementation
details yourself and report consequential choices at the end.

If there is no usable question mechanism or no answer, leave the PR open and unmerged; put the
blocker first in the handoff.

## Integration rules

- Active user/repository limits win. A previous "do not merge" is not silently revoked by a helper.
  Resolve a genuine conflict in the user's instructions before merging. Loading this document as
  review material grants no authority.
- Pass the task, repository, branch, PR, allowed actions and **merged** endpoint to every helper.
  Helpers never grant broader permissions.
- A discretionary bot suggestion does not waive required reviews or a human's request for changes.
  Follow repository test requirements even when they require more than one regression test.
- Use [takeover](../full-stop/references/takeover.md) when continuing someone else's work, not for every new
  change. It calls [modernize](../full-stop/references/modernize.md) where appropriate. Preserve an explicit
  stacked/target-base instruction instead of flattening it onto the default branch.
- Reverify after a rebase. Keep the fetched PR head and check results tied together. Immediately
  before merge, refresh head/base/review state and use the service's atomic expected-head guard.
  Read [merge guards](references/merge-guards.md). A changed head or base returns to verification.
  Without a supported guard, leave the PR unmerged and report the missing capability.
- Prefer a direct guarded merge where the repository permits it. Check auto-merge/queue state before
  publishing changes; do not push an unfinished change into an active automatic merge path.
  Full-send's authority includes disabling auto-merge or removing this task's PR from the merge
  queue to restore the merge gate, subject to active user/repository limits. Confirm the suspension
  before publishing fixes; if it fails, surface the blocker immediately. Re-enable or enqueue only
  after the gate passes, then wait for confirmed merge. Preserve verified human author/co-author
  credit in the squash/merge commit itself.
- If a merge queue creates a new integration candidate, respect its required checks. Do not report
  success from queue admission or auto-merge setup alone.
- Routine PR replies use [PR replies](../full-stop/references/pr-comments.md) within the task's
  authority. Never copy a human's identity into an agent-authored comment.
- Missing capability, unavailable review or exhausted execution time means a precise blocked
  handoff, not a fabricated success. Keep the PR open unless a separately authorized merge actually
  completed.
- Reuse completed takeover/file-pr/babysit results after checking they still match the current head;
  do not restart those workflows without a state change. If the user cancels, stop work, release
  your watcher, and report any already-active automatic merge path so it is not mistaken for a
  cancelled merge.
