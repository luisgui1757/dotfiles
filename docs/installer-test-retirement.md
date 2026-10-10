# Installer runtime and test retirement

The major release replaces the monolithic POSIX/PowerShell installers, Nix and
chezmoi activation with the Go installer. This is an intentional runtime removal,
not a compatibility shim. `setup.sh` / `setup.ps1` and `migrate.sh` / `migrate.ps1`
launch the reviewed checkout bootstrap. The machine JSON interface and native
resource drivers own planning, approval, application, checks, updates and removal.

## Preserved boundaries

- Existing user files and personal application state remain protected by the
  configuration, profile, archive and native-provider ownership tests.
- Released live links can still resolve their source files. Passive `home/`
  files, including `.chezmoitemplates` Ghostty/Herdr/LazyGit targets, and Windows
  source templates remain. Removing `.chezmoiroot`, activation metadata and run
  scripts prevents them from acting as a second installer.
- Historical Markdown, release notes, schema-1 publication proofs and the
  immutable-release inventory generators remain. The generators read exact Git
  release trees; they do not require an active chezmoi installation.
- There is no new WSL, Intel macOS, psmux, Nix, Scoop, winget, DNF, pacman, zypper
  or Alpine provisioning path. The supported Linux native provider is APT on
  Debian/Ubuntu; unsupported-provider behavior is deliberately retired.

## Canonical gates

`make ci` runs configuration/editor/shell checks, official Renovate validation and
extraction, bootstrap/migration-evidence Python tests, and Go formatting, vet and
race tests. `test.ps1` retains exact PowerShell analyzer diagnostics, Pester and
Neovim tests and adds the Go and Python bootstrap gates. The three `test.yml`
jobs remain configuration/quality checks. `installer-engine.yml` owns native
four-target core, fresh native-provider and Neovim lifecycle acceptance; it is
maintained separately from the retired workflows.

`e2e-install.yml` and `nix.yml` are retired. Their required check names are replaced
by the actual emitted native job names in `.github/check-identities.json`, the
integrity ruleset, and the safeguarded apply script. The checked-in stage is
`installer-required-pending-apply`: this change does **not** claim live GitHub
policy has been changed. The initial pre-merge cutover requires an authorized maintainer to change the
required contexts after all new checks pass on the exact reviewed PR head, while
preserving the other review/security rules. The safeguard helper deliberately
requires policy committed on exact main, so it cannot itself unblock that first
merge; after merge it audits/reconciles the canonical main policy. Preflight, frozen
transaction inputs, app provenance, exact main, rollback, narrow mutations and
readback checks remain enforced. Historical schema-1 safeguard snapshots and schema-2 check metadata retain
their old restore validation.

## Coverage mapping

Retiring a function-only test does not certify a new implementation. The mappings
below identify the current behavior checks and the required native acceptance.
Hosted native checks must pass on the final PR head; local mocks or cross-builds
do not replace them.

