# Known release issues

## v1.0.1 (2026-10-28)

Replaces v1.0.0, which was tagged but never published: its certification runs
failed every macOS Node download on degraded hosted networking. v1.0.1 adds
resumable archive downloads. The open items listed for v1.0.0 below still apply.

## v1.0.0 (2026-10-25, not published)

On a managed Windows laptop, "Inspecting your system" can take some minutes.
The cause is not known.

## v0.4.4 on macOS

`v0.4.4` pins Homebrew 6.0.1. That version cannot install the current AeroSpace
and Ghostty casks, so a fresh macOS installation with that release can fail. The
release is immutable and keeps that Homebrew version.

Do not use v0.4.4 for a new macOS installation or for a v0.1.0 migration. Use a
later release that contains the
[Homebrew compatibility fix](https://github.com/luisgui1757/dotfiles/pull/85).

Linux, WSL and Windows do not use the affected Homebrew runtime.
