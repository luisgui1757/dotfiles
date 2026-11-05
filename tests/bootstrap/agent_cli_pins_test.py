"""Offline tests for scripts/update-agent-cli-pins.py (no network, no gpg)."""

import base64
import copy
import gzip
import hashlib
import http.client
import importlib.util
import io
import json
import pathlib
import subprocess
import sys
import tarfile
import tempfile
import unittest
import urllib.parse
import zipfile
from unittest import mock

sys.dont_write_bytecode = True
ROOT = pathlib.Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("update_agent_cli_pins", ROOT / "scripts" / "update-agent-cli-pins.py")
pins_script = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(pins_script)


def tgz(files):
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as archive:
        for name, data in files.items():
            info = tarfile.TarInfo(name)
            info.size, info.mode = len(data), 0o755
            archive.addfile(info, io.BytesIO(data))
    return buffer.getvalue()


def zipped(files):
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as archive:
        for name, data in files.items():
            archive.writestr(name, data)
    return buffer.getvalue()


def sri(data):
    return "sha512-" + base64.b64encode(hashlib.sha512(data).digest()).decode()


def layout(pin):
    return set(pin.get("commands", {}).values()) | set(pin.get("required_files", [])) | set(pin.get("excluded_files", []))


class FakeNetwork:
    def __init__(self):
        self.texts, self.files = {}, {}

    def text(self, url):
        return self.texts[url]

    def json(self, url):
        return json.loads(self.texts[url])

    def download(self, url, destination):
        data = self.files[url]
        if isinstance(data, Exception):
            raise data
        destination.write_bytes(data)
        return {"sha256": hashlib.sha256(data).hexdigest(), "sha512": hashlib.sha512(data).digest(), "size": len(data)}


