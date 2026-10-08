"""Missing Apple payload preparation preserves receipts and rejects foreign paths."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from tests.bootstrap.posix_fixture import bash, shell_path

ROOT = Path(__file__).resolve().parents[2]


class AppleReceiptFixtureTest(unittest.TestCase):
    def run_receipts(self, receipts, files="Library/Developer/CommandLineTools/usr/bin/clang\n", query_status=0, files_status=0, info_status=0, location="/", directories=".\nLibrary\nLibrary/Developer\nLibrary/Developer/CommandLineTools\n"):
        workflow = (ROOT / ".github/workflows/installer-engine.yml").read_text()
        start = workflow.index('          receipts="$(pkgutil')
        end = workflow.index('          # Historical receipts remain unchanged;', start)
        block = "\n".join(line[10:] for line in workflow[start:end].splitlines())
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            (base / "input").write_text(receipts, encoding="utf-8", newline="\n")
            (base / "members").write_text(directories + files, encoding="utf-8", newline="\n")
            (base / "files").write_text(files, encoding="utf-8", newline="\n")
            pkgutil = base / "pkgutil"
            pkgutil.write_text(r'''#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/calls"
case "$1" in
  --pkgs) cat "$FIXTURE/input"; exit "$QUERY_STATUS" ;;
  --pkg-info) printf 'package-id: %s\nvolume: /\nlocation: %s\n' "$2" "$LOCATION"; exit "$INFO_STATUS" ;;
  --files) cat "$FIXTURE/members"; exit "$FILES_STATUS" ;;
  --only-files) test "$#" = 3; test "$2" = --files; cat "$FIXTURE/files"; exit "$FILES_STATUS" ;;
  *) exit 99 ;;
esac
''', encoding="utf-8", newline="\n")
            pkgutil.chmod(0o755)
            result = subprocess.run([bash(), "-c", "set -euo pipefail\n" + block], text=True, encoding="utf-8", capture_output=True,
                                    env={**os.environ, "PATH": str(base) + os.pathsep + os.environ["PATH"], "FIXTURE": shell_path(base), "QUERY_STATUS": str(query_status), "FILES_STATUS": str(files_status), "INFO_STATUS": str(info_status), "LOCATION": location})
            self.assertEqual((base / "input").read_text(), receipts)
            return result, (base / "calls").read_text()

    def test_empty_receipts_are_valid(self):
        result, calls = self.run_receipts("")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, "--pkgs\n")

    def test_historical_receipts_are_recorded_read_only(self):
        result, calls = self.run_receipts("com.apple.pkg.CLTools_Executables\ncom.apple.pkg.Xcode\nother.com.apple.pkg.CLTools\ncom.apple.pkg.CLTools_SDK_macOS\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls.splitlines(), ["--pkgs", "--pkg-info com.apple.pkg.CLTools_Executables", "--only-files --files com.apple.pkg.CLTools_Executables", "--pkg-info com.apple.pkg.CLTools_SDK_macOS", "--only-files --files com.apple.pkg.CLTools_SDK_macOS"])
        self.assertIn("package-id: com.apple.pkg.CLTools_Executables", result.stdout)

    def test_receipt_directories_do_not_claim_outside_payload_files(self):
        result, _ = self.run_receipts("com.apple.pkg.CLTools_SDK_macOS13\n", directories=".\nLibrary\nLibrary/Developer\nLibrary/Developer/CommandLineTools\nprivate\nprivate/tmp\n")
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_directory_only_receipt_has_no_payload_files(self):
        result, _ = self.run_receipts("com.apple.pkg.CLTools_SDK_macOS13\n", files="", directories="private\nprivate/tmp\n")
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_outside_or_noncanonical_payload_member_is_rejected(self):
        for path in ("private/tmp/payload", "Applications/Xcode.app", "Library/Developer/OtherTool/bin", "Library/Developer/CommandLineTools/../OtherTool", "Library/Developer/CommandLineTools//file", "/Library/Developer/CommandLineTools/file"):
            with self.subTest(path=path):
                result, _ = self.run_receipts("com.apple.pkg.CLTools_Executables\n", path + "\n")
                self.assertNotEqual(result.returncode, 0)

    def test_all_invalid_files_are_reported_with_bounded_output(self):
        paths = ["private/tmp/" + str(i) + "x" * 300 for i in range(25)]
        result, _ = self.run_receipts("com.apple.pkg.CLTools_SDK_macOS13\n", files="\n".join(paths) + "\n")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(paths[0][:256], result.stderr)
        self.assertIn(paths[19][:256], result.stderr)
        self.assertNotIn(paths[20][:256], result.stderr)
        self.assertIn("5 additional invalid payload files", result.stderr)
        self.assertLess(len(result.stderr.encode()), 7000)

    def test_receipts_are_checked_before_any_payload_move(self):
        workflow = (ROOT / ".github/workflows/installer-engine.yml").read_text()
        self.assertNotIn("pkgutil --forget", workflow)
        self.assertLess(workflow.index("# Historical receipts remain unchanged;"), workflow.index('sudo mv "$application" "$baseline/"'))
        self.assertLess(workflow.index("TestNativeAppleCLTPreinstalledReuse$"), workflow.index('receipts="$(pkgutil'))

    def test_package_and_payload_query_failures_remain_fatal(self):
        for query, files, info in ((2, 0, 0), (0, 3, 0), (0, 0, 4)):
            result, _ = self.run_receipts("com.apple.pkg.CLTools_Executables\n", query_status=query, files_status=files, info_status=info)
            self.assertNotEqual(result.returncode, 0)

    def test_nonroot_receipt_location_is_rejected(self):
        result, _ = self.run_receipts("com.apple.pkg.CLTools_Executables\n", location="/outside")
        self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
