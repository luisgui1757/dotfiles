#!/usr/bin/env bash
# Backup recovery is keyed by the validated filename timestamp/collision suffix,
# never mutable filesystem mtime. Malformed candidates fail before target removal.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$REPO_ROOT/tests/.cache/uninstall-backup-order-test"
rm -rf "$WORK"
mkdir -p "$WORK/Home With Spaces"
HOME="$WORK/Home With Spaces"
export HOME
trap 'rm -rf "$WORK"' EXIT

DOTFILES_UNINSTALL_SOURCE_ONLY=1 source "$REPO_ROOT/uninstall.sh"
trap 'rm -f "$DIR_CANDIDATES_FILE"; rm -rf "$WORK"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

file_target="$HOME/config file"
printf old > "$file_target.bak.20260101-010101"
printf newer > "$file_target.bak.20260202-020202"
printf collision > "$file_target.bak.20260202-020202.2"
touch -t 203001010101 "$file_target.bak.20260101-010101"
touch -t 202001010101 "$file_target.bak.20260202-020202.2"
[[ "$(newest_backup "$file_target")" == "$file_target.bak.20260202-020202.2" ]] \
    || fail "filename order lost to opposing mtime order"

dir_target="$HOME/config-dir"
mkdir -p "$dir_target.bak.20260303-030303" "$dir_target.bak.20260303-030303.1"
touch -t 203001010101 "$dir_target.bak.20260303-030303"
[[ "$(newest_backup "$dir_target")" == "$dir_target.bak.20260303-030303.1" ]] \
    || fail "directory collision suffix was not selected"

malformed_target="$HOME/malformed"
printf valid > "$malformed_target.bak.20260101-010101"
printf bad > "$malformed_target.bak.latest"
if newest_backup "$malformed_target" >/dev/null 2>&1; then
    fail "malformed backup name was accepted"
fi
invalid_date_target="$HOME/invalid-date"
printf bad > "$invalid_date_target.bak.20260230-010101"
if newest_backup "$invalid_date_target" >/dev/null 2>&1; then
    fail "impossible backup timestamp was accepted"
fi

restore_target="$HOME/managed link"
ln -s "$REPO_ROOT/README.md" "$restore_target"
printf older > "$restore_target.bak.20260101-010101"
printf newest > "$restore_target.bak.20260404-040404.1"
touch -t 203001010101 "$restore_target.bak.20260101-010101"
DRY_RUN=0
RESTORE_BACKUPS=1
remove_managed_target "$restore_target" >/dev/null
[[ -f "$restore_target" && ! -L "$restore_target" ]] || fail "file backup was not restored"
[[ "$(cat "$restore_target")" == newest ]] || fail "wrong file backup restored"

restore_dir="$HOME/managed-dir"
ln -s "$REPO_ROOT/nvim" "$restore_dir"
mkdir -p "$restore_dir.bak.20260505-050505" "$restore_dir.bak.20260505-050505.1"
printf selected > "$restore_dir.bak.20260505-050505.1/marker"
touch -t 203001010101 "$restore_dir.bak.20260505-050505"
remove_managed_target "$restore_dir" >/dev/null
[[ -d "$restore_dir" && ! -L "$restore_dir" ]] || fail "directory backup was not restored"
[[ "$(cat "$restore_dir/marker")" == selected ]] || fail "wrong directory backup restored"

safe_target="$HOME/safe-on-malformed"
ln -s "$REPO_ROOT/README.md" "$safe_target"
printf valid > "$safe_target.bak.20260101-010101"
printf malformed > "$safe_target.bak.01"
if remove_managed_target "$safe_target" >/dev/null 2>&1; then
    fail "malformed candidates did not fail removal"
fi
[[ -L "$safe_target" ]] || fail "target was removed before backup candidates were validated"

echo "OK"
