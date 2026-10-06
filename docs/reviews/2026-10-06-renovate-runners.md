# Ubuntu 26.04 runner review

PR #84 moves hosted Linux execution to Ubuntu 26.04. The independent native
installer fixture remains an `ubuntu:24.04` container.

## Findings and fixes

- **Fixed: wrong Microsoft package repository.** Hosted job `112115136888`
  downloaded the Ubuntu 24.04 repository package on Ubuntu 26.04, downgraded
  `packages-microsoft-prod`, and failed at a deleted-keyring conffile prompt.
  The workflow now downloads the matching 26.04 package, verifies SHA-256
  `b2e7c6b9328e0c6a68bfe5d0045be90ceb192b32e0ca906c6c529b9012726f39`, and
  uses `dpkg --force-confmiss` to restore the vendor keyring noninteractively.
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

`tests/static/ubuntu_runner_contract_test.sh` semantically loads the workflows,
requires the Microsoft repository to match the runner, checks missing-keyring
restoration, and renders each matrix job name against its proof identity.
It failed on the original PR and passed after the fix. The existing required
check identity test also passed. The full local and hosted gates plus independent
review must pass before merge.

Independent review found one Low documentation issue: the native-container
rationale still called the hosted runner Ubuntu 24.04. The comment now says
"hosted Ubuntu runner"; the intentional container and compatibility names remain
24.04. The local `make ci` gate passed, as did the final static suite.
