#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-noninteractive-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home"
mkdir -p "$TMP_ROOT/fakebin"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/deps.args"
EOF
cat > "$TMP_ROOT/fakebin/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -s) printf '%s\n' Linux ;;
    -m) printf '%s\n' x86_64 ;;
    *) command uname "$@" ;;
esac
EOF
cat > "$TMP_ROOT/fakebin/nix" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "eval" ]]; then
    printf '%s\n%s\n' "fake-home-manager-rev" "sha256-fake"
fi
EOF
cat > "$TMP_ROOT/fakebin/home-manager" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$TMP_ROOT/fakebin/id" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in -u) echo 1000 ;; -un) echo tester ;; *) exit 1 ;; esac
EOF
cat > "$TMP_ROOT/fakebin/getent" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == passwd && "\${2:-}" == tester ]] || exit 1
printf '%s\n' 'tester:x:1000:1000:Test User:$HOME:/bin/zsh'
EOF
chmod +x "$TMP_ROOT/fakebin/uname" "$TMP_ROOT/fakebin/nix" "$TMP_ROOT/fakebin/home-manager" \
    "$TMP_ROOT/fakebin/id" "$TMP_ROOT/fakebin/getent"

output="$(PATH="$TMP_ROOT/fakebin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --skip-bootstrap --skip-nvim --skip-agents </dev/null)"

[[ "$output" == *"note: no TTY detected; running with --all"* ]]
grep -Fx -- "--all" "$TMP_ROOT/deps.args" >/dev/null

echo "OK"
