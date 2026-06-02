#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/install-nvim-linux-fail-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export NVIM_TEST_ROOT="$TMP_ROOT"
export TMPDIR="$TMP_ROOT/tmp"
PATH="$TMP_ROOT/bin:$PATH"

cat > "$TMP_ROOT/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
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
            shift
            ;;
    esac
done
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
    return 1
}

YES_ALL=1
DRY_RUN=0
if output="$(install_nvim_linux 2>&1)"; then
    echo "FAIL: install_nvim_linux returned success after checksum failure" >&2
    echo "$output" >&2
    exit 1
fi

[[ "$output" == *"FAIL: checksum mismatch for nvim-linux-x86_64.tar.gz"* ]]
grep -F "31cf85945cb600d96cdf69f88bc68bec814acbff50863c5546adef3a1bcef260" "$TMP_ROOT/sha.log" >/dev/null
if [[ -e "$TMP_ROOT/tar.log" ]]; then
    echo "FAIL: extraction ran after checksum failure" >&2
    exit 1
fi
if [[ -e "$TMP_ROOT/sudo.log" ]] && grep -F "/usr/local/bin/nvim" "$TMP_ROOT/sudo.log" >/dev/null; then
    echo "FAIL: nvim symlink was attempted after checksum failure" >&2
    exit 1
fi

echo "OK"
