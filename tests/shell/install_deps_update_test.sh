#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2317,SC2329
# SC2317/SC2329 are false positives here: command stubs are invoked indirectly
# by the sourced install-deps.sh functions under test.
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

mkversioned_tool() {
    local path="$1" version="$2"
    mkdir -p "$(dirname "$path")"
    cat > "$path" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--version" ]]; then
    printf '%s\n' "$version"
    exit 0
fi
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
    local jq_version="1.0"
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
            "list --formula ripgrep") printf '%s\n' "$brew_prefix/bin/rg"; return 0 ;;
            "outdated --formula --quiet ripgrep") printf '%s\n' "ripgrep"; return 1 ;;
            "upgrade ripgrep") return 0 ;;
            *) return 1 ;;
        esac
    }
    dpkg-query() {
        case "$*" in
            "-S $apt_root/bin/jq") printf '%s\n' "jq: $apt_root/bin/jq" ;;
            "-W -f=\${Version} jq") printf '%s\n' "$jq_version" ;;
            *) return 1 ;;
        esac
    }
    apt-cache() {
        [[ "$*" == "policy jq" ]] || return 1
        printf '%s\n' "  Installed: 1.0" "  Candidate: 2.0"
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        if [[ "$*" == "apt-get install -y --only-upgrade jq" ]]; then
            jq_version="2.0"
        fi
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
            "list --formula git") printf '%s\n' "$brew_prefix/bin/git"; return 0 ;;
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

case_brew_formula_without_file_ownership_blocks() {
    local root="$TMP_ROOT/brew-file-owner" brew_prefix="$TMP_ROOT/brew-file-owner/homebrew"
    mkdir -p "$brew_prefix/bin" "$brew_prefix/Cellar/ripgrep/15.1.0/bin"
    mktool "$brew_prefix/bin/rg"
    mktool "$brew_prefix/Cellar/ripgrep/15.1.0/bin/other"
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
        case "$*" in
            "list --formula ripgrep") printf '%s\n' "$brew_prefix/Cellar/ripgrep/15.1.0/bin/other"; return 0 ;;
            *) return 1 ;;
        esac
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "Brew-prefix rg without formula file ownership did not fail closed"
    fi
    assert_contains "blocked   rg" "$root.err" "Brew file-ownership mismatch was not blocked"
    assert_contains "source-under-brew-prefix-but-formula-does-not-own-source" "$root.err" "Brew file-ownership blocked reason was not precise"
    assert_not_contains "brew upgrade ripgrep" "$COMMAND_LOG" "Brew file-ownership mismatch attempted an upgrade"
}

case_brew_prefix_symlink_to_external_blocks() {
    local root="$TMP_ROOT/brew-prefix-escape" brew_prefix="$TMP_ROOT/brew-prefix-escape/homebrew" external="$TMP_ROOT/brew-prefix-escape/external"
    mkdir -p "$brew_prefix/bin" "$external"
    mktool "$external/rg"
    ln -s "$external/rg" "$brew_prefix/bin/rg"
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
        case "$*" in
            "list --formula ripgrep") printf '%s\n' "$brew_prefix/bin/rg"; return 0 ;;
            "outdated --formula --quiet ripgrep") printf '%s\n' "ripgrep"; return 1 ;;
            "upgrade ripgrep") return 0 ;;
            *) return 1 ;;
        esac
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "Brew-prefix rg symlinked outside the prefix did not fail closed"
    fi
    assert_contains "blocked   rg" "$root.err" "Brew prefix symlink escape was not blocked"
    assert_contains "source-under-brew-prefix-but-resolved-source-outside-prefix" "$root.err" "Brew prefix symlink escape reason was not precise"
    assert_not_contains "brew upgrade ripgrep" "$COMMAND_LOG" "Brew prefix symlink escape attempted an upgrade"
}

