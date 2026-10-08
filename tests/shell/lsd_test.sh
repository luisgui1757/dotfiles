#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2016
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"

fail() {
    echo "FAIL: $*"
    exit 1
}

line_no() {
    local pattern="$1" file="$2" match
    match="$(grep -nF "$pattern" "$file" || true)"
    [[ -n "$match" ]] || fail "missing line: $pattern"
    printf '%s\n' "${match%%:*}"
}

zshrc="$ACTIVE_ZSH"
grep -F 'command -v lsd' "$zshrc" >/dev/null \
    || fail "zshrc lsd aliases are not guarded by command -v"
grep -F "di=38;2;246;193;119" "$zshrc" >/dev/null \
    || fail "zshrc does not set Rose Pine directory color in LS_COLORS"
ls_colors_classes=(
    'no' 'fi' 'di' 'ln' 'pi' 'so' 'do' 'bd' 'cd' 'or'
    'mi' 'su' 'sg' 'ca' 'tw' 'ow' 'st' 'ex'
)
for class in "${ls_colors_classes[@]}"; do
    grep -F "$class=" "$zshrc" >/dev/null \
        || fail "zshrc Rose Pine LS_COLORS is missing class: $class"
done
ls_colors_line="$(line_no 'export LS_COLORS=' "$ACTIVE_ZSH_CORE")"
completion_line="$(line_no "zstyle ':completion:*' list-colors" "$ACTIVE_ZSH_CORE")"
[[ "$ls_colors_line" -lt "$completion_line" ]] \
    || fail "LS_COLORS must be initialized before zsh completion list-colors"
zsh_palette="$(env LS_COLORS='di=01;34:ex=01;32:fi=00' \
    HOME="$DOTFILES_SHELL_FIXTURES/zsh" zsh -d -f -i -c 'source "$1" >/dev/null 2>&1 || exit 1; print -r -- "$LS_COLORS"' zsh "$ACTIVE_ZSH_CORE")"
[[ "$zsh_palette" == *'di=38;2;246;193;119'* ]] \
    || fail "zshrc did not install the Rose Pine directory color by default"
[[ "$zsh_palette" != *'di=01;34'* ]] \
    || fail "zshrc inherited ambient LS_COLORS instead of the repo palette"
zsh_override_palette="$(env LS_COLORS='di=01;34:ex=01;32:fi=00' \
    DOTFILES_LS_COLORS='di=01;34:ex=01;32:fi=00' \
    HOME="$DOTFILES_SHELL_FIXTURES/zsh" zsh -d -f -i -c 'source "$1" >/dev/null 2>&1 || exit 1; print -r -- "$LS_COLORS"' zsh "$ACTIVE_ZSH_CORE")"
[[ "$zsh_override_palette" == 'di=01;34:ex=01;32:fi=00' ]] \
    || fail "zshrc did not honor DOTFILES_LS_COLORS as the explicit override"
for value in "ls='lsd'" "l='lsd -l'" "la='lsd -a'" "lla='lsd -la'" "lt='lsd --tree'"; do
    grep -F "$value" "$zshrc" >/dev/null || fail "missing lsd alias: $value"
done

grep -F 'theme: custom' "$REPO_ROOT/lsd/config.yaml" >/dev/null \
    || fail "lsd/config.yaml must enable the custom colors.yaml theme"
grep -F 'user: 222' "$REPO_ROOT/lsd/colors.yaml" >/dev/null \
    || fail "lsd/colors.yaml is missing Rose Pine gold user color"
grep -F 'tree-edge: 103' "$REPO_ROOT/lsd/colors.yaml" >/dev/null \
    || fail "lsd/colors.yaml is missing Rose Pine subtle tree-edge color"

if command -v lsd >/dev/null 2>&1; then
    tmp="$(mktemp -d)"
    trap 'active_shell_cleanup; rm -rf "$tmp"' EXIT
    mkdir -p "$tmp/listing/normal" "$tmp/listing/open" "$tmp/listing/sticky"
    chmod 0777 "$tmp/listing/open"
    chmod 1777 "$tmp/listing/sticky"
    : > "$tmp/listing/plain"
    : > "$tmp/listing/run"
    chmod +x "$tmp/listing/run"
    out="$(env -u NO_COLOR \
        XDG_CONFIG_HOME="$REPO_ROOT" \
        LS_COLORS="$zsh_palette" \
        lsd --color always -la "$tmp/listing")"
    [[ "$out" == *$'\033[38;2;246;193;119mnormal'* ]] \
        || fail "lsd did not apply LS_COLORS Rose Pine normal directory color"
    [[ "$out" == *$'\033[38;2;246;193;119mopen'* ]] \
        || fail "lsd did not apply LS_COLORS Rose Pine other-writable directory color"
    [[ "$out" == *$'\033[38;2;246;193;119msticky'* ]] \
        || fail "lsd did not apply LS_COLORS Rose Pine sticky directory color"
    [[ "$out" != *$'\033[48;5;2m'* ]] \
        || fail "lsd fell back to the default green background for special directories"
    [[ "$out" == *$'\033[38;2;235;111;146mrun'* ]] \
        || fail "lsd did not apply LS_COLORS Rose Pine executable color"
    [[ "$out" == *$'\033[38;5;222m'* ]] \
        || fail "lsd did not apply colors.yaml Rose Pine metadata colors"
else
    echo "skipped dynamic lsd theme check: lsd not installed"
fi

echo "OK"
