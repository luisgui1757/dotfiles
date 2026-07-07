# Dotfiles Roadmap

Last audited: 2026-06-18 on branch `audit/full-roadmap-review-2026-06-18`.
Baseline: `main` at `c3c22b7`.

This is the adversarial post-merge roadmap for the chezmoi migration and the
current setup/CI surface. The goal is not "good enough"; the repo should have a
single obvious setup path, enforce the tests it claims are required, avoid
mutable supply-chain execution where practical, and make greenfield evidence
repeatable instead of tribal.

## Audit Evidence

- Local `make ci`: passed on 2026-06-18 after the final fix round. This covered
  static checks, shellcheck, Neovim specs, shell/starship/tmux/ghostty tests,
  Renovate schema validation, and the POSIX migration bundle.
- Local migration checks passed inside `make ci` and when run directly:
  `template_test.sh`, `parity_gate.sh`, `greenfield_roundtrip.sh`,
  `uninstall_safety_test.sh`, `windows_render_test.sh`, and `oracle_test.sh`.
- After Neovim tests populated generated plugin caches under `tests/.cache`,
  `editorconfig-checker` and Git-backed `chezmoi doctor` exposed traversal
  brittleness. `editorconfig_check.sh` now feeds a pruned per-file list,
  `invariants_test.sh` guards that pruning, and `parity_gate.sh` runs
  `chezmoi doctor` against a temporary copy of `home/` outside the Git checkout.
- `startup_spec.lua` now prewarms the locked Lazy plugin graph only when missing
  and uses a strict best-of-three warm measurement so local scheduler/filesystem
  outliers do not masquerade as production startup regressions.
- Live GitHub protection was applied and verified on 2026-06-18 with
  `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles`.
- The active `Protect main: integrity` ruleset is active, strict, has no bypass
  actors, and requires `ubuntu`, `macos`, `windows`, `chezmoi-parity`,
  `chezmoi-parity-macos`, `chezmoi-parity-windows`,
  `e2e containers / ubuntu-24.04`, `setup.sh / ubuntu-24.04`,
  `setup.sh / macos-15`, and `setup.ps1 / windows-2025`.
- Classic branch-protection fallback is strict and requires the same context
  set. GitHub returns those contexts in a different order than the ruleset API,
  so the apply-script verifier compares exact set membership.
- Post-fix audit hardening added regression coverage for POSIX uninstall
  dry-run immutability, mirrored chezmoi/Starship/tree-sitter pins,
  required-check list duplication, and the Windows Sandbox bootstrap trust root.
