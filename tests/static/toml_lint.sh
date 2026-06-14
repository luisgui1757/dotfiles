#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

files=()
while IFS= read -r f; do
    files+=("$f")
done < <(find "$REPO_ROOT" -type f -name "*.toml" -not -path "*/.git/*" -not -path "*/tests/.cache/*" -not -path "$REPO_ROOT/home/*")
[[ "${#files[@]}" -eq 0 ]] && { echo "no toml files"; exit 0; }

if command -v taplo >/dev/null 2>&1; then
    taplo_failed=0
    taplo_panicked=0
    for f in "${files[@]}"; do
        taplo_output=""
        if ! taplo_output="$(taplo lint "$f" 2>&1)"; then
            if grep -Eq 'panicked at|Attempted to create a NULL object' <<<"$taplo_output"; then
                echo "taplo panicked on $f; falling back to python tomllib"
                printf '%s\n' "$taplo_output"
                taplo_panicked=1
                break
            fi
            echo "FAIL: $f"
            printf '%s\n' "$taplo_output"
            taplo_failed=1
        fi
    done
    [[ "$taplo_failed" -eq 0 ]] || exit 1
    if [[ "$taplo_panicked" -eq 0 ]]; then
        echo "OK (taplo)"
        exit 0
    fi
fi
if command -v python3 >/dev/null 2>&1; then
    for f in "${files[@]}"; do
        python3 - "$f" <<'PY'
import sys, tomllib
with open(sys.argv[1], 'rb') as fh:
    tomllib.load(fh)
PY
    done
    echo "OK (python tomllib)"
    exit 0
fi
echo "skipped: no toml parser available"
