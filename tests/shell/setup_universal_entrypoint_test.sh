#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2031,SC2034,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

state="$({
    DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --upgrade
    printf '%s:%s\n' "$UPDATE_MODE" "$ALL"
})"
[[ "$state" == "1:1" ]] || fail "--upgrade did not alias non-interactive --update"

home="$WORK/home"
fake_repo="$WORK/repo"
fake_bin="$WORK/bin"
mkdir -p "$home/.nix-profile/bin" "$home/.nix-profile/etc/profile.d" "$fake_repo/scripts" "$fake_bin"
cat > "$home/.nix-profile/bin/nix" <<'PROFILE_NIX'
#!/usr/bin/env bash
case "${1:-}" in
    --version) echo 'nix (profile fixture) 2.34.0' ;;
    store) [[ "${2:-}" == info ]] ;;
    *) exit 90 ;;
esac
PROFILE_NIX
chmod +x "$home/.nix-profile/bin/nix"

old_home="$HOME"
old_path="$PATH"

activate_nix_profile() {
    local profile="$HOME/.nix-profile/etc/profile.d/nix.sh"
    [[ -f "$profile" ]] || return 1
    # Fixture boundary: the real helper/profile contract has separate tests.
    # shellcheck disable=SC1090
    source "$profile"
    [[ "$(command -v nix)" == "$fake_bin/nix" ]]
}

# Source setup again inside an isolated environment so this probe exercises the
# production function without replacing the fixture used by the later bootstrap
# cases in the parent shell.
(
    HOME="$home"
    PATH="/usr/bin:/bin"
    __ETC_PROFILE_NIX_SOURCED=1
    export HOME PATH __ETC_PROFILE_NIX_SOURCED
    DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh"
    activate_nix_profile
    case "$(command -v nix)" in
        /nix/var/nix/profiles/default/bin/nix|"$home/.nix-profile/bin/nix") ;;
        *) fail "setup did not recover a canonical Nix profile binary after guarded profile sourcing" ;;
    esac
)

cat > "$fake_repo/scripts/install-nix-prerequisite.sh" <<'HELPER'
#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == "--install" ]]
printf '%s\n' "$*" > "$SETUP_UNIVERSAL_TEST_ROOT/nix-helper.args"
cat > "$HOME/.nix-profile/etc/profile.d/nix.sh" <<EOF
export PATH="$SETUP_UNIVERSAL_TEST_ROOT/bin:\$PATH"
EOF
HELPER
cat > "$fake_bin/nix" <<'NIX'
#!/usr/bin/env bash
case "${1:-}" in
    --version) echo 'nix (fixture) 2.34.0' ;;
    store) [[ "${2:-}" == info ]] ;;
    *) exit 91 ;;
esac
NIX
chmod +x "$fake_repo/scripts/install-nix-prerequisite.sh" "$fake_bin/nix"

HOME="$home"
PATH="/usr/bin:/bin"
SCRIPT_DIR="$fake_repo"
SKIP_DEPS=0
DRY_RUN=1
ALL=1
SETUP_UNIVERSAL_TEST_ROOT="$WORK"
export HOME PATH SETUP_UNIVERSAL_TEST_ROOT
ensure_nix_prerequisite >/dev/null
[[ "$NIX_PREREQUISITE_DRY_RUN_PLANNED" -eq 1 && ! -e "$WORK/nix-helper.args" ]] ||
    fail "fresh dry-run did not preview Nix bootstrap without invoking it"
DRY_RUN=0
NIX_PREREQUISITE_DRY_RUN_PLANNED=0
ensure_nix_prerequisite >/dev/null
[[ "$(< "$WORK/nix-helper.args")" == "--install" ]] ||
    fail "setup did not invoke the verified Nix prerequisite helper"
command -v nix >/dev/null 2>&1 || fail "setup did not activate the installed Nix profile"

PATH="$old_path"
HOME="$old_home"
export PATH HOME

SCRIPT_DIR="$WORK/current-release"
HOME="$WORK/migration-home"
XDG_STATE_HOME="$HOME/.local/state"
ALL=1
DRY_RUN=0
SKIP_DEPS=0
mkdir -p "$SCRIPT_DIR/scripts" "$HOME" "$WORK/old-release" "$WORK/recovery"
cat > "$SCRIPT_DIR/scripts/upgrade-v0.1.0.sh" <<'MIGRATOR'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\t%s\n' "$1" "$2" >> "$SETUP_UNIVERSAL_TEST_ROOT/migration.calls"
case "$1" in
    --apply) printf 'Recovery directory: %s\n' "$SETUP_UNIVERSAL_TEST_ROOT/recovery" ;;
    --accept) echo 'accepted fixture migration' ;;
    *) exit 92 ;;
