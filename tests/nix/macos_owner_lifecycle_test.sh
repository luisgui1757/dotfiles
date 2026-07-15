#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
DOTFILES_MACOS_OWNER_LIFECYCLE_SOURCE_ONLY=1 source "$REPO_ROOT/tests/macos_owner_lifecycle.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

FAKE_BREW_PREFIX="$WORK/prefix"
FAKE_BREW_REPOSITORY="$WORK/prefix/Library/.homebrew-is-managed-by-nix"
fake_brew="$WORK/brew"
export FAKE_BREW_PREFIX FAKE_BREW_REPOSITORY

cat > "$fake_brew" <<'BREW'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
    --prefix) printf '%s\n' "$FAKE_BREW_PREFIX" ;;
    --repository) printf '%s\n' "$FAKE_BREW_REPOSITORY" ;;
    tap) printf '%s\n' 'nikitabobko/tap' ;;
    *) exit 91 ;;
esac
BREW
chmod +x "$fake_brew"
export BREW="$fake_brew"

# The production lifecycle is Darwin-only and correctly uses BSD stat. This
# unit test runs on both CI hosts, so isolate that host-specific command while
# retaining the real path-selection and ownership assertions.
stat() {
    [[ "${1:-}" == "-f" && "${2:-}" == "%Su" && -d "${3:-}" ]] || return 92
    id -un
}

mkdir -p \
    "$FAKE_BREW_PREFIX/Library/Taps/nikitabobko/homebrew-tap" \
    "$FAKE_BREW_REPOSITORY/Library/Taps"

assert_no_scanned_tap_artifacts >/dev/null
assert_tap_ownership >/dev/null
echo "ok  : lifecycle derives installed taps from Homebrew prefix, not managed repository"

artifact="$FAKE_BREW_PREFIX/Library/Taps/nikitabobko/homebrew-tap.dotfiles-failed-20260713000000"
mkdir -p "$artifact"
if (assert_no_scanned_tap_artifacts >/dev/null 2>&1); then
    fail "lifecycle accepted an in-prefix setup recovery artifact"
fi
echo "ok  : lifecycle rejects setup recovery artifacts below prefix Library/Taps"

echo "all macOS owner lifecycle tap-boundary behaviors OK"
