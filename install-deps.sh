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
#   ./install-deps.sh --dry-run print what would be installed without acting

set -euo pipefail

YES_ALL=0
DRY_RUN=0
NVIM_LINUX_VERSION="v0.12.2"
NVIM_LINUX_X86_64_SHA256="31cf85945cb600d96cdf69f88bc68bec814acbff50863c5546adef3a1bcef260"
NVIM_LINUX_ARM64_SHA256="f697d4e4582b6e4b5c3c26e76e06ce26efa08ba1768e03fd2733fcc422bb0490"
HACK_NERD_FONT_VERSION="v3.4.0"
HACK_NERD_FONT_SHA256="8ca33a60c791392d872b80d26c42f2bfa914a480f9eb2d7516d9f84373c36897"
# Ghostty on Ubuntu: we pin + SHA-256 verify the mkasberg/ghostty-ubuntu
# installer SCRIPT (one version + one checksum, like the Neovim / Hack pins).
# The script itself fetches the matching .deb from the same project's GitHub
# release assets over HTTPS at run time. Bump the version + SHA together.
GHOSTTY_UBUNTU_VERSION="1.3.1-0-ppa2"
GHOSTTY_UBUNTU_INSTALL_SHA256="7517776f6d862ec523e627840af4806e13385302f653ae9f7a86aa6d5af1cae5"
for arg in "$@"; do
    case "$arg" in
        --all|-y)   YES_ALL=1 ;;
        --dry-run)  DRY_RUN=1 ;;
        -h|--help)
            sed -n '2,12p' "$0"; exit 0 ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done

# ---- Bash 3.2-safe helpers ---------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }
verify_sha256() {
    local f="$1" expected="$2" got
    if have shasum; then
        got="$(shasum -a 256 "$f" | awk '{print $1}')"
    elif have sha256sum; then
        got="$(sha256sum "$f" | awk '{print $1}')"
    else
        echo "  FAIL: need shasum or sha256sum to verify $f" >&2
        return 1
    fi
    [[ "$got" == "$expected" ]]
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
is_ubuntu() {
    local id=""
    if [[ -r /etc/os-release ]]; then
        id="$(awk -F= '$1=="ID"{gsub(/"/, "", $2); print $2; exit}' /etc/os-release 2>/dev/null || true)"
    fi
    [[ "$id" == "ubuntu" ]]
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

homebrew_bin() {
    if have brew; then command -v brew; return 0; fi
    local candidate
    for candidate in /opt/homebrew/bin/brew /usr/local/bin/brew /home/linuxbrew/.linuxbrew/bin/brew "$HOME/.linuxbrew/bin/brew"; do
        if [[ -x "$candidate" ]]; then printf '%s\n' "$candidate"; return 0; fi
    done
    return 1
}

enable_homebrew_for_current_shell() {
    local brew_bin
    brew_bin="$(homebrew_bin)" || return 1
    eval "$("$brew_bin" shellenv)"
    hash -r 2>/dev/null || true
}

persist_homebrew_shellenv() {
    local brew_bin brew_prefix marker block rc wrote=0
    local rcs
    brew_bin="$(homebrew_bin)" || return 0
    brew_prefix="${brew_bin%/bin/brew}"
    marker="# >>> dotfiles: Homebrew shellenv >>>"
    block="$(cat <<EOF
$marker
if [ -x "$brew_prefix/bin/brew" ]; then
    eval "\$($brew_prefix/bin/brew shellenv)"
fi
# <<< dotfiles: Homebrew shellenv <<<
EOF
)"

    rcs=("$HOME/.zshrc.local")
    if [[ "$(uname -s)" == "Linux" ]]; then
        rcs+=("$HOME/.bashrc")
    fi

    for rc in "${rcs[@]}"; do
        if [[ -f "$rc" ]] && grep -qF "$marker" "$rc"; then
            continue
        fi
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: append Homebrew shellenv to $rc"
        else
            mkdir -p "$(dirname "$rc")"
            {
                printf '\n%s\n' "$block"
            } >> "$rc"
            wrote=1
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
    local kind
    if [[ "$(uname -s)" == "Darwin" ]]; then
        kind="required (no other package manager on macOS)"
    else
        kind="recommended (unlocks taplo / hyperfine / newer CLI tools that apt may not carry)"
    fi
    echo "Homebrew is not installed. $kind."
    if ask "Install Homebrew via the official installer?"; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
            return 1
        fi
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" || return 1
        # Plumb brew into THIS shell so subsequent installs use it, then make
        # future zsh/bash sessions find it without manual Homebrew "Next steps".
        enable_homebrew_for_current_shell || true
        persist_homebrew_shellenv
        return 0
    fi
    return 1
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

