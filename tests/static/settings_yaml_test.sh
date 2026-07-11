#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CHECK="$REPO_ROOT/tests/static/assert_no_probot_branches.rb"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

command -v ruby >/dev/null 2>&1 || {
    echo "FAIL: ruby is required for semantic .github/settings.yml policy checks" >&2
    exit 1
}

expect_rejected() {
    local name="$1"
    shift
    local fixture="$WORK/$name.yml" output rc
    printf '%s\n' "$@" > "$fixture"
    set +e
    output="$(ruby "$CHECK" "$fixture" 2>&1)"
    rc=$?
    set -e
    [[ "$rc" -ne 0 ]] || {
        echo "FAIL: semantic Settings guard accepted $name" >&2
        exit 1
    }
    grep -F "top-level branches key" <<<"$output" >/dev/null || {
        echo "FAIL: semantic Settings guard rejected $name for the wrong reason" >&2
        printf '%s\n' "$output" >&2
        exit 1
    }
}

ruby "$CHECK" "$REPO_ROOT/.github/settings.yml"

safe_fixture="$WORK/nested-branches.yml"
printf '%s\n' \
    'repository:' \
    '  description: branches are discussed here' \
    'metadata:' \
    '  branches: nested values do not grant Probot ownership' \
    > "$safe_fixture"
ruby "$CHECK" "$safe_fixture"

expect_rejected block \
    'repository: {}' \
    'branches:' \
    '  - name: main'
expect_rejected inline-array \
    'repository: {}' \
    'branches: [{name: main, protection: {required_status_checks: {strict: true}}}]'
expect_rejected inline-map \
    'repository: {}' \
    'branches: {main: {protection: null}}'
expect_rejected null \
    'repository: {}' \
    'branches: null'
expect_rejected alias \
    'protected: &protected [{name: main}]' \
    'repository: {}' \
    'branches: *protected'
expect_rejected merge-alias \
    'defaults: &defaults' \
    '  branches: [{name: main}]' \
    'repository: {}' \
    '<<: *defaults'

echo "OK: semantic YAML guard rejects every top-level Probot branches shape"
