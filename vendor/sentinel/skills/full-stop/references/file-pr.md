# File PR

Internal procedure for creating or updating a verified PR with a short, useful description.
Use the task's repository, branch and publication authority.

1. Confirm the repository, hosting service, intended base and head, including
   forks or stacked branches. Read contribution rules and the PR template.
   Inspect the full diff and status; exclude unrelated files and private data.
2. Confirm verification and any required independent review cover the current
   change. Leave rebasing and further development to the calling workflow.
3. Commit and push only when authorized. Preserve human contributor credit;
   follow the requested commit structure. Verify the remote head before filing.
4. Find the existing PR for this source repository, branch and target. Update it
   instead of creating a duplicate. Link a replaced PR and retain its human
   contributors' credit. Close it only with explicit authority.
5. Draft the problem and resulting behavior, then the verification and material
   limitations. Apply [TLDR](../../tldr/SKILL.md), then [Unslop](../../unslop/SKILL.md).
   Prefer a short paragraph and a few useful bullets. Follow a required template
   without padding it. Do not add model signatures, generated-by footers or
   automatic review receipts. Requested review evidence belongs in verification.
   Surface conflicting repository disclosure rules before publishing.
6. Use the service's supported API or CLI with an exact body file or structured
   argument. Preserve existing human content. Set base and head explicitly when
   ambiguous. A verified new change can be ready for review; preserve an existing
   draft until readiness and task authority permit promotion.
7. Read back the URL, base, head SHA, draft state, description and attachments.
   After an ambiguous error, inspect the result and retry only missing work.

Use [attachments](attachments.md) for useful visual evidence.
Return to the caller, which may continue with [babysit-pr](babysit-pr.md).
Filing never grants merge authority or permission to change repository protection.
