#!/usr/bin/env bash
# Homebrew must own and reconcile the completion links consumed by new shells,
# including its core `_brew` link that `brew completions link` does not repair.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

FAIL=0
LINK_RUNS=0
fail() { echo "FAIL: $*" >&2; FAIL=1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

brew() {
    case "$*" in
        "completions link")
            LINK_RUNS=$((LINK_RUNS + 1))
            return "${BREW_RC:-0}"
            ;;
        --prefix) printf '%s\n' "$BREW_PREFIX" ;;
        --repository) printf '%s\n' "$BREW_REPOSITORY" ;;
        *)
            echo "unexpected brew arguments: $*" >&2
            return 97
            ;;
    esac
}

PM=apt
DRY_RUN=0
link_homebrew_completions || fail "non-Homebrew hosts should be a no-op"
[[ "$LINK_RUNS" -eq 0 ]] || fail "non-Homebrew hosts invoked brew"

PM=brew
DRY_RUN=1
preview="$(link_homebrew_completions)" || fail "dry-run preview failed"
[[ "$preview" == *"would: brew completions link + verify core _brew"* ]] || fail "dry-run did not preview the full reconciliation"
[[ "$LINK_RUNS" -eq 0 ]] || fail "dry-run invoked brew"

DRY_RUN=0
BREW_RC=0
BREW_PREFIX="$WORK/standard"
BREW_REPOSITORY="$BREW_PREFIX"
mkdir -p "$BREW_PREFIX/completions/zsh" "$BREW_PREFIX/share/zsh/site-functions"
printf '%s\n' '# standard Homebrew completion' > "$BREW_PREFIX/completions/zsh/_brew"
ln -s "$WORK/removed/_brew" "$BREW_PREFIX/share/zsh/site-functions/_brew"
link_homebrew_completions >/dev/null || fail "standard Homebrew reconciliation failed"
[[ "$LINK_RUNS" -eq 1 ]] || fail "successful reconciliation did not invoke brew exactly once"
[[ -r "$BREW_PREFIX/share/zsh/site-functions/_brew" ]] || fail "dangling core completion was not repaired"
[[ "$(real_source_path "$BREW_PREFIX/share/zsh/site-functions/_brew")" == \
    "$(real_source_path "$BREW_PREFIX/completions/zsh/_brew")" ]] ||
    fail "standard core completion resolves to the wrong source"

# nix-homebrew reports a marker repository, while Library/Homebrew points into
# the active package generation. The published link must follow that stable
# logical path instead of embedding the current Nix store generation directly.
BREW_PREFIX="$WORK/nix-homebrew"
BREW_REPOSITORY="$BREW_PREFIX/Library/.homebrew-is-managed-by-nix"
NIX_PACKAGE="$WORK/nix/store/brew-package"
mkdir -p "$BREW_REPOSITORY" "$BREW_PREFIX/Library" \
    "$NIX_PACKAGE/Library/Homebrew" "$NIX_PACKAGE/completions/zsh"
printf '%s\n' '# nix-homebrew completion' > "$NIX_PACKAGE/completions/zsh/_brew"
ln -s "$NIX_PACKAGE/Library/Homebrew" "$BREW_PREFIX/Library/Homebrew"
link_homebrew_completions >/dev/null || fail "nix-homebrew reconciliation failed"
[[ "$LINK_RUNS" -eq 2 ]] || fail "nix-homebrew reconciliation did not invoke brew exactly once"
core_link="$BREW_PREFIX/share/zsh/site-functions/_brew"
[[ -r "$core_link" ]] || fail "nix-homebrew core completion was not published"
[[ "$(real_source_path "$core_link")" == "$(real_source_path "$NIX_PACKAGE/completions/zsh/_brew")" ]] ||
    fail "nix-homebrew core completion resolves outside the active package"
[[ "$(readlink "$core_link")" == "$BREW_PREFIX/Library/Homebrew/../../completions/zsh/_brew" ]] ||
    fail "nix-homebrew link embedded a generation-specific source"

BREW_RC=23
if link_homebrew_completions >/dev/null 2>&1; then
    fail "completion-link failure was accepted"
fi
[[ "$LINK_RUNS" -eq 3 ]] || fail "failed reconciliation did not invoke brew exactly once"

BREW_RC=0
BREW_PREFIX="$WORK/missing-source"
BREW_REPOSITORY="$BREW_PREFIX"
mkdir -p "$BREW_PREFIX"
if link_homebrew_completions >/dev/null 2>&1; then
    fail "missing core completion source was accepted"
fi
[[ "$LINK_RUNS" -eq 4 ]] || fail "missing-source reconciliation did not invoke brew exactly once"

BREW_PREFIX="$WORK/missing-repository"
BREW_REPOSITORY=""
mkdir -p "$BREW_PREFIX/completions/zsh"
printf '%s\n' '# active source' > "$BREW_PREFIX/completions/zsh/_brew"
if link_homebrew_completions >/dev/null 2>&1; then
    fail "missing Homebrew repository identity was accepted"
fi
[[ "$LINK_RUNS" -eq 5 ]] || fail "missing-repository reconciliation did not invoke brew exactly once"

BREW_PREFIX="$WORK/non-symlink-conflict"
BREW_REPOSITORY="$BREW_PREFIX"
mkdir -p "$BREW_PREFIX/completions/zsh" "$BREW_PREFIX/share/zsh/site-functions"
printf '%s\n' '# active source' > "$BREW_PREFIX/completions/zsh/_brew"
printf '%s\n' 'preserve me' > "$BREW_PREFIX/share/zsh/site-functions/_brew"
if link_homebrew_completions >/dev/null 2>&1; then
    fail "non-symlink completion conflict was overwritten"
fi
[[ "$(cat "$BREW_PREFIX/share/zsh/site-functions/_brew")" == "preserve me" ]] ||
    fail "non-symlink completion conflict was not preserved"
[[ "$LINK_RUNS" -eq 6 ]] || fail "conflict reconciliation did not invoke brew exactly once"

grep -F "elif [[ \"\$PM\" == \"brew\" ]] && ! link_homebrew_completions" "$REPO_ROOT/install-deps.sh" >/dev/null ||
    fail "update mode does not reconcile Homebrew completions"
grep -F 'if ! link_homebrew_completions; then' "$REPO_ROOT/install-deps.sh" >/dev/null ||
    fail "install mode does not reconcile Homebrew completions"

if [[ "$FAIL" -ne 0 ]]; then exit 1; fi
echo "all Homebrew completion reconciliation invariants OK"
