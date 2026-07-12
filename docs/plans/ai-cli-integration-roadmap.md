# Roadmap: AI-CLI integration (Sentinel + Pi) into dotfiles

Status: **SUPERSEDED HISTORICAL PLAN.** The shipped dotfiles/Sentinel path now
uses the pinned Sentinel checkout plus the repo's current installer behavior.
Do not follow the older "NO Git-Bash / NO cygpath" guidance below as current
dotfiles implementation truth; verify against `setup.sh`, `setup.ps1`,
`install-deps.sh`, `install-deps.ps1`, `README.md`, and `CLAUDE.md` instead.

Originally planned as: **PLANNED — not implemented.** Reconciled with a Codex 5.5 xhigh second
opinion (no vendor, no submodule, npm-only Pi, after the pilot migration branch merges) and
then **superseding the Git-Bash mechanism** per the owner's call: instead of
shelling Sentinel's bash installer through Git-Bash on Windows, **Sentinel gains a
native PowerShell installer** and dotfiles calls Sentinel's installer per-OS (bash
on Unix, pwsh on Windows). That deletes the entire Git-Bash / cygpath / WSL-bash
layer. **The new make-or-break is renderer byte-parity**: the Sentinel bundle
sha256 must be `eff45a5e…` identically on every OS or `--check` lies cross-platform
(an adversarial parity pass confirmed 7 real divergence traps and dismissed 11;
all folded in below). Build it on a **fresh branch after the pilot migration branch merges**
(do not pile it onto the in-test migration branch).

## What each thing is

- **Sentinel** (`luisgui1757/sentinel`) — a small, language-agnostic *rulebook* for
  AI coding assistants. `core/` holds generic rules (MODES / INVARIANTS /
  EXECUTION / WORKFLOW / TESTING / MEMORY). `tools/install` inlines a delimited
  `<!-- AGENT-RULES:BEGIN … END -->` block (stamped with the bundle sha256) into
  the entrypoint each AI CLI auto-reads, preserving your own text around it. It is
  a **tool you run**, not config to vendor.
- **Pi** (`@earendil-works/pi-coding-agent`, npm/TypeScript) — a minimal
  open-source terminal coding agent. Reads `AGENTS.md` per-repo and
  `~/.pi/agent/AGENTS.md` globally. So **Pi is just one more Sentinel consumer** —
  Sentinel already writes `~/.pi/agent/AGENTS.md`.

## The integration model (decided)

Three repos, one-way deps; **no submodule, no vendoring into dotfiles' public
history**:

```
dotfiles ──installs──► pi (npm) + sentinel (pinned clone)
                           └── sentinel tools/install --global ──injects──►
                                 ~/.claude/CLAUDE.md, ~/.codex/AGENTS.md,
                                 ~/.config/opencode/AGENTS.md, ~/.pi/agent/AGENTS.md
```

Sentinel's adoption models are **install** (`--global` = every repo on this
machine; `--target DIR` = commit rules into one repo) and **vendor** (copy
`core/`+`MANIFEST.json`, pin a SHA, `tools/verify-vendor`). For "apply to
everything on my machine" the answer is **install `--global`**, NOT vendor.

## The cross-platform crux (and its resolution)

**Sentinel's installer is bash today** (`tools/install`, `tools/sentinel-lib.sh`).
The decided resolution is **not** to shell that bash through Git-Bash on Windows.
Instead **Sentinel gains a native PowerShell installer** (`tools/install.ps1`, in
the Sentinel repo) that ports the renderer byte-for-byte, and **dotfiles calls
Sentinel's installer per-OS**:

| OS | How dotfiles runs Sentinel `--global` |
|---|---|
| macOS / Linux / WSL | natively from `install-deps.sh` — `bash <sentinel>/tools/install --global` (unchanged) |
| **native Windows** | natively from `install-deps.ps1` — `pwsh -NoProfile -NonInteractive -File <sentinel>\tools\install.ps1 -Global`. No Git-Bash, no `cygpath`, no WSL-bash rejection. The pwsh installer reads `%CODEX_HOME%` / `$env:USERPROFILE` / `%XDG_CONFIG_HOME%` / `%PI_CODING_AGENT_DIR%` natively and writes `%USERPROFILE%\.claude\CLAUDE.md`, `%USERPROFILE%\.pi\agent\AGENTS.md`, etc. — the same paths the Windows CLIs read. |

dotfiles passes the **identical flag strings** on both OSes (`--global` ⇄
`-Global`, `--target DIR` ⇄ `-Target DIR`, `--check` ⇄ `-Check`, `--dry-run` ⇄
`-DryRun`, `--remove`/`--uninstall` ⇄ `-Remove`). The pwsh installer is **pure
ASCII, no `Invoke-Expression`, no `$Home`/`$IsWindows`/`$IsLinux` assignment**, and
its exit codes mirror bash (0 ok, 1 drift/missing/refusal, 2 arg error, 3
unterminated block).

