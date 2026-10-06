#!/usr/bin/env bash
# Verify the reviewed Homebrew release lines and the config actually installed.
set -euo pipefail

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
[[ $# == 1 && -r "$1" ]] || fail "Usage: check-herdr-runtime.sh READABLE_CONFIG"
command -v herdr >/dev/null 2>&1 || fail "Herdr is not on PATH"
identity="$(herdr --version)" || fail "Herdr identity probe failed"
if [[ ! "$identity" =~ ^herdr\ 0\.(7|9)\.(0|[1-9][0-9]*)$ ]]; then
    fail "Herdr identity is outside reviewed stable 0.7.x / 0.9.x: $identity"
fi
minor="${BASH_REMATCH[1]}"
patch="${BASH_REMATCH[2]}"
# String patterns avoid overflow for unusually large semver patch components.
case "$minor:$patch" in
    7:[0-4]|9:[0-2]) fail "Herdr identity predates the reviewed minimum: $identity" ;;
esac
HERDR_CONFIG_PATH="$1" herdr config check || fail "Herdr rejected the managed config"
printf 'PASS: %s accepted the managed config\n' "$identity"
