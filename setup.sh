#!/usr/bin/env bash
# setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.
#
# Local usage (from a checked-out copy):
#   ./setup.sh                     interactive: Y/n per dep, then config + sync
#   ./setup.sh --all               non-interactive: install everything missing
#   ./setup.sh --update            update package-manager tools + Mason only
#   ./setup.sh --dry-run           preview every step
#   ./setup.sh --skip-deps         already have nvim/starship; just config+sync
#   ./setup.sh --skip-bootstrap    back-compat alias: skip config apply
#   ./setup.sh --skip-config       already configured; just sync plugins+LSP
#   ./setup.sh --skip-nvim         skip nvim plugin + Mason sync
#   ./setup.sh --experimental-wsl-gui
#                                  WSL opt-in: install/link Linux GUI terminal bits
#
# Remote usage (no checkout yet):
#   curl -fsSL https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.sh | bash -s -- --all
#
# The remote form clones the repo to $DOTFILES_DEST (default ~/dotfiles)
# and then re-invokes itself locally. Set DOTFILES_DEST=/some/other/path
# in the environment if you want a different location.

set -euo pipefail

REPO_URL="https://github.com/luisgui1757/dotfiles.git"
DEFAULT_DEST="$HOME/dotfiles"

ALL=0
DRY_RUN=0
UPDATE_MODE=0
SKIP_DEPS=0
SKIP_BOOTSTRAP=0
SKIP_NVIM=0
BEST_EFFORT=0
EXPERIMENTAL_WSL_GUI=0
usage() {
    cat <<'EOF'
setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.

Local usage:
  ./setup.sh                     interactive: one prompt, then end-to-end
  ./setup.sh --all               non-interactive: install everything missing
  ./setup.sh --update            update package-manager tools + Mason only
  ./setup.sh --dry-run           preview every step
  ./setup.sh --skip-deps         already installed; just config + sync
  ./setup.sh --skip-bootstrap    back-compat alias: skip config apply
  ./setup.sh --skip-config       already configured; just sync plugins + LSP
  ./setup.sh --skip-nvim         skip nvim plugin + Mason sync
  ./setup.sh --best-effort       continue past plugin/LSP/Mason phase failures
  ./setup.sh --experimental-wsl-gui
                                WSL opt-in: install/link Linux Ghostty + Linux fonts

Remote usage:
  curl -fsSL https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.sh | bash -s -- --all
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
        --best-effort)    BEST_EFFORT=1 ;;
        --experimental-wsl-gui)
                          EXPERIMENTAL_WSL_GUI=1 ;;
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

