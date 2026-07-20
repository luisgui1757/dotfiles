# Upgrading dotfiles releases

## One public entry point

After cloning an exact annotated release, setup is the only normal user-facing
command:

```bash
./setup.sh --all
```

```powershell
.\setup.ps1 -All
```

The same command handles a clean machine, an idempotent rerun, and the supported
v0.1.0 migration. `--update` / `-Update` runs that complete reconciliation and
then refreshes the proven drift-tolerant dependency/Mason edge. `--upgrade` /
`-Upgrade` is an alias.

Setup never fetches a moving branch or mutates an old release checkout. Git
acquisition remains explicit: clone the exact next annotated release beside the
old checkout, then run setup from the new checkout.

`./setup.sh --allow-unreleased` is a separate greenfield/already-current-release field-test
lane. It authorizes only a clean exact current branch head of the official
repository for POSIX prerequisite bootstrap. It is not an upgrade authority and
must not be used to migrate a live v0.1.0 checkout; the versioned migration
tools remain exact-tag-only.

## v0.1.0 to v0.4.0

`v0.1.0` is already a chezmoi release. On POSIX, its managed files are live
symlinks into the checkout. Do **not** run `git pull`, switch that checkout to a
new revision, or run an upgrade from `main`: doing so can change live config
before recovery exists.

These commands become user-facing only after the annotated `v0.4.0` release
exists. Before then, remain on `v0.1.0`.

### Common preparation

Keep the checkout that currently owns v0.1.0 and clone v0.4.0 beside it:

```bash
git clone --branch v0.4.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles-v0.4.0
cd ~/dotfiles-v0.4.0
```

```powershell
git clone --branch v0.4.0 --single-branch `
  https://github.com/luisgui1757/dotfiles.git "$HOME\dotfiles-v0.4.0"
Set-Location "$HOME\dotfiles-v0.4.0"
```

Do not stash, delete, or overwrite local changes to make migration pass. Setup
requires the exact old and new official release identities and refuses to guess
which user bytes are authoritative.

Setup normally discovers v0.1.0 from its live Neovim/shell config ownership. If
the old checkout uses an unusual path and cannot be discovered, identify it for
the same setup invocation:

```bash
DOTFILES_V0_1_CHECKOUT=/actual/path/to/v0.1.0 ./setup.sh --all
```

```powershell
$env:DOTFILES_V0_1_CHECKOUT = 'D:\actual\path\to\v0.1.0'
.\setup.ps1 -All
Remove-Item Env:DOTFILES_V0_1_CHECKOUT
```

The override is not trusted blindly; setup still requires the exact annotated
v0.1.0 tag object and peeled commit, and the migration performs its complete
official-remote, clean-tree, target-identity, and historical-config preflight.

### Apple Silicon macOS, Linux x86_64, and Linux aarch64

Run:

```bash
./setup.sh --all
```

That one invocation:

1. resolves and validates the real target account and home;
2. installs the release-pinned, SHA-256-verified Nix prerequisite when missing
   and activates it in the current process;
3. detects exact live v0.1.0 ownership or a resumable applied recovery;
4. runs the side-by-side digest-bound Nix/config transaction;
5. automatically rolls back v0.1.0 on migration failure or interruption;
6. verifies and accepts the reversible core under the explicit non-interactive
   `--all` contract;
7. retains the private recovery directory for evidence;
8. completes native/deferred dependencies, config repointing, locked Neovim
   restore, parsers, Mason, and Sentinel.

If an earlier migration reached `applied` but setup stopped before acceptance,
rerunning the same command resumes at validated acceptance. A recovery in
`prepared`, `applying`, `rolling-back`, or `recovery-required` fails closed and
prints its exact rollback command instead of starting another transaction.
An unfinished v0.2.0 or v0.3.0 recovery must first be accepted or rolled back
from its retained exact release checkout. v0.4.0 setup detects both older active
namespaces and refuses to start or resume a second release transaction around
either one.

After success, open a new login shell and verify:

```bash
nix store info
command -v rg fd fzf jq lazygit node starship zoxide nvim
chezmoi --source ~/dotfiles-v0.4.0/home --destination "$HOME" \
  verify --include files,symlinks
