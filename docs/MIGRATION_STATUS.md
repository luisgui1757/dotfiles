# Chezmoi Migration Status

## chezmoi owns (config layer)

`home/` is the active chezmoi source tree for the dotfiles config layer. It now
backs the public `setup.sh` / `setup.ps1` Phase 2 config apply. The same logical
config must stay single-source: when a top-level config has a managed copy or
template under `home/`, update both in the same change and let the parity gate
prove byte equality. Wave C stage 4 retired the old direct config scripts and
made the parity gate canonical-only; the N-green counter and required-check
application remain owner-tracked release controls. Greenfield desktop evidence
is tracked in `tests/greenfield/LEDGER.md`; docs and launchers default to
`main`, with PR validation handled by explicit branch overrides in the
greenfield runbook.

| Config | Source file(s) | Per-OS target(s) | Chezmoi mechanism |
|---|---|---|---|
| Neovim | `nvim/`; `home/dot_config/symlink_nvim.tmpl`; `windows/chezmoi-localappdata/symlink_nvim.tmpl` | macOS/Linux: `~/.config/nvim`; Windows: actual LocalApplicationData `nvim` | Directory symlink to repo `nvim/` on every OS; Windows uses the dedicated known-folder destination state. |
| Starship | `starship/starship.toml`; `home/dot_config/starship.toml` | macOS/Linux: `~/.config/starship.toml`; Windows: `%USERPROFILE%\.config\starship.toml` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| zshenv | `shells/zshenv`; `home/dot_zshenv` | POSIX: `~/.zshenv`; Windows: ignored | POSIX symlink via `mode = "symlink"`. |
| zshrc | `shells/zshrc`; `home/dot_zshrc` | POSIX: `~/.zshrc`; Windows: ignored | POSIX symlink via `mode = "symlink"`. |
| Ghostty | `ghostty/config`; `home/.chezmoitemplates/ghostty/config` | macOS: `~/Library/Application Support/com.mitchellh.ghostty/config`; Linux: `~/.config/ghostty/config`; Windows: n/a | Per-path POSIX `symlink_config.tmpl` entries into `.chezmoitemplates`. |
| WezTerm | `wezterm/wezterm.lua`; `home/.chezmoitemplates/wezterm/wezterm.lua`; `home/dot_config/wezterm/wezterm.lua` | macOS/Linux/WSL GUI opt-in: `~/.config/wezterm/wezterm.lua`; Windows: `%USERPROFILE%\.config\wezterm\wezterm.lua` | POSIX path-specific symlinks; Windows copy. WSL skips Linux GUI terminal config unless setup passes the explicit GUI override. |
| AeroSpace | `aerospace/aerospace.toml`; `home/dot_config/aerospace/aerospace.toml` | macOS: `~/.config/aerospace/aerospace.toml`; Linux/Windows: ignored | macOS symlink via `mode = "symlink"`. |
| lazygit | `lazygit/config.yml`; `home/.chezmoitemplates/lazygit/config.yml`; `windows/chezmoi-localappdata/lazygit/symlink_config.yml.tmpl` | macOS: `~/Library/Application Support/lazygit/config.yml`; Linux/WSL: `~/.config/lazygit/config.yml`; Windows: actual LocalApplicationData `lazygit\config.yml` | POSIX path-specific symlinks; Windows known-folder symlink to the native config. |
| gh-dash | `gh-dash/config.yml`; `home/dot_config/gh-dash/config.yml` | macOS/Linux/WSL: `~/.config/gh-dash/config.yml`; Windows: `%USERPROFILE%\.config\gh-dash\config.yml` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| lsd | `lsd/config.yaml`; `lsd/colors.yaml`; `home/dot_config/lsd/config.yaml`; `home/dot_config/lsd/colors.yaml` | macOS/Linux/WSL: `~/.config/lsd/{config.yaml,colors.yaml}`; Windows: `%USERPROFILE%\.config\lsd\{config.yaml,colors.yaml}` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. The shell profiles own Rose Pine `LS_COLORS` for file/directory names, with `DOTFILES_LS_COLORS` as the explicit override; `colors.yaml` owns long-list metadata. |
| tmux | `tmux/tmux.conf`; `home/dot_tmux.conf` | POSIX: `~/.tmux.conf`; Windows: `%USERPROFILE%\.tmux.conf` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| tmux Windows overlay | `tmux/tmux.windows.conf`; `home/dot_tmux.windows.conf` | Windows: `%USERPROFILE%\.tmux.windows.conf`; POSIX: ignored | Windows copy only; `tmux.conf` sources it with `source-file -q`. |
| tmux POSIX overlay | `tmux/tmux.posix.conf`; `home/dot_tmux.posix.conf` | POSIX: `~/.tmux.posix.conf`; Windows: ignored | POSIX symlink only. Holds the native-clipboard `if-shell` probes, which hang psmux at config-load time, so it is **never** deployed on Windows; `tmux.conf` sources it with `source-file -q`. |
| psmux | `tmux/psmux.conf`; `home/dot_psmux.conf` | Windows: `%USERPROFILE%\.psmux.conf`; POSIX: ignored | Windows copy only. It is the first native-Windows multiplexer entrypoint and source-files the tmux Windows overlay. |
| Generated Rose Pine tmux/psmux bar | `tmux/psmux-rose-pine.ps1`; generated `tmux/psmux-rose-pine.{main,moon,dawn}.conf`; `home/dot_tmux.rose-pine.ps1`; `home/dot_tmux.rose-pine.*.conf` | POSIX/Windows: `~/.tmux.rose-pine.{main,moon,dawn}.conf`; Windows also gets `~/.tmux.rose-pine.ps1` | Source generator plus checked generated configs; POSIX symlinks, Windows copies. |
| Windows Terminal | `windows-terminal/settings.fragment.jsonc`; `home/.chezmoitemplates/windows-terminal/{settings.fragment.jsonc,merge-settings.ps1}` | Windows packaged + portable settings paths | `setup.ps1` is the only publisher. Chezmoi exposes no WT target. Setup independently merges each target's own state, stages beside the destination, validates all plans, creates separate verified backups, atomically publishes with concurrent-change detection, and rolls back the multi-target transaction on failure. |
| PowerShell profiles | `shells/powershell_profile.ps1`; `windows/chezmoi-documents/{PowerShell,WindowsPowerShell}/symlink_*_profile.ps1.tmpl` | actual Documents known folder for ConsoleHost, VS Code, and ISE; active runtime `$PROFILE` must resolve to one of them | Dedicated Documents destination state; every supported host profile symlinks to the canonical source and setup post-checks consumption. |
| zsh plugins | `scripts/ensure-pinned-zsh-plugin.sh`; `home/.chezmoiscripts/run_onchange_after_20-ensure-zsh-plugin-pins.sh.tmpl` | POSIX: `~/.local/share/dotfiles/zsh-plugins/{fzf-tab,zsh-autosuggestions}`; Windows: ignored | Install-deps and pin/helper changes in chezmoi share the serialized sibling-stage publisher. Unproved payloads are quarantined before fetch; only expected-origin, exact-HEAD, clean, tracked-entry-file checkouts publish atomically. Generic git-repo externals are intentionally absent. |

