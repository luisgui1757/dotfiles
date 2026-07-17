#!/usr/bin/env bash
# install-deps.sh -- interactively install dependencies on macOS / Linux / WSL.
#
# Designed to work on STOCK BASH 3.2 (macOS default) -- no associative
# arrays, no mapfile, no namerefs, no ${var,,}. Tested under bash 3.2.57
# and bash 5.2.
#
# Usage:
#   ./install-deps.sh           prompt Y/n for each tool
#   ./install-deps.sh --all     skip prompts, install everything
#   ./install-deps.sh --update  update present manager-owned tools and proven dotfiles artifacts
#   ./install-deps.sh --dry-run print what would be installed without acting
#   ./install-deps.sh --experimental-wsl-gui
#                              WSL opt-in: Linux Ghostty + Linux fontconfig fonts

set -euo pipefail

YES_ALL=0
DRY_RUN=0
UPDATE_ONLY=0
EXPERIMENTAL_WSL_GUI="${DOTFILES_EXPERIMENTAL_WSL_GUI:-0}"
HOMEBREW_INSTALL_COMMIT="3f6f98f9856618dbe216c4a50d42ff5a8b3b86c8"
HOMEBREW_INSTALL_SHA256="99287f194a8b3c9e6b0203a11a5fa54518be57209343e6bb954dec4635796d9d"
NVIM_LINUX_VERSION="v0.12.4"
NVIM_LINUX_X86_64_SHA256="012bf3fcac5ade43914df3f174668bf64d05e049a4f032a388c027b1ebd78628"
NVIM_LINUX_ARM64_SHA256="ceb7e88c6b681f0515d135dcdfad54f5eb4373b25ce6172197cd9a69c758063f"
CHEZMOI_VERSION="v2.71.0"
CHEZMOI_LINUX_X86_64_SHA256="6ea2040ecc0e82d3dac604289e100b0157afefcd94ebb818e5f6e31655156d34"
CHEZMOI_LINUX_ARM64_SHA256="d8fb35f9d43237b4f6d022cad40e1094957b990cfaee5f3b131ded65422b0983"
LAZYGIT_LINUX_VERSION="v0.63.0"
LAZYGIT_LINUX_X86_64_SHA256="cf5cfa3e116d7775f3600a51ec1d9ce7ba554a08b9566c7c2da83cb0023efabf"
LAZYGIT_LINUX_ARM64_SHA256="aac147abf5ce43afe6ae8bcb14b0d479111975a189302d7a99386deca70d57f7"
STARSHIP_VERSION="v1.26.0"
STARSHIP_LINUX_X86_64_SHA256="321f0dd7af8340a5f2e6a8fec6538a04f617486f9ec70d878f91c09cd8deef22"
STARSHIP_LINUX_ARM64_SHA256="dc30189378d2f2e287384e8a692d3f95ad1df64cf0e8c36aa9201516028aed6b"
TREE_SITTER_CLI_LINUX_VERSION="v0.26.10"
TREE_SITTER_CLI_LINUX_X86_64_SHA256="5aca1172aae08050d0d1184046377d850c04065205185ebafde361afff8d9f62"
TREE_SITTER_CLI_LINUX_ARM64_SHA256="6a455e6c0c21ddb732d182e3c46e3a8ca1121718254ce684a9dc730ff2367e02"
FZF_TAB_VERSION="v1.3.0"
FZF_TAB_COMMIT="d7e0234614dbe5369fdd760907d12c0e05a4dccc"
ZSH_AUTOSUGGESTIONS_VERSION="v0.7.1"
ZSH_AUTOSUGGESTIONS_COMMIT="e52ee8ca55bcc56a17c828767a3f98f22a68d4eb"
GH_DASH_VERSION="v4.25.1"   # dlvhdr/gh-dash pinned gh-extension tag; mirror in install-deps.ps1 ($GhDashVersion)
GH_DASH_TAG_OBJECT="e6ebbd7e83e30161b9192ce3339972d2c8269e7f"
GH_DASH_COMMIT="49f37e4832956c57bf52d4ea8b1b1e5c0f863700"
PI_CLI_PACKAGE="@earendil-works/pi-coding-agent"
PI_CLI_VERSION="0.80.9"
PI_CLI_INTEGRITY="sha512-Clgx2Bg5NbMcCpGxusSDQwE+GC0g/d6sCBluE9aypPgSgtJ6n8VmZIIT6auXObMskpRgkr+XZ77wG5hf+cSDtg=="
TPM_COMMIT="e261deb1b47614eed3400089ce7197dc68acc4eb"
# Functional tmux plugins (Omer-style set). The Rose Pine status bar is NOT a
# plugin here -- it is a repo-owned generated config (tmux/psmux-rose-pine.ps1),
# so rose-pine/tmux is no longer installed.
TMUX_SENSIBLE_COMMIT="25cb91f42d020f675bb0a2ce3fbd3a5d96119efa"
TMUX_YANK_COMMIT="acfd36e4fcba99f8310a7dfb432111c242fe7392"
TMUX_RESURRECT_COMMIT="cff343cf9e81983d3da0c8562b01616f12e8d548"
TMUX_CONTINUUM_COMMIT="0698e8f4b17d6454c71bf5212895ec055c578da0"
HACK_NERD_FONT_VERSION="v3.4.0"
HACK_NERD_FONT_SHA256="8ca33a60c791392d872b80d26c42f2bfa914a480f9eb2d7516d9f84373c36897"
# Ghostty on Debian-family hosts: install only exact mkasberg/ghostty-ubuntu
# release assets whose bytes and package metadata are reviewed here. The
# upstream install.sh queries mutable releases/latest and downloads an
# unchecked .deb, so it is deliberately not executed. Bump the version and all
# applicable architecture/distro hashes together.
GHOSTTY_UBUNTU_VERSION="1.3.1-0-ppa2"
GHOSTTY_UBUNTU_AMD64_2404_SHA256="478d440153ef544426418efc7d6d8901715359f452c46be29071901a94b8cd47"
GHOSTTY_UBUNTU_ARM64_2404_SHA256="91063815b6ce3d834d59714b4ad0310f744448b6716836d035b3d331d1923363"
GHOSTTY_UBUNTU_AMD64_2510_SHA256="793bde1c31163d8e1d12ea939c8b941f7908170e57bbf19b121434a0f6621c59"
GHOSTTY_UBUNTU_ARM64_2510_SHA256="c6a4fd4fd786b4bdea42036650ef1724f535c4b636329f488f7ece36820d3d6b"
GHOSTTY_DEBIAN_AMD64_TRIXIE_SHA256="9fda8e418d7a7f58149ba3ba823a255d6b80f8bb5431b3bd7e912ff597715b2e"
GHOSTTY_DEBIAN_ARM64_TRIXIE_SHA256="73f384e62c419d7a7809d686bf579fea5e23f52742b34f70c74d6adf0e72f8ab"
GHOSTTY_UBUNTU_ASSET_VERSION="${GHOSTTY_UBUNTU_VERSION/-0-/-0.}"
GHOSTTY_UBUNTU_PACKAGE_VERSION="${GHOSTTY_UBUNTU_VERSION/-0-/-0~}"
# WezTerm on Ubuntu (amd64): pin + SHA-256 verify the OFFICIAL .deb from the
# stable release. macOS uses the Homebrew cask; Windows uses install-deps.ps1's
# catalog. arm64 Linux and non-Ubuntu hosts get manual guidance -- upstream did
# not publish an arm64 .sha256 sidecar for this stable tag, so we do not pin a
# checksum we cannot verify. The amd64 SHA below was verified against upstream's
# published `wezterm-...Ubuntu22.04.deb.sha256` sidecar on 2026-07-07. Bump the
# version + SHA together. (Windows installs WezTerm from the install-deps.ps1
# catalog via winget/scoop/choco, so there is no Windows version pin to mirror.)
WEZTERM_VERSION="20240203-110809-5046fc22"
WEZTERM_DEB_AMD64_SHA256="86358dab5794a4fb63f7c91dd68d4fdc3da58faad648a58fc77d2bd51c7b0686"
# Herdr (agent multiplexer). macOS + Linuxbrew use the canonical homebrew-core
# formula (`brew install herdr`). Native Linux without brew installs the pinned
# release binary, SHA-256 verified. Upstream publishes no checksum sidecar, so
# these SHAs were computed from the pinned v0.7.4 assets on 2026-07-16 (bump the
# version + both SHAs together). NOT the herdr.dev install.sh remote-eval path.
# Native Windows uses install-deps.ps1's separate pinned, SHA-256-verified
# preview .exe path, never the herdr.dev install.ps1 remote-eval path.
HERDR_VERSION="v0.7.4"
HERDR_LINUX_X86_64_SHA256="bc0fc02d4ba500f9cac2353a43e67fe036785ecca6eb55378e050fac3c103059"
HERDR_LINUX_ARM64_SHA256="544e0002de42806d1ab64ccdef3a7e7414f24717b0b6b022bc9e57d2eefd26a2"
PYLATEXENC_BUILD_BACKEND_VERSION="80.9.0"
PYLATEXENC_BUILD_BACKEND_SHA256="062d34222ad13e0cc312a4c02d73f059e86a4acbfbdea8f8f76b28c99f306922"
PYLATEXENC_VERSION="2.10"
PYLATEXENC_SHA256="3dd8fd84eb46dc30bee1e23eaab8d8fb5a7f507347b23e5f38ad9675c84f40d3"
for arg in "$@"; do
    case "$arg" in
        --all|-y)   YES_ALL=1 ;;
        --dry-run)  DRY_RUN=1 ;;
        --update)   UPDATE_ONLY=1 ;;
        --experimental-wsl-gui)
                    EXPERIMENTAL_WSL_GUI=1
                    export DOTFILES_EXPERIMENTAL_WSL_GUI=1 ;;
        -h|--help)
            sed -n '2,14p' "$0"; exit 0 ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done
[[ "$EXPERIMENTAL_WSL_GUI" == "1" ]] || EXPERIMENTAL_WSL_GUI=0

# This installer owns consent for every package mutation. Homebrew 6 enables
# ask mode by default for install/upgrade, so letting an accepted setup action
# reach brew unchanged creates a second confirmation and can hang --all runs.
# Scope the official no-ask setting to this child process; ordinary user brew
# commands outside setup keep their normal behavior. An inherited explicit ask
# must be cleared because it takes precedence over HOMEBREW_NO_ASK.
unset HOMEBREW_ASK
export HOMEBREW_NO_ASK=1

INSTALL_FAILURES_COUNT=0
INSTALL_FAILURES_DETAIL=""
MANAGED_CLI_AUDITED="|"

# ---- Bash 3.2-safe helpers ---------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }

record_install_failure() {
    local tool="$1" manager="$2" pkg="$3" rc="${4:-1}"
    [[ "$DRY_RUN" -eq 1 ]] && return 0
    INSTALL_FAILURES_COUNT=$((INSTALL_FAILURES_COUNT + 1))
    INSTALL_FAILURES_DETAIL="${INSTALL_FAILURES_DETAIL}  FAIL: ${tool} via ${manager} (${pkg}) exit=${rc}"$'\n'
}

# Run one recoverable install step without letting `set -e` bypass the final
# consolidated summary. A callee may already record a more precise failure; in
# that case the before/after count prevents a duplicate entry. Preconditions
# that make the whole installer unsafe (unsupported package manager, invalid
# invocation) remain explicit immediate exits outside this wrapper.
run_install_step() {
    local tool="$1" manager="$2" pkg="$3"
    shift 3
    local before="$INSTALL_FAILURES_COUNT" rc=0
    "$@" || rc=$?
    if [[ "$rc" -ne 0 ]]; then
        if [[ "$INSTALL_FAILURES_COUNT" -eq "$before" ]]; then
            record_install_failure "$tool" "$manager" "$pkg" "$rc"
        fi
        printf "  FAIL: %-26s recoverable install step failed; continuing to collect install failures\n" "$tool" >&2
    fi
    return 0
}

run_catalog_install() {
    local tool="$1" purpose="${2:-}" pkg
    pkg="$(pkg_for "$tool")"
    run_install_step "$tool" "${PM:-unknown}" "${pkg:-unavailable}" install "$tool" "$purpose"
}

exit_if_install_failures() {
    if [[ "$INSTALL_FAILURES_COUNT" -eq 0 ]]; then
        return 0
    fi
    echo
    printf "install-deps: %s accepted install path(s) failed:\n" "$INSTALL_FAILURES_COUNT" >&2
    printf "%s" "$INSTALL_FAILURES_DETAIL" >&2
    echo "install-deps: failing so setup cannot report success after blocked dependency installs." >&2
    exit 1
}

prepend_unique_path_dir() {
    local dir="$1" entry remaining new_path="$1"
    remaining="${PATH-}:"
    while [[ "$remaining" == *:* ]]; do
        entry="${remaining%%:*}"
        remaining="${remaining#*:}"
        [[ "$entry" == "$dir" ]] && continue
        new_path="$new_path:$entry"
    done
    PATH="$new_path"
}

# Idempotently put ~/.local/bin first on PATH for THIS process (pinned
# binaries, pip --user, and chezmoi all land there). Moving an existing later
# entry prevents an older global command from shadowing the verified install.
ensure_local_bin_on_path() {
    prepend_unique_path_dir "$HOME/.local/bin"
    export PATH
    hash -r 2>/dev/null || true
}
verify_sha256() {
    local f="$1" expected="$2" got
    got="$(sha256_file "$f")" || {
        echo "  FAIL: need shasum or sha256sum to verify $f" >&2
        return 1
    }
    [[ "$got" == "$expected" ]]
}

sha256_file() {
    local f="$1"
    if have shasum; then
        shasum -a 256 "$f" | awk '{print $1}'
    elif have sha256sum; then
        sha256sum "$f" | awk '{print $1}'
    else
        return 1
    fi
}
have_any() {
    local b
    for b in "$@"; do
        if command -v "$b" >/dev/null 2>&1; then return 0; fi
    done
    return 1
}
# Some tools install under a different binary name on certain distros
# (Debian/Ubuntu ship fd-find as `fdfind`). Map tool -> space-separated list
# of binaries to accept as "installed".
binaries_for() {
    case "$1" in
        fd)       echo "fd fdfind" ;;
        wl-copy)  echo "wl-copy wl-paste" ;;
        *)        echo "$1" ;;
    esac
}
is_wsl() { grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null; }
can_show_gui() { [[ -n "${DISPLAY:-}" || -n "${WAYLAND_DISPLAY:-}" ]]; }
wsl_gui_opt_in() { [[ "${EXPERIMENTAL_WSL_GUI:-0}" == "1" ]]; }
os_release_value() {
    local key="$1"
    [[ -r /etc/os-release ]] || return 1
    awk -F= -v key="$key" '
        $1 == key {
            value = substr($0, index($0, "=") + 1)
            sub(/^"/, "", value)
            sub(/"$/, "", value)
            print value
            exit
        }
    ' /etc/os-release
}
is_ubuntu() {
    local id="" id_like=""
    if [[ -r /etc/os-release ]]; then
        id="$(os_release_value ID 2>/dev/null || true)"
        id_like="$(os_release_value ID_LIKE 2>/dev/null || true)"
    fi
    [[ "$id" == "ubuntu" ]] && return 0
    case " $id_like " in
        *" ubuntu "*|*" debian "*) return 0 ;;
        *) return 1 ;;
    esac
}

native_linux_pm() {
    [[ "$(uname -s)" == "Linux" ]] || { echo unknown; return; }
    if   have apt-get; then echo apt
    elif have dnf;     then echo dnf
    elif have pacman;  then echo pacman
    elif have zypper;  then echo zypper
    elif have apk;     then echo apk
    else echo unknown
    fi
}

detect_update_pm() {
    detect_pm
}

homebrew_bin() {
    if have brew; then command -v brew; return 0; fi
    local candidate
    for candidate in /opt/homebrew/bin/brew /usr/local/bin/brew /home/linuxbrew/.linuxbrew/bin/brew "$HOME/.linuxbrew/bin/brew"; do
        if [[ -x "$candidate" ]]; then printf '%s\n' "$candidate"; return 0; fi
    done
    return 1
}

