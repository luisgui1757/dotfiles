#!/usr/bin/env bash
# setup.sh --home-manager is an EXPLICIT, opt-in, dry-run-safe, Linux/WSL-only
# step. Prove: the default flow never applies Home Manager (even with --all); a
# --home-manager --dry-run run only PREVIEWS; --home-manager on macOS is skipped;
# and --home-manager --all on Linux DOES invoke the switch. Stubs home-manager +
# nix so no real activation happens.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/home-manager-setup-test"
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
home-manager() { echo "home-manager \$*" >> "\$CALLS"; }
nix() { [ "\${1:-}" = run ] && echo "nix run \$*" >> "\$CALLS"; return 0; }
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo "$fake_os" ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" $setup_args >/dev/null 2>&1
run_home_manager_switch >/dev/null 2>&1
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

assert_eq "default flow (--all, no --home-manager) never applies Home Manager" \
    NOCALL "$(probe '--all' Linux)"
assert_eq "--home-manager --dry-run only previews (no switch)" \
    NOCALL "$(probe '--all --dry-run --home-manager' Linux)"
assert_eq "--home-manager on macOS is skipped (use --nix-darwin)" \
    NOCALL "$(probe '--all --home-manager' Darwin)"
assert_eq "--home-manager --all on Linux invokes home-manager switch" \
    CALLED "$(probe '--all --home-manager' Linux)"

[[ "$fail" -eq 0 ]] && echo "all setup --home-manager behaviors OK"
exit "$fail"
