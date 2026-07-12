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

## v0.1.0 to v0.2.0

`v0.1.0` is already a chezmoi release. On POSIX, its managed files are live
symlinks into the checkout. Do **not** run `git pull`, switch that checkout to a
new revision, or run an upgrade from `main`: doing so can change live config
before recovery exists.

These commands become user-facing only after the annotated `v0.2.0` release
exists. Before then, remain on `v0.1.0`.

### Common preparation

Keep the checkout that currently owns v0.1.0 and clone v0.2.0 beside it:

```bash
git clone --branch v0.2.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles-v0.2.0
cd ~/dotfiles-v0.2.0
```

```powershell
git clone --branch v0.2.0 --single-branch `
  https://github.com/luisgui1757/dotfiles.git "$HOME\dotfiles-v0.2.0"
Set-Location "$HOME\dotfiles-v0.2.0"
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

After success, open a new login shell and verify:

```bash
nix store ping
command -v rg fd fzf jq lazygit node starship zoxide nvim
chezmoi --source ~/dotfiles-v0.2.0/home --destination "$HOME" \
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
known folders because UserProfile, LocalApplicationData, Documents, and the
runtime PowerShell profile are resolved independently.

After success, open Neovim, lazygit, PowerShell, and every installed Windows
Terminal variant. Keep the old checkout and protected recovery folder until
those checks pass.

### WSL2 split-host upgrade

WSL still has two independent owners and therefore two setup invocations:

1. Clone v0.2.0 on Windows and run `.\setup.ps1 -All` for host Terminal, font,
   clipboard, and Windows tools.
2. Clone v0.2.0 separately inside the Linux home—never under `/mnt/c`—and run
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
`:Lazy update`, and it never rewrites repository lockfiles.

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

## Release evidence gate

Do not publish v0.2.0 or direct v0.1.0 users here until all of these are true:

- the annotated `v0.2.0` tag and peeled commit are immutable and match the
  release notes;
- the exact-tag POSIX fixture proves setup-owned Nix bootstrap, automatic
  migration orchestration, rollback, retry/resume, acceptance, and final
  reconciliation on Apple Silicon and Linux;
- the Windows Pester and native Windows exact-tag runs prove setup-owned
  migration orchestration and recovery;
- real WSL host/guest, redirected Windows, divergent stable
  packaged/Preview/portable Terminal, and Apple Silicon owner-host migrations
  are recorded in `tests/MANUAL.md` and the append-only review ledger;
- the full release gate and public-secret scan pass on the tagged tree.
