#!/usr/bin/env bash
# Active POSIX profiles initialize the selected private Starship directly.
# The retired mtime-based precompile cache is not part of this contract.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"
fixture_home="$DOTFILES_SHELL_FIXTURES/posix"
mkdir -p "$fixture_home/.cache"
printf '%s\n' 'throw-stale-cache' > "$fixture_home/.cache/starship-init.zsh"
for attempt in 1 2; do
    result=$(HOME="$fixture_home" zsh -d -f -i -c 'source "$1" || exit 1; print -r -- "$DOTFILES_FIXTURE_STARSHIP"' zsh "$ACTIVE_ZSH" 2>&1)
    [[ "$result" == 'loaded' ]] || { echo "FAIL: active Starship initialization $attempt: $result"; exit 1; }
done
grep -Fx 'throw-stale-cache' "$fixture_home/.cache/starship-init.zsh" >/dev/null
echo "OK"
