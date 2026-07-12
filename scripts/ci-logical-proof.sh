#!/usr/bin/env bash
# Publish/verify an immutable-by-run marker that binds a stable logical check
# identity to the actual runner-specific proof job that produced it.
set -euo pipefail

usage() {
    echo "usage: $0 emit <marker> <logical-context> <legacy-context> | verify <marker> <logical-context> <legacy-context>" >&2
    exit 2
}

valid_text() {
    [[ -n "$1" && "$1" != *$'\n'* && "$1" != *$'\r'* ]]
}

emit_marker() (
    local marker="$1" logical="$2" legacy="$3" temp source_head_sha executed_sha
    if ! valid_text "$logical" || ! valid_text "$legacy"; then
        echo "FAIL: proof identities must be nonempty single-line values" >&2
        exit 1
    fi
    source_head_sha="${DOTFILES_SOURCE_HEAD_SHA:-}"
    executed_sha="${GITHUB_SHA:-}"
    [[ "$source_head_sha" =~ ^[0-9a-f]{40}$ ]] || {
        echo "FAIL: DOTFILES_SOURCE_HEAD_SHA is not a full commit identity" >&2
        exit 1
    }
    [[ "$executed_sha" =~ ^[0-9a-f]{40}$ ]] || {
        echo "FAIL: GITHUB_SHA is not a full executed commit identity" >&2
        exit 1
    }
    [[ "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ && "${GITHUB_RUN_ATTEMPT:-}" =~ ^[0-9]+$ ]] || {
        echo "FAIL: GitHub run identity is unavailable" >&2
        exit 1
    }
    mkdir -p "$(dirname "$marker")"
    temp="${marker}.$$.tmp"
    trap 'rm -f "$temp"' EXIT HUP INT TERM
    umask 077
    {
        printf 'schema=2\n'
        printf 'source_head_sha=%s\n' "$source_head_sha"
        printf 'executed_sha=%s\n' "$executed_sha"
        printf 'run_id=%s\n' "$GITHUB_RUN_ID"
        printf 'run_attempt=%s\n' "$GITHUB_RUN_ATTEMPT"
        printf 'logical_context=%s\n' "$logical"
        printf 'legacy_context=%s\n' "$legacy"
    } > "$temp"
    mv -f "$temp" "$marker"
    trap - EXIT HUP INT TERM
)

verify_marker() {
    local marker="$1" expected_logical="$2" expected_legacy="$3"
    local schema="" source_head_sha="" executed_sha="" run_id="" run_attempt=""
    local logical="" legacy="" seen="" key value
    [[ -f "$marker" ]] || { echo "FAIL: logical proof marker is missing: $marker" >&2; return 1; }
    while IFS='=' read -r key value; do
        case " $seen " in *" $key "*) echo "FAIL: duplicate proof field: $key" >&2; return 1 ;; esac
        case "$key" in
            schema) schema="$value" ;;
            source_head_sha) source_head_sha="$value" ;;
            executed_sha) executed_sha="$value" ;;
            run_id) run_id="$value" ;;
            run_attempt) run_attempt="$value" ;;
            logical_context) logical="$value" ;;
            legacy_context) legacy="$value" ;;
            *) echo "FAIL: unknown proof field: $key" >&2; return 1 ;;
        esac
        seen="$seen $key"
    done < "$marker"

    [[ "$schema" == "2" ]] || { echo "FAIL: unsupported logical proof schema" >&2; return 1; }
    [[ "$source_head_sha" == "${DOTFILES_SOURCE_HEAD_SHA:-}" && "$source_head_sha" =~ ^[0-9a-f]{40}$ ]] || {
        echo "FAIL: logical proof does not bind the current source head SHA" >&2
        return 1
    }
    [[ "$executed_sha" == "${GITHUB_SHA:-}" && "$executed_sha" =~ ^[0-9a-f]{40}$ ]] || {
        echo "FAIL: logical proof does not bind the current executed SHA" >&2
        return 1
    }
    [[ "$run_id" == "${GITHUB_RUN_ID:-}" && "$run_attempt" == "${GITHUB_RUN_ATTEMPT:-}" ]] || {
        echo "FAIL: logical proof belongs to a different workflow run" >&2
        return 1
    }
    [[ "$logical" == "$expected_logical" ]] || { echo "FAIL: logical proof context mismatch" >&2; return 1; }
    [[ "$legacy" == "$expected_legacy" ]] || { echo "FAIL: legacy proof context mismatch" >&2; return 1; }
}

[[ "$#" -eq 4 ]] || usage
mode="$1"
shift
case "$mode" in
    emit) emit_marker "$@" ;;
    verify) verify_marker "$@" ;;
    *) usage ;;
esac
