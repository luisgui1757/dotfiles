#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

date() {
    printf '%s\n' "20260617-120000"
}

if grep -F "\$RANDOM" "$REPO_ROOT/install-deps.sh" >/dev/null; then
    fail "install-deps.sh still has a separate \$RANDOM backup loop"
fi

settings="$WORK/vscode/settings.json"
mkdir -p "$(dirname "$settings")"
printf '// jsonc\n{\n  "editor.fontSize": 14\n}\n' > "$settings"
touch "$settings.bak.20260617-120000"

set_vscode_theme "$settings" >/dev/null
[[ -f "$settings.bak.20260617-120000.1" ]] \
    || fail "VS Code JSONC backup did not use unique_backup_path collision suffix"

mkdir -p "$WORK/bin" "$WORK/home/.local/bin"
export HOME="$WORK/home"
PATH="$WORK/bin:/usr/bin:/bin"
PM=apt
YES_ALL=1
DRY_RUN=0
FDFIND_READY=0

cat > "$WORK/bin/fdfind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
EOF
chmod +x "$WORK/bin/fdfind"

ask() { return 0; }
pm_install() { FDFIND_READY=1; return 0; }
have() {
    case "$1" in
        fd) return 1 ;;
        fdfind) [[ "$FDFIND_READY" -eq 1 ]] ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}
have_any() {
    local b
    for b in "$@"; do
        have "$b" && return 0
    done
    return 1
}

fd_link="$HOME/.local/bin/fd"
printf '%s\n' "user fd binary" > "$fd_link"
touch "$fd_link.bak.20260617-120000"

install fd "Telescope find_files backend" >/dev/null
[[ -f "$fd_link.bak.20260617-120000.1" ]] \
    || fail "fd shim backup did not use unique_backup_path collision suffix"

echo "OK"