| Retired family | Current proof / intentional semantic change |
| --- | --- |
| `tests/migration/*`, old `Upgrade.Tests.ps1`, Nix owner lifecycle scripts | `legacy_migration_test.go`, `legacy_installed_command_test.go`, `legacy_profile_evidence_test.py`, `legacy_inventory_test.py`: reviewed release evidence, live-link detachment, profile preservation, interrupted recovery, and real migration-then-setup command. Old v0.1-to-v0.4 transaction execution is removed; unfinished released transactions are refused for explicit recovery. |
| chezmoi template/parity/round-trip and uninstall mocks | `config_targets_test.go`, `config_driver_test.go`, `config_restore_test.go`, `config_review_test.go`: canonical resource/source coverage, independently redirected Windows folders, install/update/remove, first baseline, edits, missing sources, changed approvals and restoration. Passive historical mirrors no longer define the new product. |
| `tests/nix/*`, Nix bootstrap/identity mocks | `platform_test.go`, `linux_apt_test.go`, `homebrew_bootstrap_test.go`, native APT/Homebrew/Apple developer tools jobs. Nix evaluation, generations and Home Manager activation are intentionally removed. |
| old `InstallDeps.Tests.ps1`, `Setup.Tests.ps1`, `Uninstall.Tests.ps1`; shell `setup_*`, dry-run, empty flags, update and failure-accumulation mocks | `application_test.go`, `controller_test.go`, `installed_command_native_test.go`, `public_entrypoint_test.py`, `InstallerBootstrap.Tests.ps1`, `installer_bootstrap_test.sh`: approved plans, explicit machine interface, status/exit propagation, read-only check and installed-command lifecycle. Old flag parsing and source-only internal function calls are removed. |
| package table, APT retry, Homebrew bootstrap/completion/shellenv, C compiler, CMake, npm, Python venv mocks | `linux_apt_test.go`, `homebrew_bootstrap_test.go`, `native_*_test.go`, `archive_*_test.go`, profile shell execution tests. Native commands are mocked only at process boundaries; fresh provider jobs prove actual packages and toolchains. |
| per-tool download tests (Neovim, Starship, LazyGit, gh-dash, Herdr, Ghostty, WezTerm, tree-sitter, fonts, VS Code, Pi) | `archive_test.go`, `archive_preparation_test.go`, `archive_deb_test.go`, `archive_zip_links_test.go`, `archive_*_native_test.go`: corrupt/unsafe bytes, exact pin identity, executable/layout requirements, no-replace publication, update, check, shared ownership and offline removal. Desktop native fixtures prove real app resources/signatures/runtime where the host permits it. |
| Windows full Git / Make preparation | `archive_portable_git_test.go` proves unchanged sibling script staging, exclusive creation, error/unfinished refusal and ownership-bound cleanup. `archive_portable_git_cmd_windows_test.go` exercises real CMD self-deletion with private files; `archive_windows_tools_native_test.go` still requires full pinned Git/Make install, changed-version update, interactive staging, SSH/LFS and removal on a disposable runner. Compilation is not native proof. |
| latex2text/Python installation mocks | `archive_language_test.go`, `archive_preparation_native_test.go`, `archive_yamllint_test.go`, `archive_yamllint_native_test.go`: pinned offline wheels/source, private environments, complete dependencies, stable Python and actual commands. |
| checksum/downloader/temp cleanup/zsh publisher stale-race tests | Archive corruption/path/link tests, `archive_crash_test.go`, `archive_cleanup_test.go`, bootstrap adversarial tests, and native worker lost-reply/recovery tests. The old separate shell publisher and generic temp lifecycle are removed. |
| tmux plugin installer mocks | `tmux_profiles_native_test.go` executes the verified plugin directories and current `tmux/plugins.sh` loader, including session restoration and safe quoting. TPM installation is retired. |
| old `ci_logical_proof_test.sh` and marker producers | `tests/bootstrap/release_test.py`: schema-2 exact-tag native job matrix, first attempt, successful exact-head jobs, canonical archived evidence hashes and tag cache policy. Published schema-1 proof bytes remain unchanged. |
| Pi theme merge/uninstall helper tests | Go scoped JSON settings tests cover preservation, invalid JSON, locks and exact owned fields. `pi_theme_test.sh` retains reviewed palette/hash/token assertions; `pi_keybindings_test.sh` remains. The Node helper is removed. |
| notes-vault installer prompt | `tests/nvim/spec/notes_path_spec.lua` and Go preferences/profile tests cover the current explicit setting. The old shell-local prompt/persistence function is removed. |
| fzf, lsd and zsh-plugin mixed tests | Keep guarded integration, completion order, aliases, Rose Pine palette, override and actual rendering assertions. Only old package tables/publishers and active chezmoi mirror parity are removed. Managed runtime profile behavior is also executed by Go shell tests. |
| WSL, devilspie2 install, psmux and obsolete native providers | Unsupported provisioning semantics are intentionally removed. Current terminal/editor configuration tests remain; this does not claim WSL support. |
| broad PowerShell warning baseline | Analyze only current entrypoints, bootstrap, profiles and test harnesses. Nine reviewed warnings retain exact filename/rule/message/extent fingerprint enforcement; diagnostics are not suppressed. |
| active generator/profile assertions applied to passive released mirrors | Keep current tmux generator and active PowerShell no-idle-hook behavior checks. `legacy_inventory_test.py` verifies retained release bytes independently; `checkout_test.go` checks their recorded hashes after a physical `core.autocrlf=true` checkout. Passive source semantics are historical, not regenerated from active recipes. |

Configuration JSON/TOML/YAML, EditorConfig, action SHA pinning, supply-chain remote
execution, repository safeguards, Neovim lock/startup/parser/LSP, shell startup and
keymaps, Starship rendering/performance, tmux, Ghostty and AeroSpace
configuration checks remain part of the gate.

