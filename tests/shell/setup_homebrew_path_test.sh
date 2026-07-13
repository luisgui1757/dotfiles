#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-homebrew-path-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home/.linuxbrew/bin" "$TMP_ROOT/home/.linuxbrew/opt/make/libexec/gnubin" \
    "$TMP_ROOT/home/.local/state/nix/profile/bin" "$TMP_ROOT/brewbin" "$TMP_ROOT/home"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/deps.args"
EOF

# Fake brew is placed ON PATH (in its own dir, NOT the prefix bin). setup.sh's
# refresh_runtime_path checks `command -v brew` first, so this shadows any real
# Homebrew on the host (e.g. /opt/homebrew on macOS runners) and keeps the test
# hermetic. The fake nvim lives in the prefix bin, which is NOT on PATH until the
# refresh evals this brew's shellenv -- so the test still exercises that refresh.
cat > "$TMP_ROOT/brewbin/brew" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "shellenv" ]]; then
    cat <<BREWENV
export HOMEBREW_PREFIX="$HOME/.linuxbrew";
export PATH="$HOME/.linuxbrew/bin:\$PATH";
BREWENV
elif [[ "${1:-}" == "--prefix" && "${2:-}" == "make" ]]; then
    printf '%s\n' "$HOME/.linuxbrew/opt/make"
fi
EOF
cat > "$TMP_ROOT/brewbin/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -s) echo Linux ;;
    -m) echo x86_64 ;;
    *) command /usr/bin/uname "$@" ;;
esac
EOF
cat > "$TMP_ROOT/brewbin/id" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -u) echo 1000 ;;
    -un) echo tester ;;
    *) exit 1 ;;
esac
EOF
cat > "$TMP_ROOT/brewbin/getent" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == passwd && "\${2:-}" == tester ]] || exit 1
printf '%s\n' 'tester:x:1000:1000:Test User:$TMP_ROOT/home:/bin/zsh'
EOF

cat > "$TMP_ROOT/home/.linuxbrew/bin/nvim" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${DOTFILES_TREESITTER_SYNC_INSTALL:-}" == "1" ]]; then
    printf '%s\n' "DOTFILES_TREESITTER_SYNC_INSTALL=1" >> "$SETUP_TEST_ROOT/nvim.log"
fi
printf 'PATH=%s\n' "$PATH" >> "$SETUP_TEST_ROOT/nvim.log"
printf '%s\n' "$*" >> "$SETUP_TEST_ROOT/nvim.log"
EOF

chmod +x "$TMP_ROOT/brewbin/brew" "$TMP_ROOT/brewbin/uname" \
    "$TMP_ROOT/brewbin/id" "$TMP_ROOT/brewbin/getent" \
    "$TMP_ROOT/home/.linuxbrew/bin/nvim"
# A provisioned macOS test host has an absolute /run/current-system profile.
# The fake user's final Nix-profile entry must keep the fixture nvim ahead of
# that host path while the gnubin assertion still proves Brew shellenv ran.
ln -s "$TMP_ROOT/home/.linuxbrew/bin/nvim" "$TMP_ROOT/home/.local/state/nix/profile/bin/nvim"
ln -s "$TMP_ROOT/brewbin/brew" "$TMP_ROOT/home/.local/state/nix/profile/bin/brew"

output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="$TMP_ROOT/brewbin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --skip-deps --skip-bootstrap --skip-agents </dev/null)"

[[ "$output" == *"Phase 3/6: restore Neovim plugins"* ]]
[[ "$output" == *"Phase 4/6: install Tree-sitter parsers"* ]]
[[ "$output" == *"Phase 5/6: install LSP servers + formatters"* ]]
[[ "$output" != *"nvim not on PATH yet"* ]]
grep -F -- "--headless +Lazy! restore +qa" "$TMP_ROOT/nvim.log" >/dev/null
grep -F -- "DOTFILES_TREESITTER_SYNC_INSTALL=1" "$TMP_ROOT/nvim.log" >/dev/null
grep -F -- "--headless +lua require('lazy').load({ plugins = { 'nvim-treesitter' } }) +qa" "$TMP_ROOT/nvim.log" >/dev/null
grep -F -- "--headless +lua require('util.mason_tools').run_checked('MasonToolsInstallSync')" "$TMP_ROOT/nvim.log" >/dev/null
grep -F "$TMP_ROOT/home/.linuxbrew/opt/make/libexec/gnubin" "$TMP_ROOT/nvim.log" >/dev/null

echo "OK"