# Domain-account fallback: we can't chsh, so make INTERACTIVE bash re-exec into
# zsh. Idempotent (a marked block, re-run safe), interactive-only (scp/rsync and
# scripts stay bash), and it also points login shells (tmux, ssh) at ~/.bashrc
# so the guard fires there too. Reversible — delete the marked block to undo.
ensure_bash_execs_zsh() {
    local rc="$HOME/.bashrc" profile="$HOME/.bash_profile"
    local marker="# >>> dotfiles: exec zsh (domain login; chsh unavailable) >>>"
    if [[ -f "$rc" ]] && grep -qF "$marker" "$rc"; then
        echo "  ok        exec-zsh guard already present in ~/.bashrc"
    else
        cat >> "$rc" <<'EOF'

# >>> dotfiles: exec zsh (domain login; chsh unavailable) >>>
# Interactive bash re-execs into zsh. Guards: not already zsh (no loop), only
# interactive shells (scp/rsync/scripts stay bash), and zsh must be installed.
if [ -z "${ZSH_VERSION:-}" ] && [ -n "${BASH_VERSION:-}" ] && [[ $- == *i* ]] && command -v zsh >/dev/null 2>&1; then
    SHELL="$(command -v zsh)"; export SHELL
    exec zsh
fi
# <<< dotfiles: exec zsh (domain login; chsh unavailable) <<<
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
    local zsh_path="$1" current="$2"
    if ! ask "Make zsh your default login shell (chsh)? current: ${current:-unknown}"; then
        printf "  skipped   %-26s kept %s\n" "default shell" "${current:-current shell}"
        echo "            (~/.zshrc is symlinked, but tmux / new terminals keep"
        echo "             launching ${current##*/} until the login shell changes)"
        return 0
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: register $zsh_path in /etc/shells if needed, then chsh -s $zsh_path"
        return 0
    fi
    if ! ensure_in_etc_shells "$zsh_path"; then
        echo "  WARN: could not register $zsh_path in /etc/shells; skipping chsh"
        return 0
    fi
    if set_login_shell "$zsh_path"; then
        printf "  changed   %-26s login shell -> %s\n" "default shell" "$zsh_path"
        echo "            log out and back in for it to take effect (tmux started"
        echo "            after re-login then launches zsh, not bash)"
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

# Set "workbench.colorTheme":"Rosé Pine" in a VS Code settings.json. The value
# is a literal é (UTF-8) — it must match the theme label contributed by the
# mvllow.rose-pine extension exactly. Handles:
#   - absent/empty  -> write a fresh minimal settings.json
#   - valid JSON    -> jq-merge the key (preserves existing settings)
#   - JSONC/no jq   -> leave the file untouched and print an instruction
#                      (VS Code settings are usually JSONC; never clobber them)
set_vscode_theme() {
    local settings="${1:-}"
    [[ -n "$settings" ]] || settings="$(vscode_settings_path)"
    mkdir -p "$(dirname "$settings")" 2>/dev/null || true
    if [[ ! -s "$settings" ]]; then
        printf '{\n  "workbench.colorTheme": "Rosé Pine"\n}\n' > "$settings"
        printf "  set       %-26s workbench.colorTheme (new settings.json)\n" "rose-pine (vscode)"
        return 0
    fi
    if have jq && jq -e . "$settings" >/dev/null 2>&1; then
        local tmp; tmp="$(mktemp)"
        if jq '. + {"workbench.colorTheme":"Rosé Pine"}' "$settings" > "$tmp"; then
            mv "$tmp" "$settings"
            printf "  set       %-26s workbench.colorTheme (merged)\n" "rose-pine (vscode)"
        else
            rm -f "$tmp"
            echo "  WARN: could not merge theme into $settings"
        fi
        return 0
    fi
    echo "  note      set workbench.colorTheme to \"Rose Pine\" in $settings (left untouched: comments / no jq)"
    return 0
}

install_nvim_linux() {
    if have nvim; then
        printf "  ok        %-26s already installed\n" "nvim"
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
    rm -rf "$tmp"
    printf "  installed %-26s -> %s/bin/nvim\n" "nvim" "$install_dir"
}

install_ghostty_macos() {
    if have ghostty; then
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
    brew install --cask ghostty || echo "  WARN: ghostty cask install failed"
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
rg|ripgrep|ripgrep|ripgrep|ripgrep|ripgrep|ripgrep
fd|fd|fd-find|fd-find|fd|fd|fd
fzf|fzf|fzf|fzf|fzf|fzf|fzf
starship|starship||||||
tmux|tmux|tmux|tmux|tmux|tmux|tmux
zsh|zsh|zsh|zsh|zsh|zsh|zsh
python3|python@3.12|python3|python3|python|python311|python3
node|node|nodejs|nodejs|nodejs|nodejs|nodejs
shellcheck|shellcheck|shellcheck|ShellCheck|shellcheck|ShellCheck|shellcheck
jq|jq|jq|jq|jq|jq|jq
bats|bats-core|bats|bats|bats|bats|bats
hyperfine|hyperfine|hyperfine|hyperfine|hyperfine|hyperfine|hyperfine
taplo|taplo|||taplo-cli||
yamllint|yamllint|yamllint|yamllint|yamllint|yamllint|yamllint
editorconfig-checker|editorconfig-checker|||editorconfig-checker||
xclip||xclip|xclip|xclip|xclip|xclip
wl-copy||wl-clipboard|wl-clipboard|wl-clipboard|wl-clipboard|wl-clipboard
unzip|unzip|unzip|unzip|unzip|unzip|unzip
fc-cache|fontconfig|fontconfig|fontconfig|fontconfig|fontconfig|fontconfig
EOF
)

pkg_for() {
    local tool="$1"
    local row
    row=$(printf '%s\n' "$PKG_TABLE" | awk -F'|' -v t="$tool" '$1==t{print; exit}')
    [[ -z "$row" ]] && { echo ""; return; }
    local idx
    case "$PM" in
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

pm_install() {
    local pkgs=("$@")
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: $PM install ${pkgs[*]}"; return 0
    fi
    case "$PM" in
        brew)   brew install "${pkgs[@]}" ;;
        apt)    maybe_sudo apt-get update -qq && maybe_sudo apt-get install -y "${pkgs[@]}" ;;
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
        apt)    maybe_sudo apt-get update -qq && maybe_sudo apt-get install -y "${pkgs[@]}" ;;
        dnf)    maybe_sudo dnf install -y "${pkgs[@]}" ;;
        pacman) maybe_sudo pacman -S --noconfirm --needed "${pkgs[@]}" ;;
        zypper) maybe_sudo zypper install -y "${pkgs[@]}" ;;
        apk)    maybe_sudo apk add "${pkgs[@]}" ;;
        *)      return 1 ;;
    esac
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
                native_linux_pm_install apt build-essential || echo "  WARN: C compiler install failed; continuing"
            fi
            ;;
        dnf)
            if ask "Install C compiler toolchain (gcc gcc-c++ make)?"; then
                native_linux_pm_install dnf gcc gcc-c++ make || echo "  WARN: C compiler install failed; continuing"
            fi
            ;;
        pacman)
            if ask "Install C compiler toolchain (base-devel)?"; then
                native_linux_pm_install pacman base-devel || echo "  WARN: C compiler install failed; continuing"
            fi
            ;;
        zypper)
            if ask "Install C compiler toolchain (gcc gcc-c++ make)?"; then
                native_linux_pm_install zypper gcc gcc-c++ make || echo "  WARN: C compiler install failed; continuing"
            fi
            ;;
        apk)
            if ask "Install C compiler toolchain (build-base)?"; then
                native_linux_pm_install apk build-base || echo "  WARN: C compiler install failed; continuing"
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
        pm_install $pkg || echo "  WARN: $tool install failed; continuing"
        # Post-install hint for fd-find on apt (binary lands as 'fdfind',
        # not 'fd'). Telescope's find_files uses fd by default.
        if [[ "$tool" == "fd" ]] && [[ "$PM" == "apt" ]] && ! have fd && have fdfind; then
            echo "  note      apt installed fd as 'fdfind'. Telescope expects 'fd'."
            echo "            One-time alias:  mkdir -p ~/.local/bin && ln -sfn \"\$(command -v fdfind)\" ~/.local/bin/fd"
            echo "            and ensure ~/.local/bin is on PATH."
        fi
    else
        printf "  skipped   %-26s\n" "$tool"
    fi
}

