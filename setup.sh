#!/usr/bin/env bash
# setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.
#
# Local usage (from a checked-out copy):
#   ./setup.sh                     interactive: dependency prompts, then config + sync
#   ./setup.sh --all               non-interactive: apply Nix package layer, then install everything missing
#   ./setup.sh --update            update proven dependency tools/artifacts + Mason only
#   ./setup.sh --dry-run           preview every step
#   ./setup.sh --skip-deps         already provisioned; skip Nix + native deps
#   ./setup.sh --skip-bootstrap    back-compat alias: skip config apply
#   ./setup.sh --skip-config       already configured; just sync nvim
#   ./setup.sh --skip-nvim         skip nvim plugin/parser/Mason sync
#   ./setup.sh --skip-agents       skip global Polaris agent-policy install
#   ./setup.sh --experimental-wsl-gui
#                                  WSL opt-in: install/link Linux GUI terminal bits
#
# First run (no checkout yet):
#   git clone https://github.com/luisgui1757/dotfiles.git "${DOTFILES_DEST:-$HOME/dotfiles}"
#   cd "${DOTFILES_DEST:-$HOME/dotfiles}"
#   ./setup.sh --all
#
# Set DOTFILES_DEST=/some/other/path in the environment if you want a
# different checkout location.

set -euo pipefail

REPO_URL="https://github.com/luisgui1757/dotfiles.git"
DEFAULT_DEST="$HOME/dotfiles"

ALL=0
DRY_RUN=0
UPDATE_MODE=0
SKIP_DEPS=0
SKIP_BOOTSTRAP=0
SKIP_NVIM=0
SKIP_AGENTS=0
BEST_EFFORT=0
EXPERIMENTAL_WSL_GUI=0
NIX_DARWIN=0
HOME_MANAGER=0
NIX_HOMEBREW_TAPS_PATH=""
NIX_HOMEBREW_TAPS_BACKUP=""
POLARIS_REPO_URL="https://github.com/luisgui1757/polaris.git"
POLARIS_VERSION="0.1.2"
POLARIS_TAG="v0.1.2"
POLARIS_REF="ecca742fa9ed1243a73981955850c1a8ef3e3b04"
POLARIS_CACHE_ROOT="$HOME/.local/share/dotfiles/polaris"
usage() {
    cat <<'EOF'
setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.

Local usage:
  ./setup.sh                     interactive: dependency prompts, then config + sync
  ./setup.sh --all               non-interactive: apply Nix package layer, then install everything missing
  ./setup.sh --update            update proven dependency tools/artifacts + Mason only
  ./setup.sh --dry-run           preview every step
  ./setup.sh --skip-deps         already provisioned; skip Nix + native deps
  ./setup.sh --skip-bootstrap    back-compat alias: skip config apply
  ./setup.sh --skip-config       already configured; just sync nvim
  ./setup.sh --skip-nvim         skip nvim plugin/parser/Mason sync
  ./setup.sh --skip-agents       skip global Polaris agent-policy install
  ./setup.sh --best-effort       continue past plugin/LSP/Mason phase failures
  ./setup.sh --experimental-wsl-gui
                                WSL opt-in: install/link Linux Ghostty + Linux fonts
  ./setup.sh --nix-darwin        compatibility alias; macOS setup applies nix-darwin by default
  ./setup.sh --home-manager      compatibility alias; Linux/WSL setup applies Home Manager by default

First run:
  git clone https://github.com/luisgui1757/dotfiles.git "${DOTFILES_DEST:-$HOME/dotfiles}"
  cd "${DOTFILES_DEST:-$HOME/dotfiles}"
  ./setup.sh --all
EOF
}

