#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2329
# setup.sh enforces the Home Manager package layer on Linux/WSL by default.
# Prove: default --all invokes the switch; dry-run only PREVIEWS; --skip-deps is
# the explicit already-provisioned escape even when paired with the compatibility
# alias; macOS is skipped; and first-run bootstrap uses the flake.lock-pinned
# Home Manager rev + narHash, never the mutable registry alias.
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

probe_missing_nix() {
    local script="$WORK/missing-nix.sh"
    {
        cat <<EOF
set -uo pipefail
PATH="/usr/bin:/bin"
export PATH
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo Linux ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
run_home_manager_switch
EOF
    } > "$script"
    bash "$script" 2>&1
}

probe_dry_run_missing_nix() {
    local script="$WORK/dry-run-missing-nix.sh"
    {
        cat <<EOF
set -uo pipefail
PATH="/usr/bin:/bin"
export PATH
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo Linux ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run >/dev/null 2>&1
run_home_manager_switch
EOF
    } > "$script"
    bash "$script" 2>&1
}

probe_decline() {
    local script="$WORK/decline.sh"
    {
        cat <<EOF
set -uo pipefail
PATH="/usr/bin:/bin"
export PATH
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "$LOCKED_HOME_MANAGER_REV" "$LOCKED_HOME_MANAGER_NAR_HASH"
        return 0
    fi
    return 0
}
home-manager() { return 0; }
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo Linux ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" >/dev/null 2>&1
ALL=0
run_home_manager_switch <<<"n"
EOF
    } > "$script"
    bash "$script" 2>&1
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

assert_eq "default flow (--all) applies Home Manager on Linux" \
    "home-manager switch --flake $REPO_ROOT#x86_64-linux --impure" \
    "$(probe '--all' Linux installed)"
assert_eq "default dry-run only previews (no switch)" \
    NOCALL "$(probe '--all --dry-run' Linux)"
assert_eq "--skip-deps skips the default Nix layer for already-provisioned hosts" \
    NOCALL "$(probe '--all --skip-deps' Linux)"
assert_eq "--skip-deps still wins when paired with --home-manager" \
    NOCALL "$(probe '--all --skip-deps --home-manager' Linux)"
assert_eq "--home-manager on macOS is skipped (use --nix-darwin)" \
    NOCALL "$(probe '--all --home-manager' Darwin)"
assert_eq "--home-manager compatibility alias still invokes home-manager switch" \
    "home-manager switch --flake $REPO_ROOT#x86_64-linux --impure" \
    "$(probe '--all --home-manager' Linux installed)"
assert_eq "default bootstrap uses locked Home Manager rev+narHash" \
    "nix run $LOCKED_HOME_MANAGER_REF -- switch --flake $REPO_ROOT#x86_64-linux --impure" \
    "$(probe '--all' Linux bootstrap)"

guarded_home="$WORK/guarded-linux-home"
mkdir -p "$guarded_home/.nix-profile/bin"
cat > "$guarded_home/.nix-profile/bin/nix" <<EOF
#!/usr/bin/env bash
case "\${1:-}" in
    --version) echo 'nix (guarded Linux profile fixture) 2.34.0' ;;
    store) [[ "\${2:-}" == info ]] ;;
    eval) printf '%s\n%s\n' '$LOCKED_HOME_MANAGER_REV' '$LOCKED_HOME_MANAGER_NAR_HASH' ;;
    *) exit 93 ;;
esac
EOF
chmod +x "$guarded_home/.nix-profile/bin/nix"
if guarded_output="$({
        HOME="$guarded_home"
        # shellcheck disable=SC2030  # command substitution intentionally isolates this fixture PATH
        PATH="/usr/bin:/bin"
        __ETC_PROFILE_NIX_SOURCED=1
        export HOME PATH __ETC_PROFILE_NIX_SOURCED
        DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run >/dev/null
        # Bash 3.2 misparses a compact case statement here while parsing the
        # surrounding command substitution. Keep this form portable to macOS.
        uname() { if [[ "${1:-}" == "-m" ]]; then echo x86_64; else echo Linux; fi; }
        activate_nix_profile
        ensure_nix_prerequisite
        printf 'nix=%s planned=%s\n' "$(command -v nix)" "$NIX_PREREQUISITE_DRY_RUN_PLANNED"
        run_home_manager_switch
    })"; then
    guarded_rc=0
