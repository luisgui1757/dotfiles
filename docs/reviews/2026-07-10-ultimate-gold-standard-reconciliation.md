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
| UGR-023 | ACCEPTED/FIXED | Starship cache publication is atomic/validated/rollback-safe; Sentinel stages clean on failure/signal/retry; analyzer warnings have exact stable fingerprints; JSON traversal is NUL-safe; shell lint is strict; Nix ownership scanner catches nested/wrapped/imported bypasses while allowing system policy; direct-artifact compatible/stale/partial cases remain focused. | final closure batch (hash pending), with direct-artifact/shell-lint portions in `aa48aad` | Profile/Sentinel/JSON path/Nix scanner self-tests, analyzer full entry point, direct-artifact update suites | README, CLAUDE, ROADMAP, MIGRATION_STATUS | Platform-specific CI remains pending. |

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
control flow, and the Sentinel trap-only helper became an inline EXIT transaction;
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
  set, synchronized Mason, verified Sentinel, and completed the strict
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
| UGR-023 | ACCEPTED/FIXED | `eac92bc` plus `aa48aad` make Starship cache and Sentinel staging transactional, analyzer identities exact, JSON traversal NUL-safe, shell lint strict, Nix ownership structural, and direct-artifact updates compatibility-aware. | Profile/Sentinel/JSON-path/Nix-scanner/analyzer/direct-artifact tests; full local and hosted gates green. | README, CLAUDE, ROADMAP, MIGRATION_STATUS. | None beyond the platform/manual rows already named. |

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

## Independent PR #49 safeguard correction — entry 31

- The same independent review reproduced a second defect at immutable head
  `8c0bfb268592830d7213e4d3113d7bf61eb47101`. The old preflight returned before
  checking ruleset uniqueness or the expected live policy. The script then
  patched repository merge settings and Actions permissions before
  `upsert_ruleset` detected a duplicate ruleset. A later failure could therefore
  leave partial live mutation, and the runbook's suggested rerun/reapply path was
  not a rollback because it contained only the new desired payloads.
- The repair commit `fix(safeguards): preflight and recover the live cutover`
  makes every decision read-only before the first write. It validates exact
  local branch/origin/live-main identity, clean reviewed sources, the exact
  three active rulesets and complete legacy-or-stable policy stage, GitHub
  Actions app `15368`, exact test/Nix/E2E workflow events and run identities,
  every expected job, and skipped broad caches. A second full readback detects
  concurrent change before mutation.
- Only the three resources that differ in this transition are written: Actions
  SHA pinning, the integrity required-check set, and classic required checks.
  Their old payloads are stored with private permissions under Git metadata.
  Apply/readback failure or interruption automatically restores and verifies all
  three; incomplete rollback prints an exact, tested `--restore` command.
  Failure-injection tests cover wrong branch/remote, duplicate rulesets,
  unexpected contexts, dirty sources, wrong app/event/cache provenance,
  successful apply, repeated write-free apply, manual restore, partial apply
  rollback, and rollback-failure recovery.

### Finding status amendments

| ID | Status after entry 31 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | Both re-review defects are repaired with behavioral failure-injection coverage: truthful dual-SHA proof schema plus complete zero-write preflight and transactional recovery. | Re-run exact-head CI and independent review; after merge, record the exact merged-main cache-free/provenance gate, then owner applies and reads back the live stable posture. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, ROADMAP, MIGRATION_STATUS, MANUAL, supply-chain and branch-protection runbooks, and this append-only ledger now state the actual preflight/mutation/recovery contract. | Append final-head and post-merge live evidence only after those runs occur. |

## PR #49 re-review repair verification — entry 32

- Commit `54c03fd0ffddece073bc056b8ec992218253e0b1` implements and tests
  logical-proof schema 2. Pull-request jobs bind both the source head and the
  synthetic merge commit they executed; push and dispatch jobs bind the same
  commit in both fields.
- Commit `ffb5558c7bc16f795e03891df6f5521fd6a427cf` implements and tests the
  safeguard transaction. Every repository, branch, live-policy, ruleset,
  workflow, job, app, event, and cache-free requirement is read and validated
  before the first write. The three cutover resources are snapshotted under
  private Git metadata, narrowly mutated, read back, and automatically restored
  on failure or interruption.
- The real `--preflight-only` entry point was exercised from the PR branch and
  rejected it before any mutation because it was not checked out as exact live
  `main`. Live GitHub safeguards were not changed during implementation or
  verification.

### Local verification at entry 32

| Gate | Result |
|---|---|
| `git diff --check f104bf066e4af7d4d707fe22ba36600711f1ae14..HEAD` | PASS |
| `bash -n` over tracked `*.sh` | PASS, 135 scripts |
| `make lint` | PASS |
| `bash tests/static/run_all.sh` | PASS, including the safeguard transaction, required identities, provenance, and policy scanners |
| `bash tests/shell/run_all.sh` | PASS, including distinct PR source/executed identities, dispatch identity, drift, missing-input, and obsolete-schema failures |
| `make test-migration` | PASS: template, parity, round-trip, uninstall, Windows render, and sourceable-payload oracle |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: PSScriptAnalyzer clean; Pester 234 passed, 0 failed, 0 skipped; Neovim entry point returned 0 |
| `make test-nvim` | PASS, including pinned bootstrap, checked Tree-sitter deletion, 316 language assertions, and two-project clangd isolation |
| `make test` | PASS |
| `make validate-renovate` | PASS: official validator and exactly 82 reviewed dependency records |
| `bash tests/nix/run_all.sh` | PASS |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux derivations were not claimed as local proof |
| `make ci` | PASS, ending `local pre-PR gate passed` |
| Gitleaks 8.30.1, full PR range `f104bf0..ffb5558` | PASS: seven commits, no leaks |
| Gitleaks 8.30.1, clean tracked archives | The base and repaired head each contain the same two `generic-api-key` Windows Terminal fragment false positives at the same paths and lines; no finding was added |
| Added private-path audit | PASS: no local owner home path; the four non-example email-shaped values are the documented public `actions@github.com` app identity |

### Finding status amendments

| ID | Status after entry 32 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | Both implementation commits and every focused/local aggregate gate passed; live policy remains unchanged and legacy producers still gate the PR. | Independent review and exact-head hosted workflows; after merge, exact merged-main cache-free proof followed by owner preflight/apply/readback. |
| UGR-021 | PARTIAL | No cache or environment result was relabeled: branch-head cache-free proof remains recorded, while this repair received local behavioral proof only. | Exact repaired-head hosted proof, merged-main cache-free proof, and the documented real WSL/redirected-Windows/dual-Terminal/desktop manual runs. |
| UGR-022 | ACCEPTED/FIXED | All behavior, runbook, recovery, security, roadmap, migration, manual, and reconciliation descriptions match the repaired implementation and measured local results. | Append exact repaired-head and post-merge live results only after they occur. |

## PR #49 repaired-head hosted proof — entry 33

