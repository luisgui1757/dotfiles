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
