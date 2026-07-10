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
