# Temporary Windows native proof (2026-10-11)

This file and the adjacent probe helpers belong only to the existing diagnostic
branch. They are not production Git changes or delivery capabilities.

The 8994dd0 quick-job log (job 114334530446, SHA-256
`aa0e4178c2b441f40a8f3abb7fa3c5bb22d56b9dc75634421d5363aa4f817e0c`)
shows a setup error in the new default-path probe: it queried native folders after
Git/Make's fixture had already set USERPROFILE to its private Unicode home.
There is no LOCALAPPDATA assignment in that fixture; native known-folder
resolution nevertheless returned its expanded private profile. Capture the real
folders at fixture entry, before any personal environment override, and pass that
observed value to the unchanged strict default-layout proof. Do not shorten or
relocate any product path. The independent full Neovim job samples its folders
before these unrelated Git/Make overrides and remains valid and undisturbed.

The quick job additionally provisions checksum-pinned Neovim through ArchiveDriver,
runs the actual minimal_init and the unchanged pinned checkout spec with the
already verified managed Git first on PATH, then checks and removes the private
archive. Require all 13 Windows cases, including the exact254 root refusal,
original-target preservation, lock/stage cleanup and successful nested files
beyond320 characters. The normal Plenary timeout and native job deadlines remain.
All assertion failures are fatal; no log-only success, skip, retry or fake command.

Windows amd64 test compilation and Go vet are local checks. Native outcomes must
be added only after the hosted jobs complete; the ongoing 8994dd0 complete public
Neovim install/use/update/shared/final-removal lifecycle is a separate required
proof. No branch publication should cancel that running job.

This next temporary run has one job: the original complete Git/Make lifecycle
with the corrected default-folder probe and the 13-case Lua suite. The already
attested process, module and compiler timing stages are not repeated here. The
complete public Neovim lifecycle remains attributed to source
`8994dd01688075a21d4bae471a944b608ed0a9f2`, run 38093492507, job 114334530582.
There is no concurrency cancellation or alteration of that running job. No
canonical delivery workflow changes or required checks are part of this slice.
