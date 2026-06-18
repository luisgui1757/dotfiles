# Dotfiles Roadmap

Last audited: 2026-06-18 on branch `audit/full-roadmap-review-2026-06-18`.
Baseline: `main` at `c3c22b7`.

This is the adversarial post-merge roadmap for the chezmoi migration and the
current setup/CI surface. The goal is not "good enough"; the repo should have a
single obvious setup path, enforce the tests it claims are required, avoid
mutable supply-chain execution where practical, and make greenfield evidence
repeatable instead of tribal.

## Audit Evidence

- Local `make test-static`: passed before dynamic tests populated generated
  caches.
- Local `make test`: passed.
- Local `make validate-renovate`: passed.
- Local migration checks passed when run directly:
  `template_test.sh`, `parity_gate.sh`, `greenfield_roundtrip.sh`,
  `uninstall_safety_test.sh`, `windows_render_test.sh`, and `oracle_test.sh`.
- After `make test` populated `tests/.cache`, the `editorconfig-checker` phase of
  `make test-static` produced no diagnostics and was stopped after repeated
  timeouts. `editorconfig-checker ROADMAP.md` and a no-index whitespace check for
  this roadmap both passed, so this is a generated-cache sweep problem rather
  than a roadmap formatting problem.
- Live GitHub protection was checked with `gh api`. The active integrity
  ruleset and classic branch-protection fallback currently require only:
  `ubuntu`, `macos`, `windows`, `e2e containers / ubuntu-24.04`,
  `setup.sh / ubuntu-24.04`, `setup.sh / macos-15`, and
  `setup.ps1 / windows-2025`.
- PR #23 did run the three `chezmoi-parity*` jobs successfully before merge, but
  those contexts are not currently part of the live required-check contract.

## P0 - Required Gate Reality

### 1. Live `main` protection does not require the `chezmoi-parity*` jobs

Status: open.

Evidence:

- `.github/workflows/test.yml:213` defines `chezmoi-parity`.
- `.github/workflows/test.yml:256` defines `chezmoi-parity-macos`.
- `.github/workflows/test.yml:292` defines `chezmoi-parity-windows`.
- `.github/rulesets/main-integrity.json:42` through
  `.github/rulesets/main-integrity.json:52` require those contexts in the
  checked-in payload.
- `scripts/apply-repo-safeguards.sh:136` through
  `scripts/apply-repo-safeguards.sh:138` include those contexts in the apply
  script.
- Live GitHub currently omits those contexts from both the active
  `Protect main: integrity` ruleset and classic branch protection.

Risk:

The config layer is now owned by `home/` and validated by the parity jobs, but
future PRs can merge without those jobs being non-bypassable. Static tests prove
the checked-in files agree; they do not prove GitHub is enforcing them.

Canonical solution:

1. Run `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles` with an
   owner/admin credential.
2. Verify the live `Protect main: integrity` ruleset by fetching the specific
   ruleset ID and asserting the required-status contexts, not only the ruleset
   list endpoint.
3. Update `docs/security/branch-protection.md` with exact verification commands
   for the live required contexts.
4. Record the date and output summary in `docs/MIGRATION_STATUS.md`, then move
   the "required checks not live" item to resolved.

## P1 - Supply Chain Integrity

### 2. Mutable remote installer scripts still execute in first-run paths

Status: open.

Evidence:

- `install-deps.sh:278` runs the Homebrew `HEAD` installer fetched from GitHub.
- `install-deps.sh:1327` runs `get.chezmoi.io` as a fetched shell script.
- `install-deps.sh:1858` pipes the Starship installer into `sh`.
- `install-deps.ps1:157` downloads `https://get.scoop.sh` and executes it.
- `.github/workflows/test.yml:48` through `.github/workflows/test.yml:49`
  fetch and run the Starship installer in CI without a digest check.
- `.github/workflows/test.yml:225`, `.github/workflows/test.yml:268`, and
  `.github/workflows/test.yml:310` through `.github/workflows/test.yml:311`
  install chezmoi from fetched installer scripts.
- The repo already has stronger patterns for other downloads, for example
  Ghostty script verification in `install-deps.sh:1945`, Hack font verification
  in `install-deps.ps1:753`, and Windows Terminal portable verification in
  `install-deps.ps1:1315`.

Risk:

The repo has two supply-chain standards at once. Direct archives and some
installer scripts are pinned and hash-checked, while package-manager/bootstrap
scripts remain mutable trust roots. A compromised upstream script or transient
server response becomes code execution during setup.

Canonical solution:

1. Define a repository-wide policy: every direct network executable must be
   pinned and verified, or explicitly documented as an unavoidable trust root.
2. Prefer release artifacts with SHA-256 verification over installer scripts.
3. Where a project only provides a bootstrap script, pin a reviewed revision or
   remove automatic bootstrap and make the package manager a named prerequisite.
