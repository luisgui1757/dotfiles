#!/usr/bin/env bash
# Transactional v0.1.0 -> v0.2.0 migration for macOS, Linux, and WSL userland.
# Run this script only from a separate, exact v0.2.0 release checkout.
set -euo pipefail

old_tag="v0.1.0"
old_tag_object="a3b4d6d7b6d289959cac68d76faec96219b3e310"
old_commit="015617362830280bf85c7142e69d0681d376d453"
new_tag="v0.2.0"
official_repo="https://github.com/luisgui1757/dotfiles.git"
script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")"
default_new_checkout="$(cd "$(dirname "$script_path")/.." && pwd -P)"
transaction_active=0
rollback_running=0
active_recovery=""
preflight_old_checkout=""
preflight_new_checkout=""
preflight_platform=""
prepared_recovery=""
preparing_recovery=""
loaded_recovery=""
loaded_old_source=""
loaded_new_source=""
loaded_platform=""
loaded_nix_command=""
loaded_new_targets=""
loaded_absent_parent_dirs=""
loaded_providers_before=""
loaded_flake_lock=""
loaded_brew_repository=""
loaded_brew_formulae=""
loaded_brew_casks=""
loaded_tap_backups=""

usage() {
    cat <<'EOF'
upgrade-v0.1.0.sh --preflight-only <retained-v0.1.0-checkout>
upgrade-v0.1.0.sh --apply          <retained-v0.1.0-checkout>
upgrade-v0.1.0.sh --rollback       <recovery-directory>
upgrade-v0.1.0.sh --accept         <recovery-directory>

The preflight and apply commands must run from a separate checkout at the exact
annotated v0.2.0 release. The old v0.1.0 checkout remains untouched and is the
acceptance identity; apply archives both exact commits into private recovery and
uses only those frozen trees for publication and rollback. It rolls back
automatically on failure or interruption and requires explicit acceptance before
either checkout or the recovery directory may be removed.
EOF
}

fail() {
    echo "FAIL: $*" >&2
    return 1
}

remove_private_tree() {
    local path="$1"
    [[ -n "$path" && -d "$path" ]] || return 0
    chmod -R u+w "$path" 2>/dev/null || true
    rm -rf "$path"
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is required for the release migration."
}

canonical_directory() {
    local directory="$1"
    [[ -d "$directory" && ! -L "$directory" ]] || return 1
    (cd "$directory" && pwd -P)
}

normalize_remote() {
    local remote="$1" normalized
    case "$remote" in
        https://github.com/*)
            normalized="${remote#https://github.com/}"
            ;;
        git@github.com:*)
            normalized="${remote#git@github.com:}"
            ;;
        ssh://git@github.com/*)
            normalized="${remote#ssh://git@github.com/}"
            ;;
        *)
            return 1
            ;;
    esac
    normalized="${normalized%.git}"
    [[ "$normalized" == "luisgui1757/dotfiles" ]]
}