enable_homebrew_for_current_shell() {
    local brew_bin brew_env active_brew
    local expected_prefix expected_repository active_prefix active_repository
    local old_path="$PATH" old_manpath="${MANPATH-}" old_infopath="${INFOPATH-}"
    local old_prefix="${HOMEBREW_PREFIX-}" old_cellar="${HOMEBREW_CELLAR-}"
    local old_repository="${HOMEBREW_REPOSITORY-}"
    local had_manpath="${MANPATH+x}" had_infopath="${INFOPATH+x}"
    local had_prefix="${HOMEBREW_PREFIX+x}" had_cellar="${HOMEBREW_CELLAR+x}"
    local had_repository="${HOMEBREW_REPOSITORY+x}"
    brew_bin="$(homebrew_bin)" || return 1
    expected_prefix="$("$brew_bin" --prefix 2>/dev/null)" || {
        echo "  FAIL: $brew_bin did not report its Homebrew prefix; PATH is unchanged" >&2
        return 1
    }
    expected_repository="$("$brew_bin" --repository 2>/dev/null)" || {
        echo "  FAIL: $brew_bin did not report its Homebrew repository; PATH is unchanged" >&2
        return 1
    }
    if [[ "$expected_prefix" != /* || "$expected_repository" != /* ]]; then
        echo "  FAIL: $brew_bin reported a non-absolute Homebrew identity; PATH is unchanged" >&2
        return 1
    fi
    if ! brew_env="$("$brew_bin" shellenv)"; then
        echo "  FAIL: $brew_bin shellenv failed; PATH is unchanged" >&2
        return 1
    fi
    # Homebrew intentionally emits no output when its bin/sbin entries are
    # already first in PATH. Empty stdout is therefore valid only when the
    # selected brew is already the executable this shell resolves.
    if [[ -n "$brew_env" ]] && ! eval "$brew_env"; then
        PATH="$old_path"; export PATH
        if [[ -n "$had_manpath" ]]; then MANPATH="$old_manpath"; export MANPATH; else unset MANPATH; fi
        if [[ -n "$had_infopath" ]]; then INFOPATH="$old_infopath"; export INFOPATH; else unset INFOPATH; fi
        if [[ -n "$had_prefix" ]]; then HOMEBREW_PREFIX="$old_prefix"; export HOMEBREW_PREFIX; else unset HOMEBREW_PREFIX; fi
        if [[ -n "$had_cellar" ]]; then HOMEBREW_CELLAR="$old_cellar"; export HOMEBREW_CELLAR; else unset HOMEBREW_CELLAR; fi
        if [[ -n "$had_repository" ]]; then HOMEBREW_REPOSITORY="$old_repository"; export HOMEBREW_REPOSITORY; else unset HOMEBREW_REPOSITORY; fi
        hash -r 2>/dev/null || true
        echo "  FAIL: $brew_bin shellenv output could not be evaluated; the prior environment was restored. Repair Homebrew and retry." >&2
        return 1
    fi
    hash -r 2>/dev/null || true
    active_brew="$(command -v brew 2>/dev/null || true)"
    active_prefix=""
    active_repository=""
    if [[ -n "$active_brew" ]]; then
        active_prefix="$("$active_brew" --prefix 2>/dev/null || true)"
        active_repository="$("$active_brew" --repository 2>/dev/null || true)"
    fi
    # nix-darwin exposes brew through /run/current-system/sw while `brew
    # shellenv` correctly activates the architecture-native /opt/homebrew or
    # /usr/local entrypoint. Prove both commands address the same installation
    # by canonical prefix + repository instead of requiring the wrapper and
    # activated executable to have the same pathname.
    if [[ -z "$active_brew" || -z "$active_prefix" || -z "$active_repository" ]] ||
        [[ "$(real_source_path "$active_prefix")" != "$(real_source_path "$expected_prefix")" ]] ||
        [[ "$(real_source_path "$active_repository")" != "$(real_source_path "$expected_repository")" ]]; then
        PATH="$old_path"; export PATH
        if [[ -n "$had_manpath" ]]; then MANPATH="$old_manpath"; export MANPATH; else unset MANPATH; fi
        if [[ -n "$had_infopath" ]]; then INFOPATH="$old_infopath"; export INFOPATH; else unset INFOPATH; fi
        if [[ -n "$had_prefix" ]]; then HOMEBREW_PREFIX="$old_prefix"; export HOMEBREW_PREFIX; else unset HOMEBREW_PREFIX; fi
        if [[ -n "$had_cellar" ]]; then HOMEBREW_CELLAR="$old_cellar"; export HOMEBREW_CELLAR; else unset HOMEBREW_CELLAR; fi
        if [[ -n "$had_repository" ]]; then HOMEBREW_REPOSITORY="$old_repository"; export HOMEBREW_REPOSITORY; else unset HOMEBREW_REPOSITORY; fi
        hash -r 2>/dev/null || true
        echo "  FAIL: $brew_bin shellenv did not activate the selected Homebrew installation; the prior environment was restored. Remove the PATH shadow and retry." >&2
        return 1
    fi
    enable_homebrew_make_gnubin_for_current_shell "$brew_bin" || true
    hash -r 2>/dev/null || true
}

# Homebrew owns the completion symlinks consumed by zsh/bash. Package/tap
# migrations can leave those links pointing at a removed repository checkout,
# which makes every new shell print compinit errors even though brew itself
# works. `brew completions link` reconciles tap completions, but Homebrew 6 does
# not repair its own core `_brew` link. Reconcile both surfaces and prove that
# the core link resolves to the active Homebrew implementation on macOS,
# nix-homebrew, and Linuxbrew.
link_homebrew_completions() {
    local brew_prefix brew_repository core_source candidate
    local site_dir core_link tmp source_real link_real

    [[ "$PM" == "brew" ]] || return 0
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: brew completions link + verify core _brew"
        return 0
    fi
    if ! brew completions link >/dev/null; then
        echo "  FAIL: Homebrew could not link its shell completions; repair Homebrew and retry." >&2
        return 1
    fi

    brew_prefix="$(brew --prefix 2>/dev/null || true)"
    brew_repository="$(brew --repository 2>/dev/null || true)"
    case "$brew_prefix" in
        /*) ;;
        *)
            echo "  FAIL: Homebrew did not report an absolute prefix while reconciling completions." >&2
            return 1
            ;;
    esac
    case "$brew_repository" in
        /*) ;;
        *)
            echo "  FAIL: Homebrew did not report an absolute repository while reconciling completions." >&2
            return 1
            ;;
    esac

    # Official Homebrew keeps the core completion below `brew --repository`.
    # nix-homebrew exposes a marker repository instead, while
    # $prefix/Library/Homebrew points into the active Nix generation; resolving
    # `../..` after that symlink reaches the matching package's completions.
    core_source=""
    for candidate in \
        "$brew_repository/completions/zsh/_brew" \
        "$brew_prefix/Library/Homebrew/../../completions/zsh/_brew" \
        "$brew_prefix/completions/zsh/_brew"; do
        if [[ -f "$candidate" && -r "$candidate" ]]; then
            core_source="$candidate"
            break
        fi
    done
    if [[ -z "$core_source" ]]; then
        echo "  FAIL: active Homebrew has no readable core zsh completion (_brew); reinstall or repair Homebrew and retry." >&2
        return 1
    fi

    site_dir="$brew_prefix/share/zsh/site-functions"
    core_link="$site_dir/_brew"
    if ! mkdir -p "$site_dir"; then
        echo "  FAIL: could not create Homebrew's zsh completion directory: $site_dir" >&2
        return 1
    fi

    source_real="$(real_source_path "$core_source")"
    if [[ -r "$core_link" ]]; then
        link_real="$(real_source_path "$core_link")"
    else
        link_real=""
    fi
    if [[ "$link_real" != "$source_real" ]]; then
        if [[ -e "$core_link" && ! -L "$core_link" ]]; then
            echo "  FAIL: refusing to replace non-symlink Homebrew completion: $core_link" >&2
            return 1
        fi
        if ! tmp="$(mktemp -d "$site_dir/.dotfiles-brew-completion.XXXXXX")"; then
            echo "  FAIL: could not stage Homebrew's core zsh completion beside $core_link" >&2
            return 1
        fi
        trap 'rm -rf "$tmp"; trap - RETURN' RETURN
        if ! ln -s "$core_source" "$tmp/_brew" || ! mv -f "$tmp/_brew" "$core_link"; then
            echo "  FAIL: could not publish Homebrew's core zsh completion: $core_link" >&2
            return 1
        fi
        rmdir "$tmp" 2>/dev/null || true
    fi

    if [[ ! -r "$core_link" ]] ||
        [[ "$(real_source_path "$core_link")" != "$source_real" ]]; then
        echo "  FAIL: Homebrew's core zsh completion did not resolve to the active implementation: $core_link" >&2
        return 1
    fi
    printf "  ok        %-26s linked + core _brew verified\n" "Homebrew completions"
}

enable_homebrew_make_gnubin_for_current_shell() {
    local brew_bin="$1" make_prefix gnubin
    make_prefix="$("$brew_bin" --prefix make 2>/dev/null || true)"
    [[ -n "$make_prefix" ]] || return 0
    gnubin="$make_prefix/libexec/gnubin"
    [[ -d "$gnubin" ]] || return 0
    case ":$PATH:" in
        *":$gnubin:"*) ;;
        *) PATH="$gnubin:$PATH"; export PATH ;;
    esac
}

persist_homebrew_shellenv() {
    local brew_bin brew_prefix marker end_marker block wrote=0
    local tmp block_tmp rc
    local rcs
    brew_bin="$(homebrew_bin)" || return 0
    brew_prefix="$("$brew_bin" --prefix 2>/dev/null)" || {
        echo "  FAIL: $brew_bin did not report the Homebrew prefix; shell startup files are unchanged" >&2
        return 1
    }
    if [[ "$brew_prefix" != /* ]]; then
        echo "  FAIL: $brew_bin reported a non-absolute Homebrew prefix; shell startup files are unchanged" >&2
        return 1
    fi
    marker="# >>> dotfiles: Homebrew shellenv >>>"
    end_marker="# <<< dotfiles: Homebrew shellenv <<<"
    block="$(cat <<EOF
$marker
if [ -x "$brew_prefix/bin/brew" ]; then
    eval "\$($brew_prefix/bin/brew shellenv)"
    dotfiles_make_prefix="\$($brew_prefix/bin/brew --prefix make 2>/dev/null || true)"
    dotfiles_make_gnubin="\$dotfiles_make_prefix/libexec/gnubin"
    dotfiles_path_with_colons=":\$PATH:"
    if [ -n "\$dotfiles_make_prefix" ] && [ -d "\$dotfiles_make_gnubin" ] &&
        [ "\${dotfiles_path_with_colons#*:\$dotfiles_make_gnubin:}" = "\$dotfiles_path_with_colons" ]; then
        export PATH="\$dotfiles_make_gnubin:\$PATH"
    fi
    unset dotfiles_make_prefix dotfiles_make_gnubin dotfiles_path_with_colons
fi
$end_marker
EOF
)"

    rcs=("$HOME/.zshrc.local")
    if [[ "$(uname -s)" == "Linux" ]]; then
        rcs+=("$HOME/.bashrc")
    fi

    for rc in "${rcs[@]}"; do
        if [[ "$DRY_RUN" -eq 1 ]]; then
            if [[ -f "$rc" ]] && grep -qF "$marker" "$rc"; then
                echo "  would: refresh Homebrew shellenv in $rc"
            else
                echo "  would: append Homebrew shellenv to $rc"
            fi
        else
            mkdir -p "$(dirname "$rc")"
            tmp="$rc.tmp.$$"
            block_tmp="$rc.block.$$"
            printf '%s\n' "$block" > "$block_tmp"
            if [[ -f "$rc" ]] && grep -qF "$marker" "$rc"; then
                if ! awk -v start="$marker" -v end="$end_marker" -v block_file="$block_tmp" '
                    $0 == start {
                        if (seen) {
                            exit 4
                        }
                        while ((getline line < block_file) > 0) {
                            print line
                        }
                        close(block_file)
                        inside = 1
                        seen = 1
                        next
                    }
                    inside && $0 == end {
                        inside = 0
                        next
                    }
                    !inside {
                        print
                    }
                    END {
                        if (inside) {
                            exit 2
                        }
                    }
                ' "$rc" > "$tmp"; then
                    rm -f "$tmp" "$block_tmp"
                    echo "  WARN: managed Homebrew shellenv block in $rc is malformed; leaving it unchanged" >&2
                    continue
                fi
            else
                if [[ -f "$rc" ]]; then
                    cp "$rc" "$tmp"
                else
                    : > "$tmp"
                fi
                {
                    printf '\n%s\n' "$block"
                } >> "$tmp"
            fi
            rm -f "$block_tmp"
            if [[ -f "$rc" ]] && cmp -s "$tmp" "$rc"; then
                rm -f "$tmp"
            else
                mv "$tmp" "$rc"
                wrote=1
            fi
        fi
    done
    if [[ "$wrote" -eq 1 ]]; then
        printf "  set       %-26s persisted shellenv for future shells\n" "homebrew PATH"
    fi
}

require_downloader() {
    if [[ "$(uname -s)" == "Darwin" ]]; then
        return 0
    fi
    if have curl; then
        return 0
    fi

    if [[ -z "${PM:-}" ]]; then
        PM="$(detect_pm)"
    fi
    case "$PM" in
        brew|apt|dnf|pacman|zypper|apk) ;;
        *)
            echo "  FAIL: need curl for direct downloads, but no supported package manager was detected" >&2
            return 1
            ;;
    esac

    echo "  need      curl missing; installing curl + CA certificates"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        pm_install curl ca-certificates
        return 0
    fi
    if ! pm_install curl ca-certificates; then
        return 1
    fi
    if ! have curl; then
        echo "  FAIL: curl still not found after installing curl ca-certificates" >&2
        return 1
    fi
}

# ask MUST be defined before maybe_install_brew uses it.
ask() {
    local prompt="$1"
    if [[ "$YES_ALL" -eq 1 || "$DRY_RUN" -eq 1 ]]; then return 0; fi
    printf "  %s [Y/n] " "$prompt"
    local answer
    if ! read -r answer; then return 1; fi
    answer="${answer:-y}"
    [[ "$answer" =~ ^[Yy]$ ]]
}

# sudo if needed, root-tolerant, container-tolerant. Echo to stderr when
# sudo is missing AND we're not root so the user knows why a step skipped.
maybe_sudo() {
    if [[ "$(id -u 2>/dev/null || echo 0)" -eq 0 ]]; then
        "$@"; return $?
    fi
    if have sudo; then
        sudo "$@"; return $?
    fi
    echo "  WARN: sudo not found and not running as root; skipping: $*" >&2
    return 1
}

# Debian-family package installs must stay unattended even when a dependency
# (notably tzdata) has debconf questions. Put the environment assignment after
# sudo so it survives sudo's environment filtering for ordinary users.
apt_get_noninteractive() {
    maybe_sudo env DEBIAN_FRONTEND=noninteractive apt-get "$@"
}

# ---- OS / package-manager detection ------------------------------------------
detect_pm() {
    if homebrew_bin >/dev/null 2>&1; then echo brew; return; fi
    if [[ "$(uname -s)" == "Darwin" ]]; then echo brew_missing; return; fi
    if [[ "$(uname -s)" == "Linux" ]]; then
        if   have apt-get; then echo apt
        elif have dnf;     then echo dnf
        elif have pacman;  then echo pacman
        elif have zypper;  then echo zypper
        elif have apk;     then echo apk
        else echo unknown
        fi
        return
    fi
    echo unknown
}

# ---- Homebrew bootstrap ------------------------------------------------------
maybe_install_brew() {
    if have brew; then return 0; fi
    if [[ "$(uname -s)" == "Linux" && "${DOTFILES_SKIP_BREW_BOOTSTRAP:-}" == "1" ]]; then
        echo "Homebrew bootstrap skipped by DOTFILES_SKIP_BREW_BOOTSTRAP=1; keeping native Linux package manager."
        return 1
    fi
    local kind
    if [[ "$(uname -s)" == "Darwin" ]]; then
        kind="required (no other package manager on macOS)"
    else
        kind="recommended (unlocks taplo / hyperfine / newer CLI tools that apt may not carry)"
    fi
    echo "Homebrew is not installed. $kind."
    if ask "Install Homebrew via the official installer?"; then
        local url tmp script
        url="https://raw.githubusercontent.com/Homebrew/install/${HOMEBREW_INSTALL_COMMIT}/install.sh"
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: curl -fsSL $url -o /tmp/homebrew-install.sh"
            echo "         verify sha256 $HOMEBREW_INSTALL_SHA256"
            echo "         NONINTERACTIVE=1 /bin/bash /tmp/homebrew-install.sh"
            return 0
        fi
        require_downloader || return 1
        tmp="$(mktemp -d)"
        trap 'rm -rf "$tmp"; trap - RETURN' RETURN
        script="$tmp/homebrew-install.sh"
        if ! curl -fsSL "$url" -o "$script"; then
            echo "  FAIL: Homebrew installer download failed from $url"
            rm -rf "$tmp"
            return 1
        fi
        if ! verify_sha256 "$script" "$HOMEBREW_INSTALL_SHA256"; then
            echo "  FAIL: checksum mismatch for Homebrew installer at $HOMEBREW_INSTALL_COMMIT"
            rm -rf "$tmp"
            return 1
        fi
        NONINTERACTIVE=1 /bin/bash "$script" || {
            rm -rf "$tmp"
            return 1
        }
        rm -rf "$tmp"
        # Plumb brew into THIS shell so subsequent installs use it, then make
        # future zsh/bash sessions find it without manual Homebrew "Next steps".
        if ! enable_homebrew_for_current_shell; then
            return 1
        fi
        if ! persist_homebrew_shellenv; then
            return 1
        fi
        return 0
    fi
    return 1
}

bootstrap_package_manager() {
    local homebrew_activated=0
    if [[ "$PM" == "brew" ]]; then
        if ! enable_homebrew_for_current_shell; then
            return 1
        fi
        if ! persist_homebrew_shellenv; then
            return 1
        fi
        homebrew_activated=1
    elif [[ "$PM" == "brew_missing" ]]; then
        if maybe_install_brew; then
            if [[ "$DRY_RUN" -eq 1 ]]; then PM="brew"; else PM="$(detect_pm)"; fi
            homebrew_activated=1
        else
            return 1
        fi
    elif [[ "$(uname -s)" == "Linux" ]]; then
        echo "Detected $PM as the system package manager."
        if maybe_install_brew; then
            PM="$(detect_pm)"
            homebrew_activated=1
        fi
    fi

    if [[ "$PM" == "brew" ]]; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  plan      Homebrew-dependent phases continue after the previewed bootstrap"
        elif [[ "$homebrew_activated" -eq 0 ]]; then
            if ! enable_homebrew_for_current_shell; then
                return 1
            fi
        fi
    fi
    return 0
}

# ---- Login shell: adopt zsh (chsh) -------------------------------------------
# Installing the zsh *package* only drops a binary on disk — it does NOT make
# zsh your login shell. Until /etc/passwd is updated, bare TTYs, SSH sessions,
# and every tmux pane keep launching whatever the account's shell is (bash on
# most Linux), so the symlinked ~/.zshrc never gets sourced. macOS already
# defaults to zsh, so this no-ops there. Idempotent, consent-gated, dry-run-safe.
zsh_bin() { command -v zsh 2>/dev/null; }

# Read the account's CURRENT login shell from the authoritative source per OS
# (NOT $SHELL, which is stale within a session after a chsh). Always exits 0.
current_login_shell() {
    local user shell=""
    user="$(id -un 2>/dev/null || echo "${USER:-}")"
    if [[ "$(uname -s)" == "Darwin" ]]; then
        shell="$(dscl . -read "/Users/$user" UserShell 2>/dev/null | awk '{print $2}')" || true
    elif have getent; then
        shell="$(getent passwd "$user" 2>/dev/null | cut -d: -f7)" || true
    elif [[ -r /etc/passwd ]]; then
        shell="$(awk -F: -v u="$user" '$1==u{print $7; exit}' /etc/passwd 2>/dev/null)" || true
    fi
    [[ -n "$shell" ]] || shell="${SHELL:-}"
    printf '%s' "$shell"
}

# chsh refuses a shell that isn't listed in /etc/shells. Register it (root only)
# when missing; return non-zero if we can't, so the caller skips chsh cleanly.
ensure_in_etc_shells() {
    local shell="$1" uid
    [[ -r /etc/shells ]] || return 0   # no file -> chsh may still proceed
    if grep -qxF "$shell" /etc/shells; then return 0; fi
    echo "  note      $shell not in /etc/shells; registering it (chsh needs this)"
    uid="$(id -u 2>/dev/null || echo 1000)"
    if [[ "$uid" -eq 0 ]]; then
        printf '%s\n' "$shell" >> /etc/shells
    elif have sudo; then
        printf '%s\n' "$shell" | sudo tee -a /etc/shells >/dev/null
    else
        echo "  manual    add it first:  echo '$shell' | sudo tee -a /etc/shells"
        return 1
    fi
}

# Run chsh by the least-privileged route that works: as root directly, via
# sudo (reuses cached creds, non-interactive-friendly), or plain chsh (PAM
# prompts for the user's own password) as a last resort.
set_login_shell() {
    local shell="$1" user="${2:-}" uid
    [[ -n "$user" ]] || user="$(id -un)"
    uid="$(id -u 2>/dev/null || echo 1000)"
    if [[ "$uid" -eq 0 ]]; then
        chsh -s "$shell" "$user"
    elif have sudo; then
        sudo chsh -s "$shell" "$user"
    else
        chsh -s "$shell"
    fi
}

# True when $1 is defined in the LOCAL /etc/passwd, i.e. chsh (which edits that
# file) can change its shell. Domain accounts (AD/LDAP via SSSD/winbind) resolve
# through NSS — `getent` finds them but they are NOT in /etc/passwd, so chsh
# bails with "user '<name>' does not exist in /etc/passwd".
is_local_account() {
    [[ -r /etc/passwd ]] || return 1
    awk -F: -v u="$1" '$1==u{found=1} END{exit !found}' /etc/passwd
}

# Interactive bash fallback: make bash re-exec into zsh when Linux terminals or
# tmux still start bash. Domain accounts cannot chsh; local graphical sessions
# can also keep a stale $SHELL after chsh until full logout. Idempotent (marked
# block, re-run safe), interactive-only (scp/rsync/scripts stay bash), and it
# points login shells (tmux, ssh) at ~/.bashrc so the guard fires there too.
# Reversible — delete the marked block to undo.
ensure_bash_execs_zsh() {
    local rc="$HOME/.bashrc" profile="$HOME/.bash_profile"
    local marker="# >>> dotfiles: exec zsh (interactive bash fallback) >>>"
    local legacy_marker="# >>> dotfiles: exec zsh (domain login; chsh unavailable) >>>"
    if [[ -f "$rc" ]] && { grep -qF "$marker" "$rc" || grep -qF "$legacy_marker" "$rc"; }; then
        echo "  ok        exec-zsh guard already present in ~/.bashrc"
    else
        cat >> "$rc" <<'EOF'

# >>> dotfiles: exec zsh (interactive bash fallback) >>>
# Interactive bash re-execs into zsh. Guards: not already zsh (no loop), only
# interactive shells (scp/rsync/scripts stay bash), and zsh must be installed.
if [ -z "${ZSH_VERSION:-}" ] && [ -n "${BASH_VERSION:-}" ] && [[ $- == *i* ]] && command -v zsh >/dev/null 2>&1; then
    SHELL="$(command -v zsh)"; export SHELL
    exec zsh
fi
# <<< dotfiles: exec zsh (interactive bash fallback) <<<
EOF
    fi
    # tmux / ssh start a LOGIN bash, which reads ~/.bash_profile (not ~/.bashrc).
    # Make sure it sources ~/.bashrc so the guard above runs there too.
    if [[ ! -f "$profile" ]] || ! grep -qF '.bashrc' "$profile"; then
        cat >> "$profile" <<'EOF'

# dotfiles: ensure interactive login shells read ~/.bashrc
[ -f "$HOME/.bashrc" ] && . "$HOME/.bashrc"
EOF
    fi
    return 0
}

# chsh path — local account, the textbook case.
adopt_zsh_chsh() {
    local zsh_path="$1" current="$2" os prompt
    os="$(uname -s)"
    prompt="Make zsh your default login shell (chsh)? current: ${current:-unknown}"
    if [[ "$os" != "Darwin" ]]; then
        prompt="Make zsh your default shell now (chsh + interactive bash fallback)? current: ${current:-unknown}"
    fi
    if ! ask "$prompt"; then
        printf "  skipped   %-26s kept %s\n" "default shell" "${current:-current shell}"
        echo "            (~/.zshrc is symlinked, but tmux / new terminals keep"
        echo "             launching ${current##*/} until the login shell changes)"
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: register $zsh_path in /etc/shells if needed, then chsh -s $zsh_path"
        if [[ "$os" != "Darwin" ]]; then
            echo "  would: add an interactive 'exec zsh' guard to ~/.bashrc"
            echo "         (+ make ~/.bash_profile source ~/.bashrc for login shells)"
        fi
        return 0
    fi
    if ! ensure_in_etc_shells "$zsh_path"; then
        echo "  WARN: could not register $zsh_path in /etc/shells; skipping chsh"
        return 0
    fi
    if set_login_shell "$zsh_path"; then
        printf "  changed   %-26s login shell -> %s\n" "default shell" "$zsh_path"
        if [[ "$os" != "Darwin" ]]; then
            ensure_bash_execs_zsh
            printf "  changed   %-26s ~/.bashrc now execs zsh for interactive shells\n" "default shell"
            echo "            open a new shell (or new tmux) to land in zsh; chsh also"
            echo "            fixes the real login shell for future sessions"
        else
            echo "            open a new terminal; log out/in if an app kept the old shell"
        fi
    else
        echo "  WARN: chsh failed; login shell unchanged"
        echo "        manual:  chsh -s '$zsh_path'"
    fi
    return 0
}

# Fallback path — domain/non-local account, where chsh cannot help.
adopt_zsh_domain() {
    local zsh_path="$1" current="$2"
    printf "  note      %-26s domain/non-local account; chsh can't change it\n" "default shell"
    if ! ask "Re-exec interactive bash into zsh via ~/.bashrc instead? (reversible)"; then
        printf "  skipped   %-26s kept %s\n" "default shell" "${current:-current shell}"
        echo "            (the 'proper' fix is admin-side: set your directory"
        echo "             loginShell or SSSD default_shell to $zsh_path)"
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: add an interactive 'exec zsh' guard to ~/.bashrc"
        echo "         (+ make ~/.bash_profile source ~/.bashrc for login shells)"
        return 0
    fi
    ensure_bash_execs_zsh
    printf "  changed   %-26s ~/.bashrc now execs zsh for interactive shells\n" "default shell"
    echo "            open a new shell (or new tmux) to land in zsh"
    return 0
}

set_default_shell_zsh() {
    if ! have zsh; then
        printf "  skipped   %-26s zsh not installed; login shell unchanged\n" "default shell"
        return 0
    fi
    local zsh_path current user
    zsh_path="$(zsh_bin)"
    current="$(current_login_shell 2>/dev/null || true)"
    user="$(id -un 2>/dev/null || echo "${USER:-}")"

    # Already a zsh? (covers macOS's default + idempotent re-runs). Compare by
    # basename so /bin/zsh, /usr/bin/zsh, and a brew zsh all count as "done".
    if [[ "${current##*/}" == "zsh" ]]; then
        printf "  ok        %-26s already %s\n" "default shell" "${current:-zsh}"
        return 0
    fi

    # Domain accounts (AD/LDAP) aren't in /etc/passwd, so chsh fails on them;
    # re-exec bash into zsh instead. macOS accounts live in dscl, not passwd
    # files, yet chsh works there — so only take the domain branch on Linux.
    if [[ "$(uname -s)" != "Darwin" ]] && ! is_local_account "$user"; then
        adopt_zsh_domain "$zsh_path" "$current"
    else
        adopt_zsh_chsh "$zsh_path" "$current"
    fi
}

# ---- Notes / Obsidian vault -------------------------------------------------
# obsidian.nvim resolves its vault from $NOTES_VAULT (else an OS default; see
# nvim/lua/util/notes_path.lua). We persist the user's choice to ~/.zshrc.local
# (gitignored, sourced by shells/zshrc) so nvim picks it up on the next shell.

# Append `export NOTES_VAULT=<path>` to ~/.zshrc.local and create the dir.
# Expands a leading ~; echoes the resolved path. Split out from the prompt so
# tests can exercise it without a tty.
persist_notes_vault() {
    local path="$1" rc="$HOME/.zshrc.local"
    # Expand a leading ~ the user typed literally (it came from `read`, so the
    # shell's own tilde expansion never ran). Matching a literal ~, not expanding.
    # shellcheck disable=SC2088
    case "$path" in
        "~") path="$HOME" ;;
        "~/"*) path="$HOME/${path#\~/}" ;;
    esac
    mkdir -p "$path" 2>/dev/null || true
    {
        printf '\n# dotfiles: notes/Obsidian vault for obsidian.nvim (set by install-deps.sh)\n'
        printf 'export NOTES_VAULT=%q\n' "$path"
    } >> "$rc"
    printf '%s' "$path"
}

configure_notes_vault() {
    local rc="$HOME/.zshrc.local"
    if [[ -n "${NOTES_VAULT:-}" ]]; then
        printf "  ok        %-26s NOTES_VAULT already set (%s)\n" "notes vault" "$NOTES_VAULT"
        return 0
    fi
    if [[ -f "$rc" ]] && grep -q '^[[:space:]]*export NOTES_VAULT=' "$rc"; then
        printf "  ok        %-26s NOTES_VAULT already in ~/.zshrc.local\n" "notes vault"
        return 0
    fi
    # Never block non-interactive runs (--all / piped stdin / dry-run); just hint.
    if [[ "$YES_ALL" -eq 1 || "$DRY_RUN" -eq 1 || ! -t 0 ]]; then
        printf "  skipped   %-26s export NOTES_VAULT in ~/.zshrc.local to point obsidian.nvim at your vault\n" "notes vault"
        return 0
    fi
    printf "  Path to your notes / Obsidian vault for obsidian.nvim\n"
    printf "  (absolute or ~-relative; blank = OS default): "
    local path
    if ! read -r path || [[ -z "$path" ]]; then
        printf "  skipped   %-26s using the OS default (see nvim/lua/util/notes_path.lua)\n" "notes vault"
        return 0
    fi
    local resolved
    resolved="$(persist_notes_vault "$path")"
    printf "  set       %-26s NOTES_VAULT=%s\n" "notes vault" "$resolved"
    echo "            (in ~/.zshrc.local; open a new shell or 'source ~/.zshrc.local')"
}

# ---- VS Code: Rose Pine theme (pure helpers; tested) ------------------------
# Where VS Code stores user settings.json, per OS.
vscode_settings_path() {
    if [[ "$(uname -s)" == "Darwin" ]]; then
        printf '%s' "$HOME/Library/Application Support/Code/User/settings.json"
    else
        printf '%s' "${XDG_CONFIG_HOME:-$HOME/.config}/Code/User/settings.json"
    fi
}

# Set the VS Code Rose Pine theme plus Hack Nerd Font settings. The theme value
# is a literal é (UTF-8) because it must match the mvllow.rose-pine label.
write_vscode_jsonc_settings() {
    local settings="$1" tmp="$2"
    local theme="Rosé Pine"
    local font="'Hack Nerd Font', Consolas, monospace"
    local cr_count lf_count bare_lf_count use_crlf cr_char lf_char
    cr_char=$'\r'
    lf_char=$'\n'
    cr_count="$(LC_ALL=C tr -cd "$cr_char" < "$settings" | wc -c | tr -d ' ')"
    lf_count="$(LC_ALL=C tr -cd "$lf_char" < "$settings" | wc -c | tr -d ' ')"
    bare_lf_count=$((lf_count - cr_count))
    use_crlf=0
    if (( cr_count > bare_lf_count )); then
        use_crlf=1
    fi
    awk -v theme="$theme" -v font="$font" -v use_crlf="$use_crlf" '
function json_quote(s, out) {
    out = s
    gsub(/\\/, "\\\\", out)
    gsub(/"/, "\\\"", out)
    return "\"" out "\""
}
function is_ws(c) {
    return c == " " || c == "\t" || c == "\r" || c == "\n"
}
function skip_trivia(pos, c, n) {
    while (pos <= len) {
        c = substr(text, pos, 1)
        if (is_ws(c)) {
            pos++
            continue
        }
        if (c == "/" && pos < len) {
            n = substr(text, pos + 1, 1)
            if (n == "/") {
                pos += 2
                while (pos <= len && substr(text, pos, 1) != "\n") {
                    pos++
                }
                continue
            }
            if (n == "*") {
                pos += 2
                while (pos < len && !(substr(text, pos, 1) == "*" && substr(text, pos + 1, 1) == "/")) {
                    pos++
                }
                if (pos < len) {
                    pos += 2
                }
                continue
            }
        }
        break
    }
    return pos
}
function scan_string_end(pos, i, c, escaped) {
    escaped = 0
    for (i = pos + 1; i <= len; i++) {
        c = substr(text, i, 1)
        if (escaped) {
            escaped = 0
            continue
        }
        if (c == "\\") {
            escaped = 1
            continue
        }
        if (c == "\"") {
            return i
        }
    }
    return 0
}
function find_value_end(pos, i, c, n, end, curly, square) {
    pos = skip_trivia(pos)
    if (pos > len) {
        return 0
    }
    c = substr(text, pos, 1)
    if (c == "\"") {
        return scan_string_end(pos)
    }
    if (c == "{" || c == "[") {
        curly = 0
        square = 0
        i = pos
        while (i <= len) {
            c = substr(text, i, 1)
            if (c == "/" && i < len) {
                n = substr(text, i + 1, 1)
                if (n == "/") {
                    i += 2
                    while (i <= len && substr(text, i, 1) != "\n") {
                        i++
                    }
                    continue
                }
                if (n == "*") {
                    i += 2
                    while (i < len && !(substr(text, i, 1) == "*" && substr(text, i + 1, 1) == "/")) {
                        i++
                    }
                    if (i < len) {
                        i += 2
                    }
                    continue
                }
            }
            if (c == "\"") {
                i = scan_string_end(i)
                if (i == 0) {
                    return 0
                }
            } else if (c == "{") {
                curly++
            } else if (c == "}") {
                curly--
                if (curly == 0 && square == 0) {
                    return i
                }
            } else if (c == "[") {
                square++
            } else if (c == "]") {
                square--
                if (curly == 0 && square == 0) {
                    return i
                }
            }
            i++
        }
        return 0
    }
    end = pos
    for (i = pos; i <= len; i++) {
        c = substr(text, i, 1)
        if (c == "," || c == "}") {
            break
        }
        if (c == "/" && i < len) {
            n = substr(text, i + 1, 1)
            if (n == "/" || n == "*") {
                break
            }
        }
        end = i
    }
    while (end >= pos && is_ws(substr(text, end, 1))) {
        end--
    }
    return end
}
{
    lines[++line_count] = $0
}
END {
    keys[1] = "workbench.colorTheme"
    keys[2] = "workbench.preferredDarkColorTheme"
    keys[3] = "workbench.preferredLightColorTheme"
    keys[4] = "window.autoDetectColorScheme"
    keys[5] = "editor.fontFamily"
    keys[6] = "terminal.integrated.fontFamily"
    values[1] = json_quote(theme)
    values[2] = json_quote(theme)
    values[3] = json_quote(theme)
    values[4] = "false"
    values[5] = json_quote(font)
    values[6] = json_quote(font)
    eol = use_crlf ? "\r\n" : "\n"

    for (i = 1; i <= line_count; i++) {
        line = lines[i]
        sub(/\r$/, "", line)
        text = text line eol
    }
    len = length(text)
    root = 0
    curly = 0
    square = 0
    i = 1
    while (i <= len) {
        c = substr(text, i, 1)
        if (c == "/" && i < len) {
            n = substr(text, i + 1, 1)
            if (n == "/") {
                i += 2
                while (i <= len && substr(text, i, 1) != "\n") {
                    i++
                }
                continue
            }
            if (n == "*") {
                i += 2
                while (i < len && !(substr(text, i, 1) == "*" && substr(text, i + 1, 1) == "/")) {
                    i++
                }
                if (i < len) {
                    i += 2
                }
                continue
            }
        }
        if (c == "\"") {
            end = scan_string_end(i)
            if (end == 0) {
                exit 2
            }
            if (curly == 1 && square == 0) {
                after = skip_trivia(end + 1)
                if (after <= len && substr(text, after, 1) == ":") {
                    key = substr(text, i + 1, end - i - 1)
                    for (k = 1; k <= 6; k++) {
                        if (key == keys[k]) {
                            vstart = skip_trivia(after + 1)
                            vend = find_value_end(vstart)
                            if (vend == 0) {
                                exit 2
                            }
                            seen[k] = 1
                            rep_count++
                            rep_start[rep_count] = vstart
                            rep_end[rep_count] = vend
                            rep_value[rep_count] = values[k]
                        }
                    }
                }
            }
            i = end + 1
            continue
        }
        if (c == "{") {
            curly++
            if (root == 0) {
                root = i
            }
        } else if (c == "}") {
            curly--
            if (curly < 0) {
                curly = 0
            }
        } else if (c == "[") {
            square++
        } else if (c == "]" && square > 0) {
            square--
        }
        i++
    }
    if (root == 0) {
        exit 2
    }
    for (r = rep_count; r >= 1; r--) {
        text = substr(text, 1, rep_start[r] - 1) rep_value[r] substr(text, rep_end[r] + 1)
    }
    len = length(text)
    missing_count = 0
    for (k = 1; k <= 6; k++) {
        if (!seen[k]) {
            missing_count++
            missing_keys[missing_count] = keys[k]
            missing_values[missing_count] = values[k]
        }
    }
    if (missing_count > 0) {
        first = skip_trivia(root + 1)
        has_existing = first <= len && substr(text, first, 1) != "}"
        insert_at = root + 1
        if (substr(text, insert_at, 2) == "\r\n") {
            insert_at += 2
        } else if (substr(text, insert_at, 1) == "\n") {
            insert_at++
        }
        block = eol
        for (m = 1; m <= missing_count; m++) {
            line = "  \"" missing_keys[m] "\": " missing_values[m]
            if (has_existing || m < missing_count) {
                line = line ","
            }
            block = block line eol
        }
        text = substr(text, 1, root) block substr(text, insert_at)
    }
    printf "%s", text
}
' "$settings" > "$tmp"
}

