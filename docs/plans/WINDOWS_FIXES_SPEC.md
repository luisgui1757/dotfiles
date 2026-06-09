# Windows Fixes — Codex-ready Implementation Spec (W1–W3)

_Companion to `docs/plans/CODEX_EXECUTION_PLAN.md` (the master sequencing) and a sibling of
`docs/plans/CHEZMOI_WAVE_A_SPEC.md` (same house format). This spec turns the three owner-found Windows
items into something **Codex 5.5 xhigh** can execute literally, grounded line-by-line in the real
`luisgui1757/dotfiles` tree._

The three items:

| ID | Item | One-line gap |
|----|------|--------------|
| **W1** | Windows Terminal Rose Pine | The fragment already *has* Rose Pine; the merge that applies it is opt-in (`-MergeWindowsTerminal`), so a default install never themes WT. |
| **W2** | PowerShell via scoop (keep latest) | `pwsh` is *already* scoop-first in `$Catalog`; the missing half is **"keep latest"** (no `scoop update`) plus a missing catalog regression guard. |
| **W3** | psmux sporadic "auth failed" | `scoop bucket add psmux <github>` git-clones with interactive credential prompts enabled and exit-code blind, so it sporadically hangs/fails; the chezmoi port inherits the same line. |

## How to use this spec (Codex 5.5 xhigh)

- Execute **top to bottom** per section. Each section gives **paste-ready edits keyed to a `file:line`
  anchor**, the tests to add, the acceptance checklist, the docs to update, and the **chezmoi delta**
  (the exact change to `CHEZMOI_WAVE_A_SPEC.md` so the migration inherits the fix).
- **Do not fabricate line numbers.** Every anchor below was read from the working tree at **HEAD
  `96d85ee`**. Line numbers drift the moment you start editing a file — re-resolve each anchor against
  the current buffer before applying the next edit in the same file. If an anchor no longer matches
  what's quoted here, **STOP and surface the drift**; do not guess.
- **W2 and W3 both edit `install-deps.ps1`.** Apply them in one branch or sequence W3→W2 to avoid a
  line-number collision (see `CODEX_EXECUTION_PLAN.md` ordering table).
- Windows-only behavior has **limited CI reach**. Where a check only runs on the `windows-2025` lane
  (or only by hand), the section says so explicitly — do not over-claim coverage.

## Verification status

- **Grounded** against the worktree at HEAD `96d85ee` (the same commit `CHEZMOI_WAVE_A_SPEC.md` was
  grounded against). Every `file:line` was read from source.
- **Adversarial pass:** the three sections were produced by per-item grounding agents and cross-checked
  against an independent read of `setup.ps1`, `bootstrap.ps1`, `install-deps.ps1`, and the `tests/`
  fixtures. Load-bearing anchors (`bootstrap.ps1:470` merge close, `ps1_test.ps1:204-214` rose-pine
  test, `InstallDeps.Tests.ps1:218-234` catalog-guard pattern, `repo_policy_test.sh`/`ps1_parse.sh`
  existence) were re-verified.

---

## W1 — Windows Terminal Rose Pine

### Gap & root cause

The Rose Pine definition is **not** missing: the fragment carries the top-level `theme="rose-pine"`,
`profiles.defaults.colorScheme="rose-pine"`, `schemes[rose-pine]`, and `themes[rose-pine]`
(`windows-terminal/settings.fragment.jsonc:28,52,64,89`), and the merge engine applies them faithfully
(`bootstrap.ps1:448-463`). The bug is **flag-plumbing**: the WT merge is gated behind the opt-in
`-MergeWindowsTerminal` switch (`setup.ps1:26`, `bootstrap.ps1:294`), and `-All` deliberately forwards
only `DryRun` to `bootstrapArgs` (`setup.ps1:122-124`). So the dominant invocations — `.\setup.ps1
-All`, the bare interactive run, and the no-TTY auto-`-All` (`setup.ps1:61-65`) — install the WT app
but never write `settings.json`, leaving Windows Terminal on its stock light scheme.

The fix flips the merge to **default-on with an opt-out** (`-SkipWindowsTerminalMerge`), keeps
`-MergeWindowsTerminal` as a back-compat no-op alias, and amends CLAUDE.md invariant 15. It is
fail-safe: the merge warns-and-skips when `settings.json` is absent (`bootstrap.ps1:300-301`), backs up
before writing (`bootstrap.ps1:312-316`), and merges arrays **by name** rather than overwriting
(`bootstrap.ps1:344-447`).

