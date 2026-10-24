# Field findings: real machines versus CI

Problems found while migrating the owner's own machines to the native installer
(PR #87, October 2026). Hosted CI passed for almost all of them; they surfaced
only on real hardware, under real corporate policy, or in the hands of a person
following instructions. Read this before a full-scope review: each entry names
what to recheck so a fix is not silently undone.

The append-only, dated detail lives in
[the review ledger](2026-10-08-interactive-installer-overhaul.md); durable rules
live in [CLAUDE.md](../../CLAUDE.md). This page is the curated index. Update an
entry's status when it changes; add new entries rather than rewriting history.

Each entry: **symptom** (what the user saw), **cause**, **resolution**, **status**,
and **recheck** (what a reviewer or a future change must keep).

## The machines

| Machine | Relevant traits |
|---|---|
| Corporate Windows laptop | Documents redirected to OneDrive; winget has no source (IT removed it); Scoop, winget and Chocolatey history from the released installer; Adobe Creative Cloud running; ordinary user with UAC prompts allowed. |
| Corporate Ubuntu 24.04 laptop (amd64) | X11 session; APT mirrors reachable only on the company VPN; IT tooling switched 89 AppArmor profiles to `complain` on 2026-10-15; user namespace restriction on (Ubuntu default); shared Mac-layout Logitech keyboard. |
| Personal Mac (Apple Silicon) | Released Nix/nix-darwin/Home Manager setup to retire; Homebrew; Ghostty hosting the agent session. |

## Windows

1. **Migrate stopped on Windows Terminal.**
   *Symptom:* "integration.windows-terminal: pre-existing, uncertain or changed
   resource requires explicit adoption", on every run. *Cause:* the existing
   settings used legacy combined action/key bindings, which only Windows Terminal
   itself can migrate. *Resolution:* the user renamed `settings.json` and launched
   the dotfiles Windows Terminal once, so the app rewrote it, then migrate adopted
   it. Colors looked "busted" until the merge reapplied the theme. *Status:* done.
   *Recheck:* the adoption guard and the "ambiguous legacy overlaps require the
   native application" rule (CLAUDE.md).
2. **"Access is denied" removing a workspace in OneDrive Documents.**
   *Cause:* OneDrive marks synced folders ReadOnly and Go's `os.Remove` clears
   that attribute only for files. *Resolution:* `removeOwned` /
   `removeEmptyDirectory` clear it on the installer's own empty directories, retry
   once and restore it otherwise. *Status:* fixed (62bc427 era). *Recheck:* never
   clear it on links, occupied folders or unrelated paths.
3. **Wrong support advice: deleting `.dotfiles-config-*` folders.** The agent
   said they could be deleted; the user did, and migrate then refused ("configuration
   artifact changed") because those folders are recovery evidence. The user
   restored them from the Recycle Bin. *Recheck:* support instructions must never
   tell a user to delete preserved or workspace paths; they are evidence.
4. **No winget source.** *Symptom:* `0x8A150012` (SOURCE_NAME_DOES_NOT_EXIST) on
   every winget inventory read. *Cause:* IT removed winget's source. Nothing can
   prove winget ownership on that machine. *Resolution:* owner-approved "keep these
   leftovers and finish" in interactive migrate, with paths listed so the user can
   remove them by hand. *Status:* done. *Recheck:* interrupted, unreadable,
   uninspectable and blocked work must never be keepable.
5. **One URL-installed Scoop app blocked every Scoop retirement.** *Symptom:*
   "is still required by Scoop apps" with an empty app name. *Cause:* the
   dependency check read every installed app's full install record; `helm`,
   installed from a URL, has no bucket or version. *Resolution:* read only each
   app's manifest dependencies. *Status:* fixed and confirmed on the laptop (nine
   Scoop tools retired).
6. **Adobe's `node.exe` blocked removing Scoop's Node.js.** *Symptom:* step 10/20
   failed: "found 8148 C:\Program Files\Adobe\...\node.exe, approved """.
   *Cause:* the running-process gate matched the process name anywhere; it would
   also have blocked Scoop's PowerShell 7 whenever any `pwsh` ran (including the
   shell running migrate) and Python whenever any `python.exe` ran. *Resolution:*
   with a known installation directory (Scoop), only processes running from it
   block, as in Scoop's own test; names still apply to winget/Chocolatey gates and
   to processes whose image cannot be read. *Status:* fixed (fda470d), Windows
   suites green. *Recheck:* keep the native "same-named program elsewhere"
   regression.