4. Add a static guard that rejects new `curl | sh`, `sh -c "$(curl ...)"`,
   `Invoke-RestMethod` plus `scriptblock::Create`, and raw installer execution
   unless the call appears in a reviewed allowlist with a rationale.
5. Include the public remote setup one-liners in the policy. If they remain as a
   convenience path, document that they trust current `main`; otherwise move the
   recommended path to `git clone` plus local `setup`.

## P1 - Greenfield Proof

### 3. Windows Sandbox greenfield path still points at `chezmoi-pilot`

Status: open.

Evidence:

- `tests/greenfield/windows-sandbox.wsb:15` downloads
  `raw.githubusercontent.com/luisgui1757/dotfiles/chezmoi-pilot/...`.
- `tests/greenfield/sandbox-bootstrap.ps1:2` defaults `$Ref` to
  `chezmoi-pilot`.
- `tests/greenfield/README.md:90` through `tests/greenfield/README.md:92`
  instruct users to change a `chezmoi-pilot` ref.
- `tests/greenfield/RUNBOOK.md:16` through `tests/greenfield/RUNBOOK.md:17`
  still say to use `chezmoi-pilot` until merge.
- `tests/greenfield/RUNBOOK.md:58` through `tests/greenfield/RUNBOOK.md:62`
  also fetch `chezmoi-pilot` in the manual Windows path.
- `tests/greenfield/RUNBOOK.md:114` and `tests/greenfield/RUNBOOK.md:138`
  clone `chezmoi-pilot` for macOS and Linux VM paths.

Risk:

The advertised clean-machine Windows proof can test the wrong branch or fail
before setup starts. Since the branch has now merged, this turns a high-value
manual gate into stale documentation and weak evidence.

Canonical solution:

1. Default every greenfield script and runbook path to `main`.
2. Add an explicit `-Ref` or documented URL edit for PR validation.
3. Add a cheap static test that fails if `chezmoi-pilot` appears outside
   archived historical docs.
4. After updating, run the Windows Sandbox path once and append the result to a
   greenfield evidence log.

### 4. Greenfield visual/manual evidence has no durable ledger

Status: open.

Evidence:

- `tests/greenfield/README.md:178` through `tests/greenfield/README.md:189`
  define visual checks for terminal colors, glyphs, psmux, and VS Code.
- `tests/greenfield/RUNBOOK.md:170` through `tests/greenfield/RUNBOOK.md:240`
  define the copy-paste manual checklist.
- `docs/MIGRATION_STATUS.md:129` through `docs/MIGRATION_STATUS.md:132` still
  record `0 / 10` green Ubuntu parity runs.

Risk:

The repo has strong scripted gates, but the desktop/UI claims still depend on
manual checks whose latest result is not recorded. Future reviewers cannot tell
whether a visual surface has been exercised after a major setup migration.

Canonical solution:

1. Add a small append-only greenfield evidence ledger, for example
   `tests/greenfield/LEDGER.md`.
2. Record environment, branch/SHA, command, pass/fail, and remaining manual
   observations for Windows Sandbox, WSL, macOS, and Linux VM runs.
3. Keep the N-green counter in `docs/MIGRATION_STATUS.md` synchronized with
   that ledger.
4. Do not make visual checks pretend to be automated; document them as manual
   evidence with dates and exact machines.

## P1 - Automation Coverage

### 5. Renovate schema validation is documented but not merge-gated

Status: open.

Evidence:

- `CLAUDE.md:479` through `CLAUDE.md:490` says Renovate must be validated with
  Renovate's own schema validator.
- `Makefile:81` through `Makefile:82` defines `make validate-renovate`.
- `Makefile:59` defines `make test` without `validate-renovate`.
- `.github/workflows/test.yml:78` through `.github/workflows/test.yml:85`
  runs `make test-static` and `make lint`, but not `make validate-renovate`.
- `tests/static/json_lint.sh` proves JSON syntax, not Renovate schema behavior.

Risk:

A schema-invalid `renovate.json` can merge green while update automation
silently stops, misgroups runner image updates, or mishandles pinned constants.
The current config validates, so this is a gate-coverage gap rather than a
current config failure.

Canonical solution:

1. Add `make validate-renovate` to the required Ubuntu CI path.
2. Make missing Node/npm/npx fatal under `CI=true`.
3. Keep `make test-static` fast if desired, but add a `make ci` or
   `make test-required` target that matches the required local proof bundle.

### 6. There is no single canonical local "full gate" command

Status: open.

Evidence:

- `make test` passed locally, but it does not run the migration parity suite or
  Renovate schema validation.
- The actual post-audit proof required at least:
  `make test`, `make validate-renovate`, and the migration scripts under
  `tests/migration/`.
- `README.md:321` through `README.md:325` points to greenfield docs separately
  from the test entry point.

Risk:

Future agents can honestly run `make test`, see green, and still miss required
migration/update-policy evidence. That is not a canonical production gate.

Canonical solution:

1. Add a top-level `make ci` or `make test-required`.
2. Include `make test`, `make validate-renovate`, and the host-appropriate
   migration checks.
