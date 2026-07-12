#!/usr/bin/env bash
# Install the pinned upstream Nix prerequisite from a checksum-verified release.
# This never executes network bytes before their published digest matches the
# review-pinned digest below.
set -euo pipefail

nix_version="2.34.0"
release_tag="v0.2.0"
official_repo="https://github.com/luisgui1757/dotfiles.git"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
checkout="$(cd "$script_dir/.." && pwd -P)"

usage() {
    echo "usage: $0 --install" >&2
    exit 2
}

[[ "$#" -eq 1 && "$1" == "--install" ]] || usage
[[ "$(id -u)" -ne 0 ]] || {
    echo "FAIL: run as the target non-root user; the reviewed installer invokes sudo when needed." >&2
    exit 1
}

for command_name in git curl tar; do
    command -v "$command_name" >/dev/null 2>&1 || {
        echo "FAIL: $command_name is required." >&2
        exit 1
    }
done

normalize_remote() {
    local remote="$1" normalized
    case "$remote" in
        https://github.com/*) normalized="${remote#https://github.com/}" ;;
        git@github.com:*) normalized="${remote#git@github.com:}" ;;
        ssh://git@github.com/*) normalized="${remote#ssh://git@github.com/}" ;;
        *) return 1 ;;
    esac
    normalized="${normalized%.git}"
    [[ "$normalized" == "luisgui1757/dotfiles" ]]
}

repo_git() {
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_CONFIG_SYSTEM=/dev/null \
    GIT_CONFIG_GLOBAL=/dev/null \
    GIT_CONFIG_COUNT=0 \
    GIT_CONFIG_PARAMETERS='' \
    GIT_TEMPLATE_DIR='' \
        git -C "$checkout" \
        -c core.fsmonitor=false \
        -c core.untrackedCache=false \
        -c core.hooksPath=/dev/null \
        -c init.templateDir= \
        "$@"
}

head_commit="$(repo_git rev-parse --verify 'HEAD^{commit}')"
tag_commit="$(repo_git rev-parse --verify "refs/tags/$release_tag^{commit}")"
tag_object="$(repo_git rev-parse --verify "refs/tags/$release_tag")"
tag_type="$(repo_git cat-file -t "refs/tags/$release_tag")"
origin="$(repo_git remote get-url origin)"
[[ "$head_commit" == "$tag_commit" && "$tag_type" == "tag" ]] || {
    echo "FAIL: Nix installation must run from the exact annotated $release_tag release." >&2
    exit 1
}
normalize_remote "$origin" || {
    echo "FAIL: checkout origin is not the official repository." >&2
    exit 1
}
[[ -z "$(repo_git status --porcelain=v1 --untracked-files=all)" ]] || {
    echo "FAIL: release checkout has tracked or untracked changes." >&2
    exit 1
}
remote_refs="$(git ls-remote --tags "$official_repo" \
    "refs/tags/$release_tag" "refs/tags/$release_tag^{}")"
remote_tag_object="$(printf '%s\n' "$remote_refs" | awk -v ref="refs/tags/$release_tag" '$2 == ref { print $1 }')"
remote_commit="$(printf '%s\n' "$remote_refs" | awk -v ref="refs/tags/$release_tag^{}" '$2 == ref { print $1 }')"
[[ "$tag_object" == "$remote_tag_object" && "$head_commit" == "$remote_commit" ]] || {
    echo "FAIL: local $release_tag does not match the official immutable release." >&2
    exit 1
}

if command -v nix >/dev/null 2>&1; then
    nix --version
    nix store ping >/dev/null
    echo "Nix is already usable; no installation was attempted."
    exit 0
fi

os="$(uname -s)"
arch="$(uname -m)"
case "$os:$arch" in
    Darwin:arm64|Darwin:aarch64)
        system="aarch64-darwin"
        expected_sha256="47cb78c9fdc7b630dbbb9a89869c8e8bcd8c9eb17be036fba18585120693a4c1"
        install_mode="--daemon"
        ;;
    Darwin:*)
        echo "FAIL: the macOS Nix prerequisite requires Apple Silicon (arm64); detected $arch." >&2
        exit 1
        ;;
    Linux:x86_64|Linux:amd64)
        system="x86_64-linux"
        expected_sha256="5676b0887f1274e62edd175b6611af49aa8170c69c16877aa9bc6cebceb19855"
        install_mode="--no-daemon"
        if [[ -d /run/systemd/system ]] && command -v systemctl >/dev/null 2>&1; then
            install_mode="--daemon"
        fi
        ;;
    Linux:aarch64|Linux:arm64)
        system="aarch64-linux"
        expected_sha256="cfddd4008b57a71464a16d5232cba79b1c76ae9dc81bbf71b4972b0118bc29c5"
        install_mode="--no-daemon"
        if [[ -d /run/systemd/system ]] && command -v systemctl >/dev/null 2>&1; then
            install_mode="--daemon"
        fi
        ;;
    *)
        echo "FAIL: unsupported Nix prerequisite platform: $os $arch" >&2
        exit 1
        ;;
esac

work="$(mktemp -d "${TMPDIR:-/tmp}/dotfiles-nix-install.XXXXXX")"
chmod 700 "$work"
cleanup() {
    rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
archive="nix-$nix_version-$system.tar.xz"
url="https://releases.nixos.org/nix/nix-$nix_version/$archive"
curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location \
    --output "$work/$archive" "$url"

if command -v sha256sum >/dev/null 2>&1; then
    actual_sha256="$(sha256sum "$work/$archive" | awk '{print $1}')"
else
    actual_sha256="$(shasum -a 256 "$work/$archive" | awk '{print $1}')"
fi
[[ "$actual_sha256" == "$expected_sha256" ]] || {
    echo "FAIL: Nix release digest mismatch; downloaded bytes were not executed." >&2
    exit 1
}

if tar -tJf "$work/$archive" | awk '
    /^\// || /(^|\/)\.\.($|\/)/ { bad=1 }
    END { exit bad ? 0 : 1 }
'; then
    echo "FAIL: verified archive contains an unsafe path." >&2
    exit 1
fi
tar -xJf "$work/$archive" -C "$work"
installer="$work/nix-$nix_version-$system/install"
[[ -f "$installer" && ! -L "$installer" && -x "$installer" ]] || {
    echo "FAIL: verified Nix archive does not contain the expected installer." >&2
    exit 1
}

echo "Verified upstream Nix $nix_version for $system: $expected_sha256"
"$installer" "$install_mode"

for profile in \
    /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh \
    "$HOME/.nix-profile/etc/profile.d/nix.sh"; do
    if [[ -f "$profile" ]]; then
        # The path is one of two fixed installer outputs.
        # shellcheck disable=SC1090
        source "$profile"
        break
    fi
done
command -v nix >/dev/null 2>&1 || {
    echo "FAIL: installer returned success but Nix is unavailable in the verification shell." >&2
    exit 1
}
nix --version
nix store ping >/dev/null
echo "Nix prerequisite installed and verified; setup may continue in this shell."
