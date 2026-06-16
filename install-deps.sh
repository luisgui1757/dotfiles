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
#   ./install-deps.sh --update  update only present package-manager tools
#   ./install-deps.sh --dry-run print what would be installed without acting
#   ./install-deps.sh --experimental-wsl-gui
#                              WSL opt-in: Linux Ghostty + Linux fontconfig fonts

set -euo pipefail

YES_ALL=0
DRY_RUN=0
UPDATE_ONLY=0
EXPERIMENTAL_WSL_GUI="${DOTFILES_EXPERIMENTAL_WSL_GUI:-0}"
NVIM_LINUX_VERSION="v0.12.2"
NVIM_LINUX_X86_64_SHA256="31cf85945cb600d96cdf69f88bc68bec814acbff50863c5546adef3a1bcef260"
NVIM_LINUX_ARM64_SHA256="f697d4e4582b6e4b5c3c26e76e06ce26efa08ba1768e03fd2733fcc422bb0490"
CHEZMOI_VERSION="v2.70.5"
LAZYGIT_LINUX_VERSION="v0.62.2"
LAZYGIT_LINUX_X86_64_SHA256="8b9a4c2d0969cbea92b45c956dd2a44e1ba76900c9df49f1c60984045ce77984"
LAZYGIT_LINUX_ARM64_SHA256="9ab63dd75a7e9711c4c68a37d77f4334b8099a5d6a3f8fbe8f4e2768b159c9e9"
TREE_SITTER_CLI_LINUX_VERSION="v0.26.9"
TREE_SITTER_CLI_LINUX_X86_64_SHA256="0ea5daaef79145fe73786f0e3cdc43b62b22ddb36f7f6676c9f8bb72434d78e9"
TREE_SITTER_CLI_LINUX_ARM64_SHA256="8b6c0f53593ce17c7eb90eb08de5ffb9f513f3db585b1fbef12219cacf7e8a68"
FZF_TAB_VERSION="v1.3.0"
FZF_TAB_COMMIT="d7e0234614dbe5369fdd760907d12c0e05a4dccc"
ZSH_AUTOSUGGESTIONS_VERSION="v0.7.1"
ZSH_AUTOSUGGESTIONS_COMMIT="e52ee8ca55bcc56a17c828767a3f98f22a68d4eb"
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
wsl_gui_opt_in() { [[ "${EXPERIMENTAL_WSL_GUI:-0}" == "1" ]]; }
is_ubuntu() {
    local id="" id_like=""
    if [[ -r /etc/os-release ]]; then
        id="$(awk -F= '$1=="ID"{gsub(/"/, "", $2); print $2; exit}' /etc/os-release 2>/dev/null || true)"
        id_like="$(awk -F= '$1=="ID_LIKE"{gsub(/"/, "", $2); print $2; exit}' /etc/os-release 2>/dev/null || true)"
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
    case "$(uname -s)" in
        Darwin)
            if homebrew_bin >/dev/null 2>&1; then echo brew
            else echo brew_missing
            fi
            ;;
        Linux)
            native_linux_pm
            ;;
        *)
            echo unknown
            ;;
    esac
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
    local backup timestamp
    timestamp="$(date +%Y%m%d%H%M%S)"
    backup="$settings.bak.$timestamp"
    while [[ -e "$backup" ]]; do
        backup="$settings.bak.$timestamp.$RANDOM"
    done
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
    if have nvim; then
        printf "  ok        %-26s already installed\n" "nvim"
        return
    fi
    if [[ "$(native_linux_pm)" == "apk" ]]; then
        if ! ask "Install nvim via apk (native Alpine package)?"; then
            printf "  skipped   %-26s\n" "nvim"
            return
        fi
        native_linux_pm_install apk neovim || {
            echo "  WARN: nvim install failed; continuing"
            return
        }
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

