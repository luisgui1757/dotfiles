#!/usr/bin/env python3
"""Propose verified archive pin updates without major version changes.

Claude Code, Codex, GitHub Copilot CLI, Kiro CLI and Muse Code move quickly, so a daily
workflow runs this script and proposes the result as a pull request. Every
pinned byte is verified the same way the reviewed pins were prepared:

- Claude Code: the stable channel's release manifest must carry a valid
  detached signature from Anthropic's release key (fixed fingerprint); each npm
  platform package must match its published SHA-512 integrity, and the binary
  inside it must match the signed manifest checksum.
- Codex: each platform npm package must match its published SHA-512 integrity
  and still contain the pinned commands and required files.
- Copilot CLI: each asset must match both its GitHub release digest and the
  release's SHA256SUMS.txt, and still contain its single executable. The
  release must be at least three days old.
- Kiro CLI (macOS/Linux): each file must match the size and SHA-256 in Kiro's
  release manifest (no versioned manifest exists, so the current one is read at
  bump time and the pin records the fixed-version URL). Linux ZIPs keep their
  layout.
- Muse Code: the muse-stable channel names a version and its manifest. Each
  file must match the size and SHA-256 in that manifest. The manifest and the
  files use fixed-version URLs, so the pin records them directly.

Only version, URL and SHA-256 change; any other layout change fails that tool
for manual review.
GitHub recipes and manual exceptions are in archive-pin-updates.json.
GitHub assets must match their published SHA-256 and the installed layout.
A GitHub recipe release must be at least three days old.
Network, signature checks and the clock are injectable for tests.
"""

from __future__ import annotations

import argparse
import base64
import datetime
import gzip
import hashlib
import http.client
import json
import os
import pathlib
import re
import subprocess
import sys
import tarfile
import tempfile
import urllib.parse
import urllib.request
import zipfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
PINS = ROOT / "installer" / "archive-pins.json"
AGENT_TOOLS = ("tool.claude", "tool.codex", "tool.copilot", "tool.kiro", "tool.muse")
POLICY = ROOT / "scripts" / "archive-pin-updates.json"
STABLE = re.compile(r"[0-9]+\.[0-9]+\.[0-9]+(?:\.[0-9]+)?")
CLAUDE_RELEASES = "https://downloads.claude.ai/claude-code-releases"
CLAUDE_KEY = "https://downloads.claude.ai/keys/claude-code.asc"
CLAUDE_FINGERPRINT = "31DDDE24DDFAB679F42D7BD2BAA929FF1A7ECACE"
CLAUDE_PLATFORMS = {"darwin/arm64": "darwin-arm64", "linux/amd64": "linux-x64", "linux/arm64": "linux-arm64", "windows/amd64": "win32-x64"}
CODEX_PLATFORMS = CLAUDE_PLATFORMS
COPILOT_ASSETS = {"darwin/arm64": "copilot-darwin-arm64.tar.gz", "linux/amd64": "copilot-linux-x64.tar.gz",
                  "linux/arm64": "copilot-linux-arm64.tar.gz", "windows/amd64": "copilot-win32-x64.zip"}
KIRO_BASE = "https://prod.download.cli.kiro.dev/stable/"
KIRO_FILES = {"darwin/arm64": "Kiro CLI.dmg", "linux/amd64": "kirocli-x86_64-linux.zip",
              "linux/arm64": "kirocli-aarch64-linux.zip"}
MUSE_CHANNEL = "https://api.meta.ai/muse-code/channels/muse-stable"
MUSE_BASE = "https://lookaside.facebook.com/lookaside/muse/download/?channel=muse&version={version}&file={file}"
MUSE_VERSION = re.compile(r"([0-9]+)\.([0-9]+)\.([0-9]+)-R([0-9]+)\.([0-9]+)")
MUSE_FILES = {"darwin/arm64": ("aarch64_macos", "muse-aarch64-macos"), "linux/amd64": ("x86_linux", "muse-x86-linux"),
              "linux/arm64": ("aarch64_linux", "muse-aarch64-linux"), "windows/amd64": ("x86_windows", "muse-x86-windows.exe")}
