#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TARGET_HOME=""
USE_CURRENT_HOME=0

usage() {
    cat <<'EOF'
macos-greenfield.sh -- run setup.sh and validate.sh for a macOS greenfield check.

Usage:
  tests/greenfield/macos-greenfield.sh --current-home
  tests/greenfield/macos-greenfield.sh --home <dir>

Modes:
  --current-home   run against the current account HOME. Use this after logging
                   into a fresh macOS user account or a macOS VM.
  --home <dir>     run with an explicit HOME sandbox. This is useful for config
                   debugging, but it is not a clean OS because package installs
                   still affect the current macOS host.
EOF
}

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --current-home)
            USE_CURRENT_HOME=1
            shift
            ;;
        --home)
            [[ "$#" -ge 2 ]] || { echo "FAIL: --home needs a path" >&2; exit 2; }
            TARGET_HOME="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "FAIL: unknown arg: $1" >&2
            exit 2
            ;;
    esac
done

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

run_and_capture() {
    local label="$1" log="$2" rc
    shift 2
    set +e
    "$@" 2>&1 | tee "$log"
    rc=${PIPESTATUS[0]}
    set -e
    if [[ "$rc" -ne 0 ]]; then
        fail "$label exited $rc; log: $log"
    fi
    if grep -Eq '^[[:space:]]*FAIL:' "$log"; then
        fail "$label emitted a FAIL marker; log: $log"
    fi
}

if [[ "$(uname -s)" != "Darwin" ]]; then
    fail "macos-greenfield.sh must run on macOS"
fi

if [[ -n "$TARGET_HOME" && "$USE_CURRENT_HOME" -eq 1 ]]; then
    fail "choose either --current-home or --home, not both"
fi

if [[ -z "$TARGET_HOME" && "$USE_CURRENT_HOME" -eq 0 ]]; then
    usage
    cat <<'EOF'

Default greenfield flow:
  1. Create a fresh macOS user account, or boot a clean macOS VM.
  2. Log into that account.
  3. Clone this repo.
  4. Run: bash tests/greenfield/macos-greenfield.sh --current-home

For config-layer debugging without a clean OS, pass --home <dir>.
EOF
    exit 2
fi

if [[ "$USE_CURRENT_HOME" -eq 1 ]]; then
    TARGET_HOME="$HOME"
else
    mkdir -p "$TARGET_HOME"
    TARGET_HOME="$(cd "$TARGET_HOME" && pwd -P)"
fi

LOG_DIR="${TMPDIR:-/tmp}/dotfiles-greenfield-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$LOG_DIR"
SETUP_LOG="$LOG_DIR/setup-sh.log"

echo "macos-greenfield: repo=$REPO_ROOT"
echo "macos-greenfield: home=$TARGET_HOME"
echo "macos-greenfield: log=$SETUP_LOG"

HOME="$TARGET_HOME" run_and_capture "setup.sh" "$SETUP_LOG" "$REPO_ROOT/setup.sh" --all

if grep -Fq "skipped: Phase 3-5" "$SETUP_LOG"; then
    fail "setup.sh skipped Phase 3-5; log: $SETUP_LOG"
fi
grep -F "Phase 3/6" "$SETUP_LOG" >/dev/null || fail "setup.sh did not run Phase 3/6; log: $SETUP_LOG"
grep -F "Phase 4/6" "$SETUP_LOG" >/dev/null || fail "setup.sh did not run Phase 4/6; log: $SETUP_LOG"
grep -F "Phase 5/6" "$SETUP_LOG" >/dev/null || fail "setup.sh did not run Phase 5/6; log: $SETUP_LOG"
grep -F "Phase 6/6" "$SETUP_LOG" >/dev/null || fail "setup.sh did not run Phase 6/6; log: $SETUP_LOG"

HOME="$TARGET_HOME" "$REPO_ROOT/tests/greenfield/validate.sh" --repo "$REPO_ROOT"

echo "PASS: macOS greenfield setup and validation completed"
echo "Logs: $LOG_DIR"
