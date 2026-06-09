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

assert_active() {
    local label="$1" path="$2" rendered_ignore="$3"
    if is_ignored "$path" "$rendered_ignore"; then
        fail "$label: expected active, got ignored ($path)"
    fi
    pass "$label: active ($path)"
}

assert_ignored() {
    local label="$1" path="$2" rendered_ignore="$3"
    if ! is_ignored "$path" "$rendered_ignore"; then
        fail "$label: expected ignored, got active ($path)"
    fi
    pass "$label: ignored ($path)"
}

assert_tmux_windows_gate() {
    local os="$1" rendered_ignore="$2"

    case "$os" in
        windows)
            assert_active "tmux.windows.conf gate ($os)" ".tmux.windows.conf" "$rendered_ignore"
            ;;
        darwin|linux)
            assert_ignored "tmux.windows.conf gate ($os)" ".tmux.windows.conf" "$rendered_ignore"
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

assert_nvim_gate() {
    local os="$1" rendered_ignore="$2"

    case "$os" in
        darwin|linux)
            assert_active "nvim POSIX gate ($os)" ".config/nvim" "$rendered_ignore"
            assert_ignored "nvim Windows gate ($os)" "AppData" "$rendered_ignore"
            ;;
        windows)
            assert_ignored "nvim POSIX gate ($os)" ".config/nvim" "$rendered_ignore"
            assert_active "nvim Windows gate ($os)" "AppData" "$rendered_ignore"
            ;;
        *)
            fail "unsupported OS fixture: $os"
            ;;
    esac
}

assert_zsh_gate() {
    local os="$1" rendered_ignore="$2"

    case "$os" in
        darwin|linux)
            assert_active "zshenv gate ($os)" ".zshenv" "$rendered_ignore"
            assert_active "zshrc gate ($os)" ".zshrc" "$rendered_ignore"
            ;;
        windows)
            assert_ignored "zshenv gate ($os)" ".zshenv" "$rendered_ignore"
            assert_ignored "zshrc gate ($os)" ".zshrc" "$rendered_ignore"
            ;;
        *)
            fail "unsupported OS fixture: $os"
            ;;
    esac
}

assert_ghostty_gate() {
    local os="$1" rendered_ignore="$2" expected_path active_count active_path

    case "$os" in
        darwin)
            expected_path="Library/Application Support/com.mitchellh.ghostty"
            ;;
        linux)
            expected_path=".config/ghostty"
            ;;
        windows)
            expected_path=""
            ;;
        *)
            fail "unsupported OS fixture: $os"
            ;;
    esac

    active_count=0
    active_path=""
    for path in \
        "Library/Application Support/com.mitchellh.ghostty" \
        ".config/ghostty"
    do
        if ! is_ignored "$path" "$rendered_ignore"; then
            active_count=$((active_count + 1))
            active_path="$path"
        fi
    done

    if [[ -z "$expected_path" ]]; then
        if [[ "$active_count" -ne 0 ]]; then
            fail "ghostty gate ($os): expected no active path, got $active_path"
        fi
        pass "ghostty gate ($os): no active path"
        return
    fi

    if [[ "$active_count" -ne 1 ]]; then
        fail "ghostty gate ($os): expected exactly one active path, got $active_count"
    fi
    if [[ "$active_path" != "$expected_path" ]]; then
        fail "ghostty gate ($os): expected $expected_path, got $active_path"
    fi
    pass "ghostty gate ($os): active path $active_path"
}

assert_powershell_gate() {
    local os="$1" rendered_ignore="$2"

    case "$os" in
        windows)
            assert_active "PowerShell profile gate ($os)" "Documents" "$rendered_ignore"
            ;;
        darwin|linux)
            assert_ignored "PowerShell profile gate ($os)" "Documents" "$rendered_ignore"
            ;;
        *)
            fail "unsupported OS fixture: $os"
            ;;
    esac
}

assert_starship_gate() {
    local os="$1" rendered_ignore="$2"
    assert_active "starship gate ($os)" ".config/starship.toml" "$rendered_ignore"
}

for os in darwin linux windows; do
    printf 'targetOS: %s\n' "$os" > "$fixture/.chezmoidata.yaml"
    rendered_ignore="$rendered/.chezmoiignore.$os"
    chezmoi --source "$fixture" execute-template < "$IGNORE_TEMPLATE" > "$rendered_ignore"

    assert_tmux_windows_gate "$os" "$rendered_ignore"
    assert_lazygit_gate "$os" "$rendered_ignore"
    assert_nvim_gate "$os" "$rendered_ignore"
    assert_zsh_gate "$os" "$rendered_ignore"
    assert_ghostty_gate "$os" "$rendered_ignore"
    assert_powershell_gate "$os" "$rendered_ignore"
    assert_starship_gate "$os" "$rendered_ignore"
done