**The new make-or-break is BYTE-PARITY between the two renderers.** The bundle
sha256 must be identical cross-OS or `tools/install --check`'s `cmp -s` byte
comparison lies on Windows. Verified live at the pinned SHA: the RAW bundle is
**9206 bytes** ending `holds.\n\n` and hashes to
`eff45a5e7dc888f3c92642ccf677f7d5564e77c642cee1051ae1e005b6d558c2` (the value
stamped in the committed `AGENTS.md`/`CLAUDE.md`/`.github/copilot-instructions.md`);
the full managed block is **9774 bytes**. A pwsh port must reproduce **seven
verified traps**, each guarded by the parity test:

1. **Hash-vs-embed trailing newline.** `bundle=$(...)` strips trailing newlines and
   `printf '%s\n'` re-adds one, so the EMBEDDED form is **9205 bytes** (`holds.\n`)
   while the HASHED form is the 9206-byte raw `holds.\n\n`. SHA over the raw bytes
   (→ `eff45a5e…`); embed `$raw.TrimEnd("`n")+"`n"`. Hashing the embed form gives
   the wrong `be4d1733…`; embedding the raw form makes the block one byte too long
   and `cmp -s` drifts.
2. **CRLF working-tree (`.gitattributes` gap).** `core/*.md` resolve to `text:
   auto` with NO `eol=lf` (only `*.sh/*.bash/*.zsh/*.toml/*.conf` get it), so a
   Windows `core.autocrlf=true` checkout yields CRLF markdown → a 9417-byte bundle
   hashing to `0fb1c557…`. Fixed at the source by a one-line `.gitattributes`
   addition (below), plus a forced-LF clone and renderer CRLF→LF normalization.
3. **pwsh newline/BOM injection.** `Get-Content`+`-join`, `Out-File`, `Set-Content`
   inject CRLF/BOM on Windows. Read with `[System.IO.File]::ReadAllText` (UTF-8
   no-BOM), join with `` `n `` only, write via `WriteAllText` + `UTF8Encoding($false)`,
   hash over `[Text.Encoding]::UTF8.GetBytes($raw)`; invoke with `-NoProfile`.
4. **Heading-demotion awk, bug-for-bug.** Toggle `$fence` on `^\x60\x60\x60`
   (anchored, backticks only — tilde `~~~` must NOT toggle, 4-backtick and
   ```` ```lang ```` DO, indented fences do NOT), THEN demote `^#{1,6} ` outside
   fences. awk's ORS adds a missing final newline, so a no-trailing-newline file
   still emits `\n\n`.
5. **Per-file separator.** Each file is followed by exactly one `printf '\n'`; the
   port must re-emit each line + `` `n `` (adding a missing final newline like awk),
   THEN append one `` `n `` separator — a "preserve-as-is" port drifts by one byte.
6. **sha output case.** `sha256sum`/`shasum` emit lowercase; `Get-FileHash` is
   uppercase. The port must `.ToLowerInvariant()` the digest and compute it via
   `[SHA256]::Create().ComputeHash` over no-BOM bytes, never `Get-FileHash` of a
   temp file.
7. **compose append-vs-replace asymmetry.** The bash append path cats the block
   verbatim while the replace path's awk `getline`/ORS re-adds a trailing LF, so a
   block missing its final newline writes one way on first install and DRIFTS on
   the next `--check`. Render the block ending `END -->` + exactly one LF; make the
   pwsh `-Check` an ordinal/byte (SHA-of-bytes) compare, never a normalized `-eq`.

Rejected alternatives: **Git-Bash** (the previously-planned Option A) requires
Git-for-Windows bash resolution, rejecting `C:\Windows\System32\bash.exe`,
`cygpath`-normalizing HOME + checkout + `CODEX_HOME`/`XDG_CONFIG_HOME`/
`PI_CODING_AGENT_DIR`, and an unverified MSYS path-translation/CRLF question — the
native port deletes all of it. **Vendor-render** still needs a Windows renderer AND
violates the no-vendoring invariant. **Skip-Windows** breaks the cross-platform
parity that is the whole point of these dotfiles. The pwsh re-implementation cost is
paid **once, in Sentinel** (not dotfiles), guarded by a byte-parity test, and removes
a whole class of MSYS/cygpath/WSL footguns from the Windows installer.

## Sentinel-side prerequisite (must land first)

The Windows path depends on work **in the Sentinel repo** (sent as a Sentinel PR
before dotfiles wires it up):

