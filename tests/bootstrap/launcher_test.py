"""Process-boundary launcher regressions; no downloads or user-home writes."""

import hashlib
import csv
import os
from pathlib import Path
import shutil
import shlex
import subprocess
import tarfile
import tempfile
import unittest

from tests.bootstrap.posix_fixture import bash, shell_path

ROOT = Path(__file__).resolve().parents[2]
GO = r'''#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$1" >> "$FIXTURE/log"
[[ $GOENV == off && $GOWORK == off && $GOTOOLCHAIN == local && $CGO_ENABLED == 0 ]]
[[ $GOPATH == "$XDG_CACHE_HOME/"* && $GOCACHE == "$XDG_CACHE_HOME/"* ]]
if [[ ${CHECK_MODULE_CACHE:-0} == 1 ]]; then
  mkdir -p "$GOMODCACHE"
  printf 'compiler module write\n' > "$GOMODCACHE/compiler-probe"
  [[ $GOMODCACHE == "$GOPATH/pkg/mod" ]] || { echo 'compiler used inherited module cache' >&2; exit 44; }
fi
case "$1" in
  version) printf 'go version %s linux/amd64\n' "${FAKE_VERSION:-go1.27.1}" ;;
  mod) echo 'all modules verified' ;;
  list) printf '%s\n' "$PWD/main.go" "$PWD/embedded.json" ;;
  build)
    [[ ${FAIL_BUILD:-0} == 0 ]] || exit 42
    [[ ${CHANGE_SOURCE:-0} == 0 ]] || echo '// changed during build' >> main.go
    while [[ $1 != -o ]]; do shift; done
    cp "$FIXTURE/application" "$2"
    chmod +x "$2"
    ;;
  *) exit 43 ;;
esac
'''
APP = r'''#!/usr/bin/env bash
set -euo pipefail
[[ ${GOENV:-} == personal-setting ]]
if [[ ${CHECK_MODULE_CACHE:-0} == 1 ]]; then printf '%s\n' "$GOMODCACHE" > "$FIXTURE/application-module-cache"; fi
printf '%s\n' "$#" "$DOTFILES_CHECKOUT" > "$FIXTURE/application-result"
if [[ $# == 1 ]]; then printf '%s\n' "$1" >> "$FIXTURE/application-result"; cat; fi
exit "${APPLICATION_EXIT:-0}"
'''
CURL = r'''#!/usr/bin/env bash
set -euo pipefail
printf 'download\n' >> "$FIXTURE/log"
while [[ $1 != --output ]]; do shift; done
cp "$FIXTURE/go.tar.gz" "$2"
'''


class LauncherTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="dotfiles bootstrap ü ' ")
        self.root = Path(self.temporary.name)
        self.checkout = self.root / "trusted clone á"
        (self.checkout / "scripts").mkdir(parents=True)
        (self.checkout / "installer").mkdir()
        shutil.copy2(ROOT / "scripts/installer-bootstrap.sh", self.checkout / "scripts")
        shutil.copy2(ROOT / "setup.sh", self.checkout)
        self.write("installer/go.mod", "module fixture\n\ngo 1.27.1\n")
        self.write("installer/go.sum", "")
        self.write("installer/main.go", "package main\n")
        self.write("installer/embedded.json", "{}\n")
        (self.root / "bin").mkdir()
        self.executable(self.root / "bin/curl", CURL)
        self.executable(self.root / "bin/uname", '#!/bin/sh\ncase "$1" in -s) echo "${FAKE_OS:-Linux}";; -m) echo "${FAKE_ARCH:-x86_64}";; esac\n')
        self.executable(self.root / "go/bin/go", GO)
        self.executable(self.root / "application", APP)
        with tarfile.open(self.root / "go.tar.gz", "w:gz") as archive:
            def executable_archive(entry):
                # Windows chmod cannot express Unix execute bits. The fixture
                # represents the Linux archive's real executable mode.
                if entry.isdir() or entry.name == "go/bin/go":
                    entry.mode = 0o755
                return entry
            archive.add(self.root / "go", arcname="go", filter=executable_archive)
        data = (self.root / "go.tar.gz").read_bytes()
        self.digest = hashlib.sha256(data).hexdigest()
        self.pin = "version\tplatform\tfilename\tsha256\tsize\n" + (
            f"go1.27.1\tlinux-amd64\tgo1.27.1.linux-amd64.tar.gz\t{self.digest}\t{len(data)}\n"
        )
        self.write("installer/bootstrap-toolchain.tsv", self.pin)
        self.environment = dict(os.environ, HOME=shell_path(self.root / "home"),
                                XDG_CACHE_HOME=shell_path(self.root / "cache"),
                                FIXTURE=shell_path(self.root), GOENV="personal-setting")

    def tearDown(self):
        self.temporary.cleanup()

    def write(self, relative, text):
        (self.checkout / relative).write_text(text, encoding="utf-8", newline="\n")

    @staticmethod
    def executable(path, text):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8", newline="\n")
        path.chmod(0o755)

    def launch(self, *args, success=True, shell=None, entrypoint=None, **environment):
        # Git/bin/bash.exe prepends its own tools. Install boundary doubles only
        # after that wrapper initializes the real Bash environment.
        result = subprocess.run([*(shell or [bash()]), "-c", 'umask "${CALLER_UMASK:-0022}"; export PATH="$1:$PATH"; shift; exec bash "$@"',
                                 "bootstrap-fixture", shell_path(self.root / "bin"),
                                 shell_path(self.checkout / (f"{entrypoint}.sh" if entrypoint else "scripts/installer-bootstrap.sh")), *args],
                                env=dict(self.environment, **environment), input='{"operation":"check"}\n',
                                capture_output=True, text=True, encoding="utf-8", cwd=self.root)
        if success:
            self.assertEqual(result.returncode, 0, result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def events(self):
        return (self.root / "log").read_text().splitlines()

    def test_menu_machine_arguments_source_cache_and_environment(self):
        self.launch()
        self.assertEqual((self.root / "application-result").read_text(encoding="utf-8").splitlines(),
                         ["0", shell_path(self.checkout)])
        result = self.launch("machine")
        self.assertEqual(result.stdout, '{"operation":"check"}\n')
        self.assertEqual(self.events().count("download"), 1)
        self.assertEqual(self.events().count("build"), 1)
        self.write("installer/irrelevant_test.go", "package main // test edits\n")
        self.write("installer/scratch.tmp", "not a build input")
        self.launch()
        self.assertEqual(self.events().count("build"), 1)
        self.write("installer/main.go", "package main // changed\n")
        self.launch()
        self.assertEqual(self.events().count("build"), 2)
        self.write("installer/embedded.json", '{"changed": true}\n')
        self.launch()
        self.assertEqual(self.events().count("build"), 3)

    def test_compiler_module_cache_is_private_and_application_keeps_caller_environment(self):
        personal = self.root / "personal Go cache"
        personal.mkdir()
        (personal / "preserve").write_text("personal module bytes")
        self.launch("machine", entrypoint="setup", CHECK_MODULE_CACHE="1", GOMODCACHE=shell_path(personal))
        self.assertEqual((self.root / "application-module-cache").read_text(encoding="utf-8").strip(), shell_path(personal))
        self.assertEqual(sorted(p.name for p in personal.iterdir()), ["preserve"])
        self.assertEqual((personal / "preserve").read_text(), "personal module bytes")
        self.assertEqual((self.root / "cache/dotfiles/bootstrap/gopath/pkg/mod/compiler-probe").read_text(), "compiler module write\n")
        self.assertEqual(self.events().count("download"), 1)
        self.assertEqual(self.events().count("build"), 1)

    def test_public_launchers_restore_caller_umask_but_keep_bootstrap_private(self):
        self.executable(self.root / "application", APP.replace(
            'exit "${APPLICATION_EXIT:-0}"',
            'umask > "$FIXTURE/engine-umask"\n'
            ': > "$FIXTURE/engine-file"\n'
            'mkdir "$FIXTURE/engine-directory"\n'
            'exit "${APPLICATION_EXIT:-0}"'))
        # Check every compiler invocation, including cache validation on warm runs.
        self.executable(self.root / "go/bin/go", GO.replace(
            'set -euo pipefail', 'set -euo pipefail\n[[ $(umask) == 0077 ]]'))
        with tarfile.open(self.root / "go.tar.gz", "w:gz") as archive:
            def executable_archive(entry):
                if entry.isdir() or entry.name == "go/bin/go":
                    entry.mode = 0o755
                return entry
            archive.add(self.root / "go", arcname="go", filter=executable_archive)
        data = (self.root / "go.tar.gz").read_bytes()
        self.write("installer/bootstrap-toolchain.tsv", self.pin.replace(
            f"{self.digest}\t", f"{hashlib.sha256(data).hexdigest()}\t").rsplit("\t", 1)[0] + f"\t{len(data)}\n")
        for mask in ["0022", "0002", "0027"]:
            with self.subTest(mask=mask):
                (self.root / "engine-file").unlink(missing_ok=True)
                if (self.root / "engine-directory").exists():
                    (self.root / "engine-directory").rmdir()
                self.launch(*([] if mask == "0022" else ["machine"]), entrypoint="setup", CALLER_UMASK=mask)
                self.assertEqual((self.root / "engine-umask").read_text().strip(), mask)
                if os.name != "nt":
                    self.assertEqual((self.root / "engine-file").stat().st_mode & 0o777, 0o666 & ~int(mask, 8))
                    self.assertEqual((self.root / "engine-directory").stat().st_mode & 0o777, 0o777 & ~int(mask, 8))
                    self.assertEqual((self.root / "cache/dotfiles/bootstrap").stat().st_mode & 0o777, 0o700)
                (self.root / "engine-file").unlink()
                (self.root / "engine-directory").rmdir()
        self.assertEqual(self.events().count("download"), 1)
        self.assertEqual(self.events().count("build"), 1)

    def test_shell_wrapper_cannot_shadow_fixture_commands(self):
        system_tools = self.root / "wrapper tools"
        self.executable(system_tools / "uname", '#!/bin/sh\necho unsupported-wrapper-platform\n')
        wrapper = self.root / "shell wrapper"
        self.executable(wrapper, '#!/usr/bin/env bash\nexport PATH=' + shlex.quote(shell_path(system_tools))
                        + ':"$PATH"\nexec ' + shlex.quote(shell_path(bash())) + ' "$@"\n')
        self.launch(shell=[bash(), shell_path(wrapper)])
        self.assertEqual(self.events().count("download"), 1)
        self.assertTrue((self.root / "application-result").is_file())

    def test_legacy_flags_rejected_before_cache_mutation(self):
        self.launch("--all", success=False)
        self.assertFalse((self.root / "cache").exists())

    def test_intel_macos_and_unknown_platform_rejected_before_download(self):
        for operating_system, arch in [("Darwin", "x86_64"), ("FreeBSD", "amd64")]:
            with self.subTest(operating_system=operating_system):
                self.launch(success=False, FAKE_OS=operating_system, FAKE_ARCH=arch)
                self.assertFalse((self.root / "cache").exists())

    def test_duplicate_target_and_go_version_drift_fail_before_download(self):
        self.write("installer/bootstrap-toolchain.tsv", self.pin + self.pin.splitlines()[-1] + "\n")
        self.launch(success=False)
        self.assertFalse((self.root / "cache").exists())
        self.write("installer/bootstrap-toolchain.tsv", self.pin)
        self.write("installer/go.mod", "module fixture\n\ngo 1.27.2\n")
        self.launch(success=False)
        self.assertFalse((self.root / "cache").exists())

    def test_bad_download_never_extracts_or_executes(self):
        self.write("installer/bootstrap-toolchain.tsv", self.pin.replace(self.digest, "0" * 64))
        result = self.launch(success=False)
        self.assertIn("checksum or size mismatch", result.stderr)
        self.assertEqual(self.events(), ["download"])
        self.assertEqual(list((self.root / "cache/dotfiles/bootstrap").glob("toolchain-*")), [])

    def test_modified_cached_archive_rejected_before_compiler_execution(self):
        self.launch()
        before = self.events()
        archive = self.root / "cache/dotfiles/bootstrap/go1.27.1.linux-amd64.tar.gz"
        archive.write_bytes(b"damaged")
        # Any checkout change needs the compiler, which needs the archive.
        self.write("installer/notes.md", "changed\n")
        self.launch(success=False)
        self.assertEqual(self.events(), before)

    def test_failed_build_and_source_change_never_publish_binary(self):
        for setting in ["FAIL_BUILD", "CHANGE_SOURCE"]:
            with self.subTest(setting=setting):
                self.launch(success=False, **{setting: "1"})
                self.assertEqual(list((self.root / "cache/dotfiles/bootstrap").glob("dotfiles-*")), [])
                self.assertFalse((self.root / "application-result").exists())

    def test_cached_binary_corruption_fails_without_running_it(self):
        self.launch()
        result = self.root / "application-result"
        result.unlink()
        binaries = [p for p in (self.root / "cache/dotfiles/bootstrap").glob("dotfiles-*")
                    if not p.name.endswith(".sha256")]
        binaries[0].write_text("#!/bin/sh\nexit 0\n")
        self.launch(success=False)
        self.assertFalse(result.exists())

    def test_wrong_cached_compiler_version_rejected_even_on_binary_cache_hit(self):
        self.launch()
        (self.root / "application-result").unlink()
        # A non-input change keeps the binary identity but requires the compiler.
        self.write("installer/irrelevant_test.go", "package main\n")
        self.launch(success=False, FAKE_VERSION="go1.27.2")
        self.assertFalse((self.root / "application-result").exists())

    def test_unchanged_checkout_reuses_verified_build_without_toolchain(self):
        self.launch()
        (self.root / "log").unlink()
        (self.root / "cache/dotfiles/bootstrap/go1.27.1.linux-amd64.tar.gz").unlink()
        result = self.launch("machine")
        self.assertEqual(result.stdout, '{"operation":"check"}\n')
        self.assertFalse((self.root / "log").exists(), "unchanged checkout extracted or ran the toolchain")
        self.assertEqual((self.root / "application-result").read_text(encoding="utf-8").splitlines()[:2], ["1", shell_path(self.checkout)])

    def test_any_installer_change_or_link_takes_the_verified_path(self):
        self.launch()
        for change in ["file", "nested", "link", "script"]:
            with self.subTest(change=change):
                (self.root / "log").unlink()
                if change == "file":
                    self.write("installer/README.md", "documentation only\n")
                elif change == "nested":
                    (self.checkout / "installer/testdata").mkdir(exist_ok=True)
                    self.write("installer/testdata/fixture.json", "{}\n")
                elif change == "link":
                    (self.checkout / "installer/link").symlink_to(self.checkout / "installer/main.go")
                else:
                    with (self.checkout / "scripts/installer-bootstrap.sh").open("a", encoding="utf-8", newline="\n") as script:
                        script.write("# changed launcher\n")
                self.launch()
                self.assertIn("version", self.events())
                if change == "link":
                    self.launch()
                    self.assertGreaterEqual(self.events().count("version"), 2, "a link enabled reuse")
        self.assertEqual(self.events().count("build"), 1, "the changed launcher alone rebuilds")

    def test_fast_path_still_refuses_a_damaged_binary(self):
        self.launch()
        (self.root / "application-result").unlink()
        (self.root / "log").unlink()
        binaries = [p for p in (self.root / "cache/dotfiles/bootstrap").glob("dotfiles-*") if not p.name.endswith(".sha256")]
        binaries[0].write_text("#!/bin/sh\nexit 0\n")
        result = self.launch(success=False)
        self.assertIn("cached installer is damaged", result.stderr)
        self.assertFalse((self.root / "application-result").exists())
        self.assertFalse((self.root / "log").exists())

    def test_build_that_saw_a_changing_checkout_records_no_reuse(self):
        self.launch(success=False, CHANGE_SOURCE="1")
        self.assertEqual(list((self.root / "cache/dotfiles/bootstrap").glob("checkout-*")), [])

    def test_modified_extracted_compiler_is_never_executed(self):
        self.launch()
        extracted = self.root / f"cache/dotfiles/bootstrap/toolchain-{self.digest}/go/bin/go"
        marker = self.root / "unverified-compiler-ran"
        self.executable(extracted, '#!/bin/sh\ntouch "$FIXTURE/unverified-compiler-ran"\nexit 42\n')
        self.launch()
        self.assertFalse(marker.exists())

    def test_contended_lock_is_preserved(self):
        lock = self.root / "cache/dotfiles/bootstrap/lock"
        lock.mkdir(parents=True)
        self.launch(success=False)
        self.assertTrue(lock.is_dir())
        self.assertFalse((self.root / "log").exists())

    def test_installer_exit_status_is_preserved(self):
        result = self.launch(success=False, APPLICATION_EXIT="23")
        self.assertEqual(result.returncode, 23)

    def test_production_pins_match_module_and_supported_targets(self):
        lines = (ROOT / "installer/bootstrap-toolchain.tsv").read_text().splitlines()
        pins = list(csv.DictReader((line for line in lines if not line.startswith("#")), delimiter="\t"))
        self.assertEqual({row["platform"] for row in pins},
                         {"darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"})
        self.assertEqual(len(pins), 4)
        module = (ROOT / "installer/go.mod").read_text()
        version = next(line.split()[1] for line in module.splitlines() if line.startswith("go "))
        self.assertEqual({row["version"] for row in pins}, {"go" + version})
        for pin in pins:
            self.assertRegex(pin["sha256"], r"^[a-f0-9]{64}$")
            self.assertGreater(int(pin["size"]), 0)


if __name__ == "__main__":
    unittest.main()
