# Interactive installer status

**Implementation complete; delivery acceptance is the current PR check set.**
Updated 2026-10-10. This document records scope and dated evidence. Current
readiness comes from [PR #87](https://github.com/luisgui1757/dotfiles/pull/87):
all 23 required native/quality checks and independent review must pass before it
is ready. Dated results below certify their stated source, not a later head.

[PR #87](https://github.com/luisgui1757/dotfiles/pull/87) is the single delivery PR
and retains one implementation commit. #88 is incorporated and closed; #86 is
superseded. Published checkpoint `28c92f5` passed the complete local gate and all
six foreign build/test-compilation checks. Its completed hosted matrices passed
14 of 23 jobs (12 installer, two quality). Windows lifecycle, macOS/Linux desktop and active-shell coverage corrections
are consolidated in the delivery source. Actual Opus 5.5 reviewed the immutable published source and found
three additional coverage/palette/passive-source gaps; final source review and
native acceptance remain required. These results do not establish readiness.

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
