#!/usr/bin/env bash
# Herdr must consume the same forced-dark Rose Pine theme on POSIX and through
# Windows' independently redirected roaming ApplicationData known folder.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CONFIG="$REPO_ROOT/herdr/config.toml"
MIRROR="$REPO_ROOT/home/.chezmoitemplates/herdr/config.toml"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

python3 - "$CONFIG" <<'PY'
import pathlib
import sys
import tomllib

path = pathlib.Path(sys.argv[1])
with path.open("rb") as handle:
    config = tomllib.load(handle)
if config.get("onboarding") is not False:
    raise SystemExit("Herdr onboarding must be disabled for the managed config")
theme = config.get("theme", {})
if theme.get("name") != "rose-pine":
    raise SystemExit("Herdr theme.name must be rose-pine")
if theme.get("auto_switch") is not False:
    raise SystemExit("Herdr theme.auto_switch must be false")
PY

cmp -s "$CONFIG" "$MIRROR" || fail "canonical Herdr config and chezmoi mirror differ"
grep -F '.chezmoitemplates/herdr/config.toml' \
    "$REPO_ROOT/home/dot_config/herdr/symlink_config.toml.tmpl" >/dev/null ||
    fail "POSIX Herdr target does not reference the managed mirror"
grep -F '\herdr\config.toml' \
    "$REPO_ROOT/windows/chezmoi-appdata/herdr/symlink_config.toml.tmpl" >/dev/null ||
    fail "Windows ApplicationData overlay does not reference the canonical Herdr config"

for script in setup.ps1 uninstall.ps1; do
    grep -F "'ApplicationData'" "$REPO_ROOT/$script" >/dev/null ||
        fail "$script does not resolve roaming ApplicationData"
    grep -F 'windows\chezmoi-appdata' "$REPO_ROOT/$script" >/dev/null ||
        fail "$script does not own the Herdr ApplicationData overlay"
    grep -F 'appdata.boltdb' "$REPO_ROOT/$script" >/dev/null ||
        fail "$script does not use the Herdr overlay state boundary"
done

echo "all Herdr Rose Pine theme invariants OK"
