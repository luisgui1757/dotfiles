#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2329
# setup.sh enforces the nix-darwin package layer on macOS by default. Prove:
# default --all invokes the sudo activation shape; dry-run only PREVIEWS (never
# switches); --skip-deps is the explicit already-provisioned escape even when
# paired with the compatibility alias; non-macOS hosts are skipped; Apple
# Silicon selects the only configuration; retired Intel and unsupported
# architectures fail closed; tap migration rolls back transactionally; and first-run bootstrap
# uses the flake.lock-pinned nix-darwin rev + narHash, never a mutable alias.
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
DOTFILES_TARGET_USER=tester
DOTFILES_TARGET_HOME=/Users/tester
export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME
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
    local fake_arch="${1:-ppc64}"
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
uname() { case "\${1:-}" in -m) echo "$fake_arch" ;; *) echo Darwin ;; esac; }
DOTFILES_TARGET_USER=tester
DOTFILES_TARGET_HOME=/Users/tester
export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME
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
sudo() {
    if [[ "\${1:-}" == env ]]; then
        shift
        while [[ "\${1:-}" == *=* ]]; do export "\$1"; shift; done
    fi
    "\$@"
}
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
ALL=1
DRY_RUN=0
DOTFILES_HOMEBREW_LIBRARY="$library"
DOTFILES_TEST_TIMESTAMP=20260708000000
prepare_nix_homebrew_declarative_taps >/dev/null
[[ ! -e "$library/Taps" && -d "$library/Taps.dotfiles-pre-nix-20260708000000.1/homebrew/homebrew-core" ]]
EOF
    } > "$script"
    mkdir -p "$library/Taps.dotfiles-pre-nix-20260708000000"
    bash "$script" 2>&1
}

probe_taps_rollback() {
    local mode="$1" script="$WORK/taps-rollback-$1.sh"
    local library="$WORK/homebrew-rollback-$1/Library"
    rm -rf "${library%/Library}"
    mkdir -p "$library/Taps/homebrew/homebrew-core"
    {
        cat <<EOF
set -uo pipefail
sudo() {
    if [[ "\${1:-}" == env ]]; then
        shift
        while [[ "\${1:-}" == *=* ]]; do export "\$1"; shift; done
    fi
    "\$@"
}
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
ALL=1
DRY_RUN=0
DOTFILES_TARGET_USER=tester
DOTFILES_TARGET_HOME=/Users/tester
DOTFILES_HOMEBREW_LIBRARY="$library"
DOTFILES_TEST_TIMESTAMP=20260708000001
export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME DOTFILES_HOMEBREW_LIBRARY
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
        return 0
    fi
    mkdir -p "$library/Taps/replacement"
    return 1
}
EOF
        if [[ "$mode" == "installed" ]]; then
            cat <<EOF
darwin-rebuild() {
    mkdir -p "$library/Taps/replacement"
    return 1
}
EOF
        fi
        cat <<EOF
run_nix_darwin_switch >/dev/null 2>&1 && rc=0 || rc=\$?
[[ "\$rc" -ne 0 ]] || { echo 'activation unexpectedly succeeded'; exit 1; }
[[ -d "$library/Taps/homebrew/homebrew-core" ]] || { echo 'original taps missing after rollback'; find "$library" -maxdepth 3 -print; exit 1; }
[[ ! -e "$library/Taps.dotfiles-pre-nix-20260708000001" ]] || { echo 'backup was not consumed by rollback'; exit 1; }
[[ -d "$library/Taps.dotfiles-failed-20260708000001/replacement" ]] || { echo 'failed replacement was not quarantined'; find "$library" -maxdepth 3 -print; exit 1; }
EOF
    } > "$script"
    bash "$script" 2>&1
}

