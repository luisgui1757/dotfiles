# Canonical gold-standard review — 2026-07-09

- **Scope:** entire repository, adversarial review against an uncompromised
  canonical gold-standard dotfiles project.
- **Baseline:** branch `main` at `6380d8a49d7f9586ea0225116cae2143d5c57d38`,
  clean worktree, no open PRs.
- **Mode:** REVIEW ONLY / read-only. No fixes were applied. This file is the
  review artifact, added on explicit owner request after the review completed.
- **Method:** six specialist line-by-line reviews (POSIX installers, Windows
  installers, test suite, CI/rulesets/Renovate, Nix + chezmoi, docs/hygiene),
  cross-checked against direct source probes; live GitHub state fetched with an
  authenticated `gh`; all runnable local suites executed on the macOS host.
  Every P1 claim was re-verified at source by the orchestrating session.
- **Ledger rule:** this file is append-only history. Fix findings by changing
  code/tests/docs and recording resolutions (here or in a successor dated
  review), not by rewriting findings.

## VERDICT: NOT GOLD-STANDARD READY

**Executive summary.** This repo is unusually close to gold standard — the
three-plane architecture (Nix packages / native+deferred provisioning / chezmoi
configs) is real and enforced at three layers, the 12 required checks exactly
match live GitHub rulesets with zero integrity bypass, all GitHub Actions are
SHA-pinned with least-privilege tokens, the migration/parity gate is a genuine
adversarial oracle, and every local suite passed on review day (static, lint,
shell, migration, nvim, tmux, starship, ghostty, wezterm, aerospace, nix,
Renovate schema). What blocks the verdict is concentrated and enumerable:
**two P1s** — (1) install-failure propagation is broken at both public entry
points (POSIX `install-deps.sh` has no failure aggregation and exits 0 after
warn-and-continue failures; `setup.ps1` discards `install-deps.ps1`'s
carefully-built nonzero exit, including the documented `blocked`-exits-nonzero
contract in `-Update` mode), and (2) the VS Build Tools bootstrapper is
downloaded from a mutable URL and executed — elevated — with zero verification,
and the supply-chain scanner is structurally blind to that pattern — plus a P2
band of status-doc drift from the July Nix wave, two claimed-but-nonexistent /
never-enforced checks, and a handful of provenance/coverage gaps. Nothing found
is destructive, secret-leaking, or a false merge gate.

---

## Findings

### P1

#### DF-CGS-001 [P1] Install-failure propagation is broken at both public entry points (exit 0 with real failures)

- Location: `install-deps.sh:3299` (`pm_install $pkg || echo "  WARN: …
  continuing"`), `install-deps.sh:4120` (unconditional `install-deps: done`),
  `install-deps.sh:1944-1947`, `:2013-2015`, `:1508-1509`, `:1650-1651`,
  `:3453-3455` (`FAIL:`-marker-then-`return 0` sites); `setup.ps1:1458`
  (`& … install-deps.ps1 @depsArgs` — exit never checked), `setup.ps1:1476`
  (`$global:LASTEXITCODE = 0` reset), `setup.ps1:1411` + `:1450-1452`
  (`-Update` runner unchecked, then `exit 0`).
- Wrong behavior / gap: POSIX — every generic catalog install failure (git, rg,
  fd, fzf, lsd, tmux, zsh, cmake, node…) is `WARN:`-and-continue with no
  aggregation, no summary, and final exit 0; five more sites print `FAIL:` then
  continue with exit 0. Windows — `install-deps.ps1` aggregates correctly and
  exits 1 (`Exit-InstallDepsIfFailures`, `install-deps.ps1:3031-3045`), but
  `setup.ps1` discards that exit code in Phase 1 and in `-Update` mode, then
  explicitly resets `LASTEXITCODE`. A local `./setup.sh --all` or
  `.\setup.ps1 -All`/`-Update` can report success with tools missing;
  `setup.ps1 -Update` silently swallows the documented "`blocked` exits
  nonzero" contract everywhere (no CI compensation — e2e never runs `-Update`).
- Proof: `grep -c 'INSTALL_FAILURES\|FAILURES+='` on `install-deps.sh` → 0;
  `Exit-InstallDepsIfFailures` prints `  FAIL: …` + `exit 1`
  (`install-deps.ps1:3031-3045`) — which is exactly why CI's `-All` log grep
  (`e2e-install.yml:266`) compensates on Windows in CI only.
