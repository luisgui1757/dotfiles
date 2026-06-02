#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

expected="$tmp/expected.txt"
settings="$tmp/settings.txt"
safeguards="$tmp/safeguards.txt"

{
  awk '
    /^jobs:/ { in_jobs = 1; next }
    in_jobs && /^[^ ]/ { in_jobs = 0 }
    in_jobs && /^  [A-Za-z0-9_-]+:$/ {
      line = $0
      sub(/^  /, "", line)
      sub(/:$/, "", line)
      print line
    }
  ' .github/workflows/test.yml

  awk '/^[[:space:]]*- id:/ { print "e2e containers / " $3 }' \
    .github/workflows/e2e-install.yml

  awk '
    /^[[:space:]]*os:[[:space:]]*$/ { in_os = 1; next }
    in_os && /^[[:space:]]*-[[:space:]]*/ {
      line = $0
      sub(/^[[:space:]]*-[[:space:]]*/, "", line)
      print "setup.sh / " line
      next
    }
    in_os && ! /^[[:space:]]*-[[:space:]]*/ { in_os = 0 }
  ' .github/workflows/e2e-install.yml

  awk '
    /^[[:space:]]*name:[[:space:]]*setup\.ps1 \// {
      line = $0
      sub(/^[[:space:]]*name:[[:space:]]*/, "", line)
      print line
      exit
    }
  ' .github/workflows/e2e-install.yml
} > "$expected"

awk '
  /^[[:space:]]*contexts:[[:space:]]*$/ { in_contexts = 1; next }
  in_contexts && /^[[:space:]]*-[[:space:]]*/ {
    line = $0
    sub(/^[[:space:]]*-[[:space:]]*/, "", line)
    print line
    next
  }
  in_contexts && ! /^[[:space:]]*-[[:space:]]*/ { in_contexts = 0 }
' .github/settings.yml > "$settings"

awk '
  /"contexts": \[/ { in_contexts = 1; next }
  in_contexts && /^[[:space:]]*\]/ { in_contexts = 0; next }
  in_contexts {
    line = $0
    sub(/^[[:space:]]*"/, "", line)
    sub(/",?[[:space:]]*$/, "", line)
    if (line != "") print line
  }
' scripts/apply-repo-safeguards.sh > "$safeguards"

if ! diff -u "$expected" "$settings"; then
    echo "FAIL: .github/settings.yml required checks are out of sync" >&2
    exit 1
fi

if ! diff -u "$expected" "$safeguards"; then
    echo "FAIL: scripts/apply-repo-safeguards.sh required checks are out of sync" >&2
    exit 1
fi

echo "OK"
