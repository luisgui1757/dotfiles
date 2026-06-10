#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
IMAGE="${DOTFILES_GREENFIELD_IMAGE:-ubuntu:24.04}"
EXPECTED_PM="${EXPECTED_PM:-apt}"

usage() {
    cat <<'EOF'
docker-greenfield.sh -- reproduce the Ubuntu container e2e install locally.

Usage:
  tests/greenfield/docker-greenfield.sh

Environment:
  DOTFILES_GREENFIELD_IMAGE   container image, default ubuntu:24.04
  EXPECTED_PM                 expected native package manager, default apt
EOF
}

case "${1:-}" in
    -h|--help)
        usage
        exit 0
        ;;
    "")
        ;;
    *)
        echo "FAIL: unknown arg: $1" >&2
        exit 2
        ;;
esac

if ! command -v docker >/dev/null 2>&1; then
    echo "FAIL: docker is not on PATH" >&2
    exit 1
fi

docker run --rm \
    --env EXPECTED_PM="$EXPECTED_PM" \
    --volume "$REPO_ROOT:/repo:ro" \
    "$IMAGE" \
    bash /repo/tests/ci/container-e2e.sh
