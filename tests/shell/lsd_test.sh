#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*"
    exit 1
}

for pm in brew apt dnf pacman zypper apk; do
    PM="$pm"
    [[ "$(pkg_for lsd)" == "lsd" ]] || fail "pkg_for lsd failed for $pm"
done

grep -q 'install lsd ' "$REPO_ROOT/install-deps.sh" \
    || fail "install-deps.sh no longer installs lsd"
grep -qF 'lsd|lsd|lsd|lsd|lsd|lsd|lsd' "$REPO_ROOT/install-deps.sh" \
    || fail "PKG_TABLE is missing the cross-platform lsd row"

zshrc="$REPO_ROOT/shells/zshrc"
grep -F 'command -v lsd' "$zshrc" >/dev/null \
    || fail "zshrc lsd aliases are not guarded by command -v"
for line in \
    "alias ls='lsd'" \
    "alias l='lsd -l'" \
    "alias la='lsd -a'" \
    "alias lla='lsd -la'" \
    "alias lt='lsd --tree'"; do
    grep -F "$line" "$zshrc" >/dev/null || fail "missing zsh alias: $line"
done

if ! diff -q "$zshrc" "$REPO_ROOT/home/dot_zshrc" >/dev/null; then
    fail "home/dot_zshrc is not byte-identical to shells/zshrc"
fi

echo "OK"
