# Moving to the native installer

This major release replaces Nix and chezmoi with the interactive installer.
There is no old-runtime compatibility mode. Its supported platforms are Apple
Silicon macOS, Ubuntu/Debian amd64/arm64 and native Windows amd64.

## Existing installation

Preserve local checkout changes first. Update to the reviewed new revision, either
in the existing checkout or a separate checkout you intend to keep. Retained
live-link source paths remain passive data so updating the checkout does not
break those links before migration. Pi's removed sources are the explicit
exception described below. This retention does not run an old installer.

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
After successful preparation (and Homebrew conversion on macOS), the normal menu
opens. Choose tools and explicitly
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

## Git configuration handover

Git defaults now use a marked include in a regular personal XDG Git file.
`pull.rebase = true` and `fetch.prune = true` are defaults; personal values later
in that file or in `~/.gitconfig` override them. The include follows the current
checkout, so edits to its defaults take effect without an installer update.

For a released chezmoi Git symlink, migration preserves the original link and
copies every readable setting into a regular file. This includes personal name,
email and other settings that the former `git config --global` write-through
behavior might have added to the source. It does not rewrite that old checkout.
Personal sync links outside the recognized dotfiles layouts stay unchanged.
Copied personal values still override the new defaults; the released default file
has no pull policy, so its migration allows the new rebase default to take effect.
Unreadable links and non-UTF-8 Git files require explicit repair or conversion.

An earlier build of this unreleased PR may already own the whole `config.git`
file. First use **Remove tools → Git** in setup. An unchanged owned file restores
its first personal baseline. For changed files or uncertain ownership with intact
recovery evidence, select the optional **legacy settings restoration**: it saves
the current file separately and restores that first baseline. Settings added to
that copy (including Git identity and credential helpers) become inactive until
you copy them back from the reported recovery path. Declining keeps the file
intact and offers restoration again on the next run; it never makes the obsolete
mapping installable. Saved proof that a first publication was cancelled before
any move, or explicitly restored, releases its bookkeeping without modifying the
personal file. Missing recovery evidence blocks only this configuration. Migrate any restored
legacy link, then select Git again. Migration refuses to detach setup's owned
link. The old unowned receipt is released when the new integration takes over;
its saved backup remains intact. Do not delete installer state to bypass this
handover. On Windows this same flow covers the former whole-file copy.
Retired mappings do not retain Git or managed shell blocks. Any old Keep entry
for such a mapping is dropped from the next plan while its file stays protected.
When Git remains selected during handover, resume setup after the old receipt is
released so the new include can acquire its own baseline.

Git follows `HOME`/`XDG_CONFIG_HOME` on Windows as well as POSIX. An explicit
`GIT_CONFIG_GLOBAL` bypasses both personal global files; unset it before applying
these defaults. If the discovered path changes between shells, setup keeps
removal attached to the originally approved file and reports the changed path.
Remove that integration before selecting Git at the new location.
Git targets that resolve inside the checkout are blocked, including redirected
parent directories. Historical targets that move or go offline cannot block
independent selections; any uninspectable old lock evidence remains disclosed.

## Retiring the earlier installation

Migration includes a separate retirement phase after the replacement setup
passes its check. On macOS, Nix-managed Homebrew is converted before setup;
its installed formulae, casks and taps must remain unchanged during conversion.
Conversion preserves the old Homebrew source beside its original paths and
restores the reviewed Homebrew 7.0.9 repository. It reads installation records
directly, so the old Homebrew need not understand newer cask definitions. It does
not run a global Homebrew update or migrate packages or taps. Redirected taps,
submodules, partial clones and executable Git filters require inspection before conversion;
preview never runs personal tap filter commands, including worktree-specific
settings. The same protection applies to legacy plugin checkouts. Empty test
submodules from the released plugin setup are recognized; populated or cached
submodule data stays. History inspection refuses partial clones instead of
downloading missing objects.
The later retirement preview lists the exact old tools, files, services and
accounts it can remove, including administrator actions and permanent Nix store
deletion. Check each item you want to retire and review that preview.

