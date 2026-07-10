#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
PROOF="$REPO_ROOT/scripts/ci-logical-proof.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

export GITHUB_SHA=0123456789abcdef0123456789abcdef01234567
export GITHUB_RUN_ID=12345
export GITHUB_RUN_ATTEMPT=2
marker="$tmp/path with spaces/proof.env"
logical="setup.sh / linux"
legacy="setup.sh / ubuntu-24.04"

bash "$PROOF" emit "$marker" "$logical" "$legacy"
bash "$PROOF" verify "$marker" "$logical" "$legacy"

if bash "$PROOF" verify "$marker" "setup.sh / macos" "$legacy" >/dev/null 2>&1; then
    echo "FAIL: wrong logical identity was accepted"
    exit 1
fi
sed 's/^head_sha=.*/head_sha=ffffffffffffffffffffffffffffffffffffffff/' "$marker" > "$tmp/tampered.env"
if bash "$PROOF" verify "$tmp/tampered.env" "$logical" "$legacy" >/dev/null 2>&1; then
    echo "FAIL: wrong head identity was accepted"
    exit 1
fi
{ cat "$marker"; printf 'schema=1\n'; } > "$tmp/duplicate.env"
if bash "$PROOF" verify "$tmp/duplicate.env" "$logical" "$legacy" >/dev/null 2>&1; then
    echo "FAIL: duplicate proof fields were accepted"
    exit 1
fi
if bash "$PROOF" verify "$tmp/missing.env" "$logical" "$legacy" >/dev/null 2>&1; then
    echo "FAIL: missing proof marker was accepted"
    exit 1
fi

echo "OK: stable logical proof markers bind exact run, SHA, and legacy proof"