case_brew_external_symlink_to_prefix_is_unmanaged() {
    local root="$TMP_ROOT/brew-external-symlink" brew_prefix="$TMP_ROOT/brew-external-symlink/homebrew" shadow="$TMP_ROOT/brew-external-symlink/shadow"
    mkdir -p "$brew_prefix/bin" "$brew_prefix/Cellar/ripgrep/15.1.0/bin" "$shadow/bin"
    mktool "$brew_prefix/Cellar/ripgrep/15.1.0/bin/rg"
    ln -s "$brew_prefix/Cellar/ripgrep/15.1.0/bin/rg" "$shadow/bin/rg"
    PATH="$shadow/bin:/usr/bin:/bin"
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
        case "$*" in
            "list --formula ripgrep") printf '%s\n' "$brew_prefix/Cellar/ripgrep/15.1.0/bin/rg"; return 0 ;;
            "outdated --formula --quiet ripgrep") printf '%s\n' "ripgrep"; return 1 ;;
            "upgrade ripgrep") return 0 ;;
            *) return 1 ;;
        esac
    }

    update_catalog_tools > "$root.out"

    assert_contains "unmanaged rg" "$root.out" "external symlink to Brew rg was not unmanaged"
    assert_contains "source=$shadow/bin/rg" "$root.out" "external symlink did not report the PATH source"
    assert_not_contains "brew upgrade ripgrep" "$COMMAND_LOG" "external symlink to Brew rg attempted a Brew upgrade"
}

case_brew_outdated_probe_failure_blocks() {
    local root="$TMP_ROOT/brew-probe-fail" brew_prefix="$TMP_ROOT/brew-probe-fail/homebrew"
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
        case "$*" in
            "list --formula ripgrep") printf '%s\n' "$brew_prefix/bin/rg"; return 0 ;;
            "outdated --formula --quiet ripgrep") return 42 ;;
            *) return 1 ;;
        esac
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "Brew outdated probe failure did not fail closed"
    fi
    assert_contains "blocked   rg" "$root.err" "Brew outdated probe failure was not blocked"
    assert_contains "reason=outdated-check-failed" "$root.err" "Brew outdated probe failure reason was not precise"
    assert_not_contains "current   rg" "$root.out" "Brew outdated probe failure was reported current"
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

case_apt_advertised_upgrade_without_version_change_blocks() {
    local root="$TMP_ROOT/apt-no-change" apt_root="$TMP_ROOT/apt-no-change/usr"
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
        printf '%s\n' "  Installed: 1.0" "  Candidate: 2.0"
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        return 0
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "apt advertised upgrade without version change did not fail closed"
    fi
    assert_contains "apt-get update -qq" "$COMMAND_LOG" "apt stale-version case did not refresh metadata"
    assert_contains "apt-get install -y --only-upgrade jq" "$COMMAND_LOG" "apt stale-version case did not attempt scoped upgrade"
    assert_contains "blocked   jq" "$root.err" "apt stale-version case was not blocked"
    assert_contains "reason=post-upgrade-version-unchanged" "$root.err" "apt stale-version blocked reason was not precise"
    assert_not_contains "updated   jq" "$root.out" "apt stale-version case was falsely reported updated"
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

case_dnf_rpm_owner_updates() {
    local root="$TMP_ROOT/dnf-update" rpm_root="$TMP_ROOT/dnf-update/usr"
    mkdir -p "$rpm_root/bin"
    mktool "$rpm_root/bin/cmake"
    PATH="$rpm_root/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=dnf
    INSTALL_DEPS_UPDATE_TOOLS="cmake"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "dnf"; }
    rpm() {
        local query=""
        for query in "$@"; do :; done
        [[ "$query" == "$rpm_root/bin/cmake" ]] || return 1
        printf '%s\n' "cmake"
    }
    dnf() {
        printf 'dnf %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "check-update --quiet cmake") return 100 ;;
            *) return 1 ;;
        esac
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        return 0
    }

    update_catalog_tools > "$root.out"
    assert_contains "updated   cmake" "$root.out" "dnf-owned cmake was not reported updated"
    assert_contains "owner=dnf package=cmake source=$rpm_root/bin/cmake" "$root.out" "dnf output did not include source proof"
    assert_contains "dnf upgrade -y cmake" "$COMMAND_LOG" "dnf-owned cmake did not use scoped dnf upgrade"
}