class AgentCLIPinBumpTest(unittest.TestCase):
    versions = {"tool.claude": "2.1.400", "tool.codex": "0.200.0", "tool.copilot": "1.1.0", "tool.kiro": "2.40.0"}

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.pins_path = pathlib.Path(self.temporary.name) / "archive-pins.json"
        self.original = (ROOT / "installer" / "archive-pins.json").read_text(encoding="utf-8")
        self.pins_path.write_text(self.original, encoding="utf-8")
        self.pins = json.loads(self.original)["resources"]
        self.network = FakeNetwork()
        self.verified = []
        self.publish_claude()
        self.publish_codex()
        self.publish_copilot()
        self.publish_kiro()

    def verify(self, manifest, signature, key):
        self.verified.append((manifest, signature, key))

    def publish_claude(self, version=None):
        version = version or self.versions["tool.claude"]
        base = pins_script.CLAUDE_RELEASES
        net, platforms = self.network, {}
        net.texts[f"{base}/stable"] = (version + "\n").encode()
        for target, platform in pins_script.CLAUDE_PLATFORMS.items():
            binary = f"claude {platform} {version}".encode()
            name = self.pins["tool.claude"][target]["commands"]["claude"]
            archive = tgz({"package/" + name: binary, "package/package.json": b"{}"})
            package = f"@anthropic-ai/claude-code-{platform}"
            url = f"https://registry.npmjs.org/{package}/-/claude-code-{platform}-{version}.tgz"
            net.files[url] = archive
            net.texts[f"https://registry.npmjs.org/{urllib.parse.quote(package, safe='@')}/{version}"] = json.dumps({"dist": {"tarball": url, "integrity": sri(archive)}}).encode()
            platforms[platform] = {"checksum": hashlib.sha256(binary).hexdigest()}
        net.texts[f"{base}/{version}/manifest.json"] = json.dumps({"version": version, "platforms": platforms}).encode()
        net.texts[f"{base}/{version}/manifest.json.sig"] = b"signature"
        net.texts[pins_script.CLAUDE_KEY] = b"key"

    def publish_codex(self, latest=None):
        version = latest or self.versions["tool.codex"]
        tags, releases = {"latest": version}, {}
        for target, platform in pins_script.CODEX_PLATFORMS.items():
            release = f"{version}-{platform}"
            archive = tgz({"package/" + name: b"x" for name in layout(self.pins["tool.codex"][target])})
            url = f"https://registry.npmjs.org/@openai/codex/-/codex-{release}.tgz"
            self.network.files[url] = archive
            tags[platform] = release
            releases[release] = {"dist": {"tarball": url, "integrity": sri(archive)}}
        self.network.texts["https://registry.npmjs.org/%40openai%2Fcodex"] = json.dumps({"dist-tags": tags, "versions": releases}).encode()

    def publish_copilot(self):
        version = self.versions["tool.copilot"]
        assets, sums = [], []
        for target, name in pins_script.COPILOT_ASSETS.items():
            pin = self.pins["tool.copilot"][target]
            files = {entry: b"copilot" for entry in layout(pin)}
            data = zipped(files) if pin["format"] == "zip" else tgz(files)
            url = f"https://github.com/github/copilot-cli/releases/download/v{version}/{name}"
            self.network.files[url] = data
            digest = hashlib.sha256(data).hexdigest()
            assets.append({"name": name, "browser_download_url": url, "digest": "sha256:" + digest})
            sums.append(f"{digest}  {name}")
        sums_url = f"https://github.com/github/copilot-cli/releases/download/v{version}/SHA256SUMS.txt"
        assets.append({"name": "SHA256SUMS.txt", "browser_download_url": sums_url})
        self.network.texts[sums_url] = "\n".join(sums).encode()
        self.network.texts["https://api.github.com/repos/github/copilot-cli/releases/latest"] = json.dumps({"tag_name": "v" + version, "prerelease": False, "assets": assets}).encode()

    def publish_kiro(self):
        version = self.versions["tool.kiro"]
        packages = []
        for target, name in pins_script.KIRO_FILES.items():
            pin = self.pins["tool.kiro"][target]
            data = zipped({"kirocli/" + entry: b"kiro" for entry in layout(pin)}) if pin["format"] == "zip" else f"{name} bytes".encode()
            self.network.files[pins_script.KIRO_BASE + urllib.parse.quote(f"{version}/{name}")] = data
            packages.append({"download": f"{version}/{name}", "sha256": hashlib.sha256(data).hexdigest(), "size": len(data)})
        self.network.texts[pins_script.KIRO_BASE + "latest/manifest.json"] = json.dumps({"version": version, "packages": packages}).encode()

    def run_bump(self, tools=None, expected_status=0):
        summary = pathlib.Path(self.temporary.name) / "summary.md"
        failures = pathlib.Path(self.temporary.name) / "failures.txt"
        argv = ["--pins", str(self.pins_path), "--summary", str(summary), "--failures", str(failures)]
        argv += ["--tools", tools or ",".join(self.versions)]
        with mock.patch("builtins.print"):
            self.assertEqual(pins_script.main(argv, network=self.network, verify=self.verify), expected_status)
        return json.loads(self.pins_path.read_text(encoding="utf-8"))["resources"], summary.read_text(encoding="utf-8"), failures.read_text(encoding="utf-8")

    def test_bumps_every_tool_after_its_vendor_verification(self):
        resources, summary, failures = self.run_bump()
        self.assertEqual(failures, "")
        self.assertEqual(len(self.verified), 1)
        for tool, version in self.versions.items():
            for target, pin in resources[tool].items():
                before = self.pins[tool][target]
                self.assertEqual(pin["version"], version, (tool, target))
                self.assertIn(version, urllib.parse.unquote(pin["url"]))
                data = self.network.files[pin["url"]]
                self.assertEqual(pin["sha256"], hashlib.sha256(data).hexdigest())
                unchanged = {key: value for key, value in pin.items() if key not in ("version", "url", "sha256")}
                self.assertEqual(unchanged, {key: value for key, value in before.items() if key not in ("version", "url", "sha256")})
            self.assertIn(f"`{tool}`:", summary)
        self.assertEqual(resources["tool.kiro"]["darwin/arm64"]["url"], pins_script.KIRO_BASE + "2.40.0/Kiro%20CLI.dmg")

    def test_current_versions_leave_the_pins_byte_identical(self):
        self.versions = {tool: sorted({pin["version"] for pin in self.pins[tool].values()})[0] for tool in self.versions}
        self.network = FakeNetwork()
        self.publish_claude()
        self.publish_codex()
        self.publish_copilot()
        self.publish_kiro()
        _, summary, failures = self.run_bump()
        self.assertEqual(failures, "")
        self.assertEqual(self.pins_path.read_text(encoding="utf-8"), self.original)
        self.assertEqual(summary.count("is current"), 4)

    def assertOnlyToolFailed(self, tool, message):
        resources, _, failures = self.run_bump(expected_status=1)
        self.assertIn(f"`{tool}`", failures)
        self.assertIn(message, failures)
        self.assertEqual(resources[tool], self.pins[tool])
        for other in set(self.versions) - {tool}:
            self.assertEqual({pin["version"] for pin in resources[other].values()}, {self.versions[other]})

    def test_claude_needs_a_valid_signed_manifest(self):
        def reject(*_):
            raise pins_script.BumpError("Claude Code release manifest signature is not valid")
        self.verify = reject
        self.assertOnlyToolFailed("tool.claude", "signature is not valid")

    def test_claude_binary_must_match_the_signed_manifest(self):
        base = pins_script.CLAUDE_RELEASES
        manifest = json.loads(self.network.texts[f"{base}/2.1.400/manifest.json"])
        manifest["platforms"]["linux-x64"]["checksum"] = "0" * 64
        self.network.texts[f"{base}/2.1.400/manifest.json"] = json.dumps(manifest).encode()
        self.assertOnlyToolFailed("tool.claude", "does not match the signed manifest")

    def test_npm_packages_must_match_their_published_integrity(self):
        url = "https://registry.npmjs.org/@openai/codex/-/codex-0.200.0-win32-x64.tgz"
        self.network.files[url] += b"tampered"
        self.assertOnlyToolFailed("tool.codex", "SHA-512 integrity")

    def test_prereleases_are_never_pinned(self):
        self.publish_codex("0.201.0-alpha.3")
        self.assertOnlyToolFailed("tool.codex", "not a plain release version")

    def test_codex_layout_changes_need_manual_review(self):
        target = "linux/amd64"
        pin = self.pins["tool.codex"][target]
        archive = tgz({"package/" + name: b"x" for name in layout(pin) if not name.endswith("bwrap")})
        url = "https://registry.npmjs.org/@openai/codex/-/codex-0.200.0-linux-x64.tgz"
        self.network.files[url] = archive
        document = json.loads(self.network.texts["https://registry.npmjs.org/%40openai%2Fcodex"])
        document["versions"]["0.200.0-linux-x64"]["dist"]["integrity"] = sri(archive)
        self.network.texts["https://registry.npmjs.org/%40openai%2Fcodex"] = json.dumps(document).encode()
        self.assertOnlyToolFailed("tool.codex", "changed its layout")

    def test_copilot_assets_must_match_digest_and_sha256sums(self):
        sums_url = "https://github.com/github/copilot-cli/releases/download/v1.1.0/SHA256SUMS.txt"
        self.network.texts[sums_url] = self.network.texts[sums_url].replace(b"copilot-win32-x64.zip", b"other.zip")
        self.assertOnlyToolFailed("tool.copilot", "SHA256SUMS.txt")

    def test_kiro_files_must_match_their_release_manifest(self):
        url = pins_script.KIRO_BASE + "2.40.0/kirocli-aarch64-linux.zip"
        self.network.files[url] = self.network.files[url] + b"\0"
        self.assertOnlyToolFailed("tool.kiro", "does not match its release manifest")

    def test_major_release_is_reported_without_changing_pins(self):
        self.versions = {**self.versions, "tool.kiro": "3.0.0"}
        self.publish_kiro()
        resources, summary, failures = self.run_bump("tool.kiro")
        self.assertEqual(resources["tool.kiro"], self.pins["tool.kiro"])
        self.assertEqual(failures, "")
        self.assertIn("3.0.0 held", summary)

    def test_failed_verification_returns_a_failure_status(self):
        self.network.files[pins_script.KIRO_BASE + "2.40.0/kirocli-aarch64-linux.zip"] += b"tampered"
        with mock.patch("builtins.print"):
            status = pins_script.main(["--pins", str(self.pins_path), "--tools", "tool.kiro"], network=self.network)
        self.assertEqual(status, 1)


class GitHubPinBumpTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.path = pathlib.Path(self.temporary.name) / "archive-pins.json"
        self.original = (ROOT / "installer/archive-pins.json").read_text()
        self.path.write_text(self.original)
        self.pins = json.loads(self.original)["resources"]
        self.policy = json.loads((ROOT / "scripts/archive-pin-updates.json").read_text())
        self.network = FakeNetwork()
        self.releases = []

    def publish(self, tool, version):
        recipe = self.policy["github"][tool]
        assets = []
        for target, pin in self.pins[tool].items():
            name = recipe["assets"][target].format(version=version)
            url = f"https://github.com/{recipe['repository']}/releases/download/{recipe['tag'].format(version=version)}/{name}"
            prefix = "root/" * pin.get("strip_components", 0)
            files = {prefix + entry: b"binary" for entry in layout(pin)}
            if pin["format"] == "file":
                data = b"binary"
            elif pin["format"] == "gzip":
                data = gzip.compress(b"binary")
            elif pin["format"] == "zip":
                data = zipped(files)
            else:
                data = tgz(files)
            self.network.files[url] = data
            assets.append({"name": name, "browser_download_url": url, "digest": "sha256:" + hashlib.sha256(data).hexdigest()})
        self.release = {"tag_name": recipe['tag'].format(version=version), "draft": False, "prerelease": False, "assets": assets}
        self.releases.insert(0, self.release)
        self.api = f"https://api.github.com/repos/{recipe['repository']}/releases?per_page=100"
        self.sync()

    def sync(self):
        self.network.texts[self.api] = json.dumps(self.releases).encode()
        self.network.texts[self.api.replace("releases?per_page=100", "releases/latest")] = json.dumps(self.releases[0]).encode()

    def run_bump(self, tool, expected=0):
        summary = self.path.with_suffix(".md")
        with mock.patch("builtins.print"):
            result = pins_script.main(["--pins", str(self.path), "--tools", tool, "--summary", str(summary)], network=self.network)
        self.assertEqual(result, expected)
        return json.loads(self.path.read_text())["resources"], summary.read_text()

    def test_github_archives_and_single_files_update_all_targets(self):
        for tool, version in [("tool.fzf", "0.99.0"), ("tool.t3", "0.99.0"), ("tool.jq", "1.99.0"), ("tool.taplo", "0.99.0")]:
            with self.subTest(tool=tool):
                self.publish(tool, version)
                resources, _ = self.run_bump(tool)
                for target, pin in resources[tool].items():
                    self.assertEqual(pin["version"], version)
                    self.assertEqual(pin["sha256"], hashlib.sha256(self.network.files[pin["url"]]).hexdigest())
                    self.assertEqual({k: v for k, v in pin.items() if k not in ("version", "url", "sha256")},
                                     {k: v for k, v in self.pins[tool][target].items() if k not in ("version", "url", "sha256")})

    def test_invalid_github_releases_leave_every_target_unchanged(self):
        for fault in ("missing digest", "bad digest", "missing asset", "layout", "prerelease"):
            with self.subTest(fault=fault):
                self.path.write_text(self.original)
                self.releases.clear()
                self.publish("tool.fzf", "0.99.0")
                asset = self.release["assets"][-1]
                if fault == "missing digest":
                    asset.pop("digest")
                elif fault == "bad digest":
                    asset["digest"] = "sha256:" + "0" * 64
                elif fault == "missing asset":
                    self.release["assets"].pop()
                elif fault == "layout":
                    data = tgz({"unexpected": b"x"})
                    self.network.files[asset["browser_download_url"]] = data
                    asset["digest"] = "sha256:" + hashlib.sha256(data).hexdigest()
                elif fault == "prerelease":
                    self.release["prerelease"] = True
                self.sync()
                resources, _ = self.run_bump("tool.fzf", expected=1)
                self.assertEqual(resources, self.pins)

    def test_major_github_release_is_held_before_download(self):
        self.publish("tool.fzf", "1.0.0")
        self.network.files.clear()
        resources, summary = self.run_bump("tool.fzf")
        self.assertEqual(resources, self.pins)
        self.assertIn("1.0.0 held", summary)

    def test_ignored_tag_is_never_proposed(self):
        self.publish("tool.xremap", "0.16.6")
        self.publish("tool.xremap", "0.15.20")
        self.publish("tool.xremap", "0.15.10")
        resources, _ = self.run_bump("tool.xremap")
        self.assertEqual({pin["version"] for pin in resources["tool.xremap"].values()}, {"0.15.20"})

    def test_release_below_the_pin_is_never_a_downgrade(self):
        self.publish("tool.fzf", "0.1.0")
        resources, summary = self.run_bump("tool.fzf")
        self.assertEqual(resources, self.pins)
        self.assertIn("no release in the pinned major version is newer than the pin", summary)

    def test_older_release_with_a_higher_number_does_not_replace_latest(self):
        self.publish("tool.fzf", "0.99.0")
        self.publish("tool.fzf", "0.80.0")
        resources, _ = self.run_bump("tool.fzf")
        self.assertEqual({pin["version"] for pin in resources["tool.fzf"].values()}, {"0.80.0"})

    def test_maintenance_release_below_the_pin_does_not_hide_a_newer_release(self):
        self.publish("tool.fzf", "0.80.0")
        self.publish("tool.fzf", "0.74.1")
        resources, _ = self.run_bump("tool.fzf")
        self.assertEqual({pin["version"] for pin in resources["tool.fzf"].values()}, {"0.80.0"})

    def test_held_major_still_lets_the_pinned_major_update(self):
        self.publish("tool.fzf", "0.99.0")
        self.publish("tool.fzf", "1.0.0")
        resources, summary = self.run_bump("tool.fzf")
        self.assertEqual({pin["version"] for pin in resources["tool.fzf"].values()}, {"0.99.0"})
        self.assertIn("1.0.0 held", summary)

    def test_github_assets_respect_the_installer_download_limit(self):
        for size, status in [(536_870_912, 0), (536_870_913, 1), (538_725_004, 1)]:
            with self.subTest(size=size):
                self.path.write_text(self.original)
                self.releases.clear()
                self.publish("tool.t3", "0.99.0")
                self.release["assets"][-1]["size"] = size
                self.sync()
                resources, summary = self.run_bump("tool.t3", expected=status)
                if status:
                    self.assertEqual(resources, self.pins)
                    self.assertIn("size limit", summary)
                else:
                    self.assertEqual({pin["version"] for pin in resources["tool.t3"].values()}, {"0.99.0"})

    def test_one_malformed_reply_fails_only_its_tool(self):
        for fault in ("list reply", "null size", "truncated download"):
            with self.subTest(fault=fault):
                self.path.write_text(self.original)
                self.releases = []
                self.publish("tool.fd", "10.99.0")
                self.releases = []
                self.publish("tool.fzf", "0.99.0")
                if fault == "list reply":
                    self.network.texts[self.api.replace("releases?per_page=100", "releases/latest")] = json.dumps(self.releases).encode()
                elif fault == "null size":
                    self.release["assets"][-1]["size"] = None
                    self.sync()
                else:
                    self.network.files[self.release["assets"][-1]["browser_download_url"]] = http.client.IncompleteRead(b"")
                resources, summary = self.run_bump("tool.fd,tool.fzf", expected=1)
                self.assertEqual({pin["version"] for pin in resources["tool.fd"].values()}, {"10.99.0"})
                self.assertEqual(resources["tool.fzf"], self.pins["tool.fzf"])
                self.assertIn("Failed verification:\n- `tool.fzf`", summary)

    def test_superfile_accepts_the_publishers_dot_slash_archive_prefix(self):
        self.publish("tool.superfile", "1.99.0")
        asset = next(asset for asset in self.release["assets"] if asset["name"] == "superfile-linux-v1.99.0-amd64.tar.gz")
        data = tgz({"./dist/superfile-linux-v1.99.0-amd64/spf": b"binary"})
        self.network.files[asset["browser_download_url"]] = data
        asset["digest"] = "sha256:" + hashlib.sha256(data).hexdigest()
        self.sync()
        resources, _ = self.run_bump("tool.superfile")
        self.assertEqual({pin["version"] for pin in resources["tool.superfile"].values()}, {"1.99.0"})

    def test_gersemi_cannot_update_without_the_mason_snapshot(self):
        url = "https://github.com/BlankSpruce/gersemi/releases/download/0.29.3/gersemi-windows.exe"
        data = b"binary"
        self.network.files[url] = data
        self.network.texts["https://api.github.com/repos/BlankSpruce/gersemi/releases/latest"] = json.dumps({
            "tag_name": "0.29.3", "assets": [{"name": "gersemi-windows.exe", "browser_download_url": url,
                                             "digest": "sha256:" + hashlib.sha256(data).hexdigest()}]
        }).encode()
        with mock.patch("sys.stderr", new=io.StringIO()), mock.patch("builtins.print"), self.assertRaises(SystemExit) as rejected:
            pins_script.main(["--pins", str(self.path), "--tools", "tool.gersemi"], network=self.network)
        self.assertEqual(rejected.exception.code, 2)
        self.assertEqual(self.path.read_text(), self.original)

    def test_coverage_rejects_an_unlisted_resource_and_an_unlisted_target(self):
        for change in ("resource", "target"):
            with self.subTest(change=change):
                document = json.loads(self.original)
                if change == "resource":
                    document["resources"]["tool.new"] = copy.deepcopy(self.pins["tool.fzf"])
                else:
                    document["resources"]["tool.fzf"]["linux/new"] = copy.deepcopy(self.pins["tool.fzf"]["linux/amd64"])
                self.path.write_text(json.dumps(document))
                result = subprocess.run([sys.executable, str(ROOT / "scripts/update-agent-cli-pins.py"),
                                         "--pins", str(self.path), "--check-coverage"], capture_output=True, text=True)
                self.assertEqual(result.returncode, 1)
                self.assertIn("Pin coverage failed", result.stderr)

    def test_checked_in_pin_coverage(self):
        result = subprocess.run([sys.executable, str(ROOT / "scripts/update-agent-cli-pins.py"), "--check-coverage"],
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)


class SignatureAndChecksumParsingTest(unittest.TestCase):
    def test_validsig_requires_the_primary_release_key(self):
        good = "[GNUPG:] GOODSIG BAA929FF1A7ECACE Anthropic\n[GNUPG:] VALIDSIG AAAA 2026-10-21 1 0 4 0 1 10 00 " + pins_script.CLAUDE_FINGERPRINT + "\n"
        self.assertTrue(pins_script.parse_validsig(good, pins_script.CLAUDE_FINGERPRINT))
        self.assertFalse(pins_script.parse_validsig(good.replace(pins_script.CLAUDE_FINGERPRINT, "F" * 40), pins_script.CLAUDE_FINGERPRINT))
        self.assertFalse(pins_script.parse_validsig("[GNUPG:] GOODSIG BAA929FF1A7ECACE Anthropic\n", pins_script.CLAUDE_FINGERPRINT))

    def test_sha256sums_parsing(self):
        digest = "a" * 64
        self.assertEqual(pins_script.parse_sha256sums(f"{digest}  copilot-linux-x64.tar.gz\n{digest} *x.zip\nnoise\n"),
                         {"copilot-linux-x64.tar.gz": digest, "x.zip": digest})


if __name__ == "__main__":
    unittest.main()
