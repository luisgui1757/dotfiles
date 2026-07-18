#!/usr/bin/env bash
# Pi gets the audited Rose Pine data file while setup changes only the global
# `theme` setting. The merge helper must preserve every unrelated preference,
# serialize with Pi's lock convention, and fail without damaging bad input.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
THEME="$REPO_ROOT/pi/rose-pine.json"
MIRROR="$REPO_ROOT/home/dot_pi/agent/themes/rose-pine.json"
HELPER="$REPO_ROOT/scripts/configure-pi-theme.mjs"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

command -v node >/dev/null 2>&1 || fail "node is required"
cmp -s "$THEME" "$MIRROR" || fail "canonical Pi theme and chezmoi mirror differ"
for integration in \
    'setup.sh:configure_pi_theme_selection' \
    'setup.ps1:Invoke-PiThemeSelectionMerge' \
    'uninstall.sh:unset_pi_theme_selection' \
    'uninstall.ps1:Invoke-PiThemeSelectionCleanup'
do
    file="${integration%%:*}"
    function_name="${integration#*:}"
    grep -F "$function_name" "$REPO_ROOT/$file" >/dev/null ||
        fail "$file does not integrate the Pi theme settings helper"
done

python3 - "$THEME" <<'PY'
import json
import pathlib
import sys

theme = json.loads(pathlib.Path(sys.argv[1]).read_text())
expected_vars = {
    "base": "#191724", "surface": "#1f1d2e", "overlay": "#26233a",
    "muted": "#6e6a86", "subtle": "#908caa", "text": "#e0def4",
    "love": "#eb6f92", "gold": "#f6c177", "rose": "#ebbcba",
    "pine": "#31748f", "foam": "#9ccfd8", "iris": "#c4a7e7",
    "highlightLow": "#21202e", "highlightMed": "#403d52",
    "highlightHigh": "#524f67",
}
expected_colors = {
    "accent", "bashMode", "border", "borderAccent", "borderMuted",
    "customMessageBg", "customMessageLabel", "customMessageText", "dim",
    "error", "mdCode", "mdCodeBlock", "mdCodeBlockBorder", "mdHeading",
    "mdHr", "mdLink", "mdLinkUrl", "mdListBullet", "mdQuote",
    "mdQuoteBorder", "muted", "selectedBg", "success", "syntaxComment",
    "syntaxFunction", "syntaxKeyword", "syntaxNumber", "syntaxOperator",
    "syntaxPunctuation", "syntaxString", "syntaxType", "syntaxVariable",
    "text", "thinkingHigh", "thinkingLow", "thinkingMedium",
    "thinkingMinimal", "thinkingOff", "thinkingText", "thinkingXhigh",
    "toolDiffAdded", "toolDiffContext", "toolDiffRemoved", "toolErrorBg",
    "toolOutput", "toolPendingBg", "toolSuccessBg", "toolTitle",
    "userMessageBg", "userMessageText", "warning",
}
if theme.get("name") != "rose-pine":
    raise SystemExit("Pi theme name must be rose-pine")
if theme.get("vars") != expected_vars:
    raise SystemExit("Pi Rose Pine palette drifted")
if set(theme.get("colors", {})) != expected_colors:
    raise SystemExit("Pi theme must define the exact 51-token schema")
if theme.get("export") != {
    "pageBg": "#191724", "cardBg": "#1f1d2e", "infoBg": "#26233a"
}:
    raise SystemExit("Pi export palette drifted")
PY

# Absent settings become a minimal selection file.
settings="$WORK/absent/settings.json"
node "$HELPER" set "$settings" rose-pine >/dev/null
python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {"theme": "rose-pine"}
PY

# Existing settings retain nested and unknown values; a second set is byte-idempotent.
settings="$WORK/merge/settings.json"
mkdir -p "$(dirname "$settings")"
printf '%s\n' '{"theme":"dark","defaultProvider":"example","nested":{"keep":true}}' > "$settings"
node "$HELPER" set "$settings" rose-pine >/dev/null
python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {
    "theme": "rose-pine", "defaultProvider": "example", "nested": {"keep": True}
}
PY
before="$(shasum -a 256 "$settings" | awk '{print $1}')"
node "$HELPER" set "$settings" rose-pine >/dev/null
after="$(shasum -a 256 "$settings" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail "idempotent Pi theme set rewrote settings"

# Invalid JSON and a non-object root fail closed without changing the bytes.
for invalid in broken array; do
    settings="$WORK/$invalid/settings.json"
    mkdir -p "$(dirname "$settings")"
    if [[ "$invalid" == broken ]]; then
        printf '%s\n' '{"theme":' > "$settings"
    else
        printf '%s\n' '[]' > "$settings"
    fi
    before="$(shasum -a 256 "$settings" | awk '{print $1}')"
    if node "$HELPER" set "$settings" rose-pine >/dev/null 2>&1; then
        fail "invalid $invalid Pi settings were accepted"
    fi
    after="$(shasum -a 256 "$settings" | awk '{print $1}')"
    [[ "$before" == "$after" ]] || fail "invalid $invalid Pi settings changed"
done

# An active Pi-compatible settings.json.lock blocks the merge and preserves bytes.
settings="$WORK/locked/settings.json"
mkdir -p "$(dirname "$settings")" "$settings.lock"
printf '%s\n' '{"theme":"dark","keep":1}' > "$settings"
before="$(shasum -a 256 "$settings" | awk '{print $1}')"
if node "$HELPER" set "$settings" rose-pine >/dev/null 2>&1; then
    fail "Pi theme helper ignored the active settings lock"
fi
after="$(shasum -a 256 "$settings" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail "locked Pi settings changed"

# Uninstall removes only the managed selection. A later user choice wins.
settings="$WORK/unset-managed/settings.json"
mkdir -p "$(dirname "$settings")"
printf '%s\n' '{"theme":"rose-pine","keep":1}' > "$settings"
node "$HELPER" unset "$settings" rose-pine >/dev/null
python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {"keep": 1}
PY

settings="$WORK/unset-user/settings.json"
mkdir -p "$(dirname "$settings")"
printf '%s\n' '{"theme":"light","keep":1}' > "$settings"
before="$(shasum -a 256 "$settings" | awk '{print $1}')"
node "$HELPER" unset "$settings" rose-pine >/dev/null
after="$(shasum -a 256 "$settings" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail "uninstall overwrote a user-selected Pi theme"

# Unsetting absent settings is a no-op and must not create .pi directories.
settings="$WORK/unset-absent/settings.json"
node "$HELPER" unset "$settings" rose-pine >/dev/null
[[ ! -e "$(dirname "$settings")" ]] || fail "absent Pi settings cleanup created directories"

echo "all Pi Rose Pine theme and settings-merge invariants OK"