# Starship has an official one-liner installer that works without a PM.
install_starship_curl() {
    if have starship; then
        printf "  ok        %-26s already installed\n" "starship"
        return
    fi
    if ask "Install starship (prompt) via official curl installer?"; then
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: curl -sS https://starship.rs/install.sh | sh -s -- -y"
        else
            curl -sS https://starship.rs/install.sh | sh -s -- -y || echo "  WARN: starship install failed"
        fi
    else
        printf "  skipped   %-26s\n" "starship"
    fi
}

install_nerd_font() {
    if fc-list 2>/dev/null | grep -qi "hack.*nerd"; then
        printf "  ok        %-26s already installed\n" "Hack Nerd Font"
        return
    fi
    if [[ "$PM" == "brew" ]] && [[ "$(uname -s)" == "Darwin" ]]; then
        if ask "Install Hack Nerd Font (used by ghostty / Windows Terminal)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: brew install --cask font-hack-nerd-font"
            else
                brew install --cask font-hack-nerd-font || echo "  WARN: cask install failed"
            fi
        fi
        return
    fi
    if ! ask "Install Hack Nerd Font (download + extract to user font dir)?"; then
        printf "  skipped   %-26s\n" "Hack Nerd Font"
        return
    fi
    require_downloader || return 1
    if ! have_any unzip bsdtar; then
        echo "  need      unzip missing; installing extractor for Hack Nerd Font"
        install unzip "extract Hack Nerd Font archive"
        if [[ "$DRY_RUN" -ne 1 ]] && ! have_any unzip bsdtar; then
            echo "  FAIL: need 'unzip' or 'bsdtar' to extract the font archive"
            return 1
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
    if ! curl -fL -o "$tmp/Hack.zip" "$url"; then
        echo "  FAIL: download failed; install Hack Nerd Font manually from nerd-fonts releases"
        rm -rf "$tmp"; return 1
    fi
    if ! verify_sha256 "$tmp/Hack.zip" "$HACK_NERD_FONT_SHA256"; then
        echo "  FAIL: checksum mismatch for Hack.zip"
        rm -rf "$tmp"; return 1
    fi
    mkdir -p "$font_dir"
    if have unzip; then
        unzip -oq "$tmp/Hack.zip" -d "$font_dir"
    else
        bsdtar -xf "$tmp/Hack.zip" -C "$font_dir"
    fi
    rm -rf "$tmp"
    if have fc-cache; then
        fc-cache -f "$(dirname "$font_dir")" >/dev/null 2>&1 || true
    fi
    printf "  installed %-26s -> %s\n" "Hack Nerd Font" "$font_dir"
}

