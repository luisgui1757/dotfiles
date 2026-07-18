#!/usr/bin/env bash
# Container end-to-end check for the Ubuntu container gate in
# .github/workflows/e2e-install.yml. Lives in a file (not inline YAML) to avoid
# fragile nested shell quoting.
#
# Two modes:
#   (no args)   root: install prereqs for $EXPECTED_PM, create an unprivileged
#               user, copy the read-only /repo mount, then re-exec as that user.
#   --as-user   run the REAL install-deps.sh --all (native PM, no brew) +
#               chezmoi config apply, then assert the result.
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

assert_file_content() {
    # assert_file_content <managed-path> <canonical-source>
    [[ -e "$1" ]] || fail "$1 does not exist"
    [[ -f "$1" ]] || fail "$1 does not dereference to a file"
    cmp -s "$1" "$2" || fail "$1 content differs from $2"
}

assert_dir_resolves() {
    # assert_dir_resolves <managed-dir> <canonical-source-dir>
    local actual expected
    [[ -d "$1" ]] || fail "$1 is not a directory"
    actual="$(readlink -f "$1")"
    expected="$(readlink -f "$2")"
    [[ "$actual" == "$expected" ]] || fail "$1 resolves to $actual, expected $expected"
}

assert_tool_runs() {
    local cmd="$1"; shift
    command -v "$cmd" >/dev/null 2>&1 || fail "$cmd is not on PATH"
    "$cmd" "$@" >/dev/null 2>&1 || fail "$cmd $* failed"
}

if [[ "${1:-}" == "--as-user" ]]; then
    cd "$HOME/dotfiles" || fail "repo copy missing at $HOME/dotfiles"

    repo="$(pwd -P)"

    run_and_capture "install-deps.sh" "$HOME/install-deps.log" ./install-deps.sh --all
    grep -F "package manager=$EXPECTED_PM" "$HOME/install-deps.log" >/dev/null \
        || fail "install-deps did not keep native package manager $EXPECTED_PM"
    if grep -F "package manager=brew" "$HOME/install-deps.log" >/dev/null; then
        fail "install-deps switched to brew (native PM expected)"
    fi

    # install_nvim_linux symlinks nvim into /usr/local/bin, apt's fd-find is
    # symlinked to ~/.local/bin/fd, and install-deps installs chezmoi into
    # ~/.local/bin. A real login shell has these on PATH; add them BEFORE we call
    # chezmoi for the config apply (and the tool-presence checks below).
    for d in /usr/local/bin "$HOME/.local/bin"; do
        case ":$PATH:" in
            *":$d:"*) ;;
            *) [[ -d "$d" ]] && PATH="$d:$PATH" ;;
        esac
    done
    export PATH

    run_and_capture "chezmoi init" "$HOME/chezmoi-init.log" \
        chezmoi --source "$repo/home" init
    run_and_capture "chezmoi apply" "$HOME/chezmoi-apply.log" \
        chezmoi --source "$repo/home" --no-tty --force apply

    assert_tool_runs rg --version
    assert_tool_runs fd --version
    assert_tool_runs fzf --version
    assert_tool_runs tmux -V
    assert_tool_runs zsh --version
    assert_tool_runs git --version
    assert_tool_runs lazygit --version
    assert_tool_runs tree-sitter --version
    assert_tool_runs cmake --version
    assert_tool_runs lsd --version
    assert_tool_runs zoxide --version
    assert_tool_runs gh --version
    assert_tool_runs wezterm --version
    assert_tool_runs herdr --version

    nvim_line="$(nvim --version | head -n 1)"
    case "$nvim_line" in
        "NVIM v0.12"* | "NVIM v1."*) ;;
        *) fail "nvim version is below 0.12: $nvim_line" ;;
    esac

    assert_dir_resolves "$HOME/.config/nvim" "$repo/nvim"
    assert_file_content "$HOME/.config/nvim/init.lua" "$repo/nvim/init.lua"
    assert_file_content "$HOME/.config/starship.toml" "$repo/starship/starship.toml"
    assert_file_content "$HOME/.config/lsd/config.yaml" "$repo/lsd/config.yaml"
    assert_file_content "$HOME/.config/lsd/colors.yaml" "$repo/lsd/colors.yaml"
    assert_file_content "$HOME/.tmux.conf" "$repo/tmux/tmux.conf"
    assert_file_content "$HOME/.zshenv" "$repo/shells/zshenv"
    assert_file_content "$HOME/.zshrc" "$repo/shells/zshrc"
    assert_file_content "$HOME/.config/lazygit/config.yml" "$repo/lazygit/config.yml"
    for theme_name in rose-pine rose-pine-moon rose-pine-dawn; do
        assert_file_content \
            "$HOME/.pi/agent/themes/$theme_name.json" \
            "$repo/pi/$theme_name.json"
    done

    plugin_root="$HOME/.local/share/dotfiles/zsh-plugins"
    [[ -r "$plugin_root/fzf-tab/fzf-tab.plugin.zsh" ]] \
        || fail "fzf-tab plugin file missing"
    [[ -r "$plugin_root/zsh-autosuggestions/zsh-autosuggestions.zsh" ]] \
        || fail "zsh-autosuggestions plugin file missing"

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
