#!/usr/bin/env bash
# setup.sh enforces the nix-darwin package layer on macOS by default. Prove:
# default --all invokes the sudo activation shape; dry-run only PREVIEWS (never
# switches); --skip-deps is the explicit already-provisioned escape even when
# paired with the compatibility alias; non-macOS hosts are skipped; unsupported
# Intel macOS fails closed; and first-run bootstrap uses the flake.lock-pinned
# nix-darwin rev + narHash, never the mutable registry alias.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/nix-darwin-setup-test"
rm -rf "$WORK"
mkdir -p "$WORK"
trap 'rm -rf "$WORK"' EXIT
read -r LOCKED_NIX_DARWIN_REV LOCKED_NIX_DARWIN_NAR_HASH LOCKED_NIX_DARWIN_NAR_HASH_ENCODED < <(
    python3 - <<'PY' "$REPO_ROOT/flake.lock"
import json
import sys
import urllib.parse

locked = json.load(open(sys.argv[1], encoding="utf-8"))["nodes"]["nix-darwin"]["locked"]
nar_hash = locked["narHash"]
print(locked["rev"], nar_hash, urllib.parse.quote(nar_hash, safe="-._~"))
PY
)
LOCKED_NIX_DARWIN_REF="github:nix-darwin/nix-darwin/$LOCKED_NIX_DARWIN_REV?narHash=$LOCKED_NIX_DARWIN_NAR_HASH_ENCODED#darwin-rebuild"

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
    local setup_args="$1" fake_os="$2" mode="${3:-installed}" fake_arch="${4:-arm64}" github_actions="${5:-0}"
    local script="$WORK/probe.sh"
    : > "$WORK/calls"
    {
        cat <<EOF
set -uo pipefail
CALLS="$WORK/calls"
LOCKED_NIX_DARWIN_REV="$LOCKED_NIX_DARWIN_REV"
PATH="/usr/bin:/bin"
export PATH
DOTFILES_HOMEBREW_LIBRARY="$WORK/probe-homebrew/Library"
mkdir -p "\$DOTFILES_HOMEBREW_LIBRARY"
export DOTFILES_HOMEBREW_LIBRARY
if [ "$github_actions" = "1" ]; then
    export DOTFILES_TEST_GITHUB_ACTIONS=1
fi
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "\$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
        return 0
    fi
    [ "\${1:-}" = run ] && echo "nix run \$*" >> "\$CALLS"
    return 0
}
sudo() { echo "sudo \$*" >> "\$CALLS"; return 0; }
uname() { case "\${1:-}" in -m) echo "$fake_arch" ;; *) echo "$fake_os" ;; esac; }
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

cleanup_override_probe() {
    local github_actions="$1" runner_environment="$2" runner_os="$3" test_override="${4:-0}"
    local script="$WORK/cleanup-override-probe.sh"
    cat > "$script" <<EOF
set -uo pipefail
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
unset DOTFILES_SETUP_SOURCE_ONLY_ACTIVE
export GITHUB_ACTIONS="$github_actions"
export RUNNER_ENVIRONMENT="$runner_environment"
export RUNNER_OS="$runner_os"
if [ "$test_override" = "1" ]; then
    export DOTFILES_TEST_GITHUB_ACTIONS=1
else
    unset DOTFILES_TEST_GITHUB_ACTIONS
fi
if nix_darwin_hosted_ci_cleanup_override; then
    echo override-on
else
    echo override-off
fi
EOF
    bash "$script"
}

probe_missing_nix() {
    local script="$WORK/missing-nix.sh"
    {
        cat <<EOF
set -uo pipefail
PATH="/usr/bin:/bin"
export PATH
sudo() { return 0; }
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
run_nix_darwin_switch
EOF
    } > "$script"
    bash "$script" 2>&1
}

probe_unsupported_darwin_arch() {
    local script="$WORK/unsupported-darwin-arch.sh"
    {
        cat <<EOF
set -uo pipefail
PATH="/usr/bin:/bin"
export PATH
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
        return 0
    fi
    return 0
}
sudo() { return 0; }
darwin-rebuild() { return 0; }
uname() { case "\${1:-}" in -m) echo x86_64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
run_nix_darwin_switch
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
sudo() { return 0; }
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run >/dev/null 2>&1
run_nix_darwin_switch
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
        printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
        return 0
    fi
    return 0
}
sudo() { return 0; }
darwin-rebuild() { return 0; }
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" >/dev/null 2>&1
ALL=0
run_nix_darwin_switch <<<"n"
EOF
    } > "$script"
    bash "$script" 2>&1
}

