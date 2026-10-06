# Ubuntu 26.04 runner review

PR #84 moves hosted Linux execution to Ubuntu 26.04. The independent native
installer fixture remains an `ubuntu:24.04` container.

## Findings and fixes

- **Superseded intermediate fix: wrong Microsoft package repository.** Hosted job `112115136888`
  downloaded the Ubuntu 24.04 repository package on Ubuntu 26.04, downgraded
  `packages-microsoft-prod`, and failed at a deleted-keyring conffile prompt.
  The first correction downloaded the matching 26.04 package, verified SHA-256
  `b2e7c6b9328e0c6a68bfe5d0045be90ceb192b32e0ca906c6c529b9012726f39`, and
  used `dpkg --force-confmiss` to restore the vendor keyring noninteractively.
  A real Ubuntu 26.04 container reproduced the old downgrade/prompt failure
  after installing prerequisites and removing the keyring. The corrected
  invocation then succeeded and restored the nonempty keyring. The download
  source is Microsoft's `config/ubuntu/26.04/packages-microsoft-prod.deb`.
- **Fixed: release producer names drift with runner labels.** The proposed
  matrix still emitted `setup.sh / ubuntu-26.04` and
  `nix flake check (ubuntu-26.04)` while proof metadata and release certification
  consume the compatibility identities ending in `ubuntu-24.04`. Both jobs now
  render their existing `legacy_context` as the display name. Logical required
  check identities and branch-protection settings remain unchanged.

The first `tests/static/ubuntu_runner_contract_test.sh` revision semantically
loaded the workflows, required the Microsoft repository to match the runner,
checked missing-keyring restoration, and rendered each matrix job name against
its proof identity.
It failed on the original PR and passed after the fix. The existing required
check identity test also passed. The full local and hosted gates plus independent
review must pass before merge.

Independent review found one Low documentation issue: the native-container
rationale still called the hosted runner Ubuntu 24.04. The comment now says
"hosted Ubuntu runner"; the intentional container and compatibility names remain
24.04. The local `make ci` gate passed, as did the final static suite.

## Hosted verification follow-up

The first corrected hosted run (job `112120265137`) proved the missing-keyring
repair, then failed because Microsoft's Ubuntu 26.04 repository has no
`powershell` package. The final workflow removes the redundant repository setup
and apt installation. The exact hosted image
[`ubuntu26/20260927.149`](https://github.com/actions/runner-images/blob/ubuntu26/20260927.149/images/ubuntu/Ubuntu2604-Readme.md)
already provides PowerShell 7.6.6. CI now unconditionally locates `pwsh` and
executes a major-version check (7 or newer), so missing PowerShell cannot turn
static parsing into a silent skip. This follows the same image-owned runtime
contract used by hosted Windows.

The runner regression was updated to require that runtime probe and reject the
unavailable apt path. It failed before this correction and passed afterward.
The supply-chain guard drops only the two assertions for the removed `.deb`;
all remaining downloaded artifacts retain their integrity checks. The earlier
matching-repository approach above remains recorded as an insufficient fix,
not the final implementation.

After the hosted follow-up correction, the full local `make ci` gate passed.

Rereview found one Low stale README download-list entry for the now-removed
Microsoft repository package. The entry was removed; the hosted PowerShell
requirement remains documented in the agent guide and supply-chain ledger.
