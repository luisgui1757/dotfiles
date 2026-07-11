# Ultimate gold-standard reconciliation — 2026-07-10

This is the append-only implementation ledger for branch
`fix/ultimate-gold-standard-close-2026-07-10`, based on live `origin/main`
`85375b2bdec9d3a998e8023a44b41d03a32f3eaa`. Later entries supersede earlier
status entries; history is never deleted or rewritten.

## Initial inventory

| ID | Initial status | Required resolution surface |
|---|---|---|
| UGR-001 | ACCEPTED | Independent transactional packaged/portable Windows Terminal merges and recovery |
| UGR-002 | ACCEPTED | Architecture-specific Intel and Apple Silicon Darwin configurations and proof |
| UGR-003 | ACCEPTED | Fail-closed locked Lazy/Plenary checkout proof before execution |
| UGR-004 | ACCEPTED | Uniform POSIX install failure accumulation |
| UGR-005 | ACCEPTED | Pi npm tarball SRI bound to installed bytes |
| UGR-006 | ACCEPTED | Staged, verified, self-healing zsh plugin publication |
| UGR-007 | ACCEPTED | Exact compatible Windows Tree-sitter CLI |
| UGR-008 | ACCEPTED | Verified Microsoft CI package bytes before privileged install |
| UGR-009 | ACCEPTED | gh-dash/Windows Terminal/Actions provenance tails |
| UGR-010 | ACCEPTED | Explicit native chezmoi exit handling under both preference states |
| UGR-011 | ACCEPTED | Home Manager session variables in fresh Linux/WSL zsh |
| UGR-012 | ACCEPTED | Complete Brew-less macOS dry-run plan |
| UGR-013 | ACCEPTED | Transactional nix-homebrew tap migration rollback |
| UGR-014 | ACCEPTED | Filename-keyed strict backup restoration |
| UGR-015 | ACCEPTED | Canonical POSIX identity/home and Windows known-folder targets |
| UGR-016 | ACCEPTED | Invocation/I/O-aware PowerShell profile guard |
| UGR-017 | ACCEPTED | Checked scoped Tree-sitter deletion |
| UGR-018 | ACCEPTED | Per-project clangd compile database behavior |
| UGR-019 | ACCEPTED | Live-truthful Renovate discovery and inventory proof |
| UGR-020 | ACCEPTED | Stable required-check transition without fake checks |
| UGR-021 | ACCEPTED | Honest cache-free/manual greenfield proof lanes |
| UGR-022 | ACCEPTED | Documentation/status truth repair |
| UGR-023 | ACCEPTED | Focused smaller reliability and checker gaps |

## Resolution entries

### UGR-003 — implementation entry 1

- Status: FIXED, pending final full-gate confirmation.
- Evidence: the pre-change `make ci` reproduced a real-init mutation of the
  prewarmed Lazy cache. `nvim/init.lua` and `tests/nvim/minimal_init.lua` now
  call the shared fail-closed checkout helper before runtimepath mutation.
- Implementation: valid full 40-hex lock parsing; expected origin, exact HEAD,
  clean state, usable worktree, and required-entrypoint checks; locked sibling
  staging; exact-commit fetch; verified publication with previous-checkout
  rollback; cleanup on injected fetch/checkout failure; concurrent first-start
  reuse.
- Focused test: `pinned_git_checkout_spec.lua` — 9 passed, 0 failed.
- Documentation: README troubleshooting, CLAUDE invariant 23, ROADMAP status,
  and this ledger.
- Residual/manual proof: full `make test-nvim` and cross-platform CI remain to
  run after the complete branch is assembled.

### UGR-003 — implementation commit identity

- Implementation commit: `60dd01a` (`fix(nvim): prove locked plugin checkouts
  before execution`).

### UGR-002 / UGR-011 / UGR-012 / UGR-013 / UGR-015 — POSIX implementation entry 1

- Status: UGR-002 FIXED pending the real Intel lane result; UGR-011 FIXED with
  real WSL proof pending; UGR-012 FIXED; UGR-013 FIXED; UGR-015 PARTIAL because
  the Windows known-folder half remains.
- Reproduction/evidence: baseline setup selected only aarch64 Darwin and rejected
  x86_64; the flake derived homes from usernames; zsh did not source Home Manager
  session state; Brew-less dry-run returned failure from the previewed bootstrap;
  and the tap pre-move had no rollback path.
- Implementation: separate aarch64/x86_64 Darwin outputs and exact normalized
  selection; one authoritative non-root account/home boundary threaded through
  Nix and sudo; architecture/repository-aware Homebrew paths; collision-safe tap
  backup with activation/bootstrap/signal rollback; one-shot canonical Home
  Manager session-vars startup; complete Brew-less dry-run package-manager plan.
- Focused tests: `setup_nix_darwin_test.sh`, `darwin_config_test.sh`,
  `linux_home_test.sh`, `setup_target_identity_test.sh`,
  `home_manager_session_vars_test.sh`, `brewless_darwin_dry_run_test.sh`, and
  `homebrew_shellenv_test.sh`.
- CI/proof surface: added non-required `macos-26-intel` Nix and full setup lanes;
  scheduled/manual setup lanes are cache-free; Linux proves fresh zsh state with
  no PATH injection; macOS invokes real Ghostty, WezTerm, and AeroSpace config
  consumers.
- Documentation: README, CLAUDE invariant 24, ROADMAP, MIGRATION_STATUS,
  MANUAL, greenfield pending-proof section, and this ledger.
- Residual/manual proof: actual Intel and WSL runs, manual desktop visual proof,
  and the Windows half of UGR-015. No ledger evidence row is claimed before a
  run exists.
- Implementation commit: `1423a47` (`fix(posix): close architecture and
  identity gaps`).

### UGR-021 — implementation entry 1

- Status: PARTIAL.
- Implemented here: broad setup caches run only for pull requests and are keyed
  by OS plus architecture; scheduled/manual clean-install lanes are cache-free.
  macOS setup validates configs through the real Ghostty, WezTerm, and AeroSpace
  binaries, and Linux checks a fresh Home Manager-backed zsh without PATH
  injection.
- Remaining: Windows E2E font assertion, actual Intel/WSL/desktop run evidence,
  and any owner-recorded manual visual observations.

### UGR-001 / UGR-014 — implementation entry 1

- Status: FIXED, pending final full-gate and Windows-host confirmation.
- Reproduction: baseline `Copy-WindowsTerminalSettingsForUnpackaged` performed a
  forced full-file packaged-to-portable copy, backed up only packaged state, and
  downgraded portable write failure to a warning. Both uninstallers selected
  backups by mutable filesystem mtime.
- Implementation: setup excludes WT from chezmoi publication and plans packaged
  and portable targets from each target's own bytes. All outputs stage in their
  destination directory and parse/byte-validate before per-target verified
  backups or publication. Same-directory atomic replace captures the exact
  pre-publication bytes, closing the final source-check race; a multi-target
  failure rolls back prior publications. Named mutex serialization, cleanup,
  dry-run, skip, retry, idempotency, and explicit unsafe-rollback recovery are
  included. Bare chezmoi exposes no WT target.
- Recovery: POSIX/Windows backup selection validates
  `<target>.bak.<YYYYMMDD-HHMMSS>[.n]` and orders by filename timestamp plus
  collision suffix, never mtime. Malformed candidates fail before target
  removal. Windows uninstall validates both WT paths before restoring either,
  atomically restores independent backups, and preserves displaced current
  settings as `settings.json.uninstall-current.*`.
- Focused tests: 13 transactional WT Pester cases; 6 Windows uninstall/order
  Pester cases; `uninstall_backup_order_test.sh`; updated Windows render/apply/
  round-trip oracles. Cases include packaged-only, portable-only, divergent
  dual installs, missing targets, invalid JSON, stage/backup/publish failure,
  both concurrency windows, collision, dry-run, skip, retry, repeated setup,
  files/directories, opposing mtimes, malformed names, and dual restoration.
- Documentation: README, `windows-terminal/README.md`, CLAUDE, ROADMAP,
  MIGRATION_STATUS, and this ledger.
- Implementation commit: pending creation of this cohesive data-safety commit;
  an append-only identity entry will follow.

### UGR-001 / UGR-014 — implementation commit identity

- Implementation commit: `f1c9e2c` (`fix(windows): make terminal settings
  transactional`).

### UGR-004 through UGR-009 — implementation entry 1

- Status: UGR-004, UGR-005, UGR-006, UGR-007, UGR-008, and UGR-009 FIXED,
  pending final full gates and native-Windows CI. The checked-in Actions
  SHA-pinning safeguard is implemented but pending owner application after
  merge; no live setting was changed.
- UGR-004: every recoverable POSIX main-flow install crosses
  `run_install_step`. It absorbs `set -e`, records a nonzero callee only when
  the callee did not already record a precise failure, continues to a later
  sentinel, prints one consolidated summary, and exits nonzero. The audit
  covers catalog installs plus Neovim, chezmoi, lazygit, Starship, tmux/zsh
  plugins, Ghostty/WezTerm, Herdr, Pi, Tree-sitter, pylatexenc, fonts, and GUI
  paths.
- UGR-005: POSIX and Windows run `npm pack`, validate the single pack metadata
  identity and actual tarball SHA-512 bytes against the mirrored SRI, and pass
  only that verified local tarball to npm install. Unique temp state is cleaned
  on mismatch, network/install failure, interruption, success, and retry.
- UGR-006: generic zsh git-repo externals are removed. Install-deps and a
  pin/helper-sensitive chezmoi `run_onchange` script call one serialized publisher that neutralizes an
  unproved sourceable target, fetches the exact commit into a same-parent
  sibling, proves origin/HEAD/cleanliness/worktree/tracked regular entry file,
  and publishes atomically. Clean pin changes self-heal; unsafe prior payloads
  remain quarantined for recovery.
- UGR-007: Windows Tree-sitter no longer accepts mutable Scoop/npm fallback
  results. Existing compatible `0.26.10` commands remain untouched; stale,
  partial, or incompatible state repairs through architecture-specific,
  SHA-256-verified `v0.26.10` release archives with pre/post version proof and
  rollback-safe publication under the LocalApplicationData known folder.
- UGR-008: required Ubuntu CI pins the Ubuntu 24.04 Microsoft repository `.deb`
  SHA-256 before `sudo dpkg -i`. The supply-chain scanner self-tests and enforces
  download-to-privileged-package verification ordering.
- UGR-009: gh-dash `v4.25.1` is paired with annotated tag object
  `e6ebbd7e83e30161b9192ce3339972d2c8269e7f` and peeled commit
  `49f37e4832956c57bf52d4ea8b1b1e5c0f863700`; installers verify the mapping and
  pin the commit. The Sandbox Terminal helper imports the production
  version/hash, publishes transactionally, and never queries latest or mirrors
  packaged settings. All external Actions uses are full-SHA scanned. The
  safeguard script requests `sha_pinning_required=true`; live was observed
  false and remains untouched.
- Focused tests: POSIX Pi tarball/network/metadata/partial/install/cleanup/retry;
  PowerShell Pi SRI and lifecycle; zsh failure neutralization/self-heal/
  concurrency; failure-accumulator sentinel; Windows Tree-sitter architecture,
  compatible/stale/partial/checksum/rollback/dry-run cases; gh-dash moved-tag
  rejection; privileged-package and Actions scanner self-tests; PowerShell
  parser and pin consistency.