else
    guarded_rc=$?
fi
if [[ "$guarded_rc" -eq 0 ]] && [[ "$guarded_output" == *" planned=0"* ]] &&
    { [[ "$guarded_output" == *"nix=/nix/var/nix/profiles/default/bin/nix"* ]] ||
      [[ "$guarded_output" == *"nix=$guarded_home/.nix-profile/bin/nix"* ]]; } &&
    [[ "$guarded_output" == *"home-manager switch --flake $REPO_ROOT#x86_64-linux --impure"* ]]; then
    echo "ok  : guarded stale PATH recovers the Linux profile without prerequisite reinstall"
else
    echo "FAIL: Linux guarded-profile recovery did not reach Home Manager"
    printf '%s\n' "$guarded_output"
    fail=1
fi

missing_nix_output="$(probe_missing_nix)" && missing_nix_rc=0 || missing_nix_rc=$?
if [[ "$missing_nix_rc" -ne 0 ]] && [[ "$missing_nix_output" == *"FAIL: Nix is required for Linux/WSL setup"* ]]; then
    echo "ok  : Linux/WSL setup fails closed when Nix is missing"
else
    echo "FAIL: Linux/WSL setup did not fail closed when Nix was missing"
    printf '%s\n' "$missing_nix_output"
    fail=1
fi

dry_run_missing_nix_output="$(probe_dry_run_missing_nix)" && dry_run_missing_nix_rc=0 || dry_run_missing_nix_rc=$?
if [[ "$dry_run_missing_nix_rc" -eq 0 ]] && [[ "$dry_run_missing_nix_output" == *"would fail: Nix is required for Linux/WSL setup"* ]]; then
    echo "ok  : Linux/WSL dry-run previews missing-Nix failure without aborting"
else
    echo "FAIL: Linux/WSL dry-run without Nix did not preview cleanly"
    printf '%s\n' "$dry_run_missing_nix_output"
    fail=1
fi

decline_output="$(probe_decline)" && decline_rc=0 || decline_rc=$?
if [[ "$decline_rc" -ne 0 ]] && [[ "$decline_output" == *"FAIL: Linux/WSL setup requires Home Manager"* ]]; then
    echo "ok  : interactive decline fails closed on Linux/WSL"
else
    echo "FAIL: Linux/WSL interactive decline did not fail closed"
    printf '%s\n' "$decline_output"
    fail=1
fi

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
# shellcheck disable=SC2031  # the guarded-profile fixture intentionally left the outer PATH unchanged
old_path="$PATH"
PATH="$dry_bin:/usr/bin:/bin"
export PATH
dry_output="$(
    DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run >/dev/null 2>&1
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
    echo "ok  : setup.sh dispatches the required Home Manager function"
else
    echo "FAIL: setup.sh no longer dispatches run_home_manager_switch"
    fail=1
fi
dispatch_line="$(grep -nE '^[[:space:]]*run_home_manager_switch[[:space:]]*$' "$REPO_ROOT/setup.sh" | cut -d: -f1 | head -n1)"
phase1_line="$(grep -nE 'Phase 1/6: install dependencies' "$REPO_ROOT/setup.sh" | cut -d: -f1 | head -n1)"
if [[ -n "$dispatch_line" && -n "$phase1_line" && "$dispatch_line" -lt "$phase1_line" ]]; then
    echo "ok  : Home Manager dispatch precedes Phase 1 dependency installation"
else
    echo "FAIL: Home Manager dispatch no longer precedes Phase 1"
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