install_lazygit_linux() {
    if have lazygit; then
        printf "  ok        %-26s already installed\n" "lazygit"
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
            rm -rf "$tmp"
            printf "  installed %-26s -> /usr/local/bin/lazygit\n" "lazygit"
            return
        fi
        echo "  WARN: could not install lazygit to /usr/local/bin; trying user-local bin"
    fi

    install_target="$HOME/.local/bin/lazygit"
    mkdir -p "$HOME/.local/bin"
    cp "$tmp/lazygit" "$install_target"
    chmod 0755 "$install_target"
    rm -rf "$tmp"
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        PATH="$HOME/.local/bin:$PATH"
        export PATH
        hash -r 2>/dev/null || true
    fi
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

install_tree_sitter_cli_linux() {
    if have tree-sitter; then
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
    rm -rf "$tmp"
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        PATH="$HOME/.local/bin:$PATH"
        export PATH
        hash -r 2>/dev/null || true
    fi
    printf "  installed %-26s -> %s\n" "tree-sitter" "$install_target"
}

install_tree_sitter_cli() {
    if have tree-sitter; then
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
            native_linux_pm_install apk tree-sitter || \
                printf "  manual    %-26s apk add tree-sitter failed; install the musl tree-sitter CLI manually\n" "tree-sitter"
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
# fail with "ensurepip is not available" until venv + pip are present. brew, dnf,
# and pacman python already bundle them, so this only does work on apt systems.
ensure_python_pip_venv() {
    command -v python3 >/dev/null 2>&1 || return 0
    if python3 -c 'import ensurepip, venv' >/dev/null 2>&1; then
        printf "  ok        %-26s venv + pip present\n" "python venv/pip"
        return
    fi
    if [[ "$PM" == "brew" ]]; then
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
            native_linux_pm_install apt python3-venv python3-pip \
                || echo "  FAIL: python3-venv/python3-pip install failed (Mason PyPI tools will not build)"
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
            native_linux_pm_install "$native_pm" python3-pip \
                || echo "  WARN: python3-pip install failed; continuing"
            ;;
        *)
            printf "  manual    %-26s install your distro python3-venv + python3-pip\n" "python venv/pip"
            ;;
    esac
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
            native_linux_pm_install apt npm \
                || echo "  FAIL: npm install failed (Mason npm tools will not build)"
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
            native_linux_pm_install "$native_pm" npm \
                || echo "  WARN: npm install failed; continuing"
            ;;
        *)
            printf "  manual    %-26s install your distro npm package\n" "npm"
            ;;
    esac
}

install_chezmoi() {
    if have chezmoi; then
        printf "  ok        %-26s already installed\n" "chezmoi"
        return
    fi

    if [[ "$PM" == "brew" ]]; then
        install chezmoi "dotfiles config manager"
        return
    fi

    if [[ "$(uname -s)" != "Linux" ]]; then
        install chezmoi "dotfiles config manager"
        return
    fi

    if ! ask "Install chezmoi (dotfiles config manager, pinned ${CHEZMOI_VERSION})?"; then
        printf "  skipped   %-26s\n" "chezmoi"
        return
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: sh -c \"\$(curl -fsLS get.chezmoi.io)\" -- -b \"\$HOME/.local/bin\" -t \"$CHEZMOI_VERSION\""
        return
    fi

    require_downloader || return 1
    mkdir -p "$HOME/.local/bin"
    if sh -c "$(curl -fsLS get.chezmoi.io)" -- -b "$HOME/.local/bin" -t "$CHEZMOI_VERSION"; then
        if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
            PATH="$HOME/.local/bin:$PATH"
            export PATH
            hash -r 2>/dev/null || true
        fi
        printf "  installed %-26s %s -> %s\n" "chezmoi" "$CHEZMOI_VERSION" "$HOME/.local/bin/chezmoi"
    else
        echo "  WARN: chezmoi install failed; continuing"
    fi
}

zsh_plugin_root() {
    printf '%s\n' "${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins"
}

zsh_plugin_ok() {
    local target="$1" expected_commit="$2" plugin_file="$3" current
    [[ -d "$target/.git" ]] || return 1
    [[ -r "$target/$plugin_file" ]] || return 1
    current="$(git -C "$target" rev-parse HEAD 2>/dev/null || true)"
    [[ "$current" == "$expected_commit" ]]
}

