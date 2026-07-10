#!/usr/bin/env bash
# Validate renovate.json with Renovate's schema under Renovate's supported
# Node runtime. Renovate currently supports the Node 24 LTS line; running the
# validator with a newer odd/current host Node can emit EBADENGINE.
set -euo pipefail

RENOVATE_NODE_VERSION="24.18.0"
RENOVATE_VERSION="43.257.4"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v npx >/dev/null 2>&1; then
    if [[ "${CI:-}" == "true" ]]; then
        echo "FAIL: npx is required in CI to validate renovate.json" >&2
        exit 1
    fi
    echo "skipped renovate: npx not installed"
    exit 0
fi

export NPM_CONFIG_CACHE="${NPM_CONFIG_CACHE:-${TMPDIR:-/tmp}/dotfiles-renovate-npm-cache}"
export RENOVATE_NODE_VERSION RENOVATE_VERSION
mkdir -p "$NPM_CONFIG_CACHE"
renovate_tmp="$(mktemp -d)"
trap 'rm -rf "$renovate_tmp"' EXIT HUP INT TERM
export DOTFILES_RENOVATE_JSON="$renovate_tmp/extract.jsonl"
export DOTFILES_RENOVATE_STDERR="$renovate_tmp/extract.err"

# shellcheck disable=SC2016 # expand node version inside the Node 24 subprocess.
npx --yes --package "node@$RENOVATE_NODE_VERSION" -- bash -c '
set -euo pipefail
echo "renovate validator node: $(node -v)"
npm exec --yes --package "renovate@$RENOVATE_VERSION" -- renovate-config-validator --strict renovate.json
if ! LOG_LEVEL=debug LOG_FORMAT=json \
    npm exec --yes --package "renovate@$RENOVATE_VERSION" -- \
    renovate --platform=local --dry-run=extract >"$DOTFILES_RENOVATE_JSON" 2>"$DOTFILES_RENOVATE_STDERR"; then
    cat "$DOTFILES_RENOVATE_JSON" >&2
    cat "$DOTFILES_RENOVATE_STDERR" >&2
    exit 1
fi
if [[ ! -s "$DOTFILES_RENOVATE_JSON" ]]; then
    echo "FAIL: Renovate local extract succeeded but emitted no JSON proof" >&2
    cat "$DOTFILES_RENOVATE_STDERR" >&2
    exit 1
fi
'

python3 scripts/validate-renovate-inventory.py \
    "$DOTFILES_RENOVATE_JSON" tests/static/renovate_expected_inventory.txt
