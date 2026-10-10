"""Exact released evidence and passive live-link compatibility at major cutover."""

import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import unittest

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]


def load_script(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class LegacyProfileEvidenceTest(unittest.TestCase):
    def test_profile_evidence_matches_all_exact_release_sources(self):
        subprocess.run([sys.executable, str(ROOT / "scripts/generate-legacy-profile-evidence.py"), "--check"], check=True)

    def test_intermediate_releases_introduce_no_omitted_target_shapes(self):
        inventory = json.loads((ROOT / "installer/legacy-config-targets.json").read_text())
        known = {(item["root"], item["path"], item["form"], item.get("link_expression"))
                 for release in inventory["releases"] for item in release["targets"]}
        generator = load_script("legacy_targets", ROOT / "scripts/generate-legacy-config-targets.py")
        profiles = load_script("legacy_profiles", ROOT / "scripts/generate-legacy-profile-evidence.py")
        for commit in profiles.RELEASES.values():
            for row in generator.git("ls-tree", "-rz", "--full-tree", commit, "home", "windows").split(b"\0"):
                if not row:
                    continue
                info, raw_path = row.split(b"\t", 1)
                path = raw_path.decode()
                if path.startswith("home/.chezmoi") or path == "windows/chezmoi-overlay.toml":
                    continue
                item = generator.target(path, info.decode().split()[2], generator.git("show", f"{commit}:{path}"))
                self.assertIn((item["root"], item["path"], item["form"], item.get("link_expression")), known, (commit, path))

    def test_pull_before_migration_keeps_passive_released_link_sources(self):
        inventory = json.loads((ROOT / "installer/legacy-config-targets.json").read_text())
        for release in inventory["releases"]:
            for item in release["targets"]:
                if item["form"] == "source-file" and set(item["platforms"]) & {"linux", "darwin"}:
                    self.assertTrue((ROOT / item["source"]).is_file(), item["source"])
            for item in release["metadata"]:
                if item["source"].startswith("home/.chezmoitemplates/") and any(item["source"].removeprefix("home/") in target.get("link_expression", "") for target in release["targets"]):
                    self.assertTrue((ROOT / item["source"]).is_file(), item["source"])
        # These explicit links bypass the home/ template names on released installs.
        self.assertTrue((ROOT / "nvim/init.lua").is_file())
        self.assertTrue((ROOT / "shells/powershell_profile.ps1").is_file())
        self.assertTrue((ROOT / "herdr/config.windows.toml").is_file())
        self.assertTrue((ROOT / "lazygit/config.windows.yml").is_file())


if __name__ == "__main__":
    unittest.main()
