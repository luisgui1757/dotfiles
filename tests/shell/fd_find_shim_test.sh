#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$(mktemp -d)"
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

ask() { return 0; }
pm_install() { FDFIND_READY=1; return 0; }

have() {
    case "$1" in
        fd) return 1 ;;
        fdfind) [[ "$FDFIND_READY" -eq 1 ]] ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}
# install()'s "already installed?" check uses have_any (not have), so stub it to
# honor the same fd/fdfind state -- otherwise the fake fdfind on PATH makes
# install short-circuit to "already installed" before reaching the shim logic.
have_any() {
    local b
    for b in "$@"; do
        have "$b" && return 0
    done
    return 1
}

# A pre-existing real fd file that the shim logic must back up, not clobber.
printf '%s\n' "user fd binary" > "$HOME/.local/bin/fd"

output="$(install fd "Telescope find_files backend")"

[[ "$output" == *"backup    fd"* ]] || { echo "FAIL: no backup line in output: $output" >&2; exit 1; }
[[ "$output" == *"set       fd"* ]] || { echo "FAIL: no set line in output: $output" >&2; exit 1; }
[[ -L "$HOME/.local/bin/fd" ]] || { echo "FAIL: fd is not a symlink" >&2; exit 1; }
target="$(readlink "$HOME/.local/bin/fd")"
[[ "$target" == "$TMP_ROOT/bin/fdfind" ]] || { echo "FAIL: fd -> $target, expected $TMP_ROOT/bin/fdfind" >&2; exit 1; }

backup=""
while IFS= read -r candidate; do
    backup="$candidate"
    break
done < <(find "$HOME/.local/bin" -type f -name 'fd.bak.*' -print)

[[ -n "$backup" ]] || { echo "FAIL: pre-existing fd file was not backed up" >&2; exit 1; }
grep -F "user fd binary" "$backup" >/dev/null

echo "OK"
