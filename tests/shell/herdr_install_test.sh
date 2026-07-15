#!/usr/bin/env bash
# Positive coverage for Herdr's native-Linux direct artifact path: install the
# pinned binary, write durable provenance, and let update mode prove ownership.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/herdr-install-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export TMPDIR="$TMP_ROOT/tmp"
export HOME="$TMP_ROOT/home"
PATH="/usr/bin:/bin"
export PATH
FAKE_HERDR="$TMP_ROOT/herdr.fixture"
cat > "$FAKE_HERDR" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "--version" ]]; then
    printf '%s\n' "herdr ${HERDR_VERSION#v}"
    exit 0
fi
exit 0
EOF
HERDR_LINUX_X86_64_SHA256="$(shasum -a 256 "$FAKE_HERDR" | awk '{print $1}')"
YES_ALL=1
DRY_RUN=0
PM=unknown

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        -m) printf '%s\n' "x86_64" ;;
        *) command uname "$@" ;;
    esac
}

have() { command -v "$1" >/dev/null 2>&1; }
require_downloader() { return 0; }
homebrew_bin() { return 1; }
native_linux_pm() { printf '%s\n' "unknown"; }

curl() {
    local out=""
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
    cp "$FAKE_HERDR" "$out"
}

install_herdr > "$TMP_ROOT/install.out"

dest="$HOME/.local/bin/herdr"
marker="$HOME/.local/share/dotfiles/provenance/herdr.env"
[[ -x "$dest" ]]
grep -F "tool=herdr" "$marker" >/dev/null
grep -F "version=$HERDR_VERSION" "$marker" >/dev/null
grep -F "source_url=https://github.com/ogulcancelik/herdr/releases/download/${HERDR_VERSION}/herdr-linux-x86_64" "$marker" >/dev/null
grep -F "command_path=$dest" "$marker" >/dev/null

PATH="$HOME/.local/bin:$PATH"
export PATH
INSTALL_DEPS_UPDATE_TOOLS="herdr"
update_catalog_tools > "$TMP_ROOT/update.out"
grep -E "current[[:space:]]+herdr[[:space:]].*owner=dotfiles-artifact" "$TMP_ROOT/update.out" >/dev/null
grep -F "version=$HERDR_VERSION" "$TMP_ROOT/update.out" >/dev/null

echo "OK"
