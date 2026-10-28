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
The released install's Homebrew block in `.zshrc.local` stays as personal
configuration. It runs after managed shell setup and can put Homebrew ahead of
managed tools; review it if you keep duplicate tools or want different PATH
precedence.
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
file. First untick **Git** in setup's **Choose tools**. An unchanged owned file restores
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
Scroll through the complete scope before confirming: it names native identities,
remaining steps, preservation paths and anything retained. The summary counts
these as removals; Homebrew conversion is a change. Administrator counts describe
the actual remaining steps, including when resuming an interrupted retirement.

The released setup also wrote Sentinel 0.1.2 policy for OpenCode
(`${XDG_CONFIG_HOME:-~/.config}/opencode/AGENTS.md`) and Pi
(`~/.pi/agent/AGENTS.md`). Pi is not supported now. The retirement item for the
released Pi policy deletes `~/.pi/agent/AGENTS.md` only when it holds exactly
the released policy bytes. A file with other text, or a link, stays complete.
The preview names it, and migration still completes.

Migration does not delete the OpenCode file, because OpenCode is now a Sentinel
host. When you select Sentinel rules and skills, Sentinel replaces the old block
in that file. Text outside the block stays.

If migration stops at a phase, it names that phase and points to the report it
already printed instead of repeating it. A check report closes with its screen,
so its stop message repeats it in full.

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
Unreadable Neovim runtime journals also block that resource locally. Deselecting
it preserves the runtime; other tools remain available. If an active operation's
prior publication cannot be proved, Abandon refuses and identifies the journal
that must be recovered or inspected.
Stale links inside root's private temporary directory are checked read-only with
existing administrator credentials. If they cannot be inspected, migration stops
without treating them as absent.
The same bounded inspection handles vendor configuration and remaining boot-time
files readable only by root, without changing their permissions. If the preview reports missing administrator
credentials, run `sudo -v` and retry `migrate`; machine mode never prompts.
The receipt's Nix services are stopped before the final process check. If an
outside process still uses Nix, removal stops with the store intact; close that
process and resume migration. The services remain stopped during this pause.
On Linux, an incomplete process inspection also stops removal. This includes
processes changing address space during the check and a main thread exiting
while workers remain alive. The diagnostic identifies the process and reference;
finish or close that process before resuming. An unreadable process is never
assumed to have stopped using the store.
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

Some leftovers can never be proved safe to remove, for example when a managed
Windows laptop has no winget source or a package was updated outside the released
installer. Once everything else is retired, interactive migrate asks whether to
keep the remaining items and finish. Checked items stay exactly as they are; the
report lists each one with its folder, file or package name under "Kept", so you
can remove it yourself, and migration completes. Unchecked items, or any later
change to a kept item, leave migration unfinished and appear in the next run.
Interrupted, blocked or uninspectable retirements are never offered as kept.

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
choose **Update tools**. Updates preserve your selections and reconcile
only the declared pins/native packages. Complete Update before opening
managed Neovim after changing checkout pins: Lazy pin drift stops configured
startup, and Mason registry drift disables its language-server/formatter setup.
A failed update leaves those mismatches visible until repair succeeds; ordinary
editor startup never rewrites the published generation.

Untick tools in **Choose tools** to remove them and review the shared-dependency
choices on the confirmation. Personal data and pre-existing packages are
preserved; unchanged owned resources are removed/restored surgically.

Claude Code and Codex CLI are separate selections. Select either or both after
migration; their standalone binaries do not require Node/npm. Authentication,
sessions and personal CLI settings stay local. Claude's managed shell disables
its own updater; use the setup menu's Update action for reviewed CLI pins.

GitHub Copilot CLI is a separate selection on every platform. A Copilot you
installed with npm, `gh`, winget, Homebrew or its install script stays untouched;
the dotfiles copy takes precedence in configured shells. Remove the other copy with
its own tool if you no longer want it. Its cached updates under `~/.copilot/pkg`
belong to that copy. The managed shell sets `COPILOT_AUTO_UPDATE=false`; on
Windows, PowerShell 7 is added as Copilot's prerequisite.

Kiro CLI is a separate selection on macOS and Linux. It is not offered on Windows,
where Kiro ships it only as an MSI installer; install it yourself there if you want
it (Sentinel rules and skills also go to it). If
you installed Kiro CLI yourself, the dotfiles copy takes precedence in configured
shells and your copy is left alone; remove it with its own uninstaller if you no
longer want it. Setup sets only `app.disableAutoupdates` in
`${KIRO_HOME:-~/.kiro}/settings/cli.json` (an existing `false` is shown for explicit
adoption first) and restores your file on removal.

