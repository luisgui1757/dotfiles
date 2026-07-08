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

assert_contains() {
    local name="$1" file="$2" needle="$3"
    if [[ -z "$needle" ]]; then
        echo "FAIL: $name -- empty expected value; update the extractor"
        fail=1
    elif ! grep -F -- "$needle" "$file" >/dev/null; then
        echo "FAIL: $name -- '$file' does not document '$needle'"
        fail=1
    else
        echo "ok  : $name documents $needle"
    fi
}

sh_const() { grep -E "^$1=" install-deps.sh | head -1 | cut -d'"' -f2; }
yml_const() { awk -v key="$1" '$1 == key ":" { print $2; exit }' .github/workflows/test.yml; }
setup_sh_const() { grep -E "^$1=" setup.sh | head -1 | cut -d'"' -f2; }
setup_ps_const() { grep -E "^[[:space:]]*\\\$$1[[:space:]]*=" setup.ps1 | head -1 | cut -d"'" -f2; }

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

# --- tmux/psmux plugin pins: installers <-> docs ------------------------------
# POSIX: TPM + the functional plugins (sensible/yank/resurrect/continuum) in
# install-deps.sh. Windows: the psmux/psmux-plugins monorepo commit in
# install-deps.ps1 (vendored resurrect/continuum). The Rose Pine bar is a
# repo-owned generated config, NOT a plugin, so rose-pine/tmux is retired.
ps_const() { grep -E "^[[:space:]]*\\\$$1[[:space:]]*=" install-deps.ps1 | head -1 | cut -d"'" -f2; }
tpm_commit="$(sh_const TPM_COMMIT)"
psmux_plugins_commit="$(ps_const PsmuxPluginsCommit)"

for pin_name in TPM_COMMIT TMUX_SENSIBLE_COMMIT TMUX_YANK_COMMIT TMUX_RESURRECT_COMMIT TMUX_CONTINUUM_COMMIT; do
    pin_value="$(sh_const "$pin_name")"
    if [[ ! "$pin_value" =~ ^[0-9a-f]{40}$ ]]; then
        echo "FAIL: $pin_name must be an immutable 40-character lowercase commit SHA, got '$pin_value'"
        fail=1
    else
        echo "ok  : $pin_name is immutable commit SHA"
    fi
done
if [[ ! "$psmux_plugins_commit" =~ ^[0-9a-f]{40}$ ]]; then
    echo "FAIL: PsmuxPluginsCommit must be an immutable 40-character lowercase commit SHA, got '$psmux_plugins_commit'"
    fail=1
else
    echo "ok  : PsmuxPluginsCommit is immutable commit SHA"
fi

# rose-pine/tmux must stay fully retired from the pin surface.
if grep -q 'ROSE_PINE_TMUX_COMMIT' install-deps.sh; then
    echo "FAIL: ROSE_PINE_TMUX_COMMIT must be removed from install-deps.sh (rose-pine/tmux is retired)"
    fail=1
else
    echo "ok  : rose-pine/tmux pin retired"
fi

# Docs mirror the plugin provisioning pins.
for doc in README.md CLAUDE.md; do
    assert_contains "TPM commit ($doc)" "$doc" "$tpm_commit"
    assert_contains "tmux-sensible commit ($doc)" "$doc" "$(sh_const TMUX_SENSIBLE_COMMIT)"
    assert_contains "tmux-yank commit ($doc)" "$doc" "$(sh_const TMUX_YANK_COMMIT)"
    assert_contains "tmux-resurrect commit ($doc)" "$doc" "$(sh_const TMUX_RESURRECT_COMMIT)"
    assert_contains "tmux-continuum commit ($doc)" "$doc" "$(sh_const TMUX_CONTINUUM_COMMIT)"
    assert_contains "psmux-plugins commit ($doc)" "$doc" "$psmux_plugins_commit"
done

