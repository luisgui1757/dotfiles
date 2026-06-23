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
DEPS_FLAGS=(--update)
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
grep -Fx -- '--headless +MasonToolsUpdateSync +qa' "$TMP_ROOT/nvim.args" >/dev/null \
    || fail "setup update did not invoke MasonToolsUpdateSync"
if printf '%s\n' "$output" | grep -Eq 'chezmoi|Lazy(!| sync)|Tree-sitter|MasonToolsInstallSync|Phase 2|Phase 3'; then
    echo "$output"
    fail "setup update touched config, Lazy, Tree-sitter, or Mason install paths"
fi
printf '%s\n' "$output" | grep -F "Plugins (lazy-lock.json), pinned binaries, and configs update via \`git pull\` then re-run setup" >/dev/null \
    || fail "closing note for pinned core updates was missing"

echo "OK"
