#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 - <<'PY'
import pathlib
fixture=pathlib.Path('installer/archive_desktop_native_test.go').read_text()
for required in ('codesign', '"--verify", "--deep", "--strict"', '"--version"', '"AeroSpace.app"', '"Ghostty.app"'):
    assert required in fixture, required
assert '**AeroSpace managed-config consumption**' in pathlib.Path('tests/MANUAL.md').read_text()
print('OK: native Mac archive fixture checks real app identity/signature; TCC config proof stays manual')
PY