probe_taps_signal_rollback() {
    local script="$WORK/taps-signal-rollback.sh"
    local library="$WORK/homebrew-signal/Library"
    rm -rf "${library%/Library}"
    mkdir -p "$library/Taps/homebrew/homebrew-core"
    cat > "$script" <<EOF
set -uo pipefail
sudo() {
    if [[ "\${1:-}" == env ]]; then
        shift
        while [[ "\${1:-}" == *=* ]]; do export "\$1"; shift; done
    fi
    "\$@"
}
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
ALL=1
DRY_RUN=0
DOTFILES_TARGET_USER=tester
DOTFILES_TARGET_HOME=/Users/tester
DOTFILES_HOMEBREW_LIBRARY="$library"
DOTFILES_TEST_TIMESTAMP=20260708000002
export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME DOTFILES_HOMEBREW_LIBRARY
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
    fi
}
darwin-rebuild() {
    mkdir -p "$library/Taps/replacement"
    kill -TERM \$\$
}
run_nix_darwin_switch
EOF
    bash "$script" >/dev/null 2>&1 && return 1
    [[ -d "$library/Taps/homebrew/homebrew-core" ]]
}

probe_homebrew_library() {
    local arch="$1" expected="$2" script
    script="$WORK/homebrew-library-$arch.sh"
    cat > "$script" <<EOF
set -uo pipefail
PATH=/usr/bin:/bin
export PATH
uname() { case "\${1:-}" in -m) echo "$arch" ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" >/dev/null 2>&1
[[ "\$(nix_homebrew_library_dir)" == "$expected" ]]
EOF
    bash "$script"
}

probe_existing_homebrew_repository() {
    local script="$WORK/homebrew-existing-repository.sh"
    local repository="$WORK/Brew Repository"
    mkdir -p "$repository/Library"
    cat > "$script" <<EOF
set -uo pipefail
brew() { [[ "\${1:-}" == --repository ]] && printf '%s\n' "$repository"; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" >/dev/null 2>&1
[[ "\$(nix_homebrew_library_dir)" == "$repository/Library" ]]
EOF
    bash "$script"
}

probe_taps_retry() {
    local script="$WORK/taps-retry.sh" library="$WORK/homebrew-retry/Library"
    rm -rf "${library%/Library}"
    mkdir -p "$library/Taps/homebrew/homebrew-core"
    cat > "$script" <<EOF
set -uo pipefail
sudo() {
    if [[ "\${1:-}" == env ]]; then
        shift
        while [[ "\${1:-}" == *=* ]]; do export "\$1"; shift; done
    fi
    "\$@"
}
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
ALL=1
DRY_RUN=0
DOTFILES_TARGET_USER=tester
DOTFILES_TARGET_HOME=/Users/tester
DOTFILES_HOMEBREW_LIBRARY="$library"
DOTFILES_TEST_TIMESTAMP=20260708000003
export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME DOTFILES_HOMEBREW_LIBRARY
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
    fi
}
attempt=0
darwin-rebuild() {
    attempt=\$((attempt + 1))
    mkdir -p "$library/Taps/replacement"
    [[ "\$attempt" -gt 1 ]]
}
run_nix_darwin_switch >/dev/null 2>&1 && exit 1 || true
[[ -d "$library/Taps/homebrew/homebrew-core" ]]
run_nix_darwin_switch >/dev/null 2>&1
[[ -d "$library/Taps/replacement" ]]
[[ -d "$library/Taps.dotfiles-pre-nix-20260708000003/homebrew/homebrew-core" ]]
EOF
    bash "$script"
}

probe_taps_rollback_failure() {
    local script="$WORK/taps-rollback-failure.sh" library="$WORK/homebrew-rollback-failure/Library"
    rm -rf "${library%/Library}"
    mkdir -p "$library/Taps/homebrew/homebrew-core"
    cat > "$script" <<EOF
set -uo pipefail
sudo() {
    if [[ "\${1:-}" == env ]]; then
        shift
        while [[ "\${1:-}" == *=* ]]; do export "\$1"; shift; done
    fi
    if [[ "\${1:-}" == mv && "\${2:-}" == *Taps.dotfiles-pre-nix-* && "\${3:-}" == "$library/Taps" ]]; then
        return 1
    fi
    "\$@"
}
uname() { case "\${1:-}" in -m) echo arm64 ;; *) echo Darwin ;; esac; }
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
ALL=1
DRY_RUN=0
DOTFILES_TARGET_USER=tester
DOTFILES_TARGET_HOME=/Users/tester
DOTFILES_HOMEBREW_LIBRARY="$library"
DOTFILES_TEST_TIMESTAMP=20260708000004
export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME DOTFILES_HOMEBREW_LIBRARY
nix() {
    if [ "\${1:-}" = eval ]; then
        printf '%s\n%s\n' "$LOCKED_NIX_DARWIN_REV" "$LOCKED_NIX_DARWIN_NAR_HASH"
    fi
}
darwin-rebuild() { mkdir -p "$library/Taps/replacement"; return 1; }
output="\$(run_nix_darwin_switch 2>&1)" && rc=0 || rc=\$?
[[ "\$rc" -ne 0 ]]
[[ -d "$library/Taps.dotfiles-pre-nix-20260708000004/homebrew/homebrew-core" ]]
[[ "\$output" == *"Restore it manually with:"* ]]
EOF
    bash "$script"
}

