# Releasing dotfiles

A merge that changes `version` in `release/manifest.json` publishes that
version. The [release workflow](../.github/workflows/release.yml) does the
work. Nobody runs a release command or types a confirmation.

Releases up to v1.1.0 used candidate and closure pull requests. Their
manifests, notes and proofs in `release/proofs/` stay unchanged. The proof of a
later release is its `release-proof.json` asset on GitHub.

## Release a version

Open one pull request with these changes:

1. Set `version` in `release/manifest.json` to the next `vMAJOR.MINOR.PATCH`.
2. Add the reviewed notes as `docs/releases/<version>.md`.
3. Add the code changes of the release, if there are some.

The notes have this shape:

```markdown
# v1.2.0 — Reviewed release title

## Highlights

- Reviewed user-facing changes.

## Compatibility and upgrade

State the supported platforms, migration boundary and local-checkout entrypoint.
```

The H1 text is the title of the GitHub release. The release body is the notes
from `## Highlights` to the end. Do not claim that unexecuted native,
owner-host or visual checks passed. `make release-check` validates the
manifest and the notes, and the required checks of the pull request run it.

When the pull request merges, the release workflow publishes the version. A
merge that does not change the version publishes nothing. Merge is an owner
action, unless the owner gives it to an agent.

Archive, component and wheel checksums, the bootstrap toolchain, the Gitleaks
pin in `release.yml` and the
[Mason registry snapshot](../nvim/mason-registry/README.md) are manually
reviewed pins. Update them with upstream provenance and runtime evidence.
The daily pin bot proposes verified updates for its explicit archive recipes.
It groups them into one PR and holds major releases for manual review.
Renovate lists the new releases of all pins on its dashboard as notices.
See [archive pin proposals](security/supply-chain.md#archive-pin-proposals) for
coverage and exceptions. Renovate covers the other declared workflow, Go and
validator dependencies. It does not calculate archive checksums.

## What the release workflow checks

The workflow runs `scripts/release.py publish` on the merge commit. It stops
at the first failure:

1. If the merge did not change the version, the workflow stops and changes
   nothing.
2. If the version already has a published release, the workflow stops. It
   changes nothing when that release is immutable, and fails when it is not.
3. The version must be newer than the latest GitHub release, and that release
   must be immutable. It becomes the previous release.
4. The merge commit must be the squash merge of one pull request into `main`.
   Each check on the head of that pull request must be complete, and no check
   can fail. Each required check must pass one time from GitHub Actions. The
   live required checks of `main` must equal `.github/check-identities.json`.
   The merged tree must equal the tree of the pull request head.
5. Gitleaks scans the commits from the previous release to the merge commit,
   with redaction. Put a reviewed false positive into `.gitleaksignore` as its
   exact fingerprint (`commit:file:rule:line`) with the reason. Never add a
   pattern or a rule for a full path.
6. The workflow creates the annotated tag on the merge commit.
7. A run of `installer-engine.yml` at the merge commit must pass on its first
   attempt. The workflow uses the push run on `main` that the merge started.
   When that run does not exist, does not pass or gets a new attempt, the
   workflow uses a first-attempt `workflow_dispatch` run on the tag that passed
   or still runs, or starts one. A later merge into `main` can cancel the push
   run, so the release then takes longer. Each native job in
   `.github/check-identities.json` must pass at the exact commit. The proof keeps
   the API records of the run and its jobs, and the workflow source of the
   commit, with their size and SHA-256. Each setup-go step must disable the Go
   cache outside pull requests, and the workflow must have no broad Actions
   cache step. Gitleaks scans this evidence.
8. A fresh public clone without credentials must give the same tag object and
   commit, and pass the release manifest checks.
9. The workflow creates a draft with the reviewed title and body, and uploads
   `release-proof.json`. GitHub must report the same size and SHA-256 for the
   asset.
10. The workflow publishes the draft as the latest release. GitHub must report
    it as immutable, latest, not a draft and not a prerelease.

URLs alone are not evidence. Native package and archive fixtures in the hosted
jobs verify the selected bytes and runtime results. Cross-compilation does not
replace them.

## Support window

`main` reads each state that a release tag from v1.2.0 on can leave on a
machine. This includes receipts and saved recipes that the release keeps
without a change. A machine can skip releases from v1.2.0 on, so `main` keeps
each of their readers. `TestEachResourceOfAReleaseStaysInTheCatalog` fails when
a resource of such a release tag leaves the catalog and is not in
`retiredResources`.

An older installation updates to v1.2.0 first. A v0.4.x installation runs
migrate from the v1.0.1 checkout before that. Then it updates to `main` and
chooses Update tools.

## Recovery

- Re-run the failed run of the release workflow. A re-run keeps the tag, a
  first-attempt push or exact-tag run that passed or still runs, and the draft.
  It starts a new exact-tag run only when no such run exists. A re-run push run
  does not count, so the release then starts an exact-tag run.
- Never move or delete a release tag. A tag that exists must peel to the merge
  commit.
- When the code of a version cannot pass, merge the fix with a newer version.
  The old version keeps its notes, and its tag if the workflow made one. It
  gets no release. v1.0.0 was replaced this way by v1.0.1 (2026-10-28): each
  macOS Node download in its tag runs failed on degraded hosted networking.
- The workflow runs one release at a time. While a release runs, GitHub keeps
  only the newest waiting run. A version that merged between them gets no
  release.
- A draft whose `release-proof.json` differs from the new proof stops the
  workflow. Delete that draft, then re-run.
- If immutable releases are off, the workflow publishes a mutable release and
  then fails. Delete that release, turn on immutable releases, then re-run.
- GitHub can refuse a tag push from the workflow token when the tagged commit
  changes a workflow file. Then no tag exists and nothing is published. Release
  that change with a later version change that does not change a workflow.

Unexecuted redirected-Windows, divergent Windows Terminal, physical Linux,
Apple-Silicon owner-host and visual checks remain explicit residual evidence.
The release notes in [releases](releases/) record the history of each version.
[Known issues](KNOWN-ISSUES.md) lists the problems of each release.
