# Upgrading dotfiles releases

## v0.1.0 to v0.2.0

`v0.1.0` is already a chezmoi release. On POSIX, its managed files are live
symlinks into the checkout. Do **not** run `git pull`, switch that checkout to a
new revision, or run an upgrade from `main`: doing so can change live config
before the new setup has captured recovery state.

The only supported transition is between two clean, separate checkouts of the
official annotated release tags. The migration tools verify the local tag
object, peeled commit, official remote tag, clean checkout, target identity,
and exact v0.1.0 managed state before mutation. They retain the v0.1.0 checkout
and private recovery material until explicit acceptance.

These commands become user-facing only after the annotated `v0.2.0` release
exists. Before then, remain on `v0.1.0`; the scripts deliberately reject a
branch, moving `main`, lightweight tag, altered release, dirty checkout, or
in-place transition.

### Common preparation

Choose the checkout that currently owns the v0.1.0 config. These examples use
`~/dotfiles` on POSIX and `$HOME\dotfiles` on Windows. If yours differs, pass
the real path.

Create the new checkout beside it; never reuse the old directory:

```bash
git clone --branch v0.2.0 --single-branch \
  https://github.com/luisgui1757/dotfiles.git ~/dotfiles-v0.2.0
```

```powershell
git clone --branch v0.2.0 --single-branch `
  https://github.com/luisgui1757/dotfiles.git "$HOME\dotfiles-v0.2.0"
```

Do not stash, delete, or overwrite local changes to make preflight pass. Keep
the old checkout and resolve its tracked or untracked state deliberately. The
migration refuses to infer which user bytes are authoritative.

### Nix prerequisite for macOS, Linux, and WSL

Nix is a platform prerequisite, not a hidden setup side effect. From the exact
v0.2.0 checkout, use the reviewed helper:

```bash
~/dotfiles-v0.2.0/scripts/install-nix-prerequisite.sh --install
```

It downloads the official upstream Nix `2.34.0` binary release over HTTPS,
checks the review-pinned SHA-256 for `aarch64-darwin`, `x86_64-linux`, or
`aarch64-linux`, and executes nothing until the digest matches. It selects the
recommended multi-user install on macOS and systemd Linux; WSL/non-systemd
Linux uses the upstream single-user mode. Open a new shell afterward and
require both commands to succeed:

```bash
nix --version
nix store ping
```

The versioned release archives and adjacent hashes are published by the
[official Nix release service](https://releases.nixos.org/nix/nix-2.34.0/),
and Nix documents multi-user as the recommended macOS/systemd-Linux mode in
the [installation manual](https://nix.dev/manual/nix/latest/installation/installation).
There is no pipe-to-shell path in this repository.

### Apple Silicon macOS

Preflight is read-only:

```bash
~/dotfiles-v0.2.0/scripts/upgrade-v0.1.0.sh \
  --preflight-only ~/dotfiles
```

Apply from the retained v0.1.0 checkout into the exact v0.2.0 checkout:

```bash
~/dotfiles-v0.2.0/scripts/upgrade-v0.1.0.sh --apply ~/dotfiles
```

The tool prints a private recovery directory. Keep that path and both
checkouts. It records the package-provider boundary, Homebrew inventory,
nix-homebrew tap backup state, exact flake lock, both config target sets, and
read-only trees extracted from the exact old/new commits. Nix activation and
config publication use only the frozen new tree; later edits to either checkout
cannot change a transaction write. Rollback likewise uses only the frozen
v0.1.0 tree.
If any phase after nix-darwin activation fails or is interrupted, it removes
the first nix-darwin generation through the lock-pinned uninstaller, restores
the prior tap repository and newly introduced Homebrew items, and reapplies
the exact v0.1.0 config.

Verify in a new login shell before acceptance:

```bash
test -e /run/current-system
nix store ping
command -v rg fd fzf jq lazygit node starship zoxide
chezmoi --source /exact/recovery/new-release/home --destination "$HOME" \
  verify --include files,symlinks
```

Rollback or accept using the exact commands printed in `RECOVERY.txt`:

```bash
/exact/recovery/upgrade-v0.1.0.sh --rollback /exact/recovery
/exact/recovery/upgrade-v0.1.0.sh --accept /exact/recovery
```

Acceptance verifies the release commit, config, and package layer again and
closes automatic rollback. Keep the old checkout through the additive setup
below; archive or remove it only after that setup and application checks pass.
Keep recovery material until personal data has been independently checked.
Until full setup runs, POSIX config links intentionally resolve into the frozen
recovery tree. Full setup backs those exact links up and repoints config to the
v0.2.0 checkout. Only after that repoint and independent checks may the
recovery directory be made writable and removed.

After acceptance, install native/deferred tools and restore editor state as a
separate additive phase:

```bash
cd ~/dotfiles-v0.2.0
./setup.sh --all
```

The release transaction deliberately passed `--skip-native-deps`,
`--skip-config-scripts`, `--skip-nvim`, and `--skip-agents`. Config files and
links publish only after target-parent creation and backup; chezmoi run scripts,
native packages, and executable caches remain outside the rollback boundary.
If later additive provisioning fails, the
accepted Nix/config core remains verified; fix the reported provider and rerun.
Setup never performs a blanket package-manager upgrade.

### Native Linux x86_64 or aarch64

After the Nix prerequisite, run:

```bash
~/dotfiles-v0.2.0/scripts/upgrade-v0.1.0.sh \
  --preflight-only ~/dotfiles
