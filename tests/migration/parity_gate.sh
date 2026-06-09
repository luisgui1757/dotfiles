#!/usr/bin/env bash
set -euo pipefail

# Determinism: the legacy install_zsh_plugins slice honors ${XDG_DATA_HOME:-...},
# while the chezmoi externals install to a fixed ~/.local/share path. Unset
# XDG_DATA_HOME so BOTH sides resolve to $HOME/.local/share and P3/P4 is an
# apples-to-apples comparison. (An XDG_DATA_HOME-aware managed root is Wave B.)
unset XDG_DATA_HOME

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SRC="$REPO_ROOT/home"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

pass() {
    echo "PASS: $*"
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        fail "required command '$1' is not on PATH"
    fi
}

sha() {
    local path="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$path" | cut -d' ' -f1
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$path" | cut -d' ' -f1
    else
        fail "sha256sum or shasum is required"
    fi
}

mode() {
    local path="$1"
    if stat -c '%a' "$path" >/dev/null 2>&1; then
        stat -c '%a' "$path"
    else
        stat -f '%Lp' "$path"
    fi
}

deref() {
    local path="$1"
    if readlink -f "$path" >/dev/null 2>&1; then
        readlink -f "$path"
    else
        python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$path"
    fi
}

git_head() {
    local dir="$1"
    git -C "$dir" rev-parse HEAD
}

# label|type|darwin target|linux target|canonical repo source|chezmoi source copy
# Target paths mirror bootstrap.sh's universal links and darwin/linux branches.
# Windows-only copied configs keep empty POSIX targets but remain in this one
# manifest so the single-source assert covers them.
manifest_entries() {
    cat <<'EOF'
tmux.conf|config-file|.tmux.conf|.tmux.conf|tmux/tmux.conf|home/dot_tmux.conf
tmux.windows.conf|config-file|||tmux/tmux.windows.conf|home/dot_tmux.windows.conf
lazygit config|config-file|Library/Application Support/lazygit/config.yml|.config/lazygit/config.yml|lazygit/config.yml|home/.chezmoitemplates/lazygit/config.yml
nvim|nvim|.config/nvim|.config/nvim|nvim|
starship|config-file|.config/starship.toml|.config/starship.toml|starship/starship.toml|home/dot_config/starship.toml
zshenv|config-file|.zshenv|.zshenv|shells/zshenv|home/dot_zshenv
zshrc|config-file|.zshrc|.zshrc|shells/zshrc|home/dot_zshrc
ghostty config|config-file|Library/Application Support/com.mitchellh.ghostty/config|.config/ghostty/config|ghostty/config|home/.chezmoitemplates/ghostty/config
powershell profile|config-file|||shells/powershell_profile.ps1|home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1
EOF
}

target_rel_for_host() {
    local darwin_rel="$1" linux_rel="$2"

    case "$target_os" in
        darwin)
            printf '%s\n' "$darwin_rel"
            ;;
        linux)
            printf '%s\n' "$linux_rel"
            ;;
        *)
            fail "unsupported target_os for manifest lookup: $target_os"
            ;;
    esac
}

assert_exists_both() {
    local label="$1" rel_path="$2" old_path new_path

    old_path="$HOME_OLD/$rel_path"
    new_path="$HOME_NEW/$rel_path"

    [[ -e "$old_path" ]] || fail "$label: missing old path $old_path"
    [[ -e "$new_path" ]] || fail "$label: missing new path $new_path"
    pass "$label: exists on both sides"
}

assert_config_file_pair() {
    local label="$1" rel_path="$2" old_path new_path old_real new_real old_sha new_sha old_mode new_mode

    old_path="$HOME_OLD/$rel_path"
    new_path="$HOME_NEW/$rel_path"

    assert_exists_both "$label" "$rel_path"

    [[ -L "$old_path" ]] || fail "$label: old path is not a symlink: $old_path"
    [[ -L "$new_path" ]] || fail "$label: new path is not a symlink: $new_path"
    pass "$label: same type on both sides (symlinked config file)"

    old_real="$(deref "$old_path")"
    new_real="$(deref "$new_path")"
    [[ -f "$old_real" ]] || fail "$label: old symlink does not dereference to a file: $old_real"
    [[ -f "$new_real" ]] || fail "$label: new symlink does not dereference to a file: $new_real"

    old_sha="$(sha "$old_real")"
    new_sha="$(sha "$new_real")"
    [[ "$old_sha" == "$new_sha" ]] || fail "$label: dereferenced SHA mismatch old=$old_sha new=$new_sha"
    pass "$label: dereferenced content SHA matches"

    old_mode="$(mode "$old_real")"
    new_mode="$(mode "$new_real")"
    [[ "$old_mode" == "$new_mode" ]] || fail "$label: dereferenced mode mismatch old=$old_mode new=$new_mode"
    pass "$label: dereferenced mode matches ($old_mode)"
}

