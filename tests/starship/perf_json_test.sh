#!/usr/bin/env bash
# Exercise the actual budget checker with old/new external JSON shapes.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
python3 - "$SCRIPT_DIR/check_perf.py" <<'PY'
import json
from pathlib import Path
import subprocess
import sys
import tempfile

def modern(mean, unit="second"):
    return {"schema_version": 2, "results": [{"summary": {
        "time_wall_clock": {"mean": mean, "unit": unit},
        "time_cpu": {"mean": 0.001, "unit": "second"}}}]}

cases = [
    ({"results": [{"mean": 0.080}]}, True),
    ({"results": [{"mean": 0.0801}]}, False),
    (modern(0.080), True), (modern(0.0801), False),
    (modern(0), True), (modern(-1), False),
    (modern(float("nan")), False), (modern(True), False),
    (modern(0.01, "millisecond"), False),
    ({"schema_version": 3, "results": [{"mean": 0.01}]}, False),
    ({"results": []}, False), ({"results": [{}, {}]}, False),
    ({"results": [{}]}, False),
]
with tempfile.TemporaryDirectory(prefix="prompt result ") as directory:
    path = Path(directory) / "measurement.json"
    for data, expected in cases:
        path.write_text(json.dumps(data))
        result = subprocess.run([sys.executable, sys.argv[1], str(path), "80"],
                                text=True, capture_output=True)
        assert (result.returncode == 0) == expected, (data, result)
        if not expected:
            assert "FAIL:" in result.stderr, result
print("PASS: both Hyperfine formats, exact budget, malformed results and units")
PY
