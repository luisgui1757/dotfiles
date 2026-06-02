#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/install-nerd-font-extractor-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export TMPDIR="$TMP_ROOT/tmp"
export XDG_DATA_HOME="$TMP_ROOT/data"
EXTRACTOR_READY=0
LOG="$TMP_ROOT/log"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

fc-list() { return 0; }

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
    printf 'curl\n' >> "$LOG"
    printf 'fake font archive\n' > "$out"
}

unzip() {
    printf 'unzip %s\n' "$*" >> "$LOG"
}

verify_sha256() { return 0; }

have() {
    case "$1" in
        curl) return 0 ;;
        unzip) [[ "$EXTRACTOR_READY" -eq 1 ]] ;;
        bsdtar) return 1 ;;
        fc-cache) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

have_any() {
    if [[ "$*" == "unzip bsdtar" ]]; then
        [[ "$EXTRACTOR_READY" -eq 1 ]]
        return
    fi
    local b
    for b in "$@"; do
        have "$b" && return 0
    done
    return 1
}

install() {
    [[ "$1" == "unzip" ]] || return 1
    printf 'install %s\n' "$*" >> "$LOG"
    EXTRACTOR_READY=1
}

YES_ALL=1
DRY_RUN=0
PM=apt

install_nerd_font >/dev/null

grep -Fx 'install unzip extract Hack Nerd Font archive' "$LOG" >/dev/null
awk 'BEGIN { install_seen=0 } /^install unzip/ { install_seen=1 } /^curl$/ && !install_seen { exit 1 } END { exit !install_seen }' "$LOG"
grep -F 'unzip -oq' "$LOG" >/dev/null

echo "OK"
