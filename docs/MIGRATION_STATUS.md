# Chezmoi Migration Status

## chezmoi owns (config layer)

`home/` is the active chezmoi source tree for the dotfiles config layer. It now
backs the public `setup.sh` / `setup.ps1` Phase 2 config apply. The same logical
config must stay single-source: when a top-level config has a managed copy or
template under `home/`, update both in the same change and let the parity gate
prove byte equality. Wave C stage 4 retired the old direct config scripts and
made the parity gate canonical-only; the N-green counter and required-check
application remain owner-tracked release controls.

| Config | Source file(s) | Per-OS target(s) | Chezmoi mechanism |
|---|---|---|---|
| Neovim | `nvim/`; `home/dot_config/symlink_nvim.tmpl`; `home/AppData/Local/symlink_nvim.tmpl` | macOS/Linux: `~/.config/nvim`; Windows: `%LOCALAPPDATA%\nvim` | Directory symlink to repo `nvim/` on every OS. |
| Starship | `starship/starship.toml`; `home/dot_config/starship.toml` | macOS/Linux: `~/.config/starship.toml`; Windows: `%USERPROFILE%\.config\starship.toml` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| zshenv | `shells/zshenv`; `home/dot_zshenv` | POSIX: `~/.zshenv`; Windows: ignored | POSIX symlink via `mode = "symlink"`. |
| zshrc | `shells/zshrc`; `home/dot_zshrc` | POSIX: `~/.zshrc`; Windows: ignored | POSIX symlink via `mode = "symlink"`. |
| Ghostty | `ghostty/config`; `home/.chezmoitemplates/ghostty/config` | macOS: `~/Library/Application Support/com.mitchellh.ghostty/config`; Linux: `~/.config/ghostty/config`; Windows: n/a | Per-path POSIX `symlink_config.tmpl` entries into `.chezmoitemplates`. |
| lazygit | `lazygit/config.yml`; `home/.chezmoitemplates/lazygit/config.yml` | macOS: `~/Library/Application Support/lazygit/config.yml`; Linux/WSL: `~/.config/lazygit/config.yml`; Windows: `%LOCALAPPDATA%\lazygit\config.yml` | POSIX path-specific symlinks; Windows rendered copy from the shared template. |
| tmux | `tmux/tmux.conf`; `home/dot_tmux.conf` | POSIX: `~/.tmux.conf`; Windows: `%USERPROFILE%\.tmux.conf` | POSIX symlink via `mode = "symlink"`; Windows copy via `mode = "file"`. |
| tmux Windows overlay | `tmux/tmux.windows.conf`; `home/dot_tmux.windows.conf` | Windows: `%USERPROFILE%\.tmux.windows.conf`; POSIX: ignored | Windows copy only; `tmux.conf` sources it with `source-file -q`. |
| tmux POSIX overlay | `tmux/tmux.posix.conf`; `home/dot_tmux.posix.conf` | POSIX: `~/.tmux.posix.conf`; Windows: ignored | POSIX symlink only. Holds the native-clipboard `if-shell` probes, which hang psmux at config-load time, so it is **never** deployed on Windows; `tmux.conf` sources it with `source-file -q`. |
| Windows Terminal | `windows-terminal/settings.fragment.jsonc`; `home/.chezmoitemplates/windows-terminal/settings.fragment.jsonc` | Windows: `%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json` | `setup.ps1` backs up an existing pre-merge file, then `modify_settings.json.ps1.tmpl` performs the read-modify-write merge. |
| PowerShell profile | `shells/powershell_profile.ps1`; `home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1` | Windows PS7: `%USERPROFILE%\Documents\PowerShell\Microsoft.PowerShell_profile.ps1`; Windows PowerShell 5.1 and POSIX pwsh profile paths stay out of chezmoi scope | Windows PS7 profile copy via `mode = "file"`. |
| zsh plugins | `home/.chezmoiexternal.toml.tmpl`; `home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl` | POSIX: `~/.local/share/dotfiles/zsh-plugins/{zsh-autocomplete,zsh-autosuggestions}`; Windows: ignored | Pinned `.chezmoiexternal` git repos plus `run_onchange_` exact-commit assertion. |

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
- the 5 pinned binary/font installers: Neovim Linux, lazygit Linux, Hack Nerd
  Font, Ubuntu Ghostty, and win32yank
