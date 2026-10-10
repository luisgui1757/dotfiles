"""Released targets are bounded discovery data, never inferred ownership."""

import hashlib
import json
from pathlib import Path
import subprocess
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]


class ReleasedInventoryTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inventory = json.loads((ROOT / "installer/legacy-config-targets.json").read_text())
        cls.releases = {r["tag"]: r for r in cls.inventory["releases"]}

    def test_generated_inventory_matches_exact_release_sources(self):
        subprocess.run([sys.executable, str(ROOT / "scripts/generate-legacy-config-targets.py"), "--check"], check=True)

    def test_legacy_conventional_windows_targets_remain_distinct_from_known_folders(self):
        old = self.releases["v0.1.0"]["targets"]
        new = self.releases["v0.4.4"]["targets"]
        self.assertTrue(any(t["root"] == "home" and t["path"] == "AppData/Local/nvim" for t in old))
        self.assertTrue(any(t["root"] == "local_app_data" and t["path"] == "nvim" for t in new))
        self.assertTrue(any(t["root"] == "documents" and t["path"] == "PowerShell/Microsoft.PowerShell_profile.ps1" for t in new))
        self.assertFalse(any(t["path"].startswith(("AppData/", "Documents/")) for t in new))

    def test_old_sources_and_retired_psmux_are_preserved_as_migration_evidence(self):
        for release in self.releases.values():
            targets = {t["path"]: t for t in release["targets"] if t["root"] == "home"}
            self.assertEqual(targets[".zshrc"]["source"], "home/dot_zshrc")
            self.assertEqual(targets[".zshrc"]["platforms"], ["darwin", "linux"])
            self.assertEqual(targets[".psmux.conf"]["platforms"], ["windows"])
            self.assertEqual(targets[".config/nvim"]["form"], "explicit-link")
            self.assertIn(".chezmoi.sourceDir", targets[".config/nvim"]["link_expression"])

    def test_windows_terminal_transform_does_not_claim_whole_file_ownership(self):
        target = next(t for t in self.releases["v0.1.0"]["targets"] if t["path"].endswith("LocalState/settings.json"))
        self.assertEqual(target["form"], "scoped-modification")

    def test_retired_wezterm_keeps_released_live_link_source(self):
        target = next(t for t in self.releases["v0.4.4"]["targets"] if t["path"] == ".config/wezterm/wezterm.lua")
        self.assertEqual(target["source"], "home/dot_config/wezterm/wezterm.lua")
        self.assertEqual(hashlib.sha256((ROOT / target["source"]).read_bytes()).hexdigest(), target["source_sha256"])

    def test_passive_tmux_keeps_exact_released_live_link_bytes(self):
        for release in self.releases.values():
            with self.subTest(release=release["tag"]):
                target = next(t for t in release["targets"] if t["path"] == ".tmux.conf")
                self.assertEqual(target["source"], "home/dot_tmux.conf")
                self.assertEqual(hashlib.sha256((ROOT / target["source"]).read_bytes()).hexdigest(), target["source_sha256"])

    def test_present_passive_target_sources_keep_latest_released_bytes(self):
        latest = {target["source"]: target for release in self.releases.values() for target in release["targets"]}
        for source, target in latest.items():
            path = ROOT / source
            if path.is_file():
                with self.subTest(source=source):
                    self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), target["source_sha256"])

    def test_linked_passive_templates_keep_latest_released_bytes(self):
        latest = {item["source"]: item for release in self.releases.values() for item in release["metadata"]}
        expressions = [target.get("link_expression", "") for release in self.releases.values() for target in release["targets"]]
        for source, item in latest.items():
            if source.startswith("home/.chezmoitemplates/") and any(source.removeprefix("home/") in expression for expression in expressions):
                with self.subTest(source=source):
                    self.assertEqual(hashlib.sha256((ROOT / source).read_bytes()).hexdigest(), item["sha256"])

    def test_present_passive_profiles_keep_latest_released_bytes(self):
        evidence = json.loads((ROOT / "installer/legacy-profile-evidence.json").read_text())
        latest = {item["source"]: item for item in evidence["profiles"]}
        for source, item in latest.items():
            path = ROOT / source
            if path.is_file():
                with self.subTest(source=source):
                    self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), item["sha256"])

    def test_targets_have_no_duplicates_absolute_paths_or_parent_traversal(self):
        for release in self.releases.values():
            keys = [(t["root"], t["path"]) for t in release["targets"]]
            self.assertEqual(len(keys), len(set(keys)))
            for target in release["targets"]:
                self.assertFalse(Path(target["path"]).is_absolute())
                self.assertNotIn("..", Path(target["path"]).parts)
                self.assertEqual(len(target["source_sha256"]), 64)


if __name__ == "__main__":
    unittest.main()
