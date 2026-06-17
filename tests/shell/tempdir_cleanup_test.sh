#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/tempdir-cleanup-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

awk '
    /mktemp -d/ {
        line = $0
        if ((getline nextline) <= 0 || nextline !~ /trap .*rm -rf "\$tmp".*RETURN/) {
            print "missing cleanup trap after: " line
            failed = 1
        }
    }
    END { exit failed }
' "$REPO_ROOT/install-deps.sh" || fail "not every mktemp -d site installs a RETURN cleanup trap"

assert_tmp_empty() {
    local dir="$1" leaked
    leaked="$(find "$dir" -mindepth 1 -maxdepth 1 -type d -print -quit)"
    [[ -z "$leaked" ]] || fail "temporary directory leaked: $leaked"
}

font_root="$TMP_ROOT/font"
mkdir -p "$font_root/bin" "$font_root/tmp" "$font_root/home"
cat > "$font_root/bin/fc-list" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
cat > "$font_root/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
out=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -o)
            out="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
printf 'fake font archive\n' > "$out"
EOF
cat > "$font_root/bin/unzip" <<'EOF'
#!/usr/bin/env bash
exit 42
EOF
chmod +x "$font_root/bin/fc-list" "$font_root/bin/curl" "$font_root/bin/unzip"

if REPO_ROOT="$REPO_ROOT" TEST_ROOT="$font_root" bash <<'BASH' >/dev/null 2>&1
set -euo pipefail
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
export TMPDIR="$TEST_ROOT/tmp"
export HOME="$TEST_ROOT/home"
export XDG_DATA_HOME="$TEST_ROOT/data"
PATH="$TEST_ROOT/bin:/usr/bin:/bin"
verify_sha256() { return 0; }
YES_ALL=1
DRY_RUN=0
PM=apt
install_nerd_font
BASH
then
    fail "install_nerd_font succeeded despite failing unzip"
fi
assert_tmp_empty "$font_root/tmp"

lazygit_root="$TMP_ROOT/lazygit"
mkdir -p "$lazygit_root/bin" "$lazygit_root/tmp" "$lazygit_root/home"
cat > "$lazygit_root/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
out=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -o)
            out="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
printf 'fake lazygit archive\n' > "$out"
EOF
cat > "$lazygit_root/bin/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
dest=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -C)
            dest="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
printf 'fake lazygit binary\n' > "$dest/lazygit"
EOF
chmod +x "$lazygit_root/bin/curl" "$lazygit_root/bin/tar"

if REPO_ROOT="$REPO_ROOT" TEST_ROOT="$lazygit_root" bash <<'BASH' >/dev/null 2>&1
set -euo pipefail
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
export TMPDIR="$TEST_ROOT/tmp"
export HOME="$TEST_ROOT/home"
PATH="$TEST_ROOT/bin:/usr/bin:/bin"
uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}
id() {
    if [[ "${1:-}" == "-u" ]]; then
        printf '%s\n' 1000
    else
        command id "$@"
    fi
}
have() {
    case "$1" in
        lazygit|sudo) return 1 ;;
        *) command -v "$1" >/dev/null 2>&1 ;;
    esac
}
native_linux_pm() { printf '%s\n' "apt"; }
verify_sha256() { return 0; }
cp() { return 42; }
YES_ALL=1
DRY_RUN=0
PM=apt
install_lazygit_linux
BASH
then
    fail "install_lazygit_linux succeeded despite failing fallback copy"
fi
assert_tmp_empty "$lazygit_root/tmp"

echo "OK"