The migration oracle is manifest-driven:
`tests/migration/parity_gate.sh`, `tests/migration/oracle_test.sh`, and
`tests/migration/windows_apply_test.ps1` run across the Ubuntu, macOS, and
Windows `chezmoi-parity*` CI jobs. Static linters intentionally exclude
`home/`; the parity gate validates managed copies against the canonical
top-level sources instead.

## install-deps owns (provisioning -- deliberately NOT in chezmoi)

Provisioning stays in `install-deps`, not chezmoi run-scripts:

- package installs from Unix `PKG_TABLE` and Windows `$Catalog`
- psmux installation on Windows, including the hardened `Add-ScoopBucketSafe`
  bucket-add path in `install-deps.ps1`
- pinned binary/font/script installers and direct artifacts: Homebrew installer,
  Neovim Linux, native-Linux chezmoi, lazygit Linux, Starship Linux,
  tree-sitter CLI Linux/Windows, WezTerm Ubuntu `.deb`, Herdr Linux, Herdr Windows
  preview, Hack Nerd Font, Windows Terminal portable zip, Ubuntu Ghostty, Scoop
  installer, Pi CLI packed-tarball SRI, and pinned `setuptools`/`pylatexenc`
  converter wheels/sdists
- the zsh login-shell switch and domain-account fallback
- devilspie2 package install, daemon rule, and autostart entry
- VS Code install, `mvllow.rose-pine` extension install, and
  VS Code user settings merge