# --- gh-dash pinned extension tag: install-deps.sh <-> install-deps.ps1 <-> docs
gh_dash_sh="$(sh_const GH_DASH_VERSION)"
gh_dash_ps="$(ps_const GhDashVersion)"
assert_eq "gh-dash tag (install-deps.sh == install-deps.ps1)" "$gh_dash_sh" "$gh_dash_ps"
assert_contains "gh-dash tag (CLAUDE.md)" "CLAUDE.md" "$gh_dash_sh"

# --- Pi CLI pinned npm package: install-deps.sh <-> install-deps.ps1 <-> docs
pi_cli_version_sh="$(sh_const PI_CLI_VERSION)"
pi_cli_integrity_sh="$(sh_const PI_CLI_INTEGRITY)"
pi_cli_version_ps="$(ps_const PiCliVersion)"
pi_cli_integrity_ps="$(ps_const PiCliIntegrity)"
assert_eq "Pi CLI version (install-deps.sh == install-deps.ps1)" "$pi_cli_version_sh" "$pi_cli_version_ps"
assert_eq "Pi CLI integrity (install-deps.sh == install-deps.ps1)" "$pi_cli_integrity_sh" "$pi_cli_integrity_ps"
assert_contains "Pi CLI version (README.md)" "README.md" "$pi_cli_version_sh"
assert_contains "Pi CLI version (CLAUDE.md)" "CLAUDE.md" "$pi_cli_version_sh"
assert_contains "Pi CLI integrity (CLAUDE.md)" "CLAUDE.md" "$pi_cli_integrity_sh"

# --- Polaris: setup.sh <-> setup.ps1 -----------------------------------------
polaris_version_sh="$(setup_sh_const POLARIS_VERSION)"
polaris_tag_sh="$(setup_sh_const POLARIS_TAG)"
polaris_ref_sh="$(setup_sh_const POLARIS_REF)"
polaris_version_ps="$(setup_ps_const PolarisVersion)"
polaris_tag_ps="$(setup_ps_const PolarisTag)"
polaris_ref_ps="$(setup_ps_const PolarisRef)"

assert_eq "Polaris version (setup.sh == setup.ps1)" "$polaris_version_sh" "$polaris_version_ps"
assert_eq "Polaris tag (setup.sh == setup.ps1)" "$polaris_tag_sh" "$polaris_tag_ps"
assert_eq "Polaris commit (setup.sh == setup.ps1)" "$polaris_ref_sh" "$polaris_ref_ps"
assert_contains "Polaris version (README.md)" "README.md" "$polaris_version_sh"
assert_contains "Polaris tag (README.md)" "README.md" "$polaris_tag_sh"
assert_contains "Polaris commit (README.md)" "README.md" "$polaris_ref_sh"
assert_contains "Polaris version (CLAUDE.md)" "CLAUDE.md" "$polaris_version_sh"
assert_contains "Polaris tag (CLAUDE.md)" "CLAUDE.md" "$polaris_tag_sh"
assert_contains "Polaris commit (CLAUDE.md)" "CLAUDE.md" "$polaris_ref_sh"
if [[ "$polaris_tag_sh" != "v$polaris_version_sh" ]]; then
    echo "FAIL: Polaris tag must match version as v<version>, got version '$polaris_version_sh' and tag '$polaris_tag_sh'"
    fail=1
else
    echo "ok  : Polaris tag matches version"
fi
if [[ ! "$polaris_ref_sh" =~ ^[0-9a-f]{40}$ ]]; then
    echo "FAIL: Polaris ref must be an immutable 40-character lowercase commit SHA, got '$polaris_ref_sh'"
    fail=1
else
    echo "ok  : Polaris ref is immutable commit SHA"
fi

if [[ "$fail" -ne 0 ]]; then
    echo "pin consistency: DRIFT detected -- recompute/propagate the mirrored pin(s)."
    exit 1
fi
echo "pin consistency: OK"
