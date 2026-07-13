#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/fakebin" "$TMP_ROOT/repo"

cat > "$TMP_ROOT/repo/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$SETUP_UPDATE_TEST_ROOT/install-deps.args"
EOF
chmod +x "$TMP_ROOT/repo/install-deps.sh"

cat > "$TMP_ROOT/fakebin/nvim" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$SETUP_UPDATE_TEST_ROOT/nvim.args"
EOF
chmod +x "$TMP_ROOT/fakebin/nvim"

SCRIPT_DIR="$TMP_ROOT/repo"
DEPS_FLAGS=()
DRY_RUN=0
SKIP_DEPS=0
SKIP_NVIM=0
BEST_EFFORT=0
HOME="$TMP_ROOT/home"
PATH="$TMP_ROOT/fakebin:/usr/bin:/bin"
SETUP_UPDATE_TEST_ROOT="$TMP_ROOT"
export HOME PATH SETUP_UPDATE_TEST_ROOT

refresh_runtime_path() {
    :
}

output="$(run_update_mode 2>&1)"

grep -Fx -- '--update' "$TMP_ROOT/install-deps.args" >/dev/null \
    || fail "setup update did not invoke install-deps.sh --update"
grep -Fx -- "--headless +lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')" "$TMP_ROOT/nvim.args" >/dev/null \
    || fail "setup update did not invoke the checked MasonToolsUpdateSync wrapper"
if printf '%s\n' "$output" | grep -Eq 'chezmoi|Lazy(!| sync)|Tree-sitter|MasonToolsInstallSync|Phase 2|Phase 3'; then
    echo "$output"
    fail "setup update touched config, Lazy, Tree-sitter, or Mason install paths"
fi
printf '%s\n' "$output" | grep -F "checked-out release, pinned plugins, configs, Nix layer, and missing tools were reconciled" >/dev/null \
    || fail "closing note for the preceding full reconciliation was missing"

rm -f "$TMP_ROOT/install-deps.args" "$TMP_ROOT/nvim.args"
SKIP_NATIVE_DEPS=1
SKIP_NVIM=1
output="$(run_update_mode 2>&1)"
[[ ! -e "$TMP_ROOT/install-deps.args" ]] ||
    fail "--skip-native-deps still invoked install-deps.sh"
[[ ! -e "$TMP_ROOT/nvim.args" ]] ||
    fail "--skip-nvim still invoked Mason update"
printf '%s\n' "$output" | grep -F "skipped: update dependency phase via --skip-deps/--skip-native-deps" >/dev/null ||
    fail "native-dependency skip boundary was not reported"

echo "OK"