MAX_DOWNLOAD = 512 << 20
MIN_RELEASE_AGE = datetime.timedelta(days=3)


class BumpError(Exception):
    """A release that cannot be pinned without manual review."""


class UnchangedRelease(Exception):
    """A current release or a major release held for review."""


class Network:
    """Bounded HTTPS access; only api.github.com requests carry a token."""

    def __init__(self, token: str = ""):
        self.token = token

    def _open(self, url: str):
        parts = urllib.parse.urlsplit(url)
        if parts.scheme != "https":
            raise BumpError(f"refusing non-HTTPS URL {url}")
        request = urllib.request.Request(url, headers={"User-Agent": "dotfiles-agent-cli-pins"})
        if self.token and parts.hostname == "api.github.com":
            request.add_header("Authorization", f"Bearer {self.token}")
        if parts.hostname == "registry.npmjs.org" and parts.path.count("/") == 1:
            request.add_header("Accept", "application/vnd.npm.install-v1+json")
        return urllib.request.urlopen(request, timeout=120)

    def text(self, url: str) -> bytes:
        limit = 32 << 20
        with self._open(url) as response:
            data = response.read(limit + 1)
        if len(data) > limit:
            raise BumpError(f"{url} exceeds the metadata size limit")
        return data

    def json(self, url: str):
        return json.loads(self.text(url))

    def download(self, url: str, destination: pathlib.Path) -> dict:
        sha256, sha512, size = hashlib.sha256(), hashlib.sha512(), 0
        with self._open(url) as response, destination.open("wb") as output:
            while chunk := response.read(1 << 20):
                size += len(chunk)
                if size > MAX_DOWNLOAD:
                    raise BumpError(f"{url} exceeds the download size limit")
                sha256.update(chunk)
                sha512.update(chunk)
                output.write(chunk)
        return {"sha256": sha256.hexdigest(), "sha512": sha512.digest(), "size": size}


def require_stable(tool: str, version: str) -> str:
    if not STABLE.fullmatch(version or ""):
        raise BumpError(f"{tool} latest stable version {version!r} is not a plain release version")
    return version


def release_key(tool: str, version: str) -> tuple:
    if tool == "tool.muse":
        match = MUSE_VERSION.fullmatch(version or "")
        if not match:
            raise BumpError(f"{tool} latest stable version {version!r} is not a Muse release version")
        return tuple(map(int, match.groups()))
    return tuple(map(int, require_stable(tool, version).split(".")))


def check_version(tool: str, pins: dict, version: str) -> None:
    proposed = release_key(tool, version)
    current = {pin["version"] for pin in pins[tool].values()}
    previous = [release_key(tool, item) for item in current]
    if any(proposed < item for item in previous):
        raise BumpError(f"{version} would downgrade a pinned target")
    if any(proposed[0] != item[0] for item in previous):
        raise UnchangedRelease(f"{version} held for major version review (pinned: {', '.join(sorted(current))})")
    if current == {version}:
        raise UnchangedRelease(f"{version} is current")


def check_coverage(pins: dict, policy: dict) -> None:
    groups = [set(AGENT_TOOLS), set(policy["github"]), set(policy["manual"])]
    if any(a & b for i, a in enumerate(groups) for b in groups[i + 1:]):
        raise BumpError("pin update policy has duplicate owners")
    covered = set().union(*groups)
    if set(pins) != covered:
        raise BumpError(f"pin coverage differs: missing {sorted(set(pins) - covered)}, stale {sorted(covered - set(pins))}")
    for tool, reason in policy["manual"].items():
        if not isinstance(reason, str) or not reason.strip():
            raise BumpError(f"{tool} needs a manual review reason")
    for tool, recipe in policy["github"].items():
        if set(recipe["assets"]) != set(pins[tool]):
            raise BumpError(f"{tool} recipe must cover exactly its pinned targets")
        for target, pin in pins[tool].items():
            if pin["url"] != github_url(recipe, target, pin["version"]):
                raise BumpError(f"{tool} on {target} does not match its explicit release recipe")
            if pin["format"] not in {"tar.gz", "tar.xz", "zip", "gzip", "file"}:
                raise BumpError(f"{tool} on {target} needs a layout verification recipe")