install_zsh_plugin_repo() {
    local name="$1" repo="$2" ref="$3" expected_commit="$4" plugin_file="$5"
    local root target backup current
    root="$(zsh_plugin_root)"
    target="$root/$name"

    if zsh_plugin_ok "$target" "$expected_commit" "$plugin_file"; then
        printf "  ok        %-26s %s\n" "$name" "$ref"
        return 0
    fi

    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would: git clone --depth 1 --branch $ref $repo $target"
        echo "         verify git commit $expected_commit"
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
        git clone --depth 1 --branch "$ref" "$repo" "$target" >/dev/null 2>&1 || {
            printf "  WARN: could not clone %-18s from %s\n" "$name" "$repo"
            return 1
        }
    else
        if git -C "$target" remote get-url origin >/dev/null 2>&1; then
            git -C "$target" remote set-url origin "$repo"
        else
            git -C "$target" remote add origin "$repo"
        fi
        git -C "$target" fetch --depth 1 origin "refs/tags/$ref:refs/tags/$ref" >/dev/null 2>&1 || {
            printf "  WARN: could not fetch %-18s tag %s\n" "$name" "$ref"
            return 1
        }
        git -C "$target" checkout --force "$expected_commit" >/dev/null 2>&1 || {
            printf "  WARN: could not checkout %-18s commit %s\n" "$name" "$expected_commit"
            return 1
        }
    fi

    current="$(git -C "$target" rev-parse HEAD 2>/dev/null || true)"
    if [[ "$current" != "$expected_commit" ]]; then
        printf "  FAIL: %-26s got commit %s, expected %s\n" "$name" "${current:-unknown}" "$expected_commit"
        return 1
    fi
    if [[ ! -r "$target/$plugin_file" ]]; then
        printf "  FAIL: %-26s missing %s\n" "$name" "$plugin_file"
        return 1
    fi
    printf "  installed %-26s %s\n" "$name" "$ref"
}

install_zsh_plugins() {
    # fzf-tab gives the fzf-driven fuzzy Tab completion menu; zsh-autosuggestions
    # gives the inline gray history hint. See shells/zshrc + CLAUDE.md invariant 13.
    local root fzf_tab_dir autosuggestions_dir
    root="$(zsh_plugin_root)"
    fzf_tab_dir="$root/fzf-tab"
    autosuggestions_dir="$root/zsh-autosuggestions"

    if zsh_plugin_ok "$fzf_tab_dir" "$FZF_TAB_COMMIT" "fzf-tab.plugin.zsh" &&
        zsh_plugin_ok "$autosuggestions_dir" "$ZSH_AUTOSUGGESTIONS_COMMIT" "zsh-autosuggestions.zsh"; then
        printf "  ok        %-26s pinned refs already installed\n" "zsh plugins"
        return 0
    fi
    if ! ask "Install fzf-tab + zsh-autosuggestions (repo-managed pinned refs)?"; then
        printf "  skipped   %-26s\n" "zsh plugins"
        return 0
    fi

    # Attempt BOTH plugins (one failing must not skip the other), but do NOT
    # swallow the result with `|| true`: a swallowed failure let setup report
    # success while a required plugin was absent. Aggregate and emit a FAIL:
    # marker so CI catches it; real-user setup still continues (no set -e) -- the
    # fuzzy-Tab menu / gray hint is simply missing until the next good run.
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
        printf "  FAIL: %-26s one or more pinned zsh plugins failed to install\n" "zsh plugins" >&2
        return 1
    fi
    return 0
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
chezmoi|chezmoi|||||
lazygit|lazygit|||||
starship|starship||||||
tmux|tmux|tmux|tmux|tmux|tmux|tmux
zsh|zsh|zsh|zsh|zsh|zsh|zsh
python3|python@3.12|python3|python3|python|python311|python3
node|node|nodejs|nodejs|nodejs|nodejs|nodejs
tree-sitter|tree-sitter-cli|||||
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
tar|gnu-tar|tar|tar|tar|tar|tar
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
        apt)    maybe_sudo apt-get update -qq || echo "  WARN: apt-get update failed; installing from the existing apt cache" >&2
                maybe_sudo apt-get install -y "${pkgs[@]}" ;;
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
        apt)    maybe_sudo apt-get update -qq || echo "  WARN: apt-get update failed; installing from the existing apt cache" >&2
                maybe_sudo apt-get install -y "${pkgs[@]}" ;;
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

pm_update() {
    local tool="$1" pkg="$2"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        case "$PM" in
            brew)   printf "  would update %-26s via brew: brew upgrade %s\n" "$tool" "$pkg" ;;
            apt)    printf "  would update %-26s via apt: apt-get install --only-upgrade %s\n" "$tool" "$pkg" ;;
            dnf)    printf "  would update %-26s via dnf: dnf upgrade %s\n" "$tool" "$pkg" ;;
            pacman) printf "  would update %-26s via pacman: pacman -S %s\n" "$tool" "$pkg" ;;
            zypper) printf "  would update %-26s via zypper: zypper update %s\n" "$tool" "$pkg" ;;
            apk)    printf "  would update %-26s via apk: apk upgrade %s\n" "$tool" "$pkg" ;;
        esac
        return 0
    fi
    case "$PM" in
        brew)   brew upgrade "$pkg" ;;
        apt)    maybe_sudo apt-get update -qq || echo "  WARN: apt-get update failed; upgrading from the existing apt cache" >&2
                maybe_sudo apt-get install -y --only-upgrade "$pkg" ;;
        dnf)    maybe_sudo dnf upgrade -y "$pkg" ;;
        pacman) maybe_sudo pacman -S --noconfirm "$pkg" ;;
        zypper) maybe_sudo zypper update -y "$pkg" ;;
        apk)    maybe_sudo apk upgrade "$pkg" ;;
    esac
    local rc=$?
    if [[ "$rc" -eq 0 ]]; then
        printf "  updated   %-26s via %s\n" "$tool" "$PM"
    else
        printf "  WARN: %s update of %s returned %s\n" "$PM" "$pkg" "$rc" >&2
    fi
    return "$rc"
}

