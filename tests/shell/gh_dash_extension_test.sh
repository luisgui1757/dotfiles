#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
# Unit tests for install_gh_dash_extension: gh-absent skip, pinned dry-run
# command, and idempotence when the extension is already installed. Uses the
# INSTALL_DEPS_SOURCE_ONLY seam so no real install runs; gh is stubbed.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

# --- gh absent -> skipped, no install attempt --------------------------------
(
    have() { return 1; }          # force "gh not present"
    DRY_RUN=0; YES_ALL=1
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"skipped"* && "$out" == *"gh CLI not installed"* ]] \
        || fail "gh-absent must skip; got: $out"
)

# --- gh present, extension missing, dry-run -> pinned would: command ----------
(
    gh() { case "$*" in "extension list") return 0 ;; *) return 0 ;; esac; }  # list empty
    DRY_RUN=1; YES_ALL=1
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"would: gh extension install dlvhdr/gh-dash --pin $GH_DASH_VERSION"* ]] \
        || fail "dry-run must print the pinned install command; got: $out"
)

# --- gh present, extension already installed -> idempotent ok -----------------
(
    gh() { case "$*" in "extension list") echo "gh dash  dlvhdr/gh-dash  $GH_DASH_VERSION" ;; *) return 0 ;; esac; }
    DRY_RUN=0; YES_ALL=1
    out="$(install_gh_dash_extension)"
    [[ "$out" == *"ok"* && "$out" == *"already installed"* ]] \
        || fail "already-installed must be idempotent ok; got: $out"
)

echo "OK"
