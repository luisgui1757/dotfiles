#!/usr/bin/env bash
# setup.sh must resolve one authoritative invoking account/home and use it for
# every subsequent POSIX target. It may not infer a home from a username.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/setup-target-identity-test"
rm -rf "$WORK"
mkdir -p "$WORK/Real Home" "$WORK/Other Home"
ln -s "$WORK/Real Home" "$WORK/Home Link"
trap 'rm -rf "$WORK"' EXIT

DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" >/dev/null

fail() { echo "FAIL: $1"; exit 1; }

valid_case() (
    id() {
        case "${1:-}" in
            -u) echo 501 ;;
            -un) echo 'domain.user' ;;
            *) return 1 ;;
        esac
    }
    account_home_directory() { printf '%s\n' "$WORK/Real Home"; }
    HOME="$WORK/Home Link"
    resolve_target_identity
    [[ "$DOTFILES_TARGET_USER" == "domain.user" ]]
    [[ "$DOTFILES_TARGET_HOME" == "$WORK/Real Home" ]]
    [[ "$HOME" == "$WORK/Real Home" ]]
    [[ "$DEFAULT_DEST" == "$WORK/Real Home/dotfiles" ]]
    [[ "$POLARIS_CACHE_ROOT" == "$WORK/Real Home/.local/share/dotfiles/polaris" ]]
)
valid_case || fail "validated account/home with spaces and a canonical HOME link was rejected"

if (
    id() { [[ "${1:-}" == -u ]] && echo 0 || echo root; }
    account_home_directory() { echo /root; }
    HOME=/root
    resolve_target_identity >/dev/null 2>&1
); then
    fail "root invocation was accepted"
fi

if (
    id() { [[ "${1:-}" == -u ]] && echo 501 || echo tester; }
    account_home_directory() { printf '%s\n' "$WORK/Real Home"; }
    HOME="$WORK/Other Home"
    resolve_target_identity >/dev/null 2>&1
); then
    fail "mismatched HOME and account record were accepted"
fi

if (
    id() { [[ "${1:-}" == -u ]] && echo 501 || echo tester; }
    account_home_directory() { return 1; }
    HOME="$WORK/Real Home"
    resolve_target_identity >/dev/null 2>&1
); then
    fail "missing authoritative account record was accepted"
fi

echo "OK"
