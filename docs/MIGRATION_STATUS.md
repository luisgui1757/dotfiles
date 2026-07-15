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
| Herdr | `herdr/config.toml`; `herdr/config.windows.toml`; `home/.chezmoitemplates/herdr/config.toml`; `home/dot_config/herdr/symlink_config.toml.tmpl`; `windows/chezmoi-appdata/herdr/symlink_config.toml.tmpl` | macOS/Linux/WSL: `~/.config/herdr/config.toml`; Windows: actual roaming ApplicationData `herdr\config.toml` | POSIX path-specific symlink and dedicated Windows ApplicationData destination state; both force built-in `rose-pine`, while Windows additionally selects `pwsh.exe` so panes load the managed PowerShell 7 profile and PSReadLine history UI. |
| lazygit | `lazygit/config.yml`; `home/.chezmoitemplates/lazygit/config.yml`; `windows/chezmoi-localappdata/lazygit/symlink_config.yml.tmpl` | macOS: `~/Library/Application Support/lazygit/config.yml`; Linux/WSL: `~/.config/lazygit/config.yml`; Windows: actual LocalApplicationData `lazygit\config.yml` | POSIX path-specific symlinks; Windows known-folder symlink to the native config. |
| gh-dash | `gh-dash/config.yml`; `home/dot_config/gh-dash/config.yml` | macOS/Linux/WSL: `~/.config/gh-dash/config.yml`; Windows: `%USERPROFILE%\.config\gh-dash\config.yml` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| lsd | `lsd/config.yaml`; `lsd/colors.yaml`; `home/dot_config/lsd/config.yaml`; `home/dot_config/lsd/colors.yaml` | macOS/Linux/WSL: `~/.config/lsd/{config.yaml,colors.yaml}`; Windows: `%USERPROFILE%\.config\lsd\{config.yaml,colors.yaml}` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. The shell profiles own Rose Pine `LS_COLORS` for file/directory names, with `DOTFILES_LS_COLORS` as the explicit override; `colors.yaml` owns long-list metadata. |
| tmux | `tmux/tmux.conf`; `home/dot_tmux.conf` | POSIX: `~/.tmux.conf`; Windows: `%USERPROFILE%\.tmux.conf` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| tmux Windows overlay | `tmux/tmux.windows.conf`; `home/dot_tmux.windows.conf` | Windows: `%USERPROFILE%\.tmux.windows.conf`; POSIX: ignored | Windows copy only; `tmux.conf` sources it with `source-file -q`. |
| tmux POSIX overlay | `tmux/tmux.posix.conf`; `home/dot_tmux.posix.conf` | POSIX: `~/.tmux.posix.conf`; Windows: ignored | POSIX symlink only. Holds the native-clipboard `if-shell` probes, which hang psmux at config-load time, so it is **never** deployed on Windows; `tmux.conf` sources it with `source-file -q`. |
| psmux | `tmux/psmux.conf`; `home/dot_psmux.conf` | Windows: `%USERPROFILE%\.psmux.conf`; POSIX: ignored | Windows copy only. It is the first native-Windows multiplexer entrypoint and source-files the tmux Windows overlay. |
| Generated Rose Pine tmux/psmux bar | `tmux/psmux-rose-pine.ps1`; generated `tmux/psmux-rose-pine.{main,moon,dawn}.conf`; `home/dot_tmux.rose-pine.ps1`; `home/dot_tmux.rose-pine.*.conf` | POSIX/Windows: `~/.tmux.rose-pine.{main,moon,dawn}.conf`; Windows also gets `~/.tmux.rose-pine.ps1` | Source generator plus checked generated configs; POSIX symlinks, Windows copies. |
| Windows Terminal | `windows-terminal/settings.fragment.jsonc`; `home/.chezmoitemplates/windows-terminal/{settings.fragment.jsonc,merge-settings.ps1}`; `scripts/windows-terminal-targets.ps1` | Windows stable packaged + Preview + Canary + portable settings paths | `setup.ps1` is the only publisher. Chezmoi exposes no WT target. One validated enumerator is shared by setup, release migration/recovery, and uninstall. Setup independently merges each selected target's own state, stages beside the destination, validates all plans, creates separate verified backups, atomically publishes with concurrent-change detection, and rolls back the multi-target transaction on failure. |
| PowerShell profiles | `shells/powershell_profile.ps1`; `windows/chezmoi-documents/{PowerShell,WindowsPowerShell}/symlink_*_profile.ps1.tmpl` | actual Documents known folder for ConsoleHost, VS Code, and ISE; active runtime `$PROFILE` must resolve to one of them | Dedicated Documents destination state; every supported host profile symlinks to the canonical source and setup post-checks consumption. |
| zsh plugins | `scripts/ensure-pinned-zsh-plugin.sh`; `home/.chezmoiscripts/run_onchange_after_20-ensure-zsh-plugin-pins.sh.tmpl` | POSIX: `~/.local/share/dotfiles/zsh-plugins/{fzf-tab,zsh-autosuggestions}`; Windows: ignored | Install-deps and pin/helper changes in chezmoi share the serialized sibling-stage publisher. Unproved payloads are quarantined before fetch; only expected-origin, exact-HEAD, clean, tracked-entry-file checkouts publish atomically. Generic git-repo externals are intentionally absent. |