- the distro/package-manager matrix and manager fallback policy
- no-TTY auto-all, best-effort continuation, and dry-run installer semantics

Rationale: re-owning this duplicates the best-tested, highest-risk part of the
repo for little operational gain.

The canonical split is `chezmoi=dotfiles, install-deps=provisioning`. VS Code
theme/font setup stays provisioning-adjacent because it is app-install-gated and
depends on the `code` CLI being available.

## Uninstall

`uninstall.sh` and `uninstall.ps1` are safe teardown tools for greenfield
testing on a machine that already has the config layer applied. They enumerate
the layer with `chezmoi --source <repo>/home managed --path-style absolute`,
then remove only targets they can prove are repo-owned:

- POSIX and Windows symlinks are removed only when their resolved target points
  inside this checkout (`home/` or the canonical repo tree such as `nvim/`).
- Windows copy-mode files are removed only when `chezmoi --source <repo>/home
  verify <target>` confirms they still match the managed state (byte-exact via
  chezmoi's own logic, with none of the stdout-redirect encoding/CRLF pitfalls a
  `cat`-and-hash comparison would hit on Windows). User-modified files warn and
  stay in place.
- Bootstrap-style backups named `<target>.bak.<YYYYMMDD-HHMMSS>[.n]` are
  restored after a target is removed. Selection uses the validated filename
  timestamp plus numeric collision suffix, never filesystem mtime; malformed or
  ambiguous candidates fail before removal. Pass `--no-restore-backups` /
  `-NoRestoreBackups` to skip restoration.
- The zsh plugin externals under
  `~/.local/share/dotfiles/zsh-plugins/{fzf-tab,zsh-autosuggestions}`
  are removed unless `--keep-externals` / `-KeepExternals` is passed. A checkout
  with uncommitted/untracked changes (or one whose cleanliness cannot be
  verified) is kept even under `--all`; pass `--force-externals` /
  `-ForceExternals` to remove it anyway. This protects in-place edits to the
  vendored plugin clones from being lost.
- Windows Terminal `settings.json` is never deleted. `uninstall.ps1` validates
  both packaged/portable candidate sets and backup JSON before restoring either,
  atomically restores the selected pre-setup bytes, and preserves the displaced
  current file as `settings.json.uninstall-current.<timestamp>[.n]`.

Both scripts support dry-run and non-interactive flags:
`--dry-run --all --keep-externals --no-restore-backups --force-externals` on
POSIX, and `-DryRun -All -KeepExternals -NoRestoreBackups -ForceExternals` on
Windows. POSIX dry-run also leaves empty zsh-external parent directories in
place instead of pruning `~/.local/share/dotfiles`. Adversarial safety (dry-run
does not mutate, dirty external preserved, user-replaced managed file skipped,
broken repo-symlink still cleaned) is covered by
`tests/migration/uninstall_safety_test.sh`.

## Owner sign-off / known caveats

### Resolved

- [x] Packaged and portable Windows Terminal settings are independent
      user-owned merge transactions. The old full-file mirror and best-effort
      warning path are removed. Setup stages/validates all outputs, backs up
      each divergent target, detects concurrent changes through atomic rollback
      bytes, fails setup on any unsafe operation, and supports dry-run, skip,
      retry, idempotency, and independent uninstall restoration.
- [x] POSIX and Windows uninstall choose backups from validated filename
      timestamps/collision suffixes instead of mtime. Opposing mtime order,
      files/directories, collisions, malformed names, and pre-mutation failure
      are covered by shell and Pester tests.

