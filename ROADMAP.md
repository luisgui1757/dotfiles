# Dotfiles Roadmap

Last audited: 2026-06-18 on branch `audit/full-roadmap-review-2026-06-18`.
Baseline: `main` at `c3c22b7`.

This is the adversarial post-merge roadmap for the chezmoi migration and the
current setup/CI surface. The goal is not "good enough"; the repo should have a
single obvious setup path, enforce the tests it claims are required, avoid
mutable supply-chain execution where practical, and make greenfield evidence
repeatable instead of tribal.

## Audit Evidence

- Local `make ci`: passed on 2026-06-18 after the final fix round. This covered
  static checks, shellcheck, Neovim specs, shell/starship/tmux/ghostty tests,
  Renovate schema validation, and the POSIX migration bundle.
- Local migration checks passed inside `make ci` and when run directly:
  `template_test.sh`, `parity_gate.sh`, `greenfield_roundtrip.sh`,
  `uninstall_safety_test.sh`, `windows_render_test.sh`, and `oracle_test.sh`.
- After Neovim tests populated generated plugin caches under `tests/.cache`,
  `editorconfig-checker` and Git-backed `chezmoi doctor` exposed traversal
  brittleness. `editorconfig_check.sh` now feeds a pruned per-file list,
  `invariants_test.sh` guards that pruning, and `parity_gate.sh` runs
  `chezmoi doctor` against a temporary copy of `home/` outside the Git checkout.
- `startup_spec.lua` now prewarms the locked Lazy plugin graph only when missing
  and uses a strict best-of-three warm measurement so local scheduler/filesystem
  outliers do not masquerade as production startup regressions.
- Live GitHub protection was applied and verified on 2026-06-18 with
  `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles`.
- The active `Protect main: integrity` ruleset is active, strict, has no bypass
  actors, and requires `ubuntu`, `macos`, `windows`, `chezmoi-parity`,
  `chezmoi-parity-macos`, `chezmoi-parity-windows`,
  `e2e containers / ubuntu-24.04`, `setup.sh / ubuntu-24.04`,
  `setup.sh / macos-15`, and `setup.ps1 / windows-2025`.
- Classic branch-protection fallback is strict and requires the same context
  set. GitHub returns those contexts in a different order than the ruleset API,
  so the apply-script verifier compares exact set membership.
- Post-fix audit hardening added regression coverage for POSIX uninstall
  dry-run immutability, mirrored chezmoi/Starship/tree-sitter pins,
  required-check list duplication, and the Windows Sandbox bootstrap trust root.

## P0 - Required Gate Reality

### 1. Live `main` protection does not require the `chezmoi-parity*` jobs

Status: done. Live GitHub enforcement was applied and verified on 2026-06-18.

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
- Live GitHub now includes those contexts in both the active
  `Protect main: integrity` ruleset and classic branch-protection fallback.

Risk:

Resolved: future PRs cannot merge through the protected branch path unless the
three `chezmoi-parity*` jobs pass alongside the rest of the required suite.
Static tests still prove checked-in required-check lists agree, and
`scripts/apply-repo-safeguards.sh` now verifies the live ruleset and classic
fallback required-context sets after applying them.

Canonical solution:

1. DONE - Run `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles` with an
   owner/admin credential.
2. DONE - Verify the live `Protect main: integrity` ruleset by fetching the
   specific ruleset ID and asserting the required-status contexts, not only the
   ruleset list endpoint.
3. DONE - Update `docs/security/branch-protection.md` with exact verification
   commands for the live required contexts.
4. DONE - Record the date and output summary in `docs/MIGRATION_STATUS.md`, then
   move the "required checks not live" item to resolved.

## P1 - Supply Chain Integrity

### 2. Mutable remote installer scripts still execute in first-run paths

Status: fixed in `audit/full-roadmap-review-2026-06-18`.

Evidence:

- `install-deps.sh` no longer runs the Homebrew `HEAD` installer; it downloads
  a pinned Homebrew installer commit and verifies SHA-256 before execution.
- Native-Linux `install-deps.sh` no longer runs `get.chezmoi.io`; it downloads
  the pinned chezmoi GitHub release archive and verifies SHA-256 before
  installing.
- `install-deps.sh` no longer pipes the Starship installer into `sh`; native
  Linux/WSL without brew uses pinned Starship release archives with SHA-256
  verification, while Alpine uses its native package.
- `install-deps.ps1:157` downloads `https://get.scoop.sh` and executes it.
- CI no longer runs the Starship installer script as fallback; it downloads the
  pinned Starship release tarball and verifies SHA-256 before extraction.
- CI no longer runs `get.chezmoi.io`; POSIX parity jobs use
  `scripts/install-pinned-chezmoi.sh`, and Windows parity downloads the pinned
  chezmoi zip and verifies SHA-256 before extraction.
- Recommended setup docs no longer use raw `curl | bash`/`iwr` execution of the
  current default branch; they use `git clone` plus local `setup`.
