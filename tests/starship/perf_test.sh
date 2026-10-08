#!/usr/bin/env bash
# Assert mean prompt render time stays under budget.
# Budget is loose for CI (slow shared runners); tighter on local dev machines.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! command -v starship >/dev/null 2>&1; then
    echo "skipped: starship not installed"
    exit 0
fi
if ! command -v hyperfine >/dev/null 2>&1; then
    echo "skipped: hyperfine not installed (brew install hyperfine)"
    exit 0
fi

# Loose budget — CI runners can be slow. The point is to catch a 5x regression,
# not measure absolute speed.
budget_ms=80
if [[ "${CI:-}" == "true" ]]; then budget_ms=150; fi

# Use a real git repo as cwd so the git_status module actually runs.
# Self-configure user.email/name so the commit works even on CI runners
# without a global git identity.
TMP_REPO="$(mktemp -d)"
trap 'rm -rf "$TMP_REPO"' EXIT
( cd "$TMP_REPO" \
    && git init -q \
    && git -c user.email=ci@example.com -c user.name=ci commit --allow-empty -qm "test" \
    && touch foo )

JSON_OUT="$TMP_REPO/hyperfine.json"
(
    cd "$TMP_REPO"
    STARSHIP_CONFIG="$REPO_ROOT/starship/starship.toml" \
        hyperfine --warmup 3 --runs 20 --export-json "$JSON_OUT" \
        "starship prompt --jobs 0 --status 0 --cmd-duration 0" >/dev/null
)

# Compare the unrounded wall-clock measurement from either supported schema.
python3 "$REPO_ROOT/tests/starship/check_perf.py" "$JSON_OUT" "$budget_ms"