7. **Walls of `#< CLIXML` XML in the live output.** *Cause:* Windows PowerShell
   children serialize progress/information records on stderr. *Resolution:* the
   live display hides the marker and `<Objs>` documents (also when split by the
   4096-byte wrap); logs and evidence keep every byte. Fixed scripts were not
   edited because their bytes are saved command identities. *Status:* fixed.
8. **"Inspecting your system" took many minutes.** *Cause:* not determined. The
   user was asked for a process-tree snapshot but the run finished first.
   Suspects: endpoint antivirus scanning every file read, and OneDrive placeholders.
   *Status:* open, unmeasured. *Recheck:* time a Windows preview on a managed
   laptop; consider a progress line during inspection.
9. **Sentinel skills already installed in `~/.kiro` by Sentinel's own installer.**
   *Symptom:* "Sentinel upstream ownership or recovery evidence exists at
   ...\.kiro\skills\.sentinel-skills.json". *Cause:* expected refusal; the new
   Kiro skills root met an existing upstream install. *Resolution:* remove with
   upstream `node tools/skills.mjs --host kiro --global --remove` (refuses edited
   folders, leaves an empty registry that setup accepts). *Status:* given to the
   user. *Recheck:* never adopt another installer's skills.
10. **Kiro CLI on Windows: first hosted run (60a3148) failed.** (a) The agent
    lifecycle wrote Kiro's settings switch into the runner's real profile: on
    Windows the installer resolves home through known folders, not the fixture's
    HOME; the fixture now passes `KIRO_HOME`. (b) The non-elevated MSI unpack hung
    until the 15-minute step timeout; the restricted child had no timeout of its own
    (a test binary run directly has none) and printed nothing. Changed: msiexec is
    awaited directly with a 10-minute bound instead of `Start-Process -Wait` (which
    also waits for descendants), and the restricted child gets `-test.timeout` and
    file output. *Status:* root cause of (b) unproven; waiting for the next hosted
    run. (c) The public migration fixture then refused the leftover state (cascade).
    The next run (1dca9af) proved (a) fixed and showed (b) precisely: as an
    ordinary user msiexec never created its log or any file in 10 minutes, so the
    unpack moved off Windows Installer (preparation revision 1): a stdlib Go read
    of the MSI's one embedded cabinet, which reproduced the pinned MSI's cabinet
    byte for byte, then the system `expand.exe`. Why msiexec blocked for the
    restricted token is not established. At cec98fb the cabinet copy was exact,
    then expand.exe could not open a path containing `ü`: it uses the system code
    page, so a profile like C:\Users\Müller would fail too. With plain relative
    names it then failed both ways: as an administrator it left the copied
    `.prepare-kiro.cab` instead of `KiroCliExe`, and as the restricted user it could
    not start in the stage ("The directory name is invalid"). Preparation revision 2
    removes expand.exe: setup decodes the cabinet's LZX data in Go
    (`cabinet_lzx.go`) and checks the pinned size and SHA-256, which libarchive
    reproduces independently. The real cabinet decodes locally in about 2 s.
    *Status:* superseded the same day: the owner withdrew Kiro CLI from Windows
    rather than rely on unpacking a vendor installer, so the MSI/cabinet code and
    its hosted step are removed. Kiro ships Windows only as an MSI; a plain archive
    from Kiro would let it return like the macOS/Linux pins. *Recheck:* if Windows
    Kiro returns, never route it through msiexec or another Windows unpacker
    without a native ordinary-user pass; never pass non-ASCII paths to code-page
    programs.
