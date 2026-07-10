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
    local marker="$1" logical="$2" legacy="$3" temp
    if ! valid_text "$logical" || ! valid_text "$legacy"; then
        echo "FAIL: proof identities must be nonempty single-line values" >&2
        exit 1
    fi
    [[ "${GITHUB_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || {
        echo "FAIL: GITHUB_SHA is not a full commit identity" >&2
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
        printf 'schema=1\n'
        printf 'head_sha=%s\n' "$GITHUB_SHA"
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
    local schema="" head_sha="" run_id="" run_attempt="" logical="" legacy="" seen="" key value
    [[ -f "$marker" ]] || { echo "FAIL: logical proof marker is missing: $marker" >&2; return 1; }
    while IFS='=' read -r key value; do
        case " $seen " in *" $key "*) echo "FAIL: duplicate proof field: $key" >&2; return 1 ;; esac
        case "$key" in
            schema) schema="$value" ;;
            head_sha) head_sha="$value" ;;
            run_id) run_id="$value" ;;
            run_attempt) run_attempt="$value" ;;
            logical_context) logical="$value" ;;
            legacy_context) legacy="$value" ;;
            *) echo "FAIL: unknown proof field: $key" >&2; return 1 ;;
        esac
        seen="$seen $key"
    done < "$marker"

    [[ "$schema" == "1" ]] || { echo "FAIL: unsupported logical proof schema" >&2; return 1; }
    [[ "$head_sha" == "${GITHUB_SHA:-}" && "$head_sha" =~ ^[0-9a-f]{40}$ ]] || {
        echo "FAIL: logical proof does not bind the current head SHA" >&2
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
