# Branch Protection

This public repository protects `main` with three active GitHub repository
rulesets plus a classic branch-protection fallback. Both required-check layers
are transitioned by `scripts/apply-repo-safeguards.sh`; `.github/settings.yml`
intentionally does not own branch protection. The split is intentional:

| Layer | Bypass actors | Purpose |
|---|---:|---|
| `Protect main: integrity` | none | Requires PRs, strict required CI checks, squash-only merges, linear history, no branch deletion, and no non-fast-forward updates. |
| `Protect main: review` | `luisgui1757` on pull requests only | Requires one approval, CODEOWNER review, stale-review dismissal, last-push approval, and resolved review threads without letting owner bypass skip CI. |
| `Protect main: owner updates` | `luisgui1757` on pull requests only | Allows only the owner to update `main`, so a bot or limited automation token can open PRs but cannot merge them. |

Do not merge the two rulesets. A single bypassable ruleset would let the owner
bypass review friction and CI in the same action. The integrity ruleset has no
bypass actors so a CI-failing commit cannot be merged into `main` through the
normal protected-branch path. The owner-updates ruleset is separate so
least-privilege automation can write branches and PRs without being able to
update `main`.

## Apply

This cutover is deliberately narrower than a general "make live look like the
files" reconciler. After the merged-main proof below, run the complete no-write
preflight first, then the owner-authorized apply:

