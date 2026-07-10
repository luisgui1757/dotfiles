#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

python3 - <<'PY'
import json
import pathlib
import re
import subprocess
import sys

with open("renovate.json", encoding="utf-8") as fh:
    config = json.load(fh)

if config.get("nix", {}).get("enabled") is not True:
    raise SystemExit("FAIL: Renovate's beta Nix manager is not explicitly enabled")
if config.get("rebaseWhen") != "behind-base-branch":
    raise SystemExit("FAIL: Renovate does not enforce the repository's behind-main policy")

tracked_files = [pathlib.Path(path) for path in subprocess.check_output(["git", "ls-files"], text=True).splitlines()]
failures = []


def renovate_regex_to_python(pattern):
    # Renovate regex managers use ECMAScript-style named groups.
    return re.sub(r"\(\?<([A-Za-z_][A-Za-z0-9_]*)>", r"(?P<\1>", pattern)


def manager_file_regex(raw):
    if raw.startswith("/") and raw.endswith("/"):
        raw = raw[1:-1]
    return re.compile(raw)


for index, manager in enumerate(config.get("customManagers", []), start=1):
    description = manager.get("description", f"custom manager #{index}")
    file_patterns = manager.get("managerFilePatterns", [])
    match_strings = manager.get("matchStrings", [])
    if not file_patterns:
        failures.append(f"{description}: missing managerFilePatterns")
        continue
    if not match_strings:
        failures.append(f"{description}: missing matchStrings")
        continue

    try:
        file_res = [manager_file_regex(pattern) for pattern in file_patterns]
    except re.error as exc:
        failures.append(f"{description}: invalid managerFilePatterns regex: {exc}")
        continue

    matching_files = [
        path for path in tracked_files
        if any(regex.search(path.as_posix()) for regex in file_res)
    ]
    if not matching_files:
        failures.append(f"{description}: managerFilePatterns match no tracked files")
        continue

    for raw_pattern in match_strings:
        try:
            pin_regex = re.compile(renovate_regex_to_python(raw_pattern), re.MULTILINE | re.DOTALL)
        except re.error as exc:
            failures.append(f"{description}: invalid matchStrings regex: {exc}: {raw_pattern}")
            continue

        matched_files = []
        for path in matching_files:
            try:
                text = path.read_text(encoding="utf-8")
            except UnicodeDecodeError as exc:
                failures.append(f"{description}: could not decode {path}: {exc}")
                continue
            if pin_regex.search(text):
                matched_files.append(path.as_posix())

        if not matched_files:
            failures.append(f"{description}: matchStrings pattern matched no intended files: {raw_pattern}")

if failures:
    for failure in failures:
        print(f"FAIL: {failure}")
    sys.exit(1)

print("OK: every Renovate custom manager matches at least one tracked pin")
PY

if ! grep -Fq '"currentValueTemplate": "master"' renovate.json; then
    echo "FAIL: ScoopInstaller/Install must track its live master branch" >&2
    exit 1
fi
