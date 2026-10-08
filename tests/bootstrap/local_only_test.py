"""Piped setup cannot discover a checkout or bootstrap into the caller's home."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from tests.bootstrap.posix_fixture import bash, shell_path

ROOT = Path(__file__).resolve().parents[2]


class LocalOnlyTests(unittest.TestCase):
    def test_posix_piped_wrapper_fails_before_creating_state(self):
        with tempfile.TemporaryDirectory() as temporary:
            result = subprocess.run([bash(), "-s"], input=(ROOT / "setup.sh").read_text(),
                                    cwd=temporary, env={**os.environ, "HOME": shell_path(temporary)},
                                    capture_output=True, text=True, encoding="utf-8")
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(list(Path(temporary).iterdir()), [])

    @unittest.skipUnless(shutil.which("pwsh"), "requires PowerShell")
    def test_powershell_piped_wrapper_fails_before_creating_state(self):
        with tempfile.TemporaryDirectory() as temporary:
            environment = {**os.environ, "USERPROFILE": temporary, "HOME": temporary, "LOCALAPPDATA": temporary}
            # PowerShell itself creates module/cache directories on first startup.
            # Establish that runtime baseline before asserting wrapper side effects.
            subprocess.run([shutil.which("pwsh"), "-NoProfile", "-NonInteractive", "-Command", "exit 0"],
                           cwd=temporary, env=environment, capture_output=True, check=True)
            before = sorted(str(path.relative_to(temporary)) for path in Path(temporary).rglob("*"))
            result = subprocess.run([shutil.which("pwsh"), "-NoProfile", "-NonInteractive", "-Command",
                                     (ROOT / "setup.ps1").read_text()], cwd=temporary, env=environment,
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(sorted(str(path.relative_to(temporary)) for path in Path(temporary).rglob("*")), before)



if __name__ == "__main__":
    unittest.main()
