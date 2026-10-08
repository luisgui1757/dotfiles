"""Run POSIX boundary fixtures in the host's actual Bash, including Git Bash."""

import os
from pathlib import Path
import shutil


def bash():
    if os.name == "nt":
        # Windows' system bash.exe is a WSL launcher, not the Git Bash runtime
        # used by hosted shell steps. Never start a guest to run these fixtures.
        git = shutil.which("git")
        if git:
            candidate = Path(git).resolve().parents[1] / "bin/bash.exe"
            if candidate.is_file():
                return str(candidate)
        raise RuntimeError("POSIX boundary fixtures on Windows require Git Bash")
    found = shutil.which("bash")
    if not found:
        raise RuntimeError("POSIX boundary fixtures require Bash")
    return found


def shell_path(path):
    path = Path(path).resolve()
    if os.name == "nt":
        if len(path.drive) != 2 or path.drive[1] != ":":
            raise ValueError("Git Bash fixtures require a local drive path")
        return "/" + path.drive[0].lower() + path.as_posix()[2:]
    return str(path)
