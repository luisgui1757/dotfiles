#!/usr/bin/env bash
# setup.sh --home-manager is an EXPLICIT, opt-in, dry-run-safe, Linux/WSL-only
# step. Prove: the default flow never applies Home Manager (even with --all); a
# --home-manager --dry-run run only PREVIEWS; --home-manager on macOS is skipped;
# --home-manager --all on Linux invokes the switch; and first-run bootstrap uses
# the flake.lock-pinned Home Manager rev + narHash, never the mutable registry alias.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/home-manager-setup-test"
rm -rf "$WORK"
mkdir -p "$WORK"
trap 'rm -rf "$WORK"' EXIT
read -r LOCKED_HOME_MANAGER_REV LOCKED_HOME_MANAGER_NAR_HASH LOCKED_HOME_MANAGER_NAR_HASH_ENCODED < <(
    python3 - <<'PY' "$REPO_ROOT/flake.lock"
import json
import sys
import urllib.parse

locked = json.load(open(sys.argv[1], encoding="utf-8"))["nodes"]["home-manager"]["locked"]
nar_hash = locked["narHash"]
print(locked["rev"], nar_hash, urllib.parse.quote(nar_hash, safe="-._~"))
PY
)
LOCKED_HOME_MANAGER_REF="github:nix-community/home-manager/$LOCKED_HOME_MANAGER_REV?narHash=$LOCKED_HOME_MANAGER_NAR_HASH_ENCODED#home-manager"

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
LOCKED_HOME_MANAGER_REV="$LOCKED_HOME_MANAGER_REV"
PATH="/usr/bin:/bin"
export PATH
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "\$LOCKED_HOME_MANAGER_REV" "$LOCKED_HOME_MANAGER_NAR_HASH"
        return 0
    fi
    [ "\${1:-}" = run ] && echo "nix \$*" >> "\$CALLS"
    return 0
}
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo "$fake_os" ;; esac; }
EOF
        if [[ "$mode" == "installed" ]]; then
            printf '%s\n' "home-manager() { echo \"home-manager \$*\" >> \"\$CALLS\"; }"
        fi
        cat <<EOF
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" $setup_args >/dev/null 2>&1
run_home_manager_switch >/dev/null 2>&1
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

assert_eq "default flow (--all, no --home-manager) never applies Home Manager" \
    NOCALL "$(probe '--all' Linux)"
assert_eq "--home-manager --dry-run only previews (no switch)" \
    NOCALL "$(probe '--all --dry-run --home-manager' Linux)"
assert_eq "--home-manager on macOS is skipped (use --nix-darwin)" \
    NOCALL "$(probe '--all --home-manager' Darwin)"
assert_eq "--home-manager --all on Linux invokes home-manager switch" \
    "home-manager switch --flake $REPO_ROOT#x86_64-linux --impure" \
    "$(probe '--all --home-manager' Linux installed)"
assert_eq "--home-manager bootstrap uses locked Home Manager rev+narHash" \
    "nix run $LOCKED_HOME_MANAGER_REF -- switch --flake $REPO_ROOT#x86_64-linux --impure" \
    "$(probe '--all --home-manager' Linux bootstrap)"

dry_bin="$WORK/dry-bin"
mkdir -p "$dry_bin"
cat > "$dry_bin/nix" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "eval" ]]; then
    printf '%s\n%s\n' "$LOCKED_HOME_MANAGER_REV" "$LOCKED_HOME_MANAGER_NAR_HASH"
    exit 0
fi
exit 0
EOF
chmod +x "$dry_bin/nix"
old_path="$PATH"
PATH="$dry_bin:/usr/bin:/bin"
export PATH
dry_output="$(
    DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run --home-manager >/dev/null 2>&1
    uname() { if [[ "${1:-}" == "-m" ]]; then echo x86_64; else echo Linux; fi; }
    run_home_manager_switch
)"
PATH="$old_path"
export PATH
if [[ "$dry_output" == *"home-manager switch --flake $REPO_ROOT#x86_64-linux --impure"* ]] &&
    [[ "$dry_output" == *"$LOCKED_HOME_MANAGER_REF"* ]] &&
    [[ "$dry_output" != *"nix run home-manager"* ]]; then
    echo "ok  : dry-run previews installed switch and locked bootstrap ref with narHash"
else
    echo "FAIL: dry-run output did not show locked Home Manager bootstrap rev+narHash ref"
    printf '%s\n' "$dry_output"
    fail=1
fi

if grep -Eq '^[[:space:]]*run_home_manager_switch[[:space:]]*$' "$REPO_ROOT/setup.sh"; then
    echo "ok  : setup.sh dispatches the Home Manager opt-in function"
else
    echo "FAIL: setup.sh no longer dispatches run_home_manager_switch"
    fail=1
fi

if enable_nix_path; then
    real_ref="$(
        DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --home-manager >/dev/null
        pinned_home_manager_run_ref
    )"
    assert_eq "real Nix parser returns locked Home Manager bootstrap ref" \
        "$LOCKED_HOME_MANAGER_REF" \
        "$real_ref"
else
    echo "ok  : real Nix parser check skipped (nix not installed)"
fi

[[ "$fail" -eq 0 ]] && echo "all setup --home-manager behaviors OK"
exit "$fail"
