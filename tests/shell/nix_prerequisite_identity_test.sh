#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
REAL_GIT="$(command -v git)"
ORIGINAL_PATH="$PATH"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

pass() {
    echo "ok  : $*"
}

cat > "$work/bin/git" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do
    if [[ "$arg" == "ls-remote" ]]; then
        printf 'ls-remote\n' >> "${FAKE_REMOTE_CALL_LOG:?}"
        if [[ "${FAKE_REMOTE_MODE:-ok}" == "fail" ]]; then
            echo "simulated remote transport failure" >&2
            exit 42
        fi
        [[ -f "${FAKE_REMOTE_REFS_FILE:?}" ]] || exit 43
        cat "$FAKE_REMOTE_REFS_FILE"
        exit 0
    fi
done
exec "${REAL_GIT:?}" "$@"
EOF

cat > "$work/bin/nix" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
    --version) echo "nix (Nix) 2.34.0" ;;
    store)
        [[ "${2:-}" == "info" ]]
        if [[ "${FAKE_NIX_FEATURES_DISABLED:-0}" == "1" ]] &&
            ! grep -Fq 'nix-command flakes' "${XDG_CONFIG_HOME:-$HOME/.config}/nix/nix.conf" 2>/dev/null; then
            echo "error: experimental Nix feature 'nix-command' is disabled" >&2
            exit 49
        fi
        ;;
    *) exit 44 ;;
esac
EOF
chmod +x "$work/bin/git" "$work/bin/nix"

new_fixture() {
    local name="$1"
    fixture="$work/$name"
    mkdir -p "$fixture/scripts"
    cp "$REPO_ROOT/scripts/install-nix-prerequisite.sh" "$fixture/scripts/"
    chmod +x "$fixture/scripts/install-nix-prerequisite.sh"
    "$REAL_GIT" -C "$fixture" init -q -b fixture
    printf 'fixture\n' > "$fixture/tracked.txt"
    "$REAL_GIT" -C "$fixture" add .
    "$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
        commit -qm fixture
    "$REAL_GIT" -C "$fixture" remote add origin \
        https://github.com/luisgui1757/dotfiles.git
}

run_helper() {
    local repo="$1" refs_file="$2" mode="${3:-ok}" allow_unreleased="${4:-0}"
    local -a helper_args=(--install)
    local run_path="${RUN_PATH_OVERRIDE:-$work/bin:$ORIGINAL_PATH}"
    local run_home="${RUN_HOME_OVERRIDE:-$HOME}"
    local run_xdg_config_home="${RUN_XDG_CONFIG_HOME_OVERRIDE:-$run_home/.config}"
    [[ "$allow_unreleased" -eq 1 ]] && helper_args+=(--allow-unreleased)
    remote_call_log="$work/remote-calls.log"
    : > "$remote_call_log"
    set +e
    output="$(HOME="$run_home" XDG_CONFIG_HOME="$run_xdg_config_home" PATH="$run_path" \
        REAL_GIT="$REAL_GIT" \
        FAKE_REMOTE_REFS_FILE="$refs_file" \
        FAKE_REMOTE_CALL_LOG="$remote_call_log" \
        FAKE_REMOTE_MODE="$mode" \
        "$repo/scripts/install-nix-prerequisite.sh" "${helper_args[@]}" 2>&1)"
    rc=$?
    set -e
    remote_call_count="$(wc -l < "$remote_call_log" | tr -d '[:space:]')"
}

assert_clean_diagnostic() {
    [[ "$output" != *"fatal:"* ]] || fail "raw Git fatal escaped:\n$output"
}

new_fixture prerelease-attached
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
refs="$work/prerelease-attached.refs"
printf '%s\trefs/heads/fix/bootstrap\n' "$head_commit" > "$refs"
run_helper "$fixture" "$refs"
[[ "$rc" -eq 0 ]] || fail "official prerelease branch head was rejected:\n$output"
[[ "$output" == *"Verified prerelease checkout: refs/heads/fix/bootstrap at $head_commit"* ]] ||
    fail "prerelease success did not report its exact official branch identity"
[[ "$remote_call_count" == "1" ]] || fail "prerelease decision used $remote_call_count remote snapshots"
assert_clean_diagnostic
pass "exact official prerelease branch head is accepted"

new_fixture prerelease-detached
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
"$REAL_GIT" -C "$fixture" checkout -q --detach "$head_commit"
refs="$work/prerelease-detached.refs"
printf '%s\trefs/heads/fix/bootstrap\n' "$head_commit" > "$refs"
run_helper "$fixture" "$refs"
[[ "$rc" -eq 0 ]] || fail "detached exact source head was rejected:\n$output"
assert_clean_diagnostic
pass "detached checkout of an exact official prerelease branch head is accepted"

