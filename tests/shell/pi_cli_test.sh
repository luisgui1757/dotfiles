#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2030,SC2031
# Unit tests for the pinned Pi CLI npm installer. The real package is installed
# from npm by setup; these tests stub node/npm so no network or global install runs.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$REPO_ROOT/tests/.cache/pi-cli-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

write_node_stub() {
    local dir="$1" ready="${2:-1}"
    cat > "$dir/node" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--version" ]]; then
    printf '%s\n' "${ready:+v24.11.0}"
    exit 0
fi
if [[ "\${1:-}" == "-e" ]]; then
    exit $([[ "$ready" == "1" ]] && echo 0 || echo 1)
fi
exit 0
EOF
    chmod +x "$dir/node"
}

write_npm_stub() {
    local dir="$1" integrity="$2"
    cat > "$dir/npm" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
    "view @earendil-works/pi-coding-agent@0.80.3")
        printf '%s\n' "${PI_TEST_INTEGRITY:-}"
        exit 0
        ;;
    "install -g")
        printf '%s\n' "$*" >> "${PI_TEST_LOG:?}"
        mkdir -p "$HOME/.local/bin"
        cat > "$HOME/.local/bin/pi" <<'EOS'
#!/usr/bin/env bash
printf '%s\n' "0.80.3"
EOS
        chmod +x "$HOME/.local/bin/pi"
        exit 0
        ;;
    "prefix -g")
        printf '%s\n' "$HOME/.local/bin"
        exit 0
        ;;
esac
printf 'unexpected npm args: %s\n' "$*" >&2
exit 2
EOF
    chmod +x "$dir/npm"
    PI_TEST_INTEGRITY="$integrity"; export PI_TEST_INTEGRITY
}

# --- 1. Already at the pinned version -> idempotent, no npm call -------------
(
    HOME="$TMP_ROOT/current-home"; export HOME
    bin="$TMP_ROOT/current-bin"; mkdir -p "$bin" "$HOME"
    PATH="$bin:$PATH"; export PATH
    cat > "$bin/pi" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "0.80.3"
EOF
    chmod +x "$bin/pi"
    cat > "$bin/npm" <<'EOF'
#!/usr/bin/env bash
exit 99
EOF
    chmod +x "$bin/npm"
    YES_ALL=1; DRY_RUN=0
    out="$(install_pi_cli 2>&1)"
    [[ "$out" == *"already installed (0.80.3)"* ]] || fail "current pi must be idempotent; got: $out"
)

# --- 2. Dry-run prints pinned package + expected integrity -------------------
(
    HOME="$TMP_ROOT/dry-home"; export HOME
    bin="$TMP_ROOT/dry-bin"; mkdir -p "$bin" "$HOME"
    PATH="$bin:$PATH"; export PATH
    write_node_stub "$bin" 1
    write_npm_stub "$bin" "$PI_CLI_INTEGRITY"
    YES_ALL=1; DRY_RUN=1
    out="$(install_pi_cli 2>&1)"
    [[ "$out" == *"npm view ${PI_CLI_PACKAGE}@${PI_CLI_VERSION} dist.integrity"* ]] || fail "dry-run missing integrity probe; got: $out"
    [[ "$out" == *"$PI_CLI_INTEGRITY"* ]] || fail "dry-run missing expected integrity; got: $out"
    [[ "$out" == *"npm install -g --prefix"* && "$out" == *"${PI_CLI_PACKAGE}@${PI_CLI_VERSION}"* ]] || fail "dry-run missing pinned install; got: $out"
)

# --- 3. Integrity mismatch fails before npm install --------------------------
(
    HOME="$TMP_ROOT/mismatch-home"; export HOME
    bin="$TMP_ROOT/mismatch-bin"; mkdir -p "$bin" "$HOME"
    PATH="$bin:$PATH"; export PATH
    log="$TMP_ROOT/mismatch.log"; PI_TEST_LOG="$log"; export PI_TEST_LOG
    write_node_stub "$bin" 1
    write_npm_stub "$bin" "sha512-not-the-pin"
    YES_ALL=1; DRY_RUN=0
    set +e
    out="$(install_pi_cli 2>&1)"
    rc=$?
    set -e
    [[ "$rc" -ne 0 ]] || fail "integrity mismatch must return nonzero"
    [[ "$out" == *"integrity mismatch"* ]] || fail "integrity mismatch output missing; got: $out"
    [[ ! -e "$log" ]] || fail "npm install ran despite integrity mismatch: $(cat "$log")"
)

# --- 4. Verified install writes ~/.local/bin/pi and verifies the version ------
(
    HOME="$TMP_ROOT/install-home"; export HOME
    bin="$TMP_ROOT/install-bin"; mkdir -p "$bin" "$HOME"
    PATH="$bin:$PATH"; export PATH
    log="$TMP_ROOT/install.log"; PI_TEST_LOG="$log"; export PI_TEST_LOG
    write_node_stub "$bin" 1
    write_npm_stub "$bin" "$PI_CLI_INTEGRITY"
    YES_ALL=1; DRY_RUN=0
    out="$(install_pi_cli 2>&1)"
    [[ "$out" == *"installed"*"0.80.3"* ]] || fail "install success missing; got: $out"
    [[ -x "$HOME/.local/bin/pi" ]] || fail "install did not publish ~/.local/bin/pi"
    grep -F "install -g --prefix $HOME/.local ${PI_CLI_PACKAGE}@${PI_CLI_VERSION}" "$log" >/dev/null \
        || fail "npm install args wrong: $(cat "$log")"
)

echo "OK"
