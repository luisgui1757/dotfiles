# Branch Protection

This public repository protects `main` with three active GitHub repository
rulesets plus the classic branch-protection fallback in `.github/settings.yml`.
The split is intentional:

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

After changing `.github/rulesets/*.json`, run:

```bash
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

The script sets squash-only repository settings, keeps auto-merge disabled,
upserts the three rulesets, keeps the classic branch protection fallback
aligned, requires full-SHA external GitHub Actions references at the repository
policy layer, and enables GitHub security alerts/security fixes where the plan
supports them. If GitHub has duplicate rulesets with the same protected name,
the script fails closed; delete the duplicate live ruleset before re-running it.

## Context Renames

Never rename a required check in place. The integrity ruleset has no bypass
actors, so removing an old context before the live setting changes deadlocks the
PR; requiring a new context before GitHub has observed it creates the inverse
deadlock. `.github/check-identities.json` records the transition.

Stage 1 is this branch: legacy runner-versioned contexts remain required in all
four safeguard sources, while workflows also emit stable per-OS logical checks.
Each new check downloads the exact producer artifact and verifies the head SHA,
run id/attempt, logical identity, and legacy producer through
`scripts/ci-logical-proof.sh`; it is not a fake/no-op check.

After this PR merges, use this exact sequence:

1. Verify all six logical checks passed on the merged `main` SHA.
2. Open a second PR changing `.github/check-identities.json` to the candidate
   stage and updating `.github/settings.yml`,
   `.github/rulesets/main-integrity.json`, and
   `scripts/apply-repo-safeguards.sh` to `candidateRequired`. Do not remove the
   legacy producer jobs; they still let the live legacy ruleset gate this PR.
3. Merge that second PR only after the still-live legacy contexts pass.
4. From updated `main`, have the owner apply the checked-in safeguards:

```bash
scripts/apply-repo-safeguards.sh luisgui1757/dotfiles
```

5. Verify both the integrity ruleset and classic fallback require the candidate
   logical set. Runner labels can then change without renaming a required
   context. No live ruleset mutation is authorized from the stage-1 PR.

The 2026-07-10 cache-free merged-main run `29096335827` is an explicit gate on
step 2. Its first attempt exposed an asynchronous nvim-treesitter build race in
the Apple Silicon producer, so the logical macOS proof was not green. Do not
open or apply the context-switch stage until the waitable-update plus headless
auto-install repair is merged and all six logical checks pass on that newer
merged-main SHA. Branch-head run `29100106370` disproved the first,
build-hook-only patch; neither that run nor a rerun of an older SHA can prove
the complete repaired behavior. Exact repaired behavior head
`e5cf3e23299cbb42a157c307f2a7259979fcada0` passed cache-free run
`29103732329`, including every producer and all four setup logical proofs. That
closes branch-head proof but does not relax step 2: merge the repair, rerun on
its merged-main SHA, and require all six setup-plus-Nix logical checks before
opening the context-switch PR.

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

Checked-in desired posture during stage 1 (the current live exception is called
out immediately below):

- merge commits disabled;
- rebase merges disabled;
- squash merges enabled;
- auto-merge disabled;
- Actions enabled with `sha_pinning_required: true` after the owner applies the
  post-merge safeguard update;
- required checks are strict and include exactly:
  `ubuntu`, `macos`, `windows`, `chezmoi-parity`, `chezmoi-parity-macos`,
  `chezmoi-parity-windows`, `nix flake check (ubuntu-24.04)`,
  `nix flake check (macos-26)`, `e2e containers / ubuntu-24.04`,
  `setup.sh / ubuntu-24.04`, `setup.sh / macos-26`, and
  `setup.ps1 / windows-2025` in both the integrity ruleset and classic fallback;
- the workflows additionally emit the six `candidateRequired` logical contexts
  in `.github/check-identities.json`, but they are not live-required until the
  second checked-in migration PR and owner apply above;
- only `Protect main: review` and `Protect main: owner updates` have bypass actors;
- each bypass actor is `luisgui1757` with `bypass_mode: pull_request`;
- `Protect main: integrity` has no bypass actors.

Checked-in versus live truth matters: at the start of the 2026-07-10 closure
branch, the live Actions permissions endpoint reported
`sha_pinning_required: false`. The branch updates the desired safeguard script
but does not apply it. The owner must run the apply command after merge; until
then this item is pending live confirmation, not an already-active safeguard.

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
