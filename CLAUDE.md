# Guide for coding agents

This file is the canonical guide for coding agents in this repository.
`AGENTS.md` only points to this file. Do not copy this guide into another file.
`README.md` is the operator guide. Keep its first sections short: platforms,
install, update and removal, then the cheat sheets.

## The repository

This repository holds personal dotfiles and a native Go installer. The
installer shows a menu of tools. It resolves their dependencies, applies the
shared Rose Pine configuration and records what it owns. Update, Check, repair
and removal use these records.

Release v1.0.0 replaced the earlier Nix and chezmoi setup. `migrate.sh` and
`migrate.ps1` move a v0.4.x installation to the new installer.

## Supported platforms

| Target | Support |
|---|---|
| macOS | Apple Silicon (arm64) only. The installer stops Intel and Rosetta before provider discovery. |
| Linux | Ubuntu and Debian on amd64 and arm64. Only `ID=ubuntu` or `ID=debian` in os-release passes. `ID_LIKE` does not add a distribution. |
| Windows | Native Windows amd64. The minimum is Windows 10 or Windows Server 2016. |
| WSL | WSL uses the Linux graph. There is no WSL detector and no setup of the Windows host. |

APT installs system packages on Ubuntu and Debian. Homebrew installs them on
macOS. Signed Microsoft installers supply the Windows runtimes and Build Tools.
Portable tools use private archives with SHA-256 pins on all three systems.
Other distributions, Linuxbrew, musl and Intel macOS are out of scope.

## Layout

| Path | Content |
|---|---|
| `installer/` | The Go installer. `resources.json` is the catalog, `archive-pins.json` holds the archive pins. |
| `installer/shells/` | The active shell integration sources |
| `installer/config-targets.json` | The configuration target manifest |
| `setup.sh`, `setup.ps1` | Interactive entrypoints. They run only from a local clone. |
| `migrate.sh`, `migrate.ps1` | Migration from the Nix and chezmoi releases |
| `scripts/installer-bootstrap.*` | The verified private Go bootstrap |
| `nvim/`, `tmux/`, `herdr/`, `ghostty/`, `windows-terminal/`, `aerospace/`, `xremap/`, `git/`, `lazygit/`, `gh-dash/`, `starship/`, `lsd/` | Tool configuration |
| `home/`, `shells/`, `windows/chezmoi-*` | Passive released files for migration. Do not edit them. |
| `release/` | The release manifest and proofs |
| `tests/` | Tests by area. `tests/static/` holds the repository checks. |
| `.github/` | Workflows, rulesets and the identities of the checks that must pass |
| `docs/` | Upgrade, release and security guides, and release notes |

## Commands

| Command | Use |
|---|---|
| `make ci` | The full local gate. Run it before a pull request. |
| `.\test.ps1` | The local gate on Windows. CI also uses it. |
| `make test` | Static checks, lint, and the Neovim, shell, Starship, tmux, Ghostty and AeroSpace tests |
| `make test-static` | `tests/static/run_all.sh` |
| `make test-installer` | `gofmt`, `go vet` and `go test -race -timeout 30m ./...` in `installer/` |
| `make test-bootstrap` | Python tests for the launchers and the migration evidence |
| `make validate-renovate` | The Renovate configuration and its extraction inventory |
| `make release-check`, `release-prepare`, `release-publish` | Release commands. Read `docs/RELEASING.md` first. |

`make ci` runs `test`, `validate-renovate`, `test-bootstrap` and
`test-installer`. It needs the Go version in `installer/go.mod` and Ruby.
Linux process tests need `cc` with pthreads. The C++ isolation test needs
`clang++`. A sub-target skips with a message when its tool is missing.

CI has three workflows:

- `test.yml` runs the configuration tests on Ubuntu, macOS and Windows.
- `installer-engine.yml` runs the installer core matrix and the native
  lifecycle jobs on disposable GitHub runners.
- `agent-cli-pins.yml` proposes agent CLI pin updates each week.

`.github/check-identities.json` lists the checks that must pass. CodeQL adds
its own checks. A native job can continue after a failed step. Read the result
of each step. All of these checks must pass on the final head.

## Safety rules

- Do not run native package, font or GUI tests on an everyday workstation.
  Only opted-in disposable runners run them. The `DOTFILES_TEST_*` variables,
  for example `DOTFILES_TEST_NATIVE_PACKAGES=1`, enable them.
- Do not delete state, journals, locks or receipts to get a clean install.
- Ownership comes only from a provider proof of a recorded operation. Equal
  bytes, a package name or an inventory difference never give ownership.
