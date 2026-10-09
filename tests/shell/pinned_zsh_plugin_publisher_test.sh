#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
PUBLISHER="$REPO_ROOT/scripts/ensure-pinned-zsh-plugin.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

make_repo() {
    local work="$1" bare="$2" first second
    mkdir -p "$work"
    git -C "$work" init -q
    git -C "$work" config user.name test
    git -C "$work" config user.email test@example.invalid
    printf '%s\n' 'plugin-v1' > "$work/plugin.zsh"
    git -C "$work" add plugin.zsh
    git -C "$work" commit -qm v1
    first="$(git -C "$work" rev-parse HEAD)"
    printf '%s\n' 'plugin-v2' > "$work/plugin.zsh"
    git -C "$work" commit -qam v2
    second="$(git -C "$work" rev-parse HEAD)"
    git clone -q --bare "$work" "$bare"
    printf '%s %s\n' "$first" "$second"
}

read -r commit1 commit2 <<EOF
$(make_repo "$WORK/source" "$WORK/plugin.git")
EOF
repo="file://$WORK/plugin.git"
export DOTFILES_PINNED_GIT_ALLOW_FILE=1

# Template-time state probes must be strictly read-only. A fresh bootstrap
# marker reports the post-install fingerprint without creating the target
# parent; an existing marker reports the missing checkout without doing so.
check_only_target="$WORK/check-only/home/.local/share/dotfiles/zsh-plugins/plugin"
check_only_marker="$WORK/check-only-marker"
check_only_output="$(
    DOTFILES_PINNED_GIT_CHECK_ONLY=1 \
    DOTFILES_PINNED_GIT_BOOTSTRAP_MARKER="$check_only_marker" \
    /bin/bash "$PUBLISHER" check-only-plugin "$repo" v1 "$commit1" plugin.zsh "$check_only_target"
)"
[[ "$check_only_output" == "ready:$commit1" ]] ||
    fail "fresh check-only probe did not report the bootstrap fingerprint"
[[ ! -e "$WORK/check-only" ]] ||
    fail "fresh check-only probe created the absent target parent"
printf '%s\n' initialized > "$check_only_marker"
check_only_output="$(
    DOTFILES_PINNED_GIT_CHECK_ONLY=1 \
    DOTFILES_PINNED_GIT_BOOTSTRAP_MARKER="$check_only_marker" \
    /bin/bash "$PUBLISHER" check-only-plugin "$repo" v1 "$commit1" plugin.zsh "$check_only_target"
)"
[[ "$check_only_output" == "invalid" ]] ||
    fail "check-only probe with initialized state accepted a missing checkout"
[[ ! -e "$WORK/check-only" ]] ||
    fail "initialized check-only probe created the absent target parent"

target="$WORK/managed/plugin"
/bin/bash "$PUBLISHER" test-plugin "$repo" v1 "$commit1" plugin.zsh "$target" >/dev/null
[[ "$(git -C "$target" rev-parse HEAD)" == "$commit1" ]] || fail "initial exact commit was not published"
[[ "$(cat "$target/plugin.zsh")" == plugin-v1 ]] || fail "initial required plugin file is wrong"
[[ "$(git -C "$target" remote get-url origin)" == "$repo" ]] || fail "published origin is wrong"
[[ -z "$(git -C "$target" status --porcelain --untracked-files=all --ignored)" ]] || fail "published checkout is not clean"
check_only_output="$(
    DOTFILES_PINNED_GIT_CHECK_ONLY=1 \
    /bin/bash "$PUBLISHER" test-plugin "$repo" v1 "$commit1" plugin.zsh "$target"
)"
[[ "$check_only_output" == "ready:$commit1" ]] ||
    fail "check-only probe did not recognize a verified checkout"
[[ ! -e "${target}.lock" ]] ||
    fail "verified check-only probe created a publication lock"

# Verified-cache reuse performs no network repair and creates no staging state.
mv "$WORK/plugin.git" "$WORK/plugin.git.offline"
/bin/bash "$PUBLISHER" test-plugin "$repo" v1 "$commit1" plugin.zsh "$target" >/dev/null
mv "$WORK/plugin.git.offline" "$WORK/plugin.git"
find "$WORK/managed" -maxdepth 1 \( -name '*.stage.*' -o -name '*.lock' \) -print | grep -q . \
    && fail "verified-cache reuse leaked staging/lock state"

# Dirty executable content is neutralized before a failed repair. The fixed
# source path is absent afterward, so zshrc cannot source the bad payload.
printf '%s\n' 'malicious-local-change' > "$target/plugin.zsh"
mv "$WORK/plugin.git" "$WORK/plugin.git.offline"
set +e
failure_output="$(/bin/bash "$PUBLISHER" test-plugin "$repo" v2 "$commit2" plugin.zsh "$target" 2>&1)"
failure_rc=$?
set -e
[[ "$failure_rc" -ne 0 ]] || fail "network failure unexpectedly succeeded"
[[ ! -e "$target" ]] || fail "mismatched payload remained sourceable after failed repair"
[[ "$failure_output" == *"quarantine"* && "$failure_output" == *"could not fetch"* ]] \
    || fail "failed repair did not report quarantine and fetch failure"
dirty_quarantine="$(find "$WORK/managed" -maxdepth 1 -name 'plugin.quarantine.*' -type d -print -quit)"
[[ -n "$dirty_quarantine" && "$(cat "$dirty_quarantine/plugin.zsh")" == malicious-local-change ]] \
    || fail "dirty prior payload was not preserved for recovery"
