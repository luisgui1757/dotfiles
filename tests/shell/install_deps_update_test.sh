#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/fakebin"

for tool in rg nvim lazygit; do
    cat > "$TMP_ROOT/fakebin/$tool" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$TMP_ROOT/fakebin/$tool"
done

PATH="$TMP_ROOT/fakebin:/usr/bin:/bin"
PM=apt
DRY_RUN=0
INSTALL_DEPS_UPDATE_TOOLS=$'rg\nfd\nnvim\nlazygit'
COMMAND_LOG="$TMP_ROOT/commands.log"

uname() {
    printf '%s\n' "Linux"
}

pm_pkg_installed() {
    local _pm="$1" pkg="$2"
    [[ "$pkg" == "ripgrep" ]]
}

maybe_sudo() {
    printf '%s\n' "$*" >> "$COMMAND_LOG"
}

output="$(update_catalog_tools)"

grep -F 'apt-get update -qq' "$COMMAND_LOG" >/dev/null \
    || fail "apt update was not issued before the scoped upgrade"
grep -F 'apt-get install -y --only-upgrade ripgrep' "$COMMAND_LOG" >/dev/null \
    || fail "present rg did not use apt-get install --only-upgrade ripgrep"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+fd[[:space:]]+not installed[[:space:]]*$' \
    || fail "absent fd was not skipped"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+nvim[[:space:]]+pinned Linux direct download' \
    || fail "pinned Linux nvim was not skipped"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+lazygit[[:space:]]+pinned Linux direct download' \
    || fail "pinned Linux lazygit was not skipped"
if grep -Eq 'apt-get install -y (fd-find|neovim|lazygit)' "$COMMAND_LOG"; then
    fail "update mode attempted an install or pinned binary package"
fi

echo "OK"
