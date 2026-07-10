#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bin" "$tmp/cache"

cat > "$tmp/bin/npx" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
while [[ "$1" != "--" ]]; do shift; done
shift
exec "$@"
EOF

cat > "$tmp/bin/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case " $* " in
    *" renovate-config-validator "*) exit 0 ;;
    *" renovate --platform=local --dry-run=extract "*) exit 0 ;;
    *) echo "unexpected fake npm invocation: $*" >&2; exit 91 ;;
esac
EOF

cat > "$tmp/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "FAIL: inventory parser ran without extraction proof" >&2
exit 92
EOF
chmod +x "$tmp/bin/npx" "$tmp/bin/npm" "$tmp/bin/python3"

set +e
output="$({
    PATH="$tmp/bin:/usr/bin:/bin" \
    CI=true \
    NPM_CONFIG_CACHE="$tmp/cache" \
    bash "$REPO_ROOT/scripts/validate-renovate.sh"
} 2>&1)"
rc=$?
set -e

[[ "$rc" -ne 0 ]] || {
    echo "FAIL: empty successful Renovate extraction was accepted"
    exit 1
}
[[ "$output" == *"succeeded but emitted no JSON proof"* ]] || {
    echo "FAIL: empty extraction did not produce explicit recovery evidence"
    printf '%s\n' "$output"
    exit 1
}
[[ "$output" != *"inventory parser ran without extraction proof"* ]] || {
    echo "FAIL: missing extraction reached the inventory parser"
    exit 1
}

echo "OK"
