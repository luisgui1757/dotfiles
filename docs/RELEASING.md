# Releasing dotfiles

`release/manifest.json` is the release identity source. Published schema-1
manifests and proofs describe historical releases, including
[v0.4.4](releases/v0.4.4.md); their bytes remain unchanged. New preparation writes
schema 2, selecting `installer-engine.yml` and the native jobs required by
`.github/check-identities.json`. The manifest, versioned notes and closure proof
are the only renderer-managed release artifacts. Historical migration ledgers,
source files and prior release advisories are not rewritten.

Release commands perform external writes and require an explicit maintainer
instruction. Preparing a release opens a PR; publishing creates a tag and release.
Neither command grants permission to merge a PR.

## Prepare reviewed notes and a PR

Start from clean `main` exactly matching official `origin/main`. Supply reviewed
notes outside the checkout, using the next release version:

```markdown
# v1.0.0 — Reviewed release title

> Release candidate notes. Publish these only with the official annotated
> `v1.0.0` tag after the deterministic evidence gate below passes.

## Highlights

- Reviewed user-facing changes.

## Compatibility and upgrade

State the supported platforms, migration boundary and local-checkout entrypoint.

## Release identity

The exact annotated tag and official remote must agree.

## Evidence required before publication

- Full local and required hosted gates.
- Exact tag, archived native jobs, scans, fresh clone and immutable release readback.
```

Replace the example version and title with the reviewed release. Do not claim
unexecuted native, owner-host or visual checks passed.

```bash
make release-check
make release-prepare VERSION=v1.0.0 NOTES=/absolute/path/reviewed-notes.md
```

Preparation validates the current published proof against the official tag,
release and workflow; checks clean exact main, version ordering and note shape;
creates a sibling `release/<version>` worktree; writes schema-2 candidate metadata
and notes; runs `make ci` and `git diff --check`; then commits, pushes and opens a
preparation PR. The worktree remains for review. No source-version constants or
old installer registries are generated. Review and merge separately through the
protected squash-only path.

The contributor gate requires the Go version declared in `installer/go.mod`.
Archive/component/wheel checksums and the bootstrap toolchain are manually
reviewed pins, updated with upstream provenance and actual runtime evidence.
Renovate covers the remaining declared workflow, Go and validator dependencies;
it does not calculate heterogeneous archive checksums.

## Certify the exact merged commit

After authorized merge, update local `main` to the exact official head:

```bash
make release-publish VERSION=v1.0.0 EXPECTED_SHA=<full-40-character-merged-commit>
```

The publisher verifies the unique merged preparation PR, equality of reviewed
and merged trees, all hosted checks, the exact canonical required-check policy,
and successful GitHub Actions checks at the reviewed PR head. A stale live
required-check policy blocks publication. It also requires immutable releases,
the complete local gate and a redacted Gitleaks scan of the release range before
creating the annotated tag.

A new `workflow_dispatch` run on that exact tag must pass on its first attempt.
Every manifest-bound native job must complete successfully with the exact commit,
run ID and attempt. Certification archives canonical API run/job records and the
exact-commit workflow source inside `release-proof.json`, with bounded byte size
and SHA-256. URLs alone are not evidence. Every setup-go step must disable tag
caching and the workflow must contain no broad Actions cache step. Branch Go
module caches do not establish cache-free release proof.

The archived evidence is scanned. A fresh credential-free detached public clone
must reproduce the annotated tag object, peeled commit and release-manifest
checks. Native package/archive fixtures verify actual selected bytes and runtime
outcomes in the hosted jobs; cross-compilation does not substitute for them.

Only then does the command create a draft with the exact reviewed public body,
upload the certification and verify GitHub's asset size and digest. The
certification truthfully records that immutable publication is pending. Type the
complete printed `PUBLISH IMMUTABLE ...` phrase to cross that irreversible
boundary. A mismatch leaves the draft unpublished. Final readback must be
immutable/latest/non-draft/non-prerelease. The closure PR records observed release
state, archived evidence and the uploaded asset's digest. It is not auto-merged.

## Recovery

- Before tag creation, correct the preparation tree and rerun its gates.
- An existing official annotated tag is accepted only if its local and remote
  objects peel to the exact expected commit. Never move or delete a release tag.
- `RUN_ID=<id>` reuses an observed first-attempt exact-tag run after a local
  failure; every identity and job is revalidated.
- A declined confirmation leaves the verified draft and exact proof asset.
  Reuse the same version, commit and run ID; existing bytes cannot be clobbered.
- If publication succeeded but closure failed, the publisher revalidates the
  immutable certification asset and reconstructs closure from live readback
  without republishing.
- Branch/worktree collisions fail closed. Preserve unmerged work and establish
  PR/release state before removing a collision.

Unexecuted redirected-Windows, divergent Windows Terminal, physical Linux,
Apple-Silicon owner-host and visual checks remain explicit residual evidence.
See [the test retirement map](installer-test-retirement.md) for current versus
historical gates, and [known issues](KNOWN-ISSUES.md) for version-specific history.
