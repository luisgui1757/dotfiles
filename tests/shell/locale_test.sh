#!/usr/bin/env bash
set -euo pipefail

if ! command -v zsh >/dev/null 2>&1; then
    echo "skipped: zsh not installed"
    exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"
TMP_ROOT="$REPO_ROOT/tests/.cache/locale-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT/bin"
trap 'active_shell_cleanup; rm -rf "$TMP_ROOT"' EXIT

cat > "$TMP_ROOT/bin/locale" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-a" ]]; then
    cat "$LOCALE_FIXTURE"
    exit 0
fi
exit 2
EOF
chmod +x "$TMP_ROOT/bin/locale"

run_case() {
    local name="$1" locales="$2" expected="$3"
    local home="$TMP_ROOT/home-$name"
    local fixture="$TMP_ROOT/locales-$name.txt"
    mkdir -p "$home"
    printf '%s\n' "$locales" > "$fixture"

    local raw value
    raw=$(
        HOME="$home" \
        PATH="$TMP_ROOT/bin:$PATH" \
        LOCALE_FIXTURE="$fixture" \
        LANG=C LC_ALL=C LANGUAGE=C LC_MESSAGES=C \
        zsh -d -f -i -c "source \"$ACTIVE_ZSH_CORE\" || exit 1; print -r -- '@@LOCALE@@'\$LANG'|'\$LC_ALL'|'\$LANGUAGE'|'\$LC_MESSAGES'@@END@@'" 2>&1
    )
    value=$(printf '%s\n' "$raw" | grep -oE '@@LOCALE@@.*@@END@@' | sed -e 's/^@@LOCALE@@//' -e 's/@@END@@$//')
    if [[ "$value" != "$expected" ]]; then
        echo "FAIL: $name expected $expected, got $value"
        printf '%s\n' "$raw"
        exit 1
    fi
}

run_case "en-us" $'C\nen_US.UTF-8\nC.UTF-8' "en_US.UTF-8|en_US.UTF-8|en_US.UTF-8|en_US.UTF-8"
run_case "c-utf8" $'C\nC.UTF-8' "C.UTF-8|C.UTF-8|C.UTF-8|C.UTF-8"
run_case "unchanged" $'C\nPOSIX' "C|C|C|C"

echo "OK"