Nix removal first handles the earlier login shell, Home Manager links and
nix-darwin system integration. It checks other accounts, garbage-collection roots,
mounted filesystems and running processes before deleting the store. Close old
Nix-based terminal sessions and open a fresh login session if processes still use
it. Packages in the Nix store disappear; the preview calls out replacements you
did not select. Personal Nix configuration remains, and earlier state/configuration
is preserved at the listed recovery locations. The store itself is not backed up.

Migration identifies upstream Nix and compatible Determinate installations from
their on-disk evidence; you do not need to choose the installer type. Determinate
removal validates the vendor receipt and native account/service/volume identities.
The uninstaller must be protected and root-owned, or match the reviewed official
release hash for its exact Mac version. The ordinary Mac installer preserves the
installing user's ownership; migration authenticates those bytes without running
them during preview. Root then copies and verifies both the binary and receipt
in a private directory before execution. The preview identifies that trust source,
the retained temporary copies, and the system Nix configuration backup.
Unknown receipt actions or unrelated consumers block removal locally.
Stale links inside root's private temporary directory are checked read-only with
existing administrator credentials. If they cannot be inspected, migration stops
without treating them as absent.
The same bounded inspection handles vendor configuration and remaining boot-time
files readable only by root, without changing their permissions. If the preview reports missing administrator
credentials, run `sudo -v` and retry `migrate`; machine mode never prompts.
The receipt's Nix services are stopped before the final process check. If an
outside process still uses Nix, removal stops with the store intact; close that
process and resume migration. The services remain stopped during this pause.
On Linux, inactive, failed or unloaded units do not block recovery after a
restart or partial uninstall. Socket and service activation links are recorded
in the preview and
removed only when unchanged. This includes systemd links resolved into the Nix
store; upstream retirement removes those links before their source disappears. Older saved evidence cannot authorize a newly
observed link; a changed approval requires a fresh preview. If an older unfinished
stop recipe keeps failing on a missing or failed unit, abandon that operation and review
migration again; saved commands are not silently rewritten.

If a Nix upgrade changed the profile, or the store was deleted outside migration,
an old store-resolved activation link may no longer be provable. Migration keeps
it and names its exact path. After confirming that it belongs to the old Nix
installation, remove only that link with the displayed `sudo rm -- <path>`
command, then rerun `migrate`. Do not substitute a store or unit-file path.
This advice applies only to links in `sockets.target.wants` or
`multi-user.target.wants`. Primary unit/tmpfiles links and files or directories
at activation paths stay for manual inspection.

On Linux, retirement switches the earlier Linuxbrew zsh login to setup's selected
native zsh, or system Bash when zsh is unselected, before removing the formula.
It can also repair this account's login if that formula was already removed.
Another account using the formula keeps it installed; migration never changes
other accounts' login shells.

chezmoi retirement removes the verified executable through its original provider;
it never runs `purge` or `destroy`. An independent chezmoi source blocks its
removal. Pi session data, personal configuration, and unrelated packages stay.
Exact direct-artifact and plugin evidence covers the published `v0.4.4` and the
pre-overhaul `main` commit `ae9a6446`; a filename alone is not removal authority.

Unchecked or unrecognized items remain visible and keep migration incomplete.
Changes to the installation after preview require another review. Keep the old
checkout and recovery files until you have checked your personal data and adopted
applications. Running setup repeatedly reconciles its saved selection and verified
resources; it does not run the old installer again.

An earlier completed retirement does not select a later reinstallation for
removal. Each migration respects the current checkboxes and keeps the previous
operation history and recovery paths. Linux migration also detects tools that
the released installer installed through Linuxbrew; new Linux setup uses APT
and private archives. Shared package-manager infrastructure and unproved or
externally required dependencies stay installed. Migration never runs Homebrew
autoremove or removes another application's dependencies. The final report also lists
intentionally kept components, including shared Windows packages and a native
font family reused by the selected setup.

An interrupted upstream Linux store deletion records its approved membership and
holds a separate deletion lock. If its worker is lost, recovery requires a fresh
approval of the remaining files after the earlier deletion has stopped. Newly
added root/store members block recovery. A removed default-profile manifest can
be recovered only with saved proof that it contained installer runtime packages;
an older journal without that proof cannot authorize another account's files.

