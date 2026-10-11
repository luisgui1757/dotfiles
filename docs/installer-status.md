# Interactive installer status

**Delivery candidate; readiness follows the exact PR head.**
Updated 2026-10-11. This document records scope and dated evidence. Current
readiness comes from [PR #87](https://github.com/luisgui1757/dotfiles/pull/87):
all 23 required native/quality checks and independent review must pass before it
is ready. Dated results below certify their stated source, not a later head.

[PR #87](https://github.com/luisgui1757/dotfiles/pull/87) is the single delivery PR
and retains one implementation commit. #88 is incorporated and closed; #86 is
superseded. Published checkpoint `044f9780705faa996914ed95de2aed2126802ba9`
passed the full local gate, all six foreign all-package build/test-compilation
checks and actual Opus v7 review without material defects. Its hosted matrix
passed **19 of 23 jobs**; all three CodeQL analyses and the aggregate passed.
The four failed contexts remain recorded as failures: Ubuntu completion-directory
permissions on the hosted image, a PowerShell helper noun, the Windows fixture's
missing explicit shared-PowerShell removal choice, and Windows compiler-query
accounting lost across an unsafe API pointer boundary.

Each correction has native scoped proof. The Ubuntu helper and complete unchanged
shell suite pass on amd64 and arm64. Actual PSScriptAnalyzer, module binding and
complete shared-shell lifecycle pass on Windows. The corrected Windows API
boundary passes real kernel output/limit checks, the complete process-lifetime
suite and six compiler-reuse runs. Exact HANDLE waits, late-member refusal,
outside-process protection and original time bounds remain unchanged. Those
proofs certify their frozen sources; the dated ledger preserves diagnostic
failures and non-reproduced claims. Final readiness requires the consolidated
local gate, independent review and every canonical native/quality job plus
CodeQL on the exact amended PR head.

## Scope and roadmap

Supported targets are macOS arm64, native Windows amd64, and Ubuntu/Debian on
amd64/arm64. Linux native acceptance uses Ubuntu 26.04 and Debian 13. WSL uses
the ordinary Linux lifecycle. Fedora, macOS Intel and separate WSL provisioning
are outside this release. Nix and chezmoi are removed, with explicit migration
and passive historical live-link sources instead of a compatibility runtime.

| Milestone | Implementation | Acceptance contract |
|---|---|---|
| 1. CLI tools and prerequisites | DONE: all providers, independent fd/ripgrep choices, APT, Homebrew and Windows toolchains connected | Native install, runtime use, update and surgical removal, including fresh provider/toolchain jobs |
| 2. Neovim | DONE: private locked plugins, parsers, Mason tools and language dependencies | Full selection, real language use, update, shared removal and personal-data preservation on all four targets |
| 3. Product integrations | DONE: desktop apps/fonts, scoped settings, Sentinel and development checks | Native desktop/font and configured GUI lifecycles on every supported target |
| 4. Public cutover | DONE: verified Go bootstrap, migration entrypoints and retired old runtime | Public bootstrap and migration lifecycle, including Windows PowerShell 5.1 |
| 5. Delivery | DONE: consolidation into the single PR and implementation commit; merge/release remain owner actions | Green final-head native checks and independent review before ready status; authorized required-context transition before merge |

The design is settled. Additional infrastructure needs a demonstrated defect or
an actual supported-tool requirement. Focused regressions precede the integrated
gate; unchanged passing areas are not repeatedly redesigned. Local mocks and
cross-compilation do not substitute for real native lifecycle acceptance.

## Checkbox coverage

Every selection below has all provider routes connected. This records product
scope and implementation, not a claim that its final native acceptance passed.
A dash means unavailable by design.

| Checkbox | macOS arm64 | Ubuntu/Debian | Windows |
|---|---|---|---|
| Neovim | Connected | Connected | Connected |
| VS Code | Connected | Connected | Connected |
| zsh | Connected | Connected | — |
| PowerShell 7 | — | — | Connected |
| Starship | Connected | Connected | Connected |
| lsd | Connected | Connected | Connected |
| zoxide | Connected | Connected | Connected |
| fzf | Connected | Connected | Connected |
| ripgrep | Connected | Connected | Connected |
| fd | Connected | Connected | Connected |
| tmux | Connected | Connected | — |
| Ghostty | Connected | Connected | — |
| Windows Terminal | — | — | Connected |
| AeroSpace | Connected | — | — |
| Herdr | Connected | Connected | Connected |
| Git defaults | Connected | Connected | Connected |
| lazygit | Connected | Connected | Connected |
| GitHub CLI | Connected | Connected | Connected |
| gh-dash | Connected | Connected | Connected |
| Pi | Connected | Connected | Connected |
| Sentinel policy | Connected | Connected | Connected |
| Repository development checks | Connected | Connected | Connected |

Neovim provisions the locked notes plugin automatically but activates it only
for an existing vault. Notes are not a separate checkbox. Compiler/SDK packages,
fonts and runtime libraries are hidden dependencies of their consumers.

## Published evidence

Checkpoint `28c92f5c8c2ec0e0f68ad9616660454e5991b0b4` passed `make -k ci`
with unchanged source hashes, all-package Windows amd64 and Linux amd64/arm64
builds and test compilation, and the 684-file EditorConfig check.
[Installer run 38082752162](https://github.com/luisgui1757/dotfiles/actions/runs/38082752162)
passed 12 of 20 jobs; [quality run 38082752292](https://github.com/luisgui1757/dotfiles/actions/runs/38082752292)
passed macOS and Ubuntu, while Windows failed. All four fresh Ubuntu/Debian APT
lanes now passed complete Neovim installation from absent prerequisites. Both
Ubuntu Neovim lifecycles, fresh Homebrew, fresh VC++ runtime, fresh Build Tools
and three POSIX core jobs passed. The remaining eight installer jobs remain
failed; scoped successes inside them do not change their parent conclusions.

Windows core passed public setup/migrate in 71.76 seconds, native Rust with the
274-character stdlib path (94.73 seconds), LaTeX (98.76 seconds), Yamllint
(88.70 seconds), and repeated Build Tools inspection (29.86 seconds). Windows
Desktop passed font lifecycle and real Terminal Settings Save/check/removal;
VS Code shortcut preparation failed before the desktop application launch.
The separate temporary diagnostic branch creates no PR and certifies no required
check. Its exact head `48fbcc8b5e258298630680f8d6e6edc279e57035`,
[run 38085071529](https://github.com/luisgui1757/dotfiles/actions/runs/38085071529),
passed the corrected real PowerShell 5.1 extraction, COM shortcut and protocol
boundaries, but still failed process completion and Git's strict runtime probe.
Those failures remain open until canonical correction and native rerun.

The actual immutable Opus v4 review executed `claude-opus-5-5` with requested
`xhigh` effort (the provider did not expose effective effort), made 176 read-only
tool calls, and left all 829 source/evidence files unchanged. Prior D1/D2/D3 were
revalidated. New F1 active-shell coverage, F2 POSIX palette and F3 passive Windows
LazyGit hash coverage are accepted; all thirteen originally omitted E1 logs have
been verified against their original handoff hashes for the next review.


Checkpoint `077683f8bc45e1feb4c65824fe734876aec839ee` passed `make -k ci`
with unchanged source hashes, plus Windows amd64 and Linux amd64/arm64 builds
and all-package test compilation. The PR merge checkout has the same tree as
that published head. [Quality run 38076760170](https://github.com/luisgui1757/dotfiles/actions/runs/38076760170)
passed on macOS, Ubuntu and Windows. [Installer run 38076760167](https://github.com/luisgui1757/dotfiles/actions/runs/38076760167)
has passed both Ubuntu core jobs, macOS core, fresh Homebrew, fresh VC++ runtime,
and both Ubuntu Neovim lifecycles. Its other native failures remain open below;
Windows core failed its aggregate archive deadline, PortableGit preparation and
public bootstrap extraction; its actual Rust/LaTeX/Yamllint and Pi theme steps
passed. Those scoped passes do not establish a passing core job. The actual Opus review resolved prior findings and
identified the Rust identity and passive-source corrections now being integrated.

Earlier checkpoint `fbca6c224d03ac57597628464ba0bf92e94f2df7` passed `make -k ci`
(including the 438-second Go race suite) with unchanged source hashes. Its Windows
root-package test compilation passed; final all-package foreign compilation is
still required. Completed native runs at that exact head are:

- [Installer run 38071197318](https://github.com/luisgui1757/dotfiles/actions/runs/38071197318):
  9 of 20 jobs passed: fresh Homebrew, fresh Ubuntu/Debian APT on both
  architectures, fresh Windows VC++ runtime, both Ubuntu core jobs and macOS core.
- [Quality run 38071197325](https://github.com/luisgui1757/dotfiles/actions/runs/38071197325):
  macOS and Ubuntu passed; Windows failed its Git Bash bootstrap fixtures.

Passing steps inside failed jobs are narrower evidence. Windows completed native
archive/filesystem checks, VC++ inspection, LaTeX with forced cp1252 and Python
replacement, and Yamllint's lifecycle. Windows Desktop completed font lifecycle.
They do not turn their parent jobs into passes. Fresh APT passes at this checkpoint
did not yet exercise a complete Neovim installation; that requirement is added
in the current correction batch.

Earlier checkpoint `e1c987d` passed the full local gate and
[all four native engine jobs](https://github.com/luisgui1757/dotfiles/actions/runs/38053630162),
including archive updates, configuration/profile recovery, Windows process-death
and settings-lock cases, Pi application use, and Homebrew outside-consumer update.
That is historical engine evidence, not final product certification.

## Hosted failures and current corrections

| Area | Observed failure | Current state |
|---|---|---|
| Windows Build Tools | Query regressions at `28c92f5` return before exact owned child handles signal | Verified private-job handles now signal before inspection returns under one existing five-second bound. Exact native diagnostic `9ff8dd0` passes all five unchanged immediate-wait/outside-survival cases (1.69 seconds), plus real late-member refusal. Final-head native acceptance remains required. Fresh real Build Tools lifecycle already passed at `28c92f5`. |
| Windows Rust | MSVC could not open a 273-character physical stdlib path; unrelated installer rebuilds also changed desired identity | At `077683f`, real rustc compile/run, offline Cargo and Clippy, check and removal pass with a 274-character stdlib path. Desired identity now follows only the launcher recipe while exact copied-byte provenance remains. Native public rebuild/check proof is required for that latest change. |
| Windows PortableGit | At `077683f`, extraction succeeds, then Git rejects `GIT_CONFIG_GLOBAL=NUL` and CMD loses its self-deleting post-install input | Use Git’s `/dev/null` spelling and execute an unchanged, exclusively staged sibling script. Nonzero exits, unfinished cleanup and runtime failures remain fatal. Focused regressions and Windows compilation pass; actual CMD and Git/Make lifecycle reruns remain required. |
| Windows public bootstrap | Earlier archive-module extraction exceeded the public fixture deadline | Public setup/migrate passes at `28c92f5` in 71.76 seconds. Corrected ordinary extraction also passes under actual Windows PowerShell 5.1 on the diagnostic runner; final-head coverage remains required. |
| Windows quality | At `28c92f5`, strict process/extraction fixtures and real clangd document symbols failed | Extraction is corrected and native-proven; exact child-handle completion is under correction. Clangd now records bounded original request/client/stderr diagnostics without relaxing its deadline; the Windows failure cause remains unproved. |
| Apple developer tools | Preservation now passes; receipt directory `private` is wrongly treated as an outside payload file | The latest real BOM also names an outside installer temporary file. Validate canonical receipt paths and record existing outside files for exact preservation before any CLT move; historical absent entries grant no ownership. Traversal, malformed paths and failed queries remain fatal. Focused red/green and private macOS metadata regressions pass; actual missing-payload provisioning remains pending. |
| Windows Terminal | Earlier actual UIA Save serialized an equivalent `closeTab` object and lost ownership | At `28c92f5`, actual Settings Save changed bytes and mtime, followed by check/removal PASS (26.66 seconds). Windows font lifecycle also passed (22.91 seconds); the separate desktop-application step failed later. |
| Linux Neovim | Earlier bytecode-name and APT residual-state failures | Both Ubuntu Neovim lifecycle jobs and all four fresh Ubuntu/Debian APT plus complete Neovim lanes pass at `28c92f5`. Final-head native acceptance remains required. |
| macOS Neovim | At `28c92f5`, the native entry diff proves 21 added node-gyp Python bytecode entries inside the Node payload | Scope `PYTHONDONTWRITEBYTECODE=1` to sync/check children. Real pinned gyp invocation fails the full Node snapshot before and preserves it after; integrity checks remain unchanged. Complete macOS Neovim lifecycle rerun is still required. |
| Windows Neovim | v3 reviewed/fresh plans differ only in compiler consumers: absent → `vctip.exe` | Compiler inspection captures consumers before its probe spawns the surviving child. Isolate that query's process tree without relaxing consumer or approval checks. Windows native regression and full Neovim rerun remain required; no automatic apply retry. |
| WezTerm desktop | Earlier native startup failures | Removed from all OSes by explicit owner scope decision. Historical failures are not fixes or passes; remaining desktop apps still need native acceptance. |
| Windows desktop shortcut | At `28c92f5`, VS Code shortcut preparation rejected the native digest | Scoped PowerShell progress handling and separate read-only query streams have actual PowerShell 5.1/COM diagnostic-run proof. Complete configured desktop lifecycle is still required. |
| Review F1 | Shortcut preparation retry can strand operations | Operation/attempt-scoped preparation and existing journal-bound source recovery integrated. Durable failure/unfinished-result, hash and resume regressions pass; real Windows COM rerun required. |
| Review F5 | A newly created higher-priority Bash profile strands the recorded target | Reuse the receipt-validated baseline path. Fresh-controller install/update/removal, forged-evidence and interrupted-operation regressions pass; personal profile bytes survive. |
| Review F2/F3/F4 | Old TPM fallback, missing Linux Mason prerequisites and misleading Windows Make privilege | Corrections integrated with focused regressions. Fresh Ubuntu/Debian jobs now exercise the complete Neovim lifecycle from absent curl/unzip. |
| Review F6/F7 | Unused zprofile copies/test and inconsistent Go timeout | Remove dead copies; retain generated-profile behavior tests. Local gate now uses the same 20-minute Go budget as native CI. |
| Review D1/D3 | Unrelated installer rebuild invalidated Windows Rust; change checklist named a missing language matrix | Separate desired launcher revision from exact operation/version executable provenance; controller red/green preserves the generation and still detects installed tampering. Native public rebuild/check proof is added. Correct the documentation path. |
| Review D2 | Six passive released sources changed before migration | Restore their exact v0.4.4 bytes; inventory hashes protect all present recorded passive targets/profiles and referenced templates. Migration recognizes the restored PowerShell profile and still preserves personal edits; active shell, tmux and default LazyGit recipes remain independent. |

Publication requires a complete local gate and all-package foreign compilation;
final acceptance requires the native runs.
The [append-only review ledger](reviews/2026-10-08-interactive-installer-overhaul.md)
records exact regressions, current proof and rejected alternatives. The published
local gate above proves `077683f`; later working-tree corrections need their own gate.

## Remaining delivery gates

1. Finish native diagnosis/corrections and pass the integrated local gate.
2. Pass all 23 canonical quality/native jobs on the final PR head. Native package
   and font mutations run only on explicitly opted-in disposable GitHub hosts.
3. Resolve the completed Opus 5.5 full-delta review findings and obtain review
   of the corrected final delta. The actual reviewer was `claude-opus-5-5`; xhigh
   was requested but effective effort is unexposed. It returned defects and
   verification gaps, not runtime approval.
4. Reconcile the live required checks using the
   [documented pre-merge transition](security/branch-protection.md), preserving
   security/review policy. Checked-in context names are not live policy evidence.
5. Leave one reviewed commit and one ready PR. Merge and release publication
   remain owner actions.

AeroSpace configuration/tiling needs user-granted Accessibility consent. The
hosted fixture reports launch separately when consent is unavailable; it never
bypasses TCC. Archived pins are reviewed inputs: Update reconciles the current
checkout rather than silently fetching arbitrary upstream versions.

The obsolete Linux compiler-to-Make ordering edge is removed. Neovim still
requires both tools directly; APT owns build-essential's native Make dependency.
Compiler-first and Make-first controller fixtures verify update, selective Make
release and final native removal without catalog ordering workarounds.

The missing-payload Apple job preserves historical receipts, moved tool roots and representative payload files, developer siblings and Homebrew. It proves neither a never-installed Mac nor a same-version reinstall until the actual native result establishes that version relationship.

## 2026-10-10 — WezTerm retired by owner scope decision

The active catalog has 22 capabilities after removing WezTerm on macOS, Linux
and Windows. Its four resources, four platform archive pins, configuration,
launcher hooks and dedicated smoke tests are removed. Eight exclusive Linux
libraries are pruned; shared fonts/tools/runtime libraries and APT ownership
regressions remain. VS Code now exercises the public singleton desktop lifecycle
and personal-launcher adoption/restoration. Passive released WezTerm bytes and
discovery records remain unchanged, with no removal authority inferred from them.
The earlier native launch failures remain unproved historical failures, not fixes.
Focused validation is recorded in the review ledger; the remaining desktop
native lifecycle still requires the disposable hosted matrix.

## 2026-10-10 — APT reinstall after retained conffiles

The Debian 13 arm64 fresh Neovim job exposed a provider evidence error after the
preceding native fixture removed Git. dpkg correctly treats `config-files` as
absent payload but logs its remembered old version on the next `install` action.
The recovery parser now accepts that action without requiring `<none>`; native
success, exact operation attribution and final configuration remain required.
Focused successful/interrupted recovery regressions and a real disposable
install/remove/reinstall fixture cover retained personal conffiles without purge.
The full fresh Neovim hosted lifecycle remains a separate required proof.

## 2026-10-10 — Explicit native archive sweep budget

The native archive sweep now has an explicit 20-minute aggregate timeout. Windows
job `114285248186` exhausted Go's implicit 10-minute package budget while Python
was verifying its final removal, after its install/use/update/integrity checks.
Python itself had run for 98 seconds; the preceding 13 pins had already completed.
Individual command deadlines, all lifecycle assertions and the 60-minute core-job
limit remain unchanged. The complete archive sweep still needs a passing hosted
rerun. The same failed job separately passed real Rust compile/Cargo/Clippy and
Pi terminal-theme checks; those passes do not establish a passing core job.

## 2026-10-10 — Windows bootstrap extraction correction

At `077683f`, system PowerShell 5.1 downloaded the pinned Go ZIP in 1,003 ms and
verified it in 112 ms, then hit the existing five-minute fixture deadline during
archive extraction. The bootstrap now calls the built-in .NET ZIP extractor,
removing the archive module's measured per-entry script overhead. Exact-byte
extraction and boundary regressions pass locally; this is not a Windows timing
result. Native PowerShell 5.1 extraction and the complete public migration/setup
lifecycle remain required on the original paths and deadline.


## 2026-10-10 — Windows Neovim Git long paths

At `28c92f5`, both Linux Neovim lifecycles pass. Windows reaches synchronization
but Git rejects the managed Lazy staging path as too long. PortableGit now enables
its documented long-path support in its private system configuration before the
archive fingerprint; the isolated pinned-checkout helper supplies the same setting
explicitly. Recipe revision 1 makes this an ordinary update for completed older
payloads and rejects replaying unfinished older preparation. Local real-Git
configuration, archive lifecycle and long-path Lua regressions cover the change;
actual Windows long-path Git and full Neovim lifecycle proof remain required.


## 2026-10-10 — Windows clangd request diagnosis

At `28c92f5`, the Windows quality fixture starts two distinct initialized clangd
clients with their own project roots, then a document-symbol request times out.
The same fixture passed on Windows at `56a4860`; its test, production LSP config,
minimal init and runner have no intervening changes. The failure cause remains
unknown. Failure-only diagnostics now retain the request error, client/exit state
and at most 8 KiB of those owned servers' stderr. The original deadline and
project-isolation assertions remain required. Local actual-clangd success and a
bounded process-fixture failure validate diagnostics, not Windows remediation.


## 2026-10-10 — macOS published application process inventory

At `28c92f5`, desktop job `114302869025` received a successful NSWorkspace launch
PID but found no owned process through `ps comm`. A real private non-GUI process
regression proves that published directory aliases retain their public path in
that field; it also proves that an alias inside the package can falsely include
an outside executable. macOS fixture inventory now queries the kernel executable
vnode path and compares it with the resolved private package root. The regression
verifies owned cleanup, outside-process preservation, removed-executable handling
and cancellation. Actual window and configured-settings witnesses are unchanged.
This corrects the inventory evidence; it does not establish that VS Code reached
a usable window in the failed run. The complete disposable macOS desktop lifecycle
remains required, without changing HOME, shortening paths or bypassing publication.

The `28c92f5` Windows core job passed the public setup/migrate lifecycle in
71.76 seconds, including cold download (1.352 s), verified extraction (15.858 s)
and build (16.767 s). The ordinary extraction fixture failed earlier because it
omitted mandatory archive verification and treated Utility module first-load
progress as extraction progress. The fixture now reproduces that preceding
Get-FileHash phase; its Stop guard and malformed/escaping/existing-file checks
remain. Actual Windows execution of this fixture correction is pending.

## Current shell and passive-source review corrections

Opus v4 F2 is corrected: the POSIX palette is a shared fragment emitted only for
selected lsd or zsh, before completion. Actual generated-profile Bash tests cover
lsd without zsh, the explicit override and unselected personal preservation. F3
now protects the passive Windows LazyGit link destination with its released
metadata hash and the physical `core.autocrlf=true` checkout test. Final integrated
and native acceptance remain required.

## 2026-10-10 — Linux Code native package correction

Both Linux desktop jobs at `28c92f5` reached Chromium's sandbox startup and
rejected the private archive's helper. The official Code 1.141.0 DEBs are now
routed through the existing APT lifecycle and launch `/usr/share/code/code`,
matching Ubuntu's shipped AppArmor profile. They do not install a setuid helper.
The 29 duplicated Code library edges and 26 exclusive hidden library resources
are removed; Ghostty's three shared requirements remain. Existing native Code
stays unowned, and fixed maintainer-script input preserves personal Microsoft
repository/key/answer state, including deferred pool removal.

Focused regressions pass. Actual worker-backed install/recovery, repair, update
and removal pass in disposable Ubuntu 26.04 ARM (root) and Debian 13 ARM
(nonroot/sudo), with apt-utils preconfiguration and personal repository/key/true
answer sentinels preserved. The first native removal exposed missing dpkg
Essential/Protected observation; the corrected pool retains/discloses those
packages and their dependencies. This does not establish GUI startup. Both Linux architectures still require the
public configured desktop lifecycle on the corrected source. All four fresh
Ubuntu/Debian APT-plus-Neovim jobs passed at `28c92f5`; those results do not cover
this later Code correction.

Windows Git/Make at `28c92f5` reaches the strict PortableGit runtime phase after
extraction and upstream post-install completion, then exits 255 with a .NET-style
path-length error. The exact subcommand was not logged. Fixed runtime labels and
upstream GCM stack tracing in the same disposable private version probe are now
prepared; no runtime check is removed, no payload path is shortened, and no GCM
long-path correction is claimed before the native diagnostic rerun.

At `28c92f5`, fresh Windows Build Tools install/compile/remove and ordinary native
reuse/environment both pass. The five private process-tree boundary scenarios
still fail because their exact child HANDLE is not signaled when inspection
returns. A failure-only five-second HANDLE wait now records exit state and elapsed
time without changing the immediate assertion. Final object rundown versus a
containment defect remains unresolved pending native output.

Temporary diagnostic `48fbcc8` / job `114309772969` passes all four actual
PowerShell 5.1 extraction cases, real private COM shortcut lifecycle and both
progress regressions. All five Build Tools process cases demonstrate exit code 1
before HANDLE signal, followed by signal 2.21–3.45 ms after the query returned.
The provider now captures verified private-job HANDLEs and waits within the same
five-second total bound, refusing changing membership. Native zero-time assertions
and a new late-member refusal case remain required in the next diagnostic run.
PortableGit now stops before LFS/GCM at the adjacent version/private-config
assertions; separate value/exit diagnostics are prepared. Its original .NET-style
failure is still unresolved, not disproved by this earlier failure.


## 2026-10-10 — Active generated-shell coverage (review F1)

The shell/Pester tests now compose current recipes through `configureShellProfiles`
and publish them with the actual profile driver into private test directories.
They no longer use frozen release files as evidence of current shell behavior.
Active invocation guards, real PSReadLine options/keybindings and ordering,
zsh live keymaps, completion/plugin order, locale, colors, PATH and startup are
covered. New active coverage exposed inherited PATH duplicates; the zsh recipe
again retains first-occurrence ordering while removing duplicates. Its regression
fails before the declaration and passes afterward.

The existing Linux/macOS quality jobs generate and execute interactive zsh; the
native archive zsh lifecycle now also runs on Linux. Windows selected-shell native
acceptance includes PowerShell and lsd and inspects real PSReadLine state using
the managed interpreter. Local PowerShell/zsh process proof is distinct from those
pending hosted native lifecycles. Passive release hashes remain unchanged.

### Windows diagnostic follow-up: native HANDLE proof and Git path boundaries

Exact temporary head `9ff8dd0` passes all five immediate child-HANDLE/outside-work
cases plus real late-member refusal, PowerShell 5.1 extraction, desktop query and
COM publication. Git/Make still fails: its version is correct, but `/etc/gitconfig`
misses the private file while an explicit native path reads `core.longpaths=true`.
The independent GCM trace identifies .NET suggestion-directory creation beneath
private TMP, beyond the native 248-character directory boundary. The correction
uses `cygpath` for native Git and a separate fixed GCM phase with extended TMP/TEMP
spelling for the same directory. No settings, executable bytes or physical paths
are changed. Focused checks pass; complete native Git/Make and Neovim acceptance
remain pending the corrected candidate, not inferred from the HANDLE passes.

### Windows Git diagnostic: supported long-file coverage

Temporary head `db366b6`, job `114315143713`, passes both verified Git archive
install/update preparations, their native GCM checks and actual suggestion files
in the original >248-character private temporary directories. Published Git
reads its private system `core.longpaths=true`; ordinary initialization and Unicode
add/commit pass. The job then fails on our newly added roughly 450-character
repository root, a separate unsupported upstream working-directory boundary
([PR 3877](https://github.com/git-for-windows/git/pull/3877)). This assertion is
replaced by >320-character nested directory/file paths inside the existing normal
worktree, with plain add/commit, ordinary-root clone, checkout, diff and exact-byte
verification. Product code and all original managed paths remain unchanged.
Full Git/Make, normal published GCM, LFS/SSH, preservation/removal and Windows
Neovim acceptance remain pending; the failed diagnostic is not a lifecycle pass.

### Windows Git diagnostic: SSH identity-file argument

Temporary head `4eea1ff`, job `114317661301`, passes the corrected 428-character
long-file add/commit/clone/checkout/diff/byte checks, interactive staging, LFS,
normal published GCM and SSH key generation. It then fails because the fixture
passes an unquoted path with spaces through OpenSSH's configuration-syntax `-o`
option. The fixture now uses the documented dedicated `-i` filename argument,
with the identical key path. Real offline OpenSSH inspection reproduces exit 255
before and exit 0 after; Windows compilation/static checks pass. No product code
changes. The complete Windows SSH/Make, final payload/personal-config checks and
owned removal remain pending the next native run; this failed context stays failed.

### Windows Git diagnostic: explicit OpenSSH no-config mode

Temporary head `07e5ed1`, job `114319529231`, confirms the corrected identity
argument, then fails when bundled MSYS OpenSSH tries to open `NUL` as a config
file. The fixture now uses its documented `-F none` mode. The checksum-verified
Git archive records OpenSSH 10.5p1-1, whose exact upstream source bypasses both
user and system config for this value. An offline process regression reproduces
the missing-file failure and succeeds with `none`, preserving the identity path.
No runtime code or later assertion changes. Complete native Git/Make, final
payload/personal-config integrity and owned removal remain pending.


### 2026-10-10 — Review v5 Bash grammar and Git fixture correction

Generated interactive initialization now parses in both Bash and zsh when zsh
is selected. Only the fixed zsh compdump glob condition uses delayed parsing;
Bash never executes it. A real failed-before grammar regression now also asserts
the ready marker, selected lsd alias and empty initialization stderr. The complete
active shell suite and focused race tests pass after the correction.

The initial Lua Git correction used a 257-character staging root and 262-character
`.git` path, retaining nested committed files beyond 320 characters. Later native
proof rejected that root as an earlier pre-configuration canonicalization limit;
the 2026-10-11 correction below reproduces the actual 254-character failure.
The original >320-character repository root also demanded unsupported Windows
current-directory behavior. All 12 pinned-checkout cases pass locally with Neovim 0.12.5;
Windows execution remains required. The default Windows runtime is 78 characters
shorter than the cited hosted fixture for the same generation/PID, so the review's
username-plus-two failure prediction does not follow. No layout or ownership-path
change was made. Exact-head native jobs and the integrated full gate remain pending.


### 2026-10-11 — Ordinary npm builds preserve the managed Node payload

A real offline npm/node-gyp install succeeded but added 18 bytecode files to the
immutable Node package outside Neovim sync. Each Node pin now applies the existing
exact source-replacement mechanism at vendored gyp's import boundary. This scopes
bytecode suppression to that process, leaves personal Python settings intact and
changes desired identity for an ordinary archive update. No wrapper, global
preference, new persistence shape or fingerprint exclusion was added.

The same real offline npm build now leaves the complete Node snapshot unchanged;
archive check and owned removal pass. Old-pin update and exact rendered-source
regressions pass. All four checksum-verified pinned sources pass exact preparation (99.74 seconds).
The existing hosted native consumer/lifecycle gates still provide native execution
proof; foreign source decoding does not replace them.
The Windows ZIP lacks development headers, so its focused consumer fixture runs
the actual gyp import entrypoint directly; full Windows npm integration remains
part of the required Neovim lifecycle. The independent Git early-config-read
failure remains under investigation and is not fixed by this Node correction.


## 2026-10-11 — Linux Code lifecycle review corrections

Opus v5 F2/F4/F5/F6/F7 have focused failed-before regressions. Linux Code now
uses the native package's desktop entry directly; its graph has no private
launcher or shell attachment. Darwin/Windows archive integrations remain.
Pending APT installs from before Protected observation can finish, while fresh
native removal checks still preserve protected packages. Native version changes
no longer discard Code ownership. Old versionful receipts require the exact
completed operation, and update shows current versus pinned version, explicitly
including a downgrade. Exact old pending command arguments remain unchanged.
Completed operations remove their verified large DEB input; changed contents and
unknown worker outcomes retain recovery evidence.

Both Ubuntu diagnostic jobs at `1206f557` reproduced root-owned `/usr/share`
mode 0777, which the reported runner image intentionally provisions. Production
correctly refuses that writable ancestor. The disposable desktop job now records
and establishes a physical root-owned 0755 `/usr/share` before acceptance, without
recursively changing paths or changing package modes. This fixture precondition
is not a production installation repair. Native package and GUI proof remain
separate; final rerun evidence is recorded below when available.

The narrow host-precondition rerun at `b1f4a191` (run 38090322930) passes the
original native Code lifecycle on Ubuntu amd64 (job 114325258197, 51.39 s) and
arm64 (114325258288, 61.28 s), with repository/key/true-answer sentinels preserved
through every phase. Both logs show only `/usr/share` changing from root-owned
0777 to 0755. This establishes the hosted health cause and fixture correction;
it does not cover the later review corrections/downgrade or actual GUI launch.


The corrected provider passes actual disposable Ubuntu 26 ARM/root (104.15 s)
and Debian 13 ARM/nonroot-sudo (97.57 s) lifecycle probes. Each verifies original
worker-reply recovery, repair, a real external dpkg epoch-version upgrade,
reviewed APT downgrade back to the exact pin, package-owned desktop entry
presence/removal, verified DEB-cache cleanup, and unchanged personal repository,
key and prior true debconf answer. The failed-before native downgrade trace is
retained. Both containers were removed after raw evidence capture. Focused
APT/Code/controller/planner regressions, race checks, vet and Linux/Windows
cross-compilation pass. Integrated Ubuntu amd64/arm64 GUI acceptance remains
required; container package evidence does not establish sandbox/window behavior.
Cleanup covers saved transaction inputs and resumable pre-correction operations;
it does not sweep unknown historical/orphan cache directories.


### 2026-10-11 — Native shell fixtures use native provider observations

The v5 Linux core failed two fixtures before shell execution: foreign Windows
composition could not inspect VC++ registration, and Linux zsh had no connected
APT provider. Native shell lifecycles now retain the actual OS identity and wire
the observed provider. Pre-existing healthy zsh/VC++ prerequisites remain reused,
with post-removal preservation checks; fresh-provider lanes retain owned install
coverage. Cross-OS PowerShell composition is covered by the active generated
profile Pester process tests, including real PSReadLine order/keymaps.

A new disposable Ubuntu 26.04 arm64 container passes selected Bash features
(2.68 seconds) and real interactive zsh/plugin/shared-fzf lifecycle (17.75 seconds),
including unchanged APT inventory and personal hook. The active PowerShell profile
suite passes all 24 tests; focused race, vet and Windows compilation pass. Actual
Windows pinned-module/native registration acceptance and final-head hosted gates
remain required. No product code, host-package mutation or workflow contract changes.

## 2026-10-11 — Native clipboard no longer retains removed shell profiles

The `de769314` macOS Neovim job `114321418694` completed application consumption,
update and both reviewed deselection requests, then failed exact restoration of
`state/shells/env.sh`. A ready request can legitimately retain resources: the
catalog incorrectly made reused macOS `pbcopy`/`pbpaste` depend on shell
integration, so retaining those OS commands retained the profile blocks too.

The clipboard shell dependency now applies only to the Windows private archive.
The macOS commands and both Linux APT helpers use the system PATH and remain
protected when reused, changed or explicitly kept. The actual providers,
retained-dependency protection and profile baseline/publication protocol are
unchanged. Linux still installs both X11 and Wayland helpers independently of
the current session.

Failed-before/passing-after fresh-controller tests exercise actual macOS
read-only clipboard observation and profile install, update, shared selection,
selective removal, final removal and a new read-only check. Absent and personal
baselines restore exactly, including generated environment files. Complete
Neovim graph regressions cover native-helper retention on macOS and both Linux
architectures while preserving the Windows archive's shell dependency. The
corrected complete hosted Neovim lifecycle remains required; these focused
checks do not replace it.

Validation: focused profile/Bash-login/shell/clipboard/Neovim-prerequisite tests
pass (47.696s); Go vet, Linux amd64/arm64 and Windows amd64 test compilation,
EditorConfig, JSON parsing and diff checks pass. Catalog counts remain 115
resources and 22 capabilities; no archive pin or package selection changed.

### Desktop native folder and launch-mode boundaries

At `de76931`, macOS and Windows Code launch APIs return without a verified owned
window or settings witness. A read-only Foundation probe proves that fake HOME
and XDG overrides leave the native macOS home/application-support directories
unchanged. macOS acceptance now uses those real folders, as Windows already does,
with the existing reviewed adoption and exact restoration assertions. Controlled
mode variables are actually removed from child environments: empty values are
not equivalent to absence in Electron's Windows entrypoint. Real child-process
regressions and the no-GUI Swift helper check pass; corrected configured GUI
launches remain pending disposable hosted verification. Product launchers and
window/settings/ownership checks are unchanged.


### 2026-10-11 — Consolidated v6 correction status

Exact delivery head `de769314978411d63e509d260ae1e7ce51437f58` completed
[installer run 38089015234](https://github.com/luisgui1757/dotfiles/actions/runs/38089015234)
with nine of twenty jobs passing and
[quality run 38089015235](https://github.com/luisgui1757/dotfiles/actions/runs/38089015235)
with one of three passing: ten of twenty-three overall. Missing-payload Apple
CLT and all fresh APT Neovim lanes passed. The native Windows clangd smoke also
passed; the earlier clangd timeout is not reproduced at this head. Failures are
not cleared by passing substeps or diagnostic branches.

Actual `claude-opus-5-5` v5 review completed with requested xhigh effort; effective
effort was not exposed. It made 111 read-only Read/Glob/Grep calls and all 935
source/evidence files remained unchanged. One Glob permission denial involved
the review parent, not source; no provider/guard error occurred. F1 shared Bash
grammar and F2/F4/F5/F6/F7 native Code findings have focused failed-before and
passing-after corrections. F3's long-file fixture correction needs further
native proof: the first 257-character-root diagnostic fails before config load.
The narrower actual 254-character staging/config-read failure remains open.
Do not describe the successful diagnostic step as a successful Git variant:
all three variants failed init, while the separate full Git/Make lifecycle
passed in 95.35 seconds at `810f50e` (run 38090569304).

The new natural-exit process fixture at that diagnostic head observed lifetime
Total increasing from one to two. The unchanged late-member guard correctly
refused completion; fixture startup is being investigated before changing the
containment contract. Ordinary native PowerShell module discovery and Build
Tools inspection timing still need evidence-based resolution. The same foreign
Windows shell subcase caused macOS core's archive-sweep failure; the native
provider fixture correction covers both POSIX hosts.

The Node consumer regression is now included in the existing private-language
acceptance step on all four native targets, without adding a workflow/context.
Desktop folder/environment corrections run separately at `9ffa038d` in
[run 38091803814](https://github.com/luisgui1757/dotfiles/actions/runs/38091803814).
Its GUI outcomes are pending. Windows/macOS diagnostics do not mutate the
ordinary developer Mac. The merged correction source still requires its full
local gate, actual immutable review and all twenty-three final-head checks.


### 2026-10-11 — Corrected desktop launch proof and Windows Terminal route

Temporary head `9ffa038d4f57f672a2e399fc3e2f1537f05e07b3`,
[run 38091803814](https://github.com/luisgui1757/dotfiles/actions/runs/38091803814),
passes the complete macOS desktop/font job `114329573680`. The actual published
Code singleton and group have owned native windows and consume their managed
settings; Ghostty consumes its configuration; update, check, owned removal and
exact personal-baseline restoration pass (desktop 113.67 seconds). AeroSpace's
published native application launches, with configuration/tiling still explicitly
unverified until owner-granted Accessibility consent. Log SHA256:
`413927f3b2defb8ad2040faf65e9d2ecb2b9604de867b7f9def19e5df2ffc801`.

Windows job `114329573865` passes the corrected Code singleton window/settings
witness in 8.47 seconds and native fonts. The group then fails at public preview:
`tool.windows-terminal has no connected native provider`. Its unpackaged pin
and action existed but its catalog binding did not; isolated Terminal tests
used ArchiveDriver directly and could not catch application routing. The new
all-pin/native-provider regression reproduces this exact missing route across
supported target catalogs. Adding only the existing Windows archive binding
makes it pass; no driver fallback or new provider is added. Every other declared
pin already routes correctly, including FontDriver's special publication path.
Log SHA256: `ddb4dfe768d99148754ef70f14507d167e76102723697af7ed9a13a31804d9fd`.

The group failure remains failed. Full Windows group update/check/removal and
personal restoration require the corrected native rerun. The macOS diagnostic
certifies its exact scoped source, not all twenty-three delivery checks.


### 2026-10-11 — Correct the Git regression to the actual failure boundary

Windows diagnostic head `810f50e`, job `114325970355`, passes its broader Git/Make
lifecycle but all three 257-character Git-root probes fail before config loads.
That root was not a valid supported-boundary assumption. The Lua regression now
keeps the actual production failure's 254-character staging root and 266-character
`.git/config`, plus the same nested committed files beyond 320 characters and all
publication/identity/recovery checks. This is a corrected failed-before fixture,
not a production fix or Windows pass. Its 12 cases pass locally on pinned
Neovim 0.12.5 macOS; native full-consumer 254-character probes remain pending.
Neither fixture home/generation roots nor product ownership paths are shortened.

Windows PS7 → Go → PS5.1 module inheritance: **correction implemented; native
verification pending**. The quality job's Get-FileHash failures are distinct from
the Bash-launched core job, where the same extraction/COM/progress tests passed.
Fixed script children now use their current engine's built-in modules; bootstrap
binds Management/Utility manifests explicitly. The new native regression proves
the original autoload failure and checks real query/job/worker/bootstrap hashes.
The separate Build Tools reuse 30-second timeout has no identified stage yet.


### 2026-10-11 — Windows and macOS complete desktop rerun passes

At temporary head `b86a3316df3061cbdeb228381e466e7de0352064`,
[run 38092427237](https://github.com/luisgui1757/dotfiles/actions/runs/38092427237)
passes both native jobs. Windows `114331421213` completes fonts, published Code
singleton/group and Windows Terminal launch, settings consumption, update,
read-only checks, owned removal and exact personal restoration (desktop
179.19 seconds). macOS `114331421334` repeats the full
Code/Ghostty lifecycle and AeroSpace consent-boundary launch successfully
(102.01 seconds). No assertions, deadlines or product
launchers were weakened. AeroSpace configuration/tiling remains consent-gated,
not certified by a launch.

Raw log SHA256 values: Windows `73d750e055ab291227f5b65ce3017ebd8407cdfaa99f722fa650d1c1d74cede0`;
macOS `3d6b9b06f3599961aef89154fb749295cf3b953193551fe47f062a5744f4bbcf`. This closes the scoped desktop failure and proves
the missing Terminal catalog binding correction natively. Canonical final-head
acceptance still requires the complete twenty-three-job delivery matrix.

### Windows native module correction proof

At temporary head `3cedd3ed81f1ec0cf43e84d3ad9baa76ee479902`, job114332033874
ran the actual PowerShell7 → Go → Windows PowerShell5.1 boundary. The raw
Get-FileHash command reproduced CommandNotFoundException; corrected vendor,
private compiler-job, integration, durable-worker and bootstrap hashes all passed.
Extraction4.41s (including unsafe/existing-file refusal), actual COM publication
and removal3.39s, real progress1.17s and both legacy journal refusals0.14s passed.
Exact product code hashes match the frozen module handoff. The complete job
remains failed for independent process-fixture and timing-instrumentation errors.
No Build Tools deadline conclusion or full native-matrix completion is implied.


### 2026-10-11 — Remaining native evidence and rejected fixes

The module correction has matching native red/green proof, and Windows/macOS
complete desktop lifecycles pass their scoped diagnostic reruns. Linux Code's
new native downgrade lifecycle and fonts pass on both hosted architectures;
its GUI run stalled inside launcher wait and was cancelled as a failed diagnostic.
A private process-boundary reproduction proves that a GUI descendant retaining
exec-managed writer pipes can keep Cmd.Run waiting beyond its context after
launcher exit. The fixture correction uses native file handles and bounded
private diagnostic reads; root failure, timeout, owned-window/settings and final
restoration assertions remain required.

The Windows natural-exit test's single-member premise was false: its ready job
has two live members. The corrected test will prove one held member's natural
exit and capture/await all remaining private members, preserving lifetime-count
and late-member rejection. This does not authorize a production containment
relaxation. The attempted Build Tools timing instrumentation refused an ambiguous
certificate-chain marker before launching the query; it supplied no timeout
cause. Its exact marker is being corrected and the thirty-second product budget
remains unchanged.

All eight Windows Git configuration/prefix variants failed the complete checkout
sequence. At the actual 254 root, system config allowed init but repository
discovery still failed. The full artificial data-root layout also placed final
locked plugin roots at 258/259 characters; changing staging alone cannot fix it.
The canonical known-folder layout keeps those roots at 184 characters or less.
Reject the incomplete system-config fix, metadata aliases and a speculative
cache-layout rewrite. The public Windows lifecycle will observe its actual OS
application folders, while separate tests retain long files beyond 320 characters
and explicit refusal/cleanup at the unsupported 254 root. Full native Neovim
install/use/update/shared removal/final restoration remains the closure gate.

## 2026-10-11 — Desktop launcher inherited-output wait

Temporary source `a5493783d8acfe1b6cb0c17dc7bfd19bb38061e8`, run `38092318318`,
passes the corrected real Code lifecycle (including reviewed downgrade) and
native font rendering/removal on Ubuntu amd64 and arm64. Both full desktop jobs
then remain inside `vscode-singleton` for more than nine minutes, beyond the
fixture's 90-second launch context. ARM records the installed Code process
activating accessibility services. The temporary run was explicitly cancelled
to collect that phase trace; it is not a successful GUI lifecycle.

A bounded private process reproduction proves that an exited launcher can still
leave `Cmd.Run` waiting on writer pipes held by its living child, past context
expiry. Native desktop fixtures now use exclusive private regular-file output,
retaining the same 1 MiB diagnostic read bound, exit/context/error checks,
90-second context and actual executable/window/settings assertions. Owned GUI
cleanup precedes the final log read. The regression releases its exact private
child and joins the original waiter even on the failed-before path. No package,
provider, sandbox or application behavior changes. Full corrected hosted GUI
acceptance remains pending the subsequent checkpoint.

Validation: failed-before process regression reproduces the inherited-pipe wait;
corrected focused cases pass (0.553s), including explicit nonzero exit,
cancellation and output-limit boundaries. Race tests pass (2.610s); Go vet,
Linux amd64/arm64 and Windows amd64 test compilation, EditorConfig and diff
checks pass. No everyday-workstation GUI or native package mutation was used.


## 2026-10-11 — Native Windows Neovim paths and Git's repository-root boundary

At diagnostic head `3cedd3ed81f1ec0cf43e84d3ad9baa76ee479902`, run
`38092637104`, job `114332033874`, all eight controlled Git variants fail the
complete sequence. At a 254-character stage, isolated system config permits init
but both normal and explicit `.git` repository discovery fail on the next command.
The extended prefix fails init. All four 257-character controls fail init.
The encompassing Git/Make lifecycle passes in 133.30 seconds; that is not a
Neovim or variant pass. Raw log SHA-256:
`bebe8e8b22024989136ed7d5e4ea36ebac99dd4c4381cddcff7fcca84e61859d`.

The native fixture's temporary XDG root adds 75 characters over its actual
LocalAppData default. Final locked plugin roots also reach 258/259 characters,
so a compact staging name cannot correct full Lazy and sync verification.
Use real observed Windows known folders for the public lifecycle, preserving its
private HOME, complete operation identities, configuration/profile/editor-data
baselines, notes, and every install/use/update/shared/final-removal assertion.
No Git recipe, staging scheme, metadata alias or product path changes are made.
The Lua suite separates supported-root files beyond 320 characters from the exact
254-root native refusal, checking original-target preservation and stage/lock cleanup.

The real process-boundary regression fails before forwarding the explicit native
XDG setting and passes after. Windows cross-compilation and local pinned Neovim
checks are scoped evidence; actual strict default-layout Git and the complete
Windows public Neovim lifecycle remain required on the temporary hosted branch.
Do not mark the Windows Neovim failure closed before those consumers pass.

Windows query capture correction: **native proof DONE** at 8994dd0, job 114334530446.
All five original process modes, outside-process survival, late-member refusal
and natural 2→0 capture passed. Existing Build Tools reuse/compiler environment
and repeated stable observations passed 20.37s with all five queries under 11s and
the unchanged 30-second limit. The prior isolated timeout's stage remains unproved;
no timeout change was made. Full job remains failed for an independent temporary
Git known-folder probe setup error; full native-matrix completion is not claimed.

## 2026-10-11 — Existing APT alternative consumers during desktop install

The corrected launcher at diagnostic `8a1af68` passed the complete ARM Code/font/
configured desktop lifecycle, including actual Code and Ghostty windows/settings,
update/check and removal. The amd64 Code lifecycle, fonts and Code singleton GUI
passed, then the full-group apply rejected GTK4. Failure-only observation at
`7bf54fe` proved preinstalled Chrome/Edge GTK3-or-GTK4 declarations were hidden
while GTK4 was absent and appeared after the earlier approved libadwaita install.

APT now preserves unresolved declaration names and discloses those consumers for
an absent package. Exact same-plan introduction proof and the executor's strict
consumer equality remain unchanged. Removal retains libraries needed by personal
consumers; it never acquires the browser or removes its existing GTK3 alternative.
Focused controller/parser, architecture/Provides, foreign-change rejection and
legacy pending-intent recovery tests pass. Corrected full hosted amd64 desktop
acceptance remains pending; ARM's earlier pass is not final-head certification.


## 2026-10-11 — Windows native Git/Lua proof and the PowerShell language prerequisite

The strict temporary native proof passes at `43b7e544c54af3c6d1048b73a2ada80604aeb7a1`,
run `38094448973`, job `114337344648`: complete default-layout Git sequences for
lazy.nvim (stage 179/target 168/config 180/nested 514) and the longest locked plugin
(stage 195/target 184/config 196/nested 530), exact 254/config 266 refusal, and all 13
actual pinned-Neovim checkout cases with zero failures/errors. The original
Git/Make install/update/consume/check/remove lifecycle also passes in 125.21s.
Raw log SHA-256: `f7ac5f14be70be964d6fd07c36e245cf6071cce944a93036d6d2924bc0d11f03`.
The earlier quick probe sampled native folders after its own USERPROFILE override;
that temporary diagnostic mistake is corrected by sampling before the override.
No production Git configuration, staging, metadata or path layout changed.

The independent complete Neovim job at `8994dd01688075a21d4bae471a944b608ed0a9f2`
(run 38093492507/job 114334530582) reaches initial public apply/check ready, survives
an installer rebuild, and passes both notes-vault states. Its strict language
smoke has exactly one failure: pwsh is missing. Node's payload remains healthy
and byte-identical. Raw SHA-256:
`ff7a7ef42518172286b827b90d711c00f2cde263d82fb84aa14a8d72a71e41ba`.
Update and shared/final removal were not reached, so the full lifecycle stays open.

Add the existing shared PowerShell archive to nvim.sync's Windows prerequisites;
its existing command-directory assembly then supplies pwsh to real Neovim use.
Closure/order/PATH regressions fail before and pass after, including the absence
of that dependency on both Linux targets and macOS. Correct the smoke's stale
legacy PSES lookup to the existing managed-runtime helper, matching production.
Missing runtime/bundle still fails on Windows. Focused race/vet/compile/syntax
checks are scoped evidence; the corrected complete Windows lifecycle remains
required and runs independently on the same temporary diagnostic branch.

### Native receipt for the absent-consumer correction

The temporary diagnostic head `49def22d24fa9894ef39b1fce0833efec9a231d7`
passed Ubuntu amd64 job `114337270150` in run `38094424487`: Code's real pinned
install/repair/approved epoch downgrade/remove (90.56s), native font rendering and
removal (6.90s), and the complete public configured desktop lifecycle (168.42s).
The latter includes Code singleton and full-group Code/Ghostty native windows,
actual settings witnesses, update/check, final removal and personal-file
preservation. Its unchanged assertions passed after the absent-consumer fix.
ARM passed the complete lifecycle at earlier diagnostic `8a1af68` and again at
`7bf54fe`; both architectures still require the integrated final-head checks.


## 2026-10-11 — CoreText query data/code boundary

CodeQL check 114321518235 on de769314 reported unsafe quoting at font_native.go:109.
Source and read-only native reproduction reject the reported injection: fixed
SFNT names became a complete JSON array expression, with direct process argv and
no enclosing quoted string. Keep the query static and pass the serialized names
as a separate osascript argument instead. No suppression, query policy, native
registration recipe, fingerprint or ownership behavior changes.

The new boundary regression fails before and passes after. Native CoreText output
for all 12 fixed faces is byte-identical before/after, and empty/single/multiple
quote/Unicode inputs pass without code execution. This local proof is read-only;
no font install or registration ran. Focused font tests and foreign compilation
are separate evidence; a new hosted CodeQL result remains required before the
previous failed check can be considered cleared.

## 2026-10-11 — Architecture-qualified absent APT consumers

The controller regression reproduced the remaining absent-consumer boundary on
both supported architectures: a browser's pre-existing GTK3-or-GTK4 declaration
with an explicit native qualifier disappeared before GTK4 installation, then
invalidated the approved plan when the same declaration resolved to its installed
identity. Foreign-qualified and bare alternatives already passed.

APT now uses its existing observed platform architecture when matching absent
selected packages, preserving native-qualified consumers without inventing
foreign consumers. Both architectures pass install/update/removal and later pool
collection, with personal browser/GTK3 preservation. The focused APT suite also
keeps changed/unattributed consumer rejection and legacy recovery green. No
parser, native command, persisted schema or executor guard changed. This is
controller/process-boundary fixture evidence; integrated final-head native
acceptance remains required.

## 2026-10-11 — Code same-pin updates and optional input cleanup

Opus v6 findings N1/N3 are corrected: an owned healthy Code at its recorded
reviewed pin is kept by the existing planner without downloading its DEB again.
A changed version/pin still plans update (including an approved downgrade), and
failed/missing payload still requires repair/install. A legacy receipt without
the existing Desired field performs one reviewed reconciliation, then uses the
same keep behavior. Reused personal Code remains unowned.

Native success and ownership are saved before optional input cleanup. Changed
DEBs, unexpected files, redirects or inspection failures preserve and disclose
their exact directory without blocking the completed package. The disclosure
survives restart, update and removal; unknown native outcomes still cannot claim
completion or discard their inputs. No command, ownership comparator or saved
schema changed. Controller/process-boundary red/green cases and focused APT/Code
checks prove this slice; final-head hosted native acceptance remains required.


## 2026-10-11 — Completed Windows vendor journals remain readable

Opus v6 N2 is fixed in code: adding the child module-path prelude had made exact
completed VC++/Build Tools commands differ from the current encoder, blocking
observation, check and removal. For complete journals only, validation also accepts
the exact previous encoding of the same trusted fixed script. Operation identity,
program, flags, input, environment and recorded successful completion remain
bound; pending legacy commands are refused unchanged and never dispatched.

Both real controller/process-boundary regressions fail before and pass after
through completed legacy observation/check/removal preview and a new exact owned
removal. Completed history remains byte-identical. Corrupted commands and missing
matching native evidence remain errors. This fixture proof performs no native
package mutation; final integration and hosted gates remain separate requirements.


## 2026-10-11 — Windows fixture baseline and Unicode boundary fidelity

Two v6 review risks are closed in test source. The raw upstream PowerShell hash
baseline may legitimately succeed after an upstream fix; that exact success is
now diagnostic alongside the known autoload failure. Wrong/empty output or other
failures remain errors. PowerShell 7 and every corrected native boundary assertion
remain required. The native Git refusal fixture pads and checks its 254/266
boundary in UTF-16 units, preserving its actual refusal and original/cleanup checks.

The baseline policy regression fails before and passes afterward. An actual
checksum-verified pinned Neovim probe of the fixture's source reproduces old
Unicode/astral padding at 247/259 and 252/264 units, then proves 254/266 for both
and the unchanged ASCII sample. Windows compilation and source/syntax checks are
focused evidence; this slice runs no native package mutation and does not replace
the final hosted Windows fixture execution. Product/runtime/layout are unchanged.

## 2026-10-11 — Complete Windows Neovim lifecycle proved

The corrected PowerShell prerequisite and managed-bundle smoke lookup pass the
complete public Neovim lifecycle at temporary head
`70fe3496f8e771ebcd1457a895b414eff32be0e6`, run `38094883281`,
[job 114338646414](https://github.com/luisgui1757/dotfiles/actions/runs/38094883281/job/114338646414).
The unchanged test passes in 1162.47s: initial install/check, check after an
installer-only rebuild, both notes-vault states, strict smoke with 257 checks
including actual PowerShell language-server attachment, update, shared removal
with Pi still runnable, final removal and exact personal/configuration/editor-data
preservation. The six relevant fixture/spec/PSES test/smoke source hashes match
the frozen handoffs; the nvim.sync resource projection matches the approved edge.

Raw log SHA-256:
`0dfaec306f00d2978f99fc66cae750c9f05a4682fb942986e32541809f6373f4`.
The hashed native receipt records source and job metadata alongside the earlier
43b7 Git/13-case success. This closes the Windows Neovim native-proof gap described
above; the historical 8994 missing-pwsh run remains failed. All 23 exact final-head
checks and independent review remain the delivery gate.


## 2026-10-11 — Integrated v6 review closure and final gate

Actual Opus `claude-opus-5-5` completed the immutable v6 review with xhigh
requested (effective effort unexposed), 98 read-only tool calls, no permission
or guard failures, and all 1185 source/evidence hashes unchanged. Execution
summary SHA-256: `a2cb9007dccb7a0cbc298a9ebdaa0bd7df348a83721175889ec742877ec4f33c`.
Review SHA-256: `a9dacc684decc6d238b05524e1ae8095579004f5883713915ec6b96756e80fb3`.
It confirmed all seven v5 findings corrected. N1/N2/N3 are accepted and corrected
in the focused entries above; their frozen code hashes were independently
checked against the writer before and after integration. Two Windows fixture
assumptions are corrected without changing product behavior or mandatory checks.

The first v6 integrated `make -k ci` failed only the PowerShell static scanner's
apostrophe-containing comment; installer tests passed in 501.879s and the
remaining configuration/editor/shell/bootstrap/Renovate checks passed. The
comment is now plain prose without an apostrophe, and the static gate passes.
The failed receipt remains failed; no diagnostic or checker suppression was
added. Final acceptance requires a fresh integrated full gate, all-package
foreign build/test compilation, a focused review of this last correction batch,
and all 23 native/quality checks plus CodeQL on the exact published head.


## 2026-10-11 — Final focused review and unchanged-source local gate pass

Actual Opus `claude-opus-5-5` completed the focused v7 correction review with
xhigh requested (effective effort unexposed), 56 Read/Glob/Grep calls,
no permission/API/guard failures, and all 811 immutable source/evidence
hashes unchanged. It found **no material defects** in this final batch, confirmed
N1/N2/N3 fixed, and accepted the small post-v6 changes. Execution summary SHA-256:
`51d0605e4152f70d2f81d3b22ead164b280e9a3bd0fb979028038967a29808dd`. Review SHA-256:
`59f013c0dc8434da3d4711b203adc2f121901af71ea389f16cdbfe6f513c4296`.

The historical N2 provenance gap is independently closed against actual
`de769314978411d63e509d260ae1e7ce51437f58` Git objects: every vendor script/prelude
constant remains byte-identical; only the new command encoder/module binding
changes. Build Tools process source itself is byte-identical. Constant-tail
SHA-256 values are `8184e8a8caa0e1bc25827ddf475b738994fad500e40eb9a7b85f3e8018d80351`
and `294d1297182fa141686f09cae4113c0964c1bf3cc5955eefa772d1432f278ab3`.
Future recipe edits require a completed-history audit; fixtures regenerated
from a new script cannot substitute for historical evidence.

`make -k ci` passes with all 747 source hashes unchanged: installer tests 494.376s,
terminal tests 14.000s, configuration/editor/shell/bootstrap/Renovate gates green.
All-package builds and test compilation also pass for Windows amd64 and Linux
amd64/arm64 on the same unchanged source. This final append changes only Markdown;
a focused static gate verifies the finished documentation before publication.
The earlier v6 static-comment failure remains a failure in its archived receipt.

Nonblocking review risks are recorded without speculative runtime changes. Durable
retained-input disclosures match other drivers and prevent renewed deletion
authority; changed inputs still block later repair before mutation and disclose
the retained path. Architecture:all roots are absent from this catalog; such a
future addition needs consumer-matching proof. An old policy snapshot may fail
readback if GitHub adds a returned restriction: this is prior, fail-closed behavior
and cannot justify dropping the flag. Native hosted checks and separately
authorized policy write/readback remain real acceptance boundaries. The reviewer
also confirms the Windows Neovim raw log's actual PSES attachment, 257 smoke checks
and full 1162.47s lifecycle pass. All 23 canonical native/quality jobs and CodeQL
must pass at the exact published head; earlier/scoped passes do not replace them.


### 2026-10-11 — Ubuntu completion test-host correction

Final-head quality job 114349412458 correctly failed Starship/zsh startup on
insecure hosted completion directories. Diagnostic run 38098923912 reproduced
both unchanged assertions and identified root-owned mode 0777
`/usr/share/zsh/vendor-completions` and `/usr/share/zsh`; their `/usr/share`
ancestor was also writable. Run 38099014868 proved that physical/root-owned
validation followed by nonrecursive 0755 on exactly those three directories
restores strict compaudit and both tests.

The shared disposable-host helper now covers Ubuntu quality and core interactive
zsh, and replaces Desktop's existing single-parent preparation without expanding
that recipe. Production completion and all startup/GUI assertions remain intact.
Unsafe ownership, redirected directories/ancestors and unsupported host/recipe
invocations have process/filesystem refusal coverage in a private mount namespace.
The full final-head quality/native matrix still requires a new published-head run.

The exact factored helper and refusal fixture passed on disposable Ubuntu in
run 38099271054/job 114351621069 at
`b73c38d5c35c8fd55ddc7f344e4f563e3d5d32f8`, followed by both unchanged startup
tests passing. Raw log SHA-256:
`37ccfaebfb15a19c8b05d294157c8f41ebc0ff471f62b49d3275f09ca2ce3e9c`.
No permissions or packages were changed on the developer workstation.


### 2026-10-11 — Frozen completion helper verified on Ubuntu ARM

Review v8's architecture evidence gap is closed for the actual Ubuntu 26 ARM
image `20260927.135.1`: run 38100313604/job 114354731869 at
`3322d7929c941e9dbb2192d77919b66ac01582ee` passed the exact frozen helper
and refusal fixture, strict compaudit, and unchanged full `make test-shell`
(including 25 bootstrap tests and ShellCheck). The job verified source SHA-256
values before execution. All three reviewed directories, including
`/usr/share/zsh/vendor-completions`, exist as physical root:root mode 0777
directories and become 0755 through the unchanged helper. No missing-directory
accommodation or production change is justified by these observations.

The raw passing log SHA-256 is
`fe21a763e728c6ff715d785eb55e2ed91164c6ccf5d0e902667824e7fcca9f89`.
The earlier ARM diagnostic 38100200478/114354384785 passed helper/zsh checks
but its full suite failed because the temporary shallow checkout lacked the
released `v0.1.0` tag. That original failure remains preserved; matching the
canonical quality workflow's `fetch-depth: 0` fixed only the diagnostic setup.
Final amended-head required checks remain a separate delivery requirement.


### 2026-10-11 — Windows bootstrap helper naming correction

Final-head Windows quality retry 114351013020 passed dependency installation,
installer race tests (641.935s), all 48 bootstrap/migration Python tests, all 49
Pester tests and the Lua suites. It failed on the single PSScriptAnalyzer 1.25.0
`PSUseSingularNouns` finding at the bootstrap module-binding helper. Renaming
that helper and both AST lookup references changes no binding, extraction,
cache or invocation behavior and adds no suppression.

Focused local extraction/parser and baseline-policy race checks pass (2.694s),
as does Windows amd64 test compilation. Native run 38100201494/job 114354387651 at
`6ebcdd0ceeb203f2ca633d6839a4edb003749917` also passes: actual PSScriptAnalyzer
1.25.0 reproduces the original one finding and reports zero after the rename;
all four PS5.1 extraction cases pass (2.21s), followed by the real PS7 → Go →
PS5.1 inherited-module regression (2.53s). The original quality failure remains
failed. Native raw log SHA-256:
`742b19bc24f1e6b81c569d7a625323f4f9253a8d5dcd1e55f8ab286c5443b086`.
The separate Gallery provisioning failure was not reproduced on the same image:
the unchanged install succeeded with PSGallery already registered, so no
repository/provider change was made.


### 2026-10-11 — Windows selected-shell fixture honors shared removal

Final-head core job 114349412548 failed only the selected-shell archive fixture:
it deselected its shared PowerShell without explicit removal authorization,
then incorrectly expected the retained runtime's shell integration to disappear.
The whole owned block and exact personal suffix remained; no personal edit was
lost. All independent language, Git/Make and public migration steps passed.

The fixture now first proves default retention of its owned PowerShell archive
and exact shell/personal bytes, then authorizes only `tool.powershell` removal.
Existing archive absence, exact personal/BOM restoration, other profile cleanup
and pre-existing VC++ runtime checks stay intact. Planner/runtime behavior and
POSIX flow are unchanged. Focused shared-retention/profile race tests pass
(2.226s), as does Windows test compilation. The complete native Windows shell
lifecycle passes in 52.26s at `b974f8261a32554e7684bb3253bfce1c474807a0`,
run 38100700339/job 114355857812. Both default retention and explicit removal
are observed before the unchanged final preservation assertions pass. Raw log
SHA-256: `2ccef4a8982dbd54c3d35a8e1494ff0f4cc2dd02d6166fb797d5cfc4438ef154`.
The original final-head core failure remains recorded; this scoped pass does
not replace the next full published-head matrix.


### 2026-10-11 — Opus v8 clears the disposable Ubuntu host correction

The actual `claude-opus-5-5` read-only review reports no material safety or
correctness defects and calls the seven-file runner correction the smallest
complete change. The client recorded 43 Read/Glob/Grep calls, no API, guard or
permission errors, and 769 unchanged supplied files. xhigh was requested;
effective effort is not exposed. Review SHA-256:
`3c29b887a7f70520345fb914e5765b18c0b27229dd253eecff35487965fa8add`.
The architecture assumption about the ARM vendor-completion directory is closed
by the exact frozen-source native ARM receipt above. The dedicated disposable
VM has no root/sudo privilege boundary for the noted check/chmod interval; full
prevalidation and postconditions remain. No product security bypass, additional
provider or absence accommodation is justified. The original four failed
canonical contexts on `044f978` remain failed (19/23 passed); scoped corrective
proofs do not replace the next amended-head matrix. CodeQL's three analyses and
aggregate passed on that checkpoint.


### 2026-10-11 — Windows compiler-query pointer boundary corrected

Windows Neovim job 114349412580 at `044f978` passed strict startup, update and
selective removal, then refused final removal at the compiler-query membership
guard. Diagnostic job 114354654925 reproduced a zero baseline followed by
Total45/Active0 despite completed exact HANDLE waits. The problem was a lost
query output buffer: the pinned x/sys plain-uintptr wrapper can outlive the
caller's movable Go stack address. The typed boundary now preserves each buffer
until the annotated native call, including the corresponding fixed job-limit
input. Membership, Active-zero, exact HANDLE, atomic assignment, output and time
bounds remain unchanged. Failure-only counts and bounded private-job IDs retain
useful evidence without paths, environment or a machine-wide process inventory.

At exact temporary head `97d5596ef74cfd04d4360af37295157997bd944d`, run
38100961481/job 114356621691 reproduced the old Query defect against an actual
empty Windows job: the successful call left all-ones accounting sentinels intact.
The corrected accounting/member/limit cases pass at 128 varied stack depths,
alongside every existing success/error/cancel/output-limit, outside-survival,
natural-exit and deliberate late-member refusal case. Six real compiler
reuse/environment lifecycles pass (34.93, 23.70, 23.95, 23.72, 23.65 and 23.78s).
Local Windows compilation/vet and focused provider race checks also pass.

That diagnostic job remains **failed**: its separate expected-red Set test did
not reproduce lost input and deliberately threw; the corrected test and reuse
steps both concluded success. Old PID-buffer failure was likewise not reproduced.
The Set/PID corrections follow the same documented unsafe-pointer contract, with
passing real limit readback and member-query coverage, not a claimed native red
for each call. No repeated series was used to erase this limitation. Raw log
SHA-256: `9eb5ac8fb77974830205023f4ec0756260e2599ecda46ee08fb369e06b11fe55`.
The [review ledger](reviews/2026-10-08-interactive-installer-overhaul.md#2026-10-11--accepted-windows-job-buffer-lifetime-defect)
records source and compiler evidence. The original failed full lifecycle stays
failed; all final amended-head native checks, including complete Windows Neovim
removal, remain required before delivery readiness.


### 2026-10-11 — Consolidated final correction gate and Opus review pass

The complete `make -k ci` gate passes with all 749 supplied source hashes
unchanged. Installer tests pass in 512.970s; unchanged terminal tests validly
reuse their passing Go cache. Configuration, editor, shell, bootstrap/migration,
policy and official Renovate checks also pass. All-package Windows amd64 and
Linux amd64/arm64 builds and test compilation pass against the identical source.
These local/cross-compile proofs do not substitute for native lifecycle checks.

Actual `claude-opus-5-5` final focused review finds no material defects in the
Windows API buffer boundary, bootstrap helper rename or shared-shell fixture.
It confirms minimal completeness and unchanged completion/ownership safeguards.
The client records 46 read-only calls, no API/guard/permission errors and all
796 snapshot/evidence hashes unchanged. xhigh was requested; effective effort
is not exposed. Review SHA-256:
`5ea58e925d64dbe1710f3915f9c3b8e4fc4ba7b928044e3fad6387d1a42bb17d`.
No runtime or test change follows that review.

The cold-call sentinel reproduces the original Query defect in an isolated native
invocation; warmed full-suite calls need not reproduce it. The same comment
promises fresh goroutines and varied stack depths, not fresh native procedure
lookup for every subcase. Keep this proof limit explicit alongside old Set/PID
non-reproduction. The original failing complete Windows Neovim lifecycle is the
mandatory highest-fidelity regression at the amended head. The reviewer recommends
no change to the existing one-member fixture's small accounting window. These
nonblocking observations do not justify new process machinery or weaker guards.

The four earlier canonical failures remain failed. This source is ready for the
one-commit amendment in PR #87; all 23 canonical native/quality contexts and
CodeQL must pass on that exact published head before the PR is ready. Required
context cutover remains a separate explicitly authorized owner action before
merge. No live policy, daily-Mac native packages, fonts or GUI were changed.


### 2026-10-11 — Windows desktop cleanup and fresh launch evidence

At final head `127e35d676c69a65068b55e15c0c3508c6a7f3dc`, Windows Desktop
job 114360377438 passed fonts, Terminal settings and the VS Code singleton,
then failed the second Code invocation in deferred cleanup with
`TerminateProcess: Access is denied.` Raw log SHA-256:
`19ff2311be0276b5a10586f87013d88e7e13ad6de1e446c0a76e95ef80841c41`.
Windows documents that an already terminated process produces this error;
the old fixture distinguished only os.ErrProcessDone. The fixture now validates
the acquired process HANDLE's image under its private root before signaling it,
and accepts a termination denial only if that exact object is already signaled.
Live denial remains an error. The existing cleanup deadline and outer inventory
are retained, and POSIX signal behavior is unchanged.

The two Code invocations also reused one settings witness in their shared home.
Each invocation now has a unique absent witness, preserving the preceding file.
The local filesystem regression fails before this change and passes afterward;
the native lifecycle must obtain fresh extension-host evidence on both launches.
Windows compilation/vet and the focused local race test pass. Actual Windows
kernel and full public Desktop lifecycle proof remain pending in the separate
scoped diagnostic; the original final-head job remains failed.