# Handles:
#   - absent/empty -> write a fresh minimal settings.json
#   - valid JSON -> jq-merge the top-level settings
#   - JSONC -> comment-aware top-level edit with a timestamped backup
set_vscode_theme() {
    local settings="${1:-}"
    local theme="Rosé Pine"
    local font="'Hack Nerd Font', Consolas, monospace"
    [[ -n "$settings" ]] || settings="$(vscode_settings_path)"
    mkdir -p "$(dirname "$settings")" 2>/dev/null || true
    # FORCE dark Rose Pine. When window.autoDetectColorScheme is true (Settings
    # Sync / an imported profile can enable it) VS Code IGNORES colorTheme and
    # uses workbench.preferredDark/LightColorTheme (defaulting to Dark Modern).
    # So pin autoDetect off (a JSON boolean, not a string) AND point both
    # preferred slots at the same dark Rose Pine -- no OS-scheme combination can
    # yield a light theme (same forced-dark rule as Ghostty; see tests/MANUAL.md).
    if [[ ! -s "$settings" ]]; then
        printf '{\n  "workbench.colorTheme": "%s",\n  "workbench.preferredDarkColorTheme": "%s",\n  "workbench.preferredLightColorTheme": "%s",\n  "window.autoDetectColorScheme": false,\n  "editor.fontFamily": "%s",\n  "terminal.integrated.fontFamily": "%s"\n}\n' \
            "$theme" "$theme" "$theme" "$font" "$font" > "$settings"
        printf "  set       %-26s theme and fonts (new settings.json)\n" "rose-pine (vscode)"
        return 0
    fi
    if have jq && jq -e . "$settings" >/dev/null 2>&1; then
        local tmp; tmp="$(mktemp)"
        if jq --arg theme "$theme" --arg font "$font" \
            '. + {
                "workbench.colorTheme": $theme,
                "workbench.preferredDarkColorTheme": $theme,
                "workbench.preferredLightColorTheme": $theme,
                "window.autoDetectColorScheme": false,
                "editor.fontFamily": $font,
                "terminal.integrated.fontFamily": $font
            }' "$settings" > "$tmp"; then
            mv "$tmp" "$settings"
            printf "  set       %-26s theme and fonts (merged)\n" "rose-pine (vscode)"
        else
            rm -f "$tmp"
            echo "  WARN: could not merge VS Code settings into $settings"
        fi
        return 0
    fi
    local backup
    backup="$(unique_backup_path "$settings")"
    cp "$settings" "$backup"
    local tmp; tmp="$(mktemp)"
    if write_vscode_jsonc_settings "$settings" "$tmp"; then
        mv "$tmp" "$settings"
        printf "  set       %-26s theme and fonts (jsonc edit; backup: %s)\n" "rose-pine (vscode)" "$backup"
    else
        rm -f "$tmp"
        echo "  WARN: could not edit VS Code settings in $settings (backup: $backup)"
    fi
    return 0
}

install_nvim_linux() {
    if have nvim && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "nvim"
        return
    fi
    if [[ "$(native_linux_pm)" == "apk" ]]; then
        if ! ask "Install nvim via apk (native Alpine package)?"; then
            printf "  skipped   %-26s\n" "nvim"
            return
        fi
        local rc=0
        native_linux_pm_install apk neovim || rc=$?
        if [[ "$rc" -ne 0 ]]; then
            record_install_failure "nvim" apk "neovim" "$rc"
            echo "  FAIL: nvim apk install failed; continuing to collect install failures"
            return
        fi
        if have nvim; then
            printf "  installed %-26s via apk\n" "nvim"
        fi
        return
    fi
    local machine arch asset url install_dir expected tmp tarball
    machine="$(uname -m)"
    case "$machine" in
        x86_64|amd64)
            arch="x86_64"
            expected="$NVIM_LINUX_X86_64_SHA256"
            ;;
        aarch64|arm64)
            arch="arm64"
            expected="$NVIM_LINUX_ARM64_SHA256"
            ;;
        *)
            printf "  manual    %-26s unsupported Linux arch: %s\n" "nvim" "$machine"
            echo "            install from https://github.com/neovim/neovim/releases"
            return
            ;;
    esac

    asset="nvim-linux-${arch}.tar.gz"
    url="https://github.com/neovim/neovim/releases/download/${NVIM_LINUX_VERSION}/${asset}"
    install_dir="/opt/nvim-linux-${arch}"

    if ! ask "Install nvim (official Neovim stable Linux ${arch} tarball)?"; then
        printf "  skipped   %-26s\n" "nvim"
        return
    fi
    require_downloader || return 1
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fsSL $url -o /tmp/$asset"
        echo "         sudo rm -rf $install_dir"
        echo "         sudo tar -xzf /tmp/$asset -C /opt"
        echo "         sudo ln -sfn $install_dir/bin/nvim /usr/local/bin/nvim"
        return
    fi

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    tarball="$tmp/$asset"
    if ! curl -fsSL "$url" -o "$tarball"; then
        echo "  FAIL: nvim download failed from $url"
        rm -rf "$tmp"
        return 1
    fi
    if ! verify_sha256 "$tarball" "$expected"; then
        echo "  FAIL: checksum mismatch for $asset"
        rm -rf "$tmp"
        return 1
    fi
    if ! maybe_sudo rm -rf "$install_dir"; then
        echo "  FAIL: could not clear $install_dir"
        rm -rf "$tmp"
        return 1
    fi
    if ! maybe_sudo tar -xzf "$tarball" -C /opt; then
        echo "  FAIL: could not extract $asset into /opt"
        rm -rf "$tmp"
        return 1
    fi
    if ! maybe_sudo ln -sfn "$install_dir/bin/nvim" /usr/local/bin/nvim; then
        echo "  FAIL: could not link /usr/local/bin/nvim"
        rm -rf "$tmp"
        return 1
    fi
    write_direct_artifact_provenance "nvim" "/usr/local/bin/nvim" "$install_dir/bin/nvim" "$install_dir" "$url" "$NVIM_LINUX_VERSION" "$expected"
    rm -rf "$tmp"
    printf "  installed %-26s -> %s/bin/nvim\n" "nvim" "$install_dir"
}

install_lazygit_linux() {
    if have lazygit && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "lazygit"
        return
    fi
    if [[ "$(native_linux_pm)" == "apk" ]]; then
        if ! ask "Install lazygit via apk (native Alpine package)?"; then
            printf "  skipped   %-26s\n" "lazygit"
            return
        fi
        local rc=0
        native_linux_pm_install apk lazygit || rc=$?
        if [[ "$rc" -ne 0 ]]; then
            record_install_failure "lazygit" apk "lazygit" "$rc"
            echo "  FAIL: lazygit apk install failed; continuing to collect install failures"
            return
        fi
        if have lazygit; then
            printf "  installed %-26s via apk\n" "lazygit"
        fi
        return
    fi

    local machine arch expected version_no_v asset url tmp tarball install_target
    machine="$(uname -m)"
    case "$machine" in
        x86_64|amd64)
            arch="x86_64"
            expected="$LAZYGIT_LINUX_X86_64_SHA256"
            ;;
        aarch64|arm64)
            arch="arm64"
            expected="$LAZYGIT_LINUX_ARM64_SHA256"
            ;;
        *)
            printf "  manual    %-26s unsupported Linux arch: %s\n" "lazygit" "$machine"
            echo "            install from https://github.com/jesseduffield/lazygit/releases"
            return
            ;;
    esac

    version_no_v="${LAZYGIT_LINUX_VERSION#v}"
    asset="lazygit_${version_no_v}_linux_${arch}.tar.gz"
    url="https://github.com/jesseduffield/lazygit/releases/download/${LAZYGIT_LINUX_VERSION}/${asset}"

    if ! ask "Install lazygit (pinned GitHub release ${LAZYGIT_LINUX_VERSION}, Linux ${arch})?"; then
        printf "  skipped   %-26s\n" "lazygit"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fsSL $url -o /tmp/$asset"
        echo "         verify sha256 $expected"
        echo "         install lazygit -> /usr/local/bin/lazygit"
        echo "         fallback -> \$HOME/.local/bin/lazygit when sudo is unavailable"
        return
    fi
    require_downloader || return 1
    if ! have tar; then
        echo "  need      tar missing; installing extractor for lazygit"
        install tar "extract lazygit release archive"
        if ! have tar; then
            echo "  FAIL: need tar to extract $asset"
            return 1
        fi
    fi

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    tarball="$tmp/$asset"
    if ! curl -fsSL "$url" -o "$tarball"; then
        echo "  FAIL: lazygit download failed from $url"
        rm -rf "$tmp"
        return 1
    fi
    if ! verify_sha256 "$tarball" "$expected"; then
        echo "  FAIL: checksum mismatch for $asset"
        rm -rf "$tmp"
        return 1
    fi
    if ! tar -xzf "$tarball" -C "$tmp" lazygit; then
        echo "  FAIL: could not extract lazygit from $asset"
        rm -rf "$tmp"
        return 1
    fi

    if [[ "$(id -u 2>/dev/null || echo 0)" -eq 0 ]] || have sudo; then
        if maybe_sudo mkdir -p /usr/local/bin &&
            maybe_sudo cp "$tmp/lazygit" /usr/local/bin/lazygit &&
            maybe_sudo chmod 0755 /usr/local/bin/lazygit; then
            write_direct_artifact_provenance "lazygit" "/usr/local/bin/lazygit" "/usr/local/bin/lazygit" "/usr/local/bin" "$url" "$LAZYGIT_LINUX_VERSION" "$expected"
            rm -rf "$tmp"
            printf "  installed %-26s -> /usr/local/bin/lazygit\n" "lazygit"
            return
        fi
        echo "  WARN: could not install lazygit to /usr/local/bin; trying user-local bin"
    fi

    install_target="$HOME/.local/bin/lazygit"
    if ! mkdir -p "$HOME/.local/bin" ||
        ! cp "$tmp/lazygit" "$install_target" ||
        ! chmod 0755 "$install_target"; then
        echo "  FAIL: could not install lazygit to $install_target"
        return 1
    fi
    write_direct_artifact_provenance "lazygit" "$install_target" "$install_target" "$(dirname "$install_target")" "$url" "$LAZYGIT_LINUX_VERSION" "$expected"
    rm -rf "$tmp"
    ensure_local_bin_on_path
    printf "  installed %-26s -> %s\n" "lazygit" "$install_target"
}

install_lazygit() {
    if [[ "$PM" == "brew" ]]; then
        install lazygit "terminal git UI"
    elif [[ "$(uname -s)" == "Linux" ]]; then
        install_lazygit_linux
    else
        install lazygit "terminal git UI"
    fi
}

install_starship_linux() {
    if have starship && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "starship"
        return
    fi
    if [[ "$(native_linux_pm)" == "apk" ]]; then
        if ! ask "Install starship via apk (native Alpine package)?"; then
            printf "  skipped   %-26s\n" "starship"
            return
        fi
        local rc=0
        native_linux_pm_install apk starship || rc=$?
        if [[ "$rc" -ne 0 ]]; then
            record_install_failure "starship" apk "starship" "$rc"
            echo "  FAIL: starship apk install failed; continuing to collect install failures"
            return
        fi
        if have starship; then
            printf "  installed %-26s via apk\n" "starship"
        fi
        return
    fi

    local machine arch expected asset url tmp tarball source_bin install_target
    machine="$(uname -m)"
    case "$machine" in
        x86_64|amd64)
            arch="x86_64"
            expected="$STARSHIP_LINUX_X86_64_SHA256"
            asset="starship-x86_64-unknown-linux-gnu.tar.gz"
            ;;
        aarch64|arm64)
            arch="aarch64"
            expected="$STARSHIP_LINUX_ARM64_SHA256"
            asset="starship-aarch64-unknown-linux-musl.tar.gz"
            ;;
        *)
            printf "  manual    %-26s unsupported Linux arch: %s\n" "starship" "$machine"
            echo "            install from https://github.com/starship/starship/releases"
            return
            ;;
    esac

    url="https://github.com/starship/starship/releases/download/${STARSHIP_VERSION}/${asset}"

    if ! ask "Install starship (pinned GitHub release ${STARSHIP_VERSION}, Linux ${arch})?"; then
        printf "  skipped   %-26s\n" "starship"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fsSL $url -o /tmp/$asset"
        echo "         verify sha256 $expected"
        echo "         install starship -> /usr/local/bin/starship"
        echo "         fallback -> \$HOME/.local/bin/starship when sudo is unavailable"
        return
    fi
    require_downloader || return 1
    if ! have tar; then
        echo "  need      tar missing; installing extractor for starship"
        install tar "extract starship release archive"
        if ! have tar; then
            echo "  FAIL: need tar to extract $asset"
            return 1
        fi
    fi

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    tarball="$tmp/$asset"
    if ! curl -fsSL "$url" -o "$tarball"; then
        echo "  FAIL: starship download failed from $url"
        rm -rf "$tmp"
        return 1
    fi
    if ! verify_sha256 "$tarball" "$expected"; then
        echo "  FAIL: checksum mismatch for $asset"
        rm -rf "$tmp"
        return 1
    fi
    if ! tar -xzf "$tarball" -C "$tmp" starship; then
        echo "  FAIL: could not extract starship from $asset"
        rm -rf "$tmp"
        return 1
    fi

    source_bin="$tmp/starship"
    if [[ ! -f "$source_bin" ]]; then
        source_bin="$(find "$tmp" -type f -name starship -print -quit)"
    fi
    if [[ -z "$source_bin" || ! -f "$source_bin" ]]; then
        echo "  FAIL: starship binary missing from $asset"
        rm -rf "$tmp"
        return 1
    fi

    if [[ "$(id -u 2>/dev/null || echo 0)" -eq 0 ]] || have sudo; then
        if maybe_sudo mkdir -p /usr/local/bin &&
            maybe_sudo cp "$source_bin" /usr/local/bin/starship &&
            maybe_sudo chmod 0755 /usr/local/bin/starship; then
            write_direct_artifact_provenance "starship" "/usr/local/bin/starship" "/usr/local/bin/starship" "/usr/local/bin" "$url" "$STARSHIP_VERSION" "$expected"
            rm -rf "$tmp"
            printf "  installed %-26s -> /usr/local/bin/starship\n" "starship"
            return
        fi
        echo "  WARN: could not install starship to /usr/local/bin; trying user-local bin"
    fi

    install_target="$HOME/.local/bin/starship"
    if ! mkdir -p "$HOME/.local/bin" ||
        ! cp "$source_bin" "$install_target" ||
        ! chmod 0755 "$install_target"; then
        echo "  FAIL: could not install starship to $install_target"
        rm -rf "$tmp"
        return 1
    fi
    write_direct_artifact_provenance "starship" "$install_target" "$install_target" "$(dirname "$install_target")" "$url" "$STARSHIP_VERSION" "$expected"
    rm -rf "$tmp"
    ensure_local_bin_on_path
    printf "  installed %-26s -> %s\n" "starship" "$install_target"
}

install_starship() {
    if [[ "$PM" == "brew" ]]; then
        install starship "cross-shell prompt"
    elif [[ "$(uname -s)" == "Linux" ]]; then
        install_starship_linux
    else
        install starship "cross-shell prompt"
    fi
}

install_tree_sitter_cli_linux() {
    if have tree-sitter && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "tree-sitter"
        return
    fi

    local machine arch expected asset url tmp archive extract_dir source_bin install_target
    machine="$(uname -m)"
    case "$machine" in
        x86_64|amd64)
            arch="x64"
            expected="$TREE_SITTER_CLI_LINUX_X86_64_SHA256"
            ;;
        aarch64|arm64)
            arch="arm64"
            expected="$TREE_SITTER_CLI_LINUX_ARM64_SHA256"
            ;;
        *)
            printf "  manual    %-26s unsupported Linux arch: %s\n" "tree-sitter" "$machine"
            echo "            install from https://github.com/tree-sitter/tree-sitter/releases"
            return
            ;;
    esac

    asset="tree-sitter-cli-linux-${arch}.zip"
    url="https://github.com/tree-sitter/tree-sitter/releases/download/${TREE_SITTER_CLI_LINUX_VERSION}/${asset}"

    if ! ask "Install tree-sitter CLI (pinned GitHub release ${TREE_SITTER_CLI_LINUX_VERSION}, Linux ${arch})?"; then
        printf "  skipped   %-26s\n" "tree-sitter"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fsSL $url -o /tmp/$asset"
        echo "         verify sha256 $expected"
        echo "         unzip tree-sitter -> \$HOME/.local/bin/tree-sitter"
        return
    fi
    require_downloader || return 1
    if ! have unzip; then
        echo "  need      unzip missing; installing extractor for tree-sitter"
        install unzip "extract tree-sitter CLI release archive"
        if ! have unzip; then
            echo "  FAIL: need unzip to extract $asset"
            return 1
        fi
    fi

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    archive="$tmp/$asset"
    extract_dir="$tmp/extract"
    mkdir -p "$extract_dir"
    if ! curl -fsSL "$url" -o "$archive"; then
        echo "  FAIL: tree-sitter download failed from $url"
        rm -rf "$tmp"
        return 1
    fi
    if ! verify_sha256 "$archive" "$expected"; then
        echo "  FAIL: checksum mismatch for $asset"
        rm -rf "$tmp"
        return 1
    fi
    if ! unzip -oq "$archive" -d "$extract_dir"; then
        echo "  FAIL: could not extract tree-sitter from $asset"
        rm -rf "$tmp"
        return 1
    fi

    source_bin="$extract_dir/tree-sitter"
    if [[ ! -f "$source_bin" ]]; then
        source_bin="$(find "$extract_dir" -type f -name tree-sitter -print -quit)"
    fi
    if [[ -z "$source_bin" || ! -f "$source_bin" ]]; then
        echo "  FAIL: tree-sitter binary missing from $asset"
        rm -rf "$tmp"
        return 1
    fi

    install_target="$HOME/.local/bin/tree-sitter"
    mkdir -p "$HOME/.local/bin"
    cp "$source_bin" "$install_target"
    chmod 0755 "$install_target"
    write_direct_artifact_provenance "tree-sitter" "$install_target" "$install_target" "$(dirname "$install_target")" "$url" "$TREE_SITTER_CLI_LINUX_VERSION" "$expected"
    rm -rf "$tmp"
    ensure_local_bin_on_path
    printf "  installed %-26s -> %s\n" "tree-sitter" "$install_target"
}

install_tree_sitter_cli() {
    if have tree-sitter && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "tree-sitter"
        return
    fi
    if [[ "$(uname -s)" == "Linux" && "$PM" != "brew" ]]; then
        # Alpine/musl cannot execute the glibc-linked upstream release binary
        # (same reason install_nvim_linux carves Alpine out to apk). Use the
        # native package so we never leave a non-runnable binary on PATH.
        if [[ "$(native_linux_pm)" == "apk" ]]; then
            if ! ask "Install tree-sitter CLI via apk (native Alpine package)?"; then
                printf "  skipped   %-26s\n" "tree-sitter"
                return
            fi
            local rc=0
            native_linux_pm_install apk tree-sitter || rc=$?
            if [[ "$rc" -ne 0 ]]; then
                record_install_failure "tree-sitter" apk "tree-sitter" "$rc"
                printf "  FAIL: %-26s apk add tree-sitter failed; continuing to collect install failures\n" "tree-sitter" >&2
            fi
            return
        fi
        install_tree_sitter_cli_linux
    else
        # brew (macOS + Linuxbrew): the package is `tree-sitter-cli`, NOT
        # `tree-sitter`. Homebrew split the formula -- `tree-sitter` now installs
        # only libtree-sitter (no CLI binary), so a fresh machine would be left
        # without the `tree-sitter` executable nvim-treesitter `main` needs to
        # generate/build parsers. `tree-sitter-cli` provides the `tree-sitter`
        # binary (0.26.x, matching the pinned Linux release). The PKG_TABLE brew
        # column carries that name; `binaries_for` still checks for `tree-sitter`.
        install tree-sitter "nvim-treesitter main parser CLI"
    fi
}

# Debian/Ubuntu ship python3 WITHOUT ensurepip/venv -- they live in the separate
# python3-venv + python3-pip packages. Mason installs clang-format / ruff /
# gersemi from PyPI, which runs `python3 -m venv` + pip, so on apt those tools
# fail with "ensurepip is not available" until venv + pip are present. A Linux
# PM=brew selection does not prove the active python3 is Homebrew-owned: Ubuntu's
# /usr/bin/python3 may still win PATH, so Linux always checks its native manager.
ensure_python_pip_venv() {
    command -v python3 >/dev/null 2>&1 || return 0
    if python3 -c 'import ensurepip, venv' >/dev/null 2>&1; then
        printf "  ok        %-26s venv + pip present\n" "python venv/pip"
        return
    fi
    if [[ "$(uname -s)" == "Darwin" && "$PM" == "brew" ]]; then
        return 0
    fi
    local native_pm
    native_pm="$(native_linux_pm 2>/dev/null || true)"
    case "$native_pm" in
        apt)
            if ! ask "Install python3-venv + python3-pip (Mason PyPI tools need them)?"; then
                printf "  skipped   %-26s\n" "python venv/pip"
                return
            fi
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would:    apt-get install -y python3-venv python3-pip"
                return
            fi
            if native_linux_pm_install apt python3-venv python3-pip; then
                :
            else
                local rc=$?
                record_install_failure "python venv/pip" apt "python3-venv python3-pip" "$rc"
                echo "  FAIL: python3-venv/python3-pip install failed (Mason PyPI tools will not build)"
            fi
            ;;
        dnf|zypper)
            if ! ask "Install python3-pip (Mason PyPI tools need it)?"; then
                printf "  skipped   %-26s\n" "python venv/pip"
                return
            fi
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would:    install python3-pip"
                return
            fi
            if native_linux_pm_install "$native_pm" python3-pip; then
                :
            else
                local rc=$?
                record_install_failure "python venv/pip" "$native_pm" "python3-pip" "$rc"
                echo "  FAIL: python3-pip install failed; continuing to collect install failures"
            fi
            ;;
        *)
            printf "  manual    %-26s install your distro python3-venv + python3-pip\n" "python venv/pip"
            ;;
    esac
}

pylatexenc_venv_dir() {
    printf '%s\n' "$HOME/.local/share/dotfiles/python-tools/pylatexenc"
}

pylatexenc_converter_ready() {
    local venv_dir venv_python converter
    venv_dir="$(pylatexenc_venv_dir)"
    venv_python="$venv_dir/bin/python"
    converter="$venv_dir/bin/latex2text"
    [[ -x "$venv_python" && -x "$converter" ]] || return 1
    "$venv_python" - "$PYLATEXENC_VERSION" <<'PY' >/dev/null 2>&1
import importlib.metadata
import sys

try:
    version = importlib.metadata.version("pylatexenc")
except importlib.metadata.PackageNotFoundError:
    raise SystemExit(1)

raise SystemExit(0 if version == sys.argv[1] else 1)
PY
}

write_latex2text_shim() {
    local converter="$1" shim="$2"
    mkdir -p "$(dirname "$shim")"
    cat > "$shim" <<EOF
#!/usr/bin/env sh
exec "$converter" "\$@"
EOF
    chmod 0755 "$shim"
}

