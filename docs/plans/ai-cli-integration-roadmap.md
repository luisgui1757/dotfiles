# Roadmap: AI-CLI integration (Polaris + Pi) into dotfiles

Status: **PLANNED — not implemented.** Best-guess canonical design; a Codex 5.5
xhigh second opinion is being folded in. Build it on a **fresh branch after
`chezmoi-pilot` merges** (do not pile it onto the in-test migration branch).

## What each thing is

- **Polaris** (`luisgui1757/polaris`) — a small, language-agnostic *rulebook* for
  AI coding assistants. `core/` holds generic rules (MODES / INVARIANTS /
  EXECUTION / WORKFLOW / TESTING / MEMORY). `tools/install` inlines a delimited
  `<!-- AGENT-RULES:BEGIN … END -->` block (stamped with the bundle sha256) into
  the entrypoint each AI CLI auto-reads, preserving your own text around it. It is
  a **tool you run**, not config to vendor.
- **Pi** (`@earendil-works/pi-coding-agent`, npm/TypeScript) — a minimal
  open-source terminal coding agent. Reads `AGENTS.md` per-repo and
  `~/.pi/agent/AGENTS.md` globally. So **Pi is just one more Polaris consumer** —
  Polaris already writes `~/.pi/agent/AGENTS.md`.

## The integration model (decided)

Three repos, one-way deps; **no submodule, no vendoring into dotfiles' public
history**:

```
dotfiles ──installs──► pi (npm) + polaris (pinned clone)
                           └── polaris tools/install --global ──injects──►
                                 ~/.claude/CLAUDE.md, ~/.codex/AGENTS.md,
                                 ~/.config/opencode/AGENTS.md, ~/.pi/agent/AGENTS.md
```

Polaris's adoption models are **install** (`--global` = every repo on this
machine; `--target DIR` = commit rules into one repo) and **vendor** (copy
`core/`+`MANIFEST.json`, pin a SHA, `tools/verify-vendor`). For "apply to
everything on my machine" the answer is **install `--global`**, NOT vendor.

## The cross-platform crux (and its resolution)

**Polaris's installer is entirely bash** (`tools/install`, `tools/polaris-lib.sh`
— no `.ps1`). It resolves targets via `$HOME` (`$HOME/.claude/CLAUDE.md`,
`${PI_CODING_AGENT_DIR:-$HOME/.pi/agent}/AGENTS.md`, …). So:

| OS | How `tools/install --global` runs |
|---|---|
| macOS / Linux / WSL | natively from `install-deps.sh` — `bash <polaris>/tools/install --global` |
| **native Windows** | via **Git-Bash** from `install-deps.ps1` — git is a HARD prereq, so Git-for-Windows ships `bash.exe`. Resolve it from `git`'s install root (`<git>\usr\bin\bash.exe` or `<git>\bin\bash.exe`) and run `& $bash <polaris>/tools/install --global`. Under Git-Bash `$HOME` = `%USERPROFILE%`, so it writes `%USERPROFILE%\.claude\CLAUDE.md`, `%USERPROFILE%\.pi\agent\AGENTS.md`, etc. — the same paths the Windows CLIs read. |

Rejected alternatives: a **pwsh re-implementation** of the renderer (duplicate
logic, drift risk); **vendor on Windows** (still needs bash to render); **skip on
Windows** (breaks cross-platform parity — only acceptable as a fallback if the
Git-Bash path fails). **Open risk to verify:** Git-Bash `$HOME` / MSYS path
translation / CRLF when the bash script writes Windows-side files.

## Honoring the dotfiles invariants

- **Tool, not chezmoi config.** Wire it into **install-deps** (the tool phase),
  NOT chezmoi — the entrypoints Polaris writes are *agent config*, which dotfiles
  deliberately does not chezmoi-manage.
- **Never sync personal agent state.** dotfiles installs the Polaris *tool* and
  runs `--global` (a generic marked block). It must NEVER vendor Polaris content
  into dotfiles' own public history and NEVER sync the owner's real `~/.claude`
  settings/keys/allowlists. Proposed invariant addition:
  > *dotfiles installs the AI-rules tool (Polaris) and lets `--global` inject a
  > generic marked block; it never syncs personal agent state and never vendors
  > rules into this repo's history.*
  This is the `claude/settings.json` PII-leak lesson, generalized.
- **Pin + verify.** Clone Polaris at a pinned `POLARIS_REF` (commit SHA), like the
  other pinned downloads; verify integrity with Polaris's own bundle sha256 /
  `tools/verify-vendor` against a pinned `POLARIS_BUNDLE_SHA256`. Renovate bumps
  the ref; a human reviews the new bundle hash (same pattern as the lazygit /
  tree-sitter pins). CI stays red until reviewed.
- **Idempotent + drift-gated + reversible.** `--global` is re-runnable (marked
  block); add `tools/install --check` as a test/CI drift gate; `--remove` is a
  clean uninstall (keeps the user's own text); `--dry-run` previews.
- **Consent + dry-run.** Gate behind the existing `ask` / `Ask` prompts (auto-yes
  under `--all` / `-All`); honor `--dry-run` / `-DryRun` (print a `would:` line).

## Pi integration

- Install cross-platform via **`npm install -g @earendil-works/pi-coding-agent`**
  — node is already installed on every OS (prettier / tree-sitter). Add it as a
  dedicated installer (like `Install-TreeSitterCli`) or a catalog+npm-fallback
  entry; verify `pi --version` resolves after install (mind npm global PATH on
  Linux non-root and the in-process PATH refresh on Windows).
- **Composition:** install Pi first, then Polaris `--global` writes
  `~/.pi/agent/AGENTS.md`. Pi then auto-loads the rules at startup. No extra
  wiring — Polaris already targets Pi.

## Implementation checklist (when built)

1. `install-deps.sh`: `install_pi_cli` (npm global) + `install_polaris`
   (clone @ pinned SHA → `~/.local/share/polaris`, `verify-vendor`, then
   `bash tools/install --global`); consent-gated, dry-run-safe.
2. `install-deps.ps1`: `Install-PiCli` (npm) + `Install-Polaris` (clone →
   `%LOCALAPPDATA%\polaris`, resolve Git-Bash from git, `& $bash tools/install
   --global`); `-All`-gated/consent, DryRun-safe, best-effort + FAIL marker.
3. Pin constants: `POLARIS_REF`, `POLARIS_BUNDLE_SHA256`; renovate.json custom
   manager (version bumpable, SHA human-reviewed — context-only, NOT currentDigest).
4. Tests: shell + Pester coverage (clone+verify+invoke stubbed; pinned-ref +
   fail-closed checksum; Git-Bash resolution); optional `tools/install --check`
   drift gate; container-e2e presence assert for `pi`.
5. Docs: CLAUDE.md invariant addition + a "what weird but intentional" note;
   README dependency mention.

## Open questions (Codex pass + a real run will answer)

- Git-Bash `$HOME` / MSYS path translation / CRLF when writing Windows files.
- Exact Pi npm package name + whether `pi` lands on PATH cleanly per OS.
- Whether to also `tools/install --target ~/dotfiles` so the dotfiles repo itself
  carries the rules for contributors (orthogonal, optional, one-time + commit).
- Renovate hashing of `POLARIS_BUNDLE_SHA256` (context-only match, like the other
  direct-download SHAs).

## Recommendation

Do it on a **fresh branch after `chezmoi-pilot` merges.** Canonical shape:
**install Pi via npm; clone Polaris at a pinned SHA; run `tools/install --global`
(via Git-Bash on Windows); pin + verify; idempotent with a `--check` drift gate;
never vendor, never sync personal agent state.**
