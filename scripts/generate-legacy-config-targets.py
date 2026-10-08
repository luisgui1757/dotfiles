#!/usr/bin/env python3
"""Regenerate bounded release-source inventory without running chezmoi/templates."""

import argparse
import hashlib
import json
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[1]
RELEASES = {
    "v0.1.0": "015617362830280bf85c7142e69d0681d376d453",
    "v0.4.4": "05874e536372f6a73f8971c84e675e95666662d4",
}
PLATFORMS = ["darwin", "linux", "windows"]
OVERLAYS = {
    "windows/chezmoi-localappdata/": "local_app_data",
    "windows/chezmoi-appdata/": "roaming_app_data",
    "windows/chezmoi-documents/": "documents",
}


def git(*args):
    return subprocess.check_output(["git", "-C", str(ROOT), *args])


def platforms(path):
    if path.startswith(("windows/", "home/AppData/", "home/Documents/")):
        return ["windows"]
    if path.startswith(("home/Library/", "home/dot_config/aerospace/")):
        return ["darwin"]
    if path.startswith(("home/dot_config/ghostty/", "home/dot_config/lazygit/")):
        return ["linux"]
    if path in ["home/dot_psmux.conf", "home/dot_tmux.windows.conf", "home/dot_tmux.rose-pine.ps1"]:
        return ["windows"]
    if path in ["home/dot_zshenv", "home/dot_zshrc", "home/dot_tmux.posix.conf", "home/dot_config/symlink_nvim.tmpl"] or path.startswith("home/dot_config/herdr/"):
        return ["darwin", "linux"]
    return PLATFORMS


def target(path, blob, data):
    root, relative = "home", path.removeprefix("home/")
    for prefix, folder in OVERLAYS.items():
        if path.startswith(prefix):
            root, relative = folder, path.removeprefix(prefix)
            break
    parts = []
    explicit_link = False
    scoped = False
    for part in relative.split("/"):
        if part.startswith("dot_"):
            part = "." + part[4:]
        if part.startswith("symlink_"):
            explicit_link = True
            part = part[8:]
        if part.startswith("modify_"):
            scoped = True
            part = part[7:]
        parts.append(part.removesuffix(".tmpl"))
    if scoped:
        parts[-1] = parts[-1].removesuffix(".ps1")
    result = {
        "root": root,
        "path": "/".join(parts),
        "platforms": platforms(path),
        "source": path,
        "blob": blob,
        "source_sha256": hashlib.sha256(data).hexdigest(),
        "form": "explicit-link" if explicit_link else "scoped-modification" if scoped else "rendered-file" if path.endswith(".tmpl") else "source-file",
    }
    if explicit_link:
        result["link_expression"] = data.decode("utf-8").strip()
    if "linux" in result["platforms"] and result["path"] in [".config/ghostty/config", ".config/wezterm/wezterm.lua"]:
        result["conditional"] = "Released WSL installs may omit this target unless experimentalWslGui was enabled. Presence still needs independent ownership evidence."
    return result


def generate():
    releases = []
    for tag, commit in RELEASES.items():
        if git("rev-parse", f"{tag}^{{commit}}").decode().strip() != commit:
            raise ValueError(f"{tag} no longer resolves to its reviewed release commit")
        record = {"tag": tag, "commit": commit, "metadata": [], "targets": []}
        for row in git("ls-tree", "-rz", "--full-tree", commit, "home", "windows").split(b"\0"):
            if not row:
                continue
            info, raw_path = row.split(b"\t", 1)
            mode, kind, raw_blob = info.decode().split()
            path, blob = raw_path.decode(), raw_blob
            if mode != "100644" or kind != "blob":
                raise ValueError(f"unreviewed source representation: {path}")
            data = git("show", f"{commit}:{path}")
            if path.startswith("home/.chezmoi") or path == "windows/chezmoi-overlay.toml":
                record["metadata"].append({"source": path, "blob": blob, "sha256": hashlib.sha256(data).hexdigest()})
            elif path.startswith("home/") or any(path.startswith(prefix) for prefix in OVERLAYS):
                record["targets"].append(target(path, blob, data))
            else:
                raise ValueError(f"unreviewed Windows source: {path}")
        releases.append(record)
    return {
        "schema": 1,
        "purpose": "Bounded released configuration discovery input; never deletion or adoption authority.",
        "coverage": "v0.1.0 and v0.4.4 source metadata only. Intermediate releases, script-created changes, package ownership, and complete migration remain separate proofs.",
        "semantics": {
            "source-file": "Released POSIX mode links to the recorded home source path; Windows copies source bytes.",
            "explicit-link": "The recorded template describes a link; it is evidence to interpret, never code to execute.",
            "rendered-file": "Rendered bytes require separately reviewed template inputs; source SHA256 is not the installed file hash.",
            "scoped-modification": "A scoped settings transformation, never ownership of the complete target file.",
            "roots": "Resolve Windows known folders independently; v0.1.0 home/AppData and home/Documents targets deliberately remain conventional home-relative paths.",
            "retention": "Keep legacy checkout source files and existing shared infrastructure until an approved, recoverable migration replaces their consumers.",
        },
        "releases": releases,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    output = ROOT / "installer/legacy-config-targets.json"
    rendered = json.dumps(generate(), indent=2, ensure_ascii=False) + "\n"
    if args.check:
        if output.read_text(encoding="utf-8") != rendered:
            raise SystemExit("legacy config inventory differs from reviewed release metadata; regenerate it")
    else:
        output.write_text(rendered, encoding="utf-8")


if __name__ == "__main__":
    main()
