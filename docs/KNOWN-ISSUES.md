# Known release issues

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
[unreleased branch testing procedure](../README.md#testing-an-unreleased-official-branch)
can exercise the corrected source. It does not authorize migrating a live
v0.1.0 installation. Existing-Mac in-place upgrades have not been verified by
the fresh-install checks.

Linux, WSL and Windows do not use the affected macOS Homebrew runtime.
