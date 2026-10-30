#!/usr/bin/env bash
# Lint the starship toml — parse-check only. Uses taplo when healthy, with a
# tomllib fallback for the known macOS taplo dynamic-store panic.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# config-schema.json is .github/config-schema.json of Starship v1.26.0 (ISC, see
# LICENSES/starship-ISC.txt). Taplo uses it instead of the $schema URL, so the
# lint does not use the network. Replace it when the Starship pin changes.
SCHEMA_URL="file://$REPO_ROOT/tests/starship/config-schema.json"

if command -v taplo >/dev/null 2>&1; then
    taplo_output=""
    if ! taplo_output="$(taplo lint --schema "$SCHEMA_URL" "$REPO_ROOT/starship/starship.toml" 2>&1)"; then
        if grep -Eq 'panicked at|Attempted to create a NULL object' <<<"$taplo_output"; then
            echo "taplo panicked on $REPO_ROOT/starship/starship.toml; falling back to python tomllib"
            printf '%s\n' "$taplo_output"
        else
            printf '%s\n' "$taplo_output"
            exit 1
        fi
    else
        printf '%s' "$taplo_output"
        [[ -z "$taplo_output" || "$taplo_output" == *$'\n' ]] || printf '\n'
        echo "OK (taplo)"
        exit 0
    fi
fi
if command -v python3 >/dev/null 2>&1; then
    python3 - "$REPO_ROOT/starship/starship.toml" <<'PY'
import sys, tomllib
with open(sys.argv[1], "rb") as f:
    tomllib.load(f)
PY
    echo "OK (python tomllib)"
    exit 0
fi
echo "skipped: no toml parser available"
