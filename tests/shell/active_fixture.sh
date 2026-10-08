#!/usr/bin/env bash
# Source from shell checks. The batch shares one freshly generated fixture;
# standalone checks own and clean their own temporary export.
ACTIVE_SHELL_OWNED=''
active_shell_cleanup() {
    if [[ -n "$ACTIVE_SHELL_OWNED" ]]; then rm -rf "$ACTIVE_SHELL_OWNED"; fi
}
if [[ -z "${DOTFILES_SHELL_FIXTURES:-}" ]]; then
    command -v go >/dev/null || { echo 'FAIL: Go is required to generate the active shell profiles' >&2; exit 1; }
    DOTFILES_SHELL_FIXTURES="$(mktemp -d)"
    DOTFILES_SHELL_FIXTURES="$(cd "$DOTFILES_SHELL_FIXTURES" && pwd -P)"
    ACTIVE_SHELL_OWNED="$DOTFILES_SHELL_FIXTURES"
    trap active_shell_cleanup EXIT
    DOTFILES_WRITE_SHELL_FIXTURES="$DOTFILES_SHELL_FIXTURES" go -C "$REPO_ROOT/installer" test -count=1 -run '^TestWriteActiveShellFixtures$' .
fi
export DOTFILES_SHELL_FIXTURES
export ACTIVE_ZSH="$DOTFILES_SHELL_FIXTURES/posix/state/shells/init.sh"
export ACTIVE_ZSH_CORE="$DOTFILES_SHELL_FIXTURES/zsh/state/shells/init.sh"
[[ -s "$ACTIVE_ZSH" && -s "$ACTIVE_ZSH_CORE" ]] || { echo 'FAIL: active shell generation is incomplete' >&2; exit 1; }
