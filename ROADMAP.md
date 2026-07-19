# Dotfiles Roadmap

Last audited: 2026-07-19 on branch `release/v0.3.0`.
Baseline: `main` at `bde9bc34724c52960f64d503c68b692d64240760`.

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
- `startup_spec.lua` now preclones locked plugin checkouts into its isolated
  cache, asserts that nvim-treesitter parser builds do not run during the
  benchmark, and uses a strict best-of-three warm measurement so local
  scheduler/filesystem outliers do not masquerade as production startup
  regressions.
- Live GitHub protection was applied and verified on 2026-06-18 with
  `scripts/apply-repo-safeguards.sh luisgui1757/dotfiles`.
- The active `Protect main: integrity` ruleset is active, strict, has no bypass
  actors, and requires `ubuntu`, `macos`, `windows`, `chezmoi-parity`,
  `chezmoi-parity-macos`, `chezmoi-parity-windows`,
  `nix flake check (ubuntu-24.04)`, `nix flake check (macos-26)`,
  `e2e containers / ubuntu-24.04`, `setup.sh / ubuntu-24.04`,
  `setup.sh / macos-26`, and `setup.ps1 / windows-2025`.
- Classic branch-protection fallback is strict and requires the same context
  set. GitHub returns those contexts in a different order than the ruleset API,
  so the apply-script verifier compares exact set membership.
- Post-fix audit hardening added regression coverage for POSIX uninstall
  dry-run immutability, mirrored chezmoi/Starship/tree-sitter pins,
  required-check list duplication, and the Windows Sandbox bootstrap boundary.
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

## Nix + Tooling Migration (2026-07, DONE)

An adversarial architecture review (2026-07-06) accepted, with required changes,
a staged migration toward Nix-owned POSIX **packages** (nix-darwin + Home Manager
+ declarative Homebrew on macOS; Home Manager standalone on Linux/WSL userland)
while **chezmoi stays the single owner of every dotfile target on every OS**, and
native Windows stays on `setup.ps1` + native package managers (Nix has no
supported native-Windows story; WSL2 only). Ruling highlights: exactly one owner
per path (Nix/Home Manager and chezmoi must never co-own a file); GUI /
TCC-sensitive apps (AeroSpace, WezTerm, Herdr) come from vendor channels
(casks / pinned artifacts), never nixpkgs; Herdr native Windows uses a pinned,
SHA-256-verified preview `.exe`; no `Invoke-Expression` / `curl|sh` / `irm|iex`
remote-eval installers.

Sequenced PRs (split for independent, revertable blast radius):