Pi has been removed, including its copied themes/keybindings and Sentinel target.
Existing released Pi packages and personal data remain outside this installer's
ownership. Keep an old checkout if a personal Pi configuration still links to it.
To test an earlier unreleased native-installer revision, first remove Pi and its
old Sentinel selection with that exact revision. Finish any pending operation
there before you advance. Migration retires the released Pi policy file
(see [Retiring the earlier installation](#retiring-the-earlier-installation)).
Never discard state or recovery journals.

Sentinel rules and skills replace the two v1.0.0 selections, Sentinel policy and
Sentinel skills. Read [Sentinel rules and skills](#sentinel-rules-and-skills)
before the first apply after the update.

The POSIX launchers create private cache/staging files under umask `077`, then
restore the caller's umask before starting the installer. They do not change your
shell's file-creation policy.
Both launchers isolate the compiler's module/build caches; personal Go settings
remain intact when the installer starts.

For historical v0.1.0-to-v0.4.4 procedures, read
[the immutable former guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/UPGRADING.md).
Those commands belong to their corresponding release, not this installer.

Desktop app additions (2026-10-13 candidate): select each independently through
setup. Linux Obsidian used a private Flathub stable installation until
2026-10-22 and now uses the official portable build, started without Chromium's
sandbox (disclosed before approval). If Obsidian was already selected, **Update
tools** installs the portable build and repoints your existing launcher; it
removes nothing, so the earlier Flatpak stays until you next apply **Choose
tools**, which removes it with your approval. Its `~/.var/app/md.obsidian.Obsidian`
data and private runtime are kept, and ordinary user/system Flatpak stores are
not managed. Obsidian then keeps its settings in `~/.config/obsidian` instead of
`~/.var/app/md.obsidian.Obsidian/config/obsidian`, so it asks once for your vault:
choose **Open folder as vault**; vaults themselves are untouched.
Linux OpenWhispr amd64 uses the official portable archive: its upstream launcher
may disable the Chromium sandbox on restricted-userns systems, disclosed before
approval. No Flatpak or Linux arm64 build is currently published. Zen Linux
updates through setup and uses a stable personal profile across package generations.
Tailscale uses normal package removal, preserving service configuration and state;
removing machine-wide owned packages requires the separate removal choice.

VS Code's supported `update.mode: none` preference keeps the application on the
reviewed installer version. Theme, font and update settings have one owner.
An earlier theme-only installation must first restore its saved settings through
**Choose tools**, then **Resume**. Restoration preserves the earlier file
separately, including unchanged copies. Modified old settings need the optional
restoration approval. If restored settings contain a personal theme, font or
update preference, Resume cannot add that new approval: choose **Abandon**, then
**Choose tools** and approve adoption. Removal restores those values and
permissions. Missing or damaged evidence blocks handover; preserve it and restore
the matching settings path and evidence before continuing.
A bundle that already self-updated stays protected and requires explicit
replacement approval; changing the preference does not claim changed bytes.
This application-wide preference can also propagate to other machines through
[Settings Sync](https://code.visualstudio.com/docs/configure/settings-sync).
Exclude `update.mode` from sync in Code if those machines should update separately.


## Sentinel rules and skills

Dotfiles v1.0.0 had two selections, Sentinel policy and Sentinel skills. It
copied the Sentinel 0.2.0 files itself. Now one selection, Sentinel rules and
skills, runs the Sentinel installer at a pinned commit. A saved selection of
either earlier choice becomes this selection.

Before you update the checkout, finish or abandon an interrupted Sentinel
operation with v1.0.0. Follow [Interruptions and conflicts](#interruptions-and-conflicts).

The first apply after the update does the handoff:

1. Setup deletes each skill folder that the v1.0.0 receipts list as a dotfiles
   copy. It deletes a folder only when a hash check against the saved journal
   proves that the folder holds the bytes that dotfiles copied.
2. Sentinel installs its own copies of the skills.
3. Sentinel replaces the old rules block in place. Text outside the block stays.
4. Setup releases the old receipts.

Setup does not delete a skill folder that changed or that a link now leads to.
It reports the folder, and Sentinel stops at that folder. Move the folder to a
different location, then apply again.

If v1.0.0 adopted an earlier rules block, the handoff drops that saved block.
Removal through dotfiles does not bring it back, and the text outside the block
stays.

If you untick Sentinel in the first run after the update, the old copies stay.
To remove them, select Sentinel one time so that the handoff runs. Then untick
it.

Sentinel puts skills for Codex, Copilot and OpenCode in `~/.agents/skills` and
Kiro files in `~/.kiro`. The handoff also deletes the v1.0.0 copies in
`$COPILOT_HOME/skills` and in a custom `KIRO_HOME`. With `COPILOT_HOME` set,
Copilot does not read `~/.agents/skills`. When `KIRO_HOME` names another folder,
setup shows a note. The old rules block in `$KIRO_HOME/steering/AGENTS.md`
stays. Remove it by hand if Kiro does not need it.

## T3 Code

T3 Code is a new optional selection, never an automatic migration or provider
replacement. It uses the official standalone package with a browser UI on Apple
Silicon macOS, Ubuntu/Debian amd64/arm64 and Windows amd64. The fixed stable
`v0.1.0` pin supplies Orchestrator V2. Finish any pending nightly installation
with its original checkout first; then stop T3, update the checkout and choose
**Update tools** to replace the managed nightly with stable. Personal
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

### Zen macOS native application

The macOS selection now uses Homebrew's official `zen` cask and normal browser
updates. Existing authentic app bundles are reused without acquiring removal
rights. Finish an interrupted earlier operation using its original checkout
before changing revisions; keep all journals and failed generations.

For an earlier private `tool.zen` installation, use **Choose tools** and keep Zen
selected. Review the new native app and the old launcher/package cleanup together.
The old launcher is removed before an unchanged private bundle. A bundle that
already self-updated remains protected; its original ownership checks are not
relaxed. The old archive pins and preparation revisions remain recovery-only.
A retained `DotfilesZen.app` still has Zen's bundle ID and may appear in macOS
application routing. Inspect that retained copy before choosing a default browser.

The new installation does not edit profiles, `profiles.ini`, `installs.ini`,
preferences or default-browser registration. Zen handles its own profile setup;
a browser version older than an existing profile can display a downgrade warning.
Quit all Zen copies before changing the native installation. Remove preserves
personal profiles and caches and requires per-item approval. It verifies the
recorded app's signer and bundle ID, but a manual reinstall from the same developer
at the same path is indistinguishable from a normal browser update. An unverified
app, changed uninstall recipe or an existing shared `zen` command blocks removal.

### Windows Python preparation correction

An earlier unreleased installer can stop while preparing Python bytecode under
Windows' default path policy. Keep its transaction and original checkout. Use
that original installer to review the recovery choices before changing revisions;
Restore or Abandon is available only when the saved evidence permits it. Do not
delete the failed generation or journal to bypass recovery.

Once recovery is complete, update the checkout and run setup again. The corrected
preparation uses extended paths without changing Windows policy or moving package
directories. Existing completed LaTeX and yamllint environments receive an
ordinary approved Update for the new preparation revision. Personal Python
environments and pip settings are outside this change.

### Windows Neovim formatter correction

Windows now uses standalone Gersemi and clang-format archives at the same pinned
versions as the POSIX Mason tools. An approved Update builds a fresh Neovim
runtime and installs those automatic prerequisites; no new checkbox is needed.

If Neovim synchronization failed before publishing its runtime, Abandon preserves
the partial generation and journal so setup can start a fresh operation with the
corrected checkout. A previously abandoned operation no longer traps Neovim in
Pending. If publication itself was interrupted, recover that exact operation
with its original checkout before abandoning or changing source. Never delete
the journal or partial generation to bypass recovery. Abandonment uses the
existing provider finalization order: it may finish cleanup of independently
verified obsolete archive generations before refusing an interrupted Neovim
publication. That refusal preserves the Neovim publication and recovery evidence.
If saved evidence is damaged, Check names its location and keeps unrelated tools
available. An active operation with unreadable publication evidence cannot be
abandoned safely; preserve it for recovery rather than deleting its journal.

### Windows VC++ registry reconciliation

Build Tools installation can leave differing VC++ version entries across the
Windows registry views. Setup now accepts the newer entry only when the installed
bundle, both MSI providers and the runtime DLL versions agree. Older entries are
preserved and disclosed, and block runtime removal. Other conflicts still fail.
Completed receipts from the earlier adapter remain readable. Finish an interrupted
old VC++ operation with its original checkout before updating; its saved command
cannot be reinterpreted as the new recipe.


### Windows legacy profile ownership

An earlier elevated setup can leave Git configuration or a PowerShell profile
owned by `BUILTIN\Administrators`. Windows may let your normal account edit its
contents but refuse to assign that owner to a replacement file. Setup preserves
the original file and its permissions, removes its refused staged copy, and
retains the saved operation. Repeating the same non-elevated attempt cannot fix
that refusal.

Open PowerShell as administrator **under the same Windows account**, return to
the same checkout, and run `./setup.ps1`. Choose Resume (or finish the saved
restoration if restoration was already started). Review and approve only the
one-resource recovery plan. After that resource recovers, leave the elevated
terminal and run setup from a normal PowerShell window to review remaining work.
Do not approve an entire new installation as administrator for this exception.

Setup never takes ownership or elevates profile writes automatically. You can
instead explicitly resolve the file's ownership outside setup if that is your
intended policy. Preserving an administrator-owned file may require the same
authority for later changes, removal or restoration of that file; ordinary Check
and unrelated tool selections do not require elevation.
This elevated route applies to `BUILTIN\Administrators`. For another owner, resolve
ownership explicitly outside setup; administrator elevation alone is not a
general way to preserve an arbitrary owner SID.

### Private Python tools and existing terminal sessions

The managed `latex2text` and `yamllint` commands now use their own command-only
directories. Their private Python/pip executables no longer enter your shell PATH.
Update replaces these two tool generations and the shell integration; the selected
base Python stays unchanged. Existing saved operations retain their approved recipe.
After Update, open a fresh terminal from the desktop and restart applications that
launch shells so they inherit the new command paths.

Windows private Python preparation also disables pip configuration
using Python's exact null-device spelling. This prevents machine or environment
configuration from changing the pinned installation. For interrupted operations,
finish recovery with the original checkout before updating.

Windows retirement can remove multiple released PATH entries in one approved
plan while preserving unrelated entries. If an older interrupted retirement
reports that a later tool changed after approval, abandon that remaining plan
through migration recovery, then review a fresh retirement plan. Completed
removals and their preserved evidence remain recorded.

Personal shell edits may use different line endings from the managed
block without triggering a repair. Existing UTF-16 profiles retain their encoding;
saved marked blocks remain readable for interrupted recovery even though the
journal fragment omits the full file's BOM.

### Continuing removal after restoring personal settings

Restoring an adopted settings file completes removal of that integration even
when the personal file remains present. A subsequent removal preview releases
its former prerequisites when the saved completion and restored baseline still
match. Personal files remain in place. A new edit or unavailable recovery evidence
keeps the affected dependencies protected without erasing the earlier completion
record; restore the evidence and review a fresh plan before continuing.
For an intentional settings edit, explicitly select and adopt that configuration,
then remove it again to establish the edited settings as the new personal baseline.
Reselecting without adoption preserves the earlier removal record; any unresolved
adoption or evidence requirement still appears as pending work.

Finish an interrupted operation with its original checkout before changing source.
When no resource remains in flight, public Abandon can archive the finished part
and preserve its receipts, followed by the corrected checkout and a fresh removal
review. Never edit state or delete recovery journals to bypass this boundary.
Older unreleased native builds could replace the completed-removal status with
retained status. Those records remain conservative: explicitly select and adopt
the affected configuration, then remove it again to establish a new verified
lifecycle. The released chezmoi/Nix migration uses its separate handover workflow.

### Earlier unreleased Mac bootstrap and application images

An earlier build of this unreleased PR recorded fresh Homebrew ownership using a
device number that macOS changes at reboot. Its schema-1 evidence cannot safely
establish the new volume identity. Keep that evidence and the working Homebrew,
CLT and packages; no public recovery command can rebind it. Verify the corrected
fresh bootstrap in a clean environment. This does not affect the released Nix
installer or an existing Homebrew reused without a new ownership record.

Update replaces managed DMG application generations using the corrected image
transport, which avoids adding AppleDouble files that invalidate vendor signatures.
Finish interrupted operations with their original checkout first. Personal app
data stays outside the replacement; do not remove quarantine or disable Gatekeeper.

An earlier native-Zen candidate could install the official app but reject its
Finder metadata, or lose install attribution when `HOMEBREW_NO_EMOJI` was set.
Preserve the failed app, native receipt and installer journal. If installation
has finished and the command record proves Zen was absent beforehand, quit Zen,
remove only that newly installed cask through normal Homebrew uninstall without
force/zap/autoremove, and verify the original package inventory and personal data.
Then public Abandon can archive the interrupted plan before a corrected fresh
preview. Do not use this procedure for a pre-existing or uncertain app. Corrected
builds preserve Finder metadata and quarantine, verify the vendor signature and
sealed resources, and bind attribution output settings into the new cask intent.

The same inherited `HOMEBREW_NO_EMOJI` preference also hides formula installation
markers. New formula and cask operations explicitly retain those markers; earlier
schema-3 operations keep their original command environment for inspection and
recovery. Never rewrite an interrupted operation to grant it new attribution.

If you installed Zen yourself through Homebrew and then removed only its app,
restore it with
`HOMEBREW_NO_AUTOREMOVE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew reinstall --cask zen`,
or remove the stale record with
`HOMEBREW_NO_AUTOREMOVE=1 brew uninstall --cask zen` before selecting it in setup.
These environment settings prevent unrelated dependency removal and, for
reinstall, automatic package/cache cleanup. Setup does not adopt your existing
native app or its package record.
