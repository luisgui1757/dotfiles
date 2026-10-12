"""Offline tests for scripts/update-agent-cli-pins.py (no network, no gpg)."""

import base64
import copy
import hashlib
import importlib.util
import io
import json
import pathlib
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

    def run_bump(self, tools=None):
        summary = pathlib.Path(self.temporary.name) / "summary.md"
        failures = pathlib.Path(self.temporary.name) / "failures.txt"
        argv = ["--pins", str(self.pins_path), "--summary", str(summary), "--failures", str(failures)]
        if tools:
            argv += ["--tools", tools]
        with mock.patch("builtins.print"):
            self.assertEqual(pins_script.main(argv, network=self.network, verify=self.verify), 0)
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
        resources, _, failures = self.run_bump()
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
