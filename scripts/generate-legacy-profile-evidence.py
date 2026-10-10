#!/usr/bin/env python3
"""Freeze exact released profile bytes and the one script-created Bash hook."""

import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
RELEASES = {
    "v0.1.0": "015617362830280bf85c7142e69d0681d376d453",
    "v0.2.0": "22cfad80e904e003f52932ae6d6403520df00d3c",
    "v0.3.0": "c8507312153620b9b30fe2c84980c62bccb3b25a",
    "v0.4.0": "6317b375a0724804d7a8d895753364cc036e5658",
    "v0.4.1": "bac8cc97177b3bb58119fde5720b31e6b57febcc",
    "v0.4.2": "fdd628b34a58a3ecf3a1bef3de72f7cd4ac7dfc0",
    "v0.4.3": "e3e459a20c23ae546b26d5206d13b648b29e8788",
    "v0.4.4": "05874e536372f6a73f8971c84e675e95666662d4",
}
SOURCES = {
    "home/dot_zshrc": "zshrc",
    "home/dot_zshenv": "zshenv",
    "home/Documents/PowerShell/Microsoft.PowerShell_profile.ps1": "powershell",
    "shells/powershell_profile.ps1": "powershell",
}


def git(*args):
    return subprocess.check_output(["git", "-C", str(ROOT), *args])


def generate():
    profiles, hooks = [], []
    for tag, commit in RELEASES.items():
        if git("rev-parse", f"{tag}^{{commit}}").decode().strip() != commit:
            raise ValueError(f"{tag} differs from its reviewed commit")
        paths = set(git("ls-tree", "-rz", "--name-only", commit).decode().split("\0"))
        for source, kind in SOURCES.items():
            if source not in paths:
                continue
            data = git("show", f"{commit}:{source}")
            profiles.append({"tag": tag, "commit": commit, "kind": kind, "source": source,
                             "blob": git("rev-parse", f"{commit}:{source}").decode().strip(),
                             "sha256": hashlib.sha256(data).hexdigest()})
        source = git("show", f"{commit}:install-deps.sh").decode()
        start = "# >>> dotfiles: exec zsh (interactive bash fallback) >>>\n"
        end = "# <<< dotfiles: exec zsh (interactive bash fallback) <<<\n"
        # Match the emitted heredoc, not the marker variable in shell code.
        hook = start + source.split("\n" + start, 1)[1].split(end, 1)[0] + end
        hooks.append({"tag": tag, "commit": commit, "source": "install-deps.sh",
                      "sha256": hashlib.sha256(hook.encode()).hexdigest(), "content": hook})
    if len({item["content"] for item in hooks}) != 1:
        raise ValueError("released Bash hook changed; explicitly review the additional shape")
    return {"schema": 1, "profiles": profiles, "bash_hooks": hooks}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    target = ROOT / "installer/legacy-profile-evidence.json"
    rendered = json.dumps(generate(), indent=2) + "\n"
    if args.check:
        if target.read_text() != rendered:
            raise SystemExit("released profile evidence drifted; regenerate from the exact tags")
    else:
        target.write_text(rendered)


if __name__ == "__main__":
    main()
