#!/usr/bin/env bash
# Regression guard: fetches prune stale remote-tracking refs by default while
# ~/.gitconfig remains a later, user-owned override layer.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CONFIG="$REPO_ROOT/git/config"
MIRROR="$REPO_ROOT/home/dot_config/git/config"
TEST_HOME="$(mktemp -d)"
trap 'rm -rf "$TEST_HOME"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

cmp -s "$CONFIG" "$MIRROR" ||
    fail "canonical Git config and chezmoi mirror differ"
[[ "$(git config --file "$CONFIG" --bool --get fetch.prune)" == "true" ]] ||
    fail "managed Git config must enable fetch.prune"

mkdir -p "$TEST_HOME/.config/git"
cp "$CONFIG" "$TEST_HOME/.config/git/config"
managed_value="$(
    env HOME="$TEST_HOME" XDG_CONFIG_HOME="$TEST_HOME/.config" GIT_CONFIG_NOSYSTEM=1 \
        git -C "$TEST_HOME" config --bool --get fetch.prune
)"
[[ "$managed_value" == "true" ]] ||
    fail "XDG Git config did not provide the fetch.prune default"

git config --file "$TEST_HOME/.gitconfig" fetch.prune false
override_value="$(
    env HOME="$TEST_HOME" XDG_CONFIG_HOME="$TEST_HOME/.config" GIT_CONFIG_NOSYSTEM=1 \
        git -C "$TEST_HOME" config --bool --get fetch.prune
)"
[[ "$override_value" == "false" ]] ||
    fail "the user .gitconfig must remain the later override layer"

echo "Git fetch pruning default and user override order OK"
