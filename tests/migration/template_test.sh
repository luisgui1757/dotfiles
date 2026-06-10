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
        darwin)    printf '%s\n' ".config/ghostty" ".config/lazygit" ".tmux.windows.conf" "AppData" "Documents" ;;
        linux)     printf '%s\n' ".tmux.windows.conf" "AppData" "Documents" "Library" ;;
        linux-wsl) printf '%s\n' ".config/ghostty" ".tmux.windows.conf" "AppData" "Documents" "Library" ;;
        windows)   printf '%s\n' ".config/ghostty" ".config/lazygit" ".config/nvim" ".zshenv" ".zshrc" "Library" ;;
        *)         fail "unsupported OS fixture: $1" ;;
    esac
}

fixture="$(mktemp -d)"
rendered="$(mktemp -d)"
trap 'rm -rf "$fixture" "$rendered"' EXIT

# linux-wsl injects targetOS=linux + isWsl=true to prove ghostty is gated on WSL
# (Windows-host terminal) while lazygit stays managed -- matching legacy bootstrap.
for case_name in darwin linux linux-wsl windows; do
    case "$case_name" in
        linux-wsl) os=linux; iswsl=true ;;
        *)         os="$case_name"; iswsl=false ;;
    esac
    printf 'targetOS: %s\nisWsl: %s\n' "$os" "$iswsl" > "$fixture/.chezmoidata.yaml"
    rendered_ignore="$rendered/.chezmoiignore.$case_name"
    chezmoi --source "$fixture" execute-template < "$IGNORE_TEMPLATE" > "$rendered_ignore"

    actual="$(grep -vE '^[[:space:]]*$' "$rendered_ignore" | sort -u)"
    expected="$(expected_for "$case_name" | sort -u)"

    if [[ "$actual" != "$expected" ]]; then
        {
            echo "  expected:"; printf '%s\n' "$expected" | sed 's/^/    /'
            echo "  actual:";   printf '%s\n' "$actual"   | sed 's/^/    /'
        } >&2
        fail "$case_name: rendered .chezmoiignore does not match the expected per-OS ignore set"
    fi
    pass "$case_name: .chezmoiignore renders exactly the expected per-OS ignore set"
done
