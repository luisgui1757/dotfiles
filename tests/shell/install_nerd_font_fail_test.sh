#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/install-nerd-font-fail-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export FONT_TEST_ROOT="$TMP_ROOT"
export TMPDIR="$TMP_ROOT/tmp"
export XDG_DATA_HOME="$TMP_ROOT/data"
PATH="$TMP_ROOT/bin:$PATH"

cat > "$TMP_ROOT/bin/fc-list" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

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
printf 'fake font archive\n' > "$out"
EOF

cat > "$TMP_ROOT/bin/unzip" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "$FONT_TEST_ROOT/unzip.log"
EOF

chmod +x "$TMP_ROOT/bin/fc-list" "$TMP_ROOT/bin/curl" "$TMP_ROOT/bin/unzip"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 1
}

YES_ALL=1
DRY_RUN=0
PM=apt
output_file="$TMP_ROOT/output.log"
install_nerd_font >"$output_file" 2>&1
output="$(cat "$output_file")"

[[ "$output" == *"FAIL: checksum mismatch for Hack.zip"* ]]
[[ "$INSTALL_FAILURES_COUNT" -eq 1 ]]
grep -F "8ca33a60c791392d872b80d26c42f2bfa914a480f9eb2d7516d9f84373c36897" "$TMP_ROOT/sha.log" >/dev/null
if [[ -e "$TMP_ROOT/unzip.log" ]]; then
    echo "FAIL: font extraction ran after checksum failure" >&2
    exit 1
fi
if [[ -e "$XDG_DATA_HOME/fonts/HackNerdFont" ]]; then
    echo "FAIL: font directory was created after checksum failure" >&2
    exit 1
fi
set +e
summary="$( ( exit_if_install_failures ) 2>&1 )"
summary_rc=$?
set -e
if [[ "$summary_rc" -eq 0 ]]; then
    echo "FAIL: final install failure summary returned success after checksum failure" >&2
    echo "$summary" >&2
    exit 1
fi
[[ "$summary" == *"install-deps: 1 accepted install path(s) failed:"* ]]
[[ "$summary" == *"Hack Nerd Font via direct (Hack.zip) exit=sha256"* ]]

echo "OK"
