# Herdr Homebrew compatibility

## Problem and scope

Homebrew supplied Herdr 0.9.3 during a fresh Ubuntu 26.04 installation of
dotfiles v0.4.4. The public `setup.sh --all` completed successfully and Herdr
accepted the managed config, but hosted POSIX verification only admitted 0.7.x.
The old workflow snippet reproduced the failure against that installed binary:
`FAIL: Herdr stable identity is outside the reviewed 0.7.x line: herdr 0.9.3`.

The runtime gate now accepts stable 0.7.x from 0.7.5 and stable 0.9.x from 0.9.3,
then requires the installed config to parse. The 0.7.5 floor matches the existing
README and supply-chain contract; the previous implementation accidentally
admitted 0.7.4. Other minor lines and prereleases remain unreviewed. Homebrew is
still a mutable package channel; this gate is compatibility evidence, not an
artifact pin or a guarantee about future patches. Direct Linux v0.7.5 and the
Windows preview pins are unchanged.

## Upstream review

Reviewed official release notes for
[0.8.0](https://github.com/herdrdev/herdr/releases/tag/v0.8.0),
[0.9.0](https://github.com/herdrdev/herdr/releases/tag/v0.9.0),
[0.9.2](https://github.com/herdrdev/herdr/releases/tag/v0.9.2), and
[0.9.3](https://github.com/herdrdev/herdr/releases/tag/v0.9.3).
The removed standalone `--no-session` mode and pane graphics API are not used
by the managed configs. The current CLI, config parser and persistent-server
behavior were exercised on Linux; Windows and macOS runtime behavior still
require their own hosted/manual proof. The repository's Windows preview was
not upgraded based on Linux results.

## Verification record

- Baseline `make ci` passed at `5154aae11097eb95215113c712ce28fbfe6b4427`.
- Before: the exact previous workflow guard failed on installed Herdr 0.9.3.
- After: `scripts/check-herdr-runtime.sh` passed against the same real binary
  and the installed POSIX config. The Windows config also parsed with that
  Linux binary; this is parser evidence only, not Windows runtime evidence.
- A real terminal attached to a prepared Herdr workspace, invoked `Ctrl+B w`,
  and detached through `Ctrl+B q` with exit 0. A second attach/detach exercised
  reconnect; the pane shell remained alive between clients. This initial PTY
  probe did not exercise provider login or Mosh; later Mosh proof is noted below.
- `tests/shell/herdr_runtime_test.sh` covers both accepted release lines,
  minimum patch boundaries, malformed/prerelease/unreviewed identities, large
  patch components, failed version probes, missing configs and config rejection.
- `tests/static/herdr_theme_test.sh` keeps the actual hosted workflow connected
  to the runtime helper. Release policy behavior is tested through the CLI
  boundary rather than literal copies of its implementation.
- Post-change `make ci` passed. The system Bash 3.2 behavioral test and focused
  ShellCheck also passed. Local platform skips remain explicit in that gate;
  they do not substitute for remote CI. Independent review is a separate
  delivery gate and is not implied by these test results.

## Decisions and rejected alternatives

Claude Opus 5.5, requested at xhigh effort, identified the stale CI bound during
read-only consultation. It advised retaining the existing dotfiles layout and
testing the newer runtime before changing the gate. This consultation is
separate from final independent review.

A plain non-interactive `zsh -c` does not load the interactive tool PATH. That is
intentional shell behavior, not an installer failure. Services must set their
own executable paths/environment. No `.zshenv` PATH change, new headless profile
or unrelated package trimming is justified by this reproduction.

## Independent review follow-up

The first Opus 5.5/xhigh review found no blocking code defect. Its low findings
and evidence requests were assessed as follows:

- FIXED: the missing-config test now lets the external command accept its path,
  so only the script's own readability guard can reject it. Missing-command and
  wrong-argument cases are covered too. README/security bounds are tied to the
  versions exercised by the behavioral test.
- FIXED: removed the unrelated shell-environment sentence from the Herdr guide
  bullet; the rejected diagnosis remains documented above.
- VERIFIED: the old `ogulcancelik/herdr` GitHub API endpoint resolves to
  `herdrdev/herdr` as of 2026-10-05. The new release-note links use that canonical
  owner. Changing existing installer pins or Renovate ownership is outside this
  compatibility fix; their redirect remains valid.
- VERIFIED: explicit POSIX and Windows config paths both return exit 0 on the
  recorded Linux 0.9.3 binary. Invalid TOML and an unknown `keys` entry each
  return exit 1, proving the explicit config path is consumed. The binary's
  resolved path, SHA-256 and before/after exit codes are recorded with the
  deployment evidence.
- VERIFIED: the strengthened PTY probe waits for the navigator-specific
  `Go to` title and checks that the pane shell PID survives detach. Two runs
  passed with the same shell. A separate real Mosh session also exercised the
  navigator and clean detach; this does not certify enhanced modifier transport.
- VERIFIED BY SOURCE: v0.9.3 still maps `goto` to `OpenNavigator`, supports
  `workspace_picker`, and maps indexed `switch_workspace` and `focus_agent`
  to their corresponding actions in
  [input/keybindings.rs](https://github.com/herdrdev/herdr/blob/v0.9.3/src/input/keybindings.rs).
  [config/sidebar.rs](https://github.com/herdrdev/herdr/blob/v0.9.3/src/config/sidebar.rs)
  retains validated agent `rows`, `state_text` and `row_gap`. Full agent-provider
  workflows and every enhanced key chord are separate runtime coverage.
- RETAINED WITH REASON: historical v0.7.5 Shift+Enter attribution describes the
  unchanged pinned Linux binary. It is not evidence of 0.9.3 transport behavior.
  macOS/Windows runtime and remote CI results must still be reported separately.

The follow-up review found another low-severity fixture issue: wrong-argument
tests inherited the missing-config path. FIXED by resetting that path and
asserting no config command runs. Isolated mutations of the argument-count
and readability guard make their corresponding tests fail. The separate Mosh
probe has a saved successful trace; it checks navigator absence before opening
it, selects the prepared pane with Enter, detaches, and verifies the preserved
shell PID.
Its earlier Escape-based automation timed out over Mosh; Enter avoids that
input-timing dependency. This is a probe correction, not a Herdr keymap change.
The original guest-PTY checks were rerun twice successfully. The upstream-owner
record now includes the requested URL, HTTP 301 and redirect target.

## Owner-authorized additional correction round

The owner explicitly authorized one additional correction/review round after
the second review. This does not reset the earlier round count. The wording
above now identifies the two mutation checks actually performed, rather than
claiming mutation proof for every production guard. No Herdr runtime behavior,
configuration, or Windows/Linux artifact pin changes in this round. The macOS
review record documents the remaining cross-platform corrections and evidence.
