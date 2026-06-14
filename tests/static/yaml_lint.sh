#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
export REPO_ROOT

files=$(find "$REPO_ROOT" -type f \( -name "*.yml" -o -name "*.yaml" \) -not -path "*/.git/*" -not -path "*/tests/.cache/*" -not -path "$REPO_ROOT/home/*")
[[ -z "$files" ]] && { echo "no yaml files"; exit 0; }

if command -v yamllint >/dev/null 2>&1; then
    # shellcheck disable=SC2086  # $files is intentionally word-split
    yamllint -d "{extends: relaxed, rules: {line-length: disable, document-start: disable}}" $files
    echo "OK (yamllint)"
    exit 0
fi

if command -v python3 >/dev/null 2>&1 && python3 -c "import yaml" 2>/dev/null; then
    python3 - <<'PY'
import sys, yaml, glob, os
root = os.environ["REPO_ROOT"]
fail = 0
for ext in ("yml","yaml"):
    for f in glob.glob(f"{root}/**/*.{ext}", recursive=True):
        if "/.git/" in f: continue
        if f.startswith(f"{root}/home/"): continue
        try:
            with open(f) as fh: list(yaml.safe_load_all(fh))
        except Exception as e:
            print("FAIL:", f, e); fail = 1
sys.exit(fail)
PY
    echo "OK (python yaml)"
    exit 0
fi
echo "skipped: yamllint and pyyaml unavailable (pip install pyyaml or brew install yamllint)"
