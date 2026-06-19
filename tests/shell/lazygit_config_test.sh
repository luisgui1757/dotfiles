#!/usr/bin/env bash
# Regression guard: lazygit configs must be parseable by lazygit v0.58.x.
# Two specific failure shapes we have hit and want pinned:
#   1) array-form values for fields lazygit types as Go string ("cannot
#      unmarshal !!seq into string").
#   2) keybinding values like <alt+j> or <a-j> which are NOT in
#      pkg/config/keynames.go LabelByKey; lazygit rejects them at startup
#      with "Unrecognized key".
#   3) losing the printable move-commit binding: moveDownCommit must be J and
#      moveUpCommit must be K.
#   4) losing the native-Windows Ctrl-G return binding used inside psmux.
#
# We strip YAML comments (everything from # to end of line) before grepping
# so the warnings in the config header do not trip the test.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
COMMON_CFG="$REPO_ROOT/lazygit/config.yml"
WINDOWS_CFG="$REPO_ROOT/lazygit/config.windows.yml"

stripped_for() {
    sed -E 's/[[:space:]]*#.*$//' "$1"
}

fail=0
match() {
    local cfg=$1 pattern=$2
    stripped_for "$cfg" | grep -nE "$pattern" | grep -v '^[0-9]*:[[:space:]]*$' || true
}

value_for() {
    local cfg=$1 key=$2
    stripped_for "$cfg" |
        sed -nE "s/^[[:space:]]*$key:[[:space:]]*['\"]?([^'\"]+)['\"]?[[:space:]]*$/\1/p" |
        head -n 1
}

is_registered_multi_key() {
    case "$1" in
        '<disabled>' | '<f1>' | '<f2>' | '<f3>' | '<f4>' | '<f5>' | '<f6>' | \
            '<f7>' | '<f8>' | '<f9>' | '<f10>' | '<f11>' | '<f12>' | \
            '<insert>' | '<delete>' | '<home>' | '<end>' | '<pgup>' | \
            '<pgdown>' | '<up>' | '<s-up>' | '<down>' | '<s-down>' | \
            '<left>' | '<right>' | '<tab>' | '<backtab>' | '<enter>' | \
            '<a-enter>' | '<esc>' | '<backspace>' | '<c-space>' | '<c-/>' | \
            '<space>' | '<c-a>' | '<c-b>' | '<c-c>' | '<c-d>' | '<c-e>' | \
            '<c-f>' | '<c-g>' | '<c-j>' | '<c-k>' | '<c-l>' | '<c-n>' | \
            '<c-o>' | '<c-p>' | '<c-q>' | '<c-r>' | '<c-s>' | '<c-t>' | \
            '<c-u>' | '<c-v>' | '<c-w>' | '<c-x>' | '<c-y>' | '<c-z>' | \
            '<c-4>' | '<c-5>' | '<c-6>' | '<c-8>' | \
            'mouse wheel up' | 'mouse wheel down')
            return 0
            ;;
    esac
    return 1
}

assert_lazygit_key_shape() {
    local name=$1 value=$2
    if [[ -z "$value" ]]; then
        echo "FAIL: missing lazygit keybinding value for $name"
        fail=1
        return
    fi
    if [[ ${#value} -eq 1 ]]; then
        return
    fi
    if is_registered_multi_key "$value"; then
        return
    fi
    echo "FAIL: $name = '$value' is neither a single character nor a registered lazygit multi-char key"
    fail=1
}

assert_config_shape() {
    local cfg=$1 label=$2

    # 1) No array form on string-typed fields.
    hits=$(match "$cfg" '^[[:space:]]+(moveDownCommit|moveUpCommit|return|.*Commit):[[:space:]]*\[')
    if [[ -n "$hits" ]]; then
        echo "FAIL: array form found on a lazygit string field in $label"
        echo "$hits"
        fail=1
    fi

    # 2) Refuse the long modifier forms.
    hits=$(match "$cfg" '<(alt|ctrl|shift|meta)\+')
    if [[ -n "$hits" ]]; then
        echo "FAIL: <modifier+key> long form is rejected by lazygit v0.58 in $label"
        echo "$hits"
        fail=1
    fi

    # 3) Refuse Alt+letter (only <a-enter> is registered).
    hits=$(match "$cfg" '<[aA]-[[:alpha:]]>' | grep -vi '<a-enter>' || true)
    if [[ -n "$hits" ]]; then
        echo "FAIL: <a-LETTER> in config; lazygit v0.58 only registers <a-enter> in $label"
        echo "$hits"
        fail=1
    fi

    keybinding_count=$(match "$cfg" '^keybinding:[[:space:]]*$' | wc -l | tr -d ' ')
    if [[ "$keybinding_count" != "1" ]]; then
        echo "FAIL: $label must contain exactly one top-level keybinding block (found $keybinding_count)"
        fail=1
    fi

    # 4) Positive shape check: move-commit bindings must be lazygit-registered.
    move_down=$(value_for "$cfg" moveDownCommit)
    move_up=$(value_for "$cfg" moveUpCommit)
    assert_lazygit_key_shape "$label moveDownCommit" "$move_down"
    assert_lazygit_key_shape "$label moveUpCommit" "$move_up"

    # 5) Positive value check: the move-commit bindings must stay printable J/K.
    if [[ "$move_down" != 'J' ]]; then
        echo "FAIL: $label moveDownCommit = '$move_down' (want 'J')"
        fail=1
    fi
    if [[ "$move_up" != 'K' ]]; then
        echo "FAIL: $label moveUpCommit = '$move_up' (want 'K')"
        fail=1
    fi
}

assert_config_shape "$COMMON_CFG" "lazygit/config.yml"
assert_config_shape "$WINDOWS_CFG" "lazygit/config.windows.yml"

windows_return=$(value_for "$WINDOWS_CFG" return)
assert_lazygit_key_shape "lazygit/config.windows.yml return" "$windows_return"
if [[ "$windows_return" != '<c-g>' ]]; then
    echo "FAIL: lazygit/config.windows.yml return = '$windows_return' (want '<c-g>')"
    fail=1
fi

common_return=$(value_for "$COMMON_CFG" return)
if [[ -n "$common_return" ]]; then
    echo "FAIL: lazygit/config.yml must not override universal.return globally"
    fail=1
fi

[[ "$fail" -eq 0 ]] && echo "OK" || exit 1
