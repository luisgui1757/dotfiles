#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/install-nvim-linux-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export NVIM_TEST_ROOT="$TMP_ROOT"
export TMPDIR="$TMP_ROOT/tmp"
export DOTFILES_PROVENANCE_DIR="$TMP_ROOT/provenance"
PATH="$TMP_ROOT/bin:$PATH"

cat > "$TMP_ROOT/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
url=""
out=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -o)
            out="$2"
            shift 2
            ;;
        -*)
            shift
            ;;
        *)
            url="$1"
            shift
            ;;
    esac
done
printf '%s\n' "$url" > "$NVIM_TEST_ROOT/curl.log"
printf 'fake nvim archive\n' > "$out"
EOF

cat > "$TMP_ROOT/bin/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "$NVIM_TEST_ROOT/tar.log"
EOF

chmod +x "$TMP_ROOT/bin/curl" "$TMP_ROOT/bin/tar"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

have() {
    if [[ "$1" == "nvim" ]]; then return 1; fi
    command -v "$1" >/dev/null 2>&1
}

native_linux_pm() {
    printf '%s\n' "apt"
}

maybe_sudo() {
    printf '%s\n' "$*" >> "$TMP_ROOT/sudo.log"
    case "$1" in
        tar) "$@" ;;
        rm|ln) return 0 ;;
        *) "$@" ;;
    esac
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 0
}

sha256_file() {
    case "$1" in
        /opt/nvim-linux-x86_64/bin/nvim)
            printf '%s\n' "fake-installed-nvim-binary-sha256"
            ;;
        *)
            command shasum -a 256 "$1" | awk '{print $1}'
            ;;
    esac
}

YES_ALL=1
DRY_RUN=0
output="$(install_nvim_linux)"

[[ "$output" == *"installed nvim"* ]]
grep -F "https://github.com/neovim/neovim/releases/download/v0.12.4/nvim-linux-x86_64.tar.gz" "$TMP_ROOT/curl.log" >/dev/null
grep -F "012bf3fcac5ade43914df3f174668bf64d05e049a4f032a388c027b1ebd78628" "$TMP_ROOT/sha.log" >/dev/null
grep -F -- "-xzf" "$TMP_ROOT/tar.log" >/dev/null
grep -F -- "-C /opt" "$TMP_ROOT/tar.log" >/dev/null
grep -F "/opt/nvim-linux-x86_64" "$TMP_ROOT/sudo.log" >/dev/null
grep -F "/usr/local/bin/nvim" "$TMP_ROOT/sudo.log" >/dev/null
grep -F "tool=nvim" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null
grep -F "schema=2" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null
grep -F "version=$NVIM_LINUX_VERSION" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null
grep -F "sha256=$NVIM_LINUX_X86_64_SHA256" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null
grep -F "binary_sha256=" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null
grep -F "command_path=/usr/local/bin/nvim" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null
grep -F "binary_path=/opt/nvim-linux-x86_64/bin/nvim" "$DOTFILES_PROVENANCE_DIR/nvim.env" >/dev/null

echo "OK"
