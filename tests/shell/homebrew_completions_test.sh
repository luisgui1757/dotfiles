#!/usr/bin/env bash
# Homebrew must own and reconcile the completion links consumed by new shells.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

FAIL=0
RUNS=0
fail() { echo "FAIL: $*" >&2; FAIL=1; }

brew() {
    RUNS=$((RUNS + 1))
    [[ "$*" == "completions link" ]] || {
        echo "unexpected brew arguments: $*" >&2
        return 97
    }
    return "${BREW_RC:-0}"
}

PM=apt
DRY_RUN=0
link_homebrew_completions || fail "non-Homebrew hosts should be a no-op"
[[ "$RUNS" -eq 0 ]] || fail "non-Homebrew hosts invoked brew"

PM=brew
DRY_RUN=1
preview="$(link_homebrew_completions)" || fail "dry-run preview failed"
[[ "$preview" == *"would: brew completions link"* ]] || fail "dry-run did not preview the command"
[[ "$RUNS" -eq 0 ]] || fail "dry-run invoked brew"

DRY_RUN=0
BREW_RC=0
link_homebrew_completions >/dev/null || fail "successful reconciliation failed"
[[ "$RUNS" -eq 1 ]] || fail "successful reconciliation did not invoke brew exactly once"

BREW_RC=23
if link_homebrew_completions >/dev/null 2>&1; then
    fail "completion-link failure was accepted"
fi
[[ "$RUNS" -eq 2 ]] || fail "failed reconciliation did not invoke brew exactly once"

grep -F "elif [[ \"\$PM\" == \"brew\" ]] && ! link_homebrew_completions" "$REPO_ROOT/install-deps.sh" >/dev/null ||
    fail "update mode does not reconcile Homebrew completions"
grep -F 'if ! link_homebrew_completions; then' "$REPO_ROOT/install-deps.sh" >/dev/null ||
    fail "install mode does not reconcile Homebrew completions"

if [[ "$FAIL" -ne 0 ]]; then exit 1; fi
echo "all Homebrew completion reconciliation invariants OK"