probe_shell_rc_migration() {
    local mode="$1" script="$WORK/shell-rc-$1.sh" etc_dir="$WORK/etc-$1"
    rm -rf "$etc_dir"
    mkdir -p "$etc_dir"
    printf '%s\n' original-bash > "$etc_dir/bashrc"
    printf '%s\n' original-zsh > "$etc_dir/zshrc"
    if [[ "$mode" == "collision" ]]; then
        printf '%s\n' older-backup > "$etc_dir/bashrc.before-nix-darwin"
    fi
    cat > "$script" <<EOF
set -uo pipefail
DOTFILES_SETUP_SOURCE_ONLY=1 source "$REPO_ROOT/setup.sh" --all >/dev/null 2>&1
DOTFILES_TEST_NIX_DARWIN_ETC_DIR="$etc_dir"
DOTFILES_TEST_TIMESTAMP=20260710000000
export DOTFILES_TEST_NIX_DARWIN_ETC_DIR DOTFILES_TEST_TIMESTAMP
sudo() {
    if [[ "$mode" == "partial-move" && "\${1:-}" == mv && "\${2:-}" == "$etc_dir/zshrc" ]]; then
        return 1
    fi
    command "\$@"
}
case "$mode" in
    success)
        prepare_nix_darwin_shell_rc_migration
        printf '%s\n' managed-bash > "$etc_dir/bashrc"
        printf '%s\n' managed-zsh > "$etc_dir/zshrc"
        complete_nix_darwin_shell_rc_migration
        [[ "\$(cat "$etc_dir/bashrc.before-nix-darwin")" == original-bash ]]
        [[ "\$(cat "$etc_dir/zshrc.before-nix-darwin")" == original-zsh ]]
        [[ "\$(cat "$etc_dir/bashrc")" == managed-bash ]]
        [[ "\$(cat "$etc_dir/zshrc")" == managed-zsh ]]
        ;;
    rollback)
        prepare_nix_darwin_shell_rc_migration
        printf '%s\n' managed-bash > "$etc_dir/bashrc"
        printf '%s\n' managed-zsh > "$etc_dir/zshrc"
        rollback_nix_darwin_shell_rc_migration
        [[ "\$(cat "$etc_dir/bashrc")" == original-bash ]]
        [[ "\$(cat "$etc_dir/zshrc")" == original-zsh ]]
        [[ "\$(cat "$etc_dir/bashrc.dotfiles-failed-20260710000000")" == managed-bash ]]
        [[ "\$(cat "$etc_dir/zshrc.dotfiles-failed-20260710000000")" == managed-zsh ]]
        [[ ! -e "$etc_dir/bashrc.before-nix-darwin" ]]
        [[ ! -e "$etc_dir/zshrc.before-nix-darwin" ]]
        ;;
    collision)
        output="\$(prepare_nix_darwin_shell_rc_migration 2>&1)" && exit 1
        [[ "\$(cat "$etc_dir/bashrc")" == original-bash ]]
        [[ "\$(cat "$etc_dir/bashrc.before-nix-darwin")" == older-backup ]]
        [[ "\$(cat "$etc_dir/zshrc")" == original-zsh ]]
        [[ "\$output" == *"neither was changed"* ]]
        ;;
    partial-move)
        prepare_nix_darwin_shell_rc_migration >/dev/null 2>&1 && exit 1
        [[ "\$(cat "$etc_dir/bashrc")" == original-bash ]]
        [[ "\$(cat "$etc_dir/zshrc")" == original-zsh ]]
        [[ ! -e "$etc_dir/bashrc.before-nix-darwin" ]]
        [[ ! -e "$etc_dir/zshrc.before-nix-darwin" ]]
        ;;
    signal)
        prepare_nix_darwin_shell_rc_migration >/dev/null
        printf '%s\n' managed-bash > "$etc_dir/bashrc"
        printf '%s\n' managed-zsh > "$etc_dir/zshrc"
        nix_homebrew_activation_interrupted TERM
        ;;
