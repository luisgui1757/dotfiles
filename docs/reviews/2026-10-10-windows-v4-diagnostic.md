# Temporary Windows v4 native diagnostic candidate

This checkout starts at `28c92f5c8c2ec0e0f68ad9616660454e5991b0b4` and composes
the exact accepted frozen patches in this order:

| Frozen patch | SHA-256 |
| --- | --- |
| `nvim-git-longpaths.patch` | `986c0793cfa9a66287c0baa5c1f13ad9dac5b63e7aafbae0d5d80d80ece8f004` |
| `v4-shortcut-progress-code.patch` | `fa653212b8e4ad2a56188ea842731efe9308440b1b9b406119ccdb597224ddc8` |
| `v4-desktop-query-code.patch` | `257b4e85110ae173f94efba80ee32e562dafe9a7e09ee73b92d57e1aae7a7248` |
| `v4-bootstrap-extraction-code.patch` | `a1fdd060644fee6478071af38c9cc41b19c4f32eb621f4076e93e084faf6a0a0` |
| `v4-git-runtime-diagnostic-code.patch` | `3143478f4bec47121be9a8879dd4dd40971581035edcc0dc0bb4d1c36788bc17` |
| `v4-query-rundown-code.patch` | `c69a710487d8849a6047b96c3078c7d62f54d23b9a93e0efaba30d6bba58f8b3` |

Their matching handoff manifests and raw evidence remain in the task artifact
directory. Their guide/status/review changes are reconciled here, preserving
append-only review history. No guessed GCM or process-completion fix is added.

The temporary `windows-diagnostic.yml` runs only on pushes to
`diagnostics/installer-windows-v4`. It uses the repository's exact pinned checkout
and setup-go actions, read-only contents permission, Windows Server 2025, Go
1.27.1 and an explicit native Windows amd64 assertion. Commands run in the Go
module directory. Ordinary tests exercise system PowerShell 5.1 extraction/COM,
real encoded progress and the exact-HANDLE process-tree boundary. The original
opted-in private Git/Make lifecycle runs afterward even if those tests fail.
Both steps retain their nonzero exit and the job remains failed on either failure.

The Git probe keeps the original payload paths and pinned bytes. Fixed step labels
and GCM tracing on the original private version probe identify its native failure;
secret tracing is disabled and no authentication is performed. The process-tree
test still fails if its exact child HANDLE is unsignaled when inspection returns;
later diagnostic waiting cannot turn that failure into a pass.

The root agent will inspect and publish one temporary diagnostic branch, collect
the native evidence, then remove that branch. There is no PR for this checkout.
This temporary workflow and note must not enter the delivery commit or substitute
for its required native acceptance jobs. The delivery remains the existing PR
with a single commit. Windows compilation and focused static checks are local
preflight only; the native diagnostic result is pending publication.

Local preflight passed: Windows amd64 test-binary compilation, all changed-file
EditorConfig/gofmt checks, workflow YAML lint and Bash syntax, explicit branch/
permission/working-directory/failure-preservation checks, and `git diff --check`.
No full gate was repeated and no commit, branch publication or native mutation
was performed while assembling this diagnostic checkout.