repo_git() {
    local checkout="$1"
    shift
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

assert_clean_checkout() {
    local checkout="$1" label="$2" status
    if ! status="$(repo_git "$checkout" status --porcelain=v1 --untracked-files=all 2>/dev/null)"; then
        fail "could not inspect the $label checkout: $checkout"
        return 1
    fi
    if [[ -n "$status" ]]; then
        echo "FAIL: $label checkout has tracked or untracked changes: $checkout" >&2
        printf '%s\n' "$status" | sed 's/^/      /' >&2
        return 1
    fi
}

remote_tag_identity() {
    local tag="$1" refs tag_object peeled
    refs="$(git ls-remote --tags "$official_repo" "refs/tags/$tag" "refs/tags/$tag^{}")" || {
        fail "could not read the official $tag tag identity."
        return 1
    }
    tag_object="$(printf '%s\n' "$refs" | awk -v ref="refs/tags/$tag" '$2 == ref { print $1 }')"
    peeled="$(printf '%s\n' "$refs" | awk -v ref="refs/tags/$tag^{}" '$2 == ref { print $1 }')"
    [[ "$tag_object" =~ ^[0-9a-f]{40}$ && "$peeled" =~ ^[0-9a-f]{40}$ ]] || {
        fail "$tag must be an annotated official release tag."
        return 1
    }
    printf '%s\n%s\n' "$tag_object" "$peeled"
}

assert_release_checkout() {
    local checkout="$1" tag="$2" expected_commit="${3:-}" expected_tag_object="${4:-}"
    local label="$5" head tag_commit tag_type local_tag_object origin remote_identity
    local remote_tag_object remote_commit
    head="$(repo_git "$checkout" rev-parse --verify 'HEAD^{commit}' 2>/dev/null || true)"
    tag_commit="$(repo_git "$checkout" rev-parse --verify "refs/tags/$tag^{commit}" 2>/dev/null || true)"
    tag_type="$(repo_git "$checkout" cat-file -t "refs/tags/$tag" 2>/dev/null || true)"
    local_tag_object="$(repo_git "$checkout" rev-parse --verify "refs/tags/$tag" 2>/dev/null || true)"
    [[ "$head" =~ ^[0-9a-f]{40}$ && "$head" == "$tag_commit" && "$tag_type" == "tag" ]] || {
        fail "$label checkout is not the exact annotated $tag release: $checkout"
        return 1
    }
    if [[ -n "$expected_commit" && "$head" != "$expected_commit" ]]; then
        fail "$label checkout commit is $head; expected $expected_commit."
        return 1
    fi
    if [[ -n "$expected_tag_object" && "$local_tag_object" != "$expected_tag_object" ]]; then
        fail "$label tag object is $local_tag_object; expected $expected_tag_object."
        return 1
    fi
    origin="$(repo_git "$checkout" remote get-url origin 2>/dev/null || true)"
    normalize_remote "$origin" || {
        fail "$label checkout origin is not the official repository: ${origin:-missing}"
        return 1
    }
    remote_identity="$(remote_tag_identity "$tag")" || return 1
    remote_tag_object="$(printf '%s\n' "$remote_identity" | sed -n '1p')"
    remote_commit="$(printf '%s\n' "$remote_identity" | sed -n '2p')"
    [[ "$local_tag_object" == "$remote_tag_object" && "$head" == "$remote_commit" ]] || {
        fail "$label checkout does not match the official immutable $tag identity."
        return 1
    }
    assert_clean_checkout "$checkout" "$label"
}

assert_local_release_checkout() {
    local checkout="$1" tag="$2" expected_commit="$3" expected_tag_object="$4" label="$5"
    local head tag_commit tag_type tag_object origin
    head="$(repo_git "$checkout" rev-parse --verify 'HEAD^{commit}' 2>/dev/null || true)"
    tag_commit="$(repo_git "$checkout" rev-parse --verify "refs/tags/$tag^{commit}" 2>/dev/null || true)"
    tag_type="$(repo_git "$checkout" cat-file -t "refs/tags/$tag" 2>/dev/null || true)"
    tag_object="$(repo_git "$checkout" rev-parse --verify "refs/tags/$tag" 2>/dev/null || true)"
    [[ "$head" == "$expected_commit" && "$tag_commit" == "$expected_commit" && \
        "$tag_type" == "tag" && "$tag_object" == "$expected_tag_object" ]] || {
        fail "$label release checkout or tag moved; refusing recovery."
        return 1
    }
    origin="$(repo_git "$checkout" remote get-url origin 2>/dev/null || true)"
    normalize_remote "$origin" || {
        fail "$label checkout origin is not the official repository: ${origin:-missing}"
        return 1
    }
    assert_clean_checkout "$checkout" "$label"
}

account_home() {
    local user="$1" os="$2" result
    if [[ "$os" == "Darwin" ]]; then
        result="$(dscl . -read "/Users/$user" NFSHomeDirectory 2>/dev/null | awk '{$1=""; sub(/^ /, ""); print}')"
    elif command -v getent >/dev/null 2>&1; then
        result="$(getent passwd "$user" | awk -F: 'NR == 1 { print $6 }')"
    else
        result="$(awk -F: -v user="$user" '$1 == user { print $6; exit }' /etc/passwd)"
    fi
    [[ -n "$result" ]] || return 1
    canonical_directory "$result"
}

assert_target_identity() {
    local os="$1" user expected_home actual_home
    [[ "$(id -u)" -ne 0 ]] || {
        fail "run the migration as the target non-root user, not root."
        return 1
    }
    user="$(id -un)"
    expected_home="$(account_home "$user" "$os")" || {
        fail "could not resolve the account-record home for $user."
        return 1
    }
    actual_home="$(canonical_directory "$HOME")" || {
        fail "HOME is not an existing real directory: $HOME"
        return 1
    }
    [[ "$actual_home" == "$expected_home" ]] || {
        fail "HOME resolves to $actual_home, but $user's account home is $expected_home."
        return 1
    }
}

home_manager_state_exists() {
    local state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
    local data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
    local user profiles
    user="$(id -un)"
    for profiles in \
        "$state_home/nix/profiles/home-manager" \
        "/nix/var/nix/profiles/per-user/$user/home-manager" \
        "$state_home/home-manager" \
        "$data_home/home-manager"; do
        if [[ -e "$profiles" || -L "$profiles" ]]; then
            return 0
        fi
    done
    return 1
}

nix_darwin_state_exists() {
    [[ -e /run/current-system || -L /run/current-system ]] ||
        command -v darwin-rebuild >/dev/null 2>&1
}

verify_old_config() (
    local source="$1" managed target expected_file expected_ref
    trap '[[ -z "${expected_file:-}" ]] || rm -f "$expected_file"' EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    if chezmoi --source "$source/home" --destination "$HOME" \
        verify --include files,symlinks >/dev/null 2>&1; then
        return 0
    fi
    managed="$(chezmoi --source "$source/home" --destination "$HOME" \
        managed --path-style absolute --include files,symlinks)" || return 1
    while IFS= read -r target; do
        [[ -n "$target" ]] || continue
        expected_file="$(mktemp)"
        if ! chezmoi --source "$source/home" --destination "$HOME" \
            cat "$target" > "$expected_file" 2>/dev/null; then
            rm -f "$expected_file"
            return 1
        fi
        expected_ref="$(< "$expected_file")"
        if [[ -n "$expected_ref" && -e "$expected_ref" ]]; then
            if [[ -d "$target" && -d "$expected_ref" ]]; then
                diff -qr "$target" "$expected_ref" >/dev/null 2>&1 || {
                    rm -f "$expected_file"
                    return 1
                }
            elif [[ -f "$expected_ref" && ( -f "$target" || -L "$target" ) ]]; then
                cmp -s "$target" "$expected_ref" || {
                    rm -f "$expected_file"
                    return 1
                }
            else
                rm -f "$expected_file"
                return 1
            fi
        elif [[ ( -f "$target" || -L "$target" ) && ! -d "$target" ]]; then
            cmp -s "$target" "$expected_file" || {
                rm -f "$expected_file"
                return 1
            }
        else
            rm -f "$expected_file"
            return 1
        fi
        rm -f "$expected_file"
    done <<< "$managed"
)

platform_preflight() {
    local os="$1" arch
    arch="$(uname -m)"
    case "$os" in
        Darwin)
            [[ "$arch" == "arm64" || "$arch" == "aarch64" ]] || {
                fail "the v0.2.0 macOS upgrade requires Apple Silicon (arm64); detected $arch."
                return 1
            }
            require_command brew || return 1
            if nix_darwin_state_exists; then
                fail "an existing nix-darwin installation is outside the v0.1.0 migration contract."
                return 1
            fi
            ;;
        Linux)
            case "$arch" in
                x86_64|amd64|aarch64|arm64) ;;
                *) fail "unsupported Linux architecture: $arch"; return 1 ;;
            esac
            if home_manager_state_exists || command -v home-manager >/dev/null 2>&1; then
                fail "an existing Home Manager installation is outside the v0.1.0 migration contract."
                return 1
            fi
            ;;
        *)
            fail "unsupported POSIX platform: $os"
            return 1
            ;;
    esac
}