is_pinned_direct_update_tool() {
    local tool="$1"
    [[ "$(uname -s)" == "Linux" ]] || return 1
    case "$tool" in
        nvim|lazygit|tree-sitter) return 0 ;;
        *) return 1 ;;
    esac
}

update_catalog_tool() {
    local tool="$1" pkg
    [[ -n "$tool" ]] || return 0

    if is_pinned_direct_update_tool "$tool"; then
        printf "  skipped   %-26s pinned Linux direct download; update via git pull + setup\n" "$tool"
        return 0
    fi

    if ! update_tool_present "$tool"; then
        printf "  skipped   %-26s not installed\n" "$tool"
        return 0
    fi

    pkg="$(pkg_for "$tool")"
    if [[ -z "$pkg" ]]; then
        printf "  skipped   %-26s no %s package in catalog\n" "$tool" "$PM"
        return 0
    fi

    if ! pm_pkg_installed "$PM" "$pkg"; then
        printf "  skipped   %-26s present, but %s does not manage %s\n" "$tool" "$PM" "$pkg"
        return 0
    fi

    pm_update "$tool" "$pkg" || true
}

update_catalog_tools() {
    local tool
    while IFS= read -r tool; do
        [[ -n "$tool" ]] || continue
        update_catalog_tool "$tool"
    done <<EOF
$(catalog_tools)
EOF
}

run_update_mode() {
    PM="$(detect_update_pm)"
    OS_LABEL="$(uname -s)"
    if is_wsl; then OS_LABEL="WSL ($OS_LABEL)"; fi
    echo "install-deps: update mode OS=$OS_LABEL  package manager=$PM  dry-run=$DRY_RUN"
    echo

    if [[ "$PM" == "brew_missing" ]]; then
        echo "install-deps: Homebrew is not installed; update mode will not bootstrap it." >&2
        return 1
    fi
    if [[ "$PM" == "unknown" ]]; then
        echo "install-deps: no supported package manager found for update mode." >&2
        return 1
    fi
    if [[ "$PM" == "brew" ]]; then
        enable_homebrew_for_current_shell || true
    fi

    update_catalog_tools
    echo
    echo "note: pinned binaries (Neovim/lazygit/tree-sitter Linux archives, Hack Nerd Font, Windows Terminal portable), PSFzf, plugins, and configs update via git pull and re-running setup."
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
            if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
                PATH="$HOME/.local/bin:$PATH"
                export PATH
                hash -r 2>/dev/null || true
            fi
            printf "  set       %-26s ~/.local/bin/fd -> fdfind\n" "fd"
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
            echo "  would: curl -fsSL https://starship.rs/install.sh | sh -s -- -y"
        else
            # -f: fail (non-zero) on an HTTP 4xx/5xx instead of piping an error
            # page into sh. -L: follow redirects. Keeps the documented official
            # curl|sh install path, just makes the fetch fail-closed.
            curl -fsSL https://starship.rs/install.sh | sh -s -- -y || echo "  WARN: starship install failed"
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
    if is_ubuntu; then
        if ask "Install ghostty via Ubuntu .deb installer (mkasberg/ghostty-ubuntu, pinned $GHOSTTY_UBUNTU_VERSION)?"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: curl -fsSL $ubuntu_url"
                echo "         verify sha256 $GHOSTTY_UBUNTU_INSTALL_SHA256"
                echo "         bash install.sh   (fetches + apt-installs the matching .deb)"
            else
                require_downloader || return 1
                if ! run_ghostty_ubuntu_installer "$ubuntu_url"; then
                    echo "  WARN: Ubuntu ghostty installer failed; continuing"
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
                maybe_sudo snap install ghostty --classic || echo "  WARN: snap install failed"
            fi
            return
        fi
    fi
    echo "  manual    ghostty has no native $PM package. Options:"
    echo "              - ubuntu:  re-run this script for the verified pinned installer"
    echo "              - manual:  curl -fsSL $ubuntu_url | bash   (unverified fallback, pinned $GHOSTTY_UBUNTU_VERSION)"
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
        "C compiler|compiler|" \
        "rg|command|rg" \
        "fd|command|" \
        "fzf|command|fzf" \
        "chezmoi|command|chezmoi" \
        "lazygit|command|lazygit" \
        "starship|command|starship" \
        "tmux|command|tmux" \
        "zsh|command|zsh" \
        "zsh plugins|zsh-plugins|" \
        "code|command|code" \
        "python3|command|python3" \
        "node|command|node" \
        "tree-sitter|command|tree-sitter" \
        "shellcheck|command|shellcheck" \
        "jq|command|jq" \
        "bats|command|bats" \
        "hyperfine|command|hyperfine" \
        "taplo|command|taplo" \
        "yamllint|command|yamllint" \
        "editorconfig-checker|command|editorconfig-checker"

    if [[ "$(uname -s)" == "Darwin" && ( "$PM" == "brew" || "$PM" == "brew_missing" ) ]]; then
        printf '%s\n' "ghostty|command|ghostty"
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
    local tool="$1" kind="$2" bins
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
            fc-list 2>/dev/null | grep -qi "hack.*nerd"
            ;;
        zsh-plugins)
            local root fzf_tab_dir autosuggestions_dir
            root="$(zsh_plugin_root)"
            fzf_tab_dir="$root/fzf-tab"
            autosuggestions_dir="$root/zsh-autosuggestions"
            zsh_plugin_ok "$fzf_tab_dir" "$FZF_TAB_COMMIT" "fzf-tab.plugin.zsh" &&
                zsh_plugin_ok "$autosuggestions_dir" "$ZSH_AUTOSUGGESTIONS_COMMIT" "zsh-autosuggestions.zsh"
            ;;
        *)
            return 1
            ;;
    esac
}

