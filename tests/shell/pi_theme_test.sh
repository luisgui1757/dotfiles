#!/usr/bin/env bash
# Pi gets the three canonical Fable-tuned Rose Pine data files while setup
# changes only the global `theme` setting. The merge helper must
# preserve managed variant choices and every unrelated preference, serialize
# with Pi's lock convention, and fail without damaging bad input.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
HELPER="$REPO_ROOT/scripts/configure-pi-theme.mjs"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

command -v node >/dev/null 2>&1 || fail "node is required"
THEME_NAMES=(
    rose-pine rose-pine-moon rose-pine-dawn
)
UNINSTALL_THEME_NAMES=(
    "${THEME_NAMES[@]}"
    rose-pine-fable rose-pine-moon-fable rose-pine-dawn-fable
)
if find "$REPO_ROOT/pi" "$REPO_ROOT/home/dot_pi/agent/themes" \
    -maxdepth 1 -type f -name '*-fable.json' -print | grep -q .; then
    fail "retired Pi *-fable source files remain"
fi
for retired_hash in \
    36d25cc144bc38ab849ec5f47f839c8aa8a8946416557c5e14939183fff56805 \
    45813d7827fbe091f2029f8e0bfccb0927d1923576ebfb94cebb192b5235953c \
    9f33de93c8749e2fc79831e07b175bda5018e08261372fb4e1b4b507408b4ad9 \
    2f18ee6657d6748d13b13287760494e05d4fefad3d10c824becafaf6210c3bf0 \
    f0a8f234c826b37998c3035178b47265c167aa9f1ae8f896f2bf81eeb48f256a \
    57adc5fe3252ed4511d79c31beb9ed46cee6b4d3946fbdc47c71e4bdc094bad8
do
    grep -F "$retired_hash" "$REPO_ROOT/setup.sh" >/dev/null ||
        fail "setup.sh is missing a reviewed retired-theme hash"
    grep -F "$retired_hash" "$REPO_ROOT/setup.ps1" >/dev/null ||
        fail "setup.ps1 is missing a reviewed retired-theme hash"
done
for theme_name in "${THEME_NAMES[@]}"; do
    cmp -s \
        "$REPO_ROOT/pi/$theme_name.json" \
        "$REPO_ROOT/home/dot_pi/agent/themes/$theme_name.json" ||
        fail "canonical Pi $theme_name theme and chezmoi mirror differ"
done
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

python3 - \
    "$REPO_ROOT/pi/rose-pine.json" \
    "$REPO_ROOT/pi/rose-pine-moon.json" \
    "$REPO_ROOT/pi/rose-pine-dawn.json" <<'PY'
import hashlib
import json
import pathlib
import sys