run_preflight() {
    local old_input="$1" new_input="$2" old_checkout new_checkout os
    for command_name in git jq nix chezmoi python3 tar; do
        require_command "$command_name" || return 1
    done
    old_checkout="$(canonical_directory "$old_input")" || {
        fail "v0.1.0 checkout is not an existing real directory: $old_input"
        return 1
    }
    new_checkout="$(canonical_directory "$new_input")" || {
        fail "v0.2.0 checkout is not an existing real directory: $new_input"
        return 1
    }
    [[ "$old_checkout" != "$new_checkout" ]] || {
        fail "in-place migration is forbidden; retain v0.1.0 and use a separate v0.2.0 checkout."
        return 1
    }
    assert_release_checkout "$old_checkout" "$old_tag" "$old_commit" \
        "$old_tag_object" "old" || return 1
    assert_release_checkout "$new_checkout" "$new_tag" "" "" "new" || return 1
    os="$(uname -s)"
    assert_target_identity "$os" || return 1
    platform_preflight "$os" || return 1
    nix --version >/dev/null || return 1
    nix store ping >/dev/null || return 1
    verify_old_config "$old_checkout" || {
        fail "live config does not exactly match the retained v0.1.0 checkout; no mutation was attempted."
        return 1
    }
    preflight_old_checkout="$old_checkout"
    preflight_new_checkout="$new_checkout"
    preflight_platform="$os"
}

write_private_file() {
    local path="$1" value="$2"
    [[ "$value" != *$'\n'* ]] || {
        fail "recovery values may not contain newlines: $path"
        return 1
    }
    printf '%s\n' "$value" > "$path"
    chmod 600 "$path"
}

set_stage() {
    local recovery="$1" stage="$2" temporary
    temporary="$recovery/stage.tmp"
    printf '%s\n' "$stage" > "$temporary"
    chmod 600 "$temporary"
    mv "$temporary" "$recovery/stage"
}

capture_providers() {
    local output="$1" command_name source
    : > "$output"
    for command_name in rg fd fzf jq lazygit node starship zoxide nvim tree-sitter chezmoi gh cmake; do
        source="$(command -v "$command_name" 2>/dev/null || true)"
        printf '%s\t%s\n' "$command_name" "${source:-absent}" >> "$output"
    done
    chmod 600 "$output"
}

capture_mac_state() {
    local recovery="$1" brew_repository library
    brew_repository="$(brew --repository)"
    library="$brew_repository/Library"
    write_private_file "$recovery/brew-repository" "$brew_repository"
    brew list --formula --versions | LC_ALL=C sort > "$recovery/brew-formulae.before"
    brew list --cask --versions | LC_ALL=C sort > "$recovery/brew-casks.before"
    # Compatibility evidence for recovery directories created by releases that
    # predate mixed Homebrew tap ownership. Current setup creates no new backup.
    find "$library" -mindepth 1 -maxdepth 1 -type d \
        -name 'Taps.dotfiles-pre-nix-*' -print 2>/dev/null | LC_ALL=C sort \
        > "$recovery/tap-backups.before"
    chmod 600 "$recovery/brew-formulae.before" \
        "$recovery/brew-casks.before" "$recovery/tap-backups.before"
}

sha256_file() {
    python3 -c '
import hashlib, sys
h = hashlib.sha256()
with open(sys.argv[1], "rb") as stream:
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        h.update(chunk)
print(h.hexdigest())
' "$1"
}

tree_fingerprint() {
    python3 - "$1" <<'PY'
import hashlib
import json
import os
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
records = []
for path in sorted(root.rglob("*"), key=lambda value: value.relative_to(root).as_posix()):
    relative = path.relative_to(root).as_posix()
    mode = path.lstat().st_mode
    if stat.S_ISLNK(mode):
        records.append(["symlink", relative, os.readlink(path)])
    elif stat.S_ISDIR(mode):
        records.append(["directory", relative])
    elif stat.S_ISREG(mode):
        records.append(["file", relative, hashlib.sha256(path.read_bytes()).hexdigest()])
    else:
        raise SystemExit(f"unsupported release-tree entry: {relative}")
print(json.dumps(records, separators=(",", ":"), ensure_ascii=True))
PY
}

make_tree_read_only() {
    local root="$1"
    find "$root" -type f -perm -u=x -exec chmod 500 {} +
    find "$root" -type f ! -perm -u=x -exec chmod 400 {} +
    find "$root" -type d -exec chmod 500 {} +
}