case_zypper_rpm_owner_updates() {
    local root="$TMP_ROOT/zypper-update" rpm_root="$TMP_ROOT/zypper-update/usr"
    mkdir -p "$rpm_root/bin"
    mktool "$rpm_root/bin/cmake"
    PATH="$rpm_root/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=zypper
    INSTALL_DEPS_UPDATE_TOOLS="cmake"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "zypper"; }
    rpm() {
        local query=""
        for query in "$@"; do :; done
        [[ "$query" == "$rpm_root/bin/cmake" ]] || return 1
        printf '%s\n' "cmake"
    }
    zypper() {
        printf 'zypper %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "--non-interactive list-updates -t package cmake")
                printf '%s\n' "v | repo | cmake | 4.3.4"
                return 0
                ;;
            *) return 1 ;;
        esac
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        return 0
    }

    update_catalog_tools > "$root.out"
    assert_contains "updated   cmake" "$root.out" "zypper-owned cmake was not reported updated"
    assert_contains "owner=zypper package=cmake source=$rpm_root/bin/cmake" "$root.out" "zypper output did not include source proof"
    assert_contains "zypper update -y cmake" "$COMMAND_LOG" "zypper-owned cmake did not use scoped zypper update"
}

case_zypper_outdated_probe_failure_blocks() {
    local root="$TMP_ROOT/zypper-probe-fail" rpm_root="$TMP_ROOT/zypper-probe-fail/usr"
    mkdir -p "$rpm_root/bin"
    mktool "$rpm_root/bin/cmake"
    PATH="$rpm_root/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=zypper
    INSTALL_DEPS_UPDATE_TOOLS="cmake"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "zypper"; }
    rpm() {
        local query=""
        for query in "$@"; do :; done
        [[ "$query" == "$rpm_root/bin/cmake" ]] || return 1
        printf '%s\n' "cmake"
    }
    zypper() {
        printf 'zypper %s\n' "$*" >> "$COMMAND_LOG"
        return 7
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "zypper outdated probe failure did not fail closed"
    fi
    assert_contains "blocked   cmake" "$root.err" "zypper probe failure was not blocked"
    assert_contains "reason=outdated-check-failed" "$root.err" "zypper probe failure reason was not precise"
    assert_not_contains "current   cmake" "$root.out" "zypper probe failure was reported current"
}

case_apk_owner_updates() {
    local root="$TMP_ROOT/apk-update" apk_root="$TMP_ROOT/apk-update/usr"
    mkdir -p "$apk_root/bin"
    mktool "$apk_root/bin/lsd"
    PATH="$apk_root/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=apk
    INSTALL_DEPS_UPDATE_TOOLS="lsd"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "apk"; }
    apk() {
        printf 'apk %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "info --who-owns $apk_root/bin/lsd")
                printf '%s\n' "$apk_root/bin/lsd is owned by lsd-1.2.0-r0"
                return 0
                ;;
            "version -l < lsd")
                printf '%s\n' "lsd-1.2.0-r0 < 1.2.1-r0"
                return 0
                ;;
            *) return 1 ;;
        esac
    }
    maybe_sudo() {
        printf '%s\n' "$*" >> "$COMMAND_LOG"
        return 0
    }

    update_catalog_tools > "$root.out"
    assert_contains "updated   lsd" "$root.out" "apk-owned lsd was not reported updated"
    assert_contains "owner=apk package=lsd source=$apk_root/bin/lsd" "$root.out" "apk output did not include source proof"
    assert_contains "apk upgrade lsd" "$COMMAND_LOG" "apk-owned lsd did not use scoped apk upgrade"
}

case_apk_outdated_probe_failure_blocks() {
    local root="$TMP_ROOT/apk-probe-fail" apk_root="$TMP_ROOT/apk-probe-fail/usr"
    mkdir -p "$apk_root/bin"
    mktool "$apk_root/bin/lsd"
    PATH="$apk_root/bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=apk
    INSTALL_DEPS_UPDATE_TOOLS="lsd"

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "apk"; }
    apk() {
        printf 'apk %s\n' "$*" >> "$COMMAND_LOG"
        case "$*" in
            "info --who-owns $apk_root/bin/lsd")
                printf '%s\n' "$apk_root/bin/lsd is owned by lsd-1.2.0-r0"
                return 0
                ;;
            "version -l < lsd") return 9 ;;
            *) return 1 ;;
        esac
    }

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "apk outdated probe failure did not fail closed"
    fi
    assert_contains "blocked   lsd" "$root.err" "apk probe failure was not blocked"
    assert_contains "reason=outdated-check-failed" "$root.err" "apk probe failure reason was not precise"
    assert_not_contains "current   lsd" "$root.out" "apk probe failure was reported current"
}

