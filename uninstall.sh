#!/usr/bin/env bash
# uninstall.sh -- safely remove the chezmoi-managed config layer.
#
# This is intentionally NOT `chezmoi purge` or `chezmoi destroy`: this repo is
# the in-place source checkout, and purge-style commands would remove source
# state/config that the owner may still need.

set -euo pipefail

DRY_RUN=0
ALL=0
KEEP_EXTERNALS=0
RESTORE_BACKUPS=1
FORCE_EXTERNALS=0

usage() {
    sed -n '2,22p' "$0"
    cat <<'EOF'

Usage:
  ./uninstall.sh [--dry-run] [--all] [--keep-externals]
                 [--no-restore-backups] [--force-externals]

Flags:
  --dry-run              print the plan, touch nothing
  --all                  non-interactive; accept each removal category
  --keep-externals       keep zsh plugin git checkouts
  --no-restore-backups   do not restore <target>.bak.<timestamp> backups
  --force-externals      remove zsh plugin checkouts even with uncommitted
                         changes (default: keep a dirty/unverifiable checkout)
EOF
}

for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=1 ;;
        --all|-y) ALL=1 ;;
        --keep-externals) KEEP_EXTERNALS=1 ;;
        --no-restore-backups) RESTORE_BACKUPS=0 ;;
        --force-externals) FORCE_EXTERNALS=1 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Unknown arg: $arg" >&2; exit 2 ;;
    esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
SOURCE_DIR="$REPO_ROOT/home"

removed=0
restored=0
skipped=0
warnings=0
dirs_removed=0
externals_removed=0
DIR_CANDIDATES_FILE="$(mktemp)"
trap 'rm -f "$DIR_CANDIDATES_FILE"' EXIT

warn() {
    warnings=$((warnings + 1))
    echo "WARN: $*" >&2
}

have() {
    command -v "$1" >/dev/null 2>&1
}

realpath_or_self() {
    local path="$1"
    if have realpath; then
        realpath "$path" 2>/dev/null || printf '%s\n' "$path"
    elif readlink -f / >/dev/null 2>&1; then
        readlink -f "$path" 2>/dev/null || printf '%s\n' "$path"
    elif have python3; then
        python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$path"
    else
        printf '%s\n' "$path"
    fi
}

