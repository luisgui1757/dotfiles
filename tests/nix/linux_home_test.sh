#!/usr/bin/env bash
# Assert the standalone Linux/WSL Home Manager config is packages-only and stays
# split-host on WSL: a non-empty home.packages set, homeDirectory under the Linux
# /home (never a Windows-host /mnt/c path), and the one allowed programs.* module
# (programs.home-manager, the standalone CLI). Uses cross-platform pure `nix eval`;
# skips gracefully without nix.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

if ! command -v nix >/dev/null 2>&1 && [ -e /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh ]; then
    # shellcheck disable=SC1091
    . /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
fi
if ! command -v nix >/dev/null 2>&1; then
    echo "SKIP: nix not installed (linux home-manager config eval)"
    exit 0
fi

fail=0
assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [[ "$expected" == "$actual" ]]; then
        echo "ok  : $desc"
    else
        echo "FAIL: $desc (expected [$expected], got [$actual])"
        fail=1
    fi
}

for arch in x86_64-linux aarch64-linux; do
    cfg=".#homeConfigurations.\"$arch\".config"
    pkgcount="$(nix eval "$cfg.home.packages" --apply 'builtins.length' 2>/dev/null || echo 0)"
    if [[ "${pkgcount:-0}" -ge 1 ]]; then
        echo "ok  : $arch home.packages is non-empty ($pkgcount)"
    else
        echo "FAIL: $arch home.packages is empty ($pkgcount)"
        fail=1
    fi

    homedir="$(nix eval --raw "$cfg.home.homeDirectory" 2>/dev/null || echo "")"
    case "$homedir" in
        /home/*) echo "ok  : $arch homeDirectory is a Linux /home path ($homedir)" ;;
        *)
            echo "FAIL: $arch homeDirectory is not under /home ($homedir)"
            fail=1
            ;;
    esac
    case "$homedir" in
        /mnt/c/* | *:* )
            echo "FAIL: $arch homeDirectory points at a Windows-host path ($homedir)"
            fail=1
            ;;
        *) echo "ok  : $arch homeDirectory is not a Windows-host path (split-host preserved)" ;;
    esac

    assert_eq "$arch enables the standalone programs.home-manager CLI" \
        'true' "$(nix eval "$cfg.programs.home-manager.enable" 2>/dev/null)"

    # nvim + the tree-sitter CLI are ABI-coupled to nvim-treesitter parser builds
    # and are intentionally DEFERRED (stay native). Prove they are NOT in the set.
    names="$(nix eval --json "$cfg.home.packages" --apply 'ps: map (p: p.pname or p.name or "") ps' 2>/dev/null | jq -r '.[]' 2>/dev/null || echo "")"
    if printf '%s\n' "$names" | grep -qiE '(^|[^[:alnum:]])(neovim|nvim|tree-sitter)([^[:alnum:]]|$)'; then
        echo "FAIL: $arch HM set includes an ABI-coupled deferred tool (neovim/tree-sitter)"
        fail=1
    else
        echo "ok  : $arch HM set excludes nvim + tree-sitter (deferred, stay native)"
    fi
done

[[ "$fail" -eq 0 ]] && echo "all linux home-manager config assertions OK"
exit "$fail"