- Removal deletes only owned, unchanged and unused resources. Personal data,
  packages that were there before and outside consumers stay.
- Never run a global autoremove. Never use Homebrew `--ignore-dependencies` or
  cask `--force`. Never purge an APT package.
- Never change kernel, sysctl, AppArmor, TCC, Gatekeeper or quarantine
  settings. Never sign vendor bytes again. Never change the account login shell.
- Never write personal agent settings, credentials, models or sessions.
- Setup runs only from a local clone. Do not document a remote script that
  goes through a pipe into a shell.
- Never disable Node TLS certificate checks.
- Each mutation needs a full preview and an explicit confirmation. End of
  input and keys typed early never approve.
- Keep unrelated changes in a dirty worktree.
- Merge and release publication are owner actions, unless the owner gives them
  to you.

## Installer invariants

- `installer/resources.json` is the only dependency list. The planner is pure.
  Providers do the platform IO. The menu has no second list. Categories are
  display data only.
- A resource reaches a provider only through its platform binding.
- An approval binds the source, catalog, target, selection, state and observed
  artifacts. When the plan changes, the review shows again. Never apply a
  stale plan.
- Save `Receipt.OperationID` and the provider intent before a mutation. Save
  the native result before the reply.
- Keep the first baseline through each update. Removal restores it.
- An unknown or pending observation never proves absence. It stays local to its
  resource and does not stop independent resources.
- Resume uses the original executable and checkout. It never builds old intent
  again from current pins.
- Saved recipes, schemas and command encodings stay readable. Never change,
  rewrite or replay an old recipe. A new recipe gets a new revision.
- State is strict schema-1 JSON. An unknown field or schema fails and does not
  change the file.
- Constructors are pure. Find the platform, the known folders and the
  environment, such as `ZDOTDIR` and `CODEX_HOME`, at the process boundary.
- Get Windows known folders from the system. Never guess them from `HOME`.
- Retained resources keep their dependencies. Do not disable the retained
  closure to stop a configuration leak.
- A native worker holds the provider lock until its child exits. The
  controller asks for the password one time, in the foreground. A background
  worker never reads a password.
- Verify each archive checksum before extraction. Archive generations use the
  operation ID. Never weaken a payload fingerprint to accept a vendor update.
- `ConfigDriver` uses live links on POSIX. On Windows it copies files and uses a
  junction for Neovim. `ProfileDriver` owns marked blocks and named JSON fields
  only. Outside edits, BOMs and line endings stay.
- The bootstrap verifies the pinned Go archive before each extraction. An
  unchanged checkout reuses its verified build. A fingerprint of the pin, the
  launcher and all files under `installer/` identifies that build. The launcher
  checks the hash of the cached binary before each launch.
- Update applies the reviewed pins of the checkout. It never gets the latest
  release.
- Check only reads. It never installs or repairs.
- The apply log is a record, not evidence.

## Sentinel

