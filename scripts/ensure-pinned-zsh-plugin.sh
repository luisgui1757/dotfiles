#!/usr/bin/env bash
# Atomically provision one sourceable zsh plugin checkout at an exact commit.
# Bash 3.2 compatible; production accepts only HTTPS GitHub origins.
set -euo pipefail

if [[ "$#" -ne 6 ]]; then
    echo "usage: ensure-pinned-zsh-plugin.sh NAME REPO REF COMMIT REQUIRED_FILE TARGET" >&2
    exit 2
fi

name="$1"
repo="$2"
ref="$3"
commit="$4"
required_file="$5"
target="$6"

if [[ "${#commit}" -ne 40 ]]; then
    echo "FAIL: $name pin is not a full lowercase 40-hex commit: $commit" >&2
    exit 2
fi
case "$commit" in
    *[!0-9a-f]*) echo "FAIL: $name pin is not a full lowercase 40-hex commit: $commit" >&2; exit 2 ;;
esac
case "/$required_file/" in
    *'/../'*|*'/./'*|*'//'*) echo "FAIL: $name required file is unsafe: $required_file" >&2; exit 2 ;;
esac
case "$required_file" in
    ''|/*) echo "FAIL: $name required file is unsafe: $required_file" >&2; exit 2 ;;
esac
case "$repo" in
    https://github.com/*.git) ;;
    file://*|/*)
        [[ "${DOTFILES_PINNED_GIT_ALLOW_FILE:-0}" == "1" ]] || {
            echo "FAIL: $name origin must be an HTTPS GitHub .git URL" >&2
            exit 2
        }
        ;;
    *) echo "FAIL: $name origin must be an HTTPS GitHub .git URL" >&2; exit 2 ;;
esac

parent="$(dirname "$target")"
mkdir -p "$parent"
parent="$(cd "$parent" && pwd -P)"
target="$parent/$(basename "$target")"
lock="${target}.lock"
stage=""
quarantine=""
have_lock=0

cleanup() {
    rc=$?
    trap - EXIT HUP INT TERM
    [[ -z "$stage" ]] || rm -rf "$stage"
    if [[ "$have_lock" -eq 1 ]]; then rm -rf "$lock"; fi
    exit "$rc"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

git_safe() {
    if [[ "${DOTFILES_PINNED_GIT_ALLOW_FILE:-0}" == "1" ]]; then
        env GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
            GIT_OPTIONAL_LOCKS=0 \
            GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=Never GIT_TEMPLATE_DIR= \
            git -c core.hooksPath=/dev/null -c core.fsmonitor=false \
            -c core.untrackedCache=false -c credential.helper= \
            -c protocol.file.allow=always "$@"
    else
        env GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
            GIT_OPTIONAL_LOCKS=0 \
            GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=Never GIT_TEMPLATE_DIR= \
            git -c core.hooksPath=/dev/null -c core.fsmonitor=false \
            -c core.untrackedCache=false -c credential.helper= \
            -c protocol.file.allow=never "$@"
    fi
}

normalize_origin() {
    local value="$1"
    value="${value%/}"
    value="${value%.git}"
    printf '%s\n' "$value"
}

checkout_base_ok() {
    local path="$1" expected_repo="$2" file="$3"
    local inside origin root status
    [[ -d "$path/.git" && -f "$path/$file" && ! -L "$path/$file" ]] || return 1
    inside="$(git_safe -C "$path" rev-parse --is-inside-work-tree 2>/dev/null)" || return 1
    [[ "$inside" == "true" ]] || return 1
    root="$(git_safe -C "$path" rev-parse --show-toplevel 2>/dev/null)" || return 1
    [[ -n "$root" && -d "$root" ]] || return 1
    root="$(cd "$root" && pwd -P)"
    [[ "$root" == "$(cd "$path" && pwd -P)" ]] || return 1
    origin="$(git_safe -C "$path" remote get-url origin 2>/dev/null)" || return 1
    [[ "$(normalize_origin "$origin")" == "$(normalize_origin "$expected_repo")" ]] || return 1
    git_safe -C "$path" ls-files --error-unmatch -- "$file" >/dev/null 2>&1 || return 1
    git_safe -C "$path" cat-file -e "HEAD:$file" 2>/dev/null || return 1
    status="$(git_safe -C "$path" status --porcelain=v1 --untracked-files=all --ignored 2>/dev/null)" || return 1
    [[ -z "$status" ]] || return 1
    git_safe -C "$path" diff --quiet --ignore-submodules -- 2>/dev/null || return 1
    git_safe -C "$path" diff --cached --quiet --ignore-submodules -- 2>/dev/null || return 1
}

checkout_ok() {
    local path="$1" expected_repo="$2" expected_commit="$3" file="$4" head
    checkout_base_ok "$path" "$expected_repo" "$file" || return 1
    head="$(git_safe -C "$path" rev-parse --verify HEAD 2>/dev/null)" || return 1
    [[ "$head" == "$expected_commit" ]]
}

unique_quarantine() {
    local base stamp candidate n=0
    base="$1"
    stamp="$(date +%Y%m%d-%H%M%S)"
    candidate="${base}.quarantine.${stamp}"
    while [[ -e "$candidate" || -L "$candidate" ]]; do
        n=$((n + 1))
        candidate="${base}.quarantine.${stamp}.${n}"
    done
    printf '%s\n' "$candidate"
}

attempt=0
while ! mkdir "$lock" 2>/dev/null; do
    lock_pid="$(cat "$lock/pid" 2>/dev/null || true)"
    case "$lock_pid" in
        ''|*[!0-9]*) ;;
        *)
            if ! kill -0 "$lock_pid" 2>/dev/null; then
                stale="${lock}.stale.$$"
                if mv "$lock" "$stale" 2>/dev/null; then rm -rf "$stale"; fi
                continue
            fi
            ;;
    esac
    attempt=$((attempt + 1))
    if [[ "$attempt" -ge 300 ]]; then
        echo "FAIL: timed out waiting for $name publication lock: $lock" >&2
        exit 1
    fi
    sleep 0.1
done
have_lock=1
printf '%s\n' "$$" > "$lock/pid"

if [[ "${DOTFILES_PINNED_GIT_CHECK_ONLY:-0}" == "1" ]]; then
    if checkout_ok "$target" "$repo" "$commit" "$required_file"; then
        printf '%s\n' "ready:$commit"
    elif [[ -n "${DOTFILES_PINNED_GIT_BOOTSTRAP_MARKER:-}" &&
        ! -e "$DOTFILES_PINNED_GIT_BOOTSTRAP_MARKER" ]]; then
        # First apply has no chezmoi script state yet and will execute this
        # run_onchange script. Match the post-install fingerprint so a fresh
        # successful apply verifies cleanly without a redundant second run.
        printf '%s\n' "ready:$commit"
    else
        printf '%s\n' "invalid"
    fi
    exit 0
fi

if checkout_ok "$target" "$repo" "$commit" "$required_file"; then
    printf "  ok        %-26s %s (%s)\n" "$name" "$ref" "$commit"
    exit 0
fi

discard_quarantine=0
if [[ -e "$target" || -L "$target" ]]; then
    if checkout_base_ok "$target" "$repo" "$required_file"; then
        discard_quarantine=1
    fi
    quarantine="$(unique_quarantine "$target")"
    if ! mv "$target" "$quarantine"; then
        echo "FAIL: could not neutralize mismatched $name checkout at $target" >&2
        exit 1
    fi
    printf "  quarantine %-22s %s\n" "$name" "$quarantine" >&2
fi

stage="$(mktemp -d "${target}.stage.XXXXXX")"
git_safe -C "$stage" init -q --template= || {
    echo "FAIL: could not initialize $name staging checkout" >&2
    exit 1
}
git_safe -C "$stage" remote add origin "$repo"
git_safe -C "$stage" fetch --no-tags --depth 1 origin "$commit" >/dev/null 2>&1 || {
    echo "FAIL: could not fetch $name exact commit $commit" >&2
    exit 1
}
git_safe -C "$stage" checkout --detach FETCH_HEAD >/dev/null 2>&1 || {
    echo "FAIL: could not checkout $name exact commit $commit" >&2
    exit 1
}
checkout_ok "$stage" "$repo" "$commit" "$required_file" || {
    echo "FAIL: staged $name checkout failed origin/HEAD/cleanliness/file proof" >&2
    exit 1
}

if ! mv "$stage" "$target"; then
    echo "FAIL: could not atomically publish verified $name checkout to $target" >&2
    exit 1
fi
stage=""
checkout_ok "$target" "$repo" "$commit" "$required_file" || {
    echo "FAIL: published $name checkout failed final proof; neutralizing it" >&2
    quarantine_bad="$(unique_quarantine "$target")"
    if ! mv "$target" "$quarantine_bad" 2>/dev/null; then
        rm -f "$target/$required_file" 2>/dev/null || true
        if [[ -r "$target/$required_file" ]]; then
            echo "FAIL: could not neutralize the unproved published payload at $target/$required_file; do not start zsh until it is removed" >&2
        fi
    fi
    exit 1
}

if [[ -n "$quarantine" ]]; then
    if [[ "$discard_quarantine" -eq 1 ]]; then
        rm -rf "$quarantine"
    else
        printf "  recovery  %-22s preserved prior payload at %s\n" "$name" "$quarantine" >&2
    fi
fi
printf "  installed %-26s %s (%s)\n" "$name" "$ref" "$commit"
