# GitHub Copilot CLI dispatch

Check `copilot --version`, `copilot help` and the native model picker. Confirm
model IDs, supported effort levels and current tool names before dispatch.
Use a configured reviewer or the programmatic CLI with a self-contained brief:

```sh
copilot --model "$review_model" --effort "$effort" \
  --available-tools view,glob,grep --allow-tool read \
  --no-ask-user --output-format json < "$brief"
```

Confirm these flags in the installed version. `--available-tools` limits what
the model can call; `--allow-tool` only preapproves calls. Do not use broad
allow-all or share-gist options. Do not opt into project hooks, plugins or MCP
servers without inspecting them. Keep read paths within the task.

For custom subagents, request `modelPolicy: required` when the schema supports it
so a preferred model cannot silently fall back. Include repository instructions
in the brief or configure `include-custom-instructions: true`; custom subagents
may otherwise omit them. Main-agent and subagent behavior differ.

Keep execution metadata that identifies the actual model and result. Do not
use silent output if it removes the only model evidence. Treat unavailable
metadata as unverified and stop a failed requested route without substituting.

Sources: [programmatic CLI](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-programmatic-reference),
[command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference),
and [custom agents](https://docs.github.com/en/copilot/reference/custom-agents-configuration).
