package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigSnapshotAndCopyPreserveNestedBytesAndDetectLaterEdits(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
	if err := os.MkdirAll(filepath.Join(source, "nested", "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"init.lua": "return {}\n", "nested/config.json": "{\"theme\":\"rose-pine\"}\n"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := snapshotConfig(source)
	if err != nil || before.Kind != "directory" {
		t.Fatal(before, err)
	}
	if err := copyConfigPayload(source, destination); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotConfig(destination)
	if err != nil || before != after {
		t.Fatal("copied configuration differs", before, after, err)
	}
	if err := copyConfigPayload(source, destination); err == nil {
		t.Fatal("copy overwrote an existing destination")
	}
	if err := os.WriteFile(filepath.Join(destination, "nested", "config.json"), []byte("user customization"), 0600); err != nil {
		t.Fatal(err)
	}
	modified, err := snapshotConfig(destination)
	if err != nil || modified == before {
		t.Fatal("snapshot missed nested user changes", modified, err)
	}
}

func TestConfigSnapshotDoesNotFollowLinksAndCopyRejectsLinkedSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows copies need no symlink privilege; POSIX exercises link identity")
	}
	root := t.TempDir()
	link := filepath.Join(root, "config")
	if err := os.Symlink(filepath.Join(root, "absent external target"), link); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotConfig(link)
	if err != nil || before.Kind != "link" {
		t.Fatal("snapshot followed a dangling user link", before, err)
	}
	if err := copyConfigPayload(link, filepath.Join(root, "copy")); err == nil {
		t.Fatal("copy followed a user link")
	}
}

func TestConfigSnapshotRefusesOversizedUserFilesBeforeCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "large")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxConfigBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotConfig(source); err == nil {
		t.Fatal("unbounded user file was accepted")
	}
	destination := filepath.Join(root, "copy")
	if err := copyConfigPayload(source, destination); err == nil {
		t.Fatal("unbounded user file was copied")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatal("oversized source created a target", err)
	}
}
