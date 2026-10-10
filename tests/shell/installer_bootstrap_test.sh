#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
python3 -m unittest tests.bootstrap.launcher_test tests.bootstrap.legacy_inventory_test