## Interruptions and conflicts

An unfinished transaction from a released old installer must first be recovered
with that exact retained release and recovery directory. Migration reports the
blocking evidence and does not start a second operation over it.

For a new installer interruption, finish recovery **before updating the checkout**.
Resume requires the original installer executable and its matching checkout,
including the original configuration sources. If neither has changed, reopen the
same `setup` or `migrate` entrypoint and use its recovery menu. Restoration uses
saved physical artifacts without overwriting newly created personal files;
abandonment is offered only when the provider can prove it safe. Neither action
is a general substitute for resuming with the original executable.

The recovery diagnostic's **executable SHA-256** identifies the executable's
bytes, not a Git revision. The cache filename uses a different build-input key;
do not put the reported digest into a guessed filename or pass it to `git checkout`.
If you already updated, preserve your edits and recover the original matching
checkout contents at the original location from your retained revision/history.
Keep the original account and state-location settings. Locate the retained
executable by hashing the cache entries, replacing the placeholder with the full
64-character digest from the diagnostic:

```bash
wanted='EXECUTABLE_SHA256_FROM_DIAGNOSTIC'
cache="${XDG_CACHE_HOME:-$HOME/.cache}/dotfiles/bootstrap"
for candidate in "$cache"/dotfiles-*; do
  [ -f "$candidate" ] && [ -x "$candidate" ] && [ ! -L "$candidate" ] || continue
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum < "$candidate" | awk '{print $1}')
  else
    actual=$(shasum -a 256 < "$candidate" | awk '{print $1}')
  fi
  [ "$actual" != "$wanted" ] || printf '%s\n' "$candidate"
done
```

```powershell
$wanted = 'EXECUTABLE_SHA256_FROM_DIAGNOSTIC'
$cache = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'dotfiles/bootstrap'
Get-ChildItem -LiteralPath $cache -Filter 'dotfiles-*.exe' -File |
    Where-Object { -not ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) } |
    ForEach-Object {
        if ((Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash -ieq $wanted) {
            $_.FullName
        }
    }
```

Run only the matching executable from your retained trusted cache, explicitly
binding its original checkout. Use `migrate` instead of `setup` when recovering
a migration transaction:

```bash
DOTFILES_CHECKOUT='/absolute/path/to/original-checkout' DOTFILES_ENTRYPOINT=setup \
  '/exact/matching/cache/executable'
```

```powershell
$oldCheckout, $oldEntrypoint = $env:DOTFILES_CHECKOUT, $env:DOTFILES_ENTRYPOINT
try {
    $env:DOTFILES_CHECKOUT = 'C:\absolute\path\to\original-checkout'
    $env:DOTFILES_ENTRYPOINT = 'setup'
    & 'C:\exact\matching\cache\executable.exe'
} finally {
    $env:DOTFILES_CHECKOUT, $env:DOTFILES_ENTRYPOINT = $oldCheckout, $oldEntrypoint
}
```

If no cached executable matches, retain the transaction and recover the original
binary or reproduce its exact original build; verify the required executable
SHA-256 before resuming. A matching checkout name or Git revision alone does not
prove the executable identity. Do not delete state, profile locks or journals to
manufacture a clean install. A changed file, unreadable baseline or live consumer
remains a visible action item until its real cause is resolved.

## Later updates and removal

OpenWhispr, Obsidian, Tailscale and Zen are new optional selections; updating
the checkout does not select them automatically. Consult the README platform
table for current availability. Existing applications and personal notes,
browser profiles, dictation settings and Tailscale credentials are not adopted
implicitly. Tailscale is removed without package purge. Finish pending native
operations with their original executable before advancing the checkout.

First complete any pending operation with its original executable and checkout.
Then update the checkout to a reviewed revision, open `setup.sh`/`setup.ps1` and
choose **Update selected tools**. Updates preserve your selections and reconcile
only the declared pins/native packages. Complete Update/Repair before opening
managed Neovim after changing checkout pins: Lazy pin drift stops configured
startup, and Mason registry drift disables its language-server/formatter setup.
A failed update leaves those mismatches visible until repair succeeds; ordinary
editor startup never rewrites the published generation.

