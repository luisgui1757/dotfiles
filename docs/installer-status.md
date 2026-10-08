# Interactive installer status

**IN PROGRESS — NOT READY TO SHIP.** Updated 2026-10-10.

[PR #87](https://github.com/luisgui1757/dotfiles/pull/87) is the single delivery PR
and retains one implementation commit. #88 is incorporated and closed; #86 is
superseded. Published checkpoint `077683f` passed the full local gate and all
six foreign build/test-compilation checks, and received an actual Opus 5.5
full-delta review. Its quality matrix passed all three jobs. The completed installer matrix has seven passing and thirteen failing jobs. The current correction batch addresses the identified
causes and review findings. macOS Node payload drift and desktop startup remain
under diagnosis. These results do not establish readiness.

## Scope and roadmap

Supported targets are macOS arm64, native Windows amd64, and Ubuntu/Debian on
amd64/arm64. Linux native acceptance uses Ubuntu 26.04 and Debian 13. WSL uses
the ordinary Linux lifecycle. Fedora, macOS Intel and separate WSL provisioning
are outside this release. Nix and chezmoi are removed, with explicit migration
and passive historical live-link sources instead of a compatibility runtime.

| Milestone | Implementation | Required verification still outstanding |
|---|---|---|
| 1. CLI tools and prerequisites | DONE: all providers, independent fd/ripgrep choices, APT, Homebrew and Windows toolchains connected | Corrected Windows Build Tools, Rust and PortableGit lifecycles; Apple tools provisioning |
| 2. Neovim | DONE: private locked plugins, parsers, Mason tools and language dependencies | Full selection, real language use, update, shared removal and personal-data preservation on all four targets |
| 3. Product integrations | DONE: desktop apps/fonts, scoped settings, Sentinel and development checks | Corrected native desktop/font and configured GUI lifecycles |
| 4. Public cutover | DONE: verified Go bootstrap, migration entrypoints and retired old runtime | Windows public bootstrap/migration lifecycle through PowerShell 5.1 |
| 5. Delivery | IN PROGRESS: one PR and one commit | Green final native checks, full independent review, required-context transition and ready PR |

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
| Windows Build Tools | Earlier progress stderr corrupted JSON; v3 fresh owned removal then blocked on a running consumer | Dedicated bounded inspection job contains and reaps compiler-probe descendants; stdout/stderr remain separate. Process regressions and repeated real compiler observations are added; Windows native execution and full owned lifecycle rerun remain required. The removal log did not identify its consumer. |
| Windows Rust | MSVC could not open a 273-character physical stdlib path; unrelated installer rebuilds also changed desired identity | At `077683f`, real rustc compile/run, offline Cargo and Clippy, check and removal pass with a 274-character stdlib path. Desired identity now follows only the launcher recipe while exact copied-byte provenance remains. Native public rebuild/check proof is required for that latest change. |
| Windows PortableGit | At `077683f`, extraction succeeds, then Git rejects `GIT_CONFIG_GLOBAL=NUL` and CMD loses its self-deleting post-install input | Use Git’s `/dev/null` spelling and execute an unchanged, exclusively staged sibling script. Nonzero exits, unfinished cleanup and runtime failures remain fatal. Focused regressions and Windows compilation pass; actual CMD and Git/Make lifecycle reruns remain required. |
| Windows public bootstrap | Download and verification now complete quickly, but script-module ZIP extraction consumes the fixture deadline | Replace per-entry PowerShell module work with the built-in .NET extractor. Exact-byte, literal-path, error and preservation regressions pass; system PowerShell 5.1 and full public migration/setup remain required on the existing deadline. |
| Windows quality | Git Bash wrapper shadowed PATH doubles; equivalent MSYS/native directories compared unequal | Corrected fixture boundaries. The Windows quality job passed at `077683f`; final-head quality remains required. |
| Apple developer tools | Preservation now passes; receipt directory `private` is wrongly treated as an outside payload file | Inspect documented `--only-files` output; outside/noncanonical files and query failures remain fatal before moves, with bounded diagnostics. Focused red/green passes; native provisioning rerun required. |
| Windows Terminal | Actual UIA Settings Save succeeds, then equivalent `closeTab` action serialization loses ownership | Normalize only the exact argument-free string/object forms, preserving original fingerprints and raw restoration. Focused lifecycle/red-green passes; failure-only owned-field diff retains actual UIA proof for the required native rerun. |
| Linux Neovim | Normal startup bytecode cache exceeded filename limits; fresh APT jobs then rejected residual-state Git installation | Both Ubuntu Neovim lifecycle jobs passed at `077683f`. APT remove/reinstall correction has real Debian/Ubuntu conffile-preservation proof; all four fresh prerequisite lanes need the next hosted run. |
| macOS Neovim | Final verification rejects the Node payload; reference diagnostic failed on a logical temporary-path alias | Resolve only the private reference parent, with actual red/green alias and preservation regressions. Node integrity remains enforced; payload drift cause still needs the native entry diff. |
| Windows Neovim | v3 reviewed/fresh plans differ only in compiler consumers: absent → `vctip.exe` | Compiler inspection captures consumers before its probe spawns the surviving child. Isolate that query's process tree without relaxing consumer or approval checks. Windows native regression and full Neovim rerun remain required; no automatic apply retry. |
| WezTerm desktop | Earlier native startup failures | Removed from all OSes by explicit owner scope decision. Historical failures are not fixes or passes; remaining desktop apps still need native acceptance. |
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