- Documentation: README, CLAUDE, ROADMAP, MIGRATION_STATUS, security supply-chain
  identities, branch-protection live/desired truth, MANUAL, and this ledger.
- Residual/manual proof: native Windows install and rollback, a real bare
  chezmoi pin migration, and owner-applied/live Actions SHA policy after merge.
- Implementation commit: pending creation of this cohesive supply-chain commit;
  an append-only identity entry will follow.

## Final implementation classification — entry 2

This entry supersedes the initial statuses above. “Fixed” describes the
implemented behavior and local evidence, not a platform run that did not occur.
The final batch hash is appended in a later identity entry after Git creates it.

| ID | Final status | Evidence and implementation | Implementation commit | Focused tests | Documentation | Residual/manual proof |
|---|---|---|---|---|---|---|
| UGR-001 | ACCEPTED/FIXED | Reproduced forced packaged-to-portable mirroring; each packaged/Preview/portable target now merges from its own bytes with independent backup, staged validation, concurrent-change detection, atomic replace, rollback, and independent uninstall recovery. | `f1c9e2c` | 13 Setup WT transaction cases; 6 Uninstall ordering/recovery cases; Windows render/apply/round-trip oracles | README, Windows Terminal README, CLAUDE, ROADMAP, MIGRATION_STATUS | Real native-Windows divergent dual-install and uninstall run pending. |
| UGR-002 | PARTIAL | Separate `aarch64-darwin`/`x86_64-darwin` outputs, exact normalized selection, and architecture-aware Homebrew/tap transaction are implemented; official `macos-26-intel` lane exists. | `1423a47` | Darwin config evaluation/selection, Homebrew state/migration/rollback/dry-run shell suites | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL, greenfield ledger | Exact PR-head Intel runner result is required; cross-evaluation is not runtime proof. |
| UGR-003 | ACCEPTED/FIXED | Lazy/Plenary require a full locked identity and prove Git/origin/HEAD/clean/worktree/entrypoint before runtimepath; sibling staging is locked, verified, atomic, rollback-safe, and cleaned. | `60dd01a` | `pinned_git_checkout_spec.lua` (9 behavioral cases) plus full Neovim specs | README troubleshooting, CLAUDE, ROADMAP | Cross-platform CI pending; no unproved path executed locally. |
| UGR-004 | ACCEPTED/FIXED | Every recoverable POSIX main-flow installer crosses one accumulator boundary; later sentinel work and one nonzero summary are behavioral assertions. | `aa48aad` | `install_deps_failure_accumulator_test.sh`, strict shell suite | README, CLAUDE, ROADMAP, MIGRATION_STATUS | Real package-manager network failures remain CI/host evidence. |
| UGR-005 | ACCEPTED/FIXED | POSIX/Windows use `npm pack`, require metadata and actual tarball SHA-512 SRI agreement, install only the verified local tarball, validate version, and clean every exit. | `aa48aad` | POSIX Pi network/metadata/partial/install/retry cases; PowerShell Pi SRI/lifecycle cases | README, CLAUDE, ROADMAP, MANUAL | Native-Windows install run pending. |
| UGR-006 | ACCEPTED/FIXED | Shared serialized zsh publisher neutralizes unproved sourceable state, verifies sibling origin/commit/clean/file proof, atomically publishes, quarantines unsafe old data, and self-heals clean pin changes. | `aa48aad` | publisher oracle, concurrency, pin-change, failure-neutralization, chezmoi fingerprint cases | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL | Real bare `chezmoi apply` pin transition pending. |
| UGR-007 | ACCEPTED/FIXED | Exact Tree-sitter `0.26.10` predicate accepts compatible unmanaged binaries and repairs stale/partial/incompatible state from architecture-specific verified release bytes with rollback. | `aa48aad` | current/stale/partial/architecture/checksum/publication/dry-run Pester and pin consistency | README, CLAUDE, ROADMAP, MANUAL | Native-Windows runtime pending. |
| UGR-008 | ACCEPTED/FIXED | Ubuntu CI verifies the exact Microsoft repository `.deb` SHA-256 before privileged `dpkg`; the generic scanner recognizes and self-tests download-to-root-package ordering. | `aa48aad` | privileged-package scanner positive/negative self-tests; static suite | security supply-chain docs, README, CLAUDE | Required Ubuntu CI result pending. |
| UGR-009 | ACCEPTED/FIXED | gh-dash tag object and peeled commit are verified; Sandbox Terminal reuses production pin; every external `uses:` must be a full SHA; desired Actions SHA enforcement is checked in. | `aa48aad` | gh-dash moved-tag rejection, Sandbox/pin consistency, external-uses scanner self-tests | security supply-chain/branch protection, README, CLAUDE | Live `sha_pinning_required` was false and needs owner apply after merge. |
| UGR-010 | ACCEPTED/FIXED | Setup/uninstall native chezmoi calls isolate the preference with `try/finally`, capture exit/stderr explicitly, treat silent verify exit 1 as drift, and keep invocation failures fatal. | final closure batch (hash pending) | Setup/Uninstall Pester under preference true/false, drift, stderr, spaces, backup paths | README, CLAUDE, MIGRATION_STATUS | Windows CI pending; local pwsh behavior passed. |
| UGR-011 | PARTIAL | Fresh zsh sources canonical Home Manager session variables once, handles custom HOME/missing files/repeat sourcing, and the Linux E2E removes caller PATH injection. | `1423a47` | `home_manager_session_vars_test.sh`, Linux home tests | README, CLAUDE, ROADMAP, MANUAL | Exact native-Linux and WSL run results pending. |
| UGR-012 | ACCEPTED/FIXED | Brew-less Darwin dry-run models post-bootstrap brew availability and previews every later phase without claiming installation. | `1423a47` | complete noninteractive Brew-less preview/failure-plan shell cases | README, CLAUDE, ROADMAP | Darwin CI dry-run pending. |
| UGR-013 | ACCEPTED/FIXED | Tap migration is collision-checked and transactionally restores on activation/bootstrap/publication/signal failure, with explicit rollback-failure recovery. | `1423a47` | existing-rebuild/first-bootstrap/rollback/signal/collision cases | README, CLAUDE, ROADMAP, recovery docs, MANUAL | Real pre-existing-tap host injection remains manual. |
| UGR-014 | ACCEPTED/FIXED | POSIX/Windows restore candidates validate filename timestamp and suffix, ignore mtime, and reject malformed/ambiguous sets before mutation. | `f1c9e2c` | POSIX files/dirs/opposing-mtime/collision/malformed; Windows equivalent Pester | README, CLAUDE, ROADMAP, MIGRATION_STATUS | Native-Windows uninstall run pending. |
| UGR-015 | PARTIAL | POSIX has one account-record-backed identity/home; Windows independently resolves UserProfile, LocalApplicationData, Documents, and runtime profile, uses three source states, migrates recognized legacy shape after success, and post-checks consumers. | `1423a47` plus final closure batch (hash pending) | POSIX identity suite; 58 Setup/Uninstall Pester; Windows template/apply/round-trip/parity | README, CLAUDE invariants 24/25, MIGRATION_STATUS, MANUAL | Real redirected/OneDrive/alternate-drive Windows run and rollback pending. |
| UGR-016 | ACCEPTED/FIXED | Profile guard evaluates real argv, I/O redirection, CI, user-interactive state, and supported host before any cache path; batch/credential-helper processes produce no output/work. | final closure batch (hash pending) | 30 Profile Pester cases including real subprocesses and normal host cases | README troubleshooting, CLAUDE invariant 26, MANUAL | Native-Windows host matrix pending; local pwsh subprocess proof passed. |
| UGR-017 | ACCEPTED/FIXED | One data-root-scoped helper checks delete return and absence; parser, parser-info, and query cleanup fail synchronous setup on any partial removal and cannot reach built-in runtime. | final closure batch (hash pending) | 5 helper cases plus 18 Tree-sitter behavioral cases | README, CLAUDE invariant 19 | Cross-platform Neovim CI pending. |
| UGR-018 | ACCEPTED/FIXED | Removed startup-cwd compile database override; each clangd client uses canonical ancestor/build discovery and its own LSP root. | final closure batch (hash pending) | real clangd two-project/one-session isolation test; 24 LSP specs | CLAUDE LSP workflow, MANUAL | Interactive two-project confirmation remains manual; real headless clangd proof passed locally. |
| UGR-019 | PARTIAL | Nix beta manager explicitly enabled, Scoop branch live-verified as `master`, behind-base rebase restored, matrix runners use `github-runners`, and official Renovate local extraction must exactly equal 87 reviewed records. | final closure batch (hash pending) | regex matchability plus expected inventory; strict validator and official `--platform=local --dry-run=extract` passed | README, CLAUDE, ROADMAP, reconciliation | Hosted Dashboard/bot result pending after push; do not infer it from local extraction. |
| UGR-020 | PARTIAL | Stage 1 emits six stable logical checks that verify exact per-OS proof artifacts bound to run/head while all legacy contexts remain required; no no-op check or live mutation. | final closure batch (hash pending) | marker tamper/missing/duplicate tests; workflow/metadata/current-vs-candidate alignment | README, CLAUDE invariant 27, branch-protection runbook, MIGRATION_STATUS | After merge: observe logical checks, merge context-switch PR, then owner applies live safeguards. |
| UGR-021 | PARTIAL | Scheduled/manual setup caches are absent; WSL distro cache is disabled; macOS uses real WezTerm/AeroSpace/Ghostty consumers; Windows E2E asserts font files+registration; WSL stays non-required/fail-visible. | `1423a47` plus final closure batch (hash pending) | workflow/static cache contract, required-check alignment, E2E source assertions | README, CLAUDE, MANUAL, greenfield ledger | Intel, WSL, redirected Windows, Windows font E2E, and desktop GUI results pending; ledger has no fabricated rows. |
| UGR-022 | ACCEPTED/FIXED | ROADMAP baseline is live main, PR #46 is truthful DONE/merged, Make help says 80ms local/150ms CI, README documents `--best-effort`/`--skip-nvim`, and live-vs-checked-in safeguard language is explicit. | closure commits including final batch (hash pending) | doc/static/help/required-check guards | README, CLAUDE, ROADMAP, MIGRATION_STATUS, security docs, ledgers | Statuses must be refreshed again after CI/live results. |
| UGR-023 | ACCEPTED/FIXED | Starship cache publication is atomic/validated/rollback-safe; Polaris stages clean on failure/signal/retry; analyzer warnings have exact stable fingerprints; JSON traversal is NUL-safe; shell lint is strict; Nix ownership scanner catches nested/wrapped/imported bypasses while allowing system policy; direct-artifact compatible/stale/partial cases remain focused. | final closure batch (hash pending), with direct-artifact/shell-lint portions in `aa48aad` | Profile/Polaris/JSON path/Nix scanner self-tests, analyzer full entry point, direct-artifact update suites | README, CLAUDE, ROADMAP, MIGRATION_STATUS | Platform-specific CI remains pending. |

## Explicit rejected/out-of-scope candidates — entry 1

- Windows lsd relocation: REJECTED. The managed `%USERPROFILE%\.config\lsd`
  path is an upstream-supported Windows location; no reproduced consumption bug
  justified a persisted-path migration.