# ---- Locate / clone the repo -------------------------------------------------
# When invoked via `curl | bash`, BASH_SOURCE is empty and there is no
# script_dir. In that case we clone the repo and re-exec ourselves from
# the clone so all downstream paths resolve correctly.
SCRIPT_DIR=""
if [[ -n "${BASH_SOURCE[0]:-}" ]] && [[ -f "${BASH_SOURCE[0]:-}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
fi
if [[ -z "$SCRIPT_DIR" ]] || [[ ! -d "$SCRIPT_DIR/home" ]]; then
    DEST="${DOTFILES_DEST:-$DEFAULT_DEST}"
    if [[ "$UPDATE_MODE" -eq 1 ]]; then
        if [[ -f "$DEST/setup.sh" && -d "$DEST/home" ]]; then
            echo "setup.sh --update: using existing checkout at $DEST without git pull."
            exec bash "$DEST/setup.sh" "$@"
        fi
        echo "setup.sh --update needs an existing checkout at $DEST; it does not clone or pull." >&2
        exit 1
    fi
    # DryRun honor: announce what we'd clone and exit BEFORE any git op.
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "setup.sh (remote, dry-run): would clone $REPO_URL -> $DEST"
        echo "                            then re-invoke ./setup.sh $* from there."
        echo "(dry run -- no clone, no install, no writes performed)"
        exit 0
    fi
    if ! command -v git >/dev/null 2>&1; then
        git_hint="apt install git"
        if [[ "$(uname -s)" == "Darwin" ]]; then
            git_hint="brew install git"
        fi
        echo "setup.sh: git is the only prerequisite for remote setup, and it is required to clone the repo." >&2
        echo "setup.sh: install git first: $git_hint" >&2
        exit 1
    fi
    if [[ -d "$DEST/.git" ]]; then
        echo "Repo already cloned at $DEST. Pulling latest."
        if ! git -C "$DEST" pull --ff-only; then
            echo "setup.sh: 'git pull --ff-only' failed in $DEST; refusing to run against a stale checkout." >&2
            exit 1
        fi
    else
        echo "Cloning $REPO_URL -> $DEST"
        if ! git clone "$REPO_URL" "$DEST"; then
            echo "setup.sh: 'git clone' of $REPO_URL failed; cannot continue." >&2
            exit 1
        fi
    fi
    echo
    echo "Re-invoking setup.sh from the clone."
    exec bash "$DEST/setup.sh" "$@"
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

refresh_runtime_path() {
    local brew_bin brew_env dir

    [[ "$DRY_RUN" -eq 1 ]] && return 0

    for brew_bin in "$(command -v brew 2>/dev/null || true)" \
        /opt/homebrew/bin/brew \
        /usr/local/bin/brew \
        "$HOME/.linuxbrew/bin/brew" \
        /home/linuxbrew/.linuxbrew/bin/brew; do
        if [[ -n "$brew_bin" && -x "$brew_bin" ]]; then
            if brew_env="$("$brew_bin" shellenv)"; then
                eval "$brew_env"
            else
                echo "  WARN: $brew_bin shellenv failed; leaving PATH unchanged for that Homebrew prefix" >&2
            fi
            break
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
        phase "Update 1/2: update package-manager tools"
        bash "$SCRIPT_DIR/install-deps.sh" ${DEPS_FLAGS[@]+"${DEPS_FLAGS[@]}"}
    else
        echo
        echo "skipped: update dependency phase via --skip-deps"
    fi

    refresh_runtime_path

    if [[ "$SKIP_NVIM" -eq 0 ]]; then
        phase "Update 2/2: update Mason LSP servers + formatters"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: nvim --headless +MasonToolsUpdate +qa"
        elif command -v nvim >/dev/null 2>&1; then
            run_or_fail "Mason update" nvim --headless "+MasonToolsUpdate" "+qa"
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

# Test seam: `DOTFILES_SETUP_SOURCE_ONLY=1 source setup.sh` loads the helper
# functions (phase, refresh_runtime_path) without running the install phases, so
# tests can exercise refresh_runtime_path in isolation. Unset in normal runs.
if [[ -z "${DOTFILES_SETUP_SOURCE_ONLY:-}" ]]; then
    refuse_nvim_self_link_if_needed
fi
if [[ -n "${DOTFILES_SETUP_SOURCE_ONLY:-}" ]]; then
    # shellcheck disable=SC2317  # the exit is reached only when executed, not sourced
    return 0 2>/dev/null || exit 0
fi

if [[ "$UPDATE_MODE" -eq 1 ]]; then
    run_update_mode
    exit 0
fi

# ---- Phase 1: dependencies ---------------------------------------------------
if [[ "$SKIP_DEPS" -eq 0 ]]; then
    phase "Phase 1/4: install dependencies"
    bash "$SCRIPT_DIR/install-deps.sh" ${DEPS_FLAGS[@]+"${DEPS_FLAGS[@]}"}
else
    echo
    echo "skipped: Phase 1 (deps) via --skip-deps"
fi
refresh_runtime_path

# ---- Phase 2: apply configs --------------------------------------------------
if [[ "$SKIP_BOOTSTRAP" -eq 0 ]]; then
    phase "Phase 2/4: apply configs with chezmoi"
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

# ---- Phase 3: install Neovim plugins -----------------------------------------
# ---- Phase 4: install LSP servers + formatters via Mason ---------------------
#
# By default, Lazy + Mason failures are FATAL — they leave the user with a
# bare nvim config and no LSP. Pass --best-effort to downgrade to warnings.
if [[ "$SKIP_NVIM" -eq 0 ]] && [[ "$DRY_RUN" -eq 0 ]]; then
    if command -v nvim >/dev/null 2>&1; then
        phase "Phase 3/4: sync Neovim plugins (lazy.nvim)"
        run_or_fail "Lazy sync" nvim --headless "+Lazy! sync" "+qa"

        phase "Phase 4/4: install LSP servers + formatters (Mason)"
        echo "  this can take 3-8 minutes on a fresh machine."
        run_or_fail "Mason install" nvim --headless "+MasonToolsInstallSync" "+qa"
    else
        echo
        echo "skipped: Phase 3-4 (nvim plugins) -- nvim not on PATH yet."
        echo "         Re-run: ./setup.sh --skip-deps --skip-config"
    fi
elif [[ "$DRY_RUN" -eq 1 ]]; then
    echo
    echo "skipped: Phase 3-4 (nvim plugins) in --dry-run mode"
else
    echo
    echo "skipped: Phase 3-4 (nvim plugins) via --skip-nvim"
fi

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
