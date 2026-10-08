#!/usr/bin/env bash
# Exercise the real performance gate with process-boundary Hyperfine exports.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

python3 - "$REPO_ROOT" <<'PY'
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

gate = Path(sys.argv[1]) / "tests/starship/perf_test.sh"

# Export shapes verified with real Hyperfine 1.20.0 and 2.0.0. V2 separates
# wall-clock and CPU metrics; the gate must keep measuring wall-clock seconds.
def legacy(mean):
    return {"results": [{"mean": mean}]}


def v2(mean, unit="second"):
    return {
        "schema_version": 2,
        "primary_metric": "time_cpu",
        "results": [{"summary": {
            "time_wall_clock": {"mean": mean, "unit": unit},
            "time_cpu": {"mean": 0.001, "unit": "second"},
        }}],
    }


cases = [
    ("legacy below local budget", legacy(0.079), "false", True, "mean = 79ms"),
    ("v2 below local budget", v2(0.079), "false", True, "mean = 79ms"),
    ("legacy above local budget", legacy(0.081), "false", False, "exceeds budget 80ms"),
    ("v2 above local budget", v2(0.081), "false", False, "exceeds budget 80ms"),
    ("legacy CI budget", legacy(0.150), "true", True, "budget 150ms"),
    ("v2 CI budget", v2(0.150), "true", True, "budget 150ms"),
    ("legacy above CI budget", legacy(0.151), "true", False, "exceeds budget 150ms"),
    ("v2 wall-clock above CI budget", v2(0.151), "true", False, "exceeds budget 150ms"),
    ("v2 wrong units", v2(0.001, "millisecond"), "true", False, "invalid Hyperfine"),
    ("unknown schema", {"schema_version": 3, **legacy(0.001)}, "true", False, "invalid Hyperfine"),
    ("empty results", {"results": []}, "true", False, "invalid Hyperfine"),
    ("multiple results", {"results": [{"mean": 0.001}, {"mean": 1}]}, "true", False, "invalid Hyperfine"),
    ("missing mean", {"results": [{}]}, "true", False, "invalid Hyperfine"),
    ("boolean mean", legacy(True), "true", False, "invalid Hyperfine"),
    ("nonfinite mean", v2(float("nan")), "true", False, "invalid Hyperfine"),
    ("negative mean", legacy(-1), "true", False, "invalid Hyperfine"),
    ("quoted temp path", v2(0), "true", True, "mean = 0ms"),
]

with tempfile.TemporaryDirectory() as directory:
    root = Path(directory)
    binaries = root / "bin"
    binaries.mkdir()
    (binaries / "starship").write_text("#!/bin/sh\nexit 0\n")
    (binaries / "hyperfine").write_text('''#!/bin/sh
while [ "$#" -gt 0 ]; do
    if [ "$1" = --export-json ]; then
        cp "$PERF_JSON_FIXTURE" "$2"
        exit
    fi
    shift
done
exit 2
''')
    for binary in binaries.iterdir():
        binary.chmod(0o755)
    fixture = root / "export.json"
    # Never let fixture Git commands inherit a caller's working tree/index.
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    env.update(PATH=str(binaries) + os.pathsep + env["PATH"], PERF_JSON_FIXTURE=str(fixture))
    for name, data, ci, passes, diagnostic in cases:
        fixture.write_text(json.dumps(data))
        case_env = {**env, "CI": ci}
        if name == "quoted temp path":
            temporary = root / "temp space 'quote"
            temporary.mkdir()
            case_env["TMPDIR"] = str(temporary)
        result = subprocess.run(["bash", str(gate)], env=case_env, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if (result.returncode == 0) != passes or diagnostic not in result.stdout:
            raise SystemExit(f"FAIL: {name}: exit {result.returncode}\n{result.stdout}")
        print(f"PASS: {name}")
PY