case_direct_artifact_current() {
    local root="$TMP_ROOT/direct-current" bin="$TMP_ROOT/direct-current/home/.local/bin"
    mkdir -p "$bin"
    HOME="$TMP_ROOT/direct-current/home"
    export HOME
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$TMP_ROOT/direct-current/provenance"
    export DOTFILES_PROVENANCE_DIR
    INSTALL_DEPS_UPDATE_TOOLS="lazygit"
    mkversioned_tool "$bin/lazygit" "lazygit ${LAZYGIT_LINUX_VERSION#v}"

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

case_direct_artifact_command_path_mismatch_blocks() {
    local root="$TMP_ROOT/direct-command-path-blocked" home="$TMP_ROOT/direct-command-path-blocked/home" canonical="$TMP_ROOT/direct-command-path-blocked/home/.local/bin" shadow="$TMP_ROOT/direct-command-path-blocked/shadow" marker_dir="$TMP_ROOT/direct-command-path-blocked/provenance"
    mkdir -p "$canonical" "$shadow" "$marker_dir"
    HOME="$home"
    export HOME
    mkversioned_tool "$canonical/lazygit" "lazygit ${LAZYGIT_LINUX_VERSION#v}"
    ln -s "$canonical/lazygit" "$shadow/lazygit"
    PATH="$shadow:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
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
    write_direct_artifact_provenance "lazygit" "$canonical/lazygit" "$canonical/lazygit" "$canonical" "$DIRECT_ARTIFACT_URL" "$DIRECT_ARTIFACT_VERSION" "$DIRECT_ARTIFACT_SHA256"

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "direct artifact shadow command path did not fail closed"
    fi
    assert_contains "blocked   lazygit" "$root.err" "direct artifact command-path mismatch was not blocked"
    assert_contains "reason=command source does not match marker command path" "$root.err" "direct artifact command-path mismatch reason was not precise"
    assert_not_contains "current   lazygit" "$root.out" "direct artifact command-path mismatch was reported current"
}

case_direct_artifact_binary_outside_install_root_blocks() {
    local root="$TMP_ROOT/direct-root-blocked" bin="$TMP_ROOT/direct-root-blocked/bin" marker_dir="$TMP_ROOT/direct-root-blocked/provenance"
    local binary_sha256
    mkdir -p "$bin" "$marker_dir"
    HOME="$TMP_ROOT/direct-root-blocked/home"
    export HOME
    mkversioned_tool "$bin/lazygit" "lazygit ${LAZYGIT_LINUX_VERSION#v}"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
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
    binary_sha256="$(sha256_file "$bin/lazygit")"
    printf '%s\n' \
        "schema=2" \
        "tool=lazygit" \
        "version=$DIRECT_ARTIFACT_VERSION" \
        "source_url=$DIRECT_ARTIFACT_URL" \
        "sha256=$DIRECT_ARTIFACT_SHA256" \
        "binary_sha256=$binary_sha256" \
        "command_path=$bin/lazygit" \
        "binary_path=$bin/lazygit" \
        "install_root=$TMP_ROOT/direct-root-blocked/not-root" \
        > "$marker_dir/lazygit.env"

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "direct artifact binary outside install root did not fail closed"
    fi
    assert_contains "blocked   lazygit" "$root.err" "direct artifact install-root mismatch was not blocked"
    assert_contains "reason=marker binary is outside install root" "$root.err" "direct artifact install-root mismatch reason was not precise"
    assert_not_contains "current   lazygit" "$root.out" "direct artifact install-root mismatch was reported current"
}

case_direct_artifact_unsupported_install_shape_blocks() {
    local root="$TMP_ROOT/direct-shape-blocked" alien="$TMP_ROOT/direct-shape-blocked/alien/bin" marker_dir="$TMP_ROOT/direct-shape-blocked/provenance"
    local binary_sha256
    mkdir -p "$alien" "$marker_dir"
    HOME="$TMP_ROOT/direct-shape-blocked/home"
    export HOME
    mkversioned_tool "$alien/starship" "starship ${STARSHIP_VERSION#v}"
    PATH="$alien:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
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
    direct_artifact_current_metadata starship
    binary_sha256="$(sha256_file "$alien/starship")"
    printf '%s\n' \
        "schema=2" \
        "tool=starship" \
        "version=$DIRECT_ARTIFACT_VERSION" \
        "source_url=$DIRECT_ARTIFACT_URL" \
        "sha256=$DIRECT_ARTIFACT_SHA256" \
        "binary_sha256=$binary_sha256" \
        "command_path=$alien/starship" \
        "binary_path=$alien/starship" \
        "install_root=$alien" \
        > "$marker_dir/starship.env"

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "direct artifact marker under unsupported install shape did not fail closed"
    fi
    assert_contains "blocked   starship" "$root.err" "unsupported direct artifact install shape was not blocked"
    assert_contains "reason=path does not match repo-pinned install shape" "$root.err" "unsupported direct artifact install-shape reason was not precise"
    assert_not_contains "current   starship" "$root.out" "unsupported direct artifact install shape was reported current"
}

case_direct_artifact_writer_rejects_binary_outside_install_root() {
    local root="$TMP_ROOT/direct-writer-root-blocked" bin="$TMP_ROOT/direct-writer-root-blocked/home/.local/bin" marker_dir="$TMP_ROOT/direct-writer-root-blocked/provenance"
    mkdir -p "$bin" "$marker_dir"
    HOME="$TMP_ROOT/direct-writer-root-blocked/home"
    export HOME
    mkversioned_tool "$bin/lazygit" "lazygit ${LAZYGIT_LINUX_VERSION#v}"
    DOTFILES_PROVENANCE_DIR="$marker_dir"
    export DOTFILES_PROVENANCE_DIR

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    direct_artifact_current_metadata lazygit
    if write_direct_artifact_provenance "lazygit" "$bin/lazygit" "$bin/lazygit" "$TMP_ROOT/direct-writer-root-blocked/not-root" "$DIRECT_ARTIFACT_URL" "$DIRECT_ARTIFACT_VERSION" "$DIRECT_ARTIFACT_SHA256" > "$root.out" 2> "$root.err"; then
        fail "direct artifact writer accepted a binary outside install root"
    fi
    assert_contains "direct artifact binary for lazygit is outside install root" "$root.err" "direct artifact writer did not explain invalid root"
    [[ ! -e "$marker_dir/lazygit.env" ]] || fail "direct artifact writer created a marker after rejecting invalid root"
}

case_direct_artifact_writer_rejects_unsupported_install_shape() {
    local root="$TMP_ROOT/direct-writer-shape-blocked" alien="$TMP_ROOT/direct-writer-shape-blocked/alien/bin" marker_dir="$TMP_ROOT/direct-writer-shape-blocked/provenance"
    mkdir -p "$alien" "$marker_dir"
    HOME="$TMP_ROOT/direct-writer-shape-blocked/home"
    export HOME
    mkversioned_tool "$alien/starship" "starship ${STARSHIP_VERSION#v}"
    DOTFILES_PROVENANCE_DIR="$marker_dir"
    export DOTFILES_PROVENANCE_DIR

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    direct_artifact_current_metadata starship
    if write_direct_artifact_provenance "starship" "$alien/starship" "$alien/starship" "$alien" "$DIRECT_ARTIFACT_URL" "$DIRECT_ARTIFACT_VERSION" "$DIRECT_ARTIFACT_SHA256" > "$root.out" 2> "$root.err"; then
        fail "direct artifact writer accepted an unsupported install shape"
    fi
    assert_contains "direct artifact install shape for starship is not repo-managed" "$root.err" "direct artifact writer did not explain unsupported install shape"
    [[ ! -e "$marker_dir/starship.env" ]] || fail "direct artifact writer created a marker after rejecting unsupported shape"
}

case_direct_artifact_checksum_mismatch_blocks() {
    local root="$TMP_ROOT/direct-checksum-blocked" bin="$TMP_ROOT/direct-checksum-blocked/home/.local/bin" marker_dir="$TMP_ROOT/direct-checksum-blocked/provenance"
    mkdir -p "$bin" "$marker_dir"
    HOME="$TMP_ROOT/direct-checksum-blocked/home"
    export HOME
    mkversioned_tool "$bin/lazygit" "lazygit ${LAZYGIT_LINUX_VERSION#v}"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
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
    printf '%s\n' "#!/usr/bin/env bash" "printf '%s\n' 'lazygit ${LAZYGIT_LINUX_VERSION#v}'" > "$bin/lazygit"
    chmod +x "$bin/lazygit"

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "direct artifact checksum mismatch did not fail closed"
    fi
    assert_contains "blocked   lazygit" "$root.err" "direct artifact checksum mismatch was not blocked"
    assert_contains "reason=marker binary checksum mismatch" "$root.err" "direct artifact checksum mismatch reason was not precise"
}

case_direct_artifact_version_mismatch_blocks() {
    local root="$TMP_ROOT/direct-version-blocked" bin="$TMP_ROOT/direct-version-blocked/home/.local/bin" marker_dir="$TMP_ROOT/direct-version-blocked/provenance"
    mkdir -p "$bin" "$marker_dir"
    HOME="$TMP_ROOT/direct-version-blocked/home"
    export HOME
    mktool "$bin/lazygit"
    PATH="$bin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=unknown
    DOTFILES_PROVENANCE_DIR="$marker_dir"
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

    if update_catalog_tools > "$root.out" 2> "$root.err"; then
        fail "direct artifact version mismatch did not fail closed"
    fi
    assert_contains "blocked   lazygit" "$root.err" "direct artifact version mismatch was not blocked"
    assert_contains "reason=binary version does not match repo pin" "$root.err" "direct artifact version mismatch reason was not precise"
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

    uname() {
        case "${1:-}" in
            -s) printf '%s\n' "Linux" ;;
            -m) printf '%s\n' "x86_64" ;;
            *) command uname "$@" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }
    mkversioned_tool "$bin/lazygit" "lazygit ${LAZYGIT_LINUX_VERSION#v}"
    direct_artifact_current_metadata lazygit
    write_direct_artifact_provenance "lazygit" "$bin/lazygit" "$bin/lazygit" "$bin" "$DIRECT_ARTIFACT_URL" "v0.0.0" "$DIRECT_ARTIFACT_SHA256"

    refresh_direct_artifact() {
        printf '%s\n' "refresh $1" >> "$COMMAND_LOG"
        direct_artifact_current_metadata "$1"
        mkversioned_tool "$bin/$1" "$1 ${DIRECT_ARTIFACT_VERSION#v}"
        write_direct_artifact_provenance "$1" "$bin/$1" "$bin/$1" "$bin" "$DIRECT_ARTIFACT_URL" "$DIRECT_ARTIFACT_VERSION" "$DIRECT_ARTIFACT_SHA256"
    }

    update_catalog_tools > "$root.out"
    assert_contains "refresh lazygit" "$COMMAND_LOG" "stale direct artifact did not invoke refresh"
    assert_contains "updated   lazygit" "$root.out" "stale direct artifact was not reported updated"
}