assert_nvim_pair() {
    local label="$1" rel_path="$2" old_path new_path old_real new_real expected_real diff_output

    old_path="$HOME_OLD/$rel_path"
    new_path="$HOME_NEW/$rel_path"

    assert_exists_both "$label" "$rel_path"

    [[ -L "$old_path" ]] || fail "$label: old path is not a directory symlink: $old_path"
    [[ -L "$new_path" ]] || fail "$label: new path is not a directory symlink: $new_path"
    pass "$label: same type on both sides (directory symlink)"

    old_real="$(deref "$old_path")"
    new_real="$(deref "$new_path")"
    expected_real="$(deref "$REPO_ROOT/nvim")"
    [[ "$old_real" == "$expected_real" ]] || fail "$label: old symlink resolves to $old_real, expected $expected_real"
    [[ "$new_real" == "$expected_real" ]] || fail "$label: new symlink resolves to $new_real, expected $expected_real"
    [[ "$old_real" == "$new_real" ]] || fail "$label: realpath mismatch old=$old_real new=$new_real"
    pass "$label: both sides dereference to repo nvim ($expected_real)"

    if ! diff_output="$(diff -r "$old_real" "$new_real" 2>&1)"; then
        printf '%s\n' "$diff_output" >&2
        fail "$label: dereferenced nvim trees differ"
    fi
    pass "$label: dereferenced nvim tree diff is empty"
}

assert_manifest_pair() {
    local label="$1" entry_type="$2" rel_path="$3"

    case "$entry_type" in
        config-file)
            assert_config_file_pair "$label" "$rel_path"
            ;;
        nvim)
            assert_nvim_pair "$label" "$rel_path"
            ;;
        *)
            fail "$label: unknown manifest type $entry_type"
            ;;
    esac
}

assert_plugin_pair() {
    local label="$1" rel_dir="$2" old_dir new_dir old_head new_head

    old_dir="$HOME_OLD/$rel_dir"
    new_dir="$HOME_NEW/$rel_dir"

    [[ -d "$old_dir/.git" ]] || fail "$label: old git checkout missing at $old_dir"
    [[ -d "$new_dir/.git" ]] || fail "$label: new git checkout missing at $new_dir"
    pass "$label: git checkouts exist on both sides"

    old_head="$(git_head "$old_dir")"
    new_head="$(git_head "$new_dir")"
    [[ "$old_head" == "$new_head" ]] || fail "$label: HEAD mismatch old=$old_head new=$new_head"
    pass "$label: HEAD matches ($old_head)"
}

assert_single_sources() {
    local label entry_type darwin_rel linux_rel repo_rel home_rel repo_path home_path repo_sha home_sha

    while IFS='|' read -r label entry_type darwin_rel linux_rel repo_rel home_rel; do
        [[ "$entry_type" != "nvim" ]] || continue
        [[ -n "$home_rel" ]] || continue

        repo_path="$REPO_ROOT/$repo_rel"
        home_path="$REPO_ROOT/$home_rel"
        [[ -f "$repo_path" ]] || fail "$label: missing canonical repo source $repo_path"
        [[ -f "$home_path" ]] || fail "$label: missing chezmoi source copy $home_path"

        repo_sha="$(sha "$repo_path")"
        home_sha="$(sha "$home_path")"
        [[ "$repo_sha" == "$home_sha" ]] || \
            fail "$label: single-source SHA mismatch repo=$repo_sha home=$home_sha"
        pass "$label: single-source SHA matches"
    done < <(manifest_entries)
}

assert_manifest_for_host() {
    local label entry_type darwin_rel linux_rel repo_rel home_rel rel_path

    while IFS='|' read -r label entry_type darwin_rel linux_rel repo_rel home_rel; do
        rel_path="$(target_rel_for_host "$darwin_rel" "$linux_rel")"
        if [[ -z "$rel_path" ]]; then
            pass "$label: skipped on $target_os"
            continue
        fi
        assert_manifest_pair "$label" "$entry_type" "$rel_path"
    done < <(manifest_entries)
}

assert_absent_path() {
    local label="$1" rel_path="$2" path

    path="$HOME_NEW/$rel_path"
    if [[ -e "$path" || -L "$path" ]]; then
        fail "$label: wrong-OS path exists after apply: $path"
    fi
    pass "$label: wrong-OS path absent"
}