# Decide repo-ownership of a symlink from its LINK TEXT (readlink), not by
# realpath-resolving the link itself. realpath on a BROKEN symlink (its repo
# target was removed) fails and falls back to the link's own path in $HOME,
# wrongly classifying a repo-owned-but-broken link as "outside repo" and leaving
# it behind. Resolving the text (absolute, or relative to the link's directory)
# and normalizing it without requiring the target to exist fixes that.
link_points_into_repo() {
    local link="$1" dest="$2" abs repo_real
    [[ -n "$dest" ]] || return 1
    case "$dest" in
        /*) abs="$dest" ;;
        *)  abs="$(dirname "$link")/$dest" ;;
    esac
    if have python3; then
        abs="$(python3 -c 'import os,sys; print(os.path.normpath(sys.argv[1]))' "$abs" 2>/dev/null || printf '%s' "$abs")"
    fi
    repo_real="$(realpath_or_self "$REPO_ROOT")"
    case "$abs/" in
        "$repo_real"/*) return 0 ;;
        *) return 1 ;;
    esac
}

target_exists() {
    [[ -e "$1" || -L "$1" ]]
}

newest_backup() {
    local target="$1" newest="" candidate suffix timestamp collision
    local newest_timestamp="" newest_collision="0" malformed=0 ambiguous=0
    local LC_ALL=C
    for candidate in "$target".bak.*; do
        target_exists "$candidate" || continue
        suffix="${candidate#"$target.bak."}"
        if [[ ! "$suffix" =~ ^([0-9]{8}-[0-9]{6})(\.([1-9][0-9]*))?$ ]]; then
            echo "FAIL: malformed backup candidate for $target: $candidate" >&2
            malformed=1
            continue
        fi
        timestamp="${BASH_REMATCH[1]}"
        collision="${BASH_REMATCH[3]:-0}"
        if ! valid_backup_timestamp "$timestamp"; then
            echo "FAIL: malformed backup timestamp for $target: $candidate" >&2
            malformed=1
            continue
        fi
        if [[ -z "$newest" || "$timestamp" > "$newest_timestamp" ]] ||
            { [[ "$timestamp" == "$newest_timestamp" ]] && numeric_string_gt "$collision" "$newest_collision"; }; then
            newest="$candidate"
            newest_timestamp="$timestamp"
            newest_collision="$collision"
            ambiguous=0
        elif [[ "$timestamp" == "$newest_timestamp" && "$collision" == "$newest_collision" ]]; then
            ambiguous=1
        fi
    done
    [[ "$malformed" -eq 0 ]] || return 2
    if [[ "$ambiguous" -eq 1 ]]; then
        echo "FAIL: ambiguous backup candidates for $target at $newest_timestamp collision $newest_collision" >&2
        return 2
    fi
    [[ -n "$newest" ]] || return 1
    printf '%s\n' "$newest"
}

numeric_string_gt() {
    local left="$1" right="$2"
    if [[ "${#left}" -ne "${#right}" ]]; then
        [[ "${#left}" -gt "${#right}" ]]
    else
        [[ "$left" > "$right" ]]
    fi
}

valid_backup_timestamp() {
    local value="$1" year month day hour minute second max_day
    year=$((10#${value:0:4}))
    month=$((10#${value:4:2}))
    day=$((10#${value:6:2}))
    hour=$((10#${value:9:2}))
    minute=$((10#${value:11:2}))
    second=$((10#${value:13:2}))
    [[ "$year" -ge 1 && "$month" -ge 1 && "$month" -le 12 &&
        "$hour" -le 23 && "$minute" -le 59 && "$second" -le 59 ]] || return 1
    case "$month" in
        1|3|5|7|8|10|12) max_day=31 ;;
        4|6|9|11) max_day=30 ;;
        2)
            max_day=28
            if (( year % 400 == 0 || (year % 4 == 0 && year % 100 != 0) )); then max_day=29; fi
            ;;
    esac
    [[ "$day" -ge 1 && "$day" -le "$max_day" ]]
}

record_parent_dirs() {
    local path="$1" dir home_real dir_real
    home_real="$(realpath_or_self "$HOME")"
    dir="$(dirname "$path")"
    while [[ -n "$dir" && "$dir" != "/" ]]; do
        dir_real="$(realpath_or_self "$dir")"
        [[ "$dir_real" == "$home_real" ]] && break
        printf '%s\n' "$dir" >> "$DIR_CANDIDATES_FILE"
        dir="$(dirname "$dir")"
    done
}

restore_backup_if_present() {
    local target="$1" backup="${2:-}"
    [[ "$RESTORE_BACKUPS" -eq 1 ]] || return 0
    [[ -n "$backup" ]] || return 0
    if [[ "$DRY_RUN" -eq 1 ]]; then
        # Accurate preview: in a real run the target was just removed, so the
        # restore would proceed; show that rather than the exists-guard warning.
        echo "  would: restore $backup -> $target"
        return 0
    fi
    if target_exists "$target"; then
        warn "not restoring $backup because $target already exists"
        skipped=$((skipped + 1))
        return 0
    fi
    mv "$backup" "$target"
    restored=$((restored + 1))
    echo "  restored  $target <- $backup"
}

prompt_category() {
    local prompt="$1"
    if [[ "$ALL" -eq 1 || "$DRY_RUN" -eq 1 ]]; then
        return 0
    fi
    if [[ ! -t 0 ]]; then
        warn "no TTY and --all was not passed; skipping $prompt"
        return 1
    fi
    local answer
    printf "%s [y/N] " "$prompt"
    if ! read -r answer; then
        return 1
    fi
    [[ "$answer" =~ ^[Yy]$ ]]
}

is_windows_terminal_settings() {
    case "$1" in
        *"/AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/settings.json") return 0 ;;
        *) return 1 ;;
    esac
}

is_external_path() {
    case "$1" in
        "$HOME/.local/share/dotfiles/zsh-plugins"/*|"$HOME/.local/share/dotfiles/zsh-plugins") return 0 ;;
        *) return 1 ;;
    esac
}

require_managed_targets() {
    if ! have chezmoi; then
        echo "uninstall: required command 'chezmoi' is not on PATH" >&2
        exit 1
    fi
    if [[ ! -d "$SOURCE_DIR" ]]; then
        echo "uninstall: missing chezmoi source dir: $SOURCE_DIR" >&2
        exit 1
    fi

    local output
    if ! output="$(chezmoi --source "$SOURCE_DIR" managed --path-style absolute 2>&1)"; then
        printf '%s\n' "$output" >&2
        cat >&2 <<EOF
uninstall: could not enumerate managed targets.
Run this only after the chezmoi source has been initialized for this HOME, e.g.:
  chezmoi --source "$SOURCE_DIR" init
EOF
        exit 1
    fi
    printf '%s\n' "$output" | awk 'NF > 0 { print length($0) "\t" $0 }' | sort -rn | cut -f2-
}

remove_managed_target() {
    local target="$1" link_target backup="" backup_rc=0
    is_external_path "$target" && return 0

    if is_windows_terminal_settings "$target"; then
        if target_exists "$target"; then
            warn "leaving Windows Terminal settings.json untouched: $target"
            echo "      WT merge is idempotent but not invertible; restore manually from backup if needed."
            if backup="$(newest_backup "$target")"; then
                printf '      newest backup: %s\n' "$backup"
            else
                backup_rc=$?
                [[ "$backup_rc" -eq 1 ]] || return "$backup_rc"
            fi
        fi
        return 0
    fi

    if ! target_exists "$target"; then
        return 0
    fi

    if [[ "$RESTORE_BACKUPS" -eq 1 ]]; then
        if backup="$(newest_backup "$target")"; then
            :
        else
            backup_rc=$?
            if [[ "$backup_rc" -ne 1 ]]; then return "$backup_rc"; fi
            backup=""
        fi
    fi

    if [[ -L "$target" ]]; then
        link_target="$(readlink "$target" || true)"
        if link_points_into_repo "$target" "$link_target"; then
            if [[ "$DRY_RUN" -eq 1 ]]; then
                echo "  would: remove symlink $target -> $link_target"
            else
                rm "$target"
                removed=$((removed + 1))
                echo "  removed   $target"
            fi
            record_parent_dirs "$target"
            restore_backup_if_present "$target" "$backup"
            return 0
        fi
        warn "skipping symlink outside repo: $target -> $link_target"
        skipped=$((skipped + 1))
        return 0
    fi

    if [[ -d "$target" ]]; then
        record_parent_dirs "$target"
        return 0
    fi

    warn "skipping user-owned/non-symlink target: $target"
    skipped=$((skipped + 1))
}

remove_empty_dirs() {
    [[ -s "$DIR_CANDIDATES_FILE" ]] || return 0
    while IFS= read -r dir; do
        [[ -d "$dir" ]] || continue
        if rmdir "$dir" 2>/dev/null; then
            dirs_removed=$((dirs_removed + 1))
            echo "  rmdir     $dir"
        fi
    done < <(awk '{ print length($0) "\t" $0 }' "$DIR_CANDIDATES_FILE" | sort -rn | cut -f2- | awk '!seen[$0]++')
}

external_is_dirty() {
    # A pinned chezmoi external is a clean detached-HEAD clone. Treat any
    # uncommitted/staged change or untracked file as user work to preserve. If
    # git is unavailable or the path is not a git repo, cleanliness cannot be
    # verified, so err on the safe side and treat it as dirty.
    local dir="$1" status
    have git || return 0
    git -C "$dir" rev-parse --git-dir >/dev/null 2>&1 || return 0
    # --ignored so a user file that matches the plugin's .gitignore (a cache or
    # build artifact git treats as ignored) still counts as dirty and is kept,
    # not silently removed -- plain --porcelain omits ignored files. If the
    # status query itself fails, cleanliness is unknown -> treat as dirty.
    status="$(git -C "$dir" status --porcelain --ignored 2>/dev/null)" || return 0
    [[ -n "$status" ]]
}

remove_externals() {
    local root name dir
    [[ "$KEEP_EXTERNALS" -eq 0 ]] || {
        echo "  kept      zsh plugin externals (--keep-externals)"
        return 0
    }
    if ! prompt_category "Remove zsh plugin externals under ~/.local/share/dotfiles/zsh-plugins?"; then
        skipped=$((skipped + 1))
        return 0
    fi

    root="$HOME/.local/share/dotfiles/zsh-plugins"
    for name in fzf-tab zsh-autosuggestions; do
        dir="$root/$name"
        target_exists "$dir" || continue
        if [[ "$FORCE_EXTERNALS" -ne 1 ]] && external_is_dirty "$dir"; then
            warn "keeping $dir: uncommitted or unverifiable changes (use --force-externals to remove)"
            skipped=$((skipped + 1))
            continue
        fi
        if [[ "$DRY_RUN" -eq 1 ]]; then
            echo "  would: remove external $dir"
        else
            rm -rf "$dir"
            externals_removed=$((externals_removed + 1))
            echo "  removed   $dir"
        fi
    done
    if [[ "$DRY_RUN" -ne 1 ]]; then
        rmdir "$root" "$HOME/.local/share/dotfiles" 2>/dev/null || true
    fi
}

if [[ "${DOTFILES_UNINSTALL_SOURCE_ONLY:-}" == "1" ]]; then
    return 0
fi

echo "uninstall: repo=$REPO_ROOT source=$SOURCE_DIR dry-run=$DRY_RUN restore-backups=$RESTORE_BACKUPS"
echo

managed_targets="$(require_managed_targets)"
if [[ -n "$managed_targets" ]] && prompt_category "Remove chezmoi-managed config targets?"; then
    while IFS= read -r target; do
        [[ -n "$target" ]] || continue
        remove_managed_target "$target"
    done <<< "$managed_targets"
else
    skipped=$((skipped + 1))
fi

if [[ "$DRY_RUN" -ne 1 ]]; then
    remove_empty_dirs
fi
remove_externals

echo
if [[ "$removed" -eq 0 && "$restored" -eq 0 && "$dirs_removed" -eq 0 && "$externals_removed" -eq 0 ]]; then
    echo "uninstall: nothing to remove"
fi
printf 'summary: removed=%s restored=%s dirs_removed=%s externals_removed=%s skipped=%s warnings=%s\n' \
    "$removed" "$restored" "$dirs_removed" "$externals_removed" "$skipped" "$warnings"