probe_taps_migration() {
    local script="$WORK/taps-migration.sh"
    local library="$WORK/homebrew/Library"
    rm -rf "$WORK/homebrew"
    mkdir -p "$library/Taps/homebrew/homebrew-core"
    {
        cat <<EOF
set -uo pipefail
sudo() { command "\$@"; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
ALL=1
DRY_RUN=0
DOTFILES_HOMEBREW_LIBRARY="$library"
DOTFILES_TEST_TIMESTAMP=20260708000000
prepare_nix_homebrew_declarative_taps >/dev/null
[[ ! -e "$library/Taps" && -d "$library/Taps.dotfiles-pre-nix-20260708000000/homebrew/homebrew-core" ]]
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

assert_eq "default flow (--all) applies nix-darwin on macOS" \
    "sudo darwin-rebuild switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all' Darwin installed)"
assert_eq "default dry-run only previews (no switch)" \
    NOCALL "$(probe '--all --dry-run' Darwin)"
assert_eq "--skip-deps skips the default Nix layer for already-provisioned hosts" \
    NOCALL "$(probe '--all --skip-deps' Darwin)"
assert_eq "--skip-deps still wins when paired with --nix-darwin" \
    NOCALL "$(probe '--all --skip-deps --nix-darwin' Darwin)"
assert_eq "--nix-darwin on a non-macOS host is skipped" \
    NOCALL "$(probe '--all --nix-darwin' Linux)"
assert_eq "--nix-darwin compatibility alias still invokes sudo darwin-rebuild switch" \
    "sudo darwin-rebuild switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all --nix-darwin' Darwin installed)"
assert_eq "GitHub-hosted macOS activation passes the cleanup-check override through sudo" \
    "sudo env DOTFILES_NIX_DARWIN_HOSTED_CI=1 darwin-rebuild switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all' Darwin installed arm64 1)"
assert_eq "cleanup override is limited to GitHub-hosted macOS runners" \
    override-on "$(cleanup_override_probe true github-hosted macOS)"
assert_eq "self-hosted macOS runners keep Homebrew cleanup = check" \
    override-off "$(cleanup_override_probe true self-hosted macOS)"
assert_eq "GitHub-hosted non-macOS runners do not request the darwin cleanup override" \
    override-off "$(cleanup_override_probe true github-hosted Linux)"
assert_eq "default bootstrap uses locked nix-darwin rev" \
    "sudo nix run $LOCKED_NIX_DARWIN_REF -- switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all' Darwin bootstrap)"
assert_eq "GitHub-hosted macOS bootstrap passes the cleanup-check override through sudo" \
    "sudo env DOTFILES_NIX_DARWIN_HOSTED_CI=1 nix run $LOCKED_NIX_DARWIN_REF -- switch --flake $REPO_ROOT#dotfiles --impure" \
    "$(probe '--all' Darwin bootstrap arm64 1)"
assert_eq "Intel macOS activation does not switch" \
    NOCALL "$(probe '--all' Darwin installed x86_64)"

missing_nix_output="$(probe_missing_nix)" && missing_nix_rc=0 || missing_nix_rc=$?
if [[ "$missing_nix_rc" -ne 0 ]] && [[ "$missing_nix_output" == *"FAIL: Nix is required for macOS setup"* ]]; then
    echo "ok  : macOS setup fails closed when Nix is missing"
else
    echo "FAIL: macOS setup did not fail closed when Nix was missing"
    printf '%s\n' "$missing_nix_output"
    fail=1
fi

unsupported_arch_output="$(probe_unsupported_darwin_arch)" && unsupported_arch_rc=0 || unsupported_arch_rc=$?
if [[ "$unsupported_arch_rc" -ne 0 ]] && [[ "$unsupported_arch_output" == *"FAIL: no supported nix-darwin activation config for arch x86_64"* ]]; then
    echo "ok  : Intel macOS setup fails closed before activation"
else
    echo "FAIL: Intel macOS setup did not fail closed"
    printf '%s\n' "$unsupported_arch_output"
    fail=1
fi

dry_run_missing_nix_output="$(probe_dry_run_missing_nix)" && dry_run_missing_nix_rc=0 || dry_run_missing_nix_rc=$?
if [[ "$dry_run_missing_nix_rc" -eq 0 ]] && [[ "$dry_run_missing_nix_output" == *"would fail: Nix is required for macOS setup"* ]]; then
    echo "ok  : macOS dry-run previews missing-Nix failure without aborting"
else
    echo "FAIL: macOS dry-run without Nix did not preview cleanly"
    printf '%s\n' "$dry_run_missing_nix_output"
    fail=1
fi

decline_output="$(probe_decline)" && decline_rc=0 || decline_rc=$?
if [[ "$decline_rc" -ne 0 ]] && [[ "$decline_output" == *"FAIL: macOS setup requires nix-darwin"* ]]; then
    echo "ok  : interactive decline fails closed on macOS"
else
    echo "FAIL: macOS interactive decline did not fail closed"
    printf '%s\n' "$decline_output"
    fail=1
fi

if probe_taps_migration >/dev/null; then
    echo "ok  : existing Homebrew taps move aside before declarative nix-homebrew activation"
else
    echo "FAIL: existing Homebrew taps were not migrated before nix-homebrew activation"
    fail=1
fi

dry_bin="$WORK/dry-bin"
mkdir -p "$dry_bin"
cat > "$dry_bin/nix" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "eval" ]]; then
    printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
    exit 0
fi
exit 0
EOF
chmod +x "$dry_bin/nix"
old_path="$PATH"
PATH="$dry_bin:/usr/bin:/bin"
export PATH
dry_homebrew_library="$WORK/dry-homebrew/Library"
mkdir -p "$dry_homebrew_library"
old_dotfiles_homebrew_library="${DOTFILES_HOMEBREW_LIBRARY-}"
old_dotfiles_homebrew_library_was_set=0
[[ "${DOTFILES_HOMEBREW_LIBRARY+x}" == x ]] && old_dotfiles_homebrew_library_was_set=1
DOTFILES_HOMEBREW_LIBRARY="$dry_homebrew_library"
export DOTFILES_HOMEBREW_LIBRARY
dry_output="$(
    DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all --dry-run >/dev/null 2>&1
    uname() { if [[ "${1:-}" == "-m" ]]; then echo arm64; else echo Darwin; fi; }
    run_nix_darwin_switch
)"
if [[ "$old_dotfiles_homebrew_library_was_set" -eq 1 ]]; then
    DOTFILES_HOMEBREW_LIBRARY="$old_dotfiles_homebrew_library"
    export DOTFILES_HOMEBREW_LIBRARY
