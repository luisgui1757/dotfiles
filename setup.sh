#!/usr/bin/env bash
# Human menu by default; automation supplies the explicit machine request.
set -euo pipefail
checkout="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
export DOTFILES_ENTRYPOINT=setup
exec bash "$checkout/scripts/installer-bootstrap.sh" "$@"
