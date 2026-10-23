#!/usr/bin/env bash
# The hosted Ubuntu image deliberately makes /usr/share recursively writable.
# Restore only these reviewed test boundaries; never run on a personal host.
set -euo pipefail

if [[ "${GITHUB_ACTIONS:-}" != true || "${RUNNER_ENVIRONMENT:-}" != github-hosted || "$(uname -s)" != Linux ]]; then
    echo 'FAIL: Linux host preparation requires a disposable GitHub-hosted runner' >&2
    exit 1
fi

paths=(/usr/share)
case "${1:-}" in
    desktop) ;;
    shell) paths+=(/usr/share/zsh /usr/share/zsh/vendor-completions) ;;
    userns) ;;
    *) echo 'Usage: prepare-disposable-linux-host.sh desktop|shell|userns' >&2; exit 1 ;;
esac
[[ "$#" -eq 1 ]] || { echo 'FAIL: unexpected host preparation arguments' >&2; exit 1; }

# Ubuntu restricts unprivileged user namespaces through AppArmor by default;
# the hosted image relaxes that. Desktop acceptance restores the stock policy
# so applications run as they do on an ordinary Ubuntu desktop.
if [[ "$1" == userns ]]; then
    setting=/proc/sys/kernel/apparmor_restrict_unprivileged_userns
    test -r "$setting"
    test "$(cat /sys/module/apparmor/parameters/enabled)" = Y
    echo "Test host before: kernel.apparmor_restrict_unprivileged_userns=$(cat "$setting")"
    sudo -n sysctl -q -w kernel.apparmor_restrict_unprivileged_userns=1
    test "$(cat "$setting")" = 1
    echo "Test host after: kernel.apparmor_restrict_unprivileged_userns=$(cat "$setting")"
    exit 0
fi

# Validate the complete fixed set before changing any mode. Resolving the full
# path rejects redirected ancestors as well as a link at the final component.
for path in "${paths[@]}"; do
    test -d "$path"
    test "$(readlink -e "$path")" = "$path"
    test "$(stat -c %u:%g "$path")" = 0:0
    stat -c 'Test host before: %n owner=%u group=%g mode=%a' "$path"
done
for path in "${paths[@]}"; do
    sudo -n chmod 0755 "$path"
    test "$(stat -c %u:%g:%a "$path")" = 0:0:755
    stat -c 'Test host after: %n owner=%u group=%g mode=%a' "$path"
done

if [[ "$1" == shell ]]; then
    if ! audit=$(zsh -d -f -c 'autoload -Uz compaudit; compaudit' 2>&1); then
        printf '%s\n' "$audit" >&2
        exit 1
    fi
    [[ -z "$audit" ]] || { printf 'FAIL: unexpected completion audit output: %s\n' "$audit" >&2; exit 1; }
fi