- The repo already has stronger patterns for other downloads, for example
  Ghostty script verification in `install-deps.sh`, Hack font verification in
  `install-deps.ps1`, and Windows Terminal portable verification in
  `install-deps.ps1`.

Risk:

The repo has two supply-chain standards at once. Direct archives and some
installer scripts are pinned and hash-checked, while package-manager/bootstrap
scripts remain mutable trust roots. A compromised upstream script or transient
server response becomes code execution during setup.

Resolution:

1. Repository policy now requires direct network executables to be pinned and
   verified, or explicitly allowlisted with a rationale.
2. Native Linux chezmoi moved from `get.chezmoi.io` script execution to a
   SHA-256-verified release artifact.
3. The Starship curl installer fallback was replaced by SHA-256-verified
   release artifacts on native Linux/WSL without brew.
4. `tests/static/supply_chain_remote_execution_test.sh` rejects new
   `curl | sh`, `sh -c "$(curl ...)"`, raw `/tmp/*install*.sh` execution, and
   PowerShell `scriptblock::Create` execution unless the exact line appears in
   the reviewed allowlist.
5. CI Starship, tree-sitter CLI, and chezmoi installs now use SHA-256-verified
   release artifacts.
6. The remaining mutable installer trust roots are Scoop bootstrap on Windows
   (consent-gated package-manager bootstrap) and the documented disposable
   Windows Sandbox self-bootstrap path; both are explicit in the static
   allowlist.

## P1 - Greenfield Proof

### 3. Windows Sandbox greenfield path still points at the retired pilot branch

Status: done.

Evidence:

- `tests/greenfield/windows-sandbox.wsb` now downloads the bootstrap from
  `main` and passes `-Ref 'main'` by default.
- `tests/greenfield/sandbox-bootstrap.ps1` now defaults `$Ref` to `main`.
- `tests/greenfield/README.md` and `tests/greenfield/RUNBOOK.md` document the
  explicit PR/branch override path.
- `tests/static/stale_greenfield_refs_test.sh` fails if the retired pilot
  branch name appears outside archived historical docs.

Risk:

Resolved: advertised clean-machine proofs now test `main` unless a reviewer
explicitly opts into a PR/branch override.

Canonical solution:

1. DONE - Default every greenfield script and runbook path to `main`.
2. DONE - Add an explicit `-Ref` or documented URL edit for PR validation.
3. DONE - Add a cheap static test for the retired branch name outside archived
   historical docs.
4. ENVIRONMENTAL - Run the Windows Sandbox path once on a Windows host and append
   the result to `tests/greenfield/LEDGER.md`. This macOS checkout cannot launch
   Windows Sandbox; the script, XML, parser, and stale-ref guard are validated
   locally.

### 4. Greenfield visual/manual evidence has no durable ledger

Status: done.

Evidence:

- `tests/greenfield/LEDGER.md` is now the append-only record for
  clean-machine and visual evidence.
- `tests/greenfield/README.md` and `tests/greenfield/RUNBOOK.md` point
  reviewers to the ledger after automated and manual runs.
- `docs/MIGRATION_STATUS.md` now points the N-green counter at the ledger.

Risk:

Resolved for documentation structure: manual checks remain honest manual
evidence, and the repo now has one durable place to append them.

Canonical solution:

1. DONE - Add a small append-only greenfield evidence ledger:
   `tests/greenfield/LEDGER.md`.
2. DONE - Include environment, branch/SHA, command, pass/fail, and remaining
   manual observations fields.
3. DONE - Point the N-green counter in `docs/MIGRATION_STATUS.md` at the
   ledger.
4. DONE - Keep visual checks documented as manual evidence with dates and exact
   machines.

## P1 - Automation Coverage

### 5. Renovate schema validation is documented but not merge-gated

Status: done.

Evidence:

- `.github/workflows/test.yml` now runs `make validate-renovate` in the required
  Ubuntu job immediately after static lint.
- `scripts/validate-renovate.sh` now fails instead of skipping when `npx` is
  missing under `CI=true`.
- `scripts/validate-renovate.sh` pins both the Node runtime and Renovate
  validator package; `renovate.json` owns those pins through custom managers.
- `Makefile` exposes `make ci` / `make test-required`, which include Renovate
  schema validation.

Risk:

Resolved: a schema-invalid `renovate.json` now fails the required Ubuntu CI job
and the canonical local `make ci` gate.

Canonical solution:

1. DONE - Add `make validate-renovate` to the required Ubuntu CI path.
2. DONE - Make missing Node/npm/npx fatal under `CI=true`.
3. DONE - Keep `make test-static` fast, and add a `make ci` or
   `make test-required` target that matches the required local proof bundle.

### 6. There is no single canonical local "full gate" command

Status: done.

Evidence:

- `Makefile` now defines `make ci` and `make test-required`.
- The full local gate runs `make test`, `make validate-renovate`, and
  `make test-migration`.
