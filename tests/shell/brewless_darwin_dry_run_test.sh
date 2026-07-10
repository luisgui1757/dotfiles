#!/usr/bin/env bash
# A Brew-less macOS dry-run previews bootstrap, then continues with Brew-backed
# phases rather than aborting mid-plan or claiming that Brew was installed.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

DRY_RUN=1
YES_ALL=1
PM=brew_missing
uname() { [[ "${1:-}" == -s ]] && echo Darwin || command uname "$@"; }
homebrew_bin() { return 1; }
have() { [[ "$1" == brew ]] && return 1; command -v "$1" >/dev/null 2>&1; }
enable_homebrew_for_current_shell() { echo "FAIL: dry-run attempted live brew shellenv" >&2; return 1; }
persist_homebrew_shellenv() { echo "FAIL: dry-run attempted persistent shell mutation" >&2; return 1; }

output_file="${TMPDIR:-/tmp}/dotfiles-brewless-dry-run.$$"
trap 'rm -f "$output_file"' EXIT
bootstrap_package_manager > "$output_file"
output="$(cat "$output_file")"
[[ "$PM" == brew ]] || { echo "FAIL: dry-run did not model Brew after bootstrap preview"; exit 1; }
[[ "$output" == *"would: curl -fsSL"* ]] || { echo "FAIL: bootstrap download was not previewed"; exit 1; }
[[ "$output" == *"plan      Homebrew-dependent phases continue"* ]] || { echo "FAIL: later Brew phases were not kept in the plan"; exit 1; }
[[ "$output" != *"Homebrew installed"* && "$output" != *"was installed"* ]] || {
    echo "FAIL: dry-run falsely claimed an installation"
    exit 1
}

sentinel=""
[[ "$PM" == brew ]] && sentinel=later-phase-reached
[[ "$sentinel" == later-phase-reached ]] || { echo "FAIL: later phase sentinel was not reached"; exit 1; }
echo "OK"
