# Sources and licenses

Sentinel uses the [MIT license](LICENSE). Imported work retains its own license.

## Helix collection

Delivery, writing and document skills were brought from
[Helix PR #30](https://github.com/luisgui1757/helix/pull/30), revision
`9ede165fedbcff085d354291f936d1cba241ba47`. The collection includes the work from
Helix PRs #28 and #29. This integration updates host routes, wording, document
verification and internal helpers. Beast UI adapts the maintainer's existing
ordered Impeccable quality pass into a portable skill.

## Unslop

Adapted from Lauren Tan's [pstack Unslop](https://github.com/cursor/plugins/tree/main/pstack/skills/unslop).
The [import manifest](upstream/imports.json) records its exact revision. Its
[MIT license](skills/unslop/LICENSE) is included. Helix's scope and factual
preservation safeguards remain; the numbered rules and their IDs are unchanged.

## Frontend-design

Anthropic's [Frontend-design](https://github.com/anthropics/skills/tree/main/skills/frontend-design)
is included unchanged at the manifest's revision, with its
[Apache 2.0 license](skills/frontend-design/LICENSE.txt). Tests verify both files.

## Impeccable

Paul Bakaus's [Impeccable](https://github.com/pbakaus/impeccable)
is included at the manifest's revision.
The `.agents/skills/impeccable` distribution is copied with its original entrypoint
named `UPSTREAM.md`. The [upstream lock](upstream/lock.json) records
every original file's digest. No upstream file is reformatted or edited.

Sentinel adds a short entrypoint and a pinned engine helper. Engine asset sizes
and digests come from its official release metadata and travel with the skill's
engine version. The helper avoids
mutable PATH selection and verifies cached bytes before execution. Setup is
explicit; hooks and provider configuration are not installed.

The [Apache license](skills/impeccable/LICENSE),
[upstream notices](skills/impeccable/NOTICE.md),
[platform reference MIT license](skills/impeccable/PLATFORM-LICENSE), and
[screenshot library MIT license](skills/impeccable/SCREENSHOT-LICENSE) are included.
The latter covers the bundled modern-screenshot library. Upstream notices identify
the platform-design-skills source.

Formatting exceptions cover only the imported files with upstream newline or
whitespace differences. Their source hashes remain checked; secret scanning still
covers the bundle.

See [upstream updates](docs/upstream.md) for reproducible imports and review.
