package installer

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func saveRetiredFixture(t *testing.T, os string, keep []string, receipts map[string]Receipt) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	if err := SaveState(path, State{Schema: 1, Context: Context{OS: os, Arch: "arm64"}, Target: "/home/user", Selected: []string{}, Keep: keep, Receipts: receipts}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadStateReleasesARetiredRecordThatSetupNoLongerOwns(t *testing.T) {
	removed := Receipt{Ownership: "reused", Status: "removed", After: Observation{Preserved: []string{"/home/user/.config/git/config"}}}
	reused := Receipt{Ownership: "reused", Status: "ready", After: Observation{Present: true}}
	for _, fixture := range []struct {
		os       string
		receipts map[string]Receipt
	}{
		{"linux", map[string]Receipt{"config.git": removed, "integration.vscode": removed, "tool.flatpak": reused, "tool.obsidian": removed, "tool.fzf": reused}},
		{"darwin", map[string]Receipt{"tool.zen": removed, "desktop.zen": removed, "tool.fzf": reused}},
	} {
		path := saveRetiredFixture(t, fixture.os, []string{"tool.flatpak", "tool.fzf"}, fixture.receipts)
		state, err := LoadState(path, "/home/user")
		if err != nil {
			t.Fatal(fixture.os, err)
		}
		if len(state.Receipts) != 1 || state.Receipts["tool.fzf"].Ownership != "reused" || !slices.Equal(state.Keep, []string{"tool.fzf"}) {
			t.Fatal("a released record of a retired resource stayed", fixture.os, state.Receipts, state.Keep)
		}
	}
}

func TestLoadStateStopsOnARetiredRecordThatSetupStillOwns(t *testing.T) {
	for _, fixture := range []struct {
		os, id, ownership, advice string
	}{
		{"linux", "config.git", "created", "Git"},
		{"linux", "integration.vscode", "uncertain", "VS Code"},
		{"linux", "tool.flatpak", "created", "Flatpak"},
		{"linux", "tool.obsidian", "created", "Obsidian"},
		{"darwin", "tool.zen", "created", "Zen"},
		{"darwin", "desktop.zen", "created", "Zen"},
	} {
		path := saveRetiredFixture(t, fixture.os, nil, map[string]Receipt{fixture.id: {Ownership: fixture.ownership, Status: "ready", After: Observation{Present: true}}})
		_, err := LoadState(path, "/home/user")
		if err == nil || !strings.Contains(err.Error(), fixture.id) || !strings.Contains(err.Error(), "v1.2.0") || !strings.Contains(err.Error(), fixture.advice) {
			t.Fatal("an owned record of a retired resource did not stop with the v1.2.0 step", fixture.id, err)
		}
	}
	for _, fixture := range []struct{ os, id string }{{"linux", "tool.zen"}, {"linux", "desktop.zen"}, {"darwin", "tool.obsidian"}} {
		path := saveRetiredFixture(t, fixture.os, nil, map[string]Receipt{fixture.id: {Ownership: "created", Status: "ready", After: Observation{Present: true}}})
		if state, err := LoadState(path, "/home/user"); err != nil || state.Receipts[fixture.id].Ownership != "created" {
			t.Fatal("a resource that this platform still supports was treated as retired", fixture, err)
		}
	}
}

func TestSavedPackageGenerationBeforeTheFloorIsRefused(t *testing.T) {
	disk := ArchivePin{Format: "dmg", DMGRevision: 0}
	if err := disk.Validate(); err == nil || !strings.Contains(err.Error(), "Check out "+supportFloor) {
		t.Fatal("a revision-0 disk-image generation was not refused with the floor release step", err)
	}
	for _, command := range []string{"Scripts/yamllint.exe", "commands/yamllint.exe"} {
		python := ArchivePin{Yamllint: &YamllintPin{}, Commands: map[string]string{"yamllint": command}}
		if err := validateArchivePreparation(python); err == nil || !strings.Contains(err.Error(), "Check out "+supportFloor) {
			t.Fatal("a revision-0 Windows Python generation was not refused with the floor release step", command, err)
		}
	}
}
