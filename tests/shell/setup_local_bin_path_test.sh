#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
fail() { echo "FAIL: $1" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
export HOME="$WORK/home"
mkdir -p "$HOME/.local/bin"

# Load setup.sh's helper functions without running the install/config/sync
# phases. Unit-testing refresh_runtime_path directly avoids the nvim-precedence
# fragility of a full setup.sh run (a real nvim in /usr/local/bin on a CI runner
# would shadow a stub in ~/.local/bin, since the refresh APPENDS ~/.local/bin).
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"

# F7: a normal refresh must put ~/.local/bin on PATH, so install-deps' fd-find
# symlink (~/.local/bin/fd on apt) resolves for Phase 3-4 and fresh shells.
DRY_RUN=0
PATH="/usr/bin:/bin"
refresh_runtime_path
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) fail "refresh_runtime_path did not add ~/.local/bin to PATH" ;;
esac

# F9: under --dry-run the refresh must be a no-op (the dry-run contract promises
# nothing is changed -- no PATH mutation, no brew shellenv eval, no hash -r).
DRY_RUN=1
PATH="/usr/bin:/bin"
before="$PATH"
refresh_runtime_path
[[ "$PATH" == "$before" ]] || fail "refresh_runtime_path mutated PATH during --dry-run"

echo "OK"
