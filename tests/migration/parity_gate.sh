#!/usr/bin/env bash
set -euo pipefail

# Determinism: zsh plugin externals install to a fixed ~/.local/share path.
# The gate sets XDG_DATA_HOME to a hostile value below so fixed-root drift cannot
# pass by accidentally inheriting the default.

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

deref() {
    local path="$1"
    if readlink -f "$path" >/dev/null 2>&1; then
        readlink -f "$path"
    else
        python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$path"
    fi
}

# label|type|darwin target|linux target|canonical repo source|chezmoi source copy
# Target paths mirror the supported POSIX config layer for this host.
# Windows-only copied configs keep empty POSIX targets but remain in this one
# manifest so the single-source assert covers them.
manifest_entries() {
    cat <<'EOF'
tmux.conf|config-file|.tmux.conf|.tmux.conf|tmux/tmux.conf|home/dot_tmux.conf
tmux.posix.conf|config-file|.tmux.posix.conf|.tmux.posix.conf|tmux/tmux.posix.conf|home/dot_tmux.posix.conf
psmux.conf|config-file|||tmux/psmux.conf|home/dot_psmux.conf
tmux.windows.conf|config-file|||tmux/tmux.windows.conf|home/dot_tmux.windows.conf
psmux rose-pine renderer|config-file|||tmux/psmux-rose-pine.ps1|home/dot_tmux.rose-pine.ps1
rose-pine main conf|config-file|.tmux.rose-pine.main.conf|.tmux.rose-pine.main.conf|tmux/psmux-rose-pine.main.conf|home/dot_tmux.rose-pine.main.conf
rose-pine moon conf|config-file|.tmux.rose-pine.moon.conf|.tmux.rose-pine.moon.conf|tmux/psmux-rose-pine.moon.conf|home/dot_tmux.rose-pine.moon.conf
rose-pine dawn conf|config-file|.tmux.rose-pine.dawn.conf|.tmux.rose-pine.dawn.conf|tmux/psmux-rose-pine.dawn.conf|home/dot_tmux.rose-pine.dawn.conf
lazygit config|config-file|Library/Application Support/lazygit/config.yml|.config/lazygit/config.yml|lazygit/config.yml|home/.chezmoitemplates/lazygit/config.yml
lsd config|config-file|.config/lsd/config.yaml|.config/lsd/config.yaml|lsd/config.yaml|home/dot_config/lsd/config.yaml
lsd colors|config-file|.config/lsd/colors.yaml|.config/lsd/colors.yaml|lsd/colors.yaml|home/dot_config/lsd/colors.yaml
nvim|nvim|.config/nvim|.config/nvim|nvim|
starship|config-file|.config/starship.toml|.config/starship.toml|starship/starship.toml|home/dot_config/starship.toml
zshenv|config-file|.zshenv|.zshenv|shells/zshenv|home/dot_zshenv
zshrc|config-file|.zshrc|.zshrc|shells/zshrc|home/dot_zshrc
ghostty config|config-file|Library/Application Support/com.mitchellh.ghostty/config|.config/ghostty/config|ghostty/config|home/.chezmoitemplates/ghostty/config
powershell profile|config-file|||shells/powershell_profile.ps1|home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1
EOF
}

# label|canonical repo source|chezmoi source copy
# Shared template-only sources with no POSIX host target belong here, not in the
# per-OS apply manifest above.
single_source_entries() {
    cat <<'EOF'
windows-terminal fragment|windows-terminal/settings.fragment.jsonc|home/.chezmoitemplates/windows-terminal/settings.fragment.jsonc
windows lazygit config|lazygit/config.windows.yml|home/.chezmoitemplates/lazygit/config.windows.yml
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

assert_config_file_canonical() {
    local label="$1" rel_path="$2" repo_rel="$3" new_path repo_path new_real new_sha repo_sha

    new_path="$APPLY_HOME/$rel_path"
    repo_path="$REPO_ROOT/$repo_rel"

    [[ -e "$new_path" ]] || fail "$label: missing new path $new_path"
    [[ -L "$new_path" ]] || fail "$label: new target is not a symlink: $new_path"
    [[ -f "$repo_path" ]] || fail "$label: missing canonical repo source $repo_path"

    new_real="$(deref "$new_path")"
    [[ -f "$new_real" ]] || fail "$label: new target does not dereference to a file: $new_real"

    new_sha="$(sha "$new_real")"
    repo_sha="$(sha "$repo_path")"
    [[ "$new_sha" == "$repo_sha" ]] || fail "$label: canonical SHA mismatch new=$new_sha repo=$repo_sha"
    pass "$label: dereferenced content matches canonical source"
}

assert_nvim_canonical() {
    local label="$1" rel_path="$2" repo_rel="$3" new_path new_real expected_real diff_output

    new_path="$APPLY_HOME/$rel_path"
    expected_real="$(deref "$REPO_ROOT/$repo_rel")"

    [[ -e "$new_path" ]] || fail "$label: missing new path $new_path"
    [[ -L "$new_path" ]] || fail "$label: new path is not a directory symlink: $new_path"

    new_real="$(deref "$new_path")"
    [[ "$new_real" == "$expected_real" ]] || fail "$label: new symlink resolves to $new_real, expected $expected_real"
    pass "$label: new side dereferences to repo nvim ($expected_real)"

    if ! diff_output="$(diff -r "$new_real" "$expected_real" 2>&1)"; then
        printf '%s\n' "$diff_output" >&2
        fail "$label: dereferenced nvim tree differs from canonical source"
    fi
    pass "$label: dereferenced nvim tree matches canonical source"
}

assert_manifest_canonical() {
    local label="$1" entry_type="$2" rel_path="$3" repo_rel="$4"

    case "$entry_type" in
        config-file)
            assert_config_file_canonical "$label" "$rel_path" "$repo_rel"
            ;;
        nvim)
            assert_nvim_canonical "$label" "$rel_path" "$repo_rel"
            ;;
        *)
            fail "$label: unknown manifest type $entry_type"
            ;;
    esac
}