install_pylatexenc_converter() {
    local venv_dir venv_python converter shim req
    venv_dir="$(pylatexenc_venv_dir)"
    venv_python="$venv_dir/bin/python"
    converter="$venv_dir/bin/latex2text"
    shim="$HOME/.local/bin/latex2text"

    if pylatexenc_converter_ready; then
        [[ -x "$shim" ]] || write_latex2text_shim "$converter" "$shim"
        ensure_local_bin_on_path
        printf "  ok        %-26s pylatexenc %s\n" "latex2text" "$PYLATEXENC_VERSION"
        return 0
    fi

    if ! have python3; then
        echo "  FAIL: python3 is required before installing latex2text"
        return 1
    fi
    if ! ask "Install latex2text via a pinned pylatexenc venv (Markdown equations)?"; then
        printf "  skipped   %-26s\n" "latex2text"
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would:    python3 -m venv $venv_dir"
        echo "  would:    pip install --require-hashes setuptools==$PYLATEXENC_BUILD_BACKEND_VERSION"
        echo "             sha256=$PYLATEXENC_BUILD_BACKEND_SHA256"
        echo "  would:    pip install --require-hashes --no-build-isolation pylatexenc==$PYLATEXENC_VERSION"
        echo "             sha256=$PYLATEXENC_SHA256"
        echo "  would:    write ~/.local/bin/latex2text shim"
        return 0
    fi

    mkdir -p "$(dirname "$venv_dir")" "$HOME/.local/bin"
    python3 -m venv "$venv_dir" || {
        echo "  FAIL: could not create pylatexenc venv at $venv_dir"
        return 1
    }

    req="$(mktemp)"
    printf 'setuptools==%s --hash=sha256:%s\n' "$PYLATEXENC_BUILD_BACKEND_VERSION" "$PYLATEXENC_BUILD_BACKEND_SHA256" > "$req"
    if ! "$venv_python" -m pip install --disable-pip-version-check --no-cache-dir --require-hashes --only-binary=:all: --no-deps -r "$req"; then
        rm -f "$req"
        echo "  FAIL: pinned setuptools install failed"
        return 1
    fi
    printf 'pylatexenc==%s --hash=sha256:%s\n' "$PYLATEXENC_VERSION" "$PYLATEXENC_SHA256" > "$req"
    if ! "$venv_python" -m pip install --disable-pip-version-check --no-cache-dir --require-hashes --no-deps --no-build-isolation -r "$req"; then
        rm -f "$req"
        echo "  FAIL: pylatexenc install failed"
        return 1
    fi
    rm -f "$req"

    if [[ ! -x "$converter" ]]; then
        echo "  FAIL: pylatexenc installed without executable latex2text"
        return 1
    fi

    write_latex2text_shim "$converter" "$shim"
    ensure_local_bin_on_path
    printf "  installed %-26s pylatexenc %s\n" "latex2text" "$PYLATEXENC_VERSION"
}

# Debian/Ubuntu's `nodejs` apt package does NOT bundle npm -- it is a separate
# `npm` package. Mason installs pyright, prettier, the bash/yaml/json language
# servers, and js-debug-adapter from npm, so without npm those Mason tools fail
# to install. brew/dnf node bundle npm; on apt and pacman it can be separate.
ensure_npm() {
    command -v node >/dev/null 2>&1 || return 0
    if command -v npm >/dev/null 2>&1; then
        printf "  ok        %-26s already installed\n" "npm"
        return
    fi
    if [[ "$PM" == "brew" ]]; then
        return 0
    fi
    local native_pm
    native_pm="$(native_linux_pm 2>/dev/null || true)"
    case "$native_pm" in
        apt)
            if ! ask "Install npm (Mason npm tools -- pyright, prettier, LSPs -- need it)?"; then
                printf "  skipped   %-26s\n" "npm"
                return
            fi
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would:    apt-get install -y npm"
                return
            fi
            if native_linux_pm_install apt npm; then
                :
            else
                local rc=$?
                record_install_failure "npm" apt "npm" "$rc"
                echo "  FAIL: npm install failed (Mason npm tools will not build)"
            fi
            ;;
        dnf|pacman|zypper)
            if ! ask "Install npm (Mason npm tools need it)?"; then
                printf "  skipped   %-26s\n" "npm"
                return
            fi
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would:    install npm via $native_pm"
                return
            fi
            if native_linux_pm_install "$native_pm" npm; then
                :
            else
                local rc=$?
                record_install_failure "npm" "$native_pm" "npm" "$rc"
                echo "  FAIL: npm install failed; continuing to collect install failures"
            fi
            ;;
        *)
            printf "  manual    %-26s install your distro npm package\n" "npm"
            ;;
    esac
}

pi_cli_node_ready() {
    command -v node >/dev/null 2>&1 || return 1
    node -e 'const [maj,min]=process.versions.node.split(".").map(Number); process.exit(maj > 22 || (maj === 22 && min >= 19) ? 0 : 1)' >/dev/null 2>&1
}

pi_cli_version() {
    local canonical="$HOME/.local/bin/pi"
    [[ -x "$canonical" ]] || return 1
    "$canonical" --version 2>/dev/null | awk 'NF { print $1; exit }'
}

pi_cli_warn_duplicate_installations() {
    audit_managed_cli_command pi pi "$HOME/.local/bin/pi"
}

verify_pi_cli_tarball_sri() {
    local tarball="$1" expected="$2"
    node - "$tarball" "$expected" <<'NODE'
const crypto = require('crypto');
const fs = require('fs');
const path = process.argv[2];
const expected = process.argv[3];
const match = /^(sha512)-([A-Za-z0-9+/]+={0,2})$/.exec(expected || '');
if (!match) process.exit(2);
const bytes = fs.readFileSync(path);
if (bytes.length === 0) process.exit(3);
const want = Buffer.from(match[2], 'base64');
const got = crypto.createHash(match[1]).update(bytes).digest();
process.exit(want.length === got.length && crypto.timingSafeEqual(want, got) ? 0 : 1);
NODE
}

install_pi_cli_verified_tarball() {
    local spec tmp pack_json metadata filename reported tarball
    spec="${PI_CLI_PACKAGE}@${PI_CLI_VERSION}"
    tmp="$(mktemp -d "${TMPDIR:-/tmp}/dotfiles-pi.XXXXXX")" || return 1
    (
        trap 'rm -rf "$tmp"' EXIT
        trap 'exit 129' HUP
        trap 'exit 130' INT
        trap 'exit 143' TERM

        pack_json="$tmp/npm-pack.json"
        if ! npm pack --ignore-scripts --json --pack-destination "$tmp" "$spec" > "$pack_json"; then
            printf "  FAIL: %-26s npm pack failed for %s\n" "pi" "$spec" >&2
            return 1
        fi

        metadata="$tmp/pack-metadata.tsv"
        if ! node - "$pack_json" > "$metadata" <<'NODE'
const fs = require('fs');
const path = require('path');
const parsed = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
if (!Array.isArray(parsed) || parsed.length !== 1) process.exit(2);
const filename = parsed[0] && parsed[0].filename;
const integrity = parsed[0] && parsed[0].integrity;
if (typeof filename !== 'string' || path.basename(filename) !== filename ||
    typeof integrity !== 'string' || /[\t\r\n]/.test(filename + integrity)) process.exit(3);
process.stdout.write(filename + '\t' + integrity + '\n');
NODE
        then
            printf "  FAIL: %-26s npm pack returned invalid metadata for %s\n" "pi" "$spec" >&2
            return 1
        fi
        IFS=$'\t' read -r filename reported < "$metadata"
        if [[ "$reported" != "$PI_CLI_INTEGRITY" ]]; then
            printf "  FAIL: %-26s npm pack metadata integrity mismatch for %s\n" "pi" "$spec" >&2
            printf "        expected %s\n" "$PI_CLI_INTEGRITY" >&2
            printf "        got      %s\n" "${reported:-<empty>}" >&2
            return 1
        fi

        tarball="$tmp/$filename"
        if [[ ! -s "$tarball" ]] || ! verify_pi_cli_tarball_sri "$tarball" "$PI_CLI_INTEGRITY"; then
            printf "  FAIL: %-26s packed tarball bytes do not match pinned SRI for %s\n" "pi" "$spec" >&2
            return 1
        fi
        if ! npm install -g --prefix "$HOME/.local" "$tarball" \
            "@earendil-works/pi-agent-core@$PI_CLI_VERSION" \
            "@earendil-works/pi-ai@$PI_CLI_VERSION" \
            "@earendil-works/pi-tui@$PI_CLI_VERSION"; then
            printf "  FAIL: %-26s npm install failed for verified local tarball %s\n" "pi" "$filename" >&2
            return 1
        fi
    )
}

install_pi_cli() {
    local current node_version
    if [[ -x "$HOME/.local/bin/pi" ]]; then
        ensure_local_bin_on_path
    fi
    current="$(pi_cli_version || true)"
    if [[ "$current" == "$PI_CLI_VERSION" ]]; then
        printf "  ok        %-26s already installed (%s)\n" "pi" "$PI_CLI_VERSION"
        pi_cli_warn_duplicate_installations
        return 0
    fi
    if ! command -v npm >/dev/null 2>&1; then
        printf "  skipped   %-26s npm is not installed\n" "pi"
        return 0
    fi
    if ! pi_cli_node_ready; then
        node_version="$(node --version 2>/dev/null || printf 'unknown')"
        printf "  skipped   %-26s requires Node >= 22.19.0; current node is %s\n" "pi" "$node_version"
        echo "            public POSIX setup supplies Node 24 through the Nix package layer"
        return 0
    fi
    if ! ask "Install Pi CLI (${PI_CLI_PACKAGE}@${PI_CLI_VERSION}, packed tarball SRI verified)?"; then
        printf "  skipped   %-26s\n" "pi"
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: npm pack --ignore-scripts --json --pack-destination <temp> ${PI_CLI_PACKAGE}@${PI_CLI_VERSION}"
        echo "         require pack metadata integrity and tarball bytes to match $PI_CLI_INTEGRITY"
        echo "  would: npm install -g --prefix \"$HOME/.local\" <verified-local-tarball> <exact same-release Pi companions>"
        return 0
    fi
    mkdir -p "$HOME/.local"
    install_pi_cli_verified_tarball || return 1
    ensure_local_bin_on_path
    current="$(pi_cli_version || true)"
    if [[ "$current" != "$PI_CLI_VERSION" ]]; then
        printf "  FAIL: %-26s expected %s after install, got %s\n" "pi" "$PI_CLI_VERSION" "${current:-<missing>}" >&2
        return 1
    fi
    printf "  installed %-26s %s\n" "pi" "$PI_CLI_VERSION"
    pi_cli_warn_duplicate_installations
}

install_chezmoi() {
    if have chezmoi && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "chezmoi"
        return
    fi

    if [[ "$PM" == "brew" && "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        install chezmoi "dotfiles config manager"
        return
    fi

    if [[ "$(uname -s)" != "Linux" ]]; then
        install chezmoi "dotfiles config manager"
        return
    fi

    local machine arch expected version_no_v asset url tmp tarball source_bin install_target
    machine="$(uname -m)"
    case "$machine" in
        x86_64|amd64)
            arch="amd64"
            expected="$CHEZMOI_LINUX_X86_64_SHA256"
            ;;
        aarch64|arm64)
            arch="arm64"
            expected="$CHEZMOI_LINUX_ARM64_SHA256"
            ;;
        *)
            printf "  manual    %-26s unsupported Linux arch: %s\n" "chezmoi" "$machine"
            echo "            install from https://github.com/twpayne/chezmoi/releases"
            return
            ;;
    esac

    version_no_v="${CHEZMOI_VERSION#v}"
    asset="chezmoi_${version_no_v}_linux_${arch}.tar.gz"
    url="https://github.com/twpayne/chezmoi/releases/download/${CHEZMOI_VERSION}/${asset}"

    if ! ask "Install chezmoi (pinned GitHub release ${CHEZMOI_VERSION}, Linux ${arch})?"; then
        printf "  skipped   %-26s\n" "chezmoi"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fsSL $url -o /tmp/$asset"
        echo "         verify sha256 $expected"
        echo "         extract chezmoi -> \$HOME/.local/bin/chezmoi"
        return
    fi

    require_downloader || return 1
    if ! have tar; then
        echo "  need      tar missing; installing extractor for chezmoi"
        install tar "extract chezmoi release archive"
        if ! have tar; then
            echo "  FAIL: need tar to extract $asset"
            return 1
        fi
    fi

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    tarball="$tmp/$asset"
    if ! curl -fsSL "$url" -o "$tarball"; then
        echo "  FAIL: chezmoi download failed from $url"
        rm -rf "$tmp"
        return 1
    fi
    if ! verify_sha256 "$tarball" "$expected"; then
        echo "  FAIL: checksum mismatch for $asset"
        rm -rf "$tmp"
        return 1
    fi
    if ! tar -xzf "$tarball" -C "$tmp"; then
        echo "  FAIL: could not extract chezmoi from $asset"
        rm -rf "$tmp"
        return 1
    fi

    source_bin="$tmp/chezmoi"
    if [[ ! -f "$source_bin" ]]; then
        source_bin="$(find "$tmp" -type f -name chezmoi -print -quit)"
    fi
    if [[ -z "$source_bin" || ! -f "$source_bin" ]]; then
        echo "  FAIL: chezmoi binary missing from $asset"
        rm -rf "$tmp"
        return 1
    fi

    install_target="$HOME/.local/bin/chezmoi"
    mkdir -p "$HOME/.local/bin"
    cp "$source_bin" "$install_target"
    chmod 0755 "$install_target"
    write_direct_artifact_provenance "chezmoi" "$install_target" "$install_target" "$(dirname "$install_target")" "$url" "$CHEZMOI_VERSION" "$expected"
    rm -rf "$tmp"
    ensure_local_bin_on_path
    printf "  installed %-26s %s -> %s\n" "chezmoi" "$CHEZMOI_VERSION" "$install_target"
}

zsh_plugin_root() {
    printf '%s\n' "$HOME/.local/share/dotfiles/zsh-plugins"
}

zsh_plugin_git() {
    env GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
        GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=Never \
        git -c core.hooksPath=/dev/null -c core.fsmonitor=false \
        -c core.untrackedCache=false -c credential.helper= "$@"
}

zsh_plugin_ok() {
    local target="$1" repo="$2" expected_commit="$3" plugin_file="$4"
    local current origin root status
    [[ -d "$target/.git" ]] || return 1
    [[ -f "$target/$plugin_file" && ! -L "$target/$plugin_file" ]] || return 1
    root="$(zsh_plugin_git -C "$target" rev-parse --show-toplevel 2>/dev/null)" || return 1
    [[ -n "$root" && "$(cd "$root" && pwd -P)" == "$(cd "$target" && pwd -P)" ]] || return 1
    origin="$(zsh_plugin_git -C "$target" remote get-url origin 2>/dev/null)" || return 1
    [[ "${origin%.git}" == "${repo%.git}" ]] || return 1
    current="$(zsh_plugin_git -C "$target" rev-parse --verify HEAD 2>/dev/null)" || return 1
    [[ "$current" == "$expected_commit" ]] || return 1
    zsh_plugin_git -C "$target" ls-files --error-unmatch -- "$plugin_file" >/dev/null 2>&1 || return 1
    status="$(zsh_plugin_git -C "$target" status --porcelain=v1 --untracked-files=all --ignored 2>/dev/null)" || return 1
    [[ -z "$status" ]]
}

install_zsh_plugin_repo() {
    local name="$1" repo="$2" ref="$3" expected_commit="$4" plugin_file="$5"
    local root target repo_root helper
    root="$(zsh_plugin_root)"
    target="$root/$name"

    if zsh_plugin_ok "$target" "$repo" "$expected_commit" "$plugin_file"; then
        printf "  ok        %-26s %s\n" "$name" "$ref"
        return 0
    fi

    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: neutralize any unproved $target payload"
        echo "         fetch reviewed ref $ref at exact commit $expected_commit from $repo into a sibling stage"
        echo "         prove origin/HEAD/cleanliness/$plugin_file, then publish atomically"
        return 0
    fi
    if ! have git; then
        printf "  manual    %-26s git is required for pinned plugin install\n" "$name"
        return 1
    fi

    repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
    helper="$repo_root/scripts/ensure-pinned-zsh-plugin.sh"
    if [[ ! -r "$helper" ]]; then
        printf "  FAIL: %-26s publisher helper missing: %s\n" "$name" "$helper" >&2
        return 1
    fi
    /bin/bash "$helper" "$name" "$repo" "$ref" "$expected_commit" "$plugin_file" "$target"
}

install_zsh_plugins() {
    # fzf-tab gives the fzf-driven fuzzy Tab completion menu; zsh-autosuggestions
    # gives the inline gray history hint. See shells/zshrc + CLAUDE.md invariant 13.
    local root fzf_tab_dir autosuggestions_dir
    root="$(zsh_plugin_root)"
    fzf_tab_dir="$root/fzf-tab"
    autosuggestions_dir="$root/zsh-autosuggestions"

    if zsh_plugin_ok "$fzf_tab_dir" "https://github.com/Aloxaf/fzf-tab.git" "$FZF_TAB_COMMIT" "fzf-tab.plugin.zsh" &&
        zsh_plugin_ok "$autosuggestions_dir" "https://github.com/zsh-users/zsh-autosuggestions.git" "$ZSH_AUTOSUGGESTIONS_COMMIT" "zsh-autosuggestions.zsh"; then
        printf "  ok        %-26s pinned refs already installed\n" "zsh plugins"
        return 0
    fi
    if ! ask "Install fzf-tab + zsh-autosuggestions (repo-managed pinned refs)?"; then
        printf "  skipped   %-26s\n" "zsh plugins"
        return 0
    fi

    # Attempt BOTH plugins (one failing must not skip the other) and do NOT
    # swallow the result with `|| true` -- that let setup report success with a
    # plugin absent. Each `|| rc=1` absorbs a failure (so `set -e` -- enabled at
    # the top of this file -- does not abort mid-list), then we emit a FAIL:
    # marker so CI catches it. We deliberately still RETURN 0 (continue): zsh
    # plugins are non-critical (the shell works, just without the fuzzy-Tab menu /
    # gray hint), so login-shell adoption and the rest of setup still run. This is
    # the same emit-FAIL-and-continue pattern as install_ghostty_linux.
    local rc=0
    install_zsh_plugin_repo \
        "fzf-tab" \
        "https://github.com/Aloxaf/fzf-tab.git" \
        "$FZF_TAB_VERSION" \
        "$FZF_TAB_COMMIT" \
        "fzf-tab.plugin.zsh" || rc=1
    install_zsh_plugin_repo \
        "zsh-autosuggestions" \
        "https://github.com/zsh-users/zsh-autosuggestions.git" \
        "$ZSH_AUTOSUGGESTIONS_VERSION" \
        "$ZSH_AUTOSUGGESTIONS_COMMIT" \
        "zsh-autosuggestions.zsh" || rc=1
    if [[ "$rc" -ne 0 ]]; then
        record_install_failure "zsh plugins" git "fzf-tab/zsh-autosuggestions" "$rc"
        printf "  FAIL: %-26s one or more pinned zsh plugins failed to install\n" "zsh plugins" >&2
    fi
    return 0
}

install_gh_dash_extension() {
    # gh-dash is a gh CLI extension (there is no brew/apt package), pinned to
    # GH_DASH_VERSION. It is only useful once `gh auth login` has run, and an
    # UNAUTHENTICATED `gh extension install` hits GitHub's anonymous API rate
    # limit and fails -- so we require auth before touching the extension. The
    # chezmoi-managed config is applied regardless; this step only gates the
    # extension binary. When gh is unauthenticated we skip cleanly (NOT a FAIL)
    # and tell the user to authenticate and rerun. An authenticated install
    # failure still emits a FAIL: marker for CI but does not abort setup (same
    # emit-FAIL-and-continue pattern as install_zsh_plugins).
    if ! have gh; then
        printf "  skipped   %-26s gh CLI not installed (gh-dash is a gh extension)\n" "gh-dash"
        return 0
    fi
    if ! gh auth status >/dev/null 2>&1; then
        printf "  skipped   %-26s run 'gh auth login' then rerun to install gh-dash\n" "gh-dash"
        return 0
    fi

    # gh-dash is a binary extension, so gh's --pin contract accepts a release
    # tag (commit refs are for script extensions). Verify the reviewed annotated
    # tag -> commit mapping before installation, then pass the tag to gh.
    # Verify the *installed* tag, not merely presence -- a different release
    # must be re-pinned, and a plain install of an already-present extension
    # errors, so a mismatch takes the remove+install path.
    local list installed_ver reinstall=0
    list="$(gh extension list 2>/dev/null || true)"
    if printf '%s\n' "$list" | grep -q 'dlvhdr/gh-dash'; then
        installed_ver="$(printf '%s\n' "$list" \
            | awk '{ for (i = 1; i <= NF; i++) if ($i == "dlvhdr/gh-dash") { print $(i + 1); exit } }')"
        if [[ "$installed_ver" == "$GH_DASH_VERSION" ]]; then
            printf "  ok        %-26s already installed (%s -> %s)\n" "gh-dash" "$GH_DASH_VERSION" "$GH_DASH_COMMIT"
            return 0
        fi
        reinstall=1
    fi

    local prompt
    if [[ "$reinstall" -eq 1 ]]; then
        prompt="Re-pin gh-dash to $GH_DASH_VERSION (currently ${installed_ver:-unknown})?"
    else
        prompt="Install gh-dash (pinned gh extension $GH_DASH_VERSION)?"
    fi
    if ! ask "$prompt"; then
        printf "  skipped   %-26s\n" "gh-dash"
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: verify $GH_DASH_VERSION tag object $GH_DASH_TAG_OBJECT peels to $GH_DASH_COMMIT"
        if [[ "$reinstall" -eq 1 ]]; then
            echo "  would: gh extension remove dash && gh extension install dlvhdr/gh-dash --pin $GH_DASH_VERSION"
        else
            echo "  would: gh extension install dlvhdr/gh-dash --pin $GH_DASH_VERSION"
        fi
        return 0
    fi

    local tag_object peeled_commit
    tag_object="$(gh api "repos/dlvhdr/gh-dash/git/ref/tags/$GH_DASH_VERSION" --jq .object.sha 2>/dev/null || true)"
    if [[ "$tag_object" != "$GH_DASH_TAG_OBJECT" ]]; then
        record_install_failure "gh-dash" gh "dlvhdr/gh-dash@$GH_DASH_VERSION" tag-object
        printf "  FAIL: %-26s tag object mismatch: got %s expected %s\n" "gh-dash" "${tag_object:-<empty>}" "$GH_DASH_TAG_OBJECT" >&2
        return 0
    fi
    peeled_commit="$(gh api "repos/dlvhdr/gh-dash/git/tags/$tag_object" --jq .object.sha 2>/dev/null || true)"
    if [[ "$peeled_commit" != "$GH_DASH_COMMIT" ]]; then
        record_install_failure "gh-dash" gh "dlvhdr/gh-dash@$GH_DASH_VERSION" peeled-commit
        printf "  FAIL: %-26s peeled commit mismatch: got %s expected %s\n" "gh-dash" "${peeled_commit:-<empty>}" "$GH_DASH_COMMIT" >&2
        return 0
    fi

    if [[ "$reinstall" -eq 1 ]]; then
        local remove_rc=0
        gh extension remove dash >/dev/null 2>&1 || remove_rc=$?
        if [[ "$remove_rc" -ne 0 ]]; then
            record_install_failure "gh-dash" gh remove:dash "$remove_rc"
            printf "  FAIL: %-26s could not remove the mismatched extension\n" "gh-dash" >&2
            return 0
        fi
    fi
    if gh extension install dlvhdr/gh-dash --pin "$GH_DASH_VERSION"; then
        if [[ "$reinstall" -eq 1 ]]; then
            printf "  installed %-26s %s -> %s (re-pinned)\n" "gh-dash" "$GH_DASH_VERSION" "$GH_DASH_COMMIT"
        else
            printf "  installed %-26s %s -> %s\n" "gh-dash" "$GH_DASH_VERSION" "$GH_DASH_COMMIT"
        fi
    else
        local rc=$?
        record_install_failure "gh-dash" gh "dlvhdr/gh-dash@$GH_DASH_VERSION" "$rc"
        printf "  FAIL: %-26s gh extension install dlvhdr/gh-dash --pin %s failed\n" "gh-dash" "$GH_DASH_VERSION" >&2
    fi
    return 0
}

tmux_plugin_root() {
    printf '%s\n' "$HOME/.local/share/dotfiles/tmux-plugins"
}

tmux_plugin_ok() {
    local target="$1" expected_commit="$2" required_file="$3" current
    [[ -d "$target/.git" ]] || return 1
    [[ -r "$target/$required_file" ]] || return 1
    current="$(git -C "$target" rev-parse HEAD 2>/dev/null || true)"
    [[ "$current" == "$expected_commit" ]]
}

install_tmux_plugin_repo() {
    local name="$1" repo="$2" expected_commit="$3" dirname="$4" required_file="$5"
    local root target backup current
    root="$(tmux_plugin_root)"
    target="$root/$dirname"

    if tmux_plugin_ok "$target" "$expected_commit" "$required_file"; then
        printf "  ok        %-26s %s\n" "$name" "$expected_commit"
        return 0
    fi

    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: git init/fetch exact commit $expected_commit from $repo"
        echo "         checkout -> $target and verify $required_file"
        return 0
    fi
    if ! have git; then
        printf "  manual    %-26s git is required for pinned plugin install\n" "$name"
        return 1
    fi

    mkdir -p "$root"
    if [[ -e "$target" && ! -d "$target/.git" ]]; then
        backup="$(unique_backup_path "$target")"
        mv "$target" "$backup"
        printf "  backup    %-26s %s\n" "$name" "$backup"
    fi

    if [[ ! -d "$target/.git" ]]; then
        mkdir -p "$target"
        git -C "$target" init -q || {
            printf "  WARN: could not init %-20s at %s\n" "$name" "$target"
            return 1
        }
        git -C "$target" remote add origin "$repo" >/dev/null 2>&1 || true
    else
        if git -C "$target" remote get-url origin >/dev/null 2>&1; then
            git -C "$target" remote set-url origin "$repo"
        else
            git -C "$target" remote add origin "$repo"
        fi
    fi

    git -C "$target" fetch --depth 1 origin "$expected_commit" >/dev/null 2>&1 || {
        printf "  WARN: could not fetch %-18s commit %s\n" "$name" "$expected_commit"
        return 1
    }
    git -C "$target" checkout --force FETCH_HEAD >/dev/null 2>&1 || {
        printf "  WARN: could not checkout %-15s commit %s\n" "$name" "$expected_commit"
        return 1
    }

    current="$(git -C "$target" rev-parse HEAD 2>/dev/null || true)"
    if [[ "$current" != "$expected_commit" ]]; then
        printf "  FAIL: %-26s got commit %s, expected %s\n" "$name" "${current:-unknown}" "$expected_commit"
        return 1
    fi
    if [[ ! -r "$target/$required_file" ]]; then
        printf "  FAIL: %-26s missing %s\n" "$name" "$required_file"
        return 1
    fi
    printf "  installed %-26s %s\n" "$name" "$expected_commit"
}

