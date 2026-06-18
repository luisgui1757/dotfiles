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

for tool in rg nvim lazygit starship tree-sitter; do
    cat > "$TMP_ROOT/fakebin/$tool" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$TMP_ROOT/fakebin/$tool"
done

PATH="$TMP_ROOT/fakebin:/usr/bin:/bin"
PM=apt
DRY_RUN=0
INSTALL_DEPS_UPDATE_TOOLS=$'rg\nfd\nnvim\nlazygit\nstarship\ntree-sitter'
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
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+starship[[:space:]]+pinned Linux direct download' \
    || fail "pinned Linux starship was not skipped"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+tree-sitter[[:space:]]+pinned Linux direct download' \
    || fail "pinned Linux tree-sitter was not skipped"
if grep -Eq 'apt-get install -y (fd-find|neovim|lazygit|starship|tree-sitter)' "$COMMAND_LOG"; then
    fail "update mode attempted an install or pinned binary package"
fi

PM=apk
: > "$COMMAND_LOG"
native_linux_pm() {
    printf '%s\n' "apk"
}
pm_pkg_installed() {
    local _pm="$1" pkg="$2"
    case "$pkg" in
        neovim|lazygit|starship|tree-sitter) return 0 ;;
        *) return 1 ;;
    esac
}
output="$(update_catalog_tools)"

printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+nvim[[:space:]]+pinned Linux direct download' \
    && fail "apk-managed nvim was skipped as a pinned Linux direct download"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+tree-sitter[[:space:]]+pinned Linux direct download' \
    && fail "apk-managed tree-sitter was skipped as a pinned Linux direct download"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+starship[[:space:]]+pinned Linux direct download' \
    && fail "apk-managed starship was skipped as a pinned Linux direct download"
grep -F 'apk upgrade neovim' "$COMMAND_LOG" >/dev/null \
    || fail "apk-managed nvim did not use apk upgrade neovim"
grep -F 'apk upgrade tree-sitter' "$COMMAND_LOG" >/dev/null \
    || fail "apk-managed tree-sitter did not use apk upgrade tree-sitter"
grep -F 'apk upgrade lazygit' "$COMMAND_LOG" >/dev/null \
    || fail "apk-managed lazygit did not use apk upgrade lazygit"
grep -F 'apk upgrade starship' "$COMMAND_LOG" >/dev/null \
    || fail "apk-managed starship did not use apk upgrade starship"

PM=brew
: > "$COMMAND_LOG"
pm_pkg_installed() {
    local _pm="$1" pkg="$2"
    case "$pkg" in
        neovim|lazygit|starship|tree-sitter-cli) return 0 ;;
        *) return 1 ;;
    esac
}
brew() {
    printf '%s\n' "brew $*" >> "$COMMAND_LOG"
}
output="$(update_catalog_tools)"

printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+nvim[[:space:]]+pinned Linux direct download' \
    && fail "Linuxbrew-managed nvim was skipped as a pinned Linux direct download"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+tree-sitter[[:space:]]+pinned Linux direct download' \
    && fail "Linuxbrew-managed tree-sitter was skipped as a pinned Linux direct download"
printf '%s\n' "$output" | grep -Eq '^  skipped[[:space:]]+starship[[:space:]]+pinned Linux direct download' \
    && fail "Linuxbrew-managed starship was skipped as a pinned Linux direct download"
grep -F 'brew upgrade neovim' "$COMMAND_LOG" >/dev/null \
    || fail "Linuxbrew-managed nvim did not use brew upgrade neovim"
grep -F 'brew upgrade tree-sitter-cli' "$COMMAND_LOG" >/dev/null \
    || fail "Linuxbrew-managed tree-sitter did not use brew upgrade tree-sitter-cli"
grep -F 'brew upgrade starship' "$COMMAND_LOG" >/dev/null \
    || fail "Linuxbrew-managed starship did not use brew upgrade starship"

echo "OK"