assert_single_source_pair() {
    local label="$1" repo_rel="$2" home_rel="$3" repo_path home_path repo_sha home_sha

    repo_path="$REPO_ROOT/$repo_rel"
    home_path="$REPO_ROOT/$home_rel"
    [[ -f "$repo_path" ]] || fail "$label: missing canonical repo source $repo_path"
    [[ -f "$home_path" ]] || fail "$label: missing chezmoi source copy $home_path"

    repo_sha="$(sha "$repo_path")"
    home_sha="$(sha "$home_path")"
    [[ "$repo_sha" == "$home_sha" ]] || \
        fail "$label: single-source SHA mismatch repo=$repo_sha home=$home_sha"
    pass "$label: single-source SHA matches"
}

assert_single_sources() {
    local label entry_type darwin_rel linux_rel repo_rel home_rel

    while IFS='|' read -r label entry_type darwin_rel linux_rel repo_rel home_rel; do
        [[ "$entry_type" != "nvim" ]] || continue
        [[ -n "$home_rel" ]] || continue
        assert_single_source_pair "$label" "$repo_rel" "$home_rel"
    done < <(manifest_entries)

    while IFS='|' read -r label repo_rel home_rel; do
        assert_single_source_pair "$label" "$repo_rel" "$home_rel"
    done < <(single_source_entries)
}

assert_manifest_canonical_for_host() {
    local label entry_type darwin_rel linux_rel repo_rel home_rel rel_path

    while IFS='|' read -r label entry_type darwin_rel linux_rel repo_rel home_rel; do
        rel_path="$(target_rel_for_host "$darwin_rel" "$linux_rel")"
        if [[ -z "$rel_path" ]]; then
            pass "$label: skipped on $target_os"
            continue
        fi
        assert_manifest_canonical "$label" "$entry_type" "$rel_path" "$repo_rel"
    done < <(manifest_entries)
}

assert_absent_path() {
    local label="$1" rel_path="$2" path

    path="$APPLY_HOME/$rel_path"
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

APPLY_HOME="$(mktemp -d)"
DOCTOR_SRC="$(mktemp -d)"
export XDG_DATA_HOME="$APPLY_HOME/xdg-data"
cleanup() {
    rm -rf "$APPLY_HOME"
    rm -rf "$DOCTOR_SRC"
}
trap cleanup EXIT

pass "canonical-only mode is default; comparing chezmoi-applied output to repo sources"

env HOME="$APPLY_HOME" chezmoi --source "$SRC" init
env HOME="$APPLY_HOME" chezmoi --source "$SRC" --no-tty --force apply
pass "chezmoi apply completed"

assert_manifest_canonical_for_host

assert_single_sources
assert_wrong_os_absent

second_apply_output=""
if ! second_apply_output="$(env HOME="$APPLY_HOME" chezmoi --source "$SRC" apply 2>&1)"; then
    printf '%s\n' "$second_apply_output" >&2
    fail "second chezmoi apply exited nonzero"
fi
if [[ -n "${second_apply_output//[[:space:]]/}" ]]; then
    echo "Second chezmoi apply output:" >&2
    printf '%s\n' "$second_apply_output" >&2
    fail "second chezmoi apply produced output"
fi
pass "second chezmoi apply is idempotent"

env HOME="$APPLY_HOME" chezmoi --source "$SRC" verify || \
    fail "chezmoi verify failed"
pass "chezmoi verify clean"

doctor_output=""
doctor_rc=0
cp -R "$SRC/." "$DOCTOR_SRC/"
doctor_output="$(env HOME="$APPLY_HOME" chezmoi --source "$DOCTOR_SRC" --no-tty doctor --no-network 2>&1)" || doctor_rc=$?
if grep -Eq '^[[:space:]]*error([[:space:]:]|$)' <<<"$doctor_output"; then
    echo "chezmoi doctor output:" >&2
    printf '%s\n' "$doctor_output" >&2
    fail "chezmoi doctor reported error-level results"
fi
if [[ "$doctor_rc" -ne 0 ]]; then
    echo "chezmoi doctor output:" >&2
    printf '%s\n' "$doctor_output" >&2
    fail "chezmoi doctor exited $doctor_rc without a leading error row"
fi
pass "chezmoi doctor has no error-level results"
