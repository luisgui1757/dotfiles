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

# Hyperfine 1.x exports mean directly; schema 2 exports named metric summaries.
# Keep the existing wall-clock seconds and integer-millisecond budget semantics.
mean_ms=$(python3 - "$JSON_OUT" <<'PY'
import json
import math
import sys

try:
    with open(sys.argv[1]) as source:
        data = json.load(source)
    results = data["results"]
    if not isinstance(results, list) or len(results) != 1:
        raise ValueError("expected exactly one benchmark result")
    version = data.get("schema_version", 1)
    if type(version) is not int:
        raise ValueError("schema version must be an integer")
    if version == 1:
        mean = results[0]["mean"]
    elif version == 2:
        wall_clock = results[0]["summary"]["time_wall_clock"]
        if wall_clock["unit"] != "second":
            raise ValueError("wall-clock unit must be second")
        mean = wall_clock["mean"]
    else:
        raise ValueError(f"unsupported schema version {version}")
    if type(mean) not in (int, float) or not math.isfinite(mean) or mean < 0:
        raise ValueError("mean must be a finite nonnegative number")
    print(int(mean * 1000))
except (OSError, KeyError, TypeError, ValueError, OverflowError) as error:
    raise SystemExit(f"FAIL: invalid Hyperfine wall-clock result: {error}")
PY
)

echo "starship prompt mean = ${mean_ms}ms (budget ${budget_ms}ms)"
if [[ "$mean_ms" -gt "$budget_ms" ]]; then
    echo "FAIL: prompt mean ${mean_ms}ms exceeds budget ${budget_ms}ms"
    exit 1
fi
echo "OK"