11. **Windows race suite exceeded its time limit, twice.** *Cause:* five new
    Sentinel skills tests added ~230 s on Windows (file-heavy tests run ~3.4x
    slower there than on a Mac; the suite already used 1078 of 1200 s). *Resolution:*
    read-only previews for presence variants, one lifecycle per distinct root, and
    only the roots under test; local time 67 s to 13 s. It timed out again at
    1dca9af with no single regression: the same tests ran 16% slower on that
    runner (962 s at 67aae3b, 1057 s at 60a3148). The owner raised the full race
    suite to 30 minutes. *Recheck:* new file-heavy tests still need measuring; the
    limit catches hangs, it is not room to fill.

## Ubuntu

1. **Apply stopped off the VPN, then "needs your action" after resuming.**
   *Cause:* the company APT mirrors resolve only on the VPN; Resume recovered the
   one interrupted step and, by design, asked for a fresh review of the remaining
   work. *Status:* working as designed, but the wording confused the user.
   *Recheck:* consider clearer wording for "recovered, now review the rest".
2. **Ghostty did nothing when clicked.** *Symptom:* `ghostty-bin: symbol lookup
   error: undefined symbol: gtk_css_section_get_bytes`. *Cause:* the pinned DEB
   was built for Debian 13 (GTK 4.16); Ubuntu 24.04 has older GTK. *Resolution:*
   pin upstream's 24.04 build; `TestNativeGhosttyLoadsOnOldestSupportedUbuntu`
   loads it in a digest-pinned 24.04 image and requires the old build to fail.
   *Recheck:* GTK/libadwaita are forward-compatible only; pin the oldest supported
   release's build.
3. **Generic dock icon and a second Zen window.** *Cause:* private launchers had
   no `StartupWMClass`. *Resolution:* declare Ghostty's and Zen's real window
   classes; CI requires a mapped window with that class.
4. **Pasted commands failed in zsh.** *Cause:* interactive comments are off, so a
   trailing `# comment` became arguments. *Recheck:* support commands must be
   comment-free and must not use `&&` for Windows PowerShell 5.1.
5. **Unticking Mac keyboard shortcuts kept its udev rule.** *Cause:* catalog
   `machine` scope means "possibly shared, keep on deselection". *Resolution:* the
   keyboard-access grant has user scope with a privileged observation, so it goes
   with the tool. Found by the hosted lifecycle.
6. **Black screen after the first login with the remapper.** A reboot fixed it;
   cause unknown (no logs collected). *Status:* open, not reproduced.
