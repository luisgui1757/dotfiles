#!/usr/bin/env bash
# A remote app uses a non-interactive login shell, not .zshrc.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
python3 - "$REPO_ROOT" <<'PY'
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

repo = Path(sys.argv[1])
zsh = shutil.which("zsh")
if not zsh:
    raise SystemExit("FAIL: zsh is required for the login-shell regression")
with tempfile.TemporaryDirectory(prefix="zsh login ") as directory:
    target = Path(directory)
    profile = repo / "shells/zprofile"
    if profile.exists():
        shutil.copyfile(profile, target / ".zprofile")
    (target / ".zshrc").write_text('print -u2 "FAIL: interactive configuration ran"\n')
    local_bin = target / ".local/bin"
    local_bin.mkdir(parents=True)
    tool = local_bin / "dotfiles-login-probe"
    tool.write_text("#!/bin/sh\nprintf '%s\\n' login-ready\n")
    tool.chmod(0o755)
    environment = {**os.environ, "HOME": str(target), "ZDOTDIR": str(target),
                   "PATH": "/usr/bin:/bin"}
    result = subprocess.run([zsh, "-d", "-lc", "dotfiles-login-probe"],
                            env=environment, text=True, capture_output=True)
    assert result.returncode == 0 and result.stdout == "login-ready\n" and not result.stderr, result
    # Loading the profile repeatedly must preserve one user-local entry.
    environment["PATH"] = str(local_bin) + ":/usr/bin:/bin"
    result = subprocess.run([zsh, "-d", "-lc", 'source "$HOME/.zprofile"; print -l -- $path'],
                            env=environment, text=True, capture_output=True, check=True)
    assert result.stdout.splitlines().count(str(local_bin)) == 1, result.stdout
    assert not result.stderr, result.stderr
    tool.unlink()
    local_bin.rmdir()
    environment["PATH"] = "/usr/bin:/bin"
    result = subprocess.run([zsh, "-d", "-lc", "print -r -- $PATH"],
                            env=environment, text=True, capture_output=True, check=True)
    assert str(local_bin) not in result.stdout and not result.stderr, result
print("PASS: login shell finds user-local commands without interactive startup; repeated and missing-directory cases pass")
PY
