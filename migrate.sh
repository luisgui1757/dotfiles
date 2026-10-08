#!/usr/bin/env bash
# Preserve and detach released shell profiles before ordinary setup adoption.
set -euo pipefail
checkout="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
export DOTFILES_ENTRYPOINT=migrate
exec bash "$checkout/scripts/installer-bootstrap.sh" "$@"
