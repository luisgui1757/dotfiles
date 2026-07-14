#!/usr/bin/env bash
# Greenfield Ubuntu must install curl before setup invokes the Nix prerequisite
# helper, while preserving --dry-run and --skip-deps semantics.
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/repo"

cat > "$WORK/repo/install-deps.sh" <<'INSTALLER'
#!/usr/bin/env bash
[[ "${INSTALL_DEPS_SOURCE_ONLY:-}" == "1" ]] || exit 90
detect_pm() { printf '%s\n' apt; }
require_downloader() {
    printf '%s:%s:%s\n' "$DRY_RUN" "$PM" "$#" > "$SETUP_NIX_DOWNLOADER_TEST_ROOT/call"
}
INSTALLER

SCRIPT_DIR="$WORK/repo"
SETUP_NIX_DOWNLOADER_TEST_ROOT="$WORK"
SKIP_DEPS=0
DRY_RUN=1
export SETUP_NIX_DOWNLOADER_TEST_ROOT
nix_downloader_available() { return 1; }

ensure_nix_downloader
[[ "$(< "$WORK/call")" == "1:apt:0" ]] ||
    fail "setup did not reuse the dependency installer's downloader bootstrap in dry-run mode"

rm -f "$WORK/call"
nix_downloader_available() { return 0; }
ensure_nix_downloader
[[ ! -e "$WORK/call" ]] || fail "setup bootstrapped curl when it was already available"

nix_downloader_available() { return 1; }
SKIP_DEPS=1
ensure_nix_downloader
[[ ! -e "$WORK/call" ]] || fail "--skip-deps still bootstrapped curl"

echo "OK"
