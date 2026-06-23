#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2016
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

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

for pm in brew apt dnf pacman zypper apk; do
    PM="$pm"
    [[ "$(pkg_for lsd)" == "lsd" ]] || fail "pkg_for lsd failed for $pm"
done

grep -q 'install lsd ' "$REPO_ROOT/install-deps.sh" \
    || fail "install-deps.sh no longer installs lsd"
grep -qF 'lsd|lsd|lsd|lsd|lsd|lsd|lsd' "$REPO_ROOT/install-deps.sh" \
    || fail "PKG_TABLE is missing the cross-platform lsd row"

zshrc="$REPO_ROOT/shells/zshrc"
grep -F 'command -v lsd' "$zshrc" >/dev/null \
    || fail "zshrc lsd aliases are not guarded by command -v"
grep -F "'di=38;2;246;193;119'" "$zshrc" >/dev/null \
    || fail "zshrc does not set Rose Pine directory color in LS_COLORS"
grep -F 'export LS_COLORS="${LS_COLORS:-${(j.:.)_dotfiles_ls_colors}}"' "$zshrc" >/dev/null \
    || fail "zshrc does not default LS_COLORS without overriding user-provided values"
ls_colors_line="$(line_no 'export LS_COLORS="${LS_COLORS:-${(j.:.)_dotfiles_ls_colors}}"' "$zshrc")"
completion_line="$(line_no "zstyle ':completion:*' list-colors" "$zshrc")"
[[ "$ls_colors_line" -lt "$completion_line" ]] \
    || fail "LS_COLORS must be initialized before zsh completion list-colors"
for line in \
    "alias ls='lsd'" \
    "alias l='lsd -l'" \
    "alias la='lsd -a'" \
    "alias lla='lsd -la'" \
    "alias lt='lsd --tree'"; do
    grep -F "$line" "$zshrc" >/dev/null || fail "missing zsh alias: $line"
done

if ! diff -q "$zshrc" "$REPO_ROOT/home/dot_zshrc" >/dev/null; then
    fail "home/dot_zshrc is not byte-identical to shells/zshrc"
fi

for rel in config.yaml colors.yaml; do
    diff -q "$REPO_ROOT/lsd/$rel" "$REPO_ROOT/home/dot_config/lsd/$rel" >/dev/null \
        || fail "home/dot_config/lsd/$rel is not byte-identical to lsd/$rel"
done
grep -F 'theme: custom' "$REPO_ROOT/lsd/config.yaml" >/dev/null \
    || fail "lsd/config.yaml must enable the custom colors.yaml theme"
grep -F 'user: 222' "$REPO_ROOT/lsd/colors.yaml" >/dev/null \
    || fail "lsd/colors.yaml is missing Rose Pine gold user color"
grep -F 'tree-edge: 103' "$REPO_ROOT/lsd/colors.yaml" >/dev/null \
    || fail "lsd/colors.yaml is missing Rose Pine subtle tree-edge color"

if command -v lsd >/dev/null 2>&1; then
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    mkdir -p "$tmp/listing"
    : > "$tmp/listing/plain"
    : > "$tmp/listing/run"
    chmod +x "$tmp/listing/run"
    out="$(env -u NO_COLOR \
        XDG_CONFIG_HOME="$REPO_ROOT/home/dot_config" \
        LS_COLORS='di=38;2;246;193;119:fi=38;2;224;222;244:ex=38;2;235;111;146' \
        lsd --color always -la "$tmp/listing")"
    [[ "$out" == *$'\033[38;2;246;193;119m'* ]] \
        || fail "lsd did not apply LS_COLORS Rose Pine directory color"
    [[ "$out" == *$'\033[38;2;235;111;146mrun'* ]] \
        || fail "lsd did not apply LS_COLORS Rose Pine executable color"
    [[ "$out" == *$'\033[38;5;222m'* ]] \
        || fail "lsd did not apply colors.yaml Rose Pine metadata colors"
else
    echo "skipped dynamic lsd theme check: lsd not installed"
fi

echo "OK"
