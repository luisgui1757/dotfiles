#!/usr/bin/env bash
# Cross-file pin-consistency guard.
#
# Several version/checksum/commit pins are MIRRORED across multiple files that
# Renovate (or a human) can bump independently. Renovate cannot recompute a
# SHA-256 or propagate a value into a sibling file, so a bump updates one surface
# and silently strands the others until CI catches it -- which only happens if a
# test like this exists. This fired for real: the nvim v0.12.2 -> v0.12.3 bump
# updated install-deps.sh + test.yml but stranded the version+SHA hardcoded in
# tests/shell/install_nvim_linux{,_fail}_test.sh.
#
# This test is the canonical drift guard: it extracts each pin from every mirror
# and asserts they agree. It needs no network and no tools beyond coreutils/grep.
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT" || exit 1

fail=0
assert_eq() {
    local name="$1" a="$2" b="$3"
    if [[ -z "$a" || -z "$b" ]]; then
        echo "FAIL: $name -- could not extract a value (a='$a' b='$b'); update the extractor"
        fail=1
    elif [[ "$a" != "$b" ]]; then
        echo "FAIL: $name DRIFT -- '$a' != '$b'"
        fail=1
    else
        echo "ok  : $name = $a"
    fi
}

sh_const() { grep -E "^$1=" install-deps.sh | head -1 | cut -d'"' -f2; }
yml_const() { awk -v key="$1" '$1 == key ":" { print $2; exit }' .github/workflows/test.yml; }

# --- nvim Linux: install-deps.sh <-> test.yml <-> the two install tests -------
nvim_ver_sh="$(sh_const NVIM_LINUX_VERSION)"
nvim_x86_sh="$(sh_const NVIM_LINUX_X86_64_SHA256)"
nvim_ver_yml="$(grep -E '^\s*NVIM_LINUX_VERSION:' .github/workflows/test.yml | awk '{print $2}')"
nvim_x86_yml="$(grep -E '^\s*NVIM_LINUX_X86_64_SHA256:' .github/workflows/test.yml | awk '{print $2}')"
nvim_ver_test="$(grep -oE 'download/v[0-9][0-9.]*/nvim-linux' tests/shell/install_nvim_linux_test.sh | grep -oE 'v[0-9][0-9.]*' | head -1)"
nvim_sha_test="$(grep -oE '[0-9a-f]{64}' tests/shell/install_nvim_linux_test.sh | head -1)"
nvim_sha_fail="$(grep -oE '[0-9a-f]{64}' tests/shell/install_nvim_linux_fail_test.sh | head -1)"

assert_eq "nvim version (install-deps.sh == test.yml)"            "$nvim_ver_sh"  "$nvim_ver_yml"
assert_eq "nvim x86_64 SHA (install-deps.sh == test.yml)"         "$nvim_x86_sh"  "$nvim_x86_yml"
assert_eq "nvim version (install-deps.sh == install_nvim test)"   "$nvim_ver_sh"  "$nvim_ver_test"
assert_eq "nvim x86_64 SHA (install-deps.sh == install_nvim test)" "$nvim_x86_sh" "$nvim_sha_test"
assert_eq "nvim x86_64 SHA (install-deps.sh == nvim FAIL test)"    "$nvim_x86_sh" "$nvim_sha_fail"

# --- mirrored direct-download pins: install-deps.sh <-> required Ubuntu CI ---
assert_eq "chezmoi version (install-deps.sh == test.yml)" \
    "$(sh_const CHEZMOI_VERSION)" "$(yml_const CHEZMOI_VERSION)"
assert_eq "chezmoi x86_64 SHA (install-deps.sh == test.yml)" \
    "$(sh_const CHEZMOI_LINUX_X86_64_SHA256)" "$(yml_const CHEZMOI_LINUX_X86_64_SHA256)"
assert_eq "chezmoi arm64 SHA (install-deps.sh == test.yml)" \
    "$(sh_const CHEZMOI_LINUX_ARM64_SHA256)" "$(yml_const CHEZMOI_LINUX_ARM64_SHA256)"
assert_eq "starship version (install-deps.sh == test.yml)" \
    "$(sh_const STARSHIP_VERSION)" "$(yml_const STARSHIP_VERSION)"
assert_eq "starship x86_64 SHA (install-deps.sh == test.yml)" \
    "$(sh_const STARSHIP_LINUX_X86_64_SHA256)" "$(yml_const STARSHIP_LINUX_X86_64_SHA256)"
assert_eq "tree-sitter version (install-deps.sh == test.yml)" \
    "$(sh_const TREE_SITTER_CLI_LINUX_VERSION)" "$(yml_const TREE_SITTER_CLI_LINUX_VERSION)"
assert_eq "tree-sitter x86_64 SHA (install-deps.sh == test.yml)" \
    "$(sh_const TREE_SITTER_CLI_LINUX_X86_64_SHA256)" "$(yml_const TREE_SITTER_CLI_LINUX_X86_64_SHA256)"

# --- zsh plugins: install-deps.sh <-> chezmoi external (tag) <-> verify (commit)
ext_tag() { grep -A4 "$1" home/.chezmoiexternal.toml.tmpl | grep -oE '"v[0-9][0-9.]*"' | tr -d '"' | head -1; }
verify_commit() { grep -oE "$1 [0-9a-f]{40}" home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl | awk '{print $2}' | head -1; }

assert_eq "fzf-tab tag (install-deps.sh == chezmoi external)" \
    "$(sh_const FZF_TAB_VERSION)" "$(ext_tag 'Aloxaf/fzf-tab.git')"
assert_eq "fzf-tab commit (install-deps.sh == verify script)" \
    "$(sh_const FZF_TAB_COMMIT)" "$(verify_commit 'fzf-tab')"
assert_eq "zsh-autosuggestions tag (install-deps.sh == chezmoi external)" \
    "$(sh_const ZSH_AUTOSUGGESTIONS_VERSION)" "$(ext_tag 'zsh-users/zsh-autosuggestions.git')"
assert_eq "zsh-autosuggestions commit (install-deps.sh == verify script)" \
    "$(sh_const ZSH_AUTOSUGGESTIONS_COMMIT)" "$(verify_commit 'zsh-autosuggestions')"

if [[ "$fail" -ne 0 ]]; then
    echo "pin consistency: DRIFT detected -- recompute/propagate the mirrored pin(s)."
    exit 1
fi
echo "pin consistency: OK"
