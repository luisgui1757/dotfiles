# Guard the reviewed commit when merging

Identify the hosting service from the verified PR and authenticated tooling.
Read its current merge API or CLI documentation. Require an atomic condition
that rejects the merge if the source head differs from the reviewed commit.
Also refresh the target, approvals, checks and queue state immediately beforehand.

Examples to verify against installed tools and current service documentation:

- GitHub: `gh pr merge --match-head-commit`.
- GitLab: the merge request API's `sha` precondition.
- Azure Repos: the completion request's `lastMergeSourceCommit`.
- CodeCommit: the merge operation's `sourceCommitId` or CLI `--source-commit-id`.

These are service-specific guards, not interchangeable commands. Use the
repository's permitted merge method and expected target where supported.
For Bitbucket or another service, inspect its actual capabilities rather than
assuming a guard exists. If no suitable guard is available, leave the change
unmerged and hand back that limitation. A post-merge comparison cannot undo a race.

Respect merge queues and their integration checks. Verify the resulting merged
state and commit; queue admission or enabling auto-merge is not completion.
