# Renovate major installer update review

PR #82 builds on the reconciled patch/minor pins from PR #81. It upgrades Pi
0.99.2 to 1.0.4, the pylatexenc build backend from setuptools 83.0.0 to 84.0.0,
and the Renovate validator from 43.288.0 to 44.138.0. Node 24.21.0 stays within
Renovate's supported Node 24 range. All current docs, shell/PowerShell fixtures,
and hosted Pi version assertions move with the executable pins.

## Artifact verification

The original version-only update retained the previous Pi SRI and setuptools
wheel hash. Actual downloaded bytes failed those old identities. The corrected
pins match both official registry metadata and independently computed digests:

- [Pi 1.0.4 tarball](https://registry.npmjs.org/@earendil-works/pi-coding-agent/-/pi-coding-agent-1.0.4.tgz):
  `sha512-+956nfMFHr5lDUVY/2Q4k+YzojzBuCaBXFgj0eSlXVGr7QVliVddKdc1Pz6yVg1dOlJQmb67doOVrlMsIcIdaw==`.
- [setuptools 84.0.0 wheel](https://pypi.org/project/setuptools/84.0.0/#files):
  `51a52592b3b99e102b609654876bd65f19f999935166d1352678931132b0c670`.

The cross-file pin guard failed before the mirror corrections and passed after.
The existing npm tamper/failure/idempotence and converter hash/build tests remain
active; no checker or installer verification is suppressed.

## Compatibility and limits

A real isolated npm global install of the verified Pi tarball plus all seven
exact-release companions passed CLI version/help, loading each canonical theme,
resolving the managed newline keys, and recursively checking that all eight Pi
modules resolve to 1.0.4. No provider credentials or API calls were used.

The [upstream changelog](https://github.com/earendil-works/pi/blob/v1.0.4/packages/coding-agent/CHANGELOG.md)
introduces a fullscreen default and renames `azure-openai-responses` to `azure`.
README documents the regular-scrollback option and Azure migration. Providers,
authentication and display preferences remain local; setup does not acquire
ownership of them. Pi 1.0.1 also removed its npm shrinkwrap. The seven exact
companions prevent mixed monorepo APIs, but other transitive npm dependencies
are not locked by this repository.

A temporary Python virtual environment installed the hash-verified setuptools
84 wheel and built the hash-pinned pylatexenc 2.11 source without build isolation;
`latex2text --version` succeeded. Full local `make ci`, independent review and all
required hosted checks are mandatory before merge. Hosted Windows covers the
PowerShell checks skipped on the local macOS machine.

The first major-update local gate caught a stale regex-escaped Pi version in
the exact npm argument assertion. That assertion and the matching PowerShell
regex fixtures now require 1.0.4. The hosted Windows Terminal regex correction
from PR #81 is also retained. No assertion was relaxed.