1. **Native `tools/install.ps1`** — self-locating via `$PSScriptRoot`; flag surface
   `[switch]$Global,[string]$Target,[switch]$Check,[switch]$DryRun,[switch]$Remove`
   (+ `[Alias('Uninstall')]`), resolving composed/conflicting action flags
   left-to-right to mirror bash's last-wins. Ports `sentinel_render_bundle` +
   `compose_into`/`remove_block` honoring all seven traps above. Global targets use
   a local `$homeDir` (`$env:HOME` else `$env:USERPROFILE`), NOT `$Home`.
2. **`.gitattributes` fix** — add `*.md text eol=lf` (ideally also `core/** text
   eol=lf`). Verified gap: `git check-attr -a core/MODES.md` → `text: auto` (no
   eol) while `*.sh` → `eol: lf`; `*.md` was simply omitted. The one-liner forces
   LF on every checkout/platform with zero renderer code and matches the file's own
   stated intent — the highest-value, durable, cross-consumer fix.
3. **Cross-impl golden-hash gate** (Sentinel CI: Pester on Windows + bats on Unix,
   golden in `tests/golden-bundle.sha256`): bash `sentinel_bundle_sha256` and the
   pwsh renderer over the same checkout are byte-identical and both equal
   `eff45a5e…`; a CRLF-synthesized `core/*.md` STILL hashes to the LF pin (proves
   `eol=lf`/normalization is load-bearing); the embedded stamp equals the raw sha
   (`eff45a5e…`) and NOT the embed-form `be4d1733…`; round-trip `-DryRun` vs
   `--dry-run` byte-match for append / replace-in-place / dup-collapse /
   unterminated-refuse / remove-keep / remove-delete; plus adversarial fence/heading
   fixtures and a core-lint forbidding tilde fences or ATX-in-fence.

## Honoring the dotfiles invariants

- **Tool, not chezmoi config.** Wire it into **install-deps** (the tool phase),
  NOT chezmoi — the entrypoints Sentinel writes are *agent config*, which dotfiles
  deliberately does not chezmoi-manage.
- **Never sync personal agent state.** dotfiles installs the Sentinel *tool* and
  runs `--global` (a generic marked block). It must NEVER vendor Sentinel content
  into dotfiles' own public history and NEVER sync the owner's real `~/.claude`
  settings/keys/allowlists. Proposed invariant addition:
  > *dotfiles installs the AI-rules tool (Sentinel) and lets `--global` inject a
  > generic marked block; it never syncs personal agent state and never vendors
  > rules into this repo's history.*
  This is the `claude/settings.json` PII-leak lesson, generalized.
- **Pin + verify.** Clone Sentinel at a pinned `SENTINEL_REF` (commit SHA), like the
  other pinned downloads; verify integrity with Sentinel's own bundle sha256 /
  `tools/verify-vendor` against a pinned `SENTINEL_BUNDLE_SHA256`. Renovate bumps
  the ref; a human reviews the new bundle hash (same pattern as the lazygit /
  tree-sitter pins). CI stays red until reviewed. **Starting pins (Codex-computed,
  re-verified here — `verify-vendor` reports "bundle MATCHES"):**
  `SENTINEL_REPO_URL=https://github.com/luisgui1757/sentinel.git`,
  `SENTINEL_REF=65c96982eb055cca3d2a2bcf86844ca902b76c53`,
  `SENTINEL_BUNDLE_SHA256=eff45a5e7dc888f3c92642ccf677f7d5564e77c642cee1051ae1e005b6d558c2`.
  Install root: `${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/sentinel` (Unix) /
  `%LOCALAPPDATA%\dotfiles\sentinel` (Windows). **Clone with `-c
  core.autocrlf=false -c core.eol=lf`** (on the clone and every later
  fetch/checkout) so `core/*.md` land LF regardless of the user's global git
  config — the dotfiles-side belt to Sentinel's `.gitattributes` suspenders. Flow:
  clone → checkout detached REF → assert `git rev-parse HEAD == SENTINEL_REF` →
  verify the bundle sha256 (`tools/verify-vendor` on Unix; on Windows re-render via
  the native pwsh renderer and compare to `SENTINEL_BUNDLE_SHA256` with the existing
  `Test-FileSha256` facility, fail-closed) → `tools/install --global` /
  `tools/install.ps1 -Global`.
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
- **Composition:** install Pi first, then Sentinel `--global` writes
  `~/.pi/agent/AGENTS.md`. Pi then auto-loads the rules at startup. No extra
  wiring — Sentinel already targets Pi.

## Implementation checklist (when built)