schema = "https://raw.githubusercontent.com/earendil-works/pi/main/packages/coding-agent/src/modes/interactive/theme/theme-schema.json"
expected_hashes = {
    "rose-pine": "47393b1ae8c7e31cb6e484d270604bbb0a98d9c99c04a87cf85bc65f4c7b47f5",
    "rose-pine-moon": "9dcfb2f727ecef62075aab8a7e28a3b84198f605b8a2c6cf615e8868ab0bc556",
    "rose-pine-dawn": "7c32014116c6e4e974a2e5e3126d3b42cd83bd20e19731d93e6854d845b735e4",
}
expected_palettes = {
    "rose-pine": {
        "base": "#191724", "surface": "#1f1d2e", "overlay": "#26233a",
        "muted": "#6e6a86", "subtle": "#908caa", "text": "#e0def4",
        "love": "#eb6f92", "gold": "#f6c177", "rose": "#ebbcba",
        "pine": "#31748f", "foam": "#9ccfd8", "iris": "#c4a7e7",
        "highlightLow": "#21202e", "highlightMed": "#403d52",
        "highlightHigh": "#524f67",
    },
    "rose-pine-moon": {
        "base": "#232136", "surface": "#2a273f", "overlay": "#393552",
        "muted": "#6e6a86", "subtle": "#908caa", "text": "#e0def4",
        "love": "#eb6f92", "gold": "#f6c177", "rose": "#ea9a97",
        "pine": "#3e8fb0", "foam": "#9ccfd8", "iris": "#c4a7e7",
        "highlightLow": "#2a283e", "highlightMed": "#44415a",
        "highlightHigh": "#56526e",
    },
    "rose-pine-dawn": {
        "base": "#faf4ed", "surface": "#fffaf3", "overlay": "#f2e9e1",
        "muted": "#9893a5", "subtle": "#797593", "text": "#575279",
        "love": "#b4637a", "gold": "#ea9d34", "rose": "#d7827e",
        "pine": "#286983", "foam": "#56949f", "iris": "#907aa9",
        "highlightLow": "#f4ede8", "highlightMed": "#dfdad9",
        "highlightHigh": "#cecacd",
    },
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
themes = {}
for raw_path in sys.argv[1:]:
    path = pathlib.Path(raw_path)
    raw = path.read_bytes()
    theme = json.loads(raw)
    name = theme.get("name")
    if name != path.stem or name not in expected_hashes:
        raise SystemExit(f"Pi theme filename/name mismatch: {path.name}: {name!r}")
    if theme.get("$schema") != schema:
        raise SystemExit(f"Pi {name} schema URL drifted")
    if hashlib.sha256(raw).hexdigest() != expected_hashes[name]:
        raise SystemExit(f"Pi {name} reviewed mapping drifted")
    variables = theme.get("vars", {})
    if len(variables) != 39:
        raise SystemExit(f"Pi {name} must keep the 15 official and 24 derived colors")
    for key, value in expected_palettes[name].items():
        if variables.get(key) != value:
            raise SystemExit(f"Pi {name} official palette drifted at {key}")
    colors = theme.get("colors", {})
    if set(colors) != expected_colors:
        raise SystemExit(f"Pi {name} must define the exact 51-token schema")
    undefined = {value for value in colors.values() if value not in variables and value != ""}
    if undefined:
        raise SystemExit(f"Pi {name} references undefined colors: {sorted(undefined)!r}")
    if theme.get("export") != {
        "pageBg": "base", "cardBg": "surface", "infoBg": "highlightLow"
    }:
        raise SystemExit(f"Pi {name} export palette drifted")
    themes[name] = theme

expected_canonical_roles = {
    "rose-pine": {
        "border": "highlightMed",
        "borderMuted": "highlightLow",
        "dim": "mutedDark1",
        "thinkingText": "iris",
        "selectedBg": "highlightMed",
        "userMessageText": "text",
        "syntaxComment": "muted",
        "syntaxFunction": "rose",
        "syntaxVariable": "text",
        "syntaxNumber": "gold",
    },
    "rose-pine-moon": {
        "border": "highlightMed",
        "borderMuted": "highlightLow",
        "dim": "mutedDark1",
        "thinkingText": "iris",
        "selectedBg": "highlightMed",
        "userMessageText": "text",
        "syntaxComment": "muted",
        "syntaxFunction": "rose",
        "syntaxVariable": "text",
        "syntaxNumber": "gold",
    },
    "rose-pine-dawn": {
        "mdLinkUrl": "pine",
    },
}
for name, expected_roles in expected_canonical_roles.items():
    colors = themes[name]["colors"]
    for token, expected_value in expected_roles.items():
        if colors.get(token) != expected_value:
            raise SystemExit(
                f"Pi {name} canonical Fable role drifted at {token}: "
                f"{colors.get(token)!r} != {expected_value!r}"
            )
PY

# Absent settings become a minimal selection file.
settings="$WORK/absent/settings.json"
node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null
python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {"theme": "rose-pine"}
PY

# Existing settings retain nested and unknown values; a second set is byte-idempotent.
settings="$WORK/merge/settings.json"
mkdir -p "$(dirname "$settings")"
printf '%s\n' '{"theme":"dark","defaultProvider":"example","nested":{"keep":true}}' > "$settings"
node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null
python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {
    "theme": "rose-pine", "defaultProvider": "example", "nested": {"keep": True}
}
PY
before="$(shasum -a 256 "$settings" | awk '{print $1}')"
node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null
after="$(shasum -a 256 "$settings" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail "idempotent Pi theme set rewrote settings"

# Setup reruns preserve every canonical choice instead of reselecting Main.
for managed_theme in "${THEME_NAMES[@]}"; do
    settings="$WORK/set-managed-$managed_theme/settings.json"
    mkdir -p "$(dirname "$settings")"
    printf '{"theme":"%s","keep":1}\n' "$managed_theme" > "$settings"
    before="$(shasum -a 256 "$settings" | awk '{print $1}')"
    node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null
    after="$(shasum -a 256 "$settings" | awk '{print $1}')"
    [[ "$before" == "$after" ]] ||
        fail "setup overwrote managed Pi theme: $managed_theme"
done

# A retired trial selection migrates to canonical Main because its alias file is gone.
for retired_theme in rose-pine-fable rose-pine-moon-fable rose-pine-dawn-fable; do
    settings="$WORK/set-retired-$retired_theme/settings.json"
    mkdir -p "$(dirname "$settings")"
    printf '{"theme":"%s","keep":1}\n' "$retired_theme" > "$settings"
    node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null
    python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {
    "theme": "rose-pine", "keep": 1
}
PY
done

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
    if node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null 2>&1; then
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
if node "$HELPER" set "$settings" "${THEME_NAMES[@]}" >/dev/null 2>&1; then
    fail "Pi theme helper ignored the active settings lock"
fi
after="$(shasum -a 256 "$settings" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail "locked Pi settings changed"

# Uninstall removes any managed selection atomically. A later user choice wins.
for managed_theme in "${UNINSTALL_THEME_NAMES[@]}"; do
    settings="$WORK/unset-managed-$managed_theme/settings.json"
    mkdir -p "$(dirname "$settings")"
    printf '{"theme":"%s","keep":1}\n' "$managed_theme" > "$settings"
    node "$HELPER" unset "$settings" "${UNINSTALL_THEME_NAMES[@]}" >/dev/null
    python3 - "$settings" <<'PY'
import json, pathlib, sys
assert json.loads(pathlib.Path(sys.argv[1]).read_text()) == {"keep": 1}
PY
done

settings="$WORK/unset-user/settings.json"
mkdir -p "$(dirname "$settings")"
printf '%s\n' '{"theme":"light","keep":1}' > "$settings"
before="$(shasum -a 256 "$settings" | awk '{print $1}')"
node "$HELPER" unset "$settings" "${UNINSTALL_THEME_NAMES[@]}" >/dev/null
after="$(shasum -a 256 "$settings" | awk '{print $1}')"
[[ "$before" == "$after" ]] || fail "uninstall overwrote a user-selected Pi theme"

# Unsetting absent settings is a no-op and must not create .pi directories.
settings="$WORK/unset-absent/settings.json"
node "$HELPER" unset "$settings" "${UNINSTALL_THEME_NAMES[@]}" >/dev/null
[[ ! -e "$(dirname "$settings")" ]] || fail "absent Pi settings cleanup created directories"

echo "all Pi Rose Pine theme and settings-merge invariants OK"