def github_url(recipe: dict, target: str, version: str) -> str:
    tag = recipe["tag"].format(version=version)
    asset = recipe["assets"][target].format(version=version)
    return f"https://github.com/{recipe['repository']}/releases/download/{tag}/{urllib.parse.quote(asset)}"


def verify_sri(integrity: str, sha512: bytes, what: str) -> None:
    expected = "sha512-" + base64.b64encode(sha512).decode()
    if integrity != expected:
        raise BumpError(f"{what} does not match its published SHA-512 integrity")


def parse_sha256sums(text: str) -> dict:
    sums = {}
    for line in text.splitlines():
        match = re.fullmatch(r"([0-9a-f]{64}) [ *](\S+)", line.strip())
        if match:
            sums[match.group(2)] = match.group(1)
    return sums


def parse_validsig(status: str, fingerprint: str) -> bool:
    """gpg --status-fd: VALIDSIG ends with the primary key fingerprint."""
    for line in status.splitlines():
        fields = line.split()
        if len(fields) >= 3 and fields[:2] == ["[GNUPG:]", "VALIDSIG"] and fields[-1].upper() == fingerprint:
            return True
    return False


def gpg_verify(manifest: bytes, signature: bytes, key: bytes) -> None:
    """Verify Claude's detached manifest signature with only the pinned key."""
    with tempfile.TemporaryDirectory() as home:
        base = pathlib.Path(home)
        for name, data in (("key.asc", key), ("manifest.json", manifest), ("manifest.json.sig", signature)):
            (base / name).write_bytes(data)
        environment = {**os.environ, "GNUPGHOME": str(base / "gnupg")}
        (base / "gnupg").mkdir(mode=0o700)
        subprocess.run(["gpg", "--batch", "--quiet", "--import", str(base / "key.asc")], env=environment, check=True, capture_output=True)
        listed = subprocess.run(["gpg", "--batch", "--with-colons", "--fingerprint"], env=environment, check=True, capture_output=True, text=True).stdout
        primaries = [line.split(":")[9] for line in listed.splitlines() if line.startswith("fpr:")]
        if CLAUDE_FINGERPRINT not in primaries:
            raise BumpError("Claude Code release key does not have the published fingerprint")
        result = subprocess.run(["gpg", "--batch", "--status-fd", "1", "--verify", str(base / "manifest.json.sig"), str(base / "manifest.json")],
                                env=environment, capture_output=True, text=True)
        if result.returncode != 0 or not parse_validsig(result.stdout, CLAUDE_FINGERPRINT):
            raise BumpError("Claude Code release manifest signature is not valid for Anthropic's release key")


def tar_members(path: pathlib.Path) -> dict:
    with tarfile.open(path, "r:*") as archive:
        return {member.name: member for member in archive.getmembers()}


def tar_file_sha256(path: pathlib.Path, name: str) -> str:
    with tarfile.open(path, "r:gz") as archive:
        stream = archive.extractfile(name)
        if stream is None:
            raise BumpError(f"{path.name} lacks file {name}")
        digest = hashlib.sha256()
        while chunk := stream.read(1 << 20):
            digest.update(chunk)
        return digest.hexdigest()