- `make test-migration` runs the POSIX migration template, parity, round-trip,
  uninstall safety, Windows render, and oracle checks with `~/.local/bin` on
  `PATH`.
- `README.md` and `CLAUDE.md` document `make ci` as the exact local pre-PR gate.

Risk:

Resolved: future agents have one top-level command for the required local proof
bundle. `make test` remains the current-host fast suite; `make ci` is the full
pre-PR gate.

Canonical solution:

1. DONE - Add a top-level `make ci` or `make test-required`.
2. DONE - Include `make test`, `make validate-renovate`, and the host-appropriate
   migration checks.
3. DONE - Keep OS-specific skips explicit and fail missing required tools in CI.
4. DONE - Document that `make test` is the current-host fast suite, while the
   new target is the exact pre-PR local gate.

### 7. The static editorconfig sweep is not stable after generated test caches exist

Status: done.

Evidence:

- `tests/nvim/minimal_init.lua:53` through `tests/nvim/minimal_init.lua:72`
  clones and checks out Plenary under `tests/.cache/plenary.nvim`.
- `.gitignore:5` ignores `tests/.cache/`, so the cache is intentionally local
  generated state.
- `tests/static/json_lint.sh`, `tests/static/yaml_lint.sh`,
  `tests/static/toml_lint.sh`, `tests/static/ps1_parse.sh`, and shell lint
  already exclude `tests/.cache`.
- `tests/static/editorconfig_check.sh` now excludes `.git`, `.claude`,
  `tests/.cache`, `home`, and `nvim/lazy-lock.json`.
- After `make test`, the local `tests/.cache` contained 7,435 paths and 43 MB of
  generated dependency/test state. After the fix, `bash tests/static/run_all.sh`
  passes with that cache present.
- `tests/static/invariants_test.sh` fails if the editorconfig sweep stops
  excluding generated `tests/.cache` content.

Risk:

Resolved: the fast static gate no longer scans generated Plenary/cache content
after dynamic tests populate `tests/.cache`.

Canonical solution:

1. DONE - Exclude `tests/.cache` from `tests/static/editorconfig_check.sh`,
   matching the other static scanners.
2. DEFERRED - Prefer keeping generated dependency clones outside the repo root if a future
   harness cleanup makes that practical.
3. DONE - Add a regression test or invariant that all repo-wide file walkers exclude
   `tests/.cache` unless they are intentionally testing cache contents.

## P2 - Config Semantics

### 8. zsh plugin root semantics split between XDG-aware runtime and fixed chezmoi paths

Status: done. The canonical contract is fixed
`~/.local/share/dotfiles/zsh-plugins`; install-deps, runtime, chezmoi externals,
pin verification, uninstall, greenfield validation, container/WSL validation,
and parity fixtures now use or assert that root.

Evidence:

- `shells/zshrc:69` loads zsh plugins from the fixed
  `$HOME/.local/share/dotfiles/zsh-plugins` root.
- `install-deps.sh` installs zsh plugins into the fixed
  `$HOME/.local/share/dotfiles/zsh-plugins` root.
- `home/.chezmoiexternal.toml.tmpl:4` and
  `home/.chezmoiexternal.toml.tmpl:11` install chezmoi externals to fixed
  `.local/share` paths.
- `home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl:5`
  through line 6 verify the fixed `.local/share` path.
- `uninstall.sh:176` through `uninstall.sh:179` and `uninstall.ps1:155` through
  `uninstall.ps1:160` only classify fixed `.local/share` externals.
- `tests/migration/parity_gate.sh`, `tests/migration/oracle_test.sh`,
  `tests/migration/greenfield_roundtrip.sh`, and
  `tests/migration/uninstall_safety_test.sh` now set a hostile `XDG_DATA_HOME`
  while asserting the fixed plugin root.
- `tests/ci/container-e2e.sh`, `tests/wsl/e2e.sh`, and
  `tests/greenfield/validate.sh` validate the fixed plugin root.

Risk:

Resolved: hosts with `XDG_DATA_HOME` set still install, apply, verify, source,
validate, and uninstall zsh plugins through the same fixed repo-managed tree.

Canonical solution:

DONE - Removed XDG support from `install-deps.sh`'s `zsh_plugin_root()` so the
installer uses the same fixed `~/.local/share/dotfiles/zsh-plugins` path as
every other zsh plugin surface.

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

1. DONE - Apply and verify live repository safeguards so the parity jobs are
   actually required.
2. DONE - Fix stale greenfield branch references and add a guard against
   reintroducing them.
3. DONE - Make the static editorconfig sweep ignore generated caches.
4. DONE - Add Renovate validation and a canonical top-level full-gate target.
5. DONE - Decide and enforce the supply-chain policy for remote executable scripts.
6. DONE - Resolve the zsh plugin root contract.
7. DONE - Start a greenfield evidence ledger and move the N-green counter out
   of informal memory.