new_fixture unpublished-head
published_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
printf 'second\n' >> "$fixture/tracked.txt"
"$REAL_GIT" -C "$fixture" add tracked.txt
"$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    commit -qm second
refs="$work/unpublished-head.refs"
printf '%s\trefs/heads/fix/bootstrap\n' "$published_commit" > "$refs"
run_helper "$fixture" "$refs"
[[ "$rc" -ne 0 ]] || fail "unpublished local HEAD was accepted"
[[ "$output" == *"checkout HEAD must be a current official branch head"* ]] ||
    fail "unpublished HEAD failure was not actionable:\n$output"
assert_clean_diagnostic
pass "unpublished or stale local HEAD fails with an explicit diagnostic"

new_fixture remote-failure
refs="$work/remote-failure.refs"
: > "$refs"
run_helper "$fixture" "$refs" fail
[[ "$rc" -ne 0 ]] || fail "remote identity query failure was accepted"
[[ "$output" == *"could not verify release and branch identities from the official repository"* ]] ||
    fail "remote failure was not translated to the reviewed diagnostic:\n$output"
assert_clean_diagnostic
pass "remote identity failure is explicit and does not leak a raw Git fatal"

new_fixture exact-release
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
"$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    tag -a v0.2.0 -m release
tag_object="$($REAL_GIT -C "$fixture" rev-parse refs/tags/v0.2.0)"
refs="$work/exact-release.refs"
{
    printf '%s\trefs/tags/v0.2.0\n' "$tag_object"
    printf '%s\trefs/tags/v0.2.0^{}\n' "$head_commit"
    printf '%s\trefs/heads/main\n' "$head_commit"
} > "$refs"
run_helper "$fixture" "$refs"
[[ "$rc" -eq 0 ]] || fail "exact annotated release was rejected:\n$output"
[[ "$output" == *"Verified immutable release checkout: v0.2.0 at $head_commit"* ]] ||
    fail "release success did not report the immutable identity"
[[ "$remote_call_count" == "1" ]] || fail "release decision used $remote_call_count remote snapshots"
assert_clean_diagnostic
pass "published annotated release accepts only its exact tag object and peeled commit"

run_helper "$fixture" "$refs" ok 1
[[ "$rc" -eq 0 ]] || fail "exact annotated release was rejected with the test override:\n$output"
[[ "$output" == *"Verified immutable release checkout: v0.2.0 at $head_commit"* ]] ||
    fail "test override displaced the immutable release identity"
assert_clean_diagnostic
pass "the explicit test override still prefers an exact immutable release"

new_fixture published-without-local-tag
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
"$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    tag -a v0.2.0 -m release
tag_object="$($REAL_GIT -C "$fixture" rev-parse refs/tags/v0.2.0)"
refs="$work/published-without-local-tag.refs"
{
    printf '%s\trefs/tags/v0.2.0\n' "$tag_object"
    printf '%s\trefs/tags/v0.2.0^{}\n' "$head_commit"
    printf '%s\trefs/heads/main\n' "$head_commit"
} > "$refs"
"$REAL_GIT" -C "$fixture" tag -d v0.2.0 >/dev/null
run_helper "$fixture" "$refs"
[[ "$rc" -ne 0 ]] || fail "official branch fallback remained open after release publication"
[[ "$output" == *"v0.2.0 is published; use a fresh exact-tag checkout"* ]] ||
    fail "published-release transition did not give the exact checkout recovery:\n$output"
assert_clean_diagnostic
pass "release publication closes the prerelease branch-head path"

new_fixture explicitly-authorized-unreleased
release_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
"$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    tag -a v0.2.0 -m release
tag_object="$($REAL_GIT -C "$fixture" rev-parse refs/tags/v0.2.0)"
printf 'branch head\n' >> "$fixture/tracked.txt"
"$REAL_GIT" -C "$fixture" add tracked.txt
"$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    commit -qm branch-head
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
refs="$work/explicitly-authorized-unreleased.refs"
{
    printf '%s\trefs/tags/v0.2.0\n' "$tag_object"
    printf '%s\trefs/tags/v0.2.0^{}\n' "$release_commit"
    printf '%s\trefs/heads/fix/linux-nix-profile-read\n' "$head_commit"
} > "$refs"
run_helper "$fixture" "$refs"
[[ "$rc" -ne 0 ]] || fail "published branch head was accepted without explicit authorization"
[[ "$output" == *"local v0.2.0 does not match the official immutable annotated release"* ]] ||
    fail "default release boundary did not remain closed:\n$output"