def archive_names(path: pathlib.Path, form: str, strip: int) -> set:
    if form == "zip":
        with zipfile.ZipFile(path) as archive:
            names = [name for name in archive.namelist() if not name.endswith("/")]
    else:
        names = [name for name, member in tar_members(path).items() if not member.isdir()]
    stripped = set()
    for name in names:
        parts = name.removeprefix("./").split("/")
        if len(parts) > strip:
            stripped.add("/".join(parts[strip:]))
    return stripped


def require_layout(tool: str, target: str, pin: dict, names: set) -> None:
    wanted = set(pin.get("commands", {}).values()) | set(pin.get("required_files", [])) | set(pin.get("excluded_files", []))
    missing = sorted(wanted - names)
    if missing:
        raise BumpError(f"{tool} on {target} changed its layout (missing {', '.join(missing)}); review the pin manually")


def released_before(release: dict, cutoff: datetime.datetime) -> bool:
    try:
        return datetime.datetime.fromisoformat(release["published_at"]) <= cutoff
    except (KeyError, TypeError, ValueError):
        return False


def github_release(network, pins: dict, work: pathlib.Path, tool: str, recipe: dict, held: list, now: datetime.datetime):
    cutoff = now - MIN_RELEASE_AGE
    pattern = re.escape(recipe["tag"]).replace(re.escape("{version}"), "(.+)")

    def stable(release):
        if release.get("tag_name") in recipe.get("ignore", ()):
            return None
        match = re.fullmatch(pattern, release.get("tag_name", ""))
        if release.get("prerelease") or release.get("draft") or not match or not STABLE.fullmatch(match.group(1)):
            return None
        return tuple(map(int, match.group(1).split("."))), match.group(1), release

    api = f"https://api.github.com/repos/{recipe['repository']}/releases"
    majors = {int(pin["version"].split(".")[0]) for pin in pins[tool].values()}
    pinned = max(tuple(map(int, require_stable(tool, pin["version"]).split("."))) for pin in pins[tool].values())
    latest = stable(network.json(f"{api}/latest"))
    current = latest and latest[0][0] in majors and latest[0] >= pinned
    if current and released_before(latest[2], cutoff):
        _, version, release = latest
    else:
        if latest and latest[0][0] > max(majors):
            held.append(f"- `{tool}`: {latest[1]} held for major version review")
        eligible = [candidate for candidate in map(stable, network.json(f"{api}?per_page=100"))
                    if candidate and candidate[0][0] in majors]
        if not eligible:
            if latest:
                raise UnchangedRelease("no newer release in the pinned major version")
            raise BumpError(f"{tool} has no stable release that matches its recipe")
        best = max((candidate for candidate in eligible if released_before(candidate[2], cutoff)),
                   key=lambda candidate: candidate[0], default=None)
        if current and (not best or best[0] <= pinned):
            raise UnchangedRelease(f"{latest[1]} is not three days old yet")
        if not best or best[0] < pinned:
            raise UnchangedRelease("no release in the pinned major version is newer than the pin")
        _, version, release = best
    check_version(tool, pins, version)
    assets = {asset["name"]: asset for asset in release.get("assets", [])}
    updates = {}
    for target, pin in pins[tool].items():
        name = recipe["assets"][target].format(version=version)
        asset = assets.get(name)
        url = github_url(recipe, target, version)
        if not asset or asset.get("browser_download_url") != url:
            raise BumpError(f"{tool} release lacks {name}")
        if asset.get("size", 0) > MAX_DOWNLOAD:
            raise BumpError(f"{tool} {name} exceeds the download size limit")
        digest = asset.get("digest") or ""
        if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
            raise BumpError(f"{tool} {name} lacks a published SHA-256")
        file = work / "download"
        result = network.download(url, file)
        if digest != "sha256:" + result["sha256"]:
            raise BumpError(f"{tool} {name} does not match its release digest")
        if pin["format"] in {"file", "gzip"}:
            if pin["format"] == "gzip":
                with gzip.open(file, "rb") as stream:
                    while stream.read(1 << 20):
                        pass
            names = {pin["file"]}
        else:
            names = archive_names(file, pin["format"], pin.get("strip_components", 0))
        require_layout(tool, target, pin, names)
        updates[target] = {"version": version, "url": url, "sha256": result["sha256"]}
    return version, updates


