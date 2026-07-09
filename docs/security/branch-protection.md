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
aligned, and enables GitHub security alerts/security fixes where the plan
supports them. If GitHub has duplicate rulesets with the same protected name,
the script fails closed; delete the duplicate live ruleset before re-running it.

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
```

Expected live posture:

- merge commits disabled;
- rebase merges disabled;
- squash merges enabled;
- auto-merge disabled;
- required checks are strict and include exactly:
  `ubuntu`, `macos`, `windows`, `chezmoi-parity`, `chezmoi-parity-macos`,
  `chezmoi-parity-windows`, `nix flake check (ubuntu-24.04)`,
  `nix flake check (macos-26)`, `e2e containers / ubuntu-24.04`,
  `setup.sh / ubuntu-24.04`, `setup.sh / macos-26`, and
  `setup.ps1 / windows-2025` in both the integrity ruleset and classic fallback;
- only `Protect main: review` and `Protect main: owner updates` have bypass actors;
- each bypass actor is `luisgui1757` with `bypass_mode: pull_request`;
- `Protect main: integrity` has no bypass actors.

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
