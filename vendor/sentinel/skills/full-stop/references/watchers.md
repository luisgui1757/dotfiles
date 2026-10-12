# PR watcher examples

Use only capabilities actually exposed by the current host.

- **T3 Code:** `watch_pull_request` monitors checks, comments, reviews and
  conflicts, then wakes the parent task. Read its current schema; subagents may
  not register watches. Unregister when handing back or abandoning the PR.
- **Claude Code:** `/loop` or an available Monitor tool can schedule checks while
  the session remains active. They are session tools, not proof of a durable
  hosted watcher. Read the current completion and cancellation behavior.
- **GitHub CLI:** `gh pr checks --watch` waits for checks only. Fetch review
  decisions, inline threads and comments separately.

For another forge, use its supported checks/status endpoint and bounded polling
at a rate appropriate to that service. Track the reviewed head. Do not infer
approval from silence, and do not start an unattended recurring job without
permission. A missing human approval gets a visible handoff once agent work ends.

Sources: [T3 source control](https://github.com/pingdotgg/t3code/blob/main/docs/user/source-control.md),
[Claude scheduled tasks](https://code.claude.com/docs/en/scheduled-tasks),
[GitHub checks](https://cli.github.com/manual/gh_pr_checks).