def claude(network, pins: dict, work: pathlib.Path, verify=gpg_verify):
    version = require_stable("Claude Code", network.text(f"{CLAUDE_RELEASES}/stable").decode().strip())
    check_version("tool.claude", pins, version)
    manifest_bytes = network.text(f"{CLAUDE_RELEASES}/{version}/manifest.json")
    verify(manifest_bytes, network.text(f"{CLAUDE_RELEASES}/{version}/manifest.json.sig"), network.text(CLAUDE_KEY))
    manifest = json.loads(manifest_bytes)
    if manifest.get("version") != version:
        raise BumpError("Claude Code signed manifest names another version")
    updates = {}
    for target, platform in CLAUDE_PLATFORMS.items():
        package = f"@anthropic-ai/claude-code-{platform}"
        metadata = network.json(f"https://registry.npmjs.org/{urllib.parse.quote(package, safe='@')}/{version}")
        url = metadata["dist"]["tarball"]
        if url != f"https://registry.npmjs.org/{package}/-/claude-code-{platform}-{version}.tgz":
            raise BumpError(f"Claude Code {platform} tarball is not the registry's canonical URL")
        file = work / f"claude-{platform}.tgz"
        result = network.download(url, file)
        verify_sri(metadata["dist"]["integrity"], result["sha512"], f"Claude Code {platform}")
        binary = "package/" + pins["tool.claude"][target]["commands"]["claude"]
        if tar_file_sha256(file, binary) != manifest["platforms"][platform]["checksum"]:
            raise BumpError(f"Claude Code {platform} binary does not match the signed manifest")
        updates[target] = {"version": version, "url": url, "sha256": result["sha256"]}
    return version, updates


def codex(network, pins: dict, work: pathlib.Path):
    metadata = network.json("https://registry.npmjs.org/%40openai%2Fcodex")
    tags = metadata["dist-tags"]
    version = require_stable("Codex", tags.get("latest", ""))
    check_version("tool.codex", pins, version)
    updates = {}
    for target, platform in CODEX_PLATFORMS.items():
        release = f"{version}-{platform}"
        if tags.get(platform) != release or release not in metadata["versions"]:
            raise BumpError(f"Codex {platform} is not published for {version}")
        dist = metadata["versions"][release]["dist"]
        url = dist["tarball"]
        if url != f"https://registry.npmjs.org/@openai/codex/-/codex-{release}.tgz":
            raise BumpError(f"Codex {platform} tarball is not the registry's canonical URL")
        file = work / f"codex-{platform}.tgz"
        result = network.download(url, file)
        verify_sri(dist["integrity"], result["sha512"], f"Codex {platform}")
        pin = pins["tool.codex"][target]
        require_layout("Codex", target, pin, archive_names(file, "tar.gz", pin.get("strip_components", 0)))
        updates[target] = {"version": version, "url": url, "sha256": result["sha256"]}
    return version, updates