The migration oracle is manifest-driven:
`tests/migration/parity_gate.sh`, `tests/migration/oracle_test.sh`, and
`tests/migration/windows_apply_test.ps1` run across the Ubuntu, macOS, and
Windows `chezmoi-parity*` CI jobs. Static linters intentionally exclude
`home/`; the parity gate validates managed copies against the canonical
top-level sources instead.

## Versioned v0.1.0 release migration

`v0.1.0` is a chezmoi release, not a pre-chezmoi install. Its POSIX targets are
live symlinks into the source checkout, so the former README `git pull` path was
unsafe: it could publish new bytes before current setup reached backup. The
canonical v0.2.0 path is now side-by-side and exact-tag-only:

- `setup.sh --all` and `setup.ps1 -All` are the sole normal user entrypoints.
  They discover exact live v0.1.0 ownership, invoke the platform transaction,
  resume an already-applied recovery at validation/acceptance, retain private
  recovery evidence, and continue through full idempotent setup. POSIX setup
  also invokes the checksum-reviewed Nix prerequisite helper when Nix is absent.
  `--update` / `-Update` runs this complete reconciliation before its scoped
  proven-owner dependency and Mason refresh; `--upgrade` / `-Upgrade` is an
  alias.

- `scripts/upgrade-v0.1.0.sh` handles Apple Silicon macOS, native Linux, and
  WSL guest state. It validates both official annotated releases, the target
  account/home, Nix, clean trees, and exact historical config before mutation;
  captures private package/config recovery plus digest-bound exact-commit
  source trees; applies only Nix plus config
  files/links while deferring chezmoi run scripts; and
  automatically removes the first nix-darwin/Home Manager activation and
  reapplies v0.1.0 on later failure or interruption.
- `scripts/upgrade-v0.1.0.ps1` handles native Windows without Nix. It applies
  config files/symlinks with dependencies, Neovim caches, agent policy, and
  chezmoi run scripts skipped; resolves actual known folders; freezes both
  exact release trees plus stable packaged, Preview, Canary, and portable Terminal recovery bytes under a
  protected ACL, publishes and rolls back only from those trees, retains
  conventional v0.1 targets until acceptance, removes only transaction-created
  overlay state on rollback, and validates all four canonical Terminal paths
  before any restore write.
- `scripts/install-nix-prerequisite.sh` installs only checksum-reviewed upstream
  Nix 2.34.0 release archives. Before v0.2.0 publication it accepts only a clean
  exact current official branch head; publication closes that path and requires
  the exact annotated tag object and peeled commit. Setup owns its invocation;
  no downloaded bytes execute before the platform SHA-256 matches, and the
  verified installer runs non-interactively in the selected daemon mode with
  `nix-command flakes` persisted. A retry reconciles the disabled-feature state
  left by an otherwise-complete upstream install.