```bash
scripts/apply-repo-safeguards.sh --preflight-only luisgui1757/dotfiles
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

Before its first mutation, the script completes one read-only preflight and a
second concurrency readback. It requires all of the following:

- local branch `main`, exact live-main HEAD, and an `origin` that resolves to
  `github.com/luisgui1757/dotfiles`;
- repository visibility is explicitly public (`private:false`,
  `visibility:public`) in both preflight captures;
- clean reviewed safeguard, workflow, and logical-proof sources;
- exactly the three named active repository rulesets, with the review and
  owner-update payloads already identical to the reviewed files;
- integrity ruleset, classic protection, and Actions permissions in one
  internally consistent legacy stage (first apply) or stable stage (idempotent
  rerun), never a partial mix;
- the reviewed squash-only merge and public security posture;
- successful `test.yml` and `nix.yml` push/manual runs plus an
  `e2e-install.yml` workflow-dispatch run on the exact live-main SHA;
- every expected producer/logical job supplied by GitHub Actions app `15368`,
  with every broad `actions/cache` step skipped in the E2E dispatch.

Unexpected live drift, duplicate/missing rulesets, wrong repository/branch,
wrong check app/run/event, cached E2E evidence, source changes, or concurrent
preflight changes abort with zero live writes. The apply then changes only
Actions SHA pinning, the integrity required-check set, and the classic required
checks. Unchanged merge, review, owner-update, and security policy is verified,
not rewritten. Every capture directory is caller-owned and removed on success,
failure, or interruption; a failed second capture cannot hide the first.

The Probot Settings app synchronizes `.github/settings.yml` after changes reach
the default branch. Its upstream implementation processes branch settings only
when a `branches` key is present ([reviewed source at
`3629848d090115df71f6d5cf431561e67077ee27`](https://github.com/repository-settings/app/blob/3629848d090115df71f6d5cf431561e67077ee27/lib/settings.js#L24-L36)).
This repository deliberately omits that key. Otherwise the app could apply the
future classic contexts immediately after merge, before the owner-run ruleset
transaction, leaving a mixed stage that the fail-closed preflight would reject.
The guard parses YAML and rejects a top-level `branches` key even when it is an
inline array/map, `null`, or an alias. Regex matching of one presentation is not
considered proof that Probot lacks branch ownership.

Immediately before mutation, the script stores the complete recovery material
under `.git/dotfiles-safeguards/recovery.*` with private permissions. Any
mutation or readback failure automatically restores all three changed resources
and verifies the old stage. Before any restore write, it requires every consumed
snapshot file, copies those bytes into a private temporary directory, and
validates the frozen set against the manifest's exact legacy/stable stage:
Actions pinning, integrity contexts and app IDs, unique live ruleset identity,
bypass actors, branch conditions, every consumed full-classic key, disabled
review/restriction sections normalized from either GitHub-omitted keys or
explicit `null`, and the narrow classic restore payload. Missing, malformed,
enabled optional policy, incomplete, altered, cross-stage, wrong-target, or
internally inconsistent source bytes fail with zero writes; changes to the
retained source after freezing cannot affect publication. The expected
legacy/stable policy is loaded from the manifest's exact captured Git commit;
that commit must still be live `main`, and a moved or unavailable commit fails
before mutation. Integrity and full/narrow classic expectations are rebuilt
from that same captured `check-identities.json`; restore never calls a
worktree-backed context helper. A coherently altered classic snapshot matching
the running checkout therefore fails before all three writes instead of
creating a captured-integrity/worktree-classic mixed stage. Restore publishes
and verifies only the frozen validated bytes.

Apply has the same publication boundary. After the second live capture, it reads
the check metadata and integrity source from exact committed Git objects, checks
the worktree still matches, derives the classic and Actions payloads, validates
the manifest and all cross-file identities, then makes the transaction directory
read-only. All three API writes consume only those files; no post-validation
write reads a checkout path. Postflight derives integrity, classic, review, and
owner expectations only from that read-only transaction and compares unchanged
repository/ruleset state with the frozen second capture. It then repeats the
local exact-main/clean-source boundary check after readback, closing the interval
between the first postflight boundary check and expectation construction. A
snapshot created before that boundary is deleted
if apply aborts without a live write. Once mutation begins—or apply succeeds—the
original snapshot remains available for independent readback and recovery. If
automatic rollback cannot complete, use the exact path printed by the failure:

```bash
scripts/apply-repo-safeguards.sh --restore '/exact/snapshot/path' luisgui1757/dotfiles
```

Do not guess through unexpected drift or delete a duplicate blindly. Inspect the
live discrepancy and recovery snapshot, correct it through an independently
reviewed change, and rerun the no-write preflight.

## Context Renames

Never rename a required check in place. The integrity ruleset has no bypass
actors, so removing an old context before the live setting changes deadlocks the
PR; requiring a new context before GitHub has observed it creates the inverse
deadlock. `.github/check-identities.json` records the transition.

This PR is the checked-in cutover stage. The ruleset, required-check metadata,
and transactional apply path name the stable per-OS logical checks, while
workflows continue to emit both the legacy runner-versioned producers and
stable checks. Each stable check downloads the
exact producer artifact and verifies the source head SHA, actually executed
SHA, run id/attempt, logical identity, and legacy producer through
`scripts/ci-logical-proof.sh`; it is not a fake/no-op check. GitHub executes a
synthetic merge commit for `pull_request`, so schema 2 records that commit
separately from `github.event.pull_request.head.sha`. On push, schedule, and
workflow dispatch the two identities normally match. Because live GitHub
remains on the legacy set, those producer jobs continue to gate this PR without
a rename deadlock.

After this PR merges, use this exact sequence:

1. Dispatch cache-free `e2e-install.yml` on the exact merged `main` SHA and
   require the four current producers plus all four stable setup logical checks
   to pass. Require the two stable Nix logical checks on that same merged SHA.
2. From an exact clean local `main` whose `origin` is this repository, have the
   owner run the no-write preflight. It independently verifies the legacy live
   set, complete live posture, exact workflow/run/app provenance, and cache-free
   E2E evidence before returning success.
3. Apply the checked-in stable cutover. Retain the printed recovery snapshot
   until independent readback/review is complete:

```bash
scripts/apply-repo-safeguards.sh --preflight-only luisgui1757/dotfiles
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

4. Verify both the integrity ruleset and classic fallback require the stable
   logical set and Actions reports `sha_pinning_required:true`. Runner labels
   can then change without renaming a required context. If readback fails, the
   script restores the prior legacy stage automatically or prints the exact
   `--restore` command. No live mutation is authorized from this PR itself.