- [x] POSIX setup now resolves one authoritative non-root target account and
      account-record home before Nix, Home Manager, chezmoi, or native setup.
      It rejects a mismatched ambient `HOME` instead of fabricating
      `/Users/<user>` or `/home/<user>`, and threads the validated values through
      the flake/sudo boundary. Darwin has separate aarch64 and x86_64 activation
      configurations; Homebrew paths follow the actual repository/architecture.
      First nix-darwin bootstrap collision-checks and preserves existing
      `/etc/bashrc` and `/etc/zshrc` as `.before-nix-darwin`, rolling both back
      on failure/interruption while retaining failed generated output.
- [x] nix-homebrew tap migration is transactional. Existing taps move to a
      collision-safe backup, and installed/bootstrap activation failure or an
      interruption quarantines the failed replacement and restores the old
      state. A rollback failure leaves the backup intact and prints exact manual
      recovery rather than guessing.
- [x] Fresh Linux/WSL zsh startup consumes Home Manager's canonical session-vars
      file once from the XDG profile, `~/.nix-profile`, or the
      system-integrated `/etc/profiles/per-user/<effective-user>` profile, with
      no-Nix hosts guarded. Brew-less macOS dry-run now previews all later
      phases instead of aborting after the bootstrap plan.

- [x] Windows `nvim` directory-symlink round-trip is fixed in commit `eed6690`.
      The Windows template renders a clean, backslash, no-`..` absolute path
      into repo `nvim/`, so `chezmoi verify` no longer reports perpetual drift.
      It still needs Developer Mode or elevation because it is a directory
      symlink.
- [x] The migration ruleset payloads are checked in. The three
      `chezmoi-parity*` contexts are listed in the repository safeguard files;
      applying them live is still an owner action.
- [x] Completed execution/spec plan docs are archived in `docs/archive/`.
      `docs/MIGRATION_STATUS.md` is the living migration status document.
- [x] Wave C stage 3 moved public `setup.sh` / `setup.ps1` Phase 2 to chezmoi.
      Setup initializes chezmoi, backs up pre-existing divergent managed targets
      before forced apply, preserves `--skip-bootstrap` / `-SkipBootstrap` as
      aliases for the new config skip, and keeps the Windows Developer
      Mode/elevation pre-flight before apply.
- [x] Wave C stage 4 is done on the pilot migration branch: the legacy direct
      config scripts and their direct test harness were deleted, the old Make
      target was retired, setup remains chezmoi-native, the native-apt container
      e2e applies via chezmoi, and `tests/migration/parity_gate.sh` is
      canonical-only by default.
- [x] Wave C review fixes restored `setup.ps1`'s copy-based Windows Terminal
      pre-merge backup, tightened setup's self-link and backup-match guards, and
      made the POSIX parity gate prove config-file targets are symlinks, not
      only content-equivalent files.
- [x] Windows setup now matches the POSIX backup-match logic for copy-mode
      targets: `setup.ps1` captures `chezmoi cat` bytes to a temp file, keeps
      the symlink-reference path branch, and byte-compares ordinary managed
      files against the captured content before deciding to create a
      `<target>.bak.<timestamp>` backup.
- [x] Greenfield Windows Sandbox, manual Windows, macOS VM, and Linux VM
      instructions now default to `main`; PR validation uses documented branch
      overrides, and `tests/static/stale_greenfield_refs_test.sh` guards against
      reintroducing the retired pilot branch name outside archived historical
      docs.
- [x] `tests/greenfield/LEDGER.md` is the append-only evidence ledger for
      clean-machine automated runs and manual visual observations.
- [x] `XDG_DATA_HOME` is intentionally not modeled for zsh plugin externals.
      The repo-wide contract is the fixed
      `~/.local/share/dotfiles/zsh-plugins` root. Chezmoi installs there,
      `install-deps.sh` installs there, `shells/zshrc` sources there first, the
      verifier checks there, uninstall removes there, and parity tests assert
      that root under a hostile `XDG_DATA_HOME`.
