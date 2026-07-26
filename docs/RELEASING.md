# Releasing dotfiles

Releases use one manifest-bound state machine instead of a hand-copied command
log. `release/manifest.json` identifies the current candidate or published
release, the exact public repository and workflow, and the four logical proof
contracts. A published release points to its checked-in closure proof under
`release/proofs/`.

The automation does not write release prose. A maintainer must supply reviewed
Markdown because commit subjects are not evidence and cannot safely determine
compatibility, user impact, or residual gaps.

## 1. Write candidate notes

Create a Markdown file outside the repository with this exact shape:

```markdown
# v0.4.4 — Reviewed release title

> Release candidate notes. Publish these only with the official annotated
> `v0.4.4` tag after the deterministic evidence gate below passes.

## Highlights

- User-facing, reviewed changes.

## Compatibility and upgrade

State the compatibility boundary and exact-tag upgrade path.

## Release identity

State the exact-tag and official-remote boundary.

## Evidence required before publication

- full local and hosted gates;
- exact tag, proof, scan, clone, and immutable release readback.
```

Do not include placeholders or claim that an unexecuted owner-host/manual row
passed. The six residual evidence categories remain explicit unless they were
actually executed and separately recorded.

## 2. Prepare the reviewed tree

Start on clean `main`, exactly equal to official `origin/main`:

```bash
make release-check
make release-prepare VERSION=v0.4.4 NOTES=/absolute/path/v0.4.4.md
```

Preparation fails before editing unless the published manifest, proof, official
remote, clean branch, exact main head, semantic version, and note shape all
validate. It then:

1. creates a sibling worktree on `release/v0.4.4`;
2. advances every controlled current-version surface;
3. adds the previous release to the shared POSIX/PowerShell legacy-recovery
   registries;
4. creates the candidate source row, evidence gate, roadmap entry, manual-test
   status, release notes, and candidate manifest;
5. preserves the tracked mode of every rewritten file, then runs `make ci`
   against the active-candidate state and `git diff --check`;
6. commits, pushes, and opens the preparation pull request.

The preparation worktree is intentionally retained for review. The command
never merges the PR. Review the semantic notes and generated diff, wait for all
required checks, then merge through the protected squash-only path.

## 3. Certify and publish exact merged main

Update local `main` to the exact merged commit and copy its full SHA from the
live repository. Publication refuses abbreviations or a different local/remote
head:

```bash
make release-publish \
  VERSION=v0.4.4 \
  EXPECTED_SHA=0123456789abcdef0123456789abcdef01234567
```

The command verifies the unique merged preparation PR, identical reviewed and
squash-merged trees, every required check, immutable-release policy, the full
local gate, and Gitleaks across the prior-release range before creating an
annotated tag. It then dispatches a new exact-tag run and requires:

- the exact four producer and four stable logical jobs, all successful at the
  expected SHA;
- all three PR-only cache steps skipped;
- both POSIX logs reporting the exact immutable tag identity;
- exactly four downloaded schema-2 proof markers, each independently verified
  against source SHA, executed SHA, run ID, attempt, logical context, and legacy
  context;
- a redacted scan of the downloaded proof tree;
- a credential-free detached public clone reproducing the tag object, peeled
  commit, release-upgrade gate, and Nix-prerequisite no-op identity path.

Only after those checks does it create a private draft release with the exact
reviewed public body. It generates and uploads `release-proof.json`, then reads
back the asset size and GitHub-computed SHA-256. That asset is intentionally a
pre-publication certification: it truthfully records the draft release ID and
that immutable publication is still pending. It cannot claim its own later
immutability.

The sole irreversible boundary requires typing the complete phrase printed by
the command, including tag and full commit SHA. A mismatch leaves the verified
draft unpublished. After confirmation, the command publishes it as latest,
requires immutable/latest/non-draft/non-prerelease readback, and opens a closure
PR. The closure proof records final publication time/state plus the exact digest
of the already-uploaded certification asset.

## Recovery and resumption

- Before the tag is pushed, failures have no release-side state. Inspect and
  correct the preparation tree rather than bypassing a gate.
- After the annotated tag exists, rerunning publication accepts it only when its
  local and official tag objects peel to `EXPECTED_SHA`.
- To reuse an already-observed exact-tag first-attempt run after a local failure,
  add `RUN_ID=<id>`. The run is still fully revalidated; this is not an evidence
  override.
- If confirmation is declined, the exact draft and proof asset remain private.
  Rerun with the same version, SHA, and run ID to revalidate them.
- If publication succeeded but closure work failed, rerun the same command. It
  downloads and validates the immutable certification asset, reconstructs the
  final proof from live readback, and opens the closure PR without republishing.
- Branch or worktree collisions fail closed. Remove them only after proving the
  associated PR/release state and preserving anything not merged.

Never delete or move an official release tag to recover from a failure. Never
edit an immutable release or hand-author a closure identity. Diagnose the failed
gate and resume from the observed state.
