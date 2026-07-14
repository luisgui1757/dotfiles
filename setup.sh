#!/usr/bin/env bash
# setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.
#
# Local usage (from a checked-out copy):
#   ./setup.sh                     interactive: dependency prompts, then config + sync
#   ./setup.sh --all               non-interactive: install or migrate, then reconcile everything
#   ./setup.sh --update            reconcile the release, then refresh proven tools + Mason
#   ./setup.sh --upgrade           alias for --update
#   ./setup.sh --dry-run           preview every step
#   ./setup.sh --skip-deps         already provisioned; skip Nix + native deps
#   ./setup.sh --skip-native-deps  keep Nix activation; skip native/deferred deps
#   ./setup.sh --skip-bootstrap    back-compat alias: skip config apply
#   ./setup.sh --skip-config       already configured; just sync nvim
#   ./setup.sh --skip-config-scripts
#                                  apply files/links without chezmoi run scripts
#   ./setup.sh --skip-nvim         skip nvim plugin/parser/Mason sync
#   ./setup.sh --skip-agents       skip global Sentinel agent-policy install
#   ./setup.sh --experimental-wsl-gui
#                                  WSL opt-in: install/link Linux GUI terminal bits
#
# First run (no checkout yet):
#   git clone --branch v0.2.0 --single-branch https://github.com/luisgui1757/dotfiles.git "${DOTFILES_DEST:-$HOME/dotfiles}"
#   cd "${DOTFILES_DEST:-$HOME/dotfiles}"
#   ./setup.sh --all
#
# Set DOTFILES_DEST=/some/other/path in the environment if you want a
# different checkout location.

set -euo pipefail

REPO_URL="https://github.com/luisgui1757/dotfiles.git"
RELEASE_TAG="v0.2.0"
DEFAULT_DEST="$HOME/dotfiles"

ALL=0
DRY_RUN=0
UPDATE_MODE=0
SKIP_DEPS=0
SKIP_NATIVE_DEPS=0
SKIP_BOOTSTRAP=0
SKIP_CONFIG_SCRIPTS=0
SKIP_NVIM=0
SKIP_AGENTS=0
BEST_EFFORT=0
EXPERIMENTAL_WSL_GUI=0
NIX_DARWIN=0
HOME_MANAGER=0
NIX_DARWIN_RC_SOURCES=()
NIX_DARWIN_RC_BACKUPS=()
NIX_HOMEBREW_LEGACY_TAPS=()
NIX_HOMEBREW_LEGACY_BACKUPS=()
NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT=""
SENTINEL_REPO_URL="https://github.com/luisgui1757/sentinel.git"
SENTINEL_VERSION="0.1.2"
SENTINEL_REF="ecafffa858666343c1639f996d177f460163e93e"
SENTINEL_CACHE_ROOT="$HOME/.local/share/dotfiles/sentinel"
V0_1_COMMIT="015617362830280bf85c7142e69d0681d376d453"
V0_1_TAG_OBJECT="a3b4d6d7b6d289959cac68d76faec96219b3e310"
V0_1_RECOVERY_PATH=""
V0_1_RECOVERY_STAGE=""
V0_1_RECOVERY_OLD_CHECKOUT=""
COMPLETED_V0_1_RECOVERY=""
NIX_PREREQUISITE_DRY_RUN_PLANNED=0
usage() {
    cat <<'EOF'
setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.

Local usage:
  ./setup.sh                     interactive: dependency prompts, then config + sync
  ./setup.sh --all               non-interactive: install or migrate, then reconcile everything
  ./setup.sh --update            reconcile the release, then refresh proven tools + Mason
  ./setup.sh --upgrade           alias for --update
  ./setup.sh --dry-run           preview every step
  ./setup.sh --skip-deps         already provisioned; skip Nix + native deps
  ./setup.sh --skip-native-deps  apply Nix/config but skip native/deferred deps
  ./setup.sh --skip-bootstrap    back-compat alias: skip config apply
  ./setup.sh --skip-config       already configured; just sync nvim
  ./setup.sh --skip-config-scripts
                                apply files/links without chezmoi run scripts
  ./setup.sh --skip-nvim         skip nvim plugin/parser/Mason sync
  ./setup.sh --skip-agents       skip global Sentinel agent-policy install
  ./setup.sh --best-effort       continue past plugin/LSP/Mason phase failures
  ./setup.sh --experimental-wsl-gui
                                WSL opt-in: install/link Linux Ghostty + Linux fonts
  ./setup.sh --nix-darwin        compatibility alias; macOS setup applies nix-darwin by default
  ./setup.sh --home-manager      compatibility alias; Linux/WSL setup applies Home Manager by default

First run:
  git clone --branch v0.2.0 --single-branch https://github.com/luisgui1757/dotfiles.git "${DOTFILES_DEST:-$HOME/dotfiles}"
  cd "${DOTFILES_DEST:-$HOME/dotfiles}"
  ./setup.sh --all
EOF
}

for arg in "$@"; do
    case "$arg" in
        --all|-y)         ALL=1 ;;
        --dry-run)        DRY_RUN=1 ;;
        --update|--upgrade)
                          UPDATE_MODE=1; ALL=1 ;;
        --skip-deps)      SKIP_DEPS=1 ;;
        --skip-native-deps)
                          SKIP_NATIVE_DEPS=1 ;;
        --skip-bootstrap|--skip-config)
                          SKIP_BOOTSTRAP=1 ;;
        --skip-config-scripts)
                          SKIP_CONFIG_SCRIPTS=1 ;;
        --skip-nvim)      SKIP_NVIM=1 ;;
        --skip-agents)    SKIP_AGENTS=1 ;;
        --best-effort)    BEST_EFFORT=1 ;;
        --experimental-wsl-gui)
                          EXPERIMENTAL_WSL_GUI=1 ;;
        --nix-darwin)     NIX_DARWIN=1 ;;
        --home-manager)   HOME_MANAGER=1 ;;
        -h|--help)        usage; exit 0 ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done
if [[ "$EXPERIMENTAL_WSL_GUI" -eq 1 ]]; then
    export DOTFILES_EXPERIMENTAL_WSL_GUI=1
fi
if [[ "$ALL" -eq 0 && "$DRY_RUN" -eq 0 && ! -t 0 ]]; then
    ALL=1
    echo "note: no TTY detected; running with --all"
    set -- "$@" --all
fi

