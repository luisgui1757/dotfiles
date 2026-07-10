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
