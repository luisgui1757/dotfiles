package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Reproduce the undefined size values captured from Windows' filesystem
// boundary, while retaining the real directory's mode and modification time.
// No identity verdict or internal inspector function is mocked.
type directorySizeObservation struct {
	os.FileInfo
	size int64
}

func (i directorySizeObservation) Size() int64 { return i.size }

func TestConfigInspectionWindowsDirectorySizeContract(t *testing.T) {
	info, err := os.Lstat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int64{4096, 16384, 65536} {
		before, after := directorySizeObservation{info, 0}, directorySizeObservation{info, size}
		if changed := inspectionMetadataChanged(before, after); changed != (runtime.GOOS != "windows") {
			t.Fatalf("directory size %d was treated contrary to the native metadata contract: changed=%v", size, changed)
		}
	}
}

func TestConfigInspectionStillRejectsDefinedMetadataChanges(t *testing.T) {
	for _, kind := range []string{"file-size", "mode", "directory-time"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "entry")
			if kind == "directory-time" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				writeConfigFixture(t, path, "original content")
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file-size":
				writeConfigFixture(t, path, "larger changed content")
				if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(path, 0400); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(path, before.Mode().Perm()); err != nil {
						t.Error(err)
					}
				})
			case "directory-time":
				writeConfigFixture(t, filepath.Join(path, "new-child"), "new content")
				// Set a deterministic distinct timestamp even on coarse filesystems.
				changed := before.ModTime().Add(2 * time.Second)
				if err := os.Chtimes(path, changed, changed); err != nil {
					t.Fatal(err)
				}
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) || !inspectionMetadataChanged(before, after) {
				t.Fatal("defined metadata change escaped inspection", before, after, err)
			}
		})
	}
}