# Download, SHA-256 verify, and run the pinned ghostty-ubuntu installer. We
# verify the installer SCRIPT before executing it (unlike a bare `curl | bash`),
# so an upstream change at the pinned tag fails closed instead of running blind.
# Caller must have already passed require_downloader (curl is used below).
run_ghostty_ubuntu_installer() {
    local url="$1" tmp script rc=0
    tmp="$(mktemp -d)"
    script="$tmp/ghostty-ubuntu-install.sh"
    if ! curl -fsSL -o "$script" "$url"; then
        echo "  FAIL: could not download ghostty installer"
        rm -rf "$tmp"; return 1
    fi
    if ! verify_sha256 "$script" "$GHOSTTY_UBUNTU_INSTALL_SHA256"; then
        echo "  FAIL: checksum mismatch for ghostty install.sh (pinned $GHOSTTY_UBUNTU_VERSION)"
        echo "        upstream changed; review it, then bump GHOSTTY_UBUNTU_VERSION + SHA together"
        rm -rf "$tmp"; return 1
    fi
    /bin/bash "$script" || rc=$?
    rm -rf "$tmp"
    return "$rc"
}

# Ghostty: Linux packaging varies. Homebrew's Ghostty formula is macOS-only,
# so Linux/WSL should prefer distro/community packages or manual install guidance.
install_ghostty_linux() {
    local ubuntu_url
    ubuntu_url="https://raw.githubusercontent.com/mkasberg/ghostty-ubuntu/${GHOSTTY_UBUNTU_VERSION}/install.sh"
    if have ghostty; then
        printf "  ok        %-26s already installed\n" "ghostty"
        return
    fi
    if is_wsl && ! can_show_gui; then
        printf "  skipped   %-26s WSL GUI display not detected; enable WSLg or use a Windows host terminal\n" "ghostty"
        return
    fi
    [[ "$PM" == "brew" ]] && printf "  skipped   %-26s Homebrew formula is macOS-only on Linux\n" "ghostty via brew"
    if is_ubuntu; then
        if ask "Install ghostty via Ubuntu .deb installer (mkasberg/ghostty-ubuntu, pinned $GHOSTTY_UBUNTU_VERSION)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: curl -fsSL $ubuntu_url"
                echo "         verify sha256 $GHOSTTY_UBUNTU_INSTALL_SHA256"
                echo "         bash install.sh   (fetches + apt-installs the matching .deb)"
            else
                require_downloader || return 1
                run_ghostty_ubuntu_installer "$ubuntu_url" || echo "  WARN: Ubuntu ghostty installer failed"
            fi
            return
        fi
    fi
    if have snap; then
        if ask "Install ghostty via snap (community package)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: sudo snap install ghostty --classic"
            else
                maybe_sudo snap install ghostty --classic || echo "  WARN: snap install failed"
            fi
            return
        fi
    fi
    echo "  manual    ghostty has no native $PM package. Options:"
    echo "              - ubuntu:  curl -fsSL $ubuntu_url | bash   (pinned $GHOSTTY_UBUNTU_VERSION)"
    echo "              - snap:    sudo snap install ghostty --classic"
    echo "              - flatpak: search 'ghostty' on flathub"
    echo "              - source:  https://ghostty.org/docs/install/build"
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
                brew install --cask visual-studio-code || echo "  WARN: cask install failed"
            fi
        fi
        return
    fi
    if have snap; then
        if ask "Install Visual Studio Code via snap?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: sudo snap install code --classic"
            else
                maybe_sudo snap install code --classic || echo "  WARN: snap install failed"
            fi
            return
        fi
    elif have flatpak; then
        if ask "Install Visual Studio Code via flatpak?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: flatpak install -y flathub com.visualstudio.code"
            else
                flatpak install -y flathub com.visualstudio.code || echo "  WARN: flatpak install failed"
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
        echo "  would: code --install-extension mvllow.rose-pine; set workbench.colorTheme = Rose Pine"
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
    install_devilspie2_linux || echo "  WARN: devilspie2 install failed; install it via your package manager"
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

# Test seam: `INSTALL_DEPS_SOURCE_ONLY=1 source install-deps.sh` defines the
# installer functions WITHOUT running any package installs.
if [[ -n "${INSTALL_DEPS_SOURCE_ONLY:-}" ]]; then
    # shellcheck disable=SC2317  # the exit is reached only when executed, not sourced
    return 0 2>/dev/null || exit 0
fi

PM="$(detect_pm)"
if [[ "$PM" == "brew" ]]; then
    enable_homebrew_for_current_shell || true
    persist_homebrew_shellenv
fi
OS_LABEL="$(uname -s)"
if is_wsl; then OS_LABEL="WSL ($OS_LABEL)"; fi

# Bootstrap brew if asked. Re-detect after.
if [[ "$PM" == "brew_missing" ]]; then
    if maybe_install_brew; then PM="$(detect_pm)"
    else PM="unknown"; fi
elif [[ "$PM" != "brew" && "$(uname -s)" == "Linux" ]]; then
    echo "Detected $PM as the system package manager."
    if maybe_install_brew; then PM="$(detect_pm)"; fi
fi

if [[ "$PM" == "unknown" ]]; then
    echo "install-deps: no supported package manager found." >&2
    echo "  Supported: brew (mac/Linux), apt (Debian/Ubuntu), dnf (Fedora)," >&2
    echo "             pacman (Arch), zypper (openSUSE), apk (Alpine)." >&2
    exit 1
fi

echo "install-deps: OS=$OS_LABEL  package manager=$PM  dry-run=$DRY_RUN  yes-all=$YES_ALL"
echo

# One-shot "install everything" vs the per-item prompts. Skipped when --all was
# already passed, and when there's no tty to read from (e.g. curl | bash).
# Enter / Y == everything (recommended); n == choose per tool.
if [[ "$YES_ALL" -ne 1 && "$DRY_RUN" -ne 1 && -t 0 ]]; then
    printf "Install EVERYTHING without further prompts? [Y/n]  (n = choose per tool) "
    if IFS= read -r _all_ans && [[ "$_all_ans" =~ ^[Nn] ]]; then
        echo "  -> per-item prompts"
    else
        YES_ALL=1
        echo "  -> installing everything; no further prompts"
    fi
    unset _all_ans
    echo
fi

# ---- Sections ----------------------------------------------------------------
section() { echo; echo "== $1 =="; }

section "core editor stack"
install git "version control, required by lazy.nvim"
if [[ "$(uname -s)" == "Linux" && "$PM" != "brew" ]]; then
    install_nvim_linux
else
    install nvim "Neovim 0.11+, the editor"
fi
install make "needed for some plugin builds (notably LuaSnip jsregexp)"
install_c_toolchain_linux
install rg "ripgrep, powers Telescope live_grep"
install fd "fd, powers Telescope find_files"
install fzf "fuzzy finder: Ctrl-R history, Ctrl-T files, Alt-C cd (zsh wiring in shells/zshrc)"

section "prompt"
if [[ "$PM" == "brew" ]]; then
    install starship
else
    install_starship_curl
fi

section "terminal multiplexer + shell"
install tmux
install zsh
set_default_shell_zsh   # make zsh the login shell so tmux/terminals launch it

section "terminals (optional)"
if [[ "$(uname -s)" == "Darwin" ]] && [[ "$PM" == "brew" ]]; then
    install_ghostty_macos
elif [[ "$(uname -s)" == "Linux" ]]; then
    install_ghostty_linux
fi
setup_ghostty_maximize   # GNOME/X11 only: enforce Ghostty maximize via devilspie2 (opt-in)

section "editor: VS Code (optional)"
install_vscode
configure_vscode_rose_pine

section "fonts"
install fc-cache "font config (needed to install Hack Nerd Font on Linux)"
install_nerd_font

section "language tooling (for LSP / formatter back-ends)"
install python3 "needed by pyright"
install node "needed by prettier and JS tooling"

if is_wsl; then
    section "WSL clipboard bridge"
    check_wsl_clipboard
elif [[ "$(uname -s)" == "Linux" ]]; then
    section "Linux clipboard helpers"
    install xclip "X11 clipboard"
    install wl-copy "Wayland clipboard"
fi

section "developer / test dependencies (optional)"
install shellcheck "shell script linter"
install jq "JSON CLI, general-purpose tool used by many scripts"
install bats "bats-core, for tests/bootstrap/sh_test.bats"
install hyperfine "starship prompt perf test"
install taplo "TOML linter"
install yamllint "YAML linter"
install editorconfig-checker

section "notes / Obsidian vault (optional)"
configure_notes_vault

echo
echo "install-deps: done"
if [[ "$DRY_RUN" -eq 1 ]]; then echo "(dry run -- nothing was installed)"; fi
echo
echo "Next: run ./setup.sh, or let setup.sh continue if it invoked this phase."
