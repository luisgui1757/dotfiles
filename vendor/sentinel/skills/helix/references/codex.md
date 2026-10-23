# Codex dispatch

Check `codex --version`, `codex exec --help` and the current native model catalog.
Use the requested literal model ID and a supported reasoning level. Native
subagents inherit the launching session's permissions; a reviewer label alone
does not make them read-only.

A separate read-only CLI session can review a prepared brief on stdin:

```sh
codex exec --cd "$repo" --sandbox read-only --model "$review_model" \
  -c "model_reasoning_effort=\"$effort\"" --json --ephemeral - < "$brief"
```

Here the repository, brief, model and effort are already verified values. Supply
repository rules and the complete diff in the brief. Inspect local configuration
and extensions before running; the sandbox flag does not restrict an external
MCP server. Where supported, use `--ignore-user-config` and disable unneeded
extensions for an isolated review, while explicitly supplying required rules.
Do not copy authentication files or weaken execution policy.

Read the session's actual model, effort and sandbox metadata. If a requested
field is not exposed, report that limit. Check process completion and the final
review result. Use the host's cancellation API or terminate the CLI process and
confirm it stopped before replacing it.

Sources: [non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode)
and [native roles](../../setup-helix/references/codex.md).
