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

PATH="$TMP_ROOT/fakebin:/usr/bin:/bin"
PM=brew_missing
INSTALL_DEPS_SCAN_ITEMS=$'present-tool|command|present-tool\nmissing-tool|command|missing-tool'

pm_install() {
    fail "table scan attempted a package-manager install"
}

maybe_install_brew() {
    fail "table scan attempted a Homebrew bootstrap"
}

install() {
    fail "table scan attempted an install function"
}

output="$(print_install_dependency_table)"

printf '%s\n' "$output" | grep -Eq '^brew[[:space:]]+missing[[:space:]]+-[[:space:]]+install[[:space:]]*$' \
    || fail "pre-bootstrap package manager row missing or wrong"
printf '%s\n' "$output" | grep -Eq '^present-tool[[:space:]]+present[[:space:]]+present-tool 1\.2\.3[[:space:]]+skip[[:space:]]*$' \
    || fail "present tool row missing or wrong"
printf '%s\n' "$output" | grep -Eq '^missing-tool[[:space:]]+missing[[:space:]]+-[[:space:]]+install[[:space:]]*$' \
    || fail "missing tool row missing or wrong"
printf '%s\n' "$output" | grep -F '1 present, 2 missing' >/dev/null \
    || fail "summary count missing or wrong"

echo "OK"
