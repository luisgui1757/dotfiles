#!/usr/bin/env bash
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

fixture="$(mktemp -d)"
rendered="$(mktemp -d)"
trap 'rm -rf "$fixture" "$rendered"' EXIT

is_ignored() {
    local path="$1" rendered_ignore="$2"
    grep -Fxq "$path" "$rendered_ignore"
}

assert_tmux_windows_gate() {
    local os="$1" rendered_ignore="$2"

    case "$os" in
        windows)
            if is_ignored ".tmux.windows.conf" "$rendered_ignore"; then
                fail "tmux.windows.conf gate ($os): expected active, got ignored"
            fi
            pass "tmux.windows.conf gate ($os): active"
            ;;
        darwin|linux)
            if ! is_ignored ".tmux.windows.conf" "$rendered_ignore"; then
                fail "tmux.windows.conf gate ($os): expected ignored, got active"
            fi
            pass "tmux.windows.conf gate ($os): ignored"
            ;;
        *)
            fail "unsupported OS fixture: $os"
            ;;
    esac
}

assert_lazygit_gate() {
    local os="$1" rendered_ignore="$2" expected_path active_count active_path

    case "$os" in
        darwin)
            # dir-level marker: macOS lazygit lives under Library/Application Support
            expected_path="Library/Application Support/lazygit"
            ;;
        linux)
            expected_path=".config/lazygit"
            ;;
        windows)
            # Windows lazygit + WT both live under AppData (one ignore marker)
            expected_path="AppData"
            ;;
        *)
            fail "unsupported OS fixture: $os"
            ;;
    esac

    active_count=0
    active_path=""
    for path in \
        "Library/Application Support/lazygit" \
        ".config/lazygit" \
        "AppData"
    do
        if ! is_ignored "$path" "$rendered_ignore"; then
            active_count=$((active_count + 1))
            active_path="$path"
        fi
    done

    if [[ "$active_count" -ne 1 ]]; then
        fail "lazygit gate ($os): expected exactly one active path, got $active_count"
    fi
    if [[ "$active_path" != "$expected_path" ]]; then
        fail "lazygit gate ($os): expected $expected_path, got $active_path"
    fi
    pass "lazygit gate ($os): active path $active_path"
}

for os in darwin linux windows; do
    printf 'targetOS: %s\n' "$os" > "$fixture/.chezmoidata.yaml"
    rendered_ignore="$rendered/.chezmoiignore.$os"
    chezmoi --source "$fixture" execute-template < "$IGNORE_TEMPLATE" > "$rendered_ignore"

    assert_tmux_windows_gate "$os" "$rendered_ignore"
    assert_lazygit_gate "$os" "$rendered_ignore"
done