payload_files() {
    local os="$1"
    printf '%s\n' \
        old-checkout new-checkout old-commit new-commit new-tag-object \
        platform target-home nix-command nix-version.before providers.before \
        old-targets.txt new-targets.txt absent-parent-dirs.before flake.lock \
        old-release.tar new-release.tar old-release.tree new-release.tree \
        upgrade-v0.1.0.sh
    if [[ "$os" == "Darwin" ]]; then
        printf '%s\n' brew-repository brew-formulae.before brew-casks.before \
            tap-backups.before
    fi
}

capture_absent_parent_dirs() {
    local targets="$1" output="$2"
    python3 - "$HOME" "$targets" "$output" <<'PY'
import os
import pathlib
import sys

home = os.path.realpath(sys.argv[1])
targets = pathlib.Path(sys.argv[2]).read_text().splitlines()
missing = set()
for raw in targets:
    target = os.path.abspath(raw)
    if os.path.commonpath((home, target)) != home or target == home:
        raise SystemExit(f"managed target is outside HOME: {raw}")
    parent = os.path.dirname(target)
    while parent != home:
        if not os.path.lexists(parent):
            missing.add(parent)
        next_parent = os.path.dirname(parent)
        if next_parent == parent:
            raise SystemExit(f"managed target parent escaped HOME: {raw}")
        parent = next_parent
pathlib.Path(sys.argv[3]).write_text(
    "".join(f"{path}\n" for path in sorted(missing, key=lambda value: (-value.count(os.sep), value)))
)
PY
    chmod 600 "$output"
}

write_payload_manifest() {
    local recovery="$1" os="$2" file
    : > "$recovery/payload.sha256"
    while IFS= read -r file; do
        printf '%s  %s\n' "$(sha256_file "$recovery/$file")" "$file" \
            >> "$recovery/payload.sha256"
        if [[ "$file" == "upgrade-v0.1.0.sh" ]]; then
            chmod 500 "$recovery/$file"
        else
            chmod 400 "$recovery/$file"
        fi
    done < <(payload_files "$os")
    chmod 400 "$recovery/payload.sha256"
}

prepare_recovery() {
    local old_checkout="$1" new_checkout="$2" os="$3" state_root recovery
    local head tag_object
    state_root="${XDG_STATE_HOME:-$HOME/.local/state}/dotfiles/migrations"
    mkdir -p "$state_root"
    chmod 700 "$state_root"
    recovery="$(mktemp -d "$state_root/v0.1.0-to-v0.2.0.XXXXXX")"
    chmod 700 "$recovery"
    preparing_recovery="$recovery"
    head="$(repo_git "$new_checkout" rev-parse 'HEAD^{commit}')"
    tag_object="$(repo_git "$new_checkout" rev-parse "refs/tags/$new_tag")"
    write_private_file "$recovery/old-checkout" "$old_checkout"
    write_private_file "$recovery/new-checkout" "$new_checkout"
    write_private_file "$recovery/old-commit" "$old_commit"
    write_private_file "$recovery/new-commit" "$head"
    write_private_file "$recovery/new-tag-object" "$tag_object"
    write_private_file "$recovery/platform" "$os"
    write_private_file "$recovery/target-home" "$(canonical_directory "$HOME")"
    write_private_file "$recovery/nix-command" "$(command -v nix)"
    repo_git "$old_checkout" archive --format=tar "$old_commit" \
        > "$recovery/old-release.tar"
    repo_git "$new_checkout" archive --format=tar "$head" \
        > "$recovery/new-release.tar"
    mkdir "$recovery/old-release" "$recovery/new-release"
    tar -xf "$recovery/old-release.tar" -C "$recovery/old-release"
    tar -xf "$recovery/new-release.tar" -C "$recovery/new-release"
    tree_fingerprint "$recovery/old-release" > "$recovery/old-release.tree"
    tree_fingerprint "$recovery/new-release" > "$recovery/new-release.tree"
    make_tree_read_only "$recovery/old-release"
    make_tree_read_only "$recovery/new-release"
    nix --version > "$recovery/nix-version.before"
    chmod 600 "$recovery/nix-version.before"
    capture_providers "$recovery/providers.before"
    chezmoi --source "$recovery/old-release/home" --destination "$HOME" \
        managed --path-style absolute --include files,symlinks \
        > "$recovery/old-targets.txt"
    chezmoi --source "$recovery/new-release/home" --destination "$HOME" \
        managed --path-style absolute --include files,symlinks \
        > "$recovery/new-targets.txt"
    chmod 600 "$recovery/old-targets.txt" "$recovery/new-targets.txt"
    capture_absent_parent_dirs "$recovery/new-targets.txt" \
        "$recovery/absent-parent-dirs.before"
    cp "$recovery/new-release/flake.lock" "$recovery/flake.lock"
    cp "$script_path" "$recovery/upgrade-v0.1.0.sh"
    chmod 500 "$recovery/upgrade-v0.1.0.sh"
    if [[ "$os" == "Darwin" ]]; then
        capture_mac_state "$recovery"
    fi
    set_stage "$recovery" prepared
    printf '%s\n' \
        "Private recovery material for v0.1.0 -> v0.2.0." \
        "Old checkout: $old_checkout" \
        "New checkout: $new_checkout" \
        "Rollback: $recovery/upgrade-v0.1.0.sh --rollback '$recovery'" \
        "Accept:   $recovery/upgrade-v0.1.0.sh --accept '$recovery'" \
        > "$recovery/RECOVERY.txt"
    chmod 600 "$recovery/RECOVERY.txt"
    write_payload_manifest "$recovery" "$os"
    prepared_recovery="$recovery"
}

