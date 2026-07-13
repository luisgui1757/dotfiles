#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
IMAGE="${DOTFILES_LINUX_LIFECYCLE_IMAGE:-dotfiles-linux-owner-lifecycle:local}"

if ! command -v docker >/dev/null 2>&1; then
    echo "FAIL: docker is not on PATH" >&2
    exit 1
fi

docker build \
    --file "$REPO_ROOT/tests/greenfield/linux-owner-lifecycle.Dockerfile" \
    --tag "$IMAGE" \
    "$REPO_ROOT"

docker run --rm --interactive --tty \
    --volume "$REPO_ROOT:/repo:ro" \
    "$IMAGE" \
    bash /repo/tests/ci/linux-owner-lifecycle-container.sh