def copilot(network, pins: dict, work: pathlib.Path, now: datetime.datetime):
    api = "https://api.github.com/repos/github/copilot-cli/releases"
    release = network.json(f"{api}/latest")
    if release.get("prerelease") or release.get("draft"):
        raise BumpError("Copilot CLI latest release is not a stable release")
    version = require_stable("Copilot CLI", release.get("tag_name", "").removeprefix("v"))
    check_version("tool.copilot", pins, version)
    cutoff = now - MIN_RELEASE_AGE
    if not released_before(release, cutoff):
        def key(candidate):
            return release_key("tool.copilot", candidate["tag_name"].removeprefix("v"))
        pinned = max(release_key("tool.copilot", pin["version"]) for pin in pins["tool.copilot"].values())
        newer = [candidate for candidate in network.json(f"{api}?per_page=100")
                 if not candidate.get("prerelease") and not candidate.get("draft") and released_before(candidate, cutoff)
                 and STABLE.fullmatch(candidate.get("tag_name", "").removeprefix("v")) and key(candidate) > pinned]
        if not newer:
            raise UnchangedRelease(f"{version} is not three days old yet")
        release = max(newer, key=key)
        version = release["tag_name"].removeprefix("v")
        check_version("tool.copilot", pins, version)
    assets = {asset["name"]: asset for asset in release.get("assets", [])}
    if "SHA256SUMS.txt" not in assets:
        raise BumpError("Copilot CLI release lacks SHA256SUMS.txt")
    sums = parse_sha256sums(network.text(assets["SHA256SUMS.txt"]["browser_download_url"]).decode())
    updates = {}
    for target, name in COPILOT_ASSETS.items():
        asset = assets.get(name)
        url = f"https://github.com/github/copilot-cli/releases/download/v{version}/{name}"
        if not asset or asset.get("browser_download_url") != url:
            raise BumpError(f"Copilot CLI {name} is missing from v{version}")
        file = work / name
        result = network.download(url, file)
        if asset.get("digest") != "sha256:" + result["sha256"] or sums.get(name) != result["sha256"]:
            raise BumpError(f"Copilot CLI {name} does not match its release digest and SHA256SUMS.txt")
        pin = pins["tool.copilot"][target]
        require_layout("Copilot CLI", target, pin, archive_names(file, pin["format"], pin.get("strip_components", 0)))
        updates[target] = {"version": version, "url": url, "sha256": result["sha256"]}
    return version, updates


def kiro(network, pins: dict, work: pathlib.Path):
    manifest = network.json(KIRO_BASE + "latest/manifest.json")
    version = require_stable("Kiro CLI", manifest.get("version", ""))
    check_version("tool.kiro", pins, version)
    published = {package["download"]: package for package in manifest.get("packages", [])}
    updates = {}
    for target, name in KIRO_FILES.items():
        package = published.get(f"{version}/{name}")
        if not package or not re.fullmatch(r"[0-9a-f]{64}", package.get("sha256", "")):
            raise BumpError(f"Kiro CLI {name} is missing from the {version} manifest")
        url = KIRO_BASE + urllib.parse.quote(f"{version}/{name}")
        file = work / name.replace(" ", "-")
        result = network.download(url, file)
        if result["sha256"] != package["sha256"] or result["size"] != package.get("size"):
            raise BumpError(f"Kiro CLI {name} does not match its release manifest")
        pin = pins["tool.kiro"][target]
        updates[target] = {"version": version, "url": url, "sha256": result["sha256"]}
        if pin["format"] == "zip":
            require_layout("Kiro CLI", target, pin, archive_names(file, "zip", pin.get("strip_components", 0)))
    return version, updates


def muse(network, pins: dict, work: pathlib.Path):
    channel = network.json(MUSE_CHANNEL)
    if channel.get("channel") != "muse-stable" or channel.get("state") != "public":
        raise BumpError("Muse Code channel is not the public muse-stable channel")
    version = channel.get("version", "")
    check_version("tool.muse", pins, version)
    if channel.get("manifest_url") != MUSE_BASE.format(version=version, file="manifest.json"):
        raise BumpError("Muse Code channel names an unexpected manifest URL")
    manifest = network.json(channel["manifest_url"])
    if manifest.get("version") != version or manifest.get("checksum_algorithm") != "sha256":
        raise BumpError("Muse Code manifest names another version or checksum")
    updates = {}
    for target, (platform, name) in MUSE_FILES.items():
        artifact = manifest.get("artifacts", {}).get(platform) or {}
        url = MUSE_BASE.format(version=version, file=name)
        if artifact.get("url") != url or not re.fullmatch(r"[0-9a-f]{64}", artifact.get("checksum", "")):
            raise BumpError(f"Muse Code {name} is missing from the {version} manifest")
        result = network.download(url, work / name)
        if result["sha256"] != artifact["checksum"] or result["size"] != artifact.get("size"):
            raise BumpError(f"Muse Code {name} does not match its release manifest")
        updates[target] = {"version": version, "url": url, "sha256": result["sha256"]}
    return version, updates