## WezTerm scope retirement (2026-10-10)

The owner removed WezTerm from every OS. Its checkbox, package pins, configuration,
launcher recipes, exclusive Windows argument encoder and dedicated configuration
tests are removed; those tests no longer describe a product requirement. Public
desktop singleton/adoption/restoration coverage now uses VS Code, with separate
consumer subtests so its temporary extension is cleaned up between launches.
Shared fonts, shell tools and native libraries remain. Eight Linux libraries with
no remaining catalog consumers are pruned. APT dependency ownership, selective and
full removal, recovery, reselection and outside-consumer tests retain their real
package relation as explicit fixture data. Historical failure records are not
reclassified as passes.

The passive `home/dot_config/wezterm/wezterm.lua`, generated released target
inventory and its generator remain unchanged: old live links and historical
discovery still need those bytes. The current installer neither publishes nor
removes that personal/released WezTerm state. Historical manual/release checklists
and review records remain evidence of their recorded revisions.

## Pins and future releases

Archive pins and `installer/bootstrap-toolchain.tsv` are reviewed manually.
Maintainers update versions, official published digests (or explicitly documented
computed digests), nested package/component hashes and runtime proof together.
User **Update** reconciles pins in the updated checkout; it does not fetch an
arbitrary latest upstream version. Renovate continues to cover real workflow,
Go/module and validator/CI-tool dependencies. Managers for removed files are
retired, and the official extraction inventory must match the remaining graph.
There is no claimed generic automation for heterogeneous archive hashes.

Future release preparation writes schema 2 and binds `installer-engine.yml` plus
its exact reviewed job names. Certification embeds canonical GitHub run/job
records and exact-commit workflow source with size and SHA-256, checks every job's
commit/run/attempt/success, and verifies tag caching is disabled in that source.
The release-range and evidence scans, reviewed PR/tree/checks, exact annotated
tag, fresh credential-free clone, explicit immutable-publication confirmation,
asset digest and final readback remain. Candidate and closure rendering update
the release manifest, versioned notes and proof only; historical migration prose
and source version stubs are not rewritten.

No release, tag, publication, live safeguard mutation or merge is performed by
this retirement change. The final handoff must distinguish local gate results,
current hosted evidence and any pending owner-host/manual acceptance.

## Local retirement verification

The frozen retirement snapshot passed `make ci`, including the retained
configuration/editor/shell suites, official Renovate extraction (63 records),
all 33 bootstrap Python tests, and Go formatting/vet/race tests. Source hashes
were identical before and after the gate. The final documentation and
candidate-manifest test-fixture adjustment received focused checks afterward.
This is local proof for that snapshot; the integrated final head still requires
the full gate and every canonical hosted context.

The unused `check-herdr-runtime.sh` Homebrew release-range helper and its mock
test are also retired. Private archive identity/version checks replace mutable
Homebrew range checks. The actual compiled-public-command lifecycle now executes
`herdr config check` against the installed managed configuration after install,
update, repair and selective removal on each native target. Retiring old README
range assertions does not waive native configuration consumption.


The superseded Apple preparation tests that required `pkgutil --forget` and a
receipt-free host are retired as an incorrect vendor contract. Their replacement
executes the real preparation receipt block with read-only package queries,
accepted ancestor directory entries, outside/noncanonical payload rejection and
fatal query errors before any move. Existing native reuse and missing-payload
bootstrap with historical receipts are distinct phases of the renamed Apple job.
The old receipt-veto assertion becomes historical-receipt install plus exact
approval/recovery identity, changed install-time, healthy payload and preservation
checks. This does not retire the no-receipt lifecycle or weaken unknown-outcome,
occupied-path, foreign-selection or native-health assertions.


The unused new `shells/zprofile` and `home/dot_zprofile` copies and their copy-only
shell test are retired after review F6. The actual generated login profile remains
covered by `shell_profiles_test.go`, including quiet user-local PATH and personal
hook behavior. The tmux configuration test now seeds an executable old TPM fixture
and proves it is never run when the managed attachment is absent. The released
passive `home/dot_tmux.conf` is restored to exact baseline bytes. The static
extended-key assertion now targets only active `tmux/tmux.conf`; released-inventory
tests hash the passive tmux and WezTerm sources against their exact release records.
A migration source must not acquire current-runtime requirements.