# ---- Locate the repo ---------------------------------------------------------
# Piped/remote setup is intentionally disabled. Running from stdin would execute
# mutable default-branch code before a local checkout exists, so setup fails
# closed and tells the user how to clone first.
SCRIPT_DIR=""
if [[ -n "${BASH_SOURCE[0]:-}" ]] && [[ -f "${BASH_SOURCE[0]:-}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
fi
if [[ -z "$SCRIPT_DIR" ]] || [[ ! -d "$SCRIPT_DIR/home" ]]; then
    DEST="${DOTFILES_DEST:-$DEFAULT_DEST}"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "setup.sh: no local checkout was detected; remote/piped setup is disabled."
        echo "  Clone first, then run setup locally:"
        echo "    git clone --branch $RELEASE_TAG --single-branch $REPO_URL \"$DEST\""
        echo "    cd \"$DEST\""
        echo "    ./setup.sh --all"
        echo "(dry run -- no clone, no install, no writes performed)"
    fi
    echo "setup.sh must be run from a local clone. Remote/piped clone-and-reinvoke setup is disabled because it would execute mutable default-branch code." >&2
    echo "Clone first, then run setup locally:" >&2
    echo "  git clone --branch $RELEASE_TAG --single-branch $REPO_URL \"$DEST\"" >&2
    echo "  cd \"$DEST\"" >&2
    echo "  ./setup.sh --all" >&2
    exit 1
fi

cd "$SCRIPT_DIR"

# ---- Forward flags to sub-scripts --------------------------------------------
DEPS_FLAGS=()
[[ "$ALL" -eq 1 ]]      && DEPS_FLAGS+=(--all)
[[ "$DRY_RUN" -eq 1 ]]  && DEPS_FLAGS+=(--dry-run)
[[ "$EXPERIMENTAL_WSL_GUI" -eq 1 ]] && DEPS_FLAGS+=(--experimental-wsl-gui)

CHEZMOI_SOURCE="$SCRIPT_DIR/home"
CHEZMOI_BASE_ARGS=(--source "$CHEZMOI_SOURCE")
CHEZMOI_CONFIG_ARGS=()
CHEZMOI_DATA_ARGS=()
CHEZMOI_APPLY_ARGS=()
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
if [[ "$EXPERIMENTAL_WSL_GUI" -eq 1 ]]; then
    CHEZMOI_DATA_ARGS+=(--override-data '{"experimentalWslGui":true}')
fi
if [[ "$SKIP_CONFIG_SCRIPTS" -eq 1 ]]; then
    CHEZMOI_APPLY_ARGS+=(--include "files,symlinks")
fi

phase() {
    echo
    echo "================================================================"
    echo "==  $1"
    echo "================================================================"
}

release_git() {
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

activate_nix_profile() {
    local profile nix_bin nix_bin_dir
    command -v nix >/dev/null 2>&1 && return 0

    # A login shell may already have sourced nix-daemon.sh, after which
    # Homebrew's path_helper can replace PATH while the upstream profile guard
    # remains set. Re-sourcing then correctly no-ops. Recover the two canonical
    # profile binaries directly before asking the guarded scripts to run.
    for nix_bin in \
        /nix/var/nix/profiles/default/bin/nix \
        "$HOME/.nix-profile/bin/nix"; do
        [[ -x "$nix_bin" ]] || continue
        nix_bin_dir="$(dirname "$nix_bin")"
        PATH="$nix_bin_dir:$PATH"
        export PATH
        command -v nix >/dev/null 2>&1 && return 0
    done

    for profile in \
        /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh \
        "$HOME/.nix-profile/etc/profile.d/nix.sh"; do
        if [[ -f "$profile" ]]; then
            # Both paths are fixed outputs of the verified upstream installer.
            # shellcheck disable=SC1090
            source "$profile"
            command -v nix >/dev/null 2>&1 && return 0
        fi
    done
    return 1
}

ensure_nix_prerequisite() {
    local helper="$SCRIPT_DIR/scripts/install-nix-prerequisite.sh"
    [[ "$SKIP_DEPS" -eq 0 ]] || return 0
    if activate_nix_profile; then
        nix --version >/dev/null
        if nix store info >/dev/null 2>&1; then
            return 0
        fi
        # A prior upstream install may have completed before the helper proved
        # nix-command. Let the identity-checked helper reconcile that config.
        [[ -f "$helper" && ! -L "$helper" && -x "$helper" ]] || {
            echo "  FAIL: verified Nix prerequisite helper is missing or unsafe: $helper" >&2
            return 1
        }
        "$helper" --install
        nix store info >/dev/null
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        NIX_PREREQUISITE_DRY_RUN_PLANNED=1
        echo "  would: install and verify the release-pinned Nix prerequisite"
        return 0
    fi
    [[ -f "$helper" && ! -L "$helper" && -x "$helper" ]] || {
        echo "  FAIL: verified Nix prerequisite helper is missing or unsafe: $helper" >&2
        return 1
    }
    if [[ "$ALL" -eq 0 && -t 0 ]] &&
        ! ask_yes_no_default_yes "Install the required verified Nix prerequisite?"; then
        echo "  FAIL: setup requires Nix on macOS/Linux/WSL." >&2
        return 1
    fi
    "$helper" --install
    activate_nix_profile || {
        echo "  FAIL: Nix installation returned success but setup cannot activate it." >&2
        return 1
    }
    nix --version >/dev/null
    nix store info >/dev/null
    echo "  ok        Nix prerequisite installed and active"
}

canonical_checkout_directory() {
    local path="$1"
    [[ -d "$path" && ! -L "$path" ]] || return 1
    (cd "$path" && pwd -P)
}

exact_v0_1_checkout() {
    local candidate="$1" head tag_object
    candidate="$(canonical_checkout_directory "$candidate" 2>/dev/null)" || return 1
    head="$(release_git "$candidate" rev-parse --verify 'HEAD^{commit}' 2>/dev/null || true)"
    tag_object="$(release_git "$candidate" rev-parse --verify refs/tags/v0.1.0 2>/dev/null || true)"
    [[ "$head" == "$V0_1_COMMIT" && "$tag_object" == "$V0_1_TAG_OBJECT" ]]
}

symlink_target_absolute() {
    local link="$1" target parent
    [[ -L "$link" ]] || return 1
    target="$(readlink "$link")" || return 1
    if [[ "$target" == /* ]]; then
        printf '%s\n' "$target"
        return 0
    fi
    parent="$(cd "$(dirname "$link")" && pwd -P)" || return 1
    printf '%s/%s\n' "$parent" "$target"
}

v0_1_candidate_from_live_config() {
    local target parent candidate
    if target="$(symlink_target_absolute "$HOME/.config/nvim" 2>/dev/null)" &&
        [[ -d "$target" ]]; then
        candidate="$(dirname "$(cd "$target" && pwd -P)")"
        exact_v0_1_checkout "$candidate" && {
            canonical_checkout_directory "$candidate"
            return 0
        }
    fi
    if target="$(symlink_target_absolute "$HOME/.zshrc" 2>/dev/null)" &&
        [[ -e "$target" ]]; then
        parent="$(cd "$(dirname "$target")" && pwd -P)"
        if [[ "$(basename "$parent")" == home ]]; then
            candidate="$(dirname "$parent")"
            exact_v0_1_checkout "$candidate" && {
                canonical_checkout_directory "$candidate"
                return 0
            }
        fi
    fi
    return 1
}

read_setup_recovery_scalar() {
    local file="$1"
    [[ -f "$file" && ! -L "$file" ]] || return 1
    python3 -c '
import pathlib, sys
raw = pathlib.Path(sys.argv[1]).read_bytes()
if (not raw.endswith(b"\n") or raw.count(b"\n") != 1 or b"\r" in raw
        or b"\0" in raw or len(raw) == 1):
    raise SystemExit(1)
sys.stdout.buffer.write(raw[:-1])
' "$file"
}

load_pending_v0_1_recovery() {
    local root directory stage new_checkout old_checkout
    local -a active=() rolled_back=()
    V0_1_RECOVERY_PATH=""
    V0_1_RECOVERY_STAGE=""
    V0_1_RECOVERY_OLD_CHECKOUT=""
    root="${XDG_STATE_HOME:-$HOME/.local/state}/dotfiles/migrations"
    [[ -d "$root" && ! -L "$root" ]] || return 0
    while IFS= read -r -d '' directory; do
        [[ -d "$directory" && ! -L "$directory" ]] || {
            echo "  FAIL: migration recovery path is not a real directory: $directory" >&2
            return 1
        }
        if ! stage="$(read_setup_recovery_scalar "$directory/stage")" ||
            ! new_checkout="$(read_setup_recovery_scalar "$directory/new-checkout")" ||
            ! old_checkout="$(read_setup_recovery_scalar "$directory/old-checkout")"; then
            echo "  FAIL: migration recovery identity is incomplete or unsafe: $directory" >&2
            return 1
        fi
        [[ "$new_checkout" == "$SCRIPT_DIR" ]] || continue
        case "$stage" in
            prepared|applying|applied|rolling-back|recovery-required)
                active+=("$directory")
                ;;
            rolled-back)
                rolled_back+=("$old_checkout")
                ;;
            accepted)
                ;;
            *)
                echo "  FAIL: migration recovery stage is invalid: $directory ($stage)" >&2
                return 1
                ;;
        esac
    done < <(find "$root" -mindepth 1 -maxdepth 1 \
        -name 'v0.1.0-to-v0.2.0.*' -print0)
    if [[ "${#active[@]}" -gt 1 ]]; then
        echo "  FAIL: multiple unfinished v0.1.0 migrations target this checkout." >&2
        printf '        %s\n' "${active[@]}" >&2
        return 1
    fi
    if [[ "${#active[@]}" -eq 1 ]]; then
        V0_1_RECOVERY_PATH="${active[0]}"
        V0_1_RECOVERY_STAGE="$(read_setup_recovery_scalar "$V0_1_RECOVERY_PATH/stage")"
        V0_1_RECOVERY_OLD_CHECKOUT="$(read_setup_recovery_scalar "$V0_1_RECOVERY_PATH/old-checkout")"
        return 0
    fi
    if [[ "${#rolled_back[@]}" -gt 0 ]]; then
        local unique=""
        for old_checkout in "${rolled_back[@]}"; do
            if [[ -z "$unique" ]]; then
                unique="$old_checkout"
            elif [[ "$unique" != "$old_checkout" ]]; then
                echo "  FAIL: rolled-back migrations disagree about the v0.1.0 checkout." >&2
                return 1
            fi
        done
        V0_1_RECOVERY_STAGE="rolled-back"
        V0_1_RECOVERY_OLD_CHECKOUT="$unique"
    fi
}

run_v0_1_migrator() {
    local mode="$1" argument="$2"
    DOTFILES_RELEASE_MIGRATION_ACTIVE=1 \
        "$SCRIPT_DIR/scripts/upgrade-v0.1.0.sh" "$mode" "$argument"
}

maybe_complete_v0_1_upgrade() {
    local old_checkout="" output recovery count
    [[ -z "${DOTFILES_RELEASE_MIGRATION_ACTIVE:-}" ]] || return 0
    [[ "$SKIP_DEPS" -eq 0 ]] || return 0
    load_pending_v0_1_recovery || return 1
    case "$V0_1_RECOVERY_STAGE" in
        applied)
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: verify and accept the pending v0.1.0 migration at $V0_1_RECOVERY_PATH"
                return 0
            fi
            run_v0_1_migrator --accept "$V0_1_RECOVERY_PATH"
            COMPLETED_V0_1_RECOVERY="$V0_1_RECOVERY_PATH"
            return 0
            ;;
        prepared|applying|rolling-back|recovery-required)
            echo "  FAIL: unfinished v0.1.0 migration requires recovery first: $V0_1_RECOVERY_PATH" >&2
            echo "        $V0_1_RECOVERY_PATH/upgrade-v0.1.0.sh --rollback '$V0_1_RECOVERY_PATH'" >&2
            return 1
            ;;
        rolled-back)
            old_checkout="$V0_1_RECOVERY_OLD_CHECKOUT"
            ;;
    esac
    if [[ -n "${DOTFILES_V0_1_CHECKOUT:-}" ]]; then
        exact_v0_1_checkout "$DOTFILES_V0_1_CHECKOUT" || {
            echo "  FAIL: DOTFILES_V0_1_CHECKOUT is not the exact v0.1.0 checkout: $DOTFILES_V0_1_CHECKOUT" >&2
            return 1
        }
        old_checkout="$(canonical_checkout_directory "$DOTFILES_V0_1_CHECKOUT")"
    elif [[ -z "$old_checkout" ]]; then
        old_checkout="$(v0_1_candidate_from_live_config 2>/dev/null || true)"
    fi
    [[ -n "$old_checkout" ]] || return 0
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: transactionally migrate exact v0.1.0 state from $old_checkout"
        return 0
    fi
    if [[ "$ALL" -eq 0 && -t 0 ]] &&
        ! ask_yes_no_default_yes "Migrate the detected v0.1.0 installation and continue?"; then
        echo "  retained  v0.1.0 unchanged at $old_checkout"
        return 1
    fi
    output="$(run_v0_1_migrator --apply "$old_checkout" 2>&1)" || {
        local rc=$?
        printf '%s\n' "$output" >&2
        return "$rc"
    }
    printf '%s\n' "$output"
    recovery="$(printf '%s\n' "$output" | awk -F': ' '/^Recovery directory:/ { print $2 }')"
    count="$(printf '%s\n' "$output" | grep -c '^Recovery directory:' || true)"
    [[ "$count" -eq 1 && -n "$recovery" && -d "$recovery" && ! -L "$recovery" ]] || {
        echo "  FAIL: migration succeeded without one valid recovery identity." >&2
        return 1
    }
    run_v0_1_migrator --accept "$recovery"
    COMPLETED_V0_1_RECOVERY="$recovery"
    echo "  accepted  verified v0.1.0 core migration; recovery retained at $recovery"
}

account_home_directory() {
    local user="$1" record record_user home=""
    case "$(uname -s)" in
        Darwin)
            command -v dscl >/dev/null 2>&1 || return 1
            record="$(dscl . -read "/Users/$user" NFSHomeDirectory 2>/dev/null)" || return 1
            home="${record#NFSHomeDirectory:}"
            home="${home#"${home%%[![:space:]]*}"}"
            ;;
        Linux)
            if command -v getent >/dev/null 2>&1; then
                record="$(getent passwd "$user" 2>/dev/null)" || return 1
            elif [[ -r /etc/passwd ]]; then
                record="$(awk -F: -v wanted="$user" '$1 == wanted { print; exit }' /etc/passwd)"
            fi
            [[ -n "$record" ]] || return 1
            case "$record" in *$'\n'*) return 1 ;; esac
            record_user="${record%%:*}"
            [[ "$record_user" == "$user" ]] || return 1
            home="$(printf '%s\n' "$record" | awk -F: '{ print $6 }')"
            ;;
        *) return 1 ;;
    esac
    [[ "$home" == /* && "$home" != "/" && "$home" != *$'\n'* ]] || return 1
    printf '%s\n' "$home"
}

resolve_target_identity() {
    local uid user account_home env_home_real account_home_real
    uid="$(id -u 2>/dev/null)" || {
        echo "  FAIL: could not resolve the invoking POSIX user id." >&2
        return 1
    }
    case "$uid" in
        ''|*[!0-9]*)
            echo "  FAIL: invoking POSIX user id is not numeric: $uid" >&2
            return 1
            ;;
    esac
    if [[ "$uid" -eq 0 ]]; then
        echo "  FAIL: do not run setup.sh itself as root or through sudo." >&2
        echo "        Run it as the target user; setup invokes sudo only for the bounded system steps." >&2
        return 1
    fi
    user="$(id -un 2>/dev/null)" || {
        echo "  FAIL: could not resolve the invoking POSIX username." >&2
        return 1
    }
    case "$user" in
        ''|root|*/*|*:*|*' '*|*$'\t'*|*$'\r'*|*$'\n'*)
        echo "  FAIL: refusing ambiguous target username: ${user:-<empty>}" >&2
        return 1
        ;;
    esac
    account_home="$(account_home_directory "$user")" || {
        echo "  FAIL: could not resolve the authoritative home directory for $user." >&2
        echo "        Check the local/directory-service account record, then re-run setup." >&2
        return 1
    }
    if [[ ! -d "$account_home" ]]; then
        echo "  FAIL: the resolved home directory does not exist: $account_home" >&2
        return 1
    fi
    if [[ -z "${HOME:-}" || ! -d "$HOME" ]]; then
        echo "  FAIL: HOME is unset or not a directory; resolved account home is $account_home" >&2
        return 1
    fi
    env_home_real="$(cd "$HOME" && pwd -P)"
    account_home_real="$(cd "$account_home" && pwd -P)"
    if [[ "$env_home_real" != "$account_home_real" ]]; then
        echo "  FAIL: HOME targets $env_home_real but account $user resolves to $account_home_real." >&2
        echo "        Refusing to split Nix, chezmoi, and setup across different homes." >&2
        return 1
    fi

    DOTFILES_TARGET_USER="$user"
    DOTFILES_TARGET_HOME="$account_home"
    export DOTFILES_TARGET_USER DOTFILES_TARGET_HOME
    HOME="$account_home"
    export HOME
    DEFAULT_DEST="$HOME/dotfiles"
    SENTINEL_CACHE_ROOT="$HOME/.local/share/dotfiles/sentinel"
}

refresh_runtime_path() {
    local brew_bin brew_env dir make_prefix gnubin nix_user

    [[ "$DRY_RUN" -eq 1 ]] && return 0

    for brew_bin in "$(command -v brew 2>/dev/null || true)" \
        /opt/homebrew/bin/brew \
        /usr/local/bin/brew \
        "$HOME/.linuxbrew/bin/brew" \
        /home/linuxbrew/.linuxbrew/bin/brew; do
        if [[ -n "$brew_bin" && -x "$brew_bin" ]]; then
            if brew_env="$("$brew_bin" shellenv)"; then
                eval "$brew_env"
                make_prefix="$("$brew_bin" --prefix make 2>/dev/null || true)"
                gnubin="$make_prefix/libexec/gnubin"
                if [[ -n "$make_prefix" && -d "$gnubin" && ":$PATH:" != *":$gnubin:"* ]]; then
                    PATH="$gnubin:$PATH"
                fi
            else
                echo "  WARN: $brew_bin shellenv failed; leaving PATH unchanged for that Homebrew prefix" >&2
            fi
            break
        fi
    done

    nix_user="${DOTFILES_TARGET_USER:-$(id -un 2>/dev/null || true)}"
    for dir in \
        /nix/var/nix/profiles/default/bin \
        /run/current-system/sw/bin \
        ${nix_user:+"/etc/profiles/per-user/$nix_user/bin"} \
        "$HOME/.nix-profile/bin" \
        "$HOME/.local/state/nix/profile/bin"; do
        if [[ -n "$dir" && -d "$dir" && ":$PATH:" != *":$dir:"* ]]; then
            PATH="$dir:$PATH"
        fi
    done

    for dir in /usr/local/bin "$HOME/.local/bin"; do
        if [[ -d "$dir" && ":$PATH:" != *":$dir:"* ]]; then
            PATH="$PATH:$dir"
        fi
    done
    export PATH
    hash -r 2>/dev/null || true
}

sentinel_checkout_dir() {
    printf '%s/%s\n' "$SENTINEL_CACHE_ROOT" "$SENTINEL_REF"
}

sentinel_git() {
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_CONFIG_SYSTEM=/dev/null \
    GIT_CONFIG_GLOBAL=/dev/null \
    GIT_CONFIG_COUNT=0 \
    GIT_CONFIG_PARAMETERS='' \
    GIT_TEMPLATE_DIR='' \
        git \
        -c core.fsmonitor=false \
        -c core.untrackedCache=false \
        -c core.hooksPath=/dev/null \
        -c init.templateDir= \
        "$@"
}

sentinel_cache_git() {
    local checkout="$1"
    shift
    sentinel_git --git-dir="$checkout/.git" --work-tree="$checkout" "$@"
}

assert_sentinel_checkout_clean() {
    local checkout="$1" status

    if ! status="$(sentinel_cache_git "$checkout" status --porcelain=v1 --untracked-files=all --ignored=matching 2>/dev/null)"; then
        echo "  FAIL: could not inspect Sentinel cache worktree: $checkout" >&2
        exit 1
    fi

    if [[ -n "$status" ]]; then
        echo "  FAIL: Sentinel cache has local changes; refusing to execute it: $checkout" >&2
        printf '%s\n' "$status" | sed 's/^/        /' >&2
        echo "        Remove this cache directory and rerun setup to fetch the pinned checkout again." >&2
        exit 1
    fi
}

assert_sentinel_artifact() {
    local checkout="$1" version="$2" ref="$3" head checkout_version

    head="$(sentinel_cache_git "$checkout" rev-parse --verify 'HEAD^{commit}' 2>/dev/null || true)"
    if [[ "$head" != "$ref" ]]; then
        echo "  FAIL: Sentinel cache is not at the pinned commit: $checkout" >&2
        echo "        expected $ref, found ${head:-unknown}" >&2
        exit 1
    fi

    checkout_version="$(tr -d '[:space:]' < "$checkout/VERSION" 2>/dev/null || true)"
    if [[ "$checkout_version" != "$version" ]]; then
        echo "  FAIL: Sentinel cache VERSION mismatch: expected $version, found ${checkout_version:-missing}" >&2
        exit 1
    fi
}

ask_yes_no_default_yes() {
    local prompt="$1" reply
    printf "  %s [Y/n] " "$prompt"
    IFS= read -r reply || return 1
    case "$reply" in
        ""|[Yy]|[Yy][Ee][Ss]) return 0 ;;
        *) return 1 ;;
    esac
}

should_apply_agent_policy() {
    [[ "$SKIP_AGENTS" -eq 0 ]] || return 1
    [[ "$ALL" -eq 1 || "$DRY_RUN" -eq 1 ]] && return 0
    [[ -t 0 ]] || return 0
    ask_yes_no_default_yes "Apply Sentinel global agent rules?"
}

ensure_sentinel_checkout() (
    local checkout tmp="" cleanup_rc=0
    SENTINEL_STAGE_DIR=""
    trap '
        cleanup_rc=$?
        trap - EXIT HUP INT TERM
        if [[ -n "${SENTINEL_STAGE_DIR:-}" ]]; then
            rm -rf "$SENTINEL_STAGE_DIR"
        fi
        exit "$cleanup_rc"
    ' EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM

    checkout="$(sentinel_checkout_dir)"

    if [[ -d "$checkout/.git" ]]; then
        assert_sentinel_artifact "$checkout" "$SENTINEL_VERSION" "$SENTINEL_REF"
        assert_sentinel_checkout_clean "$checkout"
        printf '%s\n' "$checkout"
        return 0
    fi

    if [[ -e "$checkout" || -L "$checkout" ]]; then
        echo "  FAIL: Sentinel cache path exists but is not a git checkout: $checkout" >&2
        exit 1
    fi

    if ! command -v git >/dev/null 2>&1; then
        echo "  FAIL: git is required to fetch Sentinel. Re-run without --skip-deps, or install git first." >&2
        exit 1
    fi

    mkdir -p "$SENTINEL_CACHE_ROOT"
    tmp="$(mktemp -d "$SENTINEL_CACHE_ROOT/.tmp.XXXXXX")"
    SENTINEL_STAGE_DIR="$tmp"

    sentinel_git clone "$SENTINEL_REPO_URL" "$tmp"
    sentinel_git -C "$tmp" checkout --detach "$SENTINEL_REF"

    assert_sentinel_artifact "$tmp" "$SENTINEL_VERSION" "$SENTINEL_REF"
    assert_sentinel_checkout_clean "$tmp"

    mv "$tmp" "$checkout"
    tmp=""
    SENTINEL_STAGE_DIR=""
    printf '%s\n' "$checkout"
)

run_sentinel_agent_policy() {
    local checkout installer

    if [[ "$SKIP_AGENTS" -eq 1 ]]; then
        echo
        echo "skipped: Phase 6/6 (agent policy) via --skip-agents"
        return 0
    fi

    if ! should_apply_agent_policy; then
        echo
        echo "skipped: Phase 6/6 (agent policy)"
        return 0
    fi

    phase "Phase 6/6: apply global agent policy (Sentinel)"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: clone/fetch Sentinel $SENTINEL_VERSION (@ $SENTINEL_REF)"
        echo "         into $(sentinel_checkout_dir)"
        echo "  would: run Sentinel tools/install --global, then --global --check"
        return 0
    fi

    checkout="$(ensure_sentinel_checkout)"
    installer="$checkout/tools/install"
    if [[ ! -x "$installer" ]]; then
        echo "  FAIL: Sentinel installer missing or not executable: $installer" >&2
        exit 1
    fi

    bash "$installer" --global
    bash "$installer" --global --check
}

unique_backup() {
    local base="$1"
    if [[ ! -e "$base" && ! -L "$base" ]]; then printf '%s' "$base"; return; fi
    local i=1
    while [[ -e "${base}.${i}" || -L "${base}.${i}" ]]; do i=$((i + 1)); done
    printf '%s' "${base}.${i}"
}

realpath_or_self() {
    if command -v realpath >/dev/null 2>&1; then
        realpath "$1" 2>/dev/null || echo "$1"
    elif command -v readlink >/dev/null 2>&1 && readlink -f / >/dev/null 2>&1; then
        readlink -f "$1" 2>/dev/null || echo "$1"
    else
        ( cd "$1" 2>/dev/null && pwd -P ) || echo "$1"
    fi
}

refuse_nvim_self_link_if_needed() {
    local nvim_target="$HOME/.config/nvim" target_real repo_real repo_nvim_real

    [[ -e "$nvim_target" || -L "$nvim_target" ]] || return 0
    # An existing SYMLINK is the normal already-installed case: it points into
    # the repo, so its dereferenced value equals <repo>/nvim, but the target
    # LOCATION is not the repo. chezmoi safely replaces it -- do NOT refuse.
    # Only a REAL (non-symlink) directory AT the target that resolves to the repo
    # root or <repo>/nvim is a genuine self-overlap (the repo lives there).
    [[ -L "$nvim_target" ]] && return 0

    target_real="$(realpath_or_self "$nvim_target")"
    repo_real="$(realpath_or_self "$SCRIPT_DIR")"
    repo_nvim_real="$(realpath_or_self "$SCRIPT_DIR/nvim")"

    if [[ "$target_real" != "$repo_real" && "$target_real" != "$repo_nvim_real" ]]; then
        return 0
    fi

    cat >&2 <<EOF
setup.sh: the repo is currently at $SCRIPT_DIR, which overlaps the path
that setup.sh would configure as the Neovim target.

Move the repo elsewhere first (e.g. ~/dotfiles), then re-run setup.sh:

    mv "$SCRIPT_DIR" "$HOME/dotfiles"
    cd "$HOME/dotfiles"
    ./setup.sh
EOF
    exit 1
}

run_chezmoi() {
    chezmoi "${CHEZMOI_BASE_ARGS[@]}" \
        ${CHEZMOI_CONFIG_ARGS[@]+"${CHEZMOI_CONFIG_ARGS[@]}"} \
        ${CHEZMOI_DATA_ARGS[@]+"${CHEZMOI_DATA_ARGS[@]}"} \
        "$@"
}

render_chezmoi_config_template() {
    local output="$1"
    chezmoi "${CHEZMOI_BASE_ARGS[@]}" \
        ${CHEZMOI_DATA_ARGS[@]+"${CHEZMOI_DATA_ARGS[@]}"} \
        execute-template --init < "$CHEZMOI_SOURCE/.chezmoi.toml.tmpl" > "$output"
}

target_content_matches_chezmoi() {
    local target="$1" expected_file expected_ref

    expected_file="$(mktemp)"
    trap 'rm -f "$expected_file"; trap - RETURN' RETURN
    if ! run_chezmoi cat "$target" > "$expected_file" 2>/dev/null; then
        rm -f "$expected_file"
        return 1
    fi

    expected_ref="$(cat "$expected_file")"
    if [[ -n "$expected_ref" && -e "$expected_ref" ]]; then
        if [[ -d "$target" && -d "$expected_ref" ]]; then
            if diff -qr "$target" "$expected_ref" >/dev/null 2>&1; then
                rm -f "$expected_file"
                return 0
            fi
        elif [[ -f "$expected_ref" && ( -f "$target" || -L "$target" ) ]]; then
            if cmp -s "$target" "$expected_ref"; then
                rm -f "$expected_file"
                return 0
            fi
        fi
    elif [[ -f "$target" || -L "$target" ]]; then
        if cmp -s "$target" "$expected_file"; then
            rm -f "$expected_file"
            return 0
        fi
    fi

    rm -f "$expected_file"
    return 1
}

target_already_matches() {
    local target="$1"
    run_chezmoi verify "$target" >/dev/null 2>&1 ||
        target_content_matches_chezmoi "$target"
}

backup_preexisting_managed_targets() {
    local managed_output target backup

    managed_output="$(run_chezmoi managed --path-style absolute --include files,symlinks)"
    if [[ -z "$managed_output" ]]; then
        echo "  backup    no managed file/symlink targets found"
        return 0
    fi

    while IFS= read -r target; do
        [[ -n "$target" ]] || continue
        if [[ ! -e "$target" && ! -L "$target" ]]; then
            continue
        fi

        if target_already_matches "$target"; then
            echo "  ok        $target"
            continue
        fi

        backup="$(unique_backup "${target}.bak.${TIMESTAMP}")"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  backup    $target -> $backup; then chezmoi apply"
        else
            mv "$target" "$backup"
            echo "  backed up $target -> $backup"
        fi
    done <<<"$managed_output"
}

ensure_managed_target_parents() {
    local managed_output target parent
    [[ "$DRY_RUN" -eq 0 ]] || return 0
    managed_output="$(run_chezmoi managed --path-style absolute --include files,symlinks)"
    while IFS= read -r target; do
        [[ -n "$target" ]] || continue
        parent="$(dirname "$target")"
        if [[ -e "$parent" && ! -d "$parent" ]]; then
            echo "  FAIL: managed target parent is not a directory: $parent" >&2
            return 1
        fi
        mkdir -p "$parent"
    done <<<"$managed_output"
}

run_or_fail() {
    local label="$1"; shift
    if "$@"; then return 0; fi
    local rc=$?
    if [[ "$BEST_EFFORT" -eq 1 ]]; then
        echo "  WARN: $label exited $rc (continuing because --best-effort is set)"
        return 0
    fi
    echo "  FAIL: $label exited $rc"
    echo "        To continue past plugin/LSP failures, re-run with --best-effort."
    exit "$rc"
}

cleanup_chezmoi_dry_config() {
    rm -f "${CHEZMOI_DRY_CONFIG:-}"
    CHEZMOI_CONFIG_ARGS=()
}

run_update_mode() {
    local update_flags=(--update)
    if [[ "${#DEPS_FLAGS[@]}" -gt 0 ]]; then
        update_flags=("${DEPS_FLAGS[@]}" --update)
    fi
    if [[ "$SKIP_DEPS" -eq 0 && "$SKIP_NATIVE_DEPS" -eq 0 ]]; then
        phase "Update 1/2: refresh proven dependency tools and artifacts"
        bash "$SCRIPT_DIR/install-deps.sh" "${update_flags[@]}"
    else
        echo
        echo "skipped: update dependency phase via --skip-deps/--skip-native-deps"
    fi

    refresh_runtime_path

    if [[ "$SKIP_NVIM" -eq 0 ]]; then
        phase "Update 2/2: update Mason LSP servers + formatters"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: nvim --headless +lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
        elif command -v nvim >/dev/null 2>&1; then
            run_or_fail "Mason update" nvim --headless \
                "+lua require('util.mason_tools').run_checked('MasonToolsUpdateSync')"
        else
            echo "  skipped   Mason update: nvim not on PATH"
        fi
    else
        echo
        echo "skipped: Mason update via --skip-nvim"
    fi

    echo
    echo "The checked-out release, pinned plugins, configs, Nix layer, and missing tools were reconciled before this scoped refresh."
    echo
    echo "================================================================"
    echo "==  setup.sh: update done"
    echo "================================================================"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "(dry run -- nothing was actually installed or changed)"
    fi
}

# Read the committed flake.lock with Nix's JSON parser so first-run bootstrap
# commands use the same pinned inputs as the repo flake.
flake_lock_github_metadata() {
    local node="$1" owner="$2" repo="$3" expr metadata
    expr='let
  lock = builtins.fromJSON (builtins.readFile ./flake.lock);
  node = lock.nodes."'"$node"'".locked or {};
  owner = node.owner or "";
  repo = node.repo or "";
  rev = node.rev or "";
  narHash = node.narHash or "";
in
  if owner == "'"$owner"'" && repo == "'"$repo"'" && rev != "" && narHash != "" then rev + "\n" + narHash
  else throw "flake.lock node '"$node"' is not github:'"$owner"'/'"$repo"' with a locked rev and narHash"'
    if ! metadata="$(nix eval --impure --raw --expr "$expr" 2>/dev/null)"; then
        echo "  FAIL: could not read locked $owner/$repo revision and narHash from flake.lock" >&2
        return 1
    fi
    printf '%s\n' "$metadata"
}

flake_lock_github_rev() {
    flake_lock_github_metadata "$@" | sed -n '1p'
}

flake_lock_github_nar_hash() {
    flake_lock_github_metadata "$@" | sed -n '2p'
}

nix_flake_ref_query_encode() {
    local value="$1"
    value="${value//%/%25}"
    value="${value//+/%2B}"
    value="${value//\//%2F}"
    value="${value//=/%3D}"
    value="${value//#/%23}"
    value="${value//&/%26}"
    printf '%s\n' "$value"
}

pinned_nix_darwin_run_ref() {
    local rev nar_hash
    rev="$(flake_lock_github_rev "nix-darwin" "nix-darwin" "nix-darwin")" || return 1
    nar_hash="$(flake_lock_github_nar_hash "nix-darwin" "nix-darwin" "nix-darwin")" || return 1
    printf 'github:nix-darwin/nix-darwin/%s?narHash=%s#darwin-rebuild\n' \
        "$rev" "$(nix_flake_ref_query_encode "$nar_hash")"
}

pinned_home_manager_run_ref() {
    local rev nar_hash
    rev="$(flake_lock_github_rev "home-manager" "nix-community" "home-manager")" || return 1
    nar_hash="$(flake_lock_github_nar_hash "home-manager" "nix-community" "home-manager")" || return 1
    printf 'github:nix-community/home-manager/%s?narHash=%s#home-manager\n' \
        "$rev" "$(nix_flake_ref_query_encode "$nar_hash")"
}

sudo_nix_darwin_activation() {
    local target_user="${DOTFILES_TARGET_USER:-}" target_home="${DOTFILES_TARGET_HOME:-}"
    if [[ -z "$target_user" || -z "$target_home" ]]; then
        echo "  FAIL: validated target identity is missing before nix-darwin activation." >&2
        return 1
    fi
    sudo -H env DOTFILES_TARGET_USER="$target_user" DOTFILES_TARGET_HOME="$target_home" "$@"
}

nix_darwin_sudo_preview() {
    local target_user="${DOTFILES_TARGET_USER:-<resolved-user>}"
    local target_home="${DOTFILES_TARGET_HOME:-<resolved-home>}"
    printf 'sudo -H env DOTFILES_TARGET_USER=%q DOTFILES_TARGET_HOME=%q ' "$target_user" "$target_home"
    printf '%s' "$1"
}

nix_darwin_rebuild_command() {
    local candidate
    if candidate="$(command -v darwin-rebuild 2>/dev/null)" && [[ -n "$candidate" ]]; then
        printf '%s\n' "$candidate"
        return 0
    fi

    # Source-only tests must not inherit an installed host generation. Their
    # explicit executable seam models the stale-PATH post-activation state.
    if [[ "${DOTFILES_SETUP_SOURCE_ONLY_ACTIVE:-}" == "1" ]]; then
        candidate="${DOTFILES_TEST_NIX_DARWIN_REBUILD:-}"
        [[ -z "$candidate" ]] && return 1
        if [[ "$candidate" != /* || ! -x "$candidate" ]]; then
            echo "  FAIL: nix-darwin rebuild test path must be an absolute executable: $candidate" >&2
            return 1
        fi
        printf '%s\n' "$candidate"
        return 0
    fi

    # A shell opened before the first successful activation does not yet have
    # /run/current-system/sw/bin on PATH. Resolve the installed generation
    # directly so an ordinary retry cannot be misclassified as first bootstrap.
    for candidate in \
        /run/current-system/sw/bin/darwin-rebuild \
        /nix/var/nix/profiles/system/sw/bin/darwin-rebuild; do
        if [[ -x "$candidate" ]]; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done
    return 1
}

nix_darwin_etc_dir() {
    local etc_dir="/etc"
    # The override is a source-only oracle seam. A production environment
    # cannot redirect privileged shell-file migration away from /etc.
    if [[ "${DOTFILES_SETUP_SOURCE_ONLY_ACTIVE:-}" == "1" ]]; then
        if [[ -n "${DOTFILES_TEST_NIX_DARWIN_ETC_DIR:-}" ]]; then
            etc_dir="$DOTFILES_TEST_NIX_DARWIN_ETC_DIR"
        else
            # Source-only probes must never inspect or move the host's real
            # privileged files merely because they exercise activation logic.
            etc_dir="/nonexistent/dotfiles-source-only-etc"
        fi
    fi
    if [[ "$etc_dir" != /* ]]; then
        echo "  FAIL: nix-darwin etc directory is not absolute: $etc_dir" >&2
        return 1
    fi
    printf '%s\n' "${etc_dir%/}"
}

nix_homebrew_taps_dir() {
    local taps_dir="/opt/homebrew/Library/Taps"
    if [[ "${DOTFILES_SETUP_SOURCE_ONLY_ACTIVE:-}" == "1" ]]; then
        if [[ -n "${DOTFILES_TEST_NIX_HOMEBREW_TAPS_DIR:-}" ]]; then
            taps_dir="$DOTFILES_TEST_NIX_HOMEBREW_TAPS_DIR"
        else
            taps_dir="/nonexistent/dotfiles-source-only-taps"
        fi
    fi
    [[ "$taps_dir" == /* ]] || {
        echo "  FAIL: Homebrew taps directory is not absolute: $taps_dir" >&2
        return 1
    }
    printf '%s\n' "${taps_dir%/}"
}

nix_darwin_shell_rc_is_managed() {
    local etc_dir="$1" name="$2" source expected
    source="$etc_dir/$name"
    expected="$etc_dir/static/$name"
    [[ -L "$source" ]] || return 1
    [[ "$(readlink "$source" 2>/dev/null || true)" == "$expected" ]]
}

legacy_nix_homebrew_tap_paths() {
    printf '%s\n' \
        "homebrew/homebrew-core" \
        "homebrew/homebrew-cask" \
        "nikitabobko/homebrew-tap"
}

legacy_nix_homebrew_in_tree_artifacts() {
    local taps_dir="$1" rel tap artifact
    while IFS= read -r rel; do
        tap="$taps_dir/$rel"
        for artifact in \
            "$tap".dotfiles-pre-user-taps-* \
            "$tap".dotfiles-failed-*; do
            [[ -e "$artifact" || -L "$artifact" ]] || continue
            printf '%s\n' "$artifact"
        done
    done < <(legacy_nix_homebrew_tap_paths)
}

relocate_legacy_nix_homebrew_in_tree_artifacts() {
    local taps_dir="$1" artifact rel recovery_root="" destination
    while IFS= read -r artifact; do
        if [[ -z "$recovery_root" ]]; then
            recovery_root="$(unique_backup "${taps_dir}.dotfiles-recovery-$TIMESTAMP")"
        fi
        rel="${artifact#"$taps_dir"/}"
        destination="$recovery_root/$rel"
        if ! sudo mkdir -p "$(dirname "$destination")"; then
            echo "  FAIL: could not create external Homebrew tap recovery directory." >&2
            return 1
        fi
        if ! sudo mv "$artifact" "$destination"; then
            echo "  FAIL: could not move Homebrew-scanned recovery artifact outside $taps_dir: $artifact" >&2
            return 1
        fi
        echo "  recover   Homebrew tap artifact moved outside scanned Taps: $destination"
    done < <(legacy_nix_homebrew_in_tree_artifacts "$taps_dir")
}

legacy_nix_homebrew_transaction_root_is_safe() {
    local taps_dir="$1" root="$2"
    [[ -n "$root" ]] || return 1
    case "$root" in
        "$taps_dir".dotfiles-transaction-*) return 0 ;;
        *) return 1 ;;
    esac
}

legacy_nix_homebrew_tap_is_generated() {
    local tap="$1" owner
    [[ -d "$tap" && ! -L "$tap" && ! -e "$tap/.git" ]] || return 1
    if [[ "${DOTFILES_SETUP_SOURCE_ONLY_ACTIVE:-}" == "1" ]] &&
        [[ "${DOTFILES_TEST_NIX_HOMEBREW_LEGACY_TAPS:-}" == "1" ]]; then
        return 0
    fi
    owner="$(stat -f '%u' "$tap" 2>/dev/null || true)"
    [[ "$owner" == "0" ]]
}

preview_legacy_nix_homebrew_tap_migration() {
    local taps_dir rel tap artifact
    taps_dir="$(nix_homebrew_taps_dir)" || return 1
    while IFS= read -r artifact; do
        echo "  would: move Homebrew-scanned recovery artifact outside Taps: $artifact"
    done < <(legacy_nix_homebrew_in_tree_artifacts "$taps_dir")
    while IFS= read -r rel; do
        tap="$taps_dir/$rel"
        if legacy_nix_homebrew_tap_is_generated "$tap"; then
            echo "  would: migrate legacy root-owned Homebrew tap $rel back to target-user ownership"
        fi
    done < <(legacy_nix_homebrew_tap_paths)
}

prepare_legacy_nix_homebrew_tap_migration() {
    local taps_dir rel tap backup i found=0
    taps_dir="$(nix_homebrew_taps_dir)" || return 1
    NIX_HOMEBREW_LEGACY_TAPS=()
    NIX_HOMEBREW_LEGACY_BACKUPS=()
    NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT=""

    # Older setup versions stored transaction paths under Library/Taps, where
    # Homebrew interpreted them as real taps. Move only those exact setup-owned
    # artifact shapes outside the scanned directory before asking Brew to run.
    relocate_legacy_nix_homebrew_in_tree_artifacts "$taps_dir" || return 1

    # Preflight every exact legacy path before moving any. An ordinary user tap
    # is a Git checkout and never matches this root-owned, non-Git snapshot
    # shape, so unrelated taps remain outside the transaction. Transaction
    # storage is a sibling of Taps because every child directory can be parsed
    # as a tap by Homebrew.
    while IFS= read -r rel; do
        tap="$taps_dir/$rel"
        legacy_nix_homebrew_tap_is_generated "$tap" || continue
        found=1
    done < <(legacy_nix_homebrew_tap_paths)
    [[ "$found" -eq 1 ]] || return 0

    NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT="$(
        unique_backup "${taps_dir}.dotfiles-transaction-$TIMESTAMP"
    )"
    if ! sudo mkdir -p "$NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT/original"; then
        echo "  FAIL: could not create external Homebrew tap transaction directory." >&2
        NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT=""
        return 1
    fi

    while IFS= read -r rel; do
        tap="$taps_dir/$rel"
        legacy_nix_homebrew_tap_is_generated "$tap" || continue
        backup="$NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT/original/$rel"
        if ! sudo mkdir -p "$(dirname "$backup")"; then
            echo "  FAIL: could not prepare external backup path for $tap." >&2
            rollback_legacy_nix_homebrew_tap_migration || true
            return 1
        fi
        i="${#NIX_HOMEBREW_LEGACY_TAPS[@]}"
        NIX_HOMEBREW_LEGACY_TAPS[i]="$tap"
        NIX_HOMEBREW_LEGACY_BACKUPS[i]="$backup"
        if ! sudo mv "$tap" "$backup"; then
            echo "  FAIL: could not stage legacy root-owned Homebrew tap $tap." >&2
            rollback_legacy_nix_homebrew_tap_migration || true
            return 1
        fi
        echo "  migrate   legacy root-owned Homebrew tap $rel -> target-user managed"
    done < <(legacy_nix_homebrew_tap_paths)
}

rollback_legacy_nix_homebrew_tap_migration() {
    local taps_dir root rel i tap backup failed rc=0 replacement_preserved=0
    taps_dir="$(nix_homebrew_taps_dir)" || return 1
    root="$NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT"
    if [[ -n "$root" ]] && ! legacy_nix_homebrew_transaction_root_is_safe "$taps_dir" "$root"; then
        echo "  FAIL: refusing unsafe legacy tap rollback root: $root" >&2
        return 1
    fi
    i=$((${#NIX_HOMEBREW_LEGACY_TAPS[@]} - 1))
    while [[ "$i" -ge 0 ]]; do
        tap="${NIX_HOMEBREW_LEGACY_TAPS[$i]}"
        backup="${NIX_HOMEBREW_LEGACY_BACKUPS[$i]}"
        if [[ ! -e "$backup" && ! -L "$backup" ]]; then
            if [[ ! -e "$tap" && ! -L "$tap" ]]; then
                echo "  FAIL: legacy Homebrew tap rollback is missing both $tap and $backup." >&2
                rc=1
            fi
            i=$((i - 1))
            continue
        fi
        if [[ -e "$tap" || -L "$tap" ]]; then
            rel="${tap#"$taps_dir"/}"
            failed="$(unique_backup "$root/failed/$rel")"
            if ! sudo mkdir -p "$(dirname "$failed")"; then
                echo "  FAIL: could not create external failed-tap quarantine for $tap." >&2
                rc=1
                i=$((i - 1))
                continue
            fi
            if ! sudo mv "$tap" "$failed"; then
                echo "  FAIL: could not quarantine replacement Homebrew tap $tap." >&2
                rc=1
                i=$((i - 1))
                continue
            fi
            replacement_preserved=1
            echo "  note      failed replacement Homebrew tap preserved at $failed"
        fi
        if ! sudo mv "$backup" "$tap"; then
            echo "  FAIL: could not restore legacy Homebrew tap $backup -> $tap." >&2
            rc=1
        else
            echo "  restored  legacy Homebrew tap $tap"
        fi
        i=$((i - 1))
    done
    if [[ "$rc" -eq 0 && -n "$root" ]]; then
        if [[ "$replacement_preserved" -eq 1 ]]; then
            if sudo rm -rf "$root/original"; then
                echo "  recovery  failed Homebrew tap output retained outside scanned Taps at $root"
            else
                echo "  FAIL: restored taps, but could not prune original snapshots at $root/original." >&2
                rc=1
            fi
        elif ! sudo rm -rf "$root"; then
            echo "  FAIL: restored taps, but could not remove empty transaction root $root." >&2
            rc=1
        fi
    fi
    if [[ "$rc" -eq 0 ]]; then
        NIX_HOMEBREW_LEGACY_TAPS=()
        NIX_HOMEBREW_LEGACY_BACKUPS=()
        NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT=""
    fi
    return "$rc"
}

complete_legacy_nix_homebrew_tap_migration() {
    local taps_dir root backup i
    taps_dir="$(nix_homebrew_taps_dir)" || return 1
    root="$NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT"
    [[ -z "$root" ]] && return 0
    if ! legacy_nix_homebrew_transaction_root_is_safe "$taps_dir" "$root"; then
        echo "  FAIL: refusing unsafe legacy tap transaction cleanup: $root" >&2
        return 1
    fi
    for ((i = 0; i < ${#NIX_HOMEBREW_LEGACY_TAPS[@]}; i++)); do
        backup="${NIX_HOMEBREW_LEGACY_BACKUPS[$i]}"
        case "$backup" in
            "$root"/original/*) ;;
            *)
                echo "  FAIL: refusing unsafe legacy tap backup cleanup: $backup" >&2
                return 1
                ;;
        esac
    done
    if sudo rm -rf "$root"; then
        echo "  clean     removed migrated legacy tap snapshots outside scanned Taps: $root"
    else
        echo "  note      legacy tap snapshots retained outside scanned Taps for inspection: $root" >&2
    fi
    NIX_HOMEBREW_LEGACY_TAPS=()
    NIX_HOMEBREW_LEGACY_BACKUPS=()
    NIX_HOMEBREW_LEGACY_TRANSACTION_ROOT=""
}

preview_nix_darwin_shell_rc_migration() {
    local etc_dir source backup name
    etc_dir="$(nix_darwin_etc_dir)" || return 1
    for name in bashrc zshrc; do
        source="$etc_dir/$name"
        backup="$source.before-nix-darwin"
        [[ -e "$source" || -L "$source" ]] || continue
        if nix_darwin_shell_rc_is_managed "$etc_dir" "$name"; then
            echo "  would: retain nix-darwin-managed $source and its existing recovery backup"
        elif [[ -e "$backup" || -L "$backup" ]]; then
            echo "  would fail: cannot preserve $source because $backup already exists."
        else
            echo "  would: sudo mv $source $backup before first nix-darwin activation"
        fi
    done
}

prepare_nix_darwin_shell_rc_migration() {
    local etc_dir source backup name i
    etc_dir="$(nix_darwin_etc_dir)" || return 1
    NIX_DARWIN_RC_SOURCES=()
    NIX_DARWIN_RC_BACKUPS=()

    # Preflight both paths before moving either one. nix-darwin deliberately
    # refuses unrecognized /etc files and documents the exact
    # .before-nix-darwin backup shape; never overwrite an earlier backup.
    for name in bashrc zshrc; do
        source="$etc_dir/$name"
        backup="$source.before-nix-darwin"
        [[ -e "$source" || -L "$source" ]] || continue
        nix_darwin_shell_rc_is_managed "$etc_dir" "$name" && continue
        if [[ -e "$backup" || -L "$backup" ]]; then
            echo "  FAIL: first nix-darwin bootstrap must preserve $source, but backup $backup already exists." >&2
            echo "        Compare both files and resolve the collision before retrying; neither was changed." >&2
            return 1
        fi
    done

    for name in bashrc zshrc; do
        source="$etc_dir/$name"
        backup="$source.before-nix-darwin"
        [[ -e "$source" || -L "$source" ]] || continue
        nix_darwin_shell_rc_is_managed "$etc_dir" "$name" && continue
        i="${#NIX_DARWIN_RC_SOURCES[@]}"
        NIX_DARWIN_RC_SOURCES[i]="$source"
        NIX_DARWIN_RC_BACKUPS[i]="$backup"
        if ! sudo mv "$source" "$backup"; then
            echo "  FAIL: could not preserve $source at $backup before nix-darwin bootstrap." >&2
            if ! rollback_nix_darwin_shell_rc_migration; then
                echo "        Partial shell-file rollback failed; follow the recovery instructions above." >&2
            fi
            return 1
        fi
        echo "  backup    pre-nix-darwin shell file $source -> $backup"
    done
}

rollback_nix_darwin_shell_rc_migration() {
    local i source backup failed_base failed suffix rc=0
    i=$((${#NIX_DARWIN_RC_SOURCES[@]} - 1))
    while [[ "$i" -ge 0 ]]; do
        source="${NIX_DARWIN_RC_SOURCES[$i]}"
        backup="${NIX_DARWIN_RC_BACKUPS[$i]}"
        if [[ ! -e "$backup" && ! -L "$backup" ]]; then
            if [[ ! -e "$source" && ! -L "$source" ]]; then
                echo "  FAIL: shell-file rollback is missing both $source and $backup." >&2
                echo "        Restore $source from a known-good backup before retrying." >&2
                rc=1
            fi
            i=$((i - 1))
            continue
        fi
        if [[ -e "$source" || -L "$source" ]]; then
            failed_base="$source.dotfiles-failed-${DOTFILES_TEST_TIMESTAMP:-$(date +%Y%m%d%H%M%S)}"
            failed="$failed_base"
            suffix=0
            while [[ -e "$failed" || -L "$failed" ]]; do
                suffix=$((suffix + 1))
                failed="$failed_base.$suffix"
            done
            if ! sudo mv "$source" "$failed"; then
                echo "  FAIL: could not quarantine failed nix-darwin output at $source." >&2
                echo "        Original content remains safe at $backup; move the replacement aside and restore it manually." >&2
                rc=1
                i=$((i - 1))
                continue
            fi
            echo "  note      failed nix-darwin shell output preserved at $failed"
        fi
        if ! sudo mv "$backup" "$source"; then
            echo "  FAIL: could not restore pre-nix-darwin shell file $backup -> $source." >&2
            echo "        Restore it manually with: sudo mv '$backup' '$source'" >&2
            rc=1
        else
            echo "  restored  pre-nix-darwin shell file $source"
        fi
        i=$((i - 1))
    done
    if [[ "$rc" -eq 0 ]]; then
        NIX_DARWIN_RC_SOURCES=()
        NIX_DARWIN_RC_BACKUPS=()
    fi
    return "$rc"
}

complete_nix_darwin_shell_rc_migration() {
    local backup
    if [[ "${#NIX_DARWIN_RC_BACKUPS[@]}" -gt 0 ]]; then
        for backup in "${NIX_DARWIN_RC_BACKUPS[@]}"; do
            echo "  backup    pre-nix-darwin shell file retained at $backup"
        done
    fi
    NIX_DARWIN_RC_SOURCES=()
    NIX_DARWIN_RC_BACKUPS=()
}

nix_darwin_activation_interrupted() {
    local signal="$1" rc=130 rollback_failed=0
    [[ "$signal" == "TERM" ]] && rc=143
    trap - INT TERM
    echo "  FAIL: nix-darwin activation interrupted by $signal; restoring pre-activation state." >&2
    rollback_nix_darwin_shell_rc_migration || rollback_failed=1
    rollback_legacy_nix_homebrew_tap_migration || rollback_failed=1
    if [[ "$rollback_failed" -ne 0 ]]; then
        echo "        Automatic rollback was incomplete; follow the recovery instructions above before retrying." >&2
    fi
    exit "$rc"
}

# Required POSIX package layer: apply Nix (nix-darwin) on macOS before native
# dependency provisioning. It activates declarative Homebrew (WezTerm/AeroSpace
# casks, Herdr brew) and the nix-owned CLI package set. chezmoi still owns every
# dotfile (invariant 22). setup resolves one authoritative target user/home and
# passes both through sudo explicitly for impure flake evaluation.
run_nix_darwin_switch() {
    local explicit=0 arch raw_arch config_name flake_ref bootstrap_ref rebuild_command=""
    [[ "$NIX_DARWIN" -eq 1 ]] && explicit=1
    if [[ "$(uname -s)" != "Darwin" ]]; then
        [[ "$explicit" -eq 1 ]] || return 0
        echo
        echo "skipped: --nix-darwin is macOS-only (nix-darwin has no Linux/Windows target)"
        return 0
    fi
    if [[ "$SKIP_DEPS" -eq 1 ]]; then
        echo
        echo "skipped: Nix package layer via --skip-deps"
        return 0
    fi
    phase "Required POSIX package layer: apply nix-darwin packages (macOS)"
    raw_arch="$(uname -m)"
    case "$raw_arch" in
        arm64|aarch64) arch="aarch64" ;;
        *)
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would fail: macOS setup requires Apple Silicon (arm64); detected $raw_arch."
                return 0
            fi
            echo "  FAIL: macOS setup requires Apple Silicon (arm64); detected $raw_arch." >&2
            return 1
            ;;
    esac
    config_name="dotfiles-aarch64"
    if ! command -v nix >/dev/null 2>&1; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            if [[ "$NIX_PREREQUISITE_DRY_RUN_PLANNED" -eq 1 ]]; then
                echo "  would: activate locked nix-darwin after setup installs the verified Nix prerequisite"
                return 0
            fi
            echo "  would fail: Nix is required for macOS setup. Install Nix first"
            echo "              (for example, the notarized Determinate installer)."
            return 0
        fi
        echo "  FAIL: Nix is required for macOS setup. Install Nix first (for example,"
        echo "        the notarized Determinate installer), then re-run ./setup.sh."
        echo "        This repo deliberately does not add a pipe-to-shell Nix installer."
        return 1
    fi
    flake_ref="$SCRIPT_DIR#$config_name"
    bootstrap_ref="$(pinned_nix_darwin_run_ref)" || return 1
    rebuild_command="$(nix_darwin_rebuild_command)" || rebuild_command=""
    echo "  Runs '$(nix_darwin_sudo_preview "${rebuild_command:-darwin-rebuild}") switch --flake $flake_ref --impure':"
    echo "  declarative Homebrew (WezTerm/AeroSpace casks, Herdr brew) + nix CLI set."
    echo "  It uses sudo for nix-darwin's upstream activation shape while passing"
    echo "  the setup-validated target user/home explicitly through the boundary."
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: $(nix_darwin_sudo_preview "${rebuild_command:-darwin-rebuild}") switch --flake $flake_ref --impure"
        if [[ -z "$rebuild_command" ]]; then
            echo "         (first-time bootstrap: $(nix_darwin_sudo_preview "nix") run $bootstrap_ref -- switch --flake $flake_ref --impure)"
            preview_nix_darwin_shell_rc_migration
        fi
        preview_legacy_nix_homebrew_tap_migration
        return 0
    fi
    if [[ "$ALL" -eq 0 ]]; then
        if ! ask_yes_no_default_yes "Apply the required nix-darwin package layer now?"; then
            echo "  FAIL: macOS setup requires nix-darwin for package provisioning."
            echo "        Re-run with --skip-deps only on a host that is already provisioned."
            return 1
        fi
    fi
    trap 'nix_darwin_activation_interrupted INT' INT
    trap 'nix_darwin_activation_interrupted TERM' TERM
    if ! prepare_legacy_nix_homebrew_tap_migration; then
        trap - INT TERM
        return 1
    fi
    if [[ -n "$rebuild_command" ]]; then
        if ! sudo_nix_darwin_activation "$rebuild_command" switch --flake "$flake_ref" --impure; then
            trap - INT TERM
            echo "  FAIL: nix-darwin activation failed; setup did not apply the requested Nix package layer." >&2
            if ! rollback_nix_darwin_shell_rc_migration; then
                echo "        Shell-file rollback also failed; follow the recovery instructions above before retrying." >&2
            fi
            rollback_legacy_nix_homebrew_tap_migration || true
            echo "        Re-run './setup.sh' after fixing the activation error." >&2
            return 1
        fi
    else
        echo "  bootstrapping nix-darwin from flake.lock ($bootstrap_ref)..."
        if ! prepare_nix_darwin_shell_rc_migration; then
            trap - INT TERM
            rollback_legacy_nix_homebrew_tap_migration || true
            return 1
        fi
        if ! sudo_nix_darwin_activation nix run "$bootstrap_ref" -- switch --flake "$flake_ref" --impure; then
            trap - INT TERM
            echo "  FAIL: nix-darwin bootstrap activation failed; setup did not apply the requested Nix package layer." >&2
            if ! rollback_nix_darwin_shell_rc_migration; then
                echo "        Shell-file rollback also failed; follow the recovery instructions above before retrying." >&2
            fi
            rollback_legacy_nix_homebrew_tap_migration || true
            echo "        Re-run './setup.sh' after fixing the activation error." >&2
            return 1
        fi
    fi
    trap - INT TERM
    complete_legacy_nix_homebrew_tap_migration
    complete_nix_darwin_shell_rc_migration
    echo "  ok        nix-darwin package layer applied"
}

# Required POSIX package layer: apply Home Manager on Linux/WSL before native
# dependency provisioning. It installs the nix-owned CLI package set into the
# user profile (no root). On WSL it writes ONLY to the Linux ~/.nix-profile, so
# the split-host model is preserved. --impure lets the flake resolve $USER.
run_home_manager_switch() {
    local explicit=0
    [[ "$HOME_MANAGER" -eq 1 ]] && explicit=1
    if [[ "$(uname -s)" != "Linux" ]]; then
        [[ "$explicit" -eq 1 ]] || return 0
        echo
        echo "skipped: --home-manager is Linux/WSL-only (macOS uses --nix-darwin)"
        return 0
    fi
    if [[ "$SKIP_DEPS" -eq 1 ]]; then
        echo
        echo "skipped: Nix package layer via --skip-deps"
        return 0
    fi
    phase "Required POSIX package layer: apply Home Manager packages (Linux/WSL)"
    if ! command -v nix >/dev/null 2>&1; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            if [[ "$NIX_PREREQUISITE_DRY_RUN_PLANNED" -eq 1 ]]; then
                echo "  would: activate locked Home Manager after setup installs the verified Nix prerequisite"
                return 0
            fi
            echo "  would fail: Nix is required for Linux/WSL setup. Install Nix first."
            return 0
        fi
        echo "  FAIL: Nix is required for Linux/WSL setup. Install Nix first, then"
        echo "        re-run ./setup.sh. This repo deliberately does not add a"
        echo "        pipe-to-shell Nix installer."
        return 1
    fi
    local arch config_name flake_ref bootstrap_ref
    arch="$(uname -m)"
    case "$arch" in
        x86_64 | amd64) config_name="x86_64-linux" ;;
        aarch64 | arm64) config_name="aarch64-linux" ;;
        *)
            echo "  FAIL: no supported Home Manager config for Linux arch $arch"
            return 1
            ;;
    esac
    flake_ref="$SCRIPT_DIR#$config_name"
    bootstrap_ref="$(pinned_home_manager_run_ref)" || return 1
    echo "  Runs 'home-manager switch --flake $flake_ref --impure' (no root;"
    echo "  WSL writes only to the Linux ~/.nix-profile, never /mnt/c)."
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: home-manager switch --flake $flake_ref --impure"
        echo "         (first-time bootstrap: nix run $bootstrap_ref -- switch --flake $flake_ref --impure)"
        return 0
    fi
    if [[ "$ALL" -eq 0 ]]; then
        if ! ask_yes_no_default_yes "Apply the required Home Manager package layer now?"; then
            echo "  FAIL: Linux/WSL setup requires Home Manager for package provisioning."
            echo "        Re-run with --skip-deps only on a host that is already provisioned."
            return 1
        fi
    fi
    if command -v home-manager >/dev/null 2>&1; then
        if ! home-manager switch --flake "$flake_ref" --impure; then
            echo "  FAIL: Home Manager activation failed; setup did not apply the requested Nix package layer." >&2
            echo "        Re-run './setup.sh' after fixing the activation error." >&2
            return 1
        fi
    else
        echo "  bootstrapping Home Manager from flake.lock ($bootstrap_ref)..."
        if ! nix run "$bootstrap_ref" -- switch --flake "$flake_ref" --impure; then
            echo "  FAIL: Home Manager bootstrap activation failed; setup did not apply the requested Nix package layer." >&2
            echo "        Re-run './setup.sh' after fixing the activation error." >&2
            return 1
        fi
    fi
    echo "  ok        Home Manager package layer applied"
}

# Test seam: `DOTFILES_SETUP_SOURCE_ONLY=1 source setup.sh` loads the helper
# functions (phase, refresh_runtime_path) without running the install phases, so
# tests can exercise refresh_runtime_path in isolation. Unset in normal runs.
if [[ -z "${DOTFILES_SETUP_SOURCE_ONLY:-}" ]]; then
    resolve_target_identity
    refuse_nvim_self_link_if_needed
    ensure_nix_prerequisite
    maybe_complete_v0_1_upgrade
fi
if [[ -n "${DOTFILES_SETUP_SOURCE_ONLY:-}" ]]; then
    DOTFILES_SETUP_SOURCE_ONLY_ACTIVE=1
    # shellcheck disable=SC2317  # the exit is reached only when executed, not sourced
    return 0 2>/dev/null || exit 0
fi

# ---- Required POSIX package layer --------------------------------------------
run_nix_darwin_switch
run_home_manager_switch
refresh_runtime_path

# ---- Phase 1: dependencies ---------------------------------------------------
if [[ "$SKIP_DEPS" -eq 0 && "$SKIP_NATIVE_DEPS" -eq 0 ]]; then
    phase "Phase 1/6: install dependencies"
    bash "$SCRIPT_DIR/install-deps.sh" ${DEPS_FLAGS[@]+"${DEPS_FLAGS[@]}"}
else
    echo
    echo "skipped: Phase 1 (native/deferred deps) via --skip-deps/--skip-native-deps"
fi
refresh_runtime_path

# ---- Phase 2: apply configs --------------------------------------------------
if [[ "$SKIP_BOOTSTRAP" -eq 0 ]]; then
    phase "Phase 2/6: apply configs with chezmoi"
    if ! command -v chezmoi >/dev/null 2>&1; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            # The dogfood dry-run runs BEFORE Phase 1 installs chezmoi; preview
            # rather than fail (a real run has chezmoi on PATH after Phase 1).
            echo "  would: chezmoi (installed in Phase 1) backs up any divergent"
            echo "         pre-existing config, then 'chezmoi apply' the config layer"
        else
            echo "  FAIL: chezmoi is not on PATH after Phase 1"
            echo "        Re-run without --skip-deps, or install chezmoi first."
            exit 1
        fi
    elif [[ "$DRY_RUN" -eq 1 ]]; then
        CHEZMOI_DRY_CONFIG="$(mktemp)"
        trap 'cleanup_chezmoi_dry_config; trap - EXIT' EXIT
        render_chezmoi_config_template "$CHEZMOI_DRY_CONFIG"
        CHEZMOI_CONFIG_ARGS=(--config "$CHEZMOI_DRY_CONFIG" --config-format toml)
        backup_preexisting_managed_targets
        run_chezmoi --dry-run --verbose apply \
            ${CHEZMOI_APPLY_ARGS[@]+"${CHEZMOI_APPLY_ARGS[@]}"}
        cleanup_chezmoi_dry_config
        trap - EXIT
    else
        if [[ "$SKIP_CONFIG_SCRIPTS" -eq 1 ]]; then
            echo "  retain    existing chezmoi state; frozen release source is read-only"
        else
            chezmoi "${CHEZMOI_BASE_ARGS[@]}" init
        fi
        backup_preexisting_managed_targets
        ensure_managed_target_parents
        run_chezmoi --no-tty --force apply \
            ${CHEZMOI_APPLY_ARGS[@]+"${CHEZMOI_APPLY_ARGS[@]}"}
    fi
else
    echo
    echo "skipped: Phase 2 (config) via --skip-bootstrap/--skip-config"
fi

# ---- Phase 3: restore locked Neovim plugins ----------------------------------
# ---- Phase 4: install Tree-sitter parsers -------------------------------------
# ---- Phase 5: install LSP servers + formatters via Mason ----------------------
#
# By default, Lazy + Tree-sitter + Mason failures are FATAL — they leave the
# user with a bare or weak nvim config. Pass --best-effort to downgrade to
# warnings.
if [[ "$SKIP_NVIM" -eq 0 ]] && [[ "$DRY_RUN" -eq 0 ]]; then
    if command -v nvim >/dev/null 2>&1; then
        phase "Phase 3/6: restore Neovim plugins (lazy.nvim)"
        run_or_fail "Lazy restore" nvim --headless "+Lazy! restore" "+qa"

        phase "Phase 4/6: install Tree-sitter parsers"
        echo "  this compiles nvim-treesitter parsers and can take several minutes."
        run_or_fail "Tree-sitter parser install" env DOTFILES_TREESITTER_SYNC_INSTALL=1 \
            nvim --headless "+lua require('lazy').load({ plugins = { 'nvim-treesitter' } })" "+qa"

        phase "Phase 5/6: install LSP servers + formatters (Mason)"
        echo "  this can take 3-8 minutes on a fresh machine."
        run_or_fail "Mason install" nvim --headless \
            "+lua require('util.mason_tools').run_checked('MasonToolsInstallSync')"
    else
        echo
        echo "skipped: Phase 3-5 (nvim plugins/parsers/tools) -- nvim not on PATH yet."
        echo "         Re-run: ./setup.sh --skip-deps --skip-config"
    fi
elif [[ "$DRY_RUN" -eq 1 ]]; then
    echo
    echo "skipped: Phase 3-5 (nvim plugins/parsers/tools) in --dry-run mode"
else
    echo
    echo "skipped: Phase 3-5 (nvim plugins/parsers/tools) via --skip-nvim"
fi

run_sentinel_agent_policy

if [[ "$UPDATE_MODE" -eq 1 ]]; then
    run_update_mode
fi

# ---- Summary -----------------------------------------------------------------
echo
echo "================================================================"
echo "==  setup.sh: done"
echo "================================================================"
echo
echo "Repo:    $SCRIPT_DIR"
if [[ -n "$COMPLETED_V0_1_RECOVERY" ]]; then
    echo "Upgrade: v0.1.0 migrated and verified; recovery retained at"
    echo "         $COMPLETED_V0_1_RECOVERY"
fi
if [[ "$DRY_RUN" -eq 0 ]]; then
    echo
    echo "IMPORTANT: open a NEW terminal before using the tools. This shell predates"
    echo "           the install, so brew/nvim on PATH and the login-shell switch to"
    echo "           zsh only take effect in newly started shells."
    if command -v brew >/dev/null 2>&1; then
        echo "           (to use them in THIS shell now: eval \"\$($(command -v brew) shellenv)\")"
    fi
fi
echo
echo "Try it (new shell):  nvim   (<Space>fg live grep, :wnf save w/o format)"
echo "Tests:               make test"
echo
if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "(dry run -- nothing was actually installed or changed)"
fi