- Native-Windows Nix, a required WSL canary, a symmetry devcontainer,
  synchronized public-repo secrets, and CodeQL without a supported language
  surface: OUT OF SCOPE by product/security contract; no implementation.
- A blanket ban on nix-darwin `system.defaults`, `environment.etc`, or `launchd`:
  REJECTED. The structural ownership scanner targets Home Manager dotfile
  ownership options and its self-test explicitly permits legitimate system
  policy surfaces.
- Treating cache-dependent evidence as proof the underlying behavior is broken,
  or weakening the multi-OS product contract to avoid a feasible fix: REJECTED.
  Proof is labeled separately from behavior, and feasible platform paths remain.

## Supply-chain implementation identity — entry 2

- UGR-004 through UGR-009 implementation commit: `aa48aad`
  (`fix(supply-chain): bind executable installer identities`). This supersedes
  the pending identity in the earlier implementation entry without rewriting
  its history.

## Rejected-finding evidence — entry 2

- Windows lsd relocation remains REJECTED after live upstream verification on
  2026-07-10. `lsd-rs/lsd` default branch `main`, commit
  `fecadf36235be734b3fd97c44e237f3a29eb1073`, `src/config_file.rs:169-188`
  explicitly lists `%APPDATA%\lsd` and `%USERPROFILE%\.config\lsd` as the two
  Windows config search locations. No consumption failure was reproduced, so a
  persisted-target migration would add risk without fixing a defect.

## Prior-review preservation — entry 1

- Recovered `docs/reviews/2026-07-09-gold-standard-review.md` by reading
  `stash@{0}^3` from the original checkout and adding the recovered bytes here.
  No stash was popped, dropped, rewritten, or deleted; the original untracked
  review prompts were left untouched.

## Priority 2/3 implementation identity — entry 3

- UGR-010, the Windows half of UGR-015, UGR-016 through UGR-023, and their
  focused tests/documentation were committed as `eac92bc`
  (`fix(platform): close lifecycle and proof gaps`). This immutable identity
  supersedes every “final closure batch (hash pending)” cell above; the status
  and residual-proof classifications themselves remain unchanged.

## Local verification — entry 4

Executed on 2026-07-10 against `eac92bc` in the clean macOS worktree. This
documentation-only result entry was appended afterward:

| Command / suite | Result |
|---|---|
| `git diff --check` | PASS |
| `bash -n` over every tracked `*.sh` | PASS, 130 scripts |
| `make lint` | PASS, strict ShellCheck including `shells/zshrc` |
| `bash tests/static/run_all.sh` | PASS |
| `bash tests/shell/run_all.sh` | PASS |
| `make test-migration` | PASS: templates, parity, greenfield round-trip, uninstall safety/order, Windows render, and zsh publisher/drift oracle |
| PowerShell parser (`tests/static/ps1_parse.sh`) | PASS for every tracked PowerShell surface |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: analyzer exact fingerprint, Pester 231 passed / 0 failed / 0 skipped, and all 17 Neovim spec files exited 0 |
| Focused Windows Terminal Pester filter | PASS: 19 passed / 0 failed / 0 skipped (13 setup transaction plus 6 uninstall/recovery cases) |
| `make test-nvim` | PASS, including locked bootstrap, checked deletion, and real two-project clangd isolation |
| `make validate-renovate` | PASS: Renovate 43.257.4 validator and official local extraction exactly matched 87 reviewed records |
| `nix flake check` | PASS on the aarch64-darwin host; incompatible-system builds were omitted by Nix, while both Darwin architectures and both Linux Home Manager shapes were separately evaluated by the Nix suites |
| `make test` | PASS |
| `make ci` | PASS (`local pre-PR gate passed`) |

An earlier pre-final shell-suite run correctly failed on two new ShellCheck
diagnostics. The ambiguous logical-marker expression was rewritten as explicit
control flow, and the Polaris trap-only helper became an inline EXIT transaction;
the focused tests, strict lint, full shell suite, `make test`, and `make ci` all
passed afterward. No suppression or weakened test was added.

Local macOS execution is not native-Windows, WSL2, Intel-macOS, redirected-known-
folder, or desktop-GUI runtime proof. Those environments, the exact PR-head CI
results, and hosted Renovate Dashboard ownership remain unavailable/pending at
this entry and are not recorded as greenfield evidence.

## PR #47 required-check repair — entry 5

- Exact failing head: `5d8772fb817cc73ef5ad9b27a43566050e8de0b7`.
- Reproduced CI failures: required `ubuntu` and `e2e containers / ubuntu-24.04`
  both failed at `install-deps.sh:2007` because the dependency-table
  `zsh-plugins` presence scan retained the pre-hardening three-argument call and
  omitted expected origin. Required `chezmoi-parity-windows` completed every
  round-trip assertion successfully, then GitHub failed the step because a
  handled verify-drift exit 1 remained in global `LASTEXITCODE`.
- Root fixes: both dependency-table plugin probes now pass target, origin,
  commit, and required file. Setup/uninstall native adapters still return the
  captured exit explicitly but neutralize global native status after restoring
  the caller preference; the Windows round-trip entry point also declares
  explicit success only after every assertion completes.
- Regression tests: `install_dependency_table_test.sh` checks both complete zsh
  identities and fails on arity/origin drift. Setup/Uninstall Pester now asserts
  handled match, drift, and invocation-error paths leave `LASTEXITCODE=0` while
  retaining their true/false/throw contract. Focused and full rerun results and
  the repair commit identity follow in later append-only entries.

## PR #47 repair identity and local verification — entry 6

- Implementation commit: `d8ac735` (`fix(ci): repair cross-host verification
  contracts`).
- Focused PASS: full `install-deps.sh --dry-run --all` dependency scan;
  `install_dependency_table_test.sh`; accumulator regression; strict ShellCheck;
  PowerShell parser; and Setup/Uninstall Pester 58 passed / 0 failed / 0 skipped.
- Full PASS: `pwsh -NoLogo -NoProfile -File ./test.ps1` matched the analyzer
  fingerprint, passed all 231 Pester tests, and ran every Neovim spec file;
  `make ci` ended with `local pre-PR gate passed`.
- Remote rerun remains pending at this entry. The earlier failing checks are
  retained above as evidence and must be superseded by green exact-head runs,
  not manually rerun on the obsolete head.

## PR #47 Ubuntu Renovate transport repair — entry 7

- Exact failing head: `4e198f5f3278dc4daded670b81fa461b6336514a`.
- Required `ubuntu` progressed past the zsh scan and then failed because
  Renovate's official local extract returned success without creating the
  optional `LOG_FILE`; inventory validation therefore saw no proof file.
- Root fix: run the official extract with JSON logging and capture its stdout as
  the reviewed evidence stream. Stderr remains diagnostic-only. A successful
  command that emits an empty JSON stream now fails explicitly before the
  inventory parser.
- Regression: `renovate_validation_transport_test.sh` stubs a successful,
  output-free extract and proves it fails with the precise missing-proof error;
  the real `make validate-renovate` must still match all 87 inventory records.
  Focused/full results and immutable commit identity follow append-only.

## PR #47 setup and Windows integration repair — entry 8

- Exact failing head: `4e198f5f3278dc4daded670b81fa461b6336514a`.
- Required Ubuntu and macOS setup reproduced Homebrew's documented idempotent
  behavior: `brew shellenv` exits 0 with empty stdout when its bin/sbin already
  lead PATH. Setup incorrectly recorded that valid existing state as failure.
  It now evaluates only successful nonempty output and accepts empty output only
  after canonical executable proof; empty output with brew absent still fails.
- The same setup logs exposed a distinct Lazy restore assertion. The verified
  lazy.nvim checkout was intentionally detached, but its shallow exact-commit
  fetch had no `origin/HEAD`, so Lazy could not serialize a branch into its lock.
  Staging now records and proves the locked branch remote ref and symbolic
  origin HEAD at the exact locked commit before publication. The checkout stays
  detached and no branch tip is trusted for execution.
- Required Windows reproduced canonicalization and proof-environment defects:
  `Get-RealExistingPath` handled symbolic links but not directory junctions, so
  valid redirected Neovim targets appeared divergent; the generic suite also
  lacked the real chezmoi executable needed by its native drift tests. Junctions
  now resolve through their targets, and CI installs the exact existing
  version/checksum-pinned Windows chezmoi asset before Pester.
- Focused PASS on macOS: Homebrew shellenv regression; full Neovim suite; fresh
  isolated production `Lazy! restore` with 30 locked plugins and no assertion;
  Setup/Uninstall Pester 59 passed / 0 failed / 0 skipped; supply-chain scanner,
  pin consistency, YAML lint; Renovate empty-transport oracle and real inventory
  extraction (87 exact records). Native Windows junction proof and exact-head
  setup results remain pending until the pushed CI rerun.
- Implementation commit identity and full-gate/exact-head results follow in the
  next append-only entry.

## PR #47 Windows Tree-sitter stage repair — entry 9

- Exact failing head: `4e198f5f3278dc4daded670b81fa461b6336514a`;
  `setup.ps1 / windows-2025` downloaded and checksum-verified the exact
  Tree-sitter `v0.26.10` archive, validated its extracted `tree-sitter.exe`, then
  failed only after copying those bytes to `.tree-sitter.exe.stage.<guid>`.
- Root cause: Windows dispatches a native executable by its final extension.
  The suffix after `.exe` made the same bytes non-executable, so the staged
  version proof returned empty. This was a publication-shape bug, not an asset,
  checksum, ABI, or version-predicate mismatch.
- Root fix: the same-parent transactional stage is now
  `.tree-sitter.stage.<guid>.exe`; exact-version validation still occurs before
  atomic replace/move and after publication, with the old target retained or
  restored on failure.
- Regression: the stale/partial repair oracle now records every staged/source/
  target validation path and returns incompatible for any path whose final
  extension is not `.exe`, reproducing the old failure boundary. Native Windows
  rerun evidence remains pending until this repair is pushed.

## PR #47 explicit Intel Nix installer selection — entry 10

- Exact observed head: `4e198f5f3278dc4daded670b81fa461b6336514a`.
  The real `macos-26-intel` setup lane launched on an x86_64 GitHub host and
  reached nix-darwin/Homebrew/editor setup. Its Determinate action post-step
  reported that current Determinate Nix no longer supports Intel hosts and had
  automatically pinned the last compatible installer as a temporary fallback.
- Official/current reconciliation: GitHub still offers `macos-26-intel`; Nix's
  upstream installer supports multi-user macOS; current Determinate documents
  Apple Silicon, not Intel, as its supported macOS host. Intel is therefore a
  genuine platform-specific bootstrap difference, not grounds to narrow the
  product contract.
- Implementation: Intel matrix rows alone select
  `cachix/install-nix-action@a49548c11d9846ad46ecc0115273879b045f001c`
  (`v31.10.7`), whose reviewed composite pins upstream Nix `2.34.8` at a
  versioned `releases.nixos.org` URL. Other rows retain the full-SHA Determinate
  action. A static oracle proves both conditions in both workflows; the general
  external-action SHA scanner and Renovate inventory cover the new dependency.
- This mechanism is implemented but not runtime proof. A green exact final-head
  Intel setup and flake run are still required before UGR-002 can be classified
  fully proved.

## Intel Darwin support-window evidence — entry 11

