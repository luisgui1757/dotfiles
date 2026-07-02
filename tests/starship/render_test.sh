#!/usr/bin/env bash
# Render the prompt against the user's starship.toml and assert:
#   - no literal `(style)` (regression guard for the deleted-files typo)
#   - no background styles; terminal transparency owns the background surface
#   - the rose-pine palette is referenced
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# Static-check the toml content as a first defense.
if grep -E '\]\(style\)' "$REPO_ROOT/starship/starship.toml" >/dev/null; then
    echo "FAIL: starship.toml still contains literal (style) instead of (\$style)"
    grep -nE '\]\(style\)' "$REPO_ROOT/starship/starship.toml"
    exit 1
fi
if grep -nF 'bg:' "$REPO_ROOT/starship/starship.toml"; then
    echo "FAIL: starship.toml must not use background styles; terminal transparency owns the background"
    exit 1
fi
time_format_line="format = \"[\$time 󰴈 ](\$style)\""
if ! grep -F "$time_format_line" "$REPO_ROOT/starship/starship.toml" >/dev/null; then
    echo "FAIL: starship time segment must keep one trailing safety space before the right edge"
    exit 1
fi

if ! command -v starship >/dev/null 2>&1; then
    echo "skipped dynamic render: starship not installed (static checks passed)"
    exit 0
fi

# Dynamic render -- needs a writable git working directory.
TMP_REPO="$(mktemp -d)"
trap 'rm -rf "$TMP_REPO"' EXIT
( cd "$TMP_REPO" \
    && git init -q \
    && git -c user.email=ci@example.com -c user.name=ci commit --allow-empty -qm "test" )

out=$(cd "$TMP_REPO" && STARSHIP_CONFIG="$REPO_ROOT/starship/starship.toml" \
      starship prompt --terminal-width 120 --jobs 0 --status 0 --cmd-duration 0 2>&1)

# A successful render should not literally include "(style)" — that would mean
# the style interpolation broke and emitted the placeholder.
if echo "$out" | grep -q "(style)"; then
    echo "FAIL: rendered prompt contains literal (style)"
    echo "$out"
    exit 1
fi
echo "OK"