- Source of truth: README/ROADMAP update-mode contract ("`blocked` exits
  nonzero"); CLAUDE.md documents `$InstallFailures` as the Windows contract —
  the sh side has no counterpart.
- Multi-location check: both installers, both setup entry points, both
  `-Update` paths audited; CI `-All` paths are partially compensated by `FAIL:`
  greps + explicit tool asserts; local runs and `-Update` are not.
- Recommended fix: add an `INSTALL_FAILURES` accumulator + end-of-run summary +
  nonzero exit to `install-deps.sh` (mirroring the ps1); in `setup.ps1`, check
  `$LASTEXITCODE` after both `install-deps.ps1` invocations and propagate (keep
  `-BestEffort` as the explicit downgrade).
- Test/doc updates required: shell test for the sh summary/exit;
  `Setup.Tests.ps1` exit-propagation cases (Phase 1 and `-Update`);
  README/CLAUDE failure-semantics paragraphs.
- Confidence: high.

#### DF-CGS-002 [P1] VS Build Tools bootstrapper: unpinned, unverified, executed elevated — and the supply-chain scanner cannot see it

- Location: `install-deps.ps1:33` (`$VsBuildToolsBootstrapperUrl =
  'https://aka.ms/vs/17/release/vs_BuildTools.exe'` — a moving alias), `:2839`
  (`Invoke-WebRequest … -OutFile $installer`), `:2841-2846` (`Start-Process …
  -Wait`, retry `-Verb RunAs`); scanner blind spot:
  `tests/static/supply_chain_remote_execution_test.sh:259-267` flags `& $var`
  execution of downloads but never `Start-Process`.
- Wrong behavior / gap: the repo's own invariant ("Direct network executables
  are pinned and verified before execution; bootstrap scripts acceptable only
  when pinned to an immutable commit and hash-verified") is violated on the
  required `-All` path, with elevation; the static guard that exists to catch
  this class passes green.
- Proof: no `Get-FileHash`/`Get-AuthenticodeSignature` between download and
  `Start-Process`; README's verified-artifact list omits vs_BuildTools;
  CLAUDE.md documents it only as a catalog-rule exception, not a provenance
  exception.
- Source of truth: CLAUDE.md "Direct network executables are pinned and
  verified"; `supply_chain_remote_execution_test.sh` policy header.
- Multi-location check: only occurrence of the pattern repo-wide (Scoop and
  Homebrew bootstraps are pinned+SHA-verified; all other archives are
  SHA-verified).
- Recommended fix: SHA-pinning is impractical for this frequently-reissued
  Microsoft bootstrapper — verify the Authenticode signature (Status `Valid`,
  chain to Microsoft Corporation) before `Start-Process`, fail closed into
  `$InstallFailures` otherwise; extend the scanner to flag `Start-Process` of
  an `Invoke-WebRequest -OutFile` target without intervening verification, and
  allowlist this site once verified.
- Test/doc updates required: Pester case (signature-check gate), scanner
  self-test entry, CLAUDE/README supply-chain sections.
- Confidence: high (mitigation: HTTPS to Microsoft's canonical alias — this is
  why it is P1, not P0).

### P2

#### DF-CGS-003 [P2] Pi CLI integrity is verify-request-A / install-request-B (TOCTOU) on both OSes

- Location: `install-deps.sh:1681-1721`; `install-deps.ps1:2666-2721`.
- Wrong behavior / gap: `npm view … dist.integrity` is compared to the pin,
  then a separate `npm install -g` re-fetches metadata+tarball; the pin never
  constrains the installed artifact; post-checks are version-string only.
- Proof: two independent npm invocations; nothing passes the sha512 to the
  install.
- Source of truth: README "installs … after verifying npm's `dist.integrity`" —
  the mechanism is weaker than the implied guarantee.
- Multi-location check: identical shape in sh and ps1 (design gap, not a port
  bug).
- Recommended fix: `npm pack` → hash the tarball against the sha512 pin →
  `npm install -g <verified-tarball>`, both installers.
- Test/doc updates required: update `InstallDeps.Tests.ps1` + shell Pi test;
  README wording.
- Confidence: high (mechanics), low (exploitability).

#### DF-CGS-004 [P2] zsh-plugin pin verification fails loud but not closed — a tag-moved payload stays installed and gets sourced

- Location: `home/.chezmoiexternal.toml.tmpl:4-16` (clone by tag),
  `home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl:14-19`
  (verify after clone, `exit 1`, no removal), `install-deps.sh:1873-1897`
  (clones into live target, `return 1` without removal), `shells/zshrc:150-161`
  (sources whatever is readable).
- Wrong behavior / gap: on upstream tag compromise, apply/setup fails (good)
  but the attacker's `fzf-tab.plugin.zsh` remains in the root the next
  interactive zsh sources.
- Proof: `tests/migration/oracle_test.sh:120-141` proves the FAIL fires;
  nothing proves quarantine.
- Source of truth: the pin-verification design intent (fail closed).
- Multi-location check: both writers (install-deps + chezmoi external) share
  the flaw.
- Recommended fix: clone to temp → verify commit → move into place
  (install-deps); onchange script renames a mismatched checkout to
  `<name>.quarantine.<ts>` before `exit 1`.
- Test/doc updates required: extend `oracle_test.sh` to assert the payload is
  neutralized; CLAUDE invariant note.
- Confidence: high.

#### DF-CGS-005 [P2] `ghostty +validate-config` is enforced nowhere in CI (everywhere-skipped check)

- Location: `tests/ghostty/validate.sh:6-9` (skip exit 0); `test.yml:96-135`
  (ubuntu job has no ghostty step); `test.yml:155` (macos job installs no
  ghostty → skip).
- Wrong behavior / gap: the only real-parser validation of `ghostty/config`
  runs solely on machines that happen to have ghostty (it did run and pass on
  the review host); an invalid config can merge green.
- Proof: no `test-ghostty` in the ubuntu step list; no ghostty in the macos
  install line; e2e jobs never invoke it.
- Source of truth: audit rule — a check skipped everywhere is not enforced.
- Multi-location check: `scheme_test.sh`/`clipboard_test.sh` (greps) do run in
  CI; only the parser leg is orphaned.
- Recommended fix: run
  `ghostty +validate-config --config-file=ghostty/config` in the
  `setup.sh / macos-15` e2e job (setup `--all` already installs the cask
  there), keeping the local skip.
- Test/doc updates required: e2e step + Makefile help note.
- Confidence: high.

#### DF-CGS-006 [P2] CLAUDE.md claims WSL "bats coverage" that does not exist; CI installs bats for nothing

- Location: `CLAUDE.md:606` ("the existing `DOTFILES_FORCE_OS=wsl` bats
  coverage"); `test.yml:57` and `:155` (bats/bats-core installed).
- Wrong behavior / gap: zero `.bats` files and zero `DOTFILES_FORCE_OS`
  references exist repo-wide — the documented WSL merge-proxy leg is phantom;
  real WSL coverage is the Ubuntu container + `template_test.sh` WSL renders +
  stub-based `wsl_gui_tools_test.sh` + the non-required canary.
- Proof: `find … -name '*.bats'` → 0; `grep -r DOTFILES_FORCE_OS` over code →
  0.
- Source of truth: the codebase.
- Multi-location check: bats appears only in the two CI install lines and
  Makefile skip docs.
- Recommended fix: either add the claimed WSL-mode installer test (as
  `*_test.sh`; the runner cannot even discover `.bats`) or correct CLAUDE.md
  and drop the dead bats installs from both CI jobs.
- Test/doc updates required: CLAUDE.md correction is mandatory either way.
- Confidence: high.

#### DF-CGS-007 [P2] PSScriptAnalyzer gates only the profile; the ~4700 lines of installers are parse-checked but never linted

- Location: `test.ps1:54` (sole PSSA invocation); overstated by
  `e2e-install.yml:272-273` and CLAUDE.md's CI section.
- Wrong behavior / gap: `setup.ps1`, `install-deps.ps1`, `uninstall.ps1`,
  `tmux/psmux-rose-pine.ps1`, and tests get AST parse only
  (`tests/static/ps1_parse.sh`).
- Proof: grep for `Invoke-ScriptAnalyzer` → `test.ps1:54` (profile) and
  `Profile.Tests.ps1:9` (same file) only.
- Source of truth: CLAUDE.md/README "PSScriptAnalyzer runs at Warning,Error"
  framing.
- Multi-location check: no other analyzer invocation repo-wide.
- Recommended fix: extend `Invoke-ScriptAnalyzer` to the full `.ps1` surface
  with a reviewed baseline; fix the doc claims.
- Test/doc updates required: `test.ps1` + CLAUDE/README CI descriptions.
- Confidence: high.

#### DF-CGS-008 [P2] Windows `lsd` config is deployed to a path lsd likely never reads

- Location: `home/.chezmoiignore:16-21` (lsd not Windows-gated) →
  `%USERPROFILE%\.config\lsd\*`; `README.md:302`,
  `docs/MIGRATION_STATUS.md:24`; `shells/powershell_profile.ps1:533-541`
  (plain `lsd`, no `--config-file`).
- Wrong behavior / gap: upstream lsd documents `%APPDATA%\lsd\config.yaml` on
  Windows; if so, `icons.theme`, `color.theme: custom`, and all of
  `colors.yaml` silently do not apply on native Windows while every test stays
  green (names still colored via `LS_COLORS`). Same bug class as the old
  `%APPDATA%` lazygit miss, opposite direction.
- Proof: zero `%APPDATA%\lsd` handling repo-wide; apply test asserts deployment
  only (`tests/migration/windows_apply_test.ps1:403-409`).
- Source of truth: lsd upstream config-path documentation (verify on a live
  Windows host before coding).
- Multi-location check: POSIX paths are correct; only Windows consumption is
  suspect.
- Recommended fix: first verify on a Windows host; then either manage
  `AppData/Roaming/lsd/*` via chezmoi or pass `--config-file` in the profile
  functions; update README/MIGRATION_STATUS and add a consumption assertion.
- Test/doc updates required: apply-test row + README/MIGRATION_STATUS lines.
- Confidence: medium-high.

#### DF-CGS-009 [P2] Interactive zsh on Linux/WSL may never see the Nix-owned CLI set; zshrc doesn't wire the Nix profile PATH

- Location: `shells/zshrc:38` (only PATH edit: `~/.local/bin`);
  `setup.sh:203-213` (knows the four Nix profile dirs — process-local only);
  the WSL canary uses `nix-bin`, whose hook is bash-oriented
  `/etc/profile.d/nix.sh`.
- Wrong behavior / gap: fresh Linux/WSL zsh logins can be missing
  rg/fd/fzf/jq/lazygit/node/starship/zoxide, and every zshrc consumer is
  guard-degraded (`command -v`), so the prompt/fzf-tab/zoxide silently vanish
  rather than error. e2e asserts nix-path resolution inside the setup shell,
  not a fresh interactive zsh.
- Proof: no `nix` reference in `shells/zshrc`.
- Source of truth: Nix profile PATH-wiring requirements per install method;
  invariant 22 (packages-only HM cannot wire zsh session vars).
- Multi-location check: macOS is covered incidentally by nix-darwin's
  `/etc/zsh*` management (see DF-CGS-026); Linux/WSL is the exposed leg.
- Recommended fix: defensively prepend the four Nix profile `bin` dirs in
  `shells/zshrc` (existence-guarded; inert without Nix).
- Test/doc updates required: `zsh_startup_test`/`zsh_plugins_test` addition;
  README platform note.
- Confidence: medium (in-repo mechanism verified; distro zsh-startup wiring not
  live-tested in this review).

#### DF-CGS-010 [P2] A Renovate runner-image bump renames matrix-derived required contexts and deadlocks its own PR; no alignment guard exists

- Location: `e2e-install.yml:62` (`setup.sh / ${{ matrix.os }}`),
  `nix.yml:21-26`; `.github/rulesets/main-integrity.json` contexts;
  `renovate.json:404-409` (runner-image bumps are explicitly proposed).
- Wrong behavior / gap: `macos-15 → macos-16` makes the PR emit
  `setup.sh / macos-16` while the no-bypass integrity ruleset still requires
  `setup.sh / macos-15` — unmergeable until the owner pre-applies edited
  rulesets across 4 mirrors (ruleset JSON, `settings.yml`,
  `apply-repo-safeguards.sh` ×2 places, the runbook).
- Proof: required contexts embed matrix values; no test asserts ruleset
  contexts ⊆ workflow-produced job names or 4-mirror agreement.
- Source of truth: GitHub check-context naming semantics + the ruleset payloads.
- Multi-location check: `setup.ps1 / windows-2025` and
  `e2e containers / ubuntu-24.04` use literal names decoupled from `runs-on`
  (only misleading names on bump, not deadlocks).
- Recommended fix: static test asserting context/producer alignment and mirror
  agreement; document the pre-apply ordering in
  `docs/security/branch-protection.md`.
- Test/doc updates required: extend `tests/static/required_checks_test.sh`;
  runbook section.
- Confidence: high.

#### DF-CGS-011 [P2] Renovate coverage gaps: no matcher-coverage guard; pylatexenc/setuptools and the Scoop installer commit unmanaged; several pin mirrors unguarded

- Location: `renovate.json` (all 32 matchStrings verified matching on review
  day — nothing keeps that true); `install-deps.sh:83-86` ↔
  `install-deps.ps1:34-37` (pylatexenc/setuptools: no manager, no
  `pin_consistency` row); `install-deps.ps1:26-27` (`$ScoopInstallerCommit`, no
  git-refs manager unlike its Homebrew/cargo-binstall siblings); Hack font
  sh↔ps1 mirror and Pi version in `e2e-install.yml:185,363` missing from
  `tests/static/pin_consistency_test.sh`; `CLAUDE.md:683-684` validator example
  already drifted (`node@24.11.0`/`renovate@43.230.1` vs the script's
  `24.18.0`/`43.256.0` — live proof of the drift class).
- Wrong behavior / gap: regex managers require exact version/SHA adjacency; one
  inserted comment line silently disables a manager (no PRs, no error);
  uncovered pins age silently; unguarded mirrors strand on bumps.
- Proof: local simulation of every matchString against its target file; grep of
  `pin_consistency_test.sh` for the missing rows.
- Source of truth: `renovate.json` + the pinned constants inventory.
- Multi-location check: full pin inventory cross-checked across sh/ps1/CI/docs.
- Recommended fix: static test executing each customManager matchString against
  its file and failing on zero matches; add the two managers; add the
  pin_consistency rows; replace CLAUDE's literals with a pointer to
  `scripts/validate-renovate.sh`.
- Test/doc updates required: new static test + pin rows + CLAUDE edit.
- Confidence: high.

#### DF-CGS-012 [P2] Required ubuntu CI installs an unverified Microsoft repo-config deb with sudo

- Location: `test.yml:82-84` (`curl … packages-microsoft-prod.deb` →
  `sudo dpkg -i`, no checksum); lesser siblings: unpinned `pip install yamllint
  editorconfig-checker` (`:76`), `choco install neovim` (`:221`),
  cargo-binstall-fetched binaries (`:75` — the installer script is verified,
  the fetched binaries are not).
- Wrong behavior / gap: weakest supply-chain link in a required gate for a repo
  whose policy is pin+verify. Bounded blast radius: read-only token, zero
  secrets — CI-result integrity only.
- Proof: no `sha256sum -c` between the curl and the dpkg.
- Source of truth: repo supply-chain invariant (scoped to "direct GitHub
  downloads", so this is technically out of contract — the gold-standard bar is
  what flags it).
- Multi-location check: only sudo-install of an unverified remote artifact in
  the workflows.
- Recommended fix: pin + SHA-verify the MS deb (it changes rarely), or install
  pwsh from a pinned GitHub release like nvim/starship.
- Test/doc updates required: workflow edit; optionally extend the supply-chain
  scanner to workflow files for this pattern.
- Confidence: high (fact), medium (severity).

#### DF-CGS-013 [P2] e2e caches with prefix restore-keys erode the macOS/Windows fresh-install proof

- Location: `e2e-install.yml:77-90`, `:239-248` (lazy/mason/parser/scoop/brew
  caches; `restore-keys` prefix fallback).
- Wrong behavior / gap: a broken fresh-download path for an unchanged
  plugin/Mason tool can stay green indefinitely on hosted jobs — the nightly
  schedule shares the same cache scope, so it is not cache-free either; the
  Ubuntu container is the only true clean-image proof (Linux/apt only).
- Proof: prefix restore-keys always restore the newest prior archive even when
  the lockfile hash changes.
- Source of truth: the e2e jobs' stated purpose ("real install guarantee",
  "fresh hosted runners").
- Multi-location check: both cache steps; the pinned-cache-major-in-key rule
  itself holds (verified against `repo_policy_test.sh:124-139`).
- Recommended fix: accept as a documented speed tradeoff plus a periodic
  cache-buster (e.g. monthly `workflow_dispatch` run with caches disabled or a
  date-segmented key), recorded in the greenfield ledger.
- Test/doc updates required: workflow input/step; README CI section sentence.
- Confidence: high.

#### DF-CGS-014 [P2] Status-doc drift wave from the July Nix migration (six surfaces)

- Location and gaps:
  - `docs/security/branch-protection.md:61-65` — says required checks "include
    exactly" 10 contexts; the canonical payloads require 12 (both
    `nix flake check` contexts missing) and no test guards this runbook.
  - `README.md:578` — "gated by two workflows" (three; README itself cites
    nix.yml at `:171`).
  - `ROADMAP.md:58,106-110` — still headers the shipped Nix migration "in
    progress"/"are being delivered" (merged as #44/#45), violating its own
    flip-to-DONE rule at `:231-232`.
  - `docs/MIGRATION_STATUS.md:46-47` — "the 5 pinned binary/font installers"
    (now ~12); the config table lacks gh-dash / wezterm / aerospace /
    `psmux.conf` / rose-pine-conf rows.
  - `WINDOWS_THEME_FIXES.md:3-4,167` — self-expired root-level handoff doc
    ("Delete this file once the fixes land") whose Fix 3 recommends
    `terminal-features` in the psmux overlay, now explicitly banned by
    invariant + `windows_conf_test.sh`.
  - `docs/plans/ai-cli-integration-roadmap.md:3-8,219` — live-looking "PLANNED"
    doc mandating "NO Git-Bash, NO cygpath", the exact opposite of the shipped,
    CLAUDE-mandated Sentinel mechanism.
