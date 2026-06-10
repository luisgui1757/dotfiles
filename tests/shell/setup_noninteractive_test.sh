#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-noninteractive-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/deps.args"
EOF

output="$(bash "$TMP_ROOT/setup.sh" --skip-bootstrap --skip-nvim </dev/null)"

[[ "$output" == *"note: no TTY detected; running with --all"* ]]
grep -Fx -- "--all" "$TMP_ROOT/deps.args" >/dev/null

echo "OK"
