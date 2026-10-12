---
name: setup-helix
description: Choose writer and reviewer models for Helix or second-opinion in Codex, Claude Code, Copilot CLI, Kiro or T3. Use to change role preferences, not to implement a task.
---

# Set up Helix

Use the current host's settings. Inside T3 with orchestration tools available,
use [T3 discovery and choices](references/t3.md) before the direct-host references.
Probe T3 capabilities only inside T3. A requested reviewer can run through a
supported CLI or API for another configured provider; read
[host dispatch](../helix/references/dispatch.md). Do not add or authenticate
providers without task authority. Do not install a router,
store credentials, or edit the shared skills to hold personal preferences.

Read the repository instructions and the relevant host reference:

- [Codex](references/codex.md)
- [Claude Code](references/claude-code.md)
- [GitHub Copilot CLI](references/copilot.md)
- [Kiro](references/kiro.md)
- [T3 Code](references/t3.md)

Identify the host, configuration scope, desired writer and reviewer models,
and reasoning levels from the request and existing configuration. Ask only
for missing choices. Keep defaults when the user wants defaults; do not select
models or efforts on their behalf. Show any broader effect of a setting
before writing it. Other hosts are outside this bundle's support.
Do not invent configuration keys or claim compatibility from file syntax alone.

With no choices supplied, guide the user through setup. Show the current
settings, then ask for scope and reviewer model using the host's question
controls, or numbered text choices when those controls are unavailable.
Discover models as described in the host reference. Before asking for a model,
print the complete numbered catalog with literal IDs and names. Include keeping
the current or inherited setting. Do not hide catalog entries: if a confirmed
restriction applies, name the affected entries and its source instead.
Use a question control only when all choices fit; otherwise ask for a number
or ID in text. Do not replace the catalog with a recommended shortlist.
Ask for effort after the model, using only its reported levels and an option
to keep the applicable default. Do not invent missing capabilities or treat
catalog membership as verified account access. An absent or failed catalog
leaves discovery unverified: offer to keep settings or accept an exact ID from
the native model picker. Do not install dependencies to obtain a list.

Keep the current writer unless the user asks to change it. Skip questions
already answered in the request. In guided setup, collect every choice first.
Then show the exact final edit to every affected file, including required
documentation, with no placeholders. In T3, preview supported T3 settings
or clearly labeled task-scoped choices as described in its reference. Explain
the scope and broader effect, then ask Save/Cancel for settings edits or Use/Cancel
for task-scoped choices that save nothing. Choosing a model or effort is not save approval.
A preselected answer or silence is not consent. If answers are unavailable,
list all missing choices (scope, model and effort as applicable), offer to keep
settings or supply exact values manually, and stop without edits. A complete,
authorized setup request needs no extra confirmation beyond host approvals.
It still needs the supported-settings check below before saving.

Inspect only relevant, non-secret configuration fields. Preserve unrelated
settings, instructions, comments, permissions, and hooks. Make the smallest
authorized edit, report its path and diff, and update repository documentation
when required. Repeating the same setup should leave the files unchanged.
Do not overwrite an existing role or resolve a conflicting preference by guess.
If the host requires approval for a protected configuration file, present the
exact pending edit through its approval flow. A denied edit remains pending;
do not switch tools or weaken permissions to get around it. Update documentation
only to describe what was actually saved.

The writer is the current session. Give the exact launch command, or supported
T3 new-thread/delegation choices, for a requested writer change; saving
configuration does not switch the running session.
Distinguish the running writer from saved defaults; if the session settings
are not exposed, report them as unverified rather than infer them from a file.
Keep model IDs literal when a version matters. Before saving, check the requested
model and effort against current native metadata or documentation, even for a
complete request. If discovery fails, use manual choices and identify anything
still unverified. If a requested setting is unsupported, stop without
substituting another model or effort.

Distinguish **configured**, **verified**, and **unverified**. When execution
verification is authorized, launch a fresh session and a small read-only child
task without restating saved reviewer settings in the prompt or spawn call.
For T3 task-scoped choices, pass the explicit target and label this a dispatch
probe, not proof that defaults were saved.
Check native execution records for the resolved model and effort. Configuration
files, self-reports, and response timing are not proof. A runtime rejection
leaves verification blocked; do not silently change the saved choice to pass.
After guided setup, offer this verification separately; explain that it uses
the account's model allowance. Discovery itself starts no inference turn.

Report what was saved, its scope, what a new session must do, and which settings
were observed. Do not claim a full delivery workflow was tested by a dispatch
probe. Setup does not implement the user's feature or certify an audit.
