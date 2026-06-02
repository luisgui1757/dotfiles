#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-local-bin-path-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/home/.local/bin" "$TMP_ROOT/brewbin"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
: > "$TMP_ROOT/bootstrap.sh"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat > "$HOME/.local/bin/nvim" <<'NVIM'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$SETUP_TEST_ROOT/nvim.log"
NVIM
chmod +x "$HOME/.local/bin/nvim"
EOF
chmod +x "$TMP_ROOT/install-deps.sh"

# Fake brew placed ON PATH so refresh_runtime_path's `command -v brew` finds it
# FIRST and evals its (no-op) shellenv, shadowing any real Homebrew on the host
# (e.g. /opt/homebrew on macOS runners, /home/linuxbrew on Ubuntu). With the real
# brew shadowed, the refresh falls through to appending ~/.local/bin -- where the
# fake nvim lives -- which is exactly what this test verifies. The brew stub also
# records that its shellenv ran (brew.log) so the dry-run case can assert it did NOT.
cat > "$TMP_ROOT/brewbin/brew" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "shellenv" ]]; then
    printf 'ran\n' > "$SETUP_TEST_ROOT/brew.log"
fi
EOF
chmod +x "$TMP_ROOT/brewbin/brew"

output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="$TMP_ROOT/brewbin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --skip-bootstrap </dev/null)"

[[ "$output" == *"Phase 3/4: sync Neovim plugins"* ]]
[[ "$output" == *"Phase 4/4: install LSP servers + formatters"* ]]
[[ "$output" != *"nvim not on PATH yet"* ]]
grep -F -- "--headless +Lazy! sync +qa" "$TMP_ROOT/nvim.log" >/dev/null
grep -F -- "--headless +MasonToolsInstallSync +qa" "$TMP_ROOT/nvim.log" >/dev/null

rm -f "$TMP_ROOT/nvim.log" "$TMP_ROOT/brew.log"

output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="$TMP_ROOT/brewbin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --dry-run --all </dev/null)"

[[ "$output" == *"skipped: Phase 3-4 (nvim plugins) in --dry-run mode"* ]]
if [[ -e "$TMP_ROOT/brew.log" ]]; then
    echo "FAIL: setup.sh refreshed Homebrew PATH during dry-run" >&2
    exit 1
fi

echo "OK"