else
    unset DOTFILES_HOMEBREW_LIBRARY
fi
PATH="$old_path"
export PATH
if [[ "$dry_output" == *"sudo darwin-rebuild switch --flake $REPO_ROOT#dotfiles --impure"* ]] &&
    [[ "$dry_output" == *"$LOCKED_NIX_DARWIN_REF"* ]] &&
    [[ "$dry_output" != *"nix run nix-darwin"* ]]; then
    echo "ok  : dry-run previews sudo activation and locked bootstrap ref with narHash"
else
    echo "FAIL: dry-run output did not show sudo activation with locked bootstrap rev+narHash ref"
    printf '%s\n' "$dry_output"
    fail=1
fi

if grep -Eq '^[[:space:]]*run_nix_darwin_switch[[:space:]]*$' "$REPO_ROOT/setup.sh"; then
    echo "ok  : setup.sh dispatches the required nix-darwin function"
else
    echo "FAIL: setup.sh no longer dispatches run_nix_darwin_switch"
    fail=1
fi
dispatch_line="$(grep -nE '^[[:space:]]*run_nix_darwin_switch[[:space:]]*$' "$REPO_ROOT/setup.sh" | cut -d: -f1 | head -n1)"
phase1_line="$(grep -nE 'Phase 1/6: install dependencies' "$REPO_ROOT/setup.sh" | cut -d: -f1 | head -n1)"
if [[ -n "$dispatch_line" && -n "$phase1_line" && "$dispatch_line" -lt "$phase1_line" ]]; then
    echo "ok  : nix-darwin dispatch precedes Phase 1 dependency installation"
else
    echo "FAIL: nix-darwin dispatch no longer precedes Phase 1"
    fail=1
fi

if enable_nix_path; then
    real_ref="$(
        DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --nix-darwin >/dev/null
        pinned_nix_darwin_run_ref
    )"
    assert_eq "real Nix parser returns locked nix-darwin bootstrap ref" \
        "$LOCKED_NIX_DARWIN_REF" \
        "$real_ref"
else
    echo "ok  : real Nix parser check skipped (nix not installed)"
fi

[[ "$fail" -eq 0 ]] && echo "all setup --nix-darwin behaviors OK"
exit "$fail"
