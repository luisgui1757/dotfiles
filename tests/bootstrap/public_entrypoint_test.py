"""Public wrappers launch only their checkout bootstrap and preserve its status."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from tests.bootstrap.posix_fixture import bash, shell_path

ROOT = Path(__file__).resolve().parents[2]


class PublicEntrypointTest(unittest.TestCase):
    def test_posix_launchers_bind_entrypoint_and_preserve_cwd_arguments_and_exit(self):
        for entrypoint in ["setup", "migrate"]:
            for arguments in [[], ["machine"]]:
                with self.subTest(entrypoint=entrypoint, arguments=arguments), tempfile.TemporaryDirectory() as temporary:
                    root = Path(temporary) / "trusted clone ü with spaces"
                    (root / "scripts").mkdir(parents=True)
                    shutil.copy2(ROOT / f"{entrypoint}.sh", root)
                    bootstrap = root / "scripts/installer-bootstrap.sh"
                    # Git Bash may name the native temp directory /tmp. Report
                    # its native identity instead of requiring one POSIX alias.
                    pwd = "$(pwd -W)" if os.name == "nt" else "$PWD"
                    bootstrap.write_text('#!/usr/bin/env bash\nprintf "%s\\n" "$DOTFILES_ENTRYPOINT" "' + pwd + '" "$#" "${1-}"\nexit 17\n', encoding="utf-8", newline="\n")
                    result = subprocess.run([bash(), shell_path(root / f"{entrypoint}.sh"), *arguments], cwd=temporary,
                                            env={**os.environ, "DOTFILES_ENTRYPOINT": "wrong-entrypoint"}, text=True, encoding="utf-8", capture_output=True)
                    self.assertEqual(result.returncode, 17, result.stderr)
                    actual_entrypoint, location, count, argument = result.stdout.splitlines()
                    self.assertEqual([actual_entrypoint, count, argument], [entrypoint, str(len(arguments)), arguments[0] if arguments else ""])
                    self.assertEqual(Path(location).resolve(), Path(temporary).resolve())

    @unittest.skipUnless(shutil.which("pwsh"), "PowerShell process-boundary test requires pwsh")
    def test_powershell_launchers_bind_entrypoint_and_preserve_arguments_and_exit(self):
        for entrypoint in ["setup", "migrate"]:
            for arguments in [[], ["machine"]]:
                with self.subTest(entrypoint=entrypoint, arguments=arguments), tempfile.TemporaryDirectory() as temporary:
                    root = Path(temporary) / "trusted clone ü with spaces"
                    (root / "scripts").mkdir(parents=True)
                    shutil.copy2(ROOT / f"{entrypoint}.ps1", root)
                    bootstrap = root / "scripts/installer-bootstrap.ps1"
                    bootstrap.write_text('[ordered]@{ Entrypoint = $env:DOTFILES_ENTRYPOINT; Arguments = @($args); Location = (Get-Location).Path } | ConvertTo-Json -Compress\nexit 17\n')
                    result = subprocess.run([shutil.which("pwsh"), "-NoProfile", "-File", str(root / f"{entrypoint}.ps1"), *arguments], cwd=temporary,
                                            env={**os.environ, "DOTFILES_ENTRYPOINT": "wrong-entrypoint"}, text=True, encoding="utf-8", capture_output=True)
                    self.assertEqual(result.returncode, 17, result.stderr)
                    self.assertEqual(json.loads(result.stdout), {"Entrypoint": entrypoint, "Arguments": arguments, "Location": str(Path(temporary).resolve())})


if __name__ == "__main__":
    unittest.main()
