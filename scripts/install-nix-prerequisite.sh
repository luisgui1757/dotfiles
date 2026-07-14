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

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

if ! head_commit="$(repo_git rev-parse --verify --quiet 'HEAD^{commit}' 2>/dev/null)"; then
    fail "checkout HEAD is not a valid commit."
fi
if ! origin="$(repo_git remote get-url origin 2>/dev/null)"; then
    fail "checkout has no readable origin remote."
fi
normalize_remote "$origin" || {
    fail "checkout origin is not the official repository."
}
if ! checkout_status="$(repo_git status --porcelain=v1 --untracked-files=all 2>/dev/null)"; then
    fail "checkout cleanliness could not be verified."
fi
[[ -z "$checkout_status" ]] || fail "checkout has tracked or untracked changes."

# Query release and branch identities in one advertisement. Before v0.2.0 is
# published, an exact current official branch head is the prerelease authority.
# The moment the tag appears in this snapshot, that branch path closes and only
# the immutable annotated release is accepted.
work="$(mktemp -d "${TMPDIR:-/tmp}/dotfiles-nix-install.XXXXXX")"
chmod 700 "$work"
mkdir "$work/remote-query"
cleanup() {
    rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

nix_extra_conf="$work/nix-extra.conf"
printf '%s\n' 'extra-experimental-features = nix-command flakes' > "$nix_extra_conf"
chmod 600 "$nix_extra_conf"

ensure_user_nix_features() {
    local config_root config stage rendered
    config_root="${XDG_CONFIG_HOME:-$HOME/.config}/nix"
    config="$config_root/nix.conf"
    [[ ! -L "$config" ]] || fail "refusing to replace symlinked Nix user config: $config"
    [[ ! -e "$config" || -f "$config" ]] || fail "Nix user config is not a regular file: $config"
    mkdir -p "$config_root"
    stage="$(mktemp "$config_root/.nix.conf.dotfiles.XXXXXX")"
    rendered="$work/nix-user-conf.rendered"
    if [[ -f "$config" ]]; then
        cp -p "$config" "$stage"
        awk '
            function has_token(value, token, count, parts, index) {
                count = split(value, parts, /[[:space:]]+/)
                for (index = 1; index <= count; index++) {
                    if (parts[index] == token) return 1
                }
                return 0
            }
            BEGIN { updated = 0 }
            {
                line = $0
                if (!updated && line !~ /^[[:space:]]*#/) {
                    equals = index(line, "=")
                    if (equals > 0) {
                        key = substr(line, 1, equals - 1)
                        gsub(/[[:space:]]/, "", key)
                        if (key == "experimental-features" ||
                            key == "extra-experimental-features") {
                            value = substr(line, equals + 1)
                            comment = ""
                            comment_at = index(value, "#")
                            if (comment_at > 0) {
                                comment = substr(value, comment_at)
                                value = substr(value, 1, comment_at - 1)
                            }
                            addition = ""
                            if (!has_token(value, "nix-command")) addition = addition " nix-command"
                            if (!has_token(value, "flakes")) addition = addition " flakes"
                            print substr(line, 1, equals) value addition comment
                            updated = 1
                            next
                        }
                    }
                }
                print line
            }
            END {
                if (!updated) print "extra-experimental-features = nix-command flakes"
            }
        ' "$config" > "$rendered"
        cat "$rendered" > "$stage"
    else
        printf '%s\n' 'extra-experimental-features = nix-command flakes' > "$stage"
        chmod 600 "$stage"
    fi
    mv -f "$stage" "$config"
    echo "Configured Nix user features: nix-command flakes"
}

remote_git() (
    unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_OBJECT_DIRECTORY
    unset GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_CONFIG GIT_PROXY_COMMAND
    unset GIT_SSH GIT_SSH_COMMAND GIT_ASKPASS SSH_ASKPASS
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_CONFIG_SYSTEM=/dev/null \
    GIT_CONFIG_GLOBAL=/dev/null \
    GIT_CONFIG_COUNT=0 \
    GIT_CONFIG_PARAMETERS='' \
    GIT_TEMPLATE_DIR='' \
    GIT_TERMINAL_PROMPT=0 \
    GCM_INTERACTIVE=0 \
        git -C "$work/remote-query" \
        -c credential.helper= \
        -c core.hooksPath=/dev/null \
        -c http.sslVerify=true \
        -c protocol.ext.allow=never \
        -c protocol.file.allow=never \
        "$@"
)

if ! remote_refs="$(remote_git ls-remote "$official_repo" \
    "refs/tags/$release_tag" "refs/tags/$release_tag^{}" 'refs/heads/*' \
    2>/dev/null)"; then
    fail "could not verify release and branch identities from the official repository."
fi

tag_ref="refs/tags/$release_tag"
peeled_tag_ref="$tag_ref^{}"
invalid_remote_refs="$(printf '%s\n' "$remote_refs" | awk \
    -v tag_ref="$tag_ref" -v peeled_tag_ref="$peeled_tag_ref" '
    NF && (NF != 2 || length($1) != 40 || $1 !~ /^[0-9a-f]+$/ ||
        ($2 != tag_ref && $2 != peeled_tag_ref && $2 !~ /^refs\/heads\//)) { count++ }
    END { print count + 0 }
')"
[[ "$invalid_remote_refs" == "0" ]] || fail "official repository returned malformed identity data."

remote_tag_object_count="$(printf '%s\n' "$remote_refs" | awk -v ref="$tag_ref" \
    '$2 == ref { count++ } END { print count + 0 }')"
remote_commit_count="$(printf '%s\n' "$remote_refs" | awk -v ref="$peeled_tag_ref" \
    '$2 == ref { count++ } END { print count + 0 }')"

if [[ "$remote_tag_object_count" == "0" && "$remote_commit_count" == "0" ]]; then
    matched_branch="$(printf '%s\n' "$remote_refs" | awk -v head="$head_commit" \
        '$1 == head && $2 ~ /^refs\/heads\// { print $2; exit }')"
    [[ -n "$matched_branch" ]] || fail \
        "before $release_tag is published, checkout HEAD must be a current official branch head."
    echo "Verified prerelease checkout: $matched_branch at $head_commit"
else
    [[ "$remote_tag_object_count" == "1" && "$remote_commit_count" == "1" ]] || fail \
        "official $release_tag must be one unique annotated tag."
    remote_tag_object="$(printf '%s\n' "$remote_refs" | awk -v ref="$tag_ref" \
        '$2 == ref { print $1 }')"
    remote_commit="$(printf '%s\n' "$remote_refs" | awk -v ref="$peeled_tag_ref" \
        '$2 == ref { print $1 }')"
    if ! tag_object="$(repo_git rev-parse --verify --quiet "$tag_ref" 2>/dev/null)" ||
        ! tag_commit="$(repo_git rev-parse --verify --quiet "$tag_ref^{commit}" 2>/dev/null)" ||
        ! tag_type="$(repo_git cat-file -t "$tag_object" 2>/dev/null)"; then
        fail "$release_tag is published; use a fresh exact-tag checkout of the official release."
    fi
    [[ "$tag_type" == "tag" && "$tag_object" == "$remote_tag_object" &&
        "$tag_commit" == "$remote_commit" && "$head_commit" == "$remote_commit" ]] || fail \
        "local $release_tag does not match the official immutable annotated release."
    echo "Verified immutable release checkout: $release_tag at $head_commit"
fi

if command -v nix >/dev/null 2>&1; then
    nix --version
    if nix store info >/dev/null 2>&1; then
        echo "Nix is already usable; no installation was attempted."
        exit 0
    fi
    nix_error="$(nix store info 2>&1 || true)"
    [[ "$nix_error" == *"experimental Nix feature 'nix-command' is disabled"* ]] ||
        fail "Nix is installed but its store is unusable: $nix_error"
    ensure_user_nix_features
    nix store info >/dev/null || fail "Nix feature reconciliation did not make its store usable."
    echo "Nix is installed; required user features were reconciled."
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
"$installer" "$install_mode" --yes --nix-extra-conf-file "$nix_extra_conf"

if [[ "$install_mode" == "--no-daemon" ]]; then
    # The upstream flag persists /etc/nix/nix.conf only for daemon installs.
    # Single-user Linux therefore needs the same additive setting in its user config.
    ensure_user_nix_features
fi

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
nix store info >/dev/null
echo "Nix prerequisite installed and verified; setup may continue in this shell."
