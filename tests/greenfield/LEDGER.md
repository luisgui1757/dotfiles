# Greenfield Evidence Ledger

Append-only record for clean-machine and visual validation. Keep automated
script results separate from manual observations; a desktop surface counts only
when someone inspected it on that machine.

| Date | Environment | Ref / SHA | Command or path | Automated result | Manual visual result | Notes |
|---|---|---|---|---|---|---|
| 2026-06-18 | Static docs guard | `audit/full-roadmap-review-2026-06-18` | `bash tests/static/stale_greenfield_refs_test.sh` | Pass | Not applicable | Ledger created; no Windows Sandbox, WSL, macOS VM, or Linux VM greenfield run recorded by this docs/static update. |

## Pending proof (not evidence entries)

- Intel macOS: wait for the exact ultimate-closure PR head to complete the
  non-required `macos-26-intel` Nix and full setup lanes; record the run URL,
  SHA, image, and result here only after it occurs.
- WSL2, redirected Windows known folders, and desktop GUI/visual behavior still
  require their real environments. Workflow or harness presence does not count
  as a run.

## Entry Template

| Date | Environment | Ref / SHA | Command or path | Automated result | Manual visual result | Notes |
|---|---|---|---|---|---|---|
| YYYY-MM-DD | Windows Sandbox / WSL / macOS VM / Linux VM | branch at commit | exact command or `.wsb` path | pass/fail with log path | pass/fail/skipped rows | remaining manual observations |
