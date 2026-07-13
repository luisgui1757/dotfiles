#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
# Unit tests for install_gh_dash_extension. gh-dash needs an authenticated gh to
# be useful, and an unauthenticated `gh extension install` hits GitHub's
# anonymous rate limit -- so the installer is auth-gated, verifies the installed
# pin, and re-pins on mismatch. Uses the INSTALL_DEPS_SOURCE_ONLY seam (no real
# install); gh is stubbed via AUTH_RC / EXT_LIST / INSTALL_RC / CALL_LOG.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

# Shared gh stub. `auth status` returns AUTH_RC; `extension list` prints EXT_LIST;
# `extension install` returns INSTALL_RC. remove/install invocations are recorded
# to CALL_LOG when set, so the re-pin path can be asserted.
gh() {
    case "$1 ${2:-}" in
        "auth status")
            return "${AUTH_RC:-0}"
            ;;
        "extension list")
            if [[ -n "${EXT_LIST:-}" ]]; then printf '%s\n' "$EXT_LIST"; fi
            return 0
            ;;
        "extension remove")
            if [[ -n "${CALL_LOG:-}" ]]; then echo "remove ${*:3}" >> "$CALL_LOG"; fi
            return 0
            ;;
        "extension install")
            if [[ -n "${CALL_LOG:-}" ]]; then echo "install ${*:3}" >> "$CALL_LOG"; fi
            return "${INSTALL_RC:-0}"
            ;;
        api\ repos/dlvhdr/gh-dash/git/ref/tags/*)
            printf '%s\n' "${TAG_OBJECT_RESULT:-$GH_DASH_TAG_OBJECT}"
            return "${API_RC:-0}"
            ;;
        api\ repos/dlvhdr/gh-dash/git/tags/*)
            printf '%s\n' "${PEELED_COMMIT_RESULT:-$GH_DASH_COMMIT}"
            return "${API_RC:-0}"
            ;;
        *)
            return 0
            ;;
    esac
}

# --- 1. gh absent -> skipped, no install attempt -----------------------------
(
    have() { return 1; }          # force "gh not present"
    DRY_RUN=0; YES_ALL=1
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"skipped"* && "$out" == *"gh CLI not installed"* ]] \
        || fail "gh-absent must skip; got: $out"
    [[ "$out" != *"FAIL"* ]] || fail "gh-absent must not emit FAIL; got: $out"
)

# --- 2. gh present but unauthenticated -> skipped, no FAIL --------------------
(
    AUTH_RC=1; DRY_RUN=0; YES_ALL=1
    out="$(install_gh_dash_extension 2>&1)"
    [[ "$out" == *"skipped"* && "$out" == *"gh auth login"* ]] \
        || fail "unauthenticated must skip with a 'gh auth login' hint; got: $out"
    [[ "$out" != *"FAIL"* ]] || fail "unauthenticated must NOT emit FAIL; got: $out"
)

# --- 3. authenticated, missing, dry-run -> pinned would: command -------------
(
    AUTH_RC=0; EXT_LIST=""; DRY_RUN=1; YES_ALL=1
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"tag object $GH_DASH_TAG_OBJECT peels to $GH_DASH_COMMIT"* ]] \
        || fail "dry-run must expose the reviewed tag-to-commit identity; got: $out"
    [[ "$out" == *"would: gh extension install dlvhdr/gh-dash --pin $GH_DASH_VERSION"* ]] \
        || fail "authenticated+missing dry-run must print the pinned install command; got: $out"
)

# --- 4. authenticated, installed at the expected pin -> idempotent ok ---------
(
    AUTH_RC=0; EXT_LIST="gh dash  dlvhdr/gh-dash  $GH_DASH_VERSION"; DRY_RUN=0; YES_ALL=1
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"ok"* && "$out" == *"already installed"* ]] \
        || fail "matching pin must be idempotent ok; got: $out"
)

# --- 5. authenticated, installed at the WRONG pin -> force remove+install -----
(
    AUTH_RC=0; EXT_LIST="gh dash  dlvhdr/gh-dash  v4.20.0"; INSTALL_RC=0; DRY_RUN=0; YES_ALL=1
    CALL_LOG="$(mktemp "${TMPDIR:-/tmp}/ghdash.XXXXXX")"
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"installed"* && "$out" == *"re-pinned"* ]] \
        || fail "wrong pin must re-pin; got: $out"
    grep -q "remove dash" "$CALL_LOG" \
        || fail "wrong pin must remove the old extension first; log: $(cat "$CALL_LOG")"
    grep -q "install dlvhdr/gh-dash --pin $GH_DASH_VERSION" "$CALL_LOG" \
        || fail "wrong pin must reinstall at the expected pin; log: $(cat "$CALL_LOG")"
    rm -f "$CALL_LOG"
)

# --- 7. moved tag is rejected before extension mutation ----------------------
(
    AUTH_RC=0; EXT_LIST=""; TAG_OBJECT_RESULT=deadbeef; DRY_RUN=0; YES_ALL=1
    INSTALL_FAILURES_COUNT=0; INSTALL_FAILURES_DETAIL=""
    CALL_LOG="$(mktemp "${TMPDIR:-/tmp}/ghdash.XXXXXX")"
    out_file="$(mktemp "${TMPDIR:-/tmp}/ghdash-out.XXXXXX")"
    install_gh_dash_extension >"$out_file" 2>&1
    out="$(cat "$out_file")"
    [[ "$out" == *"tag object mismatch"* ]] || fail "moved tag diagnostic missing: $out"
    [[ "$INSTALL_FAILURES_COUNT" -eq 1 ]] || fail "moved tag must record one failure"
    [[ ! -s "$CALL_LOG" ]] || fail "extension mutated after tag mismatch: $(cat "$CALL_LOG")"
    rm -f "$CALL_LOG" "$out_file"
)

# --- 6. authenticated, missing, install fails -> FAIL marker -----------------
(
    AUTH_RC=0; EXT_LIST=""; INSTALL_RC=1; DRY_RUN=0; YES_ALL=1
    out="$(install_gh_dash_extension 2>&1)"
    [[ "$out" == *"FAIL"* ]] \
        || fail "authenticated install failure must emit FAIL; got: $out"
)

echo "OK"
