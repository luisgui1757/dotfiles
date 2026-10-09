# Remote login command path, 2026-10-09

## Scope and decision

Native user-local tools are available from `.zshrc`, but remote desktop clients
start a non-interactive login shell that does not read it. A real VM probe found
neither coding CLI through `zsh -lc`; `zsh -lic` found both.

Add a small chezmoi-owned `.zprofile`, with a canonical `shells/zprofile` twin.
It adds an existing user-local bin directory once and emits no output. Loading
`.zshrc` would invoke interactive widgets; putting PATH changes in `.zshenv`
would affect every zsh and break the existing ownership convention. Neither is
needed. This intentionally does not initialize package managers in login shells.

Native Windows ignores the target. WSL uses the Linux login-shell behavior.
The parity manifest checks both source copies and the installed target; the
template matrix checks the Windows exclusion.

## Verification

- Baseline `make ci` passed before edits.
- New real-shell regression failed before the fix: the synthetic native command
  could not be found by a non-interactive login shell.
- Final gate, Linux runtime verification and Opus 5.5/xhigh review are pending.

This is a source fix, not a new release. No Windows profile or installer package
selection changed. Review and deployment receipts will be appended below.

## Local gate

The final real-shell regression passes for command resolution, silent startup,
paths containing spaces, duplicate prevention and a missing user-local directory.
`make ci` passed, including source/apply parity and the Windows ignore-template
matrix. The native Windows apply test now also asserts `.zprofile` stays absent.
Actual Linux VM verification, hosted cross-platform checks and independent
Opus review follow; they are not implied by the Mac gate.

## Opus review and corrections

Opus 5.5/xhigh requested safe adoption of an existing profile and confirmation of
Codex's executable type. Added `.zprofile.local`, documented migration before
replacement, and extended the backup/uninstall round-trip with a pre-seeded
profile. The hook regression failed before and passes after the change.

The real VM has no `.zprofile`. Its Codex is a static ELF standalone binary,
0.161.0; Claude is native 2.1.285. Both execute by absolute path. The conditional
Node-backed concern is therefore not applicable to this deployment. A managed
login-shell apply and actual `zsh -lc` version checks remain pending review.

All initial Windows and macOS hosted jobs passed. Ubuntu first exposed an
unreproduced concurrent publisher failure, then a confirmed Hyperfine 2 schema
incompatibility. The existing publisher passed 30 isolated Linux repeats and the
retry. The performance parser now handles both upstream formats without changing
the budget, with legacy, schema-2, unit, invalid-number and boundary tests.
Evidence and final hosted/review/deployment results follow; prior pending entries
above describe the state when they were written.

Correction-round local `make ci` passed in full, including the profile hook,
pre-seeded profile backup/restore and both Hyperfine JSON shapes. The actual
VM login shell still fails before deployment with `codex: command not found`
and exit 127, while both native binaries run directly. Those captured results
will accompany the next independent review. Hosted checks are rerun on this
correction commit.