lock_metadata() {
    local lock="$1" node="$2" owner="$3" repository="$4"
    jq -er --arg node "$node" --arg owner "$owner" --arg repository "$repository" '
      .nodes[$node].locked
      | select(.type == "github" and .owner == $owner and .repo == $repository)
      | select(.rev | test("^[0-9a-f]{40}$"))
      | select(.narHash | type == "string" and length > 0)
      | .rev, .narHash
    ' "$lock"
}

query_encode() {
    local value="$1"
    value="${value//%/%25}"
    value="${value//+/%2B}"
    value="${value//\//%2F}"
    value="${value//=/%3D}"
    value="${value//#/%23}"
    value="${value//&/%26}"
    printf '%s\n' "$value"
}

pinned_app_ref() {
    local lock="$1" node="$2" owner="$3" repository="$4" app="$5"
    local metadata rev nar_hash
    metadata="$(lock_metadata "$lock" "$node" "$owner" "$repository")" || return 1
    rev="$(printf '%s\n' "$metadata" | sed -n '1p')"
    nar_hash="$(printf '%s\n' "$metadata" | sed -n '2p')"
    printf 'github:%s/%s/%s?narHash=%s#%s\n' \
        "$owner" "$repository" "$rev" "$(query_encode "$nar_hash")" "$app"
}

path_resolves_within() {
    local path="$1" root="$2" resolved
    resolved="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$path")" || return 1
    [[ "$resolved" == "$root" || "$resolved" == "$root/"* ]]
}

restore_old_config() {
    local recovery="$1" old_checkout="$2" new_checkout="$3" target
    while IFS= read -r target; do
        [[ -n "$target" ]] || continue
        if [[ -L "$target" ]] && path_resolves_within "$target" "$new_checkout"; then
            rm "$target"
        elif [[ -e "$target" && ! -L "$target" ]]; then
            echo "FAIL: recovery will not replace an unexpected non-symlink target: $target" >&2
            return 1
        fi
    done <<< "$loaded_new_targets"
    chezmoi --source "$old_checkout/home" --destination "$HOME" init
    chezmoi --source "$old_checkout/home" --destination "$HOME" \
        --no-tty --force apply --include files,symlinks
    verify_old_config "$old_checkout" || return 1
    prune_absent_parent_dirs
}

restore_mac_taps() {
    local recovery="$1" repository library current candidate_count candidate quarantine
    repository="$loaded_brew_repository"
    library="$repository/Library"
    # Restore only a legacy tap-migration backup created during this upgrade.
    # With mutable taps, current setup creates none and this returns unchanged.
    current="$recovery/tap-backups.current"
    find "$library" -mindepth 1 -maxdepth 1 -type d \
        -name 'Taps.dotfiles-pre-nix-*' -print 2>/dev/null | LC_ALL=C sort > "$current"
    candidate="$recovery/tap-backups.new"
    comm -13 <(printf '%s\n' "$loaded_tap_backups") "$current" > "$candidate"
    candidate_count="$(awk 'NF { count++ } END { print count + 0 }' "$candidate")"
    if [[ "$candidate_count" -eq 0 ]]; then
        return 0
    fi
    [[ "$candidate_count" -eq 1 ]] || {
        fail "multiple new nix-homebrew tap backups require manual recovery; see $candidate"
        return 1
    }
    candidate="$(sed -n '1p' "$candidate")"
    [[ -d "$candidate" && ! -L "$candidate" ]] || {
        fail "recorded tap backup is missing or unsafe: $candidate"
        return 1
    }
    if [[ -e "$library/Taps" || -L "$library/Taps" ]]; then
        quarantine="$library/Taps.dotfiles-upgrade-failed-$(date +%Y%m%d-%H%M%S)"
        sudo mv "$library/Taps" "$quarantine"
    fi
    sudo mv "$candidate" "$library/Taps"
}

brew_item_present_before() {
    local inventory="$1" item="$2"
    printf '%s\n' "$inventory" |
        awk -v item="$item" '$1 == item { found=1 } END { exit found ? 0 : 1 }'
}

remove_new_mac_packages() {
    local recovery="$1" item
    for item in wezterm aerospace; do
        if ! brew_item_present_before "$loaded_brew_casks" "$item" && \
            brew list --cask "$item" >/dev/null 2>&1; then
            brew uninstall --cask "$item"
        fi
    done
    item=herdr
    if ! brew_item_present_before "$loaded_brew_formulae" "$item" && \
        brew list --formula "$item" >/dev/null 2>&1; then
        brew uninstall --formula "$item"
    fi
}

rollback_package_layer() {
    local recovery="$1" os="$2" nix_command app_ref
    nix_command="$loaded_nix_command"
    [[ -x "$nix_command" ]] || {
        fail "recorded Nix command is unavailable: $nix_command"
        return 1
    }
    if [[ "$os" == "Darwin" ]]; then
        if nix_darwin_state_exists; then
            app_ref="$(pinned_app_ref <(printf '%s\n' "$loaded_flake_lock") \
                nix-darwin nix-darwin nix-darwin darwin-uninstaller)" || return 1
            printf '\n' | sudo "$nix_command" \
                --extra-experimental-features 'nix-command flakes' run "$app_ref"
        fi
        restore_mac_taps "$recovery"
        remove_new_mac_packages "$recovery"
        if nix_darwin_state_exists; then
            fail "nix-darwin remains active after rollback."
            return 1
        fi
    else
        if home_manager_state_exists || command -v home-manager >/dev/null 2>&1; then
            app_ref="$(pinned_app_ref <(printf '%s\n' "$loaded_flake_lock") \
                home-manager nix-community home-manager home-manager)" || return 1
            printf 'y\n' | "$nix_command" run "$app_ref" -- uninstall
        fi
        if home_manager_state_exists; then
            fail "Home Manager state remains after rollback."
            return 1
        fi
    fi
}

