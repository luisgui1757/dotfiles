#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

workflow=.github/workflows/e2e-install.yml

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

grep -F "[ ! -d \"\$HOME/.local/bin\" ] || PATH=\"\$HOME/.local/bin:\$PATH\"" "$workflow" >/dev/null \
    || fail "POSIX post-install validation must model the managed zsh user-local PATH"
grep -F "aerospace_app_identity=\"\$(\"\$aerospace_app_bin\" --version)\"" "$workflow" >/dev/null \
    || fail "macOS e2e must invoke the installed AeroSpace app binary"
grep -F "aerospace_cli_identity=\"\$(aerospace --version 2>&1)\"" "$workflow" >/dev/null \
    || fail "macOS e2e must invoke the installed AeroSpace CLI"
grep -F "aerospace CLI client version: \$aerospace_app_identity" "$workflow" >/dev/null \
    || fail "macOS e2e must bind the installed AeroSpace app and CLI identities"
grep -F "UNAVAILABLE: AeroSpace config-consumption runtime proof requires a user-granted Accessibility (TCC) desktop session" "$workflow" >/dev/null \
    || fail "hosted CI must classify the missing TCC-backed AeroSpace proof explicitly"
grep -F "AeroSpace loaded \$aerospace_config" "$workflow" >/dev/null \
    && fail "hosted CI must not claim AeroSpace config consumption without TCC"
grep -F "open -gja AeroSpace" "$workflow" >/dev/null \
    && fail "hosted CI must not launch AeroSpace into its pre-config TCC wait loop"
grep -F '**AeroSpace managed-config consumption**' tests/MANUAL.md >/dev/null \
    || fail "manual ledger must retain the TCC-backed AeroSpace config proof"

grep -F "for package in wezterm aerospace herdr; do" "$workflow" >/dev/null \
    || fail "macOS e2e must check every nix-darwin-owned package"
grep -F "grep -Eq \"^[[:space:]]*ok[[:space:]]+\$package[[:space:]]+already installed\$\" \"\$setup_log\"" "$workflow" >/dev/null \
    || fail "macOS e2e must reject a native retry hiding failed nix-darwin installation"
grep -F "expected_brew_version=\"\$(nix eval --raw '.#darwinConfigurations.dotfiles.config.nix-homebrew.package.version')\"" "$workflow" >/dev/null \
    || fail "macOS e2e must obtain the selected Nix package version"
grep -F "[[ \"\${brew_identity%%\$'\\n'*}\" == \"Homebrew \$expected_brew_version\" ]]" "$workflow" >/dev/null \
    || fail "macOS e2e must compare the installed Homebrew version to the Nix package"
grep -F "HOMEBREW_DEVELOPER=1 brew ruby \"\$repo/tests/nix/homebrew_cask_compatibility.rb\"" "$workflow" >/dev/null \
    || fail "macOS e2e must run the real Homebrew cask probe without persisting developer mode"

echo "OK"
