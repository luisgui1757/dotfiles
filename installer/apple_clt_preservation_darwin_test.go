package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAppleCLTPreservationHashesLargeRepresentativeFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "clang")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// The hosted Apple compiler exceeds the old 256 MiB fixture limit. A
	// private sparse file reproduces the boundary without touching real tools.
	if err := errors.Join(file.Truncate((256<<20)+1), file.Close()); err != nil {
		t.Fatal(err)
	}
	before := appleCLTNativeSnapshot(t, context.Background(), root, root, []string{"clang"})
	file, err = os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteAt([]byte{1}, 256<<20)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	after := appleCLTNativeSnapshot(t, context.Background(), root, root, []string{"clang"})
	if before.Files["clang"] == "" || before.Files["clang"] == after.Files["clang"] {
		t.Fatal("preservation did not hash bytes beyond the former size limit")
	}
}