- Local `nix flake check --print-build-logs` passed after evaluating both Darwin
  configurations, but emitted Nixpkgs' official warning that 26.05 is the final
  `x86_64-darwin` release. Official release notes state package/platform support
  continues until 26.05 reaches end of support on 2026-12-31; 26.11 will not
  build or support Intel Darwin packages from source.
- Final classification implication: UGR-002's current macOS 26 Intel path is a
  feasible supported implementation today and must still receive exact-head
  runtime proof. It also carries a dated residual: migrate Intel's package plane
  before 2026-12-31 without narrowing the public macOS contract.
- No warning suppression was added. `intel_nix_installer_test.sh` rejects
  `allowDeprecatedx86_64Darwin` so future work cannot manufacture a quieter
  green run while the support deadline is being ignored.

## Final repair local verification — entry 12

Executed on 2026-07-10 in the isolated macOS repair worktree after the
Renovate, Homebrew, Lazy, Windows junction/chezmoi, Tree-sitter stage, and Intel
Nix installer changes. The implementation commit identity follows append-only
after Git creates it.

| Command / suite | Result |
|---|---|
| `git diff --check` | PASS |
| `bash -n` over every repository `*.sh` | PASS, 132 scripts |
| repository shell lint (`make lint`) | PASS, strict ShellCheck |
| `bash tests/static/run_all.sh` | PASS, including action SHA, Intel installer, Nix ownership, supply-chain, pin, workflow, JSON/TOML/YAML, and invariant guards |
| `bash tests/shell/run_all.sh` | PASS, including Homebrew empty-output and Renovate empty-transport regressions |
| `make test-migration` | PASS: template, parity, round-trip, uninstall safety/order, Windows render, checked zsh publisher, and verify-drift oracle |
| PowerShell parser (`tests/static/ps1_parse.sh`) | PASS for every tracked PowerShell surface |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: analyzer exact fingerprint, Pester 232 passed / 0 failed / 0 skipped, all 17 Neovim spec files exited 0 |
| Focused Windows Terminal Pester filter | PASS: 19 passed / 0 failed / 0 skipped |
| `make test-nvim` | PASS, including 9 locked-checkout, 5 checked-delete, and real two-project clangd cases |
| Fresh isolated production `nvim --headless "+Lazy! restore" "+qa"` | PASS with a clean XDG/HOME tree; no lock assertion or init/command callback error |
| `make validate-renovate` | PASS: strict validator plus official local extraction exactly matched 89 reviewed records |
| `nix flake check --print-build-logs` | PASS on aarch64-darwin; both Darwin configurations evaluated, host check reused cache, incompatible systems were honestly reported as omitted, and the Intel 26.05 support warning remains visible |
| `make test` | PASS |
| `make ci` | PASS, ended `local pre-PR gate passed` |

The initial combined static run failed only because the invariant still required
the superseded `locked_commit` call string. It was strengthened to require
`locked_identity` plus `branch = lazy_branch`; static, test, and CI gates then
passed. No skip, allowlist, suppression, or weakened assertion was added.

These local results are not native-Windows junction/Tree-sitter proof, real
Intel proof for the new upstream-Nix action, WSL2, redirected known-folder,
desktop-GUI, or hosted Renovate Dashboard proof. Those remain pending the
pushed exact-head runs or manual environments.

## Homebrew failure-state hardening — entry 13

- Before commit, the shellenv repair received one final transactional audit.
  Syntax/command failure during evaluated shellenv output or failure to activate
  the selected brew now restores the exact prior PATH, MANPATH, INFOPATH,
  HOMEBREW_PREFIX, HOMEBREW_CELLAR, and HOMEBREW_REPOSITORY set/unset state,
  clears the command hash, and prints an explicit repair/retry instruction.
- The focused fixture injects output that mutates PATH and HOMEBREW_PREFIX before
  executing `false`; setup must reject it, restore both prior states, and emit
  recovery evidence. This complements the nonzero-command, valid-empty, and
  empty-without-active-brew cases recorded above.
- Post-hardening `make ci` passed again and ended `local pre-PR gate passed`;
  strict ShellCheck and the focused Homebrew regression also passed directly.

## Final repair implementation identity — entry 14

- Implementation commit: `d2e2149bc2c41eb9c38eaae8afe77912397b4386`
  (`fix(ci): close live cross-platform regressions`).
- The commit binds the entry 7-13 repairs, their focused regressions, and their
  Markdown contract updates into one immutable behavioral identity.
- Exact-head hosted verification is not inferred from the local PASS results in
  entry 12. It remains pending until PR #47 executes this commit (or a later
  documentation-only descendant) on Ubuntu, macOS Apple Silicon, macOS Intel,
  and Windows.

## Exact-head cross-platform reconciliation — entry 15

- Exact head: `7a446c31def84bdef6da11b23dab21f79ca13336`.
  Twelve checks passed, including all three Nix evaluations, generic macOS and
  Windows, all three chezmoi parity jobs, and the Ubuntu container. Four primary
  jobs failed and therefore the staged logical proof jobs correctly failed too.
- Required Ubuntu failed only in `zsh_plugins_test.sh`: the preview did not print
  either reviewed tag, but bare `[[ ... ]]` assertions had been ineffective on
  local Bash 3.2 and became real failures on modern Bash. Preview now states
  reviewed tag plus exact commit; every assertion has an explicit portable
  failure branch. The focused test passes on Bash 3.2 and Ubuntu 24.04.
- Required Ubuntu setup completed all six phases, then its clean login zsh could
  not resolve Home Manager `rg`. The pinned Home Manager source explicitly
  documents a third canonical
  `/etc/profiles/per-user/$USER/etc/profile.d/hm-session-vars.sh` location.
  Managed zsh now checks that path using `id -un` after the XDG and legacy
  per-home paths. Focused custom-HOME/idempotence/no-profile coverage passes;
  exact hosted proof remains pending.
- Non-required Intel setup installed upstream Nix on a real x86_64 host, built
  the selected `dotfiles-x86_64` configuration, and reached activation.
  nix-darwin then refused the runner's unrecognized `/etc/bashrc` and
  `/etc/zshrc`, exactly instructing `.before-nix-darwin` preservation. First
  bootstrap now preflights both backup names, moves neither on collision,
  preserves both before activation, restores both on ordinary/partial/signal
  failure, and quarantines generated replacements. Focused success, collision,
  partial-move, rollback, signal, and retry tests pass.
- Required Apple Silicon setup completed all phases but emitted two false
  Homebrew failure markers. nix-darwin selected
  `/run/current-system/sw/bin/brew`; its valid `shellenv` activated
  `/opt/homebrew/bin/brew`. Those paths name one installation, so pathname
  equality was the wrong predicate. Setup now proves canonical prefix plus
  repository for selected and active commands, persists the reported native
  prefix, and retains transactional environment restoration. Wrapper/native,
  empty, mismatch, command failure, partial evaluation, and retry regressions
  pass. A required Brew-less macOS bootstrap/activation failure now reaches the
  consolidated nonzero summary before any package install instead of degrading
  to an unexplained unknown-manager exit.
- Required Windows setup installed the exact Tree-sitter artifact successfully,
  then `chezmoi apply` exited 1. The PowerShell adapter had captured stderr but
  failed to print it, so that run cannot prove a more specific root. The main
  source already exposes no WT target; setup now applies it without an absolute
  Windows target selector list and prints captured native stderr on failure.
  The Pester apply-boundary oracle passes. A new native rerun is required and
  any remaining diagnostic will be treated as a new root cause.
- Implementation commit identity, complete local gates, and replacement hosted
  results follow append-only. None of these repairs is yet claimed as hosted
  proof from local execution.

## Repair identity and local verification — entry 16

- Behavioral repair commit:
  `bd46346f9630b359d26efd909525c9c35356f478` (`fix(ci): reconcile
  exact-head platform evidence`). This commit contains the implementation,
  focused regressions, and contemporaneous README/CLAUDE/ROADMAP/status/manual
  updates described in entry 15.
- Passed on macOS aarch64: `git diff --check`; `bash -n` over all 132 tracked
  shell scripts; `make lint`; `bash tests/static/run_all.sh`;
  `bash tests/shell/run_all.sh`; `make test-migration`; the full PowerShell
  `test.ps1` entry point (PSScriptAnalyzer, 234/234 Pester tests, and all
  Neovim specs); focused Setup Pester (52/52, including the Windows Terminal
  transaction and native-stderr subprocess); `make test-nvim`; `make test`;
  `make validate-renovate` (official validator plus 89 extracted dependency
  records); `nix flake check`; and `make ci`.
- The local Nix check evaluated both exported Darwin configurations and passed
  the native aarch64 check. Nix explicitly omitted incompatible-system builds,
  so this is configuration evidence, not Intel/Linux runtime proof.
- Hosted proof remains pending for the documentation-only descendant that
  contains this entry. WSL, redirected Windows known folders, desktop GUI
  behavior, and live Renovate ownership remain separate manual/live evidence
  requirements; no cache-backed or static result is promoted to those claims.

## Generic Ubuntu clangd runtime dependency — entry 17

- Exact head `0c853d066362602f14dc251a6d3fbf3980102048`, run
  `29090161177`, job `86353322008`, passed install, static, Renovate,
  shellcheck, shell, Starship, and tmux phases. Neovim then passed the locked
  bootstrap and checked-delete specs but failed the two-project clangd spec at
  its explicit real-binary precondition: `vim.fn.executable("clangd") == 0`.
- This disproves neither the per-client clangd implementation nor its local
  runtime result. It proves the generic Ubuntu lane omitted the real runtime
  dependency, so it could not own the claimed cross-platform proof.
- The Ubuntu lane now installs the distro `clangd` package before Neovim tests.
  `clangd_runtime_ci_test.sh` binds that provisioning to the spec's real-binary
  and distinct-client assertions. Static/focused local results follow in the
  implementation commit; hosted runtime confirmation remains pending a rerun.

## Canonical Home Manager session path — entry 18

- Exact head `0c853d066362602f14dc251a6d3fbf3980102048`, run
  `29090161175`, job `86353321538`, completed Home Manager activation and all
  six setup phases without a setup `FAIL:` marker. Its post-install clean
  `env -i ... zsh -l -i -c 'command -v rg'` assertion still failed.
- The pinned Home Manager source evaluates `home.profileDirectory` to
  `/home/runner/.nix-profile`, but this repository evaluated
  `home.sessionPath` to `[]`. Therefore the canonical `hm-session-vars.sh`
  existed yet did not export the profile `bin`; adding another lookup path in
  zsh could not make `rg` resolvable from a caller-stripped PATH.
- Standalone Linux Home Manager now sets `home.sessionPath` to the evaluated
  `home.profileDirectory/bin`. This keeps chezmoi as the only zshrc owner while
  making Home Manager's canonical session-vars file carry its own package path.
  Both Linux architecture evaluations assert the exact relationship. Hosted
  clean-login confirmation remains pending the next run.

## Hosted AeroSpace proof boundary and Intel Pi path — entry 19

- Exact head `0c853d066362602f14dc251a6d3fbf3980102048`, run
  `29090161175`: Apple Silicon completed setup and reached only the AeroSpace
  query, which returned no config path; Intel completed setup but failed earlier
  because the separate post-install shell omitted `~/.local/bin`, where the
  verified Pi tarball installation had published `pi` successfully.