case_nix_owned_tool_reports_owner_nix() {
    # A tool whose command source resolves under a Nix store/profile is
    # owner=nix and is NOT updated (refreshed via the opt-in Nix layer). Covers a
    # ~/.nix-profile source (matched directly) AND a plain source whose realpath
    # resolves into /nix/store. Proves update mode never shells out to `nix`.
    local root="$TMP_ROOT/nix-owned"
    local profbin="$root/home/.nix-profile/bin"
    local wrapbin="$root/wrap/bin"
    mkdir -p "$profbin" "$wrapbin"
    mktool "$profbin/rg"
    mktool "$wrapbin/jq"
    PATH="$profbin:$wrapbin:/usr/bin:/bin"
    export PATH
    DRY_RUN=0
    PM=brew
    INSTALL_DEPS_UPDATE_TOOLS="rg
jq"

    real_source_path() {
        case "$1" in
            "$wrapbin/jq") printf '%s\n' "/nix/store/abc123-jq-1.7.1/bin/jq" ;;
            *) printf '%s\n' "$1" ;;
        esac
    }
    homebrew_bin() { return 1; }
    native_linux_pm() { printf '%s\n' "unknown"; }
    # If update mode ever shells out to nix for an owned tool, record it.
    nix() { printf 'NIX_INVOKED %s\n' "$*" >> "$root.out"; return 0; }

    # Direct path-form unit checks of the resolver.
    local p
    for p in \
        "/nix/store/xxx/bin/rg" \
        "/home/u/.nix-profile/bin/rg" \
        "/Users/u/.nix-profile/bin/rg" \
        "/etc/profiles/per-user/u/bin/rg" \
        "/run/current-system/sw/bin/rg" \
        "/home/u/.local/state/nix/profile/bin/rg"; do
        nix_owns_tool_source "$p" || fail "nix_owns_tool_source should own $p"
    done
    for p in "/opt/homebrew/bin/rg" "/usr/bin/rg" "/home/u/.local/bin/rg" ""; do
        if nix_owns_tool_source "$p"; then fail "nix_owns_tool_source wrongly owned '$p'"; fi
    done

    update_catalog_tools > "$root.out"
    assert_contains "owner=nix reason=managed by the Nix layer" "$root.out" "nix-owned rg did not report the owner=nix reason"
    grep -Eq "skipped[[:space:]]+rg[[:space:]].*owner=nix" "$root.out" || fail "nix-profile rg was not skipped as owner=nix"
    grep -Eq "skipped[[:space:]]+jq[[:space:]].*owner=nix" "$root.out" || fail "/nix/store jq was not skipped as owner=nix"
    assert_not_contains "NIX_INVOKED" "$root.out" "update mode shelled out to nix for a nix-owned tool"
    assert_not_contains "owner=brew" "$root.out" "nix-owned tool was mis-claimed by brew"
}

