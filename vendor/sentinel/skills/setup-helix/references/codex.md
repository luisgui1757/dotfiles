# Codex settings

## Discover choices

Use an account-context model catalog exposed by the running host, not model
names mentioned in instructions or a delegation tool's parameter schema. Otherwise, with
Node.js 22.19 or newer already installed, run the bundled helper from the
project being configured (resolve its path from this skill's location):

```sh
node <skill-directory>/scripts/discover-models.mjs codex
```

The helper uses that directory's saved native configuration and environment.
Parent-session command-line overrides and profiles do not carry over. If those
change provider or model availability, use the active host's metadata or the
manual-choice path; do not present a different configuration's catalog as current.

It initializes the installed `codex app-server`, reads every `model/list`
page with hidden models excluded, then closes the process. It does not start
a thread, run inference, read credentials, or save configuration. Use the
returned `id`, `name` and per-model `efforts` for the menu. An explicit spawn
parameter's model list does not establish restrictions on saved subagent defaults.
Show all catalog entries; label a restriction only with evidence that it applies
to the setting being saved, and name its source. These are host-listed
choices, not proof of account access or successful child execution.

The helper has a timeout and fails visibly on missing or invalid metadata.
Codex's sandbox can prevent its nested CLI from starting. If the host offers
command approval, request it for this exact metadata command; explain that
the CLI needs its normal startup access. Do not change sandbox settings or
grant persistent permission rules. If approval is unavailable or denied, use
the skill's manual-choice path without another attempt. Do not use a stored catalog or
a list from another host. A `null` effort list means capabilities were not
reported; keep effort inherited or check native documentation before saving
an explicit level. The protocol was exercised on CLI 0.160.0; other versions
must return valid metadata before discovery can be claimed.

## Save settings

Use native project configuration in `.codex/config.toml`, or user configuration
in `$CODEX_HOME/config.toml` (`~/.codex/config.toml` by default) when that scope
was requested. Project configuration requires the host to trust the project.
Do not grant trust or weaken permissions as a side effect of choosing models.
An untrusted project can contain valid settings that the host ignores. In CLI
0.160.0, a command-line `projects.<path>.trust_level` override did not enable
the project layer. Use the host's normal trust flow and verify loading in a
fresh session; do not treat the presence of the file as proof.

The verified configuration path uses these native keys:

```toml
[agents]
default_subagent_model = "<requested reviewer model>"
default_subagent_reasoning_effort = "<requested reviewer effort>"
```

These are defaults for **all native subagents in that scope**, not just Helix.
Explain this and get that scope choice if the request has not already made it.
Merge the requested keys into an existing `[agents]` table; preserve other keys
and tables. To return to inherited defaults, remove only the overrides the
user asked to clear and check for higher or lower configuration layers.

An explicit spawn value overrides these defaults. A selected model without
either an explicit or configured effort uses that model's default effort.
A custom agent file can override both.
Inspect relevant competing settings before claiming a saved default will win.
Do not remove unrelated roles to make it win.

Codex subagents inherit the parent turn's sandbox and approval settings.
Custom agent files cannot override `sandbox_mode` or `approval_policy`; their
instructions and feature toggles remove tools at most and do not create a
read-only sandbox. Setup therefore cannot save a read-only Codex reviewer: say
that a reviewer started by the writer shares the writer's sandbox, and do not
present a model, effort or role setting as enforcement. Enforced read-only
review needs a separate session started with `codex --sandbox read-only` that
runs the handed-off brief.

Codex also documents named custom agents in `.codex/agents/*.toml`. Do not assume
the installed client's delegation tool can select one. Our 0.160.0 evaluation
could pass explicit model and effort values but did not validate a named-role
selector. If the user needs Helix-only settings, check current support first;
otherwise give per-invocation settings without pretending they were persisted
as a working named role. Do not create a separate routing file.

Start the requested writer with:

```sh
codex --model <writer-model> -c 'model_reasoning_effort="<writer-effort>"'
```

In a fresh session, invoke Helix or second-opinion without model overrides to
use the saved subagent defaults. A fresh child must still omit inherited
conversation history. Verify its resolved `model` and `effort` in the native
child session records, matched to the parent dispatch. Read only records from
the verification run; do not search unrelated conversations or account files.

Check installed behavior against the official
[subagent documentation](https://learn.chatgpt.com/docs/agent-configuration/subagents)
and [configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
when it differs from these examples.