- The pinned AeroSpace cask is `0.21.1-Beta`. Its tag peels to upstream commit
  `cfd4eab235b254ff5f1a1b9180a3997ae060162a`; that exact source calls
  `waitForAccessibilityPermission_nonCancellable()` before both user-config
  parsing and `startUnixSocketServer()`. The wait loop exits only after
  `AXIsProcessTrusted*` succeeds. Therefore a headless hosted launch cannot
  validate config consumption and must not be represented as runtime proof.
- Hosted macOS now invokes the installed app and CLI and requires their
  version/hash identities to agree, then prints an explicit `UNAVAILABLE`
  classification for TCC-backed config consumption. The real managed-path and
  warning-as-error reload checks remain in `tests/MANUAL.md` for a user-granted
  desktop session. Ghostty and WezTerm retain their real binary config checks.
- The post-install shell now prepends `~/.local/bin`, matching the managed zsh
  consumer path, before validating verified user-local artifacts such as Pi.
  `macos_gui_runtime_ci_test.sh` rejects both the old false AeroSpace claim and
  future loss of the consumer path. Exact hosted confirmation remains pending.

## Native Windows exact-head result — entry 20

- Exact head `0c853d066362602f14dc251a6d3fbf3980102048`, run
  `29090161175`, job `86353321527`, and its stable logical proof job both
  passed on `windows-2025`.
- The public setup completed phases 1-6 and emitted no `FAIL:` marker. Runtime
  assertions accepted the checksum-verified Tree-sitter `0.26.10` release
  artifact, installed Hack Nerd Font files and found their application-visible
  registry registration, accepted Pi `0.80.3`, restored Lazy, built the parser
  set, synchronized Mason, verified Polaris, and completed the strict
  257-check Neovim LSP/parser/formatter smoke.
- This is native Windows hosted proof for those conventional-path assertions.
  It is not a redirected Documents/LocalApplicationData run, a divergent
  packaged-plus-portable Windows Terminal transaction/uninstall run, or manual
  desktop GUI proof. Those residuals remain open and are not inferred from the
  green job.

## Runtime follow-up identity and local verification — entry 21

- Behavioral implementation commit:
  `f89f61c2dc33a99bbd5921fc4b6a577843aa5348` (`fix(ci): close
  exact-head runtime regressions`). It binds the generic Ubuntu clangd runtime,
  canonical standalone Home Manager session path, user-local post-install
  consumer path, and honest AeroSpace TCC proof boundary to their focused tests
  and documentation.
- Focused PASS: `clangd_runtime_ci_test.sh`,
  `macos_gui_runtime_ci_test.sh`, `linux_home_test.sh` for both Linux
  architectures, Home Manager session-vars shell coverage, strict ShellCheck,
  YAML lint, and `git diff --check`.
- Full PASS: `nix flake check --print-build-logs` evaluated both Darwin and
  Linux exported configurations and built the native toolchain check; it kept
  the Intel support-window warning visible and honestly omitted incompatible
  system builds. `make ci` ended `local pre-PR gate passed`, including strict
  lint, all Neovim specs (real two-project clangd included), shell/static/Nix
  suites, Renovate validator plus 89-record official extraction, and the full
  migration/parity/oracle bundle. `bash -n` passed for all 134 repository shell
  scripts outside generated caches.
- The first `make ci` attempt failed only because the new static oracle's
  literal workflow patterns triggered ShellCheck SC2016. The patterns were
  rewritten with escaped expansion markers; no suppression or lint weakening
  was added, and the complete gate then passed.
- Hosted confirmation is not inferred from these local results. It requires the
  exact pushed descendant on Ubuntu, Apple Silicon, Intel, and Windows; WSL,
  redirected Windows, divergent dual Terminal state, and TCC-enabled AeroSpace
  config consumption remain separate manual/runtime dependencies.

## Native-Linux login-shell oracle correction — entry 22

- Exact head `28006783a5112bfa3af3b0deb2f59fbf9f457a4e`, run
  `29091430087`, job `86357442860`, completed Home Manager activation and all
  six public setup phases. Its post-install shell check then failed with the
  same high-level message as the prior run.
- The new run disproved the prior root-cause attribution. Setup's own dependency
  table recorded zsh missing, installed
  `/home/linuxbrew/.linuxbrew/bin/zsh`, and changed the account login shell to
  that path. The assertion nevertheless invoked `/usr/bin/zsh`, which did not
  exist, and redirected the decisive diagnostic to `/dev/null`. It never
  exercised `hm-session-vars.sh` or `rg` resolution.
- The oracle now resolves the effective account with `id -un`, requires exactly
  one `getent passwd` record, requires that record's shell be executable zsh,
  and runs that exact shell as login+interactive under `env -i`. Stderr is
  captured and printed on failure, stdout must be one nonempty command path,
  and the resolved executable must still land in `/nix/store`.
- `home_manager_session_vars_test.sh` binds the workflow to the account record,
  rejects the old hardcoded path, and requires preserved failure diagnostics.
  The canonical Home Manager session-path configuration remains necessary and
  unchanged. Hosted behavior confirmation requires the next exact-head run.
- Implementation commit: `8a09cf3`. Focused proof passed via
  `bash tests/shell/home_manager_session_vars_test.sh`; repository YAML lint,
  full shell lint, policy/check-identity tests, `git diff --check`, Bash syntax
  over all 134 tracked shell scripts, and the complete `make ci` pre-PR gate
  also passed on the identical implementation tree before the commit was
  created. The direct invocation of ShellCheck on the whole YAML document and
  raw default-profile `yamllint` were rejected as invalid test commands; the
  repository-aware equivalents above are the evidence.

## Exact behavior-head closure — entry 23

This entry is the final authoritative classification for the implemented
behavior. It supersedes every earlier pending runtime classification without
rewriting that history.

- Base: `85375b2bdec9d3a998e8023a44b41d03a32f3eaa`.
- Exact behavior head:
  `f4b63953f2f982702a685358b09e89bae2d78fdd`.
- Hosted runs: generic/parity `29092384006`, Nix `29092384007`, and public
  setup/e2e `29092384014` all completed successfully.
- Runtime evidence: real Ubuntu completed Home Manager and all six phases, used
  the account-record Linuxbrew zsh under `env -i`, resolved Nix-owned `rg`, and
  passed the 257-check smoke. Real Intel installed upstream Nix 2.34.8, selected
  `dotfiles-x86_64`, kept the Nixpkgs 26.05 sunset warning visible, completed
  nix-darwin plus all six phases, and passed the same post-install smoke. Apple
  Silicon and native Windows completed their full public setup paths. The
  generic Ubuntu job passed the real two-project clangd isolation spec (1/1,
  zero failures).
- Proof boundary: PR setup caches were enabled. These runs are hosted runtime
  evidence, not cache-free scheduled/manual, WSL, redirected-Windows,
  divergent-dual-Terminal, owner-host tap rollback, or desktop GUI/TCC proof.

### Finding-by-finding final status

