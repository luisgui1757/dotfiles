# Moving to the native installer

This major release replaces Nix and chezmoi with the interactive installer.
There is no old-runtime compatibility mode. Its supported platforms are Apple
Silicon macOS, Ubuntu/Debian amd64/arm64 and native Windows amd64.

## Existing installation

Preserve local checkout changes first. Update to the reviewed new revision, either
in the existing checkout or a separate checkout you intend to keep. Released
live-link source paths remain passive data so updating the checkout does not
break those links before migration. This retention does not run an old installer.

From the new checkout run:

```bash
./migrate.sh
```

```powershell
.\migrate.ps1
```

Migration previews the detected shell profiles. Check each profile you want to
detach, then approve its exact preview. The original artifact and a readable copy
are retained at the reported recovery paths. Exact released bytes are recognized
from pinned evidence; unknown/personal commands are never copied automatically
into the new shell. Review those saved files and reapply personal settings you
still need. For the released bash-to-zsh hook, only the exact marked block is
removed; surrounding bytes stay intact.

Unselected profiles stay unchanged and migration reports the remaining work.
After successful preparation, the normal menu opens. Choose tools and explicitly
adopt existing tool configurations where offered. The installer preserves their
first baseline and uses that same baseline for later removal. A completed
migration receipt is historical evidence: rerunning migration cannot replay the
old detachment over newly installed profile blocks.

The new Windows installer discovers actual known folders independently, including
redirected Documents and Start Menu Programs. Neovim uses a directory junction;
other configuration uses copies/scoped settings. Developer Mode is not required.
Windows Terminal manages only its pinned stable unpackaged instance. Existing
Store, Preview and Canary installations remain personal. If that instance still
contains ambiguous legacy combined actions, launch it once to let Terminal migrate
its own settings, then retry the reviewed setup.

## Retained infrastructure

Migration does not claim or uninstall existing Nix, Home Manager, nix-darwin,
chezmoi state, native packages or login-shell settings. Those may serve unrelated
software, and old installer inventory is not removal authority. Their retention
is disclosed. The new installation runs without them; removing such shared
infrastructure is a separate owner-managed task after checking its other users.
Keep the old checkout/recovery files until you have checked your personal data and
all adopted applications. Do not delete an old Nix-provided login shell while it
is still your account's login shell; change it through the OS account settings
before retiring that infrastructure.

## Interruptions and conflicts

An unfinished transaction from a released old installer must first be recovered
with that exact retained release and recovery directory. Migration reports the
blocking evidence and does not start a second operation over it.

For a new installer interruption, reopen the same checkout entrypoint and use the
recovery menu. Resume binds to the saved source and operation; restoration uses
saved physical artifacts without overwriting newly created personal files.
Do not delete state, profile locks or journals to manufacture a clean install.
A changed file, unreadable baseline or live consumer remains a visible action
item until its real cause is resolved.

## Later updates and removal

Open `setup.sh`/`setup.ps1` and choose **Update selected tools** after updating the
checkout to a reviewed revision. Updates preserve your selections and reconcile
only the declared pins/native packages. Use **Remove tools** for removal and
review its shared-dependency choices. Personal data and pre-existing packages are
preserved; unchanged owned resources are removed/restored surgically.

For historical v0.1.0-to-v0.4.4 procedures, read
[the immutable former guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/UPGRADING.md).
Those commands belong to their corresponding release, not this installer.
