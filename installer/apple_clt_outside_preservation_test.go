package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

type appleCLTOutsidePath struct {
	Snapshot configSnapshot `json:"snapshot"`
	Metadata string         `json:"metadata"`
}

func appleCLTOutsidePaths(data []byte) ([]string, error) {
	if len(data) > 256<<10 || !utf8.Valid(data) {
		return nil, errors.New("outside receipt paths exceed bounded UTF-8 input")
	}
	var paths []string
	for _, name := range strings.Split(string(data), "\n") {
		if name == "" {
			continue
		}
		if len(name) > 4096 || !strings.HasPrefix(name, "/") || name == "/" || path.Clean(name) != name || strings.ContainsAny(name, "\x00\r") || name == "/Library/Developer/CommandLineTools" || strings.HasPrefix(name, "/Library/Developer/CommandLineTools/") {
			return nil, errors.New("outside receipt path is not a canonical outside file")
		}
		paths = append(paths, name)
	}
	paths = sortedUnique(paths)
	if len(paths) > 64 {
		return nil, errors.New("too many outside receipt paths to verify")
	}
	return paths, nil
}

func snapshotAppleCLTOutsidePath(ctx context.Context, name string) (appleCLTOutsidePath, error) {
	snapshot, err := snapshotTree(name, 32<<20, 1)
	if err != nil || snapshot.Kind == "directory" {
		return appleCLTOutsidePath{}, errors.Join(errors.New("cannot preserve outside receipt file"), err)
	}
	if snapshot.Kind == "absent" {
		return appleCLTOutsidePath{Snapshot: snapshot}, nil
	}
	metadata, err := macOSPrerequisiteQuery(ctx, false, "/usr/bin/stat", nil, "-f", "%u:%g:%Lp:%m:%i", name)
	if err != nil {
		return appleCLTOutsidePath{}, err
	}
	fresh, err := snapshotTree(name, 32<<20, 1)
	if err != nil || fresh != snapshot {
		return appleCLTOutsidePath{}, errors.Join(errors.New("outside receipt file changed while recording preservation"), err)
	}
	return appleCLTOutsidePath{Snapshot: snapshot, Metadata: string(metadata)}, nil
}

func verifyAppleCLTOutsidePaths(ctx context.Context, before map[string]appleCLTOutsidePath) error {
	for name, saved := range before {
		actual, err := snapshotAppleCLTOutsidePath(ctx, name)
		if err != nil || actual != saved {
			return errors.Join(fmt.Errorf("Apple provisioning changed preserved outside receipt file %s", name), err)
		}
	}
	return nil
}

func TestNativeAppleCLTRecordOutsideReceiptPreservation(t *testing.T) {
	baseline := appleCLTNativeGuard(t)
	data, err := readDocument(filepath.Join(baseline, "outside-receipt-paths.txt"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := appleCLTOutsidePaths(data)
	if err != nil {
		t.Fatal(err)
	}
	var saved appleCLTNativePreservation
	data, err = readDocument(filepath.Join(baseline, "preservation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Decode(data, &saved); err != nil || saved.OutsideRecorded {
		t.Fatal("outside preservation must be recorded exactly once", err)
	}
	saved.Outside = map[string]appleCLTOutsidePath{}
	for _, name := range paths {
		snapshot, err := snapshotAppleCLTOutsidePath(context.Background(), name)
		if err != nil {
			t.Fatal(name, err)
		}
		// A historical BOM can name a removed installer temporary file. Its
		// absence grants no ownership, and no pre-existing artifact needs saving.
		if snapshot.Snapshot.Kind != "absent" {
			saved.Outside[name] = snapshot
		}
	}
	saved.OutsideRecorded = true
	if err := saveDocument(filepath.Join(baseline, "preservation.json"), saved); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %d outside receipt paths; preserving all %d existing files without moving or deleting them", len(paths), len(saved.Outside))
}

func TestAppleCLTOutsideReceiptPathBoundary(t *testing.T) {
	valid := []byte("/private/tmp/.marker\n/private/tmp/.marker\n/Library/Developer/OtherTool/file\n")
	got, err := appleCLTOutsidePaths(valid)
	if err != nil || !reflect.DeepEqual(got, []string{"/Library/Developer/OtherTool/file", "/private/tmp/.marker"}) {
		t.Fatal(got, err)
	}
	for _, value := range []string{"relative", "/", "/private/../file", "/private//file", "/private/file\r", "/private/file\x00", "/Library/Developer/CommandLineTools", "/Library/Developer/CommandLineTools/usr/bin/clang", string([]byte{0xff}), strings.Repeat("x", (256<<10)+1)} {
		if _, err := appleCLTOutsidePaths([]byte(value)); err == nil {
			t.Fatalf("unsafe receipt path accepted: %.64q", value)
		}
	}
	var many []string
	for i := range 65 {
		many = append(many, fmt.Sprintf("/private/tmp/%d", i))
	}
	if _, err := appleCLTOutsidePaths([]byte(strings.Join(many, "\n"))); err == nil {
		t.Fatal("outside inventory count is not bounded")
	}
}

func TestAppleCLTExistingOutsideReceiptFilesRemainPreserved(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native macOS metadata in private files only")
	}
	for _, change := range []string{"none", "bytes", "permissions", "removed", "link"} {
		t.Run(change, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "outside ü personal file")
			if err := os.WriteFile(name, []byte("preserve original bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if change == "link" {
				link := name + ".link"
				if err := os.Symlink(name, link); err != nil {
					t.Fatal(err)
				}
				name = link
			}
			before, err := snapshotAppleCLTOutsidePath(context.Background(), name)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "bytes":
				err = os.WriteFile(name, []byte("changed"), 0600)
			case "permissions":
				err = os.Chmod(name, 0644)
			case "removed":
				err = os.Remove(name)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = verifyAppleCLTOutsidePaths(context.Background(), map[string]appleCLTOutsidePath{name: before})
			if (change == "none" || change == "link") != (err == nil) {
				t.Fatalf("outside preservation mismatch for %s: %v", change, err)
			}
		})
	}
}