7. **`Option+2` switched Zen tabs instead of workspaces after `git pull`.**
   *Cause:* xremap's `--watch=config` watches the config file and its own
   directory; the config is a live link into the checkout, where git replaces the
   file (unlink, create) in a directory xremap does not watch, so the keymap never
   reloaded and `Option+2` still arrived as `Alt+2` (Zen's tab key). *Resolution:*
   the login entry resolves the link before exec, plus a 500 ms reload debounce;
   the native test replaces the file as git does and requires a reload.
   *Status:* fixed in 1dca9af; the native reload proof passed on amd64 and arm64.
   Existing installs need Update and one login.
8. **GNOME gives `Super+1...9` to the dock.** Mirroring AeroSpace workspaces needs
   the user's one-time, reversible gsettings commands (README). Setup never writes
   GNOME settings.
9. **Obsidian did nothing when clicked.** (a) *Symptom:* `bwrap: setting up uid
   map: Permission denied` / `ldconfig failed`. *Cause:* IT tooling set 89 AppArmor
   profiles to `complain` on 2026-10-15, including `/etc/apparmor.d/flatpak`
   (stock: `flags=(unconfined)` with `userns`), so Flatpak's bubblewrap lost its
   user-namespace exception; `dpkg -V` showed the files modified. The user restored
   that one profile by hand; IT may flip it back. (b) Then: `error:
   app/md.obsidian.Obsidian/x86_64/stable (commit ba4cac4f...) not installed`.
   *Cause (our bug):* Electron inside Flatpak (zypak) starts helpers through the
   Flatpak portal, which only knows standard installations, not setup's private
   store. `--no-sandbox` opened the window. *Resolution (owner choice):* Linux
   Obsidian moves to its official portable build (`tool.obsidian-portable`,
   1.14.4 tar.gz) launched with `--no-sandbox`, disclosed before approval,
   independent of AppArmor and the portal. The Flatpak binding stays only to see
   and remove earlier installs: Update repoints the existing launcher, and a
   Choose tools apply removes the old private Flatpak with approval, keeping
   `~/.var/app` data. Obsidian's settings move to `~/.config/obsidian`, so the
   vault is opened once. *Status:* proven on amd64 and arm64 at cec98fb. *Recheck:*
   hosted Ubuntu images relax the userns restriction and have no session portal;
   the Linux desktop lifecycle now restores Ubuntu's stock restriction on the
   disposable runner (test-host precondition only).
10. **Zen back/forward on Linux.** The user tried `Alt+←` (Zen's Linux label);
    with the remapper, `Cmd` is Ctrl and `Option+←` is word-left, so neither sends
    plain `Alt+←`. Back/forward is `Cmd+[` / `Cmd+]`. Mapping `Cmd+←/→` to
    back/forward in browsers was offered and declined (2026-10-22): the remapper
    cannot tell a page from a text field, so it would also navigate while typing.
11. **Support commands broke on the `ls` alias.** `ls` is `lsd` in dotfiles
    shells, which rejects GNU flags such as `--time-style`. *Recheck:* use `stat`
    or `command ls` in instructions.
12. **Live Obsidian store listed as "Preserved files for inspection".** Cosmetic:
    the Flatpak observation always reports its store; kept receipts never refresh.
    Deliberately not changed. Moot now that Linux Obsidian left Flatpak.
13. **OpenWhispr showed a generic gear icon in the dock** (2026-10-23). *Cause:*
    the same as Ubuntu 3, missed for OpenWhispr. Its main process calls
    `app.setDesktopName("open-whispr.desktop")`, and Electron 41 derives both parts
    of WM_CLASS from that name, so the window class is `open-whispr` while the
    launcher is `dotfiles-openwhispr.desktop` with no `StartupWMClass`. The unit
    test had recorded "no class" for OpenWhispr instead of checking it.
    *Resolution:* the launcher declares `StartupWMClass=open-whispr`; the unit
    test fails on the previous recipe, and the hosted Linux desktop lifecycle now
    requires an OpenWhispr window with that class. Existing installs need Update,
    then a restart of OpenWhispr. *Recheck:* every new private launcher declares
    its application's real window class.
14. **Ghostty and Herdr opened bash, not zsh** (2026-10-23). *Cause:* the laptop
    account is a company directory account (`getent passwd` gives `/bin/bash`;
    no line in `/etc/passwd`), so `chsh` cannot change it and the released
    installer never could. Ghostty starts the login shell, and Herdr, tmux and
    Neovim terminals start `$SHELL`. A `.bashrc` that only runs `exec zsh` would
    leave `SHELL=/bin/bash`, so every Herdr pane would still be bash.
    *Resolution (owner choice):* the optional **zsh from bash terminals**
    capability hands interactive terminal bash to zsh with `SHELL` exported.
    Testing found that macOS bash 3.2 exits when `exec` fails during startup, so
    the handoff keeps `execfail` set around it. *Recheck:* a broken zsh must leave
    a working bash; the real login shell is never changed.
15. **`/etc/hosts` opened in a non-Neovim editor** (2026-10-24). *Cause:* setup
    never set `EDITOR`, so sudoedit used the system default. *Resolution (owner
    request):* selected Neovim exports `EDITOR`/`VISUAL=nvim` and `SUDO_EDITOR`
    as the managed nvim's full path. Disposable Ubuntu containers showed that
    sudoedit ignores PATH (sudo 1.9.15 `secure_path`, sudo-rs 0.2.13 on 26.04),
    so a bare `nvim` there gives "command not found"; that a path with a space
    breaks sudo-rs even when escaped; and that sudo-rs refuses any file below a
    user-writable directory such as `/tmp`. *Recheck:* `sudo -e /etc/hosts`
    opens your configured nvim; macOS has no `sudoedit` command.
16. **Setup stopped on an unrelated APT source with an expired key** (2026-10-24).
    *Symptom:* resuming an interrupted clangd install, the package-list refresh
    failed with `EXPKEYSIG B8AC39B0876D807E` for the OneDrive client's openSUSE
    repository, and the failure report printed APT's output twice more after the
    live copy. *Cause:* the repository's key had been extended upstream (now
    expiring 2028-12-18), but the laptop still had the expired copy. The installer
    refreshes with `apt-get -o APT::Update::Error-Mode=any update`. In a disposable
    Ubuntu 24.04 container, plain `apt-get update` only warned (exit 0) about a
    signature failure on a source with earlier verified lists, while that mode
    exited 100. The repeat came from the APT driver appending the output to a
    worker error that already ended with it. *Resolution:* the owner refreshed the
    key; the report shows the output once and says to fix or disable the failing
    source, then resume. By owner decision the refresh is now plain `apt-get
    update`, so such a warning no longer stops setup; saved strict refresh
    commands stay valid and re-run unchanged.
    `TestLinuxAPTFailedRefreshExplainsItsSourceOnceAndResumes` and
    `TestLinuxAPTRefreshIsPlainAndSavedStrictRefreshStaysValid` fail on the
    previous code. *Recheck:* a refresh error (not a warning) still stops setup.
17. **Saving `.zshrc` in Neovim stopped setup** (2026-10-24). *Symptom:* every
    tool waited on `integration.shells: profile block markers were edited`,
    without naming a file. *Cause:* `~/.zshrc` was created by setup, so its block
    opened the file with an empty first line. The owner added a line in Neovim,
    and saving ran shfmt (Conform formats zsh on save), which drops a file's
    leading blank lines; the managed shfmt v3.14.1 does so with or without the zsh
    dialect. The parser needed a newline before the start marker. *Resolution:*
    the owner restored the empty line; a block whose start marker opens the file
    (after any BOM) is now recognized as the block that was written, and the error
    names the file. `TestProfileBlockOpeningTheFileSurvivesFormatterAndLifecycle`
    fails on the previous parser. *Recheck:* setup-created `.zshenv`, `.zprofile`
    and `.zshrc` start with the block; text before a marker is still refused.

## macOS

1. **Retirement blocked by running processes.** Nix removal stopped while the old
   Herdr ran lazygit and an old codex node; the user closed Herdr. *Recheck:* the
   kernel consumer scan is never bypassed.
2. **The tmux session hosting the work died twice** (after the Nix uninstall and
   after Herdr retirement). Cause unknown; no crash logs. Each item's state was
   persisted and resumed.
3. **Old Ghostty still hosts the session.** Retiring `/Applications/Ghostty.app`
   waits until the user switches to `~/Applications/Ghostty.app`; the empty `/nix`
   disappears after a restart. The gate matches Ghostty's bundle ID, so the new
   copy blocks it too: migrate has to run from Terminal.app. *Status:* done; the
   old Ghostty was retired on 2026-10-22, migration completed on 2026-10-23, and
   after the restart `/nix` is gone with no Nix volume, mount, build users,
   services or commands left.
4. **Sentinel upstream registry and old `npx skills` installs** blocked dotfiles
   skills; an empty upstream registry is now accepted. Kiro is targeted only when
   present.
5. **Migrate stopped at Nix twice with "a password is required".** The
   retirement's read-only check of root-only files (`/etc/fstab`) uses existing
   `sudo -n` credentials and never prompts. sudo's ticket lasts five minutes and
   belongs to the terminal where it was typed; the system log showed the last
   `sudo` an hour before the run. Running `sudo -v && ./migrate.sh` on one line
   finished it. *Status:* working as designed; the message names `sudo -v`.
   *Recheck:* consider having interactive migrate refresh sudo in the foreground
   before a preview that needs root-only evidence.

## Hosted CI blind spots and flakes

- **No real desktop session:** no session D-Bus/portal on Xvfb, and GitHub's Ubuntu
  images relax `kernel.apparmor_restrict_unprivileged_userns`. Flatpak/Electron
  sandbox problems (Ubuntu 9) are invisible unless a test recreates them.
- **No corporate policy:** no OneDrive, no endpoint antivirus, no missing winget
  source, no AppArmor complain-mode tooling.
- **GLib desktop entries:** GDesktopAppInfo checks the Exec program before it
  expands `%%`, so a launcher whose program path contains `%` cannot start
  through GIO; Chromium rewrites `/proc/<pid>/cmdline` as one space-separated
  title, so process-argument checks must split on spaces too.
- **Administrator runners:** ordinary-user behaviour needs the restricted-token
  fixtures; their child must have its own timeout and file output (Windows 10b).
- **Fresh Windows vendor: first inspection at the 30-second bound.** VC++
  inspection exit 1 with no diagnostic at ~30 s (91b8257; passed on 67aae3b), then
  Build Tools inspection "context deadline exceeded" at 30.07 s (366f7d9) and
  30.15 s (e9f728b) in `TestWindowsBuildToolsNativeOwnedLifecycle`. Both Build Tools
  failures ran on runner image windows-2022 20261021 (host build 20348.5761); every
  pass, including bab9926 between them, ran on 20261012 (build 20348.5622), until
  49076d1 passed on 20261021, so the new image makes it more likely, not certain.
  The pinned test container is build 20348.4171. WMI is ruled out: a separate
  container timed after the lifecycle ran its first WMI query in 584 ms (593 ms on
  20261012) and its whole PowerShell probe in about 5 s. Current hypothesis: the
  first container started after the image pull is slow; in the Build-Tools job the
  lifecycle was that container, while VC-runtime first runs its Python check. The
  timing step now runs right after the pull, measuring that first container and
  leaving the lifecycle in a later one. At 451a525 that first PowerShell container
  took 31.9 s (20261012) and 34.7 s (20261021) against 8.5 to 9.8 s for a second
  one, while its own script ran in 5 to 7 s; both vendor jobs then passed. The same
  run showed the limit failing outside containers: core and Neovim lifecycle on
  windows-2025 20261021, where Visual Studio is installed, exceeded 30 s on the
  first Build Tools inspection (the whole three-inspection test took 28.7 s on
  20261007 and 40.2 s on 20261021), and the installer itself stopped with
  "inspect C/C++ toolchain". *Resolution (owner choice):* the Build Tools
  inspection bound is 120 s; VC++ inspection stays at 30 s. *Recheck:* a cold
  first inspection on a slow Windows machine is the case that limit protects.
- **Flakes seen once and not reproduced:** Neovim Mason `json-lsp: failed` with no npm detail on macOS core
  (fda470d; passed at 60a3148); Neovim `plugin differs from lock: nvim-treesitter`
  on macOS (cec98fb); a Codex helper joining the private job during reaping in the
  Windows agent lifecycle (cec98fb; passed at 1dca9af); macOS runner-capacity
  cancellations.
- **Linux Nix scanner race (fixed).** `TestLegacyLinuxReaderWorkerExecStress`
  reported no consumer for a process holding a store file, on Ubuntu ARM (67e7a41)
  and again on Ubuntu amd64 (451a525). Reproduced in a disposable arm64 Linux 6.12
  container (about 1 in 100 runs of 3000 scans) and instrumented: after a read
  failed during a worker thread's exec, the pidfd polled ready, so the process
  counted as gone. A C probe measured it directly: 14 of about 6 million fresh
  pidfds polled ready while the process stayed alive (State R right afterwards; a
  re-poll was not ready). *Resolution:* readiness counts only if a fresh
  pidfd_open of the PID then finds no thread-group leader; 400 stress runs passed,
  and `TestLegacyLinuxReaderTransientPidfdReadinessIsNotExit` fails on the previous
  code. *Recheck:* this scan guards Nix store deletion; never accept a single
  kernel observation as proof that a process is gone.
- **Runner image drift:** GitHub's macOS 26 image 20261021 replaced OpenSSL 3.6.5
  with 3.5.9. The released-setup fixture installs python@3.12 from Homebrew's
  2026-07-26 snapshot, which brings `openssl@3` 3.6.3; Homebrew always upgrades an
  outdated dependency, so the exact pre-existing-package guard failed
  (`TestNativeLegacyRetirementDarwinReleasedSetup`, bab9926). *Resolution (owner
  choice):* the guard keeps ownership evidence exact and accepts only an added,
  linked keg beside the original. *Recheck:* any other change to a pre-existing
  package still fails.
- **Bot PRs:** pushes or PRs made with `GITHUB_TOKEN` do not start the required
  checks, and this repository disallows Actions-created PRs; the weekly agent pin
  workflow uses an owner-created GitHub App (variable and secret set 2026-10-22).

## Support and process lessons

- Interpret owner wording before building: "vanilla AeroSpace" meant "today's
  config", not AeroSpace's stock defaults, and app pinning was taught, not
  configured. Two drafts were reverted before delivery.
- `git reset --hard` (used to update the checkout before merge) discards local
  edits, including personal AeroSpace pin rules; tell users to commit them.
- Background agents hit turn limits and shipped Windows code that failed its
  first hosted run; treat their work as unverified until final-head CI passes.
- Report flakes as non-reproduction, not as fixed; keep the failed context.
- Do not run Check or a preview while the user runs setup or migrate: both take
  the exclusive installer lock, and the user's run stops with "another installer
  holds the mutation lock" (an agent's concurrent check did this on the Mac,
  2026-10-23).

