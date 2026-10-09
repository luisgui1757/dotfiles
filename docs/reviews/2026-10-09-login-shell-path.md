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
