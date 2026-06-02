#!/usr/bin/env bash
# Container end-to-end check for the Ubuntu container gate in
# .github/workflows/e2e-install.yml. Lives in a file (not inline YAML) to avoid
# fragile nested shell quoting.
#
# Two modes:
#   (no args)   root: install prereqs for $EXPECTED_PM, create an unprivileged
#               user, copy the read-only /repo mount, then re-exec as that user.
#   --as-user   run the REAL install-deps.sh --all (native PM, no brew) +
#               bootstrap.sh, then assert the result.
set -euo pipefail

EXPECTED_PM="${EXPECTED_PM:?EXPECTED_PM must be set}"

fail() { echo "FAIL: $*" >&2; exit 1; }

run_and_capture() {
    local label="$1" log="$2" rc
    shift 2
    set +e
    "$@" 2>&1 | tee "$log"
    rc=${PIPESTATUS[0]}
    set -e
    if [[ "$rc" -ne 0 ]]; then
        fail "$label exited $rc"
    fi
    if grep -Eq '^[[:space:]]*FAIL:' "$log"; then
        fail "$label emitted a FAIL marker"
    fi
}

assert_link() {
    # assert_link <path> <expected-target>
    [[ -L "$1" ]] || fail "$1 is not a symlink"
    _actual="$(readlink "$1")"
    [[ "$_actual" == "$2" ]] || fail "$1 points to $_actual, expected $2"
}

if [[ "${1:-}" == "--as-user" ]]; then
    cd "$HOME/dotfiles" || fail "repo copy missing at $HOME/dotfiles"

    run_and_capture "install-deps.sh" "$HOME/install-deps.log" ./install-deps.sh --all
    grep -F "package manager=$EXPECTED_PM" "$HOME/install-deps.log" >/dev/null \
        || fail "install-deps did not keep native package manager $EXPECTED_PM"
    if grep -F "package manager=brew" "$HOME/install-deps.log" >/dev/null; then
        fail "install-deps switched to brew (native PM expected)"
    fi

    run_and_capture "bootstrap.sh" "$HOME/bootstrap.log" ./bootstrap.sh

    # install_nvim_linux symlinks nvim into /usr/local/bin, and apt's fd-find is
    # symlinked to ~/.local/bin/fd. A real login shell has both on PATH.
    for d in /usr/local/bin "$HOME/.local/bin"; do
        case ":$PATH:" in
            *":$d:"*) ;;
            *) [[ -d "$d" ]] && PATH="$d:$PATH" ;;
        esac
    done
    export PATH

    for cmd in nvim rg fd fzf tmux zsh git; do
        command -v "$cmd" >/dev/null 2>&1 || fail "$cmd is not on PATH"
    done

    nvim_line="$(nvim --version | head -n 1)"
    case "$nvim_line" in
        "NVIM v0.11"* | "NVIM v0.12"* | "NVIM v1."*) ;;
        *) fail "nvim version is below 0.11: $nvim_line" ;;
    esac

    repo="$(pwd -P)"
    assert_link "$HOME/.config/nvim" "$repo/nvim"
    assert_link "$HOME/.config/starship.toml" "$repo/starship/starship.toml"
    assert_link "$HOME/.tmux.conf" "$repo/tmux/tmux.conf"
    assert_link "$HOME/.zshrc" "$repo/shells/zshrc"
    assert_link "$HOME/.config/lazygit/config.yml" "$repo/lazygit/config.yml"

    echo "OK: $EXPECTED_PM container e2e passed"
    exit 0
fi

# ---- root prep ---------------------------------------------------------------
case "$EXPECTED_PM" in
    apt)
        export DEBIAN_FRONTEND=noninteractive
        apt-get update
        apt-get install -y --no-install-recommends \
            bash sudo ca-certificates curl tar gzip unzip xz-utils \
            findutils coreutils procps passwd git
        ;;
    *)
        fail "unknown package manager: $EXPECTED_PM; add a matrix entry and root prep branch together"
        ;;
esac

if command -v update-ca-certificates >/dev/null 2>&1; then
    update-ca-certificates >/dev/null 2>&1 || true
fi

if command -v useradd >/dev/null 2>&1; then
    useradd -m -s /bin/bash dotfiles
else
    adduser -D -s /bin/bash dotfiles
fi

printf '%s\n' "dotfiles ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/dotfiles
chmod 0440 /etc/sudoers.d/dotfiles

cp -a /repo /home/dotfiles/dotfiles
chown -R dotfiles:dotfiles /home/dotfiles/dotfiles

exec sudo -H -u dotfiles env \
    DOTFILES_SKIP_BREW_BOOTSTRAP=1 \
    EXPECTED_PM="$EXPECTED_PM" \
    bash "/home/dotfiles/dotfiles/tests/ci/container-e2e.sh" --as-user
