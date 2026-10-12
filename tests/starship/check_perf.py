"""Check wall-clock mean from Hyperfine 1.x or schema 2, in seconds.

Source: sharkdp/hyperfine v2.0.0 src/export/json.rs and src/metric.rs.
The budget remains 80 ms locally or 150 ms in CI; never round before comparing.
"""
import json
import math
from pathlib import Path
import sys


def check(path, budget_ms):
    data = json.loads(Path(path).read_text())
    results = data["results"]
    if not isinstance(results, list) or len(results) != 1:
        raise ValueError("expected exactly one prompt benchmark")
    version = data.get("schema_version", 1)
    if type(version) is not int:
        raise ValueError("schema version must be an integer")
    if version == 1:
        seconds = results[0]["mean"]
    elif version == 2:
        wall = results[0]["summary"]["time_wall_clock"]
        if wall["unit"] != "second":
            raise ValueError("wall-clock unit must be second")
        seconds = wall["mean"]
    else:
        raise ValueError(f"unsupported Hyperfine schema {version!r}")
    if isinstance(seconds, bool) or not isinstance(seconds, (int, float)) or not math.isfinite(seconds) or seconds < 0:
        raise ValueError("wall-clock mean must be finite and nonnegative")
    milliseconds = seconds * 1000
    print(f"starship prompt mean = {milliseconds:g}ms (budget {budget_ms:g}ms)")
    if milliseconds > budget_ms:
        raise ValueError(f"prompt mean {milliseconds:g}ms exceeds budget {budget_ms:g}ms")
    print("OK")


if __name__ == "__main__":
    try:
        check(sys.argv[1], float(sys.argv[2]))
    except (ValueError, KeyError, TypeError, OSError, OverflowError) as error:
        raise SystemExit(f"FAIL: invalid Hyperfine or over-budget prompt measurement: {error}") from None