esac
EOF
    if [[ "$mode" == "signal" ]]; then
        bash "$script" >/dev/null 2>&1 && return 1
        [[ "$(cat "$etc_dir/bashrc")" == original-bash ]] &&
            [[ "$(cat "$etc_dir/zshrc")" == original-zsh ]] &&
            [[ "$(cat "$etc_dir/bashrc.dotfiles-failed-20260710000000")" == managed-bash ]] &&
            [[ "$(cat "$etc_dir/zshrc.dotfiles-failed-20260710000000")" == managed-zsh ]]
        return
    fi
    bash "$script"
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
    "sudo env DOTFILES_TARGET_USER=tester DOTFILES_TARGET_HOME=/Users/tester darwin-rebuild switch --flake $REPO_ROOT#dotfiles-aarch64 --impure" \
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
    "sudo env DOTFILES_TARGET_USER=tester DOTFILES_TARGET_HOME=/Users/tester darwin-rebuild switch --flake $REPO_ROOT#dotfiles-aarch64 --impure" \
    "$(probe '--all --nix-darwin' Darwin installed)"
assert_eq "GitHub-hosted macOS activation passes the cleanup-check override through sudo" \
    "sudo env DOTFILES_TARGET_USER=tester DOTFILES_TARGET_HOME=/Users/tester DOTFILES_NIX_DARWIN_HOSTED_CI=1 darwin-rebuild switch --flake $REPO_ROOT#dotfiles-aarch64 --impure" \
    "$(probe '--all' Darwin installed arm64 1)"
assert_eq "cleanup override is limited to GitHub-hosted macOS runners" \
    override-on "$(cleanup_override_probe true github-hosted macOS)"
assert_eq "self-hosted macOS runners keep Homebrew cleanup = check" \
    override-off "$(cleanup_override_probe true self-hosted macOS)"
assert_eq "GitHub-hosted non-macOS runners do not request the darwin cleanup override" \
    override-off "$(cleanup_override_probe true github-hosted Linux)"
assert_eq "default bootstrap uses locked nix-darwin rev" \
    "sudo env DOTFILES_TARGET_USER=tester DOTFILES_TARGET_HOME=/Users/tester nix run $LOCKED_NIX_DARWIN_REF -- switch --flake $REPO_ROOT#dotfiles-aarch64 --impure" \
    "$(probe '--all' Darwin bootstrap)"
assert_eq "GitHub-hosted macOS bootstrap passes the cleanup-check override through sudo" \
    "sudo env DOTFILES_TARGET_USER=tester DOTFILES_TARGET_HOME=/Users/tester DOTFILES_NIX_DARWIN_HOSTED_CI=1 nix run $LOCKED_NIX_DARWIN_REF -- switch --flake $REPO_ROOT#dotfiles-aarch64 --impure" \
    "$(probe '--all' Darwin bootstrap arm64 1)"
retired_intel_output="$(probe_unsupported_darwin_arch x86_64)" && retired_intel_rc=0 || retired_intel_rc=$?
if [[ "$retired_intel_rc" -ne 0 ]] && [[ "$retired_intel_output" == *"FAIL: Intel macOS support is retired; this repo supports Apple Silicon only."* ]] && [[ "$retired_intel_output" == *"migrate the host before rerunning setup"* ]]; then
    echo "ok  : retired Intel macOS fails closed with migration guidance"