find "$WORK/managed" -maxdepth 1 \( -name '*.stage.*' -o -name '*.lock' \) -print | grep -q . \
    && fail "failed repair leaked staging/lock state"

# Retry self-heals after the repository is reachable, without deleting the
# dirty quarantine that may contain user data.
mv "$WORK/plugin.git.offline" "$WORK/plugin.git"
/bin/bash "$PUBLISHER" test-plugin "$repo" v2 "$commit2" plugin.zsh "$target" >/dev/null
[[ "$(git -C "$target" rev-parse HEAD)" == "$commit2" ]] || fail "retry did not publish the new pin"
[[ -d "$dirty_quarantine" ]] || fail "retry deleted the dirty recovery quarantine"

# A legitimate clean pin change replaces atomically and removes its disposable
# old managed checkout instead of accumulating recovery directories.
clean_target="$WORK/clean/plugin"
/bin/bash "$PUBLISHER" clean-plugin "$repo" v1 "$commit1" plugin.zsh "$clean_target" >/dev/null
/bin/bash "$PUBLISHER" clean-plugin "$repo" v2 "$commit2" plugin.zsh "$clean_target" >/dev/null
[[ "$(git -C "$clean_target" rev-parse HEAD)" == "$commit2" ]] || fail "clean pin update did not self-heal"
if find "$WORK/clean" -maxdepth 1 -name 'plugin.quarantine.*' -print | grep -q .; then
    fail "clean pin update retained a disposable old checkout"
fi

# Non-Git/partial payloads are preserved outside the sourceable path while the
# verified checkout is published.
partial_target="$WORK/partial/plugin"
mkdir -p "$partial_target"
printf '%s\n' partial > "$partial_target/plugin.zsh"
/bin/bash "$PUBLISHER" partial-plugin "$repo" v2 "$commit2" plugin.zsh "$partial_target" >/dev/null
[[ "$(git -C "$partial_target" rev-parse HEAD)" == "$commit2" ]] || fail "partial payload was not repaired"
partial_quarantine="$(find "$WORK/partial" -maxdepth 1 -name 'plugin.quarantine.*' -type d -print -quit)"
[[ -n "$partial_quarantine" && "$(cat "$partial_quarantine/plugin.zsh")" == partial ]] \
    || fail "partial prior payload was not preserved"

# Concurrent first starts serialize and converge on one proved checkout.
# uutils 0.10 can return success when mkdir(2) returned EEXIST in a race.
# Model that external-command behavior deterministically, without replacing
# the kernel/filesystem or any publisher function.
real_mkdir="$(command -v mkdir)"
mkdir -p "$WORK/racy-bin"
cat > "$WORK/racy-bin/mkdir" <<'SH'
#!/bin/bash
if [[ "$#" -eq 1 && "$1" == *.lock ]]; then
    "$PUBLISHER_TEST_MKDIR" -p "$1"
    sleep 0.15
else
    exec "$PUBLISHER_TEST_MKDIR" "$@"
fi
SH
chmod +x "$WORK/racy-bin/mkdir"
export PUBLISHER_TEST_MKDIR="$real_mkdir"
run_concurrent() {
    local concurrent="$WORK/$1/plugin" launch_path="$2"
    PATH="$launch_path" /bin/bash "$PUBLISHER" concurrent-plugin "$repo" v2 "$commit2" plugin.zsh "$concurrent" >"$WORK/concurrent.1.log" 2>&1 &
    pid1=$!
    PATH="$launch_path" /bin/bash "$PUBLISHER" concurrent-plugin "$repo" v2 "$commit2" plugin.zsh "$concurrent" >"$WORK/concurrent.2.log" 2>&1 &
    pid2=$!
    concurrent_failed=0
    wait "$pid1" || concurrent_failed=1
    wait "$pid2" || concurrent_failed=1
    if [[ "$concurrent_failed" -ne 0 ]]; then
        cat "$WORK/concurrent.1.log" "$WORK/concurrent.2.log" >&2
        fail "concurrent publisher failed"
    fi
    [[ "$(git -C "$concurrent" rev-parse HEAD)" == "$commit2" ]] || fail "concurrent publication produced the wrong pin"
    if find "$(dirname "$concurrent")" -maxdepth 1 \( -name '*.stage.*' -o -name '*.lock' \) -print | grep -q .; then
        fail "concurrent publication leaked staging/lock state"
    fi
}
run_concurrent concurrent "$PATH"
run_concurrent racy-mkdir "$WORK/racy-bin:$PATH"

# Fresh install-deps publishes plugins before installing Python. Only the
# publisher's existing shell/Git tools are available at that point.
mkdir -p "$WORK/bootstrap-bin"
for tool in basename cat date dirname env git mkdir mktemp mv rm sleep; do
    ln -s "$(command -v "$tool")" "$WORK/bootstrap-bin/$tool"
done
bootstrap_target="$WORK/bootstrap/plugin"
PATH="$WORK/bootstrap-bin" /bin/bash "$PUBLISHER" bootstrap-plugin "$repo" v2 "$commit2" plugin.zsh "$bootstrap_target" >/dev/null
[[ "$(git -C "$bootstrap_target" rev-parse HEAD)" == "$commit2" ]] || fail "bootstrap without Python did not publish the exact pin"
[[ ! -e "${bootstrap_target}.lock" ]] || fail "bootstrap publication leaked its lock"

echo "OK"
