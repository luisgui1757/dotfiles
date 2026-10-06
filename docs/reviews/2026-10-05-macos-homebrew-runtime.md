# macOS Homebrew runtime compatibility

## Scope and cause

The owner explicitly requires all dotfiles CI failures fixed before the AI VM
deployment continues, including verification of Windows impact. This adds the
macOS installation failure to PR #85 without resetting the Herdr review rounds.

The macOS E2E failed identically on baseline `5154aae` and PR commit `25f3edf`.
Both used Homebrew 6.0.1 from the committed nix-homebrew lock, with mutable
current casks. AeroSpace's postflight steps interpolated a version unsupported
by that runtime. Ghostty's completion artifacts had no installation rank, so
they could precede the app containing their source files.

The change updates only the `nix-homebrew` and `brew-src` lock nodes using
`nix flake update nix-homebrew`. The selected nix-homebrew commit is
`c11cccfdd36dd69b5323d70d354b2853379c2426`; its upstream lock selects Homebrew
7.0.4 at `edb70f031e4170c780799633a1226ff73e1077f4`. The official Homebrew tag
resolves to that same commit. Nixpkgs, Home Manager and nix-darwin are unchanged.

## Consultation and decisions

Claude Opus 5.5, requested at xhigh effort, independently read the existing
runtime and installation logs. It supported the targeted lock update and
rejected local Homebrew patches, moving package ownership, and a brew-src
override that would disagree with nix-homebrew's embedded version metadata.
No package trimming was justified.

The consultation also identified false success in Homebrew 6.0.1's parallel
bundle installer: `false` was treated as success because it was not `nil`.
The macOS workflow now requires native dependency setup to find WezTerm,
AeroSpace and Herdr already installed by nix-darwin. A second installation
attempt cannot silently stand in for that ownership proof.

The new focused Ruby probe runs the actual Homebrew parser and artifact sorter,
without mocks or installation. It supplements the existing full macOS install
with deterministic coverage of version interpolation and each completion type
arriving before its source app in metadata. `HOMEBREW_DEVELOPER=1` is scoped to
the probe so its developer command does not persist a developer-mode setting.

Reviewed the nix-homebrew upstream delta. Production changes are confined to
the selected Homebrew version and its environment wrapper. Mutable tap ownership,
automatic adoption and explicit AeroSpace tap trust retain their configuration.

## Verification status

- Before: the focused probe fails on real Homebrew 6.0.1 for postflight version
  interpolation and all three completion types. The original macOS install also
  failed in both unchanged main and the PR.
- After: the same probe passes on an isolated copy of the exact Homebrew 7.0.4
  source selected by the new lock. It installs no applications.
- Windows: no Windows installer or configuration file changes are needed.
  Native Windows never consumes nix-homebrew. The original PR's real Windows
  installation and test jobs passed. See the commit-specific results below
  and the current PR checks for combined-change verification.
- Linux/WSL: Home Manager does not import nix-homebrew. Their package inputs
  remain unchanged. See the commit-specific results below and current PR checks.
- Full local checks, real final macOS installation, final Windows/Linux CI and
  independent review remain required before this change is complete.
- A real user's in-place macOS Homebrew upgrade has not been run. Building the
  Nix result does not activate it on the maintainer's Mac.
- Existing release v0.4.4 remains immutable and retains its old lock. This PR
  does not publish a new release or claim to repair that released artifact.

## Sources