## Open items

| Item | Status |
|---|---|
| Kiro MSI unpack as an ordinary user (Windows 10b) | Closed: Kiro CLI withdrawn from Windows by owner decision (2026-10-22); MSI unpacking removed |
| xremap reload after a checkout update (Ubuntu 7) | Fixed; hosted proof passed on amd64 and arm64 (1dca9af) |
| Obsidian portable build on Linux (Ubuntu 9) | Proven on amd64 and arm64 (cec98fb); owner to update |
| Slow Windows inspection (Windows 8) | Unmeasured |
| Black screen after first login (Ubuntu 6) | Not reproduced |
| Weekly agent pin PRs need a GitHub App | Working: the App opened PR #93 (agent CLI pins), merged as `b723e97` (2026-10-24) |
| Retire old Ghostty on the Mac (macOS 3) | Done; migration complete and `/nix` gone after the restart (2026-10-23) |
| OpenWhispr Linux launcher class (Ubuntu 13) | Fixed and confirmed on the laptop (2026-10-23) |
| zsh on a directory account (Ubuntu 14) | Merged in PR #91 (`a6246ba`); applied on the laptop (2026-10-24) |
| Neovim as the sudoedit editor (Ubuntu 15) | Merged in PR #92 (`9ec6a9b`); hosted sudo proof passed on macOS and Ubuntu 26.04 (`a9cfa40`); applied on the laptop (2026-10-24) |
| Expired third-party APT key stopped setup (Ubuntu 16) | Owner refreshed the key (2026-10-24); report fixed and refresh made plain `apt-get update` (owner decision) in the follow-up PR |
| Block at the top of `.zshrc` after a Neovim save (Ubuntu 17) | Owner restored the line (2026-10-24); parser fixed in the follow-up PR |