esac
MIGRATOR
chmod +x "$SCRIPT_DIR/scripts/upgrade-v0.1.0.sh"
v0_1_candidate_from_live_config() {
    printf '%s\n' "$SETUP_UNIVERSAL_TEST_ROOT/old-release"
}
export HOME XDG_STATE_HOME SETUP_UNIVERSAL_TEST_ROOT
maybe_complete_v0_1_upgrade >/dev/null
expected_calls="$(printf '%s\t%s\n%s\t%s' \
    --apply "$WORK/old-release" --accept "$WORK/recovery")"
[[ "$(< "$WORK/migration.calls")" == "$expected_calls" ]] ||
    fail "setup did not apply then accept the detected v0.1.0 migration"
[[ "$COMPLETED_V0_1_RECOVERY" == "$WORK/recovery" ]] ||
    fail "setup did not retain the completed recovery identity"

rm -f "$WORK/migration.calls"
pending="$XDG_STATE_HOME/dotfiles/migrations/v0.1.0-to-v0.2.0.pending"
mkdir -p "$pending"
printf '%s\n' applied > "$pending/stage"
printf '%s\n' "$SCRIPT_DIR" > "$pending/new-checkout"
printf '%s\n' "$WORK/old-release" > "$pending/old-checkout"
v0_1_candidate_from_live_config() { return 1; }
maybe_complete_v0_1_upgrade >/dev/null
[[ "$(< "$WORK/migration.calls")" == "$(printf '%s\t%s' --accept "$pending")" ]] ||
    fail "setup did not resume an applied migration at acceptance"

printf '%s\n' invalid-stage > "$pending/stage"
if maybe_complete_v0_1_upgrade > "$WORK/invalid-recovery.out" 2>&1; then
    fail "setup accepted an invalid pending recovery stage"
fi
grep -F 'migration recovery stage is invalid' "$WORK/invalid-recovery.out" >/dev/null ||
    fail "invalid recovery failure did not identify the stage boundary"

printf '%s' applied > "$pending/stage"
if maybe_complete_v0_1_upgrade > "$WORK/malformed-recovery.out" 2>&1; then
    fail "setup accepted a recovery scalar without its exact newline framing"
fi
grep -F 'migration recovery identity is incomplete or unsafe' "$WORK/malformed-recovery.out" >/dev/null ||
    fail "malformed recovery failure did not identify the recovery boundary"

rm -rf "$pending"
real_recovery="$WORK/real-recovery"
mkdir -p "$real_recovery"
printf '%s\n' applied > "$real_recovery/stage"
printf '%s\n' "$SCRIPT_DIR" > "$real_recovery/new-checkout"
printf '%s\n' "$WORK/old-release" > "$real_recovery/old-checkout"
ln -s "$real_recovery" "$XDG_STATE_HOME/dotfiles/migrations/v0.1.0-to-v0.2.0.symlink"
if maybe_complete_v0_1_upgrade > "$WORK/symlink-recovery.out" 2>&1; then
    fail "setup accepted a symlinked migration recovery directory"
fi
grep -F 'migration recovery path is not a real directory' "$WORK/symlink-recovery.out" >/dev/null ||
    fail "symlinked recovery failure did not identify the directory boundary"

sentinel_line="$(grep -nE '^[[:space:]]*run_sentinel_agent_policy[[:space:]]*$' "$REPO_ROOT/setup.sh" | tail -n1 | cut -d: -f1)"
update_line="$(grep -nE '^[[:space:]]*run_update_mode[[:space:]]*$' "$REPO_ROOT/setup.sh" | tail -n1 | cut -d: -f1)"
phase1_line="$(grep -n 'Phase 1/6: install dependencies' "$REPO_ROOT/setup.sh" | tail -n1 | cut -d: -f1)"
ensure_nix_line="$(grep -nE '^[[:space:]]*ensure_nix_prerequisite[[:space:]]*$' "$REPO_ROOT/setup.sh" | tail -n1 | cut -d: -f1)"
migration_line="$(grep -nE '^[[:space:]]*maybe_complete_v0_1_upgrade[[:space:]]*$' "$REPO_ROOT/setup.sh" | tail -n1 | cut -d: -f1)"
[[ -n "$phase1_line" && -n "$sentinel_line" && -n "$update_line" &&
    "$phase1_line" -lt "$update_line" && "$sentinel_line" -lt "$update_line" ]] ||
    fail "--update does not reconcile the full release before the scoped refresh"
[[ -n "$ensure_nix_line" && -n "$migration_line" &&
    "$ensure_nix_line" -lt "$phase1_line" && "$migration_line" -lt "$phase1_line" ]] ||
    fail "setup does not bootstrap prerequisites and migrate before package/config publication"

echo "OK"
