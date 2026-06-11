#!/usr/bin/env bash
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

cat > "$TMP_ROOT/fakebin/present-tool" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
    printf '%s\n' "present-tool 1.2.3"
    exit 0
fi
exit 0
EOF
chmod +x "$TMP_ROOT/fakebin/present-tool"

PATH="$TMP_ROOT/fakebin:$PATH"
PM=apt
INSTALL_DEPS_SCAN_ITEMS=$'present-tool|command|present-tool\nmissing-tool|command|missing-tool'

pm_install() {
    fail "table scan attempted a package-manager install"
}

install() {
    fail "table scan attempted an install function"
}

output="$(print_install_dependency_table)"

printf '%s\n' "$output" | grep -Eq '^present-tool[[:space:]]+present[[:space:]]+present-tool 1\.2\.3[[:space:]]+skip[[:space:]]*$' \
    || fail "present tool row missing or wrong"
printf '%s\n' "$output" | grep -Eq '^missing-tool[[:space:]]+missing[[:space:]]+-[[:space:]]+install[[:space:]]*$' \
    || fail "missing tool row missing or wrong"
printf '%s\n' "$output" | grep -F '1 present, 1 missing' >/dev/null \
    || fail "summary count missing or wrong"

echo "OK"