- Exact source head `4dbdb959674f5a062cffe44daae242318f4c1b67`
  passed all 18 emitted checks. The six generic/parity checks passed in
  [`29140112029`](https://github.com/luisgui1757/dotfiles/actions/runs/29140112029),
  both legacy Nix producers and both stable Nix checks passed in
  [`29140112035`](https://github.com/luisgui1757/dotfiles/actions/runs/29140112035),
  and all four E2E producers plus four stable setup checks passed in
  [`29140112030`](https://github.com/luisgui1757/dotfiles/actions/runs/29140112030).
  Every check-run was emitted by GitHub Actions app `15368`.
- All six downloaded logical markers use schema 2. They record
  `source_head_sha=4dbdb959674f5a062cffe44daae242318f4c1b67` and
  `executed_sha=0397ad36194e86c91b3a3aace5f0028885c03e7e`. The executed
  commit's two parents are exact base
  `f104bf066e4af7d4d707fe22ba36600711f1ae14` and that exact source head.
  Consumers in the same runs verified both identities, run ID/attempt, and the
  matching legacy/logical context pair.
- Gitleaks 8.30.1 found no leak in the downloaded Nix or E2E proof artifacts.
  The PR E2E cache steps ran normally; this result is intentionally not called
  cache-free proof. Live integrity/classic safeguards remain on the 12 legacy
  contexts, Actions SHA pinning remains false, and no implementation or test
  command mutated those live settings.

### Finding status amendments

| ID | Status after entry 33 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | The repaired implementation head passed all 12 live-required and all six stable checks, and its schema-2 artifacts prove source/executed identity truthfully. | Independent re-review; after merge, exact merged-main cache-free/provenance proof followed by owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Exact repaired-head Ubuntu, Apple Silicon, Windows, container, generic/parity, Nix, and stable logical proof is green. The run used ordinary PR caches and therefore does not satisfy the merged-main cache-free gate. | Merged-main cache-free proof plus the documented real WSL, redirected-Windows, dual-Terminal, and desktop/TCC manual runs. |
| UGR-022 | ACCEPTED/FIXED | ROADMAP, MIGRATION_STATUS, the PR body, and this append-only ledger now record the repaired-head hosted result without promoting it to cache-free or post-merge proof. | Append merged-main/live-apply evidence only after those events occur. |

## Independent PR #49 recovery/public-posture correction — entry 34

- Independent review of immutable head
  `998e3f19c38a827185261bdbb0d7aeee7def24a0` reproduced two additional
  fail-closed defects. Explicit restore accepted an altered integrity context
  and verified the unreviewed payload against itself. Removing
  `classic-live.json` allowed all three restore writes before post-write
  verification failed. Separately, a fixture with `private:true` and
  `visibility:private` passed the preflight described as the public-repository
  posture. These are accepted defects; private snapshot permissions and the
  repository's currently public live value do not disprove corruption,
  incompleteness, or concurrent visibility drift.
- The recovery repair requires all five consumed files before mutation, freezes
  them in a private temporary directory, and derives the only acceptable
  Actions, integrity, classic restore, and full classic state from the
  manifest's exact legacy/stable stage. It validates exact contexts, GitHub
  Actions app IDs, unique live integrity ruleset identity, bypass actors, branch
  conditions, and unrelated classic protections, then writes and verifies only
  frozen bytes. Failure cases cover every missing file, altered context,
  bypass, branch condition, cross-stage manifest/Actions data, altered classic
  state, wrong ruleset ID, and mutation of the retained source snapshot after
  validation.
- Both preflight captures now require `private:false` and
  `visibility:public`, and the concurrent-state fingerprint includes both
  fields. Behavioral cases reject initial private state and a public-to-private
  second-read transition with zero writes.
- `docs/MIGRATION_STATUS.md` no longer claims a current Intel activation
  configuration. Current activation is Apple-Silicon-only; historical Intel
  evidence remains append-only and explicitly retired.

### Finding status amendments

| ID | Status after entry 34 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | Recovery and public-posture gaps are repaired with behavioral zero-write failure cases and frozen-byte publication; the stable-context transition remains staged only. | Exact-head CI and independent re-review; after merge, merged-main cache-free proof followed by owner preflight/apply/readback. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, ROADMAP, MIGRATION_STATUS, security runbooks, and this ledger now state the exact public-preflight and recovery contract; the stale active-Intel sentence is corrected. | Refresh PR-body and exact-head evidence after push. |

## Probot branch-protection ownership correction — entry 35

- A second review called the checked-in `.github/settings.yml` stable contexts
  advisory-only because the Settings app installation could not be observed
  through the available read-only token. That uncertainty is not an acceptable
  cutover dependency. The file explicitly declared itself Probot-synced, and
  the app documents that default-branch changes are applied automatically.
  Upstream commit
  [`3629848d090115df71f6d5cf431561e67077ee27`](https://github.com/repository-settings/app/blob/3629848d090115df71f6d5cf431561e67077ee27/lib/settings.js#L24-L36)
  confirms that branch processing occurs only when a `branches` key exists.
- `.github/settings.yml` now contains repository-level settings only and
  deliberately omits `branches`. The owner-run safeguard transaction is the
  sole checked-in writer for integrity and classic required-check cutover. This
  prevents a default-branch Settings app sync from moving classic protection to
  stable names while the integrity ruleset remains legacy, a mixed stage that
  would otherwise strand the fail-closed preflight until manual recovery.
- `required_checks_test.sh` and `repo_policy_test.sh` reject any future top-level
  `branches` block in `.github/settings.yml` while continuing to prove the
  stable identities agree across metadata, the integrity ruleset, the apply
  function, and its narrow API payload. Focused required-check, repository
  policy, and YAML lint cases pass. README, CLAUDE, ROADMAP,
  MIGRATION_STATUS, and the branch-protection runbook record the single-writer
  boundary. No live repository setting was read-write or changed.

### Finding status amendments

| ID | Status after entry 35 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | The staged required-check cutover now has one transactional writer; Probot cannot race classic protection on merge. Recovery/public-posture repairs remain behaviorally covered. | Exact-head CI and independent re-review; after merge, exact merged-main cache-free proof followed by owner preflight/apply/readback. |
| UGR-022 | ACCEPTED/FIXED | Current safeguard ownership and default-branch app behavior are documented consistently, without relying on an unverifiable installation assumption. | Append exact repaired-head and post-merge results only after they occur. |

## Final local verification for entries 34–35 — entry 36

- Recovery/public-visibility implementation commit:
  `dfef60c6626de8feb8498cce7edb678a73dcec69`.
- Single-writer/Probot implementation commit:
  `374b3b84cf16371497fb8c78996244625b4db628`.
- The primary checkout, stashes, untracked review prompts, real HOME, and live
  GitHub safeguards were not modified. Verification ran in a disposable clone
  on Apple Silicon macOS.

| Check | Exact local result |
|---|---|
| `git diff --check` | PASS |
| `bash -n` over tracked shell scripts | PASS: 135/135 |
| `make lint` | PASS |
| Focused safeguard transaction suite | PASS: incomplete/altered/cross-stage restore, frozen-byte publication, public/private drift, rollback, retry, and idempotency |
| Focused required-check/repository-policy/YAML suites | PASS: Probot branch ownership absent; canonical stable identities remain aligned |
| `bash tests/static/run_all.sh` | PASS |
| `bash tests/shell/run_all.sh` | PASS |
| `make test-migration` | PASS |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: PSScriptAnalyzer clean; Pester 234 passed, 0 failed, 0 skipped; Neovim entry point returned 0 |
| `make test-nvim` | PASS, including pinned bootstrap, checked Tree-sitter deletion, and real two-project clangd isolation |
| `make test` | PASS |
| `make validate-renovate` | PASS: official validator and exactly 82 reviewed dependency records |
| `bash tests/nix/run_all.sh` | PASS |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems were omitted, not claimed as runtime proof |
| `make ci` | PASS: final behavior head ended `local pre-PR gate passed` |
| Gitleaks 8.30.1, full PR range `f104bf0..374b3b8` | PASS: 11 commits, no leaks |
| Gitleaks 8.30.1, clean tracked archives | Base and repaired head contain the same two `generic-api-key` Windows Terminal fragment false positives at the same two paths and line 49; no finding was added |
| Added private-identifier audit | PASS: no local owner home path or private email; matches are only public GitHub transport/app identities and `example.invalid` test data |

Hosted exact-head checks, logical marker downloads, and final live readback are
recorded only after the repaired head is pushed and those events complete.

## Repaired-head hosted proof and live readback — entry 37

- Exact source head `dc13b7b81b45e0c6de8008e17fc890742391b882`
  passed all 18 emitted checks. Generic/parity passed in
  [`29149010795`](https://github.com/luisgui1757/dotfiles/actions/runs/29149010795),
  both Nix producers and both stable Nix checks passed in
  [`29149010866`](https://github.com/luisgui1757/dotfiles/actions/runs/29149010866),
  and all four E2E producers plus all four stable setup checks passed in
  [`29149010801`](https://github.com/luisgui1757/dotfiles/actions/runs/29149010801).
  Every check-run was emitted by GitHub Actions app `15368`.
- All six downloaded markers passed the checked-in schema-2 verifier. They bind
  `source_head_sha=dc13b7b81b45e0c6de8008e17fc890742391b882`
  to `executed_sha=763f810a8eb20879f9dd4edf776eb95a258b9a44`,
  with exact run ID/attempt and legacy/logical pair. The executed commit is the
  live `refs/pull/49/merge`; its parents are exact base
  `f104bf066e4af7d4d707fe22ba36600711f1ae14` and that exact source head.
  Gitleaks found no secret in the six artifacts.
- Final read-only live inspection still reports a public repository, the exact
  12 legacy required contexts with GitHub Actions identity `15368` in both the
  integrity ruleset and classic fallback, strict classic checks, no integrity
  bypass, and `sha_pinning_required:false`. No test, push, workflow, or PR-body
  operation applied the staged cutover. The E2E PR run used ordinary caches and
  is not the pending merged-main cache-free proof.

### Finding status amendments

| ID | Status after entry 37 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | The repaired source head passed every legacy and stable check; marker identities and live unchanged policy were independently read back. | Independent re-review; after merge, exact merged-main cache-free proof followed by owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Exact-head hosted Ubuntu, Apple Silicon, Windows, container, Nix, generic/parity, and logical proof is green without relabeling ordinary caches as clean-install proof. | Merged-main cache-free proof plus real WSL, redirected-Windows, dual-Terminal, and desktop/TCC manual runs. |
| UGR-022 | ACCEPTED/FIXED | The append-only ledger now records the actual repaired-head runs, dual identities, artifact scan, and unchanged live posture. | Append merged-main/live-apply evidence only after those events occur. |

## Final PR #49 transaction-input hardening — entry 38

- Independent review of immutable source head
  `95dfe6efb7d94c9dae482a013e5bcd72f326b8e3` reproduced four additional
  fail-closed defects despite all 18 hosted checks being green. Apply read the
  integrity payload from the mutable checkout after its first live write;
  restore accepted a full-classic snapshot missing a required nullable field;
  the Probot ownership tests recognized only one multiline `branches:` spelling;
  and a failed second capture leaked the first temporary directory. These are
  accepted defects, not waived P3 observations.
- Commit `0084620ead67c7d7fd1e1fcb98db93bafe5895ec` closes the complete class:
  after the second capture, apply freezes check metadata, integrity, manifest,
  classic, and Actions inputs from the exact committed tree in one private
  read-only transaction directory, cross-validates the set, and gives every API
  write only those files. Recovery requires every consumed full-classic key,
  freezes all snapshot bytes, loads expected policy from the manifest's captured
  commit only while it is still live `main`, and rejects moved/unavailable,
  malformed, symlinked, altered-app, narrow/full, or cross-stage material before
  any write. Capture directories are caller-owned and cleaned on every exit;
  recovery snapshots are pruned on pre-mutation failure and retained once a
  mutation may have occurred.
- The Settings ownership guard now parses YAML with bounded Ruby/Psych input.
  Block, inline-array, inline-map, null, direct-alias, and merge-alias top-level
  `branches` fixtures all fail, while nested prose/data remains valid. The CI
  jobs install Ruby explicitly. Logical-proof coverage now also pins run-ID,
  attempt, executed-SHA, run-identity, and empty-context rejection. The unused
  generic mutation helper is removed, and restore distinguishes readback failure
  from a successful-but-different readback.
- Implementation and verification ran only in a disposable exact-head clone.
  The stale primary checkout, its three stashes, its untracked review files, and
  live GitHub safeguards were not changed.

### Local verification

| Check | Exact result |
|---|---|
| `git diff --check` | PASS |
| `bash -n` over repository `*.sh` | PASS: 136/136 |
| Focused safeguard transaction suite | PASS: frozen apply inputs; complete classic schema; symlink/malformed/app-ID/narrow-full/moved-policy rejection; zero-write cleanup; restore/readback/rollback/retry/idempotency |
| Semantic Settings YAML suite | PASS: all top-level block/inline/null/alias shapes rejected |
| Logical-proof identity suite | PASS: source/executed SHA, run ID/attempt, context, schema, missing/duplicate, and emit-time validation |
| `bash tests/static/run_all.sh` | PASS |
| `bash tests/shell/run_all.sh` | PASS |
| `make lint` | PASS |
| `make test-migration` | PASS |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: PSScriptAnalyzer clean; Pester 234 passed, 0 failed/skipped; Neovim entry point returned 0 |
| `make validate-renovate` | PASS: official validator and exactly 82 reviewed records |
| `bash tests/nix/run_all.sh` | PASS |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems were omitted, not claimed as local runtime proof |
| `make ci` | PASS: ended `local pre-PR gate passed` |

### Finding status amendments

| ID | Status after entry 38 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | The transaction, recovery schema, semantic Probot exclusion, and logical-proof regression gaps are repaired and locally green; the live transition remains staged only. | Push the exact repaired head, require all hosted checks and independent re-review, then after merge obtain exact merged-main cache-free proof before owner preflight/apply/readback. |
| UGR-021 | PARTIAL | No local result is relabeled as WSL, redirected Windows, dual-Terminal, desktop/TCC, or merged-main cache-free evidence. | Complete the documented live/manual environments after merge. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, ROADMAP, MIGRATION_STATUS, manual instructions, security runbooks, workflow dependencies, tests, and this append-only entry describe the repaired boundary. | Append exact pushed-head hosted/artifact/live-readback evidence only after it exists. |

## Final transaction-hardening hosted proof — entry 39

- Exact pushed source head
  `a9fef7dfac7f0f6832b57433fe2a5cff4b144d2c` passed all 18 emitted
  checks. Generic/parity passed in
  [`29163663315`](https://github.com/luisgui1757/dotfiles/actions/runs/29163663315),
  both Nix producers and both stable Nix checks passed in
  [`29163663329`](https://github.com/luisgui1757/dotfiles/actions/runs/29163663329),
  and all four E2E producers plus all four stable E2E checks passed in
  [`29163663344`](https://github.com/luisgui1757/dotfiles/actions/runs/29163663344).
  Every check-run was emitted exactly once by GitHub Actions app `15368`; all
  three runs were `pull_request`, attempt 1, on that exact source head.
- All six downloaded schema-2 markers passed the checked-in verifier. They bind
  `source_head_sha=a9fef7dfac7f0f6832b57433fe2a5cff4b144d2c` to
  `executed_sha=f4a63197bbfd43c1de7c0d73fb1dffa47cfdad44`, with exact run
  ID, attempt, logical context, and legacy context. The executed commit is the
  live `refs/pull/49/merge`; its ordered parents are exact base
  `f104bf066e4af7d4d707fe22ba36600711f1ae14` and that exact source
  head. Gitleaks 8.30.1 found no secret in the six artifacts.
- Final read-only live inspection still reports a public repository, three
  unique active rulesets, the exact 12 legacy required contexts from app
  `15368` in both integrity and classic protection, strict classic checks, no
  integrity bypass, and `sha_pinning_required:false`. Review and owner-update
  bypass remain limited to owner `139752288` in pull-request mode. No push,
  workflow, test, artifact download, or PR operation applied the staged
  cutover. The E2E PR run used ordinary PR caches and is not the pending
  merged-main cache-free proof.

### Finding status amendments

| ID | Status after entry 39 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | The repaired transaction head passed all legacy and stable checks; all six dual-SHA markers and unchanged live policy were independently read back. | Independent final review; after merge, exact merged-main cache-free proof followed by owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Exact-head hosted Ubuntu, Apple Silicon, Windows, container, Nix, generic/parity, and logical proof are green without relabeling ordinary PR caches as clean-install proof. | Merged-main cache-free proof plus real WSL, redirected-Windows, dual-Terminal, and desktop/TCC manual runs. |
| UGR-022 | ACCEPTED/FIXED | The append-only ledger records the accepted defects, canonical implementation, local gates, exact hosted runs, dual identities, artifact scan, and unchanged live posture. | Append merged-main/live-apply evidence only after those events occur. |

## Captured-policy restore and v0.1.0 release migration — entry 40

- Independent review of exact source head
  `fcec13763a2d51f8787dcf0be85937e94495c38e` reproduced two P1 defects despite
  a green repository gate. Safeguard restore derived classic expectations from
  the running checkout while integrity expectations came from the manifest
  commit, permitting a coherent worktree/classic snapshot alteration to publish
  a mixed stage. Separately, the README told v0.1.0 users to update their
  checkout before setup, but v0.1.0 is already chezmoi-based and POSIX targets
  are live checkout symlinks; exact macOS/Linux reproduction changed live config
  before backup and produced no old-byte backup.
- Both are accepted contract defects. Restore now validates the captured
  `check-identities.json`, derives legacy/stable integrity and full/narrow
  classic policy from that single captured file, and never invokes a
  worktree-backed context helper. Legacy and stable adversarial fixtures mutate
  the running checkout/script and snapshot classic state coherently; both fail
  before any API write.
- Release upgrades are now exact-tag, side-by-side transactions. POSIX and
  Windows tools require clean official annotated v0.1.0/v0.2.0 checkouts,
  retained historical config, authoritative target identity, and private
  recovery until acceptance. Both tools archive the exact release commits and
  bind publication, verification, and rollback to validated frozen trees rather
  than the mutable retained checkouts. POSIX applies only Nix plus config and
  removes the first nix-darwin/Home Manager activation on later failure. Windows remains
  non-Nix, applies config with dependencies skipped, freezes exact packaged and
  portable Terminal recovery bytes under a protected ACL, retains conventional
  v0.1 known-folder targets while rollback is open, removes only created overlay
  state on rollback, and validates both targets before either restore write. Native/deferred package provisioning is
  deliberately outside the reversible release transaction; Windows also skips
  Neovim caches, agent policy, and chezmoi run scripts.
- The Nix prerequisite is a versioned, checksum-verified upstream Nix 2.34.0
  archive for the three supported POSIX systems. No downloaded archive is
  extracted or executed before its reviewed SHA-256 matches; Intel macOS fails
  before download. README, upgrading/release docs, CLAUDE invariants, roadmap,
  migration status, supply-chain policy, manual evidence rows, and this ledger
  now distinguish v0.1.0 from pre-chezmoi history and prohibit moving-branch
  release instructions.

### Focused behavior proof

| Check | Exact result |
|---|---|
| Captured-policy safeguard suite | PASS: legacy and stable worktree/classic drift reject with zero writes; existing restore/apply/readback/rollback/retry/idempotency cases remain green |
| Exact v0.1.0 POSIX fixture | PASS: exact tag/inventory, in-place and dirty rejection, write-free preflight, post-activation exit 42, automatic Home Manager/config rollback, retry, and explicit acceptance |
| Windows upgrade Pester | PASS on non-Windows logic host: 4 recovery cases, 0 failed/skipped; native protected-ACL case is defined only for the Windows runner |
| Release-upgrade static policy | PASS: historical identities/inventory, v0.2.0 tag contract, rollback phase split, Nix hashes, CI reach, and no moving-branch/placeholder documentation |

### Finding status amendments

| ID | Status after entry 40 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | Safeguard restore now has one captured-commit policy source for integrity and classic, with legacy/stable zero-write attacks covered. | Full gate, exact pushed-head independent review, then merged-main cache-free proof and owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Exact historical POSIX migration is automated and failure-injected without relabeling it as owner-host, WSL, or redirected-Windows proof. | Apple Silicon owner-host, real WSL2, native aarch64, redirected/dual-Terminal Windows, desktop/TCC, and merged-main rows. |
| UGR-022 | ACCEPTED/FIXED | The unsafe README command and pre-chezmoi misclassification are removed; versioned migration, provenance, rollback, unsupported Intel, and release gates are discoverable. | Append final local/hosted/tag/manual evidence only after it occurs. |

## Release-upgrade recovery closure and final local gate — entry 41

- The release migration now keeps chezmoi run scripts outside its reversible
  core with `--skip-config-scripts`; only Nix activation and backed-up config
  files/links publish before acceptance. The exact historical harness runs the
  real setup backup/config path, proves the old zsh link is retained as a
  collision-safe backup before publication, and confirms no deferred plugin
  publisher ran. Setup creates required managed-target parents itself; rollback
  prunes only transaction-created parents that remain empty.
- POSIX recovery has a private exact-file digest manifest and validates every
  scalar, stage, checkout/tag/origin/cleanliness boundary, target inventory,
  absent-parent inventory, flake lock, Nix provider, and command-provider
  inventory before its first rollback write. Coherently altered target and
  lock payloads plus an invalid stage all reject without changing live config.
  Every publication and rollback source is a digest-bound exact tree beneath
  private recovery. A coherent post-validation checkout mutation cannot change
  published or restored bytes. Both injected exit 42 and a real
  post-publication TERM restore v0.1.0 and remove the first
  Home Manager or nix-darwin activation.
- The macOS fixture selects the Apple-Silicon/nix-darwin branch, locked
  bootstrap and uninstaller, config backup, and deferred-script boundary while
  mocking privileged `/etc`, Homebrew, and Nix effects. It is automated proof,
  not the still-required real owner-host row. Linux exercises the locked Home
  Manager bootstrap/uninstall branch. Hosted workflow jobs select the matching
  fixture mode and fetch the exact v0.1.0 tag history.
- Windows recovery now validates its complete known-folder state boundary
  before uninstall, requires the recovery script/provider/RECOVERY material,
  verifies the exact pre-migration provider boundary after rollback, and
  enforces an owner/System/Administrators-only protected ACL. It archives both
  exact commits, digest-validates those source trees, and runs setup, readback,
  uninstall, and rollback only from recovery. Terminal recovery remains
  all-target-validated before either write. The native ACL case is defined for
  the Windows runner; non-Windows Pester covers twelve recovery and failure
  cases. Its setup call explicitly skips dependencies, Neovim caches, agent
  policy, and chezmoi run scripts so the reversible boundary contains only
  config files/symlinks, known-folder overlays, and Terminal settings.
- The analyzer baseline changed only because the existing setup update-message
  `Write-Host` extent now points users to reviewed release migration instead of
  `git pull`. Old/new analyzer identity comparison found no new warning group
  or count; the reviewed fingerprint is now
  `1ca7e2f50a9e7e7fbe999197c2ef3bb66f6f3833a481ede9fc667dbba6b7b5b8`.

### Final local verification

| Check | Exact result |
|---|---|
| `git diff --check` and shell syntax/lint | PASS; no whitespace errors; changed shell scripts parse; repository lint green |
| `bash tests/static/run_all.sh` | PASS, including captured-policy safeguards and release-upgrade policy |
| `bash tests/shell/run_all.sh` | PASS, including dual-SHA proof and setup flag behavior |
| `make test-migration` | PASS, including exact v0.1.0 apply/failure/TERM/rollback/retry/acceptance and the existing parity/uninstall/oracle bundle |
| `TEST_UPGRADE_PLATFORM=Darwin bash tests/migration/v0_1_upgrade_test.sh` | PASS: Apple-Silicon/nix-darwin fixture mode |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: exact analyzer fingerprint, 249 Pester passed with zero failed/skipped locally, and all Neovim specs |
| `make validate-renovate` | PASS: official validator and exactly 83 reviewed dependency records |
| `bash tests/nix/run_all.sh` | PASS |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems were reported as omitted, not runtime proof |
| `make ci` | PASS: ended `local pre-PR gate passed` |

No real Windows/WSL/redirected-folder/dual-Terminal/owner-host migration, final
v0.2.0 tag, push, hosted result, merge, live safeguard mutation, or merged-main
cache-free proof is claimed by this entry.

### Finding status amendments

| ID | Status after entry 41 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-020 | PARTIAL | Captured-commit safeguard restore and release recovery reject altered/cross-source material before mutation; all local transaction gates pass. | Exact pushed-head review; after merge, merged-main cache-free proof and owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Exact historical Linux/macOS fixture modes are failure- and signal-injected without relabeling mocks as owner-host or WSL proof. | Real Apple Silicon owner host, WSL2, native aarch64, redirected/dual-Terminal Windows, desktop/TCC, and merged-main rows. |
| UGR-022 | ACCEPTED/FIXED | README, upgrading/release docs, setup flags, roadmap, status, manual matrix, security docs, and this ledger describe the exact release and recovery boundaries. | Append pushed/hosted/tag/manual evidence only when it exists. |

## First hosted release-migration pass and strict-LSP lifecycle repair — entry 42

- Exact pushed source head
  `9da0ee63c1423efae660e50680404fb6df7db1e3` produced real mixed hosted
  evidence rather than a green claim. Nix run
  [`29180481911`](https://github.com/luisgui1757/dotfiles/actions/runs/29180481911)
  passed both producers and both logical checks. Test run
  [`29180481912`](https://github.com/luisgui1757/dotfiles/actions/runs/29180481912)
  passed five jobs but failed Ubuntu `chezmoi-parity` job `86617021397` in the
  exact v0.1.0 fixture: direct historical chezmoi publication could not create
  a Ghostty target whose parent directory did not yet exist. The fixture now
  derives every exact historical managed file/symlink target after `chezmoi
  init` and creates only those parents before applying v0.1.0. A disposable
  Ubuntu 24.04 x86_64 container using pinned chezmoi 2.71.0 and its reviewed
  SHA-256 then passed the complete in-place rejection, preflight, drift,
  activation/config failure, rollback, altered recovery, TERM, retry, and
  acceptance sequence. The Darwin fixture remained green.
- E2E run
  [`29180481941`](https://github.com/luisgui1757/dotfiles/actions/runs/29180481941)
  passed Ubuntu container `86617021374`, Ubuntu setup `86617021396`, Windows
  setup `86617021378`, and their logical proofs. Apple Silicon job
  `86617021389` completed setup and attached the first isolated neocmake client,
  then timed out starting a second formatter-only neocmake client; its logical
  check correctly failed because no proof artifact existed. This is not a
  product setup failure and is not waived as a retry: the strict smoke still
  had two independent client lifecycles for one CMake behavior proof.
- The canonical test repair keeps every real assertion while removing the
  redundant lifecycle. Each realistic formatter fixture is now copied into the
  same minimal project used for its attachment probe; after that client
  attaches, the smoke requires the exact Conform formatter set, runs the
  formatter, writes the result, waits for diagnostics, and rejects warnings or
  errors before stopping the client. No timeout increased and no server,
  formatter, save, diagnostic, parser, capture, or syntax gate was skipped.
  Tier-1 coverage pins the single attach-wait call site and same-project
  formatter invocation. Three repeated strict Apple-Silicon runs from an
  isolated clone of the installed runtime passed all 257 checks, including
  neocmake plus gersemi, on every run. The first aggregate run correctly
  rejected an over-broad new source assertion that counted the helper
  declaration as a call; the assertion was narrowed to the actual assignment,
  its focused 317-case spec passed, and the complete gate then passed.

### Focused repair verification

| Check | Exact result |
|---|---|
| `bash tests/migration/v0_1_upgrade_test.sh` | PASS: full exact Linux release transaction |
| Disposable Ubuntu 24.04 x86_64 exact-release fixture | PASS with pinned/verified chezmoi 2.71.0; reproduces the formerly missing historical parent before running all recovery cases |
| `make test-nvim` | PASS, including the single-lifecycle Tier-1 regression assertion |
| Strict production LSP smoke repeated three times | PASS: 257/257 each run; real neocmake attach and gersemi/diagnostic proof, isolated HOME/data copy |
| `TEST_UPGRADE_PLATFORM=Darwin bash tests/migration/v0_1_upgrade_test.sh` | PASS: full exact Apple-Silicon/nix-darwin fixture transaction |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: analyzer fingerprint, 249/249 Pester, all Neovim specs |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems omitted, not claimed as runtime proof |
| `make ci` | PASS: ended `local pre-PR gate passed` after the focused assertion correction |
| `git diff --check` | PASS |

No final repaired-head hosted result, logical artifact, immutable v0.2.0 tag,
merge, live safeguard mutation, or merged-main cache-free proof is claimed by
this entry.

### Finding status amendments

| ID | Status after entry 42 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-018 | ACCEPTED/FIXED | Strict LSP attach and formatter compatibility now share one isolated project/client lifecycle; three repeated real-tool runs passed without a timeout increase. | Require the final pushed-head macOS producer and logical proof to pass. Interactive confirmation remains optional. |
| UGR-020 | PARTIAL | The release/safeguard implementation remains locally green; first hosted Nix proof passed, while unrelated failed jobs correctly blocked complete PR proof. | Final repaired-head checks/artifacts and independent review; after merge, merged-main cache-free proof and owner preflight/apply/readback. |
| UGR-021 | PARTIAL | The failed hosted rows are recorded exactly; no partial run is promoted to green and no local/container result is relabeled as WSL, redirected Windows, dual Terminal, desktop/TCC, or merged-main evidence. | Final repaired-head hosted proof plus the existing real/manual environments. |
| UGR-022 | ACCEPTED/FIXED | CLAUDE, roadmap, migration status, manual matrix, greenfield ledger, and this append-only entry describe both hosted failures and their causal repairs. | Append final hosted/tag/manual evidence only when it exists. |

## Repaired release-migration and strict-LSP hosted proof — entry 43

- Exact repaired behavior head
  `d744948cdccc51f3d79e45aa78f82c46445df0c6` passed every transition check.
  Generic/parity run
  [`29181215799`](https://github.com/luisgui1757/dotfiles/actions/runs/29181215799)
  passed all six jobs, including the formerly failing Ubuntu exact-v0.1.0
  fixture. Nix run
  [`29181215800`](https://github.com/luisgui1757/dotfiles/actions/runs/29181215800)
  passed both real producers and both logical checks. E2E run
  [`29181215803`](https://github.com/luisgui1757/dotfiles/actions/runs/29181215803)
  passed Ubuntu container `86619065292`, Ubuntu setup `86619065299`, Apple
  Silicon `86619065302`, Windows `86619065296`, and logical proofs
  `86619772805`, `86619772768`, `86619772795`, and `86619772770`. The Apple
  Silicon producer completed the combined real neocmake attachment plus
  gersemi/save/diagnostic assertion that failed before the lifecycle repair.
- The head has 21 unique completed successful check-runs: the 18 expected
  legacy/stable transition checks plus successful CodeQL default-setup checks
  `Analyze (actions)`, `Analyze (python)`, and `CodeQL`. No failed, skipped,
  pending, duplicate-name, or non-success result remained. The 18 transition
  checks and both Analyze checks came from GitHub Actions app `15368`; the
  aggregate CodeQL check came from app `57789`.
- All six downloaded schema-2 markers passed the checked-in verifier. Artifact
  IDs `8256458379` and `8256474189` belong to Nix run `29181215800`; IDs
  `8256477160`, `8256494635`, `8256523247`, and `8256530035` belong to E2E run
  `29181215803`. Every marker binds source head `d744948…` to executed synthetic
  merge `048052bf7bfdb957ec91e94d4127a5efb6f47c68`, attempt 1, with exact
  run/logical/legacy identity. The merge commit is GitHub-authored with ordered
  parents base `f104bf066e4af7d4d707fe22ba36600711f1ae14` then source head
  `d744948…`.
- Final read-only PR inspection still showed OPEN, non-draft, MERGEABLE, and
  policy BLOCKED only with `REVIEW_REQUIRED`; base, head, branch names, and live
  `main` remained exact. Gitleaks 8.30.1 scanned all 18 base-to-head commits and
  approximately 405 KB with zero findings. This PR run used ordinary PR caches;
  it is not the pending merged-main cache-free proof. No review, approval,
  merge, release tag, live safeguard mutation, or v0.2.0 publication occurred.

### Finding status amendments

| ID | Status after entry 43 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-018 | ACCEPTED/FIXED | The combined isolated attach/formatter lifecycle passed three repeated local strict runs and the exact Apple-Silicon hosted producer. | Optional interactive confirmation only. |
| UGR-020 | PARTIAL | All 18 transition checks and six dual-SHA artifacts are green on the exact repaired head; live cutover remains intentionally unapplied. | Independent final review; after merge, merged-main cache-free proof and owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Exact-head hosted Linux, Apple Silicon, Windows, container, Nix, generic/parity, security, and logical proof are green without relabeling PR caches as clean-install proof. | Merged-main cache-free proof plus real WSL, redirected-Windows, dual-Terminal, owner-host migration, and desktop/TCC rows. |
| UGR-022 | ACCEPTED/FIXED | Roadmap, migration status, manual matrix, greenfield ledger, and this append-only entry now record both failed discovery heads and the exact repaired hosted proof. | Append final tag, merged-main, live-apply, and manual evidence only when it exists. |

## Apple-Silicon-only product-surface closure — entry 44

- Owner direction removed the remaining dedicated Intel macOS product surface.
  `scripts/install-pinned-chezmoi.sh` no longer selects or downloads a Darwin
  x86_64 archive, and `.github/workflows/test.yml` no longer carries its
  checksum. `setup.sh`, the checksum-verified Nix prerequisite, and the exact
  v0.1.0 release migrator now share a generic Apple-Silicon-only boundary
  instead of an Intel-specific branch or migration path.
- Current user-facing README, upgrade, release, and supply-chain documentation
  contains no Intel-specific procedure or retained-support guidance. It states
  only the positive Apple Silicon contract. The roadmap, migration status, and
  canonical agent guide are synchronized with that behavior.
- `tests/static/darwin_platform_contract_test.sh` scans every active Darwin
  product surface for removed outputs, runners, installer selectors, checksums,
  and user-facing wording. Its executable negative fixture supplies a Darwin
  x86_64 machine identity to the real pinned chezmoi installer and proves a
  failure at the platform boundary before download or publication. Historical
  hosted rows remain untouched in append-only ledgers; they are evidence, not a
  product path.

### Local verification

| Check | Exact result |
|---|---|
| `bash tests/static/darwin_platform_contract_test.sh` | PASS: active-product scan clean; real pinned chezmoi helper rejected Darwin x86_64 before download/publication |
| `bash tests/nix/setup_nix_darwin_test.sh` | PASS: Apple Silicon selection and generic unsupported-architecture failure remain fail-closed before activation |
| `bash tests/nix/darwin_config_test.sh` | PASS: only `dotfiles` and `dotfiles-aarch64`, both `aarch64-darwin` |
| `bash tests/static/release_upgrade_test.sh` | PASS: exact-tag upgrade identity, recovery, and documentation contract remains complete |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: PSScriptAnalyzer, 249 Pester passed with zero failed/skipped, and all Neovim specs |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems reported as omitted, not runtime proof |
| `make ci` | PASS: ended `local pre-PR gate passed` |
| `git diff --check` and changed-shell `bash -n` | PASS |

No pushed-head hosted result, review, approval, merge, release tag, live
safeguard mutation, or merged-main cache-free proof is claimed by this entry.

### Finding status amendments

| ID | Status after entry 44 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-002 | REJECTED (superseded contract) | Apple Silicon is the only active Darwin output, runner, pinned installer artifact, setup target, release migration, and current documentation path. The generic boundary rejects every other macOS architecture before mutation. | None; historical rows remain evidence only. |
| UGR-020 | PARTIAL | The safeguard and release transaction contracts remain green through the full local gate; this architecture closure does not apply live safeguards. | Final pushed-head checks/artifacts and independent review; after merge, merged-main cache-free proof and owner preflight/apply/readback. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, roadmap, migration status, upgrade/release/supply-chain docs, tests, and this append-only entry describe the same Apple-Silicon-only product contract. | Append pushed/tag/manual evidence only after it exists. |

## Sentinel repository rename cutover — entry 45

- The canonical upstream is now the public repository
  `https://github.com/luisgui1757/sentinel`. Its renamed default-branch tree is
  exact commit `ecafffa858666343c1639f996d177f460163e93e` and declares
  `VERSION=0.1.2`. The published `v0.1.2` tag still peels to the pre-rename
  commit `ecca742fa9ed1243a73981955850c1a8ef3e3b04`; setup therefore pins the
  renamed commit plus `VERSION` and does not manufacture a false tag mapping.
- Both setup entry points, their cache roots, functions, variables, prompts,
  diagnostics, mock repositories, pin mirrors, and tests use Sentinel
  exclusively. The shell test path is now
  `tests/shell/setup_sentinel_test.sh`. Current and historical tracked prose was
  normalized to the sole product name so the committed tree has no retired-name
  residue while Git history continues to retain the original evidence.
- `tests/static/sentinel_naming_test.sh` reconstructs the retired token only at
  runtime and scans every tracked path and regular-file payload
  case-insensitively. The guard therefore proves zero residue without embedding
  the forbidden token in its own source.

### Local verification

| Check | Exact result |
|---|---|
| Real isolated-HOME Sentinel setup | PASS: cloned `luisgui1757/sentinel`, detached at `ecafffa858666343c1639f996d177f460163e93e`, validated `VERSION=0.1.2`, installed and checked four global entrypoints, and found zero retired-name hits in the checkout or rendered blocks |
| `bash tests/shell/setup_sentinel_test.sh` | PASS: immutable checkout, hostile Git config isolation, dirty/untracked/ignored/core.worktree refusal, wrong-version fail-closed behavior, retry cleanup, and interruption cleanup |
| Focused `Setup.Tests.ps1` | PASS: 55 passed, 0 failed/skipped |
| `bash tests/static/pin_consistency_test.sh` | PASS: POSIX/Windows/docs mirror version `0.1.2` and exact commit `ecafffa858666343c1639f996d177f460163e93e` |
| `bash tests/static/sentinel_naming_test.sh` | PASS: Sentinel is the sole tracked agent-policy product name |
| `make ci` | PASS: complete local pre-PR gate, including 83 reviewed Renovate records |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems were omitted, not promoted to runtime proof |
| `git diff --check` | PASS |

No pushed-head hosted result, review, approval, merge, matching Sentinel release
tag, live safeguard mutation, or merged-main cache-free proof is claimed by this
entry.

## Sentinel Windows analyzer-baseline repair — entry 46

- Commit `c7160db6ffd42522ae5c2d313164ca258dadcdff` reached the hosted Test
  workflow in run
  [`29186835212`](https://github.com/luisgui1757/dotfiles/actions/runs/29186835212).
  Windows job `86634390599` completed all 250 Pester tests successfully, including
  the Sentinel setup cases, then failed only because the exact
  PSScriptAnalyzer baseline still expected three diagnostics removed with the
  obsolete tag-refusal branch.
- Recomputing the analyzer result from the complete tracked PowerShell set
  produced zero errors, 93 `setup.ps1` `PSAvoidUsingWriteHost` warnings, and
  exact warning fingerprint
  `bcc1ab1021d43f70770a1af90d803077bca5ff1d9c023abe94646e544865bf8d`.
  `test.ps1` now binds that exact result; no warning category was suppressed or
  excluded.

### Local verification

| Check | Exact result |
|---|---|
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: exact analyzer baseline, 249 Pester passed with zero failed/skipped, and all Neovim specs |
| Exact analyzer recomputation | PASS: zero errors, 93 setup progress warnings, fingerprint `bcc1ab1021d43f70770a1af90d803077bca5ff1d9c023abe94646e544865bf8d` |

No repaired-head hosted result, review, approval, merge, release tag, live
safeguard mutation, or merged-main cache-free proof is claimed by this entry.

## Three-variant Terminal and frozen-postflight closure — entry 47

- Independent reviews of stale head `bc2124290e8b3860fb56fcbf40c0899ff30027a5`
  disagreed about Windows Terminal Preview and safeguard postflight. The active
  product contract was decisive: current migration/manual documentation already
  required stable packaged, Preview packaged, and portable Terminal discovery,
  while production setup, migration recovery, and uninstall still enumerated
  only two paths. Behavior commit
  `10736d3004823867abe861f67f728d1fe174c6d2` closes that gap through one
  schema-validated target enumerator shared by setup, release migration,
  rollback/retry, acceptance, and uninstall.
- Windows recovery now records `Kind`, canonical `Path`, original existence and
  hash, expected publication presence and hash, and a per-target backup identity
  for all three targets. Recovery remains permissive only while deciding whether
  rollback/retry is safe. Apply completion and explicit acceptance have a
  separate strict boundary: every expected target must equal its exact expected
  hash and every expected-absent target must remain absent before the migration
  can become `applied` or `accepted`. Pre-migration, missing, mixed, and external
  bytes cannot close rollback authority.
- Safeguard apply now freezes check identities plus integrity, review, and owner
  ruleset policy from exact committed objects. Postflight derives integrity,
  classic, review, and owner expectations only from that read-only transaction,
  compares repository/ruleset state with the frozen second capture, and repeats
  the exact-main/clean-source boundary after readback. The deterministic
  after-boundary source mutation case performs the three apply writes, detects
  drift without consuming it as policy, and performs the three-resource
  rollback.
- The Apple-Silicon-only static guard now covers both setup entry points, both
  dependency installers, both release migrators, the safeguard guide, and every
  Nix module. The empty-flag shell regression now captures a nonzero setup exit
  before printing its diagnostic instead of letting `set -e` make that message
  unreachable.
- Historical verification totals remain historical. The current behavior tree
  has 141 tracked shell scripts and all 141 parse. Local `test.ps1` now discovers
  and passes 257 Pester cases with zero failed/skipped; entry 46's hosted 250-case
  result remains exact for its older head. A new hosted total is not claimed
  until the final pushed head runs. The earlier UGR-004 evidence citation named
  a nonexistent `install_deps_failure_accumulator_test.sh`; the actual passing
  regression is `tests/shell/install_failure_accumulation_test.sh`.

### Local verification

| Check | Exact result |
|---|---|
| `git diff --check` and `bash -n` over tracked `*.sh` | PASS: no diff errors; 141/141 shell scripts parsed |
| `make lint` | PASS: strict shell lint |
| `bash tests/static/run_all.sh` | PASS, including the three-resource safeguard transaction, frozen postflight mutation, semantic Probot, required-check, release, Apple-Silicon-only, provenance, and Sentinel naming guards |
| `bash tests/shell/run_all.sh` | PASS, including reachable empty-array diagnostics and dual-SHA logical proof |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: exact analyzer baseline, 257 Pester passed with zero failed/skipped, and all Neovim specs |
| Focused Setup/Upgrade/Uninstall Pester | PASS: 83 cases; Preview-only, divergent three-variant, schema identity, exact acceptance, and all-before-any uninstall recovery included |
| `bash tests/static/repo_safeguards_preflight_test.sh` | PASS: complete apply/restore/rollback suite including after-postflight-boundary source mutation |
| `make ci` | PASS: ended `local pre-PR gate passed` |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems omitted, not promoted to runtime proof |

### Finding status amendments

| ID | Status after entry 47 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-001 | ACCEPTED/FIXED | One canonical enumerator and the setup/migration/uninstall transactions cover stable packaged, Preview packaged, and portable targets independently; acceptance requires exact expected state. | Real three-variant native-Windows apply/rollback/uninstall remains a v0.2.0 release row. |
| UGR-002 | REJECTED (superseded contract) | The expanded active-surface guard passes and only Apple Silicon remains a current macOS product path. | None; historical rows remain evidence only. |
| UGR-015 | PARTIAL | Redirected-path logic plus three-variant identity/recovery is automated and green. | Real redirected/OneDrive/alternate-drive Windows plus three installed variants. |
| UGR-020 | PARTIAL | Postflight expectations and unchanged surfaces are transaction-frozen; deterministic late local drift rolls back all three writes. | Final hosted proof; after merge, exact-main cache-free proof and owner preflight/apply/readback. |
| UGR-021 | PARTIAL | Local implementation/failure injection is complete without promoting it to real WSL, redirected Windows, three-variant Windows, desktop/TCC, or merged-main evidence. | Existing real/manual and post-merge rows. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, roadmap, migration status, upgrade/release/security docs, manual matrix, and this ledger now state the same three-target and frozen-readback contracts with current local counts. | Append final hosted/tag/manual evidence only when it exists. |

No pushed-head hosted result, logical artifact, review, approval, merge, release
tag, live safeguard mutation, or merged-main cache-free proof is claimed by this
entry.

## Universal setup entrypoint and update reconciliation — entry 48

- Behavior commit `56e7703f00983f4a26b245a6ca6e3a2d0adf34c7` makes
  `setup.sh --all` and `setup.ps1 -All` the sole normal installation and
  supported v0.1.0 migration commands from an exact release checkout. POSIX
  setup installs and activates the release-pinned, checksum-verified Nix
  prerequisite when absent. Both entrypoints detect exact live v0.1.0
  ownership, invoke the existing digest-bound side-by-side transaction, resume
  an `applied` recovery at validated acceptance, retain recovery evidence, and
  continue the ordinary idempotent setup phases.
- `--update` / `-Update` now performs that same complete reconciliation before
  the existing proven-owner dependency and synchronous Mason refresh.
  `--upgrade` / `-Upgrade` is an alias. Neither spelling fetches Git, follows a
  moving branch, performs a blanket package-manager upgrade, or rewrites a
  repository lock.
- Pending recovery discovery is fail-closed: only regular, exactly framed
  scalar inputs are consumed; invalid, incomplete, unsafe, or conflicting
  recoveries cannot start a second migration. Explicit
  `DOTFILES_V0_1_CHECKOUT` overrides still pass exact tag-object/commit checks
  and the migrator's full clean-tree, remote, identity, and historical-state
  preflight.
- The first full exact-v0.1 migration run exposed a real recursive-discovery
  defect: a standalone migrator invoked its frozen new-release setup while the
  old config was still live. Both migrators now mark that nested execution as
  an active release transaction, so frozen setup cannot recursively start a
  second migration. The exact historical migration harness reproduces the old
  boundary and now passes through apply, failure rollback, interruption,
  recovery tamper rejection, success, and acceptance.
- Documentation, roadmap, migration status, release notes, greenfield guidance,
  and manual evidence rows now describe the same public interface. The
  annotated v0.2.0 tag and required real-host rows remain release gates; this
  entry does not direct users to an unpublished tag or moving `main`.

### Local verification

| Check | Exact result |
|---|---|
| `git diff --check` and `bash -n` over tracked `*.sh` | PASS: no diff errors; 142/142 shell scripts parsed |
| `make lint` | PASS: strict shell lint |
| `bash tests/shell/setup_universal_entrypoint_test.sh` | PASS: verified Nix bootstrap preview/install activation, update aliasing/order, v0.1 apply/accept/resume, and malformed/unsafe recovery refusal |
| Focused `Setup.Tests.ps1` | PASS: 62 passed, 0 failed/skipped, including Windows setup-owned apply/accept/resume and exact recovery framing |
| `make test-migration` | PASS: exact peeled v0.1.0 POSIX migration, frozen nested-setup recursion guard, rollback, retry boundaries, and config parity |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS: exact analyzer baseline, 263 Pester passed with zero failed/skipped, and all Neovim specs |
| `bash tests/static/run_all.sh` and `bash tests/shell/run_all.sh` | PASS |
| `bash tests/nix/run_all.sh` | PASS |
| `make validate-renovate` | PASS: 83 reviewed dependency records |
| `nix flake check --print-build-logs` | PASS on Apple Silicon; incompatible Linux systems omitted, not promoted to runtime proof |
| `make ci` | PASS: ended `local pre-PR gate passed` |
| Gitleaks `886025c491a456ce5e9cfb2c84575bffce3f7199..56e7703f00983f4a26b245a6ca6e3a2d0adf34c7` | PASS: one commit, no leaks |
| `bash tests/static/sentinel_naming_test.sh` | PASS: Sentinel remains the sole tracked agent-policy product name |

### Finding status amendments

| ID | Status after entry 48 | Exact evidence | Remaining work |
|---|---|---|---|
| UGR-011 | PARTIAL | WSL guest setup now owns verified Nix bootstrap and uses the same one-command entrypoint without relabeling Linux CI as WSL proof. | Real WSL2 Windows-host plus Linux-guest install/migration/retry run. |
| UGR-020 | PARTIAL | Universal setup preserves the exact release transaction and does not mutate live repository safeguards. | Existing merged-main cache-free and owner cutover evidence. |
| UGR-021 | PARTIAL | Fresh and historical setup orchestration is locally green; no real-host or final-tag row is fabricated. | Exact tagged Apple Silicon/Linux/Windows/WSL and redirected/three-variant Windows evidence. |
| UGR-022 | ACCEPTED/FIXED | README, CLAUDE, roadmap, migration status, upgrade/release docs, greenfield/manual guidance, and this ledger agree on setup-all and update/upgrade semantics. | Append final hosted/tag/manual evidence only when it exists. |
| UGR-023 | ACCEPTED/FIXED | Focused shell/PowerShell recovery-shape and nested-migration regression tests fail closed and pass in the full gate. | None beyond the release/manual rows above. |

No pushed-head hosted result, logical artifact, review, approval, merge, release
tag, live safeguard mutation, or merged-main cache-free proof is claimed by this
entry.

## Universal setup hosted-gate repair — entry 49

- Initial pushed head `1663f0affdcc4d256cc26a520b5e53b639804e89`
  produced two real hosted failures in Test run
  [`29194538792`](https://github.com/luisgui1757/dotfiles/actions/runs/29194538792).
  Ubuntu job `86655036690` reached the new universal-entrypoint regression with
  a runner-provisioned `/usr/bin/nix`; the fixture incorrectly inferred that
  restricting `PATH` alone represented a missing-Nix host. Windows job
  `86655036708` completed all 264 Pester tests with zero failures/skips, then
  correctly failed because the analyzer baseline still expected 93 setup
  progress warnings instead of the exact new 101-warning surface.
- Repair commit `63f6cceaca1f39a7f6dd36191dbe2434750a0799`
  makes the Nix bootstrap fixture own its profile-activation system boundary,
  so its missing-to-installed transition is deterministic whether or not the
  host already has Nix. Production prerequisite behavior is unchanged.
- The PowerShell analyzer baseline now binds 101 reviewed
  `setup.ps1` `PSAvoidUsingWriteHost` progress diagnostics and exact warning
  fingerprint
  `ce7e7ae0f8e35956809322b1ce6f055e8878640f1197fa5ebc601d87ba84afaa`.
  No analyzer rule, path, warning, or test was suppressed.

### Repair verification

| Check | Exact result |
|---|---|
| `bash tests/shell/setup_universal_entrypoint_test.sh` | PASS with ambient Nix present; fixture still proves dry-run is write-free and real mode invokes/activates its verified helper |
| `make lint` | PASS |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` | PASS locally: exact 101-warning analyzer baseline/fingerprint, 263 Pester passed with zero failed/skipped, and all Neovim specs |
| Gitleaks `886025c491a456ce5e9cfb2c84575bffce3f7199..63f6cceaca1f39a7f6dd36191dbe2434750a0799` | PASS: three commits, no leaks |
| `bash tests/static/sentinel_naming_test.sh` | PASS |

Entry 48's first-head statement that the exact analyzer baseline passed is
superseded by this measured failure and repair. Historical local Pester and all
other green evidence remain valid; repaired-head hosted evidence is not claimed
until the new head runs.

No review, approval, merge, release tag, live safeguard mutation, or
merged-main cache-free proof is claimed by this entry.

## nix-darwin stale-PATH retry repair — entry 50

- A real Apple Silicon owner-host retry after the first successful nix-darwin
  activation failed before a second activation. The still-open terminal could
  not resolve `darwin-rebuild` on `PATH`, so setup incorrectly re-entered first
  bootstrap and rejected the legitimate retained
  `/etc/{bashrc,zshrc}.before-nix-darwin` recovery files. The Homebrew tap
  transaction rolled back all three staged legacy snapshots, so the failure was
  explicit and left no partial tap migration.
- Current host state proved the alternative explanation false: nix-darwin was
  installed at `/run/current-system/sw/bin/darwin-rebuild`, `/etc/bashrc` and
  `/etc/zshrc` were exact links to `/etc/static/{bashrc,zshrc}`, and both
  original recovery files remained intact. This was an idempotency defect in
  setup discovery, not a user-created backup collision.
- Setup now resolves the installed current-system or system-profile rebuild
  command after ordinary `PATH` lookup. First-bootstrap shell migration skips
  only exact nix-darwin `/etc/static` links; unmanaged sources with occupied
  backup names still fail before either file moves. Rollback continues to own
  only shell files moved by the current invocation.
- Automated coverage reproduces the owner-host shape with a stale `PATH`, an
  installed absolute rebuild command, managed shell links, and retained
  backups. It also directly proves managed-link retry idempotency while keeping
  the prior collision, partial-move, failure, signal, and rollback cases.

### Repair verification

| Check | Exact result |
|---|---|
| Restricted-`PATH` real-host `./setup.sh --all --dry-run` | PASS: selected `/run/current-system/sw/bin/darwin-rebuild`; no bootstrap migration was selected |
| `bash tests/nix/setup_nix_darwin_test.sh` | PASS: stale-PATH installed-generation and managed-link retry regressions plus all prior nix-darwin cases |
| `bash tests/nix/run_all.sh` | PASS |
| `/opt/homebrew/bin/shellcheck setup.sh tests/nix/setup_nix_darwin_test.sh` | PASS |
| `git diff --check` and Bash parse checks | PASS |
| `make ci` | PASS: ended `local pre-PR gate passed` |
| Privileged owner-host retry | PENDING: sudo credential expired; no password was requested or captured by automation |

No pushed-head hosted result, privileged retry, review, approval, merge,
release tag, live safeguard mutation, or merged-main cache-free proof is claimed
by this entry.

## Homebrew tap scan-boundary repair — entry 51

- The next real Apple Silicon owner-host retry reached nix-darwin activation and
  disproved entry 50's remaining assumption about tap rollback storage.
  Setup staged the retired root-owned `nikitabobko/homebrew-tap` snapshot as
  `nikitabobko/homebrew-tap.dotfiles-pre-user-taps-*` below
  `Library/Taps`. Homebrew enumerated that diagnostic directory as another live
  tap, then failed because `aerospace` existed in both it and the replacement
  `nikitabobko/tap` checkout.
- Rollback restored all three original root-owned snapshots but retained the
  failed replacement as `homebrew-tap.dotfiles-failed-*` under the same scan
  root, so a plain retry would reproduce duplicate-tap discovery. This was a
  setup-owned transaction-boundary defect, not a pre-existing Tart/Cirrus
  package conflict and not a reason to uninstall user packages.
- Setup now stores original snapshots, failed replacements, and diagnostic
  recovery trees beside `Library/Taps`, never below it. Before a new migration,
  it recognizes only the exact three tap paths plus exact recovery suffixes
  emitted by the broken predecessor and relocates those artifacts outside the
  scan tree. Unrelated tap names and the whole `Library/Taps` directory remain
  outside the selector.
- The failure regression proves restored originals plus failed replacement
  retention outside the scan tree. The retry regression seeds an old in-tree
  artifact, proves automatic external relocation, proves every activation-time
  backup is external, proves success removes the transaction root, and proves a
  second activation is idempotent.
- `tests/macos_owner_lifecycle.sh` is now the canonical destructive host smoke:
  install, update, config uninstall, reinstall, final update, final full
  greenfield validation, and before/after Homebrew inventory preservation. It
  keeps the password prompt on the owner's terminal and logs no credential.

### Repair verification

| Check | Exact result |
|---|---|
| `bash -n setup.sh tests/nix/setup_nix_darwin_test.sh tests/macos_owner_lifecycle.sh` | PASS |
| `/opt/homebrew/bin/shellcheck setup.sh tests/nix/setup_nix_darwin_test.sh tests/macos_owner_lifecycle.sh` | PASS |
| `bash tests/nix/setup_nix_darwin_test.sh` | PASS, including external transaction, prior-artifact recovery, failed-output quarantine, and second-run idempotency cases |
| Real-host `./setup.sh --all --dry-run` | PASS: selected `/run/current-system/sw/bin/darwin-rebuild`, scheduled the exact stale failed-tap artifact for external relocation, and selected only the three retired root-owned snapshots |
| `make ci` | PASS: ended `local pre-PR gate passed` on the complete repaired behavior/documentation tree |
| Full owner-host lifecycle runner | PENDING until this repaired committed head is invoked from the owner's terminal |

UGR-013 remains ACCEPTED/FIXED at the code/regression level, but the real
owner-host lifecycle row remains pending. No pushed-head hosted result,
privileged lifecycle result, review, approval, merge, release tag, live
safeguard mutation, or merged-main cache-free proof is claimed by this entry.

## Guarded Nix-profile PATH recovery — entry 52

- The first committed owner-host lifecycle invocation at
  `a3fc2f5040fa40434c10580502a115e261062403` stopped before nix-darwin with
  Git's `fatal: Needed a single revision`. A second ordinary run repeated it,
  so retry was stopped and the lifecycle was instrumented.
- The exact trace proved setup saw no `nix` on `PATH`, then sourced
  `/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh`. The login shell
  had already set that profile's sourced guard, so the source correctly no-op'd.
  Setup then misclassified installed Nix as absent, invoked the prerequisite
  installer, and that release-only helper rejected the untagged development
  branch at its tag revision check. This was not a corrupt repository or a
  failed nix-darwin/Homebrew activation.
- The environment shape is valid: Homebrew's `path_helper` refresh can replace
  `PATH` after the Nix profile guard is set. Setup now checks the canonical
  daemon and user profile binaries directly before sourcing guarded profile
  scripts. The lifecycle runner also carries the daemon profile bin explicitly.
- A focused regression sets the profile guard, removes Nix from `PATH`, and
  requires canonical profile recovery without invoking the prerequisite helper.
  A real stale-PATH dry-run reaches installed nix-darwin normally.
- The first post-repair lifecycle install at `3e937f2` then completed all six
  setup phases in 17 seconds. Its post-install assertion—not setup—failed by
  deriving `Library/Taps` from `brew --repository`. Under nix-homebrew that
  repository is `.homebrew-is-managed-by-nix`; live tap clones remain below
  `brew --prefix`. The runner now uses the prefix, and a fake managed-repository
  regression proves both correct discovery and rejection of an in-prefix
  `.dotfiles-failed-*` artifact.
- Separately, the owner ran ordinary `./setup.sh --all` successfully at
  `a3fc2f5`: all six phases completed. Live readback found the active
  current-system rebuild command, target-user-owned `nikitabobko/tap`, and no
  setup recovery artifact under Homebrew's scanned `Library/Taps`. This proves
  the repaired tap boundary's install path, but not yet the complete lifecycle.

### Repair verification

| Check | Exact result |
|---|---|
| Instrumented owner-host lifecycle at `a3fc2f5` | FAIL before mutation: installed Nix was absent from stale PATH and guarded profile sourcing no-op'd |
| Owner-terminal `./setup.sh --all` at `a3fc2f5` | PASS: nix-darwin plus all six setup phases completed |
| Live tap/system readback after install | PASS: current-system rebuild executable, `nikitabobko/tap` target-user owned, no `.dotfiles-*` tap enumerated or stored below `Library/Taps` |
| `bash tests/shell/setup_universal_entrypoint_test.sh` | PASS: guarded canonical-profile recovery plus existing bootstrap/migration cases |
| Stale-PATH `./setup.sh --all --dry-run` | PASS: Nix store and installed current-system rebuild resolved without prerequisite reinstall |
| `make ci` | PASS: ended `local pre-PR gate passed` on the guarded-profile recovery tree |
| Owner lifecycle phase 1 at `3e937f2` | PASS: all six setup phases completed in 17 seconds; runner then failed its incorrect repository-derived tap assertion |
| `bash tests/nix/macos_owner_lifecycle_test.sh` | PASS: prefix/repository split and in-prefix recovery-artifact rejection |
| `make ci` after the lifecycle tap-root correction | PASS: ended `local pre-PR gate passed` |
| Full lifecycle on the new recovery head | PENDING |

The complete install/update/uninstall/reinstall/update row remains pending. No
pushed-head hosted result, completed lifecycle, review, approval, merge, release
tag, live safeguard mutation, or merged-main cache-free proof is claimed by this
entry.

## Cross-platform owner-lifecycle and theme closure — entry 53

- Exact committed head `51c5211b4b3dee4f0758533beac5e18345d668a1`
  completed the digest-pinned local Ubuntu 24.04 arm64 owner lifecycle: real
  Home Manager plus native apt install, update, config uninstall, idempotent
  uninstall retry, reinstall, final update, and 36/36 full validation. Every
  pre-existing native package remained installed. This proves the Linux runtime
  surface in the pinned container, not a physical Linux host or WSL.
- The same run consumed the chezmoi-managed Herdr config at
  `~/.config/herdr/config.toml`; the canonical file forces Herdr's built-in
  `rose-pine` theme with onboarding and automatic light/dark switching disabled.
  POSIX uses the XDG path and native Windows uses an independent roaming
  ApplicationData chezmoi state so the actual application consumer is managed
  on every platform.
- The first pushed CI run on that head exposed two test-host portability
  defects, not installer failures: the macOS lifecycle unit fixture invoked BSD
  `stat` while running on Ubuntu, and ShellCheck rejected a fixture function
  definition that appeared after its first production-function call. The
  macOS-only command is now stubbed only at the cross-host unit-test boundary;
  the universal setup test runs the real production profile recovery in an
  isolated re-source while defining the later bootstrap fixture before use. No
  lint suppression or product-behavior relaxation was added.
- Homebrew completion repair is now setup-owned after activation in install and
  update modes. It runs `brew completions link` through the selected Homebrew,
  fails closed on repair failure, and removes the prior hidden manual recovery
  step for stale `_brew` completion links.
- A config-only owner-host apply then created the real Herdr consumer path at
  `~/.config/herdr/config.toml` as a chezmoi-managed symlink to the canonical
  template. `herdr status` parsed the resulting configuration successfully and
  reported client `0.7.3`, stable channel, with no running server. This is
  application/config parse evidence, not a visual theme-session observation.

### Closure verification

| Check | Exact result |
|---|---|
| `./tests/greenfield/docker-linux-owner-lifecycle.sh` at `51c5211b4b3dee4f0758533beac5e18345d668a1` | PASS: full five-phase lifecycle, 36/36 final validation, all pre-existing packages preserved; host log retained at `tests/.cache/linux-owner-lifecycle-docker-20260713-093950.log` |
| `bash tests/nix/macos_owner_lifecycle_test.sh` | PASS on macOS after the Ubuntu test-boundary repair |
| `bash tests/shell/setup_universal_entrypoint_test.sh` | PASS: real guarded-profile recovery plus verified bootstrap and migration cases |
| `bash tests/shell/lint.sh` | PASS: no SC2218 and no new suppressions |
| `PATH=/opt/homebrew/bin:$PATH make ci` | PASS: ended `local pre-PR gate passed` on the complete repair, proof, and documentation tree |
| `./setup.sh --all --skip-deps --skip-config-scripts --skip-nvim --skip-agents` plus `herdr status` | PASS: live Mac consumer path created, bytes match `herdr/config.toml`, and Herdr parses the managed config |
| Full Apple Silicon owner lifecycle | PENDING: the dedicated owner Terminal is at its one `sudo -v` credential prompt; no credential is captured or automated |

No physical-Linux, WSL, Windows desktop, Herdr visual-session, completed macOS
lifecycle, final pushed-head hosted, approval, merge, release tag, safeguard
mutation, or merged-main cache-free proof is claimed by this entry.

## PowerShell analyzer identity closure — entry 54

- Pushed head `0e5f48bdc32e9cb5b32a1cac26f16ce281b8f816` reached the
  Windows `test.ps1` entry point in hosted Test run
  [`29233944017`](https://github.com/luisgui1757/dotfiles/actions/runs/29233944017).
  PSScriptAnalyzer 1.25.0 retained every reviewed rule/path/count group but
  rejected the stale exact identity fingerprint: expected `5630775a...f090`,
  actual `284a5c26...718a`. The runner then passed all 266 Pester tests and
  invoked every Neovim spec; the job's final exit 1 was the correctly retained
  analyzer failure, not a Neovim failure.
- The stale fingerprint predated this branch's later reviewed changes to
  warning extents in `setup.ps1` and related PowerShell surfaces, including the
  fail-closed Mason command text. Recomputing with the same analyzer version on
  macOS produced the same `284a5c26ff6986b5bb4805367417a09958e5bea39de25edfefc14487c175718a`
  identity as Windows. The baseline is refreshed to that exact reviewed
  identity without changing a rule, scanned path, group count, diagnostic, or
  suppression.

### Repair verification

| Check | Exact result |
|---|---|
| Hosted Windows `test.ps1` at `0e5f48b` before repair | FAIL: exact analyzer identity mismatch; Pester 266 passed, 0 failed/skipped; all 18 Neovim specs were invoked |
| Local PSScriptAnalyzer 1.25.0 recomputation | PASS: exact rule/path/count groups retained; independently produced fingerprint `284a5c26ff6986b5bb4805367417a09958e5bea39de25edfefc14487c175718a` |
| `pwsh -NoLogo -NoProfile -File ./test.ps1` after repair | PASS: analyzer exact fingerprint, 265 Pester passed with zero failed/skipped, and all 18 Neovim specs exited 0 |

The repaired pushed-head hosted result remains pending. No completed macOS
lifecycle, approval, merge, release tag, live safeguard mutation, or
merged-main cache-free proof is claimed by this entry.