| ID | Final status | Evidence and implementation commit(s) | Focused/full tests | Documentation | Residual/manual proof |
|---|---|---|---|---|---|
| UGR-001 | ACCEPTED/FIXED | Reproduced destructive packaged-to-portable mirroring. `f1c9e2c` makes every Terminal installation an independent staged, validated, concurrency-checked, atomic merge/recovery target. | 19 focused Setup/Uninstall Pester cases plus Windows render/apply/round-trip oracles; native Windows workflows green. | README, Windows Terminal README, CLAUDE, ROADMAP, MIGRATION_STATUS, this ledger. | Real divergent packaged+portable install and dual uninstall restoration remain manual. |
| UGR-002 | ACCEPTED/FIXED | `1423a47` exports and exactly selects both Darwin architectures; `d2e2149`, `bd46346`, and `f89f61c` close the real Intel installer/bootstrap/consumer regressions. Intel Nix job `86360593091` and setup job `86360593153` passed. | Both Darwin evaluations, selection/unsupported-arch, Homebrew/tap transaction/dry-run suites; real Intel Nix/setup and 257 smoke passed. | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL, greenfield ledger. | Nixpkgs 26.05 support ends 2026-12-31; owner-host taps and desktop GUI remain manual. |
| UGR-003 | ACCEPTED/FIXED | `60dd01a` requires locked full-SHA proof of Git/origin/HEAD/clean/worktree/entrypoint before Lazy or Plenary reaches runtimepath, with locked sibling repair and rollback. | 9 behavioral bootstrap cases, all Neovim specs, generic Ubuntu/macOS/Windows green. | README troubleshooting, CLAUDE, ROADMAP, this ledger. | None beyond normal cross-host operation; no unproved path was executed by tests. |
| UGR-004 | ACCEPTED/FIXED | `aa48aad` routes every recoverable POSIX main-flow installer through one failure accumulator and preserves later sentinel work plus one nonzero summary. | Failure injection for all artifact/catalog families; full shell/static and Ubuntu container/setup green. | README, CLAUDE, ROADMAP, MIGRATION_STATUS. | Real third-party outage behavior is necessarily host/network evidence. |
| UGR-005 | ACCEPTED/FIXED | `aa48aad` makes POSIX and Windows install only a locally packed tarball whose bytes match the pinned SRI, then validates version and cleans every exit. | POSIX and Pester mismatch/network/partial/install/cleanup/retry cases; native Windows and POSIX setup green. | README, CLAUDE, ROADMAP, MANUAL, pin docs. | None for the conventional hosted paths; local Pi auth/session state remains intentionally unmanaged. |
| UGR-006 | ACCEPTED/FIXED | `aa48aad` neutralizes unproved sourceable payloads, verifies same-parent staging, publishes atomically, quarantines bad state, and self-heals legitimate pin changes. | Oracle, concurrency, pin-change, failure-neutralization, and chezmoi fingerprint cases. | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL. | A real offline bare-chezmoi pin transition remains manual. |
| UGR-007 | ACCEPTED/FIXED | `aa48aad` defines exact Tree-sitter 0.26.10 compatibility and repairs only stale/partial/incompatible state from architecture-specific verified release bytes. | Current/stale/partial/unmanaged/architecture/checksum/publication/dry-run Pester plus pin consistency; native setup passed. | README, CLAUDE, ROADMAP, MANUAL. | None for the hosted x64 path; ARM/x86 Windows runtime remains environment-specific. |
| UGR-008 | ACCEPTED/FIXED | `aa48aad` verifies the exact Microsoft repository `.deb` before privileged `dpkg` and extends the generic remote-to-root execution scanner. | Positive/negative scanner self-tests; required Ubuntu generic/setup jobs passed. | Supply-chain security doc, README, CLAUDE. | None. |
| UGR-009 | ACCEPTED/FIXED | `aa48aad` binds gh-dash tag object to peeled commit, reuses the production Terminal pin in Sandbox, full-SHA-scans every external action, and checks in desired SHA enforcement. | Moved-tag rejection, Sandbox/pin consistency, action scanner self-tests. | Supply-chain and branch-protection docs, README, CLAUDE. | Live `sha_pinning_required` remains false; owner applies only after the staged ruleset migration. |
| UGR-010 | ACCEPTED/FIXED | `eac92bc` isolates native-command preference with `try/finally`, distinguishes expected drift from invocation failure, and preserves explicit stderr/exit handling. | Setup/Uninstall Pester under both preference states, drift, stderr, spaces, backup creation/restoration; Windows workflows green. | README, CLAUDE, MIGRATION_STATUS. | None for tested hosts. |
| UGR-011 | PARTIAL | `1423a47` adds canonical Home Manager session-vars sourcing, `f89f61c` exports the evaluated profile bin, and `8a09cf3` executes the actual account-record zsh. Native Linux job `86360593139` passed. | Custom HOME, missing profiles, repeated sourcing, both Linux evaluations, exact hosted clean-login proof. | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL, greenfield ledger. | Real WSL userland proof remains required; native Linux is proven. |
| UGR-012 | ACCEPTED/FIXED | `1423a47` models post-bootstrap brew availability during dry-run and continues all later phases without claiming installation. | Complete Brew-less Darwin preview, noninteractive, retry, and failure-plan cases; macOS workflows green. | README, CLAUDE, ROADMAP. | None. |
| UGR-013 | ACCEPTED/FIXED | `1423a47` makes tap migration transactional across activation/bootstrap/publication/signals, with collision and rollback-failure recovery; `bd46346` adds shell-file bootstrap rollback exposed by Intel. | Existing-rebuild and first-bootstrap branches, collision, partial move, signal, rollback failure, retry. | README, CLAUDE, ROADMAP, recovery docs, MANUAL. | Failure injection against a real owner host with existing taps remains manual. |
| UGR-014 | ACCEPTED/FIXED | `f1c9e2c` selects only validated filename timestamps/collision suffixes and fails on malformed or ambiguous candidates, never mtime. | POSIX files/directories/opposing mtimes/collisions/malformed plus Windows equivalent Pester. | README, CLAUDE, ROADMAP, MIGRATION_STATUS. | Real Windows dual-target restoration is covered by UGR-001's manual row. |
| UGR-015 | PARTIAL | `1423a47` establishes one POSIX account/home boundary; `eac92bc` resolves Windows UserProfile, LocalApplicationData, Documents, and runtime profiles with legacy migration and post-consumption checks. | POSIX identity suite; 58 Setup/Uninstall Pester cases; Windows template/apply/round-trip/parity; conventional Windows workflows green. | README, CLAUDE invariants 24/25, MIGRATION_STATUS, MANUAL. | Real redirected/OneDrive/alternate-drive Windows migration and rollback remain required. |
| UGR-016 | ACCEPTED/FIXED | `eac92bc` guards on real argv, redirected I/O, CI, user-interactive state, and supported hosts before cache/filesystem work. | 30 Profile Pester cases including real subprocesses, preference states, and normal host behavior; Windows workflow green. | README troubleshooting, CLAUDE invariant 26, MANUAL. | Interactive VS Code/ISE visual confirmation remains manual but does not block the fail-closed implementation. |
| UGR-017 | ACCEPTED/FIXED | `eac92bc` centralizes data-root-scoped deletion, checks return plus absence, and fails synchronous cleanup without touching built-in runtime paths. | 5 checked-delete and 18 Tree-sitter behavioral cases; all Neovim specs green on Ubuntu/macOS/Windows. | README, CLAUDE invariant 19. | None. |
| UGR-018 | ACCEPTED/FIXED | `eac92bc` removes startup-cwd compile-database freezing; `f89f61c` provisions real clangd in CI. | Real two-project/one-session spec passed on Ubuntu (job `86360593114`), plus 24 LSP specs. | CLAUDE LSP workflow, ROADMAP, MANUAL. | Interactive editor confirmation remains manual; runtime client isolation is automated and passed. |
| UGR-019 | PARTIAL | `eac92bc` enables the Nix manager, corrects Scoop to `master`, restores behind-base rebasing, exposes matrix runners, and validates an exact 89-record official extraction. | Regex matchability, expected inventory, official validator, and local dry-run extraction passed. | README, CLAUDE, ROADMAP, this ledger. | Dashboard #7 last reran against default branch at 2026-07-10 12:05 UTC and still shows old no-Nix/Scoop-main state; post-merge bot proof is pending. |
| UGR-020 | PARTIAL | `eac92bc` implements deadlock-free stage 1: six stable logical checks verify exact artifact/run/head proof while all 12 legacy required contexts remain. All six logical jobs passed on behavior head. | Marker tamper/missing/duplicate tests, workflow metadata/alignment, and live logical jobs `86361159946`, `86361159987`, `86363763769`, `86363763779`, `86363763791`, `86363763792`. | README, CLAUDE invariant 27, branch-protection runbook, MIGRATION_STATUS. | After merge: observe default-branch logical checks, merge a context-switch PR, then owner applies live safeguards. |
| UGR-021 | PARTIAL | `1423a47`, `eac92bc`, and `f89f61c` make scheduled/manual setup cache-free, keep WSL fail-visible/non-required, add real binary checks where credible, assert Windows fonts, and state AeroSpace TCC unavailability honestly. | Cache contract, GUI/runtime source guards, required-check alignment; exact hosted Ubuntu/ARM/Intel/Windows setup passed. | README, CLAUDE, MANUAL, greenfield ledger. | Cache-free scheduled/manual, WSL, redirected Windows, dual Terminal, and desktop/TCC visual runs remain required; PR-cache results are not promoted. |
| UGR-022 | ACCEPTED/FIXED | `5d8772f` and append-only follow-ups repair the main baseline, PR #46 state, timing claim, setup flags, safeguards truth, and every status changed by this implementation. | Documentation/static/help/required-check guards and full `make ci`. | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL, security docs, both ledgers. | Future statuses must be updated after the staged safeguards PRs and manual runs. |
| UGR-023 | ACCEPTED/FIXED | `eac92bc` plus `aa48aad` make Starship cache and Polaris staging transactional, analyzer identities exact, JSON traversal NUL-safe, shell lint strict, Nix ownership structural, and direct-artifact updates compatibility-aware. | Profile/Polaris/JSON-path/Nix-scanner/analyzer/direct-artifact tests; full local and hosted gates green. | README, CLAUDE, ROADMAP, MIGRATION_STATUS. | None beyond the platform/manual rows already named. |

### Verification classification

| Classification | Exact result |
|---|---|
| Passed locally | `git diff --check`; Bash syntax over 134 tracked shell scripts; repository shell lint; `tests/static/run_all.sh`; `tests/shell/run_all.sh`; migration/parity/round-trip/uninstall/oracle bundle; PSScriptAnalyzer plus 234/234 Pester and all 17 Neovim spec files through `test.ps1`; focused Windows Terminal Pester 19/19; `make test-nvim`; `make test`; `make validate-renovate` with 89 exact records; `nix flake check --print-build-logs`; `make ci`. |
| Passed hosted on behavior head | All six generic/parity jobs in run `29092384006`; all three architecture Nix jobs and two stable logical Nix jobs in `29092384007`; Ubuntu container, Ubuntu/Apple-Silicon/Intel/Windows setup, and four stable logical setup jobs in `29092384014`. |
| Skipped intentionally | Local Nix omitted incompatible-system builds while still evaluating every exported configuration; the WSL canary was not made a required PR check. |
| Unavailable | Local native-Windows, WSL2, Intel hardware, redirected known folders, divergent dual Terminal, and user-granted desktop TCC/visual execution. Hosted AeroSpace config consumption was explicitly classified unavailable rather than passed. |
| Pending live/manual | Renovate default-branch bot result; cache-free scheduled/manual lanes; WSL; redirected Windows; dual Terminal recovery; owner-host tap rollback; desktop GUI/TCC; stage-2 ruleset migration. |

### Live repository relationship at the behavior head

- `origin/main` remained
  `85375b2bdec9d3a998e8023a44b41d03a32f3eaa`; PR #47 was the only open PR.
- The active integrity ruleset still required the exact 12 legacy contexts and
  strict behind-main status. Every one passed on the behavior head.
- The active review ruleset still required one code-owner/last-push approval and
  thread resolution. The PR was `MERGEABLE` but correctly `BLOCKED` pending
  independent review.
- Live Actions policy remained `sha_pinning_required:false`; the checked-in
  target is true. No live safeguard was mutated.

## Post-merge cache-free reconciliation — entry 24

This append-only entry records the first agreed follow-up after PR #47 merged.
It does not rewrite the exact behavior-head closure above.