- the zsh login-shell switch and domain-account fallback
- devilspie2 package install, daemon rule, and autostart entry
- VS Code install, `mvllow.rose-pine` extension install, and
  `workbench.colorTheme` merge
- the distro/package-manager matrix and manager fallback policy
- no-TTY auto-all, best-effort continuation, and dry-run installer semantics

Rationale: re-owning this duplicates the best-tested, highest-risk part of the
repo for little operational gain.

The canonical split is `chezmoi=dotfiles, install-deps=provisioning`. VS Code
theme setup stays provisioning-adjacent because it is app-install-gated and
JSONC-fragile.

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
- Bootstrap-style backups named `<target>.bak.<timestamp>` are restored after
  a target is removed. Pass `--no-restore-backups` / `-NoRestoreBackups` to
  skip restoration.
- The zsh plugin externals under
  `~/.local/share/dotfiles/zsh-plugins/{zsh-autocomplete,zsh-autosuggestions}`
  are removed unless `--keep-externals` / `-KeepExternals` is passed. A checkout
  with uncommitted/untracked changes (or one whose cleanliness cannot be
  verified) is kept even under `--all`; pass `--force-externals` /
  `-ForceExternals` to remove it anyway. This protects in-place edits to the
  vendored plugin clones from being lost.
- Windows Terminal `settings.json` is never deleted. The merge is idempotent
  but not invertible, so the scripts warn and print the newest matching backup
  path when present.

Both scripts support dry-run and non-interactive flags:
`--dry-run --all --keep-externals --no-restore-backups --force-externals` on
POSIX, and `-DryRun -All -KeepExternals -NoRestoreBackups -ForceExternals` on
Windows. Adversarial safety (dirty external preserved, user-replaced managed
file skipped, broken repo-symlink still cleaned) is covered by
`tests/migration/uninstall_safety_test.sh`.

## Owner sign-off / known caveats

### Resolved

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
- [x] Wave C stage 4 is done on branch `chezmoi-pilot`: the legacy direct
      config scripts and their direct test harness were deleted, the old Make
      target was retired, setup remains chezmoi-native, the native-apt container
      e2e applies via chezmoi, and `tests/migration/parity_gate.sh` is
      canonical-only by default.
- [x] Wave C review fixes restored `setup.ps1`'s copy-based Windows Terminal
      pre-merge backup, tightened setup's self-link and backup-match guards, and
      made the POSIX parity gate prove config-file targets are symlinks, not
      only content-equivalent files.

### Open

- [ ] N-green-runs counter for Wave C: `0 / 10` consecutive green Ubuntu parity
      runs recorded here. Stage 4 has landed on branch `chezmoi-pilot`; this
      counter remains owner release evidence alongside green macOS/Windows
      parity arms.
- [ ] Making `chezmoi-parity`, `chezmoi-parity-macos`, and
      `chezmoi-parity-windows` live required checks remains an owner action via
      `scripts/apply-repo-safeguards.sh`.
- [ ] `XDG_DATA_HOME` is not modeled for externals. Chezmoi installs zsh plugins
      to fixed `.local/share`, and the verifier checks that same fixed path; an
      XDG-aware managed root is Wave B.
- [ ] No secrets or `age` tier has been started.
- [ ] The Windows PowerShell profile managed by chezmoi is the PowerShell 7
      path (`Documents\PowerShell\Microsoft.PowerShell_profile.ps1`). The
      Windows PowerShell 5.1 path (`Documents\WindowsPowerShell\...`) remains
      out of scope because the repo is pwsh-first. Host-shell-specific
      `$PROFILE` paths remain outside the static chezmoi tree.
- [ ] The POSIX pwsh profile
      (`~/.config/powershell/Microsoft.PowerShell_profile.ps1`) is intentionally
      not migrated. It is install-gated and provisioning-adjacent, like VS Code.
- [ ] Windows Terminal Preview and redirected `%LOCALAPPDATA%` remain Wave B.
- [ ] Full WSL parity is still not a required automated gate, but chezmoi now
      models WSL through the generated `isWsl` data value and skips Linux
      Ghostty by default. `setup.sh --experimental-wsl-gui` passes the
      `experimentalWslGui` data override so WSL Ghostty is managed only on the
      explicit GUI-terminal opt-in path.
- [ ] zsh exact-pin checks re-assert when the pin script changes, not on manual
      checkout drift. `refreshPeriod = "0"` means there is no automatic drift.