[Sentinel](https://github.com/luisgui1757/sentinel) holds the shared rules
and skills for coding agents.

- The `sentinel` capability needs the hidden `integration.sentinel-installer`
  resource. That resource needs `tool.git`.
- `installer/sentinel.go` holds the only pin, a full commit hash. Renovate
  tracks the Sentinel `main` branch and opens a pull request when it moves.
- Apply fetches that commit into `<state>/sentinel/checkout`. Git runs without
  system or global configuration. `HEAD` must equal the pin, and each file must
  equal the tree of the pin as raw bytes. Git index flags, local filters
  and replacement objects do not count.
- Then dotfiles runs the Sentinel installer of that checkout. Dotfiles does not
  copy or edit Sentinel files. Check, update and removal run Sentinel only from
  a verified checkout of the pin. Removal fetches the pin again when necessary.
- `<state>/sentinel/record.json` saves the intent before a change and the
  completion after it. Resume runs the installer again. The record also keeps
  the hosts and the five folder variables of the install for check and removal.
- The hosts are Codex, Claude Code, Copilot, Kiro and OpenCode. A host counts
  when its command or folder exists, or when the same selection installs it.
- Check runs `--check`. Exit 1 means drift. Any other failure is an error.
- `integration.sentinel` and the `skills.sentinel-*` resources exist only for
  the v1.0.0 handoff. Do not add behavior to them.

## Product configuration

- Neovim sets the Space leader before it loads Lazy. Only Rose Pine loads
  early, with priority 1000.
- Conform is the only format-on-save handler. `:WNF` saves one time without a
  format.
- Use `vim.uv`, `client:supports_method(...)` and `vim.lsp.log.set_level(...)`.
- render-markdown owns Markdown rendering. The Obsidian UI stays disabled. The
  Obsidian plugin starts only with an existing vault. Never create a vault.
- Do not install copies of the bundled Neovim parsers: `c`, `lua`, `markdown`,
  `markdown_inline`, `query`, `vim` and `vimdoc`.
- Neovim runs from private runtime generations. Mason uses the snapshot in
  `nvim/mason-registry/`. Never resolve the latest version.
- zsh: Tab opens fzf-tab over compinit, then autosuggestions. Keep Ctrl-R,
  Ctrl-T, Alt-C and the prefix history search on Up and Down. Enable vi mode
  before the key bindings. Never bind a bare Escape to `kill-whole-line`.
- No global binding uses a bare Alt-h, Alt-j, Alt-k, Alt-l or Alt-c. These
  chords belong to Neovim and fzf. AeroSpace uses Ctrl-Alt chords.
  `tests/aerospace/keymap_test.sh` checks this rule.
- PowerShell enables PSReadLine vi mode before the key bindings. Interactive
  setup stops for batch, encoded, redirected and CI hosts. Profiles never use
  `Invoke-Expression`.
- tmux uses Ctrl-B, windows and panes from 1, and the four pinned plugins. There
  is no TPM. Never call the continuum auto-start handler.
- Herdr runs on all three systems. tmux runs on macOS and Linux. Do not set the
  Herdr `default_shell` on Windows.
- LazyGit keeps the upstream keybindings. Starship git-status styles use
  `($style)`.
- The shell profile puts managed folders before system and personal `PATH`
  entries. Local hooks run after it.
- The `zsh-handoff` capability starts zsh from an interactive bash terminal. It
  never calls `chsh`.
- Claude Code gets `DISABLE_UPDATES=1`. Copilot gets `COPILOT_AUTO_UPDATE=false`.
  `integration.kiro` owns only `app.disableAutoupdates`.
- Kiro CLI is not on Windows, because Kiro ships it there only as an MSI.
- AeroSpace keeps its default layout. Do not pin applications to workspaces.
- The Mac keyboard shortcuts on Linux use xremap. In a terminal, Cmd never
  sends a control character. The only privilege is one udev rule.

## Conventions

- Go uses `gofmt` with tabs. Raw Go strings also pass EditorConfig. Lua uses
  StyLua with two spaces. Other files follow `.editorconfig`.
- `.gitattributes` keeps all text LF, also Windows scripts.
- Never edit a generated file. Change its source and run its generator in
  `scripts/`.
- Passive released files keep their recorded release bytes. Release inventory
  hashes protect them.
- A new editor plugin changes its spec and `nvim/lazy-lock.json` together.
- A new LSP, formatter or parser changes `tests/nvim/language_matrix.lua`, the
  plugin configuration and the strict smoke fixture.
- A new hidden native dependency goes into the catalog with its platform
  bindings.
- PowerShell helpers use singular nouns. Do not suppress `PSUseSingularNouns`.
- Add an abstraction only for duplication that exists now.
- When a key, command, configuration path or platform changes, change the
  README cheat sheet in the same change.

## Tests

- A bug fix needs a regression test that fails before the fix.
- Prefer process-boundary tests. A native lifecycle on a disposable runner is
  the highest proof. Cross-compilation is not native proof.
- The Neovim spec runners isolate the XDG data, state and cache folders.
- Shell tests use the `configureShellProfiles` output, not the frozen `shells/`
  files.
- Terminal tests use real PTY and ConPTY sessions.
- Git fixtures remove inherited `GIT_*` variables.
- The Go race suite has a 30-minute limit. Do not increase it to hide a hang.

## Security and releases

- The repository policy keeps CodeQL, private vulnerability reporting and
  immutable releases.
- The rulesets are in `.github/rulesets/`. `docs/security/branch-protection.md`
  describes the cutover. A checked-in policy does not prove the live settings.
- Releases follow `docs/RELEASING.md` and `release/manifest.json`. Release
  preparation commits only its rendered files.
- `.gitleaksignore` holds reviewed Gitleaks false positives as exact
  fingerprints.
- Archive and Linux Code DEB pins change only with review and native runtime
  proof. Renovate covers exactly its extraction inventory.
- The agent CLI pin workflow never merges. Its pull request must pass the
  native checks.