> **Ordering caveat (don't skip):** the merge requires an existing `settings.json` — Windows Terminal
> writes that file on first launch. On a freshly-installed-but-never-launched WT, default-on correctly
> **warns and skips** (it does not fabricate a file). That is the accepted behavior for Wave A; a
> future "seed a minimal settings.json so first-run is themed" enhancement is out of scope here and is
> noted as an open item in `CODEX_EXECUTION_PLAN.md`.

### Fix — five paste-ready edits

**1. `setup.ps1:26` — param block.** Replace:

```powershell
    [switch]$MergeWindowsTerminal,
```

with:

```powershell
    [switch]$MergeWindowsTerminal,   # back-compat no-op: WT merge is now default-on
    [switch]$SkipWindowsTerminalMerge,
```

**2. `setup.ps1:10` — header usage comment.** Replace:

```powershell
#   .\setup.ps1 -MergeWindowsTerminal     also merge the WT rose-pine fragment
```

with:

```powershell
#   .\setup.ps1 -SkipWindowsTerminalMerge   bootstrap+sync but leave WT settings.json untouched
#   .\setup.ps1 -MergeWindowsTerminal        (no-op alias; the WT rose-pine merge is now default-on)
```

**3. `setup.ps1:122-124` — the `bootstrapArgs` assembly.** Replace exactly:

```powershell
$bootstrapArgs = @{}
if ($DryRun)               { $bootstrapArgs['DryRun']               = $true }
if ($MergeWindowsTerminal) { $bootstrapArgs['MergeWindowsTerminal'] = $true }
```

with:

```powershell
$bootstrapArgs = @{}
if ($DryRun)                   { $bootstrapArgs['DryRun']                   = $true }
# WT settings merge is now a DEFAULT bootstrap step (opt-out, not opt-in).
# -MergeWindowsTerminal is retained as a harmless no-op alias for back-compat.
$null = $MergeWindowsTerminal  # reference the alias so PSScriptAnalyzer doesn't flag it unused
if ($SkipWindowsTerminalMerge) { $bootstrapArgs['SkipWindowsTerminalMerge'] = $true }
```

> The `$null = $MergeWindowsTerminal` line is **defensive, not a CI gate**: with the old
> `if ($MergeWindowsTerminal)` forward removed, the alias parameter is otherwise unreferenced. This
> repo's CI does **not** run PSScriptAnalyzer over `setup.ps1`/`bootstrap.ps1` (the only PSSA scans are
> `test.ps1` + `Profile.Tests.ps1`, both targeting `shells/powershell_profile.ps1`), so an unused
> parameter here would NOT fail CI. Keep the `$null =` consumption anyway for editor/IDE lint hygiene
> and in case the analyzer scope is ever broadened — just don't justify it with a false CI claim.

**4. `bootstrap.ps1:10-14` — param block.** Replace exactly:

```powershell
[CmdletBinding()]
param(
    [switch]$DryRun,
    [switch]$MergeWindowsTerminal
)
```

with:

```powershell
[CmdletBinding()]
param(
    [switch]$DryRun,
    [switch]$MergeWindowsTerminal,        # back-compat no-op alias; merge is default-on
    [switch]$SkipWindowsTerminalMerge
)
```

**5. `bootstrap.ps1:294` — the merge gate.** Replace exactly the single line `if ($MergeWindowsTerminal) {`:

```powershell
# WT settings merge: DEFAULT-ON. Skips itself safely when settings.json does not
# exist yet (WT never launched) -- see the not-found warn below. Opt out with
# -SkipWindowsTerminalMerge. -MergeWindowsTerminal is a retained no-op alias.
$null = $MergeWindowsTerminal   # referenced so the back-compat switch is not flagged unused
if (-not $SkipWindowsTerminalMerge) {
```

> The closing `}` for this block is at **`bootstrap.ps1:470`** — leave it untouched. The entire merge
> body (`bootstrap.ps1:295-469`: candidate-path resolution, warn-and-skip, backup, the six helper
> functions, the `:448-463` merge sequence) is **unchanged** — only the gate flips.

### Tests

All new tests go in `tests/bootstrap/ps1_test.ps1`, reusing the existing `Describe "bootstrap.ps1
-MergeWindowsTerminal"` fixture (`:149`; `BeforeEach` seeds `$script:WTSettings` under a confined
`$FakeHome`). Add `It`s that drive `& $script:Bootstrap` **without** any WT switch:

- [ ] **Default-on merges (the test that fails on today's code, passes after).** Seed the minimal
      `'{"profiles":{"defaults":{},"list":[]},"actions":[],"schemes":[],"themes":[]}'` (mirror
      `:205-206`), invoke `& $script:Bootstrap | Out-Null` with **no** switch, then assert exactly as
      `:211-213` plus the colorScheme: `$merged.theme | Should -Be 'rose-pine'`; the named `schemes`/
      `themes` entries `Should -Not -BeNullOrEmpty`; `$merged.profiles.defaults.colorScheme | Should -Be
      'rose-pine'`.
- [ ] **`-SkipWindowsTerminalMerge` is a byte-identical no-op.** Seed a settings.json, capture
      `(Get-FileHash $script:WTSettings).Hash`, run `& $script:Bootstrap -SkipWindowsTerminalMerge`,
      assert the hash is unchanged and no `settings.json.bak.*` was written.
- [ ] **Legacy alias still merges.** The existing `:204-214` test invokes `& $script:Bootstrap
      -MergeWindowsTerminal`; it must stay green (the alias is a no-op that does not suppress the
      now-default merge). Retitle the parent `Describe` to `"bootstrap.ps1 WT merge"` or add an explicit
      `It "still merges when the legacy -MergeWindowsTerminal alias is passed"`.
- [ ] **Missing settings.json: warn-and-skip, no throw, no fabrication.** In a profile that does **not**
      seed `$script:WTSettings`, run `{ & $script:Bootstrap } | Should -Not -Throw` and assert
      `Test-Path $script:WTSettings | Should -BeFalse` (the `:300-301` warn path holds under default-on).
- [ ] **Static lint (local/manual only — NOT a CI gate):** if you run PSScriptAnalyzer over `setup.ps1`
      / `bootstrap.ps1` locally, the `$null = $MergeWindowsTerminal` references keep it clean of
      `PSReviewUnusedParameter`. Note this repo's CI does **not** analyze these two files (the only
      `Invoke-ScriptAnalyzer` calls target `shells/powershell_profile.ps1`), so this is editor hygiene,
      not an automated gate.

