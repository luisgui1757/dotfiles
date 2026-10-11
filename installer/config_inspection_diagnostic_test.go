package installer

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	failure := configInspectionFailure(path, strings.Repeat("n", 4096), "file", before, after, true, nil)
	text := failure.Error()
	for _, wanted := range []string{"configuration changed during inspection", "[truncated]", "kind=file", "same_file=true", "size=32", "size=7", "mtime_ns=", "not needed"} {
		if !strings.Contains(text, wanted) {
			t.Fatal("missing failure metadata", wanted, text)
		}
	}
	if len(text) > 1100 || strings.Contains(text, path) || strings.Contains(text, "personal content") {
		t.Fatal("diagnostic exceeded its content/path boundary", text)
	}
	failure = configInspectionFailure(path, "private-file", "file", before, nil, false, os.ErrNotExist)
	if !errors.Is(failure, os.ErrNotExist) || !strings.Contains(failure.Error(), "after={unavailable}") {
		t.Fatal("diagnostic hid original Lstat failure", failure)
	}
}

func TestConfigInspectionWindowsIdentityProbeIsReadOnlyAndReportsErrors(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows identity boundary")
	}
	path := filepath.Join(t.TempDir(), "private-file")
	writeConfigFixture(t, path, "unchanged")
	before, err := snapshotConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	output := configInspectionIdentityDiagnostic(path)
	if !strings.Contains(output, "volume=") || !strings.Contains(output, "query_error=<nil> close_error=<nil>") || strings.Contains(output, path) {
		t.Fatal("native identity probe failed", output)
	}
	after, err := snapshotConfig(path)
	if err != nil || after != before {
		t.Fatal("native identity diagnostic modified the file", after, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal("native diagnostic leaked its handle", err)
	}
	if output := configInspectionIdentityDiagnostic(path); !strings.Contains(output, "CreateFile error=") || strings.Contains(output, path) {
		t.Fatal("native identity diagnostic hid the error", output)
	}
}
