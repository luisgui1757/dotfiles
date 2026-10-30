#!/usr/bin/env bash
# The managed spf function: Shift+Q in superfile changes the shell to the last
# folder. superfile saves that folder as shell code, so the function must read
# it as a literal path and never run it. Checked in zsh and in bash.
# shellcheck disable=SC2016
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# shellcheck source=tests/shell/active_fixture.sh
source "$REPO_ROOT/tests/shell/active_fixture.sh"

fail() {
    echo "FAIL: $*"
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'active_shell_cleanup; rm -rf "$TMP_ROOT"' EXIT
TMP_ROOT="$(cd "$TMP_ROOT" && pwd -P)"

# A stand-in for spf: it reports its last-folder file like the real
# `spf path-list --lastdir-file`, and "quits" by writing SPF_TEST_CONTENT there.
mkdir -p "$TMP_ROOT/bin" "$TMP_ROOT/start"
cat > "$TMP_ROOT/bin/spf" <<'SH'
#!/bin/sh
if [ "$1" = path-list ] && [ "$2" = --lastdir-file ]; then
    printf '%s\n' "$SPF_TEST_LAST"
    exit 0
fi
if [ -n "${SPF_TEST_CONTENT-}" ]; then printf '%s' "$SPF_TEST_CONTENT" > "$SPF_TEST_LAST"; fi
exit "${SPF_TEST_STATUS:-0}"
SH
chmod +x "$TMP_ROOT/bin/spf"

target="$TMP_ROOT/it's a \$(touch $TMP_ROOT/pwned) folder"
mkdir -p "$target"
# Variables keep the replacement literal. Bash 3.2 (macOS) does not remove the
# quotes in a replacement inside double quotes, and later versions do.
quote="'" escaped_quote="'\\''"
escaped="cd '${target//$quote/$escaped_quote}'"

run() { # run <shell> <content> <status>
    local shell="$1" content="$2" status="$3"
    rm -f "$TMP_ROOT/lastdir"
    local probe='source "$1" >/dev/null 2>&1 || exit 90
cd "$2" || exit 91
spf --any-argument
printf "%s\n%s\n" "$?" "$PWD"'
    local -a command=(bash --noprofile --norc -i -c)
    if [[ "$shell" == zsh ]]; then command=(zsh -d -f -i -c); fi
    HOME="$DOTFILES_SHELL_FIXTURES/posix" PATH="$TMP_ROOT/bin:/usr/bin:/bin" \
        SPF_TEST_LAST="$TMP_ROOT/lastdir" SPF_TEST_CONTENT="$content" SPF_TEST_STATUS="$status" \
        "${command[@]}" "$probe" "$shell" "$ACTIVE_ZSH" "$TMP_ROOT/start" 2>/dev/null
}

for shell in zsh bash; do
    if ! command -v "$shell" >/dev/null 2>&1; then
        echo "skipped: $shell not installed"
        continue
    fi

    output="$(run "$shell" "$escaped" 3)"
    [[ "$output" == "3"$'\n'"$target" ]] \
        || fail "$shell: Shift+Q did not change to the saved folder or lost the exit status: $output"
    [[ ! -e "$TMP_ROOT/lastdir" ]] || fail "$shell: the saved folder file stayed"

    output="$(run "$shell" "touch '$TMP_ROOT/pwned'" 0)"
    [[ "$output" == "0"$'\n'"$TMP_ROOT/start" ]] || fail "$shell: changed folder for content that is not a cd line: $output"
    [[ ! -e "$TMP_ROOT/lastdir" ]] || fail "$shell: the rejected folder file stayed"

    output="$(run "$shell" "" 0)"
    [[ "$output" == "0"$'\n'"$TMP_ROOT/start" ]] || fail "$shell: a plain quit changed the folder: $output"

    [[ ! -e "$TMP_ROOT/pwned" ]] || fail "$shell: the saved folder file was run as code"
done
echo "OK"