- Merged-main base:
  `5e3e7c6d93c400d67f6160c6f8f09be56aac10d3` (PR #47 squash merge).
- Manual cache-free workflow-dispatch run:
  [`29096335827`](https://github.com/luisgui1757/dotfiles/actions/runs/29096335827).
  Every broad install/plugin cache step was skipped.
- Attempt 1: Ubuntu container `86373717048`, public Ubuntu `86373717119`, and
  native Windows `86373717139` passed. Apple Silicon `86373717142` failed the
  strict smoke after Lazy reported the nvim-treesitter build complete while
  compiler output continued; only 98/99 languages completed and Pascal had no
  captures. Intel `86373717122` independently failed transient
  `api.github.com` DNS resolution and restored the original system shell files
  and Homebrew taps.
- Attempt 2, same unrepaired SHA: Apple Silicon `86378834721` and logical macOS
  `86382233846` passed. Intel `86378834701` installed 99/99 parsers but failed
  because the original CMake fixture's neocmake client did not attach within 45
  seconds; the later formatter CMake fixture did attach. This timing-dependent
  retry does not validate code absent from that SHA and leaves the full
  cache-free matrix open.
- Alternative hypothesis check: Neovim `0.12.4`, Tree-sitter CLI `0.26.10`,
  locked nvim-treesitter commit
  `4916d6592ede8c07973490d9322f187e07dfefac`, and locked Pascal revision
  `042119eca2e18a60e56317fb06ee3ba5c32cb447` built successfully in a separate
  clean runtime and produced ten captures. Deterministic Pascal parser/query
  incompatibility was therefore rejected; the hosted log's overlapping
  publication is the reproduced cause.
- Behavioral repair commit:
  `fc22028d2a9ba5ddddb8343ba64fc7d208c8fee7`. The Lazy build hook now bypasses
  the command load trigger, calls nvim-treesitter's waitable update API with
  `max_jobs = 1`, waits up to 15 minutes, and requires exactly `true` before
  restore can advance. Phase 4 remains the explicit complete parser install.
- Focused regression: the two new Tree-sitter behaviors first failed against
  command-form `build = ":TSUpdate"`, then passed after the repair. Final
  Tree-sitter spec result was 20 passed, 0 failed; `make test-nvim` passed.
- Full local proof on the identical behavioral tree: `git diff --check`; Bash
  syntax for every tracked shell script; repository shell lint;
  `tests/static/run_all.sh`; `tests/shell/run_all.sh`; PowerShell parsing and
  234/234 Pester with 0 skipped plus all 17 Neovim specs through `test.ps1`;
  `make test-migration`; `make test`; `make validate-renovate` with exactly 89
  records; `nix flake check --print-build-logs`; and `make ci` ending
  `local pre-PR gate passed`.

### Finding status amendments

| ID | Status after entry 24 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-019 | ACCEPTED/FIXED | Dashboard #7 reran against merged `main` at 2026-07-10 13:17 UTC and now exposes the `nix` manager inventory, `macos 26-intel` runner labels, and `ScoopInstaller/Install master`; no lookup-problem section remains. Local official extraction remains exactly 89 reviewed records. | Normal future Renovate operation only; do not turn this bot result into greenfield host evidence. |
| UGR-020 | PARTIAL | Stage 1 logical checks exist, but cache-free merged-main run `29096335827` did not produce a fully green matrix. The active integrity ruleset and classic fallback remain strict on the exact 12 legacy contexts. | Merge the repair, pass all six logical checks on its merged-main SHA, then open the separate checked-in context-switch PR. Only after that PR merges may the owner apply live safeguards. |
| UGR-021 | PARTIAL | The cache-free lane ran and exposed a real asynchronous Tree-sitter publication race. Commit `fc22028d2a9ba5ddddb8343ba64fc7d208c8fee7` repairs the boundary with behavioral proof. | A workflow-dispatch run on the repair's merged-main SHA must pass every producer and logical check. WSL, redirected Windows, dual Terminal, and desktop/TCC proof remain separate. |

### Live safeguard boundary after entry 24

- Integrity ruleset `17363189` and the classic fallback still require the exact
  12 legacy contexts with strict behind-main enforcement.
- Actions remains enabled with `sha_pinning_required:false`; checked-in desired
  state remains true for the later owner-applied migration.
- No ruleset, classic protection setting, or Actions policy was mutated by this
  follow-up.

## Branch-head cache-free second-root repair — entry 25

This append-only entry records evidence discovered after entry 24 and the
corresponding behavioral repair. It does not promote a branch run to
merged-main greenfield proof.

- PR #48's first required Ubuntu container job, run `29100012131`, job
  `86386173483`, failed while installing Ghostty. The repo had authenticated the
  bytes of upstream `install.sh`, but that reviewed script then queried mutable
  `releases/latest`, selected a release asset at runtime, downloaded an
  unchecked `.deb`, and passed it to privileged apt. The immediate empty lookup
  exposed the reliability defect; the unchecked package-to-root flow was the
  more serious provenance defect.
- The first cache-free PR-branch run,
  [`29100106370`](https://github.com/luisgui1757/dotfiles/actions/runs/29100106370),
  exercised exact head `1f03199f9d420e534bfade544ae7d74f1cfb002a` with broad
  caches disabled. Ubuntu container, Apple Silicon, and Windows producers
  passed. Public Ubuntu lost Astro captures and Intel lost GraphQL captures.
  Their logs showed the waited nvim-treesitter build callback complete, then a
  separate ordinary headless Lazy config load start the interactive
  asynchronous declared-parser installer before Phase 4. This disproved the
  build-hook-only repair rather than being rerun away.
- Behavioral repair commit:
  `93ce7fecd92a06583ac7b4211dfe2e1c169dac53` (`fix(setup): close
  cache-free install races`). Ordinary headless Neovim config loads now refuse
  the interactive asynchronous parser-install path; only a real UI or the
  explicit synchronous Phase 4 flag can start declared-parser installation.
  The already-waited, serialized Lazy build callback remains in force.
- The same commit removes execution of the Ghostty installer script. Setup now
  maps only reviewed Ubuntu 24.04, Ubuntu 25.10, and Debian trixie
  `amd64`/`arm64` identities to an exact release URL. It verifies nonempty bytes,
  one of six pinned SHA-256 values, `Package=ghostty`, exact architecture, and
  dpkg version `1.3.1-0~ppa2` before privileged apt; it then verifies the
  installed version and command. Private staging is checked, signal-cleaned,
  and removed on every exit. Failures before apt preserve the host; apt or
  post-publication failures emit explicit package-manager recovery guidance and
  enter the consolidated failure summary exactly once.
- Independent artifact review downloaded all six release assets and recomputed
  their SHA-256 values. Each matched the committed constant. `dpkg-deb` control
  inspection also matched package name, architecture, and exact version for all
  six. This is byte/metadata review, not a claim that every distro/architecture
  package was installed on real hardware.
- Focused PASS: Tree-sitter 21/21 (including the new real-headless no-async
  behavior); Ghostty mapping/success and staging/download/digest/metadata/apt/
  publication failure suites; failure accumulation; WSL preview; temporary
  cleanup; pin consistency; generalized privileged-package scanner self-tests;
  strict shell lint; and `git diff --check`.
- Full local PASS on the behavioral tree: Bash syntax over all 134 tracked shell
  scripts; `tests/static/run_all.sh`; `tests/shell/run_all.sh`; PowerShell parse
  and PSScriptAnalyzer plus 234/234 Pester with zero skips and all 17 Neovim
  specs via `test.ps1`; `make test-migration`; `make test-nvim`; `make test`;
  `make validate-renovate` with the exact 89-record official inventory;
  `nix flake check --print-build-logs`; and `make ci` ending `local pre-PR gate
  passed`. Nix retained the explicit x86_64-darwin support-window warning and
  reported incompatible local systems as omitted rather than tested.

### Finding status amendments

| ID | Status after entry 25 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-004 | ACCEPTED/FIXED | The pre-existing accumulator correctly converted Ghostty's recoverable installer failure into a final nonzero summary; commit `93ce7fecd92a06583ac7b4211dfe2e1c169dac53` replaces the mutable nested installer and adds staging/precondition/failure injection without a bare `set -e` escape. | Hosted exact-head confirmation remains part of UGR-021; real third-party outages remain external evidence. |
| UGR-008 | ACCEPTED/FIXED | The general download-to-privileged-package scanner now recognizes the repo's `maybe_sudo` and `verify_sha256` helpers and carries positive/negative self-tests; the exact Ghostty flow is required by the scanner and package tests. | None for static coverage; real package consumption is listed separately in MANUAL. |
| UGR-020 | PARTIAL | Stage 1 remains intact and all legacy plus logical identities are still emitted. The prior cache-free runs are failed evidence, so stage 2 is not yet safe. | Push this repair, pass its PR checks and a cache-free exact-head matrix, merge it, then require a newer merged-main logical matrix before opening the context-switch PR. |
| UGR-021 | PARTIAL | Cache-free execution found and drove both the second Tree-sitter repair and exact Ghostty package provenance. Local proof is complete for commit `93ce7fecd92a06583ac7b4211dfe2e1c169dac53`. | Every producer and logical job must pass on a new cache-free run of the pushed repair head, then again on its merged-main SHA. WSL, redirected Windows, dual Terminal, and desktop/TCC proof remain separate. |

### Safeguard and manual-proof boundary after entry 25

- The active integrity ruleset and classic fallback still require the exact 12
  legacy contexts. No live ruleset, branch protection, or Actions setting was
  changed.
- A real supported Debian-family Ghostty install remains unchecked in
  `tests/MANUAL.md`; the hosted Ubuntu container will exercise the Ubuntu 24.04
  amd64 asset on the pushed head.
- WSL, redirected Windows, divergent dual Windows Terminal state, owner-host tap
  rollback, and desktop GUI/TCC proof remain unclaimed.

## Ubuntu shell-fixture environment correction — entry 26

- PR #48 generic Ubuntu run `29102957625`, job `86396205735`, passed every
  shell case through `wsl_gui_tools_test.sh`; that test then exited before its
  `OK`, and `make test-shell` reported failure. Its final scenario set only the
  mocked `is_ubuntu` result to false but left `native_linux_pm` ambient. On the
  macOS development host the ambient result was `unknown`; on Ubuntu CI it was
  `apt`, so production's intentional plain-Debian apt routing correctly kept
  selecting the test's mocked reviewed `.deb` instead of reaching the scenario's
  intended non-apt Snap fallback.
- Alternative-cause check: both the PR Ubuntu container job `86396205982` and
  cache-free Ubuntu container job `86396379773` passed the real native apt
  setup, and the dedicated Ghostty mapping/success/failure cases passed inside
  the failed generic job. The red context was therefore fixture dependence on
  the host package manager, not a product-path failure.
- Test repair commit:
  `addd0efecbfea869f98804d5055f37d47e5b9793` (`test(shell): isolate Ghostty
  fallback fixture`). The fixture now supplies its native package manager
  explicitly: `apt` for the reviewed Debian-family branch and `unknown` for the
  non-apt fallback branch. It no longer inherits the executor OS.
- Focused and aggregate local PASS on the corrected tree:
  `wsl_gui_tools_test.sh`, complete `make test-shell`, strict shell lint,
  `git diff --check`, and `make ci` ending `local pre-PR gate passed`.
- This test-only correction does not promote the earlier cache-free run to
  exact-final-head evidence. Push it, let the PR workflows restart, and dispatch
  a new cache-free run on the resulting immutable head.

## Exact repaired behavior-head hosted closure — entry 27

- Exact repaired behavior head:
  `e5cf3e23299cbb42a157c307f2a7259979fcada0`.
- Cache-free workflow-dispatch run:
  [`29103732329`](https://github.com/luisgui1757/dotfiles/actions/runs/29103732329),
  completed successfully with every broad cache step skipped. Producers passed:
  Ubuntu container `86399025475` (3m17s), public Ubuntu `86399025519` (7m53s),
  Apple Silicon `86399025503` (10m33s), Intel `86399025491` (17m06s), and
  native Windows `86399025722` (16m44s). Stable setup logical proofs passed:
  container/Linux `86403118150`, setup/Linux `86403118145`, setup/macOS
  `86403118099`, and setup/Windows `86403118074`.
- Final behavior-head PR workflows also passed. Generic/parity run
  `29103728407` closed the Ubuntu fixture correction and passed Ubuntu, macOS,
  Windows, and all three parity jobs. Nix run `29103728279` passed Ubuntu,
  Apple Silicon, Intel, and logical Linux/macOS. E2E run `29103728188` passed
  all five producers and all four stable setup logical proofs. Every one of the
  12 live-required legacy contexts and all six candidate logical contexts was
  green on the same SHA.
- The cache-free Ubuntu container installed through the exact verified Ghostty
  package path; the prior mutable-installer failure did not recur. Apple
  Silicon, Intel, Ubuntu, and Windows strict parser/capture assertions all
  passed; neither the waited build callback nor ordinary headless config
  launched overlapping parser publication.
- This is exact automated branch-head evidence for the conventional GitHub
  environments. It is not a merged-main run and does not claim WSL, redirected
  Windows, divergent dual Windows Terminal state, owner-host tap rollback, or
  desktop GUI/TCC behavior.

### Finding status amendments

| ID | Status after entry 27 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-004 | ACCEPTED/FIXED | Both PR container `86398980438` and cache-free container `86399025475` passed the exact Ghostty package path; generic Ubuntu `86398854097` passed the corrected failure/fallback suite. | Normal third-party outage behavior remains external runtime evidence. |
| UGR-020 | PARTIAL | All 12 legacy and all six candidate logical contexts passed on repaired behavior head `e5cf3e23299cbb42a157c307f2a7259979fcada0`; live safeguards remain unchanged. | Merge PR #48, repeat cache-free/logical proof on its merged-main SHA, then open and merge the separate context-switch PR before owner-applied safeguards. |
| UGR-021 | PARTIAL | Cache-free run `29103732329` passed all five producers and all four setup logical proofs on the exact repaired behavior head; the same SHA passed both Nix logical proofs in `29103728279`. | Merged-main confirmation plus WSL, redirected Windows, dual Terminal, and desktop/TCC evidence remain. |

### Live relationship at entry 27

- Integrity ruleset `17363189` and classic branch protection still require the
  exact 12 legacy contexts with strict behind-main enforcement; all passed.
- Review ruleset `17363190` still requires one code-owner approval, last-push
  approval, stale-review dismissal, and thread resolution. PR #48 is
  `MERGEABLE` but correctly `BLOCKED` with `REVIEW_REQUIRED`.
- Owner-update ruleset `17363555` remains active. Actions remains enabled with
  `sha_pinning_required:false`; the checked-in desired value remains true for
  the later owner-applied stage. No live safeguard was mutated.
- This evidence entry is a documentation-only descendant of the immutable
  behavior head above. Required checks must still pass on the final PR head;
  the behavior-head cache-free result is not relabeled as a docs-head run.

## Lean platform and stable-safeguard cutover — entry 28

- Base and live `main` SHA:
  `f104bf066e4af7d4d707fe22ba36600711f1ae14` (PR #48 merge). `origin/main`
  still resolved to that SHA immediately before push preparation; no open PR
  existed.
- Cache-free merged-main run
  [`29114125798`](https://github.com/luisgui1757/dotfiles/actions/runs/29114125798)
  passed Ubuntu container `86433246345`, public Ubuntu `86433246387`, native
  Windows `86433246309`, historical Intel `86433246315`, and their reachable
  logical proofs. Apple Silicon `86433246367` failed because the first strict
  neocmakelsp probe shared the large fixture tree and did not attach within 45
  seconds; the later isolated formatter CMake project attached and accepted
  gersemi output in the same process. Commit `d6b4ec6` gives every initial LSP
  probe a distinct minimal project root and adds behavioral project-isolation
  coverage without weakening the real server, attach, formatting, diagnostic,
  or capture gates.
- Owner direction supersedes the earlier Intel support requirement. Commit
  `a112fe2` makes Apple Silicon the only Darwin contract, removes the Intel
  flake output, selector, runners, and Intel-only Nix action, and fails x86_64
  setup before Nix/Homebrew activation with migration guidance. Historical
  Intel ledger rows remain append-only; they are not current support.
- The hosted WSL2 canary is retired in `a112fe2`. Its only scheduled run
  [`29072773410`](https://github.com/luisgui1757/dotfiles/actions/runs/29072773410)
  and manual rerun
  [`29114215045`](https://github.com/luisgui1757/dotfiles/actions/runs/29114215045)
  reached real WSL2 but stalled before setup output and required cancellation.
  [GitHub documents hosted nested virtualization as experimental and officially
  unsupported](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners).
  Linux with fabricated WSL environment variables is rejected as
  fake proof. The real throwaway-distro and split-host manual harnesses remain.
- Commit `b378e60` switches all four checked-in safeguard mirrors to the stable
  logical identities while workflows retain legacy producers so the still-live
  legacy rules can gate this PR. The apply script now refuses to mutate unless
  its checkout is exact live `main`, safeguard sources are clean, and every
  stable context succeeded on that SHA; `--preflight-only` proves the same
  boundary without writes. Live safeguards were not changed.

### Finding status amendments

| ID | Status after entry 28 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-002 | REJECTED (superseded contract) | Explicit owner direction retires Intel; `a112fe2` removes every active Intel configuration/lane and adds exact fail-closed selection/evaluation guards. | None. Historical Intel results remain evidence only. |
| UGR-011 | PARTIAL | Native Linux clean-session proof remains green; the real WSL harness is preserved and no Linux proxy is mislabeled as WSL. | Run the manual throwaway WSL harness on a supported real Windows/WSL2 host. |
| UGR-020 | PARTIAL | `b378e60` completes the checked-in stable-context cutover and its exact-main/check-success preflight; legacy producer names remain emitted. | After merge, pass cache-free plus all six logical checks on the exact merged SHA, then owner runs preflight/apply/readback. |
| UGR-021 | PARTIAL | Cache-free merged-main run `29114125798` exposed the CMake proof defect fixed by `d6b4ec6`; unreliable hosted WSL runs are recorded and the workflow is retired. | Branch-head and merged-main cache-free proof; real WSL, redirected Windows, dual Terminal, and desktop/TCC evidence remain manual. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL, greenfield docs/ledger, security docs, and this append-only reconciliation state the lean platform and checked-in/live split truthfully. | Refresh exact PR/check/live status after push and after merge. |

### Local verification at entry 28

| Gate | Result |
|---|---|
| `git diff --check` | PASS |
| `bash -n` over tracked `*.sh` | PASS, 135 scripts |
| `make lint` | PASS after replacing the retired workflow's obsolete one-item guard loop; no suppression |
| `bash tests/static/run_all.sh` | PASS; the final tree also repeated static coverage through both umbrella targets |
| `bash tests/shell/run_all.sh` | PASS |
| `make test-migration` | PASS: template, parity, round-trip, uninstall, Windows render, and sourceable-payload oracle |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: PSScriptAnalyzer and 234 Pester cases, 0 failed, 0 skipped; Neovim entry point also returned 0 |
| Windows Terminal focused Pester cases | PASS inside the 234-case entry point: independent targets, invalid JSON, write/backup/publication/concurrency/collision/dry-run/skip/retry/idempotency and dual-path uninstall |
| `make test-nvim` | PASS, including locked bootstrap, LSP project isolation, checked deletion, 316 language assertions, and real two-project clangd |
| `make test` | PASS |
| `make validate-renovate` | PASS: official validator and exact 82-record local extraction |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; both Apple-Silicon Darwin attributes evaluated and the local toolchain check built; incompatible Linux builds were reported as not run locally |
| `make ci` | PASS, ending `local pre-PR gate passed` |
| Public-content audit | PASS: no tracked private local path, credential-shaped addition, or private key; Gitleaks 8.30.1 found no leak in the three implementation commits or uncommitted verification correction |

### Live relationship at entry 28

- Integrity ruleset `17363189`, review ruleset `17363190`, owner-update ruleset
  `17363555`, and classic protection remain active. Classic protection is strict
  on the exact twelve legacy contexts. Actions is enabled with
  `allowed_actions:all` and `sha_pinning_required:false`.
- Checked-in desired state uses the twelve stable/general contexts (six generic
  or parity names plus six stable logical names) and
  `sha_pinning_required:true`. This is a deliberate post-merge owner action,
  not a claim about live GitHub.
- No native Windows, WSL, redirected-known-folder, desktop/TCC, or post-merge
  stable-safeguard proof ran locally. Intel is no longer a pending environment;
  it is outside the owner-directed product contract.

## Exact lean behavior-head hosted proof — entry 29

- Immutable behavior head:
  `f097995b49a2189db327903a20743e7cb69ba665` on PR #49.
- Generic/parity run
  [`29120077646`](https://github.com/luisgui1757/dotfiles/actions/runs/29120077646)
  passed `windows` `86452871259`, `macos` `86452871272`, `ubuntu`
  `86452871309`, and parity jobs `86452871288`, `86452871295`, and
  `86452871302`.
- Nix run
  [`29120077669`](https://github.com/luisgui1757/dotfiles/actions/runs/29120077669)
  passed Apple Silicon `86452871102`, Ubuntu `86452871114`, and stable logical
  proofs `86453448279` / `86453448216`.
- Cached PR E2E run
  [`29120077871`](https://github.com/luisgui1757/dotfiles/actions/runs/29120077871)
  passed Apple Silicon `86452872089`, Ubuntu `86452872093`, Ubuntu container
  `86452872094`, Windows `86452872252`, and stable logical jobs
  `86457865643`, `86457865587`, `86457865572`, and `86457865570`. The cached
  Windows producer took 27m31s but completed setup and post-install proof; its
  duration was not relabeled as failure or waived.
- Cache-free workflow-dispatch run
  [`29120109175`](https://github.com/luisgui1757/dotfiles/actions/runs/29120109175)
  skipped every broad cache and passed the four current producers: Ubuntu
  container `86452977445` (2m33s), Ubuntu `86452977536` (7m28s), Apple Silicon
  `86452977452` (8m44s), and Windows `86452977443` (15m56s). Stable logical
  proofs `86455881986`, `86455881913`, `86455881923`, and `86455881943`
  validated their exact artifacts. There was no Intel or WSL job.
- This closes exact branch-head hosted proof for the isolated CMake LSP repair
  and the lean current matrix. It does not claim merged-main, WSL, redirected
  Windows, divergent dual Terminal, or desktop/TCC proof. This appended evidence
  commit is a documentation-only descendant; its required checks must still pass.

### Finding status amendments

| ID | Status after entry 29 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-002 | REJECTED (superseded contract) | All hosted current-product workflows contain Apple Silicon and no Intel job; the exact platform guards passed in every generic/Nix/setup lane. | None; Intel is retired, not pending proof. |
| UGR-020 | PARTIAL | All twelve still-live legacy contexts and all six stable logical contexts passed on the behavior head; checked-in sources target stable names and the live safeguards remain unchanged. | Merge, repeat cache-free/logical proof on exact merged main, then owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Cache-free run `29120109175` passed every current producer and logical setup proof on the exact behavior head, including the repaired Apple Silicon strict CMake path. | Merged-main confirmation plus real WSL, redirected Windows, dual Terminal, and desktop/TCC manual evidence. |
| UGR-022 | ACCEPTED/FIXED | ROADMAP, MIGRATION_STATUS, MANUAL, greenfield ledger, branch-protection runbook, reconciliation, and PR body now distinguish behavior-head pass from pending merged-main/manual proof. | Refresh only the final documentation-head checks and post-merge results. |

### Live relationship at entry 29

- PR #49 was `MERGEABLE` and correctly `BLOCKED` for review while the behavior
  head's required checks ran. All twelve live-required legacy checks passed.
- Live integrity/review/owner-update rulesets, classic required contexts, and
  Actions permissions were not mutated. `sha_pinning_required` remained false.
- The exact post-merge owner sequence remains: cache-free merged-main dispatch,
  verify all four setup producers + four setup logical + two Nix logical checks,
  run `--preflight-only`, apply, then read back both safeguard layers and Actions
  permissions.

## Independent PR #49 re-review correction — entry 30

- Independent review of immutable head
  `8c0bfb268592830d7213e4d3113d7bf61eb47101` downloaded the final-head logical
  artifacts and disproved their documented head field. Pull-request run
  `29121873434` reported PR source head `8c0bfb268592830d7213e4d3113d7bf61eb47101`
  through the workflow API, while its marker stored
  `head_sha=39316a5b385a6b69bf1332ffd19ab8329024621b`. The latter is GitHub's synthetic
  merge of base `f104bf066e4af7d4d707fe22ba36600711f1ae14` and that PR head, as required by
  the official `pull_request` event model. The producer work was real, but the
  durable field and documentation were false.
- The repair commit `fix(ci): bind logical proofs to source and executed SHAs`
  introduces marker schema 2. It records `source_head_sha` from
  `github.event.pull_request.head.sha || github.sha` and `executed_sha` from
  `GITHUB_SHA`, and verifies both in the consumer job. Behavioral shell coverage
  proves distinct pull-request identities, equal dispatch/push identities,
  drift rejection, missing-source rejection before publication, and obsolete
  schema rejection.

### Finding status amendments

| ID | Status after entry 30 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | Stable proof artifacts now distinguish the source head from the executed merge result and fail closed on either mismatch. | Complete the separate safeguard preflight/rollback repair, re-run exact-head workflows, merge, then perform merged-main cache-free proof and the owner-applied live cutover. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, ROADMAP, MIGRATION_STATUS, the safeguard runbook, and this append-only entry now describe GitHub pull-request execution truthfully. | Revalidate the final implementation head and append live results. |
