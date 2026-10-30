# Upgrading

## From v0.4.x

Release v1.0.1 is the last release that has `migrate.sh` and `migrate.ps1`.
They move a v0.4.x installation (Nix and chezmoi) to this installer. Later
revisions do not have them. Use two steps:

1. Preserve your local changes. Check out the `v1.0.1` tag and run its migrate
   entrypoint. Finish all phases, also the runtime retirement and the final
   Check. The [v1.0.1 upgrade guide](https://github.com/luisgui1757/dotfiles/blob/v1.0.1/docs/UPGRADING.md)
   describes each phase.
2. Update the checkout to the new revision. Open setup and choose
   **Update tools**.

```bash
git checkout v1.0.1
./migrate.sh
git checkout main
./setup.sh
```

On Windows, use `.\migrate.ps1` and `.\setup.ps1`.

Setup reports the marks of an earlier installation that migrate did not retire.
It does not use a Homebrew prefix that nix-homebrew still manages. Migrate keeps
its records and recovery copies in the installer state folder. Use the v1.0.1
checkout to examine them.

## Windows

The new Windows installer discovers actual known folders independently, including
redirected Documents and Start Menu Programs. Neovim uses a directory junction;
other configuration uses copies/scoped settings. Developer Mode is not required.
Windows Terminal manages only its pinned stable unpackaged instance. Existing
Store, Preview and Canary installations remain personal. If that instance still
contains ambiguous legacy combined actions, launch it once to let Terminal migrate
its own settings, then retry the reviewed setup.

## Git configuration

Git defaults now use a marked include in a regular personal XDG Git file.
`pull.rebase = true` and `fetch.prune = true` are defaults; personal values later
in that file or in `~/.gitconfig` override them. The include follows the current
checkout, so edits to its defaults take effect without an installer update.

For a released chezmoi Git symlink, v1.0.1 migrate preserves the original link and
copies every readable setting into a regular file. This includes personal name,
email and other settings that the former `git config --global` write-through
behavior might have added to the source. It does not rewrite that old checkout.
Personal sync links outside the recognized dotfiles layouts stay unchanged.
Copied personal values still override the new defaults; the released default file
has no pull policy, so its migration allows the new rebase default to take effect.
Unreadable links and non-UTF-8 Git files require explicit repair or conversion.

Git follows `HOME`/`XDG_CONFIG_HOME` on Windows as well as POSIX. An explicit
`GIT_CONFIG_GLOBAL` bypasses both personal global files; unset it before applying
these defaults. If the discovered path changes between shells, setup keeps
removal attached to the originally approved file and reports the changed path.
Remove that integration before selecting Git at the new location.
Git targets that resolve inside the checkout are blocked, including redirected
parent directories. Historical targets that move or go offline cannot block
independent selections; any uninspectable old lock evidence remains disclosed.

## Interruptions and conflicts

An unfinished transaction from a released old installer must first be recovered
with that exact retained release and recovery directory. Recover an unfinished
migrate operation with the v1.0.1 checkout.

For a new installer interruption, finish recovery **before updating the checkout**.
Resume requires the original installer executable and its matching checkout,
including the original configuration sources. If neither has changed, reopen the
same `setup` entrypoint and use its recovery menu. Restoration uses
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
binding its original checkout. For a migrate transaction, use the v1.0.1
executable and checkout, and `migrate` instead of `setup`:

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

Claude Code and Codex CLI are separate selections. Select either or both;
their standalone binaries do not require Node/npm. Authentication,
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
v1.0.1 migrate retires the released Pi policy file.

The POSIX launchers create private cache/staging files under umask `077`, then
restore the caller's umask before starting the installer. They do not change your
shell's file-creation policy.
Both launchers isolate the compiler's module/build caches; personal Go settings
remain intact when the installer starts.

For historical v0.1.0-to-v0.4.4 procedures, read
[the immutable former guide](https://github.com/luisgui1757/dotfiles/blob/ae9a6446eb0a837a144d77d2a6345db967c61e4e/docs/UPGRADING.md).
Those commands belong to their corresponding release, not this installer.

Desktop apps: select each independently through setup. Linux Obsidian uses the
official portable build, started without Chromium's sandbox (disclosed before
approval). Linux OpenWhispr amd64 uses the official portable archive: its upstream launcher
may disable the Chromium sandbox on restricted-userns systems, disclosed before
approval. No Flatpak or Linux arm64 build is currently published. Zen Linux
updates through setup and uses a stable personal profile across package generations.
Tailscale uses normal package removal, preserving service configuration and state;
removing machine-wide owned packages requires the separate removal choice.

VS Code's supported `update.mode: none` preference keeps the application on the
reviewed installer version. Theme, font and update settings have one owner.
A bundle that already self-updated stays protected and requires explicit
replacement approval; changing the preference does not claim changed bytes.
This application-wide preference can also propagate to other machines through
[Settings Sync](https://code.visualstudio.com/docs/configure/settings-sync).
Exclude `update.mode` from sync in Code if those machines should update separately.


## T3 Code

T3 Code is a new optional selection, never an automatic migration or provider
replacement. It uses the official standalone package with a browser UI on Apple
Silicon macOS, Ubuntu/Debian amd64/arm64 and Windows amd64. The fixed stable
`v0.1.0` pin supplies Orchestrator V2. Personal T3 state stays outside the
package; back it up before running a newer server.

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

## Windows legacy profile ownership

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

## Continuing removal after restoring personal settings

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

## A personal Homebrew Zen without its app

If you installed Zen yourself through Homebrew and then removed only its app,
restore it with
`HOMEBREW_NO_AUTOREMOVE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew reinstall --cask zen`,
or remove the stale record with
`HOMEBREW_NO_AUTOREMOVE=1 brew uninstall --cask zen` before selecting it in setup.
These environment settings prevent unrelated dependency removal and, for
reinstall, automatic package/cache cleanup. Setup does not adopt your existing
native app or its package record.
