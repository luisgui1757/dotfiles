#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 scripts/release.py check
python3 - <<'PY'
import json,pathlib
root=pathlib.Path('.')
for entry in ('setup.sh','setup.ps1','migrate.sh','migrate.ps1'):
    assert 'installer-bootstrap' in (root/entry).read_text(), entry
assert json.loads((root/'installer/legacy-config-targets.json').read_text())
assert json.loads((root/'installer/legacy-profile-evidence.json').read_text())
print('OK: release evidence and explicit Go migration entrypoints remain available')
PY
