#!/usr/bin/env bash
# Regression guard for the VS Code Rose Pine theme setter in install-deps.sh.
# set_vscode_theme writes the Rose Pine theme plus Hack Nerd Font settings into
# settings.json. The theme label has U+00E9; assert its UTF-8 bytes via
# $'\xc3\xa9' so this test does not hardcode the glyph.
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# shellcheck disable=SC1090,SC1091  # dynamic path; shellcheck can't follow it
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
set +e

fail() { echo "FAIL: $1"; exit 1; }
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
eacute=$'\xc3\xa9'   # UTF-8 bytes for U+00E9
theme="Ros${eacute} Pine"
font="'Hack Nerd Font', Consolas, monospace"

json_value() {
    jq -r --arg key "$1" '.[$key]' "$2"
}

assert_all_settings_with_jq() {
    local file="$1"
    [[ "$(json_value "workbench.colorTheme" "$file")" == "$theme" ]] || fail "$file missing Rose Pine theme"
    # Forced dark: both preferred slots point at the SAME dark theme, and
    # autoDetect is pinned off as a real JSON boolean (string "false" would be
    # silently ignored by VS Code and let it fall back to the default theme).
    [[ "$(json_value "workbench.preferredDarkColorTheme" "$file")" == "$theme" ]] || fail "$file missing preferredDark theme"
    [[ "$(json_value "workbench.preferredLightColorTheme" "$file")" == "$theme" ]] || fail "$file missing preferredLight theme"
    jq -e '."window.autoDetectColorScheme" == false' "$file" >/dev/null || fail "$file autoDetectColorScheme is not boolean false"
    [[ "$(json_value "editor.fontFamily" "$file")" == "$font" ]] || fail "$file missing editor font"
    [[ "$(json_value "terminal.integrated.fontFamily" "$file")" == "$font" ]] || fail "$file missing terminal font"
}

live_theme_count() {
    grep -Ec '^  "workbench\.colorTheme"[[:space:]]*:' "$1"
}

assert_all_settings_text() {
    local file="$1"
    grep -Fq "\"workbench.colorTheme\": \"$theme\"" "$file" || fail "$file missing Rose Pine theme text"
    grep -Fq "\"workbench.preferredDarkColorTheme\": \"$theme\"" "$file" || fail "$file missing preferredDark text"
    grep -Fq "\"workbench.preferredLightColorTheme\": \"$theme\"" "$file" || fail "$file missing preferredLight text"
    grep -Eq '"window\.autoDetectColorScheme"[[:space:]]*:[[:space:]]*false' "$file" || fail "$file missing bare-false autoDetect"
    grep -Fq '"window.autoDetectColorScheme": "false"' "$file" && fail "$file wrote autoDetect as a STRING, not a boolean"
    grep -Fq "\"editor.fontFamily\": \"$font\"" "$file" || fail "$file missing editor font text"
    grep -Fq "\"terminal.integrated.fontFamily\": \"$font\"" "$file" || fail "$file missing terminal font text"
}

command -v jq >/dev/null 2>&1 || fail "jq is required for this regression test"

# 1) Absent settings.json -> fresh file with all keys and the U+00E9 bytes.
s1="$WORK/a/settings.json"
set_vscode_theme "$s1" >/dev/null
[[ -f "$s1" ]] || fail "fresh settings.json was not created"
assert_all_settings_with_jq "$s1"
grep -q "$eacute" "$s1" || fail "fresh theme value lost its U+00E9 UTF-8 bytes"

# 2) Existing valid JSON -> jq merge, preserving other keys.
s2="$WORK/b/settings.json"; mkdir -p "$WORK/b"
printf '{ "editor.fontSize": 14 }\n' > "$s2"
set_vscode_theme "$s2" >/dev/null
[[ "$(jq -r '."editor.fontSize"' "$s2")" == "14" ]] || fail "merge clobbered an existing key"
assert_all_settings_with_jq "$s2"
grep -q "$eacute" "$s2" || fail "merged theme lost its U+00E9 bytes"

# 3) JSONC with comments -> comment-aware edit and idempotency.
s3="$WORK/c/settings.json"; mkdir -p "$WORK/c"
printf '// header comment\n{\n  "editor.fontSize": 14, // inline\n}\n' > "$s3"
set_vscode_theme "$s3" >/dev/null
grep -Fq '// header comment' "$s3" || fail "JSONC header comment was not preserved"
grep -Fq '// inline' "$s3" || fail "JSONC inline comment was not preserved"
grep -Fq '"editor.fontSize": 14' "$s3" || fail "JSONC edit clobbered editor.fontSize"
assert_all_settings_text "$s3"
[[ "$(live_theme_count "$s3")" == "1" ]] || fail "JSONC edit wrote duplicate live theme keys"
compgen -G "$s3.bak.*" >/dev/null || fail "JSONC edit did not create a backup"
set_vscode_theme "$s3" >/dev/null
[[ "$(live_theme_count "$s3")" == "1" ]] || fail "second JSONC edit wrote duplicate live theme keys"

# 4) Existing JSONC top-level theme -> replace in place, no duplicate key.
s4="$WORK/d/settings.json"; mkdir -p "$WORK/d"
printf '{\n  // keep this comment\n  "workbench.colorTheme": "Old",\n  "editor.fontSize": 14\n}\n' > "$s4"
set_vscode_theme "$s4" >/dev/null
[[ "$(live_theme_count "$s4")" == "1" ]] || fail "existing JSONC theme was duplicated"
grep -Fq "\"workbench.colorTheme\": \"$theme\"" "$s4" || fail "existing JSONC theme was not replaced"
grep -Fq '"workbench.colorTheme": "Old"' "$s4" && fail "old JSONC theme value survived"
grep -Fq '// keep this comment' "$s4" || fail "comment near existing JSONC theme was not preserved"
assert_all_settings_text "$s4"

# 5) Commented-out key and string mentions are not mistaken for live keys.
s5="$WORK/e/settings.json"; mkdir -p "$WORK/e"
printf '{\n  // "workbench.colorTheme": "x"\n  "notes": "workbench.colorTheme inside a string value",\n  "editor.fontSize": 14\n}\n' > "$s5"
set_vscode_theme "$s5" >/dev/null
grep -Fq '// "workbench.colorTheme": "x"' "$s5" || fail "commented-out theme line was not preserved"
grep -Fq '"notes": "workbench.colorTheme inside a string value"' "$s5" || fail "string value mentioning theme was not preserved"
[[ "$(live_theme_count "$s5")" == "1" ]] || fail "commented/string theme mention confused live key count"
assert_all_settings_text "$s5"

# 6) Nested keys are left alone, and CRLF settings stay CRLF.
s6="$WORK/f/settings.json"; mkdir -p "$WORK/f"
printf '{\r\n  // force JSONC fallback\r\n  "nested": {\r\n    "workbench.colorTheme": "Nested Old"\r\n  }\r\n}\r\n' > "$s6"
set_vscode_theme "$s6" >/dev/null
grep -Fq '"workbench.colorTheme": "Nested Old"' "$s6" || fail "nested theme key was replaced"
[[ "$(live_theme_count "$s6")" == "1" ]] || fail "top-level theme key missing or duplicated with nested key present"
assert_all_settings_text "$s6"
cr_char=$'\r'
cr_count="$(LC_ALL=C tr -cd "$cr_char" < "$s6" | wc -c | tr -d ' ')"
[[ "$cr_count" -gt 0 ]] || fail "CRLF JSONC file lost carriage returns"

echo "OK"
