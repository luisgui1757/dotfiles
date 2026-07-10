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

target="$WORK/managed/plugin"
/bin/bash "$PUBLISHER" test-plugin "$repo" v1 "$commit1" plugin.zsh "$target" >/dev/null
[[ "$(git -C "$target" rev-parse HEAD)" == "$commit1" ]] || fail "initial exact commit was not published"
[[ "$(cat "$target/plugin.zsh")" == plugin-v1 ]] || fail "initial required plugin file is wrong"
[[ "$(git -C "$target" remote get-url origin)" == "$repo" ]] || fail "published origin is wrong"
[[ -z "$(git -C "$target" status --porcelain --untracked-files=all --ignored)" ]] || fail "published checkout is not clean"

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
concurrent="$WORK/concurrent/plugin"
/bin/bash "$PUBLISHER" concurrent-plugin "$repo" v2 "$commit2" plugin.zsh "$concurrent" >"$WORK/concurrent.1.log" 2>&1 &
pid1=$!
/bin/bash "$PUBLISHER" concurrent-plugin "$repo" v2 "$commit2" plugin.zsh "$concurrent" >"$WORK/concurrent.2.log" 2>&1 &
pid2=$!
wait "$pid1" || fail "first concurrent publisher failed"
wait "$pid2" || fail "second concurrent publisher failed"
[[ "$(git -C "$concurrent" rev-parse HEAD)" == "$commit2" ]] || fail "concurrent publication produced the wrong pin"
find "$WORK/concurrent" -maxdepth 1 \( -name '*.stage.*' -o -name '*.lock' \) -print | grep -q . \
    && fail "concurrent publication leaked staging/lock state"

echo "OK"
