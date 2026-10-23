# Babysit PR

Internal procedure for handling CI and PR feedback until the caller's readiness gate holds.

## Watch, judge, fix, verify

Inherit the PR, branch, allowed actions and finish line from the caller. This helper never merges,
enables auto-merge, enqueues, closes a PR, or broadens the task. It returns control at readiness.
Read-only requests stay read-only.

1. Inspect the current PR head/base, status, auto-merge/queue state, required checks, review
   decisions, inline review threads, top-level comments and review summaries. Recheck automatic
   merge state before each push and apply the caller's rule if it is active. Do not treat an empty
   summary as an absence of inline findings. Identify expected bots from repository configuration
   and current activity.
2. Track unresolved findings by stable ID with the head reviewed, disposition and evidence. Read
   each claim against the code and actual behavior. Severity labels are claims, not facts. Fix
   in-scope bugs; reject wrong claims with evidence; record out-of-scope work without growing the
   task.
3. Diagnose failing CI before changing code. Fix failures this task causes. For unrelated
   infrastructure failures, use a permitted retry only when evidence warrants it; repeated unchanged
   failure is a blocker, not an endless rerun loop. Never weaken checks to get green.
4. Verify a fix. Carry the caller's selected review procedure, findings and used rounds forward.
   Complete required independent review of behavior-changing fixes **before pushing**, including
   on an existing PR. A selected [Sol-loop](../../sol-loop/SKILL.md) keeps its budget and agreement rules;
   do not restart it or add a generic review loop. An unavailable reviewer or exhausted budget
   blocks publication. Commit/push only when authorized and that gate is satisfied, then confirm the
   remote head contains the fix. Only then claim it is fixed remotely. Reply through
   [PR replies](pr-comments.md). Do not resolve a human's requested change
   merely because you disagree; retain the blocker until the reviewer or repository process clears
   it.
5. Each new push restarts affected checks/reviews. Refresh the caller's independent review when
   behavior changes. Reconcile newer human or contributor edits rather than overwriting them.

## Waiting without losing the task

- Use an advertised host PR watcher when present. Read its schema, register this PR, and use the
  supported wake/wait mechanism. Use only tools exposed by the current host.
- Keep the task alive while CI and expected bot reviews are pending. Once agent-side work is done,
  return a missing required human approval or an unresolved human decision to the caller for a
  visible handoff; do not wait indefinitely for a human or label the PR merge-ready. Give only
  progress updates the host/user requires. Unregister the watcher before final handoff or abandoning
  the PR.
- Without a PR watcher, use supported waiting tools for checks and a rate-respecting refresh for
  reviews. See [watcher examples](watchers.md). A CI-only watcher does not cover reviews;
  refresh those separately.
- If a known expected bot never starts or finishes, investigate its trigger and status. Do not infer
  approval from silence. Retry only through a supported, authorized path; otherwise report the
  missing review as blocked.
- If execution cannot continue, clean up the watcher and give the current head, pending items and
  exact resumption action. Do not label it ready merely because you must stop.

## Finish

Re-read current head/base, required checks, expected bot completion, findings and human review
state. Check the caller's gate on those same facts. A changed head/base returns to verification; an
unknown status stays pending.

For full-stop, return ready and unmerged, with any queue-only checks explicitly unrun. For
full-send, return ready-to-merge to its parent, which alone evaluates merge authority and performs the
guarded merge. Report a blocker
when access, an unresolved decision or an unavailable required service prevents that boundary.