- Proof: `.github/rulesets/main-integrity.json` context list; git log
  (#42-#45); `tmux/tmux.windows.conf:11-12`; `setup.sh:42-44` vs the plan's
  stale pins.
- Source of truth: rulesets, workflows, merged PR history, shipped code.
- Multi-location check: pins/flags/mechanics elsewhere in README/CLAUDE are in
  exact doc-code sync (the drift is concentrated in status/security prose no
  test guards).
- Recommended fix: one docs PR — update the runbook to 12 contexts (or point at
  the ruleset JSON as canonical), "three workflows", flip ROADMAP headers,
  refresh MIGRATION_STATUS, archive `WINDOWS_THEME_FIXES.md` and the ai-cli
  plan under `docs/archive/` with superseded banners.
- Test/doc updates required: the docs themselves; optionally a pin row for the
  runbook context list.
- Confidence: high.

#### DF-CGS-015 [P2] `setup.ps1`'s chezmoi-verify helper lacks the promotion hardening its uninstall twin documents — a drifted pre-existing target can crash the backup scan

- Location: `setup.ps1:793-800` (no try/catch) vs `uninstall.ps1:137-147`
  (documented hazard: `$PSNativeCommandUseErrorActionPreference` promotes
  verify's normal nonzero "drifted" exit into a terminating throw); e2e sets
  that preference (`e2e-install.yml:253`) but fresh runners have no
  pre-existing targets, so it never fires in CI.
- Wrong behavior / gap: exactly the upgrade-from-pre-chezmoi scenario README
  advertises (`README.md:81-99`) can crash instead of backing up.
- Proof: helper comparison; preference propagation traced.
- Source of truth: uninstall.ps1's own documented rationale for the guard.
- Multi-location check: `Invoke-ChezmoiOrExit`/`Invoke-ChezmoiOutput` share the
  exposure (degrades diagnostics only — still fail-closed).
- Recommended fix: wrap the verify call like `uninstall.ps1`, or toggle the
  preference inside the chezmoi helpers as the Sentinel wrappers already do.
- Test/doc updates required: `Setup.Tests.ps1` case with a drifted target under
  the strict preference.
- Confidence: high (mechanism), medium (field frequency).

#### DF-CGS-016 [P2] Windows tree-sitter CLI is unpinned while the repo pins v0.26.10 elsewhere specifically for parser-ABI reproducibility

- Location: `install-deps.ps1:322` (scoop manifest floats), `:2630`
  (`npm install -g tree-sitter-cli`, no version).
- Wrong behavior / gap: a 0.27 release changes `generate` output on Windows
  only — reintroducing the E5113 drift class invariant 19 exists to prevent.
- Proof: no version constraint on either Windows install arm; Linux pins
  `TREE_SITTER_CLI_LINUX_VERSION=v0.26.10` with SHA.
- Source of truth: CLAUDE.md Nix section ("pinned to v0.26.10 precisely to keep
  that build reproducible").
- Multi-location check: macOS uses the Homebrew `tree-sitter-cli` formula
  (floats too, but Homebrew-mediated — accepted class); Windows is the
  unpinned direct gap.
- Recommended fix: pin the npm fallback (`tree-sitter-cli@0.26.10`) or mirror
  the Linux pinned-release zip install on Windows; add a pin mirror row.
- Test/doc updates required: Pester case + pin_consistency row + CLAUDE note.
- Confidence: high (gap), medium (timing).

#### DF-CGS-017 [P2] `test.ps1`'s Pester gate can pass when a spec container fails discovery; Windows font failures are mostly unrecorded

- Location: `test.ps1:70-73` (checks only `FailedCount -gt 0` — a Pester 5
  discovery failure yields `FailedCount 0`, `Result 'Failed'`);
  `install-deps.ps1:1822-1824` (Hack font generic catch → warning only, no
  `$InstallFailures`, no `FAIL:`); the Windows e2e job never asserts the font.
- Wrong behavior / gap: a spec file that throws during discovery passes the
  required Windows gate with its tests never executed; `-All` can exit 0 with
  the font absent (only the checksum-mismatch path records a failure,
  `install-deps.ps1:1796-1799`).
- Proof: Pester 5 result semantics; catch-block inspection;
  `e2e-install.yml:347-364` assertion list (no font).
- Source of truth: Pester 5 documented result model; the `$InstallFailures`
  contract.
- Multi-location check: `tests/nvim/run.ps1` aggregates real per-spec exit
  codes correctly (not affected).
- Recommended fix: also require `$result.Result -eq 'Passed'` and nonzero
  `TotalCount` (or `Run.Throw`); record font failures in `$InstallFailures`;
  add a font-presence assert to the Windows e2e job.
- Test/doc updates required: `test.ps1`, `InstallDeps.Tests.ps1` catch case,
  e2e step.
- Confidence: medium-high / high.

#### DF-CGS-018 [P2] `--dry-run` on a brew-less macOS aborts mid-preview with exit 1

- Location: `install-deps.sh:397-401` (dry-run brew arm `return 1`),
  `:4003-4019` (`PM="unknown"` → `exit 1`); `setup.sh:25,864` (`set -e` kills
  the rest of the preview).
- Wrong behavior / gap: on the primary fresh-macOS path, dry-run prints the
  brew "would" lines then declares "no supported package manager found" and
  exits 1, so Phases 2-6 never preview — violating "preview every step"
  (`setup.sh:8`). Contrast the missing-Nix dry-run pattern that previews the
  failure and continues (`setup.sh:717-720`).
- Proof: code path is unconditional under `DRY_RUN=1` with no brew present.
- Source of truth: setup.sh's own dry-run contract and the missing-Nix
  precedent.
- Multi-location check: CI dogfoods dry-run only on runners that already have
  brew, so the branch is never exercised there.
- Recommended fix: in dry-run, treat brew as "would exist" (keep `PM=brew` for
  preview) with a "real run would prompt/bootstrap" note.
- Test/doc updates required: shell dry-run case on a stubbed brew-less Darwin.
- Confidence: high.

### P3

#### DF-CGS-019 [P3] Container e2e tool probes are presence-only

- Location: `tests/ci/container-e2e.sh:77-79`.
- Gap: 13 of 14 tools get `command -v` only; a corrupt/wrong-arch pinned
  artifact passes (only nvim is executed).
- Fix: cheap `--version` probe per tool (as `tests/wsl/e2e.sh:84` does).
- Tests/docs: container-e2e edit only. Confidence: high.

#### DF-CGS-020 [P3] WezTerm/AeroSpace configs never validated by their real binaries

- Location: `tests/wezterm/wezterm_smoke.lua` (hand-stubbed `wezterm` module);
  `tests/aerospace/keymap_test.sh` (grep-only); e2e asserts app presence only.
- Gap: a config key the stub tolerates but real WezTerm rejects, or an
  AeroSpace semantic error, passes every gate; only `tests/MANUAL.md` covers it
  by eye.
- Fix: config-load probe (`wezterm --config-file … ls-fonts`) in e2e; AeroSpace
  config check on the mac job if available. Confidence: medium-high.

#### DF-CGS-021 [P3] WSL2 canary failures are invisible

- Location: `e2e-install.yml:417` (job-level `continue-on-error: true`).
- Gap: the nightly scheduled run reports success even when the canary fails, so
  the "nightly signal" decays silently.
- Fix: keep the canary nightly + dispatch, non-required, off PRs; drop
  `continue-on-error` so scheduled failures notify (cannot block merges). See
  Automation Recommendations. Confidence: high.

#### DF-CGS-022 [P3] gh-dash is the only network executable installed from a mutable tag with no commit/hash pairing

- Location: `install-deps.sh:2006`; `install-deps.ps1:2973`; post-checks are
  version-text only.
- Fix: verify the tag's commit ID (pattern already used for zsh plugins) or
  record as a documented exception. Confidence: high/medium.

#### DF-CGS-023 [P3] Fatal-vs-continue is inconsistent across sibling installers

- Location: WezTerm `.deb` failure aborts all later phases
  (`install-deps.sh:3538` → `:4059`) while a failed ripgrep is silent success
  (`:3299`); Ghostty's identical shape is warn-continue (`:3453-3455`).
- Fix: adopting DF-CGS-001's accumulator lets optional tools degrade to
  recorded-failure-continue uniformly while still exiting nonzero.
  Confidence: high (behavior), medium (desirability).

#### DF-CGS-024 [P3] Sentinel clone failure leaks temp dirs into the persistent cache root

- Location: `setup.sh:329-333` — RETURN trap does not fire when `set -e` exits
  the command-substitution subshell; `~/.local/share/dotfiles/sentinel/.tmp.*`
  accumulates (never OS-cleaned, unlike `/tmp`).
- Fix: sweep `.tmp.*` at entry of `ensure_sentinel_checkout` or EXIT-trap a
  subshell scope. Confidence: high.

#### DF-CGS-025 [P3] Assorted verified code/test nits

- `tests/shell/lint.sh:22` — `shellcheck … zshrc || true` can never fail
  (decorative).
- `tests/nvim/spec/leader_spec.lua:29-31` — runtime assert re-reads what the
  harness set (tautology; the static ordering half is the real guard).
- `tests/static/invariants_test.sh:153-162` — grep exit 2 (missing file) passes
  the `+Lazy! sync` ban; pre-assert the listed paths exist.
- `tests/static/json_lint.sh:58-60` — unquoted `$json_files` loop; a
  space-containing path would be skipped/mangled.
- `uninstall.sh:107-117` — "newest" backup picked by mtime, not the sortable
  name timestamp (`mv` preserves mtime).
- `install-deps.sh:192` — unchecked `eval "$(brew shellenv)"` (setup.sh:189-198
  has the checked form).
- `install-deps.ps1:1504` — dead branch retains the CLAUDE-banned "present, but
  Scoop does not manage" wording.
- `install-deps.ps1:2482` — `winget install psmux` fuzzy match (no `--id -e`).
- `install-deps.ps1:2270-2280` — VS Code clean-JSON merge path writes with no
  backup; the JSONC backup lacks the collision counter; neither matches the
  "shared unique_backup_path helper" CLAUDE describes.
- Fixes as stated per item. Confidence: high.

#### DF-CGS-026 [P3] Invariant-22 static guard has known bypass shapes; nix-darwin's `/etc/zsh*` nuance is undocumented

- Location: `tests/static/nix_architecture_test.sh:90-96` misses nested
  attrsets (`home = { file = … }`), external `imports`, and unbanned nix-darwin
  config-writing surfaces (`environment.etc`, `system.defaults`, `launchd.*`,
  `system.activationScripts`). Separately, nix-darwin's default `programs.zsh`
  likely manages `/etc/zshenv`/`/etc/zshrc` on macOS (load-bearing for Nix PATH
  there), while the guard would block declaring `programs.zsh.enable`
  explicitly and "Nix owns NO config file" does not mention the `/etc` level.
- Fix: runtime eval assertion (declared `home.file` attr names ⊆ HM-internal
  set) to close nested/imported holes; extend the static ban list; scope the
  programs-allowlist to `nix/home/**`; verify `programs.zsh.enable` on the
  proving host and document the `/etc` nuance under invariant 22.
- Confidence: high (regex behavior), low-medium (the `programs.zsh` default —
  unverified this review).

#### DF-CGS-027 [P3] chezmoi-external pin bump strands bare `chezmoi apply` until setup re-pins

- Location: `home/.chezmoiexternal.toml.tmpl:7,14` (`refreshPeriod = "0"`,
  shallow detached-at-tag clones).
- Gap: after a legit pin bump, the onchange verifier fails until install-deps'
  self-heal (`install-deps.sh:1877-1890`) runs; the documented setup flow
  recovers (Phase 1 precedes Phase 2), the documented bare-`chezmoi apply`
  workflow strands.
- Fix: teach the onchange script the same fetch+checkout heal, or document the
  recovery. Confidence: high.

#### DF-CGS-028 [P3] Windows profile target assumes non-redirected Documents

- Location: `home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1` →
  `%USERPROFILE%\Documents\PowerShell\…`.
- Gap: with OneDrive Documents redirection, `$PROFILE` resolves under
  `OneDrive\Documents\…`, so the chezmoi copy is a dead file; undocumented.
- Fix: setup.ps1 post-apply probe comparing `$PROFILE` to the managed path
  (warn on mismatch) + README note. Confidence: medium.

#### DF-CGS-029 [P3] Remaining doc/hygiene polish

- README omits `--best-effort`/`--skip-deps`/`--skip-nvim` and uninstall
  `--force-externals` flags; its "What Setup Does" diagram omits the enforced
  pre-Phase-1 Nix layer; repo-layout trees in README/CLAUDE omit `nix/`,
  `wezterm/`, `aerospace/`, `lazygit/`, `scripts/`, etc.
- `.gitignore` covers `.claude/` but not `.codex/`/`.pi/` (CLAUDE says all
  three stay untracked).
- `docs/plans/treesitter-toolchain-handoff.md:5` says "five phases" (six).
- e2e `ubuntu:24.04` is tag-pinned, not digest-pinned.
- ROADMAP/MIGRATION_STATUS dated 10-context lists deserve a one-line addendum
  for the two nix contexts added 2026-07-08.
- Distro claims beyond the tested matrix (Alpine/dnf/zypper/pacman) carry no
  "unit-tested, not CI-proven" qualifier.
- setup phase ordering is only presence-asserted in e2e
  (`e2e-install.yml:107-110`); assert marker order.
- `tests/greenfield/LEDGER.md` has a single bootstrap entry across the whole
  #39-#45 wave — the evidence process was never exercised.
- README troubleshooting has no row for the open VS Code KNOWN ISSUE, and the
  self-link/clone-anywhere guidance is CLAUDE-only.
- Fixes: fold into the DF-CGS-014 docs PR + one static tweak each.
  Confidence: high.

---

## No-Issue Areas

- **Branch protection / merge gate**: live rulesets fetched and diffed —
  byte-equivalent to the checked-in payloads; integrity has zero bypass actors,
  strict 12/12 contexts all produced on every PR (no path filters, no `if:` on
  required jobs); squash-only + no auto-merge + secret-scanning/push-protection
  enabled live; `apply-repo-safeguards.sh` fails closed on duplicates and
  self-verifies.
- **CI hygiene**: all 17 `uses:` full-SHA-pinned with Renovate digest
  maintenance; `permissions: contents: read` everywhere; zero secrets;
  concurrency + timeouts on every job; no `pull_request_target`.
- **Update mode (both OSes)**: evidence-first ownership proofs, fail-closed
  `blocked`, no blanket upgrades, `owner=nix` reported without ever shelling to
  `nix` — the strongest subsystem in the repo, with ~30 adversarial regression
  cases.
- **Supply-chain posture overall**: every other download pin+SHA-verified
  before use; no `curl | sh` / `Invoke-Expression` anywhere (byte-scanned);
  the Sentinel pipeline (hostile-git-config isolation, tag-peel + VERSION +
  clean-worktree asserts) is genuinely gold-standard; the scanner self-tests
  its own regexes.
- **Migration/parity gate**: an actual oracle — per-OS symlink+byte parity,
  wrong-OS absence, idempotent re-apply, adversarial pin corruption,
  blank-stdin WT merge, uninstall data-loss scenarios on both platforms; the
  manifest covers every user-visible surface.
- **Test infrastructure**: all eight `run_all.sh` runners aggregate failures
  correctly; skips are visible, bounded, and (with the two flagged exceptions)
  have at least one CI leg where they cannot skip; tests are hermetic
  (sandboxed HOME everywhere it matters).
- **Nvim suite**: startup budget measured correctly (isolated XDG, precloned
  locked plugins, zero parser builds, best-of-3); the Tier-1/Tier-2 split makes
  language-surface breakage a required-check failure.
- **Nix layer**: packages-only verified at source/static/eval levels;
  flake.lock fresh (all inputs < 30 days at review); no Nix/chezmoi path
  co-ownership; neovim/tree-sitter deferral documented with a negative
  assertion.
- **Public-repo hygiene**: no tracked `.DS_Store`/junk; no tokens/keys/personal
  paths in tracked content or history (pickaxe-scanned); `AGENTS.md` a true
  thin pointer; MIT license sane. History still contains the pre-purge
  `claude/` files and archived codename mentions handled by the separate
  sanitized clone — present as expected, nothing new leaked.

## Automation Recommendations

1. **WSL2 canary — KEEP, but drop `continue-on-error`**
   (`e2e-install.yml:417`). Keep it nightly + dispatch, non-required, off PRs
   (correct as designed); job-level `continue-on-error` makes a genuine WSL
   regression produce a green scheduled run nobody is notified about — worse
   than an occasionally-red nightly. Removing it cannot block merges. If flake
   noise proves excessive, reinstate with an explicit failure-summary step
   instead. Do not add it to required checks.
2. Add the ghostty parser-validation leg to the macOS e2e job (DF-CGS-005) and
   a `wezterm` config-load probe to all three e2e jobs (DF-CGS-020).
3. Extend PSSA to the full `.ps1` surface and harden the Pester result check
   (DF-CGS-007 / DF-CGS-017).
4. Add static guards: required-context ↔ workflow-name alignment + 4-mirror
   agreement (DF-CGS-010); Renovate matchString coverage (DF-CGS-011); e2e
   phase-marker *order* assertion; container `--version` probes (DF-CGS-019).
5. Add a monthly cache-free e2e dispatch (DF-CGS-013) and record runs in the
   greenfield ledger — which should also start receiving the real Sandbox/VM
   entries it was created for.
6. Either implement or delete the claimed WSL bats coverage; drop the dead bats
   installs (DF-CGS-006).
7. Keep the greenfield harnesses (Sandbox/WSL/macOS VM) manual — they model
   TCC/desktop/virtualization surfaces CI cannot; automating them would create
   perpetual flake, not proof.

## Renovate Recommendations

- **Add**: a `git-refs` digest manager for `ScoopInstaller/Install` (same shape
  as Homebrew/cargo-binstall); pypi regex managers for `pylatexenc` +
  `setuptools` (hashes stay human-reviewed); pin_consistency rows for the Hack
  font sh↔ps1 mirror, the Pi version in `e2e-install.yml`, pylatexenc sh↔ps1,
  and the validate-renovate ↔ CLAUDE.md mirror (or drop the literals from the
  doc); a matcher-coverage static test so refactors cannot silently disable
  managers.
- **Verify live**: whether the `nix` manager actually proposes updates for all
  7 flake inputs (nix-darwin, home-manager, nix-homebrew, the three taps) on
  the Dependency Dashboard — historically it covered only `nixpkgs`; if
  partial, either enable `lockFileMaintenance` for nix or document the taps as
  deliberately manual.
- **Consider**: `git-refs` digest managers for the TPM/tmux/psmux plugin
  commits (the repo already accepts that pattern), or one sentence in
  `renovate.json`'s description recording them as deliberately manual; docker
  digest pinning for `ubuntu:24.04`.
- **Deliberately leave manual** (correctly so today): `lazy-lock.json` plugins
  (Lazy-refreshed, behavior-tested), Mason tools (no lockfile by design),
  Sentinel tag/commit/VERSION (must move together), VS BuildTools (unpinnable
  alias — the fix is signature verification, not Renovate), PSFzf (PSGallery
  floating, documented), and all adjacent SHA-256/commit constants
  (human-reviewed by policy — the red-until-recomputed contract demonstrably
  works: the 2026-07-09 constants PR went red, then green after human
  recompute).

## Containerization Recommendation

**No additional containerization.** The single Ubuntu container is correctly
scoped as the only clean-image native-apt proof and should stay. A devcontainer
would be actively wrong for this repo: the product *is* host-level provisioning
(login shells, fonts, GUI apps, symlink privileges, package managers) — a
devcontainer models none of it and adds a maintenance surface that would drift.
macOS cannot be containerized; Windows containers do not model the
Scoop/Developer-Mode/font/WT desktop surface — hosted runners remain the right
representative fixtures, with Windows Sandbox / throwaway WSL / macOS VM as the
manual greenfield tier (already well-built). The one genuine gap containers
cannot fix — cache-masked fresh installs on macOS/Windows — is addressed by the
periodic cache-free run above.

## Verification Performed (2026-07-09, macOS host)

- Git/GitHub state: branch `main` @ `6380d8a`, clean worktree (re-confirmed
  clean after all tests); `gh` authenticated; live rulesets (3, full JSON),
  merge settings, security_and_analysis, 12 recent runs, 0 open PRs fetched.
- Local gates, all **PASS**: `make lint`, `make test-static`,
  `make test-shell`, `make test-migration`, `make test-tmux`,
  `make test-starship`, `make test-ghostty` (including a real
  `ghostty +validate-config` — binary present on this host),
  `make test-wezterm`, `make test-aerospace`, `make test-nix`,
  `make test-nvim` (exit 0, every spec file `Failed: 0`),
  `scripts/validate-renovate.sh` (schema valid under pinned Node 24).
- History scan: pickaxe for purged-PII markers and token/key patterns (no
  secrets; known pre-purge `claude/` history and archived mentions present as
  expected).
- Source re-verification of every P1 and the load-bearing P2 claims (exact
  lines quoted in findings).
- Six specialist line-by-line reviews, cross-checked against each other and
  against direct probes.
- Artifacts created by testing: generated caches under `tests/.cache/`
  (gitignored by design) and npm caches in temp — nothing in the tracked tree;
  worktree verified clean afterward.

## Verification Not Performed

- No real installs/uninstalls/`chezmoi apply` executed (REVIEW mode) —
  installer behavior verified from source plus existing green CI evidence (the
  nightly e2e was mid-run during review).
- No Windows or WSL host: all `.ps1`/psmux behavior is source-level;
  `test.ps1`/Pester not executed; the lsd read-path (DF-CGS-008) and
  OneDrive-redirect (DF-CGS-028) findings carry verify-on-Windows caveats.
- `nix flake check`/`nix eval` not run (would fetch inputs); flake facts from
  `flake.lock` (jq) + CI evidence; the `programs.zsh.enable` default
  (DF-CGS-026) unverified.
- Live Renovate Dependency Dashboard not inspected (nix-manager input coverage
  unverified).
- No greenfield VM/Sandbox run; startup-budget numbers accepted from the
  passing spec, not independently benchmarked.

## Prioritized Roadmap

1. **Fix failure propagation at both entry points** (DF-CGS-001):
   `install-deps.sh` accumulator/summary/exit-1; `setup.ps1` `$LASTEXITCODE`
   checks after Phase 1 and `-Update`; regression tests in `tests/shell/` +
   `Setup.Tests.ps1`; README/CLAUDE failure-semantics text. Fold DF-CGS-023
   (fatal-vs-continue taxonomy) and the font-recording half of DF-CGS-017 into
   the same design.
2. **Close the VS BuildTools hole** (DF-CGS-002): Authenticode verification
   before `Start-Process` + scanner extension + Pester case + supply-chain doc
   update.
3. **Docs-truth PR** (DF-CGS-014 + DF-CGS-029): branch-protection runbook to 12
   contexts, README "three workflows"/flags/diagram/layout, ROADMAP headers to
   DONE, MIGRATION_STATUS refresh, archive `WINDOWS_THEME_FIXES.md` + the
   ai-cli plan, fix the CLAUDE validator example and the phantom-bats sentence,
   `.gitignore` `.codex/`/`.pi/`.
4. **Enforce the never-enforced checks** (DF-CGS-005/-006/-007/-017): ghostty
   validate in macOS e2e, WSL bats decision, PSSA full surface, Pester result
   hardening.
5. **Supply-chain P2s** (DF-CGS-003/-004/-016/-022): Pi `npm pack`+hash install
   on both OSes; zsh-plugin quarantine-on-mismatch (+ oracle test); pin the
   Windows tree-sitter CLI; pair the gh-dash tag with its commit.
6. **CI robustness batch** (DF-CGS-010/-011/-012/-013/-019/-021):
   context-alignment test, Renovate matcher-coverage test + new managers + pin
   rows, MS deb pinning, monthly cache-free e2e, container `--version` probes,
   canary `continue-on-error` removal.
7. **Platform correctness follow-ups**
   (DF-CGS-008/-009/-015/-018/-024/-026/-027/-028): verify-then-fix Windows lsd
   path, zshrc Nix PATH, `Test-ChezmoiVerify` hardening, macOS dry-run preview,
   Sentinel temp sweep, invariant-22 guard tightening, external-bump self-heal,
   `$PROFILE` redirect probe.

## Gold-standard scorecard

| Area | Score | Justification |
|---|---:|---|
| Architecture coherence | 4.5 | One product, three cleanly-enforced planes, complete ownership map; only the `/etc`-level nix nuance and one documented dual-writer path. |
| Setup/install reliability | 3.5 | Four e2e-proven paths, but exit-0-with-failures locally on both OSes, the `-Update` swallow, and the macOS dry-run abort hole the entry-point contract. |
| Cross-platform coverage | 4 | Ubuntu container + hosted mac/win + honest best-effort WSL; Alpine/Arch/dnf/zypper are stub-tested only; Windows lsd consumption gap. |
| Test quality | 4.5 | Rigorous, hermetic, adversarial, self-testing scanners; docked for two never-enforced/phantom checks and presence-only container probes. |
| CI/automation quality | 4 | 12/12 required contexts, SHA-pinned, least-privilege; rename-deadlock trap, cache erosion, one unverified sudo deb. |
| Renovate/dependency maintenance | 4 | 32 verified-matching managers, sane grouping/policy; missing coverage guard and three uncovered pin surfaces. |
| Supply-chain hardening | 4 | Near-exemplary pin+verify discipline and scanner; one unverified elevated execution + Pi TOCTOU + gh-dash tag-only keep it from 5. |
| Documentation accuracy | 3.5 | Pins/mechanics in exact doc-code sync (guarded); status/security prose drifted through the Nix wave, incl. one claimed-nonexistent test. |
| Maintainability/simplicity | 4 | Large but disciplined; invariants + rejected tradeoffs recorded; some dead code and root-level clutter. |
| Maintenance-mode readiness | 4 | Renovate + drift guards + strong gates mean it can coast; needs roadmap items 1-3 before "walk away safe". |

**Bottom line:** the distance to gold standard is two P1 fixes, one docs PR,
and a bounded set of guard additions — all enumerated above; nothing structural
needs to change.
