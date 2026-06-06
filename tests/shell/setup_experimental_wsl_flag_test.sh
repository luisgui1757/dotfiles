#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-experimental-wsl-flag-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/deps.args"
printf '%s\n' "${DOTFILES_EXPERIMENTAL_WSL_GUI:-}" > "$(cd "$(dirname "$0")" && pwd -P)/deps.env"
EOF
cat > "$TMP_ROOT/bootstrap.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/bootstrap.args"
printf '%s\n' "${DOTFILES_EXPERIMENTAL_WSL_GUI:-}" > "$(cd "$(dirname "$0")" && pwd -P)/bootstrap.env"
EOF

output="$(bash "$TMP_ROOT/setup.sh" --all --skip-nvim --experimental-wsl-gui 2>&1)"
[[ "$output" == *"setup.sh: done"* ]]
grep -Fx -- "--experimental-wsl-gui" "$TMP_ROOT/deps.args" >/dev/null
grep -Fx -- "--experimental-wsl-gui" "$TMP_ROOT/bootstrap.args" >/dev/null
grep -Fx -- "1" "$TMP_ROOT/deps.env" >/dev/null
grep -Fx -- "1" "$TMP_ROOT/bootstrap.env" >/dev/null

echo "OK"
