package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveResumesRecordedPartialGenerationCleanup(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "later-personal-file"}[edit], func(t *testing.T) {
			c, d, _ := archiveController(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			current, err := d.current("tool.shared")
			if err != nil {
				t.Fatal(err)
			}
			generation := d.versionDirectory("tool.shared", current.Version)
			version, err := d.version("tool.shared", current.Version)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := inspectTree(generation, maxPackageBytes, maxPackageEntries)
			if err != nil {
				t.Fatal(err)
			}
			cleanup := archiveCleanup{Schema: 1, Generation: current.Version, Version: version, Entries: entries}
			if err := saveDocument(d.cleanupPath("tool.shared", current.Version), cleanup); err != nil {
				t.Fatal(err)
			}
			// Complete the native removal but stop before engine finalization.
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt := state.Receipts["tool.shared"]
			receipt.OperationID = strings.Repeat("e", 64)
			r, _ := c.Catalog.Resource("tool.shared")
			if _, err := d.Remove(context.Background(), r, receipt); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(generation, "version.json")); err != nil {
				t.Fatal(err)
			}
			personal := filepath.Join(generation, "payload", "personal")
			if edit {
				writeConfigFixture(t, personal, "keep my data")
			}
			preserved, err := d.FinishTransaction(context.Background(), Plan{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(generation, "payload", "tool")); !os.IsNotExist(err) {
				t.Fatal("owned leftover survived", err)
			}
			if edit {
				data, err := os.ReadFile(personal)
				if err != nil || string(data) != "keep my data" || !cleanupPreserved(personal, preserved["tool.shared"]) {
					t.Fatal("lost personal data/disclosure", err, preserved)
				}
			} else if _, err := os.Lstat(generation); !os.IsNotExist(err) {
				t.Fatal("obsolete generation survives", err)
			}
		})
	}
}

func TestArchiveReplacementNeverReclaimsPreviouslySavedGeneration(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	old, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(old, "tool"), "user version")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "repair", Adopt: []string{"tool.shared"}})
	// Restoring the original bytes does not revoke the earlier promise to keep it.
	if err := os.WriteFile(filepath.Join(old, "tool"), []byte("version one"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, selected := range [][]string{{}, {"first"}, {}} {
		result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: selected})
		if _, err := os.Stat(filepath.Join(old, "tool")); err != nil {
			t.Fatal("deleted previously saved generation", err)
		}
		if !strings.Contains(result.Message, filepath.Dir(old)) {
			t.Fatal("lost disclosure", result)
		}
	}
}