**CI reach — be honest.** Run `.\test.ps1` (PSScriptAnalyzer + Pester) on Windows. **The seeded Pester
tests are the only automated proof of the live merge.** The hosted `windows-2025` e2e runner installs
WT but never launches it, so `LocalState/settings.json` does not exist during e2e — a real `.\setup.ps1
-All` there exercises only the default-on **warn-and-skip** path (proving "default-on does not crash
setup", not "Rose Pine reached WT"). On macOS/Linux, `make test-static` cannot exercise the pwsh merge
at all. Do not claim e2e covers the live merge.

### Acceptance

- [ ] `.\setup.ps1 -All` on a machine that has launched WT at least once leaves `settings.json` with
      `theme=rose-pine`, `profiles.defaults.colorScheme=rose-pine`, and named `schemes[rose-pine]` +
      `themes[rose-pine]`, with the pre-merge file backed up to `settings.json.bak.<timestamp>`.
- [ ] `.\setup.ps1 -All -SkipWindowsTerminalMerge` leaves an existing `settings.json` byte-identical.
- [ ] `.\bootstrap.ps1 -MergeWindowsTerminal` still merges (back-compat alias).
- [ ] On a machine where WT was never launched, `.\setup.ps1 -All` prints `Windows Terminal
      settings.json not found; skipping merge.` and exits 0 — no file fabricated.
- [ ] `.\test.ps1` green: the new default-on test passes, the existing `-MergeWindowsTerminal` tests
      still pass, PSScriptAnalyzer reports no new Warning/Error.

### Docs to update (same change)

- **`CLAUDE.md` invariant 15 (REQUIRED amendment).** This fix changes the invariant. Replace the
  invariant-15 bullet's "The app install and `-MergeWindowsTerminal` settings merge are separate steps."
  ending with language stating the app install and the merge are still **separate code paths**
  (`install-deps.ps1` vs `bootstrap.ps1`) but the merge now runs **by default** during bootstrap; opt
  out with `-SkipWindowsTerminalMerge`; `-MergeWindowsTerminal` is a retained no-op alias; the merge
  fail-safes (warn + skip) when `settings.json` is absent so default-on never breaks an unlaunched WT.
  Also fix the "Bootstrap details" line that says to run `.\bootstrap.ps1 -MergeWindowsTerminal`.
- **`README.md`** (the lines telling users to pass `-MergeWindowsTerminal` for Rose Pine): state the
  merge runs by default on any `.\setup.ps1` run, document `-SkipWindowsTerminalMerge`, and note the
  surprise that a hand-edited `theme` is reset to `rose-pine` on every run (merge is idempotent-by-name;
  pre-merge file is backed up).
- **`windows-terminal/README.md`**: replace the "run `bootstrap.ps1 -MergeWindowsTerminal`" instruction
  with the default-on description; keep the "read-modify-write merge, never a symlink" framing.
- **`PLAN.md` / `tests/MANUAL.md`**: update any step that invokes `-MergeWindowsTerminal` as *the* way
  to theme WT; keep alias mentions only as back-compat notes.

### chezmoi delta

Two edits to `docs/plans/CHEZMOI_WAVE_A_SPEC.md`. The merge **body** the spec ports verbatim (the six
helpers `bootstrap.ps1:329-447` + the `:448-463` sequence) is untouched — only the *default-ness*
converges, which is parity, not a behavior change.

1. **DC-3 Step 2 preamble.** The chezmoi `modify_settings.json.ps1.tmpl` is already default-on by
   construction (a `modify_` script runs on every `chez apply`). Add a sentence recording that the
   **legacy** `bootstrap.ps1` merge is now *also* default-on, so the chezmoi `modify_` default-on
   behavior is intentional **parity**, not a new divergence.
2. **DC-6 Step 4, the WT parity probe.** Change the OLD-side command from `bootstrap.ps1
   -MergeWindowsTerminal` to `bootstrap.ps1` **with no switch** (note `-MergeWindowsTerminal` is an
   accepted back-compat alias). Leave the deep-compare assertions unchanged. This must land in the
   legacy repo **before/alongside** the migration so the DC-6 harness keeps invoking a command that
   actually performs the merge.

---

## W2 — PowerShell via scoop (keep latest)

### Gap & root cause

The "install-via-scoop" half is **already shipping**: `pwsh` is a first-class `$Catalog` row with
`scoop = 'pwsh'` (`install-deps.ps1:123`), it is in `$BinaryName` (`install-deps.ps1:142`), `Install-One`
tries scoop first via its ordered `@('scoop', $Pm, 'winget', 'choco')` candidate list
(`install-deps.ps1:179`), and the `modern shell` section invokes it (`install-deps.ps1:467`). The
CLAUDE.md claim that "install-deps.ps1 owns installing PowerShell 7" is accurate, not aspirational.

Two real gaps remain:

1. **"keep latest" has no implementation.** `Install-One` is strictly install-if-missing
   (`install-deps.ps1:164-167`) and the repo runs no `scoop update` anywhere, so `pwsh` pins to whatever
   scoop first fetched.
2. **No regression test guards the `pwsh` catalog entry** — the lazygit/wt pattern
   (`InstallDeps.Tests.ps1:218-234`) was never extended to `pwsh`, even though it is load-bearing for the
   psmux overlay (`tmux/tmux.windows.conf` `default-shell pwsh`) and chezmoi's default `.ps1`
   interpreter (`pwsh -NoLogo -File`).

### Fix

**Install half — NO CODE CHANGE.** Confirm, do not re-add. `install-deps.ps1:123` and `:467` already
exist and are correct (`scoop = 'pwsh'` resolves PowerShell 7 from the scoop **main** bucket — no
bucket-add needed). Do **not** add a second `$Catalog` row or a duplicate `Install-One pwsh` call.

**"keep latest" — RECOMMENDED: implement the consent-gated single-tool updater (option b2).** The owner
**explicitly** asked for "keep latest", so do implement it. The objection that a `scoop update` breaks
the "run twice = zero diffs" contract does **not** apply to a scoped single-package update: `scoop update
pwsh` is a no-op when pwsh is already latest, and only acts when upstream genuinely published a newer
version — which is the definition of "keep latest". The contract this MUST avoid is `scoop update *`
(it would silently upgrade `taplo`/`win32yank`/`node`/fonts outside scope). Add this function after
`Install-One` (after `install-deps.ps1:229`):

```powershell
# ---- Optional: keep a single scoop tool current ------------------------------
# scoop pins to the installed version until `scoop update <pkg>`. This is the
# explicit, consent-gated, idempotent "keep latest" step for ONE tool. We do NOT
# run `scoop update *` -- that would upgrade every scoop tool (taplo, win32yank,
# nerd-fonts, ...) outside the caller's intent and break the "run twice = no-op"
# contract. Safe to call when the tool is absent (the install path owns that) and
# when the tool was installed by another manager (scoop update then no-ops/warns).
function Update-ScoopTool {
    param([string]$tool)
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) { return }
    if (-not (Test-Tool $tool)) { return }   # not installed yet -> install path owns it
    $pkg = $Catalog[$tool].scoop
    if (-not $pkg) { return }
    # Only update if scoop actually manages this tool (avoids warning on a
    # winget/choco-installed pwsh that Install-One picked when scoop was absent).
    $managed = (scoop list $pkg 2>$null | Select-String -SimpleMatch $pkg)
    if (-not $managed) { return }
    if (-not (Ask "Update ${tool} to the latest scoop version?")) { return }
    if ($DryRun) {
        Write-Host ("  would:    scoop update; scoop update {0}" -f $pkg)
        return
    }
    scoop update | Out-Null               # refresh manifests only
    scoop update $pkg
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("  updated   {0,-26} via scoop" -f $tool)
    } else {
        Write-Warning ("  scoop update of {0} failed (exit {1})" -f $pkg, $LASTEXITCODE)
    }
}
```

and its call site — replace `install-deps.ps1:467` (`Install-One pwsh`) with:

```powershell
Install-One pwsh
Update-ScoopTool pwsh
```

> **Alternative (b1, if the owner overrides):** do nothing for keep-latest and simply record "keep
> latest = install-if-missing only, consistent with all catalog tools" in `PLAN.md`. Choose b1 **only**
> if the owner explicitly prefers strict parity with every other tool over the literal "keep latest"
> ask. The default for Codex is **b2** (it matches the written requirement).

**Regression test — REQUIRED.** In `tests/powershell/InstallDeps.Tests.ps1`, insert after the
"registers Windows Terminal in the Windows package catalog" block (ends ~`:234`), before the `}` that
closes the `Describe`, mirroring the lazygit/wt guards (`:218-234`):

```powershell
    It "registers PowerShell 7 (pwsh) in the Windows package catalog" {
        . $script:ImportInstallDepsForTest

        $Catalog.ContainsKey('pwsh') | Should -BeTrue
        $BinaryName['pwsh'] | Should -Be 'pwsh'
        $Catalog['pwsh'].scoop | Should -Be 'pwsh'
        $Catalog['pwsh'].winget | Should -Be 'Microsoft.PowerShell'
        $Catalog['pwsh'].choco | Should -Be 'powershell-core'
    }
```

If b2 is adopted, also add a dry-run test asserting `Update-ScoopTool pwsh` emits exactly one
`would: ... scoop update pwsh` line and **no** `scoop update *` anywhere.

### Tests

- **REQUIRED — catalog guard (Windows-only reach).** `tests/powershell/InstallDeps.Tests.ps1`. Asserts
  the `pwsh` row + `BinaryName`. Runs only on the Windows CI leg (`.\test.ps1`), same reach as the
  lazygit/wt guards — acceptable. Expect the suite total to increase by exactly 1.
- **Install-path proof (manual, Windows host).** `.\install-deps.ps1 -DryRun` with scoop present and no
  pwsh → under `== modern shell ==`, expect `would:    scoop install pwsh`.
- **Package-name proof (external, done).** `scoop info pwsh` resolves from the main bucket and installs
  PowerShell 7.x exposing `pwsh.exe`. No bucket-add needed (`main` is implicit).
- **b2 only.** `.\install-deps.ps1 -DryRun -All` with pwsh already scoop-managed → one
  `would: scoop update pwsh`, and **no** `scoop update *`.

### Acceptance

- [ ] Install-via-scoop confirmed DONE with citations `install-deps.ps1:123` + `:467`; no duplicate
      `$Catalog['pwsh']` row and no duplicate `Install-One pwsh` introduced.
- [ ] **keep-latest implemented (b2):** `Update-ScoopTool pwsh` is consent-gated (`Ask`), dry-run-safe,
      scoped to the single `pwsh` package, reuses the `Get-Command scoop` guard, and no-ops on a
      non-scoop-managed pwsh. (Or, if owner chose b1, the decision is recorded in `PLAN.md`.)
- [ ] `tests/powershell/InstallDeps.Tests.ps1` has a passing test asserting `$Catalog['pwsh'].scoop -eq
      'pwsh'` (+ winget/choco/BinaryName).
- [ ] **No `scoop update *`** anywhere in the repo.
- [ ] The psmux overlay dependency (`tmux/tmux.windows.conf` `default-shell pwsh`) and the
      `bootstrap.ps1` "install via install-deps.ps1 first" warning remain satisfied — no regression.

### Docs to update

- **`PLAN.md` (REQUIRED).** Mark "install PowerShell via scoop (keep latest)" DONE; cite
  `install-deps.ps1:123` (catalog, `scoop = 'pwsh'`) and `:467` (`Install-One pwsh`, scoop-first); record
  the keep-latest decision (b2 `Update-ScoopTool pwsh`, or b1 if overridden).
- **`CLAUDE.md`** — if b2, add one line under "Things that look weird but are intentional" stating
  `Update-ScoopTool` is the **only** scoop update path and is single-package + consent-gated (never
  `scoop update *`), so a future reader doesn't "fix" it into a blanket upgrade. No invariant change.
- **`README.md`** — no change.

### chezmoi delta

This is a **legacy-path item, Wave-B-deferred** for chezmoi. The `pwsh` package install belongs to
Appendix A's deferred "Full `PKG_TABLE`/`$Catalog` → `.chezmoidata` merge" row (`CHEZMOI_WAVE_A_SPEC.md`
Out-of-scope list), which is Wave B. Wave A does **not** delete `install-deps.ps1` (Resolved decision
#2), so the existing pwsh-via-scoop install keeps owning PowerShell 7 throughout the pilot and the
parity gate. **Do not port pwsh into a chezmoi run-script for Wave A.**

The pilot has a real pwsh **dependency** (not an install): the default `.ps1` interpreter is `pwsh
-NoLogo -File` (Verified mechanics §9), used by the DC-3 `.ps1.tmpl` run-scripts. Add **one bullet to
Step 0 (Prerequisites)** of `CHEZMOI_WAVE_A_SPEC.md`:

> - **Windows parity-harness prerequisite (DC-3):** `pwsh` must already be on `PATH` (installed by the
>   legacy `install-deps.ps1` → `$Catalog` `pwsh`/scoop, `install-deps.ps1:123,467`) **before** `chezmoi
>   apply` runs any `.ps1.tmpl` run-script, because the default `.ps1` interpreter is `pwsh -NoLogo
>   -File`. On a Windows-PowerShell-5.1-only host, install pwsh first or override `[interpreters.ps1]
>   command = "powershell"`. No new chezmoi package step in Wave A; the `$Catalog → .chezmoidata` pwsh
>   row lands in Wave B.

No change to any DC step, the parity gate, or the resolved decisions.

---

## W3 — psmux sporadic "auth failed"

### Gap & root cause

A colleague sporadically sees psmux fail to install with "authentication failed". Both symptoms trace to
one un-hardened line — `scoop bucket add psmux https://github.com/psmux/scoop-psmux 2>$null | Out-Null`
(`install-deps.ps1:377`) — which `git clone`s the bucket repo. Two intermittent failures hide behind it:

1. **`GIT_TERMINAL_PROMPT` is not `0` and `GCM_INTERACTIVE` is not `0`**, so a credential challenge
   (anonymous-HTTPS clone hitting a 401/403, a stale Git Credential Manager cache entry, or GCM deciding
   to re-auth) makes git block on a console prompt or pop a browser nobody answers. Over psmux/SSH/
   `setup.ps1`/`chez apply` there is no answerable console, so it dead-hangs and eventually emits
   "authentication failed". Because `setup.ps1` runs `install-deps.ps1` as a child and only inspects
   `$LASTEXITCODE` for bootstrap, a credential prompt hangs the **entire** setup.
2. **`scoop bucket add` does not check the underlying `git clone` exit code** (ScoopInstaller/Scoop
   #5482/#5814), so a transient network/SSL/early-EOF/rate-limit failure registers an **empty** bucket
   and reports success — surfacing only on the next `scoop install psmux` as a confusing downstream
   error; and `2>$null` suppresses the diagnostic so the user never learns why.

The same un-hardened pattern repeats at `install-deps.ps1:53-54` (`Ensure-ScoopBuckets`: extras/
nerd-fonts) and `install-deps.ps1:261` (Hack Nerd Font path), and the identical line is **already ported
into the chezmoi run-script** (`CHEZMOI_WAVE_A_SPEC.md` DC-3 Step 1), so the parity gate would encode the
bug verbatim. Net root cause: the bucket-add is interactive-prompt-capable, non-idempotent, exit-code
blind, and stderr-suppressed.

### Fix

One shared helper makes every `scoop bucket add` idempotent, non-interactive (fail-fast, not hang),
exit-code-aware, and clean-fall-through. It uses only native commands + `-ErrorAction SilentlyContinue`
and **never throws**, so it is safe under both the legacy `$ErrorActionPreference='Continue'`
(`install-deps.ps1:21`) and the chezmoi run-script's `$ErrorActionPreference='Stop'`.

**Edit 1 — `install-deps.ps1`: insert the helper after `Add-ScoopToPathForCurrentProcess` (after line
48, before `Ensure-ScoopBuckets` at line 50).** Paste verbatim:

```powershell
function Add-ScoopBucketSafe {
    # Idempotent, non-interactive `scoop bucket add`. Returns $true if the bucket
    # is present AND populated afterward, $false otherwise. NEVER throws, so a
    # failed clone falls through to the next package manager instead of hanging
    # or aborting (matters under $ErrorActionPreference='Stop' too -- the chezmoi
    # run-script port relies on this).
    #
    # Hardens two real, sporadic failures of `scoop bucket add` (it git-clones):
    #   1) git / Git Credential Manager prompting (or popping a browser) over a
    #      non-interactive console (psmux / SSH / setup.ps1 / chez apply) -> a
    #      credential challenge would otherwise HANG the whole run and eventually
    #      surface as "authentication failed". GIT_TERMINAL_PROMPT=0 +
    #      GCM_INTERACTIVE=0 make git/GCM FAIL FAST instead.
    #   2) ScoopInstaller/Scoop#5482 / #5814: `scoop bucket add` reports success
    #      even when the underlying clone fails, leaving an EMPTY bucket. We verify
    #      the bucket dir is non-empty and purge a half-clone so retry is clean.
    #
    # When $Url is empty, fall back to the bare `scoop bucket add <name>` form so
    # scoop's known-bucket table resolves the canonical URL (extras / nerd-fonts).
    param(
        [Parameter(Mandatory)][string]$Name,
        [string]$Url = ''
    )
    $scoopRoot = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
    $bucketDir = Join-Path (Join-Path $scoopRoot 'buckets') $Name
    $populated = {
        (Test-Path -LiteralPath $bucketDir) -and
        (@(Get-ChildItem -LiteralPath $bucketDir -Force -ErrorAction SilentlyContinue).Count -gt 0)
    }

    if (& $populated) { return $true }   # already added + populated: skip the clone

    $oldPrompt = $env:GIT_TERMINAL_PROMPT
    $oldGcm = $env:GCM_INTERACTIVE
    $env:GIT_TERMINAL_PROMPT = '0'   # git: no terminal prompt -> fail instead of block
    $env:GCM_INTERACTIVE = '0'       # GCM: never prompt / open a browser -> fail fast
    try {
        foreach ($attempt in 1..2) {
            # 2>&1 keeps the diagnostic (NOT 2>$null) so a real failure is visible.
            if ([string]::IsNullOrEmpty($Url)) {
                scoop bucket add $Name 2>&1 | Out-Null
            } else {
                scoop bucket add $Name $Url 2>&1 | Out-Null
            }
            if (& $populated) { return $true }
            # Purge a half-cloned / empty bucket so the next attempt starts clean
            # (Scoop#5482: a registered-but-empty bucket otherwise blocks re-add).
            if (Test-Path -LiteralPath $bucketDir) {
                Remove-Item -LiteralPath $bucketDir -Recurse -Force -ErrorAction SilentlyContinue
            }
            scoop bucket rm $Name 2>&1 | Out-Null
        }
        Write-Warning ("scoop bucket add {0} did not populate a usable bucket; recover with 'scoop bucket rm {0}' then re-run" -f $Name)
        return $false
    } finally {
        $env:GIT_TERMINAL_PROMPT = $oldPrompt
        $env:GCM_INTERACTIVE = $oldGcm
    }
}
```

**Edit 2 — `install-deps.ps1:50-55`: replace the body of `Ensure-ScoopBuckets`** so extras/nerd-fonts
route through the helper (bare-name form so scoop resolves the canonical URL):

```powershell
function Ensure-ScoopBuckets {
    if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) { return }
    if ($DryRun) { return }
    Add-ScoopBucketSafe -Name 'extras' | Out-Null
    Add-ScoopBucketSafe -Name 'nerd-fonts' | Out-Null
}
```

**Edit 3 — `install-deps.ps1:261`: replace the bare bucket-add in the Hack Nerd Font scoop path.** Change
`scoop bucket add nerd-fonts 2>$null | Out-Null` to:

```powershell
        Add-ScoopBucketSafe -Name 'nerd-fonts' | Out-Null
```

**Edit 4 — `install-deps.ps1:376-384`: replace the scoop branch of `Install-Psmux`** (the winget/choco
fallback at `:385-401` stays byte-for-byte). The bucket has a custom URL not in scoop's known table, so
pass it explicitly:

```powershell
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        if (Add-ScoopBucketSafe -Name 'psmux' -Url 'https://github.com/psmux/scoop-psmux') {
            scoop install psmux
            if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) {
                Write-Host ("  installed {0,-26} via scoop" -f "psmux")
                return
            }
            Write-Warning "scoop install of psmux failed; trying winget..."
        } else {
            Write-Warning "scoop bucket add psmux failed (clone auth/network); trying winget..."
        }
    }
```

Also update the `$DryRun` preview at `install-deps.ps1:373` so it no longer advertises the old command:

```powershell
        Write-Host "  would: Add-ScoopBucketSafe psmux; scoop install psmux  (fallback: winget / choco)"
```

### Tests

- **STATIC PARSE (cross-platform, the off-Windows automated reach):** `bash tests/static/ps1_parse.sh`
  must still print `all ps1 files parse`. The helper uses only PS-5.1-parseable constructs (scriptblock
  vars, `1..2`, `[string]::IsNullOrEmpty`), so the AST gate stays green.
- **REGRESSION GUARD (cheap, cross-platform — the durable guard):** add a check to
  `tests/static/repo_policy_test.sh` asserting `install-deps.ps1` contains **zero** bare `scoop bucket
  add` lines (every occurrence must be inside `Add-ScoopBucketSafe`) and that `Add-ScoopBucketSafe` is
  defined:

  ```python
  ps1 = pathlib.Path("install-deps.ps1").read_text(encoding="utf-8")
  if "function Add-ScoopBucketSafe" not in ps1:
      fail("install-deps.ps1 must define Add-ScoopBucketSafe")
  for i, line in enumerate(ps1.splitlines(), start=1):
      s = line.strip()
      if s.startswith("scoop bucket add ") or "| scoop bucket add " in s:
          fail(f"install-deps.ps1:{i} uses a bare 'scoop bucket add'; route it through Add-ScoopBucketSafe")
  ```

  > If `repo_policy_test.sh` isn't already a python harness, add this as a `python3 - <<'PY' … PY`
  > block or an equivalent `grep -nE '^\s*scoop bucket add '` that fails on any match. This is the
  > **only** psmux-bucket guard that runs on the Linux/macOS CI arms.

- **PARITY GREP (cross-platform):** in the same block, assert the spec stays in lockstep —
  `docs/plans/CHEZMOI_WAVE_A_SPEC.md` must contain `Add-ScoopBucketSafe` and must **not** contain a bare
  `scoop bucket add psmux ... 2>$null`.

- **PESTER UNIT (Windows CI only — the psmux bucket path has NO Linux/macOS reach).** Add to
  `tests/powershell/InstallDeps.Tests.ps1`, reusing the `INSTALL_DEPS_PS1_SOURCE_ONLY` dot-source seam
  (`:12-24`/`:75-83`) and the `scoop` capture mock (`:115-126`):
  - `It 'skips scoop bucket add when the psmux bucket already exists'` — mock `Test-Path`/`Get-ChildItem`
    so `$bucketDir` looks populated; assert `scoop` is **never** invoked with `bucket add`.
  - `It 'sets GIT_TERMINAL_PROMPT and GCM_INTERACTIVE to 0 during the add and restores them after'` —
    capture both env vars at call time (`'0'`), assert restoration after (including the `$null` case).
  - `It 'returns false and falls through to winget when the bucket never populates'` — keep `$bucketDir`
    empty (Scoop#5482 state); assert `$false`, a `scoop bucket rm` purge attempt, and that `Install-Psmux`
    then invokes `winget`.
  - **Extend** the existing `It 'adds required buckets when Scoop already exists'` (`:113-127`): mock
    `Test-Path` → `$false` for the buckets dir so the add fires, and keep the two `Should -Contain`
    assertions (`'bucket add extras'`, `'bucket add nerd-fonts'`).

  CI reaches these only on `windows-2025` via `.\test.ps1`. The real clone/credential-prompt behavior is
  **not** covered by any CI arm — see Acceptance for the manual gate.

### Acceptance

- [ ] `bash tests/static/ps1_parse.sh` prints `all ps1 files parse`.
- [ ] `bash tests/static/repo_policy_test.sh` passes: zero bare `scoop bucket add` in `install-deps.ps1`,
      `Add-ScoopBucketSafe` defined, and the spec parity grep green.
- [ ] On Windows, `.\test.ps1` runs the new Pester cases green (skip-when-populated;
      GIT_TERMINAL_PROMPT/GCM_INTERACTIVE `'0'` during + restored after; non-populating → `$false` →
      winget fallthrough).
- [ ] **MANUAL (the honest gate — no CI for the real clone):** on a clean Windows box, run
      `.\install-deps.ps1` over an SSH/psmux session with no usable interactive console. With the fix a
      clone that would have prompted **fails fast** (no hang, no browser pop) and the run continues to
      winget/choco; on success `scoop bucket list` shows a populated `psmux` bucket and `Get-Command
      psmux` resolves.
- [ ] **MANUAL idempotency:** run `.\install-deps.ps1` twice — the second run does **no** `scoop bucket
      add` (the helper returns `$true` early).
- [ ] **PARITY:** the DC-3 Step 1 code block in `CHEZMOI_WAVE_A_SPEC.md` contains `Add-ScoopBucketSafe`,
      has no bare `2>$null` bucket-add, and still ends with the `exit 1` survivor.

### Docs to update

- **`CLAUDE.md`** — extend the existing psmux/scoop bullets: every `scoop bucket add` now routes through
  `Add-ScoopBucketSafe` (idempotent; non-interactive via `GIT_TERMINAL_PROMPT=0` + `GCM_INTERACTIVE=0` so
  a clone credential challenge fails fast instead of hanging; verifies the bucket populated to defeat
  Scoop#5482's "registered-but-empty" bug). Note the psmux bucket-add passes a custom URL and falls
  through to winget/choco on failure. Add a guarded-convention line: "All `scoop bucket add` calls in
  `install-deps.ps1` must go through `Add-ScoopBucketSafe` (guarded by `tests/static/repo_policy_test.sh`)."
- **`README.md`** — no change.

### chezmoi delta

Touches **DC-3 → Step 1** (`run_once_after_10-install-psmux.ps1.tmpl`). The ported run-script carries the
identical bug and runs under the stricter `$ErrorActionPreference='Stop'`, so the fix **must** land in
both places in the same change or the parity gate encodes the bug. The helper never throws → Stop-safe.

**Replace the DC-3 Step 1 code block** with the hardened version below — define `Add-ScoopBucketSafe`
at the top and swap the bare `scoop bucket add psmux … 2>$null | Out-Null` for `if (Add-ScoopBucketSafe
'psmux' '…') { scoop install psmux; … }`. The `$ErrorActionPreference='Stop'`, `Test-Tool`, winget/choco
fallback, and `exit 1` survivor stay unchanged:

```powershell
# home/.chezmoiscripts/run_once_after_10-install-psmux.ps1.tmpl
{{- if eq .targetOS "windows" -}}
$ErrorActionPreference = 'Stop'
function Test-Tool($n) { [bool](Get-Command $n -ErrorAction SilentlyContinue) }

# Idempotent, non-interactive scoop bucket add. NEVER throws (Stop-safe);
# GIT_TERMINAL_PROMPT/GCM_INTERACTIVE off so a clone auth challenge fails fast
# instead of hanging chez apply, and we verify the bucket populated
# (ScoopInstaller/Scoop#5482). Mirrors install-deps.ps1 Add-ScoopBucketSafe.
function Add-ScoopBucketSafe($Name, $Url) {
  $root = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
  $dir  = Join-Path (Join-Path $root 'buckets') $Name
  $ok   = { (Test-Path -LiteralPath $dir) -and (@(Get-ChildItem -LiteralPath $dir -Force -ErrorAction SilentlyContinue).Count -gt 0) }
  if (& $ok) { return $true }
  $op = $env:GIT_TERMINAL_PROMPT; $og = $env:GCM_INTERACTIVE
  $env:GIT_TERMINAL_PROMPT = '0'; $env:GCM_INTERACTIVE = '0'
  try {
    foreach ($i in 1..2) {
      if ([string]::IsNullOrEmpty($Url)) { scoop bucket add $Name 2>&1 | Out-Null }
      else { scoop bucket add $Name $Url 2>&1 | Out-Null }
      if (& $ok) { return $true }
      if (Test-Path -LiteralPath $dir) { Remove-Item -LiteralPath $dir -Recurse -Force -ErrorAction SilentlyContinue }
      scoop bucket rm $Name 2>&1 | Out-Null
    }
    Write-Warning "scoop bucket add $Name did not populate a usable bucket; recover with 'scoop bucket rm $Name' then re-run"
    return $false
  } finally { $env:GIT_TERMINAL_PROMPT = $op; $env:GCM_INTERACTIVE = $og }
}

if (Test-Tool 'psmux') { Write-Host 'ok        psmux already installed'; return }

# 1) scoop (custom bucket first) — install-deps.ps1:376-384
if (Test-Tool 'scoop') {
  if (Add-ScoopBucketSafe 'psmux' 'https://github.com/psmux/scoop-psmux') {
    scoop install psmux
    if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) { Write-Host 'installed psmux via scoop'; return }
  }
  Write-Warning 'scoop install of psmux failed; trying winget...'
}
# 2) winget — install-deps.ps1:386-390
if (Test-Tool 'winget') {
  winget install psmux --accept-source-agreements --accept-package-agreements --silent
  if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) { Write-Host 'installed psmux via winget'; return }
  Write-Warning 'winget install of psmux failed; trying choco...'
}
# 3) choco — install-deps.ps1:394-398
if (Test-Tool 'choco') {
  choco install psmux -y
  if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) { Write-Host 'installed psmux via choco'; return }
}
Write-Warning 'psmux install failed via scoop/winget/choco.'
exit 1   # required survivor: fail so run_once_ records NO success and retries next apply
{{- end -}}
```

Also add one sentence to the DC-3 Step 1 prose: the psmux bucket-add is hardened via `Add-ScoopBucketSafe`
(idempotent + non-interactive, mirroring `install-deps.ps1`) **because** the chezmoi run-script is
non-interactive and `Stop`-strict — an un-hardened clone credential prompt would hang `chez apply` with
no answerable console. **Ordering:** this is a legacy-installer correctness fix and may land before the
chezmoi migration mechanics, but the spec edit must land in the **same change** — if the legacy installer
is hardened and the spec is not, the Wave-A port reintroduces the bug and the parity gate blesses it.

---

## Consolidated test commands

```bash
# Cross-platform (POSIX dev host or CI):
bash tests/static/ps1_parse.sh          # W3: all ps1 parse
bash tests/static/repo_policy_test.sh   # W3: no bare scoop bucket add + spec parity grep
make test-static                        # full static gate

# Windows host / windows-2025 lane (the only automated proof of pwsh merge + Pester guards):
.\test.ps1                              # PSScriptAnalyzer + Pester (W1 default-on, W2 catalog, W3 bucket)
```

## chezmoi-spec deltas (rollup — see `CODEX_EXECUTION_PLAN.md` for the master table)

| Fix | `CHEZMOI_WAVE_A_SPEC.md` section | One-line delta |
|-----|----------------------------------|----------------|
| W1 | DC-3 Step 2 preamble; DC-6 Step 4 parity probe | Note legacy merge is now default-on (parity); OLD-side probe runs `bootstrap.ps1` with no switch. |
| W2 | Step 0 Prerequisites | Add pwsh-on-PATH prerequisite bullet (pilot's `.ps1` interpreter). No install ported (Wave B). |
| W3 | DC-3 Step 1 code block + prose | Port `Add-ScoopBucketSafe`; replace bare `scoop bucket add psmux … 2>$null`. |