BUMPERS = {"tool.claude": claude, "tool.codex": codex, "tool.copilot": copilot, "tool.kiro": kiro, "tool.muse": muse}


def apply_updates(pins: dict, tool: str, updates: dict) -> None:
    current = pins["resources"][tool]
    if set(updates) != set(current):
        raise BumpError(f"{tool} release does not cover exactly its pinned targets")
    for target, fields in updates.items():
        current[target].update(fields)


def main(argv=None, network=None, verify=gpg_verify, now=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--pins", type=pathlib.Path, default=PINS)
    parser.add_argument("--tools")
    parser.add_argument("--check-coverage", action="store_true")
    parser.add_argument("--summary", type=pathlib.Path)
    parser.add_argument("--failures", type=pathlib.Path)
    args = parser.parse_args(argv)
    network = network or Network(os.environ.get("GITHUB_TOKEN", ""))
    now = now or datetime.datetime.now(datetime.timezone.utc)
    document = json.loads(args.pins.read_text(encoding="utf-8"))
    policy = json.loads(POLICY.read_text(encoding="utf-8"))
    try:
        check_coverage(document["resources"], policy)
    except BumpError as error:
        print(f"Pin coverage failed: {error}", file=sys.stderr)
        return 1
    if args.check_coverage:
        print(f"Pin coverage passed: {len(document['resources'])} resources")
        return 0
    lines, failures, held = [], [], []
    selected = args.tools.split(",") if args.tools else [*AGENT_TOOLS, *policy["github"]]
    for tool in selected:
        if tool not in BUMPERS and tool not in policy["github"]:
            parser.error(f"unknown tool {tool}")
        current = sorted({pin["version"] for pin in document["resources"][tool].values()})
        try:
            with tempfile.TemporaryDirectory() as work:
                pins = document["resources"]
                if tool == "tool.claude":
                    version, updates = claude(network, pins, pathlib.Path(work), verify)
                elif tool == "tool.copilot":
                    version, updates = copilot(network, pins, pathlib.Path(work), now)
                elif tool in policy["github"]:
                    version, updates = github_release(network, pins, pathlib.Path(work), tool, policy["github"][tool], held, now)
                else:
                    version, updates = BUMPERS[tool](network, pins, pathlib.Path(work))
            apply_updates(document, tool, updates)
            lines.append(f"- `{tool}`: {', '.join(current)} -> {version}")
            lines += [f"  - {target}: `{fields['sha256']}`" for target, fields in sorted(updates.items())]
        except UnchangedRelease as reason:
            lines.append(f"- `{tool}`: {reason}")
        except (BumpError, KeyError, ValueError, TypeError, AttributeError, OSError, EOFError, http.client.HTTPException,
                tarfile.TarError, zipfile.BadZipFile, subprocess.CalledProcessError) as error:
            failures.append(f"- `{tool}`: {error}")
    args.pins.write_text(json.dumps(document, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    manual = [f"- `{tool}`: {reason}" for tool, reason in sorted(policy["manual"].items())]
    summary = "\n".join(["Archive pins:", *lines, *(["", "Held for major version review:", *held] if held else []),
                         *(["", "Failed verification:", *failures] if failures else []),
                         "", "Manual pins (the Renovate dashboard lists their new releases):", *manual]) + "\n"
    print(summary, end="")
    if args.summary:
        args.summary.write_text(summary, encoding="utf-8")
    if args.failures:
        args.failures.write_text("".join(line + "\n" for line in failures), encoding="utf-8")
    return int(bool(failures))


if __name__ == "__main__":
    sys.exit(main())
