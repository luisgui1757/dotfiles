#!/usr/bin/env bash
# Assert the nix-darwin config renders the required declarative-Homebrew knobs
# (the migration ruling): WezTerm + AeroSpace casks, Herdr brew, no auto
# update/upgrade, cleanup = "check" on real hosts, hosted-CI cleanup override,
# autoMigrate = true, mutableTaps = false, and Determinate owns the daemon
# (nix.enable = false).
# Uses cross-platform pure `nix eval`; skips gracefully when nix is unavailable
# in a local shell, while CI installs Nix and runs this as real enforcement
# coverage.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v nix >/dev/null 2>&1 && [ -e /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh ]; then
    # shellcheck disable=SC1091
    . /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
fi
if ! command -v nix >/dev/null 2>&1; then
    echo "SKIP: nix not installed (nix-darwin config eval)"
    exit 0
fi
command -v jq >/dev/null 2>&1 || {
    echo "SKIP: jq not installed"
    exit 0
}

cfg='.#darwinConfigurations.dotfiles.config'
fail=0

# 2>/dev/null suppresses the harmless "Git tree is dirty" warning on stderr.
eval_json() { nix eval --json "$cfg.$1" 2>/dev/null; }
eval_raw() { nix eval "$cfg.$1" 2>/dev/null; }

assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [[ "$expected" == "$actual" ]]; then
        echo "ok  : $desc"
    else
        echo "FAIL: $desc (expected [$expected], got [$actual])"
        fail=1
    fi
}

casks="$(eval_json homebrew.casks | jq -r '[.[].name] | sort | join(",")')"
assert_eq "declarative casks are aerospace + wezterm (vendor channel, not nixpkgs)" \
    "aerospace,wezterm" "$casks"

brews="$(eval_json homebrew.brews | jq -r '[.[].name] | sort | join(",")')"
assert_eq "declarative brews include herdr" "herdr" "$brews"

assert_eq "homebrew cleanup = check (report drift, non-destructive)" '"check"' "$(eval_raw homebrew.onActivation.cleanup)"
ci_cleanup="$(env DOTFILES_NIX_DARWIN_HOSTED_CI=1 nix eval --raw --impure "$cfg.homebrew.onActivation.cleanup" 2>/dev/null || true)"
assert_eq "hosted CI disables Homebrew cleanup check for disposable runners" "none" "$ci_cleanup"
assert_eq "homebrew autoUpdate disabled" 'false' "$(eval_raw homebrew.onActivation.autoUpdate)"
assert_eq "homebrew upgrade disabled" 'false' "$(eval_raw homebrew.onActivation.upgrade)"
assert_eq "nix-homebrew autoMigrate adopts existing Homebrew installs" 'true' "$(eval_raw nix-homebrew.autoMigrate)"
assert_eq "nix-homebrew mutableTaps = false (pinned taps)" 'false' "$(eval_raw nix-homebrew.mutableTaps)"
trusted_taps="$(eval_json nix-homebrew.trust.taps | jq -r 'sort | join(",")')"
assert_eq "nix-homebrew trusts the pinned AeroSpace tap for Homebrew 5 cask loading" "nikitabobko/tap" "$trusted_taps"
assert_eq "nix.enable = false (Determinate owns the daemon)" 'false' "$(eval_raw nix.enable)"
assert_eq "system.primaryUser is set (placeholder in pure eval)" '"runner"' "$(eval_raw system.primaryUser)"

actual_system="$(nix eval --raw '.#darwinConfigurations.dotfiles-aarch64.pkgs.stdenv.hostPlatform.system' 2>/dev/null || true)"
assert_eq "dotfiles-aarch64 evaluates only Apple Silicon" "aarch64-darwin" "$actual_system"
alias_system="$(nix eval --raw '.#darwinConfigurations.dotfiles.pkgs.stdenv.hostPlatform.system' 2>/dev/null || true)"
assert_eq "compatibility alias deliberately remains Apple Silicon" "aarch64-darwin" "$alias_system"
config_names="$(nix eval --json '.#darwinConfigurations' --apply 'configs: builtins.attrNames configs' 2>/dev/null | jq -r 'sort | join(",")' || true)"
assert_eq "Darwin exports contain no retired Intel configuration" "dotfiles,dotfiles-aarch64" "$config_names"

target_cfg='.#darwinConfigurations.dotfiles-aarch64.config'
target_user="$(env DOTFILES_TARGET_USER=alice DOTFILES_TARGET_HOME='/Users/Alice Example' \
    SUDO_USER=wrong USER=root nix eval --raw --impure "$target_cfg.system.primaryUser" 2>/dev/null || true)"
assert_eq "validated setup target overrides ambient sudo/user identities" "alice" "$target_user"
target_home="$(env DOTFILES_TARGET_USER=alice DOTFILES_TARGET_HOME='/Users/Alice Example' \
    SUDO_USER=wrong USER=root nix eval --raw --impure "$target_cfg.users.users.alice.home" 2>/dev/null || true)"
assert_eq "validated target home supports spaces without fabrication" "/Users/Alice Example" "$target_home"
target_hm_user="$(env DOTFILES_TARGET_USER=alice DOTFILES_TARGET_HOME='/Users/Alice Example' \
    SUDO_USER=wrong USER=root nix eval --raw --impure "$target_cfg.home-manager.users.alice.home.username" 2>/dev/null || true)"
assert_eq "validated target identity drives Home Manager" "alice" "$target_hm_user"

# Home Manager on darwin is packages-only: the user's home.packages is a
# non-empty list. (The stronger "our source declares NO home.file / xdg.configFile
# / programs.<tool>" boundary is enforced at the SOURCE level by
# tests/static/nix_architecture_test.sh -- a runtime home.file count would only
# see Home Manager's own internal session-vars machinery, not our config.)
hm="$cfg.home-manager.users.runner"
pkgcount="$(nix eval "$hm.home.packages" --apply 'builtins.length' 2>/dev/null || echo 0)"
if [[ "${pkgcount:-0}" -ge 1 ]]; then
    echo "ok  : Home Manager (darwin) declares a non-empty home.packages set ($pkgcount)"
else
    echo "FAIL: Home Manager (darwin) home.packages is empty ($pkgcount)"
    fail=1
fi
names="$(nix eval --json "$hm.home.packages" --apply 'ps: map (p: p.pname or p.name or "") ps' 2>/dev/null | jq -r '.[]' 2>/dev/null || echo "")"
if printf '%s\n' "$names" | grep -qx 'nodejs'; then
    echo "ok  : Home Manager (darwin) includes nodejs (Node 24 runtime for pinned npm-backed Pi CLI)"
else
    echo "FAIL: Home Manager (darwin) does not include nodejs for the Pi CLI npm runtime"
    fail=1
fi

[[ "$fail" -eq 0 ]] && echo "all nix-darwin config assertions OK"
exit "$fail"