```

Keep the old checkout and printed recovery directory until personal application
data has been checked. Automatic rollback authority ends only after the
transaction's config/package verification passes and setup accepts it; the
recovery directory remains private evidence.

No macOS path exists outside Apple Silicon.

### Native Windows

Windows remains non-Nix. Enable Developer Mode, open a normal PowerShell, and
run:

```powershell
.\setup.ps1 -All
```

Setup discovers exact v0.1.0 through the live Neovim link (or the validated
override), then runs the digest-bound config/known-folder/Terminal transaction,
requires the expected post-migration state for stable packaged, Preview
packaged, and portable Windows Terminal, accepts the verified core, retains the
protected recovery folder, and completes packages, config repointing, Neovim,
and Sentinel.

The same command covers conventional, redirected, OneDrive, and alternate-drive
known folders because UserProfile, LocalApplicationData, ApplicationData,
Documents, and the runtime PowerShell profile are resolved independently.

After success, open Neovim, lazygit, PowerShell, and every installed Windows
Terminal variant. Keep the old checkout and protected recovery folder until
those checks pass.

### WSL2 split-host upgrade

WSL still has two independent owners and therefore two setup invocations:

1. Clone v0.4.0 on Windows and run `.\setup.ps1 -All` for host Terminal, font,
   clipboard, and Windows tools.
2. Clone v0.4.0 separately inside the Linux home—never under `/mnt/c`—and run
   `./setup.sh --all` for the guest Nix/config/tool stack.
3. Run `tests/wsl/e2e.sh` in the guest and verify host Windows Terminal,
   `win32yank`, font, and PowerShell behavior.

Each setup invocation verifies and accepts its own platform transaction. A
failed guest migration rolls its guest state back automatically without
invalidating an already-verified Windows host installation.

## Updating an installed release

From the exact release checkout:

```bash
./setup.sh --update
# alias: ./setup.sh --upgrade
```

```powershell
.\setup.ps1 -Update
# alias: .\setup.ps1 -Upgrade
```

Update first runs the same install/migration/idempotent reconciliation as all
mode. It then performs only scoped updates for present tools whose package or
direct-artifact ownership is proven, plus synchronous Mason updates. It never
runs a blanket package-manager upgrade, `git pull`, `nix flake update`, or
`:Lazy update`, and it never rewrites repository lockfiles. On macOS the retry
is safe in the terminal that performed first activation: setup resolves the
installed current-system `darwin-rebuild` outside stale `PATH` and recognizes
nix-darwin's `/etc/static` shell links plus retained backups as managed state.
Legacy Homebrew tap migration state is also retry-safe: setup keeps transaction
and failed-output roots beside `Library/Taps`, where Homebrew cannot enumerate
them as additional taps, and automatically relocates the exact in-tree recovery
names created by the broken predecessor. Do not manually untap or delete those
artifacts before retrying setup.
If the same login shell already sourced the Nix daemon profile and a later
Homebrew `path_helper` refresh removed Nix from `PATH`, setup re-adopts the
canonical daemon/user profile binary directly. Do not reinstall Nix or unset
the upstream profile guard manually.

To move to a newer dotfiles release, clone that exact annotated tag beside the
current checkout and run its setup update command. The new checkout, not Git
mutation of the old one, is the release boundary.

## Operator recovery commands

The standalone migration commands remain supported for diagnosis, deliberate
manual acceptance, and recovery. They are not required for the normal path:

```bash
./scripts/upgrade-v0.1.0.sh --preflight-only /path/to/v0.1.0
./scripts/upgrade-v0.1.0.sh --apply /path/to/v0.1.0
/exact/recovery/upgrade-v0.1.0.sh --rollback /exact/recovery
/exact/recovery/upgrade-v0.1.0.sh --accept /exact/recovery
```

```powershell
.\scripts\upgrade-v0.1.0.ps1 -PreflightOnly -OldCheckout 'C:\path\to\v0.1.0'
.\scripts\upgrade-v0.1.0.ps1 -Apply -OldCheckout 'C:\path\to\v0.1.0'
pwsh -NoProfile -File 'C:\exact\recovery\upgrade-v0.1.0.ps1' -Rollback 'C:\exact\recovery'
pwsh -NoProfile -File 'C:\exact\recovery\upgrade-v0.1.0.ps1' -Accept 'C:\exact\recovery'
```

## v0.4.0 release-candidate evidence status

The v0.4.0 release is not yet published. Its release authority is prepared on
top of exact `main` commit `2e40241f3f29e09d8f12ea309efeef4db800ecde`;
publication remains gated on:

- [ ] the reviewed release-preparation pull request merged to `main` with all
  required checks passing;
- [ ] an annotated `v0.4.0` tag whose tag object and peeled commit match the
  exact merged release-preparation commit and the official remote;
- [ ] full local and hosted gates, deterministic exact-v0.1.0 migration
  fixtures, Windows Pester coverage, and a redacted scan across
  `v0.3.0..v0.4.0` plus all downloaded logical proofs;
- [ ] a cache-free hosted release run whose POSIX lanes report the exact
  immutable `v0.4.0` tag identity;
- [ ] immutable/latest GitHub release readback matching the prepared notes.

The unchecked real WSL, redirected-Windows, divergent Windows Terminal,
physical-Linux, Apple-Silicon owner-host, and visual rows in `tests/MANUAL.md`
remain explicit residual gaps rather than implied passes.

## Historical v0.3.0 release evidence

v0.3.0 was published on 2026-07-19 under explicit owner authorization. The
deterministic publication gates passed; the real-environment rows below remain
open and are not implied by publication:

- [x] pull request #61 merged the reviewed release-preparation tree to exact
  `main` commit `c8507312153620b9b30fe2c84980c62bccb3b25a` with all required
  checks passing;
- [x] annotated tag object `473f675e863640484d4d11349bf69d01def12c43`
  peels to that exact commit and matches immutable/latest GitHub release
  `356273955`;
- [x] full local `make ci`, the deterministic exact-v0.1.0 POSIX fixture,
  Windows Pester coverage, and Gitleaks 8.30.1 scans across all 8 commits in
  `v0.2.0..v0.3.0` plus the 4 downloaded logical proofs passed;
- [x] cache-free hosted run
  [`29676087505`](https://github.com/luisgui1757/dotfiles/actions/runs/29676087505)
  passed Ubuntu, Apple Silicon macOS, Windows, the Linux container, and all four
  stable logical proofs; both POSIX lanes verified the exact immutable v0.3.0
  tag path;
- [x] GitHub release `356273955` is immutable, latest, non-draft, and
  non-prerelease, and its body matches the prepared user-facing notes.

Initial cache-free run
[`29675684683`](https://github.com/luisgui1757/dotfiles/actions/runs/29675684683)
is retained as failed evidence: macOS setup completed, but its neocmake LSP
probe missed the 45-second attach boundary. The identical exact-head PR job had
passed that probe, and the one permitted fresh full retry above passed it in
39 seconds; no failed or cancelled artifact was promoted into release proof.

The unchecked real WSL, redirected-Windows, divergent Windows Terminal,
physical-Linux, Apple-Silicon owner-host, and visual rows in `tests/MANUAL.md`
remain explicit residual gaps rather than implied passes.

## Historical v0.2.0 release evidence

v0.2.0 was published on 2026-07-15 under explicit owner authorization. The
automated publication gates passed; the real-environment rows below remain open
and are not implied by publication:

- [x] annotated tag object `cd9a60436b3064c5e2f6ed5bfd8ae0f5297f1b49`
  peels to immutable commit `22cfad80e904e003f52932ae6d6403520df00d3c`
  and matches GitHub release `354480554`;
- [x] full local `make ci`, the deterministic exact-v0.1.0 POSIX fixture,
  Windows Pester coverage, and the redacted public-secret scan passed on the
  release commit;
- [x] cache-free hosted run
  [`29419942595`](https://github.com/luisgui1757/dotfiles/actions/runs/29419942595)
  passed Ubuntu, Apple Silicon macOS, Windows, the Linux container, and all four
  stable logical proofs; both POSIX lanes verified the exact v0.2.0 tag path;
- [ ] real WSL host/guest, redirected Windows, divergent stable
  packaged/Preview/Canary/portable Terminal, physical Linux, and Apple Silicon
  owner-host migrations remain unchecked in `tests/MANUAL.md` and unclaimed in
  the append-only review ledger.