Exact repaired behavior head
`e5cf3e23299cbb42a157c307f2a7259979fcada0` passed cache-free run
`29103732329`, including every then-current producer and all four setup logical
proofs. Merged-main run `29114125798` on PR #48 merge SHA
`f104bf066e4af7d4d707fe22ba36600711f1ae14` then exposed a separate strict
CMake LSP project-isolation defect on Apple Silicon: the initial neocmakelsp
probe timed out in the shared fixture tree while the later isolated formatter
project attached in the same process. This PR repairs that boundary. Do not run
the owner apply until the post-merge run completes the four current setup
producers, four setup logical checks, and two Nix logical checks. Exact behavior
head `f097995b49a2189db327903a20743e7cb69ba665` already passed cache-free run
`29120109175` with all four current producers and all four setup logical proofs;
that is branch-head evidence, not permission to skip merged-main proof.

## Verify

```bash
make test-static
integrity_id="$(
  gh api repos/luisgui1757/dotfiles/rulesets \
    --jq '.[] | select(.name == "Protect main: integrity") | .id'
)"
gh api "repos/luisgui1757/dotfiles/rulesets/$integrity_id" \
  --jq '{
    name,
    enforcement,
    bypass_count:(.bypass_actors | length),
    strict:(.rules[] | select(.type == "required_status_checks") | .parameters.strict_required_status_checks_policy),
    contexts:(.rules[] | select(.type == "required_status_checks") | .parameters.required_status_checks | map(.context))
  }'
gh api repos/luisgui1757/dotfiles/branches/main/protection/required_status_checks \
  --jq '{strict,contexts}'
gh api repos/luisgui1757/dotfiles --jq '{allow_merge_commit,allow_squash_merge,allow_rebase_merge,allow_auto_merge,delete_branch_on_merge}'
gh api repos/luisgui1757/dotfiles/actions/permissions \
  --jq '{enabled,allowed_actions,sha_pinning_required}'
```

Checked-in desired posture during the cutover (the current live exception is called
out immediately below):

- merge commits disabled;
- rebase merges disabled;
- squash merges enabled;
- auto-merge disabled;
- Actions enabled with `sha_pinning_required: true` after the owner applies the
  post-merge safeguard update;
- required checks are strict and include exactly:
  `ubuntu`, `macos`, `windows`, `chezmoi-parity`, `chezmoi-parity-macos`,
  `chezmoi-parity-windows`, `nix flake check / linux`,
  `nix flake check / macos`, `e2e containers / linux`, `setup.sh / linux`,
  `setup.sh / macos`, and `setup.ps1 / windows` in both the integrity ruleset
  and classic fallback;
- the workflows continue emitting the six replaced legacy producer contexts in
  `.github/check-identities.json` until the live owner apply is verified;
- only `Protect main: review` and `Protect main: owner updates` have bypass actors;
- each bypass actor is `luisgui1757` with `bypass_mode: pull_request`;
- `Protect main: integrity` has no bypass actors.

Checked-in versus live truth matters: at the start of the 2026-07-10 closure
branch, the live Actions permissions endpoint reported
`sha_pinning_required: false`, and live required contexts were the twelve legacy
names. The branch updates the desired safeguard sources but does not apply them.
The owner must run the apply command only after the merged-main proof above;
until then both the SHA policy and stable-context cutover are pending live
confirmation, not already-active safeguards.

GitHub does not let pull request authors approve their own pull requests. Owner
authored PRs can use the owner review bypass, but only after the non-bypass
integrity layer has passed.

Repository deletion is not solved by branch protection in a personal account.
Routine agents and automation must use least-privilege credentials that can open
branches and pull requests but cannot administer or delete the repository.

Routine agent credentials must not be authenticated as `luisgui1757`. Use a
separate GitHub App or bot identity with branch/PR write access and no
administration permission; otherwise GitHub cannot distinguish the agent from
the owner.