run_helper "$fixture" "$refs" ok 1
[[ "$rc" -eq 0 ]] || fail "explicit official branch-head test was rejected:\n$output"
[[ "$output" == *"Verified explicitly authorized unreleased checkout: refs/heads/fix/linux-nix-profile-read at $head_commit"* ]] ||
    fail "unreleased success did not report its exact official branch identity"
[[ "$remote_call_count" == "1" ]] || fail "unreleased decision used $remote_call_count remote snapshots"
assert_clean_diagnostic
pass "published branch head requires and honors the explicit test override"

printf 'local only\n' >> "$fixture/tracked.txt"
"$REAL_GIT" -C "$fixture" add tracked.txt
"$REAL_GIT" -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    commit -qm local-only
run_helper "$fixture" "$refs" ok 1
[[ "$rc" -ne 0 ]] || fail "stale or local-only HEAD was accepted by the test override"
[[ "$output" == *"--allow-unreleased requires checkout HEAD to be a current official branch head"* ]] ||
    fail "stale test checkout failure was not actionable:\n$output"
assert_clean_diagnostic
pass "the explicit test override rejects stale and local-only commits"

new_fixture malformed-release
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
refs="$work/malformed-release.refs"
printf '%s\trefs/tags/v0.2.0\n' "$head_commit" > "$refs"
run_helper "$fixture" "$refs"
[[ "$rc" -ne 0 ]] || fail "lightweight or incomplete official release tag was accepted"
[[ "$output" == *"official v0.2.0 must be one unique annotated tag"* ]] ||
    fail "malformed official tag failure was not explicit:\n$output"
assert_clean_diagnostic
pass "lightweight or incomplete release identity fails closed"

new_fixture dirty-checkout
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
refs="$work/dirty-checkout.refs"
printf '%s\trefs/heads/fix/bootstrap\n' "$head_commit" > "$refs"
printf 'dirty\n' > "$fixture/untracked.txt"
run_helper "$fixture" "$refs"
[[ "$rc" -ne 0 ]] || fail "dirty checkout was accepted"
[[ "$output" == *"checkout has tracked or untracked changes"* ]] ||
    fail "dirty checkout failure was not explicit:\n$output"
[[ "$remote_call_count" == "0" ]] || fail "dirty checkout reached the remote identity query"
assert_clean_diagnostic
pass "dirty checkout fails before remote or installer execution"

new_fixture existing-nix-disabled-features
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
refs="$work/existing-nix-disabled-features.refs"
printf '%s\trefs/heads/fix/bootstrap\n' "$head_commit" > "$refs"
export FAKE_NIX_FEATURES_DISABLED=1
export RUN_HOME_OVERRIDE="$work/repair-home"
run_helper "$fixture" "$refs"
[[ "$rc" -eq 0 ]] || fail "disabled existing Nix features were not reconciled:\n$output"
grep -Fx 'extra-experimental-features = nix-command flakes' \
    "$RUN_HOME_OVERRIDE/.config/nix/nix.conf" >/dev/null ||
    fail "existing Nix reconciliation did not publish the required user features"
[[ "$output" == *"required user features were reconciled"* ]] ||
    fail "existing Nix reconciliation was not reported"
assert_clean_diagnostic
pass "a completed upstream install with disabled nix-command self-heals on retry"
unset FAKE_NIX_FEATURES_DISABLED RUN_HOME_OVERRIDE

rm "$work/bin/nix"
cat > "$work/bin/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -s) echo "${FAKE_UNAME_SYSTEM:-Linux}" ;;
    -m) echo "${FAKE_UNAME_ARCH:-x86_64}" ;;
    *) exit 45 ;;
esac
EOF
cat > "$work/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
while [[ "$#" -gt 0 ]]; do
    if [[ "$1" == "--output" ]]; then
        : > "$2"
        exit 0
    fi
    shift
done
exit 46
EOF
cat > "$work/bin/sha256sum" <<'EOF'
#!/usr/bin/env bash
case "$1" in
    */nix-2.34.0-aarch64-darwin.tar.xz)
        digest="47cb78c9fdc7b630dbbb9a89869c8e8bcd8c9eb17be036fba18585120693a4c1"
        ;;
    */install-multi-user)
        digest="832c033bac08eac43e2749427cb3e85d12f11d34685f44153bf044c6d32fafd0"
        ;;
    */.install-multi-user.dotfiles)
        digest="cfa68093aa33400ea20db27d1469387c576e658855ea3c23060961c36ac31adf"
        ;;
    *) exit 53 ;;
esac
echo "$digest  $1"
EOF
cat > "$work/bin/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-tJf" ]]; then
    exit 1
