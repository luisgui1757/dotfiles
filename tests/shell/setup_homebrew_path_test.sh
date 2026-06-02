#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-homebrew-path-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home/.linuxbrew/bin"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
: > "$TMP_ROOT/bootstrap.sh"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/deps.args"
EOF

cat > "$TMP_ROOT/home/.linuxbrew/bin/brew" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "shellenv" ]]; then
    cat <<BREWENV
export HOMEBREW_PREFIX="$HOME/.linuxbrew";
export HOMEBREW_CELLAR="$HOME/.linuxbrew/Cellar";
export HOMEBREW_REPOSITORY="$HOME/.linuxbrew/Homebrew";
export PATH="$HOME/.linuxbrew/bin:$HOME/.linuxbrew/sbin:\$PATH";
BREWENV
fi
EOF

cat > "$TMP_ROOT/home/.linuxbrew/bin/nvim" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$SETUP_TEST_ROOT/nvim.log"
EOF

chmod +x "$TMP_ROOT/home/.linuxbrew/bin/brew" "$TMP_ROOT/home/.linuxbrew/bin/nvim"

output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --skip-bootstrap </dev/null)"

[[ "$output" == *"Phase 3/4: sync Neovim plugins"* ]]
[[ "$output" == *"Phase 4/4: install LSP servers + formatters"* ]]
[[ "$output" != *"nvim not on PATH yet"* ]]
grep -F -- "--headless +Lazy! sync +qa" "$TMP_ROOT/nvim.log" >/dev/null
grep -F -- "--headless +MasonToolsInstallSync +qa" "$TMP_ROOT/nvim.log" >/dev/null

echo "OK"