verify_provider_boundary() {
    local recovery="$1" current
    current="$recovery/providers.rollback"
    capture_providers "$current"
    if [[ "$(< "$current")" != "$loaded_providers_before" ]]; then
        echo "FAIL: command-provider precedence differs after rollback." >&2
        diff -u <(printf '%s\n' "$loaded_providers_before") "$current" >&2 || true
        return 1
    fi
}

read_recovery_scalar() {
    local file="$1"
    python3 -c '
import pathlib, sys
raw = pathlib.Path(sys.argv[1]).read_bytes()
if not raw.endswith(b"\n") or raw.count(b"\n") != 1 or b"\r" in raw or b"\0" in raw or len(raw) == 1:
    raise SystemExit(1)
sys.stdout.buffer.write(raw[:-1])
' "$file" || fail "recovery scalar is malformed: $file"
}

assert_private_permissions() {
    local path="$1"
    python3 -c '
import os, sys
mode = os.stat(sys.argv[1], follow_symlinks=False).st_mode
raise SystemExit(0 if mode & 0o077 == 0 else 1)
' "$path" || fail "recovery permissions expose group or other access: $path"
}

verify_payload_manifest() {
    local recovery="$1" os="$2" file expected line line_number=0
    [[ -f "$recovery/payload.sha256" && ! -L "$recovery/payload.sha256" ]] || {
        fail "recovery payload manifest is missing or unsafe."
        return 1
    }
    assert_private_permissions "$recovery/payload.sha256" || return 1
    while IFS= read -r file; do
        line_number=$((line_number + 1))
        [[ -f "$recovery/$file" && ! -L "$recovery/$file" ]] || {
            fail "recovery directory is incomplete or unsafe: $file"
            return 1
        }
        assert_private_permissions "$recovery/$file" || return 1
        expected="$(sha256_file "$recovery/$file")  $file" || return 1
        line="$(sed -n "${line_number}p" "$recovery/payload.sha256")"
        [[ "$line" == "$expected" ]] || {
            fail "recovery payload digest or identity differs: $file"
            return 1
        }
    done < <(payload_files "$os")
    [[ "$(wc -l < "$recovery/payload.sha256" | tr -d ' ')" == "$line_number" ]] || {
        fail "recovery payload manifest has unexpected entries."
        return 1
    }
}

verify_release_tree() {
    local recovery="$1" name="$2" directory expected actual
    directory="$recovery/$name-release"
    [[ -d "$directory" && ! -L "$directory" ]] || {
        fail "frozen $name release tree is missing or unsafe."
        return 1
    }
    assert_private_permissions "$directory" || return 1
    expected="$(< "$recovery/$name-release.tree")"
    actual="$(tree_fingerprint "$directory")" || return 1
    [[ "$actual" == "$expected" ]] || {
        fail "frozen $name release tree differs from its validated archive."
        return 1
    }
}

validate_provider_inventory() {
    local inventory="$1" expected name source extra index=0
    local names=(rg fd fzf jq lazygit node starship zoxide nvim tree-sitter chezmoi gh cmake)
    while IFS=$'\t' read -r name source extra; do
        index=$((index + 1))
        expected="${names[$((index - 1))]:-}"
        [[ "$name" == "$expected" && -n "$source" && -z "$extra" ]] || {
            fail "captured command-provider inventory is malformed."
            return 1
        }
    done <<< "$inventory"
    [[ "$index" -eq 13 ]] || {
        fail "captured command-provider inventory is incomplete."
        return 1
    }
}

validate_absent_parent_inventory() {
    local parents="$1" targets="$2"
    python3 - "$HOME" <(printf '%s\n' "$parents") <(printf '%s\n' "$targets") <<'PY'
import os
import pathlib
import sys

home = os.path.realpath(sys.argv[1])
parents = [value for value in pathlib.Path(sys.argv[2]).read_text().splitlines() if value]
targets = [os.path.abspath(value) for value in pathlib.Path(sys.argv[3]).read_text().splitlines() if value]
if len(parents) != len(set(parents)):
    raise SystemExit("duplicate absent parent")
if parents != sorted(parents, key=lambda value: (-value.count(os.sep), value)):
    raise SystemExit("absent parents are not deepest-first")
for parent in parents:
    if not os.path.isabs(parent) or parent == home or os.path.commonpath((home, parent)) != home:
        raise SystemExit(f"unsafe absent parent: {parent}")
    if not any(target.startswith(parent + os.sep) for target in targets):
        raise SystemExit(f"absent parent is unrelated to managed targets: {parent}")
PY
}

prune_absent_parent_dirs() {
    local directory
    while IFS= read -r directory; do
        [[ -n "$directory" ]] || continue
        if [[ -d "$directory" && ! -L "$directory" ]]; then
            rmdir "$directory" 2>/dev/null || true
        fi
    done <<< "$loaded_absent_parent_dirs"
}

