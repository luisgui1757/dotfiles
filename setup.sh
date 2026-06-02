#!/usr/bin/env bash
# setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.
#
# Local usage (from a checked-out copy):
#   ./setup.sh                     interactive: Y/n per dep, then symlink + sync
#   ./setup.sh --all               non-interactive: install everything missing
#   ./setup.sh --dry-run           preview every step
#   ./setup.sh --skip-deps         already have nvim/starship; just bootstrap+sync
#   ./setup.sh --skip-bootstrap    already symlinked; just sync plugins+LSP
#   ./setup.sh --skip-nvim         skip nvim plugin + Mason sync
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
SKIP_DEPS=0
SKIP_BOOTSTRAP=0
SKIP_NVIM=0
BEST_EFFORT=0
usage() {
    cat <<'EOF'
setup.sh -- one-shot end-to-end install for macOS / Linux / WSL.

Local usage:
  ./setup.sh                     interactive: one prompt, then end-to-end
  ./setup.sh --all               non-interactive: install everything missing
  ./setup.sh --dry-run           preview every step
  ./setup.sh --skip-deps         already installed; just bootstrap + sync
  ./setup.sh --skip-bootstrap    already symlinked; just sync plugins + LSP
  ./setup.sh --skip-nvim         skip nvim plugin + Mason sync

Remote usage:
  curl -fsSL https://raw.githubusercontent.com/luisgui1757/dotfiles/main/setup.sh | bash -s -- --all
EOF
}

for arg in "$@"; do
    case "$arg" in
        --all|-y)         ALL=1 ;;
        --dry-run)        DRY_RUN=1 ;;
        --skip-deps)      SKIP_DEPS=1 ;;
        --skip-bootstrap) SKIP_BOOTSTRAP=1 ;;
        --skip-nvim)      SKIP_NVIM=1 ;;
        --best-effort)    BEST_EFFORT=1 ;;
        -h|--help)        usage; exit 0 ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done
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
if [[ -z "$SCRIPT_DIR" ]] || [[ ! -f "$SCRIPT_DIR/bootstrap.sh" ]]; then
    DEST="${DOTFILES_DEST:-$DEFAULT_DEST}"
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
        echo "setup.sh: git is the only prerequisite for remote bootstrap, and it is required to clone the repo." >&2
        echo "setup.sh: install git first: $git_hint" >&2
        exit 1
    fi
    if [[ -d "$DEST/.git" ]]; then
        echo "Repo already cloned at $DEST. Pulling latest."
        git -C "$DEST" pull --ff-only
    else
        echo "Cloning $REPO_URL -> $DEST"
        git clone "$REPO_URL" "$DEST"
    fi
    echo
    echo "Re-invoking setup.sh from the clone."
    exec bash "$DEST/setup.sh" "$@"
fi

cd "$SCRIPT_DIR"

# ---- Self-link guard ---------------------------------------------------------
# If the repo lives at one of the symlink targets we will later create,
# bootstrap.sh will try to symlink the path onto itself. Detect and refuse.
NVIM_TARGET="$HOME/.config/nvim"
if [[ "$SCRIPT_DIR" == "$NVIM_TARGET" ]]; then
    cat >&2 <<EOF
setup.sh: the repo is currently at $SCRIPT_DIR, which is the same path
that bootstrap.sh would symlink to itself.

Move the repo elsewhere first (e.g. ~/dotfiles), then re-run setup.sh:

    mv "$SCRIPT_DIR" "$HOME/dotfiles"
    cd "$HOME/dotfiles"
    ./setup.sh
EOF
    exit 1
fi

# ---- Forward flags to sub-scripts --------------------------------------------
DEPS_FLAGS=()
[[ "$ALL" -eq 1 ]]      && DEPS_FLAGS+=(--all)
[[ "$DRY_RUN" -eq 1 ]]  && DEPS_FLAGS+=(--dry-run)

BOOTSTRAP_FLAGS=()
[[ "$DRY_RUN" -eq 1 ]]  && BOOTSTRAP_FLAGS+=(--dry-run)

phase() {
    echo
    echo "================================================================"
    echo "==  $1"
    echo "================================================================"
}

refresh_runtime_path() {
    local brew_bin dir

    for brew_bin in "$(command -v brew 2>/dev/null || true)" \
        /opt/homebrew/bin/brew \
        /usr/local/bin/brew \
        "$HOME/.linuxbrew/bin/brew" \
        /home/linuxbrew/.linuxbrew/bin/brew; do
        if [[ -n "$brew_bin" && -x "$brew_bin" ]]; then
            eval "$("$brew_bin" shellenv)"
            break
        fi
    done

    dir=/usr/local/bin
    if [[ -d "$dir" && ":$PATH:" != *":$dir:"* ]]; then
        PATH="$PATH:$dir"
    fi
    export PATH
    hash -r 2>/dev/null || true
}

# ---- Phase 1: dependencies ---------------------------------------------------
if [[ "$SKIP_DEPS" -eq 0 ]]; then
    phase "Phase 1/4: install dependencies"
    bash "$SCRIPT_DIR/install-deps.sh" ${DEPS_FLAGS[@]+"${DEPS_FLAGS[@]}"}
else
    echo
    echo "skipped: Phase 1 (deps) via --skip-deps"
fi
refresh_runtime_path

# ---- Phase 2: symlink configs ------------------------------------------------
if [[ "$SKIP_BOOTSTRAP" -eq 0 ]]; then
    phase "Phase 2/4: symlink configs into place"
    bash "$SCRIPT_DIR/bootstrap.sh" ${BOOTSTRAP_FLAGS[@]+"${BOOTSTRAP_FLAGS[@]}"}
else
    echo
    echo "skipped: Phase 2 (bootstrap) via --skip-bootstrap"
fi

# ---- Phase 3: install Neovim plugins -----------------------------------------
# ---- Phase 4: install LSP servers + formatters via Mason ---------------------
#
# By default, Lazy + Mason failures are FATAL — they leave the user with a
# bare nvim config and no LSP. Pass --best-effort to downgrade to warnings.
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
        echo "         Re-run: ./setup.sh --skip-deps --skip-bootstrap"
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