install_tmux_plugins() {
    # POSIX tmux loads TPM + the functional plugins (sensible/yank/resurrect/
    # continuum) from tmux.posix.conf. The Rose Pine status bar is a repo-owned
    # generated config, NOT a plugin, so rose-pine/tmux is no longer installed.
    # Missing plugins are a real provisioning failure (session save/restore and
    # sane defaults depend on them), so this fails closed.
    local root tpm_dir rc=0
    root="$(tmux_plugin_root)"
    tpm_dir="$root/tpm"

    if tmux_plugin_ok "$tpm_dir" "$TPM_COMMIT" "tpm" &&
        tmux_plugin_ok "$root/tmux-sensible" "$TMUX_SENSIBLE_COMMIT" "sensible.tmux" &&
        tmux_plugin_ok "$root/tmux-yank" "$TMUX_YANK_COMMIT" "yank.tmux" &&
        tmux_plugin_ok "$root/tmux-resurrect" "$TMUX_RESURRECT_COMMIT" "resurrect.tmux" &&
        tmux_plugin_ok "$root/tmux-continuum" "$TMUX_CONTINUUM_COMMIT" "continuum.tmux"; then
        printf "  ok        %-26s pinned refs already installed\n" "tmux plugins"
        return 0
    fi
    if ! ask "Install TPM + tmux-sensible/yank/resurrect/continuum (repo-managed pinned refs)?"; then
        printf "  skipped   %-26s\n" "tmux plugins"
        return 0
    fi

    install_tmux_plugin_repo \
        "tpm" \
        "https://github.com/tmux-plugins/tpm.git" \
        "$TPM_COMMIT" \
        "tpm" \
        "tpm" || rc=1
    install_tmux_plugin_repo \
        "tmux-sensible" \
        "https://github.com/tmux-plugins/tmux-sensible.git" \
        "$TMUX_SENSIBLE_COMMIT" \
        "tmux-sensible" \
        "sensible.tmux" || rc=1
    install_tmux_plugin_repo \
        "tmux-yank" \
        "https://github.com/tmux-plugins/tmux-yank.git" \
        "$TMUX_YANK_COMMIT" \
        "tmux-yank" \
        "yank.tmux" || rc=1
    install_tmux_plugin_repo \
        "tmux-resurrect" \
        "https://github.com/tmux-plugins/tmux-resurrect.git" \
        "$TMUX_RESURRECT_COMMIT" \
        "tmux-resurrect" \
        "resurrect.tmux" || rc=1
    install_tmux_plugin_repo \
        "tmux-continuum" \
        "https://github.com/tmux-plugins/tmux-continuum.git" \
        "$TMUX_CONTINUUM_COMMIT" \
        "tmux-continuum" \
        "continuum.tmux" || rc=1
    if [[ "$rc" -ne 0 ]]; then
        printf "  FAIL: %-26s one or more pinned tmux plugins failed to install\n" "tmux plugins" >&2
    fi
    return "$rc"
}

homebrew_cask_installed() {
    local cask="$1"
    brew list --cask --versions "$cask" >/dev/null 2>&1
}

install_ghostty_macos() {
    if have ghostty || homebrew_cask_installed ghostty; then
        printf "  ok        %-26s already installed\n" "ghostty"
        return
    fi
    if ! ask "Install ghostty (macOS terminal) via Homebrew cask?"; then
        printf "  skipped   %-26s\n" "ghostty"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: brew install --cask ghostty"
        return
    fi
    if brew install --cask ghostty; then
        :
    else
        local rc=$?
        record_install_failure "ghostty" brew "cask:ghostty" "$rc"
        echo "  FAIL: ghostty cask install failed; continuing to collect install failures"
    fi
}

install_wezterm_macos() {
    if have wezterm || homebrew_cask_installed wezterm; then
        printf "  ok        %-26s already installed\n" "wezterm"
        return
    fi
    if ! ask "Install WezTerm (terminal) via Homebrew cask?"; then
        printf "  skipped   %-26s\n" "wezterm"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: brew install --cask wezterm"
        return
    fi
    if brew install --cask wezterm; then
        :
    else
        local rc=$?
        record_install_failure "wezterm" brew "cask:wezterm" "$rc"
        echo "  FAIL: wezterm cask install failed; continuing to collect install failures"
    fi
}

# AeroSpace: i3-like tiling WM, macOS only. Official tap cask (nikitabobko/tap).
# NOT nixpkgs. Needs an Accessibility (TCC) grant on first launch -- unscriptable.
install_aerospace_macos() {
    if have aerospace || homebrew_cask_installed aerospace; then
        printf "  ok        %-26s already installed\n" "aerospace"
        return
    fi
    if ! ask "Install AeroSpace (tiling WM, macOS) via Homebrew cask (nikitabobko/tap)?"; then
        printf "  skipped   %-26s\n" "aerospace"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: brew install --cask nikitabobko/tap/aerospace"
        return
    fi
    if brew install --cask nikitabobko/tap/aerospace; then
        :
    else
        local rc=$?
        record_install_failure "aerospace" brew "cask:nikitabobko/tap/aerospace" "$rc"
        echo "  FAIL: aerospace cask install failed; continuing to collect install failures"
    fi
    echo "  note: grant AeroSpace Accessibility permission (System Settings ->"
    echo "        Privacy & Security -> Accessibility). This TCC grant is unscriptable."
}

# ---- Package-name resolution (Bash 3.2-safe, no associative arrays) ----------
#
# Table format: lines of "tool|brew|apt|dnf|pacman|zypper|apk".
# Empty field == "not available in that PM, will fall through to manual hint".
# A leading "-" on a field marks it as flag-bearing (e.g. "-cask font-…" for brew).
#
# Keep this list in sync with the install() calls below.
PKG_TABLE=$(cat <<'EOF'
git|git|git|git|git|git|git
nvim|neovim|neovim|neovim|neovim|neovim|neovim
make|make|make|make|make|make|make
cmake|cmake|cmake|cmake|cmake|cmake|cmake
rg|ripgrep|ripgrep|ripgrep|ripgrep|ripgrep|ripgrep
fd|fd|fd-find|fd-find|fd|fd|fd
fzf|fzf|fzf|fzf|fzf|fzf|fzf
lsd|lsd|lsd|lsd|lsd|lsd|lsd
zoxide|zoxide|zoxide|zoxide|zoxide|zoxide|zoxide
chezmoi|chezmoi|||||
lazygit|lazygit|||||
starship|starship|||||
tmux|tmux|tmux|tmux|tmux|tmux|tmux
zsh|zsh|zsh|zsh|zsh|zsh|zsh
python3|python@3.12|python3|python3|python|python311|python3
node|node|nodejs|nodejs|nodejs|nodejs|nodejs
tree-sitter|tree-sitter-cli|||||
shellcheck|shellcheck|shellcheck|ShellCheck|shellcheck|ShellCheck|shellcheck
jq|jq|jq|jq|jq|jq|jq
gh|gh|gh|gh|github-cli|gh|github-cli
herdr|herdr|||||
hyperfine|hyperfine|hyperfine|hyperfine|hyperfine|hyperfine|hyperfine
taplo|taplo|||taplo-cli||
yamllint|yamllint|yamllint|yamllint|yamllint|yamllint|yamllint
editorconfig-checker|editorconfig-checker|||editorconfig-checker||
xclip||xclip|xclip|xclip|xclip|xclip
wl-copy||wl-clipboard|wl-clipboard|wl-clipboard|wl-clipboard|wl-clipboard
unzip|unzip|unzip|unzip|unzip|unzip|unzip
tar|gnu-tar|tar|tar|tar|tar|tar
fc-cache|fontconfig|fontconfig|fontconfig|fontconfig|fontconfig|fontconfig
EOF
)

pkg_for_pm() {
    local tool="$1" pm="${2:-$PM}"
    if [[ "$pm" == "apk" ]]; then
        case "$tool" in
            lazygit|starship|tree-sitter)
                printf '%s\n' "$tool"
                return
                ;;
        esac
    fi
    local row
    row=$(printf '%s\n' "$PKG_TABLE" | awk -F'|' -v t="$tool" '$1==t{print; exit}')
    [[ -z "$row" ]] && { echo ""; return; }
    local idx
    case "$pm" in
        brew)   idx=2 ;;
        apt)    idx=3 ;;
        dnf)    idx=4 ;;
        pacman) idx=5 ;;
        zypper) idx=6 ;;
        apk)    idx=7 ;;
        *)      idx=2 ;;
    esac
    printf '%s\n' "$row" | awk -F'|' -v i="$idx" '{print $i}'
}

pkg_for() {
    pkg_for_pm "$1" "$PM"
}

pm_install() {
    local pkgs=("$@")
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: $PM install ${pkgs[*]}"; return 0
    fi
    case "$PM" in
        brew)   brew install "${pkgs[@]}" ;;
        apt)    apt_get_noninteractive update -qq || echo "  WARN: apt-get update failed; installing from the existing apt cache" >&2
                apt_get_noninteractive install -y "${pkgs[@]}" ;;
        dnf)    maybe_sudo dnf install -y "${pkgs[@]}" ;;
        pacman) maybe_sudo pacman -S --noconfirm "${pkgs[@]}" ;;
        zypper) maybe_sudo zypper install -y "${pkgs[@]}" ;;
        apk)    maybe_sudo apk add "${pkgs[@]}" ;;
    esac
    local rc=$?
    if [[ "$rc" -ne 0 ]]; then
        echo "  WARN: $PM install of '${pkgs[*]}' returned $rc" >&2
    fi
    return $rc
}

native_linux_pm_install() {
    local native_pm="$1"; shift
    local pkgs=("$@")
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: $native_pm install ${pkgs[*]}"; return 0
    fi
    case "$native_pm" in
        apt)    apt_get_noninteractive update -qq || echo "  WARN: apt-get update failed; installing from the existing apt cache" >&2
                apt_get_noninteractive install -y "${pkgs[@]}" ;;
        dnf)    maybe_sudo dnf install -y "${pkgs[@]}" ;;
        pacman) maybe_sudo pacman -S --noconfirm --needed "${pkgs[@]}" ;;
        zypper) maybe_sudo zypper install -y "${pkgs[@]}" ;;
        apk)    maybe_sudo apk add "${pkgs[@]}" ;;
        *)      return 1 ;;
    esac
}

catalog_tools() {
    if [[ -n "${INSTALL_DEPS_UPDATE_TOOLS:-}" ]]; then
        printf '%s\n' "$INSTALL_DEPS_UPDATE_TOOLS"
        return
    fi
    printf '%s\n' "$PKG_TABLE" | awk -F'|' 'NF { print $1 }'
}

update_tool_present() {
    local tool="$1" bins
    bins="$(binaries_for "$tool")"
    # shellcheck disable=SC2086  # $bins is intentional word-splitting
    have_any $bins
}

update_tool_source() {
    local tool="$1" bins bin source
    bins="$(binaries_for "$tool")"
    # shellcheck disable=SC2086  # $bins is intentional word-splitting
    for bin in $bins; do
        source="$(command -v "$bin" 2>/dev/null || true)"
        if [[ -n "$source" ]]; then
            printf '%s\n' "$source"
            return 0
        fi
    done
    return 1
}

pm_pkg_installed() {
    local pm="$1" pkg="$2" pkg_name
    pkg_name="${pkg##*/}"
    case "$pm" in
        brew)
            brew list --formula "$pkg_name" >/dev/null 2>&1
            ;;
        apt)
            dpkg-query -W -f='${Status}' "$pkg" 2>/dev/null | grep -q 'install ok installed'
            ;;
        dnf|zypper)
            rpm -q "$pkg" >/dev/null 2>&1
            ;;
        pacman)
            pacman -Q "$pkg" >/dev/null 2>&1
            ;;
        apk)
            apk info -e "$pkg" >/dev/null 2>&1
            ;;
        *)
            return 1
            ;;
    esac
}

real_source_path() {
    local source="$1" dir base physical_dir
    if have realpath; then
        realpath "$source" 2>/dev/null && return 0
    fi
    if have python3; then
        python3 - "$source" <<'PY' 2>/dev/null && return 0
import os
import sys
print(os.path.realpath(sys.argv[1]))
PY
    fi
    dir="$(dirname "$source")"
    base="$(basename "$source")"
    if physical_dir="$(cd "$dir" 2>/dev/null && pwd -P)"; then
        printf '%s/%s\n' "$physical_dir" "$base"
        return 0
    fi
    printf '%s\n' "$source"
}

physical_path() {
    local source="$1" dir base physical_dir
    dir="$(dirname "$source")"
    base="$(basename "$source")"
    if physical_dir="$(cd "$dir" 2>/dev/null && pwd -P)"; then
        printf '%s/%s\n' "$physical_dir" "$base"
        return 0
    fi
    printf '%s\n' "$source"
}

path_under() {
    local path="$1" prefix="$2"
    [[ -n "$path" && -n "$prefix" ]] || return 1
    prefix="${prefix%/}"
    case "$path" in
        "$prefix"|"$prefix"/*) return 0 ;;
        *) return 1 ;;
    esac
}

package_name_matches() {
    local actual="$1" expected="$2" expected_name
    expected_name="${expected##*/}"
    [[ "$actual" == "$expected" || "$actual" == "$expected_name" ]] && return 0
    awk -v a="$actual" -v b="$expected" -v c="$expected_name" 'BEGIN { exit !((tolower(a)==tolower(b)) || (tolower(a)==tolower(c))) }'
}

brew_prefix() {
    local brew_bin
    brew_bin="$(homebrew_bin 2>/dev/null || true)"
    if [[ -n "$brew_bin" ]]; then
        "$brew_bin" --prefix 2>/dev/null || true
    fi
}

brew_formula_owns_tool_source() {
    local pkg="$1" source="$2" pkg_name files file real_source real_file
    pkg_name="${pkg##*/}"
    real_source="$(real_source_path "$source")"
    if ! files="$(brew list --formula "$pkg_name" 2>/dev/null)"; then
        return 2
    fi
    while IFS= read -r file; do
        [[ -n "$file" ]] || continue
        if [[ "$file" == "$source" || "$file" == "$real_source" ]]; then
            return 0
        fi
        real_file="$(real_source_path "$file")"
        if [[ "$real_file" == "$source" || "$real_file" == "$real_source" ]]; then
            return 0
        fi
    done <<EOF
$files
EOF
    return 1
}

brew_claims_tool_source() {
    local tool="$1" pkg="$2" source="$3" real_source source_physical prefix prefix_real owns_rc
    prefix="$(brew_prefix)"
    [[ -n "$source" && -n "$prefix" ]] || return 1
    real_source="$(real_source_path "$source")"
    source_physical="$(physical_path "$source")"
    prefix_real="$(real_source_path "$prefix")"
    if path_under "$source" "$prefix" || path_under "$source_physical" "$prefix_real"; then
        path_under "$real_source" "$prefix" || path_under "$real_source" "$prefix_real" || return 4
        if pm_pkg_installed brew "$pkg"; then
            if brew_formula_owns_tool_source "$pkg" "$source"; then
                return 0
            fi
            owns_rc=$?
            [[ "$owns_rc" -eq 2 ]] && return 2
            return 3
        fi
        return 2
    fi
    return 1
}

brew_formula_owning_tool_source() {
    local source="$1" prefix cellar real_source source_physical relative formula
    prefix="$(brew_prefix)"
    cellar="$(brew --cellar 2>/dev/null || true)"
    [[ -n "$source" && -n "$prefix" && -n "$cellar" ]] || return 1
    [[ "$cellar" == /* ]] || return 1
    cellar="$(real_source_path "$cellar")"
    source_physical="$(physical_path "$source")"
    if ! path_under "$source" "$prefix" && ! path_under "$source_physical" "$(real_source_path "$prefix")"; then
        return 1
    fi
    real_source="$(real_source_path "$source")"
    path_under "$real_source" "$cellar" || return 1
    relative="${real_source#"${cellar%/}/"}"
    [[ "$relative" == */* ]] || return 1
    formula="${relative%%/*}"
    [[ -n "$formula" ]] || return 1
    pm_pkg_installed brew "$formula" || return 1
    brew_formula_owns_tool_source "$formula" "$source" || return 1
    printf '%s\n' "$formula"
}

dpkg_claims_tool_source() {
    local pkg="$1" source="$2" real_source query owner
    real_source="$(real_source_path "$source")"
    for query in "$source" "$real_source"; do
        [[ -n "$query" ]] || continue
        owner="$(dpkg-query -S "$query" 2>/dev/null | awk -F: 'NR==1 { sub(/:.*/, "", $1); print $1 }')"
        if [[ -n "$owner" ]] && package_name_matches "$owner" "$pkg"; then
            return 0
        fi
    done
    return 1
}

rpm_claims_tool_source() {
    local pkg="$1" source="$2" real_source query owner
    real_source="$(real_source_path "$source")"
    for query in "$source" "$real_source"; do
        [[ -n "$query" ]] || continue
        owner="$(rpm -qf --qf '%{NAME}\n' "$query" 2>/dev/null | awk 'NR==1 { print }')"
        if [[ -n "$owner" ]] && package_name_matches "$owner" "$pkg"; then
            return 0
        fi
    done
    return 1
}

pacman_claims_tool_source() {
    local pkg="$1" source="$2" real_source query owner
    real_source="$(real_source_path "$source")"
    for query in "$source" "$real_source"; do
        [[ -n "$query" ]] || continue
        owner="$(pacman -Qo -q "$query" 2>/dev/null | awk 'NR==1 { print }')"
        if [[ -n "$owner" ]] && package_name_matches "$owner" "$pkg"; then
            return 0
        fi
    done
    return 1
}

apk_claims_tool_source() {
    local pkg="$1" source="$2" real_source query owner
    real_source="$(real_source_path "$source")"
    for query in "$source" "$real_source"; do
        [[ -n "$query" ]] || continue
        owner="$(apk info --who-owns "$query" 2>/dev/null | awk '{
            for (i = 1; i <= NF; i++) {
                if ($i == "by") {
                    print $(i + 1)
                    exit
                }
            }
        }')"
        owner="${owner%-[0-9]*}"
        if [[ -n "$owner" ]] && package_name_matches "$owner" "$pkg"; then
            return 0
        fi
    done
    return 1
}

native_pm_claims_tool_source() {
    local pm="$1" pkg="$2" source="$3"
    case "$pm" in
        apt) dpkg_claims_tool_source "$pkg" "$source" ;;
        dnf|zypper) rpm_claims_tool_source "$pkg" "$source" ;;
        pacman) pacman_claims_tool_source "$pkg" "$source" ;;
        apk) apk_claims_tool_source "$pkg" "$source" ;;
        *) return 1 ;;
    esac
}

accepted_system_tool_source() {
    local tool="$1" source="$2" real_source
    real_source="$(real_source_path "$source")"
    case "$(uname -s):$tool:$real_source" in
        Darwin:zsh:/bin/zsh) return 0 ;;
        *) return 1 ;;
    esac
}

# Report physically distinct managed commands that can make the same command
# name change meaning when PATH order changes. Base-OS directories are retained
# as fallbacks and are not actionable duplicates. Classification is read-only:
# competing commands are never executed, and cleanup is emitted only when the
# owning manager proves the exact package/command relationship.
managed_cli_system_fallback_path() {
    local source="$1" physical
    physical="$(physical_path "$source")"
    case "$physical" in
        /bin/* | /sbin/* | /usr/bin/* | /usr/sbin/*) return 0 ;;
        *) return 1 ;;
    esac
}

npm_package_owning_command_source() {
    local source="$1" expected_package="${2:-}" source_dir npm_candidate prefix root real relative package
    NPM_COMMAND_OWNER=""
    NPM_COMMAND_OWNER_NPM=""
    NPM_COMMAND_OWNER_PREFIX=""
    source_dir="${source%/*}"
    npm_candidate="$source_dir/npm"
    [[ "${source##*/}" != "npm" && -x "$npm_candidate" ]] || return 1
    prefix="$("$npm_candidate" prefix -g 2>/dev/null | awk 'NF { print; exit }' || true)"
    [[ -n "$prefix" ]] || return 1
    prefix="${prefix%/}"
    path_under "$source" "$prefix/bin" || return 1
    if [[ -n "$expected_package" && "${source##*/}" == "pi" ]]; then
        package="$expected_package"
    else
        root="$prefix/lib/node_modules"
        real="$(real_source_path "$source")"
        path_under "$real" "$root" || return 1
        relative="${real#"${root%/}/"}"
        if [[ "$relative" == @*/*/* ]]; then
            package="${relative%%/*}/${relative#*/}"
            package="${package%/*}"
        else
            package="${relative%%/*}"
        fi
    fi
    [[ -n "$package" ]] || return 1
    "$npm_candidate" list --global --prefix "$prefix" --depth=0 "$package" >/dev/null 2>&1 || return 1
    NPM_COMMAND_OWNER="$package"
    NPM_COMMAND_OWNER_NPM="$npm_candidate"
    NPM_COMMAND_OWNER_PREFIX="$prefix"
    return 0
}

managed_cli_report_duplicate_owner() {
    local duplicate="$1" tool="$2" expected_npm_package="" formula brew_bin
    if formula="$(brew_formula_owning_tool_source "$duplicate" 2>/dev/null)"; then
        brew_bin="$(homebrew_bin 2>/dev/null || printf 'brew')"
        printf '        owner=brew package=%s\n' "$formula" >&2
        printf '        cleanup (same user, no sudo): %q uninstall %q\n' "$brew_bin" "$formula" >&2
        return 0
    fi
    [[ "$tool" == "pi" ]] && expected_npm_package="$PI_CLI_PACKAGE"
    if npm_package_owning_command_source "$duplicate" "$expected_npm_package"; then
        printf '        owner=npm package=%s\n' "$NPM_COMMAND_OWNER" >&2
        printf '        cleanup (same user, no sudo): %q uninstall --global --prefix %q %q\n' \
            "$NPM_COMMAND_OWNER_NPM" "$NPM_COMMAND_OWNER_PREFIX" "$NPM_COMMAND_OWNER" >&2
        return 0
    fi
    if nix_owns_tool_source "$duplicate"; then
        echo "        owner=nix; reconcile it through setup.sh or the declaring Nix profile." >&2
        return 0
    fi
    echo "        owner=unknown; remove it only through its original package manager." >&2
}

audit_managed_cli_command() {
    local tool="$1" binary="$2" selected="${3:-}" key candidate candidate_real selected_real
    local -a duplicates=()
    key="|${tool}:${binary}|"
    [[ "$MANAGED_CLI_AUDITED" != *"$key"* ]] || return 0
    MANAGED_CLI_AUDITED="${MANAGED_CLI_AUDITED}${tool}:${binary}|"

    if [[ -z "$selected" ]]; then
        selected="$(command -v "$binary" 2>/dev/null || true)"
    fi
    [[ -n "$selected" && -x "$selected" ]] || return 0
    selected_real="$(real_source_path "$selected")"
    while IFS= read -r candidate; do
        [[ -n "$candidate" && -x "$candidate" ]] || continue
        candidate_real="$(real_source_path "$candidate")"
        [[ "$candidate_real" != "$selected_real" ]] || continue
        managed_cli_system_fallback_path "$candidate" && continue
        duplicates+=("$candidate")
    done < <(type -a -p "$binary" 2>/dev/null | awk '!seen[$0]++')
    [[ "${#duplicates[@]}" -gt 0 ]] || return 0

    printf '  WARN: multiple managed %s commands are on PATH\n' "$tool" >&2
    printf '        selected: %s\n' "$selected" >&2
    for candidate in "${duplicates[@]}"; do
        printf '        duplicate: %s\n' "$candidate" >&2
        managed_cli_report_duplicate_owner "$candidate" "$tool"
    done
    echo "        setup leaves foreign installations untouched; rerun after cleanup to prove one command remains." >&2
}

managed_cli_audit_items() {
    if [[ -n "${INSTALL_DEPS_AUDIT_ITEMS:-}" ]]; then
        printf '%s\n' "$INSTALL_DEPS_AUDIT_ITEMS"
        return
    fi
    install_dependency_scan_items | awk -F'|' '$2 == "command" { print $1 "|" $3 }'
    printf '%s\n' \
        "zoxide|zoxide" \
        "npm|npm" \
        "latex2text|latex2text" \
        "wezterm|wezterm" \
        "aerospace|aerospace" \
        "herdr|herdr" \
        "devilspie2|devilspie2"
}

audit_managed_cli_installations() {
    local tool binary selected bins candidate
    while IFS='|' read -r tool binary selected; do
        [[ -n "$tool" ]] || continue
        if [[ -z "$binary" ]]; then
            bins="$(binaries_for "$tool")"
            for candidate in $bins; do
                if have "$candidate"; then
                    binary="$candidate"
                    break
                fi
            done
        fi
        [[ -n "$binary" ]] || continue
        if [[ "$tool" == "pi" && -x "$HOME/.local/bin/pi" ]]; then
            selected="$HOME/.local/bin/pi"
        fi
        audit_managed_cli_command "$tool" "$binary" "$selected"
    done <<EOF
$(managed_cli_audit_items)
EOF
}

direct_artifact_provenance_dir() {
    printf '%s\n' "${DOTFILES_PROVENANCE_DIR:-$HOME/.local/share/dotfiles/provenance}"
}

direct_artifact_marker_path() {
    printf '%s/%s.env\n' "$(direct_artifact_provenance_dir)" "$1"
}

