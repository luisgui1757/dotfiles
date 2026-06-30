#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
COMMAND_LOG="$TMP_ROOT/commands.log"

mktool() {
    local path="$1"
    mkdir -p "$(dirname "$path")"
    cat > "$path" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$path"
}

assert_contains() {
    local needle="$1" file="$2" message="$3"
    grep -F "$needle" "$file" >/dev/null || fail "$message"
}

assert_not_contains() {
    local needle="$1" file="$2" message="$3"
    if grep -F "$needle" "$file" >/dev/null; then
        fail "$message"
    fi
}

run_case() {
    local name="$1"
    shift
    : > "$COMMAND_LOG"
    ( "$@" ) || fail "$name"
}

case_mixed_linuxbrew_and_apt() {
    local root="$TMP_ROOT/mixed" brew_prefix="$TMP_ROOT/mixed/homebrew" apt_root="$TMP_ROOT/mixed/usr"
    mkdir -p "$brew_prefix/bin" "$apt_root/bin"
    mktool "$brew_prefix/bin/rg"
    mktool "$apt_root/bin/jq"
    PATH="$brew_prefix/bin:$apt_root/bin:/usr/bin:/bin"
    HOME="$root/home"
    export HOME PATH
    DRY_RUN=0
    PM=brew
    INSTALL_DEPS_UPDATE_TOOLS=$'rg\njq'
    APT_UPDATE_REFRESHED=0
    APT_UPDATE_REFRESH_OK=1

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    native_linux_pm() { printf '%s\n' "apt"; }
    homebrew_bin() { printf '%s\n' "$brew_prefix/bin/brew"; }
    cat > "$brew_prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$brew_prefix"
fi
EOF
    chmod +x "$brew_prefix/bin/brew"
    brew() {
        printf 'brew %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "list --formula ripgrep") return 0 ;;
            "outdated --formula --quiet ripgrep") printf '%s\n' "ripgrep"; return 0 ;;
            "upgrade ripgrep") return 0 ;;
            *) return 1 ;;
        esac
    }
    dpkg-query() {
        case "$*" in
            "-S $apt_root/bin/jq") printf '%s\n' "jq: $apt_root/bin/jq" ;;
            "-W -f=\${Version} jq") printf '%s\n' "1.0" ;;
            *) return 1 ;;
        esac
    }
    apt-cache() {
        [[ "$*" == "policy jq" ]] || return 1
        printf '%s\n' "  Installed: 1.0" "  Candidate: 2.0"
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        return 0
    }

    update_catalog_tools > "$root.out"

    assert_contains "updated   rg" "$root.out" "Linuxbrew-owned rg was not reported updated"
    assert_contains "owner=brew package=ripgrep source=$brew_prefix/bin/rg" "$root.out" "rg output did not include Brew proof"
    assert_contains "updated   jq" "$root.out" "apt-owned jq was not reported updated"
    assert_contains "owner=apt package=jq source=$apt_root/bin/jq" "$root.out" "jq output did not include apt proof"
    assert_contains "brew upgrade ripgrep" "$COMMAND_LOG" "Linuxbrew-owned rg did not use brew upgrade ripgrep"
    assert_contains "apt-get update -qq" "$COMMAND_LOG" "apt metadata was not refreshed"
    assert_contains "apt-get install -y --only-upgrade jq" "$COMMAND_LOG" "apt-owned jq did not use a scoped upgrade"
}

case_brew_current_does_not_upgrade() {
    local root="$TMP_ROOT/brew-current" brew_prefix="$TMP_ROOT/brew-current/homebrew"
    mkdir -p "$brew_prefix/bin"
    mktool "$brew_prefix/bin/git"
    PATH="$brew_prefix/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=brew
    INSTALL_DEPS_UPDATE_TOOLS="git"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    native_linux_pm() { printf '%s\n' "unknown"; }
    homebrew_bin() { printf '%s\n' "$brew_prefix/bin/brew"; }
    cat > "$brew_prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$brew_prefix"
fi
EOF
    chmod +x "$brew_prefix/bin/brew"
    brew() {
        printf 'brew %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "list --formula git") return 0 ;;
            "outdated --formula --quiet git") return 0 ;;
            "upgrade git") return 0 ;;
            *) return 1 ;;
        esac
    }

    update_catalog_tools > "$root.out"

    assert_contains "current   git" "$root.out" "current Brew git was not reported current"
    assert_contains "owner=brew package=git source=$brew_prefix/bin/git" "$root.out" "current Brew git output lacked proof"
    assert_not_contains "brew upgrade git" "$COMMAND_LOG" "current Brew git still ran brew upgrade"
}

