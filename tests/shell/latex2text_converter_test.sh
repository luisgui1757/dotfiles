#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
HOME="$TMP_ROOT/home"
export HOME
mkdir -p "$HOME"
PYTHON_LOG="$TMP_ROOT/python.log"

python3() {
    if [[ "${1:-}" == "-m" && "${2:-}" == "venv" ]]; then
        local venv="$3"
        mkdir -p "$venv/bin"
        cat > "$venv/bin/python" <<EOF
#!/usr/bin/env sh
printf '%s\n' "\$*" >> "$PYTHON_LOG"
exit 0
EOF
        cat > "$venv/bin/latex2text" <<'EOF'
#!/usr/bin/env sh
cat
EOF
        chmod 0755 "$venv/bin/python" "$venv/bin/latex2text"
        return 0
    fi
    return 1
}

PM=apt
YES_ALL=1
DRY_RUN=1

out="$(install_pylatexenc_converter)"
[[ "$out" == *"python3 -m venv $HOME/.local/share/dotfiles/python-tools/pylatexenc"* ]] \
    || fail "dry-run did not preview the dotfiles-owned pylatexenc venv"
[[ "$out" == *"pylatexenc==$PYLATEXENC_VERSION"* ]] \
    || fail "dry-run did not pin pylatexenc version"
[[ "$out" == *"setuptools==$PYLATEXENC_BUILD_BACKEND_VERSION"* ]] \
    || fail "dry-run did not pin the pylatexenc build backend"
[[ "$out" == *"sha256=$PYLATEXENC_BUILD_BACKEND_SHA256"* ]] \
    || fail "dry-run did not print the pinned build backend hash"
[[ "$out" == *"sha256=$PYLATEXENC_SHA256"* ]] \
    || fail "dry-run did not print the pinned pylatexenc hash"
[[ ! -e "$HOME/.local/bin/latex2text" ]] \
    || fail "dry-run created the latex2text shim"

DRY_RUN=0
install_pylatexenc_converter >/dev/null

shim="$HOME/.local/bin/latex2text"
converter="$HOME/.local/share/dotfiles/python-tools/pylatexenc/bin/latex2text"
[[ -x "$shim" ]] || fail "latex2text shim is not executable"
grep -F "exec \"$converter\" \"\$@\"" "$shim" >/dev/null \
    || fail "latex2text shim does not exec the managed converter"
grep -F -- "--require-hashes" "$PYTHON_LOG" >/dev/null \
    || fail "pylatexenc install did not use pip hash-checking mode"
grep -F -- "--no-build-isolation" "$PYTHON_LOG" >/dev/null \
    || fail "pylatexenc install did not disable build isolation after pinning setuptools"
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) fail "install did not add ~/.local/bin to PATH" ;;
esac

before="$(grep -c -- "-m pip install" "$PYTHON_LOG")"
install_pylatexenc_converter >/dev/null
after="$(grep -c -- "-m pip install" "$PYTHON_LOG")"
[[ "$before" == "$after" ]] || fail "ready converter was reinstalled"

echo "OK"