assert_wrong_os_absent() {
    case "$target_os" in
        darwin)
            assert_absent_path "wrong-OS Windows root" "AppData"
            assert_absent_path "wrong-OS PowerShell Documents root" "Documents"
            assert_absent_path "wrong-OS Linux lazygit path" ".config/lazygit"
            assert_absent_path "wrong-OS Linux ghostty path" ".config/ghostty"
            ;;
        linux)
            assert_absent_path "wrong-OS Windows root" "AppData"
            assert_absent_path "wrong-OS PowerShell Documents root" "Documents"
            assert_absent_path "wrong-OS macOS Library root" "Library"
            ;;
        *)
            fail "unsupported target_os for wrong-OS check: $target_os"
            ;;
    esac
}

require_cmd bash
require_cmd chezmoi
require_cmd diff
require_cmd git
require_cmd python3

case "$(uname -s)" in
    Darwin)
        target_os="darwin"
        ;;
    Linux)
        target_os="linux"
        ;;
    *)
        fail "unsupported parity host OS: $(uname -s); expected Linux or Darwin"
        ;;
esac
pass "detected parity host OS: $target_os"

HOME_OLD="$(mktemp -d)"
HOME_NEW="$(mktemp -d)"
trap 'rm -rf "$HOME_OLD" "$HOME_NEW"' EXIT

env HOME="$HOME_OLD" "$REPO_ROOT/bootstrap.sh"
pass "old path bootstrap slice applied"

# Interpolate REPO_ROOT into the command string so install-deps.sh is sourced
# with EMPTY positional args. Its top-level arg parser (install-deps.sh:38-47)
# runs on `$@` BEFORE the INSTALL_DEPS_SOURCE_ONLY seam, so a stray "$REPO_ROOT"
# passed as $1 would be rejected as "Unknown arg" and exit 2.
# Set YES_ALL=1 AFTER sourcing: install-deps.sh:17 initializes YES_ALL=0 at
# source time, which clobbers an env-passed YES_ALL=1. Without the auto-accept,
# install_zsh_plugins' ask() gate (install-deps.sh:199-207) skips the install on
# a non-interactive stdin and P3/P4 false-FAIL.
env HOME="$HOME_OLD" INSTALL_DEPS_SOURCE_ONLY=1 bash -c \
    'source "'"$REPO_ROOT"'/install-deps.sh"; YES_ALL=1; install_zsh_plugins'
pass "old path zsh plugin slice applied"

env HOME="$HOME_NEW" chezmoi --source "$SRC" init
env HOME="$HOME_NEW" chezmoi --source "$SRC" apply
pass "new path chezmoi apply completed"

assert_manifest_for_host

assert_plugin_pair \
    "P3 zsh-autocomplete" \
    ".local/share/dotfiles/zsh-plugins/zsh-autocomplete"
assert_plugin_pair \
    "P4 zsh-autosuggestions" \
    ".local/share/dotfiles/zsh-plugins/zsh-autosuggestions"

assert_single_sources
assert_wrong_os_absent

second_apply_output=""
if ! second_apply_output="$(env HOME="$HOME_NEW" chezmoi --source "$SRC" apply 2>&1)"; then
    printf '%s\n' "$second_apply_output" >&2
    fail "new path second chezmoi apply exited nonzero"
fi
if [[ -n "${second_apply_output//[[:space:]]/}" ]]; then
    echo "Second chezmoi apply output:" >&2
    printf '%s\n' "$second_apply_output" >&2
    fail "new path second chezmoi apply produced output"
fi
pass "new path second chezmoi apply is idempotent"

env HOME="$HOME_NEW" chezmoi --source "$SRC" verify || \
    fail "new path chezmoi verify failed"
pass "new path chezmoi verify clean"

doctor_output=""
doctor_rc=0
doctor_output="$(env HOME="$HOME_NEW" chezmoi --source "$SRC" doctor 2>&1)" || doctor_rc=$?
if grep -Eq '^[[:space:]]*error([[:space:]:]|$)' <<<"$doctor_output"; then
    echo "chezmoi doctor output:" >&2
    printf '%s\n' "$doctor_output" >&2
    fail "new path chezmoi doctor reported error-level results"
fi
if [[ "$doctor_rc" -ne 0 ]]; then
    echo "chezmoi doctor output:" >&2
    printf '%s\n' "$doctor_output" >&2
    fail "new path chezmoi doctor exited $doctor_rc without a leading error row"
fi
pass "new path chezmoi doctor has no error-level results"
