#!/usr/bin/env bash
# Nix + tooling migration architecture guard (CLAUDE.md invariant 22).
#
# The Nix layer (nix-darwin + declarative Homebrew on macOS, Home Manager
# standalone on Linux/WSL userland) is a PACKAGE provider only. chezmoi is the
# single owner of every dotfile target on every OS, and native Windows stays
# non-Nix. This test enforces that boundary statically so a future change cannot
# quietly let Nix/Home Manager start owning config files or Windows-host paths.
#
# It is deliberately written to PASS on a repo with zero `.nix` files (every
# nix-file scan simply finds nothing) and to start enforcing the moment the
# flake / darwin / home-manager modules land.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

fail=0

# Collect every tracked-ish .nix file, excluding VCS and generated caches.
nix_files=()
while IFS= read -r -d '' f; do nix_files+=("$f"); done < <(
    find . \( -path './.git' -o -path './tests/.cache' \) -prune -o \
        -type f -name '*.nix' -print0
)

scan_nix() {
    # scan_nix "<description>" "<extended-regex>"  -> fail if any nix file matches
    local desc="$1" pattern="$2"
    if [[ "${#nix_files[@]}" -eq 0 ]]; then
        echo "ok  : $desc (no .nix files present yet)"
        return
    fi
    if grep -nE "$pattern" "${nix_files[@]}" >/dev/null 2>&1; then
        echo "FAIL: $desc"
        grep -nE "$pattern" "${nix_files[@]}" | head -8 | sed 's/^/  /'
        fail=1
    else
        echo "ok  : $desc"
    fi
}

# ---------------------------------------------------------------------------
# (a)+(b) Home Manager / nix-darwin is PACKAGES-ONLY. No dotfile ownership.
#   - No home.file / xdg.configFile / xdg.dataFile / xdg.desktopEntries: those
#     render files into $HOME, which is chezmoi's exclusive territory.
#   - No programs.<tool> config-generating module for a chezmoi-owned tool. HM's
#     programs.zsh/programs.neovim/etc. write ~/.zshrc, ~/.config/nvim/init.lua,
#     and friends -- exactly the files chezmoi owns. programs.home-manager
#     (which only manages HM itself) is the one allowed programs.* module.
# ---------------------------------------------------------------------------
if ! command -v python3 >/dev/null 2>&1; then
    echo "FAIL: python3 is required for the structural Nix ownership guard"
    fail=1
elif ! python3 scripts/check-nix-dotfile-ownership.py --self-test; then
    echo "FAIL: Nix ownership scanner self-test failed"
    fail=1
elif ! ownership_hits="$(python3 scripts/check-nix-dotfile-ownership.py "${nix_files[@]}" 2>&1)"; then
    echo "FAIL: HM/darwin declares a dotfile-owning option outside chezmoi"
    printf '%s\n' "$ownership_hits" | head -8 | sed 's/^/  /'
    fail=1
else
    echo "ok  : structural scan rejects direct, nested, wrapped, and imported dotfile ownership"
fi

# ---------------------------------------------------------------------------
# (c) native Windows is NON-NIX. Nix applies to WSL2 userland only, never to
#     Windows-host paths, and Windows setup never invokes Nix.
# ---------------------------------------------------------------------------
scan_nix "no .nix file references a Windows-host path (/mnt/c, C:\\, %USERPROFILE%, AppData)" \
    '(/mnt/c/|%USERPROFILE%|[A-Za-z]:\\|[\\/]AppData[\\/])'

win_setup_files=(setup.ps1 install-deps.ps1)
win_nix_pattern='(^|[^[:alnum:]_/-])(darwin-rebuild|home-manager|nix-env|nix-shell|nix-channel|nix-daemon)([^[:alnum:]_-]|$)|(^|[^[:alnum:]_/-])nix[[:space:]]+(profile|flake|build|run|develop|shell)([^[:alnum:]_-]|$)'
present_win_files=()
for f in "${win_setup_files[@]}"; do [[ -f "$f" ]] && present_win_files+=("$f"); done
if [[ "${#present_win_files[@]}" -eq 0 ]]; then
    echo "FAIL: expected Windows setup entry points not found"
    fail=1
elif grep -nE "$win_nix_pattern" "${present_win_files[@]}" >/dev/null 2>&1; then
    echo "FAIL: native Windows setup must not invoke Nix / darwin-rebuild / home-manager"
    grep -nE "$win_nix_pattern" "${present_win_files[@]}" | head -8 | sed 's/^/  /'
    fail=1
else
    echo "ok  : native Windows setup does not invoke Nix"
fi

# ---------------------------------------------------------------------------
# (d) no remote-eval Nix installer anywhere in repo CODE. The proving host was
#     bootstrapped from the notarized Determinate .pkg (provenance-checked, not
#     piped); repo code must never carry a mutable pipe-to-shell nix installer.
#     (Docs may mention Determinate for provenance -- .md files are not scanned.)
# ---------------------------------------------------------------------------
installer_code=()
for f in install-deps.sh setup.sh install-deps.ps1 setup.ps1; do
    [[ -f "$f" ]] && installer_code+=("$f")
