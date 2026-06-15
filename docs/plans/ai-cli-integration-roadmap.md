# Roadmap: AI-CLI integration (Polaris + Pi) into dotfiles

Status: **PLANNED — not implemented.** Reconciled with a Codex 5.5 xhigh second
opinion — it **agreed on every major decision** (Option A / Git-Bash, no vendor,
no submodule, npm-only Pi, after `chezmoi-pilot` merges) and added the refinements
folded in below (the careful Git-Bash invocation, verified pins, npm PATH repair).
Build it on a **fresh branch after `chezmoi-pilot` merges** (do not pile it onto
the in-test migration branch).

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

**Windows invocation must be careful — NOT a naive `& bash`** (Codex's key
refinement): the wrapper must (1) locate Git-for-Windows bash specifically and
**reject `C:\Windows\System32\bash.exe`** (the WSL launcher) / any WSL bash — else
it writes into the WSL filesystem, not Windows; (2) set `HOME="$(cygpath -u
"$USERPROFILE")"` and `cygpath -u` the Polaris checkout path; (3) normalize the env
overrides the installer reads — `CODEX_HOME`, `XDG_CONFIG_HOME`,
`PI_CODING_AGENT_DIR` — through `cygpath`; (4) run `& $gitBash --noprofile --norc
-lc 'cd "$1"; exec tools/install --global' polaris <checkout>`. The installer uses
`$HOME`/env + `mktemp`/`awk`/`cmp`/`mv` and writes LF Markdown, so it IS
Git-Bash-compatible when invoked this way (confirmed by reading the script). The
path-translation risk is real but **resolved by the cygpath normalization above**.

Rejected alternatives: a **pwsh re-implementation** of the renderer (duplicates
`compose_into` + bundle hashing + manifest parsing + drift logic — drift risk);
**vendor on Windows** (still needs bash to render); **skip native Windows**
(breaks cross-platform parity — last-resort fallback only, since Git-Bash is
guaranteed by the git prereq).

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
  tree-sitter pins). CI stays red until reviewed. **Starting pins (Codex-computed,
  re-verified here — `verify-vendor` reports "bundle MATCHES"):**
  `POLARIS_REPO_URL=https://github.com/luisgui1757/polaris.git`,
  `POLARIS_REF=65c96982eb055cca3d2a2bcf86844ca902b76c53`,
  `POLARIS_BUNDLE_SHA256=eff45a5e7dc888f3c92642ccf677f7d5564e77c642cee1051ae1e005b6d558c2`.
  Install root: `${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/polaris` (Unix) /
  `%LOCALAPPDATA%\dotfiles\polaris` (Windows). Flow: clone → checkout detached REF
  → assert `git rev-parse HEAD == POLARIS_REF` → `tools/verify-vendor <checkout>
  $POLARIS_BUNDLE_SHA256` → `tools/install --global`.
- **Static guard.** Never auto-run `tools/install --target <this repo>` (it would
  commit a rules block into dotfiles' history) and keep this repo's own `AGENTS.md`
  thin — add a test that asserts both.
- **Idempotent + drift-gated + reversible.** `--global` is re-runnable (marked
  block); add `tools/install --check` as a test/CI drift gate; `--remove` is a
  clean uninstall (keeps the user's own text); `--dry-run` previews.
- **Consent + dry-run.** Gate behind the existing `ask` / `Ask` prompts (auto-yes
  under `--all` / `-All`); honor `--dry-run` / `-DryRun` (print a `would:` line).

## Pi integration

- **npm-ONLY — do NOT add `pi` to the Scoop/winget/choco `$Catalog`** (Codex's
  refinement). Package `@earendil-works/pi-coding-agent` (npm/TypeScript,
  v0.78.1 at time of writing), binary `pi` (`dist/cli.js`), reads
  `~/.pi/agent/AGENTS.md` globally and per-repo `AGENTS.md`; `--no-context-files`
  opts out. It is not in any OS package manager, so a dedicated installer
  (`install_pi_cli` / `Install-PiCli`) running `npm install -g
  @earendil-works/pi-coding-agent` is the one correct path on every OS — node is
  already installed everywhere (prettier / tree-sitter).
- **PATH repair (the real per-OS caveat).** Verify `pi --version` resolves after
  install. On Linux non-root the npm global prefix is often unwritable / off
  PATH — fall back to `npm install -g --prefix "$HOME/.local"` and ensure
  `$HOME/.local/bin` is on PATH (same shim pattern as the `fd-find` → `fd`
  fix). On Windows the freshly-created npm global shim dir isn't on the
  in-process PATH — repair it with `Add-DirectoryToUserPath` (the same
  registry-PATH re-read the VS Code shim fix uses), don't dead-end on a missing
  `pi` in the same process.
- **Composition:** install Pi first, then Polaris `--global` writes
  `~/.pi/agent/AGENTS.md`. Pi then auto-loads the rules at startup. No extra
  wiring — Polaris already targets Pi.

## Implementation checklist (when built)

1. `install-deps.sh`: `install_pi_cli` (npm global, `$HOME/.local` prefix
   fallback when the global prefix is unwritable) + `install_polaris`
   (clone @ pinned SHA → `~/.local/share/dotfiles/polaris`, `verify-vendor`,
   then `bash tools/install --global`); consent-gated, dry-run-safe.
2. `install-deps.ps1`: `Install-PiCli` (npm, then `Add-DirectoryToUserPath` for
   the npm shim dir) + `Install-Polaris` (clone → `%LOCALAPPDATA%\dotfiles\polaris`,
   resolve Git-for-Windows bash — reject WSL `System32\bash.exe` — `& $bash
   --noprofile --norc -lc tools/install --global`); `-All`-gated/consent,
   DryRun-safe, best-effort + FAIL marker. Pi stays npm-only (NOT in `$Catalog`).
3. Pin constants: `POLARIS_REF`, `POLARIS_BUNDLE_SHA256`; renovate.json custom
   manager (version bumpable, SHA human-reviewed — context-only, NOT currentDigest).
4. Tests: shell + Pester coverage (clone+verify+invoke stubbed; pinned-ref +
   fail-closed checksum; Git-Bash resolution); optional `tools/install --check`
   drift gate; container-e2e presence assert for `pi`.
5. Docs: CLAUDE.md invariant addition + a "what weird but intentional" note;
   README dependency mention.

## Open questions (Codex pass + a real run will answer)

- Git-Bash `$HOME` / MSYS path translation / CRLF when writing Windows files
  (best-guess: resolved by the cygpath normalization + `--noprofile --norc -lc`
  above; a real Windows run confirms).
- Whether `pi` lands on PATH cleanly per OS (package name now confirmed:
  `@earendil-works/pi-coding-agent`, binary `pi`). Best-guess: needs the
  `$HOME/.local` prefix fallback on Linux non-root and `Add-DirectoryToUserPath`
  on Windows — both already in the checklist.
- Whether to also `tools/install --target ~/dotfiles` so the dotfiles repo itself
  carries the rules for contributors (orthogonal, optional, one-time + commit).
- Renovate hashing of `POLARIS_BUNDLE_SHA256` (context-only match, like the other
  direct-download SHAs).

## Recommendation

Do it on a **fresh branch after `chezmoi-pilot` merges.** Canonical shape:
**install Pi via npm; clone Polaris at a pinned SHA; run `tools/install --global`
(via Git-Bash on Windows); pin + verify; idempotent with a `--check` drift gate;
never vendor, never sync personal agent state.**
