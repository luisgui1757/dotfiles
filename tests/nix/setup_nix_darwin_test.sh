#!/usr/bin/env bash
# setup.sh --nix-darwin is an EXPLICIT, opt-in, dry-run-safe, macOS-only step.
# Prove: the default flow never touches nix-darwin (even with --all); a
# --nix-darwin --dry-run run only PREVIEWS (never switches); --nix-darwin on a
# non-macOS host is skipped; --nix-darwin --all on macOS invokes the sudo
# activation shape; and first-run bootstrap uses the flake.lock-pinned
# nix-darwin rev, never the mutable registry alias.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/nix-darwin-setup-test"
rm -rf "$WORK"
mkdir -p "$WORK"
trap 'rm -rf "$WORK"' EXIT
LOCKED_NIX_DARWIN_REV="$(
    python3 - <<'PY' "$REPO_ROOT/flake.lock"
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["nodes"]["nix-darwin"]["locked"]["rev"])
PY
)"

enable_nix_path() {
    if ! command -v nix >/dev/null 2>&1 && [ -e /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh ]; then
        unset __ETC_PROFILE_NIX_SOURCED
        # shellcheck disable=SC1091
        . /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
    fi
    command -v nix >/dev/null 2>&1
}

# probe <setup-args> <fake-uname-os> <installed|bootstrap> -> echoes attempted commands.
probe() {
    local setup_args="$1" fake_os="$2" mode="${3:-installed}"
    local script="$WORK/probe.sh"
    : > "$WORK/calls"
    {
        cat <<EOF
set -uo pipefail
CALLS="$WORK/calls"
LOCKED_NIX_DARWIN_REV="$LOCKED_NIX_DARWIN_REV"
PATH="/usr/bin:/bin"
export PATH
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n' "\$LOCKED_NIX_DARWIN_REV"
        return 0
    fi
    [ "\${1:-}" = run ] && echo "nix run \$*" >> "\$CALLS"
    return 0
}
sudo() { echo "sudo \$*" >> "\$CALLS"; return 0; }
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo "$fake_os" ;; esac; }
EOF
        if [[ "$mode" == "installed" ]]; then
            printf '%s\n' "darwin-rebuild() { echo \"darwin-rebuild \$*\" >> \"\$CALLS\"; }"
        fi
        cat <<EOF
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" $setup_args >/dev/null 2>&1
run_nix_darwin_switch >/dev/null 2>&1
EOF
    } > "$script"
    bash "$script" || true
    if [[ -s "$WORK/calls" ]]; then cat "$WORK/calls"; else echo "NOCALL"; fi
}

fail=0
assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [[ "$expected" == "$actual" ]]; then
        echo "ok  : $desc"
    else
        echo "FAIL: $desc (expected $expected, got $actual)"
        fail=1
    fi
}

assert_eq "default flow (--all, no --nix-darwin) never switches nix-darwin" \
    NOCALL "$(probe '--all' Darwin)"
assert_eq "--nix-darwin --dry-run only previews (no switch)" \
    NOCALL "$(probe '--all --dry-run --nix-darwin' Darwin)"
assert_eq "--nix-darwin on a non-macOS host is skipped" \
    NOCALL "$(probe '--all --nix-darwin' Linux)"
assert_eq "--nix-darwin --all on macOS invokes sudo darwin-rebuild switch" \
    "sudo darwin-rebuild switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all --nix-darwin' Darwin installed)"
assert_eq "--nix-darwin bootstrap uses locked nix-darwin rev" \
    "sudo nix run github:nix-darwin/nix-darwin/$LOCKED_NIX_DARWIN_REV#darwin-rebuild -- switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all --nix-darwin' Darwin bootstrap)"

dry_bin="$WORK/dry-bin"
mkdir -p "$dry_bin"
cat > "$dry_bin/nix" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "eval" ]]; then
    printf '%s\n' "$LOCKED_NIX_DARWIN_REV"
    exit 0
fi
exit 0
EOF
chmod +x "$dry_bin/nix"
old_path="$PATH"
PATH="$dry_bin:/usr/bin:/bin"
export PATH
dry_output="$(
    DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run --nix-darwin >/dev/null 2>&1
    uname() { if [[ "${1:-}" == "-m" ]]; then echo x86_64; else echo Darwin; fi; }
    run_nix_darwin_switch
)"
PATH="$old_path"
export PATH
if [[ "$dry_output" == *"sudo darwin-rebuild switch --flake $REPO_ROOT#dotfiles --impure"* ]] &&
    [[ "$dry_output" == *"github:nix-darwin/nix-darwin/$LOCKED_NIX_DARWIN_REV#darwin-rebuild"* ]] &&
    [[ "$dry_output" != *"nix run nix-darwin"* ]]; then
    echo "ok  : dry-run previews sudo activation and locked bootstrap ref"
else
    echo "FAIL: dry-run output did not show sudo activation with locked bootstrap ref"
    printf '%s\n' "$dry_output"
    fail=1
fi

if grep -Eq '^[[:space:]]*run_nix_darwin_switch[[:space:]]*$' "$REPO_ROOT/setup.sh"; then
    echo "ok  : setup.sh dispatches the nix-darwin opt-in function"
else
    echo "FAIL: setup.sh no longer dispatches run_nix_darwin_switch"
    fail=1
fi

if enable_nix_path; then
    real_ref="$(
        DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --nix-darwin >/dev/null
        pinned_nix_darwin_run_ref
    )"
    assert_eq "real Nix parser returns locked nix-darwin bootstrap ref" \
        "github:nix-darwin/nix-darwin/$LOCKED_NIX_DARWIN_REV#darwin-rebuild" \
        "$real_ref"
else
    echo "ok  : real Nix parser check skipped (nix not installed)"
fi

[[ "$fail" -eq 0 ]] && echo "all setup --nix-darwin behaviors OK"
exit "$fail"
