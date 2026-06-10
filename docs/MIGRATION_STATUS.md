# Chezmoi Migration Status

## chezmoi owns (config layer)

`home/` is the active chezmoi source tree for the dotfiles config layer. It
coexists with the legacy `setup` / `bootstrap` path while the migration proves
itself, so the same logical config must stay single-source: when a top-level
config has a managed copy or template under `home/`, update both in the same
change and let the parity gate prove byte equality. Wave C, which deletes the
old scripts and duplicate source copies, is gated on N green parity runs (the
current plan names N = 10) plus manual owner signoff.

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
| Windows Terminal | `windows-terminal/settings.fragment.jsonc`; `home/.chezmoitemplates/windows-terminal/settings.fragment.jsonc` | Windows: `%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json` | `modify_settings.json.ps1.tmpl` read-modify-write merge. |
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
      symlink, matching `bootstrap.ps1`.
- [x] The migration ruleset payloads are checked in. The three
      `chezmoi-parity*` contexts are listed in the repository safeguard files;
      applying them live is still an owner action.
- [x] Completed execution/spec plan docs are archived in `docs/archive/`.
      `docs/MIGRATION_STATUS.md` is the living migration status document.

### Open

- [ ] N-green-runs counter for Wave C: `0 / 10` consecutive green Ubuntu parity
      runs recorded here. Wave C bootstrap retirement stays blocked until this
      counter is complete and macOS/Windows parity arms are green.
- [ ] Making `chezmoi-parity`, `chezmoi-parity-macos`, and
      `chezmoi-parity-windows` live required checks remains an owner action via
      `scripts/apply-repo-safeguards.sh`.
- [ ] `XDG_DATA_HOME` is not modeled for externals. Chezmoi installs zsh plugins
      to fixed `.local/share`, and the verifier checks that same fixed path; an
      XDG-aware managed root is Wave B.
- [ ] No secrets or `age` tier has been started.
- [ ] Wave C bootstrap/setup retirement is not authorized. `setup.*` still uses
      `bootstrap.*`; full chezmoi-native setup remains a planned next step.
- [ ] The Windows PowerShell profile managed by chezmoi is the PowerShell 7
      path (`Documents\PowerShell\Microsoft.PowerShell_profile.ps1`). The
      Windows PowerShell 5.1 path (`Documents\WindowsPowerShell\...`) remains
      out of scope because the repo is pwsh-first. `bootstrap.ps1` still writes
      to `$PROFILE`, which differs by host shell.
- [ ] The POSIX pwsh profile
      (`~/.config/powershell/Microsoft.PowerShell_profile.ps1`) is intentionally
      not migrated. `bootstrap.sh` links it only when `pwsh` is present, so it
      is install-gated and provisioning-adjacent, like VS Code.
- [ ] Windows Terminal Preview and redirected `%LOCALAPPDATA%` remain Wave B.
- [ ] WSL is not parity-supported in the pilot. Chezmoi models WSL as
      `.chezmoi.os = linux` and cannot distinguish it from native Linux, so on
      WSL it would create `~/.config/ghostty/config` even though `bootstrap.sh`
      skips that path by default. This is a known harmless divergence;
      WSL-aware gating is Wave B.
- [ ] zsh exact-pin checks re-assert when the pin script changes, not on manual
      checkout drift. `refreshPeriod = "0"` means there is no automatic drift.
