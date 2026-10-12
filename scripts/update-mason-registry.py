#!/usr/bin/env python3
"""Reproduce the reviewed Mason snapshot after updating its release/hash lock."""

import hashlib
import io
import json
import pathlib
import re
import urllib.request
import zipfile


def main():
    directory = pathlib.Path(__file__).resolve().parents[1] / "nvim" / "mason-registry"
    lock = json.loads((directory / "lock.json").read_text(encoding="utf-8"))
    if lock["repository"] != "mason-org/mason-registry" or not re.fullmatch(
        r"[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z0-9-]+", lock["version"]
    ):
        raise ValueError("unexpected registry source")
    url = (
        "https://github.com/mason-org/mason-registry/releases/download/"
        + lock["version"] + "/registry.json.zip"
    )
    with urllib.request.urlopen(url, timeout=60) as response:
        archive = response.read(2 * 1024 * 1024 + 1)
    if hashlib.sha256(archive).hexdigest() != lock["archive_sha256"]:
        raise ValueError("registry archive differs from reviewed SHA-256")
    with zipfile.ZipFile(io.BytesIO(archive)) as source:
        info = source.getinfo("registry.json")
        if info.file_size > 2 * 1024 * 1024:
            raise ValueError("registry exceeds its size bound")
        registry = source.read(info)
    if hashlib.sha256(registry).hexdigest() != lock["registry_sha256"]:
        raise ValueError("registry contents differ from reviewed SHA-256")
    json.loads(registry)
    (directory / "registry.json").write_bytes(registry)
    print("Reproduced Mason registry " + lock["version"])


if __name__ == "__main__":
    main()
