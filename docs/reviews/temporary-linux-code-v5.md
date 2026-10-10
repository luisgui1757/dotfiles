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