run_case "nix-owned tool reports owner=nix" case_nix_owned_tool_reports_owner_nix
run_case "mixed Linuxbrew and apt" case_mixed_linuxbrew_and_apt
run_case "Brew current" case_brew_current_does_not_upgrade
run_case "Brew shadowed source" case_brew_shadowed_source_is_unmanaged
run_case "Brew prefix without formula" case_brew_prefix_without_formula_blocks
run_case "Brew formula without file ownership" case_brew_formula_without_file_ownership_blocks
run_case "Brew prefix symlink escape" case_brew_prefix_symlink_to_external_blocks
run_case "Brew external symlink to prefix" case_brew_external_symlink_to_prefix_is_unmanaged
run_case "Brew outdated probe failure" case_brew_outdated_probe_failure_blocks
run_case "macOS system zsh" case_macos_system_zsh_is_accepted
run_case "apt current" case_apt_current_skips_scoped_install
run_case "apt advertised upgrade without version change" case_apt_advertised_upgrade_without_version_change_blocks
run_case "pacman explicit skip" case_pacman_owner_is_explicitly_skipped
run_case "dnf rpm owner update" case_dnf_rpm_owner_updates
run_case "zypper rpm owner update" case_zypper_rpm_owner_updates
run_case "zypper probe failure" case_zypper_outdated_probe_failure_blocks
run_case "apk owner update" case_apk_owner_updates
run_case "apk probe failure" case_apk_outdated_probe_failure_blocks
run_case "direct artifact current" case_direct_artifact_current
run_case "direct artifact command path mismatch" case_direct_artifact_command_path_mismatch_blocks
run_case "direct artifact install root mismatch" case_direct_artifact_binary_outside_install_root_blocks
run_case "direct artifact unsupported install shape" case_direct_artifact_unsupported_install_shape_blocks
run_case "direct artifact writer install root guard" case_direct_artifact_writer_rejects_binary_outside_install_root
run_case "direct artifact writer install shape guard" case_direct_artifact_writer_rejects_unsupported_install_shape
run_case "direct artifact checksum mismatch" case_direct_artifact_checksum_mismatch_blocks
run_case "direct artifact version mismatch" case_direct_artifact_version_mismatch_blocks
run_case "direct artifact unmarked" case_direct_artifact_legacy_unmarked_is_unmanaged
run_case "direct artifact corrupt marker" case_direct_artifact_corrupt_marker_blocks
run_case "direct artifact refresh" case_direct_artifact_stale_marker_refreshes_pin

echo "OK"
