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
# Inherited entries must regain their managed priority after a login shell's
# system profile (macOS path_helper) has moved system commands ahead of them.
fixture_home="$DOTFILES_SHELL_FIXTURES/zsh"
for shell in /bin/sh /bin/bash /bin/zsh; do
    actual=$(PATH="$fixture_home/personal-bin:/usr/bin:$fixture_home/.local/bin:/bin:$fixture_home/.local/bin" \
        "$shell" -c ". \"\$1\"; . \"\$1\"; printf '%s' \"\$PATH\"" shell "$fixture_home/state/shells/env.sh")
    if [[ "$actual" != "$fixture_home/.local/bin:$fixture_home/personal-bin:/usr/bin:/bin" ]]; then
        echo "FAIL: $shell did not restore managed PATH priority: $actual"
        exit 1
    fi
done

if [[ "$(uname -s)" == Darwin ]]; then
    printf '#!/bin/sh\nprintf "managed-git\\n"\n' > "$fixture_home/.local/bin/git"
    chmod 700 "$fixture_home/.local/bin/git"
    actual=$(HOME="$fixture_home" ZDOTDIR="$fixture_home" PATH=/usr/bin:/bin:/usr/sbin:/sbin \
        /bin/zsh -lc 'command -v git; /bin/zsh -lc "command -v git"')
    expected=$(printf '%s\n%s' "$fixture_home/.local/bin/git" "$fixture_home/.local/bin/git")
    if [[ "$actual" != "$expected" ]]; then
        echo "FAIL: nested macOS login changed the selected Git: $actual"
        exit 1
    fi
fi
echo "OK"