- [nix-homebrew update](https://github.com/zhaofengli/nix-homebrew/commit/c11cccfdd36dd69b5323d70d354b2853379c2426)
- [Homebrew completion ordering fix](https://github.com/Homebrew/brew/commit/39470deb274925fec20a749a189a8a6c94bb6157)
- [Homebrew 7.0.4 install-step parser](https://github.com/Homebrew/brew/blob/7.0.4/Library/Homebrew/install_steps.rb)
- [Homebrew 7.0.4 bundle installer](https://github.com/Homebrew/brew/blob/7.0.4/Library/Homebrew/bundle/installer.rb)

## Hosted verification and version metadata follow-up

All 21 PR checks passed at `c63d90b`, including full macOS, Windows and Linux
installations. The macOS log proves Ghostty moved before its completions were
linked, all three declarative packages were already present during native
setup, and every new real runtime probe passed.

That log also exposed an upstream metadata regression: `brew --version`
reported `>=4.3.0 (shallow or no git repository)`. Homebrew 7 moved detection
into `utils/git.sh`, so nix-homebrew's old `brew.sh` substitutions no longer
matched. This occurred in the actual hosted installation as well as the
isolated built runtime, so it was not an artifact of the local test prefix.

Opus 5.5/xhigh confirmed that nix-homebrew's supported `extraEnv` setting can
restore the intended metadata without patching source. The launcher now gets
`HOMEBREW_VERSION` from `nix-homebrew.package.version`. The evaluated test first
binds the resolved brew input revision and hash to nix-homebrew's own lock,
then binds the package and advertised versions to that lock's release tag.
This rejects an input override that would pair different code with a stale
label. Hosted setup additionally checks the real version output.

Before the setting, that evaluated test failed specifically on the missing
advertised version, while source and package identity checks passed. Directly
supplying the selected version to the built runtime returned `Homebrew 7.0.4`.
The evaluated test passes after the setting. In a disposable copy, a real
`nix flake lock --override-input nix-homebrew/brew-src github:Homebrew/brew/6.0.1`
produces the expected source-revision and source-hash failures despite the
unchanged version label. The full macOS Nix build and flake check also pass
with the new setting; neither command activates the maintainer's Mac.
Final verification and independent review cover this additional configuration
change. Remove the explicit setting when nix-homebrew embeds the version
correctly again; no Windows or Linux runtime setting is changed by it.

## Additional correction round authorized by the owner

At `9c3f93e`, all 21 hosted checks passed, including real macOS, Windows and
Linux installations. Full local `make ci`, the Darwin system build and
`nix flake check` also passed on the reviewed files. The hosted Nix logs show
the identity assertions executing on both macOS and Linux, resolving the
reviewer's question about whether those checks had been skipped.

The second independent Opus 5.5 review found no blocking runtime defects,
but reported four low-severity findings and one wording nit. The owner
explicitly authorized one additional correction/review round. These are
the changes and their focused verification, not a claim that an earlier
review approves later edits:

- CI coverage: the existing macOS workflow-contract test now requires the
  package-ownership loop and assertion, the selected-version lookup and
  comparison, and the real Ruby cask probe. Removing each of those five
  statements independently makes that test fail; the intact workflow passes.
  Actual runtime behavior remains covered by the hosted installation and Ruby
  probe, not by these static presence checks.
- Effective package identity: the evaluated test also compares the package's
  store path to the verified input. A disposable source copy overrides only
  `nix-homebrew.package` with real Homebrew 6.0.1 while retaining version 7.0.4.
  The previous test incorrectly accepts it; the new test fails specifically
  at the effective-package assertion. The normal configuration passes. This
  catches package substitution even when its version label remains unchanged.
- Verification wording: earlier pending-status prose now points to the
  commit-specific results and current PR checks. The PR description records
  verification and review outcomes for its current head separately.
- Released installer: README now warns that immutable v0.4.4 retains the
  affected macOS runtime and points test-machine users to the existing explicit
  branch-testing procedure. Its brew-src lock is identical to failing baseline
  `5154aae`; a separate v0.4.4 macOS installation was not run. No release is
  published or changed by this PR.
- Herdr evidence: its review record now limits the mutation claim to the
  argument-count and readability guard actually tested.

This round changes tests and documentation only. Full local and hosted checks
and the independent review must cover the final PR commit. Existing limits
remain explicit: no in-place upgrade of an existing Mac, no macOS/Windows
interactive Herdr proof, and no Accessibility-dependent AeroSpace proof.

## Final upgrade-guide correction

The independent Opus 5.5 review of `6b306a2` confirmed that all five previous
findings were resolved and found no blocking runtime defects. All 21 hosted
checks passed on that commit, including 289 Windows tests with no failures or
skips. One accepted Low remained: this repository's upgrade guide still
presented the affected v0.4.4 macOS migration without the README's warning.

The owner explicitly authorized continuing the review and disregarding the
Helix correction-round limit. This follow-up adds the warning before the
upgrade guide's migration commands, links to the existing README explanation,
and preserves the prohibition on using branch testing for a live v0.1.0
migration. It changes documentation only; release identities and installer
behavior are unchanged.

The review's optional README layout nit is rejected: the warning deliberately
precedes the affected release command, whose comment distinguishes Linux/WSL
from macOS and directs Mac users to that warning. Moving it below the command
would be an editorial preference, not a correction of incorrect instructions.

The final commit still requires the full local gate, hosted checks, and a fresh
independent review. Their exact-commit outcomes belong in the PR metadata and
external verification records; this entry does not pre-approve them. The
existing installation, interactive desktop, and immutable-release limitations
above remain unchanged.

## Release-rendering follow-up

At `0deff99`, the full local gate and all 21 hosted checks passed, including
289 Windows tests with no failures or skips. Opus 5.5 confirmed the upgrade-guide
warning was fixed and the layout-nit rejection was reasonable, but found a new
Medium documentation defect: release preparation globally replaces the current
tag in README and the upgrade guide. It would rename the Homebrew 6.0.1 warning
to the next release even though that release contains the fixed runtime.

The existing release-rendering test now reproduces that error through the real
candidate renderer in a disposable repository copy. Before the correction it
fails because the rendered README attributes the old defect to the candidate.
The correction moves the version-specific advisory into `docs/KNOWN-ISSUES.md`,
outside the current-version rewrite surfaces, and keeps version-neutral links
at all affected install and migration entry points. The test also checks that
the rendered advisory stays byte-identical and both operator guides link to it.
No release-tool behavior or installation behavior changes, and no release is
published. Future releases can retain accurate historical advice without a
one-off removal rule or a manual cleanup prerequisite.

Moving the advisory also restores a neutral exact-release-install heading in
README. The earlier layout decision remains recorded above; this follow-up
addresses the release-rendering defect rather than reopening that optional nit.
Fresh full verification and independent review must cover the resulting commit.

## Maintainer guidance follow-up

At `33597025`, all 21 hosted checks, the full local gate, the clean-commit Darwin
build, and the flake check passed. Opus 5.5 confirmed the release-rendering
finding was resolved and found no runtime defects. Two accepted documentation
items remained: the macOS maintainer runbooks lacked the advisory link, and the
advisory described future releases as always selecting Homebrew 7.0.4.

The macOS VM runbook and manual-test checklist now link to the version-specific
advisory before their installation instructions. A known failure must still be
recorded as a failure; the notice does not waive a test. The advisory now states
which Homebrew version the fix selected, rather than promising the same version
in every future release. These changes affect documentation and its regression
test only.

The optional Windows README heading observation predates this PR and changes
no installation instruction. It is outside this compatibility correction and
is not an unresolved defect introduced by the change.

The release-rendering evidence is also repeated in disposable copies of the
exact before/after commits, recording the command, source identities and each
README/upgrade-guide condition independently. The upgrade-guide assertion now
normalizes Markdown line wrapping before comparison; its original literal
comparison missed the wrapped sentence. Earlier logs remain as history; their
first assertion stopped before demonstrating that condition. The reconstructed
old candidate fails both conditions, and the corrected candidate passes both.
Final verification and review outcomes remain bound to the resulting commit in
the PR metadata and external evidence records.
