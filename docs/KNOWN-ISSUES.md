# Known release issues

## Native major-release candidate (2026-10-10)

Current unresolved work and native failures are tracked in
[installer status](installer-status.md), with causes and corrections in the
[review ledger](reviews/2026-10-08-interactive-installer-overhaul.md). The new
installer replaces the Nix/chezmoi runtime and the executable zsh publisher
described below. Its scoped profile blocks preserve surrounding login settings;
whole-file replacement guidance below applies only to the historical releases.
See [the current migration guide](UPGRADING.md). Historical advisories remain
below as release-specific evidence.

## Fixed in source: user-local commands missing from zsh login shells

Native tools installed in `~/.local/bin` worked in interactive terminals but
were missing from non-interactive login shells used by remote desktop apps.
The managed `.zprofile` now adds that directory without loading interactive
configuration. This applies to macOS/Linux/WSL; Windows ignores the new target.
See [verification and review](reviews/2026-10-09-login-shell-path.md).

## macOS: v0.4.4

`v0.4.4` pins Homebrew 6.0.1, which is incompatible with current AeroSpace and
Ghostty casks. Fresh macOS installation with that release can fail. The release
is immutable and keeps that runtime even after the source fix is merged.

Do not use v0.4.4 for a new macOS installation or a v0.1.0 migration. Use a later
release containing the [Homebrew compatibility fix](https://github.com/luisgui1757/dotfiles/pull/85).
If no such release has been published, wait. The fix selected Homebrew 7.0.4;
later releases containing it are outside this advisory's affected version.

For a greenfield or already-current-release test machine, the
[historical unreleased branch testing procedure](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/README.md#testing-an-unreleased-official-branch)
can exercise the corrected source. It does not authorize migrating a live
v0.1.0 installation. Existing-Mac in-place upgrades have not been verified by
the fresh-install checks.

Linux, WSL and Windows do not use the affected macOS Homebrew runtime.

## Login-profile adoption and CI result formats, 2026-10-09

- The new managed `.zprofile` replaces an existing target. Preserve still-needed
  local login settings in `.zprofile.local` first. Normal setup makes a timestamped
  backup; direct selective chezmoi apply needs an explicit preflight/backup.
  Native Windows ignores this POSIX profile.
- Fixed in source: Hyperfine 2.0 moved the mean to
  `results[].summary.time_wall_clock.mean`. The prompt budget check accepts both
  the previous and new formats, validates units/numbers, and keeps the 80/150 ms
  limits. [Upstream schema](https://github.com/sharkdp/hyperfine/blob/v2.0.0/src/export/json.rs).
- One Ubuntu run failed the existing concurrent plugin-publisher test. Thirty
  isolated Linux repetitions and the next hosted run passed that test; no defect
  or fix is claimed from that evidence. The hosted retry then exposed the
  Hyperfine schema failure above. Both original logs are retained in the task
  archive. The final CI run must pass without excluding either test.

### Concurrent publisher cause confirmed and fixed in source

A four-worker Linux run reproduced the publisher failure. Syscall tracing showed
one `mkdir` succeed and another return `EEXIST`, while both utilities reported
success. uutils coreutils 0.10.0's precheck/error handling permits this race.
[Upstream implementation](https://github.com/uutils/coreutils/blob/0.10.0/src/uu/mkdir/src/mkdir.rs).
The publisher now creates a Bash noclobber claim inside the lock directory.
Only one contender can create that file, even if mkdir reports success twice.
The lock/PID protocol and cleanup remain; there is no new runtime dependency.

The ordinary concurrent test remains. An added external-command fixture models
the faulty mkdir success deterministically; it fails before and passes after.
Both children are awaited and their failure logs are retained in CI output.
The corrected publisher passed 100 Linux runs with four parallel workers.
The earlier "unreproduced" note records the initial investigation, not the
current finding status. Full gates and final review follow.

### Bootstrap ordering regression in the first lock correction

The first correction used Python `os.mkdir` and passed local/concurrency checks,
but the clean Ubuntu container showed that install-deps publishes plugins before
installing Python. That correction was not deployed. The final Bash claim fixes
the same race without changing installer order. A minimal-tool PATH regression
reproduces the missing-Python failure before the correction and passes afterward.
Fresh hosted checks must still prove the full container install.
