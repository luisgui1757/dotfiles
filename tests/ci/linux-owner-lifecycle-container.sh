#!/usr/bin/env bash
set -euo pipefail

[[ "$(id -u)" -eq 0 ]] || {
    echo "FAIL: container preparation must run as root" >&2
    exit 1
}
[[ -f /dotfiles.bundle ]] || {
    echo "FAIL: read-only repository bundle is missing" >&2
    exit 1
}
expected_head="${DOTFILES_LINUX_LIFECYCLE_EXPECTED_HEAD:-}"
[[ "$expected_head" =~ ^[0-9a-f]{40}$ ]] || {
    echo "FAIL: expected repository head is missing or malformed" >&2
    exit 1
}

target=/home/dotfiles/dotfiles
mkdir -p "$(dirname "$target")"
git clone --no-local /dotfiles.bundle "$target"
actual_head="$(git -C "$target" rev-parse HEAD)"
[[ "$actual_head" == "$expected_head" ]] || {
    echo "FAIL: bundled checkout head $actual_head does not match expected $expected_head" >&2
    exit 1
}
git -C "$target" diff --quiet
git -C "$target" diff --cached --quiet
chown -R dotfiles:dotfiles /home/dotfiles

exec sudo -H -u dotfiles env \
    DOTFILES_SKIP_BREW_BOOTSTRAP=1 \
    NIX_REMOTE=local \
    PATH=/nix/var/nix/profiles/default/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    bash "$target/tests/linux_owner_lifecycle.sh"
