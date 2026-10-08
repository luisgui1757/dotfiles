package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInspectionFailureReportsOnlyBoundedMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-file")
	writeConfigFixture(t, path, "personal content must not appear")
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, "changed")
	after, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	failure := configInspectionFailure(strings.Repeat("n", 4096), "file", before, after, true, nil)
	text := failure.Error()
	for _, wanted := range []string{"configuration changed during inspection; review a fresh plan", "[truncated]", "kind=file", "same_file=true", "size=32", "size=7", "mode=", "mtime_ns="} {
		if !strings.Contains(text, wanted) {
			t.Fatal("missing failure metadata", wanted, text)
		}
	}
	if len(text) > 1000 || strings.Contains(text, path) || strings.Contains(text, "personal content") {
		t.Fatal("diagnostic exceeded its content/path boundary", text)
	}
	failure = configInspectionFailure("quoted\"\nUnicode-λ-😀", "file", before, after, false, nil)
	if strings.Contains(failure.Error(), "\n") || !strings.Contains(failure.Error(), `entry="quoted\"\nUnicode-λ-😀"`) || !strings.Contains(failure.Error(), "same_file=false") {
		t.Fatal("relative name or identity verdict was not preserved safely", failure)
	}
	failure = configInspectionFailure("private-file", "file", before, nil, false, os.ErrNotExist)
	if !errors.Is(failure, os.ErrNotExist) || !strings.Contains(failure.Error(), "after={unavailable}") {
		t.Fatal("diagnostic hid original Lstat failure", failure)
	}
}

func TestConfigInspectionDiagnosticsPreserveChangedSnapshotRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "personal-config")
	writeConfigFixture(t, path, "original personal content")
	before, err := snapshotConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, path, "edited personal content")
	if err := verifyConfigSnapshot(path, before); err == nil {
		t.Fatal("changed personal configuration was accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "edited personal content" {
		t.Fatal("inspection changed personal configuration", err)
	}
}
