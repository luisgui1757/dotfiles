#!/usr/bin/env bash
# PATH probes must keep using the zsh selected before restricting PATH.
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
zsh_bin="$(command -v zsh)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/alternate bin"
: > "$tmp/invocations"
cat > "$tmp/alternate bin/zsh" <<'EOF'
#!/bin/sh
set -eu
printf '%s\n' "$1" >> "$DOTFILES_TEST_ZSH_LOG"
exec "$DOTFILES_TEST_ZSH_REAL" "$@"
EOF
chmod +x "$tmp/alternate bin/zsh"

# Delegate to the real runtime and run every original generated-profile assertion.
if ! output=$(PATH="$tmp/alternate bin:$PATH" \
    DOTFILES_TEST_ZSH_REAL="$zsh_bin" DOTFILES_TEST_ZSH_LOG="$tmp/invocations" \
    bash "$REPO_ROOT/tests/shell/path_dedupe_test.sh" 2>&1); then
    printf 'FAIL: PATH probes failed with zsh outside the restricted PATH:\n%s\n' "$output"
    exit 1
fi

expected=$(printf '%s\n' -d -c)
if [[ "$(uname -s)" == Darwin ]]; then
    expected=$(printf '%s\n' -d -c -lc -lc)
fi
actual=$(cat "$tmp/invocations")
if [[ "$actual" != "$expected" ]]; then
    printf 'FAIL: PATH probes bypassed the selected zsh; invocation modes:\n%s\n' "$actual"
    exit 1
fi
echo "OK: every PATH probe uses the discovered zsh, including paths with spaces"
