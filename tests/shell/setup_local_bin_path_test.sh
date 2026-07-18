#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
fail() { echo "FAIL: $1" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
export HOME="$WORK/home"
mkdir -p "$HOME/.local/bin"

# Load setup.sh's helper functions without running the install/config/sync
# phases. Unit-testing refresh_runtime_path directly keeps this fixture isolated
# from whichever Homebrew/Nix tools happen to exist on the host.
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"

# F7: a normal refresh must move ~/.local/bin to the front and deduplicate it,
# so freshly installed user-local tools win over stale global copies.
DRY_RUN=0
PATH="/usr/bin:$HOME/.local/bin:/bin:$HOME/.local/bin"
refresh_runtime_path
[[ "${PATH%%:*}" == "$HOME/.local/bin" ]] \
    || fail "refresh_runtime_path did not put ~/.local/bin first: $PATH"
local_count="$(printf '%s\n' "$PATH" | tr ':' '\n' | grep -Fxc "$HOME/.local/bin")"
[[ "$local_count" == "1" ]] \
    || fail "refresh_runtime_path left $local_count ~/.local/bin entries"

# F9: under --dry-run the refresh must be a no-op (the dry-run contract promises
# nothing is changed -- no PATH mutation, no brew shellenv eval, no hash -r).
DRY_RUN=1
PATH="/usr/bin:/bin"
before="$PATH"
refresh_runtime_path
[[ "$PATH" == "$before" ]] || fail "refresh_runtime_path mutated PATH during --dry-run"

mkdir -p "$WORK/bin"
cat > "$WORK/bin/brew" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "shellenv" ]]; then
    printf '%s\n' 'export PATH="/bad-brew-shellenv:$PATH"'
    exit 17
fi
exit 0
EOF
chmod +x "$WORK/bin/brew"

DRY_RUN=0
PATH="$WORK/bin:/usr/bin:/bin"
refresh_runtime_path 2>"$WORK/brew.err"
case ":$PATH:" in
    *":/bad-brew-shellenv:"*) fail "refresh_runtime_path evaled failed brew shellenv output" ;;
esac
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) fail "refresh_runtime_path did not continue after failed brew shellenv" ;;
esac
[[ "${PATH%%:*}" == "$HOME/.local/bin" ]] \
    || fail "refresh_runtime_path did not keep ~/.local/bin first after failed brew shellenv: $PATH"
grep -F "shellenv failed" "$WORK/brew.err" >/dev/null \
    || fail "refresh_runtime_path did not warn about failed brew shellenv"

echo "OK"