write_direct_artifact_provenance() {
    local tool="$1" command_path="$2" binary_path="$3" install_root="$4" url="$5" version="$6" sha256="$7"
    local marker tmp binary_sha256 binary_real install_root_real
    binary_real="$(real_source_path "$binary_path")"
    install_root_real="$(real_source_path "$install_root")"
    if ! path_under "$binary_path" "$install_root" || ! path_under "$binary_real" "$install_root_real"; then
        echo "  FAIL: direct artifact binary for $tool is outside install root $install_root" >&2
        return 1
    fi
    if ! direct_artifact_current_metadata "$tool" ||
        ! direct_artifact_install_shape_allowed "$tool" "$command_path" "$binary_path" "$install_root"; then
        echo "  FAIL: direct artifact install shape for $tool is not repo-managed: command=$command_path binary=$binary_path root=$install_root" >&2
        return 1
    fi
    if ! binary_sha256="$(sha256_file "$binary_path")"; then
        echo "  FAIL: could not checksum installed $tool binary at $binary_path" >&2
        return 1
    fi
    marker="$(direct_artifact_marker_path "$tool")"
    mkdir -p "$(dirname "$marker")"
    tmp="$marker.tmp.$$"
    {
        printf 'schema=2\n'
        printf 'tool=%s\n' "$tool"
        printf 'version=%s\n' "$version"
        printf 'source_url=%s\n' "$url"
        printf 'sha256=%s\n' "$sha256"
        printf 'binary_sha256=%s\n' "$binary_sha256"
        printf 'command_path=%s\n' "$command_path"
        printf 'binary_path=%s\n' "$binary_path"
        printf 'install_root=%s\n' "$install_root"
    } > "$tmp"
    mv "$tmp" "$marker"
}

direct_artifact_marker_value() {
    local marker="$1" key="$2"
    awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$marker" 2>/dev/null || true
}

direct_artifact_supported_tool() {
    case "$1" in
        nvim|lazygit|starship|tree-sitter|chezmoi|herdr) return 0 ;;
        *) return 1 ;;
    esac
}

DIRECT_ARTIFACT_VERSION=""
DIRECT_ARTIFACT_URL=""
DIRECT_ARTIFACT_SHA256=""
DIRECT_ARTIFACT_DEFAULT_ROOT=""
DIRECT_ARTIFACT_DEFAULT_BINARY=""

direct_artifact_current_metadata() {
    local tool="$1" machine arch asset version_no_v
    DIRECT_ARTIFACT_VERSION=""
    DIRECT_ARTIFACT_URL=""
    DIRECT_ARTIFACT_SHA256=""
    DIRECT_ARTIFACT_DEFAULT_ROOT=""
    DIRECT_ARTIFACT_DEFAULT_BINARY=""

    [[ "$(uname -s)" == "Linux" ]] || return 1
    direct_artifact_supported_tool "$tool" || return 1
    machine="$(uname -m)"
    case "$tool:$machine" in
        nvim:x86_64|nvim:amd64)
            arch="x86_64"
            asset="nvim-linux-${arch}.tar.gz"
            DIRECT_ARTIFACT_VERSION="$NVIM_LINUX_VERSION"
            DIRECT_ARTIFACT_SHA256="$NVIM_LINUX_X86_64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/neovim/neovim/releases/download/${NVIM_LINUX_VERSION}/${asset}"
            DIRECT_ARTIFACT_DEFAULT_ROOT="/opt/nvim-linux-${arch}"
            DIRECT_ARTIFACT_DEFAULT_BINARY="$DIRECT_ARTIFACT_DEFAULT_ROOT/bin/nvim"
            ;;
        nvim:aarch64|nvim:arm64)
            arch="arm64"
            asset="nvim-linux-${arch}.tar.gz"
            DIRECT_ARTIFACT_VERSION="$NVIM_LINUX_VERSION"
            DIRECT_ARTIFACT_SHA256="$NVIM_LINUX_ARM64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/neovim/neovim/releases/download/${NVIM_LINUX_VERSION}/${asset}"
            DIRECT_ARTIFACT_DEFAULT_ROOT="/opt/nvim-linux-${arch}"
            DIRECT_ARTIFACT_DEFAULT_BINARY="$DIRECT_ARTIFACT_DEFAULT_ROOT/bin/nvim"
            ;;
        lazygit:x86_64|lazygit:amd64)
            version_no_v="${LAZYGIT_LINUX_VERSION#v}"
            asset="lazygit_${version_no_v}_linux_x86_64.tar.gz"
            DIRECT_ARTIFACT_VERSION="$LAZYGIT_LINUX_VERSION"
            DIRECT_ARTIFACT_SHA256="$LAZYGIT_LINUX_X86_64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/jesseduffield/lazygit/releases/download/${LAZYGIT_LINUX_VERSION}/${asset}"
            ;;
        lazygit:aarch64|lazygit:arm64)
            version_no_v="${LAZYGIT_LINUX_VERSION#v}"
            asset="lazygit_${version_no_v}_linux_arm64.tar.gz"
            DIRECT_ARTIFACT_VERSION="$LAZYGIT_LINUX_VERSION"
            DIRECT_ARTIFACT_SHA256="$LAZYGIT_LINUX_ARM64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/jesseduffield/lazygit/releases/download/${LAZYGIT_LINUX_VERSION}/${asset}"
            ;;
        starship:x86_64|starship:amd64)
            asset="starship-x86_64-unknown-linux-gnu.tar.gz"
            DIRECT_ARTIFACT_VERSION="$STARSHIP_VERSION"
            DIRECT_ARTIFACT_SHA256="$STARSHIP_LINUX_X86_64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/starship/starship/releases/download/${STARSHIP_VERSION}/${asset}"
            ;;
        starship:aarch64|starship:arm64)
            asset="starship-aarch64-unknown-linux-musl.tar.gz"
            DIRECT_ARTIFACT_VERSION="$STARSHIP_VERSION"
            DIRECT_ARTIFACT_SHA256="$STARSHIP_LINUX_ARM64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/starship/starship/releases/download/${STARSHIP_VERSION}/${asset}"
            ;;
        tree-sitter:x86_64|tree-sitter:amd64)
            asset="tree-sitter-cli-linux-x64.zip"
            DIRECT_ARTIFACT_VERSION="$TREE_SITTER_CLI_LINUX_VERSION"
            DIRECT_ARTIFACT_SHA256="$TREE_SITTER_CLI_LINUX_X86_64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/tree-sitter/tree-sitter/releases/download/${TREE_SITTER_CLI_LINUX_VERSION}/${asset}"
            ;;
        tree-sitter:aarch64|tree-sitter:arm64)
            asset="tree-sitter-cli-linux-arm64.zip"
            DIRECT_ARTIFACT_VERSION="$TREE_SITTER_CLI_LINUX_VERSION"
            DIRECT_ARTIFACT_SHA256="$TREE_SITTER_CLI_LINUX_ARM64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/tree-sitter/tree-sitter/releases/download/${TREE_SITTER_CLI_LINUX_VERSION}/${asset}"
            ;;
        chezmoi:x86_64|chezmoi:amd64)
            version_no_v="${CHEZMOI_VERSION#v}"
            asset="chezmoi_${version_no_v}_linux_amd64.tar.gz"
            DIRECT_ARTIFACT_VERSION="$CHEZMOI_VERSION"
            DIRECT_ARTIFACT_SHA256="$CHEZMOI_LINUX_X86_64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/twpayne/chezmoi/releases/download/${CHEZMOI_VERSION}/${asset}"
            ;;
        chezmoi:aarch64|chezmoi:arm64)
            version_no_v="${CHEZMOI_VERSION#v}"
            asset="chezmoi_${version_no_v}_linux_arm64.tar.gz"
            DIRECT_ARTIFACT_VERSION="$CHEZMOI_VERSION"
            DIRECT_ARTIFACT_SHA256="$CHEZMOI_LINUX_ARM64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/twpayne/chezmoi/releases/download/${CHEZMOI_VERSION}/${asset}"
            ;;
        herdr:x86_64|herdr:amd64)
            asset="herdr-linux-x86_64"
            DIRECT_ARTIFACT_VERSION="$HERDR_VERSION"
            DIRECT_ARTIFACT_SHA256="$HERDR_LINUX_X86_64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/ogulcancelik/herdr/releases/download/${HERDR_VERSION}/${asset}"
            ;;
        herdr:aarch64|herdr:arm64)
            asset="herdr-linux-aarch64"
            DIRECT_ARTIFACT_VERSION="$HERDR_VERSION"
            DIRECT_ARTIFACT_SHA256="$HERDR_LINUX_ARM64_SHA256"
            DIRECT_ARTIFACT_URL="https://github.com/ogulcancelik/herdr/releases/download/${HERDR_VERSION}/${asset}"
            ;;
        *) return 1 ;;
    esac
}

direct_artifact_install_shape_allowed() {
    local tool="$1" command_path="$2" binary_path="$3" install_root="$4" user_root user_binary
    user_root="$HOME/.local/bin"
    user_binary="$user_root/$tool"

    case "$tool" in
        nvim)
            [[ "$command_path" == "/usr/local/bin/nvim" &&
                "$binary_path" == "$DIRECT_ARTIFACT_DEFAULT_BINARY" &&
                "$install_root" == "$DIRECT_ARTIFACT_DEFAULT_ROOT" ]]
            ;;
        lazygit|starship)
            [[ "$command_path" == "/usr/local/bin/$tool" &&
                "$binary_path" == "/usr/local/bin/$tool" &&
                "$install_root" == "/usr/local/bin" ]] ||
                [[ "$command_path" == "$user_binary" &&
                    "$binary_path" == "$user_binary" &&
                    "$install_root" == "$user_root" ]]
            ;;
        tree-sitter|chezmoi|herdr)
            [[ "$command_path" == "$user_binary" &&
                "$binary_path" == "$user_binary" &&
                "$install_root" == "$user_root" ]]
            ;;
        *) return 1 ;;
    esac
}

DIRECT_ARTIFACT_REASON=""

direct_artifact_binary_version_matches() {
    local binary_path="$1" expected="$2" expected_no_v output
    expected_no_v="${expected#v}"
    if ! output="$("$binary_path" --version 2>&1)"; then
        return 1
    fi
    case "$output" in
        *"$expected"*|*"$expected_no_v"*) return 0 ;;
        *) return 1 ;;
    esac
}

direct_artifact_claims_tool_source() {
    local tool="$1" source="$2" marker schema marker_tool version url sha256 binary_sha256 command_path binary_path install_root
    local source_real binary_real command_real install_root_real current_binary_sha256
    DIRECT_ARTIFACT_REASON=""
    direct_artifact_current_metadata "$tool" || return 1
    marker="$(direct_artifact_marker_path "$tool")"
    [[ -f "$marker" ]] || return 1

    schema="$(direct_artifact_marker_value "$marker" schema)"
    marker_tool="$(direct_artifact_marker_value "$marker" tool)"
    version="$(direct_artifact_marker_value "$marker" version)"
    url="$(direct_artifact_marker_value "$marker" source_url)"
    sha256="$(direct_artifact_marker_value "$marker" sha256)"
    binary_sha256="$(direct_artifact_marker_value "$marker" binary_sha256)"
    command_path="$(direct_artifact_marker_value "$marker" command_path)"
    binary_path="$(direct_artifact_marker_value "$marker" binary_path)"
    install_root="$(direct_artifact_marker_value "$marker" install_root)"

    if [[ "$schema" != "2" || "$marker_tool" != "$tool" || -z "$binary_sha256" || -z "$command_path" || -z "$binary_path" || -z "$install_root" ]]; then
        DIRECT_ARTIFACT_REASON="invalid provenance marker"
        return 2
    fi

    source_real="$(real_source_path "$source")"
    binary_real="$(real_source_path "$binary_path")"
    command_real="$(real_source_path "$command_path")"
    install_root_real="$(real_source_path "$install_root")"
    if [[ "$source" != "$command_path" ]]; then
        if [[ "$source_real" == "$binary_real" || "$source_real" == "$command_real" ]] ||
            path_under "$source" "$install_root" || path_under "$source_real" "$install_root_real"; then
            DIRECT_ARTIFACT_REASON="command source does not match marker command path"
            return 2
        fi
        return 1
    fi
    if [[ "$source_real" != "$binary_real" ]]; then
        if path_under "$source" "$install_root" || path_under "$source_real" "$install_root_real"; then
            DIRECT_ARTIFACT_REASON="source is inside marker root but does not match marker binary"
            return 2
        fi
        DIRECT_ARTIFACT_REASON="command source target does not match marker binary"
        return 2
    fi

    if ! path_under "$binary_path" "$install_root" || ! path_under "$binary_real" "$install_root_real"; then
        DIRECT_ARTIFACT_REASON="marker binary is outside install root"
        return 2
    fi
    if ! direct_artifact_install_shape_allowed "$tool" "$command_path" "$binary_path" "$install_root"; then
        DIRECT_ARTIFACT_REASON="path does not match repo-pinned install shape"
        return 2
    fi
    if [[ ! -x "$binary_path" ]]; then
        DIRECT_ARTIFACT_REASON="marker binary is missing or not executable"
        return 2
    fi
    if [[ "$(real_source_path "$binary_path")" != "$binary_real" ]]; then
        DIRECT_ARTIFACT_REASON="binary realpath changed during provenance check"
        return 2
    fi
    if ! current_binary_sha256="$(sha256_file "$binary_path")"; then
        DIRECT_ARTIFACT_REASON="could not checksum marker binary"
        return 2
    fi
    if [[ "$current_binary_sha256" != "$binary_sha256" ]]; then
        DIRECT_ARTIFACT_REASON="marker binary checksum mismatch"
        return 2
    fi
    if [[ "$version" != "$DIRECT_ARTIFACT_VERSION" || "$url" != "$DIRECT_ARTIFACT_URL" || "$sha256" != "$DIRECT_ARTIFACT_SHA256" ]]; then
        DIRECT_ARTIFACT_REASON="repo pin changed"
        return 3
    fi
    if ! direct_artifact_binary_version_matches "$binary_path" "$DIRECT_ARTIFACT_VERSION"; then
        DIRECT_ARTIFACT_REASON="binary version does not match repo pin"
        return 2
    fi
    return 0
}

refresh_direct_artifact() {
    local tool="$1" old_yes old_reinstall rc
    old_yes="$YES_ALL"
    old_reinstall="${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-}"
    YES_ALL=1
    DOTFILES_DIRECT_ARTIFACT_REINSTALL=1
    export DOTFILES_DIRECT_ARTIFACT_REINSTALL
    rc=0
    case "$tool" in
        nvim) install_nvim_linux || rc=$? ;;
        lazygit) install_lazygit_linux || rc=$? ;;
        starship) install_starship_linux || rc=$? ;;
        tree-sitter) install_tree_sitter_cli_linux || rc=$? ;;
        chezmoi) install_chezmoi || rc=$? ;;
        herdr) install_herdr || rc=$? ;;
        *) rc=1 ;;
    esac
    YES_ALL="$old_yes"
    if [[ -n "$old_reinstall" ]]; then
        DOTFILES_DIRECT_ARTIFACT_REINSTALL="$old_reinstall"
    else
        unset DOTFILES_DIRECT_ARTIFACT_REINSTALL
    fi
    return "$rc"
}

APT_UPDATE_REFRESHED=0
APT_UPDATE_REFRESH_OK=1

apt_refresh_metadata_once() {
    if [[ "$APT_UPDATE_REFRESHED" -eq 1 ]]; then
        return "$APT_UPDATE_REFRESH_OK"
    fi
    APT_UPDATE_REFRESHED=1
    if apt_get_noninteractive update -qq; then
        APT_UPDATE_REFRESH_OK=0
    else
        APT_UPDATE_REFRESH_OK=1
        echo "  WARN: apt-get update failed; upgrading from the existing apt cache" >&2
    fi
    return "$APT_UPDATE_REFRESH_OK"
}

apt_installed_version() {
    dpkg-query -W -f='${Version}' "$1" 2>/dev/null || true
}

apt_candidate_version() {
    apt-cache policy "$1" 2>/dev/null | awk '/^[[:space:]]*Candidate:/ { print $2; exit }' || true
}

brew_pkg_outdated() {
    local pkg="$1" out rc
    out="$(brew outdated --formula --quiet "$pkg" 2>/dev/null)"
    rc=$?
    printf '%s\n' "$out" | awk -v p="$pkg" '($1 == p) { found = 1 } END { exit !found }' && return 0
    [[ "$rc" -eq 0 ]] || return 2
    return 1
}

zypper_pkg_outdated() {
    local pkg="$1" out
    if ! out="$(zypper --non-interactive list-updates -t package "$pkg" 2>/dev/null)"; then
        return 2
    fi
    printf '%s\n' "$out" | awk -F'|' -v p="$pkg" '{ gsub(/^[ \t]+|[ \t]+$/, "", $3); if ($3 == p) found = 1 } END { exit !found }' && return 0
    return 1
}

apk_pkg_outdated() {
    local pkg="$1" out
    if ! out="$(apk version -l '<' "$pkg" 2>/dev/null)"; then
        return 2
    fi
    [[ -n "$out" ]] && return 0
    return 1
}

scoped_update_status() {
    local owner="$1" tool="$2" pkg="$3" source="$4" before candidate after rc
    if [[ "$DRY_RUN" -eq 1 ]]; then
        case "$owner" in
            brew)   printf "  would    %-26s owner=brew package=%s source=%s\n" "$tool" "$pkg" "$source" ;;
            apt)    printf "  would    %-26s owner=apt package=%s source=%s\n" "$tool" "$pkg" "$source" ;;
            dnf)    printf "  would    %-26s owner=dnf package=%s source=%s\n" "$tool" "$pkg" "$source" ;;
            pacman) printf "  skipped  %-26s owner=pacman reason=requires explicit system upgrade source=%s\n" "$tool" "$source" ;;
            zypper) printf "  would    %-26s owner=zypper package=%s source=%s\n" "$tool" "$pkg" "$source" ;;
            apk)    printf "  would    %-26s owner=apk package=%s source=%s\n" "$tool" "$pkg" "$source" ;;
        esac
        return 0
    fi
    case "$owner" in
        brew)
            if brew_pkg_outdated "$pkg"; then
                rc=0
            else
                rc=$?
            fi
            if [[ "$rc" -eq 0 ]]; then
                if brew upgrade "$pkg"; then
                    printf "  updated   %-26s owner=brew package=%s source=%s\n" "$tool" "$pkg" "$source"
                    return 0
                fi
                printf "  WARN: brew update of %s returned %s\n" "$pkg" "$?" >&2
                return 1
            fi
            if [[ "$rc" -ne 1 ]]; then
                printf "  blocked   %-26s owner=brew package=%s reason=outdated-check-failed source=%s\n" "$tool" "$pkg" "$source" >&2
                return 1
            fi
            printf "  current   %-26s owner=brew package=%s source=%s\n" "$tool" "$pkg" "$source"
            return 0
            ;;
        apt)
            before="$(apt_installed_version "$pkg")"
            apt_refresh_metadata_once || true
            candidate="$(apt_candidate_version "$pkg")"
            if [[ -n "$before" && -n "$candidate" && "$candidate" != "(none)" && "$before" == "$candidate" && "$APT_UPDATE_REFRESH_OK" -eq 0 ]]; then
                printf "  current   %-26s owner=apt package=%s source=%s\n" "$tool" "$pkg" "$source"
                return 0
            fi
            if apt_get_noninteractive install -y --only-upgrade "$pkg"; then
                after="$(apt_installed_version "$pkg")"
                if [[ -n "$before" && -n "$after" && "$before" != "$after" ]]; then
                    printf "  updated   %-26s owner=apt package=%s source=%s\n" "$tool" "$pkg" "$source"
                elif [[ "$APT_UPDATE_REFRESH_OK" -eq 0 && -n "$before" && -n "$candidate" && "$candidate" != "(none)" && "$before" != "$candidate" ]]; then
                    printf "  blocked   %-26s owner=apt package=%s reason=post-upgrade-version-unchanged source=%s\n" "$tool" "$pkg" "$source" >&2
                    return 1
                else
                    printf "  current   %-26s owner=apt package=%s source=%s\n" "$tool" "$pkg" "$source"
                fi
                return 0
            fi
            printf "  WARN: apt update of %s failed\n" "$pkg" >&2
            return 1
            ;;
        dnf)
            if dnf check-update --quiet "$pkg" >/dev/null 2>&1; then
                rc=0
            else
                rc=$?
            fi
            if [[ "$rc" -eq 0 ]]; then
                printf "  current   %-26s owner=dnf package=%s source=%s\n" "$tool" "$pkg" "$source"
                return 0
            fi
            if [[ "$rc" -ne 100 ]]; then
                printf "  blocked   %-26s owner=dnf package=%s reason=outdated-check-failed source=%s\n" "$tool" "$pkg" "$source" >&2
                return 1
            fi
            maybe_sudo dnf upgrade -y "$pkg" &&
                printf "  updated   %-26s owner=dnf package=%s source=%s\n" "$tool" "$pkg" "$source"
            ;;
        pacman)
            printf "  skipped   %-26s owner=pacman reason=requires explicit system upgrade source=%s\n" "$tool" "$source"
            return 0
            ;;
        zypper)
            if zypper_pkg_outdated "$pkg"; then
                rc=0
            else
                rc=$?
            fi
            if [[ "$rc" -eq 1 ]]; then
                printf "  current   %-26s owner=zypper package=%s source=%s\n" "$tool" "$pkg" "$source"
                return 0
            fi
            if [[ "$rc" -ne 0 ]]; then
                printf "  blocked   %-26s owner=zypper package=%s reason=outdated-check-failed source=%s\n" "$tool" "$pkg" "$source" >&2
                return 1
            fi
            if maybe_sudo zypper update -y "$pkg"; then
                printf "  updated   %-26s owner=zypper package=%s source=%s\n" "$tool" "$pkg" "$source"
                return 0
            fi
            printf "  WARN: zypper update of %s failed\n" "$pkg" >&2
            return 1
            ;;
        apk)
            if apk_pkg_outdated "$pkg"; then
                rc=0
            else
                rc=$?
            fi
            if [[ "$rc" -eq 1 ]]; then
                printf "  current   %-26s owner=apk package=%s source=%s\n" "$tool" "$pkg" "$source"
                return 0
            fi
            if [[ "$rc" -ne 0 ]]; then
                printf "  blocked   %-26s owner=apk package=%s reason=outdated-check-failed source=%s\n" "$tool" "$pkg" "$source" >&2
                return 1
            fi
            if maybe_sudo apk upgrade "$pkg"; then
                printf "  updated   %-26s owner=apk package=%s source=%s\n" "$tool" "$pkg" "$source"
                return 0
            fi
            printf "  WARN: apk update of %s failed\n" "$pkg" >&2
            return 1
            ;;
    esac
    local rc=$?
    if [[ "$rc" -ne 0 ]]; then
        printf "  WARN: %s update of %s returned %s\n" "$owner" "$pkg" "$rc" >&2
    fi
    return "$rc"
}

pm_update() {
    local tool="$1" pkg="$2"
    scoped_update_status "$PM" "$tool" "$pkg" "$(update_tool_source "$tool" || true)"
}

