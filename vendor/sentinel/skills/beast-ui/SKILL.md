---
name: beast-ui
description: Improve an existing UI or generated document view through an ordered design and performance check. Use for a complete quality pass before handoff.
---

# Beast UI

Run Impeccable's audit, layout, typeset, polish and optimize playbooks in that
order. Each stage sees the previous stage's result. A stage may find nothing
that needs changing.

Read the target and its project rules. Preserve the chosen style, factual copy
and intended behavior. Resolve the installed [Impeccable](../impeccable/SKILL.md)
skill and its base directory, then load the relevant playbook from its
`reference/` folder for each stage. Use the host's actual skill mechanism or
read the files directly; do not invent a `Skill` tool or plugin command. Run every
engine command in these playbooks through Impeccable's pinned helper.

1. **Audit.** Find accessibility, responsive layout and rendering defects.
2. **Layout.** Fix structure, spacing, hierarchy and any blocking layout defects.
3. **Typeset.** Improve heading scale, line length, fonts and readability.
4. **Polish.** Check alignment, interaction states, content and small inconsistencies.
5. **Optimize.** Measure loading, asset weight or rendering costs. Fix demonstrated
   problems without changing the approved design or adding speculative machinery.

Use Impeccable's required context and craft guidance within the authorized scope.
Do not turn a document view into product discovery, a redesign or maintenance
work. Missing project context alone does not authorize creating new project rules.
Keep the running findings in the conversation unless a report was requested.

Verify the final result with Playwright or an equivalent real browser. Inspect
screenshots at desktop and narrow widths; exercise controls and keyboard access.
Fix material defects and inspect the corrected result. A static source check or
saved screenshot without inspection does not prove the UI works.

Hand back the changed result and any remaining limitation briefly. Do not
claim a complete pass when a required playbook, engine or browser check did not
run. This skill does not commit, publish or merge unless the task authorizes it.