install_scan_version() {
    local tool="$1" kind="$2" version_bin="${3:-}" bins candidate first_line
    case "$kind" in
        font)
            printf '%s\n' "-"
            return
            ;;
        zsh-plugins)
            printf '%s\n' "$FZF_TAB_VERSION/$ZSH_AUTOSUGGESTIONS_VERSION"
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
        if install_scan_present "$tool" "$kind"; then
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
    run_update_mode
    exit $?
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
# after bootstrap so the per-tool installers see the manager that now exists.
if [[ "$PM" == "brew" ]]; then
    enable_homebrew_for_current_shell || true
    persist_homebrew_shellenv
elif [[ "$PM" == "brew_missing" ]]; then
    if maybe_install_brew; then PM="$(detect_pm)"
    else PM="unknown"; fi
elif [[ "$(uname -s)" == "Linux" ]]; then
    echo "Detected $PM as the system package manager."
    if maybe_install_brew; then PM="$(detect_pm)"; fi
fi

if [[ "$PM" == "brew" ]]; then
    enable_homebrew_for_current_shell || true
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
install git "version control, required by lazy.nvim"
if [[ "$(uname -s)" == "Linux" && "$PM" != "brew" ]]; then
    install_nvim_linux
else
    install nvim "Neovim 0.12+, the editor"
fi
install make "needed for some plugin builds (notably LuaSnip jsregexp)"
install_c_toolchain_linux
install rg "ripgrep, powers Telescope live_grep"
install fd "fd, powers Telescope find_files"
install fzf "fuzzy finder: Ctrl-R history, Ctrl-T files, Alt-C cd (zsh wiring in shells/zshrc)"
install_chezmoi
install_lazygit

section "prompt"
if [[ "$PM" == "brew" ]]; then
    install starship
else
    install_starship_curl
fi

section "terminal multiplexer + shell"
install tmux
install zsh
install_zsh_plugins
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
if is_wsl && ! wsl_gui_opt_in; then
    printf "  skipped   %-26s WSL renders through the Windows host terminal by default\n" "Hack Nerd Font"
    echo "            Run Windows setup with -MergeWindowsTerminal so Windows Terminal uses Hack Nerd Font."
    echo "            Linux fontconfig install is experimental: ./setup.sh --experimental-wsl-gui"
else
    install fc-cache "font config (needed to install Hack Nerd Font on Linux)"
    install_nerd_font
fi

section "language tooling (for LSP / formatter back-ends)"
install python3 "needed by pyright"
ensure_python_pip_venv
install node "needed by prettier and JS tooling"
ensure_npm
install_tree_sitter_cli

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
install bats "bats-core, for optional local shell tests"
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
