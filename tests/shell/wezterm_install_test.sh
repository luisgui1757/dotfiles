#!/usr/bin/env bash
# Positive coverage for the pinned WezTerm .deb path. The installer must verify
# the pinned SHA before invoking apt, and native Linux must not hide the Ubuntu
# .deb branch behind a runtime GUI-display probe.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/wezterm-install-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export TMPDIR="$TMP_ROOT/tmp"

curl() {
    local out=""
    while [[ "$#" -gt 0 ]]; do
        case "$1" in
            -o)
                out="$2"
                shift 2
                ;;
            *)
                shift
                ;;
        esac
    done
    printf 'fake-wezterm-deb\n' > "$out"
}

verify_sha256() {
    printf '%s\n' "$2" > "$TMP_ROOT/sha.log"
    [[ "$2" == "$WEZTERM_DEB_AMD64_SHA256" ]]
}

maybe_sudo() {
    printf '%s\n' "$*" >> "$TMP_ROOT/apt.log"
    [[ "${1:-}" == "env" && "${2:-}" == "DEBIAN_FRONTEND=noninteractive" \
        && "${3:-}" == "apt-get" && "${4:-}" == "install" \
        && "${5:-}" == "-y" && -f "${6:-}" ]]
}

run_wezterm_deb_install "https://example.invalid/wezterm.deb" "$WEZTERM_DEB_AMD64_SHA256"
grep -F "$WEZTERM_DEB_AMD64_SHA256" "$TMP_ROOT/sha.log" >/dev/null
grep -E '^env DEBIAN_FRONTEND=noninteractive apt-get install -y .*/wezterm\.deb$' "$TMP_ROOT/apt.log" >/dev/null

YES_ALL=1
DRY_RUN=1
PM=apt
have() { return 1; }
is_wsl() { return 1; }
is_ubuntu() { return 0; }
can_show_gui() { return 1; }
uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

output="$(install_wezterm_linux 2>&1)"
[[ "$output" == *"wezterm-${WEZTERM_VERSION}.Ubuntu22.04.deb"* ]]
[[ "$output" == *"verify sha256 $WEZTERM_DEB_AMD64_SHA256"* ]]
[[ "$output" != *"no GUI display"* ]]
[[ "$output" != *"no verified pinned"* ]]

echo "OK"
