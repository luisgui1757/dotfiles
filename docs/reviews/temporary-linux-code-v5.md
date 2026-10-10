# Temporary Linux Code diagnostic

This branch-only diagnostic starts from delivery
`de769314978411d63e509d260ae1e7ce51437f58`. It is published only on
`diagnostics/installer-linux-code-v5`, with no pull request, and does not enter the
delivery commit or replace its acceptance contexts.

The two actual jobs use `ubuntu-26.04`/amd64 and `ubuntu-26.04-arm`/arm64, the same
pinned checkout/setup-go actions and Go 1.27.1 as the canonical Desktop lifecycle.
Each asserts the actual Go host target, provisions the same apt-utils/display/font
inspection prerequisites, then opts into the unchanged
`TestNativeLinuxCodeDEBLifecycle` command and 15-minute deadline. No GUI fixture or
unrelated suite runs. Checkout credentials are not persisted; permissions are
contents-read only. Failure remains nonzero, with no skip-as-pass, continuation
exception or synthetic status.

The Code lifecycle uses the existing disposable hosted-runner guard and private
operation state while exercising the native package database. Its checks require
absent Code, actual install/recovery/repair/update/remove, and preservation of
personal repository, key and debconf fixtures. Any added health diagnostics run
only on failure; the original failure is retained. No native package command is
run on the developer workstation.

The accompanying external handoff records the exact composed source and patch
hashes, diagnostic patch provenance and local compile/static checks. Native cause
and lifecycle completion remain unverified until this exact published diagnostic
head finishes. The parent agent owns attestation, publication and later temporary
branch cleanup.

Composed failure-only metadata patch: `linux-code-health-diagnostic.patch`,
SHA-256 `826ef1311223aad0c3a94451e0bc8606666d48d58bbbd015a67e94cd1c4cdb5f`,
from the Linux package implementation lane at the same exact base. It preserves
the existing UID/mode/link predicates and empty successful output. Its focused
process regression covers owner, mode and link mismatches plus healthy silence.

Both first diagnostic jobs report the hosted image's physical `/usr/share` as
root-owned mode 0777. The next disposable-runner attempt explicitly establishes
that single parent directory as root-owned 0755 before the existing lifecycle.
It verifies non-symlink/root ownership first and logs the exact mode before and
after. This is a nonrecursive fixture precondition, not a production health-guard
relaxation or payload repair; no developer-machine permission is changed.
The Linux implementation lane supplied the reviewed workflow patch with SHA-256
`17d7dc86b315a104ff70dfb004b4c4d3fece4c72908165be4d97aa112ffd9908`.

## 2026-10-11 — Current configured Linux Desktop acceptance

This next branch-only checkpoint starts at diagnostic `b1f4a191` and copies the
current parent-integrated Linux Code/APT/planner/catalog corrections, combined
desktop native fixture/helpers, clipboard graph regression and current archive
pin/Node preparation tests from the delivery writer based on `de769314`. The
external source manifest records exact bytes. Unrelated shell, Git/Lua and
Windows query corrections are outside this diagnostic subset.

The same two Ubuntu architectures now run all three canonical Linux Desktop
steps in order: actual pinned Code install/lost-outcome recovery/repair/approved
downgrade/removal with repository/key/debconf preservation; native font
installation/rendering/check/removal; and the compiled public desktop lifecycle.
The latter exercises VS Code alone before the complete VS Code/Ghostty selection,
actual configured windows/settings, update, owned-process-only cleanup and
selective/full removal with personal launcher/configuration preservation. It
uses the package-owned canonical Code launcher and all unchanged strict process,
window, witness and ownership assertions. No shortened paths, sandbox flags,
global policy relaxation or fixture skips are introduced.

Each existing per-step deadline remains 15, 12 and 70 minutes respectively; the
job's aggregate 100-minute deadline matches canonical Desktop acceptance. The
nonrecursive `/usr/share` root-owned 0755 disposable-host preparation remains
identical. Fonts and packages mutate only explicitly opted-in GitHub-hosted
jobs. No native GUI, package, font or user policy mutation runs locally.

This workflow and temporary note never enter the delivery commit, do not open a
pull request and do not replace required acceptance contexts. The delegated
implementation agent is authorized to update only the existing diagnostic branch
with an exact old-head lease. Local focused/compile/static checks precede that
push; actual native results must still be retrieved and attributed to its exact
published head.

Preflight passes on the composed source: focused Code/clipboard/desktop child
environment tests (5.681s), Linux amd64 and arm64 test compilation, YAML parsing
and exact canonical step comparison, EditorConfig, gofmt and diff checks. Native
execution remains pending the two actual hosted jobs; compilation is not proof
of configured application consumption.

## 2026-10-11 — Bounded launcher output correction

The `a549378` attempt passed both real Code and font steps, then remained in the
VS Code singleton fixture for over nine minutes. It was cancelled after a private
process reproduction established the inherited-pipe wait; cancellation is not
GUI success. The next checkpoint changes only native fixture launcher capture
to private OS file handles with bounded reads, preserving exit/context failures,
the same 90-second context and every owned/window/settings assertion. The exact
same two jobs rerun all three Code/font/GUI lifecycle steps; no preceding test is
removed or replaced. The application/process diagnosis is recorded in the
canonical status/review append for eventual parent integration.

## 2026-10-11 — Architecture-specific APT approval evidence

At `8a1af68`, ARM passed all three complete native steps, including actual
Code/Ghostty windows, configuration witnesses, update/check and final removal.
The amd64 job passed Code, fonts and the Code singleton GUI, then rejected the
next selected group at the `libgtk-4-1` approval guard. The generic error does not
identify the changed field. A temporary failure-only read-only fixture diagnostic
records the saved next operation, fresh observation, native package/consumer
metadata, provider ledger and receipts, bounded to 64 KiB and 90 seconds. It
never retries, authorizes or changes a package. The strict original lifecycle
runs again on both architectures; the earlier amd64 result remains failed.

## 2026-10-11 — Canonical absent-consumer correction

The failure-only diagnostic proved GTK4's originally empty consumer list gained
the preinstalled Chrome and Edge packages after the owned libadwaita install.
Keep their declared absent GTK4 alternative visible in preview; no executor
comparison is relaxed. The controller regression reproduces the same error on
the old source and passes the complete focused lifecycle with outside browser
preservation on the correction. Run only the affected amd64 diagnostic lane at
this checkpoint; ARM passed the complete lifecycle twice already. Final delivery
acceptance still requires both architectures at the integrated final head.
