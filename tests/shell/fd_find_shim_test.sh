#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/fd-find-shim-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/home/.local/bin"
trap 'rm -rf "$TMP_ROOT"' EXIT

export HOME="$TMP_ROOT/home"
PATH="$TMP_ROOT/bin:/usr/bin:/bin"
PM=apt
YES_ALL=1
DRY_RUN=0
FDFIND_READY=0

cat > "$TMP_ROOT/bin/fdfind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
EOF
chmod +x "$TMP_ROOT/bin/fdfind"

ask() {
    return 0
}

pm_install() {
    FDFIND_READY=1
    return 0
}

have() {
    case "$1" in
        fd) return 1 ;;
        fdfind) [[ "$FDFIND_READY" -eq 1 ]] ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}

printf '%s\n' "user fd binary" > "$HOME/.local/bin/fd"

output="$(install fd "Telescope find_files backend")"

[[ "$output" == *"backup    fd"* ]]
[[ "$output" == *"set       fd"* ]]
[[ -L "$HOME/.local/bin/fd" ]]
target="$(readlink "$HOME/.local/bin/fd")"
[[ "$target" == "$TMP_ROOT/bin/fdfind" ]]

backup=""
while IFS= read -r candidate; do
    backup="$candidate"
    break
done < <(find "$HOME/.local/bin" -type f -name 'fd.bak.*' -print)

if [[ -z "$backup" ]]; then
    echo "FAIL: pre-existing fd file was not backed up" >&2
    exit 1
fi
grep -F "user fd binary" "$backup" >/dev/null

echo "OK"