- `tests/migration/v0_1_upgrade_test.sh` materializes the exact peeled v0.1.0
  commit, proves in-place/dirty paths fail before mutation, runs the real setup
  config/backup path, injects a failure after Home Manager/config publication,
  proves later checkout drift cannot change publication, rejects altered
  recovery payloads, and proves exact rollback, retry, and
  acceptance. Windows Pester pins digest-bound release trees, complete/frozen Terminal recovery,
  all-target concurrency rejection, known-folder state validation, and the
  pre-migration command-provider boundary.

The release remains gated on real Apple Silicon owner-host, WSL split-host,
redirected Windows, and divergent stable packaged/Preview/Canary/portable Terminal
executions. Until the annotated
v0.2.0 tag and those release rows exist, v0.1.0 users are told to remain on
v0.1.0. No non-Apple-Silicon macOS migration path is shipped or pending proof.

## install-deps owns (provisioning -- deliberately NOT in chezmoi)

Provisioning stays in `install-deps`, not chezmoi run-scripts:

- package installs from Unix `PKG_TABLE` and Windows `$Catalog`
- psmux installation on Windows, including the hardened `Add-ScoopBucketSafe`
  bucket-add path in `install-deps.ps1`
- Windows direct-artifact bin directories are de-duplicated and promoted to the
  front of process and User `PATH`, so an already-listed but shadowed managed
  Tree-sitter installation self-repairs without uninstalling the older tool
- pinned binary/font/script installers and direct artifacts: Homebrew installer,
  Neovim Linux, native-Linux chezmoi, lazygit Linux, Starship Linux,
  tree-sitter CLI Linux/Windows, WezTerm Ubuntu `.deb`, Herdr Linux, Herdr Windows
  preview, Hack Nerd Font, Windows Terminal portable zip, exact Ghostty
  Debian-family `.deb` assets, Scoop
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
  stable packaged, Preview, Canary, and portable candidate sets plus backup
  JSON before restoring any target,
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

- [x] Stable packaged, Preview, Canary, and portable Windows Terminal settings are independent
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
      the flake/sudo boundary. Darwin activation is Apple-Silicon-only;
      unsupported x86_64 fails before activation with migration guidance, and
      Homebrew paths follow the actual Apple Silicon repository.
      First nix-darwin bootstrap collision-checks and preserves existing
      `/etc/bashrc` and `/etc/zshrc` as `.before-nix-darwin`, rolling both back
      on failure/interruption while retaining failed generated output. A retry
      from the terminal that performed first activation resolves the installed
      current-system rebuild command outside stale `PATH`; exact `/etc/static`
      shell links and retained backups are accepted as already-managed state.
- [x] nix-homebrew uses mixed tap/package ownership. `mutableTaps = true` with
      no Nix-managed tap contents leaves every tap clone owned by target-user
      Homebrew, while `cleanup = "none"` preserves formulae/casks outside the
      generated Brewfile. setup never moves the whole `Library/Taps` directory;
      it transactionally migrates only the three exact root-owned, non-Git tap
      snapshots created by the retired pinned-tap shape and never selects an
      unrelated user tap. Transaction, failure, and recovery snapshots live
      beside `Library/Taps`, never below Homebrew's live-tap scan root. Setup
      automatically relocates the exact in-tree artifact names produced by the
      broken predecessor before retry. `tests/macos_owner_lifecycle.sh` is the
      destructive real-host install/update/uninstall/reinstall validation path.
      Setup prerequisite discovery also recovers a canonical daemon/user Nix
      profile binary when it is present but an already-guarded login profile no
      longer exports it after Homebrew path refresh.
