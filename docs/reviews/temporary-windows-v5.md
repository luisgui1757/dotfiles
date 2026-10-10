# Temporary Windows v5 native diagnostics

This temporary branch starts at exact delivery
`de769314978411d63e509d260ae1e7ce51437f58` and is published only on
`diagnostics/installer-windows-v5`, with no pull request. Its workflow, probe,
probe call and this note are excluded from delivery.

The existing Windows 2025 runner, pinned checkout/setup-go and Go 1.27.1 execute
the new real private-child exit-between-snapshots regression, all five original
immediate exact-HANDLE/outside-process cases and unchanged late-member refusal.
The narrow product correction removes only the invalid equality between two
independent kernel snapshots. Its frozen patch is
`windows-query-exit-race-all.patch`, SHA-256
`bd8807d31175e7ec6c3f4829dbf826da28b5c38cb801fd751cd2f6a08ea8229c`.
Cross-compilation is not native proof; fresh vendor acceptance remains separate.

The Git step still runs the full explicitly opted-in private Git/Make lifecycle,
including installation/update, LFS/GCM/SSH/Bash/Make use, integrity, personal
configuration preservation and removal. A bounded additional read-only experiment
runs after installation against the same physical 257-character fixture root:
current isolated command config, explicit extended git-dir and an isolated fixed
system config. Each variant logs actual init/remote/fetch/checkout/revparse/status
outcomes. Candidate failures are diagnostic results, not a claimed Neovim fix;
none changes product invocation or turns a failed Neovim lifecycle green.
The unchanged probe artifact is `nvim-git-config-boundary-probe_test.go`, SHA-256
`cde6af37a3e4f1b93e6455aa0f9ce3d2cf7538ae56261aa398a1795d14aed146`.

The workflow retains nonzero lifecycle failures. Its second step runs even when
the first fails to collect independent evidence; there is no continue-on-error
or success alias. Permissions are contents-read, checkout credentials are not
persisted, and actual Windows amd64 is asserted. Native commands run only in the
disposable hosted environment; no workstation native package changes occur.
The parent owns source attestation, publication and temporary-branch cleanup.

The old native capture error combines three predicates without raw counters, so
its exact interleaving is not identifiable from the log alone. The regression
forces the independently invalid cross-query Active equality; incomplete same-call
lists and newly added lifetime membership continue to fail.