3. Keep OS-specific skips explicit and fail missing required tools in CI.
4. Document that `make test` is the current-host fast suite, while the new target
   is the exact pre-PR local gate.

### 7. The static editorconfig sweep is not stable after generated test caches exist

Status: open.

Evidence:

- `tests/nvim/minimal_init.lua:53` through `tests/nvim/minimal_init.lua:72`
  clones and checks out Plenary under `tests/.cache/plenary.nvim`.
- `.gitignore:5` ignores `tests/.cache/`, so the cache is intentionally local
  generated state.
- `tests/static/json_lint.sh`, `tests/static/yaml_lint.sh`,
  `tests/static/toml_lint.sh`, `tests/static/ps1_parse.sh`, and shell lint
  already exclude `tests/.cache`.
- `tests/static/editorconfig_check.sh:13` excludes `.git`, `.claude`, `home`,
  and `nvim/lazy-lock.json`, but not `tests/.cache`.
- After `make test`, the local `tests/.cache` contained 7,435 paths and 43 MB of
  generated dependency/test state. A full `editorconfig-checker` sweep was still
  silent after 20 seconds and had to be terminated, while
  `editorconfig-checker ROADMAP.md` passed instantly.

Risk:

The fast static gate is order-dependent: `make test-static` can pass on a fresh
checkout, then become slow or hang after `make test` creates ignored local cache
state. That makes the local proof bundle brittle and encourages agents to skip
or interrupt static validation.

Canonical solution:

1. Exclude `tests/.cache` from `tests/static/editorconfig_check.sh`, matching the
   other static scanners.
2. Prefer keeping generated dependency clones outside the repo root if a future
   harness cleanup makes that practical.
3. Add a regression test or invariant that all repo-wide file walkers exclude
   `tests/.cache` unless they are intentionally testing cache contents.

## P2 - Config Semantics

### 8. zsh plugin root semantics split between XDG-aware runtime and fixed chezmoi paths

Status: open and already acknowledged in `docs/MIGRATION_STATUS.md`.

Evidence:

- `shells/zshrc:68` loads zsh plugins from
  `${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins`.
- `install-deps.sh:1335` through `install-deps.sh:1337` installs plugins to the
  same XDG-aware root.
- `home/.chezmoiexternal.toml.tmpl:4` and
  `home/.chezmoiexternal.toml.tmpl:11` install chezmoi externals to fixed
  `.local/share` paths.
- `home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl:5`
  through line 6 verify the fixed `.local/share` path.
- `uninstall.sh:176` through `uninstall.sh:179` and `uninstall.ps1:155` through
  `uninstall.ps1:160` only classify fixed `.local/share` externals.
- `docs/MIGRATION_STATUS.md:136` through `docs/MIGRATION_STATUS.md:138`
  explicitly call XDG-aware externals Wave B.

Risk:

Hosts with `XDG_DATA_HOME` set can install or load plugins from one tree while
chezmoi applies, verifies, or uninstalls another. The current parity tests unset
`XDG_DATA_HOME`, which makes the canonical fixed path deterministic but does not
prove the runtime XDG path.

Canonical solution:

Choose one contract and make it universal:

1. Either remove XDG support from zsh runtime/install until Wave B, so everything
   uses fixed `~/.local/share`.
2. Or introduce a single `zshPluginRoot` chezmoi data value and use it in
   `shells/zshrc`, `install-deps.sh`, `.chezmoiexternal.toml.tmpl`, the pin
   verifier, uninstall scripts, parity tests, and greenfield validators.

## Disproved Or Non-Blocking Assumptions

- Shell, zsh, and PowerShell syntax are not current blockers:
  `bash -n`, `zsh -n`, PowerShell parser coverage, `make test-static`, and
  `make test` passed.
- The current `.chezmoi.toml.tmpl` no longer has the prior `mode` nesting bug.
- The tmux prefix regression is not present; the shared config uses `C-b`.
- Recurring Neovim traps checked clean by source and tests: leader before
  lazy.nvim, Mason headless commands registered, conform remains the only
  format-on-save path, and Treesitter bundled-parser cleanup is scoped.
- Checked-in ruleset/settings/apply-script required-check lists are internally
  synchronized; the problem is live GitHub enforcement drift.
- `renovate.json` is currently schema-valid.
- The direct `uninstall_safety_test.sh` passes; the earlier chained local
  migration command did not reproduce a persistent repo failure.

## Preferred Execution Order

1. Apply and verify live repository safeguards so the parity jobs are actually
   required.
2. Fix stale `chezmoi-pilot` greenfield references and add a guard against
   reintroducing them.
3. Make the static editorconfig sweep ignore generated caches.
4. Add Renovate validation and a canonical top-level full-gate target.
5. Decide and enforce the supply-chain policy for remote executable scripts.
6. Resolve the zsh plugin root contract.
7. Start a greenfield evidence ledger and move the N-green counter out of
   informal memory.
