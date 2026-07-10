#!/usr/bin/env python3
"""Compare Renovate's official local extraction with the reviewed inventory."""

from __future__ import annotations

import collections
import json
import pathlib
import sys


def load_extraction(path: pathlib.Path) -> dict:
    extracted = []
    for line in path.read_text(encoding="utf-8").splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("msg") == "Extracted dependencies":
            extracted.append(event)
    if len(extracted) != 1:
        raise ValueError(f"expected one Renovate extraction event, found {len(extracted)}")
    return extracted[0]


def actual_inventory(event: dict) -> collections.Counter[tuple[str, str, str, str]]:
    inventory: collections.Counter[tuple[str, str, str, str]] = collections.Counter()
    invalid: list[str] = []
    for manager, package_files in event.get("packageFiles", {}).items():
        for package_file in package_files:
            filename = package_file.get("packageFile", "")
            for dependency in package_file.get("deps", []):
                dep_name = dependency.get("depName", "")
                datasource = dependency.get("datasource", "")
                if not dep_name or not datasource:
                    invalid.append(f"{manager}|{filename}: missing depName or datasource")
                    continue
                if not any(
                    dependency.get(field)
                    for field in ("currentValue", "currentDigest", "lockedVersion")
                ):
                    invalid.append(f"{manager}|{filename}|{datasource}|{dep_name}: missing current identity")
                skip_reason = dependency.get("skipReason")
                if skip_reason and skip_reason != "github-token-required":
                    invalid.append(
                        f"{manager}|{filename}|{datasource}|{dep_name}: skipReason={skip_reason}"
                    )
                inventory[(manager, filename, datasource, dep_name)] += 1
    if invalid:
        raise ValueError("Renovate extracted unusable dependencies:\n" + "\n".join(invalid))
    return inventory


def expected_inventory(path: pathlib.Path) -> collections.Counter[tuple[str, str, str, str]]:
    inventory: collections.Counter[tuple[str, str, str, str]] = collections.Counter()
    for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("|")
        if len(parts) != 5:
            raise ValueError(f"{path}:{line_number}: expected five pipe-separated fields")
        manager, filename, datasource, dep_name, count_text = parts
        count = int(count_text)
        if count < 1:
            raise ValueError(f"{path}:{line_number}: count must be positive")
        inventory[(manager, filename, datasource, dep_name)] += count
    return inventory


def render(inventory: collections.Counter[tuple[str, str, str, str]]) -> list[str]:
    return ["|".join((*key, str(count))) for key, count in sorted(inventory.items())]


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <renovate-json-log> <expected-inventory>", file=sys.stderr)
        return 2
    actual = actual_inventory(load_extraction(pathlib.Path(sys.argv[1])))
    expected = expected_inventory(pathlib.Path(sys.argv[2]))
    if actual != expected:
        print("FAIL: Renovate's extracted dependency inventory drifted", file=sys.stderr)
        actual_lines = set(render(actual))
        expected_lines = set(render(expected))
        for line in sorted(expected_lines - actual_lines):
            print(f"  missing: {line}", file=sys.stderr)
        for line in sorted(actual_lines - expected_lines):
            print(f"  extra:   {line}", file=sys.stderr)
        return 1

    managers = {key[0] for key in actual}
    if managers != {"github-actions", "nix", "regex"}:
        print(f"FAIL: incomplete Renovate manager inventory: {sorted(managers)}", file=sys.stderr)
        return 1
    print(f"OK: Renovate officially extracted {sum(actual.values())} reviewed dependency records")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
