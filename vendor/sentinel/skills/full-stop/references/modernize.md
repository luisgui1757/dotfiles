# Modernize

Internal procedure for bringing an existing change onto its current target base
and checking whether the change is still needed.

## Procedure

The task's scope and user/repository permissions control every mutation. This is a focused
continuation of existing work, not an architecture refresh.

1. Identify the intended behavior, source PR/branch, target base and branch you are allowed to
   change. Fetch current remote state. Preserve an explicit non-default or stacked base.
2. Read relevant changes that landed since the source diverged. Check behavior and callers, not just
   similar commit messages. Determine whether the original problem still exists, is partially
   solved, or is fully solved upstream. Reproduce or inspect the relevant flow where practical.
3. If no change remains necessary, return that result with evidence. Close the source PR only if
   this task separately authorizes closure; otherwise leave it open and recommend the disposition.
   Full-send/full-stop callers do not grant that closure permission.
4. If work remains, rebase the authorized task branch onto its current target, or use the
   repository's required update method. Work in a clean isolated checkout. Preserve authorship,
   unrelated work and the source contributor's branch when building a replacement.
5. Resolve conflicts by preserving the intended behavior in the current architecture. Remove
   superseded pieces of this change; do not modernize adjacent code. If a conflict reveals a real
   product decision, ask instead of guessing it away.
6. Run the relevant checks and the original usage/reproduction. Compare the before/after patch;
   ensure no intended behavior was lost or duplicated. Update the task's documentation and
   verification evidence.
7. Return the new base/head, remaining need, meaningful changes and verification before publishing.
   The caller reviews changed behavior or uncertain equivalence under its existing review procedure.
   Preserve findings and used rounds. Reuse earlier approval only when it still covers the change.
   Before an authorized push, the caller rechecks the remote head and uses `--force-with-lease`
   for rewrites. A rejected lease requires reconciliation, never overwriting someone else's work.

If the task is review-only, return this analysis and proposed update without editing, rebasing,
pushing or closing. Modernize does not merge.
