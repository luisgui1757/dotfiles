#!/usr/bin/env bash
set -euo pipefail

[[ "$(id -u)" -eq 0 ]] || {
    echo "FAIL: container preparation must run as root" >&2
    exit 1
}
[[ -d /repo/.git ]] || {
    echo "FAIL: read-only repository mount is missing" >&2
    exit 1
}

target=/home/dotfiles/dotfiles
mkdir -p "$(dirname "$target")"
cp -a /repo "$target"
chown -R dotfiles:dotfiles /home/dotfiles

exec sudo -H -u dotfiles env \
    DOTFILES_SKIP_BREW_BOOTSTRAP=1 \
    NIX_REMOTE=local \
    PATH=/nix/var/nix/profiles/default/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    bash "$target/tests/linux_owner_lifecycle.sh"
