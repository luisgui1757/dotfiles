#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
python3 - <<'PY'
import json, pathlib, re
pins=json.loads(pathlib.Path("installer/archive-pins.json").read_text())["resources"]
workflow=pathlib.Path(".github/workflows/test.yml").read_text()
def value(key):
    matches=re.findall(r"^  "+key+r": (\S+)$",workflow,re.M)
    assert len(matches)==1, key
    return matches[0]
for resource, version, digest in (
    ("tool.nvim", "NVIM_LINUX_VERSION", "NVIM_LINUX_X86_64_SHA256"),
    ("tool.tree-sitter", "TREE_SITTER_CLI_LINUX_VERSION", "TREE_SITTER_CLI_LINUX_X86_64_SHA256"),
):
    pin=pins[resource]["linux/amd64"]
    assert pin["version"].lstrip("v")==value(version).lstrip("v"), resource+" version drift"
    if resource == "tool.nvim":
        assert pin["sha256"]==value(digest), resource+" checksum drift"
# CI tree-sitter is ZIP; installer gzip. CI Starship is glibc; installer musl. Versions must agree,
# checksums intentionally differ because they identify different upstream bytes.
assert pins["tool.starship"]["linux/amd64"]["version"]==value("STARSHIP_VERSION").lstrip("v")
go_version=re.search(r"(?m)^go (\S+)$",pathlib.Path("installer/go.mod").read_text()).group(1)
for path in pathlib.Path(".github/workflows").glob("*.yml"):
    for pinned in re.findall(r"(?m)^ +go-version: (\S+)$",path.read_text()):
        assert pinned == go_version, str(path)+" Go toolchain drift"
print("OK: active CI and installer pins agree")
PY