for arg in "$@"; do
    case "$arg" in
        --all|-y)         ALL=1 ;;
        --dry-run)        DRY_RUN=1 ;;
        --update)         UPDATE_MODE=1 ;;
        --skip-deps)      SKIP_DEPS=1 ;;
        --skip-bootstrap|--skip-config)
                          SKIP_BOOTSTRAP=1 ;;
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
        echo "    git clone $REPO_URL \"$DEST\""
        echo "    cd \"$DEST\""
        echo "    ./setup.sh --all"
        echo "(dry run -- no clone, no install, no writes performed)"
    fi
    echo "setup.sh must be run from a local clone. Remote/piped clone-and-reinvoke setup is disabled because it would execute mutable default-branch code." >&2
    echo "Clone first, then run setup locally:" >&2
    echo "  git clone $REPO_URL \"$DEST\"" >&2
    echo "  cd \"$DEST\"" >&2
    echo "  ./setup.sh --all" >&2
    exit 1
fi

cd "$SCRIPT_DIR"

# ---- Forward flags to sub-scripts --------------------------------------------
DEPS_FLAGS=()
[[ "$ALL" -eq 1 ]]      && DEPS_FLAGS+=(--all)
[[ "$UPDATE_MODE" -eq 1 ]] && DEPS_FLAGS+=(--update)
[[ "$DRY_RUN" -eq 1 ]]  && DEPS_FLAGS+=(--dry-run)
[[ "$EXPERIMENTAL_WSL_GUI" -eq 1 ]] && DEPS_FLAGS+=(--experimental-wsl-gui)

CHEZMOI_SOURCE="$SCRIPT_DIR/home"
CHEZMOI_BASE_ARGS=(--source "$CHEZMOI_SOURCE")
CHEZMOI_CONFIG_ARGS=()
CHEZMOI_DATA_ARGS=()
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
if [[ "$EXPERIMENTAL_WSL_GUI" -eq 1 ]]; then
    CHEZMOI_DATA_ARGS+=(--override-data '{"experimentalWslGui":true}')
fi

phase() {
    echo
    echo "================================================================"
    echo "==  $1"
    echo "================================================================"
}

normalize_machine_arch() {
    case "$1" in
        arm64 | aarch64) printf '%s\n' aarch64 ;;
        x86_64 | amd64) printf '%s\n' x86_64 ;;
        *) return 1 ;;
    esac
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
    POLARIS_CACHE_ROOT="$HOME/.local/share/dotfiles/polaris"
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

polaris_checkout_dir() {
    printf '%s/%s\n' "$POLARIS_CACHE_ROOT" "$POLARIS_REF"
}