- **PR-1 `feat/ergonomics-core` - DONE (merged as #42).** No Nix, no vi-mode.
  Neovim `scrolloff = 16`; which-key.nvim (`<leader>?`, `:WhichKey`, VeryLazy); zoxide across
  zsh + PowerShell + both installers (cached, no-`Invoke-Expression` PowerShell
  init); `gh` + pinned `gh-dash` extension (`v4.25.2`) with a chezmoi-managed
  same-path config and Renovate / pin-consistency coverage.
- **PR-2 `feat/vi-mode` - DONE (merged as #43).** zsh (`bindkey -v`) +
  PSReadLine (`-EditMode Vi`) command-line vi-mode with a full re-bind matrix
  that preserves the invariant-13 fzf-tab / history stack: vi mode is enabled
  BEFORE the completion/keybinding region (zsh) and BEFORE the key handlers
  (PSReadLine) so the mode switch doesn't wipe them; Tab/Up/Down/Ctrl-R are bound
  on the right vi keymaps; explicit `KEYTIMEOUT` (chord-safe) + cursor-shape
  feedback; the psmux `OnIdle` re-apply also re-asserts the vi handlers. Recorded
  as CLAUDE.md invariant 21 and guarded by `tests/shell/zsh_vi_mode_test.sh` +
  new `Profile.Tests.ps1` vi-mode cases.
- **PR-3 `feat/wezterm` - DONE.** WezTerm on all OSes (brew cask / pinned `.deb` /
  `$Catalog`), Rose Pine + transparency + Hack Nerd Font parity, chezmoi-only
  config; not a Nix/nixpkgs GUI package.
- **PR-4 `feat/aerospace-herdr` - DONE.** AeroSpace (macOS tap cask,
  reserved-chord-safe keymap) + Herdr (macOS/Linux stable channels plus native
  Windows pinned preview binary). Herdr now consumes one chezmoi-managed,
  forced-dark built-in Rose Pine config on POSIX and from Windows' real roaming
  ApplicationData known folder. Stable `v0.7.4` and the July 16 Windows preview
  close the shifted indexed-workspace key bug; stale repo-owned Windows previews
  reconcile by exact path plus hash while unrelated binaries stay untouched.
- **PR-5 `feat/nix-skeleton` - DONE.** flake + committed `flake.lock` with ZERO
  ownership; `nix flake check` CI; disjointness test (Home Manager declares no
  file targets); Renovate `nix` manager.
- **PR-6 `feat/nix-darwin` - DONE (mixed-ownership correction 2026-07-13).**
  macOS host config: `darwinConfigurations` + `system.primaryUser`;
  nix-homebrew (`mutableTaps = true`, target-user-owned tap clones) + homebrew module
  (`cleanup = "none"`, no auto-update/upgrade); Home Manager **packages only**;
  public macOS setup applies it by default; `--update` gains an `owner=nix`
  status. Repeated setup also resolves the installed current-system rebuild
  command outside a pre-activation shell's stale `PATH` and treats exact
  `/etc/static` shell links plus retained backups as managed state.
- **PR-7 `feat/nix-linux` - DONE.** Home Manager standalone on Ubuntu/WSL userland;
  public Linux/WSL setup applies it by default; native/deferred install arms
  remain for artifacts and regression evidence (nvim last, for ABI reasons).
  The digest-pinned local Ubuntu 24.04 arm64 owner lifecycle passed at exact
  commit `51c5211b4b3dee4f0758533beac5e18345d668a1`: install, update, config
  uninstall, idempotent uninstall retry, reinstall, final update, 36/36 full
  validation, and preservation of every pre-existing native package. This is
  Linux runtime evidence, not physical-host or WSL proof.

### Mega-PR: `feat/platform-nix-tooling-mega` (2026-07-07)

PR-3 through PR-7 shipped together in ONE branch/PR
(`feat/platform-nix-tooling-mega`) because the terminal/tooling configs
(WezTerm, AeroSpace, Herdr) are the concrete packages the Nix layers then own,
and shipping them separately would mean the Nix casks/brews reference configs
that do not exist yet. The proving host is this macOS machine: Nix was installed
via the notarized Determinate package (signature + SHA-256 verified, no
remote-eval), so `flake.lock` and `nix flake check` are generated and verified
for real rather than hand-written.

Architecture ruling held for every phase: **chezmoi is the only owner of every
dotfile target on every OS**; Home Manager / nix-darwin own **packages only** (no
`home.file`, no `xdg.configFile`, no HM-generated shell/editor/terminal config);
native Windows stays **non-Nix** (`setup.ps1` + native package managers + chezmoi;
Nix touches WSL userland only, never `/mnt/c`). GUI / TCC-sensitive apps come
from vendor channels (casks / pinned artifacts), never nixpkgs.

Commit-by-commit status:

- **Commit 1 - docs/status guardrail — DONE.** PR-2 marked DONE; this section
  added; CLAUDE.md invariant 22 (chezmoi-only ownership, HM packages-only,
  native-Windows-non-Nix, no-remote-eval installer, Nix owner reporting in update
  mode); `tests/static/nix_architecture_test.sh` enforces the file-ownership
  disjointness and no-Windows-path rules statically.
- **Commit 2 - WezTerm — DONE.** Canonical `wezterm/wezterm.lua`, chezmoi
  mirror, same-path deploy on POSIX + Windows (WSL-gated off like ghostty), Rose
  Pine + transparency + Hack Nerd Font parity, shell = zsh (POSIX login shell) /
  pwsh.exe (Windows), no tmux auto-launch. Packaging: brew cask (macOS),
  Scoop/winget/choco (Windows), pinned official `.deb` with SHA-256 (native
  Ubuntu). Tests: Lua smoke (stubbed `require`), parity row, no-auto-launch static
  assertion, installer provenance, Windows `$Catalog` -> `$BinaryName`
  completeness, and required e2e PATH assertions on Linux + Windows. Runtime GUI
  / psmux-in-WezTerm visual verification remains manual-verification-pending in
  `tests/MANUAL.md`.
- **Commit 3 - AeroSpace + Herdr — DONE.** AeroSpace (macOS-only tap cask,
  `start-at-login`, reserved-chord-safe keymap avoiding Alt-h/j/k/l and Alt-c),
  `aerospace/aerospace.toml` + chezmoi mirror, TOML lint. Herdr (Homebrew /
  Linuxbrew formula, pinned native-Linux binary with provenance-backed update
  ownership, and pinned SHA-256-verified native-Windows preview `.exe` without
  `herdr.dev` remote eval). Herdr install failures now emit `FAIL:` and the
  Linux/macOS/Windows e2e gates assert the command. Its canonical config forces
  the built-in Rose Pine theme with automatic switching disabled on every OS.
  AeroSpace TCC /
  Accessibility and Herdr interactive-session behavior remain
  manual-verification-pending in `tests/MANUAL.md`.
  Hosted macOS additionally proves the real AeroSpace app and CLI binaries have
  one version/hash identity, but does not call that config-consumption proof:
  the pinned app waits for the user's Accessibility grant before parsing the
  managed file or starting its CLI server.
- **Commit 4 - Nix skeleton — DONE.** `flake.nix` + committed `flake.lock`
  (zero ownership), devShell + `checks`, `nix flake check` CI on Ubuntu + macOS,
  Renovate `nix` manager, disjointness static tests.
- **Commit 5 - nix-darwin + declarative Homebrew — DONE.**
  Architecture-specific `darwinConfigurations` with `system.primaryUser` and
  home resolved once by setup from the authoritative target account;
  nix-homebrew (target-user-owned tap clones,
  `autoMigrate = true`, `mutableTaps = true`, `trust.taps = [ "nikitabobko/tap" ]` for the
  AeroSpace cask); homebrew module (`autoUpdate = false`, `upgrade = false`,
  `cleanup = "none"`); casks WezTerm + AeroSpace; brews Herdr + selected CLI; Home
  Manager packages-only; default `setup.sh` applies `sudo darwin-rebuild switch`
  on macOS (prompted unless `--all`) with first-run bootstrap pinned to the
  locked nix-darwin rev plus `narHash`.
  Existing Homebrew installs are adopted with `autoMigrate = true`. The
  2026-07-13 correction made Homebrew explicitly mixed ownership: activation
  applies declared packages while Homebrew owns mutable tap clones as the target
  user. A scoped one-time transaction migrates only the three root-owned,
  non-Git snapshots created by the retired pinned-tap shape; unrelated user
  taps/packages are preserved, no whole-`Library/Taps` migration occurs, and CI
  uses the same contract. Transaction/recovery roots are siblings of
  `Library/Taps`, because descendants are live Homebrew tap candidates; setup
  auto-relocates the exact descendant artifacts emitted by the short-lived
  broken migration before retry. A checked-in owner-host lifecycle runner now
  covers install, update, config uninstall, reinstall, final update, package/tap
  preservation, and full validation. Its first real invocation also exposed a
  stale login-shell PATH: setup now directly re-adopts an existing canonical
  Nix profile binary when Homebrew path refresh occurs after the upstream
  already-sourced guard, rather than attempting a second Nix installation.
  The first real system activation remains manual-verification-pending in
  `tests/MANUAL.md`.
- **Commit 6 - Linux/WSL Home Manager packages-only — DONE.** HM standalone for
  native Linux + WSL userland (`homeConfigurations."<arch>-linux"`); packages
  only; default `setup.sh` applies Home Manager on Linux/WSL (prompted unless
  `--all`); split-host WSL preserved (writes only to `~/.nix-profile`, never
  `/mnt/c`); native install arms RETAINED for deferred/artifact provisioning and
  regression proof. **nvim + the tree-sitter CLI are intentionally deferred with proof:**
  they are ABI-coupled (nvim-treesitter `main` compiles parsers whose ABI must
  match nvim's built-in libtree-sitter; the CLI is pinned to v0.26.11 — invariant
  19), so a nix nvim/tree-sitter shadowing the pinned native binaries would risk
  the E5113 parser/ABI mismatch. They stay native until nvim + its parser
  toolchain can move into one ABI-matched Nix closure (follow-up). Excluded from
  `nix/home/common.nix` and asserted absent by `tests/nix/linux_home_test.sh`.
  Linux `clangd` is instead package-layer owned through `pkgs.clang-tools` on
  both supported architectures: Mason publishes no Linux arm64 clangd artifact,
  so its platform manifest excludes clangd on every Linux host rather than
  creating architecture-dependent ownership. Setup and validation invoke a
  shared checked Mason wrapper that turns command or missing-package failures
  into a nonzero Neovim exit.
  The guarded stale-`PATH` recovery is driven explicitly through the Linux Home
  Manager path, and a checked-in non-root Linux lifecycle runner mirrors the
  macOS install/update/config-uninstall/reinstall/update proof while rejecting
  removal of pre-existing native packages. The first real Home Manager
  activation on Linux/WSL remains
  manual-verification-pending in `tests/MANUAL.md`.
- **Commit 7 - setup/update ownership integration — DONE.** Unix update
  ownership recognizes Nix-owned tools: `install-deps.sh --update` resolves a
  tool's command source (or real path) and, when it lives under a Nix
  store/profile path, reports `skipped … owner=nix reason=managed by the Nix
  layer …` (reusing the documented vocabulary, not a new status word). No blanket
  `nix profile upgrade` / `nix-env -u` / `nix flake update`; no silent
  `flake.lock` rewrite (lock bumps are reviewed Renovate PRs; the layer is
  refreshed by the enforced POSIX `setup.sh` switch, with `--nix-darwin` /
  `--home-manager` kept as compatibility aliases). Existing per-manager ownership is
  preserved. Guarded by the new `nix-owned tool reports owner=nix` case in
  `install_deps_update_test.sh` + the blanket-upgrade guard in
  `nix_architecture_test.sh`.
- **Fable review remediation — DONE.** Accepted review fixes folded into this
  branch: sudo/pinned nix-darwin activation, flake.lock-pinned Home Manager
  bootstrap, packages-only static allowlist (`programs.home-manager` only,
  no `home.activation`), WezTerm Linux gate alignment with Ghostty, Ubuntu CI
  coverage for WezTerm/AeroSpace/Nix suites, Renovate managers for WezTerm +
  Herdr pins, Herdr direct-artifact provenance/update ownership, and setup-test
  robustness for bootstrap dispatch/sentinels/deferred-tool regexes.
- **Integrated final hardening — DONE.** Accepted Codex/Fable supply-chain fixes
  folded into this branch: Windows Scoop bootstrap now downloads
  `ScoopInstaller/Install` from a pinned commit, verifies SHA-256, then executes
  the local temp file while preserving elevated `-RunAsAdmin`; Windows Sandbox no
  longer remote-evals raw `main` and instead runs a mapped local checkout (the
  optional self-contained helper requires a full commit SHA and verifies
  `git rev-parse HEAD`); the supply-chain static scanner removed the old
  allowlists and proves the remaining cargo-binstall CI exception has immediate
  SHA-256 verification; Nix first-run refs include both locked rev and locked
  `narHash`; cheap guardrails now cover macOS vendor-tool e2e presence, Windows
  gh-dash config apply, deterministic zsh vi-mode/fzf-tab behavior, cursor-hook
  ordering before Starship, and the PowerShell psmux OnIdle no-EditMode reset
  invariant.
- **POSIX Nix enforcement hardening — DONE.** Public `setup.sh` now applies the
  Nix package layer by default on macOS/Linux/WSL before Phase 1 native/deferred
  installs; `--skip-deps` unconditionally skips that layer (even with
  compatibility aliases) and logs the skip; dry-run previews missing-Nix /
  unsupported-arch failures without aborting and Brew-less Darwin continues
  through every Brew-backed preview phase; the manual WSL greenfield harness
  installs Ubuntu's `nix-bin` package and enables flakes before invoking setup.
  The ultimate closure added authoritative account/home resolution,
  mixed-ownership tap preservation (superseding the earlier tap migration), and
  Home Manager session-vars startup. Apple Silicon is the only current Darwin
  contract; Intel evidence is retained in the append-only ledger as historical
  proof, not current support.
- **Pi CLI provisioning, theme, and multiline input — DONE.** Setup installs the Pi CLI on every OS as the
  pinned npm package `@earendil-works/pi-coding-agent@0.80.10` after checking npm
  `dist.integrity`; its three Pi companion modules are held to the same exact
  release. POSIX public setup gets Node 24 from the enforced Nix package
  layer; Windows uses the native Node LTS catalog path. Chezmoi deploys the
  audited Rose Pine theme plus the exact upstream `Shift+Enter` / `Ctrl+J`
  newline keybinding pair, and setup merges only the global `theme` selection;
  sessions, auth, providers, and every other `.pi/` preference remain local.
- **Gold-standard gap close — DONE (2026-07-09, PR #46).** Accepted
  install failures record and force nonzero setup/update exits; stdin/no-script-path
  setup fails closed with clone-first instructions instead of clone-and-reinvoke;
  the VS Build Tools bootstrapper must pass Authenticode Microsoft signer/chain
  verification before execution; the then-current WSL2 canary was split into a
  non-required scheduled/manual workflow; required-check sources align with `macos-26` and
  Nix contexts; Renovate custom-manager coverage and pin-consistency guards cover
  the current mirrored/manual-reviewed pin surface. PR #46 merged as
  `85375b2bdec9d3a998e8023a44b41d03a32f3eaa`; all twelve required checks passed,
  and checked-in/live required contexts were verified aligned afterward.

- **Ultimate gold-standard closure — IMPLEMENTED; exact hosted matrix passed,
  staged owner/manual proof remains
  (2026-07-10).** UGR-003 is
  implemented: Lazy and Plenary require a valid full lock identity and prove
  origin, exact HEAD, cleanliness, worktree usability, and required files before
  runtimepath mutation. Repair is locked, staged, verified, and rollback-safe;
  behavioral coverage replaces the former grep-only claim. UGR-002, UGR-011,
  UGR-012, UGR-013, and the POSIX half of UGR-015 are implemented with focused
  architecture, identity, session, dry-run, and rollback tests. The owner later
  retired the Intel product contract; WSL real-host proof remains explicitly
  pending.
  UGR-001 and UGR-014 are implemented:
  stable packaged/Preview/Canary/portable WT state is independently transactionally merged and
  recovered, while uninstall backup selection is filename-keyed and fails
  closed on malformed candidates. UGR-004 through UGR-009 are implemented:
  recoverable installs share the summary boundary; Pi and zsh executable
  payloads prove immutable identities before publication; Windows Tree-sitter
  uses exact compatible release artifacts; the required Microsoft `.deb`,
  gh-dash peeled commit, Sandbox Terminal zip, external Action refs, and desired
  Actions SHA-pinning safeguard are checked and documented. UGR-010 and the
  Windows half of UGR-015 now isolate native chezmoi status and apply configs to
  the real UserProfile, LocalApplicationData, Documents, and runtime `$PROFILE`
  paths. UGR-016 through UGR-019 and UGR-023 close the PowerShell invocation,
  checked Tree-sitter deletion, per-project clangd, Renovate inventory,
  Starship/Sentinel cleanup, diagnostic-identity, NUL-safe JSON, and structural
  Nix-ownership gaps. UGR-020 is deliberately PARTIAL: this cutover switches
  every checked-in required-check source to stable logical checks while
  retaining legacy producers for the still-live rules; the live owner apply
  happens only after the documented merged-main cache-free gate. Logical proof
  schema 2 records both the PR source head and GitHub's actually executed
  synthetic merge SHA instead of mislabeling the latter as the head. The apply
  script now completes and repeats the whole read-only boundary before its first mutation:
  exact branch/repo/main identity, clean sources, unique/exact legacy live
  policy, public visibility, GitHub-Actions app/workflow/event/run provenance,
  and cache-free E2E proof. It snapshots and rolls back the three changed
  resources on failure and retains a tested explicit recovery path. Recovery
  freezes every consumed snapshot file, requires the complete classic shape,
  rejects incomplete, altered, cross-stage, wrong-ruleset, bypass/condition, and
  full-classic-policy drift before any write, and publishes only the validated
  frozen bytes against policy from the manifest's still-live captured commit.
  Apply separately freezes every desired write from exact
  committed objects after the second capture, so a later checkout mutation
  cannot change publication. Capture directories clean on every exit and
  pre-mutation recovery snapshots are pruned. Probot Settings now owns
  repository-level settings only; a semantic YAML guard—not a presentation-
  specific regex—proves that every top-level `branches` form is absent and
  prevents a default-branch sync from racing the owner-run cutover.
  UGR-021 is PARTIAL until real WSL, redirected-Windows, cache-free scheduled or
  manual, and desktop runs exist. Historical Intel and current conventional
  Windows font-consumption lanes passed. The first PR run exposed and fixed two
  cross-host integration defects:
  the POSIX dependency-table zsh scan now passes the complete origin/commit/file
  identity, and handled Windows chezmoi drift no longer leaks a native exit code
  into the CI step. The next Ubuntu run also proved Renovate's optional
  `LOG_FILE` transport can be absent after exit 0; inventory validation now
  consumes JSON stdout and fails explicitly if no proof is emitted. The setup
  lanes then reproduced Homebrew's valid empty idempotent `shellenv` output and
  Lazy's need for proved default-branch metadata on a detached bootstrap; both
  boundaries now have behavioral regressions. Windows path proof now resolves
  junctions, and the generic Windows suite installs the exact verified chezmoi
  asset so native drift cases run with zero platform-dependent skips. The real
  Windows setup lane also proved an executable-stage filename must end in
  `.exe`; Tree-sitter publication now preserves that loader contract and its
  Pester oracle rejects any non-`.exe` validation path. Historical Intel CI explicitly
  uses full-SHA upstream-Nix installation after the real lane proved current
  Determinate Nix no longer supports x86_64-darwin hosts; the action's hidden
  last-release fallback is not treated as a platform contract. The next exact
  head (`7a446c31def84bdef6da11b23dab21f79ca13336`) supplied further runtime
  evidence: modern Ubuntu Bash exposed ineffective bare-`[[` tag assertions;
  Linux Home Manager installed its session file under the documented
  system-integrated profile; Intel reached a real x86_64 build but nix-darwin
  refused unpreserved `/etc/bashrc` and `/etc/zshrc`; and Apple Silicon exposed
  the valid nix-darwin-wrapper/native-Homebrew split. The fixes now state both
  zsh tag and commit in preview output, check every assertion portably, source
  all three official Home Manager profile locations, migrate the two system
  shell files transactionally, and prove Homebrew by prefix/repository. Windows
  apply failures now retain native stderr, and the target-free WT design lets
  the main source apply without non-portable absolute target selectors. Exact
  head `f4b63953f2f982702a685358b09e89bae2d78fdd` subsequently passed all three
  workflow runs: generic Ubuntu/macOS/Windows and parity, Nix Ubuntu/Apple
  Silicon/Intel, the Ubuntu container, public setup on Ubuntu/Apple
  Silicon/Intel/Windows, and all six stable logical proof jobs. Those Intel
  results predate the owner-directed retirement and are not a current support
  claim. No workflow
  definition is recorded as runtime proof; only those completed runs are.

- **Cache-free Tree-sitter restore/bootstrap boundary — BRANCH-HEAD PROOF
  PASSED; MERGED-MAIN CONFIRMATION PENDING.** The first manual cache-free
  merged-main run
  (`29096335827`, attempt 1, SHA `5e3e7c6d93c400d67f6160c6f8f09be56aac10d3`)
  proved that command-form Lazy `build = ":TSUpdate"` returned while its parser
  compilers were still running. Phase 4 then overlapped that unfinished work;
  the Apple Silicon lane installed only 98/99 languages and Pascal produced no
  captures. The locked Pascal parser/query pair passed a separate clean build,
  disproving deterministic incompatibility. The build hook now uses
  nvim-treesitter's waitable update task, serializes work, and fails unless the
  task completes successfully before Lazy restore returns. Behavioral tests
  prove both the wait boundary and fail-closed completion. UGR-021 and the
  stage-2 UGR-020 safeguard cutover remain PARTIAL until this repair merges and
  the cache-free logical macOS proof passes on merged `main`. Attempt 2 on the
  same unrepaired SHA passed Apple Silicon but failed Intel when the original
  CMake fixture's neocmake client did not attach within 45 seconds (the later
  formatter CMake fixture did attach); that retry is additional failed evidence,
  not repaired-head proof. The first branch-head cache-free run
  (`29100106370`, SHA `1f03199f9d420e534bfade544ae7d74f1cfb002a`)
  then disproved the narrower build-hook-only repair: Apple Silicon passed, but
  Ubuntu lost Astro captures and Intel lost GraphQL captures. Their logs showed
  Lazy restore itself loaded plugin config and launched the interactive async
  declared-parser install before Phase 4. Ordinary headless config loads now
  skip that path; only a real UI or the explicit synchronous Phase 4 flag may
  install. The new behavioral test failed on the branch head and passes after
  this second root-cause fix. Exact behavior head
  `e5cf3e23299cbb42a157c307f2a7259979fcada0` then passed cache-free run
  `29103732329`: Ubuntu container, public Ubuntu, Apple Silicon, Intel, native
  Windows, and all four setup logical proofs were green. UGR-021 remains
  PARTIAL because merged-main run `29114125798` exposed the separate CMake LSP
  project-isolation defect now repaired on this branch, and because the manual
  environments remain outstanding. The checked-in stable-context cutover is in
  this PR. Repaired PR head
  `4dbdb959674f5a062cffe44daae242318f4c1b67` passed generic/parity run
  `29140112029`, Nix run `29140112035`, and cached E2E run `29140112030`: all
  12 live-required contexts and all six stable logical contexts were green.
  The six schema-2 artifacts bind that source head to the executed synthetic
  merge commit and passed a separate leak scan. This is exact repaired-head
  proof, not cache-free merged-main proof; owner live apply still waits for the
  latter.

- **Cache-free Ghostty artifact provenance — BRANCH-HEAD PROOF PASSED;
  MERGED-MAIN CONFIRMATION PENDING.** PR #48's first Ubuntu container job
  (`29100012131` /
  `86386173483`) reproduced a second cache-free dependency: the checksum-pinned
  `mkasberg/ghostty-ubuntu` script queried unauthenticated mutable
  `releases/latest`, then exited under `set -e` when no asset URL was returned.
  More importantly, any successful lookup would have installed unchecked `.deb`
  bytes as root. The script path is removed. Setup now selects one exact
  distro/architecture release asset, verifies one of six independently reviewed
  SHA-256 values and exact package metadata before privileged apt, validates the
  installed version/command, cleans staging on every path, and prints recovery
  guidance after state-changing failures. Success/failure/mapping tests and the
  generalized privileged-package scanner bind the contract. Exact behavior
  head `e5cf3e23299cbb42a157c307f2a7259979fcada0` passed both the cached PR
  Ubuntu container (`29103728188` / `86398980438`) and cache-free Ubuntu
  container (`29103732329` / `86399025475`). The original red job remains
  defect evidence rather than being waived.

- **Cold-cache CMake LSP proof isolation — BRANCH-HEAD HOSTED PROOF PASSED;
  MERGED-MAIN PROOF PENDING.** Merged-main cache-free run `29114125798` passed Ubuntu container,
  public Ubuntu, Intel, and Windows, but Apple Silicon job `86433246367`
  failed because the first neocmakelsp attach probe opened inside the shared
  repository fixture tree. The later isolated formatter/CMake project attached
  and accepted gersemi output in the same process. Strict smoke now gives every
  initial LSP attach probe its own minimal project root; the real server,
  production config, attach timeout, formatter, diagnostics, and capture gates
  remain unchanged. Exact behavior head
  `f097995b49a2189db327903a20743e7cb69ba665` passed cache-free run
  `29120109175`: all four current producers and all four stable setup logical
  proofs were green. Later PR-head run `29180481941` exposed a second redundant
  neocmakelsp lifecycle in the formatter gate: the first isolated CMake client
  attached, then the formatter-only restart timed out. The smoke now formats,
  saves, and checks diagnostics on the already-attached isolated client; three
  repeated strict Apple-Silicon runs passed all 257 checks without increasing
  the attach timeout. Exact repaired behavior head
  `d744948cdccc51f3d79e45aa78f82c46445df0c6` then passed PR E2E run
  `29181215803`: all four producers and logical proofs were green, including
  Apple Silicon's real neocmake plus gersemi gate. A
  post-merge run on the resulting `main` SHA remains the gate before
  owner-applied safeguards.

- **Exact-head runtime dependency follow-up — PASSED.** Head
  `0c853d066362602f14dc251a6d3fbf3980102048`
  reached the real two-project clangd spec on Ubuntu and failed closed because
  the generic test lane had never installed `clangd`; all preceding Neovim
  specs passed. The lane now installs Ubuntu's distro clangd and a static
  contract test binds that provisioning to the real-client spec. Exact head
  `f4b63953f2f982702a685358b09e89bae2d78fdd`, run `29092384006`, job
  `86360593114`, passed the real two-project one-session spec (1/1, 0 failed).

- **Exact-head Home Manager session-path follow-up — PASSED ON NATIVE LINUX;
  WSL PENDING.** Head `0c853d066362602f14dc251a6d3fbf3980102048`
  proved that merely finding `hm-session-vars.sh` was insufficient: the pinned
  standalone configuration had an empty `home.sessionPath`, so its generated
  file could not put profile-owned `rg` on a clean shell PATH. Linux Home
  Manager now exports its evaluated `home.profileDirectory/bin` through
  `home.sessionPath`; both architecture evaluations bind the exact value. Exact
  head `f4b63953f2f982702a685358b09e89bae2d78fdd`, run `29092384014`, job
  `86360593139`, passed the native-Linux clean login proof. WSL is not inferred.

- **Exact-head native-Linux login-shell oracle — PASSED.** Head
  `28006783a5112bfa3af3b0deb2f59fbf9f457a4e`
  completed Home Manager and all six setup phases, then failed before session
  state was exercised: fresh Ubuntu reported zsh missing, setup installed and
  selected `/home/linuxbrew/.linuxbrew/bin/zsh`, but the assertion invoked the
  nonexistent `/usr/bin/zsh` and discarded stderr. The proof now resolves one
  effective-account record, requires its shell to be executable zsh, executes
  that exact login shell from a caller-empty environment, and prints captured
  diagnostics on failure. Exact head
  `f4b63953f2f982702a685358b09e89bae2d78fdd`, run `29092384014`, job
  `86360593139`, completed all six phases and passed that post-install oracle.

- **Exact-head native Windows setup — PASSED.** Head
  `f4b63953f2f982702a685358b09e89bae2d78fdd`, run `29092384014`, job
  `86360593122`, completed
  all six public setup phases and the required post-install checks on
  `windows-2025`. It installed and validated the exact Tree-sitter `0.26.10`
  release artifact, installed Hack Nerd Font files and proved registry
  registration, validated Pi `0.80.3`, restored plugins/parsers/Mason, and
  passed the strict 257-check LSP/parser/formatter smoke. This is real native
  Windows hosted evidence, not redirected-known-folder, divergent Windows
  Terminal, uninstall-restoration, or desktop visual proof.

- **Apple-Silicon-only macOS contract — DONE by owner direction (2026-07-12).**
  The flake, setup selector, CI matrices, pinned installers, migration tool,
  tests, and current public documentation ship only the Apple Silicon path.
  Every other macOS architecture fails through the generic platform boundary
  before Nix/Homebrew activation. Historical host results remain in append-only
  evidence only; they do not define a product path.

- **Sentinel rename cutover — DONE (2026-07-12).** Both setup entry points now
  clone `luisgui1757/sentinel` into Sentinel-only cache paths, detach at exact
  commit `ecafffa858666343c1639f996d177f460163e93e`, validate `VERSION=0.1.2`,
  and run the renamed Bash installer/check contract. The published `v0.1.2` tag
  predates the rename, so it is not used as false identity evidence. Shell,
  PowerShell, pin-consistency, and repository-wide naming tests guard the exact
  commit and require zero pre-rename product-name residue in paths or content.

- **Hosted WSL2 canary retirement — DONE (2026-07-10).** The only scheduled
  run (`29072773410` / `86297630493`) and a manual rerun (`29114215045` /
  `86433541987`) both reached real WSL2 but stalled before setup evidence and
  required cancellation. GitHub documents nested virtualization on hosted
  runners as technically possible but not officially supported. The optional
  workflow is removed instead of emitting an unreliable signal or fake
  Linux-with-WSL-environment proof. WSL product support and the real throwaway
  WSL/manual split-host harnesses remain; runtime proof is explicitly manual.

### P2 Follow-up: Secondary Supply-chain Hardening

Reconciled by the 2026-07-10 ultimate closure branch; each item is marked DONE
only with its implementation, tests, and documentation:

1. **Pi CLI verified tarball install — DONE.** POSIX and Windows use `npm pack`,
   require pack metadata and actual coding-agent tarball bytes to match the
   mirrored SRI, install it with exact same-release Pi companions, and clean
   temp state on every exit.
2. **zsh plugin quarantine-on-mismatch — DONE.** The shared serialized
   publisher neutralizes unproved executable payloads, verifies a sibling exact
   checkout, publishes atomically, preserves unsafe quarantines, and lets bare
   chezmoi self-heal legitimate pin changes.
3. **Windows Tree-sitter compatibility pin — DONE.** Mutable Scoop/npm fallback
   ownership is removed; exact `0.26.11` compatibility and the reviewed
   x64/arm64/x86 release zip hashes gate transactional publication.
4. **gh-dash tag provenance — DONE.** Tag `v4.25.2` is verified through its
   annotated object to peeled commit
   `a613ef744c99ef8d8ead33467813c6ee6086af52`, and installation pins that commit.
5. **Ubuntu CI Microsoft repo package — DONE.** Ubuntu 24.04 downloads the exact
   configuration `.deb`, checks reviewed SHA-256
   `c13f01ac7c3001b51a9281d40dde666db5e037e05512840c319832f7852bfec4`,
   then invokes `sudo dpkg`; the general scanner self-tests this ordering.

Startup SIGTERM harness changes were not made in this branch because the failure
was not reproduced; do not rewrite that test without a deterministic
reproduction and diagnostics.

Each commit flips its own status to DONE in the same commit that lands it, per
the repo's doc-discipline rule.

## P0 - Total Update Ownership Model

Status: shipped on 2026-06-30 in the combined terminal/Markdown/update PR.

The Unix update system now uses per-tool proven ownership instead of one active
package manager for the whole catalog. The install path still chooses a
preferred package manager for new installs, but the update path resolves each
present command from the executable source to a supported owner before taking
action. This covers real machines where Homebrew/Linuxbrew, native Linux
package managers, repo-pinned direct artifacts, and OS-vendor tools coexist.

The ubiquitous uncompromised canonical gold-standard is **full release
reconciliation followed by per-tool proven update ownership**:

- `--update` first runs the same idempotent install/migration reconciliation as
  `--all`, then enters the dependency drift-edge refresh. It remains neither a
  repo fetch nor machine-wide package maintenance.
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
  executable. The catalog formula remains the install default; an active
  versioned formula is instead resolved from its Cellar target and verified
  receipt before update. Formula-list presence alone cannot claim `/usr/bin`, another
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
  manual `export PATH=...` step. Install and update also ask Homebrew to
  idempotently relink its completion surface, preventing stale `_brew` links
  after tap/repository migration.
- Windows manager-owned packages now report `current` without running a mutating
  package update when Scoop `status`, winget `list --upgrade-available`, or
  Chocolatey `outdated --limit-output` has no exact package row; failed
  availability probes are recorded as update failures, not successful no-ops.
  Scoop availability uses structured status fields: exact `Name`, non-empty
  `Latest Version`, and empty `Info`/`Missing Dependencies`.
- Regression coverage lives in `tests/shell/install_deps_update_test.sh` for
  mixed Linuxbrew/apt dispatch, Homebrew `current`, shadowed Homebrew tools,
  versioned-formula ownership, Brew-prefix contradictions, Brew-prefix symlink escapes, external shadow
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
     `hyperfine`, `taplo`, `yamllint`, and similar).
   - The repo manages Homebrew shellenv, completion-link reconciliation, and any
     required PATH adoption. There should be no hidden manual repair or
     `export PATH=...` step.
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
    retrofitted into a legacy managed block without dropping user content;
    install/update completion-link reconciliation is idempotent and fail-closed.
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
- Do not make `--update` run Lazy's upstream update or rewrite
  `lazy-lock.json`; full reconciliation restores the reviewed lock instead.
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

Status: fixed; final Scoop/Sandbox mutable trust-root removal folded on
2026-07-07 in `feat/platform-nix-tooling-mega`.

Evidence:

- `install-deps.sh` no longer runs the Homebrew `HEAD` installer; it downloads
  a pinned Homebrew installer commit and verifies SHA-256 before execution.
- Native-Linux `install-deps.sh` no longer runs `get.chezmoi.io`; it downloads
  the pinned chezmoi GitHub release archive and verifies SHA-256 before
  installing.
- `install-deps.sh` no longer pipes the Starship installer into `sh`; native
  Linux/WSL without brew uses pinned Starship release archives with SHA-256
  verification, while Alpine uses its native package.
- `install-deps.ps1` no longer downloads `https://get.scoop.sh`; it downloads
  `ScoopInstaller/Install` at a full commit SHA, verifies the installer
  SHA-256, then executes the local temp file.
- `tests/greenfield/windows-sandbox.wsb` no longer downloads raw `main` or
  executes `[scriptblock]::Create(...)`; it maps a local checkout and runs the
  checked-out `sandbox-run.ps1`.
- `tests/greenfield/sandbox-bootstrap.ps1` no longer downloads branch ZIPs; the
  optional self-contained path requires a full commit SHA, fetches that object,
  verifies `git rev-parse HEAD`, then executes the checked-out local script.
- CI no longer runs the Starship installer script as fallback; it downloads the
  pinned Starship release tarball and verifies SHA-256 before extraction.
- CI no longer runs `get.chezmoi.io`; POSIX parity jobs use
  `scripts/install-pinned-chezmoi.sh`, and Windows parity downloads the pinned
  chezmoi zip and verifies SHA-256 before extraction.
- Recommended setup docs no longer use raw `curl | bash`/`iwr` execution of the
  current default branch; they use `git clone` plus local `setup`.
- The repo already has stronger patterns for other downloads, for example
  exact Ghostty `.deb` verification in `install-deps.sh`, Hack font verification in
  `install-deps.ps1`, and Windows Terminal portable verification in
  `install-deps.ps1`.

Risk:

Resolved: first-run executable downloads now use pinned artifacts or pinned
installer scripts with SHA-256 verification before execution. The static scanner
fails on future mutable remote-eval or unverified downloaded-script execution.

Resolution:

1. Repository policy now requires direct network executables to be pinned and
   verified before execution; any static allowlist entry must itself be a
   pinned+verified case with the verification proved by the test.
2. Native Linux chezmoi moved from `get.chezmoi.io` script execution to a
   SHA-256-verified release artifact.
3. The Starship curl installer fallback was replaced by SHA-256-verified
   release artifacts on native Linux/WSL without brew.
4. `tests/static/supply_chain_remote_execution_test.sh` rejects new
   `curl | sh`, `sh -c "$(curl ...)"`, raw `/tmp/*install*.sh` execution,
   `Invoke-Expression` / `iex`, PowerShell `scriptblock::Create` execution, and
   downloaded PowerShell script execution without an intervening SHA-256 check.
5. CI Starship, tree-sitter CLI, and chezmoi installs now use SHA-256-verified
   release artifacts.
6. The previous mutable Scoop and Windows Sandbox trust roots were removed from
   the static allowlist; only the pinned+verified cargo-binstall CI script
   remains allowlisted, with immediate SHA-256 verification proved by the test.

## P1 - Greenfield Proof

### 3. Windows Sandbox greenfield path still points at the retired pilot branch

Status: done.

Evidence:

- `tests/greenfield/windows-sandbox.wsb` now maps a local checkout and executes
  checked-out local repo code; it does not download raw `main`.
- `tests/greenfield/sandbox-bootstrap.ps1` has no default branch; the optional
  self-contained helper requires a full commit SHA and verifies
  `git rev-parse HEAD` before executing the checked-out local script.
- `tests/greenfield/README.md` and `tests/greenfield/RUNBOOK.md` document the
  explicit PR/branch/commit selection path.
- `tests/static/stale_greenfield_refs_test.sh` fails if the retired pilot
  branch name appears outside archived historical docs.

Risk:

Resolved: advertised clean-machine proofs now test an intentionally selected
local checkout or full commit SHA, never an implicit mutable branch.

Canonical solution:

1. DONE - Removed implicit mutable `main` execution from the Sandbox path.
2. DONE - Add an explicit full commit SHA / local checkout selection path for PR
   validation.
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
`~/.local/share/dotfiles/zsh-plugins`; install-deps, runtime, the checked
publisher, uninstall, greenfield validation, container/WSL validation, and
parity fixtures now use or assert that root.

Evidence:

- `shells/zshrc:69` loads zsh plugins from the fixed
  `$HOME/.local/share/dotfiles/zsh-plugins` root.
- `install-deps.sh` installs zsh plugins into the fixed
  `$HOME/.local/share/dotfiles/zsh-plugins` root.
- `home/.chezmoiexternal.toml.tmpl` deliberately exposes no executable
  `git-repo` target.
- `home/.chezmoiscripts/run_onchange_after_20-ensure-zsh-plugin-pins.sh.tmpl`
  invokes the canonical staged publisher against the fixed `.local/share` path
  whenever pins or the embedded publisher identity change.
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

## P1 - v0.1.0 Release Upgrade

### 9. The published in-place upgrade crossed the config backup boundary

Status: implementation done; v0.2.0 published; v0.3.0 release evidence in progress.

Evidence:

- Exact v0.1.0 POSIX config uses checkout-backed chezmoi symlinks. Updating that
  checkout changed live bytes before current setup could back them up.
- `scripts/upgrade-v0.1.0.sh` and `.ps1` now require clean, separate official
  annotated v0.1.0/v0.3.0 checkouts and retain v0.1.0 until acceptance.
- The public `setup.sh --all` / `setup.ps1 -All` entrypoints now discover exact
  live v0.1.0 ownership, invoke and verify those transactions, resume a pending
  applied recovery, accept under the explicit non-interactive all-mode
  contract, retain recovery evidence, and continue full setup. POSIX setup also
  bootstraps the verified Nix prerequisite itself. Update mode runs the same
  reconciliation before its scoped refresh, with upgrade as an alias.
- POSIX apply and rollback consume digest-bound, read-only exact-commit trees
  inside private recovery; post-validation checkout drift cannot change a
  package/config write.
- Windows apply, readback, uninstall, and rollback consume digest-bound
  exact-commit trees beneath the protected recovery ACL; retained-checkout drift
  cannot change a config or Terminal transaction write.
- POSIX migration applies only Nix plus config files/links; chezmoi run scripts,
  native/deferred packages, editor caches, and agent policy stay outside the
  reversible transaction. Windows applies only config, known-folder overlays,
  and independently recoverable Terminal state; dependencies, Neovim caches,
  agent policy, and chezmoi run scripts are explicitly skipped.
- The exact-tag POSIX harness runs the real setup backup/config path in Linux
  and Apple-Silicon/nix-darwin fixture modes. It proves dirty/in-place
  rejection, read-only preflight, post-activation failure, TERM handling,
  frozen-source publication, altered-recovery rejection, automatic
  package/config rollback, retry, and
  acceptance. Windows Pester proves frozen release-tree validation, complete Terminal recovery,
  all-target concurrency rejection, known-folder boundary validation, private
  ACL policy, and provider-boundary verification.
- `docs/UPGRADING.md` and `docs/releases/v0.3.0.md` define the supported
  per-platform commands, Apple-Silicon-only macOS boundary, WSL ordering, Nix
  provenance, and release evidence gate.

Canonical solution:

1. DONE - Remove every moving-branch/in-place v0.1.0 instruction.
2. DONE - Add exact-tag side-by-side preflight, apply, rollback, and acceptance.
3. DONE - Separate reversible Nix/config migration from additive native tools.
4. DONE - Pin the exact peeled v0.1.0 fixture and failure boundaries in CI.
5. DONE - Make setup the sole normal install/migration/update orchestrator while
   preserving exact-tag acquisition and the existing recovery transaction.
6. PENDING LIVE - Record Apple Silicon owner-host, real WSL2, redirected
   Windows, divergent stable packaged/Preview/Canary/portable Terminal, and exact tagged v0.3.0 release runs before
   publication.

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