case_brew_shadowed_source_is_unmanaged() {
    local root="$TMP_ROOT/brew-shadowed" brew_prefix="$TMP_ROOT/brew-shadowed/homebrew" shadow="$TMP_ROOT/brew-shadowed/shadow"
    mkdir -p "$brew_prefix/bin" "$shadow/bin"
    mktool "$shadow/bin/git"
    PATH="$shadow/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=brew
    INSTALL_DEPS_UPDATE_TOOLS="git"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    native_linux_pm() { printf '%s\n' "unknown"; }
    homebrew_bin() { printf '%s\n' "$brew_prefix/bin/brew"; }
    cat > "$brew_prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$brew_prefix"
fi
EOF
    chmod +x "$brew_prefix/bin/brew"
    brew() {
        printf 'brew %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "list --formula git") return 0 ;;
            *) return 1 ;;
        esac
    }

    update_catalog_tools > "$root.out"

    assert_contains "unmanaged git" "$root.out" "shadowed git was not unmanaged"
    assert_contains "source=$shadow/bin/git" "$root.out" "shadowed git did not report the resolved source"
    assert_not_contains "brew upgrade git" "$COMMAND_LOG" "shadowed git attempted a Brew upgrade"
}

case_brew_prefix_without_formula_blocks() {
    local root="$TMP_ROOT/brew-blocked" brew_prefix="$TMP_ROOT/brew-blocked/homebrew"
    mkdir -p "$brew_prefix/bin"
    mktool "$brew_prefix/bin/rg"
    PATH="$brew_prefix/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=brew
    INSTALL_DEPS_UPDATE_TOOLS="rg"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    native_linux_pm() { printf '%s\n' "unknown"; }
    homebrew_bin() { printf '%s\n' "$brew_prefix/bin/brew"; }
    cat > "$brew_prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$brew_prefix"
fi
EOF
    chmod +x "$brew_prefix/bin/brew"
    brew() {
        printf 'brew %s\n' "$*" >> "$COMMAND_LOG"
        return 1
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "Brew-prefix rg without an installed formula did not fail closed"
    fi
    assert_contains "blocked   rg" "$root.err" "Brew-prefix ownership contradiction was not blocked"
    assert_contains "source-under-brew-prefix-but-formula-not-installed" "$root.err" "Brew blocked reason was not precise"
}

case_macos_system_zsh_is_accepted() {
    local root="$TMP_ROOT/macos-system" bin="$TMP_ROOT/macos-system/bin"
    mkdir -p "$bin"
    mktool "$bin/zsh"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=brew
    INSTALL_DEPS_UPDATE_TOOLS="zsh"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Darwin" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    real_source_path() {
        if [[ "$1" == "$bin/zsh" ]]; then
            printf '%s\n' "/bin/zsh"
        else
            printf '%s\n' "$1"
        fi
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }

    update_catalog_tools > "$root.out"
    assert_contains "system    zsh" "$root.out" "macOS /bin/zsh was not accepted as a system provider"
}

case_apt_current_skips_scoped_install() {
    local root="$TMP_ROOT/apt-current" apt_root="$TMP_ROOT/apt-current/usr"
    mkdir -p "$apt_root/bin"
    mktool "$apt_root/bin/jq"
    PATH="$apt_root/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=apt
    INSTALL_DEPS_UPDATE_TOOLS="jq"
    APT_UPDATE_REFRESHED=0
    APT_UPDATE_REFRESH_OK=1

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "apt"; }
    dpkg-query() {
        case "$*" in
            "-S $apt_root/bin/jq") printf '%s\n' "jq: $apt_root/bin/jq" ;;
            "-W -f=\${Version} jq") printf '%s\n' "1.0" ;;
            *) return 1 ;;
        esac
    }
    apt-cache() {
        [[ "$*" == "policy jq" ]] || return 1
        printf '%s\n' "  Installed: 1.0" "  Candidate: 1.0"
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        return 0
    }

    update_catalog_tools > "$root.out"

    assert_contains "current   jq" "$root.out" "current apt jq was not reported current"
    assert_contains "apt-get update -qq" "$COMMAND_LOG" "apt current path did not refresh metadata once"
    assert_not_contains "apt-get install -y --only-upgrade jq" "$COMMAND_LOG" "current apt jq still ran a scoped install"
}

case_pacman_owner_is_explicitly_skipped() {
    local root="$TMP_ROOT/pacman-skip" pacroot="$TMP_ROOT/pacman-skip/usr"
    mkdir -p "$pacroot/bin"
    mktool "$pacroot/bin/make"
    PATH="$pacroot/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=pacman
    INSTALL_DEPS_UPDATE_TOOLS="make"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "pacman"; }
    pacman() {
        case "$*" in
            "-Qo -q $pacroot/bin/make") printf '%s\n' "make" ;;
            *) return 1 ;;
        esac
    }

    update_catalog_tools > "$root.out"
    assert_contains "skipped   make" "$root.out" "pacman-owned make was not skipped"
    assert_contains "owner=pacman reason=requires explicit system upgrade" "$root.out" "pacman skip reason was not explicit"
}

