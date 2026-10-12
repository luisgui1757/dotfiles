# Kiro dispatch

Check `kiro-cli --version`, `kiro-cli chat --help` and its native model catalog.
Versions differ in tool names, agent schemas and sandbox behavior. Inspect the
selected agent's resources and permissions before using it for review.

```sh
kiro-cli chat --no-interactive --agent "$review_agent" \
  --model "$review_model" --effort "$effort" --sandbox < "$brief"
```

The named reviewer must already have an appropriate tool and path allowlist.
Pass a prompt through stdin or as an argument, not both. Use `--sandbox` only
where the installed version supports it; an explicit sandbox request must fail
if protection cannot activate. A saved sandbox preference may have different
fallback behavior. Do not quietly continue without requested protection.

In current V3, `--trust-tools=read_file,grep_search` preapproves those tools;
it does not remove other tools or create a sandbox. Restrict the agent's actual
available tools and permissions first. Never replace this with trust-all.

Custom agents need explicit resources for repository instructions, steering and
skills. Include relevant rules in the brief and configure `file://` and
`skill://` resources using the current schema. Do not assume inheritance.
Inspect the execution result for actual model, effort and completion; report
unexposed fields as unverified. Confirm cancellation before starting a replacement.

Sources: [headless mode](https://kiro.dev/docs/cli/headless/),
[skills](https://kiro.dev/docs/skills/) and [steering](https://kiro.dev/docs/steering/).