- The tmux/psmux Rose Pine bar is now ONE repo-owned generated artifact sourced
  on BOTH platforms (PRs #39 / #41): `tmux/psmux-rose-pine.ps1` renders
  `tmux/psmux-rose-pine.{main,moon,dawn}.conf`, and both POSIX tmux
  (`tmux.posix.conf`) and native-Windows psmux (`tmux.windows.conf`) source the
  SAME deployed variant (`~/.tmux.rose-pine.{main,moon,dawn}.conf`,
  chezmoi-managed on both). The upstream `rose-pine/tmux` TPM plugin is retired
  from the theme surface (it shelled out ~30x at load and hung psmux/ConPTY);
  `main` is the default variant, with `moon` / `dawn` selectable via
  `@rosepine-variant`. POSIX still loads the functional TPM set
  (sensible/yank/resurrect/continuum). Tests cover plugin provisioning, unquoted
  overlay source paths, the Windows `~/.psmux.conf` warm-session guard plus
  flag-free psmux source of `~/.tmux.windows.conf`, generated psmux Rose Pine
  freshness, and the terminal-edge safety space for right-aligned prompt/status
  glyphs.

## Nix + Tooling Migration (2026-07, in progress)

An adversarial architecture review (2026-07-06) accepted, with required changes,
a staged migration toward Nix-owned POSIX **packages** (nix-darwin + Home Manager
+ declarative Homebrew on macOS; Home Manager standalone on Linux/WSL userland)
while **chezmoi stays the single owner of every dotfile target on every OS**, and
native Windows stays on `setup.ps1` + native package managers (Nix has no
supported native-Windows story; WSL2 only). Ruling highlights: exactly one owner
per path (Nix/Home Manager and chezmoi must never co-own a file); GUI /
TCC-sensitive apps (AeroSpace, WezTerm, Herdr) come from vendor channels
(casks / pinned artifacts), never nixpkgs; Herdr native Windows is preview-beta
and stays off by default; no `Invoke-Expression` / `curl|sh` / `irm|iex`
remote-eval installers.

Sequenced PRs (split for independent, revertable blast radius):

- **PR-1 `feat/ergonomics-core` - IN PROGRESS (this branch).** No Nix, no
  vi-mode. Neovim `scrolloff = 16`; which-key.nvim (`<leader>?`, VeryLazy);
  zoxide across zsh + PowerShell + both installers (cached,
  no-`Invoke-Expression` PowerShell init); `gh` + pinned `gh-dash` extension
  (`v4.25.0`) with a chezmoi-managed same-path config and Renovate /
  pin-consistency coverage.
- **PR-2 `feat/vi-mode`** - zsh (`bindkey -v`) + PSReadLine (`-EditMode Vi`)
  command-line vi-mode with a full re-bind matrix (must not break the
  invariant-13 fzf-tab / history stack) and new regression tests.
- **PR-3 `feat/wezterm`** - WezTerm on all OSes (brew cask / pinned `.deb` /
  `$Catalog`), Rose Pine + transparency + Hack Nerd Font parity, chezmoi-only
  config; not a Nix/nixpkgs GUI package.
- **PR-4 `feat/aerospace-herdr`** - AeroSpace (macOS tap cask,
  reserved-chord-safe keymap) + Herdr (macOS/Linux only; native Windows gated
  off).
- **PR-5 `feat/nix-skeleton`** - flake + committed `flake.lock` with ZERO
  ownership; `nix flake check` CI; disjointness test (Home Manager declares no
  file targets); Renovate `nix` manager.
- **PR-6 `feat/nix-darwin`** - macOS host config: `darwinConfigurations` +
  `system.primaryUser`; nix-homebrew (pinned taps, `mutableTaps = false`) +
  homebrew module (`cleanup = "check"`, no auto-update/upgrade); Home Manager
  **packages only**; `--update` gains an `owner=nix` status.
- **PR-7 `feat/nix-linux`** - Home Manager standalone on Ubuntu/WSL userland;
  per-tool apt / pinned-artifact -> nix handoff (nvim last, for ABI reasons);
  native install arms retire only after green e2e + greenfield-ledger evidence.

## P0 - Total Update Ownership Model

Status: shipped on 2026-06-30 in the combined terminal/Markdown/update PR.

The Unix update system now uses per-tool proven ownership instead of one active
package manager for the whole catalog. The install path still chooses a
preferred package manager for new installs, but the update path resolves each
present command from the executable source to a supported owner before taking
action. This covers real machines where Homebrew/Linuxbrew, native Linux
package managers, repo-pinned direct artifacts, and OS-vendor tools coexist.

The ubiquitous uncompromised canonical gold-standard is **per-tool proven
ownership**:

- `--update` is a dependency drift-edge refresh, not a repo update and not
  machine-wide package maintenance.
- `--update` updates every present dotfiles dependency that dotfiles can prove
  is owned by a supported owner.
- Ownership is resolved from the command the shell will actually execute, not
  from a package-list entry alone.
- Every update is scoped to the exact package or repo-pinned artifact for that
  one tool. Never run `brew upgrade`, `apt upgrade`, `dnf upgrade`, `pacman -Syu`,
  `scoop update *`, `winget upgrade --all`, or `choco upgrade all`.
- The output must distinguish `current`, `updated`, `system`, `unmanaged`,
  `blocked`, and `skipped`; a successful no-op must not be printed as
  `updated`.
- Repo-pinned direct downloads are dotfiles-owned artifacts only after dotfiles
  writes durable provenance for them. They may be refreshed only to the version
  and digest pinned in this repo, never to "latest upstream".
- System tools are not automatically defects. Each tool spec decides whether an
  OS-vendor provider is accepted (`/bin/zsh` on macOS), should be migrated to
  the selected developer toolchain (`jq` on a Homebrew-owned macOS profile), or
  should remain unmanaged with an explicit source path.
- Cleanup/prune remains a separate explicit operation. It must not be folded
  into `--update` just to create false cross-manager symmetry.

### Evidence

- `install-deps.sh` now resolves update ownership per tool from the command
  source, not from the global installer package manager.
- Homebrew/Linuxbrew ownership requires the PATH-visible command path and its
  resolved executable target to stay under `brew --prefix`, an installed
  formula, and `brew list --formula <formula>` file ownership of the resolved
  executable. Formula-list presence alone cannot claim `/usr/bin`, another
  shadowing path, a Brew-prefix symlink that resolves outside the Brew prefix,
  or an unrelated file under the Brew prefix.
- Native Linux ownership is source-proven through `dpkg-query -S`, `rpm -qf`,
  `pacman -Qo`, or `apk info --who-owns`, then updated through package-scoped
  manager commands. Pacman-owned packages are reported as `skipped` because a
  package-level update is not the canonical Arch operation.
- Homebrew packages now report `current` without running `brew upgrade` when
  `brew outdated --formula --quiet <pkg>` proves no update is available, and
  exact outdated stdout rows are treated as updates even when Homebrew returns
  nonzero for a named outdated formula.
- Apt metadata refresh runs once per update run. When a successful refresh shows
  installed == candidate, update mode reports `current` without running
  `apt-get install --only-upgrade`; when refresh fails, the scoped upgrade still
  runs against the existing cache.
- Repo-pinned native-Linux artifacts (`nvim`, `lazygit`, `starship`,
  `tree-sitter`, and `chezmoi`) write durable provenance on install and are
  updated only when the marker's command path matches the resolved executable,
  the marker binary lives under the recorded install root, the marker describes
  a supported repo-managed install shape, and the repo pin has changed. Neovim
  is `/usr/local/bin/nvim` pointing into `/opt/nvim-linux-*`; lazygit and
  Starship are `/usr/local/bin/<tool>` or `~/.local/bin/<tool>`; tree-sitter and
  chezmoi are `~/.local/bin/<tool>`. A shadow command path that resolves to the
  same binary is still a provenance mismatch, and an arbitrary marker-named root
  is not ownership. Legacy unmarked binaries remain `unmanaged`.
- macOS `/bin/zsh` reports `system`; normal macOS developer tools still
  resolving from `/usr/bin` report `unmanaged` with a Homebrew migration hint.
- Setup persists Homebrew shellenv and Homebrew GNU Make's `libexec/gnubin` path
  when the `make` formula is installed, so Brew-owned GNU Make is not a hidden
  manual `export PATH=...` step.
- Windows manager-owned packages now report `current` without running a mutating
  package update when Scoop `status`, winget `list --upgrade-available`, or
  Chocolatey `outdated --limit-output` has no exact package row; failed
  availability probes are recorded as update failures, not successful no-ops.
  Scoop availability uses structured status fields: exact `Name`, non-empty
  `Latest Version`, and empty `Info`/`Missing Dependencies`.
- Regression coverage lives in `tests/shell/install_deps_update_test.sh` for
  mixed Linuxbrew/apt dispatch, Homebrew `current`, shadowed Homebrew tools,
  Brew-prefix contradictions, Brew-prefix symlink escapes, external shadow
  symlinks to Brew, apt `current`, pacman skip semantics, macOS system zsh,
  direct-artifact current/unmarked/blocked/refresh behavior, command-path
  shadowing, install-root mismatches, and unsupported direct-artifact install
  shapes, plus install tests that verify provenance markers for Neovim, lazygit,
  Starship, tree-sitter CLI, and chezmoi.

### Required Design

1. Define a first-class update owner model.

   Each catalog tool needs a normalized spec:

   - logical tool name (`nvim`, `jq`, `make`, `tree-sitter`);
   - binary names that prove presence (`fd` and `fdfind` for `fd`);
   - package IDs per package manager;
   - accepted OS-vendor sources, if any;
   - repo-pinned artifact metadata, if dotfiles can own the install directly;
   - whether PATH migration is allowed or required for the tool.

   The dispatcher should resolve:

   ```text
   tool -> executable source -> owner proof -> package/artifact -> action
   ```

   It must not resolve:

   ```text
   tool -> global active package manager -> maybe package exists -> action
   ```

2. Detect all supported Unix owners, not only one active manager.

   The Unix update path should discover every relevant owner available on the
   host:

   - Homebrew/Linuxbrew;
   - native Linux package manager (`apt`, `dnf`, `pacman`, `zypper`, `apk`);
   - repo-pinned dotfiles artifacts;
   - OS-vendor/system providers.

   The install path may still choose a preferred manager for new installs. The
   update path should be stricter: it updates what is already present and owned,
   regardless of whether that owner is the default installer for new tools.

3. Prove ownership from the executable source.

   Required Unix proof rules:

   - Homebrew/Linuxbrew: resolved executable path must live under
     `brew --prefix`, the declared formula must be installed, and
     `brew list --formula <formula>` must list the resolved executable target.
     A formula list entry alone is not enough if PATH still resolves to
     `/usr/bin`, another source, or an unrelated file under the Brew prefix.
   - `apt`: resolved real path must be claimed by `dpkg-query -S`, and the
     owning Debian package must match the catalog package or an explicitly
     declared package alias.
   - `dnf`/`zypper`: resolved real path must be claimed by RPM ownership
     (`rpm -qf`), and the owning RPM must match the declared package or alias.
   - `pacman`: resolved real path must be claimed by `pacman -Qo`, matching the
     declared package or alias.
   - `apk`: resolved real path must be claimed by `apk info --who-owns`,
     matching the declared package or alias.
   - repo-pinned direct artifacts: source path, symlink target, install root,
     and a durable provenance marker must match a dotfiles-owned install shape
     before update mode may reinstall it. Legacy unmarked binaries are not
     automatically adopted.
   - OS-vendor/system: recognized paths such as macOS `/bin/zsh` may be reported
     as `system` only when the tool spec explicitly accepts that provider.
   - unknown paths: report `unmanaged source=<path>` and do nothing.

4. Refresh package metadata once per manager, then update per package.

   Each manager should have a metadata refresh phase used only when at least one
   owned package for that manager is present:

   - Homebrew: use Homebrew's own outdated state. Do not run a formula upgrade
     just to discover it is current.
   - `apt`: run `apt-get update -qq` once, best-effort as already documented.
     When metadata refresh succeeds, compare installed/candidate versions and
     use `apt-get install -y --only-upgrade <pkg>` only when a candidate is
     newer. When metadata refresh fails, preserve the existing resilience
     invariant: still run the scoped `apt-get install -y --only-upgrade <pkg>`
     against the local cache, then report `updated` only if the installed
     package version changed and otherwise report `current` with a stale-cache
     note. A failed metadata refresh alone must not skip the scoped upgrade.
   - `dnf`: use a scoped check/update path for the package, not a system-wide
     upgrade.
   - `pacman`: do not perform a system upgrade as a side effect. If Arch cannot
     safely update a single package without violating pacman's system-upgrade
     model, document that limitation and report the package as `skipped` with a
     reason, rather than pretending `pacman -S <pkg>` is always the canonical
     answer.
   - `zypper`: use scoped package updates.
   - `apk`: use scoped package upgrades.
   - Windows: use manager-specific availability probes (`scoop status`,
     `winget list --upgrade-available`, `choco outdated --limit-output`) before
     any scoped package update, and report `current` instead of mutating when the
     exact package has no available row. Scoop status rows with unhealthy `Info`
     or `Missing Dependencies` fields are update failures, not update proof.

5. Make statuses precise and stable.

   The output should use one status vocabulary across Unix and Windows:

   - `updated`: an update was available and the scoped update completed.
   - `current`: the manager proved the package is already current.
   - `system`: the resolved executable is an accepted OS-vendor provider and is
     intentionally outside dotfiles/package-manager update ownership.
   - `unmanaged`: the tool exists, but no supported owner can prove ownership of
     the resolved executable.
   - `blocked`: ownership exists or is strongly implied, but provenance is
     corrupt, contradictory, or unsafe to update.
   - `skipped`: the tool is absent or intentionally out of scope for this mode.
     It also covers proven owners whose package manager requires an explicit
     operation outside dotfiles' scoped update contract.

   Exit behavior is part of the contract:

   - `updated`, `current`, `system`, `unmanaged`, and `skipped` exit
     successfully unless another tool failed.
   - `blocked` exits nonzero because dotfiles found unsafe or contradictory
     ownership for a present dependency.
   - a scoped update command that fails exits nonzero.

   Output lines should include enough proof to debug without being noisy:

   ```text
   current   jq                        owner=brew package=jq source=/opt/homebrew/bin/jq
   updated   rg                        owner=apt package=ripgrep source=/usr/bin/rg
   system    zsh                       source=/bin/zsh
   unmanaged foo                       source=/usr/local/bin/foo
   blocked   rg                        owner=scoop reason=shim target mismatch
   skipped   make                      owner=pacman reason=requires explicit system upgrade
   skipped   code                      not installed
   ```

6. Make macOS Homebrew developer-toolchain ownership explicit.

   For this repo's macOS profile, the canonical target should be:

   - Homebrew owns normal developer CLI catalog tools (`git`, `make`, `jq`,
     `nvim`, `cmake`, `rg`, `fd`, `fzf`, `lsd`, `chezmoi`, `lazygit`,
     `starship`, `tmux`, `python3`, `node`, `tree-sitter`, `shellcheck`,
     `bats`, `hyperfine`, `taplo`, `yamllint`, and similar).
   - The repo manages Homebrew shellenv and any required PATH adoption. There
     should be no hidden manual `export PATH=...` step.
   - GNU Make's Homebrew `gnubin` path is required for this profile because
     Homebrew's formula exposes GNU Make as `gmake` by default. If the catalog
     says Homebrew owns `make`, setup must manage this PATH entry instead of
     relying on manual shell edits:

     ```sh
     export PATH="$(brew --prefix make)/libexec/gnubin:$PATH"
     ```

   - zsh remains accepted as the macOS system shell by default unless a separate
     login-shell policy intentionally adopts Homebrew zsh and handles
     `/etc/shells`, `chsh`, recovery, and rollback.
   - macOS tools that remain in `/usr/bin` but are normal catalog dependencies
     should get a clear migration hint, not a vague unmanaged line.

7. Treat repo-pinned direct downloads as dotfiles-owned artifacts only after
   provenance exists.

   Native Linux installs without Linuxbrew currently use pinned official
   releases for some tools, but the current install layout is not enough to
   prove ownership forever. The gold-standard update behavior is:

   - add durable provenance markers for fresh direct-artifact installs before
     update mode tries to own them;
   - include tool name, version, source URL, SHA-256, install root, managed
     symlink(s), binary path, installed-binary SHA-256, and installer schema
     version in that provenance;
   - prove the current executable resolves to the dotfiles-managed install
     shape, matching provenance, matching installed-binary checksum, and
     matching executable `--version` output; supported shapes are
     `/usr/local/bin/nvim` pointing into `/opt/nvim-linux-*`,
     `/usr/local/bin/{lazygit,starship}`, and
     `$HOME/.local/bin/{lazygit,starship,tree-sitter,chezmoi}`;
   - reject a different command path even if it is a symlink to the same binary;
   - reject marker binaries that do not live under the recorded install root;
   - compare the installed executable version to the repo pin;
   - reinstall the pinned artifact with SHA-256 verification only when the repo
     pin is newer or the install is corrupt;
   - report `current` when the installed artifact already matches the repo pin;
   - report `blocked` if the path looks dotfiles-owned but the symlink/install
     root/marker is inconsistent.
   - report legacy unmarked direct-download binaries as `unmanaged` unless a
     separate explicit repair/adopt operation writes provenance after validating
     the binary and install root.

   This keeps update mode hermetic to the repo's declared pins without chasing
   upstream "latest" or touching unrelated manually installed binaries.

8. Preserve Windows' stricter provenance lessons.

   Unix and Windows update ownership share the same non-negotiable guarantees:

   - a command source can only be claimed by a manager that proves it owns that
     source;
   - package-list fallback cannot claim a command resolved outside that manager;
   - corrupt provenance is `blocked`, not `unmanaged`;
   - no later manager gets to update a tool after an earlier manager's ownership
     proof is corrupt.

   On Windows, Scoop proves ownership through shim metadata before list fallback.
   Winget and Chocolatey require both an exact package-list row and a command
   source under supported manager install roots; a manual shadow such as
   `C:\Manual\PowerShell\pwsh.exe` remains `unmanaged` even when a package row
   exists.

### Shipped Regression Coverage

The shipped tests prove behavior, not just branches:

1. `tests/shell/install_deps_update_test.sh` simulates command sources and owner
   databases for Linuxbrew, apt, pacman, macOS system zsh, and repo-pinned
   direct artifacts.
2. Mixed-owner dispatch is covered by a single run where Linuxbrew owns `rg`
   while apt owns `jq`; both scoped managers are invoked, and apt reports
   `updated` only after the installed package version changes.
3. Homebrew `current` is covered by proving `brew outdated` has no row and
   asserting `brew upgrade <pkg>` is not called; Homebrew `updated` is covered
   with the real CLI shape where an exact outdated formula row may arrive with a
   nonzero exit.
4. Homebrew shadowing is covered by a formula-installed `git` whose resolved
   source is outside the Homebrew prefix; it is `unmanaged`, not updated.
5. Homebrew prefix contradictions are covered by a command under `brew --prefix`
   whose formula is not installed, and by a formula that is installed but does
   not own the resolved executable file; both runs are `blocked` and exit
   nonzero. Homebrew outdated-probe failures are also `blocked`, not `current`.
6. Apt `current` is covered by installed == candidate after one metadata refresh
   and asserts no scoped install runs. The existing apt-resilience test still
   proves a failed metadata refresh does not skip a scoped upgrade attempt.
   An advertised apt upgrade that leaves the installed version unchanged is
   covered as `blocked`, not `updated`.
7. Pacman ownership is covered as `skipped` with
   `reason=requires explicit system upgrade`.
8. Dnf/zypper RPM ownership and apk ownership are covered with manager-specific
   file-owner proofs and scoped package updates. Zypper/apk outdated-probe
   failures are covered as `blocked`.
9. Direct-artifact coverage includes matching provenance plus matching
   executable version -> `current`, binary checksum mismatch -> `blocked`,
   binary version mismatch -> `blocked`, legacy unmarked binary -> `unmanaged`,
   corrupt marker -> `blocked`, and stale marker -> refresh to the repo pin.
10. Direct-install tests verify schema-2 provenance markers and installed-binary
    checksum fields for Neovim, lazygit, Starship, tree-sitter CLI, and chezmoi.
11. Homebrew shellenv/setup tests prove the managed `make` `libexec/gnubin`
    path is added to the current setup process, persisted for future shells, and
    retrofitted into a legacy managed block without dropping user content.
12. Windows Pester coverage proves source-proven Scoop/winget/Chocolatey update
    ownership, manager-specific `current`/`updated`/availability-failure status,
    manual-source shadows with package rows, and Chocolatey-bin/package-list
    contradictions, and is run with the full gate. Scoop coverage includes lazy
    ownership-gated manifest refresh and fail-closed unhealthy `status` rows.

### Documentation Shipped

1. `README.md` documents the stable status vocabulary, Unix per-tool ownership,
   direct-artifact provenance, Windows source-proven manager proof, and Homebrew
   GNU Make `gnubin` adoption.
2. `README.md` troubleshooting now covers `system`, `unmanaged`, `blocked`,
   Homebrew `make` still resolving to `/usr/bin/make`, and mixed
   Linuxbrew/native-manager machines.
3. `CLAUDE.md` now records the per-tool owner model as the repository invariant,
   including direct-artifact marker requirements and the no-hidden-manual-export
   Homebrew GNU Make policy.
4. Cleanup/prune remains outside `--update`; this roadmap and the README keep
   update scoped to dependency drift only.

### Execution Summary

1. Replaced the global Unix update manager pass with per-tool owner resolution.
2. Added truthful `current` versus `updated` reporting for Homebrew and apt.
3. Added native Linux file-ownership proof for apt, RPM-backed managers,
   pacman, and apk.
4. Added mixed-manager dispatch.
5. Added direct-artifact provenance and marker-proven refresh semantics for
   repo-pinned native-Linux artifacts.
6. Added macOS/Homebrew developer-toolchain PATH adoption for GNU Make.
7. Updated README/CLAUDE/ROADMAP in the same change.
8. Verified with the full shell test bundle and full local gate before handoff.

### Non-Goals

- Do not make `--update` run repo `git pull`.
- Do not make `--update` run `chezmoi apply`.
- Do not make `--update` run Lazy plugin update or rewrite `lazy-lock.json`.
- Do not run blanket package-manager upgrades.
- Do not auto-trust third-party Homebrew taps.
- Do not adopt Homebrew zsh unless login-shell migration is designed as its own
  reversible policy.
- Do not claim "updated" when the manager proved the package was already
  current.

## P0 - Required Gate Reality

### 1. Live `main` protection does not require the `chezmoi-parity*` jobs

Status: done. Live GitHub enforcement was applied and verified on 2026-06-18.

Evidence:

- `.github/workflows/test.yml:213` defines `chezmoi-parity`.
- `.github/workflows/test.yml:256` defines `chezmoi-parity-macos`.
- `.github/workflows/test.yml:292` defines `chezmoi-parity-windows`.
- `.github/rulesets/main-integrity.json:42` through
  `.github/rulesets/main-integrity.json:52` require those contexts in the
  checked-in payload.
- `scripts/apply-repo-safeguards.sh:136` through
  `scripts/apply-repo-safeguards.sh:138` include those contexts in the apply
  script.
- Live GitHub now includes those contexts in both the active
  `Protect main: integrity` ruleset and classic branch-protection fallback.

Risk:

Resolved: future PRs cannot merge through the protected branch path unless the
three `chezmoi-parity*` jobs pass alongside the rest of the required suite.
Static tests still prove checked-in required-check lists agree, and
`scripts/apply-repo-safeguards.sh` now verifies the live ruleset and classic
fallback required-context sets after applying them.

Canonical solution:

1. DONE - Run `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles` with an
   owner/admin credential.
2. DONE - Verify the live `Protect main: integrity` ruleset by fetching the
   specific ruleset ID and asserting the required-status contexts, not only the
   ruleset list endpoint.
3. DONE - Update `docs/security/branch-protection.md` with exact verification
   commands for the live required contexts.
4. DONE - Record the date and output summary in `docs/MIGRATION_STATUS.md`, then
   move the "required checks not live" item to resolved.

## P1 - Supply Chain Integrity

### 2. Mutable remote installer scripts still execute in first-run paths

Status: fixed in `audit/full-roadmap-review-2026-06-18`.

Evidence:

- `install-deps.sh` no longer runs the Homebrew `HEAD` installer; it downloads
  a pinned Homebrew installer commit and verifies SHA-256 before execution.
- Native-Linux `install-deps.sh` no longer runs `get.chezmoi.io`; it downloads
  the pinned chezmoi GitHub release archive and verifies SHA-256 before
  installing.
- `install-deps.sh` no longer pipes the Starship installer into `sh`; native
  Linux/WSL without brew uses pinned Starship release archives with SHA-256
  verification, while Alpine uses its native package.
- `install-deps.ps1:157` downloads `https://get.scoop.sh` and executes it.
- CI no longer runs the Starship installer script as fallback; it downloads the
  pinned Starship release tarball and verifies SHA-256 before extraction.
- CI no longer runs `get.chezmoi.io`; POSIX parity jobs use
  `scripts/install-pinned-chezmoi.sh`, and Windows parity downloads the pinned
  chezmoi zip and verifies SHA-256 before extraction.
- Recommended setup docs no longer use raw `curl | bash`/`iwr` execution of the
  current default branch; they use `git clone` plus local `setup`.
- The repo already has stronger patterns for other downloads, for example
  Ghostty script verification in `install-deps.sh`, Hack font verification in
  `install-deps.ps1`, and Windows Terminal portable verification in
  `install-deps.ps1`.

Risk:

The repo has two supply-chain standards at once. Direct archives and some
installer scripts are pinned and hash-checked, while package-manager/bootstrap
scripts remain mutable trust roots. A compromised upstream script or transient
server response becomes code execution during setup.

Resolution:

1. Repository policy now requires direct network executables to be pinned and
   verified, or explicitly allowlisted with a rationale.
2. Native Linux chezmoi moved from `get.chezmoi.io` script execution to a
   SHA-256-verified release artifact.
3. The Starship curl installer fallback was replaced by SHA-256-verified
   release artifacts on native Linux/WSL without brew.
4. `tests/static/supply_chain_remote_execution_test.sh` rejects new
   `curl | sh`, `sh -c "$(curl ...)"`, raw `/tmp/*install*.sh` execution, and
   PowerShell `scriptblock::Create` execution unless the exact line appears in
   the reviewed allowlist.
5. CI Starship, tree-sitter CLI, and chezmoi installs now use SHA-256-verified
   release artifacts.
6. The remaining mutable installer trust roots are Scoop bootstrap on Windows
   (consent-gated package-manager bootstrap) and the documented disposable
   Windows Sandbox self-bootstrap path; both are explicit in the static
   allowlist.

## P1 - Greenfield Proof

### 3. Windows Sandbox greenfield path still points at the retired pilot branch

Status: done.

Evidence:

- `tests/greenfield/windows-sandbox.wsb` now downloads the bootstrap from
  `main` and passes `-Ref 'main'` by default.
- `tests/greenfield/sandbox-bootstrap.ps1` now defaults `$Ref` to `main`.
- `tests/greenfield/README.md` and `tests/greenfield/RUNBOOK.md` document the
  explicit PR/branch override path.
- `tests/static/stale_greenfield_refs_test.sh` fails if the retired pilot
  branch name appears outside archived historical docs.

Risk:

Resolved: advertised clean-machine proofs now test `main` unless a reviewer
explicitly opts into a PR/branch override.

Canonical solution:

1. DONE - Default every greenfield script and runbook path to `main`.
2. DONE - Add an explicit `-Ref` or documented URL edit for PR validation.
3. DONE - Add a cheap static test for the retired branch name outside archived
   historical docs.
4. ENVIRONMENTAL - Run the Windows Sandbox path once on a Windows host and append
   the result to `tests/greenfield/LEDGER.md`. This macOS checkout cannot launch
   Windows Sandbox; the script, XML, parser, and stale-ref guard are validated
   locally.

### 4. Greenfield visual/manual evidence has no durable ledger

Status: done.

Evidence:

- `tests/greenfield/LEDGER.md` is now the append-only record for
  clean-machine and visual evidence.
- `tests/greenfield/README.md` and `tests/greenfield/RUNBOOK.md` point
  reviewers to the ledger after automated and manual runs.
- `docs/MIGRATION_STATUS.md` now points the N-green counter at the ledger.

Risk:

Resolved for documentation structure: manual checks remain honest manual
evidence, and the repo now has one durable place to append them.

Canonical solution:

1. DONE - Add a small append-only greenfield evidence ledger:
   `tests/greenfield/LEDGER.md`.
2. DONE - Include environment, branch/SHA, command, pass/fail, and remaining
   manual observations fields.
3. DONE - Point the N-green counter in `docs/MIGRATION_STATUS.md` at the
   ledger.
4. DONE - Keep visual checks documented as manual evidence with dates and exact
   machines.

## P1 - Automation Coverage

### 5. Renovate schema validation is documented but not merge-gated

Status: done.

Evidence:

- `.github/workflows/test.yml` now runs `make validate-renovate` in the required
  Ubuntu job immediately after static lint.
- `scripts/validate-renovate.sh` now fails instead of skipping when `npx` is
  missing under `CI=true`.
- `scripts/validate-renovate.sh` pins both the Node runtime and Renovate
  validator package; `renovate.json` owns those pins through custom managers.
- `Makefile` exposes `make ci` / `make test-required`, which include Renovate
  schema validation.

Risk:

Resolved: a schema-invalid `renovate.json` now fails the required Ubuntu CI job
and the canonical local `make ci` gate.

Canonical solution:

1. DONE - Add `make validate-renovate` to the required Ubuntu CI path.
2. DONE - Make missing Node/npm/npx fatal under `CI=true`.
3. DONE - Keep `make test-static` fast, and add a `make ci` or
   `make test-required` target that matches the required local proof bundle.

### 6. There is no single canonical local "full gate" command

Status: done.

Evidence:

- `Makefile` now defines `make ci` and `make test-required`.
- The full local gate runs `make test`, `make validate-renovate`, and
  `make test-migration`.
- `make test-migration` runs the POSIX migration template, parity, round-trip,
  uninstall safety, Windows render, and oracle checks with `~/.local/bin` on
  `PATH`.
- `README.md` and `CLAUDE.md` document `make ci` as the exact local pre-PR gate.

Risk:

Resolved: future agents have one top-level command for the required local proof
bundle. `make test` remains the current-host fast suite; `make ci` is the full
pre-PR gate.

Canonical solution:

1. DONE - Add a top-level `make ci` or `make test-required`.
2. DONE - Include `make test`, `make validate-renovate`, and the host-appropriate
   migration checks.
3. DONE - Keep OS-specific skips explicit and fail missing required tools in CI.
4. DONE - Document that `make test` is the current-host fast suite, while the
   new target is the exact pre-PR local gate.

### 7. The static editorconfig sweep is not stable after generated test caches exist

Status: done.

Evidence:

- `tests/nvim/minimal_init.lua:53` through `tests/nvim/minimal_init.lua:72`
  clones and checks out Plenary under `tests/.cache/plenary.nvim`.
- `.gitignore:5` ignores `tests/.cache/`, so the cache is intentionally local
  generated state.
- `tests/static/json_lint.sh`, `tests/static/yaml_lint.sh`,
  `tests/static/toml_lint.sh`, `tests/static/ps1_parse.sh`, and shell lint
  already exclude `tests/.cache`.
- `tests/static/editorconfig_check.sh` now excludes `.git`, `.claude`,
  `tests/.cache`, `home`, and `nvim/lazy-lock.json`.
- After `make test`, the local `tests/.cache` contained 7,435 paths and 43 MB of
  generated dependency/test state. After the fix, `bash tests/static/run_all.sh`
  passes with that cache present.
- `tests/static/invariants_test.sh` fails if the editorconfig sweep stops
  excluding generated `tests/.cache` content.

Risk:

Resolved: the fast static gate no longer scans generated Plenary/cache content
after dynamic tests populate `tests/.cache`.

Canonical solution:

1. DONE - Exclude `tests/.cache` from `tests/static/editorconfig_check.sh`,
   matching the other static scanners.
2. DEFERRED - Prefer keeping generated dependency clones outside the repo root if a future
   harness cleanup makes that practical.
3. DONE - Add a regression test or invariant that all repo-wide file walkers exclude
   `tests/.cache` unless they are intentionally testing cache contents.

## P2 - Config Semantics

### 8. zsh plugin root semantics split between XDG-aware runtime and fixed chezmoi paths

Status: done. The canonical contract is fixed
`~/.local/share/dotfiles/zsh-plugins`; install-deps, runtime, chezmoi externals,
pin verification, uninstall, greenfield validation, container/WSL validation,
and parity fixtures now use or assert that root.

Evidence:

- `shells/zshrc:69` loads zsh plugins from the fixed
  `$HOME/.local/share/dotfiles/zsh-plugins` root.
- `install-deps.sh` installs zsh plugins into the fixed
  `$HOME/.local/share/dotfiles/zsh-plugins` root.
- `home/.chezmoiexternal.toml.tmpl:4` and
  `home/.chezmoiexternal.toml.tmpl:11` install chezmoi externals to fixed
  `.local/share` paths.
- `home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl:5`
  through line 6 verify the fixed `.local/share` path.
- `uninstall.sh:176` through `uninstall.sh:179` and `uninstall.ps1:155` through
  `uninstall.ps1:160` only classify fixed `.local/share` externals.
- `tests/migration/parity_gate.sh`, `tests/migration/oracle_test.sh`,
  `tests/migration/greenfield_roundtrip.sh`, and
  `tests/migration/uninstall_safety_test.sh` now set a hostile `XDG_DATA_HOME`
  while asserting the fixed plugin root.
- `tests/ci/container-e2e.sh`, `tests/wsl/e2e.sh`, and
  `tests/greenfield/validate.sh` validate the fixed plugin root.

Risk:

Resolved: hosts with `XDG_DATA_HOME` set still install, apply, verify, source,
validate, and uninstall zsh plugins through the same fixed repo-managed tree.

Canonical solution:

DONE - Removed XDG support from `install-deps.sh`'s `zsh_plugin_root()` so the
installer uses the same fixed `~/.local/share/dotfiles/zsh-plugins` path as
every other zsh plugin surface.

## Disproved Or Non-Blocking Assumptions

- Shell, zsh, and PowerShell syntax are not current blockers:
  `bash -n`, `zsh -n`, PowerShell parser coverage, `make test-static`, and
  `make test` passed.
- The current `.chezmoi.toml.tmpl` no longer has the prior `mode` nesting bug.
- The tmux prefix regression is not present; the shared config uses `C-b`.
- Recurring Neovim traps checked clean by source and tests: leader before
  lazy.nvim, Mason headless commands registered, conform remains the only
  format-on-save path, and Treesitter bundled-parser cleanup is scoped.
- Checked-in ruleset/settings/apply-script required-check lists are internally
  synchronized; the problem is live GitHub enforcement drift.
- `renovate.json` is currently schema-valid.
- The direct `uninstall_safety_test.sh` passes; the earlier chained local
  migration command did not reproduce a persistent repo failure.

## Preferred Execution Order

1. DONE - Apply and verify live repository safeguards so the parity jobs are
   actually required.
2. DONE - Fix stale greenfield branch references and add a guard against
   reintroducing them.
3. DONE - Make the static editorconfig sweep ignore generated caches.
4. DONE - Add Renovate validation and a canonical top-level full-gate target.
5. DONE - Decide and enforce the supply-chain policy for remote executable scripts.
6. DONE - Resolve the zsh plugin root contract.
7. DONE - Start a greenfield evidence ledger and move the N-green counter out
   of informal memory.