- [x] Fresh Linux/WSL zsh startup consumes Home Manager's canonical session-vars
      file once from the XDG profile, `~/.nix-profile`, or the
      system-integrated `/etc/profiles/per-user/<effective-user>` profile, with
      no-Nix hosts guarded. Standalone Linux sets `home.sessionPath` from the
      evaluated Home Manager profile directory, so the sourced file exports
      Nix-owned tools without caller PATH injection. Brew-less macOS dry-run
      now previews all later phases instead of aborting after the bootstrap
      plan.
      Native-Linux CI resolves and executes the effective account's actual
      login zsh from the account database; it does not assume `/usr/bin/zsh`
      exists after setup selected a platform-specific shell.

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
- [x] The checked-in ruleset, required-check metadata, and transactional apply
      path now require `ubuntu`, `macos`,
      `windows`, `chezmoi-parity`, `chezmoi-parity-macos`,
      `chezmoi-parity-windows`, `nix flake check / linux`,
      `nix flake check / macos`, `e2e containers / linux`, `setup.sh / linux`,
      `setup.sh / macos`, and `setup.ps1 / windows`. The static alignment test
      binds the ruleset, apply function, and API payload to that exact set.
      `.github/settings.yml` intentionally omits branch protection so the
      Probot Settings app cannot race the owner-run transaction on merge.
- [x] Stable logical replacements for the six runner-versioned contexts are
      emitted and bound to exact per-OS proof artifacts. Marker schema 2 binds
      the PR source head separately from GitHub's actually executed synthetic
      merge SHA; push/dispatch proofs truthfully record equal identities.
      Legacy producer names remain emitted so the currently live legacy
      safeguards can gate this cutover PR. `.github/check-identities.json` and
      `docs/security/branch-protection.md` define the post-merge cache-free gate
      and owner-applied live switch. The apply script now completes and repeats
      a full read-only preflight before mutation: exact branch/repo/main and
      public visibility, clean sources, exact legacy live policy, unique
      rulesets, exact GitHub-Actions app/workflow/event/run provenance, and
      cache-free E2E evidence. It snapshots and transactionally restores the
      three cutover resources on failure. After the second capture, apply freezes
      every desired payload from exact committed objects and publishes only that
      private read-only set. Restore freezes every consumed file, requires every
      full-classic field, validates exact manifest-stage
      Actions/integrity/classic policy and live ruleset identity, and rejects
      incomplete, altered, or cross-stage recovery material before writing, with
      expected policy loaded from the manifest's still-live captured commit and
      a tested explicit `--restore` retry. Temporary captures clean up on every
      exit, pre-mutation snapshots are pruned, and semantic YAML tests prevent
      any top-level Probot `branches` shape. UGR-020
      remains PARTIAL until the merged-main proof, live apply, and readback
      succeed. Repaired PR head
      `4dbdb959674f5a062cffe44daae242318f4c1b67` passed all 12 legacy-required
      and six stable logical contexts in runs `29140112029`, `29140112035`, and
      `29140112030`; all six downloaded schema-2 markers bound that source head
      to the executed synthetic merge SHA. The PR E2E run used ordinary PR
      caches, so it is not promoted to the pending merged-main cache-free gate.
- [x] Native Windows no longer derives LocalApplicationData, ApplicationData,
      or Documents from UserProfile. Setup/uninstall share one validated
      known-folder identity, apply separate UserProfile/LocalApplicationData/
      ApplicationData/Documents chezmoi source states, post-check application
      consumers, and preserve divergent legacy conventional-path data.
      Redirected-folder runtime proof remains listed below because this checkout
      is not that environment.
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
- [x] Native `windows-2025` public setup passed again on exact PR head
      `f4b63953f2f982702a685358b09e89bae2d78fdd` (run `29092384014`, job
      `86360593122`): all six
      phases, exact Tree-sitter `0.26.10`, Hack Nerd Font file and registry
      consumption, Pi `0.80.3`, and the strict 257-check Neovim language smoke.
      This does not close the redirected-folder or four-variant Windows Terminal manual
      rows below.

### Open

