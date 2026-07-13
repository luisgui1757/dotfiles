#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
IMAGE="${DOTFILES_LINUX_LIFECYCLE_IMAGE:-dotfiles-linux-owner-lifecycle:local}"
BUNDLE_ROOT=""

cleanup() {
    [[ -n "$BUNDLE_ROOT" ]] && rm -rf "$BUNDLE_ROOT"
}

trap cleanup EXIT

if ! command -v docker >/dev/null 2>&1; then
    echo "FAIL: docker is not on PATH" >&2
    exit 1
fi

git -C "$REPO_ROOT" diff --quiet || {
    echo "FAIL: tracked worktree changes exist; commit them before the Linux lifecycle" >&2
    exit 1
}
git -C "$REPO_ROOT" diff --cached --quiet || {
    echo "FAIL: staged changes exist; commit them before the Linux lifecycle" >&2
    exit 1
}

expected_head="$(git -C "$REPO_ROOT" rev-parse HEAD)"
BUNDLE_ROOT="$(mktemp -d)"
bundle="$BUNDLE_ROOT/dotfiles.bundle"
git -C "$REPO_ROOT" bundle create "$bundle" HEAD
git bundle verify "$bundle" >/dev/null

docker build \
    --file "$REPO_ROOT/tests/greenfield/linux-owner-lifecycle.Dockerfile" \
    --tag "$IMAGE" \
    "$REPO_ROOT"

docker run --rm --interactive --tty \
    --env "DOTFILES_LINUX_LIFECYCLE_EXPECTED_HEAD=$expected_head" \
    --mount "type=bind,source=$bundle,target=/dotfiles.bundle,readonly" \
    "$IMAGE" \
    bash /usr/local/bin/dotfiles-linux-owner-lifecycle-container