fi
[[ "${1:-}" == "-xJf" && "${3:-}" == "-C" ]] || exit 47
installer_dir="$4/nix-2.34.0-${FAKE_NIX_SYSTEM:-x86_64-linux}"
mkdir -p "$installer_dir"
cat > "$installer_dir/install" <<'INSTALLER'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "${FAKE_INSTALL_ARGS:?}"
if [[ "${3:-}" != "--no-modify-profile" ]]; then
    echo "cat: /etc/bashrc: Permission denied" >&2
    exit 50
fi
NIX_INSTALLER_NO_MODIFY_PROFILE=1
[[ "${4:-}" == "--nix-extra-conf-file" && -f "${5:-}" ]] || exit 51
cat "$5" > "${FAKE_INSTALL_CONF:?}"
exec "$(dirname "$0")/install-multi-user"
INSTALLER
cat > "$installer_dir/install-multi-user" <<'MULTI_USER_INSTALLER'
#!/usr/bin/env bash
set -euo pipefail

task() {
    echo "$*"
}

configure_shell_profile() {
    echo "Setting up shell profiles: /etc/bashrc /etc/profile.d/nix.sh /etc/zshrc /etc/bash.bashrc /etc/zsh/zshrc" >&2
    echo "cat: /etc/bashrc: Permission denied" >&2
    exit 52
}

main() {
    configure_shell_profile
cat > "${FAKE_RUNTIME_BIN:?}/nix" <<'NIX'
#!/usr/bin/env bash
case "${1:-}" in
    --version) echo "nix (Nix) 2.34.0" ;;
    store) [[ "${2:-}" == "info" ]] ;;
    *) exit 48 ;;
esac
NIX
chmod +x "${FAKE_RUNTIME_BIN:?}/nix"
}

main
MULTI_USER_INSTALLER
chmod +x "$installer_dir/install" "$installer_dir/install-multi-user"
EOF
chmod +x "$work/bin/uname" "$work/bin/curl" "$work/bin/sha256sum" "$work/bin/tar"

new_fixture noninteractive-install
head_commit="$($REAL_GIT -C "$fixture" rev-parse HEAD)"
refs="$work/noninteractive-install.refs"
printf '%s\trefs/heads/fix/bootstrap\n' "$head_commit" > "$refs"
export FAKE_INSTALL_ARGS="$work/install-args.log"
export FAKE_INSTALL_CONF="$work/install-conf.log"
export FAKE_RUNTIME_BIN="$work/bin"
export RUN_PATH_OVERRIDE="$work/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export RUN_HOME_OVERRIDE="$work/install-home"
export FAKE_UNAME_SYSTEM=Darwin
export FAKE_UNAME_ARCH=arm64
export FAKE_NIX_SYSTEM=aarch64-darwin
expected_install_mode="--daemon"
run_helper "$fixture" "$refs"
[[ "$rc" -eq 0 ]] || fail "verified installer fixture failed:\n$output"
[[ "$(sed -n '1p' "$FAKE_INSTALL_ARGS")" == "$expected_install_mode" &&
    "$(sed -n '2p' "$FAKE_INSTALL_ARGS")" == "--yes" &&
    "$(sed -n '3p' "$FAKE_INSTALL_ARGS")" == "--no-modify-profile" &&
    "$(sed -n '4p' "$FAKE_INSTALL_ARGS")" == "--nix-extra-conf-file" ]] ||
    fail "upstream installer did not receive its mode, --yes, no-profile-mutation, and extra config flags"
grep -Fx 'extra-experimental-features = nix-command flakes' "$FAKE_INSTALL_CONF" >/dev/null ||
    fail "upstream installer did not receive the reviewed Nix feature config"
if [[ "$expected_install_mode" == "--no-daemon" ]]; then
    grep -Fx 'extra-experimental-features = nix-command flakes' \
        "$RUN_HOME_OVERRIDE/.config/nix/nix.conf" >/dev/null ||
        fail "single-user Linux install did not persist required Nix user features"
else
    [[ ! -e "$RUN_HOME_OVERRIDE/.config/nix/nix.conf" ]] ||
        fail "daemon install unexpectedly published a per-user Nix config"
fi
[[ "$output" == *"Nix prerequisite installed and verified"* ]] ||
    fail "installer success was not verified in the same shell"
[[ "$output" == *"Verified local Nix daemon profile-ownership patch:"* ]] ||
    fail "daemon installer patch did not pass its exact output hash check"
[[ "$output" == *"Leaving shell profiles unchanged (--no-modify-profile)"* ]] ||
    fail "patched daemon installer did not report the no-profile ownership boundary"
assert_clean_diagnostic
pass "verified upstream installer leaves shell profiles to dotfiles and persists flake features"

echo "all Nix prerequisite checkout identity behaviors OK"
