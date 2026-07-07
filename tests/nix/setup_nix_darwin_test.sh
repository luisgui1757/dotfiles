#!/usr/bin/env bash
# setup.sh --nix-darwin is an EXPLICIT, opt-in, dry-run-safe, macOS-only step.
# Prove: the default flow never touches nix-darwin (even with --all); a
# --nix-darwin --dry-run run only PREVIEWS (never switches); --nix-darwin on a
# non-macOS host is skipped; and --nix-darwin --all on macOS DOES invoke the
# switch. Stubs darwin-rebuild + nix so no real activation happens.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/nix-darwin-setup-test"
rm -rf "$WORK"
mkdir -p "$WORK"
trap 'rm -rf "$WORK"' EXIT

# probe <setup-args> <fake-uname-os> -> echoes CALLED if a switch was attempted.
probe() {
    local setup_args="$1" fake_os="$2"
    local script="$WORK/probe.sh"
    : > "$WORK/calls"
    cat > "$script" <<EOF
set -uo pipefail
CALLS="$WORK/calls"
darwin-rebuild() { echo "darwin-rebuild \$*" >> "\$CALLS"; }
nix() { [ "\${1:-}" = run ] && echo "nix run \$*" >> "\$CALLS"; return 0; }
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo "$fake_os" ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" $setup_args >/dev/null 2>&1
run_nix_darwin_switch >/dev/null 2>&1
EOF
    bash "$script" || true
    if [[ -s "$WORK/calls" ]]; then echo "CALLED"; else echo "NOCALL"; fi
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
assert_eq "--nix-darwin --all on macOS invokes darwin-rebuild switch" \
    CALLED "$(probe '--all --nix-darwin' Darwin)"

[[ "$fail" -eq 0 ]] && echo "all setup --nix-darwin behaviors OK"
exit "$fail"
