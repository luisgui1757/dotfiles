#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="$REPO_ROOT/scripts/ci-logical-proof.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

source_head_sha="1111111111111111111111111111111111111111"
executed_sha="2222222222222222222222222222222222222222"
marker="$WORK/path with spaces/proof.env"

run_proof() {
    DOTFILES_SOURCE_HEAD_SHA="${TEST_SOURCE_HEAD_SHA-$source_head_sha}" \
    GITHUB_SHA="${TEST_EXECUTED_SHA-$executed_sha}" \
    GITHUB_RUN_ID="${TEST_RUN_ID-12345}" \
    GITHUB_RUN_ATTEMPT="${TEST_RUN_ATTEMPT-2}" \
        "$SCRIPT" "$@"
}

expect_failure() {
    local expected="$1"
    shift
    local output rc
    set +e
    output="$("$@" 2>&1)"
    rc=$?
    set -e
    [[ "$rc" -ne 0 ]] || {
        echo "FAIL: command unexpectedly succeeded: $*" >&2
        exit 1
    }
    grep -F "$expected" <<<"$output" >/dev/null || {
        echo "FAIL: expected failure text not found: $expected" >&2
        printf '%s\n' "$output" >&2
        exit 1
    }
}

run_proof emit "$marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
grep -Fx "schema=2" "$marker" >/dev/null
grep -Fx "source_head_sha=$source_head_sha" "$marker" >/dev/null
grep -Fx "executed_sha=$executed_sha" "$marker" >/dev/null
if grep -q '^head_sha=' "$marker"; then
    echo "FAIL: ambiguous schema-1 head_sha field remains" >&2
    exit 1
fi
run_proof verify "$marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
echo "ok  : pull-request proof binds source head and executed merge SHA separately"

expect_failure "logical proof context mismatch" \
    run_proof verify "$marker" "setup.sh / macos" "setup.sh / ubuntu-24.04"
expect_failure "legacy proof context mismatch" \
    run_proof verify "$marker" "setup.sh / linux" "setup.sh / macos-26"
echo "ok  : logical and legacy identity drift fail verification"

TEST_SOURCE_HEAD_SHA="3333333333333333333333333333333333333333" \
    expect_failure "does not bind the current source head SHA" \
    run_proof verify "$marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
echo "ok  : source-head drift fails verification"

TEST_EXECUTED_SHA="4444444444444444444444444444444444444444" \
    expect_failure "does not bind the current executed SHA" \
    run_proof verify "$marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
echo "ok  : executed-SHA drift fails verification"

missing_source_marker="$WORK/missing-source.env"
TEST_SOURCE_HEAD_SHA="" expect_failure "DOTFILES_SOURCE_HEAD_SHA is not a full commit identity" \
    run_proof emit "$missing_source_marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
[[ ! -e "$missing_source_marker" ]] || {
    echo "FAIL: invalid source identity published a marker" >&2
    exit 1
}
echo "ok  : missing source head fails before publication"

schema_one_marker="$WORK/schema-one.env"
sed 's/^schema=2$/schema=1/' "$marker" > "$schema_one_marker"
expect_failure "unsupported logical proof schema" \
    run_proof verify "$schema_one_marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
echo "ok  : obsolete ambiguous schema fails closed"

duplicate_marker="$WORK/duplicate.env"
{
    printf 'schema=2\n'
    cat "$marker"
} > "$duplicate_marker"
expect_failure "duplicate proof field: schema" \
    run_proof verify "$duplicate_marker" "setup.sh / linux" "setup.sh / ubuntu-24.04"
expect_failure "logical proof marker is missing" \
    run_proof verify "$WORK/missing.env" "setup.sh / linux" "setup.sh / ubuntu-24.04"
echo "ok  : duplicate and missing proof markers fail closed"

same_sha_marker="$WORK/workflow-dispatch.env"
TEST_SOURCE_HEAD_SHA="$source_head_sha" TEST_EXECUTED_SHA="$source_head_sha" \
    run_proof emit "$same_sha_marker" "setup.sh / macos" "setup.sh / macos-26"
TEST_SOURCE_HEAD_SHA="$source_head_sha" TEST_EXECUTED_SHA="$source_head_sha" \
    run_proof verify "$same_sha_marker" "setup.sh / macos" "setup.sh / macos-26"
echo "ok  : non-PR proof truthfully records equal source and executed identities"

echo "all logical proof identity behaviors OK"