load_recovery() {
    local recovery_input="$1" require_checkouts="${2:-0}"
    local recovery target_home os stage old_checkout new_checkout old_source new_source
    local captured_old_commit new_commit new_tag_object current_nix
    local expected_old_targets expected_new_targets
    recovery="$(canonical_directory "$recovery_input")" || {
        fail "recovery directory is missing or unsafe: $recovery_input"
        return 1
    }
    assert_private_permissions "$recovery" || return 1
    for file in stage RECOVERY.txt; do
        [[ -f "$recovery/$file" && ! -L "$recovery/$file" ]] || {
            fail "recovery directory is incomplete or unsafe: $file"
            return 1
        }
        assert_private_permissions "$recovery/$file" || return 1
    done
    os="$(read_recovery_scalar "$recovery/platform")" || return 1
    [[ "$os" == "Darwin" || "$os" == "Linux" ]] || {
        fail "recovery platform is invalid: $os"
        return 1
    }
    [[ "$(uname -s)" == "$os" ]] || {
        fail "recovery platform $os does not match this host."
        return 1
    }
    verify_payload_manifest "$recovery" "$os" || return 1
    verify_release_tree "$recovery" old || return 1
    verify_release_tree "$recovery" new || return 1
    stage="$(read_recovery_scalar "$recovery/stage")" || return 1
    case "$stage" in
        prepared|applying|applied|rolling-back|rolled-back|recovery-required|accepted) ;;
        *) fail "recovery stage is invalid: $stage"; return 1 ;;
    esac
    target_home="$(read_recovery_scalar "$recovery/target-home")" || return 1
    [[ "$(canonical_directory "$HOME")" == "$target_home" ]] || {
        fail "recovery belongs to HOME=$target_home, not $HOME"
        return 1
    }
    old_checkout="$(read_recovery_scalar "$recovery/old-checkout")" || return 1
    new_checkout="$(read_recovery_scalar "$recovery/new-checkout")" || return 1
    [[ "$old_checkout" == /* && "$new_checkout" == /* && "$old_checkout" != "$new_checkout" ]] || {
        fail "recovery checkout identities are malformed."
        return 1
    }
    captured_old_commit="$(read_recovery_scalar "$recovery/old-commit")" || return 1
    new_commit="$(read_recovery_scalar "$recovery/new-commit")" || return 1
    new_tag_object="$(read_recovery_scalar "$recovery/new-tag-object")" || return 1
    [[ "$captured_old_commit" == "$old_commit" && "$new_commit" =~ ^[0-9a-f]{40}$ && \
        "$new_tag_object" =~ ^[0-9a-f]{40}$ ]] || {
        fail "recovery release identity is malformed."
        return 1
    }
    if [[ "$require_checkouts" -eq 1 ]]; then
        old_checkout="$(canonical_directory "$old_checkout")" || {
            fail "retained v0.1.0 checkout is missing or unsafe."
            return 1
        }
        new_checkout="$(canonical_directory "$new_checkout")" || {
            fail "retained v0.2.0 checkout is missing or unsafe."
            return 1
        }
        assert_local_release_checkout "$old_checkout" "$old_tag" "$old_commit" \
            "$old_tag_object" "old" || return 1
        assert_local_release_checkout "$new_checkout" "$new_tag" "$new_commit" \
            "$new_tag_object" "new" || return 1
    fi
    old_source="$recovery/old-release"
    new_source="$recovery/new-release"
    cmp -s "$recovery/flake.lock" "$new_source/flake.lock" || {
        fail "captured flake.lock differs from the exact v0.2.0 release."
        return 1
    }
    cmp -s "$recovery/upgrade-v0.1.0.sh" "$new_source/scripts/upgrade-v0.1.0.sh" || {
        fail "recovery script differs from the exact v0.2.0 release."
        return 1
    }
    expected_old_targets="$(chezmoi --source "$old_source/home" --destination "$HOME" \
        managed --path-style absolute --include files,symlinks)" || return 1
    expected_new_targets="$(chezmoi --source "$new_source/home" --destination "$HOME" \
        managed --path-style absolute --include files,symlinks)" || return 1
    [[ "$(< "$recovery/old-targets.txt")" == "$expected_old_targets" && \
        "$(< "$recovery/new-targets.txt")" == "$expected_new_targets" ]] || {
        fail "captured config target inventory differs from the exact release sources."
        return 1
    }
    current_nix="$(command -v nix 2>/dev/null || true)"
    loaded_nix_command="$(read_recovery_scalar "$recovery/nix-command")" || return 1
    [[ -n "$current_nix" && "$loaded_nix_command" == "$current_nix" && \
        -x "$loaded_nix_command" ]] || {
        fail "captured Nix command differs from the current prerequisite provider."
        return 1
    }
    loaded_providers_before="$(< "$recovery/providers.before")"
    validate_provider_inventory "$loaded_providers_before" || return 1
    loaded_flake_lock="$(< "$recovery/flake.lock")"
    loaded_new_targets="$expected_new_targets"
    loaded_absent_parent_dirs="$(< "$recovery/absent-parent-dirs.before")"
    validate_absent_parent_inventory "$loaded_absent_parent_dirs" \
        "$loaded_new_targets" || {
        fail "captured absent-parent inventory is malformed."
        return 1
    }
    loaded_brew_repository=""
    loaded_brew_formulae=""
    loaded_brew_casks=""
    loaded_tap_backups=""
    if [[ "$os" == "Darwin" ]]; then
        loaded_brew_repository="$(read_recovery_scalar "$recovery/brew-repository")" || return 1
        [[ "$loaded_brew_repository" == "$(brew --repository)" ]] || {
            fail "captured Homebrew repository differs from the current installation."
            return 1
        }
        loaded_brew_formulae="$(< "$recovery/brew-formulae.before")"
        loaded_brew_casks="$(< "$recovery/brew-casks.before")"
        loaded_tap_backups="$(< "$recovery/tap-backups.before")"
    fi
    loaded_recovery="$recovery"
    loaded_old_source="$old_source"
    loaded_new_source="$new_source"
    loaded_platform="$os"
}

perform_rollback() {
    local recovery_input="$1" recovery old_source new_source os stage failures=0
    load_recovery "$recovery_input" || return 1
    recovery="$loaded_recovery"
    stage="$(read_recovery_scalar "$recovery/stage")" || return 1
    if [[ "$stage" == "accepted" ]]; then
        fail "this migration was explicitly accepted; automatic rollback authority has ended."
        return 1
    fi
    old_source="$loaded_old_source"
    new_source="$loaded_new_source"
    os="$loaded_platform"
    rollback_running=1
    set_stage "$recovery" rolling-back
    restore_old_config "$recovery" "$old_source" "$new_source" || failures=1
    rollback_package_layer "$recovery" "$os" || failures=1
    verify_provider_boundary "$recovery" || failures=1
    if [[ "$failures" -ne 0 ]] || ! verify_old_config "$old_source"; then
        set_stage "$recovery" recovery-required
        echo "RECOVERY REQUIRED: $recovery/upgrade-v0.1.0.sh --rollback '$recovery'" >&2
        return 1
    fi
    set_stage "$recovery" rolled-back
    echo "v0.1.0 config and the pre-upgrade package-provider boundary were restored."
    echo "Recovery evidence retained at: $recovery"
}

verify_new_state() {
    local recovery="$1" new_checkout="$2" os="$3"
    chezmoi --source "$new_checkout/home" --destination "$HOME" \
        verify --include files,symlinks >/dev/null
    if [[ "$os" == "Darwin" ]]; then
        nix_darwin_state_exists
    else
        home_manager_state_exists
    fi
    capture_providers "$recovery/providers.after"
}

perform_accept() {
    local recovery_input="$1" recovery stage os
    load_recovery "$recovery_input" 1 || return 1
    recovery="$loaded_recovery"
    stage="$(read_recovery_scalar "$recovery/stage")" || return 1
    [[ "$stage" == "applied" ]] || {
        fail "only an applied migration can be accepted; current stage is $stage."
        return 1
    }
    os="$loaded_platform"
    verify_new_state "$recovery" "$loaded_new_source" "$os" || {
        fail "v0.2.0 verification failed; keep both checkouts and recovery material."
        return 1
    }
    set_stage "$recovery" accepted
    echo "Migration core accepted. Keep v0.1.0 until full v0.2.0 setup succeeds."
    echo "Keep $recovery until independent application and data checks are complete."
}

handle_exit() {
    local rc=$? rollback_rc=0
    trap - EXIT HUP INT TERM
    if [[ "$transaction_active" -eq 1 && "$rollback_running" -eq 0 && -n "$active_recovery" ]]; then
        transaction_active=0
        echo "Upgrade failed after mutation began; restoring v0.1.0." >&2
        perform_rollback "$active_recovery" || rollback_rc=$?
        if [[ "$rollback_rc" -ne 0 ]]; then
            echo "RECOVERY REQUIRED: $active_recovery/upgrade-v0.1.0.sh --rollback '$active_recovery'" >&2
        fi
    elif [[ -n "$preparing_recovery" && -d "$preparing_recovery" ]]; then
        remove_private_tree "$preparing_recovery"
    fi
    exit "$rc"
}

perform_apply() {
    local old_input="$1" old_checkout new_checkout os
    run_preflight "$old_input" "$default_new_checkout" || return 1
    old_checkout="$preflight_old_checkout"
    new_checkout="$preflight_new_checkout"
    os="$preflight_platform"
    trap handle_exit EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    prepare_recovery "$old_checkout" "$new_checkout" "$os"
    active_recovery="$prepared_recovery"
    echo "Recovery directory: $active_recovery"
    if ! set_stage "$active_recovery" applying; then
        remove_private_tree "$active_recovery"
        preparing_recovery=""
        active_recovery=""
        trap - EXIT HUP INT TERM
        return 1
    fi
    preparing_recovery=""
    transaction_active=1
    DOTFILES_RELEASE_MIGRATION_ACTIVE=1 \
        "$active_recovery/new-release/setup.sh" --all --skip-native-deps \
        --skip-config-scripts --skip-nvim --skip-agents
    verify_new_state "$active_recovery" "$active_recovery/new-release" "$os"
    set_stage "$active_recovery" applied
    transaction_active=0
    trap - EXIT HUP INT TERM
    echo "v0.2.0 migration applied and verified."
    echo "Retain both checkouts and recovery material until explicit acceptance:"
    echo "  $active_recovery/upgrade-v0.1.0.sh --accept '$active_recovery'"
}

[[ "$#" -eq 2 ]] || {
    usage >&2
    exit 2
}

mode="$1"
argument="$2"
case "$mode" in
    --preflight-only)
        run_preflight "$argument" "$default_new_checkout"
        echo "v0.1.0 -> v0.2.0 preflight passed; no live state changed."
        printf 'old=%s\nnew=%s\nplatform=%s\n' \
            "$preflight_old_checkout" \
            "$preflight_new_checkout" \
            "$preflight_platform"
        ;;
    --apply)
        perform_apply "$argument"
        ;;
    --rollback)
        perform_rollback "$argument"
        ;;
    --accept)
        perform_accept "$argument"
        ;;
    *)
        usage >&2
        exit 2
        ;;
esac
