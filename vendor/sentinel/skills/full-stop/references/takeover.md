# Takeover

Internal procedure for continuing an existing branch, PR or task. Return the
prepared change to the caller for review before publication.

## Continue the intent, preserve the credit

Understand what the previous work was trying to achieve. Read the PR, branch or thread, its reviews
and later corrections from the user. Compare the handoff with the actual current code, commits and
uncommitted changes. The handoff is a lead, not proof.

Do not keep an approach merely because the previous agent chose it. Finish the same intended
behavior with the smallest sensible change.

For a replacement PR, carry the intended change onto a fresh branch. Follow the user's requested
commit structure, including a single squash commit. Continue in place or use a stack when asked. Preserve unrelated
work and do not modify the original PR without authorization.

Keep original commit authors and existing co-author trailers. Credit contributors in the PR
description and link the original PR. If squashing, preserve human author/co-author credit in the
squash/merge commit too, using verified identities; never guess an email. Do not add model signatures.

Use [modernize](modernize.md) to align the change with its target base and current
architecture. Keep all intended behavior and surfaces while simplifying; do not add features.
Recheck the source PR before finishing in case it improved while you worked.

Verify the original bug and realistic usage yourself, within repository rules and permissions.
Mocked tests alone do not prove the user experience. Name anything unverified.

Report the inherited state, your changes, verification, attribution and remaining decisions.

## Composition and branch safety

- Inherit the caller's permissions and return to it before publication. The caller owns review
  and its endpoint: ready and unmerged for Full-stop, the guarded merge for Full-send.
  Takeover alone grants no merge or closure authority.
- Finish inheritance, modernization and local verification, then return to the caller before
  publication. Full-stop and full-send both require independent review before publishing. Carry
  existing review evidence, findings and used rounds forward; do not restart the review process.
- Before switching/rebasing, use a clean task worktree or another existing suitable checkout. Never
  stash, discard or absorb unrelated work to make room.
- On a replacement branch, rebase or cherry-pick only the original change's commits onto the
  intended target, retaining authors. Do not import unrelated history or duplicate the same patch
  twice.
- Pass `modernize` the source PR and the actual destination branch. It must not rewrite the source
  contributor's branch while you are building a replacement.
- If the source changed, compare the new delta before incorporating it. Preserve improvements and
  attribution, rerun affected checks, and refresh review for changed behavior.
- A comment, body edit, push or branch rewrite needs the authority of the active task; finding a
  useful action does not expand it.