~/dotfiles-v0.2.0/scripts/upgrade-v0.1.0.sh --apply ~/dotfiles
```

The transaction installs the architecture-matched package-only Home Manager
generation, then backs up and publishes config. Native package-manager and
Linuxbrew packages are not removed. A later failure removes the first Home
Manager generation through the exact lock-pinned Home Manager app and restores
the retained v0.1.0 config.

Verify before using the printed `--accept` command:

```bash
nix store ping
test -e "${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles/home-manager" \
  -o -e "/nix/var/nix/profiles/per-user/$USER/home-manager"
env -i HOME="$HOME" USER="$USER" PATH=/usr/bin:/bin \
  zsh -l -i -c 'command -v rg; command -v starship'
chezmoi --source /exact/recovery/new-release/home --destination "$HOME" \
  verify --include files,symlinks
```

Use the printed `--rollback` command before acceptance to return to v0.1.0.
Do not use `home-manager switch --rollback` for this first-generation
transition: the release tool removes the newly introduced Home Manager state
and validates the historical config itself.

### Native Windows

Windows remains non-Nix. Enable Developer Mode, open a normal PowerShell, and
run the read-only preflight:

```powershell
& "$HOME\dotfiles-v0.2.0\scripts\upgrade-v0.1.0.ps1" `
  -PreflightOnly -OldCheckout "$HOME\dotfiles"
```

Apply the reversible config/known-folder/Windows-Terminal cutover:

```powershell
& "$HOME\dotfiles-v0.2.0\scripts\upgrade-v0.1.0.ps1" `
  -Apply -OldCheckout "$HOME\dotfiles"
```

This phase intentionally passes `-SkipDeps`, `-SkipNvim`, `-SkipAgents`, and
`-SkipConfigScripts`: dependency installation, executable caches, agent policy,
and chezmoi run scripts are not mixed into config recovery. The private
LocalApplicationData recovery folder
uses a protected ACL; it captures the exact stable packaged, Preview packaged,
and portable Windows Terminal
bytes plus digest-bound trees extracted from both exact release commits. Apply,
verification, uninstall, and rollback consume only those frozen trees; edits or
loss of either checkout after recovery capture cannot alter a transaction
write. All three canonical Terminal identities validate before any restore
write. Expected presence and post-migration hashes are recorded independently,
so explicit acceptance fails if a selected variant is absent, still has its
pre-migration bytes, or changed externally. The old
checkout remains required for acceptance. Conventional v0.1 known-folder
targets remain in place while rollback authority is open; rollback removes only
the two transaction-created chezmoi state databases. Current setup resolves
UserProfile, LocalApplicationData, Documents,
and the active PowerShell profile independently, so the same command covers
conventional, redirected, OneDrive, and alternate-drive known folders.

Before acceptance, open Neovim, lazygit, PowerShell, and every installed
Windows Terminal variant, then run the printed `-Accept` command. Use the
printed `-Rollback` command on any discrepancy. After acceptance, keep the old
checkout while full setup moves the retained conventional v0.1 targets to
collision-safe `.legacy.*` evidence and installs missing v0.2.0 native tools:

```powershell
Set-Location "$HOME\dotfiles-v0.2.0"
.\setup.ps1 -All
```

If that package phase fails, the accepted config remains verified; fix the
reported provider and rerun without removing the old checkout. Setup installs
missing scoped packages and never runs a blanket package-manager upgrade. Only
after it succeeds and repoints every managed link away from recovery may the old
checkout and private recovery tree be archived or removed.

### WSL2 split-host upgrade

Do not treat a Linux container as WSL proof. A WSL install has two owners and
two recovery records:

1. On Windows, run the native-Windows `-PreflightOnly` and `-Apply` commands.
   Keep the Windows recovery path unaccepted.
2. Inside the WSL distro, create its own exact v0.2.0 checkout in the Linux
   home (never under `/mnt/c`), install Nix there, then run the Linux
   `--preflight-only` and `--apply` commands. Keep the guest recovery path.
3. Run `tests/wsl/e2e.sh` in the guest and verify Windows Terminal, font,
   `win32yank`, and PowerShell on the host.
4. If the guest fails, run its rollback first, then run the Windows recovery
   command. If both verify, accept the guest first and Windows second.

Never accept the host while the guest is still unverified. The Windows and
Linux tools each auto-rollback their own mutation boundary; this ordering keeps
host rollback authority available until the guest is known-good.

## Commands that are not release migrations

`./setup.sh --update` and `.\setup.ps1 -Update` refresh only the deliberately
drift-tolerant tool edge and Mason. They do not fetch Git, activate the pinned
release config, restore Lazy, or perform the versioned Nix transition. Likewise,
`git pull` is not an upgrade transaction.

## Release evidence gate

Do not publish v0.2.0 or direct v0.1.0 users here until all of these are true:

- the annotated `v0.2.0` tag and peeled commit are immutable and match the
  release notes;
- `tests/migration/v0_1_upgrade_test.sh` passes from the exact tag on Apple
  Silicon and Linux, including post-activation rollback and interruption;
- the Windows Pester frozen-source/recovery suite and native Windows exact-tag
  upgrade pass;
- real WSL host/guest, redirected Windows, divergent stable packaged/Preview/portable
  Terminal, and Apple Silicon owner-host migrations are recorded in
  `tests/MANUAL.md` and the append-only review ledger;
- the full release gate and public-secret scan pass on the tagged tree.
