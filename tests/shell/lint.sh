#!/usr/bin/env bash
# Shellcheck everything we ship as a script.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v shellcheck >/dev/null 2>&1; then
    echo "skipped: shellcheck not installed (brew install shellcheck)"
    exit 0
fi

# Gather .sh files (using process substitution into an array to stay
# bash 3.2 compatible — `mapfile` is bash 4+).
sh_files=()
while IFS= read -r f; do sh_files+=("$f"); done < <(
    find . -type f -name "*.sh" -not -path "./.git/*" -not -path "./tests/.cache/*"
)

fail=0
for f in "${sh_files[@]}"; do
    shellcheck -x --shell=bash "$f" || fail=1
done
[[ "$fail" -eq 0 ]] || exit 1

# ShellCheck does not parse zsh glob qualifiers. Check the real language
# instead of suppressing parser errors; generated output uses the same gate.
command -v zsh >/dev/null || { echo 'FAIL: zsh is required for active shell syntax checks'; exit 1; }
for script in installer/shells/*.zsh; do zsh -n "$script"; done
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"
zsh -n "$ACTIVE_ZSH"
zsh -n "$ACTIVE_ZSH_CORE"
shellcheck --shell=sh "$DOTFILES_SHELL_FIXTURES/posix/state/shells/env.sh"
echo "lint OK"
