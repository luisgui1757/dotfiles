#!/usr/bin/env bash
# Validate renovate.json with Renovate's schema under Renovate's supported
# Node runtime. Renovate currently supports the Node 24 LTS line; running the
# validator with a newer odd/current host Node can emit EBADENGINE.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v npx >/dev/null 2>&1; then
    echo "skipped renovate: npx not installed"
    exit 0
fi

export NPM_CONFIG_CACHE="${NPM_CONFIG_CACHE:-${TMPDIR:-/tmp}/dotfiles-renovate-npm-cache}"
mkdir -p "$NPM_CONFIG_CACHE"

# shellcheck disable=SC2016 # expand node version inside the Node 24 subprocess.
npx --yes --package node@24.11.0 -- bash -c '
set -euo pipefail
echo "renovate validator node: $(node -v)"
npm exec --yes --package renovate@latest -- renovate-config-validator --strict renovate.json
'