done
while IFS= read -r f; do installer_code+=("$f"); done < <(
    find scripts -type f \( -name '*.sh' -o -name '*.ps1' \) 2>/dev/null | sort
)
remote_nix_pattern='install\.determinate\.systems|nixos\.org/nix/install|determinate\.systems/nix'
if [[ "${#installer_code[@]}" -eq 0 ]]; then
    echo "ok  : no installer code files to scan for remote-eval Nix"
elif grep -nE "$remote_nix_pattern" "${installer_code[@]}" >/dev/null 2>&1; then
    echo "FAIL: repo installer code must not fetch/execute a remote Nix installer"
    grep -nE "$remote_nix_pattern" "${installer_code[@]}" | head -8 | sed 's/^/  /'
    fail=1
else
    echo "ok  : no remote-eval Nix installer in repo installer code"
fi

# ---------------------------------------------------------------------------
# (e) update mode must not run a blanket Nix upgrade or silently rewrite
#     flake.lock. No installer code may RUN `nix profile upgrade`, `nix-env -u`,
#     `nix flake update`, or `nix flake lock --update-input`. Opt-in activation
#     switches are fine when they use installed tools or flake.lock-pinned
#     bootstrap refs; they are not blanket package upgrades or lock rewrites.
#     (Comments that merely describe the ban are allowed.)
# ---------------------------------------------------------------------------
blanket_nix_pattern='nix[[:space:]]+profile[[:space:]]+upgrade|nix-env[[:space:]]+([^#]*[[:space:]])?-u|nix[[:space:]]+flake[[:space:]]+update|nix[[:space:]]+flake[[:space:]]+lock[[:space:]][^#]*--(update-input|recreate-lock-file)'
blanket_hits=""
for f in install-deps.sh setup.sh install-deps.ps1 setup.ps1; do
    [[ -f "$f" ]] || continue
    while IFS= read -r line; do
        blanket_hits+="$f:$line"$'\n'
    done < <(grep -nE "$blanket_nix_pattern" "$f" 2>/dev/null | grep -vE '^[0-9]+:[[:space:]]*#')
done
if [[ -n "${blanket_hits//[[:space:]]/}" ]]; then
    echo "FAIL: installer code runs a blanket Nix upgrade / silent flake.lock rewrite:"
    printf '%s' "$blanket_hits" | sed 's/^/  /'
    fail=1
else
    echo "ok  : no blanket nix upgrade / silent flake.lock rewrite in installer code"
fi

mutable_bootstrap_pattern='nix[[:space:]]+run[[:space:]]+(nix-darwin|home-manager)([[:space:]]|$)'
mutable_bootstrap_hits=""
for f in install-deps.sh setup.sh install-deps.ps1 setup.ps1; do
    [[ -f "$f" ]] || continue
    while IFS= read -r line; do
        mutable_bootstrap_hits+="$f:$line"$'\n'
    done < <(grep -nE "$mutable_bootstrap_pattern" "$f" 2>/dev/null | grep -vE '^[0-9]+:[[:space:]]*#')
done
if [[ -n "${mutable_bootstrap_hits//[[:space:]]/}" ]]; then
    echo "FAIL: installer code uses mutable Nix registry aliases for bootstrap:"
    printf '%s' "$mutable_bootstrap_hits" | sed 's/^/  /'
    fail=1
else
    echo "ok  : Nix bootstrap commands use locked flake refs, not mutable registry aliases"
fi

wsl_harness=tests/greenfield/wsl-greenfield.ps1
if ! grep -Fq "nix-bin" "$wsl_harness"; then
    echo "FAIL: $wsl_harness must install Ubuntu's nix-bin before WSL setup.sh validation"
    fail=1
fi
if ! grep -Fq "experimental-features = nix-command flakes" "$wsl_harness"; then
    echo "FAIL: $wsl_harness must enable Nix flakes before WSL setup.sh validation"
    fail=1
fi
if [[ "$fail" -eq 0 ]]; then
    echo "ok  : manual WSL validation surface provisions Nix before setup.sh"
fi

for snippet in \
    'flake_lock_github_nar_hash' \
    'nix_flake_ref_query_encode' \
    '?narHash=%s#darwin-rebuild' \
    '?narHash=%s#home-manager'; do
    if ! grep -Fq "$snippet" setup.sh; then
        echo "FAIL: setup.sh missing Nix bootstrap narHash guard snippet: $snippet"
        fail=1
    fi
done
if [[ "$fail" -eq 0 ]]; then
    echo "ok  : Nix bootstrap refs include locked narHash query parameters"
fi

[[ "$fail" -eq 0 ]] || exit 1
echo "all nix-architecture invariants OK"
