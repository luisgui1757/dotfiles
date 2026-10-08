#!/usr/bin/env bash
# All supported hosts use upstream LazyGit keybindings from one empty mapping.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
python3 - "$REPO_ROOT" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
config = root / 'lazygit/config.yml'
content = '\n'.join(line for line in config.read_text().splitlines() if not line.lstrip().startswith('#'))
assert json.loads(content) == {}, 'LazyGit must retain upstream defaults on every OS'
# A passive released Windows source can remain for existing live links.
# The active target manifest, below, must still select upstream defaults.
targets = json.loads((root / 'installer/config-targets.json').read_text())['targets']
lazygit = [target for target in targets if target['resource'] == 'config.lazygit']
assert {p for target in lazygit for p in target['platforms']} == {'darwin', 'linux', 'windows'}
assert all(target['source'] == 'lazygit/config.yml' for target in lazygit)
windows = next(target for target in lazygit if target['platforms'] == ['windows'])
assert windows['folder'] == 'local_app_data', 'LazyGit uses LocalAppData, not roaming AppData'
print('OK: shared upstream LazyGit defaults, with native config destinations')
PY
