#!/usr/bin/env bash
# Integrate the pinned plugin payloads without another package manager or
# continuum's unrelated LaunchAgent/systemd service-management hooks.
set -euo pipefail

if [[ $# -ne 4 ]]; then
    echo 'Expected the four verified tmux plugin directories.' >&2
    exit 2
fi

"$1/sensible.tmux"
"$2/yank.tmux"
"$3/resurrect.tmux"

# Upstream's default bindings put an unquoted path into run-shell. Keep the
# configured keys, but also support private install locations containing spaces.
quote_shell() {
    local quoted=${1//\'/\'\"\'\"\'}
    printf "'%s'" "$quoted"
}
for action in save restore; do
    if [[ $action == save ]]; then default_key=C-s; else default_key=C-r; fi
    configured_keys="$(tmux show-option -gqv "@resurrect-$action")"
    read -r -a keys <<< "${configured_keys:-$default_key}"
    for key in "${keys[@]}"; do
        tmux bind-key "$key" run-shell "$(quote_shell "$3/scripts/$action.sh")"
    done
done

# These are the same pinned save/restore helpers used by continuum's entrypoint.
# The loader owns only their activation. In particular it never invokes
# handle_tmux_automatic_start.sh, which can remove unrelated startup settings.
CURRENT_DIR="$4"
# shellcheck disable=SC1091 # Verified archive paths are supplied by the Go recipe.
source "$CURRENT_DIR/scripts/helpers.sh"
# shellcheck disable=SC1091
source "$CURRENT_DIR/scripts/variables.sh"
# shellcheck disable=SC1091
source "$CURRENT_DIR/scripts/shared.sh"
: "${auto_restore_max_delay_option:?Missing pinned restore-delay option}"
: "${auto_restore_max_delay_default:?Missing pinned restore-delay default}"
: "${last_auto_save_option:?Missing pinned save-timestamp option}"

started="$(tmux display-message -p '#{start_time}')"
delay="$(get_tmux_option "$auto_restore_max_delay_option" "$auto_restore_max_delay_default")"
if [[ ! $started =~ ^[0-9]+$ || ! $delay =~ ^[0-9]+$ ]]; then
    echo 'Invalid tmux startup time or automatic-restore delay.' >&2
    exit 1
fi
fresh=false
if (( started > $(date +%s) - delay )); then fresh=true; fi

if { $fresh && ! another_tmux_server_running_on_startup; } ||
    { ! $fresh && [[ $(number_tmux_processes_except_current_server) -le $(number_current_server_client_processes) ]]; }; then
    if [[ -z $(get_tmux_option "$last_auto_save_option" '') ]]; then
        set_last_save_timestamp
    fi
    save_hook="#($(quote_shell "$CURRENT_DIR/scripts/continuum_save.sh"))"
    status="$(tmux show-option -gqv status-right)"
    if [[ $status != *"$save_hook"* ]]; then
        tmux set-option -gq status-right "$save_hook$status"
    fi
fi

if $fresh; then
    "$CURRENT_DIR/scripts/continuum_restore.sh" &
fi
