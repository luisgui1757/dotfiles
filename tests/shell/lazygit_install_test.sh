#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

have() {
    case "$1" in
        lazygit) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

YES_ALL=1
DRY_RUN=1
PM=apt

out="$(install_lazygit)"
version_no_v="${LAZYGIT_LINUX_VERSION#v}"
asset="lazygit_${version_no_v}_linux_x86_64.tar.gz"

[[ "$out" == *"github.com/jesseduffield/lazygit/releases/download/${LAZYGIT_LINUX_VERSION}/${asset}"* ]]
[[ "$out" == *"verify sha256 $LAZYGIT_LINUX_X86_64_SHA256"* ]]
[[ "$out" == *"/usr/local/bin/lazygit"* ]]
[[ "$out" == *"\$HOME/.local/bin/lazygit"* ]]

if grep -Eq '^lazygit\|lazygit\|[^|]' "$REPO_ROOT/install-deps.sh"; then
    echo "FAIL: lazygit must not use apt/dnf/pacman package rows on native Linux"
    exit 1
fi

PM=brew
out="$(install_lazygit)"
[[ "$out" == *"would: brew install lazygit"* ]]
[[ "$out" != *"github.com/jesseduffield/lazygit/releases"* ]]

echo "OK"
