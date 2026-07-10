# Greenfield Evidence Ledger

Append-only record for clean-machine and visual validation. Keep automated
script results separate from manual observations; a desktop surface counts only
when someone inspected it on that machine.

| Date | Environment | Ref / SHA | Command or path | Automated result | Manual visual result | Notes |
|---|---|---|---|---|---|---|
| 2026-06-18 | Static docs guard | `audit/full-roadmap-review-2026-06-18` | `bash tests/static/stale_greenfield_refs_test.sh` | Pass | Not applicable | Ledger created; no Windows Sandbox, WSL, macOS VM, or Linux VM greenfield run recorded by this docs/static update. |
| 2026-07-10 | GitHub-hosted Ubuntu 24.04 | `f4b63953f2f982702a685358b09e89bae2d78fdd` | [Nix job 86360593057](https://github.com/luisgui1757/dotfiles/actions/runs/29092384007/job/86360593057), [setup job 86360593139](https://github.com/luisgui1757/dotfiles/actions/runs/29092384014/job/86360593139) | Pass | Not run | Public Home Manager plus all six setup phases passed. The post-install proof executed the account-record Linuxbrew zsh under `env -i`, resolved `rg` into `/nix/store`, and passed the 257-check language smoke. This PR lane restored caches, so it is hosted runtime evidence, not the cache-free scheduled/manual lane or WSL proof. |
| 2026-07-10 | GitHub-hosted macOS 26 Apple Silicon | `f4b63953f2f982702a685358b09e89bae2d78fdd` | [Nix job 86360593055](https://github.com/luisgui1757/dotfiles/actions/runs/29092384007/job/86360593055), [setup job 86360593136](https://github.com/luisgui1757/dotfiles/actions/runs/29092384014/job/86360593136) | Pass | Not run | Architecture-matched nix-darwin, all six setup phases, real Ghostty/WezTerm validation, AeroSpace app/CLI identity, and the 257-check smoke passed. AeroSpace managed-config consumption was explicitly unavailable without a user-granted TCC desktop session. PR caches were enabled. |
| 2026-07-10 | GitHub-hosted macOS 26 Intel (`macos-26-intel`) | `f4b63953f2f982702a685358b09e89bae2d78fdd` | [Nix job 86360593091](https://github.com/luisgui1757/dotfiles/actions/runs/29092384007/job/86360593091), [setup job 86360593153](https://github.com/luisgui1757/dotfiles/actions/runs/29092384014/job/86360593153) | Pass | Not run | Real x86_64 host installed upstream Nix 2.34.8, selected only `dotfiles-x86_64`, completed nix-darwin and all six setup phases, then passed post-install and 257 language checks. The Nixpkgs 26.05 Intel sunset warning remained visible. This PR lane restored caches and is not TCC/visual or post-2026-12-31 package-plane proof. |
| 2026-07-10 | GitHub-hosted Windows 2025 | `f4b63953f2f982702a685358b09e89bae2d78fdd` | [generic job 86360593205](https://github.com/luisgui1757/dotfiles/actions/runs/29092384006/job/86360593205), [setup job 86360593122](https://github.com/luisgui1757/dotfiles/actions/runs/29092384014/job/86360593122) | Pass | Not run | Native PowerShell setup completed all six phases and post-install assertions, including exact Tree-sitter 0.26.10, Pi 0.80.3, Hack Nerd Font files plus registry consumption, Polaris, and the 257-check smoke. This conventional known-folder run is not redirected-folder, dual-Windows-Terminal, uninstall-restoration, or desktop visual proof. |

## Pending proof (not evidence entries)

- Intel macOS hosted Nix and full setup proof is recorded above. A cache-free
  scheduled/manual run, a real owner host with pre-existing taps, and desktop
  GUI/TCC observations remain separate evidence.
- WSL2, redirected Windows known folders, and desktop GUI/visual behavior still
  require their real environments. Workflow or harness presence does not count
  as a run.
- Packaged-plus-portable Windows Terminal preservation and independent uninstall
  recovery are automated in Pester but still await a real dual-install run; add
  an evidence row only after recording the exact SHA and environment.
- The Windows hosted E2E result above proves Hack Nerd Font files and registry
  registration on a conventional hosted profile. Logical check artifacts remain
  workflow plumbing, not an additional greenfield environment entry.
- Renovate schema/local extraction passed locally; hosted Dashboard ownership
  remains pending until the bot reruns against the exact PR head. Do not record
  a Dashboard row here as Intel, WSL, redirected-Windows, or desktop proof.

## Entry Template

| Date | Environment | Ref / SHA | Command or path | Automated result | Manual visual result | Notes |
|---|---|---|---|---|---|---|
| YYYY-MM-DD | Windows Sandbox / WSL / macOS VM / Linux VM | branch at commit | exact command or `.wsb` path | pass/fail with log path | pass/fail/skipped rows | remaining manual observations |