1. `install-deps.sh`: `install_pi_cli` (npm global, `$HOME/.local` prefix
   fallback when the global prefix is unwritable) + `install_sentinel`
   (clone @ pinned SHA → `~/.local/share/dotfiles/sentinel`, `verify-vendor`,
   then `bash tools/install --global`); consent-gated, dry-run-safe.
2. `install-deps.ps1`: `Install-PiCli` (npm, then `Add-DirectoryToUserPath` for
   the npm shim dir) + `Install-Sentinel` (clone with `-c core.autocrlf=false -c
   core.eol=lf` → `%LOCALAPPDATA%\dotfiles\sentinel`, checkout `--detach $SentinelRef`,
   assert `rev-parse HEAD == $SentinelRef`, re-render the bundle in pwsh and verify
   its SHA-256 == `$SentinelBundleSha256` via the existing `Test-FileSha256`
   facility, then `pwsh -NoProfile -NonInteractive -File <checkout>\tools\install.ps1
   -Global`); `-All`-gated/consent, DryRun-safe, best-effort + FAIL marker into
   `$script:InstallFailures`. **NO Git-Bash, NO `cygpath`, NO WSL-bash rejection.**
   Pi stays npm-only (NOT in `$Catalog`).
3. Pin constants: `SENTINEL_REF`, `SENTINEL_BUNDLE_SHA256`; renovate.json custom
   manager (version bumpable, SHA human-reviewed — context-only, NOT currentDigest).
   A lint asserts `$SentinelRef`/`$SentinelBundleSha256` (ps1) == `SENTINEL_REF`/
   `SENTINEL_BUNDLE_SHA256` (sh).
4. Tests: shell (`tests/shell/sentinel_install_test.sh`) + Pester
   (`INSTALL_DEPS_PS1_SOURCE_ONLY` seam) coverage — pinned-ref assert, fail-closed
   on a bad `$SentinelBundleSha256`, dry-run `would:` lines, and a regression guard
   that the pwsh path NEVER resolves or invokes `bash.exe`/`cygpath`. Plus the
   make-or-break **cross-implementation byte-parity gate** (lives in Sentinel CI; see
   the Sentinel-side prerequisite). Add `tools/install --check` / `-Check` as the
   post-`--global` idempotency drift gate on both OSes; container-e2e presence
   assert for `pi`.
5. Docs: CLAUDE.md invariant addition + a "what weird but intentional" note;
   README dependency mention.

## Open questions (a real Windows run will answer)

The byte-parity was validated against a byte-faithful reference (it reproduces both
`eff45a5e…` and the CRLF-divergence `0fb1c557…`), but pwsh could not be run in the
design sandbox. Before trusting parity in production:

- **A real Pester run on Windows** must reproduce the pwsh-rendered bundle sha256 ==
  `eff45a5e…` (and the CRLF-synthesized fixture must STILL hash to it).
- **.NET regex vs the bash ERE** on adversarial input (tab-vs-space after hashes,
  tilde fences, ```` ```lang ```` info-strings). Current `core/*.md` have zero such
  cases, so the surface is untested by real data — the parity test must ship the
  adversarial fixtures.
- **Composed/conflicting action-flag precedence** (`-Check` + `-Remove`): bash is
  last-wins; confirm the `$args` left-to-right port matches on a real run.
- **`$env:HOME` ≠ `$env:USERPROFILE`** on a box running both native pwsh and WSL
  bash — inherent to cross-OS global install; confirm the documented order matches
  expectations.
- **Atomic-rename / temp-create failure** on a network/UNC or permission-locked
  target dir — confirm the FAIL-marker path surfaces cleanly under the e2e gate.
- Whether `pi` lands on PATH cleanly per OS (package `@earendil-works/pi-coding-agent`,
  binary `pi`). Best-guess: `$HOME/.local` prefix fallback on Linux non-root +
  `Add-DirectoryToUserPath` on Windows — both in the checklist.
- Whether to also `tools/install --target ~/dotfiles` so the dotfiles repo itself
  carries the rules for contributors (orthogonal, optional, one-time + commit).
- Renovate hashing of `SENTINEL_BUNDLE_SHA256` (context-only match, like the other
  direct-download SHAs).

## Recommendation

Do it on a **fresh branch after the pilot migration branch merges**, and land the **Sentinel-side
prerequisite first** (native `tools/install.ps1` + `.gitattributes` `*.md eol=lf` +
the cross-impl golden-hash gate). Canonical shape: **install Pi via npm; clone
Sentinel at a pinned SHA with a forced-LF checkout; verify the bundle sha256; run
`tools/install --global` on Unix and `tools/install.ps1 -Global` on Windows (no
Git-Bash); idempotent with a `--check` drift gate; never vendor, never sync personal
agent state.**