Use **Remove tools** for removal and
review its shared-dependency choices. Personal data and pre-existing packages are
preserved; unchanged owned resources are removed/restored surgically.

Claude Code and Codex CLI are separate selections. Select either or both after
migration; their standalone binaries do not require Node/npm. Authentication,
sessions and personal CLI settings stay local. Claude's managed shell disables
its own updater; use the setup menu's Update action for reviewed CLI pins.

Pi has been removed, including its copied themes/keybindings and Sentinel target.
Existing released Pi packages and personal data remain outside this installer's
ownership. Keep an old checkout if a personal Pi configuration still links to it.
If testing an earlier unreleased native-installer revision, first remove Pi and
its old Sentinel selection using that exact revision, and finish any pending
operation there before advancing. Reinstall Sentinel after updating to publish
its current consumer targets. Never discard state or recovery journals.

The POSIX launchers create private cache/staging files under umask `077`, then
restore the caller's umask before starting the installer. They do not change your
shell's file-creation policy.
Both launchers isolate the compiler's module/build caches; personal Go settings
remain intact when the installer starts.

For historical v0.1.0-to-v0.4.4 procedures, read
[the immutable former guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/UPGRADING.md).
Those commands belong to their corresponding release, not this installer.

Desktop app additions (2026-10-13 candidate): select each independently through
setup. Linux Obsidian uses a private Flathub stable installation and keeps its
runtimes/app data on removal. Ordinary user/system Flatpak stores are not managed.
Linux OpenWhispr amd64 uses the official portable archive: its upstream launcher
may disable the Chromium sandbox on restricted-userns systems, disclosed before
approval. No Flatpak or Linux arm64 build is currently published. Zen Linux
updates through setup and uses a stable personal profile across package generations.
Tailscale uses normal package removal, preserving service configuration and state;
removing machine-wide owned packages requires the separate removal choice.


## T3 Code

T3 Code is a new optional selection, never an automatic migration or provider
replacement. It uses the official standalone package with a browser UI on Apple
Silicon macOS, Ubuntu/Debian amd64/arm64 and Windows amd64. The fixed stable
`v0.1.0` pin supplies Orchestrator V2. Finish any pending nightly installation
with its original checkout first; then stop T3, update the checkout and choose
**Update selected tools** to replace the managed nightly with stable. Personal
T3 state stays outside the package; back it up before running a newer server.

Run `t3` in a fresh terminal, then configure a supported provider when ready
to run agents. Existing Codex/Claude CLI logins are supported; T3's Connect with
ChatGPT can instead install its own managed Codex runtime. Codex and Claude Code
remain independent installer choices. Stop T3 before Update, Repair or Remove. No service or authentication is
configured. Existing binaries and personal `~/.t3`/`T3CODE_HOME`, conversations,
settings, credentials and project data are not adopted or removed. Use the
installer's lifecycle for its private package; T3's own install/update/uninstall
commands manage a different installation layout.


Before launching V2 against an existing T3 home, stop the server and copy its
`userdata` directory to a safe backup. Upstream copies the V1 database once into
V2; later conversations diverge while settings, attachments and workspaces are
shared. Use `t3 --base-dir /path/to/separate-t3-home` (or `T3CODE_HOME`) for an
isolated trial. This installer does not perform or verify that upstream migration.
For dotfiles-managed Codex/Claude, use dotfiles Update, not the provider Update
or Update all buttons in T3; those can invoke the CLI's own updater.

### Zen macOS launcher correction

If an earlier unreleased installer created Zen's macOS launcher and the app
does not open, finish any pending operation with its original checkout first,
then use Update in this revision. The new preparation resolves the official
bundle's physical executable path before launch, fixing its XPCOM startup failure,
and declares ARM64 so macOS does not run the script launcher under Rosetta.
The app version and vendor signature are unchanged. Profiles and browser data
remain outside the managed payload and are preserved.

The revision-1 to revision-2 macOS launcher transition uses the existing approved
archive Update and publication lifecycle. Native LaunchServices re-registration
after that particular transition has not been exercised; the native desktop gate
covers fresh installation, ordinary use and an unchanged-pin Update.
