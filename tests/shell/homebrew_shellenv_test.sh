#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $1"; exit 1; }

WORK="$REPO_ROOT/tests/.cache/homebrew-shellenv-test"
rm -rf "$WORK"
mkdir -p "$WORK/home/.linuxbrew/bin"
trap 'rm -rf "$WORK"' EXIT

export HOME="$WORK/home"
prefix="$HOME/.linuxbrew"

cat > "$prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "shellenv" ]]; then
    printf 'export HOMEBREW_PREFIX="%s"\n' "$prefix"
    printf 'export PATH="%s/bin:\$PATH"\n' "$prefix"
fi
EOF
chmod +x "$prefix/bin/brew"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

homebrew_bin() { printf '%s\n' "$prefix/bin/brew"; }

PATH="/usr/bin:/bin"
DRY_RUN=0

[[ "$(detect_pm)" == "brew" ]] || fail "detect_pm did not find Linuxbrew outside PATH"

enable_homebrew_for_current_shell
[[ "${HOMEBREW_PREFIX:-}" == "$prefix" ]] || fail "HOMEBREW_PREFIX was not exported"
[[ "$(command -v brew)" == "$prefix/bin/brew" ]] || fail "brew was not added to PATH"

persist_homebrew_shellenv
persist_homebrew_shellenv

for rc in "$HOME/.zshrc.local" "$HOME/.bashrc"; do
    [[ -f "$rc" ]] || fail "$rc was not written"
    count="$(grep -cF '# >>> dotfiles: Homebrew shellenv >>>' "$rc")"
    [[ "$count" == "1" ]] || fail "$rc marker count is $count"
done

# Single quotes are intentional: $HOME / $HOMEBREW_PREFIX must expand inside the
# inner `bash -c`, not in this outer shell.
# shellcheck disable=SC2016
env -i HOME="$HOME" PATH="/usr/bin:/bin" bash -c 'source "$HOME/.bashrc"; [[ "$HOMEBREW_PREFIX" == "$HOME/.linuxbrew" ]]'

echo "OK"
