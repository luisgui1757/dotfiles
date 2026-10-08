#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 scripts/release.py check
python3 -m unittest discover -s tests/bootstrap -p 'release_test.py'
