#!/usr/bin/env bash
# Assert the zshrc produces a duplicate-free PATH.
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT
mkdir -p "$TMP_HOME/.local/bin" "$TMP_HOME/.cargo/bin"

# Inject pre-existing PATH dupes that the zshrc should collapse. We bracket
# the print -r with a sentinel so noise from compinit/plugins (especially
# under non-interactive CI shells that lack a tty) does not pollute the
# captured PATH value.
SENTINEL_BEGIN="@@PATH_BEGIN@@"
SENTINEL_END="@@PATH_END@@"
raw=$(HOME="$TMP_HOME" PATH="/opt/homebrew/bin:$TMP_HOME/.local/bin:/usr/local/bin:$TMP_HOME/.local/bin:/opt/homebrew/bin:$PATH" \
    zsh -i -c "source $REPO_ROOT/shells/zshrc; print -r -- '$SENTINEL_BEGIN'\$PATH'$SENTINEL_END'" 2>&1) || true
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
if [[ "${path_value%%:*}" != "$TMP_HOME/.local/bin" ]]; then
    echo "FAIL: ~/.local/bin is not first on PATH"
    printf '%s\n' "$path_value"
    exit 1
fi
echo "OK"
