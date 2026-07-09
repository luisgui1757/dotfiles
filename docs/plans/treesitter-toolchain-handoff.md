# Handoff: nvim-treesitter `main` parser-build toolchain

Status: **HISTORICAL REFERENCE.** This handoff records the investigation that
rejected the "zig fixed tree-sitter" hypothesis. The current implementation has
since moved beyond the original open-questions state: setup now has five phases
with an explicit synchronous Tree-sitter parser install phase, the sync path
requires the waitable `nvim-treesitter` install task to return `true`, and the
current canonical contract lives in `CLAUDE.md` plus `README.md`.

Keep this document for provenance, but do not treat the older open-item tables
below as the live roadmap without first checking the current installer, tests,
and docs.

> **Resume on the Codex side:** the audit spec is checked in below
> ([§9](#9-how-to-continue-on-the-codex-side)); re-run with
> `codex exec -C "$(pwd)" -m gpt-5.5 -c model_reasoning_effort="xhigh" -s read-only ...`.

## 1. Scope and the question that triggered this

nvim-treesitter is pinned to `branch = "main"` (`nvim/lua/plugins/treesitter.lua:46`).
On `main`, parser install does NOT ship prebuilt parsers — it shells out to the
standalone **`tree-sitter` CLI** (`tree-sitter generate` + `tree-sitter build`), and
`tree-sitter build` compiles the generated C via the Rust **`cc` crate**, which needs
a real C compiler. So `main` has TWO runtime prerequisites the old `master` did not:
(1) the `tree-sitter` CLI on PATH, and (2) a working C compiler the cc crate accepts.

The trigger: on a fresh macOS (Apple Silicon, tart VM) run the user saw ~10 lines of
`[nvim-treesitter/install/<parser>] error: Error during "tree-sitter build": ...
ENOENT: no such file or directory (cmd): 'tree-sitter'`. The hypothesis to test was
"it was mostly solved by having a zig compiler." **That is wrong** — see
[§3](#3-the-macos-incident--root-cause) and [§4](#4-the-zig-clarification).

## 2. The toolchain map (per-OS), with file:line

| OS | `tree-sitter` CLI | C compiler (cc crate) | On PATH for the headless setup via |
|---|---|---|---|
| **macOS** | `brew install tree-sitter-cli` — the logical tool remains `tree-sitter`, but the Homebrew formula is `tree-sitter-cli` because `tree-sitter` no longer ships the CLI binary | cc/clang from Xcode CLT | `setup.sh` `refresh_runtime_path` runs `eval "$(brew shellenv)"` → `/opt/homebrew/bin` (ARM) / `/usr/local/bin` (Intel) |
| **Linux/WSL** | pinned GitHub release binary, SHA-256 verified, into `~/.local/bin` — `install_tree_sitter_cli_linux` (`install-deps.sh:1048`), constants `TREE_SITTER_CLI_LINUX_*` (`:29-31`). Alpine → `apk` (`:1147`) | `build-essential`/`gcc` (`install-deps.sh:1583-1599`) | `refresh_runtime_path` adds `$HOME/.local/bin` |
| **Windows** | scoop `tree-sitter` (catalog `:258`) via `Install-TreeSitterCli` (`install-deps.ps1:1290`); **npm `tree-sitter-cli` fallback** (`:1314`) records a FAIL marker if npm is missing (`:1310`) | **MSVC / VS Build Tools (VCTools)** — `Install-VsBuildTools` (`install-deps.ps1:1354`), `Install-VsBuildToolsWhenAll` (`:1397`, called `:1571`) | `setup.ps1` `Enter-VsDeveloperEnvironment` (`:89`) imports the VS DevShell before `Invoke-NvimSyncPhases` (`:869`, called `:1017`) so `cl.exe` resolves |

setup.sh phase order is now: Phase 1 `install-deps.sh` → `refresh_runtime_path` →
Phase 2 chezmoi → **Phase 3 `nvim --headless +Lazy! restore`** → **Phase 4
`DOTFILES_TREESITTER_SYNC_INSTALL=1 nvim ... require('lazy').load({ plugins =
{ 'nvim-treesitter' } })`** → Phase 5 Mason. Phase 4 is the explicit proof path:
it blocks on `install(...):wait(...)` and fails unless the waitable task reports
`true`.

## 3. The macOS incident — root cause

**Not a compiler problem; the *specific incident* was a transient PATH artifact —
but the toolchain has real latent installer gaps the audit surfaced (see
[§7](#7-adversarial-review-codex-55-xhigh)).** The error is `ENOENT (cmd):
'tree-sitter'` — the OS could not find the `tree-sitter` **executable** on the nvim
process's PATH. That is a step BEFORE any compilation, so no compiler (zig or
otherwise) is relevant to it.

Post-incident diagnostic on the same VM: `tree-sitter 0.26.9` installed (brew),
`/opt/homebrew/bin` on PATH, login shell `/bin/zsh`. So the CLI **was** present and
resolvable at diagnosis. The most plausible cause is a **transient first-run window**:
nvim attempted parser compilation (interactively, or via Lazy auto-sync on first
launch) in a moment when `/opt/homebrew/bin` was not yet on that process's PATH —
e.g. an nvim launched before the install finished, or before a fresh login shell
sourced `brew shellenv`. A clean `./setup.sh` avoids it because `refresh_runtime_path`
(`:384`) puts the brew bin on PATH before the Phase 4 parser install.

Why it was confusing rather than catastrophic: nvim-treesitter `main` emits one ENOENT
**per parser** (~10 lines) instead of a single "CLI missing" message.

## 4. The zig clarification

- `zig` in this repo is for the **LuaSnip jsregexp build**, NOT tree-sitter — the
  catalog entry says so verbatim (`install-deps.ps1:260`: "C compiler for the LuaSnip
  jsregexp build"); `make` likewise (`:254`). It only appears in tree-sitter's orbit
  as one candidate in the compiler-detection loops `for compiler in cc gcc clang zig cl`
  (`install-deps.sh:1566`, `:2066`).
- **zig does NOT work as the `tree-sitter build` compiler on nvim-treesitter `main`.**
  The maintainer (discussion #7920) says `main` relies entirely on `tree-sitter build`
  + the cc crate and recommends only VS Build Tools on Windows; on a windows-msvc-target
  tree-sitter CLI the cc crate emits MSVC-style flags zig cannot consume (issues #8147 /
  #6546). So zig is deliberately NOT wired into the tree-sitter path, and the Windows
  compiler is MSVC (`Install-VsBuildTools`), not zig.
- Net: the user's "zig fixed it" belief conflates two unrelated things. The macOS error
  was "CLI not on PATH"; zig is for LuaSnip; on `main` zig would not even build parsers.

## 5. What shipped (commit `f80e128`)

A **legibility guard** in `treesitter.lua` `config` (`:62-74`):

```lua
if vim.fn.executable("tree-sitter") == 1 then
  nvim_treesitter.install(treesitter_parsers)
else
  vim.schedule(function() vim.notify("nvim-treesitter: 'tree-sitter' CLI not found ...", WARN) end)
end
```

It turns N per-parser ENOENTs into ONE actionable, cross-OS message. It did **not**
change any installer — the healthy path (CLI present) is unchanged. Regression test:
`tests/nvim/spec/treesitter_spec.lua` asserts the `executable("tree-sitter")` guard and
the `vim.notify` fallback. CI-green on `f80e128` (both `test` and `e2e-install`).

## 6. Historical asymmetries / candidate hardening

This section is historical. Several items below have since been superseded by
the five-phase setup and stricter e2e/Tier-2 smoke gates; verify against current
source before treating any line as still open.

1. **macOS failure is quieter than Linux/Windows.** A failed/declined `brew install
   tree-sitter` on macOS only prints `WARN: ... continuing` (the generic `install`
   helper, `install-deps.sh`), with **no FAIL marker**, whereas the Linux pinned-download
   path emits `FAIL:` on download/checksum failure and the Windows npm fallback records
   `$script:InstallFailures`. A macOS user could end up with no CLI and a green-looking
   setup. Candidate: a post-install verification (`have tree-sitter` after
   `install_tree_sitter_cli` on brew) that emits a FAIL marker, mirroring the other OSes.
2. **No pre-Phase-4 PATH assertion.** setup.sh does not assert `tree-sitter` is on PATH
   before the Phase 4 parser install. Candidate: a guard in setup.sh that, when nvim is on
   PATH but `tree-sitter` is not, emits a clear FAIL/skip instead of letting the parser
   install dump the ENOENT spam.
3. **Async compile vs `+qa`.** Superseded for the dedicated parser phase: Phase 4
   sets `DOTFILES_TREESITTER_SYNC_INSTALL=1`, waits on the install task, and treats
   a non-`true` result as failure. Lazy's own `build = ":TSUpdate"` remains a plugin
   update hook; Phase 4 is the canonical parser proof.
4. **Guard is one-shot.** Under lazy, `config` runs once; if the guard fires (CLI absent),
   there is no in-session recovery — the user must fix PATH and restart nvim (or run
   `:TSUpdate`). The message says so, but a `:checkhealth` entry could make it discoverable.

## 7. Adversarial review (Codex 5.5 xhigh)

A read-only Codex 5.5 xhigh audit ran against the pinned tree (CODEX_EXIT=0, empty
`git diff` confirmed). **Claude independently re-verified every load-bearing claim
against the source before recording it here** (per the Codex-executes / Claude-reviews
workflow). Verdicts:

**Hypotheses.** (1) "zig fixed it" — **REFUTED**, agreeing with [§4](#4-the-zig-clarification):
the error is CLI-not-on-PATH, which precedes any compile; zig is for LuaSnip and can't
build parsers on `main`. (2) macOS ENOENT — **transient PATH/timing, but a real
installer gap exists.** (3) `refresh_runtime_path` — **conditional, not end-to-end**:
it can put the brew bin on PATH, but never proves `tree-sitter` is installed before the
Phase 3 sync.

**Confirmed gaps (Claude-verified, ranked):**

| # | Severity | Gap | Evidence (verified) | Canonical fix |
|---|---|---|---|---|
| 1 | breaks-install | **macOS brew tree-sitter failure looks green** — no FAIL marker | `install-deps.sh:1648` `pm_install $pkg \|\| echo "  WARN: ... continuing"`; contrast the Linux hard `FAIL:` at `:1100`/`:1110` | In `install_tree_sitter_cli`, post-check `have tree-sitter` after the brew install; emit a `FAIL:` marker on miss, mirroring the Linux path |
| 2 | breaks-install | **Phase 3 sync gates on `nvim`, not `tree-sitter`** | `setup.sh:424` `if command -v nvim` (no `tree-sitter` check) | Add a pre-Phase-3 `tree-sitter`-on-PATH assertion in `setup.sh` + `setup.ps1`; downgrade only under explicit `--best-effort` |
| 3 | silent-degradation | **the `f80e128` guard does not cover Lazy's `build = ":TSUpdate"`** — only the config-time `install` | guard at `treesitter.lua:62`; `build` is unconditional at `:47` | Keep the guard for UX; put the real gate in setup (fail before Lazy when CLI/compiler absent) |
| 4 | silent-degradation (tempered) | **Windows VS Build Tools failure is not in the structured `$script:InstallFailures`** | `install-deps.ps1:1394` prints a `FAIL:` *marker string* (so the e2e marker-grep DOES catch it) but does not push to the structured array | Also record it in `$script:InstallFailures` under `-All` for the end-of-run summary. **Note:** Codex rated this higher; the printed `FAIL:` marker means it is not actually silent under the e2e gate |
| 5 | silent-degradation | **`zig` satisfies the generic compiler scan but can't build tree-sitter `main`** | detection loops `cc gcc clang zig cl` at `install-deps.sh:1566`/`:2066`; `main` needs cc/clang/MSVC, not zig | Split the LuaSnip-compiler detection from a tree-sitter-compiler-readiness check (don't count zig for tree-sitter). Low real-world hit rate (macOS has clang, Linux gcc, Windows MSVC) |

**Theoretical (no current repro, keep on the radar):** the async
`nvim_treesitter.install(...)` racing headless `+qa`; CLI/parser version skew
(Linux pins CLI v0.26.10, macOS/Windows take the package-manager version while the
plugin commit is lockfile-pinned).

**Guard verdict (agreed):** `f80e128` is the right *legibility* fix but not the
canonical *correctness* fix. The canonical fix is in setup/install — verify CLI **and**
compiler before Phase 3 and fail loudly unless best-effort is explicit. Gaps 1 and 2
are the highest-value, lowest-risk install hardening.

**Doc/test debt the audit flagged (valid):** `CLAUDE.md` documents the CLI+compiler
invariant but not a hard fail before Phase 3; `treesitter_spec.lua` is static-only
(asserts the guard exists, doesn't prove setup fails when the CLI/compiler is missing).

## 8. Concrete next steps (if picked up)

1. **Ship gap 1** (macOS `FAIL:` marker when `tree-sitter` is absent post-install) — the
   highest-value, lowest-risk fix; add a shell test stubbing a failed `brew install
   tree-sitter` and asserting the marker fires.
2. **Ship gap 2** (pre-Phase-3 `tree-sitter`-on-PATH assertion in `setup.sh` + `setup.ps1`,
   honoring `--best-effort`). This is the canonical correctness fix; the `f80e128` guard
   stays as the UX layer.
3. Consider gap 5 (split LuaSnip vs tree-sitter compiler readiness) and gap 4 (structured
   Windows VSBuildTools failure record) as follow-ups.
4. Update the `CLAUDE.md` treesitter invariants (document the hard-fail-before-Phase-3
   contract) and the `treesitter-main-toolchain` memory in the same change.
5. Re-validate the full Windows path (scoop CLI → DevShell → MSVC build) and the Linux
   pinned-download path with the real `make test` / `.\test.ps1` / e2e gates before any
   commit — never ship a parser-toolchain change without them.

## 9. How to continue on the Codex side

The adversarial audit spec lives at `/tmp/codex-ts-audit-spec.txt` (not checked in —
reproduce from §1-6 if needed). Dispatch pattern (per the
`codex-execution-with-claude-review` workflow — Codex executes/audits, Claude reviews and
re-runs real tests before any commit):

```bash
codex exec -C "$(pwd)" -m gpt-5.5 -c model_reasoning_effort="xhigh" \
  -s read-only -c approval_policy="never" "$(cat /tmp/codex-ts-audit-spec.txt)" </dev/null
```

For an implementation pass (not audit), switch `-s read-only` → `-s workspace-write`, and
have Claude review the diff + run `make test` (and the Windows `.\test.ps1` / e2e) before
committing. Never ship a parser-toolchain change without the real cross-OS test gates.
