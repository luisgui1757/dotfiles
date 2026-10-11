# Temporary Windows Git inspection diagnostic

This branch is based on exact final127e35d. The original canonical Windows Neovim
job114360377352 remains failed. The temporary workflow runs the same original
full public fixture, native runner, environment, test timeout and pinned actions;
no job aliases, ignored errors or reruns of the delivery workflow are introduced.

Only the existing failed tree-inspection boundary gains bounded metadata and a
read-only Windows identity probe. The fixture adds failure-only Git provenance.
There is no fingerprint, retry, snapshot layout, package or acceptance change.
The native probe is subsequent evidence, not authority to replace a failed scan.
This workflow and this note must not enter the delivery commit. No new PR is
created and this diagnostic branch must not be merged.