# A resolved executable is Nix-owned when its command source (or its real path)
# lives under a Nix store / profile path. Update mode reports these as
# owner=nix and does NOT try to update them: Nix-owned tools are refreshed
# through the enforced POSIX Nix layer (`setup.sh`, or a reviewed flake.lock
# bump), never a blanket `nix profile upgrade` and never a silent flake.lock
# rewrite. Every other tool keeps its existing per-manager ownership -- this
# only fires when PATH actually resolves the tool from Nix.
nix_owns_tool_source() {
    local source="$1" real
    [[ -n "$source" ]] || return 1
    case "$source" in
        /nix/store/* | *"/.nix-profile/"* | *"/.local/state/nix/profile/"* | /run/current-system/sw/* | /etc/profiles/per-user/* | /nix/var/nix/profiles/*)
            return 0
            ;;
    esac
    real="$(real_source_path "$source" 2>/dev/null || true)"
    case "$real" in
        /nix/store/* | /run/current-system/sw/* | /etc/profiles/per-user/*)
            return 0
            ;;
    esac
    return 1
}

update_catalog_tool() {
    local tool="$1" pkg source brew_rc brew_owner_pkg native_pm native_pkg direct_rc hint
    [[ -n "$tool" ]] || return 0

    if ! update_tool_present "$tool"; then
        printf "  skipped   %-26s not installed\n" "$tool"
        return 0
    fi

    source="$(update_tool_source "$tool" || true)"
    if [[ -z "$source" ]]; then
        printf "  unmanaged %-26s source=unknown\n" "$tool"
        return 0
    fi

    if accepted_system_tool_source "$tool" "$source"; then
        printf "  system    %-26s source=%s\n" "$tool" "$source"
        return 0
    fi

    if nix_owns_tool_source "$source"; then
        printf "  skipped   %-26s owner=nix reason=managed by the Nix layer (setup.sh or a reviewed flake.lock bump) source=%s\n" "$tool" "$source"
        return 0
    fi

    pkg="$(pkg_for_pm "$tool" brew)"
    if [[ -n "$pkg" ]] && homebrew_bin >/dev/null 2>&1; then
        if brew_claims_tool_source "$tool" "$pkg" "$source"; then
            brew_rc=0
        else
            brew_rc=$?
        fi
        if [[ "$brew_rc" -eq 0 ]]; then
            scoped_update_status brew "$tool" "$pkg" "$source"
            return $?
        fi
        # The catalog package is the install default, not proof that it owns an
        # already-present executable. Versioned formulae such as python@3.14
        # can legitimately own the active command while python@3.12 remains
        # installed. Resolve that ownership from the executable's real Cellar
        # path and verify it against Homebrew's receipt before updating.
        if brew_owner_pkg="$(brew_formula_owning_tool_source "$source")"; then
            scoped_update_status brew "$tool" "$brew_owner_pkg" "$source"
            return $?
        fi
        if [[ "$brew_rc" -eq 2 ]]; then
            printf "  blocked   %-26s owner=brew package=%s reason=source-under-brew-prefix-but-formula-not-installed source=%s\n" "$tool" "$pkg" "$source" >&2
            return 1
        fi
        if [[ "$brew_rc" -eq 3 ]]; then
            printf "  blocked   %-26s owner=brew package=%s reason=source-under-brew-prefix-but-formula-does-not-own-source source=%s\n" "$tool" "$pkg" "$source" >&2
            return 1
        fi
        if [[ "$brew_rc" -eq 4 ]]; then
            printf "  blocked   %-26s owner=brew package=%s reason=source-under-brew-prefix-but-resolved-source-outside-prefix source=%s\n" "$tool" "$pkg" "$source" >&2
            return 1
        fi
    fi

    native_pm="$(native_linux_pm 2>/dev/null || true)"
    if [[ -n "$native_pm" && "$native_pm" != "unknown" ]]; then
        native_pkg="$(pkg_for_pm "$tool" "$native_pm")"
        if [[ -n "$native_pkg" ]] && native_pm_claims_tool_source "$native_pm" "$native_pkg" "$source"; then
            scoped_update_status "$native_pm" "$tool" "$native_pkg" "$source"
            return $?
        fi
    fi

    if direct_artifact_supported_tool "$tool"; then
        if direct_artifact_claims_tool_source "$tool" "$source"; then
            direct_rc=0
        else
            direct_rc=$?
        fi
        if [[ "$direct_rc" -eq 0 ]]; then
            printf "  current   %-26s owner=dotfiles-artifact version=%s source=%s\n" "$tool" "$DIRECT_ARTIFACT_VERSION" "$source"
            return 0
        fi
        if [[ "$direct_rc" -eq 3 ]]; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                printf "  would     %-26s owner=dotfiles-artifact version=%s source=%s\n" "$tool" "$DIRECT_ARTIFACT_VERSION" "$source"
                return 0
            fi
            if refresh_direct_artifact "$tool"; then
                source="$(update_tool_source "$tool" || true)"
                if direct_artifact_claims_tool_source "$tool" "$source"; then
                    direct_rc=0
                else
                    direct_rc=$?
                fi
                if [[ "$direct_rc" -ne 0 ]]; then
                    printf "  blocked   %-26s owner=dotfiles-artifact reason=post-update provenance mismatch source=%s\n" "$tool" "$source" >&2
                    return 1
                fi
                printf "  updated   %-26s owner=dotfiles-artifact version=%s source=%s\n" "$tool" "$DIRECT_ARTIFACT_VERSION" "$source"
                return 0
            fi
            printf "  WARN: dotfiles-artifact update of %s failed\n" "$tool" >&2
            return 1
        fi
        if [[ "$direct_rc" -eq 2 ]]; then
            printf "  blocked   %-26s owner=dotfiles-artifact reason=%s source=%s\n" "$tool" "$DIRECT_ARTIFACT_REASON" "$source" >&2
            return 1
        fi
    fi

    hint=""
    if [[ "$(uname -s)" == "Darwin" && "$source" == /usr/bin/* && "$tool" != "zsh" ]]; then
        pkg="$(pkg_for_pm "$tool" brew)"
        if [[ -n "$pkg" ]]; then
            hint=" hint=install-or-prioritize-homebrew-package:$pkg"
        fi
    fi
    printf "  unmanaged %-26s source=%s%s\n" "$tool" "$source" "$hint"
    return 0
}

update_catalog_tools() {
    local tool rc=0
    while IFS= read -r tool; do
        [[ -n "$tool" ]] || continue
        update_catalog_tool "$tool" || rc=1
    done <<EOF
$(catalog_tools)
EOF
    return "$rc"
}

run_update_mode() {
    PM="$(detect_update_pm)"
    OS_LABEL="$(uname -s)"
    if is_wsl; then OS_LABEL="WSL ($OS_LABEL)"; fi
    echo "install-deps: update mode OS=$OS_LABEL  detected package manager=$PM  dry-run=$DRY_RUN"
    echo

    local rc=0
    if [[ "$PM" == "brew" ]] && ! enable_homebrew_for_current_shell; then
        rc=1
    elif [[ "$PM" == "brew" ]] && ! link_homebrew_completions; then
        rc=1
    fi

    update_catalog_tools || rc=1
    echo
    echo "note: repo pins, PSFzf, plugins, and configs update via git pull and re-running setup; dotfiles-owned Linux artifacts refresh only when provenance proves ownership."
    return "$rc"
}

unique_backup_path() {
    local path="$1" base i
    base="$path.bak.$(date +%Y%m%d-%H%M%S)"
    if [[ ! -e "$base" ]]; then
        printf '%s\n' "$base"
        return 0
    fi
    i=1
    while [[ -e "$base.$i" ]]; do i=$((i + 1)); done
    printf '%s\n' "$base.$i"
}

have_c_compiler() {
    local compiler
    for compiler in cc gcc clang zig cl; do
        if have "$compiler"; then return 0; fi
    done
    return 1
}

install_c_toolchain_linux() {
    [[ "$(uname -s)" == "Linux" ]] || return 0
    if have_c_compiler; then
        printf "  ok        %-26s already installed\n" "C compiler"
        return
    fi

    local native_pm
    native_pm="$(native_linux_pm)"
    case "$native_pm" in
        apt)
            if ask "Install C compiler toolchain (build-essential)?"; then
                if native_linux_pm_install apt build-essential; then
                    :
                else
                    local rc=$?
                    record_install_failure "C compiler" apt "build-essential" "$rc"
                    echo "  FAIL: C compiler install failed; continuing to collect install failures"
                fi
            fi
            ;;
        dnf)
            if ask "Install C compiler toolchain (gcc gcc-c++ make)?"; then
                if native_linux_pm_install dnf gcc gcc-c++ make; then
                    :
                else
                    local rc=$?
                    record_install_failure "C compiler" dnf "gcc gcc-c++ make" "$rc"
                    echo "  FAIL: C compiler install failed; continuing to collect install failures"
                fi
            fi
            ;;
        pacman)
            if ask "Install C compiler toolchain (base-devel)?"; then
                if native_linux_pm_install pacman base-devel; then
                    :
                else
                    local rc=$?
                    record_install_failure "C compiler" pacman "base-devel" "$rc"
                    echo "  FAIL: C compiler install failed; continuing to collect install failures"
                fi
            fi
            ;;
        zypper)
            if ask "Install C compiler toolchain (gcc gcc-c++ make)?"; then
                if native_linux_pm_install zypper gcc gcc-c++ make; then
                    :
                else
                    local rc=$?
                    record_install_failure "C compiler" zypper "gcc gcc-c++ make" "$rc"
                    echo "  FAIL: C compiler install failed; continuing to collect install failures"
                fi
            fi
            ;;
        apk)
            if ask "Install C compiler toolchain (build-base)?"; then
                if native_linux_pm_install apk build-base; then
                    :
                else
                    local rc=$?
                    record_install_failure "C compiler" apk "build-base" "$rc"
                    echo "  FAIL: C compiler install failed; continuing to collect install failures"
                fi
            fi
            ;;
        *)
            printf "  manual    %-26s install cc/gcc/clang; plugin builds need a compiler\n" "C compiler"
            ;;
    esac
}

install_devilspie2_linux() {
    have devilspie2 && return 0
    local native_pm
    native_pm="$(native_linux_pm)"
    case "$native_pm" in
        apt|dnf|pacman|zypper|apk)
            native_linux_pm_install "$native_pm" devilspie2
            ;;
        *)
            return 1
            ;;
    esac
}

# install <check-binary> [purpose-string]
# Looks up package name from PKG_TABLE for current PM. Skips if installed.
# Uses binaries_for() so distro-specific aliases (fd -> fdfind on apt) count
# as "already installed".
install() {
    local tool="$1" purpose="${2:-}"
    local bins
    bins="$(binaries_for "$tool")"
    # shellcheck disable=SC2086  # $bins is intentional word-splitting
    if have_any $bins; then
        printf "  ok        %-26s already installed\n" "$tool"
        return
    fi
    local pkg
    pkg="$(pkg_for "$tool")"
    if [[ -z "$pkg" ]]; then
        printf "  manual    %-26s not in %s repos; install separately\n" "$tool" "$PM"
        return
    fi
    if ask "Install $tool${purpose:+ ($purpose)}?"; then
        # shellcheck disable=SC2086  # $pkg may carry cask flags on brew
        if pm_install $pkg; then
            :
        else
            local rc=$?
            record_install_failure "$tool" "$PM" "$pkg" "$rc"
            printf "  FAIL: %-26s install failed; continuing to collect install failures\n" "$tool" >&2
        fi
        # Post-install fix for fd-find on apt (binary lands as 'fdfind',
        # not 'fd'). Telescope's find_files uses fd by default.
        if [[ "$tool" == "fd" ]] && [[ "$PM" == "apt" ]] && ! have fd && have fdfind; then
            local fd_link fdfind_bin fd_target fd_backup
            fd_link="$HOME/.local/bin/fd"
            fdfind_bin="$(command -v fdfind)"
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would:    link ~/.local/bin/fd -> $fdfind_bin"
                return
            fi
            mkdir -p "$HOME/.local/bin"
            if [[ -L "$fd_link" ]]; then
                fd_target="$(readlink "$fd_link" || true)"
                if [[ "$fd_target" != "$fdfind_bin" ]]; then
                    fd_backup="$(unique_backup_path "$fd_link")"
                    mv "$fd_link" "$fd_backup"
                    printf "  backup    %-26s %s\n" "fd" "$fd_backup"
                fi
            elif [[ -e "$fd_link" ]]; then
                fd_backup="$(unique_backup_path "$fd_link")"
                mv "$fd_link" "$fd_backup"
                printf "  backup    %-26s %s\n" "fd" "$fd_backup"
            fi
            if [[ ! -L "$fd_link" ]]; then
                ln -s "$fdfind_bin" "$fd_link"
            fi
            ensure_local_bin_on_path
            printf "  set       %-26s ~/.local/bin/fd -> fdfind\n" "fd"
        fi
    else
        printf "  skipped   %-26s\n" "$tool"
    fi
}

hack_nerd_font_installed() {
    if fc-list 2>/dev/null | grep -qi "hack.*nerd"; then
        return 0
    fi
    # Homebrew's cask receipt is authoritative on macOS even before fontconfig
    # has indexed Apple's font directories. This keeps repeated setup runs from
    # trying to reinstall an already-present cask (and from triggering an
    # unnecessary Brew update).
    if [[ "$(uname -s)" == "Darwin" ]] && [[ "$PM" == "brew" ]]; then
        brew list --cask --versions font-hack-nerd-font >/dev/null 2>&1
        return $?
    fi
    return 1
}

install_nerd_font() {
    if hack_nerd_font_installed; then
        printf "  ok        %-26s already installed\n" "Hack Nerd Font"
        return
    fi
    if [[ "$PM" == "brew" ]] && [[ "$(uname -s)" == "Darwin" ]]; then
        if ask "Install Hack Nerd Font (used by ghostty / Windows Terminal)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: brew install --cask font-hack-nerd-font"
            else
                if brew install --cask font-hack-nerd-font; then
                    :
                else
                    local rc=$?
                    record_install_failure "Hack Nerd Font" brew "cask:font-hack-nerd-font" "$rc"
                    echo "  FAIL: Hack Nerd Font cask install failed; continuing to collect install failures"
                fi
            fi
        fi
        return
    fi
    if ! ask "Install Hack Nerd Font (download + extract to user font dir)?"; then
        printf "  skipped   %-26s\n" "Hack Nerd Font"
        return
    fi
    if ! require_downloader; then
        record_install_failure "Hack Nerd Font" direct "Hack.zip" "downloader"
        return 0
    fi
    if ! have_any unzip bsdtar; then
        echo "  need      unzip missing; installing extractor for Hack Nerd Font"
        install unzip "extract Hack Nerd Font archive"
        if [[ "$DRY_RUN" -ne 1 ]] && ! have_any unzip bsdtar; then
            echo "  FAIL: need 'unzip' or 'bsdtar' to extract the font archive"
            record_install_failure "Hack Nerd Font" direct "Hack.zip" "missing-extractor"
            return 0
        fi
    fi
    local url
    url="https://github.com/ryanoasis/nerd-fonts/releases/download/${HACK_NERD_FONT_VERSION}/Hack.zip"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fL $url"
        echo "         verify sha256 $HACK_NERD_FONT_SHA256"
        echo "         unzip -> \${XDG_DATA_HOME:-\$HOME/.local/share}/fonts/HackNerdFont/"
        echo "         fc-cache -f"
        return
    fi
    local font_dir="${XDG_DATA_HOME:-$HOME/.local/share}/fonts/HackNerdFont"
    local tmp; tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    if ! curl -fL -o "$tmp/Hack.zip" "$url"; then
        echo "  FAIL: download failed; install Hack Nerd Font manually from nerd-fonts releases"
        record_install_failure "Hack Nerd Font" direct "Hack.zip" "download"
        rm -rf "$tmp"; return 0
    fi
    if ! verify_sha256 "$tmp/Hack.zip" "$HACK_NERD_FONT_SHA256"; then
        echo "  FAIL: checksum mismatch for Hack.zip"
        record_install_failure "Hack Nerd Font" direct "Hack.zip" "sha256"
        rm -rf "$tmp"; return 0
    fi
    mkdir -p "$font_dir"
    if have unzip; then
        if ! unzip -oq "$tmp/Hack.zip" -d "$font_dir"; then
            echo "  FAIL: could not extract Hack.zip"
            record_install_failure "Hack Nerd Font" direct "Hack.zip" "extract"
            return 0
        fi
    else
        if ! bsdtar -xf "$tmp/Hack.zip" -C "$font_dir"; then
            echo "  FAIL: could not extract Hack.zip"
            record_install_failure "Hack Nerd Font" direct "Hack.zip" "extract"
            return 0
        fi
    fi
    rm -rf "$tmp"
    if have fc-cache; then
        fc-cache -f "$(dirname "$font_dir")" >/dev/null 2>&1 || true
    fi
    printf "  installed %-26s -> %s\n" "Hack Nerd Font" "$font_dir"
}

# Select the exact reviewed release asset for the Debian-family mappings
# supported by the pinned upstream release. Return 2 for an unsupported
# distro/architecture combination, and 1 when host identity cannot be read.
resolve_ghostty_deb_asset() {
    local arch id version_id ubuntu_version_id version_codename
    local ubuntu_codename debian_codename target="" sha=""

    GHOSTTY_DEB_ARCH=""
    GHOSTTY_DEB_ASSET=""
    GHOSTTY_DEB_SHA256=""
    GHOSTTY_DEB_URL=""

    if ! arch="$(dpkg --print-architecture 2>/dev/null)" || [[ -z "$arch" ]]; then
        echo "  FAIL: could not resolve dpkg architecture for the pinned Ghostty package" >&2
        return 1
    fi
    case "$arch" in
        amd64|arm64) ;;
        *) return 2 ;;
    esac

    id="$(os_release_value ID 2>/dev/null || true)"
    version_id="$(os_release_value VERSION_ID 2>/dev/null || true)"
    ubuntu_version_id="$(os_release_value UBUNTU_VERSION_ID 2>/dev/null || true)"
    version_codename="$(os_release_value VERSION_CODENAME 2>/dev/null || true)"
    ubuntu_codename="$(os_release_value UBUNTU_CODENAME 2>/dev/null || true)"
    debian_codename="$(os_release_value DEBIAN_CODENAME 2>/dev/null || true)"
    [[ -n "$id" ]] || {
        echo "  FAIL: could not resolve distro identity for the pinned Ghostty package" >&2
        return 1
    }

    case "$id" in
        ubuntu|pop|tuxedo|neon)
            target="$version_id"
            ;;
        elementary)
            target="$ubuntu_version_id"
            ;;
        debian)
            [[ "$version_codename" == "trixie" ]] && target="trixie"
            ;;
        kali)
            [[ "${version_id%%.*}" == "2025" ]] && target="trixie"
            ;;
        sparky)
            [[ "$version_id" == "8" ]] && target="trixie"
            ;;
        linuxmint|zorin)
            if [[ "$debian_codename" == "trixie" ]]; then
                target="trixie"
            elif [[ "$ubuntu_codename" == "noble" ]]; then
                target="24.04"
            elif [[ "$ubuntu_codename" == "questing" ]]; then
                target="25.10"
            fi
            ;;
        *)
            case "$ubuntu_version_id" in
                24.04|25.10) target="$ubuntu_version_id" ;;
            esac
            ;;
    esac

    case "${arch}_${target}" in
        amd64_24.04) sha="$GHOSTTY_UBUNTU_AMD64_2404_SHA256" ;;
        arm64_24.04) sha="$GHOSTTY_UBUNTU_ARM64_2404_SHA256" ;;
        amd64_25.10) sha="$GHOSTTY_UBUNTU_AMD64_2510_SHA256" ;;
        arm64_25.10) sha="$GHOSTTY_UBUNTU_ARM64_2510_SHA256" ;;
        amd64_trixie) sha="$GHOSTTY_DEBIAN_AMD64_TRIXIE_SHA256" ;;
        arm64_trixie) sha="$GHOSTTY_DEBIAN_ARM64_TRIXIE_SHA256" ;;
        *) return 2 ;;
    esac

    GHOSTTY_DEB_ARCH="$arch"
    GHOSTTY_DEB_ASSET="ghostty_${GHOSTTY_UBUNTU_ASSET_VERSION}_${arch}_${target}.deb"
    GHOSTTY_DEB_SHA256="$sha"
    GHOSTTY_DEB_URL="https://github.com/mkasberg/ghostty-ubuntu/releases/download/${GHOSTTY_UBUNTU_VERSION}/${GHOSTTY_DEB_ASSET}"
    return 0
}

# Download the selected immutable asset, prove its bytes and package identity,
# install only that local file, then verify the package manager consumed the
# expected version. Caller must already have passed require_downloader.
install_verified_ghostty_deb() {
    local url="$1" asset="$2" expected_sha="$3" expected_arch="$4"
    local tmp deb package architecture version installed_version
    (
        if ! tmp="$(mktemp -d)" || [[ -z "$tmp" ]]; then
            echo "  FAIL: could not create private staging for the pinned Ghostty package"
            return 1
        fi
        trap 'rm -rf "$tmp"' EXIT
        trap 'exit 129' HUP
        trap 'exit 130' INT
        trap 'exit 143' TERM
        deb="$tmp/$asset"

        if ! curl -fsSL --retry 3 --retry-delay 1 -o "$deb" "$url"; then
            echo "  FAIL: could not download pinned Ghostty .deb from $url"
            return 1
        fi
        if [[ ! -s "$deb" ]] || ! verify_sha256 "$deb" "$expected_sha"; then
            echo "  FAIL: checksum mismatch for $asset"
            echo "        expected reviewed mkasberg/ghostty-ubuntu@$GHOSTTY_UBUNTU_VERSION bytes; package was not installed"
            return 1
        fi
        if ! have dpkg-deb; then
            echo "  FAIL: dpkg-deb is required to validate the pinned Ghostty package before installation"
            return 1
        fi
        package="$(dpkg-deb --field "$deb" Package 2>/dev/null || true)"
        architecture="$(dpkg-deb --field "$deb" Architecture 2>/dev/null || true)"
        version="$(dpkg-deb --field "$deb" Version 2>/dev/null || true)"
        if [[ "$package" != "ghostty" || "$architecture" != "$expected_arch" || "$version" != "$GHOSTTY_UBUNTU_PACKAGE_VERSION" ]]; then
            echo "  FAIL: unexpected package metadata for $asset"
            echo "        expected Package=ghostty Architecture=$expected_arch Version=$GHOSTTY_UBUNTU_PACKAGE_VERSION"
            echo "        received Package=${package:-<missing>} Architecture=${architecture:-<missing>} Version=${version:-<missing>}"
            return 1
        fi
        if ! apt_get_noninteractive install -y "$deb"; then
            echo "  FAIL: could not install verified Ghostty package $asset"
            echo "        repair apt with 'sudo apt-get -f install', then rerun setup"
            return 1
        fi
        installed_version="$(dpkg-query -W -f='${Version}' ghostty 2>/dev/null || true)"
        if [[ "$installed_version" != "$GHOSTTY_UBUNTU_PACKAGE_VERSION" ]] || ! have ghostty; then
            echo "  FAIL: Ghostty publication could not be validated after installing $asset"
            echo "        Remove the package with 'sudo apt-get remove ghostty', then rerun setup"
            return 1
        fi
        printf "  installed %-26s %s (%s)\n" "ghostty" "$GHOSTTY_UBUNTU_PACKAGE_VERSION" "$expected_arch"
    )
}

# Ghostty: Linux packaging varies. Homebrew's Ghostty formula is macOS-only,
# so Linux/WSL should prefer distro/community packages or manual install guidance.
install_ghostty_linux() {
    local resolve_rc=0 native_pm
    if have ghostty; then
        printf "  ok        %-26s already installed\n" "ghostty"
        return
    fi
    if is_wsl && ! wsl_gui_opt_in; then
        printf "  skipped   %-26s WSL uses Windows Terminal by default\n" "ghostty"
        echo "            Linux Ghostty in WSL is experimental: re-run with --experimental-wsl-gui"
        echo "            Windows host setup: .\\setup.ps1 -All -MergeWindowsTerminal"
        return
    fi
    if is_wsl && ! can_show_gui; then
        printf "  skipped   %-26s WSL GUI display not detected\n" "ghostty"
        echo "            --experimental-wsl-gui needs WSLg/X11/Wayland to be available"
        return
    fi
    [[ "$PM" == "brew" ]] && printf "  skipped   %-26s Homebrew formula is macOS-only on Linux\n" "ghostty via brew"
    native_pm="$(native_linux_pm 2>/dev/null || true)"
    if is_ubuntu || [[ "$native_pm" == "apt" ]]; then
        resolve_ghostty_deb_asset || resolve_rc=$?
        if [[ "$resolve_rc" -eq 2 ]]; then
            printf "  manual    %-26s no reviewed %s asset for this distro/architecture\n" "ghostty" "$GHOSTTY_UBUNTU_VERSION"
            echo "            see https://github.com/mkasberg/ghostty-ubuntu/releases/tag/$GHOSTTY_UBUNTU_VERSION"
            return
        elif [[ "$resolve_rc" -ne 0 ]]; then
            record_install_failure "ghostty" apt "mkasberg/ghostty-ubuntu@$GHOSTTY_UBUNTU_VERSION" "$resolve_rc"
            echo "  FAIL: Ghostty package identity resolution failed; continuing to collect install failures"
            return
        fi
        if ask "Install ghostty from verified mkasberg/ghostty-ubuntu $GHOSTTY_UBUNTU_VERSION package bytes?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: curl -fsSL $GHOSTTY_DEB_URL -o /tmp/$GHOSTTY_DEB_ASSET"
                echo "         verify sha256 $GHOSTTY_DEB_SHA256"
                echo "         validate Package=ghostty Architecture=$GHOSTTY_DEB_ARCH Version=$GHOSTTY_UBUNTU_PACKAGE_VERSION"
                echo "         sudo apt-get install -y /tmp/$GHOSTTY_DEB_ASSET"
            else
                require_downloader || return 1
                local rc=0
                install_verified_ghostty_deb \
                    "$GHOSTTY_DEB_URL" "$GHOSTTY_DEB_ASSET" "$GHOSTTY_DEB_SHA256" "$GHOSTTY_DEB_ARCH" || rc=$?
                if [[ "$rc" -ne 0 ]]; then
                    record_install_failure "ghostty" apt "mkasberg/ghostty-ubuntu@$GHOSTTY_UBUNTU_VERSION" "$rc"
                    echo "  FAIL: verified Debian-family Ghostty package install failed; continuing to collect install failures"
                fi
            fi
            return
        fi
    fi
    if have snap; then
        if ask "Install ghostty via snap (community package)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: sudo snap install ghostty --classic"
            else
                local rc=0
                maybe_sudo snap install ghostty --classic || rc=$?
                if [[ "$rc" -ne 0 ]]; then
                    record_install_failure "ghostty" snap "ghostty --classic" "$rc"
                    echo "  FAIL: snap install failed for ghostty; continuing to collect install failures"
                fi
            fi
            return
        fi
    fi
    echo "  manual    ghostty has no native $PM package. Options:"
    echo "              - ubuntu:  re-run this script for the verified pinned installer"
    echo "              - manual:  follow https://ghostty.org/docs/install/binary"
    echo "              - snap:    sudo snap install ghostty --classic"
    echo "              - flatpak: search 'ghostty' on flathub"
    echo "              - source:  https://ghostty.org/docs/install/build"
}

# Download, SHA-256 verify, and install the pinned WezTerm .deb. We verify the
# .deb before touching the package database (unlike a bare `apt install <url>`),
# so an upstream change at the pinned tag fails closed. `apt-get install <file>`
# resolves the GUI runtime deps in one step. require_downloader already ran.
run_wezterm_deb_install() {
    local url="$1" expected="$2" tmp deb rc=0
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    deb="$tmp/wezterm.deb"
    if ! curl -fsSL -o "$deb" "$url"; then
        echo "  FAIL: could not download WezTerm .deb"
        rm -rf "$tmp"; return 1
    fi
    if ! verify_sha256 "$deb" "$expected"; then
        echo "  FAIL: checksum mismatch for WezTerm .deb (pinned $WEZTERM_VERSION)"
        echo "        upstream changed; review it, then bump WEZTERM_VERSION + SHA together"
        rm -rf "$tmp"; return 1
    fi
    apt_get_noninteractive install -y "$deb" || rc=$?
    rm -rf "$tmp"
    return "$rc"
}

# WezTerm: Homebrew cask is macOS-only, so Linux uses the official pinned .deb
# (amd64 Ubuntu). Split-host WSL skips it unless the user opts into WSL GUI,
# matching Ghostty's convention; native Linux may install it even from a
# headless shell because package installation is not a runtime display probe.
# arm64 Linux / non-Ubuntu get manual guidance.
install_wezterm_linux() {
    local url arch
    if have wezterm; then
        printf "  ok        %-26s already installed\n" "wezterm"
        return
    fi
    if is_wsl && ! wsl_gui_opt_in; then
        printf "  skipped   %-26s WSL uses the Windows-host terminal by default\n" "wezterm"
        echo "            Linux WezTerm in WSL is experimental: re-run with --experimental-wsl-gui"
        echo "            Windows host setup installs WezTerm via .\\setup.ps1 -All"
        return
    fi
    if is_wsl && ! can_show_gui; then
        printf "  skipped   %-26s WSL GUI display not detected\n" "wezterm"
        echo "            --experimental-wsl-gui needs WSLg/X11/Wayland to be available"
        return
    fi
    [[ "$PM" == "brew" ]] && printf "  skipped   %-26s Homebrew cask is macOS-only on Linux\n" "wezterm via brew"
    arch="$(uname -m)"
    if is_ubuntu && { [[ "$arch" == "x86_64" ]] || [[ "$arch" == "amd64" ]]; }; then
        url="https://github.com/wezterm/wezterm/releases/download/${WEZTERM_VERSION}/wezterm-${WEZTERM_VERSION}.Ubuntu22.04.deb"
        if ! ask "Install WezTerm via official Ubuntu .deb (pinned $WEZTERM_VERSION, SHA-256 verified)?"; then
            printf "  skipped   %-26s\n" "wezterm"
            return
        fi
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: curl -fsSL $url"
            echo "         verify sha256 $WEZTERM_DEB_AMD64_SHA256"
            echo "         apt-get install -y <downloaded .deb>   (resolves GUI deps)"
        else
            require_downloader || return 1
            if ! run_wezterm_deb_install "$url" "$WEZTERM_DEB_AMD64_SHA256"; then
                echo "  FAIL: WezTerm .deb install failed"
                return 1
            fi
        fi
        return
    fi
    echo "  manual    wezterm is not auto-installed on this Linux host. Options:"
    echo "              - ubuntu amd64: re-run for the verified pinned .deb"
    echo "              - other:        https://wezterm.org/install/linux.html"
    echo "              - flatpak:      flatpak install flathub org.wezfurlong.wezterm"
}

# Download, SHA-256 verify, and install the pinned Herdr Linux binary. We verify
# the binary before installing it (unlike the herdr.dev install.sh remote-eval
# path), so an upstream change at the pinned tag fails closed. require_downloader
# already ran.
run_herdr_linux_binary_install() {
    local url="$1" expected="$2" dest="$3" tmp bin rc=0
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    bin="$tmp/herdr"
    if ! curl -fsSL -o "$bin" "$url"; then
        echo "  FAIL: could not download herdr binary"
        rm -rf "$tmp"; return 1
    fi
    if ! verify_sha256 "$bin" "$expected"; then
        echo "  FAIL: checksum mismatch for herdr $HERDR_VERSION"
        echo "        upstream changed; review it, then bump HERDR_VERSION + SHA together"
        rm -rf "$tmp"; return 1
    fi
    mkdir -p "$(dirname "$dest")"
    cp "$bin" "$dest" && chmod 0755 "$dest" || rc=$?
    rm -rf "$tmp"
    return "$rc"
}

# Herdr: agent multiplexer. macOS + Linuxbrew use the canonical homebrew-core
# formula; native Linux without brew uses the pinned, SHA-256-verified release
# binary. Native Windows is handled separately by install-deps.ps1 with a pinned,
# SHA-256-verified preview .exe.
install_herdr() {
    local arch asset expected url dest
    if have herdr && [[ "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        printf "  ok        %-26s already installed\n" "herdr"
        return
    fi
    if [[ "$PM" == "brew" && "${DOTFILES_DIRECT_ARTIFACT_REINSTALL:-0}" != "1" ]]; then
        if ! ask "Install Herdr (agent multiplexer) via Homebrew (brew install herdr)?"; then
            printf "  skipped   %-26s\n" "herdr"
            return
        fi
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: brew install herdr"
            return
        fi
        if ! brew install herdr; then
            echo "  FAIL: herdr brew install failed"
            return 1
        fi
        return
    fi
    if [[ "$(uname -s)" != "Linux" ]]; then
        printf "  skipped   %-26s no Homebrew and not Linux\n" "herdr"
        return
    fi
    arch="$(uname -m)"
    case "$arch" in
        x86_64 | amd64)
            asset="herdr-linux-x86_64"
            expected="$HERDR_LINUX_X86_64_SHA256"
            ;;
        aarch64 | arm64)
            asset="herdr-linux-aarch64"
            expected="$HERDR_LINUX_ARM64_SHA256"
            ;;
        *)
            echo "  manual    herdr has no pinned Linux binary for arch $arch; see https://herdr.dev/docs/install/"
            return
            ;;
    esac
    url="https://github.com/ogulcancelik/herdr/releases/download/${HERDR_VERSION}/${asset}"
    dest="$HOME/.local/bin/herdr"
    if ! ask "Install Herdr (agent multiplexer) via pinned GitHub release ${HERDR_VERSION} binary (SHA-256 verified)?"; then
        printf "  skipped   %-26s\n" "herdr"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: curl -fsSL $url"
        echo "         verify sha256 $expected"
        echo "         install -m 755 -> $dest"
        return
    fi
    require_downloader || return 1
    if ! run_herdr_linux_binary_install "$url" "$expected" "$dest"; then
        echo "  FAIL: herdr binary install failed"
        return 1
    fi
    write_direct_artifact_provenance "herdr" "$dest" "$dest" "$(dirname "$dest")" "$url" "$HERDR_VERSION" "$expected"
    ensure_local_bin_on_path
    printf "  installed %-26s -> %s\n" "herdr" "$dest"
}

# VS Code: brew cask on macOS; snap, then flatpak, then a manual hint on Linux.
install_vscode() {
    if have code; then
        if is_wsl; then
            printf "  ok        %-26s code CLI available (Windows VS Code / Remote WSL)\n" "vscode"
        else
            printf "  ok        %-26s already installed\n" "vscode"
        fi
        return
    fi
    if is_wsl && ! can_show_gui; then
        printf "  manual    %-26s install VS Code on Windows, or enable WSLg for Linux GUI apps\n" "vscode"
        return
    fi
    if [[ "$(uname -s)" == "Darwin" ]] && [[ "$PM" == "brew" ]]; then
        if ask "Install Visual Studio Code (brew --cask)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: brew install --cask visual-studio-code"
            else
                local rc=0
                brew install --cask visual-studio-code || rc=$?
                if [[ "$rc" -ne 0 ]]; then
                    record_install_failure "vscode" brew "cask:visual-studio-code" "$rc"
                    echo "  FAIL: VS Code cask install failed; continuing to collect install failures"
                fi
            fi
        fi
        return
    fi
    if have snap; then
        if ask "Install Visual Studio Code via snap?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: sudo snap install code --classic"
            else
                local rc=0
                maybe_sudo snap install code --classic || rc=$?
                if [[ "$rc" -ne 0 ]]; then
                    record_install_failure "vscode" snap "code --classic" "$rc"
                    echo "  FAIL: VS Code snap install failed; continuing to collect install failures"
                fi
            fi
            return
        fi
    elif have flatpak; then
        if ask "Install Visual Studio Code via flatpak?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: flatpak install -y flathub com.visualstudio.code"
            else
                local rc=0
                flatpak install -y flathub com.visualstudio.code || rc=$?
                if [[ "$rc" -ne 0 ]]; then
                    record_install_failure "vscode" flatpak "com.visualstudio.code" "$rc"
                    echo "  FAIL: VS Code flatpak install failed; continuing to collect install failures"
                fi
            fi
            return
        fi
    fi
    echo "  manual    vscode: install from https://code.visualstudio.com/docs/setup/linux"
    echo "            (the Microsoft apt/dnf repo or snap both provide a 'code' CLI)"
}

# If a usable `code` CLI exists (VS Code detected), offer to install the Rose
# Pine theme extension and set it as the active theme.
configure_vscode_rose_pine() {
    if ! have code; then
        printf "  skipped   %-26s no 'code' CLI (open VS Code -> 'Shell Command: Install code command in PATH')\n" "rose-pine (vscode)"
        return
    fi
    if ! ask "VS Code: install the Rose Pine theme and set it active?"; then
        printf "  skipped   %-26s\n" "rose-pine (vscode)"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: code --install-extension mvllow.rose-pine; set VS Code theme and font settings"
        return
    fi
    if code --install-extension mvllow.rose-pine >/dev/null 2>&1; then
        printf "  installed %-26s mvllow.rose-pine\n" "rose-pine (vscode)"
    else
        echo "  WARN: 'code --install-extension mvllow.rose-pine' failed"
    fi
    # Pass the resolved path explicitly (the test injects its own). Passing an
    # arg also avoids shellcheck SC2120/SC2119 on the optional-$1 setter.
    set_vscode_theme "$(vscode_settings_path)"
}

# GNOME/X11: Ghostty's `maximize = true` is only a hint Mutter may ignore, so
# enforce it with devilspie2 (X11). Opt-in; only offered when Ghostty is
# installed and the session is not Wayland (devilspie2 is X11-only). Links the
# repo rule and enables it at login. Manual equivalent is in the rule's header.
setup_ghostty_maximize() {
    [[ "$(uname -s)" == "Linux" ]] || return 0
    if is_wsl && ! wsl_gui_opt_in; then return 0; fi
    have ghostty || return 0
    if [[ "${XDG_SESSION_TYPE:-}" == "wayland" ]]; then
        printf "  skipped   %-26s Wayland session; devilspie2 is X11-only (needs a GNOME extension)\n" "ghostty maximize"
        return 0
    fi
    if ! ask "Force Ghostty to open maximized on GNOME/X11 (devilspie2)?"; then
        printf "  skipped   %-26s\n" "ghostty maximize"
        return 0
    fi
    local cfg repo rule
    cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
    repo="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
    rule="$repo/linux/devilspie2/ghostty-maximize.lua"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: install devilspie2, link the maximize rule into $cfg/devilspie2/,"
        echo "         write $cfg/autostart/devilspie2.desktop, and start devilspie2"
        return 0
    fi
    local rc=0 native_pm
    native_pm="$(native_linux_pm)"
    install_devilspie2_linux || rc=$?
    if [[ "$rc" -ne 0 ]]; then
        record_install_failure "devilspie2" "$native_pm" "devilspie2" "$rc"
        echo "  FAIL: devilspie2 install failed; continuing to collect install failures"
    fi
    mkdir -p "$cfg/devilspie2" "$cfg/autostart"
    ln -sfn "$rule" "$cfg/devilspie2/ghostty-maximize.lua"
    cat > "$cfg/autostart/devilspie2.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=devilspie2
Exec=devilspie2
X-GNOME-Autostart-enabled=true
EOF
    # Start now (best-effort) so it works without a re-login -- only with a display.
    if have devilspie2 && [[ -n "${DISPLAY:-}" ]]; then
        (devilspie2 >/dev/null 2>&1 &) || true
    fi
    printf "  set       %-26s devilspie2 rule linked + autostart enabled\n" "ghostty maximize"
    echo "            open a new Ghostty window (runs automatically on next login too)"
}

# WSL clipboard bridge: check that win32yank.exe is reachable from WSL PATH.
check_wsl_clipboard() {
    if have win32yank.exe; then
        printf "  ok        %-26s win32yank.exe on PATH\n" "WSL clipboard"
        return
    fi
    echo "  manual    win32yank.exe is REQUIRED for WSL clipboard integration."
    echo "            On the Windows side:"
    echo "              scoop install win32yank   (or download from"
    echo "              https://github.com/equalsraf/win32yank/releases)"
    echo "            Then it must be on the WSL PATH (typical: it appears"
    echo "            automatically once installed via scoop)."
}

install_dependency_pm_scan_item() {
    local pm_tool=""
    case "${PM:-unknown}" in
        brew|brew_missing) pm_tool="brew" ;;
        apt|dnf|pacman|zypper|apk) pm_tool="$PM" ;;
    esac
    if [[ -n "$pm_tool" ]]; then
        printf '%s\n' "${pm_tool}|command|${pm_tool}"
    fi
}

install_dependency_scan_items() {
    printf '%s\n' \
        "git|command|git" \
        "nvim|command|nvim" \
        "make|command|make" \
        "cmake|command|cmake" \
        "C compiler|compiler|" \
        "rg|command|rg" \
        "fd|command|" \
        "fzf|command|fzf" \
        "lsd|command|lsd" \
        "chezmoi|command|chezmoi" \
        "lazygit|command|lazygit" \
        "starship|command|starship" \
        "tmux|command|tmux" \
        "tmux plugins|tmux-plugins|" \
        "zsh|command|zsh" \
        "zsh plugins|zsh-plugins|" \
        "code|command|code" \
        "python3|command|python3" \
        "node|command|node" \
        "pi|command|pi" \
        "tree-sitter|command|tree-sitter" \
        "shellcheck|command|shellcheck" \
        "jq|command|jq" \
        "hyperfine|command|hyperfine" \
        "taplo|command|taplo" \
        "yamllint|command|yamllint" \
        "editorconfig-checker|command|editorconfig-checker"

    if [[ "$(uname -s)" == "Darwin" && ( "$PM" == "brew" || "$PM" == "brew_missing" ) ]]; then
        printf '%s\n' "ghostty|macos-cask|ghostty"
    elif [[ "$(uname -s)" == "Linux" ]]; then
        if ! is_wsl || wsl_gui_opt_in; then
            printf '%s\n' "ghostty|command|ghostty"
        fi
    fi

    if ! is_wsl || wsl_gui_opt_in; then
        printf '%s\n' "fc-cache|command|fc-cache" "Hack Nerd Font|font|"
    fi

    if is_wsl; then
        printf '%s\n' "win32yank.exe|command|win32yank.exe"
    elif [[ "$(uname -s)" == "Linux" ]]; then
        printf '%s\n' "xclip|command|xclip" "wl-copy|command|wl-copy"
    fi
}

install_scan_present() {
    local tool="$1" kind="$2" version_bin="${3:-}" bins
    case "$kind" in
        command)
            bins="$(binaries_for "$tool")"
            # shellcheck disable=SC2086  # $bins is intentional word-splitting
            have_any $bins
            ;;
        compiler)
            have_c_compiler
            ;;
        font)
            hack_nerd_font_installed
            ;;
        macos-cask)
            have "$tool" || homebrew_cask_installed "$version_bin"
            ;;
        zsh-plugins)
            local root fzf_tab_dir autosuggestions_dir
            root="$(zsh_plugin_root)"
            fzf_tab_dir="$root/fzf-tab"
            autosuggestions_dir="$root/zsh-autosuggestions"
            zsh_plugin_ok "$fzf_tab_dir" "https://github.com/Aloxaf/fzf-tab.git" "$FZF_TAB_COMMIT" "fzf-tab.plugin.zsh" &&
                zsh_plugin_ok "$autosuggestions_dir" "https://github.com/zsh-users/zsh-autosuggestions.git" "$ZSH_AUTOSUGGESTIONS_COMMIT" "zsh-autosuggestions.zsh"
            ;;
        tmux-plugins)
            local root tpm_dir
            root="$(tmux_plugin_root)"
            tpm_dir="$root/tpm"
            tmux_plugin_ok "$tpm_dir" "$TPM_COMMIT" "tpm" &&
                tmux_plugin_ok "$root/tmux-sensible" "$TMUX_SENSIBLE_COMMIT" "sensible.tmux" &&
                tmux_plugin_ok "$root/tmux-yank" "$TMUX_YANK_COMMIT" "yank.tmux" &&
                tmux_plugin_ok "$root/tmux-resurrect" "$TMUX_RESURRECT_COMMIT" "resurrect.tmux" &&
                tmux_plugin_ok "$root/tmux-continuum" "$TMUX_CONTINUUM_COMMIT" "continuum.tmux"
            ;;
        *)
            return 1
            ;;
    esac
}

install_scan_version() {
    local tool="$1" kind="$2" version_bin="${3:-}" bins candidate first_line
    case "$kind" in
        macos-cask)
            brew list --cask --versions "$version_bin" 2>/dev/null | sed -n '1p'
            return
            ;;
        font)
            printf '%s\n' "-"
            return
            ;;
        zsh-plugins)
            printf '%s\n' "$FZF_TAB_VERSION/$ZSH_AUTOSUGGESTIONS_VERSION"
            return
            ;;
        tmux-plugins)
            printf '%s\n' "tpm:${TPM_COMMIT}/sensible:${TMUX_SENSIBLE_COMMIT}/yank:${TMUX_YANK_COMMIT}/resurrect:${TMUX_RESURRECT_COMMIT}/continuum:${TMUX_CONTINUUM_COMMIT}"
            return
            ;;
        compiler)
            for candidate in cc gcc clang zig cl; do
                if have "$candidate"; then
                    version_bin="$candidate"
                    break
                fi
            done
            ;;
        command)
            if [[ -z "$version_bin" ]]; then
                bins="$(binaries_for "$tool")"
                for candidate in $bins; do
                    if have "$candidate"; then
                        version_bin="$candidate"
                        break
                    fi
                done
            fi
            ;;
    esac
    if [[ -z "$version_bin" ]] || ! have "$version_bin"; then
        printf '%s\n' "-"
        return
    fi
    first_line="$("$version_bin" --version 2>/dev/null | sed -n '1p' || true)"
    if [[ -z "$first_line" ]]; then
        first_line="-"
    fi
    printf '%s\n' "$first_line"
}

scan_install_dependencies() {
    local spec_source pm_item tool kind version_bin status version action
    if [[ -n "${INSTALL_DEPS_SCAN_ITEMS:-}" ]]; then
        spec_source="$INSTALL_DEPS_SCAN_ITEMS"
    else
        spec_source="$(install_dependency_scan_items)"
    fi
    pm_item="$(install_dependency_pm_scan_item)"
    if [[ -n "$pm_item" ]]; then
        spec_source="$(printf '%s\n%s\n' "$pm_item" "$spec_source")"
    fi
    while IFS='|' read -r tool kind version_bin; do
        [[ -n "$tool" ]] || continue
        status="missing"
        version="-"
        action="install"
        if install_scan_present "$tool" "$kind" "$version_bin"; then
            status="present"
            version="$(install_scan_version "$tool" "$kind" "$version_bin")"
            action="skip"
        fi
        printf '%s|%s|%s|%s\n' "$tool" "$status" "$version" "$action"
    done <<EOF
$spec_source
EOF
}

print_install_dependency_table() {
    local rows tool status version action present=0 missing=0
    rows="$(scan_install_dependencies)"
    echo "Dependency pre-flight:"
    printf "%-22s %-8s %-34s %-7s\n" "Tool" "Status" "Version" "Action"
    printf "%-22s %-8s %-34s %-7s\n" "----------------------" "--------" "----------------------------------" "-------"
    while IFS='|' read -r tool status version action; do
        [[ -n "$tool" ]] || continue
        printf "%-22s %-8s %-34s %-7s\n" "$tool" "$status" "$version" "$action"
        if [[ "$status" == "present" ]]; then
            present=$((present + 1))
        else
            missing=$((missing + 1))
        fi
    done <<EOF
$rows
EOF
    INSTALL_SCAN_MISSING=$missing
    printf "%s present, %s missing\n" "$present" "$missing"
}

# Test seam: `INSTALL_DEPS_SOURCE_ONLY=1 source install-deps.sh` defines the
# installer functions WITHOUT running any package installs.
if [[ -n "${INSTALL_DEPS_SOURCE_ONLY:-}" ]]; then
    # shellcheck disable=SC2317  # the exit is reached only when executed, not sourced
    return 0 2>/dev/null || exit 0
fi

if [[ "$UPDATE_ONLY" -eq 1 ]]; then
    UPDATE_RC=0
    run_update_mode || UPDATE_RC=$?
    audit_managed_cli_installations
    exit "$UPDATE_RC"
fi

PM="$(detect_pm)"
OS_LABEL="$(uname -s)"
if is_wsl; then OS_LABEL="WSL ($OS_LABEL)"; fi

echo "install-deps: OS=$OS_LABEL  package manager=$PM  dry-run=$DRY_RUN  yes-all=$YES_ALL  experimental-wsl-gui=$EXPERIMENTAL_WSL_GUI"
echo
print_install_dependency_table
echo

# One-shot "install everything" vs the per-item prompts. Skipped when --all was
# already passed, and when there's no tty to read from (e.g. curl | bash).
# Enter / Y == everything (recommended); n == choose per tool.
if [[ "$YES_ALL" -ne 1 && "$DRY_RUN" -ne 1 && -t 0 ]]; then
    printf "Install the %s missing tools listed above without further prompts? [Y/n]  (n = choose per tool) " "$INSTALL_SCAN_MISSING"
    if IFS= read -r _all_ans && [[ "$_all_ans" =~ ^[Nn] ]]; then
        echo "  -> per-item prompts"
    else
        YES_ALL=1
        echo "  -> installing everything; no further prompts"
    fi
    unset _all_ans
    echo
fi

# Bootstrap brew only after the pre-flight table and one-shot prompt. Re-detect
# after a real bootstrap; dry-run models Brew as available for every later
# preview phase without claiming that the bootstrap already happened.
if ! bootstrap_package_manager; then
    record_install_failure "Homebrew bootstrap/activation" brew shellenv 1
    echo "  FAIL: Homebrew bootstrap/activation is an unrecoverable package-manager precondition; no package installs were attempted." >&2
    exit_if_install_failures
fi

if ! link_homebrew_completions; then
    record_install_failure "Homebrew completions" brew "completions link + core _brew" 1
    echo "  FAIL: Homebrew completion linking is an unrecoverable shell-startup precondition; no package installs were attempted." >&2
    exit_if_install_failures
fi

if [[ "$PM" == "unknown" ]]; then
    echo "install-deps: no supported package manager found." >&2
    echo "  Supported: brew (mac/Linux), apt (Debian/Ubuntu), dnf (Fedora)," >&2
    echo "             pacman (Arch), zypper (openSUSE), apk (Alpine)." >&2
    exit 1
fi

# ---- Sections ----------------------------------------------------------------
section() { echo; echo "== $1 =="; }

section "core editor stack"
run_catalog_install git "version control, required by lazy.nvim"
if [[ "$(uname -s)" == "Linux" && "$PM" != "brew" ]]; then
    run_install_step nvim direct "$NVIM_LINUX_VERSION" install_nvim_linux
else
    run_catalog_install nvim "Neovim 0.12+, the editor"
fi
run_catalog_install make "needed for some plugin builds (notably LuaSnip jsregexp)"
run_catalog_install cmake "CMake CLI required by neocmakelsp and CMake projects"
run_install_step "C compiler" "$(native_linux_pm)" toolchain install_c_toolchain_linux
run_catalog_install rg "ripgrep, powers Telescope live_grep"
run_catalog_install fd "fd, powers Telescope find_files"
run_catalog_install fzf "fuzzy finder: Ctrl-R history, Ctrl-T files, Alt-C cd (zsh wiring in shells/zshrc)"
run_catalog_install lsd "modern ls replacement with colors, icons, and tree view"
run_catalog_install zoxide "smarter cd: z <dir> jumps by frecency, zi picks interactively (zsh/pwsh wiring in shell profiles)"
run_install_step chezmoi direct "$CHEZMOI_VERSION" install_chezmoi
run_install_step lazygit direct "$LAZYGIT_LINUX_VERSION" install_lazygit

section "prompt"
run_install_step starship direct "$STARSHIP_VERSION" install_starship

section "terminal multiplexer + shell"
run_catalog_install tmux
run_install_step "tmux plugins" git pinned-commits install_tmux_plugins
run_catalog_install zsh
run_install_step "zsh plugins" git pinned-commits install_zsh_plugins
set_default_shell_zsh   # make zsh the login shell so tmux/terminals launch it

section "terminals (optional)"
if [[ "$(uname -s)" == "Darwin" ]] && [[ "$PM" == "brew" ]]; then
    run_install_step ghostty brew cask:ghostty install_ghostty_macos
    run_install_step wezterm brew cask:wezterm install_wezterm_macos
elif [[ "$(uname -s)" == "Linux" ]]; then
    run_install_step ghostty direct "$GHOSTTY_UBUNTU_VERSION" install_ghostty_linux
    run_install_step wezterm direct "$WEZTERM_VERSION" install_wezterm_linux
fi
run_install_step devilspie2 "$(native_linux_pm)" devilspie2 setup_ghostty_maximize

section "window manager (macOS, optional)"
if [[ "$(uname -s)" == "Darwin" ]] && [[ "$PM" == "brew" ]]; then
    run_install_step aerospace brew cask:nikitabobko/tap/aerospace install_aerospace_macos
else
    printf "  skipped   %-26s AeroSpace is macOS-only\n" "aerospace"
fi

section "agent multiplexer (optional): Herdr"
run_install_step herdr direct "$HERDR_VERSION" install_herdr

section "editor: VS Code (optional)"
run_install_step vscode "${PM:-unknown}" catalog install_vscode
run_install_step "vscode theme" config rose-pine configure_vscode_rose_pine

section "fonts"
if is_wsl && ! wsl_gui_opt_in; then
    printf "  skipped   %-26s WSL renders through the Windows host terminal by default\n" "Hack Nerd Font"
    echo "            Run Windows setup with -MergeWindowsTerminal so Windows Terminal uses Hack Nerd Font."
    echo "            Linux fontconfig install is experimental: ./setup.sh --experimental-wsl-gui"
else
    run_catalog_install fc-cache "font config (needed to install Hack Nerd Font on Linux)"
    run_install_step "Hack Nerd Font" direct Hack.zip install_nerd_font
fi

section "language tooling (for LSP / formatter back-ends)"
run_catalog_install python3 "needed by pyright"
run_install_step "python venv/pip" "$(native_linux_pm)" python3-venv ensure_python_pip_venv
run_install_step latex2text direct "pylatexenc@$PYLATEXENC_VERSION" install_pylatexenc_converter
run_catalog_install node "needed by prettier and JS tooling"
run_install_step npm "$(native_linux_pm)" npm ensure_npm
run_install_step pi npm "$PI_CLI_PACKAGE@$PI_CLI_VERSION" install_pi_cli
run_install_step tree-sitter direct "$TREE_SITTER_CLI_LINUX_VERSION" install_tree_sitter_cli

if is_wsl; then
    section "WSL clipboard bridge"
    check_wsl_clipboard
elif [[ "$(uname -s)" == "Linux" ]]; then
    section "Linux clipboard helpers"
    run_catalog_install xclip "X11 clipboard"
    run_catalog_install wl-copy "Wayland clipboard"
fi

section "developer / test dependencies (optional)"
run_catalog_install shellcheck "shell script linter"
run_catalog_install jq "JSON CLI, general-purpose tool used by many scripts"
run_catalog_install gh "GitHub CLI (gh); also required by the gh-dash PR/issue dashboard"
run_install_step gh-dash gh "dlvhdr/gh-dash@$GH_DASH_VERSION" install_gh_dash_extension
run_catalog_install hyperfine "starship prompt perf test"
run_catalog_install taplo "TOML linter"
run_catalog_install yamllint "YAML linter"
run_catalog_install editorconfig-checker

section "notes / Obsidian vault (optional)"
configure_notes_vault

audit_managed_cli_installations
exit_if_install_failures
echo
echo "install-deps: done"
if [[ "$DRY_RUN" -eq 1 ]]; then echo "(dry run -- nothing was installed)"; fi
echo
echo "Next: run ./setup.sh, or let setup.sh continue if it invoked this phase."