polaris_git() {
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

polaris_cache_git() {
    local checkout="$1"
    shift
    polaris_git --git-dir="$checkout/.git" --work-tree="$checkout" "$@"
}

assert_polaris_checkout_clean() {
    local checkout="$1" status

    if ! status="$(polaris_cache_git "$checkout" status --porcelain=v1 --untracked-files=all --ignored=matching 2>/dev/null)"; then
        echo "  FAIL: could not inspect Polaris cache worktree: $checkout" >&2
        exit 1
    fi

    if [[ -n "$status" ]]; then
        echo "  FAIL: Polaris cache has local changes; refusing to execute it: $checkout" >&2
        printf '%s\n' "$status" | sed 's/^/        /' >&2
        echo "        Remove this cache directory and rerun setup to fetch the pinned checkout again." >&2
        exit 1
    fi
}

assert_polaris_release_artifact() {
    local checkout="$1" version="$2" tag="$3" ref="$4" head tag_head checkout_version

    head="$(polaris_cache_git "$checkout" rev-parse --verify 'HEAD^{commit}' 2>/dev/null || true)"
    if [[ "$head" != "$ref" ]]; then
        echo "  FAIL: Polaris cache is not at the pinned commit: $checkout" >&2
        echo "        expected $ref, found ${head:-unknown}" >&2
        exit 1
    fi

    tag_head="$(polaris_cache_git "$checkout" rev-parse --verify "refs/tags/$tag^{commit}" 2>/dev/null || true)"
    if [[ "$tag_head" != "$ref" ]]; then
        echo "  FAIL: Polaris tag mismatch for $tag in $checkout" >&2
        echo "        expected tag to point at $ref, found ${tag_head:-missing}" >&2
        echo "        Remove this cache directory and rerun setup to fetch the pinned release artifact again." >&2
        exit 1
    fi

    checkout_version="$(tr -d '[:space:]' < "$checkout/VERSION" 2>/dev/null || true)"
    if [[ "$checkout_version" != "$version" ]]; then
        echo "  FAIL: Polaris cache VERSION mismatch: expected $version, found ${checkout_version:-missing}" >&2
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
    ask_yes_no_default_yes "Apply Polaris global agent rules?"
}

ensure_polaris_checkout() {
    local checkout tmp
    checkout="$(polaris_checkout_dir)"

    if [[ -d "$checkout/.git" ]]; then
        assert_polaris_release_artifact "$checkout" "$POLARIS_VERSION" "$POLARIS_TAG" "$POLARIS_REF"
        assert_polaris_checkout_clean "$checkout"
        printf '%s\n' "$checkout"
        return 0
    fi

    if [[ -e "$checkout" || -L "$checkout" ]]; then
        echo "  FAIL: Polaris cache path exists but is not a git checkout: $checkout" >&2
        exit 1
    fi

    if ! command -v git >/dev/null 2>&1; then
        echo "  FAIL: git is required to fetch Polaris. Re-run without --skip-deps, or install git first." >&2
        exit 1
    fi

    mkdir -p "$POLARIS_CACHE_ROOT"
    tmp="$(mktemp -d "$POLARIS_CACHE_ROOT/.tmp.XXXXXX")"
    trap 'rm -rf "$tmp"' RETURN

    polaris_git clone "$POLARIS_REPO_URL" "$tmp"
    polaris_git -C "$tmp" checkout --detach "$POLARIS_REF"

    assert_polaris_release_artifact "$tmp" "$POLARIS_VERSION" "$POLARIS_TAG" "$POLARIS_REF"
    assert_polaris_checkout_clean "$tmp"

    mv "$tmp" "$checkout"
    trap - RETURN
    printf '%s\n' "$checkout"
}

run_polaris_agent_policy() {
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

    phase "Phase 6/6: apply global agent policy (Polaris)"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: clone/fetch Polaris $POLARIS_VERSION ($POLARIS_TAG @ $POLARIS_REF)"
        echo "         into $(polaris_checkout_dir)"
        echo "  would: run Polaris tools/install --global, then --global --check"
        return 0
    fi

    checkout="$(ensure_polaris_checkout)"
    installer="$checkout/tools/install"
    if [[ ! -x "$installer" ]]; then
        echo "  FAIL: Polaris installer missing or not executable: $installer" >&2
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
    if [[ "$SKIP_DEPS" -eq 0 ]]; then
        phase "Update 1/2: update proven dependency tools and artifacts"
        bash "$SCRIPT_DIR/install-deps.sh" ${DEPS_FLAGS[@]+"${DEPS_FLAGS[@]}"}
    else
        echo
        echo "skipped: update dependency phase via --skip-deps"
    fi

    refresh_runtime_path

    if [[ "$SKIP_NVIM" -eq 0 ]]; then
        phase "Update 2/2: update Mason LSP servers + formatters"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: nvim --headless +MasonToolsUpdateSync +qa"
        elif command -v nvim >/dev/null 2>&1; then
            run_or_fail "Mason update" nvim --headless "+MasonToolsUpdateSync" "+qa"
        else
            echo "  skipped   Mason update: nvim not on PATH"
        fi
    else
        echo
        echo "skipped: Mason update via --skip-nvim"
    fi

    echo
    echo "Plugins (lazy-lock.json), pinned binaries, and configs update via \`git pull\` then re-run setup; \`:Lazy update\` re-pins plugins (a repo change)."
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

nix_darwin_hosted_ci_cleanup_override() {
    [[ "${DOTFILES_TEST_GITHUB_ACTIONS:-}" == "1" ]] && return 0
    [[ "${GITHUB_ACTIONS:-}" == "true" ]] || return 1
    [[ "${DOTFILES_SETUP_SOURCE_ONLY_ACTIVE:-}" != "1" ]] || return 1
    [[ "${RUNNER_ENVIRONMENT:-}" == "github-hosted" ]] || return 1
    [[ "${RUNNER_OS:-}" == "macOS" ]]
}

sudo_nix_darwin_activation() {
    local target_user="${DOTFILES_TARGET_USER:-}" target_home="${DOTFILES_TARGET_HOME:-}"
    if [[ -z "$target_user" || -z "$target_home" ]]; then
        echo "  FAIL: validated target identity is missing before nix-darwin activation." >&2
        return 1
    fi
    if nix_darwin_hosted_ci_cleanup_override; then
        sudo env DOTFILES_TARGET_USER="$target_user" DOTFILES_TARGET_HOME="$target_home" \
            DOTFILES_NIX_DARWIN_HOSTED_CI=1 "$@"
    else
        sudo env DOTFILES_TARGET_USER="$target_user" DOTFILES_TARGET_HOME="$target_home" "$@"
    fi
}

nix_darwin_sudo_preview() {
    local target_user="${DOTFILES_TARGET_USER:-<resolved-user>}"
    local target_home="${DOTFILES_TARGET_HOME:-<resolved-home>}"
    printf 'sudo env DOTFILES_TARGET_USER=%q DOTFILES_TARGET_HOME=%q ' "$target_user" "$target_home"
    if nix_darwin_hosted_ci_cleanup_override; then
        printf 'DOTFILES_NIX_DARWIN_HOSTED_CI=1 %s' "$1"
    else
        printf '%s' "$1"
    fi
}

nix_homebrew_library_dir() {
    local arch brew_bin repository
    if [[ -n "${DOTFILES_HOMEBREW_LIBRARY:-}" ]]; then
        printf '%s\n' "$DOTFILES_HOMEBREW_LIBRARY"
        return 0
    fi
    brew_bin="$(command -v brew 2>/dev/null || true)"
    if [[ -n "$brew_bin" ]] && repository="$("$brew_bin" --repository 2>/dev/null)" && [[ "$repository" == /* ]]; then
        printf '%s/Library\n' "${repository%/}"
        return 0
    fi
    arch="$(normalize_machine_arch "$(uname -m)")" || {
        echo "  FAIL: cannot resolve the Homebrew library for unsupported architecture $(uname -m)." >&2
        return 1
    }
    case "$arch" in
        aarch64) printf '%s\n' /opt/homebrew/Library ;;
        x86_64) printf '%s\n' /usr/local/Homebrew/Library ;;
    esac
}

prepare_nix_homebrew_declarative_taps() {
    local library taps stamp backup_base backup i
    NIX_HOMEBREW_TAPS_PATH=""
    NIX_HOMEBREW_TAPS_BACKUP=""
    library="$(nix_homebrew_library_dir)" || return 1
    taps="$library/Taps"
    [[ -e "$taps" && ! -L "$taps" ]] || return 0

    stamp="${DOTFILES_TEST_TIMESTAMP:-$(date +%Y%m%d%H%M%S)}"
    backup_base="$library/Taps.dotfiles-pre-nix-$stamp"
    backup="$backup_base"
    i=0
    while [[ -e "$backup" ]]; do
        i=$((i + 1))
        backup="$backup_base.$i"
    done

    echo "  note      nix-homebrew mutableTaps=false needs to own $taps"
    echo "            moving existing taps to $backup before activation."
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: sudo mv $taps $backup"
        return 0
    fi
    if [[ "$ALL" -eq 0 ]]; then
        if ! ask_yes_no_default_yes "Move existing Homebrew taps aside for declarative nix-homebrew?"; then
            echo "  FAIL: nix-homebrew mutableTaps=false cannot activate while $taps exists." >&2
            echo "        Move it aside or re-run with --skip-deps on an already provisioned host." >&2
            return 1
        fi
    fi
    NIX_HOMEBREW_TAPS_PATH="$taps"
    NIX_HOMEBREW_TAPS_BACKUP="$backup"
    if ! sudo mv "$taps" "$backup"; then
        NIX_HOMEBREW_TAPS_PATH=""
        NIX_HOMEBREW_TAPS_BACKUP=""
        echo "  FAIL: could not move existing Homebrew taps from $taps to $backup." >&2
        return 1
    fi
}

rollback_nix_homebrew_declarative_taps() {
    local taps="$NIX_HOMEBREW_TAPS_PATH" backup="$NIX_HOMEBREW_TAPS_BACKUP"
    local stamp failed_base failed i=0
    [[ -n "$taps" && -n "$backup" ]] || return 0
    if [[ ! -e "$backup" && ! -L "$backup" ]]; then
        echo "  FAIL: tap rollback backup is missing: $backup" >&2
        echo "        Preserve $taps and restore the backup manually before retrying." >&2
        return 1
    fi
    if [[ -e "$taps" || -L "$taps" ]]; then
        stamp="${DOTFILES_TEST_TIMESTAMP:-$(date +%Y%m%d%H%M%S)}"
        failed_base="$taps.dotfiles-failed-$stamp"
        failed="$failed_base"
        while [[ -e "$failed" || -L "$failed" ]]; do
            i=$((i + 1))
            failed="$failed_base.$i"
        done
        if ! sudo mv "$taps" "$failed"; then
            echo "  FAIL: activation failed and rollback could not move the replacement taps at $taps." >&2
            echo "        Original taps remain safe at $backup; move the replacement aside, then restore them." >&2
            return 1
        fi
        echo "  note      failed activation taps preserved at $failed"
    fi
    if ! sudo mv "$backup" "$taps"; then
        echo "  FAIL: activation failed and automatic tap rollback could not restore $backup." >&2
        echo "        Restore it manually with: sudo mv '$backup' '$taps'" >&2
        return 1
    fi
    echo "  restored  pre-activation Homebrew taps from $backup"
    NIX_HOMEBREW_TAPS_PATH=""
    NIX_HOMEBREW_TAPS_BACKUP=""
}

nix_homebrew_activation_interrupted() {
    local signal="$1" rc=130
    [[ "$signal" == "TERM" ]] && rc=143
    trap - INT TERM
    echo "  FAIL: nix-darwin activation interrupted by $signal; restoring pre-activation taps." >&2
    rollback_nix_homebrew_declarative_taps || true
    exit "$rc"
}

complete_nix_homebrew_tap_transaction() {
    if [[ -n "$NIX_HOMEBREW_TAPS_BACKUP" ]]; then
        echo "  backup    pre-activation Homebrew taps retained at $NIX_HOMEBREW_TAPS_BACKUP"
    fi
    NIX_HOMEBREW_TAPS_PATH=""
    NIX_HOMEBREW_TAPS_BACKUP=""
}

# Required POSIX package layer: apply Nix (nix-darwin) on macOS before native
# dependency provisioning. It activates declarative Homebrew (WezTerm/AeroSpace
# casks, Herdr brew) and the nix-owned CLI package set. chezmoi still owns every
# dotfile (invariant 22). setup resolves one authoritative target user/home and
# passes both through sudo explicitly for impure flake evaluation.
run_nix_darwin_switch() {
    local explicit=0 arch config_name flake_ref bootstrap_ref
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
    arch="$(normalize_machine_arch "$(uname -m)")" || {
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would fail: no supported nix-darwin activation config for arch $(uname -m)."
                return 0
            fi
            echo "  FAIL: no supported nix-darwin activation config for arch $(uname -m)."
            return 1
    }
    case "$arch" in
        aarch64) config_name="dotfiles-aarch64" ;;
        x86_64) config_name="dotfiles-x86_64" ;;
    esac
    if ! command -v nix >/dev/null 2>&1; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
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
    echo "  Runs '$(nix_darwin_sudo_preview "darwin-rebuild") switch --flake $flake_ref --impure':"
    echo "  declarative Homebrew (WezTerm/AeroSpace casks, Herdr brew) + nix CLI set."
    echo "  It uses sudo for nix-darwin's upstream activation shape while passing"
    echo "  the setup-validated target user/home explicitly through the boundary."
    if nix_darwin_hosted_ci_cleanup_override; then
        echo "  GitHub-hosted macOS runner mode: Homebrew cleanup check is disabled for"
        echo "  this disposable activation only because runner images preinstall Brew tools."
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: $(nix_darwin_sudo_preview "darwin-rebuild") switch --flake $flake_ref --impure"
        echo "         (first-time bootstrap: $(nix_darwin_sudo_preview "nix") run $bootstrap_ref -- switch --flake $flake_ref --impure)"
        return 0
    fi
    if [[ "$ALL" -eq 0 ]]; then
        if ! ask_yes_no_default_yes "Apply the required nix-darwin package layer now?"; then
            echo "  FAIL: macOS setup requires nix-darwin for package provisioning."
            echo "        Re-run with --skip-deps only on a host that is already provisioned."
            return 1
        fi
    fi
    trap 'nix_homebrew_activation_interrupted INT' INT
    trap 'nix_homebrew_activation_interrupted TERM' TERM
    if ! prepare_nix_homebrew_declarative_taps; then
        trap - INT TERM
        return 1
    fi
    if command -v darwin-rebuild >/dev/null 2>&1; then
        if ! sudo_nix_darwin_activation darwin-rebuild switch --flake "$flake_ref" --impure; then
            trap - INT TERM
            echo "  FAIL: nix-darwin activation failed; setup did not apply the requested Nix package layer." >&2
            if ! rollback_nix_homebrew_declarative_taps; then
                echo "        Tap rollback also failed; follow the recovery instructions above before retrying." >&2
            fi
            echo "        Re-run './setup.sh' after fixing the activation error." >&2
            return 1
        fi
    else
        echo "  bootstrapping nix-darwin from flake.lock ($bootstrap_ref)..."
        if ! sudo_nix_darwin_activation nix run "$bootstrap_ref" -- switch --flake "$flake_ref" --impure; then
            trap - INT TERM
            echo "  FAIL: nix-darwin bootstrap activation failed; setup did not apply the requested Nix package layer." >&2
            if ! rollback_nix_homebrew_declarative_taps; then
                echo "        Tap rollback also failed; follow the recovery instructions above before retrying." >&2
            fi
            echo "        Re-run './setup.sh' after fixing the activation error." >&2
            return 1
        fi
    fi
    trap - INT TERM
    complete_nix_homebrew_tap_transaction
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
fi
if [[ -n "${DOTFILES_SETUP_SOURCE_ONLY:-}" ]]; then
    DOTFILES_SETUP_SOURCE_ONLY_ACTIVE=1
    # shellcheck disable=SC2317  # the exit is reached only when executed, not sourced
    return 0 2>/dev/null || exit 0
fi

if [[ "$UPDATE_MODE" -eq 1 ]]; then
    run_update_mode
    exit 0
fi

# ---- Required POSIX package layer --------------------------------------------
run_nix_darwin_switch
run_home_manager_switch
refresh_runtime_path

# ---- Phase 1: dependencies ---------------------------------------------------
if [[ "$SKIP_DEPS" -eq 0 ]]; then
    phase "Phase 1/6: install dependencies"
    bash "$SCRIPT_DIR/install-deps.sh" ${DEPS_FLAGS[@]+"${DEPS_FLAGS[@]}"}
else
    echo
    echo "skipped: Phase 1 (deps) via --skip-deps"
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
        run_chezmoi --dry-run --verbose apply
        cleanup_chezmoi_dry_config
        trap - EXIT
    else
        chezmoi "${CHEZMOI_BASE_ARGS[@]}" init
        backup_preexisting_managed_targets
        run_chezmoi --no-tty --force apply
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
        run_or_fail "Mason install" nvim --headless "+MasonToolsInstallSync" "+qa"
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

run_polaris_agent_policy

# ---- Summary -----------------------------------------------------------------
echo
echo "================================================================"
echo "==  setup.sh: done"
echo "================================================================"
echo
echo "Repo:    $SCRIPT_DIR"
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
