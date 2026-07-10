#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-temp-cleanup-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/tmp" "$TMP_ROOT/home"
trap 'rm -rf "$TMP_ROOT"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

awk '
    /expected_file="\$\(mktemp\)"/ {
        line = $0
        if ((getline nextline) <= 0 || nextline !~ /trap .*rm -f "\$expected_file".*RETURN/) {
            print "missing expected-file cleanup trap after: " line
            failed = 1
        }
    }
    /CHEZMOI_DRY_CONFIG="\$\(mktemp\)"/ {
        line = $0
        if ((getline nextline) <= 0 || nextline !~ /trap .*cleanup_chezmoi_dry_config.*EXIT/) {
            print "missing dry config cleanup trap after: " line
            failed = 1
        }
    }
    END { exit failed }
' "$REPO_ROOT/setup.sh" || fail "setup.sh temp files are missing cleanup traps"

cat > "$TMP_ROOT/bin/chezmoi" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
args="$*"
case "$args" in
    *execute-template*)
        cat >/dev/null
        printf '%s\n' '[data]'
        ;;
    *managed*)
        exit 0
        ;;
    *"--dry-run"*"apply"*)
        exit 42
        ;;
    *)
        exit 0
        ;;
esac
EOF
chmod +x "$TMP_ROOT/bin/chezmoi"
cat > "$TMP_ROOT/bin/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -s) echo Linux ;;
    -m) echo x86_64 ;;
    *) command /usr/bin/uname "$@" ;;
esac
EOF
cat > "$TMP_ROOT/bin/id" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -u) echo 1000 ;;
    -un) echo tester ;;
    *) exit 1 ;;
esac
EOF
cat > "$TMP_ROOT/bin/getent" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == passwd && "\${2:-}" == tester ]] || exit 1
printf '%s\n' 'tester:x:1000:1000:Test User:$TMP_ROOT/home:/bin/zsh'
EOF
chmod +x "$TMP_ROOT/bin/uname" "$TMP_ROOT/bin/id" "$TMP_ROOT/bin/getent"

set +e
HOME="$TMP_ROOT/home" TMPDIR="$TMP_ROOT/tmp" PATH="$TMP_ROOT/bin:/usr/bin:/bin" \
    bash "$REPO_ROOT/setup.sh" --dry-run --skip-deps --skip-nvim \
    >"$TMP_ROOT/setup.out" 2>"$TMP_ROOT/setup.err"
rc=$?
set -e

[[ "$rc" -ne 0 ]] || fail "setup.sh dry-run unexpectedly succeeded"
leaked="$(find "$TMP_ROOT/tmp" -mindepth 1 -maxdepth 1 -print -quit)"
[[ -z "$leaked" ]] || fail "setup.sh dry-run leaked temp file: $leaked"

echo "OK"
