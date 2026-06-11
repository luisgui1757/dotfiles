# Greenfield / clean-machine testing

This directory is for local clean-machine validation. CI already runs the real
`setup.sh` / `setup.ps1` paths on hosted clean runners and the native `apt`
container path. These harnesses are for reproducing those installs locally and
for desktop checks CI cannot see, especially Windows Terminal, fonts, ConPTY,
and VS Code rendering.

Do not add these VM or desktop launchers to the CI matrix. They need an
interactive desktop or local virtualization and can hang or fail headless. The
shared validators here are factored so `.github/workflows/e2e-install.yml` could
call them later, but this change intentionally does not rewire CI.

For a literal copy-paste step-by-step (spin up Windows Sandbox / a `tart`
macOS or Linux VM, install, then a per-tool "run this, expect that" checklist),
see [RUNBOOK.md](RUNBOOK.md).

## Shared validators

Run the validators after a real setup:

```bash
tests/greenfield/validate.sh
```

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tests\greenfield\validate.ps1
```

The default mode mirrors the e2e post-install assertions: required tools on
`PATH`, Neovim >= 0.11, managed config paths matching the repo, zsh plugin
externals on POSIX, `chezmoi verify`, Lazy sync, Mason sync, and Mason binaries
for `lua-language-server` and `stylua`.

Caveat: the default mode runs a real `Lazy! sync`, which writes
`nvim/lazy-lock.json` THROUGH the managed `~/.config/nvim` symlink. In a real
greenfield run that symlink points at a repo COPY (the sandbox/VM/distro clone),
so this is harmless. But if you run the full validator on your own machine
against a HOME whose nvim symlinks into your LIVE checkout, the sync can bump the
lockfile in that checkout — use a throwaway clone, or `--config-only` (which
skips Lazy/Mason), to avoid an accidental plugin-pin change.

For a temp-HOME config-layer proof, use `--config-only`; this skips tool,
external clone, Lazy, and Mason checks that require a full setup:

```bash
tmp_home="$(mktemp -d)"
HOME="$tmp_home" chezmoi --source "$PWD/home" init
HOME="$tmp_home" chezmoi --source "$PWD/home" --no-tty --force apply --exclude externals,scripts
HOME="$tmp_home" tests/greenfield/validate.sh --config-only
rm -rf "$tmp_home"
```

## Windows

Primary path: Windows Sandbox. It is a built-in disposable Windows 10/11
Pro/Enterprise VM with a clean user profile each launch.

Prerequisites:

- Enable virtualization in firmware.
- Enable the Windows Sandbox optional feature, then reboot if prompted:

```powershell
Enable-WindowsOptionalFeature -Online -FeatureName Containers-DisposableClientVM -All
```

Run from the repo on Windows:

```powershell
explorer .\tests\greenfield\windows-sandbox.wsb
```

Double-clicking `windows-sandbox.wsb` is equivalent. The `.wsb` maps the repo
read-only into the sandbox and runs `sandbox-run.ps1` at logon. The script copies
the repo to `%USERPROFILE%\dotfiles`, runs `setup.ps1 -All`, rejects nonzero
exit codes and `FAIL:` markers, runs `validate.ps1`, and leaves logs on the
sandbox desktop.

Windows Sandbox starts with no winget and no Scoop. That is intentional: it
exercises the real Scoop-first bootstrap path. The `setup.ps1` Scoop path does
not require admin.

Two Sandbox gotchas:

- The `.wsb` uses a RELATIVE `HostFolder` (`..\..`), which only resolves on
  Windows 11 22H2 and newer. On older builds the sandbox fails to launch with a
  mapped-folder error; fix it by editing `windows-sandbox.wsb` and replacing
  `..\..` with the absolute path to your repo checkout.
- The Sandbox has Developer Mode OFF, so the Neovim directory symlink needs
  elevation. `setup.ps1` reports this; enable Developer Mode in the sandbox
  (Settings -> Privacy & security -> For developers) or run the config phase
  from an elevated prompt, then re-run `validate.ps1`. The single-file copied
  configs do not need it.

Fallbacks:

- Windows Home edition: use Hyper-V where available, VirtualBox, VMware, or UTM
  with a clean Windows snapshot.
- Persistent debugging: use a VM snapshot instead of Sandbox, then run:

```powershell
.\setup.ps1 -All
.\tests\greenfield\validate.ps1
```

Windows containers are not a substitute. They do not model Developer Mode
symlink behavior, Windows Terminal settings, fonts, psmux under real ConPTY, or
the visual desktop surface.

## Linux / WSL

Docker path: reproduce the `e2e containers / ubuntu-24.04` job locally with the
existing CI container script:

```bash
tests/greenfield/docker-greenfield.sh
```

This is the clean native `apt` proof: non-root user, no Linuxbrew bootstrap,
real `install-deps.sh --all`, chezmoi apply, and the existing container
assertions.

Throwaway WSL distro path: run from Windows PowerShell:

```powershell
.\tests\greenfield\wsl-greenfield.ps1
```

The script creates a uniquely named `dotfiles-greenfield-<timestamp>` Ubuntu
WSL distro, copies the repo in, runs `./setup.sh --all`, runs
`validate.sh`, and unregisters the distro on exit. Add `-Keep` to preserve the
distro for debugging:

```powershell
.\tests\greenfield\wsl-greenfield.ps1 -Keep
```

Rootfs source options:

- Modern WSL: the script first tries
  `wsl --install -d Ubuntu-24.04 --name <name> --no-launch`.
- If that WSL build does not support named installs, download an Ubuntu 24.04
  WSL rootfs tarball, then run:

```powershell
.\tests\greenfield\wsl-greenfield.ps1 -RootfsTar C:\path\ubuntu-24.04-rootfs.tar.gz
```

No Linux VM is needed for this repo. Use Docker for the clean native `apt` path
and a throwaway WSL distro for the WSL userland path.

## macOS

Fast HOME-clean path: create a fresh macOS user account, log into it, clone this
repo, then run:

```bash
bash tests/greenfield/macos-greenfield.sh --current-home
```

This gives a clean HOME and real user-level app/config behavior. It is not a
clean OS because Homebrew and system-level state may already exist on the host.

Config debugging path with an explicit HOME sandbox:

```bash
bash tests/greenfield/macos-greenfield.sh --home "$PWD/tests/.cache/macos-greenfield-home"
```

This is useful for setup/config debugging, but package installs still affect the
current macOS host.

True clean OS path: use a macOS VM on Apple Silicon through `tart`, UTM, or
VMware Fusion, clone the repo, then run the same `--current-home` command. If no
local VM is available, rely on the `setup.sh / macos-15` GitHub-hosted e2e job.

## Manual visual checklist

Run these after the automated greenfield checks on a real desktop:

- Windows Terminal opens maximized, not fullscreen, with a visible scrollbar.
- Starship prompt and language icons render with no tofu boxes.
- `ls` / `Get-ChildItem` directories are gold.
- PSReadLine ListView predictions are visible in rose/gold.
- fzf `Ctrl+R`, `Ctrl+T`, and `Alt+C` work.
- psmux loads a pane without freezing and stays at normal CPU.
- VS Code uses Rose Pine and Hack Nerd Font.
- VS Code editor glyphs and integrated terminal glyphs render correctly.
