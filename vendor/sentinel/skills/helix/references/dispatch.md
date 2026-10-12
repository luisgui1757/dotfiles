# Dispatch a writer or reviewer

Supported hosts are Codex, Claude Code, GitHub Copilot CLI and Kiro. T3 can
coordinate the providers its live catalog actually exposes. A model name is not
an executable, and a CLI being installed does not prove account access.

1. Identify the current host and the requested role, model and effort. Inspect
   configured roles and installed CLI help. Use native delegation when it can
   honor those choices. Otherwise use the requested provider's supported API,
   SDK or headless CLI. Do not install a router or silently choose another model.
2. Give the child a self-contained task, repository instructions, scope, baseline
   and evidence. Do not assume another provider or custom agent inherits rules.
   Keep one writer. Reviewers receive the full diff and check output, with
   read-only controls when available. Do not share credentials or raw account files.
3. Restrict tools and paths before launching. Review project hooks, plugins and
   MCP configuration before allowing them. Tool preapproval is not a sandbox.
   Record the actual permissions exposed, including shell or write access.
4. Wait using the host's completion mechanism. If cancelling, stop the process
   or native task and confirm termination before starting its replacement.
5. Check exit state and execution metadata. A request flag, role name or model's
   self-description does not prove execution. Report actual model and effort
   when exposed; label other settings unverified. Never silently fall back.

Read only the reference for the selected route:

| Route | Reference |
|---|---|
| Codex | [CLI and permissions](codex.md) |
| Claude Code | [Headless review](claude.md) |
| GitHub Copilot CLI | [CLI and custom agents](copilot.md) |
| Kiro | [Headless review and resources](kiro.md) |
| T3 | [Live orchestration](t3.md) |

Inside T3, discover its advertised orchestration tools, including deferred tools,
and inspect their current schemas. Outside T3, do not probe for T3 tools.
An unavailable requested route blocks that role, not unrelated authorized work.