- [x] Apple Silicon is the sole current Darwin contract by explicit owner
      direction. The flake, setup selector, CI matrices, pinned installers,
      migration tool, tests, and current user documentation contain no alternate
      macOS architecture path. The generic platform boundary rejects every
      other architecture before Nix/Homebrew activation. Historical host runs
      remain only in append-only evidence and are not current support or pending
      proof.

- [ ] Greenfield/manual evidence remains intentionally bounded:
      `tests/greenfield/LEDGER.md` now records exact-head hosted Ubuntu, Apple
      Silicon, historical Intel, and Windows automated runs. Manual cache-free run
      `29096335827` skipped every broad cache; attempt 1 passed Ubuntu,
      container, and Windows but exposed a real asynchronous nvim-treesitter
      build race on Apple Silicon while Intel independently hit transient DNS.
      Attempt 2 on the same unrepaired SHA passed Apple Silicon but failed the
      Intel neocmake attach assertion. Branch-head run `29100106370` then passed
      Apple Silicon but exposed remaining headless async parser installs via
      missing Astro captures on Ubuntu and GraphQL captures on Intel. These are
      recorded as partial/failed evidence, not promoted to a green run; the
      implementation now blocks interactive auto-install in ordinary headless
      processes as well as waiting on the build callback. Exact behavior head
      `e5cf3e23299cbb42a157c307f2a7259979fcada0` subsequently passed cache-free
      run `29103732329` across Ubuntu container, public Ubuntu, Apple Silicon,
      historical Intel, native Windows, and all four setup logical proofs.
      Merged-main run `29114125798` then passed every current producer except
      Apple Silicon, where the initial CMake LSP fixture shared a large project
      root and neocmakelsp timed out before attach; the later isolated CMake
      formatter fixture attached in the same process. The project-isolation
      repair then passed cache-free branch-head run `29120109175` on exact SHA
      `f097995b49a2189db327903a20743e7cb69ba665`: all four current producers
      and four setup logical proofs were green. PR-head run `29180481941` later
      exposed the remaining duplicate lifecycle in the opposite order: the
      isolated CMake attachment passed, then a second formatter-only
      neocmakelsp start timed out. Attachment and realistic gersemi formatting
      now share one isolated client lifecycle; three repeated strict
      Apple-Silicon runs passed all 257 checks. Exact repaired head
      `d744948cdccc51f3d79e45aa78f82c46445df0c6` then passed E2E run
      `29181215803`, including all four producers and logical proofs. Merged-main
      confirmation remains.
      No Windows Sandbox, WSL, redirected-Windows, merged-main cache-free
      confirmation, or desktop visual run is claimed. Required CI is not manual
      desktop evidence. The old Wave C `0 / 10` Ubuntu parity counter is no
      longer the current release gate; current CI proof is the required
      parity/e2e/Nix workflow set plus explicit ledger entries.
- [ ] No secrets or `age` tier has been started.
- [ ] The POSIX pwsh profile
      (`~/.config/powershell/Microsoft.PowerShell_profile.ps1`) is intentionally
      not migrated. It is install-gated and provisioning-adjacent, like VS Code.
- [ ] Redirected Windows known-folder runtime confirmation is pending a real
      Windows host with Documents, LocalApplicationData, and ApplicationData on
      alternate paths.
      Pester and migration round-trip fixtures are implementation proof, not a
      claim about a host run.
- [ ] Full WSL runtime parity remains manual. The optional hosted canary was
      retired after scheduled run `29072773410` and manual run `29114215045`
      both reached WSL2 but stalled before setup evidence; GitHub does not
      officially support the nested-virtualization dependency. Chezmoi still
      models WSL through generated `isWsl`, skips Linux Ghostty by default, and
      honors `--experimental-wsl-gui`. The real throwaway-distro and split-host
      harnesses remain the proof path; Linux CI/static coverage is only a proxy.
- [ ] Out-of-band zsh checkout tampering is rechecked by the next setup or
      pin-sensitive chezmoi apply; no background monitor is promised. Every
      publisher/verifier path neutralizes an unproved sourceable payload before
      it can fail.
