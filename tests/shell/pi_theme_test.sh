#!/usr/bin/env bash
# Exact reviewed Pi theme palette and token mapping. Settings lifecycle is tested in Go.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
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

echo "Pi Rose Pine theme invariants OK"