- [x] Checked-in `main` protection sources require `ubuntu`, `macos`, `windows`,
      `chezmoi-parity`, `chezmoi-parity-macos`, `chezmoi-parity-windows`,
      `nix flake check (ubuntu-24.04)`, `nix flake check (macos-26)`,
      `e2e containers / ubuntu-24.04`, `setup.sh / ubuntu-24.04`,
      `setup.sh / macos-26`, and `setup.ps1 / windows-2025` as of
      2026-07-09. Applying them live remains an owner/admin action through
      `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles`; the static
      required-check alignment test keeps ruleset/settings/script mirrors in
      sync.
- [x] Stable logical replacements for the six runner-versioned contexts are
      emitted and bound to exact per-OS proof artifacts. Stage 1 intentionally
      leaves legacy contexts required; `.github/check-identities.json` and
      `docs/security/branch-protection.md` define the deadlock-free second PR
      and owner-applied live switch. UGR-020 remains PARTIAL until that happens.
- [x] Native Windows no longer derives LocalApplicationData or Documents from
      UserProfile. Setup/uninstall share one validated known-folder identity,
      apply separate UserProfile/LocalApplicationData/Documents chezmoi source
      states, post-check application consumers, and preserve divergent legacy
      conventional-path data. Redirected-folder runtime proof remains listed
      below because this checkout is not that environment.
- [x] PowerShell 7, VS Code, Windows PowerShell ConsoleHost, and ISE profile
      targets are managed under the actual Documents known folder. The current
      runtime `$PROFILE` must be one of those post-apply consumers.
- [x] Packaged, Preview, and portable Windows Terminal target discovery follows
      actual LocalApplicationData. Each existing target is an independent merge
      and recovery transaction; none is mirrored from another installation.
- [x] Windows known-folder post-apply and legacy-shape ownership checks resolve
      both symbolic links and junctions before comparing canonical targets. The
      cross-platform Windows test job installs the exact checksum-pinned
      chezmoi release so expected-drift tests execute instead of skipping.
- [x] Lazy's detached executable bootstrap now proves locked default-branch
      metadata anchored to the same immutable commit. This preserves fail-closed
      execution while allowing Lazy's own lock writer to identify its branch.

### Open

- [ ] Intel macOS runtime confirmation is pending the exact PR-head
      `macos-26-intel` Nix/setup runs. Both configurations cross-evaluate and
      setup selection is behaviorally tested, but those are not runtime proof.
- [ ] Nixpkgs 26.05 is the final `x86_64-darwin` release and is supported only
      through 2026-12-31. Before that date, migrate Intel's package plane to a
      still-supported mechanism without narrowing the public macOS contract or
      moving chezmoi-owned dotfiles into the package layer. The warning remains
      intentionally unsuppressed.

- [ ] Greenfield evidence remains intentionally sparse: `tests/greenfield/LEDGER.md`
      still records no Windows Sandbox, WSL, macOS VM, or Linux VM clean-machine
      run after the initial static docs guard. Do not count required CI as manual
      desktop greenfield evidence. The old Wave C `0 / 10` Ubuntu parity counter
      is no longer the current release gate; current CI proof is the required
      parity/e2e/Nix workflow set plus any explicit ledger entries.
- [ ] No secrets or `age` tier has been started.
- [ ] The POSIX pwsh profile
      (`~/.config/powershell/Microsoft.PowerShell_profile.ps1`) is intentionally
      not migrated. It is install-gated and provisioning-adjacent, like VS Code.
- [ ] Redirected Windows known-folder runtime confirmation is pending a real
      Windows host with Documents and LocalApplicationData on alternate paths.
      Pester and migration round-trip fixtures are implementation proof, not a
      claim about a host run.
- [ ] Full WSL parity is still not a required automated gate, but chezmoi now
      models WSL through the generated `isWsl` data value and skips Linux
      Ghostty by default. `setup.sh --experimental-wsl-gui` passes the
      `experimentalWslGui` data override so WSL Ghostty is managed only on the
      explicit GUI-terminal opt-in path.
- [ ] Out-of-band zsh checkout tampering is rechecked by the next setup or
      pin-sensitive chezmoi apply; no background monitor is promised. Every
      publisher/verifier path neutralizes an unproved sourceable payload before
      it can fail.
