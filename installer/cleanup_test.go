package installer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCleanupResumesPartialDeletionWithoutRemovingLaterEdits(t *testing.T) {
	for _, changed := range []string{"new-file", "edited-file", "redirected-directory", "changed-mode"} {
		t.Run(changed, func(t *testing.T) {
			root, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "payload")
			for _, name := range []string{"already-removed", "unchanged", "sub/personal"} {
				writeConfigFixture(t, filepath.Join(path, name), name)
			}
			entries, err := inspectTree(path, maxConfigBytes, maxConfigEntries)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(path, "already-removed")); err != nil {
				t.Fatal(err)
			}
			kept := filepath.Join(path, "sub", "personal")
			switch changed {
			case "new-file":
				kept = filepath.Join(path, "sub", "new")
				writeConfigFixture(t, kept, "personal")
			case "edited-file":
				writeConfigFixture(t, kept, "edited")
			case "changed-mode":
				if err := os.Chmod(kept, 0400); err != nil {
					t.Fatal(err)
				}
			case "redirected-directory":
				foreign := filepath.Join(root, "foreign")
				if err := os.Rename(filepath.Join(path, "sub"), foreign); err != nil {
					t.Fatal(err)
				}
				if err := createDirectoryLink(foreign, filepath.Join(path, "sub")); err != nil {
					t.Fatal(err)
				}
				kept = filepath.Join(path, "sub")
			}
			before, err := snapshotConfig(kept)
			if err != nil {
				t.Fatal(err)
			}
			preserved, err := discardRecordedTree(path, entries, maxConfigBytes, maxConfigEntries)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(preserved, kept) {
				t.Fatal("changed entry not disclosed", preserved, kept)
			}
			if err := verifyConfigSnapshot(kept, before); err != nil {
				t.Fatal("changed entry lost", err)
			}
			if _, err := os.Lstat(filepath.Join(path, "unchanged")); !os.IsNotExist(err) {
				t.Fatal("unchanged leftover remains", err)
			}
			if changed == "redirected-directory" {
				data, err := os.ReadFile(filepath.Join(root, "foreign", "personal"))
				if err != nil || string(data) != "sub/personal" {
					t.Fatal("followed foreign directory", err)
				}
			}
		})
	}
}

func TestCleanupRejectsMalformedSavedEntriesBeforeMutation(t *testing.T) {
	for _, kind := range []string{"escape", "duplicate", "bad-hash", "missing-parent"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "payload")
			writeConfigFixture(t, path, "keep")
			entries, err := inspectTree(path, maxConfigBytes, maxConfigEntries)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "escape":
				entries[0].Path = "../payload"
			case "duplicate":
				entries = append(entries, entries[0])
			case "bad-hash":
				entries[0].Content = "invalid"
			case "missing-parent":
				entries = append(entries, configEntry{Path: "sub/file", Kind: "file", Content: entries[0].Content})
			}
			if _, err := discardRecordedTree(path, entries, maxConfigBytes, maxConfigEntries); err == nil {
				t.Fatal("accepted malformed intent")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "keep" {
				t.Fatal("mutated before validation", err)
			}
		})
	}
}