else
    echo "FAIL: retired Intel macOS did not fail closed with migration guidance"
    printf '%s\n' "$retired_intel_output"
    fail=1
fi

missing_nix_output="$(probe_missing_nix)" && missing_nix_rc=0 || missing_nix_rc=$?
if [[ "$missing_nix_rc" -ne 0 ]] && [[ "$missing_nix_output" == *"FAIL: Nix is required for macOS setup"* ]]; then
    echo "ok  : macOS setup fails closed when Nix is missing"
else
    echo "FAIL: macOS setup did not fail closed when Nix was missing"
    printf '%s\n' "$missing_nix_output"
    fail=1
fi

unsupported_arch_output="$(probe_unsupported_darwin_arch ppc64)" && unsupported_arch_rc=0 || unsupported_arch_rc=$?
if [[ "$unsupported_arch_rc" -ne 0 ]] && [[ "$unsupported_arch_output" == *"FAIL: no supported nix-darwin activation config for arch ppc64"* ]]; then
    echo "ok  : unsupported macOS architecture fails closed before activation"
else
    echo "FAIL: unsupported macOS architecture did not fail closed"
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
    echo "ok  : tap backup collision selects a distinct timestamp suffix"
else
    echo "FAIL: existing Homebrew taps were not migrated before nix-homebrew activation"
    fail=1
fi
if probe_taps_rollback installed; then
    echo "ok  : installed darwin-rebuild activation failure restores original taps"
else
    echo "FAIL: installed darwin-rebuild activation failure did not restore original taps"
    fail=1
fi
if probe_taps_rollback bootstrap; then
    echo "ok  : first-bootstrap activation failure restores original taps"
else
    echo "FAIL: first-bootstrap activation failure did not restore original taps"
    fail=1
fi
if probe_taps_signal_rollback; then
    echo "ok  : interrupted activation restores original taps"
else
    echo "FAIL: interrupted activation did not restore original taps"
    fail=1
fi
if probe_homebrew_library arm64 /opt/homebrew/Library; then
    echo "ok  : default Homebrew library path is Apple Silicon native"
else
    echo "FAIL: default Homebrew library path is not Apple Silicon native"
    fail=1
fi
if probe_existing_homebrew_repository; then
    echo "ok  : existing Homebrew repository path with spaces drives Library discovery"
else
    echo "FAIL: existing Homebrew repository was ignored"
    fail=1
fi
if probe_taps_retry; then
    echo "ok  : activation retry succeeds after transactional rollback"
else
    echo "FAIL: activation retry did not recover cleanly"
    fail=1
fi
if probe_taps_rollback_failure; then
    echo "ok  : rollback failure preserves backup and emits exact recovery"
else
    echo "FAIL: rollback failure was not explicit or did not preserve backup"
    fail=1
fi
if probe_shell_rc_migration success; then
    echo "ok  : first bootstrap preserves both system shell files for nix-darwin"
else
    echo "FAIL: first-bootstrap shell-file migration did not preserve originals"
    fail=1
fi
if probe_shell_rc_migration rollback; then
    echo "ok  : failed first bootstrap restores both system shell files and quarantines output"
else
    echo "FAIL: failed first bootstrap did not roll system shell files back transactionally"
    fail=1
fi
if probe_shell_rc_migration collision; then
    echo "ok  : pre-existing nix-darwin backup collision fails before either shell file moves"
else
    echo "FAIL: nix-darwin shell-file backup collision was destructive or ambiguous"
    fail=1
fi
if probe_shell_rc_migration partial-move; then
    echo "ok  : partial shell-file migration failure restores the first move"
else
    echo "FAIL: partial nix-darwin shell-file migration was not rolled back"
    fail=1
fi
if probe_shell_rc_migration signal; then
    echo "ok  : interrupted first bootstrap restores both system shell files"
else
    echo "FAIL: interrupted first bootstrap did not restore system shell files"
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
if [[ "$dry_output" == *"sudo env DOTFILES_TARGET_USER="* ]] &&
    [[ "$dry_output" == *"darwin-rebuild switch --flake $REPO_ROOT#dotfiles-aarch64 --impure"* ]] &&
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
