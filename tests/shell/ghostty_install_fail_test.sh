#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

TMP_ROOT="$REPO_ROOT/tests/.cache/ghostty-install-fail-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/tmp"
trap 'rm -rf "$TMP_ROOT"' EXIT

export GHOSTTY_FAIL_TEST_ROOT="$TMP_ROOT"
export TMPDIR="$TMP_ROOT/tmp"

curl() {
    local out=""
    if [[ "${GHOSTTY_CURL_MODE:-ok}" == "fail" ]]; then
        return 22
    fi
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
    cat > "$out" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'ran\n' > "$GHOSTTY_FAIL_TEST_ROOT/installer-ran"
EOF
}

verify_sha256() {
    printf '%s %s\n' "$1" "$2" > "$TMP_ROOT/sha.log"
    return 1
}

if output="$(run_ghostty_ubuntu_installer "https://example.invalid/install.sh" 2>&1)"; then
    echo "FAIL: run_ghostty_ubuntu_installer returned success after checksum failure" >&2
    echo "$output" >&2
    exit 1
fi

[[ "$output" == *"FAIL: checksum mismatch for ghostty install.sh"* ]]
grep -F "$GHOSTTY_UBUNTU_INSTALL_SHA256" "$TMP_ROOT/sha.log" >/dev/null
if [[ -e "$TMP_ROOT/installer-ran" ]]; then
    echo "FAIL: ghostty installer ran after checksum failure" >&2
    exit 1
fi

GHOSTTY_CURL_MODE=fail
if output="$(run_ghostty_ubuntu_installer "https://example.invalid/install.sh" 2>&1)"; then
    echo "FAIL: run_ghostty_ubuntu_installer returned success after download failure" >&2
    echo "$output" >&2
    exit 1
fi

[[ "$output" == *"FAIL: could not download ghostty installer"* ]]
if [[ -e "$TMP_ROOT/installer-ran" ]]; then
    echo "FAIL: ghostty installer ran after download failure" >&2
    exit 1
fi

echo "OK"
