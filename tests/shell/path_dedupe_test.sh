#!/usr/bin/env bash
# Assert the zshrc produces a duplicate-free PATH.
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"
TMP_HOME="$(mktemp -d)"
trap 'active_shell_cleanup; rm -rf "$TMP_HOME"' EXIT
mkdir -p "$DOTFILES_SHELL_FIXTURES/zsh/.local/bin"

# Inject pre-existing PATH dupes that the zshrc should collapse. We bracket
# the print -r with a sentinel so noise from compinit/plugins (especially
# under non-interactive CI shells that lack a tty) does not pollute the
# captured PATH value.
SENTINEL_BEGIN="@@PATH_BEGIN@@"
SENTINEL_END="@@PATH_END@@"
raw=$(HOME="$TMP_HOME" PATH="/usr/bin:/bin:/usr/bin:/bin" \
    zsh -d -f -i -c "source \"$DOTFILES_SHELL_FIXTURES/zsh/state/shells/env.sh\" || exit 1; source \"$DOTFILES_SHELL_FIXTURES/zsh/state/shells/env.sh\" || exit 1; source \"$ACTIVE_ZSH_CORE\" || exit 1; print -r -- '$SENTINEL_BEGIN'\$PATH'$SENTINEL_END'" 2>&1)
# Extract just the PATH line between the sentinels.
path_value=$(printf '%s\n' "$raw" | grep -oE "${SENTINEL_BEGIN}.*${SENTINEL_END}" | head -1 | sed -e "s/^${SENTINEL_BEGIN}//" -e "s/${SENTINEL_END}\$//")
if [[ -z "$path_value" ]]; then
    echo "FAIL: could not extract PATH from zsh output. Raw output:"
    printf '%s\n' "$raw"
    exit 1
fi
# Split on : and count duplicates.
dupes=$(printf '%s\n' "$path_value" | tr ':' '\n' | sort | uniq -d | wc -l | tr -d ' ')
if [[ "$dupes" -ne 0 ]]; then
    echo "FAIL: PATH contains $dupes duplicate entries"
    printf '%s\n' "$path_value" | tr ':' '\n' | sort | uniq -d
    exit 1
fi
if [[ "${path_value%%:*}" != "$DOTFILES_SHELL_FIXTURES/zsh/.local/bin" ]]; then
    echo "FAIL: ~/.local/bin is not first on PATH"
    printf '%s\n' "$path_value"
    exit 1
fi
echo "OK"
