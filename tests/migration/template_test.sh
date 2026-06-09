#!/usr/bin/env bash
# Hermetic, host-independent proof of the per-OS .chezmoiignore gating: render the
# ignore template against an injected targetOS and assert it equals EXACTLY the
# expected ignore set for that OS. (The parity gate proves the applied per-config
# result; this proves the gating logic on any host with no apply/network.)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
IGNORE_TEMPLATE="$REPO_ROOT/home/.chezmoiignore"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

pass() {
    echo "PASS: $*"
}

if ! command -v chezmoi >/dev/null 2>&1; then
    fail "chezmoi is required on PATH"
fi

if [[ ! -f "$IGNORE_TEMPLATE" ]]; then
    fail "missing ignore template: $IGNORE_TEMPLATE"
fi

# Expected rendered ignore SET per OS. Dir-level gating: a wrong-OS bucket
# (Library on non-darwin, AppData/Documents on non-windows, .config/{lazygit,
# ghostty} off-linux, .zshenv/.zshrc/.config/nvim on windows) is ignored whole so
# chezmoi never creates an empty wrong-OS parent dir. Bash 3.2-safe (no assoc
# arrays / mapfile).
expected_for() {
    case "$1" in
        darwin)  printf '%s\n' ".config/ghostty" ".config/lazygit" ".tmux.windows.conf" "AppData" "Documents" ;;
        linux)   printf '%s\n' ".tmux.windows.conf" "AppData" "Documents" "Library" ;;
        windows) printf '%s\n' ".config/ghostty" ".config/lazygit" ".config/nvim" ".zshenv" ".zshrc" "Library" ;;
        *)       fail "unsupported OS fixture: $1" ;;
    esac
}

fixture="$(mktemp -d)"
rendered="$(mktemp -d)"
trap 'rm -rf "$fixture" "$rendered"' EXIT

for os in darwin linux windows; do
    printf 'targetOS: %s\n' "$os" > "$fixture/.chezmoidata.yaml"
    rendered_ignore="$rendered/.chezmoiignore.$os"
    chezmoi --source "$fixture" execute-template < "$IGNORE_TEMPLATE" > "$rendered_ignore"

    actual="$(grep -vE '^[[:space:]]*$' "$rendered_ignore" | sort -u)"
    expected="$(expected_for "$os" | sort -u)"

    if [[ "$actual" != "$expected" ]]; then
        {
            echo "  expected:"; printf '%s\n' "$expected" | sed 's/^/    /'
            echo "  actual:";   printf '%s\n' "$actual"   | sed 's/^/    /'
        } >&2
        fail "$os: rendered .chezmoiignore does not match the expected per-OS ignore set"
    fi
    pass "$os: .chezmoiignore renders exactly the expected per-OS ignore set"
done
