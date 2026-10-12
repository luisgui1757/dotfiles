---
name: impeccable
description: Design, review or improve an interface using Impeccable's design playbooks. Covers hierarchy, layout, typography, accessibility, responsive behavior and performance.
license: Apache-2.0
---

# Impeccable

Read [the bundled Impeccable guidance](UPSTREAM.md) and the playbook relevant to
the task. Preserve the user's brief and existing project style. The upstream
design guidance and resources are included; these integration rules govern setup.

- Use `node <skill-base>/scripts/sentinel-engine.mjs <command>` for engine commands
  throughout the upstream guidance, including `context` and `detect`. This checks
  the pinned binary before execution instead of selecting an unrelated PATH binary.
- If the engine is missing, run the same helper with `--setup` only when the task
  permits downloading and writing its cache. Setup reports the exact destination.
  Otherwise report the missing setup and continue only independent work. Do not
  claim an engine-dependent audit or quality pass ran.
- Ignore upstream update notices. Never run `update` or `npx impeccable` against
  this bundle; update it through the Sentinel installer so pins and ownership stay intact.
- Setup installs no hooks, other skills or model configuration. Commands such as
  `hooks`, `pin`, `doctor` and project-context creation still need task authority.
- A document view uses Read mode and its selected style. It does not authorize
  unrelated product discovery, global maintenance or a change to source facts.

The helper needs Node.js 22.19 or newer. Its cache defaults to the user's
`.impeccable` directory; `IMPECCABLE_HOME` can select an authorized cache instead.
The upstream launchers remain packaged for provenance, but this bundle uses the
verified helper above. See the packaged licenses and notices.
