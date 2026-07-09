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
    case "$f" in
        ./tests/shell/*_test.sh|./tests/nix/setup_home_manager_test.sh|./tests/nix/setup_nix_darwin_test.sh)
            # Source-only fixtures intentionally source setup/install scripts via
            # runtime paths, set globals consumed by those sourced functions, and
            # override commands such as uname indirectly. Keep these test-only
            # false positives out of the lint signal; production scripts remain
            # strict. SC2317 is the same source-only fixture class: command
            # stubs are reached indirectly through sourced installer functions.
            shellcheck --shell=bash --exclude=SC1091,SC2034,SC2317,SC2329 "$f" || fail=1
            ;;
        *)
            shellcheck --shell=bash "$f" || fail=1
            ;;
    esac
done
[[ "$fail" -eq 0 ]] || exit 1

# zshrc — best-effort under bash shell-check; warnings only.
shellcheck --shell=bash --severity=warning shells/zshrc || true

echo "lint OK"
