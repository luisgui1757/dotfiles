#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 scripts/release.py check
python3 - <<'PY'
import pathlib
root=pathlib.Path('.')
for entry in ('setup.sh','setup.ps1'):
    assert 'installer-bootstrap' in (root/entry).read_text(), entry
print('OK: release evidence and the Go setup entrypoints remain available')
PY
