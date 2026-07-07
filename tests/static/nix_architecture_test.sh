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
while IFS= read -r f; do nix_files+=("$f"); done < <(
    find . -type f -name '*.nix' \
        -not -path './.git/*' \
        -not -path './tests/.cache/*' \
        | sort
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
# The trailing (\.|=|"|\{) requirement makes these match real attribute usage
# (home.file."x" = ..., xdg.configFile.foo.text = ...) but NOT documenting prose
# like "declares no home.file / xdg.configFile" in a comment.
scan_nix "HM/darwin declares no home.file dotfiles (packages-only)" \
    '(^|[^[:alnum:]_.])home\.file[[:space:]]*(\.|=|"|\{)'
scan_nix "HM/darwin declares no xdg.configFile / xdg.dataFile / xdg.desktopEntries" \
    '(^|[^[:alnum:]_.])xdg\.(configFile|dataFile|desktopEntries)[[:space:]]*(\.|=|"|\{)'
scan_nix "HM/darwin declares no config-generating programs.<tool> for chezmoi-owned tools" \
    '(^|[^[:alnum:]_.])programs\.(zsh|bash|fish|nushell|neovim|vim|tmux|starship|git|kitty|alacritty|wezterm|ghostty|zoxide|fzf|lsd|eza|bat|direnv|gh|lazygit|readline)([.[:space:]=]|$)'
scan_nix "HM/darwin writes no config via home.activation file emission" \
    '(^|[^[:alnum:]_.])home\.activation[[:space:]]*(\.|=|"|\{).*(writeText|\.config|\.zshrc|dotfile)'

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
#     `nix flake update`, or `nix flake lock --update-input`. The opt-in switches
#     (`nix run nix-darwin -- switch`, `home-manager switch`) are fine; those are
#     activation, not a blanket package upgrade or a lock rewrite. (Comments that
#     merely describe the ban are allowed.)
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

[[ "$fail" -eq 0 ]] || exit 1
echo "all nix-architecture invariants OK"