case_direct_artifact_current() {
    local root="$TMP_ROOT/direct-current" bin="$TMP_ROOT/direct-current/home/.local/bin"
    mkdir -p "$bin"
    HOME="$TMP_ROOT/direct-current/home"
    export HOME
    mktool "$bin/lazygit"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$TMP_ROOT/direct-current/provenance"
    export DOTFILES_PROVENANCE_DIR
    INSTALL_DEPS_UPDATE_TOOLS="lazygit"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }
    direct_artifact_current_metadata lazygit
    write_direct_artifact_provenance "lazygit" "$bin/lazygit" "$bin/lazygit" "$bin" "$DIRECT_ARTIFACT_URL" "$DIRECT_ARTIFACT_VERSION" "$DIRECT_ARTIFACT_SHA256"

    update_catalog_tools > "$root.out"
    assert_contains "current   lazygit" "$root.out" "matching direct artifact was not current"
    assert_contains "owner=dotfiles-artifact version=$LAZYGIT_LINUX_VERSION source=$bin/lazygit" "$root.out" "direct artifact output did not include provenance proof"
}

case_direct_artifact_legacy_unmarked_is_unmanaged() {
    local root="$TMP_ROOT/direct-unmarked" bin="$TMP_ROOT/direct-unmarked/home/.local/bin"
    mkdir -p "$bin"
    HOME="$TMP_ROOT/direct-unmarked/home"
    export HOME
    mktool "$bin/starship"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$TMP_ROOT/direct-unmarked/provenance"
    export DOTFILES_PROVENANCE_DIR
    INSTALL_DEPS_UPDATE_TOOLS="starship"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }

    update_catalog_tools > "$root.out"
    assert_contains "unmanaged starship" "$root.out" "legacy unmarked direct artifact was not unmanaged"
    assert_contains "source=$bin/starship" "$root.out" "legacy direct artifact did not report source"
}

case_direct_artifact_corrupt_marker_blocks() {
    local root="$TMP_ROOT/direct-blocked" bin="$TMP_ROOT/direct-blocked/home/.local/bin" marker_dir="$TMP_ROOT/direct-blocked/provenance"
    mkdir -p "$bin" "$marker_dir"
    HOME="$TMP_ROOT/direct-blocked/home"
    export HOME
    mktool "$bin/starship"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
    export DOTFILES_PROVENANCE_DIR
    INSTALL_DEPS_UPDATE_TOOLS="starship"
    printf '%s\n' "schema=1" "tool=starship" "version=$STARSHIP_VERSION" "source_url=bad" "sha256=bad" "command_path=$bin/starship" "binary_path=$bin/missing" "install_root=$bin" > "$marker_dir/starship.env"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "corrupt direct artifact provenance did not fail closed"
    fi
    assert_contains "blocked   starship" "$root.err" "corrupt direct artifact was not blocked"
    assert_contains "owner=dotfiles-artifact" "$root.err" "direct artifact block did not identify owner"
}

case_direct_artifact_stale_marker_refreshes_pin() {
    local root="$TMP_ROOT/direct-refresh" bin="$TMP_ROOT/direct-refresh/home/.local/bin" marker_dir="$TMP_ROOT/direct-refresh/provenance"
    mkdir -p "$bin" "$marker_dir"
    HOME="$TMP_ROOT/direct-refresh/home"
    export HOME
    mktool "$bin/lazygit"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
    export DOTFILES_PROVENANCE_DIR
    INSTALL_DEPS_UPDATE_TOOLS="lazygit"
    direct_artifact_current_metadata lazygit
    write_direct_artifact_provenance "lazygit" "$bin/lazygit" "$bin/lazygit" "$bin" "$DIRECT_ARTIFACT_URL" "v0.0.0" "$DIRECT_ARTIFACT_SHA256"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }
    refresh_direct_artifact() {
        printf '%s\n' "refresh $1" >> "$COMMAND_LOG"
        direct_artifact_current_metadata "$1"
        write_direct_artifact_provenance "$1" "$bin/$1" "$bin/$1" "$bin" "$DIRECT_ARTIFACT_URL" "$DIRECT_ARTIFACT_VERSION" "$DIRECT_ARTIFACT_SHA256"
    }

    update_catalog_tools > "$root.out"
    assert_contains "refresh lazygit" "$COMMAND_LOG" "stale direct artifact did not invoke refresh"
    assert_contains "updated   lazygit" "$root.out" "stale direct artifact was not reported updated"
}

run_case "mixed Linuxbrew and apt" case_mixed_linuxbrew_and_apt
run_case "Brew current" case_brew_current_does_not_upgrade
run_case "Brew shadowed source" case_brew_shadowed_source_is_unmanaged
run_case "Brew prefix without formula" case_brew_prefix_without_formula_blocks
run_case "macOS system zsh" case_macos_system_zsh_is_accepted
run_case "apt current" case_apt_current_skips_scoped_install
run_case "pacman explicit skip" case_pacman_owner_is_explicitly_skipped
run_case "direct artifact current" case_direct_artifact_current
run_case "direct artifact unmarked" case_direct_artifact_legacy_unmarked_is_unmanaged
run_case "direct artifact corrupt marker" case_direct_artifact_corrupt_marker_blocks
run_case "direct artifact refresh" case_direct_artifact_stale_marker_refreshes_pin

echo "OK"
